// Command acp-probe drives an ACP agent over stdio without an editor. It exists so
// the adapter can be exercised from a terminal: initialize, open a session, send one
// prompt, print the streamed answer and how the turn ended.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
)

// probeClient records what the agent sends and answers permission requests, since
// there is no human in the loop.
type probeClient struct {
	mu        sync.Mutex
	text      strings.Builder
	inHistory bool
}

func (c *probeClient) setHistory(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.inHistory = v
}

func (c *probeClient) SessionUpdate(_ context.Context, n acpsdk.SessionNotification) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.inHistory {
		if n.Update.UserMessageChunk != nil && n.Update.UserMessageChunk.Content.Text != nil {
			fmt.Printf("[history user] %s\n", n.Update.UserMessageChunk.Content.Text.Text)
		}
		if n.Update.AgentMessageChunk != nil && n.Update.AgentMessageChunk.Content.Text != nil {
			fmt.Printf("[history agent] %s\n", n.Update.AgentMessageChunk.Content.Text.Text)
		}
		return nil
	}

	if n.Update.AgentMessageChunk != nil && n.Update.AgentMessageChunk.Content.Text != nil {
		chunk := n.Update.AgentMessageChunk.Content.Text.Text
		c.text.WriteString(chunk)
		fmt.Print(chunk)
	}
	return nil
}

func (c *probeClient) RequestPermission(_ context.Context, p acpsdk.RequestPermissionRequest) (acpsdk.RequestPermissionResponse, error) {
	// Choose the first allow-shaped option; otherwise deny rather than hang.
	for _, option := range p.Options {
		if strings.Contains(strings.ToLower(string(option.Kind)), "allow") {
			return acpsdk.RequestPermissionResponse{
				Outcome: acpsdk.RequestPermissionOutcome{Selected: &acpsdk.RequestPermissionOutcomeSelected{OptionId: option.OptionId}},
			}, nil
		}
	}
	return acpsdk.RequestPermissionResponse{
		Outcome: acpsdk.RequestPermissionOutcome{Cancelled: &acpsdk.RequestPermissionOutcomeCancelled{}},
	}, nil
}

func (c *probeClient) ReadTextFile(context.Context, acpsdk.ReadTextFileRequest) (acpsdk.ReadTextFileResponse, error) {
	return acpsdk.ReadTextFileResponse{}, acpsdk.NewMethodNotFound("fs/read_text_file")
}

func (c *probeClient) WriteTextFile(context.Context, acpsdk.WriteTextFileRequest) (acpsdk.WriteTextFileResponse, error) {
	return acpsdk.WriteTextFileResponse{}, acpsdk.NewMethodNotFound("fs/write_text_file")
}

func (c *probeClient) CreateTerminal(context.Context, acpsdk.CreateTerminalRequest) (acpsdk.CreateTerminalResponse, error) {
	return acpsdk.CreateTerminalResponse{}, acpsdk.NewMethodNotFound("terminal/create")
}

func (c *probeClient) KillTerminal(context.Context, acpsdk.KillTerminalRequest) (acpsdk.KillTerminalResponse, error) {
	return acpsdk.KillTerminalResponse{}, acpsdk.NewMethodNotFound("terminal/kill")
}

func (c *probeClient) TerminalOutput(context.Context, acpsdk.TerminalOutputRequest) (acpsdk.TerminalOutputResponse, error) {
	return acpsdk.TerminalOutputResponse{}, acpsdk.NewMethodNotFound("terminal/output")
}

func (c *probeClient) ReleaseTerminal(context.Context, acpsdk.ReleaseTerminalRequest) (acpsdk.ReleaseTerminalResponse, error) {
	return acpsdk.ReleaseTerminalResponse{}, acpsdk.NewMethodNotFound("terminal/release")
}

func (c *probeClient) WaitForTerminalExit(context.Context, acpsdk.WaitForTerminalExitRequest) (acpsdk.WaitForTerminalExitResponse, error) {
	return acpsdk.WaitForTerminalExitResponse{}, acpsdk.NewMethodNotFound("terminal/wait_for_exit")
}

type mockStep struct {
	Type            string          `json:"type"`
	PlannerResponse *plannerResponse `json:"plannerResponse,omitempty"`
	UserInput       *userInput       `json:"userInput,omitempty"`
}

type plannerResponse struct {
	Response string `json:"response"`
}

type userInput struct {
	UserResponse string     `json:"userResponse"`
	Items        []userItem `json:"items"`
}

type userItem struct {
	Text string `json:"text"`
}

