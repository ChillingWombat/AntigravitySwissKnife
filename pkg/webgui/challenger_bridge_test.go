package webgui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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

	// 4. POST with a matching token is authenticated but not relayed: answering 200
	// would claim a delivery that never happened.
	reqPostValid := httptest.NewRequest(http.MethodPost, "/api/bridge/channels/webhook", bytes.NewReader([]byte(`{"object":"whatsapp_business_account"}`)))
	reqPostValid.Header.Set("X-Verify-Token", "meta-webhook-verify-token-123")
	wPostValid := httptest.NewRecorder()
	server.handleBridgeChannelsWebhook(wPostValid, reqPostValid)

	if wPostValid.Code != http.StatusNotImplemented {
		t.Errorf("expected 501 for an unrelayed POST, got: %d", wPostValid.Code)
	}

	// 5. POST with a matching query token is also refused rather than accepted
	reqPostQuery := httptest.NewRequest(http.MethodPost, "/api/bridge/channels/webhook?verify_token=meta-webhook-verify-token-123", bytes.NewReader([]byte(`{}`)))
	wPostQuery := httptest.NewRecorder()
	server.handleBridgeChannelsWebhook(wPostQuery, reqPostQuery)
	if wPostQuery.Code != http.StatusBadRequest && wPostQuery.Code != http.StatusNotImplemented {
		t.Errorf("expected 501 or 400 for an unrelayed POST, got: %d", wPostQuery.Code)
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

	// ServeRemoteHTTP delegates to RemoteManager and serves the Remote Control console
	req := httptest.NewRequest(http.MethodGet, "/remote", nil)
	w := httptest.NewRecorder()
	server.handleBridgeRemoteConsole(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK from handleBridgeRemoteConsole, got: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Antigravity Remote Control") {
		t.Errorf("expected Remote Control console HTML in response body")
	}

	// Keep-alive probe to /api/bridge/mcp with X-Swiss-KeepAlive: 1 returns 200 OK without recording a failed auth attempt
	reqKA := httptest.NewRequest(http.MethodGet, "/api/bridge/mcp", nil)
	reqKA.Header.Set("X-Swiss-KeepAlive", "1")
	wKA := httptest.NewRecorder()
	server.handleBridgeMCP(wKA, reqKA)
	if wKA.Code != http.StatusOK {
		t.Errorf("expected 200 OK for X-Swiss-KeepAlive on /api/bridge/mcp, got %d", wKA.Code)
	}
	if len(bridgeSvc.GetStatus().RecentAttempts) != 0 {
		t.Errorf("keep-alive probe must not record failed auth attempts, got %d", len(bridgeSvc.GetStatus().RecentAttempts))
	}
}

