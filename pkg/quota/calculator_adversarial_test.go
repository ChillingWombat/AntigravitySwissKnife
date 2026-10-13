package quota

import (
	"strings"
	"testing"
)

func TestAdversarial_EvaluateAutoSwitchForApp_UnusedStandbySelection(t *testing.T) {
	accounts := []AccountQuotaState{
		{
			Email:          "app_desktop@domain.com",
			Status:         "ACTIVE",
			Quota5hCurrent: 0.02,
			QuotaWeekly:    0.80,
			PlanTier:       "Free",
			Priority:       "High",
		},
		{
			Email:          "app_cli@domain.com",
			Status:         "STANDBY",
			Quota5hCurrent: 0.90,
			QuotaWeekly:    0.90,
			PlanTier:       "Free",
			Priority:       "High",
		},
		{
			Email:          "app_vscode@domain.com",
			Status:         "STANDBY",
			Quota5hCurrent: 0.85,
			QuotaWeekly:    0.85,
			PlanTier:       "Free",
			Priority:       "High",
		},
		// Ineligible standby: quota depleted
		{
			Email:          "standby_depleted@domain.com",
			Status:         "STANDBY",
			Quota5hCurrent: 0.03,
			QuotaWeekly:    0.80,
			PlanTier:       "Free",
			Priority:       "High",
		},
		// Ineligible standby: BANNED
		{
			Email:          "standby_banned@domain.com",
			Status:         "BANNED",
			Quota5hCurrent: 0.95,
			QuotaWeekly:    0.95,
			PlanTier:       "Free",
			Priority:       "High",
		},
		// Ineligible standby: COOLING
		{
			Email:          "standby_cooling@domain.com",
			Status:         "COOLING",
			Quota5hCurrent: 0.95,
			QuotaWeekly:    0.95,
			PlanTier:       "Free",
			Priority:       "High",
		},
		// Eligible unused standby accounts
		{
			Email:          "standby_good1@domain.com",
			Status:         "STANDBY",
			Quota5hCurrent: 0.60,
			QuotaWeekly:    0.80,
			PlanTier:       "Free",
			Priority:       "High",
		},
		{
			Email:          "standby_good2@domain.com",
			Status:         "STANDBY",
			Quota5hCurrent: 0.80,
			QuotaWeekly:    0.85,
			PlanTier:       "Free",
			Priority:       "High",
		},
	}

	inUseEmails := []string{"app_desktop@domain.com", "app_cli@domain.com", "app_vscode@domain.com"}

	shouldSwitch, nextAcc, reason := EvaluateAutoSwitchForApp(
		accounts,
		"app_desktop@domain.com",
		inUseEmails,
		0.05,
		0.05,
		"balanced",
		120.0,
	)

	if !shouldSwitch {
		t.Fatalf("expected shouldSwitch to be true, got false")
	}
	if nextAcc == nil {
		t.Fatalf("expected nextAcc to be non-nil")
	}

	// Must pick an UNUSED standby account (standby_good2 or standby_good1), NEVER in-use accounts (cli, vscode)
	// and NEVER ineligible accounts (depleted, banned, cooling)
	validStandbys := map[string]bool{
		"standby_good1@domain.com": true,
		"standby_good2@domain.com": true,
	}
	if !validStandbys[nextAcc.Email] {
		t.Fatalf("CRITICAL BUG: selected invalid account %s (reason: %s)", nextAcc.Email, reason)
	}

	// Because standby_good2 has higher quota (0.80 vs 0.60), balanced mode ranks it higher
	if nextAcc.Email != "standby_good2@domain.com" {
		t.Errorf("expected standby_good2@domain.com (higher quota score), got %s", nextAcc.Email)
	}
}

