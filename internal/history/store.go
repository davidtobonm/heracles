// Package history persists Heracles Execution History and mirrors readable artifacts.
package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// ErrConflict indicates that a durable state transition no longer matches current state.
var ErrConflict = errors.New("Execution History state conflict")

// Store is the authoritative local Execution History.
type Store struct {
	db      *sql.DB
	root    string
	logsDir string
	mu      sync.Mutex
}

// NewLabor describes a Labor to persist.
type NewLabor struct {
	ID      string
	Problem string
	Status  string
}

// Labor is a durable end-to-end delivery workflow.
type Labor struct {
	ID        string
	Problem   string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Event is a durable, human-readable Execution History event.
type Event struct {
	ID             int64           `json:"id"`
	LaborID        string          `json:"labor_id"`
	IssueAttemptID string          `json:"issue_attempt_id,omitempty"`
	Type           string          `json:"type"`
	Message        string          `json:"message,omitempty"`
	Payload        json.RawMessage `json:"payload,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

// Open opens or creates the Execution History under a project's .heracles directory.
func Open(ctx context.Context, projectRoot string) (*Store, error) {
	root := filepath.Join(projectRoot, ".heracles")
	logsDir := filepath.Join(root, "logs")
	for _, directory := range []string{root, logsDir, filepath.Join(root, "artifacts")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return nil, fmt.Errorf("create Execution History directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", filepath.Join(root, "history.db"))
	if err != nil {
		return nil, fmt.Errorf("open Execution History database: %w", err)
	}
	db.SetMaxOpenConns(1)

	store := &Store{db: db, root: root, logsDir: logsDir}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.syncAllJSONL(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

// Close closes the underlying SQLite database.
func (s *Store) Close() error {
	return s.db.Close()
}

// CreateLabor transactionally creates a Labor and its first event.
func (s *Store) CreateLabor(ctx context.Context, input NewLabor) (Labor, error) {
	if input.ID == "" || input.Status == "" {
		return Labor{}, errors.New("Labor requires id and status")
	}

	now := time.Now().UTC()
	labor := Labor{
		ID:        input.ID,
		Problem:   input.Problem,
		Status:    input.Status,
		CreatedAt: now,
		UpdatedAt: now,
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Labor{}, fmt.Errorf("begin Labor creation: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO labors (id, problem, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		labor.ID, labor.Problem, labor.Status, timestamp(labor.CreatedAt), timestamp(labor.UpdatedAt),
	); err != nil {
		return Labor{}, fmt.Errorf("create Labor: %w", err)
	}
	event, err := insertEvent(ctx, tx, Event{
		LaborID:   labor.ID,
		Type:      "labor.created",
		Payload:   mustJSON(map[string]string{"status": labor.Status}),
		CreatedAt: now,
	})
	if err != nil {
		return Labor{}, err
	}
	if err := tx.Commit(); err != nil {
		return Labor{}, fmt.Errorf("commit Labor creation: %w", err)
	}
	if err := s.appendJSONL(event); err != nil {
		return Labor{}, err
	}
	return labor, nil
}

// TransitionLabor atomically changes a Labor only when its current state matches expected.
func (s *Store) TransitionLabor(ctx context.Context, id, expected, next, eventType string, payload any) error {
	if id == "" || expected == "" || next == "" || eventType == "" {
		return errors.New("Labor transition requires id, expected state, next state, and event type")
	}

	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Labor transition: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx,
		`UPDATE labors SET status = ?, updated_at = ? WHERE id = ? AND status = ?`,
		next, timestamp(now), id, expected,
	)
	if err != nil {
		return fmt.Errorf("transition Labor: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect Labor transition: %w", err)
	}
	if changed != 1 {
		return fmt.Errorf("%w: Labor %q is not in %q", ErrConflict, id, expected)
	}

	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode Labor transition payload: %w", err)
	}
	event, err := insertEvent(ctx, tx, Event{
		LaborID:   id,
		Type:      eventType,
		Payload:   encodedPayload,
		CreatedAt: now,
	})
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Labor transition: %w", err)
	}
	return s.appendJSONL(event)
}

// Labor returns a durable Labor by ID.
func (s *Store) Labor(ctx context.Context, id string) (Labor, error) {
	var labor Labor
	var createdAt string
	var updatedAt string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, problem, status, created_at, updated_at FROM labors WHERE id = ?`,
		id,
	).Scan(&labor.ID, &labor.Problem, &labor.Status, &createdAt, &updatedAt)
	if err != nil {
		return Labor{}, fmt.Errorf("read Labor %q: %w", id, err)
	}
	labor.CreatedAt, err = parseTimestamp(createdAt)
	if err != nil {
		return Labor{}, err
	}
	labor.UpdatedAt, err = parseTimestamp(updatedAt)
	if err != nil {
		return Labor{}, err
	}
	return labor, nil
}

