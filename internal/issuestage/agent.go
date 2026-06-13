package issuestage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
Return JSON only as an array using fields: id, title, type, user_stories, what_to_build, acceptance_criteria, blocked_by, exclusive_scopes, tdd_exemption_reason.

Issue Author context:
` + string(contextJSON)
	result, err := author.Runner.Run(ctx, author.Profile.Provider, author.Profile, author.Workspaces, prompt)
	if err != nil {
		return nil, err
	}
	var proposals []Proposal
	if err := json.Unmarshal([]byte(result.Final), &proposals); err != nil {
		return nil, fmt.Errorf("decode structured Issue Author response: %w", err)
	}
	return proposals, nil
}
