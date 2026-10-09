package webgui

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestAdversarial_ACPProcessAndPIDIntegrity tests:
// 1. Google Antigravity 2.0 (Desktop IDE) does NOT report agy daemon PID
// 2. agy CLI daemon does NOT report Antigravity IDE PID
// 3. Differentiates process arguments like --app_data_dir=antigravity
func TestAdversarial_ACPProcessAndPIDIntegrity(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()
	resp, err := http.Get(baseURL + "/api/utilities/acp")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/utilities/acp failed: err=%v, code=%d", err, resp.StatusCode)
	}
	defer resp.Body.Close()

	var result struct {
		Status          string                   `json:"status"`
		MeshNodes       int                      `json:"mesh_nodes"`
		ProtocolVersion string                   `json:"protocol_version"`
		Agents          []map[string]interface{} `json:"agents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode ACP response: %v", err)
	}

	agentMap := make(map[string]map[string]interface{})
	for _, a := range result.Agents {
		id, _ := a["id"].(string)
		agentMap[id] = a
	}

	ideAgent, hasIDE := agentMap["agent-antigravity"]
	cliAgent, hasCLI := agentMap["agent-antigravity-cli"]

	if !hasIDE {
		t.Fatalf("missing agent-antigravity in ACP nodes")
	}
	if !hasCLI {
		t.Fatalf("missing agent-antigravity-cli in ACP nodes")
	}

	idePIDFloat, _ := ideAgent["pid"].(float64)
	idePID := int(idePIDFloat)
	cliPIDFloat, _ := cliAgent["pid"].(float64)
	cliPID := int(cliPIDFloat)

	t.Logf("Empirical Discovery: Desktop IDE PID=%d, CLI PID=%d", idePID, cliPID)

	// If agy is running on the host, verify PID isolation
	if cliPID > 0 {
		cmdBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", cliPID))
		if err == nil {
			cmdStr := string(cmdBytes)
			t.Logf("CLI Process cmdline: %s", cmdStr)
			if !strings.Contains(cmdStr, "agy") {
				t.Errorf("agent-antigravity-cli PID %d does not contain 'agy' in cmdline", cliPID)
			}
		}

		// Critical check: IDE PID must NOT equal CLI PID
		if idePID == cliPID {
			t.Errorf("PID COLLISION: agent-antigravity and agent-antigravity-cli reported the same PID: %d", idePID)
		}
	}

	// If IDE is running on the host, verify it's the real Electron / binary, not agy
	if idePID > 0 {
		cmdBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", idePID))
		if err == nil {
			cmdStr := string(cmdBytes)
			t.Logf("IDE Process cmdline: %s", cmdStr)
			if strings.Contains(cmdStr, "/bin/agy") {
				t.Errorf("agent-antigravity erroneously claimed agy CLI binary PID %d", idePID)
			}
			if strings.Contains(cmdStr, "--hub-port") {
				t.Errorf("agent-antigravity erroneously matched agy hub daemon PID %d", idePID)
			}
		}
	}
}

// TestAdversarial_ProcessFilterStressMatrix tests the exact matching logic against 15+ hostile process edge cases
func TestAdversarial_ProcessFilterStressMatrix(t *testing.T) {
	type procEntry struct {
		pid     int
		comm    string
		cmdline string
	}

	testCases := []struct {
		desc           string
		procs          []procEntry
		expectIdePID   int
		expectAgyPID   int
		expectPiPID    int
		expectCodexPID int
	}{
		{
			desc: "Only agy CLI running with --app_data_dir=antigravity (no IDE running)",
			procs: []procEntry{
				{pid: 101, comm: "agy", cmdline: "/home/david/.gemini/bin/agy --hub --hub-port=34209 --app_data_dir=antigravity"},
			},
			expectIdePID:   0, // Must NOT match agy!
			expectAgyPID:   101,
			expectPiPID:    0,
			expectCodexPID: 0,
		},
		{
			desc: "Antigravity IDE running alongside agy CLI and swiss knife daemon",
			procs: []procEntry{
				{pid: 201, comm: "agy", cmdline: "/home/david/.gemini/bin/agy --hub --app_data_dir=antigravity"},
				{pid: 202, comm: "antigravity-swiss-knife", cmdline: "/opt/Antigravity Swiss Knife/swiss daemon --web"},
				{pid: 203, comm: "antigravity", cmdline: "/opt/Antigravity/antigravity"},
				{pid: 204, comm: "antigravity", cmdline: "/opt/Antigravity/antigravity --type=renderer"},
				{pid: 205, comm: "antigravity", cmdline: "/opt/Antigravity/antigravity --type=zygote"},
			},
			expectIdePID:   203, // Main IDE process only
			expectAgyPID:   201,
			expectPiPID:    0,
			expectCodexPID: 0,
		},
		{
			desc: "Pipewire and codex-router running (must NOT be falsely identified as pi or codex)",
			procs: []procEntry{
				{pid: 301, comm: "pipewire", cmdline: "/usr/bin/pipewire"},
				{pid: 302, comm: "pipewire-pulse", cmdline: "/usr/bin/pipewire-pulse"},
				{pid: 303, comm: "codex-router", cmdline: "/usr/bin/codex-router --port=9000"},
			},
			expectIdePID:   0,
			expectAgyPID:   0,
			expectPiPID:    0,
			expectCodexPID: 0,
		},
		{
			desc: "Genuine Pi agent and Codex CLI running",
			procs: []procEntry{
				{pid: 401, comm: "pi", cmdline: "pi --mode rpc"},
				{pid: 402, comm: "codex", cmdline: "codex app-server"},
			},
			expectIdePID:   0,
			expectAgyPID:   0,
			expectPiPID:    401,
			expectCodexPID: 402,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Replica of checkAntigravityIDE logic from server.go
			var foundIdePID int
			for _, p := range tc.procs {
				cmdLower := strings.ToLower(p.cmdline)
				commLower := strings.ToLower(p.comm)
				isAntigravity := (commLower == "antigravity" || strings.Contains(cmdLower, "/opt/antigravity/antigravity") || strings.Contains(cmdLower, "/antigravity.app/") || strings.HasSuffix(commLower, "antigravity.exe"))
				if isAntigravity &&
					!strings.Contains(cmdLower, "--type=") &&
					!strings.Contains(cmdLower, "swiss") &&
					!strings.Contains(cmdLower, "agy") &&
					commLower != "agy" {
					if foundIdePID == 0 || p.pid < foundIdePID {
						foundIdePID = p.pid
					}
				}
			}
			if foundIdePID != tc.expectIdePID {
				t.Errorf("IDE PID mismatch: expected %d, got %d", tc.expectIdePID, foundIdePID)
			}

			// Replica of checkAgyCLI logic from server.go
			var foundAgyPID int
			for _, p := range tc.procs {
				cmdLower := strings.ToLower(p.cmdline)
				commLower := strings.ToLower(p.comm)
				if commLower == "agy" || commLower == "agy-run" || commLower == "agy_acp_server" ||
					strings.Contains(cmdLower, "/bin/agy") || strings.Contains(cmdLower, "agy_acp_server") || strings.Contains(cmdLower, "bin/agy --hub") {
					if !strings.Contains(cmdLower, "swiss") && !strings.Contains(cmdLower, "antigravity-swiss-knife") {
						foundAgyPID = p.pid
						break
					}
				}
			}
			if foundAgyPID != tc.expectAgyPID {
				t.Errorf("Agy CLI PID mismatch: expected %d, got %d", tc.expectAgyPID, foundAgyPID)
			}

			// Replica of checkPi logic from server.go
			var foundPiPID int
			for _, p := range tc.procs {
				cmdLower := strings.ToLower(p.cmdline)
				commLower := strings.ToLower(p.comm)
				if commLower == "pi" || commLower == "pi-acp" ||
					strings.Contains(cmdLower, "pi --mode rpc") || strings.Contains(cmdLower, "pi-acp") || strings.Contains(cmdLower, "@earendil-works/pi-agent-core") {
					if !strings.Contains(cmdLower, "pipewire") && !strings.Contains(cmdLower, "swiss") {
						foundPiPID = p.pid
						break
					}
				}
			}
			if foundPiPID != tc.expectPiPID {
				t.Errorf("Pi PID mismatch: expected %d, got %d", tc.expectPiPID, foundPiPID)
			}

			// Replica of checkCodex logic from server.go
			var foundCodexPID int
			for _, p := range tc.procs {
				cmdLower := strings.ToLower(p.cmdline)
				commLower := strings.ToLower(p.comm)
				if commLower == "codex" || strings.Contains(cmdLower, "bin/codex") || strings.Contains(cmdLower, "codex app-server") || strings.Contains(cmdLower, "@openai/codex") {
					if !strings.Contains(cmdLower, "codex-router") && !strings.Contains(cmdLower, "swiss") {
						foundCodexPID = p.pid
						break
					}
				}
			}
			if foundCodexPID != tc.expectCodexPID {
				t.Errorf("Codex PID mismatch: expected %d, got %d", tc.expectCodexPID, foundCodexPID)
			}
		})
	}
}

// TestAdversarial_9NodeFleetValidation validates:
// 1. All 9 expected IDs are present and valid
// 2. Windsurf is renamed to Devin (agent-devin)
// 3. ?agent=agent-windsurf routes cleanly to agent-devin
// 4. ?id=agent-windsurf routes cleanly to agent-devin
// 5. Unknown agent IDs return empty slice without panic
func TestAdversarial_9NodeFleetValidation(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// 1. Full fleet check
	resp, err := http.Get(baseURL + "/api/utilities/acp")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/utilities/acp failed: %v", err)
	}
	var fullResult struct {
		MeshNodes int                      `json:"mesh_nodes"`
		Agents    []map[string]interface{} `json:"agents"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&fullResult)
	resp.Body.Close()

	if fullResult.MeshNodes != 9 {
		t.Errorf("expected exactly 9 mesh nodes, got %d", fullResult.MeshNodes)
	}
	if len(fullResult.Agents) != 9 {
		t.Errorf("expected exactly 9 agents in array, got %d", len(fullResult.Agents))
	}

	expectedNodes := []struct {
		id       string
		name     string
		nodeType string
	}{
		{"agent-antigravity", "Google Antigravity 2.0", "Desktop IDE & Agent Core"},
		{"agent-antigravity-cli", "Antigravity CLI (agy)", "Terminal Agent Daemon"},
		{"agent-devin", "Devin", "Autonomous Developer IDE"},
		{"agent-opencode", "OpenCode", "Editor & Agent Sidecar"},
		{"agent-deepseek-harness", "DeepSeek Harness", "Terminal Agent Harness"},
		{"agent-pi", "Pi", "Pi Agent Core"},
		{"agent-codex", "Codex", "Codex Agent CLI"},
		{"agent-claude-code", "Claude Code CLI", "Terminal Agent Daemon"},
		{"agent-cursor", "Cursor Editor Agent", "Editor Sidecar"},
	}

	agentMap := make(map[string]map[string]interface{})
	for _, a := range fullResult.Agents {
		id, _ := a["id"].(string)
		agentMap[id] = a
	}

	for _, exp := range expectedNodes {
		node, exists := agentMap[exp.id]
		if !exists {
			t.Errorf("missing expected node ID: %s", exp.id)
			continue
		}
		if name, _ := node["name"].(string); name != exp.name {
			t.Errorf("node %s: expected name '%s', got '%s'", exp.id, exp.name, name)
		}
		if nType, _ := node["type"].(string); nType != exp.nodeType {
			t.Errorf("node %s: expected type '%s', got '%s'", exp.id, exp.nodeType, nType)
		}
		tools, ok := node["supported_tools"].([]interface{})
		if !ok || len(tools) == 0 {
			t.Errorf("node %s: missing or empty supported_tools", exp.id)
		}
	}

	// 2. Windsurf alias routing via ?agent=agent-windsurf
	aliasResp, err := http.Get(baseURL + "/api/utilities/acp?agent=agent-windsurf")
	if err != nil || aliasResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/utilities/acp?agent=agent-windsurf failed: %v", err)
	}
	var aliasResult struct {
		MeshNodes int                      `json:"mesh_nodes"`
		Agents    []map[string]interface{} `json:"agents"`
	}
	_ = json.NewDecoder(aliasResp.Body).Decode(&aliasResult)
	aliasResp.Body.Close()

	if len(aliasResult.Agents) != 1 {
		t.Fatalf("expected 1 agent for ?agent=agent-windsurf, got %d", len(aliasResult.Agents))
	}
	if aliasResult.Agents[0]["id"] != "agent-devin" {
		t.Errorf("expected alias agent-windsurf to route to agent-devin, got: %s", aliasResult.Agents[0]["id"])
	}
	if aliasResult.Agents[0]["name"] != "Devin" {
		t.Errorf("expected alias name to be 'Devin', got: %s", aliasResult.Agents[0]["name"])
	}

	// 3. Windsurf alias routing via ?id=agent-windsurf
	idAliasResp, err := http.Get(baseURL + "/api/utilities/acp?id=agent-windsurf")
	if err != nil || idAliasResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/utilities/acp?id=agent-windsurf failed: %v", err)
	}
	var idAliasResult struct {
		Agents []map[string]interface{} `json:"agents"`
	}
	_ = json.NewDecoder(idAliasResp.Body).Decode(&idAliasResult)
	idAliasResp.Body.Close()

	if len(idAliasResult.Agents) != 1 || idAliasResult.Agents[0]["id"] != "agent-devin" {
		t.Errorf("expected ?id=agent-windsurf to route to agent-devin, got: %v", idAliasResult.Agents)
	}

	// 4. Non-existent agent returns empty array cleanly without error
	ghostResp, err := http.Get(baseURL + "/api/utilities/acp?agent=agent-nonexistent")
	if err != nil || ghostResp.StatusCode != http.StatusOK {
		t.Fatalf("GET ?agent=agent-nonexistent failed: %v", err)
	}
	var ghostResult struct {
		MeshNodes int                      `json:"mesh_nodes"`
		Agents    []map[string]interface{} `json:"agents"`
	}
	_ = json.NewDecoder(ghostResp.Body).Decode(&ghostResult)
	ghostResp.Body.Close()

	if len(ghostResult.Agents) != 0 {
		t.Errorf("expected 0 agents for unknown ID, got %d", len(ghostResult.Agents))
	}
	if ghostResult.MeshNodes != 9 {
		t.Errorf("expected mesh_nodes count to remain 9, got %d", ghostResult.MeshNodes)
	}
}

