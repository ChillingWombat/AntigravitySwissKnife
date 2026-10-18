package process

import (
	"os"
	"strings"
	"testing"
)

// TestListProcessesIncludesSelf verifies the platform process enumerator on the
// host running the tests: the current process must be present with both an
// executable name and a readable command line.
func TestListProcessesIncludesSelf(t *testing.T) {
	procs, err := ListProcesses()
	if err != nil {
		t.Fatalf("ListProcesses failed: %v", err)
	}
	if len(procs) == 0 {
		t.Fatal("ListProcesses returned no entries")
	}

	self := os.Getpid()
	for _, p := range procs {
		if p.PID != self {
			continue
		}
		if strings.TrimSpace(p.Comm) == "" {
			t.Fatal("current process must report an executable name")
		}
		if strings.TrimSpace(p.Cmdline) == "" {
			t.Fatal("current process must report a command line")
		}
		return
	}
	t.Fatalf("current process %d missing from %d enumerated entries", self, len(procs))
}
