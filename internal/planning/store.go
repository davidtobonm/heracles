package planning

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileStore persists Planning Stages under a project's .heracles directory.
type FileStore struct {
	root string
}

// NewFileStore creates a durable Planning Stage store.
func NewFileStore(projectRoot string) FileStore {
	return FileStore{root: filepath.Join(projectRoot, ".heracles", "planning")}
}

// Load reads one durable Planning Stage.
func (store FileStore) Load(_ context.Context, id string) (State, error) {
	contents, err := os.ReadFile(filepath.Join(store.root, safeID(id), "state.json"))
	if os.IsNotExist(err) {
		return State{}, ErrNotFound
	}
	if err != nil {
		return State{}, fmt.Errorf("read Planning Stage: %w", err)
	}
	var state State
	if err := json.Unmarshal(contents, &state); err != nil {
		return State{}, fmt.Errorf("decode Planning Stage: %w", err)
	}
	return state, nil
}

// Save atomically writes one durable Planning Stage.
func (store FileStore) Save(_ context.Context, state State) error {
	contents, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Planning Stage: %w", err)
	}
	return atomicWrite(filepath.Join(store.root, safeID(state.ID), "state.json"), append(contents, '\n'))
}

// WritePRD atomically writes the stable PRD artifact for one Planning Stage.
func (store FileStore) WritePRD(_ context.Context, id, prd string) (string, error) {
	path := filepath.Join(store.root, safeID(id), "PRD.md")
	if err := atomicWrite(path, []byte(prd)); err != nil {
		return "", err
	}
	return path, nil
}

func atomicWrite(path string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create Planning Stage directory: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".heracles-*")
	if err != nil {
		return fmt.Errorf("create Planning Stage temporary file: %w", err)
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err := file.Write(contents); err != nil {
		_ = file.Close()
		return fmt.Errorf("write Planning Stage temporary file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close Planning Stage temporary file: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("commit Planning Stage file: %w", err)
	}
	return nil
}

func safeID(id string) string {
	return filepath.Base(id)
}

// MemoryStore is a deterministic in-memory Planning Stage store for tests and embedding.
type MemoryStore struct {
	mu        sync.Mutex
	states    map[string]State
	artifacts map[string]string
}

// NewMemoryStore creates an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{states: make(map[string]State), artifacts: make(map[string]string)}
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

// WritePRD writes one in-memory PRD.
func (store *MemoryStore) WritePRD(_ context.Context, id, prd string) (string, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	path := filepath.Join(".heracles", "planning", safeID(id), "PRD.md")
	store.artifacts[path] = prd
	return path, nil
}

// Artifact returns an in-memory artifact.
func (store *MemoryStore) Artifact(path string) string {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.artifacts[path]
}
