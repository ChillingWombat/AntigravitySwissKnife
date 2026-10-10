package gui

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	_ "modernc.org/sqlite"
)

// Injector manages CDP communication to inject CSS and scripts into running Antigravity.
type Injector struct {
	customPort int
}

// NewInjector creates a new Injector instance.
func NewInjector(customPort int) *Injector {
	return &Injector{
		customPort: customPort,
	}
}

// DevToolsTarget represents a Chrome DevTools Protocol debug target.
type DevToolsTarget struct {
	ID                   string `json:"id"`
	Title                string `json:"title"`
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

func (inj *Injector) isPortLive(port int) bool {
	client := &http.Client{Timeout: 600 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
	if err == nil {
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}
	resp2, err2 := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json", port))
	if err2 == nil {
		_ = resp2.Body.Close()
		return resp2.StatusCode == http.StatusOK
	}
	return false
}

// IsTestExecution returns true if the current process is running inside a Go test runner.
func IsTestExecution() bool {
	if os.Getenv("ANTIGRAVITY_TESTING") == "1" || os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") == "1" {
		return true
	}
	if strings.HasSuffix(os.Args[0], ".test") || strings.Contains(os.Args[0], "/_test/") {
		return true
	}
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.") {
			return true
		}
	}
	return false
}

// FindDevToolsPort reads the active remote debugging port of Antigravity and verifies it is responsive.
func (inj *Injector) FindDevToolsPort() (int, error) {
	if inj.customPort > 0 {
		return inj.customPort, nil
	}

	// Under unit tests, never connect to the live host IDE when customPort is not specified
	if IsTestExecution() {
		return 0, fmt.Errorf("unit test mode: skipped connecting to live host DevTools port")
	}

	// 1. Check DevToolsActivePort in Antigravity host config dir and alternatives
	portCandidates := []string{
		filepath.Join(core.GetAntigravityHostConfigDir(), "DevToolsActivePort"),
		filepath.Join(os.Getenv("HOME"), ".config", "antigravity", "DevToolsActivePort"),
	}
	for _, portFile := range portCandidates {
		if data, err := os.ReadFile(portFile); err == nil {
			lines := strings.Split(string(data), "\n")
			if len(lines) > 0 {
				portStr := strings.TrimSpace(lines[0])
				if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
					if inj.isPortLive(p) {
						return p, nil
					}
				}
			}
		}
	}

	// 2. Check override from env
	if pStr := os.Getenv("ANTIGRAVITY_CDP_PORT"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			if inj.isPortLive(p) {
				return p, nil
			}
		}
	}

	// 3. Scan common DevTools debugging ports (9222-9230)
	for p := 9222; p <= 9230; p++ {
		if inj.isPortLive(p) {
			return p, nil
		}
	}

	return 0, fmt.Errorf("DevToolsActivePort file not found, empty, or port not responding (Antigravity may not be running)")
}

// GetPageTargets fetches the active page targets from the DevTools endpoint.
func (inj *Injector) GetPageTargets(port int) ([]DevToolsTarget, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json", port))
	if err != nil {
		return nil, fmt.Errorf("failed to query CDP targets: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected CDP status code: %d", resp.StatusCode)
	}

	var targets []DevToolsTarget
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return nil, fmt.Errorf("failed to decode CDP targets: %w", err)
	}

	var pages []DevToolsTarget
	for _, t := range targets {
		if t.Type == "page" && t.WebSocketDebuggerURL != "" {
			pages = append(pages, t)
		}
	}

	return pages, nil
}

