package quota

import (
	"fmt"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

func TestLiveFleetVerification(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live fleet test in short mode")
	}
	store, _ := keyring.NewStore("")
	_ = keyring.SyncStoreFromCloudAccountsDB(store, "")
	accs := store.ListAccounts()
	active := store.ActiveAccount()
	summaries := PollFleetAccounts(accs, store)
	states := BuildAccountQuotaStatesFromMap(accs, summaries)
	fleet := ComputeFleetSummary(states, active)

	fmt.Println("=== LIVE FLEET VERIFICATION ===")
	fmt.Printf("Active Account: %s\n", fleet.ActiveAccount)
	for _, a := range fleet.Accounts {
		fmt.Printf("Email: %-30s | Tier: %-15s | Credits: %2.0f | 5h: %5.1f%% | Weekly: %5.1f%% | Active: %v\n",
			a.Email, a.PlanTier, a.Credits, a.Quota5hAvailable*100, a.QuotaWeekly*100, a.IsActive)
	}
}

func TestCachedFleetVerification(t *testing.T) {
	store, err := keyring.NewStore("")
	if err != nil {
		t.Fatalf("keyring.NewStore failed: %v", err)
	}
	accs := store.ListAccounts()
	active := store.ActiveAccount()
	start := time.Now()
	states := BuildAccountQuotaStatesFromMap(accs, nil)
	fleet := ComputeFleetSummary(states, active)
	elapsed := time.Since(start)

	t.Logf("Total accounts: %d, States count: %d, Elapsed: %v", fleet.TotalAccounts, len(fleet.Accounts), elapsed)

	if elapsed > 50*time.Millisecond {
		t.Errorf("BuildAccountQuotaStatesFromMap took %v, expected <50ms", elapsed)
	}
	if len(accs) > 0 && len(fleet.Accounts) != len(accs) {
		t.Errorf("expected %d accounts, got %d", len(accs), len(fleet.Accounts))
	}
	if fleet.TotalAccounts != len(accs) {
		t.Errorf("expected total %d accounts, got %d", len(accs), fleet.TotalAccounts)
	}
}

func TestQuotaCacheMergingAndDeletion(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir)

	// Step 1: Save 2 accounts
	initial := map[string]*QuotaSummary{
		"alpha@example.com": {AccountEmail: "alpha@example.com", PlanTier: "Pro", Quota5hFraction: 0.8},
		"beta@example.com":  {AccountEmail: "beta@example.com", PlanTier: "Ultra", Quota5hFraction: 0.9},
	}
	if err := SaveQuotaCache(initial); err != nil {
		t.Fatalf("SaveQuotaCache failed: %v", err)
	}

	loaded := LoadQuotaCache()
	if len(loaded) != 2 {
		t.Fatalf("expected 2 accounts in cache, got %d", len(loaded))
	}

	// Step 2: Save 1 different account - must merge, not overwrite
	delta := map[string]*QuotaSummary{
		"gamma@example.com": {AccountEmail: "gamma@example.com", PlanTier: "Plus", Quota5hFraction: 0.5},
	}
	if err := SaveQuotaCache(delta); err != nil {
		t.Fatalf("SaveQuotaCache merge failed: %v", err)
	}

	merged := LoadQuotaCache()
	if len(merged) != 3 {
		t.Fatalf("expected 3 accounts after merge, got %d", len(merged))
	}
	if merged["alpha@example.com"] == nil || merged["beta@example.com"] == nil || merged["gamma@example.com"] == nil {
		t.Errorf("missing accounts after merge: %+v", merged)
	}

	// Step 3: Delete an account
	if err := DeleteQuotaCacheEntry("beta@example.com"); err != nil {
		t.Fatalf("DeleteQuotaCacheEntry failed: %v", err)
	}

	afterDelete := LoadQuotaCache()
	if len(afterDelete) != 2 {
		t.Fatalf("expected 2 accounts after delete, got %d", len(afterDelete))
	}
	if afterDelete["beta@example.com"] != nil {
		t.Errorf("expected beta@example.com to be deleted")
	}
	if afterDelete["alpha@example.com"] == nil || afterDelete["gamma@example.com"] == nil {
		t.Errorf("unexpected accounts deleted: %+v", afterDelete)
	}
}
