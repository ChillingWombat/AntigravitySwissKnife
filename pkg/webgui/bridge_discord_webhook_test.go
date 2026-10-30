package webgui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscordWebhookRequiresBotToken(t *testing.T) {
	server := newTestServer("127.0.0.1:0", t.TempDir()+"/test.sock")

	// 1. Disabled bridge: refuse with 403 Forbidden
	req := httptest.NewRequest(http.MethodPost, "/api/bridge/channels/discord/webhook", strings.NewReader(`{"op":0,"t":"MESSAGE_CREATE"}`))
	rec := httptest.NewRecorder()
	server.handleBridgeDiscordWebhook(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("disabled bridge should answer 403, got %d", rec.Code)
	}

	// 2. Wrong token refused with 403 Forbidden
	req = httptest.NewRequest(http.MethodPost, "/api/bridge/channels/discord/webhook", strings.NewReader(`{"op":0,"t":"MESSAGE_CREATE"}`))
	req.Header.Set("Authorization", "Bot wrong-token")
	rec = httptest.NewRecorder()
	server.handleBridgeDiscordWebhook(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("wrong token should answer 403, got %d", rec.Code)
	}

	// 3. GET is rejected with 405 Method Not Allowed
	req = httptest.NewRequest(http.MethodGet, "/api/bridge/channels/discord/webhook", nil)
	rec = httptest.NewRecorder()
	server.handleBridgeDiscordWebhook(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should answer 405, got %d", rec.Code)
	}
}
