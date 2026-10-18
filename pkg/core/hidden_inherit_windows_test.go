//go:build windows

package core

import (
	"os/exec"
	"testing"
)

// Shell children that run ARBITRARY commands (cmd.exe /c ...) must get a hidden
// console WITHOUT CREATE_NO_WINDOW: the command they execute may spawn console
// subprocesses of its own, and with no console at all each of those would
// allocate a visible window.
func TestSetHiddenConsoleInherit_Flags(t *testing.T) {
	cmd := exec.Command("cmd.exe")
	SetHiddenConsoleInherit(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr not initialised")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Error("HideWindow must be set")
	}
	const (
		createNewProcessGrp = 0x00000200
		createNoWindow      = 0x08000000
		detachedProcess     = 0x00000008
	)
	flags := cmd.SysProcAttr.CreationFlags
	if flags&createNewProcessGrp == 0 {
		t.Errorf("CREATE_NEW_PROCESS_GROUP missing (flags=%#x)", flags)
	}
	if flags&createNoWindow != 0 {
		t.Errorf("CREATE_NO_WINDOW must not be set (flags=%#x)", flags)
	}
	if flags&detachedProcess != 0 {
		t.Errorf("DETACHED_PROCESS must not be set (flags=%#x)", flags)
	}
}
