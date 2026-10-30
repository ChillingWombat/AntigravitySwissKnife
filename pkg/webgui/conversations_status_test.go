package webgui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/revival"
)

// TestConversationsStatus_EmitsEveryPendingIntent proves /api/conversations/status
// publishes the whole pending set, not just the newest intent, because the in-page
// dispatcher drains one conversation per pass from that array.
func TestConversationsStatus_EmitsEveryPendingIntent(t *testing.T) {
	tmpDir := t.TempDir()
	srv := newTestServer("127.0.0.1:0", filepath.Join(tmpDir, "isolated.sock"))
	engine := revival.NewEngine(tmpDir, 0)
	// Point the detector at the temp dir: the default detector reads the real
	// Antigravity data dir, which would make the test depend on whatever the
	// developer has running.
	engine.Detector.BaseDir = tmpDir
	engine.Detector.LiveCDPEnabled = false
	srv.SetRevivalEngine(engine)

	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	convIDs := []string{
		"11111111-1111-4111-8111-111111111111",
		"22222222-2222-4222-8222-222222222222",
		"33333333-3333-4333-8333-333333333333",
	}
	base := time.Now()
	for i, convID := range convIDs {
		err := engine.Store.SaveIntent(&revival.RevivalIntent{
			IntentID:           "intent-" + convID,
			TargetApp:          "desktop",
			RootConversationID: convID,
			TriggerPrompt:      revival.DefaultTriggerPrompt,
			CreatedAt:          base.Add(time.Duration(i) * time.Second),
			Status:             revival.IntentStatusPending,
			TTLSeconds:         600,
		})
		if err != nil {
			t.Fatalf("SaveIntent(%s) failed: %v", convID, err)
		}
	}

	resp, err := http.Get("http://" + srv.Addr() + "/api/conversations/status")
	if err != nil {
		t.Fatalf("GET /api/conversations/status failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var payload struct {
		PendingIntents []struct {
			IntentID           string `json:"intent_id"`
			RootConversationID string `json:"root_conversation_id"`
			TriggerPrompt      string `json:"trigger_prompt"`
		} `json:"pending_intents"`
		PendingIntent *struct {
			RootConversationID string `json:"root_conversation_id"`
		} `json:"pending_intent"`
		CascadeID string `json:"cascade_id"`
		Prompt    string `json:"prompt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode status payload: %v", err)
	}

	if len(payload.PendingIntents) != 3 {
		t.Fatalf("expected all 3 pending intents, got %d: %+v", len(payload.PendingIntents), payload.PendingIntents)
	}
	seen := map[string]bool{}
	for _, intent := range payload.PendingIntents {
		seen[intent.RootConversationID] = true
		if intent.TriggerPrompt == "" {
			t.Errorf("intent %s lost its prompt in the status payload", intent.IntentID)
		}
	}
	for _, convID := range convIDs {
		if !seen[convID] {
			t.Errorf("status payload is missing pending intent for %s", convID)
		}
	}

	// The legacy single fields stay populated from the newest intent, and the
	// newest is convIDs[2] (latest CreatedAt).
	wantNewest := convIDs[2]
	if payload.PendingIntent == nil || payload.PendingIntent.RootConversationID != wantNewest {
		t.Errorf("expected legacy pending_intent to mirror the newest intent %s, got %v", wantNewest, payload.PendingIntent)
	}
	if payload.CascadeID != wantNewest {
		t.Errorf("expected legacy cascade_id %s, got %s", wantNewest, payload.CascadeID)
	}
	if payload.Prompt == "" {
		t.Errorf("expected legacy prompt to be populated")
	}
}

// TestConversationsAck_ClearsOnlyTheAckedIntent proves the ack endpoint is scoped:
// reviving one conversation must not silently discard the others.
func TestConversationsAck_ClearsOnlyTheAckedIntent(t *testing.T) {
	tmpDir := t.TempDir()
	srv := newTestServer("127.0.0.1:0", filepath.Join(tmpDir, "isolated.sock"))
	engine := revival.NewEngine(tmpDir, 0)
	// Point the detector at the temp dir: the default detector reads the real
	// Antigravity data dir, which would make the test depend on whatever the
	// developer has running.
	engine.Detector.BaseDir = tmpDir
	engine.Detector.LiveCDPEnabled = false
	srv.SetRevivalEngine(engine)

	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	convA := "aaaa1111-1111-4111-8111-111111111111"
	convB := "bbbb2222-2222-4222-8222-222222222222"
	now := time.Now()
	for i, convID := range []string{convA, convB} {
		err := engine.Store.SaveIntent(&revival.RevivalIntent{
			IntentID:           "intent-" + convID,
			TargetApp:          "desktop",
			RootConversationID: convID,
			TriggerPrompt:      revival.DefaultTriggerPrompt,
			CreatedAt:          now.Add(time.Duration(i) * time.Second),
			Status:             revival.IntentStatusPending,
			TTLSeconds:         600,
		})
		if err != nil {
			t.Fatalf("SaveIntent(%s) failed: %v", convID, err)
		}
	}

	body, _ := json.Marshal(map[string]string{"cascade_id": convA, "intent_id": "intent-" + convA})
	resp, err := http.Post("http://"+srv.Addr()+"/api/conversations/ack", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/conversations/ack failed: %v", err)
	}
	resp.Body.Close()

	remaining, err := engine.Store.LoadIntents()
	if err != nil {
		t.Fatalf("LoadIntents failed: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected exactly one surviving intent, got %d", len(remaining))
	}
	if remaining[0].RootConversationID != convB {
		t.Errorf("expected the un-acked conversation %s to survive, got %s", convB, remaining[0].RootConversationID)
	}
}
