package vault

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
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

func TestVaultManualDeletionSyncsToVault(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vault-manual-delete-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	convDir := filepath.Join(tmpDir, "conversations")
	vaultDir := filepath.Join(tmpDir, "vault", "conversations")
	annoDir := filepath.Join(tmpDir, "annotations")
	vaultAnnoDir := filepath.Join(tmpDir, "vault", "annotations")
	summariesDB := filepath.Join(tmpDir, "conversation_summaries.db")

	_ = os.MkdirAll(convDir, 0755)
	_ = os.MkdirAll(annoDir, 0755)

	// Create summaries DB with conv-keep and conv-delete
	db, err := sql.Open("sqlite", summariesDB)
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE conversation_summaries (conversation_id TEXT PRIMARY KEY)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	_, _ = db.Exec(`INSERT INTO conversation_summaries (conversation_id) VALUES ('conv-keep'), ('conv-delete')`)
	_ = db.Close()

	_ = os.WriteFile(filepath.Join(convDir, "conv-keep.db"), []byte("keep-data"), 0644)
	_ = os.WriteFile(filepath.Join(convDir, "conv-delete.db"), []byte("delete-data"), 0644)
	_ = os.WriteFile(filepath.Join(annoDir, "conv-delete.pbtxt"), []byte("title:\"deleted\""), 0644)

	mgr := &Manager{
		conversationsDir: convDir,
		vaultDir:         vaultDir,
		annotationsDir:   annoDir,
		vaultAnnoDir:     vaultAnnoDir,
		summariesDBPath:  summariesDB,
	}

	// 1. Initial Sync vaults both
	res1, err := mgr.Sync()
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if res1.TotalVaulted != 2 {
		t.Fatalf("expected 2 vaulted, got %d", res1.TotalVaulted)
	}

	// 2. Simulate user manually deleting 'conv-delete' in Antigravity (removed from conversation_summaries.db and conversations/)
	// and Antigravity 500-session limit pruning 'conv-keep' (kept in conversation_summaries.db, removed from conversations/)
	db2, err := sql.Open("sqlite", summariesDB)
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	_, _ = db2.Exec(`DELETE FROM conversation_summaries WHERE conversation_id = 'conv-delete'`)
	_ = db2.Close()

	_ = os.Remove(filepath.Join(convDir, "conv-delete.db"))
	_ = os.Remove(filepath.Join(annoDir, "conv-delete.pbtxt"))
	_ = os.Remove(filepath.Join(convDir, "conv-keep.db"))

	// 3. Sync: conv-delete MUST be deleted from the vault, while conv-keep MUST be rescued!
	res2, err := mgr.Sync()
	if err != nil {
		t.Fatalf("Sync 2 failed: %v", err)
	}
	if res2.DeletedCount != 1 {
		t.Errorf("expected 1 deleted from vault, got %d", res2.DeletedCount)
	}
	if res2.RescuedCount != 1 {
		t.Errorf("expected 1 rescued from vault, got %d", res2.RescuedCount)
	}
	if _, err := os.Stat(filepath.Join(vaultDir, "conv-delete.db")); !os.IsNotExist(err) {
		t.Errorf("expected conv-delete.db to be removed from vault")
	}
	if _, err := os.Stat(filepath.Join(vaultAnnoDir, "conv-delete.pbtxt")); !os.IsNotExist(err) {
		t.Errorf("expected conv-delete.pbtxt to be removed from vault annotations")
	}
	if _, err := os.Stat(filepath.Join(convDir, "conv-delete.db")); !os.IsNotExist(err) {
		t.Errorf("expected conv-delete.db NOT to be restored into live conversations")
	}
	if _, err := os.Stat(filepath.Join(convDir, "conv-keep.db")); err != nil {
		t.Errorf("expected conv-keep.db to be rescued into live conversations: %v", err)
	}
}

