package webgui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestComputerUseEndpoints(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	s := &Server{}

	// 1. GET /api/system/computer-use
	req := httptest.NewRequest(http.MethodGet, "/api/system/computer-use", nil)
	rec := httptest.NewRecorder()
	s.handleSystemComputerUse(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var res map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res["success"] != true {
		t.Errorf("expected success true in computer-use response")
	}

	// 2. POST /api/system/computer-use/calibrate
	calibBody := `{"target_os":"windows","input_x":100,"input_y":100}`
	reqCalib := httptest.NewRequest(http.MethodPost, "/api/system/computer-use/calibrate", strings.NewReader(calibBody))
	recCalib := httptest.NewRecorder()
	s.handleSystemComputerUseCalibrate(recCalib, reqCalib)
	if recCalib.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recCalib.Code)
	}

	// 3. POST /api/system/computer-use/config
	cfgBody := `{"wayland_pipewire":true,"accessibility_grounding":true,"linux_dpi_normalizer":true,"per_monitor_v2_dpi":true}`
	reqCfg := httptest.NewRequest(http.MethodPost, "/api/system/computer-use/config", strings.NewReader(cfgBody))
	recCfg := httptest.NewRecorder()
	s.handleSystemComputerUseConfig(recCfg, reqCfg)
	if recCfg.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recCfg.Code)
	}
}
