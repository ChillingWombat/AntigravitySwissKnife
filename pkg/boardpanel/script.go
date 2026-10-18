package boardpanel

// AuxPanelResolverJS is the shared panel-resolution helper embedded into every
// injected script. All three scripts (probe / collapse / restore) MUST resolve
// the Overview panel identically, otherwise the watcher can "hide" one element
// while the probe keeps reporting another one.
//
// Selector order is deliberate: the auxiliary-panel CONTAINER selectors that the
// rest of the Swiss Knife code base already relies on
// (pkg/plugins/auxiliary.go, pkg/plugins/github_extension_script.go, pkg/gui)
// come first. The broad [data-testid*="overview" i] match is only a last resort
// because it can also match nodes INSIDE the panel (tab buttons, section
// wrappers): hiding such a node leaves the panel visibly open, which is exactly
// the "Overview panel is still not auto-collapsed" failure mode.
const AuxPanelResolverJS = `
    const __swissResolveAuxPanel = () => {
      const containerSelectors = [
        '[data-testid="auxiliary-panel"]',
        '[data-swiss-aux-panel]',
        '.part.auxiliarybar'
      ];
      for (const sel of containerSelectors) {
        const el = document.querySelector(sel);
        if (el) return { el: el, source: sel };
      }
      const overviewSelectors = [
        '[data-testid="overview-panel"]',
        '[data-testid*="overview" i]'
      ];
      for (const sel of overviewSelectors) {
        const el = document.querySelector(sel);
        if (el) return { el: el, source: sel };
      }
      return { el: null, source: 'none' };
    };
    const __swissPanelAttributeOpen = (panel) => {
      const flagEl = document.querySelector('[data-aux-pane-open]');
      if (!flagEl) return null;
      const belongsToPanel = flagEl === panel ||
        (panel && typeof panel.contains === "function" && panel.contains(flagEl)) ||
        (typeof flagEl.contains === "function" && flagEl.contains(panel));
      if (!belongsToPanel) return null;
      const raw = String(flagEl.getAttribute("data-aux-pane-open") || "").toLowerCase();
      if (raw === "true") return true;
      if (raw === "false") return false;
      return null;
    };
    const __swissPanelOpen = (panel) => {
      if (!panel) return false;
      if (panel.getAttribute && panel.getAttribute("data-swiss-overview-auto-hidden") === "true") return false;
      const cs = window.getComputedStyle ? window.getComputedStyle(panel) : null;
      if (cs && (cs.display === "none" || cs.visibility === "hidden")) return false;
      if (panel.hidden === true) return false;
      if ((panel.offsetWidth || 0) === 0 && (panel.offsetHeight || 0) === 0) return false;
      return true;
    };
`

// ProbeScript is evaluated in an Antigravity renderer over CDP. It reports
// whether the Antigravity Swiss Knife main-stage page (the extension's main
// section: board / browser / files / memos) is the active view and whether the
// IDE's own right-hand "Overview" panel is currently expanded.
//
// Swiss Knife board detection uses the markers the Swiss Knife injected script
// already maintains inside the IDE (verified in pkg/plugins/github_extension_script.go:
// openMainStage()/applyMainStageLayout() set the inline display, setupLeftNavTabs()
// stamps the body attribute, and header-tab clicks refresh window.__swissActiveMainStageExt):
//   - #swiss-main-stage-container with style.display === "flex"
//   - document.body[data-swiss-stage-active]="<ext>"
//   - window.__swissActiveMainStageExt
//
// Overview panel detection resolves the auxiliary panel container through
// AuxPanelResolverJS and mirrors the visibility rules used by
// pkg/enhancements (computed style + geometry + self-collapse marker).
var ProbeScript = `(() => {
  const fallback = { swiss_board: false, overview_open: false, has_panel: false, panel_source: "none" };
  try {
` + AuxPanelResolverJS + `
    const stage = document.getElementById("swiss-main-stage-container");
    const stageOpen = Boolean(stage && stage.style.display === "flex");
    const stageAttr = (document.body && document.body.getAttribute("data-swiss-stage-active")) || "";
    const extFlag = window.__swissActiveMainStageExt;
    const swissBoard = Boolean(
      stageOpen ||
      (stageAttr !== "" && stageAttr !== "false") ||
      (typeof extFlag === "string" && extFlag !== "")
    );

    const resolved = __swissResolveAuxPanel();
    const panel = resolved.el;

    let overviewOpen = false;
    if (panel) {
      if (panel.getAttribute && panel.getAttribute("data-swiss-overview-auto-hidden") === "true") {
        overviewOpen = false;
      } else {
        overviewOpen = __swissPanelOpen(panel);
        // The IDE's own aux-pane-open marker is a positive signal only: it keeps
        // the panel detected as open during entry/transition frames where the
        // container is momentarily zero-sized but the IDE already considers it open.
        if (!overviewOpen && __swissPanelAttributeOpen(panel) === true) {
          overviewOpen = true;
        }
      }
    }

    return {
      swiss_board: swissBoard,
      overview_open: overviewOpen,
      has_panel: Boolean(panel),
      panel_source: resolved.source
    };
  } catch (err) {
    fallback.error = String(err && err.message ? err.message : err);
    return fallback;
  }
})()`

