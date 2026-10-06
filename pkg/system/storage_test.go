package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

func TestStorageInfo_Defaults(t *testing.T) {
	cfg := core.DefaultConfig()
	info := GetStorageInfo(cfg)

	if info.StorageMode != "system_default" {
		t.Errorf("expected storage_mode to be 'system_default', got %q", info.StorageMode)
	}

	if info.CurrentPaths.ConfigDir != info.SystemDefaultPaths.ConfigDir {
		t.Errorf("expected current config dir to match system default, got %q vs %q",
			info.CurrentPaths.ConfigDir, info.SystemDefaultPaths.ConfigDir)
	}

	if info.AppExecutionType == "" {
		t.Errorf("expected non-empty app execution type")
	}
}

func TestStorageMode_SwitchAndMigrate(t *testing.T) {
	tmpDir := t.TempDir()
	sysDir := filepath.Join(tmpDir, "sys_config")
	portableDir := filepath.Join(tmpDir, "app_data")

	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", sysDir)
	t.Setenv("ANTIGRAVITY_SWISS_PORTABLE_DIR", portableDir)

	_ = os.MkdirAll(sysDir, 0700)
	accountsJSON := filepath.Join(sysDir, "accounts.json")
	_ = os.WriteFile(accountsJSON, []byte(`[{"id":"user@example.com","email":"user@example.com"}]`), 0600)

	cfg := core.DefaultConfig()
	cfg.StorageMode = "system_default"

	// Verify initial state
	info := GetStorageInfo(cfg)
	if !info.CanMigrate {
		t.Errorf("expected can_migrate to be true since accounts.json exists in sys_config")
	}

	// Switch to app_portable with migration
	newInfo, err := SwitchStorageMode("app_portable", true, cfg)
	if err != nil {
		t.Fatalf("SwitchStorageMode failed: %v", err)
	}

	if newInfo.StorageMode != "app_portable" {
		t.Errorf("expected new storage mode to be app_portable, got %q", newInfo.StorageMode)
	}

	// Verify accounts.json was copied to portableDir
	migratedAccounts := filepath.Join(portableDir, "accounts.json")
	if !pathExists(migratedAccounts) {
		t.Errorf("expected accounts.json to be migrated to %q", migratedAccounts)
	}

	data, _ := os.ReadFile(migratedAccounts)
	if string(data) != `[{"id":"user@example.com","email":"user@example.com"}]` {
		t.Errorf("migrated content mismatch: %s", string(data))
	}

	// Switch back to system_default without error
	backInfo, err := SwitchStorageMode("system_default", false, cfg)
	if err != nil {
		t.Fatalf("SwitchStorageMode back failed: %v", err)
	}
	if backInfo.StorageMode != "system_default" {
		t.Errorf("expected storage mode to be system_default, got %q", backInfo.StorageMode)
	}
}

func TestStorageMode_InvalidMode(t *testing.T) {
	cfg := core.DefaultConfig()
	_, err := SwitchStorageMode("invalid_mode", false, cfg)
	if err == nil {
		t.Errorf("expected error for invalid storage mode")
	}
}
