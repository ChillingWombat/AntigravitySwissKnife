// Package acp exposes a surface over the Agent Client Protocol.
package acp

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/ChillingWombat/AntigravitySwissKnife/antigravity-acp/surface"
)

// Agent implements acpsdk.Agent on top of one surface.
type Agent struct {
	surface surface.Surface
	conn    sessionUpdater

	mu       sync.Mutex
	sessions map[string]sessionState
}

type sessionUpdater interface {
	SessionUpdate(context.Context, acpsdk.SessionNotification) error
}

type sessionState struct {
	cwd    string
	active bool
}

var _ acpsdk.Agent = (*Agent)(nil)
var _ acpsdk.AgentLoader = (*Agent)(nil)

func New(s surface.Surface) *Agent {
	return &Agent{surface: s, sessions: map[string]sessionState{}}
}

// Attach gives the agent the connection it pushes updates through. The connection
// is created after the agent, so it is set here rather than in the constructor.
func (a *Agent) Attach(conn *acpsdk.AgentSideConnection) { a.conn = conn }

func (a *Agent) Initialize(ctx context.Context, p acpsdk.InitializeRequest) (acpsdk.InitializeResponse, error) {
	if p.ProtocolVersion <= 0 {
		return acpsdk.InitializeResponse{}, acpsdk.NewInvalidParams(map[string]any{
			"error": "protocolVersion must be positive",
		})
	}
	capabilities := acpsdk.AgentCapabilities{
		PromptCapabilities: acpsdk.PromptCapabilities{},
		McpCapabilities:    acpsdk.McpCapabilities{},
	}
	if _, ok := a.surface.(surface.HistoryProvider); ok {
		capabilities.LoadSession = true
	}
	if _, ok := a.surface.(surface.SessionLister); ok {
		capabilities.SessionCapabilities.List = &acpsdk.SessionListCapabilities{}
	}
	return acpsdk.InitializeResponse{
		ProtocolVersion:   acpsdk.ProtocolVersionNumber,
		AgentInfo:         &acpsdk.Implementation{Name: "antigravity-acp", Version: "0.1.0"},
		AgentCapabilities: capabilities,
		AuthMethods:       []acpsdk.AuthMethod{},
	}, nil
}

// Authenticate is a no-op: the CLI and the app already hold Google credentials.
func (a *Agent) Authenticate(ctx context.Context, p acpsdk.AuthenticateRequest) (acpsdk.AuthenticateResponse, error) {
	return acpsdk.AuthenticateResponse{}, nil
}

func (a *Agent) Logout(ctx context.Context, p acpsdk.LogoutRequest) (acpsdk.LogoutResponse, error) {
	return acpsdk.LogoutResponse{}, acpsdk.NewMethodNotFound("logout")
}

func (a *Agent) NewSession(ctx context.Context, p acpsdk.NewSessionRequest) (acpsdk.NewSessionResponse, error) {
	id, err := a.surface.Start(ctx, surface.SessionOptions{Cwd: p.Cwd})
	if err != nil {
		return acpsdk.NewSessionResponse{}, err
	}
	a.mu.Lock()
	a.sessions[id] = sessionState{cwd: p.Cwd}
	a.mu.Unlock()
	return acpsdk.NewSessionResponse{SessionId: acpsdk.SessionId(id)}, nil
}

// Prompt runs one turn and streams the answer back as session updates.
func (a *Agent) Prompt(ctx context.Context, p acpsdk.PromptRequest) (acpsdk.PromptResponse, error) {
	sessionID := string(p.SessionId)
	if err := a.beginPrompt(sessionID); err != nil {
		return acpsdk.PromptResponse{}, err
	}
	defer a.endPrompt(sessionID)

	text := promptText(p.Prompt)
	reason, err := a.surface.Send(ctx, sessionID, text, func(ev surface.Event) error {
		if ev.Kind == surface.EventError {
			// Reporting end_turn here would tell the client the turn succeeded.
			return fmt.Errorf("surface error: %s", ev.Message)
		}
		return a.forwardEvent(ctx, p.SessionId, ev)
	})
	if err != nil {
		return acpsdk.PromptResponse{}, err
	}
	return acpsdk.PromptResponse{StopReason: stopReason(reason)}, nil
}

func (a *Agent) Cancel(ctx context.Context, p acpsdk.CancelNotification) error {
	sessionID := string(p.SessionId)
	if !a.hasSession(sessionID) {
		return unknownSessionError()
	}
	return a.surface.Cancel(ctx, sessionID)
}

func (a *Agent) CloseSession(ctx context.Context, p acpsdk.CloseSessionRequest) (acpsdk.CloseSessionResponse, error) {
	sessionID := string(p.SessionId)
	if !a.hasSession(sessionID) {
		return acpsdk.CloseSessionResponse{}, unknownSessionError()
	}
	if err := a.surface.Close(ctx, sessionID); err != nil {
		return acpsdk.CloseSessionResponse{}, err
	}
	a.mu.Lock()
	delete(a.sessions, sessionID)
	a.mu.Unlock()
	return acpsdk.CloseSessionResponse{}, nil
}