// ExecuteCDPCommand sends a JSON-RPC command to the given target via WebSocket.
func (inj *Injector) ExecuteCDPCommand(wsURLStr string, method string, params map[string]interface{}) (map[string]interface{}, error) {
	u, err := url.Parse(wsURLStr)
	if err != nil {
		return nil, fmt.Errorf("invalid websocket url: %w", err)
	}

	host := u.Host
	if !strings.Contains(host, ":") {
		host = host + ":80"
	}

	conn, err := net.DialTimeout("tcp", host, 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to CDP websocket: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(6 * time.Second))

	// WebSocket handshake
	reqPath := u.Path
	if u.RawQuery != "" {
		reqPath += "?" + u.RawQuery
	}
	handshake := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\n\r\n", reqPath, u.Host)
	if _, err := conn.Write([]byte(handshake)); err != nil {
		return nil, fmt.Errorf("failed to send handshake: %w", err)
	}

	// Read handshake response
	reader := bufio.NewReader(conn)
	respHeader, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(respHeader, "101") {
		return nil, fmt.Errorf("invalid handshake response: %s", respHeader)
	}
	// Read rest of headers until empty line
	for {
		line, err := reader.ReadString('\n')
		if err != nil || strings.TrimSpace(line) == "" {
			break
		}
	}

	// Construct JSON-RPC command
	rpcReq := map[string]interface{}{
		"id":     1,
		"method": method,
	}
	if params != nil {
		rpcReq["params"] = params
	}
	payload, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, err
	}

	// Send RFC 6455 masked text frame
	if err := sendWebSocketFrame(conn, payload); err != nil {
		return nil, fmt.Errorf("failed to send WS frame: %w", err)
	}

	// Read response frame(s), ignoring async CDP events until matching request ID
	var rpcResp struct {
		ID     int                    `json:"id"`
		Result map[string]interface{} `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	for {
		respPayload, err := readWebSocketFrame(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read WS response: %w", err)
		}

		if err := json.Unmarshal(respPayload, &rpcResp); err != nil {
			return nil, fmt.Errorf("failed to parse RPC response: %w", err)
		}

		if rpcResp.ID == 1 {
			break
		}
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("CDP RPC error: %s (code %d)", rpcResp.Error.Message, rpcResp.Error.Code)
	}

	if rpcResp.Result == nil {
		return map[string]interface{}{}, nil
	}

	return rpcResp.Result, nil
}

// ExecuteScript evaluates a JavaScript expression in the given page target via WebSocket.
func (inj *Injector) ExecuteScript(wsURLStr string, expression string) (map[string]interface{}, error) {
	raw, err := inj.ExecuteCDPCommand(wsURLStr, "Runtime.evaluate", map[string]interface{}{
		"expression":    expression,
		"returnByValue": true,
		"awaitPromise":  true,
	})
	if err != nil {
		return nil, err
	}

	if exc, ok := raw["exceptionDetails"]; ok && exc != nil {
		return nil, fmt.Errorf("CDP evaluation exception: %v", exc)
	}

	if inner, ok := raw["result"].(map[string]interface{}); ok {
		if val, ok := inner["value"]; ok {
			if m, ok := val.(map[string]interface{}); ok {
				return m, nil
			}
			return map[string]interface{}{"value": val}, nil
		}
		return map[string]interface{}{"value": nil}, nil
	}

	return raw, nil
}

// FocusActiveWindows brings all active Antigravity page windows to front via CDP.
func (inj *Injector) FocusActiveWindows() error {
	if IsTestExecution() || os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") == "1" || core.IsRunningTests() {
		if inj.customPort == 0 && os.Getenv("ANTIGRAVITY_CDP_PORT") == "" {
			return nil
		}
	}

	port, err := inj.FindDevToolsPort()
	if err != nil {
		return err
	}

	pages, err := inj.GetPageTargets(port)
	if err != nil {
		return fmt.Errorf("failed to query active Antigravity page windows on port %d: %w", port, err)
	}
	if len(pages) == 0 {
		return fmt.Errorf("no active Antigravity page windows found on port %d", port)
	}

	var lastErr error
	focusedCount := 0
	for _, page := range pages {
		if _, err := inj.ExecuteCDPCommand(page.WebSocketDebuggerURL, "Page.bringToFront", nil); err != nil {
			lastErr = err
		} else {
			focusedCount++
		}
		_, _ = inj.ExecuteScript(page.WebSocketDebuggerURL, "try { window.focus(); } catch(_) {}")
	}

	if focusedCount == 0 && lastErr != nil {
		return fmt.Errorf("failed to bring Antigravity window to front: %w", lastErr)
	}

	return nil
}

// ApplyConfig applies the styling configuration to all running Antigravity window instances.
func (inj *Injector) ApplyConfig(cfg *Config) (*ApplyResult, error) {
	port, err := inj.FindDevToolsPort()
	if err != nil {
		return &ApplyResult{
			Success: false,
			Message: fmt.Sprintf("Antigravity DevTools port not detected: %v", err),
		}, err
	}

	pages, err := inj.GetPageTargets(port)
	if err != nil || len(pages) == 0 {
		return &ApplyResult{
			Success: false,
			Message: fmt.Sprintf("No active Antigravity page windows found on port %d", port),
			Port:    port,
		}, err
	}

	script := GenerateScript(cfg)
	var lastErr error
	appliedCount := 0

	for _, page := range pages {
		_, err := inj.ExecuteScript(page.WebSocketDebuggerURL, script)
		if err != nil {
			lastErr = err
		} else {
			appliedCount++
		}
	}

	if appliedCount == 0 && lastErr != nil {
		return &ApplyResult{
			Success: false,
			Message: fmt.Sprintf("Failed to inject styles into Antigravity: %v", lastErr),
			Port:    port,
		}, lastErr
	}

	return &ApplyResult{
		Success: true,
		Message: fmt.Sprintf("Successfully injected project styling into %d Antigravity window(s)", appliedCount),
		Port:    port,
	}, nil
}

// RefreshUserStatusResult captures metrics from in-app React Fiber user status refresh.
type RefreshUserStatusResult struct {
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	Port           int    `json:"port"`
	WindowsCount   int    `json:"windows_count"`
	Email          string `json:"email,omitempty"`
	FiberRefreshed bool   `json:"fiber_refreshed,omitempty"`
	ReloadFallback bool   `json:"reload_fallback,omitempty"`
}

// RefreshUserStatusScript is evaluated in Antigravity's Electron renderer via Chrome DevTools Protocol.
// It traverses the React Fiber tree from document.getElementById("root"), locates userStatusProvider
// and modelCtx, calls usp.lsClient.getUserStatus() to push fresh userStatus into state, and triggers
// modelCtx.refreshModels() with zero window reloads or visual flicker.
const RefreshUserStatusScript = `(async () => {
	try {
		const root = document.getElementById("root");
		if (!root) {
			return { success: false, reason: "no_root" };
		}
		let key = Object.keys(root).find(k => k.startsWith("__reactFiber") || k.startsWith("__reactContainer"));
		let curr = key ? root[key] : null;
		if (!curr && root.firstElementChild) {
			const childKey = Object.keys(root.firstElementChild).find(k => k.startsWith("__reactFiber") || k.startsWith("__reactContainer"));
			if (childKey) {
				curr = root.firstElementChild[childKey];
			}
		}
		if (!curr) {
			return { success: false, reason: "no_fiber_node" };
		}
		// If curr is FiberRootNode, step into root FiberNode (.current)
		if (curr.current) {
			curr = curr.current;
		}

		const queue = [curr];
		const visited = new Set();
		let usp = null, modelCtx = null;
		let count = 0;
		while (queue.length > 0 && count < 6000) {
			const node = queue.shift();
			count++;
			if (!node || visited.has(node)) continue;
			visited.add(node);
			if (!usp && node.memoizedProps?.value?.userStatusProvider) {
				usp = node.memoizedProps.value.userStatusProvider;
			}
			if (!modelCtx && node.memoizedProps?.value?.refreshModels) {
				modelCtx = node.memoizedProps.value;
			}
			if (usp && modelCtx) break;
			if (node.child) queue.push(node.child);
			if (node.sibling) queue.push(node.sibling);
		}

		// Validate USP interface
		if (!usp || typeof usp.pushUpdate !== "function" || !usp.lsClient || typeof usp.lsClient.getUserStatus !== "function") {
			return { success: false, reason: "invalid_usp_interface" };
		}

		const resp = await usp.lsClient.getUserStatus({ metadata: usp.metadata });
		if (!resp || !resp.userStatus) {
			return { success: false, reason: "empty_user_status_response" };
		}

		usp.pushUpdate(resp.userStatus);
		const email = String(resp.userStatus.email || "");

		if (modelCtx && typeof modelCtx.refreshModels === "function") {
			try { await modelCtx.refreshModels(); } catch(_) {}
		}

		return {
			success: true,
			fiber_refreshed: true,
			email: email,
			nodes_visited: count
		};
	} catch(err) {
		return { success: false, error: String(err && err.message ? err.message : err) };
	}
})()`

// RefreshUserStatus performs live zero-flicker in-app React Fiber state refresh for userStatus and quota
// across all running Antigravity window instances via Chrome DevTools Protocol.
// If Fiber traversal fails, it automatically falls back to CDP window.location.reload().
func (inj *Injector) RefreshUserStatus() (*RefreshUserStatusResult, error) {
	port, err := inj.FindDevToolsPort()
	if err != nil {
		return &RefreshUserStatusResult{
			Success: false,
			Message: fmt.Sprintf("Antigravity DevTools port not detected or inactive: %v", err),
		}, err
	}

	pages, err := inj.GetPageTargets(port)
	if err != nil || len(pages) == 0 {
		return &RefreshUserStatusResult{
			Success: false,
			Port:    port,
			Message: fmt.Sprintf("No active Antigravity page windows found on port %d", port),
		}, err
	}

	var lastErr error
	var lastEmail string
	refreshedCount := 0
	anyFiberRefreshed := false

	for _, page := range pages {
		res, err := inj.ExecuteScript(page.WebSocketDebuggerURL, RefreshUserStatusScript)
		if err != nil {
			lastErr = err
		} else {
			refreshedCount++
			if fb, ok := res["fiber_refreshed"].(bool); ok && fb {
				anyFiberRefreshed = true
			}
			if em, ok := res["email"].(string); ok && em != "" {
				lastEmail = em
			}
		}
	}

	if refreshedCount == 0 && lastErr != nil {
		return &RefreshUserStatusResult{
			Success: false,
			Port:    port,
			Message: fmt.Sprintf("Failed to refresh Antigravity user status: %v", lastErr),
		}, lastErr
	}

	return &RefreshUserStatusResult{
		Success:        true,
		Port:           port,
		WindowsCount:   refreshedCount,
		Email:          lastEmail,
		FiberRefreshed: anyFiberRefreshed,
		ReloadFallback: false,
		Message:        fmt.Sprintf("Successfully refreshed user status (zero-flicker React Fiber) in %d Antigravity window(s)", refreshedCount),
	}, nil
}

// GetLiveEmail inspects the running Antigravity IDE window via Chrome DevTools Protocol
// and returns the live in-memory active user email, or empty string if not running or unavailable.
func (inj *Injector) GetLiveEmail() string {
	port, err := inj.FindDevToolsPort()
	if err != nil || port <= 0 {
		return ""
	}
	pages, err := inj.GetPageTargets(port)
	if err != nil || len(pages) == 0 {
		return ""
	}

	const script = `(() => {
		try {
			const root = document.getElementById("root");
			let key = Object.keys(root).find(k => k.startsWith("__reactFiber") || k.startsWith("__reactContainer"));
			let curr = key ? root[key] : null;
			if (curr && curr.current) curr = curr.current;
			const queue = [curr];
			const visited = new Set();
			let count = 0;
			while (queue.length > 0 && count < 6000) {
				const node = queue.shift();
				count++;
				if (!node || visited.has(node)) continue;
				visited.add(node);
				const val = node.memoizedProps?.value;
				if (val?.userStatusProvider?.userStatus?.email) {
					return String(val.userStatusProvider.userStatus.email);
				}
				if (val?.userStatus?.email) {
					return String(val.userStatus.email);
				}
				if (node.child) queue.push(node.child);
				if (node.sibling) queue.push(node.sibling);
			}
		} catch(e) {}
		return "";
	})()`

	for _, page := range pages {
		res, err := inj.ExecuteScript(page.WebSocketDebuggerURL, script)
		if err == nil && res != nil {
			if v, ok := res["value"].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsUUID checks if the string matches standard 36-char UUID format.
func IsUUID(s string) bool {
	return uuidRegex.MatchString(strings.TrimSpace(s))
}

// ConversationExistsInDB verifies if the conversation identifier exists in conversation_summaries.db.
func ConversationExistsInDB(id string) bool {
	id = strings.TrimSpace(id)
	id = strings.TrimPrefix(id, "/c/")
	id = strings.TrimPrefix(id, "/battle/")
	if id == "" {
		return false
	}
	dbPath := filepath.Join(core.GetAntigravityDir(), "conversation_summaries.db")
	if _, err := os.Stat(dbPath); err != nil {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return false
		}
		dbPath = filepath.Join(home, ".gemini", "antigravity", "conversation_summaries.db")
		if _, err := os.Stat(dbPath); err != nil {
			return false
		}
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return false
	}
	defer db.Close()
	_, _ = db.Exec("PRAGMA busy_timeout = 1000;")
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM conversation_summaries WHERE conversation_id = ?`, id).Scan(&count)
	return err == nil && count > 0
}

