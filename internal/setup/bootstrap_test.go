package setup_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/issuestage"
	"github.com/davidtobonm/heracles/internal/project"
	"github.com/davidtobonm/heracles/internal/setup"
)

type fakePublisher struct {
	created []issuestage.PublishInput
	nextID  int
	err     error
}

func (publisher *fakePublisher) CreateIssue(_ context.Context, input issuestage.PublishInput) (string, error) {
	if publisher.err != nil {
		return "", publisher.err
	}
	publisher.nextID++
	publisher.created = append(publisher.created, input)
	return fmt.Sprintf("https://github.com/%s/issues/%d", input.Repository, publisher.nextID), nil
}

func TestBuildBootstrapProposal(t *testing.T) {
	t.Parallel()

	repository := project.RepositoryConfig{Name: "widget", Path: "widget"}
	proposal := setup.BuildBootstrapProposal(repository, "Go")

	if proposal.Type != issuestage.TypeAFK {
		t.Errorf("proposal.Type = %q, want %q", proposal.Type, issuestage.TypeAFK)
	}
	if !strings.Contains(proposal.Title, "widget") {
		t.Errorf("proposal.Title = %q, want it to mention the repository", proposal.Title)
	}
	if !strings.Contains(proposal.WhatToBuild, "Go") {
		t.Errorf("proposal.WhatToBuild = %q, want it to mention the detected stack", proposal.WhatToBuild)
	}
	if len(proposal.AcceptanceCriteria) == 0 {
		t.Errorf("proposal.AcceptanceCriteria is empty, want at least one criterion")
	}
	if want := []string{"widget"}; len(proposal.ExclusiveScopes) != 1 || proposal.ExclusiveScopes[0] != want[0] {
		t.Errorf("proposal.ExclusiveScopes = %v, want %v", proposal.ExclusiveScopes, want)
	}
}

func TestPublishBootstrapCreatesPRDAndIssues(t *testing.T) {
	t.Parallel()

	publisher := &fakePublisher{}
	proposals := []issuestage.Proposal{
		setup.BuildBootstrapProposal(project.RepositoryConfig{Name: "widget", Path: "widget"}, "Go"),
	}

	prdURL, issueURLs, err := setup.PublishBootstrap(context.Background(), publisher, "acme/backlog", proposals)
	if err != nil {
		t.Fatalf("PublishBootstrap() error = %v", err)
	}
	if prdURL != "https://github.com/acme/backlog/issues/1" {
		t.Errorf("prdURL = %q, want issue 1", prdURL)
	}
	if got := issueURLs[proposals[0].ID]; got != "https://github.com/acme/backlog/issues/2" {
		t.Errorf("issueURLs[%q] = %q, want issue 2", proposals[0].ID, got)
	}

	if len(publisher.created) != 2 {
		t.Fatalf("created %d issues, want 2", len(publisher.created))
	}
	prdInput := publisher.created[0]
	if prdInput.Title != "Heracles Project Bootstrap" {
		t.Errorf("PRD title = %q, want %q", prdInput.Title, "Heracles Project Bootstrap")
	}
	wantLabels := []string{"heracles:prd", "heracles:approved"}
	if len(prdInput.Labels) != len(wantLabels) || prdInput.Labels[0] != wantLabels[0] || prdInput.Labels[1] != wantLabels[1] {
		t.Errorf("PRD labels = %v, want %v", prdInput.Labels, wantLabels)
	}

	issueInput := publisher.created[1]
	if !strings.Contains(issueInput.Body, prdURL) {
		t.Errorf("issue body = %q, want it to reference the PRD URL %q", issueInput.Body, prdURL)
	}
}

func TestPublishBootstrapRequiresProposals(t *testing.T) {
	t.Parallel()

	if _, _, err := setup.PublishBootstrap(context.Background(), &fakePublisher{}, "acme/backlog", nil); err == nil {
		t.Fatal("PublishBootstrap() error = nil, want error for empty proposals")
	}
}

func TestPublishBootstrapPropagatesPublisherError(t *testing.T) {
	t.Parallel()

	publisher := &fakePublisher{err: fmt.Errorf("boom")}
	proposals := []issuestage.Proposal{
		setup.BuildBootstrapProposal(project.RepositoryConfig{Name: "widget", Path: "widget"}, "Go"),
	}

	if _, _, err := setup.PublishBootstrap(context.Background(), publisher, "acme/backlog", proposals); err == nil {
		t.Fatal("PublishBootstrap() error = nil, want propagated error")
	}
}
