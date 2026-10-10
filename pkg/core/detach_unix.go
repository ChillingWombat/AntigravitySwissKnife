//go:build !windows

package core

import (
	"os/exec"
	"syscall"
)

// SetDetachedProcess configures a command to run in its own process group and session on Unix.
func SetDetachedProcess(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
}
