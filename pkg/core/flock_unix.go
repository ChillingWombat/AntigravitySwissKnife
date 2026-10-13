//go:build !windows

package core

import (
	"os"
	"syscall"
)

// LockFile acquires an exclusive lock on the file descriptor using flock.
func LockFile(f *os.File) error {
	if f == nil {
		return nil
	}
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// UnlockFile releases the lock on the file descriptor using flock.
func UnlockFile(f *os.File) error {
	if f == nil {
		return nil
	}
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
