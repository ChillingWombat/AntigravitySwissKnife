package quota

import (
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

func TestDualEngineQuotaStatusCalculation(t *testing.T) {
	accStandby := &keyring.Account{
		Email:        "dual-standby@example.com",
		Label:        "Dual Standby",
		PlanTier:     "Pro",
		Status:       "STANDBY",
		RefreshToken: "refresh-token-dual",
	}

	accActive := &keyring.Account{
		Email:        "dual-active@example.com",
		Label:        "Dual Active",
		PlanTier:     "Pro",
		Status:       "ACTIVE",
		IsActive:     true,
		RefreshToken: "refresh-token-dual-active",
	}

	// Gemini quota is depleted (0.01 <= 0.05 threshold), but Claude/GPT has 80% available
	summaryStandby := &QuotaSummary{
		AccountEmail:         "dual-standby@example.com",
		Quota5hFraction:      0.01,
		QuotaWeeklyFraction:  0.01,
		Quota5hClaudeGPT:     0.80,
		QuotaWeeklyClaudeGPT: 0.85,
		ResetSeconds5h:       3 * 3600,
		ResetSecondsWeekly:   48 * 3600,
		ResetHorizonText:     "Resets in 3h",
	}

	summaryActive := &QuotaSummary{
		AccountEmail:         "dual-active@example.com",
		Quota5hFraction:      0.01,
		QuotaWeeklyFraction:  0.01,
		Quota5hClaudeGPT:     0.75,
		QuotaWeeklyClaudeGPT: 0.80,
		ResetSeconds5h:       2 * 3600,
		ResetSecondsWeekly:   36 * 3600,
		ResetHorizonText:     "Resets in 2h",
	}

	summaries := map[string]*QuotaSummary{
		"dual-standby@example.com": summaryStandby,
		"dual-active@example.com":  summaryActive,
	}

	// 1. When allow_non_gemini_native_models is FALSE:
	// Both accounts should enter COOLING because their Gemini quota is depleted
	statesSingle := BuildAccountQuotaStatesFromMapWithThresholdsAndRouting(
		[]*keyring.Account{accStandby, accActive},
		summaries,
		0.05,
		0.05,
		false,
	)

	if len(statesSingle) != 2 {
		t.Fatalf("expected 2 states in single-engine mode, got %d", len(statesSingle))
	}

	var standbySingle, activeSingle *AccountQuotaState
	for i := range statesSingle {
		if statesSingle[i].Email == "dual-standby@example.com" {
			standbySingle = &statesSingle[i]
		} else if statesSingle[i].Email == "dual-active@example.com" {
			activeSingle = &statesSingle[i]
		}
	}

	if standbySingle == nil || standbySingle.Status != core.AccountStatusCooling {
		t.Errorf("expected standby account to enter COOLING when non-Gemini routing is disabled, got %v", standbySingle.Status)
	}
	if activeSingle == nil {
		t.Fatalf("expected activeSingle not to be nil")
	}

	// 2. When allow_non_gemini_native_models is TRUE:
	// Standby account must maintain STANDBY status (not COOLING)
	// Active account must maintain ACTIVE status (not COOLING)
	statesDual := BuildAccountQuotaStatesFromMapWithThresholdsAndRouting(
		[]*keyring.Account{accStandby, accActive},
		summaries,
		0.05,
		0.05,
		true,
	)

	var standbyDual, activeDual *AccountQuotaState
	for i := range statesDual {
		if statesDual[i].Email == "dual-standby@example.com" {
			standbyDual = &statesDual[i]
		} else if statesDual[i].Email == "dual-active@example.com" {
			activeDual = &statesDual[i]
		}
	}

	if standbyDual == nil {
		t.Fatalf("dual standby account not found")
	}
	if standbyDual.Status != core.AccountStatusStandby {
		t.Errorf("expected standby account to maintain STANDBY when non-Gemini routing is enabled, got %s", standbyDual.Status)
	}
	if standbyDual.Quota5hClaudeGPT != 0.80 {
		t.Errorf("expected Quota5hClaudeGPT to be preserved (0.80), got %f", standbyDual.Quota5hClaudeGPT)
	}
	if standbyDual.QuotaWeeklyClaudeGPT != 0.85 {
		t.Errorf("expected QuotaWeeklyClaudeGPT to be preserved (0.85), got %f", standbyDual.QuotaWeeklyClaudeGPT)
	}

	if activeDual == nil {
		t.Fatalf("dual active account not found")
	}
	if activeDual.Status != "ACTIVE" {
		t.Errorf("expected active account to maintain ACTIVE status when non-Gemini routing is enabled, got %s", activeDual.Status)
	}

	// 3. When both Gemini and non-Gemini quotas are depleted:
	// Account MUST enter COOLING even when allow_non_gemini_native_models is TRUE
	summaryBothDepleted := &QuotaSummary{
		AccountEmail:         "dual-standby@example.com",
		Quota5hFraction:      0.01,
		QuotaWeeklyFraction:  0.01,
		Quota5hClaudeGPT:     0.02,
		QuotaWeeklyClaudeGPT: 0.03,
		ResetHorizonText:     "Resets in 3h",
	}
	summariesDepleted := map[string]*QuotaSummary{
		"dual-standby@example.com": summaryBothDepleted,
	}
	statesBothDepleted := BuildAccountQuotaStatesFromMapWithThresholdsAndRouting(
		[]*keyring.Account{accStandby},
		summariesDepleted,
		0.05,
		0.05,
		true,
	)
	if len(statesBothDepleted) != 1 || statesBothDepleted[0].Status != core.AccountStatusCooling {
		t.Errorf("expected account with both providers depleted to enter COOLING, got %v", statesBothDepleted[0].Status)
	}

	// 4. When Gemini is depleted, Claude 5h is healthy, but Claude weekly is strictly 0.0 (exhausted):
	// Account MUST enter COOLING without credit overages
	summaryClaudeWeeklyDepleted := &QuotaSummary{
		AccountEmail:              "dual-standby@example.com",
		Quota5hFraction:           0.01,
		QuotaWeeklyFraction:       0.01,
		Quota5hClaudeGPT:          0.80,
		QuotaWeeklyClaudeGPT:      0.00,
		ClaudeWeeklyBucketPresent: true,
		Models: []ModelQuota{
			{ModelName: "claude-3-7-sonnet-weekly", Fraction: 0.00},
		},
		ResetHorizonText: "Resets in 3h",
	}
	statesClaudeWeeklyDepleted := BuildAccountQuotaStatesFromMapWithThresholdsAndRouting(
		[]*keyring.Account{accStandby},
		map[string]*QuotaSummary{"dual-standby@example.com": summaryClaudeWeeklyDepleted},
		0.05,
		0.05,
		true,
	)
	if len(statesClaudeWeeklyDepleted) != 1 || statesClaudeWeeklyDepleted[0].Status != core.AccountStatusCooling {
		t.Errorf("expected account with depleted Claude weekly quota (0.00) to enter COOLING, got %v", statesClaudeWeeklyDepleted[0].Status)
	}

	// 5. When Claude weekly is 0.0 but EnableCreditOverages is TRUE with positive credits:
	// Account maintains STANDBY
	accOverages := &keyring.Account{
		Email:                "dual-overages@example.com",
		Label:                "Dual Overages",
		PlanTier:             "Pro",
		Status:               "STANDBY",
		RefreshToken:         "refresh-token-overages",
		EnableCreditOverages: true,
		Credits:              10.0,
	}
	summaryOverages := &QuotaSummary{
		AccountEmail:         "dual-overages@example.com",
		Quota5hFraction:      0.01,
		QuotaWeeklyFraction:  0.01,
		Quota5hClaudeGPT:     0.80,
		QuotaWeeklyClaudeGPT: 0.00,
		Credits:              10.0,
		Models: []ModelQuota{
			{ModelName: "claude-3-7-sonnet-weekly", Fraction: 0.00},
		},
	}
	statesOverages := BuildAccountQuotaStatesFromMapWithThresholdsAndRouting(
		[]*keyring.Account{accOverages},
		map[string]*QuotaSummary{"dual-overages@example.com": summaryOverages},
		0.05,
		0.05,
		true,
	)
	if len(statesOverages) != 1 || statesOverages[0].Status != core.AccountStatusStandby {
		t.Errorf("expected account with credit overages to maintain STANDBY even with 0.00 Claude weekly, got %v", statesOverages[0].Status)
	}

	// 6. When Claude has no weekly bucket (unconstrained):
	// Account maintains STANDBY because 5h quota is healthy
	summaryUnconstrained := &QuotaSummary{
		AccountEmail:        "dual-standby@example.com",
		Quota5hFraction:     0.01,
		QuotaWeeklyFraction: 0.01,
		Quota5hClaudeGPT:    0.80,
		// QuotaWeeklyClaudeGPT not set; Models has only 5h Claude
		Models: []ModelQuota{
			{ModelName: "claude-3-7-sonnet", Fraction: 0.80},
		},
	}
	statesUnconstrained := BuildAccountQuotaStatesFromMapWithThresholdsAndRouting(
		[]*keyring.Account{accStandby},
		map[string]*QuotaSummary{"dual-standby@example.com": summaryUnconstrained},
		0.05,
		0.05,
		true,
	)
	if len(statesUnconstrained) != 1 || statesUnconstrained[0].Status != core.AccountStatusStandby {
		t.Errorf("expected account with unconstrained Claude quota to maintain STANDBY, got %v", statesUnconstrained[0].Status)
	}
}

func TestDualEngineStandbyRankingAndEligibility(t *testing.T) {
	// Account A: Gemini depleted (0.01), Claude healthy (0.90)
	accA := AccountQuotaState{
		Email:                "acc-a@example.com",
		PlanTier:             "Pro",
		Priority:             "High",
		Status:               core.AccountStatusStandby,
		RefreshToken:         "tok-a",
		Quota5hCurrent:       0.01,
		QuotaWeekly:          0.01,
		Quota5hClaudeGPT:     0.90,
		QuotaWeeklyClaudeGPT: 0.95,
	}

	// Account B: Gemini healthy (0.70), Claude depleted (0.00)
	accB := AccountQuotaState{
		Email:                "acc-b@example.com",
		PlanTier:             "Pro",
		Priority:             "High",
		Status:               core.AccountStatusStandby,
		RefreshToken:         "tok-b",
		Quota5hCurrent:       0.70,
		QuotaWeekly:          0.80,
		Quota5hClaudeGPT:     0.00,
		QuotaWeeklyClaudeGPT: 0.00,
	}

	// Account C: Both depleted (0.01), status marked COOLING
	accC := AccountQuotaState{
		Email:                "acc-c@example.com",
		PlanTier:             "Pro",
		Priority:             "High",
		Status:               core.AccountStatusCooling,
		RefreshToken:         "tok-c",
		Quota5hCurrent:       0.01,
		QuotaWeekly:          0.01,
		Quota5hClaudeGPT:     0.01,
		QuotaWeeklyClaudeGPT: 0.01,
	}

	// Account D: Gemini depleted, Claude healthy (0.85), but status was marked COOLING by legacy
	accD := AccountQuotaState{
		Email:                "acc-d@example.com",
		PlanTier:             "Pro",
		Priority:             "High",
		Status:               core.AccountStatusCooling,
		RefreshToken:         "tok-d",
		Quota5hCurrent:       0.01,
		QuotaWeekly:          0.01,
		Quota5hClaudeGPT:     0.85,
		QuotaWeeklyClaudeGPT: 0.90,
	}

	allAccounts := []AccountQuotaState{accA, accB, accC, accD}

	// 1. Single-engine mode (allowNonGemini = false):
	// Only accB should qualify because accA and accD have Gemini depleted, and accC is cooling
	rankedSingle := RankStandbyAccountsWithThresholdsAndRouting(allAccounts, 0.05, 0.05, SwitchModeBalanced, false)
	if len(rankedSingle) != 1 {
		t.Fatalf("expected 1 eligible candidate in single-engine mode, got %d", len(rankedSingle))
	}
	if rankedSingle[0].Email != "acc-b@example.com" {
		t.Errorf("expected acc-b to be the sole candidate in single-engine mode, got %s", rankedSingle[0].Email)
	}

	// 2. Dual-engine mode (allowNonGemini = true):
	// accA, accB, and accD must qualify as eligible standby candidates!
	rankedDual := RankStandbyAccountsWithThresholdsAndRouting(allAccounts, 0.05, 0.05, SwitchModeBalanced, true)
	if len(rankedDual) != 3 {
		t.Fatalf("expected 3 eligible candidates in dual-engine mode, got %d", len(rankedDual))
	}

	emails := make(map[string]bool)
	for _, cand := range rankedDual {
		emails[cand.Email] = true
	}
	if !emails["acc-a@example.com"] {
		t.Errorf("expected acc-a with Claude quota to be eligible in dual-engine mode")
	}
	if !emails["acc-b@example.com"] {
		t.Errorf("expected acc-b with Gemini quota to be eligible in dual-engine mode")
	}
	if !emails["acc-d@example.com"] {
		t.Errorf("expected acc-d with Claude quota (previously cooling) to be eligible in dual-engine mode")
	}
	if emails["acc-c@example.com"] {
		t.Errorf("expected acc-c (both depleted) to remain excluded")
	}
}

func TestDualEngineAutoSwitchEvaluation(t *testing.T) {
	// Active account has depleted Gemini (0.02) but healthy Claude (0.85)
	activeAcc := AccountQuotaState{
		Email:                "active@example.com",
		IsActive:             true,
		Status:               "ACTIVE",
		RefreshToken:         "tok-active",
		Quota5hCurrent:       0.02,
		QuotaWeekly:          0.02,
		Quota5hClaudeGPT:     0.85,
		QuotaWeeklyClaudeGPT: 0.90,
	}

	standbyAcc := AccountQuotaState{
		Email:                "standby@example.com",
		Status:               core.AccountStatusStandby,
		RefreshToken:         "tok-standby",
		Quota5hCurrent:       0.80,
		QuotaWeekly:          0.85,
		Quota5hClaudeGPT:     0.80,
		QuotaWeeklyClaudeGPT: 0.85,
	}

	accounts := []AccountQuotaState{activeAcc, standbyAcc}

	// In single-engine mode: active Gemini quota breached -> must switch!
	shouldSwitchSingle, successorSingle, _ := EvaluateAutoSwitchWithThresholdsAndRouting(
		accounts,
		"active@example.com",
		0.05,
		0.05,
		SwitchModeBalanced,
		1000.0,
		false,
	)
	if !shouldSwitchSingle || successorSingle == nil || successorSingle.Email != "standby@example.com" {
		t.Errorf("expected auto-switch in single-engine mode when Gemini is depleted")
	}

	// In dual-engine mode: active has Claude quota available -> should NOT switch!
	shouldSwitchDual, _, reasonDual := EvaluateAutoSwitchWithThresholdsAndRouting(
		accounts,
		"active@example.com",
		0.05,
		0.05,
		SwitchModeBalanced,
		1000.0,
		true,
	)
	if shouldSwitchDual {
		t.Errorf("expected auto-switch to be suppressed in dual-engine mode when Claude quota is healthy, got switch with reason: %s", reasonDual)
	}

	// Per-App rotation in dual-engine mode:
	shouldSwitchApp, _, _ := EvaluateAutoSwitchForAppWithRouting(
		accounts,
		"active@example.com",
		[]string{"active@example.com"},
		0.05,
		0.05,
		SwitchModeBalanced,
		1000.0,
		true,
	)
	if shouldSwitchApp {
		t.Errorf("expected EvaluateAutoSwitchForApp to recognize healthy Claude quota and not switch")
	}
}
