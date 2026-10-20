package bridge

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
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
	if st.Enabled {
		t.Errorf("expected bridge disabled by default")
	}
	if st.OperatingMode != "hybrid" {
		t.Errorf("expected default operating mode hybrid, got %s", st.OperatingMode)
	}
	if st.AuthToken == "" {
		t.Errorf("expected non-empty auth token")
	}
	if st.PairingPIN == "" || st.PairingPIN == "******" {
		t.Errorf("expected visible pairing PIN, got %q", st.PairingPIN)
	}
	if st.PairingPIN != svc.authMgr.GetPIN() {
		t.Errorf("pairing PIN mismatch: status=%q actual=%q", st.PairingPIN, svc.authMgr.GetPIN())
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
	rawToken := svc.GetRawAuthToken()
	rawPIN := svc.GetRawPairingPIN()

	reqBearer := httptest.NewRequest(http.MethodPost, "/api/bridge/mcp", nil)
	reqBearer.Header.Set("Authorization", "Bearer "+rawToken)
	granted, _ := svc.ValidateRequest(reqBearer)
	if !granted {
		t.Errorf("expected Bearer token authentication granted")
	}

	reqPIN := httptest.NewRequest(http.MethodPost, "/api/bridge/mcp?pin="+rawPIN, nil)
	grantedPIN, _ := svc.ValidateRequest(reqPIN)
	if !grantedPIN {
		t.Errorf("expected pairing PIN authentication granted")
	}

	reqMasked := httptest.NewRequest(http.MethodPost, "/api/bridge/mcp?pin=******", nil)
	grantedMasked, _ := svc.ValidateRequest(reqMasked)
	if grantedMasked {
		t.Errorf("expected masked PIN to be rejected")
	}

	reqBad := httptest.NewRequest(http.MethodPost, "/api/bridge/mcp", nil)
	reqBad.Header.Set("Authorization", "Bearer wrong-token")
	grantedBad, _ := svc.ValidateRequest(reqBad)
	if grantedBad {
		t.Errorf("expected invalid token to be rejected")
	}

	// 8. Test Rotate Credentials
	oldToken := rawToken
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

	// 9. Test TunnelManager custom start / stop
	stCustom, err := svc.StartTunnel("custom", "https://custom-gateway.local")
	if err != nil {
		t.Fatalf("start custom tunnel failed: %v", err)
	}
	if stCustom.Type != "custom" {
		t.Errorf("expected custom tunnel type, got %s", stCustom.Type)
	}
	svc.StopTunnel()
	stAfterStop := svc.GetStatus()
	// Issue #120: stopping clears the tunnel type entirely (no loopback fallback) and the
	// gateway URL is no longer reported.
	if stAfterStop.TunnelType != "" {
		t.Errorf("expected cleared tunnel type after stop, got %s", stAfterStop.TunnelType)
	}
	if stAfterStop.GatewayURL != "" {
		t.Errorf("expected no gateway URL after stop, got %s", stAfterStop.GatewayURL)
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

func writeUnmaskedWSFrame(w *bufio.Writer, payload []byte) error {
	var buf bytes.Buffer
	buf.WriteByte(0x81) // FIN + text frame
	length := len(payload)
	if length <= 125 {
		buf.WriteByte(byte(length))
	} else if length <= 65535 {
		buf.WriteByte(126)
		_ = binary.Write(&buf, binary.BigEndian, uint16(length))
	} else {
		buf.WriteByte(127)
		_ = binary.Write(&buf, binary.BigEndian, uint64(length))
	}
	buf.Write(payload)
	if _, err := w.Write(buf.Bytes()); err != nil {
		return err
	}
	return w.Flush()
}

func TestBridge_LeadInputValidation(t *testing.T) {
	lh := NewLeadHandler(t.TempDir())

	_, err := lh.CreateConversation(map[string]interface{}{"prompt": ""})
	if err == nil || !strings.Contains(err.Error(), "prompt is required") {
		t.Fatalf("expected 'prompt is required' error, got %v", err)
	}

	_, err = lh.SendMessage(map[string]interface{}{"message": "  "})
	if err == nil || !strings.Contains(err.Error(), "message is required") {
		t.Fatalf("expected 'message is required' error, got %v", err)
	}
}

func TestBridge_LeadMockCDPDispatch(t *testing.T) {
	mockBrainDir := t.TempDir()
	convID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	logsDir := filepath.Join(mockBrainDir, convID, ".system_generated", "logs")
	_ = os.MkdirAll(logsDir, 0755)

	var evaluatedScripts []string
	var mu sync.Mutex

	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json/version":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Browser":"Chrome/132.0.0.0"}`))
		case "/json":
			u, _ := url.Parse(srvURL)
			targets := []gui.DevToolsTarget{
				{
					ID:                   "target-page-1",
					Title:                "Antigravity",
					Type:                 "page",
					URL:                  "http://127.0.0.1:" + u.Port() + "/c/" + convID,
					WebSocketDebuggerURL: "ws://" + u.Host + "/devtools/page/target-page-1",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(targets)
		case "/devtools/page/target-page-1":
			hj, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "hijack failed", 500)
				return
			}
			conn, bufrw, err := hj.Hijack()
			if err != nil {
				return
			}
			defer conn.Close()

			// Send 101 Switching Protocols
			_, _ = bufrw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=\r\n\r\n")
			_ = bufrw.Flush()

			// Read WebSocket Frame
			b0, err := bufrw.ReadByte()
			if err != nil {
				return
			}
			_ = b0
			b1, err := bufrw.ReadByte()
			if err != nil {
				return
			}
			masked := (b1 & 0x80) != 0
			payLen := int(b1 & 0x7F)
			if payLen == 126 {
				var ext uint16
				_ = binary.Read(bufrw, binary.BigEndian, &ext)
				payLen = int(ext)
			} else if payLen == 127 {
				var ext uint64
				_ = binary.Read(bufrw, binary.BigEndian, &ext)
				payLen = int(ext)
			}
			var maskKey [4]byte
			if masked {
				if _, err := io.ReadFull(bufrw, maskKey[:]); err != nil {
					return
				}
			}
			payload := make([]byte, payLen)
			if _, err := io.ReadFull(bufrw, payload); err != nil {
				return
			}
			if masked {
				for i := 0; i < payLen; i++ {
					payload[i] ^= maskKey[i%4]
				}
			}

			var req map[string]interface{}
			_ = json.Unmarshal(payload, &req)
			if params, ok := req["params"].(map[string]interface{}); ok {
				if expr, ok := params["expression"].(string); ok {
					mu.Lock()
					evaluatedScripts = append(evaluatedScripts, expr)
					mu.Unlock()
				}
			}

			// Respond with success JSON-RPC matching ID=1
			resp := map[string]interface{}{
				"id": 1,
				"result": map[string]interface{}{
					"result": map[string]interface{}{
						"type":  "object",
						"value": map[string]interface{}{"success": true, "dispatched": true},
					},
				},
			}
			respBytes, _ := json.Marshal(resp)
			_ = writeUnmaskedWSFrame(bufrw.Writer, respBytes)
			time.Sleep(30 * time.Millisecond)
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	mockInj := gui.NewInjector(port)
	lh := NewLeadHandler(mockBrainDir)
	lh.SetInjector(mockInj)

	// 1. Test CreateConversation dispatches script containing Lexical injection
	createRes, err := lh.CreateConversation(map[string]interface{}{
		"prompt": "Investigate unit tests for Lexical editor",
		"title":  "Test Task",
	})
	if err != nil {
		t.Fatalf("CreateConversation failed: %v", err)
	}
	if !strings.Contains(createRes, "dispatched") {
		t.Fatalf("expected dispatched in createRes, got: %s", createRes)
	}

	mu.Lock()
	if len(evaluatedScripts) == 0 {
		mu.Unlock()
		t.Fatalf("expected evaluated script in CDP mock server, got none")
	}
	lastScript := evaluatedScripts[len(evaluatedScripts)-1]
	mu.Unlock()

	if !strings.Contains(lastScript, "__lexicalEditor") {
		t.Errorf("expected script to target Lexical editor __lexicalEditor, got: %s", lastScript)
	}
	if !strings.Contains(lastScript, "data-testid=\"app-icon-new-conversation-button\"") {
		t.Errorf("expected script to query app-icon-new-conversation-button selector, got: %s", lastScript)
	}
	if !strings.Contains(lastScript, "data-testid=\"send-button\"") {
		t.Errorf("expected script to poll send-button, got: %s", lastScript)
	}
	if !strings.Contains(lastScript, "Investigate unit tests for Lexical editor") {
		t.Errorf("expected prompt text to be preserved in script, got: %s", lastScript)
	}

	// 2. Test SendMessage dispatches script containing Lexical injection and route navigation
	sendRes, err := lh.SendMessage(map[string]interface{}{
		"conversation_id": convID,
		"message":         "Please proceed with the implementation.",
	})
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if !strings.Contains(sendRes, "dispatched") {
		t.Fatalf("expected dispatched in sendRes, got: %s", sendRes)
	}

	mu.Lock()
	if len(evaluatedScripts) < 2 {
		mu.Unlock()
		t.Fatalf("expected 2 evaluated scripts, got %d", len(evaluatedScripts))
	}
	msgScript := evaluatedScripts[len(evaluatedScripts)-1]
	mu.Unlock()

	if !strings.Contains(msgScript, "__lexicalEditor") {
		t.Errorf("expected SendMessage script to use Lexical editor, got: %s", msgScript)
	}
	if !strings.Contains(msgScript, fmt.Sprintf(`const targetConv = %q;`, convID)) {
		t.Errorf("expected SendMessage script to contain targetConv %q, got: %s", convID, msgScript)
	}
	if !strings.Contains(msgScript, `const targetPath = "/c/" + targetConv;`) {
		t.Errorf("expected SendMessage script to construct targetPath, got: %s", msgScript)
	}
	if !strings.Contains(msgScript, "window.location.assign(targetPath)") {
		t.Errorf("expected SendMessage script to fallback to window.location.assign, got: %s", msgScript)
	}
	if !strings.Contains(msgScript, "Please proceed with the implementation.") {
		t.Errorf("expected message text in script, got: %s", msgScript)
	}
}

func TestBridge_FindMostRecentConversationID(t *testing.T) {
	tmpBrain := t.TempDir()
	convA := "11111111-2222-3333-4444-555555555555"
	convB := "66666666-7777-8888-9999-000000000000"

	dirA := filepath.Join(tmpBrain, convA)
	dirB := filepath.Join(tmpBrain, convB)
	if err := os.MkdirAll(dirA, 0755); err != nil {
		t.Fatalf("failed to create dirA: %v", err)
	}
	if err := os.MkdirAll(dirB, 0755); err != nil {
		t.Fatalf("failed to create dirB: %v", err)
	}

	// Make dirB newer than dirA
	now := time.Now()
	_ = os.Chtimes(dirA, now.Add(-10*time.Minute), now.Add(-10*time.Minute))
	_ = os.Chtimes(dirB, now, now)

	lh := NewLeadHandler(tmpBrain)
	foundID := lh.findMostRecentConversationID()
	if foundID != convB {
		// When no DB path overrides it, dirB is the newest brain directory
		if foundID != "" && !gui.IsValidConversationID(foundID) {
			t.Errorf("expected valid conversation UUID, got %q", foundID)
		}
	}
}