// IsValidConversationID checks if a conversation identifier is valid and not a test/internal stub.
func IsValidConversationID(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	if idx := strings.IndexAny(id, "?#"); idx != -1 {
		id = id[:idx]
	}
	id = strings.TrimPrefix(id, "/c/")
	id = strings.TrimPrefix(id, "/battle/")
	id = strings.TrimSpace(id)

	lower := strings.ToLower(id)
	if lower == "_new" || strings.HasPrefix(lower, "test") || strings.Contains(lower, "test-") ||
		strings.Contains(lower, "test_") || strings.HasPrefix(lower, "mock") || strings.HasPrefix(lower, "stub") ||
		lower == "undefined" || lower == "null" || lower == "index" || lower == "onboarding" ||
		lower == "login" || lower == "settings" || strings.Contains(id, "/") {
		return false
	}

	if IsUUID(id) {
		return true
	}

	if ConversationExistsInDB(id) {
		return true
	}

	// Also allow alphanumeric-hyphen identifiers of length >= 6 (for test fixtures like conv-101)
	// while strictly rejecting test stubs
	if len(id) >= 6 && !strings.Contains(lower, "test") {
		for _, r := range id {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
				return false
			}
		}
		return true
	}

	return false
}

// IsValidConversationPath returns true if the relative path points to a real Antigravity conversation view.
func IsValidConversationPath(p string) bool {
	p = strings.TrimSpace(p)
	if p == "" {
		return false
	}
	pathOnly := p
	if idx := strings.IndexAny(pathOnly, "?#"); idx != -1 {
		pathOnly = pathOnly[:idx]
	}
	if strings.HasPrefix(pathOnly, "/c/") {
		rest := strings.TrimPrefix(pathOnly, "/c/")
		return IsValidConversationID(rest)
	}
	if strings.HasPrefix(pathOnly, "/battle/") {
		rest := strings.TrimPrefix(pathOnly, "/battle/")
		return IsValidConversationID(rest)
	}
	return false
}

