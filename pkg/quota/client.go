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
	PlanTierFree       = "Free"
	PlanTierPlus       = "Plus"
	PlanTierPro        = "Pro"
	PlanTierProTrial   = "Pro - Trial"
	PlanTierEdu        = "Edu"
	PlanTierUltra5X    = "Ultra 5X"
	PlanTierUltra10X   = "Ultra 10X"
	PlanTierUltra20X   = "Ultra 20X"
	PlanTierEnterprise = "Enterprise"
)

// IsTrialWarningText checks if any warning or tooltip message matches known trial / promotional restriction notices.
// Specifically detects notices such as:
// "Sonnet 5.5 is now available on paid Pro and Ultra plans. Third-party model access will no longer be available on your current plan starting on November 2, 2026."
func IsTrialWarningText(text string) bool {
	if text == "" {
		return false
	}
	low := strings.ToLower(text)
	if strings.Contains(low, "third-party model access will no longer be available on your current plan") ||
		strings.Contains(low, "sonnet 5.5 is now available on paid pro and ultra plans") ||
		strings.Contains(low, "paid pro and ultra plans") ||
		strings.Contains(low, "will no longer be available on your current plan") ||
		(strings.Contains(low, "third-party model access") && (strings.Contains(low, "current plan") || strings.Contains(low, "november 2"))) ||
		strings.Contains(low, "current plan starting on november 2, 2026") ||
		strings.Contains(low, "starter quota") ||
		strings.Contains(low, "trial") ||
		strings.Contains(low, "promo") ||
		strings.Contains(low, "partner offer") ||
		strings.Contains(low, "jio") {
		return true
	}
	return false
}

// NormalizePlanTier maps any API response or legacy label into canonical tier representation.
func NormalizePlanTier(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return PlanTierFree
	}
	lower := strings.ToLower(trimmed)
	if lower == "free" || lower == "free-tier" || lower == "tier_free" || lower == "starter" || lower == "starter-tier" || lower == "starter quota" {
		return PlanTierFree
	}
	if strings.Contains(lower, "trial") ||
		strings.Contains(lower, "promo") ||
		strings.Contains(lower, "google ai pro") ||
		strings.Contains(lower, "starter pro") ||
		strings.Contains(lower, "jio") ||
		strings.Contains(lower, "partner") ||
		strings.Contains(lower, "bundle") ||
		IsTrialWarningText(lower) {
		return PlanTierProTrial
	}
	if strings.Contains(lower, "20x") || strings.Contains(lower, "ultra_20x") || strings.Contains(lower, "ultra 20x") {
		return PlanTierUltra20X
	}
	if strings.Contains(lower, "10x") || strings.Contains(lower, "ultra_10x") || strings.Contains(lower, "ultra 10x") {
		return PlanTierUltra10X
	}
	if strings.Contains(lower, "5x") || strings.Contains(lower, "ultra_5x") || strings.Contains(lower, "ultra 5x") {
		return PlanTierUltra5X
	}
	if strings.Contains(lower, "ultra") {
		return PlanTierUltra20X
	}
	if strings.Contains(lower, "edu") || strings.Contains(lower, "education") || strings.Contains(lower, "student") || strings.Contains(lower, "academic") {
		return PlanTierEdu
	}
	if strings.Contains(lower, "enterprise") || strings.Contains(lower, "teams_tier_enterprise") {
		return PlanTierEnterprise
	}
	if strings.Contains(lower, "plus") {
		return PlanTierPlus
	}
	if strings.Contains(lower, "pro") || strings.Contains(lower, "standard") || strings.Contains(lower, "code assist") || strings.Contains(lower, "ai premium") || strings.Contains(lower, "g1_ai") || strings.Contains(lower, "team") {
		return PlanTierPro
	}
	return trimmed
}

var (
	CloudCodeLoadProjectURLs = []string{
		"https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist",
		"https://daily-cloudcode-pa.googleapis.com/v1internal:loadCodeAssist",
		"https://cloudcodeassist-pa.googleapis.com/v1internal:loadCodeAssist",
		"https://cloudaicompanion.googleapis.com/v1internal:loadCodeAssist",
		"https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:loadCodeAssist",
	}
	CloudCodeRetrieveQuotaURLs = []string{
		"https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary",
		"https://daily-cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary",
		"https://cloudcodeassist-pa.googleapis.com/v1internal:retrieveUserQuotaSummary",
		"https://cloudaicompanion.googleapis.com/v1internal:retrieveUserQuotaSummary",
		"https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:retrieveUserQuotaSummary",
	}
	CloudCodeModelsURLs = []string{
		"https://cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels",
		"https://daily-cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels",
		"https://cloudcodeassist-pa.googleapis.com/v1internal:fetchAvailableModels",
		"https://cloudaicompanion.googleapis.com/v1internal:fetchAvailableModels",
		"https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:fetchAvailableModels",
	}
	GoogleUserInfoURLs = []string{
		"https://www.googleapis.com/oauth2/v3/userinfo",
		"https://openidconnect.googleapis.com/v1/userinfo",
	}
)

