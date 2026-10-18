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
	revivedMap    map[string]time.Time
}

// NewEngine creates a new Engine instance.
func NewEngine(configDir string, customCDPPort int) *Engine {
	return &Engine{
		Store:         NewStore(configDir),
		Detector:      NewSessionDetector(),
		CDPTrigger:    NewCDPTrigger(customCDPPort),
		revivedMap:    make(map[string]time.Time),
	}
}

func (e *Engine) getPrompt() string {
	cfg, err := core.LoadConfig()
	if err == nil && cfg != nil {
		if p := cfg.GetSessionResumePrompt(); p != "" {
			return p
		}
	}
	return DefaultTriggerPrompt
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

	prompt := e.getPrompt()
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
		MaxAttempts:        DefaultMaxAttempts,
		TTLSeconds:         DefaultTTLSeconds,
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

	maxAttempts := intent.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = DefaultMaxAttempts
	}
	if intent.Attempts >= maxAttempts {
		_ = e.Store.UpdateStatus(intent.IntentID, IntentStatusFailed)
		return fmt.Errorf("max revival attempts (%d) reached for intent %s", maxAttempts, intent.IntentID)
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
		attempts, _ := e.Store.IncrementAttempts(intent.IntentID)
		intent.Attempts = attempts
		_ = e.Store.UpdateStatus(intent.IntentID, IntentStatusFailed, attempts)
		return err
	}

	now := time.Now()
	e.mu.Lock()
	e.lastRevivedAt = &now
	if e.revivedMap == nil {
		e.revivedMap = make(map[string]time.Time)
	}
	if intent.RootConversationID != "" {
		cleaned := strings.TrimPrefix(intent.RootConversationID, "/c/")
		e.revivedMap[cleaned] = now
	}
	e.mu.Unlock()

	intent.Resumed = true
	_ = e.Store.UpdateStatus(intent.IntentID, IntentStatusRevived)
	_ = e.Store.ClearIntent()
	return nil
}

// desktopRestoreRouteWait bounds how long the desktop revival waits for the
// target conversation route to become VERIFIED active before giving up on this
// attempt (the intent stays retryable through the existing attempts machinery,
// and the injected in-page auto-revival loop keeps trying independently).
const desktopRestoreRouteWait = 20 * time.Second

// desktopRevivalDeps is the navigation/injection surface of the desktop revival
// sequence. It exists so the strict ordering contract — the resume prompt is
// submitted only after the target route has been VERIFIED active — is unit
// testable without a live renderer (issue #122).
type desktopRevivalDeps struct {
	// RestoreRoute drives the renderer to targetPath and reports whether the
	// target route was verified active.
	RestoreRoute func(targetPath string, maxWait time.Duration) gui.RestoreResult
	// Inject submits the resume prompt into the (already verified) conversation.
	Inject func(cascadeID, prompt, intentID string, timeout time.Duration) error
}

// desktopRevivalDeps returns the live dependencies for executeDesktopRevival.
func (e *Engine) desktopRevivalDeps() desktopRevivalDeps {
	return desktopRevivalDeps{
		RestoreRoute: func(targetPath string, maxWait time.Duration) gui.RestoreResult {
			return gui.NewInjector(e.CDPTrigger.CustomPort).RestoreConversationPath(targetPath, maxWait)
		},
		Inject: func(cascadeID, prompt, intentID string, timeout time.Duration) error {
			return e.CDPTrigger.TriggerDesktopContinuation(cascadeID, prompt, intentID, timeout)
		},
	}
}

func (e *Engine) executeDesktopRevival(intent *RevivalIntent) error {
	if !gui.IsValidConversationID(intent.RootConversationID) {
		return fmt.Errorf("invalid conversation ID for desktop revival: %s", intent.RootConversationID)
	}

	targetPath := "/c/" + strings.TrimPrefix(intent.RootConversationID, "/c/")

	if gui.IsTestExecution() || os.Getenv("ANTIGRAVITY_TEST_DRY_RUN") == "1" || core.IsRunningTests() {
		// Never drive a live renderer from a test/dry-run process.
		return nil
	}

	return e.runDesktopRevival(intent, targetPath, strings.TrimSpace(intent.TriggerPrompt), e.desktopRevivalDeps())
}

