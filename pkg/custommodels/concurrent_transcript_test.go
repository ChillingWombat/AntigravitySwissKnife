package custommodels

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func transcriptFixtureLine(t *testing.T, stepIndex int, content string) []byte {
	t.Helper()
	var line bytes.Buffer
	encoder := json.NewEncoder(&line)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(map[string]any{
		"step_index":    stepIndex,
		"source":        "SYSTEM",
		"type":          "SYSTEM_MESSAGE",
		"content":       content,
		"input_tokens":  1,
		"output_tokens": 1,
	}); err != nil {
		t.Fatalf("marshal transcript line: %v", err)
	}
	return line.Bytes()
}

func openAIStreamFixture() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
}

func customProxyRequest(t *testing.T, proxy *Proxy, convID string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"model":     "deepseek-proxy",
		"requestId": "agent/" + convID + "/1",
		"request": map[string]any{
			"contents": []map[string]any{{
				"role":  "user",
				"parts": []map[string]string{{"text": "hello"}},
			}},
		},
	})
	if err != nil {
		t.Fatalf("marshal proxy request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1internal:streamGenerateContent?alt=sse", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	proxy.Handler().ServeHTTP(rr, req)
	return rr
}

func TestProxyCustomRequestsNeverRewriteHostTranscript(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ANTIGRAVITY_CONFIG_DIR", filepath.Join(root, "antigravity"))
	convID := "e05a681a-1e3c-47f0-b217-aaadc7625295"
	transcriptPath := filepath.Join(root, "antigravity", "brain", convID, ".system_generated", "logs", "transcript.jsonl")
	if err := os.MkdirAll(filepath.Dir(transcriptPath), 0755); err != nil {
		t.Fatalf("create transcript dir: %v", err)
	}
	hostLine := transcriptFixtureLine(t, 0, "<USER_SETTINGS_CHANGE>\nThe user changed setting `Model Selection` from None to Gemini 3.8 Flash (High). No need to comment on this change if the user doesn't ask about it.\n</USER_SETTINGS_CHANGE>")
	if err := os.WriteFile(transcriptPath, hostLine, 0644); err != nil {
		t.Fatalf("write host transcript: %v", err)
	}
	if !bytes.Contains(hostLine, []byte("<USER_SETTINGS_CHANGE>")) {
		t.Fatal("fixture must retain the literal settings tag that triggers the legacy rewrite")
	}

	upstream := openAIStreamFixture()
	defer upstream.Close()
	store, proxy := newProxyFixture(t, upstream.URL)

	rr := customProxyRequest(t, proxy, convID)
	if rr.Code != http.StatusOK {
		t.Fatalf("custom request status = %d, body=%s", rr.Code, rr.Body.String())
	}
	// The removed rewrite ran in a goroutine, so give a regression time to land.
	time.Sleep(250 * time.Millisecond)

	got, err := os.ReadFile(transcriptPath)
	if err != nil {
		t.Fatalf("read host transcript: %v", err)
	}
	if !bytes.Equal(got, hostLine) {
		t.Fatalf("custom request mutated host transcript:\n got: %s\nwant: %s", got, hostLine)
	}
	if got := store.GetConversationModel(convID); got != "deepseek-proxy" {
		t.Fatalf("sidecar model = %q, want deepseek-proxy", got)
	}
}

func TestConversationSidecarConcurrentRecordsPersist(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	const writers = 32
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			store.RecordConversationModel(fmt.Sprintf("conv-%02d", i), fmt.Sprintf("model-%02d", i))
		}(i)
	}
	close(start)
	wg.Wait()

	reloaded, err := NewStore(dir)
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}
	for i := 0; i < writers; i++ {
		convID := fmt.Sprintf("conv-%02d", i)
		want := fmt.Sprintf("model-%02d", i)
		if got := reloaded.GetConversationModel(convID); got != want {
			t.Errorf("%s model = %q, want %q", convID, got, want)
		}
	}
	info, err := os.Stat(filepath.Join(dir, "conversation_models.json"))
	if err != nil {
		t.Fatalf("stat conversation sidecar: %v", err)
	}
	if runtime.GOOS != "windows" {
		if got := info.Mode().Perm(); got != 0600 {
			t.Errorf("conversation sidecar mode = %04o, want 0600", got)
		}
	}
}

