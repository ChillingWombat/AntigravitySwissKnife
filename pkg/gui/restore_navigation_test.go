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
		views := "[]"
		inputs := "[]"
		queryView := "null"
		if viewMounted {
			views = `[{ dataset: { cascadeId: "bbbbbbbb-5555-6666-7777-888888888888" }, getClientRects: () => [{}], getBoundingClientRect: () => ({ width: 900, height: 600 }), contains: () => true }]`
			inputs = `[{ dataset: { cascadeId: "bbbbbbbb-5555-6666-7777-888888888888" }, getClientRects: () => [{}], getBoundingClientRect: () => ({ width: 720, height: 44 }) }]`
			queryView = "{}"
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
    if (sel.indexOf("conversation-view") !== -1) return ` + queryView + `;
    if (sel.indexOf("new-conversation-button") !== -1 || sel.indexOf("conversation-list-sidebar") !== -1) return {};
    return null;
  },
  querySelectorAll: (sel) => {
    if (sel.indexOf("conversation-view") !== -1) return ` + views + `;
    if (sel.indexOf("agent-input-box") !== -1 || sel.indexOf("contenteditable") !== -1 || sel.indexOf("chat-input-textarea") !== -1 || sel.indexOf("textarea") !== -1) return ` + inputs + `;
    return [];
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
const __targetView = { dataset: { cascadeId: "bbbbbbbb-5555-6666-7777-888888888888" }, getClientRects: () => [{}], getBoundingClientRect: () => ({ width: 900, height: 600 }), contains: () => true };
const __targetInput = { dataset: { cascadeId: "bbbbbbbb-5555-6666-7777-888888888888" }, getClientRects: () => [{}], getBoundingClientRect: () => ({ width: 720, height: 44 }) };
global.window = {
  location: { pathname: "`+target+`", search: "" },
  history: { state: { __TSR_key: "k1" }, replaceState: (s, u, url) => { __calls.replaceState.push(url); } },
  __swissLastRestoreNudge: Date.now() - 5000
};
global.document = {
  getElementById: (id) => (id === "root" ? { childElementCount: 1 } : null),
  querySelector: (sel) => {
    if (sel.indexOf("conversation-view") !== -1) return __targetView;
    if (sel.indexOf("new-conversation-button") !== -1 || sel.indexOf("conversation-list-sidebar") !== -1) return {};
    return null;
  },
  querySelectorAll: (sel) => {
    if (sel.indexOf("conversation-view") !== -1) return [__targetView];
    if (sel.indexOf("agent-input-box") !== -1 || sel.indexOf("contenteditable") !== -1 || sel.indexOf("chat-input-textarea") !== -1 || sel.indexOf("textarea") !== -1) return [__targetInput];
    return [];
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
  __swissExplicitConversationNavigation: { version: 2, kind: "conversation", pending: false, route: "`+userRoute+`" },
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

// TestRestoreConversationPathScript_UnrelatedRouteDoesNotStayFenced proves a
// persisted intent is route-owned rather than permanent: an older conversation
// route no longer authorizes restore suppression on a different route.
func TestRestoreConversationPathScript_UnrelatedRouteDoesNotStayFenced(t *testing.T) {
	target := "/c/bbbbbbbb-5555-6666-7777-888888888888"
	currentRoute := "/c/aaaaaaaa-1111-2222-3333-444444444444"
	staleIntentRoute := "/c/cccccccc-3333-4444-5555-666666666666"
	run := runRestorePathScript(t, target, `
const __state = { pathname: "`+currentRoute+`", search: "" };
global.window = {
  location: { get pathname() { return __state.pathname; }, get search() { return __state.search; }, assign: (url) => { __calls.assign.push(url); } },
  __TSR_ROUTER__: {
    state: { location: { get pathname() { return __state.pathname; } } },
    navigate: (opts) => { __calls.navigate.push(opts || {}); }
  },
  __swissExplicitConversationNavigation: { version: 2, kind: "conversation", pending: false, route: "`+staleIntentRoute+`" },
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
	if got := stringField(t, run.Res, "action"); got == "user_navigation" {
		t.Errorf("an unrelated stale intent must not fence restore: %v", run.Res)
	}
	if boolField(t, run.Res, "explicit_user_navigation") {
		t.Errorf("explicit_user_navigation must be false on an unrelated route: %v", run.Res)
	}
	clicks, _ := run.Calls["click"].(float64)
	navigations := len(run.Calls["navigate"].([]interface{}))
	if int(clicks)+navigations != 1 {
		t.Errorf("restore must use exactly one host navigation primitive on an unrelated route, got %v clicks and %d router navigations", clicks, navigations)
	}
}

// TestRestoreConversationPathScript_SelectsOnlyVisibleMatchingSidebarAnchor
// pins the #432 recovery target: a hidden duplicate anchor is never clicked,
// even when it precedes the real visible 240x32 sidebar row.
func TestRestoreConversationPathScript_SelectsOnlyVisibleMatchingSidebarAnchor(t *testing.T) {
	target := "/c/bbbbbbbb-5555-6666-7777-888888888888"
	run := runRestorePathScript(t, target, `
const target = "`+target+`";
const hidden = {
  href: target,
  getAttribute: () => target,
  getClientRects: () => [],
  getBoundingClientRect: () => ({ width: 0, height: 0 }),
  click: () => { __calls.click++; __calls.click_target = "hidden"; }
};
const visible = {
  href: target,
  getAttribute: () => target,
  getClientRects: () => [{}],
  getBoundingClientRect: () => ({ width: 240, height: 32 }),
  click: () => { __calls.click++; __calls.click_target = "visible"; }
};
global.window = {
  location: { pathname: "/", search: "", assign: (url) => { __calls.assign.push(url); } },
  __TSR_ROUTER__: {
    state: { location: { pathname: "/" } },
    navigate: (opts) => { __calls.navigate.push(opts || {}); }
  },
  history: { state: { __TSR_key: "k1" }, replaceState: (s, u, url) => { __calls.replaceState.push(url); } },
  __swissLastRestoreNudge: 0
};
global.document = {
  getElementById: (id) => (id === "root" ? { childElementCount: 1 } : null),
  querySelector: (sel) => {
    if (sel.indexOf("conversation-view") !== -1) return null;
    if (sel.indexOf("new-conversation-button") !== -1 || sel.indexOf("conversation-list-sidebar") !== -1) return {};
    return null;
  },
  querySelectorAll: (sel) => sel.indexOf('a[href*=') === 0 ? [hidden, visible] : []
};
`)
	if got := stringField(t, run.Res, "action"); got != "click_link" {
		t.Errorf("action = %q, want click_link on the visible sidebar row", got)
	}
	if clicks, _ := run.Calls["click"].(float64); clicks != 1 {
		t.Errorf("visible sidebar click count = %v, want exactly 1", clicks)
	}
	if got, _ := run.Calls["click_target"].(string); got != "visible" {
		t.Errorf("clicked target = %q, want visible (hidden duplicate must never receive the click)", got)
	}
	if navigations := len(run.Calls["navigate"].([]interface{})); navigations != 0 {
		t.Errorf("router navigation count = %d, want 0 when a visible target row exists", navigations)
	}
}

// TestRestoreConversationPathScript_RejectsStaleMountedConversationView proves
// URL/router agreement is insufficient when the mounted view and input still
// carry another cascade ID.
func TestRestoreConversationPathScript_RejectsStaleMountedConversationView(t *testing.T) {
	target := "/c/bbbbbbbb-5555-6666-7777-888888888888"
	stale := "/c/aaaaaaaa-1111-2222-3333-444444444444"
	run := runRestorePathScript(t, target, `
const target = "`+target+`";
const staleView = {
  dataset: { cascadeId: "aaaaaaaa-1111-2222-3333-444444444444" },
  getClientRects: () => [{}],
  getBoundingClientRect: () => ({ width: 1100, height: 700 }),
  contains: () => true
};
const staleInput = {
  dataset: { cascadeId: "aaaaaaaa-1111-2222-3333-444444444444" },
  getClientRects: () => [{}],
  getBoundingClientRect: () => ({ width: 720, height: 44 })
};
const visible = {
  href: target,
  getAttribute: () => target,
  getClientRects: () => [{}],
  getBoundingClientRect: () => ({ width: 240, height: 32 }),
  click: () => { __calls.click++; }
};
global.window = {
  location: { pathname: target, search: "", assign: (url) => { __calls.assign.push(url); } },
  __TSR_ROUTER__: {
    state: { location: { pathname: target } },
    navigate: (opts) => { __calls.navigate.push(opts || {}); }
  },
  history: { state: { __TSR_key: "k1" }, replaceState: (s, u, url) => { __calls.replaceState.push(url); } },
  __swissLastRestoreNudge: 0
};
global.document = {
  getElementById: (id) => (id === "root" ? { childElementCount: 1 } : null),
  querySelector: (sel) => {
    if (sel.indexOf("conversation-view") !== -1) return staleView;
    if (sel.indexOf("new-conversation-button") !== -1 || sel.indexOf("conversation-list-sidebar") !== -1) return {};
    return null;
  },
  querySelectorAll: (sel) => {
    if (sel.indexOf("conversation-view") !== -1) return [staleView];
    if (sel.indexOf("agent-input-box") !== -1 || sel.indexOf("contenteditable") !== -1) return [staleInput];
    if (sel.indexOf('a[href*=') === 0) return [visible];
    return [];
  }
};
`)
	if boolField(t, run.Res, "ready") {
		t.Errorf("a stale mounted view must not be ready: %v", run.Res)
	}
	if got := stringField(t, run.Res, "view_identity"); got != "aaaaaaaa-1111-2222-3333-444444444444" {
		t.Errorf("view_identity = %q, want the stale cascade ID for diagnostics", got)
	}
	if got := stringField(t, run.Res, "action"); got != "click_link" {
		t.Errorf("action = %q, want click_link to remount %s (stale %s)", got, target, stale)
	}
	if clicks, _ := run.Calls["click"].(float64); clicks != 1 {
		t.Errorf("visible target click count = %v, want exactly 1", clicks)
	}
}

// TestRestoreConversationPathScript_StaleRootIntentDoesNotFenceRecovery keeps a
// persisted new-conversation intent at "/" from suppressing an unrelated
// conversation recovery target.
func TestRestoreConversationPathScript_StaleRootIntentDoesNotFenceRecovery(t *testing.T) {
	target := "/c/bbbbbbbb-5555-6666-7777-888888888888"
	run := runRestorePathScript(t, target, `
const target = "`+target+`";
const visible = {
  href: target,
  getAttribute: () => target,
  getClientRects: () => [{}],
  getBoundingClientRect: () => ({ width: 240, height: 32 }),
  click: () => { __calls.click++; }
};
global.window = {
  location: { pathname: "/", search: "", assign: (url) => { __calls.assign.push(url); } },
  __TSR_ROUTER__: {
    state: { location: { pathname: "/" } },
    navigate: (opts) => { __calls.navigate.push(opts || {}); }
  },
  __swissExplicitConversationNavigation: { version: 2, kind: "new_conversation", pending: false, route: "/" },
  history: { state: { __TSR_key: "k1" }, replaceState: (s, u, url) => { __calls.replaceState.push(url); } },
  __swissLastRestoreNudge: 0
};
global.document = {
  getElementById: (id) => (id === "root" ? { childElementCount: 1 } : null),
  querySelector: (sel) => {
    if (sel.indexOf("conversation-view") !== -1) return null;
    if (sel.indexOf("new-conversation-button") !== -1 || sel.indexOf("conversation-list-sidebar") !== -1) return {};
    return null;
  },
  querySelectorAll: (sel) => sel.indexOf('a[href*=') === 0 ? [visible] : []
};
`)
	if boolField(t, run.Res, "explicit_user_navigation") {
		t.Errorf("root-scoped intent must not count as explicit conversation navigation: %v", run.Res)
	}
	if got := stringField(t, run.Res, "action"); got != "click_link" {
		t.Errorf("action = %q, want click_link despite stale persisted intent at /", got)
	}
	if clicks, _ := run.Calls["click"].(float64); clicks != 1 {
		t.Errorf("visible target click count = %v, want exactly 1", clicks)
	}
}
