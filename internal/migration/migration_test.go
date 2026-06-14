package migration_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/davidtobonm/heracles/internal/migration"
)

func writeDocument(t *testing.T, path string, document map[string]any) {
	t.Helper()
	contents, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func readDocument(t *testing.T, path string) map[string]any {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(contents, &document); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	return document
}

func TestMigratorPlanOrdersCompatibleSteps(t *testing.T) {
	t.Parallel()

	migrator := migration.Migrator{
		Current: 3,
		Steps: []migration.Step{
			{From: 1, To: 2, Apply: func(map[string]any) error { return nil }},
			{From: 2, To: 3, Apply: func(map[string]any) error { return nil }},
		},
	}

	plan, err := migrator.Plan(1)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(plan) != 2 || plan[0].From != 1 || plan[1].From != 2 {
		t.Errorf("Plan() = %#v, want ordered steps 1->2->3", plan)
	}
}

func TestMigratorPlanRejectsNewerVersion(t *testing.T) {
	t.Parallel()

	migrator := migration.Migrator{Current: 1}

	_, err := migrator.Plan(2)
	if !errors.Is(err, migration.ErrNewerFormat) {
		t.Fatalf("Plan() error = %v, want ErrNewerFormat", err)
	}
}

func TestApplyFileNoopWhenAlreadyCurrent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "state.json")
	writeDocument(t, path, map[string]any{"schema_version": float64(1), "id": "labor-1"})

	migrator := migration.Migrator{Current: 1}
	result, err := migration.ApplyFile(path, migrator, nil)
	if err != nil {
		t.Fatalf("ApplyFile() error = %v", err)
	}
	if result.Changed {
		t.Errorf("ApplyFile() Changed = true, want false for an up-to-date document")
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Errorf("ApplyFile() created a backup for an up-to-date document")
	}
}

func TestApplyFileAppliesCompatibleMigrationWithBackup(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "state.json")
	original := map[string]any{"id": "labor-1", "problem": "Ship it"}
	writeDocument(t, path, original)

	migrator := migration.Migrator{
		Current: 1,
		Steps: []migration.Step{
			{From: 0, To: 1, Description: "add heracles_version", Apply: func(document map[string]any) error {
				document["heracles_version"] = "1.0.0"
				return nil
			}},
		},
	}

	result, err := migration.ApplyFile(path, migrator, nil)
	if err != nil {
		t.Fatalf("ApplyFile() error = %v", err)
	}
	if !result.Changed || result.FromVersion != 0 || result.ToVersion != 1 {
		t.Errorf("ApplyFile() result = %#v, want changed 0 -> 1", result)
	}

	migrated := readDocument(t, path)
	if migrated["schema_version"] != float64(1) {
		t.Errorf("migrated schema_version = %v, want 1", migrated["schema_version"])
	}
	if migrated["heracles_version"] != "1.0.0" {
		t.Errorf("migrated heracles_version = %v, want 1.0.0", migrated["heracles_version"])
	}

	backup := readDocument(t, path+".bak")
	if _, ok := backup["schema_version"]; ok {
		t.Errorf("backup document = %#v, want the original document without schema_version", backup)
	}
	if backup["problem"] != "Ship it" {
		t.Errorf("backup document = %#v, want the original document preserved", backup)
	}
}

func TestApplyFileRequiresConfirmationForBreakingStep(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "state.json")
	writeDocument(t, path, map[string]any{"id": "labor-1"})

	migrator := migration.Migrator{
		Current: 1,
		Steps: []migration.Step{
			{From: 0, To: 1, Breaking: true, Description: "rename field", Apply: func(map[string]any) error { return nil }},
		},
	}

	if _, err := migration.ApplyFile(path, migrator, nil); !errors.Is(err, migration.ErrConfirmationRequired) {
		t.Fatalf("ApplyFile() error = %v, want ErrConfirmationRequired without confirm", err)
	}
	if _, err := migration.ApplyFile(path, migrator, func([]migration.Step) (bool, error) { return false, nil }); !errors.Is(err, migration.ErrConfirmationRequired) {
		t.Fatalf("ApplyFile() error = %v, want ErrConfirmationRequired when declined", err)
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Errorf("ApplyFile() created a backup despite missing confirmation")
	}

	result, err := migration.ApplyFile(path, migrator, func(plan []migration.Step) (bool, error) {
		if len(plan) != 1 || !plan[0].Breaking {
			t.Errorf("confirm called with plan = %#v, want one breaking step", plan)
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("ApplyFile() error = %v", err)
	}
	if !result.Changed {
		t.Errorf("ApplyFile() Changed = false after confirmation, want true")
	}
}

func TestApplyFileRejectsNewerFormatWithoutMutation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "state.json")
	writeDocument(t, path, map[string]any{"schema_version": float64(99), "id": "labor-1"})
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	migrator := migration.Migrator{Current: 1}
	if _, err := migration.ApplyFile(path, migrator, nil); !errors.Is(err, migration.ErrNewerFormat) {
		t.Fatalf("ApplyFile() error = %v, want ErrNewerFormat", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("ApplyFile() modified a newer-format document")
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Errorf("ApplyFile() created a backup for a newer-format document")
	}
}

func TestApplyFileReportsCorruptDocumentWithoutMutation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "state.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	migrator := migration.Migrator{Current: 1}
	if _, err := migration.ApplyFile(path, migrator, nil); !errors.Is(err, migration.ErrCorrupt) {
		t.Fatalf("ApplyFile() error = %v, want ErrCorrupt", err)
	}

	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Errorf("ApplyFile() created a backup for a corrupt document")
	}
}
