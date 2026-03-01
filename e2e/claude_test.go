package e2e_test

import (
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("line run with Claude Code", Label("claude"), func() {
	var dir string

	BeforeEach(func() {
		// Skip if claude is not available
		if _, err := exec.LookPath("claude"); err != nil {
			Skip("claude CLI not found, skipping integration test")
		}

		dir = tempRepo()
	})

	// RUN-13: A station must be able to invoke Claude Code in non-interactive mode
	It("runs a station using Claude Code in non-interactive mode [RUN-13]", func() {
		// Set up a project with a file that has an obvious issue Claude can fix
		writeFile(dir, "hello.txt", "Hello Wrold\n")
		git(dir, "add", ".")
		git(dir, "commit", "-m", "add hello with typo")

		writeConfig(dir, `agent:
  command: claude
  args: ["--dangerously-skip-permissions", "--model", "haiku", "-p"]

settings:
  watches: master

stations:
  - name: fix-typo
    prompt: "The file hello.txt contains a typo: 'Wrold' should be 'World'. Fix it. Do not create any other files."
`)

		out := lineOK(dir, "run")
		_ = out

		// Verify we're back on the watched branch
		Expect(currentBranch(dir)).To(Equal("master"))

		// Verify the station branch exists and has a commit with the skip marker
		branches := git(dir, "branch")
		Expect(branches).To(ContainSubstring("line/stn/fix-typo"))

		// Check the station branch for the fix
		git(dir, "checkout", "line/stn/fix-typo")

		content := readFile(dir, "hello.txt")
		Expect(strings.TrimSpace(content)).To(Equal("Hello World"),
			"Expected Claude to fix the typo 'Wrold' -> 'World'")

		// Verify the commit message has the skip marker
		msg := git(dir, "log", "-1", "--format=%s")
		Expect(msg).To(ContainSubstring("[skip line]"))
	})
})
