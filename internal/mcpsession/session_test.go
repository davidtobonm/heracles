package mcpsession_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidtobonm/heracles/internal/mcpsession"
)

func TestInjectWritesSkillsAndMCPConfigThenCleanupRemovesThem(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	config := mcpsession.Config{
		Provider:   "claude",
		Workspace:  workspace,
		ConfigPath: "/project/heracles.yaml",
		Executable: "/usr/local/bin/heracles",
	}

	cleanup, err := mcpsession.Inject(config)
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}

	skillPath := filepath.Join(workspace, ".claude", "skills", "grill-with-docs", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("Stat(%s) error = %v, want injected skill", skillPath, err)
	}

	mcpPath := filepath.Join(workspace, ".mcp.json")
	contents, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", mcpPath, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(contents, &doc); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	servers, ok := doc["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers = %#v, want a map", doc["mcpServers"])
	}
	heracles, ok := servers["heracles"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers.heracles = %#v, want a map", servers["heracles"])
	}
	if heracles["command"] != "/usr/local/bin/heracles" {
		t.Errorf("command = %v, want heracles executable", heracles["command"])
	}
	args, ok := heracles["args"].([]any)
	if !ok || len(args) != 4 || args[3] != "/project/heracles.yaml" {
		t.Errorf("args = %#v, want mcp serve --config /project/heracles.yaml", heracles["args"])
	}

	if err := cleanup(); err != nil {
		t.Fatalf("cleanup() error = %v", err)
	}
	if _, err := os.Stat(skillPath); !os.IsNotExist(err) {
		t.Errorf("Stat(%s) error = %v, want removed after cleanup", skillPath, err)
	}
	if _, err := os.Stat(mcpPath); !os.IsNotExist(err) {
		t.Errorf("Stat(%s) error = %v, want removed after cleanup", mcpPath, err)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".claude")); !os.IsNotExist(err) {
		t.Errorf(".claude directory = %v, want removed once empty after cleanup", err)
	}
}

func TestInjectDoesNotOverwriteExistingSkillOrMCPConfig(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()

	// A locally modified skill must survive injection.
	existingSkillDir := filepath.Join(workspace, ".claude", "skills", "grill-with-docs")
	if err := os.MkdirAll(existingSkillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(existingSkillDir, "SKILL.md"), []byte("local edits"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// An existing .mcp.json with unrelated servers and its own heracles entry
	// must survive untouched.
	existingMCP := `{"mcpServers":{"other":{"command":"other-tool"},"heracles":{"command":"/custom/heracles","args":["mcp","serve"]}}}`
	mcpPath := filepath.Join(workspace, ".mcp.json")
	if err := os.WriteFile(mcpPath, []byte(existingMCP), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	config := mcpsession.Config{
		Provider:   "claude",
		Workspace:  workspace,
		ConfigPath: "/project/heracles.yaml",
		Executable: "/usr/local/bin/heracles",
	}

	cleanup, err := mcpsession.Inject(config)
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}

	contents, err := os.ReadFile(filepath.Join(existingSkillDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(contents) != "local edits" {
		t.Errorf("SKILL.md = %q, want local edits preserved", contents)
	}

	after, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(after) != existingMCP {
		t.Errorf(".mcp.json = %s, want unchanged", after)
	}

	if err := cleanup(); err != nil {
		t.Fatalf("cleanup() error = %v", err)
	}

	// Cleanup must not remove the pre-existing skill or config.
	if _, err := os.Stat(filepath.Join(existingSkillDir, "SKILL.md")); err != nil {
		t.Errorf("Stat() error = %v, want pre-existing skill preserved", err)
	}
	after, err = os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(after) != existingMCP {
		t.Errorf(".mcp.json = %s, want unchanged after cleanup", after)
	}
}

func TestInjectMergesIntoExistingMCPConfigAndRestoresOnCleanup(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	existingMCP := `{"mcpServers":{"other":{"command":"other-tool"}}}`
	mcpPath := filepath.Join(workspace, ".mcp.json")
	if err := os.WriteFile(mcpPath, []byte(existingMCP), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	config := mcpsession.Config{
		Provider:   "claude",
		Workspace:  workspace,
		ConfigPath: "/project/heracles.yaml",
		Executable: "/usr/local/bin/heracles",
	}

	cleanup, err := mcpsession.Inject(config)
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}

	merged, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(merged, &doc); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	servers := doc["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Errorf("mcpServers = %#v, want pre-existing entry preserved", servers)
	}
	if _, ok := servers["heracles"]; !ok {
		t.Errorf("mcpServers = %#v, want heracles entry added", servers)
	}

	if err := cleanup(); err != nil {
		t.Fatalf("cleanup() error = %v", err)
	}
	restored, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var restoredDoc map[string]any
	if err := json.Unmarshal(restored, &restoredDoc); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	restoredServers := restoredDoc["mcpServers"].(map[string]any)
	if _, ok := restoredServers["heracles"]; ok {
		t.Errorf("mcpServers = %#v, want heracles entry removed by cleanup", restoredServers)
	}
	if _, ok := restoredServers["other"]; !ok {
		t.Errorf("mcpServers = %#v, want pre-existing entry preserved by cleanup", restoredServers)
	}
}

func TestInjectForCodexWritesCodexConfigToml(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	config := mcpsession.Config{
		Provider:   "codex",
		Workspace:  workspace,
		ConfigPath: "/project/heracles.yaml",
		Executable: "/usr/local/bin/heracles",
	}

	cleanup, err := mcpsession.Inject(config)
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}

	tomlPath := filepath.Join(workspace, ".codex", "config.toml")
	contents, err := os.ReadFile(tomlPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", tomlPath, err)
	}
	if !strings.Contains(string(contents), "[mcp_servers.heracles]") {
		t.Errorf("config.toml = %s, want [mcp_servers.heracles] section", contents)
	}
	if !strings.Contains(string(contents), "/project/heracles.yaml") {
		t.Errorf("config.toml = %s, want heracles config path", contents)
	}

	if err := cleanup(); err != nil {
		t.Fatalf("cleanup() error = %v", err)
	}
	if _, err := os.Stat(tomlPath); !os.IsNotExist(err) {
		t.Errorf("Stat(%s) error = %v, want removed after cleanup", tomlPath, err)
	}
}
