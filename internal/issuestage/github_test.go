package issuestage_test

import (
	"context"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/issuestage"
)

func TestGitHubPublisherUsesDeterministicCreateContract(t *testing.T) {
	t.Parallel()

	runner := &issueCommandRunner{output: "https://github.com/acme/backlog/issues/7\n"}
	publisher := issuestage.NewGitHubPublisher(runner)
	url, err := publisher.CreateIssue(context.Background(), issuestage.PublishInput{
		Repository: "acme/backlog",
		Title:      "Deliver slice",
		Body:       "Body",
		Labels:     []string{"heracles:ready", "enhancement"},
	})
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}
	if url != "https://github.com/acme/backlog/issues/7" {
		t.Errorf("url = %q", url)
	}
	for _, expected := range []string{"gh issue create --repo acme/backlog --title Deliver slice --body Body", "--label heracles:ready --label enhancement"} {
		if !strings.Contains(runner.call, expected) {
			t.Errorf("command %q does not contain %q", runner.call, expected)
		}
	}
}

type issueCommandRunner struct {
	output string
	call   string
}

func (runner *issueCommandRunner) Run(_ context.Context, command string, args ...string) ([]byte, error) {
	runner.call = command + " " + strings.Join(args, " ")
	return []byte(runner.output), nil
}
