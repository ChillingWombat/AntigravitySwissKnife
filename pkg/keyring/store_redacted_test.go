package keyring

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	realRefresh = "1//REAL_REFRESH_TOKEN_VALUE_1234567890"
	realAccess  = "ya29.REAL_ACCESS_TOKEN_VALUE_0987654321"
	redacted    = "[REDACTED]"
)

func seedRealAccount(t *testing.T, s *Store) {
	t.Helper()
	if _, err := s.ImportAccount("real@example.com", realRefresh, realAccess, "Real", ""); err != nil {
		t.Fatalf("ImportAccount: %v", err)
	}
}

func requireTokensIntact(t *testing.T, s *Store) {
	t.Helper()
	a, err := s.GetAccount("real@example.com")
	if err != nil || a == nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if a.RefreshToken != realRefresh {
		t.Errorf("refresh token clobbered: got %q", a.RefreshToken)
	}
	if a.AccessToken != realAccess {
		t.Errorf("access token clobbered: got %q", a.AccessToken)
	}
	if a.Credential != nil {
		if a.Credential.RefreshToken == redacted || a.Credential.AccessToken == redacted {
			t.Errorf("credential sub-object clobbered: rt=%q at=%q", a.Credential.RefreshToken, a.Credential.AccessToken)
		}
	}
}

// A redacted account payload (as returned by GET /api/accounts) must never be
// persisted as if the sentinels were credentials.
func TestStore_RedactedPayloadsAreNeverPersisted(t *testing.T) {
	cases := []struct {
		name string
		call func(s *Store) error
	}{
		{
			name: "UpdateAccountFull",
			call: func(s *Store) error {
				return s.UpdateAccountFull("real@example.com", "Real", "Pro", "STANDBY", "Mid", "", redacted, redacted, redacted, 0, false, false, false)
			},
		},
		{
			name: "UpdateAccountDetails",
			call: func(s *Store) error {
				return s.UpdateAccountDetails("real@example.com", "Real", "Pro", "STANDBY", "Mid", "", redacted, redacted, redacted, false)
			},
		},
		{
			name: "SetAccessToken",
			call: func(s *Store) error {
				return s.SetAccessToken("real@example.com", redacted)
			},
		},
		{
			name: "UpdateAccountTokensWithExpiry",
			call: func(s *Store) error {
				return s.UpdateAccountTokensWithExpiry("real@example.com", redacted, redacted, time.Now().Add(time.Hour))
			},
		},
		{
			name: "UpdateAccountTokensAndMetadata",
			call: func(s *Store) error {
				return s.UpdateAccountTokensAndMetadata("real@example.com", redacted, redacted, "Pro", 0)
			},
		},
		{
			name: "UpdateAccountWithStatus",
			call: func(s *Store) error {
				return s.UpdateAccountWithStatus("real@example.com", "Real", "Pro", "STANDBY", redacted, redacted, false)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newRedactedTestStore(t)
			seedRealAccount(t, s)
			if err := tc.call(s); err != nil {
				t.Fatalf("%s returned error: %v", tc.name, err)
			}
			requireTokensIntact(t, s)
		})
	}
}

func TestStore_ImportAccount_IgnoresRedactedPlaceholders(t *testing.T) {
	s := newRedactedTestStore(t)
	seedRealAccount(t, s)

	if _, err := s.ImportAccount("real@example.com", redacted, redacted, "", redacted); err != nil {
		t.Fatalf("ImportAccount: %v", err)
	}
	requireTokensIntact(t, s)
}

func TestStore_BatchImport_IgnoresRedactedPlaceholders(t *testing.T) {
	s := newRedactedTestStore(t)
	seedRealAccount(t, s)

	_, err := s.BatchImportAccounts([]BatchImportItem{{
		Email:        "real@example.com",
		RefreshToken: redacted,
		AccessToken:  redacted,
		TOTPSecret:   redacted,
		Password:     redacted,
	}})
	if err != nil {
		t.Fatalf("BatchImportAccounts: %v", err)
	}
	requireTokensIntact(t, s)
}

func TestStore_AddOrUpdateAccount_KeepsStoredTokensOnRedactedInput(t *testing.T) {
	s := newRedactedTestStore(t)
	seedRealAccount(t, s)

	if err := s.AddOrUpdateAccount(&Account{
		Email:        "real@example.com",
		Label:        "Real",
		Status:       "STANDBY",
		AccessToken:  redacted,
		RefreshToken: redacted,
	}); err != nil {
		t.Fatalf("AddOrUpdateAccount: %v", err)
	}
	requireTokensIntact(t, s)
}

// Surface sync must refuse to publish placeholder credentials into the host
// app's session stores (Windows Credential Manager, jetski file, secret service).
func TestSyncAllSurfaces_RefusesRedactedCredentials(t *testing.T) {
	acc := &Account{
		Email:        "real@example.com",
		AccessToken:  redacted,
		RefreshToken: redacted,
	}
	err := SyncAllSurfaces(acc, []string{acc.Email}, nil)
	if err == nil || !strings.Contains(err.Error(), "redaction placeholders") {
		t.Fatalf("expected SyncAllSurfaces to refuse redacted credentials, got %v", err)
	}

	if got := SyncDesktopStandaloneToken(acc); got != nil {
		t.Fatalf("SyncDesktopStandaloneToken should skip redacted account, got %v", got)
	}
	if got := WriteWindowsKeyringToken(acc); got != nil {
		t.Fatalf("WriteWindowsKeyringToken should skip redacted account, got %v", got)
	}
}

func newRedactedTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(filepath.Join(t.TempDir(), "accounts.json"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}
