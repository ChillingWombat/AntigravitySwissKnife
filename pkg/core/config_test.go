package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig_CacheAutoPrunerUnlimited(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.AutoPruneMaxAgeDays != 0.0 {
		t.Fatalf("expected default AutoPruneMaxAgeDays 0.0 (Unlimited), got %f", cfg.AutoPruneMaxAgeDays)
	}
	if cfg.AutoPruneMaxSizeGB != 0.0 {
		t.Fatalf("expected default AutoPruneMaxSizeGB 0.0 (Unlimited), got %f", cfg.AutoPruneMaxSizeGB)
	}
}

func TestLoadConfig_PreservesZeroValues(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)

	raw := map[string]interface{}{
		"auto_prune_enabled":       true,
		"auto_prune_max_age_days": 0.0,
		"auto_prune_max_size_gb":  0.0,
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("failed to marshal test config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), data, 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.AutoPruneMaxAgeDays != 0.0 {
		t.Errorf("expected loaded AutoPruneMaxAgeDays 0.0, got %f", cfg.AutoPruneMaxAgeDays)
	}
	if cfg.AutoPruneMaxSizeGB != 0.0 {
		t.Errorf("expected loaded AutoPruneMaxSizeGB 0.0, got %f", cfg.AutoPruneMaxSizeGB)
	}
	if !cfg.AutoPruneEnabled {
		t.Errorf("expected AutoPruneEnabled to be true")
	}
}

func TestLoadConfig_ClampsNegativeValues(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)

	raw := map[string]interface{}{
		"auto_prune_max_age_days": -7.5,
		"auto_prune_max_size_gb":  -10.0,
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("failed to marshal test config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), data, 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.AutoPruneMaxAgeDays != 0.0 {
		t.Errorf("expected clamped AutoPruneMaxAgeDays 0.0, got %f", cfg.AutoPruneMaxAgeDays)
	}
	if cfg.AutoPruneMaxSizeGB != 0.0 {
		t.Errorf("expected clamped AutoPruneMaxSizeGB 0.0, got %f", cfg.AutoPruneMaxSizeGB)
	}
}

func TestDefaultConfig_SubagentModelStrategy(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.SubagentModelStrategy != SubagentModelStrategyDefaultCustomOnly {
		t.Fatalf("expected default SubagentModelStrategy %q, got %q", SubagentModelStrategyDefaultCustomOnly, cfg.SubagentModelStrategy)
	}
	if cfg.GetSubagentModelStrategy() != SubagentModelStrategyDefaultCustomOnly {
		t.Fatalf("expected GetSubagentModelStrategy() %q, got %q", SubagentModelStrategyDefaultCustomOnly, cfg.GetSubagentModelStrategy())
	}
}

func TestNormalizeSubagentModelStrategy(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"auto_decide", SubagentModelStrategyAutoDecide},
		{"auto", SubagentModelStrategyAutoDecide},
		{"autodecide", SubagentModelStrategyAutoDecide},
		{"AUTO_DECIDE", SubagentModelStrategyAutoDecide},
		{" Auto ", SubagentModelStrategyAutoDecide},
		{"default_custom_only", SubagentModelStrategyDefaultCustomOnly},
		{"", SubagentModelStrategyDefaultCustomOnly},
		{"   ", SubagentModelStrategyDefaultCustomOnly},
		{"invalid_strategy", SubagentModelStrategyDefaultCustomOnly},
		{"random", SubagentModelStrategyDefaultCustomOnly},
	}

	for _, tc := range cases {
		actual := NormalizeSubagentModelStrategy(tc.input)
		if actual != tc.expected {
			t.Errorf("NormalizeSubagentModelStrategy(%q) = %q, expected %q", tc.input, actual, tc.expected)
		}
	}
}

func TestLoadConfig_SubagentModelStrategy_PersistenceAndFallback(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)

	// Test persistence of valid auto_decide
	raw := map[string]interface{}{
		"subagent_model_strategy": "auto_decide",
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("failed to marshal test config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), data, 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg.GetSubagentModelStrategy() != SubagentModelStrategyAutoDecide {
		t.Errorf("expected GetSubagentModelStrategy() %q, got %q", SubagentModelStrategyAutoDecide, cfg.GetSubagentModelStrategy())
	}

	// Test fallback for invalid strategy
	rawInvalid := map[string]interface{}{
		"subagent_model_strategy": "unknown_strategy_xyz",
	}
	dataInvalid, _ := json.Marshal(rawInvalid)
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), dataInvalid, 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg2, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg2.GetSubagentModelStrategy() != SubagentModelStrategyDefaultCustomOnly {
		t.Errorf("expected GetSubagentModelStrategy() fallback to %q, got %q", SubagentModelStrategyDefaultCustomOnly, cfg2.GetSubagentModelStrategy())
	}

	// Test SetSubagentModelStrategy
	if err := cfg2.SetSubagentModelStrategy("auto"); err != nil {
		t.Fatalf("SetSubagentModelStrategy failed: %v", err)
	}
	if cfg2.GetSubagentModelStrategy() != SubagentModelStrategyAutoDecide {
		t.Errorf("expected GetSubagentModelStrategy() %q after set, got %q", SubagentModelStrategyAutoDecide, cfg2.GetSubagentModelStrategy())
	}
}

func TestDefaultConfig_QuotaRefreshMode(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.QuotaRefreshMode != QuotaRefreshModeDynamic {
		t.Fatalf("expected default QuotaRefreshMode %q, got %q", QuotaRefreshModeDynamic, cfg.QuotaRefreshMode)
	}
	if !cfg.DynamicQuotaRefreshEnabled {
		t.Fatalf("expected default DynamicQuotaRefreshEnabled to be true")
	}
	if !cfg.IsDynamicQuotaRefreshEnabled() {
		t.Fatalf("expected IsDynamicQuotaRefreshEnabled() to return true")
	}
	if cfg.GetQuotaRefreshMode() != QuotaRefreshModeDynamic {
		t.Fatalf("expected GetQuotaRefreshMode() %q, got %q", QuotaRefreshModeDynamic, cfg.GetQuotaRefreshMode())
	}
}

func TestNormalizeQuotaRefreshMode(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"dynamic", QuotaRefreshModeDynamic},
		{"DYNAMIC", QuotaRefreshModeDynamic},
		{"manual", QuotaRefreshModeManual},
		{"MANUAL", QuotaRefreshModeManual},
		{"", QuotaRefreshModeDynamic},
		{"unknown", QuotaRefreshModeDynamic},
	}
	for _, tc := range cases {
		got := NormalizeQuotaRefreshMode(tc.input)
		if got != tc.expected {
			t.Errorf("NormalizeQuotaRefreshMode(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestLoadConfig_QuotaRefreshModePersistence(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)

	rawManual := map[string]interface{}{
		"quota_refresh_mode": "manual",
	}
	data, err := json.Marshal(rawManual)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), data, 0600); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.GetQuotaRefreshMode() != QuotaRefreshModeManual {
		t.Errorf("expected %q, got %q", QuotaRefreshModeManual, cfg.GetQuotaRefreshMode())
	}
	if cfg.IsDynamicQuotaRefreshEnabled() {
		t.Errorf("expected IsDynamicQuotaRefreshEnabled to be false for manual mode")
	}

	if err := cfg.SetQuotaRefreshMode("dynamic"); err != nil {
		t.Fatalf("SetQuotaRefreshMode failed: %v", err)
	}
	if !cfg.IsDynamicQuotaRefreshEnabled() {
		t.Errorf("expected IsDynamicQuotaRefreshEnabled to be true after setting dynamic")
	}
}


