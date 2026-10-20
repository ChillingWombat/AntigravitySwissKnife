package bridge

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
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

// TestCustomEndpointRejectsLoopbackAndHostlessURLs converts the PR #124 review reproducer
// into a regression test: a custom endpoint must carry BOTH a valid https:// scheme and a
// non-empty, non-loopback host. Host-less input such as "https://" used to pass the prefix
// check and was surfaced as a canonical (dead) endpoint; loopback hosts were accepted and
// leaked to the UI through the raw TunnelStatus fields.
func TestCustomEndpointRejectsLoopbackAndHostlessURLs(t *testing.T) {
	rejected := []string{
		"",
		"https://",
		"https:///remote",
		"https://:8766",
		"https://127.0.0.1:8766",
		"https://127.8.8.8:8766/remote",
		"https://localhost:8766/remote",
		"https://[::1]:8766",
		"http://insecure.example.com",
		"ftp://files.example.com",
		"custom.example.com",
		"https:// space.example.com",
		"https://exa mple.com",
	}
	for _, raw := range rejected {
		tm := NewTunnelManager(8766, "/remote")
		if st, err := tm.Start("custom", raw); err == nil {
			t.Errorf("Start(custom, %q) must be rejected, got active=%v gateway=%q", raw, st.Active, st.GatewayURL)
		}
		if st := tm.GetStatus(); st.Active || st.GatewayURL != "" || st.PublicURL != "" {
			t.Errorf("rejected custom URL %q left residue: active=%v gateway=%q public=%q", raw, st.Active, st.GatewayURL, st.PublicURL)
		}
	}

	// A rejected request must never tear down an already working tunnel (PR #124 review
	// reproducer 2); the live tunnel keeps being reported on every path.
	tm := NewTunnelManager(8766, "/remote")
	if _, err := tm.Start("custom", "https://live.example.com"); err != nil {
		t.Fatalf("setup: valid custom tunnel rejected: %v", err)
	}
	for _, raw := range []string{"https://", "https://127.0.0.1:8766", "http://insecure.example.com"} {
		if _, err := tm.Start("custom", raw); err == nil {
			t.Errorf("Start(custom, %q) must be rejected while a live tunnel exists", raw)
		}
		st := tm.GetStatus()
		if !st.Active || st.Type != "custom" || !strings.Contains(st.GatewayURL, "live.example.com") {
			t.Errorf("rejected %q must preserve the live tunnel, got active=%v type=%q gateway=%q", raw, st.Active, st.Type, st.GatewayURL)
		}
	}
	tm.Stop()

	// Well-formed external endpoints are still accepted; surrounding whitespace is tolerated
	// and trimmed before use.
	valid := []struct{ in, want string }{
		{"https://subdomain.ngrok-free.app", "https://subdomain.ngrok-free.app/remote"},
		{"  https://padded.example.com  ", "https://padded.example.com/remote"},
		{"https://custom.example.com/remote", "https://custom.example.com/remote"},
		{"HTTPS://UPPER.EXAMPLE.CO.UK", "HTTPS://UPPER.EXAMPLE.CO.UK/remote"},
	}
	for _, tc := range valid {
		tm := NewTunnelManager(8766, "/remote")
		st, err := tm.Start("custom", tc.in)
		if err != nil {
			t.Errorf("Start(custom, %q) must be accepted: %v", tc.in, err)
			continue
		}
		if st.GatewayURL != tc.want {
			t.Errorf("Start(custom, %q).GatewayURL = %q, want %q", tc.in, st.GatewayURL, tc.want)
		}
		tm.Stop()
	}
}

