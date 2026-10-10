package quota

import (
	crand "crypto/rand"
	"math/big"
	"math/rand"
	"strings"
	"time"
)

// DynamicFrequencyTier represents a single adaptive quota tier.
type DynamicFrequencyTier struct {
	ThresholdFraction float64       `json:"threshold_fraction"`
	Interval          time.Duration `json:"interval"`
	IntervalSeconds   int           `json:"interval_seconds"`
	TierLabel         string        `json:"tier_label"`
	Description       string        `json:"description"`
}

// Active Account Nominal Intervals (Relaxed / Quota-Friendly):
// > 75%: 5 minutes (300s)
// 50% – 75%: 3 minutes (180s)
// 25% – 50%: 2 minutes (120s)
// 10% – 25%: 60 seconds (60s)
// < 10%: 30 seconds (30s)
const (
	ActiveIntervalPristine = 300 * time.Second // > 75%
	ActiveIntervalHealthy  = 180 * time.Second // 50% - 75%
	ActiveIntervalModerate = 120 * time.Second // 25% - 50%
	ActiveIntervalLow      = 60 * time.Second  // 10% - 25%
	ActiveIntervalCritical = 30 * time.Second  // < 10%
)

// Standby Account Nominal Intervals (Relaxed / Quota-Friendly):
// > 80%: 15 minutes (900s)
// 50% – 80%: 10 minutes (600s)
// < 50%: 5 minutes (300s)
const (
	StandbyIntervalAbundant   = 900 * time.Second // > 80%
	StandbyIntervalGood       = 600 * time.Second // 50% - 80%
	StandbyIntervalRecovering = 300 * time.Second // < 50%
)

// GetActiveDynamicTiers returns the user-facing tier definitions for active accounts.
func GetActiveDynamicTiers() []DynamicFrequencyTier {
	return []DynamicFrequencyTier{
		{
			ThresholdFraction: 0.10,
			Interval:          ActiveIntervalCritical,
			IntervalSeconds:   30,
			TierLabel:         "< 10% (Near Threshold)",
			Description:       "30s check ensures auto-switch rotates before quota exhaustion",
		},
		{
			ThresholdFraction: 0.25,
			Interval:          ActiveIntervalLow,
			IntervalSeconds:   60,
			TierLabel:         "10% – 25% (Low Quota)",
			Description:       "60s check captures downward burn progression",
		},
		{
			ThresholdFraction: 0.50,
			Interval:          ActiveIntervalModerate,
			IntervalSeconds:   120,
			TierLabel:         "25% – 50% (Moderate Quota)",
			Description:       "2m check provides balanced tracking",
		},
		{
			ThresholdFraction: 0.75,
			Interval:          ActiveIntervalHealthy,
			IntervalSeconds:   180,
			TierLabel:         "50% – 75% (Healthy Quota)",
			Description:       "3m check saves API calls while account is healthy",
		},
		{
			ThresholdFraction: 1.00,
			Interval:          ActiveIntervalPristine,
			IntervalSeconds:   300,
			TierLabel:         "> 75% (Abundant Quota)",
			Description:       "5m check avoids unnecessary requests on fresh quota",
		},
	}
}

// GetStandbyDynamicTiers returns the user-facing tier definitions for standby accounts.
func GetStandbyDynamicTiers() []DynamicFrequencyTier {
	return []DynamicFrequencyTier{
		{
			ThresholdFraction: 0.50,
			Interval:          StandbyIntervalRecovering,
			IntervalSeconds:   300,
			TierLabel:         "< 50% (Recovering / Low)",
			Description:       "5m check detects reset recovery and readiness",
		},
		{
			ThresholdFraction: 0.80,
			Interval:          StandbyIntervalGood,
			IntervalSeconds:   600,
			TierLabel:         "50% – 80% (Moderate Standby)",
			Description:       "10m check verifies standby availability",
		},
		{
			ThresholdFraction: 1.00,
			Interval:          StandbyIntervalAbundant,
			IntervalSeconds:   900,
			TierLabel:         "> 80% (Abundant Standby)",
			Description:       "15m check conserves quota while account is idle and ready",
		},
	}
}

// ResolveActiveDynamicInterval calculates the active polling interval based on remaining fraction and threshold.
func ResolveActiveDynamicInterval(remainingFraction float64, autoSwitchThreshold float64) time.Duration {
	if remainingFraction < 0 {
		return ActiveIntervalLow // unknown, default to 60s
	}
	// If quota is at or below the auto-switch threshold, poll at critical 30s
	if autoSwitchThreshold > 0 && remainingFraction <= autoSwitchThreshold {
		return ActiveIntervalCritical
	}
	if remainingFraction < 0.10 {
		return ActiveIntervalCritical
	}
	if remainingFraction <= 0.25 {
		return ActiveIntervalLow
	}
	if remainingFraction <= 0.50 {
		return ActiveIntervalModerate
	}
	if remainingFraction <= 0.75 {
		return ActiveIntervalHealthy
	}
	return ActiveIntervalPristine
}

