package project_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/project"
)

func TestApplyRolePreferencesUsesRoleSpecificProfilesAndMapsOpenCodeEffort(t *testing.T) {
	t.Parallel()

	config := project.Config{Agents: project.AgentConfig{
		DefaultProfile: "default",
		Profiles: map[string]project.ProfileConfig{
			"default": {Provider: "codex", Effort: "high"},
		},
	}}
	err := project.ApplyRolePreferences(&config, map[string]project.ProfileConfig{
		"implementer": {Provider: "opencode", Model: "opencode-go/kimi-k2.6", Effort: "medium"},
		"reviewer":    {Model: "gpt-5.5"},
	})
	if err != nil {
		t.Fatalf("ApplyRolePreferences() error = %v", err)
	}
	profiles, err := agent.ResolveProfiles(config.Agents)
	if err != nil {
		t.Fatalf("ResolveProfiles() error = %v", err)
	}
	implementer := profiles.Roles[agent.RoleImplementer]
	reviewer := profiles.Roles[agent.RoleReviewer]
	if implementer.Provider != "opencode" || implementer.Variant != "medium" || implementer.Effort != "" {
		t.Errorf("implementer = %#v, want OpenCode medium variant", implementer)
	}
	if reviewer.Provider != "codex" || reviewer.Model != "gpt-5.5" || reviewer.Effort != "high" {
		t.Errorf("reviewer = %#v, want inherited Codex high effort", reviewer)
	}
}

func TestApplyRolePreferencesPreservesConfiguredOpenCodeReviewerModel(t *testing.T) {
	t.Parallel()

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

	loaded, err := project.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	err = project.ApplyRolePreferences(&loaded.Config, map[string]project.ProfileConfig{
		"implementer": {Provider: "claude", Model: "sonnet", Effort: "medium"},
		"reviewer":    {Provider: "opencode", Model: "openai/gpt-5.4", Effort: "high"},
	})
	if err != nil {
		t.Fatalf("ApplyRolePreferences() error = %v", err)
	}
	profiles, err := agent.ResolveProfiles(loaded.Config.Agents)
	if err != nil {
		t.Fatalf("ResolveProfiles() error = %v", err)
	}
	reviewer := profiles.Roles[agent.RoleReviewer]
	if reviewer.Provider != "opencode" || reviewer.Model != "openai/gpt-5.4" || reviewer.Variant != "high" || reviewer.Effort != "" {
		t.Fatalf("reviewer = %#v, want OpenCode reviewer model preserved with high variant", reviewer)
	}
}
