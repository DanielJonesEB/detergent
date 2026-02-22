package acceptance_test

import (
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("parallel station execution", func() {
	var tmpDir string
	var repoDir string
	var configPath string

	BeforeEach(func() {
		tmpDir, repoDir = setupTestRepo("line-parallel-*")
	})

	AfterEach(func() {
		cleanupTestRepo(repoDir, tmpDir)
	})

	Context("with two independent stations that both watch main", func() {
		BeforeEach(func() {
			configPath = filepath.Join(repoDir, "line.yaml")
			writeFile(configPath, `
agent:
  command: "sh"
  args: ["-c", "echo done > agent-output.txt"]

stations:
  - name: alpha
    watches: main
    prompt: "Check alpha"
  - name: beta
    watches: main
    prompt: "Check beta"
`)
		})

		It("processes both stations", func() {
			cmd := exec.Command(binaryPath, "run", "--path", configPath)
			output, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "output: %s", string(output))

			branches := runGitOutput(repoDir, "branch")
			Expect(branches).To(ContainSubstring("line/alpha"))
			Expect(branches).To(ContainSubstring("line/beta"))
		})

	})

	Context("with dependent stations (A -> B)", func() {
		BeforeEach(func() {
			configPath = filepath.Join(repoDir, "line.yaml")
			writeFile(configPath, `
agent:
  command: "sh"
  args: ["-c", "echo done > agent-output.txt"]

stations:
  - name: upstream
    watches: main
    prompt: "First pass"
  - name: downstream
    watches: upstream
    prompt: "Second pass"
`)
		})

		It("processes dependent stations sequentially", func() {
			cmd := exec.Command(binaryPath, "run", "--path", configPath)
			output, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "output: %s", string(output))

			// Both should complete, with downstream seeing upstream's output
			branches := runGitOutput(repoDir, "branch")
			Expect(branches).To(ContainSubstring("line/upstream"))
			Expect(branches).To(ContainSubstring("line/downstream"))

			// Downstream's Triggered-By should reference upstream's branch
			msg := runGitOutput(repoDir, "log", "-1", "--format=%B", "line/downstream")
			Expect(msg).To(ContainSubstring("Triggered-By:"))
		})
	})
})
