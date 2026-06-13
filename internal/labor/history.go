package labor

import (
	"context"
	"fmt"

	"github.com/davidtobonm/heracles/internal/history"
)

// HistoryStore persists detailed Labor state and mirrors cross-stage transitions into Execution History.
type HistoryStore struct {
	files   FileStore
	history *history.Store
}

// NewHistoryStore creates a Labor store backed by files and Execution History.
func NewHistoryStore(projectRoot string, executionHistory *history.Store) HistoryStore {
	return HistoryStore{files: NewFileStore(projectRoot), history: executionHistory}
}

// Load reads detailed resumable Labor state.
func (store HistoryStore) Load(ctx context.Context, id string) (State, error) {
	return store.files.Load(ctx, id)
}

// Save writes detailed state and catches Execution History up to the latest cross-stage state.
func (store HistoryStore) Save(ctx context.Context, state State) error {
	if store.history == nil {
		return fmt.Errorf("Labor HistoryStore requires Execution History")
	}
	if err := store.files.Save(ctx, state); err != nil {
		return err
	}
	laborRecord, err := store.history.Labor(ctx, state.ID)
	if err != nil {
		if _, err := store.history.CreateLabor(ctx, history.NewLabor{ID: state.ID, Problem: state.Problem, Status: state.Status}); err != nil {
			return err
		}
	} else if laborRecord.Status != state.Status {
		if err := store.history.TransitionLabor(ctx, state.ID, laborRecord.Status, state.Status, "labor."+state.Status, nil); err != nil {
			return err
		}
	}
	snapshot, err := store.history.Snapshot(ctx, state.ID)
	if err != nil {
		return err
	}
	if state.Planning.ID != "" {
		if err := store.syncStage(ctx, snapshot, history.NewStage{
			ID: state.Planning.ID, LaborID: state.ID, Kind: "planning", Status: state.Planning.Status, Ordinal: 1, ArtifactPath: state.Planning.PRDPath,
		}); err != nil {
			return err
		}
		snapshot, _ = store.history.Snapshot(ctx, state.ID)
	}
	if state.Issues.ID != "" {
		if err := store.syncStage(ctx, snapshot, history.NewStage{
			ID: state.Issues.ID, LaborID: state.ID, Kind: "issues", Status: state.Issues.Status, Ordinal: 2,
		}); err != nil {
			return err
		}
		snapshot, _ = store.history.Snapshot(ctx, state.ID)
	}
	if state.Status == StatusImplementing || state.Status == StatusBlocked || state.Status == StatusCompleted {
		if err := store.syncStage(ctx, snapshot, history.NewStage{
			ID: state.ID + "-implementation", LaborID: state.ID, Kind: "implementation", Status: state.Status, Ordinal: 3,
		}); err != nil {
			return err
		}
		snapshot, _ = store.history.Snapshot(ctx, state.ID)
	}
	for _, gate := range []Gate{state.PlanningGate, state.IssueGate} {
		if gate.Status == "" {
			continue
		}
		if err := store.syncGate(ctx, snapshot, state.ID, gate); err != nil {
			return err
		}
		snapshot, _ = store.history.Snapshot(ctx, state.ID)
	}
	return nil
}

func (store HistoryStore) syncStage(ctx context.Context, snapshot history.Snapshot, input history.NewStage) error {
	for _, stage := range snapshot.Stages {
		if stage.ID != input.ID {
			continue
		}
		if stage.Status == input.Status {
			return nil
		}
		return store.history.TransitionStage(ctx, stage.ID, stage.Status, input.Status, "stage."+input.Status, nil)
	}
	_, err := store.history.CreateStage(ctx, input)
	return err
}

func (store HistoryStore) syncGate(ctx context.Context, snapshot history.Snapshot, laborID string, gate Gate) error {
	for _, record := range snapshot.ApprovalGates {
		if record.ID != gate.ID {
			continue
		}
		if record.Status == gate.Status {
			return nil
		}
		return store.history.TransitionApprovalGate(ctx, gate.ID, record.Status, gate.Status, "approval_gate."+gate.Status, map[string]string{"decision": gate.Decision})
	}
	stageID := ""
	if gate.Kind == "planning" {
		stageID = laborID + "-planning"
	} else if gate.Kind == "issue_publication" {
		stageID = laborID + "-issues"
	}
	_, err := store.history.CreateApprovalGate(ctx, history.NewApprovalGate{
		ID: gate.ID, LaborID: laborID, StageID: stageID, Kind: gate.Kind, Status: gate.Status, Decision: gate.Decision,
	})
	return err
}
