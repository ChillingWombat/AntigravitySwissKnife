package quota

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

// AccountQuotaState represents live quota state and credential horizons for an account.
type AccountQuotaState struct {
	Email            string  `json:"email"`
	Label            string  `json:"label"`
	PlanTier         string  `json:"plan_tier"`
	IsActive         bool    `json:"is_active"`
	Status           string  `json:"status"`
	Priority         string  `json:"priority,omitempty"`
	Notes            string  `json:"notes,omitempty"`
	Password         string  `json:"password,omitempty"`
	HasTOTP          bool    `json:"has_totp"`
	TOTPSecret       string  `json:"totp_secret"`
	RefreshToken     string  `json:"refresh_token"`
	Quota5hCurrent   float64 `json:"quota_5h_current"`
	ResetSeconds     float64 `json:"reset_seconds"`
	QuotaWeekly      float64 `json:"quota_weekly"`
	Quota5hAvailable float64 `json:"quota_5h_available"`
	ResetHorizonText string  `json:"reset_horizon_text"`
}

// ComputeEffective5hAvailable calculates available quota in the next 5 hours with reset replenishing.
// If hours_until_reset <= 5.0: The account resets within the 5h window.
// Quota refreshes back to 100% (1.0), and the replenished boost is available for (5.0 - h) / 5.0.
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
		return "Resets now"
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
	Fleet5hFraction     float64             `json:"fleet_5h_fraction"`
	FleetWeeklyFraction float64             `json:"fleet_weekly_fraction"`
	TotalAccounts       int                 `json:"total_accounts"`
	ActiveAccount       string              `json:"active_account"`
	Accounts            []AccountQuotaState `json:"accounts"`
}

// ComputeFleetSummary aggregates metrics across all managed accounts.
func ComputeFleetSummary(accounts []AccountQuotaState, activeEmail string) FleetQuotaSummary {
	total := len(accounts)
	if total == 0 {
		return FleetQuotaSummary{
			Fleet5hFraction:     0.0,
			FleetWeeklyFraction: 0.0,
			TotalAccounts:       0,
			ActiveAccount:       activeEmail,
			Accounts:            accounts,
		}
	}

	sum5h := 0.0
	sumWeekly := 0.0
	for _, acc := range accounts {
		sum5h += acc.Quota5hAvailable
		sumWeekly += acc.QuotaWeekly
	}

	return FleetQuotaSummary{
		Fleet5hFraction:     math.Max(0.0, math.Min(1.0, sum5h/float64(total))),
		FleetWeeklyFraction: math.Max(0.0, math.Min(1.0, sumWeekly/float64(total))),
		TotalAccounts:       total,
		ActiveAccount:       activeEmail,
		Accounts:            accounts,
	}
}

