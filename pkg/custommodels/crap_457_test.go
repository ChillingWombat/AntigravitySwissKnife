package custommodels

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type crap457FailingWriter struct{}

func (crap457FailingWriter) Header() http.Header        { return http.Header{} }
func (crap457FailingWriter) WriteHeader(statusCode int) {}
func (crap457FailingWriter) Write([]byte) (int, error)  { return 0, errors.New("write failed") }

type crap457ErrorReader struct{}

func (crap457ErrorReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestCRAP457GatewayModelActionHelpers(t *testing.T) {
	store, gateway := newGatewayFixture(t, "http://127.0.0.1:1")
	if store == nil {
		t.Fatal("fixture returned no store")
	}
	if got := gateway.findEnabledModel("deepseek-test"); got == nil || got.ID != "deepseek-test" {
		t.Fatalf("findEnabledModel by ID = %+v", got)
	}
	if got := gateway.findEnabledModel("deepseek-chat"); got == nil || got.ID != "deepseek-test" {
		t.Fatalf("findEnabledModel by Name = %+v", got)
	}
	if got := gateway.findEnabledModel("missing"); got != nil {
		t.Fatalf("findEnabledModel(missing) = %+v", got)
	}

	rr := httptest.NewRecorder()
	modelID, action, ok := parseModelAction(rr, httptest.NewRequest(http.MethodPost, "/v1beta/models/deepseek-test:generateContent", nil))
	if !ok || modelID != "deepseek-test" || action != "generateContent" {
		t.Fatalf("parseModelAction = %q, %q, %t", modelID, action, ok)
	}
	rr = httptest.NewRecorder()
	if _, _, ok := parseModelAction(rr, httptest.NewRequest(http.MethodPost, "/v1beta/models/deepseek-test", nil)); ok || rr.Code != http.StatusBadRequest {
		t.Fatalf("missing action ok=%t status=%d", ok, rr.Code)
	}

	request, ok := readModelActionRequest(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"contents":[]}`)))
	if !ok || request == nil {
		t.Fatal("readModelActionRequest rejected valid JSON")
	}
	rr = httptest.NewRecorder()
	if _, ok := readModelActionRequest(rr, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{`))); ok || rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON ok=%t status=%d", ok, rr.Code)
	}

	frames := mergeGeminiFrames([]geminiResponse{
		{Candidates: []geminiCandidate{{Content: geminiContent{Role: "model", Parts: []geminiPart{{Text: "one"}}}}}},
		{Candidates: []geminiCandidate{{Content: geminiContent{Role: "model", Parts: []geminiPart{{Text: "two"}}}, FinishReason: "MAX_TOKENS"}}},
	})
	if len(frames.Candidates[0].Content.Parts) != 2 || frames.Candidates[0].FinishReason != "MAX_TOKENS" {
		t.Fatalf("mergeGeminiFrames = %+v", frames)
	}

	emit := newGeminiSSEEmitter(httptest.NewRecorder())
	if err := emit(geminiResponse{Candidates: []geminiCandidate{}}); err != nil {
		t.Fatalf("gemini SSE emitter: %v", err)
	}
	if err := newGeminiSSEEmitter(crap457FailingWriter{})(geminiResponse{}); err == nil {
		t.Fatal("gemini SSE emitter ignored write failure")
	}
}

func TestCRAP457ProxyEnvelopeAndEmitter(t *testing.T) {
	store, _ := newGatewayFixture(t, "http://127.0.0.1:1")
	proxy := NewProxy(store)

	for _, body := range []string{`{`, `{}`} {
		if _, ok := decodeCustomModelEnvelope([]byte(body)); ok {
			t.Fatalf("decodeCustomModelEnvelope(%q) succeeded", body)
		}
	}
	envelope, ok := decodeCustomModelEnvelope([]byte(`{"model":"deepseek-test","request":{"contents":[]}}`))
	if !ok || envelope.Model != "deepseek-test" {
		t.Fatalf("valid envelope = %+v, %t", envelope, ok)
	}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	if model := proxy.resolveCustomModel(req, "deepseek-test"); model == nil || model.ID != "deepseek-test" {
		t.Fatalf("resolveCustomModel(selected) = %+v", model)
	}

	rr := httptest.NewRecorder()
	if request, handled := decodeCustomModelRequest(rr, &customModelEnvelope{Request: []byte(`{`)}); handled != true || request != nil || rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid request handled=%t status=%d", handled, rr.Code)
	}

	for _, wrapped := range []bool{false, true} {
		rr = httptest.NewRecorder()
		if err := newProxySSEEmitter(rr, wrapped, "request-1")(geminiResponse{Candidates: []geminiCandidate{}}); err != nil {
			t.Fatalf("proxy SSE emitter wrapped=%t: %v", wrapped, err)
		}
		if !strings.Contains(rr.Body.String(), "data: ") {
			t.Fatalf("proxy SSE emitter wrapped=%t body=%q", wrapped, rr.Body.String())
		}
	}
	if err := newProxySSEEmitter(crap457FailingWriter{}, false, "")(geminiResponse{}); err == nil {
		t.Fatal("proxy SSE emitter ignored write failure")
	}
}

