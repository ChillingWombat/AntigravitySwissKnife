package bridge

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

// HTTPApprovalRequest is one host an agent asked to reach that is not on the
// outbound allowlist yet. The request itself is never sent: it waits for the
// user to approve or deny the host in the desktop app.
type HTTPApprovalRequest struct {
	Host        string `json:"host"`
	Method      string `json:"method,omitempty"`
	URL         string `json:"url,omitempty"`
	Reason      string `json:"reason,omitempty"`
	RequestedAt string `json:"requested_at"`
}

const (
	httpResponseLimit  = 200 * 1024
	httpDefaultTimeout = 30
	httpMaxTimeout     = 120
)

// normalizeAllowHost trims and lowercases one allowlist entry. A leading "*."
// marks a suffix match ("*.googleapis.com").
func normalizeAllowHost(raw string) string {
	host := strings.ToLower(strings.TrimSpace(raw))
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	if idx := strings.IndexAny(host, "/?#"); idx >= 0 {
		host = host[:idx]
	}
	return strings.TrimSuffix(host, ".")
}

// hostAllowed reports whether host matches the allowlist. Entries match exactly,
// or as a "*.suffix" wildcard.
func hostAllowed(host string, allowlist []string) bool {
	target := normalizeAllowHost(host)
	if target == "" {
		return false
	}
	for _, entry := range allowlist {
		allow := normalizeAllowHost(entry)
		if allow == "" {
			continue
		}
		if allow == target {
			return true
		}
		if strings.HasPrefix(allow, "*.") {
			suffix := strings.TrimPrefix(allow, "*")
			if strings.HasSuffix(target, suffix) && len(target) > len(suffix) {
				return true
			}
		}
	}
	return false
}

// AllowlistHosts returns the current outbound HTTP allowlist, sorted.
func (s *Service) AllowlistHosts() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]string{}, s.config.HTTPAllowlist...)
	sort.Strings(out)
	return out
}

// PendingHTTPApprovals returns the hosts an agent asked for that the user has
// not approved or denied yet.
func (s *Service) PendingHTTPApprovals() []HTTPApprovalRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]HTTPApprovalRequest{}, s.config.HTTPPendingApprovals...)
}

