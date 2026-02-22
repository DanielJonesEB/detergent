package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("line run", func() {
	var tmpDir string
	var repoDir string

	basicConfigFor := func(repoDir, agentCmd string) string {
		p := filepath.Join(repoDir, "line.yaml")
		writeFile(p, `
agent:
  command: "sh"
  args: ["-c", "`+agentCmd+`"]

settings:
  branch_prefix: "line/"

stations:
  - name: security
    watches: main
    prompt: "Review for security issues"
`)
		return p
	}

	configWithSettingsFor := func(repoDir, content string) string {
		p := filepath.Join(repoDir, "line.yaml")
		writeFile(p, content)
		return p
	}

	BeforeEach(func() {
		tmpDir, repoDir = setupTestRepo("line-test-*")
	})

	AfterEach(func() {
		cleanupTestRepo(repoDir, tmpDir)
	})

	It("exits with code 0", func() {
		configPath := basicConfigFor(repoDir, "echo 'reviewed by agent' > agent-review.txt")
		cmd := exec.Command(binaryPath, "run", "--path", configPath)
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "output: %s", string(output))
	})

	It("creates the output branch", func() {
		configPath := basicConfigFor(repoDir, "echo 'reviewed by agent' > agent-review.txt")
		cmd := exec.Command(binaryPath, "run", "--path", configPath)
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "output: %s", string(output))

		// Check that line/security branch exists
		out := runGitOutput(repoDir, "branch", "--list", "line/security")
		Expect(out).To(ContainSubstring("line/security"))
	})

	It("creates a commit on the output branch with the station tag", func() {
		configPath := basicConfigFor(repoDir, "echo 'reviewed by agent' > agent-review.txt")
		cmd := exec.Command(binaryPath, "run", "--path", configPath)
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "output: %s", string(output))

		// Check the latest commit on line/security
		msg := runGitOutput(repoDir, "log", "-1", "--format=%s", "line/security")
		Expect(msg).To(ContainSubstring("[SECURITY]"))
	})

	It("includes the Triggered-By trailer", func() {
		configPath := basicConfigFor(repoDir, "echo 'reviewed by agent' > agent-review.txt")
		cmd := exec.Command(binaryPath, "run", "--path", configPath)
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "output: %s", string(output))

		msg := runGitOutput(repoDir, "log", "-1", "--format=%B", "line/security")
		Expect(msg).To(ContainSubstring("Triggered-By:"))
	})

	It("pipes context to agent stdin", func() {
		// Use an agent that reads from stdin and writes it to a file
		stdinConfigPath := basicConfigFor(repoDir, "cat > stdin-received.txt")
		cmd := exec.Command(binaryPath, "run", "--path", stdinConfigPath)
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "output: %s", string(output))

		// Verify the agent received context via stdin by checking the file it wrote
		wtPath := filepath.Join(repoDir, ".line", "worktrees", "line", "security")
		stdinContent, err := os.ReadFile(filepath.Join(wtPath, "stdin-received.txt"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(stdinContent)).To(ContainSubstring("# Station: security"))
		Expect(string(stdinContent)).To(ContainSubstring("Review for security issues"))
	})

	It("uses default preamble when none configured", func() {
		stdinConfigPath := basicConfigFor(repoDir, "cat > stdin-received.txt")
		cmd := exec.Command(binaryPath, "run", "--path", stdinConfigPath)
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "output: %s", string(output))

		wtPath := filepath.Join(repoDir, ".line", "worktrees", "line", "security")
		content, err := os.ReadFile(filepath.Join(wtPath, "stdin-received.txt"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(ContainSubstring("You are running non-interactively"))
	})

	It("uses global preamble when configured", func() {
		stdinConfigPath := configWithSettingsFor(repoDir, `
agent:
  command: "sh"
  args: ["-c", "cat > stdin-received.txt"]

settings:
  branch_prefix: "line/"

preamble: "You are a custom global agent. Proceed silently."

stations:
  - name: security
    watches: main
    prompt: "Review for security issues"
`)
		cmd := exec.Command(binaryPath, "run", "--path", stdinConfigPath)
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "output: %s", string(output))

		wtPath := filepath.Join(repoDir, ".line", "worktrees", "line", "security")
		content, err := os.ReadFile(filepath.Join(wtPath, "stdin-received.txt"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(ContainSubstring("You are a custom global agent"))
		Expect(string(content)).NotTo(ContainSubstring("non-interactively"))
	})

	It("uses per-station preamble over global preamble", func() {
		stdinConfigPath := configWithSettingsFor(repoDir, `
agent:
  command: "sh"
  args: ["-c", "cat > stdin-received.txt"]

settings:
  branch_prefix: "line/"

preamble: "Global preamble that should be overridden."

stations:
  - name: security
    watches: main
    prompt: "Review for security issues"
    preamble: "Per-station preamble for security reviews."
`)
		cmd := exec.Command(binaryPath, "run", "--path", stdinConfigPath)
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "output: %s", string(output))

		wtPath := filepath.Join(repoDir, ".line", "worktrees", "line", "security")
		content, err := os.ReadFile(filepath.Join(wtPath, "stdin-received.txt"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(ContainSubstring("Per-station preamble for security reviews"))
		Expect(string(content)).NotTo(ContainSubstring("Global preamble that should be overridden"))
	})

	It("writes permissions settings to worktree when configured", func() {
		permConfigPath := configWithSettingsFor(repoDir, `
agent:
  command: "sh"
  args: ["-c", "cat .claude/settings.json > settings-snapshot.txt"]

settings:
  branch_prefix: "line/"

permissions:
  allow:
    - Edit
    - Write
    - "Bash(*)"

stations:
  - name: security
    watches: main
    prompt: "Review for security issues"
`)
		cmd := exec.Command(binaryPath, "run", "--path", permConfigPath)
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "output: %s", string(output))

		// The agent captured the settings file - check it was written correctly
		wtPath := filepath.Join(repoDir, ".line", "worktrees", "line", "security")
		snapshot, err := os.ReadFile(filepath.Join(wtPath, "settings-snapshot.txt"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(snapshot)).To(ContainSubstring(`"allow"`))
		Expect(string(snapshot)).To(ContainSubstring(`"Edit"`))
		Expect(string(snapshot)).To(ContainSubstring(`"Write"`))
		Expect(string(snapshot)).To(ContainSubstring(`"Bash(*)`))
	})

	It("does not write permissions when not configured", func() {
		// Use the default config (no permissions block)
		noPermConfigPath := basicConfigFor(repoDir, "test -f .claude/settings.json && echo EXISTS || echo MISSING")
		cmd := exec.Command(binaryPath, "run", "--path", noPermConfigPath)
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "output: %s", string(output))

		// Verify .claude/settings.json was NOT created in the worktree
		wtPath := filepath.Join(repoDir, ".line", "worktrees", "line", "security")
		_, err = os.Stat(filepath.Join(wtPath, ".claude", "settings.json"))
		Expect(os.IsNotExist(err)).To(BeTrue(), "settings.json should not exist when permissions not configured")
	})

	It("strips CLAUDECODE env var from agent environment", func() {
		envConfigPath := basicConfigFor(repoDir, "env > env-dump.txt")
		cmd := exec.Command(binaryPath, "run", "--path", envConfigPath)
		cmd.Env = append(os.Environ(), "CLAUDECODE=some-value")
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "output: %s", string(output))

		wtPath := filepath.Join(repoDir, ".line", "worktrees", "line", "security")
		envDump, err := os.ReadFile(filepath.Join(wtPath, "env-dump.txt"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(envDump)).NotTo(ContainSubstring("CLAUDECODE="))
		Expect(string(envDump)).To(ContainSubstring("LINE_AGENT=1"))
	})

	It("succeeds even when GIT_DIR is set in the environment", func() {
		// When a post-commit hook fires from a worktree, GIT_DIR is set
		// to the worktree's gitdir. If line run inherits this, git commands
		// targeting other worktrees can fail with ENOTDIR or wrong-repo errors.
		configPath := basicConfigFor(repoDir, "echo 'reviewed by agent' > agent-review.txt")
		cmd := exec.Command(binaryPath, "run", "--path", configPath)
		cmd.Env = append(os.Environ(),
			"GIT_DIR=/nonexistent/bogus/.git",
			"GIT_WORK_TREE=/nonexistent/bogus",
		)
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "line run should succeed despite poisoned GIT_DIR: %s", string(output))

		out := runGitOutput(repoDir, "branch", "--list", "line/security")
		Expect(out).To(ContainSubstring("line/security"))
	})

	It("acquires a lock preventing concurrent runs", func() {
		// Start two concurrent line run processes — only one should process
		configPath := basicConfigFor(repoDir, "echo 'reviewed by agent' > agent-review.txt")
		cmd1 := exec.Command(binaryPath, "run", "--path", configPath)
		cmd2 := exec.Command(binaryPath, "run", "--path", configPath)

		// Start both
		err1 := cmd1.Start()
		Expect(err1).NotTo(HaveOccurred())
		err2 := cmd2.Start()
		Expect(err2).NotTo(HaveOccurred())

		// Wait for both to finish
		waitErr1 := cmd1.Wait()
		waitErr2 := cmd2.Wait()

		// At least one should succeed, at least one should exit cleanly
		// (the locked-out one exits 0 with a log message, not an error)
		succeeded := (waitErr1 == nil || waitErr2 == nil)
		Expect(succeeded).To(BeTrue(), "at least one concurrent run should succeed")
	})

	It("is idempotent - running twice doesn't create duplicate commits", func() {
		configPath := basicConfigFor(repoDir, "echo 'reviewed by agent' > agent-review.txt")
		cmd1 := exec.Command(binaryPath, "run", "--path", configPath)
		out1, err := cmd1.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "first run: %s", string(out1))

		// Get commit count after first run
		count1 := runGitOutput(repoDir, "rev-list", "--count", "line/security")

		cmd2 := exec.Command(binaryPath, "run", "--path", configPath)
		out2, err := cmd2.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "second run: %s", string(out2))

		// Commit count should be the same
		count2 := runGitOutput(repoDir, "rev-list", "--count", "line/security")
		Expect(count2).To(Equal(count1))
	})
})
