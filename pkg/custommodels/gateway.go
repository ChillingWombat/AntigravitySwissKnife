package custommodels

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// The LLM gateway exposes the Swiss Knife custom models over the wire format the
// Antigravity CLI expects from a bring-your-own-model gateway (AGY_LLM_GATEWAY_URL):
//
//	GET  /v1beta/models                          -> model discovery
//	POST /v1beta/models/{id}:streamGenerateContent?alt=sse -> streaming turn
//	POST /v1beta/models/{id}:generateContent               -> single-shot turn
//
// Requests arrive as Gemini GenerateContent payloads; the gateway translates them
// to the configured provider (OpenAI-compatible, Anthropic, or Gemini API key),
// including tool/function declarations, and answers with Gemini SSE frames. That
// is what makes a custom model usable by the CLI agent instead of just chat.

const gatewayMaxBody = 32 * 1024 * 1024

// Gateway serves custom models to Antigravity surfaces that support a gateway URL.
type Gateway struct {
	store         *Store
	token         string
	AllowLoopback bool
	// client is overridable in tests.
	client *http.Client
}

// NewGateway builds a gateway over the custom model store.
func NewGateway(store *Store) *Gateway {
	return &Gateway{
		store:         store,
		token:         store.GatewayToken(),
		AllowLoopback: core.IsRunningTests(),
		client: &http.Client{
			Timeout: 300 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("stopped after 5 redirects")
				}
				_, _, err := ValidateEndpointSSRF(req.URL.String(), ProviderCustom, core.IsRunningTests())
				return err
			},
		},
	}
}

// Token returns the bearer token surfaces must present.
func (g *Gateway) Token() string { return g.token }

// Handler returns the gateway routes.
func (g *Gateway) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1beta/models", g.handleListModels)
	mux.HandleFunc("/v1beta/models/", g.handleModelAction)
	// OpenAI-shaped surface for surfaces configured with
	// AGY_LLM_GATEWAY_WIRE_PROTOCOL=openai (the app's language server verifies this).
	mux.HandleFunc("/v1/models", g.handleOpenAIListModels)
	mux.HandleFunc("/v1/chat/completions", g.handleOpenAIChatCompletion)
	return mux
}

// authorizeExternal gates the surfaces that outside agents can reach. When the
// integration settings expose the proxy externally with a PIN, that PIN must be
// presented (X-Swiss-PIN header or ?pin=) in addition to the gateway token.
func (g *Gateway) authorizeExternal(r *http.Request) bool {
	settings := g.store.IntegrationSettings()
	if settings.Proxy.Scope != ProxyScopeExternal {
		return true
	}
	pin := strings.TrimSpace(settings.Proxy.PIN)
	if pin == "" {
		return true
	}
	// Header only: a ?pin= query value would be recorded by proxies and access logs.
	presented := strings.TrimSpace(r.Header.Get("X-Swiss-PIN"))
	return subtle.ConstantTimeCompare([]byte(presented), []byte(pin)) == 1
}

// authorize enforces the gateway bearer token.
func (g *Gateway) authorize(r *http.Request) bool {
	if g.token == "" {
		// Fail closed: an unreadable store must not expose the model surface.
		return false
	}
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		header = strings.TrimSpace(r.Header.Get("X-Api-Key"))
		return header == g.token
	}
	return strings.TrimPrefix(header, "Bearer ") == g.token
}

// enabledModels returns the custom models the gateway may serve.
func (g *Gateway) enabledModels() []CustomModel {
	cfg := g.store.GetConfig()
	models := make([]CustomModel, 0, len(cfg.Models))
	for _, m := range cfg.Models {
		if m.Enabled {
			models = append(models, m)
		}
	}
	return models
}

