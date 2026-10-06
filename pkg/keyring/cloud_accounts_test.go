package keyring

import (
	"path/filepath"
	"testing"
)

func TestReadCloudAccountsDB(t *testing.T) {
	accounts, err := ReadCloudAccountsDB("")
	if err != nil {
		t.Fatalf("ReadCloudAccountsDB failed: %v", err)
	}
	t.Logf("Read %d accounts from cloud_accounts.db", len(accounts))
	if len(accounts) == 0 {
		t.Log("No accounts found in cloud_accounts.db (might be empty or missing)")
		return
	}

	for i, a := range accounts {
		t.Logf("Account %d: [MASKED], Plan: %s, Credits: %.0f, 5H: %.2f, Wk: %.2f, HasRefresh: %v", i+1, a.PlanTier, a.Credits, a.Quota5h, a.QuotaWeekly, a.RefreshToken != "")
		if a.Email == "" {
			t.Errorf("Expected account to have non-empty email")
		}
	}
}

func TestNormalizeCloudTier(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Ultra 5X", "Ultra 5X"},
		{"ultra_5x", "Ultra 5X"},
		{"ultra 5x", "Ultra 5X"},
		{"Ultra 10X", "Ultra 10X"},
		{"ultra_10x", "Ultra 10X"},
		{"Ultra 20X", "Ultra 20X"},
		{"ultra_20x", "Ultra 20X"},
		{"ultra", "Ultra 20X"},
		{"Pro - Trial", "Pro - Trial"},
		{"trial", "Pro - Trial"},
		{"student@stanford.edu", "Edu"},
		{"teams_tier_enterprise", "Enterprise"},
		{"Google AI Pro", "Pro"},
		{"standard", "Pro"},
		{"code assist", "Pro"},
		{"free", "Free"},
		{"tier_free", "Free"},
		{"", "Pro"},
	}

	for _, tc := range tests {
		got := normalizeCloudTier(tc.input)
		if got != tc.expected {
			t.Errorf("normalizeCloudTier(%q) = %q, expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestSyncStoreFromCloudAccountsDB_NeverMarksStandbyAccountsActive(t *testing.T) {
	tempDir := t.TempDir()
	accountsPath := filepath.Join(tempDir, "accounts.json")
	store, err := NewStore(accountsPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Import primary account
	acc1, err := store.ImportAccount("primary@gmail.com", "ref1", "acc1", "Primary", "")
	if err != nil {
		t.Fatalf("Failed to import primary: %v", err)
	}
	if !acc1.IsActive || store.ActiveAccount() != "primary@gmail.com" {
		t.Fatalf("Expected primary to be active, got active=%s", store.ActiveAccount())
	}

	// Import secondary standby account
	acc2, err := store.ImportAccount("secondary@gmail.com", "ref2", "acc2", "Secondary", "")
	if err != nil {
		t.Fatalf("Failed to import secondary: %v", err)
	}
	if acc2.IsActive {
		t.Fatalf("Expected secondary to NOT be active")
	}

	// Update details on secondary with setActive=false
	err = store.UpdateAccountDetails("secondary@gmail.com", "Secondary", "Pro", "ACTIVE", "High", "", "", "", "", false)
	if err != nil {
		t.Fatalf("UpdateAccountDetails failed: %v", err)
	}

	// Verify that secondary.IsActive is STILL false and activeEmail is STILL primary@gmail.com
	if store.ActiveAccount() != "primary@gmail.com" {
		t.Errorf("Expected active account to remain primary@gmail.com, got %s", store.ActiveAccount())
	}
	secAcc, _ := store.GetAccount("secondary@gmail.com")
	if secAcc.IsActive {
		t.Errorf("Expected secondary account to have IsActive=false, got true")
	}
	if secAcc.Status != "STANDBY" {
		t.Errorf("Expected secondary account status to be normalized to STANDBY, got %s", secAcc.Status)
	}
}


