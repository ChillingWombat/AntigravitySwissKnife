package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	// Issue #120: no loopback fallback endpoint — a fresh manager is simply "not exposed".
	if st.GatewayURL != "" {
		t.Errorf("expected empty gateway URL before a tunnel starts, got %s", st.GatewayURL)
	}
	if st.Active {
		t.Errorf("expected inactive tunnel before start")
	}
	if st.Type != "" {
		t.Errorf("expected empty tunnel type before start, got %s", st.Type)
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
	if stRemote.GatewayURL != "" {
		t.Errorf("expected empty remote gateway URL before a tunnel starts, got %s", stRemote.GatewayURL)
	}

	// 3. Custom URL appending endpointPath
	customSt, err := tmRemote.Start("custom", "https://subdomain.ngrok-free.app")
	if err != nil {
		t.Fatalf("custom tunnel start failed: %v", err)
	}
	if customSt.GatewayURL != "https://subdomain.ngrok-free.app/remote" {
		t.Errorf("expected custom URL to append /remote, got %s", customSt.GatewayURL)
	}

	// 4. Issue #120: loopback is not a remote endpoint — starting it must fail and leave the
	//     active custom tunnel untouched.
	loopSt, err := tmRemote.Start("loopback", "")
	if err == nil {
		t.Fatalf("expected loopback start to be rejected")
	}
	if loopSt.Type != "custom" || !loopSt.Active {
		t.Errorf("rejected loopback request must not tear down the active tunnel, got type=%q active=%v", loopSt.Type, loopSt.Active)
	}

	// 5. Stop clears the endpoint entirely (no loopback fallback).
	tmRemote.Stop()
	stoppedSt := tmRemote.GetStatus()
	if stoppedSt.GatewayURL != "" || stoppedSt.Type != "" || stoppedSt.Active {
		t.Errorf("expected cleared tunnel status after stop, got gateway=%q type=%q active=%v", stoppedSt.GatewayURL, stoppedSt.Type, stoppedSt.Active)
	}
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
				_, _ = tm.Start("custom", "https://concurrency.example.com")
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
	// Issue #120: no loopback endpoint before a tunnel starts.
	if st.Tunnel.GatewayURL != "" {
		t.Errorf("expected empty gateway URL before a tunnel starts, got %s", st.Tunnel.GatewayURL)
	}
	if st.EndpointURL != "" {
		t.Errorf("expected no remote endpoint URL before a tunnel starts, got %s", st.EndpointURL)
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

	// Stop tunnel clears the endpoint entirely (no loopback fallback)
	rm.StopTunnel()
	stStopped := rm.GetStatus("david@example.com", 1234)
	if stStopped.EndpointURL != "" {
		t.Errorf("expected EndpointURL cleared after stop, got %s", stStopped.EndpointURL)
	}
	if stStopped.Tunnel.GatewayURL != "" {
		t.Errorf("expected tunnel gateway URL cleared after stop, got %s", stStopped.Tunnel.GatewayURL)
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

// Issue #120: pure-function tests for external-endpoint selection and loopback detection.
func TestExternalEndpointSelection(t *testing.T) {
	cases := []struct {
		name string
		st   TunnelStatus
		want string
	}{
		{"inactive tunnel never exposes", TunnelStatus{Active: false, GatewayURL: "https://a.example.com/x"}, ""},
		{"empty gateway", TunnelStatus{Active: true, GatewayURL: ""}, ""},
		{"active https gateway", TunnelStatus{Active: true, GatewayURL: "https://abc.trycloudflare.com/api/bridge/mcp"}, "https://abc.trycloudflare.com/api/bridge/mcp"},
		{"loopback ipv4 rejected", TunnelStatus{Active: true, GatewayURL: "http://127.0.0.1:8765/api/bridge/mcp"}, ""},
		{"loopback localhost rejected", TunnelStatus{Active: true, GatewayURL: "http://localhost:8765/remote"}, ""},
		{"loopback ipv6 rejected", TunnelStatus{Active: true, GatewayURL: "http://[::1]:8765/remote"}, ""},
		{"whitespace trimmed", TunnelStatus{Active: true, GatewayURL: "  https://x.example.com/a  "}, "https://x.example.com/a"},
	}
	for _, tc := range cases {
		if got := externalEndpoint(tc.st); got != tc.want {
			t.Errorf("%s: externalEndpoint() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestIsLoopbackURL(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{"http://127.0.0.1:8765/api/bridge/mcp", true},
		{"http://127.8.8.8/remote", true},
		{"https://localhost:8766/remote", true},
		{"http://[::1]:8765/remote", true},
		{"https://abc.trycloudflare.com", false},
		{"https://mymachine.tailnet.ts.net:8765/remote", false},
		{"not-a-url", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isLoopbackURL(tc.raw); got != tc.want {
			t.Errorf("isLoopbackURL(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

func TestTunnelManager_RejectsLoopbackAndInsecureCustom(t *testing.T) {
	tm := NewTunnelManager(8765, "/api/bridge/mcp")

	// Issue #120: loopback is not a remote endpoint.
	if _, err := tm.Start("loopback", ""); err == nil {
		t.Errorf("expected loopback start to be rejected")
	}
	// External custom endpoints must be HTTPS.
	if _, err := tm.Start("custom", "http://insecure.example.com"); err == nil {
		t.Errorf("expected http custom endpoint to be rejected")
	}
	if _, err := tm.Start("custom", ""); err == nil {
		t.Errorf("expected empty custom URL to be rejected")
	}
	if _, err := tm.Start("bogus", ""); err == nil {
		t.Errorf("expected unknown tunnel type to be rejected")
	}

	// Rejections must leave the (inactive) status untouched.
	st := tm.GetStatus()
	if st.Active || st.GatewayURL != "" || st.Type != "" {
		t.Errorf("rejected starts must leave the tunnel status untouched, got active=%v gateway=%q type=%q", st.Active, st.GatewayURL, st.Type)
	}
}
