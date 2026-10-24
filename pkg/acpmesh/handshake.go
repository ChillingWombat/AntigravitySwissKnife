package acpmesh

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	handshakeTimeout   = 8 * time.Second
	piHandshakeTimeout = 12 * time.Second
	promptTimeout      = 2 * time.Minute
	TransportStdioACP  = "stdio-acp"
	TransportPiRPC     = "pi-rpc"
	acpProtocol        = "acp-v1"
	piRPCProtocol      = "pi-rpc-v1"

	acpInitializeRequest = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1,"clientCapabilities":{},"clientInfo":{"name":"antigravity-swiss-knife","version":"0.2.0"}}}` + "\n"
)

func probeContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := handshakeTimeout
	if d, ok := ctx.Deadline(); ok {
		if remaining := time.Until(d); remaining > timeout {
			timeout = remaining
		}
	}
	return context.WithTimeout(ctx, timeout)
}

func boundPiHandshakeContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); ok {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, piHandshakeTimeout)
}

var piRPCTools = []string{"read", "bash", "edit", "write", "grep", "find", "ls"}

// HandshakeResult is the measured result of negotiating with an agent. Verified
// is only true when a protocol response was received and parsed.
type HandshakeResult struct {
	AgentID      string   `json:"agent_id"`
	AgentName    string   `json:"agent_name"`
	Transport    string   `json:"transport"`
	Endpoint     string   `json:"endpoint"`
	Reachable    bool     `json:"reachable"`
	Verified     bool     `json:"verified"`
	Protocol     string   `json:"protocol"`
	AgentVersion string   `json:"agent_version,omitempty"`
	Tools        []string `json:"tools,omitempty"`
	LatencyMs    float64  `json:"ping_latency_ms"`
	Summary      string   `json:"summary"`
	Error        string   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error,omitempty"`
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	written := len(p)
	const limit = 16 * 1024
	if b.buf.Len() < limit {
		remaining := limit - b.buf.Len()
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.buf.Write(p)
	}
	return written, nil
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func measuredLatency(start time.Time) float64 {
	latency := float64(time.Since(start).Microseconds()) / 1000.0
	if latency < 0.1 {
		return 0.1
	}
	return latency
}

func newHandshakeResult(agent Resolved, transport string) HandshakeResult {
	return HandshakeResult{
		AgentID:   agent.ID,
		AgentName: agent.Name,
		Transport: transport,
		Endpoint:  strings.TrimSpace(agent.Endpoint),
	}
}

func failHandshake(result *HandshakeResult, err error) HandshakeResult {
	if err == nil {
		err = errors.New("handshake failed")
	}
	result.Reachable = false
	result.Verified = false
	result.Error = err.Error()
	result.Summary = err.Error()
	return *result
}

// ProbeAgentHandshake negotiates with an agent using the real transport that
// agent advertises. A bare socket connect is reported as reachable but not
// verified.
func ProbeAgentHandshake(ctx context.Context, agent Resolved, expandedBin string) HandshakeResult {
	if ctx == nil {
		ctx = context.Background()
	}

	switch {
	case agent.ID == PresetAntigravityACP:
		bin := resolveExecutable(expandedBin, agent.BinaryPath)
		if bin == "" {
			bin = lookPath("antigravity-acp")
		}
		return probeACPBin(ctx, agent, bin, []string{"--surface", "cli"}, TransportStdioACP)
	case agent.ID == PresetPi:
		return probePiHandshake(ctx, agent, expandedBin)
	case strings.TrimSpace(agent.Endpoint) == "":
		bin := resolveExecutable(expandedBin, agent.BinaryPath)
		args := []string{"--surface", "cli"}
		if strings.EqualFold(filepath.Base(bin), "pi-acp") {
			args = nil
		}
		return probeACPBin(ctx, agent, bin, args, TransportStdioACP)
	default:
		return probeSocketHandshake(agent)
	}
}

