package revival

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
)

// Engine orchestrates active conversation and subagent revival across app restarts and account switches.
type Engine struct {
	Store      *Store
	Detector   *SessionDetector
	CDPTrigger *CDPTrigger
	mu         sync.RWMutex
	// lastRevivedAt drives the global 45s cooldown.
	lastRevivedAt *time.Time
	// revivedMap drives the per-conversation 10-minute cooldown.
	revivedMap map[string]time.Time
	// cooldownPath is where revivedMap/lastRevivedAt persist, so a daemon
	// restart does not re-arm conversations that were already revived.
	cooldownPath string
}

// cooldownFile is the on-disk cooldown state.
type cooldownFile struct {
	LastRevivedAt *time.Time           `json:"last_revived_at,omitempty"`
	Conversations map[string]time.Time `json:"conversations,omitempty"`
}

// CooldownFileName is the persisted cooldown filename inside the config dir.
const CooldownFileName = "revival_cooldowns.json"

// NewEngine creates a new Engine instance.
func NewEngine(configDir string, customCDPPort int) *Engine {
	store := NewStore(configDir)
	e := &Engine{
		Store:        store,
		Detector:     NewSessionDetector(),
		CDPTrigger:   NewCDPTrigger(customCDPPort),
		revivedMap:   make(map[string]time.Time),
		cooldownPath: filepath.Join(store.dir, CooldownFileName),
	}
	e.loadCooldowns()
	return e
}

// loadCooldowns restores the cooldown state persisted by a previous process.
// A missing file simply means nothing was revived yet.
func (e *Engine) loadCooldowns() {
	if e.cooldownPath == "" {
		return
	}
	data, err := os.ReadFile(e.cooldownPath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "revival: failed to read cooldown state %s: %v\n", e.cooldownPath, err)
		}
		return
	}

	var state cooldownFile
	if err := json.Unmarshal(data, &state); err != nil {
		fmt.Fprintf(os.Stderr, "revival: failed to parse cooldown state %s: %v\n", e.cooldownPath, err)
		return
	}

	now := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()
	for convID, at := range state.Conversations {
		if now.Sub(at) <= RevivalCooldownDuration {
			e.revivedMap[convID] = at
		}
	}
	if state.LastRevivedAt != nil && now.Sub(*state.LastRevivedAt) <= RevivalCooldownDuration {
		last := *state.LastRevivedAt
		e.lastRevivedAt = &last
	}
}

// saveCooldowns persists the cooldown state under the engine lock. Callers must
// hold e.mu so the snapshot cannot tear.
func (e *Engine) saveCooldownsLocked() {
	if e.cooldownPath == "" {
		return
	}
	state := cooldownFile{Conversations: make(map[string]time.Time, len(e.revivedMap))}
	for convID, at := range e.revivedMap {
		state.Conversations[convID] = at
	}
	if e.lastRevivedAt != nil {
		last := *e.lastRevivedAt
		state.LastRevivedAt = &last
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "revival: failed to serialize cooldown state: %v\n", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(e.cooldownPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "revival: failed to create cooldown directory: %v\n", err)
		return
	}
	tmpPath := fmt.Sprintf("%s.%d.tmp", e.cooldownPath, time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "revival: failed to write cooldown state: %v\n", err)
		return
	}
	if err := os.Rename(tmpPath, e.cooldownPath); err != nil {
		_ = os.Remove(tmpPath)
		fmt.Fprintf(os.Stderr, "revival: failed to commit cooldown state: %v\n", err)
	}
}

// markRevived records a revival for the global and per-conversation cooldowns
// and persists them.
func (e *Engine) markRevived(convID string) {
	now := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lastRevivedAt = &now
	if e.revivedMap == nil {
		e.revivedMap = make(map[string]time.Time)
	}
	if cleaned := cleanConversationID(convID); cleaned != "" {
		e.revivedMap[cleaned] = now
	}
	e.saveCooldownsLocked()
}