// RefreshGoogleTokenFull exchanges a refresh token for a valid access token and optional rotated refresh token.
func RefreshGoogleTokenFull(refreshToken, clientID, clientSecret string) (string, string, error) {
	if refreshToken == "" {
		return "", "", fmt.Errorf("empty refresh token")
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
		return "", "", fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read refresh response: %w", err)
	}

	var res struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", "", fmt.Errorf("malformed refresh response: %w", err)
	}

	if res.Error != "" {
		return "", "", fmt.Errorf("token refresh error %s: %s", res.Error, res.ErrorDesc)
	}
	if res.AccessToken == "" {
		return "", "", fmt.Errorf("no access token in refresh response")
	}

	return res.AccessToken, res.RefreshToken, nil
}

// RefreshGoogleToken exchanges a refresh token for a valid access token.
func RefreshGoogleToken(refreshToken, clientID, clientSecret string) (string, error) {
	acc, _, err := RefreshGoogleTokenFull(refreshToken, clientID, clientSecret)
	return acc, err
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

		type rawTierItem struct {
			ID               string      `json:"id"`
			TierID           string      `json:"tierId"`
			TierIDSnake      string      `json:"tier_id"`
			Tier             string      `json:"tier"`
			TierType         string      `json:"tierType"`
			TierTypeSnake    string      `json:"tier_type"`
			Name             string      `json:"name"`
			TierName         string      `json:"tierName"`
			TierNameSnake    string      `json:"tier_name"`
			DisplayName      string      `json:"displayName"`
			DisplayNameSnake string      `json:"display_name"`
			Description      string      `json:"description"`
			IsTrial          bool        `json:"isTrial"`
			IsTrialSnake     bool        `json:"is_trial"`
			TrialStatus      string      `json:"trialStatus"`
			TrialStatusSnake string      `json:"trial_status"`
			AvailableCredits []struct {
				CreditType                  string      `json:"creditType"`
				CreditAmount                interface{} `json:"creditAmount"`
				MinimumCreditAmountForUsage interface{} `json:"minimumCreditAmountForUsage"`
			} `json:"availableCredits"`
			AvailableCreditsSnake []struct {
				CreditType                  string      `json:"credit_type"`
				CreditAmount                interface{} `json:"credit_amount"`
				MinimumCreditAmountForUsage interface{} `json:"minimum_credit_amount_for_usage"`
			} `json:"available_credits"`
		}

		var res struct {
			CloudaicompanionProject      string        `json:"cloudaicompanionProject"`
			CloudaicompanionProjectSnake string        `json:"cloudaicompanion_project"`
			CurrentTier                  *rawTierItem  `json:"currentTier"`
			CurrentTierSnake             *rawTierItem  `json:"current_tier"`
			PaidTier                     *rawTierItem  `json:"paidTier"`
			PaidTierSnake                *rawTierItem  `json:"paid_tier"`
			UserTier                     *rawTierItem  `json:"userTier"`
			UserTierSnake                *rawTierItem  `json:"user_tier"`
			SubscriptionTier             string        `json:"subscriptionTier"`
			SubscriptionTierSnake        string        `json:"subscription_tier"`
			PlanTier                     string        `json:"planTier"`
			PlanTierSnake                string        `json:"plan_tier"`
			PlanName                     string        `json:"planName"`
			PlanNameSnake                string        `json:"plan_name"`
			Tier                         string        `json:"tier"`
			TierType                     string        `json:"tierType"`
			TierTypeSnake                string        `json:"tier_type"`
			IsTrial                      bool          `json:"isTrial"`
			IsTrialSnake                 bool          `json:"is_trial"`
			TrialStatus                  string        `json:"trialStatus"`
			TrialStatusSnake             string        `json:"trial_status"`
			WarningMessage               string        `json:"warningMessage"`
			WarningMessageSnake          string        `json:"warning_message"`
			Notice                       string        `json:"notice"`
			AllowedTiers                 []rawTierItem `json:"allowedTiers"`
			AllowedTiersSnake            []rawTierItem `json:"allowed_tiers"`
			IneligibleTiers              []rawTierItem `json:"ineligibleTiers"`
			IneligibleTiersSnake         []rawTierItem `json:"ineligible_tiers"`
		}

		if err := json.Unmarshal(body, &res); err == nil {
			projectID := res.CloudaicompanionProject
			if projectID == "" {
				projectID = res.CloudaicompanionProjectSnake
			}
			if projectID == "" {
				projectID = "aicode-consumers"
			}
			result := &ProjectContextResult{
				ProjectID: projectID,
			}

			// Helper to check if item indicates a trial
			isItemTrial := func(item *rawTierItem) bool {
				if item == nil {
					return false
				}
				if item.IsTrial || item.IsTrialSnake {
					return true
				}
				tStat := strings.ToLower(item.TrialStatus + " " + item.TrialStatusSnake)
				if strings.Contains(tStat, "trial") || strings.Contains(tStat, "promo") || (strings.Contains(tStat, "active") && (strings.Contains(strings.ToLower(item.Name+" "+item.DisplayName+" "+item.Description), "trial") || strings.Contains(strings.ToLower(item.Name+" "+item.DisplayName+" "+item.Description), "promo"))) {
					return true
				}
				if IsTrialWarningText(item.Description) || IsTrialWarningText(item.DisplayName) || IsTrialWarningText(item.Name) {
					return true
				}
				return false
			}

			// Helper to extract tier name or ID from raw item
			getItemTier := func(item *rawTierItem) string {
				if item == nil {
					return ""
				}
				if isItemTrial(item) {
					return PlanTierProTrial
				}
				if item.Tier != "" {
					return item.Tier
				}
				if item.TierType != "" {
					return item.TierType
				}
				if item.TierTypeSnake != "" {
					return item.TierTypeSnake
				}
				if item.Name != "" {
					return item.Name
				}
				if item.TierName != "" {
					return item.TierName
				}
				if item.TierNameSnake != "" {
					return item.TierNameSnake
				}
				if item.DisplayName != "" {
					return item.DisplayName
				}
				if item.DisplayNameSnake != "" {
					return item.DisplayNameSnake
				}
				if item.ID != "" {
					return item.ID
				}
				if item.TierID != "" {
					return item.TierID
				}
				if item.TierIDSnake != "" {
					return item.TierIDSnake
				}
				return ""
			}

			// 1. Direct explicit paid / current / user / subscription tier
			isRootTrial := res.IsTrial || res.IsTrialSnake ||
				strings.Contains(strings.ToLower(res.TrialStatus+" "+res.TrialStatusSnake), "trial") ||
				strings.Contains(strings.ToLower(res.TrialStatus+" "+res.TrialStatusSnake), "promo") ||
				(strings.EqualFold(res.TrialStatus, "active") && (strings.Contains(strings.ToLower(res.PlanName+" "+res.PlanTier), "trial") || strings.Contains(strings.ToLower(res.PlanName+" "+res.PlanTier), "promo"))) ||
				IsTrialWarningText(res.WarningMessage) || IsTrialWarningText(res.WarningMessageSnake) ||
				IsTrialWarningText(res.Notice) ||
				IsTrialWarningText(res.PlanName) || IsTrialWarningText(res.PlanNameSnake) ||
				IsTrialWarningText(res.PlanTier) || IsTrialWarningText(res.PlanTierSnake) ||
				isItemTrial(res.CurrentTier) || isItemTrial(res.CurrentTierSnake) ||
				isItemTrial(res.PaidTier) || isItemTrial(res.PaidTierSnake) ||
				isItemTrial(res.UserTier) || isItemTrial(res.UserTierSnake)

			var rawTier string
			if isRootTrial {
				rawTier = PlanTierProTrial
			} else if t := getItemTier(res.PaidTier); t != "" {
				rawTier = t
			} else if t := getItemTier(res.PaidTierSnake); t != "" {
				rawTier = t
			} else if t := getItemTier(res.CurrentTier); t != "" {
				rawTier = t
			} else if t := getItemTier(res.CurrentTierSnake); t != "" {
				rawTier = t
			} else if t := getItemTier(res.UserTier); t != "" {
				rawTier = t
			} else if t := getItemTier(res.UserTierSnake); t != "" {
				rawTier = t
			} else if res.Tier != "" {
				rawTier = res.Tier
			} else if res.TierType != "" {
				rawTier = res.TierType
			} else if res.TierTypeSnake != "" {
				rawTier = res.TierTypeSnake
			} else if res.SubscriptionTier != "" {
				rawTier = res.SubscriptionTier
			} else if res.SubscriptionTierSnake != "" {
				rawTier = res.SubscriptionTierSnake
			} else if res.PlanTier != "" {
				rawTier = res.PlanTier
			} else if res.PlanTierSnake != "" {
				rawTier = res.PlanTierSnake
			} else if res.PlanName != "" {
				rawTier = res.PlanName
			} else if res.PlanNameSnake != "" {
				rawTier = res.PlanNameSnake
			}

			if isRootTrial && (rawTier == "" || NormalizePlanTier(rawTier) == PlanTierPro) {
				rawTier = PlanTierProTrial
			}

			// 2. Allowed tiers inspection
			allAllowed := append(res.AllowedTiers, res.AllowedTiersSnake...)
			if rawTier == "" && len(allAllowed) > 0 {
				bestTier := ""
				for _, at := range allAllowed {
					tStr := getItemTier(&at)
					norm := NormalizePlanTier(tStr)
					if norm == PlanTierUltra20X || norm == PlanTierUltra10X || norm == PlanTierUltra5X {
						bestTier = norm
						break
					}
					if norm == PlanTierEnterprise {
						bestTier = PlanTierEnterprise
					} else if norm == PlanTierEdu && bestTier != PlanTierEnterprise {
						bestTier = PlanTierEdu
					} else if (norm == PlanTierPro || norm == PlanTierProTrial) && bestTier == "" {
						bestTier = norm
					} else if norm == PlanTierPlus && bestTier == "" {
						bestTier = PlanTierPlus
					} else if bestTier == "" {
						bestTier = norm
					}
				}
				rawTier = bestTier
			}

			// 3. Ineligible tiers check (if free-tier is marked ineligible because user is on standard/paid client)
			allIneligible := append(res.IneligibleTiers, res.IneligibleTiersSnake...)
			if rawTier == "" || rawTier == PlanTierFree {
				for _, it := range allIneligible {
					tStr := strings.ToLower(getItemTier(&it))
					if strings.Contains(tStr, "free") {
						// Free tier is unsupported/ineligible, account has Code Assist / Pro access
						if isRootTrial {
							rawTier = PlanTierProTrial
						} else {
							rawTier = PlanTierPro
						}
						break
					}
				}
			}

			if isRootTrial && (rawTier == "" || NormalizePlanTier(rawTier) == PlanTierPro) {
				rawTier = PlanTierProTrial
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

			extractCredits := func(item *rawTierItem) float64 {
				if item == nil {
					return 0
				}
				for _, c := range item.AvailableCredits {
					if val := parseCreditVal(c.CreditAmount); val > 0 {
						return val
					}
				}
				for _, c := range item.AvailableCreditsSnake {
					if val := parseCreditVal(c.CreditAmount); val > 0 {
						return val
					}
				}
				return 0
			}

			var credAmount float64
			if c := extractCredits(res.PaidTier); c > 0 {
				credAmount = c
			} else if c := extractCredits(res.PaidTierSnake); c > 0 {
				credAmount = c
			} else if c := extractCredits(res.CurrentTier); c > 0 {
				credAmount = c
			} else if c := extractCredits(res.CurrentTierSnake); c > 0 {
				credAmount = c
			}
			result.Credits = credAmount
			return result, nil
		}
	}

	return nil, fmt.Errorf("failed to fetch project context from all endpoints")
}

