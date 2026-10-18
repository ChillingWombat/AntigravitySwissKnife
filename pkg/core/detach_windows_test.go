//go:build windows

package core

import (
	"os/exec"
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
