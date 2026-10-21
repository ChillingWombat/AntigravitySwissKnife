package enhancements

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Errorf("expected Enabled to be true by default")
	}
	if !cfg.PromptJumpBar.Enabled {
		t.Errorf("expected PromptJumpBar.Enabled to be true by default")
	}
	if !cfg.PromptJumpBar.ShowTooltip {
		t.Errorf("expected ShowTooltip to be true")
	}
	if !cfg.PromptJumpBar.FocusPulse {
		t.Errorf("expected FocusPulse to be true")
	}
	if !cfg.PromptJumpBar.SyncScroll {
		t.Errorf("expected SyncScroll to be true")
	}
	if cfg.PromptJumpBar.Position != "gutter" {
		t.Errorf("expected Position to be gutter, got %s", cfg.PromptJumpBar.Position)
	}
	if cfg.PromptJumpBar.DashWidth != 14 {
		t.Errorf("expected DashWidth to be 14, got %d", cfg.PromptJumpBar.DashWidth)
	}
	if cfg.PromptJumpBar.DashThickness != 2.5 {
		t.Errorf("expected DashThickness to be 2.5, got %v", cfg.PromptJumpBar.DashThickness)
	}
	if cfg.PromptJumpBar.InactiveThickness != 1.5 {
		t.Errorf("expected InactiveThickness to be 1.5, got %v", cfg.PromptJumpBar.InactiveThickness)
	}
	if cfg.PromptJumpBar.ColorMode != "default" {
		t.Errorf("expected ColorMode to be default, got %s", cfg.PromptJumpBar.ColorMode)
	}
	if cfg.ToolDensityMode != "muted" {
		t.Errorf("expected ToolDensityMode to be muted, got %s", cfg.ToolDensityMode)
	}
	if !cfg.BreakerLineEnabled {
		t.Errorf("expected BreakerLineEnabled to be true by default")
	}
	if !cfg.LeftPanelExtensionsEnabled {
		t.Errorf("expected LeftPanelExtensionsEnabled to be true by default")
	}
	if cfg.LeftPanelExtensionsMode != "single" {
		t.Errorf("expected LeftPanelExtensionsMode to be 'single' by default, got %s", cfg.LeftPanelExtensionsMode)
	}
	if cfg.DefaultNewProject != "auto" {
		t.Errorf("expected DefaultNewProject to be 'auto', got %s", cfg.DefaultNewProject)
	}
	if cfg.AutomationTargetProjectEnabled == nil || !*cfg.AutomationTargetProjectEnabled {
		t.Errorf("expected AutomationTargetProjectEnabled to be true by default")
	}
	if cfg.ContextWindowIndicatorEnabled == nil || !*cfg.ContextWindowIndicatorEnabled {
		t.Errorf("expected ContextWindowIndicatorEnabled to be true by default")
	}
	if cfg.ContextWindowClickToCompactEnabled == nil || !*cfg.ContextWindowClickToCompactEnabled {
		t.Errorf("expected ContextWindowClickToCompactEnabled to be true by default")
	}
	if !cfg.OverviewPanel.ReplaceSeeAllTriangle {
		t.Errorf("expected OverviewPanel.ReplaceSeeAllTriangle to be true by default")
	}
	if len(cfg.Extensions) != len(ExtensionIDs) {
		t.Fatalf("expected Extensions map to cover %d ids, got %d", len(ExtensionIDs), len(cfg.Extensions))
	}
	for _, id := range ExtensionIDs {
		vis, ok := cfg.Extensions[id]
		if !ok {
			t.Errorf("expected Extensions map to contain %q", id)
			continue
		}
		if !vis.AuxPanel || vis.MainPage {
			t.Errorf("expected Extensions[%q] to default to {true, false}, got %+v", id, vis)
		}
	}
}

func TestStore_Operations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "enhancements-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "enhancements.json")
	store, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := store.GetConfig()
	if !cfg.PromptJumpBar.Enabled {
		t.Errorf("expected PromptJumpBar to be enabled")
	}

	// Toggle prompt jump bar off
	if err := store.TogglePromptJumpBar(false); err != nil {
		t.Fatalf("failed to toggle prompt jump bar: %v", err)
	}

	cfgAfter := store.GetConfig()
	if cfgAfter.PromptJumpBar.Enabled {
		t.Errorf("expected PromptJumpBar to be disabled after toggle")
	}

	// Set color mode and custom color
	if err := store.SetPromptJumpBarColor("custom", "#7c3aed"); err != nil {
		t.Fatalf("failed to set color mode: %v", err)
	}

	// Set tool density and breaker line
	if err := store.SetToolDensityMode("hidden"); err != nil {
		t.Fatalf("failed to set tool density: %v", err)
	}
	if err := store.SetBreakerLine(false); err != nil {
		t.Fatalf("failed to set breaker line: %v", err)
	}
	if err := store.SetDefaultNewProject("Antigravity Swiss Knife"); err != nil {
		t.Fatalf("failed to set default new project: %v", err)
	}

	// Reload in another store instance to verify persistence
	store2, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to load store2: %v", err)
	}

	loaded := store2.GetConfig()
	if loaded.PromptJumpBar.ColorMode != "custom" || loaded.PromptJumpBar.CustomColor != "#7c3aed" {
		t.Errorf("expected custom color to persist, got mode=%s color=%s", loaded.PromptJumpBar.ColorMode, loaded.PromptJumpBar.CustomColor)
	}
	if loaded.ToolDensityMode != "hidden" {
		t.Errorf("expected ToolDensityMode hidden to persist, got %s", loaded.ToolDensityMode)
	}
	if loaded.BreakerLineEnabled != false {
		t.Errorf("expected BreakerLineEnabled false to persist")
	}
	if loaded.DefaultNewProject != "Antigravity Swiss Knife" {
		t.Errorf("expected DefaultNewProject to persist, got %s", loaded.DefaultNewProject)
	}
}

func TestStore_AutomationTargetProjectEnabledDefaultsTrueForLegacyConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "enhancements.json")
	if err := os.WriteFile(configPath, []byte(`{"version":"1.0.0","enabled":true}`), 0644); err != nil {
		t.Fatalf("failed to write legacy config: %v", err)
	}

	store, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to load legacy config: %v", err)
	}

	cfg := store.GetConfig()
	if cfg.AutomationTargetProjectEnabled == nil || !*cfg.AutomationTargetProjectEnabled {
		t.Fatalf("expected missing automation_target_project_enabled to load as true")
	}
	if cfg.ContextWindowIndicatorEnabled == nil || !*cfg.ContextWindowIndicatorEnabled {
		t.Fatalf("expected missing context_window_indicator_enabled to load as true")
	}
	if cfg.ContextWindowClickToCompactEnabled == nil || !*cfg.ContextWindowClickToCompactEnabled {
		t.Fatalf("expected missing context_window_click_to_compact_enabled to load as true")
	}

	if err := os.WriteFile(configPath, []byte(`{"version":"1.0.0","enabled":true,"automation_target_project_enabled":false,"context_window_indicator_enabled":false,"context_window_click_to_compact_enabled":false}`), 0644); err != nil {
		t.Fatalf("failed to write disabled config: %v", err)
	}
	store, err = NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to load disabled config: %v", err)
	}
	cfg = store.GetConfig()
	if cfg.AutomationTargetProjectEnabled == nil || *cfg.AutomationTargetProjectEnabled {
		t.Fatalf("expected explicit automation_target_project_enabled false to persist")
	}
	if cfg.ContextWindowIndicatorEnabled == nil || *cfg.ContextWindowIndicatorEnabled {
		t.Fatalf("expected explicit context_window_indicator_enabled false to persist")
	}
	if cfg.ContextWindowClickToCompactEnabled == nil || *cfg.ContextWindowClickToCompactEnabled {
		t.Fatalf("expected explicit context_window_click_to_compact_enabled false to persist")
	}
}

func TestGenerateEnhancementsScript(t *testing.T) {
	cfg := DefaultConfig()
	script := GenerateEnhancementsScript(cfg)

	if !strings.Contains(script, "swiss-prompt-jump-bar") {
		t.Errorf("expected script to contain swiss-prompt-jump-bar element ID")
	}
	if !strings.Contains(script, "user-input-step") {
		t.Errorf("expected script to query user-input-step testid")
	}
	if !strings.Contains(script, "swiss-prompt-tooltip") {
		t.Errorf("expected script to manage swiss-prompt-tooltip")
	}
	if !strings.Contains(script, "scrollIntoView") {
		t.Errorf("expected script to call scrollIntoView")
	}
	if !strings.Contains(script, "getLowestPromptOnScreen") {
		t.Errorf("expected script to implement getLowestPromptOnScreen")
	}
	if !strings.Contains(script, "getActiveAndHoverColor") {
		t.Errorf("expected script to implement getActiveAndHoverColor")
	}
	if !strings.Contains(script, "worked-for-collapsible") {
		t.Errorf("expected script to target worked-for-collapsible")
	}
	if !strings.Contains(script, "thinking-collapsible-trigger") {
		t.Errorf("expected script to target thinking-collapsible-trigger")
	}
	if !strings.Contains(script, "applyDefaultProjectHandler") {
		t.Errorf("expected script to implement applyDefaultProjectHandler")
	}
	if !strings.Contains(script, "new-conversation-button") {
		t.Errorf("expected script to intercept new-conversation-button")
	}
	if !strings.Contains(script, "swiss-overview-tabs-divider") {
		t.Errorf("expected script to contain swiss-overview-tabs-divider")
	}
	if !strings.Contains(script, "swiss-overview-bottom-spacer") {
		t.Errorf("expected script to contain swiss-overview-bottom-spacer cleanup")
	}
	if !strings.Contains(script, "computeStableTurnTotal") {
		t.Errorf("expected prompt jump bar to resolve a stable turn total")
	}
	if !strings.Contains(script, "computeTurnOrdinals") {
		t.Errorf("expected prompt jump bar to read list ordinals of the mounted turns")
	}
	if !strings.Contains(script, "mapStepsToTurnIndexes") {
		t.Errorf("expected prompt jump bar to map mounted steps to turn indexes")
	}
	if !strings.Contains(script, "jumpBarPromptCountsByConversation") {
		t.Errorf("expected prompt jump bar to consume the daemon user-prompt count")
	}
	if !strings.Contains(script, "aria-posinset") {
		t.Errorf("expected prompt jump bar to honor aria-posinset turn ordinals")
	}
	if !strings.Contains(script, "filterMainConversationSteps") {
		t.Errorf("expected prompt jump bar to filter nested and duplicate prompt nodes")
	}
}

// TestChatNavigatorPanel_ModeGate pins the chat_navigator_mode contract: the
// rail reads the setting from the GUI config (the same object that carries the
// chat telemetry switches), "panel" is the default for missing/unknown values,
// and "line" keeps today's dash-only rail with the hover tooltip.
func TestChatNavigatorPanel_ModeGate(t *testing.T) {
	script := GenerateEnhancementsScript(DefaultConfig())

	if !strings.Contains(script, `let chatNavigatorMode = "panel";`) {
		t.Errorf("expected the chat navigator style to default to \"panel\"")
	}
	if !strings.Contains(script, "loadedGui.chat_navigator_mode") {
		t.Errorf("expected the navigator style to be read from the GUI config next to chat_telemetry_*")
	}
	if !strings.Contains(script, "normalizeNavigatorMode(loadedGui.chat_navigator_mode)") {
		t.Errorf("expected the GUI config value to be normalized (unknown -> panel)")
	}
	if !strings.Contains(script, "function normalizeNavigatorMode(mode)") {
		t.Errorf("expected a normalizeNavigatorMode helper")
	}
	if !strings.Contains(script, "const navigatorMode = normalizeNavigatorMode(chatNavigatorMode);") {
		t.Errorf("expected the rail render to resolve the navigator mode")
	}
	if !strings.Contains(script, `if (navigatorMode !== "panel") {`) {
		t.Errorf("expected line mode to suppress the hover panel")
	}
	if !strings.Contains(script, `if (tooltip && navigatorMode === "line")`) {
		t.Errorf("expected the hover tooltip to remain exclusive to line mode")
	}
	if !strings.Contains(script, `if (!isNavigatorPanelMode()) return;`) {
		t.Errorf("expected the panel to refuse to open outside panel mode")
	}
	if !strings.Contains(script, "scheduleOpenPromptNavigatorPanel();") {
		t.Errorf("expected hovering a dash to schedule opening the hover panel")
	}
	if !strings.Contains(script, "scheduleClosePromptNavigatorPanel();") {
		t.Errorf("expected leaving a dash to schedule closing the hover panel")
	}
}

