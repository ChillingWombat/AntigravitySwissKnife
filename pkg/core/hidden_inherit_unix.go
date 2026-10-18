//go:build !windows

package core

import "os/exec"

// SetHiddenConsoleInherit is a no-op on non-Windows platforms.
func SetHiddenConsoleInherit(cmd *exec.Cmd) {}
