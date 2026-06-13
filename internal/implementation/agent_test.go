package implementation_test

import (
	"context"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/delivery"
	"github.com/davidtobonm/heracles/internal/implementation"
	"github.com/davidtobonm/heracles/internal/workspace"
)

func TestConfiguredImplementerAndReviewerUseEveryIssueWorkspace(t *testing.T) {
	t.Parallel()

	issueWorkspace := workspace.Workspace{Repositories: []workspace.Worktree{{Path: "/backend"}, {Path: "/frontend"}}}
	implementRunner := &roleRunner{result: agent.Result{Final: `{"changes":"done","evidence_policy":{"Exempt":true,"Reason":"docs only"}}`}}
	implemented, err := (implementation.AgentImplementer{Runner: implementRunner, Profile: agent.Profile{Provider: "codex"}}).Implement(context.Background(), implementation.ImplementContext{Workspace: issueWorkspace})
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}
	if implemented.Changes != "done" || strings.Join(implementRunner.workspaces, ",") != "/backend,/frontend" {
		t.Errorf("implementation/runner = %#v / %#v", implemented, implementRunner)
	}

	reviewRunner := &roleRunner{result: agent.Result{Final: `{"status":"completed","summary":"good","corrective_changes":false}`}}
	outcome, err := (implementation.AgentReviewer{Runner: reviewRunner, Profile: agent.Profile{Provider: "claude"}}).Review(context.Background(), issueWorkspace, delivery.ReviewContext{Issue: "issue"})
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if outcome.Status != "completed" || !strings.Contains(reviewRunner.prompt, "YAGNI") || strings.Join(reviewRunner.workspaces, ",") != "/backend,/frontend" {
		t.Errorf("review/runner = %#v / %#v", outcome, reviewRunner)
	}
}

type roleRunner struct {
	result     agent.Result
	workspaces []string
	prompt     string
}

func (runner *roleRunner) Run(_ context.Context, _ string, _ agent.Profile, workspaces []string, prompt string) (agent.Result, error) {
	runner.workspaces = append([]string(nil), workspaces...)
	runner.prompt = prompt
	return runner.result, nil
}
