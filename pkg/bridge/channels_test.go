package bridge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestChannelsManager_CannedStubsEliminated(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "chat_channels.json")
	cm := NewChannelsManager(cfgPath)

	// 1. Initial status must report disabled / not configured, never fake connected identities
	st := cm.GetStatus()
	if st.Telegram.Identity != "" || st.Discord.Identity != "" {
		t.Errorf("expected empty identities for unverified channels, got tg=%q dc=%q", st.Telegram.Identity, st.Discord.Identity)
	}
	if st.Telegram.StatusText != "Disabled" {
		t.Errorf("expected Telegram StatusText 'Disabled', got %q", st.Telegram.StatusText)
	}

	// 2. TestChannel when disabled returns error
	_, err := cm.TestChannel("telegram")
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Errorf("expected disabled error for telegram, got %v", err)
	}

	// 3. TestChannel when enabled but empty token returns error
	cm.config.Telegram.Enabled = true
	cm.config.Telegram.BotToken = ""
	_, err = cm.TestChannel("telegram")
	if err == nil || !strings.Contains(err.Error(), "Token is required") {
		t.Errorf("expected token required error for telegram, got %v", err)
	}

	// 4. TestChannel with mock Telegram server validating token
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "botVALID_TOKEN/getMe") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"ok": true,
				"result": map[string]interface{}{
					"id":       12345678,
					"username": "LiveVerifiedBot",
					"is_bot":   true,
				},
			})
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":          false,
			"description": "Unauthorized: invalid bot token",
		})
	}))
	defer server.Close()

	// Redirect transport to mock server
	mockClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = server.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(req)
		}),
	}
	cm.HTTPClient = mockClient

	// Test with valid token
	cm.config.Telegram.BotToken = "VALID_TOKEN"
	msg, err := cm.TestChannel("telegram")
	if err != nil {
		t.Fatalf("expected telegram test to succeed with valid token, got: %v", err)
	}
	if !strings.Contains(msg, "@LiveVerifiedBot") || !strings.Contains(msg, "12345678") {
		t.Errorf("expected verified bot info in message, got: %s", msg)
	}

	// Test with invalid token
	cm.config.Telegram.BotToken = "INVALID_TOKEN"
	_, err = cm.TestChannel("telegram")
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 error for invalid token, got: %v", err)
	}

	// 5. Test Discord verification with mock server
	discordServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bot VALID_DC_TOKEN" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":            "88889999",
				"username":      "LiveVerifiedDiscordBot",
				"discriminator": "0",
				"bot":           true,
			})
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "401: Unauthorized",
			"code":    0,
		})
	}))
	defer discordServer.Close()

	cm.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = discordServer.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	cm.config.Discord.Enabled = true
	cm.config.Discord.BotToken = "VALID_DC_TOKEN"
	dcMsg, err := cm.TestChannel("discord")
	if err != nil {
		t.Fatalf("expected discord test to succeed, got: %v", err)
	}
	if !strings.Contains(dcMsg, "LiveVerifiedDiscordBot") || !strings.Contains(dcMsg, "88889999") {
		t.Errorf("expected verified discord bot tag, got: %s", dcMsg)
	}

	// 6. Verify that GetStatus now reflects genuine connected status and verified identities
	stPost := cm.GetStatus()
	if !stPost.Discord.Connected || stPost.Discord.Identity != "LiveVerifiedDiscordBot" {
		t.Errorf("expected Discord connected=true and verified identity, got connected=%t identity=%q", stPost.Discord.Connected, stPost.Discord.Identity)
	}
}

