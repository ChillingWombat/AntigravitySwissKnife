package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVaultManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vault-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	convDir := filepath.Join(tmpDir, "conversations")
	vaultDir := filepath.Join(tmpDir, "vault", "conversations")
	annoDir := filepath.Join(tmpDir, "annotations")
	vaultAnnoDir := filepath.Join(tmpDir, "vault", "annotations")

	_ = os.MkdirAll(convDir, 0755)
	_ = os.MkdirAll(annoDir, 0755)

	// Create test conversation files
	c1 := "conv-1.db"
	c2 := "conv-2.db"
	_ = os.WriteFile(filepath.Join(convDir, c1), []byte("sqlite-conv-1-data"), 0644)
	_ = os.WriteFile(filepath.Join(convDir, c2), []byte("sqlite-conv-2-data"), 0644)

	mgr := &Manager{
		conversationsDir: convDir,
		vaultDir:         vaultDir,
		annotationsDir:   annoDir,
		vaultAnnoDir:     vaultAnnoDir,
	}

	// 1. Initial Sync: Should vault both conversations
	res, err := mgr.Sync()
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if res.NewVaulted != 2 {
		t.Errorf("expected 2 new vaulted, got %d", res.NewVaulted)
	}
	if res.TotalVaulted != 2 {
		t.Errorf("expected 2 total vaulted, got %d", res.TotalVaulted)
	}

	// Verify vaulted file exists
	if _, err := os.Stat(filepath.Join(vaultDir, c1)); err != nil {
		t.Errorf("vaulted file %s does not exist", c1)
	}

	// 2. Simulate Antigravity Pruning: Delete c1 from live conversations
	if err := os.Remove(filepath.Join(convDir, c1)); err != nil {
		t.Fatalf("failed to simulate pruning: %v", err)
	}
	if _, err := os.Stat(filepath.Join(convDir, c1)); !os.IsNotExist(err) {
		t.Fatalf("expected c1 to be deleted from live convDir")
	}

	// 3. Second Sync: Should detect pruned c1 and AUTO-RESCUE it back into convDir!
	res2, err := mgr.Sync()
	if err != nil {
		t.Fatalf("second Sync failed: %v", err)
	}
	if res2.RescuedCount != 1 {
		t.Errorf("expected 1 rescued, got %d", res2.RescuedCount)
	}
	if len(res2.RescuedIDs) != 1 || res2.RescuedIDs[0] != "conv-1" {
		t.Errorf("expected rescued ID conv-1, got %v", res2.RescuedIDs)
	}

	// Verify c1 is restored in convDir!
	restoredContent, err := os.ReadFile(filepath.Join(convDir, c1))
	if err != nil {
		t.Fatalf("rescued file c1 does not exist in convDir: %v", err)
	}
	if string(restoredContent) != "sqlite-conv-1-data" {
		t.Errorf("expected content 'sqlite-conv-1-data', got %q", string(restoredContent))
	}

	// 4. Status Check
	status, err := mgr.GetStatus(true)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.LiveCount != 2 {
		t.Errorf("expected 2 live, got %d", status.LiveCount)
	}
	if status.VaultedCount != 2 {
		t.Errorf("expected 2 vaulted, got %d", status.VaultedCount)
	}
	if status.RescuedCount != 1 {
		t.Errorf("expected 1 rescued count, got %d", status.RescuedCount)
	}
}
