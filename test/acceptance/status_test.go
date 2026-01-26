package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("detergent status", func() {
	var tmpDir string
	var repoDir string
	var configPath string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "detergent-status-*")
		Expect(err).NotTo(HaveOccurred())

		repoDir = filepath.Join(tmpDir, "repo")
		runGit(tmpDir, "init", repoDir)
		runGit(repoDir, "checkout", "-b", "main")
		writeFile(filepath.Join(repoDir, "hello.txt"), "hello\n")
		runGit(repoDir, "add", "hello.txt")
		runGit(repoDir, "commit", "-m", "initial commit")

		configPath = filepath.Join(repoDir, "detergent.yaml")
		writeFile(configPath, `
agent:
  command: "sh"
  args: ["-c", "echo 'reviewed' > agent-review.txt"]

concerns:
  - name: security
    watches: main
    prompt: "Review for security issues"
`)
	})

	AfterEach(func() {
		exec.Command("git", "-C", repoDir, "worktree", "prune").Run()
		os.RemoveAll(tmpDir)
	})

	Context("before any run", func() {
		It("shows concerns as pending", func() {
			cmd := exec.Command(binaryPath, "status", configPath)
			output, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			out := string(output)
			Expect(out).To(ContainSubstring("security"))
			Expect(out).To(ContainSubstring("pending"))
		})
	})

	Context("after a successful run", func() {
		BeforeEach(func() {
			cmd := exec.Command(binaryPath, "run", "--once", configPath)
			out, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "run failed: %s", string(out))
		})

		It("shows concerns as caught up", func() {
			cmd := exec.Command(binaryPath, "status", configPath)
			output, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			out := string(output)
			Expect(out).To(ContainSubstring("security"))
			Expect(out).To(ContainSubstring("caught up"))
		})

		It("shows the last-processed commit hash", func() {
			// Get the main branch HEAD
			head := runGitOutput(repoDir, "rev-parse", "main")

			cmd := exec.Command(binaryPath, "status", configPath)
			output, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			// Should contain first 8 chars of the hash
			Expect(string(output)).To(ContainSubstring(head[:8]))
		})
	})
})
