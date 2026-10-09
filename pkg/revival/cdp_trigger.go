package revival

import (
	"fmt"
	"strings"
	"time"

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

	script := fmt.Sprintf(`(async () => {
		try {
			const targetPath = %q;
			const curPath = window.location.pathname || "/";

			// 1. Ensure navigation to target conversation
			if (!curPath.startsWith(targetPath)) {
				if (curPath.startsWith("/onboarding")) {
					window.location.assign(targetPath);
					return { status: "navigating_from_onboarding", ready: false, success: false };
				}
				window.history.replaceState(null, "", targetPath);
				window.dispatchEvent(new PopStateEvent("popstate"));
				return { status: "navigating", ready: false, success: false };
			}

			// 1.5. Prevent duplicate trigger loops if already dispatched
			if (window.__swissCDPRevivalSent || window.__swissRevivalDispatched) {
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

			// 3. Check if agent is already actively generating, subagents are running, or messages are queued
			const isBusyOrQueued = Array.from(document.querySelectorAll("*")).some(el =>
				el.children.length === 0 && (
					el.textContent.includes("Queued Messages") ||
					el.textContent.includes("Sends after agent finishes") ||
					el.textContent.includes("subagents running") ||
					el.textContent.includes("Running ...")
				)
			);
			const generating = isBusyOrQueued ||
							   document.querySelector('[data-tooltip-id="input-send-button-cancel-tooltip"]') ||
							   document.querySelector('[data-testid="send-button-pending"]') ||
							   document.querySelector('[data-testid="agent-generating"]') ||
							   document.querySelector('[data-testid="stop-button"]');
			if (generating) {
				window.__swissCDPRevivalSent = true;
				window.__swissRevivalDispatched = true;
				return { status: "already_running_or_queued", ready: true, success: true };
			}

			// 4. Check if interactive questionnaire continue button is present
			const interactBtn = document.querySelector('[data-testid="interaction-continue-button"]');
			if (interactBtn && !interactBtn.disabled) {
				window.__swissCDPRevivalSent = true;
				window.__swissRevivalDispatched = true;
				interactBtn.click();
				return { status: "clicked_interaction_continue", ready: true, success: true };
			}

			// 5. Inject continuation prompt into Lexical editor
			window.__swissCDPRevivalSent = true;
			window.__swissRevivalDispatched = true;
			const promptText = %q;
			if (editor.isContentEditable) {
				editor.focus();
				document.execCommand("insertText", false, promptText);
				editor.dispatchEvent(new Event("input", { bubbles: true }));
			} else {
				editor.focus();
				editor.value = promptText;
				editor.dispatchEvent(new Event("input", { bubbles: true }));
			}

			// 6. Wait/poll until send button is enabled (Lexical enables it async after processing input)
			for (let i = 0; i < 35; i++) {
				const sendBtn = document.querySelector('[data-testid="send-button"]') ||
								document.querySelector('button[aria-label*="Send" i]') ||
								document.querySelector('.chat-input-toolbar button:last-child');
				if (sendBtn && !sendBtn.disabled) {
					sendBtn.click();
					return { status: "clicked_send_button", ready: true, success: true };
				}
				await new Promise(r => setTimeout(r, 100));
			}

			// 7. If send button is present, enable and click directly
			const finalBtn = document.querySelector('[data-testid="send-button"]') ||
							document.querySelector('button[aria-label*="Send" i]') ||
							document.querySelector('.chat-input-toolbar button:last-child');
			if (finalBtn) {
				finalBtn.disabled = false;
				finalBtn.click();
				return { status: "clicked_send_button_forced", ready: true, success: true };
			}

			return { status: "send_button_timeout", ready: true, success: false };
		} catch (err) {
			return { status: "error", error: String(err), ready: false, success: false };
		}
	})()`, targetPath, prompt)

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
					if success, ok := res["success"].(bool); ok && success {
						return nil
					}
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for conversation view and editor readiness to trigger revival")
}
