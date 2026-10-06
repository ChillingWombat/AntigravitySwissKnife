package quota

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

// Switch mode constants
const (
	SwitchModeBalanced      = "balanced"
	SwitchModeMaxTokens     = "max_tokens"
	SwitchModeMaxContinuous = "max_continuous"

	MinProactiveSwitchDwellSeconds = 600.0   // 10 minutes
	FiveHourWindowSeconds          = 18000.0 // 5 hours
	WeeklyWindowSeconds            = 604800.0 // 7 days
)

var horizonUnitRegex = regexp.MustCompile(`(?i)(\d+)\s*([dhms])`)


// AccountQuotaState represents live quota state and credential horizons for an account.
type AccountQuotaState struct {
	Email                string  `json:"email"`
	Label                string  `json:"label"`
	PlanTier             string  `json:"plan_tier"`
	Credits              float64 `json:"credits"`
	EnableCreditOverages bool    `json:"enable_credit_overages"`
	AllowClaudeGPT       bool    `json:"allow_claude_gpt"`
	IsActive             bool    `json:"is_active"`
	Status               string  `json:"status"`
	Priority             string  `json:"priority,omitempty"`
	Notes                string  `json:"notes,omitempty"`
	Password             string  `json:"password,omitempty"`
	HasTOTP              bool    `json:"has_totp"`
	TOTPSecret           string  `json:"totp_secret"`
	RefreshToken         string  `json:"refresh_token"`
	Quota5hCurrent       float64 `json:"quota_5h_current"`
	ResetSeconds         float64 `json:"reset_seconds"`
	QuotaWeekly          float64 `json:"quota_weekly"`
	Quota5hAvailable     float64 `json:"quota_5h_available"`
	Quota5hClaudeGPT       float64 `json:"quota_5h_claude_gpt"`
	QuotaWeeklyClaudeGPT   float64 `json:"quota_weekly_claude_gpt"`
	ResetHorizonText       string  `json:"reset_horizon_text"`
	ResetSecondsWeekly     float64 `json:"reset_seconds_weekly"`
	ResetHorizonWeeklyText string  `json:"reset_horizon_weekly_text"`
}

// ComputeEffective5hAvailable calculates available quota in the next 5 hours with reset replenishing.
func ComputeEffective5hAvailable(currentFrac float64, resetSeconds float64) float64 {
	cur := math.Max(0.0, math.Min(1.0, currentFrac))
	if cur == 0.0 && resetSeconds <= 0.0 {
		return 0.0
	}
	if resetSeconds > 0.0 {
		h := resetSeconds / 3600.0
		if h <= 5.0 {
			replenishedBoost := (1.0 - cur) * ((5.0 - h) / 5.0)
			return math.Max(0.0, math.Min(1.0, cur+replenishedBoost))
		}
	}
	return cur
}

// FormatHorizonSec returns human-readable countdown string.
func FormatHorizonSec(sec float64) string {
	s := int(math.Max(0.0, sec))
	if s == 0 {
		return "Ready"
	}
	if s < 60 {
		return fmt.Sprintf("Resets in %ds", s)
	}
	if s < 3600 {
		return fmt.Sprintf("Resets in %dm", s/60)
	}
	if s < 86400 {
		h := s / 3600
		m := (s % 3600) / 60
		return fmt.Sprintf("Resets in %dh %dm", h, m)
	}
	d := s / 86400
	h := (s % 86400) / 3600
	return fmt.Sprintf("Resets in %dd %dh", d, h)
}

// FleetQuotaSummary holds aggregate metrics for top dashboard section.
type FleetQuotaSummary struct {
	Fleet5hFraction               float64             `json:"fleet_5h_fraction"`
	FleetWeeklyFraction           float64             `json:"fleet_weekly_fraction"`
	Fleet5hAvailable              float64             `json:"fleet_5h_available"`
	FleetWeeklyAvailable          float64             `json:"fleet_weekly_available"`
	Fleet5hGeminiAvailable        float64             `json:"fleet_5h_gemini_available"`
	FleetWeeklyGeminiAvailable    float64             `json:"fleet_weekly_gemini_available"`
	Fleet5hClaudeGPTAvailable     float64             `json:"fleet_5h_claude_gpt_available"`
	FleetWeeklyClaudeGPTAvailable float64             `json:"fleet_weekly_claude_gpt_available"`
	TotalAccounts                 int                 `json:"total_accounts"`
	ActiveAccount                 string              `json:"active_account"`
	Accounts                      []AccountQuotaState `json:"accounts"`
}

