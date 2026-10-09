package quota

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

func TestEffective5hAvailable(t *testing.T) {
	// Account with 50% remaining and resets in 1 hour
	// Should get 50% + 50% * (4/5) = 90%
	avail := ComputeEffective5hAvailable(0.50, 3600.0)
	if avail < 0.89 || avail > 0.91 {
		t.Errorf("expected ~0.90, got %f", avail)
	}

	// Account resets in 6 hours (outside 5h window)
	// Should stay at current 50%
	avail6h := ComputeEffective5hAvailable(0.50, 6.0*3600.0)
	if avail6h != 0.50 {
		t.Errorf("expected 0.50, got %f", avail6h)
	}
}

func TestBuildAccountQuotaStatesAndSummary(t *testing.T) {
	accs := []*keyring.Account{
		{Email: "acc1@google.com", Label: "Dev Primary", IsActive: true},
		{Email: "acc2@google.com", Label: "Backup", IsActive: false},
	}

	summary := &QuotaSummary{
		AccountEmail: "acc1@google.com",
		Models: []ModelQuota{
			{ModelName: "gemini-2.5-pro", Fraction: 0.80, ResetTime: time.Now().Add(2 * time.Hour)},
		},
	}

	states := BuildAccountQuotaStates(accs, summary)
	if len(states) != 2 {
		t.Fatalf("expected 2 states, got %d", len(states))
	}

	fleet := ComputeFleetSummary(states, "acc1@google.com")
	if fleet.TotalAccounts != 2 {
		t.Errorf("expected 2 accounts, got %d", fleet.TotalAccounts)
	}
	if fleet.Fleet5hFraction <= 0 || fleet.Fleet5hFraction > 1.0 {
		t.Errorf("expected fleet 5h between 0 and 1, got %f", fleet.Fleet5hFraction)
	}
	if fleet.FleetWeeklyFraction <= 0 || fleet.FleetWeeklyFraction > 1.0 {
		t.Errorf("expected fleet weekly between 0 and 1, got %f", fleet.FleetWeeklyFraction)
	}
}

func TestDetermineDefaultPlanTier(t *testing.T) {
	cases := []struct {
		email        string
		explicitTier string
		want         string
	}{
		{"user@stanford.edu", "", "Edu"},
		{"student@univ-edu.org", "", "Edu"},
		{"free_user@gmail.com", "", "Free"},
		{"plus_account@gmail.com", "", "Plus"},
		{"pro_account@gmail.com", "", "Pro"},
		{"trial_tester@gmail.com", "", "Pro - Trial"},
		{"heavy_user_5x@gmail.com", "", "Ultra 5X"},
		{"ultra10_corp@gmail.com", "", "Ultra 10X"},
		{"max_ultra20x@gmail.com", "", "Ultra 20X"},
		// Explicit override takes precedence and normalizes legacy strings
		{"user@stanford.edu", "Ultra 20X", "Ultra 20X"},
		{"random@gmail.com", "Pro - Trial", "Pro - Trial"},
		{"random@gmail.com", "Google AI Pro", "Pro - Trial"},
		{"random@gmail.com", "Google AI Ultra", "Ultra 20X"},
		{"random@gmail.com", "", "Free"}, // fallback defaults to Free
	}

	for _, tc := range cases {
		got := DetermineDefaultPlanTier(tc.email, tc.explicitTier)
		if got != tc.want {
			t.Errorf("DetermineDefaultPlanTier(%q, %q) = %q, want %q", tc.email, tc.explicitTier, got, tc.want)
		}
	}
}

