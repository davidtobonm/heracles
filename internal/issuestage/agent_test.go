package issuestage_test

import (
	"context"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/issuestage"
)

func TestAgentIssueAuthorUsesSeparateConfiguredProfileAndStructuredContract(t *testing.T) {
	t.Parallel()

	runner := &fakeIssueAgentRunner{result: agent.Result{Final: `[{"id":"slice","title":"Deliver slice","type":"AFK","user_stories":[1],"what_to_build":"Build it","acceptance_criteria":["Works"]}]`}}
	author := issuestage.AgentIssueAuthor{
		Runner: runner, Profile: agent.Profile{Provider: "opencode", Model: "model-a"},
		Workspaces: []string{"/backend", "/frontend"},
	}
	proposals, err := author.Propose(context.Background(), issuestage.AuthorRequest{ApprovedPRD: "# PRD", TrackerRepository: "acme/backlog"})
	if err != nil {
		t.Fatalf("Propose() error = %v", err)
	}
	if len(proposals) != 1 || runner.provider != "opencode" || strings.Join(runner.workspaces, ",") != "/backend,/frontend" {
		t.Errorf("proposals/runner = %#v / %#v", proposals, runner)
	}
	for _, expected := range []string{"approved PRD", "AFK or HITL", "full GitHub issue URLs", "Exclusive Scopes", "integer story numbers", "exactly one JSON array"} {
		if !strings.Contains(runner.prompt, expected) {
			t.Errorf("prompt does not contain %q: %s", expected, runner.prompt)
		}
	}
}

func TestAgentIssueAuthorAcceptsProseWrappedJSON(t *testing.T) {
	t.Parallel()

	runner := &fakeIssueAgentRunner{result: agent.Result{Final: "Here is the proposal:\n[{\"id\":\"slice\",\"title\":\"Deliver slice\",\"type\":\"AFK\",\"user_stories\":[1],\"what_to_build\":\"Build it\",\"acceptance_criteria\":[\"Works\"]}]\nGood luck."}}
	author := issuestage.AgentIssueAuthor{
		Runner: runner, Profile: agent.Profile{Provider: "opencode", Model: "model-a"},
		Workspaces: []string{"/backend"},
	}
	proposals, err := author.Propose(context.Background(), issuestage.AuthorRequest{ApprovedPRD: "# PRD", TrackerRepository: "acme/backlog"})
	if err != nil {
		t.Fatalf("Propose() error = %v", err)
	}
	if len(proposals) != 1 || proposals[0].ID != "slice" {
		t.Errorf("proposals = %#v, want extracted structured JSON", proposals)
	}
}

type fakeIssueAgentRunner struct {
	result     agent.Result
	provider   string
	profile    agent.Profile
	workspaces []string
	prompt     string
}

func (runner *fakeIssueAgentRunner) Run(_ context.Context, provider string, profile agent.Profile, workspaces []string, prompt string) (agent.Result, error) {
	runner.provider = provider
	runner.profile = profile
	runner.workspaces = append([]string(nil), workspaces...)
	runner.prompt = prompt
	return runner.result, nil
}
