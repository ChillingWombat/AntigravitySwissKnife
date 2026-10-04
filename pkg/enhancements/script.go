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

    let projectColors = {};

    async function syncConfigFromServer() {
      try {
        if (typeof window !== "undefined" && window.fetch) {
          const res = await fetch("http://127.0.0.1:8765/api/enhancements");
          if (res.ok) {
            const loaded = await res.json();
            if (loaded) {
              enhConfig = loaded;
              applyEnhancementsStyles();
              renderPromptJumpBar();
              applyDefaultProjectHandler();
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

      if (mode === "default") {
        return slateGrey;
      }

      if (mode === "project") {
        const pName = detectActiveProject();
        if (pName && projectColors && projectColors[pName]) {
          return projectColors[pName];
        }
        return slateGrey; // Default to grey if project has no set color
      }

      if (mode === "custom") {
        const c = enhConfig.prompt_jump_bar?.custom_color;
        if (c && c !== "#ec4899" && c.trim() !== "") {
          return c;
        }
        return slateGrey; // Default to grey if custom color is not set
      }

      return slateGrey;
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

      let css = "";
      const densityMode = enhConfig.tool_density_mode || "muted";
      const breakerEnabled = enhConfig.breaker_line_enabled !== false;

      if (densityMode === "muted") {
        css += ` + "`" + `
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
        ` + "`" + `;
      } else if (densityMode === "hidden") {
        css += ` + "`" + `
          [data-testid="worked-for-collapsible"],
          [data-testid="thinking-collapsible-trigger"],
          [data-testid="tool-group-collapsible"],
          [data-testid="run-command-step"],
          [data-testid="subagent-node"],
          .thinking-collapsible {
            display: none !important;
          }
        ` + "`" + `;
      }

      if (breakerEnabled) {
        css += ` + "`" + `
          [data-testid="user-input-step"]:not(:first-child) {
            border-top: 1px solid var(--border, rgba(0, 0, 0, 0.075)) !important;
            margin-top: 24px !important;
            padding-top: 20px !important;
          }
        ` + "`" + `;
      }

      css += ` + "`" + `
        .swiss-prompt-dash {
          transition: width 0.18s cubic-bezier(0.4, 0, 0.2, 1), background 0.15s ease !important;
        }
        .swiss-prompt-dash:hover {
          width: 22px !important;
        }
      ` + "`" + `;

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
        bar.style.cssText = "position: absolute; left: 18px; top: 28px; z-index: 45; display: flex; flex-direction: column; gap: 7px; padding: 4px 2px; background: transparent; border: none; box-shadow: none; user-select: none;";
      } else {
        bar.style.cssText = "position: fixed; left: 268px; top: 120px; z-index: 9999; display: flex; flex-direction: column; gap: 7px; padding: 4px 2px; background: transparent; border: none; box-shadow: none; user-select: none;";
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
        dash.style.cssText = "width: " + dashWidth + "px; height: 1.5px; border-radius: 2px; background: " + defaultColor + "; cursor: pointer; transition: width 0.18s cubic-bezier(0.4, 0, 0.2, 1), background 0.15s ease;";

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
                window.location.href = dest;
              }
            }
          }, true);
        }
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
      const ob = new MutationObserver(() => {
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
        });
      });
      ob.observe(document.body, { childList: true, subtree: true });
    }

    applyEnhancementsStyles();
    renderPromptJumpBar();
    applyDefaultProjectHandler();
  } catch (err) {
    console.warn("[SwissKnife] Enhancements script exception:", err);
  }
})();
`, string(cfgJSON))
}
