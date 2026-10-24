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

type gatewayRoundTripFunc func(*http.Request) (*http.Response, error)

func (f gatewayRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// newGatewayFixture seeds a store with one enabled OpenAI-compatible model whose
// base URL points at upstream.
func newGatewayFixture(t *testing.T, upstream string) (*Store, *Gateway) {
	t.Helper()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	model := CustomModel{
		ID:            "deepseek-test",
		Name:          "deepseek-chat",
		DisplayName:   "DeepSeek Test",
		ProviderType:  ProviderOpenAI,
		BaseURL:       upstream,
		APIKey:        "sk-upstream",
		Enabled:       true,
		ContextWindow: 128000,
	}
	if err := store.SaveModel(model); err != nil {
		t.Fatalf("SaveModel: %v", err)
	}
	return store, NewGateway(store)
}

func TestGatewayRequiresBearerToken(t *testing.T) {
	_, gateway := newGatewayFixture(t, "http://127.0.0.1:1")
	handler := gateway.Handler()

	req := httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a token, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)
	req.Header.Set("Authorization", "Bearer "+gateway.Token())
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with the gateway token, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestGatewayModelDiscovery(t *testing.T) {
	_, gateway := newGatewayFixture(t, "http://127.0.0.1:1")
	req := httptest.NewRequest(http.MethodGet, "/v1beta/models?pageSize=1000", nil)
	req.Header.Set("Authorization", "Bearer "+gateway.Token())
	rr := httptest.NewRecorder()
	gateway.Handler().ServeHTTP(rr, req)

	var payload struct {
		Models []struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
		} `json:"models"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v\n%s", err, rr.Body.String())
	}
	if len(payload.Models) != 1 {
		t.Fatalf("expected one model, got %+v", payload)
	}
	if payload.Models[0].Name != "models/deepseek-test" {
		t.Errorf("expected models/<id>, got %q", payload.Models[0].Name)
	}
	if payload.Models[0].DisplayName != "DeepSeek Test" {
		t.Errorf("expected the display label, got %q", payload.Models[0].DisplayName)
	}
}

// The CLI only accepts Gemini SSE frames; verify the translation of a streamed
// OpenAI answer into that shape.
func TestGatewayStreamsGeminiFramesFromOpenAI(t *testing.T) {
	var gotTools []interface{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected upstream path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-upstream" {
			t.Errorf("expected the model API key upstream, got %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		_ = json.Unmarshal(body, &payload)
		if tools, ok := payload["tools"].([]interface{}); ok {
			gotTools = tools
		}
		if payload["model"] != "deepseek-chat" {
			t.Errorf("expected the model name upstream, got %v", payload["model"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"po\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ng\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":2,\"total_tokens\":9}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	_, gateway := newGatewayFixture(t, upstream.URL)

	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "say pong"}}},
		},
		"systemInstruction": map[string]interface{}{
			"role":  "user",
			"parts": []map[string]interface{}{{"text": "you are terse"}},
		},
		"tools": []map[string]interface{}{
			{"functionDeclarations": []map[string]interface{}{
				{"name": "view_file", "description": "read a file", "parameters": map[string]interface{}{"type": "object"}},
			}},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1beta/models/deepseek-test:streamGenerateContent?alt=sse", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+gateway.Token())
	rr := httptest.NewRecorder()
	gateway.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected an SSE response, got %q", ct)
	}
	stream := rr.Body.String()
	if !strings.Contains(stream, "data: ") {
		t.Fatalf("expected SSE frames, got %q", stream)
	}
	var text strings.Builder
	for _, line := range strings.Split(stream, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var frame struct {
			Candidates []struct {
				Content struct {
					Role  string `json:"role"`
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
				FinishReason string `json:"finishReason"`
			} `json:"candidates"`
			UsageMetadata *struct {
				TotalTokenCount int `json:"totalTokenCount"`
			} `json:"usageMetadata"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &frame); err != nil {
			t.Fatalf("frame is not valid JSON: %v (%s)", err, line)
		}
		for _, candidate := range frame.Candidates {
			for _, part := range candidate.Content.Parts {
				text.WriteString(part.Text)
			}
		}
	}
	if text.String() != "pong" {
		t.Errorf("expected the streamed text to concatenate to pong, got %q", text.String())
	}
	// Tool declarations must reach the provider, otherwise the CLI cannot act.
	if len(gotTools) != 1 {
		t.Errorf("expected the function declarations to be forwarded, got %d tools", len(gotTools))
	}
}

