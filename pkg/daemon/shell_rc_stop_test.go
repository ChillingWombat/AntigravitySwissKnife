package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/ipc"
)

func TestDaemonStopPropagatesMalformedShellRCCleanup(t *testing.T) {
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
	if _, err := custommodels.NewStore(storeDir); err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	d := &Daemon{
		Config: core.DefaultConfig(),
		Server: &ipc.Server{},
		ctx:    ctx,
		cancel: cancel,
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
