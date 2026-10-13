package revival

import (
	"fmt"
	"os"
	"sort"
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

	cleanCascadeID := strings.TrimPrefix(strings.TrimSpace(cascadeID), "/c/")
	targetPath := "/c/" + cleanCascadeID
	deadline := time.Now().Add(timeout)
	// Deterministic dedupe key per cascade intent to prevent duplicate prompts across retries
	intentKey := fmt.Sprintf("cdp-%s", cleanCascadeID)

	script := fmt.Sprintf(`(async () => {
		try {
			const targetPath = %q;
			const curPath = window.location.pathname || "/";

			// 1. Ensure navigation to target conversation
			if (!curPath.startsWith(targetPath)) {
				const convPart = targetPath.replace(/^\/c\//, "");
				const navLink = document.querySelector('a[href*="' + convPart + '"]');
				if (navLink && typeof navLink.click === "function") {
					navLink.click();
					return { status: "navigating_via_link", ready: false, success: false };
				}
				if (window.__TSR_ROUTER__ && typeof window.__TSR_ROUTER__.navigate === "function") {
					try {
						window.__TSR_ROUTER__.navigate({ to: targetPath });
						return { status: "navigating_via_tsr", ready: false, success: false };
					} catch (_) {}
				}
				// If currently on splash/root or onboarding or non-conversation route, navigate directly
				if (curPath === "/" || curPath.startsWith("/onboarding") || !curPath.startsWith("/c/")) {
					if (window.location && typeof window.location.assign === "function") {
						window.location.assign(targetPath);
					} else if (window.location) {
						window.location.href = targetPath;
					}
					return { status: "navigating_via_route", ready: false, success: false };
				}
				// If user is already on a different active conversation /c/<other>, do not hijack
				return { status: "mismatched_target_conversation", ready: false, success: false };
			}

			// 1.5. Prevent duplicate trigger loops for the exact same intent
			window.__swissDispatchedIntents = window.__swissDispatchedIntents || {};
			const intentKey = %q;
			if (window.__swissDispatchedIntents[intentKey]) {
				return { status: "already_sent", ready: true, success: true };
			}

			// 2. Locate editor element
			const editor = document.querySelector('div[aria-label="Message input"][contenteditable="true"]') ||
						   document.querySelector('[data-testid="agent-input-box"] [contenteditable="true"]') ||
						   document.querySelector('[data-lexical-editor="true"][contenteditable="true"]') ||
						   document.querySelector('.lexical-container [contenteditable="true"]') ||
						   document.querySelector('[data-testid="chat-input-textarea"]') ||
						   document.querySelector('textarea[placeholder*="Ask"]');
			if (!editor) {
				return { status: "waiting_for_editor", ready: false, success: false };
			}

			// 3. Check if agent is already actively generating, running subagents, or has queued messages
			const hasQueued = Boolean(
				document.querySelector('[data-testid="queued-messages-expand-button"]') ||
				document.querySelector('button[aria-label*="Collapse queued messages" i]') ||
				document.querySelector('button[aria-label*="Send now:" i]') ||
				Array.from(document.querySelectorAll("*")).some(el =>
					el.children.length === 0 && (
						el.textContent.includes("Queued Messages") ||
						el.textContent.includes("Sends after agent finishes")
					)
				)
			);
			const hasRunningSubagents = Boolean(
				document.querySelector('button[aria-label*="Stop Subagent" i]') ||
				document.querySelector('button[data-tooltip-id^="stop-subagent-"]') ||
				Array.from(document.querySelectorAll("*")).some(el =>
					el.children.length === 0 && /(\d+)\s+subagents?\s+running/i.test(el.textContent)
				)
			);
			const isGenerating = Boolean(
				document.querySelector('button[aria-label*="Cancel" i]') ||
				document.querySelector('button[aria-label*="Stop execution" i]') ||
				document.querySelector('button[aria-label*="Stop" i]') ||
				document.querySelector('[data-tooltip-id="input-send-button-cancel-tooltip"]') ||
				document.querySelector('[data-tooltip-id*="cancel-tooltip"]') ||
				document.querySelector('[data-tooltip-id*="stop-tooltip"]') ||
				document.querySelector('[data-testid*="cancel"]') ||
				document.querySelector('[data-testid*="stop"]') ||
				document.querySelector('[data-testid="send-button-pending"]') ||
				document.querySelector('[data-testid="agent-generating"]')
			);

			if (hasQueued || hasRunningSubagents || isGenerating) {
				// The conversation is ALREADY active and executing; continuation is fulfilled
				window.__swissDispatchedIntents[intentKey] = Date.now();
				return { status: "conversation_already_active", ready: true, success: true };
			}

			// 4. Check if interactive questionnaire continue button is present
			const interactBtn = document.querySelector('[data-testid="interaction-continue-button"]');
			if (interactBtn && !interactBtn.disabled) {
				window.__swissDispatchedIntents[intentKey] = Date.now();
				try {
					interactBtn.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, cancelable: true }));
					interactBtn.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true }));
					interactBtn.dispatchEvent(new PointerEvent('pointerup', { bubbles: true, cancelable: true }));
					interactBtn.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, cancelable: true }));
				} catch (_) {}
				interactBtn.click();
				return { status: "clicked_interaction_continue", ready: true, success: true };
			}

			// 5. Inject continuation prompt into Lexical editor
			const promptText = %q;

			if (editor.isContentEditable) {
				const curText = (editor.innerText || "").trim();
				// If editor has an active user draft, protect user draft and consider conversation active!
				if (curText.length > 0 && !curText.includes(promptText)) {
					window.__swissDispatchedIntents[intentKey] = Date.now();
					return { status: "user_draft_in_progress", ready: true, success: true };
				}
				if (!curText.includes(promptText)) {
					editor.focus();
					const lex = editor.__lexicalEditor;
					if (lex && typeof lex.update === "function" && lex._nodes) {
						lex.update(() => {
							const ParagraphNode = lex._nodes.get('paragraph')?.klass;
							const TextNode = lex._nodes.get('text')?.klass;
							const root = lex._editorState?._nodeMap?.get('root');
							if (ParagraphNode && TextNode && root) {
								root.clear();
								const p = new ParagraphNode();
								const t = new TextNode(promptText);
								p.append(t);
								root.append(p);
							}
						});
					} else {
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
					}
					try {
						editor.dispatchEvent(new InputEvent("input", {
							inputType: "insertText",
							data: promptText,
							bubbles: true,
							composed: true
						}));
					} catch (_) {
						editor.dispatchEvent(new Event("input", { bubbles: true, composed: true }));
					}
					editor.dispatchEvent(new Event("change", { bubbles: true }));
				}
			} else {
				const curVal = (editor.value || "").trim();
				if (curVal.length > 0 && !curVal.includes(promptText)) {
					window.__swissDispatchedIntents[intentKey] = Date.now();
					return { status: "user_draft_in_progress", ready: true, success: true };
				}
				if (!(editor.value || "").includes(promptText)) {
					editor.focus();
					editor.value = promptText;
					editor.dispatchEvent(new Event("input", { bubbles: true }));
					editor.dispatchEvent(new Event("change", { bubbles: true }));
				}
			}

			await new Promise(r => setTimeout(r, 60));

			const isBusy = () => Boolean(
				document.querySelector('button[aria-label*="Cancel" i]') ||
				document.querySelector('button[aria-label*="Stop execution" i]') ||
				document.querySelector('button[aria-label*="Stop Subagent" i]') ||
				document.querySelector('button[aria-label*="Stop" i]') ||
				document.querySelector('[data-tooltip-id="input-send-button-cancel-tooltip"]') ||
				document.querySelector('[data-tooltip-id*="cancel-tooltip"]') ||
				document.querySelector('[data-tooltip-id*="stop-tooltip"]') ||
				document.querySelector('[data-testid*="cancel"]') ||
				document.querySelector('[data-testid*="stop"]') ||
				document.querySelector('[data-testid="queued-messages-expand-button"]') ||
				document.querySelector('[data-testid="send-button-pending"]') ||
				document.querySelector('[data-testid="agent-generating"]') ||
				Array.from(document.querySelectorAll("*")).some(el =>
					el.children.length === 0 && (
						/(\d+)\s+subagents?\s+running/i.test(el.textContent) ||
						el.textContent.includes("Queued Messages") ||
						el.textContent.includes("Sends after agent finishes")
					)
				)
			);

			const isEditorCleared = () => {
				const cur = (editor.isContentEditable ? (editor.innerText || "") : (editor.value || "")).trim();
				return cur.length === 0 || !cur.includes(promptText);
			};

			const dispatchEnter = () => {
				try {
					editor.focus();
					const evOpts = { key: "Enter", code: "Enter", keyCode: 13, which: 13, bubbles: true, cancelable: true, composed: true };
					editor.dispatchEvent(new KeyboardEvent("keydown", evOpts));
					editor.dispatchEvent(new KeyboardEvent("keypress", evOpts));
					editor.dispatchEvent(new KeyboardEvent("keyup", evOpts));
				} catch (_) {}
			};

			const submitForm = () => {
				try {
					const form = editor.closest('form');
					if (form) {
						if (typeof form.requestSubmit === "function") {
							form.requestSubmit();
							return true;
						} else {
							form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
							return true;
						}
					}
				} catch (_) {}
				return false;
			};

			const isCancelOrStop = (btn) => {
				if (!btn) return true;
				const aria = (btn.getAttribute('aria-label') || '').toLowerCase();
				const testid = (btn.getAttribute('data-testid') || '').toLowerCase();
				const tooltip = (btn.getAttribute('data-tooltip-id') || '').toLowerCase();
				return aria.includes('cancel') || aria.includes('stop') ||
					testid.includes('cancel') || testid.includes('stop') ||
					tooltip.includes('cancel') || tooltip.includes('stop');
			};

			const findSendButton = () => {
				// 1. Direct standard selectors
				const direct = document.querySelector('[data-testid="send-button"]') ||
					document.querySelector('[data-tooltip-id="input-send-button-send-tooltip"]') ||
					document.querySelector('[data-tooltip-id*="send-tooltip"]') ||
					document.querySelector('button[aria-label*="Send" i]') ||
					document.querySelector('button[aria-label*="Submit" i]') ||
					document.querySelector('.chat-input-toolbar button:last-child');
				if (direct && !isCancelOrStop(direct)) return direct;

				// 2. Search within editor's form or container
				const container = editor.closest('form') ||
					editor.closest('.lexical-container')?.parentElement ||
					editor.closest('[data-testid*="input"]')?.parentElement ||
					editor.parentElement?.parentElement;
				if (container) {
					const btnInContainer = container.querySelector('button[type="submit"]') ||
						container.querySelector('button[aria-label*="Send" i]') ||
						container.querySelector('[data-testid*="send"]');
					if (btnInContainer && !isCancelOrStop(btnInContainer)) return btnInContainer;
				}

				// 3. Search all buttons for send SVGs or attributes
				const allButtons = Array.from(document.querySelectorAll('button'));
				for (const btn of allButtons) {
					if (isCancelOrStop(btn)) continue;
					const aria = (btn.getAttribute('aria-label') || '').toLowerCase();
					const testid = (btn.getAttribute('data-testid') || '').toLowerCase();
					const tooltip = (btn.getAttribute('data-tooltip-id') || '').toLowerCase();
					if (aria.includes('send') || testid.includes('send') || tooltip.includes('send') || btn.type === 'submit') {
						return btn;
					}
				}
				return null;
			};

			const triggerClick = (btn) => {
				if (!btn) return false;
				try {
					if (btn.disabled) {
						btn.disabled = false;
						btn.removeAttribute('disabled');
					}
					btn.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, cancelable: true, composed: true }));
					btn.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true, composed: true }));
					btn.dispatchEvent(new PointerEvent('pointerup', { bubbles: true, cancelable: true, composed: true }));
					btn.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, cancelable: true, composed: true }));
					btn.click();
					return true;
				} catch (_) {
					try { btn.click(); return true; } catch (_) { return false; }
				}
			};

			// 6. Robust dispatch loop: attempt Enter key, form submit, and button clicks with verification
			for (let i = 0; i < 25; i++) {
				if (isBusy()) {
					window.__swissDispatchedIntents[intentKey] = Date.now();
					return { status: "conversation_became_active", ready: true, success: true };
				}
				if (isEditorCleared()) {
					window.__swissDispatchedIntents[intentKey] = Date.now();
					return { status: "clicked_send_button", ready: true, success: true };
				}

				const sendBtn = findSendButton();
				if (sendBtn) {
					triggerClick(sendBtn);
				}

				dispatchEnter();
				submitForm();

				await new Promise(r => setTimeout(r, 80));

				if (isBusy()) {
					window.__swissDispatchedIntents[intentKey] = Date.now();
					return { status: "conversation_became_active", ready: true, success: true };
				}
				if (isEditorCleared()) {
					window.__swissDispatchedIntents[intentKey] = Date.now();
					return { status: "clicked_send_button", ready: true, success: true };
				}
			}

			// Only return timeout without marking __swissDispatchedIntents so future retries succeed!
			return { status: "send_button_timeout", ready: true, success: false };
		} catch (err) {
			return { status: "error", error: String(err), ready: false, success: false };
		}
	})()`, targetPath, intentKey, prompt)

	mismatchedCount := 0
	for time.Now().Before(deadline) {
		_, targets, err := c.TargetFinder(c.CustomPort)
		if err == nil && len(targets) > 0 {
			var candidateTargets []gui.DevToolsTarget
			for _, target := range targets {
				if target.WebSocketDebuggerURL == "" {
					continue
				}
				urlLower := strings.ToLower(target.URL)
				titleLower := strings.ToLower(target.Title)

				isElectronScheme := strings.HasPrefix(urlLower, "vscode-file://") ||
					strings.HasPrefix(urlLower, "file://") ||
					strings.HasPrefix(urlLower, "app://") ||
					strings.HasPrefix(urlLower, "electron://")

				isLoopback := strings.HasPrefix(urlLower, "https://127.0.0.1:") ||
					strings.HasPrefix(urlLower, "http://127.0.0.1:") ||
					strings.HasPrefix(urlLower, "https://localhost:") ||
					strings.HasPrefix(urlLower, "http://localhost:")

				if isLoopback {
					isAntigravity := strings.Contains(urlLower, "/c/") ||
						strings.Contains(urlLower, "antigravity") ||
						strings.Contains(titleLower, "antigravity") ||
						strings.Contains(titleLower, "workbench") ||
						target.Type == "page" || target.Type == "webview"
					if !isAntigravity {
						continue
					}
				} else if !isElectronScheme {
					continue
				}

				candidateTargets = append(candidateTargets, target)
			}

			// Prioritize target whose URL directly contains targetPath
			sort.SliceStable(candidateTargets, func(i, j int) bool {
				iMatch := strings.Contains(candidateTargets[i].URL, cleanCascadeID)
				jMatch := strings.Contains(candidateTargets[j].URL, cleanCascadeID)
				if iMatch != jMatch {
					return iMatch
				}
				return false
			})

			for _, target := range candidateTargets {
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
						if status == "user_draft_in_progress" || status == "already_sent" || status == "clicked_send_button" || status == "clicked_interaction_continue" {
							return nil
						}
						if status == "mismatched_target_conversation" {
							mismatchedCount++
							if mismatchedCount >= 6 {
								return nil
							}
						} else {
							mismatchedCount = 0
						}
					}
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for conversation view and editor readiness to trigger revival")
}
