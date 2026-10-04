package quota

import (
	"fmt"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// ModelQuota contains consumption metrics for a specific Gemini model.
type ModelQuota struct {
	ModelName    string    `json:"model_name"`
	Fraction     float64   `json:"fraction"`
	ResetTime    time.Time `json:"reset_time"`
	ResetText    string    `json:"reset_text"`
	HealthStatus string    `json:"health_status"`
}

// ComputeHealth determines the status tier based on remaining fraction.
func ComputeHealth(fraction float64) string {
	if fraction > 0.30 {
		return core.StatusHealthy
	} else if fraction >= 0.10 {
		return core.StatusWarning
	}
	return core.StatusExhausted
}

// FormatResetHorizon converts a future reset time into a user-friendly countdown string.
func FormatResetHorizon(resetTime time.Time, now time.Time) string {
	if resetTime.IsZero() || resetTime.Before(now) {
		return "Ready"
	}
	diff := resetTime.Sub(now)
	if diff < time.Minute {
		return fmt.Sprintf("Resets in %ds", int(diff.Seconds()))
	} else if diff < time.Hour {
		return fmt.Sprintf("Resets in %dm", int(diff.Minutes()))
	}
	hours := int(diff.Hours())
	mins := int(diff.Minutes()) % 60
	return fmt.Sprintf("Resets in %dh %dm", hours, mins)
}

// QuotaSummary consolidates per-model quotas for an account.
type QuotaSummary struct {
	AccountEmail  string       `json:"account_email"`
	Models        []ModelQuota `json:"models"`
	MinFraction   float64      `json:"min_fraction"`
	OverallHealth string       `json:"overall_health"`
	LastPolled    time.Time    `json:"last_polled"`
}
