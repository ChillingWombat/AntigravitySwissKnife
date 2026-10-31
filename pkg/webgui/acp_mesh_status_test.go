package webgui

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/acpmesh"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// A detected process whose endpoint never answers is installed, not connected, and
// nothing about it may look negotiated.
func TestDetectedProcessWithoutEndpointIsNotConnected(t *testing.T) {
	agent := acpmesh.Resolved{
		ID:                "antigravity",
		Name:              "Google Antigravity 2.0",
		Endpoint:          "unix:///run/user/1000/antigravity-acp.sock",
		Version:           "v1.2.0-draft",
		Tools:             []string{"read_file", "write_file"},
		StatusWhenRunning: "active_hosting",
	}
	node := buildACPNode(agent, true, 4242, false)

	if node["status"] != "installed_idle" {
		t.Errorf("status = %v, want installed_idle", node["status"])
	}
	if node["ping_latency_ms"] != nil {
		t.Errorf("latency = %v, want nil when the endpoint is silent", node["ping_latency_ms"])
	}
	if node["acp_version"] != "" {
		t.Errorf("version = %v, want empty without a handshake", node["acp_version"])
	}
	if tools, _ := node["supported_tools"].([]string); len(tools) != 0 {
		t.Errorf("tools = %v, want none without a handshake", tools)
	}
	if node["last_handshake"] != "never" {
		t.Errorf("handshake = %v, want never", node["last_handshake"])
	}
}

// A reachable endpoint reports the measured latency and the declared metadata.
func TestReachableEndpointReportsMeasuredLatency(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("no loopback listener available")
	}
	defer listener.Close()

	agent := acpmesh.Resolved{
		ID:                "pi",
		Name:              "Pi",
		Endpoint:          listener.Addr().String(),
		Version:           "v1.1.2",
		Tools:             []string{"chat_eval"},
		StatusWhenRunning: "connected",
	}
	node := buildACPNode(agent, true, 1, true)

	if node["status"] != "connected" {
		t.Errorf("status = %v, want connected", node["status"])
	}
	if node["ping_latency_ms"] == nil {
		t.Error("latency missing for a reachable endpoint")
	}
	if node["acp_version"] != "v1.1.2" {
		t.Errorf("version = %v", node["acp_version"])
	}
}

// Nothing running and nothing reachable must not read as a healthy daemon.
func TestAbsentAgentReportsNotFound(t *testing.T) {
	node := buildACPNode(acpmesh.Resolved{ID: "ghost", Endpoint: "127.0.0.1:1"}, false, 0, false)
	if node["status"] != "not_found" {
		t.Errorf("status = %v, want not_found", node["status"])
	}
}

func TestInstalledStdioPresetsReportInstalled(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "agent")
	if err := os.WriteFile(bin, []byte("fake"), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	for _, id := range []string{acpmesh.PresetPi, acpmesh.PresetCodex} {
		t.Run(id, func(t *testing.T) {
			node := buildACPNode(acpmesh.Resolved{
				ID:         id,
				Name:       id,
				BinaryPath: bin,
			}, false, 0, false)

			if node["status"] != "installed_idle" {
				t.Fatalf("status = %v, want installed_idle", node["status"])
			}
			if node["transport"] != "stdio" {
				t.Fatalf("transport = %v, want stdio", node["transport"])
			}
			if node["ping_latency_ms"] != nil || node["last_handshake"] != "never" {
				t.Fatalf("unnegotiated node must not report a handshake: %#v", node)
			}
		})
	}
}

func TestAntigravityACPBinFallbackInCWD(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	bin := filepath.Join(binDir, "antigravity-acp")
	if err := os.WriteFile(bin, []byte("fake"), 0o755); err != nil {
		t.Fatalf("write fallback binary: %v", err)
	}
	t.Chdir(dir)

	for _, id := range []string{acpmesh.PresetAntigravityACP} {
		node := buildACPNode(acpmesh.Resolved{
			ID:         id,
			Name:       "Bundled Antigravity ACP",
			BinaryPath: filepath.Join(dir, "missing", "antigravity-acp"),
		}, false, 0, false)

		if node["status"] != "installed_idle" {
			t.Fatalf("%s status = %v, want installed_idle from CWD fallback", id, node["status"])
		}
		if node["binary_path"] != bin {
			t.Fatalf("%s binary_path = %v, want %s", id, node["binary_path"], bin)
		}
	}
}