// TestTunnelStatusNeverExposesLoopbackEndpoint asserts that no outward-facing path of the
// bridge ever carries a loopback or host-less endpoint, because the raw TunnelStatus is
// serialized verbatim to the UI (RemoteControlPage.tsx renders tunnel.gateway_url).
func TestTunnelStatusNeverExposesLoopbackEndpoint(t *testing.T) {
	assertExternal := func(where, raw string) {
		t.Helper()
		if raw == "" {
			t.Errorf("%s is empty while a tunnel is active", where)
			return
		}
		if isLoopbackURL(raw) {
			t.Errorf("%s exposes a loopback endpoint: %q", where, raw)
		}
		if u, err := url.Parse(raw); err != nil || u.Hostname() == "" {
			t.Errorf("%s is not a host-bearing URL: %q", where, raw)
		}
	}

	// 1. A loopback custom endpoint is rejected outright — there is no "active but hidden"
	//    state that could desync the UI.
	tm := NewTunnelManager(8766, "/remote")
	if _, err := tm.Start("custom", "https://127.0.0.1:8766"); err == nil {
		t.Fatal("loopback custom endpoint must be rejected")
	}

	// 2. A legitimate endpoint is surfaced consistently through the raw TunnelStatus and the
	//    canonical endpoint helper.
	if _, err := tm.Start("custom", "https://remote.example.com"); err != nil {
		t.Fatalf("valid custom tunnel rejected: %v", err)
	}
	st := tm.GetStatus()
	assertExternal("TunnelManager.GetStatus().GatewayURL", st.GatewayURL)
	assertExternal("TunnelManager.GetStatus().PublicURL", st.PublicURL)
	if got := externalEndpoint(st); got != "https://remote.example.com/remote" {
		t.Errorf("externalEndpoint() = %q, want the external endpoint", got)
	}
	tm.Stop()

	// 3. RemoteManager: canonical endpoint, nested raw tunnel status and persisted config all
	//    go through the same sanitized TunnelStatus.
	rm := NewRemoteManager(filepath.Join(t.TempDir(), "remote_control.json"))
	if _, err := rm.StartTunnel("custom", "https://remote.example.com"); err != nil {
		t.Fatalf("RemoteManager.StartTunnel failed: %v", err)
	}
	rs := rm.GetStatus("test@example.com", 0)
	assertExternal("RemoteControlStatus.EndpointURL", rs.EndpointURL)
	assertExternal("RemoteControlStatus.Tunnel.GatewayURL", rs.Tunnel.GatewayURL)
	assertExternal("RemoteControlStatus.Tunnel.PublicURL", rs.Tunnel.PublicURL)
	assertExternal("RemoteControlConfig.TunnelURL", rm.GetConfig().TunnelURL)
	rm.StopTunnel()

	// 4. Gemini Bridge service status exposes the same raw snapshot via BridgeStatus.Tunnel.
	svc, err := NewService(filepath.Join(t.TempDir(), "gemini_bridge.json"))
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	if _, err := svc.StartTunnel("custom", "https://remote.example.com"); err != nil {
		t.Fatalf("Service.StartTunnel failed: %v", err)
	}
	bs := svc.GetStatus()
	assertExternal("BridgeStatus.GatewayURL", bs.GatewayURL)
	assertExternal("BridgeStatus.Tunnel.GatewayURL", bs.Tunnel.GatewayURL)
	assertExternal("BridgeStatus.Tunnel.PublicURL", bs.Tunnel.PublicURL)
	svc.StopTunnel()

	// 5. Defense in depth: even a polluted raw snapshot is sanitized on read, so a future
	//    write path can never leak loopback endpoints to the UI.
	polluted := NewTunnelManager(8766, "/remote")
	polluted.mu.Lock()
	polluted.status = TunnelStatus{
		Active:     true,
		Type:       "custom",
		GatewayURL: "https://127.0.0.1:8766/remote",
		PublicURL:  "https://127.0.0.1:8766",
	}
	polluted.mu.Unlock()
	got := polluted.GetStatus()
	if got.GatewayURL != "" || got.PublicURL != "" {
		t.Errorf("GetStatus must sanitize polluted loopback endpoints, got gateway=%q public=%q", got.GatewayURL, got.PublicURL)
	}
	if gotEp := externalEndpoint(got); gotEp != "" {
		t.Errorf("externalEndpoint must not surface polluted loopback, got %q", gotEp)
	}

	// 6. Persisted config is sanitized both on load and on update, so the config API can
	//    neither serve a stale loopback endpoint nor accept a new one.
	staleRemoteCfg := filepath.Join(t.TempDir(), "remote_control.json")
	staleJSON := []byte(`{"tunnel_type":"custom","tunnel_url":"https://127.0.0.1:8766/remote"}`)
	if err := os.WriteFile(staleRemoteCfg, staleJSON, 0644); err != nil {
		t.Fatalf("write stale remote config: %v", err)
	}
	staleRM := NewRemoteManager(staleRemoteCfg)
	if loaded := staleRM.GetConfig().TunnelURL; loaded != "" {
		t.Errorf("loaded remote config must drop stale loopback TunnelURL, got %q", loaded)
	}
	if err := staleRM.UpdateConfig(RemoteControlConfig{TunnelURL: "https://127.0.0.1:8766/remote"}); err != nil {
		t.Fatalf("RemoteManager.UpdateConfig failed: %v", err)
	}
	if stored := staleRM.GetConfig().TunnelURL; stored != "" {
		t.Errorf("RemoteManager.UpdateConfig must strip loopback TunnelURL, got %q", stored)
	}

	staleBridgeCfg := filepath.Join(t.TempDir(), "gemini_bridge.json")
	if err := os.WriteFile(staleBridgeCfg, staleJSON, 0644); err != nil {
		t.Fatalf("write stale bridge config: %v", err)
	}
	staleSvc, err := NewService(staleBridgeCfg)
	if err != nil {
		t.Fatalf("NewService(stale config) failed: %v", err)
	}
	if loaded := staleSvc.GetConfig().TunnelURL; loaded != "" {
		t.Errorf("loaded bridge config must drop stale loopback TunnelURL, got %q", loaded)
	}
	if err := staleSvc.UpdateConfig(Config{TunnelURL: "https://127.0.0.1:8766/remote"}); err != nil {
		t.Fatalf("Service.UpdateConfig failed: %v", err)
	}
	if stored := staleSvc.GetConfig().TunnelURL; stored != "" {
		t.Errorf("Service.UpdateConfig must strip loopback TunnelURL, got %q", stored)
	}
}

