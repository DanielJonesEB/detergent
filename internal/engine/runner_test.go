package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteReadTrigger(t *testing.T) {
	dir := t.TempDir()
	hash := "abc123def456"

	if err := WriteTrigger(dir, hash); err != nil {
		t.Fatalf("WriteTrigger: %v", err)
	}

	gotHash, gotTime, err := ReadTrigger(dir)
	if err != nil {
		t.Fatalf("ReadTrigger: %v", err)
	}
	if gotHash != hash {
		t.Errorf("hash = %q, want %q", gotHash, hash)
	}
	if gotTime.IsZero() {
		t.Error("mod time should not be zero")
	}
}

func TestReadTriggerMissing(t *testing.T) {
	dir := t.TempDir()

	hash, modTime, err := ReadTrigger(dir)
	if err != nil {
		t.Fatalf("ReadTrigger: %v", err)
	}
	if hash != "" {
		t.Errorf("hash = %q, want empty", hash)
	}
	if !modTime.IsZero() {
		t.Errorf("mod time = %v, want zero", modTime)
	}
}

func TestTriggerModTimeAdvances(t *testing.T) {
	dir := t.TempDir()

	if err := WriteTrigger(dir, "first"); err != nil {
		t.Fatalf("WriteTrigger first: %v", err)
	}
	_, time1, err := ReadTrigger(dir)
	if err != nil {
		t.Fatalf("ReadTrigger first: %v", err)
	}

	// Ensure filesystem time granularity is respected
	time.Sleep(50 * time.Millisecond)

	if err := WriteTrigger(dir, "second"); err != nil {
		t.Fatalf("WriteTrigger second: %v", err)
	}
	_, time2, err := ReadTrigger(dir)
	if err != nil {
		t.Fatalf("ReadTrigger second: %v", err)
	}

	if !time2.After(time1) {
		t.Errorf("second mod time %v should be after first %v", time2, time1)
	}
}

func TestWriteReadPID(t *testing.T) {
	dir := t.TempDir()

	if err := WritePID(dir); err != nil {
		t.Fatalf("WritePID: %v", err)
	}

	got := ReadPID(dir)
	if got != os.Getpid() {
		t.Errorf("PID = %d, want %d", got, os.Getpid())
	}
}

func TestReadPIDMissing(t *testing.T) {
	dir := t.TempDir()

	got := ReadPID(dir)
	if got != 0 {
		t.Errorf("PID = %d, want 0", got)
	}
}

func TestIsRunnerAliveNoPID(t *testing.T) {
	dir := t.TempDir()

	if IsRunnerAlive(dir) {
		t.Error("expected false when no PID file")
	}
}

func TestIsRunnerAliveStalePID(t *testing.T) {
	dir := t.TempDir()

	// Write a PID that almost certainly doesn't exist
	pidPath := PIDPath(dir)
	os.MkdirAll(filepath.Dir(pidPath), 0755)
	os.WriteFile(pidPath, []byte("999999999\n"), 0644)

	if IsRunnerAlive(dir) {
		t.Error("expected false for non-existent PID")
	}
}

func TestIsRunnerAliveCurrentProcess(t *testing.T) {
	dir := t.TempDir()

	if err := WritePID(dir); err != nil {
		t.Fatalf("WritePID: %v", err)
	}

	if !IsRunnerAlive(dir) {
		t.Error("expected true for current process PID")
	}
}

func TestRemovePID(t *testing.T) {
	dir := t.TempDir()

	if err := WritePID(dir); err != nil {
		t.Fatalf("WritePID: %v", err)
	}

	RemovePID(dir)

	if _, err := os.Stat(PIDPath(dir)); !os.IsNotExist(err) {
		t.Error("PID file should not exist after RemovePID")
	}
}

func TestTriggerPath(t *testing.T) {
	got := TriggerPath("/repo")
	want := filepath.Join("/repo", ".line", "trigger")
	if got != want {
		t.Errorf("TriggerPath = %q, want %q", got, want)
	}
}

func TestPIDPath(t *testing.T) {
	got := PIDPath("/repo")
	want := filepath.Join("/repo", ".line", "runner.pid")
	if got != want {
		t.Errorf("PIDPath = %q, want %q", got, want)
	}
}