func TestNormalizePlanTier(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "Free"},
		{"free", "Free"},
		{"Free", "Free"},
		{"plus", "Plus"},
		{"Plus", "Plus"},
		{"pro", "Pro"},
		{"Pro", "Pro"},
		{"Google AI Pro", "Pro - Trial"},
		{"PRO", "Pro"},
		{"Pro - Trial", "Pro - Trial"},
		{"trial", "Pro - Trial"},
		{"Pro Trial", "Pro - Trial"},
		{"edu", "Edu"},
		{"Edu", "Edu"},
		{"Ultra 5X", "Ultra 5X"},
		{"5x", "Ultra 5X"},
		{"Ultra 10X", "Ultra 10X"},
		{"10x", "Ultra 10X"},
		{"Ultra 20X", "Ultra 20X"},
		{"20x", "Ultra 20X"},
		{"Google AI Ultra", "Ultra 20X"},
		{"Ultra", "Ultra 20X"},
	}

	for _, tt := range tests {
		got := NormalizePlanTier(tt.input)
		if got != tt.want {
			t.Errorf("NormalizePlanTier(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRankStandbyAccounts_Guardrails(t *testing.T) {
	accounts := []AccountQuotaState{
		{
			Email:            "active@example.com",
			IsActive:         true,
			Status:           "ACTIVE",
			Quota5hCurrent:   0.90,
			QuotaWeekly:      0.90,
			Quota5hAvailable: 0.90,
		},
		{
			// 0% current quota, but replenishes in 1 hour (available: 80%)
			// MUST be excluded by immediate capacity guardrail
			Email:            "exhausted_current@example.com",
			IsActive:         false,
			Status:           "STANDBY",
			Quota5hCurrent:   0.00,
			QuotaWeekly:      0.90,
			Quota5hAvailable: 0.80,
		},
		{
			// Weekly depleted without credits
			Email:            "depleted_weekly@example.com",
			IsActive:         false,
			Status:           "STANDBY",
			Quota5hCurrent:   0.80,
			QuotaWeekly:      0.02,
			Quota5hAvailable: 0.80,
		},
		{
			// Weekly depleted WITH credits and overages enabled
			Email:                "overages_allowed@example.com",
			IsActive:             false,
			Status:               "STANDBY",
			Quota5hCurrent:       0.70,
			QuotaWeekly:          0.02,
			Quota5hAvailable:     0.70,
			Credits:              15.0,
			EnableCreditOverages: true,
		},
		{
			// Healthy candidate with high capacity
			Email:            "healthy_top@example.com",
			IsActive:         false,
			Status:           "STANDBY",
			Quota5hCurrent:   0.95,
			QuotaWeekly:      0.90,
			Quota5hAvailable: 0.95,
		},
		{
			// Healthy candidate with mid capacity
			Email:            "healthy_mid@example.com",
			IsActive:         false,
			Status:           "STANDBY",
			Quota5hCurrent:   0.50,
			QuotaWeekly:      0.60,
			Quota5hAvailable: 0.50,
		},
		{
			Email:            "broken_err@example.com",
			IsActive:         false,
			Status:           "ERROR",
			Quota5hCurrent:   0.90,
			QuotaWeekly:      0.90,
			Quota5hAvailable: 0.90,
		},
		{
			Email:            "broken_ban@example.com",
			IsActive:         false,
			Status:           "BANNED",
			Quota5hCurrent:   0.90,
			QuotaWeekly:      0.90,
			Quota5hAvailable: 0.90,
		},
	}

	ranked := RankStandbyAccounts(accounts, 0.10)

	// Expected order: healthy_top (score ~0.93), healthy_mid (score 0.54), overages_allowed (score 0.428)
	if len(ranked) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(ranked))
	}
	if ranked[0].Email != "healthy_top@example.com" {
		t.Errorf("expected #1 to be healthy_top@example.com, got %s", ranked[0].Email)
	}
	if ranked[1].Email != "healthy_mid@example.com" {
		t.Errorf("expected #2 to be healthy_mid@example.com, got %s", ranked[1].Email)
	}
	if ranked[2].Email != "overages_allowed@example.com" {
		t.Errorf("expected #3 to be overages_allowed@example.com, got %s", ranked[2].Email)
	}
}

func TestMultiDayHorizonFormatting(t *testing.T) {
	// FormatHorizonSec
	h1 := FormatHorizonSec(3600 * 2)
	if h1 != "Resets in 2h 0m" {
		t.Errorf("expected 'Resets in 2h 0m', got %q", h1)
	}
	h2 := FormatHorizonSec(86400*3 + 3600*5)
	if h2 != "Resets in 3d 5h" {
		t.Errorf("expected 'Resets in 3d 5h', got %q", h2)
	}

	// FormatResetHorizon
	now := time.Now()
	f1 := FormatResetHorizon(now.Add(2*time.Hour+15*time.Minute), now)
	if f1 != "Resets in 2h 15m" {
		t.Errorf("expected 'Resets in 2h 15m', got %q", f1)
	}
	f2 := FormatResetHorizon(now.Add(5*24*time.Hour+3*time.Hour), now)
	if f2 != "Resets in 5d 3h" {
		t.Errorf("expected 'Resets in 5d 3h', got %q", f2)
	}
}

