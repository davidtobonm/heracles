package implementation_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/davidtobonm/heracles/internal/changeset"
	"github.com/davidtobonm/heracles/internal/delivery"
	"github.com/davidtobonm/heracles/internal/implementation"
	"github.com/davidtobonm/heracles/internal/tracker"
	"github.com/davidtobonm/heracles/internal/workspace"
)

func TestImplementationStageRunsCompleteAttemptSequence(t *testing.T) {
	t.Parallel()

	fixture := newFixture()
	state, err := fixture.service.Run(context.Background(), request())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if state.Status != implementation.StatusCompleted {
		t.Fatalf("state = %#v, want completed", state)
	}
	want := "claim,workspace,implement,review,verify,deliver,complete,finalize:success"
	if strings.Join(fixture.calls, ",") != want {
		t.Errorf("calls = %q, want %q", strings.Join(fixture.calls, ","), want)
	}
	for _, status := range []string{
		implementation.StatusClaimed, implementation.StatusWorkspaceReady, implementation.StatusImplemented,
		implementation.StatusReviewed, implementation.StatusVerified, implementation.StatusDelivered, implementation.StatusCompleted,
	} {
		if !eventStatus(state.Events, status) {
			t.Errorf("events = %#v, want transition to %q", state.Events, status)
		}
	}
	if len(state.ChangeSet.PullRequests) != 1 || state.ChangeSet.PullRequests[0].Repository != "backend" {
		t.Errorf("Change Set = %#v, want touched backend only", state.ChangeSet)
	}
}

func TestImplementationStageAcceptsVerifiedReviewerCorrections(t *testing.T) {
	t.Parallel()

	fixture := newFixture()
	fixture.reviewer.outcome.CorrectiveChanges = true
	fixture.reviewer.outcome.Verification = fixture.verifier.results
	state, err := fixture.service.Run(context.Background(), request())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if state.Status != implementation.StatusCompleted || !state.Review.CorrectiveChanges {
		t.Errorf("state = %#v, want completed corrected review", state)
	}
}

func TestImplementationFailureBlocksSharedStateAndRetryResumesWorkspace(t *testing.T) {
	t.Parallel()

	fixture := newFixture()
	fixture.implementer.err = errors.New("agent interrupted")
	state, err := fixture.service.Run(context.Background(), request())
	if err == nil || state.Status != implementation.StatusFailed {
		t.Fatalf("Run() = %#v, %v; want durable failure", state, err)
	}
	if strings.Join(fixture.calls, ",") != "claim,workspace,implement,block,finalize:failed" {
		t.Errorf("failure calls = %#v", fixture.calls)
	}

	fixture.implementer.err = nil
	state, err = fixture.service.Retry(context.Background(), request().AttemptID)
	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}
	if state.Status != implementation.StatusCompleted {
		t.Errorf("retry state = %#v, want completed", state)
	}
	calls := strings.Join(fixture.calls, ",")
	if strings.Count(calls, "workspace") != 1 || !strings.Contains(calls, "retry,implement,review,verify,deliver,complete") {
		t.Errorf("retry calls = %q, want preserved workspace and resumed implementation", calls)
	}
}

func TestImplementationResumeDoesNotRepeatCommittedSteps(t *testing.T) {
	t.Parallel()

	fixture := newFixture()
	initial := implementation.State{
		AttemptID: "attempt-1", LaborID: "labor-1", Issue: request().Issue, PRD: "# PRD",
		Status: implementation.StatusImplemented, Workspace: fixture.workspace.value,
		Implementation: fixture.implementer.result, ChangeSetRepositories: request().ChangeSetRepositories,
	}
	if err := fixture.store.Save(context.Background(), initial); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	state, err := fixture.service.Run(context.Background(), implementation.Request{AttemptID: "attempt-1"})
	if err != nil {
		t.Fatalf("Run(resume) error = %v", err)
	}
	if state.Status != implementation.StatusCompleted || strings.Contains(strings.Join(fixture.calls, ","), "claim") || strings.Contains(strings.Join(fixture.calls, ","), "implement") {
		t.Errorf("resume state/calls = %#v / %#v", state, fixture.calls)
	}
}

func TestImplementationBlocksInvalidEvidenceBeforeReview(t *testing.T) {
	t.Parallel()

	fixture := newFixture()
	fixture.implementer.result.Evidence = fixture.implementer.result.Evidence[:1]
	state, err := fixture.service.Run(context.Background(), request())
	if err == nil || state.Status != implementation.StatusBlocked {
		t.Fatalf("Run() = %#v, %v; want evidence block", state, err)
	}
	if strings.Contains(strings.Join(fixture.calls, ","), "review") {
		t.Errorf("review ran after invalid evidence: %#v", fixture.calls)
	}
}

func request() implementation.Request {
	reference, _ := tracker.ParseReference("https://github.com/acme/backlog/issues/7")
	return implementation.Request{
		AttemptID: "attempt-1", LaborID: "labor-1", Issue: tracker.Issue{Reference: reference, URL: reference.URL(), Title: "Deliver API", Body: "Issue body"},
		PRD: "# PRD",
		ChangeSetRepositories: []changeset.Repository{
			{Name: "backend", GitHub: "acme/backend", Head: "heracles/acme-backlog-7", Base: "main"},
			{Name: "frontend", GitHub: "acme/frontend", Head: "heracles/acme-backlog-7", Base: "main"},
		},
	}
}