// TestChatNavigatorPanel_Helpers runs the extracted pure helpers in Node: the
// mode gate, the prompt-text condensation and the virtualized-turn text cache
// (keyed by conversation + ordinal, unknown turns -> muted "…", never invented).
func TestChatNavigatorPanel_Helpers(t *testing.T) {
	script := GenerateEnhancementsScript(DefaultConfig())

	startMarker := "/* __SWISS_NAV_PANEL_HELPERS_START__ */"
	endMarker := "/* __SWISS_NAV_PANEL_HELPERS_END__ */"
	startIdx := strings.Index(script, startMarker)
	endIdx := strings.Index(script, endMarker)
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		t.Fatalf("expected the pure navigator helpers to be delimited by %q and %q", startMarker, endMarker)
	}
	helpers := script[startIdx+len(startMarker) : endIdx]

	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}

	driver := helpers + `
const assert = require("assert");

// Mode gate: only the exact "line" value opts out; everything else is "panel".
assert.strictEqual(normalizeNavigatorMode("line"), "line");
assert.strictEqual(normalizeNavigatorMode("panel"), "panel");
assert.strictEqual(normalizeNavigatorMode(undefined), "panel", "missing setting must default to panel");
assert.strictEqual(normalizeNavigatorMode(null), "panel");
assert.strictEqual(normalizeNavigatorMode(""), "panel");
assert.strictEqual(normalizeNavigatorMode("mirrored"), "panel", "unknown values must not disable the panel");

// Prompt text: collapsed to one line, trimmed, capped at ~200 chars.
assert.strictEqual(condensePromptText("  hello \n  world  "), "hello world");
assert.strictEqual(condensePromptText(null), "");
assert.strictEqual(condensePromptText(undefined), "");
assert.strictEqual(condensePromptText("   "), "");
const capped = condensePromptText("x".repeat(400));
assert.strictEqual(capped.length, 201, "long prompts must be capped at 200 chars plus an ellipsis");
assert.ok(capped.endsWith("…"));

// The cache is keyed by (conversation, ordinal): a mounted window of the
// virtualized transcript must not shadow other turns or other conversations.
const cache = {};
cachePromptText(cache, "conv-a", 0, "first prompt");
cachePromptText(cache, "conv-a", 3, "fourth prompt");
assert.strictEqual(getCachedPromptText(cache, "conv-a", 0), "first prompt");
assert.strictEqual(getCachedPromptText(cache, "conv-a", 3), "fourth prompt");

// Unknown turns render the muted ellipsis, never fabricated text.
assert.strictEqual(getCachedPromptText(cache, "conv-a", 1), "…");
assert.strictEqual(getCachedPromptText(cache, "conv-a", 99), "…");
assert.strictEqual(getCachedPromptText(cache, "conv-b", 0), "…", "another conversation must not leak cached text");
assert.strictEqual(getCachedPromptText(cache, null, 0), "…");
assert.strictEqual(getCachedPromptText(cache, "conv-a", -1), "…");

// Empty prompt text is not cached; the same ordinal in another conversation is
// an independent entry.
cachePromptText(cache, "conv-a", 7, "   ");
assert.strictEqual(getCachedPromptText(cache, "conv-a", 7), "…", "blank prompts must not be cached");
cachePromptText(cache, "conv-b", 0, "conv b first");
assert.notStrictEqual(promptTextCacheKey("conv-a", 0), promptTextCacheKey("conv-b", 0));
assert.strictEqual(getCachedPromptText(cache, "conv-a", 0), "first prompt");

// Row markup: dash marker + single-line, left-aligned, ellipsized text.
function makeEl(tag) {
  const el = {
    tagName: tag,
    className: "",
    style: {},
    attributes: {},
    children: [],
    textContent: "",
    setAttribute(k, v) { this.attributes[k] = v; },
    getAttribute(k) { return this.attributes[k]; },
    appendChild(c) { c.parentElement = this; this.children.push(c); return c; }
  };
  el.classList = {
    contains: (c) => el.className.split(" ").indexOf(c) !== -1,
    add: (c) => { if (el.className.split(" ").indexOf(c) === -1) el.className = (el.className + " " + c).trim(); },
    remove: (c) => { el.className = el.className.split(" ").filter((x) => x && x !== c).join(" "); }
  };
  return el;
}
const doc = { createElement: (tag) => makeEl(tag) };
const ui = {
  dashWidth: 14,
  hoverDashWidth: 22,
  activeThickness: 2.5,
  inactiveThickness: 1.5,
  activeColor: "ACTIVE",
  defaultColor: "DEFAULT",
  activeRowBackground: "ROW_BG",
  hoverRowBackground: "HOVER_ROW_BG",
  mutedColor: "MUTED"
};
const activated = [];
const row = createNavigatorRow(doc, 4, "cached prompt text", true, ui, (idx) => activated.push(idx));
assert.strictEqual(row.attributes.role, "menuitem");
assert.strictEqual(row.attributes.tabindex, "-1");
assert.strictEqual(row.attributes["data-prompt-index"], "4");
assert.ok(row.className.indexOf("swiss-navigator-row-active") !== -1, "the active turn must be highlighted");
assert.strictEqual(row.style.background, "ROW_BG");

const marker = row.children[0];
assert.strictEqual(marker.className, "swiss-navigator-row-dash active");
assert.strictEqual(marker.style.width, "14px");
assert.strictEqual(marker.style.height, "2.5px");
assert.strictEqual(marker.style.background, "ACTIVE");
assert.ok(marker.style.cssText.indexOf("transition: width 0.18s cubic-bezier(0.4, 0, 0.2, 1)") !== -1, "marker has smooth width transition");

// Test row hover expanding left indicator line to 22px and restoring to 14px
assert.strictEqual(typeof row.onmouseenter, "function");
assert.strictEqual(typeof row.onmouseleave, "function");
row.onmouseenter();
assert.strictEqual(marker.style.width, "22px", "hover expands indicator line to 22px");
assert.strictEqual(marker.style.background, "ACTIVE");
assert.ok(row.className.indexOf("swiss-navigator-row-hover") !== -1);
row.onmouseleave();
assert.strictEqual(marker.style.width, "14px", "leaving restores indicator line to 14px");
assert.strictEqual(row.className.indexOf("swiss-navigator-row-hover"), -1);
assert.strictEqual(row.style.background, "ROW_BG", "active row keeps active background");

const label = row.children[1];
assert.strictEqual(label.className, "swiss-navigator-row-text");
assert.strictEqual(label.textContent, "cached prompt text");
assert.ok(label.style.cssText.indexOf("white-space: nowrap") !== -1, "rows must stay on a single line");
assert.ok(label.style.cssText.indexOf("text-overflow: ellipsis") !== -1, "rows must ellipsize");
assert.ok(label.style.cssText.indexOf("text-align: left") !== -1, "rows must be left-aligned");
assert.ok(label.style.cssText.indexOf("text-align: right") === -1, "the reference screenshot's mirrored direction is wrong");

row.onclick();
assert.deepStrictEqual(activated, [4], "clicking a row must activate that turn");

// Unknown turns: muted ellipsis label, inactive default-colored marker.
const unknownRow = createNavigatorRow(doc, 5, "", false, ui, () => {});
assert.strictEqual(unknownRow.className, "swiss-navigator-row");
assert.strictEqual(unknownRow.style.background, "transparent");
assert.strictEqual(unknownRow.children[1].textContent, "…");
assert.strictEqual(unknownRow.children[1].style.color, "MUTED");
assert.strictEqual(unknownRow.children[0].className, "swiss-navigator-row-dash");
assert.strictEqual(unknownRow.children[0].style.height, "1.5px");
assert.strictEqual(unknownRow.children[0].style.background, "DEFAULT");

// Inactive row hover expands marker and applies hover background, then restores
unknownRow.onmouseenter();
assert.strictEqual(unknownRow.children[0].style.width, "22px");
assert.strictEqual(unknownRow.children[0].style.background, "ACTIVE");
assert.strictEqual(unknownRow.style.background, "HOVER_ROW_BG");
assert.ok(unknownRow.className.indexOf("swiss-navigator-row-hover") !== -1);
unknownRow.onmouseleave();
assert.strictEqual(unknownRow.children[0].style.width, "14px");
assert.strictEqual(unknownRow.children[0].style.background, "DEFAULT");
assert.strictEqual(unknownRow.style.background, "transparent");
assert.strictEqual(unknownRow.className.indexOf("swiss-navigator-row-hover"), -1);

console.log("nav panel helpers OK");
`

	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(driver)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("chat navigator helper test failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "nav panel helpers OK") {
		t.Fatalf("expected the helper driver to run, got:\n%s", string(out))
	}
}

// TestChatNavigatorPanel_IsolationAndDismiss pins the hover panel's lifecycle:
// fixed-position element attached to document.body (so the chat pane cannot clip
// it), positioned over the rail at rect.left and clamped to the viewport, opened
// and closed through a ~150ms grace timer, dismissed on transcript scroll,
// Escape or rail rebuild, and jumping through the exact dash click path.
func TestChatNavigatorPanel_IsolationAndDismiss(t *testing.T) {
	script := GenerateEnhancementsScript(DefaultConfig())

	required := []string{
		`id = "swiss-prompt-navigator-panel"`,
		`panel.setAttribute("data-swiss-component", "prompt-navigator-panel")`,
		`panel.setAttribute("role", "menu")`,
		`panel.style.cssText = "position: fixed; z-index: 10001;`,
		`document.body.appendChild(panel)`,
		`let left = rect.left;`,
		`bar.setAttribute("data-swiss-panel-open", "true");`,
		`bar.style.opacity = "0";`,
		`bar.style.visibility = "hidden";`,
		`bar.style.pointerEvents = "none";`,
		`bar.removeAttribute("data-swiss-panel-open");`,
		`bar.style.opacity = "1";`,
		`bar.style.visibility = "visible";`,
		`bar.style.pointerEvents = "auto";`,
		`bar.onmouseenter = () => { scheduleOpenPromptNavigatorPanel(); };`,
		`bar.onmouseleave = () => { scheduleClosePromptNavigatorPanel(); };`,
		`gap: 7px; padding: 4px 2px;`,
		`.swiss-prompt-dash::before`,
		`position: relative !important;`,
		`position: relative; width: `,
		`marker.style.width = hoverWidth + "px";`,
		`marker.style.width = ui.dashWidth + "px";`,
		`vh * 0.6`,
		`const NAV_PANEL_GRACE_MS = 150;`,
		`panel.onmouseenter = cancelNavigatorPanelClose;`,
		`panel.onmouseleave = scheduleClosePromptNavigatorPanel;`,
		`function onNavigatorPanelOutsideScroll(e)`,
		`viewport.addEventListener("scroll", onNavigatorPanelOutsideScroll`,
		`if (panel && typeof panel.remove === "function") panel.remove();`,
		`function triggerNavigatorJump(turnIdx)`,
		`dash.click();`,
		`'.swiss-prompt-dash[data-prompt-index="' + turnIdx + '"]'`,
		`bar.setAttribute("tabindex", "0");`,
		`function handleNavigatorKeydown(e)`,
		`if (e.key === "Escape") {`,
		`if (e.key === "ArrowDown" || e.key === "ArrowUp") {`,
		`if (e.key === "Enter") {`,
		`if (e.key === "Escape")`,
	}
	for _, want := range required {
		if !strings.Contains(script, want) {
			t.Errorf("expected the chat navigator panel to contain %q", want)
		}
	}

	// The panel is created exactly once, inside the opener: while closed there is
	// no element in the DOM at all, so it cannot intercept pointer events.
	if got := strings.Count(script, `id = "swiss-prompt-navigator-panel"`); got != 1 {
		t.Errorf("expected a single panel creation site, got %d", got)
	}
	if got := strings.Count(script, "}, NAV_PANEL_GRACE_MS);"); got != 2 {
		t.Errorf("expected the grace delay on both open and close timers, got %d", got)
	}

	// The panel is torn down on disable, on an empty rail, on rail rebuild, and
	// when switching to line mode.
	if got := strings.Count(script, "closePromptNavigatorPanel();"); got < 5 {
		t.Errorf("expected the panel to be dropped on teardown/rebuild paths, got %d close call sites", got)
	}

	if !strings.Contains(script, "document.addEventListener(\"keydown\", (e) => handleNavigatorKeydown(e), true);") {
		t.Errorf("expected a document-level capture keydown listener for the navigator")
	}
}

func TestPromptJumpBar_HidesOutsideChatView(t *testing.T) {
	script := GenerateEnhancementsScript(DefaultConfig())
	startMarker := "function renderPromptJumpBar() {"
	endMarker := "function isNavigatorPanelMode() {"
	startIdx := strings.Index(script, startMarker)
	endIdx := strings.Index(script, endMarker)
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		t.Fatalf("expected renderPromptJumpBar to be extractable")
	}
	renderFn := script[startIdx:endIdx]

	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}

	driver := `
const assert = require("assert");
class El {
  constructor() {
    this.style = {};
    this.attributes = {};
    this.parentElement = null;
  }
  setAttribute(k, v) { this.attributes[k] = v; }
  removeAttribute(k) { delete this.attributes[k]; }
  getAttribute(k) { return this.attributes[k] !== undefined ? this.attributes[k] : null; }
  querySelectorAll() { return []; }
  remove() {}
}

const body = new El();
const bar = new El();
const tooltip = new El();
bar.parentElement = body;

const document = {
  body: body,
  getElementById: (id) => {
    if (id === "swiss-prompt-jump-bar") return bar;
    if (id === "swiss-prompt-tooltip") return tooltip;
    return null;
  },
  querySelector: () => null,
  querySelectorAll: () => []
};
const enhConfig = { enabled: true, prompt_jump_bar: { enabled: true } };
const chatNavigatorMode = "panel";
const normalizeNavigatorMode = () => "panel";
const getJumpBarSteps = () => [];
const isMainStageActive = () => false;
let closeCalls = 0;
const closePromptNavigatorPanel = () => { closeCalls++; };

` + renderFn + `

renderPromptJumpBar();
assert.strictEqual(bar.style.display, "none", "a page without conversation-view or prompt steps must hide the rail");
assert.strictEqual(tooltip.style.display, "none", "the old tooltip must not survive navigation away from chat");
assert.strictEqual(closeCalls, 1, "the navigator panel must be closed outside chat");
console.log("non-chat jump bar gate OK");
`

	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(driver)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("non-chat jump bar test failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "non-chat jump bar gate OK") {
		t.Fatalf("expected the non-chat jump bar driver to complete, got:\n%s", string(out))
	}
}

