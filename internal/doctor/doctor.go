// Package doctor validates a Heracles project before a Labor starts.
package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/environment"
	"github.com/davidtobonm/heracles/internal/project"
	"github.com/davidtobonm/heracles/internal/tracker"
)

// RequiredLabels lists the shared-state labels every Issue Tracker repository
// must define for Heracles to coordinate Labors.
var RequiredLabels = []string{
	tracker.LabelReady,
	tracker.LabelBlocked,
	tracker.LabelInProgress,
	tracker.LabelReview,
	tracker.LabelDone,
	tracker.LabelHITL,
	tracker.LabelTDDExempt,
	tracker.LabelImplementation,
	tracker.LabelObsolete,
}

// System provides non-agent executable and command probes.
type System interface {
	LookPath(string) (string, error)
	Run(context.Context, string, ...string) error
	Output(context.Context, string, ...string) (string, error)
}

// Diagnostic is one diagnostic result. Warning marks a non-blocking
// deficiency that is still reported but does not fail Report.OK.
type Diagnostic struct {
	Name    string
	OK      bool
	Warning bool
	Message string
}

// Report is the complete diagnostic result.
type Report struct {
	OK     bool
	Checks []Diagnostic
}

// String renders a stable human-readable diagnostic report.
func (report Report) String() string {
	var output strings.Builder
	for _, check := range report.Checks {
		status := "ok"
		switch {
		case check.OK:
			status = "ok"
		case check.Warning:
			status = "warn"
		default:
			status = "failed"
		}
		fmt.Fprintf(&output, "[%s] %s: %s\n", status, check.Name, check.Message)
	}
	return output.String()
}

// CheckProject validates repositories, Issue Tracker access and labels,
// branches, workspaces, agent profiles, provider capabilities and
// authentication, verification commands and environment, auto-merge
// permissions, and CI. Failures are blocking unless marked Warning.
func CheckProject(ctx context.Context, loaded project.LoadedConfig, registry agent.Registry, system System) Report {
	report := Report{OK: true}
	add := func(name string, err error, success string) {
		if err != nil {
			report.OK = false
			report.Checks = append(report.Checks, Diagnostic{Name: name, Message: err.Error()})
			return
		}
		report.Checks = append(report.Checks, Diagnostic{Name: name, OK: true, Message: success})
	}
	addWarn := func(name string, err error, success string) {
		if err != nil {
			report.Checks = append(report.Checks, Diagnostic{Name: name, Warning: true, Message: err.Error()})
			return
		}
		report.Checks = append(report.Checks, Diagnostic{Name: name, OK: true, Message: success})
	}

	_, err := system.LookPath("git")
	add("git executable", err, "available")
	_, err = system.LookPath("gh")
	add("GitHub CLI executable", err, "available")
	if err == nil {
		authErr := system.Run(ctx, "gh", "auth", "status")
		add("GitHub authentication", authErr, "authenticated")
		if authErr == nil {
			add("Issue Tracker", system.Run(ctx, "gh", "repo", "view", loaded.Config.IssueTracker.GitHub, "--json", "nameWithOwner"), loaded.Config.IssueTracker.GitHub)
			checkTrackerLabels(ctx, system, loaded.Config.IssueTracker.GitHub, add, addWarn)
		}
	}

	for _, repository := range loaded.Config.Repositories {
		path, pathErr := loaded.RepositoryPath(repository.Name)
		if pathErr != nil {
			add("Target Repository "+repository.Name, pathErr, "")
			continue
		}
		add("Target Repository "+repository.Name, system.Run(ctx, "git", "-C", path, "rev-parse", "--git-dir"), path)
		if repository.BaseBranch != "" {
			add("Target Repository "+repository.Name+" base branch", system.Run(ctx, "git", "-C", path, "rev-parse", "--verify", "--quiet", repository.BaseBranch), repository.BaseBranch)
		}
		checkVerification(system, repository, add)
		checkAutoMerge(ctx, system, loaded.Config.Delivery.AutoMerge, repository, addWarn, add)
		checkCI(ctx, system, repository, addWarn, add)
	}

	checkWorkspaces(loaded, addWarn, add)

	profiles, err := agent.ResolveProfiles(loaded.Config.Agents)
	if err != nil {
		add("Agent Profiles", err, "")
		return report
	}
	add("Agent Profiles", nil, "all Agent Roles resolve")

	names := make([]string, 0, len(profiles.Profiles))
	for name := range profiles.Profiles {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		profile := profiles.Profiles[name]
		adapter, adapterErr := registry.Adapter(profile.Provider)
		if adapterErr != nil {
			add("Agent Profile "+name, adapterErr, "")
			continue
		}
		if validateErr := adapter.Validate(profile); validateErr != nil {
			add("Agent Profile "+name, validateErr, "")
			continue
		}
		_, executableErr := system.LookPath(adapter.Executable())
		add("Agent Profile "+name, executableErr, profile.Provider+" available and supported")
		if executableErr == nil {
			if diagnostic := diagnoseProvider(ctx, system, profile); diagnostic != nil {
				if diagnostic.OK {
					report.Checks = append(report.Checks, *diagnostic)
				} else {
					report.OK = false
					report.Checks = append(report.Checks, *diagnostic)
				}
			}
		}
	}

	report.Checks = append(report.Checks, Diagnostic{Name: "MCP", OK: true, Message: "stdio MCP control surface available via `heracles mcp`"})
	report.Checks = append(report.Checks, Diagnostic{Name: "Skills", OK: true, Message: "shipped skills available via `heracles skills` and automatic session injection"})

	return report
}

