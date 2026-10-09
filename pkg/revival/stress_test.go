package revival

import (
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// =========================================================================
// 1. Subagent Ancestor Tree Stress-Testing
// =========================================================================

// TestStress_AncestorTree_DeepNesting tests ancestor hierarchy traversal across various depths:
// depth 1, depth 2, depth 3, depth 4, depth 8, depth 10, depth 12, depth 15.
func TestStress_AncestorTree_DeepNesting(t *testing.T) {
	depthsToTest := []int{1, 2, 3, 4, 8, 10, 12, 15}

	for _, maxDepth := range depthsToTest {
		t.Run(fmt.Sprintf("Depth_%d", maxDepth), func(t *testing.T) {
			tmpDir := t.TempDir()
			dbPath := setupMockSummariesDB(t, tmpDir)

			db, err := sql.Open("sqlite", dbPath)
			if err != nil {
				t.Fatalf("failed to open db: %v", err)
			}
			defer db.Close()

			now := time.Now()
			// Insert root conversation (depth 0, parent "")
			rootID := "root-conv-000"
			_, err = db.Exec(`
				INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
				VALUES (?, 'Root Conversation', '["file:///root"]', '', 0, 0, 'CASCADE_RUN_STATUS_IDLE', ?);
			`, rootID, now.Add(-time.Duration(maxDepth+1)*time.Minute).Format(time.RFC3339))
			if err != nil {
				t.Fatalf("failed to insert root: %v", err)
			}

			// Insert chain of subagents from depth 1 to maxDepth
			for d := 1; d <= maxDepth; d++ {
				currID := fmt.Sprintf("subagent-depth-%03d", d)
				var parentID string
				if d == 1 {
					parentID = rootID
				} else {
					parentID = fmt.Sprintf("subagent-depth-%03d", d-1)
				}

				// The deepest subagent is the most recently modified and is running
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

			detector := &SessionDetector{BaseDir: tmpDir}
			info, err := detector.Detect("")
			if err != nil {
				t.Fatalf("Detect failed at maxDepth %d: %v", maxDepth, err)
			}

			leafID := fmt.Sprintf("subagent-depth-%03d", maxDepth)
			if info.ActiveSubagentID != leafID {
				t.Errorf("expected ActiveSubagentID=%s, got %s", leafID, info.ActiveSubagentID)
			}
			if !info.NeedsRevival {
				t.Errorf("expected NeedsRevival=true for active leaf subagent")
			}

			if maxDepth <= 50 {
				// Within the 50-step traversal loop, detector MUST reach the root conversation
				if info.ConversationID != rootID {
					t.Errorf("maxDepth=%d (<=50): expected root conversation %s, got %s", maxDepth, rootID, info.ConversationID)
				}
				if !info.IsTopLevel {
					t.Errorf("maxDepth=%d (<=50): expected IsTopLevel=true, got false", maxDepth)
				}
			} else {
				// When maxDepth > 50, the loop bounds at 50 steps.
				// It stops at ancestor at (maxDepth - 50), which has depth > 0.
				expectedReached := fmt.Sprintf("subagent-depth-%03d", maxDepth-50)
				t.Logf("maxDepth=%d (>50): step limit reached as expected. Reached ancestor %s (IsTopLevel=%v)",
					maxDepth, info.ConversationID, info.IsTopLevel)
				if info.ConversationID != expectedReached {
					t.Errorf("maxDepth=%d (>50): expected to reach ancestor %s, got %s", maxDepth, expectedReached, info.ConversationID)
				}
				if info.IsTopLevel {
					t.Errorf("maxDepth=%d (>50): intermediate ancestor should NOT be marked IsTopLevel", maxDepth)
				}
			}
		})
	}
}

// TestStress_AncestorTree_OrphanSubagent tests behavior when parent_conversation_id points to a non-existent row.
func TestStress_AncestorTree_OrphanSubagent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := setupMockSummariesDB(t, tmpDir)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	now := time.Now().Format(time.RFC3339)
	// Subagent points to "missing-parent-999" which does NOT exist in the database
	_, err = db.Exec(`
		INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
		VALUES ('orphan-sub-001', 'Orphan Subagent', '[]', 'missing-parent-999', 2, 1, 'CASCADE_RUN_STATUS_RUNNING', ?);
	`, now)
	if err != nil {
		t.Fatalf("failed to insert orphan subagent: %v", err)
	}

	detector := &SessionDetector{BaseDir: tmpDir}
	info, err := detector.Detect("")
	if err != nil {
		t.Fatalf("Detect failed unexpectedly on orphan subagent: %v", err)
	}

	if info == nil {
		t.Fatalf("expected non-nil info")
	}

	// Should not panic, should fallback gracefully to the orphaned subagent
	if info.ConversationID != "orphan-sub-001" {
		t.Errorf("expected ConversationID=orphan-sub-001, got %s", info.ConversationID)
	}
	if info.ActiveSubagentID != "orphan-sub-001" {
		t.Errorf("expected ActiveSubagentID=orphan-sub-001, got %s", info.ActiveSubagentID)
	}
	if info.IsTopLevel {
		t.Errorf("expected IsTopLevel=false for orphaned subagent with nesting_depth=2")
	}
	if !info.NeedsRevival {
		t.Errorf("expected NeedsRevival=true due to running status")
	}
}

// TestStress_AncestorTree_Cycles tests resilience against cyclic parent pointers:
// 1) Self-referential (A -> A)
// 2) Mutual recursion (A -> B -> A)
func TestStress_AncestorTree_Cycles(t *testing.T) {
	t.Run("SelfReferential", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := setupMockSummariesDB(t, tmpDir)

		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatalf("failed to open db: %v", err)
		}
		defer db.Close()

		now := time.Now().Format(time.RFC3339)
		// A points to A
		_, err = db.Exec(`
			INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
			VALUES ('cycle-self', 'Self Referencing', '[]', 'cycle-self', 1, 1, 'CASCADE_RUN_STATUS_RUNNING', ?);
		`, now)
		if err != nil {
			t.Fatalf("failed to insert self-referential row: %v", err)
		}

		detector := &SessionDetector{BaseDir: tmpDir}
		done := make(chan struct{})
		var info *ActiveSessionInfo
		var detectErr error

		go func() {
			info, detectErr = detector.Detect("")
			close(done)
		}()

		select {
		case <-done:
			if detectErr != nil {
				t.Fatalf("Detect failed: %v", detectErr)
			}
			if info.ConversationID != "cycle-self" {
				t.Errorf("expected cycle-self, got %s", info.ConversationID)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("Detect hung in infinite loop on self-referential parent pointer!")
		}
	})

	t.Run("MutualRecursion", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := setupMockSummariesDB(t, tmpDir)

		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatalf("failed to open db: %v", err)
		}
		defer db.Close()

		now := time.Now()
		// B points to A, A points to B
		_, err = db.Exec(`
			INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
			VALUES ('cycle-A', 'Cycle A', '[]', 'cycle-B', 1, 0, 'CASCADE_RUN_STATUS_IDLE', ?);
		`, now.Add(-1*time.Minute).Format(time.RFC3339))
		if err != nil {
			t.Fatalf("failed to insert cycle A: %v", err)
		}

		_, err = db.Exec(`
			INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
			VALUES ('cycle-B', 'Cycle B', '[]', 'cycle-A', 2, 1, 'CASCADE_RUN_STATUS_RUNNING', ?);
		`, now.Format(time.RFC3339))
		if err != nil {
			t.Fatalf("failed to insert cycle B: %v", err)
		}

		detector := &SessionDetector{BaseDir: tmpDir}
		done := make(chan struct{})
		var info *ActiveSessionInfo
		var detectErr error

		go func() {
			info, detectErr = detector.Detect("")
			close(done)
		}()

		select {
		case <-done:
			if detectErr != nil {
				t.Fatalf("Detect failed: %v", detectErr)
			}
			t.Logf("Mutual recursion terminated safely in %s with depth %d", info.ConversationID, info.NestingDepth)
		case <-time.After(3 * time.Second):
			t.Fatalf("Detect hung in infinite loop on mutual cyclic parent pointers!")
		}
	})
}

