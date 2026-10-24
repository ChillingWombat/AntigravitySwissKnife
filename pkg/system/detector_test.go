package system

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func test(t *testing.T) {
	// dummy
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		v1, v2 string
		want   int
	}{
		{"2.19.1", "2.19.1", 0},
		{"v2.19.1", "2.19.1", 0},
		{"2.19.2", "2.19.1", 1},
		{"2.19.0", "2.19.1", -1},
		{"2.20.0", "2.19.1", 1},
		{"1.6.0", "1.6.0", 0},
		{"1.7.0", "1.6.0", 1},
		{"1.5.9", "1.6.0", -1},
		{"2.19.1.5", "2.19.1", 1},
		{"2.19", "2.19.0", 0},
	}

	for _, tc := range cases {
		got := compareVersions(tc.v1, tc.v2)
		if got != tc.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", tc.v1, tc.v2, got, tc.want)
		}
	}
}

func TestDetector_MockedDesktopAndExtension(t *testing.T) {
	tmpDir := t.TempDir()

	// Mock Desktop App
	desktopDir := filepath.Join(tmpDir, "Antigravity")
	resDir := filepath.Join(desktopDir, "resources")
	_ = os.MkdirAll(resDir, 0755)
	pkgJSON := filepath.Join(resDir, "package.json")
	_ = os.WriteFile(pkgJSON, []byte(`{"name":"antigravity","version":"2.19.1"}`), 0644)

	// Mock VS Code Extension
	vscodeExtsDir := filepath.Join(tmpDir, ".vscode", "extensions")
	extDir := filepath.Join(vscodeExtsDir, "google.google-antigravity-1.6.0")
	_ = os.MkdirAll(extDir, 0755)
	extPkgJSON := filepath.Join(extDir, "package.json")
	_ = os.WriteFile(extPkgJSON, []byte(`{"name":"google-antigravity","version":"1.6.0"}`), 0644)

	// Set env overrides
	t.Setenv("ANTIGRAVITY_APP_PATH", desktopDir)
	t.Setenv("ANTIGRAVITY_RESOURCES_DIR", resDir)
	t.Setenv("VSCODE_EXTENSIONS_DIR", vscodeExtsDir)

	detector := NewDetector()
	res := detector.DetectAll()

	if !res.DesktopApp.Installed {
		t.Errorf("expected DesktopApp.Installed to be true")
	}
	if res.DesktopApp.Version != "2.19.1" {
		t.Errorf("expected DesktopApp.Version = 2.19.1, got %q", res.DesktopApp.Version)
	}
	if !res.DesktopApp.UpToDate {
		t.Errorf("expected DesktopApp.UpToDate to be true")
	}

	if !res.VSCodeExtension.Installed {
		t.Errorf("expected VSCodeExtension.Installed to be true")
	}
	if res.VSCodeExtension.Version != "1.6.0" {
		t.Errorf("expected VSCodeExtension.Version = 1.6.0, got %q", res.VSCodeExtension.Version)
	}
	if !res.VSCodeExtension.UpToDate {
		t.Errorf("expected VSCodeExtension.UpToDate to be true")
	}

	// Test check updates
	upd, err := detector.CheckUpdates()
	if err != nil {
		t.Fatalf("unexpected error on CheckUpdates: %v", err)
	}
	if !upd.DesktopApp.Installed || !upd.VSCodeExtension.Installed {
		t.Errorf("expected both to be installed on recheck")
	}
}

func TestDetector_NotInstalled(t *testing.T) {
	emptyDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_APP_PATH", filepath.Join(emptyDir, "nonexistent-app"))
	t.Setenv("ANTIGRAVITY_RESOURCES_DIR", filepath.Join(emptyDir, "nonexistent-res"))
	t.Setenv("VSCODE_EXTENSIONS_DIR", filepath.Join(emptyDir, "nonexistent-ext"))

	detector := NewDetector()
	res := detector.DetectAll()

	if res.DesktopApp.Installed {
		// Only fail if standard /opt/Antigravity doesn't exist on host system
		if _, err := os.Stat("/opt/Antigravity"); os.IsNotExist(err) {
			t.Errorf("expected DesktopApp.Installed to be false")
		}
	}
	if res.VSCodeExtension.Installed {
		t.Errorf("expected VSCodeExtension.Installed to be false in empty custom dir")
	}
}

