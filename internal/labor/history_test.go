package labor_test

import (
	"context"
	"testing"

	"github.com/davidtobonm/heracles/internal/history"
	"github.com/davidtobonm/heracles/internal/labor"
)

func TestLaborHistoryStoreMirrorsStagesAndDistinctApprovalGates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := t.TempDir()
	executionHistory, err := history.Open(ctx, root)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = executionHistory.Close() })

	fixture := newFixture()
	fixture.store = nil
	fixture.service.Store = labor.NewHistoryStore(root, executionHistory)
	state, err := fixture.service.Run(ctx, request())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if _, err := fixture.service.DecidePlanning(ctx, state.ID, labor.DecisionApprove, "approved"); err != nil {
		t.Fatalf("DecidePlanning() error = %v", err)
	}
	if _, err := fixture.service.DecideIssues(ctx, state.ID, labor.DecisionApprove, "approved"); err != nil {
		t.Fatalf("DecideIssues() error = %v", err)
	}

	snapshot, err := executionHistory.Snapshot(ctx, state.ID)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if snapshot.Labor.Status != labor.StatusCompleted || len(snapshot.Stages) != 3 || len(snapshot.ApprovalGates) != 2 {
		t.Errorf("snapshot = %#v, want completed Labor, three stages, and two gates", snapshot)
	}
	if snapshot.ApprovalGates[0].ID == snapshot.ApprovalGates[1].ID {
		t.Errorf("Approval Gates are not distinct: %#v", snapshot.ApprovalGates)
	}
}
