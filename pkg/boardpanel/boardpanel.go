// Package boardpanel watches the running Google Antigravity IDE over the Chrome
// DevTools Protocol (CDP) and automatically collapses the IDE's own right-hand
// "Overview" panel while the Antigravity Swiss Knife board page is the active view.
//
// The Overview panel belongs to the Antigravity IDE's own UI (it is the auxiliary
// / right-hand side panel hosting Review, Files, Terminal, Plugins, Subagents,
// Background Tasks, Artifacts, Prompt Draft ...). It is NOT part of the Swiss
// Knife web UI, so the only way to drive it is through CDP, exactly like the
// existing live-CDP fibers in pkg/gui and pkg/revival.
//
// Safety contract:
//   - Antigravity not running / CDP port missing / probe fails -> skip silently.
//   - Only acts while the Swiss Knife board page is the active view.
//   - Edge triggered: the panel is collapsed once per *entry* into the board
//     page. If the user re-opens it while staying on the page it stays open
//     until they leave and come back.
//   - Collapsing prefers the IDE's own toggle control and only falls back to a
//     scoped inline style when no native control can be found.
package boardpanel

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
)

const (
	// DefaultPollInterval is how often the watcher re-probes the IDE.
	DefaultPollInterval = 2 * time.Second
	// MinPollInterval guards against pathological ticker values.
	MinPollInterval = 500 * time.Millisecond
	// EvalTimeout bounds a single Runtime.evaluate round-trip per page target.
	EvalTimeout = 3 * time.Second
)

// PageState is the observed state of a single Antigravity page target.
type PageState struct {
	// SwissBoard reports whether the Antigravity Swiss Knife board page is the
	// active view in this window.
	SwissBoard bool
	// OverviewOpen reports whether the IDE's right-hand Overview panel is expanded.
	OverviewOpen bool
	// HasOverview reports whether the Overview panel element was found at all.
	HasOverview bool
}

// Action is the side effect the watcher wants to perform on a page target.
type Action int

const (
	// ActionNone leaves the page untouched.
	ActionNone Action = iota
	// ActionHideOverview collapses the IDE's Overview panel.
	ActionHideOverview
	// ActionRestoreOverview re-expands a panel this watcher collapsed itself,
	// so leaving the board page does not leak the collapsed state elsewhere.
	ActionRestoreOverview
)

func (a Action) String() string {
	switch a {
	case ActionHideOverview:
		return "hide_overview"
	case ActionRestoreOverview:
		return "restore_overview"
	default:
		return "none"
	}
}

// ParsePageState converts a raw CDP Runtime.evaluate result into a PageState.
// It accepts both the wrapped form ({"value": {...}}) produced by
// gui.Injector.ExecuteScript and the bare object form.
func ParsePageState(raw map[string]interface{}) (PageState, bool) {
	if raw == nil {
		return PageState{}, false
	}
	v := raw["value"]
	if v == nil {
		v = raw
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return PageState{}, false
	}
	var st PageState
	st.SwissBoard, _ = m["swiss_board"].(bool)
	st.OverviewOpen, _ = m["overview_open"].(bool)
	st.HasOverview, _ = m["has_panel"].(bool)
	return st, true
}

// ParseActionResult converts a raw collapse/restore evaluation result into
// (ok, method). Method is one of: native_toggle, global_toggle, style_fallback,
// already_collapsed, restore, none.
func ParseActionResult(raw map[string]interface{}) (bool, string) {
	if raw == nil {
		return false, "none"
	}
	v := raw["value"]
	if v == nil {
		v = raw
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return false, "none"
	}
	res, _ := m["ok"].(bool)
	if !res {
		// Older payloads / plain boolean shape.
		if b, isBool := m["hidden"].(bool); isBool {
			res = b
		}
	}
	method, _ := m["method"].(string)
	if method == "" {
		method = "none"
	}
	return res, method
}

// visitState is the per-page-target edge-trigger memory.
type visitState struct {
	// inside is true while the board page is the active view.
	inside bool
	// collapsedByUs records that this watcher collapsed the panel during the
	// current visit, so it can restore it when the user leaves the page.
	collapsedByUs bool
}

// next folds a fresh observation into the visit state and returns the action to take.
//
// Semantics (edge triggered, never fights the user):
//   - board not active  -> reset visit; restore only if we collapsed it ourselves.
//   - entering the board -> collapse the Overview panel if it is expanded.
//   - already inside     -> never collapse again, even if the panel is re-opened.
func (v *visitState) next(st PageState) Action {
	if !st.SwissBoard {
		wasInside := v.inside
		collapsedByUs := v.collapsedByUs
		v.inside = false
		v.collapsedByUs = false
		if wasInside && collapsedByUs && !st.OverviewOpen {
			return ActionRestoreOverview
		}
		return ActionNone
	}
	if !v.inside {
		v.inside = true
		v.collapsedByUs = false
		if st.OverviewOpen {
			return ActionHideOverview
		}
		return ActionNone
	}
	return ActionNone
}