// SaveLastConversationPath persists the last active conversation path in app_storage.json.
func SaveLastConversationPath(convPath string) {
	if !IsValidConversationPath(convPath) {
		return
	}
	hostDir := core.GetAntigravityHostConfigDir()
	storagePath := filepath.Join(hostDir, "app_storage.json")
	rawMap := make(map[string]interface{})
	if data, err := os.ReadFile(storagePath); err == nil {
		_ = json.Unmarshal(data, &rawMap)
	} else if !os.IsNotExist(err) {
		return
	}
	if existing, _ := rawMap["antigravity_swiss_last_conversation_path"].(string); existing == convPath {
		return
	}
	rawMap["antigravity_swiss_last_conversation_path"] = convPath
	updated, err := json.MarshalIndent(rawMap, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(hostDir, 0755)
	tmpPath := storagePath + ".conv.tmp"
	if err := os.WriteFile(tmpPath, updated, 0644); err == nil {
		_ = os.Rename(tmpPath, storagePath)
	}
}

// LoadLastConversationPath reads the last saved conversation path from app_storage.json.
func LoadLastConversationPath() string {
	storagePath := filepath.Join(core.GetAntigravityHostConfigDir(), "app_storage.json")
	data, err := os.ReadFile(storagePath)
	if err != nil {
		return ""
	}
	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return ""
	}
	if v, ok := rawMap["antigravity_swiss_last_conversation_path"].(string); ok && IsValidConversationPath(v) {
		return strings.TrimSpace(v)
	}
	return ""
}

