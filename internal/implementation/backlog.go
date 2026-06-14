package implementation

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/davidtobonm/heracles/internal/scheduler"
	"github.com/davidtobonm/heracles/internal/tracker"
)

// ErrBacklogBlocked indicates no Ready Issue can run while open work remains.
var ErrBacklogBlocked = errors.New("Implementation Stage backlog is genuinely blocked")

// BacklogSource exposes fresh shared tracker state.
type BacklogSource interface {
	ReadyIssues(context.Context) ([]tracker.Issue, error)
	OpenIssues(context.Context) ([]tracker.Issue, error)
}

// BacklogRunner repeatedly schedules newly eligible Ready Issues.
type BacklogRunner struct {
	Source    BacklogSource
	Scheduler scheduler.Scheduler
	Executor  scheduler.Executor
	Profile   string
	Limit     int
}

// BacklogResult is the terminal defined-backlog outcome.
type BacklogResult struct {
	Completed []string
	Exhausted bool
}

// Run continues until the defined backlog is empty or genuinely blocked.
func (runner BacklogRunner) Run(ctx context.Context) (BacklogResult, error) {
	if runner.Source == nil || runner.Executor == nil {
		return BacklogResult{}, errors.New("Backlog Runner requires Source and Executor")
	}
	var result BacklogResult
	for {
		ready, err := runner.Source.ReadyIssues(ctx)
		if err != nil {
			return result, err
		}
		if len(ready) == 0 {
			open, err := runner.Source.OpenIssues(ctx)
			if err != nil {
				return result, err
			}
			open = slices.DeleteFunc(open, func(issue tracker.Issue) bool {
				return slices.Contains(issue.Labels, tracker.LabelDone)
			})
			if len(open) == 0 {
				result.Exhausted = true
				return result, nil
			}
			return result, fmt.Errorf("%w: %d open issues remain", ErrBacklogBlocked, len(open))
		}
		if runner.Limit > 0 {
			remaining := runner.Limit - len(result.Completed)
			if remaining <= 0 {
				return result, nil
			}
			if len(ready) > remaining {
				ready = ready[:remaining]
			}
		}
		candidates := make([]scheduler.Candidate, len(ready))
		for index, issue := range ready {
			candidates[index] = scheduler.Candidate{
				Key: issue.URL, Scopes: tracker.ExclusiveScopes(issue.Body), Profile: runner.Profile,
			}
		}
		batch, err := runner.Scheduler.Run(ctx, candidates, runner.Executor)
		result.Completed = append(result.Completed, batch.Completed...)
		if err != nil {
			return result, err
		}
		if runner.Limit > 0 && len(result.Completed) >= runner.Limit {
			return result, nil
		}
	}
}
