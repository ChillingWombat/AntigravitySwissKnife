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

	switch model.ProviderType {
	case ProviderAnthropic:
		// Test Anthropic Messages endpoint
		url := baseURL
		if !strings.HasSuffix(url, "/messages") {
			if !strings.HasSuffix(url, "/v1") && !strings.Contains(url, "/v1/") {
				url += "/v1"
			}
			url += "/messages"
		}
		payload := map[string]interface{}{
			"model":      model.Name,
			"max_tokens": 1,
			"messages": []map[string]string{
				{"role": "user", "content": "ping"},
			},
		}
		data, _ := json.Marshal(payload)
		req, err = http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("x-api-key", model.APIKey)
			req.Header.Set("anthropic-version", "2023-06-01")
		}

	case ProviderGemini:
		// Test Gemini generateContent endpoint
		url := baseURL
		if !strings.Contains(url, ":generateContent") {
			if !strings.HasSuffix(url, "/v1beta") && !strings.Contains(url, "/v1beta/") {
				url += "/v1beta"
			}
			url = fmt.Sprintf("%s/models/%s:generateContent", url, model.Name)
		}
		if model.APIKey != "" && !strings.Contains(url, "key=") {
			sep := "?"
			if strings.Contains(url, "?") {
				sep = "&"
			}
			url += sep + "key=" + model.APIKey
		}
		payload := map[string]interface{}{
			"contents": []map[string]interface{}{
				{"parts": []map[string]string{{"text": "ping"}}},
			},
		}
		data, _ := json.Marshal(payload)
		req, err = http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			if model.APIKey != "" {
				req.Header.Set("x-goog-api-key", model.APIKey)
			}
		}

	case ProviderLocal, ProviderOpenAI:
		fallthrough
	default:
		// Test OpenAI format: /chat/completions
		url := baseURL
		if !strings.HasSuffix(url, "/chat/completions") {
			if !strings.HasSuffix(url, "/v1") && !strings.Contains(url, "/v1/") {
				url += "/v1"
			}
			url += "/chat/completions"
		}
		payload := map[string]interface{}{
			"model":      model.Name,
			"max_tokens": 1,
			"messages": []map[string]string{
				{"role": "user", "content": "ping"},
			},
		}
		data, _ := json.Marshal(payload)
		req, err = http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
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
