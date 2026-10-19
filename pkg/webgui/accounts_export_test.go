package webgui

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

func TestAccountsExportUnredactedCredentials(t *testing.T) {
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	tmpDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir)

	// Pre-populate keyring store with sensitive credentials
	store, err := keyring.NewStore(filepath.Join(tmpDir, "accounts.json"))
	if err != nil {
		t.Fatalf("failed to initialize keyring store: %v", err)
	}

	importItems := []keyring.BatchImportItem{
		{
			Email:        "vault-user@example.com",
			Password:     "SuperSecretPass2026!",
			TOTPSecret:   "JBSWY3DPEHPK3PXP",
			MFA:          "JBSWY3DPEHPK3PXP",
			RefreshToken: "1//oauth_refresh_unredacted_token",
			Label:        "Vault Account",
			PlanTier:     "Pro",
			Priority:     "High",
		},
	}

	count, err := store.BatchImportAccounts(importItems)
	if err != nil {
		t.Fatalf("failed to batch import account: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 imported account, got %d", count)
	}

	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	// 1. Test GET /api/accounts/export returns actual unredacted credentials
	resp, err := http.Get("http://" + srv.Addr() + "/api/accounts/export")
	if err != nil {
		t.Fatalf("GET /api/accounts/export failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	var exported []keyring.AccountExport
	if err := json.Unmarshal(body, &exported); err != nil {
		t.Fatalf("failed to parse JSON export: %v", err)
	}

	if len(exported) != 1 {
		t.Fatalf("expected 1 exported account, got %d", len(exported))
	}

	item := exported[0]
	if item.Email != "vault-user@example.com" {
		t.Errorf("expected email 'vault-user@example.com', got '%s'", item.Email)
	}
	if item.Password != "SuperSecretPass2026!" {
		t.Errorf("expected password 'SuperSecretPass2026!', got '%s' (must NOT be [REDACTED])", item.Password)
	}
	if item.TOTPSecret != "JBSWY3DPEHPK3PXP" {
		t.Errorf("expected TOTPSecret 'JBSWY3DPEHPK3PXP', got '%s' (must NOT be [REDACTED])", item.TOTPSecret)
	}
	if item.MFA != "JBSWY3DPEHPK3PXP" {
		t.Errorf("expected MFA 'JBSWY3DPEHPK3PXP', got '%s' (must NOT be [REDACTED])", item.MFA)
	}
	if item.RefreshToken != "1//oauth_refresh_unredacted_token" {
		t.Errorf("expected RefreshToken '1//oauth_refresh_unredacted_token', got '%s' (must NOT be [REDACTED])", item.RefreshToken)
	}

	// 2. Test GET /api/accounts/export?download=1 sets Content-Disposition header
	respDl, err := http.Get("http://" + srv.Addr() + "/api/accounts/export?download=1")
	if err != nil {
		t.Fatalf("GET /api/accounts/export?download=1 failed: %v", err)
	}
	defer respDl.Body.Close()

	cd := respDl.Header.Get("Content-Disposition")
	expectedCD := "attachment; filename=\"antigravity_accounts.json\""
	if cd != expectedCD {
		t.Errorf("expected Content-Disposition '%s', got '%s'", expectedCD, cd)
	}

	// 3. Test round-trip restore into a new clean environment via batch import
	tmpDir2 := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir2)
	srv2 := NewServer("127.0.0.1:0", "")
	if err := srv2.Start(); err != nil {
		t.Fatalf("srv2.Start error: %v", err)
	}
	defer srv2.Stop()

	// Import the exported raw JSON into srv2
	respImport, err := http.Post(
		"http://"+srv2.Addr()+"/api/accounts/batch-import",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("POST /api/accounts/batch-import failed: %v", err)
	}
	defer respImport.Body.Close()

	if respImport.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from batch-import, got %d", respImport.StatusCode)
	}

	// Read exported data from srv2 to ensure full restoration of secrets
	respExport2, err := http.Get("http://" + srv2.Addr() + "/api/accounts/export")
	if err != nil {
		t.Fatalf("GET /api/accounts/export from srv2 failed: %v", err)
	}
	defer respExport2.Body.Close()

	var restored []keyring.AccountExport
	if err := json.NewDecoder(respExport2.Body).Decode(&restored); err != nil {
		t.Fatalf("failed to decode restored accounts: %v", err)
	}

	if len(restored) != 1 {
		t.Fatalf("expected 1 restored account, got %d", len(restored))
	}
	if restored[0].Password != "SuperSecretPass2026!" {
		t.Errorf("restored password mismatch: got '%s'", restored[0].Password)
	}
	if restored[0].TOTPSecret != "JBSWY3DPEHPK3PXP" {
		t.Errorf("restored TOTPSecret mismatch: got '%s'", restored[0].TOTPSecret)
	}
	if restored[0].RefreshToken != "1//oauth_refresh_unredacted_token" {
		t.Errorf("restored RefreshToken mismatch: got '%s'", restored[0].RefreshToken)
	}
}

func TestAccountsExportFilterByEmail(t *testing.T) {
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	tmpDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir)

	store, err := keyring.NewStore(filepath.Join(tmpDir, "accounts.json"))
	if err != nil {
		t.Fatalf("failed to initialize keyring store: %v", err)
	}

	importItems := []keyring.BatchImportItem{
		{
			Email:        "alpha@example.com",
			Password:     "PassAlpha123!",
			TOTPSecret:   "JBSWY3DPEHPK3PXP",
			RefreshToken: "1//refresh_token_alpha",
		},
		{
			Email:        "beta@example.com",
			Password:     "PassBeta456!",
			TOTPSecret:   "JBSWY3DPEHPK3PXQ",
			RefreshToken: "1//refresh_token_beta",
		},
	}

	if _, err := store.BatchImportAccounts(importItems); err != nil {
		t.Fatalf("failed to batch import accounts: %v", err)
	}

	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	// 1. Filter for alpha@example.com (case-insensitive)
	respAlpha, err := http.Get("http://" + srv.Addr() + "/api/accounts/export?email=ALPHA@example.com")
	if err != nil {
		t.Fatalf("GET /api/accounts/export?email=... failed: %v", err)
	}
	defer respAlpha.Body.Close()

	var alphaList []keyring.AccountExport
	if err := json.NewDecoder(respAlpha.Body).Decode(&alphaList); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(alphaList) != 1 {
		t.Fatalf("expected 1 account for alpha, got %d", len(alphaList))
	}
	if alphaList[0].Email != "alpha@example.com" {
		t.Errorf("expected alpha@example.com, got %s", alphaList[0].Email)
	}
	if alphaList[0].RefreshToken != "1//refresh_token_alpha" {
		t.Errorf("expected 1//refresh_token_alpha, got %s", alphaList[0].RefreshToken)
	}

	// 2. Filter for non-existent email
	respNone, err := http.Get("http://" + srv.Addr() + "/api/accounts/export?email=nonexistent@example.com")
	if err != nil {
		t.Fatalf("GET /api/accounts/export?email=nonexistent failed: %v", err)
	}
	defer respNone.Body.Close()

	var noneList []keyring.AccountExport
	if err := json.NewDecoder(respNone.Body).Decode(&noneList); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(noneList) != 0 {
		t.Fatalf("expected 0 accounts for nonexistent email, got %d", len(noneList))
	}
}
