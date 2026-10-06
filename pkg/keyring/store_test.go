package keyring

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	// 2b. Setup app_storage.json with active desktop login user: true_desktop@google.com
	// app_storage.json must take precedence over external jetski-standalone-oauth-token
	_ = os.WriteFile(filepath.Join(configDir, "app_storage.json"), []byte(`{"jetski.onboarding.lastLoginUsername":"true_desktop@google.com"}`), 0600)
	detected = ResolveRunningAntigravityAccount(tmpDir, configDir)
	if detected == nil || detected.Email != "true_desktop@google.com" {
		t.Fatalf("expected true_desktop@google.com from app_storage.json to take precedence, got %v", detected)
	}
	_ = os.Remove(filepath.Join(configDir, "app_storage.json"))

	// 3. Test Store Reconcile with unimported account and autoImport=false
	accPath := filepath.Join(tmpDir, "accounts.json")
	store, err := NewStore(accPath)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
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
			ID:         "alice@work.com",
			Password:   "AlicePass2026!",
			MFA:        "JBSWY3DPEHPK3PXP",
			OathToken:  "1//oauth_refresh_alice",
			Label:      "Alice Work",
			PlanTier:   "Pro",
			Priority:   "High",
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

	// 2. SetActiveAccount rejects switching to COOLDOWN
	err = store.SetActiveAccount("cooling@example.com")
	if err == nil {
		t.Fatalf("expected error switching to COOLDOWN account, got nil")
	}
	expectedCooldownMsg := "account cooling@example.com is in cooldown waiting for quota reset and cannot be switched on"
	if !strings.Contains(err.Error(), expectedCooldownMsg) {
		t.Fatalf("expected error message %q, got %q", expectedCooldownMsg, err.Error())
	}
	if store.ActiveAccount() != "active@example.com" {
		t.Fatalf("active account changed despite switch rejection: %s", store.ActiveAccount())
	}

	// 3. SetActiveAccount rejects switching to BANNED
	err = store.SetActiveAccount("banned@example.com")
	if err == nil {
		t.Fatalf("expected error switching to BANNED account, got nil")
	}
	if !strings.Contains(err.Error(), "is banned and cannot be switched on") {
		t.Fatalf("expected banned rejection message, got %q", err.Error())
	}

	// 4. UpdateAccountFull and UpdateAccountDetails reject setActive on COOLDOWN
	err = store.UpdateAccountFull("cooling@example.com", "Cooling User", "Pro", "COOLDOWN", "Mid", "", "", "", "", 0, false, false, true)
	if err == nil || !strings.Contains(err.Error(), expectedCooldownMsg) {
		t.Fatalf("expected UpdateAccountFull to reject setActive on COOLDOWN, got: %v", err)
	}
	err = store.UpdateAccountDetails("cooling@example.com", "Cooling User", "Pro", "COOLDOWN", "Mid", "", "", "", "", true)
	if err == nil || !strings.Contains(err.Error(), expectedCooldownMsg) {
		t.Fatalf("expected UpdateAccountDetails to reject setActive on COOLDOWN, got: %v", err)
	}

	// 5. Transition to STANDBY allows switching on
	err = store.UpdateAccountStatus("cooling@example.com", "STANDBY")
	if err != nil {
		t.Fatalf("UpdateAccountStatus to STANDBY failed: %v", err)
	}
	coolingAcc, err := store.GetAccount("cooling@example.com")
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


