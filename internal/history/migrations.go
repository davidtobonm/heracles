package history

import (
	"context"
	"fmt"
)

const currentSchemaVersion = 1

const schema = `
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;

CREATE TABLE IF NOT EXISTS schema_migrations (
	version INTEGER PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS labors (
	id TEXT PRIMARY KEY,
	problem TEXT NOT NULL,
	status TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS stages (
	id TEXT PRIMARY KEY,
	labor_id TEXT NOT NULL REFERENCES labors(id),
	kind TEXT NOT NULL,
	status TEXT NOT NULL,
	ordinal INTEGER NOT NULL,
	artifact_path TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS issue_attempts (
	id TEXT PRIMARY KEY,
	labor_id TEXT NOT NULL REFERENCES labors(id),
	issue_url TEXT NOT NULL,
	attempt INTEGER NOT NULL,
	status TEXT NOT NULL,
	workspace_path TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(issue_url, attempt)
);

CREATE TABLE IF NOT EXISTS approval_gates (
	id TEXT PRIMARY KEY,
	labor_id TEXT NOT NULL REFERENCES labors(id),
	stage_id TEXT REFERENCES stages(id),
	kind TEXT NOT NULL,
	status TEXT NOT NULL,
	decision TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS change_sets (
	id TEXT PRIMARY KEY,
	labor_id TEXT NOT NULL REFERENCES labors(id),
	issue_attempt_id TEXT REFERENCES issue_attempts(id),
	status TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS artifacts (
	id TEXT PRIMARY KEY,
	labor_id TEXT NOT NULL REFERENCES labors(id),
	issue_attempt_id TEXT REFERENCES issue_attempts(id),
	kind TEXT NOT NULL,
	path TEXT NOT NULL,
	sha256 TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	labor_id TEXT NOT NULL REFERENCES labors(id),
	issue_attempt_id TEXT REFERENCES issue_attempts(id),
	type TEXT NOT NULL,
	message TEXT NOT NULL DEFAULT '',
	payload_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL
);

INSERT OR IGNORE INTO schema_migrations (version) VALUES (1);
`

func (s *Store) migrate(ctx context.Context) error {
	version, exists, err := s.existingSchemaVersion(ctx)
	if err != nil {
		return err
	}
	if exists && version > currentSchemaVersion {
		return fmt.Errorf("Execution History uses newer schema version %d; this binary supports %d", version, currentSchemaVersion)
	}

	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate Execution History: %w", err)
	}
	return nil
}

func (s *Store) existingSchemaVersion(ctx context.Context) (int, bool, error) {
	var tableCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations'`).Scan(&tableCount); err != nil {
		return 0, false, fmt.Errorf("inspect Execution History schema: %w", err)
	}
	if tableCount == 0 {
		return 0, false, nil
	}

	var version int
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version); err != nil {
		return 0, false, fmt.Errorf("read Execution History schema version: %w", err)
	}
	return version, true, nil
}
