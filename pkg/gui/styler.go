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

// ParseColorWithAlpha parses a hex string (3, 4, 6, 8 chars) or rgb/rgba string into r, g, b and alpha components.
func ParseColorWithAlpha(colorStr string) (r, g, b int, alpha float64, err error) {
	colorStr = strings.TrimSpace(colorStr)
	if strings.HasPrefix(colorStr, "#") {
		hex := strings.TrimPrefix(colorStr, "#")
		if len(hex) == 3 {
			r, err1 := strconv.ParseInt(string(hex[0])+string(hex[0]), 16, 32)
			g, err2 := strconv.ParseInt(string(hex[1])+string(hex[1]), 16, 32)
			b, err3 := strconv.ParseInt(string(hex[2])+string(hex[2]), 16, 32)
			if err1 != nil || err2 != nil || err3 != nil {
				return 0, 0, 0, 1.0, fmt.Errorf("invalid 3-digit hex: %s", hex)
			}
			return int(r), int(g), int(b), 1.0, nil
		}
		if len(hex) == 4 {
			r, err1 := strconv.ParseInt(string(hex[0])+string(hex[0]), 16, 32)
			g, err2 := strconv.ParseInt(string(hex[1])+string(hex[1]), 16, 32)
			b, err3 := strconv.ParseInt(string(hex[2])+string(hex[2]), 16, 32)
			a, err4 := strconv.ParseInt(string(hex[3])+string(hex[3]), 16, 32)
			if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
				return 0, 0, 0, 1.0, fmt.Errorf("invalid 4-digit hex: %s", hex)
			}
			return int(r), int(g), int(b), float64(a) / 255.0, nil
		}
		if len(hex) == 6 {
			val, err := strconv.ParseInt(hex, 16, 32)
			if err != nil {
				return 0, 0, 0, 1.0, fmt.Errorf("invalid 6-digit hex: %s", hex)
			}
			return int((val >> 16) & 0xFF), int((val >> 8) & 0xFF), int(val & 0xFF), 1.0, nil
		}
		if len(hex) == 8 {
			val, err := strconv.ParseInt(hex, 16, 64)
			if err != nil {
				return 0, 0, 0, 1.0, fmt.Errorf("invalid 8-digit hex: %s", hex)
			}
			r := int((val >> 24) & 0xFF)
			g := int((val >> 16) & 0xFF)
			b := int((val >> 8) & 0xFF)
			a := int(val & 0xFF)
			return r, g, b, float64(a) / 255.0, nil
		}
		return 0, 0, 0, 1.0, fmt.Errorf("hex string must be 3, 4, 6, or 8 chars: %s", hex)
	} else if strings.HasPrefix(colorStr, "rgba(") {
		inner := strings.TrimSuffix(strings.TrimPrefix(colorStr, "rgba("), ")")
		parts := strings.Split(inner, ",")
		if len(parts) == 4 {
			r, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
			g, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
			b, _ := strconv.Atoi(strings.TrimSpace(parts[2]))
			a, _ := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
			return r, g, b, a, nil
		}
	} else if strings.HasPrefix(colorStr, "rgb(") {
		inner := strings.TrimSuffix(strings.TrimPrefix(colorStr, "rgb("), ")")
		parts := strings.Split(inner, ",")
		if len(parts) == 3 {
			r, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
			g, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
			b, _ := strconv.Atoi(strings.TrimSpace(parts[2]))
			return r, g, b, 1.0, nil
		}
	}
	r, g, b, err = HexToRGB(colorStr)
	return r, g, b, 1.0, err
}

