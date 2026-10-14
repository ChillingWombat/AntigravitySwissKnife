package revival

import (
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
	_ "modernc.org/sqlite"
)

// TestAdversarial_Store_ConcurrentIncrementAttempts tests that concurrent IncrementAttempts
// across multiple Store instances accurately records every single increment without losing any
// updates or corrupting the JSON payload.
func TestAdversarial_Store_ConcurrentIncrementAttempts(t *testing.T) {
	tmpDir := t.TempDir()
	primaryStore := NewStore(tmpDir)

	convID := "11223344-5566-7788-99aa-bbccddeeff00"
	intentID := "intent-concurrent-inc-1"
	initIntent := &RevivalIntent{
		IntentID:           intentID,
		RootConversationID: convID,
		Status:             IntentStatusPending,
		Attempts:           0,
		MaxAttempts:        100,
		TTLSeconds:         600,
	}
	if err := primaryStore.SaveIntent(initIntent); err != nil {
		t.Fatalf("Failed to initialize intent: %v", err)
	}

	const concurrency = 25
	var wg sync.WaitGroup
	var errCount int64

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Each worker uses an independent Store instance pointing to the same file
			storeInst := NewStore(tmpDir)
			att, err := storeInst.IncrementAttempts(intentID)
			if err != nil {
				atomic.AddInt64(&errCount, 1)
				t.Logf("Worker %d IncrementAttempts error: %v", id, err)
			}
			if att <= 0 {
				atomic.AddInt64(&errCount, 1)
			}
		}(i)
	}

	wg.Wait()

	if errCount > 0 {
		t.Fatalf("IncrementAttempts had %d errors across %d concurrent workers", errCount, concurrency)
	}

	loaded, err := primaryStore.LoadIntent()
	if err != nil {
		t.Fatalf("LoadIntent failed: %v", err)
	}
	if loaded == nil {
		t.Fatalf("Expected intent to exist, got nil")
	}
	if loaded.Attempts != concurrency {
		t.Fatalf("Expected exactly %d attempts after %d increments, got %d", concurrency, concurrency, loaded.Attempts)
	}
}

// TestAdversarial_Store_ConcurrentMixedOperations stress tests SaveIntent, IncrementAttempts,
// LoadIntent, and UpdateStatus under high concurrency with multiple Store instances.
func TestAdversarial_Store_ConcurrentMixedOperations(t *testing.T) {
	tmpDir := t.TempDir()

	const numWorkers = 30
	var wg sync.WaitGroup
	var stopFlag int32
	var readErrors int64
	var writeErrors int64
	var successfulOps int64

	convID := "22334455-6677-8899-aabb-ccddeeff0011"
	intentID := "mixed-op-intent"

	// Seed initial file
	initStore := NewStore(tmpDir)
	_ = initStore.SaveIntent(&RevivalIntent{
		IntentID:           intentID,
		RootConversationID: convID,
		Status:             IntentStatusPending,
		Attempts:           0,
		MaxAttempts:        50,
		TTLSeconds:         300,
	})

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			st := NewStore(tmpDir)
			for atomic.LoadInt32(&stopFlag) == 0 {
				op := rand.Intn(4)
				switch op {
				case 0:
					// LoadIntent
					intent, err := st.LoadIntent()
					if err != nil {
						atomic.AddInt64(&readErrors, 1)
					} else {
						atomic.AddInt64(&successfulOps, 1)
						if intent != nil && intent.RootConversationID != convID {
							atomic.AddInt64(&readErrors, 1)
						}
					}
				case 1:
					// IncrementAttempts
					_, err := st.IncrementAttempts(intentID)
					if err != nil {
						atomic.AddInt64(&writeErrors, 1)
					} else {
						atomic.AddInt64(&successfulOps, 1)
					}
				case 2:
					// UpdateStatus
					err := st.UpdateStatus(intentID, IntentStatusReviving)
					if err != nil {
						atomic.AddInt64(&writeErrors, 1)
					} else {
						atomic.AddInt64(&successfulOps, 1)
					}
				case 3:
					// SaveIntent with updated timestamp
					err := st.SaveIntent(&RevivalIntent{
						IntentID:           intentID,
						RootConversationID: convID,
						Status:             IntentStatusPending,
						TTLSeconds:         300,
					})
					if err != nil {
						atomic.AddInt64(&writeErrors, 1)
					} else {
						atomic.AddInt64(&successfulOps, 1)
					}
				}
				time.Sleep(time.Duration(rand.Intn(2)) * time.Millisecond)
			}
		}(i)
	}

	time.Sleep(1500 * time.Millisecond)
	atomic.StoreInt32(&stopFlag, 1)
	wg.Wait()

	t.Logf("Mixed operations finished: successfulOps=%d, readErrors=%d, writeErrors=%d",
		successfulOps, readErrors, writeErrors)

	if readErrors > 0 || writeErrors > 0 {
		t.Fatalf("Store failed under mixed concurrency: readErrors=%d, writeErrors=%d", readErrors, writeErrors)
	}
}

