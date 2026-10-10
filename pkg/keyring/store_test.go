package keyring

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/fingerprint"
)

func TestKeyringStoreCRUDAndRotation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swiss_test_keyring_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	accPath := filepath.Join(tmpDir, "accounts.json")
	store, err := NewStore(accPath)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	acc1 := &Account{
		Email:      "user1@gmail.com",
		Label:      "Primary Work",
		TOTPSecret: "JBSWY3DPEHPK3PXP",
	}
	acc2 := &Account{
		Email: "user2@gmail.com",
		Label: "Secondary Backup",
	}

	if err := store.AddOrUpdateAccount(acc1); err != nil {
		t.Fatalf("AddOrUpdateAccount acc1 error: %v", err)
	}
	if err := store.AddOrUpdateAccount(acc2); err != nil {
		t.Fatalf("AddOrUpdateAccount acc2 error: %v", err)
	}

	// First account should be auto-active
	if store.ActiveAccount() != "user1@gmail.com" {
		t.Errorf("expected user1 to be active, got %s", store.ActiveAccount())
	}

	// Switch to acc2
	if err := store.SetActiveAccount("user2@gmail.com"); err != nil {
		t.Fatalf("SetActiveAccount error: %v", err)
	}
	if store.ActiveAccount() != "user2@gmail.com" {
		t.Errorf("expected user2 to be active, got %s", store.ActiveAccount())
	}

	// Reload from disk to verify atomic persistence
	store2, err := NewStore(accPath)
	if err != nil {
		t.Fatalf("NewStore reload error: %v", err)
	}
	if store2.ActiveAccount() != "user2@gmail.com" {
		t.Errorf("expected user2 to remain active post reload, got %s", store2.ActiveAccount())
	}
	if len(store2.ListAccounts()) != 2 {
		t.Errorf("expected 2 accounts, got %d", len(store2.ListAccounts()))
	}
}

func TestListAccounts_DeterministicSortedOrder(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swiss_test_sort_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(filepath.Join(tmpDir, "accounts.json"))
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	emails := []string{"zeta@gmail.com", "alpha@gmail.com", "beta@gmail.com", "delta@gmail.com"}
	for _, em := range emails {
		if err := store.AddOrUpdateAccount(&Account{Email: em}); err != nil {
			t.Fatalf("AddOrUpdateAccount error: %v", err)
		}
	}

	for i := 0; i < 10; i++ {
		list := store.ListAccounts()
		if len(list) != 4 {
			t.Fatalf("expected 4 accounts, got %d", len(list))
		}
		if list[0].Email != "alpha@gmail.com" || list[1].Email != "beta@gmail.com" || list[2].Email != "delta@gmail.com" || list[3].Email != "zeta@gmail.com" {
			t.Fatalf("unexpected order: %v, %v, %v, %v", list[0].Email, list[1].Email, list[2].Email, list[3].Email)
		}
	}
}

func TestImportAndScanner(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swiss_test_scan_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	accPath := filepath.Join(tmpDir, "accounts.json")
	store, err := NewStore(accPath)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	// Test ImportAccount
	acc, err := store.ImportAccount("imported@google.com", "1//test_refresh_token", "ya29.test_access", "My Label", "JBSWY3DPEHPK3PXP")
	if err != nil {
		t.Fatalf("ImportAccount error: %v", err)
	}
	if acc.RefreshToken != "1//test_refresh_token" {
		t.Errorf("expected refresh token to be saved")
	}
	if !acc.HasTOTP {
		t.Errorf("expected HasTOTP to be true")
	}

	// Test Scanner
	scanner := NewScanner(store)
	results, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scanner.Scan error: %v", err)
	}
	found := false
	for _, r := range results {
		if r.Email == "imported@google.com" {
			found = true
			if !r.HasRefresh {
				t.Errorf("expected scanned account to have refresh token flag")
			}
		}
	}
	if !found {
		t.Errorf("expected scanner to find imported account")
	}
}

func TestMultiSurfaceResolutionAndAutoImport(t *testing.T) {
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	configDir := filepath.Join(tmpDir, ".config", "Antigravity")
	_ = os.MkdirAll(configDir, 0755)
	t.Setenv("ANTIGRAVITY_CONFIG_DIR", configDir)

	geminiDir := filepath.Join(tmpDir, ".gemini")
	_ = os.MkdirAll(geminiDir, 0755)
	cliDir := filepath.Join(geminiDir, "antigravity-cli")
	_ = os.MkdirAll(cliDir, 0755)

	// Helper to create token JSON with mock JWT id_token
	createTokenJSON := func(email string) string {
		claims := `{"email":"` + email + `"}`
		enc := base64.RawURLEncoding.EncodeToString([]byte(claims))
		jwt := "eyJhbGciOiJSUzI1NiJ9." + enc + ".signature"
		return `{"token":{"access_token":"ya29.test","refresh_token":"1//test_ref","id_token":"` + jwt + `"},"auth_method":"consumer"}`
	}

	// 1. Setup CLI account: cli_user@google.com
	_ = os.WriteFile(filepath.Join(cliDir, "antigravity-oauth-token"), []byte(createTokenJSON("cli_user@google.com")), 0600)

	// Since only CLI is configured, CLI should win
	detected := ResolveRunningAntigravityAccount(tmpDir, configDir)
	if detected == nil || detected.Email != "cli_user@google.com" {
		t.Fatalf("expected cli_user@google.com when only CLI is present, got %v", detected)
	}

	// 2. Setup Desktop account: desktop_user@google.com
	_ = os.WriteFile(filepath.Join(geminiDir, "jetski-standalone-oauth-token"), []byte(createTokenJSON("desktop_user@google.com")), 0600)

	// Now Desktop and CLI both exist: Desktop must win (Desktop > VS Code > CLI)
	detected = ResolveRunningAntigravityAccount(tmpDir, configDir)
	if detected == nil || detected.Email != "desktop_user@google.com" {
		t.Fatalf("expected desktop_user@google.com to win over CLI, got %v", detected)
	}

	// 2b. Setup app_storage.json with stale onboarding username: stale_onboarding@google.com
	// Live jetski-standalone-oauth-token must take precedence over stale onboarding app_storage.json
	_ = os.WriteFile(filepath.Join(configDir, "app_storage.json"), []byte(`{"jetski.onboarding.lastLoginUsername":"stale_onboarding@google.com"}`), 0600)
	detected = ResolveRunningAntigravityAccount(tmpDir, configDir)
	if detected == nil || detected.Email != "desktop_user@google.com" {
		t.Fatalf("expected desktop_user@google.com from jetski-standalone-oauth-token to take precedence over stale app_storage.json, got %v", detected)
	}

	// 2c. When standalone token is absent, app_storage.json acts as fallback for Desktop surface
	_ = os.Remove(filepath.Join(geminiDir, "jetski-standalone-oauth-token"))
	detected = ResolveRunningAntigravityAccount(tmpDir, configDir)
	if detected == nil || detected.Email != "stale_onboarding@google.com" {
		t.Fatalf("expected stale_onboarding@google.com from app_storage.json as fallback when standalone token is absent, got %v", detected)
	}
	_ = os.Remove(filepath.Join(configDir, "app_storage.json"))
	_ = os.WriteFile(filepath.Join(geminiDir, "jetski-standalone-oauth-token"), []byte(createTokenJSON("desktop_user@google.com")), 0600)

	// 3. Test Store Reconcile with unimported account and autoImport=false
	accPath := filepath.Join(tmpDir, "accounts.json")
	store, err := NewStore(accPath)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.homeDir = tmpDir
	store.antigravityConfigDir = configDir
	// Add an unrelated account
	_ = store.AddOrUpdateAccount(&Account{Email: "vault_account@google.com", Label: "Vault Account"})

	// Reconcile with autoImport=false: since desktop_user is NOT in vault, vault's active account is preserved
	reconciled, err := store.ReconcileActiveAccount(false, []string{"vault_account@google.com"}, nil)
	if err != nil {
		t.Fatalf("ReconcileActiveAccount error: %v", err)
	}
	if reconciled == nil || reconciled.Email != "vault_account@google.com" {
		t.Errorf("expected vault_account@google.com to be preserved when autoImport=false, got %v", reconciled)
	}
	if store.ActiveAccount() != "vault_account@google.com" {
		t.Errorf("expected active account to remain vault_account@google.com when autoImport=false, got %s", store.ActiveAccount())
	}

	// 4. Test Store Reconcile with autoImport=true:
	// desktop_user is NOT in vault, so it must be auto-imported as STANDBY, preserving vault_account as active!
	reconciled, err = store.ReconcileActiveAccount(true, []string{"vault_account@google.com"}, nil)
	if err != nil {
		t.Fatalf("ReconcileActiveAccount with autoImport=true error: %v", err)
	}
	if reconciled == nil || reconciled.Email != "vault_account@google.com" {
		t.Fatalf("expected vault_account@google.com to be preserved as active, got %v", reconciled)
	}
	if store.ActiveAccount() != "vault_account@google.com" {
		t.Errorf("expected active account to remain vault_account@google.com, got %s", store.ActiveAccount())
	}
	target, _ := store.GetAccount("desktop_user@google.com")
	if target == nil {
		t.Fatalf("expected desktop_user@google.com to be auto-imported into vault")
	}
	if target.IsActive {
		t.Errorf("expected auto-imported account to NOT be active")
	}
	if target.Status != "STANDBY" {
		t.Errorf("expected auto-imported account to have status STANDBY, got %s", target.Status)
	}

	// 5. Test Store Reconcile when account is already in vault
	// Switch active in vault back to vault_account
	_ = store.SetActiveAccount("vault_account@google.com")
	// Clear the 3-second manual switch latch cooldown so background reconciliation takes effect
	store.lastManualSwitchTime = time.Time{}
	// Now reconcile: running session is still desktop_user@google.com, which is now in vault.
	// It should automatically switch back to desktop_user@google.com!
	reconciled, err = store.ReconcileActiveAccount(false, []string{"vault_account@google.com", "desktop_user@google.com"}, nil)
	if err != nil {
		t.Fatalf("Reconcile error: %v", err)
	}
	if reconciled == nil || reconciled.Email != "desktop_user@google.com" {
		t.Errorf("expected desktop_user@google.com to be reconciled as active")
	}
	if store.ActiveAccount() != "desktop_user@google.com" {
		t.Errorf("expected active account to align with running session")
	}
}

