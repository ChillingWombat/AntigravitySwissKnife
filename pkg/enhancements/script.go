package enhancements

import (
	"encoding/json"
	"fmt"
)

// GenerateEnhancementsScript produces client-side JavaScript for Antigravity Desktop & VS Code webviews
// implementing the Devin-style Quick Prompt Jump Bar, Tool/Thinking Color Density Muting/Hiding,
// Conversation Breaker Lines, and Predefined Default Project for New Conversations.
func GenerateEnhancementsScript(cfg *EnhancementsConfig) string {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		cfgJSON = []byte(`{"enabled":true,"prompt_jump_bar":{"enabled":true,"show_tooltip":true,"focus_pulse":true,"sync_scroll":true,"position":"gutter","dash_width":14,"color_mode":"default","custom_color":"#0b57d0"},"tool_density_mode":"muted","breaker_line_enabled":true,"default_new_project":"auto"}`)
	}

	return fmt.Sprintf(`
/* === Antigravity Swiss Knife: App Enhancements === */
(() => {
  try {
    let enhConfig = %s;
    if (!enhConfig) return;
    if (typeof window !== "undefined") {
      window.__SWISS_ENH_CONFIG__ = enhConfig;
      if (enhConfig.overview_panel && enhConfig.overview_panel.aux_tabs_format) {
        localStorage.setItem("antigravity_swiss_aux_tab_format", enhConfig.overview_panel.aux_tabs_format);
      }
      localStorage.setItem("antigravity_swiss_left_panel_enabled", String(enhConfig.left_panel_extensions_enabled !== false));
      localStorage.setItem("antigravity_swiss_left_panel_mode", enhConfig.left_panel_extensions_mode || "single");
      localStorage.setItem("antigravity_swiss_main_section_enabled", String(enhConfig.main_section_extensions_enabled !== false));
    }

    let projectColors = {};

    async function syncConfigFromServer() {
      try {
        if (typeof window !== "undefined" && window.fetch) {
          const res = await fetch("http://127.0.0.1:8765/api/enhancements");
          if (res.ok) {
            const loaded = await res.json();
            if (loaded) {
              enhConfig = loaded;
              window.__SWISS_ENH_CONFIG__ = loaded;
              if (loaded.overview_panel && loaded.overview_panel.aux_tabs_format) {
                const prevFmt = localStorage.getItem("antigravity_swiss_aux_tab_format");
                if (prevFmt !== loaded.overview_panel.aux_tabs_format) {
                  localStorage.setItem("antigravity_swiss_aux_tab_format", loaded.overview_panel.aux_tabs_format);
                  window.dispatchEvent(new CustomEvent("swiss-aux-tab-format-updated"));
                }
              }
              const prevLeftEnabled = localStorage.getItem("antigravity_swiss_left_panel_enabled");
              const prevLeftMode = localStorage.getItem("antigravity_swiss_left_panel_mode");
              const prevMainSection = localStorage.getItem("antigravity_swiss_main_section_enabled");
              const newLeftEnabled = String(loaded.left_panel_extensions_enabled !== false);
              const newLeftMode = loaded.left_panel_extensions_mode || "single";
              const newMainSection = String(loaded.main_section_extensions_enabled !== false);
              if (prevLeftEnabled !== newLeftEnabled || prevLeftMode !== newLeftMode || prevMainSection !== newMainSection) {
                localStorage.setItem("antigravity_swiss_left_panel_enabled", newLeftEnabled);
                localStorage.setItem("antigravity_swiss_left_panel_mode", newLeftMode);
                localStorage.setItem("antigravity_swiss_main_section_enabled", newMainSection);
                window.dispatchEvent(new CustomEvent("swiss-left-nav-config-updated", {
                  detail: { enabled: loaded.left_panel_extensions_enabled !== false, mode: newLeftMode, main_section_enabled: loaded.main_section_extensions_enabled !== false }
                }));
                if (typeof window.setupLeftNavTabs === "function") window.setupLeftNavTabs();
              }
              applyEnhancementsStyles();
              renderPromptJumpBar();
              applyDefaultProjectHandler();
              applyOverviewPanelEnhancements();
            }
          }

          const resGui = await fetch("http://127.0.0.1:8765/api/gui/config");
          if (resGui.ok) {
            const loadedGui = await resGui.json();
            if (loadedGui && loadedGui.project_colors) {
              projectColors = loadedGui.project_colors;
            }
          }
        }
      } catch (_) {}
    }
    syncConfigFromServer();
    setInterval(syncConfigFromServer, 25000);

    function isDarkMode() {
      return document.documentElement.classList.contains("dark") ||
        document.body.classList.contains("dark") ||
        window.getComputedStyle(document.body).backgroundColor.includes("rgb(30,") ||
        window.getComputedStyle(document.body).backgroundColor.includes("rgb(32,");
    }

    function detectActiveProject() {
      const title = document.title || "";
      const parts = title.split(" - ");
      if (parts.length >= 3) {
        return parts[parts.length - 2].trim();
      }
      const activeRow = document.querySelector('[data-testid="conversation-row-sidebar"][aria-selected="true"]') ||
                        document.querySelector('[data-testid="conversation-row-sidebar"].bg-accent') ||
                        document.querySelector('[data-testid="conversation-row-sidebar"][data-state="active"]');
      if (activeRow) {
        return activeRow.getAttribute("data-swiss-project");
      }
      return null;
    }

    function getActiveAndHoverColor() {
      const dark = isDarkMode();
      const mode = enhConfig.prompt_jump_bar?.color_mode || "default";
      const slateGrey = dark ? "#94a3b8" : "#475569"; // Calm slate grey

      let chosen = slateGrey;
      if (mode === "project") {
        const pName = detectActiveProject();
        if (pName && projectColors && projectColors[pName]) {
          chosen = projectColors[pName];
        }
      } else if (mode === "custom") {
        const c = enhConfig.prompt_jump_bar?.custom_color;
        if (c && c !== "#ec4899" && c.trim() !== "") {
          chosen = c;
        }
      }

      if (chosen && chosen.startsWith("#")) {
        let hex = chosen.slice(1);
        if (hex.length === 3) {
          hex = hex[0] + hex[0] + hex[1] + hex[1] + hex[2] + hex[2];
        }
        if (hex.length >= 6) {
          const r = parseInt(hex.slice(0, 2), 16);
          const g = parseInt(hex.slice(2, 4), 16);
          const b = parseInt(hex.slice(4, 6), 16);
          if (!isNaN(r) && !isNaN(g) && !isNaN(b)) {
            const lum = (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255.0;
            if (dark && lum < 0.15) {
              return "#e2e8f0";
            }
            if (!dark && lum > 0.85) {
              return "#334155";
            }
          }
        }
      }

      return chosen;
    }

    function applyEnhancementsStyles() {
      let styleTag = document.getElementById("swiss-enhancements-dynamic-styles");
      if (!styleTag) {
        styleTag = document.createElement("style");
        styleTag.id = "swiss-enhancements-dynamic-styles";
        document.head.appendChild(styleTag);
      }

      if (!enhConfig.enabled) {
        styleTag.textContent = "";
        return;
      }

      const dark = isDarkMode();
      let css = "";
      const densityMode = enhConfig.tool_density_mode || "muted";
      const breakerEnabled = enhConfig.breaker_line_enabled !== false;

      if (densityMode === "muted") {
        css += `+"`"+`
          [data-testid="worked-for-collapsible"],
          [data-testid="thinking-collapsible-trigger"],
          [data-testid="tool-group-collapsible"],
          [data-testid="run-command-step"],
          [data-testid="subagent-node"],
          .thinking-collapsible {
            opacity: 0.48 !important;
            filter: grayscale(0.65) !important;
            transition: opacity 0.2s ease, filter 0.2s ease !important;
          }
          [data-testid="worked-for-collapsible"]:hover,
          [data-testid="thinking-collapsible-trigger"]:hover,
          [data-testid="tool-group-collapsible"]:hover,
          [data-testid="run-command-step"]:hover,
          [data-testid="subagent-node"]:hover,
          .thinking-collapsible:hover {
            opacity: 0.95 !important;
            filter: none !important;
          }
        `+"`"+`;
      } else if (densityMode === "hidden") {
        css += `+"`"+`
          [data-testid="worked-for-collapsible"],
          [data-testid="thinking-collapsible-trigger"],
          [data-testid="tool-group-collapsible"],
          [data-testid="run-command-step"],
          [data-testid="subagent-node"],
          .thinking-collapsible {
            display: none !important;
          }
        `+"`"+`;
      }

      if (breakerEnabled) {
        css += `+"`"+`
          [data-testid="user-input-step"]:not(:first-child) {
            border-top: 1px solid var(--border, rgba(0, 0, 0, 0.075)) !important;
            margin-top: 24px !important;
            padding-top: 20px !important;
          }
        `+"`"+`;
      }

      css += `+"`"+`
        .swiss-prompt-dash {
          transition: width 0.18s cubic-bezier(0.4, 0, 0.2, 1), background 0.15s ease !important;
        }
        .swiss-prompt-dash:hover {
          width: 22px !important;
        }
      `+"`"+`;

      // Overview Panel Styling
      const op = enhConfig.overview_panel;
      if (op && op.enabled) {
        if (op.division_style === "divider_line") {
          const thickness = op.line_thickness || 1;
          const widthPct = op.line_width_percent || 100;
          let lineCol = op.line_color;
          if (!lineCol || lineCol === "#e2e8f0") {
            lineCol = dark ? "rgba(255, 255, 255, 0.12)" : "#e2e8f0";
          }
          const extraMargin = op.line_margin || 0;
          const lineStyle = op.line_style || "solid";
          css += `+"`"+`
            .swiss-overview-divider {
              height: 0px !important;
              min-height: 0px !important;
              max-height: 0px !important;
              padding: 0 !important;
              overflow: hidden !important;
              font-size: 0px !important;
              line-height: 0 !important;
              width: calc(${widthPct}%% - 12px) !important;
              border: none !important;
              border-top: ${thickness}px ${lineStyle} ${lineCol} !important;
              margin-top: calc(-12px - 7px + ${extraMargin}px) !important;
              margin-bottom: calc(-12px - 7px + ${extraMargin}px) !important;
              margin-left: 6px !important;
              margin-right: 6px !important;
              display: block !important;
              box-sizing: border-box !important;
            }
          `+"`"+`;
        } else if (op.division_style === "border_zone") {
          const radius = op.zone_border_radius || 8;
          const zoneBorder = op.zone_border_color || (dark ? "rgba(255, 255, 255, 0.12)" : "#e2e8f0");
          const padding = op.zone_padding || 10;
          const gap = op.zone_gap || 10;
          const bgCol = dark ? "#212124" : (op.zone_background_contrast === "whiter" ? "#ffffff" : "#f8fafc");
          css += `+"`"+`
            .swiss-overview-zone {
              background-color: ${bgCol} !important;
              border: 1px solid ${zoneBorder} !important;
              border-radius: ${radius}px !important;
              padding: ${padding}px !important;
              margin-bottom: ${gap}px !important;
              box-shadow: 0 1px 3px rgba(0, 0, 0, 0.03) !important;
              transition: border-color 0.15s ease, box-shadow 0.15s ease !important;
            }
            .swiss-overview-zone:hover {
              border-color: ${dark ? "rgba(255, 255, 255, 0.22)" : "#cbd5e1"} !important;
            }
          `+"`"+`;
        }

        if (op.replace_see_all_triangle) {
          css += `+"`"+`
            [data-swiss-overview-divider="true"] {
              display: flex !important;
              align-items: center !important;
              justify-content: center !important;
              width: 100%% !important;
              height: 14px !important;
              min-height: 14px !important;
              max-height: 14px !important;
              padding: 0 !important;
              margin: 0 !important;
              background: transparent !important;
              border: none !important;
              box-shadow: none !important;
              cursor: pointer !important;
              outline: none !important;
              line-height: 1 !important;
            }
            .swiss-overview-tabs-divider {
              position: relative !important;
              display: flex !important;
              align-items: center !important;
              justify-content: center !important;
              width: 100%% !important;
              height: 14px !important;
              min-height: 14px !important;
              max-height: 14px !important;
              padding: 0 !important;
              margin: 0 auto !important;
              box-sizing: border-box !important;
              cursor: pointer !important;
              user-select: none !important;
              line-height: 1 !important;
            }
            .swiss-overview-tabs-pill {
              display: inline-flex !important;
              align-items: center !important;
              justify-content: center !important;
              width: 14px !important;
              height: 10px !important;
              color: #64748b !important;
              font-size: 8px !important;
              background: transparent !important;
              transition: all 0.18s ease !important;
            }
            .swiss-overview-tabs-triangle {
              display: inline-block !important;
              font-size: 8px !important;
              line-height: 1 !important;
              text-align: center !important;
              transition: transform 0.2s cubic-bezier(0.4, 0, 0.2, 1) !important;
            }
            [data-theme="dark"] .swiss-overview-tabs-pill,
            .dark .swiss-overview-tabs-pill {
              color: #94a3b8 !important;
            }
            [data-swiss-overview-divider="true"]:hover .swiss-overview-tabs-pill,
            .swiss-overview-tabs-divider:hover .swiss-overview-tabs-pill {
              color: #1e293b !important;
              transform: scale(1.18) !important;
            }
            [data-theme="dark"] [data-swiss-overview-divider="true"]:hover .swiss-overview-tabs-pill,
            .dark [data-swiss-overview-divider="true"]:hover .swiss-overview-tabs-pill {
              color: #f1f5f9 !important;
            }
            .swiss-see-triangle-btn {
              cursor: pointer !important;
              display: inline-flex !important;
              align-items: center !important;
              gap: 4px !important;
              font-size: 11px !important;
              font-weight: 600 !important;
              color: var(--primary, #0b57d0) !important;
              padding: 2px 6px !important;
              border-radius: 4px !important;
              background-color: rgba(11, 87, 208, 0.08) !important;
              border: 1px solid rgba(11, 87, 208, 0.15) !important;
              user-select: none !important;
            }
            .swiss-see-triangle-btn:hover {
              background-color: rgba(11, 87, 208, 0.15) !important;
            }
          `+"`"+`;
        }
      }

      if (styleTag.textContent !== css) {
        styleTag.textContent = css;
      }
    }

    let isClickJumping = false;
    let clickTargetIdx = -1;
    let jumpTimer = null;

    let lastRenderedStepsCount = -1;
    let lastActiveIdx = -1;
    let lastAppliedActiveColor = "";
    let lastAppliedDefaultColor = "";

    function getLowestPromptOnScreen(steps, viewport) {
      if (isClickJumping && clickTargetIdx >= 0) {
        return clickTargetIdx;
      }

      const vpRect = viewport ? viewport.getBoundingClientRect() : { top: 0, bottom: window.innerHeight };
      let latestVisibleIdx = -1;

      for (let i = 0; i < steps.length; i++) {
        const r = steps[i].getBoundingClientRect();
        if (r.top < vpRect.bottom - 20 && r.bottom > vpRect.top + 20) {
          latestVisibleIdx = i;
        }
      }

      if (latestVisibleIdx !== -1) {
        return latestVisibleIdx;
      }

      for (let i = steps.length - 1; i >= 0; i--) {
        const r = steps[i].getBoundingClientRect();
        if (r.top <= vpRect.top + 40) {
          return i;
        }
      }

      return 0;
    }

    function renderPromptJumpBar() {
      const isEnabled = enhConfig.enabled && enhConfig.prompt_jump_bar?.enabled;
      let bar = document.getElementById("swiss-prompt-jump-bar");
      let tooltip = document.getElementById("swiss-prompt-tooltip");

      if (!isEnabled) {
        if (bar) bar.remove();
        if (tooltip) tooltip.remove();
        lastRenderedStepsCount = -1;
        lastActiveIdx = -1;
        return;
      }

      const steps = Array.from(document.querySelectorAll('[data-testid="user-input-step"]'));
      if (steps.length === 0) {
        if (bar) bar.style.display = "none";
        lastRenderedStepsCount = 0;
        return;
      }

      const convView = document.querySelector('[data-testid="conversation-view"]');
      const viewport = document.querySelector('[data-testid="autoscroll-viewport"]') ||
                       document.querySelector('.overflow-y-auto');
      const targetParent = convView || document.body;

      if (!bar) {
        bar = document.createElement("div");
        bar.id = "swiss-prompt-jump-bar";
        bar.setAttribute("data-swiss-component", "prompt-jump-bar");
        targetParent.appendChild(bar);
      } else if (bar.parentElement !== targetParent) {
        targetParent.appendChild(bar);
      }

      bar.style.display = "flex";

      const dark = isDarkMode();
      const isGutter = enhConfig.prompt_jump_bar?.position !== "floating";
      const dashWidth = enhConfig.prompt_jump_bar?.dash_width || 14;

      if (isGutter && convView) {
        bar.style.cssText = "position: absolute; left: 18px; top: 28px; z-index: 45; display: flex; flex-direction: column; gap: 7px; padding: 4px 2px; background: transparent; border: none; box-shadow: none; user-select: none; pointer-events: none;";
      } else {
        bar.style.cssText = "position: fixed; left: 268px; top: 120px; z-index: 9999; display: flex; flex-direction: column; gap: 7px; padding: 4px 2px; background: transparent; border: none; box-shadow: none; user-select: none; pointer-events: none;";
      }

      if (!tooltip) {
        tooltip = document.createElement("div");
        tooltip.id = "swiss-prompt-tooltip";
        tooltip.style.cssText = "position: fixed; display: none; z-index: 10000; background: var(--card, #ffffff); color: var(--foreground, #101010); padding: 6px 10px; border-radius: 6px; font-size: 11px; max-width: 280px; box-shadow: 0 4px 12px rgba(0,0,0,0.1); pointer-events: none; font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.4; border: 1px solid var(--border, rgba(0,0,0,0.08));";
        document.body.appendChild(tooltip);
      }

      const defaultColor = dark ? "rgba(148, 163, 184, 0.45)" : "rgba(100, 116, 139, 0.42)";
      const activeColor = getActiveAndHoverColor();

      function updateActiveIndicator(activeIdx) {
        if (activeIdx === lastActiveIdx && activeColor === lastAppliedActiveColor && defaultColor === lastAppliedDefaultColor) {
          return;
        }

        const dashes = bar.querySelectorAll(".swiss-prompt-dash");
        dashes.forEach((d, idx) => {
          if (idx === activeIdx) {
            d.classList.add("active");
            d.style.height = "3.5px";
            d.style.background = activeColor;
          } else {
            d.classList.remove("active");
            d.style.height = "1.5px";
            d.style.background = defaultColor;
          }
        });

        lastActiveIdx = activeIdx;
        lastAppliedActiveColor = activeColor;
        lastAppliedDefaultColor = defaultColor;
      }

      // If dashes already match steps count, DO NOT rebuild DOM elements!
      // This eliminates the shining / flickering bug caused by destroying & recreating DOM nodes during streaming.
      const existingDashes = bar.querySelectorAll(".swiss-prompt-dash");
      if (existingDashes.length === steps.length && lastRenderedStepsCount === steps.length) {
        updateActiveIndicator(getLowestPromptOnScreen(steps, viewport));
        return;
      }

      // Rebuild dashes only when step count actually changes
      bar.innerHTML = "";
      lastRenderedStepsCount = steps.length;

      steps.forEach((step, idx) => {
        const textEl = step.querySelector('.whitespace-pre-wrap') || step;
        const fullText = (textEl.textContent || "").replace(/\s+/g, ' ').trim();
        const snippet = fullText.length > 75 ? fullText.substring(0, 75) + "..." : fullText;

        const dash = document.createElement("div");
        dash.className = "swiss-prompt-dash";
        dash.setAttribute("data-prompt-index", idx);

        const hoverWidth = Math.max(22, dashWidth + 8);

        // Height is explicitly fixed without transition to prevent bouncing / shining
        dash.style.cssText = "width: " + dashWidth + "px; height: 1.5px; border-radius: 2px; background: " + defaultColor + "; cursor: pointer; pointer-events: auto; transition: width 0.18s cubic-bezier(0.4, 0, 0.2, 1), background 0.15s ease;";

        dash.onmouseenter = () => {
          dash.style.width = hoverWidth + "px";
          dash.style.background = activeColor;
          if (enhConfig.prompt_jump_bar?.show_tooltip && tooltip) {
            const rect = dash.getBoundingClientRect();
            tooltip.innerHTML = "<div style='font-weight: 500; font-size: 11px; color: var(--muted-foreground, #64748b); margin-bottom: 2px;'>Prompt #" + (idx + 1) + (dash.classList.contains("active") ? " (Current)" : "") + "</div><div style='color: var(--foreground, #101010); font-size: 12px; font-weight: 400;'>" + snippet + "</div>";
            tooltip.style.left = (rect.right + 12) + "px";
            tooltip.style.top = (rect.top - 10) + "px";
            tooltip.style.display = "block";
          }
        };

        dash.onmouseleave = () => {
          dash.style.width = dashWidth + "px";
          const cur = dash.classList.contains("active");
          dash.style.background = cur ? activeColor : defaultColor;
          if (tooltip) tooltip.style.display = "none";
        };

        dash.onclick = () => {
          isClickJumping = true;
          clickTargetIdx = idx;
          if (jumpTimer) clearTimeout(jumpTimer);

          updateActiveIndicator(idx);
          step.scrollIntoView({ behavior: "smooth", block: "start" });

          if (enhConfig.prompt_jump_bar?.focus_pulse) {
            step.style.outline = "2px solid " + activeColor;
            step.style.outlineOffset = "4px";
            step.style.borderRadius = "8px";
            step.style.transition = "outline 0.4s ease";
            setTimeout(() => {
              step.style.outline = "none";
            }, 1200);
          }

          jumpTimer = setTimeout(() => {
            isClickJumping = false;
            clickTargetIdx = -1;
            const curSteps = Array.from(document.querySelectorAll('[data-testid="user-input-step"]'));
            updateActiveIndicator(getLowestPromptOnScreen(curSteps, viewport));
          }, 1000);
        };

        bar.appendChild(dash);
      });

      lastActiveIdx = -1; // Force indicator sync on fresh render
      updateActiveIndicator(getLowestPromptOnScreen(steps, viewport));
    }

    // Default Predefined Project for New Conversations
    function getSectionIdForProject(projectName) {
      if (!projectName || projectName === "auto") return null;
      const target = projectName.trim().toLowerCase();

      // Method 1: Scan project links in sidebar headers: a[aria-label="New Conversation in Project"]
      const links = document.querySelectorAll('a[aria-label="New Conversation in Project"]');
      for (const l of links) {
        const header = l.closest('.group\\/header, [data-project-card], [data-index]');
        const text = (header ? header.textContent : "").trim().toLowerCase();
        if (text.includes(target)) {
          const href = l.getAttribute("href") || "";
          const m = href.match(/section=([a-zA-Z0-9_-]+)/);
          if (m) return m[1];
        }
      }

      // Method 2: Scan elements tagged with data-swiss-project
      const swissEls = document.querySelectorAll('[data-swiss-project]');
      for (const el of swissEls) {
        const pName = (el.getAttribute("data-swiss-project") || "").toLowerCase();
        if (pName.includes(target) || target.includes(pName)) {
          const l = el.querySelector('a[aria-label="New Conversation in Project"]') ||
                    el.closest('[data-index]')?.querySelector('a[aria-label="New Conversation in Project"]');
          if (l) {
            const m = (l.getAttribute("href") || "").match(/section=([a-zA-Z0-9_-]+)/);
            if (m) return m[1];
          }
        }
      }

      // Method 3: Inspect React Fiber on conversation list sidebar
      const sidebar = document.querySelector('[data-testid="conversation-list-sidebar"]');
      if (sidebar) {
        const fiberKey = Object.keys(sidebar).find(k => k.startsWith('__reactFiber'));
        let fiber = sidebar[fiberKey];
        while (fiber) {
          if (fiber.memoizedProps?.items) {
            for (const it of fiber.memoizedProps.items) {
              if (it.type === "header" && it.label && it.label.toLowerCase().includes(target)) {
                return it.id.replace("header-", "");
              }
            }
            break;
          }
          fiber = fiber.return;
        }
      }

      return null;
    }

    function applyDefaultProjectHandler() {
      const defaultProj = enhConfig.default_new_project;
      if (!defaultProj || defaultProj === "auto") {
        // Reset "+ New Conversation" button href to default "/"
        const btn = document.querySelector('[data-testid="new-conversation-button"]');
        if (btn && btn.getAttribute("data-swiss-overridden") === "true") {
          btn.setAttribute("href", "/");
          btn.removeAttribute("data-swiss-overridden");
        }
        return;
      }

      const sectionId = getSectionIdForProject(defaultProj);
      if (!sectionId) return;

      const targetHref = "/?section=" + sectionId;

      // 1. Update "+ New Conversation" button
      const newConvBtn = document.querySelector('[data-testid="new-conversation-button"]');
      if (newConvBtn) {
        newConvBtn.setAttribute("href", targetHref);
        newConvBtn.setAttribute("data-swiss-overridden", "true");

        if (!newConvBtn.__swissDefaultBound) {
          newConvBtn.__swissDefaultBound = true;
          newConvBtn.addEventListener("click", (e) => {
            const defP = enhConfig.default_new_project;
            if (defP && defP !== "auto") {
              const sid = getSectionIdForProject(defP);
              if (sid) {
                const dest = "/?section=" + sid;
                if (window.location.pathname === "/" && window.location.search === "?section=" + sid) {
                  return;
                }
                e.preventDefault();
                e.stopPropagation();
                if (typeof window.closeMainStage === "function") {
                  try { window.closeMainStage(); } catch (_) {}
                }
                try {
                  window.history.pushState({}, "", dest);
                  window.dispatchEvent(new PopStateEvent("popstate"));
                } catch (_) {}
                if (window.location.search !== "?section=" + sid) {
                  window.location.href = dest;
                }
              }
            }
          }, true);
        }
      }

      // Also support titlebar new conversation button
      const appIconBtn = document.querySelector('[data-testid="app-icon-new-conversation-button"]');
      if (appIconBtn && !appIconBtn.__swissDefaultBound) {
        appIconBtn.__swissDefaultBound = true;
        appIconBtn.addEventListener("click", (e) => {
          const defP = enhConfig.default_new_project;
          if (defP && defP !== "auto") {
            const sid = getSectionIdForProject(defP);
            if (sid) {
              const dest = "/?section=" + sid;
              if (window.location.pathname === "/" && window.location.search === "?section=" + sid) {
                return;
              }
              e.preventDefault();
              e.stopPropagation();
              if (typeof window.closeMainStage === "function") {
                try { window.closeMainStage(); } catch (_) {}
              }
              try {
                window.history.pushState({}, "", dest);
                window.dispatchEvent(new PopStateEvent("popstate"));
              } catch (_) {}
              if (window.location.search !== "?section=" + sid) {
                window.location.href = dest;
              }
            }
          }
        }, true);
      }

      // 2. Draft screen checks (when landing on "/" without section)
      if (window.location.pathname === "/") {
        const search = window.location.search || "";
        if (!search.includes("section=")) {
          // No section specified, automatically route to predefined project
          window.location.replace(targetHref);
          return;
        }

        // 3. Inspect project selector trigger in draft conversation
        const trigger = document.querySelector('[data-testid="project-selector-trigger"]');
        if (trigger) {
          const curLabel = trigger.getAttribute("aria-label") || trigger.textContent || "";
          if (!curLabel.toLowerCase().includes(defaultProj.toLowerCase())) {
            if (!trigger.__swissAutoSwitching) {
              trigger.__swissAutoSwitching = true;
              trigger.click();
              setTimeout(() => {
                const items = Array.from(document.querySelectorAll('[data-testid="project-selector-item"], [role="menuitem"], [role="option"]'));
                for (const item of items) {
                  if ((item.textContent || "").toLowerCase().includes(defaultProj.toLowerCase())) {
                    item.click();
                    break;
                  }
                }
                setTimeout(() => {
                  trigger.__swissAutoSwitching = false;
                }, 500);
              }, 150);
            }
          }
        }
      }
    }

    function applyOverviewPanelEnhancements() {
      // Ensure terminal scope selector and terminal container remain strictly in factory style
      document.querySelectorAll(".swiss-overview-zone").forEach(el => {
        const text = (el.textContent || "");
        if (text.includes('Terminals') ||
            el.closest('[data-tab-id="terminal"]') ||
            el.querySelector('[data-testid*="scope"], [aria-label*="scope" i], [aria-label*="terminal" i]') ||
            Array.from(el.querySelectorAll('h3, h4, [role="heading"], span, div')).some(h => (h.textContent || '').trim() === 'Terminals')) {
          el.classList.remove("swiss-overview-zone");
        }
      });
      document.querySelectorAll(".swiss-overview-divider").forEach(el => {
        if (el.nextElementSibling && (
            (el.nextElementSibling.textContent || '').includes('Terminals') ||
            el.nextElementSibling.closest('[data-tab-id="terminal"]') ||
            el.nextElementSibling.querySelector('[data-testid*="scope"], [aria-label*="scope" i]')
        )) {
          el.remove();
        }
      });

      const op = enhConfig.overview_panel;
      if (!op || !op.enabled) {
        document.querySelectorAll(".swiss-overview-zone").forEach(el => el.classList.remove("swiss-overview-zone"));
        document.querySelectorAll(".swiss-overview-divider").forEach(el => el.remove());
        document.querySelectorAll(".swiss-overview-bottom-spacer").forEach(el => el.remove());
        document.querySelectorAll('[data-swiss-overview-divider="true"]').forEach(btn => {
          btn.removeAttribute("data-swiss-overview-divider");
          const orig = btn.getAttribute("data-orig-see-text") || "See all";
          btn.textContent = orig;
        });
        document.querySelectorAll(".swiss-overview-parent").forEach(el => {
          el.classList.remove("swiss-overview-parent");
          el.style.removeProperty("gap");
        });
        return;
      }

      // Cleanup unused division styles:
      // Strip any residual card styling if border_zone is not selected
      if (op.division_style !== "border_zone") {
        document.querySelectorAll(".swiss-overview-zone").forEach(el => el.classList.remove("swiss-overview-zone"));
        document.querySelectorAll(".swiss-overview-parent").forEach(el => el.style.removeProperty("gap"));
      }
      // Strip any divider lines if divider_line is not selected
      if (op.division_style !== "divider_line") {
        document.querySelectorAll(".swiss-overview-divider").forEach(el => el.remove());
      }

      const titles = [
        "Subagents",
        "Files Changed",
        "Artifacts",
        "Uploads",
        "Background Tasks",
        "Goals",
        "Goal",
        "Skills Used"
      ];

      const searchRoot = document.querySelector('[data-testid*="overview"]') ||
                         document.querySelector('[data-testid="auxiliary-panel"], .part.auxiliarybar, aside') ||
                         document;
      const allCandidates = Array.from(searchRoot.querySelectorAll("h3, h4, [role='heading'], span, button, div"));
      const sectionHeaders = allCandidates.filter(el => {
        if (!el || el.children.length > 2) return false;
        // Never style terminal scope selector or terminal containers: preserve factory style
        const elText = (el.textContent || "").trim();
        if (elText.toLowerCase().includes("terminal") ||
            el.closest('[data-tab-id="terminal"]') ||
            el.closest('.terminal-view') ||
            el.querySelector('[data-testid*="scope"], [aria-label*="scope" i]') ||
            el.parentElement?.querySelector('button[aria-haspopup="menu"]')) {
          return false;
        }
        return titles.some(t => elText === t || elText.startsWith(t + " ") || elText.startsWith(t + "(") || (elText.startsWith(t) && /^\d+$/.test(elText.slice(t.length))));
      });

      const sectionContainers = [];

      sectionHeaders.forEach((hdr) => {
        if (hdr.closest('[data-tab-id="terminal"]') || (hdr.textContent || '').includes('Terminals')) {
          return;
        }
        let container = hdr.closest('.gap-6 > div') ||
                        hdr.closest('.w-full.flex.flex-col.gap-2') ||
                        hdr.closest('[class*="flex-col"][class*="gap-2"]') ||
                        hdr.closest('[class*="section"]') ||
                        hdr.closest('[data-testid*="section"]') ||
                        (hdr.parentElement && hdr.parentElement !== document.body ? hdr.parentElement : null);
        if (!container) return;

        // Skip hidden containers
        if (container.offsetParent === null && container.offsetHeight === 0 && (!container.classList || !container.classList.contains("gap-6"))) {
          const cs = window.getComputedStyle(container);
          if (cs.display === "none") return;
        }

        if (!sectionContainers.includes(container)) {
          sectionContainers.push(container);
        }
      });

      // Clean up orphaned or misplaced divider lines (before first section, after last section, or next to non-sections)
      document.querySelectorAll(".swiss-overview-divider").forEach(div => {
        const next = div.nextElementSibling;
        if (!next || !sectionContainers.includes(next) || sectionContainers.indexOf(next) === 0) {
          div.remove();
        }
      });

      sectionContainers.forEach((container, idx) => {
        const parent = container.parentElement;
        if (parent && !parent.classList.contains("swiss-overview-parent")) {
          parent.classList.add("swiss-overview-parent");
        }
        if (op.division_style === "border_zone") {
          if (!container.classList.contains("swiss-overview-zone")) {
            container.classList.add("swiss-overview-zone");
          }
          if (parent) {
            parent.style.setProperty("gap", "0px", "important");
          }
        } else if (op.division_style === "divider_line") {
          if (idx > 0) {
            let divider = container.previousElementSibling;
            if (!divider || !divider.classList.contains("swiss-overview-divider")) {
              divider = document.createElement("div");
              divider.className = "swiss-overview-divider";
              container.parentElement?.insertBefore(divider, container);
            }
            const parent = container.parentElement;
            if (parent) {
              const cs = window.getComputedStyle(parent);
              let rowGap = parseFloat(cs.rowGap || cs.gap);
              if (isNaN(rowGap) || rowGap <= 0) {
                if (parent.classList && (parent.classList.contains("gap-6") || parent.className.includes("gap-6"))) {
                  rowGap = 24;
                } else if (parent.classList && (parent.classList.contains("gap-4") || parent.className.includes("gap-4"))) {
                  rowGap = 16;
                } else if (parent.classList && (parent.classList.contains("gap-2") || parent.className.includes("gap-2"))) {
                  rowGap = 8;
                } else {
                  rowGap = 24;
                }
              }
              const desiredHalfGap = 5 + Math.max(0, op.line_margin || 0);
              const netMargin = (rowGap > 0) ? (desiredHalfGap - rowGap) : desiredHalfGap;
              divider.style.setProperty("margin-top", netMargin + "px", "important");
              divider.style.setProperty("margin-bottom", netMargin + "px", "important");
              const wPct = op.line_width_percent || 100;
              if (wPct >= 100) {
                divider.style.setProperty("margin-left", "6px", "important");
                divider.style.setProperty("margin-right", "6px", "important");
              } else {
                divider.style.setProperty("margin-left", "auto", "important");
                divider.style.setProperty("margin-right", "auto", "important");
              }
              divider.style.setProperty("width", "calc(" + wPct + "%% - 12px)", "important");
            }
          }
        }
      });

      const seeButtons = [];
      sectionContainers.forEach(container => {
        Array.from(container.querySelectorAll("button, a, span, div[role='button']")).forEach(el => {
          if (!el || el.closest('[data-index]') || el.closest('[data-testid*="sidebar"]') || el.getAttribute("data-swiss-divider") === "true") return;
          const t = (el.textContent || "").trim();
          const isMore = /^(see|show)\s+(all|more)/i.test(t);
          const isLess = /^(see|show)\s+(less|fewer)/i.test(t);
          if (el.getAttribute("data-swiss-overview-divider") === "true" || isMore || isLess) {
            if (!seeButtons.includes(el)) seeButtons.push(el);
          }
        });
      });

      if (op.replace_see_all_triangle) {
        seeButtons.forEach(btn => {
          let rawText = (btn.textContent || "").trim();
          if (rawText === "▾" || rawText === "▴" || rawText === "▼" || rawText === "▲" || !rawText) {
            rawText = btn.getAttribute("data-orig-see-text") || "See all";
          }
          const lower = rawText.toLowerCase();
          const isExpandMore = lower.includes("see all") || lower.includes("show more") || lower.includes("see more") || lower.includes("show all");
          const symbol = isExpandMore ? "▾" : "▴";
          const titleText = isExpandMore ? rawText : (lower.includes("less") || lower.includes("fewer") ? rawText : "Show fewer");
          btn.setAttribute("data-swiss-overview-divider", "true");
          btn.setAttribute("data-orig-see-text", rawText);
          btn.setAttribute("title", titleText);
          if (!btn.__swissOverviewClickBound) {
            btn.__swissOverviewClickBound = true;
            btn.addEventListener("click", () => {
              const curOrig = btn.getAttribute("data-orig-see-text") || "See all";
              const curLower = curOrig.toLowerCase();
              const wasMore = curLower.includes("see all") || curLower.includes("show more") || curLower.includes("see more") || curLower.includes("show all");
              const nextOrig = wasMore
                ? (curLower.includes("show") ? "Show less" : "See less")
                : (curLower.includes("show") ? "Show more" : "See all");
              btn.setAttribute("data-orig-see-text", nextOrig);
            });
          }
          const curTriangle = btn.querySelector(".swiss-overview-tabs-triangle");
          if (!curTriangle || curTriangle.textContent !== symbol) {
            btn.innerHTML = '<div class="swiss-overview-tabs-divider">' +
              '<div class="swiss-overview-tabs-pill"><span class="swiss-overview-tabs-triangle">' + symbol + '</span></div>' +
            '</div>';
          }
        });
      } else {
        document.querySelectorAll('[data-swiss-overview-divider="true"]').forEach(btn => {
          btn.removeAttribute("data-swiss-overview-divider");
          const orig = btn.getAttribute("data-orig-see-text") || "See all";
          btn.textContent = orig;
        });
      }

      searchRoot.querySelectorAll(".swiss-overview-bottom-spacer").forEach(el => el.remove());
    }

    // Attach scroll and DOM observers
    if (!window.__swissEnhancementsObserver) {
      window.__swissEnhancementsObserver = true;

      const updateScrollHandler = () => {
        if (isClickJumping) return;
        requestAnimationFrame(() => {
          const bar = document.getElementById("swiss-prompt-jump-bar");
          if (!bar) return;
          const steps = Array.from(document.querySelectorAll('[data-testid="user-input-step"]'));
          const viewport = document.querySelector('[data-testid="autoscroll-viewport"]') ||
                           document.querySelector('.overflow-y-auto');

          const activeIdx = getLowestPromptOnScreen(steps, viewport);
          const dark = isDarkMode();
          const defaultColor = dark ? "rgba(148, 163, 184, 0.45)" : "rgba(100, 116, 139, 0.42)";
          const activeColor = getActiveAndHoverColor();

          if (activeIdx === lastActiveIdx && activeColor === lastAppliedActiveColor && defaultColor === lastAppliedDefaultColor) {
            return;
          }

          bar.querySelectorAll(".swiss-prompt-dash").forEach((d, idx) => {
            if (idx === activeIdx) {
              d.classList.add("active");
              d.style.height = "3.5px";
              d.style.background = activeColor;
            } else {
              d.classList.remove("active");
              d.style.height = "1.5px";
              d.style.background = defaultColor;
            }
          });

          lastActiveIdx = activeIdx;
          lastAppliedActiveColor = activeColor;
          lastAppliedDefaultColor = defaultColor;
        });
      };

      const handleUserInteraction = () => {
        if (isClickJumping) {
          isClickJumping = false;
          clickTargetIdx = -1;
          if (jumpTimer) clearTimeout(jumpTimer);
          updateScrollHandler();
        }
      };

      const vp = document.querySelector('[data-testid="autoscroll-viewport"]') || document.querySelector('.overflow-y-auto');
      if (vp) {
        vp.addEventListener("scroll", updateScrollHandler, { passive: true });
        vp.addEventListener("wheel", handleUserInteraction, { passive: true });
        vp.addEventListener("touchmove", handleUserInteraction, { passive: true });
      }

      window.addEventListener("scroll", updateScrollHandler, { passive: true, capture: true });
      window.addEventListener("wheel", handleUserInteraction, { passive: true });
      window.addEventListener("touchmove", handleUserInteraction, { passive: true });
      window.addEventListener("keydown", (e) => {
        if (["ArrowUp", "ArrowDown", "PageUp", "PageDown", "Home", "End", " "].includes(e.key)) {
          handleUserInteraction();
        }
        // Keyboard shortcut Ctrl+N / Cmd+N interceptor for default project
        if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "n" && !e.shiftKey && !e.altKey) {
          const defProj = enhConfig.default_new_project;
          if (defProj && defProj !== "auto") {
            const sid = getSectionIdForProject(defProj);
            if (sid) {
              e.preventDefault();
              e.stopPropagation();
              window.location.href = "/?section=" + sid;
            }
          }
        }
      }, { passive: false });

      let scheduledRaf = null;
      let lastObservedStepCount = -1;
      const ob = new MutationObserver((mutations) => {
        const hasExternalMutation = mutations.some(m => {
          const target = m.target;
          if (target && target.nodeType === 1) {
            const el = target;
            if (el.id && el.id.startsWith("swiss-")) return false;
            if (el.hasAttribute && el.hasAttribute("data-swiss-overview-divider")) return false;
            if (el.classList && (
              el.classList.contains("swiss-overview-zone") ||
              el.classList.contains("swiss-overview-divider") ||
              el.classList.contains("swiss-overview-bottom-spacer") ||
              el.classList.contains("swiss-overview-spacer-line") ||
              el.classList.contains("swiss-overview-tabs-divider") ||
              el.classList.contains("swiss-overview-tabs-pill") ||
              el.classList.contains("swiss-overview-tabs-line") ||
              el.classList.contains("swiss-overview-tabs-triangle") ||
              el.classList.contains("swiss-prompt-dash") ||
              el.classList.contains("swiss-see-triangle-btn")
            )) return false;
          }
          return true;
        });
        if (!hasExternalMutation) return;

        if (scheduledRaf) return;
        scheduledRaf = requestAnimationFrame(() => {
          scheduledRaf = null;
          applyEnhancementsStyles();

          const curSteps = document.querySelectorAll('[data-testid="user-input-step"]').length;
          if (curSteps !== lastObservedStepCount) {
            lastObservedStepCount = curSteps;
            renderPromptJumpBar();
          } else {
            updateScrollHandler();
          }

          applyDefaultProjectHandler();
          applyOverviewPanelEnhancements();
        });
      });
      if (window.__swissEnhancementsObserver) {
        try { window.__swissEnhancementsObserver.disconnect(); } catch (_) {}
      }
      window.__swissEnhancementsObserver = ob;
      ob.observe(document.body, { childList: true, subtree: true });
    }

    applyEnhancementsStyles();
    renderPromptJumpBar();
    applyDefaultProjectHandler();
    applyOverviewPanelEnhancements();
  } catch (err) {
    console.warn("[SwissKnife] Enhancements script exception:", err);
  }
})();
`, string(cfgJSON))
}
