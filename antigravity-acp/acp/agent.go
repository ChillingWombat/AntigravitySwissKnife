// Package acp exposes a surface over the Agent Client Protocol.
package acp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/ChillingWombat/AntigravitySwissKnife/antigravity-acp/surface"
)

// Agent implements acpsdk.Agent on top of one surface.
type Agent struct {
	surface surface.Surface
	conn    *acpsdk.AgentSideConnection

	mu       sync.Mutex
	sessions map[string]acpsdk.SessionId
}

var _ acpsdk.Agent = (*Agent)(nil)
var _ acpsdk.AgentLoader = (*Agent)(nil)

func New(s surface.Surface) *Agent {
	return &Agent{surface: s, sessions: map[string]acpsdk.SessionId{}}
}

// Attach gives the agent the connection it pushes updates through. The connection
// is created after the agent, so it is set here rather than in the constructor.
func (a *Agent) Attach(conn *acpsdk.AgentSideConnection) { a.conn = conn }

func (a *Agent) Initialize(ctx context.Context, p acpsdk.InitializeRequest) (acpsdk.InitializeResponse, error) {
	if p.ProtocolVersion != acpsdk.ProtocolVersionNumber {
		return acpsdk.InitializeResponse{}, acpsdk.NewInvalidParams(map[string]any{
			"error": "unsupported protocolVersion",
		})
	}
	return acpsdk.InitializeResponse{
		ProtocolVersion: acpsdk.ProtocolVersionNumber,
		AgentInfo:       &acpsdk.Implementation{Name: "antigravity-acp", Version: "0.1.0"},
		AgentCapabilities: acpsdk.AgentCapabilities{
			LoadSession:         true,
			PromptCapabilities:  acpsdk.PromptCapabilities{},
			McpCapabilities:     acpsdk.McpCapabilities{},
			SessionCapabilities: acpsdk.SessionCapabilities{List: &acpsdk.SessionListCapabilities{}},
		},
		AuthMethods: []acpsdk.AuthMethod{},
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
	a.sessions[id] = acpsdk.SessionId(id)
	a.mu.Unlock()
	return acpsdk.NewSessionResponse{SessionId: acpsdk.SessionId(id)}, nil
}

// Prompt runs one turn and streams the answer back as session updates.
func (a *Agent) Prompt(ctx context.Context, p acpsdk.PromptRequest) (acpsdk.PromptResponse, error) {
	text := promptText(p.Prompt)
	reason, err := a.surface.Send(ctx, string(p.SessionId), text, func(ev surface.Event) error {
		if ev.Kind == surface.EventError {
			// Reporting end_turn here would tell the client the turn succeeded.
			return fmt.Errorf("surface error: %s", ev.Message)
		}
		if ev.Kind != surface.EventText || ev.Text == "" {
			return nil
		}
		return a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: p.SessionId,
			Update:    acpsdk.UpdateAgentMessageText(ev.Text),
		})
	})
	if err != nil {
		return acpsdk.PromptResponse{}, err
	}
	return acpsdk.PromptResponse{StopReason: stopReason(reason)}, nil
}

func (a *Agent) Cancel(ctx context.Context, p acpsdk.CancelNotification) error {
	return a.surface.Cancel(ctx, string(p.SessionId))
}

func (a *Agent) CloseSession(ctx context.Context, p acpsdk.CloseSessionRequest) (acpsdk.CloseSessionResponse, error) {
	if err := a.surface.Close(ctx, string(p.SessionId)); err != nil {
		return acpsdk.CloseSessionResponse{}, err
	}
	a.mu.Lock()
	delete(a.sessions, string(p.SessionId))
	a.mu.Unlock()
	return acpsdk.CloseSessionResponse{}, nil
}

// ListSessions reports the conversations the surface knows about.
func (a *Agent) ListSessions(ctx context.Context, _ acpsdk.ListSessionsRequest) (acpsdk.ListSessionsResponse, error) {
	infos, err := a.surface.List(ctx)
	if err != nil {
		return acpsdk.ListSessionsResponse{}, err
	}
	sessions := make([]acpsdk.SessionInfo, 0, len(infos))
	for _, info := range infos {
		entry := acpsdk.SessionInfo{SessionId: acpsdk.SessionId(info.ID)}
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
	events, err := provider.History(ctx, string(p.SessionId))
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
	a.sessions[string(p.SessionId)] = p.SessionId
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
	return acpsdk.SetSessionConfigOptionResponse{}, nil
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