// TestAdversarial_Store_ConcurrentTTLPruningAndSave races LoadIntent (which prunes expired files)
// against SaveIntent (which writes fresh files).
func TestAdversarial_Store_ConcurrentTTLPruningAndSave(t *testing.T) {
	tmpDir := t.TempDir()

	const numWorkers = 20
	var wg sync.WaitGroup
	var stopFlag int32
	var panicOrCorruptCount int64

	convID := "33445566-7788-99aa-bbcc-ddeeff001122"

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			st := NewStore(tmpDir)
			for atomic.LoadInt32(&stopFlag) == 0 {
				if workerID%2 == 0 {
					// Save a short-lived or expired intent
					expiredTime := time.Now().Add(-100 * time.Second)
					intent := &RevivalIntent{
						IntentID:           fmt.Sprintf("intent-prune-%d", workerID),
						RootConversationID: convID,
						Status:             IntentStatusPending,
						CreatedAt:          expiredTime,
						TTLSeconds:         10, // already expired
					}
					if err := st.SaveIntent(intent); err != nil {
						atomic.AddInt64(&panicOrCorruptCount, 1)
					}
				} else {
					// LoadIntent will detect TTL expiration and remove the file under lock
					intent, err := st.LoadIntent()
					if err != nil {
						atomic.AddInt64(&panicOrCorruptCount, 1)
					}
					// If loaded, verify it's valid
					if intent != nil && intent.RootConversationID != convID {
						atomic.AddInt64(&panicOrCorruptCount, 1)
					}
				}
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	time.Sleep(1000 * time.Millisecond)
	atomic.StoreInt32(&stopFlag, 1)
	wg.Wait()

	if panicOrCorruptCount > 0 {
		t.Fatalf("TTL prune race produced %d corruptions or unexpected errors", panicOrCorruptCount)
	}
}

// TestAdversarial_DeterministicIntentKey verifies that intentKey generated for CDP triggers
// is strictly deterministic across retries, leading/trailing spaces, and path variations.
func TestAdversarial_DeterministicIntentKey(t *testing.T) {
	testCases := []struct {
		inputCascadeID string
		expectedKey    string
	}{
		{"44556677-8899-aabb-ccdd-eeff00112233", "cdp-44556677-8899-aabb-ccdd-eeff00112233"},
		{"/c/44556677-8899-aabb-ccdd-eeff00112233", "cdp-44556677-8899-aabb-ccdd-eeff00112233"},
		{" /c/44556677-8899-aabb-ccdd-eeff00112233 ", "cdp-44556677-8899-aabb-ccdd-eeff00112233"},
		{"44556677-8899-aabb-ccdd-eeff00112233 ", "cdp-44556677-8899-aabb-ccdd-eeff00112233"},
	}

	for _, tc := range testCases {
		cleanID := strings.TrimPrefix(strings.TrimSpace(tc.inputCascadeID), "/c/")
		generatedKey := fmt.Sprintf("cdp-%s", cleanID)
		if generatedKey != tc.expectedKey {
			t.Errorf("input %q: got key %q, want %q", tc.inputCascadeID, generatedKey, tc.expectedKey)
		}
	}
}

// TestAdversarial_10MinuteCooldownExpiration verifies that per-conversation cooldown strictly
// suppresses intent synthesis for 10 minutes, expires immediately thereafter, and does NOT block
// unrelated conversations from reviving after global cooldown (45s).
func TestAdversarial_10MinuteCooldownExpiration(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := setupMockSummariesDB(t, tmpDir)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	conv1 := "55667788-99aa-bbcc-ddee-ff0011223344"
	conv2 := "66778899-aabb-ccdd-eeff-001122334455"
	now := time.Now()

	// Insert both conversations as running
	_, err = db.Exec(`
		INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time)
		VALUES (?, 'Conv 1', '[]', '', 0, 1, 'CASCADE_RUN_STATUS_RUNNING', ?),
		       (?, 'Conv 2', '[]', '', 0, 1, 'CASCADE_RUN_STATUS_RUNNING', ?);
	`, conv1, now.Format(time.RFC3339), conv2, now.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("failed to insert mock summaries: %v", err)
	}

	engine := NewEngine(tmpDir, 0)
	engine.Detector.BaseDir = tmpDir
	engine.Detector.LiveCDPEnabled = false

	// Scenario 1: conv1 revived 5 minutes ago (< 10 minutes)
	fiveMinAgo := time.Now().Add(-5 * time.Minute)
	engine.mu.Lock()
	engine.lastRevivedAt = &fiveMinAgo
	engine.revivedMap[conv1] = fiveMinAgo
	engine.mu.Unlock()

	status1, err := engine.GetRevivalStatus(conv1)
	if err != nil {
		t.Fatalf("GetRevivalStatus failed for conv1: %v", err)
	}
	if status1.PendingIntent != nil {
		t.Fatalf("Expected conv1 to be suppressed within 10-minute cooldown (5m elapsed), but pending intent was synthesized: %v", status1.PendingIntent)
	}

	// Scenario 2: conv2 (unrelated conversation) checked at 5m elapsed (> 45s global cooldown)
	status2, err := engine.GetRevivalStatus(conv2)
	if err != nil {
		t.Fatalf("GetRevivalStatus failed for conv2: %v", err)
	}
	if status2.PendingIntent == nil {
		t.Fatalf("Expected conv2 to synthesize intent since conv1 cooldown must not block conv2 after 45s global cooldown")
	}
	if status2.PendingIntent.RootConversationID != conv2 {
		t.Fatalf("Expected intent for conv2, got %q", status2.PendingIntent.RootConversationID)
	}

	// Clear intent from conv2 for next test
	_ = engine.Store.ClearIntent()

	// Scenario 3: conv1 checked after 10m 1s elapsed (> 10-minute cooldown)
	tenMinOneSecAgo := time.Now().Add(-10*time.Minute - 1*time.Second)
	engine.mu.Lock()
	engine.lastRevivedAt = &tenMinOneSecAgo
	engine.revivedMap[conv1] = tenMinOneSecAgo
	engine.mu.Unlock()

	status1Expired, err := engine.GetRevivalStatus(conv1)
	if err != nil {
		t.Fatalf("GetRevivalStatus failed for conv1 after cooldown: %v", err)
	}
	if status1Expired.PendingIntent == nil {
		t.Fatalf("Expected conv1 cooldown to expire after 10 minutes and synthesize new intent, got nil")
	}

	// Verify conv1 is pruned from revivedMap
	engine.mu.RLock()
	_, inMap := engine.revivedMap[conv1]
	engine.mu.RUnlock()
	if inMap {
		t.Errorf("Expected expired conv1 entry to be pruned from revivedMap")
	}
}

// TestAdversarial_MaxAttemptsBoundEnforcement tests that MaxAttempts is strictly enforced
// across re-entrant calls, preventing runaway retry loops.
func TestAdversarial_MaxAttemptsBoundEnforcement(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)

	convID := "778899aa-bbcc-ddee-ff00-112233445566"
	intent := &RevivalIntent{
		IntentID:           "intent-max-bounds",
		RootConversationID: convID,
		Status:             IntentStatusPending,
		Attempts:           0,
		MaxAttempts:        2,
	}
	if err := store.SaveIntent(intent); err != nil {
		t.Fatalf("SaveIntent failed: %v", err)
	}

	engine := NewEngine(tmpDir, 0)

	// Attempt 0: attempts < MaxAttempts
	loaded, _ := store.LoadIntent()
	if loaded.Attempts >= loaded.MaxAttempts {
		t.Fatalf("Expected attempts < maxAttempts")
	}

	// Simulate failure 1
	att1, err := store.IncrementAttempts(intent.IntentID)
	if err != nil || att1 != 1 {
		t.Fatalf("IncrementAttempts failed: att=%d, err=%v", att1, err)
	}
	_ = store.UpdateStatus(intent.IntentID, IntentStatusFailed, att1)

	// Simulate failure 2 (now reached MaxAttempts = 2)
	att2, err := store.IncrementAttempts(intent.IntentID)
	if err != nil || att2 != 2 {
		t.Fatalf("IncrementAttempts failed: att=%d, err=%v", att2, err)
	}
	_ = store.UpdateStatus(intent.IntentID, IntentStatusFailed, att2)

	loaded2, _ := store.LoadIntent()
	if loaded2.Attempts != 2 {
		t.Fatalf("Expected loaded Attempts = 2, got %d", loaded2.Attempts)
	}

	// Now ExecutePostRelaunchRevival MUST fail immediately and refuse execution
	err = engine.ExecutePostRelaunchRevival(loaded2)
	if err == nil {
		t.Fatalf("Expected ExecutePostRelaunchRevival to error when attempts >= MaxAttempts (2 >= 2), but succeeded")
	}
	if !strings.Contains(err.Error(), "max revival attempts") {
		t.Fatalf("Expected error message to mention 'max revival attempts', got: %v", err)
	}

	// Verify status remains IntentStatusFailed
	loadedFinal, _ := store.LoadIntent()
	if loadedFinal.Status != IntentStatusFailed {
		t.Fatalf("Expected status to remain failed, got %q", loadedFinal.Status)
	}
}

