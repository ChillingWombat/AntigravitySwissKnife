package custommodels

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestProxyExecutesSelectedGCPModelThroughVertex(t *testing.T) {
	installVertexTokenTransport(t)

	var requests atomic.Int32
	var gotPath, gotAuth string
	var gotBody geminiRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		gotPath = r.URL.RequestURI()
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode Vertex request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"text\":\"vertex-ok\"}]},\"finishReason\":\"STOP\"}]}\n\n")
	}))
	defer upstream.Close()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	model := CustomModel{
		ID:                   "gcp-app-test",
		Name:                 "gemini-2.5-pro",
		DisplayName:          "Vertex Test",
		ProviderType:         ProviderGCP,
		BaseURL:              upstream.URL,
		GCPRegion:            "us-central1",
		GCPServiceAccountKey: testVertexServiceAccountKey(t),
		Enabled:              true,
	}
	if err := store.SaveModel(model); err != nil {
		t.Fatalf("SaveModel: %v", err)
	}
	proxy := NewProxy(store)
	proxy.gw.AllowLoopback = true
	proxy.gw.client = upstream.Client()

	rr := runAppStreamEnvelope(t, proxy, "models/gcp-app-test", "TOP-SECRET-PROMPT")
	if rr.Code != http.StatusOK {
		t.Fatalf("proxy status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("Vertex requests = %d, want 1; body = %s", got, rr.Body.String())
	}
	if gotAuth != "Bearer vertex-test-token" {
		t.Fatalf("Vertex Authorization = %q", gotAuth)
	}
	wantPath := "/v1/projects/credential-project/locations/us-central1/publishers/google/models/gemini-2.5-pro:streamGenerateContent?alt=sse"
	if gotPath != wantPath {
		t.Fatalf("Vertex path = %q, want %q", gotPath, wantPath)
	}
	if len(gotBody.Contents) != 1 || len(gotBody.Contents[0].Parts) != 1 || gotBody.Contents[0].Parts[0].Text != "TOP-SECRET-PROMPT" {
		t.Fatalf("Vertex request did not carry the app prompt: %+v", gotBody.Contents)
	}
	frames := decodeAppSSEFrames(t, rr.Body.Bytes())
	if len(frames) == 0 {
		t.Fatal("expected at least one Cloud Code frame")
	}
	var text strings.Builder
	for _, frame := range frames {
		for _, candidate := range frame.Candidates {
			for _, part := range candidate.Content.Parts {
				text.WriteString(part.Text)
			}
		}
	}
	if text.String() != "vertex-ok" {
		t.Fatalf("streamed text = %q, want vertex-ok; body = %s", text.String(), rr.Body.String())
	}
}

func TestProxyProviderFailureEndsSSEWithSingleTerminalFrame(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	model := CustomModel{
		ID:           "terminal-model",
		Name:         "terminal-upstream",
		ProviderType: ProviderOpenAI,
		BaseURL:      "http://127.0.0.1:9",
		APIKey:       "sk-test",
		Enabled:      true,
	}
	if err := store.SaveModel(model); err != nil {
		t.Fatalf("SaveModel: %v", err)
	}
	proxy := NewProxy(store)
	proxy.gw.AllowLoopback = true
	proxy.gw.client = &http.Client{Transport: gatewayRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("provider unavailable")
	})}

	oldWriter := log.Writer()
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(oldWriter) })

	rr := runAppStreamEnvelope(t, proxy, "terminal-model", "TOP-SECRET-PROMPT")
	if rr.Code != http.StatusOK {
		t.Fatalf("proxy status = %d, body = %s", rr.Code, rr.Body.String())
	}
	assertTerminalProviderFailure(t, rr, "provider unavailable")
	logText := logs.String()
	if got := strings.Count(logText, "provider failure"); got != 1 {
		t.Fatalf("provider failure logs = %d, want 1; logs = %q", got, logText)
	}
	if !strings.Contains(logText, "stage=stream") || !strings.Contains(logText, `model="terminal-model"`) || !strings.Contains(logText, "provider=openai") {
		t.Fatalf("failure log lacks redacted stage/model/provider context: %q", logText)
	}
	if strings.Contains(logText, "TOP-SECRET-PROMPT") || strings.Contains(logText, "sk-test") {
		t.Fatalf("failure log leaked request credentials or prompt: %q", logText)
	}
}

func TestProxyRejectsInvalidGCPCredentialsBeforeRequest(t *testing.T) {
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer upstream.Close()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	model := CustomModel{
		ID:                   "gcp-invalid-test",
		Name:                 "gemini-2.5-pro",
		ProviderType:         ProviderGCP,
		BaseURL:              upstream.URL,
		GCPServiceAccountKey: `{}`,
		Enabled:              true,
	}
	if err := store.SaveModel(model); err != nil {
		t.Fatalf("SaveModel: %v", err)
	}
	proxy := NewProxy(store)
	proxy.gw.AllowLoopback = true
	proxy.gw.client = upstream.Client()

	rr := runAppStreamEnvelope(t, proxy, "gcp-invalid-test", "hello")
	assertTerminalProviderFailure(t, rr, "service_account")
	if got := requests.Load(); got != 0 {
		t.Fatalf("Vertex requests = %d, want 0 before credential validation", got)
	}
}

