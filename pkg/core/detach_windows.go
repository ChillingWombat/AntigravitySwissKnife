//go:build windows

package core

import (
	"os/exec"
	"syscall"
)

// SetDetachedProcess configures a command to run in its own process group on Windows.
func SetDetachedProcess(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008 // DETACHED_PROCESS
}
