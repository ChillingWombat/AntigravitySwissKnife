package bridge

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

type mockAgentBackend struct {
	mu           sync.Mutex
	name         string
	created      []map[string]interface{}
	sent         []map[string]interface{}
	stopped      []map[string]interface{}
	mockResp     string
	mockArtifact []string
	delay        time.Duration
}

func newMockBackend() *mockAgentBackend {
	return &mockAgentBackend{
		name:         "mock",
		mockResp:     "Task completed successfully by Antigravity.",
		mockArtifact: []string{"main.go", "main_test.go"},
	}
}

func (m *mockAgentBackend) Name() string { return m.name }

func (m *mockAgentBackend) CreateConversation(args map[string]interface{}) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.created = append(m.created, args)
	res := map[string]interface{}{
		"status":          "dispatched",
		"conversation_id": "conv-test-123",
		"submission_step": 0,
	}
	b, _ := json.Marshal(res)
	return string(b), nil
}

func (m *mockAgentBackend) SendMessage(args map[string]interface{}) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, args)
	res := map[string]interface{}{
		"status":          "message_sent",
		"conversation_id": args["conversation_id"],
	}
	b, _ := json.Marshal(res)
	return string(b), nil
}

func (m *mockAgentBackend) GetStatus(args map[string]interface{}) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := map[string]interface{}{
		"status":        "completed",
		"is_generating": false,
		"artifacts":     m.mockArtifact,
	}
	b, _ := json.Marshal(res)
	return string(b), nil
}

func (m *mockAgentBackend) AwaitResponse(args map[string]interface{}) (string, error) {
	m.mu.Lock()
	delay := m.delay
	resp := m.mockResp
	artifacts := m.mockArtifact
	m.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}

	res := map[string]interface{}{
		"status":          "completed",
		"conversation_id": "conv-test-123",
		"response":        resp,
		"artifacts":       artifacts,
	}
	b, _ := json.Marshal(res)
	return string(b), nil
}

func (m *mockAgentBackend) ListConversations(args map[string]interface{}) (string, error) {
	return "[]", nil
}

func (m *mockAgentBackend) CompactConversation(args map[string]interface{}) (string, error) {
	return `{"status":"compacted"}`, nil
}

func (m *mockAgentBackend) StopConversation(args map[string]interface{}) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = append(m.stopped, args)
	return `{"status":"stopped"}`, nil
}

func (m *mockAgentBackend) ListModels(args map[string]interface{}) (string, error) {
	return "[]", nil
}

func (m *mockAgentBackend) GetModel(args map[string]interface{}) (string, error) {
	return "{}", nil
}

func (m *mockAgentBackend) SetModel(args map[string]interface{}) (string, error) {
	return "{}", nil
}

func TestA2AAgentCard_Descriptor(t *testing.T) {
	card := DefaultA2AAgentCard("https://my-tunnel.ts.net:8765")

	if card.Name != "Antigravity Swiss Knife" {
		t.Errorf("unexpected agent name: %s", card.Name)
	}
	if card.ProtocolVersion != "0.3.0" {
		t.Errorf("unexpected protocol version: %s", card.ProtocolVersion)
	}
	if !strings.HasPrefix(card.URL, "https://my-tunnel.ts.net:8765") {
		t.Errorf("unexpected URL: %s", card.URL)
	}
	if !card.Capabilities.Streaming || !card.Capabilities.AsyncTasks || !card.Capabilities.TaskCancellation {
		t.Errorf("expected streaming, async_tasks, and task_cancellation to be true")
	}

	foundSE := false
	for _, skill := range card.Skills {
		if skill.ID == SkillSoftwareEngineering {
			foundSE = true
			if skill.InputSchema == nil {
				t.Errorf("expected software_engineering skill to define input schema")
			}
		}
	}
	if !foundSE {
		t.Errorf("expected SkillSoftwareEngineering in card.Skills")
	}
}

func TestA2ATaskManager_CreateAndCompleteTask(t *testing.T) {
	mock := newMockBackend()
	mgr := NewA2ATaskManager(func() agentBackend { return mock })

	task, err := mgr.CreateTask(SkillSoftwareEngineering, "Implement A2A task orchestrator", "A2A Feature")
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if task.ID == "" {
		t.Fatalf("expected non-empty task ID")
	}
	if task.Prompt != "Implement A2A task orchestrator" {
		t.Errorf("expected prompt preserved, got: %s", task.Prompt)
	}

	// Wait for task to finish in background
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	completed, err := mgr.AwaitTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("AwaitTask failed: %v", err)
	}
	if completed.Status != TaskStatusCompleted {
		t.Errorf("expected task status completed, got %s", completed.Status)
	}
	if completed.Result == nil || completed.Result.Text == "" {
		t.Fatalf("expected completed task to have result text")
	}
	if len(completed.Result.Artifacts) != 2 {
		t.Errorf("expected 2 artifacts, got %d", len(completed.Result.Artifacts))
	}
}

func TestA2ATaskManager_ValidationAndErrors(t *testing.T) {
	mock := newMockBackend()
	mgr := NewA2ATaskManager(func() agentBackend { return mock })

	_, err := mgr.CreateTask("", "", "")
	if err == nil {
		t.Errorf("expected error for empty prompt")
	}

	_, err = mgr.GetTask("non-existent-task-id")
	if err == nil {
		t.Errorf("expected error for non-existent task")
	}
}

func TestA2ATaskManager_CancelTask(t *testing.T) {
	mock := newMockBackend()
	mock.delay = 1 * time.Second
	mgr := NewA2ATaskManager(func() agentBackend { return mock })

	task, err := mgr.CreateTask(SkillSoftwareEngineering, "Slow task that will be canceled", "Slow Task")
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	canceled, err := mgr.CancelTask(task.ID)
	if err != nil {
		t.Fatalf("CancelTask failed: %v", err)
	}
	if canceled.Status != TaskStatusCanceled {
		t.Errorf("expected status canceled, got: %s", canceled.Status)
	}

	mock.mu.Lock()
	stoppedCount := len(mock.stopped)
	mock.mu.Unlock()
	if stoppedCount == 0 {
		t.Errorf("expected backend.StopConversation to be called upon cancel")
	}
}

func TestA2ATaskManager_EventStreaming(t *testing.T) {
	mock := newMockBackend()
	mgr := NewA2ATaskManager(func() agentBackend { return mock })

	events, unsubscribe := mgr.SubscribeAllEvents()
	defer unsubscribe()

	task, err := mgr.CreateTask(SkillSoftwareEngineering, "Stream test", "Streaming")
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	var receivedTypes []string
	timeout := time.After(2 * time.Second)

loop:
	for {
		select {
		case ev := <-events:
			if ev.TaskID == task.ID {
				receivedTypes = append(receivedTypes, ev.Type)
				if ev.Type == "task.completed" {
					break loop
				}
			}
		case <-timeout:
			t.Fatalf("timed out waiting for events, received: %v", receivedTypes)
		}
	}

	if len(receivedTypes) == 0 {
		t.Fatalf("expected events for task %s", task.ID)
	}
}