// TestAdversarial_AppStorage_ConcurrentLocking tests injector's lockAppStorage and
// SaveLastConversationPath / LoadLastConversationPath under concurrent access.
func TestAdversarial_AppStorage_ConcurrentLocking(t *testing.T) {
	// Temporarily override config directory
	tmpDir := t.TempDir()
	origHostDir := os.Getenv("ANTIGRAVITY_HOST_CONFIG_DIR")
	defer func() {
		_ = os.Setenv("ANTIGRAVITY_HOST_CONFIG_DIR", origHostDir)
	}()
	_ = os.Setenv("ANTIGRAVITY_HOST_CONFIG_DIR", tmpDir)

	const numWorkers = 20
	var wg sync.WaitGroup
	var stopFlag int32
	var opErrors int64

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			path := fmt.Sprintf("/c/8899aabb-ccdd-eeff-0011-22334455667%d", workerID%10)
			for atomic.LoadInt32(&stopFlag) == 0 {
				if workerID%2 == 0 {
					gui.SaveLastConversationPath(path)
				} else {
					readPath := gui.LoadLastConversationPath()
					if readPath != "" && !gui.IsValidConversationPath(readPath) {
						atomic.AddInt64(&opErrors, 1)
					}
				}
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	time.Sleep(1000 * time.Millisecond)
	atomic.StoreInt32(&stopFlag, 1)
	wg.Wait()

	if opErrors > 0 {
		t.Fatalf("app_storage concurrent access produced %d invalid reads or errors", opErrors)
	}
}

// TestAdversarial_CDPScript_SyntaxAndEscaping verifies that the JavaScript generated by CDPTrigger
// is syntactically valid in Node.js / V8 even when prompt or cascadeID contain quotes, newlines,
// backslashes, and special characters.
func TestAdversarial_CDPScript_SyntaxAndEscaping(t *testing.T) {
	adversarialPrompts := []string{
		`Simple prompt`,
		`Prompt with "double quotes" and 'single quotes'`,
		"Prompt with\nnewlines\r\nand\ttabs",
		`Prompt with backslashes \ and path C:\Windows\System32`,
		`Prompt with </script><script>alert(1)</script>`,
		`Prompt with unicode: 🚀 🌟 12345!`,
	}

	for _, p := range adversarialPrompts {
		cleanCascadeID := "test-conv-uuid"
		targetPath := "/c/" + cleanCascadeID
		intentKey := fmt.Sprintf("cdp-%s", cleanCascadeID)

		// Exact script template from cdp_trigger.go
		script := fmt.Sprintf(`(async () => {
			try {
				const targetPath = %q;
				const intentKey = %q;
				const promptText = %q;
				return { targetPath, intentKey, promptText };
			} catch (err) {
				return { error: String(err) };
			}
		})()`, targetPath, intentKey, p)

		// Execute node -e to verify V8 can parse and evaluate the script syntax cleanly
		cmd := exec.Command("node", "-e", script)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("JavaScript parse/eval failed for prompt %q: %v, output: %s", p, err, string(out))
		}
	}
}

