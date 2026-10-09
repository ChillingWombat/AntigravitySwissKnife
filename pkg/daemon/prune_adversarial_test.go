package daemon

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/cache"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/ipc"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/webgui"
)

// TestAdversarial_ConfigEdgeCases stress tests LoadConfig for 0.0 persistence, negative clamping, and missing keys.
func TestAdversarial_ConfigEdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)

	// Scenario 1: User explicitly saves 0.0 to disk
	rawZero := map[string]interface{}{
		"auto_prune_enabled":     true,
		"auto_prune_max_age_days": 0.0,
		"auto_prune_max_size_gb":  0.0,
	}
	data, err := json.Marshal(rawZero)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	cfgPath := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(cfgPath, data, 0600); err != nil {
		t.Fatalf("write error: %v", err)
	}

	cfg, err := core.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.AutoPruneMaxSizeGB != 0.0 {
		t.Errorf("Scenario 1 failed: expected AutoPruneMaxSizeGB 0.0, got %f (reverted to non-zero!)", cfg.AutoPruneMaxSizeGB)
	}
	if cfg.AutoPruneMaxAgeDays != 0.0 {
		t.Errorf("Scenario 1 failed: expected AutoPruneMaxAgeDays 0.0, got %f", cfg.AutoPruneMaxAgeDays)
	}

	// Scenario 2: Negative numbers saved to disk (-1.0, -5.0, -999.0)
	rawNegative := map[string]interface{}{
		"auto_prune_max_age_days": -5.0,
		"auto_prune_max_size_gb":  -1.0,
	}
	dataNeg, _ := json.Marshal(rawNegative)
	_ = os.WriteFile(cfgPath, dataNeg, 0600)

	cfgNeg, err := core.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfgNeg.AutoPruneMaxSizeGB != 0.0 {
		t.Errorf("Scenario 2 failed: expected clamped AutoPruneMaxSizeGB 0.0, got %f", cfgNeg.AutoPruneMaxSizeGB)
	}
	if cfgNeg.AutoPruneMaxAgeDays != 0.0 {
		t.Errorf("Scenario 2 failed: expected clamped AutoPruneMaxAgeDays 0.0, got %f", cfgNeg.AutoPruneMaxAgeDays)
	}

	// Scenario 3: Keys omitted from config.json -> must default to 0.0 (Unlimited)
	_ = os.WriteFile(cfgPath, []byte(`{"storage_mode":"system_default"}`), 0600)
	cfgOmitted, err := core.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfgOmitted.AutoPruneMaxSizeGB != 0.0 {
		t.Errorf("Scenario 3 failed: expected default 0.0 for omitted AutoPruneMaxSizeGB, got %f", cfgOmitted.AutoPruneMaxSizeGB)
	}
	if cfgOmitted.AutoPruneMaxAgeDays != 0.0 {
		t.Errorf("Scenario 3 failed: expected default 0.0 for omitted AutoPruneMaxAgeDays, got %f", cfgOmitted.AutoPruneMaxAgeDays)
	}
}

// TestAdversarial_PruneZeroLimits_Safety checks that pruning with max_size_gb=0 and min_age_days=0 deletes 0 files.
func TestAdversarial_PruneZeroLimits_Safety(t *testing.T) {
	brainDir := t.TempDir()

	// Populate brain directory with active and stale files
	activeID := "cascade-active-shielded"
	activeDir := filepath.Join(brainDir, activeID, "scratch")
	_ = os.MkdirAll(activeDir, 0700)
	activeFile := filepath.Join(activeDir, "important.json")
	_ = os.WriteFile(activeFile, []byte("vital conversation data"), 0600)

	staleDir := filepath.Join(brainDir, "conv-stale", "scratch")
	_ = os.MkdirAll(staleDir, 0700)
	staleFile := filepath.Join(staleDir, "scratchpad.txt")
	_ = os.WriteFile(staleFile, []byte("scratchpad data"), 0600)

	stepsDir := filepath.Join(brainDir, "conv-stale", "steps")
	_ = os.MkdirAll(stepsDir, 0700)
	stepsFile := filepath.Join(stepsDir, "step1.log")
	_ = os.WriteFile(stepsFile, make([]byte, 1024*1024), 0600) // 1MB

	tasksDir := filepath.Join(brainDir, "conv-stale", "tasks")
	_ = os.MkdirAll(tasksDir, 0700)
	tasksFile := filepath.Join(tasksDir, "task.log")
	_ = os.WriteFile(tasksFile, make([]byte, 1024*1024), 0600) // 1MB

	// Mark files as 90 days old
	oldTime := time.Now().Add(-90 * 24 * time.Hour)
	_ = os.Chtimes(activeFile, oldTime, oldTime)
	_ = os.Chtimes(staleFile, oldTime, oldTime)
	_ = os.Chtimes(stepsFile, oldTime, oldTime)
	_ = os.Chtimes(tasksFile, oldTime, oldTime)

	pruner := cache.NewPruner(brainDir)

	// Execute with 0 age and 0 size (Unlimited)
	opts := cache.PruneOptions{
		MinAgeDays:      0.0,
		MaxSizeGB:       0.0,
		PruneScratch:    true,
		PruneSteps:      true,
		PruneTasks:      true,
		DryRun:          false,
		ActiveCascadeID: activeID,
	}

	res, err := pruner.Prune(opts)
	if err != nil {
		t.Fatalf("Prune with 0 limits failed: %v", err)
	}

	if res.FilesDeleted != 0 {
		t.Fatalf("CRITICAL BUG: Expected 0 files deleted when MinAgeDays=0 and MaxSizeGB=0, but deleted %d files!", res.FilesDeleted)
	}
	if res.BytesReclaimed != 0 {
		t.Errorf("expected 0 bytes reclaimed, got %d", res.BytesReclaimed)
	}

	// Verify all files remain on disk
	for _, f := range []string{activeFile, staleFile, stepsFile, tasksFile} {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Fatalf("CRITICAL BUG: File %s was deleted unexpectedly under Unlimited 0 limits!", f)
		}
	}
}

