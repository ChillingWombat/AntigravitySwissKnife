package acp

import (
	"context"
	"errors"
	"sync"
	"testing"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/ChillingWombat/AntigravitySwissKnife/antigravity-acp/surface"
	clisurface "github.com/ChillingWombat/AntigravitySwissKnife/antigravity-acp/surface/cli"
	lssurface "github.com/ChillingWombat/AntigravitySwissKnife/antigravity-acp/surface/ls"
)

type fakeSurface struct {
	name    string
	id      string
	infos   []surface.SessionInfo
	send    func(context.Context, string, string, func(surface.Event) error) (surface.StopReason, error)
	model   string
	cancel  int
	closed  int
	started int
}

func (s *fakeSurface) Name() string {
	if s.name == "" {
		return "fake"
	}
	return s.name
}

func (s *fakeSurface) Start(context.Context, surface.SessionOptions) (string, error) {
	s.started++
	if s.id == "" {
		return "session-1", nil
	}
	return s.id, nil
}

func (s *fakeSurface) Send(ctx context.Context, sessionID, prompt string, emit func(surface.Event) error) (surface.StopReason, error) {
	if s.send != nil {
		return s.send(ctx, sessionID, prompt, emit)
	}
	return surface.StopEndTurn, nil
}

func (s *fakeSurface) Cancel(context.Context, string) error {
	s.cancel++
	return nil
}

func (s *fakeSurface) List(context.Context) ([]surface.SessionInfo, error) {
	return s.infos, nil
}

func (s *fakeSurface) Close(context.Context, string) error {
	s.closed++
	return nil
}

func (s *fakeSurface) SetModel(_ context.Context, _ string, model string) error {
	s.model = model
	return nil
}

type historyFakeSurface struct {
	*fakeSurface
	events []surface.Event
}

func (s *historyFakeSurface) History(context.Context, string) ([]surface.Event, error) {
	return s.events, nil
}

type blockingSurface struct {
	*fakeSurface
	sendStarted chan struct{}
	releaseSend chan struct{}
}

func (s *blockingSurface) Send(context.Context, string, string, func(surface.Event) error) (surface.StopReason, error) {
	close(s.sendStarted)
	<-s.releaseSend
	return surface.StopEndTurn, nil
}

type recordingUpdater struct {
	mu      sync.Mutex
	updates []acpsdk.SessionNotification
}

func (u *recordingUpdater) SessionUpdate(_ context.Context, notification acpsdk.SessionNotification) error {
	u.mu.Lock()
	u.updates = append(u.updates, notification)
	u.mu.Unlock()
	return nil
}

func TestInitializeNegotiatesProtocolVersion(t *testing.T) {
	agent := New(&fakeSurface{})

	for _, version := range []acpsdk.ProtocolVersion{0, -1} {
		if _, err := agent.Initialize(context.Background(), acpsdk.InitializeRequest{ProtocolVersion: version}); err == nil {
			t.Errorf("protocol version %d: expected invalid params", version)
		}
	}

	for _, version := range []acpsdk.ProtocolVersion{acpsdk.ProtocolVersionNumber, acpsdk.ProtocolVersionNumber + 1} {
		resp, err := agent.Initialize(context.Background(), acpsdk.InitializeRequest{ProtocolVersion: version})
		if err != nil {
			t.Fatalf("protocol version %d: %v", version, err)
		}
		if resp.ProtocolVersion != acpsdk.ProtocolVersionNumber {
			t.Errorf("protocol version %d negotiated to %d", version, resp.ProtocolVersion)
		}
	}
}

func TestInitializeDerivesCapabilities(t *testing.T) {
	cliAgent := New(clisurface.New("agy"))
	cliResp, err := cliAgent.Initialize(context.Background(), acpsdk.InitializeRequest{ProtocolVersion: acpsdk.ProtocolVersionNumber})
	if err != nil {
		t.Fatalf("CLI initialize: %v", err)
	}
	if cliResp.AgentCapabilities.LoadSession {
		t.Error("CLI advertised session/load without a HistoryProvider")
	}
	if cliResp.AgentCapabilities.SessionCapabilities.List != nil {
		t.Error("CLI advertised session/list without a SessionLister")
	}

	lsAgent := New(lssurface.New("http://127.0.0.1:1", "token"))
	lsResp, err := lsAgent.Initialize(context.Background(), acpsdk.InitializeRequest{ProtocolVersion: acpsdk.ProtocolVersionNumber})
	if err != nil {
		t.Fatalf("LS initialize: %v", err)
	}
	if !lsResp.AgentCapabilities.LoadSession {
		t.Error("LS did not advertise session/load")
	}
	if lsResp.AgentCapabilities.SessionCapabilities.List == nil {
		t.Error("LS did not advertise session/list")
	}
}

