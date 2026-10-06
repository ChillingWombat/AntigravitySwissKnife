package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Config represents the application configuration.
type Config struct {
	AutoSwitchEnabled          bool     `json:"auto_switch_enabled"`
	AutoSwitchThreshold        float64  `json:"auto_switch_threshold"`
	AutoSwitchWeeklyThreshold  float64  `json:"auto_switch_weekly_threshold"`
	SwitchMode                 string   `json:"switch_mode,omitempty"`
	PollingIntervalSec         int      `json:"polling_interval_seconds"`
	ActivePollingIntervalSec   int      `json:"active_polling_interval_seconds"`
	StandbyPollingIntervalSec  int      `json:"standby_polling_interval_seconds"`
	StandbyRandomJitterSec     int      `json:"standby_random_jitter_seconds"`
	WarmupEnabled              bool     `json:"warmup_enabled"`
	WarmupLeadTimeSec          float64  `json:"warmup_lead_time_seconds"`
	PreferredNativeModel       string   `json:"preferred_native_model,omitempty"`
	AllowAICreditsUsage        bool     `json:"allow_ai_credits_usage"`
	AllowNonGeminiNativeModels bool     `json:"allow_non_gemini_native_models"`
	ModelSourceHierarchy       []string `json:"model_source_hierarchy,omitempty"`
	DefaultGeminiModel          string   `json:"default_gemini_model,omitempty"`
	DefaultCustomModel          string   `json:"default_custom_model,omitempty"`
	DefaultNonGeminiModel       string   `json:"default_non_gemini_model,omitempty"`
	DefaultGeminiReasoningLevel string   `json:"default_gemini_reasoning_level,omitempty"`
	ActiveAccount              string   `json:"active_account"`
	AntigravityProtectedPID    int      `json:"antigravity_protected_pid"`
	AutoImportActiveAccount    bool     `json:"auto_import_active_account"`
	AppPasswordEnabled         bool     `json:"app_password_enabled"`
	AppPasswordHash            string   `json:"app_password_hash,omitempty"`
	StorageMode                string   `json:"storage_mode,omitempty"`
	AnonymousErrorReports      bool                         `json:"anonymous_error_reports"`
	AnonymousTelemetry         bool                         `json:"anonymous_telemetry"`
	DesktopAppPath             string                       `json:"desktop_app_path,omitempty"`
	AgyCLIPath                 string                       `json:"agy_cli_path,omitempty"`
	VSCodeExtensionPath        string                       `json:"vscode_extension_path,omitempty"`
	AppAccountOverrides        map[string]map[string]string `json:"app_account_overrides,omitempty"`
	Memo                       MemoConfig                   `json:"memo"`
	PreferredIDE               string                       `json:"preferred_ide,omitempty"`

	mu sync.RWMutex `json:"-"`
}

// MemoConfig represents settings for the Quick Memos extension.
type MemoConfig struct {
	StorageLocation string `json:"storage_location"` // "global" | "project"
	ViewScope       string `json:"view_scope"`       // "all" | "current"
	SearchScope     string `json:"search_scope"`     // "text" | "all"
}

// DefaultConfig returns default configuration parameters.
func DefaultConfig() *Config {
	return &Config{
		AutoSwitchEnabled:          true,
		AutoSwitchThreshold:        DefaultAutoSwitchThresholdFraction,
		AutoSwitchWeeklyThreshold:  DefaultAutoSwitchWeeklyThresholdFraction,
		SwitchMode:                 DefaultSwitchMode,
		AutoImportActiveAccount:    false,
		PollingIntervalSec:         DefaultPollingIntervalSeconds,
		ActivePollingIntervalSec:   120, // 2 minutes
		StandbyPollingIntervalSec:  900, // 15 minutes
		StandbyRandomJitterSec:     30,  // up to 30s jitter gap
		WarmupEnabled:              true,
		WarmupLeadTimeSec:          DefaultWarmupLeadTimeSeconds,
		PreferredNativeModel:       "gemini",
		AllowAICreditsUsage:        false,
		AllowNonGeminiNativeModels: false,
		ModelSourceHierarchy:       []string{"gemini", "custom", "non_gemini", "credits"},
		DefaultGeminiModel:         "gemini-3.8-flash-high",
		DefaultCustomModel:         "",
		DefaultNonGeminiModel:      "claude-opus-4-6",
		DefaultGeminiReasoningLevel: "high",
		StorageMode:                "system_default",
		AnonymousErrorReports:      true,
		AnonymousTelemetry:         false,
		Memo: MemoConfig{
			StorageLocation: "global",
			ViewScope:       "all",
			SearchScope:     "text",
		},
		PreferredIDE: "code",
	}
}

