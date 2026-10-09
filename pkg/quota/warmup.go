package quota

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

// OneTokenSubRequest contains the inner generation payload for Cloud Code.
type OneTokenSubRequest struct {
	Contents         []ContentItem    `json:"contents"`
	GenerationConfig GenerationConfig `json:"generationConfig"`
}

// OneTokenPayload creates the lightweight 1-token keep-alive request structure.
type OneTokenPayload struct {
	Project string              `json:"project"`
	Model   string              `json:"model"`
	Request OneTokenSubRequest  `json:"request"`
}

type ContentItem struct {
	Role  string     `json:"role,omitempty"`
	Parts []PartItem `json:"parts"`
}

type PartItem struct {
	Text string `json:"text"`
}

type GenerationConfig struct {
	MaxOutputTokens int `json:"maxOutputTokens"`
}

// Build1TokenKeepAliveJSON serializes a minimal ping payload to activate the next horizon.
func Build1TokenKeepAliveJSON() ([]byte, error) {
	return Build1TokenKeepAliveJSONWithModelAndProject("gemini-3-flash", "aicode-consumers")
}

// Build1TokenKeepAliveJSONWithModelAndProject creates a keep-alive payload with custom model and project.
func Build1TokenKeepAliveJSONWithModelAndProject(model, project string) ([]byte, error) {
	if model == "" {
		model = "gemini-3-flash"
	}
	if project == "" {
		project = "aicode-consumers"
	}
	p := OneTokenPayload{
		Project: project,
		Model:   model,
		Request: OneTokenSubRequest{
			Contents: []ContentItem{
				{
					Role: "user",
					Parts: []PartItem{
						{Text: "ping"},
					},
				},
			},
			GenerationConfig: GenerationConfig{
				MaxOutputTokens: 1,
			},
		},
	}
	return json.Marshal(p)
}

// WarmupScheduler tracks post-reset verification delays and determines cooldown skipping and ignition timing.
type WarmupScheduler struct {
	PostResetDelay time.Duration
	LeadTime       time.Duration // alias for backward compatibility
}

// GetDelay returns the configured post-reset delay duration, falling back to LeadTime if PostResetDelay is zero.
func (ws *WarmupScheduler) GetDelay() time.Duration {
	if ws == nil {
		return 0
	}
	if ws.PostResetDelay != 0 {
		return ws.PostResetDelay
	}
	return ws.LeadTime
}

// ShouldSkipCooldownPolling returns true if resetTime is non-zero and now is strictly before resetTime.
// Automatic background polling is skipped during cooldown to eliminate unnecessary network traffic and rate limit pressure.
func (ws *WarmupScheduler) ShouldSkipCooldownPolling(resetTime time.Time, now time.Time) bool {
	if resetTime.IsZero() {
		return false
	}
	return now.Before(resetTime)
}

// ShouldTriggerPostResetIgnition returns true if resetTime is non-zero and the delay window has elapsed
// (!now.Before(resetTime.Add(delay)), i.e. now >= resetTime + delay), indicating the account is ready
// for quota reset verification and 1-token ignition.
func (ws *WarmupScheduler) ShouldTriggerPostResetIgnition(resetTime time.Time, now time.Time) bool {
	if resetTime.IsZero() {
		return false
	}
	delay := ws.GetDelay()
	ignitionTarget := resetTime.Add(delay)
	return !now.Before(ignitionTarget)
}

// ShouldTrigger is preserved for backward compatibility and delegates to ShouldTriggerPostResetIgnition.
func (ws *WarmupScheduler) ShouldTrigger(resetTime time.Time, now time.Time) bool {
	return ws.ShouldTriggerPostResetIgnition(resetTime, now)
}

