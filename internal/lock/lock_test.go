package lock_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/davidtobonm/heracles/internal/lock"
)

func TestAcquireBlocksConcurrentLaborWhileProcessIsRunning(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "labor.lock")
	running := func(int) bool { return true }

	held, err := lock.Acquire(path, "labor-1", running)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if held.State().LaborID != "labor-1" {
		t.Errorf("State().LaborID = %q, want %q", held.State().LaborID, "labor-1")
	}

	if _, err := lock.Acquire(path, "labor-2", running); !errors.Is(err, lock.ErrHeld) {
		t.Fatalf("Acquire() error = %v, want %v", err, lock.ErrHeld)
	}
}

func TestAcquireReplacesStaleLockFromNonRunningProcess(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "labor.lock")
	notRunning := func(int) bool { return false }

	if _, err := lock.Acquire(path, "labor-1", notRunning); err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	held, err := lock.Acquire(path, "labor-2", notRunning)
	if err != nil {
		t.Fatalf("Acquire() error = %v, want stale lock replaced", err)
	}
	if held.State().LaborID != "labor-2" {
		t.Errorf("State().LaborID = %q, want %q", held.State().LaborID, "labor-2")
	}
}

func TestReleaseRemovesOwnLockButNotAReplacement(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "labor.lock")
	running := func(int) bool { return true }
	notRunning := func(int) bool { return false }

	held, err := lock.Acquire(path, "labor-1", running)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if err := held.Release(); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("lock file still exists after Release(), err = %v", err)
	}

	// A lock acquired after Release replaces the released lock; the original
	// Lock's Release must not remove the replacement.
	replacement, err := lock.Acquire(path, "labor-2", notRunning)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	if err := held.Release(); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("replacement lock file removed by stale Release(), err = %v", err)
	}
	if err := replacement.Release(); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
}

func TestAcquireRejectsEmptyLaborID(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "labor.lock")
	if _, err := lock.Acquire(path, "", func(int) bool { return false }); err == nil {
		t.Fatal("Acquire() error = nil, want error for empty labor ID")
	}
}

func TestDefaultProcessCheckerReportsCurrentProcessRunning(t *testing.T) {
	t.Parallel()

	if !lock.DefaultProcessChecker(os.Getpid()) {
		t.Error("DefaultProcessChecker(os.Getpid()) = false, want true for the running test process")
	}
}
