package custommodels

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func message(role, content string) openAIMessage {
	return openAIMessage{Role: role, Content: content}
}

// The app sends its system prompt as messages[0], so the seed must skip it.
func TestConversationSeedUsesOpeningUserMessage(t *testing.T) {
	turn1 := []openAIMessage{
		message("system", "You are Antigravity."),
		message("user", "first question"),
	}
	turn3 := []openAIMessage{
		message("system", "You are Antigravity."),
		message("user", "first question"),
		message("assistant", "answer"),
		message("user", "follow-up question"),
	}
	seed := conversationSeed(turn1)
	if seed != "first question" {
		t.Fatalf("seed = %q, want the opening user message", seed)
	}
	if grown := conversationSeed(turn3); grown != seed {
		t.Errorf("seed changed as history grew: %q then %q", seed, grown)
	}
}

func TestConversationSeedFallsBackWhenNoUserText(t *testing.T) {
	if seed := conversationSeed([]openAIMessage{message("system", "prompt only")}); seed != "" {
		t.Errorf("expected an empty seed, got %q", seed)
	}
}

// The header must be identical across turns of one conversation and different
// between conversations that open differently.
func TestOpenCodeSessionIDIsStableAndDistinct(t *testing.T) {
	model := &CustomModel{ID: "openai-deepseek-flash-1"}

	first := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	applyProviderHeaders(first, model, "https://opencode.ai/inference/go/openai/v1/chat/completions", "first question")
	later := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	applyProviderHeaders(later, model, "https://opencode.ai/inference/go/openai/v1/chat/completions", "first question")
	other := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	applyProviderHeaders(other, model, "https://opencode.ai/inference/go/openai/v1/chat/completions", "different question")

	id := first.Header.Get("x-opencode-session")
	if id == "" {
		t.Fatal("no session header was set")
	}
	if later.Header.Get("x-opencode-session") != id {
		t.Error("session id changed between turns of the same conversation")
	}
	if other.Header.Get("x-opencode-session") == id {
		t.Error("two different conversations share a session id")
	}
	if ua := first.Header.Get("User-Agent"); ua != "antigravity-swiss-knife/1.0" {
		t.Errorf("user agent = %q", ua)
	}
}

func TestApplyProviderHeadersLeavesOtherProvidersAlone(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	applyProviderHeaders(req, &CustomModel{ID: "m"}, "https://api.deepseek.com/chat/completions", "seed")
	if got := req.Header.Get("x-opencode-session"); got != "" {
		t.Errorf("non-openCode provider received an openCode header: %q", got)
	}
}
