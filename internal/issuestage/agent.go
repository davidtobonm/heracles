package issuestage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/davidtobonm/heracles/internal/agent"
)

// AgentRunner is the provider-neutral execution boundary used by AgentIssueAuthor.
type AgentRunner interface {
	Run(context.Context, string, agent.Profile, []string, string) (agent.Result, error)
}

// AgentIssueAuthor uses a configured Agent Profile to propose issues.
type AgentIssueAuthor struct {
	Runner     AgentRunner
	Profile    agent.Profile
	Workspaces []string
}

// Propose asks the configured Issue Author for structured tracer-bullet proposals.
func (author AgentIssueAuthor) Propose(ctx context.Context, request AuthorRequest) ([]Proposal, error) {
	if author.Runner == nil || len(author.Workspaces) == 0 {
		return nil, errors.New("Agent Issue Author requires a Runner and Target Repository workspaces")
	}
	contextJSON, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode Issue Author context: %w", err)
	}
	prompt := `You are the configured Heracles Issue Author.

Convert the approved PRD into independently executable tracer-bullet issue proposals.
Every proposal must classify as AFK or HITL, cover user stories, contain acceptance criteria, use full GitHub issue URLs for dependencies, and declare Exclusive Scopes when concurrent execution would be unsafe.
The user_stories field must contain only integer story numbers from the approved PRD, never story descriptions or strings.
Your final assistant message must be exactly one JSON array with no markdown fences and no surrounding commentary.
Use only these fields per item: id, title, type, user_stories, what_to_build, acceptance_criteria, blocked_by, exclusive_scopes, tdd_exemption_reason.

Issue Author context:
` + string(contextJSON)
	profile := author.Profile
	if strings.EqualFold(profile.Provider, "claude") {
		profile.ExtraArgs = append(append([]string(nil), profile.ExtraArgs...), "--output-format", "json", "--json-schema", issueAuthorSchema)
	}
	result, err := author.Runner.Run(ctx, profile.Provider, profile, author.Workspaces, prompt)
	if err != nil {
		return nil, err
	}
	var proposals []Proposal
	if err := agent.DecodeStructuredJSON(result.Final, &proposals); err != nil {
		return nil, fmt.Errorf("decode structured Issue Author response: %w", err)
	}
	return proposals, nil
}

const issueAuthorSchema = `{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"title":{"type":"string"},"type":{"type":"string"},"user_stories":{"type":"array","items":{"type":"integer"}},"what_to_build":{"type":"string"},"acceptance_criteria":{"type":"array","items":{"type":"string"}},"blocked_by":{"type":"array","items":{"type":"string"}},"exclusive_scopes":{"type":"array","items":{"type":"string"}},"tdd_exemption_reason":{"type":"string"}},"required":["id","title","type","user_stories","what_to_build","acceptance_criteria"],"additionalProperties":false}}`