// ListSessions reports the conversations the surface knows about.
func (a *Agent) ListSessions(ctx context.Context, _ acpsdk.ListSessionsRequest) (acpsdk.ListSessionsResponse, error) {
	lister, ok := a.surface.(surface.SessionLister)
	if !ok {
		return acpsdk.ListSessionsResponse{}, acpsdk.NewMethodNotFound(acpsdk.AgentMethodSessionList)
	}
	infos, err := lister.List(ctx)
	if err != nil {
		return acpsdk.ListSessionsResponse{}, err
	}
	sessions := make([]acpsdk.SessionInfo, 0, len(infos))
	for _, info := range infos {
		cwd := info.Cwd
		if cwd == "" {
			a.mu.Lock()
			cwd = a.sessions[info.ID].cwd
			a.mu.Unlock()
		}
		if cwd == "" {
			if wd, wdErr := os.Getwd(); wdErr == nil && wd != "" {
				cwd = wd
			} else {
				cwd = "/"
			}
		}
		entry := acpsdk.SessionInfo{SessionId: acpsdk.SessionId(info.ID), Cwd: cwd}
		if info.Title != "" {
			title := info.Title
			entry.Title = &title
		}
		if info.UpdatedAt > 0 {
			stamp := time.Unix(info.UpdatedAt, 0).UTC().Format(time.RFC3339)
			entry.UpdatedAt = &stamp
		}
		sessions = append(sessions, entry)
	}
	return acpsdk.ListSessionsResponse{Sessions: sessions}, nil
}

// LoadSession replays a recorded conversation as updates, which is what the
// protocol requires before the client may prompt again.
func (a *Agent) LoadSession(ctx context.Context, p acpsdk.LoadSessionRequest) (acpsdk.LoadSessionResponse, error) {
	provider, ok := a.surface.(surface.HistoryProvider)
	if !ok {
		return acpsdk.LoadSessionResponse{}, acpsdk.NewMethodNotFound("session/load")
	}
	sid := string(p.SessionId)
	a.mu.Lock()
	state := a.sessions[sid]
	a.mu.Unlock()
	if state.active {
		return acpsdk.LoadSessionResponse{}, acpsdk.NewInvalidParams(map[string]any{
			"error": "session has an active prompt",
		})
	}
	events, err := provider.History(ctx, sid)
	if err != nil {
		return acpsdk.LoadSessionResponse{}, err
	}
	for _, ev := range events {
		if ev.Kind != surface.EventText || ev.Text == "" {
			continue
		}
		if err := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: p.SessionId,
			Update:    acpsdk.UpdateAgentMessageText(ev.Text),
		}); err != nil {
			return acpsdk.LoadSessionResponse{}, err
		}
	}
	a.mu.Lock()
	a.sessions[string(p.SessionId)] = sessionState{cwd: p.Cwd}
	a.mu.Unlock()
	return acpsdk.LoadSessionResponse{}, nil
}

func (a *Agent) ResumeSession(context.Context, acpsdk.ResumeSessionRequest) (acpsdk.ResumeSessionResponse, error) {
	return acpsdk.ResumeSessionResponse{}, acpsdk.NewMethodNotFound("session/resume")
}

func (a *Agent) SetSessionMode(context.Context, acpsdk.SetSessionModeRequest) (acpsdk.SetSessionModeResponse, error) {
	return acpsdk.SetSessionModeResponse{}, acpsdk.NewMethodNotFound(acpsdk.AgentMethodSessionSetMode)
}

// SetSessionConfigOption changes the model for a session when the surface can.
// The request is a union: only the id-valued form names a model.
func (a *Agent) SetSessionConfigOption(ctx context.Context, p acpsdk.SetSessionConfigOptionRequest) (acpsdk.SetSessionConfigOptionResponse, error) {
	sessionID := ""
	switch {
	case p.ValueId != nil:
		sessionID = string(p.ValueId.SessionId)
	case p.Boolean != nil:
		sessionID = string(p.Boolean.SessionId)
	}
	if !a.hasSession(sessionID) {
		return acpsdk.SetSessionConfigOptionResponse{}, unknownSessionError()
	}
	selector, ok := a.surface.(surface.ModelSelector)
	if !ok {
		return acpsdk.SetSessionConfigOptionResponse{}, acpsdk.NewMethodNotFound(acpsdk.AgentMethodSessionSetConfigOption)
	}
	if p.ValueId == nil {
		return acpsdk.SetSessionConfigOptionResponse{}, acpsdk.NewInvalidParams(map[string]any{
			"error": "only the model option takes an id value",
		})
	}
	if p.ValueId.ConfigId != "model" {
		return acpsdk.SetSessionConfigOptionResponse{}, acpsdk.NewInvalidParams(map[string]any{
			"error": "only the model option is supported",
		})
	}
	if err := selector.SetModel(ctx, string(p.ValueId.SessionId), string(p.ValueId.Value)); err != nil {
		return acpsdk.SetSessionConfigOptionResponse{}, err
	}
	return acpsdk.SetSessionConfigOptionResponse{
		ConfigOptions: []acpsdk.SessionConfigOption{modelConfigOption(p.ValueId.Value)},
	}, nil
}

