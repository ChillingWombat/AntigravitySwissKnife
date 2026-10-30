package webgui

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

// ============================================================================
// CHALLENGE 1: AUTH BYPASS ADVERSARIAL TEST SUITE
// ============================================================================

// TestChallenger_AuthBypass_SensitiveEndpointsRequireAuth tests that requests
// without authentication credentials (no X-Session-Token and no Authorization header)
// sent to all sensitive /api/* endpoints are strictly rejected with HTTP 401 Unauthorized.
func TestChallenger_AuthBypass_SensitiveEndpointsRequireAuth(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)

	srv := newTestServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// List of sensitive endpoints that MUST enforce default-deny authentication
	sensitiveEndpoints := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/files/list", ""},
		{http.MethodPost, "/api/cache/prune", "{}"},
		{http.MethodGet, "/api/accounts", ""},
		{http.MethodGet, "/api/accounts/list", ""},
		{http.MethodGet, "/api/files/read?path=test.txt", ""},
		{http.MethodPost, "/api/files/write", `{"path":"test.txt","content":"data"}`},
		{http.MethodPost, "/api/files/delete", `{"path":"test.txt"}`},
		{http.MethodGet, "/api/status", ""},
		{http.MethodGet, "/api/surfaces", ""},
		{http.MethodGet, "/api/quota/fleet", ""},
		{http.MethodPost, "/api/custom_models/refresh_quotas", ""},
		{http.MethodGet, "/api/keyring/active", ""},
		{http.MethodGet, "/api/gui/config", ""},
		{http.MethodGet, "/api/token/metrics", ""},
		{http.MethodGet, "/api/extensions", ""},
	}

	for _, ep := range sensitiveEndpoints {
		var req *http.Request
		var err error
		if ep.body != "" {
			req, err = http.NewRequest(ep.method, baseURL+ep.path, strings.NewReader(ep.body))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, err = http.NewRequest(ep.method, baseURL+ep.path, nil)
		}
		if err != nil {
			t.Fatalf("failed to create request for %s %s: %v", ep.method, ep.path, err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("HTTP request to %s %s failed: %v", ep.method, ep.path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("CRITICAL AUTH BYPASS VULNERABILITY: endpoint %s %s allowed unauthenticated access (status %d, expected 401)",
				ep.method, ep.path, resp.StatusCode)
		}

		var res map[string]interface{}
		if err := json.Unmarshal(body, &res); err == nil {
			if res["success"] == true {
				t.Errorf("endpoint %s %s returned success=true despite missing credentials", ep.method, ep.path)
			}
			errStr, _ := res["error"].(string)
			if !strings.Contains(strings.ToLower(errStr), "unauthorized") {
				t.Errorf("endpoint %s %s returned error %q, expected 'Unauthorized'", ep.method, ep.path, errStr)
			}
		}
	}
}

// TestChallenger_AuthBypass_ForgedTokensRejected verifies that sending forged,
// truncated, or spoofed credentials consistently yields HTTP 401 Unauthorized.
func TestChallenger_AuthBypass_ForgedTokensRejected(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)

	srv := newTestServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	forgedAttempts := []struct {
		name       string
		headerKey  string
		headerVal  string
		queryParam string
	}{
		{"Arbitrary Hex Token", "X-Session-Token", "deadbeef00112233445566778899aabbccddeeff00112233445566778899aabb", ""},
		{"Short Fake Token", "X-Session-Token", "fake-token-123", ""},
		{"Empty Token Header", "X-Session-Token", "", ""},
		{"Forged Bearer Token", "Authorization", "Bearer forged-bearer-token-xyz999", ""},
		{"Empty Bearer Token", "Authorization", "Bearer ", ""},
		{"Non-Bearer Auth Scheme", "Authorization", "Basic YWRtaW46cGFzc3dvcmQ=", ""},
		{"Query Parameter Token Spoof", "", "", "?token=spoofed-query-token"},
		{"Wrong App Password Header", "X-App-Password", "WrongPassword123!", ""},
		{"Query Parameter Password Spoof", "", "", "?password=WrongPassword123!"},
	}

	for _, att := range forgedAttempts {
		reqURL := baseURL + "/api/status" + att.queryParam
		req, err := http.NewRequest(http.MethodGet, reqURL, nil)
		if err != nil {
			t.Fatalf("failed to create request for %s: %v", att.name, err)
		}
		if att.headerKey != "" {
			req.Header.Set(att.headerKey, att.headerVal)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("HTTP request for %s failed: %v", att.name, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("CRITICAL AUTH BYPASS: %s was granted access (status %d, expected 401)",
				att.name, resp.StatusCode)
		}
	}
}

// TestChallenger_AuthBypass_PublicWhitelistEndpoints verifies that legitimately
// public endpoints (/healthz, /, /index.html, /api/auth/status, /api/auth/unlock)
// are accessible without authentication.
func TestChallenger_AuthBypass_PublicWhitelistEndpoints(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)

	srv := newTestServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	publicEndpoints := []struct {
		method         string
		path           string
		body           string
		expectNotEqual int
	}{
		{http.MethodGet, "/healthz", "", http.StatusUnauthorized},
		{http.MethodGet, "/", "", http.StatusUnauthorized},
		{http.MethodGet, "/index.html", "", http.StatusUnauthorized},
		{http.MethodGet, "/api/auth/status", "", http.StatusUnauthorized},
		{http.MethodPost, "/api/auth/unlock", `{"password":"test"}`, http.StatusUnauthorized},
	}

	for _, ep := range publicEndpoints {
		var req *http.Request
		if ep.body != "" {
			req, _ = http.NewRequest(ep.method, baseURL+ep.path, strings.NewReader(ep.body))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(ep.method, baseURL+ep.path, nil)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("public endpoint request %s %s failed: %v", ep.method, ep.path, err)
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized {
			t.Errorf("whitelisted public endpoint %s %s returned 401 Unauthorized; must be publicly accessible",
				ep.method, ep.path)
		}
	}
}

// TestChallenger_AuthBypass_ValidSessionTokenPermitted verifies that presenting
// a genuine session token via X-Session-Token or Authorization: Bearer successfully
// grants access to sensitive endpoints.
func TestChallenger_AuthBypass_ValidSessionTokenPermitted(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)

	srv := newTestServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()
	token := srv.GetSessionToken()
	if token == "" {
		t.Fatalf("server did not generate a startup session token")
	}

	// 1. Via X-Session-Token
	req1, _ := http.NewRequest(http.MethodGet, baseURL+"/api/status", nil)
	req1.Header.Set("X-Session-Token", token)
	resp1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatalf("request 1 failed: %v", err)
	}
	resp1.Body.Close()
	if resp1.StatusCode == http.StatusUnauthorized {
		t.Errorf("valid session token in X-Session-Token was rejected with 401")
	}

	// 2. Via Authorization: Bearer <token>
	req2, _ := http.NewRequest(http.MethodGet, baseURL+"/api/status", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("request 2 failed: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode == http.StatusUnauthorized {
		t.Errorf("valid session token in Authorization Bearer was rejected with 401")
	}

	// 3. Via Query parameter ?token=<token>
	req3, _ := http.NewRequest(http.MethodGet, baseURL+"/api/status?token="+token, nil)
	resp3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatalf("request 3 failed: %v", err)
	}
	resp3.Body.Close()
	if resp3.StatusCode == http.StatusUnauthorized {
		t.Errorf("valid session token in query string was rejected with 401")
	}
}

