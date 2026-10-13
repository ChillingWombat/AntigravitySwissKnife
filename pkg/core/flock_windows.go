//go:build windows

package core

import (
	"os"

	"golang.org/x/sys/windows"
)

// LockFile acquires an exclusive lock on the file descriptor using LockFileEx.
func LockFile(f *os.File) error {
	if f == nil {
		return nil
	}
	var ol windows.Overlapped
	return windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &ol)
}

// UnlockFile releases the lock on the file descriptor using UnlockFileEx.
func UnlockFile(f *os.File) error {
	if f == nil {
		return nil
	}
	var ol windows.Overlapped
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &ol)
}
