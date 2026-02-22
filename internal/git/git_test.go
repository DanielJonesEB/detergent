package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunSanitizesGitEnvVars(t *testing.T) {
	// If GIT_DIR or GIT_WORK_TREE leak into child git processes
	// (e.g. inherited from a post-commit hook), worktree operations
	// can target the wrong repository or fail with ENOTDIR.
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %s: %v", out, err)
	}
	// Write a file and commit so rev-parse HEAD works
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	execCmd := exec.Command("git", "add", ".")
	execCmd.Dir = dir
	execCmd.Run()
	execCmd = exec.Command("git", "commit", "-m", "init", "--no-gpg-sign")
	execCmd.Dir = dir
	execCmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=t@t",
	)
	execCmd.Run()

	// Poison the environment with GIT_DIR pointing to a bogus location.
	// If run() doesn't sanitize these, the child git process will fail.
	t.Setenv("GIT_DIR", "/nonexistent/bogus/.git")
	t.Setenv("GIT_WORK_TREE", "/nonexistent/bogus")
	t.Setenv("GIT_INDEX_FILE", "/nonexistent/bogus/index")

	repo := NewRepo(dir)
	// This should succeed because run() strips the poisoned env vars.
	_, err := repo.run("rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("run() should sanitize GIT_DIR/GIT_WORK_TREE but got error: %v", err)
	}
}

func TestIsTransient(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want bool
	}{
		{"index open failed", "fatal: .git/index: index file open failed: Not a directory", true},
		{"index lock", "Unable to create '/repo/.git/index.lock': File exists.", true},
		{"ref lock", "error: cannot lock ref 'refs/heads/main': is at abc123 but expected def456", true},
		{"unknown revision", "fatal: ambiguous argument 'nonexistent': unknown revision", false},
		{"branch exists", "fatal: A branch named 'feature' already exists.", false},
		{"not a repo", "fatal: not a git repository (or any of the parent directories): .git", false},
		{"empty string", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransient(tt.msg); got != tt.want {
				t.Errorf("isTransient(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}

func TestRunNoRetryOnNonTransient(t *testing.T) {
	// Set up a real git repo so the error comes from an actual git command.
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %s: %v", out, err)
	}

	// Track sleep calls to verify no retries happen.
	var sleepCount int64
	orig := sleepFunc
	sleepFunc = func(d time.Duration) { atomic.AddInt64(&sleepCount, 1) }
	t.Cleanup(func() { sleepFunc = orig })

	repo := NewRepo(dir)
	// rev-parse on a nonexistent ref is a non-transient error.
	_, err := repo.run("rev-parse", "nonexistent-ref-abc123")
	if err == nil {
		t.Fatal("expected error for nonexistent ref, got nil")
	}
	if atomic.LoadInt64(&sleepCount) != 0 {
		t.Errorf("expected 0 retries for non-transient error, got %d", sleepCount)
	}
}

func TestRunRetriesTransientThenSucceeds(t *testing.T) {
	// Create a helper script that fails transiently on the first call,
	// then succeeds on the second.
	dir := t.TempDir()

	// The script uses a marker file to track whether it's been called before.
	marker := filepath.Join(dir, "marker")
	script := filepath.Join(dir, "git")
	err := os.WriteFile(script, []byte(`#!/bin/sh
if [ ! -f "`+marker+`" ]; then
    touch "`+marker+`"
    echo "fatal: .git/index: index file open failed: Not a directory" >&2
    exit 128
fi
echo "success"
`), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	// Override PATH so our fake git is found first.
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)
	t.Cleanup(func() { os.Setenv("PATH", origPath) })

	// No real sleeping.
	var sleepCount int64
	origSleep := sleepFunc
	sleepFunc = func(d time.Duration) { atomic.AddInt64(&sleepCount, 1) }
	t.Cleanup(func() { sleepFunc = origSleep })

	repo := NewRepo(dir)
	out, err := repo.run("status")
	if err != nil {
		t.Fatalf("expected success after retry, got: %v", err)
	}
	if strings.TrimSpace(out) != "success" {
		t.Errorf("unexpected output: %q", out)
	}
	if n := atomic.LoadInt64(&sleepCount); n != 1 {
		t.Errorf("expected 1 retry sleep, got %d", n)
	}
}

func TestRunExhaustsRetries(t *testing.T) {
	// Create a script that always fails with a transient error.
	dir := t.TempDir()
	script := filepath.Join(dir, "git")
	err := os.WriteFile(script, []byte(`#!/bin/sh
echo "fatal: .git/index: index file open failed: Not a directory" >&2
exit 128
`), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	origPath := os.Getenv("PATH")
	os.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)
	t.Cleanup(func() { os.Setenv("PATH", origPath) })

	var sleepCount int64
	origSleep := sleepFunc
	sleepFunc = func(d time.Duration) { atomic.AddInt64(&sleepCount, 1) }
	t.Cleanup(func() { sleepFunc = origSleep })

	repo := NewRepo(dir)
	_, err = repo.run("status")
	if err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}
	if !strings.Contains(err.Error(), "index file open failed") {
		t.Errorf("error should contain transient message, got: %v", err)
	}
	// Should have slept retryMaxAttempts-1 times (between attempts).
	expected := int64(retryMaxAttempts - 1)
	if n := atomic.LoadInt64(&sleepCount); n != expected {
		t.Errorf("expected %d retry sleeps, got %d", expected, n)
	}
}
