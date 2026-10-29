package revival

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
	_ "modernc.org/sqlite"
)

// insertSummary is a small helper for the multi-intent fixtures.
func insertSummary(t *testing.T, dbPath, convID, parentID string, depth int, notFullyIdle int, status string, modTime time.Time) {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open %s: %v", dbPath, err)
	}
	defer db.Close()
	_, err = db.Exec(`
		INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
		VALUES (?, ?, '[]', ?, ?, ?, ?, ?);`,
		convID, "Conversation "+convID, parentID, depth, notFullyIdle, status, modTime.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("failed to insert summary %s: %v", convID, err)
	}
}

// TestDetector_FinishedConversationOnLivePath_DoesNotNeedRevival is the regression
// test for the root cause: Detect's live-renderer branch used to force
// NeedsRevival = true for any conversation that was open and touched within
// MaxRevivalStaleness, discarding the honest DB/trajectory verdict.
//
// The live path is injected through the detector's LivePathResolver seam, so this
// test actually executes that branch. Every prior test set LiveCDPEnabled = false,
// which is exactly why the suite never caught the bug.
func TestDetector_FinishedConversationOnLivePath_DoesNotNeedRevival(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := setupMockSummariesDB(t, tmpDir)

	convID := "5f0a1b22-3c4d-4e5f-8a90-1b2c3d4e5f60"
	// Exactly the live evidence: finished turn, idle status, final step (3).
	insertSummary(t, dbPath, convID, "", 0, 0, "CASCADE_RUN_STATUS_IDLE", time.Now())
	setupMockTrajectoryDB(t, tmpDir, convID, 3)

	detector := NewSessionDetector()
	detector.BaseDir = tmpDir
	detector.LiveCDPEnabled = true
	detector.LivePathResolver = func() string { return "/c/" + convID }

	// Precondition: the live path really is consulted.
	if !detector.liveRouteAvailable() {
		t.Fatalf("test setup broken: live path is not reachable, the branch under test would not run")
	}

	session, err := detector.Detect("desktop")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if session.ConversationID != convID {
		t.Fatalf("expected the open conversation %s, got %s", convID, session.ConversationID)
	}
	if session.NeedsRevival {
		t.Errorf("REGRESSION: a finished conversation that merely happens to be open reported NeedsRevival = true "+
			"(not_fully_idle=%v, status=%s, last trajectory step=3)", session.NotFullyIdle, session.Status)
	}

	// And the prompt the dispatchers consume must stay blank.
	engine := NewEngine(tmpDir, 0)
	engine.Detector = detector
	engine.Detector.LivePathResolver = func() string { return "/c/" + convID }

	intents, err := engine.CapturePreSwitchIntents("desktop")
	if err != nil {
		t.Fatalf("CapturePreSwitchIntents failed: %v", err)
	}
	if len(intents) != 1 {
		t.Fatalf("expected exactly the layout-preserving intent for the open conversation, got %d", len(intents))
	}
	if intents[0].TriggerPrompt != "" || intents[0].Prompt != "" {
		t.Errorf("REGRESSION: finished conversation received a resume prompt %q (trigger %q)",
			intents[0].Prompt, intents[0].TriggerPrompt)
	}
}