// markCollapsed records a successful collapse for the current visit.
func (v *visitState) markCollapsed() { v.collapsedByUs = true }

// markRestored clears the self-collapse memory after a successful restore.
func (v *visitState) markRestored() { v.collapsedByUs = false }

// Tracker keeps one visitState per CDP page target. It is safe for concurrent use.
type Tracker struct {
	mu     sync.Mutex
	visits map[string]*visitState
}

// NewTracker returns an empty Tracker.
func NewTracker() *Tracker {
	return &Tracker{visits: make(map[string]*visitState)}
}

// Next folds the observation for the given target into its visit state.
func (t *Tracker) Next(id string, st PageState) Action {
	if id == "" {
		return ActionNone
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	v := t.visits[id]
	if v == nil {
		v = &visitState{}
		t.visits[id] = v
	}
	return v.next(st)
}

// MarkCollapsed records that the panel for this target was collapsed by us.
func (t *Tracker) MarkCollapsed(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if v := t.visits[id]; v != nil {
		v.markCollapsed()
	}
}

// MarkRestored records that a self-collapsed panel was restored.
func (t *Tracker) MarkRestored(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if v := t.visits[id]; v != nil {
		v.markRestored()
	}
}

// Forget drops the visit state of a target (e.g. when it disappears).
func (t *Tracker) Forget(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.visits, id)
}

// Prune drops every visit state whose target id is not in keep.
func (t *Tracker) Prune(keep map[string]bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for id := range t.visits {
		if !keep[id] {
			delete(t.visits, id)
		}
	}
}

// Target is the minimal CDP page-target description this package needs.
type Target struct {
	ID    string
	Title string
	Type  string
	URL   string
	WSURL string
}

// Session is the CDP surface the watcher depends on. It exists so the watcher
// can be unit tested with a fake IDE instead of a live one.
type Session interface {
	// Port resolves the active Antigravity DevTools port.
	Port() (int, error)
	// Targets lists the debuggable page targets on that port.
	Targets(port int) ([]Target, error)
	// Evaluate runs expression in the target and returns the raw result.
	Evaluate(wsURL string, expression string) (map[string]interface{}, error)
}

// InjectorSession is the production Session backed by the shared gui.Injector.
type InjectorSession struct {
	inj *gui.Injector
}

// NewInjectorSession builds a Session on top of the live Antigravity CDP port.
func NewInjectorSession() *InjectorSession {
	return &InjectorSession{inj: gui.NewInjector(0)}
}

// NewInjectorSessionWithPort builds a Session pinned to an explicit port (tests).
func NewInjectorSessionWithPort(port int) *InjectorSession {
	return &InjectorSession{inj: gui.NewInjector(port)}
}

// Port resolves the DevTools port.
func (s *InjectorSession) Port() (int, error) {
	if s == nil || s.inj == nil {
		return 0, fmt.Errorf("boardpanel: no CDP injector")
	}
	return s.inj.FindDevToolsPort()
}

// Targets lists page targets on the port.
func (s *InjectorSession) Targets(port int) ([]Target, error) {
	if s == nil || s.inj == nil {
		return nil, fmt.Errorf("boardpanel: no CDP injector")
	}
	pages, err := s.inj.GetPageTargets(port)
	if err != nil {
		return nil, err
	}
	out := make([]Target, 0, len(pages))
	for _, p := range pages {
		out = append(out, Target{ID: p.ID, Title: p.Title, Type: p.Type, URL: p.URL, WSURL: p.WebSocketDebuggerURL})
	}
	return out, nil
}

// Evaluate runs expression in the target with a bounded deadline.
func (s *InjectorSession) Evaluate(wsURL string, expression string) (map[string]interface{}, error) {
	if s == nil || s.inj == nil {
		return nil, fmt.Errorf("boardpanel: no CDP injector")
	}
	return s.inj.ExecuteScriptWithTimeout(wsURL, expression, EvalTimeout)
}

// IsAntigravityPage reports whether a CDP target is an Antigravity IDE window
// (and not, say, a stray devtools page or a browser tab the user opened).
func IsAntigravityPage(t Target) bool {
	if t.WSURL == "" {
		return false
	}
	urlLower := strings.ToLower(t.URL)
	titleLower := strings.ToLower(t.Title)

	switch {
	case strings.HasPrefix(urlLower, "https://127.0.0.1:"),
		strings.HasPrefix(urlLower, "http://127.0.0.1:"),
		strings.HasPrefix(urlLower, "https://localhost:"),
		strings.HasPrefix(urlLower, "http://localhost:"):
		// The IDE serves its renderer from a loopback HTTPS port. Accept it when
		// the URL or the window title smells like Antigravity.
		return strings.Contains(urlLower, "antigravity") ||
			strings.Contains(titleLower, "antigravity") ||
			strings.Contains(urlLower, "/c/") ||
			t.Type == "page" ||
			t.Type == "webview"
	case strings.HasPrefix(urlLower, "vscode-file://"),
		strings.HasPrefix(urlLower, "file://"),
		strings.HasPrefix(urlLower, "app://"),
		strings.HasPrefix(urlLower, "electron://"):
		return true
	}
	return false
}