func TestUpdateAccountFull_AllowClaudeGPTAndNewAccount(t *testing.T) {
	tmpDir := t.TempDir()
	accPath := filepath.Join(tmpDir, "accounts.json")
	store, err := NewStore(accPath)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	// 1. Create a brand new account via UpdateAccountFull
	err = store.UpdateAccountFull("new_user@google.com", "New User Alias", "Pro", "ACTIVE", "High", "Notes here", "pass123", "", "", 100, true, true, true)
	if err != nil {
		t.Fatalf("UpdateAccountFull error creating new account: %v", err)
	}

	acc, err := store.GetAccount("new_user@google.com")
	if err != nil {
		t.Fatalf("GetAccount error: %v", err)
	}
	if acc.Label != "New User Alias" {
		t.Errorf("expected label 'New User Alias', got %s", acc.Label)
	}
	if !acc.AllowClaudeGPT {
		t.Errorf("expected AllowClaudeGPT to be true")
	}
	if !acc.EnableCreditOverages {
		t.Errorf("expected EnableCreditOverages to be true")
	}

	// 2. Reload store from disk to ensure AllowClaudeGPT persisted
	storeReloaded, err := NewStore(accPath)
	if err != nil {
		t.Fatalf("NewStore reload error: %v", err)
	}
	accReloaded, err := storeReloaded.GetAccount("new_user@google.com")
	if err != nil {
		t.Fatalf("GetAccount after reload error: %v", err)
	}
	if !accReloaded.AllowClaudeGPT {
		t.Errorf("expected AllowClaudeGPT to persist as true after reload")
	}

	// 3. Toggle AllowClaudeGPT to false
	err = storeReloaded.UpdateAccountFull("new_user@google.com", "New User Alias", "Pro", "ACTIVE", "High", "Notes here", "pass123", "", "", 100, true, false, false)
	if err != nil {
		t.Fatalf("UpdateAccountFull error toggling AllowClaudeGPT: %v", err)
	}
	accToggled, _ := storeReloaded.GetAccount("new_user@google.com")
	if accToggled.AllowClaudeGPT {
		t.Errorf("expected AllowClaudeGPT to be false after toggle")
	}

	// 4. Update with empty planTier and 0 credits: should preserve Pro tier and 100 credits
	err = storeReloaded.UpdateAccountFull("new_user@google.com", "Updated Alias", "", "ACTIVE", "High", "Notes", "pass123", "", "", 0, true, false, false)
	if err != nil {
		t.Fatalf("UpdateAccountFull error: %v", err)
	}
	accPreserved, _ := storeReloaded.GetAccount("new_user@google.com")
	if accPreserved.PlanTier != "Pro" {
		t.Errorf("expected PlanTier to remain 'Pro', got %s", accPreserved.PlanTier)
	}
	if accPreserved.Credits != 100 {
		t.Errorf("expected Credits to remain 100, got %f", accPreserved.Credits)
	}

	// 5. Update with "Free" planTier and 0 credits: should STILL preserve Pro tier and 100 credits
	err = storeReloaded.UpdateAccountFull("new_user@google.com", "Updated Alias", "Free", "ACTIVE", "High", "Notes", "pass123", "", "", 0, true, false, false)
	if err != nil {
		t.Fatalf("UpdateAccountFull error: %v", err)
	}
	accPreservedFree, _ := storeReloaded.GetAccount("new_user@google.com")
	if accPreservedFree.PlanTier != "Pro" {
		t.Errorf("expected PlanTier to remain 'Pro' when passing Free, got %s", accPreservedFree.PlanTier)
	}
	if accPreservedFree.Credits != 100 {
		t.Errorf("expected Credits to remain 100 when passing 0, got %f", accPreservedFree.Credits)
	}

	// 6. Explicit upgrade to Enterprise with new credits should update both
	err = storeReloaded.UpdateAccountFull("new_user@google.com", "Updated Alias", "Enterprise", "ACTIVE", "High", "Notes", "pass123", "", "", 250, true, false, false)
	if err != nil {
		t.Fatalf("UpdateAccountFull error: %v", err)
	}
	accEnterprise, _ := storeReloaded.GetAccount("new_user@google.com")
	if accEnterprise.PlanTier != "Enterprise" {
		t.Errorf("expected PlanTier to update to Enterprise, got %s", accEnterprise.PlanTier)
	}
	if accEnterprise.Credits != 250 {
		t.Errorf("expected Credits to update to 250, got %f", accEnterprise.Credits)
	}

	// 7. Creating an account with empty label defaults label to email
	err = storeReloaded.UpdateAccountFull("no_alias@google.com", "", "Pro", "ACTIVE", "High", "", "", "", "", 0, false, false, false)
	if err != nil {
		t.Fatalf("UpdateAccountFull error creating account with empty label: %v", err)
	}
	accNoAlias, err := storeReloaded.GetAccount("no_alias@google.com")
	if err != nil {
		t.Fatalf("GetAccount for no_alias failed: %v", err)
	}
	if accNoAlias.Label != "no_alias@google.com" {
		t.Errorf("expected Label to default to email 'no_alias@google.com', got '%s'", accNoAlias.Label)
	}
}

