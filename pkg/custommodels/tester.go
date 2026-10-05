package custommodels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// TestResult represents the outcome of an endpoint connectivity test.
type TestResult struct {
	Success    bool   `json:"success"`
	LatencyMs  int64  `json:"latency_ms"`
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Endpoint   string `json:"endpoint"`
}

// Tester tests connectivity and format compatibility for custom models.
type Tester struct {
	client *http.Client
}

// NewTester initializes a tester with a 5-second timeout.
func NewTester() *Tester {
	return &Tester{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// TestEndpoint validates communication with the target model provider endpoint.
func (t *Tester) TestEndpoint(model CustomModel) (*TestResult, error) {
	baseURL := strings.TrimRight(model.BaseURL, "/")
	if baseURL == "" {
		return &TestResult{
			Success: false,
			Message: "Base URL is empty",
		}, fmt.Errorf("empty base url")
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var req *http.Request
	var err error

	resolvedURL := ResolveEndpoint(model.ProviderType, baseURL, model.Name)

	switch model.ProviderType {
	case ProviderCustom:
		payload := map[string]interface{}{
			"model":      model.Name,
			"max_tokens": 1,
			"messages": []map[string]string{
				{"role": "user", "content": "ping"},
			},
		}
		data, _ := json.Marshal(payload)
		req, err = http.NewRequestWithContext(ctx, "POST", resolvedURL, bytes.NewReader(data))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			if model.APIKey != "" {
				req.Header.Set("Authorization", "Bearer "+model.APIKey)
				req.Header.Set("x-api-key", model.APIKey)
			}
		}

	case ProviderAnthropic:
		payload := map[string]interface{}{
			"model":      model.Name,
			"max_tokens": 1,
			"messages": []map[string]string{
				{"role": "user", "content": "ping"},
			},
		}
		data, _ := json.Marshal(payload)
		req, err = http.NewRequestWithContext(ctx, "POST", resolvedURL, bytes.NewReader(data))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("x-api-key", model.APIKey)
			req.Header.Set("anthropic-version", "2023-06-01")
		}

	case ProviderGemini:
		urlWithKey := resolvedURL
		if model.APIKey != "" && !strings.Contains(urlWithKey, "key=") {
			sep := "?"
			if strings.Contains(urlWithKey, "?") {
				sep = "&"
			}
			urlWithKey += sep + "key=" + model.APIKey
		}
		payload := map[string]interface{}{
			"contents": []map[string]interface{}{
				{"parts": []map[string]string{{"text": "ping"}}},
			},
		}
		data, _ := json.Marshal(payload)
		req, err = http.NewRequestWithContext(ctx, "POST", urlWithKey, bytes.NewReader(data))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			if model.APIKey != "" {
				req.Header.Set("x-goog-api-key", model.APIKey)
			}
		}

	case ProviderLocal, ProviderOpenAI:
		fallthrough
	default:
		payload := map[string]interface{}{
			"model":      model.Name,
			"max_tokens": 1,
			"messages": []map[string]string{
				{"role": "user", "content": "ping"},
			},
		}
		data, _ := json.Marshal(payload)
		req, err = http.NewRequestWithContext(ctx, "POST", resolvedURL, bytes.NewReader(data))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			if model.APIKey != "" {
				req.Header.Set("Authorization", "Bearer "+model.APIKey)
			}
		}
	}

	if err != nil {
		return &TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to construct request: %v", err),
		}, err
	}

	resp, err := t.client.Do(req)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		// If connection refused on local server, provide helpful hint
		if strings.Contains(err.Error(), "connection refused") {
			return &TestResult{
				Success:   false,
				LatencyMs: latency,
				Endpoint:  req.URL.String(),
				Message:   fmt.Sprintf("Connection refused at %s. Ensure local server (e.g. Ollama or vLLM) is running.", req.URL.Host),
			}, nil
		}
		return &TestResult{
			Success:   false,
			LatencyMs: latency,
			Endpoint:  req.URL.String(),
			Message:   fmt.Sprintf("Network error: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	res := &TestResult{
		StatusCode: resp.StatusCode,
		LatencyMs:  latency,
		Endpoint:   req.URL.String(),
	}

	// 200 OK or 400 with model message means endpoint is alive and responsive
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		res.Success = true
		res.Message = fmt.Sprintf("Success! Provider responded with %d OK in %dms.", resp.StatusCode, latency)
	} else if resp.StatusCode == 401 || resp.StatusCode == 403 {
		res.Success = false
		res.Message = fmt.Sprintf("Authentication failed (%d Unauthorized/Forbidden). Please verify your API key.", resp.StatusCode)
	} else if resp.StatusCode == 404 {
		res.Success = false
		res.Message = fmt.Sprintf("Endpoint returned 404 Not Found at %s. Check base URL or model name.", req.URL.Path)
	} else {
		res.Success = resp.StatusCode < 500
		res.Message = fmt.Sprintf("Endpoint reached (HTTP %d in %dms).", resp.StatusCode, latency)
	}

	return res, nil
}

// ResolveEndpoint returns the exact HTTP URL for the target protocol.
func ResolveEndpoint(providerType ProviderType, rawBaseURL string, modelName string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(rawBaseURL), "/")
	if baseURL == "" {
		return ""
	}

	switch providerType {
	case ProviderCustom:
		// Raw endpoint: directly use user-entered address without any auto-appending
		return strings.TrimSpace(rawBaseURL)

	case ProviderAnthropic:
		if strings.HasSuffix(baseURL, "/messages") {
			return baseURL
		}
		if strings.HasSuffix(baseURL, "/v1") {
			return baseURL + "/messages"
		}
		if strings.Contains(baseURL, "/v1/") {
			return baseURL + "/messages"
		}
		return baseURL + "/v1/messages"

	case ProviderGemini:
		if strings.Contains(baseURL, ":generateContent") {
			return baseURL
		}
		if strings.HasSuffix(baseURL, "/v1beta") || strings.Contains(baseURL, "/v1beta/") {
			return fmt.Sprintf("%s/models/%s:generateContent", baseURL, modelName)
		}
		return fmt.Sprintf("%s/v1beta/models/%s:generateContent", baseURL, modelName)

	case ProviderOpenAI, ProviderLocal:
		fallthrough
	default:
		// If user entered full completions endpoint directly (with or without trailing s):
		if strings.HasSuffix(baseURL, "/chat/completions") || strings.HasSuffix(baseURL, "/chat/completion") {
			return baseURL
		}
		// If base URL already ends with /v1
		if strings.HasSuffix(baseURL, "/v1") {
			return baseURL + "/chat/completions"
		}
		// If URL already contains version segment /v1/ or /v2/
		if strings.Contains(baseURL, "/v1/") || strings.Contains(baseURL, "/v2/") {
			return baseURL + "/chat/completions"
		}
		return baseURL + "/v1/chat/completions"
	}
}

