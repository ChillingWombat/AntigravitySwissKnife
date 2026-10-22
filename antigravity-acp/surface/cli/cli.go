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

	"github.com/ChillingWombat/AntigravitySwissKnife/antigravity-acp/surface"
)

// Binary is overridden in tests and by --agy-binary.
type Binary string

// Surface runs agy turns as subprocesses, one conversation per session id.
type Surface struct {
	Bin string
}

func New(bin string) *Surface {
	if bin == "" {
		bin = "agy"
	}
	return &Surface{Bin: bin}
}

func (s *Surface) Name() string { return "cli" }

// Start mints a session id. The CLI allocates its own conversation id on the
// first turn; Send records it so later turns resume the same conversation.
func (s *Surface) Start(ctx context.Context, opts surface.SessionOptions) (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "cli-" + hex.EncodeToString(buf), nil
}

type streamEvent struct {
	Event       string `json:"event"`
	StepUpdate  *struct {
		ConversationID string  `json:"conversation_id"`
		StepIndex      int     `json:"step_index"`
		State          string  `json:"state"`
		StepType       string  `json:"step_type"`
		TextDelta      string  `json:"text_delta"`
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
	args := []string{"--output-format", "stream-json", "-p", prompt}
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
