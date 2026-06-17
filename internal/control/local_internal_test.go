package control

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/history"
	"github.com/davidtobonm/heracles/internal/implementation"
	"github.com/davidtobonm/heracles/internal/issues"
	"github.com/davidtobonm/heracles/internal/labor"
	"github.com/davidtobonm/heracles/internal/lock"
	"github.com/davidtobonm/heracles/internal/planning"
	"github.com/davidtobonm/heracles/internal/project"
	"github.com/davidtobonm/heracles/internal/status"
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
	calls []struct {
		id, prdIssueURL, prdPath string
		overrides                map[string]project.ProfileConfig
	}
}

func (generator *fakeIssueGenerator) Generate(_ context.Context, id, prdIssueURL, prdPath string, overrides map[string]project.ProfileConfig) error {
	generator.calls = append(generator.calls, struct {
		id, prdIssueURL, prdPath string
		overrides                map[string]project.ProfileConfig
	}{id, prdIssueURL, prdPath, overrides})
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

type fakeIssueTrackerRunner struct {
	output string
	calls  []string
}

func (runner *fakeIssueTrackerRunner) Run(_ context.Context, command string, args ...string) ([]byte, error) {
	runner.calls = append(runner.calls, command+" "+strings.Join(args, " "))
	return []byte(runner.output), nil
}

func TestExecuteIssuesRejectsUnapprovedPRDIssue(t *testing.T) {
	t.Parallel()

	runner := &fakeIssueTrackerRunner{output: `{"number":9,"title":"PRD","body":"# PRD","url":"https://github.com/acme/backlog/issues/9","createdAt":"2026-01-01T00:00:00Z","state":"OPEN","labels":[{"name":"heracles:prd"},{"name":"heracles:review"}]}`}
	local := &Local{root: t.TempDir(), trackerClient: tracker.NewGitHubClient(runner)}

	if _, err := local.Execute(context.Background(), Operation{Name: "issues", PRDIssueURL: "https://github.com/acme/backlog/issues/9"}); err == nil {
		t.Fatal("Execute(issues) error = nil, want error for an unapproved PRD Issue")
	}
}

func TestExecuteIssuesStartsBackgroundGenerationForApprovedPRDIssue(t *testing.T) {
	t.Parallel()

	runner := &fakeIssueTrackerRunner{output: `{"number":9,"title":"PRD","body":"# PRD\n\nBuild it.","url":"https://github.com/acme/backlog/issues/9","createdAt":"2026-01-01T00:00:00Z","state":"OPEN","labels":[{"name":"heracles:prd"},{"name":"heracles:approved"}]}`}
	generator := &fakeIssueGenerator{}
	root := t.TempDir()
	local := &Local{root: root, trackerClient: tracker.NewGitHubClient(runner), issueGenerator: generator}

	result, err := local.Execute(context.Background(), Operation{Name: "issues", PRDIssueURL: "https://github.com/acme/backlog/issues/9"})
	if err != nil {
		t.Fatalf("Execute(issues) error = %v", err)
	}
	if result.Status != issues.StatusStarted {
		t.Errorf("result.Status = %q, want %q", result.Status, issues.StatusStarted)
	}
	if len(generator.calls) != 1 || generator.calls[0].prdIssueURL != "https://github.com/acme/backlog/issues/9" {
		t.Fatalf("Generate() calls = %#v, want one background call for the PRD Issue", generator.calls)
	}
	contents, err := os.ReadFile(generator.calls[0].prdPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(contents), "Build it.") {
		t.Errorf("approved PRD mirror = %q, want approved PRD Issue body", contents)
	}
}

func TestExecuteIssuesRejectsConcurrentGenerationForSameID(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	local := &Local{root: root}

	held, err := lock.Acquire(local.issueGenerationLockPath("prd-acme-app-9"), "prd-acme-app-9", nil)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer func() { _ = held.Release() }()

	_, err = local.Execute(context.Background(), Operation{
		Name: "issues", ID: "prd-acme-app-9", PRD: "# PRD", PRDIssueURL: "https://github.com/acme/app/issues/9",
	})
	if err == nil {
		t.Fatal("Execute(issues) error = nil, want error for a concurrent in-flight generation run")
	}
	if !errors.Is(err, lock.ErrHeld) {
		t.Errorf("Execute(issues) error = %v, want errors.Is(..., lock.ErrHeld)", err)
	}
}

func TestExecuteIssuesReleasesLockAfterGeneration(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	local := &Local{root: root, trackerRepo: "acme/backlog", issuesService: issues.Service{
		Author:  fakeIssueAuthor{},
		Tracker: fakeIssueTracker{},
		Store:   issues.NewFileStore(root),
	}}

	if _, err := local.Execute(context.Background(), Operation{
		Name: "issues", ID: "prd-acme-app-9", PRD: "# PRD", PRDIssueURL: "https://github.com/acme/app/issues/9",
	}); err != nil {
		t.Fatalf("Execute(issues) error = %v", err)
	}

	if _, err := os.Stat(local.issueGenerationLockPath("prd-acme-app-9")); !os.IsNotExist(err) {
		t.Errorf("lock file stat err = %v, want it removed after generation completes", err)
	}
}

type fakeIssueAuthor struct{}

func (fakeIssueAuthor) Propose(context.Context, issues.AuthorRequest) (issues.AuthorResponse, error) {
	return issues.AuthorResponse{Blocked: "missing user stories"}, nil
}

type fakeIssueTracker struct{}

func (fakeIssueTracker) ListOpenIssues(context.Context, string) ([]tracker.Issue, error) {
	return nil, nil
}

func (fakeIssueTracker) CreateIssue(context.Context, string, string, string, []string) (string, error) {
	return "", nil
}

func (fakeIssueTracker) UpdateIssue(context.Context, tracker.Reference, string, string, []string) error {
	return nil
}

func (fakeIssueTracker) SetLabels(context.Context, tracker.Reference, []string) error {
	return nil
}

func (fakeIssueTracker) Comment(context.Context, tracker.Reference, string) error {
	return nil
}

func TestIssueGenerationArgsForwardsRoleOverrides(t *testing.T) {
	t.Parallel()

	args := issueGenerationArgs("prd-acme-app-9", "/path/PRD.md", "https://github.com/acme/app/issues/9", "/path/heracles.yaml", map[string]project.ProfileConfig{
		"issue_author": {Provider: "codex", Model: "gpt-5.4", Effort: "high"},
	})

	want := []string{
		"issues", "--id", "prd-acme-app-9", "--prd", "/path/PRD.md", "--prd-issue", "https://github.com/acme/app/issues/9",
		"--issue_author", "codex", "--issue_author-model", "gpt-5.4", "--issue_author-effort", "high",
		"--config", "/path/heracles.yaml",
	}
	if len(args) != len(want) {
		t.Fatalf("issueGenerationArgs() = %#v, want %#v", args, want)
	}
	for index := range want {
		if args[index] != want[index] {
			t.Fatalf("issueGenerationArgs() = %#v, want %#v", args, want)
		}
	}
}

func TestIssueGenerationArgsOmitsEmptyOverrideFields(t *testing.T) {
	t.Parallel()

	args := issueGenerationArgs("prd-acme-app-9", "/path/PRD.md", "https://github.com/acme/app/issues/9", "", map[string]project.ProfileConfig{
		"issue_author": {Effort: "high"},
	})

	want := []string{
		"issues", "--id", "prd-acme-app-9", "--prd", "/path/PRD.md", "--prd-issue", "https://github.com/acme/app/issues/9",
		"--issue_author-effort", "high",
	}
	if len(args) != len(want) {
		t.Fatalf("issueGenerationArgs() = %#v, want %#v", args, want)
	}
	for index := range want {
		if args[index] != want[index] {
			t.Fatalf("issueGenerationArgs() = %#v, want %#v", args, want)
		}
	}
}

func TestExecuteIssuesForwardsRoleOverridesToBackgroundGenerator(t *testing.T) {
	t.Parallel()

	runner := &fakeIssueTrackerRunner{output: `{"number":9,"title":"PRD","body":"# PRD\n\nBuild it.","url":"https://github.com/acme/backlog/issues/9","createdAt":"2026-01-01T00:00:00Z","state":"OPEN","labels":[{"name":"heracles:prd"},{"name":"heracles:approved"}]}`}
	generator := &fakeIssueGenerator{}
	root := t.TempDir()
	local := &Local{root: root, trackerClient: tracker.NewGitHubClient(runner), issueGenerator: generator}

	overrides := map[string]project.ProfileConfig{
		"issue_author": {Provider: "codex", Model: "gpt-5.4", Effort: "high"},
	}
	if _, err := local.Execute(context.Background(), Operation{
		Name: "issues", PRDIssueURL: "https://github.com/acme/backlog/issues/9", RoleOverrides: overrides,
	}); err != nil {
		t.Fatalf("Execute(issues) error = %v", err)
	}
	if len(generator.calls) != 1 {
		t.Fatalf("Generate() calls = %#v, want one background call", generator.calls)
	}
	got := generator.calls[0].overrides["issue_author"]
	want := overrides["issue_author"]
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Generate() overrides = %#v, want %#v", got, want)
	}
}