// runDesktopRevival is the SERIALIZED desktop continuation sequence:
//
//	phase 1 — navigate the renderer to the target conversation and wait until the
//	          target route is verified active (address bar + app-acknowledged
//	          route + mounted conversation view);
//	phase 2 — only then inject and submit the resume prompt.
//
// Why the gate exists (issue #122): the page-side restore and the prompt
// injection both drive the SPA. The address bar is rewritten by
// history.replaceState before the app has loaded the target conversation, and
// the restore's own nudges used to race the app's in-flight route transition.
// Injecting after a *failed* or merely URL-rewritten restore therefore landed the
// resume prompt in the previously rendered conversation. Refusing to inject here
// is safe: the injected in-page auto-revival loop keeps retrying (with the same
// dedupe key, so a later successful attempt still submits exactly one prompt).
//
// An empty prompt (idle conversation) only restores the route and never injects.
func (e *Engine) runDesktopRevival(intent *RevivalIntent, targetPath, prompt string, deps desktopRevivalDeps) error {
	if deps.RestoreRoute == nil {
		return fmt.Errorf("desktop revival dependencies are not configured")
	}

	// Phase 1: navigation must be settled and verified before anything is typed
	// into the composer.
	res := deps.RestoreRoute(targetPath, desktopRestoreRouteWait)
	if prompt == "" {
		// Idle conversation: the route restore is best effort and nothing is
		// injected, so an unverified route is not an error here.
		return nil
	}
	if deps.Inject == nil {
		return fmt.Errorf("desktop revival dependencies are not configured")
	}
	if !res.Verified {
		return fmt.Errorf("target conversation route %s could not be verified (reason=%s, address bar=%q, router=%q): refusing to submit the resume prompt into an unverified view",
			targetPath, res.Reason, res.LastPath, res.LastRoutePath)
	}

	// Phase 2: the target conversation is mounted and stable -> inject.
	return deps.Inject(intent.RootConversationID, prompt, intent.IntentID, 30*time.Second)
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
	// Hidden console (not DETACHED_PROCESS): agy is a console app whose node
	// helpers inherit its console — with no console they would each allocate a
	// visible window.
	return core.LaunchHiddenConsoleProcess(cmd)
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
	// code resolves to code.cmd on Windows (console batch) — same hidden-console
	// treatment as agy so its node child does not open a visible window.
	return core.LaunchHiddenConsoleProcess(cmd)
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
		prompt = e.getPrompt()
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
		MaxAttempts:        DefaultMaxAttempts,
		TTLSeconds:         DefaultTTLSeconds,
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

	e.mu.Lock()
	now := time.Now()
	// Prune expired cooldown entries
	for k, v := range e.revivedMap {
		if now.Sub(v) > RevivalCooldownDuration {
			delete(e.revivedMap, k)
		}
	}
	last := e.lastRevivedAt
	var targetRevivedAt *time.Time
	if session != nil && session.ConversationID != "" && e.revivedMap != nil {
		cleanID := strings.TrimPrefix(session.ConversationID, "/c/")
		if t, ok := e.revivedMap[cleanID]; ok {
			targetRevivedAt = &t
		}
	}
	e.mu.Unlock()

	// Enforce 10-minute cooldown expiration
	convRevivedRecently := targetRevivedAt != nil && time.Since(*targetRevivedAt) < RevivalCooldownDuration
	// Global cooldown (45s) after any revival action
	globalCooldown := last != nil && time.Since(*last) < GlobalRevivalCooldownDuration
	recentlyRevived := globalCooldown || convRevivedRecently

	if recentlyRevived && pending != nil && strings.HasPrefix(pending.IntentID, "auto-revive-") {
		_ = e.Store.ClearIntent()
		pending = nil
	}

	canRetryFailed := false
	if pending != nil && pending.Status == IntentStatusFailed {
		maxAttempts := pending.MaxAttempts
		if maxAttempts <= 0 {
			maxAttempts = DefaultMaxAttempts
		}
		if pending.Attempts < maxAttempts && time.Since(pending.CreatedAt) > 15*time.Second {
			canRetryFailed = true
		} else if time.Since(pending.CreatedAt) > 60*time.Second {
			_ = e.Store.ClearIntent()
			pending = nil
		}
	}

	// Synthesize and atomically persist intent if needed
	if !recentlyRevived && (pending == nil || pending.Resumed || canRetryFailed) && session != nil && session.NeedsRevival && session.ConversationID != "" {
		now := time.Now()
		prompt := e.getPrompt()
		pending = &RevivalIntent{
			IntentID:           fmt.Sprintf("auto-revive-%s", strings.TrimPrefix(session.ConversationID, "/c/")),
			TargetApp:          session.TargetApp,
			RootConversationID: session.ConversationID,
			ActiveSubagentID:   session.ActiveSubagentID,
			TriggerPrompt:      prompt,
			CreatedAt:          now,
			Status:             IntentStatusPending,
			CascadeID:          session.ConversationID,
			Prompt:             prompt,
			Timestamp:          now.UnixMilli(),
			Attempts:           0,
			MaxAttempts:        DefaultMaxAttempts,
			TTLSeconds:         DefaultTTLSeconds,
			Resumed:            false,
		}
		_ = e.Store.SaveIntent(pending)
	}

	var lastCopy *time.Time
	if last != nil {
		t := *last
		lastCopy = &t
	}
	success := (pending != nil && pending.Status == IntentStatusRevived) || (last != nil && time.Since(*last) < GlobalRevivalCooldownDuration)

	return &RevivalStatus{
		ActiveSession: session,
		PendingIntent: pending,
		LastRevivedAt: lastCopy,
		Success:       success,
	}, nil
}

// AcknowledgeContinuation marks the pending intent as resumed and cleans up.
func (e *Engine) AcknowledgeContinuation(cascadeID string) error {
	cleanedTarget := strings.TrimPrefix(strings.TrimSpace(cascadeID), "/c/")
	if cleanedTarget == "" {
		return fmt.Errorf("cascadeID cannot be empty")
	}

	intent, err := e.Store.LoadIntent()
	if err != nil {
		return err
	}
	if intent == nil {
		return nil
	}

	cleanedRoot := strings.TrimPrefix(strings.TrimSpace(intent.RootConversationID), "/c/")
	cleanedCascade := strings.TrimPrefix(strings.TrimSpace(intent.CascadeID), "/c/")
	if cleanedTarget != cleanedRoot && cleanedTarget != cleanedCascade {
		return fmt.Errorf("acknowledged cascade ID %q does not match active intent %q", cleanedTarget, cleanedRoot)
	}

	now := time.Now()
	e.mu.Lock()
	e.lastRevivedAt = &now
	if e.revivedMap == nil {
		e.revivedMap = make(map[string]time.Time)
	}
	e.revivedMap[cleanedTarget] = now
	e.mu.Unlock()

	intent.Resumed = true
	intent.Status = IntentStatusRevived
	_ = e.Store.ClearIntent()
	return nil
}
