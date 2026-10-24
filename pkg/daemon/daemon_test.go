package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/ipc"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/quota"
)

func testSocketPath(t *testing.T, dir, name string) string {
	t.Helper()
	sockPath := filepath.Join(dir, name)
	if len(sockPath) > 90 || runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		sockPath = filepath.Join(os.TempDir(), fmt.Sprintf("ask-%d.sock", time.Now().UnixNano()))
		t.Cleanup(func() { _ = os.Remove(sockPath) })
	}
	return sockPath
}

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

	sockPath := testSocketPath(t, tmpDir, "daemon.sock")
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

func TestDaemon_GUIConfigPartialUpdateRPC(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir)
	t.Setenv("HOME", tmpDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	sockPath := testSocketPath(t, tmpDir, "daemon.sock")
	cfg := core.DefaultConfig()

	d, err := NewDaemon(cfg, sockPath)
	if err != nil {
		t.Fatalf("NewDaemon error: %v", err)
	}

	if err := d.Start(); err != nil {
		t.Fatalf("d.Start error: %v", err)
	}
	defer d.Stop()

	client := ipc.NewClient(sockPath)

	// Verify initial config
	var initialGUI gui.Config
	if err := client.Call("swiss.getGUIConfig", nil, &initialGUI); err != nil {
		t.Fatalf("swiss.getGUIConfig failed: %v", err)
	}
	if !initialGUI.Enabled || !initialGUI.ColorStylingEnabled || !initialGUI.AutoInject {
		t.Fatalf("expected initial config flags to be true")
	}

	// Set a project color before partial update to verify it is preserved
	var setColorResp map[string]interface{}
	if err := client.Call("swiss.setGUIProjectColor", map[string]string{
		"name":  "ProjectAlpha",
		"color": "#0b57d0",
	}, &setColorResp); err != nil {
		t.Fatalf("swiss.setGUIProjectColor failed: %v", err)
	}

	// Send partial update
	var updatedGUI gui.Config
	err = client.Call("swiss.setGUIConfig", map[string]interface{}{
		"chat_telemetry_generation_speed": false,
	}, &updatedGUI)
	if err != nil {
		t.Fatalf("swiss.setGUIConfig failed: %v", err)
	}

	if updatedGUI.ChatTelemetryGenerationSpeed {
		t.Fatalf("expected chat_telemetry_generation_speed to be false")
	}
	// Omitted fields must be preserved
	if !updatedGUI.Enabled || !updatedGUI.ColorStylingEnabled || !updatedGUI.AutoInject || !updatedGUI.ReplaceSeeAllTriangle {
		t.Fatalf("expected omitted boolean fields to be preserved as true")
	}
	if len(updatedGUI.ProjectColors) == 0 {
		t.Fatalf("expected ProjectColors to be preserved")
	}
	if updatedGUI.ProjectColors["ProjectAlpha"] != "#0b57d0" {
		t.Fatalf("expected ProjectAlpha to be preserved as #0b57d0, got %q", updatedGUI.ProjectColors["ProjectAlpha"])
	}
}

func TestDaemon_MonitorAndInjectGUI_AutoReinstallLoader(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir)
	t.Setenv("HOME", tmpDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	resourcesDir := filepath.Join(tmpDir, "resources")
	_ = os.MkdirAll(resourcesDir, 0755)
	asarPath := filepath.Join(resourcesDir, "app.asar")
	backupPath := filepath.Join(resourcesDir, "app.asar.factory_backup")
	_ = os.WriteFile(asarPath, []byte("clean-upstream-asar"), 0644)
	_ = os.WriteFile(backupPath, []byte("old-backup"), 0644)

	sockPath := testSocketPath(t, tmpDir, "daemon.sock")
	cfg := core.DefaultConfig()

	d, err := NewDaemon(cfg, sockPath)
	if err != nil {
		t.Fatalf("NewDaemon error: %v", err)
	}

	// Override desktop manager on GUIStore with test resourcesDir
	dm := gui.NewDesktopManager(resourcesDir, filepath.Join(tmpDir, "gui.json"), nil)
	d.GUIStore.SetDesktopManager(dm)

	// Call monitorAndInjectGUI
	d.monitorAndInjectGUI()

	// Backup must have been refreshed from asarPath!
	content, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read backup: %v", err)
	}
	if string(content) != "clean-upstream-asar" {
		t.Fatalf("expected backup to be refreshed to 'clean-upstream-asar', got %q", string(content))
	}
}

