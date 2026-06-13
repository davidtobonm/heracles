package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
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

func TestInitCreatesDetectedProjectConfiguration(t *testing.T) {
	t.Parallel()

	repositoryPath := filepath.Join(t.TempDir(), "widget")
	runCommand(t, "", "git", "init", "--initial-branch=main", repositoryPath)
	runCommand(t, repositoryPath, "git", "remote", "add", "origin", "git@github.com:example/widget.git")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.RunWithOptions([]string{"init"}, &stdout, &stderr, cli.Options{
		WorkingDirectory: repositoryPath,
	})

	if exitCode != 0 {
		t.Fatalf("RunWithOptions(init) exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Initialized Project Configuration") {
		t.Errorf("stdout = %q, want initialization confirmation", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(repositoryPath, "heracles.yaml")); err != nil {
		t.Errorf("Project Configuration was not created: %v", err)
	}
}

func TestInitHelpExitsSuccessfully(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := cli.Run([]string{"init", "--help"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("Run(init --help) exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "-tracker") {
		t.Errorf("help output = %q, want init flags", stderr.String())
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

func runCommand(t testing.TB, workingDirectory, command string, args ...string) {
	t.Helper()

	process := exec.Command(command, args...)
	process.Dir = workingDirectory
	if output, err := process.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", command, args, err, output)
	}
}