// A tool call from an OpenAI-compatible provider must come back as a Gemini
// functionCall part, which is how the CLI receives actions.
func TestGatewayTranslatesToolCalls(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"view_file","arguments":"{\"path\":"}}]}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"a.txt\"}"}}]}}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	_, gateway := newGatewayFixture(t, upstream.URL)
	body := `{"contents":[{"role":"user","parts":[{"text":"read a.txt"}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1beta/models/deepseek-test:streamGenerateContent?alt=sse", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+gateway.Token())
	rr := httptest.NewRecorder()
	gateway.Handler().ServeHTTP(rr, req)

	stream := rr.Body.String()
	if !strings.Contains(stream, `"functionCall"`) || !strings.Contains(stream, "view_file") {
		t.Fatalf("expected a Gemini functionCall frame, got %s", stream)
	}
	if !strings.Contains(stream, `\"path\":\"a.txt\"`) && !strings.Contains(stream, `"path":"a.txt"`) {
		t.Errorf("expected the accumulated arguments to be parsed, got %s", stream)
	}
}

// Follow-up turns carry functionResponse parts; they must become tool messages
// upstream so the model sees the tool result.
func TestGatewayTranslatesFunctionResponses(t *testing.T) {
	var seen map[string]interface{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &seen)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"ok"}}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	_, gateway := newGatewayFixture(t, upstream.URL)
	body := `{"contents":[
		{"role":"user","parts":[{"text":"read a.txt"}]},
		{"role":"model","parts":[{"functionCall":{"name":"view_file","args":{"path":"a.txt"}}}]},
		{"role":"user","parts":[{"functionResponse":{"name":"view_file","response":{"content":"hello"}}}]}
	]}`
	req := httptest.NewRequest(http.MethodPost, "/v1beta/models/deepseek-test:streamGenerateContent", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+gateway.Token())
	rr := httptest.NewRecorder()
	gateway.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}

	messages, _ := seen["messages"].([]interface{})
	roles := make([]string, 0, len(messages))
	for _, raw := range messages {
		if m, ok := raw.(map[string]interface{}); ok {
			role, _ := m["role"].(string)
			roles = append(roles, role)
		}
	}
	joined := strings.Join(roles, ",")
	if !strings.Contains(joined, "tool") {
		t.Fatalf("expected a tool message for the function response, got roles %s", joined)
	}
	if !strings.Contains(joined, "assistant") {
		t.Errorf("expected the earlier functionCall to become an assistant message, got %s", joined)
	}
}

func TestGatewayUnknownModel(t *testing.T) {
	_, gateway := newGatewayFixture(t, "http://127.0.0.1:1")
	req := httptest.NewRequest(http.MethodPost, "/v1beta/models/nope:streamGenerateContent", strings.NewReader(`{"contents":[]}`))
	req.Header.Set("Authorization", "Bearer "+gateway.Token())
	rr := httptest.NewRecorder()
	gateway.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown model, got %d", rr.Code)
	}
}

func TestGatewayReportsProviderFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "quota exceeded", http.StatusTooManyRequests)
	}))
	defer upstream.Close()

	_, gateway := newGatewayFixture(t, upstream.URL)
	body := `{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1beta/models/deepseek-test:streamGenerateContent", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+gateway.Token())
	rr := httptest.NewRecorder()
	gateway.Handler().ServeHTTP(rr, req)

	stream := rr.Body.String()
	if !strings.Contains(stream, "quota exceeded") {
		t.Fatalf("expected the provider error to surface, got %s", stream)
	}
}

func TestGatewayBlocksCloudMetadataSSRF(t *testing.T) {
	targets := []string{
		"http://169.254.169.254/latest/meta-data",
		"http://metadata.google.internal/computeMetadata/v1",
	}
	providers := []ProviderType{ProviderCustom, ProviderAnthropic, ProviderGemini}

	for _, target := range targets {
		for _, provider := range providers {
			t.Run(string(provider)+"/"+target, func(t *testing.T) {
				upstreamCalls := 0
				gateway := &Gateway{
					AllowLoopback: true,
					client: &http.Client{Transport: gatewayRoundTripFunc(func(*http.Request) (*http.Response, error) {
						upstreamCalls++
						return nil, fmt.Errorf("upstream should not be called")
					})},
				}
				model := &CustomModel{
					ID:           "metadata-ssrf",
					Name:         "test-model",
					ProviderType: provider,
					BaseURL:      target,
				}
				req := httptest.NewRequest(http.MethodPost, "/v1beta/models/metadata-ssrf:generateContent", nil)

				err := gateway.runTurn(req, model, &geminiRequest{}, func(geminiResponse) error { return nil })
				if err == nil || !strings.Contains(err.Error(), "ssrf blocked") {
					t.Fatalf("expected cloud metadata SSRF rejection, got %v", err)
				}
				if upstreamCalls != 0 {
					t.Fatalf("metadata URL reached the upstream transport %d times", upstreamCalls)
				}
			})
		}
	}
}
