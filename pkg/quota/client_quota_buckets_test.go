package quota

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Reproduces the live retrieveUserQuotaSummary payload shape: buckets carry
// generic display names ("Weekly Limit Remaining") and the exhausted Claude
// weekly bucket reports remainingFraction 0. The weekly value must stay 0 —
// it must not inherit the 5h value from the presence heuristic.
func TestFetchLiveQuotaBreakdown_ExhaustedClaudeWeeklyStaysZero(t *testing.T) {
	now := time.Now()
	payload := `{
		"subscription_tier": "pro",
		"is_trial": true,
		"groups": [
			{
				"displayName": "Gemini Models",
				"description": "Models within this group: Gemini Flash, Gemini Pro",
				"buckets": [
					{"bucketId": "gemini-weekly", "displayName": "Weekly Limit Remaining", "window": "weekly", "remainingFraction": 0.028601466, "resetTime": "` + now.Add(72*time.Hour).Format(time.RFC3339) + `"},
					{"bucketId": "gemini-5h", "displayName": "Five Hour Limit Remaining", "window": "5h", "remainingFraction": 1, "resetTime": "` + now.Add(4*time.Hour).Format(time.RFC3339) + `"}
				]
			},
			{
				"displayName": "Claude and GPT models",
				"description": "Models within this group: Claude Opus, Claude Sonnet, GPT-OSS",
				"buckets": [
					{"bucketId": "3p-weekly", "displayName": "Weekly Limit Remaining", "window": "weekly", "remainingFraction": 0, "resetTime": "` + now.Add(72*time.Hour).Format(time.RFC3339) + `"},
					{"bucketId": "3p-5h", "displayName": "Five Hour Limit Remaining", "window": "5h", "disabled": true, "remainingFraction": 1, "resetTime": "` + now.Add(4*time.Hour).Format(time.RFC3339) + `"}
				]
			}
		]
	}`

	origURLs := CloudCodeRetrieveQuotaURLs
	defer func() { CloudCodeRetrieveQuotaURLs = origURLs }()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	CloudCodeRetrieveQuotaURLs = []string{srv.URL}

	bd, err := FetchLiveQuotaBreakdown("dummy_token", "")
	if err != nil {
		t.Fatalf("FetchLiveQuotaBreakdown failed: %v", err)
	}

	if bd.QuotaWeeklyClaudeGPT != 0 {
		t.Errorf("exhausted Claude weekly must stay 0, got %f", bd.QuotaWeeklyClaudeGPT)
	}
	if bd.Quota5hClaudeGPT != 1 {
		t.Errorf("expected Claude 5h = 1, got %f", bd.Quota5hClaudeGPT)
	}
	if bd.QuotaWeeklyGemini < 0.02 || bd.QuotaWeeklyGemini > 0.04 {
		t.Errorf("expected Gemini weekly ~0.0286, got %f", bd.QuotaWeeklyGemini)
	}
	if bd.Quota5hGemini != 1 {
		t.Errorf("expected Gemini 5h = 1, got %f", bd.Quota5hGemini)
	}
	if !bd.HasClaudeGPTRights {
		t.Errorf("expected HasClaudeGPTRights=true for the 3p group")
	}

	// Without the weekly bucket at all (tier where weekly does not apply), the
	// 5h value still mirrors onto the weekly slot.
	payloadNoWeekly := `{
		"subscription_tier": "pro",
		"groups": [
			{
				"displayName": "Claude and GPT models",
				"buckets": [
					{"bucketId": "3p-5h", "displayName": "Five Hour Limit Remaining", "window": "5h", "remainingFraction": 0.42, "resetTime": "` + now.Add(4*time.Hour).Format(time.RFC3339) + `"}
				]
			}
		]
	}`
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payloadNoWeekly))
	}))
	CloudCodeRetrieveQuotaURLs = []string{srv2.URL}

	bd2, err := FetchLiveQuotaBreakdown("dummy_token", "")
	if err != nil {
		t.Fatalf("FetchLiveQuotaBreakdown (no weekly) failed: %v", err)
	}
	if bd2.Quota5hClaudeGPT != 0.42 {
		t.Errorf("expected Claude 5h = 0.42, got %f", bd2.Quota5hClaudeGPT)
	}
	if bd2.QuotaWeeklyClaudeGPT != 0.42 {
		t.Errorf("expected weekly to mirror 5h when the weekly bucket is absent, got %f", bd2.QuotaWeeklyClaudeGPT)
	}
}
