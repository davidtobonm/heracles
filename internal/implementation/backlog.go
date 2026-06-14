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
	// PRDURL scopes the backlog to one Parent PRD's issues, per
	// `heracles run <prd-url>`. Empty processes every eligible issue.
	PRDURL string
}

// BacklogResult is the terminal defined-backlog outcome.
type BacklogResult struct {
	Completed []string
	Exhausted bool
	// PendingHITL lists open HITL Issue URLs that remain after the backlog
	// is exhausted but do not block any remaining agent work.
	PendingHITL []string `json:"pending_hitl,omitempty"`
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
		ready = runner.scoped(ready)
		if len(ready) == 0 {
			open, err := runner.Source.OpenIssues(ctx)
			if err != nil {
				return result, err
			}
			open = runner.scoped(open)
			open = slices.DeleteFunc(open, func(issue tracker.Issue) bool {
				return slices.Contains(issue.Labels, tracker.LabelDone)
			})
			if len(open) == 0 {
				result.Exhausted = true
				return result, nil
			}
			var pendingHITL []string
			blocked := false
			for _, issue := range open {
				if slices.Contains(issue.Labels, tracker.LabelHITL) {
					pendingHITL = append(pendingHITL, issue.URL)
				} else {
					blocked = true
				}
			}
			if !blocked {
				result.Exhausted = true
				result.PendingHITL = pendingHITL
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

// scoped filters issues to those linked to PRDURL, when set.
func (runner BacklogRunner) scoped(issues []tracker.Issue) []tracker.Issue {
	if runner.PRDURL == "" {
		return issues
	}
	filtered := make([]tracker.Issue, 0, len(issues))
	for _, issue := range issues {
		if url, ok := tracker.ParentPRDURL(issue.Body); ok && url == runner.PRDURL {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}