// CheckResult summarises one watcher pass.
type CheckResult struct {
	Targets int    // Antigravity page targets seen
	Probed  int    // targets successfully probed
	Hidden  int    // panels collapsed during this pass
	Restored int   // panels restored during this pass
	Method  string // collapse method used by the last successful collapse
}

// Watcher polls the IDE and applies the auto-hide decision.
type Watcher struct {
	Session  Session
	Interval time.Duration
	Enabled  func() bool
	Logger   *log.Logger

	tracker *Tracker
}

// NewWatcher builds a Watcher. enabled is consulted on every pass so flipping
// the config knob takes effect without restarting the daemon.
func NewWatcher(sess Session, interval time.Duration, enabled func() bool) *Watcher {
	if sess == nil {
		sess = NewInjectorSession()
	}
	if interval < MinPollInterval {
		interval = DefaultPollInterval
	}
	if enabled == nil {
		enabled = func() bool { return true }
	}
	return &Watcher{
		Session:  sess,
		Interval: interval,
		Enabled:  enabled,
		Logger:   log.New(os.Stderr, "[boardpanel] ", log.LstdFlags),
		tracker:  NewTracker(),
	}
}

// Tracker exposes the per-target visit state (tests / diagnostics).
func (w *Watcher) Tracker() *Tracker { return w.tracker }

// Run blocks until ctx is cancelled, probing the IDE every Interval.
func (w *Watcher) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval())
	defer ticker.Stop()
	for {
		if _, err := w.Check(ctx); err != nil {
			w.debugf("watch pass failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Watcher) interval() time.Duration {
	if w.Interval < MinPollInterval {
		return DefaultPollInterval
	}
	return w.Interval
}

func (w *Watcher) debugf(format string, args ...interface{}) {
	if w.Logger == nil {
		return
	}
	w.Logger.Printf(format, args...)
}

// Check performs a single pass over every Antigravity page target.
// It never returns an error for "Antigravity is not running" style conditions.
func (w *Watcher) Check(ctx context.Context) (CheckResult, error) {
	var res CheckResult
	if w.Session == nil {
		return res, nil
	}
	if w.Enabled != nil && !w.Enabled() {
		return res, nil
	}
	select {
	case <-ctx.Done():
		return res, ctx.Err()
	default:
	}

	port, err := w.Session.Port()
	if err != nil || port <= 0 {
		// Antigravity not running or CDP unavailable: expected, stay quiet.
		return res, nil
	}
	targets, err := w.Session.Targets(port)
	if err != nil || len(targets) == 0 {
		return res, nil
	}

	keep := make(map[string]bool, len(targets))
	for _, t := range targets {
		if !IsAntigravityPage(t) {
			continue
		}
		keep[t.ID] = true
		res.Targets++

		select {
		case <-ctx.Done():
			return res, ctx.Err()
		default:
		}

		raw, err := w.Session.Evaluate(t.WSURL, ProbeScript)
		if err != nil {
			w.debugf("probe failed on target %s: %v", t.ID, err)
			continue
		}
		st, ok := ParsePageState(raw)
		if !ok {
			w.debugf("unexpected probe payload on target %s", t.ID)
			continue
		}
		res.Probed++

		action := w.tracker.Next(t.ID, st)
		switch action {
		case ActionHideOverview:
			raw, err := w.Session.Evaluate(t.WSURL, CollapseScript)
			if err != nil {
				w.debugf("collapse failed on target %s: %v", t.ID, err)
				continue
			}
			hidden, method := ParseActionResult(raw)
			if hidden {
				res.Hidden++
				res.Method = method
				w.tracker.MarkCollapsed(t.ID)
				w.debugf("collapsed overview panel on target %s via %s", t.ID, method)
			} else {
				w.debugf("collapse reported no-op on target %s (%s)", t.ID, method)
			}
		case ActionRestoreOverview:
			raw, err := w.Session.Evaluate(t.WSURL, RestoreScript)
			if err != nil {
				w.debugf("restore failed on target %s: %v", t.ID, err)
				continue
			}
			restored, _ := ParseActionResult(raw)
			if restored {
				res.Restored++
				w.tracker.MarkRestored(t.ID)
				w.debugf("restored overview panel on target %s", t.ID)
			}
		default:
		}
	}

	w.tracker.Prune(keep)
	return res, nil
}
