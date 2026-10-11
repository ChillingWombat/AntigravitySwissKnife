package bridge

import (
	"encoding/json"
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
	if st.TunnelType != "loopback" {
		t.Errorf("expected default loopback tunnel type")
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

	// 3. Test MCP tools/list
	listReq := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	listRespBytes, err := svc.HandleMCPRequest(listReq)
	if err != nil {
		t.Fatalf("tools/list failed: %v", err)
	}
	var listResp map[string]interface{}
	_ = json.Unmarshal(listRespBytes, &listResp)
	toolsList := listResp["result"].(map[string]interface{})["tools"].([]interface{})
	if len(toolsList) != 6 {
		t.Errorf("expected 6 tools, got %d", len(toolsList))
	}

	// 4. Test MCP tools/call for system info
	callReq := []byte(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"antigravity_system_info","arguments":{}}}`)
	callRespBytes, err := svc.HandleMCPRequest(callReq)
	if err != nil {
		t.Fatalf("tools/call failed: %v", err)
	}
	var callResp map[string]interface{}
	_ = json.Unmarshal(callRespBytes, &callResp)
	callResult := callResp["result"].(map[string]interface{})
	contentArr := callResult["content"].([]interface{})
	firstContent := contentArr[0].(map[string]interface{})
	if firstContent["type"] != "text" {
		t.Errorf("expected type text")
	}

	// 5. Test UpdateConfig persistence
	cfg := svc.config
	cfg.TunnelType = "cloudflare"
	cfg.TunnelURL = "https://gemini-spark.trycloudflare.com/mcp"
	if err := svc.UpdateConfig(cfg); err != nil {
		t.Fatalf("update config failed: %v", err)
	}
	stUpdated := svc.GetStatus()
	if stUpdated.TunnelType != "cloudflare" {
		t.Errorf("expected updated tunnel type cloudflare")
	}
	if stUpdated.GatewayURL != "https://gemini-spark.trycloudflare.com/mcp" {
		t.Errorf("expected updated gateway url")
	}
	if _, statErr := os.Stat(cfgPath); statErr != nil {
		t.Errorf("config file was not persisted: %v", statErr)
	}
}
