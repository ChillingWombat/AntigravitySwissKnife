package revival

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
)

// Engine orchestrates active conversation and subagent revival across app restarts and account switches.
type Engine struct {
	Store         *Store
	Detector      *SessionDetector
	CDPTrigger    *CDPTrigger
	mu            sync.RWMutex
	lastRevivedAt *time.Time
}

// NewEngine creates a new Engine instance.
func NewEngine(configDir string, customCDPPort int) *Engine {
	return &Engine{
		Store:      NewStore(configDir),
		Detector:   NewSessionDetector(),
		CDPTrigger: NewCDPTrigger(customCDPPort),
	}
}

// CapturePreSwitchState captures active conversation state before credential changes or app restarts,
// persists a RevivalIntent to disk, and pins the layout in app_storage.json.
func (e *Engine) CapturePreSwitchState(targetApp string) (*RevivalIntent, error) {
	session, err := e.Detector.Detect(targetApp)
	if err != nil || session == nil || session.ConversationID == "" {
		return nil, err
	}

	if !gui.IsValidConversationID(session.ConversationID) {
		if pinned := gui.LoadPinnedConversationPath(); gui.IsValidConversationPath(pinned) {
			session.ConversationID = strings.TrimPrefix(pinned, "/c/")
			session.TargetPath = pinned
		} else if fromDB := gui.QueryLatestTopLevelConversationPath(); gui.IsValidConversationPath(fromDB) {
			session.ConversationID = strings.TrimPrefix(fromDB, "/c/")
			session.TargetPath = fromDB
		} else {
			return nil, fmt.Errorf("refusing to persist revival intent: captured conversation ID %q is invalid", session.ConversationID)
		}
	}

	intentID := fmt.Sprintf("intent-%d", time.Now().UnixNano())
	now := time.Now()

	prompt := DefaultTriggerPrompt
	if !session.NeedsRevival {
		prompt = ""
	}
	intent := &RevivalIntent{
		IntentID:           intentID,
		TargetApp:          targetApp,
		RootConversationID: session.ConversationID,
		ActiveSubagentID:   session.ActiveSubagentID,
		TriggerPrompt:      prompt,
		CreatedAt:          now,
		Status:             IntentStatusPending,
		CascadeID:          session.ConversationID,
		Prompt:             prompt,
		Timestamp:          now.UnixMilli(),
		Attempts:           0,
		MaxAttempts:        2,
		TTLSeconds:         90,
		Resumed:            false,
	}

	// Persist pending intent
	if err := e.Store.SaveIntent(intent); err != nil {
		return nil, fmt.Errorf("failed to persist revival intent: %w", err)
	}

	// Guarantee that app_storage.json has last conversation path set
	targetPath := session.TargetPath
	if targetPath == "" {
		targetPath = "/c/" + session.ConversationID
	}
	gui.SaveLastConversationPath(targetPath)

	return intent, nil
}

// ExecutePostRelaunchRevival executes prompt injection and navigation continuation after IDE relaunch.
func (e *Engine) ExecutePostRelaunchRevival(intent *RevivalIntent) error {
	var err error
	if intent == nil {
		intent, err = e.Store.LoadIntent()
		if err != nil || intent == nil {
			return err
		}
	}

	if intent.Resumed || intent.Status == IntentStatusRevived {
		return nil
	}

	_ = e.Store.UpdateStatus(intent.IntentID, IntentStatusReviving)
	targetApp := strings.ToLower(strings.TrimSpace(intent.TargetApp))

	switch targetApp {
	case "agy":
		err = e.executeCLIRevival(intent)
	case "vscode":
		err = e.executeVSCodeRevival(intent)
	default: // desktop or all
		err = e.executeDesktopRevival(intent)
	}

	if err != nil {
		intent.Attempts++
		_ = e.Store.UpdateStatus(intent.IntentID, IntentStatusFailed)
		return err
	}

	now := time.Now()
	e.mu.Lock()
	e.lastRevivedAt = &now
	e.mu.Unlock()

	intent.Resumed = true
	_ = e.Store.UpdateStatus(intent.IntentID, IntentStatusRevived)
	_ = e.Store.ClearIntent()
	return nil
}

func (e *Engine) executeDesktopRevival(intent *RevivalIntent) error {
	if !gui.IsValidConversationID(intent.RootConversationID) {
		return fmt.Errorf("invalid conversation ID for desktop revival: %s", intent.RootConversationID)
	}

	targetPath := "/c/" + strings.TrimPrefix(intent.RootConversationID, "/c/")

	// Asynchronously ensure URL routing only in non-test mode with a valid target path
	if !gui.IsTestExecution() && os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") != "1" && !core.IsRunningTests() && gui.IsValidConversationPath(targetPath) {
		go func() {
			_ = gui.NewInjector(e.CDPTrigger.CustomPort).RestoreConversationPath(targetPath, 25*time.Second)
		}()
	}

	prompt := strings.TrimSpace(intent.TriggerPrompt)
	if prompt == "" {
		// Skip prompt injection for idle conversations
		return nil
	}

	if gui.IsTestExecution() || os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") == "1" || core.IsRunningTests() {
		return nil
	}

	return e.CDPTrigger.TriggerDesktopContinuation(intent.RootConversationID, prompt, 30*time.Second)
}

