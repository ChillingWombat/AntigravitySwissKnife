// Package surface defines what an Antigravity surface must provide so one ACP
// agent can drive the CLI, the desktop app, or an editor integration.
package surface

import "context"

// EventKind separates the things a surface can report during a turn.
type EventKind int

const (
	EventText EventKind = iota
	EventToolStart
	EventToolUpdate
	EventError
	// EventUserText is a user turn replayed from history. Surfaces emit it only
	// from History, never from Send, so live turns stay one-directional.
	EventUserText
)

// Event is the adapter's own vocabulary. Translation into ACP updates lives in
// acp/translate so surfaces never import protocol types.
type Event struct {
	Kind    EventKind
	Text    string
	ToolID  string
	Tool    string
	State   string
	Message string
}

// StopReason explains why a turn ended.
type StopReason string

const (
	StopEndTurn   StopReason = "end_turn"
	StopCancelled StopReason = "cancelled"
	StopRefusal   StopReason = "refusal"
	StopMaxTokens StopReason = "max_tokens"
)

// SessionOptions is what the client asked for when opening a session.
type SessionOptions struct {
	Cwd    string
	Model  string
	Effort string
	Mode   string
}

// SessionInfo is one conversation a surface knows about.
type SessionInfo struct {
	ID        string
	Title     string
	UpdatedAt int64
	Cwd       string
}

// ModelSelector is implemented by surfaces that can change the model of an open
// session. Selection goes through the adapter because the Antigravity picker is fed
// by the server's own catalogue and cannot list gateway models.
type ModelSelector interface {
	SetModel(ctx context.Context, sessionID, model string) error
}

// HistoryProvider is implemented by surfaces that can replay a past conversation.
// It exists as a separate interface so a surface without history is not forced to
// pretend it has any.
type HistoryProvider interface {
	History(ctx context.Context, sessionID string) ([]Event, error)
}

// SessionLister is implemented by surfaces that can enumerate stored sessions.
type SessionLister interface {
	List(ctx context.Context) ([]SessionInfo, error)
}

// Surface is one way of talking to Antigravity. Implementations own their
// transport; the ACP layer only sees Send and its event stream.
type Surface interface {
	// Name identifies the surface in logs and capabilities, e.g. "cli".
	Name() string
	// Start opens a conversation and returns its stable id.
	Start(ctx context.Context, opts SessionOptions) (string, error)
	// Send runs one turn. emit is called in order until the turn ends or ctx is
	// cancelled. Implementations must not call emit after returning.
	Send(ctx context.Context, sessionID, prompt string, emit func(Event) error) (StopReason, error)
	// Cancel stops an in-flight turn without killing the session if the surface
	// supports that; otherwise it terminates the process behind the session.
	Cancel(ctx context.Context, sessionID string) error
	// Close releases the session.
	Close(ctx context.Context, sessionID string) error
}