// ComputeFleetSummary aggregates metrics across all managed accounts.
func ComputeFleetSummary(accounts []AccountQuotaState, activeEmail string) FleetQuotaSummary {
	total := len(accounts)
	if total == 0 {
		return FleetQuotaSummary{
			Fleet5hFraction:               0.0,
			FleetWeeklyFraction:           0.0,
			Fleet5hAvailable:              0.0,
			FleetWeeklyAvailable:          0.0,
			Fleet5hGeminiAvailable:        0.0,
			FleetWeeklyGeminiAvailable:    0.0,
			Fleet5hClaudeGPTAvailable:     0.0,
			FleetWeeklyClaudeGPTAvailable: 0.0,
			TotalAccounts:                 0,
			ActiveAccount:                 activeEmail,
			Accounts:                      accounts,
		}
	}

	sum5h := 0.0
	sumWeekly := 0.0
	sum5hClaude := 0.0
	sumWeeklyClaude := 0.0

	for _, acc := range accounts {
		sum5h += acc.Quota5hAvailable
		sumWeekly += acc.QuotaWeekly
		sum5hClaude += acc.Quota5hClaudeGPT
		sumWeeklyClaude += acc.QuotaWeeklyClaudeGPT
	}

	avg5h := math.Max(0.0, math.Min(1.0, sum5h/float64(total)))
	avgWeekly := math.Max(0.0, math.Min(1.0, sumWeekly/float64(total)))
	avg5hClaude := math.Max(0.0, math.Min(1.0, sum5hClaude/float64(total)))
	avgWeeklyClaude := math.Max(0.0, math.Min(1.0, sumWeeklyClaude/float64(total)))

	normActive := strings.TrimSpace(strings.ToLower(activeEmail))
	for i := range accounts {
		accounts[i].IsActive = (normActive != "" && strings.ToLower(strings.TrimSpace(accounts[i].Email)) == normActive)
	}

	return FleetQuotaSummary{
		Fleet5hFraction:               avg5h,
		FleetWeeklyFraction:           avgWeekly,
		Fleet5hAvailable:              avg5h,
		FleetWeeklyAvailable:          avgWeekly,
		Fleet5hGeminiAvailable:        avg5h,
		FleetWeeklyGeminiAvailable:    avgWeekly,
		Fleet5hClaudeGPTAvailable:     avg5hClaude,
		FleetWeeklyClaudeGPTAvailable: avgWeeklyClaude,
		TotalAccounts:                 total,
		ActiveAccount:                 activeEmail,
		Accounts:                      accounts,
	}
}

