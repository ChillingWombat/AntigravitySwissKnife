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
