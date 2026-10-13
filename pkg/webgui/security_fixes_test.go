package webgui

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

func TestCORSStrictValidation(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// Adversarial origins that MUST be rejected (Issue 43)
	badOrigins := []string{
		"http://127.0.0.1.evil.com",
		"http://127.0.0.1.attacker.org:8765",
		"https://localhost.attacker.io",
		"http://evil.com",
		"http://not-localhost.com",
		"null",
		"*",
	}

	for _, origin := range badOrigins {
		req, _ := http.NewRequest(http.MethodOptions, baseURL+"/api/gui/config", nil)
		req.Header.Set("Origin", origin)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("OPTIONS request failed: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for bad origin %q, got %d", origin, resp.StatusCode)
		}
		if allow := resp.Header.Get("Access-Control-Allow-Origin"); allow != "" {
			t.Errorf("expected no Access-Control-Allow-Origin for bad origin %q, got %q", origin, allow)
		}
	}

	// Legitimate origins that MUST be accepted
	goodOrigins := []string{
		"http://127.0.0.1:8765",
		"http://localhost:5173",
		"http://127.0.0.1:3000",
		"vscode-file://vscode-app",
		"plugin://swiss-tools",
	}

	for _, origin := range goodOrigins {
		req, _ := http.NewRequest(http.MethodOptions, baseURL+"/api/gui/config", nil)
		req.Header.Set("Origin", origin)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("OPTIONS request failed: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("expected 204 NoContent for good origin %q, got %d", origin, resp.StatusCode)
		}
		if allow := resp.Header.Get("Access-Control-Allow-Origin"); allow != origin {
			t.Errorf("expected Access-Control-Allow-Origin %q, got %q", origin, allow)
		}
	}
}

func TestAccountsRedactionAndPreservation(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)

	kStore, err := keyring.NewStore(filepath.Join(tempDir, "accounts.json"))
	if err != nil {
		t.Fatalf("keyring.NewStore failed: %v", err)
	}

	testAcc := keyring.Account{
		Email:        "vault-test@example.com",
		Label:        "Initial Label",
		Password:     "SuperSecretPassword123!",
		RefreshToken: "rt_sensitive_google_token_999",
		AccessToken:  "ya29.sensitive_access_token_888",
		TOTPSecret:   "JBSWY3DPEHPK3PXP",
	}
	if err := kStore.AddOrUpdateAccount(&testAcc); err != nil {
		t.Fatalf("AddOrUpdateAccount failed: %v", err)
	}

	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// 1. GET /api/accounts must return redacted secrets (Issue 40)
	resp, err := http.Get(baseURL + "/api/accounts")
	if err != nil {
		t.Fatalf("GET /api/accounts failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Ensure raw body never contains any plaintext secret
	for _, secret := range []string{
		"SuperSecretPassword123!",
		"rt_sensitive_google_token_999",
		"ya29.sensitive_access_token_888",
		"JBSWY3DPEHPK3PXP",
	} {
		if strings.Contains(bodyStr, secret) {
			t.Fatalf("leak detected: response body contains plaintext secret %q", secret)
		}
	}

	var accounts []keyring.Account
	if err := json.Unmarshal(body, &accounts); err != nil {
		t.Fatalf("unmarshal accounts failed: %v", err)
	}
	if len(accounts) == 0 {
		t.Fatalf("expected at least 1 account")
	}

	var target *keyring.Account
	for i := range accounts {
		if accounts[i].Email == "vault-test@example.com" {
			target = &accounts[i]
			break
		}
	}
	if target == nil {
		t.Fatalf("expected account vault-test@example.com not found")
	}
	if target.Password != "[REDACTED]" {
		t.Errorf("expected Password [REDACTED], got %q", target.Password)
	}
	if target.RefreshToken != "[REDACTED]" {
		t.Errorf("expected RefreshToken [REDACTED], got %q", target.RefreshToken)
	}
	if target.AccessToken != "[REDACTED]" {
		t.Errorf("expected AccessToken [REDACTED], got %q", target.AccessToken)
	}
	if target.TOTPSecret != "[REDACTED]" {
		t.Errorf("expected TOTPSecret [REDACTED], got %q", target.TOTPSecret)
	}

	// 2. PUT /api/accounts updating with [REDACTED] must PRESERVE existing secrets
	updatePayload := keyring.Account{
		Email:        "vault-test@example.com",
		Label:        "Updated Label with Preserved Secrets",
		Password:     "[REDACTED]",
		RefreshToken: "[REDACTED]",
		AccessToken:  "[REDACTED]",
		TOTPSecret:   "[REDACTED]",
	}
	payloadBytes, _ := json.Marshal(updatePayload)
	req, _ := http.NewRequest(http.MethodPut, baseURL+"/api/accounts", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	putResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/accounts failed: %v", err)
	}
	putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from PUT /api/accounts, got %d", putResp.StatusCode)
	}

	// Verify in storage that original secrets are preserved
	kStore2, err := keyring.NewStore(filepath.Join(tempDir, "accounts.json"))
	if err != nil {
		t.Fatalf("keyring.NewStore reload failed: %v", err)
	}
	reloaded, err := kStore2.GetAccount("vault-test@example.com")
	if err != nil {
		t.Fatalf("GetAccount failed: %v", err)
	}
	if reloaded.Label != "Updated Label with Preserved Secrets" {
		t.Errorf("expected updated label, got %q", reloaded.Label)
	}
	if reloaded.Password != "SuperSecretPassword123!" {
		t.Errorf("expected preserved Password, got %q", reloaded.Password)
	}
	if reloaded.RefreshToken != "rt_sensitive_google_token_999" {
		t.Errorf("expected preserved RefreshToken, got %q", reloaded.RefreshToken)
	}
	if reloaded.AccessToken != "ya29.sensitive_access_token_888" {
		t.Errorf("expected preserved AccessToken, got %q", reloaded.AccessToken)
	}
	if reloaded.TOTPSecret != "JBSWY3DPEHPK3PXP" {
		t.Errorf("expected preserved TOTPSecret, got %q", reloaded.TOTPSecret)
	}
}

