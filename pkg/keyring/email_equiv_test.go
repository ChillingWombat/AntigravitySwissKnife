package keyring

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmailsEquivalent(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"GersioOlba.p238@gmail.com", "gersioolbap238@gmail.com", true},
		{"gersioolba.p238@googlemail.com", "gersioolbap238@gmail.com", true},
		{"User.Name@Gmail.com", "username@gmail.com", true},
		{"same@example.com", "same@example.com", true},
		{"Same@Example.com", "same@example.com", true},
		{"first.last@outlook.com", "firstlast@outlook.com", false},
		{"user@gmail.com", "user@googlemail.com", true},
		{"user+tag@gmail.com", "user@gmail.com", false},
		{"a@gmail.com", "b@gmail.com", false},
		{"", "someone@gmail.com", false},
		{"broken@", "broken@", true},
	}
	for _, c := range cases {
		if got := EmailsEquivalent(c.a, c.b); got != c.want {
			t.Errorf("EmailsEquivalent(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestImportAccount_GmailDotVariantUpdatesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	accPath := filepath.Join(tmpDir, "accounts.json")
	store, err := NewStore(accPath)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	_, err = store.ImportAccount("GersioOlba.p238@gmail.com", "1//old_refresh", "ya29.old_access", "My Label", "")
	if err != nil {
		t.Fatalf("ImportAccount error: %v", err)
	}

	// Importing the dotless variant must update the existing canonical entry,
	// not create a duplicate account.
	acc, err := store.ImportAccount("gersioolbap238@gmail.com", "1//new_refresh", "", "", "")
	if err != nil {
		t.Fatalf("second ImportAccount error: %v", err)
	}
	if acc.Email != "GersioOlba.p238@gmail.com" {
		t.Errorf("expected import to resolve to canonical email, got %q", acc.Email)
	}
	if acc.RefreshToken != "1//new_refresh" {
		t.Errorf("expected refresh token to be updated on the canonical entry")
	}
	if got := len(store.ListAccounts()); got != 1 {
		t.Errorf("expected exactly 1 account after variant import, got %d", got)
	}
	if dup, _ := store.GetAccount("gersioolbap238@gmail.com"); dup != nil {
		t.Errorf("dotless variant must not exist as a separate account")
	}
}

func TestReconcile_GmailDotVariant_NoDuplicateImport(t *testing.T) {
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	configDir := filepath.Join(tmpDir, ".config", "Antigravity")
	_ = os.MkdirAll(configDir, 0755)
	t.Setenv("ANTIGRAVITY_CONFIG_DIR", configDir)

	geminiDir := filepath.Join(tmpDir, ".gemini")
	_ = os.MkdirAll(geminiDir, 0755)

	// Live desktop token carries the dotless form of the Gmail address.
	createTokenJSON := func(email string) string {
		claims := `{"email":"` + email + `"}`
		enc := base64.RawURLEncoding.EncodeToString([]byte(claims))
		jwt := "eyJhbGciOiJSUzI1NiJ9." + enc + ".signature"
		return `{"token":{"access_token":"ya29.test","refresh_token":"1//test_ref","id_token":"` + jwt + `"},"auth_method":"consumer"}`
	}
	_ = os.WriteFile(filepath.Join(geminiDir, "jetski-standalone-oauth-token"), []byte(createTokenJSON("gersioolbap238@gmail.com")), 0600)

	accPath := filepath.Join(tmpDir, "accounts.json")
	store, err := NewStore(accPath)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	store.homeDir = tmpDir
	store.antigravityConfigDir = configDir

	_, err = store.ImportAccount("GersioOlba.p238@gmail.com", "1//rt", "ya29.at", "", "")
	if err != nil {
		t.Fatalf("ImportAccount error: %v", err)
	}

	// autoImport enabled: an equivalent (dotless) detection must NOT import a duplicate.
	_, err = store.ReconcileActiveAccount(true, []string{"GersioOlba.p238@gmail.com"}, nil)
	if err != nil {
		t.Fatalf("ReconcileActiveAccount error: %v", err)
	}

	accounts := store.ListAccounts()
	if len(accounts) != 1 {
		names := make([]string, 0, len(accounts))
		for _, a := range accounts {
			names = append(names, a.Email)
		}
		t.Fatalf("expected 1 account, got %d: %v", len(accounts), names)
	}
	if got := store.ActiveAccount(); got != "GersioOlba.p238@gmail.com" {
		t.Errorf("expected canonical account to become active, got %q", got)
	}
	if dup, _ := store.GetAccount("gersioolbap238@gmail.com"); dup != nil {
		t.Errorf("dotless variant must not be auto-imported")
	}
}

func TestStoreLoad_DeduplicatesCasingVariants(t *testing.T) {
	tmpDir := t.TempDir()
	accPath := filepath.Join(tmpDir, "accounts.json")

	// Raw JSON map with two identical accounts differing only by casing
	rawJSON := `{
  "accounts": {
    "ChillingWombat@gmail.com": {
      "email": "ChillingWombat@gmail.com",
      "label": "chillingwombat@gmail.com",
      "plan_tier": "Pro - Trial",
      "status": "STANDBY",
      "priority": "Mid",
      "password": "secret_password",
      "credential": {
        "access_token": "ya29.test1",
        "refresh_token": "1//test_ref1"
      }
    },
    "chillingwombat@gmail.com": {
      "email": "chillingwombat@gmail.com",
      "label": "chillingwombat@gmail.com",
      "plan_tier": "Pro - Trial",
      "status": "ACTIVE",
      "priority": "Mid",
      "credential": {
        "access_token": "ya29.test2",
        "refresh_token": "1//test_ref2"
      }
    }
  },
  "active_account": "chillingwombat@gmail.com",
  "version": 1
}`
	if err := os.WriteFile(accPath, []byte(rawJSON), 0600); err != nil {
		t.Fatalf("failed to write raw accounts.json: %v", err)
	}

	store, err := NewStore(accPath)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	accounts := store.ListAccounts()
	if len(accounts) != 1 {
		emails := make([]string, 0, len(accounts))
		for _, a := range accounts {
			emails = append(emails, a.Email)
		}
		t.Fatalf("expected exactly 1 account after deduplication, got %d: %v", len(accounts), emails)
	}

	acc := accounts[0]
	if !acc.IsActive {
		t.Errorf("expected merged account to be active")
	}
	if acc.Password != "secret_password" {
		t.Errorf("expected merged account to retain password")
	}

	// Verify store.ActiveAccount matches canonical email
	if !strings.EqualFold(store.ActiveAccount(), "chillingwombat@gmail.com") {
		t.Errorf("expected active account to match chillingwombat@gmail.com, got %q", store.ActiveAccount())
	}
}

func TestAddOrUpdateAccount_CasingVariantUpdatesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	accPath := filepath.Join(tmpDir, "accounts.json")
	store, err := NewStore(accPath)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	err = store.AddOrUpdateAccount(&Account{
		Email:        "UpperUser@example.com",
		Label:        "Original Label",
		PlanTier:     "Free",
		Status:       "STANDBY",
		RefreshToken: "1//orig_ref",
	})
	if err != nil {
		t.Fatalf("AddOrUpdateAccount error: %v", err)
	}

	// Add casing variant: should update existing without creating duplicate
	err = store.AddOrUpdateAccount(&Account{
		Email:        "upperuser@example.com",
		Label:        "Updated Label",
		PlanTier:     "Pro",
		Status:       "ACTIVE",
		RefreshToken: "1//new_ref",
	})
	if err != nil {
		t.Fatalf("second AddOrUpdateAccount error: %v", err)
	}

	accounts := store.ListAccounts()
	if len(accounts) != 1 {
		t.Fatalf("expected exactly 1 account, got %d", len(accounts))
	}
	if accounts[0].PlanTier != "Pro" {
		t.Errorf("expected updated plan tier Pro, got %q", accounts[0].PlanTier)
	}
}

func TestUpdateAccountFull_CasingVariantUpdatesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	accPath := filepath.Join(tmpDir, "accounts.json")
	store, err := NewStore(accPath)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	err = store.AddOrUpdateAccount(&Account{
		Email:        "MixedCase@example.com",
		Label:        "Original Label",
		PlanTier:     "Free",
		Status:       "STANDBY",
		RefreshToken: "1//orig_ref",
	})
	if err != nil {
		t.Fatalf("AddOrUpdateAccount error: %v", err)
	}

	err = store.UpdateAccountFull("mixedcase@example.com", "New Label", "Pro", "ACTIVE", "High", "", "", "", "1//new_ref", 0, false, false, true)
	if err != nil {
		t.Fatalf("UpdateAccountFull error: %v", err)
	}

	accounts := store.ListAccounts()
	if len(accounts) != 1 {
		t.Fatalf("expected exactly 1 account, got %d", len(accounts))
	}
	if accounts[0].Label != "New Label" {
		t.Errorf("expected updated label, got %q", accounts[0].Label)
	}
}
