package e2e_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("line schema", func() {
	It("outputs valid JSON with expected top-level keys [SCH-1]", func() {
		dir := tempRepo()
		out := lineOK(dir, "schema")

		var schema map[string]any
		Expect(json.Unmarshal([]byte(out), &schema)).To(Succeed())

		props, ok := schema["properties"].(map[string]any)
		Expect(ok).To(BeTrue(), "schema should have properties")
		Expect(props).To(HaveKey("agent"))
		Expect(props).To(HaveKey("settings"))
		Expect(props).To(HaveKey("gates"))
		Expect(props).To(HaveKey("stations"))
	})

	It("includes descriptions on properties [SCH-1]", func() {
		dir := tempRepo()
		out := lineOK(dir, "schema")

		var schema map[string]any
		Expect(json.Unmarshal([]byte(out), &schema)).To(Succeed())
		Expect(schema).To(HaveKey("description"))

		// Check that nested properties also have descriptions
		props := schema["properties"].(map[string]any)
		settings := props["settings"].(map[string]any)
		Expect(settings).To(HaveKey("description"))
	})
})

var _ = Describe("line validate", func() {
	var dir string

	BeforeEach(func() {
		dir = tempRepo()
	})

	It("prints valid for a correct config [VAL-1]", func() {
		writeDefaultConfig(dir)
		out := lineOK(dir, "validate")
		Expect(out).To(Equal("valid"))
	})

	It("reports missing watches [VAL-1]", func() {
		writeConfig(dir, `agent:
  command: echo

settings: {}

stations:
  - name: review
    prompt: "Review code"
`)
		_, err := line(dir, "validate")
		Expect(err).To(HaveOccurred())
	})

	It("reports duplicate station names [VAL-1]", func() {
		writeConfig(dir, `agent:
  command: echo

settings:
  watches: master

stations:
  - name: review
    prompt: "Review code"
  - name: review
    prompt: "Review again"
`)
		out, err := line(dir, "validate")
		Expect(err).To(HaveOccurred())
		Expect(out).To(ContainSubstring("duplicate station name"))
	})

	It("reports station without resolvable command [VAL-1]", func() {
		writeConfig(dir, `settings:
  watches: master

stations:
  - name: review
    prompt: "Review code"
`)
		out, err := line(dir, "validate")
		Expect(err).To(HaveOccurred())
		Expect(out).To(ContainSubstring("no resolvable command"))
	})
})

var _ = Describe("line explain", func() {
	It("outputs non-empty text with key terms [EXP-1]", func() {
		dir := tempRepo()
		out := lineOK(dir, "explain")

		Expect(out).NotTo(BeEmpty())
		Expect(out).To(ContainSubstring("stations"))
		Expect(out).To(ContainSubstring("gates"))
		Expect(out).To(ContainSubstring("agent"))
		Expect(out).To(ContainSubstring("line.yaml"))
	})
})
