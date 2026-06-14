package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/project"
)

func TestApplyPreferencesKeepsUpdatedOpenCodeReviewerModel(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	root := t.TempDir()
	configPath := filepath.Join(root, "heracles.yaml")
	if err := os.WriteFile(configPath, []byte(`version: 1
issue_tracker:
  github: acme/backlog
repositories:
  - name: app
    path: .
    github: acme/app
    base_branch: main
agents:
  default_profile: default
  profiles:
    default:
      provider: codex
      timeout: 1h
      env_allowlist: [PATH, HOME]
      concurrency: 1
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".heracles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".heracles", "preferences.yaml"), []byte(`agents:
  implementer:
    provider: claude
    model: sonnet
    effort: medium
  reviewer:
    provider: opencode
    model: openai/gpt-5.4
    effort: high
`), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := project.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := applyPreferences(&loaded, home, nil); err != nil {
		t.Fatalf("applyPreferences() error = %v", err)
	}
	profiles, err := agent.ResolveProfiles(loaded.Config.Agents)
	if err != nil {
		t.Fatalf("ResolveProfiles() error = %v", err)
	}
	reviewer := profiles.Roles[agent.RoleReviewer]
	if reviewer.Provider != "opencode" || reviewer.Model != "openai/gpt-5.4" || reviewer.Variant != "high" || reviewer.Effort != "" {
		t.Fatalf("reviewer = %#v, want updated OpenCode reviewer model with high variant", reviewer)
	}
}

func TestApplyPreferencesLaunchOverrideTakesPrecedenceForAllRoles(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	root := t.TempDir()
	configPath := filepath.Join(root, "heracles.yaml")
	if err := os.WriteFile(configPath, []byte(`version: 1
issue_tracker:
  github: acme/backlog
repositories:
  - name: app
    path: .
    github: acme/app
    base_branch: main
agents:
  default_profile: default
  profiles:
    default:
      provider: codex
      timeout: 1h
      env_allowlist: [PATH, HOME]
      concurrency: 1
`), 0o644); err != nil {
		t.Fatal(err)
	}

	globalPath, err := project.GlobalPreferencesPath(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := project.WritePreferences(globalPath, project.Preferences{Agents: map[string]project.ProfileConfig{
		"planner": {Provider: "codex", Model: "global-model"},
	}}); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(root, ".heracles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := project.WritePreferences(project.ProjectPreferencesPath(configPath), project.Preferences{Agents: map[string]project.ProfileConfig{
		"planner": {Provider: "codex", Model: "project-model"},
	}}); err != nil {
		t.Fatal(err)
	}

	loaded, err := project.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	launch := map[string]project.ProfileConfig{
		"planner": {Provider: "codex", Model: "launch-model"},
	}
	if err := applyPreferences(&loaded, home, launch); err != nil {
		t.Fatalf("applyPreferences() error = %v", err)
	}
	profiles, err := agent.ResolveProfiles(loaded.Config.Agents)
	if err != nil {
		t.Fatalf("ResolveProfiles() error = %v", err)
	}
	planner := profiles.Roles[agent.RolePlanner]
	if planner.Model != "launch-model" {
		t.Fatalf("planner.Model = %q, want %q (launch override precedence)", planner.Model, "launch-model")
	}
}
