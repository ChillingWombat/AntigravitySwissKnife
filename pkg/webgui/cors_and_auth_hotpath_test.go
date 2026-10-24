package webgui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

func TestHandleAccountUpdate_PreservesEmptyAndHonorsClear(t *testing.T) {
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	const (
		email            = "mfa-handler@example.com"
		originalPassword = "original-password"
		originalSecret   = "{{SECRET_handler}}"
	)

	store, err := keyring.NewStore("")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	seedAccount := func(t *testing.T) {
		t.Helper()
		if err := store.UpdateAccountFull(email, "MFA Handler", "Pro", "ACTIVE", "High", "", originalPassword, originalSecret, "", 0, false, false, false); err != nil {
			t.Fatalf("seed account: %v", err)
		}
	}
	seedAccount(t)

	srv := NewServer("127.0.0.1:0", filepath.Join(home, "missing-ipc.sock"))
	updateCredentials := func(t *testing.T, value string) {
		t.Helper()
		body := fmt.Sprintf(`{"email":%q,"password":%q,"totp_secret":%q}`, email, value, value)
		req := httptest.NewRequest(http.MethodPost, "/api/gui/accounts/update", strings.NewReader(body))
		rec := httptest.NewRecorder()
		srv.handleAccountUpdate(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("update status = %d, body = %q", rec.Code, rec.Body.String())
		}
	}

	reloadAccount := func(t *testing.T) *keyring.Account {
		t.Helper()
		reloaded, err := keyring.NewStore("")
		if err != nil {
			t.Fatalf("reload store: %v", err)
		}
		account, err := reloaded.GetAccount(email)
		if err != nil {
			t.Fatalf("reload account: %v", err)
		}
		if account == nil {
			t.Fatal("account missing after handler update")
		}
		return account
	}

	assertCredentials := func(t *testing.T, wantPassword, wantTOTPSecret string) {
		t.Helper()
		account := reloadAccount(t)
		if account.Password != wantPassword {
			t.Fatalf("password = %q, want %q", account.Password, wantPassword)
		}
		if account.TOTPSecret != wantTOTPSecret {
			t.Fatalf("totp_secret = %q, want %q", account.TOTPSecret, wantTOTPSecret)
		}
	}

	updateCredentials(t, "")
	assertCredentials(t, originalPassword, originalSecret)

	updateCredentials(t, "[REDACTED]")
	assertCredentials(t, originalPassword, originalSecret)

	updateCredentials(t, "[CLEAR]")
	assertCredentials(t, "", "")

	seedAccount(t)
	updateCredentials(t, "CLEAR")
	assertCredentials(t, "", "")
}