// handleListModels answers the CLI's model discovery probe.
func (g *Gateway) handleListModels(w http.ResponseWriter, r *http.Request) {
	if !g.authorizeExternal(r) {
		log.Printf("[llm-gateway] rejected Gemini model discovery: missing or wrong proxy PIN")
		http.Error(w, "proxy PIN required", http.StatusUnauthorized)
		return
	}
	if !g.authorize(r) {
		log.Printf("[llm-gateway] rejected model discovery: bad gateway token")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	log.Printf("[llm-gateway] model discovery")
	type geminiModel struct {
		Name                       string   `json:"name"`
		DisplayName                string   `json:"displayName"`
		Description                string   `json:"description,omitempty"`
		SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		InputTokenLimit            int      `json:"inputTokenLimit,omitempty"`
		OutputTokenLimit           int      `json:"outputTokenLimit,omitempty"`
	}
	models := make([]geminiModel, 0)
	for _, m := range g.enabledModels() {
		label := m.DisplayName
		if label == "" {
			label = m.Name
		}
		models = append(models, geminiModel{
			Name:                       "models/" + m.ID,
			DisplayName:                label,
			Description:                fmt.Sprintf("%s via %s", m.Name, m.ProviderType),
			SupportedGenerationMethods: []string{"generateContent", "streamGenerateContent"},
			InputTokenLimit:            m.ContextWindow,
		})
	}
	writeGatewayJSON(w, http.StatusOK, map[string]interface{}{"models": models})
}

// handleModelAction routes ".../models/{id}:streamGenerateContent" and siblings.
func (g *Gateway) handleModelAction(w http.ResponseWriter, r *http.Request) {
	if !g.authorizeExternal(r) {
		log.Printf("[llm-gateway] rejected generation: missing or wrong proxy PIN")
		http.Error(w, "proxy PIN required", http.StatusUnauthorized)
		return
	}
	if !g.authorize(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/v1beta/models/")
	modelID, action, found := strings.Cut(rest, ":")
	if !found {
		http.Error(w, "missing action, expected :generateContent or :streamGenerateContent", http.StatusBadRequest)
		return
	}
	modelID = strings.TrimSuffix(strings.Trim(modelID, "/"), "/")

	var target *CustomModel
	for _, m := range g.enabledModels() {
		if m.ID == modelID || m.Name == modelID {
			copy := m
			target = &copy
			break
		}
	}
	if target == nil {
		http.Error(w, fmt.Sprintf("unknown model %q", modelID), http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, gatewayMaxBody))
	if err != nil {
		http.Error(w, "read request: "+err.Error(), http.StatusBadRequest)
		return
	}
	var request geminiRequest
	if err := json.Unmarshal(body, &request); err != nil {
		http.Error(w, "invalid GenerateContent payload: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("[llm-gateway] %s model=%q provider=%s stream=%t", action, modelID, target.ProviderType, strings.HasPrefix(action, "streamGenerateContent"))
	stream := strings.HasPrefix(action, "streamGenerateContent")
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		emit := func(frame geminiResponse) error {
			data, marshalErr := json.Marshal(frame)
			if marshalErr != nil {
				return marshalErr
			}
			if _, writeErr := fmt.Fprintf(w, "data: %s\n\n", data); writeErr != nil {
				return writeErr
			}
			if flusher != nil {
				flusher.Flush()
			}
			return nil
		}
		if err := g.runTurn(r, target, &request, emit); err != nil {
			logProviderFailure("[llm-gateway]", target)
			_ = emit(providerFailureFrame(err))
			return
		}
		_ = emit(geminiResponse{Candidates: []geminiCandidate{{
			Content:      geminiContent{Role: "model", Parts: []geminiPart{}},
			FinishReason: "STOP",
		}}})
		return
	}

	var frames []geminiResponse
	if err := g.runTurn(r, target, &request, func(frame geminiResponse) error {
		frames = append(frames, frame)
		return nil
	}); err != nil {
		writeGatewayJSON(w, http.StatusBadGateway, geminiResponse{Error: &geminiError{Code: 502, Message: err.Error(), Status: "BAD_GATEWAY"}})
		return
	}
	merged := geminiResponse{Candidates: []geminiCandidate{{Content: geminiContent{Role: "model"}, FinishReason: "STOP"}}}
	for _, frame := range frames {
		if len(frame.Candidates) > 0 {
			merged.Candidates[0].Content.Parts = append(merged.Candidates[0].Content.Parts, frame.Candidates[0].Content.Parts...)
			if frame.Candidates[0].FinishReason != "" {
				merged.Candidates[0].FinishReason = frame.Candidates[0].FinishReason
			}
		}
		if frame.UsageMetadata != nil {
			merged.UsageMetadata = frame.UsageMetadata
		}
	}
	writeGatewayJSON(w, http.StatusOK, merged)
}

// runTurn dispatches one translated turn to the model's provider.
func (g *Gateway) runTurn(r *http.Request, model *CustomModel, request *geminiRequest, emit func(geminiResponse) error) error {
	switch model.ProviderType {
	case ProviderAnthropic:
		return g.runAnthropic(r, model, request, emit)
	case ProviderGemini:
		return g.runGeminiAPI(r, model, request, emit)
	case ProviderGCP:
		return g.runVertex(r, model, request, emit)
	default:
		return g.runOpenAICompatible(r, model, request, emit)
	}
}

func logProviderFailure(surface string, model *CustomModel) {
	log.Printf("%s provider failure stage=stream model=%q provider=%s", surface, model.ID, model.ProviderType)
}

const providerFailureTextLimit = 320

// providerFailureFrame ends the stream without discarding the provider failure.
// STOP is the terminal marker Antigravity recognizes; Error remains the result.
func providerFailureFrame(err error) geminiResponse {
	message := strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, strings.TrimSpace(err.Error()))), " ")
	if message == "" {
		message = "unknown provider error"
	}
	runes := []rune(message)
	if len(runes) > providerFailureTextLimit {
		message = string(runes[:providerFailureTextLimit]) + "..."
	}
	message = "Provider request failed: " + message
	return geminiResponse{
		Error: &geminiError{Code: 500, Message: message, Status: "INTERNAL"},
		Candidates: []geminiCandidate{{
			Content:      geminiContent{Role: "model", Parts: []geminiPart{{Text: message}}},
			FinishReason: "STOP",
		}},
	}
}

// thinkingHeadroomTokens is the smallest max_tokens a reasoning model is given.
// Reasoning is billed against that cap, so a smaller one lets the model reason
// itself out of an answer and the caller sees an empty turn.
const thinkingHeadroomTokens = 8192

// ---- provider: OpenAI-compatible chat completions ----

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    interface{}      `json:"content,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAIToolCall struct {
	Index    int    `json:"index,omitempty"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function"`
}

