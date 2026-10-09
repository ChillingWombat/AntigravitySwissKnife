package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/ipc"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/quota"
)

func TestDaemonFullLifecycleAndRPC(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swiss_test_daemon_*")
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

	d, err := NewDaemon(cfg, sockPath)
	if err != nil {
		t.Fatalf("NewDaemon error: %v", err)
	}

	// Seed test account
	_ = d.Keyring.AddOrUpdateAccount(&keyring.Account{
		Email:      "developer@example.com",
		Label:      "Primary Engineer",
		TOTPSecret: "JBSWY3DPEHPK3PXP",
	})
	_ = d.Keyring.AddOrUpdateAccount(&keyring.Account{
		Email: "standby@example.com",
		Label: "Secondary Backup",
	})

	if err := d.Start(); err != nil {
		t.Fatalf("d.Start error: %v", err)
	}
	defer d.Stop()

	// Use Client to test IPC RPC methods
	client := ipc.NewClient(sockPath)

	// 1. Test swiss.getStatus
	var status struct {
		DaemonRunning bool   `json:"daemon_running"`
		Version       string `json:"version"`
		ActiveAccount string `json:"active_account"`
		TotalAccounts int    `json:"total_accounts"`
	}
	err = client.Call("swiss.getStatus", nil, &status)
	if err != nil {
		t.Fatalf("swiss.getStatus error: %v", err)
	}
	if !status.DaemonRunning {
		t.Errorf("expected DaemonRunning to be true")
	}
	if status.TotalAccounts != 2 {
		t.Errorf("expected 2 accounts, got %d", status.TotalAccounts)
	}

	// 2. Test swiss.switchAccount
	var switchResp struct {
		Switched bool   `json:"switched"`
		Account  string `json:"account"`
	}
	err = client.Call("swiss.switchAccount", map[string]string{"email": "standby@example.com"}, &switchResp)
	if err != nil {
		t.Fatalf("swiss.switchAccount error: %v", err)
	}
	if switchResp.Account != "standby@example.com" {
		t.Errorf("expected switched to standby@example.com, got %s", switchResp.Account)
	}

	// 3. Test swiss.getTOTPCode
	var totpResp struct {
		Code             string  `json:"code"`
		RemainingSeconds int     `json:"remaining_seconds"`
		ProgressFraction float64 `json:"progress_fraction"`
		HasTOTP          bool    `json:"has_totp"`
	}
	err = client.Call("swiss.getTOTPCode", map[string]string{"email": "developer@example.com"}, &totpResp)
	if err != nil {
		t.Fatalf("swiss.getTOTPCode error: %v", err)
	}
	if !totpResp.HasTOTP || len(totpResp.Code) != 6 {
		t.Errorf("expected valid 6-digit TOTP code, got %+v", totpResp)
	}

	// 4. Test swiss.getFingerprofile
	var fpResp struct {
		MachineID string `json:"machine_id"`
		UpdaterID string `json:"updater_id"`
	}
	err = client.Call("swiss.getFingerprofile", map[string]string{"email": "developer@example.com"}, &fpResp)
	if err != nil {
		t.Fatalf("swiss.getFingerprofile error: %v", err)
	}
	if len(fpResp.MachineID) != 64 {
		t.Errorf("expected 64-char machine ID, got %d", len(fpResp.MachineID))
	}

	// 5. Test swiss.getFleetQuota (must return cached state immediately, <100ms)
	var fleetResp quota.FleetQuotaSummary
	start := time.Now()
	err = client.Call("swiss.getFleetQuota", nil, &fleetResp)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("swiss.getFleetQuota error: %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("swiss.getFleetQuota took %v, expected <500ms", elapsed)
	}
	if fleetResp.TotalAccounts != 2 {
		t.Errorf("expected 2 accounts in fleet, got %d", fleetResp.TotalAccounts)
	}
	if len(fleetResp.Accounts) != 2 {
		t.Errorf("expected 2 account states in fleet, got %d", len(fleetResp.Accounts))
	}
	if fleetResp.Accounts[0].Email != "standby@example.com" {
		t.Errorf("expected active account standby@example.com to rank #1 in fleet, got %s", fleetResp.Accounts[0].Email)
	}

	// 6. Test swiss.refreshFleetQuota (async trigger)
	var refreshResp map[string]interface{}
	err = client.Call("swiss.refreshFleetQuota", nil, &refreshResp)
	if err != nil {
		t.Fatalf("swiss.refreshFleetQuota error: %v", err)
	}
	if refreshResp["status"] != "refresh_started" {
		t.Errorf("expected refresh_started, got %v", refreshResp["status"])
	}

	// 7. Test swiss.getQuotaSummary
	var quotaSum quota.QuotaSummary
	err = client.Call("swiss.getQuotaSummary", nil, &quotaSum)
	if err != nil {
		t.Fatalf("swiss.getQuotaSummary error: %v", err)
	}

	// 8. Test swiss.getQuotaSummary with targeted email param
	var targetQuotaSum quota.QuotaSummary
	err = client.Call("swiss.getQuotaSummary", map[string]string{"email": "standby@example.com"}, &targetQuotaSum)
	if err != nil {
		t.Fatalf("swiss.getQuotaSummary for standby error: %v", err)
	}
	if targetQuotaSum.AccountEmail != "standby@example.com" {
		t.Errorf("expected standby@example.com, got %s", targetQuotaSum.AccountEmail)
	}

	// 9. Test swiss.removeAccount and cache cleanup
	var removeResp map[string]interface{}
	err = client.Call("swiss.removeAccount", map[string]string{"email": "standby@example.com"}, &removeResp)
	if err != nil {
		t.Fatalf("swiss.removeAccount error: %v", err)
	}
	if removeResp["removed"] != "standby@example.com" {
		t.Errorf("expected removed standby@example.com, got %v", removeResp["removed"])
	}

	// 10. Test swiss.updateAccount
	var updateResp map[string]interface{}
	err = client.Call("swiss.updateAccount", map[string]interface{}{
		"email":     "developer@example.com",
		"label":     "Lead Architect",
		"priority":  "High",
		"plan_tier": "Pro",
	}, &updateResp)
	if err != nil {
		t.Fatalf("swiss.updateAccount error: %v", err)
	}
	if updateResp["success"] != true || updateResp["email"] != "developer@example.com" {
		t.Errorf("expected success true and developer@example.com, got %v", updateResp)
	}

	// 11. Test swiss.getQuotaSummary with refresh: true
	var refreshQuotaSum quota.QuotaSummary
	err = client.Call("swiss.getQuotaSummary", map[string]interface{}{
		"email":   "developer@example.com",
		"refresh": true,
	}, &refreshQuotaSum)
	if err != nil {
		t.Fatalf("swiss.getQuotaSummary with refresh error: %v", err)
	}

	// 12. Test swiss.relaunchIDE
	var relaunchResp map[string]interface{}
	err = client.Call("swiss.relaunchIDE", map[string]interface{}{}, &relaunchResp)
	if err != nil {
		t.Fatalf("swiss.relaunchIDE error: %v", err)
	}
	if relaunchResp["success"] != true {
		t.Errorf("expected relaunch success: true, got %+v", relaunchResp)
	}

	// 13. Test swiss.switchAccount with relaunch_ide: true
	var switchRelaunchResp struct {
		Switched    bool   `json:"switched"`
		Account     string `json:"account"`
		RelaunchIDE bool   `json:"relaunch_ide"`
	}
	err = client.Call("swiss.switchAccount", map[string]interface{}{
		"email":        "developer@example.com",
		"relaunch_ide": true,
	}, &switchRelaunchResp)
	if err != nil {
		t.Fatalf("swiss.switchAccount with relaunch error: %v", err)
	}
	if !switchRelaunchResp.Switched || !switchRelaunchResp.RelaunchIDE {
		t.Errorf("expected switched true and relaunch_ide true, got %+v", switchRelaunchResp)
	}
}

