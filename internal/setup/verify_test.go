package setup_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/davidtobonm/heracles/internal/setup"
)

func writeFile(t *testing.T, dir, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestDetectVerificationMakeCheckTarget(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "Makefile", "check: lint test\n\t@true\n\nlint:\n\t@true\n")
	writeFile(t, dir, "go.mod", "module example.com/widget\n\ngo 1.24\n")

	commands, confident := setup.DetectVerification(dir)
	if !confident {
		t.Fatalf("DetectVerification() confident = false, want true")
	}
	if want := []string{"make check"}; !reflect.DeepEqual(commands, want) {
		t.Errorf("DetectVerification() = %v, want %v", commands, want)
	}
}

func TestDetectVerificationJustCheckRecipe(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "Justfile", "check:\n    cargo test\n")

	commands, confident := setup.DetectVerification(dir)
	if !confident {
		t.Fatalf("DetectVerification() confident = false, want true")
	}
	if want := []string{"just check"}; !reflect.DeepEqual(commands, want) {
		t.Errorf("DetectVerification() = %v, want %v", commands, want)
	}
}

func TestDetectVerificationPackageJSONCheckScript(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"scripts": {"check": "vitest run", "lint": "eslint ."}}`)

	commands, confident := setup.DetectVerification(dir)
	if !confident {
		t.Fatalf("DetectVerification() confident = false, want true")
	}
	if want := []string{"npm run check"}; !reflect.DeepEqual(commands, want) {
		t.Errorf("DetectVerification() = %v, want %v", commands, want)
	}
}

func TestDetectVerificationPackageJSONCheckScriptWithYarn(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"scripts": {"check": "vitest run"}}`)
	writeFile(t, dir, "yarn.lock", "")

	commands, _ := setup.DetectVerification(dir)
	if want := []string{"yarn run check"}; !reflect.DeepEqual(commands, want) {
		t.Errorf("DetectVerification() = %v, want %v", commands, want)
	}
}

func TestDetectVerificationGoModFallback(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/widget\n\ngo 1.24\n")

	commands, confident := setup.DetectVerification(dir)
	if !confident {
		t.Fatalf("DetectVerification() confident = false, want true")
	}
	if want := []string{"gofmt -l .", "go vet ./...", "go test ./..."}; !reflect.DeepEqual(commands, want) {
		t.Errorf("DetectVerification() = %v, want %v", commands, want)
	}
}

func TestDetectVerificationPackageJSONScriptsFallback(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"scripts": {"lint": "eslint .", "test": "vitest run", "build": "vite build"}}`)

	commands, confident := setup.DetectVerification(dir)
	if !confident {
		t.Fatalf("DetectVerification() confident = false, want true")
	}
	want := []string{"npm run lint", "npm run test", "npm run build"}
	if !reflect.DeepEqual(commands, want) {
		t.Errorf("DetectVerification() = %v, want %v", commands, want)
	}
}

func TestDetectVerificationPyprojectWithRuffAndPytest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", "[tool.ruff]\nline-length = 100\n\n[tool.pytest.ini_options]\n")

	commands, confident := setup.DetectVerification(dir)
	if !confident {
		t.Fatalf("DetectVerification() confident = false, want true")
	}
	want := []string{"ruff check .", "pytest"}
	if !reflect.DeepEqual(commands, want) {
		t.Errorf("DetectVerification() = %v, want %v", commands, want)
	}
}

func TestDetectVerificationRequirementsTxtWithPytest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "requirements.txt", "pytest==8.0.0\nrequests==2.31.0\n")

	commands, confident := setup.DetectVerification(dir)
	if !confident {
		t.Fatalf("DetectVerification() confident = false, want true")
	}
	want := []string{"pytest"}
	if !reflect.DeepEqual(commands, want) {
		t.Errorf("DetectVerification() = %v, want %v", commands, want)
	}
}

func TestDetectVerificationCargoToml(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "Cargo.toml", "[package]\nname = \"widget\"\nversion = \"0.1.0\"\n")

	commands, confident := setup.DetectVerification(dir)
	if !confident {
		t.Fatalf("DetectVerification() confident = false, want true")
	}
	want := []string{"cargo fmt --check", "cargo test"}
	if !reflect.DeepEqual(commands, want) {
		t.Errorf("DetectVerification() = %v, want %v", commands, want)
	}
}

func TestDetectVerificationUnrecognizedStack(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# Widget\n")

	commands, confident := setup.DetectVerification(dir)
	if confident {
		t.Fatalf("DetectVerification() confident = true, want false")
	}
	if commands != nil {
		t.Errorf("DetectVerification() commands = %v, want nil", commands)
	}
}

func TestDetectStack(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		file  string
		stack string
	}{
		{name: "flutter", file: "pubspec.yaml", stack: "Dart/Flutter"},
		{name: "go", file: "go.mod", stack: "Go"},
		{name: "node", file: "package.json", stack: "Node.js"},
		{name: "python pyproject", file: "pyproject.toml", stack: "Python"},
		{name: "python requirements", file: "requirements.txt", stack: "Python"},
		{name: "rust", file: "Cargo.toml", stack: "Rust"},
		{name: "maven", file: "pom.xml", stack: "Java (Maven)"},
		{name: "gradle", file: "build.gradle", stack: "Java/Kotlin (Gradle)"},
		{name: "ruby", file: "Gemfile", stack: "Ruby"},
		{name: "php", file: "composer.json", stack: "PHP"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeFile(t, dir, testCase.file, "")
			if got := setup.DetectStack(dir); got != testCase.stack {
				t.Errorf("DetectStack() = %q, want %q", got, testCase.stack)
			}
		})
	}
}

func TestDetectStackUnrecognized(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# Widget\n")

	if got := setup.DetectStack(dir); got != "" {
		t.Errorf("DetectStack() = %q, want empty for unrecognized stack", got)
	}
}

func TestRepositoryHasFilesIgnoresVersionControlAndHeraclesDirectories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("Mkdir(.git) error = %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, ".heracles"), 0o755); err != nil {
		t.Fatalf("Mkdir(.heracles) error = %v", err)
	}

	if setup.RepositoryHasFiles(dir) {
		t.Error("RepositoryHasFiles() = true, want false for a repository with only .git/.heracles")
	}

	writeFile(t, dir, "main.go", "package main\n")
	if !setup.RepositoryHasFiles(dir) {
		t.Error("RepositoryHasFiles() = false, want true once a real file exists")
	}
}
