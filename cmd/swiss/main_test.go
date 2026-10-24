package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

func TestCLIHelpAndVersion(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "version")
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("version failed: %v, out: %s", err, string(out))
	}
	if !strings.Contains(string(out), core.AppVersion) {
		t.Errorf("expected version to contain %s, got: %s", core.AppVersion, string(out))
	}
	if !strings.Contains(string(out), "("+runtime.Version()+")") {
		t.Errorf("expected version to contain runtime %s, got: %s", runtime.Version(), string(out))
	}

	cmdHelp := exec.Command("go", "run", ".", "help")
	cmdHelp.Env = os.Environ()
	outHelp, err := cmdHelp.CombinedOutput()
	if err != nil {
		t.Fatalf("help failed: %v, out: %s", err, string(outHelp))
	}
	if !strings.Contains(string(outHelp), "Usage:") {
		t.Errorf("expected help output, got: %s", string(outHelp))
	}
}

func TestCLIAccountsAndStatus(t *testing.T) {
	cmdStatus := exec.Command("go", "run", ".", "status", "--json")
	cmdStatus.Env = os.Environ()
	outStatus, err := cmdStatus.CombinedOutput()
	if err != nil {
		t.Fatalf("status failed: %v, out: %s", err, string(outStatus))
	}
	if !strings.Contains(string(outStatus), "active_account") {
		t.Errorf("expected JSON status with active_account, got: %s", string(outStatus))
	}

	missingSocket := filepath.Join(t.TempDir(), "missing.sock")
	cmdQuota := exec.Command("go", "run", ".", "quota", "--socket", missingSocket)
	cmdQuota.Env = append(os.Environ(), "ANTIGRAVITY_SWISS_CONFIG_DIR="+t.TempDir())
	outQuota, err := cmdQuota.CombinedOutput()
	if err == nil {
		t.Fatalf("expected quota without daemon to fail, got: %s", string(outQuota))
	}
	if !strings.Contains(string(outQuota), "Daemon unavailable") {
		t.Errorf("expected daemon unavailable message, got: %s", string(outQuota))
	}
}

func TestCLIQuotaWithoutDaemonDoesNotFabricate(t *testing.T) {
	missingSocket := filepath.Join(t.TempDir(), "missing.sock")
	cmd := exec.Command("go", "run", ".", "quota", "--json", "--socket", missingSocket)
	cmd.Env = append(os.Environ(), "ANTIGRAVITY_SWISS_CONFIG_DIR="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected quota without a daemon to fail, got: %s", string(out))
	}

	output := string(out)
	for _, want := range []string{`"error"`, `"daemon_running": false`} {
		if !strings.Contains(output, want) {
			t.Errorf("expected honest daemon-unavailable output containing %s, got: %s", want, output)
		}
	}
	if strings.Contains(output, "gemini-3.8-flash") {
		t.Errorf("quota failure must not fabricate model quotas: %s", output)
	}
}
