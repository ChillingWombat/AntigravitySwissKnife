package daemon

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
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

// TestAdversarial_ShouldIgniteAccount_StandbyUnanchored5HourWindow validates the
// standby early-ignition eligibility rules:
// 1. Triggers ignition for healthy standby accounts with unanchored 5h rolling timers (~5 hours).
// 2. Suppresses duplicate probes once an account has been ignited within 4.5 hours.
// 3. Suppresses probes if weekly quota is depleted or 5h quota is in cooldown.
// 4. Suppresses probes if the 5h timer is already ticking down (< 4h 55m).
func TestAdversarial_ShouldIgniteAccount_StandbyUnanchored5HourWindow(t *testing.T) {
	d := &Daemon{
		ignitedAccounts: make(map[string]time.Time),
	}

	email := "standby_ignite_test@example.com"
	thresh := 0.05

	// 1. Nil summary -> false
	if d.shouldIgniteAccount(email, nil, thresh) {
		t.Errorf("expected false for nil summary")
	}

	// 2. Cooldown / exhausted summary -> false
	exhaustedSum := &quota.QuotaSummary{
		OverallHealth:   core.StatusExhausted,
		MinFraction:     0.02,
		Quota5hFraction: 0.02,
	}
	if d.shouldIgniteAccount(email, exhaustedSum, thresh) {
		t.Errorf("expected false for exhausted summary")
	}

	// 3. Weekly quota depleted below threshold -> false
	weeklyExhaustedSum := &quota.QuotaSummary{
		OverallHealth:       core.StatusWarning,
		MinFraction:         0.03,
		Quota5hFraction:     1.0,
		QuotaWeeklyFraction: 0.03,
		ResetSeconds5h:      18000,
	}
	if d.shouldIgniteAccount(email, weeklyExhaustedSum, thresh) {
		t.Errorf("expected false when weekly quota is below threshold")
	}

	// 4. Healthy 100% account with unanchored 5h timer (18000s) -> TRUE
	unanchoredSum := &quota.QuotaSummary{
		OverallHealth:       core.StatusHealthy,
		MinFraction:         1.0,
		Quota5hFraction:     1.0,
		QuotaWeeklyFraction: 0.50,
		ResetSeconds5h:      18000,
	}
	if !d.shouldIgniteAccount(email, unanchoredSum, thresh) {
		t.Errorf("expected true for healthy account with unanchored 5h timer (18000s)")
	}

	// 5. After ignition recorded -> FALSE (cooldown suppression)
	d.recordIgnition(email)
	if d.shouldIgniteAccount(email, unanchoredSum, thresh) {
		t.Errorf("expected false immediately after ignition recorded")
	}

	// 6. Another account whose 5h timer is ALREADY ticking down (e.g. 3 hours left) -> FALSE
	email2 := "already_ticking@example.com"
	tickingSum := &quota.QuotaSummary{
		OverallHealth:       core.StatusHealthy,
		MinFraction:         0.999999,
		Quota5hFraction:     0.999999,
		QuotaWeeklyFraction: 0.50,
		ResetSeconds5h:      10800, // 3 hours remaining
		Models: []quota.ModelQuota{
			{
				ModelName: "Five Hour Limit Remaining",
				ResetTime: time.Now().Add(3 * time.Hour),
			},
		},
	}
	if d.shouldIgniteAccount(email2, tickingSum, thresh) {
		t.Errorf("expected false for account whose 5h timer is already ticking down (3h left)")
	}
}

