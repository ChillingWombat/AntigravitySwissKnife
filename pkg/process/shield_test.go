package process

import (
	"errors"
	"os"
	"syscall"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/revival"
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

func TestGetAntigravityBinaryPath(t *testing.T) {
	p := core.GetAntigravityBinaryPath()
	if p == "" {
		t.Fatalf("expected non-empty Antigravity binary path")
	}
}

func TestRelaunchHostIDEDryRun(t *testing.T) {
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")
	shield := NewShield(0)
	if err := shield.RelaunchHostIDE(); err != nil {
		t.Fatalf("expected nil error on dry run relaunch, got %v", err)
	}
}

func TestIsProcessAlive(t *testing.T) {
	if !isProcessAlive(os.Getpid()) {
		t.Fatalf("expected current process PID %d to be alive", os.Getpid())
	}
	if isProcessAlive(-999) {
		t.Fatalf("expected negative PID to be reported as not alive")
	}
}

func TestRelaunchHostIDE_ClosedIDEDoesNotSpawn(t *testing.T) {
	shield := NewShield(0)
	shield.SetProcessFinder(func() ([]ProcessInfo, error) {
		return []ProcessInfo{}, nil
	})
	// When Antigravity is not running, RelaunchHostIDE must return nil without error and without launching
	err := shield.RelaunchHostIDE()
	if err != nil {
		t.Fatalf("expected nil error when Antigravity is not running, got %v", err)
	}
}

func TestShield_RevivalEngineCoordination(t *testing.T) {
	shield := NewShield(0)
	tmpDir := t.TempDir()
	engine := revival.NewEngine(tmpDir, 9222)
	shield.SetRevivalEngine(engine)

	if shield.getRevivalEngine() != engine {
		t.Fatalf("expected shield to return configured revival engine")
	}
}

func TestLaunchHostIDE_RunningCallsFocus(t *testing.T) {
	called := false
	focusCalled := false
	shield := NewShield(0)
	shield.SetProcessFinder(func() ([]ProcessInfo, error) {
		called = true
		return []ProcessInfo{
			{PID: 1234, Name: "antigravity", Cmdline: "/opt/Antigravity/antigravity"},
		}, nil
	})
	shield.SetFocusFunc(func() error {
		focusCalled = true
		return nil
	})

	err := shield.LaunchHostIDE()
	if err != nil {
		t.Fatalf("expected nil error on LaunchHostIDE when already running, got: %v", err)
	}
	if !called {
		t.Fatalf("expected processFinder to be invoked when checking if Antigravity is running")
	}
	if !focusCalled {
		t.Fatalf("expected focusFunc to be invoked when Antigravity is detected running")
	}
}

func TestShield_IsAntigravityRunningWindowsAndMixedCase(t *testing.T) {
	shield := NewShield(0)
	shield.SetProcessFinder(func() ([]ProcessInfo, error) {
		return []ProcessInfo{
			{PID: 5678, Name: "Antigravity.exe", Cmdline: `C:\Program Files\Antigravity\Antigravity.exe`},
		}, nil
	})

	if !shield.IsAntigravityRunning() {
		t.Fatalf("expected Antigravity.exe to be detected as running")
	}
}

func TestLaunchHostIDE_ClosedIDETestMode(t *testing.T) {
	shield := NewShield(0)
	shield.SetProcessFinder(func() ([]ProcessInfo, error) {
		return []ProcessInfo{}, nil
	})

	err := shield.LaunchHostIDE()
	if err != nil {
		t.Fatalf("expected nil error on LaunchHostIDE in test mode when stopped, got: %v", err)
	}
}


