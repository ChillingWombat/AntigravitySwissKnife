package core

import (
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	AppName    = "Antigravity Swiss Knife"
	AppVersion = "0.1.0"
	AppID      = "com.antigravity.swiss-knife"

	// Default timeouts and intervals
	DefaultPollingIntervalSeconds            = 60
	DefaultAutoSwitchThresholdFraction       = 0.05
	DefaultAutoSwitchWeeklyThresholdFraction = 0.05
	DefaultWarmupLeadTimeSeconds             = 2.0
	DefaultPostResetDelaySeconds             = 2.0
	DefaultSocketTimeout                     = 10 * time.Second
	DefaultShutdownGracePeriod               = 3 * time.Second

	// Account Switcher Modes
	SwitchModeBalanced           = "balanced"
	SwitchModeMaxTokens          = "max_tokens"
	SwitchModeMaxContinuous      = "max_continuous"
	DefaultSwitchMode            = SwitchModeBalanced
	DefaultMinSwitchDwellSeconds = 600

	// Quota Refresh Frequency Modes
	QuotaRefreshModeDynamic = "dynamic"
	QuotaRefreshModeManual  = "manual"
	DefaultQuotaRefreshMode = QuotaRefreshModeDynamic

	// Multi-App Account Synchronization Modes
	MultiAppSyncModeShared     = "shared"
	MultiAppSyncModeIndividual = "individual"
	DefaultMultiAppSyncMode    = MultiAppSyncModeShared

	// Gemini Subagent Custom Model Strategies
	SubagentModelStrategyDefaultCustomOnly = "default_custom_only"
	SubagentModelStrategyAutoDecide        = "auto_decide"
	DefaultSubagentModelStrategy           = SubagentModelStrategyDefaultCustomOnly

	// Target Apps for Account Switching
	TargetAppAll     = "all"
	TargetAppDesktop = "desktop"
	TargetAppCLI     = "agy"
	TargetAppVSCode  = "vscode"

	// Quota Health Statuses
	StatusHealthy   = "HEALTHY"
	StatusWarning   = "WARNING"
	StatusExhausted = "EXHAUSTED"

	// Account Statuses
	AccountStatusActive   = "ACTIVE"
	AccountStatusStandby  = "STANDBY"
	AccountStatusCooldown = "COOLDOWN"
	AccountStatusCooling  = "COOLING"
	AccountStatusError    = "ERROR"
	AccountStatusBanned   = "BANNED"

	// Minimalist Light Theme Design Tokens
	LightSurface              = "#f0f4f9"
	LightSurfaceContainer     = "#ffffff"
	LightSurfaceContainerHigh = "#e9eef6"
	LightOutline              = "#e5e9f0"
	LightTextPrimary          = "#1f1f1f"
	LightTextSecondary        = "#444746"
	LightAccentPrimary        = "#0b57d0"
	LightColorHealthy         = "#137333"
	LightColorWarning         = "#b06000"
	LightColorExhausted       = "#b3261e"
)

// GetConfigDir returns platform-specific config directory:
// Linux: ~/.config/antigravity-swiss or $XDG_CONFIG_HOME/antigravity-swiss
// Windows: %APPDATA%\antigravity-swiss
// macOS: ~/Library/Application Support/antigravity-swiss
func GetConfigDir() string {
	if dir := os.Getenv("ANTIGRAVITY_SWISS_CONFIG_DIR"); dir != "" {
		return dir
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "antigravity-swiss")
		}
	} else if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, "Library", "Application Support", "antigravity-swiss")
		}
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "antigravity-swiss")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	return filepath.Join(home, ".config", "antigravity-swiss")
}

// GetRuntimeDir returns $XDG_RUNTIME_DIR/antigravity-swiss or fallback.
func GetRuntimeDir() string {
	if dir := os.Getenv("ANTIGRAVITY_SWISS_RUNTIME_DIR"); dir != "" {
		return dir
	}
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		return filepath.Join(xdg, "antigravity-swiss")
	}
	return filepath.Join(os.TempDir(), "antigravity-swiss")
}

// GetSocketPath returns the Unix domain socket path.
func GetSocketPath() string {
	if p := os.Getenv("ANTIGRAVITY_SWISS_SOCKET"); p != "" {
		return p
	}
	return filepath.Join(GetRuntimeDir(), "daemon.sock")
}

// GetAntigravityDir returns ~/.gemini/antigravity or configured override.
func GetAntigravityDir() string {
	if dir := os.Getenv("ANTIGRAVITY_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	return filepath.Join(home, ".gemini", "antigravity")
}

// GetBrainDir returns ~/.gemini/antigravity/brain.
func GetBrainDir() string {
	return filepath.Join(GetAntigravityDir(), "brain")
}

// GetConversationsDir returns ~/.gemini/antigravity/conversations.
func GetConversationsDir() string {
	return filepath.Join(GetAntigravityDir(), "conversations")
}

// GetConversationVaultDir returns ~/.gemini/antigravity/vault/conversations.
func GetConversationVaultDir() string {
	return filepath.Join(GetAntigravityDir(), "vault", "conversations")
}

// GetAnnotationsVaultDir returns ~/.gemini/antigravity/vault/annotations.
func GetAnnotationsVaultDir() string {
	return filepath.Join(GetAntigravityDir(), "vault", "annotations")
}

// GetAntigravityHostConfigDir returns host Antigravity settings directory per OS:
// Linux: ~/.config/Antigravity
// Windows: %APPDATA%\Antigravity
// macOS: ~/Library/Application Support/Antigravity
func GetAntigravityHostConfigDir() string {
	if dir := os.Getenv("ANTIGRAVITY_HOST_CONFIG_DIR"); dir != "" {
		return dir
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "Antigravity")
		}
	} else if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, "Library", "Application Support", "Antigravity")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	return filepath.Join(home, ".config", "Antigravity")
}

