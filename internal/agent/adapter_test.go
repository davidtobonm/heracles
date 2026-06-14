package agent_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/davidtobonm/heracles/internal/agent"
)

func TestProviderAdaptersBuildValidatedNonInteractiveInvocations(t *testing.T) {
	t.Parallel()

	registry := agent.DefaultRegistry()
	workspaces := []string{"/workspace", "/shared"}
	tests := []struct {
		provider string
		profile  agent.Profile
		command  string
		args     []string
		stdin    string
	}{
		{
			provider: "codex",
			profile:  agent.Profile{Model: "gpt-5.4", Effort: "high", ExtraArgs: []string{"--search"}},
			command:  "codex",
			args:     []string{"exec", "--json", "--dangerously-bypass-approvals-and-sandbox", "-C", "/workspace", "-m", "gpt-5.4", `model_reasoning_effort="high"`, "--search"},
			stdin:    "do the work",
		},
		{
			provider: "claude",
			profile:  agent.Profile{Model: "sonnet", Effort: "high"},
			command:  "claude",
			args:     []string{"-p", "--output-format", "stream-json", "--verbose", "--add-dir", "/shared", "--model", "sonnet", "--effort", "high"},
			stdin:    "do the work",
		},
		{
			provider: "claude",
			profile:  agent.Profile{Model: "sonnet", Effort: "high", ExtraArgs: []string{"--output-format", "json", "--json-schema", "{\"type\":\"object\"}"}},
			command:  "claude",
			args:     []string{"-p", "--model", "sonnet", "--effort", "high", "--output-format", "json", "--json-schema", "{\"type\":\"object\"}"},
			stdin:    "do the work",
		},
		{
			provider: "opencode",
			profile:  agent.Profile{Model: "anthropic/sonnet", Variant: "high"},
			command:  "opencode",
			args:     []string{"run", "--dir", "/workspace", "--format", "json", "--model", "anthropic/sonnet", "--variant", "high", "do the work"},
		},
		{
			provider: "kimi",
			profile:  agent.Profile{Model: "kimi-k2.5"},
			command:  "kimi",
			args:     []string{"--print", "--yolo", "--output-format", "stream-json", "--work-dir", "/workspace", "--add-dir", "/shared", "--model", "kimi-k2.5"},
			stdin:    "do the work",
		},
		{
			provider: "openclaw",
			profile:  agent.Profile{Model: "gpt-5", Effort: "high"},
			command:  "openclaw",
			args:     []string{"run", "--print", "--full-access", "--output-format", "json", "--work-dir", "/workspace", "--add-dir", "/shared", "--model", "gpt-5", "--effort", "high"},
			stdin:    "do the work",
		},
		{
			provider: "hermes",
			profile:  agent.Profile{Model: "hermes-4", Variant: "thinking"},
			command:  "hermes",
			args:     []string{"run", "--dir", "/workspace", "--unsafe", "--format", "json", "--model", "hermes-4", "--variant", "thinking", "do the work"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.provider, func(t *testing.T) {
			adapter, err := registry.Adapter(testCase.provider)
			if err != nil {
				t.Fatalf("Adapter() error = %v", err)
			}
			invocation, err := adapter.Invocation(testCase.profile, workspaces, "do the work")
			if err != nil {
				t.Fatalf("Invocation() error = %v", err)
			}
			if invocation.Command != testCase.command {
				t.Errorf("command = %q, want %q", invocation.Command, testCase.command)
			}
			if !adapter.Capabilities().Model {
				t.Errorf("%s adapter does not declare model capability", testCase.provider)
			}
			for _, expected := range testCase.args {
				if !slices.Contains(invocation.Args, expected) {
					t.Errorf("args = %#v, want %q", invocation.Args, expected)
				}
			}
			if invocation.Stdin != testCase.stdin {
				t.Errorf("stdin = %q, want %q", invocation.Stdin, testCase.stdin)
			}
		})
	}
}

func TestProviderAdaptersIncludeVerifiedFullPermissionBypassFlags(t *testing.T) {
	t.Parallel()

	registry := agent.DefaultRegistry()
	workspaces := []string{"/workspace"}
	for provider, flag := range map[string]string{
		"codex":    "--dangerously-bypass-approvals-and-sandbox",
		"claude":   "--dangerously-skip-permissions",
		"opencode": "--dangerously-skip-permissions",
		"kimi":     "--yolo",
		"openclaw": "--full-access",
		"hermes":   "--unsafe",
	} {
		t.Run(provider, func(t *testing.T) {
			adapter, err := registry.Adapter(provider)
			if err != nil {
				t.Fatalf("Adapter() error = %v", err)
			}
			invocation, err := adapter.Invocation(agent.Profile{}, workspaces, "do the work")
			if err != nil {
				t.Fatalf("Invocation() error = %v", err)
			}
			if !slices.Contains(invocation.Args, flag) {
				t.Errorf("%s args = %#v, want verified full-permission bypass flag %q", provider, invocation.Args, flag)
			}
		})
	}
}

func TestProviderAdaptersBuildInteractiveInvocationsWithThePromptAsTheInitialMessage(t *testing.T) {
	t.Parallel()

	registry := agent.DefaultRegistry()
	workspaces := []string{"/workspace", "/shared"}
	tests := []struct {
		provider string
		profile  agent.Profile
		command  string
		bypass   string
	}{
		{provider: "codex", profile: agent.Profile{Model: "gpt-5.4", Effort: "high"}, command: "codex", bypass: "--dangerously-bypass-approvals-and-sandbox"},
		{provider: "claude", profile: agent.Profile{Model: "sonnet", Effort: "high"}, command: "claude", bypass: "--dangerously-skip-permissions"},
		{provider: "opencode", profile: agent.Profile{Model: "anthropic/sonnet", Variant: "high"}, command: "opencode", bypass: "--dangerously-skip-permissions"},
		{provider: "kimi", profile: agent.Profile{Model: "kimi-k2.5"}, command: "kimi", bypass: "--yolo"},
		{provider: "openclaw", profile: agent.Profile{Model: "gpt-5", Effort: "high"}, command: "openclaw", bypass: "--full-access"},
		{provider: "hermes", profile: agent.Profile{Model: "hermes-4", Variant: "thinking"}, command: "hermes", bypass: "--unsafe"},
	}

	for _, testCase := range tests {
		t.Run(testCase.provider, func(t *testing.T) {
			adapter, err := registry.Adapter(testCase.provider)
			if err != nil {
				t.Fatalf("Adapter() error = %v", err)
			}
			invocation, err := adapter.InteractiveInvocation(testCase.profile, workspaces, "let's plan")
			if err != nil {
				t.Fatalf("InteractiveInvocation() error = %v", err)
			}
			if invocation.Command != testCase.command {
				t.Errorf("command = %q, want %q", invocation.Command, testCase.command)
			}
			if invocation.Stdin != "" {
				t.Errorf("stdin = %q, want empty for an interactive session", invocation.Stdin)
			}
			if len(invocation.Args) == 0 || invocation.Args[len(invocation.Args)-1] != "let's plan" {
				t.Errorf("args = %#v, want the prompt as the final argument", invocation.Args)
			}
			if !slices.Contains(invocation.Args, testCase.bypass) {
				t.Errorf("args = %#v, want verified full-permission bypass flag %q", invocation.Args, testCase.bypass)
			}
			for _, flag := range []string{"--print", "-p", "exec", "run", "--output-format", "--json", "--format"} {
				if slices.Contains(invocation.Args, flag) {
					t.Errorf("args = %#v, want no non-interactive flag %q", invocation.Args, flag)
				}
			}
		})
	}
}

func TestProviderCapabilitiesRejectUnsupportedSettings(t *testing.T) {
	t.Parallel()

	registry := agent.DefaultRegistry()
	for name, testCase := range map[string]struct {
		provider string
		profile  agent.Profile
		expected string
	}{
		"unknown provider":    {provider: "antigravity", expected: "unsupported provider"},
		"codex variant":       {provider: "codex", profile: agent.Profile{Variant: "fast"}, expected: "variant"},
		"opencode effort":     {provider: "opencode", profile: agent.Profile{Effort: "high"}, expected: "effort"},
		"kimi effort":         {provider: "kimi", profile: agent.Profile{Effort: "high"}, expected: "effort"},
		"hermes effort":       {provider: "hermes", profile: agent.Profile{Effort: "high"}, expected: "effort"},
		"openclaw variant":    {provider: "openclaw", profile: agent.Profile{Variant: "fast"}, expected: "variant"},
		"unsupported effort":  {provider: "codex", profile: agent.Profile{Effort: "maximum"}, expected: "maximum"},
		"nonpositive timeout": {provider: "codex", profile: agent.Profile{Timeout: -time.Second}, expected: "timeout"},
	} {
		t.Run(name, func(t *testing.T) {
			adapter, err := registry.Adapter(testCase.provider)
			if err == nil {
				err = adapter.Validate(testCase.profile)
			}
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), testCase.expected) {
				t.Fatalf("validation error = %v, want %q", err, testCase.expected)
			}
		})
	}
}