// TestChatNavigatorPanel_Runtime drives the extracted panel logic (helpers +
// logic between the __SWISS_NAV_PANEL_* markers) inside a mock DOM: the panel
// lands on document.body as a fixed element to the right of the rail, lists the
// cached prompts with the active turn highlighted, is opened/closed through the
// grace timer, jumps through the dash click path, and closes on Escape.
func TestChatNavigatorPanel_Runtime(t *testing.T) {
	script := GenerateEnhancementsScript(DefaultConfig())

	logicStart := "/* __SWISS_NAV_PANEL_LOGIC_START__ */"
	logicEnd := "/* __SWISS_NAV_PANEL_LOGIC_END__ */"
	startIdx := strings.Index(script, logicStart)
	endIdx := strings.Index(script, logicEnd)
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		t.Fatalf("expected the panel logic to be delimited by %q and %q", logicStart, logicEnd)
	}
	logic := script[startIdx+len(logicStart) : endIdx]

	helpersStart := "/* __SWISS_NAV_PANEL_HELPERS_START__ */"
	helpersEnd := "/* __SWISS_NAV_PANEL_HELPERS_END__ */"
	helpersStartIdx := strings.Index(script, helpersStart)
	helpersEndIdx := strings.Index(script, helpersEnd)
	if helpersStartIdx == -1 || helpersEndIdx == -1 || helpersEndIdx <= helpersStartIdx {
		t.Fatalf("expected the pure navigator helpers to be delimited")
	}
	helpers := script[helpersStartIdx+len(helpersStart) : helpersEndIdx]

	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}

	driver := `
const assert = require("assert");

function matches(el, sel) {
  if (sel.charAt(0) === "." && sel.indexOf("[") === -1) {
    return el.className.split(" ").indexOf(sel.substring(1)) !== -1;
  }
  if (sel.indexOf('[data-prompt-index="') !== -1) {
    const m = sel.match(/\[data-prompt-index="(\d+)"\]/);
    return el.className.split(" ").indexOf("swiss-prompt-dash") !== -1 && el.getAttribute("data-prompt-index") === m[1];
  }
  if (sel === '[role="menuitem"]') return el.getAttribute("role") === "menuitem";
  return false;
}
function collect(root, sel, out) {
  for (const child of root.children) {
    if (matches(child, sel)) out.push(child);
    collect(child, sel, out);
  }
  return out;
}
class El {
  constructor(tag) {
    this.tagName = tag;
    this.id = "";
    this.className = "";
    this.style = {};
    this.attributes = {};
    this.children = [];
    this.textContent = "";
    this.parentElement = null;
    this.clicks = 0;
    const self = this;
    this.classList = {
      contains: (c) => self.className.split(" ").indexOf(c) !== -1,
      add: (c) => { if (self.className.split(" ").indexOf(c) === -1) self.className = (self.className + " " + c).trim(); },
      remove: (c) => { self.className = self.className.split(" ").filter((x) => x && x !== c).join(" "); }
    };
  }
  setAttribute(k, v) { this.attributes[k] = v; if (k === "id") this.id = v; }
  getAttribute(k) { return this.attributes[k] !== undefined ? this.attributes[k] : null; }
  removeAttribute(k) { delete this.attributes[k]; }
  appendChild(c) { c.parentElement = this; this.children.push(c); return c; }
  remove() {
    if (this.parentElement) {
      const i = this.parentElement.children.indexOf(this);
      if (i !== -1) this.parentElement.children.splice(i, 1);
      this.parentElement = null;
    }
  }
  querySelectorAll(sel) { return collect(this, sel, []); }
  querySelector(sel) { const all = this.querySelectorAll(sel); return all.length ? all[0] : null; }
  contains(node) { if (node === this) return true; return this.children.some((c) => c.contains(node)); }
  closest(sel) { let cur = this; while (cur) { if (matches(cur, sel)) return cur; cur = cur.parentElement; } return null; }
  click() { this.clicks++; if (typeof this.onclick === "function") this.onclick(); }
  focus() { document.activeElement = this; }
  scrollIntoView() { this.scrolledIntoView = true; }
  getBoundingClientRect() { return { top: 100, left: 18, right: 40, bottom: 400, width: 22, height: 300 }; }
}

const body = new El("body");
const bar = new El("div");
bar.id = "swiss-prompt-jump-bar";
bar.style.display = "flex";
bar.setAttribute("data-prompt-index-holder", "1");
const dashes = [];
for (let i = 0; i < 3; i++) {
  const dash = new El("div");
  dash.className = "swiss-prompt-dash" + (i === 0 ? " active" : "");
  dash.setAttribute("data-prompt-index", String(i));
  bar.appendChild(dash);
  dashes.push(dash);
}
body.appendChild(bar);

const windowListeners = [];
const document = {
  body: body,
  activeElement: null,
  createElement: (tag) => new El(tag),
  getElementById: (id) => {
    if (id === "swiss-prompt-jump-bar") return bar;
    const found = body.children.filter((c) => c.id === id);
    return found.length ? found[0] : null;
  },
  querySelector: () => null,
  querySelectorAll: () => []
};
const window = {
  innerWidth: 1024,
  innerHeight: 768,
  addEventListener: (type, handler, options) => windowListeners.push({ type: type, handler: handler, options: options }),
  removeEventListener: (type, handler, options) => {
    const i = windowListeners.findIndex((l) => l.type === type && l.handler === handler);
    if (i !== -1) windowListeners.splice(i, 1);
  }
};
const enhConfig = { prompt_jump_bar: { dash_width: 14, dash_thickness: 2.5, inactive_thickness: 1.5 } };
const isDarkMode = () => false;
const getActiveAndHoverColor = () => "ACTIVE";
let chatNavigatorMode = "panel";
let navigatorPanelOpenTimer = null;
let navigatorPanelCloseTimer = null;
let navigatorPanelDismissTargets = [];
const jumpBarPromptTextCache = {};
let lastJumpBarConversationKey = "conv-a";
let lastJumpBarTotal = 3;

` + helpers + `
` + logic + `

const getPanel = () => document.getElementById("swiss-prompt-navigator-panel");
const wait = (ms) => new Promise((r) => setTimeout(r, ms));

(async () => {
  cachePromptText(jumpBarPromptTextCache, "conv-a", 0, "first prompt");
  cachePromptText(jumpBarPromptTextCache, "conv-a", 2, "third prompt");

  // The rail keeps a panel attached to document.body to the RIGHT of the rail.
  openPromptNavigatorPanel();
  const panel = getPanel();
  assert.ok(panel, "the hover panel must be created in panel mode");
  assert.strictEqual(panel.parentElement, body, "the panel must live in document.body (chat pane overflow can clip it)");
  assert.ok(panel.style.cssText.indexOf("position: fixed") !== -1, "the panel must be fixed-positioned");
  assert.ok(panel.style.cssText.indexOf("overflow-y: auto") !== -1, "long lists must scroll");
  assert.ok(panel.style.cssText.indexOf("pointer-events: auto") !== -1);
  assert.strictEqual(panel.getAttribute("role"), "menu");
  assert.strictEqual(panel.style.left, "18px", "panel overlays the rail at rect.left (18px)");
  assert.strictEqual(panel.style.top, "100px", "panel aligns with the rail top");

  assert.strictEqual(bar.getAttribute("data-swiss-panel-open"), "true");
  assert.strictEqual(bar.style.opacity, "0");
  assert.strictEqual(bar.style.visibility, "hidden");
  assert.strictEqual(bar.style.pointerEvents, "none");

  const rows = panel.querySelectorAll(".swiss-navigator-row");
  assert.strictEqual(rows.length, 3, "one row per turn");
  assert.strictEqual(rows[0].className, "swiss-navigator-row swiss-navigator-row-active", "the active turn is highlighted");
  assert.strictEqual(rows[1].className, "swiss-navigator-row");
  assert.strictEqual(rows[0].querySelector(".swiss-navigator-row-text").textContent, "first prompt");
  assert.strictEqual(rows[1].querySelector(".swiss-navigator-row-text").textContent, "…", "unmounted turns show the muted ellipsis");
  assert.strictEqual(rows[2].querySelector(".swiss-navigator-row-text").textContent, "third prompt");

  // Row hover expands indicator line to 22px
  const marker0 = rows[0].querySelector(".swiss-navigator-row-dash");
  assert.strictEqual(marker0.style.width, "14px");
  rows[0].onmouseenter();
  assert.strictEqual(marker0.style.width, "22px", "row hover expands marker to 22px");
  assert.ok(rows[0].className.indexOf("swiss-navigator-row-hover") !== -1);
  rows[0].onmouseleave();
  assert.strictEqual(marker0.style.width, "14px", "row leave restores marker to 14px");
  assert.strictEqual(rows[0].className.indexOf("swiss-navigator-row-hover"), -1);

  assert.strictEqual(rows[0].getAttribute("role"), "menuitem");
  assert.strictEqual(rows[2].getAttribute("data-prompt-index"), "2");

  // Clicking a row jumps through the exact dash click path.
  rows[2].click();
  assert.strictEqual(dashes[2].clicks, 1, "row clicks must go through the dash click handler");

  // Closing removes the element entirely (nothing to intercept pointer events).
  closePromptNavigatorPanel();
  assert.strictEqual(getPanel(), null, "closing must remove the panel from the document");
  assert.strictEqual(windowListeners.length, 0, "dismiss listeners must be detached on close");
  assert.strictEqual(bar.getAttribute("data-swiss-panel-open"), null);
  assert.strictEqual(bar.style.opacity, "1");
  assert.strictEqual(bar.style.visibility, "visible");
  assert.strictEqual(bar.style.pointerEvents, "auto");

  // Line mode refuses to open a panel.
  chatNavigatorMode = "line";
  scheduleOpenPromptNavigatorPanel();
  await wait(200);
  assert.strictEqual(getPanel(), null, "line mode must never open the panel");

  // Panel mode opens after the ~150ms grace delay, and closes after the same
  // grace so the pointer can travel from the rail into the panel.
  chatNavigatorMode = "panel";
  scheduleOpenPromptNavigatorPanel();
  assert.strictEqual(getPanel(), null, "the open must wait for the grace delay");
  await wait(220);
  assert.ok(getPanel(), "the panel must open after the grace delay");
  assert.strictEqual(bar.getAttribute("data-swiss-panel-open"), "true");
  assert.strictEqual(bar.style.opacity, "0");
  scheduleClosePromptNavigatorPanel();
  assert.ok(getPanel(), "the close must wait for the grace delay");
  await wait(220);
  assert.strictEqual(getPanel(), null, "the panel must close after the grace delay");
  assert.strictEqual(bar.getAttribute("data-swiss-panel-open"), null);
  assert.strictEqual(bar.style.opacity, "1");

  // Keyboard: ArrowDown on the rail opens the panel and walks the rows.
  handleNavigatorKeydown({ key: "ArrowDown", target: bar, preventDefault: () => {} });
  const reopened = getPanel();
  assert.ok(reopened, "ArrowDown on the rail must open the panel");
  const reopenedRows = reopened.querySelectorAll(".swiss-navigator-row");
  assert.strictEqual(document.activeElement, reopenedRows[1], "ArrowDown moves focus to the next row");
  handleNavigatorKeydown({ key: "ArrowDown", target: document.activeElement, preventDefault: () => {} });
  assert.strictEqual(document.activeElement, reopenedRows[2], "ArrowDown keeps walking the rows");

  // Enter jumps the focused row through the dash path.
  handleNavigatorKeydown({ key: "Enter", target: document.activeElement, preventDefault: () => {} });
  assert.strictEqual(dashes[2].clicks, 2, "Enter must jump via the dash click path");

  // Escape closes the panel.
  handleNavigatorKeydown({ key: "Escape", target: bar, preventDefault: () => {} });
  assert.strictEqual(getPanel(), null, "Escape must close the panel");

  // The keyboard path honours the "last seen turn" highlight for the active row.
  rows[0].classList.remove("swiss-navigator-row-active");
  dashes[1].classList.add("active");
  dashes[0].classList.remove("active");
  openPromptNavigatorPanel();
  const panel2 = getPanel();
  assert.strictEqual(panel2.querySelector(".swiss-navigator-row-active").getAttribute("data-prompt-index"), "1", "the panel must highlight the rail's active turn");
  updateNavigatorPanelActiveHighlight(2);
  assert.strictEqual(panel2.querySelector(".swiss-navigator-row-active").getAttribute("data-prompt-index"), "2", "the highlight must follow the rail");
  closePromptNavigatorPanel();

  console.log("nav panel runtime OK");
})().catch((e) => { console.error(e); process.exit(1); });
`

	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(driver)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("chat navigator runtime test failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "nav panel runtime OK") {
		t.Fatalf("expected the runtime driver to complete, got:\n%s", string(out))
	}
}

// TestPromptJumpBar_StableTurnEnumeration pins the "jumper" contract: the rail
// must show one bar per real user prompt in the main conversation, with a count
// that stays stable while virtualization changes the mounted window but yields
// to the authoritative transcript prompt count when stale state is disproved.
func TestPromptJumpBar_StableTurnEnumeration(t *testing.T) {

	script := GenerateEnhancementsScript(DefaultConfig())

	// 1. The rail no longer derives its bar count from the mounted DOM alone.
	if strings.Contains(script, "if (existingDashes.length === steps.length && lastRenderedStepsCount === steps.length)") {
		t.Errorf("expected the jump bar rebuild guard to use the stable turn total instead of the mounted step count")
	}
	if !strings.Contains(script, "existingDashes.length === totalTurns &&") {
		t.Errorf("expected the jump bar to rebuild only when the stable turn total changes")
	}
	if !strings.Contains(script, "lastJumpBarConversationKey === conversationKey") {
		t.Errorf("expected the jump bar to rebuild when the conversation changes even if the bar count matches")
	}
	if !strings.Contains(script, "for (let turnIdx = 0; turnIdx < totalTurns; turnIdx++)") {
		t.Errorf("expected one dash per turn, rendered in fixed ascending order")
	}
	if !strings.Contains(script, "jumpBarTurnRegistry") {
		t.Errorf("expected a per-conversation high-water registry so the rail never shrinks mid-scroll")
	}
	if !strings.Contains(script, "jumpBarPromptCountsByConversation[targetId] = data.user_prompt_count;") {
		t.Errorf("expected the daemon chat-metrics user-prompt count to feed the jump bar")
	}
	if strings.Contains(script, "jumpBarPromptCountsByConversation[targetId] = data.turns.length;") {
		t.Errorf("assistant execution turns must not size the prompt navigator")
	}
	if strings.Contains(script, `steps = Array.from(document.querySelectorAll('[data-testid="user-input-step"]'));`) {
		t.Errorf("prompt detection must never fall back to the whole document")
	}

	// 1b. The active dash must stay in turn-index space: the click jump resolves in
	// the turn space, while getLowestPromptOnScreen keeps step-array indexing.
	if !strings.Contains(script, "if (isClickJumping && clickTargetIdx >= 0) return clickTargetIdx;") {
		t.Errorf("expected the click jump to pin the active dash in turn-index space")
	}
	if got := strings.Count(script, "return clickTargetIdx;"); got != 1 {
		t.Errorf("expected exactly one clickTargetIdx return (inside resolveActiveTurnIndex), got %d", got)
	}

	// 2. Behaviour of the extracted pure helpers under a virtualized transcript.
	startMarker := "/* __SWISS_JUMP_BAR_HELPERS_START__ */"
	endMarker := "/* __SWISS_JUMP_BAR_HELPERS_END__ */"
	startIdx := strings.Index(script, startMarker)
	endIdx := strings.Index(script, endMarker)
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		t.Fatalf("expected the pure jump bar helpers to be delimited by %q and %q", startMarker, endMarker)
	}
	helpers := script[startIdx+len(startMarker) : endIdx]

	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}

	driver := helpers + `
const assert = require("assert");

function mockStep(ordinal, holderOrdinal, opts) {
  opts = opts || {};
  return {
    textContent: opts.text || "",
    parentElement: opts.parentElement || null,
    getAttribute: (k) => {
      if (k === "data-index" && ordinal !== null && ordinal !== undefined) return String(ordinal);
      if (k === "aria-posinset" && ordinal !== null && ordinal !== undefined) return String(ordinal);
      if (opts.attrs && Object.prototype.hasOwnProperty.call(opts.attrs, k)) return opts.attrs[k];
      return null;
    },
    closest: (sel) => {
      if (sel === "[data-index]" && holderOrdinal !== null && holderOrdinal !== undefined) {
        return { getAttribute: (k) => (k === "data-index" ? String(holderOrdinal) : null) };
      }
      if (sel === '[data-testid="conversation-view"]') return opts.view || null;
      if (sel === '[data-testid="subagent-node"]') return opts.subagent ? {} : null;
      if (sel === '[data-testid="agent-input-box"], .no-focus-agent-input') return opts.inputBox ? {} : null;
      if (sel === '[data-testid="user-input-step"]') return opts.parentStep || null;
      return null;
    },
    querySelector: (sel) => sel === ".whitespace-pre-wrap" ? { textContent: opts.text || "" } : null
  };
}

// A virtualized window of a 9-turn session only mounts turns 7 and 8.
let ordinals = computeTurnOrdinals([mockStep(7), mockStep(8)]);
assert.deepStrictEqual(ordinals, [7, 8], "explicit list ordinals must be read");
assert.strictEqual(computeStableTurnTotal(ordinals, 2, null, "conv-a", jumpBarTurnRegistry), 9, "without daemon data, the rail must size itself from the mounted ordinals");

// Scrolling up mounts a different window of the same session: count must not change.
const scrolled = computeStableTurnTotal(computeTurnOrdinals([mockStep(2), mockStep(3)]), 2, null, "conv-a", jumpBarTurnRegistry);
assert.strictEqual(scrolled, 9, "the bar count must stay stable while the virtualized window moves");

// Turns without ordinals: the daemon prompt count covers the whole session.
const daemonTotal = computeStableTurnTotal(computeTurnOrdinals([mockStep(null), mockStep(null)]), 2, 12, "conv-b", jumpBarTurnRegistry);
assert.strictEqual(daemonTotal, 12, "the daemon prompt count must size the rail when the DOM exposes no ordinals");

// Without ordinals and without the daemon, the count still must not shrink mid-session.
const first = computeStableTurnTotal(computeTurnOrdinals([mockStep(null), mockStep(null), mockStep(null)]), 3, null, "conv-c", jumpBarTurnRegistry);
assert.strictEqual(first, 3, "three mounted turns must produce three bars");
const remounted = computeStableTurnTotal(computeTurnOrdinals([mockStep(null)]), 1, null, "conv-c", jumpBarTurnRegistry);
assert.strictEqual(remounted, 3, "the rail must not drop bars when a turn is unmounted by the virtualized list");

// Ordinals are also read from a wrapping list item (data-index on the holder).
assert.deepStrictEqual(computeTurnOrdinals([mockStep(null, 4)]), [4], "holder data-index ordinals must be read");

// Switching conversations must use that conversation's own count.
const otherConv = computeStableTurnTotal(computeTurnOrdinals([mockStep(null), mockStep(null)]), 2, null, "conv-d", jumpBarTurnRegistry);
assert.strictEqual(otherConv, 2, "a new conversation must not inherit the previous conversation bar count");

// A stale high-water mark must yield to the authoritative prompt count. A single
// real prompt can produce many assistant/system execution turns, but only one
// navigator row may remain.
const onePromptWithLargeOrdinal = computeStableTurnTotal([7], 1, 1, "conv-stale", jumpBarTurnRegistry);
assert.strictEqual(onePromptWithLargeOrdinal, 1, "the authoritative prompt count must remove ghost rows and ignore oversized ordinals");
const grownBeforeTranscriptCatchup = computeStableTurnTotal([1], 2, 1, "conv-stale", jumpBarTurnRegistry);
assert.strictEqual(grownBeforeTranscriptCatchup, 2, "a newly mounted prompt may appear before the transcript refresh");
const shrunkAgain = computeStableTurnTotal([0], 1, 1, "conv-stale", jumpBarTurnRegistry);
assert.strictEqual(shrunkAgain, 1, "the registry must shrink again when stale DOM evidence disappears");

// Steps are mapped onto their turn indexes, in insertion order, without duplicates.
assert.deepStrictEqual(mapStepsToTurnIndexes([7, 8], 9), [7, 8], "ordinal steps must keep their turn indexes");
assert.deepStrictEqual(mapStepsToTurnIndexes([null, null], 3), [0, 1], "steps without ordinals must map positionally");
assert.deepStrictEqual(mapStepsToTurnIndexes([5, null, 5], 7), [5, 0, 1], "duplicate ordinals must be de-duplicated and gaps filled");

// Only prompt nodes in the main conversation survive. Nested conversation views,
// inline subagent transcripts, editor inputs, and duplicate clones are excluded.
const mainView = { contains: () => true };
const nestedView = { contains: () => true };
const firstPrompt = mockStep(0, null, { view: mainView, text: "first prompt" });
const duplicatePrompt = mockStep(0, null, { view: mainView, text: "first prompt" });
const repeatedTextPrompt = mockStep(3, null, { view: mainView, text: "first prompt" });
const nestedConversationPrompt = mockStep(1, null, { view: nestedView, text: "nested prompt" });
const inlineSubagentPrompt = mockStep(2, null, { view: mainView, subagent: true, text: "subagent prompt" });
const inputPrompt = mockStep(4, null, { view: mainView, inputBox: true, text: "input draft" });
const nestedStepParent = { closest: (sel) => sel === '[data-testid="user-input-step"]' ? {} : null };
const holderPrompt = mockStep(null, null, { view: mainView, parentElement: nestedStepParent, text: "nested duplicate" });
const filtered = filterMainConversationSteps([
  firstPrompt,
  duplicatePrompt,
  repeatedTextPrompt,
  firstPrompt,
  nestedConversationPrompt,
  inlineSubagentPrompt,
  inputPrompt,
  holderPrompt
], mainView);
assert.deepStrictEqual(filtered, [firstPrompt, repeatedTextPrompt], "main-conversation prompt detection must exclude nested, subagent, input, and duplicate nodes");

console.log("jump-bar helpers OK");
`

	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(driver)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("jump bar helper test failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "jump-bar helpers OK") {
		t.Fatalf("expected the helper driver to run, got:\n%s", string(out))
	}
}

