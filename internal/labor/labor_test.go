package labor_test

import (
	"context"
	"errors"
	"testing"

	"github.com/davidtobonm/heracles/internal/implementation"
	"github.com/davidtobonm/heracles/internal/issuestage"
	"github.com/davidtobonm/heracles/internal/labor"
	"github.com/davidtobonm/heracles/internal/planning"
)

func TestLaborRunsThreeStagesWithDistinctApprovalGates(t *testing.T) {
	t.Parallel()

	fixture := newFixture()
	state, err := fixture.service.Run(context.Background(), request())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if state.Status != labor.StatusAwaitingPlanningApproval || state.PlanningGate.ID == state.IssueGate.ID {
		t.Fatalf("state = %#v, want distinct Planning approval pause", state)
	}

	state, err = fixture.service.DecidePlanning(context.Background(), "labor-1", labor.DecisionApprove, "Scope approved")
	if err != nil {
		t.Fatalf("DecidePlanning() error = %v", err)
	}
	if state.Status != labor.StatusAwaitingIssueApproval || state.PlanningGate.Status != labor.GateApproved || state.IssueGate.Status != labor.GatePending {
		t.Fatalf("state = %#v, want issue-publication approval pause", state)
	}

	state, err = fixture.service.DecideIssues(context.Background(), "labor-1", labor.DecisionApprove, "Breakdown approved")
	if err != nil {
		t.Fatalf("DecideIssues() error = %v", err)
	}
	if state.Status != labor.StatusCompleted || !state.Backlog.Exhausted {
		t.Errorf("state = %#v, want empty-backlog completion", state)
	}
}

