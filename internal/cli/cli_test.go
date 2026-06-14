package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestPlanCommandRecordsPRDIssueURLAndLocalPath(t *testing.T) {
	t.Parallel()

	surface := &fakeControl{}
	var stdout, stderr bytes.Buffer
	exit := cli.RunWithOptions([]string{
		"plan", "--id", "session-1",
		"--prd-issue", "https://github.com/acme/backlog/issues/9",
		"--prd", ".heracles/planning/session-1/PRD.md",
	}, &stdout, &stderr, cli.Options{Control: surface})
	if exit != 0 {
		t.Fatalf("plan exit = %d; stderr = %q", exit, stderr.String())
	}
	if len(surface.operations) != 1 {
		t.Fatalf("operations = %#v", surface.operations)
	}
	operation := surface.operations[0]
	if operation.Name != "plan" || operation.ID != "session-1" {
		t.Fatalf("operation = %#v", operation)
	}
	if operation.PRDIssueURL != "https://github.com/acme/backlog/issues/9" || operation.PRD != ".heracles/planning/session-1/PRD.md" {
		t.Errorf("operation = %#v, want recorded PRD Issue URL and local PRD path", operation)
	}
}

func TestPlanCommandRejectsPRDIssueWithoutLocalPRDPath(t *testing.T) {
	t.Parallel()

	surface := &fakeControl{}
	var stdout, stderr bytes.Buffer
	exit := cli.RunWithOptions([]string{"plan", "--id", "session-1", "--prd-issue", "https://github.com/acme/backlog/issues/9"}, &stdout, &stderr, cli.Options{Control: surface})
	if exit != 2 {
		t.Fatalf("plan exit = %d, want 2; stderr = %q", exit, stderr.String())
	}
	if len(surface.operations) != 0 {
		t.Errorf("operations = %#v, want no Control Surface call", surface.operations)
	}
}

func TestIssuesCommandAcceptsPRDIssueURL(t *testing.T) {
	t.Parallel()

	surface := &fakeControl{}
	var stdout, stderr bytes.Buffer
	exit := cli.RunWithOptions([]string{"issues", "https://github.com/acme/backlog/issues/9"}, &stdout, &stderr, cli.Options{Control: surface})
	if exit != 0 {
		t.Fatalf("issues exit = %d, want 0; stderr = %q", exit, stderr.String())
	}
	if len(surface.operations) != 1 {
		t.Fatalf("operations = %#v", surface.operations)
	}
	operation := surface.operations[0]
	if operation.Name != "issues" || operation.PRDIssueURL != "https://github.com/acme/backlog/issues/9" || operation.ID != "" || operation.PRD != "" {
		t.Errorf("operation = %#v, want only a PRD Issue URL", operation)
	}
}

func TestIssuesCommandRejectsPRDIssueURLWithOtherFlags(t *testing.T) {
	t.Parallel()

	surface := &fakeControl{}
	var stdout, stderr bytes.Buffer
	exit := cli.RunWithOptions([]string{"issues", "https://github.com/acme/backlog/issues/9", "--id", "prd-9"}, &stdout, &stderr, cli.Options{Control: surface})
	if exit != 2 {
		t.Fatalf("issues exit = %d, want 2; stderr = %q", exit, stderr.String())
	}
	if len(surface.operations) != 0 {
		t.Errorf("operations = %#v, want no Control Surface call", surface.operations)
	}
}

func TestIssuesCommandRequiresIDPRDAndPRDIssueWithoutPositional(t *testing.T) {
	t.Parallel()

	surface := &fakeControl{}
	var stdout, stderr bytes.Buffer
	exit := cli.RunWithOptions([]string{"issues", "--id", "prd-9"}, &stdout, &stderr, cli.Options{Control: surface})
	if exit != 2 {
		t.Fatalf("issues exit = %d, want 2; stderr = %q", exit, stderr.String())
	}
	if len(surface.operations) != 0 {
		t.Errorf("operations = %#v, want no Control Surface call", surface.operations)
	}
}

