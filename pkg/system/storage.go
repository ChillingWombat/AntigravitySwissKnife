package system

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// StoragePaths defines the core directories used by the application runtime.
type StoragePaths struct {
	ConfigDir       string `json:"config_dir"`
	CredentialsPath string `json:"credentials_path"`
	TempDir         string `json:"temp_dir"`
	SocketPath      string `json:"socket_path"`
}

// AppZoneDetail describes path and account overrides for one application type.
type AppZoneDetail struct {
	AppType          string            `json:"app_type"` // "desktop", "agy", "vscode"
	DisplayName      string            `json:"display_name"`
	DetectedPath     string            `json:"detected_path"`
	CustomPath       string            `json:"custom_path"`
	ActivePath       string            `json:"active_path"`
	Installed        bool              `json:"installed"`
	Version          string            `json:"version"`
	AccountOverrides map[string]string `json:"account_overrides"`
}

// AppZonesInfo aggregates the 3 app zones.
type AppZonesInfo struct {
	Desktop AppZoneDetail `json:"desktop"`
	Agy     AppZoneDetail `json:"agy"`
	VSCode  AppZoneDetail `json:"vscode"`
}

// ClearCacheResult reports the outcome of a clear cache action for an application.
type ClearCacheResult struct {
	Success      bool   `json:"success"`
	AppType      string `json:"app_type"`
	FreedBytes   int64  `json:"freed_bytes"`
	DeletedFiles int    `json:"deleted_files"`
	Message      string `json:"message"`
}

// StorageInfo represents system storage configuration and detected runtime paths.
type StorageInfo struct {
	StorageMode        string       `json:"storage_mode"` // "system_default" or "app_portable"
	CurrentPaths       StoragePaths `json:"current_paths"`
	SystemDefaultPaths StoragePaths `json:"system_default_paths"`
	AppPortablePaths   StoragePaths `json:"app_portable_paths"`
	AppExecutionType   string       `json:"app_execution_type"` // "unzipped_folder", "standalone_binary", "system_package"
	AppExecutionDetail string       `json:"app_execution_detail"`
	CanMigrate         bool         `json:"can_migrate"`
	AppZones           AppZonesInfo `json:"app_zones"`
}

// DetectAppExecutionType inspects how the app is packaged and executed.
func DetectAppExecutionType() (string, string) {
	if os.Getenv("APPIMAGE") != "" {
		return "standalone_binary", "Running as standalone packaged AppImage binary."
	}

	exePath, err := os.Executable()
	if err != nil {
		return "unzipped_folder", "Portable directory runner."
	}

	exeDir := filepath.Dir(exePath)
	cleanPath := filepath.Clean(exePath)

	// Check if in system directory
	if strings.HasPrefix(cleanPath, "/usr/") || strings.HasPrefix(cleanPath, "/opt/") ||
		strings.Contains(cleanPath, "Program Files") {
		return "system_package", fmt.Sprintf("Installed system package (%s).", cleanPath)
	}

	// Check for unzipped folder markers: package.json, scripts/, electron/, bin/
	markers := []string{"package.json", "scripts", "electron", "bin", "assets"}
	hasMarker := false
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(exeDir, m)); err == nil {
			hasMarker = true
			break
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(exeDir), m)); err == nil {
			hasMarker = true
			break
		}
	}

	if hasMarker {
		return "unzipped_folder", fmt.Sprintf("Unzipped portable directory with runner scripts and binaries (%s).", exeDir)
	}

	return "standalone_binary", fmt.Sprintf("Standalone single application binary (%s).", cleanPath)
}

// ResolveAppDir returns the application directory root.
func ResolveAppDir() string {
	if custom := os.Getenv("ANTIGRAVITY_SWISS_APP_DIR"); custom != "" {
		return custom
	}

	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		// If binary is in bin/ subdirectory, root is parent
		if filepath.Base(exeDir) == "bin" {
			return filepath.Dir(exeDir)
		}
		return exeDir
	}

	cwd, err := os.Getwd()
	if err == nil {
		return cwd
	}
	return "."
}

// ResolvePortableDataDir returns the path to the portable data directory.
func ResolvePortableDataDir() string {
	if custom := os.Getenv("ANTIGRAVITY_SWISS_PORTABLE_DIR"); custom != "" {
		return custom
	}
	appDir := ResolveAppDir()
	return filepath.Join(appDir, "data")
}