func TestSwitchModesAndPlanTierRanking(t *testing.T) {
	// 1. Free accounts demoted strictly behind healthy paid accounts
	accFree := AccountQuotaState{
		Email:            "free@example.com",
		Label:            "Free User",
		PlanTier:         "Free",
		Status:           "STANDBY",
		Quota5hCurrent:   1.0,
		Quota5hAvailable: 1.0,
		QuotaWeekly:      1.0,
	}
	accPro := AccountQuotaState{
		Email:            "pro@example.com",
		Label:            "Pro User",
		PlanTier:         "Pro",
		Status:           "STANDBY",
		Quota5hCurrent:   0.60,
		Quota5hAvailable: 0.60,
		QuotaWeekly:      0.70,
	}

	ranked := RankStandbyAccountsWithMode([]AccountQuotaState{accFree, accPro}, 0.10, SwitchModeBalanced)
	if len(ranked) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(ranked))
	}
	if ranked[0].Email != "pro@example.com" {
		t.Errorf("expected paid Pro account to rank above 100%% Free account, got %s", ranked[0].Email)
	}

	// 2. MaxContinuous mode: Ultra 20X preferred over Pro with same remaining quota
	accUltra := AccountQuotaState{
		Email:            "ultra@example.com",
		Label:            "Ultra User",
		PlanTier:         "Ultra 20X",
		Status:           "STANDBY",
		Quota5hCurrent:   0.90,
		Quota5hAvailable: 0.90,
		QuotaWeekly:      0.90,
	}
	accProFull := AccountQuotaState{
		Email:            "pro_full@example.com",
		Label:            "Pro Full",
		PlanTier:         "Pro",
		Status:           "STANDBY",
		Quota5hCurrent:   0.90,
		Quota5hAvailable: 0.90,
		QuotaWeekly:      0.90,
	}

	rankedCont := RankStandbyAccountsWithMode([]AccountQuotaState{accProFull, accUltra}, 0.10, SwitchModeMaxContinuous)
	if len(rankedCont) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(rankedCont))
	}
	if rankedCont[0].Email != "ultra@example.com" {
		t.Errorf("expected Ultra 20X to rank above Pro in max_continuous, got %s", rankedCont[0].Email)
	}

	// 3. MaxTokens mode: Standby with earlier expiring 5h window preferred
	accResetSoon := AccountQuotaState{
		Email:            "reset_soon@example.com",
		Label:            "Reset Soon",
		PlanTier:         "Pro",
		Status:           "STANDBY",
		Quota5hCurrent:   0.80,
		ResetSeconds:     1800.0, // Resets in 30m
		Quota5hAvailable: 0.96,
		QuotaWeekly:      0.80,
	}
	accResetLate := AccountQuotaState{
		Email:            "reset_late@example.com",
		Label:            "Reset Late",
		PlanTier:         "Pro",
		Status:           "STANDBY",
		Quota5hCurrent:   0.80,
		ResetSeconds:     14400.0, // Resets in 4h
		Quota5hAvailable: 0.84,
		QuotaWeekly:      0.80,
	}

	rankedTokens := RankStandbyAccountsWithMode([]AccountQuotaState{accResetLate, accResetSoon}, 0.10, SwitchModeMaxTokens)
	if len(rankedTokens) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(rankedTokens))
	}
	if rankedTokens[0].Email != "reset_soon@example.com" {
		t.Errorf("expected account with earlier expiring reset window to rank first in max_tokens, got %s", rankedTokens[0].Email)
	}
}

func TestEvaluateAutoSwitch_ModesAndDwell(t *testing.T) {
	active := AccountQuotaState{
		Email:            "active@example.com",
		IsActive:         true,
		PlanTier:         "Pro",
		Status:           "ACTIVE",
		Quota5hCurrent:   0.80,
		ResetSeconds:     7200.0,
		QuotaWeekly:      0.85,
		Quota5hAvailable: 0.80,
	}
	standbyUnstarted := AccountQuotaState{
		Email:            "standby_ready@example.com",
		PlanTier:         "Pro",
		Status:           "STANDBY",
		Quota5hCurrent:   1.0,
		ResetSeconds:     0.0, // Unstarted clock
		QuotaWeekly:      0.90,
		Quota5hAvailable: 1.0,
	}

	accounts := []AccountQuotaState{active, standbyUnstarted}

	// Case 1: Balanced mode with healthy quota does NOT switch
	shouldSwitch, _, _ := EvaluateAutoSwitch(accounts, active.Email, 0.05, SwitchModeBalanced, 1200.0)
	if shouldSwitch {
		t.Errorf("balanced mode should NOT switch healthy account")
	}

	// Case 2: MaxTokens mode with dwell < 600s does NOT switch
	shouldSwitchShortDwell, _, _ := EvaluateAutoSwitch(accounts, active.Email, 0.05, SwitchModeMaxTokens, 300.0)
	if shouldSwitchShortDwell {
		t.Errorf("max_tokens mode should NOT switch if dwell < 600s")
	}

	// Case 3: MaxTokens mode with dwell >= 600s DOES ignite idle standby clock
	shouldSwitchIgnite, succ, reason := EvaluateAutoSwitch(accounts, active.Email, 0.05, SwitchModeMaxTokens, 700.0)
	if !shouldSwitchIgnite || succ == nil || succ.Email != "standby_ready@example.com" {
		t.Errorf("expected proactive switch to ignite standby clock, got shouldSwitch=%v, succ=%v, reason=%s", shouldSwitchIgnite, succ, reason)
	}

	// Case 4: MaxTokens mode does NOT switch if active is about to reset (<= 45m / 2700s)
	activeImminentReset := active
	activeImminentReset.ResetSeconds = 1200.0 // 20m remaining
	accountsImminent := []AccountQuotaState{activeImminentReset, standbyUnstarted}
	shouldSwitchImminent, _, _ := EvaluateAutoSwitch(accountsImminent, active.Email, 0.05, SwitchModeMaxTokens, 800.0)
	if shouldSwitchImminent {
		t.Errorf("max_tokens mode should NOT switch away if active is about to reset soon (harvesting expiring quota)")
	}

	// Case 5: Threshold breach switches in all modes
	activeExhausted := active
	activeExhausted.Quota5hCurrent = 0.03 // <= 0.05
	accountsExhausted := []AccountQuotaState{activeExhausted, standbyUnstarted}
	shouldSwitchBreach, succBreach, _ := EvaluateAutoSwitch(accountsExhausted, active.Email, 0.05, SwitchModeMaxContinuous, 50.0)
	if !shouldSwitchBreach || succBreach == nil || succBreach.Email != "standby_ready@example.com" {
		t.Errorf("threshold breach must trigger auto switch in all modes")
	}
}