// PollFleetAccounts polls live quotas for all accounts with credentials concurrently.
func PollFleetAccounts(accounts []*keyring.Account, store *keyring.Store) map[string]*QuotaSummary {
	results := make(map[string]*QuotaSummary)
	if len(accounts) == 0 {
		return results
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, acc := range accounts {
		if acc == nil || acc.Email == "" {
			continue
		}

		wg.Add(1)
		go func(targetAcc *keyring.Account) {
			defer wg.Done()
			qSummary, _ := PollAccountLiveQuota(targetAcc)
			if qSummary != nil {
				mu.Lock()
				results[strings.ToLower(strings.TrimSpace(targetAcc.Email))] = qSummary
				mu.Unlock()
				if store != nil {
					planTier := targetAcc.PlanTier
					if qSummary.PlanTier != "" {
						planTier = qSummary.PlanTier
					}
					credits := targetAcc.Credits
					if qSummary.Credits > 0 {
						credits = qSummary.Credits
					}
					_ = store.UpdateAccountQuotaMetadata(targetAcc.Email, planTier, credits)
				}
			}
		}(acc)
	}

	wg.Wait()
	return results
}

// BuildAccountQuotaStates creates AccountQuotaState items from stored accounts.
func BuildAccountQuotaStates(accounts []*keyring.Account, activeSummary *QuotaSummary) []AccountQuotaState {
	return BuildAccountQuotaStatesWithThreshold(accounts, activeSummary, core.DefaultAutoSwitchThresholdFraction)
}

// BuildAccountQuotaStatesWithThreshold creates AccountQuotaState items from stored accounts with custom threshold.
func BuildAccountQuotaStatesWithThreshold(accounts []*keyring.Account, activeSummary *QuotaSummary, threshold float64) []AccountQuotaState {
	var summaries map[string]*QuotaSummary
	if activeSummary != nil {
		summaries = map[string]*QuotaSummary{
			strings.ToLower(strings.TrimSpace(activeSummary.AccountEmail)): activeSummary,
		}
	}
	return BuildAccountQuotaStatesFromMapWithThreshold(accounts, summaries, threshold)
}

// BuildAccountQuotaStatesFromMap builds account states using live summaries and cached agent db.
func BuildAccountQuotaStatesFromMap(accounts []*keyring.Account, summaries map[string]*QuotaSummary) []AccountQuotaState {
	return BuildAccountQuotaStatesFromMapWithThreshold(accounts, summaries, core.DefaultAutoSwitchThresholdFraction)
}

// BuildAccountQuotaStatesFromMapWithThreshold builds account states using live summaries, cached agent db, and custom exhaustion threshold.
func BuildAccountQuotaStatesFromMapWithThreshold(accounts []*keyring.Account, summaries map[string]*QuotaSummary, threshold float64) []AccountQuotaState {
	results := make([]AccountQuotaState, 0, len(accounts))
	now := time.Now()

	// Load cached cloud_accounts.db as fast baseline
	cachedMap := make(map[string]keyring.DiscoveredCloudAccount)
	if cachedAccs, err := keyring.ReadCloudAccountsDB(""); err == nil {
		for _, ca := range cachedAccs {
			cachedMap[strings.ToLower(strings.TrimSpace(ca.Email))] = ca
		}
	}

	for _, acc := range accounts {
		email := strings.TrimSpace(acc.Email)
		if email == "" {
			continue
		}
		normEmail := strings.ToLower(email)

		label := acc.Label
		if label == "" {
			label = email
		}

		status := "STANDBY"
		if acc.Status != "" {
			status = strings.ToUpper(acc.Status)
		} else if acc.IsActive {
			status = "ACTIVE"
		}

		cur5h := 0.0
		curSec := 0.0
		curWeekly := 0.0
		curSecWeekly := 0.0
		cur5hClaude := 1.0
		curWeeklyClaude := 1.0
		resText := "Not Polled"
		resWeeklyText := "Not Polled"
		tier := acc.PlanTier
		credits := acc.Credits

		// 1. Check live summary first
		if summaries != nil && summaries[normEmail] != nil {
			s := summaries[normEmail]
			if s.PlanTier != "" {
				tier = s.PlanTier
			}
			credits = s.Credits
			cur5h = s.Quota5hFraction
			curWeekly = s.QuotaWeeklyFraction
			if cur5h == 0 && curWeekly == 0 && len(s.Models) > 0 {
				for _, m := range s.Models {
					mLower := strings.ToLower(m.ModelName)
					if strings.Contains(mLower, "weekly") {
						curWeekly = m.Fraction
						if !m.ResetTime.IsZero() && m.ResetTime.After(now) {
							curSecWeekly = m.ResetTime.Sub(now).Seconds()
							resWeeklyText = FormatResetHorizon(m.ResetTime, now)
						}
					} else {
						cur5h = m.Fraction
						if !m.ResetTime.IsZero() && m.ResetTime.After(now) {
							curSec = m.ResetTime.Sub(now).Seconds()
							resText = FormatResetHorizon(m.ResetTime, now)
						}
					}
				}
				if curWeekly == 0 && cur5h > 0 {
					curWeekly = cur5h
				}
			}
			if s.Quota5hClaudeGPT > 0 {
				cur5hClaude = s.Quota5hClaudeGPT
			}
			if s.QuotaWeeklyClaudeGPT > 0 {
				curWeeklyClaude = s.QuotaWeeklyClaudeGPT
			}
			if s.ResetSeconds5h > 0 {
				curSec = s.ResetSeconds5h
			}
			if s.ResetHorizonText != "" {
				resText = s.ResetHorizonText
			}
			if s.ResetSecondsWeekly > 0 {
				curSecWeekly = s.ResetSecondsWeekly
			}
			if s.ResetHorizonWeeklyText != "" && s.ResetHorizonWeeklyText != "Not Polled" {
				resWeeklyText = s.ResetHorizonWeeklyText
			}
			if !s.ResetTimeWeekly.IsZero() && s.ResetTimeWeekly.After(now) {
				curSecWeekly = s.ResetTimeWeekly.Sub(now).Seconds()
				resWeeklyText = FormatResetHorizon(s.ResetTimeWeekly, now)
			}
			if (resWeeklyText == "" || resWeeklyText == "Not Polled" || resWeeklyText == "Ready") && len(s.Models) > 0 {
				for _, m := range s.Models {
					if strings.Contains(strings.ToLower(m.ModelName), "weekly") && !m.ResetTime.IsZero() && m.ResetTime.After(now) {
						curSecWeekly = m.ResetTime.Sub(now).Seconds()
						resWeeklyText = FormatResetHorizon(m.ResetTime, now)
						break
					}
				}
			}
			if (resWeeklyText == "" || resWeeklyText == "Not Polled" || resWeeklyText == "Ready") && cachedMap != nil {
				if ca, ok := cachedMap[normEmail]; ok && ca.ResetTimeWeekly != "" {
					if rt, parseErr := time.Parse(time.RFC3339, ca.ResetTimeWeekly); parseErr == nil && rt.After(now) {
						curSecWeekly = rt.Sub(now).Seconds()
						resWeeklyText = FormatHorizonSec(curSecWeekly)
					}
				}
			}
		} else if ca, ok := cachedMap[normEmail]; ok {
			// 2. Check cached agent DB
			if caSt := strings.ToUpper(strings.TrimSpace(ca.Status)); caSt == "BANNED" || caSt == "ERROR" {
				status = caSt
			}
			if ca.PlanTier != "" {
				tier = ca.PlanTier
			}
			credits = ca.Credits
			cur5h = ca.Quota5h
			curWeekly = ca.QuotaWeekly
			if ca.Quota5hClaudeGPT > 0 {
				cur5hClaude = ca.Quota5hClaudeGPT
			}
			if ca.QuotaWeeklyClaudeGPT > 0 {
				curWeeklyClaude = ca.QuotaWeeklyClaudeGPT
			}
			if ca.ResetTime5h != "" {
				if rt, parseErr := time.Parse(time.RFC3339, ca.ResetTime5h); parseErr == nil && rt.After(now) {
					curSec = rt.Sub(now).Seconds()
					resText = FormatHorizonSec(curSec)
				}
			}
			if ca.ResetTimeWeekly != "" {
				if rt, parseErr := time.Parse(time.RFC3339, ca.ResetTimeWeekly); parseErr == nil && rt.After(now) {
					curSecWeekly = rt.Sub(now).Seconds()
					resWeeklyText = FormatHorizonSec(curSecWeekly)
				}
			}
		}

		if tier == "" || tier == PlanTierFree {
			tier = DetermineDefaultPlanTier(email, acc.PlanTier)
		} else {
			tier = NormalizePlanTier(tier)
		}

		avail5h := ComputeEffective5hAvailable(cur5h, curSec)
		if resText == "Not Polled" && curSec > 0 {
			resText = FormatHorizonSec(curSec)
		} else if resText == "Not Polled" && cur5h > 0 {
			resText = "Ready"
		}

		if resWeeklyText == "Not Polled" && curSecWeekly > 0 {
			resWeeklyText = FormatHorizonSec(curSecWeekly)
		} else if resWeeklyText == "Not Polled" && curWeekly > 0 {
			resWeeklyText = "Ready"
		}

		prio := acc.Priority
		if prio == "" {
			prio = "High"
		}

		// Cooldown evaluation for valid (non-banned, non-error) non-active accounts:
		// A valid account whose quota is below threshold and waiting to be reset enters COOLDOWN.
		// If quota recovers above threshold after reset, status returns to STANDBY.
		if status != "BANNED" && status != "ERROR" && !acc.IsActive {
			_, inCached := cachedMap[normEmail]
			hasPolledData := cur5h > 0 || curWeekly > 0 || curSec > 0 || curSecWeekly > 0 || (resText != "" && resText != "Not Polled") || (summaries != nil && summaries[normEmail] != nil) || inCached
			if hasPolledData {
				isBelow := cur5h <= threshold || (curWeekly <= 0.05 && !(acc.EnableCreditOverages && credits > 0))
				if isBelow {
					status = "COOLDOWN"
				} else if status == "COOLDOWN" {
					status = "STANDBY"
				}
			}
		}

		results = append(results, AccountQuotaState{
			Email:                  email,
			Label:                  label,
			PlanTier:               tier,
			Credits:                credits,
			EnableCreditOverages:   acc.EnableCreditOverages,
			AllowClaudeGPT:         acc.AllowClaudeGPT,
			IsActive:               acc.IsActive,
			Status:                 status,
			Priority:               prio,
			Notes:                  acc.Notes,
			Password:               acc.Password,
			HasTOTP:                acc.HasTOTP,
			TOTPSecret:             acc.TOTPSecret,
			RefreshToken:           acc.RefreshToken,
			Quota5hCurrent:         cur5h,
			ResetSeconds:           curSec,
			QuotaWeekly:            curWeekly,
			Quota5hAvailable:       avail5h,
			Quota5hClaudeGPT:       cur5hClaude,
			QuotaWeeklyClaudeGPT:   curWeeklyClaude,
			ResetHorizonText:       resText,
			ResetSecondsWeekly:     curSecWeekly,
			ResetHorizonWeeklyText: resWeeklyText,
		})
	}

	return results
}

// DetermineDefaultPlanTier computes a membership tier based on explicit tier or email domain/label heuristics.
func DetermineDefaultPlanTier(email string, explicitTier string) string {
	if explicitTier != "" && explicitTier != PlanTierFree {
		return NormalizePlanTier(explicitTier)
	}
	lower := strings.ToLower(email)
	if strings.Contains(lower, ".edu") || strings.Contains(lower, "edu.") || strings.Contains(lower, "-edu") {
		return PlanTierEdu
	}
	if strings.Contains(lower, "enterprise") {
		return PlanTierEnterprise
	}
	if strings.Contains(lower, "ultra20") || strings.Contains(lower, "20x") {
		return PlanTierUltra20X
	}
	if strings.Contains(lower, "ultra10") || strings.Contains(lower, "10x") {
		return PlanTierUltra10X
	}
	if strings.Contains(lower, "ultra5") || strings.Contains(lower, "5x") {
		return PlanTierUltra5X
	}
	if strings.Contains(lower, "ultra") {
		return PlanTierUltra20X
	}
	if strings.Contains(lower, "trial") {
		return PlanTierProTrial
	}
	if strings.Contains(lower, "plus") {
		return PlanTierPlus
	}
	if explicitTier == PlanTierFree || strings.Contains(lower, "free") {
		return PlanTierFree
	}
	return PlanTierPro
}

// ClassifyErrorStatus classifies an error code or message into "BANNED", "ERROR", or "STANDBY".
func ClassifyErrorStatus(statusCode int, errCode string, errMsg string) string {
	combined := strings.ToLower(errCode + " " + errMsg)
	if strings.Contains(combined, "suspend") ||
		strings.Contains(combined, "banned") ||
		strings.Contains(combined, "disabled") ||
		strings.Contains(combined, "terminated") ||
		strings.Contains(combined, "violates") ||
		strings.Contains(combined, "account_disabled") ||
		strings.Contains(combined, "permenantly_disabled") {
		return "BANNED"
	}
	if statusCode >= 500 ||
		strings.Contains(combined, "invalid_grant") ||
		strings.Contains(combined, "unauthorized_client") ||
		strings.Contains(combined, "auth_error") ||
		strings.Contains(combined, "token_refresh_failed") ||
		strings.Contains(combined, "expired") {
		return "ERROR"
	}
	return "STANDBY"
}

// clamp01 bounds a floating point value to [0.0, 1.0].
func clamp01(v float64) float64 {
	if math.IsNaN(v) || v <= 0.0 {
		return 0.0
	}
	if v >= 1.0 {
		return 1.0
	}
	return v
}

// ParseHorizonTextSeconds extracts approximate seconds from formatted countdown text.
// Examples: "Resets in 2h 15m" -> 8100, "Resets in 3d 5h" -> 277200, "Resets in 45m" -> 2700, "Ready" -> 0.
func ParseHorizonTextSeconds(text string) float64 {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || strings.EqualFold(trimmed, "ready") || strings.EqualFold(trimmed, "not polled") || strings.EqualFold(trimmed, "resets now") {
		return 0.0
	}
	matches := horizonUnitRegex.FindAllStringSubmatch(trimmed, -1)
	if len(matches) == 0 {
		return 0.0
	}
	var totalSec float64
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		val, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		switch strings.ToLower(m[2]) {
		case "d":
			totalSec += val * 86400.0
		case "h":
			totalSec += val * 3600.0
		case "m":
			totalSec += val * 60.0
		case "s":
			totalSec += val
		}
	}
	return totalSec
}

