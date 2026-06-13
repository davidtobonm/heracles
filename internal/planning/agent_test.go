package planning_test

import (
	"context"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/planning"
)

func TestAgentPlannerUsesConfiguredProfileAndAllRepositoryWorkspaces(t *testing.T) {
	t.Parallel()

	runner := &fakeAgentRunner{result: agent.Result{Final: `{"prd":"# PRD","documentation_updates":[{"path":"CONTEXT.md","reason":"needed","needed":true}]}`}}
	profile := agent.Profile{Name: "planner", Provider: "claude", Model: "sonnet"}
	planner := planning.AgentPlanner{Runner: runner, Profile: profile}
	response, err := planner.Plan(context.Background(), planning.PlannerRequest{
		Problem: "Clarify delivery",
		Repositories: []planning.RepositoryContext{
			{Name: "backend", Path: "/work/backend"},
			{Name: "frontend", Path: "/work/frontend"},
		},
		Documents:      []planning.Document{{Path: "CONTEXT.md", Contents: "Vocabulary"}},
		QuestionBudget: 20,
	})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if response.PRD != "# PRD" || runner.provider != "claude" || runner.profile.Model != "sonnet" {
		t.Errorf("response/profile = %#v / %#v, want configured Planner result", response, runner.profile)
	}
	if strings.Join(runner.workspaces, ",") != "/work/backend,/work/frontend" {
		t.Errorf("workspaces = %#v, want all Target Repositories", runner.workspaces)
	}
	for _, expected := range []string{"Clarify delivery", "CONTEXT.md", "Question Budget", "JSON"} {
		if !strings.Contains(runner.prompt, expected) {
			t.Errorf("prompt does not contain %q: %s", expected, runner.prompt)
		}
	}
}

type fakeAgentRunner struct {
	result     agent.Result
	provider   string
	profile    agent.Profile
	workspaces []string
	prompt     string
}

func (runner *fakeAgentRunner) Run(_ context.Context, provider string, profile agent.Profile, workspaces []string, prompt string) (agent.Result, error) {
	runner.provider = provider
	runner.profile = profile
	runner.workspaces = append([]string(nil), workspaces...)
	runner.prompt = prompt
	return runner.result, nil
}
