//go:build !windows

package acpmesh

import (
	"fmt"
	"time"
)

// probePipe reports that named pipes cannot be dialled on this platform. The
// endpoint stays configurable so a registry shared between hosts still renders.
func probePipe(endpoint string, timeout time.Duration) error {
	return fmt.Errorf("named pipe endpoints are only supported on Windows: %s", endpoint)
}
