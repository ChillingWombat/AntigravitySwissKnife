package quota

import (
	"math/rand"
	"testing"
	"time"
)

func TestResolveActiveDynamicInterval(t *testing.T) {
	threshold := 0.05

	tests := []struct {
		name      string
		fraction  float64
		threshold float64
		expected  time.Duration
	}{
		// < 10% (critical)
		{"exhausted zero", 0.0, threshold, 30 * time.Second},
		{"below threshold 3%", 0.03, threshold, 30 * time.Second},
		{"at threshold 5%", 0.05, threshold, 30 * time.Second},
		{"below 10% (8%)", 0.08, threshold, 30 * time.Second},
		{"custom threshold 12% override", 0.11, 0.12, 30 * time.Second},

		// 10% - 25% (low)
		{"at 10%", 0.10, threshold, 60 * time.Second},
		{"at 15%", 0.15, threshold, 60 * time.Second},
		{"at 25%", 0.25, threshold, 60 * time.Second},

		// 25% - 50% (moderate)
		{"at 26%", 0.26, threshold, 120 * time.Second},
		{"at 40%", 0.40, threshold, 120 * time.Second},
		{"at 50%", 0.50, threshold, 120 * time.Second},

		// 50% - 75% (healthy)
		{"at 51%", 0.51, threshold, 180 * time.Second},
		{"at 65%", 0.65, threshold, 180 * time.Second},
		{"at 75%", 0.75, threshold, 180 * time.Second},

		// > 75% (pristine / full)
		{"at 76%", 0.76, threshold, 300 * time.Second},
		{"at 90%", 0.90, threshold, 300 * time.Second},
		{"at 100%", 1.00, threshold, 300 * time.Second},

		// Negative / unknown fallback
		{"negative unknown", -1.0, threshold, 60 * time.Second},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveActiveDynamicInterval(tc.fraction, tc.threshold)
			if got != tc.expected {
				t.Errorf("ResolveActiveDynamicInterval(%f, %f) = %v, expected %v", tc.fraction, tc.threshold, got, tc.expected)
			}
		})
	}
}

func TestResolveStandbyDynamicInterval(t *testing.T) {
	tests := []struct {
		name     string
		fraction float64
		expected time.Duration
	}{
		// < 50% (recovering / low)
		{"exhausted zero", 0.0, 300 * time.Second},
		{"at 20%", 0.20, 300 * time.Second},
		{"at 49%", 0.49, 300 * time.Second},

		// 50% - 80% (good)
		{"at 50%", 0.50, 600 * time.Second},
		{"at 65%", 0.65, 600 * time.Second},
		{"at 80%", 0.80, 600 * time.Second},

		// > 80% (abundant / fresh)
		{"at 81%", 0.81, 900 * time.Second},
		{"at 95%", 0.95, 900 * time.Second},
		{"at 100%", 1.00, 900 * time.Second},

		// Negative / unknown fallback
		{"negative unknown", -1.0, 300 * time.Second},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveStandbyDynamicInterval(tc.fraction)
			if got != tc.expected {
				t.Errorf("ResolveStandbyDynamicInterval(%f) = %v, expected %v", tc.fraction, got, tc.expected)
			}
		})
	}
}

func TestRollDynamicJitter_PerCycleRandomness(t *testing.T) {
	r := rand.New(rand.NewSource(42))

	nominal5m := 300 * time.Second
	roll1 := RollDynamicJitter(nominal5m, r)
	roll2 := RollDynamicJitter(nominal5m, r)
	roll3 := RollDynamicJitter(nominal5m, r)

	// In 35s max delta, rolls should be within [265s, 335s]
	for idx, roll := range []time.Duration{roll1, roll2, roll3} {
		if roll < 265*time.Second || roll > 335*time.Second {
			t.Errorf("roll %d (%v) out of expected range [265s, 335s]", idx+1, roll)
		}
	}
	// At least two rolls should differ with random source
	if roll1 == roll2 && roll2 == roll3 {
		t.Errorf("expected varying jitter per cycle, got identical values: %v", roll1)
	}

	// Test 30s critical tier: max delta +/-3s
	nominal30s := 30 * time.Second
	for i := 0; i < 20; i++ {
		roll := RollDynamicJitter(nominal30s, r)
		if roll < 27*time.Second || roll > 33*time.Second {
			t.Errorf("30s roll %v out of range [27s, 33s]", roll)
		}
	}

	// Test crypto/rand fallback when r == nil
	cryptoRoll := RollDynamicJitter(nominal5m, nil)
	if cryptoRoll < 265*time.Second || cryptoRoll > 335*time.Second {
		t.Errorf("crypto roll %v out of expected range [265s, 335s]", cryptoRoll)
	}
}

func TestExtractRemaining5HFraction(t *testing.T) {
	// 1. Nil summary
	if _, ok := ExtractRemaining5HFraction(nil); ok {
		t.Errorf("expected false for nil summary")
	}

	// 2. Direct Quota5hFraction
	s1 := &QuotaSummary{
		Quota5hFraction: 0.72,
		LastPolled:      time.Now(),
	}
	if f, ok := ExtractRemaining5HFraction(s1); !ok || f != 0.72 {
		t.Errorf("expected (0.72, true), got (%f, %v)", f, ok)
	}

	// 3. Fallback from models list
	s2 := &QuotaSummary{
		Models: []ModelQuota{
			{ModelName: "gemini-2.5-pro", Fraction: 0.45},
			{ModelName: "gemini-weekly", Fraction: 0.90},
		},
		LastPolled: time.Now(),
	}
	if f, ok := ExtractRemaining5HFraction(s2); !ok || f != 0.45 {
		t.Errorf("expected (0.45, true), got (%f, %v)", f, ok)
	}

	// 4. Exhausted genuine 0.0 with valid poll
	s3 := &QuotaSummary{
		Quota5hFraction: 0.0,
		LastPolled:      time.Now(),
	}
	if f, ok := ExtractRemaining5HFraction(s3); !ok || f != 0.0 {
		t.Errorf("expected (0.0, true), got (%f, %v)", f, ok)
	}

	// 5. Error status returns false
	s4 := &QuotaSummary{
		ErrorStatus: "UNAUTHENTICATED",
	}
	if _, ok := ExtractRemaining5HFraction(s4); ok {
		t.Errorf("expected false for error status")
	}
}

func TestGetDynamicTiersCatalog(t *testing.T) {
	activeTiers := GetActiveDynamicTiers()
	if len(activeTiers) != 5 {
		t.Fatalf("expected 5 active tiers, got %d", len(activeTiers))
	}
	if activeTiers[0].IntervalSeconds != 30 {
		t.Errorf("expected tier 0 to be 30s, got %d", activeTiers[0].IntervalSeconds)
	}
	if activeTiers[4].IntervalSeconds != 300 {
		t.Errorf("expected tier 4 to be 300s, got %d", activeTiers[4].IntervalSeconds)
	}

	standbyTiers := GetStandbyDynamicTiers()
	if len(standbyTiers) != 3 {
		t.Fatalf("expected 3 standby tiers, got %d", len(standbyTiers))
	}
	if standbyTiers[0].IntervalSeconds != 300 {
		t.Errorf("expected standby tier 0 to be 300s, got %d", standbyTiers[0].IntervalSeconds)
	}
	if standbyTiers[2].IntervalSeconds != 900 {
		t.Errorf("expected standby tier 2 to be 900s, got %d", standbyTiers[2].IntervalSeconds)
	}
}
