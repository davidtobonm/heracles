// Package migration applies versioned, backed-up migrations to JSON
// documents such as `.heracles/labors/<id>/state.json`, per ADR 0030:
// compatible migrations run automatically after a backup, breaking
// migrations require confirmation before mutation, and an older binary
// rejects a newer schema version without writing it.
package migration

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// ErrNewerFormat indicates a document's schema version is newer than this
// binary supports. An older binary must reject and never rewrite it.
var ErrNewerFormat = errors.New("document uses a newer schema version than this binary supports")

// ErrConfirmationRequired indicates the migration plan includes a breaking
// step that was not confirmed, so the document was left unmodified.
var ErrConfirmationRequired = errors.New("migration requires confirmation")

// ErrCorrupt indicates a document cannot be parsed as JSON and so cannot be
// migrated or resumed.
var ErrCorrupt = errors.New("document is corrupt and cannot be parsed")

// Step is one versioned migration applied to a decoded JSON document.
type Step struct {
	From, To    int
	Breaking    bool
	Description string
	Apply       func(map[string]any) error
}

// Migrator describes the schema versions supported for one document kind.
type Migrator struct {
	// Current is the schema version this binary writes and resumes.
	Current int
	// Steps are the registered migrations, each advancing one document
	// from From to To.
	Steps []Step
}

// Plan returns the ordered steps required to bring a document at version up
// to Current. It returns ErrNewerFormat if version exceeds Current.
func (migrator Migrator) Plan(version int) ([]Step, error) {
	if version > migrator.Current {
		return nil, fmt.Errorf("%w: version %d, supported %d", ErrNewerFormat, version, migrator.Current)
	}
	var plan []Step
	for current := version; current < migrator.Current; {
		step, ok := migrator.step(current)
		if !ok {
			return nil, fmt.Errorf("no migration registered from schema version %d to %d", current, migrator.Current)
		}
		plan = append(plan, step)
		current = step.To
	}
	return plan, nil
}

func (migrator Migrator) step(from int) (Step, bool) {
	for _, step := range migrator.Steps {
		if step.From == from {
			return step, true
		}
	}
	return Step{}, false
}

// Result describes the outcome of applying a migration plan to a file.
type Result struct {
	FromVersion int
	ToVersion   int
	Changed     bool
	BackupPath  string
}

// ApplyFile reads the JSON document at path and migrates it to
// migrator.Current using the registered steps.
//
// If the document is already at Current, ApplyFile returns without
// modifying it. If the plan includes a breaking step, confirm is called
// with the planned steps; if confirm is nil, returns false, or returns an
// error, ApplyFile returns ErrConfirmationRequired without mutating the
// file. Otherwise ApplyFile writes a `.bak` backup of the original file
// before applying the plan and rewriting the document.
func ApplyFile(path string, migrator Migrator, confirm func([]Step) (bool, error)) (Result, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Result{}, fmt.Errorf("read %s: %w", path, err)
	}
	var document map[string]any
	if err := json.Unmarshal(contents, &document); err != nil {
		return Result{}, fmt.Errorf("%w: %s: %v", ErrCorrupt, path, err)
	}
	version := schemaVersion(document)
	plan, err := migrator.Plan(version)
	if err != nil {
		return Result{}, err
	}
	if len(plan) == 0 {
		return Result{FromVersion: version, ToVersion: version}, nil
	}
	for _, step := range plan {
		if !step.Breaking {
			continue
		}
		if confirm == nil {
			return Result{}, fmt.Errorf("%w: %s", ErrConfirmationRequired, step.Description)
		}
		ok, err := confirm(plan)
		if err != nil {
			return Result{}, err
		}
		if !ok {
			return Result{}, ErrConfirmationRequired
		}
		break
	}

	backupPath := path + ".bak"
	if err := os.WriteFile(backupPath, contents, 0o644); err != nil {
		return Result{}, fmt.Errorf("backup %s: %w", path, err)
	}
	for _, step := range plan {
		if err := step.Apply(document); err != nil {
			return Result{}, fmt.Errorf("migrate %s from version %d to %d: %w", path, step.From, step.To, err)
		}
		document["schema_version"] = step.To
	}
	migrated, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return Result{}, fmt.Errorf("encode %s: %w", path, err)
	}
	if err := os.WriteFile(path, append(migrated, '\n'), 0o644); err != nil {
		return Result{}, fmt.Errorf("write %s: %w", path, err)
	}
	return Result{FromVersion: version, ToVersion: migrator.Current, Changed: true, BackupPath: backupPath}, nil
}

func schemaVersion(document map[string]any) int {
	value, ok := document["schema_version"]
	if !ok {
		return 0
	}
	number, ok := value.(float64)
	if !ok {
		return 0
	}
	return int(number)
}
