package setup

import (
	"context"
	"fmt"

	"github.com/davidtobonm/heracles/internal/issuestage"
	"github.com/davidtobonm/heracles/internal/project"
)

// bootstrapPRDTitle names the pre-approved PRD issue created when a Target
// Repository has no detected verification commands.
const bootstrapPRDTitle = "Heracles Project Bootstrap"

// BuildBootstrapProposal proposes establishing quality-gate commands for a
// repository whose verification commands could not be detected.
func BuildBootstrapProposal(repository project.RepositoryConfig, detectedStack string) issuestage.Proposal {
	what := fmt.Sprintf("Add a format-check, lint, test, and aggregate `check` command to %s so Heracles can verify changes automatically.", repository.Name)
	criteria := []string{
		"Running the aggregate check command exits non-zero on a formatting, lint, or test failure.",
		"Running the aggregate check command exits zero on a clean checkout.",
	}
	if detectedStack != "" {
		what = fmt.Sprintf("Add format-check, lint, test, and aggregate `check` commands for the %s project at %s.", detectedStack, repository.Name)
	}

	return issuestage.Proposal{
		ID:                 "bootstrap-" + repository.Name,
		Title:              fmt.Sprintf("Establish quality-gate commands for %s", repository.Name),
		Type:               issuestage.TypeAFK,
		UserStories:        []int{1},
		WhatToBuild:        what,
		AcceptanceCriteria: criteria,
		ExclusiveScopes:    []string{repository.Path},
	}
}

// PublishBootstrap creates a pre-approved Heracles Project Bootstrap PRD
// issue and one implementation issue per proposal, per ADR 0021.
func PublishBootstrap(ctx context.Context, publisher issuestage.Publisher, trackerRepo string, proposals []issuestage.Proposal) (string, map[string]string, error) {
	if len(proposals) == 0 {
		return "", nil, fmt.Errorf("PublishBootstrap requires at least one proposal")
	}

	prdBody := "Heracles detected one or more Target Repositories without established verification commands. " +
		"This pre-approved PRD tracks adding format-check, lint, test, and aggregate `check` commands so Heracles " +
		"can verify changes automatically, per ADR 0021.\n"
	prdURL, err := publisher.CreateIssue(ctx, issuestage.PublishInput{
		Repository: trackerRepo,
		Title:      bootstrapPRDTitle,
		Body:       prdBody,
		Labels:     []string{"heracles:prd", "heracles:approved"},
	})
	if err != nil {
		return "", nil, fmt.Errorf("create Heracles Project Bootstrap PRD: %w", err)
	}

	issueURLs := make(map[string]string, len(proposals))
	for _, proposal := range proposals {
		body := issuestage.Body(proposal) + fmt.Sprintf("\n\n_Part of Heracles Project Bootstrap: %s_\n", prdURL)
		url, err := publisher.CreateIssue(ctx, issuestage.PublishInput{
			Repository: trackerRepo,
			Title:      proposal.Title,
			Body:       body,
			Labels:     issuestage.Labels(proposal),
		})
		if err != nil {
			return prdURL, issueURLs, fmt.Errorf("publish proposal %q: %w", proposal.ID, err)
		}
		issueURLs[proposal.ID] = url
	}

	return prdURL, issueURLs, nil
}