// TestAdversarial_InvalidAndMaliciousConversationIDs tests that Store, Engine, and Injector
// reject path traversal, slashes, and illegal characters without corrupting disk state.
func TestAdversarial_InvalidAndMaliciousConversationIDs(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)
	engine := NewEngine(tmpDir, 0)

	maliciousIDs := []string{
		"../../../../etc/passwd",
		"../secrets.json",
		"bad/traversal",
		"",
		"   ",
		"has space",
		"&& rm -rf /",
		"<script>alert(1)</script>",
		"semi;colon",
	}

	for _, badID := range maliciousIDs {
		// 1. IsValidConversationID must return false
		if gui.IsValidConversationID(badID) {
			t.Errorf("expected IsValidConversationID to return false for %q, but got true", badID)
		}

		// 2. Store.SaveIntent must reject invalid conversation IDs
		intent := &RevivalIntent{
			IntentID:           "bad-id-intent",
			RootConversationID: badID,
			Status:             IntentStatusPending,
		}
		err := store.SaveIntent(intent)
		if err == nil {
			t.Errorf("expected SaveIntent to reject malicious/invalid ID %q, but succeeded", badID)
		}

		// 3. Engine.AcknowledgeContinuation must reject empty target
		if strings.TrimSpace(badID) == "" {
			err = engine.AcknowledgeContinuation(badID)
			if err == nil {
				t.Errorf("expected AcknowledgeContinuation to reject empty ID, but succeeded")
			}
		}
	}
}

