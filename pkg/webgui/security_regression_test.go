package webgui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

// TestSecurityRegression_DefaultDenyAuthEnforcement validates the loopback authentication security contract:
// 1. Whitelisted public endpoints (static UI assets, health checks, unlock dialog, MCP bridge) succeed without a token.
// 2. All protected business endpoints strictly enforce Default-Deny and return HTTP 401 Unauthorized when unauthenticated.
// 3. Authenticated requests with X-Session-Token or Authorization Bearer header successfully pass the auth barrier.
// 4. Requests with invalid or forged session tokens are denied with HTTP 401.
func TestSecurityRegression_DefaultDenyAuthEnforcement(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")

	srv := NewServer("127.0.0.1:0", "")
	srv.daemonSessionToken = "daemon-secret-test-token-777"
	mux := srv.setupRoutes()
	handler := srv.authMiddleware(mux)

	// 1. Verify isPublicPath classifier contract
	publicPathChecks := []struct {
		path     string
		expected bool
	}{
		{"/", true},
		{"/index.html", true},
		{"/health", true},
		{"/healthz", true},
		{"/api/auth/status", true},
		{"/api/auth/unlock", true},
		{"/mcp", true},
		{"/mcp/stream", true},
		{"/api/bridge/mcp", true},
		{"/assets/app.js", true},
		{"/api/status", false},
		{"/api/accounts", false},
		{"/api/switch", false},
		{"/api/custom_models", false},
		{"/api/accounts/export", false},
	}
	for _, pc := range publicPathChecks {
		if got := isPublicPath(pc.path); got != pc.expected {
			t.Errorf("[AUTH REGRESSION] isPublicPath(%q) = %v, want %v", pc.path, got, pc.expected)
		}
	}

	// 2. Whitelisted public routes served through auth middleware without session token
	publicRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/"},
		{http.MethodGet, "/index.html"},
		{http.MethodGet, "/health"},
		{http.MethodGet, "/healthz"},
		{http.MethodGet, "/api/auth/status"},
		{http.MethodPost, "/api/auth/unlock"},
		{http.MethodOptions, "/mcp"},
		{http.MethodGet, "/assets/test.css"},
	}

	for _, tc := range publicRoutes {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code == http.StatusUnauthorized {
			t.Errorf("[AUTH REGRESSION] Public whitelisted route %s %s was erroneously blocked with 401 Unauthorized", tc.method, tc.path)
		}
	}

	// 3. Protected business routes that MUST return 401 Unauthorized without credentials
	protectedRoutes := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/status", ""},
		{http.MethodGet, "/api/accounts", ""},
		{http.MethodPost, "/api/switch", `{"email":"test@example.com"}`},
		{http.MethodGet, "/api/custom_models", ""},
		{http.MethodGet, "/api/bridge/remote/config", ""},
		{http.MethodGet, "/api/bridge/channels/config", ""},
		{http.MethodGet, "/api/system/installations", ""},
		{http.MethodGet, "/api/gui/projects", ""},
		{http.MethodGet, "/api/tokens/chat-metrics", ""},
		{http.MethodGet, "/api/quota/fleet", ""},
		{http.MethodGet, "/api/files/list?path=" + tempDir, ""},
		{http.MethodGet, "/api/accounts/export", ""},
		{http.MethodPost, "/api/memos/extract", `{"memo_id":"security-probe"}`},
	}

	for _, tc := range protectedRoutes {
		// A. Unauthenticated request -> Must return 401 Unauthorized
		var reqBody *bytes.Reader
		if tc.body != "" {
			reqBody = bytes.NewReader([]byte(tc.body))
		} else {
			reqBody = bytes.NewReader(nil)
		}
		reqUnauth := httptest.NewRequest(tc.method, tc.path, reqBody)
		recUnauth := httptest.NewRecorder()
		handler.ServeHTTP(recUnauth, reqUnauth)

		if recUnauth.Code != http.StatusUnauthorized {
			t.Errorf("[AUTH REGRESSION] Protected route %s %s allowed unauthenticated access (got HTTP %d, want 401)",
				tc.method, tc.path, recUnauth.Code)
		}
		if !strings.Contains(recUnauth.Body.String(), "valid session token or app password required") {
			t.Errorf("[AUTH REGRESSION] Protected route %s %s did not return expected error message: %s",
				tc.method, tc.path, recUnauth.Body.String())
		}

		// B. Invalid token -> Must return 401 Unauthorized
		reqInvalid := httptest.NewRequest(tc.method, tc.path, bytes.NewReader([]byte(tc.body)))
		reqInvalid.Header.Set("X-Session-Token", "forged-invalid-session-token-999")
		recInvalid := httptest.NewRecorder()
		handler.ServeHTTP(recInvalid, reqInvalid)

		if recInvalid.Code != http.StatusUnauthorized {
			t.Errorf("[AUTH REGRESSION] Protected route %s %s accepted forged session token (got HTTP %d, want 401)",
				tc.method, tc.path, recInvalid.Code)
		}

		// C. Authenticated with X-Session-Token -> Must pass auth barrier (Code != 401)
		reqAuth := httptest.NewRequest(tc.method, tc.path, bytes.NewReader([]byte(tc.body)))
		reqAuth.Header.Set("X-Session-Token", srv.daemonSessionToken)
		recAuth := httptest.NewRecorder()
		handler.ServeHTTP(recAuth, reqAuth)

		if recAuth.Code == http.StatusUnauthorized {
			t.Errorf("[AUTH REGRESSION] Route %s %s rejected valid X-Session-Token with 401 Unauthorized", tc.method, tc.path)
		}

		// D. Authenticated with Authorization: Bearer <token> -> Must pass auth barrier (Code != 401)
		reqBearer := httptest.NewRequest(tc.method, tc.path, bytes.NewReader([]byte(tc.body)))
		reqBearer.Header.Set("Authorization", "Bearer "+srv.daemonSessionToken)
		recBearer := httptest.NewRecorder()
		handler.ServeHTTP(recBearer, reqBearer)

		if recBearer.Code == http.StatusUnauthorized {
			t.Errorf("[AUTH REGRESSION] Route %s %s rejected valid Bearer token with 401 Unauthorized", tc.method, tc.path)
		}
	}
}

