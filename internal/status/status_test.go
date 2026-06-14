package status_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/davidtobonm/heracles/internal/history"
	"github.com/davidtobonm/heracles/internal/implementation"
	"github.com/davidtobonm/heracles/internal/labor"
	"github.com/davidtobonm/heracles/internal/status"
)

func newInspector(t *testing.T) (status.Inspector, *history.Store, string) {
	t.Helper()
	root := t.TempDir()
	executionHistory, err := history.Open(context.Background(), root)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = executionHistory.Close() })
	return status.Inspector{History: executionHistory, Store: labor.NewFileStore(root)}, executionHistory, root
}

func TestInspectorInspectReportsResumeGuidanceForInProgressLabor(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	inspector, executionHistory, root := newInspector(t)

	if _, err := executionHistory.CreateLabor(ctx, history.NewLabor{ID: "labor-1", Problem: "Ship it", Status: labor.StatusImplementing}); err != nil {
		t.Fatalf("CreateLabor() error = %v", err)
	}
	state := labor.State{
		ID: "labor-1", Problem: "Ship it", Status: labor.StatusImplementing,
		PlanningGate: labor.Gate{ID: "labor-1-planning-approval", Kind: "planning", Status: labor.GateApproved},
		IssueGate:    labor.Gate{ID: "labor-1-issue-publication-approval", Kind: "issue_publication", Status: labor.GateApproved},
		Backlog:      implementation.BacklogResult{Completed: []string{"issue-1"}, PendingHITL: []string{"https://github.com/acme/repo/issues/9"}},
	}
	if err := labor.NewFileStore(root).Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	report, err := inspector.Inspect(ctx, "")
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if report.ID != "labor-1" || report.Stage != labor.StatusImplementing {
		t.Errorf("report = %#v, want labor-1 implementing", report)
	}
	if !report.Resumable || report.Blocked {
		t.Errorf("report = %#v, want resumable and not blocked", report)
	}
	if len(report.PendingHITL) != 1 || report.PendingHITL[0] != "https://github.com/acme/repo/issues/9" {
		t.Errorf("PendingHITL = %#v, want pending HITL Issue carried over from Backlog", report.PendingHITL)
	}
	if !strings.Contains(report.Guidance, "heracles resume labor-1") {
		t.Errorf("Guidance = %q, want resume guidance", report.Guidance)
	}
	if report.RecoveryError != "" {
		t.Errorf("RecoveryError = %q, want empty", report.RecoveryError)
	}
}

func TestInspectorReportsBlockedLaborWithBlockerMessage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	inspector, executionHistory, root := newInspector(t)

	if _, err := executionHistory.CreateLabor(ctx, history.NewLabor{ID: "labor-2", Problem: "Ship it", Status: labor.StatusBlocked}); err != nil {
		t.Fatalf("CreateLabor() error = %v", err)
	}
	state := labor.State{
		ID: "labor-2", Problem: "Ship it", Status: labor.StatusBlocked,
		Events: []labor.Event{
			{Status: labor.StatusImplementing, Message: "Labor created", CreatedAt: time.Now()},
			{Status: labor.StatusBlocked, Message: "no Ready Issues and Human-In-The-Loop Issues block remaining work", CreatedAt: time.Now()},
		},
	}
	if err := labor.NewFileStore(root).Save(ctx, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	report, err := inspector.Inspect(ctx, "labor-2")
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if !report.Blocked || !report.Resumable {
		t.Errorf("report = %#v, want blocked and resumable", report)
	}
	if len(report.Blockers) != 1 || !strings.Contains(report.Blockers[0], "Human-In-The-Loop") {
		t.Errorf("Blockers = %#v, want the latest blocking event message", report.Blockers)
	}
	if !strings.Contains(report.Guidance, "heracles resume labor-2") {
		t.Errorf("Guidance = %q, want resume guidance", report.Guidance)
	}
}

func TestInspectorReportsRecoveryErrorForCorruptState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	inspector, executionHistory, root := newInspector(t)

	if _, err := executionHistory.CreateLabor(ctx, history.NewLabor{ID: "labor-3", Problem: "Ship it", Status: labor.StatusImplementing}); err != nil {
		t.Fatalf("CreateLabor() error = %v", err)
	}
	stateDir := filepath.Join(root, ".heracles", "labors", "labor-3")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "state.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	report, err := inspector.Inspect(ctx, "labor-3")
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if report.RecoveryError == "" {
		t.Errorf("RecoveryError = %q, want a non-empty recovery error", report.RecoveryError)
	}
	if report.Resumable {
		t.Errorf("Resumable = true, want false for unrecoverable state")
	}
	if !strings.Contains(report.Guidance, "start a new Labor") {
		t.Errorf("Guidance = %q, want guidance to start a new Labor", report.Guidance)
	}
}

func TestInspectorListReportsAllLabors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	inspector, executionHistory, root := newInspector(t)

	for _, id := range []string{"labor-a", "labor-b"} {
		if _, err := executionHistory.CreateLabor(ctx, history.NewLabor{ID: id, Problem: "Ship it", Status: labor.StatusCompleted}); err != nil {
			t.Fatalf("CreateLabor() error = %v", err)
		}
		if err := labor.NewFileStore(root).Save(ctx, labor.State{ID: id, Problem: "Ship it", Status: labor.StatusCompleted}); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}

	reports, err := inspector.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(reports) != 2 {
		t.Fatalf("List() = %#v, want two Labors", reports)
	}
	for _, report := range reports {
		if report.Stage != labor.StatusCompleted || report.Resumable || report.Guidance != "Labor complete" {
			t.Errorf("report = %#v, want completed and non-resumable", report)
		}
	}
}

func TestInspectorInspectUnknownIDReturnsError(t *testing.T) {
	t.Parallel()

	inspector, _, _ := newInspector(t)
	if _, err := inspector.Inspect(context.Background(), "missing"); err == nil {
		t.Fatal("Inspect() error = nil, want error for unknown Labor")
	}
}

func TestInspectorInspectWithNoLaborsReturnsErrNoLabors(t *testing.T) {
	t.Parallel()

	inspector, _, _ := newInspector(t)
	if _, err := inspector.Inspect(context.Background(), ""); err != status.ErrNoLabors {
		t.Errorf("Inspect() error = %v, want ErrNoLabors", err)
	}
}
