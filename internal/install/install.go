// Package install resolves and writes Heracles binaries into user and system command locations.
package install

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Scope selects a user-level or system-level install location.
type Scope string

const (
	// ScopeUser installs into a per-user command location.
	ScopeUser Scope = "user"
	// ScopeSystem installs into a machine-wide command location.
	ScopeSystem Scope = "system"
)

// Environment abstracts OS-specific lookups so install locations are deterministic to test.
type Environment struct {
	GOOS    string
	Getenv  func(string) string
	HomeDir string
}

func (env Environment) getenv(key string) string {
	if env.Getenv == nil {
		return ""
	}
	return env.Getenv(key)
}

// BinaryName returns the platform-specific executable name.
func BinaryName(goos string) string {
	if goos == "windows" {
		return "heracles.exe"
	}
	return "heracles"
}

// DefaultDir resolves the default install directory for scope on the given environment.
func DefaultDir(scope Scope, env Environment) (string, error) {
	if env.GOOS == "windows" {
		switch scope {
		case ScopeUser:
			if dir := env.getenv("LOCALAPPDATA"); dir != "" {
				return filepath.Join(dir, "Heracles"), nil
			}
			return "", errors.New("LOCALAPPDATA is not set")
		case ScopeSystem:
			if dir := env.getenv("ProgramFiles"); dir != "" {
				return filepath.Join(dir, "Heracles"), nil
			}
			return "", errors.New("ProgramFiles is not set")
		default:
			return "", fmt.Errorf("unsupported install scope %q", scope)
		}
	}

	switch scope {
	case ScopeUser:
		if dir := env.getenv("XDG_BIN_HOME"); dir != "" {
			return dir, nil
		}
		if env.HomeDir == "" {
			return "", errors.New("home directory is not set")
		}
		return filepath.Join(env.HomeDir, ".local", "bin"), nil
	case ScopeSystem:
		return "/usr/local/bin", nil
	default:
		return "", fmt.Errorf("unsupported install scope %q", scope)
	}
}

// Target is a resolved install location.
type Target struct {
	Dir  string
	Path string
}

// Resolve computes the install target for scope, an optional directory override, and environment.
func Resolve(scope Scope, overrideDir string, env Environment) (Target, error) {
	dir := overrideDir
	if dir == "" {
		resolved, err := DefaultDir(scope, env)
		if err != nil {
			return Target{}, err
		}
		dir = resolved
	}
	return Target{Dir: dir, Path: filepath.Join(dir, BinaryName(env.GOOS))}, nil
}

// Install copies the binary at sourcePath to target.Path, creating directories as needed and
// setting executable permissions.
func Install(sourcePath string, target Target) error {
	if err := os.MkdirAll(target.Dir, 0o755); err != nil {
		return fmt.Errorf("create install directory: %w", err)
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source binary: %w", err)
	}
	defer source.Close()

	temp, err := os.CreateTemp(target.Dir, "heracles-install-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if _, err := io.Copy(temp, source); err != nil {
		temp.Close()
		return fmt.Errorf("copy binary: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Chmod(tempPath, 0o755); err != nil {
		return fmt.Errorf("set executable permission: %w", err)
	}
	if err := os.Rename(tempPath, target.Path); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}
	return nil
}

// OnPath reports whether dir appears as an entry of the PATH environment variable.
func OnPath(dir string, env Environment) bool {
	separator := ":"
	if env.GOOS == "windows" {
		separator = ";"
	}
	for _, entry := range strings.Split(env.getenv("PATH"), separator) {
		if entry == dir {
			return true
		}
	}
	return false
}