// CalculateColorMultiplier calculates the dynamic opacity multiplier based on color alpha and luminance.
// If the project color already has opacity / alpha or is lighter, the conversation tab dynamically scales
// to achieve the lower opacity without altering the displayed setting percentage.
func CalculateColorMultiplier(r, g, b int, alpha float64) float64 {
	alphaMult := alpha
	if alphaMult <= 0 {
		alphaMult = 1.0
	} else if alphaMult > 1.0 {
		alphaMult = 1.0
	}

	// Perceived luminance using standard ITU-R BT.601 / WCAG coefficients
	y := (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)) / 255.0
	lightnessMult := 1.0
	if y > 0.6 {
		lightnessMult = 1.0 - (y-0.6)*0.75
		if lightnessMult < 0.2 {
			lightnessMult = 0.2
		}
	}

	mult := alphaMult * lightnessMult
	if mult < 0.1 {
		mult = 0.1
	} else if mult > 1.0 {
		mult = 1.0
	}
	return mult
}

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
		opacity = 0.15
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
	sb.WriteString(`
/* Virtual list container must remain transparent to avoid color/shadow spill */
[data-index],
[data-index][data-swiss-project] {
  background: transparent !important;
}
`)

	if cfg.ColorStylingEnabled {
		for project, hex := range cfg.ProjectColors {
		if project == "" || hex == "" {
			continue
		}
		r, g, b, alpha, err := ParseColorWithAlpha(hex)
		if err != nil {
			r, g, b = 11, 87, 208 // fallback to blue
			alpha = 1.0
		}
		mult := CalculateColorMultiplier(r, g, b, alpha)
		effectiveOpacity := opacity * mult
		effectiveHoverOpacity := hoverOpacity * mult
		if effectiveHoverOpacity > 1.0 {
			effectiveHoverOpacity = 1.0
		}
		effectiveSelectedOpacity := selectedOpacity * mult
		if effectiveSelectedOpacity > 1.0 {
			effectiveSelectedOpacity = 1.0
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
`, safeName, safeName, r, g, b, effectiveOpacity, r, g, b, effectiveOpacity, r, g, b, effectiveOpacity, borderWidth, borderStyle, safeName, safeName, r, g, b, effectiveHoverOpacity, safeName, safeName, r, g, b, effectiveOpacity, selectedBorderStyle, fontWeight, safeName, safeName, r, g, b, effectiveHoverOpacity, selectedBorderStyle)
		} else {
			selectedBorderStyle := ""
			activeBgOpacity := effectiveSelectedOpacity
			if isLeftBarMode {
				selectedBorderStyle = fmt.Sprintf("\n  border-left: 3px solid %s !important;", hex)
				activeBgOpacity = effectiveOpacity
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
`, safeName, safeName, r, g, b, effectiveOpacity, r, g, b, effectiveOpacity, r, g, b, effectiveOpacity, borderStyle, safeName, safeName, r, g, b, effectiveHoverOpacity, safeName, safeName, r, g, b, activeBgOpacity, selectedBorderStyle, fontWeight)
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
  position: relative !important;
  display: flex !important;
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
  position: absolute !important;
  top: 50% !important;
  left: 0 !important;
  right: 0 !important;
  width: 100% !important;
  height: 1px !important;
  transform: translateY(-50%) !important;
  background: rgba(148, 163, 184, 0.35) !important;
  transition: background-color 0.18s ease !important;
  z-index: 1 !important;
}
.swiss-convo-tabs-pill {
  position: absolute !important;
  bottom: 50% !important;
  left: 50% !important;
  transform: translateX(-50%) !important;
  margin-bottom: 1px !important;
  z-index: 2 !important;
  display: inline-flex !important;
  align-items: center !important;
  justify-content: center !important;
  width: 16px !important;
  height: 11px !important;
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
.swiss-project-bottom-spacer {
  position: relative !important;
  height: 32px !important;
  width: 100% !important;
  pointer-events: none !important;
  box-sizing: border-box !important;
}
.swiss-project-spacer-line {
  position: absolute !important;
  top: 50% !important;
  left: 0 !important;
  right: 0 !important;
  width: 100% !important;
  height: 1px !important;
  transform: translateY(-50%) !important;
  background: rgba(148, 163, 184, 0.35) !important;
  z-index: 1 !important;
}
[data-theme="dark"] .swiss-convo-tabs-line,
.dark .swiss-convo-tabs-line,
[data-theme="dark"] .swiss-project-spacer-line,
.dark .swiss-project-spacer-line {
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
  transform: translateX(-50%) scale(1.18) !important;
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

	tabsMode := "dynamic"
	if cfg != nil && cfg.ConversationTabsMode != "" {
		tabsMode = cfg.ConversationTabsMode
	}
	tabsFixedLimit := 6
	if cfg != nil && cfg.ConversationTabsFixedLimit > 0 {
		tabsFixedLimit = cfg.ConversationTabsFixedLimit
	}
	tabsAgeThreshold := "14d"
	if cfg != nil && cfg.ConversationTabsAgeThreshold != "" {
		tabsAgeThreshold = cfg.ConversationTabsAgeThreshold
	}
	tabsMin := 3
	if cfg != nil && cfg.ConversationTabsMin > 0 {
		tabsMin = cfg.ConversationTabsMin
	}
	tabsMax := 6
	if cfg != nil && cfg.ConversationTabsMax > 0 {
		tabsMax = cfg.ConversationTabsMax
	}
	replaceSeeAllTriangle := true
	if cfg != nil {
		replaceSeeAllTriangle = cfg.ReplaceSeeAllTriangle
	}
	consistentProjectSpacing := true
	if cfg != nil {
		consistentProjectSpacing = cfg.ConsistentProjectSpacing
	}
	consistentProjectSpacingLine := true
	if cfg != nil {
		consistentProjectSpacingLine = cfg.ConsistentProjectSpacingLine
	}
	isBorderMode := cfg != nil && cfg.ActiveConversationIndicator == "border"
	borderWidth := "2px"
	if cfg != nil && cfg.ActiveConversationBorderWidth != "" {
		borderWidth = cfg.ActiveConversationBorderWidth
	}
	isLeftBarMode := cfg != nil && (cfg.ActiveConversationIndicator == "left_bar" || cfg.ActiveConversationIndicator == "left_accent_bar" || cfg.SolidLeftEdge)
	fontWeight := "600"
	if cfg != nil && !cfg.ActiveConversationBold {
		fontWeight = "400"
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
  const replaceSeeAllTriangle = %t;
  const consistentProjectSpacing = %t;
  const consistentProjectSpacingLine = %t;
  const isBorderMode = %t;
  const borderWidth = %q;
  const isLeftBarMode = %t;
  const fontWeight = %q;
  window.__swissArchivedProjects = Array.isArray(archivedProjects) ? archivedProjects : [];
  window.__swissIsBorderMode = isBorderMode;
  window.__swissBorderWidth = borderWidth;
  window.__swissIsLeftBarMode = isLeftBarMode;
  window.__swissFontWeight = fontWeight;

  // 1. Manage stylesheet
  let styleEl = document.getElementById("antigravity-swiss-styles");
  if (!isEnabled) {
    if (styleEl) styleEl.remove();
    document.querySelectorAll("[data-swiss-project]").forEach(el => el.removeAttribute("data-swiss-project"));
    document.querySelectorAll(".swiss-project-bottom-spacer").forEach(el => el.remove());
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

      // Compute uncontracted project spacer indices
      const spacerIndices = new Set();
      if (consistentProjectSpacing) {
        const projects = [];
        let curProj = null;
        items.forEach((it, idx) => {
          if (it.type === "header") {
            curProj = {
              headerIdx: idx,
              groupId: it.id.replace("header-", ""),
              rows: [],
              hasShowMore: false
            };
            projects.push(curProj);
          } else if (it.type === "row" && curProj) {
            curProj.rows.push(idx);
          } else if (it.type === "show-more" && curProj) {
            curProj.hasShowMore = true;
          }
        });

        projects.forEach(p => {
          if (hiddenIndices.has(p.headerIdx)) return;
          if (!p.hasShowMore) {
            const visibleRows = p.rows.filter(rIdx => !hiddenIndices.has(rIdx));
            if (visibleRows.length > 0) {
              spacerIndices.add(visibleRows[visibleRows.length - 1]);
            } else {
              spacerIndices.add(p.headerIdx);
            }
          }
        });
      } else {
        container.querySelectorAll(".swiss-project-bottom-spacer").forEach(el => el.remove());
      }

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
            const sp = el.querySelector(".swiss-project-bottom-spacer");
            if (sp) sp.remove();
            return;
          }

          el.style.display = "";

          // Consistent blank spacing below projects without contracted conversation tabs
          let spacer = el.querySelector(".swiss-project-bottom-spacer");
          if (consistentProjectSpacing && spacerIndices.has(idx)) {
            if (!spacer) {
              spacer = document.createElement("div");
              spacer.className = "swiss-project-bottom-spacer";
              el.appendChild(spacer);
            }
            let line = spacer.querySelector(".swiss-project-spacer-line");
            if (consistentProjectSpacingLine) {
              if (!line) {
                line = document.createElement("div");
                line.className = "swiss-project-spacer-line";
                spacer.appendChild(line);
              }
            } else if (line) {
              line.remove();
            }
          } else if (spacer) {
            spacer.remove();
          }

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

          // Project show-more button handling: sleek divider with centered solid triangle
          if (item.type === "show-more" || (el.querySelector("button") && (el.querySelector("button").textContent.includes("See all") || el.querySelector("button").textContent.includes("See less") || el.querySelector("button").getAttribute("data-swiss-divider") === "true"))) {
            const btn = el.querySelector("button");
            if (btn) {
              if (replaceSeeAllTriangle) {
                const rawLabel = item.label || btn.getAttribute("data-swiss-orig-label") || btn.textContent.trim();
                const isSeeAll = rawLabel.toLowerCase().includes("see all");
                const symbol = isSeeAll ? "▾" : "▴";
                btn.setAttribute("data-swiss-divider", "true");
                btn.setAttribute("data-swiss-orig-label", rawLabel);
                btn.setAttribute("title", rawLabel);
                const curTriangle = btn.querySelector(".swiss-convo-tabs-triangle");
                if (!curTriangle || curTriangle.textContent !== symbol) {
                  btn.innerHTML = '<div class="swiss-convo-tabs-divider">' +
                    '<div class="swiss-convo-tabs-pill"><span class="swiss-convo-tabs-triangle">' + symbol + '</span></div>' +
                    '<div class="swiss-convo-tabs-line"></div>' +
                  '</div>';
                }
              } else if (btn.getAttribute("data-swiss-divider") === "true") {
                btn.removeAttribute("data-swiss-divider");
                const orig = btn.getAttribute("data-swiss-orig-label") || item.label || btn.getAttribute("title") || "See all";
                btn.textContent = orig;
              }
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

  // 3a. Dynamic scoped in-DOM project styling & cross-window sync
  window.__swissDynamicColors = window.__swissDynamicColors || {};

  function renderDynamicProjectStyles(projName, colorHex) {
    try {
      if (colorHex) {
        window.__swissDynamicColors[projName] = colorHex;
      } else if (projName) {
        delete window.__swissDynamicColors[projName];
      }

      let dynStyleEl = document.getElementById("antigravity-swiss-dynamic-colors");
      if (!dynStyleEl) {
        dynStyleEl = document.createElement("style");
        dynStyleEl.id = "antigravity-swiss-dynamic-colors";
        (document.head || document.documentElement).appendChild(dynStyleEl);
      }

      let cssRules = [];
      cssRules.push(
        '/* Virtual list container must remain transparent to avoid color/shadow spill */' +
        '[data-index],' +
        '[data-index][data-swiss-project] {' +
        '  background: transparent !important;' +
        '}'
      );
      for (const [pName, hex] of Object.entries(window.__swissDynamicColors)) {
        if (!pName || !hex) continue;
        const safeP = pName.replace(/"/g, '\\\\\"');

        let r = 11, g = 87, b = 208;
        if (hex.startsWith("#")) {
          const rawHex = hex.slice(1);
          if (rawHex.length === 3) {
            r = parseInt(rawHex[0] + rawHex[0], 16) || 11;
            g = parseInt(rawHex[1] + rawHex[1], 16) || 87;
            b = parseInt(rawHex[2] + rawHex[2], 16) || 208;
          } else if (rawHex.length === 6) {
            r = parseInt(rawHex.slice(0, 2), 16) || 11;
            g = parseInt(rawHex.slice(2, 4), 16) || 87;
            b = parseInt(rawHex.slice(4, 6), 16) || 208;
          }
        }

        const baseOpacity = 0.15;
        const lum = (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255.0;
        const lightMult = lum > 0.6 ? Math.max(0.2, 1.0 - (lum - 0.6) * 0.75) : 1.0;
        const effOpacity = (baseOpacity * lightMult).toFixed(2);
        const effHoverOpacity = Math.min(1.0, (baseOpacity + 0.08) * lightMult).toFixed(2);
        const effSelectedOpacity = Math.min(1.0, (baseOpacity + 0.16) * lightMult).toFixed(2);

        cssRules.push(
          '/* Dynamic Project Label Card */' +
          '[data-swiss-project="' + safeP + '"][data-project-card="true"],' +
          '[data-swiss-project="' + safeP + '"] [data-project-card="true"],' +
          '[data-swiss-project="' + safeP + '"][data-project-card],' +
          '[data-swiss-project="' + safeP + '"] [data-project-card],' +
          '[data-project-card][data-swiss-project="' + safeP + '"] {' +
          '  background-color: ' + hex + ' !important;' +
          '  color: #ffffff !important;' +
          '  border-radius: 8px !important;' +
          '  border: none !important;' +
          '}' +
          '[data-swiss-project="' + safeP + '"][data-project-card="true"] *,' +
          '[data-swiss-project="' + safeP + '"] [data-project-card="true"] *,' +
          '[data-swiss-project="' + safeP + '"][data-project-card] *,' +
          '[data-swiss-project="' + safeP + '"] [data-project-card] *,' +
          '[data-project-card][data-swiss-project="' + safeP + '"] * {' +
          '  color: #ffffff !important;' +
          '}' +
          '[data-swiss-project="' + safeP + '"] [class*="group/header"] button,' +
          '[data-swiss-project="' + safeP + '"] button[aria-label="Project options"] svg,' +
          '[data-swiss-project="' + safeP + '"] button[aria-label*="conversation"] svg {' +
          '  color: #ffffff !important;' +
          '  fill: #ffffff !important;' +
          '}'
        );

        if (isBorderMode) {
          let selectedBorderStyle = 'border: ' + borderWidth + ' solid ' + hex + ' !important;';
          if (isLeftBarMode) {
            selectedBorderStyle += ' border-left: 3px solid ' + hex + ' !important;';
          }
          cssRules.push(
            '/* Conversation rows light tint, border-mode */' +
            '[data-swiss-project="' + safeP + '"][data-testid="conversation-row-sidebar"],' +
            '[data-swiss-project="' + safeP + '"] [data-testid="conversation-row-sidebar"],' +
            '[data-testid="conversation-row-sidebar"][data-swiss-project="' + safeP + '"] {' +
            '  --sidebar-secondary: rgba(' + r + ', ' + g + ', ' + b + ', ' + effOpacity + ') !important;' +
            '  --sidebar-muted: rgba(' + r + ', ' + g + ', ' + b + ', ' + effOpacity + ') !important;' +
            '  background-color: rgba(' + r + ', ' + g + ', ' + b + ', ' + effOpacity + ') !important;' +
            '  border: ' + borderWidth + ' solid transparent !important;' +
            '  border-radius: 8px !important;' +
            '  transition: background-color 0.15s ease, border-color 0.15s ease !important;' +
            '}' +
            '[data-swiss-project="' + safeP + '"][data-testid="conversation-row-sidebar"]:hover,' +
            '[data-swiss-project="' + safeP + '"] [data-testid="conversation-row-sidebar"]:hover,' +
            '[data-testid="conversation-row-sidebar"][data-swiss-project="' + safeP + '"]:hover {' +
            '  background-color: rgba(' + r + ', ' + g + ', ' + b + ', ' + effHoverOpacity + ') !important;' +
            '}' +
            '[data-swiss-project="' + safeP + '"][data-testid="conversation-row-sidebar"][data-selected="true"],' +
            '[data-swiss-project="' + safeP + '"] [data-testid="conversation-row-sidebar"][data-selected="true"],' +
            '[data-testid="conversation-row-sidebar"][data-swiss-project="' + safeP + '"][data-selected="true"] {' +
            '  background-color: rgba(' + r + ', ' + g + ', ' + b + ', ' + effOpacity + ') !important;' +
            '  ' + selectedBorderStyle +
            '  font-weight: ' + fontWeight + ' !important;' +
            '}' +
            '[data-swiss-project="' + safeP + '"][data-testid="conversation-row-sidebar"][data-selected="true"]:hover,' +
            '[data-swiss-project="' + safeP + '"] [data-testid="conversation-row-sidebar"][data-selected="true"]:hover,' +
            '[data-testid="conversation-row-sidebar"][data-swiss-project="' + safeP + '"][data-selected="true"]:hover {' +
            '  background-color: rgba(' + r + ', ' + g + ', ' + b + ', ' + effHoverOpacity + ') !important;' +
            '  ' + selectedBorderStyle +
            '}'
          );
        } else {
          let selectedBorderStyle = '';
          let activeBgOpacity = effSelectedOpacity;
          if (isLeftBarMode) {
            selectedBorderStyle = 'border-left: 3px solid ' + hex + ' !important;';
            activeBgOpacity = effOpacity;
          }
          cssRules.push(
            '/* Conversation rows light tint */' +
            '[data-swiss-project="' + safeP + '"][data-testid="conversation-row-sidebar"],' +
            '[data-swiss-project="' + safeP + '"] [data-testid="conversation-row-sidebar"],' +
            '[data-testid="conversation-row-sidebar"][data-swiss-project="' + safeP + '"] {' +
            '  --sidebar-secondary: rgba(' + r + ', ' + g + ', ' + b + ', ' + effOpacity + ') !important;' +
            '  --sidebar-muted: rgba(' + r + ', ' + g + ', ' + b + ', ' + effOpacity + ') !important;' +
            '  background-color: rgba(' + r + ', ' + g + ', ' + b + ', ' + effOpacity + ') !important;' +
            '  border: none !important;' +
            '  border-left: none !important;' +
            '  border-radius: 8px !important;' +
            '  transition: background-color 0.15s ease !important;' +
            '}' +
            '[data-swiss-project="' + safeP + '"][data-testid="conversation-row-sidebar"]:hover,' +
            '[data-swiss-project="' + safeP + '"] [data-testid="conversation-row-sidebar"]:hover,' +
            '[data-testid="conversation-row-sidebar"][data-swiss-project="' + safeP + '"]:hover {' +
            '  background-color: rgba(' + r + ', ' + g + ', ' + b + ', ' + effHoverOpacity + ') !important;' +
            '}' +
            '[data-swiss-project="' + safeP + '"][data-testid="conversation-row-sidebar"][data-selected="true"],' +
            '[data-swiss-project="' + safeP + '"] [data-testid="conversation-row-sidebar"][data-selected="true"],' +
            '[data-testid="conversation-row-sidebar"][data-swiss-project="' + safeP + '"][data-selected="true"] {' +
            '  background-color: rgba(' + r + ', ' + g + ', ' + b + ', ' + activeBgOpacity + ') !important;' +
            (selectedBorderStyle ? '  ' + selectedBorderStyle : '') +
            '  font-weight: ' + fontWeight + ' !important;' +
            '}'
          );
        }
      }
      dynStyleEl.textContent = cssRules.join("\n");
      if (typeof window.__swissUpdateTagsAndDraggables === "function") {
        window.__swissUpdateTagsAndDraggables();
      }
    } catch (e) {
      console.warn("[SwissKnife] renderDynamicProjectStyles error:", e);
    }
  }
  window.__swissRenderDynamicProjectStyles = renderDynamicProjectStyles;

  // Cross-window BroadcastChannel setup
  if (!window.__swissSyncChannel && typeof BroadcastChannel !== "undefined") {
    try {
      window.__swissSyncChannel = new BroadcastChannel("swiss-sync");
      window.__swissSyncChannel.onmessage = (ev) => {
        const msg = ev?.data;
        if (!msg) return;
        if (msg.type === "color-update" && msg.project) {
          renderDynamicProjectStyles(msg.project, msg.color);
        } else if (msg.type === "color-delete" && msg.project) {
          renderDynamicProjectStyles(msg.project, null);
        }
      };
    } catch (_) {}
  }

  // Track the most recently clicked project options button or right-clicked project
  if (!window.__swissProjectOptionsTrackerBound) {
    window.__swissProjectOptionsTrackerBound = true;

    const trackProjectTarget = (target) => {
      if (!target) return;
      const header = target.closest('[data-swiss-project], [data-project-label], [data-project-card], [class*="group/header"], [data-project-id], [data-testid="lifted-context-menu-trigger"]');
      if (header) {
        let pName = header.getAttribute("data-swiss-project") ||
                    header.getAttribute("data-project-label");
        if (!pName) {
          const card = header.matches("[data-project-card]") ? header : header.querySelector("[data-project-card]");
          if (card) {
            pName = card.getAttribute("data-swiss-project") || card.getAttribute("data-project-label");
            if (!pName) {
              const span = card.querySelector("span.truncate, span");
              pName = span ? span.textContent.trim() : card.textContent.trim();
            }
          }
        }
        if (!pName) {
          const card = header.querySelector("[data-project-card]") || header;
          pName = card ? card.textContent.trim() : "";
        }
        if (pName) {
          window.__swissLastClickedProject = { name: pName, time: Date.now() };
        }
      }
    };

    document.addEventListener("contextmenu", (e) => {
      trackProjectTarget(e.target);
      if (typeof window.__swissEnhanceProjectOptionsMenu === "function") {
        requestAnimationFrame(window.__swissEnhanceProjectOptionsMenu);
        setTimeout(window.__swissEnhanceProjectOptionsMenu, 50);
        setTimeout(window.__swissEnhanceProjectOptionsMenu, 150);
      }
    }, { capture: true, passive: true });

    document.addEventListener("pointerdown", (e) => {
      if (e.button === 2) {
        trackProjectTarget(e.target);
      } else {
        const btn = e.target.closest('button[aria-label="Project options"]');
        if (btn) {
          trackProjectTarget(btn);
        }
      }
      if (typeof window.__swissEnhanceProjectOptionsMenu === "function") {
        requestAnimationFrame(window.__swissEnhanceProjectOptionsMenu);
        setTimeout(window.__swissEnhanceProjectOptionsMenu, 50);
        setTimeout(window.__swissEnhanceProjectOptionsMenu, 150);
      }
    }, { capture: true, passive: true });
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
      if (!projectName && window.__swissLastClickedProject && (Date.now() - window.__swissLastClickedProject.time < 15000)) {
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

        // 1. Optimistic client-side in-DOM scoped rendering immediately
        renderDynamicProjectStyles(projectName, hex);

        // 2. Cross-window sync via BroadcastChannel
        if (window.__swissSyncChannel) {
          try {
            window.__swissSyncChannel.postMessage({ type: "color-update", project: projectName, color: hex });
          } catch (_) {}
        }

        if (window.__swissToast) {
          window.__swissToast("Updated color for " + projectName);
        }

        // 3. Background persistence to daemon
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

          // 1. Optimistic client-side in-DOM scoped styling removal immediately
          renderDynamicProjectStyles(projectName, null);

          // 2. Cross-window sync via BroadcastChannel
          if (window.__swissSyncChannel) {
            try {
              window.__swissSyncChannel.postMessage({ type: "color-delete", project: projectName });
            } catch (_) {}
          }

          if (window.__swissToast) {
            window.__swissToast("Reset color for " + projectName);
          }

          // 3. Background persistence to dedicated color delete endpoint
          try {
            await fetch("http://127.0.0.1:8765/api/gui/color/delete", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ name: projectName })
            });
          } catch (e) {
            console.warn("[SwissKnife] Reset color failed:", e);
          }
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
      } finally {
        setTimeout(() => {
          isUpdatingSwiss = false;
        }, 50);
      }
    });
  }

  triggerSwissUpdate();

  // 1. Passive scroll listeners
  if (!window.__swissScrollCaptured) {
    window.__swissScrollCaptured = true;
    window.addEventListener("scroll", triggerSwissUpdate, { passive: true });
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
    let menuAppeared = false;
    for (const m of mutations) {
      const t = m.target;
      if (!t) continue;
      if (t.id === "antigravity-swiss-styles" || 
          t.id === "antigravity-swiss-dynamic-colors" || 
          t.id === "swiss-project-context-menu" || 
          t.id === "swiss-custom-models-section" || 
          t.id === "swiss-toast-notification" ||
          (typeof t.id === "string" && t.id.startsWith("swiss-")) ||
          t.hasAttribute?.("data-swiss-divider") || 
          t.hasAttribute?.("data-swiss-project") ||
          t.classList?.contains("swiss-convo-tabs-divider") || 
          t.classList?.contains("swiss-convo-tabs-pill") ||
          t.classList?.contains("swiss-project-bottom-spacer") ||
          t.classList?.contains("swiss-project-spacer-line") ||
          t.closest?.("[id^='swiss-'], [class*='swiss-'], [data-swiss-project]")) {
        continue;
      }
      if (t.getAttribute?.("role") === "menu" || t.querySelector?.('[role="menu"]')) {
        menuAppeared = true;
      }
      relevant = true;
    }
    if (!relevant) return;

    bindSidebarScroll();
    if (menuAppeared && typeof window.__swissEnhanceProjectOptionsMenu === "function") {
      requestAnimationFrame(window.__swissEnhanceProjectOptionsMenu);
    }
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
  }, 2000);

  return {
    applied: true,
    colorEnabled: isColorEnabled,
    dragEnabled: isDragEnabled,
    taggedCount: document.querySelectorAll("[data-swiss-project]").length
  };
})();`, string(cssJSON), enabled, colorStylingEnabled, dragRearrangeEnabled, string(orderJSON), string(archivedJSON), tabsMode, tabsFixedLimit, tabsAgeThreshold, tabsMin, tabsMax, replaceSeeAllTriangle, consistentProjectSpacing, consistentProjectSpacingLine, isBorderMode, borderWidth, isLeftBarMode, fontWeight)

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
