package process

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// ProcessInfo holds information about a detected system process.
type ProcessInfo struct {
	PID         int    `json:"pid"`
	PPID        int    `json:"ppid"`
	Name        string `json:"name"`
	Cmdline     string `json:"cmdline"`
	IsHostIDE   bool   `json:"is_host_ide"`
	IsProtected bool   `json:"is_protected"`
}

// Shield safeguards the host Antigravity IDE from accidental termination.
type Shield struct {
	protectedPID int
}

// NewShield initializes a Shield. If protectedPID is 0, it reads from environment or detects host.
func NewShield(protectedPID int) *Shield {
	if protectedPID <= 0 {
		if envPID := os.Getenv("ANTIGRAVITY_HOST_PID"); envPID != "" {
			if p, err := strconv.Atoi(envPID); err == nil && p > 0 {
				protectedPID = p
			}
		}
	}
	return &Shield{protectedPID: protectedPID}
}

// ProtectedPID returns the currently protected host PID.
func (s *Shield) ProtectedPID() int {
	return s.protectedPID
}

// SetProtectedPID updates the protected PID.
func (s *Shield) SetProtectedPID(pid int) {
	s.protectedPID = pid
}

// IsProtected returns true if the PID must not be signaled or terminated.
func (s *Shield) IsProtected(pid int) bool {
	if pid <= 0 {
		return true // invalid PID
	}
	if s.protectedPID > 0 && pid == s.protectedPID {
		return true
	}
	// Own process is always protected
	if pid == os.Getpid() {
		return true
	}
	return false
}

// SafeKill sends a signal to a PID only if it is NOT protected.
func (s *Shield) SafeKill(pid int, sig syscall.Signal) error {
	if s.IsProtected(pid) {
		return fmt.Errorf("%w: PID %d", core.ErrHostProtected, pid)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(sig)
}

// FindAntigravityProcesses scans Linux /proc for running antigravity instances.
func (s *Shield) FindAntigravityProcesses() ([]ProcessInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	var results []ProcessInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}

		cmdlineBytes, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil {
			continue
		}
		cmdline := strings.ReplaceAll(string(cmdlineBytes), "\x00", " ")
		if strings.Contains(strings.ToLower(cmdline), "antigravity") {
			commBytes, _ := os.ReadFile(filepath.Join("/proc", entry.Name(), "comm"))
			comm := strings.TrimSpace(string(commBytes))

			isHost := (s.protectedPID > 0 && pid == s.protectedPID)
			results = append(results, ProcessInfo{
				PID:         pid,
				Name:        comm,
				Cmdline:     strings.TrimSpace(cmdline),
				IsHostIDE:   isHost,
				IsProtected: s.IsProtected(pid),
			})
		}
	}
	return results, nil
}
