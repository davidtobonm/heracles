package agent_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/project"
)

func TestResolveProfilesInheritsDefaultsAndAssignsEveryRole(t *testing.T) {
	t.Parallel()

	config := project.AgentConfig{
		DefaultProfile: "default",
		Profiles: map[string]project.ProfileConfig{
			"default": {
				Provider:     "codex",
				Model:        "gpt-5.4",
				Effort:       "medium",
				Timeout:      "30m",
				ExtraArgs:    []string{"--search"},
				EnvAllowlist: []string{"PATH", "HOME"},
				Concurrency:  2,
			},
			"reviewer": {
				Extends:  "default",
				Provider: "claude",
				Model:    "sonnet",
				Effort:   "high",
			},
		},
		Roles: project.RoleConfig{Reviewer: "reviewer"},
	}
	original := config.Profiles["reviewer"]

	resolved, err := agent.ResolveProfiles(config)
	if err != nil {
		t.Fatalf("ResolveProfiles() error = %v", err)
	}

	for _, role := range []agent.Role{agent.RolePlanner, agent.RoleIssueAuthor, agent.RoleImplementer} {
		profile := resolved.Roles[role]
		if profile.Name != "default" || profile.Provider != "codex" {
			t.Errorf("%s profile = %#v, want default profile", role, profile)
		}
	}
	reviewer := resolved.Roles[agent.RoleReviewer]
	if reviewer.Provider != "claude" || reviewer.Model != "sonnet" || reviewer.Effort != "high" || reviewer.Timeout != 30*time.Minute || reviewer.Concurrency != 2 {
		t.Errorf("reviewer profile = %#v, want inherited profile with overrides", reviewer)
	}
	if !reflect.DeepEqual(config.Profiles["reviewer"], original) {
		t.Errorf("ResolveProfiles() mutated source config: %#v", config.Profiles["reviewer"])
	}
}

func TestResolveProfilesDoesNotCarryProviderSpecificSettingsAcrossProviders(t *testing.T) {
	t.Parallel()

	resolved, err := agent.ResolveProfiles(project.AgentConfig{
		DefaultProfile: "default",
		Profiles: map[string]project.ProfileConfig{
			"default":  {Provider: "codex", Model: "gpt-5.4", Effort: "high", Timeout: "20m"},
			"opencode": {Extends: "default", Provider: "opencode", Model: "anthropic/sonnet", Variant: "high"},
		},
		Roles: project.RoleConfig{Reviewer: "opencode"},
	})
	if err != nil {
		t.Fatalf("ResolveProfiles() error = %v", err)
	}

	profile := resolved.Roles[agent.RoleReviewer]
	if profile.Effort != "" || profile.Model != "anthropic/sonnet" || profile.Variant != "high" || profile.Timeout != 20*time.Minute {
		t.Errorf("provider-changing profile = %#v, want common inheritance without old provider settings", profile)
	}
}

func TestResolveProfilesRejectsInvalidInheritanceAndSettings(t *testing.T) {
	t.Parallel()

	for name, testCase := range map[string]struct {
		config   project.AgentConfig
		expected string
	}{
		"cycle": {
			config: project.AgentConfig{
				DefaultProfile: "a",
				Profiles: map[string]project.ProfileConfig{
					"a": {Extends: "b", Provider: "codex"},
					"b": {Extends: "a"},
				},
			},
			expected: "cycle",
		},
		"invalid timeout": {
			config: project.AgentConfig{
				DefaultProfile: "default",
				Profiles:       map[string]project.ProfileConfig{"default": {Provider: "codex", Timeout: "eventually"}},
			},
			expected: "invalid timeout",
		},
		"unknown role profile": {
			config: project.AgentConfig{
				DefaultProfile: "default",
				Profiles:       map[string]project.ProfileConfig{"default": {Provider: "codex"}},
				Roles:          project.RoleConfig{Reviewer: "missing"},
			},
			expected: "unknown Agent Profile",
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := agent.ResolveProfiles(testCase.config)
			if err == nil || !strings.Contains(err.Error(), testCase.expected) {
				t.Fatalf("ResolveProfiles() error = %v, want %q", err, testCase.expected)
			}
		})
	}
}
