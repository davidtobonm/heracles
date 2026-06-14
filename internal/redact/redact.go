// Package redact scrubs secret values from text before it is displayed or
// persisted.
package redact

import (
	"sort"
	"strings"
)

// Placeholder replaces every redacted secret value.
const Placeholder = "***REDACTED***"

// minimumLength is the shortest value that gets redacted, so that trivial
// substrings (e.g. "1" or "") are never scrubbed from unrelated text.
const minimumLength = 4

// Redactor scrubs a fixed set of secret values from text.
type Redactor struct {
	values []string
}

// New creates a Redactor for the given secret values. Values shorter than
// the minimum length and duplicates are ignored.
func New(values []string) *Redactor {
	seen := make(map[string]bool, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if len(value) < minimumLength || seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	// Replace longer values first so a secret that is a prefix of another
	// secret value is not left partially redacted.
	sort.Slice(unique, func(i, j int) bool { return len(unique[i]) > len(unique[j]) })
	return &Redactor{values: unique}
}

// String returns s with every configured secret value replaced by Placeholder.
func (r *Redactor) String(s string) string {
	if r == nil {
		return s
	}
	for _, value := range r.values {
		s = strings.ReplaceAll(s, value, Placeholder)
	}
	return s
}
