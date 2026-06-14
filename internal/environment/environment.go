// Package environment resolves which launch-environment variables a process
// receives and identifies values that look like secrets.
package environment

import "strings"

// EssentialVariables lists process variables every launched command receives
// regardless of its Agent Profile or Target Repository allowlist.
var EssentialVariables = []string{
	"HOME",
	"PATH",
	"LANG",
	"LC_ALL",
	"LC_CTYPE",
	"SHELL",
	"TERM",
	"TMPDIR",
	"USER",
	"LOGNAME",
	"XDG_CONFIG_HOME",
	"XDG_CACHE_HOME",
	"XDG_DATA_HOME",
	"SSH_AUTH_SOCK",
}

// Filter returns the entries from source whose variable names are in
// EssentialVariables or allowlist, preserving source values.
func Filter(allowlist, source []string) []string {
	values := make(map[string]string, len(source))
	for _, entry := range source {
		name, _, found := strings.Cut(entry, "=")
		if found {
			values[name] = entry
		}
	}
	names := append([]string(nil), EssentialVariables...)
	names = append(names, allowlist...)
	seen := make(map[string]bool, len(names))
	filtered := make([]string, 0, len(names))
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		if entry, exists := values[name]; exists {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// Missing returns the names from required whose value is absent or empty in
// source, in the order given.
func Missing(required, source []string) []string {
	values := make(map[string]string, len(source))
	for _, entry := range source {
		name, value, found := strings.Cut(entry, "=")
		if found {
			values[name] = value
		}
	}
	var missing []string
	for _, name := range required {
		if values[name] == "" {
			missing = append(missing, name)
		}
	}
	return missing
}

// secretNamePatterns lists case-insensitive substrings that mark an
// environment variable name as holding a secret value.
var secretNamePatterns = []string{"TOKEN", "SECRET", "PASSWORD", "PASSWD", "KEY", "CREDENTIAL", "CERT"}

// IsSecretName reports whether name looks like it holds a secret value, based
// on common naming conventions for credentials.
func IsSecretName(name string) bool {
	upper := strings.ToUpper(name)
	for _, pattern := range secretNamePatterns {
		if strings.Contains(upper, pattern) {
			return true
		}
	}
	return false
}

// SecretValues returns the non-empty values from entries in env whose
// variable names look like secrets, for use with internal/redact.
func SecretValues(env []string) []string {
	var values []string
	for _, entry := range env {
		name, value, found := strings.Cut(entry, "=")
		if found && value != "" && IsSecretName(name) {
			values = append(values, value)
		}
	}
	return values
}
