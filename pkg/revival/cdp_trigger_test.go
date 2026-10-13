package revival

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
)

// TestCDPTrigger_ScriptContent_HardenedDispatch verifies that the script generated
// by TriggerDesktopContinuation incorporates all prompt dispatch hardening mechanisms:
// 1. Enter key simulation (keydown, keypress, keyup with keyCode 13)
// 2. Form submission fallback
// 3. Robust button search (parent container traversal, SVG, type=submit, aria-label)
// 4. Pointer and Mouse event dispatch sequence (pointerdown, mousedown, pointerup, mouseup, click)
// 5. Verification that intent is NEVER recorded before send is confirmed
func TestCDPTrigger_ScriptContent_HardenedDispatch(t *testing.T) {
	convID := "99887766-5544-3322-1100-aabbccddeeff"
	prompt := "Please continue ongoing tasks and subagents."

	var capturedScript string
	trigger := &CDPTrigger{
		CustomPort: 9222,
		TargetFinder: func(port int) (int, []gui.DevToolsTarget, error) {
			return port, []gui.DevToolsTarget{
				{
					ID:                   "target-1",
					Title:                "Antigravity Workbench",
					Type:                 "page",
					URL:                  "http://127.0.0.1:9222/c/" + convID,
					WebSocketDebuggerURL: "ws://127.0.0.1:9222/devtools/page/target-1",
				},
			}, nil
		},
		ScriptExecutor: func(wsURL, expression string) (map[string]interface{}, error) {
			capturedScript = expression
			return map[string]interface{}{
				"status":  "clicked_send_button",
				"ready":   true,
				"success": true,
			}, nil
		},
	}

	err := trigger.TriggerDesktopContinuation(convID, prompt, 2*time.Second)
	if err != nil {
		t.Fatalf("TriggerDesktopContinuation failed: %v", err)
	}

	if capturedScript == "" {
		t.Fatalf("expected script to be captured, but was empty")
	}

	// 1. Must include Enter key simulation with KeyboardEvent
	requiredEnterSnippets := []string{
		"dispatchEnter",
		`key: "Enter"`,
		`code: "Enter"`,
		"keyCode: 13",
		"KeyboardEvent",
	}
	for _, snip := range requiredEnterSnippets {
		if !strings.Contains(capturedScript, snip) {
			t.Errorf("expected script to contain Enter key simulation snippet %q", snip)
		}
	}

	// 2. Must include form submit fallback
	if !strings.Contains(capturedScript, "submitForm") || !strings.Contains(capturedScript, "requestSubmit") {
		t.Errorf("expected script to contain form submission fallback (submitForm / requestSubmit)")
	}

	// 3. Must include robust button search traversing containers and checking type=submit
	requiredButtonSnippets := []string{
		"findSendButton",
		"button[type=\"submit\"]",
		"triggerClick",
		"PointerEvent",
		"pointerdown",
		"pointerup",
		"mousedown",
		"mouseup",
	}
	for _, snip := range requiredButtonSnippets {
		if !strings.Contains(capturedScript, snip) {
			t.Errorf("expected script to contain button discovery/click snippet %q", snip)
		}
	}

	// 4. Must verify editor is cleared or busy state before marking intent
	if !strings.Contains(capturedScript, "isEditorCleared") {
		t.Errorf("expected script to verify isEditorCleared")
	}
	if !strings.Contains(capturedScript, "isBusy") {
		t.Errorf("expected script to verify isBusy")
	}

	// 5. Must NOT prematurely record window.__swissDispatchedIntents before the dispatch loop
	// Check that before the dispatch loop ("for (let i = 0; i < 25; i++)"),
	// __swissDispatchedIntents is only recorded for early exits (already_active, interaction_continue, user_draft)
	preLoopIndex := strings.Index(capturedScript, "for (let i = 0; i < 25; i++)")
	if preLoopIndex == -1 {
		t.Fatalf("could not find dispatch loop in captured script")
	}
	preLoopCode := capturedScript[:preLoopIndex]
	// Count occurrences of window.__swissDispatchedIntents[intentKey] = Date.now() before the loop
	// Valid occurrences:
	// - if (hasQueued || hasRunningSubagents || isGenerating) -> conversation_already_active
	// - if (interactBtn && !interactBtn.disabled) -> clicked_interaction_continue
	// - if (curText.length > 0 && !curText.includes(promptText)) -> user_draft_in_progress
	// - if (curVal.length > 0 && !curVal.includes(promptText)) -> user_draft_in_progress
	// INVALID occurrence would be unconditionally setting it right after "const promptText = %q;"
	unconditionalMarker := fmt.Sprintf("const promptText = %q;\n\t\t\twindow.__swissDispatchedIntents[intentKey] = Date.now();", prompt)
	if strings.Contains(preLoopCode, unconditionalMarker) {
		t.Errorf("SECURITY REGRESSION: detected premature unconditional window.__swissDispatchedIntents recording before send is confirmed!")
	}
}

// TestCDPTrigger_RetryOnSendTimeout verifies that when an initial send attempt times out
// (e.g. editor is still holding text and send button didn't trigger immediately),
// TriggerDesktopContinuation does NOT prematurely terminate with success or set the intent,
// but retries until send succeeds or deadline expires.
func TestCDPTrigger_RetryOnSendTimeout(t *testing.T) {
	convID := "11223344-5566-7788-9900-aabbccddeeff"
	prompt := "Please continue"

	var callCount int32
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
			count := atomic.AddInt32(&callCount, 1)
			if count == 1 {
				// First attempt fails with send_button_timeout (editor still has text)
				return map[string]interface{}{
					"status":  "send_button_timeout",
					"ready":   true,
					"success": false,
				}, nil
			}
			// Second attempt succeeds
			return map[string]interface{}{
				"status":  "clicked_send_button",
				"ready":   true,
				"success": true,
			}, nil
		},
	}

	err := trigger.TriggerDesktopContinuation(convID, prompt, 3*time.Second)
	if err != nil {
		t.Fatalf("expected retry to succeed on 2nd attempt, got error: %v", err)
	}

	if atomic.LoadInt32(&callCount) < 2 {
		t.Errorf("expected at least 2 attempts (retry after timeout), got %d", callCount)
	}
}

// TestCDPTrigger_UserDraftProtection verifies that if user is typing their own draft,
// the trigger halts cleanly without overwriting user text.
func TestCDPTrigger_UserDraftProtection(t *testing.T) {
	convID := "33445566-7788-9900-1122-aabbccddeeff"
	prompt := "Please continue"

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
			return map[string]interface{}{
				"status":  "user_draft_in_progress",
				"ready":   true,
				"success": true,
			}, nil
		},
	}

	err := trigger.TriggerDesktopContinuation(convID, prompt, 2*time.Second)
	if err != nil {
		t.Fatalf("expected user_draft_in_progress to complete cleanly, got error: %v", err)
	}
}