type fixture struct {
	calls       []string
	store       *implementation.MemoryStore
	tracker     *fakeTracker
	workspace   *fakeWorkspace
	implementer *fakeImplementer
	reviewer    *fakeReviewer
	verifier    *fakeVerifier
	deliverer   *fakeDeliverer
	service     implementation.Service
}

func newFixture() *fixture {
	fixture := &fixture{store: implementation.NewMemoryStore()}
	fixture.tracker = &fakeTracker{calls: &fixture.calls}
	fixture.workspace = &fakeWorkspace{calls: &fixture.calls, value: workspace.Workspace{
		Root: "/workspace", Branch: "heracles/acme-backlog-7",
		Repositories: []workspace.Worktree{{Name: "backend", Path: "/workspace/backend"}, {Name: "frontend", Path: "/workspace/frontend"}},
	}}
	start := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	fixture.implementer = &fakeImplementer{calls: &fixture.calls, result: implementation.ImplementationResult{
		Changes: "Implemented API", Evidence: []delivery.Evidence{
			{Kind: delivery.RedEvidence, Command: "go test", ExitCode: 1, StartedAt: start, FinishedAt: start.Add(time.Second), ArtifactPath: "red.json"},
			{Kind: delivery.GreenEvidence, Command: "go test", ExitCode: 0, StartedAt: start.Add(time.Minute), FinishedAt: start.Add(time.Minute + time.Second), ArtifactPath: "green.json"},
		},
	}}
	fixture.reviewer = &fakeReviewer{calls: &fixture.calls, outcome: delivery.ReviewOutcome{Status: "completed", Summary: "Looks good"}}
	fixture.verifier = &fakeVerifier{calls: &fixture.calls, results: []delivery.Verification{{Repository: "backend", Command: "go test", Execution: delivery.Execution{ExitCode: 0}}}}
	fixture.deliverer = &fakeDeliverer{calls: &fixture.calls}
	fixture.service = implementation.Service{
		Store: fixture.store, Tracker: fixture.tracker, Workspaces: fixture.workspace, Implementer: fixture.implementer,
		Reviewer: fixture.reviewer, Verifier: fixture.verifier, Deliverer: fixture.deliverer,
	}
	return fixture
}

func eventStatus(events []implementation.Event, status string) bool {
	for _, event := range events {
		if event.Status == status {
			return true
		}
	}
	return false
}

type fakeTracker struct{ calls *[]string }

func (value *fakeTracker) Claim(context.Context, tracker.Reference, string) (tracker.Issue, error) {
	*value.calls = append(*value.calls, "claim")
	return tracker.Issue{}, nil
}
func (value *fakeTracker) Block(context.Context, tracker.Reference, string) error {
	*value.calls = append(*value.calls, "block")
	return nil
}
func (value *fakeTracker) Complete(context.Context, tracker.Reference, string) error {
	*value.calls = append(*value.calls, "complete")
	return nil
}
func (value *fakeTracker) Retry(context.Context, tracker.Reference, string) error {
	*value.calls = append(*value.calls, "retry")
	return nil
}

type fakeWorkspace struct {
	calls *[]string
	value workspace.Workspace
}

func (value *fakeWorkspace) Create(context.Context, workspace.Request) (workspace.Workspace, error) {
	*value.calls = append(*value.calls, "workspace")
	return value.value, nil
}
func (value *fakeWorkspace) Touched(context.Context, workspace.Workspace) ([]string, error) {
	return []string{"backend"}, nil
}
func (value *fakeWorkspace) Finalize(_ context.Context, _ workspace.Workspace, outcome workspace.Outcome) error {
	*value.calls = append(*value.calls, "finalize:"+string(outcome))
	return nil
}

type fakeImplementer struct {
	calls  *[]string
	result implementation.ImplementationResult
	err    error
}

func (value *fakeImplementer) Implement(context.Context, implementation.ImplementContext) (implementation.ImplementationResult, error) {
	*value.calls = append(*value.calls, "implement")
	return value.result, value.err
}

type fakeReviewer struct {
	calls   *[]string
	outcome delivery.ReviewOutcome
}

func (value *fakeReviewer) Review(context.Context, workspace.Workspace, delivery.ReviewContext) (delivery.ReviewOutcome, error) {
	*value.calls = append(*value.calls, "review")
	return value.outcome, nil
}

type fakeVerifier struct {
	calls   *[]string
	results []delivery.Verification
}

func (value *fakeVerifier) Verify(context.Context, workspace.Workspace, []string) ([]delivery.Verification, error) {
	*value.calls = append(*value.calls, "verify")
	return value.results, nil
}

type fakeDeliverer struct{ calls *[]string }

func (value *fakeDeliverer) Deliver(_ context.Context, request changeset.Request) (changeset.ChangeSet, error) {
	*value.calls = append(*value.calls, "deliver")
	var pullRequests []changeset.PullRequest
	for _, repository := range request.Repositories {
		if repository.Touched {
			pullRequests = append(pullRequests, changeset.PullRequest{Repository: repository.Name, URL: "https://github.com/" + repository.GitHub + "/pull/1"})
		}
	}
	return changeset.ChangeSet{ID: request.ID, IssueURL: request.IssueURL, Status: changeset.StatusOpen, PullRequests: pullRequests}, nil
}
