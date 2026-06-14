// Package state defines the durable schema version for
// `.heracles/labors/<id>/state.json` and the migrations that bring older
// Labor state up to date, per ADR 0030.
package state

import (
	"errors"
	"fmt"

	"github.com/davidtobonm/heracles/internal/migration"
)

// CurrentSchemaVersion is the Labor-state schema version this binary
// writes and resumes.
const CurrentSchemaVersion = 1

// Migrator describes the supported Labor-state schema versions for
// `.heracles/labors/<id>/state.json`.
var Migrator = migration.Migrator{
	Current: CurrentSchemaVersion,
	Steps: []migration.Step{
		{
			From: 0, To: 1,
			Description: "record the Heracles version that created and last ran this Labor",
			Apply: func(document map[string]any) error {
				if _, ok := document["heracles_version"]; !ok {
					document["heracles_version"] = "unknown"
				}
				return nil
			},
		},
	},
}

// MigrateFile brings a Labor's durable state.json up to CurrentSchemaVersion,
// creating a `.bak` backup before any compatible migration. If the document
// was created by a newer, incompatible Heracles version, MigrateFile leaves
// it unmodified and returns a descriptive error.
func MigrateFile(path string) (migration.Result, error) {
	result, err := migration.ApplyFile(path, Migrator, nil)
	if errors.Is(err, migration.ErrNewerFormat) {
		return result, fmt.Errorf("%s was created by a newer Heracles version and cannot be resumed by this binary; upgrade Heracles to resume this Labor: %w", path, err)
	}
	if errors.Is(err, migration.ErrCorrupt) {
		return result, fmt.Errorf("%s is corrupt; this Labor's local state is unrecoverable and GitHub issues and pull requests are left unchanged, start a new Labor against the same approved PRD: %w", path, err)
	}
	return result, err
}
