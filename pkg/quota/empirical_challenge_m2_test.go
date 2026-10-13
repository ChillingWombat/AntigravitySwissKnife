package quota

import (
	"strings"
	"testing"
)

// TestChallenge_MultiAppAutoSwitch_NoCollisionInSameTick rigorously tests that
// multiple apps depleting in the same tick never get assigned the same successor account,
// in both Phase 1 (unused standbys) and Phase 2 (shared pool fallback).
func TestChallenge_MultiAppAutoSwitch_NoCollisionInSameTick(t *testing.T) {
	// Setup fleet:
	// App 1 (Desktop) active on acc_desktop@domain.com (depleted)
	// App 2 (CLI) active on acc_cli@domain.com (depleted)
	// App 3 (VSCode) active on acc_vscode@domain.com (depleted)
	//
	// Standbys:
	// standby1@domain.com (healthy: 5h=0.80)
	// standby2@domain.com (healthy: 5h=0.70)
	//
	// Shared pool:
	// shared_worker@domain.com (healthy: 5h=0.90, active on a background task)

	accounts := []AccountQuotaState{
		{
			Email:           "acc_desktop@domain.com",
			Quota5hCurrent:  0.01,
			QuotaWeekly:     0.50,
			Status:          "ACTIVE",
			RefreshToken:    "valid_token_desktop",
		},
		{
			Email:           "acc_cli@domain.com",
			Quota5hCurrent:  0.02,
			QuotaWeekly:     0.50,
			Status:          "ACTIVE",
			RefreshToken:    "valid_token_cli",
		},
		{
			Email:           "acc_vscode@domain.com",
			Quota5hCurrent:  0.03,
			QuotaWeekly:     0.50,
			Status:          "ACTIVE",
			RefreshToken:    "valid_token_vscode",
		},
		{
			Email:           "standby1@domain.com",
			Quota5hCurrent:  0.80,
			QuotaWeekly:     0.80,
			Status:          "STANDBY",
			RefreshToken:    "valid_token_standby1",
		},
		{
			Email:           "standby2@domain.com",
			Quota5hCurrent:  0.70,
			QuotaWeekly:     0.70,
			Status:          "STANDBY",
			RefreshToken:    "valid_token_standby2",
		},
		{
			Email:           "shared_worker@domain.com",
			Quota5hCurrent:  0.90,
			QuotaWeekly:     0.90,
			Status:          "STANDBY",
			RefreshToken:    "valid_token_shared",
		},
	}

	inUseEmails := []string{
		"acc_desktop@domain.com",
		"acc_cli@domain.com",
		"acc_vscode@domain.com",
		"shared_worker@domain.com", // in-use so it's in Phase 2
	}

	assignedInTick := make([]string, 0)

	// Step 1: App 1 (Desktop) evaluates
	should1, succ1, reason1 := EvaluateAutoSwitchForApp(
		accounts,
		"acc_desktop@domain.com",
		inUseEmails,
		0.05, 0.05,
		"balanced",
		120.0,
		assignedInTick...,
	)
	if !should1 || succ1 == nil {
		t.Fatalf("Desktop expected to switch, got should=%v, reason=%s", should1, reason1)
	}
	if succ1.Email != "standby1@domain.com" {
		t.Fatalf("Desktop expected standby1 (highest unused), got %s", succ1.Email)
	}

	// Update tick state as daemon does:
	assignedInTick = append(assignedInTick, succ1.Email)
	for i, em := range inUseEmails {
		if em == "acc_desktop@domain.com" {
			inUseEmails[i] = succ1.Email
			break
		}
	}

	// Step 2: App 2 (CLI) evaluates in the same tick
	should2, succ2, reason2 := EvaluateAutoSwitchForApp(
		accounts,
		"acc_cli@domain.com",
		inUseEmails,
		0.05, 0.05,
		"balanced",
		120.0,
		assignedInTick...,
	)
	if !should2 || succ2 == nil {
		t.Fatalf("CLI expected to switch, got should=%v, reason=%s", should2, reason2)
	}
	// CLI MUST NOT get standby1! It must get standby2!
	if succ2.Email == "standby1@domain.com" {
		t.Fatalf("COLLISION BUG: CLI was assigned standby1 which was already assigned to Desktop in the same tick!")
	}
	if succ2.Email != "standby2@domain.com" {
		t.Fatalf("CLI expected standby2, got %s", succ2.Email)
	}

	// Update tick state again:
	assignedInTick = append(assignedInTick, succ2.Email)
	for i, em := range inUseEmails {
		if em == "acc_cli@domain.com" {
			inUseEmails[i] = succ2.Email
			break
		}
	}

	// Step 3: App 3 (VSCode) evaluates in the same tick (0 unused standbys remaining -> triggers Phase 2 shared fallback!)
	should3, succ3, reason3 := EvaluateAutoSwitchForApp(
		accounts,
		"acc_vscode@domain.com",
		inUseEmails,
		0.05, 0.05,
		"balanced",
		120.0,
		assignedInTick...,
	)
	if !should3 || succ3 == nil {
		t.Fatalf("VSCode expected to switch via Phase 2 shared fallback, got should=%v, reason=%s", should3, reason3)
	}
	// VSCode MUST NOT get standby1 or standby2! It must get shared_worker!
	if succ3.Email == "standby1@domain.com" || succ3.Email == "standby2@domain.com" {
		t.Fatalf("COLLISION BUG: VSCode fell back to an account assigned in the same tick: %s", succ3.Email)
	}
	if succ3.Email != "shared_worker@domain.com" {
		t.Fatalf("VSCode expected shared_worker, got %s", succ3.Email)
	}

	// Step 4: Now suppose a 4th app evaluates in the same tick. All eligible accounts have been assigned!
	assignedInTick = append(assignedInTick, succ3.Email)
	should4, succ4, reason4 := EvaluateAutoSwitchForApp(
		accounts,
		"acc_vscode@domain.com",
		inUseEmails,
		0.05, 0.05,
		"balanced",
		120.0,
		assignedInTick...,
	)
	if should4 || succ4 != nil {
		t.Fatalf("Expected no switch when all accounts are assigned in tick, but got succ=%v", succ4)
	}
	if !strings.Contains(reason4, "No eligible standby or shared accounts available") {
		t.Fatalf("Expected no eligible accounts reason, got %q", reason4)
	}
}

