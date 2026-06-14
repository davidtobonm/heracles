package agent_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/davidtobonm/heracles/internal/agent"
	"github.com/davidtobonm/heracles/internal/redact"
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
		Timeout:      3 * time.Second,
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
		t.Errorf("allowed environment = %#v, want allowlisted variable when no essentials are present", environment)
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

func TestNormalizeOutputIgnoresTrailingCodexLifecycleEvents(t *testing.T) {
	t.Parallel()

	output := "{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"[{\\\"id\\\":\\\"slice\\\"}]\"}}\n" +
		"{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":10}}\n"
	final, err := agent.NormalizeOutput(output)
	if err != nil {
		t.Fatalf("NormalizeOutput() error = %v", err)
	}
	if final != `[{"id":"slice"}]` {
		t.Fatalf("NormalizeOutput() = %q", final)
	}
}

func TestNormalizeOutputPrefersTerminalResultOverIntermediateClaudeMessages(t *testing.T) {
	t.Parallel()

	output := "{\"type\":\"assistant\",\"message\":{\"content\":[{\"type\":\"text\",\"text\":\"Now let's inspect the diff.\"}]}}\n" +
		"{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"result\":\"{\\\"changes\\\":\\\"done\\\"}\"}\n"
	final, err := agent.NormalizeOutput(output)
	if err != nil {
		t.Fatalf("NormalizeOutput() error = %v", err)
	}
	if final != `{"changes":"done"}` {
		t.Fatalf("NormalizeOutput() = %q, want terminal result payload", final)
	}
}

func TestNormalizeOutputPrefersStructuredTerminalResult(t *testing.T) {
	t.Parallel()

	output := "{\"type\":\"assistant\",\"message\":{\"content\":[{\"type\":\"text\",\"text\":\"Working...\"}]}}\n" +
		"{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"result\":\"Done.\",\"structured_output\":{\"changes\":\"done\",\"evidence_policy\":{\"Exempt\":true,\"Reason\":\"n/a\"}}}\n"
	final, err := agent.NormalizeOutput(output)
	if err != nil {
		t.Fatalf("NormalizeOutput() error = %v", err)
	}
	if final != `{"changes":"done","evidence_policy":{"Exempt":true,"Reason":"n/a"}}` {
		t.Fatalf("NormalizeOutput() = %q, want structured terminal payload", final)
	}
}

func TestNormalizeOutputIgnoresTrailingDiagnosticsAfterTerminalResult(t *testing.T) {
	t.Parallel()

	output := "{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"result\":\"Done.\",\"structured_output\":{\"ok\":true}}\n" +
		"[ede_diagnostic] result_type=user last_content_type=n/a stop_reason=tool_use\n"
	final, err := agent.NormalizeOutput(output)
	if err != nil {
		t.Fatalf("NormalizeOutput() error = %v", err)
	}
	if final != `{"ok":true}` {
		t.Fatalf("NormalizeOutput() = %q, want terminal structured output", final)
	}
}

func TestNormalizeOutputReturnsTerminalProviderErrors(t *testing.T) {
	t.Parallel()

	output := "{\"type\":\"assistant\",\"message\":{\"content\":[{\"type\":\"text\",\"text\":\"Still working\"}]}}\n" +
		"{\"type\":\"result\",\"subtype\":\"error_during_execution\",\"is_error\":true}\n"
	_, err := agent.NormalizeOutput(output)
	if err == nil || !strings.Contains(err.Error(), "error_during_execution") {
		t.Fatalf("NormalizeOutput() error = %v, want terminal provider error", err)
	}
}

func TestRunnerIncludesStdoutWhenProviderFailsWithoutStderr(t *testing.T) {
	fake := fakeexec.New(t, fakeexec.Response{
		Stdout:   `{"type":"error","message":"unsupported model"}` + "\n",
		ExitCode: 1,
	})
	binDirectory := t.TempDir()
	if err := os.Symlink(fake.Path, filepath.Join(binDirectory, "claude")); err != nil {
		t.Fatalf("link fake claude: %v", err)
	}
	path := binDirectory + string(os.PathListSeparator) + os.Getenv("PATH")
	t.Setenv("PATH", path)

	runner := agent.NewRunner(agent.DefaultRegistry(), []string{"PATH=" + path})
	_, err := runner.Run(context.Background(), "claude", agent.Profile{
		Timeout:      3 * time.Second,
		EnvAllowlist: []string{"PATH"},
	}, []string{t.TempDir()}, "review")
	if err == nil || !strings.Contains(err.Error(), "unsupported model") {
		t.Fatalf("Run() error = %v, want stdout-backed failure", err)
	}
}

func TestAllowedEnvironmentIncludesEssentialVariables(t *testing.T) {
	t.Parallel()

	environment := agent.AllowedEnvironment(
		[]string{"CUSTOM_TOKEN"},
		[]string{
			"HOME=/tmp/home",
			"PATH=/usr/bin",
			"TERM=xterm-256color",
			"XDG_CONFIG_HOME=/tmp/config",
			"CUSTOM_TOKEN=yes",
			"UNRELATED_SECRET=no",
		},
	)
	for _, expected := range []string{"HOME=/tmp/home", "PATH=/usr/bin", "TERM=xterm-256color", "XDG_CONFIG_HOME=/tmp/config", "CUSTOM_TOKEN=yes"} {
		if !contains(environment, expected) {
			t.Fatalf("AllowedEnvironment() = %#v, want %q", environment, expected)
		}
	}
	if contains(environment, "UNRELATED_SECRET=no") {
		t.Fatalf("AllowedEnvironment() = %#v, should exclude unrelated secret", environment)
	}
}

func TestRunnerRedactsAllowlistedSecretValuesFromOutput(t *testing.T) {
	fake := fakeexec.New(t, fakeexec.Response{
		Stdout: "token=super-secret-output-value\n",
	})
	binDirectory := t.TempDir()
	if err := os.Symlink(fake.Path, filepath.Join(binDirectory, "codex")); err != nil {
		t.Fatalf("link fake codex: %v", err)
	}
	path := binDirectory + string(os.PathListSeparator) + os.Getenv("PATH")
	t.Setenv("PATH", path)

	runner := agent.NewRunner(agent.DefaultRegistry(), []string{
		"PATH=" + path,
		"HERACLES_API_KEY=super-secret-output-value",
	})
	result, err := runner.Run(context.Background(), "codex", agent.Profile{
		Model:        "gpt-test",
		Effort:       "medium",
		Timeout:      3 * time.Second,
		EnvAllowlist: []string{"PATH", "HERACLES_API_KEY"},
	}, []string{t.TempDir()}, "deliver issue")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.Contains(result.Stdout, "super-secret-output-value") {
		t.Errorf("result.Stdout = %q, secret value leaked", result.Stdout)
	}
	if !strings.Contains(result.Stdout, redact.Placeholder) {
		t.Errorf("result.Stdout = %q, want redaction placeholder", result.Stdout)
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
