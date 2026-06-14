package agent

import (
	"fmt"
	"runtime"
	"slices"
	"strings"
)

// Invocation is a validated non-interactive agent CLI command.
type Invocation struct {
	Command string
	Args    []string
	Stdin   string
}

// Adapter declares one agent CLI's capabilities and invocation contract.
type Adapter interface {
	Name() string
	Executable() string
	Capabilities() Capabilities
	Validate(Profile) error
	Invocation(Profile, []string, string) (Invocation, error)
}

// Capabilities declares the profile settings supported by one provider. An
// empty Platforms means the provider is supported on every platform.
type Capabilities struct {
	Model     bool
	Efforts   []string
	Variant   bool
	Platforms []string
}

// Registry contains the supported provider adapters.
type Registry struct {
	adapters map[string]Adapter
	order    []string
}

// DefaultRegistry returns the supported v1 provider adapters.
func DefaultRegistry() Registry {
	adapters := []Adapter{
		providerAdapter{name: "codex", executable: "codex", model: true, efforts: []string{"low", "medium", "high", "xhigh"}, build: codexInvocation},
		providerAdapter{name: "claude", executable: "claude", model: true, efforts: []string{"low", "medium", "high", "max"}, build: claudeInvocation},
		providerAdapter{name: "opencode", executable: "opencode", model: true, variant: true, promptArg: true, build: opencodeInvocation},
		providerAdapter{name: "kimi", executable: "kimi", model: true, build: kimiInvocation},
		providerAdapter{name: "openclaw", executable: "openclaw", model: true, efforts: []string{"low", "medium", "high"}, build: openclawInvocation},
		providerAdapter{name: "hermes", executable: "hermes", model: true, variant: true, promptArg: true, build: hermesInvocation},
	}
	registry := Registry{adapters: make(map[string]Adapter, len(adapters)), order: make([]string, len(adapters))}
	for index, adapter := range adapters {
		registry.adapters[adapter.Name()] = adapter
		registry.order[index] = adapter.Name()
	}
	return registry
}

// Adapter returns a supported provider adapter.
func (r Registry) Adapter(name string) (Adapter, error) {
	adapter, exists := r.adapters[strings.ToLower(name)]
	if !exists {
		return nil, fmt.Errorf("unsupported provider %q", name)
	}
	return adapter, nil
}

// Names returns the supported provider names in their declared order.
func (r Registry) Names() []string {
	return append([]string(nil), r.order...)
}

type providerAdapter struct {
	name       string
	executable string
	model      bool
	efforts    []string
	variant    bool
	platforms  []string
	promptArg  bool
	build      func(Profile, []string, string) Invocation
}

func (a providerAdapter) Name() string {
	return a.name
}

func (a providerAdapter) Executable() string {
	return a.executable
}

func (a providerAdapter) Capabilities() Capabilities {
	return Capabilities{
		Model:     a.model,
		Efforts:   append([]string(nil), a.efforts...),
		Variant:   a.variant,
		Platforms: append([]string(nil), a.platforms...),
	}
}

func (a providerAdapter) Validate(profile Profile) error {
	if !platformSupported(a.platforms, runtime.GOOS) {
		return fmt.Errorf("provider %s is not supported on %s", a.name, runtime.GOOS)
	}
	if profile.Model != "" && !a.model {
		return fmt.Errorf("provider %s does not support model settings", a.name)
	}
	if profile.Effort != "" {
		if len(a.efforts) == 0 {
			return fmt.Errorf("provider %s does not support effort settings", a.name)
		}
		if !slices.Contains(a.efforts, profile.Effort) {
			return fmt.Errorf("provider %s does not support effort %q; supported values: %s", a.name, profile.Effort, strings.Join(a.efforts, ", "))
		}
	}
	if profile.Variant != "" && !a.variant {
		return fmt.Errorf("provider %s does not support variant settings", a.name)
	}
	if profile.Timeout < 0 {
		return fmt.Errorf("provider %s timeout must be positive", a.name)
	}
	return nil
}

