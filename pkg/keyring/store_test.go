package keyring

import (
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