func probePiHandshake(ctx context.Context, agent Resolved, expandedBin string) HandshakeResult {
	ctx, cancel := boundPiHandshakeContext(ctx)
	defer cancel()

	var piErr error
	acpTarget := expandedBin
	if !isPiACPName(acpTarget) && isPiACPName(agent.BinaryPath) {
		acpTarget = agent.BinaryPath
	}
	acpBin := resolvePiACPExecutable(acpTarget)
	if isPiACPName(expandedBin) || isPiACPName(agent.BinaryPath) {
		if acpBin != "" {
			result := probeACPBin(ctx, agent, acpBin, nil, TransportStdioACP)
			if result.Verified {
				return result
			}
			piErr = errors.New(result.Error)
		}
	}

	piBin := resolvePiExecutable(expandedBin, agent.BinaryPath)
	if piBin != "" {
		result := probePiRPC(ctx, agent, piBin)
		if result.Verified {
			return result
		}
		piErr = errors.New(result.Error)
	}

	if acpBin != "" {
		result := probeACPBin(ctx, agent, acpBin, nil, TransportStdioACP)
		if result.Verified {
			if piErr != nil {
				if result.Summary != "" {
					result.Summary = fmt.Sprintf("%s (pi-rpc fallback: %v)", result.Summary, piErr)
				} else {
					result.Summary = fmt.Sprintf("pi-rpc fallback: %v", piErr)
				}
			}
			return result
		}
		if piErr == nil {
			piErr = errors.New(result.Error)
		}
	}

	if strings.TrimSpace(agent.Endpoint) != "" {
		result := probeSocketHandshake(agent)
		if result.Reachable {
			return result
		}
	}
	if piErr != nil {
		result := newHandshakeResult(agent, TransportPiRPC)
		return failHandshake(&result, fmt.Errorf("Pi handshake failed: %w", piErr))
	}
	result := newHandshakeResult(agent, TransportPiRPC)
	return failHandshake(&result, errors.New("pi, pi-acp and the configured endpoint are unavailable"))
}

