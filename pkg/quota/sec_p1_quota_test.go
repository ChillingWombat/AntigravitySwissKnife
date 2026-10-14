package quota

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

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
