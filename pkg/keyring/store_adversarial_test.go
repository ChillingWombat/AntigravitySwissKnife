package keyring

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAdversarial_ReconcileActiveAccount_UnimportedSessionNoWipe(t *testing.T) {
	tmpDir := t.TempDir()
	accPath := filepath.Join(tmpDir, "accounts.json")
	store, err := NewStore(accPath)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	activeAcc := &Account{
		Email:    "primary_active@domain.com",
		Label:    "Primary Active",
		Status:   "ACTIVE",
		IsActive: true,
	}
	standby1 := &Account{
		Email:    "standby_1@domain.com",
		Label:    "Standby 1",
		Status:   "STANDBY",
		IsActive: false,
	}
	standby2 := &Account{
		Email:    "standby_2@domain.com",
		Label:    "Standby 2",
		Status:   "STANDBY",
		IsActive: false,
	}

	_ = store.AddOrUpdateAccount(activeAcc)
	_ = store.AddOrUpdateAccount(standby1)
	_ = store.AddOrUpdateAccount(standby2)
	_ = store.SetActiveAccount("primary_active@domain.com")

	if store.ActiveAccount() != "primary_active@domain.com" {
		t.Fatalf("setup failed: active account is %s", store.ActiveAccount())
	}

	// Mock external unimported session on disk in app_storage.json
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	antigravityDir := filepath.Join(tmpHome, ".config", "Antigravity")
	_ = os.MkdirAll(antigravityDir, 0755)
	_ = os.WriteFile(filepath.Join(antigravityDir, "app_storage.json"), []byte(`{"jetski.onboarding.lastLoginUsername":"unimported_intruder@gmail.com"}`), 0644)
	t.Setenv("ANTIGRAVITY_CONFIG_DIR", antigravityDir)

	allEmails := []string{"primary_active@domain.com", "standby_1@domain.com", "standby_2@domain.com"}
	reconciled, err := store.ReconcileActiveAccount(false, allEmails, nil)
	if err != nil {
		t.Fatalf("ReconcileActiveAccount returned unexpected error: %v", err)
	}

	// 1. Vault activeEmail must NOT be wiped!
	if store.ActiveAccount() != "primary_active@domain.com" {
		t.Errorf("CRITICAL BUG: active account was wiped or modified! Expected primary_active@domain.com, got %q", store.ActiveAccount())
	}

	// 2. primary_active must still have IsActive=true and Status=ACTIVE
	storedActive, _ := store.GetAccount("primary_active@domain.com")
	if storedActive == nil || !storedActive.IsActive || storedActive.Status != "ACTIVE" {
		t.Errorf("CRITICAL BUG: active account state corrupted: %+v", storedActive)
	}

	// 3. Standby accounts must remain untouched
	s1, _ := store.GetAccount("standby_1@domain.com")
	s2, _ := store.GetAccount("standby_2@domain.com")
	if s1 == nil || s1.IsActive || s1.Status != "STANDBY" {
		t.Errorf("CRITICAL BUG: standby1 state corrupted: %+v", s1)
	}
	if s2 == nil || s2.IsActive || s2.Status != "STANDBY" {
		t.Errorf("CRITICAL BUG: standby2 state corrupted: %+v", s2)
	}

	// 4. Reconciled account returned must be the preserved active account
	if reconciled == nil || reconciled.Email != "primary_active@domain.com" {
		t.Errorf("expected reconciled account to be primary_active@domain.com, got %v", reconciled)
	}

	// 5. Inspect accounts.json directly on disk to verify persistence
	diskData, err := os.ReadFile(accPath)
	if err != nil {
		t.Fatalf("failed to read accounts.json: %v", err)
	}
	var diskMap struct {
		ActiveAccount string `json:"active_account"`
		Accounts      map[string]struct {
			Email  string `json:"email"`
			Status string `json:"status"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(diskData, &diskMap); err != nil {
		t.Fatalf("failed to unmarshal disk accounts.json: %v", err)
	}
	if diskMap.ActiveAccount != "primary_active@domain.com" {
		t.Errorf("CRITICAL BUG: disk accounts.json active_account was wiped! Got %q", diskMap.ActiveAccount)
	}
	if accOnDisk, ok := diskMap.Accounts["primary_active@domain.com"]; !ok || accOnDisk.Status != "ACTIVE" {
		t.Errorf("CRITICAL BUG: disk accounts.json primary_active.Status is not ACTIVE! Got %+v", accOnDisk)
	}
}

func TestAdversarial_ReconcileActiveAccount_EmptyVaultSafeReturn(t *testing.T) {
	tmpDir := t.TempDir()
	accPath := filepath.Join(tmpDir, "accounts.json")
	store, err := NewStore(accPath)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	// Store has 0 accounts in vault, activeEmail is empty
	if store.ActiveAccount() != "" {
		t.Fatalf("expected initial empty active account, got %s", store.ActiveAccount())
	}

	// Mock external session
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	antigravityDir := filepath.Join(tmpHome, ".config", "Antigravity")
	_ = os.MkdirAll(antigravityDir, 0755)
	_ = os.WriteFile(filepath.Join(antigravityDir, "app_storage.json"), []byte(`{"jetski.onboarding.lastLoginUsername":"unimported@gmail.com"}`), 0644)
	t.Setenv("ANTIGRAVITY_CONFIG_DIR", antigravityDir)

	reconciled, err := store.ReconcileActiveAccount(false, []string{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reconciled != nil {
		t.Errorf("expected nil reconciled account when vault is empty and unimported detected, got %v", reconciled)
	}
	if store.ActiveAccount() != "" {
		t.Errorf("expected empty active account, got %s", store.ActiveAccount())
	}
}

func TestAdversarial_SyncOfflineSurfaces(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")

	antigravityDir := filepath.Join(tmpHome, ".config", "Antigravity")
	_ = os.MkdirAll(antigravityDir, 0755)
	appStoragePath := filepath.Join(antigravityDir, "app_storage.json")
	_ = os.WriteFile(appStoragePath, []byte(`{"jetski.onboarding.lastLoginUsername":"old@gmail.com"}`), 0644)
	t.Setenv("ANTIGRAVITY_CONFIG_DIR", antigravityDir)

	geminiDir := filepath.Join(tmpHome, ".gemini")
	_ = os.MkdirAll(geminiDir, 0755)

	acc := &Account{
		Email:        "new_switched@domain.com",
		RefreshToken: "1//mock-refresh-token",
		AccessToken:  "mock-access-token",
		Status:       "ACTIVE",
		IsActive:     true,
	}

	err := SyncAllSurfaces(acc, []string{"new_switched@domain.com"}, nil)
	if err != nil {
		t.Fatalf("SyncAllSurfaces failed: %v", err)
	}

	// Verify app_storage.json was cleanly updated silently
	data, err := os.ReadFile(appStoragePath)
	if err != nil {
		t.Fatalf("failed to read app_storage.json: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse app_storage.json: %v", err)
	}
	if parsed["jetski.onboarding.lastLoginUsername"] != "new_switched@domain.com" {
		t.Errorf("expected jetski.onboarding.lastLoginUsername to be new_switched@domain.com, got %v", parsed["jetski.onboarding.lastLoginUsername"])
	}

	// Verify standalone token file was written
	standaloneTokenPath := filepath.Join(geminiDir, "jetski-standalone-oauth-token")
	stData, err := os.ReadFile(standaloneTokenPath)
	if err != nil {
		t.Fatalf("failed to read standalone token file: %v", err)
	}
	var tokenPayload AntigravitySecretPayload
	if err := json.Unmarshal(stData, &tokenPayload); err != nil {
		t.Fatalf("failed to unmarshal standalone token file: %v", err)
	}
	if tokenPayload.Token.AccessToken != "mock-access-token" {
		t.Errorf("expected access token to be mock-access-token, got %s", tokenPayload.Token.AccessToken)
	}
}
