package webgui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetChatMetricsForConversation(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

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

func TestGetChatMetricsForConversation_MultiStepTurnAggregation(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

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
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

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
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

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
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

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