func TestSortAccountQuotaStates_AllTiers(t *testing.T) {
	active := AccountQuotaState{Email: "active@example.com", IsActive: true, PlanTier: "Pro", Quota5hCurrent: 0.8, Quota5hAvailable: 0.8, QuotaWeekly: 0.8}
	ultraStandby := AccountQuotaState{Email: "ultra@example.com", PlanTier: "Ultra 20X", Quota5hCurrent: 0.9, Quota5hAvailable: 0.9, QuotaWeekly: 0.9}
	proStandby := AccountQuotaState{Email: "pro@example.com", PlanTier: "Pro", Quota5hCurrent: 0.85, Quota5hAvailable: 0.85, QuotaWeekly: 0.85}
	freeStandby := AccountQuotaState{Email: "free@example.com", PlanTier: "Free", Quota5hCurrent: 1.0, Quota5hAvailable: 1.0, QuotaWeekly: 1.0}
	cooling := AccountQuotaState{Email: "cooling@example.com", PlanTier: "Pro", Quota5hCurrent: 0.02, Quota5hAvailable: 0.8, QuotaWeekly: 0.8}
	errAcc := AccountQuotaState{Email: "err@example.com", Status: "ERROR", Quota5hCurrent: 1.0, QuotaWeekly: 1.0}
	banAcc := AccountQuotaState{Email: "ban@example.com", Status: "BANNED", Quota5hCurrent: 1.0, QuotaWeekly: 1.0}

	list := []AccountQuotaState{banAcc, freeStandby, proStandby, errAcc, cooling, active, ultraStandby}
	sorted := SortAccountQuotaStates(list, "active@example.com", 0.05, "auto", SwitchModeMaxContinuous)

	if len(sorted) != 7 {
		t.Fatalf("expected 7 items, got %d", len(sorted))
	}
	// Row 0: Active
	if sorted[0].Email != "active@example.com" {
		t.Errorf("expected row 0 = active, got %s", sorted[0].Email)
	}
	// Row 1: ultraStandby (paid top candidate)
	if sorted[1].Email != "ultra@example.com" {
		t.Errorf("expected row 1 = ultra, got %s", sorted[1].Email)
	}
	// Row 2: proStandby (paid second candidate)
	if sorted[2].Email != "pro@example.com" {
		t.Errorf("expected row 2 = pro, got %s", sorted[2].Email)
	}
	// Row 3: freeStandby (Tier 2: Free candidates placed strictly after paid)
	if sorted[3].Email != "free@example.com" {
		t.Errorf("expected row 3 = free, got %s", sorted[3].Email)
	}
	// Row 4: cooling
	if sorted[4].Email != "cooling@example.com" {
		t.Errorf("expected row 4 = cooling, got %s", sorted[4].Email)
	}
	// Row 5: error
	if sorted[5].Email != "err@example.com" {
		t.Errorf("expected row 5 = err, got %s", sorted[5].Email)
	}
	// Row 6: banned
	if sorted[6].Email != "ban@example.com" {
		t.Errorf("expected row 6 = ban, got %s", sorted[6].Email)
	}
}