func (g *Gateway) runOpenAICompatible(r *http.Request, model *CustomModel, req *geminiRequest, emit func(geminiResponse) error) error {
	endpoint := ResolveEndpoint(model.ProviderType, model.BaseURL, model.Name)
	if endpoint == "" {
		return fmt.Errorf("model %q has no base URL", model.ID)
	}
	if _, _, err := ValidateEndpointSSRF(endpoint, model.ProviderType, g.AllowLoopback || model.ProviderType == ProviderLocal || model.ProviderType == ProviderCustom); err != nil {
		return err
	}

	messages, callIDs := geminiToOpenAIMessages(req)
	payload := map[string]interface{}{
		"model":    model.Name,
		"messages": messages,
		"stream":   true,
	}
	if len(req.Tools) > 0 {
		if tools := geminiToolsToOpenAI(req.Tools); len(tools) > 0 {
			payload["tools"] = tools
		}
	}
	if gen := req.GenerationConfig; gen != nil {
		if v, ok := gen["temperature"].(float64); ok {
			payload["temperature"] = v
		}
		if v, ok := gen["maxOutputTokens"].(float64); ok && v > 0 {
			payload["max_tokens"] = int(v)
		}
		if v, ok := gen["topP"].(float64); ok {
			payload["top_p"] = v
		}
	}
	if (model.SupportsThinking || model.ThinkingLevel != "") && strings.ToLower(model.ThinkingLevel) != "off" {
		payload["reasoning_effort"] = strings.ToLower(model.ThinkingLevel)
		// Reasoning tokens are billed against max_tokens. The language server's
		// cascade sends a small cap, so a reasoning model can spend the whole
		// budget thinking and stream no visible text at all. The cascade rejects
		// that with "model output must contain either output text or tool calls",
		// which surfaced as an intermittent ACP failure. Leave room for an answer.
		if curMax, ok := payload["max_tokens"].(int); ok && curMax < thinkingHeadroomTokens {
			payload["max_tokens"] = thinkingHeadroomTokens
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if model.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+model.APIKey)
		httpReq.Header.Set("x-api-key", model.APIKey)
	}
	applyProviderHeaders(httpReq, model, endpoint, conversationSeed(messages))

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("call provider: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("provider returned %s: %s", resp.Status, strings.TrimSpace(string(snippet)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	type pendingCall struct {
		id   string
		name string
		args strings.Builder
	}
	var order []int
	pending := map[int]*pendingCall{}
	var usage *geminiUsage

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string           `json:"content"`
					ReasoningContent string           `json:"reasoning_content"`
					Reasoning        string           `json:"reasoning"`
					ToolCalls        []openAIToolCall `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		for _, choice := range chunk.Choices {
			reasoning := choice.Delta.ReasoningContent
			if reasoning == "" {
				reasoning = choice.Delta.Reasoning
			}
			if reasoning != "" {
				if err := emit(geminiResponse{Candidates: []geminiCandidate{{
					Content: geminiContent{Role: "model", Parts: []geminiPart{{Text: reasoning, Thought: true}}},
				}}}); err != nil {
					return err
				}
			}
			if choice.Delta.Content != "" {
				if err := emit(geminiResponse{Candidates: []geminiCandidate{{
					Content: geminiContent{Role: "model", Parts: []geminiPart{{Text: choice.Delta.Content}}},
				}}}); err != nil {
					return err
				}
			}
			for _, call := range choice.Delta.ToolCalls {
				entry, ok := pending[call.Index]
				if !ok {
					entry = &pendingCall{}
					pending[call.Index] = entry
					order = append(order, call.Index)
				}
				if call.ID != "" {
					entry.id = call.ID
				}
				if call.Function.Name != "" {
					entry.name = call.Function.Name
				}
				if call.Function.Arguments != "" {
					entry.args.WriteString(call.Function.Arguments)
				}
			}
		}
		if chunk.Usage != nil {
			usage = &geminiUsage{
				PromptTokenCount:     chunk.Usage.PromptTokens,
				CandidatesTokenCount: chunk.Usage.CompletionTokens,
				TotalTokenCount:      chunk.Usage.TotalTokens,
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read provider stream: %w", err)
	}

	if len(order) > 0 {
		parts := make([]geminiPart, 0, len(order))
		for index, idx := range order {
			entry := pending[idx]
			if entry == nil || entry.name == "" {
				continue
			}
			var args map[string]interface{}
			if raw := strings.TrimSpace(entry.args.String()); raw != "" {
				_ = json.Unmarshal([]byte(raw), &args)
			}
			if args == nil {
				args = map[string]interface{}{}
			}
			id := entry.id
			if id == "" {
				id = toolCallID(entry.name, index)
			}
			if _, used := callIDs[id]; !used {
				callIDs[id] = entry.name
			}
			parts = append(parts, geminiPart{FunctionCall: &geminiFunctionCall{Name: entry.name, Args: args}})
		}
		if len(parts) > 0 {
			if err := emit(geminiResponse{
				Candidates:    []geminiCandidate{{Content: geminiContent{Role: "model", Parts: parts}, FinishReason: "STOP"}},
				UsageMetadata: usage,
			}); err != nil {
				return err
			}
			return nil
		}
	}
	if usage != nil {
		_ = emit(geminiResponse{UsageMetadata: usage})
	}
	return nil
}

// ---- provider: Anthropic messages ----

func (g *Gateway) runAnthropic(r *http.Request, model *CustomModel, req *geminiRequest, emit func(geminiResponse) error) error {
	endpoint := ResolveEndpoint(ProviderAnthropic, model.BaseURL, model.Name)
	if _, _, err := ValidateEndpointSSRF(endpoint, model.ProviderType, g.AllowLoopback || model.ProviderType == ProviderLocal || model.ProviderType == ProviderCustom); err != nil {
		return err
	}
	messages, system := geminiToAnthropicMessages(req)
	payload := map[string]interface{}{
		"model":      model.Name,
		"messages":   messages,
		"max_tokens": 8192,
		"stream":     true,
	}
	if system != "" {
		payload["system"] = system
	}
	if len(req.Tools) > 0 {
		if tools := geminiToolsToAnthropic(req.Tools); len(tools) > 0 {
			payload["tools"] = tools
		}
	}
	if gen := req.GenerationConfig; gen != nil {
		if v, ok := gen["maxOutputTokens"].(float64); ok && v > 0 {
			payload["max_tokens"] = int(v)
		}
		if v, ok := gen["temperature"].(float64); ok {
			payload["temperature"] = v
		}
	}
	if (model.SupportsThinking || model.ThinkingLevel != "") && strings.ToLower(model.ThinkingLevel) != "off" {
		budget := 4096
		switch strings.ToLower(model.ThinkingLevel) {
		case "low":
			budget = 2048
		case "medium":
			budget = 4096
		case "high", "xhigh", "max":
			budget = 8192
		}
		curMax, _ := payload["max_tokens"].(int)
		if curMax <= budget {
			payload["max_tokens"] = budget + 4096
		}
		payload["thinking"] = map[string]interface{}{
			"type":          "enabled",
			"budget_tokens": budget,
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	if model.APIKey != "" {
		httpReq.Header.Set("x-api-key", model.APIKey)
		httpReq.Header.Set("Authorization", "Bearer "+model.APIKey)
	}
	resp, err := g.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("call provider: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("provider returned %s: %s", resp.Status, strings.TrimSpace(string(snippet)))
	}

	type block struct {
		kind  string
		name  string
		id    string
		input strings.Builder
	}
	blocks := map[int]*block{}
	var order []int
	var usage *geminiUsage

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var event struct {
			Type    string `json:"type"`
			Index   int    `json:"index"`
			Content struct {
				Type  string      `json:"type"`
				Name  string      `json:"name"`
				ID    string      `json:"id"`
				Text  string      `json:"text"`
				Input interface{} `json:"input"`
			} `json:"content_block"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				Thinking    string `json:"thinking"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
			Usage struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
			Message struct {
				Usage struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		switch event.Type {
		case "content_block_start":
			entry := &block{kind: event.Content.Type, name: event.Content.Name, id: event.Content.ID}
			blocks[event.Index] = entry
			order = append(order, event.Index)
			if (event.Content.Type == "text" || event.Content.Type == "thinking") && event.Content.Text != "" {
				thought := event.Content.Type == "thinking"
				_ = emit(geminiResponse{Candidates: []geminiCandidate{{
					Content: geminiContent{Role: "model", Parts: []geminiPart{{Text: event.Content.Text, Thought: thought}}},
				}}})
			}
		case "content_block_delta":
			entry := blocks[event.Index]
			if entry == nil {
				continue
			}
			if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
				if err := emit(geminiResponse{Candidates: []geminiCandidate{{
					Content: geminiContent{Role: "model", Parts: []geminiPart{{Text: event.Delta.Text}}},
				}}}); err != nil {
					return err
				}
			}
			if event.Delta.Type == "thinking_delta" && event.Delta.Thinking != "" {
				if err := emit(geminiResponse{Candidates: []geminiCandidate{{
					Content: geminiContent{Role: "model", Parts: []geminiPart{{Text: event.Delta.Thinking, Thought: true}}},
				}}}); err != nil {
					return err
				}
			}
			if event.Delta.PartialJSON != "" {
				entry.input.WriteString(event.Delta.PartialJSON)
			}
		case "message_start":
			usage = &geminiUsage{
				PromptTokenCount:     event.Message.Usage.InputTokens,
				CandidatesTokenCount: event.Message.Usage.OutputTokens,
			}
		case "message_delta":
			if usage == nil {
				usage = &geminiUsage{}
			}
			usage.CandidatesTokenCount = event.Usage.OutputTokens
			if usage.PromptTokenCount > 0 {
				usage.TotalTokenCount = usage.PromptTokenCount + usage.CandidatesTokenCount
			}
		}
	}

	parts := make([]geminiPart, 0)
	for _, idx := range order {
		entry := blocks[idx]
		if entry == nil || entry.kind != "tool_use" || entry.name == "" {
			continue
		}
		var args map[string]interface{}
		if raw := strings.TrimSpace(entry.input.String()); raw != "" {
			_ = json.Unmarshal([]byte(raw), &args)
		}
		if args == nil {
			args = map[string]interface{}{}
		}
		parts = append(parts, geminiPart{FunctionCall: &geminiFunctionCall{Name: entry.name, Args: args}})
	}
	if len(parts) > 0 {
		if err := emit(geminiResponse{
			Candidates:    []geminiCandidate{{Content: geminiContent{Role: "model", Parts: parts}, FinishReason: "STOP"}},
			UsageMetadata: usage,
		}); err != nil {
			return err
		}
		return nil
	}
	if usage != nil {
		_ = emit(geminiResponse{UsageMetadata: usage})
	}
	return nil
}

// ---- provider: Gemini API key (generativelanguage) ----

func (g *Gateway) runGeminiAPI(r *http.Request, model *CustomModel, req *geminiRequest, emit func(geminiResponse) error) error {
	base := strings.TrimRight(model.BaseURL, "/")
	if base == "" {
		base = "https://generativelanguage.googleapis.com/v1beta"
	}
	if !strings.Contains(base, "/v1beta") && !strings.Contains(base, "/v1/") {
		base += "/v1beta"
	}
	endpoint := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse", base, model.Name)
	if _, _, err := ValidateEndpointSSRF(endpoint, model.ProviderType, g.AllowLoopback || model.ProviderType == ProviderLocal || model.ProviderType == ProviderCustom); err != nil {
		return err
	}
	payload := map[string]interface{}{
		"contents":          normalizeGeminiContents(req.Contents),
		"systemInstruction": req.SystemInstruction,
	}
	if req.Tools != nil {
		payload["tools"] = req.Tools
	}
	if req.GenerationConfig != nil {
		payload["generationConfig"] = req.GenerationConfig
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if model.APIKey != "" {
		httpReq.Header.Set("x-goog-api-key", model.APIKey)
	}
	resp, err := g.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("call provider: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("provider returned %s: %s", resp.Status, strings.TrimSpace(string(snippet)))
	}
	// The provider already speaks Gemini SSE: forward the candidate parts.
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var frame geminiResponse
		if err := json.Unmarshal([]byte(data), &frame); err != nil {
			continue
		}
		if frame.Error != nil {
			return fmt.Errorf("provider error: %s", frame.Error.Message)
		}
		if len(frame.Candidates) == 0 && frame.UsageMetadata == nil {
			continue
		}
		if err := emit(frame); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// ---- wire types ----

type geminiRequest struct {
	Contents          []geminiContent        `json:"contents"`
	SystemInstruction *geminiContent         `json:"systemInstruction,omitempty"`
	GenerationConfig  map[string]interface{} `json:"generationConfig,omitempty"`
	Tools             []geminiTool           `json:"tools,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                `json:"text,omitempty"`
	Thought          bool                  `json:"thought,omitempty"`
	FunctionCall     *geminiFunctionCall   `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResult `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args,omitempty"`
}

