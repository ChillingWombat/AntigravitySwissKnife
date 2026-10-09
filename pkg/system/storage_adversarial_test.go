package system

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// TestAdversarial_AppPortableRejection verifies that SwitchStorageMode
// strictly and unconditionally rejects "app_portable" and never creates ./data or copies files.
func TestAdversarial_AppPortableRejection(t *testing.T) {
	tmpDir := t.TempDir()
	sysDir := filepath.Join(tmpDir, "sys_config")
	appDir := filepath.Join(tmpDir, "app_dir")
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", sysDir)
	t.Setenv("ANTIGRAVITY_SWISS_APP_DIR", appDir)

	if err := os.MkdirAll(sysDir, 0700); err != nil {
		t.Fatalf("failed to create sysDir: %v", err)
	}
	if err := os.MkdirAll(appDir, 0700); err != nil {
		t.Fatalf("failed to create appDir: %v", err)
	}

	// Create dummy credential & config files in sysDir
	_ = os.WriteFile(filepath.Join(sysDir, "accounts.json"), []byte(`[{"id":"user1"}]`), 0600)
	_ = os.WriteFile(filepath.Join(sysDir, "config.json"), []byte(`{"storage_mode":"system_default"}`), 0600)

	cfg := core.DefaultConfig()

	// 1. Attempt SwitchStorageMode with "app_portable" and migrateData = true
	info, err := SwitchStorageMode("app_portable", true, cfg)
	if err == nil {
		t.Fatalf("CRITICAL SECURITY FLAW: SwitchStorageMode accepted 'app_portable' with migrateData=true")
	}
	if info != nil {
		t.Fatalf("CRITICAL: SwitchStorageMode returned non-nil info on error: %+v", info)
	}

	// Verify ./data was NOT created anywhere in appDir
	expectedDataDir := filepath.Join(appDir, "data")
	if _, statErr := os.Stat(expectedDataDir); !os.IsNotExist(statErr) {
		t.Fatalf("CRITICAL: SwitchStorageMode created ./data directory at %q despite error!", expectedDataDir)
	}

	// 2. Attempt SwitchStorageMode with "app_portable" and migrateData = false
	info2, err2 := SwitchStorageMode("app_portable", false, cfg)
	if err2 == nil {
		t.Fatalf("CRITICAL: SwitchStorageMode accepted 'app_portable' with migrateData=false")
	}
	if info2 != nil {
		t.Fatalf("CRITICAL: SwitchStorageMode returned non-nil info on error: %+v", info2)
	}

	// Verify cfg.StorageMode remains unchanged
	if cfg.StorageMode != "system_default" {
		t.Fatalf("cfg.StorageMode mutated to %q despite failure", cfg.StorageMode)
	}

	// 3. Test permutations and case sensitivity
	permutations := []string{
		"APP_PORTABLE",
		"App_Portable",
		" app_portable",
		"app_portable ",
		"app_portable\x00",
		"app_portable/sub",
		"../app_portable",
		"portable",
	}

	for _, mode := range permutations {
		t.Run("Permutation_"+mode, func(t *testing.T) {
			pInfo, pErr := SwitchStorageMode(mode, true, cfg)
			if pErr == nil {
				t.Fatalf("CRITICAL: SwitchStorageMode accepted permutation %q", mode)
			}
			if pInfo != nil {
				t.Fatalf("returned non-nil info for %q", mode)
			}
			if _, statErr := os.Stat(expectedDataDir); !os.IsNotExist(statErr) {
				t.Fatalf("created ./data for permutation %q", mode)
			}
		})
	}
}

