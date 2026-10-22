package custommodels

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newProxyFixture seeds a store with one custom model and returns a proxy whose
// upstream is the given test server.
func newProxyFixture(t *testing.T, upstream string) (*Store, *Proxy) {
	t.Helper()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.SaveModel(CustomModel{
		ID:           "deepseek-proxy",
		Name:         "deepseek-chat",
		DisplayName:  "DeepSeek Proxy",
		ProviderType: ProviderOpenAI,
		BaseURL:      upstream,
		APIKey:       "sk-up",
		Enabled:      true,
	}); err != nil {
		t.Fatalf("SaveModel: %v", err)
	}
	proxy := NewProxy(store)
	proxy.Upstream = upstream
	return store, proxy
}

// Native traffic must reach Google verbatim: same method, path, query, headers,
// body and status.
func TestProxyPassesNativeTrafficThrough(t *testing.T) {
	var seen struct {
		method, path, query, auth, body string
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen.method = r.Method
		seen.path = r.URL.Path
		seen.query = r.URL.RawQuery
		seen.auth = r.Header.Get("Authorization")
		seen.body = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	_, proxy := newProxyFixture(t, upstream.URL)
	body := `{"model":"gemini-4-argon-medium","request":{"contents":[]}}`
	req := httptest.NewRequest(http.MethodPost, "/v1internal:streamGenerateContent?alt=sse", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer ya29.native")
	rr := httptest.NewRecorder()
	proxy.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if seen.path != "/v1internal:streamGenerateContent" || seen.query != "alt=sse" {
		t.Errorf("path/query must be preserved, got %q?%q", seen.path, seen.query)
	}
	if seen.auth != "Bearer ya29.native" {
		t.Errorf("the caller's bearer token must be forwarded, got %q", seen.auth)
	}
	if seen.body != body {
		t.Errorf("body must be forwarded verbatim, got %q", seen.body)
	}
	if rr.Body.String() != `{"ok":true}` {
		t.Errorf("response must pass through, got %q", rr.Body.String())
	}
}

// A custom model on the app path must answer with the Cloud Code envelope; bare
// Gemini frames are rejected by the language server.
func TestProxyWrapsAppSSEFrames(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"pong"}}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	_, proxy := newProxyFixture(t, upstream.URL)
	body := `{"project":"aicode-consumers","requestId":"agent/req-1","model":"deepseek-proxy","request":{"contents":[{"role":"user","parts":[{"text":"say pong"}]}]},"requestType":"agent"}`
	req := httptest.NewRequest(http.MethodPost, "/v1internal:streamGenerateContent?alt=sse", strings.NewReader(body))
	rr := httptest.NewRecorder()
	proxy.Handler().ServeHTTP(rr, req)

	stream := rr.Body.String()
	if !strings.Contains(stream, `"response"`) {
		t.Fatalf("expected the Cloud Code envelope, got %s", stream)
	}
	if !strings.Contains(stream, `"traceId"`) || !strings.Contains(stream, `"metadata"`) {
		t.Errorf("envelope must carry traceId and metadata, got %s", stream)
	}
	if !strings.Contains(stream, `"text":"pong"`) {
		t.Errorf("expected the provider text inside the envelope, got %s", stream)
	}
	// Every frame must parse as the envelope shape.
	for _, line := range strings.Split(stream, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var frame map[string]json.RawMessage
		if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &frame); err != nil {
			t.Fatalf("frame is not JSON: %v", err)
		}
		if _, ok := frame["response"]; !ok {
			t.Errorf("frame missing the response envelope: %s", line)
		}
	}
}

// The same translation on the CLI gateway path must NOT wrap: the CLI wants bare
// Gemini frames.
func TestProxyLeavesGatewayFramesUnwrapped(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"pong"}}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	store, _ := newProxyFixture(t, upstream.URL)
	gateway := NewGateway(store)
	body := `{"contents":[{"role":"user","parts":[{"text":"say pong"}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1beta/models/deepseek-proxy:streamGenerateContent?alt=sse", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+gateway.Token())
	rr := httptest.NewRecorder()
	gateway.Handler().ServeHTTP(rr, req)

	stream := rr.Body.String()
	if strings.Contains(stream, `"traceId"`) {
		t.Errorf("CLI frames must stay bare, got %s", stream)
	}
	if !strings.Contains(stream, `"candidates"`) {
		t.Errorf("expected bare Gemini candidates, got %s", stream)
	}
}

// An app model override routes a native model name to a custom provider.
func TestProxyNeverInterceptsOtherEndpoints(t *testing.T) {
	hits := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{"models":{}}`))
	}))
	defer upstream.Close()

	_, proxy := newProxyFixture(t, upstream.URL)
	for _, path := range []string{"/v1internal:fetchAvailableModels", "/v1internal:loadCodeAssist", "/v1internal:listExperiments"} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		rr := httptest.NewRecorder()
		proxy.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("%s: expected passthrough 200, got %d", path, rr.Code)
		}
	}
	if hits != 3 {
		t.Errorf("expected all three calls upstream, got %d", hits)
	}
}
