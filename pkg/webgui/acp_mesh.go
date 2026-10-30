package webgui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/acpmesh"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/process"
)

// AcpHandshakeLog is one server-measured protocol probe.
type AcpHandshakeLog struct {
	ID             string `json:"id"`
	Timestamp      string `json:"timestamp"`
	FromAgent      string `json:"from_agent"`
	ToAgent        string `json:"to_agent"`
	Action         string `json:"action"`
	PayloadSummary string `json:"payload_summary"`
	Status         string `json:"status"`
}

// writeJSONStatus writes a JSON body with an explicit HTTP status code.
func writeJSONStatus(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeACPError(w http.ResponseWriter, status int, msg string) {
	writeJSONStatus(w, status, map[string]interface{}{"success": false, "error": msg})
}

// ---------------------------------------------------------------------------
// Process detection
// ---------------------------------------------------------------------------

// matchACPAgent reports whether p looks like the given preset agent. The rules
// are the historical matchers, kept verbatim so Linux behaviour does not change.
func matchACPAgent(id string, p process.ProcEntry) bool {
	cmdLower := strings.ToLower(p.Cmdline)
	// Windows Toolhelp reports "agy.exe" while Unix comm reports "agy"; compare
	// against the extension-stripped name and keep the raw value for the
	// historical ".exe" suffix rule.
	commRaw := strings.ToLower(p.Comm)
	commLower := strings.TrimSuffix(commRaw, ".exe")

	switch id {
	case acpmesh.PresetAntigravityACP:
		return (commLower == "antigravity-acp" ||
			commLower == "antigravity" ||
			commLower == "agy" ||
			commLower == "agy-run" ||
			commLower == "agy_acp_server" ||
			strings.Contains(cmdLower, "antigravity-acp") ||
			strings.Contains(cmdLower, "agy_acp_server") ||
			strings.Contains(cmdLower, "/opt/antigravity/antigravity") ||
			strings.Contains(cmdLower, "/antigravity.app/") ||
			strings.HasSuffix(commRaw, "antigravity.exe")) &&
			!strings.Contains(cmdLower, "--type=") &&
			!strings.Contains(cmdLower, "swiss")
	case acpmesh.PresetDevin:
		if !(commLower == "devin" || commLower == "devin-desktop" || commLower == "windsurf" ||
			strings.Contains(cmdLower, "devin-desktop") || strings.Contains(cmdLower, "bin/devin") ||
			strings.Contains(cmdLower, "bin/windsurf")) {
			return false
		}
		return !strings.Contains(cmdLower, "swiss") && !strings.Contains(cmdLower, "--type=")
	case acpmesh.PresetOpenCode:
		if !(commLower == "opencode" || strings.Contains(cmdLower, "bin/opencode") ||
			strings.Contains(cmdLower, "opencode acp") || strings.Contains(cmdLower, "opencode serve")) {
			return false
		}
		return !strings.Contains(cmdLower, "swiss")
	case acpmesh.PresetDeepSeek:
		if !(commLower == "dsh" || commLower == "deepseek" || commLower == "deepseek-harness" ||
			strings.Contains(cmdLower, "deepseek-harness") || strings.Contains(cmdLower, "bin/dsh") ||
			strings.Contains(cmdLower, "@deepseek-ai/dsh")) {
			return false
		}
		return !strings.Contains(cmdLower, "swiss")
	case acpmesh.PresetPi:
		if !(commLower == "pi" || commLower == "pi-acp" ||
			strings.Contains(cmdLower, "pi --mode rpc") || strings.Contains(cmdLower, "pi-acp") ||
			strings.Contains(cmdLower, "@earendil-works/pi-agent-core")) {
			return false
		}
		return !strings.Contains(cmdLower, "pipewire") && !strings.Contains(cmdLower, "swiss")
	case acpmesh.PresetCodex:
		if !(commLower == "codex" || strings.Contains(cmdLower, "bin/codex") ||
			strings.Contains(cmdLower, "codex app-server") || strings.Contains(cmdLower, "@openai/codex")) {
			return false
		}
		return !strings.Contains(cmdLower, "codex-router") && !strings.Contains(cmdLower, "swiss")
	case acpmesh.PresetClaudeCode:
		if !(commLower == "claude" || strings.Contains(cmdLower, "bin/claude") ||
			strings.Contains(cmdLower, "@anthropic-ai/claude-code")) {
			return false
		}
		return !strings.Contains(cmdLower, "swiss")
	case acpmesh.PresetCursor:
		if !(commLower == "cursor" || strings.Contains(cmdLower, "/cursor") || strings.Contains(cmdLower, "cursor.app")) {
			return false
		}
		return !strings.Contains(cmdLower, "--type=") && !strings.Contains(cmdLower, "swiss")
	}
	return false
}

// matchCustomAgent matches a user-added agent by the basename of its configured
// binary path, so custom nodes can still report a PID.
func matchCustomAgent(agent acpmesh.Resolved, p process.ProcEntry) bool {
	binPath := strings.ReplaceAll(agent.BinaryPath, `\`, "/")
	base := strings.ToLower(path.Base(binPath))
	if base == "" || base == "." {
		return false
	}
	commLower := strings.ToLower(p.Comm)
	if commLower == base || strings.HasSuffix(commLower, "/"+base) {
		return true
	}
	return strings.Contains(strings.ToLower(p.Cmdline), strings.ToLower(agent.BinaryPath))
}

// pgrepACPAgent is the historical fallback used when the process table cannot be
// enumerated. It is a no-op on Windows, where pgrep does not exist.
func pgrepACPAgent(id string) (bool, int) {
	if runtime.GOOS == "windows" {
		return false, 0
	}

	exact := map[string][]string{
		acpmesh.PresetAntigravityACP: {"antigravity-acp", "antigravity", "agy", "agy-run", "agy_acp_server"},
		acpmesh.PresetDevin:          {"devin", "devin-desktop", "windsurf"},
		acpmesh.PresetOpenCode:       {"opencode"},
		acpmesh.PresetDeepSeek:       {"dsh", "deepseek-harness"},
		acpmesh.PresetPi:             {"pi", "pi-acp"},
		acpmesh.PresetCodex:          {"codex"},
		acpmesh.PresetClaudeCode:     {"claude"},
		acpmesh.PresetCursor:         {"cursor"},
	}
	patterns := map[string][]string{
		acpmesh.PresetAntigravityACP: {"antigravity-acp", "/bin/agy", "agy_acp_server"},
		acpmesh.PresetDevin:          {"devin-desktop", "bin/devin", "bin/windsurf"},
		acpmesh.PresetOpenCode:       {"bin/opencode", "opencode acp"},
		acpmesh.PresetDeepSeek:       {"deepseek-harness", "bin/dsh"},
		acpmesh.PresetCodex:          {"bin/codex", "codex app-server"},
		acpmesh.PresetClaudeCode:     {"bin/claude"},
		acpmesh.PresetCursor:         {"/opt/Cursor/cursor"},
	}

	for _, name := range exact[id] {
		for _, pid := range pgrepPIDs("-x", name) {
			if id == acpmesh.PresetAntigravityACP && !antigravityCmdlineAllowed(pid) {
				continue
			}
			return true, pid
		}
	}
	for _, pattern := range patterns[id] {
		for _, pid := range pgrepPIDs("-f", pattern) {
			if id == acpmesh.PresetAntigravityACP && !antigravityCmdlineAllowed(pid) {
				continue
			}
			return true, pid
		}
	}
	return false, 0
}

func pgrepPIDs(flag, pattern string) []int {
	out, err := exec.Command("pgrep", flag, pattern).Output()
	if err != nil {
		return nil
	}
	var pids []int
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if pid, err := strconv.Atoi(strings.TrimSpace(line)); err == nil && pid > 0 {
			pids = append(pids, pid)
		}
	}
	return pids
}

// antigravityCmdlineAllowed filters Electron sub-processes when only pgrep is
// available. Without procfs the pid is accepted as-is.
func antigravityCmdlineAllowed(pid int) bool {
	cmdBytes, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil {
		return true
	}
	cmd := strings.ToLower(string(cmdBytes))
	return !strings.Contains(cmd, "--type=") && !strings.Contains(cmd, "swiss") && !strings.Contains(cmd, "agy")
}

// findACPAgentProcess resolves the running process for an agent. Presets use the
// historical matchers; custom agents fall back to their configured binary path.
func findACPAgentProcess(procs []process.ProcEntry, agent acpmesh.Resolved) (bool, int) {
	best := 0
	for _, p := range procs {
		matched := false
		if agent.Custom {
			matched = matchCustomAgent(agent, p)
		} else {
			matched = matchACPAgent(agent.ID, p)
		}
		if !matched {
			continue
		}
		if best == 0 || p.PID < best {
			best = p.PID
		}
	}
	if best > 0 {
		return true, best
	}
	if agent.Custom {
		return false, 0
	}
	return pgrepACPAgent(agent.ID)
}

// ---------------------------------------------------------------------------
// Node presentation
// ---------------------------------------------------------------------------

// buildACPNode turns a resolved agent plus process state into the JSON shape the
// frontend has always consumed (port_socket, ping_latency_ms, ...), extended with
// the custom / os_default / editable flags.
func buildACPNode(agent acpmesh.Resolved, running bool, pid int, hasExplicitEndpoint bool) map[string]interface{} {
	reachable, latency := false, 0.0
	if running || agent.Custom || hasExplicitEndpoint {
		reachable, latency = acpmesh.ProbeEndpoint(agent.Endpoint, 20*time.Millisecond)
	}

	binaryPath := agent.BinaryPath
	if agent.ID == acpmesh.PresetAntigravityACP {
		binaryPath = resolveAntigravityACPPath(binaryPath)
	}

	// Report only what was measured. A detected process with a silent endpoint is
	// installed, not connected, and its preset tool list is not a negotiation.
	status := "not_found"
	var latencyValue interface{}
	version, tools := "", []string{}
	handshake := "never"
	transport := "socket"
	if strings.TrimSpace(agent.Endpoint) == "" {
		// stdio servers are started by their client, so the binary is the state.
		transport = "stdio"
		if _, statErr := os.Stat(expandHome(binaryPath)); statErr == nil {
			status = "installed_idle"
		}
	}
	switch {
	case reachable:
		status = "connected"
		if agent.StatusWhenRunning == "active_hosting" {
			status = "active_hosting"
		}
		latencyValue = latency
		version = agent.Version
		tools = agent.Tools
		handshake = "just now"
	case running:
		status = "installed_idle"
	}
	if !reachable && (agent.Custom || agent.Declared) {
		// A user typed this metadata themselves; echoing it is not a fake handshake,
		// while a preset's invented values would be.
		version, tools = agent.Version, agent.Tools
	}

	return map[string]interface{}{
		"id":              agent.ID,
		"name":            agent.Name,
		"type":            agent.Type,
		"binary_path":     binaryPath,
		"pid":             pid,
		"port_socket":     agent.Endpoint,
		"acp_version":     version,
		"status":          status,
		"ping_latency_ms": latencyValue,
		"supported_tools": tools,
		"last_handshake":  handshake,
		"transport":       transport,
		"custom":          agent.Custom,
		"os_default":      agent.OSDefault,
		"editable":        true,
	}
}

// resolveAntigravityACPPath checks the configured path, then the bundled bin
// directory and the launched bundle's bin directory before PATH.
func resolveAntigravityACPPath(configured string) string {
	if candidate := expandHome(configured); candidate != "" {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}

	names := []string{"antigravity-acp"}
	if runtime.GOOS == "windows" {
		names = append(names, "antigravity-acp.exe")
	}
	var roots []string
	if executable, err := os.Executable(); err == nil {
		roots = append(roots, filepath.Dir(executable))
	}
	if cwd, err := os.Getwd(); err == nil {
		roots = append(roots, cwd)
	}
	for _, root := range roots {
		for _, name := range names {
			candidate := filepath.Join(root, "bin", name)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	if resolved, err := exec.LookPath("antigravity-acp"); err == nil {
		return resolved
	}
	return configured
}

// expandHome resolves ~ and %VAR% so binary presence can be checked on every OS.
func expandHome(path string) string {
	if strings.Contains(path, "%") {
		path = os.Expand(path, func(name string) string { return os.Getenv(name) })
	}
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
}

// buildACPMeshResponse assembles the full registry response for a config.
func buildACPMeshResponse(cfg *core.Config) map[string]interface{} {
	stored := cfg.GetACPAgents()
	explicitEndpoint := make(map[string]bool, len(stored))
	for _, a := range stored {
		if strings.TrimSpace(a.Endpoint) != "" {
			explicitEndpoint[strings.TrimSpace(a.ID)] = true
		}
	}

	resolved := acpmesh.Merge(stored, acpmesh.CurrentEnv())
	procs, _ := process.ListProcesses()

	nodes := make([]map[string]interface{}, 0, len(resolved))
	for _, agent := range resolved {
		running, pid := findACPAgentProcess(procs, agent)
		nodes = append(nodes, buildACPNode(agent, running, pid, explicitEndpoint[agent.ID]))
	}

	return map[string]interface{}{
		"status":           "online",
		"mesh_nodes":       len(nodes),
		"protocol_version": "v1.2.0-draft",
		"agents":           nodes,
	}
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (s *Server) handleUtilitiesACP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cfg, err := core.LoadConfig()
	if err != nil {
		writeACPError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := buildACPMeshResponse(cfg)

	requestedAgent := r.URL.Query().Get("agent")
	if requestedAgent == "" {
		requestedAgent = r.URL.Query().Get("id")
	}
	if requestedAgent == "agent-windsurf" {
		requestedAgent = acpmesh.PresetDevin
	}
	if requestedAgent == "agent-antigravity" || requestedAgent == "antigravity" {
		requestedAgent = acpmesh.PresetAntigravityACP
	}
	if requestedAgent != "" {
		agents, _ := response["agents"].([]map[string]interface{})
		filtered := make([]map[string]interface{}, 0, 1)
		for _, a := range agents {
			if a["id"] == requestedAgent {
				filtered = append(filtered, a)
			}
		}
		response["agents"] = filtered
	}

	writeJSON(w, response)
}

// handleUtilitiesACPAgents implements POST (create/override) and DELETE (remove
// custom agent / reset preset override) for the ACP registry.
func (s *Server) handleUtilitiesACPAgents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleACPAgentSave(w, r)
	case http.MethodDelete:
		s.handleACPAgentDelete(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleACPAgentSave(w http.ResponseWriter, r *http.Request) {
	var input acpmesh.AgentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeACPError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	cfg, err := core.LoadConfig()
	if err != nil {
		writeACPError(w, http.StatusInternalServerError, err.Error())
		return
	}

	stored := cfg.GetACPAgents()
	taken := make(map[string]bool, len(stored)+len(acpmesh.Presets()))
	for _, p := range acpmesh.Presets() {
		taken[p.ID] = true
	}
	for _, a := range stored {
		taken[strings.TrimSpace(a.ID)] = true
	}

	entry, _, err := acpmesh.NormalizeAgentInput(input, taken)
	if err != nil {
		writeACPError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := cfg.SetACPAgents(acpmesh.UpsertAgent(stored, entry)); err != nil {
		writeACPError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, buildACPMeshResponse(cfg))
}

func (s *Server) handleACPAgentDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeACPError(w, http.StatusBadRequest, "query parameter id is required")
		return
	}
	if id == "agent-windsurf" {
		id = acpmesh.PresetDevin
	}
	if id == "agent-antigravity" || id == "antigravity" {
		id = acpmesh.PresetAntigravityACP
	}

	cfg, err := core.LoadConfig()
	if err != nil {
		writeACPError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated, removed := acpmesh.RemoveAgent(cfg.GetACPAgents(), id)
	if !removed {
		writeACPError(w, http.StatusNotFound, "no stored configuration for agent "+id)
		return
	}
	if err := cfg.SetACPAgents(updated); err != nil {
		writeACPError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, buildACPMeshResponse(cfg))
}

// handleUtilitiesACPAgentDetect reports observed endpoints for one agent: the
// configured endpoint, the OS default, TCP ports the running process owns, and
// socket files that exist on disk. Only observed values are returned.
func (s *Server) handleUtilitiesACPAgentDetect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeACPError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		writeACPError(w, http.StatusBadRequest, "id is required")
		return
	}
	if req.ID == "agent-windsurf" {
		req.ID = acpmesh.PresetDevin
	}
	if req.ID == "agent-antigravity" || req.ID == "antigravity" {
		req.ID = acpmesh.PresetAntigravityACP
	}

	cfg, err := core.LoadConfig()
	if err != nil {
		writeACPError(w, http.StatusInternalServerError, err.Error())
		return
	}

	env := acpmesh.CurrentEnv()
	resolved := acpmesh.Merge(cfg.GetACPAgents(), env)

	var agent *acpmesh.Resolved
	for i := range resolved {
		if resolved[i].ID == req.ID {
			agent = &resolved[i]
			break
		}
	}
	if agent == nil {
		writeACPError(w, http.StatusNotFound, "unknown agent "+req.ID)
		return
	}

	osDefault := agent.Endpoint
	if preset, ok := acpmesh.PresetByID(agent.ID); ok {
		osDefault = preset.DefaultEndpoint(env)
	}

	procs, _ := process.ListProcesses()
	running, pid := findACPAgentProcess(procs, *agent)
	if !running {
		pid = 0
	}

	var ports []int
	if pid > 0 {
		ports = acpmesh.DetectListeningPorts(pid)
	}

	var socketPaths []string
	for _, endpoint := range []string{agent.Endpoint, osDefault} {
		if acpmesh.EndpointKind(endpoint) != acpmesh.KindUnix {
			continue
		}
		socketPath := strings.TrimPrefix(endpoint, "unix://")
		if _, err := os.Stat(socketPath); err == nil {
			socketPaths = append(socketPaths, socketPath)
		}
	}

	candidates := acpmesh.BuildCandidates(agent.Endpoint, osDefault, ports, socketPaths)
	for i := range candidates {
		candidates[i].Reachable, _ = acpmesh.ProbeEndpoint(candidates[i].Endpoint, 30*time.Millisecond)
	}

	writeJSON(w, map[string]interface{}{
		"candidates": candidates,
		"os_default": osDefault,
		"pid":        pid,
	})
}

// handleUtilitiesACPPing negotiates with one agent, or the whole registry, using
// the real protocol transport and returns both measured results and updated nodes.
func (s *Server) handleUtilitiesACPPing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeACPError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "agent-windsurf" {
		req.ID = acpmesh.PresetDevin
	}
	if req.ID == "agent-antigravity" || req.ID == "antigravity" {
		req.ID = acpmesh.PresetAntigravityACP
	}

	cfg, err := core.LoadConfig()
	if err != nil {
		writeACPError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resolved := acpmesh.Merge(cfg.GetACPAgents(), acpmesh.CurrentEnv())
	selected := make([]acpmesh.Resolved, 0, len(resolved))
	for _, agent := range resolved {
		if req.ID == "" || agent.ID == req.ID {
			selected = append(selected, agent)
		}
	}
	if req.ID != "" && len(selected) == 0 {
		writeACPError(w, http.StatusNotFound, "unknown agent "+req.ID)
		return
	}

	mesh := buildACPMeshResponse(cfg)
	nodes, _ := mesh["agents"].([]map[string]interface{})
	nodeByID := make(map[string]map[string]interface{}, len(nodes))
	for _, node := range nodes {
		id, _ := node["id"].(string)
		nodeByID[id] = node
	}

	results := make([]acpmesh.HandshakeResult, 0, len(selected))
	logs := make([]AcpHandshakeLog, 0, len(selected))
	onlineCount := 0
	for _, agent := range selected {
		expandedBin := agent.BinaryPath
		node := nodeByID[agent.ID]
		if node != nil {
			if path, ok := node["binary_path"].(string); ok && path != "" {
				expandedBin = path
			}
		}

		result := acpmesh.ProbeAgentHandshake(r.Context(), agent, expandedBin)
		results = append(results, result)
		if result.Reachable {
			onlineCount++
		}

		timestamp := time.Now().UTC().Format(time.RFC3339)
		if node != nil {
			applyHandshakeToNode(node, result, timestamp)
		}
		logs = append(logs, handshakeLog(result, timestamp))
	}

	writeJSON(w, map[string]interface{}{
		"status":       "ok",
		"results":      results,
		"logs":         logs,
		"agents":       nodes,
		"online_count": onlineCount,
	})
}

func applyHandshakeToNode(node map[string]interface{}, result acpmesh.HandshakeResult, timestamp string) {
	node["transport"] = result.Transport
	if result.Endpoint != "" {
		if current, _ := node["port_socket"].(string); strings.TrimSpace(current) == "" {
			node["port_socket"] = result.Endpoint
		}
	}

	if result.Reachable {
		status := "connected"
		if node["status"] == "active_hosting" && result.Transport == "socket" {
			status = "active_hosting"
		}
		node["status"] = status
		node["ping_latency_ms"] = result.LatencyMs
		node["acp_version"] = result.AgentVersion
		node["supported_tools"] = append([]string{}, result.Tools...)
		node["last_handshake"] = timestamp
		return
	}

	node["ping_latency_ms"] = nil
	node["acp_version"] = ""
	node["supported_tools"] = []string{}
	node["last_handshake"] = "failed " + timestamp
	pid, _ := node["pid"].(int)
	endpoint, _ := node["port_socket"].(string)
	binaryPath, _ := node["binary_path"].(string)
	switch {
	case pid > 0:
		node["status"] = "installed_idle"
	case strings.TrimSpace(endpoint) != "":
		node["status"] = "unreachable"
	case strings.TrimSpace(binaryPath) != "":
		if _, err := os.Stat(expandHome(binaryPath)); err == nil {
			node["status"] = "installed_idle"
		} else {
			node["status"] = "not_found"
		}
	default:
		node["status"] = "not_found"
	}
}

func handshakeLog(result acpmesh.HandshakeResult, timestamp string) AcpHandshakeLog {
	action := "ENDPOINT_PROBE"
	status := "warning"
	switch result.Transport {
	case "stdio-acp":
		action = "ACP_INITIALIZE"
	case "pi-rpc":
		action = "PI_GET_STATE"
	}
	if result.Verified {
		status = "success"
	} else if !result.Reachable {
		status = "error"
	}

	summary := result.Summary
	if result.Error != "" {
		summary = result.Error
	} else if result.Reachable {
		summary = fmt.Sprintf("%s (%.2f ms)", summary, result.LatencyMs)
	}
	return AcpHandshakeLog{
		ID:             fmt.Sprintf("handshake-%d-%s", time.Now().UnixNano(), result.AgentID),
		Timestamp:      timestamp,
		FromAgent:      "Antigravity Swiss Knife",
		ToAgent:        result.AgentName,
		Action:         action,
		PayloadSummary: summary,
		Status:         status,
	}
}

// handleUtilitiesACPPrompt sends one prompt over the agent's supported protocol.
func (s *Server) handleUtilitiesACPPrompt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID     string `json:"id"`
		Prompt string `json:"prompt"`
		Cwd    string `json:"cwd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeACPError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		writeACPError(w, http.StatusBadRequest, "id is required")
		return
	}
	if req.ID == "agent-windsurf" {
		req.ID = acpmesh.PresetDevin
	}
	if req.ID == "agent-antigravity" || req.ID == "antigravity" {
		req.ID = acpmesh.PresetAntigravityACP
	}
	if strings.TrimSpace(req.Prompt) == "" {
		writeACPError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	cfg, err := core.LoadConfig()
	if err != nil {
		writeACPError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resolved := acpmesh.Merge(cfg.GetACPAgents(), acpmesh.CurrentEnv())
	var agent *acpmesh.Resolved
	for i := range resolved {
		if resolved[i].ID == req.ID {
			agent = &resolved[i]
			break
		}
	}
	if agent == nil {
		writeACPError(w, http.StatusNotFound, "unknown agent "+req.ID)
		return
	}

	expandedBin := agent.BinaryPath
	if agent.ID == acpmesh.PresetAntigravityACP {
		expandedBin = resolveAntigravityACPPath(expandedBin)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	result, err := acpmesh.PromptAgent(ctx, *agent, expandedBin, req.Prompt, req.Cwd)
	if err != nil {
		writeACPError(w, http.StatusBadGateway, err.Error())
		return
	}
	response := map[string]interface{}{
		"status":   "ok",
		"agent_id": agent.ID,
	}
	for key, value := range result {
		response[key] = value
	}
	writeJSON(w, response)
}

// handleUtilitiesACPSyncRegistry fetches the official ACP registry and updates preset settings.
func (s *Server) handleUtilitiesACPSyncRegistry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		RegistryURL string `json:"registry_url"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	cfg, err := core.LoadConfig()
	if err != nil {
		writeACPError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	updatedOverrides, count, err := acpmesh.SyncRegistry(ctx, req.RegistryURL, cfg.GetACPAgents())
	if err != nil {
		writeACPError(w, http.StatusBadGateway, "failed to sync registry: "+err.Error())
		return
	}

	if err := cfg.SetACPAgents(updatedOverrides); err != nil {
		writeACPError(w, http.StatusInternalServerError, "failed to save updated agents: "+err.Error())
		return
	}

	resp := buildACPMeshResponse(cfg)
	resp["synced"] = count
	if count == 0 {
		resp["summary"] = "ACP preset settings are already up to date with official registry"
	} else {
		resp["summary"] = fmt.Sprintf("Successfully synced %d preset ACP settings from official registry", count)
	}
	writeJSON(w, resp)
}
