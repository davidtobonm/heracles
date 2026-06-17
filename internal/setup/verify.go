package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DetectVerification inspects a repository's build files for a verification
// command. confident is true when a recognized stack or aggregate command
// was found; commands is the ordered list of shell commands to run.
func DetectVerification(repoPath string) (commands []string, confident bool) {
	if command, ok := detectAggregateCommand(repoPath); ok {
		return []string{command}, true
	}

	if commands, ok := detectGoCommands(repoPath); ok {
		return commands, true
	}
	if commands, ok := detectNodeCommands(repoPath); ok {
		return commands, true
	}
	if commands, ok := detectPythonCommands(repoPath); ok {
		return commands, true
	}
	if commands, ok := detectRustCommands(repoPath); ok {
		return commands, true
	}

	return nil, false
}

func detectAggregateCommand(repoPath string) (string, bool) {
	if hasMakeTarget(filepath.Join(repoPath, "Makefile"), "check") {
		return "make check", true
	}
	if hasJustRecipe(filepath.Join(repoPath, "Justfile"), "check") {
		return "just check", true
	}
	if hasPackageScript(filepath.Join(repoPath, "package.json"), "check") {
		return nodeRunCommand(repoPath, "check"), true
	}
	return "", false
}

func detectGoCommands(repoPath string) ([]string, bool) {
	if !fileExists(filepath.Join(repoPath, "go.mod")) {
		return nil, false
	}
	return []string{"gofmt -l .", "go vet ./...", "go test ./..."}, true
}

func detectNodeCommands(repoPath string) ([]string, bool) {
	path := filepath.Join(repoPath, "package.json")
	scripts, ok := packageScripts(path)
	if !ok {
		return nil, false
	}

	var commands []string
	for _, name := range []string{"lint", "test", "build"} {
		if _, exists := scripts[name]; exists {
			commands = append(commands, nodeRunCommand(repoPath, name))
		}
	}
	if len(commands) == 0 {
		return nil, false
	}
	return commands, true
}

func detectPythonCommands(repoPath string) ([]string, bool) {
	hasPyproject := fileExists(filepath.Join(repoPath, "pyproject.toml"))
	hasRequirements := fileExists(filepath.Join(repoPath, "requirements.txt"))
	if !hasPyproject && !hasRequirements {
		return nil, false
	}

	var commands []string
	if hasPyproject {
		contents, err := os.ReadFile(filepath.Join(repoPath, "pyproject.toml"))
		if err == nil {
			text := string(contents)
			if strings.Contains(text, "ruff") {
				commands = append(commands, "ruff check .")
			}
			if strings.Contains(text, "pytest") {
				commands = append(commands, "pytest")
			}
		}
	}
	if len(commands) == 0 && hasRequirements {
		contents, err := os.ReadFile(filepath.Join(repoPath, "requirements.txt"))
		if err == nil {
			text := string(contents)
			if strings.Contains(text, "ruff") {
				commands = append(commands, "ruff check .")
			}
			if strings.Contains(text, "pytest") {
				commands = append(commands, "pytest")
			}
		}
	}
	if len(commands) == 0 {
		return nil, false
	}
	return commands, true
}

func detectRustCommands(repoPath string) ([]string, bool) {
	if !fileExists(filepath.Join(repoPath, "Cargo.toml")) {
		return nil, false
	}
	return []string{"cargo fmt --check", "cargo test"}, true
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DetectStack names the project's primary language/framework from common
// marker files, for use in human-readable text such as the Heracles Project
// Bootstrap proposal. It returns "" when no recognized stack is found.
func DetectStack(repoPath string) string {
	switch {
	case fileExists(filepath.Join(repoPath, "pubspec.yaml")):
		return "Dart/Flutter"
	case fileExists(filepath.Join(repoPath, "go.mod")):
		return "Go"
	case fileExists(filepath.Join(repoPath, "package.json")):
		return "Node.js"
	case fileExists(filepath.Join(repoPath, "pyproject.toml")), fileExists(filepath.Join(repoPath, "requirements.txt")):
		return "Python"
	case fileExists(filepath.Join(repoPath, "Cargo.toml")):
		return "Rust"
	case fileExists(filepath.Join(repoPath, "pom.xml")):
		return "Java (Maven)"
	case fileExists(filepath.Join(repoPath, "build.gradle")), fileExists(filepath.Join(repoPath, "build.gradle.kts")):
		return "Java/Kotlin (Gradle)"
	case fileExists(filepath.Join(repoPath, "Gemfile")):
		return "Ruby"
	case fileExists(filepath.Join(repoPath, "composer.json")):
		return "PHP"
	default:
		return ""
	}
}

// RepositoryHasFiles reports whether repoPath contains any tracked-looking
// file beyond version-control and Heracles housekeeping directories. A
// repository with no files yet (a brand-new, still-empty project) has no
// existing code for the Heracles Project Bootstrap to add checks to.
func RepositoryHasFiles(repoPath string) bool {
	entries, err := os.ReadDir(repoPath)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		switch entry.Name() {
		case ".git", ".heracles":
			continue
		default:
			return true
		}
	}
	return false
}

var makeTargetPattern = regexp.MustCompile(`(?m)^([A-Za-z0-9_.-]+):`)

func hasMakeTarget(path, target string) bool {
	contents, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, match := range makeTargetPattern.FindAllStringSubmatch(string(contents), -1) {
		for _, name := range strings.Fields(match[1]) {
			if name == target {
				return true
			}
		}
	}
	return false
}

var justRecipePattern = regexp.MustCompile(`(?m)^([A-Za-z0-9_-]+)( .*)?:`)

func hasJustRecipe(path, recipe string) bool {
	contents, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, match := range justRecipePattern.FindAllStringSubmatch(string(contents), -1) {
		if match[1] == recipe {
			return true
		}
	}
	return false
}

func packageScripts(path string) (map[string]string, bool) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var manifest struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(contents, &manifest); err != nil {
		return nil, false
	}
	if manifest.Scripts == nil {
		return nil, false
	}
	return manifest.Scripts, true
}

func hasPackageScript(path, script string) bool {
	scripts, ok := packageScripts(path)
	if !ok {
		return false
	}
	_, exists := scripts[script]
	return exists
}

func nodeRunCommand(repoPath, script string) string {
	switch {
	case fileExists(filepath.Join(repoPath, "pnpm-lock.yaml")):
		return "pnpm run " + script
	case fileExists(filepath.Join(repoPath, "yarn.lock")):
		return "yarn run " + script
	default:
		return "npm run " + script
	}
}
