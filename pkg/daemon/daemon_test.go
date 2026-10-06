package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/ipc"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
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
}