func TestCooldownAccountLifecycleAndRanking(t *testing.T) {
	// 1. Account quota below threshold enters COOLDOWN
	acc := &keyring.Account{
		Email:    "cooling_test@example.com",
		Label:    "Cooling Test",
		PlanTier: "Pro",
		Status:   "STANDBY",
		IsActive: false,
	}

	depletedSummary := &QuotaSummary{
		AccountEmail:        "cooling_test@example.com",
		PlanTier:            "Pro",
		Quota5hFraction:     0.03, // below 0.05 threshold
		QuotaWeeklyFraction: 0.50,
	}

	summaries := map[string]*QuotaSummary{
		"cooling_test@example.com": depletedSummary,
	}

	states := BuildAccountQuotaStatesFromMapWithThreshold([]*keyring.Account{acc}, summaries, 0.05)
	if len(states) != 1 {
		t.Fatalf("expected 1 state, got %d", len(states))
	}
	if states[0].Status != core.AccountStatusCooling {
		t.Fatalf("expected status %s when quota below threshold, got %s", core.AccountStatusCooling, states[0].Status)
	}

	// 2. Returns to STANDBY when quota recovers above threshold (for both COOLING and legacy COOLDOWN)
	accCooling := &keyring.Account{
		Email:    "cooling_test@example.com",
		Label:    "Cooling Test",
		PlanTier: "Pro",
		Status:   core.AccountStatusCooling,
		IsActive: false,
	}
	accLegacyCooldown := &keyring.Account{
		Email:    "legacy_cooldown@example.com",
		Label:    "Legacy Cooldown Test",
		PlanTier: "Pro",
		Status:   core.AccountStatusCooldown,
		IsActive: false,
	}

	recoveredSummary := &QuotaSummary{
		AccountEmail:        "cooling_test@example.com",
		PlanTier:            "Pro",
		Quota5hFraction:     0.90, // recovered above threshold
		QuotaWeeklyFraction: 0.85,
	}
	recoveredLegacySummary := &QuotaSummary{
		AccountEmail:        "legacy_cooldown@example.com",
		PlanTier:            "Pro",
		Quota5hFraction:     0.90,
		QuotaWeeklyFraction: 0.85,
	}

	summariesRecovered := map[string]*QuotaSummary{
		"cooling_test@example.com":    recoveredSummary,
		"legacy_cooldown@example.com": recoveredLegacySummary,
	}

	statesRecovered := BuildAccountQuotaStatesFromMapWithThreshold([]*keyring.Account{accCooling, accLegacyCooldown}, summariesRecovered, 0.05)
	if len(statesRecovered) != 2 {
		t.Fatalf("expected 2 states, got %d", len(statesRecovered))
	}
	for _, st := range statesRecovered {
		if st.Status != core.AccountStatusStandby {
			t.Fatalf("expected status %s after quota resets for %s, got %s", core.AccountStatusStandby, st.Email, st.Status)
		}
	}

	// Legacy COOLDOWN status in keyring store without polled data is normalized to COOLING
	accUnpolledLegacy := &keyring.Account{
		Email:    "unpolled@example.com",
		Label:    "Unpolled Legacy",
		PlanTier: "Pro",
		Status:   core.AccountStatusCooldown,
		IsActive: false,
	}
	statesLegacy := BuildAccountQuotaStatesFromMapWithThreshold([]*keyring.Account{accUnpolledLegacy}, nil, 0.05)
	if len(statesLegacy) != 1 || statesLegacy[0].Status != core.AccountStatusCooling {
		t.Fatalf("expected legacy COOLDOWN to be normalized to COOLING, got %v", statesLegacy[0].Status)
	}

	// 3. RankStandbyAccounts and RankStandbyAccountsWithMode strictly exclude COOLDOWN accounts
	standbyHealthy := AccountQuotaState{
		Email:           "healthy@example.com",
		Status:          core.AccountStatusStandby,
		IsActive:        false,
		PlanTier:        "Pro",
		Quota5hCurrent:  0.80,
		QuotaWeekly:     0.80,
	}
	standbyCooldown := AccountQuotaState{
		Email:           "cooldown@example.com",
		Status:          core.AccountStatusCooldown,
		IsActive:        false,
		PlanTier:        "Pro",
		Quota5hCurrent:  0.95, // High quota should still be excluded if status is COOLDOWN
		QuotaWeekly:     0.95,
	}

	candidates := RankStandbyAccounts([]AccountQuotaState{standbyCooldown, standbyHealthy}, 0.05)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].Email != "healthy@example.com" {
		t.Fatalf("expected healthy@example.com candidate, got %s", candidates[0].Email)
	}

	// Verify all switch modes exclude COOLDOWN
	for _, mode := range []string{SwitchModeBalanced, SwitchModeMaxTokens, SwitchModeMaxContinuous} {
		mCandidates := RankStandbyAccountsWithMode([]AccountQuotaState{standbyCooldown, standbyHealthy}, 0.05, mode)
		if len(mCandidates) != 1 || mCandidates[0].Email != "healthy@example.com" {
			t.Fatalf("mode %s must exclude COOLDOWN accounts, got %v", mode, mCandidates)
		}
	}

	standbyCooling := AccountQuotaState{
		Email:          "cooling_cand@example.com",
		Status:         core.AccountStatusCooling,
		IsActive:       false,
		PlanTier:       "Pro",
		Quota5hCurrent: 0.95,
		QuotaWeekly:    0.95,
	}
	candCooling := RankStandbyAccounts([]AccountQuotaState{standbyCooling, standbyHealthy}, 0.05)
	if len(candCooling) != 1 || candCooling[0].Email != "healthy@example.com" {
		t.Fatalf("expected COOLING account to be excluded from standby candidates, got %v", candCooling)
	}

	// 4. EvaluateAutoSwitch excludes COOLDOWN accounts
	activeExhausted := AccountQuotaState{
		Email:           "active@example.com",
		Status:          core.AccountStatusActive,
		IsActive:        true,
		PlanTier:        "Pro",
		Quota5hCurrent:  0.02,
		QuotaWeekly:     0.50,
	}

	// When only COOLDOWN accounts exist, should not switch
	shouldSwitch, succ, reason := EvaluateAutoSwitch([]AccountQuotaState{activeExhausted, standbyCooldown}, activeExhausted.Email, 0.05, SwitchModeBalanced, 100)
	if shouldSwitch || succ != nil {
		t.Fatalf("expected no switch when only COOLDOWN standby exists, got shouldSwitch=%v, succ=%v, reason=%s", shouldSwitch, succ, reason)
	}

	// When both COOLDOWN and healthy exist, selects healthy
	shouldSwitch2, succ2, _ := EvaluateAutoSwitch([]AccountQuotaState{activeExhausted, standbyCooldown, standbyHealthy}, activeExhausted.Email, 0.05, SwitchModeBalanced, 100)
	if !shouldSwitch2 || succ2 == nil || succ2.Email != "healthy@example.com" {
		t.Fatalf("expected switch to healthy@example.com, got succ=%v", succ2)
	}

	// 5. SortAccountQuotaStates ranks COOLDOWN accounts in Tier 3
	cooldownState := AccountQuotaState{
		Email:            "cooldown@example.com",
		Status:           core.AccountStatusCooldown,
		PlanTier:         "Pro",
		Quota5hCurrent:   0.02,
		Quota5hAvailable: 0.8,
		QuotaWeekly:      0.8,
	}
	sortedList := SortAccountQuotaStates([]AccountQuotaState{standbyHealthy, cooldownState}, "", 0.05, "auto", SwitchModeBalanced)
	if len(sortedList) != 2 || sortedList[0].Email != "healthy@example.com" || sortedList[1].Email != "cooldown@example.com" {
		t.Fatalf("expected healthy before cooldown in auto sort, got %v", sortedList)
	}

	// 6. Active account below threshold is pinned to Tier 0 and does not enter COOLDOWN
	activeAcc := &keyring.Account{
		Email:    "active_low@example.com",
		Label:    "Active Low",
		PlanTier: "Pro",
		Status:   "ACTIVE",
		IsActive: true,
	}
	activeLowSummary := &QuotaSummary{
		AccountEmail:    "active_low@example.com",
		PlanTier:        "Pro",
		Quota5hFraction: 0.01,
	}
	activeStates := BuildAccountQuotaStatesWithThreshold([]*keyring.Account{activeAcc}, activeLowSummary, 0.05)
	if len(activeStates) != 1 || activeStates[0].Status == core.AccountStatusCooldown {
		t.Fatalf("active account must never enter COOLDOWN status, got %s", activeStates[0].Status)
	}

	sortedWithActiveLow := SortAccountQuotaStates([]AccountQuotaState{standbyHealthy, activeStates[0]}, activeStates[0].Email, 0.05, "auto", SwitchModeBalanced)
	if len(sortedWithActiveLow) != 2 || sortedWithActiveLow[0].Email != "active_low@example.com" {
		t.Fatalf("active account even with low quota must remain pinned at Row 0, got %v", sortedWithActiveLow)
	}
}

