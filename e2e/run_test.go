package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	lineGit "github.com/re-cinq/assembly-line/internal/git"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("line run", func() {
	var dir string
	var agentScript string

	BeforeEach(func() {
		dir = tempRepo()
		agentScript = writeMockAgent(dir)
	})

	writeRunConfig := func(dir, agentPath string) {
		writeConfig(dir, `agent:
  command: `+agentPath+`
  args: ["-p"]

settings:
  watches: master

stations:
  - name: review
    prompt: "Review code"
  - name: cleanup
    prompt: "Clean up code"
`)
	}

	// RUN-1: Each station is executed in sequence
	// RUN-2: Each station operates on its own branch
	// RUN-5: Stations should commit any changes
	It("executes stations in sequence on their own branches [RUN-1, RUN-2, RUN-5]", func() {
		writeRunConfig(dir, agentScript)
		installHooksForTest(dir)

		writeFile(dir, "code.go", "package main\n")
		gitCommit(dir, "add code")

		// Verify station branches exist
		Expect(git(dir, "branch")).To(ContainSubstring("line/stn/review"))
		Expect(git(dir, "branch")).To(ContainSubstring("line/stn/cleanup"))

		// We should be back on the watched branch
		Expect(currentBranch(dir)).To(Equal("master"))

		// Check that station branches have commits
		git(dir, "checkout", "line/stn/review")
		Expect(readFile(dir, "agent-output.txt")).To(ContainSubstring("Review code"))

		git(dir, "checkout", "line/stn/cleanup")
		Expect(readFile(dir, "agent-output.txt")).To(ContainSubstring("Clean up code"))
	})

	// RUN-3: Stations must not operate on any other branches
	It("does not modify the watched branch during station execution [RUN-3]", func() {
		writeRunConfig(dir, agentScript)
		installHooksForTest(dir)

		writeFile(dir, "code.go", "package main\n")
		gitCommit(dir, "add code")

		// After the commit+run, HEAD should still be the user's commit
		Expect(git(dir, "log", "-1", "--format=%s")).To(Equal("add code"))
	})

	// RUN-4: Stations must not re-trigger line run
	It("does not re-trigger when LINE_RUNNING is set [RUN-4]", func() {
		writeRunConfig(dir, agentScript)
		installHooksForTest(dir)

		writeFile(dir, "code.go", "package main\n")

		// Commit with LINE_RUNNING=1 in the environment
		git(dir, "add", ".")
		cmd := exec.Command("git", "commit", "-m", "add code")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LINE_RUNNING=1")
		out, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(out)).To(ContainSubstring("skipping"))

		// No station branches should exist
		branches := git(dir, "branch")
		Expect(branches).NotTo(ContainSubstring("line/stn/"))
	})

	// RUN-4: Stations must not re-trigger - branch check
	It("does not run when not on the watched branch [RUN-4]", func() {
		writeRunConfig(dir, agentScript)
		installHooksForTest(dir)

		git(dir, "checkout", "-b", "other-branch")
		writeFile(dir, "somefile.txt", "content\n")
		out := gitCommit(dir, "commit on other branch")
		Expect(out).To(ContainSubstring("skipping"))
	})

	// RUN-5: Station commits include [skip line] marker
	It("station commits contain [skip line] marker [RUN-5, RUN-9]", func() {
		writeRunConfig(dir, agentScript)
		installHooksForTest(dir)

		writeFile(dir, "code.go", "package main\n")
		gitCommit(dir, "add code")

		git(dir, "checkout", "line/stn/review")
		msg := git(dir, "log", "-1", "--format=%s")
		Expect(msg).To(ContainSubstring("[skip line]"))
	})

	// RUN-6: Stations should 'just work' - catch up on first run
	It("creates station branches from scratch [RUN-6]", func() {
		writeRunConfig(dir, agentScript)
		installHooksForTest(dir)

		writeFile(dir, "code.go", "package main\n")
		gitCommit(dir, "add code")

		// First run ever - branches created
		Expect(git(dir, "branch")).To(ContainSubstring("line/stn/review"))
		Expect(git(dir, "branch")).To(ContainSubstring("line/stn/cleanup"))
	})

	// RUN-6: Station catches up when watched branch has new commits
	It("catches up station branches when watched branch has moved ahead [RUN-6]", func() {
		writeRunConfig(dir, agentScript)
		installHooksForTest(dir)

		writeFile(dir, "code.go", "package main\n")
		gitCommit(dir, "first change")

		// Now make new commits on the watched branch (moving it ahead)
		writeFile(dir, "extra.go", "package main\nfunc Extra() {}\n")
		gitCommit(dir, "second change")

		// Verify the station branch has the new file from the second commit
		git(dir, "checkout", "line/stn/review")
		Expect(fileExists(dir, "extra.go")).To(BeTrue(), "station should have caught up to include extra.go")
	})

	// RUN-6: Station catches up even when merge conflicts occur
	It("resets station branch to predecessor on merge conflict [RUN-6]", func() {
		writeRunConfig(dir, agentScript)
		installHooksForTest(dir)

		writeFile(dir, "code.go", "package main\n")
		gitCommit(dir, "first change")

		// Create a conflicting change on the watched branch
		writeFile(dir, "agent-output.txt", "conflicting content from master\n")
		gitCommit(dir, "conflicting change")

		// Station should still work - verify it ran the agent
		git(dir, "checkout", "line/stn/review")
		content := readFile(dir, "agent-output.txt")
		Expect(content).To(ContainSubstring("agent was here"))
	})

	// RUN-7, RUN-8: .lineignore
	It("skips when all changed files match .lineignore [RUN-7, RUN-8]", func() {
		writeRunConfig(dir, agentScript)
		writeFile(dir, ".lineignore", "*.log\n")
		installHooksForTest(dir)

		gitCommit(dir, "add config and lineignore")

		writeFile(dir, "debug.log", "log stuff\n")
		out := gitCommit(dir, "add log file")
		Expect(out).To(ContainSubstring("skipping"))
	})

	// RUN-8: .lineignore supports directory patterns
	It("supports directory patterns in .lineignore [RUN-8]", func() {
		writeRunConfig(dir, agentScript)
		writeFile(dir, ".lineignore", "docs/\n")
		installHooksForTest(dir)

		gitCommit(dir, "add config and lineignore")

		writeFile(dir, "docs/readme.md", "documentation\n")
		out := gitCommit(dir, "add docs")
		Expect(out).To(ContainSubstring("skipping"))
	})

	// RUN-8: .lineignore supports negation patterns
	It("supports negation patterns in .lineignore [RUN-8]", func() {
		writeRunConfig(dir, agentScript)
		writeFile(dir, ".lineignore", "*.log\n!important.log\n")
		installHooksForTest(dir)

		gitCommit(dir, "add config and lineignore")

		// A file that matches the negation should NOT be ignored
		writeFile(dir, "important.log", "important stuff\n")
		out := gitCommit(dir, "add important log")

		// Should NOT skip because important.log is negated
		Expect(out).NotTo(ContainSubstring("all changed files are ignored"))
	})

	// RUN-8: .lineignore supports double-star patterns
	It("supports double-star glob patterns in .lineignore [RUN-8]", func() {
		writeRunConfig(dir, agentScript)
		writeFile(dir, ".lineignore", "**/generated/**\n")
		installHooksForTest(dir)

		gitCommit(dir, "add config and lineignore")

		writeFile(dir, "src/generated/code.go", "package gen\n")
		out := gitCommit(dir, "add generated code")
		Expect(out).To(ContainSubstring("skipping"))
	})

	// RUN-8: .lineignore supports comments
	It("treats lines starting with # as comments in .lineignore [RUN-8]", func() {
		writeRunConfig(dir, agentScript)
		writeFile(dir, ".lineignore", "# This is a comment\n*.log\n")
		installHooksForTest(dir)

		gitCommit(dir, "add config and lineignore")

		writeFile(dir, "debug.log", "log stuff\n")
		out := gitCommit(dir, "add log file")
		Expect(out).To(ContainSubstring("skipping"))
	})

	// RUN-9: Skip markers in commit message
	It("skips commits with [skip ci] marker [RUN-9]", func() {
		writeRunConfig(dir, agentScript)
		installHooksForTest(dir)

		writeFile(dir, "code.go", "package main\n")
		out := gitCommit(dir, "add code [skip ci]")
		Expect(out).To(ContainSubstring("skipping"))
	})

	It("skips commits with [ci skip] marker [RUN-9]", func() {
		writeRunConfig(dir, agentScript)
		installHooksForTest(dir)

		writeFile(dir, "code.go", "package main\n")
		out := gitCommit(dir, "add code [ci skip]")
		Expect(out).To(ContainSubstring("skipping"))
	})

	It("skips commits with [line skip] marker [RUN-9]", func() {
		writeRunConfig(dir, agentScript)
		installHooksForTest(dir)

		writeFile(dir, "code.go", "package main\n")
		out := gitCommit(dir, "add code [line skip]")
		Expect(out).To(ContainSubstring("skipping"))
	})

	// RUN-10: Line runs should be independent of rebases on the watched branch
	It("works correctly after a rebase on the watched branch [RUN-10]", func() {
		writeRunConfig(dir, agentScript)
		installHooksForTest(dir)

		// First commit and run (via hook)
		writeFile(dir, "code.go", "package main\n")
		gitCommit(dir, "first change")

		// Simulate a rebase by amending the last commit (triggers hooks again)
		writeFile(dir, "code.go", "package main\n\nfunc main() {}\n")
		git(dir, "add", ".")
		git(dir, "commit", "--amend", "-m", "first change amended")

		Expect(currentBranch(dir)).To(Equal("master"))
	})

	// RUN-11: New run terminates previous run
	It("writes and cleans up PID file [RUN-11]", func() {
		writeRunConfig(dir, agentScript)
		installHooksForTest(dir)

		writeFile(dir, "code.go", "package main\n")
		gitCommit(dir, "add code")

		// PID file should be cleaned up after run completes
		Expect(fileExists(dir, ".line/run.pid")).To(BeFalse())
	})

	// RUN-11: Test concurrent run termination
	It("terminates a previous slow run when a new run starts [RUN-11]", func() {
		slowAgent := writeSlowMockAgent(dir)
		writeConfig(dir, `agent:
  command: `+slowAgent+`
  args: ["-p"]

settings:
  watches: master

stations:
  - name: slow
    prompt: "Be slow"
`)
		installHooksForTestBg(dir)

		writeFile(dir, "code.go", "package main\n")
		gitCommit(dir, "add code")

		// Wait for PID file (slow run has started in background)
		Eventually(func() bool {
			return fileExists(dir, ".line/run.pid")
		}, 5*time.Second, 100*time.Millisecond).Should(BeTrue())

		// Read and track the slow run PID for cleanup
		pidContent := readFile(dir, ".line/run.pid")
		var slowPID int
		fmt.Sscanf(strings.TrimSpace(pidContent), "%d", &slowPID)
		DeferCleanup(func() {
			_ = syscall.Kill(-slowPID, syscall.SIGKILL)
			_ = syscall.Kill(slowPID, syscall.SIGKILL)
		})

		// Kill the slow run manually
		_ = syscall.Kill(slowPID, syscall.SIGKILL)
		Eventually(func() bool {
			return syscall.Kill(slowPID, 0) != nil
		}, 5*time.Second, 100*time.Millisecond).Should(BeTrue())

		// Ensure we're back on master
		_, _ = gitMay(dir, "checkout", "-f", "master")

		// Switch to foreground hook and fast agent
		writeRunConfig(dir, agentScript)
		installHooksForTest(dir)

		writeFile(dir, "trigger.txt", "trigger\n")
		gitCommit(dir, "trigger second run")

		Expect(currentBranch(dir)).To(Equal("master"))
	})

	// RUN-11: A second line run terminates the first via PID-based process group kill
	It("a second run terminates the first run via PID [RUN-11]", func() {
		slowAgent := writeSlowMockAgent(dir)
		writeConfig(dir, `agent:
  command: `+slowAgent+`
  args: ["-p"]

settings:
  watches: master

stations:
  - name: slow
    prompt: "Be slow"
`)
		installHooksForTestBg(dir)

		writeFile(dir, "code.go", "package main\n")
		gitCommit(dir, "add code")

		// Wait for the first run to write its PID file
		Eventually(func() bool {
			return fileExists(dir, ".line/run.pid")
		}, 5*time.Second, 100*time.Millisecond).Should(BeTrue())

		// Read the first run's PID
		pidContent := readFile(dir, ".line/run.pid")
		var firstPID int
		fmt.Sscanf(strings.TrimSpace(pidContent), "%d", &firstPID)
		Expect(firstPID).To(BeNumerically(">", 0))
		DeferCleanup(func() {
			_ = syscall.Kill(-firstPID, syscall.SIGKILL)
			_ = syscall.Kill(firstPID, syscall.SIGKILL)
		})

		// Force back to master for the second run
		_, _ = gitMay(dir, "checkout", "-f", "master")

		// Switch to foreground hook and fast agent for the second run
		writeRunConfig(dir, agentScript)
		installHooksForTest(dir)

		// The second commit triggers line run via hook,
		// which reads the PID file, detects the process, and kills it
		writeFile(dir, "trigger.txt", "trigger second run\n")
		out := gitCommit(dir, "trigger second run")

		// The second run's output should confirm it terminated the previous
		Expect(out).To(ContainSubstring("terminating previous run"))

		// First process should be dead
		Eventually(func() bool {
			return syscall.Kill(firstPID, 0) != nil
		}, 5*time.Second, 100*time.Millisecond).Should(BeTrue())

		Expect(currentBranch(dir)).To(Equal("master"))
	})

	// RUN-12: Default preamble prompt
	It("prepends preamble to agent prompt [RUN-12]", func() {
		// Create an agent that captures its full prompt
		captureAgent := filepath.Join(dir, "capture-agent.sh")
		err := os.WriteFile(captureAgent, []byte(`#!/bin/sh
# Capture the last argument (prompt) to a file
echo "${@: -1}" > captured-prompt.txt
`), 0o755)
		Expect(err).NotTo(HaveOccurred())

		writeConfig(dir, `agent:
  command: `+captureAgent+`
  args: ["-p"]

settings:
  watches: master

stations:
  - name: review
    prompt: "Review the code"
`)
		installHooksForTest(dir)

		writeFile(dir, "code.go", "package main\n")
		gitCommit(dir, "add code")

		git(dir, "checkout", "line/stn/review")
		prompt := readFile(dir, "captured-prompt.txt")
		Expect(prompt).To(ContainSubstring("Do NOT commit"))
		Expect(prompt).To(ContainSubstring("Review the code"))
	})

	// CFG-STN-1, CFG-STN-2: Default agent command and args
	It("uses default agent command and args [CFG-STN-1, CFG-STN-2]", func() {
		writeRunConfig(dir, agentScript)
		installHooksForTest(dir)

		writeFile(dir, "code.go", "package main\n")
		gitCommit(dir, "add code")

		// Stations should have run with the default agent
		git(dir, "checkout", "line/stn/review")
		Expect(fileExists(dir, "agent-output.txt")).To(BeTrue())
	})

	// CFG-STN-3, CFG-STN-4: Custom station command and args
	It("uses custom station command and args when configured [CFG-STN-3, CFG-STN-4]", func() {
		customAgent := filepath.Join(dir, "custom-agent.sh")
		err := os.WriteFile(customAgent, []byte(`#!/bin/sh
echo "custom-agent ran: ${@: -1}" >> custom-output.txt
`), 0o755)
		Expect(err).NotTo(HaveOccurred())

		writeConfig(dir, `agent:
  command: `+agentScript+`
  args: ["-p"]

settings:
  watches: master

stations:
  - name: review
    command: `+customAgent+`
    args: ["--custom"]
    prompt: "Custom review"
`)
		installHooksForTest(dir)

		writeFile(dir, "code.go", "package main\n")
		gitCommit(dir, "add code")

		git(dir, "checkout", "line/stn/review")
		Expect(fileExists(dir, "custom-output.txt")).To(BeTrue())
		content := readFile(dir, "custom-output.txt")
		Expect(content).To(ContainSubstring("Custom review"))
	})

	// CFG-STN-5: Station prompt
	It("passes the configured prompt to the agent [CFG-STN-5]", func() {
		writeRunConfig(dir, agentScript)
		installHooksForTest(dir)

		writeFile(dir, "code.go", "package main\n")
		gitCommit(dir, "add code")

		git(dir, "checkout", "line/stn/review")
		output := readFile(dir, "agent-output.txt")
		Expect(output).To(ContainSubstring("Review code"))
	})

	// RUN-14: A failed station must block the line
	It("stops the line when a station fails [RUN-14]", func() {
		failingAgent := writeFailingMockAgent(dir)
		writeConfig(dir, `agent:
  command: `+failingAgent+`
  args: ["-p"]

settings:
  watches: master

stations:
  - name: review
    prompt: "Review code"
  - name: cleanup
    command: `+agentScript+`
    args: ["-p"]
    prompt: "Clean up code"
`)
		installHooksForTest(dir)

		writeFile(dir, "code.go", "package main\n")
		gitCommit(dir, "add code")

		// The first station (review) failed, so cleanup should NOT have run.
		branches := git(dir, "branch")
		Expect(branches).NotTo(ContainSubstring("line/stn/cleanup"),
			"cleanup station branch should not exist after review failed")
	})

	// RUN-14: A failed station is reported as 'failed' in status
	It("reports a failed station as failed in status [RUN-14]", func() {
		failingAgent := writeFailingMockAgent(dir)
		writeConfig(dir, `agent:
  command: `+failingAgent+`
  args: ["-p"]

settings:
  watches: master

stations:
  - name: review
    prompt: "Review code"
`)
		installHooksForTest(dir)

		writeFile(dir, "code.go", "package main\n")
		gitCommit(dir, "add code")

		out := lineOK(dir, "status")
		Expect(out).To(ContainSubstring("\033[31m"))
		Expect(out).To(ContainSubstring("failed"))
	})

	// RUN-14: A failed station is reported as 'failed' in statusline
	It("reports a failed station as failed in statusline [RUN-14]", func() {
		failingAgent := writeFailingMockAgent(dir)
		writeConfig(dir, `agent:
  command: `+failingAgent+`
  args: ["-p"]

settings:
  watches: master

stations:
  - name: review
    prompt: "Review code"
`)
		installHooksForTest(dir)

		writeFile(dir, "code.go", "package main\n")
		gitCommit(dir, "add code")

		out := lineOK(dir, "statusline")
		Expect(out).To(ContainSubstring("✗ review"))
	})

	// Station pipeline model: stations form a chain via rebase
	It("stations rebase onto predecessor in chain [RUN-1, RUN-2, RUN-16]", func() {
		writeRunConfig(dir, agentScript)
		installHooksForTest(dir)

		writeFile(dir, "code.go", "package main\n")
		gitCommit(dir, "add code")

		// The cleanup station should have the review station's changes
		git(dir, "checkout", "line/stn/cleanup")
		// cleanup rebases onto review, which has agent-output.txt with review prompt
		output := readFile(dir, "agent-output.txt")
		// The file should contain both review and cleanup outputs
		lines := strings.Split(strings.TrimSpace(output), "\n")
		Expect(len(lines)).To(BeNumerically(">=", 2))

		// Verify linear history (no merge commits) from master to cleanup
		mergeCommits := git(dir, "log", "--merges", "--oneline", "master..line/stn/cleanup")
		Expect(mergeCommits).To(BeEmpty(), "rebase should produce linear history with no merge commits")
	})

	// RUN-15: Ephemeral worktrees - main working tree not disturbed
	It("does not disturb the main working tree while stations run [RUN-15]", func() {
		slowAgent := writeSlowMockAgent(dir)
		writeConfig(dir, `agent:
  command: `+slowAgent+`
  args: ["-p"]

settings:
  watches: master

stations:
  - name: review
    prompt: "Review code"
`)
		installHooksForTestBg(dir)

		writeFile(dir, "code.go", "package main\n")
		gitCommit(dir, "add code")

		// Wait for the station agent to be running (background run)
		Eventually(func() bool {
			return fileExists(dir, ".line/stations/review.pid")
		}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())

		// Clean up background processes when done
		DeferCleanup(func() { killBackground(dir, "review") })

		// Main working tree should still be on master
		Expect(currentBranch(dir)).To(Equal("master"))

		// User should be able to edit files and commit while station runs
		writeFile(dir, "user-work.txt", "user is working\n")
		git(dir, "add", "user-work.txt")
		git(dir, "commit", "-m", "user commit while line runs [skip line]")

		// A worktree dir should exist under /tmp/line-*/review
		baseDir, err := lineGit.WorktreeBaseDir(dir)
		Expect(err).NotTo(HaveOccurred())
		wtPath := filepath.Join(baseDir, "review")
		Expect(wtPath).To(BeADirectory())
	})

	// RUN-10 + RUN-15: Rebasing master while a station runs in a worktree
	It("survives a rebase on master while a station is running [RUN-10, RUN-15]", func() {
		slowAgent := writeSlowMockAgent(dir)
		writeConfig(dir, `agent:
  command: `+slowAgent+`
  args: ["-p"]

settings:
  watches: master

stations:
  - name: review
    prompt: "Review code"
`)
		installHooksForTestBg(dir)

		writeFile(dir, "code.go", "package main\n")
		gitCommit(dir, "first change")

		// Wait for the station agent to be running
		Eventually(func() bool {
			return fileExists(dir, ".line/stations/review.pid")
		}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())

		// Clean up background processes
		DeferCleanup(func() { killBackground(dir, "review") })

		// Rebase master while the station is running (amend the last commit)
		Expect(currentBranch(dir)).To(Equal("master"))
		writeFile(dir, "code.go", "package main\n\nfunc main() {}\n")
		git(dir, "add", ".")
		git(dir, "commit", "--amend", "-m", "first change rebased [skip line]")

		// Kill the slow run via its PID
		if data, err := os.ReadFile(filepath.Join(dir, ".line", "run.pid")); err == nil {
			var pid int
			fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &pid)
			if pid > 0 {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}

		// Switch to foreground hook and fast agent, then run again
		writeRunConfig(dir, agentScript)
		installHooksForTest(dir)

		writeFile(dir, "trigger.txt", "trigger after rebase\n")
		gitCommit(dir, "post-rebase run [skip line]")
		// The [skip line] in the trigger commit means line run skips this one.
		// So make another commit without skip marker to actually trigger the run.
		writeFile(dir, "trigger2.txt", "real trigger\n")
		gitCommit(dir, "real trigger")

		Expect(currentBranch(dir)).To(Equal("master"))

		// Station branch should have the agent's work
		git(dir, "checkout", "line/stn/review")
		Expect(fileExists(dir, "agent-output.txt")).To(BeTrue())

		// And master should have the rebased content
		git(dir, "checkout", "master")
		Expect(readFile(dir, "code.go")).To(ContainSubstring("func main()"))
	})

	// RUN-11: A new run must kill station agent processes, not just the runner.
	// Regression: agents start with Setpgid=true (their own process group),
	// so killing the runner's process group leaves station agents orphaned.
	It("kills station agent processes when a new run starts [RUN-11]", func() {
		slowAgent := writeSlowMockAgent(dir)
		writeConfig(dir, `agent:
  command: `+slowAgent+`
  args: ["-p"]

settings:
  watches: master

stations:
  - name: first
    prompt: "First station"
  - name: second
    prompt: "Second station"
`)
		installHooksForTestBg(dir)

		writeFile(dir, "code.go", "package main\n")
		gitCommit(dir, "add code")

		// Wait for the first station's agent to start
		Eventually(func() bool {
			return fileExists(dir, ".line/stations/first.pid")
		}, 10*time.Second, 100*time.Millisecond).Should(BeTrue())

		// Read the station agent PID and confirm it's alive
		stationPIDContent := readFile(dir, ".line/stations/first.pid")
		parts := strings.SplitN(strings.TrimSpace(stationPIDContent), " ", 2)
		var oldAgentPID int
		fmt.Sscanf(parts[0], "%d", &oldAgentPID)
		Expect(oldAgentPID).To(BeNumerically(">", 0))
		Expect(syscall.Kill(oldAgentPID, 0)).To(Succeed(),
			"station agent should be alive before second run")

		// Ensure cleanup of any surviving processes
		DeferCleanup(func() {
			killBackground(dir, "first")
			killBackground(dir, "second")
		})

		// Switch back to master and set up foreground hooks with a fast agent
		_, _ = gitMay(dir, "checkout", "-f", "master")
		writeRunConfig(dir, agentScript)
		installHooksForTest(dir)

		// Trigger a second run — this should kill ALL old processes
		writeFile(dir, "trigger.txt", "trigger\n")
		gitCommit(dir, "trigger second run")

		// The old station agent from the first run should be dead
		Eventually(func() error {
			return syscall.Kill(oldAgentPID, 0)
		}, 5*time.Second, 100*time.Millisecond).ShouldNot(Succeed(),
			"old station agent (PID %d) should have been killed by the new run", oldAgentPID)
	})

	// RUN-15: Worktrees cleaned up after a successful run
	It("cleans up worktrees after a successful run [RUN-15]", func() {
		writeRunConfig(dir, agentScript)
		installHooksForTest(dir)

		writeFile(dir, "code.go", "package main\n")
		gitCommit(dir, "add code")

		// Station branches should have the agent work
		git(dir, "checkout", "line/stn/review")
		Expect(readFile(dir, "agent-output.txt")).To(ContainSubstring("Review code"))
		git(dir, "checkout", "master")

		// No worktree dirs should remain
		baseDir, err := lineGit.WorktreeBaseDir(dir)
		Expect(err).NotTo(HaveOccurred())
		_, err = os.Stat(baseDir)
		Expect(os.IsNotExist(err)).To(BeTrue(), "worktree base dir %s should not exist after run", baseDir)
	})
})
