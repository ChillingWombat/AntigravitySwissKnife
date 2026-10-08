package gui

import (
	"bufio"
	"bytes"
	"crypto/rand"
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
	"strconv"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
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

// FindDevToolsPort reads the active remote debugging port of Antigravity and verifies it is responsive.
func (inj *Injector) FindDevToolsPort() (int, error) {
	if inj.customPort > 0 {
		return inj.customPort, nil
	}

	// 1. Check DevToolsActivePort in Antigravity host config dir
	activePortPath := filepath.Join(core.GetAntigravityHostConfigDir(), "DevToolsActivePort")
	if data, err := os.ReadFile(activePortPath); err == nil {
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

	// 2. Check override from env
	if pStr := os.Getenv("ANTIGRAVITY_CDP_PORT"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			if inj.isPortLive(p) {
				return p, nil
			}
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

// ExecuteScript evaluates a JavaScript expression in the given page target via WebSocket.
func (inj *Injector) ExecuteScript(wsURLStr string, expression string) (map[string]interface{}, error) {
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
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

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
		"method": "Runtime.evaluate",
		"params": map[string]interface{}{
			"expression":    expression,
			"returnByValue": true,
			"awaitPromise":  true,
		},
	}
	payload, err := json.Marshal(rpcReq)
	if err != nil {
		return nil, err
	}

	// Send RFC 6455 masked text frame
	if err := sendWebSocketFrame(conn, payload); err != nil {
		return nil, fmt.Errorf("failed to send WS frame: %w", err)
	}

	// Read response frame
	respPayload, err := readWebSocketFrame(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read WS response: %w", err)
	}

	var rpcResp struct {
		ID     int `json:"id"`
		Result struct {
			Result struct {
				Type  string      `json:"type"`
				Value interface{} `json:"value"`
			} `json:"result"`
			ExceptionDetails interface{} `json:"exceptionDetails"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respPayload, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to parse RPC response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("CDP RPC error: %s (code %d)", rpcResp.Error.Message, rpcResp.Error.Code)
	}

	if m, ok := rpcResp.Result.Result.Value.(map[string]interface{}); ok {
		return m, nil
	}
	return map[string]interface{}{"value": rpcResp.Result.Result.Value}, nil
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
