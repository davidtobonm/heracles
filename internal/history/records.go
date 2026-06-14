package history

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// NewStage describes a Stage to persist.
type NewStage struct {
	ID           string
	LaborID      string
	Kind         string
	Status       string
	Ordinal      int
	ArtifactPath string
}

// Stage is a durable Planning, Issue, or Implementation Stage.
type Stage struct {
	NewStage
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewIssueAttempt describes an issue attempt to persist.
type NewIssueAttempt struct {
	ID            string
	LaborID       string
	IssueURL      string
	Attempt       int
	Status        string
	WorkspacePath string
}

// IssueAttempt is a durable attempt to deliver one Ready Issue.
type IssueAttempt struct {
	NewIssueAttempt
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewApprovalGate describes an Approval Gate to persist.
type NewApprovalGate struct {
	ID       string
	LaborID  string
	StageID  string
	Kind     string
	Status   string
	Decision string
}

// ApprovalGate is a durable human decision within a Labor.
type ApprovalGate struct {
	NewApprovalGate
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewChangeSet describes a Change Set to persist.
type NewChangeSet struct {
	ID             string
	LaborID        string
	IssueAttemptID string
	Status         string
}

// ChangeSet is the durable linked delivery for one issue.
type ChangeSet struct {
	NewChangeSet
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewArtifact describes a human-readable evidence artifact.
type NewArtifact struct {
	ID             string
	LaborID        string
	IssueAttemptID string
	Kind           string
	Name           string
	Contents       []byte
}

// Artifact is durable metadata for a human-readable artifact.
type Artifact struct {
	ID             string
	LaborID        string
	IssueAttemptID string
	Kind           string
	Path           string
	SHA256         string
	CreatedAt      time.Time
}

// Snapshot contains the complete durable state linked to one Labor.
type Snapshot struct {
	Labor         Labor
	Stages        []Stage
	IssueAttempts []IssueAttempt
	ApprovalGates []ApprovalGate
	ChangeSets    []ChangeSet
	Artifacts     []Artifact
	Events        []Event
}

// CreateStage persists a Stage and linked event.
func (s *Store) CreateStage(ctx context.Context, input NewStage) (Stage, error) {
	if input.ID == "" || input.LaborID == "" || input.Kind == "" || input.Status == "" {
		return Stage{}, errors.New("Stage requires id, labor id, kind, and status")
	}
	now := time.Now().UTC()
	stage := Stage{NewStage: input, CreatedAt: now, UpdatedAt: now}
	event := Event{LaborID: input.LaborID, Type: "stage.created", Payload: mustJSON(map[string]any{"id": input.ID, "kind": input.Kind, "status": input.Status}), CreatedAt: now}
	err := s.record(ctx, event,
		`INSERT INTO stages (id, labor_id, kind, status, ordinal, artifact_path, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		input.ID, input.LaborID, input.Kind, input.Status, input.Ordinal, input.ArtifactPath, timestamp(now), timestamp(now),
	)
	return stage, err
}

// CreateIssueAttempt persists an issue attempt and linked event.
func (s *Store) CreateIssueAttempt(ctx context.Context, input NewIssueAttempt) (IssueAttempt, error) {
	if input.ID == "" || input.LaborID == "" || input.IssueURL == "" || input.Attempt < 1 || input.Status == "" {
		return IssueAttempt{}, errors.New("Issue Attempt requires id, labor id, issue URL, positive attempt, and status")
	}
	now := time.Now().UTC()
	attempt := IssueAttempt{NewIssueAttempt: input, CreatedAt: now, UpdatedAt: now}
	event := Event{LaborID: input.LaborID, IssueAttemptID: input.ID, Type: "issue_attempt.created", Payload: mustJSON(map[string]any{"issue_url": input.IssueURL, "attempt": input.Attempt, "status": input.Status}), CreatedAt: now}
	err := s.record(ctx, event,
		`INSERT INTO issue_attempts (id, labor_id, issue_url, attempt, status, workspace_path, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		input.ID, input.LaborID, input.IssueURL, input.Attempt, input.Status, input.WorkspacePath, timestamp(now), timestamp(now),
	)
	return attempt, err
}

// CreateApprovalGate persists an Approval Gate and linked event.
func (s *Store) CreateApprovalGate(ctx context.Context, input NewApprovalGate) (ApprovalGate, error) {
	if input.ID == "" || input.LaborID == "" || input.Kind == "" || input.Status == "" {
		return ApprovalGate{}, errors.New("Approval Gate requires id, labor id, kind, and status")
	}
	now := time.Now().UTC()
	gate := ApprovalGate{NewApprovalGate: input, CreatedAt: now, UpdatedAt: now}
	event := Event{LaborID: input.LaborID, Type: "approval_gate.created", Payload: mustJSON(map[string]string{"id": input.ID, "kind": input.Kind, "status": input.Status}), CreatedAt: now}
	err := s.record(ctx, event,
		`INSERT INTO approval_gates (id, labor_id, stage_id, kind, status, decision, created_at, updated_at) VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?)`,
		input.ID, input.LaborID, input.StageID, input.Kind, input.Status, input.Decision, timestamp(now), timestamp(now),
	)
	return gate, err
}

// CreateChangeSet persists a Change Set and linked event.
func (s *Store) CreateChangeSet(ctx context.Context, input NewChangeSet) (ChangeSet, error) {
	if input.ID == "" || input.LaborID == "" || input.Status == "" {
		return ChangeSet{}, errors.New("Change Set requires id, labor id, and status")
	}
	now := time.Now().UTC()
	changeSet := ChangeSet{NewChangeSet: input, CreatedAt: now, UpdatedAt: now}
	event := Event{LaborID: input.LaborID, IssueAttemptID: input.IssueAttemptID, Type: "change_set.created", Payload: mustJSON(map[string]string{"id": input.ID, "status": input.Status}), CreatedAt: now}
	err := s.record(ctx, event,
		`INSERT INTO change_sets (id, labor_id, issue_attempt_id, status, created_at, updated_at) VALUES (?, ?, NULLIF(?, ''), ?, ?, ?)`,
		input.ID, input.LaborID, input.IssueAttemptID, input.Status, timestamp(now), timestamp(now),
	)
	return changeSet, err
}

// TransitionStage atomically changes a Stage status.
func (s *Store) TransitionStage(ctx context.Context, id, expected, next, eventType string, payload any) error {
	return s.transitionRecord(ctx, "stages", "Stage", id, expected, next, eventType, payload, "")
}

// TransitionIssueAttempt atomically changes an Issue Attempt status.
func (s *Store) TransitionIssueAttempt(ctx context.Context, id, expected, next, eventType string, payload any) error {
	return s.transitionRecord(ctx, "issue_attempts", "Issue Attempt", id, expected, next, eventType, payload, id)
}

// UpdateIssueAttemptWorkspacePath synchronizes durable workspace metadata without a status transition.
func (s *Store) UpdateIssueAttemptWorkspacePath(ctx context.Context, id, workspacePath string) error {
	if id == "" {
		return errors.New("Issue Attempt workspace sync requires id")
	}
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE issue_attempts SET workspace_path = ?, updated_at = ? WHERE id = ?`,
		workspacePath, timestamp(now), id,
	)
	if err != nil {
		return fmt.Errorf("sync Issue Attempt workspace path: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect Issue Attempt workspace sync: %w", err)
	}
	if changed != 1 {
		return fmt.Errorf("Issue Attempt %q not found", id)
	}
	return nil
}

// TransitionApprovalGate atomically changes an Approval Gate status.
func (s *Store) TransitionApprovalGate(ctx context.Context, id, expected, next, eventType string, payload any) error {
	return s.transitionRecord(ctx, "approval_gates", "Approval Gate", id, expected, next, eventType, payload, "")
}

// TransitionChangeSet atomically changes a Change Set status.
func (s *Store) TransitionChangeSet(ctx context.Context, id, expected, next, eventType string, payload any) error {
	return s.transitionRecord(ctx, "change_sets", "Change Set", id, expected, next, eventType, payload, "")
}

// WriteArtifact writes a human-readable artifact and persists linked metadata.
func (s *Store) WriteArtifact(ctx context.Context, input NewArtifact) (Artifact, error) {
	if input.ID == "" || input.LaborID == "" || input.Kind == "" || input.Name == "" {
		return Artifact{}, errors.New("Artifact requires id, labor id, kind, and name")
	}

	relativePath := filepath.Join("artifacts", safeSegment(input.LaborID), safeSegment(input.IssueAttemptID), safeSegment(input.Kind), safeSegment(input.ID)+"-"+filepath.Base(input.Name))
	absolutePath := filepath.Join(s.root, relativePath)
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
		return Artifact{}, fmt.Errorf("create artifact directory: %w", err)
	}
	file, err := os.OpenFile(absolutePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return Artifact{}, fmt.Errorf("create artifact: %w", err)
	}
	if _, err := file.Write(input.Contents); err != nil {
		_ = file.Close()
		_ = os.Remove(absolutePath)
		return Artifact{}, fmt.Errorf("write artifact: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(absolutePath)
		return Artifact{}, fmt.Errorf("close artifact: %w", err)
	}

	digest := sha256.Sum256(input.Contents)
	now := time.Now().UTC()
	artifact := Artifact{
		ID:             input.ID,
		LaborID:        input.LaborID,
		IssueAttemptID: input.IssueAttemptID,
		Kind:           input.Kind,
		Path:           relativePath,
		SHA256:         hex.EncodeToString(digest[:]),
		CreatedAt:      now,
	}
	event := Event{LaborID: input.LaborID, IssueAttemptID: input.IssueAttemptID, Type: "artifact.written", Payload: mustJSON(map[string]string{"id": input.ID, "kind": input.Kind, "path": relativePath, "sha256": artifact.SHA256}), CreatedAt: now}
	err = s.record(ctx, event,
		`INSERT INTO artifacts (id, labor_id, issue_attempt_id, kind, path, sha256, created_at) VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?)`,
		artifact.ID, artifact.LaborID, artifact.IssueAttemptID, artifact.Kind, artifact.Path, artifact.SHA256, timestamp(now),
	)
	if err != nil {
		_ = os.Remove(absolutePath)
		return Artifact{}, err
	}
	return artifact, nil
}

// Snapshot returns the durable records needed to inspect or resume a Labor.
func (s *Store) Snapshot(ctx context.Context, laborID string) (Snapshot, error) {
	labor, err := s.Labor(ctx, laborID)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{Labor: labor}
	if snapshot.Stages, err = s.stages(ctx, laborID); err != nil {
		return Snapshot{}, err
	}
	if snapshot.IssueAttempts, err = s.issueAttempts(ctx, laborID); err != nil {
		return Snapshot{}, err
	}
	if snapshot.ApprovalGates, err = s.approvalGates(ctx, laborID); err != nil {
		return Snapshot{}, err
	}
	if snapshot.ChangeSets, err = s.changeSets(ctx, laborID); err != nil {
		return Snapshot{}, err
	}
	if snapshot.Artifacts, err = s.artifacts(ctx, laborID); err != nil {
		return Snapshot{}, err
	}
	if snapshot.Events, err = s.Events(ctx, laborID); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

func (s *Store) record(ctx context.Context, event Event, query string, args ...any) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Execution History record: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("persist Execution History record: %w", err)
	}
	event, err = insertEvent(ctx, tx, event)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit Execution History record: %w", err)
	}
	return s.appendJSONL(event)
}

func (s *Store) transitionRecord(ctx context.Context, table, label, id, expected, next, eventType string, payload any, issueAttemptID string) error {
	if id == "" || expected == "" || next == "" || eventType == "" {
		return fmt.Errorf("%s transition requires id, expected state, next state, and event type", label)
	}

	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin %s transition: %w", label, err)
	}
	defer tx.Rollback()

	var laborID string
	if err := tx.QueryRowContext(ctx, `SELECT labor_id FROM `+table+` WHERE id = ?`, id).Scan(&laborID); err != nil {
		return fmt.Errorf("read %s for transition: %w", label, err)
	}
	query := `UPDATE ` + table + ` SET status = ?, updated_at = ?`
	args := []any{next, timestamp(now)}
	if table == "issue_attempts" {
		if workspacePath, ok := transitionWorkspacePath(payload); ok {
			query += `, workspace_path = ?`
			args = append(args, workspacePath)
		}
	}
	query += ` WHERE id = ? AND status = ?`
	args = append(args, id, expected)
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("transition %s: %w", label, err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect %s transition: %w", label, err)
	}
	if changed != 1 {
		return fmt.Errorf("%w: %s %q is not in %q", ErrConflict, label, id, expected)
	}
	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode %s transition payload: %w", label, err)
	}
	event, err := insertEvent(ctx, tx, Event{LaborID: laborID, IssueAttemptID: issueAttemptID, Type: eventType, Payload: encodedPayload, CreatedAt: now})
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit %s transition: %w", label, err)
	}
	return s.appendJSONL(event)
}

func transitionWorkspacePath(payload any) (string, bool) {
	switch value := payload.(type) {
	case map[string]string:
		workspacePath, ok := value["workspace_path"]
		if !ok || workspacePath == "" {
			return "", false
		}
		return workspacePath, true
	case map[string]any:
		workspacePath, ok := value["workspace_path"].(string)
		if !ok || workspacePath == "" {
			return "", false
		}
		return workspacePath, true
	default:
		return "", false
	}
}

func (s *Store) stages(ctx context.Context, laborID string) ([]Stage, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, labor_id, kind, status, ordinal, artifact_path, created_at, updated_at FROM stages WHERE labor_id = ? ORDER BY ordinal, id`, laborID)
	if err != nil {
		return nil, fmt.Errorf("read Stages: %w", err)
	}
	defer rows.Close()
	var values []Stage
	for rows.Next() {
		var value Stage
		var createdAt, updatedAt string
		if err := rows.Scan(&value.ID, &value.LaborID, &value.Kind, &value.Status, &value.Ordinal, &value.ArtifactPath, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan Stage: %w", err)
		}
		value.CreatedAt, _ = parseTimestamp(createdAt)
		value.UpdatedAt, _ = parseTimestamp(updatedAt)
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) issueAttempts(ctx context.Context, laborID string) ([]IssueAttempt, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, labor_id, issue_url, attempt, status, workspace_path, created_at, updated_at FROM issue_attempts WHERE labor_id = ? ORDER BY issue_url, attempt`, laborID)
	if err != nil {
		return nil, fmt.Errorf("read Issue Attempts: %w", err)
	}
	defer rows.Close()
	var values []IssueAttempt
	for rows.Next() {
		var value IssueAttempt
		var createdAt, updatedAt string
		if err := rows.Scan(&value.ID, &value.LaborID, &value.IssueURL, &value.Attempt, &value.Status, &value.WorkspacePath, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan Issue Attempt: %w", err)
		}
		value.CreatedAt, _ = parseTimestamp(createdAt)
		value.UpdatedAt, _ = parseTimestamp(updatedAt)
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) approvalGates(ctx context.Context, laborID string) ([]ApprovalGate, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, labor_id, COALESCE(stage_id, ''), kind, status, decision, created_at, updated_at FROM approval_gates WHERE labor_id = ? ORDER BY created_at, id`, laborID)
	if err != nil {
		return nil, fmt.Errorf("read Approval Gates: %w", err)
	}
	defer rows.Close()
	var values []ApprovalGate
	for rows.Next() {
		var value ApprovalGate
		var createdAt, updatedAt string
		if err := rows.Scan(&value.ID, &value.LaborID, &value.StageID, &value.Kind, &value.Status, &value.Decision, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan Approval Gate: %w", err)
		}
		value.CreatedAt, _ = parseTimestamp(createdAt)
		value.UpdatedAt, _ = parseTimestamp(updatedAt)
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) changeSets(ctx context.Context, laborID string) ([]ChangeSet, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, labor_id, COALESCE(issue_attempt_id, ''), status, created_at, updated_at FROM change_sets WHERE labor_id = ? ORDER BY created_at, id`, laborID)
	if err != nil {
		return nil, fmt.Errorf("read Change Sets: %w", err)
	}
	defer rows.Close()
	var values []ChangeSet
	for rows.Next() {
		var value ChangeSet
		var createdAt, updatedAt string
		if err := rows.Scan(&value.ID, &value.LaborID, &value.IssueAttemptID, &value.Status, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan Change Set: %w", err)
		}
		value.CreatedAt, _ = parseTimestamp(createdAt)
		value.UpdatedAt, _ = parseTimestamp(updatedAt)
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *Store) artifacts(ctx context.Context, laborID string) ([]Artifact, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, labor_id, COALESCE(issue_attempt_id, ''), kind, path, sha256, created_at FROM artifacts WHERE labor_id = ? ORDER BY created_at, id`, laborID)
	if err != nil {
		return nil, fmt.Errorf("read artifacts: %w", err)
	}
	defer rows.Close()
	var values []Artifact
	for rows.Next() {
		var value Artifact
		var createdAt string
		if err := rows.Scan(&value.ID, &value.LaborID, &value.IssueAttemptID, &value.Kind, &value.Path, &value.SHA256, &createdAt); err != nil {
			return nil, fmt.Errorf("scan artifact: %w", err)
		}
		value.CreatedAt, _ = parseTimestamp(createdAt)
		values = append(values, value)
	}
	return values, rows.Err()
}

func safeSegment(value string) string {
	if value == "" {
		return "none"
	}
	return strings.Map(func(character rune) rune {
		switch {
		case character >= 'a' && character <= 'z':
			return character
		case character >= 'A' && character <= 'Z':
			return character
		case character >= '0' && character <= '9':
			return character
		case character == '-', character == '_', character == '.':
			return character
		default:
			return '-'
		}
	}, value)
}
