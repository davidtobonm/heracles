package install_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/davidtobonm/heracles/internal/install"
)

func env(goos string, values map[string]string, home string) install.Environment {
	return install.Environment{
		GOOS:    goos,
		HomeDir: home,
		Getenv: func(key string) string {
			return values[key]
		},
	}
}

func TestDefaultDirUserUnixUsesHomeLocalBin(t *testing.T) {
	t.Parallel()

	dir, err := install.DefaultDir(install.ScopeUser, env("linux", nil, "/home/dev"))
	if err != nil {
		t.Fatalf("DefaultDir() error = %v", err)
	}
	if want := filepath.Join("/home/dev", ".local", "bin"); dir != want {
		t.Errorf("DefaultDir() = %q, want %q", dir, want)
	}
}

func TestDefaultDirUserUnixPrefersXDGBinHome(t *testing.T) {
	t.Parallel()

	dir, err := install.DefaultDir(install.ScopeUser, env("darwin", map[string]string{"XDG_BIN_HOME": "/opt/bin"}, "/home/dev"))
	if err != nil {
		t.Fatalf("DefaultDir() error = %v", err)
	}
	if dir != "/opt/bin" {
		t.Errorf("DefaultDir() = %q, want %q", dir, "/opt/bin")
	}
}

func TestDefaultDirSystemUnix(t *testing.T) {
	t.Parallel()

	for _, goos := range []string{"linux", "darwin"} {
		dir, err := install.DefaultDir(install.ScopeSystem, env(goos, nil, "/home/dev"))
		if err != nil {
			t.Fatalf("DefaultDir(%s) error = %v", goos, err)
		}
		if dir != "/usr/local/bin" {
			t.Errorf("DefaultDir(%s) = %q, want %q", goos, dir, "/usr/local/bin")
		}
	}
}

func TestDefaultDirWindows(t *testing.T) {
	t.Parallel()

	userDir, err := install.DefaultDir(install.ScopeUser, env("windows", map[string]string{"LOCALAPPDATA": `C:\Users\dev\AppData\Local`}, `C:\Users\dev`))
	if err != nil {
		t.Fatalf("DefaultDir(user) error = %v", err)
	}
	if want := filepath.Join(`C:\Users\dev\AppData\Local`, "Heracles"); userDir != want {
		t.Errorf("DefaultDir(user) = %q, want %q", userDir, want)
	}

	systemDir, err := install.DefaultDir(install.ScopeSystem, env("windows", map[string]string{"ProgramFiles": `C:\Program Files`}, `C:\Users\dev`))
	if err != nil {
		t.Fatalf("DefaultDir(system) error = %v", err)
	}
	if want := filepath.Join(`C:\Program Files`, "Heracles"); systemDir != want {
		t.Errorf("DefaultDir(system) = %q, want %q", systemDir, want)
	}
}

func TestDefaultDirUnsupportedEnvironment(t *testing.T) {
	t.Parallel()

	if _, err := install.DefaultDir(install.ScopeUser, env("linux", nil, "")); err == nil {
		t.Fatal("DefaultDir() error = nil, want error for missing home directory")
	}

	if _, err := install.DefaultDir(install.ScopeUser, env("windows", nil, "")); err == nil {
		t.Fatal("DefaultDir() error = nil, want error for missing LOCALAPPDATA")
	}
}

func TestResolvePrefersOverrideDirectory(t *testing.T) {
	t.Parallel()

	target, err := install.Resolve(install.ScopeUser, "/custom/dir", env("linux", nil, "/home/dev"))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if target.Dir != "/custom/dir" {
		t.Errorf("Resolve() Dir = %q, want %q", target.Dir, "/custom/dir")
	}
	if want := filepath.Join("/custom/dir", "heracles"); target.Path != want {
		t.Errorf("Resolve() Path = %q, want %q", target.Path, want)
	}
}

func TestResolveWindowsBinaryName(t *testing.T) {
	t.Parallel()

	target, err := install.Resolve(install.ScopeUser, "", env("windows", map[string]string{"LOCALAPPDATA": `C:\Users\dev\AppData\Local`}, `C:\Users\dev`))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if filepath.Base(target.Path) != "heracles.exe" {
		t.Errorf("Resolve() Path = %q, want suffix heracles.exe", target.Path)
	}
}

func TestInstallCopiesExecutableBinary(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "heracles-build")
	contents := []byte("#!/bin/sh\necho heracles\n")
	if err := os.WriteFile(sourcePath, contents, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	targetDir := filepath.Join(t.TempDir(), "nested", "bin")
	target := install.Target{Dir: targetDir, Path: filepath.Join(targetDir, "heracles")}

	if err := install.Install(sourcePath, target); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	installed, err := os.ReadFile(target.Path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(installed) != string(contents) {
		t.Errorf("installed contents = %q, want %q", installed, contents)
	}

	info, err := os.Stat(target.Path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("installed file mode = %v, want executable bit set", info.Mode())
	}
}

func TestInstallOverwritesExistingBinary(t *testing.T) {
	t.Parallel()

	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "heracles-build")
	if err := os.WriteFile(sourcePath, []byte("new"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	targetDir := t.TempDir()
	target := install.Target{Dir: targetDir, Path: filepath.Join(targetDir, "heracles")}
	if err := os.WriteFile(target.Path, []byte("old"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := install.Install(sourcePath, target); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	installed, err := os.ReadFile(target.Path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(installed) != "new" {
		t.Errorf("installed contents = %q, want %q", installed, "new")
	}
}

func TestOnPath(t *testing.T) {
	t.Parallel()

	unixEnv := env("linux", map[string]string{"PATH": "/usr/bin:/home/dev/.local/bin:/usr/local/bin"}, "/home/dev")
	if !install.OnPath("/home/dev/.local/bin", unixEnv) {
		t.Error("OnPath() = false, want true for directory present on PATH")
	}
	if install.OnPath("/opt/heracles/bin", unixEnv) {
		t.Error("OnPath() = true, want false for directory absent from PATH")
	}

	windowsEnv := env("windows", map[string]string{"PATH": `C:\Windows\System32;C:\Users\dev\AppData\Local\Heracles`}, `C:\Users\dev`)
	if !install.OnPath(`C:\Users\dev\AppData\Local\Heracles`, windowsEnv) {
		t.Error("OnPath() = false, want true for directory present on Windows PATH")
	}
}
