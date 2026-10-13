package quota

import (
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestChallenger_ClampFraction_Stress tests ClampFraction across extreme numeric boundaries,
// NaN, infinities, subnormals, and precision edges.
func TestChallenger_ClampFraction_Stress(t *testing.T) {
	testCases := []struct {
		name     string
		input    float64
		expected float64
	}{
		{"NaN", math.NaN(), 0.0},
		{"+Inf", math.Inf(1), 0.0},
		{"-Inf", math.Inf(-1), 0.0},
		{"LargeNegative", -1e300, 0.0},
		{"NegativeTen", -10.0, 0.0},
		{"NegativeOne", -1.0, 0.0},
		{"SmallNegative", -1e-15, 0.0},
		{"NegativeZero", math.Copysign(0.0, -1.0), 0.0},
		{"ExactZero", 0.0, 0.0},
		{"SmallPositive", 1e-15, 1e-15},
		{"Quarter", 0.25, 0.25},
		{"Half", 0.5, 0.5},
		{"NearOne", 0.999999999999999, 0.999999999999999},
		{"ExactOne", 1.0, 1.0},
		{"SlightlyAboveOne", 1.000000000000001, 1.0},
		{"OnePointFive", 1.5, 1.0},
		{"Two", 2.0, 1.0},
		{"LargePositive", 1e300, 1.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClampFraction(tc.input)
			if tc.name == "SmallPositive" || tc.name == "NearOne" {
				if math.Abs(got-tc.expected) > 1e-12 {
					t.Errorf("ClampFraction(%v) = %v; want %v", tc.input, got, tc.expected)
				}
			} else {
				if got != tc.expected {
					t.Errorf("ClampFraction(%v) = %v; want %v", tc.input, got, tc.expected)
				}
			}
		})
	}
}

// TestChallenger_ClassifyQuotaWindow_BoundaryAndFallbacks tests classifyQuotaWindow under
// explicit tokens, duration thresholds (>12h vs <=6h), and ambiguous intermediate windows.
func TestChallenger_ClassifyQuotaWindow_BoundaryAndFallbacks(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name      string
		window    string
		bucketID  string
		modelName string
		rTime     time.Time
		expected  string
	}{
		// Token precedence
		{"Token_Weekly_Window", "weekly", "", "unknown-model", time.Time{}, "weekly"},
		{"Token_Week_Window", "week_rolling", "", "unknown-model", time.Time{}, "weekly"},
		{"Token_7D_BucketID", "", "bucket-7d-limit", "unknown-model", time.Time{}, "weekly"},
		{"Token_Weekly_ModelName", "", "", "gemini-weekly-pro", time.Time{}, "weekly"},
		{"Token_5h_Window", "5h", "", "unknown-model", time.Time{}, "5h"},
		{"Token_5h_BucketID", "", "bucket-5h-standard", "unknown-model", time.Time{}, "5h"},
		{"Token_5h_ModelName", "", "", "claude-5h-sonnet", time.Time{}, "5h"},

		// Duration classification when no tokens match
		{"Duration_13h_Future", "", "", "custom-model", now.Add(13 * time.Hour), "weekly"},
		{"Duration_24h_Future", "", "", "custom-model", now.Add(24 * time.Hour), "weekly"},
		{"Duration_7Days_Future", "", "", "custom-model", now.Add(7 * 24 * time.Hour), "weekly"},
		{"Duration_4h_Future", "", "", "custom-model", now.Add(4 * time.Hour), "5h"},
		{"Duration_1h_Future", "", "", "custom-model", now.Add(1 * time.Hour), "5h"},
		{"Duration_5m_Future", "", "", "custom-model", now.Add(5 * time.Minute), "5h"},

		// Fallback when zero time or expired
		{"Zero_Time", "", "", "custom-model", time.Time{}, "5h"},
		{"Past_Time", "", "", "custom-model", now.Add(-1 * time.Hour), "5h"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyQuotaWindow(tc.window, tc.bucketID, tc.modelName, tc.rTime, now)
			if got != tc.expected {
				t.Errorf("classifyQuotaWindow(%q, %q, %q, %v, now) = %q; want %q",
					tc.window, tc.bucketID, tc.modelName, tc.rTime, got, tc.expected)
			}
		})
	}
}