func TestCORS_VaryAndCredentials(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	handler := srv.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	t.Run("loopback origin gets credentials and vary", func(t *testing.T) {
		loopbackOrigins := []string{
			"http://127.0.0.1:8765",
			"http://localhost:5173",
			"http://[::1]:8765",
		}

		for _, origin := range loopbackOrigins {
			req := httptest.NewRequest(http.MethodGet, "/api/gui/config", nil)
			req.Header.Set("Origin", origin)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("origin %q: expected status 200, got %d", origin, rec.Code)
			}
			if gotVary := rec.Header().Get("Vary"); gotVary != "Origin" {
				t.Errorf("origin %q: expected Vary: Origin, got %q", origin, gotVary)
			}
			if gotOrigin := rec.Header().Get("Access-Control-Allow-Origin"); gotOrigin != origin {
				t.Errorf("origin %q: expected Allow-Origin %q, got %q", origin, origin, gotOrigin)
			}
			if gotCreds := rec.Header().Get("Access-Control-Allow-Credentials"); gotCreds != "true" {
				t.Errorf("origin %q: expected Allow-Credentials: true, got %q", origin, gotCreds)
			}

			// Preflight OPTIONS test
			reqOpt := httptest.NewRequest(http.MethodOptions, "/api/gui/config", nil)
			reqOpt.Header.Set("Origin", origin)
			recOpt := httptest.NewRecorder()
			handler.ServeHTTP(recOpt, reqOpt)

			if recOpt.Code != http.StatusNoContent {
				t.Errorf("origin %q OPTIONS: expected status 204, got %d", origin, recOpt.Code)
			}
			if gotVary := recOpt.Header().Get("Vary"); gotVary != "Origin" {
				t.Errorf("origin %q OPTIONS: expected Vary: Origin, got %q", origin, gotVary)
			}
			if gotCreds := recOpt.Header().Get("Access-Control-Allow-Credentials"); gotCreds != "true" {
				t.Errorf("origin %q OPTIONS: expected Allow-Credentials: true, got %q", origin, gotCreds)
			}
		}
	})

	t.Run("non-loopback origin on external allowed path echoes origin but omits credentials", func(t *testing.T) {
		externalPaths := []string{
			"/remote",
			"/remote/session",
			"/oauth",
			"/oauth/callback",
			"/mcp",
			"/mcp/stream",
		}

		externalOrigin := "https://external.example.com"
		for _, path := range externalPaths {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Origin", externalOrigin)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("path %q: expected status 200, got %d", path, rec.Code)
			}
			if gotVary := rec.Header().Get("Vary"); gotVary != "Origin" {
				t.Errorf("path %q: expected Vary: Origin, got %q", path, gotVary)
			}
			if gotOrigin := rec.Header().Get("Access-Control-Allow-Origin"); gotOrigin != externalOrigin {
				t.Errorf("path %q: expected Allow-Origin %q, got %q", path, externalOrigin, gotOrigin)
			}
			if gotCreds := rec.Header().Get("Access-Control-Allow-Credentials"); gotCreds != "" {
				t.Errorf("path %q: expected empty Allow-Credentials for non-loopback origin, got %q", path, gotCreds)
			}

			// Preflight OPTIONS test
			reqOpt := httptest.NewRequest(http.MethodOptions, path, nil)
			reqOpt.Header.Set("Origin", externalOrigin)
			recOpt := httptest.NewRecorder()
			handler.ServeHTTP(recOpt, reqOpt)

			if recOpt.Code != http.StatusNoContent {
				t.Errorf("path %q OPTIONS: expected status 204, got %d", path, recOpt.Code)
			}
			if gotVary := recOpt.Header().Get("Vary"); gotVary != "Origin" {
				t.Errorf("path %q OPTIONS: expected Vary: Origin, got %q", path, gotVary)
			}
			if gotCreds := recOpt.Header().Get("Access-Control-Allow-Credentials"); gotCreds != "" {
				t.Errorf("path %q OPTIONS: expected empty Allow-Credentials for non-loopback origin, got %q", path, gotCreds)
			}
		}
	})

	t.Run("non-loopback origin on internal path is forbidden with vary header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/gui/config", nil)
		req.Header.Set("Origin", "https://attacker.example.com")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d", rec.Code)
		}
		if gotVary := rec.Header().Get("Vary"); gotVary != "Origin" {
			t.Errorf("expected Vary: Origin on forbidden origin, got %q", gotVary)
		}

		// Preflight OPTIONS test
		reqOpt := httptest.NewRequest(http.MethodOptions, "/api/gui/config", nil)
		reqOpt.Header.Set("Origin", "https://attacker.example.com")
		recOpt := httptest.NewRecorder()
		handler.ServeHTTP(recOpt, reqOpt)

		if recOpt.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for OPTIONS on internal path, got %d", recOpt.Code)
		}
		if gotVary := recOpt.Header().Get("Vary"); gotVary != "Origin" {
			t.Errorf("expected Vary: Origin on forbidden OPTIONS origin, got %q", gotVary)
		}
	})
}

func TestCORS_PreservesExistingVary(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	handler := srv.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, existing := range []string{"Accept-Encoding", "Accept-Encoding, Origin"} {
		t.Run(existing, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/gui/config", nil)
			req.Header.Set("Origin", "http://127.0.0.1:8765")
			rec := httptest.NewRecorder()
			rec.Header().Set("Vary", existing)

			handler.ServeHTTP(rec, req)

			originCount := 0
			values := rec.Header().Values("Vary")
			for _, line := range values {
				for _, value := range strings.Split(line, ",") {
					switch {
					case strings.EqualFold(strings.TrimSpace(value), "Origin"):
						originCount++
					case strings.EqualFold(strings.TrimSpace(value), "Accept-Encoding"):
					default:
						t.Fatalf("unexpected Vary value %q", value)
					}
				}
			}
			if originCount != 1 {
				t.Fatalf("Vary values = %q, want exactly one Origin", values)
			}
			if !strings.Contains(strings.Join(values, ","), "Accept-Encoding") {
				t.Fatalf("existing Vary header was lost: %q", values)
			}
		})
	}
}