func TestLaborRejectionsReturnCorrectStageToRevision(t *testing.T) {
	t.Parallel()

	fixture := newFixture()
	if _, err := fixture.service.Run(context.Background(), request()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	fixture.planning.afterReject = planning.State{ID: "labor-1-planning", Status: planning.StatusAwaitingApproval, PRD: "revised", PRDPath: "/revised.md", Gate: planning.Gate{Status: planning.GatePending}}
	state, err := fixture.service.DecidePlanning(context.Background(), "labor-1", labor.DecisionReject, "Revise scope")
	if err != nil {
		t.Fatalf("DecidePlanning(reject) error = %v", err)
	}
	if state.Status != labor.StatusAwaitingPlanningApproval || fixture.planning.lastDecision != planning.DecisionReject {
		t.Errorf("planning rejection state = %#v", state)
	}

	if _, err := fixture.service.DecidePlanning(context.Background(), "labor-1", labor.DecisionApprove, "Approved"); err != nil {
		t.Fatalf("DecidePlanning(approve) error = %v", err)
	}
	fixture.issues.afterReject = issuestage.State{ID: "labor-1-issues", Status: issuestage.StatusAwaitingApproval, Gate: issuestage.Gate{Status: issuestage.GatePending}}
	state, err = fixture.service.DecideIssues(context.Background(), "labor-1", labor.DecisionReject, "Revise slices")
	if err != nil {
		t.Fatalf("DecideIssues(reject) error = %v", err)
	}
	if state.Status != labor.StatusAwaitingIssueApproval || fixture.issues.lastDecision != issuestage.DecisionReject {
		t.Errorf("issue rejection state = %#v", state)
	}
}

func TestLaborInterruptionResumesWithoutRepeatingCommittedStages(t *testing.T) {
	t.Parallel()

	fixture := newFixture()
	if _, err := fixture.service.Run(context.Background(), request()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if fixture.planning.runs != 1 {
		t.Fatalf("planning runs = %d", fixture.planning.runs)
	}
	resumed, err := fixture.service.Run(context.Background(), labor.Request{ID: "labor-1"})
	if err != nil {
		t.Fatalf("Run(resume at gate) error = %v", err)
	}
	if resumed.Status != labor.StatusAwaitingPlanningApproval || fixture.planning.runs != 1 {
		t.Errorf("resume = %#v, planning runs = %d; want no repeated committed work", resumed, fixture.planning.runs)
	}

	fixture.implementation.err = errors.New("process interrupted")
	if _, err := fixture.service.DecidePlanning(context.Background(), "labor-1", labor.DecisionApprove, "Approved"); err != nil {
		t.Fatalf("DecidePlanning() error = %v", err)
	}
	state, err := fixture.service.DecideIssues(context.Background(), "labor-1", labor.DecisionApprove, "Approved")
	if err == nil || state.Status != labor.StatusBlocked {
		t.Fatalf("DecideIssues() = %#v, %v; want blocked interruption", state, err)
	}
	fixture.implementation.err = nil
	state, err = fixture.service.Resume(context.Background(), "labor-1")
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if state.Status != labor.StatusCompleted || fixture.implementation.runs != 2 {
		t.Errorf("resume state/runs = %#v / %d", state, fixture.implementation.runs)
	}
}

func TestLaborResumesIssuePublicationAfterCommittedApproval(t *testing.T) {
	t.Parallel()

	fixture := newFixture()
	if _, err := fixture.service.Run(context.Background(), request()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if _, err := fixture.service.DecidePlanning(context.Background(), "labor-1", labor.DecisionApprove, "Approved"); err != nil {
		t.Fatalf("DecidePlanning() error = %v", err)
	}
	fixture.issues.publishErr = errors.New("publication interrupted")
	state, err := fixture.service.DecideIssues(context.Background(), "labor-1", labor.DecisionApprove, "Approved")
	if err == nil || state.Status != labor.StatusIssues || state.IssueGate.Status != labor.GateApproved {
		t.Fatalf("DecideIssues() = %#v, %v; want durable committed approval", state, err)
	}
	fixture.issues.publishErr = nil
	state, err = fixture.service.Resume(context.Background(), "labor-1")
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if state.Status != labor.StatusCompleted || fixture.issues.publishes != 1 {
		t.Errorf("resume state/publications = %#v / %d", state, fixture.issues.publishes)
	}
}

func TestLaborCancelIsIrreversibleAndDoesNotTouchOtherStages(t *testing.T) {
	t.Parallel()

	fixture := newFixture()
	if _, err := fixture.service.Run(context.Background(), request()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	state, err := fixture.service.Cancel(context.Background(), "labor-1", "no longer needed")
	if err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if state.Status != labor.StatusCancelled {
		t.Fatalf("state.Status = %q, want cancelled", state.Status)
	}
	if fixture.planning.runs != 1 || fixture.issues.publishes != 0 || fixture.implementation.runs != 0 {
		t.Errorf("Cancel() invoked other stages: planning runs = %d, issue publishes = %d, implementation runs = %d", fixture.planning.runs, fixture.issues.publishes, fixture.implementation.runs)
	}

	if _, err := fixture.service.Cancel(context.Background(), "labor-1", "again"); err == nil {
		t.Error("Cancel() on an already-cancelled Labor succeeded, want an error")
	}
	if _, err := fixture.service.Resume(context.Background(), "labor-1"); err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	resumed, err := fixture.store.Load(context.Background(), "labor-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if resumed.Status != labor.StatusCancelled {
		t.Errorf("status after Resume = %q, want cancellation to remain irreversible", resumed.Status)
	}
}

func request() labor.Request {
	return labor.Request{
		ID: "labor-1", Problem: "Build the product", TrackerRepository: "acme/backlog",
		Repositories: []planning.RepositoryContext{{Name: "app", Path: "/app"}},
	}
}

type fixture struct {
	store          *labor.MemoryStore
	planning       *fakePlanning
	issues         *fakeIssues
	implementation *fakeImplementation
	service        labor.Service
}

func newFixture() *fixture {
	fixture := &fixture{store: labor.NewMemoryStore()}
	fixture.planning = &fakePlanning{
		initial: planning.State{ID: "labor-1-planning", Status: planning.StatusAwaitingApproval, PRD: "# PRD", PRDPath: "/PRD.md", Gate: planning.Gate{Status: planning.GatePending}},
	}
	fixture.issues = &fakeIssues{
		initial:   issuestage.State{ID: "labor-1-issues", Status: issuestage.StatusAwaitingApproval, Gate: issuestage.Gate{Status: issuestage.GatePending}},
		published: issuestage.State{ID: "labor-1-issues", Status: issuestage.StatusPublished, Gate: issuestage.Gate{Status: issuestage.GateApproved}},
	}
	fixture.implementation = &fakeImplementation{result: implementation.BacklogResult{Exhausted: true, Completed: []string{"issue-1"}}}
	fixture.service = labor.Service{Store: fixture.store, Planning: fixture.planning, Issues: fixture.issues, Implementation: fixture.implementation}
	return fixture
}

type fakePlanning struct {
	initial      planning.State
	afterReject  planning.State
	runs         int
	lastDecision string
}

func (stage *fakePlanning) Run(_ context.Context, _ planning.RunRequest) (planning.State, error) {
	stage.runs++
	if stage.afterReject.ID != "" {
		value := stage.afterReject
		stage.afterReject = planning.State{}
		return value, nil
	}
	return stage.initial, nil
}
func (stage *fakePlanning) Decide(_ context.Context, _ string, decision, reason string) (planning.State, error) {
	stage.lastDecision = decision
	if decision == planning.DecisionApprove {
		value := stage.initial
		value.Status = planning.StatusApproved
		value.Gate = planning.Gate{Status: planning.GateApproved, Decision: reason}
		return value, nil
	}
	value := stage.initial
	value.Status = planning.StatusRejected
	value.Gate = planning.Gate{Status: planning.GateRejected, Decision: reason}
	return value, nil
}

type fakeIssues struct {
	initial      issuestage.State
	afterReject  issuestage.State
	published    issuestage.State
	runs         int
	lastDecision string
	publishErr   error
	publishes    int
}

func (stage *fakeIssues) Run(_ context.Context, _ issuestage.RunRequest) (issuestage.State, error) {
	stage.runs++
	if stage.afterReject.ID != "" {
		value := stage.afterReject
		stage.afterReject = issuestage.State{}
		return value, nil
	}
	return stage.initial, nil
}
func (stage *fakeIssues) Decide(_ context.Context, _ string, decision, _ string) (issuestage.State, error) {
	stage.lastDecision = decision
	value := stage.initial
	if decision == issuestage.DecisionApprove {
		value.Status = issuestage.StatusApproved
		value.Gate.Status = issuestage.GateApproved
	} else {
		value.Status = issuestage.StatusRejected
		value.Gate.Status = issuestage.GateRejected
	}
	return value, nil
}
func (stage *fakeIssues) Publish(context.Context, string) (issuestage.State, error) {
	stage.publishes++
	return stage.published, stage.publishErr
}

type fakeImplementation struct {
	result implementation.BacklogResult
	err    error
	runs   int
}

func (stage *fakeImplementation) Run(context.Context) (implementation.BacklogResult, error) {
	stage.runs++
	return stage.result, stage.err
}
