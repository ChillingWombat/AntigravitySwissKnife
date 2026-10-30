package webgui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/quota"
)

// newManualSwitchTestServer spins up an isolated webgui server for manual
// switch tests. The config dir and HOME are redirected to temp dirs so the
// keyring store, quota cache, and cloud account sync never touch real user
// data. The first imported account becomes the active account.
func newManualSwitchTestServer(t *testing.T) *Server {
	t.Helper()
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	tmpDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir)
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	store, err := keyring.NewStore("")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	if _, err := store.ImportAccount("active-user@google.com", "rt_active", "at_active", "Active User", ""); err != nil {
		t.Fatalf("ImportAccount(active-user) error: %v", err)
	}

	srv := newTestServer("127.0.0.1:0", filepath.Join(tmpDir, "dummy.sock"))
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start error: %v", err)
	}
	t.Cleanup(func() { _ = srv.Stop() })
	return srv
}

func postManualSwitch(t *testing.T, srv *Server, email string) map[string]interface{} {
	t.Helper()
	payload, _ := json.Marshal(map[string]interface{}{
		"email":        email,
		"relaunch_ide": false,
	})
	resp, err := http.Post("http://"+srv.Addr()+"/api/switch", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST /api/switch error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/switch returned status %d", resp.StatusCode)
	}
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode switch response error: %v", err)
	}
	return result
}

func importTestAccount(t *testing.T, email string) {
	t.Helper()
	store, err := keyring.NewStore("")
	if err != nil {
		t.Fatalf("failed to open test store: %v", err)
	}
	if _, err := store.ImportAccount(email, "rt_"+email, "at_"+email, "Test User", ""); err != nil {
		t.Fatalf("ImportAccount(%s) error: %v", email, err)
	}
}

// seedQuotaCache writes a quota summary for the account so the status
// computation sees polled data. Fractions below the 0.05 threshold yield
// COOLING; fractions above yield STANDBY.
func seedQuotaCache(t *testing.T, email string, fraction5h, fractionWeekly float64) {
	t.Helper()
	if err := quota.SaveQuotaCache(map[string]*quota.QuotaSummary{
		email: {
			AccountEmail:        email,
			PlanTier:            "pro",
			Quota5hFraction:     fraction5h,
			QuotaWeeklyFraction: fractionWeekly,
		},
	}); err != nil {
		t.Fatalf("SaveQuotaCache error: %v", err)
	}
}

// Manual switching to an account that is NOT in STANDBY (here COOLING because
// its quota is below the auto-switch threshold) must turn auto-switch off so
// the rotation daemon does not immediately switch away from the pinned account.
func TestManualSwitchToNonStandbyAccountDisablesAutoSwitch(t *testing.T) {
	srv := newManualSwitchTestServer(t)

	importTestAccount(t, "cooling-user@google.com")
	seedQuotaCache(t, "cooling-user@google.com", 0.01, 0.01)

	// Sanity: auto-switch is enabled by default.
	cfg, err := core.LoadConfig()
	if err != nil || cfg == nil || !cfg.AutoSwitchEnabled {
		t.Fatalf("expected auto-switch enabled by default, cfg=%+v err=%v", cfg, err)
	}

	result := postManualSwitch(t, srv, "cooling-user@google.com")

	if success, ok := result["success"].(bool); !ok || !success {
		t.Errorf("expected success true in switch response, got %v", result)
	}
	if disabled, ok := result["auto_switch_disabled"].(bool); !ok || !disabled {
		t.Errorf("expected auto_switch_disabled true for non-STANDBY target, got %v", result["auto_switch_disabled"])
	}

	cfg, err = core.LoadConfig()
	if err != nil || cfg == nil {
		t.Fatalf("LoadConfig after switch error: %v", err)
	}
	if cfg.AutoSwitchEnabled {
		t.Errorf("expected auto-switch disabled in config after manual switch to non-STANDBY account")
	}
}

// Manual switching to a STANDBY account must leave the auto-switch setting
// untouched: the rotation daemon may keep managing the fleet.
func TestManualSwitchToStandbyAccountKeepsAutoSwitch(t *testing.T) {
	srv := newManualSwitchTestServer(t)

	importTestAccount(t, "standby-user@google.com")
	// Healthy quota => status STANDBY.
	seedQuotaCache(t, "standby-user@google.com", 0.9, 0.9)

	result := postManualSwitch(t, srv, "standby-user@google.com")

	if success, ok := result["success"].(bool); !ok || !success {
		t.Errorf("expected success true in switch response, got %v", result)
	}
	if disabled, ok := result["auto_switch_disabled"].(bool); ok && disabled {
		t.Errorf("expected auto_switch_disabled absent/false for STANDBY target, got %v", result["auto_switch_disabled"])
	}

	cfg, err := core.LoadConfig()
	if err != nil || cfg == nil {
		t.Fatalf("LoadConfig after switch error: %v", err)
	}
	if !cfg.AutoSwitchEnabled {
		t.Errorf("expected auto-switch to remain enabled after manual switch to STANDBY account")
	}
}

// When auto-switch is already disabled, a manual switch to a non-STANDBY
// account keeps it disabled and must not report a state change.
func TestManualSwitchNonStandbyWithAutoSwitchAlreadyDisabled(t *testing.T) {
	srv := newManualSwitchTestServer(t)

	importTestAccount(t, "cooling-user@google.com")
	seedQuotaCache(t, "cooling-user@google.com", 0.01, 0.01)

	cfg, err := core.LoadConfig()
	if err != nil || cfg == nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	cfg.AutoSwitchEnabled = false
	if err := cfg.Save(); err != nil {
		t.Fatalf("cfg.Save error: %v", err)
	}

	result := postManualSwitch(t, srv, "cooling-user@google.com")

	if success, ok := result["success"].(bool); !ok || !success {
		t.Errorf("expected success true in switch response, got %v", result)
	}
	if disabled, ok := result["auto_switch_disabled"].(bool); ok && disabled {
		t.Errorf("expected auto_switch_disabled absent/false when auto-switch already off, got %v", result["auto_switch_disabled"])
	}

	cfg, err = core.LoadConfig()
	if err != nil || cfg == nil {
		t.Fatalf("LoadConfig after switch error: %v", err)
	}
	if cfg.AutoSwitchEnabled {
		t.Errorf("expected auto-switch to stay disabled after manual switch to non-STANDBY account")
	}
}