func TestRemoteManager_TunnelLifecycleDrivesEnabledAndClearsEndpoint(t *testing.T) {
	rm := NewRemoteManager(filepath.Join(t.TempDir(), "remote_control.json"))

	if _, err := rm.StartTunnel("custom", "https://remote.example.com"); err != nil {
		t.Fatalf("StartTunnel failed: %v", err)
	}
	started := rm.GetConfig()
	if !started.Enabled {
		t.Fatal("starting a tunnel must mark the remote wrapper enabled")
	}
	if started.TunnelType != "custom" || started.TunnelURL == "" {
		t.Fatalf("started tunnel config incomplete: type=%q url=%q", started.TunnelType, started.TunnelURL)
	}

	rm.StopTunnel()
	stopped := rm.GetConfig()
	if stopped.Enabled {
		t.Fatal("stopping a tunnel must mark the remote wrapper disabled")
	}
	if stopped.TunnelType != "" || stopped.TunnelURL != "" {
		t.Fatalf("stopping a tunnel must clear endpoint state: type=%q url=%q", stopped.TunnelType, stopped.TunnelURL)
	}
}

func TestService_TunnelLifecycleDrivesEnabledAndClearsEndpoint(t *testing.T) {
	svc, err := NewService(filepath.Join(t.TempDir(), "gemini_bridge.json"))
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	if _, err := svc.StartTunnel("custom", "https://bridge.example.com"); err != nil {
		t.Fatalf("StartTunnel failed: %v", err)
	}
	started := svc.GetConfig()
	if !started.Enabled {
		t.Fatal("starting a tunnel must mark the Gemini bridge enabled")
	}
	if started.TunnelType != "custom" || started.TunnelURL == "" {
		t.Fatalf("started tunnel config incomplete: type=%q url=%q", started.TunnelType, started.TunnelURL)
	}
	if status := svc.GetStatus(); !status.Enabled {
		t.Fatal("bridge status must report enabled while the tunnel is active")
	}

	svc.StopTunnel()
	stopped := svc.GetConfig()
	if stopped.Enabled {
		t.Fatal("stopping a tunnel must mark the Gemini bridge disabled")
	}
	if stopped.TunnelType != "" || stopped.TunnelURL != "" {
		t.Fatalf("stopping a tunnel must clear endpoint state: type=%q url=%q", stopped.TunnelType, stopped.TunnelURL)
	}
	if status := svc.GetStatus(); status.Enabled || status.GatewayURL != "" {
		t.Fatalf("stopped bridge status retained enabled/endpoint state: enabled=%t url=%q", status.Enabled, status.GatewayURL)
	}
}

