package webgui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/bridge"
)

// TestChallengerBridge_WebGUI_BridgeTestEndpoint empirically tests /api/bridge/test:
// 1. When bridge is disabled, returns success: false with disabled notification.
// 2. When bridge is enabled, returns genuine surface verification without fake claims.
func TestChallengerBridge_WebGUI_BridgeTestEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gemini_bridge.json")
	bridgeSvc, err := bridge.NewService(cfgPath)
	if err != nil {
		t.Fatalf("failed to create bridge service: %v", err)
	}

	server := &Server{
		bridgeService: bridgeSvc,
	}

	// 1. Disabled state (default)
	req := httptest.NewRequest(http.MethodPost, "/api/bridge/test", nil)
	w := httptest.NewRecorder()
	server.handleBridgeTest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d", w.Code)
	}
	var resDisabled map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resDisabled); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}
	if resDisabled["success"] != false {
		t.Errorf("expected success: false for disabled bridge, got: %v", resDisabled["success"])
	}
	if !strings.Contains(resDisabled["message"].(string), "Start Remote Access") {
		t.Errorf("unexpected message for disabled bridge: %v", resDisabled["message"])
	}

	// 2. Enabled state
	if _, err := bridgeSvc.StartTunnel("custom", "https://bridge.example.com"); err != nil {
		t.Fatalf("failed to start bridge tunnel: %v", err)
	}
	defer bridgeSvc.StopTunnel()

	wEnabled := httptest.NewRecorder()
	server.handleBridgeTest(wEnabled, req)
	if wEnabled.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d", wEnabled.Code)
	}
	var resEnabled map[string]interface{}
	if err := json.Unmarshal(wEnabled.Body.Bytes(), &resEnabled); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}
	if resEnabled["success"] != true {
		t.Errorf("expected success: true for enabled bridge, got: %v", resEnabled["success"])
	}
	msg := resEnabled["message"].(string)
	if !strings.Contains(msg, "Gemini Bridge verified:") || !strings.Contains(msg, "of 3 local surfaces active") {
		t.Errorf("expected dynamic surface verification message, got: %s", msg)
	}
	// Verify that the old canned message is not present
	if strings.Contains(msg, "All local Antigravity surfaces synchronized") {
		t.Errorf("found old fake sync message in response: %s", msg)
	}
}

// TestChallengerBridge_WebGUI_BridgeChannelsTestEndpoint empirically tests /api/bridge/channels/test:
// 1. Method rejection (GET must return 405 Method Not Allowed).
// 2. Unknown channel returns success: false and descriptive error.
// 3. Disabled channel returns success: false and "bridge is disabled" error.
func TestChallengerBridge_WebGUI_BridgeChannelsTestEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gemini_bridge.json")
	bridgeSvc, err := bridge.NewService(cfgPath)
	if err != nil {
		t.Fatalf("failed to create bridge service: %v", err)
	}

	server := &Server{
		bridgeService: bridgeSvc,
	}

	// 1. Method Not Allowed for GET
	reqGet := httptest.NewRequest(http.MethodGet, "/api/bridge/channels/test", nil)
	wGet := httptest.NewRecorder()
	server.handleBridgeChannelsTest(wGet, reqGet)
	if wGet.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET, got %d", wGet.Code)
	}

	// 2. Unknown channel
	bodyUnknown, _ := json.Marshal(map[string]string{"channel": "matrix"})
	reqUnknown := httptest.NewRequest(http.MethodPost, "/api/bridge/channels/test", bytes.NewReader(bodyUnknown))
	wUnknown := httptest.NewRecorder()
	server.handleBridgeChannelsTest(wUnknown, reqUnknown)
	var resUnknown map[string]interface{}
	_ = json.Unmarshal(wUnknown.Body.Bytes(), &resUnknown)
	if resUnknown["success"] != false || !strings.Contains(resUnknown["error"].(string), "unknown channel") {
		t.Errorf("expected unknown channel error, got: %+v", resUnknown)
	}

	// 3. Disabled telegram channel
	bodyTg, _ := json.Marshal(map[string]string{"channel": "telegram"})
	reqTg := httptest.NewRequest(http.MethodPost, "/api/bridge/channels/test", bytes.NewReader(bodyTg))
	wTg := httptest.NewRecorder()
	server.handleBridgeChannelsTest(wTg, reqTg)
	var resTg map[string]interface{}
	_ = json.Unmarshal(wTg.Body.Bytes(), &resTg)
	if resTg["success"] != false || !strings.Contains(resTg["error"].(string), "disabled") {
		t.Errorf("expected disabled error for telegram, got: %+v", resTg)
	}
}