// TestDetector_DetectAll_ThreeRunningConversations covers the second requirement:
// an account switch that kills three running conversations must produce three
// intents, each with its own prompt, all readable from the collection store.
func TestDetector_DetectAll_ThreeRunningConversations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := setupMockSummariesDB(t, tmpDir)

	now := time.Now()
	convIDs := []string{
		"11111111-1111-4111-8111-111111111111",
		"22222222-2222-4222-8222-222222222222",
		"33333333-3333-4333-8333-333333333333",
	}
	for i, convID := range convIDs {
		// Distinct, ordered timestamps so "newest first" is unambiguous.
		insertSummary(t, dbPath, convID, "", 0, 1, "CASCADE_RUN_STATUS_RUNNING", now.Add(time.Duration(i)*time.Second))
		setupMockTrajectoryDB(t, tmpDir, convID, 2)
	}
	// A finished conversation that must never be picked up.
	finished := "44444444-4444-4444-8444-444444444444"
	insertSummary(t, dbPath, finished, "", 0, 0, "CASCADE_RUN_STATUS_IDLE", now)
	setupMockTrajectoryDB(t, tmpDir, finished, 3)

	detector := NewSessionDetector()
	detector.BaseDir = tmpDir
	detector.LiveCDPEnabled = false

	sessions, err := detector.DetectAll("desktop")
	if err != nil {
		t.Fatalf("DetectAll failed: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("expected 3 interrupted conversations, got %d: %+v", len(sessions), sessions)
	}
	seen := map[string]bool{}
	for _, session := range sessions {
		if !session.NeedsRevival {
			t.Errorf("detected session %s without NeedsRevival", session.ConversationID)
		}
		seen[session.ConversationID] = true
	}
	for _, convID := range convIDs {
		if !seen[convID] {
			t.Errorf("DetectAll missed interrupted conversation %s", convID)
		}
	}
	if seen[finished] {
		t.Errorf("DetectAll must not report the finished conversation %s", finished)
	}

	engine := NewEngine(tmpDir, 0)
	engine.Detector.BaseDir = tmpDir
	engine.Detector.LiveCDPEnabled = false

	intents, err := engine.CapturePreSwitchIntents("desktop")
	if err != nil {
		t.Fatalf("CapturePreSwitchIntents failed: %v", err)
	}
	if len(intents) != 3 {
		t.Fatalf("expected one intent per interrupted conversation, got %d", len(intents))
	}
	for _, intent := range intents {
		if strings.TrimSpace(intent.TriggerPrompt) == "" {
			t.Errorf("intent for interrupted conversation %s has a blank prompt", intent.RootConversationID)
		}
	}

	loaded, err := engine.Store.LoadIntents()
	if err != nil {
		t.Fatalf("LoadIntents failed: %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("LoadIntents must return all three intents, got %d", len(loaded))
	}
	for _, intent := range loaded {
		if _, ok := seen[intent.RootConversationID]; !ok {
			t.Errorf("unexpected intent for %s", intent.RootConversationID)
		}
	}
	// Newest first.
	if !loaded[0].CreatedAt.After(loaded[len(loaded)-1].CreatedAt) && !loaded[0].CreatedAt.Equal(loaded[len(loaded)-1].CreatedAt) {
		t.Errorf("LoadIntents must return newest first, got %v then %v", loaded[0].CreatedAt, loaded[len(loaded)-1].CreatedAt)
	}
}

// TestStore_ClearIntentForScopesToOneIntent proves acking one conversation leaves
// the others queued (requirement 2: one prompt per interrupted conversation).
func TestStore_ClearIntentForScopesToOneIntent(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	convIDs := []string{
		"aaaa1111-1111-4111-8111-111111111111",
		"bbbb2222-2222-4222-8222-222222222222",
		"cccc3333-3333-4333-8333-333333333333",
	}
	base := time.Now()
	for i, convID := range convIDs {
		err := store.SaveIntent(&RevivalIntent{
			IntentID:           fmt.Sprintf("intent-%d", i),
			RootConversationID: convID,
			TriggerPrompt:      DefaultTriggerPrompt,
			CreatedAt:          base.Add(time.Duration(i) * time.Second),
			Status:             IntentStatusPending,
			TTLSeconds:         600,
		})
		if err != nil {
			t.Fatalf("SaveIntent(%s) failed: %v", convID, err)
		}
	}

	loaded, err := store.LoadIntents()
	if err != nil || len(loaded) != 3 {
		t.Fatalf("expected 3 intents before ack, got %d (err=%v)", len(loaded), err)
	}

	if err := store.ClearIntentFor(convIDs[1]); err != nil {
		t.Fatalf("ClearIntentFor failed: %v", err)
	}

	remaining, err := store.LoadIntents()
	if err != nil {
		t.Fatalf("LoadIntents failed: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("expected 2 intents to survive a scoped ack, got %d", len(remaining))
	}
	for _, intent := range remaining {
		if intent.RootConversationID == convIDs[1] {
			t.Errorf("acked conversation %s is still pending", convIDs[1])
		}
	}

	// Acking something that is not pending is a no-op, not an error.
	if err := store.ClearIntentFor("dddd4444-4444-4444-8444-444444444444"); err != nil {
		t.Errorf("ClearIntentFor on an absent conversation must be a no-op, got %v", err)
	}
	if after, _ := store.LoadIntents(); len(after) != 2 {
		t.Errorf("no-op ack changed the collection: %d intents left", len(after))
	}
}

// TestEngine_RestartDoesNotReSynthesizeRevivedConversation proves cooldown state
// survives a daemon restart: a fresh Engine over the same config dir must not
// re-arm a conversation that was already revived.
func TestEngine_RestartDoesNotReSynthesizeRevivedConversation(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := setupMockSummariesDB(t, tmpDir)

	convID := "55555555-5555-4555-8555-555555555555"
	insertSummary(t, dbPath, convID, "", 0, 1, "CASCADE_RUN_STATUS_RUNNING", time.Now())
	setupMockTrajectoryDB(t, tmpDir, convID, 2)

	engine := NewEngine(tmpDir, 0)
	engine.Detector.BaseDir = tmpDir
	engine.Detector.LiveCDPEnabled = false

	intents, err := engine.CapturePreSwitchIntents("desktop")
	if err != nil || len(intents) != 1 {
		t.Fatalf("expected 1 captured intent, got %d (err=%v)", len(intents), err)
	}
	if err := engine.ExecutePostRelaunchRevival(nil); err != nil {
		t.Fatalf("ExecutePostRelaunchRevival failed: %v", err)
	}
	if left, _ := engine.Store.LoadIntents(); len(left) != 0 {
		t.Fatalf("expected the revived intent to be cleared, %d left", len(left))
	}

	// Cooldown state must be on disk.
	if _, statErr := os.Stat(filepath.Join(tmpDir, CooldownFileName)); statErr != nil {
		t.Fatalf("expected cooldown state to be persisted to %s: %v", CooldownFileName, statErr)
	}

	// Restart: brand-new engine over the same config dir.
	restarted := NewEngine(tmpDir, 0)
	restarted.Detector.BaseDir = tmpDir
	restarted.Detector.LiveCDPEnabled = false
	if !restarted.isSuppressed(convID) {
		t.Fatalf("restarted engine did not restore the cooldown for %s", convID)
	}

	reCaptured, recaptureErr := restarted.CapturePreSwitchIntents("desktop")
	if recaptureErr != nil && len(reCaptured) > 0 {
		t.Fatalf("unexpected state: %d intents and error %v", len(reCaptured), recaptureErr)
	}
	for _, intent := range reCaptured {
		if intent.RootConversationID == convID {
			t.Errorf("REGRESSION: restart re-synthesized an intent for already-revived conversation %s", convID)
		}
	}

	// The status surface must not hand it to a dispatcher either.
	status, err := restarted.GetRevivalStatus(convID)
	if err != nil {
		t.Fatalf("GetRevivalStatus failed: %v", err)
	}
	for _, intent := range status.PendingIntents {
		if intent.RootConversationID == convID {
			t.Errorf("REGRESSION: restarted engine still publishes an intent for %s", convID)
		}
	}
	if status.PendingIntent != nil && status.PendingIntent.RootConversationID == convID {
		t.Errorf("REGRESSION: legacy PendingIntent still points at already-revived %s", convID)
	}
}

// TestEngine_ExecutePostRelaunchRevival_RevivesEveryIntent proves the post-relaunch
// pass drains the whole collection rather than only the newest intent.
func TestEngine_ExecutePostRelaunchRevival_RevivesEveryIntent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := setupMockSummariesDB(t, tmpDir)

	now := time.Now()
	convIDs := []string{
		"66666666-6666-4666-8666-666666666666",
		"77777777-7777-4777-8777-777777777777",
		"88888888-8888-4888-8888-888888888888",
	}
	for i, convID := range convIDs {
		insertSummary(t, dbPath, convID, "", 0, 1, "CASCADE_RUN_STATUS_RUNNING", now.Add(time.Duration(i)*time.Second))
		setupMockTrajectoryDB(t, tmpDir, convID, 2)
	}

	engine := NewEngine(tmpDir, 0)
	engine.Detector.BaseDir = tmpDir
	engine.Detector.LiveCDPEnabled = false

	if intents, err := engine.CapturePreSwitchIntents("desktop"); err != nil || len(intents) != 3 {
		t.Fatalf("expected 3 captured intents, got %d (err=%v)", len(intents), err)
	}

	if err := engine.ExecutePostRelaunchRevival(nil); err != nil {
		t.Fatalf("ExecutePostRelaunchRevival failed: %v", err)
	}

	left, err := engine.Store.LoadIntents()
	if err != nil {
		t.Fatalf("LoadIntents failed: %v", err)
	}
	if len(left) != 0 {
		t.Fatalf("expected every intent to be revived and cleared, %d left: %+v", len(left), left)
	}
	for _, convID := range convIDs {
		if !engine.isSuppressed(convID) {
			t.Errorf("conversation %s was not marked revived", convID)
		}
	}
}

// TestStore_LoadIntentsMigratesLegacySingleObject proves the upgrade path: a
// pre-collection pending_continuation.json is read as a one-element list and the
// legacy file is removed once the plural file is committed.
func TestStore_LoadIntentsMigratesLegacySingleObject(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	convID := "99999999-9999-4999-8999-999999999999"
	legacy := fmt.Sprintf(`{
  "intent_id": "intent-legacy",
  "target_app": "desktop",
  "root_conversation_id": %q,
  "trigger_prompt": "Please continue ongoing tasks and subagents.",
  "created_at": %q,
  "status": "pending",
  "cascade_id": %q,
  "prompt": "Please continue ongoing tasks and subagents.",
  "timestamp": %d
}`, convID, time.Now().Format(time.RFC3339Nano), convID, time.Now().UnixMilli())
	if err := os.WriteFile(filepath.Join(tmpDir, LegacyIntentsFileName), []byte(legacy), 0600); err != nil {
		t.Fatalf("failed to write legacy file: %v", err)
	}

	intents, err := store.LoadIntents()
	if err != nil {
		t.Fatalf("LoadIntents must accept a legacy flat object, got %v", err)
	}
	if len(intents) != 1 {
		t.Fatalf("expected the legacy object to be read as one intent, got %d", len(intents))
	}
	if intents[0].RootConversationID != convID {
		t.Errorf("expected %s, got %s", convID, intents[0].RootConversationID)
	}
	if intents[0].TriggerPrompt == "" {
		t.Errorf("legacy trigger prompt was lost during migration")
	}

	if err := store.SaveIntent(intents[0]); err != nil {
		t.Fatalf("SaveIntent failed: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(tmpDir, IntentsFileName)); statErr != nil {
		t.Fatalf("expected the plural intents file after migration: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(tmpDir, LegacyIntentsFileName)); !os.IsNotExist(statErr) {
		t.Errorf("legacy file must be removed after a successful save, stat err = %v", statErr)
	}
}

// TestStore_SaveIntentUpsertsByConversation proves one conversation can never
// accumulate two intents (which would mean two prompts).
func TestStore_SaveIntentUpsertsByConversation(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	convID := "abcdabcd-abcd-4bcd-8bcd-abcdabcdabcd"
	for i := 0; i < 5; i++ {
		err := store.SaveIntent(&RevivalIntent{
			IntentID:           fmt.Sprintf("resynthesized-%d", i),
			RootConversationID: convID,
			TriggerPrompt:      DefaultTriggerPrompt,
			CreatedAt:          time.Now(),
			Status:             IntentStatusPending,
		})
		if err != nil {
			t.Fatalf("SaveIntent %d failed: %v", i, err)
		}
	}

	intents, err := store.LoadIntents()
	if err != nil {
		t.Fatalf("LoadIntents failed: %v", err)
	}
	if len(intents) != 1 {
		t.Fatalf("expected 1 intent after repeated saves for one conversation, got %d", len(intents))
	}
	if intents[0].IntentID != "resynthesized-4" {
		t.Errorf("expected the newest save to win, got %s", intents[0].IntentID)
	}
}

// TestDetector_DetectAllDedupesSubagentRoots proves subagent rows collapse onto
// their root conversation: three rows for one root must yield one intent.
func TestDetector_DetectAllDedupesSubagentRoots(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := setupMockSummariesDB(t, tmpDir)

	root := "e0e0e0e0-e0e0-4e0e-8e0e-e0e0e0e0e0e0"
	now := time.Now()
	insertSummary(t, dbPath, root, "", 0, 1, "CASCADE_RUN_STATUS_RUNNING", now)
	insertSummary(t, dbPath, "e1e1e1e1-e1e1-4e1e-8e1e-e1e1e1e1e1e1", root, 1, 1, "CASCADE_RUN_STATUS_RUNNING", now)
	insertSummary(t, dbPath, "e2e2e2e2-e2e2-4e2e-8e2e-e2e2e2e2e2e2", root, 2, 1, "CASCADE_RUN_STATUS_RUNNING", now)

	detector := NewSessionDetector()
	detector.BaseDir = tmpDir
	detector.LiveCDPEnabled = false

	sessions, err := detector.DetectAll("desktop")
	if err != nil {
		t.Fatalf("DetectAll failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected subagent rows to collapse onto their root, got %d sessions", len(sessions))
	}
	if sessions[0].ConversationID != root {
		t.Errorf("expected root %s, got %s", root, sessions[0].ConversationID)
	}
}

// TestStore_ConcurrentSaveAndClearFor exercises the keyed collection under
// concurrent scoped clears to make sure one conversation's ack cannot drop or
// duplicate another's intent.
func TestStore_ConcurrentSaveAndClearFor(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	const convCount = 6
	convIDs := make([]string, convCount)
	for i := range convIDs {
		convIDs[i] = fmt.Sprintf("f%07d-0000-4000-8000-000000000000", i)
	}

	var wg sync.WaitGroup
	for i, convID := range convIDs {
		wg.Add(1)
		go func(idx int, id string) {
			defer wg.Done()
			for round := 0; round < 10; round++ {
				err := store.SaveIntent(&RevivalIntent{
					IntentID:           fmt.Sprintf("intent-%d-%d", idx, round),
					RootConversationID: id,
					TriggerPrompt:      DefaultTriggerPrompt,
					CreatedAt:          time.Now(),
					Status:             IntentStatusPending,
				})
				if err != nil {
					t.Errorf("SaveIntent(%s) failed: %v", id, err)
					return
				}
			}
		}(i, convID)
	}
	wg.Wait()

	intents, err := store.LoadIntents()
	if err != nil {
		t.Fatalf("LoadIntents failed: %v", err)
	}
	if len(intents) != convCount {
		t.Fatalf("expected %d intents after concurrent saves, got %d", convCount, len(intents))
	}

	// Clear half concurrently; the other half must survive intact.
	for i := 0; i < convCount/2; i++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if err := store.ClearIntentFor(id); err != nil {
				t.Errorf("ClearIntentFor(%s) failed: %v", id, err)
			}
		}(convIDs[i])
	}
	wg.Wait()

	remaining, err := store.LoadIntents()
	if err != nil {
		t.Fatalf("LoadIntents failed: %v", err)
	}
	if len(remaining) != convCount/2 {
		t.Fatalf("expected %d intents to survive scoped clears, got %d", convCount/2, len(remaining))
	}
	for _, intent := range remaining {
		if !gui.IsValidConversationID(intent.RootConversationID) {
			t.Errorf("surviving intent has invalid conversation ID %q", intent.RootConversationID)
		}
	}
}
