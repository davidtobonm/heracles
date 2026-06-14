//go:build integration

package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCompiledBinaryExposesHelpAndVersion(t *testing.T) {
	binaryName := "heracles"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(t.TempDir(), binaryName)

	build := exec.Command("go", "build", "-o", binaryPath, "../cmd/heracles")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build binary: %v\n%s", err, output)
	}

	for _, command := range []struct {
		args     []string
		expected string
	}{
		{args: []string{"--help"}, expected: "Usage:"},
		{args: []string{"version"}, expected: "heracles version="},
	} {
		output, err := exec.Command(binaryPath, command.args...).CombinedOutput()
		if err != nil {
			t.Fatalf("run %v: %v\n%s", command.args, err, output)
		}
		if !strings.Contains(string(output), command.expected) {
			t.Errorf("run %v output %q does not contain %q", command.args, output, command.expected)
		}
	}

	repositoryPath := filepath.Join(t.TempDir(), "widget")
	run(t, "", "git", "init", "--initial-branch=main", repositoryPath)
	run(t, repositoryPath, "git", "remote", "add", "origin", "git@github.com:example/widget.git")

	initCommand := exec.Command(binaryPath, "init", "--tracker", "example/widget", "--repo", repositoryPath)
	initCommand.Dir = repositoryPath
	if output, err := initCommand.CombinedOutput(); err != nil {
		t.Fatalf("run init: %v\n%s", err, output)
	}
	if _, err := os.Stat(filepath.Join(repositoryPath, "heracles.yaml")); err != nil {
		t.Fatalf("init did not create Project Configuration: %v", err)
	}
}

func run(t testing.TB, workingDirectory, command string, args ...string) {
	t.Helper()

	process := exec.Command(command, args...)
	process.Dir = workingDirectory
	if output, err := process.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", command, args, err, output)
	}
}