// NormalizeSwitchMode standardizes switch mode identifiers.
func NormalizeSwitchMode(raw string) string {
	clean := strings.ToLower(strings.TrimSpace(raw))
	switch clean {
	case SwitchModeMaxTokens, "max_total_tokens", "max-tokens", "tokens":
		return SwitchModeMaxTokens
	case SwitchModeMaxContinuous, "continuous", "max_continues", "max-continuous", "continues":
		return SwitchModeMaxContinuous
	case SwitchModeBalanced, "default", "":
		return SwitchModeBalanced
	default:
		return SwitchModeBalanced
	}
}

// ResolveAccountPlanTier returns the canonical plan tier for an account, falling back to heuristics if blank.
func ResolveAccountPlanTier(email, tier string) string {
	t := strings.TrimSpace(tier)
	if t != "" {
		return NormalizePlanTier(t)
	}
	return DetermineDefaultPlanTier(email, "")
}

// IsFreePlanTier checks if an account belongs to the Free tier.
func IsFreePlanTier(email, tier string) bool {
	return ResolveAccountPlanTier(email, tier) == PlanTierFree
}

// PlanTierRank returns an integer ordering for plan tiers (higher = superior paid tier, Free = 0).
func PlanTierRank(tier string) int {
	norm := NormalizePlanTier(tier)
	switch norm {
	case PlanTierEnterprise:
		return 8
	case PlanTierUltra20X:
		return 7
	case PlanTierUltra10X:
		return 6
	case PlanTierUltra5X:
		return 5
	case PlanTierPro:
		return 4
	case PlanTierEdu:
		return 3
	case PlanTierProTrial:
		return 2
	case PlanTierPlus:
		return 1
	case PlanTierFree:
		return 0
	default:
		return 0
	}
}