func TestIssuesCommandReadsApprovedPRDAndPRDIssueURL(t *testing.T) {
	t.Parallel()

	prdPath := filepath.Join(t.TempDir(), "PRD.md")
	if err := os.WriteFile(prdPath, []byte("# PRD\n\nBuild it."), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	surface := &fakeControl{}
	var stdout, stderr bytes.Buffer
	exit := cli.RunWithOptions([]string{
		"issues", "--id", "prd-9", "--prd", prdPath, "--prd-issue", "https://github.com/acme/backlog/issues/9",
	}, &stdout, &stderr, cli.Options{Control: surface})
	if exit != 0 {
		t.Fatalf("issues exit = %d, want 0; stderr = %q", exit, stderr.String())
	}
	if len(surface.operations) != 1 {
		t.Fatalf("operations = %#v", surface.operations)
	}
	operation := surface.operations[0]
	if operation.ID != "prd-9" || operation.PRD != "# PRD\n\nBuild it." || operation.PRDIssueURL != "https://github.com/acme/backlog/issues/9" {
		t.Errorf("operation = %#v, want ID, approved PRD contents, and PRD Issue URL", operation)
	}
}

func TestRunAcceptsOriginalAgentLoopFlagsAndLimit(t *testing.T) {
	t.Parallel()

	surface := &fakeControl{}
	var stdout, stderr bytes.Buffer
	exit := cli.RunWithOptions([]string{
		"run",
		"--implementer", "opencode",
		"--implementer-model", "opencode-go/kimi-k2.6",
		"--implementer-effort", "medium",
		"--reviewer", "codex",
		"--reviewer-model", "gpt-5.5",
		"--reviewer-effort", "high",
		"--limit", "40",
		"--yes",
	}, &stdout, &stderr, cli.Options{Control: surface})
	if exit != 0 {
		t.Fatalf("run exit = %d; stderr = %q", exit, stderr.String())
	}
	if len(surface.operations) != 1 || surface.operations[0].Limit != 40 {
		t.Errorf("operations = %#v, want run limit 40", surface.operations)
	}
}

func TestRunWithPRDURLProcessesOnlyThatBacklogWithoutConfirmation(t *testing.T) {
	t.Parallel()

	surface := &fakeControl{}
	var stdout, stderr bytes.Buffer
	exit := cli.RunWithOptions([]string{"run", "https://github.com/acme/backlog/issues/9"}, &stdout, &stderr, cli.Options{Control: surface})
	if exit != 0 {
		t.Fatalf("run exit = %d; stderr = %q", exit, stderr.String())
	}
	if len(surface.operations) != 1 || surface.operations[0].Name != "run" || surface.operations[0].PRDIssueURL != "https://github.com/acme/backlog/issues/9" {
		t.Errorf("operations = %#v, want scoped run without confirmation", surface.operations)
	}
}

func TestBareRunRequiresConfirmation(t *testing.T) {
	t.Parallel()

	surface := &fakeControl{}
	var stdout, stderr bytes.Buffer
	exit := cli.RunWithOptions([]string{"run"}, &stdout, &stderr, cli.Options{Control: surface, Input: strings.NewReader("y\n")})
	if exit != 0 {
		t.Fatalf("run exit = %d; stderr = %q", exit, stderr.String())
	}
	if len(surface.operations) != 2 || surface.operations[0].Kind != "ready" || surface.operations[1].Name != "run" {
		t.Errorf("operations = %#v, want confirmation lookup before run", surface.operations)
	}
	if !strings.Contains(stdout.String(), "Process all") {
		t.Errorf("stdout = %q, want confirmation prompt", stdout.String())
	}
}

func TestBareRunDeclinedConfirmationDoesNotRun(t *testing.T) {
	t.Parallel()

	surface := &fakeControl{}
	var stdout, stderr bytes.Buffer
	exit := cli.RunWithOptions([]string{"run"}, &stdout, &stderr, cli.Options{Control: surface, Input: strings.NewReader("n\n")})
	if exit != 0 {
		t.Fatalf("run exit = %d; stderr = %q", exit, stderr.String())
	}
	if len(surface.operations) != 1 || surface.operations[0].Kind != "ready" {
		t.Errorf("operations = %#v, want only the confirmation lookup", surface.operations)
	}
	if !strings.Contains(stdout.String(), "cancelled") {
		t.Errorf("stdout = %q, want cancellation message", stdout.String())
	}
}

func TestRunYesFlagSkipsConfirmation(t *testing.T) {
	t.Parallel()

	surface := &fakeControl{}
	var stdout, stderr bytes.Buffer
	exit := cli.RunWithOptions([]string{"run", "--yes"}, &stdout, &stderr, cli.Options{Control: surface})
	if exit != 0 {
		t.Fatalf("run exit = %d; stderr = %q", exit, stderr.String())
	}
	if len(surface.operations) != 1 || surface.operations[0].Name != "run" {
		t.Errorf("operations = %#v, want unprompted run", surface.operations)
	}
}

func TestLaborAcceptsPositionalProblemEquivalentToFlag(t *testing.T) {
	t.Parallel()

	surface := &fakeControl{}
	var stdout, stderr bytes.Buffer
	exit := cli.RunWithOptions([]string{"labor", "Build a thing"}, &stdout, &stderr, cli.Options{Control: surface})
	if exit != 0 {
		t.Fatalf("labor exit = %d; stderr = %q", exit, stderr.String())
	}
	if len(surface.operations) != 1 || surface.operations[0].Problem != "Build a thing" || surface.operations[0].ID == "" {
		t.Errorf("operations = %#v, want positional problem and generated ID", surface.operations)
	}
}

func TestLaborConflictingPositionalAndFlagProblemsFail(t *testing.T) {
	t.Parallel()

	surface := &fakeControl{}
	var stdout, stderr bytes.Buffer
	exit := cli.RunWithOptions([]string{"labor", "Build a thing", "--problem", "Build another thing"}, &stdout, &stderr, cli.Options{Control: surface})
	if exit != 2 {
		t.Fatalf("labor exit = %d, want 2; stderr = %q", exit, stderr.String())
	}
	if len(surface.operations) != 0 {
		t.Errorf("operations = %#v, want no Control Surface call", surface.operations)
	}
}

func TestLaborRequiresProblemOrID(t *testing.T) {
	t.Parallel()

	surface := &fakeControl{}
	var stdout, stderr bytes.Buffer
	exit := cli.RunWithOptions([]string{"labor"}, &stdout, &stderr, cli.Options{Control: surface})
	if exit != 2 {
		t.Fatalf("labor exit = %d, want 2; stderr = %q", exit, stderr.String())
	}
	if len(surface.operations) != 0 {
		t.Errorf("operations = %#v, want no Control Surface call", surface.operations)
	}
}

func TestLaborResumesByIDWithoutProblem(t *testing.T) {
	t.Parallel()

	surface := &fakeControl{}
	var stdout, stderr bytes.Buffer
	exit := cli.RunWithOptions([]string{"labor", "--id", "labor-1"}, &stdout, &stderr, cli.Options{Control: surface})
	if exit != 0 {
		t.Fatalf("labor exit = %d; stderr = %q", exit, stderr.String())
	}
	if len(surface.operations) != 1 || surface.operations[0].ID != "labor-1" || surface.operations[0].Problem != "" {
		t.Errorf("operations = %#v, want resume by ID without problem", surface.operations)
	}
}

func TestConfigSetWritesGlobalAndProjectPreferences(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "heracles.yaml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		args []string
		path string
	}{
		{
			args: []string{"config", "set", "--global", "--implementer", "opencode", "--implementer-model", "opencode-go/kimi-k2.6", "--implementer-effort", "medium"},
			path: filepath.Join(home, ".config", "heracles", "preferences.yaml"),
		},
		{
			args: []string{"config", "set", "--project", "--reviewer", "codex", "--reviewer-model", "gpt-5.5", "--reviewer-effort", "high"},
			path: filepath.Join(root, ".heracles", "preferences.yaml"),
		},
	} {
		var stdout, stderr bytes.Buffer
		exit := cli.RunWithOptions(testCase.args, &stdout, &stderr, cli.Options{WorkingDirectory: root, HomeDirectory: home})
		if exit != 0 {
			t.Fatalf("%v exit = %d; stderr = %q", testCase.args, exit, stderr.String())
		}
		contents, err := os.ReadFile(testCase.path)
		if err != nil {
			t.Fatalf("read %s: %v", testCase.path, err)
		}
		if !strings.Contains(string(contents), "provider:") || !strings.Contains(string(contents), "model:") {
			t.Errorf("preferences %q missing provider/model", contents)
		}
	}
}

