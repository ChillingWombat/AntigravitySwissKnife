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
)

func TestProxyExecutesSelectedGCPModelThroughVertex(t *testing.T) {
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

func TestProxyProviderFailureEndsSSEWithSingleError(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	model := CustomModel{
		ID:           "terminal-model",
		Name:         "terminal-upstream",
		ProviderType: ProviderOpenAI,
		BaseURL:      "https://provider.invalid",
		APIKey:       "sk-test",
		Enabled:      true,
	}
	if err := store.SaveModel(model); err != nil {
		t.Fatalf("SaveModel: %v", err)
	}
	proxy := NewProxy(store)
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
	frames := decodeAppSSEFrames(t, rr.Body.Bytes())
	if len(frames) != 1 {
		t.Fatalf("terminal frames = %d, want exactly one error; body = %s", len(frames), rr.Body.String())
	}
	if frames[0].Error == nil {
		t.Fatalf("terminal frame = %+v, want error", frames[0])
	}
	if len(frames[0].Candidates) != 0 {
		t.Fatalf("terminal error also carried candidates: %+v", frames[0])
	}
	logText := logs.String()
	if got := strings.Count(logText, "provider failure"); got != 1 {
		t.Fatalf("provider failure logs = %d, want 1; logs = %q", got, logText)
	}
	if !strings.Contains(logText, `model="terminal-model"`) || !strings.Contains(logText, "provider=openai") {
		t.Fatalf("failure log lacks redacted model/provider context: %q", logText)
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
	frames := decodeAppSSEFrames(t, rr.Body.Bytes())
	if len(frames) != 1 || frames[0].Error == nil {
		t.Fatalf("frames = %+v, want one supported-state error", frames)
	}
	if !strings.Contains(strings.ToLower(frames[0].Error.Message), "service_account") {
		t.Fatalf("error = %q, want service-account validation failure", frames[0].Error.Message)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("Vertex requests = %d, want 0 before credential validation", got)
	}
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