// ============================================================================
// CHALLENGE 2: PATH DELETION WIPE ADVERSARIAL TEST SUITE
// ============================================================================

// TestChallenger_PathDeletionWipe_MaliciousInputsRejected empirically verifies
// that validateDeletablePath strictly blocks destructive inputs:
// ~, ., /, \, C:\, C:\Users\David (home dir root), D:\, /etc, /usr, ~/, ~\, "", C:\Windows, C:\Program Files
func TestChallenger_PathDeletionWipe_MaliciousInputsRejected(t *testing.T) {
	homeDir, _ := os.UserHomeDir()

	maliciousCandidates := []struct {
		name string
		path string
	}{
		{"Literal Tilde Root", "~"},
		{"Literal Dot Working Dir", "."},
		{"Forward Slash Root", "/"},
		{"Backslash Root", "\\"},
		{"Windows C Drive Root", "C:\\"},
		{"Windows C Drive Root Slash", "C:/"},
		{"Windows C Drive Volume Alias", "C:"},
		{"Windows D Drive Root", "D:\\"},
		{"User Home Directory Root", homeDir},
		{"Unix etc Directory", "/etc"},
		{"Unix usr Directory", "/usr"},
		{"Unix var Directory", "/var"},
		{"Unix bin Directory", "/bin"},
		{"Tilde Trailing Slash", "~/"},
		{"Tilde Trailing Backslash", "~\\"},
		{"Empty Path", ""},
		{"Whitespace Only", "   "},
		{"Windows System Directory", "C:\\Windows"},
		{"Windows Program Files Directory", "C:\\Program Files"},
		{"Windows Program Files x86", "C:\\Program Files (x86)"},
		{"Parent Directory Traversal Root", "../.."},
		{"Complex Traversal to Root", "..\\..\\..\\..\\.."},
	}

	for _, tc := range maliciousCandidates {
		if tc.path == "" && tc.name != "Empty Path" {
			continue
		}
		validatedPath, err := validateDeletablePath(tc.path)
		if err == nil {
			t.Errorf("CRITICAL PATH WIPE VULNERABILITY: validateDeletablePath(%q) [%s] SUCCEEDED (resolved to %q); MUST BE REJECTED!",
				tc.path, tc.name, validatedPath)
		} else {
			t.Logf("[PASS] Blocked malicious delete candidate %q (%s): %v", tc.path, tc.name, err)
		}
	}
}