type LiveQuotaBreakdown struct {
	Quota5hGemini          float64
	QuotaWeeklyGemini      float64
	Quota5hClaudeGPT       float64
	QuotaWeeklyClaudeGPT   float64
	SubscriptionTier       string
	Credits                float64
	HasClaudeGPTRights     bool
	ResetTime5h            time.Time
	ResetTimeWeekly        time.Time
	ResetHorizonText       string
	ResetSeconds5h         float64
	ResetHorizonWeeklyText string
	ResetSecondsWeekly     float64
	Models                 []ModelQuota
}

// FetchLiveQuota queries Google CloudCode API for real account quota metrics.
func FetchLiveQuota(accessToken string, project string) ([]ModelQuota, error) {
	breakdown, err := FetchLiveQuotaBreakdown(accessToken, project)
	if err != nil {
		return nil, err
	}
	return breakdown.Models, nil
}

// QuotaAPIError represents an explicit structured error returned by Google CloudCode API.
type QuotaAPIError struct {
	StatusCode int
	Status     string // "ERROR" or "BANNED"
	Reason     string
	Message    string
	ActionURL  string
}

func (e *QuotaAPIError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("%s (%s)", e.Message, e.Reason)
	}
	return e.Message
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
	var lastAPIError *QuotaAPIError

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

			if resp.StatusCode != http.StatusOK {
				var errPayload struct {
					Error struct {
						Code    int    `json:"code"`
						Message string `json:"message"`
						Status  string `json:"status"`
						Details []struct {
							Reason   string            `json:"reason"`
							Metadata map[string]string `json:"metadata"`
						} `json:"details"`
					} `json:"error"`
				}
				if jsonErr := json.Unmarshal(body, &errPayload); jsonErr == nil && errPayload.Error.Code > 0 {
					msg := strings.TrimSpace(errPayload.Error.Message)
					lowMsg := strings.ToLower(msg)
					reason := ""
					actionURL := ""
					for _, d := range errPayload.Error.Details {
						if d.Reason != "" {
							reason = d.Reason
						}
						if d.Metadata != nil && d.Metadata["validation_url"] != "" {
							actionURL = d.Metadata["validation_url"]
						}
					}
					errStatus := "ERROR"
					if strings.Contains(lowMsg, "suspended") || strings.Contains(lowMsg, "disabled") || strings.Contains(lowMsg, "banned") || strings.Contains(strings.ToLower(reason), "suspended") {
						errStatus = "BANNED"
					} else if reason == "VALIDATION_REQUIRED" || strings.Contains(lowMsg, "verify your account") {
						errStatus = "ERROR"
						if msg == "" {
							msg = "Verify your account to continue."
						}
					} else if resp.StatusCode == http.StatusUnauthorized || errPayload.Error.Status == "UNAUTHENTICATED" {
						errStatus = "ERROR"
						if msg == "" {
							msg = "Session expired or unauthenticated."
						}
					}
					lastAPIError = &QuotaAPIError{
						StatusCode: resp.StatusCode,
						Status:     errStatus,
						Reason:     reason,
						Message:    msg,
						ActionURL:  actionURL,
					}
				}
				if resp.StatusCode == http.StatusForbidden && attempt == 0 {
					continue // retry without project
				}
				continue
			}

			var res struct {
				SubscriptionTier      string `json:"subscriptionTier"`
				SubscriptionTierSnake string `json:"subscription_tier"`
				UserTier              string `json:"userTier"`
				UserTierSnake         string `json:"user_tier"`
				PlanTier              string `json:"planTier"`
				PlanTierSnake         string `json:"plan_tier"`
				IsTrial               bool   `json:"isTrial"`
				IsTrialSnake          bool   `json:"is_trial"`
				TrialStatus           string `json:"trialStatus"`
				TrialStatusSnake      string `json:"trial_status"`
				WarningMessage        string `json:"warningMessage"`
				WarningMessageSnake   string `json:"warning_message"`
				Notice                string `json:"notice"`
				AICredits             *struct {
					Credits interface{} `json:"credits"`
				} `json:"aiCredits"`
				AICreditsSnake *struct {
					Credits interface{} `json:"credits"`
				} `json:"ai_credits"`
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

			subTier := res.SubscriptionTier
			if subTier == "" {
				subTier = res.SubscriptionTierSnake
			}
			if subTier == "" {
				subTier = res.UserTier
			}
			if subTier == "" {
				subTier = res.UserTierSnake
			}
			if subTier == "" {
				subTier = res.PlanTier
			}
			if subTier == "" {
				subTier = res.PlanTierSnake
			}

			isSummaryTrial := res.IsTrial || res.IsTrialSnake ||
				strings.Contains(strings.ToLower(res.TrialStatus+" "+res.TrialStatusSnake), "trial") ||
				strings.Contains(strings.ToLower(res.TrialStatus+" "+res.TrialStatusSnake), "promo") ||
				IsTrialWarningText(res.WarningMessage) || IsTrialWarningText(res.WarningMessageSnake) ||
				IsTrialWarningText(res.Notice) ||
				IsTrialWarningText(subTier)

			if len(res.Groups) > 0 {
				for _, g := range res.Groups {
					if IsTrialWarningText(g.DisplayName) || IsTrialWarningText(g.Description) {
						isSummaryTrial = true
					}
					for _, b := range g.Buckets {
						if IsTrialWarningText(b.DisplayName) {
							isSummaryTrial = true
						}
					}
				}
			}

			if isSummaryTrial && (subTier == "" || NormalizePlanTier(subTier) == PlanTierPro) {
				subTier = PlanTierProTrial
			}

			var credAmount float64
			parseCredit := func(v interface{}) float64 {
				if v == nil {
					return 0
				}
				if f, ok := v.(float64); ok {
					return f
				}
				if s, ok := v.(string); ok {
					if pf, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
						return pf
					}
				}
				return 0
			}
			if res.AICredits != nil {
				credAmount = parseCredit(res.AICredits.Credits)
			} else if res.AICreditsSnake != nil {
				credAmount = parseCredit(res.AICreditsSnake.Credits)
			}

			breakdown := &LiveQuotaBreakdown{
				Quota5hGemini:          1.0,
				QuotaWeeklyGemini:      1.0,
				Quota5hClaudeGPT:       1.0,
				QuotaWeeklyClaudeGPT:   1.0,
				ResetHorizonText:       "Ready",
				ResetHorizonWeeklyText: "Ready",
				SubscriptionTier:       NormalizePlanTier(subTier),
				Credits:                credAmount,
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
								if !rTime.IsZero() && rTime.After(now) {
									breakdown.ResetSecondsWeekly = rTime.Sub(now).Seconds()
									breakdown.ResetHorizonWeeklyText = FormatResetHorizon(rTime, now)
								}
							}
						} else if strings.Contains(gName, "claude") || strings.Contains(gName, "gpt") || strings.Contains(bid, "3p") {
							breakdown.HasClaudeGPTRights = true
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
						breakdown.ResetTimeWeekly = rTime
						if !rTime.IsZero() && rTime.After(now) {
							breakdown.ResetSecondsWeekly = rTime.Sub(now).Seconds()
							breakdown.ResetHorizonWeeklyText = FormatResetHorizon(rTime, now)
						}
					} else {
						breakdown.Quota5hGemini = b.RemainingFraction
						breakdown.ResetTime5h = rTime
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

	if lastAPIError != nil {
		return nil, lastAPIError
	}
	return nil, fmt.Errorf("upstream quota check failed across all endpoints")
}

