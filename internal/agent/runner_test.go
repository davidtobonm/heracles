package agent_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/testutil/fakeexec"
)

func TestRunnerExecutesAdapterWithFilteredEnvironmentAndNormalizesOutput(t *testing.T) {
	fake := fakeexec.New(t, fakeexec.Response{
		Stdout: "{\"type\":\"message\",\"content\":\"working\"}\n{\"type\":\"result\",\"result\":\"completed delivery\"}\n",
	})
	binDirectory := t.TempDir()
	if err := os.Symlink(fake.Path, filepath.Join(binDirectory, "codex")); err != nil {
		t.Fatalf("link fake codex: %v", err)
	}
	path := binDirectory + string(os.PathListSeparator) + os.Getenv("PATH")
	t.Setenv("PATH", path)

	runner := agent.NewRunner(agent.DefaultRegistry(), []string{
		"PATH=" + path,
		"HERACLES_ALLOWED=yes",
		"HERACLES_SECRET=no",
	})
	result, err := runner.Run(context.Background(), "codex", agent.Profile{
		Model:        "gpt-test",
		Effort:       "medium",
		Timeout:      time.Second,
		EnvAllowlist: []string{"PATH", "HERACLES_ALLOWED"},
	}, []string{t.TempDir()}, "deliver issue")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Final != "completed delivery" {
		t.Errorf("normalized output = %q, want final provider message", result.Final)
	}

	invocation := fake.Invocation(t)
	if invocation.Stdin != "deliver issue" || !strings.Contains(invocation.Args, "gpt-test") {
		t.Errorf("fake invocation = %#v, want provider prompt and profile arguments", invocation)
	}

	environment := agent.AllowedEnvironment([]string{"HERACLES_ALLOWED"}, []string{"HERACLES_ALLOWED=yes", "HERACLES_SECRET=no"})
	if len(environment) != 1 || environment[0] != "HERACLES_ALLOWED=yes" {
		t.Errorf("allowed environment = %#v, want only allowlisted variable", environment)
	}
}

func TestRunnerEnforcesProfileTimeout(t *testing.T) {
	binDirectory := t.TempDir()
	executable := filepath.Join(binDirectory, "kimi")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\nsleep 2\n"), 0o700); err != nil {
		t.Fatalf("write fake kimi: %v", err)
	}
	path := binDirectory + string(os.PathListSeparator) + os.Getenv("PATH")
	t.Setenv("PATH", path)

	runner := agent.NewRunner(agent.DefaultRegistry(), []string{"PATH=" + path})
	_, err := runner.Run(context.Background(), "kimi", agent.Profile{
		Timeout:      10 * time.Millisecond,
		EnvAllowlist: []string{"PATH"},
	}, []string{t.TempDir()}, "wait forever")
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("Run() error = %v, want timeout", err)
	}
}
