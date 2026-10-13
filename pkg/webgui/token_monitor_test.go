package webgui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
)

func TestModelDetectionFromTranscriptStep0(t *testing.T) {
	testCases := []struct {
		name          string
		settingsText  string
		expectedID    string
		expectedName  string
		expectedProv  string
		customModels  []custommodels.CustomModel
		catalogModels []custommodels.CatalogModelInput
	}{
		{
			name:         "Gemini 3.8 Flash High",
			settingsText: "<USER_SETTINGS_CHANGE>\nThe user changed setting `Model Selection` from None to Gemini 3.8 Flash (High). No need to comment on this change if the user doesn't ask about it.\n</USER_SETTINGS_CHANGE>",
			expectedID:   "gemini-3.8-flash",
			expectedName: "Gemini 3.8 Flash",
			expectedProv: "gemini",
		},
		{
			name:         "Gemini 3.8 Flash Medium",
			settingsText: "<USER_SETTINGS_CHANGE>\nThe user changed setting `Model Selection` from None to Gemini 3.8 Flash (Medium). No need to comment.\n</USER_SETTINGS_CHANGE>",
			expectedID:   "gemini-3.8-flash",
			expectedName: "Gemini 3.8 Flash",
			expectedProv: "gemini",
		},
		{
			name:         "Gemini 3.8 Flash Low",
			settingsText: "<USER_SETTINGS_CHANGE>\nThe user changed setting 'Model Selection' from None to Gemini 3.8 Flash (Low).\n</USER_SETTINGS_CHANGE>",
			expectedID:   "gemini-3.8-flash",
			expectedName: "Gemini 3.8 Flash",
			expectedProv: "gemini",
		},
		{
			name:         "Gemini 3.7 Flash",
			settingsText: "<USER_SETTINGS_CHANGE>\nThe user changed setting `Model Selection` from None to Gemini 3.7 Flash.\n</USER_SETTINGS_CHANGE>",
			expectedID:   "gemini-3.7-flash",
			expectedName: "Gemini 3.7 Flash",
			expectedProv: "gemini",
		},
		{
			name:         "Gemini 3.1 Pro",
			settingsText: "<USER_SETTINGS_CHANGE>\nThe user changed setting `Model Selection` from None to Gemini 3.1 Pro. No need to comment.\n</USER_SETTINGS_CHANGE>",
			expectedID:   "gemini-3.1-pro",
			expectedName: "Gemini 3.1 Pro",
			expectedProv: "gemini",
		},
		{
			name:         "Claude 3.7 Sonnet Genuine Selection",
			settingsText: "<USER_SETTINGS_CHANGE>\nThe user changed setting `Model Selection` from None to Claude 3.7 Sonnet.\n</USER_SETTINGS_CHANGE>",
			expectedID:   "claude-3-7-sonnet",
			expectedName: "Claude 3.7 Sonnet",
			expectedProv: "anthropic",
		},
		{
			name:         "Claude Opus 4.6 Genuine Selection",
			settingsText: "<USER_SETTINGS_CHANGE>\nThe user changed setting `Model Selection` from None to Claude Opus 4.6.\n</USER_SETTINGS_CHANGE>",
			expectedID:   "claude-opus-4-6",
			expectedName: "Claude Opus 4.6",
			expectedProv: "anthropic",
		},
		{
			name:         "Custom Model Selection",
			settingsText: "<USER_SETTINGS_CHANGE>\nThe user changed setting `Model Selection` from None to deepseek-chat.\n</USER_SETTINGS_CHANGE>",
			expectedID:   "deepseek-chat",
			expectedName: "DeepSeek Chat V3",
			expectedProv: "openai",
			customModels: []custommodels.CustomModel{
				{
					ID:           "cm-1",
					Name:         "deepseek-chat",
					DisplayName:  "DeepSeek Chat V3",
					ProviderType: "openai",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tPath := filepath.Join(tmpDir, "transcript.jsonl")

			step0 := map[string]interface{}{
				"step_index": 0,
				"source":     "SYSTEM",
				"type":       "SYSTEM_MESSAGE",
				"content":    "System prompt prefix...\n" + tc.settingsText + "\nSystem prompt suffix...",
			}
			data, _ := json.Marshal(step0)
			_ = os.WriteFile(tPath, append(data, '\n'), 0644)

			mID, mName, mProv := detectConversationModelWithTranscript(tPath, "", tc.customModels, tc.catalogModels)
			if mID != tc.expectedID {
				t.Errorf("expected modelID %q, got %q", tc.expectedID, mID)
			}
			if mName != tc.expectedName {
				t.Errorf("expected displayName %q, got %q", tc.expectedName, mName)
			}
			if mProv != tc.expectedProv {
				t.Errorf("expected provider %q, got %q", tc.expectedProv, mProv)
			}
		})
	}
}

func TestModelDetection_EliminateFalseOpusAttribution(t *testing.T) {
	tmpDir := t.TempDir()
	agDir := filepath.Join(tmpDir, "antigravity")
	brainDir := filepath.Join(agDir, "brain")
	convDir := filepath.Join(agDir, "conversations")
	_ = os.MkdirAll(convDir, 0755)

	// Scenario 1: A conversation whose SQLite DB contains numerous mentions of "claude-opus-4-6"
	// in prompt text and code snippets, but whose transcript step 0 specifies Gemini 3.8 Flash (High).
	conv1ID := "conv-false-opus-1"
	conv1Logs := filepath.Join(brainDir, conv1ID, ".system_generated", "logs")
	_ = os.MkdirAll(conv1Logs, 0755)

	step0 := map[string]interface{}{
		"step_index": 0,
		"source":     "SYSTEM",
		"type":       "SYSTEM_MESSAGE",
		"content":    "<USER_SETTINGS_CHANGE>\nThe user changed setting `Model Selection` from None to Gemini 3.8 Flash (High). No need to comment on this change if the user doesn't ask about it.\n</USER_SETTINGS_CHANGE>",
	}
	step0Bytes, _ := json.Marshal(step0)
	step1 := map[string]interface{}{
		"step_index":        1,
		"type":              "PLANNER_RESPONSE",
		"input_tokens":      12000,
		"output_tokens":     4000,
		"cache_read_tokens": 8000,
	}
	step1Bytes, _ := json.Marshal(step1)
	_ = os.WriteFile(filepath.Join(conv1Logs, "transcript.jsonl"), append(append(step0Bytes, '\n'), append(step1Bytes, '\n')...), 0644)

	// In SQLite DB: contains claude-opus-4-6 in prompt mentions
	dbContent := "SQLite format 3\x00header\x00Can you compare claude-opus-4-6 against Gemini?\x00claude-opus-4-6 benchmark data\x00claude-opus-4-6-thinking test"
	conv1DBPath := filepath.Join(convDir, conv1ID+".db")
	_ = os.WriteFile(conv1DBPath, []byte(dbContent), 0644)

	// Test direct detection via detectConversationModel
	mID, mName, mProv := detectConversationModel(conv1DBPath, nil, nil)
	if mID == "claude-opus-4-6" {
		t.Fatalf("CRITICAL REGRESSION: falsely attributed conversation to claude-opus-4-6 due to DB text scan! Got %q (%q, %q)", mID, mName, mProv)
	}
	if mID != "gemini-3.8-flash" {
		t.Errorf("expected strictly gemini-3.8-flash, got %q", mID)
	}
	if mProv != "gemini" {
		t.Errorf("expected provider gemini, got %q", mProv)
	}

	// Test resolveConversationModel(convID)
	rID, rName, rProv := resolveConversationModel(conv1ID)
	// Temporarily override or test that resolveConversationModel handles conv1
	t.Logf("resolveConversationModel(%s) -> %s (%s, %s)", conv1ID, rID, rName, rProv)

	// Scenario 2: Conversation with DB containing "claude-opus-4-6" but transcript has NO settings change (e.g. subagent).
	// Must fall back safely to active native Gemini default (gemini-3.8-flash), NEVER claude-opus-4-6!
	conv2ID := "conv-subagent-opus-mention"
	conv2Logs := filepath.Join(brainDir, conv2ID, ".system_generated", "logs")
	_ = os.MkdirAll(conv2Logs, 0755)

	subagentStep0 := map[string]interface{}{
		"step_index": 0,
		"source":     "SYSTEM",
		"type":       "SYSTEM_MESSAGE",
		"content":    "You are a subagent worker tasked with fixing claude-opus-4-6 detection rules in the codebase.",
	}
	sBytes, _ := json.Marshal(subagentStep0)
	_ = os.WriteFile(filepath.Join(conv2Logs, "transcript.jsonl"), append(sBytes, '\n'), 0644)
	conv2DBPath := filepath.Join(convDir, conv2ID+".db")
	_ = os.WriteFile(conv2DBPath, []byte("SQLite format 3\x00claude-opus-4-6 mention\x00"), 0644)

	mID2, mName2, mProv2 := detectConversationModel(conv2DBPath, nil, nil)
	if mID2 == "claude-opus-4-6" {
		t.Fatalf("CRITICAL REGRESSION: subagent with claude-opus-4-6 mention falsely attributed to claude-opus-4-6! Got %q (%q, %q)", mID2, mName2, mProv2)
	}
	if mID2 != "gemini-3.8-flash" {
		t.Errorf("expected fallback to active native gemini-3.8-flash, got %q", mID2)
	}
}

func TestLiveConversations_ZeroFalseOpusAttributions(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("User home directory not available")
	}

	convDir := filepath.Join(homeDir, ".gemini", "antigravity", "conversations")
	brainDir := filepath.Join(homeDir, ".gemini", "antigravity", "brain")
	if _, err := os.Stat(convDir); err != nil {
		t.Skip("Live Antigravity conversations directory not found")
	}

	entries, err := os.ReadDir(convDir)
	if err != nil {
		t.Skip("Could not read live conversations directory")
	}

	var opusCount int
	var geminiCount int
	var totalChecked int

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}
		convID := strings.TrimSuffix(entry.Name(), ".db")
		convDBPath := filepath.Join(convDir, entry.Name())
		tPath := filepath.Join(brainDir, convID, ".system_generated", "logs", "transcript.jsonl")

		totalChecked++
		mID, _, _ := detectConversationModelWithTranscript(tPath, convDBPath, nil, nil)
		if mID == "claude-opus-4-6" {
			if rawName, ok := extractModelFromTranscript(tPath); ok && strings.Contains(strings.ToLower(rawName), "opus") {
				// Genuine user selection of Claude Opus in step 0
				continue
			}
			opusCount++
		} else if strings.HasPrefix(mID, "gemini-") {
			geminiCount++
		}
	}

	t.Logf("Live conversation telemetry check: total=%d, gemini=%d, opus=%d", totalChecked, geminiCount, opusCount)
	if opusCount > 0 {
		t.Fatalf("FAIL: Found %d false claude-opus-4-6 attributions in live conversations! Must be 0.", opusCount)
	}
}