// TestChallenger_PathDeletionWipe_NestedProjectPathsPermitted verifies that
// legitimate nested paths (at least 1 directory level below an allowed root)
// are properly permitted by validateDeletablePath.
func TestChallenger_PathDeletionWipe_NestedProjectPathsPermitted(t *testing.T) {
	homeDir, _ := os.UserHomeDir()
	tempDir := os.TempDir()
	cfgDir := core.GetConfigDir()

	allowedNestedPaths := []struct {
		name string
		path string
	}{
		{"Nested Project File in Home", filepath.Join(homeDir, "Projects", "test", "sub", "file.txt")},
		{"Nested Project Subfolder in Home", filepath.Join(homeDir, "Projects", "antigravity", "build", "dist")},
		{"Nested Cache Item in Config Dir", filepath.Join(cfgDir, "cache", "sessions", "temp_artifact.bin")},
		{"Nested File in Temp Dir", filepath.Join(tempDir, "swiss_challenge_wipe", "sub", "artifact.tmp")},
	}

	for _, tc := range allowedNestedPaths {
		validatedPath, err := validateDeletablePath(tc.path)
		if err != nil {
			t.Errorf("FALSE POSITIVE REJECTION: legitimate nested path %q [%s] was rejected: %v",
				tc.path, tc.name, err)
		} else if validatedPath == "" {
			t.Errorf("validated path for %q was empty", tc.path)
		} else {
			t.Logf("[PASS] Permitted legitimate nested path %q: resolved to %q", tc.path, validatedPath)
		}
	}
}

