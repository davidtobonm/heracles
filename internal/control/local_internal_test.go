package control

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/history"
	"github.com/davidtobonm/heracles/internal/implementation"
	"github.com/davidtobonm/heracles/internal/planning"
	"github.com/davidtobonm/heracles/internal/tracker"
	"github.com/davidtobonm/heracles/internal/workspace"
)

func TestResumableBacklogSourceIncludesInProgressAttempts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := history.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.CreateLabor(ctx, history.NewLabor{ID: "labor-1", Problem: "Deliver backlog", Status: "implementing"}); err != nil {
		t.Fatalf("CreateLabor() error = %v", err)
	}
	if _, err := store.CreateIssueAttempt(ctx, history.NewIssueAttempt{
		ID: "attempt-1", LaborID: "labor-1", IssueURL: "https://github.com/acme/backlog/issues/7", Attempt: 1, Status: implementation.StatusWorkspaceReady,
	}); err != nil {
		t.Fatalf("CreateIssueAttempt() error = %v", err)
	}

	source := resumableBacklogSource{
		Source: fakeBacklogSource{
			ready: []tracker.Issue{backlogIssue(8, []string{tracker.LabelReady}, time.Unix(20, 0))},
			open: []tracker.Issue{
				backlogIssue(7, []string{tracker.LabelInProgress}, time.Unix(10, 0)),
				backlogIssue(8, []string{tracker.LabelReady}, time.Unix(20, 0)),
			},
		},
		History: store,
		LaborID: "labor-1",
	}

	issues, err := source.ReadyIssues(ctx)
	if err != nil {
		t.Fatalf("ReadyIssues() error = %v", err)
	}
	if len(issues) != 2 || issues[0].Number != 7 || issues[1].Number != 8 {
		t.Fatalf("issues = %#v, want resumable in-progress issue before ready issue", issues)
	}
}

func TestResumableBacklogSourceRejectsOrphanedInProgressIssue(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := history.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	source := resumableBacklogSource{
		Source: fakeBacklogSource{
			open: []tracker.Issue{backlogIssue(7, []string{tracker.LabelInProgress}, time.Unix(10, 0))},
		},
		History: store,
		LaborID: "labor-1",
	}

	_, err = source.ReadyIssues(ctx)
	if err == nil || !strings.Contains(err.Error(), "no local Labor state exists") {
		t.Fatalf("ReadyIssues() error = %v, want orphaned in-progress issue guidance", err)
	}
}

type fakeBacklogSource struct {
	ready []tracker.Issue
	open  []tracker.Issue
}

func (source fakeBacklogSource) ReadyIssues(context.Context) ([]tracker.Issue, error) {
	return source.ready, nil
}
func (source fakeBacklogSource) OpenIssues(context.Context) ([]tracker.Issue, error) {
	return source.open, nil
}

type fakeInteractiveRunner struct {
	prompts []string
	err     error
}

func (runner *fakeInteractiveRunner) RunInteractive(_ context.Context, _ agent.Profile, _ []string, prompt string) error {
	runner.prompts = append(runner.prompts, prompt)
	return runner.err
}

type fakeIssueGenerator struct {
	calls []struct{ id, prdPath string }
}

func (generator *fakeIssueGenerator) Generate(_ context.Context, id, prdPath string) error {
	generator.calls = append(generator.calls, struct{ id, prdPath string }{id, prdPath})
	return nil
}

func newPlanningLocal(t *testing.T, runner planning.InteractiveRunner, generator planning.IssueGenerator) *Local {
	t.Helper()
	store := planning.NewMemoryStore()
	return &Local{
		root: t.TempDir(),
		workspaces: workspace.Manager{
			Repositories: []workspace.Repository{{Name: "app", Path: "/work/app"}},
		},
		planningSession: planning.SessionService{Runner: runner, Store: store, Generator: generator},
	}
}

func TestExecutePlanLaunchesInteractiveGrillingSession(t *testing.T) {
	t.Parallel()

	runner := &fakeInteractiveRunner{}
	local := newPlanningLocal(t, runner, nil)

	result, err := local.Execute(context.Background(), Operation{Name: "plan", ID: "session-1", Problem: "Build a water tracking app"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Status != planning.StatusActive {
		t.Errorf("result.Status = %q, want %q", result.Status, planning.StatusActive)
	}
	if len(runner.prompts) != 1 || !strings.Contains(runner.prompts[0], "session-1") {
		t.Fatalf("RunInteractive() calls = %#v, want one Grilling Session brief for session-1", runner.prompts)
	}
}

func TestExecutePlanRecordsPRDIssueURL(t *testing.T) {
	t.Parallel()

	runner := &fakeInteractiveRunner{}
	local := newPlanningLocal(t, runner, nil)

	if _, err := local.Execute(context.Background(), Operation{Name: "plan", ID: "session-1", Problem: "Build a water tracking app"}); err != nil {
		t.Fatalf("Execute(plan) error = %v", err)
	}

	result, err := local.Execute(context.Background(), Operation{
		Name: "plan", ID: "session-1", PRDIssueURL: "https://github.com/acme/backlog/issues/9", PRD: ".heracles/planning/session-1/PRD.md",
	})
	if err != nil {
		t.Fatalf("Execute(plan --prd-issue) error = %v", err)
	}
	if result.Status != planning.StatusAwaitingApproval {
		t.Errorf("result.Status = %q, want %q", result.Status, planning.StatusAwaitingApproval)
	}
	state, ok := result.Data.(planning.SessionState)
	if !ok || state.PRDIssueURL != "https://github.com/acme/backlog/issues/9" {
		t.Fatalf("result.Data = %#v, want recorded PRD Issue URL", result.Data)
	}
}

func TestExecuteApprovePlanningSessionLaunchesBackgroundIssueGeneration(t *testing.T) {
	t.Parallel()

	runner := &fakeInteractiveRunner{}
	generator := &fakeIssueGenerator{}
	local := newPlanningLocal(t, runner, generator)

	if _, err := local.Execute(context.Background(), Operation{Name: "plan", ID: "session-1", Problem: "Build a water tracking app"}); err != nil {
		t.Fatalf("Execute(plan) error = %v", err)
	}
	if _, err := local.Execute(context.Background(), Operation{
		Name: "plan", ID: "session-1", PRDIssueURL: "https://github.com/acme/backlog/issues/9", PRD: ".heracles/planning/session-1/PRD.md",
	}); err != nil {
		t.Fatalf("Execute(plan --prd-issue) error = %v", err)
	}

	result, err := local.Execute(context.Background(), Operation{Name: "approve", Kind: "planning", ID: "session-1"})
	if err != nil {
		t.Fatalf("Execute(approve planning) error = %v", err)
	}
	if result.Status != planning.StatusApproved {
		t.Errorf("result.Status = %q, want %q", result.Status, planning.StatusApproved)
	}
	if len(generator.calls) != 1 || generator.calls[0].id != "session-1" {
		t.Fatalf("Generate() calls = %#v, want one call for session-1", generator.calls)
	}
}

func backlogIssue(number int, labels []string, createdAt time.Time) tracker.Issue {
	reference := tracker.Reference{Owner: "acme", Repo: "backlog", Number: number}
	return tracker.Issue{
		Reference: reference,
		URL:       reference.URL(),
		Labels:    labels,
		State:     tracker.StateOpen,
		CreatedAt: createdAt,
	}
}
