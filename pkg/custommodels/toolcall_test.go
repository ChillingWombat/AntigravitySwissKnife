package custommodels

import (
	"strings"
	"testing"
)

func TestToolCallIDRoundTripsThroughFunctionName(t *testing.T) {
	names := []string{"read_file", "search.web", "very_long_tool_name", "noargs"}
	for i, name := range names {
		id := toolCallID(name, i)
		if got := toolNameFromID(id); got != name {
			t.Errorf("toolNameFromID(%q) = %q, want %q", id, got, name)
		}
		if !strings.HasPrefix(id, "call_") {
			t.Errorf("id %q lacks the call_ prefix", id)
		}
	}
	// Client-generated ids fall back to themselves rather than being mangled.
	if got := toolNameFromID("abc123"); got != "abc123" {
		t.Errorf("foreign id should pass through, got %q", got)
	}
}

// Gemini requires FunctionResponse.Name to be the function name, and plain-text
// tool output must survive the round trip.
func TestToolMessageConvertsToFunctionResponse(t *testing.T) {
	request := &openAIChatRequest{
		Messages: []openAIMessage{
			{Role: "user", Content: "read the file"},
			{Role: "assistant", ToolCalls: []openAIToolCall{newToolCall(toolCallID("read_file", 0), "read_file", `{"path":"a.txt"}`)}},
			{Role: "tool", ToolCallID: toolCallID("read_file", 0), Content: "plain text output"},
		},
	}
	converted := openAIRequestToGemini(request)
	if len(converted.Contents) != 3 {
		t.Fatalf("expected three contents, got %d", len(converted.Contents))
	}
	last := converted.Contents[2].Parts[0].FunctionResponse
	if last == nil {
		t.Fatal("tool message did not convert to a function response")
	}
	if last.Name != "read_file" {
		t.Errorf("FunctionResponse.Name = %q, want the function name", last.Name)
	}
	if got, _ := last.Response["output"].(string); got != "plain text output" {
		t.Errorf("plain text output was dropped: %+v", last.Response)
	}
}

// newToolCall builds an assistant tool call with the package's real struct type.
func newToolCall(id, name, args string) openAIToolCall {
	var call openAIToolCall
	call.ID = id
	call.Type = "function"
	call.Function.Name = name
	call.Function.Arguments = args
	return call
}