// TestAdversarial_BackgroundTickerBehavior verifies ticker routine respects 0.0 values without overwriting or deleting.
func TestAdversarial_BackgroundTickerBehavior(t *testing.T) {
	brainDir := t.TempDir()
	cfg := core.DefaultConfig()
	cfg.AutoPruneEnabled = true
	cfg.AutoPruneMaxAgeDays = 0.0
	cfg.AutoPruneMaxSizeGB = 0.0

	scratchDir := filepath.Join(brainDir, "session-1", "scratch")
	_ = os.MkdirAll(scratchDir, 0700)
	scratchFile := filepath.Join(scratchDir, "cached_state.bin")
	_ = os.WriteFile(scratchFile, []byte("active cache content"), 0600)

	pruner := cache.NewPruner(brainDir)

	// Emulate the exact code block executed in daemon.go lines 1454-1462 when ticker fires:
	autoPruneOn := cfg.AutoPruneEnabled
	pruneAge := cfg.AutoPruneMaxAgeDays
	pruneMaxGB := cfg.AutoPruneMaxSizeGB

	if !autoPruneOn {
		t.Fatalf("expected autoPruneOn to be true")
	}
	if pruneAge != 0.0 || pruneMaxGB != 0.0 {
		t.Fatalf("expected 0.0 config values, got age=%f maxGB=%f", pruneAge, pruneMaxGB)
	}

	res, err := pruner.Prune(cache.PruneOptions{
		MinAgeDays:   pruneAge,
		MaxSizeGB:    pruneMaxGB,
		PruneScratch: true,
		PruneSteps:   true,
		PruneTasks:   true,
	})
	if err != nil {
		t.Fatalf("ticker prune execution failed: %v", err)
	}
	if res.FilesDeleted != 0 {
		t.Errorf("ticker prune deleted %d files under 0.0 Unlimited settings!", res.FilesDeleted)
	}
	if _, err := os.Stat(scratchFile); os.IsNotExist(err) {
		t.Errorf("scratchFile was deleted by ticker prune under 0.0 Unlimited settings!")
	}
}

// TestAdversarial_DaemonIPC_CacheMethods tests JSON-RPC IPC methods on a live daemon.
func TestAdversarial_DaemonIPC_CacheMethods(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tmpDir)
	t.Setenv("HOME", tmpDir)
	t.Setenv("ANTIGRAVITY_TEST_MODE", "1")
	t.Setenv("ANTIGRAVITY_TEST_DRY_RUN", "1")

	sockPath := filepath.Join(tmpDir, "daemon_test.sock")
	cfg := core.DefaultConfig()

	d, err := NewDaemon(cfg, sockPath)
	if err != nil {
		t.Fatalf("NewDaemon error: %v", err)
	}
	if err := d.Start(); err != nil {
		t.Fatalf("d.Start error: %v", err)
	}
	defer d.Stop()

	client := ipc.NewClient(sockPath)

	// 1. swiss.getCacheConfig default check
	var getConf map[string]interface{}
	if err := client.Call("swiss.getCacheConfig", nil, &getConf); err != nil {
		t.Fatalf("swiss.getCacheConfig failed: %v", err)
	}
	if size, ok := getConf["max_size_gb"].(float64); !ok || size != 0.0 {
		t.Errorf("expected swiss.getCacheConfig max_size_gb 0.0, got %v", getConf["max_size_gb"])
	}
	if age, ok := getConf["prune_days"].(float64); !ok || age != 0.0 {
		t.Errorf("expected swiss.getCacheConfig prune_days 0.0, got %v", getConf["prune_days"])
	}

	// 2. swiss.setCacheConfig with 0.0
	zeroVal := 0.0
	enabled := true
	setReq := map[string]interface{}{
		"auto_prune_enabled": &enabled,
		"prune_days":        &zeroVal,
		"max_size_gb":       &zeroVal,
	}
	var setResp map[string]interface{}
	if err := client.Call("swiss.setCacheConfig", setReq, &setResp); err != nil {
		t.Fatalf("swiss.setCacheConfig failed: %v", err)
	}
	if size, ok := setResp["max_size_gb"].(float64); !ok || size != 0.0 {
		t.Errorf("expected setResp max_size_gb 0.0, got %v", setResp["max_size_gb"])
	}

	// 3. swiss.setCacheConfig with negative numbers (must clamp to 0.0)
	negVal := -3.5
	negAge := -10.0
	negSetReq := map[string]interface{}{
		"prune_days":  &negAge,
		"max_size_gb": &negVal,
	}
	var negSetResp map[string]interface{}
	if err := client.Call("swiss.setCacheConfig", negSetReq, &negSetResp); err != nil {
		t.Fatalf("swiss.setCacheConfig with negative failed: %v", err)
	}
	if size, ok := negSetResp["max_size_gb"].(float64); !ok || size != 0.0 {
		t.Errorf("expected clamped max_size_gb 0.0, got %v", negSetResp["max_size_gb"])
	}

	// 4. swiss.pruneCache with 0.0 limits
	pruneOpts := cache.PruneOptions{
		MinAgeDays: 0.0,
		MaxSizeGB:  0.0,
		DryRun:     false,
	}
	var pruneRes cache.PruneResult
	if err := client.Call("swiss.pruneCache", pruneOpts, &pruneRes); err != nil {
		t.Fatalf("swiss.pruneCache failed: %v", err)
	}
	if pruneRes.FilesDeleted != 0 {
		t.Errorf("expected swiss.pruneCache 0 files deleted under 0.0 limits, got %d", pruneRes.FilesDeleted)
	}
}