func TestDetector_DesktopAppPathResolvesToBinary(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "antigravity")
	_ = os.WriteFile(binPath, []byte("#!/bin/sh\nexit 0\n"), 0755)

	t.Setenv("ANTIGRAVITY_BIN_PATH", binPath)
	t.Setenv("ANTIGRAVITY_APP_PATH", tmpDir)

	detector := NewDetector()
	res := detector.DetectAll()

	if !res.DesktopApp.Installed {
		t.Fatalf("expected DesktopApp.Installed to be true")
	}
	if res.DesktopApp.Path != binPath {
		t.Errorf("expected DesktopApp.Path to resolve to binary %q, got %q", binPath, res.DesktopApp.Path)
	}
	if fi, err := os.Stat(res.DesktopApp.Path); err != nil || fi.IsDir() {
		t.Errorf("expected DesktopApp.Path to be a file, got dir or error: %v", err)
	}
	if res.DesktopApp.Version != "unknown" {
		t.Errorf("expected unprobed desktop version to be unknown, got %q", res.DesktopApp.Version)
	}
	if res.DesktopApp.UpToDate {
		t.Error("expected unprobed desktop version not to report up to date")
	}
}

func TestDetector_DesktopAppPathResolvesDirectoryToBinary(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "antigravity")
	_ = os.WriteFile(binPath, []byte("#!/bin/sh\nexit 0\n"), 0755)

	t.Setenv("ANTIGRAVITY_BIN_PATH", tmpDir)
	t.Setenv("ANTIGRAVITY_APP_PATH", tmpDir)

	detector := NewDetector()
	res := detector.DetectAll()

	if !res.DesktopApp.Installed {
		t.Fatalf("expected DesktopApp.Installed to be true")
	}
	if res.DesktopApp.Path != binPath {
		t.Errorf("expected DesktopApp.Path to resolve directory to binary %q, got %q", binPath, res.DesktopApp.Path)
	}
	if fi, err := os.Stat(res.DesktopApp.Path); err != nil || fi.IsDir() {
		t.Errorf("expected DesktopApp.Path to be a file, got dir or error: %v", err)
	}
}

func TestDetector_AgyVersionProbeUsesLatestVersionAndDoesNotFabricate(t *testing.T) {
	cases := []struct {
		name        string
		posixScript string
		winScript   string
		wantVersion string
		wantCurrent bool
	}{
		{
			name:        "older version",
			posixScript: "#!/bin/sh\nprintf '1.2.0\\n'\n",
			winScript:   "@echo off\r\necho 1.2.0\r\n",
			wantVersion: "1.2.0",
			wantCurrent: false,
		},
		{
			name:        "probe failure",
			posixScript: "#!/bin/sh\nexit 1\n",
			winScript:   "@echo off\r\nexit /b 1\r\n",
			wantVersion: "unknown",
			wantCurrent: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configDir := filepath.Join(tmpDir, "config")
			if err := os.MkdirAll(configDir, 0700); err != nil {
				t.Fatal(err)
			}

			agyName := "agy"
			scriptBody := tc.posixScript
			if runtime.GOOS == "windows" {
				agyName = "agy.cmd"
				scriptBody = tc.winScript
			}
			agyPath := filepath.Join(tmpDir, agyName)
			if err := os.WriteFile(agyPath, []byte(scriptBody), 0755); err != nil {
				t.Fatal(err)
			}
			configJSON := `{"agy_cli_path":` + strconv.Quote(agyPath) + `}`
			if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte(configJSON), 0600); err != nil {
				t.Fatal(err)
			}

			t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", configDir)
			info := NewDetector().DetectAll().AgyCLI
			if !info.Installed {
				t.Fatal("expected configured agy binary to be detected")
			}
			if info.Version != tc.wantVersion {
				t.Errorf("expected agy version %q, got %q", tc.wantVersion, info.Version)
			}
			if info.UpToDate != tc.wantCurrent {
				t.Errorf("expected agy UpToDate=%v, got %v", tc.wantCurrent, info.UpToDate)
			}
		})
	}
}