func (e *Engine) executeCLIRevival(intent *RevivalIntent) error {
	convID := intent.RootConversationID
	if convID == "" {
		return fmt.Errorf("missing conversation ID for CLI revival")
	}

	binPath, err := exec.LookPath("agy")
	if err != nil {
		// Check standard location
		home, _ := os.UserHomeDir()
		cand := fmt.Sprintf("%s/.local/bin/agy", home)
		if _, statErr := os.Stat(cand); statErr == nil {
			binPath = cand
		} else {
			return fmt.Errorf("agy binary not found: %w", err)
		}
	}

	if gui.IsTestExecution() || os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") == "1" || core.IsRunningTests() {
		return nil
	}

	cmd := exec.Command(binPath, "--conversation", convID, "--continue")
	return core.LaunchDetachedProcess(cmd)
}

func (e *Engine) executeVSCodeRevival(intent *RevivalIntent) error {
	convID := intent.RootConversationID
	if convID == "" {
		return fmt.Errorf("missing conversation ID for VS Code revival")
	}

	if gui.IsTestExecution() || os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") == "1" || core.IsRunningTests() {
		return nil
	}

	codePath, err := exec.LookPath("code")
	if err != nil {
		return fmt.Errorf("code binary not found for extension revival: %w", err)
	}

	uri := fmt.Sprintf("vscode://google.google-antigravity/open-conversation?cascadeId=%s&path=/c/%s", convID, convID)
	cmd := exec.Command(codePath, "--open-url", uri)
	return core.LaunchDetachedProcess(cmd)
}

// ReviveConversation manually or programmatically triggers continuation for the specified target.
func (e *Engine) ReviveConversation(targetApp string, conversationID string, forcePrompt string) error {
	if conversationID == "" {
		session, err := e.Detector.Detect(targetApp)
		if err != nil || session == nil {
			return fmt.Errorf("no active conversation detected: %w", err)
		}
		conversationID = session.ConversationID
	}

	if !gui.IsValidConversationID(conversationID) {
		if pinned := gui.LoadPinnedConversationPath(); gui.IsValidConversationPath(pinned) {
			conversationID = strings.TrimPrefix(pinned, "/c/")
		} else if fromDB := gui.QueryLatestTopLevelConversationPath(); gui.IsValidConversationPath(fromDB) {
			conversationID = strings.TrimPrefix(fromDB, "/c/")
		} else {
			return fmt.Errorf("cannot revive invalid conversation ID: %s", conversationID)
		}
	}

	prompt := forcePrompt
	if prompt == "" {
		prompt = DefaultTriggerPrompt
	}

	now := time.Now()
	intent := &RevivalIntent{
		IntentID:           fmt.Sprintf("manual-revive-%d", now.UnixNano()),
		TargetApp:          targetApp,
		RootConversationID: conversationID,
		TriggerPrompt:      prompt,
		CreatedAt:          now,
		Status:             IntentStatusPending,
		CascadeID:          conversationID,
		Prompt:             prompt,
		Timestamp:          now.UnixMilli(),
		Attempts:           0,
		MaxAttempts:        2,
		TTLSeconds:         90,
		Resumed:            false,
	}

	if err := e.Store.SaveIntent(intent); err != nil {
		return err
	}
	return e.ExecutePostRelaunchRevival(intent)
}

// GetRevivalStatus retrieves the active session, pending revival intent, and last revived timestamp.
func (e *Engine) GetRevivalStatus(targetConvID ...string) (*RevivalStatus, error) {
	var session *ActiveSessionInfo
	if len(targetConvID) > 0 && strings.TrimSpace(targetConvID[0]) != "" {
		session, _ = e.Detector.DetectForID(strings.TrimSpace(targetConvID[0]))
	} else {
		session, _ = e.Detector.Detect("")
	}
	pending, _ := e.Store.LoadIntent()

	e.mu.RLock()
	last := e.lastRevivedAt
	e.mu.RUnlock()

	success := (last != nil) || (pending != nil && pending.Status == IntentStatusRevived)

	return &RevivalStatus{
		ActiveSession: session,
		PendingIntent: pending,
		LastRevivedAt: last,
		Success:       success,
	}, nil
}

// AcknowledgeContinuation marks the pending intent as resumed and cleans up.
func (e *Engine) AcknowledgeContinuation(cascadeID string) error {
	intent, err := e.Store.LoadIntent()
	if err != nil {
		return err
	}
	if intent != nil {
		cleanedTarget := strings.TrimPrefix(strings.TrimSpace(cascadeID), "/c/")
		cleanedRoot := strings.TrimPrefix(strings.TrimSpace(intent.RootConversationID), "/c/")
		if cascadeID == "" || cleanedTarget == cleanedRoot || intent.CascadeID == cascadeID {
			now := time.Now()
			e.mu.Lock()
			e.lastRevivedAt = &now
			e.mu.Unlock()
			intent.Resumed = true
			intent.Status = IntentStatusRevived
			_ = e.Store.ClearIntent()
		}
	}
	return nil
}