type geminiFunctionResult struct {
	Name     string                 `json:"name"`
	Response map[string]interface{} `json:"response,omitempty"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDecl `json:"functionDeclarations"`
}

type geminiFunctionDecl struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason,omitempty"`
}

type geminiUsage struct {
	PromptTokenCount     int `json:"promptTokenCount,omitempty"`
	CandidatesTokenCount int `json:"candidatesTokenCount,omitempty"`
	TotalTokenCount      int `json:"totalTokenCount,omitempty"`
}

type geminiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status,omitempty"`
}

type geminiResponse struct {
	Candidates    []geminiCandidate `json:"candidates,omitempty"`
	UsageMetadata *geminiUsage      `json:"usageMetadata,omitempty"`
	Error         *geminiError      `json:"error,omitempty"`
}

// geminiToOpenAIMessages flattens a Gemini payload into OpenAI chat messages and
// returns the synthetic tool-call id map (id -> tool name) used for responses.
// pendingToolID tracks an unanswered tool call so its response can be matched by
// name, in declaration order.
type pendingToolID struct {
	name string
	id   string
	used bool
}

func geminiToOpenAIMessages(req *geminiRequest) ([]openAIMessage, map[string]string) {
	messages := make([]openAIMessage, 0, len(req.Contents)+1)
	callNames := map[string]string{}
	var pendingIDs []pendingToolID
	if req.SystemInstruction != nil {
		if text := partsText(req.SystemInstruction.Parts); text != "" {
			messages = append(messages, openAIMessage{Role: "system", Content: text})
		}
	}
	for _, content := range req.Contents {
		role := content.Role
		if role == "model" {
			role = "assistant"
		}
		if role == "" {
			role = "user"
		}
		var text strings.Builder
		var calls []openAIToolCall
		for _, part := range content.Parts {
			if part.Text != "" {
				text.WriteString(part.Text)
			}
			if part.FunctionCall != nil {
				args, _ := json.Marshal(part.FunctionCall.Args)
				id := toolCallID(part.FunctionCall.Name, len(calls)+len(callNames))
				callNames[id] = part.FunctionCall.Name
				pendingIDs = append(pendingIDs, pendingToolID{name: part.FunctionCall.Name, id: id})
				call := openAIToolCall{ID: id, Type: "function"}
				call.Function.Name = part.FunctionCall.Name
				call.Function.Arguments = string(args)
				calls = append(calls, call)
			}
			if part.FunctionResponse != nil {
				payload, _ := json.Marshal(part.FunctionResponse.Response)
				id := ""
				// Pair responses with unanswered calls of the same name in order, so
				// two calls to one tool keep distinct ids.
				for i := range pendingIDs {
					if !pendingIDs[i].used && pendingIDs[i].name == part.FunctionResponse.Name {
						id = pendingIDs[i].id
						pendingIDs[i].used = true
						break
					}
				}
				if id == "" {
					id = toolCallID(part.FunctionResponse.Name, len(calls))
				}
				messages = append(messages, openAIMessage{Role: "tool", ToolCallID: id, Content: string(payload)})
			}
		}
		if len(calls) > 0 {
			message := openAIMessage{Role: "assistant", ToolCalls: calls}
			if text.Len() > 0 {
				message.Content = text.String()
			}
			messages = append(messages, message)
			continue
		}
		if text.Len() == 0 {
			continue
		}
		messages = append(messages, openAIMessage{Role: role, Content: text.String()})
	}
	return messages, callNames
}

