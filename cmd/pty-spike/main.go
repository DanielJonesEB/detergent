// pty-spike: tests whether Claude Code streams output token-by-token when run
// interactively (without -p) inside a creack/pty PTY.
//
// Usage:
//
//	go run ./cmd/pty-spike
//
// What to look for in the output:
//   - Streaming: many small chunks arriving at different times (+Xms gaps between them)
//   - Batched: one big chunk after a long pause
//   - Raw bytes section shows ANSI escape sequences you'd need to strip
//   - Idle prompt "❯ " should appear twice (startup + after response)
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/creack/pty"
)

const (
	// idlePrompt is what Claude Code shows when it's waiting for input.
	// In raw PTY bytes, ❯ and the space may be separated by escape sequences,
	// so we search in STRIPPED text.
	idlePrompt = "❯ "
	// startupReady is a reliable signal that Claude is loaded and waiting.
	// It appears in the status bar once startup is complete.
	startupReady   = "bypass permissions on"
	startupTimeout = 90 * time.Second
	responseTimeout = 3 * time.Minute
	// Probe: something that produces a moderately long response so we can tell
	// whether it arrives in one blob or progressively.
	probe = "Count from 1 to 15, one number per line, nothing else."
)

type chunk struct {
	at   time.Duration
	data []byte
}