// GetStoragePaths computes storage paths for the given base directory.
func GetStoragePaths(configDir string, isPortable bool) StoragePaths {
	credsPath := filepath.Join(configDir, "accounts.json")
	var tempDir string
	var socketPath string

	if isPortable {
		tempDir = filepath.Join(configDir, "temp")
		socketPath = filepath.Join(configDir, "daemon.sock")
	} else {
		tempDir = core.GetRuntimeDir()
		socketPath = core.GetSocketPath()
	}

	return StoragePaths{
		ConfigDir:       configDir,
		CredentialsPath: credsPath,
		TempDir:         tempDir,
		SocketPath:      socketPath,
	}
}

// GetStorageInfo returns complete storage metadata, paths, and execution type.
func GetStorageInfo(cfg *core.Config) *StorageInfo {
	execType, execDetail := DetectAppExecutionType()

	sysPaths := GetStoragePaths(core.GetConfigDir(), false)
	portablePaths := GetStoragePaths(ResolvePortableDataDir(), true)

	mode := cfg.StorageMode
	if mode != "app_portable" {
		mode = "system_default"
	}

	curPaths := sysPaths
	if mode == "app_portable" {
		curPaths = portablePaths
	}

	// Check if source config or accounts file exists for migration
	canMigrate := false
	if mode == "system_default" {
		if pathExists(sysPaths.CredentialsPath) || pathExists(filepath.Join(sysPaths.ConfigDir, "config.json")) {
			canMigrate = true
		}
	} else {
		if pathExists(portablePaths.CredentialsPath) || pathExists(filepath.Join(portablePaths.ConfigDir, "config.json")) {
			canMigrate = true
		}
	}

	// Assemble 3 application zones: Desktop, agy CLI, and VS Code Extension
	detector := NewDetector()
	inst := detector.DetectAll()

	desktopCustom := cfg.DesktopAppPath
	desktopActive := inst.DesktopApp.Path
	if desktopCustom != "" {
		desktopActive = desktopCustom
	}
	desktopOverrides := make(map[string]string)
	if cfg.AppAccountOverrides != nil && cfg.AppAccountOverrides["desktop"] != nil {
		for k, v := range cfg.AppAccountOverrides["desktop"] {
			desktopOverrides[k] = v
		}
	}
	desktopZone := AppZoneDetail{
		AppType:          "desktop",
		DisplayName:      "Antigravity 2.0 Desktop App",
		DetectedPath:     inst.DesktopApp.Path,
		CustomPath:       desktopCustom,
		ActivePath:       desktopActive,
		Installed:        inst.DesktopApp.Installed,
		Version:          inst.DesktopApp.Version,
		AccountOverrides: desktopOverrides,
	}

	agyCustom := cfg.AgyCLIPath
	agyActive := inst.AgyCLI.Path
	if agyCustom != "" {
		agyActive = agyCustom
	}
	agyOverrides := make(map[string]string)
	if cfg.AppAccountOverrides != nil && cfg.AppAccountOverrides["agy"] != nil {
		for k, v := range cfg.AppAccountOverrides["agy"] {
			agyOverrides[k] = v
		}
	}
	agyZone := AppZoneDetail{
		AppType:          "agy",
		DisplayName:      "agy CLI",
		DetectedPath:     inst.AgyCLI.Path,
		CustomPath:       agyCustom,
		ActivePath:       agyActive,
		Installed:        inst.AgyCLI.Installed,
		Version:          inst.AgyCLI.Version,
		AccountOverrides: agyOverrides,
	}

	vscodeCustom := cfg.VSCodeExtensionPath
	vscodeActive := inst.VSCodeExtension.Path
	if vscodeCustom != "" {
		vscodeActive = vscodeCustom
	}
	vscodeOverrides := make(map[string]string)
	if cfg.AppAccountOverrides != nil && cfg.AppAccountOverrides["vscode"] != nil {
		for k, v := range cfg.AppAccountOverrides["vscode"] {
			vscodeOverrides[k] = v
		}
	}
	vscodeZone := AppZoneDetail{
		AppType:          "vscode",
		DisplayName:      "VS Code Extension",
		DetectedPath:     inst.VSCodeExtension.Path,
		CustomPath:       vscodeCustom,
		ActivePath:       vscodeActive,
		Installed:        inst.VSCodeExtension.Installed,
		Version:          inst.VSCodeExtension.Version,
		AccountOverrides: vscodeOverrides,
	}

	return &StorageInfo{
		StorageMode:        mode,
		CurrentPaths:       curPaths,
		SystemDefaultPaths: sysPaths,
		AppPortablePaths:   portablePaths,
		AppExecutionType:   execType,
		AppExecutionDetail: execDetail,
		CanMigrate:         canMigrate,
		AppZones: AppZonesInfo{
			Desktop: desktopZone,
			Agy:     agyZone,
			VSCode:  vscodeZone,
		},
	}
}

