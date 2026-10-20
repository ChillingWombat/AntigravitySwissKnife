package webgui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRemoteConfigEndpointsExposeVisiblePIN(t *testing.T) {
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", t.TempDir())
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")

	srv := NewServer("127.0.0.1:0", "")
	srv.daemonSessionToken = "test-session-token"
	handler := srv.authMiddleware(srv.setupRoutes())

	for _, path := range []string{"/api/bridge/remote/config", "/api/bridge/remote/status"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Session-Token", srv.daemonSessionToken)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s returned HTTP %d, want 200", path, rec.Code)
		}

		var payload struct {
			Config struct {
				SecurityPIN string `json:"security_pin"`
			} `json:"config"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		if payload.Config.SecurityPIN == "" || payload.Config.SecurityPIN == "******" {
			t.Fatalf("%s returned hidden security PIN %q", path, payload.Config.SecurityPIN)
		}
	}
}
