package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var binaryPath string

func TestLine(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Line E2E Suite")
}

var _ = BeforeSuite(func() {
	tmpDir, err := os.MkdirTemp("", "line-build-*")
	Expect(err).NotTo(HaveOccurred())

	binaryPath = filepath.Join(tmpDir, "line")

	// Find the module root (where go.mod lives)
	modRoot, err := filepath.Abs(filepath.Join("..", "."))
	Expect(err).NotTo(HaveOccurred())

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/line")
	cmd.Dir = modRoot
	out, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "build failed: %s", string(out))
})

var _ = AfterSuite(func() {
	if binaryPath != "" {
		os.RemoveAll(filepath.Dir(binaryPath))
	}
})