func TestService_StatusRequiresActiveTunnelNotPersistedEnabledFlag(t *testing.T) {
	svc, err := NewService(filepath.Join(t.TempDir(), "gemini_bridge.json"))
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	cfg := svc.GetConfig()
	cfg.Enabled = true
	if err := svc.UpdateConfig(cfg); err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}
	if status := svc.GetStatus(); status.Enabled {
		t.Fatal("persisted enabled flag must not report an active bridge without a live tunnel")
	}
}

func TestService_RejectedTunnelReplacementPreservesActiveSession(t *testing.T) {
	svc, err := NewService(filepath.Join(t.TempDir(), "gemini_bridge.json"))
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	if _, err := svc.StartTunnel("custom", "https://bridge.example.com"); err != nil {
		t.Fatalf("initial StartTunnel failed: %v", err)
	}
	if _, err := svc.StartTunnel("custom", "https://"); err == nil {
		t.Fatal("expected invalid replacement URL to be rejected")
	}

	cfg := svc.GetConfig()
	if !cfg.Enabled || cfg.TunnelType != "custom" || cfg.TunnelURL == "" {
		t.Fatalf("rejected replacement changed the active config: enabled=%t type=%q url=%q", cfg.Enabled, cfg.TunnelType, cfg.TunnelURL)
	}
	if status := svc.GetStatus(); !status.Enabled || !status.Tunnel.Active {
		t.Fatalf("rejected replacement stopped the active tunnel: enabled=%t active=%t", status.Enabled, status.Tunnel.Active)
	}
}

func TestService_AutoStartUsesTunnelLifecycle(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoStartTunnel = true
	cfg.TunnelType = "custom"
	cfg.TunnelURL = "https://auto.example.com"

	configPath := filepath.Join(t.TempDir(), "gemini_bridge.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	svc, err := NewService(configPath)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	defer svc.StopTunnel()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && !svc.GetConfig().Enabled {
		time.Sleep(10 * time.Millisecond)
	}
	if !svc.GetConfig().Enabled {
		t.Fatal("auto-start bypassed the tunnel lifecycle and left the bridge disabled")
	}
}


func TestTunnelManager_SetPort(t *testing.T) {
	tm := NewTunnelManager(8765, "/api/bridge/mcp")
	if tm.localPort != 8765 {
		t.Fatalf("expected 8765, got %d", tm.localPort)
	}
	tm.SetPort(9999)
	if tm.localPort != 9999 {
		t.Fatalf("expected 9999, got %d", tm.localPort)
	}
	tm.SetPort(-1)
	if tm.localPort != 9999 {
		t.Fatalf("expected 9999 after non-positive SetPort, got %d", tm.localPort)
	}
}

