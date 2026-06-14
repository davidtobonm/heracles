package labor_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/buildinfo"
	"github.com/davidtobonm/heracles/internal/labor"
	"github.com/davidtobonm/heracles/internal/migration"
	"github.com/davidtobonm/heracles/internal/state"
)

func TestFileStoreSaveRecordsHeraclesVersions(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := labor.NewFileStore(root)

	if err := store.Save(context.Background(), labor.State{ID: "labor-1", Problem: "Ship it", Status: labor.StatusNew}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Load(context.Background(), "labor-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.SchemaVersion != state.CurrentSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", loaded.SchemaVersion, state.CurrentSchemaVersion)
	}
	if loaded.HeraclesVersion != buildinfo.Version() || loaded.UpdatedByVersion != buildinfo.Version() {
		t.Errorf("state = %#v, want both versions set to %s", loaded, buildinfo.Version())
	}

	// Re-saving an existing Labor preserves the original creation version.
	loaded.UpdatedByVersion = "previous-run"
	if err := store.Save(context.Background(), loaded); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	resaved, err := store.Load(context.Background(), "labor-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if resaved.HeraclesVersion != buildinfo.Version() {
		t.Errorf("HeraclesVersion = %q, want creation version preserved (%s)", resaved.HeraclesVersion, buildinfo.Version())
	}
	if resaved.UpdatedByVersion != buildinfo.Version() {
		t.Errorf("UpdatedByVersion = %q, want stamped with the current version (%s)", resaved.UpdatedByVersion, buildinfo.Version())
	}
}

func TestFileStoreLoadMigratesLegacyStateWithBackup(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	storeRoot := filepath.Join(root, ".heracles", "labors", "labor-1")
	if err := os.MkdirAll(storeRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	legacy := map[string]any{"id": "labor-1", "problem": "Ship it", "status": labor.StatusBlocked}
	contents, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	statePath := filepath.Join(storeRoot, "state.json")
	if err := os.WriteFile(statePath, contents, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store := labor.NewFileStore(root)
	loaded, err := store.Load(context.Background(), "labor-1")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Status != labor.StatusBlocked || loaded.Problem != "Ship it" {
		t.Errorf("Load() = %#v, want legacy fields preserved", loaded)
	}
	if loaded.SchemaVersion != state.CurrentSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", loaded.SchemaVersion, state.CurrentSchemaVersion)
	}
	if _, err := os.Stat(statePath + ".bak"); err != nil {
		t.Errorf("Load() did not back up the legacy state file: %v", err)
	}
}

func TestFileStoreLoadRejectsNewerSchemaVersion(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	storeRoot := filepath.Join(root, ".heracles", "labors", "labor-1")
	if err := os.MkdirAll(storeRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	future := map[string]any{"id": "labor-1", "schema_version": state.CurrentSchemaVersion + 1}
	contents, err := json.MarshalIndent(future, "", "  ")
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(storeRoot, "state.json"), contents, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store := labor.NewFileStore(root)
	_, err = store.Load(context.Background(), "labor-1")
	if !errors.Is(err, migration.ErrNewerFormat) {
		t.Fatalf("Load() error = %v, want ErrNewerFormat", err)
	}
	if !strings.Contains(err.Error(), "newer Heracles version") {
		t.Errorf("Load() error = %q, want a message about a newer Heracles version", err.Error())
	}
}
