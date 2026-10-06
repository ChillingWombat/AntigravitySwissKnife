package quota

import (
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
		{"random@gmail.com", "Google AI Pro", "Pro"},
		{"random@gmail.com", "Google AI Ultra", "Ultra 20X"},
		{"random@gmail.com", "", "Pro"}, // fallback
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
		{"Google AI Pro", "Pro"},
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
	if states[0].Status != core.AccountStatusCooldown {
		t.Fatalf("expected status %s when quota below threshold, got %s", core.AccountStatusCooldown, states[0].Status)
	}

	// 2. Returns to STANDBY when quota recovers above threshold
	accCooling := &keyring.Account{
		Email:    "cooling_test@example.com",
		Label:    "Cooling Test",
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

	summariesRecovered := map[string]*QuotaSummary{
		"cooling_test@example.com": recoveredSummary,
	}

	statesRecovered := BuildAccountQuotaStatesFromMapWithThreshold([]*keyring.Account{accCooling}, summariesRecovered, 0.05)
	if len(statesRecovered) != 1 {
		t.Fatalf("expected 1 state, got %d", len(statesRecovered))
	}
	if statesRecovered[0].Status != core.AccountStatusStandby {
		t.Fatalf("expected status %s after quota resets, got %s", core.AccountStatusStandby, statesRecovered[0].Status)
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


