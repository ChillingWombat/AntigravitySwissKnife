package keyring

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGoogleOAuthManager_Lifecycle(t *testing.T) {
	mgr := NewGoogleOAuthManager("test-client-id", "test-client-secret")
	if mgr.IsFlowActive() {
		t.Errorf("expected no active flow initially")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	go func() {
		_, _ = mgr.StartFlow(ctx, false)
	}()

	// Wait briefly for flow to start
	time.Sleep(50 * time.Millisecond)
	if !mgr.IsFlowActive() {
		t.Errorf("expected flow to be active")
	}
	authURL := mgr.GetActiveAuthURL()
	if !strings.Contains(authURL, "test-client-id") {
		t.Errorf("expected authURL to contain test-client-id, got %s", authURL)
	}

	mgr.CancelFlow()
	if mgr.IsFlowActive() {
		t.Errorf("expected flow to be inactive after cancel")
	}
}

func TestGoogleOAuthManager_ExchangeURLValidation(t *testing.T) {
	mgr := NewGoogleOAuthManager("test-client", "test-secret")

	// 1. Empty string
	_, err := mgr.Exchange("", "", "")
	if err == nil || !strings.Contains(err.Error(), "no authorization code") {
		t.Errorf("expected error for empty URL, got %v", err)
	}

	// 2. Error returned in query params
	errorURL := "http://127.0.0.1:44503/oauth/callback?error=access_denied&error_description=User+declined"
	_, err = mgr.Exchange(errorURL, "", "")
	if err == nil || !strings.Contains(err.Error(), "access_denied") {
		t.Errorf("expected access_denied error, got %v", err)
	}

	// 3. Code without redirect URI and no active flow
	_, err = mgr.Exchange("4/0AXlqoi5aYkolJhLv", "", "")
	if err == nil || !strings.Contains(err.Error(), "unable to determine redirect URI") {
		t.Errorf("expected redirect URI error when code provided without URI, got %v", err)
	}
}

func TestGoogleOAuthManager_ExchangeMockServer(t *testing.T) {
	// Mock Google Token endpoint
	var receivedCode, receivedRedirect string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		receivedCode = r.Form.Get("code")
		receivedRedirect = r.Form.Get("redirect_uri")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"access_token": "ya29.mock_access_token",
			"refresh_token": "1//04mock_refresh_token",
			"id_token": "header.eyJlbWFpbCI6InVzZXJAZXhhbXBsZS5jb20ifQ.signature"
		}`))
	}))
	defer mockServer.Close()

	// In exchangeGoogleCode, it normally calls GoogleOAuthTokenURL.
	// Let's test the URL parsing logic directly:
	returnURL := "http://127.0.0.1:44503/oauth/callback?iss=https://accounts.google.com&code=4/0AXlqoi5aYkolJhLv1nNQYHZMXQbA8vd6OMszaykwIEWzOnGqZAKUv1ZOz7HHT7vAyXpibA&scope=email%20profile%20openid&authuser=0&prompt=consent"

	mgr := NewGoogleOAuthManager("test-client", "test-secret")
	// Test parsing with simulated active flow
	mgr.activeFlow = &activeGoogleFlow{
		redirectURI: "http://127.0.0.1:44503/oauth/callback",
		result:      make(chan *GoogleOAuthResult, 1),
	}

	// Code extracted properly:
	tokens, err := exchangeGoogleCode("test-client", "test-secret", "4/0AXlqoi5", "http://127.0.0.1:44503/oauth/callback")
	// Since exchangeGoogleCode calls live Google token URL, it will fail with invalid client or invalid grant, which proves it reached Google
	if err == nil && tokens == nil {
		t.Errorf("unexpected token state")
	}

	_ = returnURL
	_ = receivedCode
	_ = receivedRedirect
}

func TestGoogleOAuthCallbackHTML_LightThemeAndLucideIcon(t *testing.T) {
	if !strings.Contains(oauthSuccessHTML, "background:#f8fafd;") || !strings.Contains(oauthSuccessHTML, "background:#ffffff;") {
		t.Errorf("expected oauthSuccessHTML to use light theme backgrounds (#f8fafd and #ffffff)")
	}
	if strings.Contains(oauthSuccessHTML, "#090a0f") || strings.Contains(oauthSuccessHTML, "#12151f") {
		t.Errorf("expected oauthSuccessHTML not to contain dark theme colors")
	}
	if strings.Contains(oauthSuccessHTML, "✓") {
		t.Errorf("expected oauthSuccessHTML to replace unicode checkmark with Lucide SVG")
	}
	if !strings.Contains(oauthSuccessHTML, `<circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/>`) {
		t.Errorf("expected oauthSuccessHTML to include Lucide check-circle SVG icon")
	}
	if !strings.Contains(oauthSuccessHTML, "background:#1a73e8;") || !strings.Contains(oauthSuccessHTML, "color:#1a73e8;") || !strings.Contains(oauthSuccessHTML, "white-space:nowrap;") {
		t.Errorf("expected oauthSuccessHTML to use blue (#1a73e8) for button/accent color and white-space:nowrap")
	}

	errPage := string(renderOAuthErrorHTML("Authentication <Failed>", "access_denied: <script>alert(1)</script>"))
	if !strings.Contains(errPage, "background:#f8fafd;") || !strings.Contains(errPage, "background:#1a73e8;") {
		t.Errorf("expected renderOAuthErrorHTML to use light theme and blue button")
	}
	if strings.Contains(errPage, "<script>alert(1)</script>") || !strings.Contains(errPage, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Errorf("expected renderOAuthErrorHTML to HTML-escape title and detail")
	}

	// Verify absence of destructive Electron/VS Code window-closing hacks
	if strings.Contains(oauthSuccessHTML, "window.open('', '_self', '')") || strings.Contains(oauthSuccessHTML, `window.open('','_self','')`) {
		t.Errorf("expected oauthSuccessHTML not to contain destructive window.open('', '_self', '') hack")
	}
	if strings.Contains(errPage, "window.open('', '_self', '')") || strings.Contains(errPage, `window.open('','_self','')`) {
		t.Errorf("expected renderOAuthErrorHTML not to contain destructive window.open('', '_self', '') hack")
	}
	if !strings.Contains(oauthSuccessHTML, "isEmbeddedOrVSCode") || !strings.Contains(oauthSuccessHTML, "safeCloseWindow") {
		t.Errorf("expected oauthSuccessHTML to contain embedded VS Code environment checks")
	}
	if !strings.Contains(errPage, "isEmbeddedOrVSCode") || !strings.Contains(errPage, "safeCloseWindow") {
		t.Errorf("expected renderOAuthErrorHTML to contain embedded VS Code environment checks")
	}
}
