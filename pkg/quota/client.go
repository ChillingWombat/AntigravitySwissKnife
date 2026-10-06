package quota

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

const (
	GoogleTokenRefreshURL     = "https://oauth2.googleapis.com/token"
	DefaultGoogleClientID     = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"
	DefaultGoogleClientSecret = "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"
)

// Canonical plan tiers supported across Antigravity
const (
	PlanTierFree     = "Free"
	PlanTierPlus     = "Plus"
	PlanTierPro      = "Pro"
	PlanTierProTrial = "Pro - Trial"
	PlanTierEdu      = "Edu"
	PlanTierUltra5X  = "Ultra 5X"
	PlanTierUltra10X = "Ultra 10X"
	PlanTierUltra20X = "Ultra 20X"
)

// NormalizePlanTier maps any API response or legacy label into canonical tier representation.
func NormalizePlanTier(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return PlanTierFree
	}
	lower := strings.ToLower(trimmed)
	if lower == "free" {
		return PlanTierFree
	}
	if strings.Contains(lower, "trial") {
		return PlanTierProTrial
	}
	if strings.Contains(lower, "20x") {
		return PlanTierUltra20X
	}
	if strings.Contains(lower, "10x") {
		return PlanTierUltra10X
	}
	if strings.Contains(lower, "5x") {
		return PlanTierUltra5X
	}
	if strings.Contains(lower, "ultra") {
		return PlanTierUltra20X
	}
	if strings.Contains(lower, "edu") || strings.Contains(lower, "education") {
		return PlanTierEdu
	}
	if strings.Contains(lower, "plus") {
		return PlanTierPlus
	}
	if strings.Contains(lower, "pro") {
		return PlanTierPro
	}
	return trimmed
}


var (
	CloudCodeLoadProjectURLs = []string{
		"https://daily-cloudcode-pa.googleapis.com/v1internal:loadCodeAssist",
		"https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist",
		"https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:loadCodeAssist",
	}
	CloudCodeRetrieveQuotaURLs = []string{
		"https://daily-cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary",
		"https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:retrieveUserQuotaSummary",
		"https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary",
	}
	CloudCodeModelsURLs = []string{
		"https://daily-cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels",
		"https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:fetchAvailableModels",
		"https://cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels",
	}
)

// RefreshGoogleToken exchanges a refresh token for a valid access token.
func RefreshGoogleToken(refreshToken, clientID, clientSecret string) (string, error) {
	if refreshToken == "" {
		return "", fmt.Errorf("empty refresh token")
	}
	if clientID == "" {
		clientID = DefaultGoogleClientID
	}
	if clientSecret == "" {
		clientSecret = DefaultGoogleClientSecret
	}

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("refresh_token", refreshToken)
	form.Set("grant_type", "refresh_token")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm(GoogleTokenRefreshURL, form)
	if err != nil {
		return "", fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read refresh response: %w", err)
	}

	var res struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", fmt.Errorf("malformed refresh response: %w", err)
	}

	if res.Error != "" {
		return "", fmt.Errorf("token refresh error %s: %s", res.Error, res.ErrorDesc)
	}
	if res.AccessToken == "" {
		return "", fmt.Errorf("no access token in refresh response")
	}

	return res.AccessToken, nil
}

type ProjectContextResult struct {
	ProjectID string
	TierName  string
	Credits   float64
}

