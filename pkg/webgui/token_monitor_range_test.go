package webgui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
)

// fixtureStep describes one token-bearing transcript step used by the range tests.
type fixtureStep struct {
	ago          time.Duration
	freshInput   int64
	cachedInput  int64
	outputTokens int64
}

// tokenSummaryFixture mirrors the fields of TokenSummaryResponse the range tests assert on.
type tokenSummaryFixture struct {
	Range             string `json:"range"`
	TotalTokens       int64  `json:"total_tokens"`
	InputTokens       int64  `json:"input_tokens"` // prompt side: fresh + cache reads (#126)
	CachedInputTokens int64  `json:"cached_input_tokens"`
	OutputTokens      int64  `json:"output_tokens"`
	RequestsCount     int    `json:"requests_count"`
	ModelBreakdowns   []struct {
		ModelName  string `json:"model_name"`
		TotalToken int64  `json:"total_tokens"`
		Requests   int    `json:"requests"`
	} `json:"model_breakdowns"`
	ProjectBreakdowns []struct {
		ProjectName string `json:"project_name"`
		TotalToken  int64  `json:"total_tokens"`
		Requests    int    `json:"requests"`
	} `json:"project_breakdowns"`
	Trend             struct {
		BucketMS int64 `json:"bucket_ms"`
		Buckets  []struct {
			Start        int64 `json:"start"`
			FreshTokens  int64 `json:"fresh_tokens"`
			CachedTokens int64 `json:"cached_tokens"`
			OutputTokens int64 `json:"output_tokens"`
			TotalTokens  int64 `json:"total_tokens"`
			Requests     int   `json:"requests"`
			Models       []struct {
				Name     string `json:"name"`
				Tokens   int64  `json:"tokens"`
				Requests int    `json:"requests"`
			} `json:"models"`
			Projects []struct {
				Name     string `json:"name"`
				Tokens   int64  `json:"tokens"`
				Requests int    `json:"requests"`
			} `json:"projects"`
		} `json:"buckets"`
	} `json:"trend"`
}

// newRangeTestServer builds a Server wired to tmp data dir + a temp store. The
// catalog fetcher is stubbed out so the handler never touches the network.
func newRangeTestServer(t *testing.T, dataDir string, models []custommodels.CustomModel) *Server {
	t.Helper()
	store, err := custommodels.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create temp custom models store: %v", err)
	}
	for _, m := range models {
		if err := store.SaveModel(m); err != nil {
			t.Fatalf("failed to seed custom model %q: %v", m.Name, err)
		}
	}
	srv := &Server{antigravityDataDir: dataDir, customModelsStore: store}
	srv.catalogFetcher = func(bool) []custommodels.CatalogModelInput { return nil }
	return srv
}

// writeRangeTranscript writes a transcript.jsonl whose step 0 selects modelName
// (so model detection resolves it) followed by the provided token steps.
func writeRangeTranscript(t *testing.T, brainDir, convID, modelName string, now time.Time, steps []fixtureStep) {
	t.Helper()
	logsDir := filepath.Join(brainDir, convID, ".system_generated", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("failed to create transcript dir: %v", err)
	}

	lines := []string{
		fmt.Sprintf(`{"step_index":0,"source":"SYSTEM","type":"SYSTEM_MESSAGE","created_at":%q,"content":"<USER_SETTINGS_CHANGE>\nThe user changed setting `+"`"+`Model Selection`+"`"+` from None to %s.\n</USER_SETTINGS_CHANGE>"}`,
			now.Add(-30*24*time.Hour).Format(time.RFC3339Nano), modelName),
	}
	for i, st := range steps {
		lines = append(lines, fmt.Sprintf(
			`{"step_index":%d,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":%q,"input_tokens":%d,"cache_read_tokens":%d,"output_tokens":%d}`,
			i+1, now.Add(-st.ago).Format(time.RFC3339Nano), st.freshInput, st.cachedInput, st.outputTokens))
	}

	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(filepath.Join(logsDir, "transcript.jsonl"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write transcript: %v", err)
	}
}

// queryTokenSummary runs the real summary handler for the given range and decodes it.
func queryTokenSummary(t *testing.T, srv *Server, rangeKey string) tokenSummaryFixture {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/tokens/summary?range="+rangeKey, nil)
	rec := httptest.NewRecorder()
	srv.handleTokensSummary(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("range=%s: expected HTTP 200, got %d (body: %s)", rangeKey, rec.Code, rec.Body.String())
	}
	var out tokenSummaryFixture
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("range=%s: failed to decode summary JSON: %v", rangeKey, err)
	}
	return out
}

func trendBucketTotal(resp tokenSummaryFixture) int64 {
	var sum int64
	for _, b := range resp.Trend.Buckets {
		sum += b.TotalTokens
	}
	return sum
}