func TestRemoteManager_LocalHTTPServer_AndPINAuth(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "remote_control.json")

	rm := NewRemoteManager(cfgPath)
	defer rm.StopTunnel()

	// 1. Verify StartTunnel starts real local HTTP server on boundPort
	st, err := rm.StartTunnel("custom", "https://remote.example.com")
	if err != nil {
		t.Fatalf("StartTunnel failed: %v", err)
	}
	if !st.Active {
		t.Fatalf("expected active tunnel status")
	}

	boundPort := rm.boundPort
	if boundPort <= 0 {
		t.Fatalf("expected positive boundPort, got %d", boundPort)
	}

	client := &http.Client{Timeout: 5 * time.Second}

	// GET http://127.0.0.1:<boundPort>/remote
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/remote", boundPort))
	if err != nil {
		t.Fatalf("GET /remote failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /remote, got %d", resp.StatusCode)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read /remote body failed: %v", err)
	}
	if !strings.Contains(string(bodyBytes), "Antigravity Remote Control") {
		t.Fatalf("expected /remote HTML to contain 'Antigravity Remote Control', got:\n%s", string(bodyBytes))
	}

	// 2. POST /remote/api/auth rejects wrong PIN with 401
	badAuthBody := strings.NewReader(`{"pin":"000000"}`)
	respBad, err := client.Post(fmt.Sprintf("http://127.0.0.1:%d/remote/api/auth", boundPort), "application/json", badAuthBody)
	if err != nil {
		t.Fatalf("POST bad PIN failed: %v", err)
	}
	defer respBad.Body.Close()
	if respBad.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for wrong PIN, got %d", respBad.StatusCode)
	}

	// POST /remote/api/auth accepts rm.GetConfig().SecurityPIN with 200 OK
	curPIN := rm.GetConfig().SecurityPIN
	goodAuthBody := strings.NewReader(fmt.Sprintf(`{"pin":%q}`, curPIN))
	respGood, err := client.Post(fmt.Sprintf("http://127.0.0.1:%d/remote/api/auth", boundPort), "application/json", goodAuthBody)
	if err != nil {
		t.Fatalf("POST valid PIN failed: %v", err)
	}
	defer respGood.Body.Close()
	if respGood.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for valid PIN, got %d", respGood.StatusCode)
	}

	var authResult map[string]interface{}
	if err := json.NewDecoder(respGood.Body).Decode(&authResult); err != nil {
		t.Fatalf("decode auth response failed: %v", err)
	}
	if authResult["success"] != true {
		t.Fatalf("expected success: true, got %v", authResult)
	}

	// Verify cookie was set
	cookies := respGood.Cookies()
	var authCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "swiss_remote_pin" {
			authCookie = c
			break
		}
	}
	if authCookie == nil || authCookie.Value != curPIN {
		t.Fatalf("expected swiss_remote_pin cookie with valid PIN, got %v", authCookie)
	}

	// Status endpoint with cookie succeeds
	reqStatus, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/remote/api/status", boundPort), nil)
	reqStatus.AddCookie(authCookie)
	respStatus, err := client.Do(reqStatus)
	if err != nil {
		t.Fatalf("GET /remote/api/status with cookie failed: %v", err)
	}
	defer respStatus.Body.Close()
	if respStatus.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from status with cookie, got %d", respStatus.StatusCode)
	}

	// 3. rm.RotatePIN() immediately revokes previous PIN (401)
	newPIN, err := rm.RotatePIN()
	if err != nil {
		t.Fatalf("RotatePIN failed: %v", err)
	}
	if newPIN == curPIN {
		t.Fatalf("expected new PIN after rotation, got same %s", newPIN)
	}

	// Request with old cookie is now 401 Unauthorized
	reqOldCookie, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/remote/api/status", boundPort), nil)
	reqOldCookie.AddCookie(authCookie)
	respRevoked, err := client.Do(reqOldCookie)
	if err != nil {
		t.Fatalf("GET /remote/api/status after rotate failed: %v", err)
	}
	defer respRevoked.Body.Close()
	if respRevoked.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized after PIN rotation, got %d", respRevoked.StatusCode)
	}

	// Old PIN auth attempt is also 401 Unauthorized
	oldAuthBody := strings.NewReader(fmt.Sprintf(`{"pin":%q}`, curPIN))
	respOldAuth, err := client.Post(fmt.Sprintf("http://127.0.0.1:%d/remote/api/auth", boundPort), "application/json", oldAuthBody)
	if err != nil {
		t.Fatalf("POST old PIN failed: %v", err)
	}
	defer respOldAuth.Body.Close()
	if respOldAuth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for rotated old PIN, got %d", respOldAuth.StatusCode)
	}

	// New PIN auth succeeds
	newAuthBody := strings.NewReader(fmt.Sprintf(`{"pin":%q}`, newPIN))
	respNewAuth, err := client.Post(fmt.Sprintf("http://127.0.0.1:%d/remote/api/auth", boundPort), "application/json", newAuthBody)
	if err != nil {
		t.Fatalf("POST new PIN failed: %v", err)
	}
	defer respNewAuth.Body.Close()
	if respNewAuth.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for new PIN, got %d", respNewAuth.StatusCode)
	}
}