func main() {
	fmt.Println("=== PTY Interactive Mode Spike ===")
	fmt.Println("Testing: does claude stream token-by-token without -p?")
	fmt.Println()

	// Create a temp dir with a minimal git repo for Claude to work in.
	tmpDir, err := os.MkdirTemp("", "pty-spike-*")
	if err != nil {
		fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := initGitRepo(tmpDir); err != nil {
		fatalf("initGitRepo: %v", err)
	}

	// Start claude WITHOUT -p (interactive/TUI mode).
	cmd := exec.Command("claude", "--dangerously-skip-permissions")
	cmd.Dir = tmpDir
	// Claude Code refuses to launch inside another Claude Code session unless
	// CLAUDECODE is unset. Scrub it from the child environment.
	cmd.Env = filteredEnv("CLAUDECODE")
	fmt.Printf("Dir:     %s\n", tmpDir)
	fmt.Printf("Command: %s\n\n", strings.Join(cmd.Args, " "))

	t0 := time.Now()

	ptmx, err := pty.Start(cmd)
	if err != nil {
		fatalf("pty.Start: %v", err)
	}
	defer ptmx.Close()

	fmt.Printf("[+%5dms] process started pid=%d\n\n", msSince(t0), cmd.Process.Pid)

	// Shared output accumulator. The reader goroutine appends here and signals
	// readyCh whenever Claude appears ready for input, and promptCh whenever
	// the idle prompt (❯ ) appears after a response.
	var mu sync.Mutex
	var accum strings.Builder        // raw bytes
	var strippedAccum strings.Builder // ANSI-stripped text
	var chunks []chunk
	readyCh  := make(chan struct{}, 64) // startup complete
	promptCh := make(chan struct{}, 64) // idle prompt after response

	// reader goroutine
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				c := chunk{at: time.Since(t0), data: make([]byte, n)}
				copy(c.data, buf[:n])
				stripped := stripANSI(string(c.data))

				mu.Lock()
				chunks = append(chunks, c)
				accum.Write(c.data)
				strippedAccum.WriteString(stripped)

				// Startup: look for "bypass permissions on" in raw or stripped.
				if strings.Contains(stripped, startupReady) ||
					strings.Contains(string(c.data), startupReady) {
					select {
					case readyCh <- struct{}{}:
					default:
					}
				}
				// Idle prompt: search stripped (❯ may be split from space by escapes).
				if strings.Contains(stripped, idlePrompt) {
					select {
					case promptCh <- struct{}{}:
					default:
					}
				}
				mu.Unlock()

				fmt.Printf("[+%5dms] %4d bytes  %s\n",
					c.at.Milliseconds(), n, preview(c.data))
			}
			if err != nil {
				// EIO is normal when the PTY slave closes (process exited).
				return
			}
		}
	}()

	// Phase 1: wait for startup ("bypass permissions on" in status bar).
	fmt.Printf("--- Phase 1: waiting for startup (%q) ---\n", startupReady)
	t1start := time.Now()
	if !waitForPrompt(readyCh, readerDone, startupTimeout) {
		fmt.Printf("FAIL: startup signal not seen within %v\n", startupTimeout)
		dumpRaw(&mu, &accum)
		doCleanup(ptmx, cmd)
		return
	}
	startupMs := time.Since(t1start).Milliseconds()
	fmt.Printf("\n✓ Startup complete in %dms\n\n", startupMs)

	// Snapshot chunk count before sending probe so we can isolate response chunks.
	mu.Lock()
	chunksBefore := len(chunks)
	mu.Unlock()

	// Phase 2: send probe prompt.
	fmt.Printf("--- Phase 2: sending probe: %q ---\n\n", probe)
	tSend := time.Now()
	if _, err := io.WriteString(ptmx, probe+"\n"); err != nil {
		fmt.Printf("WARN: write to ptmx: %v\n", err)
	}

	// Phase 3: wait for response (❯  in stripped text signals Claude is idle again).
	fmt.Println("--- Phase 3: reading response ---")
	if !waitForPrompt(promptCh, readerDone, responseTimeout) {
		fmt.Printf("NOTE: ❯  not detected within %v (may still have streamed)\n", responseTimeout)
		fmt.Println("      Check chunk sizes below for streaming evidence.")
	}
	responseMs := time.Since(tSend).Milliseconds()
	fmt.Printf("\n✓ Response phase ended at %dms\n\n", responseMs)

	// Phase 4: SIGWINCH note.
	// Gas Town sends this to wake a detached PTY's event loop after connecting.
	// We didn't need it here, but to test: uncomment and move before Phase 3.
	//
	//   cmd.Process.Signal(syscall.SIGWINCH)
	//
	_ = syscall.SIGWINCH

	// Cleanup.
	doCleanup(ptmx, cmd)
	<-readerDone

	// Report.
	mu.Lock()
	totalChunks := len(chunks)
	responseChunks := make([]chunk, len(chunks)-chunksBefore)
	copy(responseChunks, chunks[chunksBefore:])
	mu.Unlock()

	fmt.Println("=== RESULTS ===")
	fmt.Printf("Startup time:    %dms\n", startupMs)
	fmt.Printf("Response time:   %dms\n", responseMs)
	fmt.Printf("Total chunks:    %d\n", totalChunks)
	fmt.Printf("Response chunks: %d\n", len(responseChunks))
	fmt.Println()

	switch {
	case len(responseChunks) > 3:
		fmt.Println("✓ STREAMING: multiple chunks arrived during response")
		fmt.Println("  → Interactive PTY mode improves log tailing; worth pursuing")
	case len(responseChunks) == 1:
		fmt.Println("✗ NOT STREAMING: full response arrived as one chunk")
		fmt.Println("  → Interactive mode batches same as -p; not worth the complexity")
	default:
		fmt.Printf("~ AMBIGUOUS: %d chunks — manual review needed\n", len(responseChunks))
	}

	fmt.Println()
	fmt.Println("--- Stripped output sample (first 1000 chars) ---")
	mu.Lock()
	stripped := strippedAccum.String()
	mu.Unlock()
	if len(stripped) > 1000 {
		stripped = stripped[:1000]
	}
	fmt.Println(stripped)

	fmt.Println()
	fmt.Println("--- Raw output sample (first 800 bytes) ---")
	dumpRaw(&mu, &accum)

	fmt.Println()
	fmt.Println("--- Response chunk sizes ---")
	for i, c := range responseChunks {
		fmt.Printf("  chunk %2d: +%5dms  %4d bytes  %s\n",
			i+1, c.at.Milliseconds(), len(c.data), preview(c.data))
	}
}

// waitForPrompt blocks until a prompt signal arrives, readerDone closes, or timeout.
func waitForPrompt(promptCh <-chan struct{}, done <-chan struct{}, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case <-promptCh:
			return true
		case <-done:
			return false
		case <-timer.C:
			return false
		}
	}
}

