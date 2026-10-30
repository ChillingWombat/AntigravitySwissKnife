package custommodels

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

const wrapperMarker = "# Antigravity CLI (agy) - Swiss Knife Integration Wrapper"
const legacyWrapperMarker = "# Antigravity CLI (agy) - Swiss Knife Gateway Integration Wrapper"

// CLIGatewayStatus reports the active state of CLI gateway integration.
type CLIGatewayStatus struct {
	Installed     bool     `json:"installed"`
	Configured    bool     `json:"configured"`
	CLIPath       string   `json:"cli_path"`
	WrapperPath   string   `json:"wrapper_path,omitempty"`
	ShellRCHooked bool     `json:"shell_rc_hooked"`
	Platform      string   `json:"platform"`
	GatewayURL    string   `json:"gateway_url"`
	GatewayToken  string   `json:"gateway_token"`
	EnabledModels []string `json:"enabled_models"`
}

// FindAgyBinary locates the agy executable across standard platform paths.
func FindAgyBinary() string {
	binName := "agy"
	if runtime.GOOS == "windows" {
		binName = "agy.exe"
	}

	if p, err := exec.LookPath(binName); err == nil && p != "" {
		return p
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	var candidates []string
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		candidates = []string{
			filepath.Join(home, ".local", "bin", "agy.exe"),
			filepath.Join(localAppData, "Programs", "Antigravity", "agy.exe"),
			filepath.Join(localAppData, "Programs", "Antigravity CLI", "agy.exe"),
		}
	} else {
		candidates = []string{
			filepath.Join(home, ".local", "bin", "agy"),
			filepath.Join(home, ".local", "bin", "agy.bin"),
			"/usr/local/bin/agy",
			"/usr/bin/agy",
			filepath.Join(home, ".cargo", "bin", "agy"),
		}
	}

	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

// CheckCLIGatewayStatus evaluates whether agy is installed and routed through Swiss Knife.
func CheckCLIGatewayStatus(store *Store) CLIGatewayStatus {
	status := CLIGatewayStatus{
		Platform:   runtime.GOOS,
		GatewayURL: "http://127.0.0.1:8765",
	}
	if store != nil {
		status.GatewayToken = store.GatewayToken()
		for _, m := range store.GetConfig().Models {
			if m.Enabled {
				slug := m.Name
				if slug == "" {
					slug = m.ID
				}
				status.EnabledModels = append(status.EnabledModels, slug)
			}
		}
	}

	cliPath := FindAgyBinary()
	if cliPath == "" {
		return status
	}
	status.Installed = true
	status.CLIPath = cliPath

	if runtime.GOOS == "windows" {
		dir := filepath.Dir(cliPath)
		cmdWrapper := filepath.Join(dir, "agy.cmd")
		psWrapper := filepath.Join(dir, "agy.ps1")
		if fileExists(cmdWrapper) || fileExists(psWrapper) {
			status.Configured = true
			status.WrapperPath = cmdWrapper
		}
		if os.Getenv("CLOUD_CODE_URL") != "" || os.Getenv("AGY_LLM_GATEWAY_URL") != "" {
			status.Configured = true
			status.ShellRCHooked = true
		}
	} else {
		dir := filepath.Dir(cliPath)
		binPath := filepath.Join(dir, "agy.bin")
		mainPath := filepath.Join(dir, "agy")

		data, err := os.ReadFile(mainPath)
		if err == nil && (strings.Contains(string(data), wrapperMarker) || strings.Contains(string(data), legacyWrapperMarker) || strings.Contains(string(data), "CLOUD_CODE_URL")) {
			status.Configured = true
			status.WrapperPath = mainPath
		} else if fileExists(binPath) {
			status.Configured = true
			status.WrapperPath = mainPath
		}

		home, _ := os.UserHomeDir()
		if home != "" {
			zshrc := filepath.Join(home, ".zshrc")
			if d, err := os.ReadFile(zshrc); err == nil && (strings.Contains(string(d), "CLOUD_CODE_URL") || strings.Contains(string(d), "AGY_LLM_GATEWAY_URL")) {
				status.ShellRCHooked = true
			}
			bashrc := filepath.Join(home, ".bashrc")
			if d, err := os.ReadFile(bashrc); err == nil && (strings.Contains(string(d), "CLOUD_CODE_URL") || strings.Contains(string(d), "AGY_LLM_GATEWAY_URL")) {
				status.ShellRCHooked = true
			}
		}
	}

	return status
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// SetupCLIGatewayIntegration configures the local agy installation to connect to Swiss Knife.
func SetupCLIGatewayIntegration(store *Store) error {
	cliPath := FindAgyBinary()
	if cliPath == "" {
		return fmt.Errorf("Antigravity CLI (agy) not found on PATH or standard locations")
	}

	if runtime.GOOS == "windows" {
		dir := filepath.Dir(cliPath)
		cmdContent := `@echo off
set AGY_LLM_GATEWAY_URL=
set AGY_LLM_GATEWAY_API_KEY=
set AGY_LLM_GATEWAY_MODELS=
powershell -NoProfile -Command "$tcp = New-Object Net.Sockets.TcpClient; try { $tcp.Connect('127.0.0.1', 8765); exit 0 } catch { exit 1 }" >nul 2>&1
if %ERRORLEVEL% equ 0 (
    set CLOUD_CODE_URL=http://127.0.0.1:8765
) else (
    set CLOUD_CODE_URL=
)
"%~dp0agy.exe" %*
`
		_ = os.WriteFile(filepath.Join(dir, "agy.cmd"), []byte(cmdContent), 0755)

		psContent := `Remove-Item Env:\AGY_LLM_GATEWAY_URL -ErrorAction SilentlyContinue
Remove-Item Env:\AGY_LLM_GATEWAY_API_KEY -ErrorAction SilentlyContinue
Remove-Item Env:\AGY_LLM_GATEWAY_MODELS -ErrorAction SilentlyContinue
$portOpen = $false
try {
    $tcp = New-Object Net.Sockets.TcpClient
    $tcp.Connect("127.0.0.1", 8765)
    $portOpen = $tcp.Connected
    $tcp.Close()
} catch {}
if ($portOpen) {
    $env:CLOUD_CODE_URL = "http://127.0.0.1:8765"
} else {
    Remove-Item Env:\CLOUD_CODE_URL -ErrorAction SilentlyContinue
}
& "$PSScriptRoot\agy.exe" @args
`
		_ = os.WriteFile(filepath.Join(dir, "agy.ps1"), []byte(psContent), 0755)
		return nil
	}

	// Linux & macOS: Wrapper script
	dir := filepath.Dir(cliPath)
	binPath := filepath.Join(dir, "agy.bin")
	wrapperPath := filepath.Join(dir, "agy")

	// If cliPath is not already agy.bin, check if it's the raw binary
	if cliPath != binPath {
		data, err := os.ReadFile(cliPath)
		if err == nil {
			if !strings.Contains(string(data), wrapperMarker) && !strings.Contains(string(data), legacyWrapperMarker) {
				// It's the real binary: copy to agy.bin
				_ = os.WriteFile(binPath, data, 0755)
			}
		}
	}

	wrapperScript := fmt.Sprintf(`#!/usr/bin/env bash
%s
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REAL_BIN="${DIR}/agy.bin"
if [ ! -f "$REAL_BIN" ]; then
    REAL_BIN="${DIR}/agy.real"
fi
if [ ! -f "$REAL_BIN" ]; then
    REAL_BIN="%s"
fi

# Route agy through Swiss Knife Cloud Code proxy if active; fallback to native if closed
PROXY_PORT=8765
if (exec 3<>/dev/tcp/127.0.0.1/${PROXY_PORT}) 2>/dev/null; then
    exec 3>&-
    export CLOUD_CODE_URL="http://127.0.0.1:${PROXY_PORT}"
    unset AGY_LLM_GATEWAY_URL AGY_LLM_GATEWAY_API_KEY AGY_LLM_GATEWAY_MODELS 2>/dev/null || true
else
    unset CLOUD_CODE_URL AGY_LLM_GATEWAY_URL AGY_LLM_GATEWAY_API_KEY AGY_LLM_GATEWAY_MODELS 2>/dev/null || true
fi

exec "$REAL_BIN" "$@"
`, wrapperMarker, binPath)

	if err := os.WriteFile(wrapperPath, []byte(wrapperScript), 0755); err != nil {
		return fmt.Errorf("write agy wrapper: %w", err)
	}

	CleanShellRCHooks()

	return nil
}

// CleanShellRCHooks removes static proxy environment variables from user shell RC files
// to ensure native models remain available when Swiss Knife is closed.
func CleanShellRCHooks() {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return
	}
	cleanSingleShellRC(filepath.Join(home, ".zshrc"))
	cleanSingleShellRC(filepath.Join(home, ".bashrc"))
}

func cleanSingleShellRC(rcPath string) {
	data, err := os.ReadFile(rcPath)
	if err != nil {
		return
	}
	content := string(data)
	if !strings.Contains(content, "CLOUD_CODE_URL") && !strings.Contains(content, "AGY_LLM_GATEWAY_URL") {
		return
	}
	lines := strings.Split(content, "\n")
	var filtered []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "Antigravity Swiss Knife - CLI Integration") ||
			strings.Contains(trimmed, "Antigravity Swiss Knife - CLI Gateway") ||
			strings.HasPrefix(trimmed, "export CLOUD_CODE_URL=") ||
			strings.HasPrefix(trimmed, "export AGY_LLM_GATEWAY_URL=") {
			continue
		}
		filtered = append(filtered, line)
	}
	newContent := strings.Join(filtered, "\n")
	if newContent != content {
		_ = os.WriteFile(rcPath, []byte(newContent), 0644)
	}
}

// AutoEnsureCLIGatewayIntegration is called during daemon start or installation.
func AutoEnsureCLIGatewayIntegration(store *Store) {
	if core.IsRunningTests() {
		return
	}
	status := CheckCLIGatewayStatus(store)
	if status.Installed && !status.Configured {
		_ = SetupCLIGatewayIntegration(store)
	}
}
