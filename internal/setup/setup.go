package setup

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/doctor"
	"github.com/davidtobonm/heracles/internal/issuestage"
	"github.com/davidtobonm/heracles/internal/project"
)

// Options configures one interactive `heracles init` run.
type Options struct {
	WorkingDirectory string
	HomeDirectory    string
	ConfigPath       string
	IO               IO
	Registry         agent.Registry
	System           doctor.System
	Publisher        issuestage.Publisher

	// RunBootstrapBacklog runs the Project Bootstrap Defined Backlog when the
	// user accepts. It is only invoked when at least one repository has no
	// verification commands.
	RunBootstrapBacklog func(ctx context.Context, loaded project.LoadedConfig) error
}

// Result is the outcome of one interactive `heracles init` run.
type Result struct {
	Path      string
	Config    project.Config
	Doctor    doctor.Report
	Cancelled bool
}

var roleOrder = []string{"planner", "issue_author", "implementer", "reviewer"}

// Run interactively discovers, initializes, or reconfigures a Heracles
// Project Configuration, per ADR 0021.
func Run(ctx context.Context, options Options) (Result, error) {
	io := options.IO

	path, err := project.Discover(options.WorkingDirectory, options.ConfigPath)
	isNew := err != nil
	if isNew {
		initResult, err := project.Initialize(ctx, project.InitOptions{
			WorkingDirectory: options.WorkingDirectory,
			ConfigPath:       options.ConfigPath,
		})
		if err != nil {
			return Result{}, fmt.Errorf("initialize Project Configuration: %w", err)
		}
		path = initResult.Path
	}

	loaded, err := project.Load(path)
	if err != nil {
		return Result{}, err
	}

	preferencesPath := project.ProjectPreferencesPath(loaded.Path)
	preferences, err := project.LoadPreferences(preferencesPath)
	if err != nil {
		return Result{}, err
	}

	complete, repair, cancelled, err := chooseMode(io, isNew)
	if err != nil {
		return Result{}, err
	}
	if cancelled {
		return Result{Path: loaded.Path, Config: loaded.Config, Cancelled: true}, nil
	}

	if !repair {
		if err := chooseProfiles(io, options.Registry, options.System, &preferences, loaded, complete); err != nil {
			return Result{}, err
		}
		if err := project.WritePreferences(preferencesPath, preferences); err != nil {
			return Result{}, fmt.Errorf("write Agent Role preferences: %w", err)
		}
	}

	if complete && !repair {
		choosePolicy(io, &loaded.Config)
	}

	if err := chooseVerification(io, filepath.Dir(loaded.Path), &loaded.Config, complete, repair); err != nil {
		return Result{}, err
	}

	if err := project.WriteConfig(loaded.Path, loaded.Config); err != nil {
		return Result{}, fmt.Errorf("write Project Configuration: %w", err)
	}
	loaded, err = project.Load(loaded.Path)
	if err != nil {
		return Result{}, err
	}

	if err := bootstrapUnderVerified(ctx, io, options, loaded); err != nil {
		return Result{}, err
	}

	report := doctorReport(ctx, options, loaded, preferences)
	return Result{Path: loaded.Path, Config: loaded.Config, Doctor: report}, nil
}

// chooseMode presents the top-level Fast/Complete/Repair/Cancel menu.
func chooseMode(io IO, isNew bool) (complete, repair, cancelled bool, err error) {
	if isNew {
		selected, err := SelectOption(io, "Setup", []string{"Fast Setup", "Complete Setup"}, 0)
		if err != nil {
			return false, false, false, err
		}
		return selected == 1, false, false, nil
	}

	selected, err := SelectOption(io, "Reconfigure", []string{"Fast Reconfigure", "Complete Reconfigure", "Repair Missing Values", "Cancel"}, 0)
	if err != nil {
		return false, false, false, err
	}
	switch selected {
	case 1:
		return true, false, false, nil
	case 2:
		return false, true, false, nil
	case 3:
		return false, false, true, nil
	default:
		return false, false, false, nil
	}
}