// TestStress_AncestorTree_NeedsRevivalPropagation tests that running/non-idle state anywhere
// in the ancestor tree correctly propagates needsRevival = true to root.
func TestStress_AncestorTree_NeedsRevivalPropagation(t *testing.T) {
	testCases := []struct {
		name                 string
		rootStatus           string
		rootIdle             int
		midStatus            string
		midIdle              int
		leafStatus           string
		leafIdle             int
		expectedNeedsRevival bool
	}{
		{
			name:                 "LeafRunning_MidIdle_RootIdle",
			rootStatus:           "CASCADE_RUN_STATUS_IDLE",
			rootIdle:             0,
			midStatus:            "CASCADE_RUN_STATUS_IDLE",
			midIdle:              0,
			leafStatus:           "CASCADE_RUN_STATUS_RUNNING",
			leafIdle:             0,
			expectedNeedsRevival: true,
		},
		{
			name:                 "LeafIdle_MidNotFullyIdle_RootIdle",
			rootStatus:           "CASCADE_RUN_STATUS_IDLE",
			rootIdle:             0,
			midStatus:            "CASCADE_RUN_STATUS_IDLE",
			midIdle:              1,
			leafStatus:           "CASCADE_RUN_STATUS_IDLE",
			leafIdle:             0,
			expectedNeedsRevival: true,
		},
		{
			name:                 "LeafIdle_MidRunning_RootIdle",
			rootStatus:           "CASCADE_RUN_STATUS_IDLE",
			rootIdle:             0,
			midStatus:            "CASCADE_RUN_STATUS_RUNNING",
			midIdle:              0,
			leafStatus:           "CASCADE_RUN_STATUS_IDLE",
			leafIdle:             0,
			expectedNeedsRevival: true,
		},
		{
			name:                 "LeafIdle_MidIdle_RootRunning",
			rootStatus:           "CASCADE_RUN_STATUS_RUNNING",
			rootIdle:             0,
			midStatus:            "CASCADE_RUN_STATUS_IDLE",
			midIdle:              0,
			leafStatus:           "CASCADE_RUN_STATUS_IDLE",
			leafIdle:             0,
			expectedNeedsRevival: true,
		},
		{
			name:                 "AllIdle_NoRevivalNeeded",
			rootStatus:           "CASCADE_RUN_STATUS_IDLE",
			rootIdle:             0,
			midStatus:            "CASCADE_RUN_STATUS_IDLE",
			midIdle:              0,
			leafStatus:           "CASCADE_RUN_STATUS_IDLE",
			leafIdle:             0,
			expectedNeedsRevival: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			dbPath := setupMockSummariesDB(t, tmpDir)

			db, err := sql.Open("sqlite", dbPath)
			if err != nil {
				t.Fatalf("failed to open db: %v", err)
			}
			defer db.Close()

			now := time.Now()
			_, _ = db.Exec(`
				INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
				VALUES ('root-prop', 'Root', '[]', '', 0, ?, ?, ?);
			`, tc.rootIdle, tc.rootStatus, now.Add(-2*time.Minute).Format(time.RFC3339))

			_, _ = db.Exec(`
				INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
				VALUES ('mid-prop', 'Mid', '[]', 'root-prop', 1, ?, ?, ?);
			`, tc.midIdle, tc.midStatus, now.Add(-1*time.Minute).Format(time.RFC3339))

			_, _ = db.Exec(`
				INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
				VALUES ('leaf-prop', 'Leaf', '[]', 'mid-prop', 2, ?, ?, ?);
			`, tc.leafIdle, tc.leafStatus, now.Format(time.RFC3339))

			detector := &SessionDetector{BaseDir: tmpDir}
			info, err := detector.Detect("")
			if err != nil {
				t.Fatalf("Detect failed: %v", err)
			}

			if info.ConversationID != "root-prop" {
				t.Errorf("expected root-prop, got %s", info.ConversationID)
			}
			if info.NeedsRevival != tc.expectedNeedsRevival {
				t.Errorf("expected NeedsRevival=%v, got %v", tc.expectedNeedsRevival, info.NeedsRevival)
			}
		})
	}
}