// DetectTierFromAvailableModels checks model permissions to infer account tier class.
func DetectTierFromAvailableModels(accessToken string) (string, error) {
	if accessToken == "" {
		return "", fmt.Errorf("empty access token")
	}
	client := &http.Client{Timeout: 6 * time.Second}
	payload := []byte(`{"project":""}`)

	for _, endpoint := range CloudCodeModelsURLs {
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

		var data struct {
			TieredModelIDs struct {
				FlashLite  []string `json:"flashLite"`
				Flash      []string `json:"flash"`
				Pro        []string `json:"pro"`
				Ultra      []string `json:"ultra"`
				Enterprise []string `json:"enterprise"`
			} `json:"tieredModelIds"`
			TieredModelIDsSnake struct {
				FlashLite  []string `json:"flash_lite"`
				Flash      []string `json:"flash"`
				Pro        []string `json:"pro"`
				Ultra      []string `json:"ultra"`
				Enterprise []string `json:"enterprise"`
			} `json:"tiered_model_ids"`
			Models map[string]struct {
				DisplayName string `json:"displayName"`
			} `json:"models"`
		}
		if err := json.Unmarshal(body, &data); err == nil {
			ultraModels := append(data.TieredModelIDs.Ultra, data.TieredModelIDsSnake.Ultra...)
			proModels := append(data.TieredModelIDs.Pro, data.TieredModelIDsSnake.Pro...)
			entModels := append(data.TieredModelIDs.Enterprise, data.TieredModelIDsSnake.Enterprise...)

			hasUltra := len(ultraModels) > 0
			hasEnterprise := len(entModels) > 0
			hasPro := len(proModels) > 0
			hasThirdParty := false

			for mID, mMeta := range data.Models {
				low := strings.ToLower(mID + " " + mMeta.DisplayName)
				if strings.Contains(low, "ultra") {
					hasUltra = true
				}
				if strings.Contains(low, "enterprise") {
					hasEnterprise = true
				}
				if strings.Contains(low, "claude") || strings.Contains(low, "gpt") {
					hasThirdParty = true
				}
				if strings.Contains(low, "-pro") {
					hasPro = true
				}
			}

			if hasUltra {
				return PlanTierUltra20X, nil
			}
			if hasEnterprise {
				return PlanTierEnterprise, nil
			}
			if hasPro {
				if hasThirdParty {
					return PlanTierPro, nil
				}
				return PlanTierProTrial, nil
			}
			if len(data.TieredModelIDs.Flash) > 0 || len(data.TieredModelIDs.FlashLite) > 0 {
				return PlanTierFree, nil
			}
		}
	}
	return "", fmt.Errorf("could not detect tier from models")
}