// AllowHTTPHost approves one host and clears it from the pending queue.
func (s *Service) AllowHTTPHost(rawHost string) (string, error) {
	host := normalizeAllowHost(rawHost)
	if host == "" {
		return "", fmt.Errorf("host is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	for _, existing := range s.config.HTTPAllowlist {
		if normalizeAllowHost(existing) == host {
			found = true
			break
		}
	}
	if !found {
		s.config.HTTPAllowlist = append(s.config.HTTPAllowlist, host)
	}
	s.config.HTTPPendingApprovals = dropPendingHost(s.config.HTTPPendingApprovals, host)
	if err := s.saveConfig(); err != nil {
		return "", err
	}
	return host, nil
}

// RevokeHTTPHost removes one host from the allowlist and from the pending queue.
func (s *Service) RevokeHTTPHost(rawHost string) error {
	host := normalizeAllowHost(rawHost)
	if host == "" {
		return fmt.Errorf("host is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := make([]string, 0, len(s.config.HTTPAllowlist))
	for _, existing := range s.config.HTTPAllowlist {
		if normalizeAllowHost(existing) != host {
			kept = append(kept, existing)
		}
	}
	s.config.HTTPAllowlist = kept
	s.config.HTTPPendingApprovals = dropPendingHost(s.config.HTTPPendingApprovals, host)
	return s.saveConfig()
}

// DenyHTTPHost drops a pending request without granting access.
func (s *Service) DenyHTTPHost(rawHost string) error {
	host := normalizeAllowHost(rawHost)
	if host == "" {
		return fmt.Errorf("host is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.HTTPPendingApprovals = dropPendingHost(s.config.HTTPPendingApprovals, host)
	return s.saveConfig()
}

func dropPendingHost(pending []HTTPApprovalRequest, host string) []HTTPApprovalRequest {
	kept := make([]HTTPApprovalRequest, 0, len(pending))
	for _, entry := range pending {
		if normalizeAllowHost(entry.Host) != host {
			kept = append(kept, entry)
		}
	}
	return kept
}

// queueHTTPApproval records a pending approval once per host.
func (s *Service) queueHTTPApproval(host, method, rawURL, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, entry := range s.config.HTTPPendingApprovals {
		if normalizeAllowHost(entry.Host) == host {
			return
		}
	}
	s.config.HTTPPendingApprovals = append(s.config.HTTPPendingApprovals, HTTPApprovalRequest{
		Host:        host,
		Method:      strings.ToUpper(method),
		URL:         rawURL,
		Reason:      reason,
		RequestedAt: time.Now().Format("2006-01-02 15:04:05"),
	})
	if len(s.config.HTTPPendingApprovals) > 50 {
		s.config.HTTPPendingApprovals = s.config.HTTPPendingApprovals[:50]
	}
	_ = s.saveConfig()
}

// HTTPRequest performs one outbound HTTP call for an allowlisted host.
func (s *Service) HTTPRequest(args map[string]interface{}) (string, error) {
	rawURL, _ := args["url"].(string)
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("url is required")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("invalid url %q", rawURL)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme %q: only http and https are allowed", parsed.Scheme)
	}

	host := normalizeAllowHost(parsed.Host)
	if !hostAllowed(host, s.AllowlistHosts()) {
		reason, _ := args["reason"].(string)
		s.queueHTTPApproval(host, argString(args, "method", "GET"), rawURL, reason)
		payload := map[string]interface{}{
			"status":           "approval_required",
			"host":             host,
			"request_sent":     false,
			"pending_approval": true,
			"hint": "Ask the user to approve this host in Antigravity Swiss Knife > Gemini Bridge > Outbound HTTP, " +
				"or call local_http_allowlist with {\"action\":\"allow\",\"host\":\"" + host + "\"} once they agree.",
		}
		data, _ := json.MarshalIndent(payload, "", "  ")
		return string(data), nil
	}

	method := strings.ToUpper(argString(args, "method", "GET"))
	timeout := httpDefaultTimeout
	if ts, ok := args["timeout_seconds"].(float64); ok && int(ts) > 0 {
		timeout = int(ts)
		if timeout > httpMaxTimeout {
			timeout = httpMaxTimeout
		}
	}

	var bodyReader io.Reader
	body := argString(args, "body", "")
	contentType := ""
	if headers, ok := args["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			if strings.EqualFold(k, "content-type") {
				contentType = fmt.Sprint(v)
			}
		}
	}
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	if headers, ok := args["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			req.Header.Set(k, fmt.Sprint(v))
		}
	}
	if body != "" && contentType == "" {
		if strings.HasPrefix(strings.TrimSpace(body), "{") || strings.HasPrefix(strings.TrimSpace(body), "[") {
			req.Header.Set("Content-Type", "application/json")
		}
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Antigravity-Swiss-Knife-Bridge/1.0")
	}

	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("%s %s timed out after %ds", method, rawURL, timeout)
		}
		return "", fmt.Errorf("%s %s failed: %w", method, rawURL, err)
	}
	defer resp.Body.Close()

	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, httpResponseLimit+1))
	truncated := false
	if int64(len(raw)) > httpResponseLimit {
		raw = raw[:httpResponseLimit]
		truncated = true
	}
	if readErr != nil && len(raw) == 0 {
		return "", fmt.Errorf("read response: %w", readErr)
	}

	respHeaders := map[string]string{}
	for k := range resp.Header {
		respHeaders[k] = resp.Header.Get(k)
	}
	result := map[string]interface{}{
		"status":       "ok",
		"request_sent": true,
		"method":       method,
		"url":          rawURL,
		"http_status":  resp.StatusCode,
		"duration_ms":  time.Since(start).Milliseconds(),
		"headers":      respHeaders,
		"body":         string(raw),
		"truncated":    truncated,
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return string(data), nil
}

// HTTPAllowlistCommand implements the local_http_allowlist tool.
func (s *Service) HTTPAllowlistCommand(args map[string]interface{}) (string, error) {
	action := strings.ToLower(argString(args, "action", "list"))
	host := argString(args, "host", "")
	switch action {
	case "", "list":
		payload := map[string]interface{}{
			"status":    "ok",
			"allowlist": s.AllowlistHosts(),
			"pending":   s.PendingHTTPApprovals(),
		}
		data, _ := json.MarshalIndent(payload, "", "  ")
		return string(data), nil
	case "allow", "approve", "add":
		approved, err := s.AllowHTTPHost(host)
		if err != nil {
			return "", err
		}
		payload := map[string]interface{}{
			"status":    "allowed",
			"host":      approved,
			"allowlist": s.AllowlistHosts(),
		}
		data, _ := json.MarshalIndent(payload, "", "  ")
		return string(data), nil
	case "revoke", "remove", "deny":
		if action == "deny" {
			if err := s.DenyHTTPHost(host); err != nil {
				return "", err
			}
		} else if err := s.RevokeHTTPHost(host); err != nil {
			return "", err
		}
		payload := map[string]interface{}{
			"status":    "revoked",
			"host":      normalizeAllowHost(host),
			"allowlist": s.AllowlistHosts(),
			"pending":   s.PendingHTTPApprovals(),
		}
		data, _ := json.MarshalIndent(payload, "", "  ")
		return string(data), nil
	default:
		return "", fmt.Errorf("unsupported action %q: use list, allow, revoke or deny", action)
	}
}

func argString(args map[string]interface{}, key, fallback string) string {
	if args == nil {
		return fallback
	}
	if v, ok := args[key].(string); ok {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return fallback
}