func TestAuthHotPath_LazyConfigLoad(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)
	// Write a corrupt config that would fail LoadConfig()
	_ = os.WriteFile(filepath.Join(tempDir, "config.json"), []byte("{corrupt-json"), 0600)

	originalLoad := loadConfigForAuth
	loads := 0
	loadConfigForAuth = func() (*core.Config, error) {
		loads++
		return core.LoadConfig()
	}
	t.Cleanup(func() {
		loadConfigForAuth = originalLoad
	})

	originalVerify := verifyAppPasswordForAuth
	verifications := 0
	verifyAppPasswordForAuth = func(cfg *core.Config, candidate string) bool {
		verifications++
		return false
	}
	t.Cleanup(func() {
		verifyAppPasswordForAuth = originalVerify
	})

	srv := NewServer("127.0.0.1:0", "")

	// 1. A stale session-shaped token must fail closed before config load or PBKDF2.
	staleToken := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	reqStale := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	reqStale.Header.Set("Authorization", "Bearer "+staleToken)
	if srv.validateRequestAuth(reqStale) {
		t.Fatal("expected validateRequestAuth to return false immediately for stale session token without disk load")
	}

	// 2. Unauthenticated request without credentials must not load config either.
	req := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	if srv.validateRequestAuth(req) {
		t.Fatal("expected validateRequestAuth to return false immediately when no credentials provided")
	}

	if loads != 0 {
		t.Fatalf("LoadConfig calls = %d, want 0", loads)
	}
	if verifications != 0 {
		t.Fatalf("PBKDF2 verifications = %d, want 0", verifications)
	}
}

func TestAuthHotPath_SessionTokenShapedCandidateBypassesPBKDF2(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)

	tokenShapedPassword := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cfg := core.DefaultConfig()
	if err := cfg.SetAppPassword(tokenShapedPassword); err != nil {
		t.Fatalf("SetAppPassword: %v", err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save config: %v", err)
	}

	srv := NewServer("127.0.0.1:0", "")

	// 1. Candidate shaped like a session token in Authorization header should NOT be verified as app password
	reqBearer := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	reqBearer.Header.Set("Authorization", "Bearer "+tokenShapedPassword)
	if srv.validateRequestAuth(reqBearer) {
		t.Errorf("expected token-shaped candidate to bypass PBKDF2 and return false")
	}

	// 2. Candidate shaped like a session token in X-Session-Token should NOT be verified as app password
	reqSess := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	reqSess.Header.Set("X-Session-Token", tokenShapedPassword)
	if srv.validateRequestAuth(reqSess) {
		t.Errorf("expected token-shaped candidate in X-Session-Token to bypass PBKDF2 and return false")
	}

	// 3. Candidate passed explicitly as X-App-Password SHOULD be verified
	reqAppPass := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	reqAppPass.Header.Set("X-App-Password", tokenShapedPassword)
	if !srv.validateRequestAuth(reqAppPass) {
		t.Errorf("expected X-App-Password candidate to verify successfully")
	}

	// 4. Non-token-shaped candidate in Authorization header SHOULD verify via app password
	normalPassword := "plain-app-password"
	if err := cfg.SetAppPassword(normalPassword); err != nil {
		t.Fatalf("SetAppPassword(normal): %v", err)
	}
	_ = cfg.Save()

	reqNormalBearer := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	reqNormalBearer.Header.Set("Authorization", "Bearer "+normalPassword)
	if !srv.validateRequestAuth(reqNormalBearer) {
		t.Errorf("expected non-token-shaped candidate to verify via app password")
	}
}

