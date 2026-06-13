package history_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/history"
	_ "modernc.org/sqlite"
)

func TestLaborTransitionPersistsAcrossReopenAndMirrorsJSONL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	projectRoot := t.TempDir()
	store, err := history.Open(ctx, projectRoot)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	_, err = store.CreateLabor(ctx, history.NewLabor{
		ID:      "labor-1",
		Problem: "Deliver the defined backlog",
		Status:  "planning",
	})
	if err != nil {
		t.Fatalf("CreateLabor() error = %v", err)
	}
	if err := store.TransitionLabor(ctx, "labor-1", "planning", "awaiting_prd_approval", "planning.completed", map[string]string{
		"prd": "PRD.md",
	}); err != nil {
		t.Fatalf("TransitionLabor() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	logPath := filepath.Join(projectRoot, ".heracles", "logs", "labor-1.jsonl")
	if err := os.Remove(logPath); err != nil {
		t.Fatalf("remove JSONL mirror to simulate interruption: %v", err)
	}

	reopened, err := history.Open(ctx, projectRoot)
	if err != nil {
		t.Fatalf("Open(reopen) error = %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	labor, err := reopened.Labor(ctx, "labor-1")
	if err != nil {
		t.Fatalf("Labor() error = %v", err)
	}
	if labor.Status != "awaiting_prd_approval" {
		t.Errorf("labor status = %q, want durable transition", labor.Status)
	}

	events, err := reopened.Events(ctx, "labor-1")
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	if len(events) != 2 || events[1].Type != "planning.completed" {
		t.Errorf("events = %#v, want creation and transition events", events)
	}

	logContents, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read JSONL log: %v", err)
	}
	for _, expected := range []string{`"labor_id":"labor-1"`, `"type":"planning.completed"`, `"prd":"PRD.md"`} {
		if !strings.Contains(string(logContents), expected) {
			t.Errorf("JSONL log %q does not contain %q", logContents, expected)
		}
	}
}

func TestOpenRejectsNewerSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	projectRoot := t.TempDir()
	store, err := history.Open(ctx, projectRoot)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	database, err := sql.Open("sqlite", filepath.Join(projectRoot, ".heracles", "history.db"))
	if err != nil {
		t.Fatalf("open raw database: %v", err)
	}
	if _, err := database.Exec(`DELETE FROM schema_migrations; INSERT INTO schema_migrations (version) VALUES (999);`); err != nil {
		t.Fatalf("write future migration: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close raw database: %v", err)
	}

	_, err = history.Open(ctx, projectRoot)
	if err == nil || !strings.Contains(err.Error(), "newer schema") {
		t.Fatalf("Open() error = %v, want newer schema rejection", err)
	}

	database, err = sql.Open("sqlite", filepath.Join(projectRoot, ".heracles", "history.db"))
	if err != nil {
		t.Fatalf("reopen raw database: %v", err)
	}
	defer database.Close()
	var currentVersionRows int
	if err := database.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 1`).Scan(&currentVersionRows); err != nil {
		t.Fatalf("inspect future database: %v", err)
	}
	if currentVersionRows != 0 {
		t.Fatal("opening a newer schema modified it before rejecting it")
	}
}

func TestExecutionHistoryPersistsWorkflowRecordsArtifactsAndConflicts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	projectRoot := t.TempDir()
	store, err := history.Open(ctx, projectRoot)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if _, err := store.CreateLabor(ctx, history.NewLabor{ID: "labor-1", Status: "implementing"}); err != nil {
		t.Fatalf("CreateLabor() error = %v", err)
	}
	if _, err := store.CreateStage(ctx, history.NewStage{ID: "stage-1", LaborID: "labor-1", Kind: "implementation", Status: "active", Ordinal: 3}); err != nil {
		t.Fatalf("CreateStage() error = %v", err)
	}
	if _, err := store.CreateIssueAttempt(ctx, history.NewIssueAttempt{ID: "attempt-1", LaborID: "labor-1", IssueURL: "https://github.com/acme/project/issues/7", Attempt: 2, Status: "reviewing", WorkspacePath: "/tmp/workspace"}); err != nil {
		t.Fatalf("CreateIssueAttempt() error = %v", err)
	}
	if _, err := store.CreateApprovalGate(ctx, history.NewApprovalGate{ID: "gate-1", LaborID: "labor-1", StageID: "stage-1", Kind: "prd", Status: "pending"}); err != nil {
		t.Fatalf("CreateApprovalGate() error = %v", err)
	}
	if _, err := store.CreateChangeSet(ctx, history.NewChangeSet{ID: "change-1", LaborID: "labor-1", IssueAttemptID: "attempt-1", Status: "open"}); err != nil {
		t.Fatalf("CreateChangeSet() error = %v", err)
	}
	artifact, err := store.WriteArtifact(ctx, history.NewArtifact{ID: "evidence-1", LaborID: "labor-1", IssueAttemptID: "attempt-1", Kind: "red", Name: "red.txt", Contents: []byte("expected failure")})
	if err != nil {
		t.Fatalf("WriteArtifact() error = %v", err)
	}
	for name, transition := range map[string]func() error{
		"Stage": func() error {
			return store.TransitionStage(ctx, "stage-1", "active", "completed", "stage.completed", nil)
		},
		"Issue Attempt": func() error {
			return store.TransitionIssueAttempt(ctx, "attempt-1", "reviewing", "completed", "issue_attempt.completed", nil)
		},
		"Approval Gate": func() error {
			return store.TransitionApprovalGate(ctx, "gate-1", "pending", "approved", "approval_gate.approved", nil)
		},
		"Change Set": func() error {
			return store.TransitionChangeSet(ctx, "change-1", "open", "merged", "change_set.merged", nil)
		},
	} {
		if err := transition(); err != nil {
			t.Fatalf("Transition%s() error = %v", name, err)
		}
	}

	if artifact.SHA256 == "" {
		t.Error("artifact SHA256 is empty")
	}
	contents, err := os.ReadFile(filepath.Join(projectRoot, ".heracles", artifact.Path))
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if string(contents) != "expected failure" {
		t.Errorf("artifact contents = %q, want persisted evidence", contents)
	}

	if err := store.TransitionLabor(ctx, "labor-1", "planning", "done", "labor.completed", nil); !errors.Is(err, history.ErrConflict) {
		t.Fatalf("stale TransitionLabor() error = %v, want ErrConflict", err)
	}

	snapshot, err := store.Snapshot(ctx, "labor-1")
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if len(snapshot.Stages) != 1 || len(snapshot.IssueAttempts) != 1 || len(snapshot.ApprovalGates) != 1 || len(snapshot.ChangeSets) != 1 || len(snapshot.Artifacts) != 1 {
		t.Errorf("snapshot = %#v, want all durable workflow records", snapshot)
	}
	if snapshot.Labor.Status != "implementing" {
		t.Errorf("labor status = %q, want stale transition rolled back", snapshot.Labor.Status)
	}
	if snapshot.Stages[0].Status != "completed" || snapshot.IssueAttempts[0].Status != "completed" || snapshot.ApprovalGates[0].Status != "approved" || snapshot.ChangeSets[0].Status != "merged" {
		t.Errorf("snapshot statuses were not transactionally transitioned: %#v", snapshot)
	}
}

func TestResumableLaborsExcludeTerminalHistory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := history.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	for _, labor := range []history.NewLabor{
		{ID: "active", Status: "planning"},
		{ID: "blocked", Status: "blocked"},
		{ID: "completed", Status: "completed"},
		{ID: "cancelled", Status: "cancelled"},
	} {
		if _, err := store.CreateLabor(ctx, labor); err != nil {
			t.Fatalf("CreateLabor(%s) error = %v", labor.ID, err)
		}
	}

	labors, err := store.ResumableLabors(ctx)
	if err != nil {
		t.Fatalf("ResumableLabors() error = %v", err)
	}
	if len(labors) != 2 || labors[0].ID != "active" || labors[1].ID != "blocked" {
		t.Errorf("resumable Labors = %#v, want active and blocked", labors)
	}
}
