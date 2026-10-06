package keyring

import (
	"testing"
)

func TestReadCloudAccountsDB(t *testing.T) {
	accounts, err := ReadCloudAccountsDB("")
	if err != nil {
		t.Fatalf("ReadCloudAccountsDB failed: %v", err)
	}
	t.Logf("Read %d accounts from cloud_accounts.db", len(accounts))
	if len(accounts) == 0 {
		t.Log("No accounts found in cloud_accounts.db (might be empty or missing)")
		return
	}

	foundTorres := false
	for _, a := range accounts {
		t.Logf("Account: %s, Plan: %s, Credits: %.0f, 5H: %.2f, Wk: %.2f, HasRefresh: %v", a.Email, a.PlanTier, a.Credits, a.Quota5h, a.QuotaWeekly, a.RefreshToken != "")
		if a.Email == "torreswader@gmail.com" {
			foundTorres = true
			if a.RefreshToken == "" {
				t.Errorf("Expected torreswader to have refresh token, got empty")
			}
			if a.PlanTier == "" {
				t.Errorf("Expected torreswader to have plan tier, got empty")
			}
		}
	}

	if !foundTorres {
		t.Errorf("torreswader@gmail.com was not found in cloud_accounts.db")
	}
}
