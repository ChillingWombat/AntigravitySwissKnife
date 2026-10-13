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
)

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

	for time.Now().Before(deadline) {
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

		time.Sleep(pollInterval)
	}

	// Timeout reached while still in progress
	stepIndex, _, partialContent, _ := lh.readTranscriptTail(convID)
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