func TestChallengerBridge_ExternalOriginAllowedOnOAuthAndMCP(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gemini_bridge.json")
	bridgeSvc, err := bridge.NewService(cfgPath)
	if err != nil {
		t.Fatalf("failed to create bridge service: %v", err)
	}

	srv := NewServer("127.0.0.1:0", "")
	srv.bridgeService = bridgeSvc
	mux := srv.setupRoutes()
	handler := srv.corsMiddleware(srv.authMiddleware(mux))

	// 1. POST /oauth/authorize with Origin: https://david-linux.lionfish-catfish.ts.net
	// Valid PIN posted must return 302 Found redirect with code, NOT 403 Forbidden origin.
	tailscaleOrigin := "https://david-linux.lionfish-catfish.ts.net"
	authForm := url.Values{
		"client_id":    {"test-gemini-client"},
		"redirect_uri": {"https://gemini.google.com/oauth/callback"},
		"state":        {"state-xyz-123"},
		"pin":          {bridgeSvc.GetRawPairingPIN()},
	}
	reqAuth := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(authForm.Encode()))
	reqAuth.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqAuth.Header.Set("Origin", tailscaleOrigin)
	wAuth := httptest.NewRecorder()
	handler.ServeHTTP(wAuth, reqAuth)

	if wAuth.Code != http.StatusFound {
		t.Fatalf("expected 302 Found on POST /oauth/authorize, got: %d (body: %s)", wAuth.Code, wAuth.Body.String())
	}
	loc := wAuth.Header().Get("Location")
	if !strings.Contains(loc, "code=") || !strings.Contains(loc, "state=state-xyz-123") {
		t.Errorf("expected redirect location to contain code and state, got: %s", loc)
	}
	if wAuth.Header().Get("Access-Control-Allow-Origin") != tailscaleOrigin {
		t.Errorf("expected Access-Control-Allow-Origin %q, got: %q", tailscaleOrigin, wAuth.Header().Get("Access-Control-Allow-Origin"))
	}

	// 2. POST /oauth/token with Origin: https://gemini.google.com is NOT blocked with 403 Forbidden origin.
	geminiOrigin := "https://gemini.google.com"
	tokenForm := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {"test-gemini-client"},
		"client_secret": {bridgeSvc.GetRawAuthToken()},
	}
	reqToken := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(tokenForm.Encode()))
	reqToken.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqToken.Header.Set("Origin", geminiOrigin)
	wToken := httptest.NewRecorder()
	handler.ServeHTTP(wToken, reqToken)

	if wToken.Code == http.StatusForbidden && strings.Contains(wToken.Body.String(), "Forbidden origin") {
		t.Fatalf("POST /oauth/token was blocked with 403 Forbidden origin: %s", wToken.Body.String())
	}
	if wToken.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on POST /oauth/token, got: %d (body: %s)", wToken.Code, wToken.Body.String())
	}
	if allow := wToken.Header().Get("Access-Control-Allow-Origin"); allow != geminiOrigin {
		t.Errorf("expected Access-Control-Allow-Origin to allow origin, got: %q", allow)
	}

	// 3. GET /.well-known/oauth-protected-resource and GET /.well-known/oauth-authorization-server with Origin: https://gemini.google.com return 200 OK.
	for _, wellKnownPath := range []string{
		"/.well-known/oauth-protected-resource",
		"/.well-known/oauth-authorization-server",
	} {
		reqWK := httptest.NewRequest(http.MethodGet, wellKnownPath, nil)
		reqWK.Header.Set("Origin", geminiOrigin)
		wWK := httptest.NewRecorder()
		handler.ServeHTTP(wWK, reqWK)

		if wWK.Code != http.StatusOK {
			t.Errorf("expected 200 OK for %s with Origin %s, got: %d (body: %s)", wellKnownPath, geminiOrigin, wWK.Code, wWK.Body.String())
		}
		if allow := wWK.Header().Get("Access-Control-Allow-Origin"); allow != geminiOrigin {
			t.Errorf("expected Access-Control-Allow-Origin to allow origin for %s, got: %q", wellKnownPath, allow)
		}
	}

	// 4. POST /api/bridge/mcp with Origin: https://gemini.google.com and valid Bearer token returns 200 OK.
	mcpBody := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`)
	reqMCP := httptest.NewRequest(http.MethodPost, "/api/bridge/mcp", bytes.NewReader(mcpBody))
	reqMCP.Header.Set("Content-Type", "application/json")
	reqMCP.Header.Set("Origin", geminiOrigin)
	reqMCP.Header.Set("Authorization", "Bearer "+bridgeSvc.GetRawAuthToken())
	wMCP := httptest.NewRecorder()
	handler.ServeHTTP(wMCP, reqMCP)

	if wMCP.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for POST /api/bridge/mcp, got: %d (body: %s)", wMCP.Code, wMCP.Body.String())
	}
	if wMCP.Header().Get("Access-Control-Allow-Origin") != geminiOrigin {
		t.Errorf("expected Access-Control-Allow-Origin %q, got: %q", geminiOrigin, wMCP.Header().Get("Access-Control-Allow-Origin"))
	}
	if !strings.Contains(wMCP.Header().Get("Access-Control-Expose-Headers"), "Mcp-Session-Id") {
		t.Errorf("expected Mcp-Session-Id in Access-Control-Expose-Headers, got: %q", wMCP.Header().Get("Access-Control-Expose-Headers"))
	}

	// 5. Protected local routes (e.g. GET /api/accounts) with Origin: https://evil.example.com are STILL rejected with 403 Forbidden origin!
	evilOrigin := "https://evil.example.com"
	reqEvil := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	reqEvil.Header.Set("Origin", evilOrigin)
	wEvil := httptest.NewRecorder()
	handler.ServeHTTP(wEvil, reqEvil)

	if wEvil.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for protected route with evil origin, got: %d", wEvil.Code)
	}
	if !strings.Contains(wEvil.Body.String(), "Forbidden origin") {
		t.Errorf("expected 'Forbidden origin' in error response, got: %q", wEvil.Body.String())
	}
	if wEvil.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("expected no Access-Control-Allow-Origin on rejected origin, got: %q", wEvil.Header().Get("Access-Control-Allow-Origin"))
	}
}
