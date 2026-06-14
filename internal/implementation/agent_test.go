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
	implemented, err := (implementation.AgentImplementer{Runner: implementRunner, Profile: agent.Profile{Provider: "claude"}}).Implement(context.Background(), implementation.ImplementContext{Workspace: issueWorkspace})
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}
	if implemented.Changes != "done" || strings.Join(implementRunner.workspaces, ",") != "/backend,/frontend" {
		t.Errorf("implementation/runner = %#v / %#v", implemented, implementRunner)
	}
	if !strings.Contains(implementRunner.prompt, "exactly one JSON object") {
		t.Errorf("implementer prompt does not require one JSON object: %s", implementRunner.prompt)
	}
	if !strings.Contains(strings.Join(implementRunner.profile.ExtraArgs, " "), "--json-schema") || !strings.Contains(strings.Join(implementRunner.profile.ExtraArgs, " "), "--output-format json") {
		t.Errorf("implementer profile extra args = %#v, want claude structured output flags", implementRunner.profile.ExtraArgs)
	}

	reviewRunner := &roleRunner{result: agent.Result{Final: `{"status":"completed","summary":"good","corrective_changes":false}`}}
	outcome, err := (implementation.AgentReviewer{Runner: reviewRunner, Profile: agent.Profile{Provider: "claude"}}).Review(context.Background(), issueWorkspace, delivery.ReviewContext{Issue: "issue"})
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if outcome.Status != "completed" || !strings.Contains(reviewRunner.prompt, "YAGNI") || !strings.Contains(reviewRunner.prompt, "exactly one JSON object") || strings.Join(reviewRunner.workspaces, ",") != "/backend,/frontend" {
		t.Errorf("review/runner = %#v / %#v", outcome, reviewRunner)
	}
}

func TestConfiguredImplementerAcceptsFencedJSON(t *testing.T) {
	t.Parallel()

	issueWorkspace := workspace.Workspace{Repositories: []workspace.Worktree{{Path: "/backend"}}}
	runner := &roleRunner{result: agent.Result{Final: "```json\n{\"changes\":\"done\",\"evidence_policy\":{\"Exempt\":true,\"Reason\":\"docs only\"}}\n```"}}
	implemented, err := (implementation.AgentImplementer{Runner: runner, Profile: agent.Profile{Provider: "claude"}}).Implement(context.Background(), implementation.ImplementContext{Workspace: issueWorkspace})
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}
	if implemented.Changes != "done" {
		t.Errorf("implementation = %#v, want fenced JSON parsed", implemented)
	}
	if !strings.Contains(runner.prompt, "exactly one JSON object") {
		t.Errorf("implementer prompt does not require one JSON object: %s", runner.prompt)
	}
	if !strings.Contains(strings.Join(runner.profile.ExtraArgs, " "), "--json-schema") {
		t.Errorf("implementer profile extra args = %#v, want claude schema flags", runner.profile.ExtraArgs)
	}
}

type roleRunner struct {
	result     agent.Result
	profile    agent.Profile
	workspaces []string
	prompt     string
}

func (runner *roleRunner) Run(_ context.Context, _ string, profile agent.Profile, workspaces []string, prompt string) (agent.Result, error) {
	runner.profile = profile
	runner.workspaces = append([]string(nil), workspaces...)
	runner.prompt = prompt
	return runner.result, nil
}
