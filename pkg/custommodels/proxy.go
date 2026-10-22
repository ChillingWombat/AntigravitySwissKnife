package custommodels

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// The Antigravity language server talks to Google's Cloud Code backend, and that
// endpoint can be redirected with CLOUD_CODE_URL. This proxy sits there:
//
//	language_server ──► proxy ──┬─ native model / any other endpoint ──► cloudcode (verbatim)
//	                            └─ Swiss Knife custom model ──► provider, Gemini SSE back
//
// It does NOT inject custom ids into the model list: the client curates that list
// and discards unknown ids (verified against 2.25.0). Custom models reach the app
// through the OpenAI-shaped gateway surface instead.

const (
	proxyUpstreamDefault = "https://daily-cloudcode-pa.googleapis.com"
	proxyBodyLimit       = 32 * 1024 * 1024
)

// Proxy is the Cloud Code pass-through + custom model translator.
type Proxy struct {
	store *Store
	gw    *Gateway
	// Upstream is the real Cloud Code endpoint (overridable in tests).
	Upstream string
	client   *http.Client
}

// NewProxy builds a proxy over the custom model store.
func NewProxy(store *Store) *Proxy {
	return &Proxy{
		store:    store,
		gw:       NewGateway(store),
		Upstream: proxyUpstreamDefault,
		client: &http.Client{
			Timeout: 0, // streaming: no client-side deadline
		},
	}
}

// Handler serves the /v1internal:* surface the language server calls.
func (p *Proxy) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1internal") {
			p.handle(w, r)
			return
		}
		http.NotFound(w, r)
	})
}

// handle routes one Cloud Code call.
func (p *Proxy) handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, proxyBodyLimit))
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}

	action := r.URL.Path
	if idx := strings.LastIndex(action, ":"); idx >= 0 {
		action = action[idx+1:]
	}

	if action == "streamGenerateContent" || action == "generateContent" {
		// /v1internal is the language server (app); /v1beta is the CLI gateway.
		if p.tryCustomModel(w, r, body, action == "streamGenerateContent", strings.HasPrefix(r.URL.Path, "/v1internal")) {
			return
		}
	}

	// Everything else: verbatim pass-through, including the model list, so the
	// app keeps working exactly as it does without the proxy.
	// No body rewriting: the app curates its selectable model ids client-side, so
	// extra entries in fetchAvailableModels are discarded (verified against 2.25.0).
	// Custom inference is reached through app model overrides instead.
	passthrough(w, r, body, p.Upstream, p.client)
}

// tryCustomModel answers a generation call locally when the requested model is a
// Swiss Knife custom model. It reports whether it handled the request.
func (p *Proxy) tryCustomModel(w http.ResponseWriter, r *http.Request, body []byte, stream, wrapped bool) bool {
	var envelope struct {
		Model     string          `json:"model"`
		RequestID string          `json:"requestId"`
		Request   json.RawMessage `json:"request"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || strings.TrimSpace(envelope.Model) == "" {
		return false // not an envelope we understand: let it pass through
	}
	model := p.findCustomModel(envelope.Model)
	if model == nil {
		return false
	}

	var request geminiRequest
	if len(envelope.Request) > 0 {
		if err := json.Unmarshal(envelope.Request, &request); err != nil {
			http.Error(w, "invalid request payload: "+err.Error(), http.StatusBadRequest)
			return true
		}
	}
	log.Printf("[cloudcode-proxy] %s model=%q -> custom %q (%s)", r.URL.Path, envelope.Model, model.ID, model.ProviderType)

	requestID := envelope.RequestID
	emit := func(frame geminiResponse) error {
		payload := interface{}(frame)
		if wrapped {
			// The language server expects the Cloud Code envelope around each
			// frame; bare Gemini frames are silently rejected and the turn hangs.
			payload = map[string]interface{}{
				"response": frame,
				"traceId":  eventTraceID(requestID),
				"metadata": map[string]interface{}{},
			}
		}
		data, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return marshalErr
		}
		if _, writeErr := fmt.Fprintf(w, "data: %s\n\n", data); writeErr != nil {
			return writeErr
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		return nil
	}

	if !stream {
		var frames []geminiResponse
		if err := p.gw.runTurn(r, model, &request, func(frame geminiResponse) error {
			frames = append(frames, frame)
			return nil
		}); err != nil {
			writeGatewayJSON(w, http.StatusBadGateway, geminiResponse{Error: &geminiError{Code: 502, Message: err.Error(), Status: "BAD_GATEWAY"}})
			return true
		}
		merged := geminiResponse{Candidates: []geminiCandidate{{Content: geminiContent{Role: "model"}, FinishReason: "STOP"}}}
		for _, frame := range frames {
			if len(frame.Candidates) > 0 {
				merged.Candidates[0].Content.Parts = append(merged.Candidates[0].Content.Parts, frame.Candidates[0].Content.Parts...)
			}
			if frame.UsageMetadata != nil {
				merged.UsageMetadata = frame.UsageMetadata
			}
		}
		writeGatewayJSON(w, http.StatusOK, merged)
		return true
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	if err := p.gw.runTurn(r, model, &request, emit); err != nil {
		_ = emit(geminiResponse{Error: &geminiError{Code: 500, Message: err.Error(), Status: "INTERNAL"}})
	}
	_ = emit(geminiResponse{Candidates: []geminiCandidate{{
		Content:      geminiContent{Role: "model", Parts: []geminiPart{}},
		FinishReason: "STOP",
	}}})
	return true
}

// eventTraceID builds a stable trace id for the wrapped SSE envelope.
func eventTraceID(requestID string) string {
	if trimmed := strings.TrimSpace(requestID); trimmed != "" {
		return trimmed
	}
	return "swiss-cloudcode"
}

// findCustomModel matches the model id the language server sends against the
// enabled custom models.
func (p *Proxy) findCustomModel(modelID string) *CustomModel {
	wanted := strings.TrimSpace(modelID)
	if wanted == "" {
		return nil
	}
	// Cloud Code model names arrive as "models/<id>" or bare ids.
	wanted = strings.TrimPrefix(wanted, "models/")
	for _, m := range p.gw.enabledModels() {
		if m.ID == wanted || m.Name == wanted || strings.EqualFold(m.DisplayName, modelID) {
			copy := m
			return &copy
		}
	}
	return nil
}

// passthrough forwards one request to the upstream and relays the answer. A nil
// rewrite passes the body through untouched.
func passthrough(w http.ResponseWriter, r *http.Request, body []byte, upstream string, client *http.Client) {
	target := strings.TrimRight(upstream, "/") + r.URL.Path
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, target, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "build upstream request: "+err.Error(), http.StatusBadGateway)
		return
	}
	for key, values := range r.Header {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	req.Header.Set("Host", "")
	req.Host = ""

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[cloudcode-proxy] %s upstream error: %v", r.URL.Path, err)
		http.Error(w, "upstream: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if readErr != nil {
			return
		}
	}
}

func isHopByHopHeader(key string) bool {
	switch http.CanonicalHeaderKey(key) {
	case "Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization",
		"Te", "Trailer", "Transfer-Encoding", "Upgrade", "Content-Length":
		return true
	}
	return false
}