// isSuppressed reports whether convID was revived too recently to be re-armed.
func (e *Engine) isSuppressed(convID string) bool {
	cleaned := cleanConversationID(convID)
	if cleaned == "" {
		return false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	at, ok := e.revivedMap[cleaned]
	if !ok {
		return false
	}
	return time.Since(at) < RevivalCooldownDuration
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

// CapturePreSwitchState captures the primary (newest) interrupted conversation
// state before a credential change or app restart, persists its RevivalIntent,
// and pins the layout in app_storage.json.
//
// Delegates to CapturePreSwitchIntents so the single-intent callers keep working
// while every interrupted conversation is still recorded.
func (e *Engine) CapturePreSwitchState(targetApp string) (*RevivalIntent, error) {
	intents, err := e.CapturePreSwitchIntents(targetApp)
	if len(intents) > 0 {
		return intents[0], nil
	}
	return nil, err
}

// CapturePreSwitchIntents captures every genuinely interrupted conversation
// before a credential change or app restart and persists one RevivalIntent per
// conversation, newest first.
//
// A prompt is only attached to a conversation the persisted signals mark as
// interrupted; everything else is recorded with a blank prompt, which restores
// the route without injecting anything.
func (e *Engine) CapturePreSwitchIntents(targetApp string) ([]*RevivalIntent, error) {
	sessions, detectErr := e.Detector.DetectAll(targetApp)

	// The single most relevant session is still captured even when nothing is
	// interrupted: the desktop restore needs to know which conversation to
	// reopen, and it does that with a blank prompt.
	primary, primaryErr := e.Detector.Detect(targetApp)
	if primaryErr != nil && detectErr != nil {
		return nil, detectErr
	}

	seen := make(map[string]bool, len(sessions)+1)
	intents := make([]*RevivalIntent, 0, len(sessions)+1)
	var firstErr error

	appendSession := func(session *ActiveSessionInfo) {
		if session == nil || session.ConversationID == "" {
			return
		}
		key := cleanConversationID(session.ConversationID)
		if seen[key] {
			return
		}
		intent, buildErr := e.buildIntent(targetApp, session)
		if buildErr != nil {
			if firstErr == nil {
				firstErr = buildErr
			}
			return
		}
		if intent == nil {
			// Already revived within the cooldown window.
			seen[key] = true
			return
		}
		if saveErr := e.Store.SaveIntent(intent); saveErr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to persist revival intent: %w", saveErr)
			}
			return
		}
		seen[key] = true
		intents = append(intents, intent)
	}

	for _, session := range sessions {
		appendSession(session)
	}
	if primary != nil {
		appendSession(primary)
	}

	if len(intents) > 0 {
		// Guarantee that app_storage.json has last conversation path set.
		targetPath := intents[0].RootConversationID
		if primary != nil && primary.TargetPath != "" {
			targetPath = primary.TargetPath
		} else {
			targetPath = "/c/" + targetPath
		}
		gui.SaveLastConversationPath(targetPath)
		return intents, nil
	}

	if firstErr != nil {
		return nil, firstErr
	}
	if primaryErr != nil {
		return nil, primaryErr
	}
	return nil, fmt.Errorf("no conversation detected to preserve across the switch")
}

// buildIntent turns one detected session into a persistable intent, falling back
// to the pinned/latest conversation path when the detected ID is unusable.
func (e *Engine) buildIntent(targetApp string, session *ActiveSessionInfo) (*RevivalIntent, error) {
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

	// An already-revived conversation must not be re-armed, even across a restart.
	if e.isSuppressed(session.ConversationID) {
		return nil, nil
	}

	now := time.Now()
	prompt := e.getPrompt()
	if !session.NeedsRevival {
		prompt = ""
	}

	return &RevivalIntent{
		IntentID:           fmt.Sprintf("intent-%d-%s", now.UnixNano(), cleanConversationID(session.ConversationID)),
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
	}, nil
}

// ExecutePostRelaunchRevival executes prompt injection and navigation continuation
// after IDE relaunch.
//
// When intent is nil every pending intent is revived, one per interrupted
// conversation. An explicit intent revives just that conversation.
func (e *Engine) ExecutePostRelaunchRevival(intent *RevivalIntent) error {
	if intent != nil {
		return e.reviveIntent(intent)
	}

	intents, err := e.Store.LoadIntents()
	if err != nil {
		return err
	}
	if len(intents) == 0 {
		return nil
	}

	errs := make([]error, 0, len(intents))
	for _, pending := range intents {
		if reviveErr := e.reviveIntent(pending); reviveErr != nil {
			errs = append(errs, reviveErr)
		}
	}
	return errors.Join(errs...)
}

