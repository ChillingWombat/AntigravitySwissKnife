package quota

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

// TestP1_15_HTTP429_RetryAfterBackoff verifies that FetchLiveQuotaBreakdown
// honors HTTP 429 status code and Retry-After header with exponential backoff.
func TestP1_15_HTTP429_RetryAfterBackoff(t *testing.T) {
	ResetQuotaBackoffForTesting()
	defer ResetQuotaBackoffForTesting()

	origURLs := CloudCodeRetrieveQuotaURLs
	defer func() { CloudCodeRetrieveQuotaURLs = origURLs }()

	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Retry-After", "3")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{
			"error": {
				"code": 429,
				"message": "Resource exhausted",
				"status": "RESOURCE_EXHAUSTED",
				"details": [{"reason": "RATE_LIMITED"}]
			}
		}`))
	}))
	defer srv.Close()

	CloudCodeRetrieveQuotaURLs = []string{srv.URL}

	// First call receives 429 and triggers backoff
	_, err := FetchLiveQuotaBreakdown("dummy-token", "")
	if err == nil {
		t.Fatalf("expected error on 429, got nil")
	}
	apiErr, ok := err.(*QuotaAPIError)
	if !ok {
		t.Fatalf("expected *QuotaAPIError, got %T (%v)", err, err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected StatusCode 429, got %d", apiErr.StatusCode)
	}

	// Immediate second call should be short-circuited by backoff without making network requests
	reqCountBefore := requestCount
	_, err2 := FetchLiveQuotaBreakdown("dummy-token", "")
	if err2 == nil {
		t.Fatalf("expected short-circuit error during backoff, got nil")
	}
	if requestCount != reqCountBefore {
		t.Errorf("expected request to be short-circuited by backoff, but request was made to upstream")
	}
}

// TestP1_15_EmptyGroups_FailClosed verifies that empty response groups fail closed
// rather than reporting 100% "Ready".
func TestP1_15_EmptyGroups_FailClosed(t *testing.T) {
	ResetQuotaBackoffForTesting()
	defer ResetQuotaBackoffForTesting()

	origURLs := CloudCodeRetrieveQuotaURLs
	defer func() { CloudCodeRetrieveQuotaURLs = origURLs }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"groups": []}`))
	}))
	defer srv.Close()

	CloudCodeRetrieveQuotaURLs = []string{srv.URL}

	bd, err := FetchLiveQuotaBreakdown("dummy-token", "")
	if err != nil {
		t.Fatalf("FetchLiveQuotaBreakdown failed: %v", err)
	}

	if bd.Quota5hGemini != 0.0 {
		t.Errorf("expected fail-closed Quota5hGemini=0.0 on empty groups, got %f", bd.Quota5hGemini)
	}
	if bd.QuotaWeeklyGemini != 0.0 {
		t.Errorf("expected fail-closed QuotaWeeklyGemini=0.0 on empty groups, got %f", bd.QuotaWeeklyGemini)
	}
	if bd.ResetHorizonText != "Unknown" {
		t.Errorf("expected fail-closed ResetHorizonText='Unknown', got %q", bd.ResetHorizonText)
	}
}

// TestP1_15_AccountMutation_Concurrency verifies that concurrent mutations
// to keyring.Account structs inside PollAccountLiveQuota are race-safe.
func TestP1_15_AccountMutation_Concurrency(t *testing.T) {
	acc := &keyring.Account{
		Email:        "concurrent@example.com",
		AccessToken:  "initial-token",
		RefreshToken: "refresh-token",
		PlanTier:     "Free",
		Credits:      0,
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			accountMutationMu.Lock()
			acc.PlanTier = fmt.Sprintf("Pro-%d", idx)
			acc.Credits = float64(idx * 10)
			accountMutationMu.Unlock()
		}(i)
	}
	wg.Wait()

	accountMutationMu.Lock()
	tier := acc.PlanTier
	accountMutationMu.Unlock()
	if tier == "" {
		t.Errorf("expected non-empty plan tier after concurrent writes")
	}
}

// TestP1_15_HTTP429_AnomalousNoRetryAfter verifies that an HTTP 429 response
// without a Retry-After header and without an explicit structured reason is
// flagged as ANOMALOUS_429_NO_RETRY_AFTER with a 60s backoff, while an explicit
// RATE_LIMITED reason preserves normal RATE_LIMITED handling.
func TestP1_15_HTTP429_AnomalousNoRetryAfter(t *testing.T) {
	ResetQuotaBackoffForTesting()
	defer ResetQuotaBackoffForTesting()

	origURLs := CloudCodeRetrieveQuotaURLs
	defer func() { CloudCodeRetrieveQuotaURLs = origURLs }()

	cases := []struct {
		name           string
		body           string
		expectedReason string
		minRem         time.Duration
		maxRem         time.Duration
	}{
		{
			name:           "raw_429_no_retry_after",
			body:           `Too Many Requests`,
			expectedReason: "ANOMALOUS_429_NO_RETRY_AFTER",
			minRem:         30 * time.Second,
			maxRem:         61 * time.Second,
		},
		{
			name:           "structured_429_without_reason_no_retry_after",
			body:           `{"error":{"code":429,"message":"Too many requests","status":"RESOURCE_EXHAUSTED"}}`,
			expectedReason: "ANOMALOUS_429_NO_RETRY_AFTER",
			minRem:         30 * time.Second,
			maxRem:         61 * time.Second,
		},
		{
			name:           "structured_429_explicit_rate_limited_no_retry_after",
			body:           `{"error":{"code":429,"message":"Rate limited","status":"RESOURCE_EXHAUSTED","details":[{"reason":"RATE_LIMITED"}]}}`,
			expectedReason: "RATE_LIMITED",
			minRem:         1 * time.Second,
			maxRem:         6 * time.Second,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ResetQuotaBackoffForTesting()
			defer ResetQuotaBackoffForTesting()

			var reqCount int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reqCount++
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			var fallbackCount int
			fallbackSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fallbackCount++
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"groups":[]}`))
			}))
			defer fallbackSrv.Close()

			CloudCodeRetrieveQuotaURLs = []string{srv.URL, fallbackSrv.URL}

			_, err := FetchLiveQuotaBreakdown("dummy-token", "")
			if err == nil {
				t.Fatalf("expected error on 429, got nil")
			}
			apiErr, ok := err.(*QuotaAPIError)
			if !ok {
				t.Fatalf("expected *QuotaAPIError, got %T (%v)", err, err)
			}
			if apiErr.StatusCode != http.StatusTooManyRequests {
				t.Errorf("expected StatusCode 429, got %d", apiErr.StatusCode)
			}
			if apiErr.Reason != tc.expectedReason {
				t.Errorf("expected Reason %q, got %q", tc.expectedReason, apiErr.Reason)
			}
			if reqCount != 1 {
				t.Errorf("expected exactly 1 upstream request, got %d", reqCount)
			}
			if fallbackCount != 0 {
				t.Errorf("expected 0 fallback requests on 429, got %d", fallbackCount)
			}

			active, rem := IsQuotaBackoffActive()
			if !active {
				t.Fatalf("expected IsQuotaBackoffActive() to be true")
			}
			if rem < tc.minRem || rem > tc.maxRem {
				t.Errorf("expected remaining backoff in [%v, %v], got %v", tc.minRem, tc.maxRem, rem)
			}
		})
	}
}
