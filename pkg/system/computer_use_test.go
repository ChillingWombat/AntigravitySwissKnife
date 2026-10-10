package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectHostComputerUse(t *testing.T) {
	status := DetectHostComputerUse()
	if status.CurrentOS == "" {
		t.Fatal("expected non-empty CurrentOS")
	}
	if len(status.Profiles) != 3 {
		t.Fatalf("expected 3 profiles (linux, windows, darwin), got %d", len(status.Profiles))
	}

	linuxProf, ok := status.Profiles["linux"]
	if !ok || linuxProf.OS != "linux" {
		t.Fatal("missing or invalid linux profile")
	}
	if linuxProf.GroundingBackend == "" {
		t.Error("expected non-empty grounding backend for linux")
	}

	winProf, ok := status.Profiles["windows"]
	if !ok || winProf.OS != "windows" {
		t.Fatal("missing or invalid windows profile")
	}
	if winProf.DPIScalingFactor <= 0 {
		t.Errorf("expected positive dpi scale for windows, got %f", winProf.DPIScalingFactor)
	}

	macProf, ok := status.Profiles["darwin"]
	if !ok || macProf.OS != "darwin" {
		t.Fatal("missing or invalid darwin profile")
	}
	if macProf.DPIScalingFactor != 2.0 {
		t.Errorf("expected 2.0x retina scale for darwin, got %f", macProf.DPIScalingFactor)
	}
}

func TestCalibrateDisplayCoordinates(t *testing.T) {
	// Test Linux 1.0x
	resLinux := CalibrateDisplayCoordinates("linux", 100, 200)
	if resLinux.CorrectedTargetX != 100 || resLinux.CorrectedTargetY != 200 {
		t.Errorf("linux calibration failed: expected (100, 200), got (%d, %d)", resLinux.CorrectedTargetX, resLinux.CorrectedTargetY)
	}

	// Test Windows 1.25x
	resWin := CalibrateDisplayCoordinates("windows", 100, 200)
	if resWin.CorrectedTargetX != 125 || resWin.CorrectedTargetY != 250 {
		t.Errorf("windows calibration failed: expected (125, 250), got (%d, %d)", resWin.CorrectedTargetX, resWin.CorrectedTargetY)
	}

	// Test Darwin 2.0x
	resMac := CalibrateDisplayCoordinates("darwin", 100, 200)
	if resMac.CorrectedTargetX != 200 || resMac.CorrectedTargetY != 400 {
		t.Errorf("darwin calibration failed: expected (200, 400), got (%d, %d)", resMac.CorrectedTargetX, resMac.CorrectedTargetY)
	}
}

func TestSaveAndLoadComputerUseSettings(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Ensure config dir exists
	cfgPath := filepath.Join(tmpDir, ".config", "antigravity-swiss")
	_ = os.MkdirAll(cfgPath, 0755)

	initial := LoadComputerUseSettings()
	initial.LinuxDPINormalizer = false
	initial.PerMonitorV2DPI = false

	if err := SaveComputerUseSettings(initial); err != nil {
		t.Fatalf("failed to save settings: %v", err)
	}

	// Invalidate cache
	compUseMu.Lock()
	cachedSettings = nil
	compUseMu.Unlock()

	loaded := LoadComputerUseSettings()
	if loaded.LinuxDPINormalizer != false || loaded.PerMonitorV2DPI != false {
		t.Errorf("expected reloaded settings to reflect saved values")
	}
}
