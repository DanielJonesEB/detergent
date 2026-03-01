package gate

import (
	"fmt"
	"os"
	"os/exec"
)

type Gate struct {
	Name string
	Run  string
}

// RunGates executes gates in order, failing fast on the first error.
func RunGates(gates []Gate, dir string) error {
	for _, g := range gates {
		fmt.Fprintf(os.Stderr, "gate: running %s\n", g.Name)
		cmd := exec.Command("sh", "-c", g.Run)
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("gate %q failed: %w", g.Name, err)
		}
	}
	return nil
}
