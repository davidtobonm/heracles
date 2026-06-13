package implementation_test

import (
	"context"
	"testing"

	"github.com/davidtobonm/heracles/internal/changeset"
	"github.com/davidtobonm/heracles/internal/history"
	"github.com/davidtobonm/heracles/internal/implementation"
)

func TestHistoryStoreMirrorsAttemptTransitionsAndChangeSet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	executionHistory, err := history.Open(ctx, root)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = executionHistory.Close() })
	store := implementation.NewHistoryStore(root, executionHistory)
	state := implementation.State{
		AttemptID: "attempt-1", LaborID: "labor-1", Issue: request().Issue, Status: implementation.StatusNew,
	}
	if err := store.Save(ctx, state); err != nil {
		t.Fatalf("Save(new) error = %v", err)
	}
	state.Status = implementation.StatusDelivered
	state.ChangeSet = changeset.ChangeSet{ID: "change-1", Status: changeset.StatusOpen}
	if err := store.Save(ctx, state); err != nil {
		t.Fatalf("Save(delivered) error = %v", err)
	}

	snapshot, err := executionHistory.Snapshot(ctx, "labor-1")
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if len(snapshot.IssueAttempts) != 1 || snapshot.IssueAttempts[0].Status != implementation.StatusDelivered {
		t.Errorf("attempts = %#v, want mirrored delivered attempt", snapshot.IssueAttempts)
	}
	if len(snapshot.ChangeSets) != 1 || snapshot.ChangeSets[0].ID != "change-1" {
		t.Errorf("Change Sets = %#v, want mirrored Change Set", snapshot.ChangeSets)
	}
	if len(snapshot.Events) < 4 {
		t.Errorf("events = %#v, want durable transition events", snapshot.Events)
	}
}
