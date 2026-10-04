package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config represents the application configuration.
type Config struct {
	AutoSwitchEnabled    bool    `json:"auto_switch_enabled"`
	AutoSwitchThreshold  float64 `json:"auto_switch_threshold"`
	PollingIntervalSec   int     `json:"polling_interval_seconds"`
	WarmupEnabled        bool    `json:"warmup_enabled"`
	WarmupLeadTimeSec    float64 `json:"warmup_lead_time_seconds"`
	ActiveAccount        string  `json:"active_account"`
	AntigravityProtectedPID int  `json:"antigravity_protected_pid"`

	mu sync.RWMutex `json:"-"`
}

// DefaultConfig returns default configuration parameters.
func DefaultConfig() *Config {
	return &Config{
		AutoSwitchEnabled:   true,
		AutoSwitchThreshold: DefaultAutoSwitchThresholdFraction,
		PollingIntervalSec:  DefaultPollingIntervalSeconds,
		WarmupEnabled:       true,
		WarmupLeadTimeSec:   DefaultWarmupLeadTimeSeconds,
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
