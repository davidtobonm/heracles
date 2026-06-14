package labor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/davidtobonm/heracles/internal/buildinfo"
	"github.com/davidtobonm/heracles/internal/state"
)

// FileStore persists Labors under a project's .heracles directory.
type FileStore struct{ root string }

// NewFileStore creates a durable Labor store.
func NewFileStore(projectRoot string) FileStore {
	return FileStore{root: filepath.Join(projectRoot, ".heracles", "labors")}
}

// Load migrates and reads one durable Labor, per ADR 0030.
func (store FileStore) Load(_ context.Context, id string) (State, error) {
	path := filepath.Join(store.root, filepath.Base(id), "state.json")
	if _, err := os.Stat(path); err == nil {
		if _, err := state.MigrateFile(path); err != nil {
			return State{}, err
		}
	} else if !os.IsNotExist(err) {
		return State{}, fmt.Errorf("inspect Labor: %w", err)
	}
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return State{}, ErrNotFound
	}
	if err != nil {
		return State{}, fmt.Errorf("read Labor: %w", err)
	}
	var laborState State
	if err := json.Unmarshal(contents, &laborState); err != nil {
		return State{}, fmt.Errorf("decode Labor: %w", err)
	}
	return laborState, nil
}

// Save atomically writes one durable Labor, recording the Heracles version
// that created and last ran it, per ADR 0030.
func (store FileStore) Save(_ context.Context, laborState State) error {
	if laborState.HeraclesVersion == "" {
		laborState.HeraclesVersion = buildinfo.Version()
	}
	laborState.UpdatedByVersion = buildinfo.Version()
	laborState.SchemaVersion = state.CurrentSchemaVersion
	contents, err := json.MarshalIndent(laborState, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Labor: %w", err)
	}
	path := filepath.Join(store.root, filepath.Base(laborState.ID), "state.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create Labor directory: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".heracles-*")
	if err != nil {
		return fmt.Errorf("create Labor temporary file: %w", err)
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err := file.Write(append(contents, '\n')); err != nil {
		_ = file.Close()
		return fmt.Errorf("write Labor temporary file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close Labor temporary file: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("commit Labor: %w", err)
	}
	return nil
}

// MemoryStore is a deterministic in-memory Labor store.
type MemoryStore struct {
	mu     sync.Mutex
	states map[string]State
}

// NewMemoryStore creates an empty in-memory Labor store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{states: make(map[string]State)}
}

// Load reads one Labor.
func (store *MemoryStore) Load(_ context.Context, id string) (State, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	state, exists := store.states[id]
	if !exists {
		return State{}, ErrNotFound
	}
	return state, nil
}

// Save writes one Labor.
func (store *MemoryStore) Save(_ context.Context, state State) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.states[state.ID] = state
	return nil
}