func TestExecuteStatusInspectsCurrentLaborWithoutMutation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	executionHistory, err := history.Open(ctx, root)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = executionHistory.Close() })

	if _, err := executionHistory.CreateLabor(ctx, history.NewLabor{ID: "labor-1", Problem: "Build it", Status: labor.StatusImplementing}); err != nil {
		t.Fatalf("CreateLabor() error = %v", err)
	}
	if err := labor.NewFileStore(root).Save(ctx, labor.State{ID: "labor-1", Problem: "Build it", Status: labor.StatusImplementing}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	statePath := filepath.Join(root, ".heracles", "labors", "labor-1", "state.json")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	local := &Local{root: root, history: executionHistory}

	result, err := local.Execute(ctx, Operation{Name: "status"})
	if err != nil {
		t.Fatalf("Execute(status) error = %v", err)
	}
	report, ok := result.Data.(status.Labor)
	if !ok || report.ID != "labor-1" || report.Stage != labor.StatusImplementing {
		t.Fatalf("result.Data = %#v, want status.Labor for labor-1", result.Data)
	}

	result, err = local.Execute(ctx, Operation{Name: "status", ID: "labor-1"})
	if err != nil {
		t.Fatalf("Execute(status, labor-1) error = %v", err)
	}
	report, ok = result.Data.(status.Labor)
	if !ok || report.ID != "labor-1" {
		t.Fatalf("result.Data = %#v, want status.Labor for labor-1", result.Data)
	}

	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("heracles status mutated Labor state:\nbefore = %s\nafter  = %s", before, after)
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
