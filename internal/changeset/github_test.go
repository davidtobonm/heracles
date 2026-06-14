package changeset_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/changeset"
)

func TestGitHubClientUsesDeterministicPullRequestContract(t *testing.T) {
	t.Parallel()

	runner := &fakeCommandRunner{outputs: []commandOutput{
		{output: "https://github.com/acme/backend/pull/42\n"},
		{},
		{},
		{},
	}}
	client := changeset.NewGitHubClient(runner)
	repository := changeset.Repository{Name: "backend", GitHub: "acme/backend", Head: "heracles/issue-7", Base: "main"}

	pullRequest, err := client.CreatePullRequest(context.Background(), changeset.PullRequestInput{
		Repository: repository,
		Title:      "Deliver issue 7",
		Body:       "Initial body",
	})
	if err != nil {
		t.Fatalf("CreatePullRequest() error = %v", err)
	}
	if pullRequest.Number != 42 || pullRequest.URL != "https://github.com/acme/backend/pull/42" || pullRequest.Repository != "backend" {
		t.Errorf("pull request = %#v, want decoded created PR", pullRequest)
	}
	if err := client.UpdatePullRequestBody(context.Background(), pullRequest, "Linked body"); err != nil {
		t.Fatalf("UpdatePullRequestBody() error = %v", err)
	}
	if err := client.WaitForCI(context.Background(), pullRequest); err != nil {
		t.Fatalf("WaitForCI() error = %v", err)
	}
	if err := client.Merge(context.Background(), pullRequest); err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	commands := strings.Join(runner.calls, "\n")
	for _, expected := range []string{
		"gh pr create --repo acme/backend --head heracles/issue-7 --base main --title Deliver issue 7 --body Initial body",
		"gh pr edit 42 --repo acme/backend --body Linked body",
		"gh pr checks 42 --repo acme/backend --required --watch",
		"gh pr merge 42 --repo acme/backend --merge",
	} {
		if !strings.Contains(commands, expected) {
			t.Errorf("commands %q do not contain %q", commands, expected)
		}
	}
}

func TestGitHubClientDecodesPullRequestStatus(t *testing.T) {
	t.Parallel()

	runner := &fakeCommandRunner{outputs: []commandOutput{
		{output: `{"mergedAt":null,"reviewDecision":"CHANGES_REQUESTED","statusCheckRollup":[{"name":"build","status":"completed","conclusion":"failure"},{"name":"lint","status":"completed","conclusion":"success"}]}`},
	}}
	client := changeset.NewGitHubClient(runner)
	pullRequest := changeset.PullRequest{Repository: "backend", URL: "https://github.com/acme/backend/pull/42", Number: 42}

	status, err := client.Status(context.Background(), pullRequest)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Merged {
		t.Errorf("status.Merged = true, want false for an unmerged pull request")
	}
	if !status.ChangesRequested {
		t.Errorf("status.ChangesRequested = false, want true")
	}
	if len(status.FailedChecks) != 1 || status.FailedChecks[0].Name != "build" || status.FailedChecks[0].Conclusion != "failure" {
		t.Errorf("status.FailedChecks = %#v, want only the failing build check", status.FailedChecks)
	}

	if !strings.Contains(strings.Join(runner.calls, "\n"), "gh pr view 42 --repo acme/backend --json mergedAt,reviewDecision,statusCheckRollup") {
		t.Errorf("commands = %#v, want gh pr view with status JSON fields", runner.calls)
	}
}

func TestGitHubClientReturnsActionableInvalidCreateOutput(t *testing.T) {
	t.Parallel()

	client := changeset.NewGitHubClient(&fakeCommandRunner{outputs: []commandOutput{{output: "not a pull request"}}})
	_, err := client.CreatePullRequest(context.Background(), changeset.PullRequestInput{
		Repository: changeset.Repository{Name: "backend", GitHub: "acme/backend"},
	})
	if err == nil || !strings.Contains(err.Error(), "decode created pull request URL") {
		t.Fatalf("CreatePullRequest() error = %v, want actionable decode failure", err)
	}
}

type commandOutput struct {
	output string
	err    error
}

type fakeCommandRunner struct {
	outputs []commandOutput
	calls   []string
}

func (runner *fakeCommandRunner) Run(_ context.Context, command string, args ...string) ([]byte, error) {
	runner.calls = append(runner.calls, command+" "+strings.Join(args, " "))
	if len(runner.outputs) == 0 {
		return nil, errors.New("unexpected command")
	}
	result := runner.outputs[0]
	runner.outputs = runner.outputs[1:]
	return []byte(result.output), result.err
}
