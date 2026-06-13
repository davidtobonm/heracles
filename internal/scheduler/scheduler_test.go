package scheduler_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/davidtobonm/heracles/internal/scheduler"
)

func TestSchedulerDefaultsToSequentialAndExhaustsNewlyUnblockedBacklog(t *testing.T) {
	t.Parallel()

	executor := &recordingExecutor{}
	result, err := (scheduler.Scheduler{}).Run(context.Background(), []scheduler.Candidate{
		{Key: "a", Profile: "implementer"},
		{Key: "b", Dependencies: []string{"a"}, Profile: "implementer"},
		{Key: "c", Dependencies: []string{"b"}, Profile: "implementer"},
	}, executor)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if executor.maxActive != 1 {
		t.Errorf("max active = %d, want sequential default", executor.maxActive)
	}
	if len(result.Completed) != 3 {
		t.Errorf("completed = %#v, want exhausted backlog", result.Completed)
	}
}

func TestSchedulerRespectsScopesDependenciesAndProfileCapacityWithoutStarvingWork(t *testing.T) {
	t.Parallel()

	executor := &recordingExecutor{}
	result, err := (scheduler.Scheduler{
		Concurrency:   4,
		ProfileLimits: map[string]int{"implementer": 2},
	}).Run(context.Background(), []scheduler.Candidate{
		{Key: "a", Scopes: []string{"api"}, Profile: "implementer"},
		{Key: "blocked", Dependencies: []string{"a"}, Profile: "implementer"},
		{Key: "scope-conflict", Scopes: []string{"api"}, Profile: "implementer"},
		{Key: "independent", Scopes: []string{"docs"}, Profile: "implementer"},
	}, executor)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if executor.maxActive != 2 {
		t.Errorf("max active = %d, want profile capacity 2", executor.maxActive)
	}
	if len(result.Completed) != 4 {
		t.Errorf("completed = %#v, want all work without starvation", result.Completed)
	}
	if executor.overlapped("a", "scope-conflict") {
		t.Error("overlapping Exclusive Scopes executed concurrently")
	}
	if executor.startedBefore("blocked", "a") {
		t.Error("dependency executed before prerequisite completed")
	}
}

type recordingExecutor struct {
	mu        sync.Mutex
	active    int
	maxActive int
	started   map[string]time.Time
	finished  map[string]time.Time
}

func (executor *recordingExecutor) Execute(_ context.Context, candidate scheduler.Candidate) error {
	executor.mu.Lock()
	if executor.started == nil {
		executor.started = make(map[string]time.Time)
		executor.finished = make(map[string]time.Time)
	}
	executor.active++
	if executor.active > executor.maxActive {
		executor.maxActive = executor.active
	}
	executor.started[candidate.Key] = time.Now()
	executor.mu.Unlock()

	time.Sleep(10 * time.Millisecond)

	executor.mu.Lock()
	executor.finished[candidate.Key] = time.Now()
	executor.active--
	executor.mu.Unlock()
	return nil
}

func (executor *recordingExecutor) overlapped(left, right string) bool {
	executor.mu.Lock()
	defer executor.mu.Unlock()
	return executor.started[left].Before(executor.finished[right]) && executor.started[right].Before(executor.finished[left])
}

func (executor *recordingExecutor) startedBefore(candidate, dependency string) bool {
	executor.mu.Lock()
	defer executor.mu.Unlock()
	return executor.started[candidate].Before(executor.finished[dependency])
}
