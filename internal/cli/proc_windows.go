//go:build windows

package cli

import "syscall"

// detachedProcAttr returns SysProcAttr that creates a detached process on Windows.
func detachedProcAttr() *syscall.SysProcAttr {
	// CREATE_NEW_PROCESS_GROUP detaches the child from the parent's console.
	const createNewProcessGroup = 0x00000200
	return &syscall.SysProcAttr{CreationFlags: createNewProcessGroup}
}
