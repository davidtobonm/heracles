// Package fakeexec creates deterministic fake executables for adapter contract tests.
package fakeexec

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// Response defines a fake executable's deterministic result.
type Response struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Executable describes a fake executable and its recorded invocation.
type Executable struct {
	Path       string
	argsPath   string
	stdinPath  string
	stderrPath string
}

// Invocation contains the arguments, standard input, and configured standard error.
type Invocation struct {
	Args   string
	Stdin  string
	Stderr string
}

// New creates a deterministic executable inside the test's temporary directory.
func New(t testing.TB, response Response) Executable {
	t.Helper()

	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args")
	stdinPath := filepath.Join(dir, "stdin")
	stdoutPath := filepath.Join(dir, "stdout")
	stderrPath := filepath.Join(dir, "stderr")
	executablePath := filepath.Join(dir, "fake-agent")

	writeFile(t, stdoutPath, response.Stdout, 0o600)
	writeFile(t, stderrPath, response.Stderr, 0o600)

	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + shellQuote(argsPath) + "\n" +
		"cat > " + shellQuote(stdinPath) + "\n" +
		"cat " + shellQuote(stdoutPath) + "\n" +
		"cat " + shellQuote(stderrPath) + " >&2\n" +
		"exit " + strconv.Itoa(response.ExitCode) + "\n"
	writeFile(t, executablePath, script, 0o700)

	return Executable{
		Path:       executablePath,
		argsPath:   argsPath,
		stdinPath:  stdinPath,
		stderrPath: stderrPath,
	}
}

// Invocation reads the fake executable's most recent invocation.
func (e Executable) Invocation(t testing.TB) Invocation {
	t.Helper()

	return Invocation{
		Args:   readFile(t, e.argsPath),
		Stdin:  readFile(t, e.stdinPath),
		Stderr: readFile(t, e.stderrPath),
	}
}

func writeFile(t testing.TB, path, contents string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatalf("write fake executable file: %v", err)
	}
}

func readFile(t testing.TB, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fake executable file: %v", err)
	}
	return string(contents)
}

func shellQuote(value string) string {
	return "'" + value + "'"
}
