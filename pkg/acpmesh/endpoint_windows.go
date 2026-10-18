//go:build windows

package acpmesh

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// probePipe opens a Windows named pipe. Opening a pipe blocks until a server
// instance is available, so the open runs in a goroutine guarded by timeout.
// The goroutine is left to finish on its own when the timeout fires.
func probePipe(endpoint string, timeout time.Duration) error {
	name := endpoint
	if strings.HasPrefix(name, "npipe://") {
		name = `\\.\pipe\` + strings.TrimPrefix(name, "npipe://")
	}

	done := make(chan error, 1)
	go func() {
		f, err := os.OpenFile(name, os.O_RDWR, 0)
		if err == nil {
			_ = f.Close()
		}
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("timed out opening named pipe %s", name)
	}
}
