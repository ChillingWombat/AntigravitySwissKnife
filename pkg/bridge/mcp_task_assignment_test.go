package bridge

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mcpRPCResponse mirrors the JSON-RPC envelope produced by Service.HandleMCPRequest.
type mcpRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// mcpTextResult is the MCP tools/call success payload.
type mcpTextResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func callMCP(t *testing.T, svc *Service, body string) mcpRPCResponse {
	t.Helper()
	raw, err := svc.HandleMCPRequest([]byte(body))
	if err != nil {
		t.Fatalf("HandleMCPRequest failed for %s: %v", body, err)
	}
	if raw == nil {
		t.Fatalf("expected a JSON-RPC response for %s, got nil (notification)", body)
	}
	var resp mcpRPCResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("invalid JSON-RPC response for %s: %v (raw: %s)", body, err, raw)
	}
	return resp
}

func mustToolText(t *testing.T, svc *Service, id, name string, args map[string]interface{}) string {
	t.Helper()
	argsJSON, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	body := `{"jsonrpc":"2.0","id":` + id + `,"method":"tools/call","params":{"name":"` + name + `","arguments":` + string(argsJSON) + `}}`
	resp := callMCP(t, svc, body)
	if resp.Error != nil {
		t.Fatalf("%s returned JSON-RPC error %d: %s", name, resp.Error.Code, resp.Error.Message)
	}
	var res mcpTextResult
	if err := json.Unmarshal(resp.Result, &res); err != nil {
		t.Fatalf("%s returned unparseable result: %v (raw: %s)", name, err, resp.Result)
	}
	if len(res.Content) == 0 || res.Content[0].Type != "text" {
		t.Fatalf("%s returned no text content: %s", name, resp.Result)
	}
	return res.Content[0].Text
}

