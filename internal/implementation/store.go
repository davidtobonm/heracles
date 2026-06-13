package implementation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileStore persists Implementation Stage attempts under a project's .heracles directory.
type FileStore struct {
	root string
}

// NewFileStore creates a durable Implementation Stage store.
func NewFileStore(projectRoot string) FileStore {
	return FileStore{root: filepath.Join(projectRoot, ".heracles", "implementation")}
}

// Load reads one durable attempt.
func (store FileStore) Load(_ context.Context, id string) (State, error) {
	contents, err := os.ReadFile(filepath.Join(store.root, filepath.Base(id), "state.json"))
	if os.IsNotExist(err) {
		return State{}, ErrNotFound
	}
	if err != nil {
		return State{}, fmt.Errorf("read Implementation Stage attempt: %w", err)
	}
	var state State
	if err := json.Unmarshal(contents, &state); err != nil {
		return State{}, fmt.Errorf("decode Implementation Stage attempt: %w", err)
	}
	return state, nil
}

// Save atomically writes one durable attempt.
func (store FileStore) Save(_ context.Context, state State) error {
	contents, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Implementation Stage attempt: %w", err)
	}
	path := filepath.Join(store.root, filepath.Base(state.AttemptID), "state.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create Implementation Stage directory: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".heracles-*")
	if err != nil {
		return fmt.Errorf("create Implementation Stage temporary file: %w", err)
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err := file.Write(append(contents, '\n')); err != nil {
		_ = file.Close()
		return fmt.Errorf("write Implementation Stage temporary file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close Implementation Stage temporary file: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("commit Implementation Stage attempt: %w", err)
	}
	return nil
}

// MemoryStore is a deterministic in-memory attempt store.
type MemoryStore struct {
	mu     sync.Mutex
	states map[string]State
}

// NewMemoryStore creates an empty in-memory attempt store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{states: make(map[string]State)}
}

// Load reads one attempt.
func (store *MemoryStore) Load(_ context.Context, id string) (State, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	state, exists := store.states[id]
	if !exists {
		return State{}, ErrNotFound
	}
	return state, nil
}

// Save writes one attempt.
func (store *MemoryStore) Save(_ context.Context, state State) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.states[state.AttemptID] = state
	return nil
}
