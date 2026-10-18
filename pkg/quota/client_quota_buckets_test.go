package quota

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Regression for the 2026-10-18 quota mismatch: the prod cloudcode-pa host
// served a stale gemini-weekly bucket (remainingFraction 0.0286) while the
// daily-cloudcode-pa host — the one the Antigravity language server itself
// uses — reported 1.0, matching the IDE's model picker. The daily host must
// stay first in the default endpoint list so its values win; prod is only a
// fallback.
func TestRetrieveQuotaEndpointOrder_PrefersDailyHost(t *testing.T) {
	if len(CloudCodeRetrieveQuotaURLs) < 2 {
		t.Fatalf("expected multiple retrieve-quota endpoints, got %v", CloudCodeRetrieveQuotaURLs)
	}
	if !strings.HasPrefix(CloudCodeRetrieveQuotaURLs[0], "https://daily-cloudcode-pa.googleapis.com/") {
		t.Errorf("first retrieve-quota endpoint must be daily-cloudcode-pa (IDE's own host), got %s", CloudCodeRetrieveQuotaURLs[0])
	}
	prodIdx := -1
	for i, u := range CloudCodeRetrieveQuotaURLs {
		if strings.HasPrefix(u, "https://cloudcode-pa.googleapis.com/") {
			prodIdx = i
		}
	}
	if prodIdx < 1 {
		t.Errorf("prod cloudcode-pa endpoint must remain in the list as a non-first fallback, idx=%d", prodIdx)
	}
}

// The client must serve values from the first reachable endpoint and must not
// let a later endpoint's (potentially stale) buckets overwrite them. Simulates
// daily (fresh, weekly=1.0) listed before prod (stale, weekly=0.0286).
func TestFetchLiveQuotaBreakdown_FirstEndpointWins(t *testing.T) {
	now := time.Now()
	freshPayload := `{
		"groups": [{
			"displayName": "Gemini Models",
			"buckets": [
				{"bucketId": "gemini-weekly", "displayName": "Weekly Limit Remaining", "window": "weekly", "remainingFraction": 1, "resetTime": "` + now.Add(168*time.Hour).Format(time.RFC3339) + `"},
				{"bucketId": "gemini-5h", "displayName": "Five Hour Limit Remaining", "window": "5h", "remainingFraction": 1, "resetTime": "` + now.Add(4*time.Hour).Format(time.RFC3339) + `"}
			]
		}]
	}`
	stalePayload := strings.Replace(freshPayload, `"remainingFraction": 1, "resetTime": "`+now.Add(168*time.Hour).Format(time.RFC3339)+`"`, `"remainingFraction": 0.028601466, "resetTime": "`+now.Add(72*time.Hour).Format(time.RFC3339)+`"`, 1)

	fresh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(freshPayload))
	}))
	defer fresh.Close()
	stale := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(stalePayload))
	}))
	defer stale.Close()

	origURLs := CloudCodeRetrieveQuotaURLs
	defer func() { CloudCodeRetrieveQuotaURLs = origURLs }()
	CloudCodeRetrieveQuotaURLs = []string{fresh.URL, stale.URL}

	bd, err := FetchLiveQuotaBreakdown("dummy_token", "")
	if err != nil {
		t.Fatalf("FetchLiveQuotaBreakdown failed: %v", err)
	}
	if bd.QuotaWeeklyGemini != 1 {
		t.Errorf("first-listed endpoint must win: expected Gemini weekly = 1.0, got %f (stale fallback leaked through)", bd.QuotaWeeklyGemini)
	}

	// Fallback still works when the first endpoint is unreachable.
	CloudCodeRetrieveQuotaURLs = []string{"http://127.0.0.1:1/unreachable", stale.URL}
	bd2, err := FetchLiveQuotaBreakdown("dummy_token", "")
	if err != nil {
		t.Fatalf("FetchLiveQuotaBreakdown with dead first endpoint failed: %v", err)
	}
	if bd2.QuotaWeeklyGemini < 0.02 || bd2.QuotaWeeklyGemini > 0.04 {
		t.Errorf("expected fallback endpoint's Gemini weekly ~0.0286, got %f", bd2.QuotaWeeklyGemini)
	}
}

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
