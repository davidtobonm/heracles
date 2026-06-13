package implementation

import (
	"context"
	"fmt"

	"github.com/davidtobonm/heracles/internal/history"
)

// HistoryStore persists detailed resumable state and mirrors transitions into Execution History.
type HistoryStore struct {
	files   FileStore
	history *history.Store
}

// NewHistoryStore creates an Implementation Stage store backed by files and Execution History.
func NewHistoryStore(projectRoot string, executionHistory *history.Store) HistoryStore {
	return HistoryStore{files: NewFileStore(projectRoot), history: executionHistory}
}

// Load reads detailed resumable state.
func (store HistoryStore) Load(ctx context.Context, id string) (State, error) {
	return store.files.Load(ctx, id)
}

// Save writes detailed state and catches Execution History up to its latest transition.
func (store HistoryStore) Save(ctx context.Context, state State) error {
	if store.history == nil {
		return fmt.Errorf("HistoryStore requires Execution History")
	}
	if err := store.files.Save(ctx, state); err != nil {
		return err
	}
	if err := store.ensureLabor(ctx, state); err != nil {
		return err
	}
	snapshot, err := store.history.Snapshot(ctx, state.LaborID)
	if err != nil {
		return err
	}
	stageID := state.LaborID + "-implementation"
	if !hasStage(snapshot, stageID) {
		if _, err := store.history.CreateStage(ctx, history.NewStage{
			ID: stageID, LaborID: state.LaborID, Kind: "implementation", Status: "active", Ordinal: 3,
		}); err != nil {
			return err
		}
		snapshot, err = store.history.Snapshot(ctx, state.LaborID)
		if err != nil {
			return err
		}
	}
	attempt, exists := issueAttempt(snapshot, state.AttemptID)
	if !exists {
		if _, err := store.history.CreateIssueAttempt(ctx, history.NewIssueAttempt{
			ID: state.AttemptID, LaborID: state.LaborID, IssueURL: state.Issue.URL, Attempt: 1,
			Status: state.Status, WorkspacePath: state.Workspace.Root,
		}); err != nil {
			return err
		}
	} else if attempt.Status != state.Status {
		if err := store.history.TransitionIssueAttempt(ctx, state.AttemptID, attempt.Status, state.Status, "issue_attempt."+state.Status, map[string]string{
			"workspace_path": state.Workspace.Root,
		}); err != nil {
			return err
		}
	}
	if state.ChangeSet.ID != "" {
		snapshot, err = store.history.Snapshot(ctx, state.LaborID)
		if err != nil {
			return err
		}
		changeSet, exists := durableChangeSet(snapshot, state.ChangeSet.ID)
		if !exists {
			_, err = store.history.CreateChangeSet(ctx, history.NewChangeSet{
				ID: state.ChangeSet.ID, LaborID: state.LaborID, IssueAttemptID: state.AttemptID, Status: state.ChangeSet.Status,
			})
			return err
		}
		if changeSet.Status != state.ChangeSet.Status {
			return store.history.TransitionChangeSet(ctx, state.ChangeSet.ID, changeSet.Status, state.ChangeSet.Status, "change_set."+state.ChangeSet.Status, nil)
		}
	}
	return nil
}

func (store HistoryStore) ensureLabor(ctx context.Context, state State) error {
	if _, err := store.history.Labor(ctx, state.LaborID); err == nil {
		return nil
	}
	_, err := store.history.CreateLabor(ctx, history.NewLabor{ID: state.LaborID, Problem: state.Issue.Title, Status: "implementing"})
	return err
}

func hasStage(snapshot history.Snapshot, id string) bool {
	for _, stage := range snapshot.Stages {
		if stage.ID == id {
			return true
		}
	}
	return false
}

func issueAttempt(snapshot history.Snapshot, id string) (history.IssueAttempt, bool) {
	for _, attempt := range snapshot.IssueAttempts {
		if attempt.ID == id {
			return attempt, true
		}
	}
	return history.IssueAttempt{}, false
}

func durableChangeSet(snapshot history.Snapshot, id string) (history.ChangeSet, bool) {
	for _, changeSet := range snapshot.ChangeSets {
		if changeSet.ID == id {
			return changeSet, true
		}
	}
	return history.ChangeSet{}, false
}
