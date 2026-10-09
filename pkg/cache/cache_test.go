package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCacheInspectorAndPrunerShield(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swiss_test_cache_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	brainDir := filepath.Join(tmpDir, "brain")
	convDir := filepath.Join(tmpDir, "conversations")

	// 1. Stale conversation
	staleConv := filepath.Join(brainDir, "conv-stale", "scratch")
	os.MkdirAll(staleConv, 0700)
	staleFile := filepath.Join(staleConv, "old.txt")
	os.WriteFile(staleFile, []byte("stale data to prune"), 0600)
	// Set mod time to 10 days ago
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	os.Chtimes(staleFile, oldTime, oldTime)

	// 2. Active shielded conversation
	activeCascadeID := "cascade-active-12345"
	activeConv := filepath.Join(brainDir, activeCascadeID, "scratch")
	os.MkdirAll(activeConv, 0700)
	activeFile := filepath.Join(activeConv, "important.txt")
	os.WriteFile(activeFile, []byte("active conversation data"), 0600)
	os.Chtimes(activeFile, oldTime, oldTime) // even if old, shield protects it!

	// Test Inspector
	inspector := NewInspector(brainDir, convDir)
	bd, err := inspector.ScanBreakdown(3.0)
	if err != nil {
		t.Fatalf("ScanBreakdown error: %v", err)
	}

	if bd.BrainTotalBytes <= 0 {
		t.Fatalf("expected positive BrainTotalBytes, got %d", bd.BrainTotalBytes)
	}

	// Test Pruner
	pruner := NewPruner(brainDir)
	opts := PruneOptions{
		MinAgeDays:      3.0,
		PruneScratch:    true,
		DryRun:          false,
		ActiveCascadeID: activeCascadeID,
	}

	res, err := pruner.Prune(opts)
	if err != nil {
		t.Fatalf("Prune error: %v", err)
	}

	if res.FilesDeleted != 1 {
		t.Errorf("expected 1 file deleted, got %d", res.FilesDeleted)
	}
	if res.SkippedShield != 1 {
		t.Errorf("expected 1 file shielded, got %d", res.SkippedShield)
	}

	// Verify stale file was removed
	if _, err := os.Stat(staleFile); !os.IsNotExist(err) {
		t.Errorf("expected staleFile to be deleted")
	}

	// Verify shielded file is STILL INTACT
	if _, err := os.Stat(activeFile); err != nil {
		t.Errorf("expected activeFile to be preserved by conversation shield: %v", err)
	}
}

func TestCachePrunerSizeLimitAndUnlimitedAge(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swiss_test_cache_limit_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	brainDir := filepath.Join(tmpDir, "brain")
	convDir := filepath.Join(tmpDir, "conversations")

	scratchDir := filepath.Join(brainDir, "conv-1", "scratch")
	if err := os.MkdirAll(scratchDir, 0700); err != nil {
		t.Fatal(err)
	}

	// Create two 2KB files: one 2 days old, one 1 hour old
	olderFile := filepath.Join(scratchDir, "older.bin")
	newerFile := filepath.Join(scratchDir, "newer.bin")
	payload := make([]byte, 2048)
	if err := os.WriteFile(olderFile, payload, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newerFile, payload, 0600); err != nil {
		t.Fatal(err)
	}
	twoDaysAgo := time.Now().Add(-48 * time.Hour)
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	_ = os.Chtimes(olderFile, twoDaysAgo, twoDaysAgo)
	_ = os.Chtimes(newerFile, oneHourAgo, oneHourAgo)

	// 1. With Unlimited age (0) and 5 GB size limit, neither file should be pruned
	pruner := NewPruner(brainDir)
	res, err := pruner.Prune(PruneOptions{
		MinAgeDays: 0,
		MaxSizeGB:  5.0,
	})
	if err != nil {
		t.Fatalf("Prune error: %v", err)
	}
	if res.FilesDeleted != 0 {
		t.Fatalf("expected 0 files deleted with Unlimited age and 5GB cap, got %d", res.FilesDeleted)
	}

	// 2. With Unlimited age (0) and tiny MaxSizeGB (~2.5 KB in GB), oldest file should be pruned to fit under cap
	maxSizeGB := float64(2500) / (1024 * 1024 * 1024)
	inspector := NewInspector(brainDir, convDir)
	bd, err := inspector.ScanBreakdownWithLimit(0, 2500)
	if err != nil {
		t.Fatalf("ScanBreakdownWithLimit error: %v", err)
	}
	if bd.ReclaimableBytes != 2048 {
		t.Fatalf("expected 2048 reclaimable bytes under 2500B limit, got %d", bd.ReclaimableBytes)
	}

	res2, err := pruner.Prune(PruneOptions{
		MinAgeDays: 0,
		MaxSizeGB:  maxSizeGB,
	})
	if err != nil {
		t.Fatalf("Prune with size limit error: %v", err)
	}
	if res2.FilesDeleted != 1 || res2.DeletedFiles != 1 {
		t.Fatalf("expected 1 file deleted to satisfy size cap, got %d", res2.FilesDeleted)
	}
	if _, err := os.Stat(olderFile); !os.IsNotExist(err) {
		t.Errorf("expected olderFile to be pruned first by size limit")
	}
	if _, err := os.Stat(newerFile); err != nil {
		t.Errorf("expected newerFile to be preserved once under size limit: %v", err)
	}
}