// PlanTierCapacityMultiplier returns a bounded relative token capacity factor relative to Pro (1.0).
// Higher tiers like Ultra 20X have much higher continuous headroom and clock-ignition yield,
// while Free is penalized to guarantee it is ranked last.
func PlanTierCapacityMultiplier(tier string) float64 {
	norm := NormalizePlanTier(tier)
	switch norm {
	case PlanTierEnterprise:
		return 1.45
	case PlanTierUltra20X:
		return 1.40
	case PlanTierUltra10X:
		return 1.30
	case PlanTierUltra5X:
		return 1.20
	case PlanTierPro:
		return 1.00
	case PlanTierEdu:
		return 0.98
	case PlanTierProTrial:
		return 0.92
	case PlanTierPlus:
		return 0.80
	case PlanTierFree:
		return 0.25
	default:
		return 1.00
	}
}

// PlanTierNormalizedWeight returns a normalized [0.0, 1.0] weight for blended scoring.
func PlanTierNormalizedWeight(tier string) float64 {
	norm := NormalizePlanTier(tier)
	switch norm {
	case PlanTierEnterprise:
		return 1.00
	case PlanTierUltra20X:
		return 0.95
	case PlanTierUltra10X:
		return 0.85
	case PlanTierUltra5X:
		return 0.75
	case PlanTierPro:
		return 0.55
	case PlanTierEdu:
		return 0.52
	case PlanTierProTrial:
		return 0.45
	case PlanTierPlus:
		return 0.30
	case PlanTierFree:
		return 0.05
	default:
		return 0.50
	}
}

// PriorityRank converts user priority string ("High", "Mid", "Low") to integer (0 = High, 1 = Mid, 2 = Low).
func PriorityRank(prio string) int {
	clean := strings.ToUpper(strings.TrimSpace(prio))
	switch clean {
	case "HIGH", "":
		return 0
	case "MID", "MEDIUM":
		return 1
	case "LOW":
		return 2
	default:
		return 0
	}
}

// AccountMetrics holds extracted and normalized multi-metric signals for ranking.
type AccountMetrics struct {
	Q5hCur       float64
	Q5hAvail     float64
	R5hSoonness  float64
	Q7d          float64
	R7dSoonness  float64
	Tier         string
	TierRank     int
	TierMult     float64
	TierNorm     float64
	IsFree       bool
	PriorityRank int
}

// ExtractAccountMetrics derives normalized values for all 4 metrics and plan tier.
func ExtractAccountMetrics(acc AccountQuotaState, mode string) AccountMetrics {
	q5hCur := clamp01(acc.Quota5hCurrent)
	sec5h := acc.ResetSeconds
	if sec5h <= 0.0 && acc.ResetHorizonText != "" {
		sec5h = ParseHorizonTextSeconds(acc.ResetHorizonText)
	}

	q5hAvail := ComputeEffective5hAvailable(q5hCur, sec5h)
	if acc.Quota5hAvailable > q5hAvail && sec5h <= 0.0 {
		q5hAvail = clamp01(acc.Quota5hAvailable)
	}

	var r5hSoonness float64
	if sec5h > 0.0 && sec5h <= FiveHourWindowSeconds {
		r5hSoonness = clamp01(1.0 - (sec5h / FiveHourWindowSeconds))
	} else if sec5h > FiveHourWindowSeconds {
		r5hSoonness = 0.0
	} else {
		// sec5h <= 0 (Ready / unstarted)
		if mode == SwitchModeMaxTokens && q5hCur >= 0.98 {
			r5hSoonness = 0.85 // Clock ignition bonus
		} else {
			r5hSoonness = 0.50
		}
	}

	q7d := clamp01(acc.QuotaWeekly)
	sec7d := acc.ResetSecondsWeekly
	if sec7d <= 0.0 && acc.ResetHorizonWeeklyText != "" {
		sec7d = ParseHorizonTextSeconds(acc.ResetHorizonWeeklyText)
	}

	var r7dSoonness float64
	if sec7d > 0.0 && sec7d <= WeeklyWindowSeconds {
		r7dSoonness = clamp01(1.0 - (sec7d / WeeklyWindowSeconds))
	} else if sec7d > WeeklyWindowSeconds {
		r7dSoonness = 0.0
	} else {
		r7dSoonness = 0.35
	}

	tier := ResolveAccountPlanTier(acc.Email, acc.PlanTier)
	isFree := tier == PlanTierFree

	return AccountMetrics{
		Q5hCur:       q5hCur,
		Q5hAvail:     q5hAvail,
		R5hSoonness:  r5hSoonness,
		Q7d:          q7d,
		R7dSoonness:  r7dSoonness,
		Tier:         tier,
		TierRank:     PlanTierRank(tier),
		TierMult:     PlanTierCapacityMultiplier(tier),
		TierNorm:     PlanTierNormalizedWeight(tier),
		IsFree:       isFree,
		PriorityRank: PriorityRank(acc.Priority),
	}
}

