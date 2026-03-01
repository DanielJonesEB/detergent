package e2e_test

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("line statusline", func() {
	var dir string

	BeforeEach(func() {
		dir = tempRepo()
	})

	// SL-1: One-line format showing same state as line status
	// SL-3: Provided by the statusline subcommand
	It("outputs a one-line status summary [SL-1, SL-3]", func() {
		agentScript := writeMockAgent(dir)
		writeConfig(dir, `agent:
  command: `+agentScript+`
  args: ["-p"]

settings:
  watches: master

stations:
  - name: review
    prompt: "Review code"
  - name: cleanup
    prompt: "Clean up"
`)
		writeFile(dir, "code.go", "package main\n")
		git(dir, "add", ".")
		git(dir, "commit", "-m", "add code")

		lineOK(dir, "run")

		out := lineOK(dir, "statusline")

		// Should be a single line (no newlines in the middle)
		Expect(out).NotTo(ContainSubstring("\n"))

		// Should contain station names
		Expect(out).To(ContainSubstring("review"))
		Expect(out).To(ContainSubstring("cleanup"))
	})

	// SL-1: Shows pending stations with ⏸ and ○ symbols when inactive
	It("shows ⏸ and ○ symbols with station names in one-line format [SL-1]", func() {
		writeDefaultConfig(dir)

		out := lineOK(dir, "statusline")
		Expect(out).NotTo(ContainSubstring("\n"))
		Expect(out).To(ContainSubstring("⏸"))
		Expect(out).To(ContainSubstring("○ review"))
		Expect(out).NotTo(ContainSubstring("line:"))
	})

	// SL-2: Prompts for /line-rebase when terminal station has unpicked commits
	It("prompts for /line-rebase when terminal station has commits not in watched branch [SL-2]", func() {
		agentScript := writeMockAgent(dir)
		writeConfig(dir, `agent:
  command: `+agentScript+`
  args: ["-p"]

settings:
  watches: master

stations:
  - name: review
    prompt: "Review code"
  - name: cleanup
    prompt: "Clean up"
`)
		writeFile(dir, "code.go", "package main\n")
		git(dir, "add", ".")
		git(dir, "commit", "-m", "add code")

		lineOK(dir, "run")

		out := lineOK(dir, "statusline")

		// Terminal station (cleanup) has commits not in master, so should prompt
		Expect(out).To(ContainSubstring("/line-rebase"))
	})

	// SL-2: Does NOT prompt when terminal station has no new commits
	It("does not prompt for /line-rebase when no new commits on terminal station [SL-2]", func() {
		writeDefaultConfig(dir)

		// No run, so no station branches exist
		out := lineOK(dir, "statusline")
		Expect(out).NotTo(ContainSubstring("/line-rebase"))
	})
})

var _ = Describe("line init statusline", func() {
	var dir string

	BeforeEach(func() {
		dir = tempRepo()
	})

	// INIT-6: Configures Claude Code to use line statusline
	It("configures Claude Code statusline setting [INIT-6]", func() {
		lineOK(dir, "init")

		// Should create or update .claude/settings.json with statusline config
		settingsPath := filepath.Join(dir, ".claude", "settings.json")
		data, err := os.ReadFile(settingsPath)
		Expect(err).NotTo(HaveOccurred())

		var settings map[string]any
		err = json.Unmarshal(data, &settings)
		Expect(err).NotTo(HaveOccurred())

		Expect(settings).To(HaveKey("statusLine"))
		slRaw, ok := settings["statusLine"].(map[string]any)
		Expect(ok).To(BeTrue(), "statusLine should be an object")
		Expect(slRaw["type"]).To(Equal("command"))
		Expect(slRaw["command"]).To(Equal("line statusline"))
	})

	// INIT-6 + INIT-4: Converges - doesn't clobber existing settings
	It("preserves existing Claude Code settings when configuring statusline [INIT-6, INIT-4]", func() {
		// Write existing settings
		settingsDir := filepath.Join(dir, ".claude")
		err := os.MkdirAll(settingsDir, 0o755)
		Expect(err).NotTo(HaveOccurred())
		err = os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(`{"model":"sonnet"}`), 0o644)
		Expect(err).NotTo(HaveOccurred())

		lineOK(dir, "init")

		data, err := os.ReadFile(filepath.Join(settingsDir, "settings.json"))
		Expect(err).NotTo(HaveOccurred())

		var settings map[string]any
		err = json.Unmarshal(data, &settings)
		Expect(err).NotTo(HaveOccurred())

		slRaw, ok := settings["statusLine"].(map[string]any)
		Expect(ok).To(BeTrue(), "statusLine should be an object")
		Expect(slRaw["type"]).To(Equal("command"))
		Expect(slRaw["command"]).To(Equal("line statusline"))
		Expect(settings["model"]).To(Equal("sonnet"))
	})
})
