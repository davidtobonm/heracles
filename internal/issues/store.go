package issues

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileStore persists issue generation state under a project's .heracles directory.
type FileStore struct {
	root string
}

// NewFileStore creates a durable issue generation store.
func NewFileStore(projectRoot string) FileStore {
	return FileStore{root: filepath.Join(projectRoot, ".heracles", "issues")}
}

// Load reads one durable issue generation state.
func (store FileStore) Load(_ context.Context, id string) (State, error) {
	contents, err := os.ReadFile(filepath.Join(store.root, filepath.Base(id), "state.json"))
	if os.IsNotExist(err) {
		return State{}, ErrNotFound
	}
	if err != nil {
		return State{}, fmt.Errorf("read issue generation state: %w", err)
	}
	var state State
	if err := json.Unmarshal(contents, &state); err != nil {
		return State{}, fmt.Errorf("decode issue generation state: %w", err)
	}
	return state, nil
}

// Save atomically writes one durable issue generation state.
func (store FileStore) Save(_ context.Context, state State) error {
	contents, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode issue generation state: %w", err)
	}
	path := filepath.Join(store.root, filepath.Base(state.ID), "state.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create issue generation directory: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".heracles-*")
	if err != nil {
		return fmt.Errorf("create issue generation temporary file: %w", err)
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err := file.Write(append(contents, '\n')); err != nil {
		_ = file.Close()
		return fmt.Errorf("write issue generation temporary file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close issue generation temporary file: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("commit issue generation state: %w", err)
	}
	return nil
}

// MemoryStore is a deterministic in-memory issue generation store.
type MemoryStore struct {
	mu     sync.Mutex
	states map[string]State
}

// NewMemoryStore creates an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{states: make(map[string]State)}
}

// Load reads one state.
func (store *MemoryStore) Load(_ context.Context, id string) (State, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	state, exists := store.states[id]
	if !exists {
		return State{}, ErrNotFound
	}
	return state, nil
}

// Save writes one state.
func (store *MemoryStore) Save(_ context.Context, state State) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.states[state.ID] = state
	return nil
}
