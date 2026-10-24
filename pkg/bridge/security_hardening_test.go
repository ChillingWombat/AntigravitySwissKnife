package bridge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBridgeConfigWritesArePrivate(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("HOME", configRoot)
	t.Setenv("USERPROFILE", configRoot)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(configRoot, "xdg"))

	tests := []struct {
		name   string
		save   func(t *testing.T, dir string)
		config string
	}{
		{
			name:   "bridge",
			config: "gemini_bridge.json",
			save: func(t *testing.T, dir string) {
				t.Helper()
				svc, err := NewService(filepath.Join(dir, "gemini_bridge.json"))
				if err != nil {
					t.Fatalf("NewService: %v", err)
				}
				if err := svc.UpdateConfig(svc.GetConfig()); err != nil {
					t.Fatalf("UpdateConfig: %v", err)
				}
			},
		},
		{
			name:   "remote",
			config: "remote_control.json",
			save: func(t *testing.T, dir string) {
				t.Helper()
				rm := NewRemoteManager(filepath.Join(dir, "remote_control.json"))
				cfg := DefaultRemoteConfig()
				if err := rm.UpdateConfig(cfg); err != nil {
					t.Fatalf("UpdateConfig: %v", err)
				}
			},
		},
		{
			name:   "channels",
			config: "chat_channels.json",
			save: func(t *testing.T, dir string) {
				t.Helper()
				cm := NewChannelsManager(filepath.Join(dir, "chat_channels.json"))
				if err := cm.UpdateConfig(DefaultChannelsConfig()); err != nil {
					t.Fatalf("UpdateConfig: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "config")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatalf("MkdirAll: %v", err)
			}
			tt.save(t, dir)

			dirInfo, err := os.Stat(dir)
			if err != nil {
				t.Fatalf("Stat config dir: %v", err)
			}
			if runtime.GOOS != "windows" {
				if got := dirInfo.Mode().Perm(); got != 0700 {
					t.Fatalf("config dir mode = %04o, want 0700", got)
				}
			}

			fileInfo, err := os.Stat(filepath.Join(dir, tt.config))
			if err != nil {
				t.Fatalf("Stat config file: %v", err)
			}
			if runtime.GOOS != "windows" {
				if got := fileInfo.Mode().Perm(); got != 0600 {
					t.Fatalf("config file mode = %04o, want 0600", got)
				}
			}
		})
	}
}

func TestServiceUpdateConfigUpdatesLiveAuthManager(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("HOME", configRoot)
	t.Setenv("USERPROFILE", configRoot)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(configRoot, "xdg"))

	svc, err := NewService(filepath.Join(t.TempDir(), "bridge.json"))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	cfg := svc.GetConfig()
	cfg.AuthToken = "updated-live-token"
	cfg.PairingPIN = "NewPin42"
	if err := svc.UpdateConfig(cfg); err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}

	tokenReq := httptest.NewRequest(http.MethodPost, "/api/bridge/mcp", nil)
	tokenReq.RemoteAddr = "192.0.2.10:1234"
	tokenReq.Header.Set("Authorization", "Bearer updated-live-token")
	if granted, reason := svc.authMgr.ValidateRequest(tokenReq); !granted {
		t.Fatalf("updated token was not accepted: %s", reason)
	}

	pinReq := httptest.NewRequest(http.MethodPost, "/api/bridge/mcp?pin=NewPin42", nil)
	pinReq.RemoteAddr = "192.0.2.11:1234"
	if granted, reason := svc.authMgr.ValidateRequest(pinReq); !granted {
		t.Fatalf("updated PIN was not accepted: %s", reason)
	}
}

func TestRemoteSessionCookieIsOpaqueAndDirectPINsStillWork(t *testing.T) {
	rm := NewRemoteManager(filepath.Join(t.TempDir(), "remote_control.json"))
	cfg := rm.GetConfig()
	cfg.SecurityPIN = "A7k9M2xP"
	if err := rm.UpdateConfig(cfg); err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"pin": cfg.SecurityPIN})
	authReq := httptest.NewRequest(http.MethodPost, "/remote/api/auth", strings.NewReader(string(body)))
	authReq.RemoteAddr = "192.0.2.20:1234"
	authRec := httptest.NewRecorder()
	rm.handleAuth(authRec, authReq)
	if authRec.Code != http.StatusOK {
		t.Fatalf("valid PIN auth status = %d (%s)", authRec.Code, authRec.Body.String())
	}

	var sessionCookie *http.Cookie
	for _, cookie := range authRec.Result().Cookies() {
		if cookie.Name == "swiss_remote_pin" {
			sessionCookie = cookie
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("swiss_remote_pin cookie was not set")
	}
	if sessionCookie.Value == cfg.SecurityPIN || sessionCookie.Value == "" {
		t.Fatalf("session cookie exposed the raw PIN: %q", sessionCookie.Value)
	}

	cookieReq := httptest.NewRequest(http.MethodGet, "/remote/api/status", nil)
	cookieReq.RemoteAddr = "192.0.2.21:1234"
	cookieReq.AddCookie(sessionCookie)
	if !rm.isPINAuthorized(cookieReq) {
		t.Fatal("opaque session cookie was rejected")
	}

	directCookieReq := httptest.NewRequest(http.MethodGet, "/remote/api/status", nil)
	directCookieReq.RemoteAddr = "192.0.2.22:1234"
	directCookieReq.AddCookie(&http.Cookie{Name: "swiss_remote_pin", Value: cfg.SecurityPIN})
	if !rm.isPINAuthorized(directCookieReq) {
		t.Fatal("direct PIN cookie fallback was rejected")
	}

	headerReq := httptest.NewRequest(http.MethodGet, "/remote/api/status", nil)
	headerReq.RemoteAddr = "192.0.2.23:1234"
	headerReq.Header.Set("X-Remote-PIN", cfg.SecurityPIN)
	if !rm.isPINAuthorized(headerReq) {
		t.Fatal("direct X-Remote-PIN fallback was rejected")
	}
}

func TestRemotePINChecksEnforceLockout(t *testing.T) {
	rm := NewRemoteManager(filepath.Join(t.TempDir(), "remote_control.json"))
	cfg := rm.GetConfig()
	cfg.SecurityPIN = "A7k9M2xP"
	if err := rm.UpdateConfig(cfg); err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/remote/api/status", nil)
		req.RemoteAddr = "192.0.2.30:1234"
		req.Header.Set("X-Remote-PIN", "wrongPIN")
		if rm.isPINAuthorized(req) {
			t.Fatal("wrong PIN was accepted")
		}
	}

	validReq := httptest.NewRequest(http.MethodGet, "/remote/api/status", nil)
	validReq.RemoteAddr = "192.0.2.30:1234"
	validReq.Header.Set("X-Remote-PIN", cfg.SecurityPIN)
	if rm.isPINAuthorized(validReq) {
		t.Fatal("locked client was accepted with the correct PIN")
	}
}
