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

// SetDetachedProcessHiddenConsole starts the child in its own process group with a
// HIDDEN console instead of no console (0x08000000 = CREATE_NO_WINDOW).
//
// Do NOT use this for GUI applications you want to see: HideWindow sets
// STARTF_USESHOWWINDOW/SW_HIDE, and Chromium/Electron applies the startup show
// state to its first window, so the app comes up tray-only (the "Launch
// Antigravity opens minimised to the tray" bug). Use SetDetachedGUIProcess for
// those instead.
func SetDetachedProcessHiddenConsole(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP | 0x08000000 // CREATE_NO_WINDOW
}

// SetDetachedGUIProcess prepares a windowed application (Antigravity, VS Code, …)
// for launch from the daemon.
//
// It inherits the daemon's console (there is none of our own to show) and keeps
// its own process group, but deliberately sets no SW_HIDE: SW_HIDE would hide the
// app's first window and Chromium starts tray-only — exactly the reported bug.
// CREATE_NO_WINDOW would be ignored for a GUI subsystem binary anyway.
func SetDetachedGUIProcess(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = false
	cmd.SysProcAttr.CreationFlags &^= 0x08000000 | 0x00000008 // clear CREATE_NO_WINDOW | DETACHED_PROCESS
	cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
}