// preview returns a short readable representation of raw PTY bytes.
// Strips ANSI escapes and non-printable bytes; truncates at 80 chars.
func preview(b []byte) string {
	s := stripANSI(string(b))
	var sb strings.Builder
	for _, r := range s {
		if r == '\n' {
			sb.WriteString("↵")
		} else if r == '\r' {
			// skip carriage returns
		} else if utf8.ValidRune(r) && (r >= 32 || r == '\t') {
			sb.WriteRune(r)
		}
	}
	out := sb.String()
	if len(out) > 80 {
		out = out[:80] + "…"
	}
	return out
}

// stripANSI removes ANSI/VT100 escape sequences from s.
func stripANSI(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// CSI sequence: ESC [ <params> <final byte 0x40-0x7e>
			i += 2
			for i < len(s) && (s[i] < 0x40 || s[i] > 0x7e) {
				i++
			}
			if i < len(s) {
				i++ // consume final byte
			}
		} else if s[i] == '\x1b' && i+1 < len(s) {
			// Two-byte escape sequence
			i += 2
		} else {
			out.WriteByte(s[i])
			i++
		}
	}
	return out.String()
}

// dumpRaw prints a hex + ASCII view of the first 800 accumulated bytes.
func dumpRaw(mu *sync.Mutex, accum *strings.Builder) {
	mu.Lock()
	s := accum.String()
	mu.Unlock()
	if len(s) > 800 {
		s = s[:800]
	}
	fmt.Printf("Raw (%d bytes shown):\n", len(s))
	for i := 0; i < len(s); i += 16 {
		end := i + 16
		if end > len(s) {
			end = len(s)
		}
		row := s[i:end]
		fmt.Printf("  %04x  ", i)
		for j := 0; j < 16; j++ {
			if j < len(row) {
				fmt.Printf("%02x ", row[j])
			} else {
				fmt.Printf("   ")
			}
			if j == 7 {
				fmt.Printf(" ")
			}
		}
		fmt.Printf(" |")
		for _, b := range []byte(row) {
			if b >= 32 && b < 127 {
				fmt.Printf("%c", b)
			} else {
				fmt.Printf(".")
			}
		}
		fmt.Println("|")
	}
}

// initGitRepo creates a minimal git repo in dir so Claude has a workspace.
func initGitRepo(dir string) error {
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "spike@example.com"},
		{"git", "config", "user.name", "Spike"},
	}
	for _, args := range cmds {
		c := exec.Command(args[0], args[1:]...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			return fmt.Errorf("%v: %s", args, out)
		}
	}
	readme := fmt.Sprintf("# PTY Spike\n\nCreated: %s\n", time.Now().Format(time.RFC3339))
	if err := os.WriteFile(dir+"/README.md", []byte(readme), 0644); err != nil {
		return err
	}
	for _, args := range [][]string{
		{"git", "add", "README.md"},
		{"git", "commit", "-m", "initial"},
	} {
		c := exec.Command(args[0], args[1:]...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			return fmt.Errorf("%v: %s", args, out)
		}
	}
	return nil
}

// filteredEnv returns os.Environ() with the named variables removed.
func filteredEnv(exclude ...string) []string {
	excl := make(map[string]bool, len(exclude))
	for _, k := range exclude {
		excl[k] = true
	}
	var out []string
	for _, e := range os.Environ() {
		key := e
		if i := strings.IndexByte(e, '='); i >= 0 {
			key = e[:i]
		}
		if !excl[key] {
			out = append(out, e)
		}
	}
	return out
}

func doCleanup(ptmx *os.File, cmd *exec.Cmd) {
	if _, err := io.WriteString(ptmx, "/exit\n"); err == nil {
		time.Sleep(500 * time.Millisecond)
	}
	if cmd.Process != nil {
		cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.Wait()
}

func msSince(t time.Time) int64 {
	return time.Since(t).Milliseconds()
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "FATAL: "+format+"\n", args...)
	os.Exit(1)
}
