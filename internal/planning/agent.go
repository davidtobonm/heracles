package planning

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/davidtobonm/heracles/internal/agent"
)

// AgentRunner is the provider-neutral execution boundary used by AgentPlanner.
type AgentRunner interface {
	Run(context.Context, string, agent.Profile, []string, string) (agent.Result, error)
}

// AgentPlanner uses a configured Agent Profile as the Planning Stage Planner.
type AgentPlanner struct {
	Runner  AgentRunner
	Profile agent.Profile
}

// Plan asks the configured Planner for one structured Planning Stage turn.
func (planner AgentPlanner) Plan(ctx context.Context, request PlannerRequest) (Response, error) {
	if planner.Runner == nil {
		return Response{}, errors.New("Agent Planner requires a Runner")
	}
	if len(request.Repositories) == 0 {
		return Response{}, errors.New("Agent Planner requires at least one Target Repository")
	}
	workspaces := make([]string, len(request.Repositories))
	for index, repository := range request.Repositories {
		workspaces[index] = repository.Path
	}
	contract, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return Response{}, fmt.Errorf("encode Planner context: %w", err)
	}
	prompt := `You are the configured Heracles Planner.

Explore every provided Target Repository workspace and use the relevant existing documentation in the context below.
Clarify only what is needed to make the product scope and later implementation unambiguous.
Respect the Question Budget. Finish early when possible. Request documentation updates only when they are genuinely needed.

Return JSON only, using exactly this shape:
{"questions":["..."],"prd":"...","documentation_updates":[{"path":"...","reason":"...","needed":true}]}
Set either questions or prd, never both.

Planning context:
` + string(contract)
	result, err := planner.Runner.Run(ctx, planner.Profile.Provider, planner.Profile, workspaces, prompt)
	if err != nil {
		return Response{}, err
	}
	var response struct {
		Questions            []string              `json:"questions"`
		PRD                  string                `json:"prd"`
		DocumentationUpdates []DocumentationUpdate `json:"documentation_updates"`
	}
	if err := json.Unmarshal([]byte(result.Final), &response); err != nil {
		return Response{}, fmt.Errorf("decode structured Planner response: %w", err)
	}
	return Response{
		Questions:            response.Questions,
		PRD:                  response.PRD,
		DocumentationUpdates: response.DocumentationUpdates,
	}, nil
}