// ComputeAccountRankingScore evaluates an account score according to the active switch mode.
// Incorporates all 4 metrics (5h remaining, 5h reset time, 7d remaining, 7d reset time) and Plan Tier.
func ComputeAccountRankingScore(acc AccountQuotaState, threshold float64, mode string) float64 {
	m := NormalizeSwitchMode(mode)
	metrics := ExtractAccountMetrics(acc, m)

	switch m {
	case SwitchModeMaxContinuous:
		// Primary emphasis on continuous 5h headroom, plus reset time & 7d stability
		score := 0.75*(metrics.TierMult*metrics.Q5hAvail) +
			0.08*metrics.R5hSoonness +
			0.10*metrics.Q7d +
			0.05*metrics.R7dSoonness +
			0.02*metrics.TierNorm
		return score

	case SwitchModeMaxTokens:
		// Emphasis on harvesting expiring tokens (high soonness with remaining quota) and clock ignition
		var urgency5h float64
		if acc.ResetSeconds > 0.0 && acc.ResetSeconds <= FiveHourWindowSeconds {
			urgency5h = metrics.Q5hCur * (0.45 + 0.55*metrics.R5hSoonness)
		} else if acc.ResetSeconds <= 0.0 && metrics.Q5hCur >= 0.98 {
			urgency5h = metrics.Q5hCur * 0.85
		} else {
			urgency5h = metrics.Q5hCur * 0.50
		}

		urgency7d := metrics.Q7d * (0.55 + 0.45*metrics.R7dSoonness)

		score := metrics.TierMult * (0.45*urgency5h +
			0.15*metrics.Q5hAvail +
			0.25*metrics.Q7d +
			0.15*urgency7d)
		return score

	case SwitchModeBalanced:
		fallthrough
	default:
		// Balanced compromise across all 4 metrics and plan tier
		score := metrics.TierMult * (0.42*metrics.Q5hCur +
			0.12*metrics.Q5hAvail +
			0.06*metrics.R5hSoonness +
			0.32*metrics.Q7d +
			0.08*metrics.R7dSoonness)
		return score
	}
}

// CompareStandbyCandidates determines if candidate A should be ranked ahead of candidate B.
func CompareStandbyCandidates(a, b AccountQuotaState, threshold float64, mode string) bool {
	m := NormalizeSwitchMode(mode)
	mA := ExtractAccountMetrics(a, m)
	mB := ExtractAccountMetrics(b, m)

	// Invariant 1: Free accounts are always ranked last among healthy standby candidates
	if mA.IsFree != mB.IsFree {
		return !mA.IsFree
	}

	// Invariant 2: Within same plan tier class, respect user Priority (High > Mid > Low)
	if mA.PriorityRank != mB.PriorityRank {
		return mA.PriorityRank < mB.PriorityRank
	}

	// Invariant 3: Mode-specific ranking across the 4 metrics + plan tier
	if m == SwitchModeMaxContinuous {
		cap5hA := mA.TierMult * mA.Q5hAvail
		cap5hB := mB.TierMult * mB.Q5hAvail
		if math.Abs(cap5hA-cap5hB) > 0.02 {
			return cap5hA > cap5hB
		}

		// When 5h available capacities are about the same (<= 2%), consider reset time and 7d metrics
		tieA := 0.30*mA.R5hSoonness + 0.20*mA.Q5hCur + 0.30*mA.Q7d + 0.15*mA.R7dSoonness + 0.05*mA.TierNorm
		tieB := 0.30*mB.R5hSoonness + 0.20*mB.Q5hCur + 0.30*mB.Q7d + 0.15*mB.R7dSoonness + 0.05*mB.TierNorm
		if math.Abs(tieA-tieB) > 0.0005 {
			return tieA > tieB
		}
	} else {
		scoreA := ComputeAccountRankingScore(a, threshold, m)
		scoreB := ComputeAccountRankingScore(b, threshold, m)
		if math.Abs(scoreA-scoreB) > 0.0005 {
			return scoreA > scoreB
		}
	}

	// Fallback deterministic tie-breakers
	if mA.TierRank != mB.TierRank {
		return mA.TierRank > mB.TierRank
	}
	if math.Abs(mA.Q5hAvail-mB.Q5hAvail) > 0.0005 {
		return mA.Q5hAvail > mB.Q5hAvail
	}
	if math.Abs(mA.Q5hCur-mB.Q5hCur) > 0.0005 {
		return mA.Q5hCur > mB.Q5hCur
	}
	if math.Abs(mA.R5hSoonness-mB.R5hSoonness) > 0.0005 {
		return mA.R5hSoonness > mB.R5hSoonness
	}
	if math.Abs(mA.Q7d-mB.Q7d) > 0.0005 {
		return mA.Q7d > mB.Q7d
	}
	if math.Abs(mA.R7dSoonness-mB.R7dSoonness) > 0.0005 {
		return mA.R7dSoonness > mB.R7dSoonness
	}

	// Alphabetical stability
	nameA := strings.ToLower(a.Label)
	if nameA == "" {
		nameA = strings.ToLower(a.Email)
	}
	nameB := strings.ToLower(b.Label)
	if nameB == "" {
		nameB = strings.ToLower(b.Email)
	}
	return nameA < nameB
}

