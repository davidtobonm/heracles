package planning_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/planning"
)

type recordingInteractiveRunner struct {
	prompts []string
	err     error
}

func (runner *recordingInteractiveRunner) RunInteractive(_ context.Context, _ agent.Profile, workspaces []string, prompt string) error {
	if len(workspaces) == 0 {
		return errors.New("RunInteractive() called without workspaces")
	}
	runner.prompts = append(runner.prompts, prompt)
	return runner.err
}

type recordingIssueGenerator struct {
	calls []struct{ id, prdPath string }
	err   error
}

func (generator *recordingIssueGenerator) Generate(_ context.Context, id, prdPath string) error {
	generator.calls = append(generator.calls, struct{ id, prdPath string }{id, prdPath})
	return generator.err
}

func TestSessionRunLaunchesInteractiveGrillingSession(t *testing.T) {
	t.Parallel()

	store := planning.NewMemoryStore()
	runner := &recordingInteractiveRunner{}
	service := planning.SessionService{Runner: runner, Store: store}

	state, err := service.Run(context.Background(), planning.SessionRequest{
		ID: "session-1", Problem: "Build a water tracking app",
		Repositories: []planning.RepositoryContext{{Name: "app", Path: "/work/app"}},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if state.Status != planning.StatusActive {
		t.Errorf("status = %q, want %q", state.Status, planning.StatusActive)
	}
	if state.QuestionBudget != planning.DefaultQuestionBudget {
		t.Errorf("question budget = %d, want default %d", state.QuestionBudget, planning.DefaultQuestionBudget)
	}
	if len(runner.prompts) != 1 {
		t.Fatalf("RunInteractive() calls = %d, want 1", len(runner.prompts))
	}
	for _, want := range []string{"session-1", "Build a water tracking app", "grill-with-docs", "to-prd"} {
		if !strings.Contains(runner.prompts[0], want) {
			t.Errorf("brief missing %q\n%s", want, runner.prompts[0])
		}
	}
}

func TestSessionRunRequiresProblemForNewSession(t *testing.T) {
	t.Parallel()

	service := planning.SessionService{Runner: &recordingInteractiveRunner{}, Store: planning.NewMemoryStore()}
	if _, err := service.Run(context.Background(), planning.SessionRequest{ID: "session-1"}); err == nil {
		t.Fatal("Run() error = nil, want error for missing problem")
	}
}

func TestSessionRunRequiresAtLeastOneRepository(t *testing.T) {
	t.Parallel()

	service := planning.SessionService{Runner: &recordingInteractiveRunner{}, Store: planning.NewMemoryStore()}
	if _, err := service.Run(context.Background(), planning.SessionRequest{ID: "session-1", Problem: "Build it"}); err == nil {
		t.Fatal("Run() error = nil, want error for missing Target Repository")
	}
}

func TestSessionRunDoesNotRelaunchApprovedSessions(t *testing.T) {
	t.Parallel()

	store := planning.NewMemoryStore()
	if err := store.SaveSession(context.Background(), planning.SessionState{
		ID: "session-1", Status: planning.StatusApproved, PRDIssueURL: "https://github.com/acme/backlog/issues/9",
		Gate: planning.Gate{Status: planning.GateApproved},
	}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	runner := &recordingInteractiveRunner{}
	service := planning.SessionService{Runner: runner, Store: store}

	state, err := service.Run(context.Background(), planning.SessionRequest{ID: "session-1"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if state.Status != planning.StatusApproved {
		t.Errorf("status = %q, want %q", state.Status, planning.StatusApproved)
	}
	if len(runner.prompts) != 0 {
		t.Errorf("RunInteractive() calls = %d, want 0 for an approved session", len(runner.prompts))
	}
}

func TestRecordPRDIssueTransitionsToAwaitingApproval(t *testing.T) {
	t.Parallel()

	store := planning.NewMemoryStore()
	if err := store.SaveSession(context.Background(), planning.SessionState{
		ID: "session-1", Status: planning.StatusActive, Gate: planning.Gate{Status: planning.GatePending},
	}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	service := planning.SessionService{Runner: &recordingInteractiveRunner{}, Store: store}

	state, err := service.RecordPRDIssue(context.Background(), "session-1", "https://github.com/acme/backlog/issues/9", ".heracles/planning/session-1/PRD.md")
	if err != nil {
		t.Fatalf("RecordPRDIssue() error = %v", err)
	}
	if state.Status != planning.StatusAwaitingApproval {
		t.Errorf("status = %q, want %q", state.Status, planning.StatusAwaitingApproval)
	}
	if state.PRDIssueURL != "https://github.com/acme/backlog/issues/9" || state.PRDPath == "" {
		t.Errorf("state = %#v, want recorded PRD Issue URL and path", state)
	}
}

func TestRecordPRDIssueRequiresURL(t *testing.T) {
	t.Parallel()

	store := planning.NewMemoryStore()
	if err := store.SaveSession(context.Background(), planning.SessionState{ID: "session-1", Status: planning.StatusActive}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	service := planning.SessionService{Runner: &recordingInteractiveRunner{}, Store: store}
	if _, err := service.RecordPRDIssue(context.Background(), "session-1", "", "PRD.md"); err == nil {
		t.Fatal("RecordPRDIssue() error = nil, want error for missing URL")
	}
}

func TestDecideApprovalLaunchesBackgroundIssueGenerationOnce(t *testing.T) {
	t.Parallel()

	store := planning.NewMemoryStore()
	if err := store.SaveSession(context.Background(), planning.SessionState{
		ID: "session-1", Status: planning.StatusAwaitingApproval, Gate: planning.Gate{Status: planning.GatePending},
		PRDIssueURL: "https://github.com/acme/backlog/issues/9", PRDPath: ".heracles/planning/session-1/PRD.md",
	}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	generator := &recordingIssueGenerator{}
	service := planning.SessionService{Runner: &recordingInteractiveRunner{}, Store: store, Generator: generator}

	state, err := service.Decide(context.Background(), "session-1", planning.DecisionApprove, "looks good")
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if state.Status != planning.StatusApproved || state.Gate.Status != planning.GateApproved || !state.IssuesStarted {
		t.Fatalf("state = %#v, want approved with issue generation started", state)
	}
	if len(generator.calls) != 1 || generator.calls[0].id != "session-1" || generator.calls[0].prdPath != ".heracles/planning/session-1/PRD.md" {
		t.Fatalf("Generate() calls = %#v, want one call for session-1", generator.calls)
	}

	// Re-approving an already-approved session is idempotent and must not
	// relaunch issue generation.
	state, err = service.Decide(context.Background(), "session-1", planning.DecisionApprove, "looks good")
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if state.Status != planning.StatusApproved || len(generator.calls) != 1 {
		t.Fatalf("Decide() second call = %#v, generator calls = %#v, want no relaunch", state, generator.calls)
	}
}

func TestDecideRequiresPublishedPRDIssue(t *testing.T) {
	t.Parallel()

	store := planning.NewMemoryStore()
	if err := store.SaveSession(context.Background(), planning.SessionState{ID: "session-1", Status: planning.StatusActive}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	service := planning.SessionService{Runner: &recordingInteractiveRunner{}, Store: store}
	if _, err := service.Decide(context.Background(), "session-1", planning.DecisionApprove, ""); err == nil {
		t.Fatal("Decide() error = nil, want error for an unpublished PRD Issue")
	}
}

func TestDecideRejectsRevisionRequest(t *testing.T) {
	t.Parallel()

	store := planning.NewMemoryStore()
	if err := store.SaveSession(context.Background(), planning.SessionState{
		ID: "session-1", Status: planning.StatusAwaitingApproval, Gate: planning.Gate{Status: planning.GatePending},
		PRDIssueURL: "https://github.com/acme/backlog/issues/9",
	}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	generator := &recordingIssueGenerator{}
	service := planning.SessionService{Runner: &recordingInteractiveRunner{}, Store: store, Generator: generator}

	state, err := service.Decide(context.Background(), "session-1", planning.DecisionReject, "needs more detail")
	if err != nil {
		t.Fatalf("Decide() error = %v", err)
	}
	if state.Status != planning.StatusRejected || state.Gate.Status != planning.GateRejected {
		t.Fatalf("state = %#v, want rejected", state)
	}
	if len(generator.calls) != 0 {
		t.Errorf("Generate() calls = %#v, want none for a rejected session", generator.calls)
	}
}