// LoadConfig loads config from disk or returns default config.
func LoadConfig() (*Config, error) {
	cfgPath := filepath.Join(GetConfigDir(), "config.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			_ = cfg.Save()
			return cfg, nil
		}
		return DefaultConfig(), err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return DefaultConfig(), nil // fallback safely
	}
	if cfg.AutoSwitchThreshold <= 0 {
		cfg.AutoSwitchThreshold = DefaultAutoSwitchThresholdFraction
	}
	if cfg.AutoSwitchWeeklyThreshold <= 0 {
		cfg.AutoSwitchWeeklyThreshold = DefaultAutoSwitchWeeklyThresholdFraction
	}
	if cfg.Memo.StorageLocation == "" {
		cfg.Memo.StorageLocation = "global"
	}
	if cfg.Memo.ViewScope == "" {
		cfg.Memo.ViewScope = "all"
	}
	if cfg.Memo.SearchScope == "" {
		cfg.Memo.SearchScope = "text"
	}
	return cfg, nil
}

// Save persists config to disk atomically.
func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	dir := GetConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	target := filepath.Join(dir, "config.json")
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}

// SetAppPassword updates or removes the application access password.
// Passing an empty string disables the app password.
func (c *Config) SetAppPassword(password string) error {
	if password == "" {
		c.AppPasswordEnabled = false
		c.AppPasswordHash = ""
		return c.Save()
	}
	valid, msg := ValidateAppPassword(password)
	if !valid {
		return fmt.Errorf("%s", msg)
	}
	c.AppPasswordHash = HashAppPassword(password)
	c.AppPasswordEnabled = true
	return c.Save()
}

// VerifyAppPassword verifies the candidate password against the configured hash.
func (c *Config) VerifyAppPassword(password string) bool {
	if !c.AppPasswordEnabled || c.AppPasswordHash == "" {
		return true
	}
	return VerifyAppPassword(password, c.AppPasswordHash)
}

// GetAppPath returns custom configured path for the specified app type.
func (c *Config) GetAppPath(appType string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	switch appType {
	case "desktop":
		return c.DesktopAppPath
	case "agy":
		return c.AgyCLIPath
	case "vscode":
		return c.VSCodeExtensionPath
	default:
		return ""
	}
}

// SetAppPath updates the custom configured path for the specified app type.
func (c *Config) SetAppPath(appType, path string) error {
	c.mu.Lock()
	switch appType {
	case "desktop":
		c.DesktopAppPath = path
	case "agy":
		c.AgyCLIPath = path
	case "vscode":
		c.VSCodeExtensionPath = path
	default:
		c.mu.Unlock()
		return fmt.Errorf("unknown app type %q: must be 'desktop', 'agy', or 'vscode'", appType)
	}
	c.mu.Unlock()
	return c.Save()
}

// SetAccountOverride sets or clears a custom executable override for a specific account.
func (c *Config) SetAccountOverride(appType, email, path string) error {
	c.mu.Lock()
	if c.AppAccountOverrides == nil {
		c.AppAccountOverrides = make(map[string]map[string]string)
	}
	if c.AppAccountOverrides[appType] == nil {
		c.AppAccountOverrides[appType] = make(map[string]string)
	}
	if path == "" {
		delete(c.AppAccountOverrides[appType], email)
	} else {
		c.AppAccountOverrides[appType][email] = path
	}
	c.mu.Unlock()
	return c.Save()
}

// GetAccountOverride retrieves the custom executable path override for an account.
func (c *Config) GetAccountOverride(appType, email string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.AppAccountOverrides == nil || c.AppAccountOverrides[appType] == nil {
		return ""
	}
	return c.AppAccountOverrides[appType][email]
}

// GetMemoConfig returns thread-safe copy of memo configuration.
func (c *Config) GetMemoConfig() MemoConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	res := c.Memo
	if res.StorageLocation == "" {
		res.StorageLocation = "global"
	}
	if res.ViewScope == "" {
		res.ViewScope = "all"
	}
	if res.SearchScope == "" {
		res.SearchScope = "text"
	}
	return res
}

// SetMemoConfig updates and persists memo configuration.
func (c *Config) SetMemoConfig(cfg MemoConfig) error {
	c.mu.Lock()
	if cfg.StorageLocation == "" {
		cfg.StorageLocation = "global"
	}
	if cfg.ViewScope == "" {
		cfg.ViewScope = "all"
	}
	if cfg.SearchScope == "" {
		cfg.SearchScope = "text"
	}
	c.Memo = cfg
	c.mu.Unlock()
	return c.Save()
}

// GetPreferredIDE returns the configured preferred IDE or default "code".
func (c *Config) GetPreferredIDE() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.PreferredIDE == "" {
		return "code"
	}
	return c.PreferredIDE
}

// SetPreferredIDE sets and persists the preferred IDE.
func (c *Config) SetPreferredIDE(ide string) error {
	c.mu.Lock()
	c.PreferredIDE = ide
	if c.PreferredIDE == "" {
		c.PreferredIDE = "code"
	}
	c.mu.Unlock()
	return c.Save()
}



