//go:build windows

package core

import (
	"os/exec"
	"syscall"
)

// setHiddenConsoleProcess gives a console child a NEW HIDDEN CONSOLE
// (CREATE_NEW_CONSOLE + STARTF_USESHOWWINDOW/SW_HIDE) in its own process group.
// Console children it spawns inherit the hidden console and stay invisible.
//
// DETACHED_PROCESS and CREATE_NO_WINDOW are explicitly cleared: they leave the
// child without any console, which makes every console grandchild allocate its
// own VISIBLE window (the "command window storm" observed on Antigravity launch).
func setHiddenConsoleProcess(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	const (
		createNewConsole = 0x00000010
		createNoWindow   = 0x08000000
		detachedProcess  = 0x00000008
		newProcessGroup  = 0x00000200
	)
	cmd.SysProcAttr.CreationFlags = (cmd.SysProcAttr.CreationFlags &^ (createNoWindow | detachedProcess)) |
		createNewConsole | newProcessGroup
}
