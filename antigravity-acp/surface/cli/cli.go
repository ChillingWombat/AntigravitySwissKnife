// Package cli drives the Antigravity CLI (agy) through its stream-json mode.
package cli

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/ChillingWombat/AntigravitySwissKnife/antigravity-acp/surface"
)

// Binary is overridden in tests and by --agy-binary.
type Binary string

// Surface runs agy turns as subprocesses, one conversation per session id.
type Surface struct {
	Bin string

	mu       sync.Mutex
	sessions map[string]surface.SessionOptions
	convIDs  map[string]string
}

func New(bin string) *Surface {
	if bin == "" {
		bin = "agy"
	}
	return &Surface{Bin: bin, sessions: map[string]surface.SessionOptions{}, convIDs: map[string]string{}}
}

func (s *Surface) Name() string { return "cli" }

// Start mints a session id. The CLI allocates its own conversation id on the
// first turn; Send records it so later turns resume the same conversation.
func (s *Surface) Start(ctx context.Context, opts surface.SessionOptions) (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	id := "cli-" + hex.EncodeToString(buf)
	s.mu.Lock()
	s.sessions[id] = opts
	s.mu.Unlock()
	return id, nil
}

type streamEvent struct {
	Event      string `json:"event"`
	StepUpdate *struct {
		ConversationID string `json:"conversation_id"`
		StepIndex      int    `json:"step_index"`
		State          string `json:"state"`
		StepType       string `json:"step_type"`
		TextDelta      string `json:"text_delta"`
	} `json:"step_update"`
	Init *struct {
		ConversationID string `json:"conversation_id"`
	} `json:"init"`
	Result *struct {
		ConversationID string `json:"conversation_id"`
		Status         string `json:"status"`
		Response       string `json:"response"`
	} `json:"result"`
}

// Send runs one turn and streams text deltas as they arrive.
func (s *Surface) Send(ctx context.Context, sessionID, prompt string, emit func(surface.Event) error) (surface.StopReason, error) {
	args := []string{"--output-format", "stream-json"}
	if conv := s.conversationFor(sessionID); conv != "" {
		args = append(args, "--conversation", conv)
	}
	// Execution mode decides whether the agent may modify files. Without an
	// explicit choice agy blocks on its own permission prompt, which the stream
	// cannot answer.
	mode := s.modeFor(sessionID)
	args = append(args, "--mode", mode)
	if mode != "plan" {
		args = append(args, "--dangerously-skip-permissions")
	}
	args = append(args, "-p", prompt)
	if conv := conversationFromSession(sessionID); conv != "" {
		args = append(args, "--conversation", conv)
	}
	cmd := exec.CommandContext(ctx, s.Bin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return surface.StopReason(""), err
	}
	if err := cmd.Start(); err != nil {
		return surface.StopReason(""), fmt.Errorf("start agy: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64<<10), 8<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev streamEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue // unknown line: skip rather than fail the turn
		}
		switch ev.Event {
		case "init":
			// The CLI allocates the conversation id; remember it so the next turn
			// resumes this conversation instead of starting a fresh one.
			if ev.Init != nil && ev.Init.ConversationID != "" {
				s.mu.Lock()
				s.convIDs[sessionID] = ev.Init.ConversationID
				s.mu.Unlock()
			}
		case "step_update":
			if ev.StepUpdate == nil || ev.StepUpdate.TextDelta == "" {
				continue
			}
			if err := emit(surface.Event{Kind: surface.EventText, Text: ev.StepUpdate.TextDelta}); err != nil {
				_ = cmd.Process.Kill()
				return surface.StopCancelled, err
			}
		case "result":
			if ev.Result != nil && ev.Result.Status != "" && ev.Result.Status != "SUCCESS" {
				return surface.StopRefusal, errors.New("agy status " + ev.Result.Status)
			}
		}
	}
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return surface.StopCancelled, nil
		}
		return surface.StopReason(""), fmt.Errorf("agy turn failed: %w", err)
	}
	return surface.StopEndTurn, nil
}

// modeFor returns the execution mode requested for a session. Edits are the
// default because the point of the adapter is a working agent; plan mode is the
// explicit opt-out.
// conversationFor returns the CLI conversation bound to a session, if any.
func (s *Surface) conversationFor(sessionID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.convIDs[sessionID]
}

func (s *Surface) modeFor(sessionID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if mode := s.sessions[sessionID].Mode; mode != "" {
		return mode
	}
	return "accept-edits"
}

func (s *Surface) Cancel(ctx context.Context, sessionID string) error { return nil }

func (s *Surface) List(ctx context.Context) ([]surface.SessionInfo, error) { return nil, nil }

func (s *Surface) Close(ctx context.Context, sessionID string) error { return nil }

// conversationFromSession extracts the agy conversation id embedded in a session
// id, if one has been recorded.
func conversationFromSession(sessionID string) string {
	if idx := strings.Index(sessionID, ":"); idx >= 0 {
		return sessionID[idx+1:]
	}
	return ""
}