// GetResetTime returns the earliest non-zero reset time for the QuotaSummary.
func (qs *QuotaSummary) GetResetTime() time.Time {
	if qs == nil {
		return time.Time{}
	}
	var earliest time.Time
	for _, m := range qs.Models {
		if !m.ResetTime.IsZero() {
			if earliest.IsZero() || m.ResetTime.Before(earliest) {
				earliest = m.ResetTime
			}
		}
	}
	if !earliest.IsZero() {
		return earliest
	}
	if qs.ResetSeconds5h > 0 && !qs.LastPolled.IsZero() {
		return qs.LastPolled.Add(time.Duration(qs.ResetSeconds5h) * time.Second)
	}
	if !qs.ResetTimeWeekly.IsZero() {
		return qs.ResetTimeWeekly
	}
	return time.Time{}
}

// IsCooldown returns true if the account is exhausted, rate-limited, or has depleted quota below the threshold.
func (qs *QuotaSummary) IsCooldown(threshold float64) bool {
	if qs == nil {
		return false
	}
	if threshold <= 0 {
		threshold = core.DefaultAutoSwitchThresholdFraction
	}
	if qs.OverallHealth == core.StatusExhausted || qs.MinFraction <= threshold || strings.EqualFold(qs.ErrorStatus, "RATE_LIMIT_EXCEEDED") {
		return true
	}
	return false
}

// CloudCodeGenerateContentURLs defines Google endpoints for 1-token keep-alive probes.
var CloudCodeGenerateContentURLs = []string{
	"https://daily-cloudcode-pa.googleapis.com/v1internal:generateContent",
	"https://daily-cloudcode-pa.googleapis.com/v1internal:streamGenerateContent?alt=sse",
	"https://cloudcode-pa.googleapis.com/v1internal:generateContent",
	"https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:generateContent",
	"https://cloudcodeassist-pa.googleapis.com/v1internal:generateContent",
	"https://cloudaicompanion.googleapis.com/v1internal:generateContent",
}

// Send1TokenKeepAliveProbe dispatches a 1-token keep-alive ping payload to ignite Google's 5-hour rolling timer.
func Send1TokenKeepAliveProbe(accessToken string) error {
	return Send1TokenKeepAliveProbeWithProject(accessToken, "aicode-consumers")
}

// Send1TokenKeepAliveProbeWithProject dispatches a 1-token keep-alive ping payload with a specified project.
func Send1TokenKeepAliveProbeWithProject(accessToken string, project string) error {
	if accessToken == "" {
		return fmt.Errorf("empty access token")
	}
	payload, err := Build1TokenKeepAliveJSONWithModelAndProject("gemini-3-flash", project)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	var lastErr error
	for _, endpoint := range CloudCodeGenerateContentURLs {
		req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "antigravity/2.19.1 linux/amd64")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
			return nil
		}
		lastErr = fmt.Errorf("endpoint %s returned status %d", endpoint, resp.StatusCode)
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("no generateContent endpoints reachable")
}

// IgniteAccountPostReset refreshes tokens if necessary and dispatches the 1-token probe.
func IgniteAccountPostReset(acc *keyring.Account, store *keyring.Store) error {
	if acc == nil {
		return fmt.Errorf("nil account")
	}
	token := acc.AccessToken
	if (token == "" || (!acc.TokenExpiry.IsZero() && time.Now().After(acc.TokenExpiry.Add(-2*time.Minute)))) && acc.RefreshToken != "" {
		newTok, newRefTok, err := RefreshGoogleTokenFull(acc.RefreshToken, "", "")
		if err == nil && newTok != "" {
			token = newTok
			acc.AccessToken = newTok
			if newRefTok != "" {
				acc.RefreshToken = newRefTok
			}
			acc.TokenExpiry = time.Now().Add(55 * time.Minute)
			if store != nil {
				_ = store.UpdateAccountTokensAndMetadata(acc.Email, acc.AccessToken, acc.RefreshToken, acc.PlanTier, acc.Credits)
			}
		}
	}
	if token == "" {
		return fmt.Errorf("no access token available for account %s", acc.Email)
	}
	return Send1TokenKeepAliveProbe(token)
}
