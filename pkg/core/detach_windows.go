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
// Use this for GUI apps that spawn console helpers (MCP servers launched through
// cmd.exe): with DETACHED_PROCESS the app has no console, so every console child
// allocates its own VISIBLE window — one Launch Antigravity click could pop a
// dozen terminals. With a hidden console the children inherit it and stay invisible.
func SetDetachedProcessHiddenConsole(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP | 0x08000000 // CREATE_NO_WINDOW
}
