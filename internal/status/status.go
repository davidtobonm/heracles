// Package status provides read-only inspection of current and past local
// Labors, per ADR 0031. Inspecting a Labor reads Execution History and
// Labor state without mutating either.
package status

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/davidtobonm/heracles/internal/history"
	"github.com/davidtobonm/heracles/internal/implementation"
	"github.com/davidtobonm/heracles/internal/labor"
)

// ErrNoLabors indicates that no Labor has been started in this project.
var ErrNoLabors = errors.New("no Labors found")

// Labor reports one Labor's stage, Defined Backlog progress, blockers,
// resumability, and recovery limitations.
type Labor struct {
	ID      string `json:"id"`
	Problem string `json:"problem"`
	// Stage is the Labor's current durable status, e.g. "implementing" or
	// "blocked".
	Stage     string    `json:"stage"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Resumable reports whether `heracles resume` can continue this Labor.
	Resumable bool `json:"resumable"`
	// Blocked reports whether the Labor is stalled awaiting intervention.
	Blocked bool `json:"blocked"`
	// Blockers describes why the Labor is blocked, if Blocked is true.
	Blockers []string `json:"blockers,omitempty"`
	// Guidance is a human-readable suggestion for what to do next.
	Guidance string `json:"guidance,omitempty"`

	// PlanningGate and IssueGate report the Planning and Issue Approval
	// Gates.
	PlanningGate labor.Gate `json:"planning_gate"`
	IssueGate    labor.Gate `json:"issue_gate"`

	// Backlog is the most recent Defined Backlog execution result.
	Backlog implementation.BacklogResult `json:"backlog"`
	// PendingHITL lists open Human-In-The-Loop Issue URLs that remain.
	PendingHITL []string `json:"pending_hitl,omitempty"`
	// ChangeSets lists durable delivery records for this Labor's Issues.
	ChangeSets []history.ChangeSet `json:"change_sets,omitempty"`

	// SchemaVersion, HeraclesVersion, and UpdatedByVersion describe the
	// Labor state's durable schema and the Heracles versions that created
	// and last ran it, per ADR 0030.
	SchemaVersion    int    `json:"schema_version,omitempty"`
	HeraclesVersion  string `json:"heracles_version,omitempty"`
	UpdatedByVersion string `json:"updated_by_version,omitempty"`

	// RecoveryError, if non-empty, explains why this Labor's local state
	// could not be read, per ADR 0028.
	RecoveryError string `json:"recovery_error,omitempty"`
}

// Inspector reports read-only status for current and past local Labors.
type Inspector struct {
	History *history.Store
	Store   labor.Store
}

// List reports status for every Labor, in durable creation order.
func (inspector Inspector) List(ctx context.Context) ([]Labor, error) {
	records, err := inspector.History.Labors(ctx)
	if err != nil {
		return nil, err
	}
	statuses := make([]Labor, len(records))
	for index, record := range records {
		status, err := inspector.build(ctx, record)
		if err != nil {
			return nil, err
		}
		statuses[index] = status
	}
	return statuses, nil
}

// Inspect reports status for one Labor. An empty id defaults to the most
// recently updated Labor.
func (inspector Inspector) Inspect(ctx context.Context, id string) (Labor, error) {
	if id == "" {
		records, err := inspector.History.Labors(ctx)
		if err != nil {
			return Labor{}, err
		}
		if len(records) == 0 {
			return Labor{}, ErrNoLabors
		}
		sort.Slice(records, func(i, j int) bool { return records[i].UpdatedAt.After(records[j].UpdatedAt) })
		return inspector.build(ctx, records[0])
	}
	record, err := inspector.History.Labor(ctx, id)
	if err != nil {
		return Labor{}, err
	}
	return inspector.build(ctx, record)
}

func (inspector Inspector) build(ctx context.Context, record history.Labor) (Labor, error) {
	result := Labor{
		ID:        record.ID,
		Problem:   record.Problem,
		Stage:     record.Status,
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}

	snapshot, err := inspector.History.Snapshot(ctx, record.ID)
	if err != nil {
		return Labor{}, err
	}
	result.ChangeSets = snapshot.ChangeSets

	state, err := inspector.Store.Load(ctx, record.ID)
	if err != nil {
		result.RecoveryError = err.Error()
		result.Guidance = "this Labor's local state is unrecoverable; start a new Labor against the same approved PRD"
		return result, nil
	}
	result.PlanningGate = state.PlanningGate
	result.IssueGate = state.IssueGate
	result.Backlog = state.Backlog
	result.PendingHITL = state.Backlog.PendingHITL
	result.SchemaVersion = state.SchemaVersion
	result.HeraclesVersion = state.HeraclesVersion
	result.UpdatedByVersion = state.UpdatedByVersion

	switch record.Status {
	case labor.StatusCompleted:
		result.Guidance = "Labor complete"
	case labor.StatusCancelled:
		result.Guidance = "Labor cancelled; start a new Labor to continue this work"
	case labor.StatusBlocked:
		result.Blocked = true
		result.Resumable = true
		if len(state.Events) > 0 {
			result.Blockers = []string{state.Events[len(state.Events)-1].Message}
		}
		result.Guidance = "resolve the blocker, then run: heracles resume " + record.ID
	case labor.StatusAwaitingPlanningApproval:
		result.Resumable = true
		result.Guidance = "review and approve or reject the Planning PRD: heracles approve planning " + record.ID + " / heracles reject planning " + record.ID
	case labor.StatusAwaitingIssueApproval:
		result.Resumable = true
		result.Guidance = "review and approve or reject the Issue proposals: heracles approve issues " + record.ID + " / heracles reject issues " + record.ID
	default:
		result.Resumable = true
		result.Guidance = "resume with: heracles resume " + record.ID
	}
	return result, nil
}
