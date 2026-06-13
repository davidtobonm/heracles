package issuestage_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/issuestage"
	"github.com/davidtobonm/heracles/internal/tracker"
)

func TestIssueStageAuthorsTracerBulletProposalsAndPausesForApproval(t *testing.T) {
	t.Parallel()

	store := issuestage.NewMemoryStore()
	author := &scriptedAuthor{responses: [][]issuestage.Proposal{proposals()}}
	state, err := (issuestage.Service{Author: author, Store: store}).Run(context.Background(), issuestage.RunRequest{
		ID:                "issues-1",
		ApprovedPRDPath:   "/artifacts/PRD.md",
		ApprovedPRD:       "# PRD",
		TrackerRepository: "acme/backlog",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if state.Status != issuestage.StatusAwaitingApproval || state.Gate.Status != issuestage.GatePending || len(state.Proposals) != 2 {
		t.Fatalf("state = %#v, want proposals paused at approval", state)
	}
	if author.requests[0].ApprovedPRD != "# PRD" || author.requests[0].TrackerRepository != "acme/backlog" {
		t.Errorf("author request = %#v, want approved PRD and tracker", author.requests[0])
	}
}

func TestIssuePublicationRequiresApprovalAndIsIdempotentlyResumable(t *testing.T) {
	t.Parallel()

	store := issuestage.NewMemoryStore()
	publisher := &fakePublisher{}
	service := issuestage.Service{Author: &scriptedAuthor{responses: [][]issuestage.Proposal{proposals()}}, Store: store, Publisher: publisher}
	_, err := service.Run(context.Background(), issuestage.RunRequest{
		ID: "issues-1", ApprovedPRDPath: "/PRD.md", ApprovedPRD: "# PRD", TrackerRepository: "acme/backlog",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if _, err := service.Publish(context.Background(), "issues-1"); err == nil || !strings.Contains(err.Error(), "approved") {
		t.Fatalf("Publish() before approval error = %v, want approval failure", err)
	}
	if _, err := service.Decide(context.Background(), "issues-1", issuestage.DecisionApprove, "Ship it"); err != nil {
		t.Fatalf("Decide() error = %v", err)
	}

	state, err := service.Publish(context.Background(), "issues-1")
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if state.Status != issuestage.StatusPublished || len(state.Published) != 2 || len(publisher.inputs) != 2 {
		t.Fatalf("published state = %#v, inputs = %#v", state, publisher.inputs)
	}
	for _, input := range publisher.inputs {
		for _, section := range []string{"## Type", "## User stories covered", "## What to build", "## Acceptance criteria", "## Blocked by", "## Exclusive Scopes"} {
			if !strings.Contains(input.Body, section) {
				t.Errorf("issue body missing %q: %s", section, input.Body)
			}
		}
	}
	if strings.Join(publisher.inputs[0].Labels, ",") != tracker.LabelReady || strings.Join(publisher.inputs[1].Labels, ",") != tracker.LabelHITL {
		t.Errorf("labels = %#v / %#v, want executable AFK and HITL states", publisher.inputs[0].Labels, publisher.inputs[1].Labels)
	}

	resumed, err := service.Publish(context.Background(), "issues-1")
	if err != nil {
		t.Fatalf("Publish(resume) error = %v", err)
	}
	if resumed.Status != issuestage.StatusPublished || len(publisher.inputs) != 2 {
		t.Errorf("resume created duplicates: %#v", publisher.inputs)
	}
}

func TestIssuePublicationResumesAfterPartialFailureWithoutDuplicates(t *testing.T) {
	t.Parallel()

	store := issuestage.NewMemoryStore()
	publisher := &fakePublisher{failAt: 2}
	service := issuestage.Service{Author: &scriptedAuthor{responses: [][]issuestage.Proposal{proposals()}}, Store: store, Publisher: publisher}
	if _, err := service.Run(context.Background(), issuestage.RunRequest{ID: "issues-1", ApprovedPRD: "# PRD", TrackerRepository: "acme/backlog"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if _, err := service.Decide(context.Background(), "issues-1", issuestage.DecisionApprove, "Approved"); err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	state, err := service.Publish(context.Background(), "issues-1")
	if err == nil || state.Status != issuestage.StatusPublishing || len(state.Published) != 1 {
		t.Fatalf("Publish() = %#v, %v; want durable partial publication", state, err)
	}

	publisher.failAt = 0
	state, err = service.Publish(context.Background(), "issues-1")
	if err != nil {
		t.Fatalf("Publish(resume) error = %v", err)
	}
	if state.Status != issuestage.StatusPublished || len(publisher.inputs) != 3 {
		t.Errorf("resume = %#v, calls = %#v; want first proposal skipped and second retried", state, publisher.inputs)
	}
}

func TestIssueStageRejectsInvalidProposalContracts(t *testing.T) {
	t.Parallel()

	for name, proposal := range map[string]issuestage.Proposal{
		"missing classification": {ID: "one", Title: "Title", UserStories: []int{1}, WhatToBuild: "Build", AcceptanceCriteria: []string{"Works"}},
		"relative dependency":    {ID: "one", Title: "Title", Type: issuestage.TypeAFK, UserStories: []int{1}, WhatToBuild: "Build", AcceptanceCriteria: []string{"Works"}, BlockedBy: []string{"#7"}},
		"missing acceptance":     {ID: "one", Title: "Title", Type: issuestage.TypeAFK, UserStories: []int{1}, WhatToBuild: "Build"},
	} {
		t.Run(name, func(t *testing.T) {
			service := issuestage.Service{Author: &scriptedAuthor{responses: [][]issuestage.Proposal{{proposal}}}, Store: issuestage.NewMemoryStore()}
			_, err := service.Run(context.Background(), issuestage.RunRequest{ID: "issues-1", ApprovedPRD: "# PRD", TrackerRepository: "acme/backlog"})
			if err == nil {
				t.Fatal("Run() error = nil, want proposal contract failure")
			}
		})
	}
}

func TestIssueStageRejectionRevisesWithoutReplayingApprovalPause(t *testing.T) {
	t.Parallel()

	store := issuestage.NewMemoryStore()
	first := &scriptedAuthor{responses: [][]issuestage.Proposal{proposals()}}
	service := issuestage.Service{Author: first, Store: store}
	if _, err := service.Run(context.Background(), issuestage.RunRequest{ID: "issues-1", ApprovedPRD: "# PRD", TrackerRepository: "acme/backlog"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	resumeAuthor := &scriptedAuthor{}
	if _, err := (issuestage.Service{Author: resumeAuthor, Store: store}).Run(context.Background(), issuestage.RunRequest{ID: "issues-1"}); err != nil {
		t.Fatalf("Run(resume) error = %v", err)
	}
	if len(resumeAuthor.requests) != 0 {
		t.Errorf("resume repeated author work: %#v", resumeAuthor.requests)
	}
	if _, err := service.Decide(context.Background(), "issues-1", issuestage.DecisionReject, "Split the second slice"); err != nil {
		t.Fatalf("Decide(reject) error = %v", err)
	}
	revision := &scriptedAuthor{responses: [][]issuestage.Proposal{proposals()[:1]}}
	state, err := (issuestage.Service{Author: revision, Store: store}).Run(context.Background(), issuestage.RunRequest{ID: "issues-1"})
	if err != nil {
		t.Fatalf("Run(revision) error = %v", err)
	}
	if state.Status != issuestage.StatusAwaitingApproval || revision.requests[0].RevisionFeedback != "Split the second slice" {
		t.Errorf("revision state/request = %#v / %#v", state, revision.requests[0])
	}
}

func TestIssueStageFileStoreDurablyResumesApprovalPause(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	service := issuestage.Service{
		Author: &scriptedAuthor{responses: [][]issuestage.Proposal{proposals()}},
		Store:  issuestage.NewFileStore(root),
	}
	if _, err := service.Run(context.Background(), issuestage.RunRequest{ID: "issues-1", ApprovedPRD: "# PRD", TrackerRepository: "acme/backlog"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	resumeAuthor := &scriptedAuthor{}
	state, err := (issuestage.Service{Author: resumeAuthor, Store: issuestage.NewFileStore(root)}).Run(context.Background(), issuestage.RunRequest{ID: "issues-1"})
	if err != nil {
		t.Fatalf("Run(reopened) error = %v", err)
	}
	if state.Status != issuestage.StatusAwaitingApproval || len(resumeAuthor.requests) != 0 {
		t.Errorf("resumed state = %#v, author calls = %d", state, len(resumeAuthor.requests))
	}
}

func proposals() []issuestage.Proposal {
	return []issuestage.Proposal{
		{
			ID: "api", Title: "Deliver API slice", Type: issuestage.TypeAFK, UserStories: []int{1, 2},
			WhatToBuild: "Deliver one end-to-end API slice.", AcceptanceCriteria: []string{"Request succeeds", "Tests pass"},
			BlockedBy: []string{"https://github.com/acme/backlog/issues/1"},
		},
		{
			ID: "decision", Title: "Choose rollout policy", Type: issuestage.TypeHITL, UserStories: []int{3},
			WhatToBuild: "Choose the rollout policy.", AcceptanceCriteria: []string{"Decision is recorded"},
			ExclusiveScopes: []string{"rollout policy"},
		},
	}
}

type scriptedAuthor struct {
	responses [][]issuestage.Proposal
	requests  []issuestage.AuthorRequest
}

func (author *scriptedAuthor) Propose(_ context.Context, request issuestage.AuthorRequest) ([]issuestage.Proposal, error) {
	author.requests = append(author.requests, request)
	if len(author.responses) == 0 {
		return nil, errors.New("unexpected author call")
	}
	response := author.responses[0]
	author.responses = author.responses[1:]
	return response, nil
}

type fakePublisher struct {
	inputs []issuestage.PublishInput
	failAt int
}

func (publisher *fakePublisher) CreateIssue(_ context.Context, input issuestage.PublishInput) (string, error) {
	publisher.inputs = append(publisher.inputs, input)
	if publisher.failAt == len(publisher.inputs) {
		return "", errors.New("publish interrupted")
	}
	return fmt.Sprintf("https://github.com/%s/issues/%d", input.Repository, len(publisher.inputs)), nil
}