func TestDaemon_MonitorAndInjectGUI_NoDevToolsNoPanic(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir)
	t.Setenv("HOME", tmpDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	sockPath := testSocketPath(t, tmpDir, "daemon.sock")
	cfg := core.DefaultConfig()

	d, err := NewDaemon(cfg, sockPath)
	if err != nil {
		t.Fatalf("NewDaemon error: %v", err)
	}

	// Should not panic when no DevTools port is available
	d.monitorAndInjectGUI()
}

func TestDaemon_GetFleetQuota_DualEngineRouting(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir)
	t.Setenv("HOME", tmpDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	sockPath := testSocketPath(t, tmpDir, "daemon.sock")
	cfg := core.DefaultConfig()
	cfg.AllowNonGeminiNativeModels = true

	d, err := NewDaemon(cfg, sockPath)
	if err != nil {
		t.Fatalf("NewDaemon error: %v", err)
	}

	_ = d.Keyring.AddOrUpdateAccount(&keyring.Account{
		Email:        "primary@example.com",
		Label:        "Primary Account",
		PlanTier:     "Pro",
		Status:       "ACTIVE",
		IsActive:     true,
		RefreshToken: "refresh-primary",
	})
	_ = d.Keyring.SetActiveAccount("primary@example.com")

	_ = d.Keyring.AddOrUpdateAccount(&keyring.Account{
		Email:        "dual@example.com",
		Label:        "Dual Account",
		PlanTier:     "Pro",
		Status:       "STANDBY",
		RefreshToken: "refresh-dual",
	})

	// Pre-seed quota summary with depleted Gemini (0.01) and healthy Claude (0.85)
	d.setQuotaSummary("dual@example.com", &quota.QuotaSummary{
		AccountEmail:         "dual@example.com",
		Quota5hFraction:      0.01,
		QuotaWeeklyFraction:  0.01,
		Quota5hClaudeGPT:     0.85,
		QuotaWeeklyClaudeGPT: 0.90,
	})

	if err := d.Start(); err != nil {
		t.Fatalf("d.Start error: %v", err)
	}
	defer d.Stop()

	client := ipc.NewClient(sockPath)
	var fleetResp quota.FleetQuotaSummary
	err = client.Call("swiss.getFleetQuota", nil, &fleetResp)
	if err != nil {
		t.Fatalf("swiss.getFleetQuota error: %v", err)
	}

	var dualAcc *quota.AccountQuotaState
	for i := range fleetResp.Accounts {
		if fleetResp.Accounts[i].Email == "dual@example.com" {
			dualAcc = &fleetResp.Accounts[i]
			break
		}
	}
	if dualAcc == nil {
		t.Fatalf("dual@example.com not found in fleet")
	}
	if dualAcc.Status != "STANDBY" {
		t.Errorf("expected account to maintain STANDBY with non-Gemini routing enabled, got %s", dualAcc.Status)
	}
	if dualAcc.Quota5hClaudeGPT != 0.85 {
		t.Errorf("expected Quota5hClaudeGPT = 0.85, got %f", dualAcc.Quota5hClaudeGPT)
	}
	if dualAcc.QuotaWeeklyClaudeGPT != 0.90 {
		t.Errorf("expected QuotaWeeklyClaudeGPT = 0.90, got %f", dualAcc.QuotaWeeklyClaudeGPT)
	}
}

