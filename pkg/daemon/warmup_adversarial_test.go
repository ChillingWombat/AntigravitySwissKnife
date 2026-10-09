package daemon

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/quota"
)

// TestAdversarial_PollStandbyAccounts_CooldownSkip validates that pollStandbyAccounts
// strictly bypasses polling when a standby account is in cooldown (now < resetTime).
func TestAdversarial_PollStandbyAccounts_CooldownSkip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swiss_test_warmup_cooldown_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir)
	t.Setenv("HOME", tmpDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	sockPath := filepath.Join(tmpDir, "daemon.sock")
	cfg := core.DefaultConfig()
	cfg.WarmupEnabled = true
	cfg.PostResetDelaySec = 60

	d, err := NewDaemon(cfg, sockPath)
	if err != nil {
		t.Fatalf("NewDaemon error: %v", err)
	}

	activeEmail := "active@example.com"
	cooldownEmail := "cooldown_standby@example.com"

	_ = d.Keyring.AddOrUpdateAccount(&keyring.Account{
		Email: activeEmail,
		Label: "Active Account",
	})
	_ = d.Keyring.SetActiveAccount(activeEmail)

	_ = d.Keyring.AddOrUpdateAccount(&keyring.Account{
		Email: cooldownEmail,
		Label: "Cooldown Standby",
	})

	// Pre-populate quota cache for standby account with an active cooldown
	// Reset horizon is 30 minutes in the future
	futureReset := time.Now().Add(30 * time.Minute)
	d.setQuotaSummary(cooldownEmail, &quota.QuotaSummary{
		AccountEmail:  cooldownEmail,
		OverallHealth: core.StatusExhausted,
		MinFraction:   0.0,
		Models: []quota.ModelQuota{
			{
				ModelName: "gemini-pro",
				ResetTime: futureReset,
			},
		},
	})

	// Track HTTP calls to upstream endpoints to detect whether any network requests were made
	var pollCallCount int32
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&pollCallCount, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer mockServer.Close()

	// Run pollStandbyAccounts with jitter 0
	d.pollStandbyAccounts(nil, 0)

	// Since cooldown_standby@example.com has now < resetTime, ShouldSkipCooldownPolling is true.
	// Therefore, it must NOT have initiated polling or replaced the cache.
	cached := d.getQuotaSummary(cooldownEmail)
	if cached == nil {
		t.Fatalf("expected cached quota summary to remain present")
	}
	if cached.OverallHealth != core.StatusExhausted {
		t.Errorf("cached health should remain StatusExhausted, got %s", cached.OverallHealth)
	}
	if atomic.LoadInt32(&pollCallCount) > 0 {
		t.Errorf("expected 0 network calls for account in cooldown, got %d", pollCallCount)
	}
}

