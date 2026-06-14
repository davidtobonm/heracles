package state_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/migration"
	"github.com/davidtobonm/heracles/internal/state"
)

func TestMigrateFileAddsSchemaVersionWithBackup(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "state.json")
	original := map[string]any{"id": "labor-1", "problem": "Ship it", "status": "implementing"}
	contents, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := state.MigrateFile(path)
	if err != nil {
		t.Fatalf("MigrateFile() error = %v", err)
	}
	if !result.Changed || result.ToVersion != state.CurrentSchemaVersion {
		t.Errorf("MigrateFile() result = %#v, want changed to %d", result, state.CurrentSchemaVersion)
	}

	migrated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(migrated, &document); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if document["schema_version"] != float64(state.CurrentSchemaVersion) {
		t.Errorf("schema_version = %v, want %d", document["schema_version"], state.CurrentSchemaVersion)
	}
	if document["heracles_version"] != "unknown" {
		t.Errorf("heracles_version = %v, want unknown", document["heracles_version"])
	}
	if document["problem"] != "Ship it" {
		t.Errorf("migrated document dropped existing fields: %#v", document)
	}

	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Errorf("MigrateFile() did not create a backup: %v", err)
	}
}

func TestMigrateFileIsNoopAtCurrentVersion(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "state.json")
	document := map[string]any{"id": "labor-1", "schema_version": float64(state.CurrentSchemaVersion), "heracles_version": "1.2.3"}
	contents, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := state.MigrateFile(path)
	if err != nil {
		t.Fatalf("MigrateFile() error = %v", err)
	}
	if result.Changed {
		t.Errorf("MigrateFile() Changed = true, want false at current version")
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Errorf("MigrateFile() created a backup at current version")
	}
}

func TestMigrateFileRejectsNewerSchemaVersionWithoutMutation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "state.json")
	document := map[string]any{"id": "labor-1", "schema_version": float64(state.CurrentSchemaVersion + 1)}
	contents, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err = state.MigrateFile(path)
	if !errors.Is(err, migration.ErrNewerFormat) {
		t.Fatalf("MigrateFile() error = %v, want ErrNewerFormat", err)
	}
	if !strings.Contains(err.Error(), "newer Heracles version") {
		t.Errorf("MigrateFile() error = %q, want a message about a newer Heracles version", err.Error())
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(after) != string(contents) {
		t.Errorf("MigrateFile() modified a newer-format document")
	}
}

func TestMigrateFileReportsCorruptStateAsUnrecoverable(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "state.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := state.MigrateFile(path)
	if !errors.Is(err, migration.ErrCorrupt) {
		t.Fatalf("MigrateFile() error = %v, want ErrCorrupt", err)
	}
	if !strings.Contains(err.Error(), "unrecoverable") {
		t.Errorf("MigrateFile() error = %q, want a message reporting the Labor as unrecoverable", err.Error())
	}
}
