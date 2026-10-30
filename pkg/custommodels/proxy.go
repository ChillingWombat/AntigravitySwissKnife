package custommodels

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
)

var (
	uuidRegex                 = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	userSettingsSentenceRegex = regexp.MustCompile(`(?i)(The user changed setting\s+[` + "`" + `']Model Selection[` + "`" + `'].*?to\s+)(.+?)(\.(?:\s+No need to comment|\s*$|\s*[\r\n]|</USER_SETTINGS_CHANGE>))`)
)

func formatEffortLabel(lvl string) string {
	switch strings.ToLower(strings.TrimSpace(lvl)) {
	case "low":
		return "Low"
	case "medium":
		return "Medium"
	case "high":
		return "High"
	case "extra", "xhigh":
		return "Extra"
	case "max":
		return "Max"
	case "", "off":
		return ""
	default:
		s := strings.TrimSpace(lvl)
		if len(s) > 0 {
			return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
		}
		return s
	}
}

func extractConversationID(requestID string, requestRaw json.RawMessage) string {
	if m := uuidRegex.FindString(requestID); m != "" {
		return m
	}
	if len(requestRaw) > 0 {
		var req struct {
			CascadeID     string `json:"cascade_id"`
			RootCascadeID string `json:"root_cascade_id"`
		}
		if err := json.Unmarshal(requestRaw, &req); err == nil {
			if m := uuidRegex.FindString(req.CascadeID); m != "" {
				return m
			}
			if m := uuidRegex.FindString(req.RootCascadeID); m != "" {
				return m
			}
		}
	}
	return ""
}

// Source of truth: transcript.jsonl is host-owned and never written by Swiss
// Knife. conversation_models.json is authoritative only for custom-model
// attribution.
func syncModelIdentityInRequest(req *geminiRequest, model *CustomModel) {
	if req == nil || model == nil {
		return
	}
	targetModelText := model.DisplayName
	if targetModelText == "" {
		targetModelText = model.Name
	}
	if model.ThinkingLevel != "" && strings.ToLower(model.ThinkingLevel) != "off" {
		targetModelText += fmt.Sprintf(" (%s)", formatEffortLabel(model.ThinkingLevel))
	}

	for i := range req.Contents {
		for j := range req.Contents[i].Parts {
			text := req.Contents[i].Parts[j].Text
			if strings.Contains(text, "<USER_SETTINGS_CHANGE>") {
				req.Contents[i].Parts[j].Text = userSettingsSentenceRegex.ReplaceAllString(text, "${1}"+targetModelText+"${3}")
			}
		}
	}

	instruction := fmt.Sprintf("\n[Active Model Environment]: Underlying model is configured as %s (%s). If the user asks what model you are, truthfully identify yourself as %s.",
		model.DisplayName, model.Name, targetModelText)
	if req.SystemInstruction == nil {
		req.SystemInstruction = &geminiContent{
			Role:  "system",
			Parts: []geminiPart{{Text: instruction}},
		}
	} else {
		req.SystemInstruction.Parts = append(req.SystemInstruction.Parts, geminiPart{Text: instruction})
	}
}

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
	catalog  catalogCache
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

	if isCatalogAction(action) {
		p.serveCatalog(w, r, body)
		return
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
	if err := json.Unmarshal(body, &envelope); err != nil {
		return false
	}
	if len(envelope.Request) == 0 && strings.TrimSpace(envelope.Model) == "" {
		return false // not an envelope we understand: let it pass through
	}
	model := p.findCustomModel(envelope.Model)
	if model == nil {
		isCLI := strings.Contains(r.Header.Get("User-Agent"), "antigravity/cli")
		if !isCLI || strings.TrimSpace(envelope.Model) == "" {
			model = p.activeCustomModel()
			if model != nil && model.ThinkingLevel == "" && model.SupportsThinking {
				lowerModel := strings.ToLower(envelope.Model)
				for _, lvl := range []string{"high", "medium", "low", "xhigh", "max"} {
					if strings.HasSuffix(lowerModel, "-"+lvl) {
						model.ThinkingLevel = lvl
						break
					}
				}
			}
		}
	}
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

	convID := extractConversationID(envelope.RequestID, envelope.Request)
	if convID != "" && p.store != nil {
		if err := p.store.RecordConversationModel(convID, model.ID); err != nil {
			log.Printf("[cloudcode-proxy] persist conversation model %s: %v", convID, err)
		}
	}
	syncModelIdentityInRequest(&request, model)

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
	// Match with thinking effort suffix stripped, e.g. "deepseek-v4.1-flash-high" -> "deepseek-v4.1-flash"
	lower := strings.ToLower(wanted)
	for _, lvl := range []string{"high", "medium", "low", "xhigh", "max"} {
		if strings.HasSuffix(lower, "-"+lvl) {
			base := wanted[:len(wanted)-len(lvl)-1]
			for _, m := range p.gw.enabledModels() {
				if m.ID == base || m.Name == base || strings.EqualFold(m.DisplayName, base) {
					copy := m
					copy.ThinkingLevel = lvl
					return &copy
				}
			}
		}
	}
	return nil
}

// activeCustomModel returns the custom model that should override native model
// execution, or nil if native models should execute unchanged.
func (p *Proxy) activeCustomModel() *CustomModel {
	if p.store == nil {
		return nil
	}
	cfg := p.store.GetConfig()
	activeID := strings.TrimSpace(cfg.ActiveModelID)
	enabled := p.gw.enabledModels()
	if activeID != "" {
		for _, m := range enabled {
			if m.ID == activeID || m.Name == activeID {
				copy := m
				return &copy
			}
		}
	}

	mode := strings.ToLower(strings.TrimSpace(cfg.Integration.Mode))
	isCustomOnly := mode == "custom_only"
	if !isCustomOnly && cfg.Integration.Apps != nil {
		if app, ok := cfg.Integration.Apps["antigravity"]; ok && strings.EqualFold(app.Source, "custom") {
			isCustomOnly = true
		}
	}
	if isCustomOnly || cfg.GeminiQuotaExhausted {
		defID := strings.TrimSpace(cfg.DefaultCustomModel)
		if defID != "" {
			for _, m := range enabled {
				if m.ID == defID || m.Name == defID {
					copy := m
					return &copy
				}
			}
		}
		for _, m := range enabled {
			if m.IsDefault {
				copy := m
				return &copy
			}
		}
		if len(enabled) > 0 {
			copy := enabled[0]
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
		"Te", "Trailer", "Transfer-Encoding", "Upgrade", "Content-Length",
		"Accept-Encoding", "Content-Encoding":
		return true
	}
	return false
}
