package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// Runner executes validated provider invocations.
type Runner struct {
	registry    Registry
	environment []string
}

// Result is one completed provider invocation.
type Result struct {
	Invocation Invocation
	Stdout     string
	Stderr     string
	Final      string
	ExitCode   int
}

// NewRunner creates a provider runner. A nil environment uses the current process environment.
func NewRunner(registry Registry, environment []string) Runner {
	if environment == nil {
		environment = os.Environ()
	}
	return Runner{registry: registry, environment: append([]string(nil), environment...)}
}

// Run invokes a provider with timeout and environment controls.
func (r Runner) Run(ctx context.Context, provider string, profile Profile, workspaces []string, prompt string) (Result, error) {
	adapter, err := r.registry.Adapter(provider)
	if err != nil {
		return Result{}, err
	}
	invocation, err := adapter.Invocation(profile, workspaces, prompt)
	if err != nil {
		return Result{}, err
	}

	runContext := ctx
	cancel := func() {}
	if profile.Timeout > 0 {
		runContext, cancel = context.WithTimeout(ctx, profile.Timeout)
	}
	defer cancel()

	command := exec.CommandContext(runContext, invocation.Command, invocation.Args...)
	command.Dir = workspaces[0]
	command.Env = AllowedEnvironment(profile.EnvAllowlist, r.environment)
	if invocation.Stdin != "" {
		command.Stdin = strings.NewReader(invocation.Stdin)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	runErr := command.Run()
	result := Result{Invocation: invocation, Stdout: stdout.String(), Stderr: stderr.String()}
	if runErr != nil {
		var exitError *exec.ExitError
		if errors.As(runErr, &exitError) {
			result.ExitCode = exitError.ExitCode()
		}
		if errors.Is(runContext.Err(), context.DeadlineExceeded) {
			return result, fmt.Errorf("provider %s timed out after %s", provider, profile.Timeout)
		}
		return result, fmt.Errorf("provider %s failed with exit code %d: %s", provider, result.ExitCode, strings.TrimSpace(result.Stderr))
	}

	result.Final, err = NormalizeOutput(result.Stdout)
	if err != nil {
		return result, fmt.Errorf("normalize provider %s output: %w", provider, err)
	}
	return result, nil
}

// AllowedEnvironment filters environment variables to an explicit allowlist.
func AllowedEnvironment(allowlist, source []string) []string {
	values := make(map[string]string, len(source))
	for _, entry := range source {
		name, _, found := strings.Cut(entry, "=")
		if found {
			values[name] = entry
		}
	}
	environment := make([]string, 0, len(allowlist))
	for _, name := range allowlist {
		if entry, exists := values[name]; exists {
			environment = append(environment, entry)
		}
	}
	return environment
}

// NormalizeOutput extracts the last provider message from text or JSONL output.
func NormalizeOutput(output string) (string, error) {
	var final string
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		var value any
		if json.Unmarshal([]byte(line), &value) == nil {
			if candidate := finalString(value); candidate != "" {
				final = candidate
			}
			continue
		}
		final = line
	}
	if final == "" {
		return "", errors.New("provider returned no final message")
	}
	return final, nil
}

func finalString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []any:
		for index := len(typed) - 1; index >= 0; index-- {
			if candidate := finalString(typed[index]); candidate != "" {
				return candidate
			}
		}
	case map[string]any:
		for _, key := range []string{"result", "final", "content", "text", "message"} {
			if candidate := finalString(typed[key]); candidate != "" {
				return candidate
			}
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			if candidate := finalString(typed[key]); candidate != "" {
				return candidate
			}
		}
	}
	return ""
}
