package acpmesh

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func writeACPTestScript(t *testing.T, body string) string {
	return writeACPTestScriptNamed(t, "antigravity-acp", body)
}

func writeACPTestScriptNamed(t *testing.T, name, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-based ACP stdio fixture is not available on Windows")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, name)
	content := "#!/bin/sh\n" + body
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("write fake ACP executable: %v", err)
	}
	return script
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func TestProbeAgentHandshakeACPStdio(t *testing.T) {
	requestLog := filepath.Join(t.TempDir(), "initialize.jsonl")
	script := writeACPTestScript(t, `
if [ "$1" != "--surface" ] || [ "$2" != "cli" ]; then
  printf 'unexpected args: %s %s\n' "$1" "$2" >&2
  exit 2
fi
IFS= read -r line || exit 3
printf '%s\n' "$line" > `+shellSingleQuote(requestLog)+`
printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":1,"agentInfo":{"name":"fake-antigravity-acp","version":"9.8.7"}}}'
`)

	agent := Resolved{
		ID:         PresetAntigravityACP,
		Name:       "Antigravity ACP Adapter",
		BinaryPath: script,
	}
	result := ProbeAgentHandshake(context.Background(), agent, script)

	if !result.Reachable || !result.Verified {
		t.Fatalf("expected a verified handshake, got %+v", result)
	}
	if result.Transport != "stdio-acp" {
		t.Fatalf("transport = %q, want stdio-acp", result.Transport)
	}
	if result.Endpoint != script {
		t.Fatalf("endpoint = %q, want %q", result.Endpoint, script)
	}
	if result.AgentName != "fake-antigravity-acp" || result.AgentVersion != "9.8.7" {
		t.Fatalf("unexpected agent metadata: %+v", result)
	}
	if result.LatencyMs <= 0 {
		t.Fatalf("latency must be measured, got %f", result.LatencyMs)
	}
	if result.Error != "" || result.Summary == "" {
		t.Fatalf("unexpected handshake result: %+v", result)
	}

	raw, err := os.ReadFile(requestLog)
	if err != nil {
		t.Fatalf("read initialize request: %v", err)
	}
	request := string(raw)
	if !strings.Contains(request, `"method":"initialize"`) ||
		!strings.Contains(request, `"protocolVersion":1`) ||
		!strings.Contains(request, `"name":"antigravity-swiss-knife"`) {
		t.Fatalf("unexpected initialize request: %s", request)
	}
}

