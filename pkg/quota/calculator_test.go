package quota

import (
	"testing"
	"time"

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