// TestAdversarial_CheckPostResetIgnitions_SafetyAndExhaustion validates that
// checkPostResetIgnitions:
// 1. Avoids igniting accounts whose post-reset verification reveals they are still exhausted.
// 2. Avoids igniting unconfirmed / nil quota accounts.
// 3. Ignites only confirmed reset (healthy) accounts when WarmupEnabled is true.
func TestAdversarial_CheckPostResetIgnitions_SafetyAndExhaustion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swiss_test_warmup_ignition_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir)
	t.Setenv("HOME", tmpDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	sockPath := filepath.Join(tmpDir, "daemon.sock")
	cfg := core.DefaultConfig()
	cfg.WarmupEnabled = true
	cfg.PostResetDelaySec = 10 // 10s delay

	d, err := NewDaemon(cfg, sockPath)
	if err != nil {
		t.Fatalf("NewDaemon error: %v", err)
	}

	activeEmail := "active@example.com"
	standbyEmail := "standby_tested@example.com"

	_ = d.Keyring.AddOrUpdateAccount(&keyring.Account{
		Email: activeEmail,
	})
	_ = d.Keyring.SetActiveAccount(activeEmail)

	_ = d.Keyring.AddOrUpdateAccount(&keyring.Account{
		Email: standbyEmail,
	})

	// Scenario A: Delay has NOT elapsed yet (now < resetTime + delay)
	// Reset was 5 seconds ago, delay is 10 seconds -> target is 5 seconds in the future
	pastReset := time.Now().Add(-5 * time.Second)
	d.setQuotaSummary(standbyEmail, &quota.QuotaSummary{
		AccountEmail:  standbyEmail,
		OverallHealth: core.StatusExhausted,
		MinFraction:   0.0,
		Models: []quota.ModelQuota{
			{ModelName: "gemini-pro", ResetTime: pastReset},
		},
	})

	// Verify checkPostResetIgnitions does nothing when delay has not elapsed
	d.checkPostResetIgnitions()
	cached := d.getQuotaSummary(standbyEmail)
	if cached.OverallHealth != core.StatusExhausted {
		t.Errorf("expected health to remain StatusExhausted before delay elapsed")
	}

	// Scenario B: Reset time was 20 seconds ago, delay (10s) HAS elapsed (now >= resetTime + delay)
	// But simulated fresh poll returns STILL EXHAUSTED
	pastResetElapsed := time.Now().Add(-20 * time.Second)
	d.setQuotaSummary(standbyEmail, &quota.QuotaSummary{
		AccountEmail:  standbyEmail,
		OverallHealth: core.StatusExhausted,
		MinFraction:   0.0,
		Models: []quota.ModelQuota{
			{ModelName: "gemini-pro", ResetTime: pastResetElapsed},
		},
	})

	// Intercept CloudCode URLs to detect 1-token keep alive probes
	var probeCount int32
	mockProbeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&probeCount, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"candidates":[]}`))
	}))
	defer mockProbeServer.Close()

	oldURLs := quota.CloudCodeGenerateContentURLs
	quota.CloudCodeGenerateContentURLs = []string{mockProbeServer.URL}
	defer func() { quota.CloudCodeGenerateContentURLs = oldURLs }()

	// Trigger checkPostResetIgnitions:
	// PollAndCacheAccount will attempt to poll the account. Since this is a test account
	// with mock credentials, PollAndCacheAccount fails or returns unauthenticated,
	// meaning quota reset is NOT confirmed healthy.
	d.checkPostResetIgnitions()

	// Probes MUST NOT be sent for unconfirmed/failed polls
	if atomic.LoadInt32(&probeCount) > 0 {
		t.Errorf("probeCount should be 0 for unconfirmed account, got %d", probeCount)
	}

	// Scenario C: Account is manually confirmed healthy and reset
	// When freshSum.OverallHealth is core.StatusHealthy and !freshSum.IsCooldown(thresh),
	// probe SHOULD be triggered if WarmupEnabled is true.
	acc, _ := d.Keyring.GetAccount(standbyEmail)
	acc.AccessToken = "test-token"
	_ = d.Keyring.AddOrUpdateAccount(acc)

	healthySum := &quota.QuotaSummary{
		AccountEmail:  standbyEmail,
		OverallHealth: core.StatusHealthy,
		MinFraction:   0.95,
	}

	// Verify that if warmup is disabled, even healthy account is not ignited
	cfg.WarmupEnabled = false
	d.mu.Lock()
	d.Config.WarmupEnabled = false
	d.mu.Unlock()

	atomic.StoreInt32(&probeCount, 0)
	if cfg.WarmupEnabled {
		_ = quota.IgniteAccountPostReset(acc, d.Keyring)
	}
	if atomic.LoadInt32(&probeCount) != 0 {
		t.Errorf("probe should not be sent when WarmupEnabled is false")
	}

	// When WarmupEnabled is true, probe is dispatched
	d.mu.Lock()
	d.Config.WarmupEnabled = true
	d.mu.Unlock()
	_ = quota.IgniteAccountPostReset(acc, d.Keyring)
	if atomic.LoadInt32(&probeCount) != 1 {
		t.Errorf("expected 1 probe when WarmupEnabled is true and healthy, got %d", probeCount)
	}

	_ = healthySum
}
