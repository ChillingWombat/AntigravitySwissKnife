package webgui

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestChallenger_ZeroDuration tests edge cases where duration is zero or missing.
func TestChallenger_ZeroDuration(t *testing.T) {
	tmpDir := t.TempDir()
	tPath := filepath.Join(tmpDir, "transcript.jsonl")

	// Case A: explicit duration_ms = 0
	step1 := `{"step_index":0,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-10-10T10:00:00Z","duration_ms":0,"input_tokens":1000,"output_tokens":500}`
	// Case B: missing duration_ms and no next event
	step2 := `{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-10-10T10:00:05Z","input_tokens":1000,"output_tokens":300}`

	content := strings.Join([]string{step1, step2}, "\n") + "\n"
	if err := os.WriteFile(tPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write transcript: %v", err)
	}

	info, err := os.Stat(tPath)
	if err != nil {
		t.Fatalf("failed to stat transcript: %v", err)
	}

	stats := parseTranscriptFile(tPath, info)

	// Step 1: lookahead between 10:00:00 and 10:00:05 is 5000ms. 500 / 5s = 100 TPS.
	// Step 2: has no subsequent step, duration is 0, unmeasured.
	if stats.outputTokens != 800 {
		t.Errorf("expected 800 total output tokens, got %d", stats.outputTokens)
	}
	if stats.timedOutputTokens != 500 {
		t.Errorf("expected 500 timed output tokens, got %d", stats.timedOutputTokens)
	}

	// Verify no NaN or Inf
	var tps float64
	if stats.totalDurationMs > 0 {
		tps = float64(stats.timedOutputTokens) / (float64(stats.totalDurationMs) / 1000.0)
	}
	if math.IsNaN(tps) || math.IsInf(tps, 0) {
		t.Fatalf("TPS produced NaN or Inf: %v", tps)
	}
	if tps < 0 || tps > 350.0 {
		t.Errorf("TPS out of realistic bounds [0, 350]: %v", tps)
	}
}

