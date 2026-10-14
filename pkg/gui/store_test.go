package gui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestUpdateConfigPartial_PreservesOmittedFields(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	initialCfg := store.GetConfig()
	if !initialCfg.Enabled || !initialCfg.ColorStylingEnabled || !initialCfg.AutoInject {
		t.Fatalf("expected initial config defaults to be true, got %+v", initialCfg)
	}

	if err := store.SetProjectColor("ProjectAlpha", "#0b57d0"); err != nil {
		t.Fatalf("failed to set project color: %v", err)
	}

	// Send partial JSON with only a single telemetry flag
	partialJSON := []byte(`{"chat_telemetry_output_tokens": false}`)
	updated, err := store.UpdateConfigPartial(partialJSON)
	if err != nil {
		t.Fatalf("UpdateConfigPartial returned error: %v", err)
	}

	if updated.ChatTelemetryOutputTokens {
		t.Fatalf("expected chat_telemetry_output_tokens to be false, got true")
	}

	// Verify all other omitted fields were preserved
	if !updated.Enabled {
		t.Fatalf("expected Enabled to remain true")
	}
	if !updated.ColorStylingEnabled {
		t.Fatalf("expected ColorStylingEnabled to remain true")
	}
	if !updated.AutoInject {
		t.Fatalf("expected AutoInject to remain true")
	}
	if !updated.ReplaceSeeAllTriangle {
		t.Fatalf("expected ReplaceSeeAllTriangle to remain true")
	}
	if !updated.ConsistentProjectSpacing {
		t.Fatalf("expected ConsistentProjectSpacing to remain true")
	}
	if !updated.ConsistentProjectSpacingLine {
		t.Fatalf("expected ConsistentProjectSpacingLine to remain true")
	}
	if !updated.DragRearrangeEnabled {
		t.Fatalf("expected DragRearrangeEnabled to remain true")
	}
	if updated.ProjectColors["ProjectAlpha"] != "#0b57d0" {
		t.Fatalf("expected project color for ProjectAlpha to be #0b57d0, got %q", updated.ProjectColors["ProjectAlpha"])
	}
}

func TestUpdateConfigPartial_MergesProjectColors(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := store.SetProjectColor("ProjectBeta", "#059669"); err != nil {
		t.Fatalf("failed to set initial project color: %v", err)
	}

	// Send partial update with a new project color
	partialJSON := []byte(`{"project_colors": {"AlphaProject": "#ff5500"}}`)
	updated, err := store.UpdateConfigPartial(partialJSON)
	if err != nil {
		t.Fatalf("UpdateConfigPartial returned error: %v", err)
	}

	if updated.ProjectColors["AlphaProject"] != "#ff5500" {
		t.Fatalf("expected AlphaProject to be #ff5500, got %q", updated.ProjectColors["AlphaProject"])
	}

	// Existing project color must still be present
	if updated.ProjectColors["ProjectBeta"] != "#059669" {
		t.Fatalf("expected ProjectBeta to be retained as #059669, got %q", updated.ProjectColors["ProjectBeta"])
	}
}

func TestStoreLoad_AutoHealsMissingDefaults(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "gui_improvements.json")

	// Write empty JSON object
	if err := os.WriteFile(configPath, []byte(`{}`), 0600); err != nil {
		t.Fatalf("failed to write empty json: %v", err)
	}

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := store.GetConfig()
	if !cfg.Enabled || !cfg.ColorStylingEnabled || !cfg.AutoInject || !cfg.DragRearrangeEnabled ||
		!cfg.ReplaceSeeAllTriangle || !cfg.ConsistentProjectSpacing || !cfg.ConsistentProjectSpacingLine ||
		!cfg.ChatTelemetryInputTokens || !cfg.ChatTelemetryOutputTokens || !cfg.ChatTelemetryCacheHitRatio ||
		!cfg.ChatTelemetryGenerationSpeed {
		t.Fatalf("expected all missing defaults to be healed to true, got %+v", cfg)
	}

	if cfg.ProjectColors == nil {
		t.Fatalf("expected ProjectColors to be initialized")
	}
	if cfg.ProjectOrder == nil {
		t.Fatalf("expected ProjectOrder to be initialized")
	}
}