func TestFilesPathTraversalJailing(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// 1. Path traversal reading system files (Issue 41)
	resp, err := http.Get(baseURL + "/api/files/read?path=/etc/hostname")
	if err != nil {
		t.Fatalf("GET /api/files/read failed: %v", err)
	}
	defer resp.Body.Close()
	var readRes map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&readRes)
	if readRes["success"] == true {
		t.Errorf("expected path traversal /etc/hostname to be rejected, got success=true")
	}

	// 2. Path traversal writing system files
	writePayload := map[string]string{
		"path":    "/etc/cron.d/evil_cron",
		"content": "* * * * * root reboot",
	}
	wb, _ := json.Marshal(writePayload)
	respW, err := http.Post(baseURL+"/api/files/write", "application/json", bytes.NewReader(wb))
	if err != nil {
		t.Fatalf("POST /api/files/write failed: %v", err)
	}
	defer respW.Body.Close()
	var writeRes map[string]interface{}
	_ = json.NewDecoder(respW.Body).Decode(&writeRes)
	if writeRes["success"] == true {
		t.Errorf("expected write to /etc/cron.d to be rejected, got success=true")
	}

	// 3. Path traversal deleting system files
	delPayload := map[string]string{
		"path": "/etc/hosts",
	}
	db, _ := json.Marshal(delPayload)
	respD, err := http.Post(baseURL+"/api/files/delete", "application/json", bytes.NewReader(db))
	if err != nil {
		t.Fatalf("POST /api/files/delete failed: %v", err)
	}
	defer respD.Body.Close()
	var delRes map[string]interface{}
	_ = json.NewDecoder(respD.Body).Decode(&delRes)
	if delRes["success"] == true {
		t.Errorf("expected delete of /etc/hosts to be rejected, got success=true")
	}

	// 4. Valid operations inside temp directory must succeed with 0600 permissions
	tmpFile := filepath.Join(t.TempDir(), "allowed_test.txt")
	wbValid, _ := json.Marshal(map[string]string{"path": tmpFile, "content": "valid payload"})
	respV, err := http.Post(baseURL+"/api/files/write", "application/json", bytes.NewReader(wbValid))
	if err != nil {
		t.Fatalf("POST valid file write failed: %v", err)
	}
	respV.Body.Close()

	fi, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("file was not written: %v", err)
	}
	if fi.Mode().Perm() != 0600 {
		t.Errorf("expected file mode 0600, got %#o", fi.Mode().Perm())
	}
}

