package quota

import (
	"strings"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

func TestCachedAccountFullArgonRestoredImmediately(t *testing.T) {
	now := time.Now()
	argon5h := 1.0
	argonWeekly := 1.0
	cached := &QuotaSummary{
		AccountEmail:            "pro-full@example.com",
		PlanTier:                PlanTierPro,
		Quota5hArgon:            &argon5h,
		QuotaWeeklyArgon:        &argonWeekly,
		ResetSeconds5hArgon:     0,
		ResetSecondsWeeklyArgon: 0,
		LastPolled:              now.Add(-10 * time.Minute),
	}

	breakdown := &LiveQuotaBreakdown{}
	restoreCachedArgon(breakdown, cached, now)

	if breakdown.Quota5hArgon == nil {
		t.Fatal("expected non-nil Quota5hArgon for full cached account")
	}
	if *breakdown.Quota5hArgon != 1.0 {
		t.Fatalf("expected Quota5hArgon = 1.0, got %v", *breakdown.Quota5hArgon)
	}
	if breakdown.ResetSeconds5hArgon != 0 {
		t.Fatalf("expected ResetSeconds5hArgon = 0, got %v", breakdown.ResetSeconds5hArgon)
	}
	if breakdown.ResetHorizon5hArgonText != "Ready" {
		t.Fatalf("expected ResetHorizon5hArgonText = 'Ready', got %q", breakdown.ResetHorizon5hArgonText)
	}

	if breakdown.QuotaWeeklyArgon == nil {
		t.Fatal("expected non-nil QuotaWeeklyArgon for full cached account")
	}
	if *breakdown.QuotaWeeklyArgon != 1.0 {
		t.Fatalf("expected QuotaWeeklyArgon = 1.0, got %v", *breakdown.QuotaWeeklyArgon)
	}
	if breakdown.ResetSecondsWeeklyArgon != 0 {
		t.Fatalf("expected ResetSecondsWeeklyArgon = 0, got %v", breakdown.ResetSecondsWeeklyArgon)
	}
	if breakdown.ResetHorizonWeeklyArgonText != "Ready" {
		t.Fatalf("expected ResetHorizonWeeklyArgonText = 'Ready', got %q", breakdown.ResetHorizonWeeklyArgonText)
	}
}

func TestCachedAccountPartialArgonInCooldownRestored(t *testing.T) {
	now := time.Now()
	argon5h := 0.45
	argonWeekly := 0.70
	lastPolled := now.Add(-30 * time.Minute)
	cached := &QuotaSummary{
		AccountEmail:            "pro-cooldown@example.com",
		PlanTier:                PlanTierPro,
		Quota5hArgon:            &argon5h,
		QuotaWeeklyArgon:        &argonWeekly,
		ResetSeconds5hArgon:     (2 * time.Hour).Seconds(), // 2h total reset horizon at poll time
		ResetSecondsWeeklyArgon: (48 * time.Hour).Seconds(),
		LastPolled:              lastPolled,
	}

	breakdown := &LiveQuotaBreakdown{}
	restoreCachedArgon(breakdown, cached, now)

	if breakdown.Quota5hArgon == nil {
		t.Fatal("expected non-nil Quota5hArgon for cooling cached account")
	}
	if *breakdown.Quota5hArgon != 0.45 {
		t.Fatalf("expected Quota5hArgon = 0.45, got %v", *breakdown.Quota5hArgon)
	}
	// 2h minus 30m elapsed = ~1.5h (5400s) remaining
	expectedRemaining := (90 * time.Minute).Seconds()
	if breakdown.ResetSeconds5hArgon < expectedRemaining-10 || breakdown.ResetSeconds5hArgon > expectedRemaining+10 {
		t.Fatalf("expected remaining ResetSeconds5hArgon ~ %v, got %v", expectedRemaining, breakdown.ResetSeconds5hArgon)
	}
	if !strings.Contains(breakdown.ResetHorizon5hArgonText, "Resets in") {
		t.Fatalf("expected countdown horizon, got %q", breakdown.ResetHorizon5hArgonText)
	}

	if breakdown.QuotaWeeklyArgon == nil {
		t.Fatal("expected non-nil QuotaWeeklyArgon for cooling cached account")
	}
	if *breakdown.QuotaWeeklyArgon != 0.70 {
		t.Fatalf("expected QuotaWeeklyArgon = 0.70, got %v", *breakdown.QuotaWeeklyArgon)
	}
}

func TestCachedAccountExpiredArgonRecoversToFull(t *testing.T) {
	now := time.Now()
	argon5h := 0.20
	argonWeekly := 0.35
	lastPolled := now.Add(-3 * time.Hour)
	cached := &QuotaSummary{
		AccountEmail:            "pro-expired@example.com",
		PlanTier:                PlanTierPro,
		Quota5hArgon:            &argon5h,
		QuotaWeeklyArgon:        &argonWeekly,
		ResetSeconds5hArgon:     (1 * time.Hour).Seconds(), // 1h reset horizon, but polled 3h ago -> expired
		ResetSecondsWeeklyArgon: (2 * time.Hour).Seconds(),
		LastPolled:              lastPolled,
	}

	breakdown := &LiveQuotaBreakdown{}
	restoreCachedArgon(breakdown, cached, now)

	if breakdown.Quota5hArgon == nil {
		t.Fatal("expected non-nil Quota5hArgon for expired cooldown account")
	}
	if *breakdown.Quota5hArgon != 1.0 {
		t.Fatalf("expected recovered Quota5hArgon = 1.0, got %v", *breakdown.Quota5hArgon)
	}
	if breakdown.ResetSeconds5hArgon != 0 {
		t.Fatalf("expected ResetSeconds5hArgon = 0, got %v", breakdown.ResetSeconds5hArgon)
	}
	if breakdown.ResetHorizon5hArgonText != "Ready" {
		t.Fatalf("expected ResetHorizon5hArgonText = 'Ready', got %q", breakdown.ResetHorizon5hArgonText)
	}

	if breakdown.QuotaWeeklyArgon == nil {
		t.Fatal("expected non-nil QuotaWeeklyArgon for expired cooldown account")
	}
	if *breakdown.QuotaWeeklyArgon != 1.0 {
		t.Fatalf("expected recovered QuotaWeeklyArgon = 1.0, got %v", *breakdown.QuotaWeeklyArgon)
	}
	if breakdown.ResetSecondsWeeklyArgon != 0 {
		t.Fatalf("expected ResetSecondsWeeklyArgon = 0, got %v", breakdown.ResetSecondsWeeklyArgon)
	}
}

func TestFleetSummaryBuiltFromCacheIncludesArgonOnStartup(t *testing.T) {
	now := time.Now()
	argon5hPro := 1.0
	argonWeeklyPro := 1.0
	argon5hUltra := 0.80
	argonWeeklyUltra := 0.90

	accounts := []*keyring.Account{
		{Email: "pro@example.com", PlanTier: PlanTierPro, IsActive: true},
		{Email: "ultra@example.com", PlanTier: "Ultra", IsActive: false},
	}

	cachedSummaries := map[string]*QuotaSummary{
		"pro@example.com": {
			AccountEmail:                "pro@example.com",
			PlanTier:                    PlanTierPro,
			Quota5hFraction:             1.0,
			QuotaWeeklyFraction:         1.0,
			Quota5hArgon:                &argon5hPro,
			QuotaWeeklyArgon:            &argonWeeklyPro,
			ResetSeconds5hArgon:         0,
			ResetHorizon5hArgonText:     "Ready",
			ResetSecondsWeeklyArgon:     0,
			ResetHorizonWeeklyArgonText: "Ready",
			LastPolled:                  now.Add(-5 * time.Minute),
		},
		"ultra@example.com": {
			AccountEmail:                "ultra@example.com",
			PlanTier:                    "Ultra",
			Quota5hFraction:             0.85,
			QuotaWeeklyFraction:         0.95,
			Quota5hArgon:                &argon5hUltra,
			QuotaWeeklyArgon:            &argonWeeklyUltra,
			ResetSeconds5hArgon:         3600,
			ResetHorizon5hArgonText:     "Resets in 1h 0m",
			ResetSecondsWeeklyArgon:     86400,
			ResetHorizonWeeklyArgonText: "Resets in 1d 0h",
			LastPolled:                  now,
		},
	}

	states := BuildAccountQuotaStatesFromMapWithThresholdsAndRouting(accounts, cachedSummaries, 0.05, 0.05, false)
	if len(states) != 2 {
		t.Fatalf("expected 2 states, got %d", len(states))
	}

	// Verify Pro account
	proState := states[0]
	if proState.Quota5hArgon == nil {
		t.Fatal("expected non-nil Quota5hArgon on cached Pro account state")
	}
	if *proState.Quota5hArgon != 1.0 {
		t.Fatalf("expected Pro Quota5hArgon = 1.0, got %v", *proState.Quota5hArgon)
	}
	if proState.QuotaWeeklyArgon == nil {
		t.Fatal("expected non-nil QuotaWeeklyArgon on cached Pro account state")
	}
	if *proState.QuotaWeeklyArgon != 1.0 {
		t.Fatalf("expected Pro QuotaWeeklyArgon = 1.0, got %v", *proState.QuotaWeeklyArgon)
	}
	if proState.ResetSeconds5hArgon != 0 || proState.Reset5hArgonSeconds != 0 {
		t.Fatalf("expected Pro ResetSeconds5hArgon = 0, got %v / %v", proState.ResetSeconds5hArgon, proState.Reset5hArgonSeconds)
	}
	if proState.ResetHorizon5hArgonText != "Ready" {
		t.Fatalf("expected Pro ResetHorizon5hArgonText = 'Ready', got %q", proState.ResetHorizon5hArgonText)
	}

	// Verify Ultra account
	ultraState := states[1]
	if ultraState.Quota5hArgon == nil {
		t.Fatal("expected non-nil Quota5hArgon on cached Ultra account state")
	}
	if *ultraState.Quota5hArgon != 0.80 {
		t.Fatalf("expected Ultra Quota5hArgon = 0.80, got %v", *ultraState.Quota5hArgon)
	}
	if ultraState.ResetSeconds5hArgon <= 0 || ultraState.Reset5hArgonSeconds <= 0 {
		t.Fatalf("expected Ultra ResetSeconds5hArgon > 0, got %v", ultraState.ResetSeconds5hArgon)
	}

	// Verify ComputeFleetSummary aggregates Argon quotas immediately
	fleet := ComputeFleetSummary(states, "pro@example.com")
	if fleet.Fleet5hArgonAvailable == nil {
		t.Fatal("expected non-nil Fleet5hArgonAvailable from cached startup states")
	}
	expectedAvg5h := (1.0 + 0.80) / 2.0
	if *fleet.Fleet5hArgonAvailable != expectedAvg5h {
		t.Fatalf("expected Fleet5hArgonAvailable = %v, got %v", expectedAvg5h, *fleet.Fleet5hArgonAvailable)
	}

	if fleet.FleetWeeklyArgonAvailable == nil {
		t.Fatal("expected non-nil FleetWeeklyArgonAvailable from cached startup states")
	}
	expectedAvgWeekly := (1.0 + 0.90) / 2.0
	if *fleet.FleetWeeklyArgonAvailable != expectedAvgWeekly {
		t.Fatalf("expected FleetWeeklyArgonAvailable = %v, got %v", expectedAvgWeekly, *fleet.FleetWeeklyArgonAvailable)
	}
}
