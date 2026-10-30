package bridge

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/quota"
)

// activeModelExhaustedChecker is a test seam around the live quota probe.
var activeModelExhaustedChecker = checkActiveModelExhausted

func checkActiveModelExhausted() (bool, map[string]interface{}) {
	store, err := keyring.NewStore("")
	if err != nil {
		return false, nil
	}
	active := store.ActiveAccount()
	if active == "" {
		return false, nil
	}
	cachedMap := quota.LoadQuotaCache()
	activeCached := cachedMap[strings.ToLower(strings.TrimSpace(active))]
	if activeCached == nil {
		return false, nil
	}

	if activeCached.ClaudeWeeklyBucketPresent && (activeCached.Quota5hClaudeGPT <= 0 || activeCached.QuotaWeeklyClaudeGPT <= 0) {
		return true, map[string]interface{}{
			"model_family":              "Claude / Opus",
			"claude_gpt_5h_percent":     activeCached.Quota5hClaudeGPT * 100,
			"claude_gpt_weekly_percent": activeCached.QuotaWeeklyClaudeGPT * 100,
			"account":                   active,
		}
	}
	return false, nil
}

// LeadSession tracks an active Antigravity conversation orchestrated by Gemini.
type LeadSession struct {
	ConversationID string    `json:"conversation_id"`
	Title          string    `json:"title"`
	LastPrompt     string    `json:"last_prompt"`
	SubmissionStep int       `json:"submission_step"`
	Status         string    `json:"status"` // "dispatched" | "generating" | "completed" | "error"
	LastResponse   string    `json:"last_response,omitempty"`
	Artifacts      []string  `json:"artifacts,omitempty"`
	StartedAt      time.Time `json:"started_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// LeadHandler implements the Messenger / Team Lead orchestration between Gemini and Antigravity.
type LeadHandler struct {
	mu            sync.RWMutex
	activeSession *LeadSession
	injector      *gui.Injector
	brainDir      string
}

// NewLeadHandler initializes a lead handler.
func NewLeadHandler(brainDir string) *LeadHandler {
	if brainDir == "" {
		brainDir = filepath.Join(os.Getenv("HOME"), ".gemini", "antigravity", "brain")
	}
	return &LeadHandler{
		injector: gui.NewInjector(0),
		brainDir: brainDir,
	}
}

// SetInjector assigns an injector (e.g. for testing with custom DevTools port).
func (lh *LeadHandler) SetInjector(inj *gui.Injector) {
	lh.mu.Lock()
	defer lh.mu.Unlock()
	lh.injector = inj
}

// GetInjector returns the currently configured injector.
func (lh *LeadHandler) GetInjector() *gui.Injector {
	lh.mu.RLock()
	defer lh.mu.RUnlock()
	return lh.injector
}

// GetActiveSession returns the current tracked lead session.
func (lh *LeadHandler) GetActiveSession() *LeadSession {
	lh.mu.RLock()
	defer lh.mu.RUnlock()
	if lh.activeSession == nil {
		return nil
	}
	cpy := *lh.activeSession
	return &cpy
}

// CreateConversation creates a new conversation in Antigravity 2.0 and submits the prompt.
func (lh *LeadHandler) CreateConversation(args map[string]interface{}) (string, error) {
	prompt, _ := args["prompt"].(string)
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("prompt is required")
	}
	title, _ := args["title"].(string)
	if title == "" {
		title = "Gemini Orchestrated Task"
	}

	// 1. Connect to Antigravity 2.0 Desktop via CDP
	port, err := lh.injector.FindDevToolsPort()
	if err != nil || port <= 0 {
		return "", fmt.Errorf("could not connect to running Antigravity 2.0 Desktop IDE: %w", err)
	}

	pages, err := lh.injector.GetPageTargets(port)
	if err != nil || len(pages) == 0 {
		return "", fmt.Errorf("no Antigravity 2.0 window target found: %w", err)
	}

	target := pages[0]
	for _, p := range pages {
		if strings.HasPrefix(p.URL, "https://127.0.0.1:") || strings.HasPrefix(p.URL, "http://127.0.0.1:") || strings.HasPrefix(p.URL, "https://localhost:") || strings.HasPrefix(p.URL, "http://localhost:") {
			target = p
			break
		}
	}
	promptJSON, err := json.Marshal(prompt)
	if err != nil {
		return "", fmt.Errorf("failed to marshal prompt: %w", err)
	}

	// 2. Click "New Conversation" button, then inject prompt into Lexical editor and click Send
	script := fmt.Sprintf(`(async () => {
		// Step A: Find and click New Conversation button
		const newBtn = document.querySelector('[data-testid="app-icon-new-conversation-button"]') ||
		               document.querySelector('[data-testid="new-conversation-button"]') ||
		               document.querySelector('[data-testid="new-chat-button"]') ||
		               document.querySelector('button[aria-label*="New conversation" i]') ||
		               document.querySelector('button[title*="New conversation" i]') ||
		               document.querySelector('.new-chat-btn');
		if (newBtn) {
			newBtn.click();
			await new Promise(r => setTimeout(r, 120));
		}

		// Step B: Wait for editor input box to mount
		let box = null;
		for (let i = 0; i < 25; i++) {
			box = document.querySelector('[data-testid="agent-input-box"] [contenteditable="true"]') ||
			      document.querySelector('[data-lexical-editor="true"][contenteditable="true"]') ||
			      document.querySelector('.lexical-container [contenteditable="true"]') ||
			      document.querySelector('[data-testid="chat-input-textarea"]') ||
			      document.querySelector('textarea[placeholder*="Ask"]');
			if (box) break;
			await new Promise(r => setTimeout(r, 80));
		}
		if (!box) {
			return { success: false, reason: "input_not_found" };
		}

		// Step C: Inject prompt into Lexical editor
		const promptText = %s;
		if (box.isContentEditable) {
			box.focus();
			const lex = box.__lexicalEditor;
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
					range.selectNodeContents(box);
					sel.removeAllRanges();
					sel.addRange(range);
				}
				document.execCommand("selectAll", false, null);
				document.execCommand("insertText", false, promptText);
				const hasInserted = (box.innerText || "").includes(promptText);
				if (!hasInserted) {
					try {
						box.dispatchEvent(new InputEvent("beforeinput", {
							inputType: "insertText",
							data: promptText,
							bubbles: true,
							cancelable: true
						}));
					} catch (_) {}
				}
			}
			box.dispatchEvent(new Event("input", { bubbles: true, composed: true }));
		} else {
			box.focus();
			box.value = promptText;
			box.dispatchEvent(new Event("input", { bubbles: true }));
			box.dispatchEvent(new Event("change", { bubbles: true }));
		}

		// Step D: Wait and poll until Send button is enabled
		await new Promise(r => setTimeout(r, 80));
		let sent = false;
		for (let i = 0; i < 35; i++) {
			const sendBtn = document.querySelector('[data-testid="send-button"]') ||
			                document.querySelector('[data-tooltip-id*="send-tooltip"]') ||
			                document.querySelector('button[aria-label*="Send" i]') ||
			                document.querySelector('.chat-input-toolbar button:last-child');
			if (sendBtn && !sendBtn.disabled && !sendBtn.matches('[data-tooltip-id="input-send-button-cancel-tooltip"]')) {
				sendBtn.click();
				sent = true;
				break;
			}
			await new Promise(r => setTimeout(r, 100));
		}

		if (!sent) {
			const finalBtn = document.querySelector('[data-testid="send-button"]') ||
			                 document.querySelector('[data-tooltip-id*="send-tooltip"]') ||
			                 document.querySelector('button[aria-label*="Send" i]') ||
			                 document.querySelector('.chat-input-toolbar button:last-child');
			if (finalBtn && !finalBtn.matches('[data-tooltip-id="input-send-button-cancel-tooltip"]')) {
				finalBtn.disabled = false;
				finalBtn.click();
				sent = true;
			}
		}

		return { success: true, action: "new_conversation_triggered", dispatched: sent };
	})()`, string(promptJSON))

	_, cdpErr := lh.injector.ExecuteScript(target.WebSocketDebuggerURL, script)
	if cdpErr != nil {
		return "", fmt.Errorf("CDP execution failed: %w", cdpErr)
	}

	// Give filesystem a moment to create the new brain directory
	time.Sleep(1200 * time.Millisecond)

	convID := lh.findMostRecentConversationID()
	if convID == "" {
		convID = fmt.Sprintf("conv-%d", time.Now().Unix())
	}

	currentStep := lh.getLatestStepIndex(convID)

	session := &LeadSession{
		ConversationID: convID,
		Title:          title,
		LastPrompt:     prompt,
		SubmissionStep: currentStep,
		Status:         "generating",
		StartedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	lh.mu.Lock()
	lh.activeSession = session
	lh.mu.Unlock()

	res := map[string]interface{}{
		"status":          "dispatched",
		"conversation_id": convID,
		"prompt":          prompt,
		"submission_step": currentStep,
		"message":         "Conversation initiated in local Antigravity 2.0. Call 'antigravity_lead_await_response' to receive the agent's completed response and artifacts.",
	}
	data, _ := json.MarshalIndent(res, "", "  ")
	return string(data), nil
}

// SendMessage sends a follow-up prompt or task assignment to an active conversation.
func (lh *LeadHandler) SendMessage(args map[string]interface{}) (string, error) {
	message, _ := args["message"].(string)
	if strings.TrimSpace(message) == "" {
		return "", fmt.Errorf("message is required")
	}

	convID, _ := args["conversation_id"].(string)
	if convID == "" {
		lh.mu.RLock()
		if lh.activeSession != nil {
			convID = lh.activeSession.ConversationID
		}
		lh.mu.RUnlock()
	}
	if convID == "" {
		convID = lh.findMostRecentConversationID()
	}

	port, err := lh.injector.FindDevToolsPort()
	if err != nil || port <= 0 {
		return "", fmt.Errorf("Antigravity 2.0 Desktop not reachable: %w", err)
	}
	pages, err := lh.injector.GetPageTargets(port)
	if err != nil || len(pages) == 0 {
		return "", fmt.Errorf("no target pages: %w", err)
	}

	target := pages[0]
	for _, p := range pages {
		if strings.HasPrefix(p.URL, "https://127.0.0.1:") || strings.HasPrefix(p.URL, "http://127.0.0.1:") || strings.HasPrefix(p.URL, "https://localhost:") || strings.HasPrefix(p.URL, "http://localhost:") {
			target = p
			break
		}
	}
	msgJSON, err := json.Marshal(message)
	if err != nil {
		return "", fmt.Errorf("failed to marshal message: %w", err)
	}

	convPart := ""
	if convID != "" && gui.IsValidConversationID(convID) {
		convPart = convID
	}

	script := fmt.Sprintf(`(async () => {
		const targetConv = %q;
		if (targetConv) {
			const targetPath = "/c/" + targetConv;
			const curPath = window.location.pathname || "/";
			if (!curPath.startsWith(targetPath)) {
				const navLink = document.querySelector('a[href*="' + targetConv + '"]');
				if (navLink) {
					navLink.click();
					await new Promise(r => setTimeout(r, 150));
				} else {
					window.location.assign(targetPath);
					await new Promise(r => setTimeout(r, 200));
				}
			}
		}

		let box = null;
		for (let i = 0; i < 20; i++) {
			box = document.querySelector('[data-testid="agent-input-box"] [contenteditable="true"]') ||
			      document.querySelector('[data-lexical-editor="true"][contenteditable="true"]') ||
			      document.querySelector('.lexical-container [contenteditable="true"]') ||
			      document.querySelector('[data-testid="chat-input-textarea"]') ||
			      document.querySelector('textarea[placeholder*="Ask"]');
			if (box) break;
			await new Promise(r => setTimeout(r, 80));
		}
		if (!box) return { success: false, reason: "input_not_found" };

		const promptText = %s;
		if (box.isContentEditable) {
			box.focus();
			const lex = box.__lexicalEditor;
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
					range.selectNodeContents(box);
					sel.removeAllRanges();
					sel.addRange(range);
				}
				document.execCommand("selectAll", false, null);
				document.execCommand("insertText", false, promptText);
				const hasInserted = (box.innerText || "").includes(promptText);
				if (!hasInserted) {
					try {
						box.dispatchEvent(new InputEvent("beforeinput", {
							inputType: "insertText",
							data: promptText,
							bubbles: true,
							cancelable: true
						}));
					} catch (_) {}
				}
			}
			box.dispatchEvent(new Event("input", { bubbles: true, composed: true }));
		} else {
			box.focus();
			box.value = promptText;
			box.dispatchEvent(new Event("input", { bubbles: true }));
			box.dispatchEvent(new Event("change", { bubbles: true }));
		}

		await new Promise(r => setTimeout(r, 80));
		for (let i = 0; i < 35; i++) {
			const sendBtn = document.querySelector('[data-testid="send-button"]') ||
			                document.querySelector('[data-tooltip-id*="send-tooltip"]') ||
			                document.querySelector('button[aria-label*="Send" i]') ||
			                document.querySelector('.chat-input-toolbar button:last-child');
			if (sendBtn && !sendBtn.disabled && !sendBtn.matches('[data-tooltip-id="input-send-button-cancel-tooltip"]')) {
				sendBtn.click();
				return { success: true, dispatched: true };
			}
			await new Promise(r => setTimeout(r, 100));
		}

		const finalBtn = document.querySelector('[data-testid="send-button"]') ||
		                 document.querySelector('[data-tooltip-id*="send-tooltip"]') ||
		                 document.querySelector('button[aria-label*="Send" i]') ||
		                 document.querySelector('.chat-input-toolbar button:last-child');
		if (finalBtn && !finalBtn.matches('[data-tooltip-id="input-send-button-cancel-tooltip"]')) {
			finalBtn.disabled = false;
			finalBtn.click();
			return { success: true, dispatched: true, forced: true };
		}
		return { success: false, dispatched: false, reason: "send_button_disabled" };
	})()`, convPart, string(msgJSON))

	_, cdpErr := lh.injector.ExecuteScript(target.WebSocketDebuggerURL, script)
	if cdpErr != nil {
		return "", fmt.Errorf("send message CDP failed: %w", cdpErr)
	}

	currentStep := lh.getLatestStepIndex(convID)

	lh.mu.Lock()
	if lh.activeSession != nil && lh.activeSession.ConversationID == convID {
		lh.activeSession.LastPrompt = message
		lh.activeSession.SubmissionStep = currentStep
		lh.activeSession.Status = "generating"
		lh.activeSession.UpdatedAt = time.Now()
	} else {
		lh.activeSession = &LeadSession{
			ConversationID: convID,
			LastPrompt:     message,
			SubmissionStep: currentStep,
			Status:         "generating",
			StartedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
	}
	lh.mu.Unlock()

	res := map[string]interface{}{
		"status":          "message_sent",
		"conversation_id": convID,
		"message":         "Message dispatched to Antigravity. Call 'antigravity_lead_await_response' to receive the resulting response.",
	}
	data, _ := json.MarshalIndent(res, "", "  ")
	return string(data), nil
}

// GetStatus checks execution state of a conversation.
func (lh *LeadHandler) GetStatus(args map[string]interface{}) (string, error) {
	convID, _ := args["conversation_id"].(string)
	if convID == "" {
		lh.mu.RLock()
		if lh.activeSession != nil {
			convID = lh.activeSession.ConversationID
		}
		lh.mu.RUnlock()
	}
	if convID == "" {
		convID = lh.findMostRecentConversationID()
	}
	if convID == "" {
		return "", fmt.Errorf("no active or recent conversation found")
	}

	latestStep, isDone, lastContent, err := lh.readTranscriptTail(convID)
	if err != nil {
		return "", err
	}

	artifacts := lh.listArtifacts(convID)

	status := "generating"
	if isDone {
		status = "completed"
	}

	info := map[string]interface{}{
		"conversation_id": convID,
		"status":          status,
		"latest_step":     latestStep,
		"is_completed":    isDone,
		"preview":         truncateString(lastContent, 200),
		"artifacts":       artifacts,
	}

	data, _ := json.MarshalIndent(info, "", "  ")
	return string(data), nil
}

// AwaitResponse polls the conversation transcript until Antigravity finishes its turn, then forwards the response.
func (lh *LeadHandler) AwaitResponse(args map[string]interface{}) (string, error) {
	convID, _ := args["conversation_id"].(string)
	if convID == "" {
		lh.mu.RLock()
		if lh.activeSession != nil {
			convID = lh.activeSession.ConversationID
		}
		lh.mu.RUnlock()
	}
	if convID == "" {
		convID = lh.findMostRecentConversationID()
	}
	if convID == "" {
		return "", fmt.Errorf("no conversation ID specified or active")
	}

	timeoutSec := 60
	if ts, ok := args["timeout_seconds"].(float64); ok && int(ts) > 0 {
		timeoutSec = int(ts)
		if timeoutSec > 180 {
			timeoutSec = 180
		}
	}

	lh.mu.RLock()
	baselineStep := 0
	if lh.activeSession != nil && lh.activeSession.ConversationID == convID {
		baselineStep = lh.activeSession.SubmissionStep
	}
	lh.mu.RUnlock()

	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	pollInterval := 1500 * time.Millisecond
	iterations := 0

	for time.Now().Before(deadline) {
		iterations++
		stepIndex, isDone, content, err := lh.readTranscriptTail(convID)
		if err == nil && isDone && stepIndex >= baselineStep && len(strings.TrimSpace(content)) > 0 {
			artifacts := lh.listArtifacts(convID)

			lh.mu.Lock()
			if lh.activeSession != nil && lh.activeSession.ConversationID == convID {
				lh.activeSession.Status = "completed"
				lh.activeSession.LastResponse = content
				lh.activeSession.Artifacts = artifacts
				lh.activeSession.UpdatedAt = time.Now()
			}
			lh.mu.Unlock()

			forwardedPayload := map[string]interface{}{
				"status":          "COMPLETED",
				"conversation_id": convID,
				"step_index":      stepIndex,
				"response":        content,
				"artifacts":       artifacts,
				"forwarded_to":    "Gemini Lead / Messenger",
				"next_action":     "Report this response to the user. Ask if they want follow-up tasks or modifications.",
			}
			data, _ := json.MarshalIndent(forwardedPayload, "", "  ")
			return string(data), nil
		}

		// Detect Antigravity stuck on "Connecting..." due to exhausted Claude/Opus quota:
		if iterations >= 10 && len(strings.TrimSpace(content)) == 0 && stepIndex == baselineStep {
			if exhausted, info := activeModelExhaustedChecker(); exhausted {
				res := map[string]interface{}{
					"status":                "QUOTA_EXHAUSTED",
					"conversation_id":       convID,
					"error":                 "Antigravity appears hung on 'Connecting...' because the Claude / Opus model quota is exhausted (0% remaining).",
					"exhaustion_details":    info,
					"message":               "Antigravity is waiting on 'Connecting...' and cannot proceed with the exhausted Claude/Opus model. Switch the active model to Gemini 3.8 Flash or Gemini 4 Argon, or switch Google accounts.",
					"actionable_suggestion": "Call antigravity_session_model_set with model='Gemini 3.8 Flash' or call antigravity_fleet_switch_account to switch to an account with available quota.",
				}
				data, _ := json.MarshalIndent(res, "", "  ")
				return string(data), nil
			}
		}

		time.Sleep(pollInterval)
	}

	// Timeout reached while still in progress
	stepIndex, _, partialContent, _ := lh.readTranscriptTail(convID)
	if len(strings.TrimSpace(partialContent)) == 0 && stepIndex == baselineStep {
		if exhausted, info := activeModelExhaustedChecker(); exhausted {
			res := map[string]interface{}{
				"status":                "QUOTA_EXHAUSTED",
				"conversation_id":       convID,
				"error":                 "Antigravity timed out waiting on 'Connecting...' because the Claude / Opus model quota is exhausted (0% remaining).",
				"exhaustion_details":    info,
				"message":               "Antigravity is waiting on 'Connecting...' and cannot proceed with the exhausted Claude/Opus model.",
				"actionable_suggestion": "Call antigravity_session_model_set with model='Gemini 3.8 Flash' or call antigravity_fleet_switch_account to switch to an account with available quota.",
			}
			data, _ := json.MarshalIndent(res, "", "  ")
			return string(data), nil
		}
	}
	stepIndex, _, partialContent, _ = lh.readTranscriptTail(convID)
	artifacts := lh.listArtifacts(convID)

	res := map[string]interface{}{
		"status":          "IN_PROGRESS",
		"conversation_id": convID,
		"current_step":    stepIndex,
		"partial_preview": truncateString(partialContent, 300),
		"artifacts":       artifacts,
		"message":         fmt.Sprintf("Antigravity is actively executing tasks (step %d). Call 'antigravity_lead_await_response' again to wait for final completion.", stepIndex),
	}
	data, _ := json.MarshalIndent(res, "", "  ")
	return string(data), nil
}

// ListConversations returns recent local Antigravity conversations.
func (lh *LeadHandler) ListConversations(args map[string]interface{}) (string, error) {
	limit := 10
	if l, ok := args["limit"].(float64); ok && int(l) > 0 {
		limit = int(l)
	}

	entries, err := os.ReadDir(lh.brainDir)
	if err != nil {
		return "[]", nil
	}

	type ConvSummary struct {
		ID        string `json:"id"`
		ModTime   string `json:"last_active"`
		IsActive  bool   `json:"is_active"`
		LastStep  int    `json:"last_step,omitempty"`
		Artifacts int    `json:"artifacts_count"`
	}

	type sortableEntry struct {
		name    string
		modTime time.Time
	}
	items := make([]sortableEntry, 0)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, _ := e.Info()
		mt := time.Time{}
		if info != nil {
			mt = info.ModTime()
		}
		items = append(items, sortableEntry{name: e.Name(), modTime: mt})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].modTime.After(items[j].modTime)
	})

	lh.mu.RLock()
	activeID := ""
	if lh.activeSession != nil {
		activeID = lh.activeSession.ConversationID
	}
	lh.mu.RUnlock()

	summaries := make([]ConvSummary, 0)
	for i := 0; i < len(items) && len(summaries) < limit; i++ {
		cID := items[i].name
		step, _, _, _ := lh.readTranscriptTail(cID)
		artifacts := lh.listArtifacts(cID)
		summaries = append(summaries, ConvSummary{
			ID:        cID,
			ModTime:   items[i].modTime.Format("2006-01-02 15:04:05"),
			IsActive:  cID == activeID,
			LastStep:  step,
			Artifacts: len(artifacts),
		})
	}

	data, _ := json.MarshalIndent(summaries, "", "  ")
	return string(data), nil
}

// CompactConversation invokes Antigravity's native /compact command in the active conversation.
func (lh *LeadHandler) CompactConversation(args map[string]interface{}) (string, error) {
	convID := lh.resolveConversationID(args)
	target, err := lh.activePage()
	if err != nil {
		return "", err
	}

	convJSON, _ := json.Marshal(convID)
	script := fmt.Sprintf(`(async () => {
		const targetConv = %s;
		if (targetConv) {
			const targetPath = "/c/" + targetConv;
			if (!(window.location.pathname || "").startsWith(targetPath)) {
				const navLink = document.querySelector('a[href*="' + targetConv + '"]');
				if (navLink) {
					navLink.click();
				} else {
					window.location.assign(targetPath);
				}
				await new Promise(r => setTimeout(r, 250));
			}
		}

		const contextRing = document.getElementById("swiss-context-window-ring") ||
			document.querySelector(".swiss-context-window-ring");
		if (contextRing) {
			["pointerdown", "mousedown", "pointerup", "mouseup"].forEach(type => {
				try {
					contextRing.dispatchEvent(new MouseEvent(type, { bubbles: true, cancelable: true }));
				} catch (_) {}
			});
			contextRing.click();
			await new Promise(r => setTimeout(r, 80));
			return { success: true, action: "context_ring_click", conversation_id: targetConv || null };
		}

		let inputBox = null;
		for (let i = 0; i < 20; i++) {
			inputBox = document.querySelector('[data-testid="agent-input-box"]');
			if (inputBox) break;
			await new Promise(r => setTimeout(r, 100));
		}
		if (!inputBox) {
			return { success: false, reason: "agent_input_not_found" };
		}
		function findInputBoxFiberProps(box) {
			if (!box) return null;
			if (box.memoizedProps && typeof box.memoizedProps.handleSubmit === "function") {
				return box.memoizedProps;
			}
			let fiberKey = Object.keys(box).find(k => k.startsWith("__reactFiber"));
			let startNode = fiberKey ? box[fiberKey] : null;
			if (!startNode && box.querySelector) {
				const inner = box.querySelector('[contenteditable="true"]') || box.querySelector("textarea");
				if (inner) {
					if (inner.memoizedProps) return inner.memoizedProps;
					fiberKey = Object.keys(inner).find(k => k.startsWith("__reactFiber"));
					if (fiberKey) startNode = inner[fiberKey];
				}
			}
			if (!startNode) return null;
			let bestProps = null;
			let node = startNode;
			while (node) {
				const props = node.memoizedProps;
				if (props && typeof props.handleSubmit === "function") {
					if (props.lexicalRef && props.lexicalRef.current) return props;
					if (!bestProps) bestProps = props;
				}
				node = node.return;
			}
			const queue = [startNode];
			const visited = new Set();
			while (queue.length > 0 && visited.size < 300) {
				node = queue.shift();
				if (!node || visited.has(node)) continue;
				visited.add(node);
				const props = node.memoizedProps;
				if (props && typeof props.handleSubmit === "function") {
					if (props.lexicalRef && props.lexicalRef.current) return props;
					if (!bestProps) bestProps = props;
				}
				if (node.child) queue.push(node.child);
				if (node.sibling) queue.push(node.sibling);
			}
			return bestProps;
		}
		const props = findInputBoxFiberProps(inputBox);
		if (!props || typeof props.handleSubmit !== "function") {
			return { success: false, reason: "native_submit_handler_not_found" };
		}
		const editor = (props.lexicalRef && props.lexicalRef.current) ||
			inputBox.querySelector('[contenteditable="true"]') ||
			inputBox.querySelector('[data-lexical-editor="true"]');
		if (!editor) {
			return { success: false, reason: "editor_not_found" };
		}
		let compactInfo = null;
		if (Array.isArray(props.slashCommandItems)) {
			compactInfo = props.slashCommandItems.find(item => item && (item.name === "compact" || (item.slashCommand && item.slashCommand.name === "compact")));
		}
		if (!compactInfo) {
			compactInfo = { $typeName: "exa.codeium_common_pb.SlashCommandInfo", name: "compact", type: 1 };
		}
		const state = {
			root: {
				children: [{
					children: [{
						type: "contextScopeItemMention",
						version: 1,
						detail: compactInfo,
						slashCommandInfo: compactInfo,
						text: "/compact",
						value: "/compact"
					}],
					direction: "ltr",
					format: "",
					indent: 0,
					type: "paragraph",
					version: 1
				}],
				direction: "ltr",
				format: "",
				indent: 0,
				type: "root",
				version: 1
			}
		};
		let stateApplied = false;
		if (typeof editor.setEditorState === "function") {
			try {
				if (typeof editor.parseEditorState === "function") {
					const parsed = editor.parseEditorState(JSON.stringify(state));
					if (!parsed) throw new Error("empty editor state");
					editor.setEditorState(parsed);
				} else {
					editor.setEditorState(state);
				}
				stateApplied = true;
			} catch (_) {}
		}
		if (!stateApplied) {
			if (editor.isContentEditable) {
				editor.focus();
				editor.innerText = "/compact";
				editor.dispatchEvent(new InputEvent("input", { bubbles: true, inputType: "insertText", data: "/compact" }));
			} else {
				editor.value = "/compact";
				editor.dispatchEvent(new Event("input", { bubbles: true }));
			}
		}
		await props.handleSubmit(editor, 1);
		return { success: true, action: "native_compact_submit", conversation_id: targetConv || null };
	})()`, string(convJSON))

	response, err := lh.injector.ExecuteScript(target.WebSocketDebuggerURL, script)
	if err != nil {
		return "", fmt.Errorf("compact conversation CDP failed: %w", err)
	}
	if success, ok := response["success"].(bool); ok && !success {
		reason, _ := response["reason"].(string)
		if reason == "" {
			reason = "native compact command was not dispatched"
		}
		return "", fmt.Errorf("%s", reason)
	}

	result := map[string]interface{}{
		"status":          "compact_requested",
		"conversation_id": convID,
		"message":         "Native /compact command dispatched to Antigravity.",
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return string(data), nil
}

// StopConversation clicks Antigravity's active cancel/stop control when a turn is running.
func (lh *LeadHandler) StopConversation(args map[string]interface{}) (string, error) {
	convID := lh.resolveConversationID(args)
	target, err := lh.activePage()
	if err != nil {
		return "", err
	}
	convJSON, _ := json.Marshal(convID)
	script := fmt.Sprintf(`(async () => {
		const targetConv = %s;
		if (targetConv) {
			const targetPath = "/c/" + targetConv;
			if (!(window.location.pathname || "").startsWith(targetPath)) {
				const navLink = document.querySelector('a[href*="' + targetConv + '"]');
				if (navLink) {
					navLink.click();
				} else {
					window.location.assign(targetPath);
				}
				await new Promise(r => setTimeout(r, 250));
			}
		}
		let cancel = null;
		for (let i = 0; i < 20; i++) {
			cancel = document.querySelector('[data-tooltip-id="input-send-button-cancel-tooltip"]') ||
				document.querySelector('[data-tooltip-id*="cancel-tooltip"]') ||
				document.querySelector('button[aria-label*="Cancel" i]') ||
				document.querySelector('button[aria-label*="Stop" i]') ||
				document.querySelector('[data-testid="stop-button"]');
			if (cancel) break;
			await new Promise(r => setTimeout(r, 100));
		}
		if (!cancel) {
			return { success: false, reason: "not_running" };
		}
		cancel.click();
		return { success: true, action: "cancel_click", conversation_id: targetConv || null };
	})()`, string(convJSON))
	response, err := lh.injector.ExecuteScript(target.WebSocketDebuggerURL, script)
	if err != nil {
		return "", fmt.Errorf("stop conversation CDP failed: %w", err)
	}
	if success, ok := response["success"].(bool); ok && !success {
		return "", fmt.Errorf("no in-progress generation found for conversation %s", convID)
	}
	result := map[string]interface{}{
		"status":          "stopped",
		"conversation_id": convID,
		"message":         "Cancel request sent to Antigravity.",
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return string(data), nil
}

func (lh *LeadHandler) resolveConversationID(args map[string]interface{}) string {
	convID, _ := args["conversation_id"].(string)
	convID = strings.TrimSpace(convID)
	if convID == "" {
		lh.mu.RLock()
		if lh.activeSession != nil {
			convID = lh.activeSession.ConversationID
		}
		lh.mu.RUnlock()
	}
	if convID == "" {
		convID = lh.findMostRecentConversationID()
	}
	return convID
}

func (lh *LeadHandler) activePage() (gui.DevToolsTarget, error) {
	port, err := lh.injector.FindDevToolsPort()
	if err != nil {
		return gui.DevToolsTarget{}, fmt.Errorf("Antigravity 2.0 Desktop not reachable: %w", err)
	}
	if port <= 0 {
		return gui.DevToolsTarget{}, fmt.Errorf("Antigravity 2.0 Desktop not reachable: invalid DevTools port")
	}
	pages, err := lh.injector.GetPageTargets(port)
	if err != nil {
		return gui.DevToolsTarget{}, fmt.Errorf("no Antigravity 2.0 window target found: %w", err)
	}
	if len(pages) == 0 {
		return gui.DevToolsTarget{}, fmt.Errorf("no Antigravity 2.0 window target found")
	}
	target := pages[0]
	for _, p := range pages {
		if strings.Contains(p.URL, "127.0.0.1") || strings.Contains(p.URL, "localhost") {
			target = p
			break
		}
	}
	return target, nil
}

func (lh *LeadHandler) findMostRecentConversationID() string {
	if topLevel := gui.QueryLatestTopLevelConversationPath(); gui.IsValidConversationPath(topLevel) {
		return strings.TrimPrefix(topLevel, "/c/")
	}
	entries, err := os.ReadDir(lh.brainDir)
	if err != nil {
		return ""
	}
	var newestName string
	var newestTime time.Time
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "tempmediaStorage" {
			continue
		}
		info, err := e.Info()
		if err == nil && info.ModTime().After(newestTime) {
			newestTime = info.ModTime()
			newestName = e.Name()
		}
	}
	return newestName
}

func (lh *LeadHandler) getLatestStepIndex(convID string) int {
	step, _, _, _ := lh.readTranscriptTail(convID)
	return step
}

func (lh *LeadHandler) readTranscriptTail(convID string) (int, bool, string, error) {
	logPath := filepath.Join(lh.brainDir, convID, ".system_generated", "logs", "transcript.jsonl")
	f, err := os.Open(logPath)
	if err != nil {
		return 0, false, "", fmt.Errorf("open transcript: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var lastStep int
	var lastContent string
	var isDone bool

	// Read last 25 lines
	lines := make([]string, 0, 25)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > 25 {
			lines = lines[1:]
		}
	}

	for _, line := range lines {
		var entry struct {
			StepIndex int    `json:"step_index"`
			Source    string `json:"source"`
			Type      string `json:"type"`
			Status    string `json:"status"`
			Content   string `json:"content"`
		}
		if jsonErr := json.Unmarshal([]byte(line), &entry); jsonErr == nil {
			if entry.StepIndex > lastStep {
				lastStep = entry.StepIndex
			}
			if entry.Type == "PLANNER_RESPONSE" {
				if entry.Status == "DONE" && len(entry.Content) > 0 {
					isDone = true
					lastContent = entry.Content
				} else if entry.Status == "IN_PROGRESS" || entry.Status == "RUNNING" {
					isDone = false
				}
			}
		}
	}

	return lastStep, isDone, lastContent, nil
}

func (lh *LeadHandler) listArtifacts(convID string) []string {
	dir := filepath.Join(lh.brainDir, convID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var res []string
	for _, e := range entries {
		if !e.IsDir() && (strings.HasSuffix(e.Name(), ".md") || strings.HasSuffix(e.Name(), ".png") || strings.HasSuffix(e.Name(), ".json")) {
			res = append(res, e.Name())
		}
	}
	return res
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
