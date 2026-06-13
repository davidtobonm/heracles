package cli_test

import (
	"bytes"
	"context"
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

func TestDoctorDiscoversConfigurationAndReportsChecks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	config := `version: 1
issue_tracker:
  github: example/widget
repositories:
  - name: widget
    path: .
    github: example/widget
    base_branch: main
agents:
  default_profile: default
  profiles:
    default:
      provider: codex
  roles: {}
`
	if err := os.WriteFile(filepath.Join(root, "heracles.yaml"), []byte(config), 0o644); err != nil {
		t.Fatalf("write Project Configuration: %v", err)
	}
	nested := filepath.Join(root, "docs")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatalf("create nested directory: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.RunWithOptions([]string{"doctor"}, &stdout, &stderr, cli.Options{
		WorkingDirectory: nested,
		DoctorSystem:     cliFakeSystem{},
	})

	if exitCode != 0 {
		t.Fatalf("RunWithOptions(doctor) exit code = %d, want 0; stderr = %q; stdout = %q", exitCode, stderr.String(), stdout.String())
	}
	for _, expected := range []string{"[ok] GitHub authentication", "[ok] Agent Profiles", "[ok] Target Repository widget"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("doctor output %q does not contain %q", stdout.String(), expected)
		}
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

type cliFakeSystem struct{}

func (cliFakeSystem) LookPath(executable string) (string, error) {
	return "/fake/" + executable, nil
}

func (cliFakeSystem) Run(context.Context, string, ...string) error {
	return nil
}

func runCommand(t testing.TB, workingDirectory, command string, args ...string) {
	t.Helper()

	process := exec.Command(command, args...)
	process.Dir = workingDirectory
	if output, err := process.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", command, args, err, output)
	}
}
