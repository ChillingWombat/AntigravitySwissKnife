package custommodels

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func almostEqual(a, b float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < 1e-9
}

// bustPricingCaches clears the module-level price caches so a test observes the
// freshly mocked catalogs instead of a cached answer from another test.
func bustPricingCaches() {
	googlePricingCacheMu.Lock()
	googlePricingCache = nil
	googlePricingCacheAt = time.Time{}
	googlePricingCacheMu.Unlock()

	thirdPartyCacheMu.Lock()
	thirdPartyCache = nil
	thirdPartyCacheAt = time.Time{}
	thirdPartyCacheMu.Unlock()
}

// The bundled official rate table is a last-resort fallback: a live catalog that
// already carries the current rate must win, otherwise a hard-coded entry that
// happens to match pins a stale price forever (gemini-4-argon kept the retired
// Flash-tier rate while OpenRouter/LiteLLM already served the frontier rate).
func TestFetchModelPricing_PrefersLiveCatalogOverBundledTable(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [{
				"id": "google/gemini-4-argon",
				"pricing": {
					"prompt": "0.000002",
					"completion": "0.00001",
					"input_cache_read": "0.0000001"
				}
			}]
		}`))
	}))
	defer mock.Close()

	origOpenRouter := OpenRouterModelsURL
	origLiteLLM := LiteLLMPricingURL
	origGoogle := GoogleGeminiPricingURL
	origDocs := GoogleGeminiDocsPricingURL
	defer func() {
		OpenRouterModelsURL = origOpenRouter
		LiteLLMPricingURL = origLiteLLM
		GoogleGeminiPricingURL = origGoogle
		GoogleGeminiDocsPricingURL = origDocs
		bustPricingCaches()
	}()

	OpenRouterModelsURL = mock.URL
	LiteLLMPricingURL = "" // isolate: OpenRouter only
	GoogleGeminiPricingURL = ""
	GoogleGeminiDocsPricingURL = ""
	bustPricingCaches()

	in, cached, out, src := FetchModelPricing(context.Background(), "gemini-4-argon", "gcp", "", "")
	if src != PriceSourceThirdParty {
		t.Fatalf("expected live third-party source to win, got %q (in=%v out=%v)", src, in, out)
	}
	if in == nil || out == nil || cached == nil {
		t.Fatalf("expected prices, got in=%v cached=%v out=%v", in, cached, out)
	}
	if !almostEqual(*in, 2.00) || !almostEqual(*cached, 0.10) || !almostEqual(*out, 10.00) {
		t.Errorf("expected live catalog rate 2.00/0.10/10.00, got %v/%v/%v", *in, *cached, *out)
	}
}

// When no live source has the model, the bundled table is still used (offline fallback).
func TestFetchModelPricing_FallsBackToBundledTable(t *testing.T) {
	origOpenRouter := OpenRouterModelsURL
	origLiteLLM := LiteLLMPricingURL
	origGoogle := GoogleGeminiPricingURL
	origDocs := GoogleGeminiDocsPricingURL
	defer func() {
		OpenRouterModelsURL = origOpenRouter
		LiteLLMPricingURL = origLiteLLM
		GoogleGeminiPricingURL = origGoogle
		GoogleGeminiDocsPricingURL = origDocs
		bustPricingCaches()
	}()

	// Empty catalogs: every live source returns nothing.
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data": []}`))
	}))
	defer empty.Close()
	OpenRouterModelsURL = empty.URL
	LiteLLMPricingURL = ""
	GoogleGeminiPricingURL = ""
	GoogleGeminiDocsPricingURL = ""
	bustPricingCaches()

	in, _, out, src := FetchModelPricing(context.Background(), "gemini-4-argon", "gcp", "", "")
	if src != PriceSourceProvider {
		t.Fatalf("expected bundled fallback source, got %q", src)
	}
	if in == nil || out == nil || *in != 2.00 || *out != 10.00 {
		t.Errorf("expected bundled rate 2.00/10.00, got in=%v out=%v", in, out)
	}
}

// Provider-sourced prices must expire so rate changes propagate automatically.
func TestProviderPriceStale(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name    string
		updated string
		want    bool
	}{
		{"empty timestamp is stale", "", true},
		{"unparsable timestamp is stale", "not-a-date", true},
		{"fresh timestamp is trusted", now.Format("2006-01-02 15:04"), false},
		{"one day old is trusted", now.Add(-24 * time.Hour).Format("2006-01-02 15:04"), false},
		{"eight days old is stale", now.Add(-8 * 24 * time.Hour).Format("2006-01-02 15:04"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := providerPriceStale(tc.updated, providerPriceMaxAge); got != tc.want {
				t.Errorf("providerPriceStale(%q) = %v, want %v", tc.updated, got, tc.want)
			}
		})
	}
}
