package webgui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/bridge"
)

func setupTestA2AServer(t *testing.T) (*Server, *bridge.Service, string) {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "bridge.json")
	bridgeSvc, err := bridge.NewService(cfgPath)
	if err != nil {
		t.Fatalf("failed to create bridge service: %v", err)
	}

	srv := &Server{
		bridgeService: bridgeSvc,
	}

	authToken := bridgeSvc.GetRawAuthToken()
	return srv, bridgeSvc, authToken
}

func TestA2ARoutes_AgentCard(t *testing.T) {
	srv, _, _ := setupTestA2AServer(t)

	// 1. GET /.well-known/agent.json
	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent.json", nil)
	w := httptest.NewRecorder()
	srv.handleA2AAgentCard(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	var card bridge.A2AAgentCard
	if err := json.Unmarshal(w.Body.Bytes(), &card); err != nil {
		t.Fatalf("failed to parse agent card JSON: %v", err)
	}
	if card.Name != "Antigravity Swiss Knife" {
		t.Errorf("expected name 'Antigravity Swiss Knife', got '%s'", card.Name)
	}
	if card.ProtocolVersion != "0.3.0" {
		t.Errorf("expected protocol_version 0.3.0, got %s", card.ProtocolVersion)
	}
	if !card.Capabilities.AsyncTasks {
		t.Errorf("expected async_tasks to be true")
	}

	// 2. GET /a2a/agent.json
	req2 := httptest.NewRequest(http.MethodGet, "/a2a/agent.json", nil)
	w2 := httptest.NewRecorder()
	srv.handleA2AAgentCard(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /a2a/agent.json, got %d", w2.Code)
	}
}

func TestA2ARoutes_TaskLifecycle(t *testing.T) {
	srv, _, token := setupTestA2AServer(t)

	// 1. Unauthenticated POST /a2a/tasks must be rejected with 401
	body := []byte(`{"prompt":"Build A2A bridge endpoint","title":"A2A Task"}`)
	unauthReq := httptest.NewRequest(http.MethodPost, "/a2a/tasks", bytes.NewReader(body))
	wUnauth := httptest.NewRecorder()
	srv.handleA2ATasks(wUnauth, unauthReq)

	if wUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized without auth, got %d", wUnauth.Code)
	}

	// 2. Authenticated POST /a2a/tasks with Bearer token
	req := httptest.NewRequest(http.MethodPost, "/a2a/tasks", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	srv.handleA2ATasks(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var created bridge.A2ATask
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to parse task JSON: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected non-empty task ID")
	}
	if created.Prompt != "Build A2A bridge endpoint" {
		t.Errorf("expected prompt preserved, got: %s", created.Prompt)
	}

	// 3. GET /a2a/tasks/{id}
	getReq := httptest.NewRequest(http.MethodGet, "/a2a/tasks/"+created.ID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	wGet := httptest.NewRecorder()
	srv.handleA2ATasks(wGet, getReq)

	if wGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /a2a/tasks/{id}, got %d", wGet.Code)
	}

	// 4. GET /a2a/tasks (list)
	listReq := httptest.NewRequest(http.MethodGet, "/a2a/tasks", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	wList := httptest.NewRecorder()
	srv.handleA2ATasks(wList, listReq)

	if wList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /a2a/tasks, got %d", wList.Code)
	}
	var listResp struct {
		Tasks []bridge.A2ATask `json:"tasks"`
	}
	_ = json.Unmarshal(wList.Body.Bytes(), &listResp)
	if len(listResp.Tasks) == 0 {
		t.Errorf("expected at least 1 task in list")
	}

	// 5. POST /a2a/tasks/{id}/cancel
	cancelReq := httptest.NewRequest(http.MethodPost, "/a2a/tasks/"+created.ID+"/cancel", nil)
	cancelReq.Header.Set("Authorization", "Bearer "+token)
	wCancel := httptest.NewRecorder()
	srv.handleA2ATasks(wCancel, cancelReq)

	if wCancel.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for POST /a2a/tasks/{id}/cancel, got %d", wCancel.Code)
	}
	var canceled bridge.A2ATask
	_ = json.Unmarshal(wCancel.Body.Bytes(), &canceled)
	if canceled.Status != bridge.TaskStatusCanceled {
		t.Errorf("expected task status canceled, got %s", canceled.Status)
	}
}

func TestA2ARoutes_SSEEvents(t *testing.T) {
	srv, _, token := setupTestA2AServer(t)

	// Create task first
	body := []byte(`{"prompt":"SSE stream test"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/a2a/tasks", bytes.NewReader(body))
	createReq.Header.Set("Authorization", "Bearer "+token)
	wCreate := httptest.NewRecorder()
	srv.handleA2ATasks(wCreate, createReq)

	var created bridge.A2ATask
	_ = json.Unmarshal(wCreate.Body.Bytes(), &created)

	// Subscribe to SSE events
	sseReq := httptest.NewRequest(http.MethodGet, "/a2a/tasks/"+created.ID+"/events", nil)
	sseReq.Header.Set("Authorization", "Bearer "+token)

	// Use a mock response writer that implements http.Flusher
	wSSE := httptest.NewRecorder()

	go func() {
		time.Sleep(50 * time.Millisecond)
		// Cancel to end the event stream
		_, _ = srv.bridgeService.A2ATaskManager().CancelTask(created.ID)
	}()

	srv.handleA2ATasks(wSSE, sseReq)

	contentType := wSSE.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		t.Errorf("expected Content-Type text/event-stream, got %s", contentType)
	}
	t.Logf("SSE Body:\n%s", wSSE.Body.String())
}