func TestDirectHandler_ComputerUse_NoCannedStubs(t *testing.T) {
	dh := NewDirectHandler()

	// 1. Status reports genuine capability
	st, err := dh.ComputerUse(map[string]interface{}{"action": "status"})
	if err != nil {
		t.Fatalf("ComputerUse status returned error: %v", err)
	}
	if !strings.Contains(st, "actuator ready") && !strings.Contains(st, "not configured") {
		t.Errorf("unexpected status output: %s", st)
	}

	// 2. Unsupported actions fail explicitly instead of returning fake success
	_, err = dh.ComputerUse(map[string]interface{}{"action": "click"})
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected click to return not configured error, got: %v", err)
	}

	_, err = dh.ComputerUse(map[string]interface{}{"action": "type"})
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected type to return not configured error, got: %v", err)
	}

	// 3. Empty action errors
	_, err = dh.ComputerUse(map[string]interface{}{})
	if err == nil {
		t.Errorf("expected empty action to fail")
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestChannelsManager_WhatsAppWebhookAndRecordActivity(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "chat_channels.json")
	cm := NewChannelsManager(cfgPath)

	// 1. Initial LastMessageAt should be empty for all channels
	initStatus := cm.GetStatus()
	if initStatus.WhatsApp.LastMessageAt != "" || initStatus.Telegram.LastMessageAt != "" || initStatus.Discord.LastMessageAt != "" {
		t.Errorf("expected empty LastMessageAt initially, got wa=%q tg=%q dc=%q",
			initStatus.WhatsApp.LastMessageAt, initStatus.Telegram.LastMessageAt, initStatus.Discord.LastMessageAt)
	}

	// 2. Enable WhatsApp in webhook mode
	cm.config.WhatsApp.Enabled = true
	cm.config.WhatsApp.Mode = "webhook"
	cm.config.WhatsApp.VerifyToken = ""

	// Missing verify token -> error
	_, err := cm.TestChannel("whatsapp")
	if err == nil || !strings.Contains(err.Error(), "verify token required") {
		t.Fatalf("expected verify token required error, got: %v", err)
	}

	// [REDACTED] verify token -> error
	cm.config.WhatsApp.VerifyToken = "[REDACTED]"
	_, err = cm.TestChannel("whatsapp")
	if err == nil || !strings.Contains(err.Error(), "verify token required") {
		t.Fatalf("expected verify token required error for [REDACTED], got: %v", err)
	}

	// Valid verify token -> success + verified identity "WhatsApp Webhook Gateway"
	cm.config.WhatsApp.VerifyToken = "super-secret-verify-token"
	msg, err := cm.TestChannel("whatsapp")
	if err != nil {
		t.Fatalf("expected WhatsApp webhook test to succeed, got: %v", err)
	}
	if !strings.Contains(msg, "WhatsApp custom webhook bridge verified") {
		t.Errorf("expected success message containing verification confirmation, got: %s", msg)
	}

	st := cm.GetStatus()
	if !st.WhatsApp.Connected {
		t.Errorf("expected WhatsApp connected=true")
	}
	if st.WhatsApp.Identity != "WhatsApp Webhook Gateway" {
		t.Errorf("expected WhatsApp identity 'WhatsApp Webhook Gateway', got: %q", st.WhatsApp.Identity)
	}
	if !strings.Contains(st.WhatsApp.StatusText, "Verified") {
		t.Errorf("expected StatusText to contain 'Verified', got: %q", st.WhatsApp.StatusText)
	}

	// Ensure mode: webhook takes precedence even if AccessToken is populated
	cm.config.WhatsApp.AccessToken = "old-legacy-token"
	msgWithToken, err := cm.TestChannel("whatsapp")
	if err != nil || !strings.Contains(msgWithToken, "WhatsApp custom webhook bridge verified") {
		t.Errorf("expected webhook mode to take precedence when AccessToken exists, got err=%v msg=%s", err, msgWithToken)
	}

	// 3. Test RecordActivity populates LastMessageAt in GetStatus()
	cm.RecordActivity("whatsapp")
	stPostWa := cm.GetStatus()
	if stPostWa.WhatsApp.LastMessageAt == "" {
		t.Errorf("expected non-empty WhatsApp LastMessageAt after RecordActivity")
	}
	if cm.GetLastMessageAt("whatsapp") != stPostWa.WhatsApp.LastMessageAt {
		t.Errorf("expected GetLastMessageAt to match LastMessageAt in status, got %q vs %q",
			cm.GetLastMessageAt("whatsapp"), stPostWa.WhatsApp.LastMessageAt)
	}

	// Record activity for Telegram and Discord (case insensitive)
	cm.RecordActivity("Telegram")
	cm.RecordActivity("DISCORD")
	stPostAll := cm.GetStatus()
	if stPostAll.Telegram.LastMessageAt == "" {
		t.Errorf("expected non-empty Telegram LastMessageAt after RecordActivity")
	}
	if stPostAll.Discord.LastMessageAt == "" {
		t.Errorf("expected non-empty Discord LastMessageAt after RecordActivity")
	}
}
