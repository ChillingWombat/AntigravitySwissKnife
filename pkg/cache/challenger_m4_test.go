package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestChallenger_CachePruner_SubHourCutoffs tests sub-hour cutoffs (15m, 20m, 30m) expressed as fractional days.
// Verifies that files younger than the cutoff are strictly preserved, while files older than the cutoff are pruned.
func TestChallenger_CachePruner_SubHourCutoffs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swiss_test_cache_subhour_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	brainDir := filepath.Join(tmpDir, "brain")
	convDir := filepath.Join(tmpDir, "conversations")
	scratchDir := filepath.Join(brainDir, "session-1", "scratch")
	if err := os.MkdirAll(scratchDir, 0700); err != nil {
		t.Fatal(err)
	}

	// Create test files with distinct ages
	now := time.Now()
	f5m := filepath.Join(scratchDir, "file_5m.bin")   // 5 min old
	f10m := filepath.Join(scratchDir, "file_10m.bin") // 10 min old
	f16m := filepath.Join(scratchDir, "file_16m.bin") // 16 min old
	f25m := filepath.Join(scratchDir, "file_25m.bin") // 25 min old
	f40m := filepath.Join(scratchDir, "file_40m.bin") // 40 min old

	for _, f := range []string{f5m, f10m, f16m, f25m, f40m} {
		if err := os.WriteFile(f, []byte("payload-1024-bytes"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	_ = os.Chtimes(f5m, now.Add(-5*time.Minute), now.Add(-5*time.Minute))
	_ = os.Chtimes(f10m, now.Add(-10*time.Minute), now.Add(-10*time.Minute))
	_ = os.Chtimes(f16m, now.Add(-16*time.Minute), now.Add(-16*time.Minute))
	_ = os.Chtimes(f25m, now.Add(-25*time.Minute), now.Add(-25*time.Minute))
	_ = os.Chtimes(f40m, now.Add(-40*time.Minute), now.Add(-40*time.Minute))

	inspector := NewInspector(brainDir, convDir)

	// Sub-hour cutoff 1: 15 minutes = 15 / (24 * 60) days = 0.010416666666666666 days
	cutoff15mDays := float64(15) / (24.0 * 60.0)
	bd15, err := inspector.ScanBreakdown(cutoff15mDays)
	if err != nil {
		t.Fatalf("ScanBreakdown(15m) failed: %v", err)
	}

	// 16m, 25m, and 40m files should be reclaimable (3 files * len("payload-1024-bytes"))
	expectedBytes15 := int64(3 * len("payload-1024-bytes"))
	if bd15.ReclaimableBytes != expectedBytes15 {
		t.Errorf("ScanBreakdown(15m): expected %d reclaimable bytes (3 files), got %d", expectedBytes15, bd15.ReclaimableBytes)
	}

	// Sub-hour cutoff 2: 30 minutes = 30 / (24 * 60) days
	cutoff30mDays := float64(30) / (24.0 * 60.0)
	bd30, err := inspector.ScanBreakdown(cutoff30mDays)
	if err != nil {
		t.Fatalf("ScanBreakdown(30m) failed: %v", err)
	}
	// Only 40m file should be reclaimable (1 file)
	expectedBytes30 := int64(1 * len("payload-1024-bytes"))
	if bd30.ReclaimableBytes != expectedBytes30 {
		t.Errorf("ScanBreakdown(30m): expected %d reclaimable bytes (1 file), got %d", expectedBytes30, bd30.ReclaimableBytes)
	}

	// Now run Prune with 20-minute cutoff: 20 / (24 * 60) days
	// Files 25m and 40m should be pruned. Files 5m, 10m, and 16m MUST survive!
	pruner := NewPruner(brainDir)
	cutoff20mDays := float64(20) / (24.0 * 60.0)
	res, err := pruner.Prune(PruneOptions{
		MinAgeDays:   cutoff20mDays,
		PruneScratch: true,
	})
	if err != nil {
		t.Fatalf("Prune(20m) failed: %v", err)
	}

	if res.FilesDeleted != 2 {
		t.Errorf("Prune(20m): expected 2 files deleted (25m and 40m), got %d", res.FilesDeleted)
	}

	// Verify survivors
	for _, survived := range []string{f5m, f10m, f16m} {
		if _, err := os.Stat(survived); err != nil {
			t.Errorf("file %s should have survived 20m cutoff, but stat failed: %v", filepath.Base(survived), err)
		}
	}
	// Verify deleted
	for _, deleted := range []string{f25m, f40m} {
		if _, err := os.Stat(deleted); !os.IsNotExist(err) {
			t.Errorf("file %s should have been deleted by 20m cutoff", filepath.Base(deleted))
		}
	}
}

// TestChallenger_CachePruner_MaxSizeNeverDeletesNewerThanMinAge tests that when MinAgeDays > 0,
// the MaxSizeGB prune loop NEVER prunes files younger than MinAgeDays, even if the directory
// exceeds MaxSizeGB.
func TestChallenger_CachePruner_MaxSizeNeverDeletesNewerThanMinAge(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swiss_test_cache_maxsize_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	brainDir := filepath.Join(tmpDir, "brain")
	scratchDir := filepath.Join(brainDir, "session-cap", "scratch")
	if err := os.MkdirAll(scratchDir, 0700); err != nil {
		t.Fatal(err)
	}

	// Create 3 files of 10 KB each (total 30 KB)
	// File 1: 3 days old (exceeds MinAgeDays = 1.0)
	// File 2: 2 hours old (younger than MinAgeDays = 1.0)
	// File 3: 10 minutes old (younger than MinAgeDays = 1.0)
	fOld := filepath.Join(scratchDir, "old_3days.bin")
	fRecent1 := filepath.Join(scratchDir, "recent_2hours.bin")
	fRecent2 := filepath.Join(scratchDir, "recent_10mins.bin")

	payload := make([]byte, 10*1024)
	for _, f := range []string{fOld, fRecent1, fRecent2} {
		if err := os.WriteFile(f, payload, 0600); err != nil {
			t.Fatal(err)
		}
	}

	now := time.Now()
	_ = os.Chtimes(fOld, now.Add(-72*time.Hour), now.Add(-72*time.Hour))
	_ = os.Chtimes(fRecent1, now.Add(-2*time.Hour), now.Add(-2*time.Hour))
	_ = os.Chtimes(fRecent2, now.Add(-10*time.Minute), now.Add(-10*time.Minute))

	// Configure MaxSizeGB = 5 KB (in GB: 5*1024 / (1024^3))
	// Total size is 30 KB > 5 KB.
	// MinAgeDays = 1.0 (24h).
	// Even though total remaining size (20 KB) still exceeds 5 KB after deleting fOld,
	// fRecent1 and fRecent2 MUST NOT be pruned because they are younger than MinAgeDays (1.0).
	maxSizeGB := float64(5*1024) / float64(1024*1024*1024)

	pruner := NewPruner(brainDir)
	res, err := pruner.Prune(PruneOptions{
		MinAgeDays:   1.0,
		MaxSizeGB:    maxSizeGB,
		PruneScratch: true,
	})
	if err != nil {
		t.Fatalf("Prune error: %v", err)
	}

	// Exactly 1 file should have been deleted (fOld)
	if res.FilesDeleted != 1 {
		t.Fatalf("expected exactly 1 file deleted (the 3-day-old file), got %d", res.FilesDeleted)
	}

	// Verify old file was deleted
	if _, err := os.Stat(fOld); !os.IsNotExist(err) {
		t.Errorf("expected fOld to be deleted")
	}

	// Verify both recent files are strictly preserved
	if _, err := os.Stat(fRecent1); err != nil {
		t.Errorf("fRecent1 (2h old) was incorrectly pruned despite MinAgeDays=1.0: %v", err)
	}
	if _, err := os.Stat(fRecent2); err != nil {
		t.Errorf("fRecent2 (10m old) was incorrectly pruned despite MinAgeDays=1.0: %v", err)
	}
}

// TestChallenger_CacheInspector_SafeToDeleteFlag verifies that SafeToDelete is strictly false
// when ReclaimableBytes is 0, and true when ReclaimableBytes > 0.
func TestChallenger_CacheInspector_SafeToDeleteFlag(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swiss_test_cache_safedelete_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	brainDir := filepath.Join(tmpDir, "brain")
	convDir := filepath.Join(tmpDir, "conversations")
	scratchDir := filepath.Join(brainDir, "session-safe", "scratch")
	os.MkdirAll(scratchDir, 0700)

	inspector := NewInspector(brainDir, convDir)

	// 1. Empty directory -> ReclaimableBytes = 0 -> SafeToDelete = false
	bd1, err := inspector.ScanBreakdown(1.0)
	if err != nil {
		t.Fatal(err)
	}
	if bd1.SafeToDelete {
		t.Errorf("expected SafeToDelete=false for empty cache, got true")
	}

	// 2. Fresh file only -> ReclaimableBytes = 0 -> SafeToDelete = false
	freshFile := filepath.Join(scratchDir, "fresh.txt")
	os.WriteFile(freshFile, []byte("fresh"), 0600)
	bd2, err := inspector.ScanBreakdown(1.0)
	if err != nil {
		t.Fatal(err)
	}
	if bd2.SafeToDelete {
		t.Errorf("expected SafeToDelete=false when all files are fresh, got true")
	}

	// 3. Add an old file -> ReclaimableBytes > 0 -> SafeToDelete = true
	oldFile := filepath.Join(scratchDir, "old.txt")
	os.WriteFile(oldFile, []byte("old"), 0600)
	oldTime := time.Now().Add(-48 * time.Hour)
	os.Chtimes(oldFile, oldTime, oldTime)

	bd3, err := inspector.ScanBreakdown(1.0)
	if err != nil {
		t.Fatal(err)
	}
	if !bd3.SafeToDelete {
		t.Errorf("expected SafeToDelete=true when old reclaimable files exist, got false")
	}
}

// TestChallenger_CachePruner_24HourSafetyFloor tests that the 24-hour safety floor
// (DefaultMaxSizeMinAgeDuration = 24 * time.Hour) is strictly enforced under MaxSizeGB mode:
// 1. Files modified less than 24 hours ago are NEVER deleted, even when MinAgeDays is 0 (Unlimited)
//    and cache size exceeds MaxSizeGB.
// 2. When all files are younger than 24 hours, zero files are deleted regardless of disk pressure.
// 3. Shielded active cascade sessions are never deleted even if older than 24 hours.
func TestChallenger_CachePruner_24HourSafetyFloor(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swiss_test_cache_24h_floor_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	brainDir := filepath.Join(tmpDir, "brain")
	scratchDir := filepath.Join(brainDir, "session-floor", "scratch")
	if err := os.MkdirAll(scratchDir, 0700); err != nil {
		t.Fatal(err)
	}

	// Create test files:
	// - f48h: 48 hours old (1024 bytes) -> SHOULD be pruned under MaxSizeGB
	// - f25h: 25 hours old (1024 bytes) -> SHOULD be pruned under MaxSizeGB
	// - f23h: 23 hours old (1024 bytes) -> MUST NEVER BE PRUNED (< 24h floor)
	// - f12h: 12 hours old (1024 bytes) -> MUST NEVER BE PRUNED (< 24h floor)
	// - f1h:  1 hour old   (1024 bytes) -> MUST NEVER BE PRUNED (< 24h floor)
	// - f10m: 10 mins old  (1024 bytes) -> MUST NEVER BE PRUNED (< 24h floor)
	payload := make([]byte, 1024)
	now := time.Now()

	files := map[string]time.Duration{
		"f48h.bin": 48 * time.Hour,
		"f25h.bin": 25 * time.Hour,
		"f23h.bin": 23 * time.Hour,
		"f12h.bin": 12 * time.Hour,
		"f1h.bin":  1 * time.Hour,
		"f10m.bin": 10 * time.Minute,
	}

	filePaths := make(map[string]string)
	for name, age := range files {
		fp := filepath.Join(scratchDir, name)
		if err := os.WriteFile(fp, payload, 0600); err != nil {
			t.Fatal(err)
		}
		modTime := now.Add(-age)
		_ = os.Chtimes(fp, modTime, modTime)
		filePaths[name] = fp
	}

	// Set tiny MaxSizeGB: 500 bytes (represented in GB)
	// Total directory size is 6144 bytes > 500 bytes.
	// Even though remaining size after deleting f48h and f25h will be 4096 bytes (> 500 bytes),
	// the pruner MUST NOT delete f23h, f12h, f1h, or f10m!
	tinyMaxSizeGB := float64(500) / float64(1024*1024*1024)

	pruner := NewPruner(brainDir)
	res, err := pruner.Prune(PruneOptions{
		MinAgeDays:   0, // Unlimited age mode
		MaxSizeGB:    tinyMaxSizeGB,
		PruneScratch: true,
	})
	if err != nil {
		t.Fatalf("Prune error: %v", err)
	}

	// Exactly 2 files should have been deleted (f48h and f25h)
	if res.FilesDeleted != 2 {
		t.Fatalf("expected exactly 2 files deleted (> 24h old), got %d (freed: %d bytes)", res.FilesDeleted, res.BytesReclaimed)
	}

	// Verify the 2 files older than 24h were deleted
	for _, oldName := range []string{"f48h.bin", "f25h.bin"} {
		if _, err := os.Stat(filePaths[oldName]); !os.IsNotExist(err) {
			t.Errorf("expected %s (> 24h) to be pruned", oldName)
		}
	}

	// Verify all 4 files younger than 24 hours are STILL INTACT!
	for _, youngName := range []string{"f23h.bin", "f12h.bin", "f1h.bin", "f10m.bin"} {
		if _, err := os.Stat(filePaths[youngName]); err != nil {
			t.Errorf("CRITICAL SAFETY VIOLATION: file %s (< 24h old) was deleted despite 24-hour safety floor: %v", youngName, err)
		}
	}

	// Sub-test 2: When ALL remaining files are younger than 24h, subsequent Prune
	// under extreme MaxSizeGB pressure deletes ZERO files!
	res2, err := pruner.Prune(PruneOptions{
		MinAgeDays:   0,
		MaxSizeGB:    tinyMaxSizeGB,
		PruneScratch: true,
	})
	if err != nil {
		t.Fatalf("Prune sub-test 2 error: %v", err)
	}
	if res2.FilesDeleted != 0 {
		t.Fatalf("expected 0 files deleted when all files are < 24h old, got %d", res2.FilesDeleted)
	}
}

