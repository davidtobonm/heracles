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