// checkTrackerLabels reports missing required shared-state labels in the
// Issue Tracker repository as a blocking finding, fixable with
// `heracles doctor --fix`. Tracker access failures are reported as warnings
// since they do not necessarily indicate a missing label.
func checkTrackerLabels(ctx context.Context, system System, repository string, add, addWarn func(string, error, string)) {
	missing, err := missingTrackerLabels(ctx, system, repository)
	if err != nil {
		addWarn("Tracker Labels", err, "")
		return
	}
	if len(missing) > 0 {
		add("Tracker Labels", fmt.Errorf("missing labels %s; run `heracles doctor --fix`", strings.Join(missing, ", ")), "")
		return
	}
	add("Tracker Labels", nil, "all required labels exist")
}

func missingTrackerLabels(ctx context.Context, system System, repository string) ([]string, error) {
	output, err := system.Output(ctx, "gh", "label", "list", "--repo", repository, "--json", "name", "--limit", "100")
	if err != nil {
		return nil, err
	}
	var existing []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(output), &existing); err != nil {
		return nil, fmt.Errorf("read tracker labels: %w", err)
	}
	names := make([]string, len(existing))
	for index, label := range existing {
		names[index] = label.Name
	}
	var missing []string
	for _, label := range RequiredLabels {
		if !slices.Contains(names, label) {
			missing = append(missing, label)
		}
	}
	return missing, nil
}

// checkVerification reports unavailable verification executables and missing
// required verification environment variables as blocking findings, since
// either would cause delivery verification to fail predictably.
func checkVerification(system System, repository project.RepositoryConfig, add func(string, error, string)) {
	for _, command := range repository.Verify {
		fields := strings.Fields(command)
		if len(fields) == 0 {
			continue
		}
		_, err := system.LookPath(fields[0])
		add(fmt.Sprintf("Repository %s verification command %q", repository.Name, fields[0]), err, "available")
	}
	if len(repository.VerifyEnv) == 0 {
		return
	}
	missing := environment.Missing(repository.VerifyEnv, os.Environ())
	if len(missing) > 0 {
		add("Repository "+repository.Name+" verification environment", fmt.Errorf("missing required variables: %s", strings.Join(missing, ", ")), "")
		return
	}
	add("Repository "+repository.Name+" verification environment", nil, "all required variables set")
}