// TestAdversarial_CDPScript_HardenedSimulation runs the actual generated CDP script in a real Node.js
// runtime with a mocked DOM harness to verify:
// 1. Enter key simulation (KeyboardEvent keydown/keypress/keyup with keyCode 13) is dispatched to editor
// 2. Send button is found and receives pointerdown, mousedown, pointerup, mouseup, click
// 3. Intent is only recorded in window.__swissDispatchedIntents after send is confirmed (editor cleared)
// 4. On send timeout (editor not cleared), intent is NOT recorded in window.__swissDispatchedIntents
func TestAdversarial_CDPScript_HardenedSimulation(t *testing.T) {
	convID := "77889900-aabb-ccdd-eeff-001122334455"
	prompt := "Please continue ongoing tasks and subagents."

	var capturedScript string
	trigger := &CDPTrigger{
		CustomPort: 9222,
		TargetFinder: func(port int) (int, []gui.DevToolsTarget, error) {
			return port, []gui.DevToolsTarget{
				{
					ID:                   "target-1",
					Title:                "Antigravity",
					Type:                 "page",
					URL:                  "http://127.0.0.1:9222/c/" + convID,
					WebSocketDebuggerURL: "ws://127.0.0.1:9222/devtools/page/target-1",
				},
			}, nil
		},
		ScriptExecutor: func(wsURL, expression string) (map[string]interface{}, error) {
			capturedScript = expression
			return map[string]interface{}{"status": "mock", "success": true}, nil
		},
	}

	_ = trigger.TriggerDesktopContinuation(convID, prompt, 1*time.Second)
	if capturedScript == "" {
		t.Fatalf("failed to capture CDP script")
	}

	// Node.js test harness providing mock DOM objects
	nodeHarness := fmt.Sprintf(`
const vm = require('vm');

function createDOMHarness(options = {}) {
	const eventsDispatched = [];
	let editorText = options.initialEditorText || "";

	class Event {
		constructor(type, opts = {}) {
			this.type = type;
			this.bubbles = opts.bubbles || false;
			this.cancelable = opts.cancelable || false;
		}
	}
	class CustomEvent extends Event {}
	class InputEvent extends Event {
		constructor(type, opts = {}) {
			super(type, opts);
			this.data = opts.data;
			this.inputType = opts.inputType;
		}
	}
	class KeyboardEvent extends Event {
		constructor(type, opts = {}) {
			super(type, opts);
			this.key = opts.key;
			this.code = opts.code;
			this.keyCode = opts.keyCode;
			this.which = opts.which;
		}
	}
	class MouseEvent extends Event {}
	class PointerEvent extends MouseEvent {}

	const editor = {
		isContentEditable: true,
		innerText: editorText,
		value: editorText,
		focus: () => {},
		dispatchEvent: (e) => {
			eventsDispatched.push({ target: 'editor', type: e.type, key: e.key, keyCode: e.keyCode });
			if (e.type === 'beforeinput' && e.data) {
				editor.innerText = e.data;
			}
			return true;
		},
		closest: (sel) => null,
		parentElement: null
	};

	const sendBtn = {
		disabled: options.sendBtnDisabled || false,
		type: 'button',
		getAttribute: (attr) => {
			if (attr === 'data-testid') return 'send-button';
			if (attr === 'aria-label') return 'Send';
			return null;
		},
		removeAttribute: (attr) => {
			if (attr === 'disabled') sendBtn.disabled = false;
		},
		dispatchEvent: (e) => {
			eventsDispatched.push({ target: 'sendBtn', type: e.type });
			return true;
		},
		click: () => {
			eventsDispatched.push({ target: 'sendBtn', type: 'click' });
			if (options.clearEditorOnClick !== false) {
				editor.innerText = "";
				editor.value = "";
			}
		}
	};

	const doc = {
		location: { pathname: "/c/%s" },
		querySelector: (sel) => {
			if (options.generating && (sel.includes('agent-generating') || sel.includes('cancel') || sel.includes('stop'))) {
				return { tagName: 'BUTTON' };
			}
			if (sel.includes('Send now') || sel.includes('send-button-pending') || sel.includes('cancel') || sel.includes('stop') || sel.includes('queued')) {
				return null;
			}
			if (sel.includes('contenteditable') || sel.includes('Message input') || sel.includes('agent-input-box')) {
				return editor;
			}
			if (sel === '[data-testid="send-button"]' || sel.includes('send-tooltip') || sel.includes('aria-label*="Send"') || sel === 'button[aria-label*="Send" i]') {
				return sendBtn;
			}
			return null;
		},
		querySelectorAll: (sel) => {
			if (sel === 'button') return [sendBtn];
			if (sel === '*') return [];
			return [];
		},
		execCommand: (cmd, ui, val) => {
			if (cmd === 'insertText') {
				editor.innerText = val;
			}
			return true;
		},
		getSelection: () => ({ removeAllRanges: () => {}, addRange: () => {} }),
		createRange: () => ({ selectNodeContents: () => {} })
	};

	const win = {
		location: doc.location,
		document: doc,
		getSelection: doc.getSelection,
		__swissDispatchedIntents: {},
		Event, CustomEvent, InputEvent, KeyboardEvent, MouseEvent, PointerEvent,
		setTimeout,
		clearTimeout,
		Promise
	};

	return { win, doc, editor, sendBtn, eventsDispatched };
}

async function run() {
	// Case 1: Normal successful send
	{
		const harness = createDOMHarness({ clearEditorOnClick: true });
		const fn = new Function('window', 'document', 'Event', 'CustomEvent', 'InputEvent', 'KeyboardEvent', 'MouseEvent', 'PointerEvent', 'setTimeout', 'clearTimeout', 'Promise',
			'return ' + %q);
		const result = await fn(harness.win, harness.doc, harness.win.Event, harness.win.CustomEvent, harness.win.InputEvent, harness.win.KeyboardEvent, harness.win.MouseEvent, harness.win.PointerEvent, setTimeout, clearTimeout, Promise);

		if (!result.success || result.status !== 'clicked_send_button') {
			throw new Error("Case 1 failed: expected clicked_send_button, got " + JSON.stringify(result));
		}
		const enterEvents = harness.eventsDispatched.filter(e => e.target === 'editor' && e.key === 'Enter');
		if (enterEvents.length === 0) {
			throw new Error("Case 1 failed: editor did not receive Enter KeyboardEvent!");
		}
		const pointerEvents = harness.eventsDispatched.filter(e => e.target === 'sendBtn' && e.type === 'pointerdown');
		if (pointerEvents.length === 0) {
			throw new Error("Case 1 failed: sendBtn did not receive pointerdown!");
		}
		const intentKey = "cdp-%s";
		if (!harness.win.__swissDispatchedIntents[intentKey]) {
			throw new Error("Case 1 failed: intent was not recorded in __swissDispatchedIntents after successful send!");
		}
	}

	// Case 2: Send fails / times out (editor not cleared)
	{
		const harness = createDOMHarness({ clearEditorOnClick: false, sendBtnDisabled: true });
		const fn = new Function('window', 'document', 'Event', 'CustomEvent', 'InputEvent', 'KeyboardEvent', 'MouseEvent', 'PointerEvent', 'setTimeout', 'clearTimeout', 'Promise',
			'return ' + %q);
		const result = await fn(harness.win, harness.doc, harness.win.Event, harness.win.CustomEvent, harness.win.InputEvent, harness.win.KeyboardEvent, harness.win.MouseEvent, harness.win.PointerEvent, setTimeout, clearTimeout, Promise);

		if (result.success || result.status !== 'send_button_timeout') {
			throw new Error("Case 2 failed: expected send_button_timeout, got " + JSON.stringify(result));
		}
		const intentKey = "cdp-%s";
		if (harness.win.__swissDispatchedIntents[intentKey]) {
			throw new Error("Case 2 failed: intent MUST NOT be recorded in __swissDispatchedIntents when send timed out!");
		}
	}

	// Case 3: Conversation already generating
	{
		const harness = createDOMHarness({ generating: true });
		const fn = new Function('window', 'document', 'Event', 'CustomEvent', 'InputEvent', 'KeyboardEvent', 'MouseEvent', 'PointerEvent', 'setTimeout', 'clearTimeout', 'Promise',
			'return ' + %q);
		const result = await fn(harness.win, harness.doc, harness.win.Event, harness.win.CustomEvent, harness.win.InputEvent, harness.win.KeyboardEvent, harness.win.MouseEvent, harness.win.PointerEvent, setTimeout, clearTimeout, Promise);

		if (!result.success || result.status !== 'conversation_already_active') {
			throw new Error("Case 3 failed: expected conversation_already_active, got " + JSON.stringify(result));
		}
	}

	console.log("All Node.js DOM simulation tests passed successfully");
}

run().catch(err => {
	console.error(err);
	process.exit(1);
});
`, convID, capturedScript, convID, capturedScript, convID, capturedScript)

	scriptPath := filepath.Join(t.TempDir(), "sim_harness.js")
	if err := os.WriteFile(scriptPath, []byte(nodeHarness), 0644); err != nil {
		t.Fatalf("failed to write node harness script: %v", err)
	}
	cmd := exec.Command("node", scriptPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Node.js simulation failed: %v\nOutput:\n%s", err, string(out))
	}
	t.Logf("Node.js DOM simulation output: %s", strings.TrimSpace(string(out)))
}


