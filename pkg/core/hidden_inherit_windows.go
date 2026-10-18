//go:build windows

package core

import (
	"os/exec"
	"syscall"
)

// SetHiddenConsoleInherit gives a shell child (cmd.exe) a NEW HIDDEN CONSOLE so that
// ARBITRARY console commands it runs inherit the hidden console and never open
// visible windows. Use it instead of SetHiddenConsole (CREATE_NO_WINDOW) whenever
// the child may spawn console subprocesses of its own — with no console at all,
// every such subprocess allocates its own VISIBLE window.
func SetHiddenConsoleInherit(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	const (
		createNoWindow  = 0x08000000
		detachedProcess = 0x00000008
		newProcessGroup = 0x00000200
	)
	cmd.SysProcAttr.CreationFlags = (cmd.SysProcAttr.CreationFlags &^ (createNoWindow | detachedProcess)) | newProcessGroup
}