func TestChallengerBridge_WebGUI_BridgeDispatch_LocalRunCommand(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gemini_bridge.json")
	bridgeSvc, err := bridge.NewService(cfgPath)
	if err != nil {
		t.Fatalf("failed to create bridge service: %v", err)
	}

	server := &Server{
		bridgeService: bridgeSvc,
	}

	body, _ := json.Marshal(map[string]string{
		"prompt": "echo bridge_dispatch_ok",
		"target": "local_run_command",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/bridge/dispatch", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.handleBridgeDispatch(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d (body: %s)", w.Code, w.Body.String())
	}
	var res map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}
	if res["success"] != true {
		t.Errorf("expected success: true, got: %v", res["success"])
	}
	resStr, ok := res["result"].(string)
	if !ok || !strings.Contains(resStr, "bridge_dispatch_ok") {
		t.Errorf("expected result to contain 'bridge_dispatch_ok', got: %v", res["result"])
	}
}

func TestChallengerBridge_WebGUI_BridgeChannelsConfig_RedactionAndUnredacted(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gemini_bridge.json")
	bridgeSvc, err := bridge.NewService(cfgPath)
	if err != nil {
		t.Fatalf("failed to create bridge service: %v", err)
	}

	// Update channels config with sensitive tokens
	chCfg := bridgeSvc.GetChannelsConfig()
	chCfg.WhatsApp.AccessToken = "secret-wa-access-token"
	chCfg.WhatsApp.VerifyToken = "secret-wa-verify-token"
	chCfg.Telegram.BotToken = "secret-tg-bot-token"
	chCfg.Discord.BotToken = "secret-dc-bot-token"
	if err := bridgeSvc.UpdateChannelsConfig(chCfg); err != nil {
		t.Fatalf("failed to update channels config: %v", err)
	}

	server := &Server{
		bridgeService: bridgeSvc,
	}

	// 1. Default GET returns redacted tokens
	reqDefault := httptest.NewRequest(http.MethodGet, "/api/bridge/channels/config", nil)
	wDefault := httptest.NewRecorder()
	server.handleBridgeChannelsConfig(wDefault, reqDefault)

	if wDefault.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d", wDefault.Code)
	}
	var resDefault struct {
		Success bool                  `json:"success"`
		Config  bridge.ChannelsConfig `json:"config"`
	}
	if err := json.Unmarshal(wDefault.Body.Bytes(), &resDefault); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}
	if !resDefault.Success {
		t.Errorf("expected success: true")
	}
	if resDefault.Config.WhatsApp.AccessToken != "[REDACTED]" {
		t.Errorf("expected redacted WhatsApp access token, got: %q", resDefault.Config.WhatsApp.AccessToken)
	}
	if resDefault.Config.WhatsApp.VerifyToken != "[REDACTED]" {
		t.Errorf("expected redacted WhatsApp verify token, got: %q", resDefault.Config.WhatsApp.VerifyToken)
	}
	if resDefault.Config.Telegram.BotToken != "[REDACTED]" {
		t.Errorf("expected redacted Telegram bot token, got: %q", resDefault.Config.Telegram.BotToken)
	}
	if resDefault.Config.Discord.BotToken != "[REDACTED]" {
		t.Errorf("expected redacted Discord bot token, got: %q", resDefault.Config.Discord.BotToken)
	}

	// 2. GET with ?unredacted=true returns actual unredacted tokens
	reqUnredacted := httptest.NewRequest(http.MethodGet, "/api/bridge/channels/config?unredacted=true", nil)
	wUnredacted := httptest.NewRecorder()
	server.handleBridgeChannelsConfig(wUnredacted, reqUnredacted)

	if wUnredacted.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d", wUnredacted.Code)
	}
	var resUnredacted struct {
		Success bool                  `json:"success"`
		Config  bridge.ChannelsConfig `json:"config"`
	}
	if err := json.Unmarshal(wUnredacted.Body.Bytes(), &resUnredacted); err != nil {
		t.Fatalf("failed to parse unredacted JSON response: %v", err)
	}
	if !resUnredacted.Success {
		t.Errorf("expected success: true")
	}
	if resUnredacted.Config.WhatsApp.AccessToken != "secret-wa-access-token" {
		t.Errorf("expected unredacted WhatsApp access token, got: %q", resUnredacted.Config.WhatsApp.AccessToken)
	}
	if resUnredacted.Config.WhatsApp.VerifyToken != "secret-wa-verify-token" {
		t.Errorf("expected unredacted WhatsApp verify token, got: %q", resUnredacted.Config.WhatsApp.VerifyToken)
	}
	if resUnredacted.Config.Telegram.BotToken != "secret-tg-bot-token" {
		t.Errorf("expected unredacted Telegram bot token, got: %q", resUnredacted.Config.Telegram.BotToken)
	}
	if resUnredacted.Config.Discord.BotToken != "secret-dc-bot-token" {
		t.Errorf("expected unredacted Discord bot token, got: %q", resUnredacted.Config.Discord.BotToken)
	}
}

