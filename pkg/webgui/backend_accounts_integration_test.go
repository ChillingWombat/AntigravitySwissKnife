package webgui

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/daemon"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
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

	totpBody, _ := json.Marshal(map[string]interface{}{
		"email":  "secondary@example.com",
		"secret": "JBSWY3DPEHPK3PXP",
	})
	reqTOTP, err := http.NewRequest(http.MethodPost, "http://"+server.Addr()+"/api/totp", bytes.NewReader(totpBody))
	if err != nil {
		t.Fatalf("create TOTP request: %v", err)
	}
	reqTOTP.Header.Set("Content-Type", "application/json")
	reqTOTP.Header.Set("X-Session-Token", server.GetSessionToken())
	respTOTP, err := http.DefaultClient.Do(reqTOTP)
	if err != nil {
		t.Fatalf("POST /api/totp: %v", err)
	}
	respTOTP.Body.Close()
	if respTOTP.StatusCode != http.StatusOK {
		t.Fatalf("POST /api/totp status = %d, want %d", respTOTP.StatusCode, http.StatusOK)
	}

	var totp map[string]interface{}
	if err := ipc.NewClient(socketPath).Call("swiss.getTOTPCode", map[string]string{"email": "secondary@example.com"}, &totp); err != nil {
		t.Fatalf("swiss.getTOTPCode: %v", err)
	}
	if totp["has_totp"] != true {
		t.Fatalf("RPC TOTP state = %+v, want has_totp=true", totp)
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
	t.Run("missing backend", func(t *testing.T) {
		server, err := NewServer("127.0.0.1:0", nil, nil)
		if err == nil {
			if server != nil {
				_ = server.Stop()
			}
			t.Fatal("NewServer accepted a missing account backend")
		}
	})

	t.Run("required store failure", func(t *testing.T) {
		original := newGUIConfigStore
		newGUIConfigStore = func(string) (*gui.Store, error) {
			return nil, errors.New("injected GUI store failure")
		}
		t.Cleanup(func() { newGUIConfigStore = original })

		_, err := NewServer(
			"127.0.0.1:0",
			testBackend{accounts: testAccountService{}},
			ipc.NewClient(filepath.Join(t.TempDir(), "unused.sock")),
		)
		if err == nil {
			t.Fatal("NewServer ignored a required store failure")
		}
		if !strings.Contains(err.Error(), "initialize GUI store") ||
			!strings.Contains(err.Error(), "injected GUI store failure") {
			t.Fatalf("NewServer error = %q, want wrapped GUI store failure", err)
		}
	})
}