// TestChallenger_PathDeletionWipe_HTTPDeleteEndpoint tests the live HTTP POST /api/files/delete
// endpoint with both malicious inputs and valid nested targets.
func TestChallenger_PathDeletionWipe_HTTPDeleteEndpoint(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)

	srv := newTestServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()
	token := srv.GetSessionToken()

	// 1. Unauthenticated request to /api/files/delete must return 401
	unauthReq, _ := http.NewRequest(http.MethodPost, baseURL+"/api/files/delete", strings.NewReader(`{"path":"~"}`))
	unauthReq.Header.Set("Content-Type", "application/json")
	unauthResp, err := http.DefaultClient.Do(unauthReq)
	if err != nil {
		t.Fatalf("unauth request failed: %v", err)
	}
	unauthResp.Body.Close()
	if unauthResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated /api/files/delete, got %d", unauthResp.StatusCode)
	}

	// 2. Authenticated malicious delete requests must return success=false
	maliciousPayloads := []string{
		`{"path":"~"}`,
		`{"path":"/"}`,
		`{"path":"C:\\"}`,
		`{"paths":["~", "."]}`,
		`{"path":"C:\\Windows"}`,
		`{"path":"/etc"}`,
		`{"path":"/usr"}`,
	}

	for _, payload := range maliciousPayloads {
		req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/files/delete", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Session-Token", token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("delete request failed: %v", err)
		}
		var res map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&res)
		resp.Body.Close()

		if res["success"] == true {
			t.Errorf("CRITICAL: POST /api/files/delete succeeded with malicious payload %s", payload)
		}
	}

	// 3. Authenticated valid nested delete request must succeed
	nestedSubDir := filepath.Join(tempDir, "deletable_sub")
	_ = os.MkdirAll(nestedSubDir, 0700)
	targetFile := filepath.Join(nestedSubDir, "target_to_delete.txt")
	if err := os.WriteFile(targetFile, []byte("delete me"), 0600); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	validPayload := `{"path":` + strconvQuote(targetFile) + `}`
	reqValid, _ := http.NewRequest(http.MethodPost, baseURL+"/api/files/delete", strings.NewReader(validPayload))
	reqValid.Header.Set("Content-Type", "application/json")
	reqValid.Header.Set("X-Session-Token", token)

	respValid, err := http.DefaultClient.Do(reqValid)
	if err != nil {
		t.Fatalf("valid delete request failed: %v", err)
	}
	var validRes map[string]interface{}
	_ = json.NewDecoder(respValid.Body).Decode(&validRes)
	respValid.Body.Close()

	if validRes["success"] != true {
		t.Errorf("expected valid nested file deletion to succeed, got %v", validRes)
	}
	if _, err := os.Stat(targetFile); !os.IsNotExist(err) {
		t.Errorf("expected file %s to be deleted from disk, but it still exists", targetFile)
	}
}

// ============================================================================
// CHALLENGE 3: TOKEN EXFILTRATION ADVERSARIAL TEST SUITE
// ============================================================================

// TestChallenger_TokenExfiltration_StatusAndSurfacesAndQuota verifies that
// GET /api/status, GET /api/surfaces, and GET /api/quota/fleet NEVER leak raw OAuth
// tokens, cleartext passwords, or TOTP secrets in response JSON.
func TestChallenger_TokenExfiltration_StatusAndSurfacesAndQuota(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)

	accountsFile := filepath.Join(tempDir, "accounts.json")
	kStore, err := keyring.NewStore(accountsFile)
	if err != nil {
		t.Fatalf("keyring.NewStore failed: %v", err)
	}

	// High-entropy unique secret strings to hunt for in HTTP responses
	canaryPassword := "UltraSecretPassword999!@#"
	canaryTOTP := "JBSWY3DPEHPK3PXP"
	canaryAccessToken := "ya29.secret_live_access_token_adversarial_12345"
	canaryRefreshToken := "1//secret_live_refresh_token_adversarial_67890"
	canaryIDToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.adversarial_id_token"

	testAcc := keyring.Account{
		Email:        "adversarial-audit@example.com",
		Label:        "Audit Account",
		Password:     canaryPassword,
		TOTPSecret:   canaryTOTP,
		AccessToken:  canaryAccessToken,
		RefreshToken: canaryRefreshToken,
		IDToken:      canaryIDToken,
	}
	if err := kStore.AddOrUpdateAccount(&testAcc); err != nil {
		t.Fatalf("AddOrUpdateAccount failed: %v", err)
	}

	srv := newTestServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()
	token := srv.GetSessionToken()

	allCanaries := []struct {
		name  string
		value string
	}{
		{"Plaintext Password", canaryPassword},
		{"TOTP Secret", canaryTOTP},
		{"OAuth Access Token", canaryAccessToken},
		{"OAuth Refresh Token", canaryRefreshToken},
		{"OAuth ID Token", canaryIDToken},
	}

	targetEndpoints := []string{
		"/api/status",
		"/api/surfaces",
		"/api/quota/fleet",
	}

	for _, ep := range targetEndpoints {
		req, _ := http.NewRequest(http.MethodGet, baseURL+ep, nil)
		req.Header.Set("X-Session-Token", token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET %s failed: %v", ep, err)
		}
		rawBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("failed to read response from %s: %v", ep, err)
		}

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK from %s, got %d", ep, resp.StatusCode)
		}

		bodyStr := string(rawBody)

		// 1. Raw Byte String Hunt: ensure canary secret substrings NEVER exist
		for _, canary := range allCanaries {
			if strings.Contains(bodyStr, canary.value) {
				t.Fatalf("CRITICAL CREDENTIAL EXFILTRATION LEAK: endpoint %s contains %s in raw response body!",
					ep, canary.name)
			}
		}

		// 2. Recursive JSON Key Inspection
		var parsed interface{}
		if err := json.Unmarshal(rawBody, &parsed); err != nil {
			t.Fatalf("endpoint %s response is not valid JSON: %v", ep, err)
		}

		inspectJSONForLeaks(t, ep, parsed, "")
	}
}