func TestOverviewPanel_Divider(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "enhancements-overview-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "enhancements.json")
	store, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := store.GetConfig()
	if !cfg.OverviewPanel.ReplaceSeeAllTriangle {
		t.Errorf("expected ReplaceSeeAllTriangle to default to true")
	}

	// Update and verify persistence
	cfg.OverviewPanel.ReplaceSeeAllTriangle = false
	if err := store.UpdateConfig(cfg); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	store2, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to load store2: %v", err)
	}
	loaded := store2.GetConfig()
	if loaded.OverviewPanel.ReplaceSeeAllTriangle {
		t.Errorf("expected ReplaceSeeAllTriangle false to persist")
	}
}

func TestOverviewPanel_AuxTabsFormat(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.OverviewPanel.AuxTabsFormat != "icon" {
		t.Errorf("expected AuxTabsFormat to be 'icon' by default, got %s", cfg.OverviewPanel.AuxTabsFormat)
	}

	tmpDir, err := os.MkdirTemp("", "enhancements-aux-tabs-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(filepath.Join(tmpDir, "enhancements.json"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	if err := store.SetAuxTabsFormat("icon_and_name"); err != nil {
		t.Fatalf("failed to set aux tabs format: %v", err)
	}

	if store.GetConfig().OverviewPanel.AuxTabsFormat != "icon_and_name" {
		t.Errorf("expected AuxTabsFormat to be 'icon_and_name', got %s", store.GetConfig().OverviewPanel.AuxTabsFormat)
	}

	// Invalid fallback to "icon"
	if err := store.SetAuxTabsFormat("something_else"); err != nil {
		t.Fatalf("failed to set aux tabs format: %v", err)
	}
	if store.GetConfig().OverviewPanel.AuxTabsFormat != "icon" {
		t.Errorf("expected AuxTabsFormat fallback to 'icon', got %s", store.GetConfig().OverviewPanel.AuxTabsFormat)
	}
}

func TestGenerateEnhancementsScript_Syntax(t *testing.T) {
	js := GenerateEnhancementsScript(nil)
	if js == "" {
		t.Fatalf("expected non-empty JS script")
	}

	if _, err := exec.LookPath("node"); err == nil {
		cmd := exec.Command("node", "--check")
		cmd.Stdin = strings.NewReader(js)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("node syntax error in GenerateEnhancementsScript: %v\n%s", err, string(out))
		}
	}
}

func TestGenerateEnhancementsScript_TerminalScopePreservesFactoryStyle(t *testing.T) {
	cfg := DefaultConfig()
	script := GenerateEnhancementsScript(cfg)

	// 1. Ensure "Terminals" is NOT treated as an overview panel section title
	if strings.Contains(script, "\"Terminals\"") {
		t.Errorf("expected script NOT to include 'Terminals' in overview titles so terminal scope selector retains factory style")
	}

	// 2. Ensure script explicitly protects terminal scope selector from custom zone styling
	if !strings.Contains(script, "terminal scope selector") {
		t.Errorf("expected script to explicitly reference and protect terminal scope selector")
	}
}

func TestOverviewPanel_DefaultDivisionStyle_IsDividerLine(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.OverviewPanel.DivisionStyle != "divider_line" {
		t.Errorf("expected default DivisionStyle to be 'divider_line', got %s", cfg.OverviewPanel.DivisionStyle)
	}

	script := GenerateEnhancementsScript(cfg)
	if !strings.Contains(script, "swiss-overview-divider") {
		t.Errorf("expected script to contain swiss-overview-divider")
	}
	if !strings.Contains(script, "swiss-overview-zone") {
		t.Errorf("expected script to contain cleanup logic for swiss-overview-zone")
	}
}

func TestGenerateEnhancementsScript_ContrastSafeguard(t *testing.T) {
	cfg := DefaultConfig()
	script := GenerateEnhancementsScript(cfg)

	if !strings.Contains(script, `if (dark && lum < 0.15)`) || !strings.Contains(script, `return "#e2e8f0";`) {
		t.Errorf("expected dark-mode luminance safeguard (lum < 0.15 -> #e2e8f0) in getActiveAndHoverColor")
	}
	if !strings.Contains(script, `if (!dark && lum > 0.85)`) || !strings.Contains(script, `return "#334155";`) {
		t.Errorf("expected light-mode luminance safeguard (lum > 0.85 -> #334155) in getActiveAndHoverColor")
	}
}

func TestOverviewPanel_ShowMoreButton_TriangleOnlyNoHorizontalLine(t *testing.T) {
	cfg := DefaultConfig()
	script := GenerateEnhancementsScript(cfg)

	if !strings.Contains(script, "swiss-overview-tabs-triangle") {
		t.Errorf("expected script to contain swiss-overview-tabs-triangle")
	}
	if !strings.Contains(script, "swiss-overview-tabs-divider") {
		t.Errorf("expected script to contain swiss-overview-tabs-divider")
	}
	// The show more/less button in the overview panel must NOT render a horizontal line
	if strings.Contains(script, "<div class=\"swiss-overview-tabs-line\"></div>") {
		t.Errorf("expected overview show more/less button HTML NOT to contain swiss-overview-tabs-line horizontal line element")
	}
	if strings.Contains(script, ".swiss-overview-tabs-line {") {
		t.Errorf("expected overview CSS NOT to define .swiss-overview-tabs-line rule")
	}

	// Verify both "See all/less" and "Show more/less" text patterns are matched
	if !strings.Contains(script, `isMore = /^(see|show)\s+(all|more)/i`) {
		t.Errorf("expected script to match both 'see' and 'show' expand patterns")
	}
	if !strings.Contains(script, `isLess = /^(see|show)\s+(less|fewer)/i`) {
		t.Errorf("expected script to match both 'less' and 'fewer' contract patterns")
	}
	if !strings.Contains(script, `lower.includes("show more")`) {
		t.Errorf("expected script to handle 'show more' in triangle direction determination")
	}
}

func TestOverviewPanel_DividerLine_PreservesFactorySectionDistance(t *testing.T) {
	cfg := DefaultConfig()

	script := GenerateEnhancementsScript(cfg)
	// Check that divider offsets compensate for flex gap to prevent blank areas and match factory spacing
	if !strings.Contains(script, "margin-top: calc(-12px") {
		t.Errorf("expected script CSS to compensate for flex gap with negative margin calculation")
	}
	if !strings.Contains(script, "netMargin") {
		t.Errorf("expected script JS to compute netMargin to match factory section gap")
	}
	if !strings.Contains(script, "min-height: 0px") || !strings.Contains(script, "overflow: hidden") {
		t.Errorf("expected divider CSS to have zero-height resets to avoid unwanted height inflation")
	}
	// Check horizontal insets so the divider line does not touch the edges
	if !strings.Contains(script, "margin-left: 6px !important;") || !strings.Contains(script, "margin-right: 6px !important;") {
		t.Errorf("expected divider CSS to include 6px horizontal margins")
	}
	if !strings.Contains(script, "width: calc(100% - 12px) !important;") {
		t.Errorf("expected divider CSS to shorten width with 12px inset calculation")
	}
	if !strings.Contains(script, `divider.style.setProperty("margin-left", "6px", "important")`) {
		t.Errorf("expected divider JS to set 6px horizontal margin-left")
	}
	if !strings.Contains(script, `divider.style.setProperty("width", "calc(100% - 12px)", "important")`) {
		t.Errorf("expected divider JS to set shortened width with 12px inset")
	}
	// Check that orphaned dividers are pruned
	if !strings.Contains(script, "sectionContainers.indexOf(next) === 0") {
		t.Errorf("expected script to prune orphaned dividers before the first section")
	}
}

func TestOverviewPanel_ShrunkSectionGapsAndCompactTriangle(t *testing.T) {
	cfg := DefaultConfig()
	script := GenerateEnhancementsScript(cfg)

	// Verify shrunken compact divider gap calculation (5px half gap -> 10px total section distance)
	if !strings.Contains(script, "desiredHalfGap = 5") {
		t.Errorf("expected script JS to compute desiredHalfGap = 5 for compact section spacing")
	}
	if !strings.Contains(script, "margin-top: calc(-12px - 7px)") {
		t.Errorf("expected script CSS to specify tightened negative margin calculation (-19px)")
	}

	// Verify tightened show-more button (8px height, 14x6px pill, 8px font-size)
	if !strings.Contains(script, `[data-swiss-overview-divider="true"] {`) {
		t.Errorf("expected script CSS to style data-swiss-overview-divider")
	}
	if strings.Contains(script, `div:has(> [data-swiss-overview-divider="true"])`) {
		t.Errorf("script CSS must NOT target div:has(> [data-swiss-overview-divider=\"true\"]) because that crushes the parent overview section container to 8px height")
	}
	if !strings.Contains(script, "height: 8px !important;") {
		t.Errorf("expected script CSS to tighten show-more button to 8px height")
	}
	if !strings.Contains(script, "width: 14px !important;") || !strings.Contains(script, "height: 6px !important;") {
		t.Errorf("expected script CSS to tighten show-more pill dimensions to 14x6px")
	}

	// Verify border_zone gap zeroing on parent container
	if !strings.Contains(script, `parent.style.setProperty("gap", "0px", "important")`) {
		t.Errorf("expected script JS to zero parent flex gap in border_zone mode")
	}

	// Verify divider lines rendered via ::before with 6px horizontal margins and -3px bottom margin compensation for section gap-2 (8px)
	if !strings.Contains(script, `margin-left: 6px !important;`) || !strings.Contains(script, `margin-right: 6px !important;`) {
		t.Errorf("expected script CSS to inset divider lines with 6px margins")
	}
	if !strings.Contains(script, `gap: 5px !important;`) {
		t.Errorf("expected .swiss-overview-parent to set gap: 5px !important for 5px top half-gap")
	}
	if !strings.Contains(script, `row-gap: 8px !important;`) {
		t.Errorf("expected section containers to lock row-gap: 8px !important so -3px bottom margin compensation is invariant")
	}
	if !strings.Contains(script, `margin-bottom: -3px !important;`) {
		t.Errorf("expected ::before divider to set margin-bottom: -3px !important so 8px flex gap-2 - 3px = 5px bottom half-gap")
	}
	if !strings.Contains(script, `margin: -7px auto 0 auto !important;`) {
		t.Errorf("expected [data-swiss-overview-divider=\"true\"] to pull top margin by -7px to tighten section gap-2 above triangle")
	}

	// Verify live re-injection disconnects and replaces stale MutationObserver rather than skipping
	if strings.Contains(script, "if (!window.__swissEnhancementsObserver)") {
		t.Errorf("script must not skip observer replacement when window.__swissEnhancementsObserver is already set")
	}
	if !strings.Contains(script, `typeof window.__swissEnhancementsObserver.disconnect === "function"`) {
		t.Errorf("expected script to disconnect existing window.__swissEnhancementsObserver on re-injection")
	}

	// Verify chat markdown blocks are excluded from overview section header detection
	if !strings.Contains(script, `el.closest('.md-divider-spacing')`) || !strings.Contains(script, `el.closest('[data-testid="autoscroll-viewport"]')`) {
		t.Errorf("expected script to exclude chat markdown and autoscroll viewport from overview section headers")
	}

	// Verify MutationObserver does not ignore React mutations inside overview section/divider containers
	if !strings.Contains(script, "const isSwissLeafElement = (n) =>") {
		t.Errorf("expected script observer to filter only dedicated Swiss leaf elements")
	}
}

func TestLeftPanelExtensions_StoreAndScript(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "enh-left-panel-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "enhancements.json")
	store, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := store.GetConfig()
	if !cfg.LeftPanelExtensionsEnabled {
		t.Errorf("expected LeftPanelExtensionsEnabled to be true by default")
	}
	if cfg.LeftPanelExtensionsMode != "single" {
		t.Errorf("expected LeftPanelExtensionsMode to be 'single' by default, got %s", cfg.LeftPanelExtensionsMode)
	}

	if err := store.ToggleLeftPanelExtensions(false); err != nil {
		t.Fatalf("failed to toggle left panel extensions: %v", err)
	}
	if store.GetConfig().LeftPanelExtensionsEnabled {
		t.Errorf("expected LeftPanelExtensionsEnabled to be false after toggle")
	}

	if err := store.SetLeftPanelExtensionsMode("individual"); err != nil {
		t.Fatalf("failed to set left panel extensions mode: %v", err)
	}
	if store.GetConfig().LeftPanelExtensionsMode != "individual" {
		t.Errorf("expected LeftPanelExtensionsMode to be 'individual'")
	}

	if !cfg.MainSectionExtensionsEnabled {
		t.Errorf("expected MainSectionExtensionsEnabled to be true by default")
	}
	if err := store.ToggleMainSectionExtensions(false); err != nil {
		t.Fatalf("failed to toggle main section extensions: %v", err)
	}
	if store.GetConfig().MainSectionExtensionsEnabled {
		t.Errorf("expected MainSectionExtensionsEnabled to be false after toggle")
	}

	curCfg := store.GetConfig()
	script := GenerateEnhancementsScript(&curCfg)
	if !strings.Contains(script, "antigravity_swiss_left_panel_enabled") {
		t.Errorf("expected script to sync antigravity_swiss_left_panel_enabled to localStorage")
	}
	if !strings.Contains(script, "antigravity_swiss_left_panel_mode") {
		t.Errorf("expected script to sync antigravity_swiss_left_panel_mode to localStorage")
	}
	if !strings.Contains(script, "antigravity_swiss_main_section_enabled") {
		t.Errorf("expected script to sync antigravity_swiss_main_section_enabled to localStorage")
	}
	if !strings.Contains(script, "swiss-left-nav-config-updated") {
		t.Errorf("expected script to dispatch swiss-left-nav-config-updated event")
	}
}

