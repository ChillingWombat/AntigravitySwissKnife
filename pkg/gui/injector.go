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

// FindDevToolsPort reads the active remote debugging port of Antigravity.
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
					return p, nil
				}
		}
	}

	// 2. Check override from env
	if pStr := os.Getenv("ANTIGRAVITY_CDP_PORT"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			return p, nil
		}
	}

	return 0, fmt.Errorf("DevToolsActivePort file not found or empty (Antigravity may not be running)")
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