// reviveIntent performs one conversation's revival and clears only that intent.
func (e *Engine) reviveIntent(intent *RevivalIntent) error {
	if intent == nil {
		return nil
	}
	if intent.Resumed || intent.Status == IntentStatusRevived {
		return nil
	}

	maxAttempts := intent.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = DefaultMaxAttempts
	}
	if intent.Attempts >= maxAttempts {
		if err := e.Store.UpdateStatus(intent.IntentID, IntentStatusFailed); err != nil {
			return err
		}
		return fmt.Errorf("max revival attempts (%d) reached for intent %s", maxAttempts, intent.IntentID)
	}

	if err := e.Store.UpdateStatus(intent.IntentID, IntentStatusReviving); err != nil {
		return err
	}
	targetApp := strings.ToLower(strings.TrimSpace(intent.TargetApp))

	var err error
	switch targetApp {
	case "agy":
		err = e.executeCLIRevival(intent)
	case "vscode":
		err = e.executeVSCodeRevival(intent)
	default: // desktop or all
		err = e.executeDesktopRevival(intent)
	}

	if err != nil {
		attempts, incErr := e.Store.IncrementAttempts(intent.IntentID)
		if incErr != nil {
			return errors.Join(err, incErr)
		}
		intent.Attempts = attempts
		if updateErr := e.Store.UpdateStatus(intent.IntentID, IntentStatusFailed, attempts); updateErr != nil {
			return errors.Join(err, updateErr)
		}
		return err
	}

	e.markRevived(intent.RootConversationID)

	intent.Resumed = true
	if err := e.Store.UpdateStatus(intent.IntentID, IntentStatusRevived); err != nil {
		return err
	}
	// Scoped: reviving one conversation must never discard the others.
	return e.Store.ClearIntentFor(intent.RootConversationID)
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

	cmd := core.CommandForExecutable(binPath, "--conversation", convID, "--continue")
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
	cmd := core.CommandForExecutable(codePath, "--open-url", uri)
	// code resolves to code.cmd on Windows (console batch) — same hidden-console
	// treatment as agy so its node child does not open a visible window.
	return core.LaunchHiddenConsoleProcess(cmd)
}

// GetRevivalStatus retrieves the active session, every live pending revival
// intent, and the last revived timestamp.
//
// PendingIntents is the authoritative list: an account switch that interrupted N
// conversations reports N intents, each of which must be dispatched on its own.
func (e *Engine) GetRevivalStatus(targetConvID ...string) (*RevivalStatus, error) {
	target := ""
	if len(targetConvID) > 0 {
		target = cleanConversationID(targetConvID[0])
	}

	var session *ActiveSessionInfo
	if target != "" {
		session, _ = e.Detector.DetectForID(target)
	} else {
		session, _ = e.Detector.Detect("")
	}

	e.pruneCooldowns()

	intents, err := e.Store.LoadIntents()
	if err != nil {
		return nil, err
	}

	intents, err = e.dropExhaustedFailedIntents(intents)
	if err != nil {
		return nil, err
	}

	intents, err = e.synthesizeMissingIntents(intents, session, target)
	if err != nil {
		return nil, err
	}

	// Only surface intents that are safe to dispatch right now. A conversation
	// revived within the cooldown window must never be handed to a dispatcher
	// again, and the global window keeps a burst of intents from firing at once.
	var pendingIntents []*RevivalIntent
	for _, intent := range intents {
		if intent == nil {
			continue
		}
		if e.isSuppressed(intent.RootConversationID) {
			continue
		}
		pendingIntents = append(pendingIntents, intent)
	}
	if e.globalCooldownActive() {
		pendingIntents = nil
	}

	var pending *RevivalIntent
	// The legacy single field stays scoped to the queried conversation: a caller
	// asking about conversation X must not be handed conversation Y's intent just
	// because Y happens to be the newest interrupted one.
	for _, candidate := range pendingIntents {
		if target == "" || cleanConversationID(candidate.RootConversationID) == target {
			pending = candidate
			break
		}
	}

	e.mu.RLock()
	last := e.lastRevivedAt
	e.mu.RUnlock()

	var lastCopy *time.Time
	if last != nil {
		t := *last
		lastCopy = &t
	}
	success := (pending != nil && pending.Status == IntentStatusRevived) ||
		(last != nil && time.Since(*last) < GlobalRevivalCooldownDuration)

	return &RevivalStatus{
		ActiveSession:  session,
		PendingIntents: pendingIntents,
		PendingIntent:  pending,
		LastRevivedAt:  lastCopy,
		Success:        success,
	}, nil
}

// pruneCooldowns drops per-conversation cooldown entries older than the window
// and persists the pruning so a restart sees the same state.
func (e *Engine) pruneCooldowns() {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	pruned := false
	for convID, at := range e.revivedMap {
		if now.Sub(at) > RevivalCooldownDuration {
			delete(e.revivedMap, convID)
			pruned = true
		}
	}
	if e.lastRevivedAt != nil && now.Sub(*e.lastRevivedAt) > RevivalCooldownDuration {
		e.lastRevivedAt = nil
		pruned = true
	}
	if pruned {
		e.saveCooldownsLocked()
	}
}

// globalCooldownActive reports whether the 45s global revival window is open.
func (e *Engine) globalCooldownActive() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastRevivedAt != nil && time.Since(*e.lastRevivedAt) < GlobalRevivalCooldownDuration
}