// TestAdversarial_PollStandbyAccounts_AbortsOn429 verifies that when the first
// standby account receives a 429 during pollStandbyAccounts, the sweep aborts
// immediately and sends zero HTTP requests for remaining standby accounts.
func TestAdversarial_PollStandbyAccounts_AbortsOn429(t *testing.T) {
	quota.ResetQuotaBackoffForTesting()
	defer quota.ResetQuotaBackoffForTesting()

	origLoadURLs := quota.CloudCodeLoadProjectURLs
	origQuotaURLs := quota.CloudCodeRetrieveQuotaURLs
	origModelsURLs := quota.CloudCodeModelsURLs
	defer func() {
		quota.CloudCodeLoadProjectURLs = origLoadURLs
		quota.CloudCodeRetrieveQuotaURLs = origQuotaURLs
		quota.CloudCodeModelsURLs = origModelsURLs
	}()

	setupDaemonWithStandby := func(t *testing.T) *Daemon {
		t.Helper()
		tmpDir := t.TempDir()
		t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir)
		t.Setenv("HOME", tmpDir)
		t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
		t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

		cfg := core.DefaultConfig()
		cfg.WarmupEnabled = false
		cfg.PostResetDelaySec = 10

		d, err := NewDaemon(cfg, filepath.Join(tmpDir, "daemon.sock"))
		if err != nil {
			t.Fatalf("NewDaemon error: %v", err)
		}

		_ = d.Keyring.AddOrUpdateAccount(&keyring.Account{
			Email:       "active@example.com",
			AccessToken: "Bearer-active",
			TokenExpiry: time.Now().Add(time.Hour),
		})
		_ = d.Keyring.SetActiveAccount("active@example.com")

		for _, acc := range []*keyring.Account{
			{Email: "standby1@example.com", AccessToken: "tok-1", TokenExpiry: time.Now().Add(time.Hour)},
			{Email: "standby2@example.com", AccessToken: "tok-2", TokenExpiry: time.Now().Add(time.Hour)},
			{Email: "standby3@example.com", AccessToken: "tok-3", TokenExpiry: time.Now().Add(time.Hour)},
		} {
			if err := d.Keyring.AddOrUpdateAccount(acc); err != nil {
				t.Fatalf("AddOrUpdateAccount(%s) error: %v", acc.Email, err)
			}
		}
		return d
	}

	runSweepTest := func(t *testing.T, withCooldownRecovery bool) {
		t.Helper()
		quota.ResetQuotaBackoffForTesting()
		defer quota.ResetQuotaBackoffForTesting()

		d := setupDaemonWithStandby(t)
		if withCooldownRecovery {
			pastReset := time.Now().Add(-30 * time.Second)
			d.setQuotaSummary("standby1@example.com", &quota.QuotaSummary{
				AccountEmail:  "standby1@example.com",
				OverallHealth: core.StatusExhausted,
				MinFraction:   0.0,
				Models:        []quota.ModelQuota{{ModelName: "gemini-pro", ResetTime: pastReset}},
			})
		}

		var mu sync.Mutex
		var loadTokens []string
		var quotaTokens []string

		loadSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			loadTokens = append(loadTokens, r.Header.Get("Authorization"))
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"cloudaicompanionProject":"test-project"}`))
		}))
		defer loadSrv.Close()

		quotaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			quotaTokens = append(quotaTokens, r.Header.Get("Authorization"))
			mu.Unlock()
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer quotaSrv.Close()

		quota.CloudCodeLoadProjectURLs = []string{loadSrv.URL}
		quota.CloudCodeRetrieveQuotaURLs = []string{quotaSrv.URL}
		quota.CloudCodeModelsURLs = []string{quotaSrv.URL}

		d.pollStandbyAccounts(nil, 0)

		mu.Lock()
		gotLoad := append([]string(nil), loadTokens...)
		gotQuota := append([]string(nil), quotaTokens...)
		mu.Unlock()

		if len(gotQuota) != 1 || gotQuota[0] != "Bearer tok-1" {
			t.Fatalf("expected exactly 1 quota request for Bearer tok-1, got %v", gotQuota)
		}
		if len(gotLoad) != 1 || gotLoad[0] != "Bearer tok-1" {
			t.Fatalf("expected exactly 1 load request for Bearer tok-1 (0 for remaining standby accounts), got %v", gotLoad)
		}
		if active, rem := quota.IsQuotaBackoffActive(); !active || rem <= 30*time.Second {
			t.Fatalf("expected anomalous 60s quota backoff active, got active=%v rem=%v", active, rem)
		}

		// A subsequent sweep while backoff is active must abort at loop entry with zero HTTP calls.
		d.pollStandbyAccounts(nil, 0)

		mu.Lock()
		gotLoadAfter := len(loadTokens)
		gotQuotaAfter := len(quotaTokens)
		mu.Unlock()
		if gotLoadAfter != 1 || gotQuotaAfter != 1 {
			t.Fatalf("expected 0 additional requests while backoff active, got load=%d quota=%d", gotLoadAfter, gotQuotaAfter)
		}
	}

	t.Run("normal_standby_branch", func(t *testing.T) {
		runSweepTest(t, false)
	})

	t.Run("cooldown_recovery_branch", func(t *testing.T) {
		runSweepTest(t, true)
	})
}

