package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("post-rebase cycle prevention", func() {
	var tmpDir string
	var repoDir string
	var configPath string

	BeforeEach(func() {
		tmpDir, repoDir = setupTestRepo("detergent-rebase-cycle-*")

		// Two-concern chain: security watches main, docs watches security
		configPath = filepath.Join(repoDir, "detergent.yaml")
		writeFile(configPath, `
agent:
  command: "sh"
  args: ["-c", "date +%s%N > agent-output.txt"]

concerns:
  - name: security
    watches: main
    prompt: "Review for security issues"
  - name: docs
    watches: security
    prompt: "Update documentation"
`)
	})

	AfterEach(func() {
		cleanupTestRepo(repoDir, tmpDir)
	})

	It("does not re-trigger after rebasing main onto the terminal branch", func() {
		// Run the chain once to process the initial commit
		cmd := exec.Command(binaryPath, "run", "--once", "--path", configPath)
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "first run: %s", string(output))

		// Record commit counts on both branches after first run
		secCount1 := strings.TrimSpace(runGitOutput(repoDir, "rev-list", "--count", "detergent/security"))
		docsCount1 := strings.TrimSpace(runGitOutput(repoDir, "rev-list", "--count", "detergent/docs"))

		// Simulate /detergent-rebase: fast-forward main onto detergent/docs
		runGit(repoDir, "checkout", "main")
		runGit(repoDir, "rebase", "detergent/docs")

		// Run the chain again — should detect agent commits and skip
		cmd2 := exec.Command(binaryPath, "run", "--once", "--path", configPath)
		output2, err := cmd2.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "second run: %s", string(output2))

		// Commit counts should NOT have increased — no new agent work
		secCount2 := strings.TrimSpace(runGitOutput(repoDir, "rev-list", "--count", "detergent/security"))
		docsCount2 := strings.TrimSpace(runGitOutput(repoDir, "rev-list", "--count", "detergent/docs"))
		Expect(secCount2).To(Equal(secCount1), "security branch should have no new commits")
		Expect(docsCount2).To(Equal(docsCount1), "docs branch should have no new commits")
	})

	It("soft-resets agent commits and uses Triggered-By trailers", func() {
		// Use an agent that makes a direct git commit (simulating Claude Code committing)
		commitConfigPath := filepath.Join(repoDir, "detergent-commit.yaml")
		writeFile(commitConfigPath, `
agent:
  command: "sh"
  args: ["-c", "echo agent-was-here > agent-file.txt && git add -A && git commit -m 'agent did this\n\nCo-Authored-By: Claude <noreply@anthropic.com>'"]

concerns:
  - name: security
    watches: main
    prompt: "Review for security issues"
`)

		// Run the chain — the agent will commit directly, but detergent should
		// soft-reset that commit and create its own with Triggered-By.
		cmd := exec.Command(binaryPath, "run", "--once", "--path", commitConfigPath)
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "run: %s", string(output))

		// The output branch should exist and have a commit
		branches := runGitOutput(repoDir, "branch")
		Expect(branches).To(ContainSubstring("detergent/security"))

		// The tip commit should be detergent's proper commit, not the agent's
		tipMsg := strings.TrimSpace(runGitOutput(repoDir, "log", "-1", "--format=%B", "detergent/security"))
		Expect(tipMsg).To(ContainSubstring("Triggered-By:"), "commit should have Triggered-By trailer")
		Expect(tipMsg).NotTo(ContainSubstring("agent did this"), "agent's direct commit message should not be the tip")

		// The agent's file should still be present (changes preserved via soft-reset)
		wtPath := filepath.Join(repoDir, ".detergent", "worktrees", "detergent/security")
		_, err = os.Stat(filepath.Join(wtPath, "agent-file.txt"))
		Expect(err).NotTo(HaveOccurred(), "agent's file changes should be preserved")

		// Now simulate rebase back to main and re-run — should NOT re-trigger
		runGit(repoDir, "checkout", "main")
		runGit(repoDir, "rebase", "detergent/security")

		secCount1 := strings.TrimSpace(runGitOutput(repoDir, "rev-list", "--count", "detergent/security"))

		cmd2 := exec.Command(binaryPath, "run", "--once", "--path", commitConfigPath)
		output2, err := cmd2.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "second run: %s", string(output2))

		secCount2 := strings.TrimSpace(runGitOutput(repoDir, "rev-list", "--count", "detergent/security"))
		Expect(secCount2).To(Equal(secCount1), "should not re-trigger after rebase of proper agent commit")
	})

	It("processes only user commits after rebase + new user commit", func() {
		// Run the chain once
		cmd := exec.Command(binaryPath, "run", "--once", "--path", configPath)
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "first run: %s", string(output))

		// Simulate /detergent-rebase: fast-forward main onto detergent/docs
		runGit(repoDir, "checkout", "main")
		runGit(repoDir, "rebase", "detergent/docs")

		// Add a new user commit on main (after the rebase)
		writeFile(filepath.Join(repoDir, "user-feature.txt"), "new feature\n")
		runGit(repoDir, "add", "user-feature.txt")
		runGit(repoDir, "commit", "-m", "Add new feature")

		// Write a capture script that saves context per-concern from stdin
		captureScript := filepath.Join(repoDir, "capture-agent.sh")
		writeFile(captureScript, `#!/bin/sh
ctx=$(cat)
name=$(echo "$ctx" | grep '# Concern:' | head -1 | awk '{print $NF}')
printf '%s' "$ctx" > "/tmp/detergent-rebase-test-context-$name.txt"
date +%s%N > agent-output.txt
`)
		err = os.Chmod(captureScript, 0755)
		Expect(err).NotTo(HaveOccurred())

		captureConfigPath := filepath.Join(repoDir, "detergent-capture.yaml")
		writeFile(captureConfigPath, `
agent:
  command: "`+captureScript+`"

concerns:
  - name: security
    watches: main
    prompt: "Review for security issues"
  - name: docs
    watches: security
    prompt: "Update documentation"
`)

		// Run with the capturing agent
		cmd2 := exec.Command(binaryPath, "run", "--once", "--path", captureConfigPath)
		output2, err := cmd2.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "second run: %s", string(output2))

		// Read the security concern's captured context (watches external branch main)
		secContextData, err := os.ReadFile("/tmp/detergent-rebase-test-context-security.txt")
		Expect(err).NotTo(HaveOccurred(), "should have captured security context file")
		secContext := string(secContextData)

		// Security context should contain the user commit
		Expect(secContext).To(ContainSubstring("Add new feature"))

		// Security context should NOT contain agent commits (filtered on external branch)
		Expect(secContext).NotTo(ContainSubstring("Triggered-By:"))

		// Clean up
		os.Remove("/tmp/detergent-rebase-test-context-security.txt")
		os.Remove("/tmp/detergent-rebase-test-context-docs.txt")
	})
})
