//go:build windows

package core

import "os"

// KillProcessGroup kills a detached process started via LaunchDetachedProcess.
// On Windows there is no process-group signal, so only the direct child is killed.
func KillProcessGroup(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}