// FetchProjectAndTier calls loadCodeAssist across endpoints to discover project, subscription tier, and credits.
func FetchProjectAndTier(accessToken string) (*ProjectContextResult, error) {
	if accessToken == "" {
		return nil, fmt.Errorf("empty access token")
	}

	payload := []byte(`{"metadata":{"ideType":"ANTIGRAVITY"}}`)
	client := &http.Client{Timeout: 10 * time.Second}

	for _, endpoint := range CloudCodeLoadProjectURLs {
		req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "antigravity/2.19.1 linux/amd64")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		var res struct {
			CloudaicompanionProject string `json:"cloudaicompanionProject"`
			CurrentTier             *struct {
				ID               string `json:"id"`
				Name             string `json:"name"`
				AvailableCredits []struct {
					CreditType                  string      `json:"creditType"`
					CreditAmount                interface{} `json:"creditAmount"`
					MinimumCreditAmountForUsage interface{} `json:"minimumCreditAmountForUsage"`
				} `json:"availableCredits"`
			} `json:"currentTier"`
			PaidTier *struct {
				ID               string `json:"id"`
				Name             string `json:"name"`
				AvailableCredits []struct {
					CreditType                  string      `json:"creditType"`
					CreditAmount                interface{} `json:"creditAmount"`
					MinimumCreditAmountForUsage interface{} `json:"minimumCreditAmountForUsage"`
				} `json:"availableCredits"`
			} `json:"paidTier"`
			AllowedTiers []struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				IsDefault bool   `json:"isDefault"`
			} `json:"allowedTiers"`
		}

		if err := json.Unmarshal(body, &res); err == nil {
			result := &ProjectContextResult{
				ProjectID: res.CloudaicompanionProject,
			}
			if result.ProjectID == "" {
				result.ProjectID = "aicode-consumers"
			}

			// Determine tier
			var rawTier string
			if res.PaidTier != nil && res.PaidTier.Name != "" {
				rawTier = res.PaidTier.Name
			} else if res.PaidTier != nil && res.PaidTier.ID != "" {
				rawTier = res.PaidTier.ID
			} else if res.CurrentTier != nil && res.CurrentTier.Name != "" {
				rawTier = res.CurrentTier.Name
			} else if res.CurrentTier != nil && res.CurrentTier.ID != "" {
				rawTier = res.CurrentTier.ID
			} else if len(res.AllowedTiers) > 0 {
				rawTier = res.AllowedTiers[0].Name
			}

			if rawTier != "" {
				result.TierName = NormalizePlanTier(rawTier)
			} else {
				result.TierName = ""
			}

			// Extract credits
			parseCreditVal := func(v interface{}) float64 {
				if v == nil {
					return 0
				}
				if f, ok := v.(float64); ok {
					return f
				}
				if s, ok := v.(string); ok {
					if parsed, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
						return parsed
					}
				}
				return 0
			}

			var credAmount float64
			if res.PaidTier != nil && len(res.PaidTier.AvailableCredits) > 0 {
				cItem := res.PaidTier.AvailableCredits[0]
				credAmount = parseCreditVal(cItem.CreditAmount)
			}
			if credAmount == 0 && res.CurrentTier != nil && len(res.CurrentTier.AvailableCredits) > 0 {
				cItem := res.CurrentTier.AvailableCredits[0]
				credAmount = parseCreditVal(cItem.CreditAmount)
			}
			result.Credits = credAmount
			return result, nil
		}
	}

	return nil, fmt.Errorf("failed to fetch project context from all endpoints")
}

type LiveQuotaBreakdown struct {
	Quota5hGemini        float64
	QuotaWeeklyGemini    float64
	Quota5hClaudeGPT     float64
	QuotaWeeklyClaudeGPT float64
	ResetTime5h          time.Time
	ResetTimeWeekly      time.Time
	ResetHorizonText     string
	ResetSeconds5h       float64
	Models               []ModelQuota
}

// FetchLiveQuota queries Google CloudCode API for real account quota metrics.
func FetchLiveQuota(accessToken string, project string) ([]ModelQuota, error) {
	breakdown, err := FetchLiveQuotaBreakdown(accessToken, project)
	if err != nil {
		return nil, err
	}
	return breakdown.Models, nil
}

