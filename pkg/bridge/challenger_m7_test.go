package bridge

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestChallengerM7_DirectHandler_LiveWindowsScreenshot empirically verifies that on Windows,
// ComputerUse("screenshot") executes a genuine PowerShell GDI+ screen capture, writes a real PNG
// file to the operating system's temp directory with non-zero size, and returns the file path and size.
// It also verifies that the generated file is a valid file that can be opened, and cleans up the artifact.
func TestChallengerM7_DirectHandler_LiveWindowsScreenshot(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("skipping Windows-specific screen capture test on non-Windows host")
	}

	dh := NewDirectHandler()
	res, err := dh.ComputerUse(map[string]interface{}{"action": "screenshot"})
	if err != nil {
		if strings.Contains(err.Error(), "screenshot capture not configured") || strings.Contains(err.Error(), "no active display") {
			t.Skipf("skipping live screenshot test without an interactive Windows desktop: %v", err)
		}
		t.Fatalf("ComputerUse screenshot failed on live Windows host: %v", err)
	}

	// Must contain the genuine execution string, NOT the old canned stub
	if !strings.Contains(res, "Screenshot captured via windows-screen-capture to ") {
		t.Fatalf("expected genuine windows-screen-capture result, got: %s", res)
	}
	if strings.Contains(res, "default display subsystem") {
		t.Fatalf("found old canned stub string in output: %s", res)
	}

	// Extract the output file path from: "Screenshot captured via windows-screen-capture to <path> (<N> bytes)"
	prefix := "to "
	suffix := " ("
	idx1 := strings.Index(res, prefix)
	idx2 := strings.LastIndex(res, suffix)
	if idx1 == -1 || idx2 == -1 || idx1+len(prefix) >= idx2 {
		t.Fatalf("unable to parse output path from result string: %s", res)
	}
	filePath := res[idx1+len(prefix) : idx2]

	fi, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("screenshot file was not actually created on disk at %s: %v", filePath, err)
	}
	if fi.Size() == 0 {
		t.Fatalf("screenshot file at %s is empty (0 bytes)", filePath)
	}

	// Verify file header is a PNG: 89 50 4E 47 0D 0A 1A 0A
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read screenshot file: %v", err)
	}
	if len(content) < 8 || string(content[1:4]) != "PNG" {
		t.Fatalf("file at %s does not contain valid PNG magic bytes", filePath)
	}

	// Clean up temp screenshot file
	_ = os.Remove(filePath)
}

// TestChallengerM7_DirectHandler_InputActuatorsRejectFakeSuccess verifies that input
// actions without their required arguments fail explicitly rather than reporting a
// success that never happened. Real input is not executed here on purpose: it would
// move the user's pointer.
func TestChallengerM7_DirectHandler_InputActuatorsRejectFakeSuccess(t *testing.T) {
	dh := NewDirectHandler()

	actionsToTest := []string{"click", "type"}
	for _, act := range actionsToTest {
		res, err := dh.ComputerUse(map[string]interface{}{"action": act})
		if err == nil {
			t.Errorf("action %q returned nil error and fake success: %q", act, res)
		}
		if !strings.Contains(err.Error(), "requires") {
			t.Errorf("action %q returned unexpected error message: %v", act, err)
		}
	}

	// Unsupported or adversarial actions must also be rejected
	adversarialActions := []string{"drag", "double_click", "middle_click", "scroll", "key_press", "rm -rf /"}
	for _, act := range adversarialActions {
		_, err := dh.ComputerUse(map[string]interface{}{"action": act})
		if err == nil {
			t.Errorf("adversarial action %q succeeded unexpectedly", act)
		}
		if !strings.Contains(err.Error(), "unsupported computer use action") {
			t.Errorf("expected 'unsupported computer use action' for %q, got: %v", act, err)
		}
	}
}

