package enhancements

import (
	"encoding/json"
	"fmt"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
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
		cfgJSON = []byte(`{"enabled":true,"prompt_jump_bar":{"enabled":true,"show_tooltip":true,"focus_pulse":true,"sync_scroll":true,"position":"gutter","dash_width":14,"dash_thickness":2.5,"inactive_thickness":1.5,"color_mode":"default","custom_color":"#0b57d0"},"tool_density_mode":"muted","breaker_line_enabled":true,"default_new_project":"auto"}`)
	}

	tok, _ := core.GetLocalSessionToken()

	return fmt.Sprintf(`
/* === Antigravity Swiss Knife: App Enhancements === */
(() => {
  try {
    if (typeof window !== "undefined") {
      const swissToken = %q;
      if (swissToken) {
        window.__SWISS_SESSION_TOKEN__ = swissToken;
        try { sessionStorage.setItem("antigravity_swiss_session_token", swissToken); } catch (_) {}
      }
    }
    function getSwissAuthHeaders() {
      const headers = { "Content-Type": "application/json" };
      const tok = (typeof window !== "undefined" && window.__SWISS_SESSION_TOKEN__) ||
        (typeof sessionStorage !== "undefined" && sessionStorage.getItem("antigravity_swiss_session_token")) || "";
      if (tok) {
        headers["X-Session-Token"] = tok;
      }
      return headers;
    }

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
      localStorage.setItem("antigravity_swiss_community_extensions", JSON.stringify(enhConfig.community_extensions || []));
      try {
        const rawFlags = localStorage.getItem("jetski.developer.customFlagOverrides") || "{}";
        const parsedFlags = JSON.parse(rawFlags);
        if (parsedFlags && typeof parsedFlags === "object" && !Array.isArray(parsedFlags) && (parsedFlags["remote-control-setting-enabled"] !== true || parsedFlags["jetski-sidecar-declared-permissions"] !== false || parsedFlags["enable-compact-slash-command"] !== true)) {
          parsedFlags["remote-control-setting-enabled"] = true;
          parsedFlags["jetski-sidecar-declared-permissions"] = false;
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
              renderPromptJumpBar();
              applyDefaultProjectHandler();
              applyOverviewPanelEnhancements();
            }
          }

          const resGui = await fetch("http://127.0.0.1:8765/api/gui/config", {
            headers: getSwissAuthHeaders(),
            signal: (typeof AbortSignal !== "undefined" && AbortSignal.timeout) ? AbortSignal.timeout(1500) : undefined
          });
          if (resGui.ok) {
            const loadedGui = await resGui.json();
            if (loadedGui) {
              // The chat navigator style lives in the GUI config next to the
              // chat telemetry switches; missing/unknown values mean "panel".
              const nextNavigatorMode = normalizeNavigatorMode(loadedGui.chat_navigator_mode);
              if (nextNavigatorMode !== chatNavigatorMode) {
                chatNavigatorMode = nextNavigatorMode;
                // "line" drops the panel immediately; "panel" lets the next
                // rail hover open it.
                closePromptNavigatorPanel();
              }
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

    let isClickJumping = false;
    let clickTargetIdx = -1;
    let jumpTimer = null;

    let lastJumpBarConversationKey = null;
    let lastJumpBarTotal = -1;
    let lastActiveIdx = -1;
    let lastAppliedActiveColor = "";
    let lastAppliedDefaultColor = "";
    let lastAppliedDashWidth = -1;
    let lastAppliedActiveThickness = -1;
    let lastAppliedInactiveThickness = -1;

    // Chat navigator hover panel state (issue #162). The mode mirrors the GUI
    // config ("panel" default, "line" keeps the dash-only rail); the text cache
    // stores the prompt of every turn that ever mounted, keyed by
    // (conversation, turn ordinal), because the transcript is virtualized.
    let chatNavigatorMode = "panel";
    let navigatorPanelOpenTimer = null;
    let navigatorPanelCloseTimer = null;
    let navigatorPanelDismissTargets = [];
    const jumpBarPromptTextCache = {};

    /* __SWISS_JUMP_BAR_HELPERS_START__ */
    // Pure helpers for the Quick Prompt Jump Bar (Turn Jump Navigator).
    // The chat transcript is a virtualized list: only the prompt turns near the
    // viewport stay mounted, so counting [data-testid="user-input-step"] nodes
    // directly makes the number of bars fluctuate and under-report the session.
    // These helpers resolve ONE stable turn total per conversation, in a fixed
    // order, from the best available source:
    //   1. list ordinals of the mounted turns (data-index / aria-posinset),
    //   2. the full transcript turn count reported by the daemon chat-metrics,
    //   3. a per-conversation high-water mark so the rail never shrinks mid-scroll.
    const jumpBarTurnRegistry = {};
    const jumpBarTurnCountsByConversation = {};

    function computeTurnOrdinals(stepEls) {
      const ordinals = [];
      if (!stepEls) return ordinals;
      for (let i = 0; i < stepEls.length; i++) {
        const el = stepEls[i];
        let ordinal = NaN;
        try {
          const attr = el && el.getAttribute ? (el.getAttribute("data-index") || el.getAttribute("aria-posinset")) : null;
          if (attr !== null && attr !== undefined && attr !== "") ordinal = parseInt(attr, 10);
          if (!isFinite(ordinal) && el && el.closest) {
            const holder = el.closest("[data-index]");
            if (holder && holder !== el) {
              const holderIdx = holder.getAttribute("data-index");
              if (holderIdx !== null && holderIdx !== undefined && holderIdx !== "") ordinal = parseInt(holderIdx, 10);
            }
          }
        } catch (_) { ordinal = NaN; }
        ordinals.push((isFinite(ordinal) && ordinal >= 0) ? ordinal : null);
      }
      return ordinals;
    }

    function computeStableTurnTotal(ordinals, mountedCount, daemonTurnCount, conversationKey, registry) {
      let total = mountedCount || 0;
      if (ordinals) {
        for (let i = 0; i < ordinals.length; i++) {
          const ord = ordinals[i];
          if (typeof ord === "number" && ord + 1 > total) total = ord + 1;
        }
      }
      if (typeof daemonTurnCount === "number" && daemonTurnCount > total) total = daemonTurnCount;
      const key = conversationKey || "__unknown__";
      const previous = registry[key] || 0;
      const stable = Math.max(total, previous);
      if (stable > 0) registry[key] = stable;
      return stable;
    }

    function mapStepsToTurnIndexes(ordinals, total) {
      const used = [];
      const mapped = [];
      if (!ordinals) return mapped;
      for (let i = 0; i < ordinals.length; i++) {
        const ord = ordinals[i];
        let idx = (typeof ord === "number" && ord >= 0 && ord < total && used.indexOf(ord) === -1) ? ord : -1;
        if (idx === -1) {
          idx = 0;
          while (used.indexOf(idx) !== -1) idx++;
        }
        used.push(idx);
        mapped.push(idx);
      }
      return mapped;
    }
    /* __SWISS_JUMP_BAR_HELPERS_END__ */

    /* __SWISS_NAV_PANEL_HELPERS_START__ */
    // Pure helpers for the chat navigator hover panel (issue #162).
    // The rail keeps one dash per turn; the panel lists the same turns as one
    // row each. The transcript is virtualized, so prompt texts are cached by
    // (conversation, turn ordinal) while they are mounted; turns that were
    // never mounted render a muted ellipsis instead of fabricated text.
    const NAV_PANEL_GRACE_MS = 150;
    const NAV_PANEL_MAX_TEXT = 200;
    const NAV_PANEL_UNKNOWN_TEXT = "…";

    function normalizeNavigatorMode(mode) {
      return mode === "line" ? "line" : "panel";
    }

    function promptTextCacheKey(conversationKey, ordinal) {
      return (conversationKey || "__unknown__") + "\u0000" + String(ordinal);
    }

    function condensePromptText(rawText) {
      const collapsed = (rawText === null || rawText === undefined ? "" : String(rawText)).replace(/\s+/g, " ").trim();
      if (!collapsed) return "";
      if (collapsed.length <= NAV_PANEL_MAX_TEXT) return collapsed;
      return collapsed.slice(0, NAV_PANEL_MAX_TEXT) + "…";
    }

    function cachePromptText(cache, conversationKey, ordinal, rawText) {
      if (!cache || typeof ordinal !== "number" || ordinal < 0) return "";
      const condensed = condensePromptText(rawText);
      if (!condensed) return "";
      cache[promptTextCacheKey(conversationKey, ordinal)] = condensed;
      return condensed;
    }

    function getCachedPromptText(cache, conversationKey, ordinal) {
      if (!cache || typeof ordinal !== "number" || ordinal < 0) return NAV_PANEL_UNKNOWN_TEXT;
      return cache[promptTextCacheKey(conversationKey, ordinal)] || NAV_PANEL_UNKNOWN_TEXT;
    }

    // One menu row: a small dash marker plus the single-line, left-aligned
    // prompt text. Text goes through textContent, never innerHTML.
    function createNavigatorRow(doc, turnIdx, text, isActive, ui, onActivate) {
      const row = doc.createElement("div");
      row.className = "swiss-navigator-row" + (isActive ? " swiss-navigator-row-active" : "");
      row.setAttribute("role", "menuitem");
      row.setAttribute("tabindex", "-1");
      row.setAttribute("data-prompt-index", String(turnIdx));
      row.style.cssText = "display: flex; align-items: center; gap: 8px; padding: 4px 8px; border-radius: 6px; cursor: pointer; text-align: left; user-select: none;";
      row.style.background = isActive ? ui.activeRowBackground : "transparent";

      const marker = doc.createElement("span");
      marker.className = "swiss-navigator-row-dash" + (isActive ? " active" : "");
      marker.style.cssText = "flex: 0 0 auto; display: block; border-radius: 2px; transition: width 0.18s cubic-bezier(0.4, 0, 0.2, 1), background 0.15s ease, height 0.15s ease;";
      marker.style.width = ui.dashWidth + "px";
      marker.style.height = (isActive ? ui.activeThickness : ui.inactiveThickness) + "px";
      marker.style.background = isActive ? ui.activeColor : ui.defaultColor;

      const hasText = !!(text && String(text).trim());
      const label = doc.createElement("span");
      label.className = "swiss-navigator-row-text";
      label.textContent = hasText ? text : NAV_PANEL_UNKNOWN_TEXT;
      label.style.cssText = "flex: 1 1 auto; min-width: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; text-align: left; font-size: 12px; line-height: 1.5;";
      if (!hasText) label.style.color = ui.mutedColor;

      row.appendChild(marker);
      row.appendChild(label);
      row.onclick = () => { if (onActivate) onActivate(turnIdx); };

      const hoverWidth = ui.hoverDashWidth || Math.max(22, (ui.dashWidth || 14) + 8);
      row.onmouseenter = () => {
        row.classList.add("swiss-navigator-row-hover");
        marker.style.width = hoverWidth + "px";
        marker.style.background = ui.activeColor;
        if (!row.classList.contains("swiss-navigator-row-active")) {
          row.style.background = ui.hoverRowBackground || ui.activeRowBackground;
        }
      };
      row.onmouseleave = () => {
        row.classList.remove("swiss-navigator-row-hover");
        const currentlyActive = row.classList.contains("swiss-navigator-row-active");
        marker.style.width = ui.dashWidth + "px";
        marker.style.background = currentlyActive ? ui.activeColor : ui.defaultColor;
        row.style.background = currentlyActive ? ui.activeRowBackground : "transparent";
      };
      return row;
    }
    /* __SWISS_NAV_PANEL_HELPERS_END__ */

    function resolveJumpBarConversationKey(hostEl) {
      try {
        const fiberId = getConversationIdFromElement(hostEl);
        if (fiberId) return fiberId;
      } catch (_) {}
      try {
        if (window.location && window.location.pathname) {
          const m = window.location.pathname.match(/\/c\/([^/?#]+)/);
          if (m && m[1]) return m[1];
          return window.location.pathname;
        }
      } catch (_) {}
      return "";
    }

    function getJumpBarSteps(bar) {
      const host = bar && bar.parentElement;
      let steps = [];
      try {
        const root = (host && host.querySelectorAll) ? host : document;
        steps = Array.from(root.querySelectorAll('[data-testid="user-input-step"]'));
      } catch (_) {
        steps = [];
      }
      if (!steps.length) {
        try {
          steps = Array.from(document.querySelectorAll('[data-testid="user-input-step"]'));
        } catch (_) {}
      }
      return steps;
    }

    function resolveActiveTurnIndex(steps, stepTurnIndexes, viewport) {
      // A click jump pins the active dash to the clicked turn until scrolling settles.
      if (isClickJumping && clickTargetIdx >= 0) return clickTargetIdx;
      if (!steps || !steps.length || !stepTurnIndexes || !stepTurnIndexes.length) return -1;
      const lowestStep = getLowestPromptOnScreen(steps, viewport);
      if (lowestStep >= 0 && lowestStep < stepTurnIndexes.length) return stepTurnIndexes[lowestStep];
      return -1;
    }

    // Returns the index (into the steps array) of the lowest prompt turn on screen.
    function getLowestPromptOnScreen(steps, viewport) {
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

    // Prompt text reaches this tooltip as user input: escape it before it is
    // interpolated into innerHTML, otherwise a prompt containing markup would
    // execute inside the host page.
    function escapeNavigatorText(text) {
      return String(text == null ? "" : text)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#39;");
    }

    function renderPromptJumpBar() {
      const isEnabled = enhConfig.enabled && enhConfig.prompt_jump_bar?.enabled;
      const navigatorMode = normalizeNavigatorMode(chatNavigatorMode);
      let bar = document.getElementById("swiss-prompt-jump-bar");
      let tooltip = document.getElementById("swiss-prompt-tooltip");

      if (!isEnabled) {
        if (bar) bar.remove();
        if (tooltip) tooltip.remove();
        closePromptNavigatorPanel();
        lastActiveIdx = -1;
        lastJumpBarTotal = -1;
        lastJumpBarConversationKey = null;
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
      // Keyboard support is a panel-mode feature; "line" stays exactly the
      // dash-only rail of before (no extra tab stop).
      if (navigatorMode === "panel") {
        bar.setAttribute("tabindex", "0");
        bar.setAttribute("aria-label", "Chat navigator — prompt turns");
        bar.setAttribute("aria-haspopup", "menu");
      } else {
        bar.removeAttribute("tabindex");
        bar.removeAttribute("aria-haspopup");
      }

      // Resolve one stable turn total for this conversation; steps are scoped to
      // the pane/transcript that hosts the rail.
      const steps = getJumpBarSteps(bar);
      const ordinals = computeTurnOrdinals(steps);
      const conversationKey = resolveJumpBarConversationKey(bar.parentElement);
      const daemonTurnCount = (conversationKey && jumpBarTurnCountsByConversation[conversationKey]) || 0;
      const totalTurns = computeStableTurnTotal(ordinals, steps.length, daemonTurnCount, conversationKey, jumpBarTurnRegistry);
      if (totalTurns === 0) {
        bar.style.display = "none";
        closePromptNavigatorPanel();
        lastJumpBarConversationKey = conversationKey;
        lastJumpBarTotal = 0;
        return;
      }
      const stepTurnIndexes = mapStepsToTurnIndexes(ordinals, totalTurns);

      // The transcript virtualizes turns away, so remember the prompt text of
      // every mounted turn by (conversation, turn index) while the rail
      // refreshes. The hover panel reads this cache; unmounted turns show "…".
      for (let i = 0; i < stepTurnIndexes.length; i++) {
        const step = steps[i];
        if (!step) continue;
        const textEl = step.querySelector('.whitespace-pre-wrap') || step;
        cachePromptText(jumpBarPromptTextCache, conversationKey, stepTurnIndexes[i], textEl ? textEl.textContent : "");
      }

      bar.style.display = "flex";
      if (navigatorMode !== "panel") {
        // "line" mode is the dash-only rail: never leave a stale panel behind.
        closePromptNavigatorPanel();
      }

      const dark = isDarkMode();
      const isGutter = enhConfig.prompt_jump_bar?.position !== "floating";
      const dashWidth = enhConfig.prompt_jump_bar?.dash_width || 14;
      const activeThickness = enhConfig.prompt_jump_bar?.dash_thickness || 2.5;
      const inactiveThickness = enhConfig.prompt_jump_bar?.inactive_thickness || 1.5;

      if (isGutter && convView) {
        bar.style.cssText = "position: absolute; left: 18px; top: 28px; z-index: 45; display: flex; flex-direction: column; gap: 0px; padding: 4px 6px; background: transparent; border: none; box-shadow: none; user-select: none; pointer-events: auto;";
      } else {
        bar.style.cssText = "position: fixed; left: 268px; top: 120px; z-index: 9999; display: flex; flex-direction: column; gap: 0px; padding: 4px 6px; background: transparent; border: none; box-shadow: none; user-select: none; pointer-events: auto;";
      }

      bar.onmouseenter = () => { scheduleOpenPromptNavigatorPanel(); };
      bar.onmouseleave = () => { scheduleClosePromptNavigatorPanel(); };

      if (getPromptNavigatorPanel() || bar.getAttribute("data-swiss-panel-open") === "true") {
        bar.style.opacity = "0";
        bar.style.visibility = "hidden";
        bar.style.pointerEvents = "none";
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
        if (
          activeIdx === lastActiveIdx &&
          activeColor === lastAppliedActiveColor &&
          defaultColor === lastAppliedDefaultColor &&
          dashWidth === lastAppliedDashWidth &&
          activeThickness === lastAppliedActiveThickness &&
          inactiveThickness === lastAppliedInactiveThickness
        ) {
          return;
        }

        const dashes = bar.querySelectorAll(".swiss-prompt-dash");
        dashes.forEach((d, idx) => {
          d.style.width = dashWidth + "px";
          if (idx === activeIdx) {
            d.classList.add("active");
            d.style.height = activeThickness + "px";
            d.style.background = activeColor;
          } else {
            d.classList.remove("active");
            d.style.height = inactiveThickness + "px";
            d.style.background = defaultColor;
          }
        });

        lastActiveIdx = activeIdx;
        lastAppliedActiveColor = activeColor;
        lastAppliedDefaultColor = defaultColor;
        lastAppliedDashWidth = dashWidth;
        lastAppliedActiveThickness = activeThickness;
        lastAppliedInactiveThickness = inactiveThickness;
        updateNavigatorPanelActiveHighlight(activeIdx);
      }

      // One bar per prompt turn, in fixed order (oldest first). The DOM is only
      // rebuilt when the conversation or the stable turn total changes, so the
      // rail no longer drops bars or renumbers them while the virtualized
      // transcript mounts and unmounts turns during scrolling and streaming.
      const existingDashes = bar.querySelectorAll(".swiss-prompt-dash");
      if (existingDashes.length === totalTurns &&
          lastJumpBarTotal === totalTurns &&
          lastJumpBarConversationKey === conversationKey) {
        updateActiveIndicator(resolveActiveTurnIndex(steps, stepTurnIndexes, viewport));
        return;
      }

      bar.innerHTML = "";
      // The rail is rebuilt (conversation switch or new stable total): the panel
      // rows are stale, so drop the panel; the next rail hover re-opens it.
      closePromptNavigatorPanel();
      lastJumpBarConversationKey = conversationKey;
      lastJumpBarTotal = totalTurns;

      const stepByTurnIndex = {};
      stepTurnIndexes.forEach((turnIdx, stepIdx) => { stepByTurnIndex[turnIdx] = steps[stepIdx]; });

      for (let turnIdx = 0; turnIdx < totalTurns; turnIdx++) {
        const step = stepByTurnIndex[turnIdx];
        const textEl = step ? (step.querySelector('.whitespace-pre-wrap') || step) : null;
        const fullText = textEl ? (textEl.textContent || "").replace(/\s+/g, ' ').trim() : "";
        const snippet = fullText.length > 75 ? fullText.substring(0, 75) + "..." : fullText;

        const dash = document.createElement("div");
        dash.className = "swiss-prompt-dash";
        dash.setAttribute("data-prompt-index", turnIdx);

        // Height is explicitly fixed without transition to prevent bouncing / shining
        dash.style.cssText = "width: " + dashWidth + "px; height: " + inactiveThickness + "px; padding: 3.5px 6px; box-sizing: content-box; background-clip: content-box; border-radius: 2px; background: " + defaultColor + "; cursor: pointer; pointer-events: auto; transition: width 0.18s cubic-bezier(0.4, 0, 0.2, 1), background 0.15s ease;";

        dash.onmouseenter = () => {
          const curWidth = enhConfig.prompt_jump_bar?.dash_width || 14;
          const hoverWidth = Math.max(22, curWidth + 8);
          dash.style.width = hoverWidth + "px";
          dash.style.background = activeColor;
          scheduleOpenPromptNavigatorPanel();
          if (tooltip && navigatorMode === "line") {
            const rect = dash.getBoundingClientRect();
            const snippetHTML = snippet
              ? "<div style='color: var(--foreground, #101010); font-size: 12px; font-weight: 400;'>" + escapeNavigatorText(snippet) + "</div>"
              : "<div style='color: var(--muted-foreground, #64748b); font-size: 12px; font-weight: 400;'>Jumps to this turn</div>";
            tooltip.innerHTML = "<div style='font-weight: 500; font-size: 11px; color: var(--muted-foreground, #64748b); margin-bottom: 2px;'>Prompt #" + (turnIdx + 1) + (dash.classList.contains("active") ? " (Current)" : "") + "</div>" + snippetHTML;
            tooltip.style.left = (rect.right + 12) + "px";
            tooltip.style.top = (rect.top - 10) + "px";
            tooltip.style.display = "block";
          }
        };

        dash.onmouseleave = () => {
          const curWidth = enhConfig.prompt_jump_bar?.dash_width || 14;
          dash.style.width = curWidth + "px";
          const cur = dash.classList.contains("active");
          dash.style.background = cur ? activeColor : defaultColor;
          if (tooltip) tooltip.style.display = "none";
          scheduleClosePromptNavigatorPanel();
        };

        dash.onclick = () => {
          isClickJumping = true;
          clickTargetIdx = turnIdx;
          if (jumpTimer) clearTimeout(jumpTimer);

          updateActiveIndicator(turnIdx);

          if (step) {
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
          } else if (viewport) {
            // The turn is virtualized out of the DOM: scroll the transcript
            // proportionally so the list mounts that region, then re-sync.
            const maxTop = Math.max(0, (viewport.scrollHeight || 0) - (viewport.clientHeight || 0));
            const ratio = totalTurns > 1 ? (turnIdx / (totalTurns - 1)) : 0;
            try {
              if (typeof viewport.scrollTo === "function") {
                viewport.scrollTo({ top: ratio * maxTop, behavior: "smooth" });
              } else {
                viewport.scrollTop = ratio * maxTop;
              }
            } catch (_) {
              viewport.scrollTop = ratio * maxTop;
            }
          }

          jumpTimer = setTimeout(() => {
            isClickJumping = false;
            clickTargetIdx = -1;
            renderPromptJumpBar();
          }, 1000);
        };

        bar.appendChild(dash);
      }

      lastActiveIdx = -1; // Force indicator sync on fresh render
      updateActiveIndicator(resolveActiveTurnIndex(steps, stepTurnIndexes, viewport));
    }

    /* === Chat Navigator hover panel (issue #162) ===
     * The rail keeps its dashes; in "panel" mode hovering it opens a panel to
     * its RIGHT that lists every turn (dash marker + one-line prompt text).
     * The panel is a fixed-position element attached to document.body so the
     * chat pane's overflow clipping cannot cut it off; it is removed entirely
     * when closed (never intercepts pointer events while closed) and dropped
     * again on every rail rebuild or teardown. */
    /* __SWISS_NAV_PANEL_LOGIC_START__ */
    function isNavigatorPanelMode() {
      return normalizeNavigatorMode(chatNavigatorMode) === "panel";
    }

    function getPromptNavigatorPanel() {
      return document.getElementById("swiss-prompt-navigator-panel");
    }

    function navigatorPanelDefaultColor() {
      return isDarkMode() ? "rgba(148, 163, 184, 0.45)" : "rgba(100, 116, 139, 0.42)";
    }

    function navigatorPanelActiveRowBackground() {
      return isDarkMode() ? "rgba(148, 163, 184, 0.16)" : "rgba(100, 116, 139, 0.12)";
    }

    function buildNavigatorPanelUI() {
      return {
        dashWidth: enhConfig.prompt_jump_bar?.dash_width || 14,
        hoverDashWidth: Math.max(22, (enhConfig.prompt_jump_bar?.dash_width || 14) + 8),
        activeThickness: enhConfig.prompt_jump_bar?.dash_thickness || 2.5,
        inactiveThickness: enhConfig.prompt_jump_bar?.inactive_thickness || 1.5,
        activeColor: getActiveAndHoverColor(),
        defaultColor: navigatorPanelDefaultColor(),
        activeRowBackground: navigatorPanelActiveRowBackground(),
        hoverRowBackground: isDarkMode() ? "rgba(148, 163, 184, 0.12)" : "rgba(100, 116, 139, 0.08)",
        mutedColor: "var(--muted-foreground, #64748b)"
      };
    }

    function detachNavigatorPanelDismissListeners() {
      for (let i = 0; i < navigatorPanelDismissTargets.length; i++) {
        const entry = navigatorPanelDismissTargets[i];
        try { entry.target.removeEventListener("scroll", entry.handler, entry.options); } catch (_) {}
      }
      navigatorPanelDismissTargets = [];
    }

    // Scrolls inside the panel itself (overflow-y) must not close it.
    function onNavigatorPanelOutsideScroll(e) {
      const panel = getPromptNavigatorPanel();
      if (panel && e && e.target && typeof panel.contains === "function" && panel.contains(e.target)) return;
      closePromptNavigatorPanel();
    }

    function attachNavigatorPanelDismissListeners() {
      detachNavigatorPanelDismissListeners();
      const viewport = document.querySelector('[data-testid="autoscroll-viewport"]') ||
                       document.querySelector('.overflow-y-auto');
      if (viewport && typeof viewport.addEventListener === "function") {
        viewport.addEventListener("scroll", onNavigatorPanelOutsideScroll, { passive: true });
        navigatorPanelDismissTargets.push({ target: viewport, handler: onNavigatorPanelOutsideScroll, options: { passive: true } });
      }
      if (typeof window !== "undefined" && typeof window.addEventListener === "function") {
        window.addEventListener("scroll", onNavigatorPanelOutsideScroll, { passive: true, capture: true });
        navigatorPanelDismissTargets.push({ target: window, handler: onNavigatorPanelOutsideScroll, options: { passive: true, capture: true } });
      }
    }

    function closePromptNavigatorPanel() {
      if (navigatorPanelOpenTimer) {
        clearTimeout(navigatorPanelOpenTimer);
        navigatorPanelOpenTimer = null;
      }
      if (navigatorPanelCloseTimer) {
        clearTimeout(navigatorPanelCloseTimer);
        navigatorPanelCloseTimer = null;
      }
      const panel = getPromptNavigatorPanel();
      if (panel && typeof panel.remove === "function") panel.remove();
      detachNavigatorPanelDismissListeners();
      const bar = document.getElementById("swiss-prompt-jump-bar");
      if (bar) {
        bar.removeAttribute("data-swiss-panel-open");
        bar.style.opacity = "1";
        bar.style.visibility = "visible";
        bar.style.pointerEvents = "auto";
      }
    }

    function cancelNavigatorPanelClose() {
      if (navigatorPanelCloseTimer) {
        clearTimeout(navigatorPanelCloseTimer);
        navigatorPanelCloseTimer = null;
      }
    }

    // Closing waits NAV_PANEL_GRACE_MS so the pointer can travel from the rail
    // to the panel (10px gap) without the panel vanishing under it.
    function scheduleClosePromptNavigatorPanel() {
      if (navigatorPanelOpenTimer) {
        clearTimeout(navigatorPanelOpenTimer);
        navigatorPanelOpenTimer = null;
      }
      if (!getPromptNavigatorPanel()) return;
      cancelNavigatorPanelClose();
      navigatorPanelCloseTimer = setTimeout(() => {
        navigatorPanelCloseTimer = null;
        closePromptNavigatorPanel();
      }, NAV_PANEL_GRACE_MS);
    }

    // Opening also waits the grace delay (hover intent): crossing the 7px gap
    // between two dashes cancels the pending close and keeps the panel up.
    function scheduleOpenPromptNavigatorPanel() {
      if (!isNavigatorPanelMode()) return;
      cancelNavigatorPanelClose();
      if (getPromptNavigatorPanel()) return;
      if (navigatorPanelOpenTimer) return;
      navigatorPanelOpenTimer = setTimeout(() => {
        navigatorPanelOpenTimer = null;
        openPromptNavigatorPanel();
      }, NAV_PANEL_GRACE_MS);
    }

    // Reuses the dash click path so row clicks and dash clicks stay in lockstep.
    function triggerNavigatorJump(turnIdx) {
      const bar = document.getElementById("swiss-prompt-jump-bar");
      if (!bar) return false;
      const dash = bar.querySelector('.swiss-prompt-dash[data-prompt-index="' + turnIdx + '"]');
      if (!dash || typeof dash.click !== "function") return false;
      dash.click();
      return true;
    }

    function positionPromptNavigatorPanel(panel, bar) {
      if (!panel || !bar || typeof bar.getBoundingClientRect !== "function") return;
      const rect = bar.getBoundingClientRect();
      const vw = (typeof window !== "undefined" && window.innerWidth) || 1024;
      const vh = (typeof window !== "undefined" && window.innerHeight) || 768;
      const margin = 8;
      const panelWidth = panel.offsetWidth || 320;

      let left = rect.left;
      left = Math.max(margin, Math.min(left, Math.max(margin, vw - panelWidth - margin)));

      const cap = Math.max(120, Math.min(vh * 0.6, vh - 2 * margin));
      panel.style.maxHeight = cap + "px";
      const panelHeight = Math.min(panel.offsetHeight || 0, cap);
      let top = rect.top;
      if (top + panelHeight > vh - margin) top = vh - margin - panelHeight;
      top = Math.max(margin, top);

      panel.style.left = left + "px";
      panel.style.top = top + "px";
      panel.style.maxHeight = Math.max(120, Math.min(vh * 0.6, vh - top - margin)) + "px";
    }

    function updateNavigatorPanelActiveHighlight(activeIdx) {
      const panel = getPromptNavigatorPanel();
      if (!panel) return;
      const ui = buildNavigatorPanelUI();
      panel.querySelectorAll(".swiss-navigator-row").forEach((row) => {
        const idx = parseInt(row.getAttribute("data-prompt-index"), 10);
        const isActive = idx === activeIdx;
        if (isActive) {
          row.classList.add("swiss-navigator-row-active");
        } else {
          row.classList.remove("swiss-navigator-row-active");
        }
        row.style.background = isActive ? ui.activeRowBackground : "transparent";
        const marker = row.querySelector(".swiss-navigator-row-dash");
        if (marker) {
          if (isActive) {
            marker.classList.add("active");
          } else {
            marker.classList.remove("active");
          }
          marker.style.height = (isActive ? ui.activeThickness : ui.inactiveThickness) + "px";
          marker.style.background = isActive ? ui.activeColor : ui.defaultColor;
        }
      });
    }

    function openPromptNavigatorPanel() {
      if (!isNavigatorPanelMode()) return;
      const bar = document.getElementById("swiss-prompt-jump-bar");
      if (!bar || bar.style.display === "none") return;
      const dashes = bar.querySelectorAll(".swiss-prompt-dash");
      if (!dashes || !dashes.length) return;

      closePromptNavigatorPanel();

      bar.setAttribute("data-swiss-panel-open", "true");
      bar.style.opacity = "0";
      bar.style.visibility = "hidden";
      bar.style.pointerEvents = "none";

      const conversationKey = lastJumpBarConversationKey || "";
      const totalTurns = lastJumpBarTotal > 0 ? lastJumpBarTotal : dashes.length;
      let activeIdx = -1;
      dashes.forEach((d) => {
        if (d.classList.contains("active")) {
          const idx = parseInt(d.getAttribute("data-prompt-index"), 10);
          if (!isNaN(idx)) activeIdx = idx;
        }
      });

      const ui = buildNavigatorPanelUI();
      const panel = document.createElement("div");
      panel.id = "swiss-prompt-navigator-panel";
      panel.setAttribute("data-swiss-component", "prompt-navigator-panel");
      panel.setAttribute("role", "menu");
      panel.setAttribute("aria-label", "Prompt turns");
      panel.style.cssText = "position: fixed; z-index: 10001; width: 320px; max-width: calc(100vw - 24px); overflow-y: auto; overflow-x: hidden; padding: 4px; box-sizing: border-box; background: var(--card, #ffffff); color: var(--foreground, #101010); border: 1px solid var(--border, rgba(0,0,0,0.08)); border-radius: 8px; box-shadow: 0 8px 24px rgba(0,0,0,0.16); font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; pointer-events: auto;";
      document.body.appendChild(panel);

      for (let turnIdx = 0; turnIdx < totalTurns; turnIdx++) {
        const cached = getCachedPromptText(jumpBarPromptTextCache, conversationKey, turnIdx);
        const known = cached !== NAV_PANEL_UNKNOWN_TEXT;
        const row = createNavigatorRow(document, turnIdx, known ? cached : "", turnIdx === activeIdx, ui, triggerNavigatorJump);
        panel.appendChild(row);
      }

      panel.onmouseenter = cancelNavigatorPanelClose;
      panel.onmouseleave = scheduleClosePromptNavigatorPanel;

      positionPromptNavigatorPanel(panel, bar);
      attachNavigatorPanelDismissListeners();

      // Long conversations: start the list at the current turn.
      const activeRow = panel.querySelector(".swiss-navigator-row-active");
      if (activeRow && typeof activeRow.scrollIntoView === "function") {
        try { activeRow.scrollIntoView({ block: "nearest" }); } catch (_) {}
      }
    }

    function focusNavigatorRow(panel, delta) {
      const rows = Array.from(panel.querySelectorAll(".swiss-navigator-row"));
      if (!rows.length) return;
      let idx = -1;
      try {
        const focused = document.activeElement;
        if (focused) idx = rows.indexOf(focused);
      } catch (_) {}
      if (idx === -1) {
        const activeRow = panel.querySelector(".swiss-navigator-row-active");
        idx = activeRow ? rows.indexOf(activeRow) : -1;
      }
      if (idx === -1) idx = delta > 0 ? -1 : 0;
      let next = idx + delta;
      if (next < 0) next = rows.length - 1;
      if (next >= rows.length) next = 0;
      const target = rows[next];
      if (target && typeof target.focus === "function") {
        try { target.focus(); } catch (_) {}
      }
    }

    // Keyboard support: focus the rail, ArrowDown/ArrowUp opens and walks the
    // rows, Enter jumps, Escape closes (and returns focus to the rail).
    function handleNavigatorKeydown(e) {
      if (!e || !e.key) return;
      const panel = getPromptNavigatorPanel();
      const bar = document.getElementById("swiss-prompt-jump-bar");
      if (!bar) return;
      let onRail = false;
      let onPanel = false;
      try {
        onRail = e.target === bar || (typeof bar.contains === "function" && bar.contains(e.target));
        onPanel = !!(panel && e.target && typeof panel.contains === "function" && panel.contains(e.target));
      } catch (_) {}

      if (e.key === "Escape") {
        if (panel) {
          // Escape closes the panel wherever focus is; only steal the key when
          // the user is actually inside the navigator.
          if (onPanel || onRail) e.preventDefault();
          closePromptNavigatorPanel();
          if ((onPanel || onRail) && typeof bar.focus === "function") {
            try { bar.focus(); } catch (_) {}
          }
        }
        return;
      }

      if (!isNavigatorPanelMode()) return;
      if (!onRail && !onPanel) return;

      if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        e.preventDefault();
        const delta = e.key === "ArrowDown" ? 1 : -1;
        if (!panel) {
          openPromptNavigatorPanel();
          const opened = getPromptNavigatorPanel();
          if (opened) focusNavigatorRow(opened, delta);
        } else {
          focusNavigatorRow(panel, delta);
        }
        return;
      }

      if (e.key === "Enter") {
        if (onPanel) {
          const row = e.target && typeof e.target.closest === "function" ? e.target.closest('[role="menuitem"]') : null;
          if (row) {
            e.preventDefault();
            const idx = parseInt(row.getAttribute("data-prompt-index"), 10);
            if (!isNaN(idx)) triggerNavigatorJump(idx);
          }
          return;
        }
        if (onRail) {
          e.preventDefault();
          if (!panel) {
            openPromptNavigatorPanel();
            return;
          }
          const activeRow = panel.querySelector(".swiss-navigator-row-active");
          if (activeRow) {
            const idx = parseInt(activeRow.getAttribute("data-prompt-index"), 10);
            if (!isNaN(idx)) triggerNavigatorJump(idx);
          }
        }
      }
    }
    /* __SWISS_NAV_PANEL_LOGIC_END__ */

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

    const updateScrollHandler = () => {
      if (isClickJumping) return;
      requestAnimationFrame(() => {
        const bar = document.getElementById("swiss-prompt-jump-bar");
        if (!bar) return;
        const steps = getJumpBarSteps(bar);
        const viewport = document.querySelector('[data-testid="autoscroll-viewport"]') ||
                         document.querySelector('.overflow-y-auto');

        // Re-derive the stable turn model; if the conversation or the total moved,
        // rebuild the rail instead of re-labelling a stale set of dashes.
        const ordinals = computeTurnOrdinals(steps);
        const conversationKey = resolveJumpBarConversationKey(bar.parentElement);
        const daemonTurnCount = (conversationKey && jumpBarTurnCountsByConversation[conversationKey]) || 0;
        const totalTurns = computeStableTurnTotal(ordinals, steps.length, daemonTurnCount, conversationKey, jumpBarTurnRegistry);
        if (totalTurns === 0 || totalTurns !== lastJumpBarTotal || conversationKey !== lastJumpBarConversationKey) {
          renderPromptJumpBar();
          return;
        }
        const stepTurnIndexes = mapStepsToTurnIndexes(ordinals, totalTurns);
        const activeIdx = resolveActiveTurnIndex(steps, stepTurnIndexes, viewport);
        const dark = isDarkMode();
        const defaultColor = dark ? "rgba(148, 163, 184, 0.45)" : "rgba(100, 116, 139, 0.42)";
        const activeColor = getActiveAndHoverColor();
        const dashWidth = enhConfig.prompt_jump_bar?.dash_width || 14;
        const activeThickness = enhConfig.prompt_jump_bar?.dash_thickness || 2.5;
        const inactiveThickness = enhConfig.prompt_jump_bar?.inactive_thickness || 1.5;

        if (
          activeIdx === lastActiveIdx &&
          activeColor === lastAppliedActiveColor &&
          defaultColor === lastAppliedDefaultColor &&
          dashWidth === lastAppliedDashWidth &&
          activeThickness === lastAppliedActiveThickness &&
          inactiveThickness === lastAppliedInactiveThickness
        ) {
          return;
        }

        bar.querySelectorAll(".swiss-prompt-dash").forEach((d, idx) => {
          d.style.width = dashWidth + "px";
          if (idx === activeIdx) {
            d.classList.add("active");
            d.style.height = activeThickness + "px";
            d.style.background = activeColor;
          } else {
            d.classList.remove("active");
            d.style.height = inactiveThickness + "px";
            d.style.background = defaultColor;
          }
        });

        lastActiveIdx = activeIdx;
        lastAppliedActiveColor = activeColor;
        lastAppliedDefaultColor = defaultColor;
        lastAppliedDashWidth = dashWidth;
        lastAppliedActiveThickness = activeThickness;
        lastAppliedInactiveThickness = inactiveThickness;
        updateNavigatorPanelActiveHighlight(activeIdx);
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

    const handleKeydown = (e) => {
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

      // Chat navigator panel: keyboard support while the rail (or the panel) has focus.
      if (typeof document !== "undefined" && typeof document.addEventListener === "function") {
        document.addEventListener("keydown", (e) => handleNavigatorKeydown(e), true);
      }
    }

    if (window.__swissEnhancementsObserver && typeof window.__swissEnhancementsObserver.disconnect === "function") {
      try { window.__swissEnhancementsObserver.disconnect(); } catch (_) {}
    }

    /* === Active Conversation Auto-Revival & Continuation Handler === */
    let autoRevivalInFlight = false;
    let lastRevivalCheckTime = 0;

    async function ackContinuation(cascadeId) {
      try {
        localStorage.removeItem("antigravity_swiss_pending_continuation");
        if (typeof window !== "undefined") {
          window.__swissPendingContinuation = null;
          window.postMessage({ type: "swiss-clear-pending-continuation", cascade_id: cascadeId }, "*");
        }
        if (typeof window !== "undefined" && window.fetch) {
          await fetch("http://127.0.0.1:8765/api/conversations/ack", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ cascade_id: cascadeId })
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

    async function checkAndExecuteConversationRevival() {
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
        let pending = null;
        try {
          if (typeof window !== "undefined" && window.fetch) {
            let statusUrl = "http://127.0.0.1:8765/api/conversations/status";
            if (uuidRegex.test(curID)) {
              statusUrl += "?conversation_id=" + encodeURIComponent(curID);
            }
            const res = await fetch(statusUrl);
            if (res.ok) {
              const data = await res.json();
              if (data && data.pending_intent) {
                pending = data.pending_intent;
              } else if (data && (data.cascade_id || data.root_conversation_id)) {
                pending = data;
              }
            }
          }
        } catch (_) {}

        if (!pending && typeof window !== "undefined" && window.__swissPendingContinuation) {
          pending = window.__swissPendingContinuation;
        }

        if (!pending) {
          try {
            const raw = localStorage.getItem("antigravity_swiss_pending_continuation");
            if (raw) pending = JSON.parse(raw);
          } catch (_) {}
        }

        if (!pending) return;

        const convID = pending.root_conversation_id || pending.cascade_id;
        if (!convID || pending.resumed || pending.status === "revived") return;

        const intentKey = pending.intent_id || ("auto-revive-" + convID);
        window.__swissDispatchedIntents = window.__swissDispatchedIntents || {};
        if (window.__swissDispatchedIntents[intentKey]) {
          return;
        }

        // Strictly validate convID: reject non-UUID stubs (test-webgui-conv, test, etc.)
        if (!uuidRegex.test(convID)) {
          console.warn("[SwissKnife AutoRevival] Rejecting invalid non-UUID intent:", convID);
          try { localStorage.removeItem("antigravity_swiss_pending_continuation"); } catch (_) {}
          await ackContinuation(convID);
          return;
        }

        try {
          if (sessionStorage.getItem("antigravity_swiss_revived_intent_" + intentKey) ||
              sessionStorage.getItem("antigravity_swiss_revived_conv_" + convID)) {
            window.__swissDispatchedIntents[intentKey] = Date.now();
            await ackContinuation(convID);
            return;
          }
        } catch (_) {}

        // Check TTL & creation timestamp (default 300s for desktop revival)
        const ttl = (pending.ttl_seconds || 300) * 1000;
        const createdAt = pending.created_at ? new Date(pending.created_at).getTime() : (pending.timestamp ? Number(pending.timestamp) : 0);
        if (!createdAt || isNaN(createdAt) || (now - createdAt > ttl) || (createdAt > now + 60000)) {
          try { localStorage.removeItem("antigravity_swiss_pending_continuation"); } catch (_) {}
          await ackContinuation(convID);
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

          await ackContinuation(convID);
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
          await ackContinuation(convID);
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
          await ackContinuation(convID);
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
            await ackContinuation(convID);
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
            await ackContinuation(convID);
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
              await ackContinuation(convID);
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
              await ackContinuation(convID);
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

    let customModelsListCache = null;
    let lastCustomModelsFetchTime = 0;
    async function fetchCustomModelsMetadata() {
      if (window.__swissDaemonOnline === false || !window.__swissDaemonOnline) return;
      const now = Date.now();
      if (customModelsListCache && now - lastCustomModelsFetchTime < 30000) return;
      lastCustomModelsFetchTime = now;
      try {
        const res = await fetch("http://127.0.0.1:8765/api/custom_models", {
          headers: getSwissAuthHeaders(),
          signal: (typeof AbortSignal !== "undefined" && AbortSignal.timeout) ? AbortSignal.timeout(1500) : undefined
        });
        if (res.ok) {
          const data = await res.json();
          if (data && Array.isArray(data.models)) {
            customModelsListCache = data.models;
            renderContextWindowProgressRing();
          }
        }
      } catch (_) {}
    }

    function getModelContextWindow(modelName) {
      fetchCustomModelsMetadata();
      if (!modelName) return 1000000;
      const clean = modelName.toLowerCase().trim();

      // Check user-configured custom models for overridden context window
      if (customModelsListCache && Array.isArray(customModelsListCache)) {
        for (const cm of customModelsListCache) {
          const cmName = (cm.name || "").toLowerCase().trim();
          const cmDisplay = (cm.display_name || "").toLowerCase().trim();
          const cmId = (cm.id || "").toLowerCase().trim();
          if (clean === cmName || clean === cmDisplay || clean === cmId || (cmName && clean.includes(cmName)) || (cmDisplay && clean.includes(cmDisplay))) {
            if (typeof cm.context_window === "number" && cm.context_window > 0) {
              return cm.context_window;
            }
          }
        }
      }

      // Dynamic model context window sizes
      if (clean.includes("claude") || clean.includes("sonnet") || clean.includes("opus") || clean.includes("haiku")) {
        return 200000;
      }
      if (clean.includes("gpt-4") || clean.includes("gpt4") || clean.includes("o1") || clean.includes("o3") || clean.includes("chatgpt")) {
        return 128000;
      }
      if (clean.includes("pro")) {
        return 2000000;
      }
      if (clean.includes("flash")) {
        return 1000000;
      }
      if (clean.includes("deepseek")) {
        return 64000;
      }
      if (clean.includes("mistral") || clean.includes("codestral") || clean.includes("llama")) {
        return 128000;
      }

      // Standard native models default in Antigravity
      return 1000000;
    }

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

    let lastTokensFetchTimeByConv = {};
    let cachedTokensByConv = {};
    let subagentNameToConvId = {};
    async function fetchActiveConversationTokens(convId, parentId, agentName) {
      if (window.__swissDaemonOnline === false || !window.__swissDaemonOnline) return;
      const cacheKey = convId || (parentId && agentName ? (parentId + ":" + agentName) : null);
      if (!cacheKey) return;
      const now = Date.now();
      if (now - (lastTokensFetchTimeByConv[cacheKey] || 0) < 2500) return;
      lastTokensFetchTimeByConv[cacheKey] = now;
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
          signal: (typeof AbortSignal !== "undefined" && AbortSignal.timeout) ? AbortSignal.timeout(1500) : undefined
        });
        if (res.ok) {
          const data = await res.json();
          const targetId = data.conversation_id || convId;
          if (targetId) {
            if (parentId && agentName) {
              subagentNameToConvId[parentId + ":" + agentName] = targetId;
            }
            if (data && Array.isArray(data.turns) && data.turns.length > 0) {
              // The daemon reads the full conversation transcript from disk, so
              // this turn count covers prompt turns whose DOM nodes are
              // virtualized out of the chat view. The rail uses it to size the
              // bars for the whole session instead of only the mounted window.
              const previousTurnTotal = jumpBarTurnCountsByConversation[targetId] || 0;
              jumpBarTurnCountsByConversation[targetId] = data.turns.length;
              if (data.turns.length !== previousTurnTotal) {
                renderPromptJumpBar();
              }
              const lastTurn = data.turns[data.turns.length - 1];
              const turnUsage = lastTurn.context_tokens || ((lastTurn.input_tokens || 0) + (lastTurn.output_tokens || 0));
              cachedTokensByConv[targetId] = turnUsage;
              renderContextWindowProgressRing();
            } else if (data && data.success) {
              const sumUsage = data.summary ? (data.summary.context_tokens || ((data.summary.input_tokens || 0) + (data.summary.output_tokens || 0))) : 0;
              cachedTokensByConv[targetId] = sumUsage;
              renderContextWindowProgressRing();
            }
          }
        }
      } catch (_) {}
    }

    window.__swissOnEnhancementsDaemonChanged = function(online) {
      if (!online) {
        const mainRing = document.getElementById("swiss-context-window-ring");
        if (mainRing) mainRing.remove();
        document.querySelectorAll("[id^='swiss-context-window-ring-sub-'], .swiss-context-window-ring, .swiss-context-window-ring-sub").forEach(r => r.remove());
        cachedTokensByConv = {};
        lastTokensFetchTimeByConv = {};
        customModelsListCache = null;
        lastCustomModelsFetchTime = 0;
      } else {
        cachedTokensByConv = {};
        lastTokensFetchTimeByConv = {};
        customModelsListCache = null;
        lastCustomModelsFetchTime = 0;
        syncConfigFromServer();
        renderContextWindowProgressRing();
      }
    };

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
      const trigger = inputBox ? (inputBox.querySelector('[data-testid="model-selector-trigger"]') || inputBox.querySelector('button[aria-label*="Select model"]')) : (document.querySelector('[data-testid="model-selector-trigger"]') || document.querySelector('button[aria-label*="Select model"]'));
      const modelName = trigger ? trigger.innerText.trim() : "";
      const maxTokens = getModelContextWindow(modelName);
      if (ring) {
        updateProgressRing(ring, convId, maxTokens);
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

    function updateProgressRing(ring, convId, maxTokens) {
      if (!ring) return;
      if (window.__swissDaemonOnline === false || !window.__swissDaemonOnline) {
        ring.remove();
        return;
      }
      ring.classList.add("swiss-context-window-ring");
      ring.setAttribute("role", "button");
      ring.setAttribute("tabindex", "0");
      ring.style.cssText = "display: inline-flex; align-items: center; justify-content: center; width: 20px; height: 20px; margin-left: 6px; position: relative; user-select: none; flex-shrink: 0; vertical-align: middle; cursor: pointer;";

      const ringKey = (ring && ring.id) || convId || "main";
      const isCompacting = ring.getAttribute("data-swiss-compacting") === "true" || Boolean(compactingStateByRing[ringKey] && compactingStateByRing[ringKey].active);
      if (isCompacting) {
        ring.setAttribute("data-swiss-compacting", "true");
        ring.setAttribute("title", "Compacting context window...");
        ring.setAttribute("aria-label", "Compacting context window...");
        ring.style.opacity = "1";
        ring.style.cursor = "pointer";

        const spinningHTML = '<svg width="18" height="18" viewBox="0 0 20 20" style="transform: rotate(-90deg); display: block;">' +
          '<circle cx="10" cy="10" r="7" fill="none" stroke="rgba(148, 163, 184, 0.3)" stroke-width="2" />' +
          '<circle cx="10" cy="10" r="7" fill="none" stroke="#94a3b8" stroke-width="2" stroke-linecap="round" stroke-dasharray="44" stroke-dashoffset="14" />' +
        '</svg>';
        if (ring.innerHTML !== spinningHTML) {
          ring.innerHTML = spinningHTML;
        }
      } else {
        const hasReliableData = Boolean(convId && cachedTokensByConv && typeof cachedTokensByConv[convId] === "number");
        if (!hasReliableData) {
          ring.setAttribute("title", "Context window usage unavailable — Click to compact");
          ring.setAttribute("aria-label", "Context window usage unavailable — Click to compact");
          ring.style.opacity = "0.35";
          ring.style.cursor = "pointer";
          const greyHTML = '<svg width="18" height="18" viewBox="0 0 20 20" style="transform: rotate(-90deg); display: block;">' +
            '<circle cx="10" cy="10" r="7" fill="none" stroke="rgba(148, 163, 184, 0.25)" stroke-width="2" />' +
          '</svg>';
          if (ring.innerHTML !== greyHTML) ring.innerHTML = greyHTML;
        } else {
          const usedTokens = cachedTokensByConv[convId];
          const validMax = (typeof maxTokens === "number" && maxTokens > 0) ? maxTokens : 1000000;
          const pct = Math.max(0, Math.min(100, Math.round((usedTokens / validMax) * 100)));
          const formattedUsed = formatTokensUnit(usedTokens);
          const formattedMax = formatTokensUnit(validMax);
          const tooltipText = pct + "%% (" + formattedUsed + "/" + formattedMax + ")" + " — Click to compact";

          ring.setAttribute("title", tooltipText);
          ring.setAttribute("aria-label", "Context window used: " + tooltipText);
          ring.style.opacity = "1";
          ring.style.cursor = "pointer";

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

      ring.onclick = (e) => {
        invokeNativeCompactCommand(ring, convId, e);
      };
      ring.onkeydown = (e) => {
        if (e.key === "Enter" || e.key === " " || e.keyCode === 13 || e.keyCode === 32) {
          invokeNativeCompactCommand(ring, convId, e);
        }
      };
    }

    function renderContextWindowProgressRing() {
      if (window.__swissDaemonOnline === false || !window.__swissDaemonOnline) {
        const existingMain = document.getElementById("swiss-context-window-ring");
        if (existingMain) existingMain.remove();
        document.querySelectorAll("[id^='swiss-context-window-ring-sub-'], .swiss-context-window-ring, .swiss-context-window-ring-sub").forEach(r => r.remove());
        return;
      }

      const parentConvId = (window.location.pathname || "").replace(/^\/c\//, "").split(/[?#]/)[0];
      const uuidRegex = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

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
          const modelName = trigger.innerText.trim();
          const maxTokens = getModelContextWindow(modelName);
          const convId = getConversationIdFromElement(trigger) || parentConvId;
          if (convId && uuidRegex.test(convId)) {
            fetchActiveConversationTokens(convId);
          }
          if (!mainRing) {
            mainRing = document.createElement("div");
            mainRing.id = "swiss-context-window-ring";
          }
          updateProgressRing(mainRing, convId, maxTokens);

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
        } else if (subConvId && uuidRegex.test(subConvId)) {
          fetchActiveConversationTokens(subConvId);
        }

        const subBox = agentTag.closest ? agentTag.closest('[data-testid="agent-input-box"]') : null;
        const subTrigger = subBox ? (subBox.querySelector('[data-testid="model-selector-trigger"]') || subBox.querySelector('button[aria-label*="Select model"]')) : null;
        const subModelName = subTrigger ? subTrigger.innerText.trim() : "";
        const maxTokens = getModelContextWindow(subModelName);
        updateProgressRing(subRing, subConvId, maxTokens);

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
        renderPromptJumpBar();

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
    renderPromptJumpBar();
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
`, tok, string(cfgJSON))
}