func TestExtensionsVisibility_LegacyMigration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "enh-ext-migration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "enhancements.json")

	// Legacy config without an extensions map; both global toggles disabled.
	legacy := `{
  "version": "1.0.0",
  "enabled": true,
  "left_panel_extensions_enabled": false,
  "left_panel_extensions_mode": "single",
  "main_section_extensions_enabled": false
}`
	if err := os.WriteFile(configPath, []byte(legacy), 0644); err != nil {
		t.Fatalf("failed to write legacy config: %v", err)
	}

	store, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := store.GetConfig()
	if len(cfg.Extensions) != len(ExtensionIDs) {
		t.Fatalf("expected migrated Extensions map with %d entries, got %d", len(ExtensionIDs), len(cfg.Extensions))
	}
	for _, id := range ExtensionIDs {
		vis, ok := cfg.Extensions[id]
		if !ok {
			t.Errorf("expected Extensions map to contain %q", id)
			continue
		}
		if vis.AuxPanel {
			t.Errorf("expected Extensions[%q].AuxPanel=false (left_panel_extensions_enabled was false)", id)
		}
		if vis.MainPage {
			t.Errorf("expected Extensions[%q].MainPage=false (main_section_extensions_enabled was false)", id)
		}
	}
	if cfg.LeftPanelExtensionsMode != "single" {
		t.Errorf("expected LeftPanelExtensionsMode to preserve 'single' after migration, got %s", cfg.LeftPanelExtensionsMode)
	}

	// The normalized config must be persisted once so the map is authoritative.
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to re-read persisted config: %v", err)
	}
	if !strings.Contains(string(raw), `"extensions"`) {
		t.Errorf("expected persisted enhancements.json to contain the extensions map")
	}

	// Reload: the persisted map must not be re-derived from legacy fields.
	store2, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to reload store: %v", err)
	}
	if len(store2.GetConfig().Extensions) != len(ExtensionIDs) {
		t.Errorf("expected reloaded store to keep the persisted extensions map")
	}
}

func TestExtensionsVisibility_LegacyMigration_Enabled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "enh-ext-migration-on-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "enhancements.json")

	// Legacy config with global toggles enabled and no extensions map.
	legacy := `{
  "version": "1.0.0",
  "enabled": true,
  "left_panel_extensions_enabled": true,
  "left_panel_extensions_mode": "single",
  "main_section_extensions_enabled": true
}`
	if err := os.WriteFile(configPath, []byte(legacy), 0644); err != nil {
		t.Fatalf("failed to write legacy config: %v", err)
	}

	store, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := store.GetConfig()
	for _, id := range ExtensionIDs {
		vis, ok := cfg.Extensions[id]
		if !ok {
			t.Errorf("expected Extensions map to contain %q", id)
			continue
		}
		if !vis.AuxPanel || vis.MainPage {
			t.Errorf("expected Extensions[%q] to be {true, false}, got %+v", id, vis)
		}
	}
	if cfg.LeftPanelExtensionsMode != "single" {
		t.Errorf("expected LeftPanelExtensionsMode to preserve 'single', got %s", cfg.LeftPanelExtensionsMode)
	}
}

func TestExtensionsVisibility_UpdatePreservesMap(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "enh-ext-update-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "enhancements.json")
	store, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Toggle one extension off through the map and persist.
	cfg := store.GetConfig()
	cfg.Extensions["github"] = ExtensionVisibility{AuxPanel: true, MainPage: false}
	if err := store.UpdateConfig(cfg); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	// A subsequent UpdateConfig from a client without the extensions map must
	// carry the stored map over instead of re-deriving it from legacy toggles.
	stripped := store.GetConfig()
	stripped.Extensions = nil
	if err := store.UpdateConfig(stripped); err != nil {
		t.Fatalf("failed to update config without extensions map: %v", err)
	}
	got := store.GetConfig().Extensions["github"]
	if !got.AuxPanel || got.MainPage {
		t.Errorf("expected stored extensions map to survive a legacy update, got %+v", got)
	}
}

func TestGenerateEnhancementsScript_ExtensionsMap(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Extensions["browser"] = ExtensionVisibility{AuxPanel: false, MainPage: true}
	script := GenerateEnhancementsScript(cfg)

	if !strings.Contains(script, `"extensions"`) {
		t.Errorf("expected injected __SWISS_ENH_CONFIG__ payload to carry the extensions map")
	}
	if !strings.Contains(script, `"aux_panel"`) || !strings.Contains(script, `"main_page"`) {
		t.Errorf("expected injected config to serialize aux_panel/main_page keys")
	}
}

func TestGenerateEnhancementsScript_ContextWindowProgressRing(t *testing.T) {
	cfg := DefaultConfig()
	script := GenerateEnhancementsScript(cfg)

	// Verify progress ring function definitions
	if !strings.Contains(script, "renderContextWindowProgressRing") {
		t.Errorf("expected script to contain renderContextWindowProgressRing")
	}
	if !strings.Contains(script, "getModelContextWindow") {
		t.Errorf("expected script to contain getModelContextWindow")
	}
	if !strings.Contains(script, "formatTokensUnit") {
		t.Errorf("expected script to contain formatTokensUnit")
	}
	if !strings.Contains(script, "swiss-context-window-ring") {
		t.Errorf("expected script to contain swiss-context-window-ring")
	}
	for _, token := range []string{
		"function isContextWindowIndicatorEnabled()",
		"function isContextWindowClickToCompactEnabled()",
		"function removeContextWindowProgressRings()",
		"if (!isContextWindowIndicatorEnabled() || window.__swissDaemonOnline === false || !window.__swissDaemonOnline)",
		"if (!isContextWindowClickToCompactEnabled())",
		`ring.setAttribute("role", clickToCompactEnabled ? "button" : "img")`,
		"if (clickToCompactEnabled) {",
	} {
		if !strings.Contains(script, token) {
			t.Errorf("expected context window settings guard %q", token)
		}
	}

	// Verify dynamic custom model context window lookup and 1M native default
	if !strings.Contains(script, "customModelsListCache") || !strings.Contains(script, "/api/custom_models") {
		t.Errorf("expected script to dynamically check custom models from /api/custom_models")
	}
	if !strings.Contains(script, "return 1000000") {
		t.Errorf("expected script to default native Antigravity models to 1M")
	}

	// Verify dynamic model context window sizes
	if !strings.Contains(script, `clean.includes("claude")`) || !strings.Contains(script, "200000") {
		t.Errorf("expected script to adapt Claude context window to 200K")
	}
	if !strings.Contains(script, `clean.includes("gpt-4")`) || !strings.Contains(script, "128000") {
		t.Errorf("expected script to adapt GPT-4o context window to 128K")
	}
	if !strings.Contains(script, `clean.includes("pro")`) || !strings.Contains(script, "2000000") {
		t.Errorf("expected script to adapt Gemini Pro context window to 2M")
	}
	if !strings.Contains(script, `clean.includes("flash")`) || !strings.Contains(script, "1000000") {
		t.Errorf("expected script to adapt Flash context window to 1M")
	}

	// Verify subagent progress ring rendering
	if !strings.Contains(script, "swiss-context-window-ring-sub") {
		t.Errorf("expected script to support subagent progress ring rendering")
	}

	// Verify tooltip percentage and format
	if !strings.Contains(script, `pct + "% (" + formattedUsed + "/" + formattedMax + ")"`) {
		t.Errorf("expected script to contain formatted tooltip with used and max context window")
	}

	// Verify data.summary fallback and clamped percentage
	if !strings.Contains(script, "data.summary ? (data.summary.context_tokens || ((data.summary.input_tokens || 0) + (data.summary.output_tokens || 0))) : 0") {
		t.Errorf("expected script to contain summary fallback for conversations without turns")
	}
	if !strings.Contains(script, "Math.max(0, Math.min(100, Math.round((usedTokens / validMax) * 100)))") {
		t.Errorf("expected script to clamp token percentage strictly to [0, 100]")
	}

	// Verify subagent model selector lookup
	if !strings.Contains(script, "subModelName = subTrigger ? subTrigger.innerText.trim() : \"\"") {
		t.Errorf("expected script to resolve subagent model selector when present")
	}
}

func TestGenerateEnhancementsScript_ContextWindowProgressRing_DaemonOffline(t *testing.T) {
	cfg := DefaultConfig()
	script := GenerateEnhancementsScript(cfg)

	// 1. Verify daemon offline check inside renderContextWindowProgressRing
	if !strings.Contains(script, "if (window.__swissDaemonOnline === false || !window.__swissDaemonOnline)") {
		t.Errorf("expected renderContextWindowProgressRing to guard against window.__swissDaemonOnline === false || !window.__swissDaemonOnline")
	}

	// 2. Verify daemon offline check inside fetchActiveConversationTokens
	if !strings.Contains(script, "if (window.__swissDaemonOnline === false || !window.__swissDaemonOnline) return;") {
		t.Errorf("expected fetchActiveConversationTokens to check daemon offline status before querying API")
	}

	// 3. Verify window.__swissOnEnhancementsDaemonChanged registration
	if !strings.Contains(script, "window.__swissOnEnhancementsDaemonChanged = function(online)") {
		t.Errorf("expected script to register window.__swissOnEnhancementsDaemonChanged")
	}

	// 4. Verify DOM removal of context rings when daemon is offline
	if !strings.Contains(script, `removeContextWindowProgressRings()`) || !strings.Contains(script, `swiss-context-window-ring-sub`) {
		t.Errorf("expected script to remove existing context rings when daemon is offline")
	}

	// 5. Verify Dual-Layer CSS suppression rules
	requiredCSS := []string{
		"html:not(.swiss-daemon-online) #swiss-context-window-ring",
		"html.swiss-daemon-offline #swiss-context-window-ring",
		"html:not(.swiss-daemon-online) .swiss-context-window-ring",
		"html.swiss-daemon-offline .swiss-context-window-ring",
		"html:not(.swiss-daemon-online) .swiss-inchat-metrics",
		"html.swiss-daemon-offline .swiss-inchat-metrics",
	}
	for _, sel := range requiredCSS {
		if !strings.Contains(script, sel) {
			t.Errorf("expected script to contain CSS suppression selector %q", sel)
		}
	}

	// 6. Verify auth headers function
	if !strings.Contains(script, "getSwissAuthHeaders") {
		t.Errorf("expected script to define getSwissAuthHeaders")
	}

	// 5. Node runtime execution test verifying DOM behavior when daemon is offline
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found, skipping runtime test")
	}

	harness := `
class MockElement {
  constructor(tag, id = "") {
    this.tagName = tag.toUpperCase();
    this.id = id;
    this.classList = new Set();
    this.style = {};
    this.attributes = {};
    this.children = [];
    this.parentElement = null;
    this.nextSibling = null;
    this.previousElementSibling = null;
    this.innerText = "";
  }
  setAttribute(k, v) { this.attributes[k] = v; }
  getAttribute(k) { return this.attributes[k]; }
  appendChild(child) {
    child.parentElement = this;
    this.children.push(child);
    return child;
  }
  insertBefore(newNode, refNode) {
    newNode.parentElement = this;
    const idx = refNode ? this.children.indexOf(refNode) : -1;
    if (idx === -1) {
      this.children.push(newNode);
    } else {
      this.children.splice(idx, 0, newNode);
    }
    newNode.nextSibling = refNode;
    return newNode;
  }
  remove() {
    if (this.parentElement) {
      const idx = this.parentElement.children.indexOf(this);
      if (idx !== -1) this.parentElement.children.splice(idx, 1);
      this.parentElement = null;
    }
  }
  querySelector(sel) {
    return this.children.find(c => sel.includes(c.id) || (sel.includes(".") && c.classList.has(sel.replace(".", ""))));
  }
  querySelectorAll(sel) {
    const res = [];
    for (const c of this.children) {
      if (c.id && sel.includes(c.id)) res.push(c);
      else if (sel.includes("swiss-context-window-ring") && (c.classList.has("swiss-context-window-ring") || c.classList.has("swiss-context-window-ring-sub"))) res.push(c);
    }
    return res;
  }
  closest() { return this.parentElement; }
}
const document = {
  head: new MockElement("head"),
  body: new MockElement("body"),
  createElement: (tag) => new MockElement(tag),
  getElementById: (id) => {
    if (id === "swiss-context-window-ring") return document.body.children.find(c => c.id === id) || null;
    return null;
  },
  querySelector: (sel) => {
    if (sel.includes("model-selector-trigger")) return trigger;
    return null;
  },
  querySelectorAll: (sel) => document.body.querySelectorAll(sel),
};
const trigger = new MockElement("div", "trigger");
trigger.innerText = "Gemini Pro";
document.body.appendChild(trigger);

const window = {
  location: { pathname: "/c/11111111-2222-3333-4444-555555555555" },
  __swissDaemonOnline: false,
  addEventListener: () => {},
};
const assert = require("assert");

` + script + `

// Verify that with __swissDaemonOnline = false, no ring exists
assert.strictEqual(document.getElementById("swiss-context-window-ring"), null, "ring must not be rendered when daemon is offline");

// Simulate online transition
window.__swissDaemonOnline = true;
if (typeof window.__swissOnEnhancementsDaemonChanged === "function") {
  window.__swissOnEnhancementsDaemonChanged(true);
}
// Simulate offline transition
window.__swissDaemonOnline = false;
if (typeof window.__swissOnEnhancementsDaemonChanged === "function") {
  window.__swissOnEnhancementsDaemonChanged(false);
}
assert.strictEqual(document.getElementById("swiss-context-window-ring"), null, "ring must be purged when daemon goes offline");
`

	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(harness)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node execution error in TestGenerateEnhancementsScript_ContextWindowProgressRing_DaemonOffline: %v\n%s", err, string(out))
	}
}

func TestGenerateEnhancementsScript_ContextWindowProgressRing_Deduplication(t *testing.T) {
	cfg := DefaultConfig()
	script := GenerateEnhancementsScript(cfg)

	// 1. Verify subagent input box exclusion for mainRing
	if !strings.Contains(script, "isInsideSubagentBox") {
		t.Errorf("expected script to detect if trigger is inside a subagent box")
	}

	// 2. Verify seenBoxes deduplication for subagent tags
	if !strings.Contains(script, "seenBoxes = new Set()") {
		t.Errorf("expected script to use seenBoxes Set for tag deduplication")
	}

	// 3. Verify container ring deduplication loop
	if !strings.Contains(script, "rings.length > 1") {
		t.Errorf("expected script to include defensive container ring deduplication")
	}
}