// TestSecurityRegression_CredentialScrubbingContract validates that:
// 1. General status, account list, and model endpoints never leak plaintext passwords, tokens, or API keys.
// 2. Sensitive fields in /api/accounts and /api/custom_models are safely replaced with [REDACTED].
// 3. /api/accounts/export preserves unredacted credentials to satisfy backup & restoration (Requirement R2).
func TestSecurityRegression_CredentialScrubbingContract(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")

	// Pre-populate account store with sensitive secrets
	kStore, err := keyring.NewStore(filepath.Join(tempDir, "accounts.json"))
	if err != nil {
		t.Fatalf("keyring.NewStore failed: %v", err)
	}

	testAcc := keyring.Account{
		Email:        "scrub-check@example.com",
		Password:     "SensitivePassword123!",
		RefreshToken: "rt_sensitive_google_token_777",
		AccessToken:  "ya29.sensitive_access_token_888",
		TOTPSecret:   "JBSWY3DPEHPK3PXP",
		HasTOTP:      true,
		Label:        "Security Audit Account",
	}
	if err := kStore.AddOrUpdateAccount(&testAcc); err != nil {
		t.Fatalf("AddOrUpdateAccount failed: %v", err)
	}

	// Pre-populate custom model store with secret API key
	cmStore, err := custommodels.NewStore(tempDir)
	if err != nil {
		t.Fatalf("custommodels.NewStore failed: %v", err)
	}
	_ = cmStore.SaveModel(custommodels.CustomModel{
		ID:      "model-secret-1",
		Name:    "DeepSeek V3 Private",
		BaseURL: "https://api.deepseek.com/v1",
		APIKey:  "sk-custom-secret-key-99999",
	})

	srv := NewServer("127.0.0.1:0", "")
	srv.daemonSessionToken = "sec-token-12345"
	srv.customModelsStore = cmStore
	mux := srv.setupRoutes()
	handler := srv.authMiddleware(mux)

	// A. Verify /api/accounts redacts all credentials
	reqAcc := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	reqAcc.Header.Set("X-Session-Token", srv.daemonSessionToken)
	recAcc := httptest.NewRecorder()
	handler.ServeHTTP(recAcc, reqAcc)

	if recAcc.Code != http.StatusOK {
		t.Fatalf("/api/accounts returned HTTP %d, want 200", recAcc.Code)
	}

	bodyAcc := recAcc.Body.String()
	secretsToCheck := []string{
		"SensitivePassword123!",
		"rt_sensitive_google_token_777",
		"ya29.sensitive_access_token_888",
		"JBSWY3DPEHPK3PXP",
	}
	for _, secret := range secretsToCheck {
		if strings.Contains(bodyAcc, secret) {
			t.Fatalf("[LEAK REGRESSION] /api/accounts leaked plaintext secret %q", secret)
		}
	}
	if !strings.Contains(bodyAcc, "[REDACTED]") {
		t.Errorf("[LEAK REGRESSION] /api/accounts response did not contain expected [REDACTED] tokens")
	}

	// B. Verify /api/status does not leak raw credentials
	reqStatus := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	reqStatus.Header.Set("X-Session-Token", srv.daemonSessionToken)
	recStatus := httptest.NewRecorder()
	handler.ServeHTTP(recStatus, reqStatus)

	if recStatus.Code != http.StatusOK {
		t.Fatalf("/api/status returned HTTP %d, want 200", recStatus.Code)
	}
	bodyStatus := recStatus.Body.String()
	for _, secret := range secretsToCheck {
		if strings.Contains(bodyStatus, secret) {
			t.Fatalf("[LEAK REGRESSION] /api/status leaked plaintext secret %q", secret)
		}
	}

	// C. Verify /api/custom_models redacts API keys
	reqCM := httptest.NewRequest(http.MethodGet, "/api/custom_models", nil)
	reqCM.Header.Set("X-Session-Token", srv.daemonSessionToken)
	recCM := httptest.NewRecorder()
	handler.ServeHTTP(recCM, reqCM)

	if recCM.Code != http.StatusOK {
		t.Fatalf("/api/custom_models returned HTTP %d, want 200", recCM.Code)
	}
	bodyCM := recCM.Body.String()
	if strings.Contains(bodyCM, "sk-custom-secret-key-99999") {
		t.Fatalf("[LEAK REGRESSION] /api/custom_models leaked plaintext API key")
	}

	// D. Verify /api/accounts/export preserves UNREDACTED credentials (Requirement R2)
	reqExp := httptest.NewRequest(http.MethodGet, "/api/accounts/export", nil)
	reqExp.Header.Set("X-Session-Token", srv.daemonSessionToken)
	recExp := httptest.NewRecorder()
	handler.ServeHTTP(recExp, reqExp)

	if recExp.Code != http.StatusOK {
		t.Fatalf("/api/accounts/export returned HTTP %d, want 200", recExp.Code)
	}
	bodyExp := recExp.Body.String()
	if !strings.Contains(bodyExp, "SensitivePassword123!") || !strings.Contains(bodyExp, "rt_sensitive_google_token_777") || !strings.Contains(bodyExp, "JBSWY3DPEHPK3PXP") {
		t.Fatalf("[EXPORT REGRESSION] /api/accounts/export must return UNREDACTED credentials for backup/restoration (Requirement R2), got: %s", bodyExp)
	}

	// Verify exported payload parses into valid AccountExport slice
	var exports []keyring.AccountExport
	if err := json.Unmarshal([]byte(bodyExp), &exports); err != nil {
		t.Fatalf("[EXPORT REGRESSION] /api/accounts/export returned invalid JSON: %v", err)
	}
	if len(exports) == 0 || exports[0].Password != "SensitivePassword123!" {
		t.Errorf("[EXPORT REGRESSION] Exported item did not retain exact unredacted password")
	}

	// E. Verify /api/quota/fleet does not leak raw credentials
	reqFleet := httptest.NewRequest(http.MethodGet, "/api/quota/fleet", nil)
	reqFleet.Header.Set("X-Session-Token", srv.daemonSessionToken)
	recFleet := httptest.NewRecorder()
	handler.ServeHTTP(recFleet, reqFleet)

	if recFleet.Code != http.StatusOK {
		t.Fatalf("/api/quota/fleet returned HTTP %d, want 200", recFleet.Code)
	}
	bodyFleet := recFleet.Body.String()
	for _, secret := range secretsToCheck {
		if strings.Contains(bodyFleet, secret) {
			t.Fatalf("[LEAK REGRESSION] /api/quota/fleet leaked plaintext secret %q", secret)
		}
	}
}