// TestChallenger_FetchLiveQuotaBreakdown_EndToEnd tests FetchLiveQuotaBreakdown against mock
// servers with varied quotas: Gemini-only, Claude inclusion, GPT inclusion, out-of-bounds fractions,
// and flat bucket duration classification.
func TestChallenger_FetchLiveQuotaBreakdown_EndToEnd(t *testing.T) {
	origURLs := CloudCodeRetrieveQuotaURLs
	defer func() { CloudCodeRetrieveQuotaURLs = origURLs }()

	// 1. Gemini-Only Mock (No Claude, No GPT)
	geminiOnlyPayload := `{
		"user_tier": "free",
		"groups": [
			{
				"displayName": "Gemini Models",
				"buckets": [
					{
						"bucketId": "gemini-2.5-flash-5h",
						"displayName": "Gemini 2.5 Flash",
						"window": "5h",
						"remainingFraction": 0.85,
						"resetTime": "` + time.Now().Add(3*time.Hour).Format(time.RFC3339) + `"
					}
				]
			}
		]
	}`

	srvGemini := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(geminiOnlyPayload))
	}))
	defer srvGemini.Close()

	CloudCodeRetrieveQuotaURLs = []string{srvGemini.URL}
	bd1, err := FetchLiveQuotaBreakdown("dummy_token", "")
	if err != nil {
		t.Fatalf("FetchLiveQuotaBreakdown failed: %v", err)
	}

	if bd1.HasClaude {
		t.Errorf("expected HasClaude=false for Gemini-only, got true")
	}
	if bd1.HasGPT {
		t.Errorf("expected HasGPT=false for Gemini-only, got true")
	}
	if bd1.HasClaudeGPTRights {
		t.Errorf("expected HasClaudeGPTRights=false for Gemini-only, got true")
	}
	if bd1.Quota5hClaudeGPT != 0.0 {
		t.Errorf("expected Quota5hClaudeGPT=0.0, got %f", bd1.Quota5hClaudeGPT)
	}
	if bd1.QuotaWeeklyClaudeGPT != 0.0 {
		t.Errorf("expected QuotaWeeklyClaudeGPT=0.0, got %f", bd1.QuotaWeeklyClaudeGPT)
	}
	if bd1.Quota5hGemini != 0.85 {
		t.Errorf("expected Quota5hGemini=0.85, got %f", bd1.Quota5hGemini)
	}

	// 2. Claude + GPT in Groups with out-of-bounds fractions (-0.5, 1.8)
	claudeGPTPayload := `{
		"user_tier": "pro",
		"groups": [
			{
				"displayName": "Anthropic Claude Models",
				"buckets": [
					{
						"bucketId": "claude-3-5-sonnet",
						"displayName": "Claude 3.5 Sonnet",
						"window": "5h",
						"remainingFraction": -0.5,
						"resetTime": "` + time.Now().Add(2*time.Hour).Format(time.RFC3339) + `"
					}
				]
			},
			{
				"displayName": "OpenAI GPT Models",
				"buckets": [
					{
						"bucketId": "gpt-4o-weekly",
						"displayName": "GPT-4o",
						"window": "weekly",
						"remainingFraction": 1.8,
						"resetTime": "` + time.Now().Add(48*time.Hour).Format(time.RFC3339) + `"
					}
				]
			}
		]
	}`

	srvClaudeGPT := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(claudeGPTPayload))
	}))
	defer srvClaudeGPT.Close()

	CloudCodeRetrieveQuotaURLs = []string{srvClaudeGPT.URL}
	bd2, err := FetchLiveQuotaBreakdown("dummy_token", "")
	if err != nil {
		t.Fatalf("FetchLiveQuotaBreakdown failed: %v", err)
	}

	if !bd2.HasClaude {
		t.Errorf("expected HasClaude=true, got false")
	}
	if !bd2.HasGPT {
		t.Errorf("expected HasGPT=true, got false")
	}
	if !bd2.HasClaudeGPTRights {
		t.Errorf("expected HasClaudeGPTRights=true, got false")
	}
	// Verify clamping on negative fraction (-0.5 -> 0.0)
	if bd2.Quota5hClaudeGPT != 0.0 {
		t.Errorf("expected Quota5hClaudeGPT clamped to 0.0 for -0.5 input, got %f", bd2.Quota5hClaudeGPT)
	}
	// Verify clamping on excessive fraction (1.8 -> 1.0)
	if bd2.QuotaWeeklyClaudeGPT != 1.0 {
		t.Errorf("expected QuotaWeeklyClaudeGPT clamped to 1.0 for 1.8 input, got %f", bd2.QuotaWeeklyClaudeGPT)
	}

	// 3. Flat buckets with reset intervals > 12h vs <= 6h
	futureWeekly := time.Now().Add(36 * time.Hour).Format(time.RFC3339)
	futureRolling := time.Now().Add(3 * time.Hour).Format(time.RFC3339)
	flatPayload := `{
		"user_tier": "pro",
		"buckets": [
			{
				"modelId": "gemini-custom-deep",
				"remainingFraction": 0.60,
				"resetTime": "` + futureWeekly + `"
			},
			{
				"modelId": "claude-custom-fast",
				"remainingFraction": 0.40,
				"resetTime": "` + futureRolling + `"
			}
		]
	}`

	srvFlat := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(flatPayload))
	}))
	defer srvFlat.Close()

	CloudCodeRetrieveQuotaURLs = []string{srvFlat.URL}
	bd3, err := FetchLiveQuotaBreakdown("dummy_token", "")
	if err != nil {
		t.Fatalf("FetchLiveQuotaBreakdown failed: %v", err)
	}

	// 36h reset horizon should be classified as weekly
	if bd3.QuotaWeeklyGemini != 0.60 {
		t.Errorf("expected QuotaWeeklyGemini = 0.60 for 36h reset horizon, got %f", bd3.QuotaWeeklyGemini)
	}
	// 3h reset horizon for claude should be classified as 5h
	if bd3.Quota5hClaudeGPT != 0.40 {
		t.Errorf("expected Quota5hClaudeGPT = 0.40 for 3h reset horizon, got %f", bd3.Quota5hClaudeGPT)
	}
	if !bd3.HasClaude {
		t.Errorf("expected HasClaude = true for claude-custom-fast")
	}
}
