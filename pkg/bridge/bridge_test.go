package bridge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestBridge_LifecycleAndMCP(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gemini_bridge.json")

	svc, err := NewService(cfgPath)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// 1. Initial status
	st := svc.GetStatus()
	if !st.Enabled {
		t.Errorf("expected bridge enabled by default")
	}
	if st.OperatingMode != "hybrid" {
		t.Errorf("expected default operating mode hybrid, got %s", st.OperatingMode)
	}
	if st.AuthToken == "" {
		t.Errorf("expected non-empty auth token")
	}
	if st.PairingPIN == "" {
		t.Errorf("expected non-empty pairing PIN")
	}

	// 2. Test MCP initialize
	initReq := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	initRespBytes, err := svc.HandleMCPRequest(initReq)
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}
	var initResp map[string]interface{}
	if err := json.Unmarshal(initRespBytes, &initResp); err != nil {
		t.Fatalf("unmarshal init failed: %v", err)
	}
	resMap := initResp["result"].(map[string]interface{})
	serverInfo := resMap["serverInfo"].(map[string]interface{})
	if serverInfo["name"] != "antigravity-gemini-bridge" {
		t.Errorf("unexpected serverInfo name: %v", serverInfo["name"])
	}

	// 3. Test MCP tools/list in hybrid mode
	listReq := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	listRespBytes, err := svc.HandleMCPRequest(listReq)
	if err != nil {
		t.Fatalf("tools/list failed: %v", err)
	}
	var listResp map[string]interface{}
	_ = json.Unmarshal(listRespBytes, &listResp)
	toolsList := listResp["result"].(map[string]interface{})["tools"].([]interface{})
	if len(toolsList) < 10 {
		t.Errorf("expected at least 10 tools in hybrid mode, got %d", len(toolsList))
	}

	// 4. Test mode switching
	if err := svc.SetOperatingMode("lead"); err != nil {
		t.Fatalf("set operating mode lead failed: %v", err)
	}
	leadTools := svc.ToolManifest()
	hasLeadTool := false
	hasDirectTool := false
	for _, tl := range leadTools {
		name := tl["name"].(string)
		if name == "antigravity_lead_create_conversation" {
			hasLeadTool = true
		}
		if name == "local_run_command" {
			hasDirectTool = true
		}
	}
	if !hasLeadTool {
		t.Errorf("expected lead tools in lead mode")
	}
	if hasDirectTool {
		t.Errorf("did not expect local_run_command in lead-only mode")
	}

	// Switch back to hybrid
	_ = svc.SetOperatingMode("hybrid")

	// 5. Test Direct Mode tools: write and read file
	testFile := filepath.Join(tmpDir, "test_remote.txt")
	writeRes, err := svc.CallTool("local_write_file", map[string]interface{}{
		"path":    testFile,
		"content": "Hello from Gemini Remote Actuator",
	})
	if err != nil {
		t.Fatalf("local_write_file failed: %v", err)
	}
	if writeRes == "" {
		t.Errorf("expected non-empty write result")
	}

	readRes, err := svc.CallTool("local_read_file", map[string]interface{}{
		"path": testFile,
	})
	if err != nil {
		t.Fatalf("local_read_file failed: %v", err)
	}
	if readRes != "Hello from Gemini Remote Actuator" {
		t.Errorf("unexpected read content: %s", readRes)
	}

	// 6. Test direct run command
	cmdRes, err := svc.CallTool("local_run_command", map[string]interface{}{
		"command": "echo 'swiss_remote_ok'",
	})
	if err != nil {
		t.Fatalf("local_run_command failed: %v", err)
	}
	if len(cmdRes) == 0 {
		t.Errorf("expected non-empty command output")
	}

	// 7. Test Auth validation
	reqBearer := httptest.NewRequest(http.MethodPost, "/api/bridge/mcp", nil)
	reqBearer.Header.Set("Authorization", "Bearer "+st.AuthToken)
	granted, _ := svc.ValidateRequest(reqBearer)
	if !granted {
		t.Errorf("expected Bearer token authentication granted")
	}

	reqPIN := httptest.NewRequest(http.MethodPost, "/api/bridge/mcp?pin="+st.PairingPIN, nil)
	grantedPIN, _ := svc.ValidateRequest(reqPIN)
	if !grantedPIN {
		t.Errorf("expected pairing PIN authentication granted")
	}

	reqBad := httptest.NewRequest(http.MethodPost, "/api/bridge/mcp", nil)
	reqBad.Header.Set("Authorization", "Bearer wrong-token")
	grantedBad, _ := svc.ValidateRequest(reqBad)
	if grantedBad {
		t.Errorf("expected invalid token to be rejected")
	}

	// 8. Test Rotate Credentials
	oldToken := st.AuthToken
	newToken, newPIN, err := svc.RotateCredentials()
	if err != nil {
		t.Fatalf("rotate credentials failed: %v", err)
	}
	if newToken == oldToken {
		t.Errorf("expected new token after rotation")
	}
	if newPIN == "" {
		t.Errorf("expected valid new PIN")
	}

	// 9. Test TunnelManager loopback / custom
	stCustom, err := svc.StartTunnel("custom", "https://custom-gateway.local")
	if err != nil {
		t.Fatalf("start custom tunnel failed: %v", err)
	}
	if stCustom.Type != "custom" {
		t.Errorf("expected custom tunnel type, got %s", stCustom.Type)
	}
	svc.StopTunnel()
	stAfterStop := svc.GetStatus()
	if stAfterStop.TunnelType != "loopback" {
		t.Errorf("expected loopback after stop, got %s", stAfterStop.TunnelType)
	}
}

