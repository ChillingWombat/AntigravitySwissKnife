package ls

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ChillingWombat/AntigravitySwissKnife/antigravity-acp/surface"
)

// fakeServer answers the four RPCs a turn needs and records what it was called with.
func fakeServer(t *testing.T, steps []map[string]interface{}) (*Surface, *[]string, *[]map[string]interface{}) {
	t.Helper()
	var calls []string
	var bodies []map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-codeium-csrf-token"); got != "tok" {
			t.Errorf("missing csrf header, got %q", got)
		}
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		calls = append(calls, method)
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)

		switch method {
		case "StartCascade":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"cascadeId": "cascade-1"})
		case "GetCascadeTrajectorySteps":
			offset, _ := body["stepOffset"].(float64)
			start := int(offset)
			if start > len(steps) {
				start = len(steps)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"steps": steps[start:]})
		case "GetAllCascadeTrajectories":
			// The server answers with a map keyed by cascade id.
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"trajectorySummaries": map[string]interface{}{
					"cascade-1": map[string]interface{}{
						"stepCount":        0,
						"status":           "CASCADE_RUN_STATUS_IDLE",
						"lastModifiedTime": "2026-10-22T13:58:26.003746406Z",
					},
				},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{})
		}
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, "tok"), &calls, &bodies
}

func TestStartSendsCascadeSource(t *testing.T) {
	s, _, bodies := fakeServer(t, nil)
	id, err := s.Start(context.Background(), surface.SessionOptions{Cwd: "/tmp/work"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if id != "cascade-1" {
		t.Errorf("cascade id = %q", id)
	}
	body := (*bodies)[0]
	if body["source"] != "CORTEX_TRAJECTORY_SOURCE_AGENT_API" {
		t.Errorf("source = %v", body["source"])
	}
	uris, _ := body["workspaceUris"].([]interface{})
	if len(uris) != 1 || uris[0] != "file:///tmp/work" {
		t.Errorf("workspaceUris = %v", body["workspaceUris"])
	}
}

func TestSendStreamsPlannerResponseOnce(t *testing.T) {
	steps := []map[string]interface{}{
		{"type": "CORTEX_STEP_TYPE_USER_INPUT"},
		{"type": "CORTEX_STEP_TYPE_PLANNER_RESPONSE", "plannerResponse": map[string]interface{}{"response": "hello there"}},
	}
	s, calls, _ := fakeServer(t, steps)
	var text []string
	reason, err := s.Send(context.Background(), "cascade-1", "hi", func(ev surface.Event) error {
		if ev.Kind == surface.EventText {
			text = append(text, ev.Text)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if reason != surface.StopEndTurn {
		t.Errorf("stop reason = %q", reason)
	}
	if strings.Join(text, "") != "hello there" {
		t.Errorf("text = %q", text)
	}
	seen := strings.Join(*calls, ",")
	if !strings.Contains(seen, "SendUserCascadeMessage") || !strings.Contains(seen, "GetCascadeTrajectorySteps") {
		t.Errorf("calls = %s", seen)
	}
}

func TestListReadsTrajectories(t *testing.T) {
	s, _, _ := fakeServer(t, nil)
	sessions, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != "cascade-1" {
		t.Errorf("sessions = %+v", sessions)
	}
}

// Selecting a model must change the planner model of the next turn.
func TestSetModelChangesPlannerModel(t *testing.T) {
	steps := []map[string]interface{}{
		{"type": "CORTEX_STEP_TYPE_PLANNER_RESPONSE", "plannerResponse": map[string]interface{}{"response": "done"}},
	}
	s, _, bodies := fakeServer(t, steps)
	if _, err := s.Start(context.Background(), surface.SessionOptions{Cwd: "/tmp"}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := s.SetModel(context.Background(), "cascade-1", "MODEL_PLACEHOLDER_M999"); err != nil {
		t.Fatalf("SetModel: %v", err)
	}
	if _, err := s.Send(context.Background(), "cascade-1", "hi", func(surface.Event) error { return nil }); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var planner string
	for _, body := range *bodies {
		config, _ := body["cascadeConfig"].(map[string]interface{})
		if config == nil {
			continue
		}
		plannerConfig, _ := config["plannerConfig"].(map[string]interface{})
		if plannerConfig != nil {
			planner, _ = plannerConfig["planModel"].(string)
		}
	}
	if planner != "MODEL_PLACEHOLDER_M999" {
		t.Errorf("planModel = %q, want the selected model", planner)
	}
}