// TestAdversarial_WebGUIEndpoints_EdgeCases tests webgui endpoints for 0.0, negative numbers, and empty payloads.
func TestAdversarial_WebGUIEndpoints_EdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", tempDir)

	srv := webgui.NewServer("127.0.0.1:0", filepath.Join(tempDir, "test.sock"))
	if err := srv.Start(); err != nil {
		t.Fatalf("srv.Start failed: %v", err)
	}
	defer srv.Stop()
	baseURL := "http://" + srv.Addr()

	// 1. POST /api/cache/config with negative values
	negConfigPayload := map[string]interface{}{
		"auto_prune_enabled": true,
		"prune_days":         -10.0,
		"max_size_gb":        -5.0,
	}
	negBytes, _ := json.Marshal(negConfigPayload)
	resp, err := http.Post(baseURL+"/api/cache/config", "application/json", bytes.NewReader(negBytes))
	if err != nil {
		t.Fatalf("POST /api/cache/config error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var resMap map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&resMap)
	resp.Body.Close()

	if size, ok := resMap["max_size_gb"].(float64); !ok || size != 0.0 {
		t.Errorf("expected max_size_gb clamped to 0.0, got %v", resMap["max_size_gb"])
	}

	// 2. GET /api/cache/config returns 0.0
	getResp, err := http.Get(baseURL + "/api/cache/config")
	if err != nil {
		t.Fatalf("GET /api/cache/config error: %v", err)
	}
	var getMap map[string]interface{}
	_ = json.NewDecoder(getResp.Body).Decode(&getMap)
	getResp.Body.Close()
	if size, ok := getMap["max_size_gb"].(float64); !ok || size != 0.0 {
		t.Errorf("expected persisted max_size_gb 0.0, got %v", getMap["max_size_gb"])
	}

	// 3. POST /api/cache/prune with 0.0 limits
	zeroPrunePayload := map[string]interface{}{
		"min_age_days": 0.0,
		"max_size_gb":  0.0,
		"dry_run":      false,
	}
	zeroBytes, _ := json.Marshal(zeroPrunePayload)
	pruneResp, err := http.Post(baseURL+"/api/cache/prune", "application/json", bytes.NewReader(zeroBytes))
	if err != nil {
		t.Fatalf("POST /api/cache/prune error: %v", err)
	}
	if pruneResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from prune with 0.0 limits, got %d", pruneResp.StatusCode)
	}
	var pruneResult cache.PruneResult
	_ = json.NewDecoder(pruneResp.Body).Decode(&pruneResult)
	pruneResp.Body.Close()

	if pruneResult.FilesDeleted != 0 {
		t.Errorf("expected 0 files deleted on 0.0 prune, got %d", pruneResult.FilesDeleted)
	}

	// 4. POST /api/cache/prune with negative limits (should not panic or error)
	negPrunePayload := map[string]interface{}{
		"min_age_days": -5.0,
		"max_size_gb":  -1.0,
	}
	negPruneBytes, _ := json.Marshal(negPrunePayload)
	negPruneResp, err := http.Post(baseURL+"/api/cache/prune", "application/json", bytes.NewReader(negPruneBytes))
	if err != nil {
		t.Fatalf("POST /api/cache/prune with negative limits error: %v", err)
	}
	if negPruneResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from negative prune, got %d", negPruneResp.StatusCode)
	}
	negPruneResp.Body.Close()
}