// RankStandbyAccounts sorts standby accounts in default balanced mode (backwards-compatible).
func RankStandbyAccounts(accounts []AccountQuotaState, threshold float64) []AccountQuotaState {
	return RankStandbyAccountsWithMode(accounts, threshold, SwitchModeBalanced)
}

// RankStandbyAccountsWithMode filters eligible standby candidates and sorts them by switch mode.
func RankStandbyAccountsWithMode(accounts []AccountQuotaState, threshold float64, mode string) []AccountQuotaState {
	m := NormalizeSwitchMode(mode)
	candidates := make([]AccountQuotaState, 0)

	for _, acc := range accounts {
		if acc.IsActive {
			continue
		}
		st := strings.ToUpper(acc.Status)
		if st == "BANNED" || st == "ERROR" || st == "COOLDOWN" {
			continue
		}
		if acc.Quota5hCurrent <= threshold {
			continue
		}
		hasWeekly := acc.QuotaWeekly > 0.05 || (acc.EnableCreditOverages && acc.Credits > 0)
		if !hasWeekly {
			continue
		}
		candidates = append(candidates, acc)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return CompareStandbyCandidates(candidates[i], candidates[j], threshold, m)
	})

	return candidates
}

// ShouldSwitchProactivelyMaxTokens determines if an active account should proactively rotate
// in max_tokens mode before reaching the exhaustion threshold.
func ShouldSwitchProactivelyMaxTokens(active, bestStandby AccountQuotaState, threshold float64, activeDwellSec float64) (bool, string) {
	// Guardrail 1: Minimum active use dwell time (default 10 minutes = 600 seconds)
	hasSufficientDwell := activeDwellSec >= MinProactiveSwitchDwellSeconds
	if !hasSufficientDwell && activeDwellSec <= 0.0 {
		// If dwell is untracked (daemon restart), infer dwell from active's running 5h clock (used >= 10m into 5h window)
		if active.ResetSeconds > 0.0 && active.ResetSeconds <= (FiveHourWindowSeconds - MinProactiveSwitchDwellSeconds) {
			hasSufficientDwell = true
		}
	}
	if !hasSufficientDwell {
		return false, "Minimum 10-minute active use dwell not reached"
	}

	// Guardrail 2: Do NOT switch away if active account is about to reset soon (finish its expiring quota)
	if active.ResetSeconds > 0.0 && active.ResetSeconds <= 2700.0 { // <= 45 minutes
		return false, "Active account 5h reset is imminent (harvesting active expiring quota)"
	}

	// Guardrail 3: Best standby candidate must be a healthy paid account (or active is also Free)
	if IsFreePlanTier(bestStandby.Email, bestStandby.PlanTier) && !IsFreePlanTier(active.Email, active.PlanTier) {
		return false, "Standby is Free tier while active is paid tier"
	}

	// Check Proactive Trigger 1: Idle Clock Ignition
	// Active 5h clock is running, and standby has an unstarted 100% 5h clock waiting to be ignited
	if active.ResetSeconds > 0.0 && bestStandby.Quota5hCurrent >= 0.98 && bestStandby.ResetSeconds <= 0.0 {
		return true, fmt.Sprintf("Proactive rotation to ignite standby 5h reset clock (%s)", bestStandby.Email)
	}

	// Check Proactive Trigger 2: Standby 5h reset is significantly earlier than active's
	if bestStandby.ResetSeconds > 0.0 {
		if active.ResetSeconds <= 0.0 || (bestStandby.ResetSeconds+1800.0 < active.ResetSeconds) {
			return true, fmt.Sprintf("Proactive rotation to harvest standby expiring 5h window (%s resets in %.0fs vs active %.0fs)", bestStandby.Email, bestStandby.ResetSeconds, active.ResetSeconds)
		}
	}

	// Check Proactive Trigger 3: Standby weekly reset expiring within 24h while active does not
	if bestStandby.ResetSecondsWeekly > 0.0 && bestStandby.ResetSecondsWeekly <= 86400.0 {
		if active.ResetSecondsWeekly <= 0.0 || active.ResetSecondsWeekly > 86400.0 {
			return true, fmt.Sprintf("Proactive rotation to harvest standby expiring weekly quota (%s)", bestStandby.Email)
		}
	}

	// Check Proactive Trigger 4: Standby has meaningful harvest score advantage (> 8%)
	scoreActive := ComputeAccountRankingScore(active, threshold, SwitchModeMaxTokens)
	scoreStandby := ComputeAccountRankingScore(bestStandby, threshold, SwitchModeMaxTokens)
	if scoreStandby > scoreActive*1.08 {
		return true, fmt.Sprintf("Proactive rotation: standby harvest score (%.3f) exceeds active (%.3f) by >8%%", scoreStandby, scoreActive)
	}

	return false, "Active account remains optimal for current window"
}