func TestWeeklyThreshold_EvaluationAndExclusion(t *testing.T) {
	// 1. Active account has 100% 5h quota, but weekly quota is 4% (0.04)
	activeWeeklyLow := AccountQuotaState{
		Email:            "active_weekly_low@example.com",
		IsActive:         true,
		PlanTier:         "Pro",
		Status:           "ACTIVE",
		Quota5hCurrent:   1.0,
		Quota5hAvailable: 1.0,
		QuotaWeekly:      0.04, // 4%, below 5% weekly threshold
	}
	standbyHealthy := AccountQuotaState{
		Email:            "standby_healthy@example.com",
		IsActive:         false,
		PlanTier:         "Pro",
		Status:           "STANDBY",
		Quota5hCurrent:   0.9,
		Quota5hAvailable: 0.9,
		QuotaWeekly:      0.8,
	}

	// With weekly threshold at 0.05, switch should trigger even though 5h quota is 100%
	shouldSwitch, succ, reason := EvaluateAutoSwitchWithThresholds(
		[]AccountQuotaState{activeWeeklyLow, standbyHealthy},
		activeWeeklyLow.Email,
		0.05,
		0.05,
		SwitchModeBalanced,
		600.0,
	)
	if !shouldSwitch {
		t.Fatalf("expected auto switch when weekly quota is below threshold (4%% <= 5%%)")
	}
	if succ == nil || succ.Email != standbyHealthy.Email {
		t.Fatalf("expected successor %s, got %v", standbyHealthy.Email, succ)
	}
	if !strings.Contains(reason, "weekly") {
		t.Fatalf("expected reason to mention weekly quota, got %q", reason)
	}

	// 2. Standby candidate with weekly quota below threshold is excluded from candidates
	standbyWeeklyLow := AccountQuotaState{
		Email:            "standby_weekly_low@example.com",
		IsActive:         false,
		PlanTier:         "Pro",
		Status:           "STANDBY",
		Quota5hCurrent:   1.0,
		QuotaWeekly:      0.04, // below weekly threshold 0.05
	}
	candidates := RankStandbyAccountsWithThresholds(
		[]AccountQuotaState{standbyWeeklyLow, standbyHealthy},
		0.05,
		0.05,
		SwitchModeBalanced,
	)
	if len(candidates) != 1 || candidates[0].Email != standbyHealthy.Email {
		t.Fatalf("expected only healthy standby candidate, got %v", candidates)
	}

	// 3. Account with weekly quota below threshold enters COOLDOWN via BuildAccountQuotaStatesFromMapWithThresholds
	acc := &keyring.Account{
		Email:    "test_weekly_cooldown@example.com",
		Label:    "Weekly Cooldown Account",
		PlanTier: "Pro",
		Status:   "STANDBY",
	}
	summaries := map[string]*QuotaSummary{
		"test_weekly_cooldown@example.com": {
			AccountEmail:        "test_weekly_cooldown@example.com",
			PlanTier:            "Pro",
			Quota5hFraction:     1.0,
			QuotaWeeklyFraction: 0.03, // 3%, below 0.05
		},
	}
	states := BuildAccountQuotaStatesFromMapWithThresholds([]*keyring.Account{acc}, summaries, 0.05, 0.05)
	if len(states) != 1 || states[0].Status != core.AccountStatusCooling {
		t.Fatalf("expected status %s for account with weekly quota below threshold, got %s", core.AccountStatusCooling, states[0].Status)
	}

	// 4. Auto sorting places weekly-depleted standby in Tier 3 (cooldown) behind healthy standbys
	sorted := SortAccountQuotaStatesWithThresholds(
		[]AccountQuotaState{standbyWeeklyLow, standbyHealthy},
		"",
		0.05,
		0.05,
		"auto",
		SwitchModeBalanced,
	)
	if len(sorted) != 2 || sorted[0].Email != standbyHealthy.Email || sorted[1].Email != standbyWeeklyLow.Email {
		t.Fatalf("expected healthy standby before weekly-depleted standby, got %v", sorted)
	}
}

