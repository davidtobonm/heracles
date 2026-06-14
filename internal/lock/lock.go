// Package lock enforces that only one active Labor mutates a project at a time.
package lock

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

// ErrHeld indicates another Labor currently holds the project lock.
var ErrHeld = errors.New("project Labor lock is held")

// State is the persisted content of a project Labor lock.
type State struct {
	PID        int       `json:"pid"`
	LaborID    string    `json:"labor_id"`
	Host       string    `json:"host"`
	AcquiredAt time.Time `json:"acquired_at"`
}

// ProcessChecker reports whether a local process with the given PID is
// running. Acquire uses it to decide whether an existing lock is stale.
type ProcessChecker func(pid int) bool

// Lock guards single-Labor mutation of a project. There is no force-unlock
// path: a lock can only be replaced by Acquire once ProcessChecker confirms
// the process that created it is no longer running.
type Lock struct {
	path  string
	state State
}

// State returns the persisted content of the held lock.
func (l *Lock) State() State {
	return l.state
}

// Acquire creates the project Labor lock at path for laborID. If a lock
// already exists at path and its process is still running according to
// checker, Acquire returns ErrHeld. If the existing lock's process is no
// longer running, Acquire treats it as stale, replaces it, and succeeds. A
// nil checker uses DefaultProcessChecker.
func Acquire(path, laborID string, checker ProcessChecker) (*Lock, error) {
	if laborID == "" {
		return nil, errors.New("project Labor lock requires a labor ID")
	}
	if checker == nil {
		checker = DefaultProcessChecker
	}

	if existing, err := readState(path); err == nil {
		if checker(existing.PID) {
			return nil, fmt.Errorf("%w: labor %q (pid %d)", ErrHeld, existing.LaborID, existing.PID)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	host, _ := os.Hostname()
	state := State{PID: os.Getpid(), LaborID: laborID, Host: host, AcquiredAt: time.Now().UTC()}
	if err := writeState(path, state); err != nil {
		return nil, err
	}
	return &Lock{path: path, state: state}, nil
}

// Release removes the lock file if it still records this process's PID.
// Release is idempotent: it is not an error to call it after the lock has
// already been replaced or removed.
func (l *Lock) Release() error {
	existing, err := readState(l.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if existing.PID != l.state.PID || existing.AcquiredAt != l.state.AcquiredAt {
		return nil
	}
	if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("release project Labor lock: %w", err)
	}
	return nil
}

// DefaultProcessChecker reports whether pid identifies a process currently
// running on this machine.
func DefaultProcessChecker(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		// os.FindProcess on Windows only succeeds for a running process.
		return true
	}
	return process.Signal(syscall.Signal(0)) == nil
}

func readState(path string) (State, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return State{}, err
	}
	var state State
	if err := json.Unmarshal(contents, &state); err != nil {
		return State{}, fmt.Errorf("decode project Labor lock: %w", err)
	}
	return state, nil
}

func writeState(path string, state State) error {
	contents, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode project Labor lock: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create project Labor lock directory: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".heracles-lock-*")
	if err != nil {
		return fmt.Errorf("create project Labor lock temporary file: %w", err)
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err := file.Write(append(contents, '\n')); err != nil {
		_ = file.Close()
		return fmt.Errorf("write project Labor lock temporary file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close project Labor lock temporary file: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("commit project Labor lock: %w", err)
	}
	return nil
}
