package gui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/enhancements"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/plugins"
)

// HexToRGB converts a hex string (e.g. "#7c3aed" or "7c3aed" or "#fff") into r, g, b components.
func HexToRGB(hex string) (int, int, int, error) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 3 {
		r, err1 := strconv.ParseInt(string(hex[0])+string(hex[0]), 16, 32)
		g, err2 := strconv.ParseInt(string(hex[1])+string(hex[1]), 16, 32)
		b, err3 := strconv.ParseInt(string(hex[2])+string(hex[2]), 16, 32)
		if err1 != nil || err2 != nil || err3 != nil {
			return 0, 0, 0, fmt.Errorf("invalid 3-digit hex: %s", hex)
		}
		return int(r), int(g), int(b), nil
	}
	if len(hex) == 6 {
		val, err := strconv.ParseInt(hex, 16, 32)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid 6-digit hex: %s", hex)
		}
		return int((val >> 16) & 0xFF), int((val >> 8) & 0xFF), int(val & 0xFF), nil
	}
	return 0, 0, 0, fmt.Errorf("hex string must be 3 or 6 chars: %s", hex)
}

// GenerateCSS generates custom CSS stylesheet content for all configured project colors and GUI improvements.
func GenerateCSS(cfg *Config) string {
	if cfg == nil || !cfg.Enabled || (!cfg.ColorStylingEnabled && len(cfg.ArchivedProjects) == 0 && cfg.ConversationTabsMode == "") {
		return "/* Antigravity Swiss Knife Project Styling Disabled */"
	}

	opacity := cfg.TintOpacity
	if opacity <= 0 || opacity > 1 {
		opacity = 0.14
	}
	hoverOpacity := opacity + 0.08
	if hoverOpacity > 1 {
		hoverOpacity = 1.0
	}
	selectedOpacity := opacity + 0.16
	if selectedOpacity > 1 {
		selectedOpacity = 1.0
	}

	isBorderMode := cfg.ActiveConversationIndicator == "border"
	isLeftBarMode := cfg.ActiveConversationIndicator == "left_bar" || cfg.ActiveConversationIndicator == "left_accent_bar" || cfg.SolidLeftEdge
	borderWidth := cfg.ActiveConversationBorderWidth
	if borderWidth == "" {
		borderWidth = "2px"
	}
	fontWeight := "600"
	if !cfg.ActiveConversationBold {
		fontWeight = "400"
	}


	var sb strings.Builder
	sb.WriteString("/* Antigravity Swiss Knife - Project Panel Custom Colors */\n")

	if cfg.ColorStylingEnabled {
		for project, hex := range cfg.ProjectColors {
		if project == "" || hex == "" {
			continue
		}
		r, g, b, err := HexToRGB(hex)
		if err != nil {
			r, g, b = 11, 87, 208 // fallback to blue
		}

		safeName := strings.ReplaceAll(project, `"`, `\"`)

		borderStyle := "border: none !important; border-left: none !important;"
		if isBorderMode {
			borderStyle = ""
		}
		if isLeftBarMode && !isBorderMode {
			borderStyle = "border-left: 3px solid transparent !important;"
		}

		var rowCSS string
		if isBorderMode {
			selectedBorderStyle := fmt.Sprintf("border: %s solid %s !important;", borderWidth, hex)
			if isLeftBarMode {
				selectedBorderStyle += fmt.Sprintf(" border-left: 3px solid %s !important;", hex)
			}
			rowCSS = fmt.Sprintf(`
/* Conversation Row: Clean rounded corners, light tint, border-mode */
[data-swiss-project="%s"][data-testid="conversation-row-sidebar"],
[data-swiss-project="%s"] [data-testid="conversation-row-sidebar"] {
  --sidebar-secondary: rgba(%d, %d, %d, %.2f) !important;
  --sidebar-muted: rgba(%d, %d, %d, %.2f) !important;
  background-color: rgba(%d, %d, %d, %.2f) !important;
  border: %s solid transparent !important;
  %s
  border-radius: 8px !important;
  transition: background-color 0.15s ease, border-color 0.15s ease !important;
}

/* Hover & Selected States */
[data-swiss-project="%s"][data-testid="conversation-row-sidebar"]:hover,
[data-swiss-project="%s"] [data-testid="conversation-row-sidebar"]:hover {
  background-color: rgba(%d, %d, %d, %.2f) !important;
}
[data-swiss-project="%s"][data-testid="conversation-row-sidebar"][data-selected="true"],
[data-swiss-project="%s"] [data-testid="conversation-row-sidebar"][data-selected="true"] {
  background-color: rgba(%d, %d, %d, %.2f) !important;
  %s
  font-weight: %s !important;
}
[data-swiss-project="%s"][data-testid="conversation-row-sidebar"][data-selected="true"]:hover,
[data-swiss-project="%s"] [data-testid="conversation-row-sidebar"][data-selected="true"]:hover {
  background-color: rgba(%d, %d, %d, %.2f) !important;
  %s
}
`, safeName, safeName, r, g, b, opacity, r, g, b, opacity, r, g, b, opacity, borderWidth, borderStyle, safeName, safeName, r, g, b, hoverOpacity, safeName, safeName, r, g, b, opacity, selectedBorderStyle, fontWeight, safeName, safeName, r, g, b, hoverOpacity, selectedBorderStyle)
		} else {
			selectedBorderStyle := ""
			activeBgOpacity := selectedOpacity
			if isLeftBarMode {
				selectedBorderStyle = fmt.Sprintf("\n  border-left: 3px solid %s !important;", hex)
				activeBgOpacity = opacity
			}
			rowCSS = fmt.Sprintf(`
/* Conversation Row: Clean rounded corners, light tint, solid edge optional */
[data-swiss-project="%s"][data-testid="conversation-row-sidebar"],
[data-swiss-project="%s"] [data-testid="conversation-row-sidebar"] {
  --sidebar-secondary: rgba(%d, %d, %d, %.2f) !important;
  --sidebar-muted: rgba(%d, %d, %d, %.2f) !important;
  background-color: rgba(%d, %d, %d, %.2f) !important;
  %s
  border-radius: 8px !important;
  transition: background-color 0.15s ease !important;
}

/* Hover & Selected States */
[data-swiss-project="%s"][data-testid="conversation-row-sidebar"]:hover,
[data-swiss-project="%s"] [data-testid="conversation-row-sidebar"]:hover {
  background-color: rgba(%d, %d, %d, %.2f) !important;
}
[data-swiss-project="%s"][data-testid="conversation-row-sidebar"][data-selected="true"],
[data-swiss-project="%s"] [data-testid="conversation-row-sidebar"][data-selected="true"] {
  background-color: rgba(%d, %d, %d, %.2f) !important;%s
  font-weight: %s !important;
}
`, safeName, safeName, r, g, b, opacity, r, g, b, opacity, r, g, b, opacity, borderStyle, safeName, safeName, r, g, b, hoverOpacity, safeName, safeName, r, g, b, activeBgOpacity, selectedBorderStyle, fontWeight)
		}

		sb.WriteString(fmt.Sprintf(`
/* Project Label Card */
[data-swiss-project="%s"][data-project-card="true"],
[data-swiss-project="%s"] [data-project-card="true"] {
  background-color: %s !important;
  color: #ffffff !important;
  border-radius: 8px !important;
  border: none !important;
}
[data-swiss-project="%s"][data-project-card="true"] *,
[data-swiss-project="%s"] [data-project-card="true"] * {
  color: #ffffff !important;
}

/* Project Header Action Buttons (⋮ and + buttons) */
[data-swiss-project="%s"] [class*="group/header"] button,
[data-swiss-project="%s"] button[aria-label="Project options"] svg,
[data-swiss-project="%s"] button[aria-label*="conversation"] svg {
  color: #ffffff !important;
  fill: #ffffff !important;
}
%s
/* Action Bar & Buttons on Hover: 100%% Seamless, no dark overlapping gradient strip */
[data-swiss-project="%s"][data-testid="conversation-row-sidebar"] div[style*="linear-gradient"],
[data-swiss-project="%s"] [data-testid="conversation-row-sidebar"] div[style*="linear-gradient"],
[data-swiss-project="%s"][data-testid="conversation-row-sidebar"]:hover div[style*="linear-gradient"],
[data-swiss-project="%s"] [data-testid="conversation-row-sidebar"]:hover div[style*="linear-gradient"] {
  background: transparent !important;
}

/* Natural translucent hover on action buttons */
[data-swiss-project="%s"][data-testid="conversation-row-sidebar"] button:hover,
[data-swiss-project="%s"] [data-testid="conversation-row-sidebar"] button:hover {
  background-color: rgba(255, 255, 255, 0.45) !important;
  border-radius: 6px !important;
}
`, safeName, safeName, hex, safeName, safeName, safeName, safeName, safeName, rowCSS, safeName, safeName, safeName, safeName, safeName, safeName))
		}
	}

	// Hide archived projects in sidebar
	if cfg != nil && len(cfg.ArchivedProjects) > 0 {
		sb.WriteString("\n/* Hidden / Archived Projects */\n")
		for _, arch := range cfg.ArchivedProjects {
			safeArch := strings.ReplaceAll(arch, `"`, `\"`)
			sb.WriteString(fmt.Sprintf(`
[data-swiss-project="%s"] {
  display: none !important;
}
`, safeArch))
		}
	}

	// Conversation Tabs Expand/Contract Divider with centered circular pill and solid triangle indicator
	sb.WriteString(`
/* Antigravity Swiss Knife - Conversation Tabs Divider */
div:has(> button[data-swiss-divider="true"]),
div:has(> [data-swiss-divider="true"]) {
  padding-left: 0 !important;
  padding-right: 0 !important;
  margin: 0 !important;
}
button[data-swiss-divider="true"] {
  padding: 0 !important;
  margin: 0 !important;
  width: 100% !important;
}
.swiss-convo-tabs-divider {
  display: flex !important;
  flex-direction: column !important;
  align-items: center !important;
  justify-content: center !important;
  width: 100% !important;
  height: 100% !important;
  padding: 0 !important;
  box-sizing: border-box !important;
  cursor: pointer !important;
  user-select: none !important;
}
.swiss-convo-tabs-line {
  width: 100% !important;
  height: 1px !important;
  background: rgba(148, 163, 184, 0.35) !important;
  transition: background-color 0.18s ease !important;
}
.swiss-convo-tabs-pill {
  display: inline-flex !important;
  align-items: center !important;
  justify-content: center !important;
  width: 16px !important;
  height: 11px !important;
  margin-bottom: 2px !important;
  color: #64748b !important;
  font-size: 8px !important;
  transition: all 0.18s ease !important;
}
.swiss-convo-tabs-triangle {
  display: inline-block !important;
  font-size: 8px !important;
  line-height: 1 !important;
  transition: transform 0.2s cubic-bezier(0.4, 0, 0.2, 1) !important;
}
[data-theme="dark"] .swiss-convo-tabs-line,
.dark .swiss-convo-tabs-line {
  background: rgba(148, 163, 184, 0.22) !important;
}
[data-theme="dark"] .swiss-convo-tabs-pill,
.dark .swiss-convo-tabs-pill {
  color: #94a3b8 !important;
}

/* Hover effects */
button[data-swiss-divider="true"]:hover .swiss-convo-tabs-line,
.swiss-convo-tabs-divider:hover .swiss-convo-tabs-line,
[data-swiss-custom-divider]:hover .swiss-convo-tabs-line {
  background: rgba(148, 163, 184, 0.65) !important;
}
button[data-swiss-divider="true"]:hover .swiss-convo-tabs-pill,
.swiss-convo-tabs-divider:hover .swiss-convo-tabs-pill,
[data-swiss-custom-divider]:hover .swiss-convo-tabs-pill {
  color: #1e293b !important;
  transform: scale(1.18) !important;
}
[data-theme="dark"] button[data-swiss-divider="true"]:hover .swiss-convo-tabs-pill,
.dark button[data-swiss-divider="true"]:hover .swiss-convo-tabs-pill,
[data-theme="dark"] [data-swiss-custom-divider]:hover .swiss-convo-tabs-pill,
.dark [data-swiss-custom-divider]:hover .swiss-convo-tabs-pill {
  color: #f1f5f9 !important;
}
`)

	sb.WriteString("\n" + plugins.GenerateAuxiliaryPluginsCSS() + "\n")
	return sb.String()
}