func TestSortAccountQuotaStates_CooldownVsWeeklyDepleted(t *testing.T) {
	active := AccountQuotaState{
		Email:            "jose@example.com",
		IsActive:         true,
		PlanTier:         "Pro",
		Status:           "COOLDOWN",
		Quota5hCurrent:   0.64,
		Quota5hAvailable: 0.64,
		QuotaWeekly:      0.0,
	}
	albert := AccountQuotaState{
		Email:                  "alberto@example.com",
		IsActive:               false,
		PlanTier:               "Pro",
		Status:                 "COOLDOWN",
		Quota5hCurrent:         0.0,
		Quota5hAvailable:       0.86,
		QuotaWeekly:            0.01, // 1% weekly - exhausted!
		ResetSeconds:           2400, // 40m
		ResetSecondsWeekly:     300000,
		ResetHorizonText:       "Resets in 40m",
		ResetHorizonWeeklyText: "Resets in 3d",
	}
	satya := AccountQuotaState{
		Email:                  "satya@example.com",
		IsActive:               false,
		PlanTier:               "Pro",
		Status:                 "COOLDOWN",
		Quota5hCurrent:         0.0,
		Quota5hAvailable:       0.46,
		QuotaWeekly:            0.33, // 33% weekly - healthy!
		ResetSeconds:           9720, // 2h 42m
		ResetSecondsWeekly:     400000,
		ResetHorizonText:       "Resets in 2h 42m",
		ResetHorizonWeeklyText: "Resets in 4d",
	}
	prwh := AccountQuotaState{
		Email:                  "prwh@example.com",
		IsActive:               false,
		PlanTier:               "Pro",
		Status:                 "STANDBY",
		Quota5hCurrent:         0.0,
		Quota5hAvailable:       0.34,
		QuotaWeekly:            0.83, // 83% weekly - tons of quota!
		ResetSeconds:           12000, // 3h 20m
		ResetSecondsWeekly:     500000,
		ResetHorizonText:       "Resets in 3h 20m",
		ResetHorizonWeeklyText: "Resets in 5d",
	}

	// In Balanced mode: PRWH (83% weekly) ranks ahead of Satya (33%), and both rank ahead of Albert (1%)
	sortedBal := SortAccountQuotaStatesWithThresholds(
		[]AccountQuotaState{albert, satya, prwh, active},
		active.Email,
		0.05,
		0.05,
		"auto",
		SwitchModeBalanced,
	)
	if len(sortedBal) != 4 {
		t.Fatalf("expected 4 sorted accounts, got %d", len(sortedBal))
	}
	if sortedBal[0].Email != "jose@example.com" {
		t.Errorf("expected active account in row 0, got %s", sortedBal[0].Email)
	}
	if sortedBal[1].Email != "prwh@example.com" {
		t.Errorf("expected prwh in row 1 (highest weekly in balanced), got %s", sortedBal[1].Email)
	}
	if sortedBal[2].Email != "satya@example.com" {
		t.Errorf("expected satya in row 2 (recovers 5h soon with 33%% weekly), got %s", sortedBal[2].Email)
	}
	if sortedBal[3].Email != "alberto@example.com" {
		t.Errorf("expected albert in row 3 (weekly depleted), got %s", sortedBal[3].Email)
	}

	// In MaxContinuous mode: Satya (sooner 5h recovery, 46% projected) ranks ahead of PRWH (34%), Albert remains last
	sortedCont := SortAccountQuotaStatesWithThresholds(
		[]AccountQuotaState{albert, satya, prwh, active},
		active.Email,
		0.05,
		0.05,
		"auto",
		SwitchModeMaxContinuous,
	)
	if sortedCont[0].Email != "jose@example.com" {
		t.Errorf("expected active account in row 0, got %s", sortedCont[0].Email)
	}
	if sortedCont[1].Email != "satya@example.com" {
		t.Errorf("expected satya in row 1 (sooner 5h recovery in continuous mode), got %s", sortedCont[1].Email)
	}
	if sortedCont[2].Email != "prwh@example.com" {
		t.Errorf("expected prwh in row 2, got %s", sortedCont[2].Email)
	}
	if sortedCont[3].Email != "alberto@example.com" {
		t.Errorf("expected albert in row 3, got %s", sortedCont[3].Email)
	}
}

func TestExtractAccountMetrics_WeeklyDepletionCap(t *testing.T) {
	albert := AccountQuotaState{
		Email:                  "alberto@example.com",
		Quota5hCurrent:         0.0,
		Quota5hAvailable:       0.86,
		QuotaWeekly:            0.01,
		ResetSeconds:           2400,
		ResetHorizonText:       "Resets in 40m",
		ResetSecondsWeekly:     300000,
		ResetHorizonWeeklyText: "Resets in 3d",
	}
	metrics := ExtractAccountMetrics(albert, SwitchModeBalanced)
	if metrics.Q5hAvail > 0.01 {
		t.Errorf("expected Q5hAvail to be capped by weekly quota (0.01), got %f", metrics.Q5hAvail)
	}
}

