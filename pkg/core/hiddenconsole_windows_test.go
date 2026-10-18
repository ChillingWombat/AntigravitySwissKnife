//go:build windows

package core

import (
	"os/exec"
	"syscall"
	"testing"
)

// Launching a CONSOLE child (agy/code revival) must give it a NEW HIDDEN CONSOLE,
// not no console: console grandchildren inherit the hidden console and stay
// invisible, whereas DETACHED_PROCESS/CREATE_NO_WINDOW makes every console
// grandchild allocate its own VISIBLE window.
func TestSetHiddenConsoleProcess_Flags(t *testing.T) {
	cmd := exec.Command("cmd.exe")
	setHiddenConsoleProcess(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr not initialised")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Error("HideWindow must be set")
	}
	const (
		createNewConsole    = 0x00000010
		createNewProcessGrp = 0x00000200
		createNoWindow      = 0x08000000
		detachedProcess     = 0x00000008
	)
	flags := cmd.SysProcAttr.CreationFlags
	if flags&createNewConsole == 0 {
		t.Errorf("CREATE_NEW_CONSOLE missing (flags=%#x)", flags)
	}
	if flags&createNewProcessGrp == 0 {
		t.Errorf("CREATE_NEW_PROCESS_GROUP missing (flags=%#x)", flags)
	}
	if flags&createNoWindow != 0 {
		t.Errorf("CREATE_NO_WINDOW must not be set — it strips the console so console children open visible windows (flags=%#x)", flags)
	}
	if flags&detachedProcess != 0 {
		t.Errorf("DETACHED_PROCESS must not be set — same reason as CREATE_NO_WINDOW (flags=%#x)", flags)
	}
}

func TestSetHiddenConsoleProcess_ClearsConflictingFlags(t *testing.T) {
	cmd := exec.Command("cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000 | 0x00000008}
	setHiddenConsoleProcess(cmd)

	flags := cmd.SysProcAttr.CreationFlags
	if flags&0x08000000 != 0 || flags&0x00000008 != 0 {
		t.Errorf("conflicting flags not cleared (flags=%#x)", flags)
	}
	if flags&0x00000010 == 0 {
		t.Errorf("CREATE_NEW_CONSOLE missing after clearing conflicts (flags=%#x)", flags)
	}
}