func TestBatchImportAndExportAccounts(t *testing.T) {
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	tmpDir := t.TempDir()
	accountsPath := filepath.Join(tmpDir, "accounts.json")

	store, err := NewStore(accountsPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	items := []BatchImportItem{
		{
			ID:        "alice@work.com",
			Password:  "AlicePass2026!",
			MFA:       "JBSWY3DPEHPK3PXP",
			OathToken: "1//oauth_refresh_alice",
			Label:     "Alice Work",
			PlanTier:  "Pro",
			Priority:  "High",
		},
		{
			Email:        "bob@personal.com",
			Password:     "BobPass2026!",
			TOTPSecret:   "HXDMVJECJJWSRB3HWIZR4IFUGFTMXBOZ",
			RefreshToken: "1//oauth_refresh_bob",
			Label:        "Bob Personal",
			PlanTier:     "Free",
		},
	}

	count, err := store.BatchImportAccounts(items)
	if err != nil {
		t.Fatalf("BatchImportAccounts error: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 imported accounts, got %d", count)
	}

	exported := store.ExportAccounts()
	if len(exported) != 2 {
		t.Fatalf("expected 2 exported accounts, got %d", len(exported))
	}

	var alice *AccountExport
	for i := range exported {
		if exported[i].ID == "alice@work.com" {
			alice = &exported[i]
			break
		}
	}
	if alice == nil {
		t.Fatalf("expected to find alice in exported accounts")
	}
	if alice.Password != "AlicePass2026!" {
		t.Errorf("expected password to match, got %s", alice.Password)
	}
	if alice.MFA != "JBSWY3DPEHPK3PXP" || alice.TOTPSecret != "JBSWY3DPEHPK3PXP" {
		t.Errorf("expected MFA/TOTP to match, got %s", alice.MFA)
	}
	if alice.OathToken != "1//oauth_refresh_alice" || alice.RefreshToken != "1//oauth_refresh_alice" {
		t.Errorf("expected OathToken/RefreshToken to match, got %s", alice.OathToken)
	}

	// Verify reload from disk
	reloadedStore, err := NewStore(accountsPath)
	if err != nil {
		t.Fatalf("failed to reload store: %v", err)
	}
	bob, err := reloadedStore.GetAccount("bob@personal.com")
	if err != nil {
		t.Fatalf("failed to get bob: %v", err)
	}
	if bob.Password != "BobPass2026!" {
		t.Errorf("expected reloaded bob password to match, got %s", bob.Password)
	}
	if bob.TOTPSecret != "HXDMVJECJJWSRB3HWIZR4IFUGFTMXBOZ" {
		t.Errorf("expected reloaded bob TOTPSecret to match, got %s", bob.TOTPSecret)
	}
	if bob.RefreshToken != "1//oauth_refresh_bob" {
		t.Errorf("expected reloaded bob RefreshToken to match, got %s", bob.RefreshToken)
	}
}

func TestCooldownAccountSwitching(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "keyring_cooldown_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "accounts.json")
	store, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// 1. Setup accounts
	err = store.UpdateAccountFull("active@example.com", "Active User", "Pro", "ACTIVE", "High", "", "", "", "", 100, false, false, true)
	if err != nil {
		t.Fatalf("failed to add active account: %v", err)
	}
	err = store.UpdateAccountFull("standby@example.com", "Standby User", "Pro", "STANDBY", "Mid", "", "", "", "", 50, false, false, false)
	if err != nil {
		t.Fatalf("failed to add standby account: %v", err)
	}
	err = store.UpdateAccountFull("cooling@example.com", "Cooling User", "Pro", "COOLDOWN", "Mid", "", "", "", "", 0, false, false, false)
	if err != nil {
		t.Fatalf("failed to add cooling account: %v", err)
	}
	err = store.UpdateAccountFull("banned@example.com", "Banned User", "Free", "BANNED", "Low", "", "", "", "", 0, false, false, false)
	if err != nil {
		t.Fatalf("failed to add banned account: %v", err)
	}

	if store.ActiveAccount() != "active@example.com" {
		t.Fatalf("expected active@example.com to be active, got %s", store.ActiveAccount())
	}

	// 2. SetActiveAccount allows manual switching to COOLDOWN account and promotes it to ACTIVE
	err = store.SetActiveAccount("cooling@example.com")
	if err != nil {
		t.Fatalf("expected manual switch to cooling account to succeed, got: %v", err)
	}
	if store.ActiveAccount() != "cooling@example.com" {
		t.Fatalf("expected cooling@example.com to be active, got: %s", store.ActiveAccount())
	}
	coolingAcc, _ := store.GetAccount("cooling@example.com")
	if !coolingAcc.IsActive || coolingAcc.Status != "ACTIVE" {
		t.Fatalf("expected cooling account to be promoted to ACTIVE, got: %v", coolingAcc)
	}

	// 3. SetActiveAccount rejects switching to BANNED
	err = store.SetActiveAccount("banned@example.com")
	if err == nil {
		t.Fatalf("expected error switching to BANNED account, got nil")
	}
	if !strings.Contains(err.Error(), "is banned and cannot be switched on") {
		t.Fatalf("expected banned rejection message, got %q", err.Error())
	}

	// 4. Switch back to active@example.com
	_ = store.SetActiveAccount("active@example.com")

	// 5. Transition to STANDBY allows switching on
	err = store.UpdateAccountStatus("cooling@example.com", "STANDBY")
	if err != nil {
		t.Fatalf("UpdateAccountStatus to STANDBY failed: %v", err)
	}
	coolingAcc, err = store.GetAccount("cooling@example.com")
	if err != nil || coolingAcc.Status != "STANDBY" {
		t.Fatalf("expected status STANDBY, got %v, err=%v", coolingAcc, err)
	}

	err = store.SetActiveAccount("cooling@example.com")
	if err != nil {
		t.Fatalf("expected successful switch to reset account, got error: %v", err)
	}
	if store.ActiveAccount() != "cooling@example.com" {
		t.Fatalf("expected active account to be cooling@example.com, got %s", store.ActiveAccount())
	}
	coolingAcc, _ = store.GetAccount("cooling@example.com")
	if !coolingAcc.IsActive || coolingAcc.Status != "ACTIVE" {
		t.Fatalf("expected active cooling account to have IsActive=true and Status=ACTIVE, got %v", coolingAcc)
	}
	oldActive, _ := store.GetAccount("active@example.com")
	if oldActive.IsActive || oldActive.Status != "STANDBY" {
		t.Fatalf("expected old active account to be demoted to STANDBY, got %v", oldActive)
	}

	// 6. Transitioning active account to COOLDOWN deactivates it cleanly
	err = store.UpdateAccountStatus("cooling@example.com", "COOLDOWN")
	if err != nil {
		t.Fatalf("UpdateAccountStatus to COOLDOWN failed: %v", err)
	}
	coolingAcc, _ = store.GetAccount("cooling@example.com")
	if coolingAcc.IsActive || coolingAcc.Status != "COOLDOWN" {
		t.Fatalf("expected cooling account to be deactivated, got is_active=%v, status=%s", coolingAcc.IsActive, coolingAcc.Status)
	}
	if store.ActiveAccount() != "" {
		t.Fatalf("expected empty activeAccount when active account enters cooldown, got %s", store.ActiveAccount())
	}

	// 7. Creating new account with status COOLDOWN when activeEmail is empty does not auto-activate
	err = store.UpdateAccountFull("new_cold@example.com", "Cold", "Free", "COOLDOWN", "Low", "", "", "", "", 0, false, false, false)
	if err != nil {
		t.Fatalf("failed to add new_cold account: %v", err)
	}
	coldAcc, _ := store.GetAccount("new_cold@example.com")
	if coldAcc.IsActive {
		t.Fatalf("expected new COOLDOWN account not to be active, got IsActive=true")
	}
	if store.ActiveAccount() != "" {
		t.Fatalf("expected activeEmail to remain empty, got %s", store.ActiveAccount())
	}

	// 8. Store reload from disk retains correct state
	reloadedStore, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to reload store from disk: %v", err)
	}
	reloadedCold, _ := reloadedStore.GetAccount("new_cold@example.com")
	if reloadedCold.Status != "COOLDOWN" || reloadedCold.IsActive {
		t.Fatalf("expected reloaded status COOLDOWN, got status=%s, is_active=%v", reloadedCold.Status, reloadedCold.IsActive)
	}
}