func TestRemoteConsoleHTML_NoJSSyntaxErrorAndDefaultVisibleAuth(t *testing.T) {
	// 1. Extract <script> block and verify zero JS syntax errors via Node vm.Script
	startIdx := strings.Index(remoteHTML, "<script>")
	endIdx := strings.LastIndex(remoteHTML, "</script>")
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		t.Fatalf("expected <script>...</script> block in remoteHTML")
	}
	scriptBody := remoteHTML[startIdx+len("<script>") : endIdx]

	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found on PATH")
	}

	cmd := exec.Command(nodePath, "-e", `
const fs = require("fs");
const vm = require("vm");
const src = fs.readFileSync(0, "utf8");
new vm.Script(src, { filename: "remoteHTML.js" });
`)
	cmd.Stdin = strings.NewReader(scriptBody)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("remoteHTML <script> contains a JavaScript syntax error: %v\n%s", err, string(out))
	}

	// 2. #authView must be visible on initial paint (never display:none by default)
	if strings.Contains(remoteHTML, `id="authView" class="auth-wrapper" style="display:none;"`) ||
		strings.Contains(remoteHTML, `id="authView" style="display:none"`) {
		t.Fatalf("remoteHTML must not hide #authView by default; initial paint must show the PIN unlock card")
	}

	// 3. #authError must not carry the active .error class when empty on initial paint
	if strings.Contains(remoteHTML, `id="authError" class="msg-banner error"`) {
		t.Fatalf("remoteHTML must not attach .error class to empty #authError on initial paint")
	}

	// 4. Verify Antigravity 2.0 entrance design tokens and logo SVG
	for _, required := range []string{"Welcome to Antigravity", "clip0_6153_71", "entrance-auth-panel", "logoEntrance"} {
		if !strings.Contains(remoteHTML, required) {
			t.Fatalf("expected remoteHTML to include Antigravity 2.0 entrance design element %q", required)
		}
	}
}

