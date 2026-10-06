package core

import (
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const (
	AppName    = "Antigravity Swiss Knife"
	AppVersion = "2.0.0"
	AppID      = "com.antigravity.swiss-knife"

	// Default timeouts and intervals
	DefaultPollingIntervalSeconds             = 60
	DefaultAutoSwitchThresholdFraction        = 0.05
	DefaultAutoSwitchWeeklyThresholdFraction  = 0.05
	DefaultWarmupLeadTimeSeconds              = 2.0
	DefaultSocketTimeout                = 5 * time.Second
	DefaultShutdownGracePeriod          = 3 * time.Second

	// Account Switcher Modes
	SwitchModeBalanced           = "balanced"
	SwitchModeMaxTokens          = "max_tokens"
	SwitchModeMaxContinuous      = "max_continuous"
	DefaultSwitchMode            = SwitchModeBalanced
	DefaultMinSwitchDwellSeconds = 600


	// Quota Health Statuses
	StatusHealthy   = "HEALTHY"
	StatusWarning   = "WARNING"
	StatusExhausted = "EXHAUSTED"

	// Account Statuses
	AccountStatusActive   = "ACTIVE"
	AccountStatusStandby  = "STANDBY"
	AccountStatusCooldown = "COOLDOWN"
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

