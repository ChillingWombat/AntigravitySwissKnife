package system

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/process"
)

const (
	LatestDesktopVersion   = "2.19.1"
	LatestAgyVersion       = "1.2.14"
	LatestExtensionVersion = "1.6.0"
)

// InstallationInfo represents installation and update status for a component.
type InstallationInfo struct {
	Installed     bool   `json:"installed"`
	Path          string `json:"path"`
	CustomPath    string `json:"custom_path,omitempty"`
	Version       string `json:"version"`
	UpToDate      bool   `json:"up_to_date"`
	LatestVersion string `json:"latest_version"`
	ProcessState  string `json:"process_state,omitempty"`
	TargetType    string `json:"target_type"`
}

// SystemInstallations aggregates installation status for the desktop app, agy CLI, and VS Code extension.
type SystemInstallations struct {
	DesktopApp      InstallationInfo `json:"desktop_app"`
	AgyCLI          InstallationInfo `json:"agy_cli"`
	VSCodeExtension InstallationInfo `json:"vscode_extension"`
	Platform        string           `json:"platform"`
	Arch            string           `json:"arch"`
}

// Detector handles cross-platform detection of Antigravity installations and updates.
type Detector struct {
	mu           sync.RWMutex
	lastResult   *SystemInstallations
	processShield *process.Shield
}

// NewDetector initializes a new system detector.
func NewDetector() *Detector {
	return &Detector{
		processShield: process.NewShield(0),
	}
}

// DetectAll scans the host system for Antigravity Desktop App, agy CLI, and VS Code Extension.
func (d *Detector) DetectAll() *SystemInstallations {
	d.mu.Lock()
	defer d.mu.Unlock()

	desktop := d.detectDesktopApp()
	agy := d.detectAgyCLI()
	extension := d.detectVSCodeExtension()

	res := &SystemInstallations{
		DesktopApp:      desktop,
		AgyCLI:          agy,
		VSCodeExtension: extension,
		Platform:        runtime.GOOS,
		Arch:            runtime.GOARCH,
	}

	d.lastResult = res
	return res
}

// CheckUpdates performs a re-scan and updates version check.
func (d *Detector) CheckUpdates() (*SystemInstallations, error) {
	return d.DetectAll(), nil
}

func (d *Detector) detectDesktopApp() InstallationInfo {
	appPath := core.GetAntigravityDesktopAppPath()
	binPath := core.GetAntigravityBinaryPath()
	resDir := core.GetAntigravityDesktopResourcesDir()

	info := InstallationInfo{
		Installed:     false,
		Path:          binPath,
		Version:       "",
		UpToDate:      false,
		LatestVersion: LatestDesktopVersion,
		TargetType:    "desktop_app",
		ProcessState:  "Stopped",
	}

	if cfg, _ := core.LoadConfig(); cfg != nil && cfg.DesktopAppPath != "" {
		info.CustomPath = cfg.DesktopAppPath
		if pathExists(cfg.DesktopAppPath) {
			info.Installed = true
			info.Path = cfg.DesktopAppPath
			if fi, err := os.Stat(info.Path); err == nil && fi.IsDir() {
				for _, exeName := range []string{"antigravity", "Antigravity.exe", "Contents/MacOS/Antigravity"} {
					cand := filepath.Join(info.Path, exeName)
					if pathExists(cand) {
						info.Path = cand
						break
					}
				}
			}
			info.Version = LatestDesktopVersion
			info.UpToDate = true
			return info
		}
	}

	// 1. Check if executable / bundle or resources dir exists
	binExists := pathExists(binPath)
	appExists := pathExists(appPath)
	resExists := pathExists(resDir)

	if !binExists && !appExists && !resExists {
		// Try fallback common paths
		fallbacks := []string{
			"/opt/Antigravity/antigravity",
			"/usr/bin/antigravity",
			"/opt/Antigravity",
			"/usr/lib/Antigravity",
			filepath.Join(os.Getenv("HOME"), "Applications", "Antigravity.app"),
		}
		for _, fb := range fallbacks {
			if pathExists(fb) {
				if fi, err := os.Stat(fb); err == nil && !fi.IsDir() {
					binPath = fb
					binExists = true
				} else {
					appPath = fb
					appExists = true
				}
				break
			}
		}
	}

	if binExists || appExists || resExists {
		info.Installed = true
		if binExists {
			info.Path = binPath
		} else if pathExists(filepath.Join(appPath, "antigravity")) {
			info.Path = filepath.Join(appPath, "antigravity")
		} else if pathExists(filepath.Join(appPath, "Antigravity.exe")) {
			info.Path = filepath.Join(appPath, "Antigravity.exe")
		} else if pathExists(filepath.Join(appPath, "Contents", "MacOS", "Antigravity")) {
			info.Path = filepath.Join(appPath, "Contents", "MacOS", "Antigravity")
		} else {
			info.Path = binPath
		}

		// Attempt to extract version from package.json
		version := readPackageJSONVersion(
			filepath.Join(resDir, "app", "package.json"),
			filepath.Join(resDir, "package.json"),
			filepath.Join(appPath, "resources", "app", "package.json"),
			filepath.Join(appPath, "resources", "package.json"),
			filepath.Join(appPath, "package.json"),
			filepath.Join(filepath.Dir(info.Path), "resources", "app", "package.json"),
			filepath.Join(filepath.Dir(info.Path), "resources", "package.json"),
		)

		if version == "" {
			// Fallback to active release baseline if detected
			version = LatestDesktopVersion
		}

		info.Version = version
		info.UpToDate = compareVersions(version, LatestDesktopVersion) >= 0
	}

	// Check running process state
	if d.processShield != nil {
		if procs, err := d.processShield.FindAntigravityProcesses(); err == nil && len(procs) > 0 {
			info.ProcessState = fmt.Sprintf("Running (PID %d)", procs[0].PID)
		}
	}

	return info
}

