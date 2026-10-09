package quota

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

// TestAdversarial_WarmupScheduler_Invariants thoroughly validates all mathematical
// boundaries and edge cases for ShouldSkipCooldownPolling and ShouldTriggerPostResetIgnition.
func TestAdversarial_WarmupScheduler_Invariants(t *testing.T) {
	ws := &WarmupScheduler{PostResetDelay: 90 * time.Second}
	resetTime := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)

	// 1. ShouldSkipCooldownPolling tests:
	// Invariant: strictly true when now < resetTime, strictly false when now >= resetTime
	testCasesCooldown := []struct {
		name     string
		now      time.Time
		expected bool
	}{
		{"1 hour before reset", resetTime.Add(-1 * time.Hour), true},
		{"1 second before reset", resetTime.Add(-1 * time.Second), true},
		{"1 nanosecond before reset", resetTime.Add(-1 * time.Nanosecond), true},
		{"exact reset time", resetTime, false},
		{"1 nanosecond after reset", resetTime.Add(1 * time.Nanosecond), false},
		{"1 second after reset", resetTime.Add(1 * time.Second), false},
		{"1 hour after reset", resetTime.Add(1 * time.Hour), false},
	}

	for _, tc := range testCasesCooldown {
		t.Run("CooldownSkip_"+tc.name, func(t *testing.T) {
			got := ws.ShouldSkipCooldownPolling(resetTime, tc.now)
			if got != tc.expected {
				t.Errorf("ShouldSkipCooldownPolling(%v, %v) = %v; expected %v", resetTime, tc.now, got, tc.expected)
			}
		})
	}

	// 2. ShouldTriggerPostResetIgnition tests:
	// Invariant: strictly false when now < resetTime + delay, strictly true when now >= resetTime + delay
	delay := ws.GetDelay()
	ignitionTarget := resetTime.Add(delay)

	testCasesIgnition := []struct {
		name     string
		now      time.Time
		expected bool
	}{
		{"before reset time", resetTime.Add(-10 * time.Second), false},
		{"exact reset time", resetTime, false},
		{"midway through delay window (45s)", resetTime.Add(45 * time.Second), false},
		{"1 nanosecond before delay window expires", ignitionTarget.Add(-1 * time.Nanosecond), false},
		{"exact ignition target (reset + delay)", ignitionTarget, true},
		{"1 nanosecond after ignition target", ignitionTarget.Add(1 * time.Nanosecond), true},
		{"10 seconds after ignition target", ignitionTarget.Add(10 * time.Second), true},
		{"2 hours after ignition target", ignitionTarget.Add(2 * time.Hour), true},
	}

	for _, tc := range testCasesIgnition {
		t.Run("IgnitionTrigger_"+tc.name, func(t *testing.T) {
			got := ws.ShouldTriggerPostResetIgnition(resetTime, tc.now)
			if got != tc.expected {
				t.Errorf("ShouldTriggerPostResetIgnition(%v, %v) = %v; expected %v", resetTime, tc.now, got, tc.expected)
			}
		})
	}
}

// TestAdversarial_WarmupScheduler_ZeroTimeGraceful ensures zero reset times
// never panic and consistently return false.
func TestAdversarial_WarmupScheduler_ZeroTimeGraceful(t *testing.T) {
	ws := &WarmupScheduler{PostResetDelay: 60 * time.Second}
	var zeroTime time.Time
	now := time.Now()

	// ShouldSkipCooldownPolling with zero reset time
	if ws.ShouldSkipCooldownPolling(zeroTime, now) {
		t.Errorf("expected ShouldSkipCooldownPolling to return false for zero resetTime")
	}

	// ShouldTriggerPostResetIgnition with zero reset time
	if ws.ShouldTriggerPostResetIgnition(zeroTime, now) {
		t.Errorf("expected ShouldTriggerPostResetIgnition to return false for zero resetTime")
	}

	// Both with now also zero
	if ws.ShouldSkipCooldownPolling(zeroTime, zeroTime) {
		t.Errorf("expected ShouldSkipCooldownPolling to return false when both times zero")
	}
	if ws.ShouldTriggerPostResetIgnition(zeroTime, zeroTime) {
		t.Errorf("expected ShouldTriggerPostResetIgnition to return false when both times zero")
	}

	// Nil scheduler safety
	var nilWs *WarmupScheduler
	if nilWs.ShouldSkipCooldownPolling(zeroTime, now) {
		t.Errorf("expected nil WarmupScheduler ShouldSkipCooldownPolling to return false for zero resetTime")
	}
	if nilWs.ShouldTriggerPostResetIgnition(zeroTime, now) {
		t.Errorf("expected nil WarmupScheduler ShouldTriggerPostResetIgnition to return false for zero resetTime")
	}
}

