package runner

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

const preamble = "IMPORTANT: Do NOT commit any changes. Do NOT run git commit. Make file changes only. The system will handle committing."

// agentProcess represents a running agent subprocess.
type agentProcess struct {
	cmd *exec.Cmd
}

// startAgent launches an agent subprocess with the given command, args, and prompt.
// The agent runs in its own process group for clean cleanup.
// RUN-12: The preamble is prepended to the prompt.
func startAgent(dir, command string, args []string, prompt string) (*agentProcess, error) {
	fullPrompt := preamble + "\n\n" + prompt
	fullArgs := make([]string, len(args))
	copy(fullArgs, args)
	fullArgs = append(fullArgs, fullPrompt)

	cmd := exec.Command(command, fullArgs...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Build a clean environment for the agent:
	// - Remove CLAUDECODE so Claude Code can launch as a fresh session
	// - Set LINE_RUNNING=1 to prevent retriggering
	env := cleanEnv(os.Environ(), "CLAUDECODE")
	cmd.Env = append(env, "LINE_RUNNING=1")

	// Set process group so we can kill the whole group
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting agent %q: %w", command, err)
	}

	return &agentProcess{cmd: cmd}, nil
}

// wait waits for the agent to finish.
func (a *agentProcess) wait() error {
	return a.cmd.Wait()
}

// pid returns the process ID of the agent.
func (a *agentProcess) pid() int {
	if a.cmd.Process != nil {
		return a.cmd.Process.Pid
	}
	return 0
}

// cleanEnv returns a copy of environ with the named variables removed.
func cleanEnv(environ []string, keys ...string) []string {
	result := make([]string, 0, len(environ))
	for _, e := range environ {
		skip := false
		for _, key := range keys {
			if strings.HasPrefix(e, key+"=") {
				skip = true
				break
			}
		}
		if !skip {
			result = append(result, e)
		}
	}
	return result
}
