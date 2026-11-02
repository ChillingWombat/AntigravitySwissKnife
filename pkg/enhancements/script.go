package enhancements

import (
	"encoding/json"
	"fmt"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/rendererbroker"
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
		cfgJSON = []byte(`{"enabled":true,"tool_density_mode":"muted","breaker_line_enabled":true,"default_new_project":"auto"}`)
	}

	return rendererbroker.FetchScript + fmt.Sprintf(`
/* === Antigravity Swiss Knife: App Enhancements === */
(() => {
  try {
    function getSwissAuthHeaders() {
      return { "Content-Type": "application/json" };
    }

    let enhConfig = %s;
    if (!enhConfig) return;
    function isAutomationTargetProjectEnabled() {
      return enhConfig.automation_target_project_enabled !== false;
    }
    function isContextWindowIndicatorEnabled() {
      return enhConfig.context_window_indicator_enabled !== false;
    }
    function isContextWindowClickToCompactEnabled() {
      return enhConfig.context_window_click_to_compact_enabled !== false;
    }
    if (typeof window !== "undefined") {
      window.__SWISS_ENH_CONFIG__ = enhConfig;
      if (enhConfig.overview_panel && enhConfig.overview_panel.aux_tabs_format) {
        localStorage.setItem("antigravity_swiss_aux_tab_format", enhConfig.overview_panel.aux_tabs_format);
      }
      localStorage.setItem("antigravity_swiss_left_panel_enabled", String(enhConfig.left_panel_extensions_enabled !== false));
      localStorage.setItem("antigravity_swiss_left_panel_mode", enhConfig.left_panel_extensions_mode || "single");
      localStorage.setItem("antigravity_swiss_main_section_enabled", String(enhConfig.main_section_extensions_enabled !== false));
      localStorage.setItem("antigravity_swiss_community_extensions", JSON.stringify(enhConfig.community_extensions || []));
      try {
        const rawFlags = localStorage.getItem("jetski.developer.customFlagOverrides") || "{}";
        const parsedFlags = JSON.parse(rawFlags);
        const targetProjectFlag = !isAutomationTargetProjectEnabled();
        if (parsedFlags && typeof parsedFlags === "object" && !Array.isArray(parsedFlags) && (parsedFlags["remote-control-setting-enabled"] !== true || parsedFlags["jetski-sidecar-declared-permissions"] !== targetProjectFlag || parsedFlags["enable-compact-slash-command"] !== true)) {
          parsedFlags["remote-control-setting-enabled"] = true;
          parsedFlags["jetski-sidecar-declared-permissions"] = targetProjectFlag;
          parsedFlags["enable-compact-slash-command"] = true;
          const nextFlags = JSON.stringify(parsedFlags);
          localStorage.setItem("jetski.developer.customFlagOverrides", nextFlags);
          if (typeof StorageEvent === "function") {
            window.dispatchEvent(new StorageEvent("storage", {
              key: "jetski.developer.customFlagOverrides",
              newValue: nextFlags,
              storageArea: window.localStorage
            }));
          }
        }
      } catch (_) {}
      installManageSidecarInterceptor();
      try {
        if (!window.__swissUserStatusSyncInterval) {
          const syncHostUserStatus = async () => {
            try {
              const root = document.getElementById("root");
              if (!root) return;
              const key = Object.keys(root).find(k => k.startsWith("__reactContainer") || k.startsWith("__reactFiber"));
              let curr = key ? root[key] : null;
              if (curr && curr.current) curr = curr.current;
              const queue = [curr];
              const visited = new Set();
              let store = null, resolver = null;
              while (queue.length > 0 && visited.size < 3500) {
                const node = queue.shift();
                if (!node || visited.has(node)) continue;
                visited.add(node);
                const mp = node.memoizedProps;
                if (mp) {
                  if (!store && mp.store && typeof mp.store.getState === "function") store = mp.store;
                  if (!resolver && typeof mp.agentServiceResolver === "function") resolver = mp.agentServiceResolver;
                }
                if (store && resolver) break;
                if (node.child) queue.push(node.child);
                if (node.sibling) queue.push(node.sibling);
              }
              if (store && resolver) {
                const svc = resolver();
                if (svc && typeof svc.getUserStatus === "function") {
                  const res = await svc.getUserStatus();
                  const next = res && res.userStatus;
                  if (next && next.email) {
                    const cur = store.getState()?.userStatus;
                    if (!cur || cur.email !== next.email || cur.name !== next.name) {
                      store.dispatch({ type: "userStatus/setUserStatus", payload: next });
                    }
                  }
                }
              }
            } catch (_) {}
          };
          window.__swissUserStatusSyncInterval = setInterval(syncHostUserStatus, 3000);
          setTimeout(syncHostUserStatus, 800);
        }
      } catch (_) {}
    }

    let projectColors = {};

    async function syncConfigFromServer() {
      try {
        if (typeof window !== "undefined" && window.fetch) {
          const res = await fetch("http://127.0.0.1:8765/api/enhancements", {
            headers: getSwissAuthHeaders(),
            signal: (typeof AbortSignal !== "undefined" && AbortSignal.timeout) ? AbortSignal.timeout(1500) : undefined
          });
          if (res.ok) {
            if (typeof window.setSwissDaemonOnline === "function") {
              window.setSwissDaemonOnline(true);
            } else {
              window.__swissDaemonOnline = true;
            }
            const loaded = await res.json();
            if (loaded) {
              enhConfig = loaded;
              window.__SWISS_ENH_CONFIG__ = loaded;
              renderContextWindowProgressRing();
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
              const prevExtensions = localStorage.getItem("antigravity_swiss_ext_visibility");
              const prevCommunityExts = localStorage.getItem("antigravity_swiss_community_extensions");
              const newLeftEnabled = String(loaded.left_panel_extensions_enabled !== false);
              const newLeftMode = loaded.left_panel_extensions_mode || "single";
              const newMainSection = String(loaded.main_section_extensions_enabled !== false);
              const newExtensions = JSON.stringify(loaded.extensions || null);
              const newCommunityExts = JSON.stringify(loaded.community_extensions || []);
              if (prevLeftEnabled !== newLeftEnabled || prevLeftMode !== newLeftMode || prevMainSection !== newMainSection || prevExtensions !== newExtensions || prevCommunityExts !== newCommunityExts) {
                localStorage.setItem("antigravity_swiss_left_panel_enabled", newLeftEnabled);
                localStorage.setItem("antigravity_swiss_left_panel_mode", newLeftMode);
                localStorage.setItem("antigravity_swiss_main_section_enabled", newMainSection);
                localStorage.setItem("antigravity_swiss_ext_visibility", newExtensions);
                localStorage.setItem("antigravity_swiss_community_extensions", newCommunityExts);
                window.dispatchEvent(new CustomEvent("swiss-left-nav-config-updated", {
                  detail: { enabled: loaded.left_panel_extensions_enabled !== false, mode: newLeftMode, main_section_enabled: loaded.main_section_extensions_enabled !== false }
                }));
                if (typeof window.setupLeftNavTabs === "function") window.setupLeftNavTabs();
                if (typeof window.setupAuxiliaryTabs === "function") window.setupAuxiliaryTabs();
              }
              applyEnhancementsStyles();
              applyDefaultProjectHandler();
              applyOverviewPanelEnhancements();
              renderContextWindowProgressRing();
            }
          }

          const resGui = await fetch("http://127.0.0.1:8765/api/gui/config", {
            headers: getSwissAuthHeaders(),
            signal: (typeof AbortSignal !== "undefined" && AbortSignal.timeout) ? AbortSignal.timeout(1500) : undefined
          });
          if (resGui.ok) {
            const loadedGui = await resGui.json();
            if (loadedGui) {
              if (loadedGui.project_colors) {
                projectColors = loadedGui.project_colors;
              }
            }
          }
        }
      } catch (_) {}
    }
    if (window.__swissEnhIntervalId) {
      clearInterval(window.__swissEnhIntervalId);
    }
    syncConfigFromServer();
    window.__swissEnhIntervalId = setInterval(() => {
      syncConfigFromServer();
      renderContextWindowProgressRing();
    }, 3000);

    function isDarkMode() {
      return document.documentElement.classList.contains("dark") ||
        document.body.classList.contains("dark") ||
        window.getComputedStyle(document.body).backgroundColor.includes("rgb(30,") ||
        window.getComputedStyle(document.body).backgroundColor.includes("rgb(32,");
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
      let css = `+"`"+`
        [data-swiss-hidden-by-main-stage="true"] {
          display: none !important;
        }
      `+"`"+`;
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
            opacity: 0.5 !important;
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
          position: relative !important;
          transition: width 0.18s cubic-bezier(0.4, 0, 0.2, 1), background 0.15s ease !important;
        }
        .swiss-prompt-dash::before {
          content: "" !important;
          position: absolute !important;
          top: -4px !important;
          bottom: -4px !important;
          left: -6px !important;
          right: -6px !important;
          background: transparent !important;
        }
        .swiss-prompt-dash:hover {
          width: 22px !important;
        }
      `+"`"+`;

      // Overview Panel Styling
      const op = enhConfig.overview_panel;
      if (op && op.enabled) {
        if (op.division_style === "divider_line") {
          const lineCol = dark ? "rgba(255, 255, 255, 0.12)" : "#e2e8f0";
          css += `+"`"+`
            .swiss-overview-parent,
            [data-testid*="overview"] .gap-6,
            [data-aux-pane-open="true"] .gap-6 {
              gap: 5px !important;
            }
            .swiss-overview-parent > div:not(:first-child):not(.swiss-overview-divider),
            .swiss-overview-parent > [data-swiss-overview-section="true"]:not(:first-child),
            [data-testid*="overview"] .gap-6 > div.flex-col:not(:first-child):not(.swiss-overview-divider),
            [data-aux-pane-open="true"] .gap-6 > div.flex-col:not(:first-child):not(.swiss-overview-divider) {
              row-gap: 8px !important;
            }
            .swiss-overview-parent > div:not(:first-child):not(.swiss-overview-divider)::before,
            .swiss-overview-parent > [data-swiss-overview-section="true"]:not(:first-child)::before,
            [data-testid*="overview"] .gap-6 > div.flex-col:not(:first-child):not(.swiss-overview-divider)::before,
            [data-aux-pane-open="true"] .gap-6 > div.flex-col:not(:first-child):not(.swiss-overview-divider)::before {
              content: "" !important;
              display: block !important;
              width: calc(100%% - 12px) !important;
              height: 0px !important;
              margin-top: 0px !important;
              margin-left: 6px !important;
              margin-right: 6px !important;
              margin-bottom: -3px !important;
              border: none !important;
              border-top: 1px solid ${lineCol} !important;
              box-sizing: border-box !important;
            }
            .swiss-overview-divider {
              height: 0px !important;
              min-height: 0px !important;
              max-height: 0px !important;
              padding: 0 !important;
              overflow: hidden !important;
              font-size: 0px !important;
              line-height: 0 !important;
              width: calc(100%% - 12px) !important;
              border: none !important;
              border-top: none !important;
              margin-top: calc(-12px - 7px) !important;
              margin-bottom: calc(-12px - 7px) !important;
              margin-left: 6px !important;
              margin-right: 6px !important;
              display: none !important;
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
              height: 8px !important;
              min-height: 8px !important;
              max-height: 8px !important;
              padding: 0 !important;
              padding-left: 0 !important;
              padding-right: 0 !important;
              margin: -7px auto 0 auto !important;
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
              height: 100%% !important;
              padding: 0 !important;
              padding-left: 0 !important;
              padding-right: 0 !important;
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
              height: 6px !important;
              color: #64748b !important;
              font-size: 8px !important;
              line-height: 1 !important;
              background: transparent !important;
              transition: all 0.18s ease !important;
              padding: 0 !important;
              margin: 0 auto !important;
            }
            .swiss-overview-tabs-triangle {
              display: inline-block !important;
              font-size: 8px !important;
              line-height: 1 !important;
              text-align: center !important;
              vertical-align: middle !important;
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

      css += `+"`"+`
        /* Dual-layer daemon offline suppression */
        html:not(.swiss-daemon-online) #swiss-context-window-ring,
        html.swiss-daemon-offline #swiss-context-window-ring,
        html:not(.swiss-daemon-online) [id^="swiss-context-window-ring-sub-"],
        html.swiss-daemon-offline [id^="swiss-context-window-ring-sub-"],
        html:not(.swiss-daemon-online) .swiss-context-window-ring,
        html.swiss-daemon-offline .swiss-context-window-ring,
        html:not(.swiss-daemon-online) .swiss-context-window-ring-sub,
        html.swiss-daemon-offline .swiss-context-window-ring-sub,
        html:not(.swiss-daemon-online) .swiss-inchat-metrics,
        html.swiss-daemon-offline .swiss-inchat-metrics {
          display: none !important;
        }

        @keyframes swiss-context-ring-spin {
          0%% { transform: rotate(-90deg); }
          100%% { transform: rotate(270deg); }
        }
        .swiss-context-window-ring[data-swiss-compacting="true"] svg {
          animation: swiss-context-ring-spin 0.8s linear infinite !important;
        }
      `+"`"+`;

      if (styleTag.textContent !== css) {
        styleTag.textContent = css;
      }
    }

    // Cleanup any lingering custom chat navigator from prior runs
    try {
      const oldBar = document.getElementById("swiss-prompt-jump-bar");
      if (oldBar) oldBar.remove();
      const oldPanel = document.getElementById("swiss-prompt-navigator-panel");
      if (oldPanel) oldPanel.remove();
      const oldTooltip = document.getElementById("swiss-prompt-tooltip");
      if (oldTooltip) oldTooltip.remove();
    } catch (_) {}


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

    function installManageSidecarInterceptor() {
      if (typeof window === "undefined" || !window.fetch || window.__swissManageSidecarPatched) return;
      window.__swissManageSidecarPatched = true;
      const origFetch = window.fetch.bind(window);
      window.fetch = async function(input, init) {
        try {
          let url = "";
          if (typeof input === "string") {
            url = input;
          } else if (input && typeof input.url === "string") {
            url = input.url;
          }
          if (url && url.includes("/exa.language_server_pb.LanguageServerService/ManageSidecar")) {
            let body = null;
            if (init && init.body) {
              if (typeof init.body === "string") {
                try { body = JSON.parse(init.body); } catch (_) {}
              } else if (typeof init.body === "object") {
                body = init.body;
              }
            } else if (input && typeof input.clone === "function") {
              try { body = await input.clone().json(); } catch (_) {}
            }
            if (body && (body.action === 5 || body.action === "SIDECAR_ACTION_CREATE" || body.action === "5")) {
              if (!isAutomationTargetProjectEnabled()) {
                return origFetch(input, init);
              }
              const chosenProj = document.getElementById("swiss-automation-project-select")?.value || window.__swissSelectedAutomationProjectId || "";
              if (chosenProj && chosenProj !== "__new_dedicated__" && !body.projectId) {
                body.projectId = chosenProj;
                init = init || {};
                init.body = JSON.stringify(body);
              }
              const response = await origFetch(input, init);
              if (response && (response.ok || response.status === 200) && chosenProj && chosenProj !== "__new_dedicated__") {
                try {
                  let resBody = null;
                  try {
                    if (typeof response.clone === "function") {
                      resBody = await response.clone().json();
                    } else if (typeof response.json === "function") {
                      resBody = await response.json();
                    }
                  } catch (_) {}
                  const sidecarId = (resBody && (resBody.sidecarId || (resBody.sidecar && resBody.sidecar.id) || resBody.id)) || body.sidecarId;
                  if (sidecarId) {
                    let followUpHeaders = { "Content-Type": "application/json" };
                    if (init && init.headers) {
                      if (typeof Headers !== "undefined" && init.headers instanceof Headers) {
                        followUpHeaders = new Headers(init.headers);
                        if (!followUpHeaders.has("Content-Type")) followUpHeaders.set("Content-Type", "application/json");
                      } else if (typeof init.headers === "object") {
                        followUpHeaders = Object.assign({}, init.headers, { "Content-Type": "application/json" });
                      }
                    }
                    const targetUrl = (typeof input === "string") ? input : (input && input.url) ? input.url : "/exa.language_server_pb.LanguageServerService/ManageSidecar";
                    await origFetch(targetUrl, {
                      method: "POST",
                      headers: followUpHeaders,
                      body: JSON.stringify({
                        sidecarId: sidecarId,
                        action: 6,
                        projectId: chosenProj
                      })
                    });
                  }
                } catch (_) {}
              }
              return response;
            }
          }
        } catch (_) {}
        return origFetch.apply(window, arguments);
      };
    }

    function extractExistingAntigravityProjects() {
      const projects = [];
      const seen = new Set();
      function add(id, label) {
        if (!id || typeof id !== "string") return;
        const cleanId = id.trim();
        if (!cleanId || cleanId === "outside-of-project" || seen.has(cleanId)) return;
        seen.add(cleanId);
        projects.push({
          id: cleanId,
          label: (label && typeof label === "string" && label.trim()) ? label.trim() : cleanId
        });
      }

      // Method 1: React Fiber tree from #root for sidebarSectionsProvider or sidebarSections
      try {
        const root = document.getElementById("root");
        if (root) {
          const key = Object.keys(root).find(k => k.startsWith("__reactContainer") || k.startsWith("__reactFiber"));
          let curr = key ? root[key] : null;
          if (curr && curr.current) curr = curr.current;
          const queue = [curr];
          const visited = new Set();
          while (queue.length > 0 && visited.size < 3500) {
            const node = queue.shift();
            if (!node || visited.has(node)) continue;
            visited.add(node);
            const mp = node.memoizedProps;
            if (mp) {
              const provider = mp.value?.sidebarSectionsProvider || mp.sidebarSectionsProvider;
              const candidate = (provider && (provider.sidebarSections || provider)) ||
                                mp.value?.sidebarSections ||
                                mp.sidebarSections;
              const sections = Array.isArray(candidate) ? candidate : (candidate && Array.isArray(candidate.sidebarSections) ? candidate.sidebarSections : null);
              if (Array.isArray(sections) && sections.length > 0) {
                for (const s of sections) {
                  if (s && s.sectionKind && s.sectionKind.case === "projectSection" && s.uri && s.uri !== "outside-of-project") {
                    add(s.uri, s.label || s.uri);
                  }
                }
                if (projects.length > 0) return projects;
              }
            }
            if (node.child) queue.push(node.child);
            if (node.sibling) queue.push(node.sibling);
          }
        }
      } catch (_) {}

      // Method 2: Fallback to tec modal fiber memoizedProps?.projects
      try {
        const nameInput = document.querySelector('[data-testid="new-sidecar-name"]');
        if (nameInput) {
          let currEl = nameInput;
          while (currEl && currEl !== document.body) {
            const key = Object.keys(currEl).find(k => k.startsWith("__reactFiber"));
            let fiber = key ? currEl[key] : null;
            let depth = 0;
            while (fiber && depth < 20) {
              const mp = fiber.memoizedProps;
              if (mp && mp.projects && Array.isArray(mp.projects)) {
                for (const p of mp.projects) {
                  if (p) {
                    const id = p.id || p.uri || p.projectId || p.name;
                    const label = p.label || p.name || p.title || id;
                    if (id) add(id, label);
                  }
                }
                if (projects.length > 0) return projects;
              }
              fiber = fiber.return;
              depth++;
            }
            currEl = currEl.parentElement;
          }
        }
      } catch (_) {}

      // Method 3: Fallback to scanning project items in left sidebar DOM (Projects section rows)
      try {
        const links = document.querySelectorAll('a[aria-label="New Conversation in Project"]');
        for (const l of links) {
          const href = l.getAttribute("href") || "";
          const m = href.match(/section=([a-zA-Z0-9_-]+)/);
          const sectionId = m ? m[1] : "";
          const header = l.closest ? l.closest('.group\\/header, [data-project-card], [data-index]') : null;
          const label = header ? (header.textContent || "").replace(/New Conversation.*/i, "").trim() : "";
          if (sectionId && sectionId !== "outside-of-project") {
            add(sectionId, label || sectionId);
          }
        }

        const rows = document.querySelectorAll('[data-testid="conversation-list-sidebar"] [data-project-card], [data-testid="conversation-list-sidebar"] .group\\/header');
        for (const r of rows) {
          const pid = r.getAttribute("data-project-id") || r.getAttribute("data-uri");
          const label = (r.textContent || "").replace(/New Conversation.*/i, "").trim();
          if (pid) add(pid, label || pid);
        }

        const swissEls = document.querySelectorAll('[data-swiss-project]');
        for (const el of swissEls) {
          const p = el.getAttribute("data-swiss-project");
          if (p) add(p, p);
        }
      } catch (_) {}

      return projects;
    }

    function applyAutomationTargetProjectSelector() {
      if (typeof document === "undefined") return;
      if (!isAutomationTargetProjectEnabled()) {
        document.getElementById("swiss-automation-project-field")?.remove();
        return;
      }
      const nameInput = document.querySelector('[data-testid="new-sidecar-name"]');
      if (!nameInput) return;

      const modal = (nameInput.closest && (nameInput.closest('[role="dialog"]') || nameInput.closest('.tec') || nameInput.closest('form'))) ||
                    (nameInput.parentElement && nameInput.parentElement.parentElement);

      if (modal && modal.querySelectorAll) {
        const labels = Array.from(modal.querySelectorAll("label, .text-sm, [class*='label']"));
        const hasNativeProject = labels.some(el => {
          if (el.id === "swiss-automation-project-field" || (el.closest && el.closest("#swiss-automation-project-field"))) return false;
          const text = (el.textContent || "").trim();
          return text === "Project" || text === "Project:" || (text.includes("Project") && !text.includes("Target Project"));
        });
        if (hasNativeProject) return;
      }

      if (document.getElementById("swiss-automation-project-field")) return;

      const nameContainer = (nameInput.closest && nameInput.closest('.flex.flex-col.gap-1')) || nameInput.parentElement;
      const parent = nameContainer && (nameContainer.parentNode || nameContainer.parentElement);
      if (!nameContainer || !parent) return;

      const projects = extractExistingAntigravityProjects();

      const fieldDiv = document.createElement("div");
      fieldDiv.id = "swiss-automation-project-field";
      fieldDiv.className = "flex flex-col gap-1";

      const labelEl = document.createElement("label");
      labelEl.className = "text-sm font-medium select-none text-left";
      labelEl.textContent = "Target Project";
      fieldDiv.appendChild(labelEl);

      const selectEl = document.createElement("select");
      selectEl.id = "swiss-automation-project-select";
      selectEl.setAttribute("data-testid", "swiss-automation-project-select");
      selectEl.className = "w-full rounded-lg border border-border bg-transparent px-2.5 h-8 text-sm text-foreground cursor-pointer";

      if (projects.length > 0) {
        projects.forEach((proj, idx) => {
          const opt = document.createElement("option");
          opt.value = proj.id;
          opt.textContent = proj.label || proj.id;
          if (idx === 0) {
            opt.selected = true;
            selectEl.value = proj.id;
          }
          selectEl.appendChild(opt);
        });
      } else {
        selectEl.value = "__new_dedicated__";
      }

      const newDedicatedOpt = document.createElement("option");
      newDedicatedOpt.value = "__new_dedicated__";
      newDedicatedOpt.textContent = "Create New Dedicated Project";
      if (projects.length === 0) {
        newDedicatedOpt.selected = true;
      }
      selectEl.appendChild(newDedicatedOpt);
      fieldDiv.appendChild(selectEl);

      const initialVal = selectEl.value;
      if (typeof window !== "undefined") {
        window.__swissSelectedAutomationProjectId = (initialVal && initialVal !== "__new_dedicated__") ? initialVal : "";
      }

      function syncModalFiber(val) {
        try {
          const targetVal = (val === "__new_dedicated__") ? "" : val;
          let curr = nameInput;
          while (curr && curr !== document.body) {
            const key = Object.keys(curr).find(k => k.startsWith("__reactFiber"));
            let fiber = key ? curr[key] : null;
            let depth = 0;
            while (fiber && depth < 25) {
              const mp = fiber.memoizedProps;
              if (mp) {
                if (typeof mp.onProjectIdChange === "function") {
                  try { mp.onProjectIdChange(targetVal); } catch (_) {}
                }
                if (typeof mp.setProjectId === "function") {
                  try { mp.setProjectId(targetVal); } catch (_) {}
                }
                if (typeof mp.onChangeProjectId === "function") {
                  try { mp.onChangeProjectId(targetVal); } catch (_) {}
                }
              }
              let hook = fiber.memoizedState;
              while (hook) {
                if (hook.queue && typeof hook.queue.dispatch === "function" && hook.memoizedState !== undefined) {
                  if (typeof hook.memoizedState === "string" && (hook.memoizedState === "" || projects.some(p => p.id === hook.memoizedState))) {
                    try { hook.queue.dispatch(targetVal); } catch (_) {}
                  }
                }
                hook = hook.next;
              }
              fiber = fiber.return;
              depth++;
            }
            curr = curr.parentElement;
          }
        } catch (_) {}
      }

      syncModalFiber(initialVal);

      selectEl.addEventListener("change", (e) => {
        const val = e.target.value;
        if (typeof window !== "undefined") {
          window.__swissSelectedAutomationProjectId = (val && val !== "__new_dedicated__") ? val : "";
        }
        syncModalFiber(val);
      });

      if (typeof nameContainer.insertAdjacentElement === "function") {
        nameContainer.insertAdjacentElement("afterend", fieldDiv);
      } else if (nameContainer.nextSibling) {
        parent.insertBefore(fieldDiv, nameContainer.nextSibling);
      } else {
        parent.appendChild(fieldDiv);
      }
    }

    if (typeof window !== "undefined") {
      window.extractExistingAntigravityProjects = extractExistingAntigravityProjects;
      window.applyAutomationTargetProjectSelector = applyAutomationTargetProjectSelector;
      window.installManageSidecarInterceptor = installManageSidecarInterceptor;
    }

    function isMainStageActive() {
      if (typeof window !== "undefined") {
        if (typeof window.isMainStageActive === "function" && window.isMainStageActive !== isMainStageActive) {
          try { return Boolean(window.isMainStageActive()); } catch (_) {}
        }
        if (typeof window.isSwissMainStageActive === "function" && window.isSwissMainStageActive !== isMainStageActive) {
          try { return Boolean(window.isSwissMainStageActive()); } catch (_) {}
        }
      }
      const stage = (typeof document !== "undefined") ? (document.getElementById("swiss-main-stage") || document.getElementById("swiss-main-stage-container")) : null;
      if (stage) {
        if (stage.style && stage.style.display === "none") return false;
        if (stage.style && stage.style.display && stage.style.display !== "none") return true;
        if (typeof window !== "undefined" && window.getComputedStyle) {
          const cs = window.getComputedStyle(stage);
          if (cs && cs.display === "none") return false;
          if (cs && cs.display) return true;
        }
        return Boolean(stage.offsetParent !== null || (stage.offsetWidth || 0) > 0 || (stage.offsetHeight || 0) > 0);
      }
      if (typeof document !== "undefined" && document.body) {
        const stageAttr = document.body.getAttribute("data-swiss-stage-active");
        if (stageAttr && stageAttr !== "false") return true;
      }
      if (typeof window !== "undefined" && window.__swissActiveMainStageExt) return true;
      return false;
    }

    function findOverviewPanelElements(root) {
      const doc = root || (typeof document !== "undefined" ? document : null);
      if (!doc || !doc.querySelectorAll) return [];
      const results = [];
      const seen = new Set();
      function add(el) {
        if (!el || seen.has(el)) return;
        if (el.id === "swiss-main-stage" || el.id === "swiss-main-stage-container" || el.id === "swiss-left-nav-group") return;
        if (el.closest && el.closest("#swiss-main-stage, #swiss-main-stage-container, #swiss-left-nav-group")) return;
        seen.add(el);
        results.push(el);
      }

      const directSelectors = [
        "[data-swiss-overview-panel]",
        "[data-testid=\"overview-panel-host-overlay\"]",
        "[data-testid=\"overview-panel-overlay-backdrop\"]",
        "[data-testid=\"overview-panel\"]",
        "[data-testid=\"auxiliary-panel\"]",
        "[data-swiss-aux-panel]",
        ".part.auxiliarybar"
      ];
      directSelectors.forEach(sel => {
        try {
          doc.querySelectorAll(sel).forEach(add);
        } catch (_) {}
      });

      const candidateSelectors = [
        "dialog",
        "[role=\"dialog\"]",
        "[role=\"complementary\"]",
        "aside",
        "[class*=\"overview-panel\"]",
        ".animate-overview-panel",
        ".shadow-xl"
      ];
      candidateSelectors.forEach(sel => {
        try {
          doc.querySelectorAll(sel).forEach(el => {
            const text = el.textContent || "";
            const ariaLabel = (el.getAttribute && el.getAttribute("aria-label")) || "";
            const testId = (el.getAttribute && el.getAttribute("data-testid")) || "";
            const isOverviewAria = ariaLabel.toLowerCase().includes("overview");
            const isOverviewTestId = testId.toLowerCase().includes("overview");
            const hasOverviewTabs = text.includes("Overview") &&
              (text.includes("Review") || text.includes("Files") || text.includes("Terminal") || text.includes("Plugins"));
            if (isOverviewAria || isOverviewTestId || hasOverviewTabs) {
              add(el);
            }
          });
        } catch (_) {}
      });

      try {
        const rowIndicators = doc.querySelectorAll("[data-testid*=\"overview-panel-row\"], button[data-testid*=\"overview-panel\"]");
        rowIndicators.forEach(row => {
          const container = (row.closest && row.closest("[role=\"dialog\"], [data-testid*=\"overview-panel\"], aside, [data-swiss-overview-panel], .shadow-xl")) ||
                            row.parentElement;
          if (container) add(container);
        });
      } catch (_) {}

      try {
        const allTabs = Array.from(doc.querySelectorAll("button, [role=\"tab\"], span, div")).filter(n => {
          const t = (n.textContent || "").trim();
          return t === "Overview" || t === "Review" || t === "Terminal" || t === "Files" || t === "Plugins";
        });
        if (allTabs.length >= 3) {
          const container = (allTabs[0].closest && allTabs[0].closest("[role=\"dialog\"], aside, [class*=\"card\"], [class*=\"popover\"], div.shadow-xl, div[class*=\"border\"]")) ||
                            (allTabs[0].parentElement && allTabs[0].parentElement.parentElement);
          if (container) add(container);
        }
      } catch (_) {}

      return results;
    }

    function findOverviewPanel(root) {
      const els = findOverviewPanelElements(root);
      return els.length > 0 ? els[0] : null;
    }

    function hideOverviewPanelForMainStage() {
      const elements = findOverviewPanelElements();
      elements.forEach(el => {
        if (!el) return;
        if (el.getAttribute("data-swiss-hidden-by-main-stage") !== "true") {
          el.setAttribute("data-swiss-hidden-by-main-stage", "true");
          if (el.__swissPrevDisplay === undefined) {
            el.__swissPrevDisplay = (el.style && el.style.display) ? el.style.display : "";
          }
          if (el.style && typeof el.style.setProperty === "function") {
            el.style.setProperty("display", "none", "important");
          } else if (el.style) {
            el.style.display = "none";
          }
        }
      });
    }

    function restoreOverviewPanelFromMainStage() {
      const hiddenEls = (typeof document !== "undefined" && document.querySelectorAll)
        ? Array.from(document.querySelectorAll("[data-swiss-hidden-by-main-stage=\"true\"]"))
        : [];
      hiddenEls.forEach(el => {
        if (!el) return;
        el.removeAttribute("data-swiss-hidden-by-main-stage");
        if (el.__swissPrevDisplay !== undefined) {
          if (el.__swissPrevDisplay && el.__swissPrevDisplay !== "none") {
            if (el.style && typeof el.style.setProperty === "function") {
              el.style.setProperty("display", el.__swissPrevDisplay);
            } else if (el.style) {
              el.style.display = el.__swissPrevDisplay;
            }
          } else {
            if (el.style && typeof el.style.removeProperty === "function") {
              el.style.removeProperty("display");
            }
            if (el.style) {
              el.style.display = "";
            }
          }
          delete el.__swissPrevDisplay;
        } else {
          if (el.style && typeof el.style.removeProperty === "function") {
            el.style.removeProperty("display");
          }
          if (el.style) {
            el.style.display = "";
          }
        }
      });
    }

    if (typeof window !== "undefined") {
      window.isMainStageActive = isMainStageActive;
      window.isSwissMainStageActive = isMainStageActive;
      window.findOverviewPanel = findOverviewPanel;
      window.findOverviewPanelElements = findOverviewPanelElements;
      window.hideOverviewPanelForMainStage = hideOverviewPanelForMainStage;
      window.restoreOverviewPanelFromMainStage = restoreOverviewPanelFromMainStage;
    }

    function applyOverviewPanelEnhancements() {
      if (isMainStageActive()) {
        hideOverviewPanelForMainStage();
        return;
      }
      restoreOverviewPanelFromMainStage();
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
      // Unconditionally remove any foreign swiss-overview-divider DOM nodes to prevent duplicate or stacked divider lines
      document.querySelectorAll(".swiss-overview-divider").forEach(el => el.remove());

      const op = enhConfig.overview_panel;
      if (!op || !op.enabled) {
        document.querySelectorAll(".swiss-overview-zone").forEach(el => el.classList.remove("swiss-overview-zone"));
        document.querySelectorAll(".swiss-overview-divider").forEach(el => el.remove());
        document.querySelectorAll(".swiss-overview-bottom-spacer").forEach(el => el.remove());
        document.querySelectorAll('[data-swiss-overview-section="true"]').forEach(el => el.removeAttribute("data-swiss-overview-section"));
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
        // Never style terminal scope selector, terminal containers, or chat markdown blocks: preserve factory style
        const elText = (el.textContent || "").trim();
        if (elText.toLowerCase().includes("terminal") ||
            el.closest('[data-tab-id="terminal"]') ||
            el.closest('.terminal-view') ||
            el.closest('[data-testid="autoscroll-viewport"]') ||
            el.closest('[data-testid="user-input-step"]') ||
            el.closest('.md-divider-spacing') ||
            el.closest('[data-index]') ||
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

      // Ensure all foreign swiss-overview-divider nodes are removed (including orphaned dividers before the first section where sectionContainers.indexOf(next) === 0)
      document.querySelectorAll(".swiss-overview-divider").forEach(div => {
        const next = div.nextElementSibling;
        if (!next || !sectionContainers.includes(next) || sectionContainers.indexOf(next) === 0) {
          div.remove();
        } else {
          div.remove();
        }
      });

      document.querySelectorAll('[data-swiss-overview-section="true"]').forEach(el => {
        if (!sectionContainers.includes(el)) {
          el.removeAttribute("data-swiss-overview-section");
        }
      });

      document.querySelectorAll(".swiss-overview-parent").forEach(el => {
        if (!sectionContainers.some(c => c.parentElement === el)) {
          el.classList.remove("swiss-overview-parent");
          el.style.removeProperty("gap");
        }
      });

      sectionContainers.forEach((container, idx) => {
        const parent = container.parentElement;
        if (parent && !parent.classList.contains("swiss-overview-parent")) {
          parent.classList.add("swiss-overview-parent");
        }
        if (container.getAttribute("data-swiss-overview-section") !== "true") {
          container.setAttribute("data-swiss-overview-section", "true");
        }
        if (op.division_style === "border_zone") {
          if (!container.classList.contains("swiss-overview-zone")) {
            container.classList.add("swiss-overview-zone");
          }
          if (parent) {
            parent.style.setProperty("gap", "0px", "important");
          }
        } else if (op.division_style === "divider_line") {
          // Divider lines are rendered via pure CSS ::before on section containers without inserting foreign DOM siblings, preventing React unmounting/flashing loops
          const desiredHalfGap = 5;
          const netMargin = desiredHalfGap;
          // Retain helper contract reference:
          // divider.style.setProperty("margin-left", "6px", "important");
          // divider.style.setProperty("width", "calc(100%% - 12px)", "important");
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
              requestAnimationFrame(() => {
                isEnhancingOverview = true;
                try {
                  applyOverviewPanelEnhancements();
                } finally {
                  setTimeout(() => { isEnhancingOverview = false; }, 16);
                }
              });
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

    // Attach scroll and DOM observers (disconnecting any stale observer on live re-injection)
    let isEnhancingOverview = false;
    let scheduledRaf = null;

    const updateScrollHandler = () => {};
    const handleUserInteraction = () => {};

    const handleKeydown = (e) => {
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
    };

    window.__swissUpdateScrollHandler = updateScrollHandler;
    window.__swissHandleUserInteraction = handleUserInteraction;
    window.__swissHandleKeydown = handleKeydown;

    if (!window.__swissEnhancementsListenersBound) {
      window.__swissEnhancementsListenersBound = true;
      const vp = document.querySelector('[data-testid="autoscroll-viewport"]') || document.querySelector('.overflow-y-auto');
      if (vp) {
        vp.addEventListener("scroll", () => window.__swissUpdateScrollHandler && window.__swissUpdateScrollHandler(), { passive: true });
        vp.addEventListener("wheel", () => window.__swissHandleUserInteraction && window.__swissHandleUserInteraction(), { passive: true });
        vp.addEventListener("touchmove", () => window.__swissHandleUserInteraction && window.__swissHandleUserInteraction(), { passive: true });
      }

      window.addEventListener("scroll", () => window.__swissUpdateScrollHandler && window.__swissUpdateScrollHandler(), { passive: true, capture: true });
      window.addEventListener("wheel", () => window.__swissHandleUserInteraction && window.__swissHandleUserInteraction(), { passive: true });
      window.addEventListener("touchmove", () => window.__swissHandleUserInteraction && window.__swissHandleUserInteraction(), { passive: true });
      window.addEventListener("keydown", (e) => window.__swissHandleKeydown && window.__swissHandleKeydown(e), { passive: false });

    }

    if (window.__swissEnhancementsObserver && typeof window.__swissEnhancementsObserver.disconnect === "function") {
      try { window.__swissEnhancementsObserver.disconnect(); } catch (_) {}
    }

    /* === Active Conversation Auto-Revival & Continuation Handler === */
    /* __SWISS_CONVERSATION_IDENTITY_START__ */
    const SWISS_CONVERSATION_ID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
    const SWISS_EXPLICIT_NAVIGATION_SELECTOR = '[data-testid="new-conversation-button"], [data-testid="app-icon-new-conversation-button"], [data-testid="conversation-row-sidebar"], a[href*="/c/"]';
    const SWISS_NEW_CONVERSATION_SELECTOR = '[data-testid="new-conversation-button"], [data-testid="app-icon-new-conversation-button"]';
    const SWISS_EXPLICIT_NAVIGATION_STORAGE_KEY = "antigravity_swiss_explicit_navigation";
    const SWISS_CONVERSATION_METRIC_RESET_EPOCH_KEY = "__swissConversationMetricResetEpoch";
    const SWISS_CONVERSATION_ROUTE_SYNC_MAX_RETRIES = 3;
    const SWISS_CONVERSATION_ROUTE_SYNC_RETRY_MS = 250;

    function isConversationId(value) {
      return typeof value === "string" && SWISS_CONVERSATION_ID_RE.test(value);
    }

    function parseConversationId(pathname) {
      const match = String(pathname || "").match(/^\/c\/([^/?#]+)/);
      return match && isConversationId(match[1]) ? match[1] : null;
    }

    function resolveConversationIdentity(visibleId, pathname) {
      return isConversationId(visibleId) ? visibleId : parseConversationId(pathname);
    }

    function createConversationMetricIdentityState() {
      return {
        conversationId: null,
        phase: "loading",
        lastRenderedId: null,
        routeFallbackBlocked: false
      };
    }

    function transitionConversationMetricIdentity(identity, requestGenerations, visibleId, pathname, renderedViewPresent, routeFallbackAllowed) {
      const renderedId = isConversationId(visibleId) ? visibleId : null;
      if (renderedId) {
        identity.phase = "rendered";
        identity.lastRenderedId = renderedId;
        identity.routeFallbackBlocked = false;
        return activateConversationIdentity(identity, requestGenerations, renderedId);
      }
      if (renderedViewPresent) {
        identity.phase = "blank";
        identity.routeFallbackBlocked = true;
        return activateConversationIdentity(identity, requestGenerations, null);
      }
      if (identity.phase === "rendered" && isConversationId(identity.lastRenderedId)) {
        return activateConversationIdentity(identity, requestGenerations, identity.lastRenderedId);
      }
      if (routeFallbackAllowed === false) {
        identity.routeFallbackBlocked = true;
      }
      if (identity.routeFallbackBlocked) {
        return activateConversationIdentity(identity, requestGenerations, null);
      }
      identity.phase = "loading";
      return activateConversationIdentity(identity, requestGenerations, parseConversationId(pathname));
    }

    function explicitConversationNavigationFencesRenderedRoute(intent, location, visibleId) {
      if (!isExplicitConversationNavigationActive(intent, location) || !isConversationId(visibleId)) return false;
      const targetId = intent.targetConversationId;
      if (isConversationId(targetId)) return targetId !== visibleId;
      if (intent.kind === "new_conversation") {
        const sourceId = parseConversationId(intent.sourceRoute);
        return !isConversationId(sourceId) || visibleId === sourceId;
      }
      return true;
    }

    function replaceConversationRouteInHistory(conversationId, location, history) {
      if (!isConversationId(conversationId) || !history || typeof history.replaceState !== "function") {
        return false;
      }
      try {
        const targetPath = conversationRouteTargetPath(conversationId, location);
        history.replaceState(history.state, "", targetPath);
        return true;
      } catch (_) {
        return false;
      }
    }

    function conversationRouteTargetPath(visibleId, location) {
      return "/c/" + visibleId +
        ((location && location.search) || "") +
        ((location && location.hash) || "");
    }

    function createConversationRouteSyncState() {
      return {
        semanticKey: "",
        generation: 0,
        timerId: null,
        retries: 0,
        outcome: "idle"
      };
    }

    function createConversationRouteSyncSemanticKey(visibleId, currentRoute, targetPath, intent) {
      const navigation = intent || {};
      return JSON.stringify({
        visibleId: visibleId,
        currentRoute: currentRoute,
        sourceRoute: navigation.sourceRoute || "",
        targetPath: targetPath,
        intentKind: navigation.kind || "",
        intentTargetRoute: navigation.targetRoute || "",
        intentTargetConversationId: navigation.targetConversationId || "",
        intentPending: Boolean(navigation.pending)
      });
    }

    function conversationRouteMatchesVisible(location, visibleId) {
      return parseConversationId(routeKeyForLocation(location)) === visibleId;
    }

    function cancelConversationRouteSyncTimer(state) {
      const timerId = state.timerId;
      state.timerId = null;
      if (timerId === null || timerId === undefined) return;
      try {
        clearTimeout(timerId);
      } catch (_) {}
    }

    function retireConversationRouteSync(state, outcome, keepSemanticKey) {
      state.generation += 1;
      cancelConversationRouteSyncTimer(state);
      if (!keepSemanticKey) state.semanticKey = "";
      state.retries = 0;
      state.outcome = outcome;
    }

    function attemptConversationRouteSynchronization(visibleId, location) {
      const targetPath = conversationRouteTargetPath(visibleId, location);
      const router = window.__TSR_ROUTER__;
      if (router && typeof router.navigate === "function") {
        try {
          router.navigate({ to: targetPath });
        } catch (_) {}
        return "navigate";
      }
      // The rendered conversation is authoritative, so mirroring its route in
      // host history is safe as a fallback. Never reload or emit a fake popstate.
      if (replaceConversationRouteInHistory(visibleId, location, window.history)) {
        return "replaceState";
      }
      return "none";
    }

    function isCurrentConversationRouteSyncAttempt(state, generation, semanticKey, visibleId) {
      if (state.generation !== generation || state.semanticKey !== semanticKey) return false;
      const location = window.location;
      if (conversationRouteMatchesVisible(location, visibleId)) return true;
      const intent = window.__swissExplicitConversationNavigation;
      if (explicitConversationNavigationFencesRenderedRoute(intent, location, visibleId)) return false;
      const currentRoute = routeKeyForLocation(location);
      const targetPath = conversationRouteTargetPath(visibleId, location);
      return createConversationRouteSyncSemanticKey(visibleId, currentRoute, targetPath, intent) === semanticKey;
    }

    function scheduleConversationRouteSyncRetry(state, visibleId, semanticKey) {
      const generation = state.generation + 1;
      state.generation = generation;
      state.outcome = "retrying";
      try {
        state.timerId = setTimeout(() => {
          if (!isCurrentConversationRouteSyncAttempt(state, generation, semanticKey, visibleId)) return;
          if (conversationRouteMatchesVisible(window.location, visibleId)) {
            retireConversationRouteSync(state, "navigate", false);
            return;
          }
          state.timerId = null;
          state.retries += 1;
          const location = window.location;
          const outcome = attemptConversationRouteSynchronization(visibleId, location);
          if (conversationRouteMatchesVisible(location, visibleId)) {
            retireConversationRouteSync(state, "navigate", false);
            return;
          }
          if (state.retries < SWISS_CONVERSATION_ROUTE_SYNC_MAX_RETRIES) {
            scheduleConversationRouteSyncRetry(state, visibleId, semanticKey);
            return;
          }
          const fallback = replaceConversationRouteInHistory(visibleId, location, window.history) ? "replaceState" : "none";
          retireConversationRouteSync(state, fallback, true);
        }, SWISS_CONVERSATION_ROUTE_SYNC_RETRY_MS);
      } catch (_) {
        state.timerId = null;
        const fallback = replaceConversationRouteInHistory(visibleId, window.location, window.history) ? "replaceState" : "none";
        retireConversationRouteSync(state, fallback, true);
      }
    }

    function beginConversationRouteSynchronization(state, visibleId, location, explicitIntent, semanticKey) {
      retireConversationRouteSync(state, "idle", false);
      state.semanticKey = semanticKey;
      const outcome = attemptConversationRouteSynchronization(visibleId, location);
      state.outcome = outcome;
      if (outcome === "navigate" && !conversationRouteMatchesVisible(location, visibleId)) {
        scheduleConversationRouteSyncRetry(state, visibleId, semanticKey);
      }
      return outcome;
    }

    function synchronizeRenderedConversationRoute(state, visibleId, location, explicitIntent) {
      if (!state) return "none";
      if (!isConversationId(visibleId) || explicitConversationNavigationFencesRenderedRoute(explicitIntent, location, visibleId)) {
        retireConversationRouteSync(state, "idle", false);
        return "none";
      }
      if (conversationRouteMatchesVisible(location, visibleId)) {
        retireConversationRouteSync(state, "idle", false);
        return "none";
      }
      const currentRoute = routeKeyForLocation(location);
      const targetPath = conversationRouteTargetPath(visibleId, location);
      const semanticKey = createConversationRouteSyncSemanticKey(visibleId, currentRoute, targetPath, explicitIntent);
      if (state.semanticKey === semanticKey) return state.outcome;
      return beginConversationRouteSynchronization(state, visibleId, location, explicitIntent, semanticKey);
    }

    function beginConversationMetricRequest(requestGenerations, cacheKey) {
      const generation = (requestGenerations[cacheKey] || 0) + 1;
      requestGenerations[cacheKey] = generation;
      return generation;
    }

    function isCurrentConversationMetricRequest(requestGenerations, cacheKey, generation) {
      return Boolean(cacheKey) && requestGenerations[cacheKey] === generation;
    }

    function clearConversationMetricCache(cachedTokens, cachedWindows, convId) {
      if (!convId) return;
      delete cachedTokens[convId];
      delete cachedWindows[convId];
    }

    function clearConversationMetricInstanceState() {
      conversationMetricRequestGenerations = {};
      cachedTokensByConv = {};
      cachedContextWindowByConv = {};
      lastTokensFetchTimeByConv = {};
      subagentNameToConvId = {};
    }

    function synchronizeConversationMetricResetEpoch() {
      const currentEpoch = Number(window[SWISS_CONVERSATION_METRIC_RESET_EPOCH_KEY]) || 0;
      if (currentEpoch === conversationMetricResetEpochSeen) return false;

      const renderedId = getVisibleConversationId(document.querySelector('[data-testid="conversation-view"]'));
      conversationMetricResetStaleId = conversationMetricIdentity.conversationId ||
        (isConversationId(renderedId) ? renderedId : null);
      conversationMetricResetEpochSeen = currentEpoch;
      conversationMetricIdentity = createConversationMetricIdentityState();
      conversationMetricIdentity.phase = "blank";
      conversationMetricIdentity.routeFallbackBlocked = true;
      clearConversationMetricInstanceState();
      removeContextWindowProgressRings();
      return true;
    }

    function activateConversationIdentity(identity, requestGenerations, convId) {
      const nextId = isConversationId(convId) ? convId : null;
      if (identity.conversationId && identity.conversationId !== nextId) {
        beginConversationMetricRequest(requestGenerations, identity.conversationId);
      }
      identity.conversationId = nextId;
      return nextId;
    }

    function isExplicitConversationNavigationElement(element) {
      return Boolean(element && typeof element.closest === "function" && element.closest(SWISS_EXPLICIT_NAVIGATION_SELECTOR));
    }

    function routeKeyFromHref(href) {
      try {
        const parsed = new URL(String(href || ""), "http://127.0.0.1");
        return parsed.pathname + parsed.search;
      } catch (_) {
        return "";
      }
    }

    function routeKeyForLocation(location) {
      const fromHref = location && typeof location.href === "string" ? routeKeyFromHref(location.href) : "";
      return fromHref || ((location && location.pathname) || "/") + ((location && location.search) || "");
    }

    function createExplicitConversationNavigationIntent(element, location, now) {
      if (!isExplicitConversationNavigationElement(element)) return null;
      const anchor = typeof element.closest === "function" ? element.closest("a[href]") : null;
      const targetHref = anchor ? (anchor.href || anchor.getAttribute("href") || "") : "";
      const targetRoute = targetHref ? routeKeyFromHref(targetHref) : "";
      const newConversation = Boolean(typeof element.closest === "function" && element.closest(SWISS_NEW_CONVERSATION_SELECTOR));
      return {
        version: 2,
        kind: newConversation ? "new_conversation" : "conversation",
        at: now,
        sourceRoute: routeKeyForLocation(location),
        targetRoute: targetRoute,
        targetConversationId: parseConversationId(targetRoute),
        route: "",
        conversationId: null,
        pending: true
      };
    }

    function createExplicitHistoryNavigationIntent(location, now) {
      const route = routeKeyForLocation(location);
      return {
        version: 2,
        kind: "history",
        at: now,
        sourceRoute: route,
        targetRoute: route,
        targetConversationId: parseConversationId(route),
        route: route,
        conversationId: parseConversationId(route),
        pending: false
      };
    }

    function bindExplicitConversationNavigationIntent(intent, location) {
      if (!intent || !intent.pending) return false;
      const currentRoute = routeKeyForLocation(location);
      const currentConversationId = parseConversationId(currentRoute);
      const targetReached = intent.targetConversationId
        ? currentConversationId === intent.targetConversationId
        : intent.targetRoute
          ? currentRoute === intent.targetRoute
          : intent.kind === "new_conversation" && currentRoute !== intent.sourceRoute;
      if (!targetReached) return false;
      intent.route = currentRoute;
      intent.conversationId = currentConversationId;
      intent.pending = false;
      return true;
    }

    function isExplicitConversationNavigationActive(intent, location) {
      if (!intent) return false;
      if (intent.pending) return true;
      return Boolean(intent.route) && intent.route === routeKeyForLocation(location);
    }

    function persistExplicitConversationNavigationIntent(intent) {
      if (!intent) return;
      try {
        sessionStorage.setItem(SWISS_EXPLICIT_NAVIGATION_STORAGE_KEY, JSON.stringify(intent));
      } catch (_) {}
    }

    function loadExplicitConversationNavigationIntent() {
      try {
        const raw = sessionStorage.getItem(SWISS_EXPLICIT_NAVIGATION_STORAGE_KEY);
        if (!raw) return null;
        const intent = JSON.parse(raw);
        return intent && intent.version === 2 ? intent : null;
      } catch (_) {
        return null;
      }
    }
    /* __SWISS_CONVERSATION_IDENTITY_END__ */

    let autoRevivalInFlight = false;
    let lastRevivalCheckTime = 0;

    async function ackContinuation(cascadeId, intentKey) {
      try {
        localStorage.removeItem("antigravity_swiss_pending_continuation");
        if (typeof window !== "undefined") {
          // Ack by intent ID so the daemon clears exactly the intent that was
          // dispatched and leaves the other interrupted conversations queued.
          if (intentKey) {
            window.postMessage({ type: "swiss-clear-pending-continuation", cascade_id: cascadeId, intent_id: intentKey }, "*");
          }
          const listed = Array.isArray(window.__swissPendingContinuations) ? window.__swissPendingContinuations : [];
          const remaining = listed.filter(it => {
            if (!it) return false;
            if (intentKey && it.intent_id === intentKey) return false;
            if (!intentKey) return (it.root_conversation_id || it.cascade_id) !== cascadeId;
            return true;
          });
          window.__swissPendingContinuations = remaining;
          window.__swissPendingContinuation = remaining.length > 0 ? remaining[0] : null;
        }
        if (typeof window !== "undefined" && window.fetch) {
          await fetch("http://127.0.0.1:8765/api/conversations/ack", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ cascade_id: cascadeId, intent_id: intentKey || "" })
          });
        }
      } catch (_) {}
    }


    function checkAndRecoverFromNonexistentConversation() {
      const curPath = window.location.pathname || "/";
      if (!curPath.startsWith("/c/")) return;
      const curID = curPath.replace(/^\/c\//, "").split(/[?#]/)[0];
      const uuidRegex = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

      // When on a valid UUID conversation with editor or view, pin it as active
      if (uuidRegex.test(curID)) {
        const hasEditorOrView = Boolean(document.querySelector('[data-testid="conversation-view"], [data-testid="agent-input-box"], textarea'));
        if (hasEditorOrView) {
          try { localStorage.setItem("antigravity_swiss_last_conversation_path", "/c/" + curID); } catch (_) {}
          return;
        }
      }

      const isNotFound = document.querySelector('[data-testid="not-found"]') ||
                         Array.from(document.querySelectorAll("h1, h2, h3, p")).some(el =>
                           el.textContent && (
                             el.textContent.includes("Conversation not found") ||
                             el.textContent.includes("This conversation could not be found")
                           )
                         );

      if (!uuidRegex.test(curID) || isNotFound) {
        const pinned = localStorage.getItem("antigravity_swiss_last_conversation_path") ||
                       localStorage.getItem("antigravity_swiss_pinned_conversation_path");
        if (pinned && pinned.startsWith("/c/")) {
          const pinnedID = pinned.replace(/^\/c\//, "").split(/[?#]/)[0];
          if (uuidRegex.test(pinnedID) && curID !== pinnedID) {
            const now = Date.now();
            if (!window.__swissLastRedirectTime || (now - window.__swissLastRedirectTime > 5000)) {
              window.__swissLastRedirectTime = now;
              console.warn("[SwissKnife] Redirecting from invalid/not-found conversation " + curPath + " to pinned: " + pinned);
              const navLink = document.querySelector('a[href*="' + pinnedID + '"]');
              if (navLink) {
                navLink.click();
              } else {
                window.location.assign(pinned);
              }
            }
          }
        }
      }
    }

    // clearStaleRevivalDraft removes a resume prompt left unsent in the composer.
    //
    // Double dispatch used to leave one copy sent in the transcript and a second
    // copy still sitting in the composer (a second dispatcher re-typing the same
    // text without sending). The draft is only cleared when it is exactly the
    // resume prompt: a real user draft is never touched.
    function clearStaleRevivalDraft() {
      try {
        const editor = document.querySelector('div[aria-label="Message input"][contenteditable="true"]') ||
                       document.querySelector('[data-testid="agent-input-box"] [contenteditable="true"]') ||
                       document.querySelector('[data-lexical-editor="true"][contenteditable="true"]') ||
                       document.querySelector('.lexical-container [contenteditable="true"]') ||
                       document.querySelector('[data-testid="chat-input-textarea"]') ||
                       document.querySelector('textarea[placeholder*="Ask"]');
        if (!editor) return;
        const text = (editor.isContentEditable ? (editor.innerText || "") : (editor.value || "")).trim();
        const resumeText = "Please continue ongoing tasks and subagents.";
        if (text !== resumeText && !/^Please continue\b/i.test(text)) return;
        if (editor.isContentEditable) {
          const lex = editor.__lexicalEditor;
          if (lex && typeof lex.update === "function" && lex._nodes) {
            lex.update(() => {
              const root = lex._editorState?._nodeMap?.get('root');
              if (root) root.clear();
            });
          } else {
            const sel = window.getSelection();
            if (sel) {
              const range = document.createRange();
              range.selectNodeContents(editor);
              sel.removeAllRanges();
              sel.addRange(range);
            }
            document.execCommand("selectAll", false, null);
            document.execCommand("delete", false, null);
          }
          if ((editor.innerText || "").trim().length > 0) {
            editor.dispatchEvent(new InputEvent("beforeinput", {
              inputType: "deleteContentBackward",
              bubbles: true,
              cancelable: true
            }));
          }
        } else {
          editor.value = "";
        }
        editor.dispatchEvent(new Event("input", { bubbles: true, composed: true }));
        console.warn("[SwissKnife AutoRevival] Cleared a stale resume prompt left unsent in the composer.");
      } catch (_) {}
    }

    async function checkAndExecuteConversationRevival() {
      const explicitIntent = window.__swissExplicitConversationNavigation;
      if (bindExplicitConversationNavigationIntent(explicitIntent, window.location)) {
        persistExplicitConversationNavigationIntent(explicitIntent);
      }
      if (isExplicitConversationNavigationActive(explicitIntent, window.location)) {
        return;
      }
      checkAndRecoverFromNonexistentConversation();

      if (autoRevivalInFlight) return;
      const now = Date.now();
      if (now - lastRevivalCheckTime < 1500) return;
      lastRevivalCheckTime = now;

      // Track agent generation transition to advance turn epoch when a turn finishes
      const isGeneratingNow = Boolean(
        document.querySelector('[data-testid="agent-generating"], [data-testid="stop-button"], [data-tooltip-id="input-send-button-cancel-tooltip"]')
      );
      if (window.__swissLastGeneratingState === true && !isGeneratingNow) {
        window.__swissTurnEpoch = (window.__swissTurnEpoch || 0) + 1;
      }
      window.__swissLastGeneratingState = isGeneratingNow;

      const curPath = window.location.pathname || "/";
      const curID = curPath.replace(/^\/c\//, "").split(/[?#]/)[0];
      const uuidRegex = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

      try {
        window.__swissDispatchedIntents = window.__swissDispatchedIntents || {};

        // Candidate intents for this pass. An account switch can interrupt several
        // conversations, so the daemon publishes a list; the single legacy fields
        // remain supported for older daemons.
        let candidates = [];
        let fromDaemon = false;
        try {
          if (typeof window !== "undefined" && window.fetch) {
            let statusUrl = "http://127.0.0.1:8765/api/conversations/status";
            if (uuidRegex.test(curID)) {
              statusUrl += "?conversation_id=" + encodeURIComponent(curID);
            }
            const res = await fetch(statusUrl);
            if (res.ok) {
              const data = await res.json();
              if (data && Array.isArray(data.pending_intents)) {
                candidates = data.pending_intents;
                fromDaemon = true;
              } else if (data && data.pending_intent) {
                candidates = [data.pending_intent];
                fromDaemon = true;
              } else if (data && (data.cascade_id || data.root_conversation_id)) {
                candidates = [data];
                fromDaemon = true;
              }
            }
          }
        } catch (_) {}

        if (!fromDaemon && typeof window !== "undefined" && Array.isArray(window.__swissPendingContinuations)) {
          candidates = window.__swissPendingContinuations;
        }
        if (!fromDaemon && (!candidates || candidates.length === 0) && typeof window !== "undefined" && window.__swissPendingContinuation) {
          candidates = [window.__swissPendingContinuation];
        }

        if (!candidates || candidates.length === 0) {
          try {
            const raw = localStorage.getItem("antigravity_swiss_pending_continuation");
            if (raw) candidates = [JSON.parse(raw)];
          } catch (_) {}
        }

        if (!candidates || candidates.length === 0) return;

        // One intent per pass. Intents already dispatched in this page (or already
        // revived in this tab) are skipped WITHOUT being acked, so an in-flight or
        // unacked intent never blocks the queue; acked intents are simply absent.
        const intentKeyOf = (it) => it.intent_id || ("auto-revive-" + (it.root_conversation_id || it.cascade_id));
        const isDispatched = (it) => {
          const key = intentKeyOf(it);
          if (window.__swissDispatchedIntents[key]) return true;
          try {
            if (sessionStorage.getItem("antigravity_swiss_revived_intent_" + key)) return true;
            const convID = it.root_conversation_id || it.cascade_id;
            if (convID && sessionStorage.getItem("antigravity_swiss_revived_conv_" + convID)) return true;
          } catch (_) {}
          return false;
        };

        let pending = null;
        for (const it of candidates) {
          if (!it) continue;
          if (it.resumed || it.status === "revived") continue;
          if (isDispatched(it)) continue;
          pending = it;
          break;
        }

        if (!pending) {
          // Everything left was already dispatched in this page. Conversations
          // really revived in this tab are acked so the daemon stops re-publishing
          // them; intents whose send is still in flight are left for the ack that
          // follows their confirmed send.
          clearStaleRevivalDraft();
          for (const it of candidates) {
            if (!it) continue;
            const key = intentKeyOf(it);
            const conv = it.root_conversation_id || it.cascade_id;
            let revivedHere = false;
            try {
              revivedHere = Boolean(conv && sessionStorage.getItem("antigravity_swiss_revived_conv_" + conv));
            } catch (_) {}
            if (revivedHere) {
              await ackContinuation(conv, key);
            }
          }
          return;
        }

        const convID = pending.root_conversation_id || pending.cascade_id;
        if (!convID) return;

        const intentKey = pending.intent_id || ("auto-revive-" + convID);

        window.__swissDispatchedIntents = window.__swissDispatchedIntents || {};
        if (window.__swissDispatchedIntents[intentKey]) {
          return;
        }

        // Strictly validate convID: reject non-UUID stubs (test-webgui-conv, test, etc.)
        if (!uuidRegex.test(convID)) {
          console.warn("[SwissKnife AutoRevival] Rejecting invalid non-UUID intent:", convID);
          try { localStorage.removeItem("antigravity_swiss_pending_continuation"); } catch (_) {}
          await ackContinuation(convID, intentKey);
          return;
        }

        try {
          if (sessionStorage.getItem("antigravity_swiss_revived_intent_" + intentKey) ||
              sessionStorage.getItem("antigravity_swiss_revived_conv_" + convID)) {
            window.__swissDispatchedIntents[intentKey] = Date.now();
            await ackContinuation(convID, intentKey);
            return;
          }
        } catch (_) {}

        // Check TTL & creation timestamp (default 300s for desktop revival)
        const ttl = (pending.ttl_seconds || 300) * 1000;
        const createdAt = pending.created_at ? new Date(pending.created_at).getTime() : (pending.timestamp ? Number(pending.timestamp) : 0);
        if (!createdAt || isNaN(createdAt) || (now - createdAt > ttl) || (createdAt > now + 60000)) {
          try { localStorage.removeItem("antigravity_swiss_pending_continuation"); } catch (_) {}
          await ackContinuation(convID, intentKey);
          return;
        }

        const targetPath = "/c/" + convID;
        const curPath = window.location.pathname || "/";

        // Protect user drafts: if the editor has typed text, never navigate away or hijack view
        const activeEditor = document.querySelector('[data-testid="agent-input-box"] [contenteditable="true"]') ||
                             document.querySelector('[data-lexical-editor="true"][contenteditable="true"]') ||
                             document.querySelector('.lexical-container [contenteditable="true"]') ||
                             document.querySelector('[data-testid="chat-input-textarea"]') ||
                             document.querySelector('textarea[placeholder*="Ask"]');
        if (activeEditor) {
          const curDraft = (activeEditor.isContentEditable ? (activeEditor.innerText || "") : (activeEditor.value || "")).trim();
          if (curDraft.length > 0) {
            return;
          }
        }

        // If another conversation view is currently generating, retain intent until it finishes
        const activeGenerating = document.querySelector('[data-testid="agent-generating"], [data-testid="stop-button"], button[aria-label*="Stop execution" i], button[aria-label*="Stop Subagent" i]');
        if (activeGenerating && !curPath.startsWith(targetPath)) {
          return;
        }

        // 1. If not at the target conversation route, navigate
        if (!curPath.startsWith(targetPath)) {
          if (curPath.startsWith("/onboarding")) {
            window.location.assign(targetPath);
            return;
          }
          const navLink = document.querySelector('a[href*="' + convID + '"]');
          if (navLink) {
            navLink.click();
            return;
          }
          if (window.__TSR_ROUTER__ && typeof window.__TSR_ROUTER__.navigate === "function") {
            try {
              window.__TSR_ROUTER__.navigate({ to: targetPath });
              return;
            } catch (_) {}
          }
          window.location.assign(targetPath);
          return;
        }

        // 2. Verify conversation view and input editor are mounted
        const convView = document.querySelector('[data-testid="conversation-view"]');
        const editor = document.querySelector('div[aria-label="Message input"][contenteditable="true"]') ||
                       document.querySelector('[data-testid="agent-input-box"] [contenteditable="true"]') ||
                       document.querySelector('[data-lexical-editor="true"][contenteditable="true"]') ||
                       document.querySelector('.lexical-container [contenteditable="true"]') ||
                       document.querySelector('[data-testid="chat-input-textarea"]') ||
                       document.querySelector('textarea[placeholder*="Ask"]');
        if (!convView || !editor) return;

        // 3. Check if conversation is already active: generating, running subagents, or has queued messages
        const hasQueued = Boolean(
          document.querySelector('[data-testid="queued-messages-expand-button"]') ||
          document.querySelector('button[aria-label*="Collapse queued messages" i]') ||
          document.querySelector('button[aria-label*="Send now:" i]') ||
          Array.from(document.querySelectorAll("*")).some(el =>
            el.children.length === 0 && (
              el.textContent.includes("Queued Messages") ||
              el.textContent.includes("Sends after agent finishes")
            )
          )
        );

        const hasRunningSubagents = Boolean(
          document.querySelector('button[aria-label*="Stop Subagent" i]') ||
          document.querySelector('button[data-tooltip-id^="stop-subagent-"]') ||
          Array.from(document.querySelectorAll("*")).some(el =>
            el.children.length === 0 && /(\d+)\s+subagents?\s+running/i.test(el.textContent)
          )
        );

        const isGenerating = Boolean(
          document.querySelector('button[aria-label*="Cancel" i]') ||
          document.querySelector('button[aria-label*="Stop execution" i]') ||
          document.querySelector('button[aria-label*="Stop" i]') ||
          document.querySelector('[data-tooltip-id="input-send-button-cancel-tooltip"]') ||
          document.querySelector('[data-tooltip-id*="cancel-tooltip"]') ||
          document.querySelector('[data-tooltip-id*="stop-tooltip"]') ||
          document.querySelector('[data-testid*="cancel"]') ||
          document.querySelector('[data-testid*="stop"]') ||
          document.querySelector('[data-testid="send-button-pending"]') ||
          document.querySelector('[data-testid="agent-generating"]')
        );

        if (hasQueued || hasRunningSubagents || isGenerating) {
          // Conversation is ALREADY active and executing! Do NOT inject or queue continuation prompts!
          window.__swissDispatchedIntents[intentKey] = Date.now();
          try {
            sessionStorage.setItem("antigravity_swiss_revived_intent_" + intentKey, "true");
            sessionStorage.setItem("antigravity_swiss_revived_conv_" + convID, "true");
          } catch (_) {}

          // Clean up any accidentally queued duplicate revival prompt
          const queuedDelBtn = document.querySelector('button[aria-label*="Delete: Please continue ongoing tasks and subagents." i]') ||
                               document.querySelector('button[aria-label*="Remove: Please continue ongoing tasks and subagents." i]');
          if (queuedDelBtn) {
            try {
              queuedDelBtn.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true }));
              queuedDelBtn.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, cancelable: true }));
              queuedDelBtn.click();
            } catch (_) {}
          }

          await ackContinuation(convID, intentKey);
          return;
        }

        // Check if trigger prompt is empty (idle conversation preserved across switch/restart)
        const triggerPrompt = pending.trigger_prompt !== undefined ? pending.trigger_prompt : (pending.prompt !== undefined ? pending.prompt : "");
        if (!triggerPrompt || triggerPrompt.trim() === '') {
          window.__swissDispatchedIntents[intentKey] = Date.now();
          try {
            sessionStorage.setItem("antigravity_swiss_revived_intent_" + intentKey, "true");
            sessionStorage.setItem("antigravity_swiss_revived_conv_" + convID, "true");
          } catch (_) {}
          await ackContinuation(convID, intentKey);
          return;
        }

        // 4. Check if interactive questionnaire continue button is present
        const interactBtn = document.querySelector('[data-testid="interaction-continue-button"]');
        if (interactBtn && !interactBtn.disabled) {
          window.__swissDispatchedIntents[intentKey] = Date.now();
          try {
            sessionStorage.setItem("antigravity_swiss_revived_intent_" + intentKey, "true");
            sessionStorage.setItem("antigravity_swiss_revived_conv_" + convID, "true");
          } catch (_) {}
          autoRevivalInFlight = true;
          interactBtn.click();
          await ackContinuation(convID, intentKey);
          autoRevivalInFlight = false;
          return;
        }

        // 5. Inject prompt into editor and click send
        autoRevivalInFlight = true;
        const promptText = triggerPrompt || "Please continue ongoing tasks and subagents.";

        if (editor.isContentEditable) {
          const curText = (editor.innerText || "").trim();
          if (curText.length > 0 && !curText.includes(promptText)) {
            // User is actively typing their own prompt; revival satisfied, protect user draft!
            autoRevivalInFlight = false;
            window.__swissDispatchedIntents[intentKey] = Date.now();
            try {
              sessionStorage.setItem("antigravity_swiss_revived_intent_" + intentKey, "true");
              sessionStorage.setItem("antigravity_swiss_revived_conv_" + convID, "true");
            } catch (_) {}
            await ackContinuation(convID, intentKey);
            return;
          }
          if (!curText.includes(promptText)) {
            editor.focus();
            const lex = editor.__lexicalEditor;
            if (lex && typeof lex.update === "function" && lex._nodes) {
              lex.update(() => {
                const ParagraphNode = lex._nodes.get('paragraph')?.klass;
                const TextNode = lex._nodes.get('text')?.klass;
                const root = lex._editorState?._nodeMap?.get('root');
                if (ParagraphNode && TextNode && root) {
                  root.clear();
                  const p = new ParagraphNode();
                  const t = new TextNode(promptText);
                  p.append(t);
                  root.append(p);
                }
              });
            } else {
              const sel = window.getSelection();
              if (sel) {
                const range = document.createRange();
                range.selectNodeContents(editor);
                sel.removeAllRanges();
                sel.addRange(range);
              }
              document.execCommand("selectAll", false, null);
              document.execCommand("insertText", false, promptText);
              const hasInserted = (editor.innerText || "").includes(promptText);
              if (!hasInserted) {
                try {
                  editor.dispatchEvent(new InputEvent("beforeinput", {
                    inputType: "insertText",
                    data: promptText,
                    bubbles: true,
                    cancelable: true
                  }));
                } catch (_) {}
              }
            }
            editor.dispatchEvent(new Event("input", { bubbles: true, composed: true }));
          }
        } else {
          const curVal = (editor.value || "").trim();
          if (curVal.length > 0 && !curVal.includes(promptText)) {
            autoRevivalInFlight = false;
            window.__swissDispatchedIntents[intentKey] = Date.now();
            try {
              sessionStorage.setItem("antigravity_swiss_revived_intent_" + intentKey, "true");
              sessionStorage.setItem("antigravity_swiss_revived_conv_" + convID, "true");
            } catch (_) {}
            await ackContinuation(convID, intentKey);
            return;
          }
          if (!curVal.includes(promptText)) {
            editor.focus();
            editor.value = promptText;
            editor.dispatchEvent(new Event("input", { bubbles: true }));
            editor.dispatchEvent(new Event("change", { bubbles: true }));
          }
        }

        (async () => {
          try {
            await new Promise(r => setTimeout(r, 80));

            // Check before sending that conversation didn't become busy
            const isCurrentlyBusy = () => Boolean(
              document.querySelector('button[aria-label*="Cancel" i]') ||
              document.querySelector('button[aria-label*="Stop execution" i]') ||
              document.querySelector('button[aria-label*="Stop Subagent" i]') ||
              document.querySelector('button[aria-label*="Stop" i]') ||
              document.querySelector('[data-tooltip-id*="cancel-tooltip"]') ||
              document.querySelector('[data-tooltip-id*="stop-tooltip"]') ||
              document.querySelector('[data-testid*="cancel"]') ||
              document.querySelector('[data-testid*="stop"]') ||
              document.querySelector('[data-testid="queued-messages-expand-button"]') ||
              Array.from(document.querySelectorAll("*")).some(el =>
                el.children.length === 0 && /(\d+)\s+subagents?\s+running/i.test(el.textContent)
              )
            );

            if (isCurrentlyBusy()) {
              autoRevivalInFlight = false;
              window.__swissDispatchedIntents[intentKey] = Date.now();
              try {
                sessionStorage.setItem("antigravity_swiss_revived_intent_" + intentKey, "true");
                sessionStorage.setItem("antigravity_swiss_revived_conv_" + convID, "true");
              } catch (_) {}
              await ackContinuation(convID, intentKey);
              return;
            }

            const isEditorCleared = () => {
              const cur = (editor.isContentEditable ? (editor.innerText || "") : (editor.value || "")).trim();
              return cur.length === 0 || !cur.includes(promptText);
            };

            const dispatchEnter = () => {
              try {
                editor.focus();
                const evOpts = { key: "Enter", code: "Enter", keyCode: 13, which: 13, bubbles: true, cancelable: true, composed: true };
                editor.dispatchEvent(new KeyboardEvent("keydown", evOpts));
                editor.dispatchEvent(new KeyboardEvent("keypress", evOpts));
                editor.dispatchEvent(new KeyboardEvent("keyup", evOpts));
              } catch (_) {}
            };

            const submitForm = () => {
              try {
                const form = editor.closest('form');
                if (form) {
                  if (typeof form.requestSubmit === "function") {
                    form.requestSubmit();
                    return true;
                  } else {
                    form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
                    return true;
                  }
                }
              } catch (_) {}
              return false;
            };

            const isCancelOrStop = (btn) => {
              if (!btn) return true;
              const aria = (btn.getAttribute('aria-label') || '').toLowerCase();
              const testid = (btn.getAttribute('data-testid') || '').toLowerCase();
              const tooltip = (btn.getAttribute('data-tooltip-id') || '').toLowerCase();
              return aria.includes('cancel') || aria.includes('stop') ||
                testid.includes('cancel') || testid.includes('stop') ||
                tooltip.includes('cancel') || tooltip.includes('stop');
            };

            const findSendButton = () => {
              const direct = document.querySelector('[data-testid="send-button"]') ||
                document.querySelector('[data-tooltip-id="input-send-button-send-tooltip"]') ||
                document.querySelector('[data-tooltip-id*="send-tooltip"]') ||
                document.querySelector('button[aria-label*="Send" i]') ||
                document.querySelector('button[aria-label*="Submit" i]') ||
                document.querySelector('.chat-input-toolbar button:last-child');
              if (direct && !isCancelOrStop(direct)) return direct;

              const container = editor.closest('form') ||
                editor.closest('.lexical-container')?.parentElement ||
                editor.closest('[data-testid*="input"]')?.parentElement ||
                editor.parentElement?.parentElement;
              if (container) {
                const btnInContainer = container.querySelector('button[type="submit"]') ||
                  container.querySelector('button[aria-label*="Send" i]') ||
                  container.querySelector('[data-testid*="send"]');
                if (btnInContainer && !isCancelOrStop(btnInContainer)) return btnInContainer;
              }

              const allButtons = Array.from(document.querySelectorAll('button'));
              for (const btn of allButtons) {
                if (isCancelOrStop(btn)) continue;
                const aria = (btn.getAttribute('aria-label') || '').toLowerCase();
                const testid = (btn.getAttribute('data-testid') || '').toLowerCase();
                const tooltip = (btn.getAttribute('data-tooltip-id') || '').toLowerCase();
                if (aria.includes('send') || testid.includes('send') || tooltip.includes('send') || btn.type === 'submit') {
                  return btn;
                }
              }
              return null;
            };

            const triggerClick = (btn) => {
              if (!btn) return false;
              try {
                if (btn.disabled) {
                  btn.disabled = false;
                  btn.removeAttribute('disabled');
                }
                btn.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, cancelable: true, composed: true }));
                btn.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true, composed: true }));
                btn.dispatchEvent(new PointerEvent('pointerup', { bubbles: true, cancelable: true, composed: true }));
                btn.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, cancelable: true, composed: true }));
                btn.click();
                return true;
              } catch (_) {
                try { btn.click(); return true; } catch (_) { return false; }
              }
            };

            let sent = false;
            for (let i = 0; i < 25; i++) {
              if (isCurrentlyBusy() || isEditorCleared()) {
                sent = true;
                break;
              }

              const sendBtn = findSendButton();
              if (sendBtn) {
                triggerClick(sendBtn);
              }

              dispatchEnter();
              submitForm();

              await new Promise(r => setTimeout(r, 80));

              if (isCurrentlyBusy() || isEditorCleared()) {
                sent = true;
                break;
              }
            }

            // Clean up any accidentally queued duplicate revival prompt
            const queuedDelBtn = document.querySelector('button[aria-label*="Delete: Please continue ongoing tasks and subagents." i]') ||
                                 document.querySelector('button[aria-label*="Remove: Please continue ongoing tasks and subagents." i]');
            if (queuedDelBtn) {
              try {
                triggerClick(queuedDelBtn);
              } catch (_) {}
            }

            if (sent || isCurrentlyBusy() || isEditorCleared()) {
              window.__swissDispatchedIntents[intentKey] = Date.now();
              try {
                sessionStorage.setItem("antigravity_swiss_revived_intent_" + intentKey, "true");
                sessionStorage.setItem("antigravity_swiss_revived_conv_" + convID, "true");
              } catch (_) {}
              await ackContinuation(convID, intentKey);
            }
          } catch (err) {
            console.warn("[SwissKnife AutoRevival] Send error:", err);
          } finally {
            autoRevivalInFlight = false;
          }
        })();

      } catch (err) {
        console.warn("[SwissKnife AutoRevival] Error:", err);
        autoRevivalInFlight = false;
      }
    }

    if (window.__swissRevivalInterval) {
      clearInterval(window.__swissRevivalInterval);
    }
    window.__swissRevivalInterval = setInterval(checkAndExecuteConversationRevival, 2500);
    if (!window.__swissRevivalPopstateBound) {
      window.__swissRevivalPopstateBound = true;
      window.addEventListener("popstate", () => {
        if (typeof window.__swissCheckRevival === "function") {
          setTimeout(window.__swissCheckRevival, 300);
        }
        setTimeout(renderContextWindowProgressRing, 300);
      }, { passive: true });
    }
    window.__swissCheckRevival = checkAndExecuteConversationRevival;

    function formatTokensUnit(n) {
      if (!n || isNaN(n) || n <= 0) return "0K";
      if (n >= 1000000) {
        const m = n / 1000000;
        return (m >= 10 || Number.isInteger(m) ? m.toFixed(0) : m.toFixed(1).replace(/\.0$/, "")) + "M";
      }
      const k = n / 1000;
      return (k >= 10 || Number.isInteger(k) ? Math.round(k) : k.toFixed(1).replace(/\.0$/, "")) + "K";
    }

    function getConversationIdFromElement(el) {
      if (!el) return null;
      try {
        const box = el.closest ? el.closest('[data-testid="agent-input-box"]') : el;
        const target = box || el;
        const fiberKey = Object.keys(target).find(k => k.startsWith("__reactFiber"));
        if (!fiberKey) return null;
        let curr = target[fiberKey];
        while (curr) {
          if (curr.memoizedProps) {
            const id = curr.memoizedProps.conversationId || curr.memoizedProps.cascadeId;
            if (typeof id === "string" && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(id)) {
              return id;
            }
          }
          curr = curr.return;
        }
      } catch (_) {}
      return null;
    }

    function getVisibleConversationId(el) {
      const candidates = [];
      const add = (candidate) => {
        if (candidate && candidates.indexOf(candidate) === -1) candidates.push(candidate);
      };
      add(el);
      if (el && typeof el.closest === "function") {
        add(el.closest('[data-testid="agent-input-box"]'));
      }
      const view = document.querySelector('[data-testid="conversation-view"]');
      if (view) {
        view.querySelectorAll('[data-testid="agent-input-box"]').forEach(add);
        add(view);
      }
      for (let i = 0; i < candidates.length; i++) {
        const id = getConversationIdFromElement(candidates[i]);
        if (isConversationId(id)) return id;
      }
      return null;
    }

    function resolveCurrentConversationId(el) {
      const visibleId = getVisibleConversationId(el);
      return isConversationId(visibleId) ? visibleId : conversationMetricIdentity.conversationId;
    }

    let conversationMetricIdentity = createConversationMetricIdentityState();
    let conversationRouteSyncState = createConversationRouteSyncState();
    let conversationMetricResetEpochSeen = Number(window[SWISS_CONVERSATION_METRIC_RESET_EPOCH_KEY]) || 0;
    let conversationMetricResetStaleId = null;
    let conversationMetricRequestGenerations = {};
    let lastTokensFetchTimeByConv = {};
    let cachedTokensByConv = {};
    let cachedContextWindowByConv = {};
    let subagentNameToConvId = {};
    async function fetchActiveConversationTokens(convId, parentId, agentName) {
      if (window.__swissDaemonOnline === false || !window.__swissDaemonOnline) return;
      const cacheKey = convId || (parentId && agentName ? (parentId + ":" + agentName) : null);
      if (!cacheKey) return;
      const now = Date.now();
      if (now - (lastTokensFetchTimeByConv[cacheKey] || 0) < 2500) return;
      lastTokensFetchTimeByConv[cacheKey] = now;
      const requestGeneration = beginConversationMetricRequest(conversationMetricRequestGenerations, cacheKey);
      try {
        let url = "http://127.0.0.1:8765/api/tokens/chat-metrics?";
        if (convId) {
          url += "conversation_id=" + encodeURIComponent(convId);
        } else if (parentId && agentName) {
          url += "parent_id=" + encodeURIComponent(parentId) + "&agent_name=" + encodeURIComponent(agentName) + "&scope=conversation";
        } else {
          return;
        }
        const res = await fetch(url, {
          headers: getSwissAuthHeaders(),
          signal: (typeof AbortSignal !== "undefined" && AbortSignal.timeout) ? AbortSignal.timeout(8000) : undefined
        });
        if (res.ok) {
          const data = await res.json();
          if (!isCurrentConversationMetricRequest(conversationMetricRequestGenerations, cacheKey, requestGeneration)) {
            return;
          }
          const targetId = data.conversation_id || convId;
          if (targetId) {
            if (parentId && agentName) {
              subagentNameToConvId[parentId + ":" + agentName] = targetId;
            }
            let jumpBarChanged = false;
            if (data && typeof data.user_prompt_count === "number" && data.user_prompt_count >= 0) {
              // Count only USER_EXPLICIT/USER_INPUT transcript entries. Assistant
              // execution turns and system boundaries must not create rail rows.
              const previousPromptCount = jumpBarPromptCountsByConversation[targetId];
              jumpBarPromptCountsByConversation[targetId] = data.user_prompt_count;
              jumpBarChanged = previousPromptCount !== data.user_prompt_count;
            }
            if (data && Array.isArray(data.prompts)) {
              // The transcript is authoritative for the turn list, including the
              // turns the virtualized DOM has unmounted. Absent prompts (older
              // daemons) leave the rail on its DOM-only fallback.
              const previousPrompts = jumpBarPromptsByConversation[targetId];
              jumpBarPromptsByConversation[targetId] = data.prompts;
              for (let pi = 0; pi < data.prompts.length; pi++) {
                const prompt = data.prompts[pi];
                if (!prompt || typeof prompt.index !== "number") continue;
                cachePromptText(jumpBarPromptTextCache, targetId, prompt.index, prompt.text);
              }
              if (!jumpBarChanged && (previousPrompts || []).length !== data.prompts.length) {
                jumpBarChanged = true;
              }
            }
            if (data && Array.isArray(data.turns) && data.turns.length > 0) {
              const lastTurn = data.turns[data.turns.length - 1];
              const turnUsage = lastTurn.context_tokens || ((lastTurn.input_tokens || 0) + (lastTurn.output_tokens || 0));
              cachedTokensByConv[targetId] = turnUsage;
            } else if (data && data.success) {
              const sumUsage = data.summary ? (data.summary.context_tokens || ((data.summary.input_tokens || 0) + (data.summary.output_tokens || 0))) : 0;
              cachedTokensByConv[targetId] = sumUsage;
            }
            const hostContextWindow = data ? data.context_window : null;
            if (hostContextWindow && typeof hostContextWindow.max_tokens === "number" && hostContextWindow.max_tokens > 0) {
              cachedContextWindowByConv[targetId] = hostContextWindow;
              if (typeof hostContextWindow.used_tokens === "number" && hostContextWindow.used_tokens > 0) {
                cachedTokensByConv[targetId] = hostContextWindow.used_tokens;
              }
            } else {
              delete cachedContextWindowByConv[targetId];
            }
            renderContextWindowProgressRing();
            if (jumpBarChanged) {
            }
          }
        } else if (isCurrentConversationMetricRequest(conversationMetricRequestGenerations, cacheKey, requestGeneration)) {
          clearConversationMetricCache(cachedTokensByConv, cachedContextWindowByConv, convId);
          renderContextWindowProgressRing();
        }
      } catch (_) {
        if (isCurrentConversationMetricRequest(conversationMetricRequestGenerations, cacheKey, requestGeneration)) {
          clearConversationMetricCache(cachedTokensByConv, cachedContextWindowByConv, convId);
          renderContextWindowProgressRing();
        }
      }
    }

    function removeContextWindowProgressRings() {
      const mainRing = document.getElementById("swiss-context-window-ring");
      if (mainRing) mainRing.remove();
      document.querySelectorAll("[id^='swiss-context-window-ring-sub-'], .swiss-context-window-ring, .swiss-context-window-ring-sub").forEach(r => r.remove());
    }

    function resetConversationMetricsForNewConversation(intent) {
      if (!intent || intent.kind !== "new_conversation" || !intent.pending) return false;
      window[SWISS_CONVERSATION_METRIC_RESET_EPOCH_KEY] =
        (Number(window[SWISS_CONVERSATION_METRIC_RESET_EPOCH_KEY]) || 0) + 1;
      synchronizeConversationMetricResetEpoch();
      return true;
    }

    window.__swissUpdateContextWindow = function() {
      try { renderContextWindowProgressRing(); } catch (_) {}
    };

    window.__swissOnEnhancementsDaemonChanged = function(online) {
      if (!online) {
        removeContextWindowProgressRings();
        cachedTokensByConv = {};
        cachedContextWindowByConv = {};
        lastTokensFetchTimeByConv = {};
        conversationMetricIdentity = createConversationMetricIdentityState();
        conversationMetricRequestGenerations = {};
      } else {
        cachedTokensByConv = {};
        cachedContextWindowByConv = {};
        lastTokensFetchTimeByConv = {};
        conversationMetricIdentity = createConversationMetricIdentityState();
        conversationMetricRequestGenerations = {};
        syncConfigFromServer();
        renderContextWindowProgressRing();
      }
    };

    if (!window.__swissExplicitConversationNavigationBound) {
      window.__swissExplicitConversationNavigationBound = true;
      if (!window.__swissExplicitConversationNavigation) {
        window.__swissExplicitConversationNavigation = loadExplicitConversationNavigationIntent();
      }
      if (typeof document.addEventListener === "function") {
        document.addEventListener("click", (e) => {
          if (!e.isTrusted) return;
          const intent = createExplicitConversationNavigationIntent(e.target, window.location, Date.now());
          if (!intent) return;
          window.__swissExplicitConversationNavigation = intent;
          persistExplicitConversationNavigationIntent(intent);
          resetConversationMetricsForNewConversation(intent);
          setTimeout(() => {
            if (bindExplicitConversationNavigationIntent(intent, window.location)) {
              persistExplicitConversationNavigationIntent(intent);
            }
          }, 0);
        }, true);
      }
      if (typeof window.addEventListener === "function") {
        window.addEventListener("popstate", (e) => {
          if (!e || e.isTrusted !== true) return;
          const intent = createExplicitHistoryNavigationIntent(window.location, Date.now());
          window.__swissExplicitConversationNavigation = intent;
          persistExplicitConversationNavigationIntent(intent);
        }, { passive: true });
      }
    }

    const compactingStateByRing = {};

    function findInputBoxFiberProps(box) {
      if (!box) return null;
      if (box.memoizedProps && (typeof box.memoizedProps.handleSubmit === "function" || (box.memoizedProps.lexicalRef && box.memoizedProps.lexicalRef.current))) {
        return box.memoizedProps;
      }
      let fiberKey = Object.keys(box).find(k => k.startsWith("__reactFiber"));
      let startNode = fiberKey ? box[fiberKey] : null;
      if (!startNode && box.querySelector) {
        const inner = box.querySelector('[contenteditable="true"]') || box.querySelector('textarea') || (box.children && box.children[0]);
        if (inner) {
          if (inner.memoizedProps) return inner.memoizedProps;
          fiberKey = Object.keys(inner).find(k => k.startsWith("__reactFiber"));
          if (fiberKey) startNode = inner[fiberKey];
        }
      }
      if (!startNode) return null;

      let bestProps = null;
      let curr = startNode;
      while (curr) {
        const mp = curr.memoizedProps;
        if (mp) {
          if (typeof mp.handleSubmit === "function" && mp.lexicalRef && mp.lexicalRef.current) {
            return mp;
          }
          if (!bestProps && (typeof mp.handleSubmit === "function" || (mp.lexicalRef && mp.lexicalRef.current) || typeof mp.onSend === "function" || Array.isArray(mp.slashCommandItems))) {
            bestProps = mp;
          }
        }
        curr = curr.return;
      }
      if (bestProps) return bestProps;

      const queue = [startNode];
      const visited = new Set();
      while (queue.length > 0 && visited.size < 300) {
        const node = queue.shift();
        if (!node || visited.has(node)) continue;
        visited.add(node);
        const mp = node.memoizedProps;
        if (mp) {
          if (typeof mp.handleSubmit === "function" && mp.lexicalRef && mp.lexicalRef.current) {
            return mp;
          }
          if (!bestProps && (typeof mp.handleSubmit === "function" || (mp.lexicalRef && mp.lexicalRef.current) || typeof mp.onSend === "function" || Array.isArray(mp.slashCommandItems))) {
            bestProps = mp;
          }
        }
        if (node.child) queue.push(node.child);
        if (node.sibling) queue.push(node.sibling);
      }
      return bestProps;
    }

    async function invokeNativeCompactCommand(ring, convId, e) {
      if (!isContextWindowClickToCompactEnabled()) {
        return;
      }
      if (e) {
        try {
          if (typeof e.preventDefault === "function") e.preventDefault();
          if (typeof e.stopPropagation === "function") e.stopPropagation();
        } catch (_) {}
      }
      const key = (ring && ring.id) || convId || "main";
      if (compactingStateByRing[key] && compactingStateByRing[key].active) {
        return;
      }
      if (ring) {
        ring.setAttribute("data-swiss-compacting", "true");
      }
      const startTime = Date.now();
      compactingStateByRing[key] = { active: true, startTime: startTime };

      const inputBox = (ring && ring.closest ? ring.closest('[data-testid="agent-input-box"]') : null) || document.querySelector('[data-testid="agent-input-box"]');
      if (ring) {
        updateProgressRing(ring, convId);
      }

      let dispatched = false;
      if (inputBox) {
        const props = findInputBoxFiberProps(inputBox);
        if (props && props.lexicalRef && props.lexicalRef.current && typeof props.handleSubmit === "function") {
          try {
            const editor = props.lexicalRef.current;
            let compactInfo = null;
            if (Array.isArray(props.slashCommandItems)) {
              compactInfo = props.slashCommandItems.find(item => item && (item.name === "compact" || (item.slashCommand && item.slashCommand.name === "compact")));
            }
            if (!compactInfo) {
              compactInfo = {
                $typeName: "exa.codeium_common_pb.SlashCommandInfo",
                name: "compact",
                type: 1
              };
            }
            const mentionStateJSON = {
              root: {
                children: [
                  {
                    children: [
                      {
                        type: "contextScopeItemMention",
                        version: 1,
                        detail: compactInfo,
                        slashCommandInfo: compactInfo,
                        text: "/compact",
                        value: "/compact"
                      }
                    ],
                    direction: "ltr",
                    format: "",
                    indent: 0,
                    type: "paragraph",
                    version: 1
                  }
                ],
                direction: "ltr",
                format: "",
                indent: 0,
                type: "root",
                version: 1
              }
            };
            const fallbackStateJSON = {
              root: {
                children: [
                  {
                    children: [
                      {
                        detail: 0,
                        format: 0,
                        mode: "normal",
                        style: "",
                        text: "/compact",
                        type: "text",
                        version: 1
                      }
                    ],
                    direction: "ltr",
                    format: "",
                    indent: 0,
                    type: "paragraph",
                    version: 1
                  }
                ],
                direction: "ltr",
                format: "",
                indent: 0,
                type: "root",
                version: 1
              }
            };
            let parsedState = null;
            if (typeof editor.parseEditorState === "function") {
              try {
                parsedState = editor.parseEditorState(JSON.stringify(mentionStateJSON));
              } catch (_) {
                try {
                  parsedState = editor.parseEditorState(JSON.stringify(fallbackStateJSON));
                } catch (_) {}
              }
            }
            if (parsedState && typeof editor.setEditorState === "function") {
              editor.setEditorState(parsedState);
            } else if (typeof editor.setEditorState === "function") {
              try {
                editor.setEditorState(mentionStateJSON);
              } catch (_) {
                try { editor.setEditorState(fallbackStateJSON); } catch (_) {}
              }
            }

            // MessageDeliveryStrategy.NEXT_INVOCATION = 1 (send immediately, bypassing queue)
            await props.handleSubmit(editor, 1);
            dispatched = true;
          } catch (_) {}
        }

        if (!dispatched) {
          try {
            const editable = inputBox.querySelector('[contenteditable="true"]') || inputBox.querySelector('textarea');
            if (editable) {
              if (editable.tagName === "TEXTAREA" || editable.tagName === "INPUT") {
                editable.value = "/compact";
                editable.dispatchEvent(new Event("input", { bubbles: true }));
                editable.dispatchEvent(new Event("change", { bubbles: true }));
              } else {
                if (typeof editable.focus === "function") editable.focus();
                editable.innerText = "/compact";
                editable.textContent = "/compact";
                try {
                  editable.dispatchEvent(new InputEvent("input", { bubbles: true, inputType: "insertText", data: "/compact" }));
                } catch (_) {
                  editable.dispatchEvent(new Event("input", { bubbles: true }));
                }
              }
              const sendBtn = inputBox.querySelector('button[aria-label*="Send"], button[title*="Send"], [data-testid="send-button"]') ||
                              document.querySelector('button[aria-label*="Send"], button[title*="Send"], [data-testid="send-button"]');
              if (sendBtn && !sendBtn.disabled && typeof sendBtn.click === "function") {
                sendBtn.click();
              } else {
                editable.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", code: "Enter", keyCode: 13, which: 13, altKey: true, bubbles: true }));
                editable.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", code: "Enter", keyCode: 13, which: 13, bubbles: true }));
              }
              dispatched = true;
            }
          } catch (_) {}
        }
      }

      // Wait until both conditions are met:
      // a) At least 1 full rotation cycle (850ms elapsed since startTime) has completed
      // b) Any active compaction turn in the UI has finished (poll every 150ms checking if [data-testid="agent-loading"] with "Compacting" or active cancel/stop is still running, up to max timeout so it never hangs)
      const minCycleDuration = 850;
      const maxTimeoutMs = 120000;

      const checkCompactionActive = () => {
        try {
          const loadingEl = document.querySelector('[data-testid="agent-loading"]');
          if (loadingEl) {
            const txt = (loadingEl.innerText || loadingEl.textContent || "").toLowerCase();
            if (txt.includes("compact")) return true;
          }
          const stopBtn = (inputBox ? inputBox.querySelector('[data-testid="stop-button"], button[aria-label*="Stop"], button[aria-label*="Cancel"], button[title*="Stop"]') : null) ||
                          document.querySelector('[data-testid="stop-button"], button[aria-label*="Stop"], button[aria-label*="Cancel"], button[title*="Stop"]');
          if (stopBtn) {
            const bodyText = (document.body && (document.body.innerText || document.body.textContent)) || "";
            if (bodyText.includes("Compacting") || (loadingEl && (loadingEl.innerText || "").includes("Compacting"))) {
              return true;
            }
          }
        } catch (_) {}
        return false;
      };

      const pollTimer = setInterval(() => {
        const elapsed = Date.now() - startTime;
        if (elapsed < minCycleDuration) {
          return;
        }

        const isStillRunning = checkCompactionActive();
        if (!isStillRunning || elapsed >= maxTimeoutMs) {
          clearInterval(pollTimer);
          delete compactingStateByRing[key];
          if (ring) {
            ring.removeAttribute("data-swiss-compacting");
          }
          const actualConvId = convId || (ring ? getConversationIdFromElement(ring) : null);
          if (actualConvId) {
            delete lastTokensFetchTimeByConv[actualConvId];
            fetchActiveConversationTokens(actualConvId);
          } else if (convId) {
            delete lastTokensFetchTimeByConv[convId];
            fetchActiveConversationTokens(convId);
          }
          renderContextWindowProgressRing();
        }
      }, 150);
    }

    function updateProgressRing(ring, convId) {
      if (!ring) return;
      if (!isContextWindowIndicatorEnabled() || window.__swissDaemonOnline === false || !window.__swissDaemonOnline) {
        ring.remove();
        return;
      }
      const clickToCompactEnabled = isContextWindowClickToCompactEnabled();
      const clickAction = clickToCompactEnabled ? " — Click to compact" : "";
      ring.classList.add("swiss-context-window-ring");
      ring.setAttribute("role", clickToCompactEnabled ? "button" : "img");
      ring.setAttribute("tabindex", clickToCompactEnabled ? "0" : "-1");
      ring.style.cssText = "display: inline-flex; align-items: center; justify-content: center; width: 20px; height: 20px; margin-left: 6px; position: relative; user-select: none; flex-shrink: 0; vertical-align: middle; cursor: " + (clickToCompactEnabled ? "pointer" : "default") + ";";

      const ringKey = (ring && ring.id) || convId || "main";
      const isCompacting = ring.getAttribute("data-swiss-compacting") === "true" || Boolean(compactingStateByRing[ringKey] && compactingStateByRing[ringKey].active);
      if (isCompacting) {
        ring.setAttribute("data-swiss-compacting", "true");
        ring.setAttribute("title", "Compacting context window...");
        ring.setAttribute("aria-label", "Compacting context window...");
        ring.style.opacity = "1";
        ring.style.cursor = clickToCompactEnabled ? "pointer" : "default";

        const spinningHTML = '<svg width="18" height="18" viewBox="0 0 20 20" style="transform: rotate(-90deg); display: block;">' +
          '<circle cx="10" cy="10" r="7" fill="none" stroke="rgba(148, 163, 184, 0.3)" stroke-width="2" />' +
          '<circle cx="10" cy="10" r="7" fill="none" stroke="#94a3b8" stroke-width="2" stroke-linecap="round" stroke-dasharray="44" stroke-dashoffset="14" />' +
        '</svg>';
        if (ring.innerHTML !== spinningHTML) {
          ring.innerHTML = spinningHTML;
        }
      } else {
        const hostContextWindow = convId ? cachedContextWindowByConv[convId] : null;
        let validMax = hostContextWindow ? hostContextWindow.max_tokens : 0;
        let activeCustom = null;
        if (typeof window !== "undefined" && window.__swiss_session_custom_model_id && typeof customConfig !== "undefined" && Array.isArray(customConfig?.models)) {
          activeCustom = customConfig.models.find(m => m.id === window.__swiss_session_custom_model_id);
        }
        if (!activeCustom && typeof customConfig !== "undefined" && customConfig?.active_model_id && Array.isArray(customConfig?.models)) {
          activeCustom = customConfig.models.find(m => m.id === customConfig.active_model_id);
        }
        if (activeCustom && typeof activeCustom.context_window === "number" && activeCustom.context_window > 0) {
          validMax = activeCustom.context_window;
        }

        const usedTokens = (convId && cachedTokensByConv && typeof cachedTokensByConv[convId] === "number")
          ? cachedTokensByConv[convId]
          : (hostContextWindow && typeof hostContextWindow.used_tokens === "number" ? hostContextWindow.used_tokens : 0);
        const hasReliableData = validMax > 0 && typeof usedTokens === "number";
        if (!hasReliableData) {
          const unavailableText = "Context window usage unavailable" + clickAction;
          ring.setAttribute("title", unavailableText);
          ring.setAttribute("aria-label", unavailableText);
          ring.style.opacity = "0.35";
          ring.style.cursor = clickToCompactEnabled ? "pointer" : "default";
          const greyHTML = '<svg width="18" height="18" viewBox="0 0 20 20" style="transform: rotate(-90deg); display: block;">' +
            '<circle cx="10" cy="10" r="7" fill="none" stroke="rgba(148, 163, 184, 0.25)" stroke-width="2" />' +
          '</svg>';
          if (ring.innerHTML !== greyHTML) ring.innerHTML = greyHTML;
        } else {
          const pct = Math.max(0, Math.min(100, Math.round((usedTokens / validMax) * 100)));
          const formattedUsed = formatTokensUnit(usedTokens);
          const formattedMax = formatTokensUnit(validMax);
          const tooltipText = pct + "%% (" + formattedUsed + "/" + formattedMax + " active)" + clickAction;

          ring.setAttribute("title", tooltipText);
          ring.setAttribute("aria-label", "Context window used: " + tooltipText);
          ring.style.opacity = "1";
          ring.style.cursor = clickToCompactEnabled ? "pointer" : "default";

          const circumference = 44;
          const offset = Math.max(0, circumference - (circumference * pct / 100));

          const ringHTML = '<svg width="18" height="18" viewBox="0 0 20 20" style="transform: rotate(-90deg); display: block;">' +
            '<circle cx="10" cy="10" r="7" fill="none" stroke="rgba(148, 163, 184, 0.3)" stroke-width="2" />' +
            '<circle cx="10" cy="10" r="7" fill="none" stroke="#94a3b8" stroke-width="2" stroke-linecap="round" stroke-dasharray="44" stroke-dashoffset="' + offset.toFixed(2) + '" style="transition: stroke-dashoffset 0.3s ease;" />' +
          '</svg>';

          if (ring.innerHTML !== ringHTML) {
            ring.innerHTML = ringHTML;
          }
        }
      }

      if (clickToCompactEnabled) {
        ring.onclick = (e) => {
          invokeNativeCompactCommand(ring, convId, e);
        };
        ring.onkeydown = (e) => {
          if (e.key === "Enter" || e.key === " " || e.keyCode === 13 || e.keyCode === 32) {
            invokeNativeCompactCommand(ring, convId, e);
          }
        };
      } else {
        ring.onclick = null;
        ring.onkeydown = null;
      }
    }

    function renderContextWindowProgressRing() {
      if (!isContextWindowIndicatorEnabled() || window.__swissDaemonOnline === false || !window.__swissDaemonOnline) {
        removeContextWindowProgressRings();
        return;
      }
      if (synchronizeConversationMetricResetEpoch()) return;

      const conversationView = document.querySelector('[data-testid="conversation-view"]');
      const renderedConversationId = getVisibleConversationId(conversationView);
      const explicitIntent = window.__swissExplicitConversationNavigation;
      const sourceConversationId = parseConversationId(explicitIntent && explicitIntent.sourceRoute);
      const newConversationPending = Boolean(explicitIntent && explicitIntent.kind === "new_conversation" && explicitIntent.pending);
      const newConversationRendered = newConversationPending && isConversationId(renderedConversationId) &&
        (!isConversationId(sourceConversationId) || renderedConversationId !== sourceConversationId);
      let visibleConversationId = newConversationPending && !newConversationRendered ? null : renderedConversationId;
      if (conversationMetricResetStaleId) {
        // A pending New Conversation can briefly leave the old view mounted.
        // Concrete navigation back to that conversation ends the fence.
        const pendingNewConversationWithoutTarget = newConversationPending &&
          (!isConversationId(visibleConversationId) || visibleConversationId === conversationMetricResetStaleId);
        if (pendingNewConversationWithoutTarget) {
          removeContextWindowProgressRings();
          return;
        }
        conversationMetricResetStaleId = null;
      }
      synchronizeRenderedConversationRoute(conversationRouteSyncState, visibleConversationId, window.location, explicitIntent);
      const parentConvId = transitionConversationMetricIdentity(
        conversationMetricIdentity,
        conversationMetricRequestGenerations,
        visibleConversationId,
        window.location.pathname,
        Boolean(conversationView),
        !isExplicitConversationNavigationActive(explicitIntent, window.location)
      );

      // Part 1: Main Conversation Chat Input (Model Selector Trigger)
      const trigger = document.querySelector('[data-testid="model-selector-trigger"]') ||
                      document.querySelector('button[aria-label*="Select model"]');
      let mainRing = document.getElementById("swiss-context-window-ring");
      if (trigger) {
        // If this trigger belongs to a subagent chat box, do not mount mainRing here
        const triggerBox = trigger.closest ? trigger.closest('[data-testid="agent-input-box"], .no-focus-agent-input') : null;
        const isInsideSubagentBox = triggerBox && (
          triggerBox.querySelector('div[title="Subagent"], div[title="Agent Script"], [data-testid="subagent-name-tag"]') ||
          triggerBox.querySelector('[placeholder*="subagent"], [data-placeholder*="subagent"]') ||
          (triggerBox.innerText && triggerBox.innerText.includes("Message subagent"))
        );

        if (isInsideSubagentBox) {
          if (mainRing) mainRing.remove();
        } else {
          const convId = parentConvId;
          if (isConversationId(convId)) {
            fetchActiveConversationTokens(convId);
          }
          if (!mainRing) {
            mainRing = document.createElement("div");
            mainRing.id = "swiss-context-window-ring";
          }
          updateProgressRing(mainRing, convId);

          const pillContainer = trigger.closest(".no-focus-agent-input") || trigger.parentElement;
          const thinkingPill = document.getElementById("swiss-thinking-level-pill");
          const anchor = (thinkingPill && (thinkingPill.parentNode === pillContainer || (pillContainer && thinkingPill.parentNode === pillContainer.parentNode))) ? thinkingPill : (pillContainer || trigger);
          if (anchor && anchor.parentNode && (mainRing.previousElementSibling !== anchor)) {
            anchor.parentNode.insertBefore(mainRing, anchor.nextSibling);
          }
        }
      } else if (mainRing) {
        mainRing.remove();
      }

      // Part 2: Subagent Conversation Chat Input (Agent Name Tag)
      const agentTags = [];
      const seenBoxes = new Set();

      // Collect explicitly marked subagent tags first (at most one per box)
      document.querySelectorAll('div[title="Subagent"], div[title="Agent Script"], [data-testid="subagent-name-tag"]').forEach(tag => {
        const box = tag.closest ? (tag.closest('[data-testid="agent-input-box"]') || tag.closest('.no-focus-agent-input') || tag.parentElement) : tag.parentElement;
        if (box && !seenBoxes.has(box)) {
          seenBoxes.add(box);
          agentTags.push(tag);
        }
      });

      // For input boxes with subagent placeholder that didn't match the explicit selector
      document.querySelectorAll('[data-testid="agent-input-box"]').forEach(box => {
        if (seenBoxes.has(box)) return;
        const hasSubagentPlaceholder = box.querySelector('[placeholder*="subagent"], [data-placeholder*="subagent"]') ||
                                       (box.innerText && box.innerText.includes("Message subagent"));
        if (hasSubagentPlaceholder) {
          const candidate = Array.from(box.querySelectorAll('div')).find(d => {
            const hasIcon = d.querySelector('.google-symbols, svg');
            const hasText = d.querySelector('span.text-xs, span');
            const notBtn = !d.querySelector('button') && d.closest('button') === null;
            const notSwiss = !d.id?.startsWith("swiss-") && !d.classList.contains("swiss-context-window-ring") && !d.classList.contains("swiss-context-window-ring-sub");
            return hasIcon && hasText && notBtn && notSwiss && d.innerText.trim().length > 0;
          });
          if (candidate) {
            seenBoxes.add(box);
            agentTags.push(candidate);
          }
        }
      });

      const activeSubRings = new Set();
      agentTags.forEach((agentTag, idx) => {
        if (!agentTag || !agentTag.parentNode) return;
        const subRingId = "swiss-context-window-ring-sub-" + idx;
        let subRing = document.getElementById(subRingId);
        if (!subRing) {
          subRing = document.createElement("div");
          subRing.id = subRingId;
          subRing.classList.add("swiss-context-window-ring-sub");
        }
        activeSubRings.add(subRingId);

        let subConvId = getConversationIdFromElement(agentTag);
        if (subConvId === parentConvId) {
          subConvId = null;
        }

        const nameEl = agentTag.querySelector('span:not(.google-symbols)') ||
                       Array.from(agentTag.querySelectorAll('span')).find(s => !s.classList.contains('google-symbols')) ||
                       agentTag;
        const agentName = (nameEl ? nameEl.innerText : agentTag.innerText).trim();

        if (!subConvId && agentName && parentConvId) {
          const cachedId = subagentNameToConvId[parentConvId + ":" + agentName];
          if (cachedId) {
            subConvId = cachedId;
          } else {
            subConvId = null;
            fetchActiveConversationTokens(null, parentConvId, agentName);
          }
        } else if (isConversationId(subConvId)) {
          fetchActiveConversationTokens(subConvId);
        }

        updateProgressRing(subRing, subConvId);

        if (agentTag.parentNode && (subRing.previousElementSibling !== agentTag)) {
          agentTag.parentNode.insertBefore(subRing, agentTag.nextSibling);
        }
      });

      document.querySelectorAll("[id^='swiss-context-window-ring-sub-'], .swiss-context-window-ring-sub").forEach(r => {
        if (!activeSubRings.has(r.id)) r.remove();
      });

      // Guarantee single ring per input container
      document.querySelectorAll('[data-testid="agent-input-box"], .no-focus-agent-input').forEach(container => {
        const rings = Array.from(container.querySelectorAll('.swiss-context-window-ring, .swiss-context-window-ring-sub'));
        if (rings.length > 1) {
          for (let i = 1; i < rings.length; i++) {
            rings[i].remove();
          }
        }
      });
    }

    const isSwissLeafElement = (n) =>
      n && n.nodeType === 1 && (
        (typeof n.id === "string" && n.id.startsWith("swiss-")) ||
        n.classList?.contains("swiss-context-window-ring") ||
        n.classList?.contains("swiss-overview-divider") ||
        n.classList?.contains("swiss-overview-bottom-spacer") ||
        n.classList?.contains("swiss-overview-spacer-line") ||
        n.classList?.contains("swiss-overview-tabs-divider") ||
        n.classList?.contains("swiss-overview-tabs-pill") ||
        n.classList?.contains("swiss-overview-tabs-line") ||
        n.classList?.contains("swiss-overview-tabs-triangle") ||
        n.classList?.contains("swiss-prompt-dash") ||
        n.classList?.contains("swiss-see-triangle-btn") ||
        n.classList?.contains("swiss-project-bottom-spacer") ||
        n.classList?.contains("swiss-project-spacer-line") ||
        n.hasAttribute?.("data-swiss-divider")
      );

    const ob = new MutationObserver((mutations) => {
      if (isEnhancingOverview) return;
      const hasExternalMutation = mutations.some(m => {
        if (m.type === "childList" && (m.addedNodes.length > 0 || m.removedNodes.length > 0)) {
          const added = Array.from(m.addedNodes);
          const removed = Array.from(m.removedNodes);
          const allAddedSwiss = added.length > 0 && added.every(isSwissLeafElement);
          const allRemovedSwiss = added.length === 0 && removed.length > 0 && removed.every(isSwissLeafElement);
          if (allAddedSwiss || allRemovedSwiss) return false;
        }
        const target = m.target;
        if (target && target.nodeType === 1) {
          const el = target;
          if (isSwissLeafElement(el)) return false;
          if (el.closest && el.closest("[id^='swiss-'], .swiss-overview-tabs-divider, .swiss-overview-tabs-pill, .swiss-overview-tabs-triangle, .swiss-prompt-dash, .swiss-project-bottom-spacer, .swiss-project-spacer-line, [data-swiss-divider]")) return false;
        }
        return true;
      });
      if (!hasExternalMutation) return;

      if (scheduledRaf) return;
      scheduledRaf = requestAnimationFrame(() => {
        scheduledRaf = null;
        applyEnhancementsStyles();

        // The rail re-derives its own stable turn model; renderPromptJumpBar()
        // keeps the dashes untouched when neither the conversation nor the turn
        // total changed, and only refreshes the active highlight otherwise.
    
        applyDefaultProjectHandler();
        applyAutomationTargetProjectSelector();
        renderContextWindowProgressRing();
        isEnhancingOverview = true;
        try {
          applyOverviewPanelEnhancements();
        } finally {
          setTimeout(() => { isEnhancingOverview = false; }, 16);
        }
      });
    });
    window.__swissEnhancementsObserver = ob;
    ob.observe(document.body, { childList: true, subtree: true });

    applyEnhancementsStyles();
    renderContextWindowProgressRing();
    applyDefaultProjectHandler();
    applyAutomationTargetProjectSelector();
    installManageSidecarInterceptor();
    isEnhancingOverview = true;
    try {
      applyOverviewPanelEnhancements();
    } finally {
      setTimeout(() => { isEnhancingOverview = false; }, 16);
    }
  } catch (err) {
    console.warn("[SwissKnife] Enhancements script exception:", err);
  }
})();
`, string(cfgJSON))
}