// =========================================================================
// 2. Interruption Detection Stress-Testing
// =========================================================================

// TestStress_InterruptionDetection_Matrix validates combination states:
// not_fully_idle (0 vs 1), step status (0, 1, 2, 3, 4, 5), summary status (IDLE vs RUNNING).
func TestStress_InterruptionDetection_Matrix(t *testing.T) {
	type testCase struct {
		notFullyIdle    int
		summaryStatus   string
		hasTrajDB       bool
		trajStepStatus  int
		wantNeedRevival bool
	}

	cases := []testCase{
		// not_fully_idle = 1 always forces needsRevival = true regardless of steps
		{notFullyIdle: 1, summaryStatus: "CASCADE_RUN_STATUS_IDLE", hasTrajDB: false, trajStepStatus: 0, wantNeedRevival: true},
		{notFullyIdle: 1, summaryStatus: "CASCADE_RUN_STATUS_IDLE", hasTrajDB: true, trajStepStatus: 3, wantNeedRevival: true},
		{notFullyIdle: 1, summaryStatus: "CASCADE_RUN_STATUS_IDLE", hasTrajDB: true, trajStepStatus: 4, wantNeedRevival: true},

		// CASCADE_RUN_STATUS_RUNNING forces needsRevival = true
		{notFullyIdle: 0, summaryStatus: "CASCADE_RUN_STATUS_RUNNING", hasTrajDB: false, trajStepStatus: 0, wantNeedRevival: true},
		{notFullyIdle: 0, summaryStatus: "CASCADE_RUN_STATUS_RUNNING", hasTrajDB: true, trajStepStatus: 3, wantNeedRevival: true},

		// Idle summary, step status 2 (RUNNING) in trajectory -> needsRevival = true
		{notFullyIdle: 0, summaryStatus: "CASCADE_RUN_STATUS_IDLE", hasTrajDB: true, trajStepStatus: 2, wantNeedRevival: true},

		// Idle summary, step status 3 (COMPLETED) -> needsRevival = false
		{notFullyIdle: 0, summaryStatus: "CASCADE_RUN_STATUS_IDLE", hasTrajDB: true, trajStepStatus: 3, wantNeedRevival: false},

		// Idle summary, step status 4 (FAILED / ERROR) -> needsRevival = false
		{notFullyIdle: 0, summaryStatus: "CASCADE_RUN_STATUS_IDLE", hasTrajDB: true, trajStepStatus: 4, wantNeedRevival: false},

		// Idle summary, step status 5 (FINISHED) -> needsRevival = false
		{notFullyIdle: 0, summaryStatus: "CASCADE_RUN_STATUS_IDLE", hasTrajDB: true, trajStepStatus: 5, wantNeedRevival: false},

		// Idle summary, step status 0 (UNSPECIFIED) -> needsRevival = false
		{notFullyIdle: 0, summaryStatus: "CASCADE_RUN_STATUS_IDLE", hasTrajDB: true, trajStepStatus: 0, wantNeedRevival: false},

		// Idle summary, step status 1 (QUEUED) -> needsRevival = false (unless not_fully_idle=1)
		{notFullyIdle: 0, summaryStatus: "CASCADE_RUN_STATUS_IDLE", hasTrajDB: true, trajStepStatus: 1, wantNeedRevival: false},

		// Idle summary, no trajectory DB -> needsRevival = false
		{notFullyIdle: 0, summaryStatus: "CASCADE_RUN_STATUS_IDLE", hasTrajDB: false, trajStepStatus: 0, wantNeedRevival: false},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("Case_%d_idle%d_stat%s_traj%v_step%d", i, c.notFullyIdle, c.summaryStatus, c.hasTrajDB, c.trajStepStatus), func(t *testing.T) {
			tmpDir := t.TempDir()
			dbPath := setupMockSummariesDB(t, tmpDir)

			db, err := sql.Open("sqlite", dbPath)
			if err != nil {
				t.Fatalf("failed to open db: %v", err)
			}
			defer db.Close()

			convID := fmt.Sprintf("conv-matrix-%d", i)
			now := time.Now().Format(time.RFC3339)
			_, err = db.Exec(`
				INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
				VALUES (?, 'Matrix Test', '[]', '', 0, ?, ?, ?);
			`, convID, c.notFullyIdle, c.summaryStatus, now)
			if err != nil {
				t.Fatalf("failed to insert summary: %v", err)
			}

			if c.hasTrajDB {
				setupMockTrajectoryDB(t, tmpDir, convID, c.trajStepStatus)
			}

			detector := &SessionDetector{BaseDir: tmpDir}
			info, err := detector.Detect("")
			if err != nil {
				t.Fatalf("Detect failed: %v", err)
			}

			if info.NeedsRevival != c.wantNeedRevival {
				t.Errorf("case %d: expected NeedsRevival=%v, got %v", i, c.wantNeedRevival, info.NeedsRevival)
			}
		})
	}
}

