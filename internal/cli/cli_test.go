package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/cli"
)

func TestHelpDescribesHeracles(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := cli.Run([]string{"--help"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("Run(--help) exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}

	for _, expected := range []string{"Heracles", "Usage:", "heracles version"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("help output %q does not contain %q", stdout.String(), expected)
		}
	}

	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestVersionReportsBuildMetadata(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := cli.Run([]string{"version"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("Run(version) exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}

	for _, expected := range []string{"heracles", "version=", "commit=", "built="} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("version output %q does not contain %q", stdout.String(), expected)
		}
	}

	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}
