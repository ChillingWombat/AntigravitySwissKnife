package webgui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A stranger must not be able to post prompts into the agent through the Telegram hook.
func TestTelegramWebhookRequiresBotToken(t *testing.T) {
	server := NewServer("127.0.0.1:0", t.TempDir()+"/test.sock")

	// Disabled bridge: refuse outright.
	req := httptest.NewRequest(http.MethodPost, "/api/bridge/channels/telegram/webhook", strings.NewReader(`{"update_id":1}`))
	rec := httptest.NewRecorder()
	server.handleBridgeTelegramWebhook(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("disabled bridge should answer 403, got %d", rec.Code)
	}

	// Enabled bridge with the wrong token: still refuse.
	svc := server.getBridgeService()
	cfg := svc.GetChannelsConfig()
	cfg.Telegram.Enabled = true
	cfg.Telegram.BotToken = "secret-bot-token"
	if err := svc.UpdateChannelsConfig(cfg); err != nil {
		t.Fatalf("update channels config: %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/bridge/channels/telegram/webhook?token=wrong", strings.NewReader(`{"update_id":1}`))
	rec = httptest.NewRecorder()
	server.handleBridgeTelegramWebhook(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("wrong token should answer 403, got %d", rec.Code)
	}

	// GET is not a webhook delivery.
	req = httptest.NewRequest(http.MethodGet, "/api/bridge/channels/telegram/webhook?token=secret-bot-token", nil)
	rec = httptest.NewRecorder()
	server.handleBridgeTelegramWebhook(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should answer 405, got %d", rec.Code)
	}
}
