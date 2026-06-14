package implementation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/davidtobonm/heracles/internal/implementation"
	"github.com/davidtobonm/heracles/internal/scheduler"
	"github.com/davidtobonm/heracles/internal/tracker"
)

func TestBacklogRunnerRefreshesUntilEmpty(t *testing.T) {
	t.Parallel()

	source := &fakeSource{rounds: [][]tracker.Issue{{backlogIssue(1)}, {backlogIssue(2)}, {}}}
	executor := &backlogExecutor{}
	result, err := (implementation.BacklogRunner{Source: source, Scheduler: scheduler.Scheduler{}, Executor: executor}).Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Exhausted || len(result.Completed) != 2 || len(executor.keys) != 2 {
		t.Errorf("result/executor = %#v / %#v, want exhausted refreshed backlog", result, executor.keys)
	}
}

func TestBacklogRunnerReportsGenuinelyBlockedBacklog(t *testing.T) {
	t.Parallel()

	source := &fakeSource{rounds: [][]tracker.Issue{{}}, remaining: []tracker.Issue{backlogIssue(3)}}
	result, err := (implementation.BacklogRunner{Source: source, Scheduler: scheduler.Scheduler{}, Executor: &backlogExecutor{}}).Run(context.Background())
	if !errors.Is(err, implementation.ErrBacklogBlocked) || result.Exhausted {
		t.Fatalf("Run() = %#v, %v; want blocked backlog", result, err)
	}
}

func TestBacklogRunnerCompletesWithPendingHITLThatBlocksNothing(t *testing.T) {
	t.Parallel()

	hitl := backlogIssue(3)
	hitl.Labels = []string{tracker.LabelHITL}
	source := &fakeSource{rounds: [][]tracker.Issue{{}}, remaining: []tracker.Issue{hitl}}
	result, err := (implementation.BacklogRunner{Source: source, Scheduler: scheduler.Scheduler{}, Executor: &backlogExecutor{}}).Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Exhausted || len(result.PendingHITL) != 1 || result.PendingHITL[0] != hitl.URL {
		t.Errorf("result = %#v, want exhausted with pending HITL", result)
	}
}

func TestBacklogRunnerBlocksWhenHITLIssueBlocksRemainingAFKWork(t *testing.T) {
	t.Parallel()

	hitl := backlogIssue(3)
	hitl.Labels = []string{tracker.LabelHITL}
	blockedAFK := backlogIssue(4)
	source := &fakeSource{rounds: [][]tracker.Issue{{}}, remaining: []tracker.Issue{hitl, blockedAFK}}
	result, err := (implementation.BacklogRunner{Source: source, Scheduler: scheduler.Scheduler{}, Executor: &backlogExecutor{}}).Run(context.Background())
	if !errors.Is(err, implementation.ErrBacklogBlocked) || result.Exhausted {
		t.Fatalf("Run() = %#v, %v; want blocked while AFK work remains", result, err)
	}
}

func TestBacklogRunnerScopesToPRDURL(t *testing.T) {
	t.Parallel()

	const prdURL = "https://github.com/acme/backlog/issues/1"
	scoped := backlogIssue(5)
	scoped.Body = "## Parent PRD\n\n" + prdURL + "\n"
	unscoped := backlogIssue(6)
	source := &fakeSource{rounds: [][]tracker.Issue{{scoped, unscoped}, {}}}
	executor := &backlogExecutor{}
	result, err := (implementation.BacklogRunner{Source: source, Scheduler: scheduler.Scheduler{}, Executor: executor, PRDURL: prdURL}).Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Exhausted || len(executor.keys) != 1 || executor.keys[0] != scoped.URL {
		t.Errorf("result/executor = %#v / %#v, want only the scoped issue attempted", result, executor.keys)
	}
}

func TestBacklogRunnerStopsAtRunLimit(t *testing.T) {
	t.Parallel()

	source := &fakeSource{rounds: [][]tracker.Issue{{backlogIssue(1), backlogIssue(2), backlogIssue(3)}}}
	executor := &backlogExecutor{}
	result, err := (implementation.BacklogRunner{Source: source, Scheduler: scheduler.Scheduler{}, Executor: executor, Limit: 2}).Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Exhausted || len(result.Completed) != 2 || len(executor.keys) != 2 {
		t.Errorf("result/executor = %#v / %#v, want two attempted issues", result, executor.keys)
	}
}

func backlogIssue(number int) tracker.Issue {
	reference, _ := tracker.ParseReference("https://github.com/acme/backlog/issues/" + string(rune('0'+number)))
	return tracker.Issue{Reference: reference, URL: reference.URL(), Body: "## Exclusive Scopes\n- api-" + string(rune('0'+number))}
}

type fakeSource struct {
	rounds    [][]tracker.Issue
	remaining []tracker.Issue
}

func (source *fakeSource) ReadyIssues(context.Context) ([]tracker.Issue, error) {
	round := source.rounds[0]
	source.rounds = source.rounds[1:]
	return round, nil
}
func (source *fakeSource) OpenIssues(context.Context) ([]tracker.Issue, error) {
	return source.remaining, nil
}

type backlogExecutor struct{ keys []string }

func (executor *backlogExecutor) Execute(_ context.Context, candidate scheduler.Candidate) error {
	executor.keys = append(executor.keys, candidate.Key)
	return nil
}
