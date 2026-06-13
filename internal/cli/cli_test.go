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
	"github.com/davidtobonm/heracles/internal/control"
)

func TestHelpDescribesHeracles(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := cli.Run([]string{"--help"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("Run(--help) exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}

	for _, expected := range []string{"Heracles", "Usage:", "heracles plan", "heracles labor", "heracles inspect", "heracles mcp serve", "heracles version"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("help output %q does not contain %q", stdout.String(), expected)
		}
	}

	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", stderr.String())
	}
}

func TestMCPServeUsesSameInjectedControlSurface(t *testing.T) {
	t.Parallel()

	surface := &fakeControl{}
	input := strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{}}\n{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"heracles_resume\",\"arguments\":{\"id\":\"labor-1\"}}}\n")
	var stdout, stderr bytes.Buffer
	exit := cli.RunWithOptions([]string{"mcp", "serve"}, &stdout, &stderr, cli.Options{Control: surface, Input: input})
	if exit != 0 {
		t.Fatalf("mcp serve exit = %d; stderr = %q", exit, stderr.String())
	}
	if len(surface.operations) != 1 || surface.operations[0].Name != "resume" {
		t.Errorf("MCP operations = %#v", surface.operations)
	}
	if !strings.Contains(stdout.String(), `"structuredContent"`) {
		t.Errorf("MCP output = %q", stdout.String())
	}
}

func TestControlCommandsUseSharedSurfaceAndStableJSON(t *testing.T) {
	t.Parallel()

	surface := &fakeControl{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.RunWithOptions([]string{"labor", "--id", "labor-1", "--problem", "Build it", "--json"}, &stdout, &stderr, cli.Options{Control: surface})
	if exitCode != 0 {
		t.Fatalf("labor exit = %d; stderr = %q", exitCode, stderr.String())
	}
	if len(surface.operations) != 1 || surface.operations[0].Name != "labor" || surface.operations[0].ID != "labor-1" {
		t.Errorf("operations = %#v", surface.operations)
	}
	for _, expected := range []string{`"operation": "labor"`, `"id": "labor-1"`, `"status": "awaiting_planning_approval"`} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("JSON output %q does not contain %q", stdout.String(), expected)
		}
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = cli.RunWithOptions([]string{"approve", "planning", "labor-1", "--reason", "approved"}, &stdout, &stderr, cli.Options{Control: surface})
	if exitCode != 0 || surface.operations[1].Decision != "approve" || surface.operations[1].Kind != "planning" {
		t.Errorf("approve exit/operation = %d / %#v", exitCode, surface.operations)
	}
}

func TestListInspectAndOperationsValidateArguments(t *testing.T) {
	t.Parallel()

	surface := &fakeControl{}
	for _, testCase := range []struct {
		args []string
		name string
		kind string
		id   string
	}{
		{args: []string{"list", "labors"}, name: "list", kind: "labors"},
		{args: []string{"inspect", "labor", "labor-1"}, name: "inspect", kind: "labor", id: "labor-1"},
		{args: []string{"retry", "attempt-1"}, name: "retry", id: "attempt-1"},
		{args: []string{"resume", "labor-1"}, name: "resume", id: "labor-1"},
		{args: []string{"cancel", "labor-1", "--reason", "stop"}, name: "cancel", id: "labor-1"},
	} {
		var stdout, stderr bytes.Buffer
		if exit := cli.RunWithOptions(testCase.args, &stdout, &stderr, cli.Options{Control: surface}); exit != 0 {
			t.Fatalf("%v exit = %d; stderr = %q", testCase.args, exit, stderr.String())
		}
		operation := surface.operations[len(surface.operations)-1]
		if operation.Name != testCase.name || operation.Kind != testCase.kind || operation.ID != testCase.id {
			t.Errorf("%v operation = %#v", testCase.args, operation)
		}
	}

	var stdout, stderr bytes.Buffer
	if exit := cli.RunWithOptions([]string{"inspect", "labor"}, &stdout, &stderr, cli.Options{Control: surface}); exit != 2 {
		t.Errorf("invalid inspect exit = %d, want 2", exit)
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

type fakeControl struct {
	operations []control.Operation
}

func (surface *fakeControl) Execute(_ context.Context, operation control.Operation) (control.Result, error) {
	surface.operations = append(surface.operations, operation)
	return control.Result{Operation: operation.Name, Kind: operation.Kind, ID: operation.ID, Status: "awaiting_planning_approval", Data: map[string]string{"id": operation.ID}}, nil
}

func (surface *fakeControl) Close() error { return nil }

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
