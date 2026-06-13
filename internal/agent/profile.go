// Package agent resolves profiles and runs capability-aware agent CLI adapters.
package agent

import (
	"fmt"
	"strings"
	"time"

	"github.com/davidtobonm/heracles/internal/project"
)

// Role is one responsibility within a Labor.
type Role string

const (
	RolePlanner     Role = "planner"
	RoleIssueAuthor Role = "issue_author"
	RoleImplementer Role = "implementer"
	RoleReviewer    Role = "reviewer"
)

var roles = []Role{RolePlanner, RoleIssueAuthor, RoleImplementer, RoleReviewer}

// Profile is a fully inherited and parsed Agent Profile.
type Profile struct {
	Name         string
	Provider     string
	Model        string
	Effort       string
	Variant      string
	Timeout      time.Duration
	ExtraArgs    []string
	EnvAllowlist []string
	Concurrency  int
}

// Profiles contains resolved profiles and role assignments.
type Profiles struct {
	Profiles map[string]Profile
	Roles    map[Role]Profile
}

// ResolveProfiles inherits, parses, and assigns every configured Agent Profile.
func ResolveProfiles(config project.AgentConfig) (Profiles, error) {
	if config.DefaultProfile == "" {
		return Profiles{}, fmt.Errorf("agents.default_profile is required")
	}
	if len(config.Profiles) == 0 {
		return Profiles{}, fmt.Errorf("agents.profiles requires at least one Agent Profile")
	}

	resolved := Profiles{
		Profiles: make(map[string]Profile, len(config.Profiles)),
		Roles:    make(map[Role]Profile, len(roles)),
	}
	visiting := make(map[string]bool, len(config.Profiles))

	var resolve func(string) (Profile, error)
	resolve = func(name string) (Profile, error) {
		if profile, exists := resolved.Profiles[name]; exists {
			return profile, nil
		}
		source, exists := config.Profiles[name]
		if !exists {
			return Profile{}, fmt.Errorf("unknown Agent Profile %q", name)
		}
		if visiting[name] {
			return Profile{}, fmt.Errorf("Agent Profile inheritance cycle at %q", name)
		}
		visiting[name] = true
		defer delete(visiting, name)

		profile := Profile{Name: name, Timeout: time.Hour, Concurrency: 1}
		if source.Extends != "" {
			parent, err := resolve(source.Extends)
			if err != nil {
				return Profile{}, fmt.Errorf("Agent Profile %q: %w", name, err)
			}
			profile = parent
			profile.Name = name
			profile.ExtraArgs = clone(parent.ExtraArgs)
			profile.EnvAllowlist = clone(parent.EnvAllowlist)
			if source.Provider != "" && strings.ToLower(source.Provider) != parent.Provider {
				profile.Model = ""
				profile.Effort = ""
				profile.Variant = ""
			}
		}
		if err := apply(&profile, source); err != nil {
			return Profile{}, fmt.Errorf("Agent Profile %q: %w", name, err)
		}
		if profile.Provider == "" {
			return Profile{}, fmt.Errorf("Agent Profile %q requires provider", name)
		}
		if profile.Timeout <= 0 {
			return Profile{}, fmt.Errorf("Agent Profile %q timeout must be positive", name)
		}
		if profile.Concurrency < 1 {
			return Profile{}, fmt.Errorf("Agent Profile %q concurrency must be positive", name)
		}
		resolved.Profiles[name] = profile
		return profile, nil
	}

	for name := range config.Profiles {
		if _, err := resolve(name); err != nil {
			return Profiles{}, err
		}
	}

	assignments := map[Role]string{
		RolePlanner:     config.Roles.Planner,
		RoleIssueAuthor: config.Roles.IssueAuthor,
		RoleImplementer: config.Roles.Implementer,
		RoleReviewer:    config.Roles.Reviewer,
	}
	for _, role := range roles {
		name := assignments[role]
		if name == "" {
			name = config.DefaultProfile
		}
		profile, exists := resolved.Profiles[name]
		if !exists {
			return Profiles{}, fmt.Errorf("Agent Role %q references unknown Agent Profile %q", role, name)
		}
		resolved.Roles[role] = profile
	}
	return resolved, nil
}

func apply(profile *Profile, source project.ProfileConfig) error {
	if source.Provider != "" {
		profile.Provider = strings.ToLower(source.Provider)
	}
	if source.Model != "" {
		profile.Model = source.Model
	}
	if source.Effort != "" {
		profile.Effort = source.Effort
	}
	if source.Variant != "" {
		profile.Variant = source.Variant
	}
	if source.Timeout != "" {
		parsed, err := time.ParseDuration(source.Timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout %q: %w", source.Timeout, err)
		}
		profile.Timeout = parsed
	}
	if source.ExtraArgs != nil {
		profile.ExtraArgs = clone(source.ExtraArgs)
	}
	if source.EnvAllowlist != nil {
		profile.EnvAllowlist = clone(source.EnvAllowlist)
	}
	if source.Concurrency != 0 {
		profile.Concurrency = source.Concurrency
	}
	return nil
}

func clone(values []string) []string {
	return append([]string(nil), values...)
}