func probeACPBin(ctx context.Context, agent Resolved, bin string, args []string, transport string) HandshakeResult {
	result := newHandshakeResult(agent, transport)
	result.Protocol = acpProtocol
	result.Endpoint = bin
	if bin == "" {
		return failHandshake(&result, errors.New("ACP executable not found"))
	}

	probeCtx, cancel := probeContext(ctx)
	defer cancel()

	cmd := exec.CommandContext(probeCtx, bin, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return failHandshake(&result, fmt.Errorf("open ACP stdin: %w", err))
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return failHandshake(&result, fmt.Errorf("open ACP stdout: %w", err))
	}
	stderr := &lockedBuffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return failHandshake(&result, fmt.Errorf("start %s: %w", bin, err))
	}
	defer stopProcess(cmd, stdin)

	start := time.Now()
	if _, err := io.WriteString(stdin, acpInitializeRequest); err != nil {
		return failHandshake(&result, fmt.Errorf("write ACP initialize: %w", err))
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg rpcMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.Method != "" || rpcIDString(msg.ID) != "1" {
			continue
		}
		result.LatencyMs = measuredLatency(start)
		if msg.Error != nil {
			return failHandshake(&result, fmt.Errorf("ACP initialize: %s", rpcErrorMessage(msg.Error)))
		}

		var response struct {
			ProtocolVersion int `json:"protocolVersion"`
			AgentInfo       struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"agentInfo"`
		}
		if err := json.Unmarshal(msg.Result, &response); err != nil {
			return failHandshake(&result, fmt.Errorf("parse ACP initialize result: %w", err))
		}
		if response.AgentInfo.Name != "" {
			result.AgentName = response.AgentInfo.Name
		}
		result.AgentVersion = response.AgentInfo.Version
		result.Reachable = true
		result.Verified = true
		result.Summary = fmt.Sprintf(
			"ACP initialize verified: %s %s",
			firstNonEmpty(result.AgentName, "agent"),
			firstNonEmpty(result.AgentVersion, "version unknown"),
		)
		return result
	}

	if err := scanner.Err(); err != nil {
		return failHandshake(&result, fmt.Errorf("read ACP initialize: %w", err))
	}
	if err := probeCtx.Err(); err != nil {
		return failHandshake(&result, fmt.Errorf("ACP initialize timed out: %w", err))
	}
	detail := strings.TrimSpace(stderr.String())
	if detail != "" {
		return failHandshake(&result, fmt.Errorf("ACP initialize ended without a response: %s", detail))
	}
	return failHandshake(&result, errors.New("ACP initialize ended without a response"))
}

func probePiRPC(ctx context.Context, agent Resolved, bin string) HandshakeResult {
	fastArgs := []string{"--offline", "--no-session", "--no-themes", "--no-extensions", "--no-mcp", "--no-skills", "--mode", "rpc"}
	res := probePiRPCOnce(ctx, agent, bin, fastArgs)
	if res.Verified {
		return res
	}
	if isUnsupportedPiRPCFlagError(res.Error) {
		basicArgs := []string{"--offline", "--no-session", "--no-themes", "--mode", "rpc"}
		return probePiRPCOnce(ctx, agent, bin, basicArgs)
	}
	return res
}

func isUnsupportedPiRPCFlagError(errText string) bool {
	errLower := strings.ToLower(errText)
	for _, marker := range []string{
		"unknown option",
		"unknown flag",
		"unrecognized option",
		"unknown argument",
		"invalid option",
		"flag provided but not defined",
	} {
		if strings.Contains(errLower, marker) {
			return true
		}
	}
	return false
}

func probePiRPCOnce(ctx context.Context, agent Resolved, bin string, args []string) HandshakeResult {
	result := newHandshakeResult(agent, TransportPiRPC)
	result.Protocol = piRPCProtocol
	result.Endpoint = bin

	probeCtx, cancel := probeContext(ctx)
	defer cancel()

	cmd := exec.CommandContext(probeCtx, bin, args...)
	cmd.Env = append(os.Environ(), "PI_OFFLINE=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return failHandshake(&result, fmt.Errorf("open Pi stdin: %w", err))
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return failHandshake(&result, fmt.Errorf("open Pi stdout: %w", err))
	}
	stderr := &lockedBuffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return failHandshake(&result, fmt.Errorf("start %s: %w", bin, err))
	}
	defer stopProcess(cmd, stdin)

	start := time.Now()
	if _, err := io.WriteString(stdin, "{\"id\":\"probe-1\",\"type\":\"get_state\"}\n"); err != nil {
		return failHandshake(&result, fmt.Errorf("write Pi get_state: %w", err))
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var response struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Command string `json:"command"`
			Success bool   `json:"success"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			continue
		}
		if response.Type != "response" || response.ID != "probe-1" || response.Command != "get_state" {
			continue
		}
		result.LatencyMs = measuredLatency(start)
		if !response.Success {
			message := firstNonEmpty(response.Error, "get_state returned success=false")
			return failHandshake(&result, fmt.Errorf("Pi get_state: %s", message))
		}
		result.Reachable = true
		result.Verified = true
		if verOut, verErr := exec.CommandContext(probeCtx, bin, "--version").Output(); verErr == nil {
			if v := strings.TrimSpace(string(verOut)); v != "" {
				result.AgentVersion = v
			}
		}
		result.Tools = append([]string(nil), piRPCTools...)
		result.Summary = "Pi RPC get_state verified"
		return result
	}

	if err := scanner.Err(); err != nil {
		return failHandshake(&result, fmt.Errorf("read Pi get_state: %w", err))
	}
	if err := probeCtx.Err(); err != nil {
		return failHandshake(&result, fmt.Errorf("Pi get_state timed out: %w", err))
	}
	detail := strings.TrimSpace(stderr.String())
	if detail != "" {
		return failHandshake(&result, fmt.Errorf("Pi get_state ended without a response: %s", detail))
	}
	return failHandshake(&result, errors.New("Pi get_state ended without a response"))
}

func probeSocketHandshake(agent Resolved) HandshakeResult {
	result := newHandshakeResult(agent, "socket")
	result.Protocol = "unknown"
	if strings.TrimSpace(agent.Endpoint) == "" {
		return failHandshake(&result, errors.New("agent has no endpoint or stdio executable"))
	}
	reachable, latency := ProbeEndpoint(agent.Endpoint, 250*time.Millisecond)
	result.Reachable = reachable
	result.LatencyMs = latency
	if !reachable {
		return failHandshake(&result, fmt.Errorf("endpoint %s did not answer", agent.Endpoint))
	}
	result.Summary = "Endpoint accepted a connection; protocol not verified"
	return result
}