func seedManualModel(name, displayName string, in, cache, out float64) custommodels.CustomModel {
	inP, cacheP, outP := in, cache, out
	return custommodels.CustomModel{
		ID:                   name,
		Name:                 name,
		DisplayName:          displayName,
		ProviderType:         custommodels.ProviderOpenAI,
		BaseURL:              "https://api.example.invalid/v1",
		APIKey:               "test-key",
		QuotaType:            custommodels.QuotaTypeBalance,
		BalanceValue:         "$10.00",
		InputPricePerM:       &inP,
		CachedInputPricePerM: &cacheP,
		OutputPricePerM:      &outP,
		PriceSource:          custommodels.PriceSourceManual,
		Enabled:              true,
	}
}

// TestTokenSummary_RangeFilter_SharesWindowWithHistogram is the #125 regression
// guard: the same on-disk fixture must produce different KPI totals for different
// ranges, and the histogram buckets must always sum back to the KPI totals — i.e.
// cards and chart consume the exact same window.
func TestTokenSummary_RangeFilter_SharesWindowWithHistogram(t *testing.T) {
	now := time.Now()
	dataDir := t.TempDir()
	brainDir := filepath.Join(dataDir, "brain")

	// One conversation with steps 2h ago (inside 24h), 3d ago (inside 7d only)
	// and 20d ago (only inside "all").
	writeRangeTranscript(t, brainDir, "conv-range", "gpt-4o", now, []fixtureStep{
		{ago: 2 * time.Hour, freshInput: 1000, cachedInput: 500, outputTokens: 100},
		{ago: 3 * 24 * time.Hour, freshInput: 2000, cachedInput: 1000, outputTokens: 200},
		{ago: 20 * 24 * time.Hour, freshInput: 4000, cachedInput: 2000, outputTokens: 400},
	})
	srv := newRangeTestServer(t, dataDir, []custommodels.CustomModel{
		seedManualModel("gpt-4o", "GPT-4o", 1, 0.25, 2),
	})

	cases := []struct {
		rangeKey     string
		wantFresh    int64
		wantCached   int64
		wantOutput   int64
		wantRequests int
		wantBuckets  int
	}{
		{"24h", 1000, 500, 100, 1, 24},
		{"7d", 3000, 1500, 300, 2, 7},
		{"all", 7000, 3500, 700, 3, 0}, // adaptive bucket count asserted separately
	}

	var prevTotal int64 = -1
	for _, tc := range cases {
		resp := queryTokenSummary(t, srv, tc.rangeKey)

		if resp.Range != tc.rangeKey {
			t.Errorf("range=%s: response echoed %q", tc.rangeKey, resp.Range)
		}
		if resp.CachedInputTokens != tc.wantCached {
			t.Errorf("range=%s: cached input = %d, want %d", tc.rangeKey, resp.CachedInputTokens, tc.wantCached)
		}
		// `input_tokens` is the prompt side (fresh + cache reads) per the #126
		// accounting definition.
		if resp.InputTokens != tc.wantFresh+tc.wantCached {
			t.Errorf("range=%s: input_tokens = %d, want %d (fresh+cached)", tc.rangeKey, resp.InputTokens, tc.wantFresh+tc.wantCached)
		}
		if resp.OutputTokens != tc.wantOutput {
			t.Errorf("range=%s: output = %d, want %d", tc.rangeKey, resp.OutputTokens, tc.wantOutput)
		}
		if resp.RequestsCount != tc.wantRequests {
			t.Errorf("range=%s: requests = %d, want %d", tc.rangeKey, resp.RequestsCount, tc.wantRequests)
		}
		wantTotal := tc.wantFresh + tc.wantCached + tc.wantOutput
		if resp.TotalTokens != wantTotal {
			t.Errorf("range=%s: total = %d, want %d", tc.rangeKey, resp.TotalTokens, wantTotal)
		}

		// The KPI cards and the histogram MUST share the window (#125).
		if got := trendBucketTotal(resp); got != resp.TotalTokens {
			t.Errorf("range=%s: histogram total %d != KPI total %d (cards and chart diverged)", tc.rangeKey, got, resp.TotalTokens)
		}

		if tc.wantBuckets > 0 && len(resp.Trend.Buckets) != tc.wantBuckets {
			t.Errorf("range=%s: bucket count = %d, want %d", tc.rangeKey, len(resp.Trend.Buckets), tc.wantBuckets)
		}
		if tc.rangeKey == "24h" && resp.Trend.BucketMS != int64(time.Hour/time.Millisecond) {
			t.Errorf("range=24h: bucket_ms = %d, want %d", resp.Trend.BucketMS, time.Hour/time.Millisecond)
		}
		if tc.rangeKey == "7d" && resp.Trend.BucketMS != int64(24*time.Hour/time.Millisecond) {
			t.Errorf("range=7d: bucket_ms = %d, want %d", resp.Trend.BucketMS, 24*time.Hour/time.Millisecond)
		}
		if tc.rangeKey == "all" && (resp.Trend.BucketMS <= 0 || len(resp.Trend.Buckets) == 0) {
			t.Errorf("range=all: adaptive bucketing missing (bucket_ms=%d buckets=%d)", resp.Trend.BucketMS, len(resp.Trend.Buckets))
		}

		if prevTotal >= 0 && resp.TotalTokens <= prevTotal {
			t.Errorf("range=%s: total %d did not grow beyond previous range's %d — range filter is not narrowing the window", tc.rangeKey, resp.TotalTokens, prevTotal)
		}
		prevTotal = resp.TotalTokens
	}
}

