package webgui

import (
	"encoding/json"
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
	case acpmesh.PresetAntigravity:
		return (commLower == "antigravity" ||
			strings.Contains(cmdLower, "/opt/antigravity/antigravity") ||
			strings.Contains(cmdLower, "/antigravity.app/") ||
			strings.HasSuffix(commRaw, "antigravity.exe")) &&
			!strings.Contains(cmdLower, "--type=") &&
			!strings.Contains(cmdLower, "swiss") &&
			!strings.Contains(cmdLower, "agy") &&
			commLower != "agy"
	case acpmesh.PresetAntigravityCLI:
		if !(commLower == "agy" || commLower == "agy-run" || commLower == "agy_acp_server" ||
			strings.Contains(cmdLower, "/bin/agy") || strings.Contains(cmdLower, "agy_acp_server") ||
			strings.Contains(cmdLower, "bin/agy --hub")) {
			return false
		}
		return !strings.Contains(cmdLower, "swiss") && !strings.Contains(cmdLower, "antigravity-swiss-knife")
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
		acpmesh.PresetAntigravity:    {"antigravity"},
		acpmesh.PresetAntigravityCLI: {"agy", "agy-run", "agy_acp_server"},
		acpmesh.PresetDevin:          {"devin", "devin-desktop", "windsurf"},
		acpmesh.PresetOpenCode:       {"opencode"},
		acpmesh.PresetDeepSeek:       {"dsh", "deepseek-harness"},
		acpmesh.PresetPi:             {"pi", "pi-acp"},
		acpmesh.PresetCodex:          {"codex"},
		acpmesh.PresetClaudeCode:     {"claude"},
		acpmesh.PresetCursor:         {"cursor"},
	}
	patterns := map[string][]string{
		acpmesh.PresetAntigravityCLI: {"/bin/agy", "agy_acp_server"},
		acpmesh.PresetDevin:          {"devin-desktop", "bin/devin", "bin/windsurf"},
		acpmesh.PresetOpenCode:       {"bin/opencode", "opencode acp"},
		acpmesh.PresetDeepSeek:       {"deepseek-harness", "bin/dsh"},
		acpmesh.PresetCodex:          {"bin/codex", "codex app-server"},
		acpmesh.PresetClaudeCode:     {"bin/claude"},
		acpmesh.PresetCursor:         {"/opt/Cursor/cursor"},
	}

	for _, name := range exact[id] {
		for _, pid := range pgrepPIDs("-x", name) {
			if id == acpmesh.PresetAntigravity && !antigravityCmdlineAllowed(pid) {
				continue
			}
			return true, pid
		}
	}
	for _, pattern := range patterns[id] {
		if pids := pgrepPIDs("-f", pattern); len(pids) > 0 {
			return true, pids[0]
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
		if _, statErr := os.Stat(expandHome(agent.BinaryPath)); statErr == nil {
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

	return map[string]interface{}{
		"id":              agent.ID,
		"name":            agent.Name,
		"type":            agent.Type,
		"binary_path":     agent.BinaryPath,
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

// expandHome resolves a leading ~ so binary presence can be checked.
func expandHome(path string) string {
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