func TestAuthHotPath_AppPasswordVerificationIsRateLimited(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)
	cfg := core.DefaultConfig()
	if err := cfg.SetAppPassword("legacy-password"); err != nil {
		t.Fatalf("SetAppPassword: %v", err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save config: %v", err)
	}

	originalVerify := verifyAppPasswordForAuth
	verifications := 0
	verifyAppPasswordForAuth = func(cfg *core.Config, candidate string) bool {
		verifications++
		return false
	}
	t.Cleanup(func() {
		verifyAppPasswordForAuth = originalVerify
	})

	testCases := []struct {
		name  string
		setup func(*http.Request, int)
	}{
		{
			name: "legacy bearer token",
			setup: func(req *http.Request, i int) {
				req.Header.Set("Authorization", fmt.Sprintf("Bearer invalid-token-%d", i))
			},
		},
		{
			name: "app password header",
			setup: func(req *http.Request, i int) {
				req.Header.Set("X-App-Password", fmt.Sprintf("invalid-password-%d", i))
			},
		},
		{
			name: "app password with legacy bearer",
			setup: func(req *http.Request, i int) {
				req.Header.Set("Authorization", fmt.Sprintf("Bearer invalid-token-%d", i))
				req.Header.Set("X-App-Password", fmt.Sprintf("invalid-password-%d", i))
			},
		},
		{
			name: "app password with stale session-shaped bearer",
			setup: func(req *http.Request, i int) {
				req.Header.Set("Authorization", "Bearer 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
				req.Header.Set("X-App-Password", fmt.Sprintf("invalid-password-%d", i))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			verifications = 0
			srv := NewServer("127.0.0.1:0", "")
			for i := 0; i < legacyAppPasswordAttemptLimit*3; i++ {
				req := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
				req.RemoteAddr = "198.51.100.20:4321"
				testCase.setup(req, i)
				if srv.validateRequestAuth(req) {
					t.Fatalf("invalid credentials %d were accepted", i)
				}
			}

			if verifications != legacyAppPasswordAttemptLimit {
				t.Fatalf("PBKDF2 verifications = %d, want bounded at %d", verifications, legacyAppPasswordAttemptLimit)
			}
		})
	}
}

func TestAuthHotPath_AuthUnlockRateLimitsInvalidPasswordAttempts(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)
	cfg := core.DefaultConfig()
	if err := cfg.SetAppPassword("correct-password"); err != nil {
		t.Fatalf("SetAppPassword: %v", err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save config: %v", err)
	}

	srv := NewServer("127.0.0.1:0", "")
	const limitedIP = "198.51.100.40:4321"
	for i := 0; i < legacyAppPasswordAttemptLimit; i++ {
		req := httptest.NewRequest(
			http.MethodPost,
			fmt.Sprintf("/api/auth/unlock?password=invalid-%d", i),
			strings.NewReader(fmt.Sprintf("{\"password\":\"invalid-%d\"}", i)),
		)
		req.RemoteAddr = limitedIP
		rec := httptest.NewRecorder()
		srv.handleAuthUnlock(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("attempt %d status = %d, want 200 before limit", i, rec.Code)
		}
	}

	blockedReq := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/api/auth/unlock?password=invalid-%d", legacyAppPasswordAttemptLimit),
		strings.NewReader(fmt.Sprintf("{\"password\":\"invalid-%d\"}", legacyAppPasswordAttemptLimit)),
	)
	blockedReq.RemoteAddr = limitedIP
	blockedRec := httptest.NewRecorder()
	srv.handleAuthUnlock(blockedRec, blockedReq)
	if blockedRec.Code != http.StatusTooManyRequests {
		t.Fatalf("attempt past limit status = %d, want 429", blockedRec.Code)
	}

	otherIPReq := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/unlock?password=invalid-other-ip",
		strings.NewReader(`{"password":"invalid-other-ip"}`),
	)
	otherIPReq.RemoteAddr = "198.51.100.41:4321"
	otherIPRec := httptest.NewRecorder()
	srv.handleAuthUnlock(otherIPRec, otherIPReq)
	if otherIPRec.Code != http.StatusOK {
		t.Fatalf("other IP status = %d, want 200 with independent limit", otherIPRec.Code)
	}
}