// TestTokenSummary_EmptyWindow_RendersNoSyntheticData pins down the other half of
// #125: when the selected window holds no steps the KPIs are zero and every
// histogram bucket is zero. No Math.sin-style fake curve may ever appear.
func TestTokenSummary_EmptyWindow_RendersNoSyntheticData(t *testing.T) {
	now := time.Now()
	dataDir := t.TempDir()
	brainDir := filepath.Join(dataDir, "brain")

	// Only 3 days old: outside the 24h window, inside 7d.
	writeRangeTranscript(t, brainDir, "conv-old", "gpt-4o", now, []fixtureStep{
		{ago: 3 * 24 * time.Hour, freshInput: 5000, cachedInput: 2500, outputTokens: 500},
	})
	srv := newRangeTestServer(t, dataDir, []custommodels.CustomModel{
		seedManualModel("gpt-4o", "GPT-4o", 1, 0.25, 2),
	})

	empty := queryTokenSummary(t, srv, "24h")
	if empty.TotalTokens != 0 || empty.RequestsCount != 0 {
		t.Fatalf("range=24h on a 3-day-old fixture: total=%d requests=%d, want 0/0", empty.TotalTokens, empty.RequestsCount)
	}
	if len(empty.Trend.Buckets) != 24 {
		t.Fatalf("range=24h: expected 24 empty buckets for the axes, got %d", len(empty.Trend.Buckets))
	}
	for i, b := range empty.Trend.Buckets {
		if b.TotalTokens != 0 || b.FreshTokens != 0 || b.CachedTokens != 0 || b.OutputTokens != 0 || b.Requests != 0 {
			t.Fatalf("bucket %d carries synthetic data: %+v", i, b)
		}
	}

	// Sanity: the same fixture is non-empty once the window includes it.
	full := queryTokenSummary(t, srv, "7d")
	if full.TotalTokens != 8000 || full.RequestsCount != 1 {
		t.Fatalf("range=7d on the same fixture: total=%d requests=%d, want 8000/1", full.TotalTokens, full.RequestsCount)
	}
}

