package issues

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

// AgentIssueAuthor uses a configured Agent Profile to propose implementation issues.
type AgentIssueAuthor struct {
	Runner     AgentRunner
	Profile    agent.Profile
	Workspaces []string
}

// Propose asks the configured Issue Author for structured implementation issue proposals.
func (author AgentIssueAuthor) Propose(ctx context.Context, request AuthorRequest) (AuthorResponse, error) {
	if author.Runner == nil || len(author.Workspaces) == 0 {
		return AuthorResponse{}, errors.New("Agent Issue Author requires a Runner and Target Repository workspaces")
	}
	contextJSON, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return AuthorResponse{}, fmt.Errorf("encode Issue Author context: %w", err)
	}
	prompt := `You are the configured Heracles Issue Author running the to-issues-for-heracles skill.

Convert the approved PRD into independently executable implementation issue proposals.
Every proposal must classify as AFK or HITL, cover user stories, contain acceptance criteria, declare its Target Repositories, and may declare Conflict Keys for project-defined shared resources that make concurrent execution unsafe.
The user_stories field must contain only integer story numbers from the approved PRD, never story descriptions or strings.
Assign every proposal a unique, stable, kebab-case semantic ID (e.g. "auth-login-flow"). Heracles uses this ID, not the title, to track the issue across PRD revisions.
Dependencies in blocked_by must reference either another proposal's semantic ID in this same response, or a full https://github.com/owner/repo/issues/N URL for issues outside this response. The dependency graph must be acyclic; Heracles publishes dependencies first and substitutes their URLs.
Do not reference the Parent PRD URL or embed any markers yourself; Heracles adds the Parent PRD section and tracking markers automatically.

If, and only if, safe decomposition is impossible because the approved PRD leaves an unsafe assumption, return no proposals and set "blocked" to the exact clarification needed on the Parent PRD Issue. Do not publish issues in this case.

Your final assistant message must be exactly one JSON object with no markdown fences and no surrounding commentary.
Use only these top-level fields: proposals, blocked.
Use only these fields per proposal: id, title, type, user_stories, what_to_build, acceptance_criteria, target_repositories, conflict_keys, blocked_by, tdd_exemption_reason.

Issue Author context:
` + string(contextJSON)
	profile := author.Profile
	if strings.EqualFold(profile.Provider, "claude") {
		profile.ExtraArgs = append(append([]string(nil), profile.ExtraArgs...), "--output-format", "json", "--json-schema", issueAuthorSchema)
	}
	result, err := author.Runner.Run(ctx, profile.Provider, profile, author.Workspaces, prompt)
	if err != nil {
		return AuthorResponse{}, err
	}
	var response AuthorResponse
	if err := agent.DecodeStructuredJSON(result.Final, &response); err != nil {
		return AuthorResponse{}, fmt.Errorf("decode structured Issue Author response: %w", err)
	}
	return response, nil
}

const issueAuthorSchema = `{"type":"object","properties":{"proposals":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"title":{"type":"string"},"type":{"type":"string"},"user_stories":{"type":"array","items":{"type":"integer"}},"what_to_build":{"type":"string"},"acceptance_criteria":{"type":"array","items":{"type":"string"}},"target_repositories":{"type":"array","items":{"type":"string"}},"conflict_keys":{"type":"array","items":{"type":"string"}},"blocked_by":{"type":"array","items":{"type":"string"}},"tdd_exemption_reason":{"type":"string"}},"required":["id","title","type","user_stories","what_to_build","acceptance_criteria","target_repositories"],"additionalProperties":false}},"blocked":{"type":"string"}},"additionalProperties":false}`
