package quota

import (
	"encoding/json"
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

	cfg, ok := parsed["generationConfig"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing generationConfig")
	}
	if cfg["maxOutputTokens"] != float64(1) {
		t.Errorf("expected maxOutputTokens: 1, got %v", cfg["maxOutputTokens"])
	}
}

func TestWarmupSchedulerWindow(t *testing.T) {
	ws := &WarmupScheduler{LeadTime: 2 * time.Second}
	reset := time.Now().Add(5 * time.Second)

	// 5 seconds away -> should NOT trigger yet
	if ws.ShouldTrigger(reset, time.Now()) {
		t.Errorf("should not trigger 5s ahead of reset")
	}

	// 1 second away -> SHOULD trigger
	if !ws.ShouldTrigger(reset, reset.Add(-1*time.Second)) {
		t.Errorf("should trigger within 2s lead time")
	}
}