// Prompt text is user input: it must never be interpolated into innerHTML
// unescaped, otherwise a prompt containing markup executes in the host page.
func TestChatNavigatorPanel_EscapesPromptTextInTooltip(t *testing.T) {
	script := GenerateEnhancementsScript(DefaultConfig())

	if !strings.Contains(script, "function escapeNavigatorText") {
		t.Fatal("missing escapeNavigatorText helper")
	}
	if !strings.Contains(script, "escapeNavigatorText(snippet)") {
		t.Error("tooltip snippet must be escaped before interpolation")
	}
	if strings.Contains(script, "'>\" + snippet + \"</div>") {
		t.Error("raw snippet interpolation into innerHTML is still present")
	}
	for _, entity := range []string{"&amp;", "&lt;", "&gt;", "&quot;", "&#39;"} {
		if !strings.Contains(script, entity) {
			t.Errorf("escape table missing %s", entity)
		}
	}
}

func TestAutomationTargetProjectSelector_FlagOverride(t *testing.T) {
	script := GenerateEnhancementsScript(DefaultConfig())

	for _, token := range []string{
		"function isAutomationTargetProjectEnabled()",
		"const targetProjectFlag = !isAutomationTargetProjectEnabled();",
		`parsedFlags["jetski-sidecar-declared-permissions"] = targetProjectFlag;`,
	} {
		if !strings.Contains(script, token) {
			t.Errorf("expected script to contain %q", token)
		}
	}

	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}

	driver := `
const assert = require("assert");

const storage = {
  "jetski.developer.customFlagOverrides": JSON.stringify({
    "some-other-flag": true,
    "jetski-sidecar-declared-permissions": true
  })
};

const localStorage = {
  getItem: (k) => storage[k] || null,
  setItem: (k, v) => { storage[k] = v; }
};

let dispatched = null;
const window = {
  localStorage,
  dispatchEvent: (e) => { dispatched = e; }
};
function StorageEvent(type, dict) { Object.assign(this, dict); }

function applyFlagOverride(targetProjectEnabled) {
  const rawFlags = localStorage.getItem("jetski.developer.customFlagOverrides") || "{}";
  const parsedFlags = JSON.parse(rawFlags);
  const targetProjectFlag = !targetProjectEnabled;
  if (parsedFlags && typeof parsedFlags === "object" && !Array.isArray(parsedFlags) && (parsedFlags["remote-control-setting-enabled"] !== true || parsedFlags["jetski-sidecar-declared-permissions"] !== targetProjectFlag)) {
    parsedFlags["remote-control-setting-enabled"] = true;
    parsedFlags["jetski-sidecar-declared-permissions"] = targetProjectFlag;
    localStorage.setItem("jetski.developer.customFlagOverrides", JSON.stringify(parsedFlags));
  }
  return JSON.parse(localStorage.getItem("jetski.developer.customFlagOverrides"));
}

const enabled = applyFlagOverride(true);
assert.strictEqual(enabled["remote-control-setting-enabled"], true);
assert.strictEqual(enabled["jetski-sidecar-declared-permissions"], false);
assert.strictEqual(enabled["some-other-flag"], true);

storage["jetski.developer.customFlagOverrides"] = JSON.stringify({
  "remote-control-setting-enabled": true,
  "jetski-sidecar-declared-permissions": false,
  "some-other-flag": true
});
const disabled = applyFlagOverride(false);
assert.strictEqual(disabled["jetski-sidecar-declared-permissions"], true);
assert.strictEqual(disabled["some-other-flag"], true);

console.log("flag override OK");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(driver)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node execution failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "flag override OK") {
		t.Fatalf("unexpected output: %s", string(out))
	}
}

func TestAutomationTargetProjectSelector_DOMInjection(t *testing.T) {
	script := GenerateEnhancementsScript(DefaultConfig())

	for _, token := range []string{
		"extractExistingAntigravityProjects",
		"applyAutomationTargetProjectSelector",
		"if (!isAutomationTargetProjectEnabled()) {",
		`document.getElementById("swiss-automation-project-field")?.remove();`,
		"swiss-automation-project-field",
		"swiss-automation-project-select",
		"Target Project",
		"__new_dedicated__",
		"Create New Dedicated Project",
	} {
		if !strings.Contains(script, token) {
			t.Errorf("expected script to contain %q", token)
		}
	}

	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}

	driver := `
const assert = require("assert");

class MockElement {
  constructor(tagName, id = "", className = "") {
    this.tagName = tagName.toUpperCase();
    this.id = id;
    this.className = className;
    this.style = {};
    this.children = [];
    this.parentElement = null;
    this.nextSibling = null;
    this.attributes = {};
    this.value = "";
    this.textContent = "";
    this._listeners = {};
    this.memoizedProps = null;
  }
  setAttribute(k, v) { this.attributes[k] = v; }
  getAttribute(k) { return this.attributes[k] || null; }
  removeAttribute(k) { delete this.attributes[k]; }
  appendChild(c) {
    if (c.parentElement) c.parentElement.removeChild(c);
    c.parentElement = this;
    c.parentNode = this;
    if (this.children.length > 0) {
      this.children[this.children.length - 1].nextSibling = c;
    }
    c.nextSibling = null;
    this.children.push(c);
    return c;
  }
  removeChild(c) {
    const idx = this.children.indexOf(c);
    if (idx !== -1) {
      this.children.splice(idx, 1);
      c.parentElement = null;
      c.parentNode = null;
      for (let i = 0; i < this.children.length; i++) {
        this.children[i].nextSibling = (i + 1 < this.children.length) ? this.children[i + 1] : null;
      }
    }
  }
  insertBefore(newNode, refNode) {
    if (newNode.parentElement) newNode.parentElement.removeChild(newNode);
    newNode.parentElement = this;
    newNode.parentNode = this;
    const idx = refNode ? this.children.indexOf(refNode) : -1;
    if (idx === -1) {
      if (this.children.length > 0) {
        this.children[this.children.length - 1].nextSibling = newNode;
      }
      newNode.nextSibling = null;
      this.children.push(newNode);
    } else {
      this.children.splice(idx, 0, newNode);
      newNode.nextSibling = refNode;
      if (idx > 0) {
        this.children[idx - 1].nextSibling = newNode;
      }
    }
    return newNode;
  }
  addEventListener(event, fn) {
    if (!this._listeners[event]) this._listeners[event] = [];
    this._listeners[event].push(fn);
  }
  dispatchEvent(event) {
    const list = this._listeners[event.type] || [];
    list.forEach(fn => fn(event));
  }
  closest(selector) {
    let curr = this;
    while (curr) {
      if (curr.id && selector.includes("#" + curr.id)) return curr;
      if (selector.includes("[role=\"dialog\"]") && curr.getAttribute && curr.getAttribute("role") === "dialog") return curr;
      if (selector.includes(".flex.flex-col.gap-1") && curr.className && curr.className.includes("flex flex-col gap-1")) return curr;
      if (selector.includes(".tec") && curr.className && curr.className.includes("tec")) return curr;
      if (selector.includes("form") && curr.tagName === "FORM") return curr;
      curr = curr.parentElement;
    }
    return null;
  }
  querySelector(sel) {
    return this.querySelectorAll(sel)[0] || null;
  }
  querySelectorAll(sel) {
    const results = [];
    function scan(node) {
      for (const c of node.children) {
        let match = false;
        if (sel === "label" && c.tagName === "LABEL") match = true;
        if (sel.includes("label") && c.tagName === "LABEL") match = true;
        if (sel.includes(".text-sm") && c.className && c.className.includes("text-sm")) match = true;
        if (sel.includes("[data-testid=\"new-sidecar-name\"]") && c.getAttribute && c.getAttribute("data-testid") === "new-sidecar-name") match = true;
        if (sel.includes("[data-testid=\"swiss-automation-project-select\"]") && c.getAttribute && c.getAttribute("data-testid") === "swiss-automation-project-select") match = true;
        if (sel.includes("a[aria-label=\"New Conversation in Project\"]") && c.tagName === "A" && c.getAttribute && c.getAttribute("aria-label") === "New Conversation in Project") match = true;
        if (sel.includes("[data-swiss-project]") && c.getAttribute && c.getAttribute("data-swiss-project")) match = true;
        if (match) results.push(c);
        scan(c);
      }
    }
    scan(this);
    return results;
  }
}

const head = new MockElement("head");
const document = {
  head: head,
  documentElement: { classList: { contains: () => true } },
  body: new MockElement("body"),
  createElement: (tag) => new MockElement(tag),
  getElementById: (id) => {
    function find(node) {
      if (!node) return null;
      if (node.id === id) return node;
      for (const c of (node.children || [])) {
        const found = find(c);
        if (found) return found;
      }
      return null;
    }
    return find(document.head) || find(document.body);
  },
  querySelector: (sel) => document.body.querySelector(sel),
  querySelectorAll: (sel) => document.body.querySelectorAll(sel)
};

const storage = {};
const localStorage = {
  getItem: (k) => storage[k] || null,
  setItem: (k, v) => { storage[k] = v; }
};
const sessionStorage = {
  getItem: (k) => storage[k] || null,
  setItem: (k, v) => { storage[k] = v; }
};
class MutationObserver {
  constructor(cb) { this.cb = cb; }
  observe(target, options) {}
  disconnect() {}
}
function requestAnimationFrame(cb) { return setTimeout(cb, 0); }
function cancelAnimationFrame(id) { clearTimeout(id); }

// Create root with React Fiber
const root = new MockElement("div", "root");
document.body.appendChild(root);
root.__reactFiber$test = {
  memoizedProps: {
    value: {
      sidebarSectionsProvider: {
        sidebarSections: [
          { sectionKind: { case: "projectSection" }, uri: "proj-alpha", label: "Alpha Project" },
          { sectionKind: { case: "projectSection" }, uri: "proj-beta", label: "Beta Project" },
          { sectionKind: { case: "projectSection" }, uri: "outside-of-project", label: "Outside" }
        ]
      }
    }
  }
};

// Create modal with name field
const modal = new MockElement("div", "modal-dialog", "tec");
modal.setAttribute("role", "dialog");
const nameContainer = new MockElement("div", "", "flex flex-col gap-1");
const nameLabel = new MockElement("label", "", "text-sm font-medium");
nameLabel.textContent = "Name";
nameContainer.appendChild(nameLabel);
const nameInput = new MockElement("input");
nameInput.setAttribute("data-testid", "new-sidecar-name");
nameContainer.appendChild(nameInput);
modal.appendChild(nameContainer);

const otherContainer = new MockElement("div", "", "flex flex-col gap-1");
modal.appendChild(otherContainer);
document.body.appendChild(modal);

const window = {
  document,
  localStorage,
  sessionStorage,
  getComputedStyle: () => ({ backgroundColor: "rgb(30, 30, 30)" }),
  location: { pathname: "/c/11111111-2222-3333-4444-555555555555", search: "" },
  addEventListener: () => {},
  removeEventListener: () => {}
};

` + script + `

// Test 1: extractExistingAntigravityProjects
const projects = window.extractExistingAntigravityProjects();
assert.strictEqual(projects.length, 2, "must extract 2 projects ignoring outside-of-project");
assert.strictEqual(projects[0].id, "proj-alpha");
assert.strictEqual(projects[0].label, "Alpha Project");
assert.strictEqual(projects[1].id, "proj-beta");
assert.strictEqual(projects[1].label, "Beta Project");

// Test 2: applyAutomationTargetProjectSelector
window.applyAutomationTargetProjectSelector();

const field = document.getElementById("swiss-automation-project-field");
assert.ok(field, "#swiss-automation-project-field must be injected");
assert.strictEqual(field.parentElement, modal, "field must be inside modal");
assert.strictEqual(nameContainer.nextSibling, field, "field must be inserted right after nameContainer");

const label = field.children[0];
assert.strictEqual(label.textContent, "Target Project");

const select = document.getElementById("swiss-automation-project-select");
assert.ok(select, "#swiss-automation-project-select must exist");
assert.strictEqual(select.children.length, 3, "must have 2 projects + __new_dedicated__ option");
assert.strictEqual(select.children[0].value, "proj-alpha");
assert.strictEqual(select.children[0].textContent, "Alpha Project");
assert.strictEqual(select.children[1].value, "proj-beta");
assert.strictEqual(select.children[2].value, "__new_dedicated__");
assert.strictEqual(select.children[2].textContent, "Create New Dedicated Project");

// Verify default selection and window variable
assert.strictEqual(window.__swissSelectedAutomationProjectId, "proj-alpha");

// Test 3: Change event
select.value = "proj-beta";
select.dispatchEvent({ type: "change", target: select });
assert.strictEqual(window.__swissSelectedAutomationProjectId, "proj-beta");

select.value = "__new_dedicated__";
select.dispatchEvent({ type: "change", target: select });
assert.strictEqual(window.__swissSelectedAutomationProjectId, "", "dedicated project option must reset selected project ID to empty string");

// Test 4: Idempotency (calling again must not inject duplicate)
window.applyAutomationTargetProjectSelector();
assert.strictEqual(modal.children.filter(c => c.id === "swiss-automation-project-field").length, 1);

// Test 5: Native project selector suppression
field.parentElement.removeChild(field);
const nativeLabel = new MockElement("label", "", "text-sm font-medium");
nativeLabel.textContent = "Project";
modal.insertBefore(nativeLabel, nameContainer);
window.applyAutomationTargetProjectSelector();
assert.strictEqual(document.getElementById("swiss-automation-project-field"), null, "must not inject if native Project label already present");

console.log("dom injection OK");
process.exit(0);
`

	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(driver)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node execution failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "dom injection OK") {
		t.Fatalf("unexpected output: %s", string(out))
	}
}

func TestAutomationTargetProjectSelector_DisabledRemovesInjectedField(t *testing.T) {
	cfg := DefaultConfig()
	disabled := false
	cfg.AutomationTargetProjectEnabled = &disabled
	script := GenerateEnhancementsScript(cfg)

	startMarker := "function applyAutomationTargetProjectSelector() {"
	endMarker := "\n    function isMainStageActive() {"
	startIdx := strings.Index(script, startMarker)
	endIdx := strings.Index(script[startIdx:], endMarker)
	if startIdx == -1 || endIdx == -1 {
		t.Fatal("expected applyAutomationTargetProjectSelector to be extractable")
	}
	selectorFn := script[startIdx : startIdx+endIdx]

	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}

	driver := `
const assert = require("assert");
let removed = false;
let queried = false;
const field = { remove: () => { removed = true; } };
const document = {
  getElementById: (id) => id === "swiss-automation-project-field" ? field : null,
  querySelector: () => { queried = true; return null; }
};
const enhConfig = { automation_target_project_enabled: false };
function isAutomationTargetProjectEnabled() {
  return enhConfig.automation_target_project_enabled !== false;
}

` + selectorFn + `

applyAutomationTargetProjectSelector();
assert.strictEqual(removed, true, "disabled target-project mode must remove the injected field");
assert.strictEqual(queried, false, "disabled target-project mode must return before inspecting the modal");
console.log("disabled selector OK");
`

	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(driver)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node execution failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "disabled selector OK") {
		t.Fatalf("unexpected output: %s", string(out))
	}
}

func TestAutomationTargetProjectSelector_ManageSidecarInterceptor(t *testing.T) {
	script := GenerateEnhancementsScript(DefaultConfig())

	for _, token := range []string{
		"/exa.language_server_pb.LanguageServerService/ManageSidecar",
		"action === 5",
		"if (!isAutomationTargetProjectEnabled()) {",
		"return origFetch(input, init);",
		"body.projectId = chosenProj",
		"action: 6",
		"sidecarId:",
		"projectId: chosenProj",
	} {
		if !strings.Contains(script, token) {
			t.Errorf("expected script to contain %q", token)
		}
	}

	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}

	driver := `