// TestTokenSummary_StackedSegments_SumToBucketTotal verifies the stacked
// histogram carries per-model / per-project segments whose token and request
// sums equal the bucket total, and that both units come from the same records.
func TestTokenSummary_StackedSegments_SumToBucketTotal(t *testing.T) {
	now := time.Now()
	dataDir := t.TempDir()
	brainDir := filepath.Join(dataDir, "brain")

	writeRangeTranscript(t, brainDir, "conv-a", "gpt-4o", now, []fixtureStep{
		{ago: 2 * time.Hour, freshInput: 1000, cachedInput: 0, outputTokens: 100},
		{ago: 1 * time.Hour, freshInput: 2000, cachedInput: 0, outputTokens: 200},
	})
	writeRangeTranscript(t, brainDir, "conv-b", "claude-3-7-sonnet", now, []fixtureStep{
		// Same hour bucket as conv-a's first step so one bar stacks two models.
		{ago: 2 * time.Hour, freshInput: 500, cachedInput: 500, outputTokens: 50},
	})
	srv := newRangeTestServer(t, dataDir, []custommodels.CustomModel{
		seedManualModel("gpt-4o", "GPT-4o", 1, 0.25, 2),
		seedManualModel("claude-3-7-sonnet", "Claude 3.7 Sonnet", 3, 0.3, 15),
	})

	resp := queryTokenSummary(t, srv, "24h")
	if len(resp.Trend.Buckets) != 24 {
		t.Fatalf("expected 24 hourly buckets, got %d", len(resp.Trend.Buckets))
	}

	// Native + custom breakdowns must add up to the headline totals exactly once
	// Each conversation is attributed to exactly one model, so the breakdowns
	// cannot re-count the same tokens (#126).
	var breakdownTokens int64
	var breakdownRequests int
	for _, m := range resp.ModelBreakdowns {
		breakdownTokens += m.TotalToken
		breakdownRequests += m.Requests
	}
	if breakdownTokens != resp.TotalTokens {
		t.Errorf("model breakdown tokens %d != total %d (double counting or dropped usage)", breakdownTokens, resp.TotalTokens)
	}
	if breakdownRequests != resp.RequestsCount {
		t.Errorf("model breakdown requests %d != total requests %d", breakdownRequests, resp.RequestsCount)
	}
	var projectTokens int64
	for _, p := range resp.ProjectBreakdowns {
		projectTokens += p.TotalToken
	}
	if projectTokens != resp.TotalTokens {
		t.Errorf("project breakdown tokens %d != total %d", projectTokens, resp.TotalTokens)
	}

	var sawTwoModels, sawSegments bool
	for _, b := range resp.Trend.Buckets {
		if b.TotalTokens == 0 {
			if len(b.Models) != 0 || len(b.Projects) != 0 {
				t.Errorf("empty bucket carries segments: %+v", b)
			}
			continue
		}
		var modelSum, modelReqs int64
		for _, m := range b.Models {
			modelSum += m.Tokens
			modelReqs += int64(m.Requests)
		}
		if modelSum != b.TotalTokens {
			t.Errorf("bucket %d: model segment sum %d != bucket total %d", b.Start, modelSum, b.TotalTokens)
		}
		if modelReqs != int64(b.Requests) {
			t.Errorf("bucket %d: model segment requests %d != bucket requests %d", b.Start, modelReqs, b.Requests)
		}
		var projectSum int64
		for _, p := range b.Projects {
			projectSum += p.Tokens
		}
		if projectSum != b.TotalTokens {
			t.Errorf("bucket %d: project segment sum %d != bucket total %d", b.Start, projectSum, b.TotalTokens)
		}
		if len(b.Models) > 1 {
			sawTwoModels = true
		}
		sawSegments = true
	}
	if !sawSegments {
		t.Fatal("no bucket carried segments — the histogram would render empty")
	}
	if !sawTwoModels {
		t.Fatal("expected at least one bucket stacking two different models")
	}
}

// TestBuildTokenTrend_AllRangeAdaptiveBuckets checks the "all" range bucketing:
// daily buckets for short spans, never more than maxAdaptiveBuckets, and no
// token lost when steps are folded into their bucket.
func TestBuildTokenTrend_AllRangeAdaptiveBuckets(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.Local)
	earliest := now.Add(-20 * 24 * time.Hour)
	steps := []stepWithContext{
		{rawStepEvent: rawStepEvent{createdAt: earliest, freshInput: 100, output: 10}, modelName: "m1", project: "p1"},
		{rawStepEvent: rawStepEvent{createdAt: now.Add(-2 * time.Hour), freshInput: 200, output: 20}, modelName: "m1", project: "p1"},
	}

	buckets, bucketMS := buildTokenTrend(steps, parseTokenRange("all"), time.Time{}, now)
	if bucketMS != int64(24*time.Hour/time.Millisecond) {
		t.Errorf("expected daily buckets for a 20-day span, got bucket_ms=%d", bucketMS)
	}
	if len(buckets) > maxAdaptiveBuckets {
		t.Errorf("bucket count %d exceeds maxAdaptiveBuckets %d", len(buckets), maxAdaptiveBuckets)
	}
	var sum int64
	for _, b := range buckets {
		sum += b.TotalTokens
	}
	if sum != 330 {
		t.Errorf("adaptive bucketing lost tokens: sum=%d, want 330", sum)
	}

	// A very long span (2 years) must coarsen the buckets instead of exploding.
	longSteps := []stepWithContext{{rawStepEvent: rawStepEvent{createdAt: now.Add(-2 * 365 * 24 * time.Hour), freshInput: 10, output: 1}}}
	longBuckets, longMS := buildTokenTrend(longSteps, parseTokenRange("all"), time.Time{}, now)
	if len(longBuckets) > maxAdaptiveBuckets {
		t.Errorf("2-year span produced %d buckets, exceeding cap %d", len(longBuckets), maxAdaptiveBuckets)
	}
	if longMS <= int64(24*time.Hour/time.Millisecond) {
		t.Errorf("2-year span should coarsen past daily buckets, got bucket_ms=%d", longMS)
	}
}

// TestParseTokenRange_Canonicalization locks the accepted range keys: unknown or
// empty values degrade to the all-time window instead of erroring.
func TestParseTokenRange_Canonicalization(t *testing.T) {
	cases := map[string]string{
		"24h": "24h", "24H": "24h", " 7d ": "7d", "30d": "30d", "all": "all",
		"": "all", "bogus": "all", "90d": "all",
	}
	for input, want := range cases {
		if got := parseTokenRange(input).Key; got != want {
			t.Errorf("parseTokenRange(%q).Key = %q, want %q", input, got, want)
		}
	}
}
