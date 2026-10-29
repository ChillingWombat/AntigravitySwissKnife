package ls

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ChillingWombat/AntigravitySwissKnife/antigravity-acp/surface"
)

// historyServer answers GetCascadeTrajectorySteps with a fixed trajectory.
func historyServer(t *testing.T, steps []map[string]interface{}) *Surface {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != servicePath+"GetCascadeTrajectorySteps" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"steps": steps})
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, "tok")
}

// A conversation replayed from the trajectory must carry both speakers, in order.
func TestHistoryReplaysUserAndPlannerTurnsInOrder(t *testing.T) {
	s := historyServer(t, []map[string]interface{}{
		{
			"type":      "CORTEX_STEP_TYPE_USER_INPUT",
			"userInput": map[string]interface{}{"userResponse": "remember BANANA7"},
		},
		{"type": "CORTEX_STEP_TYPE_PLANNER_RESPONSE", "plannerResponse": map[string]interface{}{"response": "stored"}},
		{ // tool steps carry no text and must not produce events
			"type": "CORTEX_STEP_TYPE_RUN_COMMAND",
		},
		{
			"type":      "CORTEX_STEP_TYPE_USER_INPUT",
			"userInput": map[string]interface{}{"items": []map[string]interface{}{{"text": "what word?"}}},
		},
		{"type": "CORTEX_STEP_TYPE_PLANNER_RESPONSE", "plannerResponse": map[string]interface{}{"response": "BANANA7"}},
	})

	events, err := s.History(context.Background(), "cascade-1")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	want := []struct {
		kind surface.EventKind
		text string
	}{
		{surface.EventUserText, "remember BANANA7"},
		{surface.EventText, "stored"},
		{surface.EventUserText, "what word?"},
		{surface.EventText, "BANANA7"},
	}
	if len(events) != len(want) {
		t.Fatalf("got %d events, want %d: %+v", len(events), len(want), events)
	}
	for i, expected := range want {
		if events[i].Kind != expected.kind || events[i].Text != expected.text {
			t.Errorf("event %d = (%v, %q), want (%v, %q)", i, events[i].Kind, events[i].Text, expected.kind, expected.text)
		}
	}
}

// Steps recorded before userResponse was populated still carry the prompt in items.
func TestHistoryFallsBackToInputItems(t *testing.T) {
	s := historyServer(t, []map[string]interface{}{
		{
			"type": "CORTEX_STEP_TYPE_USER_INPUT",
			"userInput": map[string]interface{}{
				"items": []map[string]interface{}{{"text": " first "}, {"text": ""}, {"text": "second"}},
			},
		},
	})

	events, err := s.History(context.Background(), "cascade-1")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Kind != surface.EventUserText {
		t.Errorf("kind = %v, want EventUserText", events[0].Kind)
	}
	if events[0].Text != "first\nsecond" {
		t.Errorf("text = %q, want %q", events[0].Text, "first\nsecond")
	}
}
