// Package scheduler selects dependency-safe concurrent Ready Issues.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
)

// ErrStalled indicates the remaining backlog cannot become eligible.
var ErrStalled = errors.New("Ready Issue backlog is stalled")

// Candidate is one Ready Issue eligible for scheduling policy checks.
type Candidate struct {
	Key          string
	Dependencies []string
	Scopes       []string
	Profile      string
}

// Executor runs one isolated issue attempt.
type Executor interface {
	Execute(context.Context, Candidate) error
}

// Scheduler controls issue concurrency and per-profile capacity.
type Scheduler struct {
	Concurrency   int
	ProfileLimits map[string]int
}

// Result is the exhausted backlog outcome.
type Result struct {
	Completed []string
}

// Run repeatedly selects newly unblocked work until the defined backlog is empty.
func (scheduler Scheduler) Run(ctx context.Context, candidates []Candidate, executor Executor) (Result, error) {
	if executor == nil {
		return Result{}, errors.New("Scheduler requires an Executor")
	}
	if err := validateCandidates(candidates); err != nil {
		return Result{}, err
	}

	pending := append([]Candidate(nil), candidates...)
	completed := make(map[string]bool, len(candidates))
	var result Result
	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		batch := scheduler.Select(pending, completed)
		if len(batch) == 0 {
			keys := make([]string, 0, len(pending))
			for _, candidate := range pending {
				keys = append(keys, candidate.Key)
			}
			return result, fmt.Errorf("%w: %s", ErrStalled, strings.Join(keys, ", "))
		}
		if err := executeBatch(ctx, executor, batch); err != nil {
			return result, err
		}
		for _, candidate := range batch {
			completed[candidate.Key] = true
			result.Completed = append(result.Completed, candidate.Key)
		}
		pending = slices.DeleteFunc(pending, func(candidate Candidate) bool { return completed[candidate.Key] })
	}
	return result, nil
}

// Select chooses the next dependency-safe, scope-safe, capacity-safe batch.
func (scheduler Scheduler) Select(candidates []Candidate, completed map[string]bool) []Candidate {
	limit := scheduler.Concurrency
	if limit < 1 {
		limit = 1
	}

	var selected []Candidate
	scopes := make(map[string]bool)
	profiles := make(map[string]int)
	for _, candidate := range candidates {
		if len(selected) >= limit || !dependenciesComplete(candidate, completed) {
			continue
		}
		if conflicts(candidate.Scopes, scopes) {
			continue
		}
		if profileLimit := scheduler.ProfileLimits[candidate.Profile]; profileLimit > 0 && profiles[candidate.Profile] >= profileLimit {
			continue
		}
		selected = append(selected, candidate)
		profiles[candidate.Profile]++
		for _, scope := range candidate.Scopes {
			scopes[scope] = true
		}
	}
	return selected
}

func executeBatch(ctx context.Context, executor Executor, batch []Candidate) error {
	var wait sync.WaitGroup
	var mu sync.Mutex
	var failures []error
	wait.Add(len(batch))
	for _, candidate := range batch {
		go func() {
			defer wait.Done()
			if err := executor.Execute(ctx, candidate); err != nil {
				mu.Lock()
				failures = append(failures, fmt.Errorf("execute %s: %w", candidate.Key, err))
				mu.Unlock()
			}
		}()
	}
	wait.Wait()
	return errors.Join(failures...)
}

func dependenciesComplete(candidate Candidate, completed map[string]bool) bool {
	for _, dependency := range candidate.Dependencies {
		if !completed[dependency] {
			return false
		}
	}
	return true
}

func conflicts(candidateScopes []string, active map[string]bool) bool {
	for _, scope := range candidateScopes {
		if active[scope] {
			return true
		}
	}
	return false
}

func validateCandidates(candidates []Candidate) error {
	keys := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		if candidate.Key == "" {
			return errors.New("Scheduler candidate requires key")
		}
		if keys[candidate.Key] {
			return fmt.Errorf("duplicate Scheduler candidate %q", candidate.Key)
		}
		keys[candidate.Key] = true
	}
	return nil
}
