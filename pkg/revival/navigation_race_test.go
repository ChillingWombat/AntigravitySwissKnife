package revival

import (
	"strings"
	"testing"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
)

// navRestoreCall records one RestoreRoute invocation of the fake navigator.
type navRestoreCall struct {
	targetPath string
	maxWait    time.Duration
}

// navInjectCall records one Inject invocation of the fake navigator.
type navInjectCall struct {
	cascadeID string
	prompt    string
	intentID  string
	timeout   time.Duration
}

// fakeDesktopNavigator is the deterministic stand-in for the live
// RestoreConversationPath / TriggerDesktopContinuation pair used by the desktop
// revival sequence (issue #122).
type fakeDesktopNavigator struct {
	restoreResult gui.RestoreResult
	restoreCalls  []navRestoreCall
	injectCalls   []navInjectCall
	// order records the interleaving of both phases, so the test can assert the
	// sequence is serialized (restore completes before injection starts).
	order []string
}

func (f *fakeDesktopNavigator) deps() desktopRevivalDeps {
	return desktopRevivalDeps{
		RestoreRoute: func(targetPath string, maxWait time.Duration) gui.RestoreResult {
			f.restoreCalls = append(f.restoreCalls, navRestoreCall{targetPath: targetPath, maxWait: maxWait})
			f.order = append(f.order, "restore")
			return f.restoreResult
		},
		Inject: func(cascadeID, prompt, intentID string, timeout time.Duration) error {
			f.injectCalls = append(f.injectCalls, navInjectCall{cascadeID: cascadeID, prompt: prompt, intentID: intentID, timeout: timeout})
			f.order = append(f.order, "inject")
			return nil
		},
	}
}

func newNavRaceIntent() *RevivalIntent {
	return &RevivalIntent{
		IntentID:           "intent-nav-race",
		TargetApp:          "desktop",
		RootConversationID: "aabbccdd-1122-3344-5566-778899aabbcc",
		TriggerPrompt:      DefaultTriggerPrompt,
		Status:             IntentStatusPending,
	}
}

// TestRunDesktopRevival_RefusesInjectionWhenRouteNotVerified is the core
// regression guard for issue #122.
//
// Before the fix the resume prompt was injected immediately after the restore
// call — even when the restore failed — while the page-side restore could have
// "verified" a bare address-bar rewrite (history.replaceState) that the app never
// routed to. The prompt then landed in the previously rendered conversation.
// Injection must be gated on a VERIFIED route.
func TestRunDesktopRevival_RefusesInjectionWhenRouteNotVerified(t *testing.T) {
	intent := newNavRaceIntent()
	targetPath := "/c/" + intent.RootConversationID

	nav := &fakeDesktopNavigator{
		restoreResult: gui.RestoreResult{
			Verified:      false,
			Reason:        gui.RestoreReasonTimeout,
			LastPath:      "/c/ffffffff-ffff-ffff-ffff-ffffffffffff",
			LastRoutePath: "/c/ffffffff-ffff-ffff-ffff-ffffffffffff",
		},
	}

	engine := NewEngine(t.TempDir(), 0)
	err := engine.runDesktopRevival(intent, targetPath, intent.TriggerPrompt, nav.deps())
	if err == nil {
		t.Fatalf("expected an error when the target route cannot be verified, got nil — the resume prompt must never be submitted into an unverified view")
	}
	if len(nav.injectCalls) != 0 {
		t.Fatalf("injection must not run after an unverified restore, got %d inject call(s)", len(nav.injectCalls))
	}
	if len(nav.restoreCalls) != 1 {
		t.Fatalf("expected exactly one restore attempt, got %d", len(nav.restoreCalls))
	}
	if nav.restoreCalls[0].targetPath != targetPath {
		t.Errorf("restore target path = %q, want %q", nav.restoreCalls[0].targetPath, targetPath)
	}
	// The error must carry the navigation diagnostics for the operator/logs.
	for _, want := range []string{targetPath, gui.RestoreReasonTimeout, "/c/ffffffff-ffff-ffff-ffff-ffffffffffff"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected error to mention %q, got %q", want, err.Error())
		}
	}
}