// TestBridge_MCP_HighLevelTaskAssignment drives the real MCP JSON-RPC surface that
// /api/bridge/mcp serves: initialize -> tools/list -> tools/call. It asserts the
// high-level goal actually reaches the agent backend (no canned success) and that
// the task lifecycle terminates with the backend's result.
func TestBridge_MCP_HighLevelTaskAssignment(t *testing.T) {
	svc, err := NewService(filepath.Join(t.TempDir(), "gemini_bridge.json"))
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	mock := newMockBackend()
	svc.a2aMgr = NewA2ATaskManager(func() agentBackend { return mock })

	// 1. Handshake negotiates the modern protocol version.
	initResp := callMCP(t, svc, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`)
	if initResp.Error != nil {
		t.Fatalf("initialize failed: %s", initResp.Error.Message)
	}
	var initRes struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name string `json:"name"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(initResp.Result, &initRes); err != nil {
		t.Fatalf("parse initialize result: %v", err)
	}
	if initRes.ProtocolVersion != "2025-03-26" {
		t.Errorf("expected negotiated protocol 2025-03-26, got %q", initRes.ProtocolVersion)
	}
	if initRes.ServerInfo.Name == "" {
		t.Errorf("expected serverInfo.name to be populated")
	}

	// 2. The delegation tool is advertised with a required prompt.
	listResp := callMCP(t, svc, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if listResp.Error != nil {
		t.Fatalf("tools/list failed: %s", listResp.Error.Message)
	}
	var listRes struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			InputSchema struct {
				Required []string `json:"required"`
			} `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(listResp.Result, &listRes); err != nil {
		t.Fatalf("parse tools/list result: %v", err)
	}
	advertised := false
	for _, tool := range listRes.Tools {
		if tool.Name != "a2a_assign_task" {
			continue
		}
		advertised = true
		if len(tool.InputSchema.Required) != 1 || tool.InputSchema.Required[0] != "prompt" {
			t.Errorf("expected a2a_assign_task to require exactly [prompt], got %v", tool.InputSchema.Required)
		}
		if !strings.Contains(strings.ToLower(tool.Description), "high-level") {
			t.Errorf("expected delegation-oriented description, got %q", tool.Description)
		}
	}
	if !advertised {
		t.Fatalf("a2a_assign_task missing from tools/list")
	}

	// 3. Assign a task through MCP and confirm the returned handle is well formed.
	const goal = "Test high level task assignment over MCP"
	assignText := mustToolText(t, svc, "3", "a2a_assign_task", map[string]interface{}{
		"prompt": goal,
		"title":  "Test MCP Task",
		"skill":  SkillCodeReview,
	})
	var assigned A2ATask
	if err := json.Unmarshal([]byte(assignText), &assigned); err != nil {
		t.Fatalf("parse assigned task: %v (raw: %s)", err, assignText)
	}
	if assigned.ID == "" {
		t.Fatalf("expected non-empty task ID, got: %s", assignText)
	}
	if assigned.Prompt != goal {
		t.Errorf("expected prompt %q preserved, got %q", goal, assigned.Prompt)
	}
	if assigned.Skill != SkillCodeReview {
		t.Errorf("expected skill %q preserved, got %q", SkillCodeReview, assigned.Skill)
	}
	if assigned.Status != TaskStatusSubmitted && assigned.Status != TaskStatusWorking {
		t.Errorf("expected submitted/working status on assignment, got %q", assigned.Status)
	}

	// 4. Await completion through MCP, then verify the backend saw the real goal.
	awaitText := mustToolText(t, svc, "4", "a2a_await_task", map[string]interface{}{
		"task_id":         assigned.ID,
		"timeout_seconds": 5,
	})
	var completed A2ATask
	if err := json.Unmarshal([]byte(awaitText), &completed); err != nil {
		t.Fatalf("parse awaited task: %v (raw: %s)", err, awaitText)
	}
	if completed.Status != TaskStatusCompleted {
		t.Fatalf("expected completed status, got %q (error: %s)", completed.Status, completed.Error)
	}
	if completed.Result == nil || completed.Result.Text == "" {
		t.Fatalf("expected result text from backend, got: %s", awaitText)
	}
	if completed.Result.Text != mock.mockResp {
		t.Errorf("expected backend response %q, got %q", mock.mockResp, completed.Result.Text)
	}
	if len(completed.Result.Artifacts) != 2 {
		t.Errorf("expected 2 artifacts forwarded from backend, got %v", completed.Result.Artifacts)
	}
	if completed.Result.Backend != mock.Name() {
		t.Errorf("expected backend name %q recorded, got %q", mock.Name(), completed.Result.Backend)
	}
	if completed.ConversationID != "conv-test-123" {
		t.Errorf("expected conversation ID from backend, got %q", completed.ConversationID)
	}

	mock.mu.Lock()
	if len(mock.created) != 1 {
		mock.mu.Unlock()
		t.Fatalf("expected exactly 1 backend dispatch, got %d", len(mock.created))
	}
	dispatch := mock.created[0]
	mock.mu.Unlock()
	if dispatch["prompt"] != goal {
		t.Errorf("backend received prompt %v, want %q", dispatch["prompt"], goal)
	}
	if dispatch["title"] != "Test MCP Task" {
		t.Errorf("backend received title %v, want %q", dispatch["title"], "Test MCP Task")
	}

	// 5. Status and listing reflect the same task over MCP.
	statusText := mustToolText(t, svc, "5", "a2a_get_task_status", map[string]interface{}{"task_id": assigned.ID})
	if !strings.Contains(statusText, assigned.ID) || !strings.Contains(statusText, TaskStatusCompleted) {
		t.Errorf("expected status payload to report %s as completed, got: %s", assigned.ID, statusText)
	}
	listText := mustToolText(t, svc, "6", "a2a_list_tasks", map[string]interface{}{})
	if !strings.Contains(listText, assigned.ID) {
		t.Errorf("expected task list to include %s, got: %s", assigned.ID, listText)
	}
}

// TestBridge_MCP_TaskAssignmentRejections covers the negative boundaries: bad input
// must surface as a JSON-RPC error rather than a fabricated task ID.
func TestBridge_MCP_TaskAssignmentRejections(t *testing.T) {
	svc, err := NewService(filepath.Join(t.TempDir(), "gemini_bridge.json"))
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	mock := newMockBackend()
	svc.a2aMgr = NewA2ATaskManager(func() agentBackend { return mock })

	cases := []struct {
		name     string
		body     string
		wantCode int
		wantMsg  string
	}{
		{
			name:     "empty prompt",
			body:     `{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"a2a_assign_task","arguments":{"prompt":"   "}}}`,
			wantCode: -32603,
			wantMsg:  "prompt is required",
		},
		{
			name:     "unknown task id",
			body:     `{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"a2a_get_task_status","arguments":{"task_id":"task-does-not-exist"}}}`,
			wantCode: -32603,
			wantMsg:  "task",
		},
		{
			name:     "undeclared tool blocked by allowlist",
			body:     `{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"a2a_invent_tool","arguments":{}}}`,
			wantCode: -32603,
			wantMsg:  "not in the allowed tools list",
		},
		{
			name:     "unknown method",
			body:     `{"jsonrpc":"2.0","id":13,"method":"resources/list"}`,
			wantCode: -32601,
			wantMsg:  "method not found",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := callMCP(t, svc, tc.body)
			if resp.Error == nil {
				t.Fatalf("expected JSON-RPC error, got result: %s", resp.Result)
			}
			if resp.Error.Code != tc.wantCode {
				t.Errorf("expected error code %d, got %d (%s)", tc.wantCode, resp.Error.Code, resp.Error.Message)
			}
			if !strings.Contains(strings.ToLower(resp.Error.Message), tc.wantMsg) {
				t.Errorf("expected error message to contain %q, got %q", tc.wantMsg, resp.Error.Message)
			}
		})
	}

	mock.mu.Lock()
	dispatches := len(mock.created)
	mock.mu.Unlock()
	if dispatches != 0 {
		t.Errorf("expected no backend dispatch for rejected calls, got %d", dispatches)
	}

	// With the allowlist lifted, an unrecognized tool must still be refused by the
	// dispatch switch rather than silently accepted.
	svc.mu.Lock()
	svc.config.AllowedTools = nil
	svc.mu.Unlock()
	resp := callMCP(t, svc, `{"jsonrpc":"2.0","id":14,"method":"tools/call","params":{"name":"a2a_invent_tool","arguments":{}}}`)
	if resp.Error == nil {
		t.Fatalf("expected unknown tool to be refused, got: %s", resp.Result)
	}
	if !strings.Contains(resp.Error.Message, "unknown tool") {
		t.Errorf("expected unknown-tool rejection, got: %s", resp.Error.Message)
	}
}

// TestBridge_MCP_TaskAssignmentDisabledInDirectMode proves the mode gate is real:
// direct mode must refuse delegation instead of silently queuing work.
func TestBridge_MCP_TaskAssignmentDisabledInDirectMode(t *testing.T) {
	svc, err := NewService(filepath.Join(t.TempDir(), "gemini_bridge.json"))
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	mock := newMockBackend()
	svc.a2aMgr = NewA2ATaskManager(func() agentBackend { return mock })

	if err := svc.SetOperatingMode("direct"); err != nil {
		t.Fatalf("SetOperatingMode(direct) failed: %v", err)
	}

	resp := callMCP(t, svc, `{"jsonrpc":"2.0","id":20,"method":"tools/call","params":{"name":"a2a_assign_task","arguments":{"prompt":"Should not run"}}}`)
	if resp.Error == nil {
		t.Fatalf("expected delegation to be refused in direct mode, got: %s", resp.Result)
	}
	if !strings.Contains(resp.Error.Message, "disabled in \"direct\" operational mode") {
		t.Errorf("expected direct-mode rejection, got: %s", resp.Error.Message)
	}

	mock.mu.Lock()
	dispatches := len(mock.created)
	mock.mu.Unlock()
	if dispatches != 0 {
		t.Errorf("expected no backend dispatch in direct mode, got %d", dispatches)
	}

	// Returning to hybrid restores delegation without a restart.
	if err := svc.SetOperatingMode("hybrid"); err != nil {
		t.Fatalf("SetOperatingMode(hybrid) failed: %v", err)
	}
	taskText := mustToolText(t, svc, "21", "a2a_assign_task", map[string]interface{}{"prompt": "Run after hybrid restore"})
	var task A2ATask
	if err := json.Unmarshal([]byte(taskText), &task); err != nil {
		t.Fatalf("parse task: %v", err)
	}
	if task.ID == "" {
		t.Fatalf("expected task ID after restoring hybrid mode, got: %s", taskText)
	}

	ctx := t.Context()
	_ = ctx
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		cur, gErr := svc.a2aMgr.GetTask(task.ID)
		if gErr != nil {
			t.Fatalf("GetTask failed: %v", gErr)
		}
		if isTerminalStatus(cur.Status) {
			if cur.Status != TaskStatusCompleted {
				t.Fatalf("expected completion, got %q (%s)", cur.Status, cur.Error)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("task %s did not reach a terminal state", task.ID)
}