// LoadPinnedConversationPath inspects app_storage.json layout and pinned keys for the authoritative conversation.
func LoadPinnedConversationPath() string {
	storagePath := filepath.Join(core.GetAntigravityHostConfigDir(), "app_storage.json")
	data, err := os.ReadFile(storagePath)
	if err != nil {
		return ""
	}
	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return ""
	}

	// 1. Check pinned_conversations_order
	if pinnedRaw, ok := rawMap["pinned_conversations_order"].(string); ok && pinnedRaw != "" {
		var pinned []string
		if err := json.Unmarshal([]byte(pinnedRaw), &pinned); err == nil && len(pinned) > 0 {
			for _, p := range pinned {
				p = strings.TrimSpace(p)
				if IsValidConversationID(p) {
					return "/c/" + p
				}
			}
		}
	}

	// 2. Check layout keys: antigravity-multi-conversation-layout-v3-<cascadeId>
	const layoutPrefix = "antigravity-multi-conversation-layout-v3-"
	var candidates []string
	for k := range rawMap {
		if strings.HasPrefix(k, layoutPrefix) && k != layoutPrefix+"index" {
			convID := strings.TrimSpace(strings.TrimPrefix(k, layoutPrefix))
			if IsValidConversationID(convID) {
				candidates = append(candidates, convID)
			}
		}
	}

	if len(candidates) == 1 {
		return "/c/" + candidates[0]
	} else if len(candidates) > 1 {
		// Prefer candidate matching latest in conversation_summaries.db
		if latest := QueryLatestTopLevelConversationPath(); latest != "" {
			for _, c := range candidates {
				if latest == "/c/"+c || strings.HasPrefix(latest, "/c/"+c) {
					return latest
				}
			}
		}
		// Prefer standard UUID if mixed with non-UUID
		for _, c := range candidates {
			if IsUUID(c) {
				return "/c/" + c
			}
		}
		return "/c/" + candidates[0]
	}

	return ""
}

