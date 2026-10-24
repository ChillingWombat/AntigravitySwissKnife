package ls

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
						"workspaceUris":    []string{"file:///tmp/work%20space"},
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
	if sessions[0].Cwd != "/tmp/work space" {
		t.Errorf("cwd = %q, want /tmp/work space", sessions[0].Cwd)
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

func TestSendSeedsOffsetBeforePrompt(t *testing.T) {
	var mu sync.Mutex
	steps := []map[string]interface{}{
		{"type": "CORTEX_STEP_TYPE_USER_INPUT"},
	}
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)

		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, method)
		switch method {
		case "GetAllCascadeTrajectories":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"trajectorySummaries": map[string]interface{}{
					"cascade-1": map[string]interface{}{
						"stepCount": len(steps),
						"status":    "CASCADE_RUN_STATUS_IDLE",
					},
				},
			})
		case "SendUserCascadeMessage":
			steps = append(steps, map[string]interface{}{
				"type":            "CORTEX_STEP_TYPE_PLANNER_RESPONSE",
				"plannerResponse": map[string]interface{}{"response": "fresh answer"},
			})
			_ = json.NewEncoder(w).Encode(map[string]interface{}{})
		case "GetCascadeTrajectorySteps":
			offset, _ := body["stepOffset"].(float64)
			start := int(offset)
			if start > len(steps) {
				start = len(steps)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"steps": steps[start:]})
		default:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{})
		}
	}))
	t.Cleanup(srv.Close)
	s := New(srv.URL, "tok")

	var text []string
	if _, err := s.Send(context.Background(), "cascade-1", "hi", func(ev surface.Event) error {
		if ev.Kind == surface.EventText {
			text = append(text, ev.Text)
		}
		return nil
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if strings.Join(text, "") != "fresh answer" {
		t.Errorf("text = %q, want fresh answer", text)
	}

	mu.Lock()
	gotCalls := append([]string(nil), calls...)
	mu.Unlock()
	seedAt, promptAt := -1, -1
	for i, call := range gotCalls {
		switch call {
		case "GetAllCascadeTrajectories":
			if seedAt == -1 {
				seedAt = i
			}
		case "SendUserCascadeMessage":
			promptAt = i
		}
	}
	if seedAt == -1 || promptAt == -1 || seedAt > promptAt {
		t.Errorf("calls = %v, want offset seed before prompt", gotCalls)
	}
}

func TestSendWaitsForIdleAfterText(t *testing.T) {
	var mu sync.Mutex
	steps := []map[string]interface{}{
		{"type": "CORTEX_STEP_TYPE_USER_INPUT"},
	}
	statusCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)

		mu.Lock()
		defer mu.Unlock()
		switch method {
		case "SendUserCascadeMessage":
			steps = append(steps, map[string]interface{}{
				"type":            "CORTEX_STEP_TYPE_PLANNER_RESPONSE",
				"plannerResponse": map[string]interface{}{"response": "working"},
			})
			_ = json.NewEncoder(w).Encode(map[string]interface{}{})
		case "GetCascadeTrajectorySteps":
			offset, _ := body["stepOffset"].(float64)
			start := int(offset)
			if start > len(steps) {
				start = len(steps)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"steps": steps[start:]})
		case "GetAllCascadeTrajectories":
			statusCalls++
			status := "CASCADE_RUN_STATUS_RUNNING"
			if statusCalls >= 6 {
				status = "CASCADE_RUN_STATUS_IDLE"
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"trajectorySummaries": map[string]interface{}{
					"cascade-1": map[string]interface{}{
						"stepCount": len(steps),
						"status":    status,
					},
				},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{})
		}
	}))
	t.Cleanup(srv.Close)
	s := New(srv.URL, "tok")
	s.offsets["cascade-1"] = 1

	var text []string
	if _, err := s.Send(context.Background(), "cascade-1", "hi", func(ev surface.Event) error {
		if ev.Kind == surface.EventText {
			text = append(text, ev.Text)
		}
		return nil
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if strings.Join(text, "") != "working" {
		t.Errorf("text = %q, want working", text)
	}

	mu.Lock()
	gotStatusCalls := statusCalls
	mu.Unlock()
	if gotStatusCalls < 6 {
		t.Errorf("status checks = %d, want at least 6 before ending the turn", gotStatusCalls)
	}
}

func TestStartEncodesWorkspaceURISpaces(t *testing.T) {
	s, _, bodies := fakeServer(t, nil)
	if _, err := s.Start(context.Background(), surface.SessionOptions{Cwd: "/tmp/work with spaces"}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	uris, _ := (*bodies)[0]["workspaceUris"].([]interface{})
	if len(uris) != 1 || uris[0] != "file:///tmp/work%20with%20spaces" {
		t.Errorf("workspaceUris = %v", (*bodies)[0]["workspaceUris"])
	}
}
