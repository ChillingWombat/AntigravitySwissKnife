package custommodels

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseRateLimitHeaders(t *testing.T) {
	// 1. Token remaining and limit
	h1 := make(http.Header)
	h1.Set("x-ratelimit-remaining-tokens", "250000")
	h1.Set("x-ratelimit-limit-tokens", "1000000")

	res1 := ParseRateLimitHeaders(h1)
	if res1 == nil {
		t.Fatalf("expected quota result from rate limit headers, got nil")
	}
	if res1.QuotaType != QuotaTypeQuota {
		t.Errorf("expected QuotaTypeQuota, got %s", res1.QuotaType)
	}
	if res1.QuotaValue != "250k tokens" && res1.QuotaValue != "250,000 tokens" {
		t.Errorf("unexpected QuotaValue: %s", res1.QuotaValue)
	}
	if res1.Fraction == nil || *res1.Fraction != 0.25 {
		t.Errorf("expected fraction 0.25, got %v", res1.Fraction)
	}

	// 2. Request limit percentage only
	h2 := make(http.Header)
	h2.Set("x-ratelimit-remaining-requests", "80")
	h2.Set("x-ratelimit-limit-requests", "100")

	res2 := ParseRateLimitHeaders(h2)
	if res2 == nil {
		t.Fatalf("expected quota result, got nil")
	}
	if res2.QuotaType != QuotaTypeQuota {
		t.Errorf("expected QuotaTypeQuota, got %s", res2.QuotaType)
	}
	if res2.QuotaValue != "" {
		t.Errorf("expected empty QuotaValue when only percentage is available, got %s", res2.QuotaValue)
	}
	if res2.Fraction == nil || *res2.Fraction != 0.8 {
		t.Errorf("expected fraction 0.8, got %v", res2.Fraction)
	}

	// 3. No rate limit headers
	h3 := make(http.Header)
	res3 := ParseRateLimitHeaders(h3)
	if res3 != nil {
		t.Errorf("expected nil for empty headers, got %v", res3)
	}
}

func TestDetectAndFetchQuota_UnrecognizedURL(t *testing.T) {
	m := CustomModel{
		BaseURL: "https://unknown-private-llm.corp.internal/v1",
		APIKey:  "sk-test",
	}
	res := DetectAndFetchQuota(m)
	if res.QuotaType != QuotaTypeNA {
		t.Errorf("expected QuotaTypeNA for unrecognized URL, got %s", res.QuotaType)
	}
	if res.BalanceValue != "" {
		t.Errorf("expected empty balance value, got %s", res.BalanceValue)
	}
	if res.QuotaValue != "" {
		t.Errorf("expected empty quota value, got %s", res.QuotaValue)
	}
}

func TestDetectAndFetchQuota_ProxyEndpoints(t *testing.T) {
	// Mock a OneAPI server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dashboard/billing/subscription" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"hard_limit_usd": 50.0, "total_usage": 1500}`)) // 50 - 15 = 35 USD
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	m := CustomModel{
		BaseURL: server.URL + "/v1",
		APIKey:  "sk-mock-proxy-key",
	}

	res := DetectAndFetchQuota(m)
	if res.QuotaType != QuotaTypeBalance {
		t.Errorf("expected QuotaTypeBalance, got %s", res.QuotaType)
	}
	if res.BalanceValue != "$35.00" {
		t.Errorf("expected $35.00 balance, got %s", res.BalanceValue)
	}
	if res.Fraction == nil || *res.Fraction != 0.7 {
		t.Errorf("expected fraction 0.7, got %v", res.Fraction)
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		in  int64
		exp string
	}{
		{500, "500"},
		{10000, "10k"},
		{250000, "250k"},
		{1000000, "1.0M"},
		{2500000, "2.5M"},
		{1234567, "1,234,567"},
	}

	for _, tt := range tests {
		got := formatTokens(tt.in)
		if got != tt.exp {
			t.Errorf("formatTokens(%d) = %s, expected %s", tt.in, got, tt.exp)
		}
	}
}
