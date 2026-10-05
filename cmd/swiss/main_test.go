package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

func TestCLIHelpAndVersion(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "version")
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":/home/david/.local/go/bin")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("version failed: %v, out: %s", err, string(out))
	}
	if !strings.Contains(string(out), core.AppVersion) {
		t.Errorf("expected version to contain %s, got: %s", core.AppVersion, string(out))
	}

	cmdHelp := exec.Command("go", "run", ".", "help")
	cmdHelp.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":/home/david/.local/go/bin")
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
	cmdStatus.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":/home/david/.local/go/bin")
	outStatus, err := cmdStatus.CombinedOutput()
	if err != nil {
		t.Fatalf("status failed: %v, out: %s", err, string(outStatus))
	}
	if !strings.Contains(string(outStatus), "active_account") {
		t.Errorf("expected JSON status with active_account, got: %s", string(outStatus))
	}

	cmdQuota := exec.Command("go", "run", ".", "quota")
	cmdQuota.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":/home/david/.local/go/bin")
	outQuota, err := cmdQuota.CombinedOutput()
	if err != nil {
		t.Fatalf("quota failed: %v, out: %s", err, string(outQuota))
	}
	if !strings.Contains(string(outQuota), "Gemini Model Quota Horizons") {
		t.Errorf("expected quota output with table header, got: %s", string(outQuota))
	}
}