// TestChallenger_SingleStepConversation tests conversations that end after a single step.
func TestChallenger_SingleStepConversation(t *testing.T) {
	tmpDir := t.TempDir()
	convID := "single-step-conv"
	brainLogDir := filepath.Join(tmpDir, ".gemini", "antigravity", "brain", convID, ".system_generated", "logs")
	if err := os.MkdirAll(brainLogDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Single step planner response with no duration_ms and no subsequent events
	singleStep := `{"step_index":0,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-10-10T12:00:00Z","input_tokens":5000,"output_tokens":1200}`
	tPath := filepath.Join(brainLogDir, "transcript.jsonl")
	if err := os.WriteFile(tPath, []byte(singleStep+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(tPath)
	if err != nil {
		t.Fatal(err)
	}

	// Test parseTranscriptFile
	stats := parseTranscriptFile(tPath, info)
	if stats.outputTokens != 1200 {
		t.Errorf("expected 1200 total output tokens, got %d", stats.outputTokens)
	}
	if stats.timedOutputTokens != 0 {
		t.Errorf("single step without duration should have 0 timed tokens, got %d", stats.timedOutputTokens)
	}
	if stats.totalDurationMs != 0 {
		t.Errorf("single step without duration should have 0 total duration, got %d", stats.totalDurationMs)
	}

	// Test GetChatMetricsForConversation
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	metrics, err := GetChatMetricsForConversation(convID, "main")
	if err != nil {
		t.Fatalf("GetChatMetricsForConversation failed: %v", err)
	}
	if !metrics.Success {
		t.Fatal("expected success true")
	}
	if metrics.Summary.GenerationSpeed != 0.0 {
		t.Errorf("expected 0.0 generation speed for single unmeasured step, got %f", metrics.Summary.GenerationSpeed)
	}
	if math.IsNaN(metrics.Summary.GenerationSpeed) || math.IsInf(metrics.Summary.GenerationSpeed, 0) {
		t.Fatalf("generation speed produced NaN or Inf: %v", metrics.Summary.GenerationSpeed)
	}
}

// TestChallenger_MassiveOutputBursts tests extreme bursts (e.g. 50,000 tokens in 0.1s).
func TestChallenger_MassiveOutputBursts(t *testing.T) {
	testCases := []struct {
		name         string
		tokens       int64
		durationMs   int64
		lookaheadMs  int64
		hasExplicitD bool
	}{
		{
			name:         "50k tokens in 100ms explicit duration",
			tokens:       50000,
			durationMs:   100,
			hasExplicitD: true,
		},
		{
			name:         "50k tokens in 100ms lookahead",
			tokens:       50000,
			lookaheadMs:  100,
			hasExplicitD: false,
		},
		{
			name:         "100k tokens in 1ms explicit duration",
			tokens:       100000,
			durationMs:   1,
			hasExplicitD: true,
		},
		{
			name:         "10M tokens in 50ms explicit duration",
			tokens:       10000000,
			durationMs:   50,
			hasExplicitD: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tPath := filepath.Join(tmpDir, "transcript.jsonl")

			t0 := time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)
			var lines []string

			if tc.hasExplicitD {
				step := fmt.Sprintf(`{"step_index":0,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"%s","duration_ms":%d,"input_tokens":1000,"output_tokens":%d}`,
					t0.Format(time.RFC3339Nano), tc.durationMs, tc.tokens)
				lines = append(lines, step)
			} else {
				step0 := fmt.Sprintf(`{"step_index":0,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"%s","input_tokens":1000,"output_tokens":%d}`,
					t0.Format(time.RFC3339Nano), tc.tokens)
				step1Time := t0.Add(time.Duration(tc.lookaheadMs) * time.Millisecond)
				step1 := fmt.Sprintf(`{"step_index":1,"source":"MODEL","type":"GENERIC","created_at":"%s"}`,
					step1Time.Format(time.RFC3339Nano))
				lines = append(lines, step0, step1)
			}

			if err := os.WriteFile(tPath, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
				t.Fatal(err)
			}

			info, err := os.Stat(tPath)
			if err != nil {
				t.Fatal(err)
			}

			stats := parseTranscriptFile(tPath, info)

			if stats.totalDurationMs > 0 && stats.timedOutputTokens > 0 {
				tps := float64(stats.timedOutputTokens) / (float64(stats.totalDurationMs) / 1000.0)
				if math.IsNaN(tps) || math.IsInf(tps, 0) {
					t.Fatalf("TPS produced NaN or Inf: %v", tps)
				}
				if tps > 350.0 {
					t.Fatalf("CRITICAL BOUND EXCEEDED: TPS %f > 350.0 limit on massive burst!", tps)
				}
				if tps < 0 {
					t.Fatalf("TPS is negative: %f", tps)
				}
			}

			// Also verify recentSteps duration and TPS
			for idx, step := range stats.recentSteps {
				if step.durationMs > 0 && step.output > 0 {
					stepTPS := float64(step.output) / (float64(step.durationMs) / 1000.0)
					if math.IsNaN(stepTPS) || math.IsInf(stepTPS, 0) {
						t.Fatalf("step %d TPS is NaN or Inf: %v", idx, stepTPS)
					}
					if stepTPS > 350.0 {
						t.Fatalf("step %d TPS %f > 350.0", idx, stepTPS)
					}
				}
			}
		})
	}
}

// TestChallenger_StalledTurns tests turns with 0 tokens or long 1-hour delays.
func TestChallenger_StalledTurns(t *testing.T) {
	// Subtest 1: Lookahead with 1-hour stall between steps without explicit duration_ms
	t.Run("Lookahead_1HourStall_Excluded", func(t *testing.T) {
		tmpDir := t.TempDir()
		tPath := filepath.Join(tmpDir, "transcript.jsonl")

		t0 := time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)
		t1 := t0.Add(1 * time.Hour) // 1 hour stall

		step1 := fmt.Sprintf(`{"step_index":0,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"%s","input_tokens":5000,"output_tokens":100}`,
			t0.Format(time.RFC3339Nano))
		step2 := fmt.Sprintf(`{"step_index":1,"source":"USER","type":"USER_INPUT","created_at":"%s","content":"waking up"}`,
			t1.Format(time.RFC3339Nano))

		content := strings.Join([]string{step1, step2}, "\n") + "\n"
		if err := os.WriteFile(tPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		info, err := os.Stat(tPath)
		if err != nil {
			t.Fatal(err)
		}

		stats := parseTranscriptFile(tPath, info)

		// 1-hour lookahead diff (3600s) exceeds 600,000ms limit, so it MUST be excluded
		if stats.timedOutputTokens != 0 {
			t.Errorf("expected 0 timed output tokens for stalled steps, got %d", stats.timedOutputTokens)
		}
		if stats.totalDurationMs != 0 {
			t.Errorf("expected 0 total duration for stalled steps, got %d", stats.totalDurationMs)
		}
	})

	// Subtest 2: Stalled turn with 0 output tokens and explicit 1-hour duration (duration_ms: 3600000)
	// paired with an authentic 100 TPS step (1000 tokens in 10s).
	t.Run("ZeroTokens_1HourDuration_PollutionVulnerability", func(t *testing.T) {
		tmpDir := t.TempDir()
		tPath := filepath.Join(tmpDir, "transcript.jsonl")

		t0 := time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)
		t1 := t0.Add(1 * time.Minute)

		// Step 0: Tool execution / stalled step with 0 output tokens and 1 hour duration
		step0 := fmt.Sprintf(`{"step_index":0,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"%s","duration_ms":3600000,"input_tokens":5000,"output_tokens":0}`,
			t0.Format(time.RFC3339Nano))
		// Step 1: Authentic generation step: 1000 output tokens in 10,000ms (100 TPS)
		step1 := fmt.Sprintf(`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"%s","duration_ms":10000,"input_tokens":5000,"output_tokens":1000}`,
			t1.Format(time.RFC3339Nano))

		content := strings.Join([]string{step0, step1}, "\n") + "\n"
		if err := os.WriteFile(tPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		info, err := os.Stat(tPath)
		if err != nil {
			t.Fatal(err)
		}

		stats := parseTranscriptFile(tPath, info)

		t.Logf("Stats: totalDurationMs=%dms, timedOutputTokens=%d", stats.totalDurationMs, stats.timedOutputTokens)

		if stats.timedOutputTokens != 1000 {
			t.Errorf("expected 1000 timed output tokens, got %d", stats.timedOutputTokens)
		}

		// Vulnerability check:
		// If line 411 adds step0's 3600000ms duration despite 0 output tokens,
		// totalDurationMs becomes 3610000ms, and TPS becomes 1000 / 3610s = 0.27 TPS!
		// Genuine token generation duration was only 10,000ms (100 TPS).
		tps := float64(stats.timedOutputTokens) / (float64(stats.totalDurationMs) / 1000.0)
		t.Logf("Calculated conversation TPS: %.3f", tps)

		if stats.totalDurationMs > 10000 {
			t.Errorf("VULNERABILITY CONFIRMED: 0-token stalled step polluted totalDurationMs by %dms! TPS collapsed from 100.0 TPS to %.3f TPS due to line 411 else if durMs > 0",
				stats.totalDurationMs-10000, tps)
		}
	})
}

// TestChallenger_OutOfOrderTimestamps tests transcripts with reversed or skewed timestamps.
func TestChallenger_OutOfOrderTimestamps(t *testing.T) {
	tmpDir := t.TempDir()
	tPath := filepath.Join(tmpDir, "transcript.jsonl")

	t0 := time.Date(2026, 10, 10, 10, 0, 10, 0, time.UTC)
	t1 := time.Date(2026, 10, 10, 10, 0, 5, 0, time.UTC) // 5 seconds in the past!
	t2 := time.Date(2026, 10, 10, 9, 50, 0, 0, time.UTC)  // 10 minutes in the past!

	step0 := fmt.Sprintf(`{"step_index":0,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"%s","input_tokens":1000,"output_tokens":500}`,
		t0.Format(time.RFC3339Nano))
	step1 := fmt.Sprintf(`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"%s","input_tokens":1000,"output_tokens":300}`,
		t1.Format(time.RFC3339Nano))
	step2 := fmt.Sprintf(`{"step_index":2,"source":"MODEL","type":"GENERIC","created_at":"%s"}`,
		t2.Format(time.RFC3339Nano))

	content := strings.Join([]string{step0, step1, step2}, "\n") + "\n"
	if err := os.WriteFile(tPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(tPath)
	if err != nil {
		t.Fatal(err)
	}

	stats := parseTranscriptFile(tPath, info)

	// Since timestamps are reversed, lookahead must find no forward items, totalDurationMs must remain 0
	if stats.totalDurationMs < 0 {
		t.Fatalf("totalDurationMs is negative: %d", stats.totalDurationMs)
	}
	if stats.timedOutputTokens < 0 {
		t.Fatalf("timedOutputTokens is negative: %d", stats.timedOutputTokens)
	}
	if stats.totalDurationMs != 0 {
		t.Errorf("expected 0 duration from reversed timestamps, got %d", stats.totalDurationMs)
	}
}

// TestChallenger_IdenticalTimestamps tests steps with identical timestamps (diff = 0).
func TestChallenger_IdenticalTimestamps(t *testing.T) {
	tmpDir := t.TempDir()
	tPath := filepath.Join(tmpDir, "transcript.jsonl")

	t0 := "2026-10-10T10:00:00.000Z"

	step0 := fmt.Sprintf(`{"step_index":0,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"%s","input_tokens":1000,"output_tokens":500}`, t0)
	step1 := fmt.Sprintf(`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"%s","input_tokens":1000,"output_tokens":500}`, t0)
	step2 := fmt.Sprintf(`{"step_index":2,"source":"MODEL","type":"GENERIC","created_at":"%s"}`, t0)

	content := strings.Join([]string{step0, step1, step2}, "\n") + "\n"
	if err := os.WriteFile(tPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(tPath)
	if err != nil {
		t.Fatal(err)
	}

	stats := parseTranscriptFile(tPath, info)

	// Lookahead requires nextItem.timestamp.After(item.timestamp), which is false for identical timestamps.
	// diff is 0, so duration must be 0, avoiding 0ms division.
	if stats.totalDurationMs != 0 {
		t.Errorf("expected 0ms total duration for identical timestamps, got %d", stats.totalDurationMs)
	}
	if stats.timedOutputTokens != 0 {
		t.Errorf("expected 0 timed output tokens, got %d", stats.timedOutputTokens)
	}
}

// TestChallenger_HTTPAPISummaryBounds tests /api/tokens/summary under pathological transcripts.
func TestChallenger_HTTPAPISummaryBounds(t *testing.T) {
	tmpDir := t.TempDir()
	brainDir := filepath.Join(tmpDir, "brain")
	convDir := filepath.Join(tmpDir, "conversations")
	_ = os.MkdirAll(brainDir, 0755)
	_ = os.MkdirAll(convDir, 0755)

	// Create 4 pathological conversations:
	// 1. Massive burst (50k tokens, 10ms)
	// 2. Zero duration
	// 3. Stalled 1-hour turn
	// 4. Out-of-order timestamps
	scenarios := []struct {
		convID string
		steps  []string
	}{
		{
			convID: "conv-burst",
			steps: []string{
				`{"step_index":0,"source":"SYSTEM","type":"SYSTEM_MESSAGE","content":"<USER_SETTINGS_CHANGE>\nThe user changed setting 'Model Selection' from None to Gemini 3.8 Flash.\n</USER_SETTINGS_CHANGE>"}`,
				`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-10-10T10:00:00Z","duration_ms":10,"input_tokens":2000,"output_tokens":50000}`,
			},
		},
		{
			convID: "conv-zero",
			steps: []string{
				`{"step_index":0,"source":"SYSTEM","type":"SYSTEM_MESSAGE","content":"<USER_SETTINGS_CHANGE>\nThe user changed setting 'Model Selection' from None to Gemini 3.8 Flash.\n</USER_SETTINGS_CHANGE>"}`,
				`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-10-10T10:00:00Z","duration_ms":0,"input_tokens":2000,"output_tokens":1000}`,
			},
		},
		{
			convID: "conv-stalled",
			steps: []string{
				`{"step_index":0,"source":"SYSTEM","type":"SYSTEM_MESSAGE","content":"<USER_SETTINGS_CHANGE>\nThe user changed setting 'Model Selection' from None to Gemini 3.8 Flash.\n</USER_SETTINGS_CHANGE>"}`,
				`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-10-10T10:00:00Z","duration_ms":3600000,"input_tokens":2000,"output_tokens":10}`,
			},
		},
		{
			convID: "conv-skew",
			steps: []string{
				`{"step_index":0,"source":"SYSTEM","type":"SYSTEM_MESSAGE","content":"<USER_SETTINGS_CHANGE>\nThe user changed setting 'Model Selection' from None to Gemini 3.8 Flash.\n</USER_SETTINGS_CHANGE>"}`,
				`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-10-10T10:00:10Z","input_tokens":2000,"output_tokens":1000}`,
				`{"step_index":2,"source":"MODEL","type":"GENERIC","created_at":"2026-10-10T10:00:05Z"}`,
			},
		},
	}

	for _, s := range scenarios {
		logsDir := filepath.Join(brainDir, s.convID, ".system_generated", "logs")
		_ = os.MkdirAll(logsDir, 0755)
		_ = os.WriteFile(filepath.Join(logsDir, "transcript.jsonl"), []byte(strings.Join(s.steps, "\n")+"\n"), 0644)
		_ = os.WriteFile(filepath.Join(convDir, s.convID+".db"), []byte("SQLite format 3\x00"), 0644)
	}

	server := NewServer("127.0.0.1:0", "")
	server.SetAntigravityDataDir(tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/api/tokens/summary", nil)
	w := httptest.NewRecorder()
	server.handleTokensSummary(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		AvgTPS          float64               `json:"avg_tps"`
		TotalTokens     int64                 `json:"total_tokens"`
		ModelBreakdowns []TokenModelBreakdown `json:"model_breakdowns"`
		TelemetryEvents []TokenTelemetryEvent `json:"telemetry_events"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	// Assert global AvgTPS bounds
	if math.IsNaN(resp.AvgTPS) || math.IsInf(resp.AvgTPS, 0) {
		t.Fatalf("global AvgTPS is NaN or Inf: %v", resp.AvgTPS)
	}
	if resp.AvgTPS < 0.0 || resp.AvgTPS > 350.0 {
		t.Fatalf("global AvgTPS %f outside [0.0, 350.0]", resp.AvgTPS)
	}

	// Assert model breakdown AvgTPS bounds
	for _, mb := range resp.ModelBreakdowns {
		if math.IsNaN(mb.AvgTPS) || math.IsInf(mb.AvgTPS, 0) {
			t.Fatalf("model %s AvgTPS is NaN or Inf: %v", mb.ModelID, mb.AvgTPS)
		}
		if mb.AvgTPS < 0.0 || mb.AvgTPS > 350.0 {
			t.Fatalf("model %s AvgTPS %f outside [0.0, 350.0]", mb.ModelID, mb.AvgTPS)
		}
	}

	// Assert telemetry event TPS bounds
	for _, te := range resp.TelemetryEvents {
		if math.IsNaN(te.TPS) || math.IsInf(te.TPS, 0) {
			t.Fatalf("telemetry event %s TPS is NaN or Inf: %v", te.ID, te.TPS)
		}
		if te.TPS < 0.0 || te.TPS > 350.0 {
			t.Fatalf("telemetry event %s TPS %f outside [0.0, 350.0]", te.ID, te.TPS)
		}
	}
}

// TestChallenger_RandomizedFuzzHarness generates 10,000 steps with randomized pathological timings and tokens.
func TestChallenger_RandomizedFuzzHarness(t *testing.T) {
	tmpDir := t.TempDir()
	tPath := filepath.Join(tmpDir, "transcript.jsonl")

	r := rand.New(rand.NewSource(42))
	baseTime := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	curTime := baseTime

	f, err := os.Create(tPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	for stepIdx := 0; stepIdx < 10000; stepIdx++ {
		// Randomized time jump:
		// 10% backwards jump (-5s to -1m)
		// 15% identical timestamp (0ms)
		// 15% micro jump (1ms - 100ms)
		// 30% normal jump (500ms - 30s)
		// 10% massive jump (15m - 2h)
		// 20% steady 1s jump
		dice := r.Float64()
		var delta time.Duration
		switch {
		case dice < 0.10:
			delta = -time.Duration(r.Intn(60000)+1000) * time.Millisecond
		case dice < 0.25:
			delta = 0
		case dice < 0.40:
			delta = time.Duration(r.Intn(100)+1) * time.Millisecond
		case dice < 0.70:
			delta = time.Duration(r.Intn(30000)+500) * time.Millisecond
		case dice < 0.80:
			delta = time.Duration(r.Intn(7200)+900) * time.Second
		default:
			delta = 1 * time.Second
		}
		curTime = curTime.Add(delta)

		// Randomized tokens:
		// 10% 0 tokens
		// 10% massive tokens (50,000 - 200,000)
		// 80% normal tokens (10 - 4,000)
		var outTok int64
		tokDice := r.Float64()
		switch {
		case tokDice < 0.10:
			outTok = 0
		case tokDice < 0.20:
			outTok = int64(r.Intn(150000) + 50000)
		default:
			outTok = int64(r.Intn(4000) + 10)
		}

		// Duration:
		// 30% 0 (triggers lookahead)
		// 10% negative (-100ms)
		// 10% huge (3,600,000ms)
		// 10% micro (1ms)
		// 40% normal (500ms - 10s)
		var durMs int64
		durDice := r.Float64()
		switch {
		case durDice < 0.30:
			durMs = 0
		case durDice < 0.40:
			durMs = -100
		case durDice < 0.50:
			durMs = 3600000
		case durDice < 0.60:
			durMs = int64(r.Intn(5) + 1)
		default:
			durMs = int64(r.Intn(10000) + 500)
		}

		lineMap := map[string]interface{}{
			"step_index":        stepIdx,
			"source":            "MODEL",
			"type":              "PLANNER_RESPONSE",
			"created_at":        curTime.Format(time.RFC3339Nano),
			"input_tokens":      int64(r.Intn(10000) + 100),
			"cache_read_tokens": int64(r.Intn(5000)),
			"output_tokens":     outTok,
			"duration_ms":       durMs,
		}
		data, _ := json.Marshal(lineMap)
		f.Write(append(data, '\n'))
	}
	f.Close()

	info, err := os.Stat(tPath)
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	stats := parseTranscriptFile(tPath, info)
	elapsed := time.Since(start)

	t.Logf("Fuzz harness parsed 10,000 steps in %v", elapsed)

	if stats.totalDurationMs < 0 {
		t.Fatalf("totalDurationMs is negative: %d", stats.totalDurationMs)
	}
	if stats.timedOutputTokens < 0 {
		t.Fatalf("timedOutputTokens is negative: %d", stats.timedOutputTokens)
	}

	if stats.totalDurationMs > 0 && stats.timedOutputTokens > 0 {
		tps := float64(stats.timedOutputTokens) / (float64(stats.totalDurationMs) / 1000.0)
		if math.IsNaN(tps) || math.IsInf(tps, 0) {
			t.Fatalf("TPS is NaN or Inf: %v", tps)
		}
		if tps > 350.0 {
			t.Fatalf("Fuzz TPS exceeded bound: %f > 350.0", tps)
		}
		if tps < 0 {
			t.Fatalf("Fuzz TPS is negative: %f", tps)
		}
		t.Logf("Fuzz overall average TPS: %.2f (within [0, 350] bounds)", tps)
	}
}

// TestChallenger_GetChatMetrics_EdgeCases tests GetChatMetricsForConversation across all edge cases.
func TestChallenger_GetChatMetrics_EdgeCases(t *testing.T) {
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	testCases := []struct {
		name        string
		steps       []string
		expectedErr bool
	}{
		{
			name: "Massive burst 50k tokens in 100ms",
			steps: []string{
				`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","created_at":"2026-10-10T10:00:00Z","content":"prompt"}`,
				`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-10-10T10:00:00.100Z","input_tokens":1000,"output_tokens":50000,"duration_ms":100}`,
			},
		},
		{
			name: "Zero duration",
			steps: []string{
				`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","created_at":"2026-10-10T10:00:00Z","content":"prompt"}`,
				`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-10-10T10:00:00Z","input_tokens":1000,"output_tokens":1000,"duration_ms":0}`,
			},
		},
		{
			name: "Stalled turn 1 hour delay with 0 output tokens",
			steps: []string{
				`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","created_at":"2026-10-10T10:00:00Z","content":"prompt"}`,
				`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-10-10T11:00:00Z","input_tokens":1000,"output_tokens":0,"duration_ms":3600000}`,
			},
		},
		{
			name: "Out of order timestamps",
			steps: []string{
				`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","created_at":"2026-10-10T10:00:10Z","content":"prompt"}`,
				`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-10-10T10:00:05Z","input_tokens":1000,"output_tokens":500}`,
			},
		},
		{
			name: "Identical timestamps",
			steps: []string{
				`{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","created_at":"2026-10-10T10:00:00Z","content":"prompt"}`,
				`{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-10-10T10:00:00Z","input_tokens":1000,"output_tokens":500}`,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			os.Setenv("HOME", tmpDir)

			convID := "test-conv-" + strings.ReplaceAll(tc.name, " ", "-")
			logDir := filepath.Join(tmpDir, ".gemini", "antigravity", "brain", convID, ".system_generated", "logs")
			_ = os.MkdirAll(logDir, 0755)

			_ = os.WriteFile(filepath.Join(logDir, "transcript.jsonl"), []byte(strings.Join(tc.steps, "\n")+"\n"), 0644)

			metrics, err := GetChatMetricsForConversation(convID, "main")
			if err != nil && !tc.expectedErr {
				t.Fatalf("unexpected error: %v", err)
			}
			if metrics != nil {
				if math.IsNaN(metrics.Summary.GenerationSpeed) || math.IsInf(metrics.Summary.GenerationSpeed, 0) {
					t.Fatalf("summary speed is NaN or Inf: %v", metrics.Summary.GenerationSpeed)
				}
				if metrics.Summary.GenerationSpeed > 350.0 {
					t.Fatalf("summary speed %f exceeds 350.0 bound", metrics.Summary.GenerationSpeed)
				}
				if metrics.Summary.GenerationSpeed < 0.0 {
					t.Fatalf("summary speed %f is negative", metrics.Summary.GenerationSpeed)
				}

				for idx, turn := range metrics.Turns {
					if math.IsNaN(turn.SpeedTPS) || math.IsInf(turn.SpeedTPS, 0) {
						t.Fatalf("turn %d speed is NaN or Inf: %v", idx, turn.SpeedTPS)
					}
					if turn.SpeedTPS > 350.0 {
						t.Fatalf("turn %d speed %f exceeds 350.0 bound", idx, turn.SpeedTPS)
					}
					if turn.SpeedTPS < 0.0 {
						t.Fatalf("turn %d speed %f is negative", idx, turn.SpeedTPS)
					}
				}
			}
		})
	}
}
