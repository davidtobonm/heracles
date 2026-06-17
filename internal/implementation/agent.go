package implementation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/delivery"
	"github.com/davidtobonm/heracles/internal/workspace"
)

// AgentRunner is the provider-neutral execution boundary used by agent roles.
type AgentRunner interface {
	Run(context.Context, string, agent.Profile, []string, string) (agent.Result, error)
}

// AgentImplementer uses a configured Agent Profile to implement one issue.
type AgentImplementer struct {
	Runner  AgentRunner
	Profile agent.Profile
}

// Implement asks the configured Implementer to work in every Issue Workspace worktree.
func (implementer AgentImplementer) Implement(ctx context.Context, implementContext ImplementContext) (ImplementationResult, error) {
	if implementer.Runner == nil {
		return ImplementationResult{}, errors.New("Agent Implementer requires a Runner")
	}
	workspaces := workspacePaths(implementContext.Workspace)
	if len(workspaces) == 0 {
		return ImplementationResult{}, errors.New("Agent Implementer requires an Issue Workspace")
	}
	contextJSON, err := json.MarshalIndent(implementContext, "", "  ")
	if err != nil {
		return ImplementationResult{}, fmt.Errorf("encode Implementer context: %w", err)
	}
	prompt := `You are the configured Heracles Implementer.

Implement the issue in the provided isolated worktrees. Work test-first and return auditable Red Evidence followed by Green Evidence unless the issue has a reasoned TDD Exemption.
Your final assistant message must be exactly one JSON object with no markdown fences and no surrounding commentary.
Use only these top-level fields: changes, evidence, evidence_policy.
Each evidence entry requires started_at and finished_at as RFC3339 timestamps reflecting when you actually ran that command. Red Evidence must finish before Green Evidence starts.
Example:
{"changes":"summarize the implementation","evidence":[{"kind":"red","command":"go test ./...","exit_code":1,"started_at":"2024-01-01T00:00:00Z","finished_at":"2024-01-01T00:00:05Z","artifact_path":"artifacts/red.txt"},{"kind":"green","command":"go test ./...","exit_code":0,"started_at":"2024-01-01T00:10:00Z","finished_at":"2024-01-01T00:10:05Z","artifact_path":"artifacts/green.txt"}],"evidence_policy":{"Exempt":false,"Reason":""}}

Implementation context:
` + string(contextJSON)
	profile := implementer.Profile
	if strings.EqualFold(profile.Provider, "claude") {
		profile.ExtraArgs = append(agentExtraArgs(profile.ExtraArgs), "--output-format", "json", "--json-schema", implementerSchema)
	}
	result, err := implementer.Runner.Run(ctx, profile.Provider, profile, workspaces, prompt)
	if err != nil {
		return ImplementationResult{}, err
	}
	var output ImplementationResult
	if err := agent.DecodeStructuredJSON(result.Final, &output); err != nil {
		return ImplementationResult{}, fmt.Errorf("decode structured Implementer response: %w; final output excerpt: %s", err, excerpt(result.Final))
	}
	return output, nil
}

// AgentReviewer uses a separately configured Agent Profile to review and correct one issue.
type AgentReviewer struct {
	Runner  AgentRunner
	Profile agent.Profile
}

// Review asks the configured Reviewer to inspect and optionally correct the delivery.
func (reviewer AgentReviewer) Review(ctx context.Context, issueWorkspace workspace.Workspace, reviewContext delivery.ReviewContext) (delivery.ReviewOutcome, error) {
	if reviewer.Runner == nil {
		return delivery.ReviewOutcome{}, errors.New("Agent Reviewer requires a Runner")
	}
	workspaces := workspacePaths(issueWorkspace)
	if len(workspaces) == 0 {
		return delivery.ReviewOutcome{}, errors.New("Agent Reviewer requires an Issue Workspace")
	}
	prompt := delivery.ReviewerPrompt(reviewContext) + `

Your final assistant message must be exactly one JSON object with no markdown fences and no surrounding commentary.
Use only these fields: status, summary, corrective_changes, verification.
`
	profile := reviewer.Profile
	if strings.EqualFold(profile.Provider, "claude") {
		profile.ExtraArgs = append(agentExtraArgs(profile.ExtraArgs), "--output-format", "json", "--json-schema", reviewerSchema)
	}
	result, err := reviewer.Runner.Run(ctx, profile.Provider, profile, workspaces, prompt)
	if err != nil {
		return delivery.ReviewOutcome{}, err
	}
	var outcome struct {
		Status            string                  `json:"status"`
		Summary           string                  `json:"summary"`
		CorrectiveChanges bool                    `json:"corrective_changes"`
		Verification      []delivery.Verification `json:"verification"`
	}
	if err := agent.DecodeStructuredJSON(result.Final, &outcome); err != nil {
		return delivery.ReviewOutcome{}, fmt.Errorf("decode structured Reviewer response: %w; final output excerpt: %s", err, excerpt(result.Final))
	}
	return delivery.ReviewOutcome{
		Status: outcome.Status, Summary: outcome.Summary,
		CorrectiveChanges: outcome.CorrectiveChanges, Verification: outcome.Verification,
	}, nil
}

func excerpt(text string) string {
	value := strings.TrimSpace(text)
	if len(value) <= 240 {
		return value
	}
	return strings.TrimSpace(value[:240]) + "..."
}

func agentExtraArgs(values []string) []string {
	return append([]string(nil), values...)
}

const implementerSchema = `{"type":"object","properties":{"changes":{"type":"string"},"evidence":{"type":"array","items":{"type":"object","properties":{"kind":{"type":"string"},"repository":{"type":"string"},"command":{"type":"string"},"exit_code":{"type":"integer"},"stdout":{"type":"string"},"stderr":{"type":"string"},"started_at":{"type":"string"},"finished_at":{"type":"string"},"artifact_path":{"type":"string"}},"required":["kind","command","exit_code","started_at","finished_at","artifact_path"],"additionalProperties":false}},"evidence_policy":{"type":"object","properties":{"Exempt":{"type":"boolean"},"Reason":{"type":"string"}},"required":["Exempt","Reason"],"additionalProperties":false}},"required":["changes","evidence_policy"],"additionalProperties":false}`

const reviewerSchema = `{"type":"object","properties":{"status":{"type":"string"},"summary":{"type":"string"},"corrective_changes":{"type":"boolean"},"verification":{"type":"array","items":{"type":"object","properties":{"repository":{"type":"string"},"command":{"type":"string"},"exit_code":{"type":"integer"},"stdout":{"type":"string"},"stderr":{"type":"string"},"started_at":{"type":"string"},"finished_at":{"type":"string"}},"required":["repository","command","exit_code","stdout","stderr","started_at","finished_at"],"additionalProperties":false}}},"required":["status","summary","corrective_changes","verification"],"additionalProperties":false}`

func workspacePaths(issueWorkspace workspace.Workspace) []string {
	paths := make([]string, len(issueWorkspace.Repositories))
	for index, repository := range issueWorkspace.Repositories {
		paths[index] = repository.Path
	}
	return paths
}