func TestPromptAgentACPStdio(t *testing.T) {
	requestLog := filepath.Join(t.TempDir(), "session.jsonl")
	script := writeACPTestScript(t, `
if [ "$1" != "--surface" ] || [ "$2" != "cli" ]; then
  exit 2
fi
while IFS= read -r line; do
  printf '%s\n' "$line" >> `+shellSingleQuote(requestLog)+`
  case "$line" in
    *'"method":"initialize"'*)
      printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":1,"agentInfo":{"name":"fake-antigravity-acp","version":"9.8.7"}}}'
      ;;
    *'"method":"session/new"'*)
      printf '%s\n' '{"jsonrpc":"2.0","id":2,"result":{"sessionId":"session-267"}}'
      ;;
    *'"method":"session/prompt"'*)
      printf '%s\n' '{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"session-267","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"real "}}}}'
      printf '%s\n' '{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"session-267","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"answer"}}}}'
      printf '%s\n' '{"jsonrpc":"2.0","id":3,"result":{"stopReason":"end_turn"}}'
      ;;
  esac
done
`)

	agent := Resolved{
		ID:         PresetAntigravityACP,
		Name:       "Antigravity ACP Adapter",
		BinaryPath: script,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := PromptAgent(ctx, agent, script, "Reply with real answer", t.TempDir())
	if err != nil {
		t.Fatalf("PromptAgent failed: %v", err)
	}
	if result["response"] != "real answer" {
		t.Fatalf("response = %#v, want %q", result["response"], "real answer")
	}
	if result["transport"] != "stdio-acp" {
		t.Fatalf("transport = %#v, want stdio-acp", result["transport"])
	}
	if result["session_id"] != "session-267" {
		t.Fatalf("session_id = %#v, want session-267", result["session_id"])
	}
	if result["stop_reason"] != "end_turn" {
		t.Fatalf("stop_reason = %#v, want end_turn", result["stop_reason"])
	}

	raw, err := os.ReadFile(requestLog)
	if err != nil {
		t.Fatalf("read ACP requests: %v", err)
	}
	requests := string(raw)
	for _, method := range []string{"initialize", "session/new", "session/prompt"} {
		if !strings.Contains(requests, `"method":"`+method+`"`) {
			t.Fatalf("request log is missing %s: %s", method, requests)
		}
	}
	if !strings.Contains(requests, `"prompt":[{"text":"Reply with real answer","type":"text"}]`) {
		t.Fatalf("prompt content missing from request: %s", requests)
	}
}

func TestPromptAgentPiRPC(t *testing.T) {
	requestLog := filepath.Join(t.TempDir(), "pi.jsonl")
	script := writeACPTestScript(t, `
[ "$1" = "--offline" ] || exit 2
[ "$2" = "--no-session" ] || exit 3
[ "$3" = "--no-themes" ] || exit 4
[ "$4" = "--mode" ] || exit 5
[ "$5" = "rpc" ] || exit 6
[ "$PI_OFFLINE" = "1" ] || exit 7
IFS= read -r line || exit 8
printf '%s\n' "$line" >> `+shellSingleQuote(requestLog)+`
printf '%s\n' '{"id":"prompt-1","type":"response","command":"prompt","success":true}'
printf '%s\n' '{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"pi "}}'
printf '%s\n' '{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"answer"}}'
printf '%s\n' '{"type":"agent_settled"}'
`)

	agent := Resolved{
		ID:         PresetPi,
		Name:       "Pi",
		BinaryPath: script,
		Endpoint:   "127.0.0.1:41888",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := PromptAgent(ctx, agent, script, "Reply via Pi", t.TempDir())
	if err != nil {
		t.Fatalf("PromptAgent failed: %v", err)
	}
	if result["response"] != "pi answer" {
		t.Fatalf("response = %#v, want %q", result["response"], "pi answer")
	}
	if result["transport"] != "pi-rpc" || result["protocol"] != "pi-rpc-v1" {
		t.Fatalf("unexpected Pi transport: %#v", result)
	}
	if result["stop_reason"] != "end_turn" {
		t.Fatalf("stop_reason = %#v, want end_turn", result["stop_reason"])
	}

	raw, err := os.ReadFile(requestLog)
	if err != nil {
		t.Fatalf("read Pi request: %v", err)
	}
	if !strings.Contains(string(raw), `"type":"prompt"`) ||
		!strings.Contains(string(raw), `"message":"Reply via Pi"`) {
		t.Fatalf("unexpected Pi prompt request: %s", raw)
	}
}

func TestPromptAgentPiACPStdio(t *testing.T) {
	script := writeACPTestScriptNamed(t, "pi-acp", `
while IFS= read -r line; do
  case "$line" in
    *'"method":"initialize"'*)
      printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":1,"agentInfo":{"name":"pi-acp","version":"3.4.5"}}}'
      ;;
    *'"method":"session/new"'*)
      printf '%s\n' '{"jsonrpc":"2.0","id":2,"result":{"sessionId":"pi-acp-session"}}'
      ;;
    *'"method":"session/prompt"'*)
      printf '%s\n' '{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"pi-acp-session","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"pi-acp answer"}}}}'
      printf '%s\n' '{"jsonrpc":"2.0","id":3,"result":{"stopReason":"end_turn"}}'
      ;;
  esac
done
`)

	agent := Resolved{
		ID:         PresetPi,
		Name:       "Pi",
		BinaryPath: script,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := PromptAgent(ctx, agent, script, "Reply through pi-acp", t.TempDir())
	if err != nil {
		t.Fatalf("PromptAgent failed: %v", err)
	}
	if result["response"] != "pi-acp answer" || result["transport"] != "stdio-acp" {
		t.Fatalf("unexpected pi-acp result: %#v", result)
	}
	if result["session_id"] != "pi-acp-session" {
		t.Fatalf("session_id = %#v, want pi-acp-session", result["session_id"])
	}
}

func TestProbeAgentHandshakeRealPi(t *testing.T) {
	const piPath = "/home/david/.local/bin/pi"
	if _, err := os.Stat(piPath); err != nil {
		t.Skip("installed pi binary is not present")
	}

	agent := Resolved{
		ID:         PresetPi,
		Name:       "Pi",
		BinaryPath: piPath,
		Endpoint:   "127.0.0.1:41888",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := ProbeAgentHandshake(ctx, agent, piPath)
	if !result.Reachable || !result.Verified {
		t.Fatalf("real Pi handshake failed: %+v", result)
	}
	if result.Transport != "pi-rpc" || result.Protocol != "pi-rpc-v1" {
		t.Fatalf("unexpected real Pi transport: %+v", result)
	}
	if result.LatencyMs <= 0 {
		t.Fatalf("real Pi latency was not measured: %+v", result)
	}
	wantTools := []string{"read", "bash", "edit", "write", "grep", "find", "ls"}
	if strings.Join(result.Tools, ",") != strings.Join(wantTools, ",") {
		t.Fatalf("tools = %v, want %v", result.Tools, wantTools)
	}
}

func TestProbeAgentHandshakeInstalledPiACP(t *testing.T) {
	const piACPPath = "/home/david/.config/nvm/versions/node/v24.16.0/bin/pi-acp"
	if _, err := os.Stat(piACPPath); err != nil {
		t.Skip("installed pi-acp binary is not present")
	}

	agent := Resolved{ID: "pi-acp-probe", Name: "pi-acp", BinaryPath: piACPPath}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := ProbeAgentHandshake(ctx, agent, piACPPath)
	if !result.Reachable || !result.Verified {
		t.Fatalf("installed pi-acp handshake failed: %+v", result)
	}
	if result.Transport != "stdio-acp" || result.AgentName != "pi-acp" {
		t.Fatalf("unexpected installed pi-acp result: %+v", result)
	}
}

func TestProbeAgentHandshakeBuiltAntigravityACP(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain is not available")
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	adapterDir := filepath.Join(repoRoot, "antigravity-acp")
	if _, err := os.Stat(filepath.Join(adapterDir, "cmd", "antigravity-acp")); err != nil {
		t.Skipf("antigravity-acp source is unavailable: %v", err)
	}

	binName := "antigravity-acp"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	bin := filepath.Join(t.TempDir(), binName)
	build := exec.Command("go", "build", "-o", bin, "./cmd/antigravity-acp")
	build.Dir = adapterDir
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build antigravity-acp: %v\n%s", err, output)
	}

	agent := Resolved{
		ID:         PresetAntigravityACP,
		Name:       "Antigravity ACP Adapter",
		BinaryPath: bin,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := ProbeAgentHandshake(ctx, agent, bin)
	if !result.Reachable || !result.Verified {
		t.Fatalf("built antigravity-acp handshake failed: %+v", result)
	}
	if result.Transport != "stdio-acp" {
		t.Fatalf("transport = %q, want stdio-acp", result.Transport)
	}
	if result.AgentName != "antigravity-acp" || result.AgentVersion == "" {
		t.Fatalf("unexpected built adapter metadata: %+v", result)
	}
}

func TestProbeAgentHandshakeSocket(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("no loopback listener available: %v", err)
	}
	defer listener.Close()

	agent := Resolved{
		ID:       "custom-socket-agent",
		Name:     "Socket Agent",
		Endpoint: listener.Addr().String(),
	}
	result := ProbeAgentHandshake(context.Background(), agent, "")
	if !result.Reachable {
		t.Fatalf("expected reachable socket, got %+v", result)
	}
	if result.Verified {
		t.Fatalf("a TCP connect alone must not be reported as protocol verified: %+v", result)
	}
	if result.Transport != "socket" || result.LatencyMs <= 0 {
		t.Fatalf("unexpected socket handshake result: %+v", result)
	}
}
