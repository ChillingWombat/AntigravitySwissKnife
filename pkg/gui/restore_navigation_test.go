package gui

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"testing"
)

// ---------------------------------------------------------------------------
// RestoreConversationPath page-side navigation contract (issue #122)
//
// These tests execute the real generated script (restoreConversationPathScript)
// against a mocked renderer, so the navigation policy is pinned without a live
// IDE:
//   - the address bar is never "verified" while the app's router points
//     somewhere else (a history.replaceState rewrite is not a navigation);
//   - the script never fabricates a route when the app exposes its router, it
//     navigates through real primitives instead;
//   - while the route is already acknowledged it waits for the view to mount
//     instead of firing a second, competing navigation.
// ---------------------------------------------------------------------------

type restoreScriptRun struct {
	Res   map[string]interface{} `json:"res"`
	Calls map[string]interface{} `json:"calls"`
}

// runRestorePathScript evaluates the generated restore script in Node with the
// given mock page definition (which must populate window/document and may record
// into the harness-provided __calls object).
func runRestorePathScript(t *testing.T, targetPath, mockJS string) restoreScriptRun {
	t.Helper()
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skipf("node not available: %v", err)
	}

	script := restoreConversationPathScript(targetPath)
	harness := `
const assert = require('assert');
const __calls = { click: 0, navigate: [], replaceState: [], assign: [] };
` + mockJS + `
const res = ` + script + `;
console.log(JSON.stringify({ res, calls: __calls }));
`

	cmd := exec.Command(nodePath, "-e", harness)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node failed: %v\noutput:\n%s", err, string(out))
	}
	var run restoreScriptRun
	if err := json.Unmarshal(bytes.TrimSpace(out), &run); err != nil {
		t.Fatalf("failed to decode node output: %v\noutput:\n%s", err, string(out))
	}
	return run
}

func boolField(t *testing.T, m map[string]interface{}, key string) bool {
	t.Helper()
	v, ok := m[key].(bool)
	if !ok {
		t.Fatalf("field %q missing or not a bool in %v", key, m)
	}
	return v
}

func stringField(t *testing.T, m map[string]interface{}, key string) string {
	t.Helper()
	v, ok := m[key].(string)
	if !ok {
		t.Fatalf("field %q missing or not a string in %v", key, m)
	}
	return v
}

// TestRestoreConversationPathScript_NeverFabricatesRouteWithRouter: when the app
// exposes its router and the router is on a different route, the script must
// navigate through the router and must NOT rewrite the address bar with
// history.replaceState (which would make the URL claim a conversation the app
// never loaded — the root cause of the wrong-conversation injection in #122).
func TestRestoreConversationPathScript_NeverFabricatesRouteWithRouter(t *testing.T) {
	target := "/c/bbbbbbbb-5555-6666-7777-888888888888"
	other := "/c/aaaaaaaa-1111-2222-3333-444444444444"
	run := runRestorePathScript(t, target, `
const __state = { pathname: "`+other+`", search: "" };
global.window = {
  location: { get pathname() { return __state.pathname; }, get search() { return __state.search; } },
  __TSR_ROUTER__: {
    state: { location: { get pathname() { return __state.pathname; } } },
    navigate: (opts) => { __calls.navigate.push(opts || {}); __state.pathname = (opts && opts.to) ? String(opts.to).split("?")[0] : __state.pathname; }
  },
  history: { state: { __TSR_key: "k1" }, replaceState: (s, u, url) => { __calls.replaceState.push(url); __state.pathname = String(url); } },
  __swissLastRestoreNudge: 0
};
global.document = {
  getElementById: (id) => (id === "root" ? { childElementCount: 1 } : null),
  querySelector: (sel) => {
    if (sel.indexOf("conversation-view") !== -1) return null;
    if (sel.indexOf("new-conversation-button") !== -1 || sel.indexOf("conversation-list-sidebar") !== -1) return {};
    if (sel.indexOf('a[href*=') === 0) return null;
    return null;
  }
};
`)

	if boolField(t, run.Res, "ready") {
		t.Errorf("a route the app's router never acknowledged must not be ready: %v", run.Res)
	}
	if got := stringField(t, run.Res, "action"); got != "router_navigate" {
		t.Errorf("action = %q, want router_navigate (real navigation primitive)", got)
	}
	if n := len(run.Calls["navigate"].([]interface{})); n != 1 {
		t.Errorf("expected exactly 1 router navigation, got %d (%v)", n, run.Calls["navigate"])
	}
	if n := len(run.Calls["replaceState"].([]interface{})); n != 0 {
		t.Errorf("history.replaceState must never be used while the app exposes a router, got %d call(s): %v", n, run.Calls["replaceState"])
	}
}