// PromptAgent sends one prompt to a supported agent transport and returns the
// streamed response plus the actual session identifiers.
func PromptAgent(ctx context.Context, agent Resolved, expandedBin string, prompt string, cwd string) (map[string]interface{}, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, promptTimeout)
		defer cancel()
	}

	workDir, err := resolvePromptCWD(cwd)
	if err != nil {
		return nil, err
	}

	switch {
	case agent.ID == PresetPi:
		if isPiACPName(expandedBin) || isPiACPName(agent.BinaryPath) {
			if acpBin := resolvePiACPExecutable(expandedBin); acpBin != "" {
				return promptACP(ctx, agent, acpBin, nil, prompt, workDir)
			}
		}
		piBin := resolvePiExecutable(expandedBin, agent.BinaryPath)
		if piBin != "" {
			return promptPiRPC(ctx, agent, piBin, prompt, workDir)
		}
		if acpBin := resolvePiACPExecutable(expandedBin); acpBin != "" {
			return promptACP(ctx, agent, acpBin, nil, prompt, workDir)
		}
		return nil, errors.New("pi and pi-acp are unavailable")
	case agent.ID == PresetAntigravityACP:
		bin := resolveExecutable(expandedBin, agent.BinaryPath)
		if bin == "" {
			bin = lookPath("antigravity-acp")
		}
		if bin == "" {
			return nil, errors.New("antigravity-acp executable not found")
		}
		return promptACP(ctx, agent, bin, []string{"--surface", "cli"}, prompt, workDir)
	case strings.TrimSpace(agent.Endpoint) == "":
		bin := resolveExecutable(expandedBin, agent.BinaryPath)
		if bin == "" {
			return nil, errors.New("agent stdio executable not found")
		}
		args := []string{"--surface", "cli"}
		if strings.EqualFold(filepath.Base(bin), "pi-acp") {
			args = nil
		}
		return promptACP(ctx, agent, bin, args, prompt, workDir)
	default:
		return nil, fmt.Errorf("prompt dispatch is not supported for socket agent %s", agent.ID)
	}
}

