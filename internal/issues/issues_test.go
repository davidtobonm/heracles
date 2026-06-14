package issues_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/issues"
	"github.com/davidtobonm/heracles/internal/tracker"
)

type recordingAuthor struct {
	response issues.AuthorResponse
	err      error
	requests []issues.AuthorRequest
}

func (author *recordingAuthor) Propose(_ context.Context, request issues.AuthorRequest) (issues.AuthorResponse, error) {
	author.requests = append(author.requests, request)
	return author.response, author.err
}

type fakeTracker struct {
	open      []tracker.Issue
	created   []created
	updated   []updated
	labeled   []labeled
	comments  []comment
	createErr error
}

type created struct {
	repository, title, body string
	labels                  []string
}

type updated struct {
	reference   tracker.Reference
	title, body string
	labels      []string
}

type labeled struct {
	reference tracker.Reference
	labels    []string
}

type comment struct {
	reference tracker.Reference
	body      string
}

func (f *fakeTracker) ListOpenIssues(context.Context, string) ([]tracker.Issue, error) {
	return f.open, nil
}

func (f *fakeTracker) CreateIssue(_ context.Context, repository, title, body string, labels []string) (string, error) {
	if f.createErr != nil {
		return "", f.createErr
	}
	f.created = append(f.created, created{repository: repository, title: title, body: body, labels: labels})
	return "https://github.com/acme/backlog/issues/" + nextNumber(len(f.created)), nil
}

func (f *fakeTracker) UpdateIssue(_ context.Context, reference tracker.Reference, title, body string, labels []string) error {
	f.updated = append(f.updated, updated{reference: reference, title: title, body: body, labels: labels})
	return nil
}

func (f *fakeTracker) SetLabels(_ context.Context, reference tracker.Reference, labels []string) error {
	f.labeled = append(f.labeled, labeled{reference: reference, labels: labels})
	return nil
}

func (f *fakeTracker) Comment(_ context.Context, reference tracker.Reference, body string) error {
	f.comments = append(f.comments, comment{reference: reference, body: body})
	return nil
}

func nextNumber(count int) string {
	switch count {
	case 1:
		return "100"
	case 2:
		return "101"
	default:
		return "102"
	}
}

const parentPRDURL = "https://github.com/acme/backlog/issues/36"

func validProposal() issues.Proposal {
	return issues.Proposal{
		ID: "auth-login-flow", Title: "Add login flow", Type: issues.TypeAFK,
		UserStories: []int{1, 2}, WhatToBuild: "Build the login flow.",
		AcceptanceCriteria: []string{"Users can log in."}, TargetRepositories: []string{"app"},
	}
}

