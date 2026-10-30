package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/github"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/ipc"
)

func TestDaemonStopUsesInjectedCustomModelStoreForShellRCCleanup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	rc := filepath.Join(home, ".zshrc")
	original := "before\n# >>> Antigravity Swiss Knife CLI Integration >>>\nexport CLOUD_CODE_URL=\"http://127.0.0.1:8765\"\n"
	if err := os.WriteFile(rc, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	storeDir := t.TempDir()
	config, err := json.Marshal(map[string]any{
		"integration": map[string]any{"mode": custommodels.IntegrationAPIProxy},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storeDir, "custom_models.json"), config, 0o600); err != nil {
		t.Fatal(err)
	}
	previousStore := custommodels.GetGlobalStore()
	t.Cleanup(func() { custommodels.SetGlobalStore(previousStore) })
	injectedStore, err := custommodels.NewStore(storeDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	globalStore, err := custommodels.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore global: %v", err)
	}
	custommodels.SetGlobalStore(globalStore)

	ctx, cancel := context.WithCancel(context.Background())
	d := &Daemon{
		Config:       core.DefaultConfig(),
		CustomModels: injectedStore,
		Server:       &ipc.Server{},
		ctx:          ctx,
		cancel:       cancel,
	}
	err = d.Stop()
	if err == nil || !strings.Contains(err.Error(), "clean shell RC hooks during stop") {
		t.Fatalf("Stop error = %v, want shell RC cleanup failure", err)
	}
	data, err := os.ReadFile(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("malformed shell RC changed: %q", string(data))
	}
	if _, err := os.Stat(rc + ".swiss.lock"); !os.IsNotExist(err) {
		t.Fatalf("malformed shell RC created cleanup lock, stat error = %v", err)
	}
}

func TestNewDaemonWrapsGitHubOwnerFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	original := newGitHubStore
	newGitHubStore = func(string) (*github.Store, error) {
		return nil, errors.New("github constructor failed")
	}
	t.Cleanup(func() { newGitHubStore = original })

	_, err := NewDaemon(core.DefaultConfig(), filepath.Join(home, "daemon.sock"))
	if err == nil || !strings.Contains(err.Error(), "GitHub") {
		t.Fatalf("NewDaemon error = %v, want wrapped GitHub constructor failure", err)
	}
}