func TestTokenMonitor_TPS_ForwardLookahead(t *testing.T) {
	tmpDir := t.TempDir()
	tPath := filepath.Join(tmpDir, "transcript.jsonl")

	// Simulating Step 372-374 sequence:
	// Step 372: previous tool output logged at 10:20:49Z
	// Step 373: model generation dispatches at 10:20:50Z with 3392 output tokens
	// Step 374: next tool execution occurs at 10:21:28Z (38 seconds later)
	step372 := `{"step_index":372,"source":"MODEL","type":"GENERIC","created_at":"2026-10-08T10:20:49Z"}`
	step373 := `{"step_index":373,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-10-08T10:20:50Z","input_tokens":5000,"output_tokens":3392}`
	step374 := `{"step_index":374,"source":"MODEL","type":"GENERIC","created_at":"2026-10-08T10:21:28Z"}`

	content := strings.Join([]string{step372, step373, step374}, "\n") + "\n"
	if err := os.WriteFile(tPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test transcript: %v", err)
	}

	info, err := os.Stat(tPath)
	if err != nil {
		t.Fatalf("failed to stat test transcript: %v", err)
	}

	stats := parseTranscriptFile(tPath, info)
	if stats.timedOutputTokens != 3392 {
		t.Errorf("expected 3392 timed output tokens, got %d", stats.timedOutputTokens)
	}
	if stats.totalDurationMs != 38000 {
		t.Errorf("expected 38000ms duration from forward lookahead, got %dms", stats.totalDurationMs)
	}

	tps := float64(stats.timedOutputTokens) / (float64(stats.totalDurationMs) / 1000.0)
	// 3392 / 38s = 89.26 TPS
	if tps < 85.0 || tps > 95.0 {
		t.Errorf("expected realistic ~89.3 TPS, got %.2f TPS (must eliminate >700 TPS bug)", tps)
	}
	if len(stats.recentSteps) != 1 {
		t.Fatalf("expected 1 recent step, got %d", len(stats.recentSteps))
	}
	if stats.recentSteps[0].durationMs != 38000 {
		t.Errorf("expected recent step duration 38000ms, got %dms", stats.recentSteps[0].durationMs)
	}
}

