package process

import (
	"testing"
)

func TestAdversarial_RelaunchHostIDE_ClosedIDE_NeverSpawns(t *testing.T) {
	shield := NewShield(0)
	shield.SetProcessFinder(func() ([]ProcessInfo, error) {
		return []ProcessInfo{}, nil
	})

	err := shield.RelaunchHostIDE()
	if err != nil {
		t.Fatalf("expected nil error when Antigravity is closed, got %v", err)
	}
}

func TestAdversarial_IsAntigravityRunning_ProcessFiltering(t *testing.T) {
	shield := NewShield(0)

	// 1. Zero processes
	shield.SetProcessFinder(func() ([]ProcessInfo, error) {
		return []ProcessInfo{}, nil
	})
	if shield.IsAntigravityRunning() {
		t.Errorf("expected IsAntigravityRunning=false when no processes exist")
	}

	// 2. Only auxiliary processes (e.g., electron renderers or swiss daemon itself)
	shield.SetProcessFinder(func() ([]ProcessInfo, error) {
		return []ProcessInfo{
			{
				PID:     1001,
				Name:    "antigravity",
				Cmdline: "/opt/antigravity/antigravity --type=renderer --field-trial-handle=0",
			},
			{
				PID:     1002,
				Name:    "antigravity",
				Cmdline: "/opt/antigravity/antigravity --type=zygote",
			},
			{
				PID:     1003,
				Name:    "swiss",
				Cmdline: "bin/swiss daemon --web",
			},
		}, nil
	})
	if shield.IsAntigravityRunning() {
		t.Errorf("expected IsAntigravityRunning=false when only auxiliary/swiss processes exist")
	}

	// 3. Main Antigravity IDE process exists
	shield.SetProcessFinder(func() ([]ProcessInfo, error) {
		return []ProcessInfo{
			{
				PID:     2001,
				Name:    "antigravity",
				Cmdline: "/opt/Antigravity/antigravity",
			},
		}, nil
	})
	if !shield.IsAntigravityRunning() {
		t.Errorf("expected IsAntigravityRunning=true when main IDE process is present")
	}
}
