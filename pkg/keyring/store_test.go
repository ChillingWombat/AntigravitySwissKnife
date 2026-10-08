package keyring

import (
	"database/sql"
	"encoding/base64"
	"fmt"
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

	// Reconcile with autoImport=false: since desktop_user is NOT in vault, no account must be active!
	reconciled, err := store.ReconcileActiveAccount(false, []string{"vault_account@google.com"}, nil)
	if err != nil {
		t.Fatalf("ReconcileActiveAccount error: %v", err)
	}
	if reconciled != nil {
		t.Errorf("expected nil reconciled account when autoImport=false, got %v", reconciled)
	}
	if store.ActiveAccount() != "" {
		t.Errorf("expected no active account when running account is unimported and autoImport=false, got %s", store.ActiveAccount())
	}
	for _, acc := range store.ListAccounts() {
		if acc.IsActive {
			t.Errorf("expected all vault accounts to be inactive, but %s was active", acc.Email)
		}
	}

	// 4. Test Store Reconcile with autoImport=true
	reconciled, err = store.ReconcileActiveAccount(true, []string{"vault_account@google.com"}, nil)
	if err != nil {
		t.Fatalf("ReconcileActiveAccount with autoImport=true error: %v", err)
	}
	if reconciled == nil || reconciled.Email != "desktop_user@google.com" {
		t.Fatalf("expected desktop_user@google.com to be auto-imported, got %v", reconciled)
	}
	if store.ActiveAccount() != "desktop_user@google.com" {
		t.Errorf("expected desktop_user@google.com to become active account, got %s", store.ActiveAccount())
	}
	target, _ := store.GetAccount("desktop_user@google.com")
	if target == nil || !target.IsActive {
		t.Errorf("expected desktop_user@google.com to be active in vault")
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
		Email:   "switched_user@gmail.com",
		IDToken: idTokenWithPic,
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

