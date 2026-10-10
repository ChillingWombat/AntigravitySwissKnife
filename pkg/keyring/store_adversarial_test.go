package keyring

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestAdversarial_ReconcileActiveAccount_AutoImport_NeverSwitchesActiveAccount(t *testing.T) {
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	tmpDir := t.TempDir()
	accPath := filepath.Join(tmpDir, "accounts.json")
	store, err := NewStore(accPath)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	activeAcc := &Account{
		Email:        "active_owner@domain.com",
		Label:        "Active Owner",
		Status:       "ACTIVE",
		IsActive:     true,
		RefreshToken: "1//existing-rt",
		AccessToken:  "existing-at",
	}
	standbyAcc := &Account{
		Email:        "standby@domain.com",
		Label:        "Standby",
		Status:       "STANDBY",
		IsActive:     false,
		RefreshToken: "1//standby-rt",
	}
	_ = store.AddOrUpdateAccount(activeAcc)
	_ = store.AddOrUpdateAccount(standbyAcc)
	_ = store.SetActiveAccount("active_owner@domain.com")
	store.lastManualSwitchTime = time.Time{}

	if store.ActiveAccount() != "active_owner@domain.com" {
		t.Fatalf("setup failed: active account is %s", store.ActiveAccount())
	}

	// Mock rogue intruder session on host surface
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	antigravityDir := filepath.Join(tmpHome, ".config", "Antigravity")
	_ = os.MkdirAll(antigravityDir, 0755)
	_ = os.WriteFile(filepath.Join(antigravityDir, "app_storage.json"), []byte(`{"jetski.onboarding.lastLoginUsername":"intruder_unimported@domain.com"}`), 0644)
	t.Setenv("ANTIGRAVITY_CONFIG_DIR", antigravityDir)
	store.homeDir = tmpHome
	store.antigravityConfigDir = antigravityDir

	allEmails := []string{"active_owner@domain.com", "standby@domain.com"}

	reconciled, err := store.ReconcileActiveAccount(true, allEmails, nil)
	if err != nil {
		t.Fatalf("ReconcileActiveAccount error: %v", err)
	}

	// Active account MUST NOT switch!
	if store.ActiveAccount() != "active_owner@domain.com" {
		t.Fatalf("CRITICAL BUG: active account switched to %s!", store.ActiveAccount())
	}
	if reconciled == nil || reconciled.Email != "active_owner@domain.com" {
		t.Fatalf("expected returned account to be active_owner@domain.com, got %v", reconciled)
	}

	// Verify intruder in store is STANDBY and NOT active
	intruder, err := store.GetAccount("intruder_unimported@domain.com")
	if err != nil || intruder == nil {
		t.Fatalf("expected intruder to be auto-imported into vault: %v", err)
	}
	if intruder.IsActive {
		t.Errorf("CRITICAL BUG: intruder.IsActive is true!")
	}
	if intruder.Status != "STANDBY" {
		t.Errorf("CRITICAL BUG: intruder.Status is %s, expected STANDBY!", intruder.Status)
	}

	// Verify disk persistence
	diskData, err := os.ReadFile(accPath)
	if err != nil {
		t.Fatalf("failed reading accounts.json: %v", err)
	}
	var diskMap struct {
		ActiveAccount string `json:"active_account"`
		Accounts      map[string]struct {
			Email    string `json:"email"`
			Status   string `json:"status"`
			IsActive bool   `json:"is_active"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(diskData, &diskMap); err != nil {
		t.Fatalf("unmarshal disk accounts.json: %v", err)
	}
	if diskMap.ActiveAccount != "active_owner@domain.com" {
		t.Errorf("CRITICAL BUG: disk active_account switched to %s!", diskMap.ActiveAccount)
	}
	if accOnDisk, ok := diskMap.Accounts["intruder_unimported@domain.com"]; !ok || accOnDisk.Status != "STANDBY" {
		t.Errorf("CRITICAL BUG: disk intruder is not STANDBY: %+v", accOnDisk)
	}
}

func TestAdversarial_CredentialParsingFromMessyNotes(t *testing.T) {
	testCases := []struct {
		name         string
		notes        string
		expectedRF   string
		expectedAT   string
		expectRecov  bool
	}{
		{
			name:        "Markdown code block",
			notes:       "Here are credentials:\n```json\n{\n  \"refresh_token\": \"1//rt_fenced\",\n  \"access_token\": \"ya29.at_fenced\"\n}\n```\nEnjoy!",
			expectedRF:  "1//rt_fenced",
			expectedAT:  "ya29.at_fenced",
			expectRecov: true,
		},
		{
			name:        "Multiple braces and non-JSON text before and after",
			notes:       "User info: {dept: engineering}. Auth tokens: {\"refresh_token\": \"1//rt_multi\", \"token\": \"ya29.at_multi\"} end of note.",
			expectedRF:  "1//rt_multi",
			expectedAT:  "ya29.at_multi",
			expectRecov: true,
		},
		{
			name:        "Single-quoted JSON dict",
			notes:       "{'refresh_token': '1//rt_single_quote', 'access_token': 'ya29.at_single_quote'}",
			expectedRF:  "1//rt_single_quote",
			expectedAT:  "ya29.at_single_quote",
			expectRecov: true,
		},
		{
			name:        "Deeply nested JSON structure",
			notes:       "{\"wrapper\": {\"google_oauth\": {\"refresh_token\": \"1//rt_deep_nest\", \"token\": \"ya29.at_deep_nest\"}}}",
			expectedRF:  "1//rt_deep_nest",
			expectedAT:  "ya29.at_deep_nest",
			expectRecov: true,
		},
		{
			name:        "Notes with escaped quotes inside values",
			notes:       "{\"description\": \"test \\\"escaped\\\" note\", \"refresh_token\": \"1//rt_escaped\", \"access_token\": \"ya29.at_escaped\"}",
			expectedRF:  "1//rt_escaped",
			expectedAT:  "ya29.at_escaped",
			expectRecov: true,
		},
		{
			name:        "Notes with no refresh_token",
			notes:       "{\"access_token\": \"ya29.no_refresh\"}",
			expectedRF:  "",
			expectedAT:  "",
			expectRecov: false,
		},
		{
			name:        "Empty notes",
			notes:       "",
			expectedRF:  "",
			expectedAT:  "",
			expectRecov: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			acc := &Account{
				Email:        "adversarial@domain.com",
				Status:       "ERROR",
				ErrorMessage: "Missing credentials / re-authentication required",
				Notes:        tc.notes,
			}

			recovered := acc.RecoverCredentialsFromNotes()
			if recovered != tc.expectRecov {
				t.Fatalf("expected recover=%v, got %v", tc.expectRecov, recovered)
			}

			if tc.expectRecov {
				if acc.RefreshToken != tc.expectedRF {
					t.Errorf("expected RF=%s, got %s", tc.expectedRF, acc.RefreshToken)
				}
				if tc.expectedAT != "" && acc.AccessToken != tc.expectedAT {
					t.Errorf("expected AT=%s, got %s", tc.expectedAT, acc.AccessToken)
				}
				if acc.Credential == nil || acc.Credential.RefreshToken != tc.expectedRF {
					t.Errorf("expected acc.Credential.RefreshToken to equal %s", tc.expectedRF)
				}
				if acc.ErrorMessage != "" {
					t.Errorf("expected ErrorMessage to be cleared, got %q", acc.ErrorMessage)
				}
				if acc.Status != "STANDBY" {
					t.Errorf("expected Status to be restored to STANDBY, got %s", acc.Status)
				}
			}
		})
	}
}