// geminiToAnthropicMessages flattens a Gemini payload into Anthropic messages.
func geminiToAnthropicMessages(req *geminiRequest) ([]map[string]interface{}, string) {
	system := ""
	if req.SystemInstruction != nil {
		system = partsText(req.SystemInstruction.Parts)
	}
	messages := make([]map[string]interface{}, 0, len(req.Contents))
	for _, content := range req.Contents {
		role := content.Role
		if role == "model" {
			role = "assistant"
		}
		if role == "" {
			role = "user"
		}
		blocks := make([]map[string]interface{}, 0, len(content.Parts))
		for _, part := range content.Parts {
			switch {
			case part.Text != "":
				blocks = append(blocks, map[string]interface{}{"type": "text", "text": part.Text})
			case part.FunctionCall != nil:
				blocks = append(blocks, map[string]interface{}{
					"type":  "tool_use",
					"id":    fmt.Sprintf("swiss-call-%s", part.FunctionCall.Name),
					"name":  part.FunctionCall.Name,
					"input": part.FunctionCall.Args,
				})
			case part.FunctionResponse != nil:
				blocks = append(blocks, map[string]interface{}{
					"type":        "tool_result",
					"tool_use_id": fmt.Sprintf("swiss-call-%s", part.FunctionResponse.Name),
					"content":     mustJSON(part.FunctionResponse.Response),
				})
			}
		}
		if len(blocks) == 0 {
			continue
		}
		messages = append(messages, map[string]interface{}{"role": role, "content": blocks})
	}
	return messages, system
}

