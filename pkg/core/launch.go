package core

import (
	"os"
	"os/exec"
)

// LaunchDetachedProcess starts a child command fully detached from the current process,
// with Stdin, Stdout, and Stderr redirected to /dev/null, its own process group/session,
// and a background reaper to avoid zombie processes.
func LaunchDetachedProcess(cmd *exec.Cmd) error {
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err == nil {
		defer devNull.Close()
		cmd.Stdin = devNull
		cmd.Stdout = devNull
		cmd.Stderr = devNull
	}
	SetDetachedProcess(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		_ = cmd.Wait()
	}()
	return nil
}
