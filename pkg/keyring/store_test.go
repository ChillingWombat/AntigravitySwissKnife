package keyring

import (
	"encoding/base64"
	"os"
	"path/filepath"
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