// chooseProfiles prompts for the Implementer profile and, in Complete mode,
// the remaining Agent Roles, persisting the result into role preferences.
func chooseProfiles(io IO, registry agent.Registry, system doctor.System, preferences *project.Preferences, loaded project.LoadedConfig, complete bool) error {
	availability := DetectProviders(registry, system)
	rolePreferences := make(map[string]project.ProfileConfig)

	implementerCurrent := currentRoleProfile(loaded, *preferences, "implementer")
	implementerProfile, err := ChooseProfile(io, registry, availability, "Implementer", implementerCurrent)
	if err != nil {
		return err
	}
	rolePreferences["implementer"] = implementerProfile

	sameForAll, err := Confirm(io, "Use this configuration for Planner, Issue Author, and Reviewer too?", true)
	if err != nil {
		return err
	}

	switch {
	case sameForAll:
		for _, role := range roleOrder {
			rolePreferences[role] = implementerProfile
		}
	case complete:
		for _, role := range roleOrder {
			if role == "implementer" {
				continue
			}
			current := currentRoleProfile(loaded, *preferences, role)
			if isEmptyProfile(current) {
				current = implementerProfile
			}
			profile, err := ChooseProfile(io, registry, availability, roleLabel(role), current)
			if err != nil {
				return err
			}
			rolePreferences[role] = profile
		}
	}

	preferences.Agents = project.MergeRolePreferences(preferences.Agents, rolePreferences)
	return nil
}

// choosePolicy prompts for Complete-mode-only Labor and delivery policy.
func choosePolicy(io IO, config *project.Config) {
	config.Planning.QuestionBudget, _ = promptInt(io, "Planning Question Budget", config.Planning.QuestionBudget)
	config.Labor.IssueConcurrency, _ = promptInt(io, "Implementation issue concurrency", config.Labor.IssueConcurrency)
	config.Delivery.AutoMerge, _ = Confirm(io, "Automatically merge approved Change Sets?", config.Delivery.AutoMerge)
	config.Workspaces.CleanupSuccess, _ = Confirm(io, "Clean up Issue Workspaces after successful delivery?", config.Workspaces.CleanupSuccess)
	config.Workspaces.PreserveFailed, _ = Confirm(io, "Preserve Issue Workspaces for failed attempts?", config.Workspaces.PreserveFailed)
	config.Workspaces.PreserveBlocked, _ = Confirm(io, "Preserve Issue Workspaces for blocked attempts?", config.Workspaces.PreserveBlocked)
}

// chooseVerification detects and confirms verification commands per
// repository, leaving Verify unset for repositories that need bootstrapping.
func chooseVerification(io IO, configDir string, config *project.Config, complete, repair bool) error {
	for index := range config.Repositories {
		repository := &config.Repositories[index]
		if repair && len(repository.Verify) > 0 {
			continue
		}

		repoPath := repository.Path
		if !filepath.IsAbs(repoPath) {
			repoPath = filepath.Join(configDir, repoPath)
		}
		commands, confident := DetectVerification(repoPath)
		switch {
		case confident:
			fmt.Fprintf(io.Out, "Detected verification commands for %s:\n", repository.Name)
			for _, command := range commands {
				fmt.Fprintf(io.Out, "  %s\n", command)
			}
			use, err := Confirm(io, fmt.Sprintf("Use these verification commands for %s?", repository.Name), true)
			if err != nil {
				return err
			}
			if use {
				repository.Verify = commands
			} else {
				text, err := Text(io, fmt.Sprintf("Verification commands for %s (comma or newline separated, blank to skip)", repository.Name), "")
				if err != nil {
					return err
				}
				repository.Verify = splitList(text)
			}
		default:
			fmt.Fprintf(io.Out, "No verification commands detected for %s.\n", repository.Name)
		}

		if complete && len(repository.Verify) > 0 {
			envText, err := Text(io, fmt.Sprintf("Required environment variables for %s (comma separated, optional)", repository.Name), strings.Join(repository.VerifyEnv, ","))
			if err != nil {
				return err
			}
			repository.VerifyEnv = splitList(envText)
		}
	}
	return nil
}

