package implementation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
Return JSON only using fields: changes, evidence, evidence_policy.

Implementation context:
` + string(contextJSON)
	result, err := implementer.Runner.Run(ctx, implementer.Profile.Provider, implementer.Profile, workspaces, prompt)
	if err != nil {
		return ImplementationResult{}, err
	}
	var output ImplementationResult
	if err := json.Unmarshal([]byte(result.Final), &output); err != nil {
		return ImplementationResult{}, fmt.Errorf("decode structured Implementer response: %w", err)
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

Return JSON only using fields: status, summary, corrective_changes, verification.
`
	result, err := reviewer.Runner.Run(ctx, reviewer.Profile.Provider, reviewer.Profile, workspaces, prompt)
	if err != nil {
		return delivery.ReviewOutcome{}, err
	}
	var outcome struct {
		Status            string                  `json:"status"`
		Summary           string                  `json:"summary"`
		CorrectiveChanges bool                    `json:"corrective_changes"`
		Verification      []delivery.Verification `json:"verification"`
	}
	if err := json.Unmarshal([]byte(result.Final), &outcome); err != nil {
		return delivery.ReviewOutcome{}, fmt.Errorf("decode structured Reviewer response: %w", err)
	}
	return delivery.ReviewOutcome{
		Status: outcome.Status, Summary: outcome.Summary,
		CorrectiveChanges: outcome.CorrectiveChanges, Verification: outcome.Verification,
	}, nil
}

func workspacePaths(issueWorkspace workspace.Workspace) []string {
	paths := make([]string, len(issueWorkspace.Repositories))
	for index, repository := range issueWorkspace.Repositories {
		paths[index] = repository.Path
	}
	return paths
}
