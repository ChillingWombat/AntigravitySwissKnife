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
	if !strings.Contains(resDisabled["message"].(string), "Gemini Bridge is currently disabled") {
		t.Errorf("unexpected message for disabled bridge: %v", resDisabled["message"])
	}

	// 2. Enabled state
	cfg := bridgeSvc.GetConfig()
	cfg.Enabled = true
	if err := bridgeSvc.UpdateConfig(cfg); err != nil {
		t.Fatalf("failed to enable bridge: %v", err)
	}

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