func (d *Detector) detectAgyCLI() InstallationInfo {
	info := InstallationInfo{
		Installed:     false,
		Path:          "",
		Version:       "",
		UpToDate:      false,
		LatestVersion: LatestAgyVersion,
		TargetType:    "agy_cli",
		ProcessState:  "Ready",
	}

	if cfg, _ := core.LoadConfig(); cfg != nil && cfg.AgyCLIPath != "" {
		info.CustomPath = cfg.AgyCLIPath
		if pathExists(cfg.AgyCLIPath) {
			info.Installed = true
			info.Path = cfg.AgyCLIPath
		}
	}

	if !info.Installed {
		if p, err := exec.LookPath("agy"); err == nil && p != "" {
			info.Installed = true
			info.Path = p
		} else {
			home, _ := os.UserHomeDir()
			candidates := []string{
				filepath.Join(home, ".local", "bin", "agy"),
				filepath.Join(home, ".cargo", "bin", "agy"),
				"/usr/local/bin/agy",
				"/usr/bin/agy",
			}
			for _, cand := range candidates {
				if pathExists(cand) {
					info.Installed = true
					info.Path = cand
					break
				}
			}
		}
	}

	if info.Installed {
		cmd := exec.Command(info.Path, "--version")
		if out, err := cmd.Output(); err == nil {
			v := strings.TrimSpace(string(out))
			if v != "" {
				info.Version = v
			}
		}
		if info.Version == "" {
			info.Version = LatestAgyVersion
		}
		info.UpToDate = compareVersions(info.Version, "1.2.0") >= 0
	}

	return info
}

func (d *Detector) detectVSCodeExtension() InstallationInfo {
	info := InstallationInfo{
		Installed:     false,
		Path:          "",
		Version:       "",
		UpToDate:      false,
		LatestVersion: LatestExtensionVersion,
		TargetType:    "vscode_extension",
	}

	if cfg, _ := core.LoadConfig(); cfg != nil && cfg.VSCodeExtensionPath != "" {
		info.CustomPath = cfg.VSCodeExtensionPath
		if pathExists(cfg.VSCodeExtensionPath) {
			info.Installed = true
			info.Path = cfg.VSCodeExtensionPath
			info.Version = LatestExtensionVersion
			info.UpToDate = true
			return info
		}
	}

	candidateDirs := core.GetVSCodeExtensionsDirs()

	for _, parentDir := range candidateDirs {
		if !pathExists(parentDir) {
			continue
		}

		entries, err := os.ReadDir(parentDir)
		if err != nil {
			continue
		}

		// Look for google.google-antigravity-* or google.antigravity-*
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			if strings.HasPrefix(name, "google.google-antigravity-") ||
				strings.HasPrefix(name, "google.antigravity-") ||
				strings.Contains(name, "antigravity") {
				extPath := filepath.Join(parentDir, name)
				pkgJSON := filepath.Join(extPath, "package.json")
				version := ""
				if data, err := os.ReadFile(pkgJSON); err == nil {
					var pkg struct {
						Version string `json:"version"`
					}
					if err := json.Unmarshal(data, &pkg); err == nil && pkg.Version != "" {
						version = pkg.Version
					}
				}

				if version == "" {
					// Extract version from directory name e.g. google.google-antigravity-1.6.0
					parts := strings.Split(name, "-")
					if len(parts) > 1 {
						last := parts[len(parts)-1]
						if strings.Count(last, ".") >= 1 {
							version = last
						}
					}
				}

				if version == "" {
					version = LatestExtensionVersion
				}

				info.Installed = true
				info.Path = extPath
				info.Version = version
				info.UpToDate = compareVersions(version, LatestExtensionVersion) >= 0
				return info
			}
		}
	}

	return info
}

func readPackageJSONVersion(paths ...string) string {
	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			var pkg struct {
				Version string `json:"version"`
			}
			if err := json.Unmarshal(data, &pkg); err == nil && pkg.Version != "" {
				return pkg.Version
			}
		}
	}
	return ""
}

func pathExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// compareVersions returns 1 if v1 > v2, -1 if v1 < v2, 0 if v1 == v2.
func compareVersions(v1, v2 string) int {
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var num1, num2 int
		if i < len(parts1) {
			num1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			num2, _ = strconv.Atoi(parts2[i])
		}
		if num1 > num2 {
			return 1
		}
		if num1 < num2 {
			return -1
		}
	}
	return 0
}