// checkAutoMerge warns when a repository does not allow GitHub auto-merge
// while Delivery.AutoMerge is enabled. Heracles falls back to review mode in
// that case, so this is non-blocking.
func checkAutoMerge(ctx context.Context, system System, autoMerge bool, repository project.RepositoryConfig, addWarn, add func(string, error, string)) {
	if !autoMerge || repository.GitHub == "" {
		return
	}
	output, err := system.Output(ctx, "gh", "api", "repos/"+repository.GitHub, "--jq", ".allow_auto_merge")
	if err != nil {
		addWarn("Repository "+repository.Name+" auto-merge", err, "")
		return
	}
	allowed := strings.TrimSpace(output) == "true"
	if !allowed {
		addWarn("Repository "+repository.Name+" auto-merge", errors.New("repository does not allow auto-merge; approved Change Sets will await manual merge"), "")
		return
	}
	add("Repository "+repository.Name+" auto-merge", nil, "allowed")
}

// checkCI warns when a repository has no GitHub Actions workflows
// configured, since correction cycles cannot classify CI failures without
// CI.
func checkCI(ctx context.Context, system System, repository project.RepositoryConfig, addWarn, add func(string, error, string)) {
	if repository.GitHub == "" {
		return
	}
	output, err := system.Output(ctx, "gh", "api", "repos/"+repository.GitHub+"/actions/workflows", "--jq", ".total_count")
	count := strings.TrimSpace(output)
	if err != nil || count == "" || count == "0" {
		addWarn("Repository "+repository.Name+" CI", errors.New("no GitHub Actions workflows configured"), "")
		return
	}
	add("Repository "+repository.Name+" CI", nil, count+" workflow(s) configured")
}

// checkWorkspaces warns when the configured Issue Workspace root does not yet
// exist, fixable with `heracles doctor --fix`.
func checkWorkspaces(loaded project.LoadedConfig, addWarn, add func(string, error, string)) {
	root := loaded.WorkspaceRoot()
	info, err := os.Stat(root)
	switch {
	case err == nil && info.IsDir():
		add("Workspaces", nil, root)
	case err == nil:
		add("Workspaces", fmt.Errorf("%s exists and is not a directory", root), "")
	default:
		addWarn("Workspaces", fmt.Errorf("%s does not exist; run `heracles doctor --fix`", root), "")
	}
}

func diagnoseProvider(ctx context.Context, system System, profile agent.Profile) *Diagnostic {
	switch profile.Provider {
	case "codex":
		return authStatusDiagnostic(ctx, system, profile, "codex", []string{"login", "status"}, "codex login")
	case "kimi":
		return authStatusDiagnostic(ctx, system, profile, "kimi", []string{"auth", "status"}, "kimi auth login")
	case "openclaw":
		return authStatusDiagnostic(ctx, system, profile, "openclaw", []string{"auth", "status"}, "openclaw auth login")
	case "hermes":
		return authStatusDiagnostic(ctx, system, profile, "hermes", []string{"auth", "status"}, "hermes auth login")
	case "claude":
		output, err := system.Output(ctx, "claude", "auth", "status")
		if err != nil {
			return &Diagnostic{Name: "Agent Profile " + profile.Name + " authentication", Message: err.Error()}
		}
		var status struct {
			LoggedIn bool `json:"loggedIn"`
		}
		if err := json.Unmarshal([]byte(output), &status); err != nil {
			return &Diagnostic{Name: "Agent Profile " + profile.Name + " authentication", Message: fmt.Sprintf("read claude auth status: %v", err)}
		}
		if !status.LoggedIn {
			return &Diagnostic{Name: "Agent Profile " + profile.Name + " authentication", Message: "claude is not authenticated; run `claude auth login`"}
		}
		return &Diagnostic{Name: "Agent Profile " + profile.Name + " authentication", OK: true, Message: "authenticated"}
	case "opencode":
		output, err := system.Output(ctx, "opencode", "providers", "list")
		if err != nil {
			return &Diagnostic{Name: "Agent Profile " + profile.Name + " authentication", Message: err.Error()}
		}
		if strings.Contains(output, "0 credentials") {
			return &Diagnostic{Name: "Agent Profile " + profile.Name + " authentication", Message: "opencode has no configured credentials; run `opencode providers login`"}
		}
		if profile.Model == "" {
			return &Diagnostic{Name: "Agent Profile " + profile.Name + " authentication", OK: true, Message: "credentials configured"}
		}
		modelsOutput, err := system.Output(ctx, "opencode", "models")
		if err != nil {
			return &Diagnostic{Name: "Agent Profile " + profile.Name + " model", Message: err.Error()}
		}
		if !strings.Contains(modelsOutput, profile.Model) {
			return &Diagnostic{Name: "Agent Profile " + profile.Name + " model", Message: fmt.Sprintf("opencode model %q is unavailable; run `opencode models` to choose a supported provider/model", profile.Model)}
		}
		return &Diagnostic{Name: "Agent Profile " + profile.Name + " model", OK: true, Message: profile.Model + " available"}
	}
	return nil
}

