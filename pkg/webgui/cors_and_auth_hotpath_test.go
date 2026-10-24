package webgui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

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

func TestAuthHotPath_LazyConfigLoad(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)
	// Write a corrupt config that would fail LoadConfig()
	_ = os.WriteFile(filepath.Join(tempDir, "config.json"), []byte("{corrupt-json"), 0600)

	srv := NewServer("127.0.0.1:0", "")

	// 1. Unauthenticated request without credentials: must not load config / fail closed immediately
	req := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	if srv.validateRequestAuth(req) {
		t.Fatal("expected validateRequestAuth to return false immediately when no credentials provided")
	}

	// 2. Request with stale session token: must not load config / fail closed immediately
	staleToken := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	reqStale := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	reqStale.Header.Set("Authorization", "Bearer "+staleToken)
	if srv.validateRequestAuth(reqStale) {
		t.Fatal("expected validateRequestAuth to return false immediately for stale session token without disk load")
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

		newTok := srv.createSessionToken()

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