// DetectTierFromGoogleUserInfo queries Google OAuth userinfo to identify institutional or educational accounts.
func DetectTierFromGoogleUserInfo(accessToken, email string) (string, error) {
	if accessToken == "" {
		return "", fmt.Errorf("empty access token")
	}
	client := &http.Client{Timeout: 6 * time.Second}

	for _, endpoint := range GoogleUserInfoURLs {
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
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

		var info struct {
			Email string `json:"email"`
			HD    string `json:"hd"`
		}
		if err := json.Unmarshal(body, &info); err == nil {
			hd := strings.ToLower(info.HD)
			em := strings.ToLower(info.Email)
			if em == "" {
				em = strings.ToLower(email)
			}
			if strings.HasSuffix(hd, ".edu") || strings.Contains(hd, "edu.") || strings.HasSuffix(em, ".edu") {
				return PlanTierEdu, nil
			}
			if hd != "" && !strings.Contains(hd, "gmail") && !strings.Contains(hd, "googlemail") {
				return PlanTierEnterprise, nil
			}
		}
	}
	return "", fmt.Errorf("could not detect tier from userinfo")
}

// PollAccountLiveQuota queries Google CloudCode for real quota, tier, and credits, refreshing tokens if needed.
func PollAccountLiveQuota(acc *keyring.Account) (*QuotaSummary, error) {
	if acc == nil {
		return nil, fmt.Errorf("nil account")
	}

	now := time.Now()
	accessToken := acc.AccessToken
	project := "aicode-consumers"

	var lastTokenRefreshErr error

	// Step 1: Discover Project Context & Membership Tier & Credits from loadCodeAssist
	pCtx, err := FetchProjectAndTier(accessToken)
	if (err != nil || accessToken == "") && acc.RefreshToken != "" {
		// Attempt token refresh
		newTok, newRefTok, refErr := RefreshGoogleTokenFull(acc.RefreshToken, "", "")
		if refErr != nil {
			lastTokenRefreshErr = refErr
		} else if newTok != "" {
			acc.AccessToken = newTok
			accessToken = newTok
			if newRefTok != "" {
				acc.RefreshToken = newRefTok
			}
			pCtx, err = FetchProjectAndTier(accessToken)
		}
	}

	tier := ""
	credits := acc.Credits
	if pCtx != nil {
		if pCtx.ProjectID != "" {
			project = pCtx.ProjectID
		}
		if pCtx.TierName != "" && pCtx.TierName != PlanTierFree {
			tier = NormalizePlanTier(pCtx.TierName)
		}
		if pCtx.Credits > 0 {
			credits = pCtx.Credits
		}
	}

	// Step 2: Query Live Quota Summary
	breakdown, qErr := FetchLiveQuotaBreakdown(accessToken, project)
	if qErr != nil && acc.RefreshToken != "" && accessToken != "" {
		// Try refreshing token once if quota check failed
		newTok, newRefTok, refErr := RefreshGoogleTokenFull(acc.RefreshToken, "", "")
		if refErr != nil {
			lastTokenRefreshErr = refErr
		} else if newTok != "" {
			acc.AccessToken = newTok
			accessToken = newTok
			if newRefTok != "" {
				acc.RefreshToken = newRefTok
			}
			breakdown, qErr = FetchLiveQuotaBreakdown(accessToken, project)
		}
	}

	if breakdown != nil {
		if breakdown.SubscriptionTier != "" && breakdown.SubscriptionTier != PlanTierFree {
			tier = breakdown.SubscriptionTier
		}
		if breakdown.Credits > 0 {
			credits = breakdown.Credits
		}
		if breakdown.HasClaudeGPTRights || breakdown.Quota5hClaudeGPT > 0 || breakdown.QuotaWeeklyClaudeGPT > 0 {
			if tier == "" || tier == PlanTierFree || tier == PlanTierProTrial {
				tier = PlanTierPro
			}
		} else if tier == PlanTierPro && !breakdown.HasClaudeGPTRights {
			tier = PlanTierProTrial
		}
	}

	// Step 3: Check model permissions if tier is still unconfirmed or Free
	if tier == "" || tier == PlanTierFree {
		if modelTier, mErr := DetectTierFromAvailableModels(accessToken); mErr == nil && modelTier != "" && modelTier != PlanTierFree {
			tier = modelTier
		}
	}

	// Step 4: Check Google UserInfo (educational or enterprise domain)
	if tier == "" || tier == PlanTierFree {
		if uTier, uErr := DetectTierFromGoogleUserInfo(accessToken, acc.Email); uErr == nil && uTier != "" && uTier != PlanTierFree {
			tier = uTier
		}
	}

	// Step 5: Check cached cloud_accounts.db metrics
	if cachedAccs, cErr := keyring.ReadCloudAccountsDB(""); cErr == nil {
		for _, ca := range cachedAccs {
			if strings.EqualFold(ca.Email, acc.Email) {
				if ca.PlanTier != "" {
					pTier := NormalizePlanTier(ca.PlanTier)
					if (tier == "" || tier == PlanTierFree) && pTier != PlanTierFree {
						tier = pTier
					}
					if pTier == PlanTierProTrial && (tier == PlanTierPro || tier == "") {
						tier = PlanTierProTrial
					}
				}
				if ca.Credits > 0 && credits == 0 {
					credits = ca.Credits
				}
				break
			}
		}
	}

	// Step 6: Fallback to email domain heuristics if still unconfirmed
	if tier == "" || tier == PlanTierFree {
		if heuristic := DetermineDefaultPlanTier(acc.Email, ""); heuristic != PlanTierFree {
			tier = heuristic
		}
	}

	// Step 7: Anti-Downgrade Safeguard: preserve existing non-free tier if upstream returned Free or transient error
	if (tier == "" || tier == PlanTierFree) && acc.PlanTier != "" && acc.PlanTier != PlanTierFree {
		tier = acc.PlanTier
	}
	if acc.PlanTier == PlanTierProTrial && tier == PlanTierPro {
		tier = PlanTierProTrial
	}

	if tier == "" {
		tier = PlanTierFree
	}
	tier = NormalizePlanTier(tier)

	// Persist detected tier & credits back to account struct
	acc.PlanTier = tier
	if credits > 0 {
		acc.Credits = credits
	}

	if breakdown != nil {
		minFrac := breakdown.Quota5hGemini
		if breakdown.QuotaWeeklyGemini < minFrac {
			minFrac = breakdown.QuotaWeeklyGemini
		}

		summary := &QuotaSummary{
			AccountEmail:           acc.Email,
			PlanTier:               tier,
			Credits:                credits,
			Quota5hFraction:        breakdown.Quota5hGemini,
			QuotaWeeklyFraction:    breakdown.QuotaWeeklyGemini,
			Quota5hClaudeGPT:       breakdown.Quota5hClaudeGPT,
			QuotaWeeklyClaudeGPT:   breakdown.QuotaWeeklyClaudeGPT,
			ResetSeconds5h:         breakdown.ResetSeconds5h,
			ResetHorizonText:       breakdown.ResetHorizonText,
			ResetSecondsWeekly:     breakdown.ResetSecondsWeekly,
			ResetHorizonWeeklyText: breakdown.ResetHorizonWeeklyText,
			ResetTimeWeekly:        breakdown.ResetTimeWeekly,
			Models:                 breakdown.Models,
			MinFraction:            minFrac,
			OverallHealth:          ComputeHealth(minFrac),
			LastPolled:             now,
		}
		return summary, nil
	}

	var apiErr *QuotaAPIError
	if qErr != nil {
		if ae, ok := qErr.(*QuotaAPIError); ok {
			apiErr = ae
		}
	}

	if apiErr != nil {
		errMsg := apiErr.Message
		if apiErr.Reason != "" && !strings.Contains(errMsg, apiErr.Reason) {
			errMsg = fmt.Sprintf("%s (%s)", errMsg, apiErr.Reason)
		}
		if apiErr.ActionURL != "" {
			errMsg = fmt.Sprintf("%s (Verification: %s)", errMsg, apiErr.ActionURL)
		}
		errStatus := apiErr.Status
		if errStatus == "" {
			errStatus = "ERROR"
		}
		acc.Status = errStatus
		acc.ErrorMessage = errMsg

		return &QuotaSummary{
			AccountEmail:           acc.Email,
			PlanTier:               tier,
			Credits:                credits,
			MinFraction:            0.0,
			OverallHealth:          core.StatusExhausted,
			ErrorMessage:           errMsg,
			ErrorStatus:            errStatus,
			ResetHorizonText:       "Error",
			ResetHorizonWeeklyText: "Error",
			LastPolled:             now,
		}, apiErr
	}

	if lastTokenRefreshErr != nil && acc.RefreshToken != "" {
		errMsg := fmt.Sprintf("Token refresh failed: %v", lastTokenRefreshErr)
		errStatus := "ERROR"
		acc.Status = errStatus
		acc.ErrorMessage = errMsg
		return &QuotaSummary{
			AccountEmail:           acc.Email,
			PlanTier:               tier,
			Credits:                credits,
			MinFraction:            0.0,
			OverallHealth:          core.StatusExhausted,
			ErrorMessage:           errMsg,
			ErrorStatus:            errStatus,
			ResetHorizonText:       "Error",
			ResetHorizonWeeklyText: "Error",
			LastPolled:             now,
		}, lastTokenRefreshErr
	}

	// Fallback 1: Local Swiss Knife quota cache (independent of third-party manager)
	normEmail := strings.ToLower(strings.TrimSpace(acc.Email))
	if diskCache := LoadQuotaCache(); diskCache != nil && diskCache[normEmail] != nil {
		cached := diskCache[normEmail]
		return cached, nil
	}

	// Fallback 2: Cached cloud_accounts.db metrics if network query fails
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
				resetWeeklyText := "Ready"
				var resetWeeklySec float64
				var resetWeeklyTime time.Time
				if ca.ResetTimeWeekly != "" {
					if rt, parseErr := time.Parse(time.RFC3339, ca.ResetTimeWeekly); parseErr == nil && rt.After(now) {
						resetWeeklyTime = rt
						resetWeeklySec = rt.Sub(now).Seconds()
						resetWeeklyText = FormatResetHorizon(rt, now)
					}
				}
				pTier := tier
				cAmount := credits
				if cAmount == 0 && ca.Credits > 0 {
					cAmount = ca.Credits
				}

				minFrac := ca.Quota5h
				if ca.QuotaWeekly < minFrac {
					minFrac = ca.QuotaWeekly
				}

				return &QuotaSummary{
					AccountEmail:           acc.Email,
					PlanTier:               pTier,
					Credits:                cAmount,
					Quota5hFraction:        ca.Quota5h,
					QuotaWeeklyFraction:    ca.QuotaWeekly,
					Quota5hClaudeGPT:       ca.Quota5hClaudeGPT,
					QuotaWeeklyClaudeGPT:   ca.QuotaWeeklyClaudeGPT,
					ResetSeconds5h:         resetSec,
					ResetHorizonText:       resetText,
					ResetSecondsWeekly:     resetWeeklySec,
					ResetHorizonWeeklyText: resetWeeklyText,
					ResetTimeWeekly:        resetWeeklyTime,
					Models:                 []ModelQuota{},
					MinFraction:            minFrac,
					OverallHealth:          ComputeHealth(minFrac),
					LastPolled:             now,
				}, nil
			}
		}
	}

	// Baseline fallback
	return &QuotaSummary{
		AccountEmail:           acc.Email,
		PlanTier:               tier,
		Credits:                credits,
		MinFraction:            0.0,
		OverallHealth:          core.StatusExhausted,
		ResetHorizonText:       "Not Polled",
		ResetHorizonWeeklyText: "Not Polled",
		LastPolled:             now,
	}, qErr
}
