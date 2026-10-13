package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestTunnelManager_InitializationAndPaths(t *testing.T) {
	// 1. Default Gemini Bridge tunnel manager
	tmBridge := NewTunnelManager(8765, "/api/bridge/mcp")
	if tmBridge.localPort != 8765 {
		t.Errorf("expected port 8765, got %d", tmBridge.localPort)
	}
	if tmBridge.endpointPath != "/api/bridge/mcp" {
		t.Errorf("expected /api/bridge/mcp, got %s", tmBridge.endpointPath)
	}
	st := tmBridge.GetStatus()
	if st.GatewayURL != "http://127.0.0.1:8765/api/bridge/mcp" {
		t.Errorf("unexpected bridge gateway URL: %s", st.GatewayURL)
	}

	// 2. Remote Control tunnel manager
	tmRemote := NewTunnelManager(8766, "/remote")
	if tmRemote.localPort != 8766 {
		t.Errorf("expected port 8766, got %d", tmRemote.localPort)
	}
	if tmRemote.endpointPath != "/remote" {
		t.Errorf("expected /remote, got %s", tmRemote.endpointPath)
	}
	stRemote := tmRemote.GetStatus()
	if stRemote.GatewayURL != "http://127.0.0.1:8766/remote" {
		t.Errorf("unexpected remote gateway URL: %s", stRemote.GatewayURL)
	}

	// 3. Custom URL appending endpointPath
	customSt, err := tmRemote.Start("custom", "https://subdomain.ngrok-free.app")
	if err != nil {
		t.Fatalf("custom tunnel start failed: %v", err)
	}
	if customSt.GatewayURL != "https://subdomain.ngrok-free.app/remote" {
		t.Errorf("expected custom URL to append /remote, got %s", customSt.GatewayURL)
	}

	// 4. Return to loopback
	loopSt, err := tmRemote.Start("loopback", "")
	if err != nil {
		t.Fatalf("loopback start failed: %v", err)
	}
	if loopSt.GatewayURL != "http://127.0.0.1:8766/remote" {
		t.Errorf("expected loopback gateway URL, got %s", loopSt.GatewayURL)
	}
	tmRemote.Stop()
}

func TestTunnelManager_ConcurrencyAndNoDeadlock(t *testing.T) {
	tm := NewTunnelManager(8765, "/api/bridge/mcp")

	var wg sync.WaitGroup
	// Run concurrent appendLog, GetStatus, and Start/Stop operations
	for i := 0; i < 20; i++ {
		wg.Add(3)
		go func(idx int) {
			defer wg.Done()
			tm.appendLog("test line from worker")
		}(i)
		go func() {
			defer wg.Done()
			_ = tm.GetStatus()
		}()
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				_, _ = tm.Start("loopback", "")
			} else {
				tm.Stop()
			}
		}(i)
	}

	wg.Wait()
	finalSt := tm.GetStatus()
	if len(finalSt.Logs) == 0 {
		t.Errorf("expected logs to be populated safely")
	}
}

func TestRemoteManager_TunnelIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "remote_control.json")

	rm := NewRemoteManager(cfgPath)
	st := rm.GetStatus("david@example.com", 1234)
	if st.Tunnel.GatewayURL != "http://127.0.0.1:8766/remote" {
		t.Errorf("expected initial loopback gateway URL, got %s", st.Tunnel.GatewayURL)
	}
	if !strings.Contains(st.EndpointURL, ":8766/remote") {
		t.Errorf("expected endpoint URL on port 8766, got %s", st.EndpointURL)
	}

	// Start custom tunnel
	tunSt, err := rm.StartTunnel("custom", "https://remote.example.com")
	if err != nil {
		t.Fatalf("start remote tunnel custom failed: %v", err)
	}
	if tunSt.GatewayURL != "https://remote.example.com/remote" {
		t.Errorf("expected https://remote.example.com/remote, got %s", tunSt.GatewayURL)
	}

	// Verify GetStatus reflects active custom tunnel gateway
	stAfter := rm.GetStatus("david@example.com", 1234)
	if stAfter.EndpointURL != "https://remote.example.com/remote" {
		t.Errorf("expected EndpointURL to reflect active tunnel GatewayURL, got %s", stAfter.EndpointURL)
	}

	// Stop tunnel returns to loopback
	rm.StopTunnel()
	stStopped := rm.GetStatus("david@example.com", 1234)
	if !strings.Contains(stStopped.EndpointURL, ":8766/remote") {
		t.Errorf("expected EndpointURL returned to local loopback, got %s", stStopped.EndpointURL)
	}
}

func TestBridge_ConfigAliasAndDefaultDisabled(t *testing.T) {
	// 1. Verify default configurations have Enabled == false
	bridgeDef := DefaultConfig()
	if bridgeDef.Enabled {
		t.Errorf("DefaultConfig must have Enabled: false")
	}
	remoteDef := DefaultRemoteConfig()
	if remoteDef.Enabled {
		t.Errorf("DefaultRemoteConfig must have Enabled: false")
	}

	// 2. Test bridge.json alias loading
	tmpDir := t.TempDir()
	aliasPath := filepath.Join(tmpDir, "bridge.json")
	customCfg := Config{
		Enabled:       false,
		Port:          9123,
		OperatingMode: "lead",
		AllowedTools:  []string{"test_tool"},
	}
	data, _ := json.MarshalIndent(customCfg, "", "  ")
	if err := os.WriteFile(aliasPath, data, 0644); err != nil {
		t.Fatalf("failed to write bridge.json alias: %v", err)
	}

	// Pass the non-existent gemini_bridge.json path in the same directory
	geminiPath := filepath.Join(tmpDir, "gemini_bridge.json")
	svc, err := NewService(geminiPath)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	st := svc.GetStatus()
	if st.Enabled {
		t.Errorf("expected bridge to be disabled by default")
	}
	cfg := svc.GetConfig()
	if cfg.Port != 9123 {
		t.Errorf("expected port 9123 loaded from bridge.json alias, got %d", cfg.Port)
	}
	if st.OperatingMode != "lead" {
		t.Errorf("expected operating mode 'lead' loaded from bridge.json alias, got %s", st.OperatingMode)
	}
}