// bootstrapUnderVerified publishes the Heracles Project Bootstrap PRD and
// issues for repositories with no verification commands, optionally running
// the resulting Defined Backlog immediately.
func bootstrapUnderVerified(ctx context.Context, io IO, options Options, loaded project.LoadedConfig) error {
	var proposals []issuestage.Proposal
	for _, repository := range loaded.Config.Repositories {
		if len(repository.Verify) > 0 {
			continue
		}
		proposals = append(proposals, BuildBootstrapProposal(repository, ""))
	}
	if len(proposals) == 0 {
		return nil
	}

	prdURL, _, err := PublishBootstrap(ctx, options.Publisher, loaded.Config.IssueTracker.GitHub, proposals)
	if err != nil {
		return fmt.Errorf("publish Heracles Project Bootstrap: %w", err)
	}
	fmt.Fprintf(io.Out, "Published Heracles Project Bootstrap: %s\n", prdURL)

	runNow, err := Confirm(io, "Run the Project Bootstrap Defined Backlog now?", true)
	if err != nil {
		return err
	}
	if runNow && options.RunBootstrapBacklog != nil {
		if err := options.RunBootstrapBacklog(ctx, loaded); err != nil {
			return fmt.Errorf("run Project Bootstrap Defined Backlog: %w", err)
		}
	}
	return nil
}

func doctorReport(ctx context.Context, options Options, loaded project.LoadedConfig, preferences project.Preferences) doctor.Report {
	config := loaded.Config
	config.Agents.Profiles = make(map[string]project.ProfileConfig, len(loaded.Config.Agents.Profiles))
	for name, profile := range loaded.Config.Agents.Profiles {
		config.Agents.Profiles[name] = profile
	}
	if err := project.ApplyRolePreferences(&config, preferences.Agents); err != nil {
		return doctor.Report{OK: false, Checks: []doctor.Diagnostic{{Name: "Agent Role preferences", Message: err.Error()}}}
	}
	return doctor.Check(ctx, project.LoadedConfig{Path: loaded.Path, Config: config}, options.Registry, options.System)
}

func currentRoleProfile(loaded project.LoadedConfig, preferences project.Preferences, role string) project.ProfileConfig {
	if profile, ok := preferences.Agents[role]; ok {
		return profile
	}
	name := roleAssignment(loaded.Config.Agents.Roles, role)
	if name == "" {
		name = loaded.Config.Agents.DefaultProfile
	}
	profile, ok := loaded.Config.Agents.Profiles[name]
	if !ok {
		return project.ProfileConfig{}
	}
	return project.ProfileConfig{Provider: profile.Provider, Model: profile.Model, Effort: profile.Effort, Variant: profile.Variant}
}

func roleAssignment(roles project.RoleConfig, role string) string {
	switch role {
	case "planner":
		return roles.Planner
	case "issue_author":
		return roles.IssueAuthor
	case "implementer":
		return roles.Implementer
	case "reviewer":
		return roles.Reviewer
	default:
		return ""
	}
}

func roleLabel(role string) string {
	switch role {
	case "planner":
		return "Planner"
	case "issue_author":
		return "Issue Author"
	case "implementer":
		return "Implementer"
	case "reviewer":
		return "Reviewer"
	default:
		return role
	}
}

func isEmptyProfile(profile project.ProfileConfig) bool {
	return reflect.DeepEqual(profile, project.ProfileConfig{})
}

func promptInt(io IO, question string, current int) (int, error) {
	for {
		text, err := Text(io, question, strconv.Itoa(current))
		if err != nil {
			return current, err
		}
		value, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil || value < 0 {
			fmt.Fprintln(io.Out, "enter a non-negative integer")
			continue
		}
		return value, nil
	}
}

func splitList(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	})
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			result = append(result, field)
		}
	}
	return result
}