// TestAdversarial_WarmupScheduler_ZeroDelay tests the boundary condition when delay is 0.
func TestAdversarial_WarmupScheduler_ZeroDelay(t *testing.T) {
	ws := &WarmupScheduler{PostResetDelay: 0, LeadTime: 0}
	resetTime := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)

	// When delay is 0:
	// - before reset: false
	if ws.ShouldTriggerPostResetIgnition(resetTime, resetTime.Add(-1*time.Millisecond)) {
		t.Errorf("expected false before reset time even when delay is 0")
	}
	// - exact reset: true
	if !ws.ShouldTriggerPostResetIgnition(resetTime, resetTime) {
		t.Errorf("expected true at exact reset time when delay is 0")
	}
	// - after reset: true
	if !ws.ShouldTriggerPostResetIgnition(resetTime, resetTime.Add(1*time.Millisecond)) {
		t.Errorf("expected true after reset time when delay is 0")
	}
}

// TestAdversarial_1TokenPayload_ExactJSONStructure enforces the exact wire JSON representation
// of the 1-token keep-alive payload.
func TestAdversarial_1TokenPayload_ExactJSONStructure(t *testing.T) {
	rawJSON, err := Build1TokenKeepAliveJSON()
	if err != nil {
		t.Fatalf("Build1TokenKeepAliveJSON failed: %v", err)
	}

	expectedJSON := `{"contents":[{"parts":[{"text":"ping"}]}],"generationConfig":{"maxOutputTokens":1}}`
	if string(rawJSON) != expectedJSON {
		t.Errorf("Payload mismatch:\ngot:      %s\nexpected: %s", string(rawJSON), expectedJSON)
	}

	// Decode into generic map to verify strictly no extra or missing keys
	var m map[string]interface{}
	if err := json.Unmarshal(rawJSON, &m); err != nil {
		t.Fatalf("Failed to parse JSON map: %v", err)
	}

	if len(m) != 2 {
		t.Errorf("expected exactly 2 top-level keys ('contents', 'generationConfig'), got %d: %v", len(m), m)
	}

	contents, ok := m["contents"].([]interface{})
	if !ok || len(contents) != 1 {
		t.Fatalf("expected contents to be array of length 1, got %v", m["contents"])
	}

	contentObj, ok := contents[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected contents[0] to be object, got %T", contents[0])
	}

	parts, ok := contentObj["parts"].([]interface{})
	if !ok || len(parts) != 1 {
		t.Fatalf("expected parts to be array of length 1, got %v", contentObj["parts"])
	}

	partObj, ok := parts[0].(map[string]interface{})
	if !ok || partObj["text"] != "ping" {
		t.Errorf("expected part text 'ping', got %v", partObj)
	}

	genConfig, ok := m["generationConfig"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected generationConfig object, got %T", m["generationConfig"])
	}

	if maxTokens, ok := genConfig["maxOutputTokens"].(float64); !ok || int(maxTokens) != 1 {
		t.Errorf("expected maxOutputTokens 1, got %v", genConfig["maxOutputTokens"])
	}
}

