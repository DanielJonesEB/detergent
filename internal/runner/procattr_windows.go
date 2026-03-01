//go:build windows

package runner

import "os/exec"

// setProcGroup is a no-op on Windows; process groups are managed differently.
func setProcGroup(_ *exec.Cmd) {}
