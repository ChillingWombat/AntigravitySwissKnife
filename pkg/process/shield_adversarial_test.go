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

func TestAdversarial_IsExactAntigravityExecutable(t *testing.T) {
	valid := []string{
		"antigravity",
		"antigravity.exe",
		"Antigravity",
		"Antigravity.exe",
		"/usr/bin/antigravity",
		"/opt/Antigravity/antigravity",
		`C:\Program Files\Antigravity\Antigravity.exe`,
		`C:\Users\David\AppData\Local\Programs\antigravity\Antigravity.exe`,
	}
	for _, tc := range valid {
		if !IsExactAntigravityExecutable(tc) {
			t.Errorf("expected IsExactAntigravityExecutable(%q) = true, got false", tc)
		}
	}

	invalid := []string{
		"python -m antigravity",
		"python3 -m antigravity",
		"node antigravity.js",
		"bash antigravity.sh",
		"fake-antigravity",
		"fake-antigravity.exe",
		"antigravity-helper",
		"libantigravity.so",
		"antigravity.dll",
		"swiss",
		"swiss.exe",
		"antigravity-swiss",
		"antigravity-swiss.exe",
		"",
		"   ",
	}
	for _, tc := range invalid {
		if IsExactAntigravityExecutable(tc) {
			t.Errorf("expected IsExactAntigravityExecutable(%q) = false, got true", tc)
		}
	}
}

func TestAdversarial_UnrelatedProcessesIgnored(t *testing.T) {
	shield := NewShield(0)

	// User is running unrelated processes with "antigravity" in arguments or substrings
	shield.SetProcessFinder(func() ([]ProcessInfo, error) {
		return []ProcessInfo{
			{
				PID:     3001,
				Name:    "python3",
				Cmdline: "python3 -m antigravity",
			},
			{
				PID:     3002,
				Name:    "grep",
				Cmdline: "grep -r antigravity /var/log",
			},
			{
				PID:     3003,
				Name:    "fake-antigravity",
				Cmdline: "/usr/local/bin/fake-antigravity",
			},
			{
				PID:     3004,
				Name:    "bash",
				Cmdline: "bash -c 'while true; do echo antigravity; sleep 1; done'",
			},
		}, nil
	})

	if shield.IsAntigravityRunning() {
		t.Fatalf("expected IsAntigravityRunning=false when only unrelated processes contain 'antigravity'")
	}
}