func TestBridge_LeadAndMessenger(t *testing.T) {
	mockBrainDir := t.TempDir()
	convID := "test-conv-lead-123"
	logsDir := filepath.Join(mockBrainDir, convID, ".system_generated", "logs")
	_ = os.MkdirAll(logsDir, 0755)

	transcriptContent := `{"step_index":1,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","content":"Refactor authentication"}
{"step_index":2,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","content":"I have refactored auth cleanly with Bearer token validation and 100% tests passing."}
`
	_ = os.WriteFile(filepath.Join(logsDir, "transcript.jsonl"), []byte(transcriptContent), 0644)
	_ = os.WriteFile(filepath.Join(mockBrainDir, convID, "report.md"), []byte("# Security Report"), 0644)

	lh := NewLeadHandler(mockBrainDir)

	// Test ListConversations
	listJSON, err := lh.ListConversations(map[string]interface{}{"limit": float64(5)})
	if err != nil {
		t.Fatalf("ListConversations failed: %v", err)
	}
	var convs []map[string]interface{}
	if err := json.Unmarshal([]byte(listJSON), &convs); err != nil || len(convs) != 1 {
		t.Fatalf("expected 1 conversation, got %d (err: %v)", len(convs), err)
	}
	if convs[0]["id"] != convID {
		t.Errorf("unexpected conversation id: %v", convs[0]["id"])
	}

	// Test GetStatus
	statusJSON, err := lh.GetStatus(map[string]interface{}{"conversation_id": convID})
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	var stMap map[string]interface{}
	_ = json.Unmarshal([]byte(statusJSON), &stMap)
	if stMap["status"] != "completed" {
		t.Errorf("expected status completed, got %v", stMap["status"])
	}

	// Test AwaitResponse
	respJSON, err := lh.AwaitResponse(map[string]interface{}{
		"conversation_id": convID,
		"timeout_seconds": float64(2),
	})
	if err != nil {
		t.Fatalf("AwaitResponse failed: %v", err)
	}
	var respMap map[string]interface{}
	_ = json.Unmarshal([]byte(respJSON), &respMap)
	if respMap["status"] != "COMPLETED" {
		t.Errorf("expected status COMPLETED, got %v", respMap["status"])
	}
	if respMap["response"] != "I have refactored auth cleanly with Bearer token validation and 100% tests passing." {
		t.Errorf("unexpected forwarded response: %v", respMap["response"])
	}
	artifactsArr := respMap["artifacts"].([]interface{})
	if len(artifactsArr) != 1 || artifactsArr[0] != "report.md" {
		t.Errorf("expected artifact report.md, got %v", artifactsArr)
	}
}

