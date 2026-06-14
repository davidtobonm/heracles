package cli

import (
	"flag"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/project"
)

// agentRoles lists every Agent Role that supports configuration preferences and launch overrides.
var agentRoles = []string{"planner", "issue_author", "implementer", "reviewer"}

// profileFields lists the dotted-key fields supported on agents.<role>.<field>.
var profileFields = map[string]bool{
	"provider":      true,
	"model":         true,
	"effort":        true,
	"variant":       true,
	"timeout":       true,
	"extra_args":    true,
	"env_allowlist": true,
	"concurrency":   true,
}

// listFields are agents.<role>.<field> fields that hold a comma-separated list of values.
var listFields = map[string]bool{
	"extra_args":    true,
	"env_allowlist": true,
}

// roleProfileFlags registers dashed --<role>/--<role>-model/--<role>-effort/--<role>-variant
// flags for every Agent Role and returns each role's destination profile.
func roleProfileFlags(flags *flag.FlagSet) map[string]*project.ProfileConfig {
	profiles := make(map[string]*project.ProfileConfig, len(agentRoles))
	for _, role := range agentRoles {
		profiles[role] = profileFlags(flags, role)
	}
	return profiles
}

// dottedAssignment is one parsed agents.<role>.<field>[=value] argument.
type dottedAssignment struct {
	Role     string
	Field    string
	Value    string
	HasValue bool
}

// parseDotted parses arg as an agents.<role>.<field>[=value] configuration key. It returns
// ok=false when arg is not a dotted configuration key at all (e.g. a positional argument),
// and an error when arg looks like a dotted key but names an unsupported role or field.
func parseDotted(arg string) (dottedAssignment, bool, error) {
	key, value, hasValue := arg, "", false
	if index := strings.Index(arg, "="); index >= 0 {
		key, value, hasValue = arg[:index], arg[index+1:], true
	}
	if !strings.HasPrefix(key, "agents.") {
		return dottedAssignment{}, false, nil
	}
	parts := strings.Split(key, ".")
	if len(parts) != 3 {
		return dottedAssignment{}, true, fmt.Errorf("unsupported configuration key %q", key)
	}
	role, field := parts[1], parts[2]
	if !slices.Contains(agentRoles, role) {
		return dottedAssignment{}, true, fmt.Errorf("unsupported configuration key %q: unknown Agent Role %q", key, role)
	}
	if !profileFields[field] {
		return dottedAssignment{}, true, fmt.Errorf("unsupported configuration key %q: unknown field %q", key, field)
	}
	return dottedAssignment{Role: role, Field: field, Value: value, HasValue: hasValue}, true, nil
}

// extractDottedTokens splits args into dotted agents.<role>.<field>[=value] tokens and the
// remaining arguments. Callers validate whether a value is required for their command.
func extractDottedTokens(args []string) ([]dottedAssignment, []string, error) {
	var assignments []dottedAssignment
	var remaining []string
	for _, arg := range args {
		assignment, isDotted, err := parseDotted(arg)
		if err != nil {
			return nil, nil, err
		}
		if !isDotted {
			remaining = append(remaining, arg)
			continue
		}
		assignments = append(assignments, assignment)
	}
	return assignments, remaining, nil
}

// setProfileField applies value to field on profile.
func setProfileField(profile *project.ProfileConfig, field, value string) error {
	switch field {
	case "provider":
		profile.Provider = value
	case "model":
		profile.Model = value
	case "effort":
		profile.Effort = value
	case "variant":
		profile.Variant = value
	case "timeout":
		if _, err := time.ParseDuration(value); err != nil {
			return fmt.Errorf("invalid timeout %q: %w", value, err)
		}
		profile.Timeout = value
	case "concurrency":
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 {
			return fmt.Errorf("invalid concurrency %q: must be a positive integer", value)
		}
		profile.Concurrency = parsed
	case "extra_args":
		profile.ExtraArgs = splitList(value)
	case "env_allowlist":
		profile.EnvAllowlist = splitList(value)
	default:
		return fmt.Errorf("unsupported configuration field %q", field)
	}
	return nil
}