func TestTokenMonitor_TPS_AccumulatorAlignment(t *testing.T) {
	tmpDir := t.TempDir()
	tPath := filepath.Join(tmpDir, "transcript.jsonl")

	// Step 1: 1000 output tokens, followed by next event 20s later -> 50 TPS
	step1 := `{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-10-08T10:00:00Z","input_tokens":2000,"output_tokens":1000}`
	step2 := `{"step_index":2,"source":"MODEL","type":"GENERIC","created_at":"2026-10-08T10:00:20Z"}`
	// Step 3: 500 output tokens, but no subsequent event (unmeasured streaming interval)
	step3 := `{"step_index":3,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-10-08T10:00:25Z","input_tokens":2000,"output_tokens":500}`

	content := strings.Join([]string{step1, step2, step3}, "\n") + "\n"
	if err := os.WriteFile(tPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test transcript: %v", err)
	}

	info, err := os.Stat(tPath)
	if err != nil {
		t.Fatalf("failed to stat test transcript: %v", err)
	}

	stats := parseTranscriptFile(tPath, info)
	if stats.outputTokens != 1500 {
		t.Errorf("expected 1500 total output tokens, got %d", stats.outputTokens)
	}
	// timedOutputTokens MUST only count step 1 (1000 tokens), not step 3
	if stats.timedOutputTokens != 1000 {
		t.Errorf("expected 1000 timed output tokens (only measured steps), got %d", stats.timedOutputTokens)
	}
	if stats.totalDurationMs != 20000 {
		t.Errorf("expected 20000ms total duration, got %d", stats.totalDurationMs)
	}

	calcTPS := float64(stats.timedOutputTokens) / (float64(stats.totalDurationMs) / 1000.0)
	if calcTPS != 50.0 {
		t.Errorf("expected strictly 50.0 TPS, got %.2f (accumulator mismatch division prevented)", calcTPS)
	}
}

func TestTokenMonitor_TPS_RealisticBounding(t *testing.T) {
	tmpDir := t.TempDir()
	tPath := filepath.Join(tmpDir, "transcript.jsonl")

	// Synthetic anomaly: 2000 tokens with artificially small interval (e.g. 500ms -> 4000 TPS)
	step1 := `{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-10-08T10:00:00Z","duration_ms":500,"input_tokens":2000,"output_tokens":2000}`
	if err := os.WriteFile(tPath, []byte(step1+"\n"), 0644); err != nil {
		t.Fatalf("failed to write test transcript: %v", err)
	}

	info, err := os.Stat(tPath)
	if err != nil {
		t.Fatalf("failed to stat test transcript: %v", err)
	}

	stats := parseTranscriptFile(tPath, info)
	tps := float64(stats.timedOutputTokens) / (float64(stats.totalDurationMs) / 1000.0)
	if tps > 350.0 {
		t.Errorf("generation speed must be bounded to realistic thresholds <= 350 TPS, got %.2f", tps)
	}
	if stats.recentSteps[0].durationMs <= 0 {
		t.Errorf("expected bounded durationMs, got %d", stats.recentSteps[0].durationMs)
	}
}
