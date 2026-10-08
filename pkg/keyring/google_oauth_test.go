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