func TestProxyConcurrentRequestsPreserveHostTranscript(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ANTIGRAVITY_CONFIG_DIR", filepath.Join(root, "antigravity"))
	convID := "11223344-5566-7788-99aa-bbccddeeff00"
	transcriptPath := filepath.Join(root, "antigravity", "brain", convID, ".system_generated", "logs", "transcript.jsonl")
	if err := os.MkdirAll(filepath.Dir(transcriptPath), 0755); err != nil {
		t.Fatalf("create transcript dir: %v", err)
	}

	initialLine := transcriptFixtureLine(t, 0, "<USER_SETTINGS_CHANGE>\nThe user changed setting `Model Selection` from None to Gemini 3.8 Flash (High). No need to comment on this change if the user doesn't ask about it.\n</USER_SETTINGS_CHANGE>")
	if err := os.WriteFile(transcriptPath, initialLine, 0644); err != nil {
		t.Fatalf("write initial host transcript: %v", err)
	}
	const appendedLines = 500
	hostLines := make([][]byte, 0, appendedLines)
	for i := 0; i < appendedLines; i++ {
		hostLines = append(hostLines, transcriptFixtureLine(t, i+1, fmt.Sprintf("host append %d", i)))
	}
	expected := bytes.NewBuffer(nil)
	expected.Write(initialLine)

	hostFile, err := os.OpenFile(transcriptPath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatalf("open host transcript: %v", err)
	}
	hostDone := make(chan error, 1)
	start := make(chan struct{})
	go func() {
		<-start
		for _, line := range hostLines {
			n, writeErr := hostFile.Write(line)
			if writeErr == nil && n != len(line) {
				writeErr = fmt.Errorf("short append: wrote %d of %d bytes", n, len(line))
			}
			if writeErr == nil {
				writeErr = hostFile.Sync()
			}
			if writeErr != nil {
				_ = hostFile.Close()
				hostDone <- writeErr
				return
			}
			expected.Write(line)
		}
		hostDone <- hostFile.Close()
	}()

	upstream := openAIStreamFixture()
	defer upstream.Close()
	store, proxy := newProxyFixture(t, upstream.URL)

	const requests = 24
	requestErrs := make(chan error, requests)
	var requestWG sync.WaitGroup
	for i := 0; i < requests; i++ {
		requestWG.Add(1)
		go func() {
			defer requestWG.Done()
			<-start
			body, err := json.Marshal(map[string]any{
				"model":     "deepseek-proxy",
				"requestId": "agent/" + convID + "/1",
				"request": map[string]any{
					"contents": []map[string]any{{
						"role":  "user",
						"parts": []map[string]string{{"text": "concurrent hello"}},
					}},
				},
			})
			if err != nil {
				requestErrs <- err
				return
			}
			req := httptest.NewRequest(http.MethodPost, "/v1internal:streamGenerateContent?alt=sse", bytes.NewReader(body))
			rr := httptest.NewRecorder()
			proxy.Handler().ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				requestErrs <- fmt.Errorf("status=%d body=%s", rr.Code, rr.Body.String())
				return
			}
			if !strings.Contains(rr.Body.String(), `"candidates"`) {
				requestErrs <- fmt.Errorf("response missing candidates: %s", rr.Body.String())
				return
			}
			requestErrs <- nil
		}()
	}
	close(start)

	var requestErr error
	for i := 0; i < requests; i++ {
		if err := <-requestErrs; err != nil && requestErr == nil {
			requestErr = err
		}
	}
	requestWG.Wait()
	if requestErr != nil {
		t.Fatalf("concurrent custom request: %v", requestErr)
	}
	select {
	case err := <-hostDone:
		if err != nil {
			t.Fatalf("host transcript writer: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("host transcript writer did not finish")
	}

	got, err := os.ReadFile(transcriptPath)
	if err != nil {
		t.Fatalf("read final transcript: %v", err)
	}
	if !bytes.Equal(got, expected.Bytes()) {
		t.Fatalf("host transcript changed during proxy requests: got %d bytes, want %d bytes", len(got), expected.Len())
	}
	if !bytes.HasSuffix(got, []byte("\n")) {
		t.Fatal("final transcript is not newline terminated")
	}
	lines := bytes.Split(bytes.TrimSuffix(got, []byte("\n")), []byte("\n"))
	if len(lines) != appendedLines+1 {
		t.Fatalf("final transcript has %d lines, want %d", len(lines), appendedLines+1)
	}
	inputTokens, outputTokens := 0, 0
	for i, line := range lines {
		var row struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		}
		if err := json.Unmarshal(line, &row); err != nil {
			t.Fatalf("line %d is not JSON: %v", i, err)
		}
		inputTokens += row.InputTokens
		outputTokens += row.OutputTokens
	}
	if inputTokens == 0 || outputTokens == 0 {
		t.Fatalf("token counts are not readable: input=%d output=%d", inputTokens, outputTokens)
	}
	if got := store.GetConversationModel(convID); got != "deepseek-proxy" {
		t.Fatalf("sidecar model = %q, want deepseek-proxy", got)
	}
	if got := GetRoutedConversationModel(convID); got != "deepseek-proxy" {
		t.Fatalf("routed model = %q, want deepseek-proxy", got)
	}
}