const assert = require("assert");

const fetchCalls = [];
async function mockFetch(input, init) {
  fetchCalls.push({ input, init });
  return {
    ok: true,
    status: 200,
    clone() {
      return {
        async json() {
          return { sidecarId: "sidecar-xyz-123" };
        }
      };
    },
    async json() {
      return { sidecarId: "sidecar-xyz-123" };
    }
  };
}

const window = {
  fetch: mockFetch,
  __swissSelectedAutomationProjectId: "proj-alpha",
  addEventListener: () => {}
};

const storage2 = {};
const localStorage = {
  getItem: (k) => storage2[k] || null,
  setItem: (k, v) => { storage2[k] = v; }
};
const sessionStorage = {
  getItem: (k) => storage2[k] || null,
  setItem: (k, v) => { storage2[k] = v; }
};
class MockElement2 {
  constructor(tagName, id = "", className = "") {
    this.tagName = tagName.toUpperCase();
    this.id = id;
    this.className = className;
    this.style = {};
    this.children = [];
    this.parentElement = null;
    this.attributes = {};
    this.value = "";
    this.textContent = "";
  }
  setAttribute(k, v) { this.attributes[k] = v; if (k === "id") this.id = v; }
  getAttribute(k) { return this.attributes[k] || null; }
  appendChild(c) { c.parentElement = this; this.children.push(c); return c; }
  querySelector() { return null; }
  querySelectorAll() { return []; }
}
class MutationObserver {
  constructor(cb) { this.cb = cb; }
  observe(target, options) {}
  disconnect() {}
}
function requestAnimationFrame(cb) { return setTimeout(cb, 0); }
function cancelAnimationFrame(id) { clearTimeout(id); }
window.localStorage = localStorage;
window.sessionStorage = sessionStorage;
window.location = { pathname: "/c/11111111-2222-3333-4444-555555555555", search: "" };
window.getComputedStyle = () => ({ backgroundColor: "rgb(30, 30, 30)" });

const head2 = new MockElement2("head");
const document = {
  head: head2,
  documentElement: { classList: { contains: () => true } },
  body: new MockElement2("body"),
  createElement: (tag) => new MockElement2(tag),
  getElementById: (id) => {
    if (id === "swiss-automation-project-select") {
      return { value: window.__swissSelectedAutomationProjectId || "proj-alpha" };
    }
    if (id === "swiss-enhancements-dynamic-styles") return head2;
    return null;
  },
  querySelector: () => null,
  querySelectorAll: () => []
};
window.document = document;

` + script + `

(async () => {
  // Test Case 1: action: 5 with no projectId
  fetchCalls.length = 0;
  window.__swissSelectedAutomationProjectId = "proj-alpha";

  await window.fetch("/exa.language_server_pb.LanguageServerService/ManageSidecar", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ action: 5, name: "Backup Task" })
  });

  assert.strictEqual(fetchCalls.length, 2, "must have made create call + action 6 follow-up call");

  // Verify first call has projectId injected
  const firstReq = JSON.parse(fetchCalls[0].init.body);
  assert.strictEqual(firstReq.action, 5);
  assert.strictEqual(firstReq.name, "Backup Task");
  assert.strictEqual(firstReq.projectId, "proj-alpha", "action 5 request body must have projectId injected");

  // Verify follow-up call
  assert.strictEqual(fetchCalls[1].input, "/exa.language_server_pb.LanguageServerService/ManageSidecar");
  const followUpReq = JSON.parse(fetchCalls[1].init.body);
  assert.strictEqual(followUpReq.sidecarId, "sidecar-xyz-123");
  assert.strictEqual(followUpReq.action, 6);
  assert.strictEqual(followUpReq.projectId, "proj-alpha");

  // Test Case 2: action: 5 with existing projectId (must preserve existing)
  fetchCalls.length = 0;
  await window.fetch("/exa.language_server_pb.LanguageServerService/ManageSidecar", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ action: 5, name: "Custom", projectId: "pre-existing-proj" })
  });
  const existingReq = JSON.parse(fetchCalls[0].init.body);
  assert.strictEqual(existingReq.projectId, "pre-existing-proj", "must preserve existing projectId");

  // Test Case 3: Dedicated project selected (__new_dedicated__)
  fetchCalls.length = 0;
  window.__swissSelectedAutomationProjectId = "__new_dedicated__";
  document.getElementById = () => ({ value: "__new_dedicated__" });

  await window.fetch("/exa.language_server_pb.LanguageServerService/ManageSidecar", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ action: 5, name: "Dedicated Task" })
  });

  assert.strictEqual(fetchCalls.length, 1, "must not execute follow-up call for __new_dedicated__");
  const dedicatedReq = JSON.parse(fetchCalls[0].init.body);
  assert.strictEqual(dedicatedReq.projectId, undefined, "must not inject projectId when dedicated project is chosen");

  // Test Case 4: Other endpoints must not be touched
  fetchCalls.length = 0;
  await window.fetch("/api/some/other/endpoint", {
    method: "POST",
    body: JSON.stringify({ action: 5 })
  });
  assert.strictEqual(fetchCalls.length, 1);
  assert.strictEqual(fetchCalls[0].input, "/api/some/other/endpoint");

  console.log("interceptor OK");
  process.exit(0);
})();
`

	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(driver)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node execution failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "interceptor OK") {
		t.Fatalf("unexpected output: %s", string(out))
	}
}

func TestAutomationTargetProjectSelector_DisabledInterceptorPreservesRequest(t *testing.T) {
	cfg := DefaultConfig()
	disabled := false
	cfg.AutomationTargetProjectEnabled = &disabled
	script := GenerateEnhancementsScript(cfg)

	startMarker := "function installManageSidecarInterceptor() {"
	endMarker := "function extractExistingAntigravityProjects() {"
	startIdx := strings.Index(script, startMarker)
	endIdx := strings.Index(script, endMarker)
	if startIdx == -1 || endIdx <= startIdx {
		t.Fatal("expected installManageSidecarInterceptor to be extractable")
	}
	interceptorFn := script[startIdx:endIdx]

	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}

	driver := `
const assert = require("assert");
const calls = [];
const originalBody = JSON.stringify({ action: 5, name: "Native Task" });
const window = {
  fetch: async (input, init) => {
    calls.push({ input, init });
    return { ok: true, status: 200 };
  }
};
const document = {
  getElementById: () => ({ value: "proj-alpha" })
};
const enhConfig = { automation_target_project_enabled: false };
function isAutomationTargetProjectEnabled() {
  return enhConfig.automation_target_project_enabled !== false;
}

` + interceptorFn + `

(async () => {
  installManageSidecarInterceptor();
  await window.fetch("/exa.language_server_pb.LanguageServerService/ManageSidecar", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: originalBody
  });
  assert.strictEqual(calls.length, 1, "disabled target-project mode must not send an action 6 follow-up");
  assert.strictEqual(calls[0].init.body, originalBody, "disabled target-project mode must preserve the native request body");
  assert.strictEqual(JSON.parse(calls[0].init.body).projectId, undefined);
  console.log("disabled interceptor OK");
})();
`

	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(driver)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node execution failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "disabled interceptor OK") {
		t.Fatalf("unexpected output: %s", string(out))
	}
}

func TestContextWindowProgressRing_CompactSlashCommandFlagOverride(t *testing.T) {
	script := GenerateEnhancementsScript(DefaultConfig())

	if !strings.Contains(script, "enable-compact-slash-command") {
		t.Errorf("expected script to contain enable-compact-slash-command flag override")
	}
	if !strings.Contains(script, `parsedFlags["enable-compact-slash-command"] = true;`) {
		t.Errorf("expected script to set enable-compact-slash-command to true")
	}

	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}

	driver := `
const assert = require("assert");

const storage = {
  "jetski.developer.customFlagOverrides": JSON.stringify({
    "remote-control-setting-enabled": true,
    "jetski-sidecar-declared-permissions": false
  })
};

const localStorage = {
  getItem: (k) => storage[k] || null,
  setItem: (k, v) => { storage[k] = v; }
};

let dispatched = null;
const window = {
  localStorage,
  dispatchEvent: (e) => { dispatched = e; }
};
function StorageEvent(type, dict) { Object.assign(this, dict); }

// Simulate the flag override logic from the script
const rawFlags = localStorage.getItem("jetski.developer.customFlagOverrides") || "{}";
const parsedFlags = JSON.parse(rawFlags);
	const targetProjectFlag = !true;
	if (parsedFlags && typeof parsedFlags === "object" && !Array.isArray(parsedFlags) && (parsedFlags["remote-control-setting-enabled"] !== true || parsedFlags["jetski-sidecar-declared-permissions"] !== targetProjectFlag || parsedFlags["enable-compact-slash-command"] !== true)) {
  parsedFlags["remote-control-setting-enabled"] = true;
  parsedFlags["jetski-sidecar-declared-permissions"] = targetProjectFlag;
  parsedFlags["enable-compact-slash-command"] = true;
  const nextFlags = JSON.stringify(parsedFlags);
  localStorage.setItem("jetski.developer.customFlagOverrides", nextFlags);
}

const saved = JSON.parse(localStorage.getItem("jetski.developer.customFlagOverrides"));
assert.strictEqual(saved["remote-control-setting-enabled"], true);
assert.strictEqual(saved["jetski-sidecar-declared-permissions"], false);
assert.strictEqual(saved["enable-compact-slash-command"], true);

console.log("compact flag override OK");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(driver)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node execution failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "compact flag override OK") {
		t.Fatalf("unexpected output: %s", string(out))
	}
}

