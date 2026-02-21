package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	pollingInterval = 500 * time.Millisecond
	idleTimeout     = 10 * time.Second
	maxWaitTime     = 2 * time.Minute
)

func main() {
	sessionName := flag.String("session", "line-spike", "tmux session name")
	contextFile := flag.String("context", "", "path to context file (required)")
	command := flag.String("cmd", "claude", "command to run (default: claude)")
	flag.Parse()

	if *contextFile == "" {
		log.Fatal("--context is required")
	}

	if _, err := os.Stat(*contextFile); os.IsNotExist(err) {
		log.Fatalf("context file not found: %s", *contextFile)
	}

	ctx, cancel := context.WithTimeout(context.Background(), maxWaitTime)
	defer cancel()

	// Trap signals for graceful cleanup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		fmt.Printf("\nReceived signal: %v\n", sig)
		_ = killSession(*sessionName)
		os.Exit(1)
	}()

	// Start the tmux session
	fmt.Printf("Starting tmux session: %s\n", *sessionName)
	if err := startSession(*sessionName, *command, *contextFile); err != nil {
		log.Fatalf("failed to start session: %v", err)
	}
	defer func() { _ = killSession(*sessionName) }()

	fmt.Println("\nStarting capture-pane polling (500ms interval)...")
	fmt.Println("=== OUTPUT ===")

	var lastLines []string
	var prevOutput string
	idleTime := time.Time{}
	streaming := false
	lastStreamTime := time.Time{}

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\n=== TIMEOUT ===")
			fmt.Println("Max wait time exceeded")
			return
		default:
		}

		// Poll session status
		alive, err := isSessionAlive(*sessionName)
		if err != nil {
			fmt.Printf("Error checking session: %v\n", err)
			time.Sleep(pollingInterval)
			continue
		}

		if !alive {
			// Session ended - capture final output
			output, _ := capturePane(*sessionName)
			fmt.Println(output)
			fmt.Println("\n=== SESSION EXITED ===")
			fmt.Printf("Session %s has completed\n", *sessionName)
			summarizeFindings(streaming, lastStreamTime)
			return
		}

		// Capture output
		output, err := capturePane(*sessionName)
		if err != nil {
			fmt.Printf("Error capturing pane: %v\n", err)
			time.Sleep(pollingInterval)
			continue
		}

		// Detect if output changed (streaming)
		if output != prevOutput {
			now := time.Now()
			if !streaming {
				streaming = true
				fmt.Printf("[%s] Output streaming started\n", now.Format("15:04:05.000"))
			}
			lastStreamTime = now
			idleTime = time.Time{}

			// Print only new lines
			newLines := extractNewLines(output, lastLines)
			for _, line := range newLines {
				fmt.Println(line)
			}
			lastLines = strings.Split(strings.TrimSpace(output), "\n")
			prevOutput = output
		} else {
			// Output unchanged
			if streaming && idleTime.IsZero() {
				idleTime = time.Now()
			}
			if !idleTime.IsZero() && time.Since(idleTime) > idleTimeout {
				fmt.Printf("\n[%s] Output idle for %v - assuming completion\n", time.Now().Format("15:04:05.000"), idleTimeout)
				fmt.Println("\n=== SESSION IDLE ===")
				summarizeFindings(streaming, lastStreamTime)
				return
			}
		}

		// Check for idle prompt (❯ )
		if detectPrompt(output) {
			fmt.Printf("\n[%s] Idle prompt detected (❯ )\n", time.Now().Format("15:04:05.000"))
			fmt.Println("\n=== IDLE PROMPT ===")
			summarizeFindings(streaming, lastStreamTime)
			return
		}

		time.Sleep(pollingInterval)
	}
}

func startSession(name, command, contextFile string) error {
	absContext, _ := filepath.Abs(contextFile)

	// Create a temporary script that will be executed in tmux
	// tmux new-session -d -s <name> -x 220 -y 50 "<cmd> -p <contextfile>"
	cmdStr := fmt.Sprintf("%s -p %s", command, absContext)

	cmd := exec.Command("tmux", "new-session", "-d",
		"-s", name,
		"-x", "220",
		"-y", "50",
		cmdStr)

	return cmd.Run()
}

func capturePane(sessionName string) (string, error) {
	cmd := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p")
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func isSessionAlive(sessionName string) (bool, error) {
	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	// has-session returns non-zero if session doesn't exist
	return false, nil
}

func killSession(sessionName string) error {
	cmd := exec.Command("tmux", "kill-session", "-t", sessionName)
	return cmd.Run()
}

func extractNewLines(current string, prev []string) []string {
	currentLines := strings.Split(strings.TrimSpace(current), "\n")
	if len(prev) == 0 {
		return currentLines
	}

	// Find new lines by comparing lengths and content
	var newLines []string
	startIdx := len(prev)
	if startIdx > len(currentLines) {
		startIdx = len(currentLines)
	}

	for i := startIdx; i < len(currentLines); i++ {
		newLines = append(newLines, currentLines[i])
	}

	// Also check if existing lines changed (which indicates streaming update)
	for i := 0; i < startIdx && i < len(currentLines); i++ {
		if i < len(prev) && prev[i] != currentLines[i] {
			// Line was updated - include all lines from here
			newLines = currentLines[i:]
			break
		}
	}

	return newLines
}

func detectPrompt(output string) bool {
	// Look for the idle prompt "❯ " at the start of a line
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "❯") {
			return true
		}
	}
	return false
}

func summarizeFindings(streaming bool, lastStreamTime time.Time) {
	fmt.Println("\n=== SPIKE FINDINGS ===")
	if streaming {
		fmt.Println("✓ Output DOES stream progressively during agent execution")
		fmt.Printf("  Last output change at: %s\n", lastStreamTime.Format("15:04:05.000"))
	} else {
		fmt.Println("✗ Output does NOT stream - arrives in one blob")
	}

	fmt.Println("\nNotes for tmux integration:")
	fmt.Println("- tmux requires 'brew install tmux' on macOS (not default)")
	fmt.Println("- Suggest making streaming optional with graceful fallback")
	fmt.Println("- Fall back to current -p mode if tmux not available")
	fmt.Println("- capture-pane output is clean text (ANSI stripped)")
	fmt.Println("- May need SIGWINCH wake trick for detached sessions")
}