func TestStoreLoad_AutoHealsCorruptedZeroedDefaults(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "gui_improvements.json")

	// Corrupted state where core feature flags were zeroed
	corrupted := map[string]interface{}{
		"enabled":                    false,
		"color_styling_enabled":      false,
		"replace_see_all_triangle":   false,
		"consistent_project_spacing": false,
		"auto_inject":                false,
		"project_colors":             map[string]string{},
	}
	data, _ := json.Marshal(corrupted)
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("failed to write corrupted json: %v", err)
	}

	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := store.GetConfig()
	if !cfg.Enabled || !cfg.ColorStylingEnabled || !cfg.ReplaceSeeAllTriangle || !cfg.ConsistentProjectSpacing || !cfg.AutoInject {
		t.Fatalf("expected corrupted zeroed configuration to be healed to true, got %+v", cfg)
	}

	if cfg.ProjectColors == nil {
		t.Fatalf("expected ProjectColors map to be initialized")
	}
}

func TestCopyFile_Enforces0644Permissions(t *testing.T) {
	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "src.txt")
	dst := filepath.Join(tempDir, "dst.txt")

	// Write source file with restrictive 0600 permissions
	if err := os.WriteFile(src, []byte("permission test"), 0600); err != nil {
		t.Fatalf("failed to write src: %v", err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("failed to stat dst: %v", err)
	}

	// Permissions on dst must have 0644
	perm := info.Mode().Perm()
	if runtime.GOOS != "windows" && perm != 0644 {
		t.Fatalf("expected 0644 permissions on destination file, got %#o", perm)
	}
}

func TestDesktopLoader_RefreshesBackupWhenUninstalled(t *testing.T) {
	tempDir := t.TempDir()
	asarPath := filepath.Join(tempDir, "app.asar")
	backupPath := filepath.Join(tempDir, "app.asar.factory_backup")

	// Create clean upstream asar v2 and older backup v1
	if err := os.WriteFile(asarPath, []byte("clean-upstream-version-2"), 0644); err != nil {
		t.Fatalf("failed to write asar: %v", err)
	}
	if err := os.WriteFile(backupPath, []byte("outdated-backup-version-1"), 0644); err != nil {
		t.Fatalf("failed to write backup: %v", err)
	}

	dm := NewDesktopManager(tempDir, filepath.Join(tempDir, "gui.json"), nil)
	status := dm.GetStatus()
	if status.Installed {
		t.Fatalf("expected clean upstream asar to be reported as not installed")
	}

	// Trigger InstallDesktopLoader (will refresh backup before failing extraction)
	_, _ = dm.InstallDesktopLoader()

	// Backup must now contain clean-upstream-version-2!
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read backup: %v", err)
	}
	if string(backupContent) != "clean-upstream-version-2" {
		t.Fatalf("expected backup to be refreshed to 'clean-upstream-version-2', got %q", string(backupContent))
	}
}

func TestDesktopLoader_PreservesBackupWhenAlreadyInstalled(t *testing.T) {
	tempDir := t.TempDir()
	asarPath := filepath.Join(tempDir, "app.asar")
	backupPath := filepath.Join(tempDir, "app.asar.factory_backup")

	// Create already-patched asar containing SwissLoaderMarker
	patchedContent := "already-patched-content-" + SwissLoaderMarker
	if err := os.WriteFile(asarPath, []byte(patchedContent), 0644); err != nil {
		t.Fatalf("failed to write asar: %v", err)
	}
	if err := os.WriteFile(backupPath, []byte("clean-factory-backup"), 0644); err != nil {
		t.Fatalf("failed to write backup: %v", err)
	}

	dm := NewDesktopManager(tempDir, filepath.Join(tempDir, "gui.json"), nil)
	status := dm.GetStatus()
	if !status.Installed {
		t.Fatalf("expected patched asar to be reported as installed")
	}

	// Trigger InstallDesktopLoader
	_, _ = dm.InstallDesktopLoader()

	// Backup must still be clean-factory-backup and NOT overwritten with patched content
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read backup: %v", err)
	}
	if string(backupContent) != "clean-factory-backup" {
		t.Fatalf("expected backup to be preserved as 'clean-factory-backup', got %q", string(backupContent))
	}
}
