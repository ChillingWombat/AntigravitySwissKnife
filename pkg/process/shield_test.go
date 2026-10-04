package process

import (
	"errors"
	"os"
	"syscall"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

func TestShieldPreventsHostKill(t *testing.T) {
	hostPID := 99999
	shield := NewShield(hostPID)

	if !shield.IsProtected(hostPID) {
		t.Fatalf("expected host PID %d to be protected", hostPID)
	}

	err := shield.SafeKill(hostPID, syscall.SIGTERM)
	if err == nil {
		t.Fatalf("expected error when trying to kill protected host PID")
	}
	if !errors.Is(err, core.ErrHostProtected) {
		t.Fatalf("expected ErrHostProtected, got: %v", err)
	}
}

func TestShieldProtectsSelf(t *testing.T) {
	shield := NewShield(0)
	if !shield.IsProtected(os.Getpid()) {
		t.Fatalf("expected own PID to be protected")
	}
}