// appendProfileField appends value to a list field on profile.
func appendProfileField(profile *project.ProfileConfig, field, value string) error {
	if !listFields[field] {
		return fmt.Errorf("agents.<role>.%s does not support append; use set", field)
	}
	switch field {
	case "extra_args":
		profile.ExtraArgs = append(profile.ExtraArgs, splitList(value)...)
	case "env_allowlist":
		profile.EnvAllowlist = append(profile.EnvAllowlist, splitList(value)...)
	}
	return nil
}

// unsetProfileField clears field on profile.
func unsetProfileField(profile *project.ProfileConfig, field string) error {
	switch field {
	case "provider":
		profile.Provider = ""
	case "model":
		profile.Model = ""
	case "effort":
		profile.Effort = ""
	case "variant":
		profile.Variant = ""
	case "timeout":
		profile.Timeout = ""
	case "concurrency":
		profile.Concurrency = 0
	case "extra_args":
		profile.ExtraArgs = nil
	case "env_allowlist":
		profile.EnvAllowlist = nil
	default:
		return fmt.Errorf("unsupported configuration field %q", field)
	}
	return nil
}

func splitList(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// profileIsEmpty reports whether profile has no configured fields.
func profileIsEmpty(profile project.ProfileConfig) bool {
	return profile.Extends == "" && profile.Provider == "" && profile.Model == "" &&
		profile.Effort == "" && profile.Variant == "" && profile.Timeout == "" &&
		len(profile.ExtraArgs) == 0 && len(profile.EnvAllowlist) == 0 && profile.Concurrency == 0
}

// normalizeProviderProfile applies provider-specific equivalences so validation and merging
// see the same shape that ApplyRolePreferences eventually produces. OpenCode expresses effort
// as a model variant.
func normalizeProviderProfile(profile project.ProfileConfig) project.ProfileConfig {
	if strings.EqualFold(profile.Provider, "opencode") && profile.Effort != "" && profile.Variant == "" {
		profile.Variant, profile.Effort = profile.Effort, ""
	}
	return profile
}

// validateProfileOverride immediately rejects unsupported providers or provider/model/effort/
// variant combinations within a single Agent Role update. It only validates settings that
// accompany an explicit provider change, since settings for an existing provider are validated
// once the provider is known (e.g. by internal/doctor or internal/agent.ResolveProfiles).
func validateProfileOverride(registry agent.Registry, profile project.ProfileConfig) error {
	if profile.Provider == "" {
		return nil
	}
	profile = normalizeProviderProfile(profile)
	adapter, err := registry.Adapter(profile.Provider)
	if err != nil {
		return err
	}
	timeout := time.Hour
	if profile.Timeout != "" {
		parsed, err := time.ParseDuration(profile.Timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout %q: %w", profile.Timeout, err)
		}
		timeout = parsed
	}
	return adapter.Validate(agent.Profile{
		Provider: strings.ToLower(profile.Provider),
		Model:    profile.Model,
		Effort:   profile.Effort,
		Variant:  profile.Variant,
		Timeout:  timeout,
	})
}

// mergeDottedIntoProfiles merges dotted assignments into per-role profiles, failing when a
// dashed flag and a dotted assignment conflict for the same field.
func mergeDottedIntoProfiles(profiles map[string]project.ProfileConfig, assignments []dottedAssignment) error {
	for _, assignment := range assignments {
		profile := profiles[assignment.Role]
		if conflict, value := dottedConflict(profile, assignment); conflict {
			return fmt.Errorf("conflicting values for agents.%s.%s: %q and %q", assignment.Role, assignment.Field, value, assignment.Value)
		}
		if err := setProfileField(&profile, assignment.Field, assignment.Value); err != nil {
			return err
		}
		profiles[assignment.Role] = profile
	}
	return nil
}

// dottedConflict reports whether profile already has a non-empty value for assignment.Field
// that differs from assignment.Value, indicating a dashed flag and dotted key disagree.
func dottedConflict(profile project.ProfileConfig, assignment dottedAssignment) (bool, string) {
	var current string
	switch assignment.Field {
	case "provider":
		current = profile.Provider
	case "model":
		current = profile.Model
	case "effort":
		current = profile.Effort
	case "variant":
		current = profile.Variant
	case "timeout":
		current = profile.Timeout
	default:
		return false, ""
	}
	if current != "" && current != assignment.Value {
		return true, current
	}
	return false, ""
}