func TestOpenIDECanonicalAllowlist(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()
	tmpDir := t.TempDir()

	// 1. Arbitrary executable spawn attempt (Issue 42)
	badPayload := map[string]string{
		"path": tmpDir,
		"ide":  "echo-pwned-marker",
	}
	bb, _ := json.Marshal(badPayload)
	resp, err := http.Post(baseURL+"/api/files/open_ide", "application/json", bytes.NewReader(bb))
	if err != nil {
		t.Fatalf("POST /api/files/open_ide failed: %v", err)
	}
	defer resp.Body.Close()
	var badRes map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&badRes)
	if badRes["success"] == true {
		t.Errorf("expected arbitrary IDE binary to be rejected, got success=true")
	}

	// 2. Command injection in IDE parameter
	injPayload := map[string]string{
		"path": tmpDir,
		"ide":  "code; touch /tmp/pwned",
	}
	ib, _ := json.Marshal(injPayload)
	respInj, err := http.Post(baseURL+"/api/files/open_ide", "application/json", bytes.NewReader(ib))
	if err != nil {
		t.Fatalf("POST injection failed: %v", err)
	}
	defer respInj.Body.Close()
	var injRes map[string]interface{}
	_ = json.NewDecoder(respInj.Body).Decode(&injRes)
	if injRes["success"] == true {
		t.Errorf("expected shell injection in IDE to be rejected, got success=true")
	}

	// 3. Setting disallowed IDE in config
	cfgPayload := map[string]string{
		"preferred_ide": "/bin/sh",
	}
	cb, _ := json.Marshal(cfgPayload)
	respCfg, err := http.Post(baseURL+"/api/files/ide/config", "application/json", bytes.NewReader(cb))
	if err != nil {
		t.Fatalf("POST /api/files/ide/config failed: %v", err)
	}
	defer respCfg.Body.Close()
	var cfgRes map[string]interface{}
	_ = json.NewDecoder(respCfg.Body).Decode(&cfgRes)
	if cfgRes["success"] == true {
		t.Errorf("expected disallowed preferred_ide to be rejected, got success=true")
	}

	// 4. Allowed IDEs (code, cursor, windsurf, etc.) must succeed
	goodPayload := map[string]string{
		"preferred_ide": "cursor",
	}
	gb, _ := json.Marshal(goodPayload)
	respGood, err := http.Post(baseURL+"/api/files/ide/config", "application/json", bytes.NewReader(gb))
	if err != nil {
		t.Fatalf("POST good IDE config failed: %v", err)
	}
	defer respGood.Body.Close()
	var goodRes map[string]interface{}
	_ = json.NewDecoder(respGood.Body).Decode(&goodRes)
	if goodRes["success"] != true || goodRes["preferred_ide"] != "cursor" {
		t.Errorf("expected cursor preferred_ide success, got %v", goodRes)
	}
}

func TestTOTPAuthProtection(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)

	cfg := core.DefaultConfig()
	if err := cfg.SetAppPassword("SecretMasterPass123!"); err != nil {
		t.Fatalf("SetAppPassword failed: %v", err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("cfg.Save failed: %v", err)
	}

	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// 1. Unauthenticated request to /api/totp must return 401 Unauthorized
	resp, err := http.Get(baseURL + "/api/totp?email=test@example.com")
	if err != nil {
		t.Fatalf("GET /api/totp failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for unauthenticated TOTP access, got %d", resp.StatusCode)
	}

	// 2. Authenticating via /api/auth/unlock returns session token
	unlockBody := map[string]string{"password": "SecretMasterPass123!"}
	ub, _ := json.Marshal(unlockBody)
	respUnlock, err := http.Post(baseURL+"/api/auth/unlock", "application/json", bytes.NewReader(ub))
	if err != nil {
		t.Fatalf("POST /api/auth/unlock failed: %v", err)
	}
	defer respUnlock.Body.Close()
	if respUnlock.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from unlock, got %d", respUnlock.StatusCode)
	}
	var unlockRes map[string]interface{}
	_ = json.NewDecoder(respUnlock.Body).Decode(&unlockRes)
	sessionToken, _ := unlockRes["session_token"].(string)
	if sessionToken == "" {
		t.Fatalf("expected non-empty session_token from unlock response")
	}

	// 3. Request with valid session token is authorized
	reqToken, _ := http.NewRequest(http.MethodGet, baseURL+"/api/totp?email=test@example.com", nil)
	reqToken.Header.Set("X-Session-Token", sessionToken)
	respToken, err := http.DefaultClient.Do(reqToken)
	if err != nil {
		t.Fatalf("GET /api/totp with token failed: %v", err)
	}
	respToken.Body.Close()
	if respToken.StatusCode == http.StatusUnauthorized {
		t.Errorf("expected authorized access with session token, got 401")
	}

	// 4. Request with X-App-Password header directly is also authorized
	reqDirect, _ := http.NewRequest(http.MethodGet, baseURL+"/api/totp?email=test@example.com", nil)
	reqDirect.Header.Set("X-App-Password", "SecretMasterPass123!")
	respDirect, err := http.DefaultClient.Do(reqDirect)
	if err != nil {
		t.Fatalf("GET /api/totp with direct password failed: %v", err)
	}
	respDirect.Body.Close()
	if respDirect.StatusCode == http.StatusUnauthorized {
		t.Errorf("expected authorized access with direct app password, got 401")
	}
}

func TestMaxBytesMiddlewareLimit(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// 11MB oversized body (exceeds 10MB max limit)
	oversizedBody := strings.Repeat("A", 11*1024*1024)
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/files/write", strings.NewReader(oversizedBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Connection reset or truncated read by server is acceptable
		return
	}
	defer resp.Body.Close()

	var res map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&res)
	if res["success"] == true {
		t.Errorf("expected oversized payload (>10MB) to be rejected or failed")
	}
}