// TestStress_InterruptionDetection_IdleSessionCapture inspects what CapturePreSwitchState
// produces when the conversation is completely idle (NeedsRevival == false).
// Adversarial challenge: Does CapturePreSwitchState still create a prompt injection intent for idle sessions?
func TestStress_InterruptionDetection_IdleSessionCapture(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := setupMockSummariesDB(t, tmpDir)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	now := time.Now().Format(time.RFC3339)
	// Completely idle conversation: not_fully_idle = 0, status = IDLE, step status = 3 (COMPLETED)
	_, _ = db.Exec(`
		INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
		VALUES ('conv-idle-session', 'Finished User Query', '[]', '', 0, 0, 'CASCADE_RUN_STATUS_IDLE', ?);
	`, now)
	setupMockTrajectoryDB(t, tmpDir, "conv-idle-session", 3)

	engine := NewEngine(tmpDir, 9222)
	engine.Detector.BaseDir = tmpDir

	session, err := engine.Detector.Detect("desktop")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if session.NeedsRevival {
		t.Fatalf("expected session.NeedsRevival = false for completely idle conversation!")
	}

	intent, err := engine.CapturePreSwitchState("desktop")
	if err != nil {
		t.Fatalf("CapturePreSwitchState failed: %v", err)
	}

	// Adversarial observation:
	// Even though session.NeedsRevival == false, an intent is still created with a prompt!
	if intent != nil {
		t.Logf("OBSERVATION: CapturePreSwitchState created RevivalIntent with TriggerPrompt=%q even when session.NeedsRevival=false", intent.TriggerPrompt)
	}
}

