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
	DefaultGeminiModel         string   `json:"default_gemini_model,omitempty"`
	DefaultCustomModel         string   `json:"default_custom_model,omitempty"`
	DefaultNonGeminiModel      string   `json:"default_non_gemini_model,omitempty"`
	ActiveAccount              string   `json:"active_account"`
	AntigravityProtectedPID    int      `json:"antigravity_protected_pid"`
	AutoImportActiveAccount    bool     `json:"auto_import_active_account"`
	AppPasswordEnabled         bool     `json:"app_password_enabled"`
	AppPasswordHash            string   `json:"app_password_hash,omitempty"`

	mu sync.RWMutex `json:"-"`
}

// DefaultConfig returns default configuration parameters.
func DefaultConfig() *Config {
	return &Config{
		AutoSwitchEnabled:          true,
		AutoSwitchThreshold:        DefaultAutoSwitchThresholdFraction,
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
		DefaultGeminiModel:         "gemini-2.5-pro",
		DefaultCustomModel:         "",
		DefaultNonGeminiModel:      "claude-3-7-sonnet",
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

