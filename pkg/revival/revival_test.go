package revival

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
	_ "modernc.org/sqlite"
)

func setupMockSummariesDB(t *testing.T, dir string) string {
	t.Helper()
	dbPath := filepath.Join(dir, "conversation_summaries.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open test sqlite: %v", err)
	}
	defer db.Close()

	schema := `
	CREATE TABLE conversation_summaries (
		conversation_id TEXT PRIMARY KEY,
		title TEXT NOT NULL DEFAULT "",
		preview TEXT NOT NULL DEFAULT "",
		step_count INTEGER NOT NULL DEFAULT 0,
		last_modified_time DATETIME NOT NULL,
		workspace_uris TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT "",
		source TEXT NOT NULL DEFAULT "",
		project_id TEXT NOT NULL DEFAULT "",
		agent_name TEXT NOT NULL DEFAULT "",
		parent_conversation_id TEXT NOT NULL DEFAULT "",
		nesting_depth INTEGER NOT NULL DEFAULT 0,
		battle_id TEXT NOT NULL DEFAULT "",
		winning_conversation_id TEXT NOT NULL DEFAULT "",
		not_fully_idle NUMERIC NOT NULL DEFAULT 0,
		killed NUMERIC NOT NULL DEFAULT 0,
		last_user_input_time DATETIME,
		last_user_input_step_index INTEGER NOT NULL DEFAULT -1,
		app_data_dir TEXT NOT NULL DEFAULT "",
		raw_summary BLOB,
		group_id TEXT NOT NULL DEFAULT ""
	);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}
	return dbPath
}

func setupMockTrajectoryDB(t *testing.T, dir string, convID string, lastStepStatus int) {
	t.Helper()
	convDir := filepath.Join(dir, "conversations")
	_ = os.MkdirAll(convDir, 0755)
	trajDBPath := filepath.Join(convDir, convID+".db")
	db, err := sql.Open("sqlite", trajDBPath)
	if err != nil {
		t.Fatalf("failed to open trajectory db: %v", err)
	}
	defer db.Close()

	schema := `
	CREATE TABLE steps (
		idx INTEGER PRIMARY KEY,
		step_type INTEGER NOT NULL DEFAULT 0,
		status INTEGER NOT NULL DEFAULT 0
	);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create steps schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO steps (idx, step_type, status) VALUES (0, 14, 3), (1, 132, ?);`, lastStepStatus); err != nil {
		t.Fatalf("failed to insert step: %v", err)
	}
}

func TestDetector_TopLevelConversation(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := setupMockSummariesDB(t, tmpDir)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	now := time.Now().Format(time.RFC3339)
	_, err = db.Exec(`
		INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
		VALUES ('conv-root-123', 'My Root Task', '["file:///workspace"]', '', 0, 1, 'CASCADE_RUN_STATUS_RUNNING', ?);
	`, now)
	if err != nil {
		t.Fatalf("failed to insert mock conversation: %v", err)
	}

	detector := &SessionDetector{
		BaseDir: tmpDir,
	}

	info, err := detector.Detect("desktop")
	if err != nil {
		t.Fatalf("expected active conversation, got error: %v", err)
	}

	if info.ConversationID != "conv-root-123" {
		t.Errorf("expected conv-root-123, got %s", info.ConversationID)
	}
	if !info.IsTopLevel {
		t.Errorf("expected is_top_level = true")
	}
	if !info.NeedsRevival {
		t.Errorf("expected needs_revival = true due to running status")
	}
	if info.ActiveSubagentID != "" {
		t.Errorf("expected empty activeSubagentID, got %s", info.ActiveSubagentID)
	}
}

func TestDetector_SubagentHierarchyWalking(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := setupMockSummariesDB(t, tmpDir)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	t1 := time.Now().Add(-10 * time.Minute).Format(time.RFC3339)
	t2 := time.Now().Add(-5 * time.Minute).Format(time.RFC3339)
	t3 := time.Now().Format(time.RFC3339)

	// Level 0: Root conversation (idle)
	_, _ = db.Exec(`
		INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
		VALUES ('root-abc-001', 'User Chat', '["file:///workspace"]', '', 0, 0, 'CASCADE_RUN_STATUS_IDLE', ?);
	`, t1)

	// Level 1: Orchestrator subagent
	_, _ = db.Exec(`
		INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
		VALUES ('sub-orch-002', 'Orchestrator', '["file:///workspace"]', 'root-abc-001', 1, 1, 'CASCADE_RUN_STATUS_IDLE', ?);
	`, t2)

	// Level 2: Worker subagent (most recently modified and actively running)
	_, _ = db.Exec(`
		INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
		VALUES ('sub-worker-003', 'Worker Implementer', '["file:///workspace"]', 'sub-orch-002', 2, 1, 'CASCADE_RUN_STATUS_RUNNING', ?);
	`, t3)

	detector := &SessionDetector{
		BaseDir: tmpDir,
	}

	info, err := detector.Detect("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should walk all the way to root-abc-001
	if info.ConversationID != "root-abc-001" {
		t.Errorf("expected root conversation root-abc-001, got %s", info.ConversationID)
	}
	// Active subagent recorded
	if info.ActiveSubagentID != "sub-worker-003" {
		t.Errorf("expected active subagent sub-worker-003, got %s", info.ActiveSubagentID)
	}
	if !info.NeedsRevival {
		t.Errorf("expected needs_revival = true due to subagent running status")
	}
	if !info.IsTopLevel {
		t.Errorf("expected root to be marked top-level")
	}
}

func TestDetector_TrajectoryStepInterruption(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := setupMockSummariesDB(t, tmpDir)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	now := time.Now().Format(time.RFC3339)
	// Conversation says idle in summary table
	_, _ = db.Exec(`
		INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
		VALUES ('conv-traj-test', 'Traj Test', '[]', '', 0, 0, 'CASCADE_RUN_STATUS_IDLE', ?);
	`, now)

	// But trajectory DB steps table has status == 2 (RUNNING)
	setupMockTrajectoryDB(t, tmpDir, "conv-traj-test", 2)

	detector := &SessionDetector{
		BaseDir: tmpDir,
	}

	info, err := detector.Detect("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.NeedsRevival {
		t.Errorf("expected needs_revival = true because trajectory last step was status 2 (RUNNING)")
	}
}

func TestDetector_AppStorageFallback(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "app_storage.json")

	storageContent := map[string]interface{}{
		"antigravity-multi-conversation-layout-v3-layout-convo-999": `{"rootNode":{"type":"pane"}}`,
		"antigravity_swiss_last_conversation_path":                  "/c/layout-convo-999",
	}
	data, _ := json.Marshal(storageContent)
	_ = os.WriteFile(storagePath, data, 0644)

	detector := &SessionDetector{
		BaseDir:        filepath.Join(tmpDir, "non_existent_db_dir"),
		AppStoragePath: storagePath,
	}

	info, err := detector.Detect("desktop")
	if err != nil {
		t.Fatalf("fallback to app_storage.json failed: %v", err)
	}

	if info.ConversationID != "layout-convo-999" {
		t.Errorf("expected layout-convo-999, got %s", info.ConversationID)
	}
	if info.TargetPath != "/c/layout-convo-999" {
		t.Errorf("expected target path /c/layout-convo-999, got %s", info.TargetPath)
	}
}

func TestStore_SaveLoadClear(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	now := time.Now()
	intent := &RevivalIntent{
		IntentID:           "test-intent-1",
		TargetApp:          "desktop",
		RootConversationID: "conv-101",
		ActiveSubagentID:   "sub-202",
		TriggerPrompt:      "Please continue",
		CreatedAt:          now,
		Status:             IntentStatusPending,
		TTLSeconds:         60,
	}

	if err := store.SaveIntent(intent); err != nil {
		t.Fatalf("failed to save intent: %v", err)
	}

	loaded, err := store.LoadIntent()
	if err != nil {
		t.Fatalf("failed to load intent: %v", err)
	}
	if loaded == nil {
		t.Fatalf("expected non-nil loaded intent")
	}
	if loaded.RootConversationID != "conv-101" {
		t.Errorf("expected conv-101, got %s", loaded.RootConversationID)
	}
	if loaded.CascadeID != "conv-101" {
		t.Errorf("expected cascade_id compatibility field to match root, got %s", loaded.CascadeID)
	}
	if loaded.TriggerPrompt != "Please continue" {
		t.Errorf("expected prompt 'Please continue', got %s", loaded.TriggerPrompt)
	}

	// Update status
	if err := store.UpdateStatus("test-intent-1", IntentStatusRevived); err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	updated, _ := store.LoadIntent()
	if updated.Status != IntentStatusRevived || !updated.Resumed {
		t.Errorf("expected status 'revived' and resumed=true, got status=%s, resumed=%v", updated.Status, updated.Resumed)
	}

	// Test ClearIntent
	if err := store.ClearIntent(); err != nil {
		t.Fatalf("failed to clear intent: %v", err)
	}

	cleared, err := store.LoadIntent()
	if err != nil {
		t.Fatalf("error loading cleared intent: %v", err)
	}
	if cleared != nil {
		t.Errorf("expected nil after clear, got %v", cleared)
	}
}

func TestStore_TTLExpiration(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	past := time.Now().Add(-100 * time.Second)
	intent := &RevivalIntent{
		IntentID:           "expired-intent",
		RootConversationID: "conv-old",
		CreatedAt:          past,
		Status:             IntentStatusPending,
		TTLSeconds:         30, // 30s TTL, created 100s ago
	}

	_ = store.SaveIntent(intent)

	loaded, err := store.LoadIntent()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded != nil {
		t.Errorf("expected expired intent to return nil, got %v", loaded)
	}
}

func TestCDPTrigger_MockExecution(t *testing.T) {
	cdp := NewCDPTrigger(9222)

	// Mock target finder
	cdp.TargetFinder = func(port int) (int, []gui.DevToolsTarget, error) {
		return 9222, []gui.DevToolsTarget{
			{
				ID:                   "page-1",
				URL:                  "https://127.0.0.1:41234/c/test-convo-id",
				WebSocketDebuggerURL: "ws://127.0.0.1:9222/devtools/page/1",
			},
		}, nil
	}

	// Mock script executor returning success
	calledScript := ""
	cdp.ScriptExecutor = func(wsURL, expression string) (map[string]interface{}, error) {
		calledScript = expression
		return map[string]interface{}{
			"success": true,
			"status":  "clicked_send_button",
		}, nil
	}

	err := cdp.TriggerDesktopContinuation("test-convo-id", "Continue work", 2*time.Second)
	if err != nil {
		t.Fatalf("expected successful continuation trigger, got error: %v", err)
	}
	if calledScript == "" {
		t.Errorf("expected script executor to be invoked")
	}
}

func TestCDPTrigger_Timeout(t *testing.T) {
	cdp := NewCDPTrigger(9222)

	// Target finder returns no targets
	cdp.TargetFinder = func(port int) (int, []gui.DevToolsTarget, error) {
		return 9222, []gui.DevToolsTarget{}, nil
	}

	err := cdp.TriggerDesktopContinuation("test-convo-id", "Continue work", 200*time.Millisecond)
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
}

func TestEngine_Lifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := setupMockSummariesDB(t, tmpDir)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	now := time.Now().Format(time.RFC3339)
	_, _ = db.Exec(`
		INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
		VALUES ('conv-engine-1', 'Engine Test', '[]', '', 0, 1, 'CASCADE_RUN_STATUS_RUNNING', ?);
	`, now)

	engine := NewEngine(tmpDir, 9222)
	engine.Detector.BaseDir = tmpDir

	// Mock CDP execution to succeed immediately
	engine.CDPTrigger.TargetFinder = func(port int) (int, []gui.DevToolsTarget, error) {
		return 9222, []gui.DevToolsTarget{
			{
				URL:                  "https://127.0.0.1:41234/c/conv-engine-1",
				WebSocketDebuggerURL: "ws://127.0.0.1:9222/devtools/page/1",
			},
		}, nil
	}
	engine.CDPTrigger.ScriptExecutor = func(wsURL, expression string) (map[string]interface{}, error) {
		return map[string]interface{}{"success": true}, nil
	}

	// 1. Capture Pre-Switch State
	intent, err := engine.CapturePreSwitchState("desktop")
	if err != nil {
		t.Fatalf("CapturePreSwitchState failed: %v", err)
	}
	if intent == nil || intent.RootConversationID != "conv-engine-1" {
		t.Fatalf("unexpected captured intent: %v", intent)
	}

	// 2. Query status: should see pending intent
	status, err := engine.GetRevivalStatus()
	if err != nil {
		t.Fatalf("GetRevivalStatus failed: %v", err)
	}
	if status.PendingIntent == nil || status.PendingIntent.RootConversationID != "conv-engine-1" {
		t.Errorf("expected pending intent in status, got %v", status.PendingIntent)
	}

	// 3. Execute Post-Relaunch Revival
	if err := engine.ExecutePostRelaunchRevival(intent); err != nil {
		t.Fatalf("ExecutePostRelaunchRevival failed: %v", err)
	}

	// 4. Query status again: should indicate success and lastRevivedAt
	statusAfter, _ := engine.GetRevivalStatus()
	if !statusAfter.Success {
		t.Errorf("expected success = true after revival execution")
	}
	if statusAfter.LastRevivedAt == nil {
		t.Errorf("expected non-nil lastRevivedAt")
	}

	// 5. Test AcknowledgeContinuation
	if err := engine.AcknowledgeContinuation("conv-engine-1"); err != nil {
		t.Fatalf("AcknowledgeContinuation failed: %v", err)
	}
}

func TestEngine_CapturePreSwitchState_IdleSessionProducesEmptyTriggerPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := setupMockSummariesDB(t, tmpDir)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	now := time.Now().Format(time.RFC3339)
	// Conversation is completely idle: not_fully_idle=0, status=CASCADE_RUN_STATUS_IDLE
	_, _ = db.Exec(`
		INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
		VALUES ('conv-idle-session', 'Finished Idle Task', '[]', '', 0, 0, 'CASCADE_RUN_STATUS_IDLE', ?);
	`, now)

	engine := NewEngine(tmpDir, 9222)
	engine.Detector.BaseDir = tmpDir

	intent, err := engine.CapturePreSwitchState("desktop")
	if err != nil {
		t.Fatalf("CapturePreSwitchState failed: %v", err)
	}
	if intent == nil {
		t.Fatalf("expected intent to be returned for layout preservation")
	}
	if intent.RootConversationID != "conv-idle-session" {
		t.Errorf("expected RootConversationID 'conv-idle-session', got %s", intent.RootConversationID)
	}
	// Defect 1 check: TriggerPrompt MUST be empty for idle session
	if intent.TriggerPrompt != "" {
		t.Errorf("expected TriggerPrompt to be empty when NeedsRevival == false, got %q", intent.TriggerPrompt)
	}
	if intent.Prompt != "" {
		t.Errorf("expected Prompt to be empty when NeedsRevival == false, got %q", intent.Prompt)
	}
}

func TestEngine_ExecuteDesktopRevival_SkipsPromptInjectionWhenEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	engine := NewEngine(tmpDir, 9222)

	cdpCalled := false
	engine.CDPTrigger.TargetFinder = func(port int) (int, []gui.DevToolsTarget, error) {
		cdpCalled = true
		return 9222, []gui.DevToolsTarget{
			{
				URL:                  "https://127.0.0.1:41234/c/conv-idle-skip",
				WebSocketDebuggerURL: "ws://127.0.0.1:9222/devtools/page/1",
			},
		}, nil
	}
	engine.CDPTrigger.ScriptExecutor = func(wsURL, expression string) (map[string]interface{}, error) {
		cdpCalled = true
		return map[string]interface{}{"success": true}, nil
	}

	intent := &RevivalIntent{
		IntentID:           "test-skip-prompt",
		TargetApp:          "desktop",
		RootConversationID: "conv-idle-skip",
		TriggerPrompt:      "", // Empty trigger prompt
		Prompt:             "",
		Status:             IntentStatusPending,
	}

	// Should not trigger prompt injection and should return nil error
	err := engine.executeDesktopRevival(intent)
	if err != nil {
		t.Fatalf("executeDesktopRevival failed: %v", err)
	}
	if cdpCalled {
		t.Errorf("expected CDPTrigger.TriggerDesktopContinuation to be SKIPPED when TriggerPrompt is empty")
	}
}

func TestStore_ConcurrentSavesWithoutTmpCollision(t *testing.T) {
	tmpDir := t.TempDir()

	const numWorkers = 20
	const iterations = 50
	var wg sync.WaitGroup
	var errCount int64

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			store := NewStore(tmpDir)
			for j := 0; j < iterations; j++ {
				intent := &RevivalIntent{
					IntentID:           fmt.Sprintf("intent-%d-%d", workerID, j),
					RootConversationID: fmt.Sprintf("conv-%d", workerID),
					TriggerPrompt:      "Continue concurrent work",
					CreatedAt:          time.Now(),
					Status:             IntentStatusPending,
					TTLSeconds:         60,
				}
				if err := store.SaveIntent(intent); err != nil {
					atomic.AddInt64(&errCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	if errCount > 0 {
		t.Fatalf("expected 0 errors during concurrent SaveIntent, got %d collisions/errors", errCount)
	}

	// Verify the final file is valid and loadable
	finalStore := NewStore(tmpDir)
	loaded, err := finalStore.LoadIntent()
	if err != nil {
		t.Fatalf("LoadIntent failed after concurrent writes: %v", err)
	}
	if loaded == nil {
		t.Fatalf("expected non-nil loaded intent after concurrent saves")
	}
}

func TestDetector_DeepAncestorTraversal_UpTo50Levels(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := setupMockSummariesDB(t, tmpDir)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	now := time.Now()
	rootID := "root-deep-conv-000"
	// Root conversation at depth 0
	_, err = db.Exec(`
		INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
		VALUES (?, 'Root Deep Conversation', '["file:///root"]', '', 0, 0, 'CASCADE_RUN_STATUS_IDLE', ?);
	`, rootID, now.Add(-60*time.Minute).Format(time.RFC3339))
	if err != nil {
		t.Fatalf("failed to insert root: %v", err)
	}

	// Insert chain of 25 nested subagents (depth 1 to 25, well above old limit of 10)
	const maxDepth = 25
	for d := 1; d <= maxDepth; d++ {
		currID := fmt.Sprintf("subagent-depth-%03d", d)
		var parentID string
		if d == 1 {
			parentID = rootID
		} else {
			parentID = fmt.Sprintf("subagent-depth-%03d", d-1)
		}

		isLeaf := (d == maxDepth)
		status := "CASCADE_RUN_STATUS_IDLE"
		idle := 0
		if isLeaf {
			status = "CASCADE_RUN_STATUS_RUNNING"
			idle = 1
		}

		modTime := now.Add(-time.Duration(maxDepth-d) * time.Minute).Format(time.RFC3339)
		_, err = db.Exec(`
			INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
			VALUES (?, ?, '["file:///sub"]', ?, ?, ?, ?, ?);
		`, currID, fmt.Sprintf("Subagent L%d", d), parentID, d, idle, status, modTime)
		if err != nil {
			t.Fatalf("failed to insert subagent at depth %d: %v", d, err)
		}
	}

	detector := &SessionDetector{
		BaseDir: tmpDir,
	}

	info, err := detector.Detect("")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	// Must successfully climb through 25 parent levels and resolve root conversation
	if info.ConversationID != rootID {
		t.Errorf("expected root conversation %s at depth %d, got %s", rootID, maxDepth, info.ConversationID)
	}
	if !info.IsTopLevel {
		t.Errorf("expected is_top_level = true for resolved root conversation")
	}
	leafID := fmt.Sprintf("subagent-depth-%03d", maxDepth)
	if info.ActiveSubagentID != leafID {
		t.Errorf("expected activeSubagentID = %s, got %s", leafID, info.ActiveSubagentID)
	}
	if !info.NeedsRevival {
		t.Errorf("expected needs_revival = true due to leaf subagent running status")
	}
}

func TestDetector_CyclicParentPointersTerminateSafely(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := setupMockSummariesDB(t, tmpDir)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	now := time.Now()
	// Create circular reference: sub-cycle-A -> sub-cycle-B -> sub-cycle-A
	_, _ = db.Exec(`
		INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
		VALUES ('sub-cycle-A', 'Cycle A', '[]', 'sub-cycle-B', 1, 1, 'CASCADE_RUN_STATUS_RUNNING', ?);
	`, now.Format(time.RFC3339))
	_, _ = db.Exec(`
		INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
		VALUES ('sub-cycle-B', 'Cycle B', '[]', 'sub-cycle-A', 2, 1, 'CASCADE_RUN_STATUS_IDLE', ?);
	`, now.Add(-time.Minute).Format(time.RFC3339))

	detector := &SessionDetector{BaseDir: tmpDir}

	// Must terminate cleanly in <50ms without hanging or infinite loop
	done := make(chan bool)
	go func() {
		info, err := detector.Detect("")
		if err != nil {
			t.Errorf("Detect returned unexpected error: %v", err)
		}
		if info == nil {
			t.Errorf("expected non-nil info")
		}
		done <- true
	}()

	select {
	case <-done:
		// Succeeded cleanly
	case <-time.After(1 * time.Second):
		t.Fatalf("cycle detection timed out; possible infinite loop")
	}
}