func TestChallengerBridge_WebGUI_BridgeChannelsWebhook(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gemini_bridge.json")
	bridgeSvc, err := bridge.NewService(cfgPath)
	if err != nil {
		t.Fatalf("failed to create bridge service: %v", err)
	}

	// Configure WhatsApp verify token
	chCfg := bridgeSvc.GetChannelsConfig()
	chCfg.WhatsApp.VerifyToken = "meta-webhook-verify-token-123"
	if err := bridgeSvc.UpdateChannelsConfig(chCfg); err != nil {
		t.Fatalf("failed to update channels config: %v", err)
	}

	server := &Server{
		bridgeService: bridgeSvc,
	}

	// 1. GET with matching hub.verify_token and hub.mode=subscribe returns challenge and 200 OK
	challengeVal := "challenge_string_998877"
	reqValid := httptest.NewRequest(http.MethodGet,
		"/api/bridge/channels/webhook?hub.mode=subscribe&hub.verify_token=meta-webhook-verify-token-123&hub.challenge="+challengeVal, nil)
	wValid := httptest.NewRecorder()
	server.handleBridgeChannelsWebhook(wValid, reqValid)

	if wValid.Code != http.StatusOK {
		t.Errorf("expected 200 OK for valid webhook challenge, got: %d", wValid.Code)
	}
	if wValid.Body.String() != challengeVal {
		t.Errorf("expected challenge body %q, got: %q", challengeVal, wValid.Body.String())
	}

	// 2. GET with mismatching token returns 403 Forbidden
	reqMismatch := httptest.NewRequest(http.MethodGet,
		"/api/bridge/channels/webhook?hub.mode=subscribe&hub.verify_token=wrong-token&hub.challenge="+challengeVal, nil)
	wMismatch := httptest.NewRecorder()
	server.handleBridgeChannelsWebhook(wMismatch, reqMismatch)

	if wMismatch.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for mismatched verify token, got: %d", wMismatch.Code)
	}

	// 3. GET with mismatching mode returns 403 Forbidden
	reqBadMode := httptest.NewRequest(http.MethodGet,
		"/api/bridge/channels/webhook?hub.mode=unsubscribe&hub.verify_token=meta-webhook-verify-token-123&hub.challenge="+challengeVal, nil)
	wBadMode := httptest.NewRecorder()
	server.handleBridgeChannelsWebhook(wBadMode, reqBadMode)

	if wBadMode.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for non-subscribe mode, got: %d", wBadMode.Code)
	}

	// 4. POST with matching token (via header) returns 200 OK {"success": true}
	reqPostValid := httptest.NewRequest(http.MethodPost, "/api/bridge/channels/webhook", bytes.NewReader([]byte(`{"object":"whatsapp_business_account"}`)))
	reqPostValid.Header.Set("X-Verify-Token", "meta-webhook-verify-token-123")
	wPostValid := httptest.NewRecorder()
	server.handleBridgeChannelsWebhook(wPostValid, reqPostValid)

	if wPostValid.Code != http.StatusOK {
		t.Errorf("expected 200 OK for valid POST webhook, got: %d", wPostValid.Code)
	}
	var resPost map[string]interface{}
	_ = json.Unmarshal(wPostValid.Body.Bytes(), &resPost)
	if resPost["success"] != true {
		t.Errorf("expected success: true, got: %v", resPost["success"])
	}

	// 5. POST with matching token (via query verify_token) returns 200 OK
	reqPostQuery := httptest.NewRequest(http.MethodPost, "/api/bridge/channels/webhook?verify_token=meta-webhook-verify-token-123", bytes.NewReader([]byte(`{}`)))
	wPostQuery := httptest.NewRecorder()
	server.handleBridgeChannelsWebhook(wPostQuery, reqPostQuery)
	if wPostQuery.Code != http.StatusOK {
		t.Errorf("expected 200 OK for valid query verify_token, got: %d", wPostQuery.Code)
	}

	// 6. POST with mismatching token returns 403 Forbidden
	reqPostMismatch := httptest.NewRequest(http.MethodPost, "/api/bridge/channels/webhook", bytes.NewReader([]byte(`{}`)))
	reqPostMismatch.Header.Set("X-Verify-Token", "wrong-post-token")
	wPostMismatch := httptest.NewRecorder()
	server.handleBridgeChannelsWebhook(wPostMismatch, reqPostMismatch)

	if wPostMismatch.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for mismatched POST token, got: %d", wPostMismatch.Code)
	}
}

func TestChallengerBridge_WebGUI_RemoteConsoleRouteAndAuth(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gemini_bridge.json")
	bridgeSvc, err := bridge.NewService(cfgPath)
	if err != nil {
		t.Fatalf("failed to create bridge service: %v", err)
	}

	server := &Server{
		bridgeService: bridgeSvc,
	}

	// Public path verification
	if !isPublicPath("/remote") {
		t.Errorf("expected /remote to be whitelisted in isPublicPath")
	}
	if !isPublicPath("/remote/index.html") {
		t.Errorf("expected /remote/index.html to be whitelisted in isPublicPath")
	}
	if !isPublicPath("/api/bridge/channels/webhook") {
		t.Errorf("expected /api/bridge/channels/webhook to be whitelisted in isPublicPath")
	}

	// When ServeRemoteHTTP is not yet available, returns 503
	req := httptest.NewRequest(http.MethodGet, "/remote", nil)
	w := httptest.NewRecorder()
	server.handleBridgeRemoteConsole(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when ServeRemoteHTTP is not implemented, got: %d", w.Code)
	}
}