// FetchLiveQuotaBreakdown fetches complete quota buckets for Gemini and Claude/GPT models.
func FetchLiveQuotaBreakdown(accessToken string, project string) (*LiveQuotaBreakdown, error) {
	if accessToken == "" {
		return nil, fmt.Errorf("empty access token")
	}

	if project == "" {
		project = "aicode-consumers"
	}

	client := &http.Client{Timeout: 10 * time.Second}
	now := time.Now()

	for _, endpoint := range CloudCodeRetrieveQuotaURLs {
		for attempt := 0; attempt < 2; attempt++ {
			p := project
			if attempt == 1 {
				p = "" // retry without project if first fails
			}
			payloadBytes, _ := json.Marshal(map[string]string{"project": p})
			req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payloadBytes))
			if err != nil {
				continue
			}
			req.Header.Set("Authorization", "Bearer "+accessToken)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "antigravity/2.19.1 linux/amd64")

			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode == http.StatusForbidden && attempt == 0 {
				continue // retry without project
			}
			if resp.StatusCode != http.StatusOK {
				continue
			}

			var res struct {
				Groups []struct {
					DisplayName string `json:"displayName"`
					Description string `json:"description"`
					Buckets     []struct {
						BucketID          string  `json:"bucketId"`
						DisplayName       string  `json:"displayName"`
						Window            string  `json:"window"`
						RemainingFraction float64 `json:"remainingFraction"`
						ResetTime         string  `json:"resetTime"`
					} `json:"buckets"`
				} `json:"groups"`
				Buckets []struct {
					ModelID           string  `json:"modelId"`
					RemainingFraction float64 `json:"remainingFraction"`
					ResetTime         string  `json:"resetTime"`
				} `json:"buckets"`
			}

			if err := json.Unmarshal(body, &res); err != nil {
				continue
			}

			breakdown := &LiveQuotaBreakdown{
				Quota5hGemini:        1.0,
				QuotaWeeklyGemini:    1.0,
				Quota5hClaudeGPT:     1.0,
				QuotaWeeklyClaudeGPT: 1.0,
				ResetHorizonText:     "Ready",
			}

			if len(res.Groups) > 0 {
				for _, g := range res.Groups {
					gName := strings.ToLower(g.DisplayName)
					for _, b := range g.Buckets {
						w := strings.ToLower(b.Window)
						bid := strings.ToLower(b.BucketID)
						var rTime time.Time
						if b.ResetTime != "" {
							rTime, _ = time.Parse(time.RFC3339, b.ResetTime)
						}

						modelItem := ModelQuota{
							ModelName:    b.DisplayName,
							Fraction:     b.RemainingFraction,
							ResetTime:    rTime,
							ResetText:    FormatResetHorizon(rTime, now),
							HealthStatus: ComputeHealth(b.RemainingFraction),
						}
						if modelItem.ModelName == "" {
							modelItem.ModelName = b.BucketID
						}
						breakdown.Models = append(breakdown.Models, modelItem)

						if strings.Contains(gName, "gemini") {
							if w == "5h" || strings.Contains(bid, "5h") {
								breakdown.Quota5hGemini = b.RemainingFraction
								breakdown.ResetTime5h = rTime
								if !rTime.IsZero() && rTime.After(now) {
									breakdown.ResetSeconds5h = rTime.Sub(now).Seconds()
									breakdown.ResetHorizonText = FormatResetHorizon(rTime, now)
								}
							} else if w == "weekly" || strings.Contains(bid, "weekly") {
								breakdown.QuotaWeeklyGemini = b.RemainingFraction
								breakdown.ResetTimeWeekly = rTime
							}
						} else if strings.Contains(gName, "claude") || strings.Contains(gName, "gpt") || strings.Contains(bid, "3p") {
							if w == "5h" || strings.Contains(bid, "5h") {
								breakdown.Quota5hClaudeGPT = b.RemainingFraction
							} else if w == "weekly" || strings.Contains(bid, "weekly") {
								breakdown.QuotaWeeklyClaudeGPT = b.RemainingFraction
							}
						}
					}
				}
				return breakdown, nil
			}

			// Fallback: flat buckets
			if len(res.Buckets) > 0 {
				for _, b := range res.Buckets {
					var rTime time.Time
					if b.ResetTime != "" {
						rTime, _ = time.Parse(time.RFC3339, b.ResetTime)
					}
					breakdown.Models = append(breakdown.Models, ModelQuota{
						ModelName:    b.ModelID,
						Fraction:     b.RemainingFraction,
						ResetTime:    rTime,
						ResetText:    FormatResetHorizon(rTime, now),
						HealthStatus: ComputeHealth(b.RemainingFraction),
					})
					name := strings.ToLower(b.ModelID)
					if strings.Contains(name, "weekly") {
						breakdown.QuotaWeeklyGemini = b.RemainingFraction
					} else {
						breakdown.Quota5hGemini = b.RemainingFraction
						if !rTime.IsZero() && rTime.After(now) {
							breakdown.ResetSeconds5h = rTime.Sub(now).Seconds()
							breakdown.ResetHorizonText = FormatResetHorizon(rTime, now)
						}
					}
				}
				return breakdown, nil
			}
		}
	}

	return nil, fmt.Errorf("upstream quota check failed across all endpoints")
}