func promptACP(ctx context.Context, agent Resolved, bin string, args []string, prompt string, cwd string) (map[string]interface{}, error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, bin, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open ACP stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open ACP stdout: %w", err)
	}
	stderr := &lockedBuffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", bin, err)
	}
	defer stopProcess(cmd, stdin)

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)

	if _, err := io.WriteString(stdin, acpInitializeRequest); err != nil {
		return nil, fmt.Errorf("write ACP initialize: %w", err)
	}
	initialize, err := awaitACPResult(scanner, stdin, "1", nil)
	if err != nil {
		return nil, fmt.Errorf("ACP initialize: %w%s", err, stderrSuffix(stderr))
	}
	var initResult struct {
		AgentInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"agentInfo"`
	}
	_ = json.Unmarshal(initialize, &initResult)

	newSession := struct {
		Cwd        string        `json:"cwd"`
		McpServers []interface{} `json:"mcpServers"`
	}{
		Cwd:        cwd,
		McpServers: []interface{}{},
	}
	if err := writeRPCMessage(stdin, 2, "session/new", newSession); err != nil {
		return nil, fmt.Errorf("write ACP session/new: %w", err)
	}
	sessionRaw, err := awaitACPResult(scanner, stdin, "2", nil)
	if err != nil {
		return nil, fmt.Errorf("ACP session/new: %w%s", err, stderrSuffix(stderr))
	}
	var sessionResult struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(sessionRaw, &sessionResult); err != nil {
		return nil, fmt.Errorf("parse ACP session/new result: %w", err)
	}
	if sessionResult.SessionID == "" {
		return nil, errors.New("ACP session/new returned no sessionId")
	}

	var text strings.Builder
	onNotification := func(msg rpcMessage) {
		chunk := acpMessageText(msg)
		if chunk != "" {
			text.WriteString(chunk)
		}
	}
	promptRequest := struct {
		SessionID string `json:"sessionId"`
		Prompt    []struct {
			Text string `json:"text"`
			Type string `json:"type"`
		} `json:"prompt"`
	}{
		SessionID: sessionResult.SessionID,
		Prompt: []struct {
			Text string `json:"text"`
			Type string `json:"type"`
		}{{Text: prompt, Type: "text"}},
	}
	if err := writeRPCMessage(stdin, 3, "session/prompt", promptRequest); err != nil {
		return nil, fmt.Errorf("write ACP session/prompt: %w", err)
	}
	finalRaw, err := awaitACPResult(scanner, stdin, "3", onNotification)
	if err != nil {
		return nil, fmt.Errorf("ACP session/prompt: %w%s", err, stderrSuffix(stderr))
	}
	var finalResult struct {
		StopReason string `json:"stopReason"`
	}
	if err := json.Unmarshal(finalRaw, &finalResult); err != nil {
		return nil, fmt.Errorf("parse ACP session/prompt result: %w", err)
	}

	agentName := firstNonEmpty(initResult.AgentInfo.Name, agent.Name)
	return map[string]interface{}{
		"agent_id":    agent.ID,
		"agent_name":  agentName,
		"transport":   TransportStdioACP,
		"protocol":    acpProtocol,
		"endpoint":    bin,
		"session_id":  sessionResult.SessionID,
		"response":    text.String(),
		"stop_reason": finalResult.StopReason,
		"latency_ms":  measuredLatency(start),
	}, nil
}

func promptPiRPC(ctx context.Context, agent Resolved, bin string, prompt string, cwd string) (map[string]interface{}, error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, bin, "--offline", "--no-session", "--no-themes", "--mode", "rpc")
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "PI_OFFLINE=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open Pi stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open Pi stdout: %w", err)
	}
	stderr := &lockedBuffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", bin, err)
	}
	defer stopProcess(cmd, stdin)

	request, err := json.Marshal(struct {
		ID      string        `json:"id"`
		Type    string        `json:"type"`
		Message string        `json:"message"`
		Images  []interface{} `json:"images"`
	}{
		ID:      "prompt-1",
		Type:    "prompt",
		Message: prompt,
		Images:  []interface{}{},
	})
	if err != nil {
		return nil, fmt.Errorf("encode Pi prompt: %w", err)
	}
	if _, err := stdin.Write(append(request, '\n')); err != nil {
		return nil, fmt.Errorf("write Pi prompt: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	var text strings.Builder
	responseSeen := false
	settled := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event struct {
			ID                    string `json:"id"`
			Type                  string `json:"type"`
			Command               string `json:"command"`
			Success               bool   `json:"success"`
			Error                 string `json:"error"`
			AssistantMessageEvent struct {
				Type  string `json:"type"`
				Delta string `json:"delta"`
			} `json:"assistantMessageEvent"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		switch event.Type {
		case "response":
			if event.ID != "prompt-1" {
				continue
			}
			if !event.Success {
				return nil, fmt.Errorf("Pi prompt: %s", firstNonEmpty(event.Error, "success=false"))
			}
			responseSeen = true
		case "message_update":
			if event.AssistantMessageEvent.Type == "text_delta" {
				text.WriteString(event.AssistantMessageEvent.Delta)
			}
		case "agent_settled":
			settled = true
		}
		if settled {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read Pi prompt: %w%s", err, stderrSuffix(stderr))
	}
	if !responseSeen {
		return nil, fmt.Errorf("Pi prompt ended without a response%s", stderrSuffix(stderr))
	}
	if !settled && ctx.Err() != nil {
		return nil, fmt.Errorf("Pi prompt stopped: %w", ctx.Err())
	}

	return map[string]interface{}{
		"agent_id":    agent.ID,
		"agent_name":  firstNonEmpty(agent.Name, "Pi"),
		"transport":   TransportPiRPC,
		"protocol":    piRPCProtocol,
		"endpoint":    bin,
		"response":    text.String(),
		"stop_reason": "end_turn",
		"latency_ms":  measuredLatency(start),
	}, nil
}

func awaitACPResult(scanner *bufio.Scanner, stdin io.Writer, wantID string, onNotification func(rpcMessage)) (json.RawMessage, error) {
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg rpcMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.Method != "" {
			if len(msg.ID) > 0 {
				if err := respondToACPRequest(stdin, msg); err != nil {
					return nil, err
				}
			} else if onNotification != nil {
				onNotification(msg)
			}
			continue
		}
		if rpcIDString(msg.ID) != wantID {
			continue
		}
		if msg.Error != nil {
			return nil, errors.New(rpcErrorMessage(msg.Error))
		}
		return msg.Result, nil
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}