func TestSortAccountQuotaStates_JoseAntonioVsAlbert(t *testing.T) {
	jose := AccountQuotaState{
		Email:              "jose@example.com",
		PlanTier:           "Pro",
		Status:             "COOLDOWN",
		Quota5hCurrent:     0.84,
		Quota5hAvailable:   0.84,
		QuotaWeekly:        0.0,
		ResetSecondsWeekly: 300000,
	}
	albert := AccountQuotaState{
		Email:              "alberto@example.com",
		PlanTier:           "Pro",
		Status:             "COOLDOWN",
		Quota5hCurrent:     0.0,
		Quota5hAvailable:   0.86,
		QuotaWeekly:        0.01,
		ResetSeconds:       2400,
		ResetSecondsWeekly: 300000,
	}

	sorted := SortAccountQuotaStatesWithThresholds([]AccountQuotaState{albert, jose}, "", 0.05, 0.05, "auto", SwitchModeBalanced)
	if len(sorted) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(sorted))
	}
	if sorted[0].Email != "jose@example.com" {
		t.Errorf("expected Jose Antonio (84%% 5h quota) to outrank Albert (0%% 5h quota) in Tier 4, got %s", sorted[0].Email)
	}
}

func TestSortAccountQuotaStates_WeeklyRecoveringEntersTier3(t *testing.T) {
	recovering := AccountQuotaState{
		Email:              "recovering@example.com",
		PlanTier:           "Pro",
		Status:             "COOLDOWN",
		Quota5hCurrent:     0.0,
		Quota5hAvailable:   0.5,
		QuotaWeekly:        0.0,
		ResetSeconds:       3600,
		ResetSecondsWeekly: 1800, // Weekly resets in 30m!
	}
	depleted := AccountQuotaState{
		Email:              "depleted@example.com",
		PlanTier:           "Pro",
		Status:             "COOLDOWN",
		Quota5hCurrent:     0.0,
		Quota5hAvailable:   0.5,
		QuotaWeekly:        0.01,
		ResetSeconds:       3600,
		ResetSecondsWeekly: 400000, // 4.5 days
	}

	sorted := SortAccountQuotaStatesWithThresholds([]AccountQuotaState{depleted, recovering}, "", 0.05, 0.05, "auto", SwitchModeBalanced)
	if len(sorted) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(sorted))
	}
	if sorted[0].Email != "recovering@example.com" {
		t.Errorf("expected account recovering weekly in 30m to enter Tier 3 ahead of Tier 4, got %s", sorted[0].Email)
	}
}

func TestComputeEffectiveWeeklyAvailable_Boundary(t *testing.T) {
	b5h := ComputeEffectiveWeeklyAvailable(0.02, 18000.0)
	if b5h != 0.02 {
		t.Errorf("expected exactly 0.02 at 5h boundary, got %f", b5h)
	}
	inWindow := ComputeEffectiveWeeklyAvailable(0.0, 3600.0)
	if math.Abs(inWindow-0.80) > 0.001 {
		t.Errorf("expected 0.80 for 1h reset, got %f", inWindow)
	}
}

func TestBuildAccountQuotaStatesFromMap_RecoversStaleErrorStatus(t *testing.T) {
	accs := []*keyring.Account{
		{
			Email:        "recovered@example.com",
			Status:       "ERROR",
			ErrorMessage: "stale error",
			IsActive:     false,
		},
		{
			Email:        "stillbroken@example.com",
			Status:       "ERROR",
			ErrorMessage: "Verify your account to continue. (VALIDATION_REQUIRED)",
			IsActive:     false,
		},
	}

	summaries := map[string]*QuotaSummary{
		"recovered@example.com": {
			AccountEmail:        "recovered@example.com",
			Quota5hFraction:     0.85,
			QuotaWeeklyFraction: 0.90,
			ResetHorizonText:    "Resets in 4h 30m",
			ErrorStatus:         "",
			ErrorMessage:        "",
		},
		"stillbroken@example.com": {
			AccountEmail:        "stillbroken@example.com",
			Quota5hFraction:     0.0,
			QuotaWeeklyFraction: 0.0,
			ResetHorizonText:    "Error: VALIDATION_REQUIRED",
			ErrorStatus:         "ERROR",
			ErrorMessage:        "Verify your account to continue. (VALIDATION_REQUIRED)",
		},
	}

	states := BuildAccountQuotaStatesFromMapWithThresholds(accs, summaries, 0.10, 0.05)
	if len(states) != 2 {
		t.Fatalf("expected 2 states, got %d", len(states))
	}
	if states[0].Status != "STANDBY" {
		t.Errorf("expected recovered account status STANDBY, got %q", states[0].Status)
	}
	if states[0].ErrorMessage != "" {
		t.Errorf("expected recovered account ErrorMessage cleared, got %q", states[0].ErrorMessage)
	}
	if states[1].Status != "ERROR" {
		t.Errorf("expected stillbroken account status ERROR, got %q", states[1].Status)
	}
	if !strings.Contains(states[1].ErrorMessage, "VALIDATION_REQUIRED") {
		t.Errorf("expected stillbroken account ErrorMessage preserved, got %q", states[1].ErrorMessage)
	}
}
