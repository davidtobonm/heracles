// Package output provides the shared stable JSON encoding used by Doctor,
// status, and Labor execution's `--json` output, per ADR 0031: structured
// output is indented, newline-terminated, and free of interactive prompts
// or update notices.
package output

import (
	"encoding/json"
	"io"
)

// Encode writes value to w as indented JSON terminated by a newline.
func Encode(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