// PollAccountLiveQuota queries Google CloudCode for real quota, tier, and credits, refreshing tokens if needed.
func PollAccountLiveQuota(acc *keyring.Account) (*QuotaSummary, error) {
	if acc == nil {
		return nil, fmt.Errorf("nil account")
	}

	now := time.Now()
	accessToken := acc.AccessToken
	project := "aicode-consumers"

	// Step 1: Discover Project Context & Membership Tier & Credits
	pCtx, err := FetchProjectAndTier(accessToken)
	if (err != nil || accessToken == "") && acc.RefreshToken != "" {
		// Attempt token refresh
		newTok, refErr := RefreshGoogleToken(acc.RefreshToken, "", "")
		if refErr == nil && newTok != "" {
			acc.AccessToken = newTok
			accessToken = newTok
			pCtx, err = FetchProjectAndTier(accessToken)
		}
	}

	tier := acc.PlanTier
	if tier == "" {
		tier = PlanTierPro
	}
	tier = NormalizePlanTier(tier)
	credits := acc.Credits
	if pCtx != nil {
		if pCtx.ProjectID != "" {
			project = pCtx.ProjectID
		}
		if pCtx.TierName != "" {
			tier = NormalizePlanTier(pCtx.TierName)
			acc.PlanTier = tier
		} else {
			acc.PlanTier = tier
		}
		credits = pCtx.Credits
		acc.Credits = credits
	}

	// Step 2: Query Live Quota Summary
	breakdown, qErr := FetchLiveQuotaBreakdown(accessToken, project)
	if qErr != nil && acc.RefreshToken != "" && accessToken != "" {
		// Try refreshing token once if quota check failed
		newTok, refErr := RefreshGoogleToken(acc.RefreshToken, "", "")
		if refErr == nil && newTok != "" {
			acc.AccessToken = newTok
			accessToken = newTok
			breakdown, qErr = FetchLiveQuotaBreakdown(accessToken, project)
		}
	}

	if breakdown != nil {
		minFrac := breakdown.Quota5hGemini
		if breakdown.QuotaWeeklyGemini < minFrac {
			minFrac = breakdown.QuotaWeeklyGemini
		}

		summary := &QuotaSummary{
			AccountEmail:         acc.Email,
			PlanTier:             tier,
			Credits:              credits,
			Quota5hFraction:      breakdown.Quota5hGemini,
			QuotaWeeklyFraction:  breakdown.QuotaWeeklyGemini,
			Quota5hClaudeGPT:     breakdown.Quota5hClaudeGPT,
			QuotaWeeklyClaudeGPT: breakdown.QuotaWeeklyClaudeGPT,
			ResetSeconds5h:       breakdown.ResetSeconds5h,
			ResetHorizonText:     breakdown.ResetHorizonText,
			Models:               breakdown.Models,
			MinFraction:          minFrac,
			OverallHealth:        ComputeHealth(minFrac),
			LastPolled:           now,
		}
		return summary, nil
	}

	// Fallback to cached cloud_accounts.db metrics if network query fails
	if cachedAccs, cErr := keyring.ReadCloudAccountsDB(""); cErr == nil {
		for _, ca := range cachedAccs {
			if strings.EqualFold(ca.Email, acc.Email) {
				resetText := "Ready"
				var resetSec float64
				if ca.ResetTime5h != "" {
					if rt, parseErr := time.Parse(time.RFC3339, ca.ResetTime5h); parseErr == nil && rt.After(now) {
						resetSec = rt.Sub(now).Seconds()
						resetText = FormatResetHorizon(rt, now)
					}
				}
				pTier := ca.PlanTier
				if pTier == "" {
					pTier = acc.PlanTier
				}
				if pTier == "" {
					pTier = PlanTierPro
				}
				pTier = NormalizePlanTier(pTier)
				acc.PlanTier = pTier
				cAmount := ca.Credits
				acc.Credits = cAmount

				minFrac := ca.Quota5h
				if ca.QuotaWeekly < minFrac {
					minFrac = ca.QuotaWeekly
				}

				return &QuotaSummary{
					AccountEmail:         acc.Email,
					PlanTier:             pTier,
					Credits:              cAmount,
					Quota5hFraction:      ca.Quota5h,
					QuotaWeeklyFraction:  ca.QuotaWeekly,
					Quota5hClaudeGPT:     ca.Quota5hClaudeGPT,
					QuotaWeeklyClaudeGPT: ca.QuotaWeeklyClaudeGPT,
					ResetSeconds5h:       resetSec,
					ResetHorizonText:     resetText,
					Models:               []ModelQuota{},
					MinFraction:          minFrac,
					OverallHealth:        ComputeHealth(minFrac),
					LastPolled:           now,
				}, nil
			}
		}
	}

	// Baseline fallback
	return &QuotaSummary{
		AccountEmail:     acc.Email,
		PlanTier:         tier,
		Credits:          credits,
		MinFraction:      0.0,
		OverallHealth:    core.StatusExhausted,
		ResetHorizonText: "Not Polled",
		LastPolled:       now,
	}, qErr
}