func respondToACPRequest(stdin io.Writer, msg rpcMessage) error {
	var response interface{}
	if msg.Method == "session/request_permission" {
		var params struct {
			Options []struct {
				OptionID string `json:"optionId"`
				Kind     string `json:"kind"`
			} `json:"options"`
		}
		_ = json.Unmarshal(msg.Params, &params)
		selected := ""
		for _, option := range params.Options {
			k := strings.ToLower(option.Kind)
			if strings.Contains(k, "reject") || strings.Contains(k, "deny") {
				selected = option.OptionID
				break
			}
		}
		if selected != "" {
			response = map[string]interface{}{
				"outcome": map[string]interface{}{
					"outcome":  "selected",
					"optionId": selected,
				},
			}
		} else {
			response = map[string]interface{}{
				"outcome": map[string]interface{}{"outcome": "cancelled"},
			}
		}
	} else {
		payload := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      json.RawMessage(msg.ID),
			"error": map[string]interface{}{
				"code":    -32601,
				"message": "method not found",
			},
		}
		return json.NewEncoder(stdin).Encode(payload)
	}

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(msg.ID),
		"result":  response,
	}
	return json.NewEncoder(stdin).Encode(payload)
}

func writeRPCMessage(w io.Writer, id int, method string, params interface{}) error {
	payload := struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      int         `json:"id"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params"`
	}{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	return json.NewEncoder(w).Encode(payload)
}