func TestAdversarial_EvaluateAutoSwitchForApp_FallbackWhenStandbysExhausted(t *testing.T) {
	accounts := []AccountQuotaState{
		{
			Email:          "app_desktop@domain.com",
			Status:         "ACTIVE",
			Quota5hCurrent: 0.01,
			QuotaWeekly:    0.80,
			PlanTier:       "Free",
			Priority:       "High",
		},
		{
			Email:          "app_cli@domain.com",
			Status:         "STANDBY",
			Quota5hCurrent: 0.85,
			QuotaWeekly:    0.90,
			PlanTier:       "Free",
			Priority:       "High",
		},
		{
			Email:          "app_vscode@domain.com",
			Status:         "STANDBY",
			Quota5hCurrent: 0.50,
			QuotaWeekly:    0.70,
			PlanTier:       "Free",
			Priority:       "High",
		},
		// Only other account is BANNED
		{
			Email:          "standby_banned@domain.com",
			Status:         "BANNED",
			Quota5hCurrent: 0.99,
			QuotaWeekly:    0.99,
			PlanTier:       "Free",
			Priority:       "High",
		},
	}

	inUseEmails := []string{"app_desktop@domain.com", "app_cli@domain.com", "app_vscode@domain.com"}

	shouldSwitch, nextAcc, reason := EvaluateAutoSwitchForApp(
		accounts,
		"app_desktop@domain.com",
		inUseEmails,
		0.05,
		0.05,
		"balanced",
		120.0,
	)

	if !shouldSwitch {
		t.Fatalf("expected shouldSwitch to be true, got false")
	}
	if nextAcc == nil {
		t.Fatalf("expected nextAcc to be non-nil")
	}

	// Must fall back to one of the shared in-use accounts (app_cli or app_vscode), NEVER depleted desktop or banned
	if nextAcc.Email != "app_cli@domain.com" {
		t.Errorf("expected fallback to highest quota shared account app_cli@domain.com, got %s (reason: %s)", nextAcc.Email, reason)
	}
	if !strings.Contains(reason, "falling back to shared account") {
		t.Errorf("expected reason to mention falling back to shared account, got: %s", reason)
	}
}

func TestAdversarial_EvaluateAutoSwitchForApp_CaseInsensitiveCasing(t *testing.T) {
	accounts := []AccountQuotaState{
		{
			Email:          "App_Desktop@Domain.COM",
			Status:         "ACTIVE",
			Quota5hCurrent: 0.02,
			QuotaWeekly:    0.80,
			PlanTier:       "Free",
			Priority:       "High",
		},
		{
			Email:          "App_CLI@Domain.COM",
			Status:         "STANDBY",
			Quota5hCurrent: 0.85,
			QuotaWeekly:    0.90,
			PlanTier:       "Free",
			Priority:       "High",
		},
		{
			Email:          "Standby_Unused@Domain.COM",
			Status:         "STANDBY",
			Quota5hCurrent: 0.75,
			QuotaWeekly:    0.85,
			PlanTier:       "Free",
			Priority:       "High",
		},
	}

	inUseEmails := []string{"app_desktop@domain.com", "app_cli@domain.com"}

	shouldSwitch, nextAcc, _ := EvaluateAutoSwitchForApp(
		accounts,
		"APP_DESKTOP@DOMAIN.COM",
		inUseEmails,
		0.05,
		0.05,
		"balanced",
		120.0,
	)

	if !shouldSwitch {
		t.Fatalf("expected shouldSwitch=true with casing variation")
	}
	if nextAcc == nil || !strings.EqualFold(nextAcc.Email, "Standby_Unused@Domain.COM") {
		t.Fatalf("expected Standby_Unused@Domain.COM, got %v", nextAcc)
	}
}

func TestAdversarial_EvaluateAutoSwitchForApp_CreditOveragesEligible(t *testing.T) {
	accounts := []AccountQuotaState{
		{
			Email:          "app_desktop@domain.com",
			Status:         "ACTIVE",
			Quota5hCurrent: 0.02,
			QuotaWeekly:    0.01,
			PlanTier:       "Free",
			Priority:       "High",
		},
		{
			Email:                "standby_credit@domain.com",
			Status:               "STANDBY",
			Quota5hCurrent:       0.80,
			QuotaWeekly:          0.02, // Weekly breached, but overages enabled!
			EnableCreditOverages: true,
			Credits:              100.0,
			PlanTier:             "Tier 1",
			Priority:             "High",
		},
	}

	inUseEmails := []string{"app_desktop@domain.com"}

	shouldSwitch, nextAcc, _ := EvaluateAutoSwitchForApp(
		accounts,
		"app_desktop@domain.com",
		inUseEmails,
		0.05,
		0.05,
		"balanced",
		120.0,
	)

	if !shouldSwitch {
		t.Fatalf("expected shouldSwitch=true")
	}
	if nextAcc == nil || nextAcc.Email != "standby_credit@domain.com" {
		t.Fatalf("expected standby_credit@domain.com to be eligible due to credits overage, got %v", nextAcc)
	}
}

