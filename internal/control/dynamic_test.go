package control_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/davidtobonm/heracles/internal/control"
)

func TestDynamicControlInitializesThenUsesLocalApplicationCore(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "app")
	run(t, "", "git", "init", "--initial-branch=main", root)
	run(t, root, "git", "remote", "add", "origin", "git@github.com:acme/app.git")
	surface := control.NewDynamic(root, "")
	t.Cleanup(func() { _ = surface.Close() })

	result, err := surface.Execute(context.Background(), control.Operation{Name: "init"})
	if err != nil {
		t.Fatalf("Execute(init) error = %v", err)
	}
	if result.Status != "initialized" {
		t.Errorf("init result = %#v", result)
	}
	result, err = surface.Execute(context.Background(), control.Operation{Name: "list", Kind: "labors"})
	if err != nil {
		t.Fatalf("Execute(list) error = %v", err)
	}
	if result.Status != "ok" {
		t.Errorf("list result = %#v", result)
	}
}

func run(t testing.TB, directory, command string, args ...string) {
	t.Helper()
	process := exec.Command(command, args...)
	process.Dir = directory
	if output, err := process.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", command, args, err, output)
	}
}
