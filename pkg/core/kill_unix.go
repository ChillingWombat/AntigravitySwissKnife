//go:build !windows

package core

import "syscall"

// KillProcessGroup kills a detached process group started via LaunchDetachedProcess.
// The process must have been started with Setsid (which LaunchDetachedProcess does),
// so its process group ID equals its PID.
func KillProcessGroup(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}