func TestSessionMethodsRejectUnknownSessions(t *testing.T) {
	s := &fakeSurface{}
	agent := New(s)
	ctx := context.Background()
	unknown := acpsdk.SessionId("missing")

	_, err := agent.Prompt(ctx, acpsdk.PromptRequest{
		SessionId: unknown,
		Prompt:    []acpsdk.ContentBlock{acpsdk.TextBlock("hello")},
	})
	assertInvalidParamsError(t, err, "unknown sessionId")

	err = agent.Cancel(ctx, acpsdk.CancelNotification{SessionId: unknown})
	assertInvalidParamsError(t, err, "unknown sessionId")

	_, err = agent.CloseSession(ctx, acpsdk.CloseSessionRequest{SessionId: unknown})
	assertInvalidParamsError(t, err, "unknown sessionId")

	_, err = agent.SetSessionConfigOption(ctx, acpsdk.SetSessionConfigOptionRequest{
		ValueId: &acpsdk.SetSessionConfigOptionValueId{
			SessionId: unknown,
			ConfigId:  "model",
			Value:     "model-2",
		},
	})
	assertInvalidParamsError(t, err, "unknown sessionId")
}

func TestPromptRejectsConcurrentTurnForSameSession(t *testing.T) {
	s := &blockingSurface{
		fakeSurface: &fakeSurface{},
		sendStarted: make(chan struct{}),
		releaseSend: make(chan struct{}),
	}
	agent := New(s)
	session, err := agent.NewSession(context.Background(), acpsdk.NewSessionRequest{Cwd: "/tmp"})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	firstDone := make(chan error, 1)
	go func() {
		_, err := agent.Prompt(context.Background(), acpsdk.PromptRequest{
			SessionId: session.SessionId,
			Prompt:    []acpsdk.ContentBlock{acpsdk.TextBlock("first")},
		})
		firstDone <- err
	}()

	<-s.sendStarted
	_, err = agent.Prompt(context.Background(), acpsdk.PromptRequest{
		SessionId: session.SessionId,
		Prompt:    []acpsdk.ContentBlock{acpsdk.TextBlock("second")},
	})
	if err == nil {
		t.Fatal("expected concurrent prompt to be rejected")
	}
	var requestErr *acpsdk.RequestError
	if !errors.As(err, &requestErr) || requestErr.Code != -32602 {
		t.Fatalf("concurrent prompt error = %v, want invalid params", err)
	}

	close(s.releaseSend)
	if err := <-firstDone; err != nil {
		t.Fatalf("first prompt: %v", err)
	}
}

func TestSetSessionConfigOptionReturnsModelOption(t *testing.T) {
	s := &fakeSurface{}
	agent := New(s)
	session, err := agent.NewSession(context.Background(), acpsdk.NewSessionRequest{Cwd: "/tmp"})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	resp, err := agent.SetSessionConfigOption(context.Background(), acpsdk.SetSessionConfigOptionRequest{
		ValueId: &acpsdk.SetSessionConfigOptionValueId{
			SessionId: session.SessionId,
			ConfigId:  "model",
			Value:     "model-2",
		},
	})
	if err != nil {
		t.Fatalf("SetSessionConfigOption: %v", err)
	}
	if len(resp.ConfigOptions) != 1 {
		t.Fatalf("config options = %d, want 1", len(resp.ConfigOptions))
	}
	option := resp.ConfigOptions[0]
	if option.Select == nil {
		t.Fatal("model config option is not a select")
	}
	if option.Select.Id != "model" || option.Select.CurrentValue != "model-2" {
		t.Errorf("model option = %+v", option.Select)
	}
	if s.model != "model-2" {
		t.Errorf("surface model = %q, want model-2", s.model)
	}
}

