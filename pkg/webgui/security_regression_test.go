package webgui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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

	srv := newTestServer("127.0.0.1:0", "")
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

	srv := newTestServer("127.0.0.1:0", "")
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

// TestSecurityRegression_PlatformIndependentPathContract validates the path
// contract that replaced the directory whitelist (issue #157):
//  1. Files inside legitimate temporary directories are accepted.
//  2. Relative paths and traversal sequences (../, ..\, null bytes, encoded
//     delimiters) are blocked — a relative path is never resolved against the
//     working directory.
//  3. Any absolute path, including one outside the home directory, can be
//     listed and read through the file API.
//  4. Reads of sensitive locations are allowed, destructive writes to them are
//     not (read allowed / write denied).
//  5. Dangerous deletion attempts on home directory or root aliases are rejected
//     by validateDeletablePath.
//
// Renamed from TestSecurityRegression_PlatformIndependentPathJail: there is no
// path jail any more, so the old name asserted semantics the code no longer has.
func TestSecurityRegression_PlatformIndependentPathContract(t *testing.T) {
	s := &Server{}
	tempDir := t.TempDir()
	legitFile := filepath.Join(tempDir, "safe.txt")
	if err := os.WriteFile(legitFile, []byte("ok"), 0600); err != nil {
		t.Fatalf("Failed to create safe temp file: %v", err)
	}

	// 1. Legitimate file inside TempDir must succeed
	resolved, err := validateJailedPath(legitFile)
	if err != nil {
		t.Errorf("[PATH CONTRACT] validateJailedPath rejected valid temp file: %v", err)
	}
	if resolved == "" {
		t.Errorf("[PATH CONTRACT] validateJailedPath returned empty resolved path")
	}

	// 2. Universal hostile attack vectors. All of these are either relative or
	//    malformed, so they must be rejected on every platform.
	universalHostile := []string{
		"../../../../../../../../etc/passwd",
		"../../../../../../../../etc/shadow",
		`..\..\Windows\System32\cmd.exe`,
		"relative/file.txt",
		".",
		"safe.txt\x00evil.exe",
		"safe.txt%00",
		"safe.txt%2500",
		"..%2f..%2fetc/passwd",
		"..%5c..%5cWindows",
		"%2e%2e%2f%2e%2e%2fetc/passwd",
		"%25252e%25252e/%25252e%25252e/etc/passwd",
	}

	for _, vec := range universalHostile {
		if res, err := validateJailedPath(vec); err == nil {
			t.Errorf("[PATH CONTRACT VULNERABILITY] Accepted universal hostile vector %q -> %q", vec, res)
		}

		// The same vectors must be refused by the read endpoint, not just the
		// resolver.
		req := httptest.NewRequest(http.MethodGet, "/api/files/read?path="+url.QueryEscape(vec), nil)
		w := httptest.NewRecorder()
		s.handleFilesRead(w, req)
		var resp struct {
			Success bool `json:"success"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode read response for %q: %v", vec, err)
		}
		if resp.Success {
			t.Errorf("[PATH CONTRACT VULNERABILITY] /api/files/read accepted hostile vector %q: %s", vec, w.Body.String())
		}
	}

	// 3. An absolute path outside the home directory is listable and readable.
	//    This is the behaviour the whitelist removal was requested for.
	outsideDir := createDirOutsideHome(t)
	outsideFile := filepath.Join(outsideDir, "outside-home.txt")
	if err := os.WriteFile(outsideFile, []byte("outside-ok"), 0600); err != nil {
		t.Fatalf("failed to create outside-home file: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/files/list?path="+url.QueryEscape(outsideDir), nil)
	listW := httptest.NewRecorder()
	s.handleFilesList(listW, listReq)
	var listResp struct {
		Success bool `json:"success"`
		Files   []struct {
			Name string `json:"name"`
		} `json:"files"`
	}
	if err := json.Unmarshal(listW.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if !listResp.Success {
		t.Errorf("[PATH CONTRACT] /api/files/list refused outside-home directory %q: %s", outsideDir, listW.Body.String())
	}
	found := false
	for _, f := range listResp.Files {
		if strings.EqualFold(f.Name, "outside-home.txt") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("[PATH CONTRACT] outside-home listing did not include outside-home.txt: %s", listW.Body.String())
	}

	readOutsideReq := httptest.NewRequest(http.MethodGet, "/api/files/read?path="+url.QueryEscape(outsideFile), nil)
	readOutsideW := httptest.NewRecorder()
	s.handleFilesRead(readOutsideW, readOutsideReq)
	var readOutsideResp struct {
		Success bool   `json:"success"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(readOutsideW.Body.Bytes(), &readOutsideResp); err != nil {
		t.Fatalf("decode outside-home read response: %v", err)
	}
	if !readOutsideResp.Success || readOutsideResp.Content != "outside-ok" {
		t.Errorf("[PATH CONTRACT] outside-home read failed: success=%v content=%q body=%s",
			readOutsideResp.Success, readOutsideResp.Content, readOutsideW.Body.String())
	}

	// 4. Sensitive locations are readable; destructive writes to them are not.
	var sensitiveReadPath, sensitiveWritePath string
	if runtime.GOOS == "windows" {
		sensitiveReadPath = `C:\Windows\System32\drivers\etc\hosts`
		sensitiveWritePath = `C:\Windows\System32\swiss-regression-test.txt`
	} else {
		sensitiveReadPath = "/etc/hosts"
		sensitiveWritePath = "/etc/swiss-regression-test"
	}

	reqReadSensitive := httptest.NewRequest(http.MethodGet, "/api/files/read?path="+url.QueryEscape(sensitiveReadPath), nil)
	wReadSensitive := httptest.NewRecorder()
	s.handleFilesRead(wReadSensitive, reqReadSensitive)
	var respReadSensitive struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(wReadSensitive.Body.Bytes(), &respReadSensitive); err != nil {
		t.Fatalf("decode sensitive read response: %v", err)
	}
	if !respReadSensitive.Success {
		t.Errorf("[REGRESSION] reading sensitive location %q was refused: %s", sensitiveReadPath, wReadSensitive.Body.String())
	}

	writeBody, _ := json.Marshal(map[string]string{"path": sensitiveWritePath, "content": "x"})
	reqWriteSensitive := httptest.NewRequest(http.MethodPost, "/api/files/write", bytes.NewReader(writeBody))
	wWriteSensitive := httptest.NewRecorder()
	s.handleFilesWrite(wWriteSensitive, reqWriteSensitive)
	var respWriteSensitive struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(wWriteSensitive.Body.Bytes(), &respWriteSensitive); err != nil {
		t.Fatalf("decode sensitive write response: %v", err)
	}
	if respWriteSensitive.Success {
		t.Errorf("[PATH CONTRACT VULNERABILITY] write to sensitive location %q was allowed: %s",
			sensitiveWritePath, wWriteSensitive.Body.String())
	}

	// Credential stores are refused for destructive operations on every platform.
	if home, errHome := os.UserHomeDir(); errHome == nil && home != "" {
		for _, credential := range []string{
			filepath.Join(home, ".ssh", "authorized_keys"),
			filepath.Join(home, ".aws", "credentials"),
			filepath.Join(home, ".config", "gcloud", "credentials.db"),
		} {
			if res, err := validateDestructivePath(credential); err == nil {
				t.Errorf("[PATH CONTRACT VULNERABILITY] destructive operation on credential path %q allowed -> %q", credential, res)
			}
			if res, err := validateJailedPath(credential); err != nil {
				t.Errorf("[REGRESSION] read of credential path %q was refused: %v (resolved %q)", credential, err, res)
			}
		}
	}

	// 5. Verify validateDeletablePath blocks destructive root deletions
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

	// 6. Legitimate sub-file in tempDir must be allowed for deletion
	legitDeletable := filepath.Join(tempDir, "deletable-item.json")
	if err := os.WriteFile(legitDeletable, []byte("{}"), 0600); err != nil {
		t.Fatalf("Failed to create deletable temp file: %v", err)
	}
	delResolved, err := validateDeletablePath(legitDeletable)
	if err != nil {
		t.Errorf("[PATH CONTRACT] validateDeletablePath rejected valid sub-file: %v", err)
	}
	if delResolved == "" {
		t.Errorf("[PATH CONTRACT] validateDeletablePath returned empty resolved path")
	}
}

func TestSecurityRegression_PlatformIndependentPathContract_TempRootContract(t *testing.T) {
	tempRoot := filepath.Clean(os.TempDir())
	forbiddenRoots := []string{tempRoot, "/var/folders", "/private/var/folders"}
	for _, root := range forbiddenRoots {
		if isAllowedTempPath(root) {
			t.Errorf("[TEMP ROOT VULNERABILITY] isAllowedTempPath accepted root %q", root)
		}
		if resolved, err := validateDeletablePath(root); err == nil {
			t.Errorf("[TEMP ROOT VULNERABILITY] validateDeletablePath accepted root %q -> %q", root, resolved)
		}
	}

	child := filepath.Join(t.TempDir(), "deletable.txt")
	if err := os.WriteFile(child, []byte("ok"), 0600); err != nil {
		t.Fatalf("create temp child: %v", err)
	}
	if !isAllowedTempPath(child) {
		t.Errorf("[PATH CONTRACT] isAllowedTempPath rejected strict temp child %q", child)
	}
	if resolved, err := validateDeletablePath(child); err != nil {
		t.Errorf("[PATH CONTRACT] validateDeletablePath rejected strict temp child %q: %v", child, err)
	} else if resolved == "" {
		t.Errorf("[PATH CONTRACT] validateDeletablePath returned an empty path for %q", child)
	}

	aliases := []struct {
		input string
		want  string
	}{
		{"/var/folders/AB/CD/T/", "/var/folders/AB/CD/T"},
		{"/private/var/folders/AB/CD/T", "/var/folders/AB/CD/T"},
		{"/private/var/folders", "/var/folders"},
	}
	for _, alias := range aliases {
		if got := normalizeMacOSPathAlias(alias.input); got != alias.want {
			t.Errorf("[PATH ALIAS] normalizeMacOSPathAlias(%q) = %q, want %q", alias.input, got, alias.want)
		}
	}
}
