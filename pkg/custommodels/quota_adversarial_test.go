package custommodels

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestAdversarial_DetectAndFetchQuota_CustomQuotaEndpoint_PrecedenceAndFallback
// tests that CustomQuotaEndpoint is probed first, and if it fails or returns empty,
// probing falls back to BaseURL probing, and if both fail, returns QuotaTypeNA.
func TestAdversarial_DetectAndFetchQuota_CustomQuotaEndpoint_PrecedenceAndFallback(t *testing.T) {
	var customEndpointHits int32
	var baseEndpointHits int32

	// Server that responds to BaseURL /v1/usage
	baseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&baseEndpointHits, 1)
		if r.URL.Path == "/v1/usage" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"total_credits": 200.0, "total_usage": 50.0}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer baseServer.Close()

	// 1. Scenario: Custom quota endpoint succeeds -> BaseURL should NOT be needed
	customServerSuccess := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&customEndpointHits, 1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"balance": 88.50}`))
	}))
	defer customServerSuccess.Close()

	m1 := CustomModel{
		BaseURL:             baseServer.URL + "/v1",
		APIKey:              "sk-key-1",
		CustomQuotaEndpoint: customServerSuccess.URL + "/custom-balance",
	}

	res1 := DetectAndFetchQuota(m1)
	if res1.QuotaType != QuotaTypeBalance || res1.BalanceValue != "$88.50" {
		t.Fatalf("expected custom endpoint to take precedence with $88.50, got %+v", res1)
	}
	if atomic.LoadInt32(&customEndpointHits) != 1 {
		t.Errorf("expected 1 hit on custom endpoint, got %d", atomic.LoadInt32(&customEndpointHits))
	}

	// 2. Scenario: Custom quota endpoint returns 500 error -> Fallback to BaseURL suffixes succeeds
	atomic.StoreInt32(&customEndpointHits, 0)
	atomic.StoreInt32(&baseEndpointHits, 0)

	customServerFail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&customEndpointHits, 1)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer customServerFail.Close()

	m2 := CustomModel{
		BaseURL:             baseServer.URL + "/v1",
		APIKey:              "sk-key-2",
		CustomQuotaEndpoint: customServerFail.URL + "/broken-endpoint",
	}

	res2 := DetectAndFetchQuota(m2)
	if res2.QuotaType != QuotaTypeBalance || res2.BalanceValue != "$150.00" {
		t.Fatalf("expected fallback to BaseURL with $150.00, got %+v", res2)
	}
	if atomic.LoadInt32(&customEndpointHits) != 1 {
		t.Errorf("expected 1 hit on failing custom endpoint, got %d", atomic.LoadInt32(&customEndpointHits))
	}
	if atomic.LoadInt32(&baseEndpointHits) == 0 {
		t.Errorf("expected base endpoint to be probed as fallback, got 0 hits")
	}

	// 3. Scenario: Both custom quota endpoint and BaseURL fail -> Graceful return of QuotaTypeNA
	deadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer deadServer.Close()

	m3 := CustomModel{
		BaseURL:             deadServer.URL + "/v1",
		APIKey:              "sk-key-3",
		CustomQuotaEndpoint: deadServer.URL + "/nonexistent",
	}

	res3 := DetectAndFetchQuota(m3)
	if res3.QuotaType != QuotaTypeNA {
		t.Fatalf("expected QuotaTypeNA when all endpoints fail, got %+v", res3)
	}
}

// TestAdversarial_DetectAndFetchQuota_SuffixProbing_Matrix
// tests each of the documented suffixes: /v1/usage, /usage, /api/user/usage,
// /dashboard/billing/subscription, /api/user/self across base URLs with and without /v1.
func TestAdversarial_DetectAndFetchQuota_SuffixProbing_Matrix(t *testing.T) {
	suffixes := []struct {
		suffix   string
		payload  string
		expected QuotaType
		checkVal string
	}{
		{
			suffix:   "/v1/usage",
			payload:  `{"total_credits": 100.0, "total_usage": 30.0}`,
			expected: QuotaTypeBalance,
			checkVal: "$70.00",
		},
		{
			suffix:   "/usage",
			payload:  `{"balance": 19.99}`,
			expected: QuotaTypeBalance,
			checkVal: "$19.99",
		},
		{
			suffix:   "/api/user/usage",
			payload:  `{"quota": 1000000}`,
			expected: QuotaTypeQuota,
			checkVal: "1.0M tokens",
		},
		{
			suffix:   "/dashboard/billing/subscription",
			payload:  `{"hard_limit_usd": 100.0, "total_usage": 2000}`,
			expected: QuotaTypeBalance,
			checkVal: "$80.00",
		},
		{
			suffix:   "/api/user/self",
			payload:  `{"success": true, "data": {"quota": 500000, "used_quota": 100000}}`,
			expected: QuotaTypeQuota,
			checkVal: "500k tokens",
		},
	}

	for _, sc := range suffixes {
		t.Run("Suffix_"+sc.suffix, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == sc.suffix {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(sc.payload))
					return
				}
				http.NotFound(w, r)
			}))
			defer server.Close()

			// Test with trailing /v1
			mWithV1 := CustomModel{
				BaseURL: server.URL + "/v1",
				APIKey:  "sk-test-v1",
			}
			res1 := DetectAndFetchQuota(mWithV1)
			if res1.QuotaType != sc.expected {
				t.Errorf("with /v1 base: expected QuotaType %v, got %v (%s)", sc.expected, res1.QuotaType, res1.Message)
			}
			if sc.expected == QuotaTypeBalance && res1.BalanceValue != sc.checkVal {
				t.Errorf("with /v1 base: expected balance %s, got %s", sc.checkVal, res1.BalanceValue)
			}
			if sc.expected == QuotaTypeQuota && res1.QuotaValue != sc.checkVal {
				t.Errorf("with /v1 base: expected quota %s, got %s", sc.checkVal, res1.QuotaValue)
			}

			// Test without trailing /v1
			mWithoutV1 := CustomModel{
				BaseURL: server.URL,
				APIKey:  "sk-test-direct",
			}
			res2 := DetectAndFetchQuota(mWithoutV1)
			if res2.QuotaType != sc.expected {
				t.Errorf("without /v1 base: expected QuotaType %v, got %v (%s)", sc.expected, res2.QuotaType, res2.Message)
			}
		})
	}
}

// TestAdversarial_DetectAndFetchQuota_OpenCode_FullSpec
// tests that opencode.ai provider is recognized in the registry, sends the x-opencode-session header,
// and parses various OpenCode JSON payload formats (balance, total_credits - total_usage, tokens).
func TestAdversarial_DetectAndFetchQuota_OpenCode_FullSpec(t *testing.T) {
	var receivedAuthHeader string
	var receivedSessionHeader string
	var receivedAPIKeyHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")
		receivedSessionHeader = r.Header.Get("x-opencode-session")
		receivedAPIKeyHeader = r.Header.Get("x-api-key")

		if r.URL.Path == "/api/user/usage" || r.URL.Path == "/v1/usage" || r.URL.Path == "/usage" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"data": {
					"total_credits": 150.0,
					"total_usage": 37.5,
					"quota": 2000000
				}
			}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	// Locate OpenCode entry in ProviderQuotaRegistry
	var ocEntry *ProviderQuotaRegistryEntry
	for i := range ProviderQuotaRegistry {
		if ProviderQuotaRegistry[i].Name == "OpenCode" {
			ocEntry = &ProviderQuotaRegistry[i]
			break
		}
	}
	if ocEntry == nil {
		t.Fatalf("OpenCode entry missing from ProviderQuotaRegistry")
	}

	// Verify HostPatterns contains opencode.ai
	foundPattern := false
	for _, p := range ocEntry.HostPatterns {
		if p == "opencode.ai" {
			foundPattern = true
			break
		}
	}
	if !foundPattern {
		t.Errorf("expected opencode.ai in OpenCode HostPatterns, got %v", ocEntry.HostPatterns)
	}

	// Execute FetchFunc against our mock server
	ctx := context.Background()
	client := server.Client()
	res, err := ocEntry.FetchFunc(ctx, client, server.URL+"/api/user/usage", "opencode-test-key-xyz")
	if err != nil {
		t.Fatalf("OpenCode FetchFunc returned unexpected error: %v", err)
	}

	if res.QuotaType != QuotaTypeBalance {
		t.Errorf("expected QuotaTypeBalance, got %v", res.QuotaType)
	}
	// total_credits (150.0) - total_usage (37.5) = 112.50
	if res.BalanceValue != "$112.50" {
		t.Errorf("expected balance $112.50, got %s", res.BalanceValue)
	}
	if res.Fraction == nil || *res.Fraction != 0.75 {
		t.Errorf("expected fraction 0.75, got %v", res.Fraction)
	}

	// Verify headers transmitted
	if receivedAuthHeader != "Bearer opencode-test-key-xyz" {
		t.Errorf("expected Authorization: Bearer opencode-test-key-xyz, got %q", receivedAuthHeader)
	}
	if receivedAPIKeyHeader != "opencode-test-key-xyz" {
		t.Errorf("expected x-api-key: opencode-test-key-xyz, got %q", receivedAPIKeyHeader)
	}
	if !strings.HasPrefix(receivedSessionHeader, "swiss-quota-") {
		t.Errorf("expected x-opencode-session starting with swiss-quota-, got %q", receivedSessionHeader)
	}
}

// TestAdversarial_DetectAndFetchQuota_TimeoutAndEmptyPayloads
// tests fallback behavior under network timeouts, empty responses, and non-JSON garbage.
func TestAdversarial_DetectAndFetchQuota_TimeoutAndEmptyPayloads(t *testing.T) {
	// 1. Slow server simulating timeout (> 5s context deadline)
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(6 * time.Second)
		w.Write([]byte(`{"balance": 100}`))
	}))
	defer slowServer.Close()

	mSlow := CustomModel{
		BaseURL: slowServer.URL,
		APIKey:  "sk-timeout",
	}

	start := time.Now()
	resSlow := DetectAndFetchQuota(mSlow)
	dur := time.Since(start)

	if resSlow.QuotaType != QuotaTypeNA {
		t.Errorf("expected QuotaTypeNA on timeout, got %v", resSlow.QuotaType)
	}
	if dur > 6500*time.Millisecond {
		t.Errorf("expected DetectAndFetchQuota to abort within ~5s timeout, took %v", dur)
	}

	// 2. Empty 200 response body
	emptyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}))
	defer emptyServer.Close()

	mEmpty := CustomModel{
		BaseURL: emptyServer.URL,
		APIKey:  "sk-empty",
	}
	resEmpty := DetectAndFetchQuota(mEmpty)
	if resEmpty.QuotaType != QuotaTypeNA {
		t.Errorf("expected QuotaTypeNA on empty 200 response, got %v", resEmpty.QuotaType)
	}

	// 3. HTML/Text garbage response (e.g. Cloudflare error page)
	garbageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html><body>Error 502 Bad Gateway</body></html>`))
	}))
	defer garbageServer.Close()

	mGarbage := CustomModel{
		BaseURL: garbageServer.URL,
		APIKey:  "sk-garbage",
	}
	resGarbage := DetectAndFetchQuota(mGarbage)
	if resGarbage.QuotaType != QuotaTypeNA {
		t.Errorf("expected QuotaTypeNA on HTML/Garbage response, got %v", resGarbage.QuotaType)
	}

	// 4. Dead / unroutable IP (TCP connect failure)
	mDead := CustomModel{
		BaseURL: "http://127.0.0.1:49999/v1",
		APIKey:  "sk-dead",
	}
	resDead := DetectAndFetchQuota(mDead)
	if resDead.QuotaType != QuotaTypeNA {
		t.Errorf("expected QuotaTypeNA on dead TCP port, got %v", resDead.QuotaType)
	}
}