type mockSession struct {
	cascadeID    string
	workspaceURI string
	steps        []mockStep
}

type mockServerState struct {
	mu       sync.Mutex
	sessions map[string]*mockSession
}

func (s *mockServerState) seedSession(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.sessions[id]; !exists {
		s.sessions[id] = &mockSession{
			cascadeID:    id,
			workspaceURI: "file:///tmp/mock-workspace",
			steps: []mockStep{
				{
					Type: "CORTEX_STEP_TYPE_USER_INPUT",
					UserInput: &userInput{
						UserResponse: "past user prompt",
						Items:        []userItem{{Text: "past user prompt"}},
					},
				},
				{
					Type: "CORTEX_STEP_TYPE_PLANNER_RESPONSE",
					PlannerResponse: &plannerResponse{
						Response: "past agent response",
					},
				},
			},
		}
	}
}

func startMockLanguageServer() (*httptest.Server, *mockServerState) {
	state := &mockServerState{
		sessions: map[string]*mockSession{},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		var body map[string]interface{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}

		state.mu.Lock()
		defer state.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		switch method {
		case "StartCascade":
			id := fmt.Sprintf("mock-cascade-%d", time.Now().UnixNano())
			ws := "file:///tmp/mock-workspace"
			if rawUris, ok := body["workspaceUris"].([]interface{}); ok && len(rawUris) > 0 {
				if s, ok := rawUris[0].(string); ok && s != "" {
					ws = s
				}
			}
			state.sessions[id] = &mockSession{
				cascadeID:    id,
				workspaceURI: ws,
				steps:        []mockStep{},
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"cascadeId": id})

		case "GetAllCascadeTrajectories":
			summaries := map[string]interface{}{}
			for id, sess := range state.sessions {
				summaries[id] = map[string]interface{}{
					"stepCount":        len(sess.steps),
					"status":           "CASCADE_RUN_STATUS_IDLE",
					"workspaceUri":     sess.workspaceURI,
					"workspaceUris":    []string{sess.workspaceURI},
					"lastModifiedTime": time.Now().UTC().Format(time.RFC3339Nano),
					"createdTime":      time.Now().UTC().Format(time.RFC3339Nano),
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"trajectorySummaries": summaries,
			})

		case "SendUserCascadeMessage":
			cascadeID, _ := body["cascadeId"].(string)
			sess := state.sessions[cascadeID]
			if sess == nil {
				sess = &mockSession{
					cascadeID:    cascadeID,
					workspaceURI: "file:///tmp/mock-workspace",
					steps:        []mockStep{},
				}
				state.sessions[cascadeID] = sess
			}
			promptText := "probe-ok"
			if items, ok := body["items"].([]interface{}); ok && len(items) > 0 {
				if itemMap, ok := items[0].(map[string]interface{}); ok {
					if t, ok := itemMap["text"].(string); ok && t != "" {
						promptText = t
					}
				}
			}
			sess.steps = append(sess.steps, mockStep{
				Type: "CORTEX_STEP_TYPE_USER_INPUT",
				UserInput: &userInput{
					UserResponse: promptText,
					Items:        []userItem{{Text: promptText}},
				},
			})
			sess.steps = append(sess.steps, mockStep{
				Type: "CORTEX_STEP_TYPE_PLANNER_RESPONSE",
				PlannerResponse: &plannerResponse{
					Response: "mock: " + promptText,
				},
			})
			_ = json.NewEncoder(w).Encode(map[string]interface{}{})

		case "GetCascadeTrajectorySteps":
			cascadeID, _ := body["cascadeId"].(string)
			sess := state.sessions[cascadeID]
			if sess == nil {
				sess = &mockSession{
					cascadeID:    cascadeID,
					workspaceURI: "file:///tmp/mock-workspace",
					steps: []mockStep{
						{
							Type: "CORTEX_STEP_TYPE_USER_INPUT",
							UserInput: &userInput{
								UserResponse: "past user prompt",
								Items:        []userItem{{Text: "past user prompt"}},
							},
						},
						{
							Type: "CORTEX_STEP_TYPE_PLANNER_RESPONSE",
							PlannerResponse: &plannerResponse{
								Response: "past agent response",
							},
						},
					},
				}
				state.sessions[cascadeID] = sess
			}
			offset := 0
			if rawOffset, ok := body["stepOffset"].(float64); ok {
				offset = int(rawOffset)
			}
			steps := sess.steps
			if offset > len(steps) {
				offset = len(steps)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"steps": steps[offset:],
			})

		case "CancelCascadeInvocation":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{})

		default:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{})
		}
	}))

	return srv, state
}

