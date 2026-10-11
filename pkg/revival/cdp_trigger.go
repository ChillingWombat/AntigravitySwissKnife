package revival

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
)

// ScriptExecutorFunc defines the signature for executing JS in a CDP target.
type ScriptExecutorFunc func(wsURL string, expression string) (map[string]interface{}, error)

// TargetFinderFunc defines the signature for finding the DevTools port and targets.
type TargetFinderFunc func(customPort int) (int, []gui.DevToolsTarget, error)

// CDPTrigger coordinates connecting to the Desktop IDE via DevTools and triggering prompt submission.
type CDPTrigger struct {
	CustomPort     int
	Injector       *gui.Injector
	ScriptExecutor ScriptExecutorFunc
	TargetFinder   TargetFinderFunc
}

// NewCDPTrigger initializes a new CDPTrigger.
func NewCDPTrigger(customPort int) *CDPTrigger {
	inj := gui.NewInjector(customPort)
	return &CDPTrigger{
		CustomPort: customPort,
		Injector:   inj,
		ScriptExecutor: func(wsURL, expression string) (map[string]interface{}, error) {
			return inj.ExecuteScript(wsURL, expression)
		},
		TargetFinder: func(port int) (int, []gui.DevToolsTarget, error) {
			if gui.IsTestExecution() || os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") == "1" || core.IsRunningTests() {
				return 0, nil, fmt.Errorf("live cdp connection disabled during test execution")
			}
			activePort, err := inj.FindDevToolsPort()
			if err != nil {
				return 0, nil, err
			}
			targets, err := inj.GetPageTargets(activePort)
			if err != nil {
				return activePort, nil, err
			}
			return activePort, targets, nil
		},
	}
}

