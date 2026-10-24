package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ChillingWombat/AntigravitySwissKnife/antigravity-acp/surface"
)

const mockAGYHelperEnv = "ANTIGRAVITY_ACP_TEST_HELPER"

func TestMain(m *testing.M) {
	if os.Getenv(mockAGYHelperEnv) == "1" {
		os.Exit(runMockAGY())
	}
	os.Exit(m.Run())
}

func runMockAGY() int {
	exe, err := os.Executable()
	if err != nil {
		return 1
	}
	dir := filepath.Dir(exe)
	if _, err := os.Stat(filepath.Join(dir, "record")); err == nil {
		wd, err := os.Getwd()
		if err != nil {
			return 1
		}
		if err := os.WriteFile(filepath.Join(dir, "cwd.txt"), []byte(wd+"\n"), 0644); err != nil {
			return 1
		}
		args, err := os.OpenFile(filepath.Join(dir, "args.txt"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return 1
		}
		if _, err := args.WriteString(strings.Join(os.Args[1:], " ") + "\n"); err != nil {
			args.Close()
			return 1
		}
		if err := args.Close(); err != nil {
			return 1
		}
	}
	if stderr, err := os.ReadFile(filepath.Join(dir, "stderr.txt")); err == nil {
		_, _ = os.Stderr.Write(stderr)
	}
	if stdout, err := os.ReadFile(filepath.Join(dir, "stream.ndjson")); err == nil {
		_, _ = os.Stdout.Write(stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "fail")); err == nil {
		return 7
	}
	return 0
}

func mockAGYBinary(t *testing.T, dir string) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.Open(exe)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()

	name := "agy"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(dir, name)
	dst, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		t.Fatal(err)
	}
	if err := dst.Close(); err != nil {
		t.Fatal(err)
	}
	t.Setenv(mockAGYHelperEnv, "1")
	return path
}

// fakeAGY writes a script that replays recorded stream-json lines.
func fakeAGY(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	// Lines go through a file so no shell can reinterpret the JSON escapes.
	fixture := filepath.Join(dir, "stream.ndjson")
	if err := os.WriteFile(fixture, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return mockAGYBinary(t, dir)
}

// recordingAGY writes a fake CLI that records its argv and working directory.
func recordingAGY(t *testing.T, lines ...string) (bin, argsPath, cwdPath string) {
	t.Helper()
	dir := t.TempDir()
	fixture := filepath.Join(dir, "stream.ndjson")
	if err := os.WriteFile(fixture, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	argsPath = filepath.Join(dir, "args.txt")
	cwdPath = filepath.Join(dir, "cwd.txt")
	if err := os.WriteFile(filepath.Join(dir, "record"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	bin = mockAGYBinary(t, dir)
	return bin, argsPath, cwdPath
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

func TestSendResumesTopLevelConversationAndAppliesSessionOptions(t *testing.T) {
	bin, argsPath, cwdPath := recordingAGY(t, lineInit, lineActive, lineDone, lineResult)
	cwd := t.TempDir()
	s := New(bin)
	sessionID, err := s.Start(context.Background(), surface.SessionOptions{Cwd: cwd, Effort: "high"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := s.SetModel(context.Background(), sessionID, "model-2"); err != nil {
		t.Fatalf("SetModel: %v", err)
	}

	for turn := 0; turn < 2; turn++ {
		if _, err := s.Send(context.Background(), sessionID, "hello", func(surface.Event) error { return nil }); err != nil {
			t.Fatalf("Send turn %d: %v", turn+1, err)
		}
	}

	rawArgs, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	invocations := strings.Split(strings.TrimSpace(string(rawArgs)), "\n")
	if len(invocations) != 2 {
		t.Fatalf("invocations = %d, want 2", len(invocations))
	}
	for _, want := range []string{"--model model-2", "--effort high"} {
		if !strings.Contains(invocations[0], want) {
			t.Errorf("first invocation missing %q: %s", want, invocations[0])
		}
	}
	if !strings.Contains(invocations[1], "--conversation conv-1") {
		t.Errorf("second invocation did not resume conversation: %s", invocations[1])
	}

	gotCwd, err := os.ReadFile(cwdPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(gotCwd)) != cwd {
		t.Errorf("cmd.Dir = %q, want %q", strings.TrimSpace(string(gotCwd)), cwd)
	}
}

func TestSendCapturesTopLevelResultConversationID(t *testing.T) {
	result := `{"event":"result","conversation_id":"conv-result","result":{"status":"SUCCESS"}}`
	bin, argsPath, _ := recordingAGY(t, result)
	s := New(bin)

	for turn := 0; turn < 2; turn++ {
		if _, err := s.Send(context.Background(), "cli-abc", "hello", func(surface.Event) error { return nil }); err != nil {
			t.Fatalf("Send turn %d: %v", turn+1, err)
		}
	}

	rawArgs, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	invocations := strings.Split(strings.TrimSpace(string(rawArgs)), "\n")
	if len(invocations) != 2 || !strings.Contains(invocations[1], "--conversation conv-result") {
		t.Errorf("second invocation did not use result conversation id: %s", strings.Join(invocations, "\n"))
	}
}

func TestSendIncludesStderrOnFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "stderr.txt"), []byte("backend exploded\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fail"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	bin := mockAGYBinary(t, dir)

	s := New(bin)
	_, err := s.Send(context.Background(), "cli-abc", "hello", func(surface.Event) error { return nil })
	if err == nil {
		t.Fatal("expected Send to fail")
	}
	if !strings.Contains(err.Error(), "backend exploded") {
		t.Errorf("error %q does not include stderr", err)
	}
}
