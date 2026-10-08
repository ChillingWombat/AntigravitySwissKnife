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
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		mins := int(diff.Minutes()) % 60
		return fmt.Sprintf("Resets in %dh %dm", hours, mins)
	}
	days := int(diff.Hours()) / 24
	hours := int(diff.Hours()) % 24
	return fmt.Sprintf("Resets in %dd %dh", days, hours)
}

// QuotaSummary consolidates per-model quotas for an account.
type QuotaSummary struct {
	AccountEmail           string       `json:"account_email"`
	PlanTier               string       `json:"plan_tier,omitempty"`
	Credits                float64      `json:"credits"`
	Quota5hFraction        float64      `json:"quota_5h_fraction,omitempty"`
	QuotaWeeklyFraction    float64      `json:"quota_weekly_fraction,omitempty"`
	Quota5hClaudeGPT       float64      `json:"quota_5h_claude_gpt,omitempty"`
	QuotaWeeklyClaudeGPT   float64      `json:"quota_weekly_claude_gpt,omitempty"`
	ResetSeconds5h         float64      `json:"reset_seconds_5h,omitempty"`
	ResetHorizonText       string       `json:"reset_horizon_text,omitempty"`
	ResetSecondsWeekly     float64      `json:"reset_seconds_weekly,omitempty"`
	ResetHorizonWeeklyText string       `json:"reset_horizon_weekly_text,omitempty"`
	ResetTimeWeekly        time.Time    `json:"reset_time_weekly,omitempty"`
	Models                 []ModelQuota `json:"models"`
	MinFraction            float64      `json:"min_fraction"`
	OverallHealth          string       `json:"overall_health"`
	ErrorMessage           string       `json:"error_message,omitempty"`
	ErrorStatus            string       `json:"error_status,omitempty"`
	LastPolled             time.Time    `json:"last_polled"`
}
