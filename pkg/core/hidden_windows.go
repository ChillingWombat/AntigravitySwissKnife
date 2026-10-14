//go:build windows

package core

import (
	"os/exec"
	"syscall"
)

// SetHiddenConsole ensures that console applications executed on Windows do not display a visible command prompt window.
func SetHiddenConsole(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= 0x08000000 // CREATE_NO_WINDOW
}