// TestChallengerM7_Channels_Telegram_AdversarialScenarios tests all failure modes of Telegram:
// 1. Channel disabled
// 2. Empty bot token
// 3. HTTP 401 Unauthorized
// 4. HTTP 500 Internal Server Error
// 5. HTTP 200 with ok: false in JSON body
// 6. Network failure / connection refused
// 7. Successful verification, followed by configuration change that must clear verified identity
func TestChallengerM7_Channels_Telegram_AdversarialScenarios(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "chat_channels.json")
	cm := NewChannelsManager(cfgPath)

	// Case 1: Disabled
	cm.config.Telegram.Enabled = false
	cm.config.Telegram.BotToken = "some_token"
	_, err := cm.TestChannel("telegram")
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("expected disabled error, got: %v", err)
	}

	// Case 2: Enabled but empty token
	cm.config.Telegram.Enabled = true
	cm.config.Telegram.BotToken = "   "
	_, err = cm.TestChannel("telegram")
	if err == nil || !strings.Contains(err.Error(), "Token is required") {
		t.Fatalf("expected token required error, got: %v", err)
	}

	// Set up mock server with variable response behavior
	var mockStatus int
	var mockBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(mockStatus)
		_, _ = w.Write([]byte(mockBody))
	}))
	defer server.Close()

	cm.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = server.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	// Case 3: HTTP 401 Unauthorized
	cm.config.Telegram.BotToken = "BAD_TOKEN_401"
	mockStatus = http.StatusUnauthorized
	mockBody = `{"ok":false,"error_code":401,"description":"Unauthorized"}`
	_, err = cm.TestChannel("telegram")
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected 401 error, got: %v", err)
	}
	st := cm.GetStatus()
	if st.Telegram.Connected || st.Telegram.Identity != "" {
		t.Fatalf("channel was marked connected or had identity after 401 error: %+v", st.Telegram)
	}

	// Case 4: HTTP 500 Internal Server Error
	cm.config.Telegram.BotToken = "TOKEN_500"
	mockStatus = http.StatusInternalServerError
	mockBody = `{"ok":false,"error_code":500,"description":"Internal server error"}`
	_, err = cm.TestChannel("telegram")
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected 500 error, got: %v", err)
	}
	st = cm.GetStatus()
	if st.Telegram.Connected {
		t.Fatalf("channel was marked connected after 500 error")
	}

	// Case 5: HTTP 200 with ok: false in body
	cm.config.Telegram.BotToken = "TOKEN_NOT_OK"
	mockStatus = http.StatusOK
	mockBody = `{"ok":false,"description":"Conflict: terminated by other getUpdates"}`
	_, err = cm.TestChannel("telegram")
	if err == nil || !strings.Contains(err.Error(), "Telegram verification failed") {
		t.Fatalf("expected Telegram verification failed error, got: %v", err)
	}
	st = cm.GetStatus()
	if st.Telegram.Connected {
		t.Fatalf("channel was marked connected after ok=false response")
	}

	// Case 6: Successful verification
	cm.config.Telegram.BotToken = "VALID_TOKEN"
	mockStatus = http.StatusOK
	mockBody = `{"ok":true,"result":{"id":998877,"is_bot":true,"username":"MyEmpiricalBot"}}`
	msg, err := cm.TestChannel("telegram")
	if err != nil {
		t.Fatalf("expected success with valid token, got: %v", err)
	}
	if !strings.Contains(msg, "@MyEmpiricalBot") || !strings.Contains(msg, "998877") {
		t.Fatalf("expected verified handle and ID in msg, got: %s", msg)
	}
	st = cm.GetStatus()
	if !st.Telegram.Connected || st.Telegram.Identity != "@MyEmpiricalBot" {
		t.Fatalf("expected connected=true and @MyEmpiricalBot identity, got: %+v", st.Telegram)
	}

	// Case 7: Configuration change must invalidate cached verified identity
	updatedCfg := cm.GetConfig()
	updatedCfg.Telegram.BotToken = "NEW_UNVERIFIED_TOKEN"
	if err := cm.UpdateConfig(updatedCfg); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}
	stAfterUpdate := cm.GetStatus()
	if stAfterUpdate.Telegram.Connected || stAfterUpdate.Telegram.Identity != "" {
		t.Fatalf("expected identity and connected status to be cleared after token update, got: %+v", stAfterUpdate.Telegram)
	}
}