func acpMessageText(msg rpcMessage) string {
	if msg.Method != "session/update" {
		return ""
	}
	var notification struct {
		Update struct {
			SessionUpdate string `json:"sessionUpdate"`
			Content       struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"update"`
	}
	if err := json.Unmarshal(msg.Params, &notification); err != nil {
		return ""
	}
	if notification.Update.SessionUpdate != "agent_message_chunk" ||
		notification.Update.Content.Type != "text" {
		return ""
	}
	return notification.Update.Content.Text
}

func rpcIDString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	switch id := value.(type) {
	case string:
		return id
	case float64:
		return fmt.Sprintf("%.0f", id)
	default:
		return fmt.Sprint(id)
	}
}

func rpcErrorMessage(err *rpcError) string {
	if err == nil {
		return "unknown JSON-RPC error"
	}
	if strings.TrimSpace(err.Message) != "" {
		return err.Message
	}
	return fmt.Sprintf("JSON-RPC error %d", err.Code)
}

func stopProcess(cmd *exec.Cmd, stdin io.Closer) {
	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	_ = cmd.Wait()
}

func stderrSuffix(stderr *lockedBuffer) string {
	detail := strings.TrimSpace(stderr.String())
	if detail == "" {
		return ""
	}
	return ": " + detail
}

func resolvePromptCWD(cwd string) (string, error) {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve working directory: %w", err)
		}
	}
	if !filepath.IsAbs(cwd) {
		absolute, err := filepath.Abs(cwd)
		if err != nil {
			return "", fmt.Errorf("resolve working directory %s: %w", cwd, err)
		}
		cwd = absolute
	}
	info, err := os.Stat(cwd)
	if err != nil {
		return "", fmt.Errorf("working directory %s: %w", cwd, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("working directory %s is not a directory", cwd)
	}
	return filepath.Clean(cwd), nil
}

func resolvePiExecutable(expandedBin, configured string) string {
	for _, candidate := range []string{expandedBin, configured} {
		if isPiACPName(candidate) {
			continue
		}
		if bin := resolveExecutable(candidate); bin != "" {
			return bin
		}
	}
	return lookPath("pi")
}

func resolvePiACPExecutable(expandedBin string) string {
	if isPiACPName(expandedBin) {
		if bin := resolveExecutable(expandedBin); bin != "" {
			return bin
		}
	}
	if bin := lookPath("pi-acp"); bin != "" {
		return bin
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	patterns := []string{
		filepath.Join(home, ".config", "nvm", "versions", "node", "*", "bin", "pi-acp"),
		filepath.Join(home, ".nvm", "versions", "node", "*", "bin", "pi-acp"),
	}
	if nvmDir := strings.TrimSpace(os.Getenv("NVM_DIR")); nvmDir != "" {
		patterns = append(patterns, filepath.Join(nvmDir, "versions", "node", "*", "bin", "pi-acp"))
	}
	var matches []string
	for _, pattern := range patterns {
		found, err := filepath.Glob(pattern)
		if err == nil {
			matches = append(matches, found...)
		}
	}
	seen := make(map[string]bool)
	var valid []string
	for _, m := range matches {
		if seen[m] {
			continue
		}
		seen[m] = true
		if isFile(m) {
			valid = append(valid, m)
		}
	}
	if len(valid) == 0 {
		return ""
	}
	sort.SliceStable(valid, func(i, j int) bool {
		vi := filepath.Base(filepath.Dir(filepath.Dir(valid[i])))
		vj := filepath.Base(filepath.Dir(filepath.Dir(valid[j])))
		return compareVersions(vi, vj) > 0
	})
	return valid[0]
}

func parseNumericParts(s string) []int {
	parts := strings.Split(s, ".")
	res := make([]int, 0, len(parts))
	for _, p := range parts {
		var num int
		for i := 0; i < len(p); i++ {
			if p[i] < '0' || p[i] > '9' {
				break
			}
			num = num*10 + int(p[i]-'0')
		}
		res = append(res, num)
	}
	return res
}

func parseVersionParts(s string) []int {
	return parseNumericParts(s)
}

func parseIdentifier(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	num := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
		num = num*10 + int(s[i]-'0')
	}
	return num, true
}

func splitPrerelease(s string) (base string, pre string, hasPre bool) {
	if idx := strings.IndexByte(s, '-'); idx != -1 {
		return s[:idx], s[idx+1:], true
	}
	return s, "", false
}

func comparePrereleases(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	n := len(aParts)
	if len(bParts) > n {
		n = len(bParts)
	}
	for i := 0; i < n; i++ {
		if i >= len(aParts) {
			return -1
		}
		if i >= len(bParts) {
			return 1
		}
		ap, bp := aParts[i], bParts[i]
		aNum, aIsNum := parseIdentifier(ap)
		bNum, bIsNum := parseIdentifier(bp)
		if aIsNum && bIsNum {
			if aNum != bNum {
				return aNum - bNum
			}
		} else if aIsNum && !bIsNum {
			return -1
		} else if !aIsNum && bIsNum {
			return 1
		} else {
			if cmp := strings.Compare(ap, bp); cmp != 0 {
				return cmp
			}
		}
	}
	return 0
}

func compareVersions(a, b string) int {
	aClean := strings.TrimLeft(strings.TrimSpace(a), "vV")
	bClean := strings.TrimLeft(strings.TrimSpace(b), "vV")

	if idx := strings.IndexByte(aClean, '+'); idx != -1 {
		aClean = aClean[:idx]
	}
	if idx := strings.IndexByte(bClean, '+'); idx != -1 {
		bClean = bClean[:idx]
	}

	aBase, aPre, aHasPre := splitPrerelease(aClean)
	bBase, bPre, bHasPre := splitPrerelease(bClean)

	pa := parseNumericParts(aBase)
	pb := parseNumericParts(bBase)

	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		va, vb := 0, 0
		if i < len(pa) {
			va = pa[i]
		}
		if i < len(pb) {
			vb = pb[i]
		}
		if va != vb {
			return va - vb
		}
	}

	if !aHasPre && bHasPre {
		return 1
	}
	if aHasPre && !bHasPre {
		return -1
	}
	if aHasPre && bHasPre {
		return comparePrereleases(aPre, bPre)
	}
	return 0
}

func isPiACPName(path string) bool {
	name := strings.TrimSuffix(strings.ToLower(filepath.Base(strings.TrimSpace(path))), ".exe")
	return name == "pi-acp"
}

func resolveExecutable(candidates ...string) string {
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		candidate = expandPath(candidate, CurrentEnv())
		if isFile(candidate) {
			return candidate
		}
	}
	return ""
}

func lookPath(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
