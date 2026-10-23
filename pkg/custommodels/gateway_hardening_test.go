package custommodels

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The Gemini surface is reachable from the same port, so it must enforce the PIN
// too when the proxy is exposed externally.
func TestGeminiSurfaceRequiresPinWhenExternal(t *testing.T) {
	store, gateway := newGatewayFixture(t, "http://127.0.0.1:1")
	settings := store.IntegrationSettings()
	settings.Proxy.Scope = ProxyScopeExternal
	settings.Proxy.PIN = "4311"
	if err := store.SetIntegrationSettings(settings); err != nil {
		t.Fatalf("SetIntegrationSettings: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)
	request.Header.Set("Authorization", "Bearer "+gateway.Token())
	recorder := httptest.NewRecorder()
	gateway.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("Gemini surface served without a PIN: %d", recorder.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)
	request.Header.Set("Authorization", "Bearer "+gateway.Token())
	request.Header.Set("X-Swiss-PIN", "4311")
	recorder = httptest.NewRecorder()
	gateway.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("Gemini surface rejected a valid PIN: %d (%s)", recorder.Code, recorder.Body.String())
	}

	// A query-string PIN is not accepted: it would leak into access logs.
	request = httptest.NewRequest(http.MethodGet, "/v1beta/models?pin=4311", nil)
	request.Header.Set("Authorization", "Bearer "+gateway.Token())
	recorder = httptest.NewRecorder()
	gateway.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("query-string PIN was accepted: %d", recorder.Code)
	}
}

// Two calls to the same tool in one turn must keep distinct, recoverable ids.
func TestRepeatedToolCallsKeepDistinctIDs(t *testing.T) {
	request := &geminiRequest{Contents: []geminiContent{
		{Role: "model", Parts: []geminiPart{
			{FunctionCall: &geminiFunctionCall{Name: "read_file", Args: map[string]interface{}{"path": "a"}}},
			{FunctionCall: &geminiFunctionCall{Name: "read_file", Args: map[string]interface{}{"path": "b"}}},
		}},
		{Role: "user", Parts: []geminiPart{
			{FunctionResponse: &geminiFunctionResult{Name: "read_file", Response: map[string]interface{}{"output": "A"}}},
			{FunctionResponse: &geminiFunctionResult{Name: "read_file", Response: map[string]interface{}{"output": "B"}}},
		}},
	}}
	messages, _ := geminiToOpenAIMessages(request)
	var ids []string
	for _, message := range messages {
		if message.Role == "tool" {
			ids = append(ids, message.ToolCallID)
			if name := toolNameFromID(message.ToolCallID); name != "read_file" {
				t.Errorf("tool id %q decodes to %q", message.ToolCallID, name)
			}
		}
	}
	if len(ids) != 2 {
		t.Fatalf("expected two tool messages, got %d", len(ids))
	}
	if ids[0] == ids[1] {
		t.Fatalf("both responses reused the same id: %v", ids)
	}
}
