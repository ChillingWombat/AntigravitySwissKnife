package quota

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

func TestComputeHealthAndFormatting(t *testing.T) {
	if ComputeHealth(0.85) != core.StatusHealthy {
		t.Errorf("expected HEALTHY for 0.85, got %s", ComputeHealth(0.85))
	}
	if ComputeHealth(0.20) != core.StatusWarning {
		t.Errorf("expected WARNING for 0.20, got %s", ComputeHealth(0.20))
	}
	if ComputeHealth(0.04) != core.StatusExhausted {
		t.Errorf("expected EXHAUSTED for 0.04, got %s", ComputeHealth(0.04))
	}

	now := time.Now()
	resText := FormatResetHorizon(now.Add(15*time.Minute), now)
	if resText != "Resets in 15m" {
		t.Errorf("expected 'Resets in 15m', got %q", resText)
	}
}

func Test1TokenPayloadStructure(t *testing.T) {
	data, err := Build1TokenKeepAliveJSON()
	if err != nil {
		t.Fatalf("Build1TokenKeepAliveJSON error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed["project"] != "aicode-consumers" {
		t.Errorf("expected project 'aicode-consumers', got %v", parsed["project"])
	}
	if parsed["model"] != "gemini-3-flash" {
		t.Errorf("expected model 'gemini-3-flash', got %v", parsed["model"])
	}

	reqObj, ok := parsed["request"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing request object")
	}

	cfg, ok := reqObj["generationConfig"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing generationConfig")
	}
	if cfg["maxOutputTokens"] != float64(1) {
		t.Errorf("expected maxOutputTokens: 1, got %v", cfg["maxOutputTokens"])
	}
}

func TestWarmupSchedulerWindow(t *testing.T) {
	ws := &WarmupScheduler{PostResetDelay: 2 * time.Second}
	reset := time.Now().Add(5 * time.Second)

	// In cooldown (5 seconds ahead of reset) -> ShouldSkipCooldownPolling MUST be true
	if !ws.ShouldSkipCooldownPolling(reset, time.Now()) {
		t.Errorf("expected ShouldSkipCooldownPolling to be true while now < reset")
	}

	// Ahead of reset -> ShouldTriggerPostResetIgnition MUST be false
	if ws.ShouldTriggerPostResetIgnition(reset, time.Now()) {
		t.Errorf("should not trigger ignition before reset")
	}

	// 1 second after reset (within 2s delay) -> ShouldTriggerPostResetIgnition MUST be false
	if ws.ShouldTriggerPostResetIgnition(reset, reset.Add(1*time.Second)) {
		t.Errorf("should not trigger ignition before post-reset delay has elapsed")
	}

	// 2 seconds after reset (delay elapsed) -> ShouldTriggerPostResetIgnition MUST be true
	if !ws.ShouldTriggerPostResetIgnition(reset, reset.Add(2*time.Second)) {
		t.Errorf("should trigger ignition once post-reset delay elapsed")
	}

	// 3 seconds after reset -> ShouldTriggerPostResetIgnition MUST be true
	if !ws.ShouldTriggerPostResetIgnition(reset, reset.Add(3*time.Second)) {
		t.Errorf("should trigger ignition after post-reset delay elapsed")
	}

	// Cooldown skip should be false once reset time arrives
	if ws.ShouldSkipCooldownPolling(reset, reset.Add(1*time.Second)) {
		t.Errorf("expected ShouldSkipCooldownPolling to be false once reset time passed")
	}
}

func TestWarmupScheduler_ZeroResetTimeAndAliases(t *testing.T) {
	ws := &WarmupScheduler{LeadTime: 5 * time.Second} // verify LeadTime fallback alias
	now := time.Now()

	// Zero reset time
	var zeroReset time.Time
	if ws.ShouldSkipCooldownPolling(zeroReset, now) {
		t.Errorf("zero reset time must not skip cooldown polling")
	}
	if ws.ShouldTriggerPostResetIgnition(zeroReset, now) {
		t.Errorf("zero reset time must not trigger post-reset ignition")
	}

	// Verify LeadTime fallback when PostResetDelay is 0
	if ws.GetDelay() != 5*time.Second {
		t.Errorf("expected GetDelay to return 5s from LeadTime alias, got %v", ws.GetDelay())
	}

	ws.PostResetDelay = 10 * time.Second
	if ws.GetDelay() != 10*time.Second {
		t.Errorf("expected GetDelay to prioritize PostResetDelay (10s), got %v", ws.GetDelay())
	}
}

func TestSend1TokenKeepAliveProbe_MockServer(t *testing.T) {
	var receivedBody []byte
	var receivedAuth string
	var receivedContentType string

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		receivedContentType = r.Header.Get("Content-Type")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"candidates":[]}`))
	}))
	defer mockServer.Close()

	oldURLs := CloudCodeGenerateContentURLs
	CloudCodeGenerateContentURLs = []string{mockServer.URL}
	defer func() {
		CloudCodeGenerateContentURLs = oldURLs
	}()

	err := Send1TokenKeepAliveProbe("mock-token-xyz")
	if err != nil {
		t.Fatalf("Send1TokenKeepAliveProbe error: %v", err)
	}

	if receivedAuth != "Bearer mock-token-xyz" {
		t.Errorf("expected 'Bearer mock-token-xyz', got %q", receivedAuth)
	}
	if receivedContentType != "application/json" {
		t.Errorf("expected 'application/json', got %q", receivedContentType)
	}

	var parsed OneTokenPayload
	if err := json.Unmarshal(receivedBody, &parsed); err != nil {
		t.Fatalf("failed to parse received payload: %v", err)
	}
	if len(parsed.Request.Contents) == 0 || len(parsed.Request.Contents[0].Parts) == 0 || parsed.Request.Contents[0].Parts[0].Text != "ping" {
		t.Errorf("expected text 'ping', got %+v", parsed.Request.Contents)
	}
	if parsed.Request.GenerationConfig.MaxOutputTokens != 1 {
		t.Errorf("expected maxOutputTokens: 1, got %d", parsed.Request.GenerationConfig.MaxOutputTokens)
	}
}
