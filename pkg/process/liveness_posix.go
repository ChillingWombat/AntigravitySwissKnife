//go:build !windows

package process

import (
	"os"
	"syscall"
)

func isSystemProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
