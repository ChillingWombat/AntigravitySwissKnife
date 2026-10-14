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
