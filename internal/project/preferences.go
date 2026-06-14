package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Preferences stores user-selected Agent Role defaults outside the portable Project Configuration.
type Preferences struct {
	Agents map[string]ProfileConfig `yaml:"agents,omitempty"`
}

// GlobalPreferencesPath returns the per-user preferences path.
func GlobalPreferencesPath(home string) (string, error) {
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
	}
	return filepath.Join(home, ".config", "heracles", "preferences.yaml"), nil
}

// ProjectPreferencesPath returns the local preferences path beside a Project Configuration.
func ProjectPreferencesPath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), ".heracles", "preferences.yaml")
}

// LoadPreferences reads preferences when present.
func LoadPreferences(path string) (Preferences, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return Preferences{}, nil
	}
	if err != nil {
		return Preferences{}, fmt.Errorf("open preferences: %w", err)
	}
	defer file.Close()

	var preferences Preferences
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&preferences); err != nil {
		return Preferences{}, fmt.Errorf("decode preferences: %w", err)
	}
	return preferences, nil
}

// WritePreferences persists preferences.
func WritePreferences(path string, preferences Preferences) error {
	contents, err := yaml.Marshal(preferences)
	if err != nil {
		return fmt.Errorf("encode preferences: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create preferences directory: %w", err)
	}
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		return fmt.Errorf("write preferences: %w", err)
	}
	return nil
}

// MergeRolePreferences overlays later Agent Role preferences over earlier ones.
func MergeRolePreferences(base, override map[string]ProfileConfig) map[string]ProfileConfig {
	merged := make(map[string]ProfileConfig, len(base)+len(override))
	for role, profile := range base {
		merged[role] = profile
	}
	for role, profile := range override {
		current := merged[role]
		if profile.Provider != "" && !strings.EqualFold(profile.Provider, current.Provider) {
			current.Model, current.Effort, current.Variant = "", "", ""
		}
		mergeProfile(&current, profile)
		merged[role] = current
	}
	return merged
}

// ApplyRolePreferences creates role-specific profiles without mutating shared configured profiles.
func ApplyRolePreferences(config *Config, preferences map[string]ProfileConfig) error {
	if len(preferences) == 0 {
		return nil
	}
	if config.Agents.Profiles == nil {
		config.Agents.Profiles = make(map[string]ProfileConfig)
	}
	for role, override := range preferences {
		base, err := roleProfile(config.Agents, role)
		if err != nil {
			return err
		}
		provider := override.Provider
		if provider == "" {
			provider = profileProvider(config.Agents.Profiles, base)
		}
		if strings.EqualFold(provider, "opencode") && override.Effort != "" && override.Variant == "" {
			override.Variant, override.Effort = override.Effort, ""
		}
		name := "__preference_" + role
		profile, exists := config.Agents.Profiles[name]
		if !exists {
			profile.Extends = base
		}
		if override.Provider != "" && !strings.EqualFold(override.Provider, profile.Provider) {
			profile.Model, profile.Effort, profile.Variant = "", "", ""
		}
		mergeProfile(&profile, override)
		config.Agents.Profiles[name] = profile
		assignRole(&config.Agents.Roles, role, name)
	}
	return nil
}

func profileProvider(profiles map[string]ProfileConfig, name string) string {
	seen := make(map[string]bool)
	for name != "" && !seen[name] {
		seen[name] = true
		profile, exists := profiles[name]
		if !exists {
			return ""
		}
		if profile.Provider != "" {
			return profile.Provider
		}
		name = profile.Extends
	}
	return ""
}

func roleProfile(config AgentConfig, role string) (string, error) {
	var assigned string
	switch role {
	case "planner":
		assigned = config.Roles.Planner
	case "issue_author":
		assigned = config.Roles.IssueAuthor
	case "implementer":
		assigned = config.Roles.Implementer
	case "reviewer":
		assigned = config.Roles.Reviewer
	default:
		return "", fmt.Errorf("unknown Agent Role %q", role)
	}
	if assigned == "" {
		assigned = config.DefaultProfile
	}
	return assigned, nil
}

func assignRole(roles *RoleConfig, role, profile string) {
	switch role {
	case "planner":
		roles.Planner = profile
	case "issue_author":
		roles.IssueAuthor = profile
	case "implementer":
		roles.Implementer = profile
	case "reviewer":
		roles.Reviewer = profile
	}
}

func mergeProfile(target *ProfileConfig, source ProfileConfig) {
	if source.Provider != "" {
		target.Provider = source.Provider
	}
	if source.Model != "" {
		target.Model = source.Model
	}
	if source.Effort != "" {
		target.Effort = source.Effort
	}
	if source.Variant != "" {
		target.Variant = source.Variant
	}
}