// TestRestoreConversationPathScript_WaitsWhenRouteAlreadyAcknowledged: when the
// address bar and the router already agree, the app is mounting the conversation
// view right now. Firing another navigation here is what raced the app's own
// route transition, so the script must wait instead.
func TestRestoreConversationPathScript_WaitsWhenRouteAlreadyAcknowledged(t *testing.T) {
	target := "/c/bbbbbbbb-5555-6666-7777-888888888888"
	run := runRestorePathScript(t, target, `
const __state = { pathname: "`+target+`", search: "" };
global.window = {
  location: { get pathname() { return __state.pathname; }, get search() { return __state.search; } },
  __TSR_ROUTER__: {
    state: { location: { get pathname() { return __state.pathname; } } },
    navigate: (opts) => { __calls.navigate.push(opts || {}); }
  },
  history: { state: { __TSR_key: "k1" }, replaceState: (s, u, url) => { __calls.replaceState.push(url); } },
  __swissLastRestoreNudge: Date.now() - 5000
};
global.document = {
  getElementById: (id) => (id === "root" ? { childElementCount: 1 } : null),
  querySelector: (sel) => {
    // conversation-view intentionally NOT mounted yet: the app is transitioning
    if (sel.indexOf("conversation-view") !== -1) return null;
    if (sel.indexOf("new-conversation-button") !== -1 || sel.indexOf("conversation-list-sidebar") !== -1) return {};
    if (sel.indexOf('a[href*=') === 0) return { click: () => { __calls.click++; } };
    return null;
  }
};
`)

	if boolField(t, run.Res, "ready") {
		t.Errorf("the view is not mounted yet, so the route must not be reported ready: %v", run.Res)
	}
	if got := stringField(t, run.Res, "action"); got != "wait_for_view" {
		t.Errorf("action = %q, want wait_for_view (never re-nudge an in-flight transition)", got)
	}
	if n, _ := run.Calls["click"].(float64); n != 0 {
		t.Errorf("the sidebar link must not be clicked while the app is completing the transition, got %v click(s)", n)
	}
	if n := len(run.Calls["navigate"].([]interface{})); n != 0 {
		t.Errorf("no router navigation must be issued while the app is completing the transition, got %d", n)
	}
	if n := len(run.Calls["replaceState"].([]interface{})); n != 0 {
		t.Errorf("no address-bar rewrite must be issued while the app is completing the transition, got %d", n)
	}
}

// TestRestoreConversationPathScript_ReadinessRequiresRouteAcknowledgement pins
// the verification rule itself: a mounted conversation view plus a matching
// address bar is NOT enough when the router says the app is somewhere else.
func TestRestoreConversationPathScript_ReadinessRequiresRouteAcknowledgement(t *testing.T) {
	target := "/c/bbbbbbbb-5555-6666-7777-888888888888"

	mockWithRouter := func(routerPath string, viewMounted bool) string {
		view := "null"
		if viewMounted {
			view = "{}"
		}
		return `
global.window = {
  location: { pathname: "` + target + `", search: "" },
  __TSR_ROUTER__: {
    state: { location: { pathname: "` + routerPath + `" } },
    navigate: (opts) => { __calls.navigate.push(opts || {}); }
  },
  history: { state: { __TSR_key: "k1" }, replaceState: (s, u, url) => { __calls.replaceState.push(url); } },
  __swissLastRestoreNudge: Date.now() - 5000
};
global.document = {
  getElementById: (id) => (id === "root" ? { childElementCount: 1 } : null),
  querySelector: (sel) => {
    if (sel.indexOf("conversation-view") !== -1) return ` + view + `;
    if (sel.indexOf("new-conversation-button") !== -1 || sel.indexOf("conversation-list-sidebar") !== -1) return {};
    return null;
  }
};
`
	}

	// A. Address bar matches, view mounted, but the router never routed there:
	//    this is exactly the state a history.replaceState rewrite leaves behind.
	conflicted := runRestorePathScript(t, target, mockWithRouter("/", true))
	if boolField(t, conflicted.Res, "ready") {
		t.Errorf("address bar + mounted view must NOT be ready while the router is on another route: %v", conflicted.Res)
	}
	if got := stringField(t, conflicted.Res, "route_path"); got != "/" {
		t.Errorf("route_path = %q, want the router's acknowledged path %q", got, "/")
	}
	if boolField(t, conflicted.Res, "router_observable") != true {
		t.Errorf("router_observable must be reported for diagnostics: %v", conflicted.Res)
	}

	// B. Address bar, router and view all agree -> ready.
	agreed := runRestorePathScript(t, target, mockWithRouter(target, true))
	if !boolField(t, agreed.Res, "ready") {
		t.Errorf("a fully acknowledged target route must be ready: %v", agreed.Res)
	}
	if got := stringField(t, agreed.Res, "action"); got != "none" {
		t.Errorf("action = %q, want none once the route is verified", got)
	}

	// C. No router exposed (fresh start / onboarding): the address bar + mounted
	//    view remain the best available signals, so readiness is preserved.
	noRouter := runRestorePathScript(t, target, `
global.window = {
  location: { pathname: "`+target+`", search: "" },
  history: { state: { __TSR_key: "k1" }, replaceState: (s, u, url) => { __calls.replaceState.push(url); } },
  __swissLastRestoreNudge: Date.now() - 5000
};
global.document = {
  getElementById: (id) => (id === "root" ? { childElementCount: 1 } : null),
  querySelector: (sel) => {
    if (sel.indexOf("conversation-view") !== -1) return {};
    if (sel.indexOf("new-conversation-button") !== -1 || sel.indexOf("conversation-list-sidebar") !== -1) return {};
    return null;
  }
};
`)
	if !boolField(t, noRouter.Res, "ready") {
		t.Errorf("without an observable router the address bar + mounted view must stay sufficient: %v", noRouter.Res)
	}
	if boolField(t, noRouter.Res, "router_observable") {
		t.Errorf("router_observable must be false without a router: %v", noRouter.Res)
	}
	if n := len(noRouter.Calls["replaceState"].([]interface{})); n != 0 {
		t.Errorf("no bootstrap rewrite may happen once the view is mounted, got %d", n)
	}
}