// =========================================================================
// 3. Concurrency & WAL Safety
// =========================================================================

// TestStress_Concurrency_SQLite_WAL tests whether querying SQLite in read-only WAL mode
// ever causes lock contention, busy errors, or blocks concurrent database writers.
func TestStress_Concurrency_SQLite_WAL(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := setupMockSummariesDB(t, tmpDir)

	// Connect writer with WAL and busy timeout
	writeConnStr := fmt.Sprintf("file:%s?_busy_timeout=5000", filepath.ToSlash(dbPath))
	writeDB, err := sql.Open("sqlite", writeConnStr)
	if err != nil {
		t.Fatalf("failed to open writer db: %v", err)
	}
	defer writeDB.Close()

	if _, err := writeDB.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		t.Fatalf("failed to set WAL mode: %v", err)
	}

	// Insert initial seed row
	_, _ = writeDB.Exec(`
		INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
		VALUES ('seed-conv', 'Seed', '[]', '', 0, 0, 'CASCADE_RUN_STATUS_IDLE', ?);
	`, time.Now().Format(time.RFC3339))

	var stopFlag int32
	var writerErrors int64
	var readerErrors int64
	var readSuccessCount int64
	var writeSuccessCount int64
	var lastWriterErr string
	var lastReaderErr string
	var errMu sync.Mutex

	var wg sync.WaitGroup

	// Antigravity has 1 main host process writing summaries
	wg.Add(1)
	go func() {
		defer wg.Done()
		for atomic.LoadInt32(&stopFlag) == 0 {
			id := fmt.Sprintf("conv-w-%d", time.Now().UnixNano())
			now := time.Now().Format(time.RFC3339)
			_, err := writeDB.Exec(`
				INSERT OR REPLACE INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
				VALUES (?, 'Concurrent Write', '[]', '', 0, 1, 'CASCADE_RUN_STATUS_RUNNING', ?);
			`, id, now)
			if err != nil {
				atomic.AddInt64(&writerErrors, 1)
				errMu.Lock()
				lastWriterErr = err.Error()
				errMu.Unlock()
			} else {
				atomic.AddInt64(&writeSuccessCount, 1)
			}
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Start 20 concurrent readers calling Detect()
	detector := &SessionDetector{BaseDir: tmpDir}
	for r := 0; r < 20; r++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			for atomic.LoadInt32(&stopFlag) == 0 {
				info, err := detector.Detect("")
				if err != nil || info == nil || info.ConversationID == "" {
					atomic.AddInt64(&readerErrors, 1)
				} else {
					atomic.AddInt64(&readSuccessCount, 1)
				}
				time.Sleep(2 * time.Millisecond)
			}
		}(r)
	}

	// Run for 1.5 seconds under intense contention
	time.Sleep(1500 * time.Millisecond)
	atomic.StoreInt32(&stopFlag, 1)
	wg.Wait()

	t.Logf("WAL Stress Test Results: Writes=%d (errors=%d, lastErr=%q), Reads=%d (errors=%d, lastErr=%q)",
		writeSuccessCount, writerErrors, lastWriterErr, readSuccessCount, readerErrors, lastReaderErr)

	if readerErrors > 0 {
		t.Errorf("read-only WAL readers encountered %d errors (last: %s) under concurrency!", readerErrors, lastReaderErr)
	}
	if writerErrors > 0 {
		t.Errorf("SQLite writers encountered %d errors (last: %s)!", writerErrors, lastWriterErr)
	}
}

