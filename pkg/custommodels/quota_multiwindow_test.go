package custommodels

import (
	"net/http"
	"testing"
)

// TestParseRateLimitHeaders_MultiWindow covers the #127 multi-quota behavior for
// providers that advertise several rolling windows at once (Claude's unified
// rate-limit headers expose 5H + 7D): every window must survive parsing, the
// headline fraction must be the most constrained window, and utilization-style
// headers must be inverted into remaining fractions.
func TestParseRateLimitHeaders_MultiWindow(t *testing.T) {
	h := make(http.Header)
	h.Set("anthropic-ratelimit-unified-5h-utilization", "0.30")
	h.Set("anthropic-ratelimit-unified-7d-utilization", "0.95")

	res := ParseRateLimitHeaders(h)
	if res == nil {
		t.Fatalf("expected a quota result for unified rate-limit headers, got nil")
	}
	if len(res.Buckets) != 2 {
		t.Fatalf("expected 2 quota windows (5H + 7D), got %d: %+v", len(res.Buckets), res.Buckets)
	}

	byLabel := map[string]QuotaBucket{}
	for _, b := range res.Buckets {
		byLabel[b.Label] = b
		if b.Fraction == nil {
			t.Fatalf("window %q carries no fraction", b.Label)
		}
	}
	fiveH, ok := byLabel["5H"]
	if !ok {
		t.Fatalf("missing 5H window in %+v", res.Buckets)
	}
	if got := *fiveH.Fraction; got < 0.69 || got > 0.71 {
		t.Errorf("5H utilization 0.30 must invert to ~0.70 remaining, got %.4f", got)
	}
	sevenD, ok := byLabel["7D"]
	if !ok {
		t.Fatalf("missing 7D window in %+v", res.Buckets)
	}
	if got := *sevenD.Fraction; got < 0.04 || got > 0.06 {
		t.Errorf("7D utilization 0.95 must invert to ~0.05 remaining, got %.4f", got)
	}
	// Backwards compatibility: the headline fraction is the most constrained window.
	if res.Fraction == nil || res.Fraction != sevenD.Fraction {
		t.Errorf("headline fraction should track the worst window (7D %v), got %v", sevenD.Fraction, res.Fraction)
	}

	// Remaining/limit style windows for the same provider.
	h2 := make(http.Header)
	h2.Set("x-ratelimit-5h-remaining", "60")
	h2.Set("x-ratelimit-5h-limit", "100")
	h2.Set("x-ratelimit-7d-remaining", "900")
	h2.Set("x-ratelimit-7d-limit", "1000")

	res2 := ParseRateLimitHeaders(h2)
	if res2 == nil {
		t.Fatalf("expected a quota result for windowed remaining/limit headers, got nil")
	}
	labels := []string{}
	for _, b := range res2.Buckets {
		labels = append(labels, b.Label)
	}
	if len(labels) != 2 || labels[0] != "5H" || labels[1] != "7D" {
		t.Errorf("expected deterministic [5H 7D] window order, got %v", labels)
	}
	if res2.Buckets[0].Fraction == nil || *res2.Buckets[0].Fraction != 0.6 {
		t.Errorf("5H remaining 60/100 should be 0.6, got %v", res2.Buckets[0].Fraction)
	}

	// A single-window provider must not fabricate extra buckets.
	h3 := make(http.Header)
	h3.Set("x-ratelimit-remaining-requests", "80")
	h3.Set("x-ratelimit-limit-requests", "100")
	if res3 := ParseRateLimitHeaders(h3); res3 == nil || len(res3.Buckets) != 0 {
		t.Errorf("scalar request-limit headers must stay bucket-free, got %+v", res3)
	}
}

// TestExtractQuotaBucketsFromJSON covers both documented multi-window payload
// shapes and the guarantees: labels normalized, fractions clamped, unrenderable
// entries dropped, duplicate windows deduplicated.
func TestExtractQuotaBucketsFromJSON(t *testing.T) {
	t.Run("buckets array", func(t *testing.T) {
		body := []byte(`{"buckets":[{"label":"5h","fraction":0.42,"value":"42%"},{"label":"7d","remaining":900,"limit":1000}]}`)
		buckets := ExtractQuotaBucketsFromJSON(body)
		if len(buckets) != 2 {
			t.Fatalf("expected 2 buckets, got %d: %+v", len(buckets), buckets)
		}
		if buckets[0].Label != "5H" || buckets[0].Value != "42%" {
			t.Errorf("5h bucket must normalize to label 5H with value 42%%, got %+v", buckets[0])
		}
		if buckets[1].Label != "7D" || buckets[1].Fraction == nil || *buckets[1].Fraction != 0.9 {
			t.Errorf("7d remaining/limit must yield 0.9, got %+v", buckets[1])
		}
	})

	t.Run("data limits with used/limit", func(t *testing.T) {
		body := []byte(`{"data":{"limits":[{"type":"monthly","used":25,"limit":100}]}}`)
		buckets := ExtractQuotaBucketsFromJSON(body)
		if len(buckets) != 1 || buckets[0].Label != "30D" {
			t.Fatalf("expected one 30D bucket, got %+v", buckets)
		}
		if buckets[0].Fraction == nil || *buckets[0].Fraction != 0.75 {
			t.Errorf("used 25/100 must yield 0.75 remaining, got %v", buckets[0].Fraction)
		}
	})

	t.Run("clamps, dedupes and drops empty entries", func(t *testing.T) {
		body := []byte(`{"buckets":[
			{"label":"5h","fraction":1.7},
			{"label":"5H","fraction":0.1},
			{"label":"7d"},
			{"name":"weekly","fraction":0.5}
		]}`)
		buckets := ExtractQuotaBucketsFromJSON(body)
		if len(buckets) != 2 {
			t.Fatalf("expected 2 renderable buckets (5H clamped, 7D unrenderable dropped), got %d: %+v", len(buckets), buckets)
		}
		if buckets[0].Fraction == nil || *buckets[0].Fraction != 1.0 {
			t.Errorf("fraction 1.7 must clamp to 1.0, got %v", buckets[0].Fraction)
		}
		if buckets[1].Label != "7D" || buckets[1].Fraction == nil || *buckets[1].Fraction != 0.5 {
			t.Errorf("duplicate 5H must be dropped and weekly alias mapped to 7D, got %+v", buckets[1])
		}
	})

	t.Run("scalar payload yields no buckets", func(t *testing.T) {
		if buckets := ExtractQuotaBucketsFromJSON([]byte(`{"balance":12.5}`)); len(buckets) != 0 {
			t.Errorf("scalar payload must not fabricate windows, got %+v", buckets)
		}
		if buckets := ExtractQuotaBucketsFromJSON([]byte(`not json`)); len(buckets) != 0 {
			t.Errorf("invalid JSON must yield no buckets, got %+v", buckets)
		}
	})
}

// TestFormatQuotaWindowLabel locks the display labels the dashboard relies on.
func TestFormatQuotaWindowLabel(t *testing.T) {
	cases := map[string]string{
		"5h": "5H", "5H": "5H", " 7d ": "7D", "weekly": "7D", "monthly": "30D",
		"30d": "30D", "daily": "24H", "": "Quota", "all": "Quota", "customwindow": "Customwindow",
	}
	for in, want := range cases {
		if got := FormatQuotaWindowLabel(in); got != want {
			t.Errorf("FormatQuotaWindowLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
