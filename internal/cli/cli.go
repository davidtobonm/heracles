package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/buildinfo"
	"github.com/davidtobonm/heracles/internal/control"
	"github.com/davidtobonm/heracles/internal/doctor"
	"github.com/davidtobonm/heracles/internal/install"
	"github.com/davidtobonm/heracles/internal/mcp"
	"github.com/davidtobonm/heracles/internal/project"
	"github.com/davidtobonm/heracles/internal/update"
	"gopkg.in/yaml.v3"
)

const help = `Heracles coordinates agent-driven software delivery.

Usage:
  heracles [command]

Available Commands:
  heracles plan       Run or resume a Planning Stage
  heracles issues     Run or resume an Issue Stage
  heracles run        Run the defined Implementation Stage backlog
  heracles labor      Start or resume an end-to-end Labor
  heracles list       List durable Labors, issues, Change Sets, gates, logs, or evidence
  heracles inspect    Inspect one durable workflow record
  heracles mcp serve  Start the stdio MCP Control Surface
  heracles approve    Approve a Planning or Issue publication gate
  heracles reject     Reject a Planning or Issue publication gate for revision
  heracles retry      Retry a failed or blocked issue attempt
  heracles resume     Resume an interrupted or blocked Labor
  heracles cancel     Cancel a Labor
  heracles config     Show or set global and project Agent Role preferences
  heracles doctor     Validate a project before starting a Labor
  heracles init       Initialize a portable Project Configuration
  heracles install    Install the Heracles binary into a user or system command location
  heracles update     Check for or apply Heracles self-updates
  heracles version    Print Heracles version information
`

// Options supplies process-level dependencies to the CLI.
type Options struct {
	WorkingDirectory string
	HomeDirectory    string
	DoctorSystem     doctor.System
	Control          control.Surface
	Input            io.Reader
	Executable       string
	UpdateSource     update.Source
	UpdateCachePath  string
	Now              func() time.Time
}

// Run executes the Heracles CLI and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	return RunWithOptions(args, stdout, stderr, Options{})
}

// RunWithOptions executes the Heracles CLI with explicit process-level options.
func RunWithOptions(args []string, stdout, stderr io.Writer, options Options) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprint(stdout, help)
		return 0
	}

	if args[0] == "init" {
		return runInit(args[1:], stdout, stderr, options)
	}

	if args[0] == "doctor" {
		return runDoctor(args[1:], stdout, stderr, options)
	}

	if args[0] == "config" {
		return runConfig(args[1:], stdout, stderr, options)
	}

	if args[0] == "install" {
		return runInstall(args[1:], stdout, stderr, options)
	}

	if args[0] == "update" {
		return runUpdate(args[1:], stdout, stderr, options)
	}

	if args[0] == "version" {
		fmt.Fprintln(stdout, buildinfo.String())
		return 0
	}

	if args[0] == "mcp" {
		return runMCP(args[1:], stdout, stderr, options)
	}

	for _, command := range []string{"plan", "issues", "run", "labor", "list", "inspect", "approve", "reject", "retry", "resume", "cancel"} {
		if args[0] == command {
			return runControl(command, args[1:], stdout, stderr, options)
		}
	}

	fmt.Fprintf(stderr, "unknown command %q\n", args[0])
	return 2
}

