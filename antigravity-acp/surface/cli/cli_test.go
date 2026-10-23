package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ChillingWombat/AntigravitySwissKnife/antigravity-acp/surface"
)

// fakeAGY writes a script that replays recorded stream-json lines.
func fakeAGY(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	// Lines go through a file so no shell can reinterpret the JSON escapes.
	fixture := filepath.Join(dir, "stream.ndjson")
	if err := os.WriteFile(fixture, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "agy")
	body := "#!/bin/sh\ncat '" + fixture + "'\n"
	if err := os.WriteFile(path, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

// Lines recorded from a real `agy --output-format stream-json -p` run.
const (
	lineInit   = `{"event":"init","conversation_id":"conv-1","init":{"cwd":"/tmp"}}`
	lineActive = `{"event":"step_update","step_update":{"conversation_id":"conv-1","step_index":1,"state":"ACTIVE","step_type":"agent_response","text_delta":"stream-test"}}`
	lineDone   = `{"event":"step_update","step_update":{"conversation_id":"conv-1","step_index":1,"state":"DONE","step_type":"agent_response","text_delta":"\n","duration_seconds":6.0}}`
	lineResult = `{"event":"result","result":{"conversation_id":"conv-1","status":"SUCCESS","response":"stream-test\n"}}`
)

func TestSendStreamsTextDeltas(t *testing.T) {
	s := New(fakeAGY(t, lineInit, lineActive, lineDone, lineResult))
	var got []string
	reason, err := s.Send(context.Background(), "cli-abc", "hi", func(ev surface.Event) error {
		if ev.Kind == surface.EventText {
			got = append(got, ev.Text)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if reason != surface.StopEndTurn {
		t.Errorf("stop reason = %q, want end_turn", reason)
	}
	want := []string{"stream-test", "\n"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("deltas = %q, want %q", got, want)
	}
}

func TestSendReportsFailureStatus(t *testing.T) {
	failed := `{"event":"result","result":{"conversation_id":"conv-1","status":"ERROR"}}`
	s := New(fakeAGY(t, lineInit, failed))
	reason, err := s.Send(context.Background(), "cli-abc", "hi", func(surface.Event) error { return nil })
	if err == nil {
		t.Fatal("expected an error for a failed agy status")
	}
	if reason != surface.StopRefusal {
		t.Errorf("stop reason = %q, want refusal", reason)
	}
}

func TestConversationResumeID(t *testing.T) {
	if got := conversationFromSession("cli-abc:conv-1"); got != "conv-1" {
		t.Errorf("resume id = %q, want conv-1", got)
	}
	if got := conversationFromSession("cli-abc"); got != "" {
		t.Errorf("fresh session must not carry a conversation id, got %q", got)
	}
}