// TriggerDesktopContinuation connects to the IDE via CDP, navigates to the conversation if needed,
// and submits the continuation prompt to Lexical editor.
func (c *CDPTrigger) TriggerDesktopContinuation(cascadeID string, prompt string, timeout time.Duration) error {
	if (gui.IsTestExecution() || os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") == "1" || core.IsRunningTests()) && c.CustomPort == 0 {
		return nil
	}
	if cascadeID == "" {
		return fmt.Errorf("cascadeID cannot be empty")
	}
	if prompt == "" {
		prompt = DefaultTriggerPrompt
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	targetPath := "/c/" + strings.TrimPrefix(cascadeID, "/c/")
	deadline := time.Now().Add(timeout)
	intentKey := fmt.Sprintf("cdp-%s-%d", cascadeID, time.Now().UnixMilli())

	script := fmt.Sprintf(`(async () => {
		try {
			const targetPath = %q;
			const curPath = window.location.pathname || "/";

			// 1. Ensure navigation to target conversation
			if (!curPath.startsWith(targetPath)) {
				const convPart = targetPath.replace(/^\/c\//, "");
				const navLink = document.querySelector('a[href*="' + convPart + '"]');
				if (navLink) {
					navLink.click();
					return { status: "navigating_via_link", ready: false, success: false };
				}
				// If target conversation is not active and no link found in DOM, abort to avoid disturbing user view
				return { status: "mismatched_target_conversation", ready: false, success: false };
			}

			// 1.5. Prevent duplicate trigger loops for the exact same intent
			window.__swissDispatchedIntents = window.__swissDispatchedIntents || {};
			const intentKey = %q;
			if (window.__swissDispatchedIntents[intentKey]) {
				return { status: "already_sent", ready: true, success: true };
			}

			// 2. Locate editor element
			const editor = document.querySelector('[data-testid="agent-input-box"] [contenteditable="true"]') ||
						   document.querySelector('[data-lexical-editor="true"][contenteditable="true"]') ||
						   document.querySelector('.lexical-container [contenteditable="true"]') ||
						   document.querySelector('[data-testid="chat-input-textarea"]') ||
						   document.querySelector('textarea[placeholder*="Ask"]');
			if (!editor) {
				return { status: "waiting_for_editor", ready: false, success: false };
			}

			// 3. Check if agent is already actively generating or messages are queued
			const isBusyOrQueued = Array.from(document.querySelectorAll("*")).some(el =>
				el.children.length === 0 && (
					el.textContent.includes("Queued Messages") ||
					el.textContent.includes("Sends after agent finishes")
				)
			);
			const cancelBtn = document.querySelector('[data-tooltip-id="input-send-button-cancel-tooltip"]');
			const pendingSend = document.querySelector('[data-testid="send-button-pending"]');
			const generating = isBusyOrQueued || cancelBtn || pendingSend ||
							   document.querySelector('[data-testid="agent-generating"]') ||
							   document.querySelector('[data-testid="stop-button"]');
			if (generating) {
				// Wait for agent to finish generating before injecting continuation prompt
				return { status: "waiting_for_agent_idle", ready: false, success: false };
			}

			// 4. Check if interactive questionnaire continue button is present
			const interactBtn = document.querySelector('[data-testid="interaction-continue-button"]');
			if (interactBtn && !interactBtn.disabled) {
				window.__swissDispatchedIntents[intentKey] = Date.now();
				interactBtn.click();
				return { status: "clicked_interaction_continue", ready: true, success: true };
			}

			// 5. Inject continuation prompt into Lexical editor
			const promptText = %q;
			if (editor.isContentEditable) {
				const curText = (editor.innerText || "").trim();
				// If editor has an active user draft that is not the continuation prompt, protect it!
				if (curText.length > 0 && !curText.includes(promptText)) {
					return { status: "user_draft_in_progress", ready: false, success: false };
				}
				if (!curText.includes(promptText)) {
					editor.focus();
					const sel = window.getSelection();
					if (sel) {
						const range = document.createRange();
						range.selectNodeContents(editor);
						sel.removeAllRanges();
						sel.addRange(range);
					}
					document.execCommand("selectAll", false, null);
					document.execCommand("insertText", false, promptText);
					const hasInserted = (editor.innerText || "").includes(promptText);
					if (!hasInserted) {
						try {
							editor.dispatchEvent(new InputEvent("beforeinput", {
								inputType: "insertText",
								data: promptText,
								bubbles: true,
								cancelable: true
							}));
						} catch (_) {}
					}
					editor.dispatchEvent(new Event("input", { bubbles: true, composed: true }));
				}
			} else {
				const curVal = (editor.value || "").trim();
				if (curVal.length > 0 && !curVal.includes(promptText)) {
					return { status: "user_draft_in_progress", ready: false, success: false };
				}
				if (!(editor.value || "").includes(promptText)) {
					editor.focus();
					editor.value = promptText;
					editor.dispatchEvent(new Event("input", { bubbles: true }));
					editor.dispatchEvent(new Event("change", { bubbles: true }));
				}
			}

			// 6. Wait/poll until send button is enabled (Lexical enables it async after processing input)
			for (let i = 0; i < 35; i++) {
				const sendBtn = document.querySelector('[data-testid="send-button"]') ||
								document.querySelector('[data-tooltip-id*="send-tooltip"]') ||
								document.querySelector('button[aria-label*="Send" i]') ||
								document.querySelector('.chat-input-toolbar button:last-child');
				if (sendBtn && !sendBtn.disabled && !sendBtn.matches('[data-tooltip-id="input-send-button-cancel-tooltip"]')) {
					window.__swissDispatchedIntents[intentKey] = Date.now();
					sendBtn.click();
					return { status: "clicked_send_button", ready: true, success: true };
				}
				await new Promise(r => setTimeout(r, 100));
			}

			// 7. If send button is present, enable and click directly
			const finalBtn = document.querySelector('[data-testid="send-button"]') ||
							document.querySelector('[data-tooltip-id*="send-tooltip"]') ||
							document.querySelector('button[aria-label*="Send" i]') ||
							document.querySelector('.chat-input-toolbar button:last-child');
			if (finalBtn && !finalBtn.matches('[data-tooltip-id="input-send-button-cancel-tooltip"]')) {
				finalBtn.disabled = false;
				window.__swissDispatchedIntents[intentKey] = Date.now();
				finalBtn.click();
				return { status: "clicked_send_button_forced", ready: true, success: true };
			}

			return { status: "send_button_timeout", ready: true, success: false };
		} catch (err) {
			return { status: "error", error: String(err), ready: false, success: false };
		}
	})()`, targetPath, intentKey, prompt)

	for time.Now().Before(deadline) {
		_, targets, err := c.TargetFinder(c.CustomPort)
		if err == nil && len(targets) > 0 {
			for _, target := range targets {
				if target.WebSocketDebuggerURL == "" {
					continue
				}
				// Filter to local Antigravity application pages
				if !strings.HasPrefix(target.URL, "https://127.0.0.1:") &&
					!strings.HasPrefix(target.URL, "http://127.0.0.1:") &&
					!strings.HasPrefix(target.URL, "https://localhost:") &&
					!strings.HasPrefix(target.URL, "http://localhost:") {
					continue
				}

				res, err := c.ScriptExecutor(target.WebSocketDebuggerURL, script)
				if err == nil && res != nil {
					evalRes := res
					if v, ok := res["value"].(map[string]interface{}); ok {
						evalRes = v
					}
					if success, ok := evalRes["success"].(bool); ok && success {
						return nil
					}
					if status, ok := evalRes["status"].(string); ok {
						if status == "user_draft_in_progress" || status == "mismatched_target_conversation" {
							return nil
						}
					}
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for conversation view and editor readiness to trigger revival")
}