func TestSyncStateVscdb(t *testing.T) {
	// 1. Test buildUserStatusSentinel exact protobuf wire format
	expectedSentinel := "ClMKFXVzZXJTdGF0dXNTZW50aW5lbEtleRI6CjhHaEp3Y25kb0xtUndiRUJuYldGcGJDNWpiMjA2RW5CeWQyZ3VaSEJzUUdkdFlXbHNMbU52YlE9PQ=="
	actualSentinel := buildUserStatusSentinel("prwh.dpl@gmail.com")
	if actualSentinel != expectedSentinel {
		t.Fatalf("buildUserStatusSentinel mismatch:\nexpected: %s\ngot:      %s", expectedSentinel, actualSentinel)
	}

	// 2. Test SyncStateVscdb against an isolated SQLite state.vscdb
	tmpDir, err := os.MkdirTemp("", "swiss_test_vscdb_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	origEnv := os.Getenv("ANTIGRAVITY_HOST_CONFIG_DIR")
	defer os.Setenv("ANTIGRAVITY_HOST_CONFIG_DIR", origEnv)
	os.Setenv("ANTIGRAVITY_HOST_CONFIG_DIR", tmpDir)

	vscdbDir := filepath.Join(tmpDir, "User", "globalStorage")
	if err := os.MkdirAll(vscdbDir, 0755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(vscdbDir, "state.vscdb")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB);
		INSERT INTO ItemTable(key, value) VALUES('antigravityUnifiedStateSync.userStatus', 'old_status');
		INSERT INTO ItemTable(key, value) VALUES('antigravity.profileUrl', 'https://old.example.com/avatar.png');
	`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Mint token with picture claim
	claimsWithPic := fmt.Sprintf(`{"email":"switched_user@gmail.com","picture":"https://lh3.googleusercontent.com/a/test_avatar_123"}`)
	idTokenWithPic := fmt.Sprintf("header.%s.sig", base64.RawURLEncoding.EncodeToString([]byte(claimsWithPic)))

	testAcc := &Account{
		Email:        "switched_user@gmail.com",
		AccessToken:  "ya29.test_sync_token",
		RefreshToken: "1//test_sync_refresh",
		IDToken:      idTokenWithPic,
	}

	if err := SyncStateVscdb(testAcc); err != nil {
		t.Fatalf("SyncStateVscdb failed: %v", err)
	}

	// Verify ItemTable was updated
	checkDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer checkDB.Close()

	var valStatus string
	err = checkDB.QueryRow("SELECT value FROM ItemTable WHERE key='antigravityUnifiedStateSync.userStatus'").Scan(&valStatus)
	if err != nil {
		t.Fatalf("failed to query updated userStatus: %v", err)
	}
	expectedUpdated := buildUserStatusSentinel("switched_user@gmail.com")
	if valStatus != expectedUpdated {
		t.Errorf("userStatus not updated correctly:\nexpected: %s\ngot:      %s", expectedUpdated, valStatus)
	}

	var valPic string
	err = checkDB.QueryRow("SELECT value FROM ItemTable WHERE key='antigravity.profileUrl'").Scan(&valPic)
	if err != nil {
		t.Fatalf("failed to query updated profileUrl: %v", err)
	}
	if valPic != "https://lh3.googleusercontent.com/a/test_avatar_123" {
		t.Errorf("profileUrl not updated correctly:\nexpected: https://lh3.googleusercontent.com/a/test_avatar_123\ngot:      %s", valPic)
	}

	var valOAuth string
	err = checkDB.QueryRow("SELECT value FROM ItemTable WHERE key='antigravityUnifiedStateSync.oauthToken'").Scan(&valOAuth)
	if err != nil {
		t.Fatalf("failed to query updated oauthToken: %v", err)
	}
	if valOAuth == "" {
		t.Errorf("expected non-empty oauthToken in state.vscdb ItemTable")
	}
}

func TestUpdateAccountFull_RefreshTokenInvalidatesStaleAccessToken(t *testing.T) {
	tmpDir := t.TempDir()
	accPath := filepath.Join(tmpDir, "accounts.json")
	store, err := NewStore(accPath)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	// 1. Import account with both an access token and refresh token
	acc, err := store.ImportAccount("test_token_invalidation@google.com", "1//initial_rt", "ya29.initial_at", "Test Token User", "")
	if err != nil {
		t.Fatalf("ImportAccount error: %v", err)
	}
	if acc.AccessToken != "ya29.initial_at" || acc.RefreshToken != "1//initial_rt" {
		t.Fatalf("expected initial tokens, got AT=%s RT=%s", acc.AccessToken, acc.RefreshToken)
	}

	// 2. Updating with the same refresh token keeps the cached access token
	err = store.UpdateAccountFull("test_token_invalidation@google.com", "Updated Alias", "Pro", "ACTIVE", "High", "Notes", "", "", "1//initial_rt", 0, false, false, false)
	if err != nil {
		t.Fatalf("UpdateAccountFull error: %v", err)
	}
	accUpdated, err := store.GetAccount("test_token_invalidation@google.com")
	if err != nil {
		t.Fatalf("GetAccount error: %v", err)
	}
	if accUpdated.AccessToken != "ya29.initial_at" {
		t.Errorf("expected cached AccessToken to remain untouched when RefreshToken is identical, got: %s", accUpdated.AccessToken)
	}

	// 3. Updating with a new refresh token invalidates the stale access token
	err = store.UpdateAccountFull("test_token_invalidation@google.com", "Updated Alias", "Pro", "ACTIVE", "High", "Notes", "", "", "1//new_rotated_rt", 0, false, false, false)
	if err != nil {
		t.Fatalf("UpdateAccountFull error: %v", err)
	}
	accRotated, err := store.GetAccount("test_token_invalidation@google.com")
	if err != nil {
		t.Fatalf("GetAccount error: %v", err)
	}
	if accRotated.RefreshToken != "1//new_rotated_rt" {
		t.Errorf("expected new RefreshToken, got: %s", accRotated.RefreshToken)
	}
	if accRotated.AccessToken != "" {
		t.Errorf("expected cached AccessToken to be invalidated to empty string on new RefreshToken, got: %s", accRotated.AccessToken)
	}
}

func TestUpdateAccountFull_Ya29AccessTokenPreservesRefreshToken(t *testing.T) {
	tmpDir := t.TempDir()
	accPath := filepath.Join(tmpDir, "accounts.json")
	store, err := NewStore(accPath)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	// 1. Create account with an existing long-term refresh token
	acc, err := store.ImportAccount("test_ya29@google.com", "1//persistent_rt", "", "User", "")
	if err != nil {
		t.Fatalf("ImportAccount error: %v", err)
	}
	if acc.RefreshToken != "1//persistent_rt" {
		t.Fatalf("expected RT=1//persistent_rt, got %s", acc.RefreshToken)
	}

	// 2. Updating with a ya29 token populates AccessToken and does NOT overwrite persistent RefreshToken
	err = store.UpdateAccountFull("test_ya29@google.com", "User", "Pro", "ACTIVE", "High", "", "", "", "ya29.ephemeral_at", 0, false, false, false)
	if err != nil {
		t.Fatalf("UpdateAccountFull error: %v", err)
	}
	accUpdated, err := store.GetAccount("test_ya29@google.com")
	if err != nil {
		t.Fatalf("GetAccount error: %v", err)
	}
	if accUpdated.AccessToken != "ya29.ephemeral_at" {
		t.Errorf("expected AccessToken to be updated to ya29.ephemeral_at, got: %s", accUpdated.AccessToken)
	}
	if accUpdated.RefreshToken != "1//persistent_rt" {
		t.Errorf("expected RefreshToken to remain intact as 1//persistent_rt, got: %s", accUpdated.RefreshToken)
	}

	// 3. SetAccessToken direct method works
	err = store.SetAccessToken("test_ya29@google.com", "ya29.direct_set_at")
	if err != nil {
		t.Fatalf("SetAccessToken error: %v", err)
	}
	accDirect, err := store.GetAccount("test_ya29@google.com")
	if err != nil {
		t.Fatalf("GetAccount error: %v", err)
	}
	if accDirect.AccessToken != "ya29.direct_set_at" {
		t.Errorf("expected AccessToken to be ya29.direct_set_at, got: %s", accDirect.AccessToken)
	}
}

func TestSyncHardwareProfileToDirs_AllFourSurfaces(t *testing.T) {
	tmpDir := t.TempDir()
	agConfigDir := filepath.Join(tmpDir, ".config", "Antigravity")
	geminiAgDir := filepath.Join(tmpDir, ".gemini", "antigravity")
	if err := os.MkdirAll(geminiAgDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Pre-populate antigravity_state.pbtxt with existing onboarding/model settings
	initialState := `agent_onboarding_completed: AGENT_ONBOARDING_STATE_COMPLETED
last_selected_agent_model: MODEL_PLACEHOLDER_M318
installation_uuid: "old-uuid-0000-4000-8000-000000000000"
migrate_convos_into_projects: MIGRATION_STATUS_COMPLETED
`
	statePath := filepath.Join(geminiAgDir, "antigravity_state.pbtxt")
	if err := os.WriteFile(statePath, []byte(initialState), 0644); err != nil {
		t.Fatal(err)
	}

	prof, err := fingerprint.GenerateRandom()
	if err != nil {
		t.Fatal(err)
	}

	if err := SyncHardwareProfileToDirs(prof, agConfigDir, geminiAgDir); err != nil {
		t.Fatalf("SyncHardwareProfileToDirs error: %v", err)
	}

	// 1. Verify machineid
	mBytes, err := os.ReadFile(filepath.Join(agConfigDir, "machineid"))
	if err != nil || string(mBytes) != prof.MachineID {
		t.Errorf("expected machineid=%s, got %q (err=%v)", prof.MachineID, string(mBytes), err)
	}

	// 2. Verify .updaterId
	uBytes, err := os.ReadFile(filepath.Join(agConfigDir, ".updaterId"))
	if err != nil || string(uBytes) != prof.UpdaterID {
		t.Errorf("expected .updaterId=%s, got %q (err=%v)", prof.UpdaterID, string(uBytes), err)
	}

	// 3. Verify installation_id
	iBytes, err := os.ReadFile(filepath.Join(geminiAgDir, "installation_id"))
	if err != nil || strings.TrimSpace(string(iBytes)) != prof.InstallationID {
		t.Errorf("expected installation_id=%s, got %q (err=%v)", prof.InstallationID, string(iBytes), err)
	}

	// 4. Verify antigravity_state.pbtxt updated installation_uuid while preserving onboarding state
	sBytes, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("failed to read statePath: %v", err)
	}
	sContent := string(sBytes)
	expectedLine := `installation_uuid: "` + prof.InstallationUUID + `"`
	if !strings.Contains(sContent, expectedLine) {
		t.Errorf("expected state file to contain %s, got:\n%s", expectedLine, sContent)
	}
	if !strings.Contains(sContent, "last_selected_agent_model: MODEL_PLACEHOLDER_M318") {
		t.Errorf("expected state file to preserve other lines, got:\n%s", sContent)
	}
}

func TestEnsureFreshAccessToken_RefreshesExpiredOrUnknownExpiry(t *testing.T) {
	refreshCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"ya29.fresh_switched_token","expires_in":3600}`))
	}))
	defer srv.Close()

	origEndpoint := tokenRefreshEndpoint
	tokenRefreshEndpoint = srv.URL
	defer func() { tokenRefreshEndpoint = origEndpoint }()

	// 1. Account with stale access_token and zero TokenExpiry must refresh immediately
	acc := &Account{
		Email:        "switch_user@google.com",
		AccessToken:  "ya29.stale_expired_token",
		RefreshToken: "1//valid_refresh_token",
	}
	if !EnsureFreshAccessToken(acc) {
		t.Fatalf("expected EnsureFreshAccessToken to refresh when TokenExpiry is zero")
	}
	if acc.AccessToken != "ya29.fresh_switched_token" {
		t.Fatalf("expected AccessToken=ya29.fresh_switched_token, got %s", acc.AccessToken)
	}
	if time.Until(acc.TokenExpiry) < 50*time.Minute {
		t.Fatalf("expected TokenExpiry >= 50m in future, got %v", time.Until(acc.TokenExpiry))
	}
	if refreshCalls != 1 {
		t.Fatalf("expected 1 refresh call, got %d", refreshCalls)
	}

	// 2. Subsequent call while TokenExpiry is >5m in future must NOT refresh again
	if EnsureFreshAccessToken(acc) {
		t.Fatalf("expected EnsureFreshAccessToken to return false when token is still fresh")
	}
	if refreshCalls != 1 {
		t.Fatalf("expected refreshCalls to remain 1, got %d", refreshCalls)
	}

	// 3. When AccessToken is valid, buildSecretPayload and SyncOAuthCredsJSON must ALWAYS
	// produce a valid future timestamp (so VS Code Antigravity Extension and AGY CLI do not discard it as expired).
	validAcc := &Account{
		Email:        "offline_user@google.com",
		AccessToken:  "ya29.possibly_stale",
		RefreshToken: "1//offline_refresh",
	}
	payload, err := buildSecretPayload(validAcc)
	if err != nil {
		t.Fatalf("buildSecretPayload error: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		t.Fatalf("failed to unmarshal secret payload: %v", err)
	}
	tokObj, _ := parsed["token"].(map[string]interface{})
	expStr, _ := tokObj["expiry"].(string)
	expTime, err := time.Parse(time.RFC3339, expStr)
	if err != nil {
		t.Fatalf("failed to parse token.expiry %q: %v", expStr, err)
	}
	if !expTime.After(time.Now()) {
		t.Errorf("expected valid access token to have future expiry for VS Code extension compatibility, got %v", expTime)
	}

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	if err := SyncOAuthCredsJSON(validAcc); err != nil {
		t.Fatalf("SyncOAuthCredsJSON error: %v", err)
	}
	credsRaw, err := os.ReadFile(filepath.Join(tmpHome, ".gemini", "oauth_creds.json"))
	if err != nil {
		t.Fatalf("failed to read oauth_creds.json: %v", err)
	}
	var credsMap map[string]interface{}
	if err := json.Unmarshal(credsRaw, &credsMap); err != nil {
		t.Fatalf("failed to unmarshal oauth_creds.json: %v", err)
	}
	expiryMs, _ := credsMap["expiry_date"].(float64)
	if int64(expiryMs) <= time.Now().UnixMilli() {
		t.Errorf("expected oauth_creds.json expiry_date to be in the future when AccessToken is present, got %v", int64(expiryMs))
	}

	// 4. When AccessToken is empty but RefreshToken is present, expiry must be in the past.
	emptyAtAcc := &Account{
		Email:        "refresh_only@google.com",
		AccessToken:  "",
		RefreshToken: "1//refresh_only",
	}
	pEmpty, err := buildSecretPayload(emptyAtAcc)
	if err != nil {
		t.Fatalf("buildSecretPayload error: %v", err)
	}
	var parsedEmpty map[string]interface{}
	if err := json.Unmarshal([]byte(pEmpty), &parsedEmpty); err != nil {
		t.Fatalf("failed to unmarshal secret payload: %v", err)
	}
	tokEmpty, _ := parsedEmpty["token"].(map[string]interface{})
	expEmptyStr, _ := tokEmpty["expiry"].(string)
	expEmptyTime, err := time.Parse(time.RFC3339, expEmptyStr)
	if err != nil {
		t.Fatalf("failed to parse token.expiry %q: %v", expEmptyStr, err)
	}
	if !expEmptyTime.Before(time.Now()) {
		t.Errorf("expected empty access token with refresh_token to have past expiry, got %v", expEmptyTime)
	}
}

func TestTokenExpiryPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	accPath := filepath.Join(tmpDir, "accounts.json")
	store, err := NewStore(accPath)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	_, err = store.ImportAccount("expiry_test@google.com", "1//rt_persist", "ya29.initial", "Expiry Test", "")
	if err != nil {
		t.Fatalf("ImportAccount error: %v", err)
	}

	expectedExp := time.Now().Add(45 * time.Minute).UTC().Truncate(time.Second)
	if err := store.UpdateAccountTokensWithExpiry("expiry_test@google.com", "ya29.refreshed", "1//rt_persist", expectedExp); err != nil {
		t.Fatalf("UpdateAccountTokensWithExpiry error: %v", err)
	}

	reloaded, err := NewStore(accPath)
	if err != nil {
		t.Fatalf("NewStore reload error: %v", err)
	}
	acc, err := reloaded.GetAccount("expiry_test@google.com")
	if err != nil {
		t.Fatalf("GetAccount error: %v", err)
	}
	if acc.AccessToken != "ya29.refreshed" {
		t.Errorf("expected AccessToken=ya29.refreshed, got %s", acc.AccessToken)
	}
	if !acc.TokenExpiry.UTC().Truncate(time.Second).Equal(expectedExp) {
		t.Errorf("expected TokenExpiry=%v, got %v", expectedExp, acc.TokenExpiry)
	}

	// Updating with zero expiry must NOT fabricate a 55-minute future expiry
	if err := reloaded.UpdateAccountTokensWithExpiry("expiry_test@google.com", "ya29.stale", "1//rt_persist", time.Time{}); err != nil {
		t.Fatalf("UpdateAccountTokensWithExpiry zero error: %v", err)
	}
	accZero, err := reloaded.GetAccount("expiry_test@google.com")
	if err != nil {
		t.Fatalf("GetAccount error: %v", err)
	}
	if !accZero.TokenExpiry.IsZero() {
		t.Errorf("expected zero TokenExpiry to be preserved, got %v", accZero.TokenExpiry)
	}
}

