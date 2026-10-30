package webgui

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

// TestChallenger_GoogleOAuthExchange_ReplayAttack verifies that an authorization
// code or callback URL with a valid state token cannot be submitted more than once
// to /api/oauth/google/exchange. The second attempt MUST be rejected with HTTP 400.
func TestChallenger_GoogleOAuthExchange_ReplayAttack(t *testing.T) {
	srv := newTestServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()
	token := srv.GetSessionToken()

	// 1. Initiate flow to register a valid active state
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		_, _ = srv.oauthMgr.StartFlow(ctx, false)
	}()

	time.Sleep(50 * time.Millisecond)
	if !srv.oauthMgr.IsFlowActive() {
		t.Fatalf("expected flow to be active")
	}
	defer srv.oauthMgr.CancelFlow()

	activeState := srv.oauthMgr.GetActiveState()
	if activeState == "" {
		t.Fatalf("expected active state to be present")
	}

	// 2. First exchange attempt using the valid state
	// Note: It will fail at Google token exchange (since code is fake), BUT it consumes the state!
	payload := map[string]string{
		"callback_url": "http://127.0.0.1:44503/oauth/callback?state=" + activeState + "&code=fake_google_code_1",
		"state":        activeState,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/oauth/google/exchange", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-Token", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send first exchange: %v", err)
	}
	defer resp.Body.Close()

	var res1 map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&res1)
	// First exchange consumes the state.

	// 3. Second exchange attempt with the EXACT SAME state -> MUST return HTTP 400 with state verification failure
	bodyReplay, _ := json.Marshal(payload)
	reqReplay, _ := http.NewRequest(http.MethodPost, baseURL+"/api/oauth/google/exchange", bytes.NewReader(bodyReplay))
	reqReplay.Header.Set("Content-Type", "application/json")
	reqReplay.Header.Set("X-Session-Token", token)
	respReplay, err := http.DefaultClient.Do(reqReplay)
	if err != nil {
		t.Fatalf("failed to send replayed exchange: %v", err)
	}
	defer respReplay.Body.Close()

	if respReplay.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for replayed state, got: %d", respReplay.StatusCode)
	}

	var resReplay map[string]interface{}
	_ = json.NewDecoder(respReplay.Body).Decode(&resReplay)
	errMsg, _ := resReplay["error"].(string)
	if !strings.Contains(errMsg, "oauth state verification failed") {
		t.Fatalf("expected error to mention 'oauth state verification failed', got: %s", errMsg)
	}
}

// TestChallenger_GoogleOAuthExchange_ConcurrentTampering tests that concurrent
// exchange requests racing on the same state parameter only allow one candidate through.
func TestChallenger_GoogleOAuthExchange_ConcurrentTampering(t *testing.T) {
	srv := newTestServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()
	token := srv.GetSessionToken()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		_, _ = srv.oauthMgr.StartFlow(ctx, false)
	}()

	time.Sleep(50 * time.Millisecond)
	if !srv.oauthMgr.IsFlowActive() {
		t.Fatalf("expected flow to be active")
	}
	defer srv.oauthMgr.CancelFlow()

	activeState := srv.oauthMgr.GetActiveState()
	payload := map[string]string{
		"callback_url": "http://127.0.0.1:44503/oauth/callback?state=" + activeState + "&code=fake_code",
		"state":        activeState,
	}

	concurrency := 20
	var wg sync.WaitGroup
	wg.Add(concurrency)

	csrfFailureCount := 0
	var mu sync.Mutex

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			body, _ := json.Marshal(payload)
			req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/oauth/google/exchange", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Session-Token", token)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			var res map[string]interface{}
			_ = json.NewDecoder(resp.Body).Decode(&res)
			errStr, _ := res["error"].(string)

			if strings.Contains(errStr, "oauth state verification failed") {
				mu.Lock()
				csrfFailureCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	// Exactly concurrency - 1 requests MUST fail with CSRF state verification failure
	if csrfFailureCount < concurrency-1 {
		t.Fatalf("expected at least %d CSRF state verification failures, got %d", concurrency-1, csrfFailureCount)
	}
}

// TestChallenger_GoogleOAuthURL_StateEntropy verifies that /api/oauth/google/url
// generates high-entropy 64-character hexadecimal states.
func TestChallenger_GoogleOAuthURL_StateEntropy(t *testing.T) {
	srv := newTestServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()
	token := srv.GetSessionToken()

	// 1. Without active flow, active should be false and state empty
	req1, _ := http.NewRequest(http.MethodGet, baseURL+"/api/oauth/google/url", nil)
	req1.Header.Set("X-Session-Token", token)
	resp, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatalf("GET /api/oauth/google/url failed: %v", err)
	}
	defer resp.Body.Close()

	var res map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&res)
	if res["active"] != false {
		t.Fatalf("expected active false when no flow is active, got %v", res["active"])
	}

	// 2. Start flow and check entropy
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		_, _ = srv.oauthMgr.StartFlow(ctx, false)
	}()

	time.Sleep(50 * time.Millisecond)
	defer srv.oauthMgr.CancelFlow()

	req2, _ := http.NewRequest(http.MethodGet, baseURL+"/api/oauth/google/url", nil)
	req2.Header.Set("X-Session-Token", token)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("GET /api/oauth/google/url during active flow failed: %v", err)
	}
	defer resp2.Body.Close()

	var res2 map[string]interface{}
	_ = json.NewDecoder(resp2.Body).Decode(&res2)
	if res2["active"] != true {
		t.Fatalf("expected active true")
	}

	state, ok := res2["state"].(string)
	if !ok || len(state) != 64 {
		t.Fatalf("expected 64-character hex state token, got %q (len=%d)", state, len(state))
	}
}

// TestChallenger_AccountUpdate_TOTPPreservationViaHTTP tests that calling
// POST /api/accounts/update with [REDACTED] or empty TOTP does not wipe stored TOTP secret.
func TestChallenger_AccountUpdate_TOTPPreservationViaHTTP(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("USERPROFILE", tmpHome)
	t.Setenv("HOME", tmpHome)
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpHome)

	// Pre-seed account with TOTP in keyring store
	storePath := filepath.Join(tmpHome, "accounts.json")
	store, err := keyring.NewStore(storePath)
	if err != nil {
		t.Fatalf("failed to create keyring store: %v", err)
	}

	origSecret := "JBSWY3DPEHPK3PXP"
	acc := &keyring.Account{
		Email:      "http_totp_victim@test.org",
		Label:      "Original HTTP Account",
		TOTPSecret: origSecret,
		HasTOTP:    true,
	}
	if err := store.AddOrUpdateAccount(acc); err != nil {
		t.Fatalf("AddOrUpdateAccount error: %v", err)
	}

	srv := newTestServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()
	token := srv.GetSessionToken()

	// 1. Update with [REDACTED] TOTP secret
	payloadRedacted := map[string]interface{}{
		"email":       acc.Email,
		"label":       "Renamed Label",
		"totp_secret": "[REDACTED]",
	}
	body, _ := json.Marshal(payloadRedacted)
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/accounts/update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-Token", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("update request failed: %v", err)
	}
	resp.Body.Close()

	updatedAcc, err := store.GetAccount(acc.Email)
	if err != nil {
		t.Fatalf("failed to get account: %v", err)
	}
	if updatedAcc.TOTPSecret != origSecret {
		t.Fatalf("TOTP secret wiped via HTTP update with [REDACTED]! Expected %s, got %q", origSecret, updatedAcc.TOTPSecret)
	}
	if !updatedAcc.HasTOTP {
		t.Fatalf("HasTOTP flipped to false via HTTP update!")
	}
}
