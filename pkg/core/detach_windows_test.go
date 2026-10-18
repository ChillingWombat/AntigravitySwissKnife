//go:build windows

package core

import (
	"os/exec"
	"syscall"
	"testing"
)

// Launching the IDE must give it a hidden console rather than no console:
// with DETACHED_PROCESS every console helper the app spawns (MCP servers
// launched through cmd.exe) allocates its own VISIBLE window.
func TestSetDetachedProcessHiddenConsole_Flags(t *testing.T) {
	cmd := exec.Command("cmd.exe")
	SetDetachedProcessHiddenConsole(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr not initialised")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Error("HideWindow must be set")
	}
	const (
		createNewProcessGroup = 0x00000200
		detachedProcess       = 0x00000008
		createNoWindow        = 0x08000000
	)
	flags := cmd.SysProcAttr.CreationFlags
	if flags&createNoWindow == 0 {
		t.Errorf("CREATE_NO_WINDOW missing (flags=%#x)", flags)
	}
	if flags&createNewProcessGroup == 0 {
		t.Errorf("CREATE_NEW_PROCESS_GROUP missing (flags=%#x)", flags)
	}
	if flags&detachedProcess != 0 {
		t.Errorf("DETACHED_PROCESS must not be set — it strips the console so console children open visible windows (flags=%#x)", flags)
	}
}

// A GUI app launched from the daemon must come up with a visible main window:
// HideWindow (STARTF_USESHOWWINDOW/SW_HIDE) is honoured by Chromium/Electron for
// the first window, which is exactly why "Launch Antigravity" opened tray-only.
func TestSetDetachedGUIProcess_Flags(t *testing.T) {
	cmd := exec.Command("Antigravity.exe")
	SetDetachedGUIProcess(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr not initialised")
	}
	if cmd.SysProcAttr.HideWindow {
		t.Error("HideWindow must NOT be set — SW_HIDE hides the app's first window (tray-only launch)")
	}
	const (
		createNewProcessGroup = 0x00000200
		detachedProcess       = 0x00000008
		createNoWindow        = 0x08000000
	)
	flags := cmd.SysProcAttr.CreationFlags
	if flags&createNoWindow != 0 {
		t.Errorf("CREATE_NO_WINDOW must not be set for a GUI app (flags=%#x)", flags)
	}
	if flags&detachedProcess != 0 {
		t.Errorf("DETACHED_PROCESS must not be set for a GUI app (flags=%#x)", flags)
	}
	if flags&createNewProcessGroup == 0 {
		t.Errorf("CREATE_NEW_PROCESS_GROUP missing (flags=%#x)", flags)
	}
}

// Pre-existing flags must be cleaned up rather than OR-ed on top.
func TestSetDetachedGUIProcess_ClearsConflictingFlags(t *testing.T) {
	cmd := exec.Command("Antigravity.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000 | 0x00000008}
	SetDetachedGUIProcess(cmd)
	if cmd.SysProcAttr.HideWindow {
		t.Error("HideWindow must be cleared")
	}
	if cmd.SysProcAttr.CreationFlags&(0x08000000|0x00000008) != 0 {
		t.Errorf("conflicting flags not cleared: %#x", cmd.SysProcAttr.CreationFlags)
	}
	if cmd.SysProcAttr.CreationFlags&0x00000200 == 0 {
		t.Errorf("CREATE_NEW_PROCESS_GROUP missing: %#x", cmd.SysProcAttr.CreationFlags)
	}
}