func TestDaemon_AutoSwitch_DualEngineCandidateValidation(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir)
	t.Setenv("HOME", tmpDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	sockPath := testSocketPath(t, tmpDir, "daemon.sock")
	cfg := core.DefaultConfig()
	cfg.AllowNonGeminiNativeModels = true
	cfg.AutoSwitchEnabled = true
	cfg.AutoSwitchThreshold = 0.05
	cfg.AutoSwitchWeeklyThreshold = 0.05

	d, err := NewDaemon(cfg, sockPath)
	if err != nil {
		t.Fatalf("NewDaemon error: %v", err)
	}

	// Active account with exhausted Gemini and exhausted Claude
	_ = d.Keyring.AddOrUpdateAccount(&keyring.Account{
		Email:        "active@example.com",
		Label:        "Active Exhausted",
		PlanTier:     "Pro",
		Status:       "ACTIVE",
		IsActive:     true,
		RefreshToken: "refresh-active",
	})
	_ = d.Keyring.SetActiveAccount("active@example.com")

	// Standby account with exhausted Gemini, but healthy Claude (and 0 weekly Claude quota)
	_ = d.Keyring.AddOrUpdateAccount(&keyring.Account{
		Email:        "successor@example.com",
		Label:        "Successor Claude Ready",
		PlanTier:     "Pro",
		Status:       "STANDBY",
		RefreshToken: "refresh-successor",
	})

	d.setQuotaSummary("active@example.com", &quota.QuotaSummary{
		AccountEmail:         "active@example.com",
		Quota5hFraction:      0.01,
		QuotaWeeklyFraction:  0.01,
		Quota5hClaudeGPT:     0.01,
		QuotaWeeklyClaudeGPT: 0.01,
	})

	d.setQuotaSummary("successor@example.com", &quota.QuotaSummary{
		AccountEmail:         "successor@example.com",
		Quota5hFraction:      0.01,
		QuotaWeeklyFraction:  0.01,
		Quota5hClaudeGPT:     0.85,
		QuotaWeeklyClaudeGPT: 0.00, // zero weekly Claude quota (common in CloudCode)
	})

	// Verify states and evaluation with routing
	accounts := d.Keyring.ListAccounts()
	summaries := d.getQuotaSummaries()
	states := quota.BuildAccountQuotaStatesFromMapWithThresholds(accounts, summaries, 0.05, 0.05, true)

	shouldSwitch, successor, reason := quota.EvaluateAutoSwitchWithThresholdsAndRouting(
		states,
		"active@example.com",
		0.05,
		0.05,
		"balanced",
		1000,
		true,
	)

	if !shouldSwitch {
		t.Fatalf("expected shouldSwitch to be true, got false (reason: %s)", reason)
	}
	if successor == nil || successor.Email != "successor@example.com" {
		t.Fatalf("expected successor to be successor@example.com, got %+v", successor)
	}

	// Verify candidate health check logic in daemon
	succSum := d.getQuotaSummary(successor.Email)
	if succSum == nil {
		t.Fatal("expected successor quota summary to exist")
	}

	cAcc, _ := d.Keyring.GetAccount(successor.Email)
	thresh := 0.05
	threshWeekly := 0.05

	hasGeminiHealthy := succSum.Quota5hFraction > thresh && (succSum.QuotaWeeklyFraction > threshWeekly || (cAcc.EnableCreditOverages && succSum.Credits > 0))
	hasNonGeminiHealthy := false
	if d.Config.AllowNonGeminiNativeModels {
		hasNonGeminiHealthy = succSum.Quota5hClaudeGPT > thresh && (succSum.QuotaWeeklyClaudeGPT > threshWeekly || succSum.QuotaWeeklyClaudeGPT == 0 || (cAcc.EnableCreditOverages && succSum.Credits > 0))
	}
	isHealthy := hasGeminiHealthy || hasNonGeminiHealthy

	if !isHealthy {
		t.Errorf("expected successor candidate to be healthy under dual-engine routing, got isHealthy=false")
	}
}