// TestAdversarial_Send1TokenKeepAliveProbe_Robustness checks error handling
// on empty token, network failures, and server errors.
func TestAdversarial_Send1TokenKeepAliveProbe_Robustness(t *testing.T) {
	// 1. Empty token
	if err := Send1TokenKeepAliveProbe(""); err == nil {
		t.Errorf("expected error when access token is empty")
	}

	// 2. Server 500 error
	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer errServer.Close()

	oldURLs := CloudCodeGenerateContentURLs
	CloudCodeGenerateContentURLs = []string{errServer.URL}
	defer func() { CloudCodeGenerateContentURLs = oldURLs }()

	err := Send1TokenKeepAliveProbe("valid-token")
	if err == nil {
		t.Errorf("expected error when server returns 500, got nil")
	}

	// 3. Fallback to secondary server
	okServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer okServer.Close()

	CloudCodeGenerateContentURLs = []string{errServer.URL, okServer.URL}
	err = Send1TokenKeepAliveProbe("valid-token")
	if err != nil {
		t.Errorf("expected success when fallback server returns 200, got %v", err)
	}
}

// TestAdversarial_IgniteAccountPostReset_Safety validates nil and missing token safeguards.
func TestAdversarial_IgniteAccountPostReset_Safety(t *testing.T) {
	if err := IgniteAccountPostReset(nil, nil); err == nil {
		t.Errorf("expected error for nil account")
	}

	accNoTokens := &keyring.Account{
		Email: "no-tokens@example.com",
	}
	if err := IgniteAccountPostReset(accNoTokens, nil); err == nil {
		t.Errorf("expected error for account without access or refresh token")
	}
}

// TestAdversarial_QuotaSummary_CooldownAndResetTime checks GetResetTime & IsCooldown invariants.
func TestAdversarial_QuotaSummary_CooldownAndResetTime(t *testing.T) {
	threshold := 0.20

	// 1. Nil QuotaSummary
	var nilQS *QuotaSummary
	if nilQS.IsCooldown(threshold) {
		t.Errorf("nil QuotaSummary should not be cooldown")
	}
	if !nilQS.GetResetTime().IsZero() {
		t.Errorf("nil QuotaSummary GetResetTime should be zero")
	}

	// 2. Exhausted status
	qsExhausted := &QuotaSummary{
		OverallHealth: core.StatusExhausted,
		MinFraction:   0.80, // fraction is high, but health is exhausted
	}
	if !qsExhausted.IsCooldown(threshold) {
		t.Errorf("StatusExhausted MUST be in cooldown regardless of fraction")
	}

	// 3. Fraction below threshold
	qsLowFraction := &QuotaSummary{
		OverallHealth: core.StatusHealthy,
		MinFraction:   0.15,
	}
	if !qsLowFraction.IsCooldown(threshold) {
		t.Errorf("MinFraction (0.15) <= threshold (0.20) MUST be in cooldown")
	}

	// 4. Rate limit error status
	qsRateLimit := &QuotaSummary{
		OverallHealth: core.StatusHealthy,
		MinFraction:   0.90,
		ErrorStatus:   "RATE_LIMIT_EXCEEDED",
	}
	if !qsRateLimit.IsCooldown(threshold) {
		t.Errorf("RATE_LIMIT_EXCEEDED MUST be in cooldown")
	}

	// 5. Healthy quota above threshold
	qsHealthy := &QuotaSummary{
		OverallHealth: core.StatusHealthy,
		MinFraction:   0.85,
	}
	if qsHealthy.IsCooldown(threshold) {
		t.Errorf("Healthy quota above threshold MUST NOT be in cooldown")
	}

	// 6. Earliest reset time selection across models
	t1 := time.Date(2026, 10, 10, 14, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 10, 10, 13, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 10, 10, 15, 0, 0, 0, time.UTC)

	qsModels := &QuotaSummary{
		Models: []ModelQuota{
			{ModelName: "model-1", ResetTime: t1},
			{ModelName: "model-2", ResetTime: t2}, // earliest
			{ModelName: "model-3", ResetTime: t3},
		},
	}
	if qsModels.GetResetTime() != t2 {
		t.Errorf("expected earliest reset time %v, got %v", t2, qsModels.GetResetTime())
	}
}