func TestProxyGCPStreamFailuresEndWithTerminalFrame(t *testing.T) {
	installVertexTokenTransport(t)
	key := testVertexServiceAccountKey(t)
	tests := []struct {
		name    string
		body    string
		stalled bool
		want    string
	}{
		{name: "malformed", body: "data: {\"candidates\":[\n\n", want: "decode vertex stream"},
		{name: "stalled", stalled: true, want: "request canceled"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.Header().Set("Content-Type", "text/event-stream")
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
				if tc.stalled {
					<-r.Context().Done()
					return
				}
				_, _ = io.WriteString(w, tc.body)
			}))
			defer upstream.Close()

			store, err := NewStore(t.TempDir())
			if err != nil {
				t.Fatalf("NewStore: %v", err)
			}
			model := CustomModel{
				ID:                   "gcp-stream-" + tc.name,
				Name:                 "gemini-2.5-pro",
				ProviderType:         ProviderGCP,
				BaseURL:              upstream.URL,
				GCPRegion:            "us-central1",
				GCPServiceAccountKey: key,
				Enabled:              true,
			}
			if err := store.SaveModel(model); err != nil {
				t.Fatalf("SaveModel: %v", err)
			}
			proxy := NewProxy(store)
			proxy.gw.AllowLoopback = true
			proxy.gw.client = &http.Client{Timeout: 250 * time.Millisecond}

			started := time.Now()
			rr := runAppStreamEnvelope(t, proxy, model.ID, "TOP-SECRET-PROMPT")
			if elapsed := time.Since(started); elapsed > 2*time.Second {
				t.Fatalf("failure took %s, want a bounded response", elapsed)
			}
			assertTerminalProviderFailure(t, rr, tc.want)
			if got := requests.Load(); got != 1 {
				t.Fatalf("Vertex requests = %d, want 1", got)
			}
		})
	}
}

func assertTerminalProviderFailure(t *testing.T, rr *httptest.ResponseRecorder, want string) {
	t.Helper()
	if rr.Code != http.StatusOK {
		t.Fatalf("proxy status = %d, body = %s", rr.Code, rr.Body.String())
	}
	frames := decodeAppSSEFrames(t, rr.Body.Bytes())
	if len(frames) != 1 {
		t.Fatalf("terminal frames = %d, want exactly one; body = %s", len(frames), rr.Body.String())
	}
	frame := frames[0]
	if frame.Error == nil {
		t.Fatalf("terminal frame = %+v, want error", frame)
	}
	if len(frame.Candidates) != 1 {
		t.Fatalf("terminal frame candidates = %d, want 1", len(frame.Candidates))
	}
	candidate := frame.Candidates[0]
	if candidate.FinishReason != "STOP" {
		t.Fatalf("finishReason = %q, want STOP terminal marker", candidate.FinishReason)
	}
	var text strings.Builder
	for _, part := range candidate.Content.Parts {
		text.WriteString(part.Text)
	}
	visible := strings.ToLower(frame.Error.Message + "\n" + text.String())
	if !strings.Contains(visible, strings.ToLower(want)) {
		t.Fatalf("terminal failure = %q, want %q; body = %s", visible, want, rr.Body.String())
	}
	if text.Len() == 0 {
		t.Fatal("terminal failure has no visible text for the host")
	}
}

func installVertexTokenTransport(t *testing.T) {
	t.Helper()
	originalTransport := http.DefaultTransport
	http.DefaultTransport = gatewayRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "oauth2.googleapis.com" {
			return originalTransport.RoundTrip(r)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"vertex-test-token","token_type":"Bearer","expires_in":3600}`)),
			Request:    r,
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
}

func testVertexServiceAccountKey(t *testing.T) string {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	raw, err := json.Marshal(ServiceAccountKey{
		Type:        "service_account",
		ProjectID:   "credential-project",
		PrivateKey:  string(pemKey),
		ClientEmail: "test@example.invalid",
		TokenURI:    "https://oauth2.googleapis.com/token",
	})
	if err != nil {
		t.Fatalf("marshal service account: %v", err)
	}
	return string(raw)
}

func runAppStreamEnvelope(t *testing.T, proxy *Proxy, model, prompt string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(map[string]interface{}{
		"model":     model,
		"requestId": "01234567-89ab-cdef-0123-456789abcdef",
		"request": map[string]interface{}{
			"contents": []map[string]interface{}{
				{"role": "user", "parts": []map[string]string{{"text": prompt}}},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1internal:streamGenerateContent", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	proxy.Handler().ServeHTTP(rr, req)
	return rr
}

func decodeAppSSEFrames(t *testing.T, body []byte) []geminiResponse {
	t.Helper()
	var frames []geminiResponse
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var wrapper struct {
			Response geminiResponse `json:"response"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &wrapper); err != nil {
			t.Fatalf("decode SSE frame: %v; line = %q", err, line)
		}
		frames = append(frames, wrapper.Response)
	}
	return frames
}