// SaveAppPath updates custom configured path for desktop, agy, or vscode.
func SaveAppPath(appType, path string, cfg *core.Config) (*StorageInfo, error) {
	if err := cfg.SetAppPath(appType, path); err != nil {
		return nil, err
	}
	return GetStorageInfo(cfg), nil
}

// SaveAccountOverride sets or clears executable override for an account.
func SaveAccountOverride(appType, email, path string, cfg *core.Config) (*StorageInfo, error) {
	if err := cfg.SetAccountOverride(appType, email, path); err != nil {
		return nil, err
	}
	return GetStorageInfo(cfg), nil
}

// ClearAppCache safely clears temporary and cache data for a specific application.
func ClearAppCache(appType string) (*ClearCacheResult, error) {
	var targetDirs []string
	var appName string

	switch appType {
	case "desktop":
		appName = "Antigravity 2.0 Desktop"
		brain := core.GetBrainDir()
		hostConfig := core.GetAntigravityHostConfigDir()
		targetDirs = []string{
			filepath.Join(brain, "scratch"),
			filepath.Join(brain, "steps"),
			filepath.Join(hostConfig, "Cache"),
			filepath.Join(hostConfig, "Code Cache"),
			filepath.Join(hostConfig, "GPUCache"),
		}
	case "agy":
		appName = "agy CLI"
		home, _ := os.UserHomeDir()
		targetDirs = []string{
			filepath.Join(home, ".cache", "agy"),
			filepath.Join(home, ".agy", "cache"),
			filepath.Join(os.TempDir(), "antigravity-swiss"),
		}
	case "vscode":
		appName = "VS Code Extension"
		home, _ := os.UserHomeDir()
		targetDirs = []string{
			filepath.Join(core.GetAntigravityDir(), "vscode_cache"),
			filepath.Join(home, ".config", "Code", "User", "globalStorage", "google.antigravity"),
		}
	default:
		return nil, fmt.Errorf("unknown application type %q: must be 'desktop', 'agy', or 'vscode'", appType)
	}

	var freedBytes int64
	var deletedFiles int

	for _, dir := range targetDirs {
		if !pathExists(dir) {
			continue
		}
		_ = filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				return nil
			}
			size := fi.Size()
			if removeErr := os.Remove(p); removeErr == nil {
				freedBytes += size
				deletedFiles++
			}
			return nil
		})
	}

	return &ClearCacheResult{
		Success:      true,
		AppType:      appType,
		FreedBytes:   freedBytes,
		DeletedFiles: deletedFiles,
		Message:      fmt.Sprintf("Cleared %d cache files (%s) for %s.", deletedFiles, formatBytes(freedBytes), appName),
	}, nil
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// SwitchStorageMode updates storage mode and optionally migrates existing credentials and configuration.
func SwitchStorageMode(newMode string, migrateData bool, cfg *core.Config) (*StorageInfo, error) {
	if newMode != "system_default" && newMode != "app_portable" {
		return nil, fmt.Errorf("invalid storage mode %q: must be 'system_default' or 'app_portable'", newMode)
	}

	info := GetStorageInfo(cfg)
	var srcDir, dstDir string

	if newMode == "app_portable" {
		srcDir = info.SystemDefaultPaths.ConfigDir
		dstDir = info.AppPortablePaths.ConfigDir
	} else {
		srcDir = info.AppPortablePaths.ConfigDir
		dstDir = info.SystemDefaultPaths.ConfigDir
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(dstDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create destination storage directory: %w", err)
	}

	if newMode == "app_portable" {
		// Ensure portable temp directory exists
		_ = os.MkdirAll(info.AppPortablePaths.TempDir, 0700)
	}

	if migrateData && srcDir != "" && dstDir != "" && srcDir != dstDir {
		filesToCopy := []string{"accounts.json", "config.json", "rules.json", "gui_config.json"}
		for _, file := range filesToCopy {
			srcFile := filepath.Join(srcDir, file)
			if pathExists(srcFile) {
				dstFile := filepath.Join(dstDir, file)
				if err := copyFile(srcFile, dstFile); err != nil {
					return nil, fmt.Errorf("failed migrating %s: %w", file, err)
				}
			}
		}
	}

	// Update and persist config with new storage mode
	cfg.StorageMode = newMode
	_ = cfg.Save()

	return GetStorageInfo(cfg), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// Ensure parent directory
	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
