// Package doctor validates a Heracles project before a Labor starts.
package doctor

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strings"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/project"
)

// System provides non-agent executable and command probes.
type System interface {
	LookPath(string) (string, error)
	Run(context.Context, string, ...string) error
}

// Diagnostic is one diagnostic result.
type Diagnostic struct {
	Name    string
	OK      bool
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
		if !check.OK {
			status = "failed"
		}
		fmt.Fprintf(&output, "[%s] %s: %s\n", status, check.Name, check.Message)
	}
	return output.String()
}

// Check validates repositories, GitHub authentication, profiles, capabilities, and executables.
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

	_, err := system.LookPath("git")
	add("git executable", err, "available")
	_, err = system.LookPath("gh")
	add("GitHub CLI executable", err, "available")
	if err == nil {
		authErr := system.Run(ctx, "gh", "auth", "status")
		add("GitHub authentication", authErr, "authenticated")
		if authErr == nil {
			add("Issue Tracker", system.Run(ctx, "gh", "repo", "view", loaded.Config.IssueTracker.GitHub, "--json", "nameWithOwner"), loaded.Config.IssueTracker.GitHub)
		}
	}

	for _, repository := range loaded.Config.Repositories {
		path, pathErr := loaded.RepositoryPath(repository.Name)
		if pathErr != nil {
			add("Target Repository "+repository.Name, pathErr, "")
			continue
		}
		add("Target Repository "+repository.Name, system.Run(ctx, "git", "-C", path, "rev-parse", "--git-dir"), path)
	}

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
	}
	return report
}

// Check is the public diagnostic application service.
func Check(ctx context.Context, loaded project.LoadedConfig, registry agent.Registry, system System) Report {
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
	process := exec.CommandContext(ctx, command, args...)
	output, err := process.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("%s", message)
	}
	return nil
}