// GetVSCodeHostConfigDir returns host VS Code settings directory per OS:
// Linux: ~/.config/Code
// Windows: %APPDATA%\Code
// macOS: ~/Library/Application Support/Code
func GetVSCodeHostConfigDir() string {
	if dir := os.Getenv("VSCODE_HOST_CONFIG_DIR"); dir != "" {
		return dir
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "Code")
		}
	} else if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, "Library", "Application Support", "Code")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/tmp"
	}
	return filepath.Join(home, ".config", "Code")
}

// GetAntigravityDesktopResourcesDir returns desktop resources directory per OS:
// Linux: /opt/Antigravity/resources
// Windows: %LOCALAPPDATA%\Programs\Antigravity\resources
// macOS: /Applications/Antigravity.app/Contents/Resources
func GetAntigravityDesktopResourcesDir() string {
	if dir := os.Getenv("ANTIGRAVITY_RESOURCES_DIR"); dir != "" {
		return dir
	}
	if runtime.GOOS == "windows" {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "Programs", "Antigravity", "resources")
		}
	} else if runtime.GOOS == "darwin" {
		return "/Applications/Antigravity.app/Contents/Resources"
	}
	return "/opt/Antigravity/resources"
}

// GetAntigravityDesktopAppPath returns desktop app executable or bundle root per OS:
// Linux: /opt/Antigravity
// Windows: %LOCALAPPDATA%\Programs\Antigravity
// macOS: /Applications/Antigravity.app
func GetAntigravityDesktopAppPath() string {
	if path := os.Getenv("ANTIGRAVITY_APP_PATH"); path != "" {
		return path
	}
	if runtime.GOOS == "windows" {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "Programs", "Antigravity")
		}
	} else if runtime.GOOS == "darwin" {
		return "/Applications/Antigravity.app"
	}
	return "/opt/Antigravity"
}

// GetVSCodeExtensionsDirs returns possible directories where VS Code extensions are installed.
func GetVSCodeExtensionsDirs() []string {
	if custom := os.Getenv("VSCODE_EXTENSIONS_DIR"); custom != "" {
		return []string{custom}
	}
	var dirs []string

	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		dirs = append(dirs,
			filepath.Join(home, ".vscode", "extensions"),
			filepath.Join(home, ".vscode-insiders", "extensions"),
			filepath.Join(home, ".cursor", "extensions"),
		)
		if runtime.GOOS == "windows" {
			if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
				dirs = append(dirs, filepath.Join(userProfile, ".vscode", "extensions"))
			}
		}
	}
	return dirs
}

// GetAntigravityBinaryPath returns the path to the Antigravity executable per OS.
func GetAntigravityBinaryPath() string {
	if p := os.Getenv("ANTIGRAVITY_BIN_PATH"); p != "" {
		return p
	}
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(GetAntigravityDesktopAppPath(), "Antigravity.exe")
	case "darwin":
		return filepath.Join(GetAntigravityDesktopAppPath(), "Contents", "MacOS", "Antigravity")
	default:
		// Linux: prefer /opt/Antigravity/antigravity, fallback to /usr/bin/antigravity
		if _, err := os.Stat("/opt/Antigravity/antigravity"); err == nil {
			return "/opt/Antigravity/antigravity"
		}
		if _, err := os.Stat("/usr/bin/antigravity"); err == nil {
			return "/usr/bin/antigravity"
		}
		return filepath.Join(GetAntigravityDesktopAppPath(), "antigravity")
	}
}

// IsRunningTests returns true if the current execution is within a Go test runner.
func IsRunningTests() bool {
	if os.Getenv("ANTIGRAVITY_TEST_MODE") == "1" || os.Getenv("ANTIGRAVITY_TESTING") == "1" || os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") == "1" {
		return true
	}
	if flag.Lookup("test.v") != nil {
		return true
	}
	if len(os.Args) > 0 && (strings.HasSuffix(os.Args[0], ".test") || strings.Contains(os.Args[0], "/_test/")) {
		return true
	}
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.") {
			return true
		}
	}
	return false
}

// IsTestMockEmail filters out unit test fixtures and artificial mock accounts.
func IsTestMockEmail(email string) bool {
	norm := strings.ToLower(strings.TrimSpace(email))
	if norm == "" {
		return true
	}
	if norm == "target@gmail.com" || strings.HasSuffix(norm, "@example.com") || strings.HasSuffix(norm, ".test") || strings.HasSuffix(norm, "@mock.test") {
		return true
	}
	if strings.HasPrefix(norm, "mock_") || strings.HasPrefix(norm, "test_") || strings.HasPrefix(norm, "test.") {
		return true
	}
	return false
}