// TestRestoreConversationPathScript_ExplicitUserRouteSurvivesRestore pins the
// post-error recovery contract: once the user chooses a sidebar conversation,
// an older startup restore target must not pull the host back to the failed
// Untitled Conversation. The router acknowledgement remains the navigation
// primitive when no direct user intent is present.
func TestRestoreConversationPathScript_ExplicitUserRouteSurvivesRestore(t *testing.T) {
	target := "/c/bbbbbbbb-5555-6666-7777-888888888888"
	userRoute := "/c/aaaaaaaa-1111-2222-3333-444444444444"
	run := runRestorePathScript(t, target, `
const __state = { pathname: "`+userRoute+`", search: "" };
global.window = {
  location: { get pathname() { return __state.pathname; }, get search() { return __state.search; }, assign: (url) => { __calls.assign.push(url); } },
  __TSR_ROUTER__: {
    state: { location: { get pathname() { return __state.pathname; } } },
    navigate: (opts) => { __calls.navigate.push(opts || {}); }
  },
  __swissExplicitConversationNavigation: { at: Date.now(), fromHref: "/c/`+target+`" },
  history: { state: { __TSR_key: "k1" }, replaceState: (s, u, url) => { __calls.replaceState.push(url); } },
  __swissLastRestoreNudge: 0
};
global.document = {
  getElementById: (id) => (id === "root" ? { childElementCount: 1 } : null),
  querySelector: (sel) => {
    if (sel.indexOf("conversation-view") !== -1) return {};
    if (sel.indexOf("new-conversation-button") !== -1 || sel.indexOf("conversation-list-sidebar") !== -1) return {};
    if (sel.indexOf('a[href*=') === 0) return { click: () => { __calls.click++; } };
    return null;
  }
};
`)
	if boolField(t, run.Res, "ready") {
		t.Errorf("the old restore target is not ready while the user is on another route: %v", run.Res)
	}
	if got := stringField(t, run.Res, "action"); got != "user_navigation" {
		t.Errorf("action = %q, want user_navigation", got)
	}
	if boolField(t, run.Res, "explicit_user_navigation") != true {
		t.Errorf("explicit user navigation must be reported: %v", run.Res)
	}
	if n, _ := run.Calls["click"].(float64); n != 0 {
		t.Errorf("restore must not click a sidebar link over direct user navigation, got %v", n)
	}
	if n := len(run.Calls["navigate"].([]interface{})); n != 0 {
		t.Errorf("restore must not router-navigate over direct user navigation, got %d", n)
	}
	if n := len(run.Calls["replaceState"].([]interface{})); n != 0 {
		t.Errorf("restore must not rewrite the address bar over direct user navigation, got %d", n)
	}
	if n := len(run.Calls["assign"].([]interface{})); n != 0 {
		t.Errorf("restore must not hard-navigate over direct user navigation, got %d", n)
	}
}