// BuildAccountQuotaStates creates AccountQuotaState items from stored accounts.
func BuildAccountQuotaStates(accounts []*keyring.Account, activeSummary *QuotaSummary) []AccountQuotaState {
	results := make([]AccountQuotaState, 0, len(accounts))

	var active5hFrac *float64
	var active5hSec *float64
	var activeWeeklyFrac *float64

	if activeSummary != nil {
		now := time.Now()
		for _, m := range activeSummary.Models {
			name := strings.ToLower(m.ModelName)
			if strings.Contains(name, "weekly") || strings.Contains(name, "7d") {
				frac := m.Fraction
				activeWeeklyFrac = &frac
			} else if strings.Contains(name, "gemini") || strings.Contains(name, "5h") || active5hFrac == nil {
				frac := m.Fraction
				active5hFrac = &frac
				if !m.ResetTime.IsZero() && m.ResetTime.After(now) {
					diff := m.ResetTime.Sub(now).Seconds()
					active5hSec = &diff
				}
			}
		}
		if activeWeeklyFrac == nil && active5hFrac != nil {
			activeWeeklyFrac = active5hFrac
		}
	}

	for _, acc := range accounts {
		email := strings.TrimSpace(acc.Email)
		if email == "" {
			continue
		}

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

		// Real baseline: unpolled accounts start at 0.0 unless actively polled. Zero artificial data.
		cur5h := 0.0
		curSec := 0.0
		curWeekly := 0.0

		if (acc.IsActive || (activeSummary != nil && strings.EqualFold(acc.Email, activeSummary.AccountEmail))) && active5hFrac != nil {
			cur5h = *active5hFrac
			if active5hSec != nil {
				curSec = *active5hSec
			}
			if activeWeeklyFrac != nil {
				curWeekly = *activeWeeklyFrac
			}
		}

		avail5h := ComputeEffective5hAvailable(cur5h, curSec)
		resText := "Not Polled"
		if curSec > 0 {
			resText = FormatHorizonSec(curSec)
		} else if cur5h > 0 {
			resText = "Ready"
		}
		tier := DetermineDefaultPlanTier(email, acc.PlanTier)

		prio := acc.Priority
		if prio == "" {
			prio = "High"
		}

		results = append(results, AccountQuotaState{
			Email:            email,
			Label:            label,
			PlanTier:         tier,
			IsActive:         acc.IsActive,
			Status:           status,
			Priority:         prio,
			Notes:            acc.Notes,
			Password:         acc.Password,
			HasTOTP:          acc.HasTOTP,
			TOTPSecret:       acc.TOTPSecret,
			RefreshToken:     acc.RefreshToken,
			Quota5hCurrent:   cur5h,
			ResetSeconds:     curSec,
			QuotaWeekly:      curWeekly,
			Quota5hAvailable: avail5h,
			ResetHorizonText: resText,
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
		strings.Contains(combined, "user_suspended") {
		return "BANNED"
	}
	if statusCode == 401 || statusCode == 403 ||
		strings.Contains(combined, "invalid_grant") ||
		strings.Contains(combined, "invalid_token") ||
		strings.Contains(combined, "unauthenticated") ||
		strings.Contains(combined, "interaction_required") ||
		strings.Contains(combined, "challenge") ||
		strings.Contains(combined, "verification") ||
		strings.Contains(combined, "reauth") ||
		strings.Contains(combined, "expired") {
		return "ERROR"
	}
	return "ERROR"
}

// SortAccountQuotaStates sorts a slice of AccountQuotaState based on the mode:
// - "auto": Active healthy in row 1, best standby continuous usage successors next,
//   cooling/below-threshold accounts near end, and error/banned at bottom.
// - "identity": Alphabetical by label or email.
// - "quota_5h": 5H quota available descending.
// - "quota_weekly": Weekly quota descending.
func SortAccountQuotaStates(accounts []AccountQuotaState, activeEmail string, threshold float64, mode string) []AccountQuotaState {
	res := make([]AccountQuotaState, len(accounts))
	copy(res, accounts)

	if threshold <= 0 {
		threshold = 0.10
	}

	switch mode {
	case "identity":
		sort.SliceStable(res, func(i, j int) bool {
			nameI := strings.ToLower(res[i].Label)
			if nameI == "" {
				nameI = strings.ToLower(res[i].Email)
			}
			nameJ := strings.ToLower(res[j].Label)
			if nameJ == "" {
				nameJ = strings.ToLower(res[j].Email)
			}
			if nameI != nameJ {
				return nameI < nameJ
			}
			return strings.ToLower(res[i].Email) < strings.ToLower(res[j].Email)
		})
	case "quota_5h":
		sort.SliceStable(res, func(i, j int) bool {
			diff := res[i].Quota5hAvailable - res[j].Quota5hAvailable
			if math.Abs(diff) > 0.0001 {
				return diff > 0
			}
			return res[i].QuotaWeekly > res[j].QuotaWeekly
		})
	case "quota_weekly":
		sort.SliceStable(res, func(i, j int) bool {
			diff := res[i].QuotaWeekly - res[j].QuotaWeekly
			if math.Abs(diff) > 0.0001 {
				return diff > 0
			}
			return res[i].Quota5hAvailable > res[j].Quota5hAvailable
		})
	case "auto":
		fallthrough
	default:
		sort.SliceStable(res, func(i, j int) bool {
			getTier := func(a *AccountQuotaState) int {
				st := strings.ToUpper(a.Status)
				if st == "BANNED" {
					return 4
				}
				if st == "ERROR" {
					return 3
				}
				isAct := a.IsActive || (a.Email == activeEmail)
				isBelow := a.Quota5hAvailable <= threshold || a.QuotaWeekly <= 0.05
				if isAct && !isBelow {
					return 0
				}
				if !isAct && !isBelow {
					return 1
				}
				return 2
			}

			tierI := getTier(&res[i])
			tierJ := getTier(&res[j])
			if tierI != tierJ {
				return tierI < tierJ
			}

			if tierI == 1 {
				prioRank := func(p string) int {
					switch strings.ToUpper(strings.TrimSpace(p)) {
					case "HIGH":
						return 0
					case "MID":
						return 1
					case "LOW":
						return 2
					default:
						return 0
					}
				}
				prioI := prioRank(res[i].Priority)
				prioJ := prioRank(res[j].Priority)
				if prioI != prioJ {
					return prioI < prioJ
				}

				scoreI := res[i].Quota5hAvailable*0.6 + res[i].QuotaWeekly*0.4
				scoreJ := res[j].Quota5hAvailable*0.6 + res[j].QuotaWeekly*0.4
				if math.Abs(scoreI-scoreJ) > 0.001 {
					return scoreI > scoreJ
				}
				if math.Abs(res[i].Quota5hAvailable-res[j].Quota5hAvailable) > 0.001 {
					return res[i].Quota5hAvailable > res[j].Quota5hAvailable
				}
				return res[i].QuotaWeekly > res[j].QuotaWeekly
			}

			if tierI == 2 {
				if math.Abs(res[i].Quota5hAvailable-res[j].Quota5hAvailable) > 0.001 {
					return res[i].Quota5hAvailable > res[j].Quota5hAvailable
				}
				return res[i].QuotaWeekly > res[j].QuotaWeekly
			}

			nameI := strings.ToLower(res[i].Label)
			nameJ := strings.ToLower(res[j].Label)
			return nameI < nameJ
		})
	}

	return res
}


