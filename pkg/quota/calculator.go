package quota

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

// AccountQuotaState represents live quota state and credential horizons for an account.
type AccountQuotaState struct {
	Email                string  `json:"email"`
	Label                string  `json:"label"`
	PlanTier             string  `json:"plan_tier"`
	Credits              float64 `json:"credits"`
	EnableCreditOverages bool    `json:"enable_credit_overages"`
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
	Quota5hClaudeGPT     float64 `json:"quota_5h_claude_gpt"`
	QuotaWeeklyClaudeGPT float64 `json:"quota_weekly_claude_gpt"`
	ResetHorizonText     string  `json:"reset_horizon_text"`
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
	h := s / 3600
	m := (s % 3600) / 60
	if h > 0 {
		return fmt.Sprintf("Resets in %dh %dm", h, m)
	}
	return fmt.Sprintf("Resets in %dm", m)
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
			}
		}(acc)
	}

	wg.Wait()
	return results
}

// BuildAccountQuotaStates creates AccountQuotaState items from stored accounts.
func BuildAccountQuotaStates(accounts []*keyring.Account, activeSummary *QuotaSummary) []AccountQuotaState {
	var summaries map[string]*QuotaSummary
	if activeSummary != nil {
		summaries = map[string]*QuotaSummary{
			strings.ToLower(strings.TrimSpace(activeSummary.AccountEmail)): activeSummary,
		}
	}
	return BuildAccountQuotaStatesFromMap(accounts, summaries)
}

// BuildAccountQuotaStatesFromMap builds account states using live summaries and cached agent db.
func BuildAccountQuotaStatesFromMap(accounts []*keyring.Account, summaries map[string]*QuotaSummary) []AccountQuotaState {
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
		cur5hClaude := 1.0
		curWeeklyClaude := 1.0
		resText := "Not Polled"
		tier := acc.PlanTier
		credits := acc.Credits

		// 1. Check live summary first
		if summaries != nil && summaries[normEmail] != nil {
			s := summaries[normEmail]
			if s.PlanTier != "" {
				tier = s.PlanTier
			}
			if s.Credits > 0 {
				credits = s.Credits
			}
			cur5h = s.Quota5hFraction
			curWeekly = s.QuotaWeeklyFraction
			if cur5h == 0 && curWeekly == 0 && len(s.Models) > 0 {
				for _, m := range s.Models {
					mLower := strings.ToLower(m.ModelName)
					if strings.Contains(mLower, "weekly") {
						curWeekly = m.Fraction
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
		} else if ca, ok := cachedMap[normEmail]; ok {
			// 2. Check cached agent DB
			if ca.PlanTier != "" {
				tier = ca.PlanTier
			}
			if ca.Credits > 0 {
				credits = ca.Credits
			}
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
		}

		if tier == "" {
			tier = DetermineDefaultPlanTier(email, acc.PlanTier)
		}
		if credits == 0 {
			lowerTier := strings.ToLower(tier)
			if strings.Contains(lowerTier, "ultra") {
				credits = 50
			} else if strings.Contains(lowerTier, "pro") {
				credits = 20
			}
		}

		avail5h := ComputeEffective5hAvailable(cur5h, curSec)
		if resText == "Not Polled" && curSec > 0 {
			resText = FormatHorizonSec(curSec)
		} else if resText == "Not Polled" && cur5h > 0 {
			resText = "Ready"
		}

		prio := acc.Priority
		if prio == "" {
			prio = "High"
		}

		results = append(results, AccountQuotaState{
			Email:                email,
			Label:                label,
			PlanTier:             tier,
			Credits:              credits,
			EnableCreditOverages: acc.EnableCreditOverages,
			IsActive:             acc.IsActive,
			Status:               status,
			Priority:             prio,
			Notes:                acc.Notes,
			Password:             acc.Password,
			HasTOTP:              acc.HasTOTP,
			TOTPSecret:           acc.TOTPSecret,
			RefreshToken:         acc.RefreshToken,
			Quota5hCurrent:       cur5h,
			ResetSeconds:         curSec,
			QuotaWeekly:          curWeekly,
			Quota5hAvailable:     avail5h,
			Quota5hClaudeGPT:     cur5hClaude,
			QuotaWeeklyClaudeGPT: curWeeklyClaude,
			ResetHorizonText:     resText,
		})
	}

	return results
}

// DetermineDefaultPlanTier computes a membership tier based on explicit tier or email domain/label heuristics.
func DetermineDefaultPlanTier(email string, explicitTier string) string {
	if explicitTier != "" {
		return explicitTier
	}
	lower := strings.ToLower(email)
	if strings.Contains(lower, ".edu") || strings.Contains(lower, "edu.") || strings.Contains(lower, "-edu") {
		return "Edu"
	}
	if strings.Contains(lower, "ultra20") || strings.Contains(lower, "20x") {
		return "Ultra 20X"
	}
	if strings.Contains(lower, "ultra10") || strings.Contains(lower, "10x") {
		return "Ultra 10X"
	}
	if strings.Contains(lower, "ultra5") || strings.Contains(lower, "5x") {
		return "Ultra 5X"
	}
	if strings.Contains(lower, "trial") {
		return "Pro - Trial"
	}
	if strings.Contains(lower, "plus") {
		return "Plus"
	}
	if strings.Contains(lower, "free") {
		return "Free"
	}
	return "Pro"
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

// RankStandbyAccounts sorts standby accounts to find optimal next candidate.
func RankStandbyAccounts(accounts []AccountQuotaState, threshold float64) []AccountQuotaState {
	candidates := make([]AccountQuotaState, 0)
	for _, acc := range accounts {
		if acc.IsActive {
			continue
		}
		st := strings.ToUpper(acc.Status)
		if st == "BANNED" || st == "ERROR" {
			continue
		}
		if acc.Quota5hAvailable <= threshold {
			continue
		}
		candidates = append(candidates, acc)
	}

	sort.Slice(candidates, func(i, j int) bool {
		scoreI := candidates[i].Quota5hAvailable*0.6 + candidates[i].QuotaWeekly*0.4
		scoreJ := candidates[j].Quota5hAvailable*0.6 + candidates[j].QuotaWeekly*0.4
		return scoreI > scoreJ
	})

	return candidates
}