// generateBaseScript generates the base project tags and conversation tabs styling script.
func generateBaseScript(cfg *Config) string {
	css := GenerateCSS(cfg)
	cssJSON, _ := json.Marshal(css)
	enabled := cfg != nil && cfg.Enabled
	colorStylingEnabled := enabled && cfg.ColorStylingEnabled
	dragRearrangeEnabled := enabled && cfg.DragRearrangeEnabled

	orderJSON, _ := json.Marshal(cfg.ProjectOrder)
	archivedProjects := []string{}
	if cfg != nil && cfg.ArchivedProjects != nil {
		archivedProjects = cfg.ArchivedProjects
	}
	archivedJSON, _ := json.Marshal(archivedProjects)

	tabsMode := "fixed"
	if cfg != nil && cfg.ConversationTabsMode != "" {
		tabsMode = cfg.ConversationTabsMode
	}
	tabsFixedLimit := 6
	if cfg != nil && cfg.ConversationTabsFixedLimit > 0 {
		tabsFixedLimit = cfg.ConversationTabsFixedLimit
	}
	tabsAgeThreshold := "1d"
	if cfg != nil && cfg.ConversationTabsAgeThreshold != "" {
		tabsAgeThreshold = cfg.ConversationTabsAgeThreshold
	}
	tabsMin := 2
	if cfg != nil && cfg.ConversationTabsMin > 0 {
		tabsMin = cfg.ConversationTabsMin
	}
	tabsMax := 6
	if cfg != nil && cfg.ConversationTabsMax > 0 {
		tabsMax = cfg.ConversationTabsMax
	}

	baseScript := fmt.Sprintf(`(() => {
  const css = %s;
  const isEnabled = %t;
  const isColorEnabled = %t;
  const isDragEnabled = %t;
  const configuredOrder = %s;
  const archivedProjects = %s;
  const tabsMode = %q;
  const tabsFixedLimit = %d;
  const tabsAgeThreshold = %q;
  const tabsMin = %d;
  const tabsMax = %d;
  window.__swissArchivedProjects = Array.isArray(archivedProjects) ? archivedProjects : [];

  // 1. Manage stylesheet
  let styleEl = document.getElementById("antigravity-swiss-styles");
  if (!isEnabled) {
    if (styleEl) styleEl.remove();
    document.querySelectorAll("[data-swiss-project]").forEach(el => el.removeAttribute("data-swiss-project"));
  } else {
    if (!styleEl) {
      styleEl = document.createElement("style");
      styleEl.id = "antigravity-swiss-styles";
      document.head.appendChild(styleEl);
    }
    styleEl.textContent = css;
  }

  // 2. Tag virtual list items & attach drag handlers
  function updateTagsAndDraggables() {
    try {
      const activeArchived = window.__swissArchivedProjects || archivedProjects || [];
      let container = document.querySelector(".w-full.relative > [data-index]")?.parentElement;
      if (!container) {
        const anyIndexed = document.querySelector("[data-index]");
        if (anyIndexed && anyIndexed.parentElement) {
          container = anyIndexed.parentElement;
        }
      }
      if (!container) return;

      const key = Object.keys(container).find(k => k.startsWith("__reactFiber"));
      if (!key) return;
      let fiber = container[key];
      let items = null;
      while (fiber) {
        if (fiber.memoizedProps?.items) {
          items = fiber.memoizedProps.items;
          break;
        }
        fiber = fiber.return;
      }
      if (!items) return;

      const headerMap = {};
      const idToHeaderMap = {};
      items.forEach(it => {
        if (it.type === "header") {
          const gid = it.id.replace("header-", "");
          headerMap[gid] = it.label;
          idToHeaderMap[it.id] = it;
        }
      });

      // Determine hidden indices for archived projects
      const hiddenIndices = new Set();
      if (activeArchived && activeArchived.length > 0) {
        items.forEach((it, idx) => {
          if (it.type === "header") {
            const isArchived = activeArchived.includes(it.label) || activeArchived.includes(it.id.replace("header-", ""));
            if (isArchived) hiddenIndices.add(idx);
          } else if (it.type === "row") {
            const gid = it.groupId;
            const isArchived = activeArchived.includes(headerMap[gid]) || activeArchived.includes(gid);
            if (isArchived) hiddenIndices.add(idx);
          } else if (it.type === "show-more") {
            const gid = it.groupId;
            const isArchived = activeArchived.includes(headerMap[gid]) || activeArchived.includes(gid);
            if (isArchived) hiddenIndices.add(idx);
          }
        });
      }

      // Clean up any obsolete custom divider DOM elements
      container.querySelectorAll(".swiss-custom-divider").forEach(el => el.remove());

      const itemsList = Array.from(container.querySelectorAll(":scope > [data-index]"));
      itemsList.sort((a, b) => parseInt(a.getAttribute("data-index"), 10) - parseInt(b.getAttribute("data-index"), 10));

      itemsList.forEach(el => {
        try {
          const idx = parseInt(el.getAttribute("data-index"), 10);
          const item = items[idx];
          if (!item) return;

          // Clean up legacy baseline translateY attributes
          delete el.dataset.origTranslateY;
          delete el.dataset.swissBaselineIdx;

          if (hiddenIndices.has(idx)) {
            el.style.display = "none";
            return;
          }

          el.style.display = "";

          // Project conversation row color tinting
          if (item.type === "row" && item.groupId) {
            const pName = headerMap[item.groupId];
            if (pName) {
              const row = el.matches('[data-testid="conversation-row-sidebar"]')
                ? el
                : el.querySelector('[data-testid="conversation-row-sidebar"]');
              if (isColorEnabled) {
                el.setAttribute("data-swiss-project", pName);
                if (row) row.setAttribute("data-swiss-project", pName);
              } else {
                el.removeAttribute("data-swiss-project");
                if (row) row.removeAttribute("data-swiss-project");
              }
            }
          }

          // Project show-more button handling: preserve native button text and event handlers
          if (item.type === "show-more") {
            const btn = el.querySelector("button");
            if (btn && btn.getAttribute("data-swiss-divider") === "true") {
              btn.removeAttribute("data-swiss-divider");
            }
          }

          if (item.type === "header") {
            const btn = el.matches("[data-project-card]") ? el : el.querySelector("[data-project-card]");
            if (btn) {
              if (isColorEnabled) btn.setAttribute("data-swiss-project", item.label);
              else btn.removeAttribute("data-swiss-project");
            }
            const headerGroup = el.querySelector('[class*="group/header"]') || el;
            if (isColorEnabled) {
              el.setAttribute("data-swiss-project", item.label);
              if (headerGroup) headerGroup.setAttribute("data-swiss-project", item.label);
            } else {
              el.removeAttribute("data-swiss-project");
              if (headerGroup) headerGroup.removeAttribute("data-swiss-project");
            }

            // Drag-and-drop reordering on project headers
            if (isDragEnabled) {
              headerGroup.setAttribute("draggable", "true");
              headerGroup.style.cursor = "grab";
              headerGroup.setAttribute("data-project-id", item.id.replace("header-", ""));
              headerGroup.setAttribute("data-project-label", item.label);

              if (!headerGroup.__swissDragBound) {
                headerGroup.__swissDragBound = true;
                headerGroup.addEventListener("dragstart", (e) => {
                  e.dataTransfer.setData("text/plain", headerGroup.getAttribute("data-project-id"));
                  headerGroup.style.opacity = "0.5";
                });
                headerGroup.addEventListener("dragend", () => {
                  headerGroup.style.opacity = "1";
                });
                headerGroup.addEventListener("dragover", (e) => {
                  e.preventDefault();
                  e.dataTransfer.dropEffect = "move";
                });
                headerGroup.addEventListener("drop", async (e) => {
                  e.preventDefault();
                  const srcId = e.dataTransfer.getData("text/plain");
                  const targetId = headerGroup.getAttribute("data-project-id");
                  if (srcId && targetId && srcId !== targetId) {
                    // Get current projects order from nativeStorage
                    const curItems = await window.nativeStorage?.getItems();
                    let pOrder = [];
                    try { pOrder = JSON.parse(curItems?.projectsOrder || "[]"); } catch(_) {}
                    if (!pOrder.length) {
                      pOrder = items.filter(it => it.type === "header").map(h => h.id.replace("header-", ""));
                    }
                    const srcIdx = pOrder.indexOf(srcId);
                    const tgtIdx = pOrder.indexOf(targetId);
                    if (srcIdx !== -1 && tgtIdx !== -1) {
                      pOrder.splice(srcIdx, 1);
                      pOrder.splice(tgtIdx, 0, srcId);
                      await window.nativeStorage?.updateItems({
                        projectsOrder: JSON.stringify(pOrder),
                        projectsSortBy: "custom"
                      });
                    }
                  }
                });
              }
            } else {
              headerGroup.removeAttribute("draggable");
              headerGroup.style.cursor = "";
            }
          }
        } catch (_) {}
      });

      // Apply initial configured order if specified and not yet synced
      if (isDragEnabled && configuredOrder && configuredOrder.length > 0 && !window.__swissOrderSynced) {
        window.__swissOrderSynced = true;
        (async () => {
          const cur = await window.nativeStorage?.getItems();
          let pOrder = [];
          try { pOrder = JSON.parse(cur?.projectsOrder || "[]"); } catch(_) {}
          // Map configured project names to group IDs
          const nameToId = {};
          items.forEach(it => {
            if (it.type === "header") {
              nameToId[it.label] = it.id.replace("header-", "");
            }
          });
          const mappedOrder = configuredOrder.map(name => nameToId[name]).filter(Boolean);
          if (mappedOrder.length > 0) {
            // Append any remaining IDs not in configured order
            items.forEach(it => {
              if (it.type === "header") {
                const gid = it.id.replace("header-", "");
                if (!mappedOrder.includes(gid)) mappedOrder.push(gid);
              }
            });
            await window.nativeStorage?.updateItems({
              projectsOrder: JSON.stringify(mappedOrder),
              projectsSortBy: "custom"
            });
          }
        })();
      }
      if (typeof window.__swissEnhanceProjectOptionsMenu === "function") {
        window.__swissEnhanceProjectOptionsMenu();
      }
    } catch (_) {}
  }

  // 3. Project Options Menu Enhancement (New Conversation, Archive, Set Color with 5 Presets + 10x10 Custom Palette)
  const HUES = [0, 24, 42, 85, 145, 175, 205, 250, 285, 325];
  const LIGHTNESSES = [90, 82, 74, 65, 56, 48, 40, 32, 24, 16];
  const GRID_COLORS = [];
  for (let r = 0; r < 10; r++) {
    const l = LIGHTNESSES[r];
    for (let c = 0; c < 10; c++) {
      GRID_COLORS.push("hsl(" + HUES[c] + ", 82%%, " + l + "%%)");
    }
  }

  const PRESET_COLORS = [
    { name: "Blue", hex: "#0b57d0" },
    { name: "Purple", hex: "#7c3aed" },
    { name: "Emerald", hex: "#059669" },
    { name: "Amber", hex: "#d97706" },
    { name: "Coral", hex: "#dc2626" },
  ];

  // Remove legacy floating context menu if present
  window.__showSwissColorMenu = function() {};
  const oldCtxMenu = document.getElementById("swiss-project-context-menu");
  if (oldCtxMenu) oldCtxMenu.remove();

  if (!window.__swissToast) {
    window.__swissToast = function(msg) {
      let t = document.getElementById("swiss-toast-notification");
      if (t) t.remove();
      t = document.createElement("div");
      t.id = "swiss-toast-notification";
      t.style.cssText = "position: fixed; bottom: 24px; right: 24px; z-index: 9999999; background: var(--card, #ffffff); color: var(--foreground, #101010); border: 1px solid var(--border, rgba(0, 0, 0, 0.08)); padding: 8px 14px; border-radius: 8px; font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; font-size: 12px; font-weight: 500; box-shadow: 0 4px 12px rgba(0,0,0,0.08); pointer-events: none; transition: opacity 0.3s ease; opacity: 1;";
      t.textContent = msg;
      document.body.appendChild(t);
      setTimeout(() => {
        t.style.opacity = "0";
        setTimeout(() => t.remove(), 300);
      }, 2500);
    };
  }

  // Track the most recently clicked project options button
  if (!window.__swissProjectOptionsTrackerBound) {
    window.__swissProjectOptionsTrackerBound = true;
    document.addEventListener("pointerdown", (e) => {
      const btn = e.target.closest('button[aria-label="Project options"]');
      if (btn) {
        const header = btn.closest('[data-swiss-project], [class*="group/header"], [data-project-id], [data-project-label]');
        let pName = header?.getAttribute("data-swiss-project") ||
                    header?.getAttribute("data-project-label") ||
                    header?.querySelector("[data-project-card]")?.getAttribute("data-swiss-project");
        if (!pName) {
          const card = header?.querySelector("[data-project-card]") || header;
          pName = card ? card.textContent.trim() : "";
        }
        window.__swissLastClickedProject = { name: pName, time: Date.now() };
        if (typeof window.__swissEnhanceProjectOptionsMenu === "function") {
          requestAnimationFrame(window.__swissEnhanceProjectOptionsMenu);
          setTimeout(window.__swissEnhanceProjectOptionsMenu, 50);
          setTimeout(window.__swissEnhanceProjectOptionsMenu, 150);
        }
      }
    }, true);
  }

  function enhanceProjectOptionsMenu() {
    try {
      const menu = document.querySelector('[role="menu"]');
      if (!menu) return;

      const deleteItem = menu.querySelector('[data-testid="group-header-delete-item"]')?.closest('[role="menuitem"]');
      const copyItem = menu.querySelector('[data-testid="group-header-copy-item"]');
      const settingsItem = menu.querySelector('[data-testid="group-header-settings-item"]');

      if (!deleteItem && !copyItem && !settingsItem) return;

      let projectName = "";
      const labelledBy = menu.getAttribute("aria-labelledby");
      const triggerEl = labelledBy ? document.getElementById(labelledBy) : null;
      if (triggerEl) {
        const header = triggerEl.closest('[data-swiss-project], [class*="group/header"], [data-project-id], [data-project-label]');
        projectName = header?.getAttribute("data-swiss-project") ||
                      header?.getAttribute("data-project-label") ||
                      header?.querySelector("[data-project-card]")?.getAttribute("data-swiss-project");
        if (!projectName) {
          const card = header?.querySelector("[data-project-card]") || header;
          projectName = card ? card.textContent.trim() : "";
        }
      }
      if (!projectName && window.__swissLastClickedProject && (Date.now() - window.__swissLastClickedProject.time < 3000)) {
        projectName = window.__swissLastClickedProject.name;
      }
      if (!projectName) return;

      const currentVer = "v2_archive";
      if (menu.__swissMenuVersion === currentVer && menu.__swissProjectBound === projectName && menu.querySelector("#swiss-menu-archive-action")?.onclick) return;
      menu.__swissMenuVersion = currentVer;
      menu.__swissProjectBound = projectName;

      menu.querySelectorAll("#swiss-menu-archive-action, #swiss-menu-new-convo-action, #swiss-menu-color-group").forEach(el => el.remove());

      const closeMenu = () => {
        document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
      };

      const setItemHover = (el) => {
        if (!el) return;
        el.onmouseenter = () => {
          el.style.background = "var(--secondary, rgba(0, 0, 0, 0.06))";
          el.style.color = "var(--foreground, #101010)";
        };
        el.onmouseleave = () => {
          el.style.background = "transparent";
          el.style.color = "var(--secondary-foreground, #4a4a4a)";
        };
      };

      // 1. New Conversation
      const newConvoItem = document.createElement("div");
      newConvoItem.setAttribute("role", "menuitem");
      newConvoItem.setAttribute("tabindex", "-1");
      newConvoItem.id = "swiss-menu-new-convo-action";
      newConvoItem.className = "w-full pr-2 pl-2 text-left text-[13px] cursor-pointer outline-none no-focus-ring transition-colors select-none flex items-center rounded-md py-1 gap-1.5 text-secondary-foreground";
      newConvoItem.style.cssText = "transition: background 0.1s, color 0.1s;";
      newConvoItem.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" class="shrink-0" style="margin-left: 1px; margin-right: 1px;"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>' +
        '<span>New Conversation</span>';
      setItemHover(newConvoItem);

      newConvoItem.onclick = () => {
        closeMenu();
        let gid = null;
        try {
          const indexedSample = document.querySelector(".w-full.relative > [data-index]");
          const container = indexedSample?.parentElement;
          const key = container ? Object.keys(container).find(k => k.startsWith("__reactFiber")) : null;
          let fiber = key ? container[key] : null;
          while (fiber) {
            if (fiber.memoizedProps?.items) {
              const h = fiber.memoizedProps.items.find(it => it.type === "header" && it.label === projectName);
              if (h) gid = h.id.replace("header-", "");
              break;
            }
            fiber = fiber.return;
          }
        } catch (_) {}
        if (gid) {
          window.location.href = "/?section=" + gid;
        }
      };

      // 2. Archive
      const archiveItem = document.createElement("div");
      archiveItem.setAttribute("role", "menuitem");
      archiveItem.setAttribute("tabindex", "-1");
      archiveItem.id = "swiss-menu-archive-action";
      archiveItem.className = "w-full pr-2 pl-2 text-left text-[13px] cursor-pointer outline-none no-focus-ring transition-colors select-none flex items-center rounded-md py-1 gap-1.5 text-secondary-foreground";
      archiveItem.style.cssText = "transition: background 0.1s, color 0.1s;";
      archiveItem.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" class="shrink-0" style="margin-left: 1px; margin-right: 1px;"><polyline points="21 8 21 21 3 21 3 8"/><rect width="22" height="5" x="1" y="3" rx="1"/><line x1="10" x2="14" y1="12" y2="12"/></svg>' +
        '<span>Archive</span>';
      setItemHover(archiveItem);

      archiveItem.onclick = async () => {
        closeMenu();
        if (window.__swissToast) {
          window.__swissToast("Project archived: " + projectName);
        }
        try {
          if (!archivedProjects.includes(projectName)) {
            archivedProjects.push(projectName);
          }
          if (window.__swissArchivedProjects && !window.__swissArchivedProjects.includes(projectName)) {
            window.__swissArchivedProjects.push(projectName);
          }
          if (typeof window.__swissUpdateTagsAndDraggables === "function") {
            requestAnimationFrame(window.__swissUpdateTagsAndDraggables);
          } else {
            requestAnimationFrame(updateTagsAndDraggables);
          }
          await fetch("http://127.0.0.1:8765/api/gui/projects/archive", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ name: projectName })
          });
        } catch (e) {
          console.warn("[SwissKnife] Archive backend sync:", e);
        }
      };

      if (deleteItem) {
        deleteItem.parentNode.insertBefore(newConvoItem, deleteItem);
        deleteItem.parentNode.insertBefore(archiveItem, deleteItem);
      } else {
        menu.appendChild(newConvoItem);
        menu.appendChild(archiveItem);
      }

      // 3. Set Color group
      const colorGroup = document.createElement("div");
      colorGroup.id = "swiss-menu-color-group";
      colorGroup.style.cssText = "margin-top: 1px;";

      colorGroup.innerHTML = '<div role="separator" style="height: 1px; background: var(--border, rgba(0, 0, 0, 0.08)); margin: 4px -4px 3px -4px;"></div>' +
        '<div style="display: flex; justify-content: space-between; align-items: center; padding: 2px 8px 3px 8px;">' +
          '<span style="font-size: 11px; font-weight: 500; color: var(--muted-foreground, #71717a); user-select: none;">Set Color</span>' +
          '<button class="swiss-menu-reset-btn swiss-reset-color-btn" style="border: none; background: transparent; color: var(--muted-foreground, #a1a1aa); font-size: 10px; font-weight: 400; cursor: pointer; padding: 0; transition: color 0.1s;" onmouseenter="this.style.color=\'#ef4444\'" onmouseleave="this.style.color=\'var(--muted-foreground, #a1a1aa)\'">Reset</button>' +
        '</div>' +
        '<div style="display: flex; gap: 6px; padding: 2px 8px 4px 8px; align-items: center;">' +
          PRESET_COLORS.map(function(p) {
            return '<div class="swiss-preset-swatch swiss-menu-swatch" data-color="' + p.hex + '" title="' + p.name + '" style="width: 15px; height: 15px; border-radius: 50%%; background: ' + p.hex + '; cursor: pointer; transition: transform 0.12s; border: 1.5px solid transparent; flex-shrink: 0;"></div>';
          }).join("") +
          '<div id="swiss-custom-trigger" class="swiss-menu-custom-trigger" title="Custom 10x10 Palette" style="width: 15px; height: 15px; border-radius: 50%%; background: conic-gradient(from 0deg, #6c5ce7, #a259c6, #e05260, #e66735, #e69d28, #d4be22, #88b832, #3db862, #2ca88b, #259cb8, #2d7ee8, #4c6ee0, #6c5ce7); cursor: pointer; transition: transform 0.12s; border: 1.5px solid transparent; flex-shrink: 0;"></div>' +
        '</div>' +
        '<div id="swiss-custom-grid-container" class="swiss-menu-custom-grid" style="display: none; padding: 2px 6px 4px 6px;">' +
          '<div style="display: grid; grid-template-columns: repeat(10, 16px); gap: 3px; justify-content: center; background: var(--muted, rgba(0, 0, 0, 0.03)); padding: 6px; border-radius: 6px; border: 1px solid var(--border, rgba(0, 0, 0, 0.075));">' +
            GRID_COLORS.map(function(c) {
              return '<div class="swiss-grid-cell" data-color="' + c + '" style="width: 16px; height: 16px; border-radius: 50%%; background: ' + c + '; cursor: pointer; transition: transform 0.12s;"></div>';
            }).join("") +
          '</div>' +
        '</div>';

      const customModelsGroup = menu.querySelector(".swiss-custom-models-menu-group");
      if (customModelsGroup) {
        customModelsGroup.parentNode.insertBefore(colorGroup, customModelsGroup);
      } else {
        menu.appendChild(colorGroup);
      }

      const customTrig = colorGroup.querySelector("#swiss-custom-trigger");
      if (customTrig) {
        customTrig.onmouseenter = () => {
          customTrig.style.transform = "scale(1.3)";
        };
        customTrig.onmouseleave = () => {
          customTrig.style.transform = "scale(1)";
        };
        customTrig.onclick = (e) => {
          e.stopPropagation();
          const gridBox = colorGroup.querySelector("#swiss-custom-grid-container");
          if (gridBox) {
            gridBox.style.display = (gridBox.style.display === "none") ? "block" : "none";
          }
        };
      }

      const applyColor = async (colorVal) => {
        closeMenu();
        let hex = colorVal;
        if (colorVal.startsWith("hsl")) {
          const dummy = document.createElement("div");
          dummy.style.color = colorVal;
          document.body.appendChild(dummy);
          const rgb = window.getComputedStyle(dummy).color;
          dummy.remove();
          const m = rgb.match(/\\d+/g);
          if (m) {
            hex = "#" + ((1 << 24) + (parseInt(m[0]) << 16) + (parseInt(m[1]) << 8) + parseInt(m[2])).toString(16).slice(1);
          }
        }

        if (window.__swissToast) {
          window.__swissToast("Updated color for " + projectName);
        }

        try {
          await fetch("http://127.0.0.1:8765/api/gui/color", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ name: projectName, color: hex })
          });
        } catch (e) {
          console.warn("[SwissKnife] Save color failed:", e);
        }
      };

      colorGroup.querySelectorAll(".swiss-preset-swatch, .swiss-grid-cell").forEach(el => {
        el.onmouseenter = () => {
          el.style.transform = "scale(1.3)";
          el.style.zIndex = "10";
        };
        el.onmouseleave = () => {
          el.style.transform = "scale(1)";
          el.style.zIndex = "1";
        };
        el.onclick = () => applyColor(el.getAttribute("data-color"));
      });

      const resetBtn = colorGroup.querySelector(".swiss-menu-reset-btn");
      if (resetBtn) {
        resetBtn.onclick = async () => {
          closeMenu();
          if (window.__swissToast) {
            window.__swissToast("Reset color for " + projectName);
          }
          try {
            await fetch("http://127.0.0.1:8765/api/gui/projects/delete", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ name: projectName })
            });
          } catch (e) {}
        };
      }
    } catch (_) {}
  }

  window.__swissEnhanceProjectOptionsMenu = enhanceProjectOptionsMenu;
  window.__swissUpdateTagsAndDraggables = updateTagsAndDraggables;

  let isUpdatingSwiss = false;
  let scheduledRafSwiss = null;

  function triggerSwissUpdate() {
    if (isUpdatingSwiss) return;
    if (scheduledRafSwiss) return;
    scheduledRafSwiss = requestAnimationFrame(() => {
      scheduledRafSwiss = null;
      if (isUpdatingSwiss) return;
      isUpdatingSwiss = true;
      try {
        if (typeof window.__swissUpdateTagsAndDraggables === "function") {
          window.__swissUpdateTagsAndDraggables();
        }
        if (typeof window.__swissEnhanceProjectOptionsMenu === "function") {
          window.__swissEnhanceProjectOptionsMenu();
        }
      } finally {
        setTimeout(() => {
          isUpdatingSwiss = false;
        }, 32);
      }
    });
  }

  triggerSwissUpdate();

  // 1. Global capturing scroll & wheel listeners (catches all scrolling immediately)
  if (!window.__swissScrollCaptured) {
    window.__swissScrollCaptured = true;
    window.addEventListener("scroll", triggerSwissUpdate, { capture: true, passive: true });
    window.addEventListener("wheel", triggerSwissUpdate, { capture: true, passive: true });
  }

  // 2. Direct listener on sidebar container if present
  function bindSidebarScroll() {
    const sc = document.querySelector(".w-full.relative > [data-index]")?.parentElement?.parentElement;
    if (sc && !sc.__swissScrollBound) {
      sc.__swissScrollBound = true;
      sc.addEventListener("scroll", triggerSwissUpdate, { passive: true });
    }
  }
  bindSidebarScroll();

  // 3. MutationObserver watching childList and dynamic attributes with re-entrancy protection
  if (window.__swissObserverInstance) {
    try { window.__swissObserverInstance.disconnect(); } catch (_) {}
  }
  const ob = new MutationObserver((mutations) => {
    if (isUpdatingSwiss) return;
    let relevant = false;
    for (const m of mutations) {
      const t = m.target;
      if (t && (t.id === "antigravity-swiss-styles" || t.id === "swiss-project-context-menu" || t.id === "swiss-custom-models-section" || t.hasAttribute?.("data-swiss-divider") || t.classList?.contains("swiss-convo-tabs-divider") || t.classList?.contains("swiss-convo-tabs-pill"))) {
        continue;
      }
      relevant = true;
      break;
    }
    if (!relevant) return;

    bindSidebarScroll();
    triggerSwissUpdate();
  });
  window.__swissObserverInstance = ob;
  ob.observe(document.body, {
    childList: true,
    subtree: true,
    attributes: true,
    attributeFilter: ["data-selected", "data-index", "data-testid", "data-open", "role"]
  });

  // 4. Fallback interval so no virtualized row ever misses its project styling
  if (window.__swissIntervalId) {
    clearInterval(window.__swissIntervalId);
  }
  window.__swissIntervalId = setInterval(() => {
    bindSidebarScroll();
    triggerSwissUpdate();
  }, 1000);

  return {
    applied: true,
    colorEnabled: isColorEnabled,
    dragEnabled: isDragEnabled,
    taggedCount: document.querySelectorAll("[data-swiss-project]").length
  };
})();`, string(cssJSON), enabled, colorStylingEnabled, dragRearrangeEnabled, string(orderJSON), string(archivedJSON), tabsMode, tabsFixedLimit, tabsAgeThreshold, tabsMin, tabsMax)

	return baseScript
}

// GenerateScript generates the JavaScript snippet to evaluate inside Antigravity Electron renderer.
func GenerateScript(cfg *Config) string {
	return GenerateScriptWithCustomModels(cfg, nil)
}

// GenerateScriptWithCustomModels generates the complete script including project tags, custom models, enhancements, and auxiliary plugins.
func GenerateScriptWithCustomModels(cfg *Config, cmCfg *custommodels.Config) string {
	baseScript := generateBaseScript(cfg)

	if cmCfg == nil {
		if cmStore, err := custommodels.NewStore(""); err == nil {
			c := cmStore.GetConfig()
			cmCfg = &c
		}
	}
	customScript := custommodels.GenerateCustomModelsScript(cmCfg)

	var enhCfg *enhancements.EnhancementsConfig
	if enhStore, err := enhancements.NewStore(""); err == nil {
		c := enhStore.GetConfig()
		enhCfg = &c
	}
	enhScript := enhancements.GenerateEnhancementsScript(enhCfg)

	pluginsScript := plugins.GenerateAuxiliaryPluginsScript()

	return baseScript + ";\n\n" + customScript + ";\n\n" + enhScript + ";\n\n" + pluginsScript + ";"
}
