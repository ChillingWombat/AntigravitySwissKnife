package webgui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestGetChatMetricsForConversation(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	convID := "test-conv-metrics"
	brainLogDir := filepath.Join(tmpDir, ".gemini", "antigravity", "brain", convID, ".system_generated", "logs")
	if err := os.MkdirAll(brainLogDir, 0755); err != nil {
		t.Fatal(err)
	}

	transcriptContent := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-10T10:00:00Z","content":"hello"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-10T10:00:02Z","input_tokens":1000,"cache_read_tokens":500,"output_tokens":200,"duration_ms":2000}
{"step_index":2,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-10T10:00:05Z","content":"next"}
{"step_index":3,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-10T10:00:07Z","input_tokens":2000,"cache_read_tokens":1000,"output_tokens":300,"duration_ms":3000}
`
	if err := os.WriteFile(filepath.Join(brainLogDir, "transcript.jsonl"), []byte(transcriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := GetChatMetricsForConversation(convID, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatal("expected success true")
	}
	if len(res.Turns) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(res.Turns))
	}

	// Turn 1 check: total input = 1000 + 500 = 1500, cached = 500 => hit ratio = 33.3%
	t1 := res.Turns[0]
	if t1.InputTokens != 1500 || t1.OutputTokens != 200 || t1.CachedTokens != 500 {
		t.Errorf("turn 1 metrics mismatch: %+v", t1)
	}
	if t1.SpeedTPS != 100.0 { // 200 / 2.0s = 100 TPS
		t.Errorf("expected 100 TPS, got %f", t1.SpeedTPS)
	}

	// Summary check: total in = 1500 + 3000 = 4500, total out = 500, cached = 1500 => hit ratio = 33.3%
	if res.Summary.InputTokens != 4500 {
		t.Errorf("expected 4500 total input tokens, got %d", res.Summary.InputTokens)
	}
	if res.Summary.OutputTokens != 500 {
		t.Errorf("expected 500 total output tokens, got %d", res.Summary.OutputTokens)
	}
	if res.Summary.GenerationSpeed != 100.0 { // 500 / 5.0s = 100 TPS
		t.Errorf("expected 100 TPS summary speed, got %f", res.Summary.GenerationSpeed)
	}
}

func TestGetChatMetricsForConversation_UserPromptCountIgnoresExecutionTurns(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	convID := "test-conv-prompt-count"
	brainLogDir := filepath.Join(tmpDir, ".gemini", "antigravity", "brain", convID, ".system_generated", "logs")
	if err := os.MkdirAll(brainLogDir, 0755); err != nil {
		t.Fatal(err)
	}

	// One real user prompt drives several assistant segments separated by system
	// boundaries. Those execution turns are useful for telemetry, but must not
	// become Chat Navigator rows.
	transcriptContent := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-21T05:00:00Z","content":"run the task"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-21T05:00:01Z","input_tokens":1000,"output_tokens":100,"duration_ms":1000}
{"step_index":2,"source":"SYSTEM","type":"SYSTEM_MESSAGE","status":"DONE","created_at":"2026-10-21T05:00:02Z","content":"subagent boundary"}
{"step_index":3,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-21T05:00:03Z","input_tokens":1100,"output_tokens":100,"duration_ms":1000}
{"step_index":4,"source":"SYSTEM","type":"SYSTEM_MESSAGE","status":"DONE","created_at":"2026-10-21T05:00:04Z","content":"subagent boundary"}
{"step_index":5,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-21T05:00:05Z","input_tokens":1200,"output_tokens":100,"duration_ms":1000}
`
	if err := os.WriteFile(filepath.Join(brainLogDir, "transcript.jsonl"), []byte(transcriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := GetChatMetricsForConversation(convID, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Turns) != 3 {
		t.Fatalf("expected 3 execution turns, got %d", len(res.Turns))
	}
	if res.UserPromptCount != 1 {
		t.Fatalf("expected 1 user prompt, got %d", res.UserPromptCount)
	}
}

func TestGetChatMetricsForConversation_MultiStepTurnAggregation(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	convID := "test-conv-multistep"
	brainLogDir := filepath.Join(tmpDir, ".gemini", "antigravity", "brain", convID, ".system_generated", "logs")
	if err := os.MkdirAll(brainLogDir, 0755); err != nil {
		t.Fatal(err)
	}

	transcriptContent := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-10T10:00:00Z","content":"please investigate"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-10T10:00:02Z","input_tokens":1000,"cache_read_tokens":500,"output_tokens":126,"duration_ms":1000}
{"step_index":2,"source":"MODEL","type":"GENERIC","status":"DONE","created_at":"2026-10-10T10:00:03Z","content":"tool result"}
{"step_index":3,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-10T10:00:05Z","input_tokens":1200,"cache_read_tokens":600,"output_tokens":200,"duration_ms":1000}
{"step_index":4,"source":"MODEL","type":"GENERIC","status":"DONE","created_at":"2026-10-10T10:00:06Z","content":"tool result 2"}
{"step_index":5,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-10T10:00:08Z","input_tokens":1500,"cache_read_tokens":800,"output_tokens":500,"duration_ms":2000}
{"step_index":6,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-10T10:00:10Z","content":"second question"}
{"step_index":7,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-10T10:00:12Z","input_tokens":2000,"cache_read_tokens":1000,"output_tokens":300,"duration_ms":1000}
`
	if err := os.WriteFile(filepath.Join(brainLogDir, "transcript.jsonl"), []byte(transcriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := GetChatMetricsForConversation(convID, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatal("expected success true")
	}
	if len(res.Turns) != 2 {
		t.Fatalf("expected 2 turns after aggregating multi-step tool calls, got %d", len(res.Turns))
	}

	// Turn 1 must aggregate steps 1, 3, and 5:
	// Output tokens = 126 + 200 + 500 = 826
	// Input tokens represents active prompt tokens of the latest step in the turn: 1500 + 800 = 2300 (not repeated prompt summing)
	// Cached tokens represents latest step cache: 800
	// Context tokens represents latest step context window: 2300 + 500 = 2800
	t1 := res.Turns[0]
	if t1.OutputTokens != 826 {
		t.Errorf("expected aggregated turn 1 output tokens 826, got %d", t1.OutputTokens)
	}
	if t1.InputTokens != 2300 {
		t.Errorf("expected active prompt turn 1 input tokens 2300, got %d", t1.InputTokens)
	}
	if t1.CachedTokens != 800 {
		t.Errorf("expected turn 1 cached tokens 800, got %d", t1.CachedTokens)
	}
	if t1.ContextTokens != 2800 {
		t.Errorf("expected turn 1 context tokens 2800, got %d", t1.ContextTokens)
	}

	// Turn 2 check: output = 300, input = 3000
	t2 := res.Turns[1]
	if t2.OutputTokens != 300 {
		t.Errorf("expected turn 2 output tokens 300, got %d", t2.OutputTokens)
	}
	if t2.InputTokens != 3000 {
		t.Errorf("expected turn 2 input tokens 3000, got %d", t2.InputTokens)
	}
}

func TestGetChatMetricsForConversation_TimestampDurationInference(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	convID := "test-conv-timestamps"
	brainLogDir := filepath.Join(tmpDir, ".gemini", "antigravity", "brain", convID, ".system_generated", "logs")
	if err := os.MkdirAll(brainLogDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Transcript without any explicit duration_ms fields
	transcriptContent := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-10T10:00:00Z","content":"first request"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-10T10:00:04Z","input_tokens":1000,"cache_read_tokens":500,"output_tokens":320}
{"step_index":2,"source":"MODEL","type":"GENERIC","status":"DONE","created_at":"2026-10-10T10:00:07Z","content":"tool result"}
{"step_index":3,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-10T10:00:09Z","input_tokens":1200,"cache_read_tokens":600,"output_tokens":160}
{"step_index":4,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-10T10:00:15Z","content":"second request"}
{"step_index":5,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-10T10:00:17Z","input_tokens":2000,"cache_read_tokens":1000,"output_tokens":180}
`
	if err := os.WriteFile(filepath.Join(brainLogDir, "transcript.jsonl"), []byte(transcriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := GetChatMetricsForConversation(convID, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatal("expected success true")
	}
	if len(res.Turns) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(res.Turns))
	}

	// Turn 1:
	// Step 1: 10:00:04 - 10:00:00 = 4s (4000ms), 320 output tokens
	// Step 2: 10:00:07 (tool execution)
	// Step 3: 10:00:09 - 10:00:07 = 2s (2000ms), 160 output tokens
	// Total turn 1: output = 480 tokens, duration = 6000ms => SpeedTPS = 480 / 6 = 80.0 TPS
	t1 := res.Turns[0]
	if t1.OutputTokens != 480 {
		t.Errorf("expected turn 1 output 480, got %d", t1.OutputTokens)
	}
	if t1.SpeedTPS != 80.0 {
		t.Errorf("expected genuine inferred 80.0 TPS, got %f", t1.SpeedTPS)
	}

	// Turn 2:
	// Step 5: 10:00:17 - 10:00:15 = 2s (2000ms), 180 output tokens => SpeedTPS = 180 / 2 = 90.0 TPS
	t2 := res.Turns[1]
	if t2.OutputTokens != 180 {
		t.Errorf("expected turn 2 output 180, got %d", t2.OutputTokens)
	}
	if t2.SpeedTPS != 90.0 {
		t.Errorf("expected genuine inferred 90.0 TPS, got %f", t2.SpeedTPS)
	}

	// Summary: total out = 660, total duration = 8000ms => GenerationSpeed = 660 / 8 = 82.5 TPS
	if res.Summary.GenerationSpeed != 82.5 {
		t.Errorf("expected genuine summary speed 82.5 TPS, got %f", res.Summary.GenerationSpeed)
	}
}

func TestGetChatMetricsForConversation_ZeroDurationNoFabrication(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	convID := "test-conv-zerodur"
	brainLogDir := filepath.Join(tmpDir, ".gemini", "antigravity", "brain", convID, ".system_generated", "logs")
	if err := os.MkdirAll(brainLogDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Steps logged within identical timestamp, no duration reported
	transcriptContent := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-10T10:00:00Z","content":"hello"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-10T10:00:00Z","input_tokens":1000,"cache_read_tokens":500,"output_tokens":200}
`
	if err := os.WriteFile(filepath.Join(brainLogDir, "transcript.jsonl"), []byte(transcriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := GetChatMetricsForConversation(convID, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(res.Turns))
	}

	// Must be 0.0 TPS and NOT fabricated to 76.2
	if res.Turns[0].SpeedTPS != 0.0 {
		t.Errorf("expected 0.0 TPS when duration unavailable, got %f", res.Turns[0].SpeedTPS)
	}
	if res.Summary.GenerationSpeed != 0.0 {
		t.Errorf("expected 0.0 GenerationSpeed when duration unavailable, got %f", res.Summary.GenerationSpeed)
	}
}

func TestGetChatMetricsForConversation_RealWorldStepStartTimestamps(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	convID := "test-conv-realworld-timestamps"
	brainLogDir := filepath.Join(tmpDir, ".gemini", "antigravity", "brain", convID, ".system_generated", "logs")
	if err := os.MkdirAll(brainLogDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Real-world pattern: PLANNER_RESPONSE has same created_at as previous tool output,
	// and the NEXT tool execution step has the timestamp when generation completed.
	transcriptContent := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-10T12:00:00Z","content":"analyze"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-10T12:00:00Z","input_tokens":1000,"cache_read_tokens":4000,"output_tokens":250}
{"step_index":2,"source":"MODEL","type":"GENERIC","status":"DONE","created_at":"2026-10-10T12:00:05Z","content":"result 1"}
{"step_index":3,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-10T12:00:05Z","input_tokens":1200,"cache_read_tokens":4500,"output_tokens":350}
{"step_index":4,"source":"MODEL","type":"GENERIC","status":"DONE","created_at":"2026-10-10T12:00:10Z","content":"result 2"}
{"step_index":5,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-10T12:00:15Z","content":"thanks"}
`
	if err := os.WriteFile(filepath.Join(brainLogDir, "transcript.jsonl"), []byte(transcriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := GetChatMetricsForConversation(convID, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(res.Turns))
	}

	turn := res.Turns[0]
	// Step 1: 12:00:00 to 12:00:05 = 5s (5000ms), 250 output tokens
	// Step 3: 12:00:05 to 12:00:10 = 5s (5000ms), 350 output tokens
	// Total duration = 10s (10000ms), total output = 600 tokens => SpeedTPS = 600 / 10 = 60.0 TPS
	if turn.OutputTokens != 600 {
		t.Errorf("expected 600 output tokens, got %d", turn.OutputTokens)
	}
	if turn.SpeedTPS != 60.0 {
		t.Errorf("expected 60.0 TPS, got %f", turn.SpeedTPS)
	}
	// Prompt tokens of the turn must represent latest step's prompt context (1200 + 4500 = 5700), not additive sum (10700)
	if turn.InputTokens != 5700 {
		t.Errorf("expected 5700 input tokens, got %d", turn.InputTokens)
	}
	// Context tokens must represent latest step's context window (5700 + 350 = 6050)
	if turn.ContextTokens != 6050 {
		t.Errorf("expected 6050 context tokens, got %d", turn.ContextTokens)
	}
	if res.Summary.ContextTokens != 6050 {
		t.Errorf("expected summary context tokens 6050, got %d", res.Summary.ContextTokens)
	}
}

func TestExtractUserPromptText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "wrapped request drops the metadata tail",
			in:   "<USER_REQUEST>\nwhat code should i write for q1?\n</USER_REQUEST>\n<ADDITIONAL_METADATA>\nThe current local time is: 2026-10-21\n</ADDITIONAL_METADATA>",
			want: "what code should i write for q1?",
		},
		{
			name: "unwrapped payload keeps its words",
			in:   "plain prompt with no envelope",
			want: "plain prompt with no envelope",
		},
		{
			name: "collapses every whitespace run to one space",
			in:   "<USER_REQUEST>\r\n\tfirst line\n\n   second line\t</USER_REQUEST>",
			want: "first line second line",
		},
		{
			name: "multi-line code block survives as one line",
			in:   "<USER_REQUEST>\nfix this:\n```go\nf()\n```\n</USER_REQUEST>",
			want: "fix this: ```go f() ```",
		},
		{
			name: "empty payload stays empty",
			in:   "",
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExtractUserPromptText(tc.in); got != tc.want {
				t.Errorf("ExtractUserPromptText(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestExtractUserPromptText_TruncatesOnRuneBoundary(t *testing.T) {
	long := strings.Repeat("字", chatPromptTextLimit)
	got := ExtractUserPromptText("<USER_REQUEST>" + long + "</USER_REQUEST>")
	if !utf8.ValidString(got) {
		t.Fatalf("truncated prompt must stay valid UTF-8, got %q", got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncated prompt must end with an ellipsis, got %q", got)
	}
	if utf8.RuneCountInString(got) > chatPromptTextLimit+1 {
		t.Errorf("truncated prompt must stay within the limit, got %d runes", utf8.RuneCountInString(got))
	}
}

func TestGetChatMetricsForConversation_PromptsFollowTurnOrder(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	convID := "test-conv-prompts"
	brainLogDir := filepath.Join(tmpDir, ".gemini", "antigravity", "brain", convID, ".system_generated", "logs")
	if err := os.MkdirAll(brainLogDir, 0755); err != nil {
		t.Fatal(err)
	}

	transcriptContent := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-21T05:00:00Z","content":"<USER_REQUEST>first prompt</USER_REQUEST><ADDITIONAL_METADATA>time</ADDITIONAL_METADATA>"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-21T05:00:01Z","input_tokens":10,"output_tokens":5,"duration_ms":1000}
{"step_index":2,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-21T05:00:05Z","content":"<USER_REQUEST>second prompt</USER_REQUEST>"}
{"step_index":3,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-21T05:00:06Z","input_tokens":20,"output_tokens":5,"duration_ms":1000}
`
	if err := os.WriteFile(filepath.Join(brainLogDir, "transcript.jsonl"), []byte(transcriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := GetChatMetricsForConversation(convID, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d (%+v)", len(res.Prompts), res.Prompts)
	}
	if res.Prompts[0].Index != 0 || res.Prompts[0].Text != "first prompt" {
		t.Errorf("unexpected prompt 0: %+v", res.Prompts[0])
	}
	if res.Prompts[1].Index != 1 || res.Prompts[1].Text != "second prompt" {
		t.Errorf("unexpected prompt 1: %+v", res.Prompts[1])
	}
	if res.UserPromptCount != len(res.Prompts) {
		t.Errorf("prompt list (%d) must match user_prompt_count (%d)", len(res.Prompts), res.UserPromptCount)
	}
}