func (a providerAdapter) Invocation(profile Profile, workspaces []string, prompt string) (Invocation, error) {
	if len(workspaces) == 0 {
		return Invocation{}, fmt.Errorf("provider %s requires at least one Issue Workspace path", a.name)
	}
	if err := a.Validate(profile); err != nil {
		return Invocation{}, err
	}
	invocation := a.build(profile, workspaces, prompt)
	if a.promptArg && len(invocation.Args) > 0 {
		promptIndex := len(invocation.Args) - 1
		args := append([]string(nil), invocation.Args[:promptIndex]...)
		args = append(args, profile.ExtraArgs...)
		invocation.Args = append(args, invocation.Args[promptIndex])
	} else {
		invocation.Args = append(invocation.Args, profile.ExtraArgs...)
	}
	return invocation, nil
}

func codexInvocation(profile Profile, workspaces []string, prompt string) Invocation {
	args := []string{"exec", "--json", "--dangerously-bypass-approvals-and-sandbox", "--skip-git-repo-check", "-C", workspaces[0]}
	if profile.Model != "" {
		args = append(args, "-m", profile.Model)
	}
	if profile.Effort != "" {
		args = append(args, "-c", `model_reasoning_effort="`+profile.Effort+`"`)
	}
	return Invocation{Command: "codex", Args: args, Stdin: prompt}
}

func claudeInvocation(profile Profile, workspaces []string, prompt string) Invocation {
	args := []string{"-p", "--permission-mode", "bypassPermissions", "--dangerously-skip-permissions"}
	if !hasFlag(profile.ExtraArgs, "--output-format") {
		args = append(args, "--output-format", "stream-json")
	}
	if !hasFlag(profile.ExtraArgs, "--output-format") || flagValue(profile.ExtraArgs, "--output-format") == "stream-json" {
		args = append(args, "--verbose")
	}
	for _, workspace := range workspaces[1:] {
		args = append(args, "--add-dir", workspace)
	}
	if profile.Model != "" {
		args = append(args, "--model", profile.Model)
	}
	if profile.Effort != "" {
		args = append(args, "--effort", profile.Effort)
	}
	return Invocation{Command: "claude", Args: args, Stdin: prompt}
}

func opencodeInvocation(profile Profile, workspaces []string, prompt string) Invocation {
	args := []string{"run", "--dir", workspaces[0], "--dangerously-skip-permissions", "--format", "json"}
	if profile.Model != "" {
		args = append(args, "--model", profile.Model)
	}
	if profile.Variant != "" {
		args = append(args, "--variant", profile.Variant)
	}
	args = append(args, prompt)
	return Invocation{Command: "opencode", Args: args}
}

func kimiInvocation(profile Profile, workspaces []string, prompt string) Invocation {
	args := []string{"--print", "--yolo", "--output-format", "stream-json", "--work-dir", workspaces[0]}
	for _, workspace := range workspaces[1:] {
		args = append(args, "--add-dir", workspace)
	}
	if profile.Model != "" {
		args = append(args, "--model", profile.Model)
	}
	return Invocation{Command: "kimi", Args: args, Stdin: prompt}
}

func openclawInvocation(profile Profile, workspaces []string, prompt string) Invocation {
	args := []string{"run", "--print", "--full-access", "--output-format", "json", "--work-dir", workspaces[0]}
	for _, workspace := range workspaces[1:] {
		args = append(args, "--add-dir", workspace)
	}
	if profile.Model != "" {
		args = append(args, "--model", profile.Model)
	}
	if profile.Effort != "" {
		args = append(args, "--effort", profile.Effort)
	}
	return Invocation{Command: "openclaw", Args: args, Stdin: prompt}
}

func hermesInvocation(profile Profile, workspaces []string, prompt string) Invocation {
	args := []string{"run", "--dir", workspaces[0], "--unsafe", "--format", "json"}
	if profile.Model != "" {
		args = append(args, "--model", profile.Model)
	}
	if profile.Variant != "" {
		args = append(args, "--variant", profile.Variant)
	}
	args = append(args, prompt)
	return Invocation{Command: "hermes", Args: args}
}

// platformSupported reports whether goos is a supported platform. An empty
// platforms list means every platform is supported.
func platformSupported(platforms []string, goos string) bool {
	return len(platforms) == 0 || slices.Contains(platforms, goos)
}

func hasFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag || strings.HasPrefix(arg, flag+"=") {
			return true
		}
	}
	return false
}

func flagValue(args []string, flag string) string {
	for index, arg := range args {
		if arg == flag && index+1 < len(args) {
			return args[index+1]
		}
		if strings.HasPrefix(arg, flag+"=") {
			return strings.TrimPrefix(arg, flag+"=")
		}
	}
	return ""
}