// dropExhaustedFailedIntents removes intents that ran out of attempts and are
// older than a minute, so a permanently failing conversation stops resurfacing.
func (e *Engine) dropExhaustedFailedIntents(intents []*RevivalIntent) ([]*RevivalIntent, error) {
	kept := make([]*RevivalIntent, 0, len(intents))
	for _, intent := range intents {
		if intent == nil {
			continue
		}
		if intent.Status == IntentStatusFailed {
			maxAttempts := intent.MaxAttempts
			if maxAttempts <= 0 {
				maxAttempts = DefaultMaxAttempts
			}
			exhausted := intent.Attempts >= maxAttempts
			if exhausted && time.Since(intent.CreatedAt) > 60*time.Second {
				if err := e.Store.ClearIntentFor(intent.RootConversationID); err != nil {
					return nil, err
				}
				continue
			}
		}
		kept = append(kept, intent)
	}
	return kept, nil
}

// synthesizeMissingIntents creates a pending intent for every interrupted
// conversation that does not already have one.
//
// Synthesis is gated on the persisted interruption signals (DetectAll already
// filtered to NeedsRevival) and on the cooldown windows, so a finished
// conversation is never given a prompt just for being open.
func (e *Engine) synthesizeMissingIntents(intents []*RevivalIntent, session *ActiveSessionInfo, target string) ([]*RevivalIntent, error) {
	if e.globalCooldownActive() {
		return intents, nil
	}

	have := make(map[string]bool, len(intents))
	for _, intent := range intents {
		if intent == nil {
			continue
		}
		if intent.Status == IntentStatusRevived || intent.Resumed {
			continue
		}
		have[cleanConversationID(intent.RootConversationID)] = true
	}

	candidates, err := e.Detector.DetectAll(sessionTargetApp(session))
	if err != nil {
		return nil, err
	}
	// The explicitly queried conversation is also a candidate: it may be the one
	// the caller is looking at while it is not the newest interrupted row.
	if target != "" && (session == nil || cleanConversationID(session.ConversationID) != target) {
		if info, idErr := e.Detector.DetectForID(target); idErr == nil && info != nil && info.NeedsRevival {
			candidates = append(candidates, info)
		}
	}

	for _, info := range candidates {
		if info == nil || !info.NeedsRevival {
			continue
		}
		convID := cleanConversationID(info.ConversationID)
		if convID == "" || have[convID] || e.isSuppressed(convID) {
			continue
		}

		now := time.Now()
		prompt := e.getPrompt()
		intent := &RevivalIntent{
			// Deterministic ID: every pass over the same conversation derives the
			// same in-page dedupe key, so a repeated status poll cannot produce a
			// second prompt through a different key.
			IntentID:           "auto-revive-" + convID,
			TargetApp:          info.TargetApp,
			RootConversationID: convID,
			ActiveSubagentID:   info.ActiveSubagentID,
			TriggerPrompt:      prompt,
			CreatedAt:          now,
			Status:             IntentStatusPending,
			CascadeID:          convID,
			Prompt:             prompt,
			Timestamp:          now.UnixMilli(),
			Attempts:           0,
			MaxAttempts:        DefaultMaxAttempts,
			TTLSeconds:         DefaultTTLSeconds,
			Resumed:            false,
		}
		if err := e.Store.SaveIntent(intent); err != nil {
			return nil, fmt.Errorf("failed to persist synthesized revival intent: %w", err)
		}
		have[convID] = true
		intents = append(intents, intent)
	}

	sortIntentsNewestFirst(intents)
	return intents, nil
}

// sessionTargetApp returns the app type to enumerate for, defaulting to desktop.
func sessionTargetApp(session *ActiveSessionInfo) string {
	if session != nil && strings.TrimSpace(session.TargetApp) != "" {
		return session.TargetApp
	}
	return "desktop"
}

// AcknowledgeContinuation marks one conversation's pending intent as resumed and
// removes only that intent; the other pending conversations stay queued.
func (e *Engine) AcknowledgeContinuation(cascadeID string) error {
	cleanedTarget := cleanConversationID(cascadeID)
	if cleanedTarget == "" {
		return fmt.Errorf("cascadeID cannot be empty")
	}

	intents, err := e.Store.LoadIntents()
	if err != nil {
		return err
	}

	matched := false
	for _, intent := range intents {
		if intent == nil {
			continue
		}
		cleanedRoot := cleanConversationID(intent.RootConversationID)
		cleanedCascade := cleanConversationID(intent.CascadeID)
		if cleanedTarget == cleanedRoot || cleanedTarget == cleanedCascade {
			matched = true
			break
		}
	}
	if len(intents) > 0 && !matched {
		return fmt.Errorf("acknowledged cascade ID %q does not match any pending intent", cleanedTarget)
	}

	e.markRevived(cleanedTarget)
	return e.Store.ClearIntentFor(cleanedTarget)
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
		// Deterministic per conversation: a repeated manual revive of the same
		// conversation reuses one in-page dedupe key instead of minting a new one
		// that would submit the prompt a second time.
		IntentID:           "manual-revive-" + cleanConversationID(conversationID),
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
