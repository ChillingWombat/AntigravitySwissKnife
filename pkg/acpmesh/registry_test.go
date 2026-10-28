package acpmesh

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

func TestMatchRegistryAgentToPreset(t *testing.T) {
	tests := []struct {
		remoteID string
		wantID   string
		matched  bool
	}{
		{"antigravity-acp", PresetAntigravityACP, true},
		{"agent-antigravity-acp", PresetAntigravityACP, true},
		{"claude-acp", PresetClaudeCode, true},
		{"codex-acp", PresetCodex, true},
		{"cursor", PresetCursor, true},
		{"devin", PresetDevin, true},
		{"opencode", PresetOpenCode, true},
		{"pi-acp", PresetPi, true},
		{"deepseek-harness", PresetDeepSeek, true},
		{"nonexistent-agent-xyz", "", false},
	}

	for _, tc := range tests {
		gotID, matched := MatchRegistryAgentToPreset(RemoteACPAgent{ID: tc.remoteID})
		if matched != tc.matched {
			t.Errorf("MatchRegistryAgentToPreset(%q) matched=%v, want %v", tc.remoteID, matched, tc.matched)
		}
		if gotID != tc.wantID {
			t.Errorf("MatchRegistryAgentToPreset(%q) = %q, want %q", tc.remoteID, gotID, tc.wantID)
		}
	}
}

func TestSyncRegistryWithMockServer(t *testing.T) {
	mockJSON := `{
		"version": "1.0.0",
		"agents": [
			{
				"id": "antigravity-acp",
				"name": "Google Antigravity",
				"version": "1.4.3"
			},
			{
				"id": "cursor",
				"name": "Cursor",
				"version": "v2026.10.22"
			},
			{
				"id": "unknown-future-agent",
				"name": "Future Agent",
				"version": "0.1.0"
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockJSON))
	}))
	defer server.Close()

	initialOverrides := []core.ACPAgentConfig{
		{
			ID:     "custom-agent-1",
			Name:   "My Agent",
			Custom: true,
		},
	}

	updated, count, err := SyncRegistry(context.Background(), server.URL, initialOverrides)
	if err != nil {
		t.Fatalf("SyncRegistry failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected count=2, got %d", count)
	}

	var antigravityOverride, cursorOverride, customOverride *core.ACPAgentConfig
	for i := range updated {
		if updated[i].ID == PresetAntigravityACP {
			antigravityOverride = &updated[i]
		}
		if updated[i].ID == PresetCursor {
			cursorOverride = &updated[i]
		}
		if updated[i].ID == "custom-agent-1" {
			customOverride = &updated[i]
		}
	}

	if antigravityOverride == nil {
		t.Fatal("PresetAntigravityACP override missing")
	}
	if antigravityOverride.Version != "v1.4.3" {
		t.Errorf("antigravity version = %q, want v1.4.3", antigravityOverride.Version)
	}

	if cursorOverride == nil {
		t.Fatal("PresetCursor override missing")
	}
	if cursorOverride.Version != "v2026.10.22" {
		t.Errorf("cursor version = %q, want v2026.10.22", cursorOverride.Version)
	}

	if customOverride == nil {
		t.Fatal("custom agent override was lost")
	}
}
