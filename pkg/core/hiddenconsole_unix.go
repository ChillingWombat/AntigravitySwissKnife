//go:build !windows

package core

import "os/exec"

// setHiddenConsoleProcess is equivalent to SetDetachedProcess on Unix: there is no
// Windows console concept, so the child simply gets its own session.
func setHiddenConsoleProcess(cmd *exec.Cmd) {
	SetDetachedProcess(cmd)
}