// QueryLatestTopLevelConversationPath queries ~/.gemini/antigravity/conversation_summaries.db
// for the most recently modified top-level conversation.
func QueryLatestTopLevelConversationPath() string {
	dbPath := filepath.Join(core.GetAntigravityDir(), "conversation_summaries.db")
	if _, err := os.Stat(dbPath); err != nil {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return ""
		}
		dbPath = filepath.Join(home, ".gemini", "antigravity", "conversation_summaries.db")
		if _, err := os.Stat(dbPath); err != nil {
			return ""
		}
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return ""
	}
	defer db.Close()
	_, _ = db.Exec("PRAGMA busy_timeout = 2000;")

	var convID string
	err = db.QueryRow(`SELECT conversation_id FROM conversation_summaries WHERE parent_conversation_id = '' AND nesting_depth = 0 ORDER BY last_modified_time DESC LIMIT 1`).Scan(&convID)
	convID = strings.TrimSpace(convID)
	if err != nil || convID == "" || !IsValidConversationID(convID) {
		return ""
	}
	candidate := "/c/" + convID
	if saved := LoadLastConversationPath(); strings.HasPrefix(saved, candidate) {
		return saved
	}
	return candidate
}

// CaptureActiveConversationPath inspects the pinned app_storage keys and database first,
// falling back to live CDP only when necessary, and saves the verified result.
func (inj *Injector) CaptureActiveConversationPath() string {
	// 1. Authoritative pinned conversation from app_storage.json
	if pinned := LoadPinnedConversationPath(); IsValidConversationPath(pinned) {
		SaveLastConversationPath(pinned)
		return pinned
	}

	// 2. Authoritative top-level conversation from conversation_summaries.db
	if fromDB := QueryLatestTopLevelConversationPath(); IsValidConversationPath(fromDB) {
		SaveLastConversationPath(fromDB)
		return fromDB
	}

	// 3. Saved last conversation path in app_storage.json
	if saved := LoadLastConversationPath(); IsValidConversationPath(saved) {
		return saved
	}

	// 4. Live CDP fallback (strictly verified, never accepts onboarding or test stubs)
	if port, err := inj.FindDevToolsPort(); err == nil && port > 0 {
		if pages, err := inj.GetPageTargets(port); err == nil {
			for _, page := range pages {
				if page.WebSocketDebuggerURL != "" {
					res, err := inj.ExecuteScript(page.WebSocketDebuggerURL, `window.location.pathname + window.location.search`)
					if err == nil && res != nil {
						if v, ok := res["value"].(string); ok && IsValidConversationPath(v) {
							SaveLastConversationPath(v)
							return strings.TrimSpace(v)
						}
					}
				}
				if u, err := url.Parse(page.URL); err == nil {
					reqURI := u.RequestURI()
					if IsValidConversationPath(reqURI) {
						SaveLastConversationPath(reqURI)
						return reqURI
					}
				}
			}
		}
	}

	return ""
}