// ResolveStandbyDynamicInterval calculates the standby polling interval based on remaining fraction.
func ResolveStandbyDynamicInterval(remainingFraction float64) time.Duration {
	if remainingFraction < 0 {
		return StandbyIntervalRecovering // unknown, default to 5m
	}
	if remainingFraction < 0.50 {
		return StandbyIntervalRecovering
	}
	if remainingFraction <= 0.80 {
		return StandbyIntervalGood
	}
	return StandbyIntervalAbundant
}

// RollDynamicJitter applies a per-refresh random delta (±seconds or ±minutes) to the nominal interval.
// Re-rolled afresh on every cycle iteration to prevent synchronized robotic polling.
func RollDynamicJitter(nominal time.Duration, r *rand.Rand) time.Duration {
	if nominal <= 0 {
		return nominal
	}

	var maxDeltaSec int
	switch {
	case nominal <= 30*time.Second:
		maxDeltaSec = 3 // +/- 3s for 30s
	case nominal <= 75*time.Second:
		maxDeltaSec = 8 // +/- 8s for 60s
	case nominal <= 140*time.Second:
		maxDeltaSec = 15 // +/- 15s for 120s
	case nominal <= 200*time.Second:
		maxDeltaSec = 20 // +/- 20s for 180s
	case nominal <= 360*time.Second:
		maxDeltaSec = 35 // +/- 35s for 300s (5m)
	case nominal <= 660*time.Second:
		maxDeltaSec = 60 // +/- 60s (1m) for 600s (10m)
	case nominal <= 1000*time.Second:
		maxDeltaSec = 90 // +/- 90s (1.5m) for 900s (15m)
	default:
		maxDeltaSec = 120 // +/- 120s (2m) for 1800s (30m)
	}

	var deltaSec int
	if r != nil {
		deltaSec = r.Intn(2*maxDeltaSec+1) - maxDeltaSec
	} else {
		nBig, err := crand.Int(crand.Reader, big.NewInt(int64(2*maxDeltaSec+1)))
		if err == nil {
			deltaSec = int(nBig.Int64()) - maxDeltaSec
		}
	}

	res := nominal + time.Duration(deltaSec)*time.Second
	if res < 15*time.Second {
		res = 15 * time.Second
	}
	return res
}

// ResolveActiveDynamicIntervalWithJitter calculates the active interval and applies a fresh random jitter roll.
func ResolveActiveDynamicIntervalWithJitter(remainingFraction float64, autoSwitchThreshold float64, r *rand.Rand) time.Duration {
	nominal := ResolveActiveDynamicInterval(remainingFraction, autoSwitchThreshold)
	return RollDynamicJitter(nominal, r)
}

// ResolveStandbyDynamicIntervalWithJitter calculates the standby interval and applies a fresh random jitter roll.
func ResolveStandbyDynamicIntervalWithJitter(remainingFraction float64, r *rand.Rand) time.Duration {
	nominal := ResolveStandbyDynamicInterval(remainingFraction)
	return RollDynamicJitter(nominal, r)
}

// ExtractRemaining5HFraction extracts the primary 5-hour quota fraction from a QuotaSummary.
// Returns (fraction, true) if valid quota info is found, or (0, false) if unpolled/error/nil.
func ExtractRemaining5HFraction(s *QuotaSummary) (float64, bool) {
	if s == nil {
		return 0, false
	}
	if s.Quota5hFraction > 0 {
		return s.Quota5hFraction, true
	}
	// Check models if Quota5hFraction was not set directly
	if len(s.Models) > 0 {
		for _, m := range s.Models {
			mLower := strings.ToLower(m.ModelName)
			if !strings.Contains(mLower, "weekly") {
				return m.Fraction, true
			}
		}
	}
	// Fallback to weekly quota if only weekly model quota exists
	if s.QuotaWeeklyFraction > 0 {
		return s.QuotaWeeklyFraction, true
	}
	// If no error status and last polled is not zero, fraction 0.0 is genuinely exhausted quota
	if s.ErrorStatus == "" && s.ErrorMessage == "" && !s.LastPolled.IsZero() {
		return 0.0, true
	}
	return 0, false
}