func TestAuthHotPath_QueryPasswordRateLimitIsPerIP(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)
	cfg := core.DefaultConfig()
	if err := cfg.SetAppPassword("legacy-password"); err != nil {
		t.Fatalf("SetAppPassword: %v", err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save config: %v", err)
	}

	originalVerify := verifyAppPasswordForAuth
	verifications := 0
	verifyAppPasswordForAuth = func(cfg *core.Config, candidate string) bool {
		verifications++
		return false
	}
	t.Cleanup(func() {
		verifyAppPasswordForAuth = originalVerify
	})

	srv := NewServer("127.0.0.1:0", "")
	const limitedIP = "198.51.100.50:4321"
	for i := 0; i < legacyAppPasswordAttemptLimit; i++ {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/accounts?password=invalid-%d", i), nil)
		req.RemoteAddr = limitedIP
		if srv.validateRequestAuth(req) {
			t.Fatalf("invalid query password %d was accepted", i)
		}
	}
	if verifications != legacyAppPasswordAttemptLimit {
		t.Fatalf("query password verifications = %d, want %d", verifications, legacyAppPasswordAttemptLimit)
	}

	otherIPReq := httptest.NewRequest(http.MethodGet, "/api/accounts?password=invalid-other-ip", nil)
	otherIPReq.RemoteAddr = "198.51.100.51:4321"
	if srv.validateRequestAuth(otherIPReq) {
		t.Fatal("invalid query password from another IP was accepted")
	}
	if verifications != legacyAppPasswordAttemptLimit+1 {
		t.Fatalf("verifications after other IP = %d, want %d", verifications, legacyAppPasswordAttemptLimit+1)
	}
}

func TestAuthHotPath_PruneExpiredSessionTokens(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")

	t.Run("lookup prunes expired token", func(t *testing.T) {
		expiredToken := "expiredtoken1234567890123456789012345678901234567890123456789012"
		srv.authMu.Lock()
		srv.sessionTokens[expiredToken] = time.Now().Add(-1 * time.Hour)
		srv.authMu.Unlock()

		req := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
		req.Header.Set("X-Session-Token", expiredToken)
		if srv.validateRequestAuth(req) {
			t.Fatal("expected expired token to fail validation")
		}

		srv.authMu.RLock()
		_, exists := srv.sessionTokens[expiredToken]
		srv.authMu.RUnlock()
		if exists {
			t.Errorf("expected expired token to be pruned from sessionTokens map on lookup")
		}
	})

	t.Run("createSessionToken prunes expired tokens", func(t *testing.T) {
		expired1 := "exp1_123456789012345678901234567890123456789012345678901234567890"
		expired2 := "exp2_123456789012345678901234567890123456789012345678901234567890"
		active := "act1_123456789012345678901234567890123456789012345678901234567890"

		srv.authMu.Lock()
		srv.sessionTokens[expired1] = time.Now().Add(-2 * time.Hour)
		srv.sessionTokens[expired2] = time.Now().Add(-10 * time.Minute)
		srv.sessionTokens[active] = time.Now().Add(2 * time.Hour)
		srv.authMu.Unlock()

		newTok, err := srv.createSessionToken()
		if err != nil {
			t.Fatalf("createSessionToken: %v", err)
		}

		srv.authMu.RLock()
		defer srv.authMu.RUnlock()

		if _, ok := srv.sessionTokens[expired1]; ok {
			t.Errorf("expired1 was not pruned during createSessionToken")
		}
		if _, ok := srv.sessionTokens[expired2]; ok {
			t.Errorf("expired2 was not pruned during createSessionToken")
		}
		if _, ok := srv.sessionTokens[active]; !ok {
			t.Errorf("active token was unexpectedly pruned")
		}
		if _, ok := srv.sessionTokens[newTok]; !ok {
			t.Errorf("new token was not stored")
		}
	})
}

func TestAuthUnlock_CorruptConfig_FailsClosed(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)
	_ = os.WriteFile(filepath.Join(tempDir, "config.json"), []byte("{corrupt-json"), 0600)

	srv := NewServer("127.0.0.1:0", "")
	srv.authMu.RLock()
	initialCount := len(srv.sessionTokens)
	srv.authMu.RUnlock()

	req := httptest.NewRequest(http.MethodPost, "/api/auth/unlock", strings.NewReader(`{"password":"test"}`))
	rec := httptest.NewRecorder()

	srv.handleAuthUnlock(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected HTTP 500 when config is corrupt, got %d", rec.Code)
	}

	srv.authMu.RLock()
	defer srv.authMu.RUnlock()
	if len(srv.sessionTokens) != initialCount {
		t.Fatalf("expected no new session tokens issued on corrupt config, had %d, now %d", initialCount, len(srv.sessionTokens))
	}
	if strings.Contains(rec.Body.String(), "token") {
		t.Fatalf("expected response body not to contain any session token, got: %s", rec.Body.String())
	}
}
