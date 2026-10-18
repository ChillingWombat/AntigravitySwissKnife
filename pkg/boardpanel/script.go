package boardpanel

// ProbeScript is evaluated in an Antigravity renderer over CDP. It reports
// whether the Antigravity Swiss Knife board page is the active view and whether
// the IDE's own right-hand "Overview" panel is currently expanded.
//
// Swiss Knife board detection uses the markers the Swiss Knife injected script
// already maintains inside the IDE:
//   - #swiss-main-stage-container with style.display === "flex"
//   - document.body[data-swiss-stage-active]="<ext>"
//   - window.__swissActiveMainStageExt
//
// Overview panel detection mirrors the selectors already used by
// pkg/enhancements: [data-testid*="overview"] first, then the auxiliary panel.
const ProbeScript = `(() => {
  const fallback = { swiss_board: false, overview_open: false, has_panel: false };
  try {
    const stage = document.getElementById("swiss-main-stage-container");
    const stageOpen = Boolean(stage && stage.style.display === "flex");
    const stageAttr = (document.body && document.body.getAttribute("data-swiss-stage-active")) || "";
    const extFlag = window.__swissActiveMainStageExt;
    const swissBoard = Boolean(
      stageOpen ||
      (stageAttr !== "" && stageAttr !== "false") ||
      (typeof extFlag === "string" && extFlag !== "")
    );

    const panel =
      document.querySelector('[data-testid*="overview" i]') ||
      document.querySelector('[data-testid="auxiliary-panel"]') ||
      document.querySelector('.part.auxiliarybar');

    let overviewOpen = false;
    if (panel) {
      if (panel.getAttribute("data-swiss-overview-auto-hidden") === "true") {
        overviewOpen = false;
      } else {
        const cs = window.getComputedStyle ? window.getComputedStyle(panel) : null;
        const w = panel.offsetWidth || 0;
        const h = panel.offsetHeight || 0;
        const visuallyHidden = Boolean(
          (cs && (cs.display === "none" || cs.visibility === "hidden")) ||
          panel.hidden === true ||
          (w === 0 && h === 0)
        );
        overviewOpen = !visuallyHidden;
      }
    }

    return { swiss_board: swissBoard, overview_open: overviewOpen, has_panel: Boolean(panel) };
  } catch (err) {
    fallback.error = String(err && err.message ? err.message : err);
    return fallback;
  }
})()`

// CollapseScript collapses the IDE's own Overview panel.
//
// Strategy order (the IDE's own control is always preferred over raw DOM
// mutation):
//  1. native_toggle  - click the Overview panel's own collapse/hide control.
//  2. global_toggle  - click an explicitly named Overview/side-panel toggle
//     anywhere in the window.
//  3. style_fallback - scoped inline display:none on the panel element, tagged
//     with data-swiss-overview-auto-hidden so the probe keeps reporting it as
//     collapsed and the watcher can restore it on leaving the board page.
//
// Candidates are matched only when the accessible name mentions BOTH an action
// (collapse/hide/close/toggle) AND the target (overview/auxiliary/side panel),
// so a random "close" button inside a terminal or artifact view is never hit.
const CollapseScript = `(async () => {
  const out = { ok: false, hidden: false, method: "none", before: false, after: false };
  try {
    const panel =
      document.querySelector('[data-testid*="overview" i]') ||
      document.querySelector('[data-testid="auxiliary-panel"]') ||
      document.querySelector('.part.auxiliarybar');
    if (!panel) return out;

    const isOpen = (el) => {
      if (!el) return false;
      if (el.getAttribute && el.getAttribute("data-swiss-overview-auto-hidden") === "true") return false;
      const cs = window.getComputedStyle ? window.getComputedStyle(el) : null;
      if (cs && (cs.display === "none" || cs.visibility === "hidden")) return false;
      if (el.hidden === true) return false;
      if ((el.offsetWidth || 0) === 0 && (el.offsetHeight || 0) === 0) return false;
      return true;
    };
    const nameOf = (el) => String(
      (el.getAttribute && (el.getAttribute("aria-label") || el.getAttribute("title") || el.getAttribute("data-tooltip-id"))) ||
      (el.textContent || "")
    );
    const isToggle = (el) => {
      const n = nameOf(el).toLowerCase();
      if (!n) return false;
      const hasAction = /collapse|hide|close|toggle/.test(n);
      const hasTarget = /overview|auxiliary|side\s*panel|side\s*bar|sidebar|right\s*panel/.test(n);
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

    out.before = isOpen(panel);
    if (!out.before) {
      out.ok = true;
      out.hidden = true;
      out.method = "already_collapsed";
      return out;
    }

    // 1. The panel's own control.
    let btn = Array.from(panel.querySelectorAll('button, [role="button"]')).find(el => isToggle(el));
    // 2. A window-level control explicitly named for the Overview / side panel.
    if (!btn) {
      btn = Array.from(document.querySelectorAll('button, [role="button"]')).find(el => isToggle(el));
    }
    if (btn) {
      fireClick(btn);
      await new Promise(r => setTimeout(r, 150));
      if (!isOpen(panel)) {
        out.ok = true;
        out.hidden = true;
        out.after = false;
        out.method = panel.contains(btn) ? "native_toggle" : "global_toggle";
        return out;
      }
    }

    // 3. Scoped style fallback.
    panel.setAttribute("data-swiss-overview-auto-hidden", "true");
    panel.setAttribute("data-swiss-overview-prev-display", panel.style.display || "");
    panel.style.setProperty("display", "none", "important");
    out.after = isOpen(panel);
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
const RestoreScript = `(() => {
  const out = { ok: false, method: "restore" };
  try {
    const panel =
      document.querySelector('[data-testid*="overview" i]') ||
      document.querySelector('[data-testid="auxiliary-panel"]') ||
      document.querySelector('.part.auxiliarybar');
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
