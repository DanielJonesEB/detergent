package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("line gate", func() {
	var dir string

	BeforeEach(func() {
		dir = tempRepo()
	})

	// CFG-1: Config loaded via -p flag
	// CFG-2: watches configured in YAML
	// CFG-GATE-1: An ordered list of Gates can be configured
	It("runs gates in order and fails fast [CFG-1, CFG-2, CFG-GATE-1]", func() {
		writeConfig(dir, `agent:
  command: echo

settings:
  watches: master

gates:
  - name: first
    run: "echo first >> gate-log.txt"
  - name: second
    run: "echo second >> gate-log.txt"
`)
		lineOK(dir, "gate", "-p", "line.yaml")
		log := readFile(dir, "gate-log.txt")
		Expect(log).To(Equal("first\nsecond\n"))
	})

	It("fails fast when a gate fails [CFG-GATE-1]", func() {
		writeConfig(dir, `agent:
  command: echo

settings:
  watches: master

gates:
  - name: pass
    run: "echo pass >> gate-log.txt"
  - name: fail
    run: "exit 1"
  - name: never
    run: "echo never >> gate-log.txt"
`)
		_, err := line(dir, "gate", "-p", "line.yaml")
		Expect(err).To(HaveOccurred())

		log := readFile(dir, "gate-log.txt")
		Expect(log).To(Equal("pass\n"))
		Expect(log).NotTo(ContainSubstring("never"))
	})
})