// TestSecurityRegression_PlatformIndependentPathJail validates path jail constraints:
// 1. Files inside legitimate temporary directories are accepted.
// 2. Traversal sequences (../, ..\, null bytes, encoded delimiters) are blocked.
// 3. Unix system prefixes (/etc, /proc, /sys, /root, /dev) are rejected.
// 4. Windows drive roots (C:\, D:\) and Windows system paths (C:\Windows, C:\Program Files) are rejected.
// 5. Dangerous deletion attempts on home directory or root aliases are rejected by validateDeletablePath.
func TestSecurityRegression_PlatformIndependentPathJail(t *testing.T) {
	tempDir := t.TempDir()
	legitFile := filepath.Join(tempDir, "safe.txt")
	if err := os.WriteFile(legitFile, []byte("ok"), 0600); err != nil {
		t.Fatalf("Failed to create safe temp file: %v", err)
	}

	// 1. Legitimate file inside TempDir must succeed
	resolved, err := validateJailedPath(legitFile)
	if err != nil {
		t.Errorf("[PATH JAIL] validateJailedPath rejected valid temp file: %v", err)
	}
	if resolved == "" {
		t.Errorf("[PATH JAIL] validateJailedPath returned empty resolved path")
	}

	// 2. Universal hostile attack vectors
	universalHostile := []string{
		"../../../../../../../../etc/passwd",
		"../../../../../../../../etc/shadow",
		"safe.txt\x00evil.exe",
		"safe.txt%00",
		"safe.txt%2500",
		"..%2f..%2fetc/passwd",
		"..%5c..%5cWindows",
		"%2e%2e%2f%2e%2e%2fetc/passwd",
	}

	for _, vec := range universalHostile {
		if res, err := validateJailedPath(vec); err == nil {
			t.Errorf("[PATH JAIL VULNERABILITY] Accepted universal hostile vector %q -> %q", vec, res)
		}
	}

	// 3. Platform-specific hostile vectors
	if runtime.GOOS == "windows" {
		windowsHostile := []string{
			"C:\\",
			"C:\\Windows",
			"C:\\Windows\\System32\\cmd.exe",
			"C:\\Program Files",
			"C:\\ProgramData",
			"\\\\attacker\\share\\trojan.exe",
			"//attacker/share/trojan.exe",
			"D:\\",
		}
		for _, vec := range windowsHostile {
			if res, err := validateJailedPath(vec); err == nil {
				t.Errorf("[PATH JAIL VULNERABILITY] Windows: Accepted system path vector %q -> %q", vec, res)
			}
		}
	} else {
		unixHostile := []string{
			"/etc/passwd",
			"/etc/shadow",
			"/proc/cpuinfo",
			"/sys/kernel",
			"/dev/null",
			"/root/.bashrc",
			"/bin/sh",
		}
		for _, vec := range unixHostile {
			if res, err := validateJailedPath(vec); err == nil {
				t.Errorf("[PATH JAIL VULNERABILITY] Unix: Accepted system path vector %q -> %q", vec, res)
			}
		}
	}

	// 4. Verify validateDeletablePath blocks destructive root deletions
	home, _ := os.UserHomeDir()
	dangerousDeletions := []string{
		"",
		"~",
		".",
		"/",
		"\\",
		"~/",
		"~\\",
		home,
	}
	for _, target := range dangerousDeletions {
		if target == "" {
			continue
		}
		if res, err := validateDeletablePath(target); err == nil {
			t.Errorf("[DELETION VULNERABILITY] validateDeletablePath allowed deletion of dangerous root %q -> %q", target, res)
		}
	}

	// 5. Legitimate sub-file in tempDir must be allowed for deletion
	legitDeletable := filepath.Join(tempDir, "deletable-item.json")
	if err := os.WriteFile(legitDeletable, []byte("{}"), 0600); err != nil {
		t.Fatalf("Failed to create deletable temp file: %v", err)
	}
	delResolved, err := validateDeletablePath(legitDeletable)
	if err != nil {
		t.Errorf("[PATH JAIL] validateDeletablePath rejected valid sub-file: %v", err)
	}
	if delResolved == "" {
		t.Errorf("[PATH JAIL] validateDeletablePath returned empty resolved path")
	}
}