func TestGeneratePublishesNewProposals(t *testing.T) {
	t.Parallel()

	author := &recordingAuthor{response: issues.AuthorResponse{Proposals: []issues.Proposal{validProposal()}}}
	tracker := &fakeTracker{}
	service := issues.Service{Author: author, Tracker: tracker, Store: issues.NewMemoryStore()}

	state, err := service.Generate(context.Background(), issues.GenerateRequest{
		ID: "prd-36", ParentPRDURL: parentPRDURL, ApprovedPRD: "# PRD\n\nBuild auth.", TrackerRepository: "acme/backlog",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if state.Status != issues.StatusGenerated {
		t.Fatalf("status = %q, want %q", state.Status, issues.StatusGenerated)
	}
	if len(tracker.created) != 1 {
		t.Fatalf("created = %#v, want one published issue", tracker.created)
	}
	body := tracker.created[0].body
	for _, want := range []string{"## Parent PRD", parentPRDURL, "<!-- heracles:issue-id=auth-login-flow -->", "<!-- heracles:prd-revision=" + state.Revision + " -->"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q\n%s", want, body)
		}
	}
	if !contains(tracker.created[0].labels, "heracles:implementation") || !contains(tracker.created[0].labels, "heracles:ready") {
		t.Errorf("labels = %#v, want implementation and ready", tracker.created[0].labels)
	}
	if state.Published["auth-login-flow"] != "https://github.com/acme/backlog/issues/100" {
		t.Errorf("published = %#v, want recorded created issue URL", state.Published)
	}
}

func TestGenerateIsIdempotentForUnchangedRevision(t *testing.T) {
	t.Parallel()

	author := &recordingAuthor{response: issues.AuthorResponse{Proposals: []issues.Proposal{validProposal()}}}
	tracker := &fakeTracker{}
	service := issues.Service{Author: author, Tracker: tracker, Store: issues.NewMemoryStore()}

	request := issues.GenerateRequest{ID: "prd-36", ParentPRDURL: parentPRDURL, ApprovedPRD: "# PRD\n\nBuild auth.", TrackerRepository: "acme/backlog"}
	if _, err := service.Generate(context.Background(), request); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if _, err := service.Generate(context.Background(), request); err != nil {
		t.Fatalf("Generate() second call error = %v", err)
	}
	if len(author.requests) != 1 {
		t.Errorf("Issue Author called %d times, want 1 for an unchanged revision", len(author.requests))
	}
	if len(tracker.created) != 1 {
		t.Errorf("created %d issues, want 1 for an unchanged revision", len(tracker.created))
	}
}

func TestGenerateBlocksOnExceptionalAmbiguity(t *testing.T) {
	t.Parallel()

	author := &recordingAuthor{response: issues.AuthorResponse{Blocked: "Need clarification on auth provider."}}
	tracker := &fakeTracker{}
	service := issues.Service{Author: author, Tracker: tracker, Store: issues.NewMemoryStore()}

	state, err := service.Generate(context.Background(), issues.GenerateRequest{
		ID: "prd-36", ParentPRDURL: parentPRDURL, ApprovedPRD: "# PRD\n\nBuild auth.", TrackerRepository: "acme/backlog",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if state.Status != issues.StatusBlocked || state.Blocked == "" {
		t.Fatalf("state = %#v, want blocked with a reason", state)
	}
	if len(tracker.created) != 0 || len(tracker.updated) != 0 {
		t.Fatalf("tracker = %#v, want no mutations when blocked", tracker)
	}
	if len(tracker.comments) != 1 || !strings.Contains(tracker.comments[0].body, "Need clarification") {
		t.Fatalf("comments = %#v, want actionable context on the Parent PRD Issue", tracker.comments)
	}
}

func TestGenerateUpdatesUntouchedIssueOnRevision(t *testing.T) {
	t.Parallel()

	existingBody := issues.Body(validProposal(), parentPRDURL, "old-revision", nil)
	tracker := &fakeTracker{open: []tracker.Issue{{
		Reference: trackerReference(10), URL: "https://github.com/acme/backlog/issues/10",
		Body: existingBody, Labels: []string{"heracles:implementation", "heracles:ready"},
	}}}
	updatedProposal := validProposal()
	updatedProposal.Title = "Add login flow v2"
	author := &recordingAuthor{response: issues.AuthorResponse{Proposals: []issues.Proposal{updatedProposal}}}
	service := issues.Service{Author: author, Tracker: tracker, Store: issues.NewMemoryStore()}

	state, err := service.Generate(context.Background(), issues.GenerateRequest{
		ID: "prd-36", ParentPRDURL: parentPRDURL, ApprovedPRD: "# PRD\n\nBuild auth v2.", TrackerRepository: "acme/backlog",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(tracker.created) != 0 || len(tracker.updated) != 1 {
		t.Fatalf("tracker = %#v, want one update and no creates", tracker)
	}
	if tracker.updated[0].title != "Add login flow v2" {
		t.Errorf("updated title = %q, want %q", tracker.updated[0].title, "Add login flow v2")
	}
	if state.Published["auth-login-flow"] != "https://github.com/acme/backlog/issues/10" {
		t.Errorf("published = %#v, want preserved existing URL", state.Published)
	}
}

func TestGeneratePreservesInProgressIssue(t *testing.T) {
	t.Parallel()

	existingBody := issues.Body(validProposal(), parentPRDURL, "old-revision", nil)
	tracker := &fakeTracker{open: []tracker.Issue{{
		Reference: trackerReference(10), URL: "https://github.com/acme/backlog/issues/10",
		Body: existingBody, Labels: []string{"heracles:implementation", "heracles:in-progress"},
	}}}
	updatedProposal := validProposal()
	updatedProposal.Title = "Add login flow v2"
	author := &recordingAuthor{response: issues.AuthorResponse{Proposals: []issues.Proposal{updatedProposal}}}
	service := issues.Service{Author: author, Tracker: tracker, Store: issues.NewMemoryStore()}

	_, err := service.Generate(context.Background(), issues.GenerateRequest{
		ID: "prd-36", ParentPRDURL: parentPRDURL, ApprovedPRD: "# PRD\n\nBuild auth v2.", TrackerRepository: "acme/backlog",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(tracker.created) != 0 || len(tracker.updated) != 0 {
		t.Fatalf("tracker = %#v, want in-progress issue left untouched", tracker)
	}
}

func TestGenerateMarksRemovedUntouchedIssueObsoleteWithoutClosing(t *testing.T) {
	t.Parallel()

	removedProposal := issues.Proposal{
		ID: "old-feature", Title: "Old feature", Type: issues.TypeAFK, UserStories: []int{9},
		WhatToBuild: "Old.", AcceptanceCriteria: []string{"Old."}, TargetRepositories: []string{"app"},
	}
	existingBody := issues.Body(removedProposal, parentPRDURL, "old-revision", nil)
	tracker := &fakeTracker{open: []tracker.Issue{{
		Reference: trackerReference(11), URL: "https://github.com/acme/backlog/issues/11",
		Body: existingBody, Labels: []string{"heracles:implementation", "heracles:ready"},
	}}}
	author := &recordingAuthor{response: issues.AuthorResponse{Proposals: []issues.Proposal{validProposal()}}}
	service := issues.Service{Author: author, Tracker: tracker, Store: issues.NewMemoryStore()}

	_, err := service.Generate(context.Background(), issues.GenerateRequest{
		ID: "prd-36", ParentPRDURL: parentPRDURL, ApprovedPRD: "# PRD\n\nBuild auth v2.", TrackerRepository: "acme/backlog",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(tracker.labeled) != 1 {
		t.Fatalf("labeled = %#v, want one obsolete relabel", tracker.labeled)
	}
	if !contains(tracker.labeled[0].labels, "heracles:obsolete") || contains(tracker.labeled[0].labels, "heracles:ready") {
		t.Errorf("labels = %#v, want obsolete without ready", tracker.labeled[0].labels)
	}
	if len(tracker.comments) != 1 {
		t.Errorf("comments = %#v, want one superseding comment", tracker.comments)
	}
}

func TestValidateProposalsRejectsDuplicateSemanticIDs(t *testing.T) {
	t.Parallel()

	proposal := validProposal()
	if _, err := issues.ValidateProposals([]issues.Proposal{proposal, proposal}); err == nil {
		t.Fatal("ValidateProposals() error = nil, want error for duplicate semantic IDs")
	}
}

func TestValidateProposalsRejectsCyclicDependencies(t *testing.T) {
	t.Parallel()

	first := validProposal()
	first.ID = "a"
	first.BlockedBy = []string{"b"}
	second := validProposal()
	second.ID = "b"
	second.BlockedBy = []string{"a"}
	if _, err := issues.ValidateProposals([]issues.Proposal{first, second}); err == nil {
		t.Fatal("ValidateProposals() error = nil, want error for cyclic dependencies")
	}
}

func TestValidateProposalsOrdersDependenciesBeforeDependents(t *testing.T) {
	t.Parallel()

	first := validProposal()
	first.ID = "a"
	second := validProposal()
	second.ID = "b"
	second.BlockedBy = []string{"a"}
	order, err := issues.ValidateProposals([]issues.Proposal{second, first})
	if err != nil {
		t.Fatalf("ValidateProposals() error = %v", err)
	}
	if len(order) != 2 || order[0] != "a" || order[1] != "b" {
		t.Fatalf("order = %#v, want [a b]", order)
	}
}

func TestValidateProposalsRejectsUnknownDependency(t *testing.T) {
	t.Parallel()

	proposal := validProposal()
	proposal.BlockedBy = []string{"not-a-url-or-id"}
	if _, err := issues.ValidateProposals([]issues.Proposal{proposal}); err == nil {
		t.Fatal("ValidateProposals() error = nil, want error for unknown dependency reference")
	}
}

func TestGenerateSubstitutesSemanticDependencyURLs(t *testing.T) {
	t.Parallel()

	first := validProposal()
	first.ID = "auth-backend"
	second := validProposal()
	second.ID = "auth-frontend"
	second.BlockedBy = []string{"auth-backend", "https://github.com/acme/other/issues/5"}
	author := &recordingAuthor{response: issues.AuthorResponse{Proposals: []issues.Proposal{second, first}}}
	tracker := &fakeTracker{}
	service := issues.Service{Author: author, Tracker: tracker, Store: issues.NewMemoryStore()}

	if _, err := service.Generate(context.Background(), issues.GenerateRequest{
		ID: "prd-36", ParentPRDURL: parentPRDURL, ApprovedPRD: "# PRD\n\nBuild auth.", TrackerRepository: "acme/backlog",
	}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(tracker.created) != 2 {
		t.Fatalf("created = %#v, want two published issues", tracker.created)
	}
	frontendBody := tracker.created[1].body
	if !strings.Contains(frontendBody, "https://github.com/acme/backlog/issues/100") {
		t.Errorf("frontend body missing substituted backend URL\n%s", frontendBody)
	}
	if !strings.Contains(frontendBody, "https://github.com/acme/other/issues/5") {
		t.Errorf("frontend body missing external dependency URL\n%s", frontendBody)
	}
}

func TestGenerateRequiresInputs(t *testing.T) {
	t.Parallel()

	service := issues.Service{Author: &recordingAuthor{}, Tracker: &fakeTracker{}, Store: issues.NewMemoryStore()}
	if _, err := service.Generate(context.Background(), issues.GenerateRequest{}); err == nil {
		t.Fatal("Generate() error = nil, want error for missing inputs")
	}
}

func TestFileStoreRoundTrip(t *testing.T) {
	t.Parallel()

	store := issues.NewFileStore(t.TempDir())
	if _, err := store.Load(context.Background(), "missing"); !errors.Is(err, issues.ErrNotFound) {
		t.Fatalf("Load() error = %v, want ErrNotFound", err)
	}
	state := issues.State{ID: "prd-36", ParentPRDURL: parentPRDURL, Revision: "abc", Status: issues.StatusGenerated, Published: map[string]string{"auth-login-flow": "https://github.com/acme/backlog/issues/100"}}
	if err := store.Save(context.Background(), state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := store.Load(context.Background(), "prd-36")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Revision != state.Revision || loaded.Published["auth-login-flow"] != state.Published["auth-login-flow"] {
		t.Fatalf("loaded = %#v, want %#v", loaded, state)
	}
}

func trackerReference(number int) tracker.Reference {
	return tracker.Reference{Owner: "acme", Repo: "backlog", Number: number}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