// TestChallenge_MultiAppAutoSwitch_CaseInsensitiveAssignedInTick tests that
// whitespace and uppercase differences in assignedInTick are properly normalized.
func TestChallenge_MultiAppAutoSwitch_CaseInsensitiveAssignedInTick(t *testing.T) {
	accounts := []AccountQuotaState{
		{
			Email:          "app@domain.com",
			Quota5hCurrent: 0.01,
			QuotaWeekly:    0.50,
			Status:         "ACTIVE",
			RefreshToken:   "tok",
		},
		{
			Email:          "shared1@domain.com",
			Quota5hCurrent: 0.90,
			QuotaWeekly:    0.90,
			Status:         "STANDBY",
			RefreshToken:   "tok",
		},
	}
	inUse := []string{"app@domain.com", "shared1@domain.com"}

	// Passing dirty casing and spaces in assignedInTick
	dirtyAssigned := []string{"  SHARED1@DOMAIN.COM  ", "", "   "}
	should, succ, reason := EvaluateAutoSwitchForApp(
		accounts,
		"app@domain.com",
		inUse,
		0.05, 0.05,
		"balanced",
		100.0,
		dirtyAssigned...,
	)
	if should || succ != nil {
		t.Fatalf("Expected shared1 to be excluded despite uppercase/spaces, but it was picked: %v (reason: %s)", succ, reason)
	}
}