// ResumableLabors returns all non-terminal Labors in durable creation order.
func (s *Store) ResumableLabors(ctx context.Context) ([]Labor, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, problem, status, created_at, updated_at
		 FROM labors
		 WHERE status NOT IN ('completed', 'cancelled')
		 ORDER BY created_at, id`,
	)
	if err != nil {
		return nil, fmt.Errorf("read resumable Labors: %w", err)
	}
	defer rows.Close()

	var labors []Labor
	for rows.Next() {
		var labor Labor
		var createdAt, updatedAt string
		if err := rows.Scan(&labor.ID, &labor.Problem, &labor.Status, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan resumable Labor: %w", err)
		}
		labor.CreatedAt, err = parseTimestamp(createdAt)
		if err != nil {
			return nil, err
		}
		labor.UpdatedAt, err = parseTimestamp(updatedAt)
		if err != nil {
			return nil, err
		}
		labors = append(labors, labor)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate resumable Labors: %w", err)
	}
	return labors, nil
}

// Events returns a Labor's events in durable order.
func (s *Store) Events(ctx context.Context, laborID string) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, labor_id, COALESCE(issue_attempt_id, ''), type, message, payload_json, created_at
		 FROM events WHERE labor_id = ? ORDER BY id`,
		laborID,
	)
	if err != nil {
		return nil, fmt.Errorf("read Labor events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var event Event
		var payload string
		var createdAt string
		if err := rows.Scan(&event.ID, &event.LaborID, &event.IssueAttemptID, &event.Type, &event.Message, &payload, &createdAt); err != nil {
			return nil, fmt.Errorf("scan Labor event: %w", err)
		}
		event.Payload = json.RawMessage(payload)
		event.CreatedAt, err = parseTimestamp(createdAt)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Labor events: %w", err)
	}
	return events, nil
}

func insertEvent(ctx context.Context, tx *sql.Tx, event Event) (Event, error) {
	result, err := tx.ExecContext(ctx,
		`INSERT INTO events (labor_id, issue_attempt_id, type, message, payload_json, created_at)
		 VALUES (?, NULLIF(?, ''), ?, ?, ?, ?)`,
		event.LaborID, event.IssueAttemptID, event.Type, event.Message, string(event.Payload), timestamp(event.CreatedAt),
	)
	if err != nil {
		return Event{}, fmt.Errorf("record Execution History event: %w", err)
	}
	event.ID, err = result.LastInsertId()
	if err != nil {
		return Event{}, fmt.Errorf("read Execution History event ID: %w", err)
	}
	return event, nil
}

func (s *Store) appendJSONL(event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	encoded, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode JSONL event: %w", err)
	}
	path := filepath.Join(s.logsDir, event.LaborID+".jsonl")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open JSONL log: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("append JSONL log: %w", err)
	}
	return nil
}

func (s *Store) syncAllJSONL(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM labors ORDER BY created_at, id`)
	if err != nil {
		return fmt.Errorf("list Labors for JSONL sync: %w", err)
	}

	var laborIDs []string
	for rows.Next() {
		var laborID string
		if err := rows.Scan(&laborID); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan Labor for JSONL sync: %w", err)
		}
		laborIDs = append(laborIDs, laborID)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close Labor JSONL sync rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate Labors for JSONL sync: %w", err)
	}

	for _, laborID := range laborIDs {
		events, err := s.Events(ctx, laborID)
		if err != nil {
			return err
		}
		if err := s.syncJSONL(laborID, events); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) syncJSONL(laborID string, events []Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.logsDir, laborID+".jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open JSONL log for sync: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			return fmt.Errorf("sync JSONL event: %w", err)
		}
	}
	return nil
}

func mustJSON(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}

func timestamp(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTimestamp(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse Execution History timestamp: %w", err)
	}
	return parsed, nil
}
