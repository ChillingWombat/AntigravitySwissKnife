package webgui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/daemon"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/ipc"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

func TestAccountSwitchThroughHTTPUsesDaemonState(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", configDir)
	t.Setenv("HOME", configDir)
	t.Setenv("USERPROFILE", configDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	socketPath := filepath.Join(configDir, "daemon.sock")
	d, err := daemon.NewDaemon(core.DefaultConfig(), socketPath)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	if err := d.Keyring.AddOrUpdateAccount(&keyring.Account{Email: "primary@example.com", RefreshToken: "refresh-primary"}); err != nil {
		t.Fatalf("seed primary account: %v", err)
	}
	if err := d.Keyring.AddOrUpdateAccount(&keyring.Account{Email: "secondary@example.com", RefreshToken: "refresh-secondary"}); err != nil {
		t.Fatalf("seed secondary account: %v", err)
	}
	if err := d.Keyring.SetActiveAccount("primary@example.com"); err != nil {
		t.Fatalf("seed active account: %v", err)
	}
	if err := d.Start(); err != nil {
		t.Fatalf("daemon Start: %v", err)
	}
	t.Cleanup(func() { _ = d.Stop() })

	server, err := NewServer("127.0.0.1:0", d.Backend(), ipc.NewClient(socketPath))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	if err := server.Start(); err != nil {
		t.Fatalf("web server Start: %v", err)
	}
	t.Cleanup(func() { _ = server.Stop() })

	payload, _ := json.Marshal(map[string]interface{}{"email": "secondary@example.com", "relaunch_ide": false})
	resp, err := http.Post("http://"+server.Addr()+"/api/switch", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST /api/switch: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/switch status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var status struct {
		ActiveAccount string `json:"active_account"`
	}
	if err := ipc.NewClient(socketPath).Call("swiss.getStatus", nil, &status); err != nil {
		t.Fatalf("swiss.getStatus: %v", err)
	}
	if status.ActiveAccount != "secondary@example.com" {
		t.Fatalf("RPC active account = %q, want secondary@example.com", status.ActiveAccount)
	}
}

func TestStandaloneWebServerUsesExplicitRPCClient(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", configDir)
	t.Setenv("HOME", configDir)
	t.Setenv("USERPROFILE", configDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	socketPath := filepath.Join(configDir, "daemon.sock")
	d, err := daemon.NewDaemon(core.DefaultConfig(), socketPath)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	if err := d.Keyring.AddOrUpdateAccount(&keyring.Account{Email: "primary@example.com", RefreshToken: "refresh-primary"}); err != nil {
		t.Fatalf("seed primary account: %v", err)
	}
	if err := d.Start(); err != nil {
		t.Fatalf("daemon Start: %v", err)
	}
	t.Cleanup(func() { _ = d.Stop() })

	server, err := NewRPCServer("127.0.0.1:0", socketPath)
	if err != nil {
		t.Fatalf("NewRPCServer: %v", err)
	}
	if err := server.Start(); err != nil {
		t.Fatalf("web server Start: %v", err)
	}
	t.Cleanup(func() { _ = server.Stop() })

	accounts, err := server.accountsSnapshot()
	if err != nil {
		t.Fatalf("accountsSnapshot through RPC: %v", err)
	}
	if len(accounts) != 1 || accounts[0].Email != "primary@example.com" {
		t.Fatalf("RPC accounts = %+v, want primary account", accounts)
	}
}

func TestBackendConstructorFailureStopsStartup(t *testing.T) {
	server, err := NewServer("127.0.0.1:0", nil, nil)
	if err == nil {
		if server != nil {
			_ = server.Stop()
		}
		t.Fatal("NewServer accepted a missing account backend")
	}
}