// authStatusDiagnostic reports whether command's CLI is authenticated by
// running an authentication-status subcommand. Heracles only observes the
// command's success and never reads or stores credential values.
func authStatusDiagnostic(ctx context.Context, system System, profile agent.Profile, command string, statusArgs []string, loginCommand string) *Diagnostic {
	name := "Agent Profile " + profile.Name + " authentication"
	if _, err := system.Output(ctx, command, statusArgs...); err != nil {
		return &Diagnostic{Name: name, Message: fmt.Sprintf("%s is not authenticated; run `%s`", command, loginCommand)}
	}
	return &Diagnostic{Name: name, OK: true, Message: "authenticated"}
}

// Check is the public diagnostic application service.
func Check(ctx context.Context, loaded project.LoadedConfig, registry agent.Registry, system System) Report {
	return CheckProject(ctx, loaded, registry, system)
}

// FixProject performs safe, idempotent repairs for missing Issue Tracker
// labels and the configured Issue Workspace root, then returns an updated
// Report. It never authenticates providers, changes secrets, or makes
// destructive repository changes.
func FixProject(ctx context.Context, loaded project.LoadedConfig, registry agent.Registry, system System) Report {
	if loaded.Config.IssueTracker.GitHub != "" {
		if _, err := system.LookPath("gh"); err == nil {
			if system.Run(ctx, "gh", "auth", "status") == nil {
				if missing, err := missingTrackerLabels(ctx, system, loaded.Config.IssueTracker.GitHub); err == nil {
					for _, label := range missing {
						_ = system.Run(ctx, "gh", "label", "create", label, "--repo", loaded.Config.IssueTracker.GitHub, "--color", "ededed", "--force")
					}
				}
			}
		}
	}

	root := loaded.WorkspaceRoot()
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		_ = os.MkdirAll(root, 0o755)
	}

	return CheckProject(ctx, loaded, registry, system)
}

// OSSystem probes the current machine without invoking an agent session.
type OSSystem struct{}

// LookPath locates an executable on PATH.
func (OSSystem) LookPath(executable string) (string, error) {
	path, err := exec.LookPath(executable)
	if err != nil {
		return "", fmt.Errorf("%s not installed or not on PATH", executable)
	}
	return path, nil
}

// Run executes a non-agent diagnostic command.
func (OSSystem) Run(ctx context.Context, command string, args ...string) error {
	_, err := (OSSystem{}).Output(ctx, command, args...)
	return err
}

// Output executes a non-agent diagnostic command and returns combined output.
func (OSSystem) Output(ctx context.Context, command string, args ...string) (string, error) {
	process := exec.CommandContext(ctx, command, args...)
	output, err := process.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return string(output), fmt.Errorf("%s", message)
	}
	return string(output), nil
}