func runMCP(args []string, stdout, stderr io.Writer, options Options) int {
	if len(args) == 0 || args[0] != "serve" {
		fmt.Fprintln(stderr, "usage: heracles mcp serve [--config path]")
		return 2
	}
	flags := flag.NewFlagSet("heracles mcp serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "select Project Configuration path")
	if err := flags.Parse(args[1:]); errors.Is(err, flag.ErrHelp) {
		return 0
	} else if err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "heracles mcp serve does not accept positional arguments")
		return 2
	}
	surface := options.Control
	owned := false
	if surface == nil {
		surface = control.NewDynamic(options.WorkingDirectory, *configPath)
		owned = true
	}
	if owned {
		defer surface.Close()
	}
	input := options.Input
	if input == nil {
		input = os.Stdin
	}
	if err := (mcp.Server{Surface: surface, Version: buildinfo.String()}).Serve(context.Background(), input, stdout); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runControl(command string, args []string, stdout, stderr io.Writer, options Options) int {
	flags := flag.NewFlagSet("heracles "+command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "select Project Configuration path")
	jsonOutput := flags.Bool("json", false, "emit stable machine-readable JSON")
	id := flags.String("id", "", "workflow record ID")
	problem := flags.String("problem", "", "problem description")
	prdPath := flags.String("prd", "", "approved PRD path")
	reason := flags.String("reason", "", "decision or operation reason")
	implementerProvider := flags.String("implementer", "", "override Implementer provider for this launch")
	implementerModel := flags.String("implementer-model", "", "override Implementer model for this launch")
	implementerEffort := flags.String("implementer-effort", "", "override Implementer effort for this launch")
	implementerVariant := flags.String("implementer-variant", "", "override Implementer variant for this launch")
	reviewerProvider := flags.String("reviewer", "", "override Reviewer provider for this launch")
	reviewerModel := flags.String("reviewer-model", "", "override Reviewer model for this launch")
	reviewerEffort := flags.String("reviewer-effort", "", "override Reviewer effort for this launch")
	reviewerVariant := flags.String("reviewer-variant", "", "override Reviewer variant for this launch")
	limit := flags.Int("limit", 0, "attempt at most this many issues during this run")
	if err := flags.Parse(interspersedFlags(args)); errors.Is(err, flag.ErrHelp) {
		return 0
	} else if err != nil {
		return 2
	}

	operation := control.Operation{Name: command, ID: *id, Problem: *problem, Reason: *reason, Limit: *limit}
	if operation.Limit < 0 {
		fmt.Fprintln(stderr, "--limit must be positive")
		return 2
	}
	positionals := flags.Args()
	switch command {
	case "list":
		if len(positionals) != 1 {
			fmt.Fprintln(stderr, "heracles list requires one kind")
			return 2
		}
		operation.Kind = positionals[0]
	case "inspect":
		if len(positionals) != 2 {
			fmt.Fprintln(stderr, "heracles inspect requires kind and ID")
			return 2
		}
		operation.Kind, operation.ID = positionals[0], positionals[1]
	case "approve", "reject":
		if len(positionals) != 2 {
			fmt.Fprintf(stderr, "heracles %s requires gate kind and ID\n", command)
			return 2
		}
		operation.Kind, operation.ID, operation.Decision = positionals[0], positionals[1], command
	case "retry", "resume", "cancel":
		if len(positionals) != 1 {
			fmt.Fprintf(stderr, "heracles %s requires one ID\n", command)
			return 2
		}
		operation.ID = positionals[0]
	default:
		if len(positionals) != 0 {
			fmt.Fprintf(stderr, "heracles %s does not accept positional arguments\n", command)
			return 2
		}
	}
	if command == "issues" {
		if *prdPath != "" {
			contents, err := os.ReadFile(*prdPath)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			operation.PRD = string(contents)
		}
	}
	if command == "labor" && (operation.ID == "" || operation.Problem == "") {
		fmt.Fprintln(stderr, "heracles labor requires --id and --problem")
		return 2
	}
	if (command == "plan" || command == "issues") && operation.ID == "" {
		fmt.Fprintf(stderr, "heracles %s requires --id\n", command)
		return 2
	}

	overrides := map[string]project.ProfileConfig{
		"implementer": {Provider: *implementerProvider, Model: *implementerModel, Effort: *implementerEffort, Variant: *implementerVariant},
		"reviewer":    {Provider: *reviewerProvider, Model: *reviewerModel, Effort: *reviewerEffort, Variant: *reviewerVariant},
	}
	overrides = nonEmptyProfiles(overrides)
	surface, owned, err := controlSurface(options, *configPath, overrides)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if owned {
		defer surface.Close()
	}
	result, err := surface.Execute(context.Background(), operation)
	if err != nil {
		if *jsonOutput {
			_ = json.NewEncoder(stdout).Encode(control.Result{Operation: operation.Name, Kind: operation.Kind, ID: operation.ID, Status: "error", Data: map[string]string{"error": err.Error()}})
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *jsonOutput {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "%s", result.Operation)
	if result.ID != "" {
		fmt.Fprintf(stdout, " %s", result.ID)
	}
	fmt.Fprintf(stdout, ": %s\n", result.Status)
	if result.Data != nil && (command == "list" || command == "inspect") {
		contents, _ := json.MarshalIndent(result.Data, "", "  ")
		fmt.Fprintf(stdout, "%s\n", contents)
	}
	return 0
}

func interspersedFlags(args []string) []string {
	valueFlags := map[string]bool{
		"--config": true, "--id": true, "--problem": true, "--prd": true, "--reason": true, "--limit": true,
		"--implementer": true, "--implementer-model": true, "--implementer-effort": true, "--implementer-variant": true,
		"--reviewer": true, "--reviewer-model": true, "--reviewer-effort": true, "--reviewer-variant": true,
	}
	var flags []string
	var positionals []string
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if argument == "--json" || argument == "-json" || argument == "--help" || argument == "-h" {
			flags = append(flags, argument)
			continue
		}
		if valueFlags[argument] || valueFlags["--"+argument] {
			flags = append(flags, argument)
			if index+1 < len(args) {
				index++
				flags = append(flags, args[index])
			}
			continue
		}
		if len(argument) > 2 && argument[:2] == "--" {
			flags = append(flags, argument)
			continue
		}
		positionals = append(positionals, argument)
	}
	return append(flags, positionals...)
}

func applyPreferences(loaded *project.LoadedConfig, home string, launch map[string]project.ProfileConfig) error {
	globalPath, err := project.GlobalPreferencesPath(home)
	if err != nil {
		return err
	}
	global, err := project.LoadPreferences(globalPath)
	if err != nil {
		return err
	}
	local, err := project.LoadPreferences(project.ProjectPreferencesPath(loaded.Path))
	if err != nil {
		return err
	}
	preferences := project.MergeRolePreferences(global.Agents, local.Agents)
	preferences = project.MergeRolePreferences(preferences, launch)
	return project.ApplyRolePreferences(&loaded.Config, preferences)
}

func runConfig(args []string, stdout, stderr io.Writer, options Options) int {
	if len(args) == 0 || (args[0] != "set" && args[0] != "show") {
		fmt.Fprintln(stderr, "usage: heracles config <set|show> (--global|--project) [Agent Role options]")
		return 2
	}
	command := args[0]
	flags := flag.NewFlagSet("heracles config "+command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	global := flags.Bool("global", false, "use global preferences")
	projectScope := flags.Bool("project", false, "use discovered project preferences")
	configPath := flags.String("config", "", "select Project Configuration path")
	implementer := profileFlags(flags, "implementer")
	reviewer := profileFlags(flags, "reviewer")
	if err := flags.Parse(args[1:]); errors.Is(err, flag.ErrHelp) {
		return 0
	} else if err != nil {
		return 2
	}
	if flags.NArg() != 0 || *global == *projectScope {
		fmt.Fprintln(stderr, "select exactly one of --global or --project")
		return 2
	}

	path, err := preferencesPath(options, *configPath, *global)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	preferences, err := project.LoadPreferences(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if command == "show" {
		contents, err := yaml.Marshal(preferences)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprint(stdout, string(contents))
		return 0
	}

	updates := map[string]project.ProfileConfig{"implementer": *implementer, "reviewer": *reviewer}
	updates = nonEmptyProfiles(updates)
	if len(updates) == 0 {
		fmt.Fprintln(stderr, "heracles config set requires at least one Agent Role option")
		return 2
	}
	preferences.Agents = project.MergeRolePreferences(preferences.Agents, updates)
	if err := project.WritePreferences(path, preferences); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "Updated preferences: %s\n", path)
	return 0
}

func profileFlags(flags *flag.FlagSet, role string) *project.ProfileConfig {
	profile := &project.ProfileConfig{}
	flags.StringVar(&profile.Provider, role, "", "set "+role+" provider")
	flags.StringVar(&profile.Model, role+"-model", "", "set "+role+" model")
	flags.StringVar(&profile.Effort, role+"-effort", "", "set "+role+" effort")
	flags.StringVar(&profile.Variant, role+"-variant", "", "set "+role+" variant")
	return profile
}

func nonEmptyProfiles(profiles map[string]project.ProfileConfig) map[string]project.ProfileConfig {
	result := make(map[string]project.ProfileConfig)
	for role, profile := range profiles {
		if profile.Provider != "" || profile.Model != "" || profile.Effort != "" || profile.Variant != "" {
			result[role] = profile
		}
	}
	return result
}

func preferencesPath(options Options, explicitConfig string, global bool) (string, error) {
	if global {
		return project.GlobalPreferencesPath(options.HomeDirectory)
	}
	path, err := project.Discover(options.WorkingDirectory, explicitConfig)
	if err != nil {
		return "", err
	}
	return project.ProjectPreferencesPath(path), nil
}

func controlSurface(options Options, explicitConfig string, launch map[string]project.ProfileConfig) (control.Surface, bool, error) {
	if options.Control != nil {
		return options.Control, false, nil
	}
	path, err := project.Discover(options.WorkingDirectory, explicitConfig)
	if err != nil {
		return nil, false, err
	}
	loaded, err := project.Load(path)
	if err != nil {
		return nil, false, err
	}
	if err := applyPreferences(&loaded, options.HomeDirectory, launch); err != nil {
		return nil, false, err
	}
	surface, err := control.NewLocal(context.Background(), loaded)
	return surface, true, err
}

func runDoctor(args []string, stdout, stderr io.Writer, options Options) int {
	flags := flag.NewFlagSet("heracles doctor", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "select Project Configuration path")
	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		return 0
	} else if err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "heracles doctor does not accept positional arguments")
		return 2
	}

	path, err := project.Discover(options.WorkingDirectory, *configPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	loaded, err := project.Load(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := applyPreferences(&loaded, options.HomeDirectory, nil); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	system := options.DoctorSystem
	if system == nil {
		system = doctor.OSSystem{}
	}
	report := doctor.Check(context.Background(), loaded, agent.DefaultRegistry(), system)
	fmt.Fprint(stdout, report.String())
	if !report.OK {
		return 1
	}
	return 0
}

func runInit(args []string, stdout, stderr io.Writer, options Options) int {
	flags := flag.NewFlagSet("heracles init", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var repositories repeatedString
	configPath := flags.String("config", "", "write Project Configuration to path")
	tracker := flags.String("tracker", "", "GitHub Issue Tracker as owner/repository")
	flags.Var(&repositories, "repo", "Target Repository path; repeat for multiple repositories")

	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		return 0
	} else if err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "heracles init does not accept positional arguments")
		return 2
	}

	result, err := project.Initialize(context.Background(), project.InitOptions{
		WorkingDirectory: options.WorkingDirectory,
		ConfigPath:       *configPath,
		Tracker:          *tracker,
		Repositories:     repositories,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	fmt.Fprintf(stdout, "Initialized Project Configuration at %s\n", result.Path)
	return 0
}

func runInstall(args []string, stdout, stderr io.Writer, options Options) int {
	flags := flag.NewFlagSet("heracles install", flag.ContinueOnError)
	flags.SetOutput(stderr)
	system := flags.Bool("system", false, "install into the system command location instead of the user location")
	dir := flags.String("dir", "", "install into an explicit directory instead of the default location")
	jsonOutput := flags.Bool("json", false, "emit stable machine-readable JSON")
	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		return 0
	} else if err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "heracles install does not accept positional arguments")
		return 2
	}

	scope := install.ScopeUser
	if *system {
		scope = install.ScopeSystem
	}
	homeDir, _ := os.UserHomeDir()
	env := install.Environment{GOOS: runtime.GOOS, Getenv: os.Getenv, HomeDir: homeDir}
	target, err := install.Resolve(scope, *dir, env)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	source := options.Executable
	if source == "" {
		source, err = os.Executable()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	if err := install.Install(source, target); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	onPath := install.OnPath(target.Dir, env)
	if *jsonOutput {
		if err := json.NewEncoder(stdout).Encode(map[string]any{"path": target.Path, "directory": target.Dir, "on_path": onPath}); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(stdout, "Installed heracles to %s\n", target.Path)
	if !onPath {
		fmt.Fprintf(stdout, "%s is not on PATH; add it to use the heracles command directly.\n", target.Dir)
	}
	return 0
}

func runUpdate(args []string, stdout, stderr io.Writer, options Options) int {
	flags := flag.NewFlagSet("heracles update", flag.ContinueOnError)
	flags.SetOutput(stderr)
	apply := flags.Bool("apply", false, "download, verify, and install the latest release in place of the running binary")
	check := flags.Bool("check", false, "force a fresh update check, bypassing the cache")
	jsonOutput := flags.Bool("json", false, "emit stable machine-readable JSON")
	owner := flags.String("owner", "davidtobonm", "release repository owner")
	repo := flags.String("repo", "heracles", "release repository name")
	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		return 0
	} else if err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "heracles update does not accept positional arguments")
		return 2
	}

	source := options.UpdateSource
	if source == nil {
		source = update.GitHubSource{Owner: *owner, Repo: *repo}
	}
	cachePath := options.UpdateCachePath
	if cachePath == "" {
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		cachePath = filepath.Join(cacheDir, "heracles", "update.json")
	}
	now := time.Now
	if options.Now != nil {
		now = options.Now
	}

	ctx := context.Background()
	result, err := update.Check(ctx, source, cachePath, buildinfo.Version(), now(), update.DefaultInterval, *check || *apply)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	applied := false
	if *apply && result.UpdateAvailable {
		release, err := source.Latest(ctx)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		binary, err := update.DownloadVerified(ctx, source, release, runtime.GOOS, runtime.GOARCH)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		executable := options.Executable
		if executable == "" {
			executable, err = os.Executable()
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
		}
		if err := update.Apply(executable, binary); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		applied = true
	}

	if *jsonOutput {
		payload := map[string]any{
			"current_version":  result.CurrentVersion,
			"latest_version":   result.LatestVersion,
			"update_available": result.UpdateAvailable,
			"checked":          result.Checked,
			"applied":          applied,
		}
		if err := json.NewEncoder(stdout).Encode(payload); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}

	switch {
	case applied:
		fmt.Fprintf(stdout, "updated heracles %s -> %s\n", result.CurrentVersion, result.LatestVersion)
	case *apply:
		fmt.Fprintln(stdout, "heracles is already up to date")
	case result.UpdateAvailable:
		fmt.Fprintf(stdout, "update available: %s -> %s\n", result.CurrentVersion, result.LatestVersion)
	}
	return 0
}

type repeatedString []string

func (values *repeatedString) String() string {
	return fmt.Sprint([]string(*values))
}

func (values *repeatedString) Set(value string) error {
	*values = append(*values, value)
	return nil
}