func geminiToolsToOpenAI(tools []geminiTool) []map[string]interface{} {
	out := make([]map[string]interface{}, 0)
	for _, tool := range tools {
		for _, decl := range tool.FunctionDeclarations {
			out = append(out, map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        decl.Name,
					"description": decl.Description,
					"parameters":  decl.Parameters,
				},
			})
		}
	}
	return out
}

func geminiToolsToAnthropic(tools []geminiTool) []map[string]interface{} {
	out := make([]map[string]interface{}, 0)
	for _, tool := range tools {
		for _, decl := range tool.FunctionDeclarations {
			out = append(out, map[string]interface{}{
				"name":         decl.Name,
				"description":  decl.Description,
				"input_schema": decl.Parameters,
			})
		}
	}
	return out
}

func partsText(parts []geminiPart) string {
	var builder strings.Builder
	for _, part := range parts {
		builder.WriteString(part.Text)
	}
	return builder.String()
}

func mustJSON(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func writeGatewayJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// ---- OpenAI-compatible surface (AGY_LLM_GATEWAY_WIRE_PROTOCOL=openai) ----
//
// The Antigravity surfaces can be told to use an OpenAI-shaped gateway instead of
// the Gemini one. Both are served by this same gateway over the same model store,
// so the CLI and the app see identical first-class model entries.

// handleOpenAIListModels answers GET /v1/models.
func (g *Gateway) handleOpenAIListModels(w http.ResponseWriter, r *http.Request) {
	if !g.authorizeExternal(r) {
		log.Printf("[llm-gateway] rejected OpenAI discovery: missing or wrong proxy PIN")
		http.Error(w, "proxy PIN required", http.StatusUnauthorized)
		return
	}
	if !g.authorize(r) {
		log.Printf("[llm-gateway] rejected OpenAI model discovery: bad gateway token")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	type openAIModel struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		OwnedBy string `json:"owned_by,omitempty"`
	}
	data := make([]openAIModel, 0)
	for _, m := range g.enabledModels() {
		data = append(data, openAIModel{ID: m.ID, Object: "model", OwnedBy: string(m.ProviderType)})
	}
	// Native Gemini models are not advertised: this process cannot execute them (the
	// public Gemini API rejects the app's token scopes, and Cloud Code refuses our
	// client). Native calls stay on the app's own direct path to Google.
	log.Printf("[llm-gateway] OpenAI model discovery: %d models", len(data))
	writeGatewayJSON(w, http.StatusOK, map[string]interface{}{"object": "list", "data": data})
}

// toolCallID builds a stable, name-recoverable tool call id. Encoding the function
// name lets a client echo be mapped back to the function without server state,
// which is what Gemini's FunctionResponse requires.
func toolCallID(name string, index int) string {
	slug := strings.NewReplacer(" ", "_", "/", "_", ":", "_").Replace(strings.TrimSpace(name))
	if slug == "" {
		slug = "tool"
	}
	return fmt.Sprintf("call_%s_%d", slug, index)
}

// toolNameFromID recovers the function name from an id made by toolCallID and
// falls back to the raw id for client-generated ids.
func toolNameFromID(id string) string {
	trimmed := strings.TrimSpace(id)
	if !strings.HasPrefix(trimmed, "call_") {
		return trimmed
	}
	body := strings.TrimPrefix(trimmed, "call_")
	if idx := strings.LastIndex(body, "_"); idx > 0 {
		if _, err := strconv.Atoi(body[idx+1:]); err == nil {
			return body[:idx]
		}
	}
	return body
}

// openAIChatRequest is the inbound shape for POST /v1/chat/completions.
type openAIChatRequest struct {
	Model       string                   `json:"model"`
	Messages    []openAIMessage          `json:"messages"`
	Tools       []map[string]interface{} `json:"tools"`
	Stream      bool                     `json:"stream"`
	MaxTokens   int                      `json:"max_completion_tokens"`
	MaxTokens2  int                      `json:"max_tokens"`
	Temperature *float64                 `json:"temperature"`
}

// handleOpenAIChatCompletion translates one OpenAI chat request onto the model's
// provider and streams the answer back as OpenAI deltas.
func (g *Gateway) handleOpenAIChatCompletion(w http.ResponseWriter, r *http.Request) {
	if !g.authorizeExternal(r) {
		log.Printf("[llm-gateway] rejected chat/completions: missing or wrong proxy PIN")
		http.Error(w, "proxy PIN required", http.StatusUnauthorized)
		return
	}
	if !g.authorize(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, gatewayMaxBody))
	if err != nil {
		http.Error(w, "read request: "+err.Error(), http.StatusBadRequest)
		return
	}
	var request openAIChatRequest
	if err := json.Unmarshal(body, &request); err != nil {
		http.Error(w, "invalid chat completion payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	var target *CustomModel
	for _, m := range g.enabledModels() {
		if m.ID == request.Model || m.Name == request.Model {
			copy := m
			target = &copy
			break
		}
	}
	if target == nil {
		writeGatewayJSON(w, http.StatusNotFound, map[string]interface{}{
			"error": map[string]interface{}{"message": fmt.Sprintf("unknown model %q", request.Model), "type": "invalid_request_error"},
		})
		return
	}

	geminiReq := openAIRequestToGemini(&request)
	log.Printf("[llm-gateway] chat/completions model=%q provider=%s stream=%t", request.Model, target.ProviderType, request.Stream)

	writeChunk := func(delta map[string]interface{}, finish string) error {
		chunk := map[string]interface{}{
			"id":      "chatcmpl-swiss",
			"object":  "chat.completion.chunk",
			"model":   request.Model,
			"choices": []map[string]interface{}{{"index": 0, "delta": delta, "finish_reason": finish}},
		}
		data, marshalErr := json.Marshal(chunk)
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

	if request.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		if err := writeChunk(map[string]interface{}{"role": "assistant", "content": ""}, ""); err != nil {
			return
		}
		toolIndex := 0
		sawToolCall := false
		turnErr := g.runTurn(r, target, geminiReq, func(frame geminiResponse) error {
			for _, candidate := range frame.Candidates {
				for _, part := range candidate.Content.Parts {
					if part.Text != "" {
						if err := writeChunk(map[string]interface{}{"content": part.Text}, ""); err != nil {
							return err
						}
					}
					if part.FunctionCall != nil {
						sawToolCall = true
						args, _ := json.Marshal(part.FunctionCall.Args)
						call := map[string]interface{}{
							"index": toolIndex,
							"id":    toolCallID(part.FunctionCall.Name, toolIndex),
							"type":  "function",
							"function": map[string]interface{}{
								"name":      part.FunctionCall.Name,
								"arguments": string(args),
							},
						}
						if err := writeChunk(map[string]interface{}{"tool_calls": []interface{}{call}}, ""); err != nil {
							return err
						}
						toolIndex++
					}
				}
			}
			return nil
		})
		finish := "stop"
		if sawToolCall {
			finish = "tool_calls"
		}
		if turnErr != nil {
			// An error frame is a text answer: announcing tool_calls here would
			// leave the client waiting for a tool call that never arrives.
			_ = writeChunk(map[string]interface{}{"content": "\nprovider error: " + turnErr.Error()}, "stop")
		} else {
			_ = writeChunk(map[string]interface{}{}, finish)
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		return
	}

	var text strings.Builder
	var calls []interface{}
	turnErr := g.runTurn(r, target, geminiReq, func(frame geminiResponse) error {
		for _, candidate := range frame.Candidates {
			for _, part := range candidate.Content.Parts {
				text.WriteString(part.Text)
				if part.FunctionCall != nil {
					args, _ := json.Marshal(part.FunctionCall.Args)
					calls = append(calls, map[string]interface{}{
						"index":    len(calls),
						"id":       toolCallID(part.FunctionCall.Name, len(calls)),
						"type":     "function",
						"function": map[string]interface{}{"name": part.FunctionCall.Name, "arguments": string(args)},
					})
				}
			}
		}
		return nil
	})
	if turnErr != nil {
		writeGatewayJSON(w, http.StatusBadGateway, map[string]interface{}{
			"error": map[string]interface{}{"message": turnErr.Error(), "type": "provider_error"},
		})
		return
	}
	message := map[string]interface{}{"role": "assistant", "content": text.String()}
	finishReason := "stop"
	if len(calls) > 0 {
		message["tool_calls"] = calls
		finishReason = "tool_calls"
	}
	writeGatewayJSON(w, http.StatusOK, map[string]interface{}{
		"id":      "chatcmpl-swiss",
		"object":  "chat.completion",
		"model":   request.Model,
		"choices": []map[string]interface{}{{"index": 0, "message": message, "finish_reason": finishReason}},
	})
}

// openAIRequestToGemini flattens an OpenAI chat request into the internal Gemini
// shape so the shared provider translation applies.
func openAIRequestToGemini(request *openAIChatRequest) *geminiRequest {
	out := &geminiRequest{}
	for _, message := range request.Messages {
		switch message.Role {
		case "system":
			text := contentText(message.Content)
			out.SystemInstruction = &geminiContent{Role: "user", Parts: []geminiPart{{Text: text}}}
		case "tool":
			var payload map[string]interface{}
			text := contentText(message.Content)
			if text != "" {
				if err := json.Unmarshal([]byte(text), &payload); err != nil {
					// Providers return plain text tool output as often as JSON.
					payload = map[string]interface{}{"output": text}
				}
			}
			if payload == nil {
				payload = map[string]interface{}{}
			}
			out.Contents = append(out.Contents, geminiContent{
				Role: "user",
				Parts: []geminiPart{{FunctionResponse: &geminiFunctionResult{
					Name:     toolNameFromID(message.ToolCallID),
					Response: payload,
				}}},
			})
		case "assistant":
			parts := []geminiPart{}
			if text := contentText(message.Content); text != "" {
				parts = append(parts, geminiPart{Text: text})
			}
			for _, call := range message.ToolCalls {
				var args map[string]interface{}
				if call.Function.Arguments != "" {
					_ = json.Unmarshal([]byte(call.Function.Arguments), &args)
				}
				parts = append(parts, geminiPart{FunctionCall: &geminiFunctionCall{Name: call.Function.Name, Args: args}})
			}
			if len(parts) > 0 {
				out.Contents = append(out.Contents, geminiContent{Role: "model", Parts: parts})
			}
		default:
			if text := contentText(message.Content); text != "" {
				out.Contents = append(out.Contents, geminiContent{Role: "user", Parts: []geminiPart{{Text: text}}})
			}
		}
	}
	for _, tool := range request.Tools {
		function, ok := tool["function"].(map[string]interface{})
		if !ok {
			continue
		}
		decl := geminiFunctionDecl{}
		decl.Name, _ = function["name"].(string)
		decl.Description, _ = function["description"].(string)
		if params, ok := function["parameters"].(map[string]interface{}); ok {
			decl.Parameters = params
		}
		if decl.Name == "" {
			continue
		}
		if len(out.Tools) == 0 {
			out.Tools = []geminiTool{{}}
		}
		out.Tools[0].FunctionDeclarations = append(out.Tools[0].FunctionDeclarations, decl)
	}
	config := map[string]interface{}{}
	if request.MaxTokens > 0 {
		config["maxOutputTokens"] = request.MaxTokens
	} else if request.MaxTokens2 > 0 {
		config["maxOutputTokens"] = request.MaxTokens2
	}
	if request.Temperature != nil {
		config["temperature"] = *request.Temperature
	}
	if len(config) > 0 {
		out.GenerationConfig = config
	}
	return out
}

// contentText reads an OpenAI message content field, which may be a string or an
// array of typed parts.
func contentText(content interface{}) string {
	switch value := content.(type) {
	case string:
		return value
	case []interface{}:
		var builder strings.Builder
		for _, item := range value {
			if part, ok := item.(map[string]interface{}); ok {
				if text, ok := part["text"].(string); ok {
					builder.WriteString(text)
				}
			}
		}
		return builder.String()
	}
	return ""
}

// applyProviderHeaders adds the headers some providers demand beyond auth.
// OpenCode Go requires a client identity and a stable per-conversation session id
// or it rejects the request with 400 MissingSessionID.
func applyProviderHeaders(req *http.Request, model *CustomModel, endpoint, seed string) {
	if !strings.Contains(endpoint, "opencode.ai") {
		return
	}
	req.Header.Set("User-Agent", "antigravity-swiss-knife/1.0")
	if req.Header.Get("x-opencode-session") == "" {
		sum := sha256.Sum256([]byte(model.ID + "\x00" + seed))
		req.Header.Set("x-opencode-session", hex.EncodeToString(sum[:16]))
	}
}

// conversationSeed derives a stable identifier from the opening user message, so
// every turn of one conversation carries the same session id.
func conversationSeed(messages []openAIMessage) string {
	for _, message := range messages {
		if message.Role != "user" {
			continue
		}
		switch content := message.Content.(type) {
		case string:
			if strings.TrimSpace(content) != "" {
				return content
			}
		case nil:
		default:
			if raw, err := json.Marshal(content); err == nil {
				return string(raw)
			}
		}
	}
	return ""
}

// normalizeGeminiContents ensures every turn has an explicit role and that the
// conversation never ends on a model turn (which Google Vertex AI / Gemini
// rejects with "Requests ending with a model turn are not supported.").
func normalizeGeminiContents(contents []geminiContent) []geminiContent {
	if len(contents) == 0 {
		return []geminiContent{{Role: "user", Parts: []geminiPart{{Text: "Continue."}}}}
	}
	out := make([]geminiContent, 0, len(contents)+1)
	for _, c := range contents {
		if strings.TrimSpace(c.Role) == "" {
			c.Role = "user"
		}
		out = append(out, c)
	}
	if strings.EqualFold(strings.TrimSpace(out[len(out)-1].Role), "model") {
		out = append(out, geminiContent{
			Role:  "user",
			Parts: []geminiPart{{Text: "Please continue."}},
		})
	}
	return out
}