func main() {
	mock := flag.Bool("mock", false, "spin up an in-process mock language server for zero-interference isolated testing")
	lsURL := flag.String("ls-url", "", "custom language server URL (passed to agent)")
	lsCSRF := flag.String("ls-csrf", "", "custom language server CSRF token (passed to agent)")
	server := flag.String("server", "antigravity-acp --surface ls", "agent command to spawn")
	sessionID := flag.String("session", "", "existing conversation/session ID to load instead of creating one")
	prompt := flag.String("prompt", "Reply with exactly: probe-ok", "prompt to send")
	cwd := flag.String("cwd", ".", "session working directory")
	timeout := flag.Duration("timeout", 180*time.Second, "overall deadline")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	if *mock {
		mockSrv, state := startMockLanguageServer()
		defer mockSrv.Close()
		*lsURL = mockSrv.URL
		*lsCSRF = "mock-csrf-tok"
		if strings.TrimSpace(*sessionID) != "" {
			state.seedSession(strings.TrimSpace(*sessionID))
		}
	}

	fields := strings.Fields(*server)
	binary := fields[0]
	if _, err := exec.LookPath(binary); err != nil {
		if _, statErr := os.Stat("./" + binary); statErr == nil {
			binary = "./" + binary
		}
	}

	args := append([]string(nil), fields[1:]...)
	if *lsURL != "" {
		hasURL := false
		for _, arg := range args {
			if arg == "--ls-url" || strings.HasPrefix(arg, "--ls-url=") {
				hasURL = true
				break
			}
		}
		if !hasURL {
			args = append(args, "--ls-url", *lsURL)
		}
	}
	if *lsCSRF != "" {
		hasCSRF := false
		for _, arg := range args {
			if arg == "--ls-csrf" || strings.HasPrefix(arg, "--ls-csrf=") {
				hasCSRF = true
				break
			}
		}
		if !hasCSRF {
			args = append(args, "--ls-csrf", *lsCSRF)
		}
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	if *lsURL != "" {
		cmd.Env = append(os.Environ(),
			"ANTIGRAVITY_LS_URL="+*lsURL,
			"ANTIGRAVITY_LS_CSRF="+*lsCSRF,
		)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "stdin:", err)
		os.Exit(1)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "stdout:", err)
		os.Exit(1)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "start:", err)
		os.Exit(1)
	}
	defer func() { _ = cmd.Process.Kill() }()

	client := &probeClient{}
	conn := acpsdk.NewClientSideConnection(client, stdin, stdout)

	init, err := conn.Initialize(ctx, acpsdk.InitializeRequest{ProtocolVersion: acpsdk.ProtocolVersionNumber})
	if err != nil {
		fmt.Fprintln(os.Stderr, "initialize:", err)
		os.Exit(1)
	}
	fmt.Printf("agent: %s %s | protocol %d\n", init.AgentInfo.Name, init.AgentInfo.Version, init.ProtocolVersion)

	var session acpsdk.SessionId
	if strings.TrimSpace(*sessionID) != "" {
		session = acpsdk.SessionId(strings.TrimSpace(*sessionID))
		client.setHistory(true)
		if _, err := conn.LoadSession(ctx, acpsdk.LoadSessionRequest{
			SessionId:  session,
			Cwd:        *cwd,
			McpServers: []acpsdk.McpServer{},
		}); err != nil {
			fmt.Fprintln(os.Stderr, "session/load:", err)
			os.Exit(1)
		}
		client.setHistory(false)
		fmt.Printf("loaded session: %s\n", session)
	} else {
		newSession, err := conn.NewSession(ctx, acpsdk.NewSessionRequest{Cwd: *cwd, McpServers: []acpsdk.McpServer{}})
		if err != nil {
			fmt.Fprintln(os.Stderr, "session/new:", err)
			os.Exit(1)
		}
		session = newSession.SessionId
		fmt.Printf("session: %s\n", session)
	}

	var blocks []acpsdk.ContentBlock
	blocks = append(blocks, acpsdk.ContentBlock{Text: &acpsdk.ContentBlockText{Type: "text", Text: *prompt}})
	result, err := conn.Prompt(ctx, acpsdk.PromptRequest{SessionId: session, Prompt: blocks})
	if err != nil {
		fmt.Fprintln(os.Stderr, "\nprompt:", err)
		os.Exit(1)
	}
	fmt.Printf("\nstop: %s\n", result.StopReason)
}
