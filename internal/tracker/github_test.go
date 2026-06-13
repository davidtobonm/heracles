package tracker_test

import (
	"context"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/tracker"
)

func TestGitHubClientUsesDeterministicCLIContract(t *testing.T) {
	t.Parallel()

	runner := &fakeCommandRunner{outputs: []string{
		`[{"number":7,"title":"Ready","body":"","url":"https://github.com/acme/backlog/issues/7","createdAt":"2026-01-01T00:00:00Z","state":"OPEN","labels":[{"name":"heracles:ready"},{"name":"enhancement"}]}]`,
		`{"number":7,"title":"Ready","body":"","url":"https://github.com/acme/backlog/issues/7","createdAt":"2026-01-01T00:00:00Z","state":"OPEN","labels":[{"name":"heracles:ready"},{"name":"enhancement"}]}`,
		"",
		"",
	}}
	client := tracker.NewGitHubClient(runner)

	issues, err := client.ListOpenIssues(context.Background(), "acme/backlog")
	if err != nil {
		t.Fatalf("ListOpenIssues() error = %v", err)
	}
	if len(issues) != 1 || issues[0].Number != 7 || issues[0].Labels[0] != "heracles:ready" {
		t.Errorf("issues = %#v, want decoded GitHub fixture", issues)
	}

	reference := tracker.Reference{Owner: "acme", Repo: "backlog", Number: 7}
	if err := client.SetLabels(context.Background(), reference, []string{"enhancement", tracker.LabelInProgress}); err != nil {
		t.Fatalf("SetLabels() error = %v", err)
	}
	if err := client.Comment(context.Background(), reference, "Claimed"); err != nil {
		t.Fatalf("Comment() error = %v", err)
	}

	commands := strings.Join(runner.calls, "\n")
	for _, expected := range []string{"gh issue list --repo acme/backlog", "gh issue view 7 --repo acme/backlog", "--remove-label heracles:ready", "--add-label heracles:in-progress", "gh issue comment 7 --repo acme/backlog --body Claimed"} {
		if !strings.Contains(commands, expected) {
			t.Errorf("commands %q do not contain %q", commands, expected)
		}
	}
}

type fakeCommandRunner struct {
	outputs []string
	calls   []string
}

func (runner *fakeCommandRunner) Run(_ context.Context, command string, args ...string) ([]byte, error) {
	runner.calls = append(runner.calls, command+" "+strings.Join(args, " "))
	output := runner.outputs[0]
	runner.outputs = runner.outputs[1:]
	return []byte(output), nil
}