func (a *Agent) forwardEvent(ctx context.Context, sessionID acpsdk.SessionId, ev surface.Event) error {
	var update acpsdk.SessionUpdate
	switch ev.Kind {
	case surface.EventText:
		if ev.Text == "" {
			return nil
		}
		update = acpsdk.UpdateAgentMessageText(ev.Text)
	case surface.EventToolStart:
		if ev.ToolID == "" {
			return a.forwardToolText(ctx, sessionID, ev)
		}
		status, _ := toolStatus(ev.State, acpsdk.ToolCallStatusPending)
		update = acpsdk.StartToolCall(
			acpsdk.ToolCallId(ev.ToolID),
			toolTitle(ev, "tool call"),
			acpsdk.WithStartStatus(status),
		)
	case surface.EventToolUpdate:
		if ev.ToolID == "" {
			return a.forwardToolText(ctx, sessionID, ev)
		}
		opts := make([]acpsdk.ToolCallUpdateOpt, 0, 2)
		if ev.Tool != "" || ev.Message != "" {
			opts = append(opts, acpsdk.WithUpdateTitle(toolTitle(ev, "tool call")))
		}
		if status, ok := toolStatus(ev.State, ""); ok {
			opts = append(opts, acpsdk.WithUpdateStatus(status))
		}
		update = acpsdk.UpdateToolCall(acpsdk.ToolCallId(ev.ToolID), opts...)
	default:
		return nil
	}
	return a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{SessionId: sessionID, Update: update})
}

func (a *Agent) forwardToolText(ctx context.Context, sessionID acpsdk.SessionId, ev surface.Event) error {
	text := strings.TrimSpace(strings.Join([]string{ev.Tool, ev.Text, ev.Message, ev.State}, " "))
	if text == "" {
		return nil
	}
	return a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
		SessionId: sessionID,
		Update:    acpsdk.UpdateAgentMessageText(text),
	})
}

func toolTitle(ev surface.Event, fallback string) string {
	for _, title := range []string{ev.Tool, ev.Message, ev.Text} {
		if title != "" {
			return title
		}
	}
	return fallback
}

func toolStatus(state string, fallback acpsdk.ToolCallStatus) (acpsdk.ToolCallStatus, bool) {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "pending", "queued":
		return acpsdk.ToolCallStatusPending, true
	case "active", "in-progress", "in_progress", "running", "started":
		return acpsdk.ToolCallStatusInProgress, true
	case "complete", "completed", "done", "success", "succeeded":
		return acpsdk.ToolCallStatusCompleted, true
	case "error", "failed", "failure":
		return acpsdk.ToolCallStatusFailed, true
	default:
		return fallback, fallback != ""
	}
}

func modelConfigOption(value acpsdk.SessionConfigValueId) acpsdk.SessionConfigOption {
	category := acpsdk.SessionConfigOptionCategoryModel
	options := acpsdk.SessionConfigSelectOptionsUngrouped{
		{Name: string(value), Value: value},
	}
	return acpsdk.SessionConfigOption{
		Select: &acpsdk.SessionConfigOptionSelect{
			Category:     &category,
			CurrentValue: value,
			Id:           "model",
			Name:         "Model",
			Options:      acpsdk.SessionConfigSelectOptions{Ungrouped: &options},
			Type:         "select",
		},
	}
}

func (a *Agent) hasSession(sessionID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.sessions[sessionID]
	return ok
}

func (a *Agent) beginPrompt(sessionID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	state, ok := a.sessions[sessionID]
	if !ok {
		return unknownSessionError()
	}
	if state.active {
		return acpsdk.NewInvalidParams(map[string]any{"error": "prompt already in progress"})
	}
	state.active = true
	a.sessions[sessionID] = state
	return nil
}

func (a *Agent) endPrompt(sessionID string) {
	a.mu.Lock()
	state, ok := a.sessions[sessionID]
	if ok {
		state.active = false
		a.sessions[sessionID] = state
	}
	a.mu.Unlock()
}

func unknownSessionError() error {
	return acpsdk.NewInvalidParams(map[string]any{"error": "unknown sessionId"})
}

// promptText flattens the client's content blocks into the string a surface takes.
func promptText(blocks []acpsdk.ContentBlock) string {
	var out strings.Builder
	for _, block := range blocks {
		if block.Text != nil && block.Text.Text != "" {
			out.WriteString(block.Text.Text)
		}
	}
	return out.String()
}

func stopReason(r surface.StopReason) acpsdk.StopReason {
	switch r {
	case surface.StopCancelled:
		return acpsdk.StopReasonCancelled
	case surface.StopRefusal:
		return acpsdk.StopReasonRefusal
	case surface.StopMaxTokens:
		return acpsdk.StopReasonMaxTokens
	default:
		return acpsdk.StopReasonEndTurn
	}
}
