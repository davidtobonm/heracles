package output_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/output"
)

func TestEncodeWritesIndentedJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := output.Encode(&buf, map[string]string{"status": "ok"}); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if !strings.Contains(buf.String(), "{\n  \"status\": \"ok\"\n}") {
		t.Errorf("Encode() output = %q, want indented JSON", buf.String())
	}
}