// CollapseScript collapses the IDE's own Overview panel.
//
// Strategy order (the IDE's own control is always preferred over raw DOM
// mutation):
//  1. native_toggle  - click the panel's own collapse/hide control.
//  2. aux_toggle     - click the IDE's documented auxiliary-sidebar toggle
//     ([data-testid="toggle-aux-sidebar"], the same control
//     pkg/plugins/github_extension.go styles away in the main stage).
//  3. global_toggle  - click an explicitly named Overview/side-panel toggle
//     anywhere in the window.
//  4. style_fallback - scoped inline display:none on the resolved panel
//     container, tagged with data-swiss-overview-auto-hidden so the probe keeps
//     reporting it as collapsed and the watcher can restore it on leaving the
//     board page.
//
// Candidates are matched only when the accessible name mentions BOTH an action
// (collapse/hide/close/toggle) AND the target (overview/auxiliary/side panel),
// so a random "close" button inside a terminal or artifact view is never hit.
// Every strategy is verified against the resolved container: a click that does
// not actually collapse the panel is never reported as success, otherwise the
// watcher would stop trying while the panel stays open.
var CollapseScript = `(async () => {
  const out = { ok: false, hidden: false, method: "none", before: false, after: false, panel_source: "none" };
  try {
` + AuxPanelResolverJS + `
    const resolved = __swissResolveAuxPanel();
    const panel = resolved.el;
    out.panel_source = resolved.source;
    if (!panel) return out;

    const nameOf = (el) => String(
      (el.getAttribute && (el.getAttribute("aria-label") || el.getAttribute("title") || el.getAttribute("data-tooltip-id") || el.getAttribute("data-testid"))) ||
      (el.textContent || "")
    );
    const isToggle = (el) => {
      const n = nameOf(el).toLowerCase();
      if (!n) return false;
      const hasAction = /collapse|hide|close|toggle/.test(n);
      const hasTarget = /overview|auxiliary|aux\b|side\s*panel|side\s*bar|sidebar|right\s*panel/.test(n);
      return hasAction && hasTarget;
    };
    const fireClick = (btn) => {
      try {
        btn.dispatchEvent(new PointerEvent("pointerdown", { bubbles: true, cancelable: true, composed: true }));
        btn.dispatchEvent(new MouseEvent("mousedown", { bubbles: true, cancelable: true, composed: true }));
        btn.dispatchEvent(new PointerEvent("pointerup", { bubbles: true, cancelable: true, composed: true }));
        btn.dispatchEvent(new MouseEvent("mouseup", { bubbles: true, cancelable: true, composed: true }));
        btn.click();
      } catch (_) {
        try { btn.click(); } catch (_) {}
      }
    };

    out.before = __swissPanelOpen(panel);
    if (!out.before) {
      out.ok = true;
      out.hidden = true;
      out.method = "already_collapsed";
      return out;
    }

    // 1. The panel's own control.
    let btn = Array.from(panel.querySelectorAll('button, [role="button"]')).find(el => isToggle(el));
    // 2. The IDE's documented auxiliary-sidebar toggle.
    if (!btn) {
      btn = document.querySelector('[data-testid="toggle-aux-sidebar"]');
    }
    // 3. A window-level control explicitly named for the Overview / side panel.
    if (!btn) {
      btn = Array.from(document.querySelectorAll('button, [role="button"]')).find(el => isToggle(el));
    }
    if (btn) {
      fireClick(btn);
      await new Promise(r => setTimeout(r, 150));
      if (!__swissPanelOpen(panel)) {
        out.ok = true;
        out.hidden = true;
        out.after = false;
        out.method = panel.contains(btn) ? "native_toggle" : "global_toggle";
        return out;
      }
    }

    // 4. Scoped style fallback on the resolved container.
    panel.setAttribute("data-swiss-overview-auto-hidden", "true");
    panel.setAttribute("data-swiss-overview-prev-display", panel.style.display || "");
    panel.style.setProperty("display", "none", "important");
    out.after = __swissPanelOpen(panel);
    out.hidden = !out.after;
    out.ok = out.hidden;
    out.method = out.hidden ? "style_fallback" : "none";
    return out;
  } catch (err) {
    out.error = String(err && err.message ? err.message : err);
    return out;
  }
})()`

// RestoreScript undoes a CollapseScript style_fallback. It only ever touches a
// panel this watcher collapsed itself (tagged with
// data-swiss-overview-auto-hidden), so a panel the IDE or the user closed by
// other means is never force-reopened.
var RestoreScript = `(() => {
  const out = { ok: false, method: "restore", panel_source: "none" };
  try {
` + AuxPanelResolverJS + `
    const resolved = __swissResolveAuxPanel();
    const panel = resolved.el;
    out.panel_source = resolved.source;
    if (!panel) return out;
    if (panel.getAttribute("data-swiss-overview-auto-hidden") !== "true") return out;
    const prev = panel.getAttribute("data-swiss-overview-prev-display") || "";
    panel.removeAttribute("data-swiss-overview-auto-hidden");
    panel.removeAttribute("data-swiss-overview-prev-display");
    if (prev) {
      panel.style.display = prev;
    } else {
      panel.style.removeProperty("display");
    }
    out.ok = true;
    return out;
  } catch (err) {
    out.error = String(err && err.message ? err.message : err);
    return out;
  }
})()`
