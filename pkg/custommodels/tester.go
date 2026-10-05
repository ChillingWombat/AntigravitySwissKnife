package custommodels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// TestResult represents the outcome of an endpoint connectivity test.
type TestResult struct {
	Success     bool         `json:"success"`
	LatencyMs   int64        `json:"latency_ms"`
	StatusCode  int          `json:"status_code"`
	Message     string       `json:"message"`
	Endpoint    string       `json:"endpoint"`
	QuotaResult *QuotaResult `json:"quota_result,omitempty"`
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
			"max_tokens": 16,
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
			req.Header.Set("x-opencode-session", fmt.Sprintf("swiss-test-%d", time.Now().Unix()))
		}

	case ProviderAnthropic:
		payload := map[string]interface{}{
			"model":      model.Name,
			"max_tokens": 16,
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
			"max_tokens": 16,
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
			req.Header.Set("x-opencode-session", fmt.Sprintf("swiss-test-%d", time.Now().Unix()))
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
	bodyBytes, _ := io.ReadAll(resp.Body)

	res := &TestResult{
		StatusCode: resp.StatusCode,
		LatencyMs:  latency,
		Endpoint:   req.URL.String(),
	}

	// Inspect rate limits or query provider balance/quota
	var quotaRes *QuotaResult
	if headerQuota := ParseRateLimitHeaders(resp.Header); headerQuota != nil {
		quotaRes = headerQuota
	} else {
		q := DetectAndFetchQuota(model)
		quotaRes = &q
	}
	res.QuotaResult = quotaRes

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		res.Success = true
		res.Message = fmt.Sprintf("Success! Provider responded with %d OK in %dms.", resp.StatusCode, latency)
		if quotaRes != nil && quotaRes.QuotaType == QuotaTypeBalance && quotaRes.BalanceValue != "" {
			res.Message += fmt.Sprintf(" • Balance: %s", quotaRes.BalanceValue)
		} else if quotaRes != nil && quotaRes.QuotaType == QuotaTypeQuota {
			if quotaRes.QuotaValue != "" {
				res.Message += fmt.Sprintf(" • Quota: %s", quotaRes.QuotaValue)
			} else {
				res.Message += " • Quota"
			}
		}
		return res, nil
	}

	res.Success = false

	// Extract error message from body if possible
	errorDetail := ""
	if len(bodyBytes) > 0 {
		var errObj struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
			Message string `json:"message"`
			Detail  string `json:"detail"`
		}
		if json.Unmarshal(bodyBytes, &errObj) == nil {
			if errObj.Error.Message != "" {
				errorDetail = errObj.Error.Message
			} else if errObj.Message != "" {
				errorDetail = errObj.Message
			} else if errObj.Detail != "" {
				errorDetail = errObj.Detail
			}
		} else {
			rawStr := strings.TrimSpace(string(bodyBytes))
			if !strings.HasPrefix(rawStr, "<") && len(rawStr) < 200 {
				errorDetail = rawStr
			}
		}
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		if errorDetail != "" {
			res.Message = fmt.Sprintf("Authentication failed (%d): %s", resp.StatusCode, errorDetail)
		} else {
			res.Message = fmt.Sprintf("Authentication failed (%d Unauthorized/Forbidden). Please verify your API key.", resp.StatusCode)
		}
	} else if resp.StatusCode == 404 {
		if errorDetail != "" {
			res.Message = fmt.Sprintf("Endpoint returned 404 Not Found: %s", errorDetail)
		} else {
			res.Message = fmt.Sprintf("Endpoint returned 404 Not Found at %s. Check base URL or model name.", req.URL.Path)
		}
	} else {
		if errorDetail != "" {
			res.Message = fmt.Sprintf("HTTP %d error: %s", resp.StatusCode, errorDetail)
		} else {
			statusText := http.StatusText(resp.StatusCode)
			if statusText == "" {
				statusText = "Error"
			}
			res.Message = fmt.Sprintf("Endpoint returned HTTP %d %s in %dms.", resp.StatusCode, statusText, latency)
		}
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
		trimmed := strings.TrimSpace(rawBaseURL)
		clean := strings.TrimRight(trimmed, "/")
		// If user entered a direct / full endpoint path:
		if strings.HasSuffix(clean, "/chat/completions") ||
			strings.HasSuffix(clean, "/chat/completion") ||
			strings.HasSuffix(clean, "/completions") ||
			strings.Contains(clean, "/chat") ||
			strings.Contains(clean, ":generateContent") ||
			strings.HasSuffix(clean, "/messages") {
			return trimmed
		}
		// If base URL contains version segment /v1 or /v2:
		if strings.HasSuffix(clean, "/v1") || strings.Contains(clean, "/v1/") || strings.Contains(clean, "/v2/") {
			return clean + "/chat/completions"
		}
		// Default custom endpoint convention: try /v1/chat/completions
		return clean + "/v1/chat/completions"

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