// TestAdversarial_LegacyConfigHealing verifies that any legacy config file on disk
// containing "app_portable" or invalid values is automatically healed to "system_default"
// and GetStorageInfo strictly returns "system_default" and can_migrate = false.
func TestAdversarial_LegacyConfigHealing(t *testing.T) {
	tmpDir := t.TempDir()
	sysDir := filepath.Join(tmpDir, "sys_config")
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", sysDir)
	_ = os.MkdirAll(sysDir, 0700)

	legacyConfigPath := filepath.Join(sysDir, "config.json")
	legacyContent := []byte(`{
		"storage_mode": "app_portable",
		"auto_switch_threshold": 0.5,
		"memo": {
			"storage_location": "global"
		}
	}`)
	if err := os.WriteFile(legacyConfigPath, legacyContent, 0600); err != nil {
		t.Fatalf("failed to write legacy config: %v", err)
	}

	// Load config
	cfg, err := core.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Must be healed to "system_default"
	if cfg.StorageMode != "system_default" {
		t.Fatalf("expected healed StorageMode 'system_default', got %q", cfg.StorageMode)
	}

	// GetStorageInfo check
	info := GetStorageInfo(cfg)
	if info.StorageMode != "system_default" {
		t.Fatalf("GetStorageInfo reported %q instead of 'system_default'", info.StorageMode)
	}
	if info.CanMigrate {
		t.Fatalf("GetStorageInfo reported can_migrate=true under lockdown")
	}
	if info.AppPortablePaths.ConfigDir != info.SystemDefaultPaths.ConfigDir {
		t.Fatalf("AppPortablePaths.ConfigDir %q does not match SystemDefaultPaths.ConfigDir %q",
			info.AppPortablePaths.ConfigDir, info.SystemDefaultPaths.ConfigDir)
	}

	// Save config and reload from disk
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	reloadedData, err := os.ReadFile(legacyConfigPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(reloadedData, &parsed); err != nil {
		t.Fatalf("unmarshal reloaded config failed: %v", err)
	}
	if parsed["storage_mode"] != "system_default" {
		t.Fatalf("disk config still contains storage_mode=%v, expected 'system_default'", parsed["storage_mode"])
	}
}

// TestAdversarial_DirectoryTraversalAndInjection tests arbitrary paths and attack strings in newMode.
func TestAdversarial_DirectoryTraversalAndInjection(t *testing.T) {
	tmpDir := t.TempDir()
	sysDir := filepath.Join(tmpDir, "sys_config")
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", sysDir)
	_ = os.MkdirAll(sysDir, 0700)

	cfg := core.DefaultConfig()

	attacks := []string{
		"../../../../etc/passwd",
		"/tmp/evil_storage",
		"system_default/../../evil",
		"system_default\x00extra",
		"\x00",
		"",
		"null",
		"undefined",
		"true",
		"false",
		"{\"storage_mode\":\"app_portable\"}",
		"$(touch /tmp/pwned)",
		"; rm -rf / ;",
	}

	for _, attack := range attacks {
		t.Run("Attack_"+attack, func(t *testing.T) {
			info, err := SwitchStorageMode(attack, false, cfg)
			if err == nil {
				t.Fatalf("CRITICAL: SwitchStorageMode accepted injection string %q", attack)
			}
			if info != nil {
				t.Fatalf("returned non-nil info for %q", attack)
			}
			if cfg.StorageMode != "system_default" {
				t.Fatalf("StorageMode corrupted to %q", cfg.StorageMode)
			}
		})
	}
}

// TestAdversarial_ConcurrencyStress verifies thread-safety and lockdown under heavy concurrent calls.
func TestAdversarial_ConcurrencyStress(t *testing.T) {
	tmpDir := t.TempDir()
	sysDir := filepath.Join(tmpDir, "sys_config")
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", sysDir)
	_ = os.MkdirAll(sysDir, 0700)

	cfg := core.DefaultConfig()

	var wg sync.WaitGroup
	workers := 50
	iterations := 20

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Alternately try malicious and valid calls
				if (id+j)%2 == 0 {
					_, _ = SwitchStorageMode("app_portable", true, cfg)
				} else {
					_, _ = SwitchStorageMode("system_default", false, cfg)
				}
				info := GetStorageInfo(cfg)
				if info.StorageMode != "system_default" {
					t.Errorf("race condition detected: storage_mode became %q", info.StorageMode)
				}
				if info.CanMigrate {
					t.Errorf("race condition detected: can_migrate became true")
				}
			}
		}(i)
	}

	wg.Wait()
}