// EvaluateAutoSwitch evaluates whether the active account should switch and identifies the best successor.
func EvaluateAutoSwitch(accounts []AccountQuotaState, activeEmail string, threshold float64, mode string, activeDwellSec float64) (bool, *AccountQuotaState, string) {
	m := NormalizeSwitchMode(mode)
	var active *AccountQuotaState
	for i := range accounts {
		if accounts[i].Email == activeEmail || accounts[i].IsActive {
			active = &accounts[i]
			break
		}
	}

	ranked := RankStandbyAccountsWithMode(accounts, threshold, m)
	if len(ranked) == 0 {
		return false, nil, "No eligible standby accounts above threshold"
	}

	best := &ranked[0]

	// 1. Mandatory Threshold Trigger (applies in all modes)
	if active != nil {
		is5hBreached := active.Quota5hCurrent <= threshold
		isWeeklyBreached := active.QuotaWeekly <= 0.05 && !(active.EnableCreditOverages && active.Credits > 0)
		if is5hBreached || isWeeklyBreached {
			reason := fmt.Sprintf("Active quota (5h: %.1f%%, weekly: %.1f%%) dropped below threshold (%.1f%%)", active.Quota5hCurrent*100, active.QuotaWeekly*100, threshold*100)
			return true, best, reason
		}
	}

	// 2. Proactive rotation only in MaxTokens mode
	if m == SwitchModeMaxTokens && active != nil {
		shouldProactive, reason := ShouldSwitchProactivelyMaxTokens(*active, *best, threshold, activeDwellSec)
		if shouldProactive {
			return true, best, reason
		}
	}

	return false, nil, "Active account quota is healthy"
}

// SortAccountQuotaStates sorts all accounts for UI dashboard display in accordance with sortMode and switchMode.
func SortAccountQuotaStates(accounts []AccountQuotaState, activeEmail string, threshold float64, sortMode string, switchMode string) []AccountQuotaState {
	items := make([]AccountQuotaState, len(accounts))
	copy(items, accounts)

	sMode := strings.ToLower(strings.TrimSpace(sortMode))
	swMode := NormalizeSwitchMode(switchMode)

	if sMode == "identity" {
		sort.Slice(items, func(i, j int) bool {
			nameI := strings.ToLower(items[i].Label)
			if nameI == "" {
				nameI = strings.ToLower(items[i].Email)
			}
			nameJ := strings.ToLower(items[j].Label)
			if nameJ == "" {
				nameJ = strings.ToLower(items[j].Email)
			}
			if nameI != nameJ {
				return nameI < nameJ
			}
			return items[i].Email < items[j].Email
		})
		return items
	}

	if sMode == "priority" {
		sort.Slice(items, func(i, j int) bool {
			pa := PriorityRank(items[i].Priority)
			pb := PriorityRank(items[j].Priority)
			if pa != pb {
				return pa < pb
			}
			diff := items[j].Quota5hAvailable - items[i].Quota5hAvailable
			if math.Abs(diff) > 0.0001 {
				return diff > 0
			}
			return items[j].QuotaWeekly > items[i].QuotaWeekly
		})
		return items
	}

	if sMode == "credits" {
		sort.Slice(items, func(i, j int) bool {
			return items[j].Credits > items[i].Credits
		})
		return items
	}

	if sMode == "quota_5h" {
		sort.Slice(items, func(i, j int) bool {
			diff := items[j].Quota5hAvailable - items[i].Quota5hAvailable
			if math.Abs(diff) > 0.0001 {
				return diff > 0
			}
			return items[j].QuotaWeekly > items[i].QuotaWeekly
		})
		return items
	}

	if sMode == "quota_weekly" {
		sort.Slice(items, func(i, j int) bool {
			diff := items[j].QuotaWeekly - items[i].QuotaWeekly
			if math.Abs(diff) > 0.0001 {
				return diff > 0
			}
			return items[j].Quota5hAvailable > items[i].Quota5hAvailable
		})
		return items
	}

	// sortMode == "auto" (Default)
	// Structural tiers:
	// Tier 0: Active healthy account (Row 1 pinned)
	// Tier 1: Healthy Paid Standby successors above threshold (ordered by switch mode)
	// Tier 2: Healthy Free Standby successors (Free ranked last among eligible standbys)
	// Tier 3: Cooling down / below threshold accounts
	// Tier 4: Error accounts
	// Tier 5: Banned accounts
	getTier := func(a AccountQuotaState) int {
		isAct := a.IsActive || a.Email == activeEmail
		if isAct {
			return 0
		}
		st := strings.ToUpper(a.Status)
		if st == "BANNED" {
			return 5
		}
		if st == "ERROR" {
			return 4
		}
		if st == "COOLDOWN" {
			return 3
		}
		isBelow := a.Quota5hCurrent <= threshold || (a.QuotaWeekly <= 0.05 && !(a.EnableCreditOverages && a.Credits > 0))
		if !isBelow {
			if IsFreePlanTier(a.Email, a.PlanTier) {
				return 2
			}
			return 1
		}
		return 3
	}

	sort.Slice(items, func(i, j int) bool {
		tA := getTier(items[i])
		tB := getTier(items[j])
		if tA != tB {
			return tA < tB
		}

		// Within Tier 1 or Tier 2: compare by switch mode
		if tA == 1 || tA == 2 {
			return CompareStandbyCandidates(items[i], items[j], threshold, swMode)
		}

		// Within Tier 3 (cooling down): prioritize paid over free, then highest available recovery
		if tA == 3 {
			freeI := IsFreePlanTier(items[i].Email, items[i].PlanTier)
			freeJ := IsFreePlanTier(items[j].Email, items[j].PlanTier)
			if freeI != freeJ {
				return !freeI
			}
			if math.Abs(items[i].Quota5hAvailable-items[j].Quota5hAvailable) > 0.001 {
				return items[i].Quota5hAvailable > items[j].Quota5hAvailable
			}
			return items[i].QuotaWeekly > items[j].QuotaWeekly
		}
		return items[i].Email < items[j].Email
	})

	return items
}

