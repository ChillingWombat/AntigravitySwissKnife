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

// A reasoning model bills its hidden reasoning against max_tokens. The cascade
// sends a small cap, and when that cap is smaller than the reasoning the model
// streams reasoning_content and no visible text. The cascade rejects that with
// "model output must contain either output text or tool calls", which showed up
// as an intermittent ACP failure while a custom model was active.
func TestGatewayLeavesHeadroomForTextWhenReasoningEnabled(t *testing.T) {
	for _, level := range []string{"low", "medium", "high"} {
		t.Run(level, func(t *testing.T) {
			var upstreamMaxTokens int
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				var payload map[string]interface{}
				_ = json.Unmarshal(body, &payload)
				if v, ok := payload["max_tokens"].(float64); ok {
					upstreamMaxTokens = int(v)
				}
				if payload["reasoning_effort"] != level {
					t.Errorf("reasoning_effort = %v, want %q", payload["reasoning_effort"], level)
				}
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"Red"}}]}`+"\n\n")
				fmt.Fprint(w, "data: [DONE]\n\n")
			}))
			defer upstream.Close()

			store, err := NewStore(t.TempDir())
			if err != nil {
				t.Fatalf("NewStore: %v", err)
			}
			if err := store.SaveModel(CustomModel{
				ID:               "reasoner",
				Name:             "deepseek-chat",
				ProviderType:     ProviderOpenAI,
				BaseURL:          upstream.URL,
				APIKey:           "sk-up",
				Enabled:          true,
				SupportsThinking: true,
				ThinkingLevel:    level,
			}); err != nil {
				t.Fatalf("SaveModel: %v", err)
			}
			gateway := NewGateway(store)

			payload := map[string]interface{}{
				"contents": []map[string]interface{}{
					{"role": "user", "parts": []map[string]interface{}{{"text": "name a color"}}},
				},
				// 64 is the shape of the cap the cascade sends.
				"generationConfig": map[string]interface{}{"maxOutputTokens": 64},
			}
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest(http.MethodPost, "/v1beta/models/reasoner:streamGenerateContent?alt=sse", strings.NewReader(string(body)))
			req.Header.Set("Authorization", "Bearer "+gateway.Token())
			rr := httptest.NewRecorder()
			gateway.Handler().ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
			}
			// Absolute floor, not the constant itself: a test that compares
			// against the value it mutates cannot fail.
			if upstreamMaxTokens < 4096 {
				t.Errorf("max_tokens upstream = %d, want at least 4096 so reasoning cannot starve the answer", upstreamMaxTokens)
			}
			if !strings.Contains(rr.Body.String(), `"text":"Red"`) {
				t.Errorf("expected the answer to reach the client, got %s", rr.Body.String())
			}
		})
	}
}

// A generous cap from the caller must be respected, not clamped.
func TestGatewayKeepsLargeMaxTokens(t *testing.T) {
	var upstreamMaxTokens int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		_ = json.Unmarshal(body, &payload)
		if v, ok := payload["max_tokens"].(float64); ok {
			upstreamMaxTokens = int(v)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"ok"}}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.SaveModel(CustomModel{
		ID:               "reasoner",
		Name:             "deepseek-chat",
		ProviderType:     ProviderOpenAI,
		BaseURL:          upstream.URL,
		APIKey:           "sk-up",
		Enabled:          true,
		SupportsThinking: true,
		ThinkingLevel:    "high",
	}); err != nil {
		t.Fatalf("SaveModel: %v", err)
	}
	gateway := NewGateway(store)

	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": "name a color"}}},
		},
		"generationConfig": map[string]interface{}{"maxOutputTokens": 32768},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1beta/models/reasoner:streamGenerateContent?alt=sse", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+gateway.Token())
	rr := httptest.NewRecorder()
	gateway.Handler().ServeHTTP(rr, req)

	if upstreamMaxTokens != 32768 {
		t.Errorf("max_tokens upstream = %d, want the caller's 32768", upstreamMaxTokens)
	}
}
