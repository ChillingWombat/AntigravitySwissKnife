package keyring

import (
	"encoding/base64"
	"os"
	"path/filepath"
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
