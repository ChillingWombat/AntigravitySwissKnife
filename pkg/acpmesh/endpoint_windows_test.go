//go:build windows

package acpmesh

import (
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// newTestPipe creates a named pipe server for the test and starts accepting one
// client. The returned cleanup closes the handle.
func newTestPipe(t *testing.T, suffix string) (pipeName string, cleanup func()) {
	t.Helper()
	pipeName = `\\.\pipe\swiss-acp-probe-test-` + strconv.Itoa(os.Getpid()) + "-" + suffix
	namePtr, err := windows.UTF16PtrFromString(pipeName)
	if err != nil {
		t.Fatalf("UTF16PtrFromString failed: %v", err)
	}

	handle, err := windows.CreateNamedPipe(
		namePtr,
		windows.PIPE_ACCESS_DUPLEX,
		windows.PIPE_TYPE_BYTE|windows.PIPE_WAIT,
		1,    // max instances
		4096, // out buffer
		4096, // in buffer
		0,    // default timeout
		nil,  // security attributes
	)
	if err != nil {
		t.Fatalf("CreateNamedPipe failed: %v", err)
	}

	go func() {
		_ = windows.ConnectNamedPipe(handle, nil)
	}()

	return pipeName, func() { _ = windows.CloseHandle(handle) }
}

// TestProbeEndpointNamedPipe verifies the Windows named-pipe probe against a real
// pipe created for the test. This is the code path used for socket-style presets
// on Windows, where unix sockets are unavailable.
func TestProbeEndpointNamedPipe(t *testing.T) {
	pipeName, cleanup := newTestPipe(t, "raw")
	defer cleanup()

	reachable, latency := ProbeEndpoint(pipeName, 500*time.Millisecond)
	if !reachable {
		t.Fatalf("expected the test pipe %s to be reachable", pipeName)
	}
	if latency <= 0 {
		t.Fatalf("expected a positive latency, got %v", latency)
	}
}

func TestProbeEndpointNamedPipeAlias(t *testing.T) {
	pipeName, cleanup := newTestPipe(t, "alias")
	defer cleanup()

	alias := "npipe://" + pipeName[len(`\\.\pipe\`):]
	if reachable, _ := ProbeEndpoint(alias, 500*time.Millisecond); !reachable {
		t.Fatalf("expected the npipe:// alias %q to resolve to the same pipe", alias)
	}
}

func TestProbeEndpointMissingPipe(t *testing.T) {
	reachable, _ := ProbeEndpoint(fmt.Sprintf(`\\.\pipe\swiss-acp-definitely-missing-%d`, os.Getpid()), 100*time.Millisecond)
	if reachable {
		t.Fatal("a missing named pipe must not report reachable")
	}
}