func TestCRAP457VertexHelpersAndSSE(t *testing.T) {
	model := &CustomModel{Name: "gemini-2.5-pro", GCPRegion: "us-central1"}
	key := &ServiceAccountKey{ProjectID: "key-project"}
	if got, err := vertexProjectID(model, key); err != nil || got != "key-project" {
		t.Fatalf("vertexProjectID fallback = %q, %v", got, err)
	}
	if _, err := vertexProjectID(&CustomModel{}, &ServiceAccountKey{}); err == nil {
		t.Fatal("vertexProjectID accepted missing project")
	}
	if got := vertexLocation(&CustomModel{}); got != "global" {
		t.Fatalf("vertexLocation default = %q", got)
	}
	if publisher, name, err := vertexTarget(model); err != nil || publisher == "" || name == "" {
		t.Fatalf("vertexTarget = %q, %q, %v", publisher, name, err)
	}
	if _, _, err := vertexTarget(&CustomModel{}); err == nil {
		t.Fatal("vertexTarget accepted missing model")
	}
	if got, err := vertexBaseURL(model, "global"); err != nil || got != "https://aiplatform.googleapis.com" {
		t.Fatalf("vertexBaseURL global = %q, %v", got, err)
	}
	if got, err := vertexBaseURL(model, "us-central1"); err != nil || got != "https://us-central1-aiplatform.googleapis.com" {
		t.Fatalf("vertexBaseURL region = %q, %v", got, err)
	}
	if _, err := vertexBaseURL(&CustomModel{BaseURL: "aiplatform.googleapis.com"}, "global"); err == nil {
		t.Fatal("vertexBaseURL accepted a URL without scheme")
	}

	for _, tc := range []struct {
		line string
		want bool
	}{
		{"event: ignored", false},
		{"data:", false},
		{"data: [DONE]", false},
		{"data: {\"candidates\":[]}", true},
	} {
		if _, ok := parseVertexSSELine(tc.line); ok != tc.want {
			t.Fatalf("parseVertexSSELine(%q) = %t", tc.line, ok)
		}
	}
	if _, err := decodeVertexSSEFrame([]byte(`{`)); err == nil {
		t.Fatal("decodeVertexSSEFrame accepted malformed JSON")
	}
	if _, err := decodeVertexSSEFrame([]byte(`{"error":{"message":""}}`)); err == nil || !strings.Contains(err.Error(), "unknown provider error") {
		t.Fatalf("empty provider error = %v", err)
	}

	var emitted int
	err := consumeVertexSSE(strings.NewReader("event: ignored\ndata:\ndata: {\"candidates\":[]}\n"), func(geminiResponse) error {
		emitted++
		return nil
	})
	if err != nil || emitted != 1 {
		t.Fatalf("consumeVertexSSE success err=%v emitted=%d", err, emitted)
	}
	for _, tc := range []struct {
		name string
		body io.Reader
		want string
	}{
		{"malformed", strings.NewReader("data: {\n"), "decode Vertex stream"},
		{"provider", strings.NewReader("data: {\"error\":{\"message\":\"quota\"}}\n"), "Vertex provider error: quota"},
		{"empty", strings.NewReader("event: ignored\n"), "no response frames"},
		{"reader", crap457ErrorReader{}, "read Vertex stream"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := consumeVertexSSE(tc.body, func(geminiResponse) error { return nil })
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("consumeVertexSSE error = %v, want %q", err, tc.want)
			}
		})
	}
	err = consumeVertexSSE(bytes.NewBufferString("data: {\"candidates\":[]}\n"), func(geminiResponse) error { return errors.New("emit failed") })
	if err == nil || !strings.Contains(err.Error(), "emit failed") {
		t.Fatalf("consumeVertexSSE emit failure = %v", err)
	}
}
