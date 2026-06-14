// Package cancel provides the confirmation messaging for `heracles cancel`,
// per ADR 0030: cancellation is irreversible locally and leaves GitHub work
// unchanged.
package cancel

import "fmt"

// Prompt is the confirmation question shown before cancelling Labor id.
func Prompt(id string) string {
	return fmt.Sprintf("Cancel Labor %q? This cannot be undone locally. Issues, Pull Requests, and Change Sets on GitHub are left unchanged.", id)
}