// inspectJSONForLeaks recursively traverses decoded JSON inspecting for sensitive keys.
func inspectJSONForLeaks(t *testing.T, endpoint string, node interface{}, path string) {
	forbiddenCleartextKeys := map[string]bool{
		"access_token": true,
		"id_token":     true,
		"password":     true,
		"totp_secret":  true,
		"totpsecret":   true,
	}

	switch val := node.(type) {
	case map[string]interface{}:
		for k, v := range val {
			fieldPath := k
			if path != "" {
				fieldPath = path + "." + k
			}
			lowerKey := strings.ToLower(k)

			if forbiddenCleartextKeys[lowerKey] {
				if strVal, ok := v.(string); ok && strings.TrimSpace(strVal) != "" && strVal != "[REDACTED]" {
					t.Errorf("CRITICAL EXFILTRATION: endpoint %s leaked non-empty credential key %q = %q",
						endpoint, fieldPath, strVal)
				}
			}

			if lowerKey == "refresh_token" {
				if strVal, ok := v.(string); ok && strings.TrimSpace(strVal) != "" && strVal != "[REDACTED]" {
					t.Errorf("CRITICAL EXFILTRATION: endpoint %s leaked unmasked refresh_token %q = %q",
						endpoint, fieldPath, strVal)
				}
			}

			inspectJSONForLeaks(t, endpoint, v, fieldPath)
		}
	case []interface{}:
		for i, item := range val {
			inspectJSONForLeaks(t, endpoint, item, path+"["+strconvItoa(i)+"]")
		}
	}
}

// TestChallenger_TokenExfiltration_CustomModelsRedacted verifies that
// POST /api/custom_models/refresh_quotas strictly redacts custom model API keys.
func TestChallenger_TokenExfiltration_CustomModelsRedacted(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)

	cmStore, err := custommodels.NewStore(tempDir)
	if err != nil {
		t.Fatalf("custommodels.NewStore failed: %v", err)
	}

	canaryKey := "sk-live-deepseek-super-secret-key-999888777"
	err = cmStore.SaveModel(custommodels.CustomModel{
		ID:           "deepseek-test-m4",
		Name:         "DeepSeek Test",
		ProviderType: custommodels.ProviderOpenAI,
		APIKey:       canaryKey,
		BaseURL:      "https://api.deepseek.com",
	})
	if err != nil {
		t.Fatalf("failed to add test custom model: %v", err)
	}
	if err := cmStore.Save(); err != nil {
		t.Fatalf("failed to save custom models store: %v", err)
	}

	srv := newTestServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()
	token := srv.GetSessionToken()

	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/custom_models/refresh_quotas", nil)
	req.Header.Set("X-Session-Token", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("refresh quotas request failed: %v", err)
	}
	rawBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if strings.Contains(string(rawBody), canaryKey) {
		t.Fatalf("CRITICAL LEAK: POST /api/custom_models/refresh_quotas leaked cleartext API key %q", canaryKey)
	}

	var res custommodels.Config
	if err := json.Unmarshal(rawBody, &res); err == nil {
		for _, m := range res.Models {
			if m.ID == "deepseek-test-m4" {
				if m.APIKey != "[REDACTED]" {
					t.Errorf("expected APIKey to be '[REDACTED]', got %q", m.APIKey)
				}
			}
		}
	}
}

// strconvQuote helper formats string as JSON quoted string
func strconvQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// strconvItoa helper formats integer as string
func strconvItoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}
