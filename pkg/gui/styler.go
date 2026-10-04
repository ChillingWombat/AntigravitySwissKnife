package gui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/enhancements"
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

// GenerateCSS generates custom CSS stylesheet content for all configured project colors.
func GenerateCSS(cfg *Config) string {
	if cfg == nil || !cfg.Enabled || (!cfg.ColorStylingEnabled && len(cfg.ArchivedProjects) == 0) || (len(cfg.ProjectColors) == 0 && len(cfg.ArchivedProjects) == 0) {
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
		if cfg.SolidLeftEdge {
			borderStyle = fmt.Sprintf("border-left: 3px solid %s !important;", hex)
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
  background-color: rgba(%d, %d, %d, %.2f) !important;
  font-weight: 600 !important;
}

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
`, safeName, safeName, hex, safeName, safeName, safeName, safeName, safeName, safeName, safeName, r, g, b, opacity, r, g, b, opacity, r, g, b, opacity, borderStyle, safeName, safeName, r, g, b, hoverOpacity, safeName, safeName, r, g, b, selectedOpacity, safeName, safeName, safeName, safeName, safeName, safeName))
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

	return sb.String()
}

// GenerateScript generates the JavaScript snippet to evaluate inside Antigravity Electron renderer.
func GenerateScript(cfg *Config) string {
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

	baseScript := fmt.Sprintf(`(() => {
  const css = %s;
  const isEnabled = %t;
  const isColorEnabled = %t;
  const isDragEnabled = %t;
  const configuredOrder = %s;
  const archivedProjects = %s;
  window.__swissArchivedProjects = Array.isArray(archivedProjects) ? archivedProjects : [];

  // 1. Manage stylesheet
  let styleEl = document.getElementById("antigravity-swiss-styles");
  if (!isColorEnabled) {
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

      let offsetAdjustment = 0;
      const itemsList = container.querySelectorAll(":scope > [data-index]");
      itemsList.forEach(el => {
        try {
          const idx = parseInt(el.getAttribute("data-index"), 10);
          const item = items[idx];
          if (!item) return;

          const isArchived = (item.type === "header" && (activeArchived.includes(item.label) || activeArchived.includes(item.id.replace("header-", "")))) ||
                             (item.type === "row" && (activeArchived.includes(headerMap[item.groupId]) || activeArchived.includes(item.groupId)));

          if (isArchived) {
            el.style.display = "none";
            offsetAdjustment += (item.type === "header" ? 37 : 33);
            return;
          }

          el.style.display = "";
          const origTransform = el.style.transform;
          const m = origTransform ? origTransform.match(/translateY\((\d+(\.\d+)?)px\)/) : null;
          if (m) {
            if (!el.dataset.origTranslateY || Math.abs(parseFloat(el.dataset.origTranslateY) - parseFloat(m[1])) > (offsetAdjustment + 5)) {
              el.dataset.origTranslateY = m[1];
            }
            if (offsetAdjustment > 0) {
              const origY = parseFloat(el.dataset.origTranslateY);
              const newY = Math.max(0, origY - offsetAdjustment);
              el.style.transform = "translateY(" + newY + "px)";
            } else if (el.dataset.origTranslateY) {
              el.style.transform = "translateY(" + el.dataset.origTranslateY + "px)";
              delete el.dataset.origTranslateY;
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
          } else if (item.type === "row") {
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
    } catch (_) {}
  }

  // 3. Right-Click Context Menu for Project Colors (5 Presets + 10x10 Custom Palette)
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

    window.__showSwissColorMenu = function(x, y, projectName) {
      let menu = document.getElementById("swiss-project-context-menu");
      if (menu) menu.remove();

      menu = document.createElement("div");
      menu.id = "swiss-project-context-menu";
      menu.style.cssText = "position: fixed; left: " + Math.min(x, window.innerWidth - 245) + "px; top: " + Math.min(y, window.innerHeight - 360) + "px; z-index: 999999; background: var(--card, #fbfbfb); border: 1px solid var(--border, rgba(0, 0, 0, 0.08)); border-radius: 10px; box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.05), 0 2px 4px -2px rgba(0, 0, 0, 0.05); width: 232px; padding: 4px; font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; font-size: 13px; color: var(--foreground, #101010); user-select: none;";

      var html = '<div style="padding: 4px 6px 3px 6px; display: flex; justify-content: space-between; align-items: center; margin-bottom: 2px;">' +
        '<span style="overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 190px; font-size: 12px; font-weight: 500; color: var(--muted-foreground, #71717a); line-height: 1.4;">' + projectName + '</span>' +
        '<button id="swiss-close-menu" style="border: none; background: transparent; cursor: pointer; color: var(--muted-foreground, #9ca3af); font-size: 15px; line-height: 1; padding: 0 2px; border-radius: 4px; transition: color 0.1s;" onmouseenter="this.style.color=\'var(--foreground, #101010)\'" onmouseleave="this.style.color=\'var(--muted-foreground, #9ca3af)\'">&times;</button>' +
      '</div>' +

      '<div style="height: 1px; background: var(--border, rgba(0, 0, 0, 0.075)); margin: 2px 0 3px 0;"></div>' +

      '<div id="swiss-settings-action" class="swiss-menu-item" style="padding: 5px 8px; border-radius: 6px; cursor: pointer; display: flex; align-items: center; gap: 8px; margin-bottom: 1px; font-size: 13px; font-weight: 400; color: var(--secondary-foreground, #4a4a4a); white-space: nowrap; transition: background 0.1s, color 0.1s;">' +
        '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" style="opacity: 0.8; flex-shrink: 0;"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>' +
        '<span>Project Settings</span>' +
      '</div>' +

      '<div id="swiss-new-convo-action" class="swiss-menu-item" style="padding: 5px 8px; border-radius: 6px; cursor: pointer; display: flex; align-items: center; gap: 8px; margin-bottom: 1px; font-size: 13px; font-weight: 400; color: var(--secondary-foreground, #4a4a4a); white-space: nowrap; transition: background 0.1s, color 0.1s;">' +
        '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" style="opacity: 0.8; flex-shrink: 0;"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>' +
        '<span>New Conversation in Project</span>' +
      '</div>' +

      '<div id="swiss-archive-action" class="swiss-menu-item" style="padding: 5px 8px; border-radius: 6px; cursor: pointer; display: flex; align-items: center; gap: 8px; margin-bottom: 1px; font-size: 13px; font-weight: 400; color: #c2410c; white-space: nowrap; transition: background 0.1s, color 0.1s;">' +
        '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" style="flex-shrink: 0;"><polyline points="21 8 21 21 3 21 3 8"/><rect width="22" height="5" x="1" y="3" rx="1"/><line x1="10" x2="14" y1="12" y2="12"/></svg>' +
        '<span>Hide / Archive Project</span>' +
      '</div>' +

      '<div id="swiss-copy-action" class="swiss-menu-item" style="padding: 5px 8px; border-radius: 6px; cursor: pointer; display: flex; align-items: center; gap: 8px; margin-bottom: 2px; font-size: 13px; font-weight: 400; color: var(--secondary-foreground, #4a4a4a); white-space: nowrap; transition: background 0.1s, color 0.1s;">' +
        '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" style="opacity: 0.8; flex-shrink: 0;"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>' +
        '<span>Copy Project Name</span>' +
      '</div>' +

      '<div style="height: 1px; background: var(--border, rgba(0, 0, 0, 0.075)); margin: 3px 0 2px 0;"></div>' +

      '<div style="display: flex; justify-content: space-between; align-items: center; padding: 2px 6px; margin-top: 2px; margin-bottom: 4px;">' +
        '<span style="font-size: 11px; font-weight: 500; color: var(--muted-foreground, #71717a); letter-spacing: 0.2px;">Set Color</span>' +
        '<button class="swiss-reset-color-btn" style="border: none; background: transparent; color: var(--muted-foreground, #a1a1aa); font-size: 10px; font-weight: 400; cursor: pointer; padding: 0; transition: color 0.1s;" onmouseenter="this.style.color=\'#ef4444\'" onmouseleave="this.style.color=\'var(--muted-foreground, #a1a1aa)\'">Reset</button>' +
      '</div>' +

      '<div style="display: flex; gap: 6px; padding: 1px 6px 4px 6px; align-items: center; justify-content: flex-start;">' +
        PRESET_COLORS.map(function(p) {
          return '<div class="swiss-preset-swatch" data-color="' + p.hex + '" title="' + p.name + '" style="width: 15px; height: 15px; border-radius: 50%%; background: ' + p.hex + '; cursor: pointer; transition: transform 0.12s, box-shadow 0.12s; border: 1.5px solid transparent; flex-shrink: 0;"></div>';
        }).join("") +
        '<div id="swiss-custom-trigger" title="Custom 10x10 Palette" style="width: 15px; height: 15px; border-radius: 50%%; background: conic-gradient(from 0deg, #6c5ce7, #a259c6, #e05260, #e66735, #e69d28, #d4be22, #88b832, #3db862, #2ca88b, #259cb8, #2d7ee8, #4c6ee0, #6c5ce7); cursor: pointer; transition: transform 0.12s, box-shadow 0.12s; border: 1.5px solid transparent; flex-shrink: 0;"></div>' +
      '</div>' +

      '<div id="swiss-custom-grid-container" style="display: none; margin-top: 4px; padding: 2px 4px 4px 4px;">' +
        '<div style="font-size: 10px; font-weight: 500; color: var(--muted-foreground, #71717a); margin-bottom: 4px; padding: 0 2px;">10&times;10 Palette</div>' +
        '<div id="swiss-color-grid" style="display: grid; grid-template-columns: repeat(10, 16px); gap: 3px; justify-content: center; background: var(--muted, rgba(0, 0, 0, 0.03)); padding: 6px; border-radius: 6px; border: 1px solid var(--border, rgba(0, 0, 0, 0.075));">' +
          GRID_COLORS.map(function(c) {
            return '<div class="swiss-grid-cell" data-color="' + c + '" style="width: 16px; height: 16px; border-radius: 50%%; background: ' + c + '; cursor: pointer; transition: transform 0.12s;"></div>';
          }).join("") +
        '</div>' +
      '</div>';

      menu.innerHTML = html;

      document.body.appendChild(menu);

      document.getElementById("swiss-close-menu").onclick = () => menu.remove();

      const setItemHover = (el, isWarn) => {
        if (!el) return;
        el.onmouseenter = () => {
          el.style.background = "var(--secondary, rgba(0, 0, 0, 0.06))";
          el.style.color = isWarn ? "#b45309" : "var(--foreground, #101010)";
        };
        el.onmouseleave = () => {
          el.style.background = "transparent";
          el.style.color = isWarn ? "#c2410c" : "var(--secondary-foreground, #4a4a4a)";
        };
      };

      const settingsBtn = document.getElementById("swiss-settings-action");
      if (settingsBtn) {
        setItemHover(settingsBtn, false);
        settingsBtn.onclick = async () => {
          menu.remove();
          try {
            await fetch("http://127.0.0.1:8765/api/gui/projects/open_settings", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ name: projectName })
            });
          } catch (e) {
            console.warn("[SwissKnife] Open settings failed:", e);
          }
        };
      }

      const newConvoBtn = document.getElementById("swiss-new-convo-action");
      if (newConvoBtn) {
        setItemHover(newConvoBtn, false);
        newConvoBtn.onclick = () => {
          menu.remove();
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
      }

      const archiveBtn = document.getElementById("swiss-archive-action");
      if (archiveBtn) {
        setItemHover(archiveBtn, true);
        archiveBtn.onclick = async () => {
          menu.remove();
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
            if (window.__swissToast) {
              window.__swissToast("Project archived: " + projectName);
            }
          } catch (e) {
            console.warn("[SwissKnife] Archive failed:", e);
          }
        };
      }

      const copyBtn = document.getElementById("swiss-copy-action");
      setItemHover(copyBtn, false);
      copyBtn.onclick = () => {
        navigator.clipboard.writeText(projectName);
        copyBtn.innerHTML = "<span>&check; Copied!</span>";
        setTimeout(() => menu.remove(), 600);
      };

      const customTrig = document.getElementById("swiss-custom-trigger");
      if (customTrig) {
        customTrig.onmouseenter = () => {
          customTrig.style.transform = "scale(1.3)";
          customTrig.style.boxShadow = "0 1px 4px rgba(0,0,0,0.25)";
        };
        customTrig.onmouseleave = () => {
          customTrig.style.transform = "scale(1)";
          customTrig.style.boxShadow = "none";
        };
        customTrig.onclick = (e) => {
          e.stopPropagation();
          const gridBox = document.getElementById("swiss-custom-grid-container");
          if (gridBox) {
            gridBox.style.display = (gridBox.style.display === "none") ? "block" : "none";
          }
        };
      }

      const applyColor = async (colorVal) => {
        menu.remove();
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

      menu.querySelectorAll(".swiss-preset-swatch, .swiss-grid-cell").forEach(el => {
        el.onmouseenter = () => {
          el.style.transform = "scale(1.3)";
          el.style.zIndex = "10";
          el.style.boxShadow = "0 1px 4px rgba(0,0,0,0.25)";
        };
        el.onmouseleave = () => {
          el.style.transform = "scale(1)";
          el.style.zIndex = "1";
          el.style.boxShadow = "none";
        };
        el.onclick = () => applyColor(el.getAttribute("data-color"));
      });

      menu.querySelectorAll(".swiss-reset-color-btn").forEach(btn => {
        btn.onclick = async () => {
          menu.remove();
          try {
            await fetch("http://127.0.0.1:8765/api/gui/projects/delete", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ name: projectName })
            });
          } catch (e) {}
        };
      });

      const onDocClick = (e) => {
        if (!menu.contains(e.target)) {
          menu.remove();
          document.removeEventListener("pointerdown", onDocClick);
        }
      };
      setTimeout(() => document.addEventListener("pointerdown", onDocClick), 50);
    };

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

    if (!window.__swissContextMenuBound) {
      window.__swissContextMenuBound = true;
      document.addEventListener("contextmenu", (e) => {
        const header = e.target.closest('[data-project-card], [class*="group/header"], [data-project-id], [data-project-label]');
        if (header) {
          e.preventDefault();
          e.stopPropagation();
          let pName = header.getAttribute("data-swiss-project") ||
                      header.getAttribute("data-project-label") ||
                      header.querySelector("[data-project-card]")?.getAttribute("data-swiss-project") ||
                      header.querySelector("[data-swiss-project]")?.getAttribute("data-swiss-project");
          if (!pName) {
            const card = header.querySelector("[data-project-card]") || header;
            pName = card.textContent.trim();
          }
          if (pName) {
            window.__showSwissColorMenu(e.clientX, e.clientY, pName);
          }
        }
      }, true);
    }

  window.__swissUpdateTagsAndDraggables = updateTagsAndDraggables;
  updateTagsAndDraggables();

  function triggerSwissUpdate() {
    if (typeof window.__swissUpdateTagsAndDraggables === "function") {
      requestAnimationFrame(window.__swissUpdateTagsAndDraggables);
    }
  }

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

  // 3. MutationObserver watching childList and dynamic attributes
  if (!window.__swissObserverAttached) {
    window.__swissObserverAttached = true;
    const ob = new MutationObserver(() => {
      bindSidebarScroll();
      triggerSwissUpdate();
    });
    ob.observe(document.body, {
      childList: true,
      subtree: true,
      attributes: true,
      attributeFilter: ["data-selected", "data-index", "data-testid"]
    });
  }

  // 4. Fallback interval so no virtualized row ever misses its project styling
  if (!window.__swissIntervalAttached) {
    window.__swissIntervalAttached = true;
    setInterval(() => {
      bindSidebarScroll();
      if (typeof window.__swissUpdateTagsAndDraggables === "function") {
        window.__swissUpdateTagsAndDraggables();
      }
    }, 250);
  }

  return {
    applied: true,
    colorEnabled: isColorEnabled,
    dragEnabled: isDragEnabled,
    taggedCount: document.querySelectorAll("[data-swiss-project]").length
  };
})();`, string(cssJSON), enabled, colorStylingEnabled, dragRearrangeEnabled, string(orderJSON), string(archivedJSON))

	var cmCfg *custommodels.Config
	if cmStore, err := custommodels.NewStore(""); err == nil {
		c := cmStore.GetConfig()
		cmCfg = &c
	}
	customScript := custommodels.GenerateCustomModelsScript(cmCfg)

	var enhCfg *enhancements.EnhancementsConfig
	if enhStore, err := enhancements.NewStore(""); err == nil {
		c := enhStore.GetConfig()
		enhCfg = &c
	}
	enhScript := enhancements.GenerateEnhancementsScript(enhCfg)

	return baseScript + ";\n\n" + customScript + ";\n\n" + enhScript + ";"
}

// GenerateScriptWithCustomModels generates the script including specific custom models configuration.
func GenerateScriptWithCustomModels(cfg *Config, cmCfg *custommodels.Config) string {
	if cmCfg == nil {
		if cmStore, err := custommodels.NewStore(""); err == nil {
			c := cmStore.GetConfig()
			cmCfg = &c
		}
	}
	base := GenerateScript(cfg)
	if cmCfg != nil {
		customScript := custommodels.GenerateCustomModelsScript(cmCfg)
		var enhCfg *enhancements.EnhancementsConfig
		if enhStore, err := enhancements.NewStore(""); err == nil {
			c := enhStore.GetConfig()
			enhCfg = &c
		}
		enhScript := enhancements.GenerateEnhancementsScript(enhCfg)
		return base + ";\n\n" + customScript + ";\n\n" + enhScript + ";"
	}
	return base
}
