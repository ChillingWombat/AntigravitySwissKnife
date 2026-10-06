package enhancements

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Errorf("expected Enabled to be true by default")
	}
	if !cfg.PromptJumpBar.Enabled {
		t.Errorf("expected PromptJumpBar.Enabled to be true by default")
	}
	if !cfg.PromptJumpBar.ShowTooltip {
		t.Errorf("expected ShowTooltip to be true")
	}
	if !cfg.PromptJumpBar.FocusPulse {
		t.Errorf("expected FocusPulse to be true")
	}
	if !cfg.PromptJumpBar.SyncScroll {
		t.Errorf("expected SyncScroll to be true")
	}
	if cfg.PromptJumpBar.Position != "gutter" {
		t.Errorf("expected Position to be gutter, got %s", cfg.PromptJumpBar.Position)
	}
	if cfg.PromptJumpBar.ColorMode != "default" {
		t.Errorf("expected ColorMode to be default, got %s", cfg.PromptJumpBar.ColorMode)
	}
	if cfg.ToolDensityMode != "muted" {
		t.Errorf("expected ToolDensityMode to be muted, got %s", cfg.ToolDensityMode)
	}
	if !cfg.BreakerLineEnabled {
		t.Errorf("expected BreakerLineEnabled to be true by default")
	}
	if cfg.DefaultNewProject != "auto" {
		t.Errorf("expected DefaultNewProject to be 'auto', got %s", cfg.DefaultNewProject)
	}
}

func TestStore_Operations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "enhancements-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "enhancements.json")
	store, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := store.GetConfig()
	if !cfg.PromptJumpBar.Enabled {
		t.Errorf("expected PromptJumpBar to be enabled")
	}

	// Toggle prompt jump bar off
	if err := store.TogglePromptJumpBar(false); err != nil {
		t.Fatalf("failed to toggle prompt jump bar: %v", err)
	}

	cfgAfter := store.GetConfig()
	if cfgAfter.PromptJumpBar.Enabled {
		t.Errorf("expected PromptJumpBar to be disabled after toggle")
	}

	// Set color mode and custom color
	if err := store.SetPromptJumpBarColor("custom", "#7c3aed"); err != nil {
		t.Fatalf("failed to set color mode: %v", err)
	}

	// Set tool density and breaker line
	if err := store.SetToolDensityMode("hidden"); err != nil {
		t.Fatalf("failed to set tool density: %v", err)
	}
	if err := store.SetBreakerLine(false); err != nil {
		t.Fatalf("failed to set breaker line: %v", err)
	}
	if err := store.SetDefaultNewProject("Antigravity Swiss Knife"); err != nil {
		t.Fatalf("failed to set default new project: %v", err)
	}

	// Reload in another store instance to verify persistence
	store2, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to load store2: %v", err)
	}

	loaded := store2.GetConfig()
	if loaded.PromptJumpBar.ColorMode != "custom" || loaded.PromptJumpBar.CustomColor != "#7c3aed" {
		t.Errorf("expected custom color to persist, got mode=%s color=%s", loaded.PromptJumpBar.ColorMode, loaded.PromptJumpBar.CustomColor)
	}
	if loaded.ToolDensityMode != "hidden" {
		t.Errorf("expected ToolDensityMode hidden to persist, got %s", loaded.ToolDensityMode)
	}
	if loaded.BreakerLineEnabled != false {
		t.Errorf("expected BreakerLineEnabled false to persist")
	}
	if loaded.DefaultNewProject != "Antigravity Swiss Knife" {
		t.Errorf("expected DefaultNewProject to persist, got %s", loaded.DefaultNewProject)
	}
}

func TestGenerateEnhancementsScript(t *testing.T) {
	cfg := DefaultConfig()
	script := GenerateEnhancementsScript(cfg)

	if !strings.Contains(script, "swiss-prompt-jump-bar") {
		t.Errorf("expected script to contain swiss-prompt-jump-bar element ID")
	}
	if !strings.Contains(script, "user-input-step") {
		t.Errorf("expected script to query user-input-step testid")
	}
	if !strings.Contains(script, "swiss-prompt-tooltip") {
		t.Errorf("expected script to manage swiss-prompt-tooltip")
	}
	if !strings.Contains(script, "scrollIntoView") {
		t.Errorf("expected script to call scrollIntoView")
	}
	if !strings.Contains(script, "getLowestPromptOnScreen") {
		t.Errorf("expected script to implement getLowestPromptOnScreen")
	}
	if !strings.Contains(script, "getActiveAndHoverColor") {
		t.Errorf("expected script to implement getActiveAndHoverColor")
	}
	if !strings.Contains(script, "worked-for-collapsible") {
		t.Errorf("expected script to target worked-for-collapsible")
	}
	if !strings.Contains(script, "thinking-collapsible-trigger") {
		t.Errorf("expected script to target thinking-collapsible-trigger")
	}
	if !strings.Contains(script, "applyDefaultProjectHandler") {
		t.Errorf("expected script to implement applyDefaultProjectHandler")
	}
	if !strings.Contains(script, "new-conversation-button") {
		t.Errorf("expected script to intercept new-conversation-button")
	}
}

func TestOverviewPanel_AuxTabsFormat(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.OverviewPanel.AuxTabsFormat != "icon" {
		t.Errorf("expected AuxTabsFormat to be 'icon' by default, got %s", cfg.OverviewPanel.AuxTabsFormat)
	}

	tmpDir, err := os.MkdirTemp("", "enhancements-aux-tabs-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(filepath.Join(tmpDir, "enhancements.json"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	if err := store.SetAuxTabsFormat("icon_and_name"); err != nil {
		t.Fatalf("failed to set aux tabs format: %v", err)
	}

	if store.GetConfig().OverviewPanel.AuxTabsFormat != "icon_and_name" {
		t.Errorf("expected AuxTabsFormat to be 'icon_and_name', got %s", store.GetConfig().OverviewPanel.AuxTabsFormat)
	}

	// Invalid fallback to "icon"
	if err := store.SetAuxTabsFormat("something_else"); err != nil {
		t.Fatalf("failed to set aux tabs format: %v", err)
	}
	if store.GetConfig().OverviewPanel.AuxTabsFormat != "icon" {
		t.Errorf("expected AuxTabsFormat fallback to 'icon', got %s", store.GetConfig().OverviewPanel.AuxTabsFormat)
	}
}