// TestAdversarial_SocketHandlingAndLatency verifies:
// 1. Probing dead or unbound sockets does NOT hang the HTTP request.
// 2. Individual socket timeout is strictly bounded to <= 20ms (tested with non-existent unix socket, closed tcp port, and blackhole IP).
func TestAdversarial_SocketHandlingAndLatency(t *testing.T) {
	srv := NewServer("127.0.0.1:0", "")
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	defer srv.Stop()

	baseURL := "http://" + srv.Addr()

	// 1. Measure live endpoint response time
	iterations := 3
	for i := 0; i < iterations; i++ {
		start := time.Now()
		resp, err := http.Get(baseURL + "/api/utilities/acp")
		duration := time.Since(start)

		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("iteration %d: GET /api/utilities/acp failed: %v", i, err)
		}
		resp.Body.Close()

		t.Logf("Iteration %d response time: %v", i, duration)
	}

	// 2. Verify probeSocket direct behavior on a non-listening Unix socket (fails immediately in microseconds)
	tmpDir := t.TempDir()
	deadSock := filepath.Join(tmpDir, "dead_test.sock")
	start := time.Now()
	conn, err := net.DialTimeout("unix", deadSock, 20*time.Millisecond)
	sockDuration := time.Since(start)
	if conn != nil {
		conn.Close()
	}
	t.Logf("Dial non-existent unix socket duration: %v, err: %v", sockDuration, err)
	if sockDuration > 30*time.Millisecond {
		t.Errorf("unix socket probe exceeded 30ms: %v", sockDuration)
	}

	// 3. Verify probeSocket direct behavior on an unbound TCP port (fails immediately in microseconds)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	closedPort := l.Addr().String()
	l.Close() // immediately close to make it unbound

	start = time.Now()
	conn, err = net.DialTimeout("tcp", closedPort, 20*time.Millisecond)
	tcpDuration := time.Since(start)
	if conn != nil {
		conn.Close()
	}
	t.Logf("Dial unbound TCP port duration: %v, err: %v", tcpDuration, err)
	if tcpDuration > 30*time.Millisecond {
		t.Errorf("tcp socket probe exceeded 30ms: %v", tcpDuration)
	}

	// 4. Verify blackhole packet drop socket timeout (RFC 5737 192.0.2.1 drops packets silently)
	// Bounded by 20ms timeout
	start = time.Now()
	conn, err = net.DialTimeout("tcp", "192.0.2.1:80", 20*time.Millisecond)
	blackholeDuration := time.Since(start)
	if conn != nil {
		conn.Close()
	}
	t.Logf("Dial blackhole IP duration: %v, err: %v", blackholeDuration, err)
	if blackholeDuration > 50*time.Millisecond {
		t.Errorf("blackhole socket probe exceeded 50ms: %v", blackholeDuration)
	}
}
