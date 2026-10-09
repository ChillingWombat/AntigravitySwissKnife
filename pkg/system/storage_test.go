package system

import (
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

	if info.CanMigrate {
		t.Errorf("expected can_migrate to be false under system_default lockdown")
	}
}

func TestStorageMode_SwitchAndMigrate(t *testing.T) {
	tmpDir := t.TempDir()
	sysDir := filepath.Join(tmpDir, "sys_config")
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", sysDir)

	cfg := core.DefaultConfig()
	cfg.StorageMode = "system_default"

	// Verify initial state: storage locked to system_default, can_migrate is false
	info := GetStorageInfo(cfg)
	if info.StorageMode != "system_default" {
		t.Errorf("expected storage_mode to be 'system_default', got %q", info.StorageMode)
	}
	if info.CanMigrate {
		t.Errorf("expected can_migrate to be false under system_default lockdown")
	}

	// Attempting to switch to app_portable must be rejected
	_, err := SwitchStorageMode("app_portable", true, cfg)
	if err == nil {
		t.Fatalf("expected error when switching to app_portable, got nil")
	}

	// Switching to system_default succeeds and maintains lockdown
	sysInfo, err := SwitchStorageMode("system_default", false, cfg)
	if err != nil {
		t.Fatalf("SwitchStorageMode system_default failed: %v", err)
	}
	if sysInfo.StorageMode != "system_default" {
		t.Errorf("expected storage mode to be system_default, got %q", sysInfo.StorageMode)
	}
	if sysInfo.CanMigrate {
		t.Errorf("expected can_migrate to remain false")
	}
}

func TestStorageMode_InvalidMode(t *testing.T) {
	cfg := core.DefaultConfig()
	_, err := SwitchStorageMode("invalid_mode", false, cfg)
	if err == nil {
		t.Errorf("expected error for invalid storage mode")
	}
}