func TestConfigSupportsAllAgentRolesAndDottedSyntax(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "heracles.yaml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := cli.Options{WorkingDirectory: root, HomeDirectory: home}

	var stdout, stderr bytes.Buffer
	exit := cli.RunWithOptions([]string{
		"config", "set", "--project",
		"--planner", "claude", "--planner-model", "opus", "--planner-effort", "high",
		"--issue_author", "opencode", "--issue_author-model", "openai/gpt-5.4", "--issue_author-variant", "low",
		"agents.implementer.provider=codex", "agents.implementer.model=gpt-5.1-codex", "agents.implementer.effort=high",
	}, &stdout, &stderr, opts)
	if exit != 0 {
		t.Fatalf("config set exit = %d; stderr = %q", exit, stderr.String())
	}

	contents, err := os.ReadFile(filepath.Join(root, ".heracles", "preferences.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"planner:", "provider: claude", "model: opus", "effort: high",
		"issue_author:", "provider: opencode", "model: openai/gpt-5.4", "variant: low",
		"implementer:", "provider: codex", "model: gpt-5.1-codex",
	} {
		if !strings.Contains(string(contents), want) {
			t.Errorf("preferences %q missing %q", contents, want)
		}
	}
}

func TestConfigShowPathAppendAndUnset(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "heracles.yaml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := cli.Options{WorkingDirectory: root, HomeDirectory: home}

	var stdout, stderr bytes.Buffer
	if exit := cli.RunWithOptions([]string{"config", "path", "--project"}, &stdout, &stderr, opts); exit != 0 {
		t.Fatalf("config path exit = %d; stderr = %q", exit, stderr.String())
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(resolvedRoot, ".heracles", "preferences.yaml")
	if got := strings.TrimSpace(stdout.String()); got != wantPath {
		t.Errorf("config path = %q, want %q", got, wantPath)
	}

	stdout.Reset()
	if exit := cli.RunWithOptions([]string{"config", "set", "--project", "--implementer", "codex", "agents.implementer.extra_args=--foo,--bar"}, &stdout, &stderr, opts); exit != 0 {
		t.Fatalf("config set exit = %d; stderr = %q", exit, stderr.String())
	}

	stdout.Reset()
	if exit := cli.RunWithOptions([]string{"config", "show", "--project", "agents.implementer.provider"}, &stdout, &stderr, opts); exit != 0 {
		t.Fatalf("config show exit = %d; stderr = %q", exit, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "codex" {
		t.Errorf("config show agents.implementer.provider = %q, want codex", got)
	}

	stdout.Reset()
	if exit := cli.RunWithOptions([]string{"config", "append", "--project", "agents.implementer.extra_args=--baz"}, &stdout, &stderr, opts); exit != 0 {
		t.Fatalf("config append exit = %d; stderr = %q", exit, stderr.String())
	}
	stdout.Reset()
	if exit := cli.RunWithOptions([]string{"config", "show", "--project", "agents.implementer.extra_args"}, &stdout, &stderr, opts); exit != 0 {
		t.Fatalf("config show exit = %d; stderr = %q", exit, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "--foo,--bar,--baz" {
		t.Errorf("config show agents.implementer.extra_args = %q, want --foo,--bar,--baz", got)
	}

	stdout.Reset()
	confirmed := cli.Options{WorkingDirectory: root, HomeDirectory: home, Input: strings.NewReader("y\n")}
	if exit := cli.RunWithOptions([]string{"config", "unset", "--project", "agents.implementer.provider"}, &stdout, &stderr, confirmed); exit != 0 {
		t.Fatalf("config unset exit = %d; stderr = %q", exit, stderr.String())
	}
	stdout.Reset()
	if exit := cli.RunWithOptions([]string{"config", "show", "--project", "agents.implementer.provider"}, &stdout, &stderr, opts); exit != 0 {
		t.Fatalf("config show exit = %d; stderr = %q", exit, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "" {
		t.Errorf("config show agents.implementer.provider after unset = %q, want empty", got)
	}
}

func TestConfigUnsetDeclinedConfirmationMakesNoChanges(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "heracles.yaml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := cli.Options{WorkingDirectory: root, HomeDirectory: home}

	var stdout, stderr bytes.Buffer
	if exit := cli.RunWithOptions([]string{"config", "set", "--project", "--implementer", "codex"}, &stdout, &stderr, opts); exit != 0 {
		t.Fatalf("config set exit = %d; stderr = %q", exit, stderr.String())
	}

	stdout.Reset()
	declined := cli.Options{WorkingDirectory: root, HomeDirectory: home, Input: strings.NewReader("n\n")}
	if exit := cli.RunWithOptions([]string{"config", "unset", "--project", "agents.implementer.provider"}, &stdout, &stderr, declined); exit != 0 {
		t.Fatalf("config unset exit = %d; stderr = %q", exit, stderr.String())
	}

	stdout.Reset()
	if exit := cli.RunWithOptions([]string{"config", "show", "--project", "agents.implementer.provider"}, &stdout, &stderr, opts); exit != 0 {
		t.Fatalf("config show exit = %d; stderr = %q", exit, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "codex" {
		t.Errorf("config show agents.implementer.provider = %q, want codex (unset declined)", got)
	}
}

func TestConfigSetRejectsUnsupportedProviderSettingsAndUnknownProviders(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "heracles.yaml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := cli.Options{WorkingDirectory: root, HomeDirectory: home}

	var stdout, stderr bytes.Buffer
	exit := cli.RunWithOptions([]string{"config", "set", "--project", "--implementer", "claude", "--implementer-variant", "fast"}, &stdout, &stderr, opts)
	if exit != 2 {
		t.Fatalf("exit = %d, want 2; stderr = %q", exit, stderr.String())
	}
	if !strings.Contains(stderr.String(), "variant") {
		t.Errorf("stderr = %q, want variant validation error", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	exit = cli.RunWithOptions([]string{"config", "set", "--project", "--implementer", "not-a-provider"}, &stdout, &stderr, opts)
	if exit != 2 {
		t.Fatalf("exit = %d, want 2; stderr = %q", exit, stderr.String())
	}
	if !strings.Contains(stderr.String(), "unsupported provider") {
		t.Errorf("stderr = %q, want unsupported provider error", stderr.String())
	}
}

func TestConfigSetRejectsConflictingDashedAndDottedValues(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "heracles.yaml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := cli.Options{WorkingDirectory: root, HomeDirectory: home}

	var stdout, stderr bytes.Buffer
	exit := cli.RunWithOptions([]string{"config", "set", "--project", "--implementer", "codex", "agents.implementer.provider=opencode"}, &stdout, &stderr, opts)
	if exit != 2 {
		t.Fatalf("exit = %d, want 2; stderr = %q", exit, stderr.String())
	}
	if !strings.Contains(stderr.String(), "conflicting") {
		t.Errorf("stderr = %q, want conflicting values error", stderr.String())
	}
}

func TestRunAcceptsAllRoleLaunchOverridesAndDottedSyntax(t *testing.T) {
	t.Parallel()

	surface := &fakeControl{}
	var stdout, stderr bytes.Buffer
	exit := cli.RunWithOptions([]string{
		"run",
		"--planner", "claude", "--planner-model", "opus", "--planner-effort", "high",
		"--issue_author", "opencode", "agents.issue_author.model=openai/gpt-5.4",
		"--implementer", "codex",
		"--reviewer", "codex",
		"--limit", "5",
		"--yes",
	}, &stdout, &stderr, cli.Options{Control: surface})
	if exit != 0 {
		t.Fatalf("run exit = %d; stderr = %q", exit, stderr.String())
	}
	if len(surface.operations) != 1 || surface.operations[0].Limit != 5 {
		t.Errorf("operations = %#v, want run limit 5", surface.operations)
	}
}

func TestRunRejectsUnsupportedAndConflictingLaunchOverrides(t *testing.T) {
	t.Parallel()

	surface := &fakeControl{}
	var stdout, stderr bytes.Buffer
	exit := cli.RunWithOptions([]string{"run", "--planner", "claude", "--planner-variant", "fast"}, &stdout, &stderr, cli.Options{Control: surface})
	if exit != 2 {
		t.Fatalf("exit = %d, want 2; stderr = %q", exit, stderr.String())
	}
	if len(surface.operations) != 0 {
		t.Errorf("operations = %#v, want none executed", surface.operations)
	}

	stdout.Reset()
	stderr.Reset()
	exit = cli.RunWithOptions([]string{"run", "--implementer", "codex", "agents.implementer.provider=opencode"}, &stdout, &stderr, cli.Options{Control: surface})
	if exit != 2 {
		t.Fatalf("exit = %d, want 2; stderr = %q", exit, stderr.String())
	}
	if !strings.Contains(stderr.String(), "conflicting") {
		t.Errorf("stderr = %q, want conflicting values error", stderr.String())
	}
}

func TestCancelRequiresConfirmation(t *testing.T) {
	t.Parallel()

	surface := &fakeControl{}
	var stdout, stderr bytes.Buffer
	exit := cli.RunWithOptions([]string{"cancel", "labor-1"}, &stdout, &stderr, cli.Options{Control: surface, Input: strings.NewReader("n\n")})
	if exit != 0 {
		t.Fatalf("cancel exit = %d; stderr = %q", exit, stderr.String())
	}
	if len(surface.operations) != 0 {
		t.Errorf("operations = %#v, want no operation executed without confirmation", surface.operations)
	}
	if !strings.Contains(stdout.String(), "cancelled") {
		t.Errorf("stdout = %q, want cancellation message", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	exit = cli.RunWithOptions([]string{"cancel", "labor-1"}, &stdout, &stderr, cli.Options{Control: surface, Input: strings.NewReader("y\n")})
	if exit != 0 {
		t.Fatalf("cancel exit = %d; stderr = %q", exit, stderr.String())
	}
	if len(surface.operations) != 1 || surface.operations[0].Name != "cancel" || surface.operations[0].ID != "labor-1" {
		t.Errorf("operations = %#v, want cancel executed after confirmation", surface.operations)
	}
	if !strings.Contains(stdout.String(), "This cannot be undone locally") {
		t.Errorf("stdout = %q, want irreversibility warning", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	exit = cli.RunWithOptions([]string{"cancel", "labor-2", "--yes"}, &stdout, &stderr, cli.Options{Control: surface})
	if exit != 0 {
		t.Fatalf("cancel exit = %d; stderr = %q", exit, stderr.String())
	}
	if len(surface.operations) != 2 || surface.operations[1].Name != "cancel" || surface.operations[1].ID != "labor-2" {
		t.Errorf("operations = %#v, want --yes to skip confirmation", surface.operations)
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
		{args: []string{"cancel", "labor-1", "--reason", "stop", "--yes"}, name: "cancel", id: "labor-1"},
		{args: []string{"status"}, name: "status"},
		{args: []string{"status", "labor-1"}, name: "status", id: "labor-1"},
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
	exitCode := cli.RunWithOptions([]string{"init", "--tracker", "example/widget", "--repo", repositoryPath}, &stdout, &stderr, cli.Options{
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

func TestInitWithoutFlagsRunsInteractiveSetup(t *testing.T) {
	t.Parallel()

	repositoryPath := filepath.Join(t.TempDir(), "widget")
	runCommand(t, "", "git", "init", "--initial-branch=main", repositoryPath)
	runCommand(t, repositoryPath, "git", "remote", "add", "origin", "git@github.com:example/widget.git")
	if err := os.WriteFile(filepath.Join(repositoryPath, "go.mod"), []byte("module example.com/widget\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	input := strings.NewReader(strings.Repeat("\n", 6))
	exitCode := cli.RunWithOptions([]string{"init"}, &stdout, &stderr, cli.Options{
		WorkingDirectory: repositoryPath,
		HomeDirectory:    t.TempDir(),
		Input:            input,
		DoctorSystem:     fakeInitDoctorSystem{},
	})

	if exitCode != 0 {
		t.Fatalf("RunWithOptions(init) exit code = %d, want 0; stdout = %q, stderr = %q", exitCode, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "Wrote Project Configuration to") {
		t.Errorf("stdout = %q, want write confirmation", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(repositoryPath, "heracles.yaml")); err != nil {
		t.Errorf("Project Configuration was not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repositoryPath, ".heracles", "preferences.yaml")); err != nil {
		t.Errorf("Agent Role preferences were not created: %v", err)
	}
}

type fakeInitDoctorSystem struct{}

func (fakeInitDoctorSystem) LookPath(executable string) (string, error) {
	switch executable {
	case "git", "gh", "codex", "gofmt", "go":
		return "/usr/bin/" + executable, nil
	}
	return "", os.ErrNotExist
}

func (fakeInitDoctorSystem) Run(context.Context, string, ...string) error {
	return nil
}

func (fakeInitDoctorSystem) Output(context.Context, string, ...string) (string, error) {
	return "", nil
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

func TestDoctorJSONOutputIsStableAndNonInteractive(t *testing.T) {
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

	var stdout, stderr bytes.Buffer
	exitCode := cli.RunWithOptions([]string{"doctor", "--json"}, &stdout, &stderr, cli.Options{
		WorkingDirectory: root,
		DoctorSystem:     cliFakeSystem{},
	})
	if exitCode != 0 {
		t.Fatalf("RunWithOptions(doctor --json) exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}

	var report struct {
		OK     bool `json:"OK"`
		Checks []struct {
			Name    string `json:"Name"`
			OK      bool   `json:"OK"`
			Warning bool   `json:"Warning"`
			Message string `json:"Message"`
		} `json:"Checks"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("doctor --json output is not valid JSON: %v; output = %q", err, stdout.String())
	}
	if !report.OK || len(report.Checks) == 0 {
		t.Errorf("report = %#v, want OK with checks", report)
	}
	for _, forbidden := range []string{"Continue", "?", "y/n", "update available"} {
		if strings.Contains(stdout.String(), forbidden) {
			t.Errorf("doctor --json output %q contains interactive or update content %q", stdout.String(), forbidden)
		}
	}
}

func TestDoctorFixCreatesMissingWorkspaceRoot(t *testing.T) {
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

	var before bytes.Buffer
	var beforeErr bytes.Buffer
	exitCode := cli.RunWithOptions([]string{"doctor"}, &before, &beforeErr, cli.Options{
		WorkingDirectory: root,
		DoctorSystem:     cliFakeSystem{},
	})
	if exitCode != 0 {
		t.Fatalf("RunWithOptions(doctor) exit code = %d, want 0; stderr = %q", exitCode, beforeErr.String())
	}
	if !strings.Contains(before.String(), "[warn] Workspaces") {
		t.Fatalf("doctor output %q does not contain workspace warning", before.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".heracles", "workspaces")); !os.IsNotExist(err) {
		t.Fatalf("workspace root exists before --fix: %v", err)
	}

	var after bytes.Buffer
	var afterErr bytes.Buffer
	exitCode = cli.RunWithOptions([]string{"doctor", "--fix"}, &after, &afterErr, cli.Options{
		WorkingDirectory: root,
		DoctorSystem:     cliFakeSystem{},
	})
	if exitCode != 0 {
		t.Fatalf("RunWithOptions(doctor --fix) exit code = %d, want 0; stderr = %q", exitCode, afterErr.String())
	}
	if !strings.Contains(after.String(), "[ok] Workspaces") {
		t.Errorf("doctor --fix output %q does not contain repaired workspace diagnostic", after.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".heracles", "workspaces")); err != nil {
		t.Errorf("doctor --fix did not create the workspace root: %v", err)
	}
}

func TestLaborAndRunAreBlockedByFailingDoctorPreflight(t *testing.T) {
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

	system := blockingCLIDoctorSystem{}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := cli.RunWithOptions([]string{"labor", "Build it"}, &stdout, &stderr, cli.Options{
		WorkingDirectory: root,
		DoctorSystem:     system,
	})
	if exitCode != 1 {
		t.Fatalf("RunWithOptions(labor) exit code = %d, want 1; stdout = %q", exitCode, stdout.String())
	}
	if !strings.Contains(stderr.String(), "Doctor preflight failed") {
		t.Errorf("stderr = %q, want Doctor preflight failure", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = cli.RunWithOptions([]string{"run", "--yes"}, &stdout, &stderr, cli.Options{
		WorkingDirectory: root,
		DoctorSystem:     system,
	})
	if exitCode != 1 {
		t.Fatalf("RunWithOptions(run --yes) exit code = %d, want 1; stdout = %q", exitCode, stdout.String())
	}
	if !strings.Contains(stderr.String(), "Doctor preflight failed") {
		t.Errorf("stderr = %q, want Doctor preflight failure", stderr.String())
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

func (cliFakeSystem) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

// blockingCLIDoctorSystem reports git as missing, a Doctor blocker per ADR
// 0027 that must stop heracles labor/run before they execute.
type blockingCLIDoctorSystem struct{}

func (blockingCLIDoctorSystem) LookPath(executable string) (string, error) {
	if executable == "git" {
		return "", os.ErrNotExist
	}
	return "/fake/" + executable, nil
}

func (blockingCLIDoctorSystem) Run(context.Context, string, ...string) error {
	return nil
}

func (blockingCLIDoctorSystem) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func runCommand(t testing.TB, workingDirectory, command string, args ...string) {
	t.Helper()

	process := exec.Command(command, args...)
	process.Dir = workingDirectory
	if output, err := process.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", command, args, err, output)
	}
}
