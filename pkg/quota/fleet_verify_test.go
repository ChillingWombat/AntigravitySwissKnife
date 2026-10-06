package quota

import (
	"fmt"
	"testing"

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
