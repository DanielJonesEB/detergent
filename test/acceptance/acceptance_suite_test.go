package acceptance_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var binaryPath string

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Acceptance Suite")
}

var _ = BeforeSuite(func() {
	// Build the binary once for all acceptance tests
	_, thisFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	binaryPath = filepath.Join(projectRoot, "bin", "detergent-test")

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/detergent")
	cmd.Dir = projectRoot
	cmd.Env = append(cmd.Environ(), "CGO_ENABLED=0")
	output, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "Failed to build binary: %s", string(output))
})