func TestContextWindowProgressRing_ClickToCompact(t *testing.T) {
	script := GenerateEnhancementsScript(DefaultConfig())

	for _, token := range []string{
		"swiss-context-ring-spin",
		`animation: swiss-context-ring-spin 0.8s linear infinite !important;`,
		"data-swiss-compacting",
		"invokeNativeCompactCommand",
		"contextScopeItemMention",
		"/compact",
		"handleSubmit(editor, 1)",
		"NEXT_INVOCATION = 1",
		"Compacting context window...",
		"— Click to compact",
	} {
		if !strings.Contains(script, token) {
			t.Errorf("expected script to contain %q", token)
		}
	}

	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}

	driver := `
const assert = require("assert");

class MockElement {
  constructor(tagName, id = "", className = "") {
    this.tagName = tagName.toUpperCase();
    this.id = id;
    this.className = className;
    this.classList = {
      _set: new Set(className ? className.split(" ") : []),
      add: (c) => this.classList._set.add(c),
      remove: (c) => this.classList._set.delete(c),
      has: (c) => this.classList._set.has(c),
      contains: (c) => this.classList._set.has(c),
    };
    this.style = {};
    this.children = [];
    this.parentElement = null;
    this.parentNode = null;
    this.nextSibling = null;
    this.previousElementSibling = null;
    this.attributes = {};
    this.value = "";
    this.innerText = "";
    this.textContent = "";
    this.innerHTML = "";
    this._listeners = {};
    this.memoizedProps = null;
  }
  setAttribute(k, v) {
    this.attributes[k] = v;
    if (k === "id") this.id = v;
  }
  getAttribute(k) { return this.attributes[k] !== undefined ? this.attributes[k] : null; }
  removeAttribute(k) { delete this.attributes[k]; }
  appendChild(c) {
    if (c.parentElement) c.parentElement.removeChild(c);
    c.parentElement = this;
    c.parentNode = this;
    if (this.children.length > 0) {
      const last = this.children[this.children.length - 1];
      last.nextSibling = c;
      c.previousElementSibling = last;
    }
    c.nextSibling = null;
    this.children.push(c);
    return c;
  }
  removeChild(c) {
    const idx = this.children.indexOf(c);
    if (idx !== -1) {
      this.children.splice(idx, 1);
      c.parentElement = null;
      c.parentNode = null;
      for (let i = 0; i < this.children.length; i++) {
        this.children[i].nextSibling = (i + 1 < this.children.length) ? this.children[i + 1] : null;
        this.children[i].previousElementSibling = (i > 0) ? this.children[i - 1] : null;
      }
    }
  }
  remove() {
    if (this.parentElement) {
      this.parentElement.removeChild(this);
    }
  }
  insertBefore(newNode, refNode) {
    if (newNode.parentElement) newNode.parentElement.removeChild(newNode);
    newNode.parentElement = this;
    newNode.parentNode = this;
    const idx = refNode ? this.children.indexOf(refNode) : -1;
    if (idx === -1) {
      if (this.children.length > 0) {
        const last = this.children[this.children.length - 1];
        last.nextSibling = newNode;
        newNode.previousElementSibling = last;
      }
      newNode.nextSibling = null;
      this.children.push(newNode);
    } else {
      this.children.splice(idx, 0, newNode);
      newNode.nextSibling = refNode;
      newNode.previousElementSibling = (idx > 0) ? this.children[idx - 1] : null;
      refNode.previousElementSibling = newNode;
      if (idx > 0) {
        this.children[idx - 1].nextSibling = newNode;
      }
    }
    return newNode;
  }
  addEventListener(event, fn) {
    if (!this._listeners[event]) this._listeners[event] = [];
    this._listeners[event].push(fn);
  }
  dispatchEvent(event) {
    const list = this._listeners[event.type] || [];
    list.forEach(fn => fn(event));
  }
  click() {
    if (typeof this.onclick === "function") this.onclick();
  }
  closest(selector) {
    let curr = this;
    while (curr) {
      if (curr.id && selector.includes("#" + curr.id)) return curr;
      if (selector.includes("agent-input-box") && curr.getAttribute && curr.getAttribute("data-testid") === "agent-input-box") return curr;
      if (selector.includes(".no-focus-agent-input") && curr.className && curr.className.includes("no-focus-agent-input")) return curr;
      curr = curr.parentElement;
    }
    return null;
  }
  querySelector(sel) {
    return this.querySelectorAll(sel)[0] || null;
  }
  querySelectorAll(sel) {
    const results = [];
    function scan(node) {
      for (const c of node.children) {
        let match = false;
        if (sel.includes("model-selector-trigger") && c.getAttribute && c.getAttribute("data-testid") === "model-selector-trigger") match = true;
        if (sel.includes("agent-input-box") && c.getAttribute && c.getAttribute("data-testid") === "agent-input-box") match = true;
        if (sel.includes("agent-loading") && c.getAttribute && c.getAttribute("data-testid") === "agent-loading") match = true;
        if (sel.includes("stop-button") && c.getAttribute && c.getAttribute("data-testid") === "stop-button") match = true;
        if (sel.includes("button[aria-label*=\"Select model\"]") && c.tagName === "BUTTON" && c.getAttribute && (c.getAttribute("aria-label") || "").includes("Select model")) match = true;
        if (sel.includes("swiss-context-window-ring-sub")) {
          if ((c.id && c.id.startsWith("swiss-context-window-ring-sub")) || (c.classList && c.classList.has("swiss-context-window-ring-sub"))) match = true;
        } else if (sel.includes("swiss-context-window-ring")) {
          if (c.id === "swiss-context-window-ring" || (c.classList && c.classList.has("swiss-context-window-ring"))) match = true;
        }
        if (match) results.push(c);
        scan(c);
      }
    }
    scan(this);
    return results;
  }
}

const head = new MockElement("head");
const body = new MockElement("body");
const document = {
  head: head,
  body: body,
  documentElement: { classList: { contains: () => true } },
  createElement: (tag) => new MockElement(tag),
  getElementById: (id) => {
    function find(node) {
      if (!node) return null;
      if (node.id === id) return node;
      for (const c of (node.children || [])) {
        const found = find(c);
        if (found) return found;
      }
      return null;
    }
    return find(document.head) || find(document.body);
  },
  querySelector: (sel) => document.body.querySelector(sel),
  querySelectorAll: (sel) => document.body.querySelectorAll(sel)
};

const storage = {};
const localStorage = {
  getItem: (k) => storage[k] || null,
  setItem: (k, v) => { storage[k] = v; }
};
const sessionStorage = {
  getItem: () => null,
  setItem: () => {}
};
function StorageEvent(type, dict) { Object.assign(this, dict); }
class MutationObserver {
  constructor(cb) { this.cb = cb; }
  observe(target, options) {}
  disconnect() {}
}
function requestAnimationFrame(cb) { return setTimeout(cb, 0); }
function cancelAnimationFrame(id) { clearTimeout(id); }
let loadedEnhancements = { enabled: true };
const window = {
  document: document,
  location: { pathname: "/c/33333333-4444-5555-6666-777777777777" },
  localStorage: localStorage,
  sessionStorage: sessionStorage,
  addEventListener: () => {},
  dispatchEvent: () => {},
  __swissDaemonOnline: true,
  fetch: () => Promise.resolve({ ok: true, json: () => Promise.resolve(loadedEnhancements) })
};
globalThis.fetch = window.fetch;

// Create the Antigravity Chat UI tree:
// agent-input-box -> container -> model-selector-trigger
const inputBox = new MockElement("div", "agent-input-box-1", "agent-input-box");
inputBox.setAttribute("data-testid", "agent-input-box");

const trigger = new MockElement("div", "model-trigger", "model-selector-trigger");
trigger.setAttribute("data-testid", "model-selector-trigger");
trigger.innerText = "Gemini Pro";
inputBox.appendChild(trigger);
document.body.appendChild(inputBox);

// Mock Lexical Editor and React Fiber props
const editor = {
  state: null,
  parseEditorState: (str) => JSON.parse(str),
  setEditorState: (s) => { editor.state = s; }
};
const handleSubmitCalls = [];
const mockFiberProps = {
  lexicalRef: { current: editor },
  slashCommandItems: [
    { name: "compact", type: 1 }
  ],
  handleSubmit: (ed, strategy) => {
    handleSubmitCalls.push({ editor: ed, strategy: strategy });
    return Promise.resolve();
  }
};
inputBox.__reactFiber$test = {
  memoizedProps: mockFiberProps
};

` + script + `

(async () => {
  const ring = document.getElementById("swiss-context-window-ring");
  assert(ring !== null, "progress ring must be rendered");
  assert.strictEqual(ring.getAttribute("role"), "button", "ring role must be button");
  assert(ring.getAttribute("title").includes("— Click to compact"), "idle title must include '— Click to compact'");

  // 1. Click ring to trigger compaction
  ring.click();

  // Immediately active
  assert.strictEqual(ring.getAttribute("data-swiss-compacting"), "true", "must set data-swiss-compacting to true immediately");
  assert.strictEqual(ring.getAttribute("title"), "Compacting context window...", "title must be Compacting context window...");
  assert.strictEqual(handleSubmitCalls.length, 1, "must call handleSubmit once");
  assert.strictEqual(handleSubmitCalls[0].strategy, 1, "must call handleSubmit with MessageDeliveryStrategy.NEXT_INVOCATION = 1");
  assert.strictEqual(handleSubmitCalls[0].editor, editor, "must pass lexical editor to handleSubmit");

  // Verify Lexical JSON structure has contextScopeItemMention with /compact
  assert(editor.state !== null, "editor state must be set");
  const firstChild = editor.state.root.children[0].children[0];
  assert.strictEqual(firstChild.type, "contextScopeItemMention");
  assert.strictEqual(firstChild.text, "/compact");
  assert.strictEqual(firstChild.detail.name, "compact");

  // 2. Before 850ms cycle completes (e.g. at 400ms), ring must still be rotating
  await new Promise(r => setTimeout(r, 400));
  assert.strictEqual(ring.getAttribute("data-swiss-compacting"), "true", "ring must still be rotating before 850ms elapsed");

  // 3. After full cycle completes (at 950ms total), data-swiss-compacting must be cleared
  await new Promise(r => setTimeout(r, 550));
  assert.strictEqual(ring.getAttribute("data-swiss-compacting"), null, "data-swiss-compacting must be cleared after >=850ms cycle finishes");

  // 4. Test active compaction turn in UI: ring must remain spinning until UI turn completes
  const loadingEl = new MockElement("div", "agent-loading-1");
  loadingEl.setAttribute("data-testid", "agent-loading");
  loadingEl.innerText = "Compacting conversation...";
  document.body.appendChild(loadingEl);

  ring.click();
  assert.strictEqual(ring.getAttribute("data-swiss-compacting"), "true", "must start compacting again on click");
  assert.strictEqual(handleSubmitCalls.length, 2, "must call handleSubmit a second time");

  // Wait 950ms (>= 850ms) - should still be spinning because loadingEl is present!
  await new Promise(r => setTimeout(r, 950));
  assert.strictEqual(ring.getAttribute("data-swiss-compacting"), "true", "ring must keep rotating while [data-testid=agent-loading] with Compacting is present");

  // Remove loadingEl and wait for next poll (150ms + margin = 250ms)
  loadingEl.remove();
  await new Promise(r => setTimeout(r, 250));
  assert.strictEqual(ring.getAttribute("data-swiss-compacting"), null, "ring must clear data-swiss-compacting once loading UI disappears");

  // 5. Disabling the indicator from refreshed settings removes the ring without reloading.
  loadedEnhancements = {
    enabled: true,
    context_window_indicator_enabled: false,
    context_window_click_to_compact_enabled: true
  };
  window.__swissOnEnhancementsDaemonChanged(true);
  await new Promise(r => setTimeout(r, 20));
  assert.strictEqual(document.getElementById("swiss-context-window-ring"), null, "disabling the indicator must remove the ring");

  // 6. Re-enabling the indicator restores it as a non-interactive usage display.
  loadedEnhancements = {
    enabled: true,
    context_window_indicator_enabled: true,
    context_window_click_to_compact_enabled: false
  };
  window.__swissOnEnhancementsDaemonChanged(true);
  await new Promise(r => setTimeout(r, 20));
  const indicatorOnlyRing = document.getElementById("swiss-context-window-ring");
  assert(indicatorOnlyRing !== null, "re-enabling the indicator must restore the ring");
  assert.strictEqual(indicatorOnlyRing.getAttribute("role"), "img", "click-disabled ring must not expose button role");
  assert.strictEqual(indicatorOnlyRing.getAttribute("tabindex"), "-1", "click-disabled ring must not be keyboard-interactive");
  assert(!indicatorOnlyRing.getAttribute("title").includes("Click to compact"), "click-disabled ring must not advertise compaction");
  const submitCountBeforeDisabledClick = handleSubmitCalls.length;
  indicatorOnlyRing.click();
  await new Promise(r => setTimeout(r, 20));
  assert.strictEqual(handleSubmitCalls.length, submitCountBeforeDisabledClick, "click-disabled ring must not dispatch compact");

  console.log("click to compact OK");
  process.exit(0);
})();
`

	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(driver)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node execution failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "click to compact OK") {
		t.Fatalf("unexpected output: %s", string(out))
	}
}

func TestContextWindowProgressRing_ClickToCompact_DOMFallback(t *testing.T) {
	script := GenerateEnhancementsScript(DefaultConfig())

	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}

	driver := `
const assert = require("assert");

class MockElement {
  constructor(tagName, id = "", className = "") {
    this.tagName = tagName.toUpperCase();
    this.id = id;
    this.className = className;
    this.classList = {
      _set: new Set(className ? className.split(" ") : []),
      add: (c) => this.classList._set.add(c),
      remove: (c) => this.classList._set.delete(c),
      has: (c) => this.classList._set.has(c),
      contains: (c) => this.classList._set.has(c),
    };
    this.style = {};
    this.children = [];
    this.parentElement = null;
    this.parentNode = null;
    this.nextSibling = null;
    this.previousElementSibling = null;
    this.attributes = {};
    this.value = "";
    this.innerText = "";
    this.textContent = "";
    this.innerHTML = "";
    this._listeners = {};
    this.clicks = 0;
  }
  setAttribute(k, v) {
    this.attributes[k] = v;
    if (k === "id") this.id = v;
  }
  getAttribute(k) { return this.attributes[k] !== undefined ? this.attributes[k] : null; }
  removeAttribute(k) { delete this.attributes[k]; }
  focus() {}
  appendChild(c) {
    if (c.parentElement) c.parentElement.removeChild(c);
    c.parentElement = this;
    c.parentNode = this;
    if (this.children.length > 0) {
      const last = this.children[this.children.length - 1];
      last.nextSibling = c;
      c.previousElementSibling = last;
    }
    c.nextSibling = null;
    this.children.push(c);
    return c;
  }
  removeChild(c) {
    const idx = this.children.indexOf(c);
    if (idx !== -1) {
      this.children.splice(idx, 1);
      c.parentElement = null;
      c.parentNode = null;
      for (let i = 0; i < this.children.length; i++) {
        this.children[i].nextSibling = (i + 1 < this.children.length) ? this.children[i + 1] : null;
        this.children[i].previousElementSibling = (i > 0) ? this.children[i - 1] : null;
      }
    }
  }
  remove() {
    if (this.parentElement) {
      this.parentElement.removeChild(this);
    }
  }
  insertBefore(newNode, refNode) {
    if (newNode.parentElement) newNode.parentElement.removeChild(newNode);
    newNode.parentElement = this;
    newNode.parentNode = this;
    const idx = refNode ? this.children.indexOf(refNode) : -1;
    if (idx === -1) {
      if (this.children.length > 0) {
        const last = this.children[this.children.length - 1];
        last.nextSibling = newNode;
        newNode.previousElementSibling = last;
      }
      newNode.nextSibling = null;
      this.children.push(newNode);
    } else {
      this.children.splice(idx, 0, newNode);
      newNode.nextSibling = refNode;
      newNode.previousElementSibling = (idx > 0) ? this.children[idx - 1] : null;
      refNode.previousElementSibling = newNode;
      if (idx > 0) {
        this.children[idx - 1].nextSibling = newNode;
      }
    }
    return newNode;
  }
  addEventListener(event, fn) {
    if (!this._listeners[event]) this._listeners[event] = [];
    this._listeners[event].push(fn);
  }
  dispatchEvent(event) {
    const list = this._listeners[event.type] || [];
    list.forEach(fn => fn(event));
  }
  click() {
    this.clicks++;
    if (typeof this.onclick === "function") this.onclick();
  }
  closest(selector) {
    let curr = this;
    while (curr) {
      if (curr.id && selector.includes("#" + curr.id)) return curr;
      if (selector.includes("agent-input-box") && curr.getAttribute && curr.getAttribute("data-testid") === "agent-input-box") return curr;
      if (selector.includes(".no-focus-agent-input") && curr.className && curr.className.includes("no-focus-agent-input")) return curr;
      curr = curr.parentElement;
    }
    return null;
  }
  querySelector(sel) {
    return this.querySelectorAll(sel)[0] || null;
  }
  querySelectorAll(sel) {
    const results = [];
    function scan(node) {
      for (const c of node.children) {
        let match = false;
        if (sel.includes("model-selector-trigger") && c.getAttribute && c.getAttribute("data-testid") === "model-selector-trigger") match = true;
        if (sel.includes("agent-input-box") && c.getAttribute && c.getAttribute("data-testid") === "agent-input-box") match = true;
        if (sel.includes("contenteditable") && c.getAttribute && c.getAttribute("contenteditable") === "true") match = true;
        if (sel.includes("send-button") && c.getAttribute && c.getAttribute("data-testid") === "send-button") match = true;
        if (sel.includes("swiss-context-window-ring-sub")) {
          if ((c.id && c.id.startsWith("swiss-context-window-ring-sub")) || (c.classList && c.classList.has("swiss-context-window-ring-sub"))) match = true;
        } else if (sel.includes("swiss-context-window-ring")) {
          if (c.id === "swiss-context-window-ring" || (c.classList && c.classList.has("swiss-context-window-ring"))) match = true;
        }
        if (match) results.push(c);
        scan(c);
      }
    }
    scan(this);
    return results;
  }
}

const head = new MockElement("head");
const body = new MockElement("body");
const document = {
  head: head,
  body: body,
  documentElement: { classList: { contains: () => true } },
  createElement: (tag) => new MockElement(tag),
  getElementById: (id) => {
    function find(node) {
      if (!node) return null;
      if (node.id === id) return node;
      for (const c of (node.children || [])) {
        const found = find(c);
        if (found) return found;
      }
      return null;
    }
    return find(document.head) || find(document.body);
  },
  querySelector: (sel) => document.body.querySelector(sel),
  querySelectorAll: (sel) => document.body.querySelectorAll(sel)
};

const storage = {};
const localStorage = {
  getItem: (k) => storage[k] || null,
  setItem: (k, v) => { storage[k] = v; }
};
const sessionStorage = {
  getItem: () => null,
  setItem: () => {}
};
function StorageEvent(type, dict) { Object.assign(this, dict); }
class MutationObserver {
  constructor(cb) { this.cb = cb; }
  observe() {}
  disconnect() {}
}
function requestAnimationFrame(cb) { return setTimeout(cb, 0); }
function cancelAnimationFrame(id) { clearTimeout(id); }
function InputEvent(type, dict) { Object.assign(this, dict); this.type = type; }
function Event(type, dict) { Object.assign(this, dict); this.type = type; }
function KeyboardEvent(type, dict) { Object.assign(this, dict); this.type = type; }

const window = {
  document: document,
  location: { pathname: "/c/55555555-6666-7777-8888-999999999999" },
  localStorage: localStorage,
  sessionStorage: sessionStorage,
  addEventListener: () => {},
  dispatchEvent: () => {},
  __swissDaemonOnline: true,
  fetch: () => Promise.resolve({ ok: true, json: () => Promise.resolve({ summary: { context_tokens: 12000 } }) })
};

const inputBox = new MockElement("div", "agent-input-box-dom", "agent-input-box");
inputBox.setAttribute("data-testid", "agent-input-box");

const trigger = new MockElement("div", "model-trigger-dom", "model-selector-trigger");
trigger.setAttribute("data-testid", "model-selector-trigger");
trigger.innerText = "Gemini Flash";
inputBox.appendChild(trigger);

const editable = new MockElement("div", "editable-div");
editable.setAttribute("contenteditable", "true");
inputBox.appendChild(editable);

const sendBtn = new MockElement("button", "send-btn");
sendBtn.setAttribute("data-testid", "send-button");
inputBox.appendChild(sendBtn);

document.body.appendChild(inputBox);

` + script + `

(async () => {
  const ring = document.getElementById("swiss-context-window-ring");
  assert(ring !== null, "progress ring must be rendered in DOM fallback mode");

  // Click ring
  ring.click();

  assert.strictEqual(ring.getAttribute("data-swiss-compacting"), "true", "must set data-swiss-compacting");
  assert.strictEqual(editable.innerText, "/compact", "must inject /compact into contenteditable element");
  assert.strictEqual(sendBtn.clicks, 1, "must click send button");

  // Wait full cycle (>850ms)
  await new Promise(r => setTimeout(r, 950));
  assert.strictEqual(ring.getAttribute("data-swiss-compacting"), null, "data-swiss-compacting must be cleared after completion");

  console.log("dom fallback compact OK");
  process.exit(0);
})();
`

	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(driver)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node execution failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "dom fallback compact OK") {
		t.Fatalf("unexpected output: %s", string(out))
	}
}
