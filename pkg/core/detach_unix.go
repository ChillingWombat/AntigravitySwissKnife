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

// SetDetachedProcessHiddenConsole is the Unix equivalent of the Windows variant:
// a new session already detaches from any controlling terminal.
func SetDetachedProcessHiddenConsole(cmd *exec.Cmd) {
	SetDetachedProcess(cmd)
}

// SetDetachedGUIProcess is the Unix counterpart of the Windows helper: a new
// session, no controlling terminal, and no startup show-state games.
func SetDetachedGUIProcess(cmd *exec.Cmd) {
	SetDetachedProcess(cmd)
}
