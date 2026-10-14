package process

import (
	"testing"
)

func TestChallenge_ProcessShield_CasingAndWhitespaceVariants(t *testing.T) {
	// 1. Valid exact matches with casing, whitespace, and path variants
	validVariants := []string{
		"antigravity",
		"Antigravity",
		"ANTIGRAVITY",
		"aNtIgRaViTy",
		"antigravity.exe",
		"Antigravity.exe",
		"ANTIGRAVITY.EXE",
		"Antigravity.Exe",
		"  antigravity  ",
		"\tantigravity.exe\n",
		" \r\n ANTIGRAVITY.EXE \t ",
		`/usr/bin/antigravity`,
		`/usr/bin/Antigravity`,
		`/opt/Antigravity/antigravity`,
		`/opt/google/antigravity/antigravity`,
		`C:\Program Files\Google\Antigravity\Antigravity.exe`,
		`C:\Program Files\Google\Antigravity\ANTIGRAVITY.EXE`,
		`C:\Users\David\AppData\Local\Programs\antigravity\antigravity.exe`,
		`D:\Games\Antigravity\Antigravity.Exe`,
		`\\network-share\tools\Antigravity.exe`,
	}

	for _, v := range validVariants {
		if !IsExactAntigravityExecutable(v) {
			t.Errorf("FAIL: Expected IsExactAntigravityExecutable(%q) = true, got false", v)
		}
	}

	// 2. Adversarial invalid variants: deceptively named files, extensions, companions
	invalidVariants := []string{
		"antigravity.exe.lnk",
		"antigravity.exe.bat",
		"antigravity.exe.cmd",
		"antigravity.exe.ps1",
		"antigravity.exe.tmp",
		"antigravity.exe.old",
		"antigravity.exe.bak",
		"antigravity.exe:ads",
		"antigravity-swiss",
		"antigravity-swiss.exe",
		"swiss-antigravity.exe",
		"antigravity_swiss.exe",
		"swiss.exe",
		"swiss",
		"antigravity-helper",
		"antigravity-helper.exe",
		"antigravity_crashpad_handler.exe",
		"antigravity.dll",
		"antigravity.so",
		"antigravity.dylib",
		"libantigravity.so",
		"not-antigravity.exe",
		"fake_antigravity.exe",
		"antigravity_agent",
		"python -m antigravity",
		"node antigravity.js",
		"",
		"   ",
		"\t\r\n",
	}

	for _, v := range invalidVariants {
		if IsExactAntigravityExecutable(v) {
			t.Errorf("FAIL: Expected IsExactAntigravityExecutable(%q) = false, got true", v)
		}
	}
}

func TestChallenge_ProcessShield_ProcessFinderIntegration(t *testing.T) {
	shield := NewShield(0)

	// Simulate running processes with casing and whitespace
	shield.SetProcessFinder(func() ([]ProcessInfo, error) {
		return []ProcessInfo{
			{
				PID:     101,
				Name:    "ANTIGRAVITY.EXE",
				Cmdline: `C:\Program Files\Google\Antigravity\ANTIGRAVITY.EXE`,
			},
			{
				PID:     102,
				Name:    "antigravity-swiss.exe",
				Cmdline: `C:\swiss\antigravity-swiss.exe daemon`,
			},
			{
				PID:     103,
				Name:    "antigravity.exe",
				Cmdline: `C:\Program Files\Google\Antigravity\antigravity.exe --type=crashpad-handler`,
			},
		}, nil
	})

	if !shield.IsAntigravityRunning() {
		t.Fatalf("FAIL: IsAntigravityRunning() should be true when 'ANTIGRAVITY.EXE' is running")
	}

	// Now simulate only crashpad handler and swiss running
	shield.SetProcessFinder(func() ([]ProcessInfo, error) {
		return []ProcessInfo{
			{
				PID:     102,
				Name:    "antigravity-swiss.exe",
				Cmdline: `C:\swiss\antigravity-swiss.exe daemon`,
			},
			{
				PID:     103,
				Name:    "antigravity.exe",
				Cmdline: `C:\Program Files\Google\Antigravity\antigravity.exe --type=crashpad-handler`,
			},
		}, nil
	})

	if shield.IsAntigravityRunning() {
		t.Fatalf("FAIL: IsAntigravityRunning() should be false when only crashpad-handler and swiss daemon exist")
	}
}
