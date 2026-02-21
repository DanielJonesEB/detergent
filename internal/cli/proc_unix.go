//go:build !windows

package cli

import "syscall"

// detachedProcAttr returns SysProcAttr that starts the process in a new session.
func detachedProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