func TestEnsureFreshAccessToken_FailedRefreshClearsStaleExpiryAndAppStorageTos(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"temporarily_unavailable"}`))
	}))
	defer srv.Close()

	origEndpoint := tokenRefreshEndpoint
	tokenRefreshEndpoint = srv.URL
	defer func() { tokenRefreshEndpoint = origEndpoint }()

	// Account whose token expires in 2 minutes (<5m threshold) when refresh fails
	acc := &Account{
		Email:        "near_expiry@google.com",
		AccessToken:  "ya29.near_expiry_token",
		RefreshToken: "1//valid_refresh_token",
		TokenExpiry:  time.Now().Add(2 * time.Minute),
	}
	if EnsureFreshAccessToken(acc) {
		t.Fatalf("expected EnsureFreshAccessToken to return false on HTTP 502")
	}
	if !acc.TokenExpiry.IsZero() {
		t.Fatalf("expected failed refresh to clear TokenExpiry so downstream surfaces mark token expired, got %v", acc.TokenExpiry)
	}

	payload, err := buildSecretPayload(acc)
	if err != nil {
		t.Fatalf("buildSecretPayload error: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	tokObj, _ := parsed["token"].(map[string]interface{})
	expStr, _ := tokObj["expiry"].(string)
	expTime, err := time.Parse(time.RFC3339, expStr)
	if err != nil {
		t.Fatalf("parse expiry error: %v", err)
	}
	if !expTime.After(time.Now()) {
		t.Errorf("expected valid access token in secret payload to have future expiry for VS Code extension compatibility, got %v", expTime)
	}

	// Verify SyncAppStorageLoginUser sets jetski.onboarding.lastLoginIsGcpTos
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	cfgDir := filepath.Join(tmpHome, ".config", "Antigravity")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "app_storage.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANTIGRAVITY_CONFIG_DIR", cfgDir)
	if err := SyncAppStorageLoginUser(acc.Email); err != nil {
		t.Fatalf("SyncAppStorageLoginUser error: %v", err)
	}
	rawStorage, err := os.ReadFile(filepath.Join(cfgDir, "app_storage.json"))
	if err != nil {
		t.Fatalf("read app_storage.json error: %v", err)
	}
	var storageMap map[string]interface{}
	if err := json.Unmarshal(rawStorage, &storageMap); err != nil {
		t.Fatalf("unmarshal app_storage.json error: %v", err)
	}
	if storageMap["jetski.onboarding.lastLoginUsername"] != acc.Email {
		t.Errorf("expected lastLoginUsername=%q, got %v", acc.Email, storageMap["jetski.onboarding.lastLoginUsername"])
	}
	if storageMap["jetski.onboarding.lastLoginIsGcpTos"] != "false" {
		t.Errorf("expected lastLoginIsGcpTos=\"false\", got %v", storageMap["jetski.onboarding.lastLoginIsGcpTos"])
	}
}

func TestReconcileActiveAccount_DoesNotWipeActiveAccountWhenUnrecognizedDetected(t *testing.T) {
	tmpDir := t.TempDir()
	accPath := filepath.Join(tmpDir, "accounts.json")
	store, err := NewStore(accPath)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	acc := &Account{
		Email:    "preserved-active@example.com",
		Label:    "Primary Vault Account",
		Status:   "ACTIVE",
		IsActive: true,
	}
	if err := store.AddOrUpdateAccount(acc); err != nil {
		t.Fatalf("AddOrUpdateAccount error: %v", err)
	}
	_ = store.SetActiveAccount("preserved-active@example.com")

	if store.ActiveAccount() != "preserved-active@example.com" {
		t.Fatalf("expected active account to be preserved-active@example.com, got %s", store.ActiveAccount())
	}

	// Mock environment where Antigravity detects an external unimported session
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	antigravityDir := filepath.Join(tmpHome, ".config", "Antigravity")
	_ = os.MkdirAll(antigravityDir, 0755)
	_ = os.WriteFile(filepath.Join(antigravityDir, "app_storage.json"), []byte(`{"jetski.onboarding.lastLoginUsername":"external-unimported@gmail.com"}`), 0644)
	t.Setenv("ANTIGRAVITY_CONFIG_DIR", antigravityDir)

	// Reconcile with autoImport = false
	allEmails := []string{"preserved-active@example.com"}
	reconciled, err := store.ReconcileActiveAccount(false, allEmails, nil)
	if err != nil {
		t.Fatalf("ReconcileActiveAccount returned error: %v", err)
	}

	// Crucial assertion: Active account in vault must NOT be wiped!
	if store.ActiveAccount() != "preserved-active@example.com" {
		t.Errorf("CRITICAL BUG: active account was wiped or changed! Expected 'preserved-active@example.com', got %q", store.ActiveAccount())
	}
	storedAcc, _ := store.GetAccount("preserved-active@example.com")
	if storedAcc == nil || !storedAcc.IsActive {
		t.Errorf("CRITICAL BUG: active account in store is nil or IsActive=false!")
	}
	if reconciled != nil && reconciled.Email != "preserved-active@example.com" {
		t.Errorf("expected reconciled account to be preserved-active@example.com, got %v", reconciled)
	}
}

func TestReconcileActiveAccount_AutoImportDoesNotSwitchActiveAccount(t *testing.T) {
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "accounts.json"))
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	primary := &Account{
		Email:    "alice@domain.com",
		Label:    "Alice Active",
		Status:   "ACTIVE",
		IsActive: true,
	}
	standby := &Account{
		Email:    "bob@domain.com",
		Label:    "Bob Standby",
		Status:   "STANDBY",
		IsActive: false,
	}
	_ = store.AddOrUpdateAccount(primary)
	_ = store.AddOrUpdateAccount(standby)
	_ = store.SetActiveAccount("alice@domain.com")
	store.lastManualSwitchTime = time.Time{}

	// Mock host surface with charlie
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	antigravityDir := filepath.Join(tmpHome, ".config", "Antigravity")
	_ = os.MkdirAll(antigravityDir, 0755)
	_ = os.WriteFile(filepath.Join(antigravityDir, "app_storage.json"), []byte(`{"jetski.onboarding.lastLoginUsername":"charlie@domain.com"}`), 0644)
	t.Setenv("ANTIGRAVITY_CONFIG_DIR", antigravityDir)
	store.homeDir = tmpHome
	store.antigravityConfigDir = antigravityDir

	reconciled, err := store.ReconcileActiveAccount(true, []string{"alice@domain.com", "bob@domain.com"}, nil)
	if err != nil {
		t.Fatalf("ReconcileActiveAccount error: %v", err)
	}

	// Active account must remain alice!
	if store.ActiveAccount() != "alice@domain.com" {
		t.Errorf("expected active account to remain alice@domain.com, got %s", store.ActiveAccount())
	}
	if reconciled == nil || reconciled.Email != "alice@domain.com" {
		t.Errorf("expected returned reconciled account to be alice@domain.com, got %v", reconciled)
	}

	// Charlie must be added to vault as STANDBY
	charlie, err := store.GetAccount("charlie@domain.com")
	if err != nil || charlie == nil {
		t.Fatalf("expected charlie to be added to vault: %v", err)
	}
	if charlie.IsActive {
		t.Errorf("expected charlie.IsActive to be false, got true")
	}
	if charlie.Status != "STANDBY" {
		t.Errorf("expected charlie.Status to be STANDBY, got %s", charlie.Status)
	}
}

func TestReconcileActiveAccount_EmptyVaultAutoImportsAsActive(t *testing.T) {
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	tmpDir := t.TempDir()
	store, err := NewStore(filepath.Join(tmpDir, "accounts.json"))
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	// Vault is empty: 0 accounts, activeEmail == ""
	if store.ActiveAccount() != "" || len(store.ListAccounts()) != 0 {
		t.Fatalf("expected empty vault")
	}

	// Mock host surface with charlie
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	antigravityDir := filepath.Join(tmpHome, ".config", "Antigravity")
	_ = os.MkdirAll(antigravityDir, 0755)
	_ = os.WriteFile(filepath.Join(antigravityDir, "app_storage.json"), []byte(`{"jetski.onboarding.lastLoginUsername":"charlie@domain.com"}`), 0644)
	t.Setenv("ANTIGRAVITY_CONFIG_DIR", antigravityDir)
	store.homeDir = tmpHome
	store.antigravityConfigDir = antigravityDir

	reconciled, err := store.ReconcileActiveAccount(true, []string{}, nil)
	if err != nil {
		t.Fatalf("ReconcileActiveAccount error: %v", err)
	}

	if store.ActiveAccount() != "charlie@domain.com" {
		t.Errorf("expected charlie@domain.com to become active in empty vault, got %s", store.ActiveAccount())
	}
	if reconciled == nil || reconciled.Email != "charlie@domain.com" {
		t.Errorf("expected returned reconciled account to be charlie, got %v", reconciled)
	}
	charlie, err := store.GetAccount("charlie@domain.com")
	if err != nil || charlie == nil {
		t.Fatalf("expected charlie in vault: %v", err)
	}
	if !charlie.IsActive {
		t.Errorf("expected charlie.IsActive to be true")
	}
	if charlie.Status != "ACTIVE" {
		t.Errorf("expected charlie.Status to be ACTIVE, got %s", charlie.Status)
	}
}

func TestAccount_RecoverCredentialsFromNotes(t *testing.T) {
	// 1. Pure JSON with refresh_token and access_token
	acc1 := &Account{
		Email:        "user1@google.com",
		Status:       "ERROR",
		ErrorMessage: "Missing credentials / re-authentication required",
		Notes:        `{"refresh_token": "1//rt_111", "access_token": "ya29.at_111"}`,
	}
	if !acc1.RecoverCredentialsFromNotes() {
		t.Errorf("expected credentials to be recovered for acc1")
	}
	if acc1.RefreshToken != "1//rt_111" {
		t.Errorf("expected RefreshToken 1//rt_111, got %s", acc1.RefreshToken)
	}
	if acc1.AccessToken != "ya29.at_111" {
		t.Errorf("expected AccessToken ya29.at_111, got %s", acc1.AccessToken)
	}
	if acc1.Credential == nil || acc1.Credential.RefreshToken != "1//rt_111" {
		t.Errorf("expected acc1.Credential.RefreshToken to be populated")
	}
	if acc1.ErrorMessage != "" {
		t.Errorf("expected ErrorMessage to be cleared, got %s", acc1.ErrorMessage)
	}
	if acc1.Status != "STANDBY" {
		t.Errorf("expected Status to be restored to STANDBY, got %s", acc1.Status)
	}

	// 2. JSON with "token" instead of "access_token"
	acc2 := &Account{
		Email:        "user2@google.com",
		Status:       "ERROR",
		ErrorMessage: "Missing credentials / re-authentication required",
		Notes:        `{"refresh_token": "1//rt_222", "token": "ya29.at_222"}`,
	}
	if !acc2.RecoverCredentialsFromNotes() {
		t.Errorf("expected credentials to be recovered for acc2")
	}
	if acc2.RefreshToken != "1//rt_222" || acc2.AccessToken != "ya29.at_222" {
		t.Errorf("acc2 tokens mismatch: rf=%s at=%s", acc2.RefreshToken, acc2.AccessToken)
	}

	// 3. Embedded JSON inside text notes with extra surrounding content
	acc3 := &Account{
		Email:        "user3@google.com",
		Status:       "ERROR",
		ErrorMessage: "Missing credentials / re-authentication required",
		Notes:        "Some account notes here.\n{\"client_id\":\"xxx\",\"refresh_token\":\"1//rt_333\",\"access_token\":\"ya29.at_333\"}\nKeep secret!",
	}
	if !acc3.RecoverCredentialsFromNotes() {
		t.Errorf("expected credentials to be recovered for acc3")
	}
	if acc3.RefreshToken != "1//rt_333" || acc3.AccessToken != "ya29.at_333" {
		t.Errorf("acc3 tokens mismatch: rf=%s at=%s", acc3.RefreshToken, acc3.AccessToken)
	}

	// 4. Nested JSON under "credential"
	acc4 := &Account{
		Email: "user4@google.com",
		Notes: `{"credential": {"refresh_token": "1//rt_444", "access_token": "ya29.at_444"}}`,
	}
	if !acc4.RecoverCredentialsFromNotes() {
		t.Errorf("expected credentials to be recovered for acc4")
	}
	if acc4.RefreshToken != "1//rt_444" || acc4.AccessToken != "ya29.at_444" {
		t.Errorf("acc4 tokens mismatch: rf=%s at=%s", acc4.RefreshToken, acc4.AccessToken)
	}

	// 5. Account already has RefreshToken - RecoverCredentialsFromNotes should return false without modifying
	acc5 := &Account{
		Email:        "user5@google.com",
		RefreshToken: "existing_rf",
		Notes:        `{"refresh_token": "new_rf"}`,
	}
	if acc5.RecoverCredentialsFromNotes() {
		t.Errorf("expected RecoverCredentialsFromNotes to return false when RefreshToken is non-empty")
	}
	if acc5.RefreshToken != "existing_rf" {
		t.Errorf("RefreshToken should remain existing_rf, got %s", acc5.RefreshToken)
	}
}

func TestStore_LoadAndSaveRecoverCredentialsFromNotes(t *testing.T) {
	tmpDir := t.TempDir()
	accPath := filepath.Join(tmpDir, "accounts.json")

	// Write accounts.json where an account has empty refresh_token but notes has credentials
	initialJSON := `{
  "version": 1,
  "active_account": "user@google.com",
  "accounts": {
    "user@google.com": {
      "email": "user@google.com",
      "label": "User",
      "status": "ERROR",
      "error_message": "Missing credentials / re-authentication required",
      "notes": "{\"refresh_token\": \"1//recovered_rf_disk\", \"access_token\": \"ya29.recovered_at_disk\"}"
    }
  }
}`
	if err := os.WriteFile(accPath, []byte(initialJSON), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	store, err := NewStore(accPath)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	acc, err := store.GetAccount("user@google.com")
	if err != nil {
		t.Fatalf("GetAccount error: %v", err)
	}
	if acc.RefreshToken != "1//recovered_rf_disk" {
		t.Errorf("expected RefreshToken 1//recovered_rf_disk, got %s", acc.RefreshToken)
	}
	if acc.AccessToken != "ya29.recovered_at_disk" {
		t.Errorf("expected AccessToken ya29.recovered_at_disk, got %s", acc.AccessToken)
	}
	if acc.ErrorMessage != "" {
		t.Errorf("expected ErrorMessage to be cleared, got %s", acc.ErrorMessage)
	}
	if acc.Status != "ACTIVE" {
		t.Errorf("expected Status to be ACTIVE, got %s", acc.Status)
	}

	// Now save the store and inspect the JSON written to disk
	if err := store.save(); err != nil {
		t.Fatalf("save error: %v", err)
	}

	data, err := os.ReadFile(accPath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	var saved struct {
		Accounts map[string]struct {
			IsHealthy  bool `json:"is_healthy"`
			Credential struct {
				RefreshToken string `json:"refresh_token"`
				AccessToken  string `json:"access_token"`
			} `json:"credential"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	savedUser, ok := saved.Accounts["user@google.com"]
	if !ok {
		t.Fatalf("user@google.com not found in saved accounts.json")
	}
	if !savedUser.IsHealthy {
		t.Errorf("expected is_healthy to be true")
	}
	if savedUser.Credential.RefreshToken == "" {
		t.Errorf("expected saved credential.refresh_token to be non-empty")
	}
}



