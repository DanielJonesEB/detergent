package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireRunLock(t *testing.T) {
	dir := t.TempDir()

	// First acquisition should succeed
	unlock, err := AcquireRunLock(dir)
	if err != nil {
		t.Fatalf("first AcquireRunLock should succeed: %v", err)
	}

	// Second acquisition should fail (already locked)
	_, err = AcquireRunLock(dir)
	if err == nil {
		t.Fatal("second AcquireRunLock should fail while first lock is held")
	}
	if !IsLockHeld(err) {
		t.Errorf("error should indicate lock is held, got: %v", err)
	}

	// Release the lock
	unlock()

	// Third acquisition should succeed after release
	unlock2, err := AcquireRunLock(dir)
	if err != nil {
		t.Fatalf("AcquireRunLock after release should succeed: %v", err)
	}
	unlock2()
}

func TestAcquireRunLockCleansUpStaleLock(t *testing.T) {
	dir := t.TempDir()

	// Create a stale lock file (no process holding the flock)
	lockPath := lockFilePath(dir)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Should succeed because the flock is not held even though the file exists
	unlock, err := AcquireRunLock(dir)
	if err != nil {
		t.Fatalf("AcquireRunLock should succeed on stale lock file: %v", err)
	}
	unlock()
}
