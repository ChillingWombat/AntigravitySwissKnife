package core

import (
	"os"
	"os/exec"
)

// LaunchDetachedProcess starts a child command fully detached from the current process,
// with Stdin, Stdout, and Stderr redirected to /dev/null, its own process group/session,
// and a background reaper to avoid zombie processes.
func LaunchDetachedProcess(cmd *exec.Cmd) error {
	return startDetached(cmd, SetDetachedProcess)
}

// LaunchDetachedProcessHiddenConsole is LaunchDetachedProcess with a hidden console
// (Windows) so console children of the launched GUI app do not open visible windows.
func LaunchDetachedProcessHiddenConsole(cmd *exec.Cmd) error {
	return startDetached(cmd, SetDetachedProcessHiddenConsole)
}

// LaunchHiddenConsoleProcess launches a CONSOLE application (CLI tools such as agy or
// code) detached, in its own new process group with a HIDDEN console.
//
// Unlike DETACHED_PROCESS / CREATE_NO_WINDOW (no console at all), a hidden console is
// INHERITED by console children the app spawns (e.g. agy relaunching its node helpers):
// they attach to the hidden console instead of allocating their own VISIBLE windows —
// one launch could otherwise pop a dozen terminals.
func LaunchHiddenConsoleProcess(cmd *exec.Cmd) error {
	return startDetached(cmd, setHiddenConsoleProcess)
}

// startDetached redirects stdio to the null device, applies the platform-specific
// process configuration, starts the child, and reaps it in the background.
func startDetached(cmd *exec.Cmd, configure func(*exec.Cmd)) error {
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err == nil {
		defer devNull.Close()
		cmd.Stdin = devNull
		cmd.Stdout = devNull
		cmd.Stderr = devNull
	}
	configure(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		_ = cmd.Wait()
	}()
	return nil
}
