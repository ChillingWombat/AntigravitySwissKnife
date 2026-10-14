//go:build !windows

package core

import "os/exec"

// SetHiddenConsole is a no-op on non-Windows platforms.
func SetHiddenConsole(cmd *exec.Cmd) {}