func TestRemoteManager_ProxiesLiveAntigravityWebApp(t *testing.T) {
	// Spin up a mock Antigravity language_server HTTPS web app
	var lastReceivedCSRF string
	var csrfMu sync.Mutex
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'self'")
		switch r.URL.Path {
		case "/", "/c/test-conv-123":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html><head><script>window.__APP_CONFIG__ = {"csrfToken":"fresh-ls-csrf-token-999"};</script><title>Antigravity</title></head><body><div id="root">Live Antigravity Web App</div><script src="/main.js"></script></body></html>`))
		case "/main.js":
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = w.Write([]byte(`console.log("antigravity main bundle");`))
		case "/exa.language_server_pb.LanguageServerService/GetServerConfiguration":
			csrfMu.Lock()
			lastReceivedCSRF = r.Header.Get("x-codeium-csrf-token")
			csrfMu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"config":{"standalone":true,"appDataDir":"antigravity"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	rm := NewRemoteManager(filepath.Join(t.TempDir(), "remote_control.json"))
	defer rm.StopTunnel()
	rm.SetWebOriginOverride(upstream.URL, "/c/test-conv-123")

	if _, err := rm.StartTunnel("custom", "https://remote.example.com"); err != nil {
		t.Fatalf("StartTunnel failed: %v", err)
	}

	rm.mu.RLock()
	boundPort := rm.boundPort
	rm.mu.RUnlock()
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", boundPort)

	// 1. Unauthenticated GET /remote serves the PIN unlock screen
	respUnauth, err := http.Get(baseURL + "/remote")
	if err != nil {
		t.Fatalf("GET /remote failed: %v", err)
	}
	unauthBody, _ := io.ReadAll(respUnauth.Body)
	respUnauth.Body.Close()
	if !strings.Contains(string(unauthBody), "Unlock Console") {
		t.Fatalf("expected PIN unlock screen before auth, got: %s", string(unauthBody))
	}

	// 2. Authenticate with the 6-digit Security PIN
	pin := rm.GetConfig().SecurityPIN
	authResp, err := http.Post(baseURL+"/remote/api/auth", "application/json", strings.NewReader(fmt.Sprintf(`{"pin":%q}`, pin)))
	if err != nil {
		t.Fatalf("POST /remote/api/auth failed: %v", err)
	}
	var authJSON map[string]interface{}
	_ = json.NewDecoder(authResp.Body).Decode(&authJSON)
	cookies := authResp.Cookies()
	authResp.Body.Close()

	if authJSON["web_app_available"] != true || authJSON["redirect"] != "/c/test-conv-123" {
		t.Fatalf("expected web_app_available=true and redirect=/c/test-conv-123, got: %+v", authJSON)
	}

	// 3. Authenticated GET /remote redirects to /c/test-conv-123 and serves the live Antigravity Web App
	reqApp, _ := http.NewRequest(http.MethodGet, baseURL+"/remote", nil)
	for _, ck := range cookies {
		reqApp.AddCookie(ck)
	}
	respApp, err := http.DefaultClient.Do(reqApp)
	if err != nil {
		t.Fatalf("authenticated GET /remote failed: %v", err)
	}
	appBody, _ := io.ReadAll(respApp.Body)
	respApp.Body.Close()
	if !strings.Contains(string(appBody), "Live Antigravity Web App") {
		t.Fatalf("expected proxied Antigravity Web App HTML, got: %s", string(appBody))
	}
	if !strings.Contains(string(appBody), "swiss-remote-account-sync") || !strings.Contains(string(appBody), "userStatus/setUserStatus") {
		t.Fatalf("expected proxied HTML to include injected live account sync script")
	}
	if respApp.Header.Get("Content-Security-Policy") != "" {
		t.Fatalf("expected restrictive Content-Security-Policy header to be stripped by proxy")
	}

	// 4. Authenticated Connect-RPC call with stale client CSRF token is rewritten to fresh language_server CSRF token
	reqRPC, _ := http.NewRequest(http.MethodPost, baseURL+"/exa.language_server_pb.LanguageServerService/GetServerConfiguration", strings.NewReader(`{}`))
	reqRPC.Header.Set("x-codeium-csrf-token", "stale-old-csrf-token-000")
	for _, ck := range cookies {
		reqRPC.AddCookie(ck)
	}
	respRPC, err := http.DefaultClient.Do(reqRPC)
	if err != nil {
		t.Fatalf("proxied RPC failed: %v", err)
	}
	rpcBody, _ := io.ReadAll(respRPC.Body)
	respRPC.Body.Close()
	if !strings.Contains(string(rpcBody), `"appDataDir":"antigravity"`) {
		t.Fatalf("expected proxied RPC response from language_server, got: %s", string(rpcBody))
	}
	csrfMu.Lock()
	gotCSRF := lastReceivedCSRF
	csrfMu.Unlock()
	if gotCSRF != "fresh-ls-csrf-token-999" {
		t.Fatalf("expected proxy to rewrite x-codeium-csrf-token to fresh-ls-csrf-token-999, got %q", gotCSRF)
	}
}
