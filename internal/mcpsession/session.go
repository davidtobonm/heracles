// Package mcpsession prepares a launched provider session's workspace with
// Heracles's bundled skills and MCP server configuration, per ADR 0019.
// Injection never overwrites locally modified skills or unrelated provider
// configuration, and the returned cleanup function restores the workspace to
// its prior state.
package mcpsession

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/davidtobonm/heracles/internal/skills"
)

// Config describes one provider session to prepare.
type Config struct {
	// Provider is the registered provider name (e.g. "claude", "codex").
	Provider string
	// Workspace is the session's working directory.
	Workspace string
	// ConfigPath is the Heracles Project Configuration path passed to
	// `heracles mcp serve --config`.
	ConfigPath string
	// Executable is the Heracles binary to invoke for MCP serving.
	Executable string
}

// Inject prepares config.Workspace with Heracles's bundled skills and MCP
// server configuration for config.Provider. Pre-existing skills and
// configuration are left untouched. The returned cleanup function removes
// only what Inject added, restoring any files it modified.
func Inject(config Config) (cleanup func() error, err error) {
	var cleanups []func() error
	runCleanup := func() error {
		var firstErr error
		for index := len(cleanups) - 1; index >= 0; index-- {
			if cleanupErr := cleanups[index](); cleanupErr != nil && firstErr == nil {
				firstErr = cleanupErr
			}
		}
		return firstErr
	}

	skillsCleanup, err := injectSkills(config)
	if err != nil {
		_ = runCleanup()
		return nil, err
	}
	cleanups = append(cleanups, skillsCleanup)

	configCleanup, err := injectMCPConfig(config)
	if err != nil {
		_ = runCleanup()
		return nil, err
	}
	cleanups = append(cleanups, configCleanup)

	return runCleanup, nil
}

func injectSkills(config Config) (func() error, error) {
	bundledSkills, err := skills.Bundled()
	if err != nil {
		return nil, err
	}

	dir := skills.ProjectDir(config.Provider, config.Workspace)
	var created []string
	for _, skill := range bundledSkills {
		target := filepath.Join(dir, skill.Name)
		if _, err := os.Stat(target); err == nil {
			continue
		}
		results, err := skills.Install(dir, []skills.Skill{skill}, false)
		if err != nil {
			return nil, err
		}
		if len(results) == 1 && results[0].Installed {
			created = append(created, target)
		}
	}

	return func() error {
		for _, target := range created {
			if err := os.RemoveAll(target); err != nil {
				return err
			}
		}
		removeEmptyDirs(dir, config.Workspace)
		return nil
	}, nil
}

// codexProvider is the only registered provider that reads MCP server
// configuration from TOML; all others share a JSON .mcp.json convention.
const codexProvider = "codex"

func injectMCPConfig(config Config) (func() error, error) {
	if config.Provider == codexProvider {
		return injectCodexConfig(config)
	}
	return injectJSONConfig(config)
}

func injectJSONConfig(config Config) (func() error, error) {
	path := filepath.Join(config.Workspace, ".mcp.json")
	original, existed, err := readFileIfExists(path)
	if err != nil {
		return nil, err
	}

	doc := map[string]any{}
	if existed {
		if err := json.Unmarshal(original, &doc); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	}

	servers, ok := doc["mcpServers"].(map[string]any)
	if !ok {
		servers = map[string]any{}
	}
	if _, exists := servers["heracles"]; exists {
		return noopCleanup, nil
	}
	servers["heracles"] = map[string]any{
		"command": config.Executable,
		"args":    []string{"mcp", "serve", "--config", config.ConfigPath},
	}
	doc["mcpServers"] = servers

	contents, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	contents = append(contents, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		return nil, err
	}

	if !existed {
		return func() error {
			if err := os.Remove(path); err != nil {
				return err
			}
			removeEmptyDirs(filepath.Dir(path), config.Workspace)
			return nil
		}, nil
	}
	return func() error { return os.WriteFile(path, original, 0o644) }, nil
}

func injectCodexConfig(config Config) (func() error, error) {
	path := filepath.Join(config.Workspace, ".codex", "config.toml")
	original, existed, err := readFileIfExists(path)
	if err != nil {
		return nil, err
	}
	if existed && strings.Contains(string(original), "[mcp_servers.heracles]") {
		return noopCleanup, nil
	}

	section := fmt.Sprintf("[mcp_servers.heracles]\ncommand = %q\nargs = [%q, %q, %q, %q]\n",
		config.Executable, "mcp", "serve", "--config", config.ConfigPath)

	var contents []byte
	if existed {
		joined := strings.TrimRight(string(original), "\n") + "\n\n" + section
		contents = []byte(joined)
	} else {
		contents = []byte(section)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		return nil, err
	}

	if !existed {
		return func() error {
			if err := os.Remove(path); err != nil {
				return err
			}
			removeEmptyDirs(filepath.Dir(path), config.Workspace)
			return nil
		}, nil
	}
	return func() error { return os.WriteFile(path, original, 0o644) }, nil
}

func readFileIfExists(path string) ([]byte, bool, error) {
	contents, err := os.ReadFile(path)
	if err == nil {
		return contents, true, nil
	}
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	return nil, false, err
}

// removeEmptyDirs removes dir and its now-empty ancestors, stopping at and
// excluding stopAt. It silently stops at the first non-empty directory.
func removeEmptyDirs(dir, stopAt string) {
	stopAt = filepath.Clean(stopAt)
	for dir = filepath.Clean(dir); dir != stopAt; dir = filepath.Dir(dir) {
		if err := os.Remove(dir); err != nil {
			return
		}
	}
}

func noopCleanup() error { return nil }