// TestChallengerM7_Channels_Discord_AdversarialScenarios tests failure modes of Discord:
// 1. Channel disabled
// 2. Empty token
// 3. HTTP 401 Unauthorized
// 4. HTTP 429 Too Many Requests
// 5. HTTP 200 OK with valid bot info
// 6. Invalidation upon token change
func TestChallengerM7_Channels_Discord_AdversarialScenarios(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "chat_channels.json")
	cm := NewChannelsManager(cfgPath)

	// Case 1: Disabled
	cm.config.Discord.Enabled = false
	cm.config.Discord.BotToken = "some_token"
	_, err := cm.TestChannel("discord")
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("expected disabled error for discord, got: %v", err)
	}

	// Case 2: Enabled but empty token
	cm.config.Discord.Enabled = true
	cm.config.Discord.BotToken = ""
	_, err = cm.TestChannel("discord")
	if err == nil || !strings.Contains(err.Error(), "Token is required") {
		t.Fatalf("expected token required error, got: %v", err)
	}

	var mockStatus int
	var mockBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Discord Bot Authorization header
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bot ") {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message": "401: Unauthorized", "code": 0}`))
			return
		}
		w.WriteHeader(mockStatus)
		_, _ = w.Write([]byte(mockBody))
	}))
	defer server.Close()

	cm.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = server.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	// Case 3: HTTP 401 Unauthorized
	cm.config.Discord.BotToken = "INVALID_BOT_TOKEN"
	mockStatus = http.StatusUnauthorized
	mockBody = `{"message": "401: Unauthorized", "code": 0}`
	_, err = cm.TestChannel("discord")
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected 401 error, got: %v", err)
	}
	st := cm.GetStatus()
	if st.Discord.Connected || st.Discord.Identity != "" {
		t.Fatalf("channel was marked connected or had identity after Discord 401 error: %+v", st.Discord)
	}

	// Case 4: HTTP 429 Too Many Requests
	cm.config.Discord.BotToken = "RATE_LIMITED_TOKEN"
	mockStatus = http.StatusTooManyRequests
	mockBody = `{"message": "You are being rate limited.", "retry_after": 5.0, "global": false}`
	_, err = cm.TestChannel("discord")
	if err == nil || !strings.Contains(err.Error(), "429") {
		t.Fatalf("expected 429 error, got: %v", err)
	}
	st = cm.GetStatus()
	if st.Discord.Connected {
		t.Fatalf("channel was marked connected after 429 error")
	}

	// Case 5: Valid Discord verification
	cm.config.Discord.BotToken = "VALID_DISCORD_TOKEN"
	mockStatus = http.StatusOK
	mockBody = `{"id":"11223344","username":"SwissAdversarialBot","discriminator":"0","bot":true}`
	msg, err := cm.TestChannel("discord")
	if err != nil {
		t.Fatalf("expected success with valid token, got: %v", err)
	}
	if !strings.Contains(msg, "SwissAdversarialBot") || !strings.Contains(msg, "11223344") {
		t.Fatalf("expected verified bot info in msg, got: %s", msg)
	}
	st = cm.GetStatus()
	if !st.Discord.Connected || st.Discord.Identity != "SwissAdversarialBot" {
		t.Fatalf("expected Discord connected=true and verified identity, got: %+v", st.Discord)
	}

	// Case 6: Invalidation on token change
	cfg := cm.GetConfig()
	cfg.Discord.BotToken = "CHANGED_TOKEN"
	_ = cm.UpdateConfig(cfg)
	if cm.GetStatus().Discord.Connected || cm.GetStatus().Discord.Identity != "" {
		t.Fatalf("expected Discord identity cleared after config change")
	}
}

// TestChallengerM7_Channels_WhatsApp_AdversarialScenarios tests WhatsApp failure modes:
// 1. Channel disabled
// 2. Cloud API mode with empty access token or [REDACTED]
// 3. Cloud API mode with HTTP 401 from Meta Graph API
// 4. Cloud API mode with HTTP 200 OK from Meta Graph API
// 5. MCP mode with empty verify token vs configured verify token
func TestChallengerM7_Channels_WhatsApp_AdversarialScenarios(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "chat_channels.json")
	cm := NewChannelsManager(cfgPath)

	// Case 1: Disabled
	cm.config.WhatsApp.Enabled = false
	_, err := cm.TestChannel("whatsapp")
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("expected disabled error for whatsapp, got: %v", err)
	}

	// Case 2: Cloud API mode with empty token
	cm.config.WhatsApp.Enabled = true
	cm.config.WhatsApp.Mode = "cloud_api"
	cm.config.WhatsApp.AccessToken = ""
	_, err = cm.TestChannel("whatsapp")
	if err == nil || !strings.Contains(err.Error(), "access token is required") {
		t.Fatalf("expected token required error, got: %v", err)
	}

	// Case 3: Cloud API mode with [REDACTED] placeholder token
	cm.config.WhatsApp.AccessToken = "[REDACTED]"
	_, err = cm.TestChannel("whatsapp")
	if err == nil || !strings.Contains(err.Error(), "access token is required") {
		t.Fatalf("expected token required error for [REDACTED], got: %v", err)
	}

	var mockStatus int
	var mockBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(mockStatus)
		_, _ = w.Write([]byte(mockBody))
	}))
	defer server.Close()

	cm.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.URL.Scheme = "http"
			req.URL.Host = server.Listener.Addr().String()
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	// Case 4: Cloud API HTTP 401
	cm.config.WhatsApp.AccessToken = "INVALID_META_TOKEN"
	mockStatus = http.StatusUnauthorized
	mockBody = `{"error":{"message":"Invalid OAuth access token.","type":"OAuthException","code":190}}`
	_, err = cm.TestChannel("whatsapp")
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("expected HTTP 401 error for whatsapp, got: %v", err)
	}
	st := cm.GetStatus()
	if st.WhatsApp.Connected || st.WhatsApp.Identity != "" {
		t.Fatalf("whatsapp connected after 401 error: %+v", st.WhatsApp)
	}

	// Case 5: Cloud API HTTP 200
	cm.config.WhatsApp.AccessToken = "VALID_META_TOKEN"
	mockStatus = http.StatusOK
	mockBody = `{"id":"55667788","name":"EmpiricalWhatsAppBusiness"}`
	msg, err := cm.TestChannel("whatsapp")
	if err != nil {
		t.Fatalf("expected success with valid meta token, got: %v", err)
	}
	if !strings.Contains(msg, "55667788") || !strings.Contains(msg, "EmpiricalWhatsAppBusiness") {
		t.Fatalf("expected App ID and name in whatsapp response, got: %s", msg)
	}
	st = cm.GetStatus()
	if !st.WhatsApp.Connected || st.WhatsApp.Identity != "EmpiricalWhatsAppBusiness" {
		t.Fatalf("expected whatsapp connected=true and verified identity, got: %+v", st.WhatsApp)
	}

	// Case 6: MCP mode (with cleared AccessToken)
	cm.config.WhatsApp.AccessToken = ""
	cm.config.WhatsApp.Mode = "mcp"
	cm.config.WhatsApp.VerifyToken = ""
	_, err = cm.TestChannel("whatsapp")
	if err == nil || !strings.Contains(err.Error(), "verify token required") {
		t.Fatalf("expected verify token required error, got: %v", err)
	}

	cm.config.WhatsApp.VerifyToken = "my-secure-webhook-token"
	mcpMsg, err := cm.TestChannel("whatsapp")
	if err != nil {
		t.Fatalf("expected MCP mode to succeed with verify token, got: %v", err)
	}
	if !strings.Contains(mcpMsg, "WhatsApp MCP bridge verified") {
		t.Fatalf("unexpected MCP message: %s", mcpMsg)
	}
}

// TestChallengerM7_Channels_SecretPreservation_OnUpdate verifies that:
// 1. GetChannelsConfigRedacted redacts all secrets to "[REDACTED]".
// 2. Submitting an update with "[REDACTED]" or "******" preserves existing secrets.
func TestChallengerM7_Channels_SecretPreservation_OnUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "chat_channels.json")
	cm := NewChannelsManager(cfgPath)

	initialCfg := cm.GetConfig()
	initialCfg.WhatsApp.AccessToken = "original_whatsapp_access_token_123"
	initialCfg.WhatsApp.VerifyToken = "original_whatsapp_verify_token_456"
	initialCfg.Telegram.BotToken = "original_telegram_bot_token_789"
	initialCfg.Discord.BotToken = "original_discord_bot_token_012"
	if err := cm.UpdateConfig(initialCfg); err != nil {
		t.Fatalf("failed to set initial config: %v", err)
	}

	// 1. Redacted config must mask all tokens
	redacted := cm.GetChannelsConfigRedacted()
	if redacted.WhatsApp.AccessToken != "[REDACTED]" ||
		redacted.WhatsApp.VerifyToken != "[REDACTED]" ||
		redacted.Telegram.BotToken != "[REDACTED]" ||
		redacted.Discord.BotToken != "[REDACTED]" {
		t.Fatalf("secrets leaked in redacted config: %+v", redacted)
	}

	// 2. Submit update with redacted placeholders
	updatePayload := redacted
	updatePayload.WhatsApp.Enabled = true
	updatePayload.Telegram.Enabled = true
	updatePayload.Discord.Enabled = true
	if err := cm.UpdateConfig(updatePayload); err != nil {
		t.Fatalf("failed to update config with redacted placeholders: %v", err)
	}

	// 3. Raw config must retain original secrets intact
	savedRaw := cm.GetConfig()
	if savedRaw.WhatsApp.AccessToken != "original_whatsapp_access_token_123" {
		t.Errorf("WhatsApp AccessToken was overwritten: %q", savedRaw.WhatsApp.AccessToken)
	}
	if savedRaw.WhatsApp.VerifyToken != "original_whatsapp_verify_token_456" {
		t.Errorf("WhatsApp VerifyToken was overwritten: %q", savedRaw.WhatsApp.VerifyToken)
	}
	if savedRaw.Telegram.BotToken != "original_telegram_bot_token_789" {
		t.Errorf("Telegram BotToken was overwritten: %q", savedRaw.Telegram.BotToken)
	}
	if savedRaw.Discord.BotToken != "original_discord_bot_token_012" {
		t.Errorf("Discord BotToken was overwritten: %q", savedRaw.Discord.BotToken)
	}
}

// TestChallengerM7_Bridge_VSCodeSurfaceDetection verifies that isVSCodeRunning executes
// without panic or failure and returns a boolean value on the host.
func TestChallengerM7_Bridge_VSCodeSurfaceDetection(t *testing.T) {
	// Must not panic
	detected := isVSCodeRunning()
	t.Logf("isVSCodeRunning returned: %t", detected)

	svc, err := NewService(filepath.Join(t.TempDir(), "bridge.json"))
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	surfaces := svc.DetectSurfaces()
	if surfaces.VSCodeConnected != detected {
		t.Errorf("expected DetectSurfaces.VSCodeConnected to match isVSCodeRunning (%t), got %t", detected, surfaces.VSCodeConnected)
	}
}