func writeWebGUIFakeACP(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-based ACP stdio fixture is not available on Windows")
	}
	script := filepath.Join(t.TempDir(), "antigravity-acp")
	content := `#!/bin/sh
if [ "$1" != "--surface" ] || [ "$2" != "auto" ]; then
  exit 2
fi
while IFS= read -r line; do
  case "$line" in
    *'"method":"initialize"'*)
      printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":1,"agentInfo":{"name":"webgui-fake-acp","version":"2.6.7"}}}'
      ;;
    *'"method":"session/new"'*)
      printf '%s\n' '{"jsonrpc":"2.0","id":2,"result":{"sessionId":"webgui-session"}}'
      ;;
    *'"method":"session/prompt"'*)
      printf '%s\n' '{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"webgui-session","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"webgui "}}}}'
      printf '%s\n' '{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"webgui-session","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"answer"}}}}'
      printf '%s\n' '{"jsonrpc":"2.0","id":3,"result":{"stopReason":"end_turn"}}'
      ;;
  esac
done
`
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write fake ACP executable: %v", err)
	}
	return script
}

func configureFakeAntigravityACP(t *testing.T, bin string) {
	t.Helper()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", t.TempDir())
	cfg, err := core.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if err := cfg.SetACPAgents([]core.ACPAgentConfig{{
		ID:         acpmesh.PresetAntigravityACP,
		BinaryPath: bin,
	}}); err != nil {
		t.Fatalf("save ACP agent config: %v", err)
	}
}

func TestACPPingRouteReturnsMeasuredHandshake(t *testing.T) {
	bin := writeWebGUIFakeACP(t)
	configureFakeAntigravityACP(t, bin)

	srv := newTestServer("127.0.0.1:0", "")
	mux := srv.setupRoutes()
	req := httptest.NewRequest(http.MethodPost, "/api/utilities/acp/ping", bytes.NewBufferString(`{"id":"agent-antigravity-acp"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Status      string                    `json:"status"`
		Results     []acpmesh.HandshakeResult `json:"results"`
		Logs        []AcpHandshakeLog         `json:"logs"`
		Agents      []map[string]interface{}  `json:"agents"`
		OnlineCount int                       `json:"online_count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode ping response: %v", err)
	}
	if response.Status != "ok" || len(response.Results) != 1 {
		t.Fatalf("unexpected ping response: %+v", response)
	}
	result := response.Results[0]
	if !result.Reachable || !result.Verified || result.Transport != "stdio-acp" {
		t.Fatalf("unexpected handshake result: %+v", result)
	}
	if result.AgentVersion != "2.6.7" || result.LatencyMs <= 0 {
		t.Fatalf("handshake metadata is not real: %+v", result)
	}
	if response.OnlineCount != 1 || len(response.Logs) != 1 || response.Logs[0].Status != "success" {
		t.Fatalf("unexpected logs/count: %+v", response)
	}

	var node map[string]interface{}
	for _, candidate := range response.Agents {
		if candidate["id"] == acpmesh.PresetAntigravityACP {
			node = candidate
		}
	}
	if node == nil {
		t.Fatal("updated Antigravity ACP node is missing")
	}
	if node["status"] != "connected" || node["transport"] != "stdio-acp" {
		t.Fatalf("updated node was not reconciled with the handshake: %+v", node)
	}
}

func TestACPPromptRouteDispatchesACP(t *testing.T) {
	bin := writeWebGUIFakeACP(t)
	configureFakeAntigravityACP(t, bin)

	srv := newTestServer("127.0.0.1:0", "")
	mux := srv.setupRoutes()
	body := `{"id":"agent-antigravity-acp","prompt":"Reply from the route","cwd":"` + strings.ReplaceAll(t.TempDir(), `\`, `\\`) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/utilities/acp/prompt", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var response map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode prompt response: %v", err)
	}
	if response["status"] != "ok" || response["response"] != "webgui answer" {
		t.Fatalf("unexpected prompt response: %+v", response)
	}
	if response["transport"] != "stdio-acp" || response["session_id"] != "webgui-session" {
		t.Fatalf("unexpected prompt metadata: %+v", response)
	}
}
