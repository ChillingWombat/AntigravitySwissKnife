package boardpanel

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Fake CDP session
// ---------------------------------------------------------------------------

type fakeSession struct {
	mu sync.Mutex

	port    int
	portErr error

	targets []Target
	tgtErr  error

	// state simulates a live IDE: board/overview flags per target id.
	state map[string]PageState
	// evalErr forces the probe to fail when non-empty.
	evalErr map[string]error

	evals []string // recorded expressions, in call order
}

func (f *fakeSession) Port() (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.portErr != nil {
		return 0, f.portErr
	}
	return f.port, nil
}

func (f *fakeSession) Targets(port int) ([]Target, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.tgtErr != nil {
		return nil, f.tgtErr
	}
	return append([]Target(nil), f.targets...), nil
}

func (f *fakeSession) Evaluate(wsURL string, expression string) (map[string]interface{}, error) {
	f.mu.Lock()
	f.evals = append(f.evals, expression)
	var err error
	if f.evalErr != nil {
		if e, ok := f.evalErr[wsURL]; ok && e != nil {
			err = e
		}
	}
	var st PageState
	if f.state != nil {
		st = f.state[wsURL]
	}
	f.mu.Unlock()

	if err != nil {
		return nil, err
	}

	switch {
	case expression == ProbeScript:
		return map[string]interface{}{"value": map[string]interface{}{
			"swiss_board":   st.SwissBoard,
			"overview_open": st.OverviewOpen,
			"has_panel":     st.HasOverview,
		}}, nil
	case expression == CollapseScript:
		f.mu.Lock()
		s := f.state[wsURL]
		s.OverviewOpen = false
		f.state[wsURL] = s
		f.mu.Unlock()
		return map[string]interface{}{"value": map[string]interface{}{
			"ok": true, "hidden": true, "method": "native_toggle",
		}}, nil
	case expression == RestoreScript:
		f.mu.Lock()
		s := f.state[wsURL]
		s.OverviewOpen = true
		f.state[wsURL] = s
		f.mu.Unlock()
		return map[string]interface{}{"value": map[string]interface{}{
			"ok": true, "method": "restore",
		}}, nil
	}
	return nil, fmt.Errorf("unexpected expression")
}