func TestAdversarial_EvaluateAutoSwitchForApp_AllExhausted(t *testing.T) {
	accounts := []AccountQuotaState{
		{
			Email:          "app_desktop@domain.com",
			Status:         "ACTIVE",
			Quota5hCurrent: 0.02,
			QuotaWeekly:    0.80,
		},
		{
			Email:          "app_cli@domain.com",
			Status:         "STANDBY",
			Quota5hCurrent: 0.01,
			QuotaWeekly:    0.80,
		},
		{
			Email:          "standby_depleted@domain.com",
			Status:         "STANDBY",
			Quota5hCurrent: 0.04,
			QuotaWeekly:    0.80,
		},
	}

	inUseEmails := []string{"app_desktop@domain.com", "app_cli@domain.com"}

	shouldSwitch, nextAcc, reason := EvaluateAutoSwitchForApp(
		accounts,
		"app_desktop@domain.com",
		inUseEmails,
		0.05,
		0.05,
		"balanced",
		120.0,
	)

	if shouldSwitch {
		t.Errorf("CRITICAL BUG: shouldSwitch is true when all accounts are exhausted! Got %v (reason: %s)", nextAcc, reason)
	}
	if nextAcc != nil {
		t.Errorf("expected nextAcc to be nil when all exhausted, got %v", nextAcc)
	}
}

func TestAdversarial_EvaluateAutoSwitchForApp_Phase2ExcludesAssignedInTick(t *testing.T) {
	// Account setup: app1 and app2 depleted; shared1 and shared2 healthy
	accounts := []AccountQuotaState{
		{
			Email:          "app1@domain.com",
			Status:         "ACTIVE",
			Quota5hCurrent: 0.01,
			QuotaWeekly:    0.80,
		},
		{
			Email:          "app2@domain.com",
			Status:         "ACTIVE",
			Quota5hCurrent: 0.02,
			QuotaWeekly:    0.80,
		},
		{
			Email:          "shared1@domain.com",
			Status:         "STANDBY",
			Quota5hCurrent: 0.90,
			QuotaWeekly:    0.90,
		},
		{
			Email:          "shared2@domain.com",
			Status:         "STANDBY",
			Quota5hCurrent: 0.85,
			QuotaWeekly:    0.85,
		},
	}

	// In use: both app1 and app2 active, shared1 and shared2 active in other services
	inUseEmails := []string{"app1@domain.com", "app2@domain.com", "shared1@domain.com", "shared2@domain.com"}

	// App 1 switches and claims shared1
	shouldSwitch1, nextAcc1, _ := EvaluateAutoSwitchForApp(
		accounts,
		"app1@domain.com",
		inUseEmails,
		0.05,
		0.05,
		"balanced",
		60.0,
	)
	if !shouldSwitch1 || nextAcc1 == nil || nextAcc1.Email != "shared1@domain.com" {
		t.Fatalf("expected app1 to rotate to shared1, got %v", nextAcc1)
	}

	// App 2 now evaluates in the same tick. Passing assignedInTick="shared1@domain.com"
	// Phase 2 must NOT assign shared1 again, but must fallback to shared2
	shouldSwitch2, nextAcc2, _ := EvaluateAutoSwitchForApp(
		accounts,
		"app2@domain.com",
		inUseEmails,
		0.05,
		0.05,
		"balanced",
		60.0,
		"shared1@domain.com",
	)
	if !shouldSwitch2 || nextAcc2 == nil {
		t.Fatalf("expected app2 to rotate, got nil")
	}
	if nextAcc2.Email == "shared1@domain.com" {
		t.Fatalf("CRITICAL: app2 converged on shared1@domain.com which was already assigned in this tick!")
	}
	if nextAcc2.Email != "shared2@domain.com" {
		t.Fatalf("expected app2 to select shared2@domain.com, got %s", nextAcc2.Email)
	}
}

