package webgui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

func TestHandleSettingsThemeGETDefault(t *testing.T) {
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", t.TempDir())
	srv := newTestServer("127.0.0.1:0", "")

	req := httptest.NewRequest(http.MethodGet, "/api/settings/theme", nil)
	rr := httptest.NewRecorder()
	srv.handleSettingsTheme(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	var payload struct {
		Theme string `json:"theme"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Theme != core.ThemeModeAuto {
		t.Errorf("expected default theme %q, got %q", core.ThemeModeAuto, payload.Theme)
	}
}

func TestHandleSettingsThemePOSTPersists(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)
	srv := newTestServer("127.0.0.1:0", "")

	for _, mode := range []string{core.ThemeModeDark, core.ThemeModeLight, core.ThemeModeAuto} {
		body, _ := json.Marshal(map[string]string{"theme": mode})
		req := httptest.NewRequest(http.MethodPost, "/api/settings/theme", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		srv.handleSettingsTheme(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("POST %q: expected status 200, got %d", mode, rr.Code)
		}
		var payload struct {
			Success bool   `json:"success"`
			Theme   string `json:"theme"`
		}
		if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if !payload.Success || payload.Theme != mode {
			t.Errorf("POST %q: expected success with theme %q, got %+v", mode, mode, payload)
		}

		// GET must reflect the persisted value.
		reqGet := httptest.NewRequest(http.MethodGet, "/api/settings/theme", nil)
		rrGet := httptest.NewRecorder()
		srv.handleSettingsTheme(rrGet, reqGet)
		var got struct {
			Theme string `json:"theme"`
		}
		if err := json.NewDecoder(rrGet.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode GET response: %v", err)
		}
		if got.Theme != mode {
			t.Errorf("after POST %q, GET returned %q", mode, got.Theme)
		}
	}

	// The config file on disk must carry the theme.
	data, err := os.ReadFile(filepath.Join(tempDir, "config.json"))
	if err != nil {
		t.Fatalf("failed to read persisted config: %v", err)
	}
	var persisted struct {
		Theme string `json:"theme"`
	}
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("failed to parse persisted config: %v", err)
	}
	if persisted.Theme != core.ThemeModeAuto {
		t.Errorf("expected persisted theme %q, got %q", core.ThemeModeAuto, persisted.Theme)
	}
}

func TestHandleSettingsThemePOSTRejectsInvalid(t *testing.T) {
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", t.TempDir())
	srv := newTestServer("127.0.0.1:0", "")

	body := []byte(`{"theme":"neon"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/settings/theme", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	srv.handleSettingsTheme(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for invalid theme, got %d", rr.Code)
	}
	var payload struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Success || payload.Error == "" {
		t.Errorf("expected failure with error message, got %+v", payload)
	}

	// Theme must remain at the default after a rejected write.
	cfg, err := core.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.GetTheme() != core.ThemeModeAuto {
		t.Errorf("expected theme to stay %q after rejection, got %q", core.ThemeModeAuto, cfg.GetTheme())
	}
}

func TestHandleSettingsThemeMethodNotAllowed(t *testing.T) {
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", t.TempDir())
	srv := newTestServer("127.0.0.1:0", "")

	req := httptest.NewRequest(http.MethodDelete, "/api/settings/theme", nil)
	rr := httptest.NewRecorder()
	srv.handleSettingsTheme(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", rr.Code)
	}
}
