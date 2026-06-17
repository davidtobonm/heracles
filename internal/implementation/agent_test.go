package implementation_test

import (
	"context"
	"encoding/json"
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

func TestConfiguredImplementerPromptIncludesEvidenceTimestamps(t *testing.T) {
	t.Parallel()

	issueWorkspace := workspace.Workspace{Repositories: []workspace.Worktree{{Path: "/backend"}}}
	runner := &roleRunner{result: agent.Result{Final: `{"changes":"done","evidence_policy":{"Exempt":true,"Reason":"docs only"}}`}}
	if _, err := (implementation.AgentImplementer{Runner: runner, Profile: agent.Profile{Provider: "codex"}}).Implement(context.Background(), implementation.ImplementContext{Workspace: issueWorkspace}); err != nil {
		t.Fatalf("Implement() error = %v", err)
	}
	if !strings.Contains(runner.prompt, "started_at") || !strings.Contains(runner.prompt, "finished_at") {
		t.Errorf("implementer prompt omits evidence timestamp fields: %s", runner.prompt)
	}
}

// TestNonClaudeProviderExampleEvidenceSatisfiesValidateEvidence proves that a
// non-Claude provider (which has no JSON-Schema enforcement and only ever
// sees the prompt's free-text example) cannot be misled into omitting
// started_at/finished_at: copying the example evidence verbatim must pass
// delivery.ValidateEvidence's timing gate.
func TestNonClaudeProviderExampleEvidenceSatisfiesValidateEvidence(t *testing.T) {
	t.Parallel()

	issueWorkspace := workspace.Workspace{Repositories: []workspace.Worktree{{Path: "/backend"}}}
	runner := &roleRunner{}
	if _, err := (implementation.AgentImplementer{Runner: runner, Profile: agent.Profile{Provider: "codex"}}).Implement(context.Background(), implementation.ImplementContext{Workspace: issueWorkspace}); err == nil {
		t.Fatalf("expected decode error from empty runner result, got nil")
	}

	example := extractPromptExampleJSON(t, runner.prompt)

	var parsed implementation.ImplementationResult
	if err := json.Unmarshal([]byte(example), &parsed); err != nil {
		t.Fatalf("failed to parse prompt example JSON: %v\nexample: %s", err, example)
	}
	if err := delivery.ValidateEvidence(parsed.EvidencePolicy, parsed.Evidence); err != nil {
		t.Errorf("ValidateEvidence() rejected the prompt's own example evidence: %v\nexample: %s", err, example)
	}
}

// extractPromptExampleJSON pulls the single-line JSON example out of the
// Implementer prompt text (the line beginning with the "changes" example
// object), so the regression test exercises exactly what the model is shown.
func extractPromptExampleJSON(t *testing.T, prompt string) string {
	t.Helper()
	for _, line := range strings.Split(prompt, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, `{"changes"`) {
			return trimmed
		}
	}
	t.Fatalf("could not find example JSON line in prompt: %s", prompt)
	return ""
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
