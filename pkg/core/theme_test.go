package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeThemeMode(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"auto", ThemeModeAuto},
		{"AUTO", ThemeModeAuto},
		{"light", ThemeModeLight},
		{"LIGHT", ThemeModeLight},
		{"dark", ThemeModeDark},
		{"Dark", ThemeModeDark},
		{"", ThemeModeAuto},
		{"unknown", ThemeModeAuto},
		{"  dark  ", ThemeModeDark},
	}
	for _, tc := range cases {
		got := NormalizeThemeMode(tc.input)
		if got != tc.expected {
			t.Errorf("NormalizeThemeMode(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestDefaultConfig_Theme(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Theme != ThemeModeAuto {
		t.Fatalf("expected default Theme %q, got %q", ThemeModeAuto, cfg.Theme)
	}
	if cfg.GetTheme() != ThemeModeAuto {
		t.Fatalf("expected GetTheme() %q, got %q", ThemeModeAuto, cfg.GetTheme())
	}
}

func TestLoadConfig_ThemePersistence(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)

	// Config without a theme field falls back to "auto".
	raw := map[string]interface{}{}
	data, _ := json.Marshal(raw)
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), data, 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg.GetTheme() != ThemeModeAuto {
		t.Errorf("expected fallback theme %q, got %q", ThemeModeAuto, cfg.GetTheme())
	}

	// Invalid stored theme normalizes to "auto".
	rawInvalid := map[string]interface{}{"theme": "neon"}
	dataInvalid, _ := json.Marshal(rawInvalid)
	if err := os.WriteFile(filepath.Join(tempDir, "config.json"), dataInvalid, 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
	cfg2, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg2.GetTheme() != ThemeModeAuto {
		t.Errorf("expected invalid theme to normalize to %q, got %q", ThemeModeAuto, cfg2.GetTheme())
	}

	// SetTheme persists and reloads.
	if err := cfg2.SetTheme("dark"); err != nil {
		t.Fatalf("SetTheme failed: %v", err)
	}
	cfg3, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg3.GetTheme() != ThemeModeDark {
		t.Errorf("expected persisted theme %q, got %q", ThemeModeDark, cfg3.GetTheme())
	}
}

func TestSetTheme_NilConfig(t *testing.T) {
	var cfg *Config
	if got := cfg.GetTheme(); got != ThemeModeAuto {
		t.Errorf("nil config GetTheme() = %q, expected %q", got, ThemeModeAuto)
	}
}
