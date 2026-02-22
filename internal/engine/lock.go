package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/re-cinq/assembly-line/internal/fileutil"
)

// errLockHeld is returned when another line run process holds the lock.
var errLockHeld = errors.New("another line run is already running")

// IsLockHeld reports whether err indicates the run lock is already held.
func IsLockHeld(err error) bool {
	return errors.Is(err, errLockHeld)
}

// lockFilePath returns the path to the run lock file.
func lockFilePath(repoDir string) string {
	return filepath.Join(fileutil.LineDir(repoDir), "run.lock")
}

// AcquireRunLock attempts to acquire an exclusive file lock for line run.
// Returns an unlock function on success. The lock is released when the
// returned function is called or when the process exits.
// Returns errLockHeld if another process already holds the lock.
func AcquireRunLock(repoDir string) (unlock func(), err error) {
	lockPath := lockFilePath(repoDir)
	if err := fileutil.EnsureDir(filepath.Dir(lockPath)); err != nil {
		return nil, fmt.Errorf("creating lock directory: %w", err)
	}

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("opening lock file: %w", err)
	}

	// Try non-blocking exclusive lock
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("%w", errLockHeld)
	}

	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}