func (f *fakeSession) countEvals(expr string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, e := range f.evals {
		if e == expr {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// Parsing
// ---------------------------------------------------------------------------

func TestParsePageState(t *testing.T) {
	st, ok := ParsePageState(map[string]interface{}{"value": map[string]interface{}{
		"swiss_board": true, "overview_open": true, "has_panel": true,
	}})
	if !ok {
		t.Fatalf("expected parse to succeed")
	}
	if !st.SwissBoard || !st.OverviewOpen || !st.HasOverview {
		t.Fatalf("unexpected state %+v", st)
	}

	// Bare object form (no "value" wrapper).
	st, ok = ParsePageState(map[string]interface{}{"swiss_board": false, "overview_open": false})
	if !ok {
		t.Fatalf("expected bare-object parse to succeed")
	}
	if st.SwissBoard {
		t.Errorf("expected swiss_board false")
	}

	if _, ok := ParsePageState(nil); ok {
		t.Errorf("nil payload must not parse")
	}
	if _, ok := ParsePageState(map[string]interface{}{"value": "nope"}); ok {
		t.Errorf("non-object payload must not parse")
	}
}

func TestParseActionResult(t *testing.T) {
	ok, method := ParseActionResult(map[string]interface{}{"value": map[string]interface{}{
		"ok": true, "method": "style_fallback",
	}})
	if !ok || method != "style_fallback" {
		t.Fatalf("got ok=%v method=%q", ok, method)
	}

	ok, method = ParseActionResult(map[string]interface{}{"value": map[string]interface{}{"hidden": true}})
	if !ok || method != "none" {
		t.Fatalf("legacy hidden payload: got ok=%v method=%q", ok, method)
	}

	if ok, _ := ParseActionResult(nil); ok {
		t.Errorf("nil payload must not be ok")
	}
}

// ---------------------------------------------------------------------------
// Page target filter
// ---------------------------------------------------------------------------

func TestIsAntigravityPage(t *testing.T) {
	cases := []struct {
		name string
		in   Target
		want bool
	}{
		{"loopback conversation", Target{URL: "https://127.0.0.1:54321/c/abc", Type: "page", WSURL: "ws://x"}, true},
		{"loopback titled", Target{URL: "https://127.0.0.1:54321/", Title: "Antigravity", Type: "page", WSURL: "ws://x"}, true},
		{"vscode-file scheme", Target{URL: "vscode-file://vscode-app/x", Type: "page", WSURL: "ws://x"}, true},
		{"no ws url", Target{URL: "https://127.0.0.1:54321/", Type: "page"}, false},
		{"random https site", Target{URL: "https://example.com/", Type: "page", WSURL: "ws://x"}, false},
	}
	for _, c := range cases {
		if got := IsAntigravityPage(c.in); got != c.want {
			t.Errorf("%s: IsAntigravityPage()=%v want %v", c.name, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Edge-triggered state machine
// ---------------------------------------------------------------------------

func TestVisitStateEdgeTrigger(t *testing.T) {
	v := &visitState{}

	// Not on the board page: nothing to do, even if the panel is wide open.
	if a := v.next(PageState{SwissBoard: false, OverviewOpen: true}); a != ActionNone {
		t.Fatalf("off-board must be no-op, got %v", a)
	}

	// Entering the board with the panel expanded -> collapse once.
	if a := v.next(PageState{SwissBoard: true, OverviewOpen: true}); a != ActionHideOverview {
		t.Fatalf("entering board with open panel must hide, got %v", a)
	}
	v.markCollapsed()

	// Still inside, user re-opened the panel -> never hide again.
	if a := v.next(PageState{SwissBoard: true, OverviewOpen: true}); a != ActionNone {
		t.Fatalf("must not re-hide after user re-opened, got %v", a)
	}
	if a := v.next(PageState{SwissBoard: true, OverviewOpen: false}); a != ActionNone {
		t.Fatalf("inside with collapsed panel must be no-op, got %v", a)
	}

	// Leaving the board: we collapsed it ourselves and it is still collapsed
	// -> restore so other pages are untouched.
	if a := v.next(PageState{SwissBoard: false, OverviewOpen: false}); a != ActionRestoreOverview {
		t.Fatalf("leaving board must restore self-collapsed panel, got %v", a)
	}
	v.markRestored()

	// Re-entering hides again.
	if a := v.next(PageState{SwissBoard: true, OverviewOpen: true}); a != ActionHideOverview {
		t.Fatalf("re-entering board must hide again, got %v", a)
	}
}

func TestVisitStateNoRestoreWhenUserReopened(t *testing.T) {
	v := &visitState{}
	v.next(PageState{SwissBoard: true, OverviewOpen: true}) // -> hide
	v.markCollapsed()
	// User manually re-opened the panel while staying on the board page.
	v.next(PageState{SwissBoard: true, OverviewOpen: true})
	// Leaving must not restore: the user owns the state now.
	if a := v.next(PageState{SwissBoard: false, OverviewOpen: true}); a != ActionNone {
		t.Fatalf("must not restore when the user re-opened the panel, got %v", a)
	}
}

func TestVisitStateEnteringWithCollapsedPanel(t *testing.T) {
	v := &visitState{}
	if a := v.next(PageState{SwissBoard: true, OverviewOpen: false}); a != ActionNone {
		t.Fatalf("entering an already collapsed board page must be no-op, got %v", a)
	}
	if a := v.next(PageState{SwissBoard: false, OverviewOpen: false}); a != ActionNone {
		t.Fatalf("leaving without self-collapse must be no-op, got %v", a)
	}
}

func TestTrackerPruneAndForget(t *testing.T) {
	tr := NewTracker()
	tr.Next("a", PageState{SwissBoard: true, OverviewOpen: true})
	tr.MarkCollapsed("a")
	tr.Next("b", PageState{SwissBoard: true, OverviewOpen: true})

	tr.Prune(map[string]bool{"b": true})
	// "a" was pruned, so it behaves like a fresh entry again.
	if a := tr.Next("a", PageState{SwissBoard: true, OverviewOpen: true}); a != ActionHideOverview {
		t.Fatalf("pruned target must behave as fresh entry, got %v", a)
	}
	// "b" was kept, so it must not hide again.
	if a := tr.Next("b", PageState{SwissBoard: true, OverviewOpen: true}); a != ActionNone {
		t.Fatalf("kept target must not re-hide, got %v", a)
	}

	tr.Forget("b")
	if a := tr.Next("b", PageState{SwissBoard: true, OverviewOpen: true}); a != ActionHideOverview {
		t.Fatalf("forgotten target must behave as fresh entry, got %v", a)
	}
}

// ---------------------------------------------------------------------------
// Watcher
// ---------------------------------------------------------------------------

func newTestWatcher(sess *fakeSession, enabled bool) *Watcher {
	return NewWatcher(sess, MinPollInterval, func() bool { return enabled })
}

func TestWatcherDisabled(t *testing.T) {
	sess := &fakeSession{
		port:    9222,
		targets: []Target{{ID: "t1", URL: "https://127.0.0.1:9222/c/x", Type: "page", WSURL: "ws://1"}},
		state:   map[string]PageState{"ws://1": {SwissBoard: true, OverviewOpen: true, HasOverview: true}},
	}
	w := newTestWatcher(sess, false)
	res, err := w.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if res.Targets != 0 || res.Probed != 0 || res.Hidden != 0 {
		t.Fatalf("disabled watcher must not touch anything: %+v", res)
	}
	if sess.countEvals(ProbeScript) != 0 {
		t.Errorf("disabled watcher must not evaluate")
	}
}

func TestWatcherGracefulWhenAntigravityDown(t *testing.T) {
	w := newTestWatcher(&fakeSession{portErr: fmt.Errorf("DevToolsActivePort not found")}, true)
	res, err := w.Check(context.Background())
	if err != nil {
		t.Fatalf("Antigravity down must not surface as error: %v", err)
	}
	if res.Targets != 0 {
		t.Errorf("expected no targets, got %+v", res)
	}
}

func TestWatcherProbeFailureSkipsTarget(t *testing.T) {
	sess := &fakeSession{
		port:    9222,
		targets: []Target{{ID: "t1", URL: "https://127.0.0.1:9222/c/x", Type: "page", WSURL: "ws://1"}},
		evalErr: map[string]error{"ws://1": fmt.Errorf("websocket closed")},
	}
	w := newTestWatcher(sess, true)
	res, err := w.Check(context.Background())
	if err != nil {
		t.Fatalf("probe failure must not surface as error: %v", err)
	}
	if res.Probed != 0 || res.Hidden != 0 {
		t.Errorf("expected no successful probe, got %+v", res)
	}
}

func TestWatcherHideOncePerEntry(t *testing.T) {
	sess := &fakeSession{
		port:    9222,
		targets: []Target{{ID: "t1", URL: "https://127.0.0.1:9222/c/x", Type: "page", WSURL: "ws://1"}},
		state:   map[string]PageState{"ws://1": {SwissBoard: false, OverviewOpen: true, HasOverview: true}},
	}
	w := newTestWatcher(sess, true)

	// 1. Off the board page: no collapse.
	if _, err := w.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sess.countEvals(CollapseScript) != 0 {
		t.Fatalf("must not collapse off the board page")
	}

	// 2. Enter the board page: one collapse.
	sess.mu.Lock()
	sess.state["ws://1"] = PageState{SwissBoard: true, OverviewOpen: true, HasOverview: true}
	sess.mu.Unlock()
	res, err := w.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Hidden != 1 {
		t.Fatalf("expected 1 collapse on entry, got %+v", res)
	}
	if res.Method != "native_toggle" {
		t.Fatalf("unexpected method %q", res.Method)
	}

	// 3. User re-opens the panel while still on the board page: never re-hide.
	sess.mu.Lock()
	sess.state["ws://1"] = PageState{SwissBoard: true, OverviewOpen: true, HasOverview: true}
	sess.mu.Unlock()
	for i := 0; i < 5; i++ {
		if _, err := w.Check(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if n := sess.countEvals(CollapseScript); n != 1 {
		t.Fatalf("expected exactly 1 collapse total, got %d", n)
	}

	// 4. Leaving the board restores the panel we collapsed.
	sess.mu.Lock()
	sess.state["ws://1"] = PageState{SwissBoard: false, OverviewOpen: false, HasOverview: true}
	sess.mu.Unlock()
	res, err = w.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Restored != 1 {
		t.Fatalf("expected 1 restore on exit, got %+v", res)
	}

	// 5. Re-entering collapses again.
	sess.mu.Lock()
	sess.state["ws://1"] = PageState{SwissBoard: true, OverviewOpen: true, HasOverview: true}
	sess.mu.Unlock()
	if _, err := w.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := sess.countEvals(CollapseScript); n != 2 {
		t.Fatalf("expected 2 collapses after re-entry, got %d", n)
	}
}

func TestWatcherIgnoresNonAntigravityTargets(t *testing.T) {
	sess := &fakeSession{
		port: 9222,
		targets: []Target{
			{ID: "bad", URL: "https://example.com/", Type: "page", WSURL: "ws://bad"},
			{ID: "good", URL: "https://127.0.0.1:9222/c/x", Type: "page", WSURL: "ws://good"},
		},
		state: map[string]PageState{
			"ws://bad":  {SwissBoard: true, OverviewOpen: true},
			"ws://good": {SwissBoard: true, OverviewOpen: true},
		},
	}
	w := newTestWatcher(sess, true)
	res, err := w.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Targets != 1 || res.Hidden != 1 {
		t.Fatalf("expected only the Antigravity target to be handled, got %+v", res)
	}
}

func TestWatcherRespectsContextCancel(t *testing.T) {
	sess := &fakeSession{
		port:    9222,
		targets: []Target{{ID: "t1", URL: "https://127.0.0.1:9222/c/x", Type: "page", WSURL: "ws://1"}},
		state:   map[string]PageState{"ws://1": {SwissBoard: true, OverviewOpen: true}},
	}
	w := newTestWatcher(sess, true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := w.Check(ctx); err == nil {
		t.Fatalf("expected context error")
	}
}

func TestWatcherRunStopsOnCancel(t *testing.T) {
	sess := &fakeSession{
		port:    9222,
		targets: []Target{{ID: "t1", URL: "https://127.0.0.1:9222/c/x", Type: "page", WSURL: "ws://1"}},
		state:   map[string]PageState{"ws://1": {SwissBoard: true, OverviewOpen: true}},
	}
	w := newTestWatcher(sess, true)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()
	time.Sleep(60 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("Run did not stop after context cancel")
	}
}

// ---------------------------------------------------------------------------
// Injected JavaScript
// ---------------------------------------------------------------------------

// jsHarness builds a minimal DOM good enough to exercise the probe/collapse
// scripts in Node. It deliberately avoids any real DOM implementation so the
// test stays hermetic.
const jsHarness = `
const store = {};
function makeEl(attrs, style) {
  const el = {
    attrs: Object.assign({}, attrs),
    style: Object.assign({}, style || {}),
    offsetWidth: 320,
    offsetHeight: 800,
    hidden: false,
    children: [],
    listeners: {},
  };
  el.getAttribute = (k) => (k in el.attrs ? el.attrs[k] : null);
  el.setAttribute = (k, v) => { el.attrs[k] = String(v); };
  el.removeAttribute = (k) => { delete el.attrs[k]; };
  el.contains = (other) => other === el || el.children.indexOf(other) !== -1;
  el.querySelectorAll = (sel) => el.children.filter(c => c.matches && c.matches(sel));
  el.querySelector = (sel) => el.querySelectorAll(sel)[0] || null;
  el.style.setProperty = (k, v) => { el.style[k] = v; };
  el.style.removeProperty = (k) => { delete el.style[k]; };
  el.click = () => { el.clicked = (el.clicked || 0) + 1; };
  el.dispatchEvent = () => true;
  return el;
}
function matchesSimple(el, sel) {
  // Supports: tag, [attr="v"], [attr*="v" i], .class, [role="x"]
  let s = sel.trim();
  const m = s.match(/^\[([a-zA-Z0-9-]+)\*?="?([^"\]]*)"?\s*(i)?\]$/);
  if (m) {
    const val = el.getAttribute(m[1]);
    if (val === null) return false;
    let needle = m[2];
    if (m[1] === "data-testid" && needle === "") return true;
    if (s.includes("*=")) {
      return String(val).toLowerCase().indexOf(needle.toLowerCase()) !== -1;
    }
    return String(val) === needle;
  }
  if (s.startsWith(".")) {
    return String(el.className || "").split(/\s+/).indexOf(s.slice(1)) !== -1;
  }
  return el.tag === s;
}
const registry = [];
global.document = {
  body: null,
  registry,
  getElementById(id) { return registry.find(e => e.attrs.id === id) || null; },
  querySelector(sel) { return this.querySelectorAll(sel)[0] || null; },
  querySelectorAll(sel) {
    const parts = sel.split(",").map(s => s.trim()).filter(Boolean);
    const out = [];
    registry.forEach(e => { if (parts.some(p => matchesSimple(e, p))) out.push(e); });
    return out;
  },
};
global.window = {
  getComputedStyle: (el) => ({ display: el.style.display || "flex", visibility: "visible" }),
};
global.PointerEvent = function () {};
global.MouseEvent = function () {};
`

func runNode(t *testing.T, src string) string {
	t.Helper()
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skipf("node not available: %v", err)
	}
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(jsHarness + "\n" + src)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node failed: %v\n%s", err, string(out))
	}
	return string(out)
}

func TestProbeScriptSyntax(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skipf("node not available: %v", err)
	}
	for name, src := range map[string]string{"probe": ProbeScript, "collapse": CollapseScript, "restore": RestoreScript} {
		cmd := exec.Command(nodePath, "--check", "-")
		cmd.Stdin = strings.NewReader(src)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s script is not valid JS: %v\n%s", name, err, string(out))
		}
	}
}

func TestProbeScriptDetectsSwissBoardAndOverview(t *testing.T) {
	src := `
const overview = makeEl({ "data-testid": "overview-panel" });
registry.push(overview);
const body = makeEl({ id: "body" });
document.body = body;

let r = ` + ProbeScript + `;
assert.strictEqual(r.has_panel, true, "overview panel should be found");
assert.strictEqual(r.overview_open, true, "overview panel should be expanded");
assert.strictEqual(r.swiss_board, false, "board should not be active yet");

// Board opens via the Swiss Knife main stage container.
const stage = makeEl({ id: "swiss-main-stage-container" }, { display: "flex" });
registry.push(stage);
r = ` + ProbeScript + `;
assert.strictEqual(r.swiss_board, true, "main stage display:flex means the board is active");

// Board also detected through body[data-swiss-stage-active].
stage.style.display = "none";
body.setAttribute("data-swiss-stage-active", "github");
r = ` + ProbeScript + `;
assert.strictEqual(r.swiss_board, true, "body[data-swiss-stage-active] means the board is active");
body.setAttribute("data-swiss-stage-active", "false");
r = ` + ProbeScript + `;
assert.strictEqual(r.swiss_board, false, "'false' must not count as active");

// Collapsed panel (zero size) reports closed.
overview.offsetWidth = 0; overview.offsetHeight = 0;
r = ` + ProbeScript + `;
assert.strictEqual(r.overview_open, false, "zero-size panel must report collapsed");
overview.offsetWidth = 320; overview.offsetHeight = 800;

// Self-collapsed marker wins over geometry.
overview.setAttribute("data-swiss-overview-auto-hidden", "true");
r = ` + ProbeScript + `;
assert.strictEqual(r.overview_open, false, "auto-hidden marker must report collapsed");
`
	runNode(t, "const assert = require('assert');\n"+src)
}

func TestCollapseScriptPrefersNativeToggle(t *testing.T) {
	src := `
const overview = makeEl({ "data-testid": "overview-panel" });
const toggle = makeEl({ "aria-label": "Collapse Overview" });
toggle.tag = "button";
toggle.matches = (sel) => sel.indexOf("button") !== -1;
overview.children.push(toggle);
registry.push(overview, toggle);
global.document = Object.assign({}, document, {});
document.querySelectorAll = function (sel) {
  const parts = sel.split(",").map(s => s.trim()).filter(Boolean);
  const out = [];
  registry.forEach(e => { if (parts.some(p => matchesSimple(e, p))) out.push(e); });
  return out;
};
// Simulate the IDE reacting to its own toggle: the panel collapses.
toggle.click = () => { overview.offsetWidth = 0; overview.offsetHeight = 0; overview.clicked = (overview.clicked||0)+1; };

(async () => {
  const r = await ` + CollapseScript + `;
  assert.strictEqual(r.method, "native_toggle", "must click the panel's own control first, got " + r.method);
  assert.strictEqual(r.ok, true, "collapse should succeed");
  assert.strictEqual(overview.getAttribute("data-swiss-overview-auto-hidden"), null,
    "native toggle must not fall back to inline styling");
})();
`
	runNode(t, "const assert = require('assert');\n"+src)
}

func TestCollapseScriptFallsBackToScopedStyle(t *testing.T) {
	src := `
const overview = makeEl({ "data-testid": "overview-panel" });
// A decoy "close" button must never be clicked.
const decoy = makeEl({ "aria-label": "Close" });
decoy.tag = "button";
decoy.matches = (sel) => sel.indexOf("button") !== -1;
overview.children.push(decoy);
registry.push(overview, decoy);
const realQueryAll = document.querySelectorAll;

(async () => {
  const r = await ` + CollapseScript + `;
  assert.strictEqual(decoy.clicked, undefined, "a generic Close button must not be clicked");
  assert.strictEqual(r.method, "style_fallback", "expected scoped style fallback, got " + r.method);
  assert.strictEqual(overview.getAttribute("data-swiss-overview-auto-hidden"), "true", "panel must be tagged");
  assert.strictEqual(overview.style.display, "none", "panel must be display:none");

  // Restore undoes only what we did.
  const rr = ` + RestoreScript + `;
  assert.strictEqual(rr.ok, true, "restore should succeed");
  assert.strictEqual(overview.getAttribute("data-swiss-overview-auto-hidden"), null, "marker must be removed");
})();
`
	runNode(t, "const assert = require('assert');\n"+src)
}

func TestRestoreScriptIgnoresForeignPanels(t *testing.T) {
	src := `
const overview = makeEl({ "data-testid": "overview-panel" });
// Never collapsed by us -> must be left alone.
overview.style.display = "none";
registry.push(overview);
const r = ` + RestoreScript + `;
assert.strictEqual(r.ok, false, "restore must be a no-op on panels we did not collapse");
assert.strictEqual(overview.style.display, "none", "foreign panel state must be untouched");
`
	runNode(t, "const assert = require('assert');\n"+src)
}