// TestStress_Concurrency_Store_FileWrites tests concurrent operations on Store:
// SaveIntent, LoadIntent, UpdateStatus, ClearIntent across 40 goroutines.
func TestStress_Concurrency_Store_FileWrites(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	var wg sync.WaitGroup
	var errorsCount int64
	var stopFlag int32

	// 20 concurrent writers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for atomic.LoadInt32(&stopFlag) == 0 {
				intent := &RevivalIntent{
					IntentID:           fmt.Sprintf("intent-%d-%d", id, rand.Int63()),
					RootConversationID: fmt.Sprintf("conv-%d", id),
					TriggerPrompt:      "Continue tasks",
					CreatedAt:          time.Now(),
					Status:             IntentStatusPending,
					TTLSeconds:         60,
				}
				if err := store.SaveIntent(intent); err != nil {
					atomic.AddInt64(&errorsCount, 1)
				}
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	// 20 concurrent readers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for atomic.LoadInt32(&stopFlag) == 0 {
				intent, err := store.LoadIntent()
				if err != nil {
					// ReadFile or unmarshal failed
					atomic.AddInt64(&errorsCount, 1)
				}
				if intent != nil && intent.RootConversationID == "" {
					atomic.AddInt64(&errorsCount, 1)
				}
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	// Run under concurrent load
	time.Sleep(1200 * time.Millisecond)
	atomic.StoreInt32(&stopFlag, 1)
	wg.Wait()

	if errorsCount > 0 {
		t.Errorf("Store experienced %d concurrency errors during simultaneous read/write!", errorsCount)
	} else {
		t.Logf("Store concurrency passed: 0 read/write errors across 40 parallel goroutines")
	}
}

// TestStress_Concurrency_MultiStore_TmpClash tests what happens when MULTIPLE Store instances
// (e.g. from separate packages or processes) concurrently write to the same directory.
// Because s.filePath + ".tmp" is a fixed path, multiple instances might collide!
func TestStress_Concurrency_MultiStore_TmpClash(t *testing.T) {
	tmpDir := t.TempDir()

	var wg sync.WaitGroup
	var errorsCount int64
	var stopFlag int32

	// 15 separate Store instances pointing to the same directory
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			instStore := NewStore(tmpDir)
			for atomic.LoadInt32(&stopFlag) == 0 {
				intent := &RevivalIntent{
					IntentID:           fmt.Sprintf("multi-inst-%d-%d", id, rand.Int63()),
					RootConversationID: fmt.Sprintf("conv-multi-%d", id),
					TriggerPrompt:      "Multi store prompt",
					CreatedAt:          time.Now(),
					Status:             IntentStatusPending,
					TTLSeconds:         60,
				}
				if err := instStore.SaveIntent(intent); err != nil {
					atomic.AddInt64(&errorsCount, 1)
				}
				time.Sleep(time.Duration(rand.Intn(3)) * time.Millisecond)
			}
		}(i)
	}

	time.Sleep(1000 * time.Millisecond)
	atomic.StoreInt32(&stopFlag, 1)
	wg.Wait()

	t.Logf("Multi-Store clash test finished with %d errors", errorsCount)
	if errorsCount > 0 {
		t.Logf("NOTICE: Fixed tmp filename (%s.tmp) experienced collisions when written by independent Store instances: %d errors",
			filepath.Join(tmpDir, "pending_continuation.json"), errorsCount)
	}
}

// TestStress_CorruptedFiles tests recovery when pending_continuation.json contains corrupted JSON.
func TestStress_CorruptedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	// Write invalid JSON
	_ = os.WriteFile(store.FilePath(), []byte("{ invalid json truncated..."), 0644)

	intent, err := store.LoadIntent()
	if err == nil {
		t.Errorf("expected error when loading malformed JSON, got nil (intent=%v)", intent)
	}
	if intent != nil {
		t.Errorf("expected nil intent on corrupt JSON")
	}

	// SaveIntent should safely overwrite corrupted file
	validIntent := &RevivalIntent{
		IntentID:           "recovered-intent",
		RootConversationID: "conv-clean",
		Status:             IntentStatusPending,
	}
	if err := store.SaveIntent(validIntent); err != nil {
		t.Fatalf("failed to overwrite corrupt intent: %v", err)
	}

	loaded, err := store.LoadIntent()
	if err != nil || loaded == nil || loaded.RootConversationID != "conv-clean" {
		t.Fatalf("failed to recover after corrupt file overwrite: %v", err)
	}
}