// TestRunDesktopRevival_InjectsOnlyAfterVerifiedRestore asserts the serialized
// ordering and the exact injection arguments: the prompt is submitted once, with
// the intent's cascade ID / intent ID (the shared in-page dedupe key), after the
// route verification has completed.
func TestRunDesktopRevival_InjectsOnlyAfterVerifiedRestore(t *testing.T) {
	intent := newNavRaceIntent()
	targetPath := "/c/" + intent.RootConversationID

	nav := &fakeDesktopNavigator{
		restoreResult: gui.RestoreResult{Verified: true, Reason: gui.RestoreReasonVerified, LastPath: targetPath, LastRoutePath: targetPath},
	}

	engine := NewEngine(t.TempDir(), 0)
	if err := engine.runDesktopRevival(intent, targetPath, intent.TriggerPrompt, nav.deps()); err != nil {
		t.Fatalf("runDesktopRevival failed on a verified route: %v", err)
	}

	if len(nav.order) != 2 || nav.order[0] != "restore" || nav.order[1] != "inject" {
		t.Fatalf("expected the sequence to be serialized as [restore inject], got %v", nav.order)
	}
	if len(nav.injectCalls) != 1 {
		t.Fatalf("expected exactly one injection, got %d", len(nav.injectCalls))
	}
	got := nav.injectCalls[0]
	if got.cascadeID != intent.RootConversationID {
		t.Errorf("inject cascadeID = %q, want %q", got.cascadeID, intent.RootConversationID)
	}
	if got.prompt != DefaultTriggerPrompt {
		t.Errorf("inject prompt = %q, want %q", got.prompt, DefaultTriggerPrompt)
	}
	if got.intentID != intent.IntentID {
		t.Errorf("inject intentID = %q, want %q (the shared in-page dedupe key)", got.intentID, intent.IntentID)
	}
	if got.timeout <= 0 {
		t.Errorf("inject timeout must be positive, got %v", got.timeout)
	}
}

// TestRunDesktopRevival_IdleConversationRestoresRouteWithoutInjecting pins the
// preserved idle-conversation semantics: the route is restored (best effort) and
// no prompt is submitted, even when the route could not be verified.
func TestRunDesktopRevival_IdleConversationRestoresRouteWithoutInjecting(t *testing.T) {
	intent := newNavRaceIntent()
	targetPath := "/c/" + intent.RootConversationID

	nav := &fakeDesktopNavigator{
		restoreResult: gui.RestoreResult{Verified: false, Reason: gui.RestoreReasonTimeout},
	}

	engine := NewEngine(t.TempDir(), 0)
	if err := engine.runDesktopRevival(intent, targetPath, "", nav.deps()); err != nil {
		t.Fatalf("idle conversation must not fail when the route is unverified, got %v", err)
	}
	if len(nav.restoreCalls) != 1 {
		t.Fatalf("expected the idle path to attempt a route restore, got %d calls", len(nav.restoreCalls))
	}
	if len(nav.injectCalls) != 0 {
		t.Fatalf("idle conversation must never inject a prompt, got %d calls", len(nav.injectCalls))
	}
}

// TestExecuteDesktopRevival_TargetPathNormalisation covers the path the live
// engine builds before handing it to the navigator, including a "/c/"-prefixed
// conversation ID and the invalid-ID guard (which must not touch the navigator).
func TestExecuteDesktopRevival_TargetPathNormalisation(t *testing.T) {
	engine := NewEngine(t.TempDir(), 0)

	intent := newNavRaceIntent()
	intent.RootConversationID = "/c/" + intent.RootConversationID
	if err := engine.executeDesktopRevival(intent); err != nil {
		t.Fatalf("executeDesktopRevival must stay side-effect free in test mode, got %v", err)
	}

	invalid := newNavRaceIntent()
	invalid.RootConversationID = "not a valid id"
	if err := engine.executeDesktopRevival(invalid); err == nil {
		t.Fatalf("expected an invalid conversation ID to be rejected")
	}
}
