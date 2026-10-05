package custommodels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// ModelInfo describes a fetched or detected model option.
type ModelInfo struct {
	ID               string   `json:"id"`
	DisplayName      string   `json:"display_name"`
	ContextWindow    int      `json:"context_window"`
	SupportsThinking bool     `json:"supports_thinking"`
	ThinkingLevels   []string `json:"thinking_levels,omitempty"`
	Description      string   `json:"description,omitempty"`
}

// FetchModelsRequest specifies the provider credentials and endpoint to query.
type FetchModelsRequest struct {
	ProviderType ProviderType `json:"provider_type"`
	BaseURL      string       `json:"base_url"`
	APIKey       string       `json:"api_key,omitempty"`
}

// FetchModelsResponse contains the list of discovered models or an error.
type FetchModelsResponse struct {
	Success bool        `json:"success"`
	Models  []ModelInfo `json:"models"`
	Message string      `json:"message,omitempty"`
}

// FetchModels queries the remote API for available model identifiers and metadata.
func FetchModels(req FetchModelsRequest) FetchModelsResponse {
	client := &http.Client{Timeout: 8 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	baseURL := strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")

	switch req.ProviderType {
	case ProviderGemini:
		return fetchGeminiModels(ctx, client, baseURL, req.APIKey)
	case ProviderAnthropic:
		return fetchAnthropicModels(ctx, client, baseURL, req.APIKey)
	case ProviderCustom:
		// For Custom, try OpenAI-style /models on base URL or raw endpoint
		return fetchOpenAIModels(ctx, client, baseURL, req.APIKey, true)
	case ProviderOpenAI, ProviderLocal:
		fallthrough
	default:
		return fetchOpenAIModels(ctx, client, baseURL, req.APIKey, false)
	}
}

// detectModelMetadata infers context window and reasoning/thinking capabilities.
func detectModelMetadata(id string, displayName string, rawLimit int) (int, bool, []string) {
	lower := strings.ToLower(id + " " + displayName)
	ctx := 1000000 // Default 1,000,000 tokens

	if rawLimit > 0 {
		ctx = rawLimit
	} else if strings.Contains(lower, "gemini-2.5-pro") || strings.Contains(lower, "gemini-1.5-pro") {
		ctx = 2097152
	} else if strings.Contains(lower, "gemini-2.5-flash") || strings.Contains(lower, "gemini-2.0-flash") || strings.Contains(lower, "gemini-1.5-flash") {
		ctx = 1048576
	} else if strings.Contains(lower, "claude-3-7") || strings.Contains(lower, "claude-3-5") || strings.Contains(lower, "claude-3-opus") {
		ctx = 200000
	} else if strings.Contains(lower, "o1") || strings.Contains(lower, "o3") {
		ctx = 200000
	} else if strings.Contains(lower, "gpt-4o") || strings.Contains(lower, "deepseek-v3") || strings.Contains(lower, "deepseek-chat") || strings.Contains(lower, "deepseek-r1") || strings.Contains(lower, "llama-3.3") {
		ctx = 128000
	}

	supportsThinking := false
	var thinkingLevels []string

	// Detect reasoning / thinking models
	if strings.Contains(lower, "o1") ||
		strings.Contains(lower, "o3") ||
		strings.Contains(lower, "r1") ||
		strings.Contains(lower, "reasoner") ||
		strings.Contains(lower, "thinking") ||
		strings.Contains(lower, "claude-3-7") ||
		strings.Contains(lower, "qwq") ||
		strings.Contains(lower, "gemini-2.5") {
		supportsThinking = true
		thinkingLevels = []string{"off", "low", "medium", "high"}
	}

	return ctx, supportsThinking, thinkingLevels
}

// cleanDisplayName creates a friendly label from model ID.
func cleanDisplayName(id string, existingName string) string {
	if existingName != "" && existingName != id {
		return existingName
	}
	name := id
	// Strip vendor prefixes like "models/", "anthropic/", "deepseek/"
	if idx := strings.LastIndex(name, "/"); idx != -1 {
		name = name[idx+1:]
	}
	parts := strings.Split(name, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func cleanErrorMessage(statusCode int, body []byte) string {
	var errObj struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
		Detail  string `json:"detail"`
	}
	if json.Unmarshal(body, &errObj) == nil {
		if errObj.Error.Message != "" {
			return errObj.Error.Message
		}
		if errObj.Message != "" {
			return errObj.Message
		}
		if errObj.Detail != "" {
			return errObj.Detail
		}
	}
	raw := strings.TrimSpace(string(body))
	if strings.HasPrefix(raw, "<") || strings.Contains(raw, "<html") || strings.Contains(raw, "<!DOCTYPE") {
		statusText := http.StatusText(statusCode)
		if statusText == "" {
			statusText = "Error"
		}
		return fmt.Sprintf("HTTP %d %s (Check that the Base URL and protocol are correct)", statusCode, statusText)
	}
	if len(raw) > 200 {
		raw = raw[:200] + "..."
	}
	if raw != "" {
		return raw
	}
	return fmt.Sprintf("HTTP %d %s", statusCode, http.StatusText(statusCode))
}

func fetchGeminiModels(ctx context.Context, client *http.Client, baseURL string, apiKey string) FetchModelsResponse {
	endpoint := baseURL
	if endpoint == "" {
		endpoint = "https://generativelanguage.googleapis.com"
	}
	if !strings.Contains(endpoint, "/models") {
		if strings.HasSuffix(endpoint, "/v1beta") {
			endpoint += "/models"
		} else {
			endpoint += "/v1beta/models"
		}
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return FetchModelsResponse{Success: false, Message: fmt.Sprintf("invalid endpoint url: %v", err)}
	}
	if apiKey != "" && u.Query().Get("key") == "" {
		q := u.Query()
		q.Set("key", apiKey)
		u.RawQuery = q.Encode()
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return FetchModelsResponse{Success: false, Message: err.Error()}
	}
	if apiKey != "" {
		httpReq.Header.Set("x-goog-api-key", apiKey)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return FetchModelsResponse{Success: false, Message: fmt.Sprintf("network error: %v", err)}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return FetchModelsResponse{
			Success: false,
			Message: fmt.Sprintf("API returned status %d: %s", resp.StatusCode, cleanErrorMessage(resp.StatusCode, body)),
		}
	}

	var parsed struct {
		Models []struct {
			Name                       string   `json:"name"`
			DisplayName                string   `json:"displayName"`
			Description                string   `json:"description"`
			InputTokenLimit            int      `json:"inputTokenLimit"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &parsed); err != nil {
		return FetchModelsResponse{Success: false, Message: fmt.Sprintf("failed to parse models json: %v", err)}
	}

	var results []ModelInfo
	for _, m := range parsed.Models {
		// Filter to generateContent capable models
		isGenerative := false
		if len(m.SupportedGenerationMethods) == 0 {
			isGenerative = true
		}
		for _, method := range m.SupportedGenerationMethods {
			if method == "generateContent" {
				isGenerative = true
				break
			}
		}
		if !isGenerative {
			continue
		}

		cleanID := strings.TrimPrefix(m.Name, "models/")
		ctxLimit, thinking, levels := detectModelMetadata(cleanID, m.DisplayName, m.InputTokenLimit)

		results = append(results, ModelInfo{
			ID:               cleanID,
			DisplayName:      cleanDisplayName(cleanID, m.DisplayName),
			ContextWindow:    ctxLimit,
			SupportsThinking: thinking,
			ThinkingLevels:   levels,
			Description:      m.Description,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})

	return FetchModelsResponse{
		Success: true,
		Models:  results,
		Message: fmt.Sprintf("Successfully fetched %d Gemini models", len(results)),
	}
}

func fetchAnthropicModels(ctx context.Context, client *http.Client, baseURL string, apiKey string) FetchModelsResponse {
	endpoint := baseURL
	if endpoint == "" {
		endpoint = "https://api.anthropic.com/v1/models"
	} else if !strings.Contains(endpoint, "/models") {
		if strings.HasSuffix(endpoint, "/v1") {
			endpoint += "/models"
		} else {
			endpoint += "/v1/models"
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return FetchModelsResponse{Success: false, Message: err.Error()}
	}
	if apiKey != "" {
		httpReq.Header.Set("x-api-key", apiKey)
	}
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := client.Do(httpReq)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var parsed struct {
			Data []struct {
				ID          string `json:"id"`
				DisplayName string `json:"display_name"`
			} `json:"data"`
		}
		if json.Unmarshal(body, &parsed) == nil && len(parsed.Data) > 0 {
			var results []ModelInfo
			for _, m := range parsed.Data {
				ctxLimit, thinking, levels := detectModelMetadata(m.ID, m.DisplayName, 0)
				results = append(results, ModelInfo{
					ID:               m.ID,
					DisplayName:      cleanDisplayName(m.ID, m.DisplayName),
					ContextWindow:    ctxLimit,
					SupportsThinking: thinking,
					ThinkingLevels:   levels,
				})
			}
			return FetchModelsResponse{
				Success: true,
				Models:  results,
				Message: fmt.Sprintf("Successfully fetched %d Anthropic models", len(results)),
			}
		}
	}

	// Fallback to standard Anthropic Claude models if live fetch fails or no API key provided
	defaults := []struct {
		id   string
		name string
	}{
		{"claude-3-7-sonnet-20250219", "Claude 3.7 Sonnet"},
		{"claude-3-5-sonnet-20241022", "Claude 3.5 Sonnet"},
		{"claude-3-5-haiku-20241022", "Claude 3.5 Haiku"},
		{"claude-3-opus-20240229", "Claude 3 Opus"},
	}

	var results []ModelInfo
	for _, d := range defaults {
		ctxLimit, thinking, levels := detectModelMetadata(d.id, d.name, 200000)
		results = append(results, ModelInfo{
			ID:               d.id,
			DisplayName:      d.name,
			ContextWindow:    ctxLimit,
			SupportsThinking: thinking,
			ThinkingLevels:   levels,
		})
	}

	return FetchModelsResponse{
		Success: true,
		Models:  results,
		Message: "Populated Anthropic models catalogue",
	}
}

func fetchOpenAIModels(ctx context.Context, client *http.Client, baseURL string, apiKey string, isCustom bool) FetchModelsResponse {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	cleanBase := strings.TrimRight(strings.TrimSpace(baseURL), "/")

	var candidateEndpoints []string
	if strings.HasSuffix(cleanBase, "/chat/completions") {
		candidateEndpoints = append(candidateEndpoints, strings.TrimSuffix(cleanBase, "/chat/completions")+"/models")
	} else if strings.HasSuffix(cleanBase, "/chat/completion") {
		candidateEndpoints = append(candidateEndpoints, strings.TrimSuffix(cleanBase, "/chat/completion")+"/models")
	} else if strings.HasSuffix(cleanBase, "/models") {
		candidateEndpoints = append(candidateEndpoints, cleanBase)
	} else if strings.HasSuffix(cleanBase, "/v1") {
		candidateEndpoints = append(candidateEndpoints, cleanBase+"/models")
	} else if strings.Contains(cleanBase, "/v1/") {
		candidateEndpoints = append(candidateEndpoints, strings.Split(cleanBase, "/v1/")[0]+"/v1/models")
	} else {
		// Prioritize standard /v1/models first, then fallback to /models
		candidateEndpoints = append(candidateEndpoints, cleanBase+"/v1/models", cleanBase+"/models")
	}

	var lastErrResp FetchModelsResponse
	var body []byte
	var successEndpoint string

	for _, endpoint := range candidateEndpoints {
		httpReq, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		if err != nil {
			lastErrResp = FetchModelsResponse{Success: false, Message: err.Error()}
			continue
		}
		if apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+apiKey)
			httpReq.Header.Set("x-api-key", apiKey)
		}
		httpReq.Header.Set("x-opencode-session", fmt.Sprintf("swiss-fetch-%d", time.Now().Unix()))

		resp, err := client.Do(httpReq)
		if err != nil {
			lastErrResp = FetchModelsResponse{
				Success: false,
				Message: fmt.Sprintf("Could not connect to %s: %v", endpoint, err),
			}
			continue
		}

		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			errMsg := cleanErrorMessage(resp.StatusCode, b)
			lastErrResp = FetchModelsResponse{
				Success: false,
				Message: fmt.Sprintf("Endpoint returned: %s", errMsg),
			}
			// If 404, try next candidate endpoint
			if resp.StatusCode == http.StatusNotFound && len(candidateEndpoints) > 1 {
				continue
			}
			return lastErrResp
		}

		body = b
		successEndpoint = endpoint
		break
	}

	if len(body) == 0 {
		if lastErrResp.Message != "" {
			return lastErrResp
		}
		return FetchModelsResponse{
			Success: false,
			Message: "No response received from model endpoint",
		}
	}
	_ = successEndpoint

	var results []ModelInfo

	// Try standard OpenAI format {"data": [...]}
	var openAIData struct {
		Data []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			ContextLength int    `json:"context_length"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &openAIData) == nil && len(openAIData.Data) > 0 {
		for _, m := range openAIData.Data {
			if m.ID == "" {
				continue
			}
			ctxLimit, thinking, levels := detectModelMetadata(m.ID, m.Name, m.ContextLength)
			results = append(results, ModelInfo{
				ID:               m.ID,
				DisplayName:      cleanDisplayName(m.ID, m.Name),
				ContextWindow:    ctxLimit,
				SupportsThinking: thinking,
				ThinkingLevels:   levels,
			})
		}
	}

	// Try Ollama format {"models": [...]}
	if len(results) == 0 {
		var ollamaData struct {
			Models []struct {
				Name  string `json:"name"`
				Model string `json:"model"`
			} `json:"models"`
		}
		if json.Unmarshal(body, &ollamaData) == nil && len(ollamaData.Models) > 0 {
			for _, m := range ollamaData.Models {
				id := m.Name
				if id == "" {
					id = m.Model
				}
				if id == "" {
					continue
				}
				ctxLimit, thinking, levels := detectModelMetadata(id, "", 0)
				results = append(results, ModelInfo{
					ID:               id,
					DisplayName:      cleanDisplayName(id, ""),
					ContextWindow:    ctxLimit,
					SupportsThinking: thinking,
					ThinkingLevels:   levels,
				})
			}
		}
	}

	// Try raw array format [{"id": ...}]
	if len(results) == 0 {
		var rawArray []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if json.Unmarshal(body, &rawArray) == nil && len(rawArray) > 0 {
			for _, m := range rawArray {
				id := m.ID
				if id == "" {
					id = m.Name
				}
				if id == "" {
					continue
				}
				ctxLimit, thinking, levels := detectModelMetadata(id, m.Name, 0)
				results = append(results, ModelInfo{
					ID:               id,
					DisplayName:      cleanDisplayName(id, m.Name),
					ContextWindow:    ctxLimit,
					SupportsThinking: thinking,
					ThinkingLevels:   levels,
				})
			}
		}
	}

	if len(results) == 0 {
		return FetchModelsResponse{
			Success: false,
			Message: "No compatible models found in response from " + successEndpoint,
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})

	return FetchModelsResponse{
		Success: true,
		Models:  results,
		Message: fmt.Sprintf("Successfully fetched %d models from %s", len(results), successEndpoint),
	}
}