func TestListSessionsIncludesCwd(t *testing.T) {
	s := &fakeSurface{
		infos: []surface.SessionInfo{
			{ID: "listed", Cwd: "/workspace/project"},
		},
	}
	agent := New(s)

	resp, err := agent.ListSessions(context.Background(), acpsdk.ListSessionsRequest{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(resp.Sessions) != 1 {
		t.Fatalf("sessions = %d, want 1", len(resp.Sessions))
	}
	if resp.Sessions[0].Cwd != "/workspace/project" {
		t.Errorf("cwd = %q, want /workspace/project", resp.Sessions[0].Cwd)
	}
}

func TestLoadSessionRegistersSession(t *testing.T) {
	s := &historyFakeSurface{fakeSurface: &fakeSurface{}}
	agent := New(s)
	sessionID := acpsdk.SessionId("loaded")

	if _, err := agent.LoadSession(context.Background(), acpsdk.LoadSessionRequest{SessionId: sessionID}); err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if _, err := agent.Prompt(context.Background(), acpsdk.PromptRequest{
		SessionId: sessionID,
		Prompt:    []acpsdk.ContentBlock{acpsdk.TextBlock("hello")},
	}); err != nil {
		t.Fatalf("Prompt after load: %v", err)
	}
}

// Replayed history must keep both sides of the conversation: a user turn
// delivered as an agent message makes the transcript claim the agent said it.
func TestLoadSessionReplaysBothSidesOfHistory(t *testing.T) {
	s := &historyFakeSurface{
		fakeSurface: &fakeSurface{},
		events: []surface.Event{
			{Kind: surface.EventUserText, Text: "remember BANANA7"},
			{Kind: surface.EventText, Text: "stored"},
			{Kind: surface.EventToolStart, ToolID: "tool-1", Tool: "read_file"},
		},
	}
	agent := New(s)
	updater := &recordingUpdater{}
	agent.conn = updater

	if _, err := agent.LoadSession(context.Background(), acpsdk.LoadSessionRequest{SessionId: "loaded"}); err != nil {
		t.Fatalf("LoadSession: %v", err)
	}

	if len(updater.updates) != 2 {
		t.Fatalf("replayed %d updates, want 2 (tool events carry no text)", len(updater.updates))
	}
	first := updater.updates[0].Update
	if first.UserMessageChunk == nil {
		t.Errorf("first update must be a user message, got %+v", first)
	}
	second := updater.updates[1].Update
	if second.AgentMessageChunk == nil {
		t.Errorf("second update must be an agent message, got %+v", second)
	}
}

func TestPromptForwardsToolEvents(t *testing.T) {
	s := &fakeSurface{
		send: func(_ context.Context, _ string, _ string, emit func(surface.Event) error) (surface.StopReason, error) {
			if err := emit(surface.Event{
				Kind:   surface.EventToolStart,
				ToolID: "tool-1",
				Tool:   "read_file",
				State:  "pending",
			}); err != nil {
				return "", err
			}
			if err := emit(surface.Event{
				Kind:   surface.EventToolUpdate,
				ToolID: "tool-1",
				Tool:   "read_file",
				State:  "completed",
			}); err != nil {
				return "", err
			}
			return surface.StopEndTurn, emit(surface.Event{Kind: surface.EventText, Text: "done"})
		},
	}
	agent := New(s)
	updater := &recordingUpdater{}
	agent.conn = updater
	session, err := agent.NewSession(context.Background(), acpsdk.NewSessionRequest{Cwd: "/tmp"})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	if _, err := agent.Prompt(context.Background(), acpsdk.PromptRequest{
		SessionId: session.SessionId,
		Prompt:    []acpsdk.ContentBlock{acpsdk.TextBlock("inspect")},
	}); err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if len(updater.updates) != 3 {
		t.Fatalf("updates = %d, want 3", len(updater.updates))
	}
	start := updater.updates[0].Update.ToolCall
	if start == nil || start.ToolCallId != "tool-1" || start.Title != "read_file" {
		t.Errorf("tool start = %+v", start)
	}
	update := updater.updates[1].Update.ToolCallUpdate
	if update == nil || update.ToolCallId != "tool-1" {
		t.Errorf("tool update = %+v", update)
	}
	if updater.updates[2].Update.AgentMessageChunk == nil {
		t.Error("text event was not forwarded")
	}
}

func assertInvalidParamsError(t *testing.T, err error, want string) {
	t.Helper()
	var requestErr *acpsdk.RequestError
	if !errors.As(err, &requestErr) {
		t.Fatalf("error = %v, want invalid params", err)
	}
	if requestErr.Code != -32602 {
		t.Fatalf("error code = %d, want -32602", requestErr.Code)
	}
	data, ok := requestErr.Data.(map[string]any)
	if !ok || data["error"] != want {
		t.Errorf("error data = %#v, want %q", requestErr.Data, want)
	}
}