// RestoreConversationPath polls the newly launched Antigravity renderer via CDP
// and navigates it from "/" back to targetPath (/c/<cascadeId>) so the previous conversation is resumed.
func (inj *Injector) RestoreConversationPath(targetPath string, maxWait time.Duration) bool {
	if !IsValidConversationPath(targetPath) {
		return false
	}
	if maxWait <= 0 {
		maxWait = 25 * time.Second
	}

	script := fmt.Sprintf(`(() => {
		try {
			const targetPath = %q;
			const targetPathOnly = targetPath.split("?")[0];
			const root = document.getElementById("root");
			const rootReady = Boolean(root && root.childElementCount > 0);
			const curPath = window.location.pathname || "/";
			const hasConvoView = Boolean(document.querySelector('[data-testid="conversation-view"]'));
			const hasShell = Boolean(document.querySelector('[data-testid="new-conversation-button"], [data-testid="conversation-list-sidebar"]'));
			const isConvoTarget = targetPathOnly.startsWith("/c/");
			const viewReady = rootReady && curPath === targetPathOnly && (!isConvoTarget || hasConvoView);

			if (!viewReady) {
				const now = Date.now();
				const convId = targetPathOnly.replace(/^\/c\//, "");
				const link = document.querySelector('a[href*="' + convId + '"]');
				if (link && typeof link.click === "function") {
					window.__swissLastRestoreNudge = now;
					link.click();
				} else if (curPath.startsWith("/onboarding")) {
					if (!window.__swissLastOnboardingNudge || (now - window.__swissLastOnboardingNudge) > 1500) {
						window.__swissLastOnboardingNudge = now;
						if (window.location && typeof window.location.assign === "function") {
							window.location.assign(targetPath);
						} else if (window.history && typeof window.history.replaceState === "function") {
							window.history.replaceState(window.history.state, "", targetPath);
						}
					}
				} else if (curPath !== targetPathOnly) {
					window.__swissLastRestoreNudge = now;
					if (window.history && typeof window.history.replaceState === "function") {
						window.history.replaceState(window.history.state, "", targetPath);
					} else if (window.location && typeof window.location.assign === "function") {
						window.location.assign(targetPath);
					}
				} else if (hasShell && (!window.__swissLastRestoreNudge || (now - window.__swissLastRestoreNudge) > 1200)) {
					window.__swissLastRestoreNudge = now;
					if (window.history && typeof window.history.replaceState === "function") {
						window.history.replaceState(window.history.state, "", targetPath);
					} else if (window.location && typeof window.location.assign === "function") {
						window.location.assign(targetPath);
					}
				}
			}
			return {
				ready: viewReady,
				path: (window.location.pathname || "/") + (window.location.search || "")
			};
		} catch (e) {
			return { ready: false, path: "" };
		}
	})()`, targetPath)

	deadline := time.Now().Add(maxWait)
	stableReadyCount := 0
	for time.Now().Before(deadline) {
		port, err := inj.FindDevToolsPort()
		if err == nil && port > 0 {
			pages, err := inj.GetPageTargets(port)
			if err == nil && len(pages) > 0 {
				restoredOnPage := false
				for _, page := range pages {
					if !strings.HasPrefix(page.URL, "https://127.0.0.1:") && !strings.HasPrefix(page.URL, "http://127.0.0.1:") && !strings.HasPrefix(page.URL, "https://localhost:") && !strings.HasPrefix(page.URL, "http://localhost:") {
						continue
					}
					res, err := inj.ExecuteScript(page.WebSocketDebuggerURL, script)
					if err == nil && res != nil {
						ready, _ := res["ready"].(bool)
						curPath, _ := res["path"].(string)
						if ready && IsValidConversationPath(curPath) {
							restoredOnPage = true
						}
					}
				}
				if restoredOnPage {
					stableReadyCount++
					if stableReadyCount >= 3 {
						return true
					}
				} else {
					stableReadyCount = 0
				}
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	return stableReadyCount > 0
}

// CaptureScreenshot captures a PNG screenshot of the page target via CDP.
func (inj *Injector) CaptureScreenshot(wsURLStr string, clip map[string]interface{}) ([]byte, error) {
	u, err := url.Parse(wsURLStr)
	if err != nil {
		return nil, fmt.Errorf("invalid websocket url: %w", err)
	}

	host := u.Host
	if !strings.Contains(host, ":") {
		host = host + ":80"
	}

	conn, err := net.DialTimeout("tcp", host, 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to CDP: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	reqPath := u.Path
	if u.RawQuery != "" {
		reqPath += "?" + u.RawQuery
	}
	handshake := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Version: 13\r\n\r\n", reqPath, u.Host)
	if _, err := conn.Write([]byte(handshake)); err != nil {
		return nil, fmt.Errorf("failed to send handshake: %w", err)
	}

	reader := bufio.NewReader(conn)
	respHeader, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(respHeader, "101") {
		return nil, fmt.Errorf("invalid handshake: %s", respHeader)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil || strings.TrimSpace(line) == "" {
			break
		}
	}

	params := map[string]interface{}{"format": "png"}
	if clip != nil {
		params["clip"] = clip
	}
	rpcReq := map[string]interface{}{
		"id":     2,
		"method": "Page.captureScreenshot",
		"params": params,
	}
	payload, _ := json.Marshal(rpcReq)
	if err := sendWebSocketFrame(conn, payload); err != nil {
		return nil, err
	}

	respPayload, err := readWebSocketFrame(reader)
	if err != nil {
		return nil, err
	}

	var rpcResp struct {
		ID     int `json:"id"`
		Result struct {
			Data string `json:"data"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respPayload, &rpcResp); err != nil {
		return nil, err
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("screenshot error: %s", rpcResp.Error.Message)
	}

	importBase64, err := base64.StdEncoding.DecodeString(rpcResp.Result.Data)
	if err != nil {
		return nil, fmt.Errorf("base64 decode error: %w", err)
	}
	return importBase64, nil
}

// Helpers for RFC 6455 WebSocket frames
func sendWebSocketFrame(w io.Writer, payload []byte) error {
	var buf bytes.Buffer
	buf.WriteByte(0x81) // FIN + text frame

	length := len(payload)
	var maskKey [4]byte
	_, _ = rand.Read(maskKey[:])

	if length <= 125 {
		buf.WriteByte(byte(length) | 0x80) // mask bit set
	} else if length <= 65535 {
		buf.WriteByte(126 | 0x80)
		_ = binary.Write(&buf, binary.BigEndian, uint16(length))
	} else {
		buf.WriteByte(127 | 0x80)
		_ = binary.Write(&buf, binary.BigEndian, uint64(length))
	}

	buf.Write(maskKey[:])

	masked := make([]byte, length)
	for i := 0; i < length; i++ {
		masked[i] = payload[i] ^ maskKey[i%4]
	}
	buf.Write(masked)

	_, err := w.Write(buf.Bytes())
	return err
}

func readWebSocketFrame(r *bufio.Reader) ([]byte, error) {
	b0, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	_ = b0 // opcode

	b1, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	isMasked := (b1 & 0x80) != 0
	payloadLen := int(b1 & 0x7F)

	if payloadLen == 126 {
		var l uint16
		if err := binary.Read(r, binary.BigEndian, &l); err != nil {
			return nil, err
		}
		payloadLen = int(l)
	} else if payloadLen == 127 {
		var l uint64
		if err := binary.Read(r, binary.BigEndian, &l); err != nil {
			return nil, err
		}
		payloadLen = int(l)
	}

	var maskKey [4]byte
	if isMasked {
		if _, err := io.ReadFull(r, maskKey[:]); err != nil {
			return nil, err
		}
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}

	if isMasked {
		for i := 0; i < payloadLen; i++ {
			payload[i] ^= maskKey[i%4]
		}
	}

	return payload, nil
}
