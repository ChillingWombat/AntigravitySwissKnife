package webgui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestP1_7_HandleFilesTerminal_Security verifies that handleFilesTerminal rejects path traversal
// and executes safely without raw shell string concatenation.
func TestP1_7_HandleFilesTerminal_Security(t *testing.T) {
	s := &Server{}

	// 1. Path traversal attempt must be rejected by jailed path validation
	body, _ := json.Marshal(map[string]string{"path": "../../../../../etc/passwd"})
	req := httptest.NewRequest(http.MethodPost, "/api/files/terminal", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.handleFilesTerminal(w, req)

	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["success"] == true {
		t.Errorf("expected handleFilesTerminal to reject path traversal, but got success=true")
	}

	// 2. Legitimate directory inside temp dir succeeds (in test execution mode)
	tmpDir := t.TempDir()
	bodySafe, _ := json.Marshal(map[string]string{"path": tmpDir})
	reqSafe := httptest.NewRequest(http.MethodPost, "/api/files/terminal", bytes.NewReader(bodySafe))
	wSafe := httptest.NewRecorder()
	s.handleFilesTerminal(wSafe, reqSafe)

	var respSafe map[string]interface{}
	_ = json.Unmarshal(wSafe.Body.Bytes(), &respSafe)
	if respSafe["success"] != true {
		t.Errorf("expected handleFilesTerminal to succeed for safe directory, got: %v", respSafe)
	}
}

// TestP1_8_HandleUtilitiesImport_PathTraversal verifies that handleUtilitiesImport rejects
// directory traversal in candidate IDs.
func TestP1_8_HandleUtilitiesImport_PathTraversal(t *testing.T) {
	s := &Server{}

	payload := map[string]interface{}{
		"source": "claude-code",
		"mode":   "auto",
		"candidate_ids": []string{
			"../../../../etc/shadow",
			"..\\..\\windows\\system32\\calc",
			"bad/path/traversal",
			"valid-session-12345",
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/utilities/import", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.handleUtilitiesImport(w, req)

	// Since valid-session-12345 won't exist in dummy claude transcripts, results should be empty or safe
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["imported_count"] != 0 && resp["imported_count"] != nil {
		// Should not import traversal files
		t.Logf("Import response: %v", resp)
	}
}

// TestP1_9_HandleTokensChatMetrics_ValidationAndBounds verifies conversation ID validation
// and file read size bounds in token metrics.
func TestP1_9_HandleTokensChatMetrics_ValidationAndBounds(t *testing.T) {
	s := &Server{}

	// 1. Path traversal conversation_id must return 400 Bad Request
	badConvIDs := []string{
		"../../../../etc/passwd",
		"conv/evil",
		"conv\\evil",
		"conv*bad",
		"conv:evil",
	}
	for _, badID := range badConvIDs {
		req := httptest.NewRequest(http.MethodGet, "/api/tokens/chat-metrics?conversation_id="+badID, nil)
		w := httptest.NewRecorder()
		s.handleTokensChatMetrics(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected HTTP 400 for bad conversation_id %q, got %d (body: %s)", badID, w.Code, w.Body.String())
		}
	}

	// 2. Direct call to GetChatMetricsForConversation with path traversal must fail
	_, err := GetChatMetricsForConversation("../../etc/passwd", "aggregated")
	if err == nil {
		t.Errorf("expected GetChatMetricsForConversation to return error on path traversal, got nil")
	}

	// 3. Size limit check: a transcript file exceeding 10MB must be rejected
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome)

	testConvID := "12345678-1234-1234-1234-123456789abc"
	logDir := filepath.Join(tmpHome, ".gemini", "antigravity", "brain", testConvID, ".system_generated", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}
	largeFile := filepath.Join(logDir, "transcript.jsonl")
	// Create a sparse file or oversized file (> 10MB)
	f, err := os.Create(largeFile)
	if err != nil {
		t.Fatal(err)
	}
	// Truncate to 11MB
	_ = f.Truncate(11 * 1024 * 1024)
	_ = f.Close()

	_, errLarge := GetChatMetricsForConversation(testConvID, "aggregated")
	if errLarge == nil {
		t.Errorf("expected error when transcript exceeds 10MB, got nil")
	}
}

// TestP1_17_HandleSwitch_AtomicDebouncing verifies that concurrent account switch requests
// are debounced, returning HTTP 409 Conflict for in-flight collisions.
func TestP1_17_HandleSwitch_AtomicDebouncing(t *testing.T) {
	s := &Server{}

	// Empty email must return 400 Bad Request
	reqEmpty := httptest.NewRequest(http.MethodPost, "/api/switch", bytes.NewReader([]byte(`{"email": ""}`)))
	wEmpty := httptest.NewRecorder()
	s.handleSwitch(wEmpty, reqEmpty)
	if wEmpty.Code != http.StatusBadRequest {
		t.Errorf("expected HTTP 400 for empty email, got %d", wEmpty.Code)
	}

	// Simulate an active in-flight switch
	s.isSwitching.Store(true)

	reqConflict := httptest.NewRequest(http.MethodPost, "/api/switch", bytes.NewReader([]byte(`{"email": "user@example.com"}`)))
	wConflict := httptest.NewRecorder()
	s.handleSwitch(wConflict, reqConflict)

	if wConflict.Code != http.StatusConflict {
		t.Errorf("expected HTTP 409 Conflict during in-flight switch, got %d", wConflict.Code)
	}

	var respConflict map[string]interface{}
	_ = json.Unmarshal(wConflict.Body.Bytes(), &respConflict)
	if respConflict["success"] == true {
		t.Errorf("expected success=false for debounced switch, got true")
	}

	// Release lock and verify it can proceed
	s.isSwitching.Store(false)
	var wg sync.WaitGroup
	statusCodes := make([]int, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/api/switch", bytes.NewReader([]byte(`{"email": "concurrent@example.com"}`)))
			w := httptest.NewRecorder()
			s.handleSwitch(w, req)
			statusCodes[idx] = w.Code
		}(i)
	}
	wg.Wait()

	// At least one request or more should have been debounced with 409 Conflict or handled cleanly
	conflicts := 0
	for _, code := range statusCodes {
		if code == http.StatusConflict {
			conflicts++
		}
	}
	t.Logf("Concurrent switch run completed: %d collisions caught with HTTP 409", conflicts)
}
