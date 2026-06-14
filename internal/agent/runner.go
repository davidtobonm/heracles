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

	"github.com/davidtobonm/heracles/internal/environment"
	"github.com/davidtobonm/heracles/internal/redact"
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
	redactor := redact.New(environment.SecretValues(command.Env))
	result := Result{Invocation: invocation, Stdout: redactor.String(stdout.String()), Stderr: redactor.String(stderr.String())}
	if runErr != nil {
		var exitError *exec.ExitError
		if errors.As(runErr, &exitError) {
			result.ExitCode = exitError.ExitCode()
		}
		if errors.Is(runContext.Err(), context.DeadlineExceeded) {
			return result, fmt.Errorf("provider %s timed out after %s", provider, profile.Timeout)
		}
		message := strings.TrimSpace(result.Stderr)
		if message == "" {
			message = strings.TrimSpace(result.Stdout)
		}
		if message == "" {
			message = redactor.String(runErr.Error())
		}
		return result, fmt.Errorf("provider %s failed with exit code %d: %s", provider, result.ExitCode, message)
	}

	result.Final, err = NormalizeOutput(result.Stdout)
	if err != nil {
		return result, fmt.Errorf("normalize provider %s output: %w", provider, err)
	}
	return result, nil
}

// AllowedEnvironment filters environment variables to an explicit allowlist plus
// the essential process variables required by authenticated CLI providers.
func AllowedEnvironment(allowlist, source []string) []string {
	return environment.Filter(allowlist, source)
}

// NormalizeOutput extracts the last provider message from text or JSONL output.
func NormalizeOutput(output string) (string, error) {
	var final string
	terminalSeen := false
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		var value any
		if json.Unmarshal([]byte(line), &value) == nil {
			if typed, ok := value.(map[string]any); ok {
				if terminal, ok, err := terminalResult(typed); ok {
					terminalSeen = true
					if err != nil {
						return "", err
					}
					if terminal != "" {
						return terminal, nil
					}
					continue
				}
			}
			if terminalSeen {
				continue
			}
			if candidate := finalString(value); candidate != "" {
				final = candidate
			}
			continue
		}
		if terminalSeen {
			continue
		}
		final = line
	}
	if final == "" {
		return "", errors.New("provider returned no final message")
	}
	return final, nil
}

func terminalResult(value map[string]any) (string, bool, error) {
	eventType, _ := value["type"].(string)
	if eventType != "result" {
		return "", false, nil
	}
	if structured, exists := value["structured_output"]; exists {
		payload, err := marshalTerminalJSON(structured)
		if err != nil {
			return "", true, fmt.Errorf("marshal structured provider output: %w", err)
		}
		if isError, _ := value["is_error"].(bool); isError {
			return "", true, errors.New(payload)
		}
		return payload, true, nil
	}
	if message := finalString(value["result"]); message != "" {
		if isError, _ := value["is_error"].(bool); isError {
			return "", true, errors.New(message)
		}
		return message, true, nil
	}
	if isError, _ := value["is_error"].(bool); isError {
		if subtype, _ := value["subtype"].(string); subtype != "" {
			return "", true, fmt.Errorf("provider execution ended with %s", subtype)
		}
		return "", true, errors.New("provider execution ended with an error")
	}
	return "", true, nil
}

func marshalTerminalJSON(value any) (string, error) {
	contents, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(contents), nil
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
		if eventType, _ := typed["type"].(string); eventType == "turn.completed" || eventType == "thread.started" || eventType == "assistant" || eventType == "user" || eventType == "system" || eventType == "step_start" || eventType == "step_finish" {
			return ""
		}
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
