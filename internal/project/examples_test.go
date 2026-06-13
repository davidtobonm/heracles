package project_test

import (
	"path/filepath"
	"testing"

	"github.com/davidtobonm/heracles/internal/project"
)

func TestDocumentedTopologyExamplesAreValidProjectConfigurations(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob(filepath.Join("..", "..", "examples", "*.yaml"))
	if err != nil {
		t.Fatalf("glob examples: %v", err)
	}
	if len(paths) != 4 {
		t.Fatalf("examples = %d, want single, monorepo, multiple, and separate-tracker topologies", len(paths))
	}
	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			loaded, err := project.Load(path)
			if err != nil {
				t.Fatalf("Load(%s) error = %v", path, err)
			}
			if len(loaded.Config.Repositories) == 0 || loaded.Config.Agents.DefaultProfile == "" {
				t.Errorf("example = %#v, want operable topology", loaded.Config)
			}
		})
	}
}
