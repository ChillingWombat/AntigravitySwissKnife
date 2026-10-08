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
	if cfg == nil || !cfg.Enabled || (!cfg.ColorStylingEnabled && len(cfg.ArchivedProjects) == 0 && cfg.ConversationTabsMode == "" && !cfg.ReplaceSeeAllTriangle && !cfg.ConsistentProjectSpacing) {
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

/* Unselected + New Conversation Button: normalize to clean transparent sidebar style */
body:has([data-testid="conversation-view"]) [data-testid="new-conversation-button"],
body:has([data-testid="conversation-row-sidebar"][data-selected="true"]) [data-testid="new-conversation-button"],
body:has([data-testid="history-button"].active) [data-testid="new-conversation-button"],
body:has([data-testid="history-button"][data-selected="true"]) [data-testid="new-conversation-button"],
body:has([data-testid="history-button"][aria-selected="true"]) [data-testid="new-conversation-button"],
body:has([data-testid="automations-button"].active) [data-testid="new-conversation-button"],
body:has([data-testid="automations-button"][data-selected="true"]) [data-testid="new-conversation-button"],
body:has([data-testid="automations-button"][aria-selected="true"]) [data-testid="new-conversation-button"],
[data-swiss-stage-active] [data-testid="new-conversation-button"],
body:has([data-swiss-stage-active]) [data-testid="new-conversation-button"],
body:has(.swiss-left-nav-tab.active) [data-testid="new-conversation-button"],
[data-testid="new-conversation-button"][data-selected="false"],
[data-testid="new-conversation-button"].unselected {
  background-color: transparent !important;
  border-color: transparent !important;
  border: none !important;
  color: var(--secondary-foreground, #71717a) !important;
  box-shadow: none !important;
  outline: none !important;
}

:is(.dark, [data-theme="dark"]) body:has([data-testid="conversation-view"]) [data-testid="new-conversation-button"],
:is(.dark, [data-theme="dark"]) body:has([data-testid="conversation-row-sidebar"][data-selected="true"]) [data-testid="new-conversation-button"],
:is(.dark, [data-theme="dark"]) body:has([data-testid="history-button"].active) [data-testid="new-conversation-button"],
:is(.dark, [data-theme="dark"]) body:has([data-testid="history-button"][data-selected="true"]) [data-testid="new-conversation-button"],
:is(.dark, [data-theme="dark"]) body:has([data-testid="history-button"][aria-selected="true"]) [data-testid="new-conversation-button"],
:is(.dark, [data-theme="dark"]) body:has([data-testid="automations-button"].active) [data-testid="new-conversation-button"],
:is(.dark, [data-theme="dark"]) body:has([data-testid="automations-button"][data-selected="true"]) [data-testid="new-conversation-button"],
:is(.dark, [data-theme="dark"]) body:has([data-testid="automations-button"][aria-selected="true"]) [data-testid="new-conversation-button"],
:is(.dark, [data-theme="dark"]) [data-swiss-stage-active] [data-testid="new-conversation-button"],
:is(.dark, [data-theme="dark"]) body:has([data-swiss-stage-active]) [data-testid="new-conversation-button"],
:is(.dark, [data-theme="dark"]) body:has(.swiss-left-nav-tab.active) [data-testid="new-conversation-button"],
:is(.dark, [data-theme="dark"]) [data-testid="new-conversation-button"][data-selected="false"],
:is(.dark, [data-theme="dark"]) [data-testid="new-conversation-button"].unselected {
  background-color: transparent !important;
  border-color: transparent !important;
  border: none !important;
  color: var(--secondary-foreground, #94a3b8) !important;
  box-shadow: none !important;
  outline: none !important;
}

body:has([data-testid="conversation-view"]) [data-testid="new-conversation-button"]:hover,
body:has([data-testid="conversation-row-sidebar"][data-selected="true"]) [data-testid="new-conversation-button"]:hover,
body:has([data-testid="history-button"].active) [data-testid="new-conversation-button"]:hover,
body:has([data-testid="history-button"][data-selected="true"]) [data-testid="new-conversation-button"]:hover,
body:has([data-testid="automations-button"].active) [data-testid="new-conversation-button"]:hover,
body:has([data-testid="automations-button"][data-selected="true"]) [data-testid="new-conversation-button"]:hover,
[data-swiss-stage-active] [data-testid="new-conversation-button"]:hover,
body:has([data-swiss-stage-active]) [data-testid="new-conversation-button"]:hover,
body:has(.swiss-left-nav-tab.active) [data-testid="new-conversation-button"]:hover,
[data-testid="new-conversation-button"][data-selected="false"]:hover,
[data-testid="new-conversation-button"].unselected:hover {
  background-color: var(--sidebar-muted, rgba(148, 163, 184, 0.15)) !important;
  color: var(--foreground, #1e293b) !important;
}

:is(.dark, [data-theme="dark"]) body:has([data-testid="conversation-view"]) [data-testid="new-conversation-button"]:hover,
:is(.dark, [data-theme="dark"]) body:has([data-testid="conversation-row-sidebar"][data-selected="true"]) [data-testid="new-conversation-button"]:hover,
:is(.dark, [data-theme="dark"]) body:has([data-testid="history-button"].active) [data-testid="new-conversation-button"]:hover,
:is(.dark, [data-theme="dark"]) body:has([data-testid="history-button"][data-selected="true"]) [data-testid="new-conversation-button"]:hover,
:is(.dark, [data-theme="dark"]) body:has([data-testid="automations-button"].active) [data-testid="new-conversation-button"]:hover,
:is(.dark, [data-theme="dark"]) body:has([data-testid="automations-button"][data-selected="true"]) [data-testid="new-conversation-button"]:hover,
:is(.dark, [data-theme="dark"]) [data-swiss-stage-active] [data-testid="new-conversation-button"]:hover,
:is(.dark, [data-theme="dark"]) body:has([data-swiss-stage-active]) [data-testid="new-conversation-button"]:hover,
:is(.dark, [data-theme="dark"]) body:has(.swiss-left-nav-tab.active) [data-testid="new-conversation-button"]:hover,
:is(.dark, [data-theme="dark"]) [data-testid="new-conversation-button"][data-selected="false"]:hover,
:is(.dark, [data-theme="dark"]) [data-testid="new-conversation-button"].unselected:hover {
  background-color: rgba(255, 255, 255, 0.08) !important;
  color: #f1f5f9 !important;
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

			lum := (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)) / 255.0
			textColor := "#ffffff"
			if lum > 0.6 {
				textColor = "#0f172a"
			}

			sb.WriteString(fmt.Sprintf(`
/* Project Label Card */
[data-swiss-project="%s"][data-project-card="true"],
[data-swiss-project="%s"] [data-project-card="true"] {
  background-color: %s !important;
  color: %s !important;
  border-radius: 8px !important;
  border: none !important;
}
[data-swiss-project="%s"][data-project-card="true"] *,
[data-swiss-project="%s"] [data-project-card="true"] * {
  color: %s !important;
}

/* Project Header Action Buttons (⋮ and + buttons) */
[data-swiss-project="%s"] [class*="group/header"] button,
[data-swiss-project="%s"] [class*="group/header"] a,
[data-swiss-project="%s"] a[aria-label*="conversation" i] svg,
[data-swiss-project="%s"] button[aria-label="Project options"] svg,
[data-swiss-project="%s"] button[aria-label*="conversation"] svg {
  color: %s !important;
  fill: %s !important;
}

/* Automate Tasks / Sidecar Workspace Overlay Icon Badge */
[data-swiss-project="%s"] [data-testid*="sidecar-workspace-overlay"],
[data-swiss-project="%s"] [data-project-card] [data-testid*="sidecar-workspace-overlay"],
[data-swiss-project="%s"] .group\/headerbtn [data-testid*="sidecar-workspace-overlay"],
[data-swiss-project="%s"] [data-project-card] span[class*="rounded-full"][class*="-bottom"],
[data-project-card][data-swiss-project="%s"] [data-testid*="sidecar-workspace-overlay"],
[data-project-card][data-swiss-project="%s"] span[class*="rounded-full"][class*="-bottom"] {
  background-color: %s !important;
  color: %s !important;
}
[data-swiss-project="%s"] [data-testid*="sidecar-workspace-overlay"]:hover,
[data-swiss-project="%s"] [data-project-card]:hover [data-testid*="sidecar-workspace-overlay"],
[data-swiss-project="%s"] .group\/headerbtn:hover [data-testid*="sidecar-workspace-overlay"] {
  background-color: %s !important;
}
[data-swiss-project="%s"] [data-testid*="sidecar-workspace-overlay"] svg,
[data-swiss-project="%s"] [data-project-card] [data-testid*="sidecar-workspace-overlay"] svg,
[data-swiss-project="%s"] .group\/headerbtn [data-testid*="sidecar-workspace-overlay"] svg,
[data-swiss-project="%s"] [data-project-card] span[class*="rounded-full"][class*="-bottom"] svg,
[data-project-card][data-swiss-project="%s"] [data-testid*="sidecar-workspace-overlay"] svg,
[data-project-card][data-swiss-project="%s"] span[class*="rounded-full"][class*="-bottom"] svg {
  color: %s !important;
  fill: %s !important;
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
`, safeName, safeName, hex, textColor, safeName, safeName, textColor, safeName, safeName, safeName, safeName, safeName, textColor, textColor,
				safeName, safeName, safeName, safeName, safeName, safeName, hex, textColor,
				safeName, safeName, safeName, hex,
				safeName, safeName, safeName, safeName, safeName, safeName, textColor, textColor,
				rowCSS, safeName, safeName, safeName, safeName, safeName, safeName))
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
/* Antigravity Swiss Knife - Conversation Tabs Divider (Show / Hide Button) */
[data-index]:has(.swiss-project-bottom-spacer) {
  z-index: 2 !important;
  overflow: visible !important;
}
[data-index]:has(button[data-swiss-divider="true"]),
[data-index]:has([data-swiss-divider="true"]) {
  display: flex !important;
  flex-direction: column !important;
  align-items: center !important;
  justify-content: flex-start !important;
  width: 100% !important;
  height: 14px !important;
  min-height: 14px !important;
  max-height: 14px !important;
  padding: 0 !important;
  margin: 0 !important;
  background: transparent !important;
  border: none !important;
  box-shadow: none !important;
  overflow: visible !important;
  z-index: 2 !important;
}
[data-index]:has(button[data-swiss-divider="true"]) > *:not(.swiss-project-bottom-spacer),
div:has(> button[data-swiss-divider="true"]),
div:has(> [data-swiss-divider="true"]) {
  display: flex !important;
  align-items: center !important;
  justify-content: center !important;
  width: 100% !important;
  height: 10px !important;
  min-height: 10px !important;
  max-height: 10px !important;
  padding: 0 !important;
  padding-left: 0 !important;
  padding-right: 0 !important;
  margin: 0 auto !important;
  background: transparent !important;
  border: none !important;
  box-shadow: none !important;
  overflow: visible !important;
}
button[data-swiss-divider="true"] {
  display: flex !important;
  align-items: center !important;
  justify-content: center !important;
  width: 100% !important;
  height: 10px !important;
  min-height: 10px !important;
  max-height: 10px !important;
  padding: 0 !important;
  padding-left: 0 !important;
  padding-right: 0 !important;
  margin: 0 auto !important;
  line-height: 1 !important;
  background: transparent !important;
  border: none !important;
  outline: none !important;
  cursor: pointer !important;
  box-shadow: none !important;
  overflow: visible !important;
}
button[data-swiss-divider="true"]:hover {
  background: transparent !important;
}
.swiss-convo-tabs-divider {
  display: flex !important;
  align-items: center !important;
  justify-content: center !important;
  width: 100% !important;
  height: 100% !important;
  padding: 0 !important;
  margin: 0 auto !important;
  box-sizing: border-box !important;
  cursor: pointer !important;
  user-select: none !important;
  background: transparent !important;
  overflow: visible !important;
}
.swiss-convo-tabs-pill {
  display: inline-flex !important;
  align-items: center !important;
  justify-content: center !important;
  width: 14px !important;
  height: 10px !important;
  color: #64748b !important;
  font-size: 8px !important;
  background: transparent !important;
  transition: all 0.18s ease !important;
  z-index: 3 !important;
}
.swiss-convo-tabs-triangle {
  display: inline-block !important;
  font-size: 8px !important;
  line-height: 1 !important;
  text-align: center !important;
  transition: transform 0.2s cubic-bezier(0.4, 0, 0.2, 1) !important;
}
[data-index]:not(:has(button[data-swiss-divider="true"])) .swiss-project-bottom-spacer {
  position: absolute !important;
  bottom: 0 !important;
  left: 0 !important;
  width: 100% !important;
  height: 0 !important;
  pointer-events: none !important;
  box-sizing: border-box !important;
  background: transparent !important;
  overflow: visible !important;
  margin: 0 !important;
  padding: 0 !important;
}
[data-index]:has(button[data-swiss-divider="true"]) .swiss-project-bottom-spacer {
  position: relative !important;
  width: 100% !important;
  height: 4px !important;
  pointer-events: none !important;
  box-sizing: border-box !important;
  background: transparent !important;
  overflow: visible !important;
  margin: 0 !important;
  padding: 0 !important;
}
.swiss-project-bottom-spacer {
  position: relative !important;
  width: 100% !important;
  pointer-events: none !important;
  box-sizing: border-box !important;
  background: transparent !important;
  overflow: visible !important;
  margin: 0 !important;
  padding: 0 !important;
}
.swiss-project-spacer-line {
  position: absolute !important;
  left: 0 !important;
  right: 0 !important;
  width: 100% !important;
  height: 1px !important;
  background: rgba(148, 163, 184, 0.35) !important;
  z-index: 2 !important;
  pointer-events: none !important;
}
[data-index]:has(button[data-swiss-divider="true"]) .swiss-project-spacer-line {
  top: 2px !important;
  transform: translateY(-50%) !important;
}
[data-theme="dark"] .swiss-project-spacer-line,
.dark .swiss-project-spacer-line {
  background: rgba(148, 163, 184, 0.22) !important;
}
[data-theme="dark"] .swiss-convo-tabs-pill,
.dark .swiss-convo-tabs-pill {
  color: #94a3b8 !important;
}

/* Hover effects */
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
/* Pruned / Archived conversation tabs styling */
[data-swiss-pruned="true"] {
  opacity: 0.55 !important;
  cursor: not-allowed !important;
}
[data-swiss-pruned="true"] * {
  cursor: not-allowed !important;
}
[data-swiss-pruned="true"] button,
[data-swiss-pruned="true"] [role="button"],
[data-swiss-pruned="true"] [data-testid="conversation-kebab"],
[data-swiss-pruned="true"] [data-testid="conversation-pin-button"],
[data-swiss-pruned="true"] [data-testid="conversation-archive-button"] {
  cursor: pointer !important;
}
.swiss-pruned-badge {
  display: inline-block !important;
  font-size: 10px !important;
  font-weight: 600 !important;
  opacity: 0.65 !important;
  margin-left: 6px !important;
  padding: 1px 4px !important;
  border-radius: 4px !important;
  background: rgba(148, 163, 184, 0.2) !important;
  color: inherit !important;
  vertical-align: middle !important;
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
	projectColors := make(map[string]string)
	if cfg != nil && cfg.ProjectColors != nil {
		projectColors = cfg.ProjectColors
	}
	colorsJSON, _ := json.Marshal(projectColors)

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
  const configuredColors = %s;
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

  // Intercept clicks on pruned/archived conversation tabs during capture phase to prevent infinite loading spinner
  if (!window.__swissPrunedClickBound) {
    window.__swissPrunedClickBound = true;
    document.addEventListener("click", (e) => {
      const prunedEl = e.target.closest('[data-swiss-pruned="true"]');
      if (prunedEl) {
        e.stopImmediatePropagation();
        e.preventDefault();
        if (typeof window.__swissToast === "function") {
          window.__swissToast("Conversation database was pruned or archived (file not found)");
        }
      }
    }, true);
  }

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
      if (fiber && fiber.alternate) {
        let root = fiber;
        while (root.return) root = root.return;
        if (root.stateNode && root.stateNode.current && root.stateNode.current !== root) {
          fiber = fiber.alternate;
        }
      }
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

      // Check for pruned/archived conversation DB files
      let fsModule = null;
      let convsDir = null;
      try {
        if (typeof require !== "undefined") {
          const fs = require("fs");
          const path = require("path");
          const os = require("os");
          const cfgDir = (typeof process !== "undefined" && process.env && process.env.ANTIGRAVITY_CONFIG_DIR) || "";
          convsDir = cfgDir ? path.join(cfgDir, "conversations") : path.join(os.homedir(), ".gemini", "antigravity", "conversations");
          fsModule = fs;
        }
      } catch (_) {}

      window.__swissPrunedConversations = window.__swissPrunedConversations || new Set();

      if (!window.__swissPrunedFetched && window.fetch) {
        window.__swissPrunedFetched = true;
        fetch("http://127.0.0.1:8765/api/gui/conversations/pruned")
          .then(r => r.json())
          .then(list => {
            if (Array.isArray(list)) {
              list.forEach(id => window.__swissPrunedConversations.add(id));
              if (typeof window.__swissUpdateTagsAndDraggables === "function") {
                requestAnimationFrame(window.__swissUpdateTagsAndDraggables);
              }
            }
          })
          .catch(() => {});
      }

      items.forEach(it => {
        if (it.type === "row") {
          const cid = (it.conversationId || it.id || "").replace(/^conversation-/, "");
          if (cid && fsModule && convsDir) {
            try {
              if (!fsModule.existsSync(convsDir + "/" + cid + ".db")) {
                window.__swissPrunedConversations.add(cid);
              }
            } catch (_) {}
          }
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
          } else if (it.type === "empty-placeholder") {
            const gid = it.groupId || (it.id || "").replace("empty-placeholder-", "");
            const isArchived = activeArchived.includes(headerMap[gid]) || activeArchived.includes(gid);
            if (isArchived) hiddenIndices.add(idx);
          }
        });
      }

      // Group items by project to filter conversation tabs according to tabsMode, tabsFixedLimit, and tabsAgeThreshold
      const projects = [];
      let curProj = null;
      items.forEach((it, idx) => {
        if (it.type === "header") {
          curProj = {
            headerIdx: idx,
            groupId: it.id.replace("header-", ""),
            label: it.label,
            rows: [],
            hasShowMore: false,
            showMoreIdx: -1,
            showMoreItem: null,
            emptyPlaceholderIdx: -1
          };
          projects.push(curProj);
        } else if (it.type === "section-header" || it.type === "spacer") {
          curProj = null;
        } else if (it.type === "row" && curProj && (!it.groupId || it.groupId === curProj.groupId)) {
          curProj.rows.push(idx);
        } else if (it.type === "show-more" && curProj && (!it.groupId || it.groupId === curProj.groupId)) {
          curProj.hasShowMore = true;
          curProj.showMoreIdx = idx;
          curProj.showMoreItem = it;
        } else if (it.type === "empty-placeholder" && curProj && (!it.groupId || it.groupId === curProj.groupId)) {
          curProj.emptyPlaceholderIdx = idx;
        }
      });

      window.__swissManuallyExpandedProjects = window.__swissManuallyExpandedProjects || new Set();

      // Retrieve Redux summaries if available for dynamic tabs recency
      let summaries = null;
      if (tabsMode === "dynamic") {
        try {
          const rootEl = document.getElementById("root");
          if (rootEl) {
            const rKey = Object.keys(rootEl).find(k => k.startsWith("__reactFiber") || k.startsWith("__reactContainer"));
            let curr = rootEl[rKey];
            while (curr && !summaries) {
              if (curr.memoizedProps?.store) {
                summaries = curr.memoizedProps.store.getState()?.trajectorySummaries?.summaries;
                break;
              }
              curr = curr.child;
            }
          }
        } catch (_) {}
      }

      // Parse tabsAgeThreshold into cutoff ms
      let ageCutoffMs = 0;
      if (tabsMode === "dynamic") {
        let ageDays = 14;
        const match = String(tabsAgeThreshold || "14d").match(/^(\d+)([dhm]?)$/);
        if (match) {
          const val = parseInt(match[1], 10);
          const unit = match[2];
          ageDays = unit === "h" ? val / 24 : unit === "m" ? val / (24 * 60) : val;
        }
        ageCutoffMs = Date.now() - (ageDays * 24 * 60 * 60 * 1000);
      }

      projects.forEach(p => {
        if (hiddenIndices.has(p.headerIdx)) return;

        // Calculate targetLimit for visible tabs
        let targetLimit = tabsFixedLimit > 0 ? tabsFixedLimit : 6;
        if (tabsMode === "dynamic") {
          let recentCount = 0;
          p.rows.forEach(rIdx => {
            const it = items[rIdx];
            if (!it) return;
            const cid = (it.conversationId || it.id || "").replace(/^conversation-/, "");
            const sum = summaries ? summaries[cid] : null;
            let ts = sum ? (sum.lastModifiedTime || sum.last_modified_time || sum.createdTime) : null;
            if (!ts && it.lastModifiedTime) ts = it.lastModifiedTime;
            if (!ts && it.data && it.data.lastModifiedTime) ts = it.data.lastModifiedTime;
            if (ts && new Date(ts).getTime() >= ageCutoffMs) {
              recentCount++;
            }
          });
          const minTabs = tabsMin > 0 ? tabsMin : 3;
          const maxTabs = tabsMax > 0 ? tabsMax : 6;
          targetLimit = Math.max(minTabs, Math.min(maxTabs, recentCount > 0 ? recentCount : minTabs));
        }

        p.targetLimit = targetLimit;

        // Break out of Antigravity's default collapsed truncation (e.g. only 3 tabs shown + "See all")
        // if user configured more tabs than currently visible in collapsed state
        if (p.hasShowMore && p.rows.length < targetLimit) {
          const isSeeAll = (p.showMoreItem?.label || "").toLowerCase().includes("see all");
          if (isSeeAll) {
            const showMoreBtn = container.querySelector('[data-index="' + p.showMoreIdx + '"] button');
            if (showMoreBtn && !showMoreBtn.__swissAutoExpanded) {
              showMoreBtn.__swissAutoExpanded = true;
              showMoreBtn.click();
            }
          }
        }

        // Hide rows exceeding target limit unless manually expanded by user
        const isManuallyExpanded = window.__swissManuallyExpandedProjects.has(p.groupId);
        if (p.hasShowMore && !isManuallyExpanded && p.rows.length > targetLimit) {
          for (let i = targetLimit; i < p.rows.length; i++) {
            hiddenIndices.add(p.rows[i]);
          }
        }
      });

      // Clean up any obsolete custom divider DOM elements
      container.querySelectorAll(".swiss-custom-divider").forEach(el => el.remove());

      // Compute uncontracted project spacer indices
      const spacerIndices = new Set();
      if (consistentProjectSpacing) {
        projects.forEach(p => {
          if (hiddenIndices.has(p.headerIdx)) return;
          if (!p.hasShowMore || p.showMoreIdx === undefined || p.showMoreIdx === -1 || hiddenIndices.has(p.showMoreIdx)) {
            const visibleRows = p.rows.filter(rIdx => !hiddenIndices.has(rIdx));
            if (visibleRows.length > 0) {
              spacerIndices.add(visibleRows[visibleRows.length - 1]);
            } else if (p.emptyPlaceholderIdx !== undefined && p.emptyPlaceholderIdx !== -1 && !hiddenIndices.has(p.emptyPlaceholderIdx)) {
              spacerIndices.add(p.emptyPlaceholderIdx);
            } else {
              spacerIndices.add(p.headerIdx);
            }
          } else {
            spacerIndices.add(p.showMoreIdx);
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
              const nextItem = el.nextElementSibling;
              const pInner = el.querySelector('[data-testid="conversation-row-sidebar"]') || el.firstElementChild;
              const nextInner = nextItem ? (nextItem.querySelector('[data-project-card]') || nextItem.firstElementChild) : null;
              if (pInner && nextInner) {
                const pRect = pInner.getBoundingClientRect();
                const nRect = nextInner.getBoundingClientRect();
                const spRect = spacer.getBoundingClientRect();
                const gapCenter = (pRect.bottom + nRect.top) / 2;
                line.style.setProperty("top", (gapCenter - spRect.top) + "px", "important");
              }
            } else if (line) {
              line.remove();
            }
          } else if (spacer) {
            spacer.remove();
          }

          // Project conversation row color tinting and pruned status
          if (item.type === "row") {
            const cid = (item.conversationId || item.id || "").replace(/^conversation-/, "");
            const isPruned = cid && window.__swissPrunedConversations && window.__swissPrunedConversations.has(cid);
            const row = el.matches('[data-testid="conversation-row-sidebar"]')
              ? el
              : el.querySelector('[data-testid="conversation-row-sidebar"]');

            if (item.groupId) {
              const pName = headerMap[item.groupId];
              if (pName) {
                if (isColorEnabled) {
                  el.setAttribute("data-swiss-project", pName);
                  if (row) row.setAttribute("data-swiss-project", pName);
                } else {
                  el.removeAttribute("data-swiss-project");
                  if (row) row.removeAttribute("data-swiss-project");
                }
              }
            }

            if (isPruned) {
              el.setAttribute("data-swiss-pruned", "true");
              if (row) row.setAttribute("data-swiss-pruned", "true");
              el.setAttribute("title", "Conversation database was pruned or archived (file not found)");
              if (row) row.setAttribute("title", "Conversation database was pruned or archived (file not found)");

              let badge = (row || el).querySelector(".swiss-pruned-badge");
              if (!badge) {
                badge = document.createElement("span");
                badge.className = "swiss-pruned-badge";
                badge.textContent = " [Archived]";
                const titleEl = (row || el).querySelector("span.truncate, span") || (row || el);
                if (titleEl && titleEl !== (row || el)) {
                  titleEl.appendChild(badge);
                } else {
                  (row || el).appendChild(badge);
                }
              }
            } else {
              el.removeAttribute("data-swiss-pruned");
              if (row) row.removeAttribute("data-swiss-pruned");
              const badge = (row || el).querySelector(".swiss-pruned-badge");
              if (badge) badge.remove();
            }
          }

          // Project show-more button handling: sleek divider with centered solid triangle
          if (item.type === "show-more" || (el.querySelector("button") && (el.querySelector("button").textContent.includes("See all") || el.querySelector("button").textContent.includes("See less") || el.querySelector("button").getAttribute("data-swiss-divider") === "true"))) {
            const btn = el.querySelector("button");
            if (btn) {
              const projForShowMore = projects.find(p => p.showMoreIdx === idx || (item.groupId && p.groupId === item.groupId));
              const gid = projForShowMore ? projForShowMore.groupId : (item.groupId || "");
              if (gid) {
                btn.setAttribute("data-swiss-group-id", gid);
              }
              const isManuallyExpanded = gid && window.__swissManuallyExpandedProjects.has(gid);
              const hasExcessRows = projForShowMore && projForShowMore.rows.length > projForShowMore.targetLimit;
              btn.setAttribute("data-swiss-can-expand", hasExcessRows ? "true" : "false");

              if (replaceSeeAllTriangle) {
                let labelText = (item && item.label) || btn.getAttribute("data-swiss-orig-label") || btn.textContent.trim();
                if (labelText === "▾" || labelText === "▴" || !labelText) {
                  const origAttr = btn.getAttribute("data-swiss-orig-label");
                  labelText = (origAttr && origAttr !== "▾" && origAttr !== "▴") ? origAttr : "";
                }
                const isNativeSeeAll = labelText.toLowerCase().includes("see all");
                const isNativeSeeLess = labelText.toLowerCase().includes("see less");

                let isExpanded = false;
                if (projForShowMore) {
                  const hasHiddenRows = projForShowMore.rows.some(rIdx => hiddenIndices.has(rIdx));
                  if (hasHiddenRows) {
                    isExpanded = false;
                  } else if (isManuallyExpanded) {
                    isExpanded = true;
                  } else if (isNativeSeeLess) {
                    isExpanded = true;
                  } else {
                    isExpanded = false;
                  }
                } else {
                  isExpanded = isNativeSeeLess;
                }

                const symbol = isExpanded ? "▴" : "▾";
                let titleText = isExpanded ? "Show fewer tabs" : "Show more tabs";
                if (!isExpanded && isNativeSeeAll && labelText.includes("(") && labelText.includes(")")) {
                  const m = labelText.match(/\((\d+)\)/);
                  if (m) {
                    titleText = "Show more tabs (" + m[1] + ")";
                  }
                }
                btn.setAttribute("data-swiss-divider", "true");
                if (labelText && labelText !== "▾" && labelText !== "▴") {
                  btn.setAttribute("data-swiss-orig-label", labelText);
                }
                btn.setAttribute("title", titleText);
                btn.style.setProperty("display", "flex", "important");
                btn.style.setProperty("justify-content", "center", "important");
                btn.style.setProperty("align-items", "center", "important");
                btn.style.setProperty("width", "100%%", "important");
                btn.style.setProperty("height", "10px", "important");
                btn.style.setProperty("background", "transparent", "important");
                btn.style.setProperty("overflow", "visible", "important");
                btn.style.setProperty("padding", "0", "important");
                btn.style.setProperty("margin", "0 auto", "important");

                if (el) {
                  el.style.setProperty("display", "flex", "important");
                  el.style.setProperty("flex-direction", "column", "important");
                  el.style.setProperty("align-items", "center", "important");
                  el.style.setProperty("justify-content", "flex-start", "important");
                  el.style.setProperty("background", "transparent", "important");
                  el.style.setProperty("overflow", "visible", "important");
                  el.style.setProperty("z-index", "2", "important");
                }
                const btnParent = btn.parentElement;
                if (btnParent && btnParent !== el) {
                  btnParent.style.setProperty("display", "flex", "important");
                  btnParent.style.setProperty("justify-content", "center", "important");
                  btnParent.style.setProperty("align-items", "center", "important");
                  btnParent.style.setProperty("width", "100%%", "important");
                  btnParent.style.setProperty("height", "10px", "important");
                  btnParent.style.setProperty("background", "transparent", "important");
                  btnParent.style.setProperty("overflow", "visible", "important");
                  btnParent.style.setProperty("padding", "0", "important");
                  btnParent.style.setProperty("margin", "0 auto", "important");
                }

                const curTriangle = btn.querySelector(".swiss-convo-tabs-triangle");
                if (!curTriangle || curTriangle.textContent !== symbol) {
                  btn.innerHTML = '<div class="swiss-convo-tabs-divider">' +
                    '<div class="swiss-convo-tabs-pill"><span class="swiss-convo-tabs-triangle">' + symbol + '</span></div>' +
                  '</div>';
                }
              } else if (btn.getAttribute("data-swiss-divider") === "true") {
                btn.removeAttribute("data-swiss-divider");
                const orig = btn.getAttribute("data-swiss-orig-label") || item.label || btn.getAttribute("title") || "See all";
                btn.textContent = orig;
              }

              if (!btn.__swissToggleBound) {
                btn.__swissToggleBound = true;
                btn.addEventListener("click", (e) => {
                  const curGid = btn.getAttribute("data-swiss-group-id");
                  if (!curGid) return;
                  if (window.__swissManuallyExpandedProjects.has(curGid)) {
                    e.preventDefault();
                    e.stopImmediatePropagation();
                    window.__swissManuallyExpandedProjects.delete(curGid);
                    if (typeof window.__swissUpdateTagsAndDraggables === "function") {
                      window.__swissUpdateTagsAndDraggables();
                    }
                  } else if (btn.getAttribute("data-swiss-can-expand") === "true") {
                    e.preventDefault();
                    e.stopImmediatePropagation();
                    window.__swissManuallyExpandedProjects.add(curGid);
                    if (typeof window.__swissUpdateTagsAndDraggables === "function") {
                      window.__swissUpdateTagsAndDraggables();
                    }
                  } else {
                    window.__swissManuallyExpandedProjects.add(curGid);
                  }
                }, true);
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

  // 3. Project Options Menu Enhancement (New Conversation, Archive, Set Color with 7 Presets + 10x10 Custom Palette)
  const GREYSCALE = ['#ffffff', '#f4f4f5', '#e4e4e7', '#cbd5e1', '#94a3b8', '#64748b', '#475569', '#334155', '#1e293b', '#09090b'];
  const CHROMATIC_HUES = [0, 40, 80, 120, 160, 200, 240, 280, 320];
  const LIGHTNESSES = [92, 84, 76, 68, 60, 52, 44, 36, 28, 20];
  const GRID_COLORS = [];
  for (let r = 0; r < 10; r++) {
    const l = LIGHTNESSES[r];
    for (let c = 0; c < 10; c++) {
      if (c === 0) {
        GRID_COLORS.push(GREYSCALE[r]);
      } else {
        GRID_COLORS.push("hsl(" + CHROMATIC_HUES[c - 1] + ", 82%%, " + l + "%%)");
      }
    }
  }

  const PRESET_COLORS = [
    { name: "Slate Black", hex: "#0f172a" },
    { name: "Slate Grey", hex: "#64748b" },
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
  function persistProjectColorsToDisk(colors) {
    try {
      if (typeof require !== "undefined") {
        const fs = require("fs");
        const path = require("path");
        const os = require("os");
        let cfgDir = (typeof process !== "undefined" && process.env && process.env.ANTIGRAVITY_SWISS_CONFIG_DIR) || "";
        if (!cfgDir) {
          const platform = (typeof process !== "undefined" && process.platform) || "";
          const home = (os && typeof os.homedir === "function") ? os.homedir() : "";
          if (platform === "win32") {
            const appData = (typeof process !== "undefined" && process.env && process.env.APPDATA) || "";
            cfgDir = appData ? path.join(appData, "antigravity-swiss") : path.join(home, "AppData", "Roaming", "antigravity-swiss");
          } else if (platform === "darwin") {
            cfgDir = path.join(home, "Library", "Application Support", "antigravity-swiss");
          } else {
            const xdg = (typeof process !== "undefined" && process.env && process.env.XDG_CONFIG_HOME) || "";
            cfgDir = xdg ? path.join(xdg, "antigravity-swiss") : path.join(home, ".config", "antigravity-swiss");
          }
        }
        if (!fs.existsSync(cfgDir)) {
          fs.mkdirSync(cfgDir, { recursive: true });
        }
        const configPath = path.join(cfgDir, "gui_improvements.json");
        let data = {};
        if (fs.existsSync(configPath)) {
          try {
            data = JSON.parse(fs.readFileSync(configPath, "utf8")) || {};
          } catch (_) {}
        }
        data.project_colors = colors;
        fs.writeFileSync(configPath, JSON.stringify(data, null, 2), "utf8");
      }
    } catch (e) {
      console.warn("[SwissKnife] Failed to persist project colors to disk:", e);
    }
  }

  window.__swissDynamicColors = window.__swissDynamicColors || {};
  if (configuredColors && typeof configuredColors === "object") {
    Object.assign(window.__swissDynamicColors, configuredColors);
  }
  if (typeof localStorage !== "undefined") {
    try {
      const cached = localStorage.getItem("antigravity_swiss_project_colors");
      if (cached) {
        const parsed = JSON.parse(cached);
        if (parsed && typeof parsed === "object") {
          Object.assign(window.__swissDynamicColors, parsed);
        }
      } else {
        localStorage.setItem("antigravity_swiss_project_colors", JSON.stringify(window.__swissDynamicColors));
      }
      const delCached = localStorage.getItem("antigravity_swiss_deleted_colors");
      if (delCached) {
        const delList = JSON.parse(delCached);
        if (Array.isArray(delList)) {
          delList.forEach(p => { delete window.__swissDynamicColors[p]; });
        }
      }
    } catch (_) {}
  }

  function renderDynamicProjectStyles(projName, colorHex) {
    try {
      if (colorHex) {
        window.__swissDynamicColors[projName] = colorHex;
        if (typeof localStorage !== "undefined") {
          try {
            let delList = JSON.parse(localStorage.getItem("antigravity_swiss_deleted_colors") || "[]");
            if (Array.isArray(delList)) {
              delList = delList.filter(p => p !== projName);
              localStorage.setItem("antigravity_swiss_deleted_colors", JSON.stringify(delList));
            }
          } catch (_) {}
        }
      } else if (projName) {
        delete window.__swissDynamicColors[projName];
        if (typeof localStorage !== "undefined") {
          try {
            let delList = JSON.parse(localStorage.getItem("antigravity_swiss_deleted_colors") || "[]");
            if (!Array.isArray(delList)) delList = [];
            if (!delList.includes(projName)) delList.push(projName);
            localStorage.setItem("antigravity_swiss_deleted_colors", JSON.stringify(delList));
          } catch (_) {}
        }
      }

      if (typeof localStorage !== "undefined") {
        try {
          localStorage.setItem("antigravity_swiss_project_colors", JSON.stringify(window.__swissDynamicColors));
        } catch (_) {}
      }
      persistProjectColorsToDisk(window.__swissDynamicColors);

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
            const pr = parseInt(rawHex[0] + rawHex[0], 16);
            const pg = parseInt(rawHex[1] + rawHex[1], 16);
            const pb = parseInt(rawHex[2] + rawHex[2], 16);
            if (!isNaN(pr) && !isNaN(pg) && !isNaN(pb)) {
              r = pr; g = pg; b = pb;
            }
          } else if (rawHex.length >= 6) {
            const pr = parseInt(rawHex.slice(0, 2), 16);
            const pg = parseInt(rawHex.slice(2, 4), 16);
            const pb = parseInt(rawHex.slice(4, 6), 16);
            if (!isNaN(pr) && !isNaN(pg) && !isNaN(pb)) {
              r = pr; g = pg; b = pb;
            }
          }
        }

        const baseOpacity = 0.15;
        const lum = (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255.0;
        const lightMult = lum > 0.6 ? Math.max(0.2, 1.0 - (lum - 0.6) * 0.75) : 1.0;
        const effOpacity = (baseOpacity * lightMult).toFixed(2);
        const effHoverOpacity = Math.min(1.0, (baseOpacity + 0.08) * lightMult).toFixed(2);
        const effSelectedOpacity = Math.min(1.0, (baseOpacity + 0.16) * lightMult).toFixed(2);
        const textColor = lum > 0.6 ? '#0f172a' : '#ffffff';

        cssRules.push(
          '/* Dynamic Project Label Card */' +
          '[data-swiss-project="' + safeP + '"][data-project-card="true"],' +
          '[data-swiss-project="' + safeP + '"] [data-project-card="true"],' +
          '[data-swiss-project="' + safeP + '"][data-project-card],' +
          '[data-swiss-project="' + safeP + '"] [data-project-card],' +
          '[data-project-card][data-swiss-project="' + safeP + '"] {' +
          '  background-color: ' + hex + ' !important;' +
          '  color: ' + textColor + ' !important;' +
          '  border-radius: 8px !important;' +
          '  border: none !important;' +
          '}' +
          '[data-swiss-project="' + safeP + '"][data-project-card="true"] *,' +
          '[data-swiss-project="' + safeP + '"] [data-project-card="true"] *,' +
          '[data-swiss-project="' + safeP + '"][data-project-card] *,' +
          '[data-swiss-project="' + safeP + '"] [data-project-card] *,' +
          '[data-project-card][data-swiss-project="' + safeP + '"] * {' +
          '  color: ' + textColor + ' !important;' +
          '}' +
          '[data-swiss-project="' + safeP + '"] [class*="group/header"] button,' +
          '[data-swiss-project="' + safeP + '"] [class*="group/header"] a,' +
          '[data-swiss-project="' + safeP + '"] a[aria-label*="conversation" i] svg,' +
          '[data-swiss-project="' + safeP + '"] button[aria-label="Project options"] svg,' +
          '[data-swiss-project="' + safeP + '"] button[aria-label*="conversation"] svg {' +
          '  color: ' + textColor + ' !important;' +
          '  fill: ' + textColor + ' !important;' +
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
  renderDynamicProjectStyles();

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

      const currentVer = "v3_palette_10x10";
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
        let matchedLink = null;
        if (!gid) {
          const links = document.querySelectorAll('a[aria-label="New Conversation in Project"]');
          for (const l of links) {
            const h = l.closest('.group\\/header, [data-project-card], [data-index], [data-swiss-project]');
            if (h && (h.textContent || '').trim().toLowerCase().includes(projectName.toLowerCase())) {
              matchedLink = l;
              const m = (l.getAttribute('href') || '').match(/section=([a-zA-Z0-9_-]+)/);
              if (m) { gid = m[1]; break; }
            }
          }
        }
        if (matchedLink) {
          matchedLink.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true }));
        } else if (gid) {
          try {
            window.history.pushState({}, "", "/?section=" + gid);
            window.dispatchEvent(new PopStateEvent("popstate"));
          } catch (_) {}
          if (window.location.search !== "?section=" + gid) {
            window.location.href = "/?section=" + gid;
          }
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
            return '<div class="swiss-preset-swatch swiss-menu-swatch" data-color="' + p.hex + '" title="' + p.name + '" style="width: 15px; height: 15px; border-radius: 50%%; background: ' + p.hex + '; cursor: pointer; transition: transform 0.12s; box-shadow: inset 0 0 0 1px rgba(128, 128, 128, 0.25); flex-shrink: 0; box-sizing: border-box;"></div>';
          }).join("") +
          '<div id="swiss-custom-trigger" class="swiss-menu-custom-trigger" title="Custom 10x10 Palette" style="width: 15px; height: 15px; border-radius: 50%%; background: conic-gradient(red, yellow, lime, aqua, blue, magenta, red); cursor: pointer; transition: transform 0.12s, box-shadow 0.12s; flex-shrink: 0; box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.12); box-sizing: border-box;"></div>' +
        '</div>' +
        '<div id="swiss-custom-grid-container" class="swiss-menu-custom-grid" style="display: none; padding: 2px 6px 4px 6px;">' +
          '<div style="display: grid; grid-template-columns: repeat(10, 16px); gap: 3px; justify-content: center; background: var(--muted, rgba(0, 0, 0, 0.03)); padding: 6px; border-radius: 6px; border: 1px solid var(--border, rgba(0, 0, 0, 0.075));">' +
            GRID_COLORS.map(function(c) {
              return '<div class="swiss-grid-cell" data-color="' + c + '" style="width: 16px; height: 16px; border-radius: 50%%; background: ' + c + '; cursor: pointer; transition: transform 0.12s; box-shadow: inset 0 0 0 1px rgba(128, 128, 128, 0.25); box-sizing: border-box;"></div>';
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
  document.querySelectorAll('[role="menu"]').forEach(m => { delete m.__swissMenuVersion; });

  function normalizeNewConversationButton() {
    try {
      const btn = document.querySelector('[data-testid="new-conversation-button"]');
      if (!btn) return;
      const isConv = Boolean(
        (window.location.pathname && window.location.pathname.startsWith('/c/')) ||
        document.querySelector('[data-testid="conversation-view"]') ||
        document.querySelector('[data-testid="conversation-row-sidebar"][data-selected="true"]')
      );
      const isHistory = Boolean(
        (window.location.pathname && window.location.pathname.startsWith('/history')) ||
        document.querySelector('[data-testid="history-button"].active') ||
        document.querySelector('[data-testid="history-button"][data-selected="true"]')
      );
      const isStage = Boolean(
        document.body.getAttribute('data-swiss-stage-active') ||
        document.querySelector('.swiss-left-nav-tab.active')
      );
      const isUnselected = isConv || isHistory || isStage || (window.location.pathname && window.location.pathname !== '/');
      if (isUnselected) {
        btn.setAttribute('data-selected', 'false');
        btn.setAttribute('aria-selected', 'false');
        btn.removeAttribute('data-state');
        btn.classList.remove('active');
        btn.classList.add('unselected');
      } else {
        btn.setAttribute('data-selected', 'true');
        btn.setAttribute('aria-selected', 'true');
        btn.classList.remove('unselected');
      }
    } catch (_) {}
  }
  if (!window.normalizeNewConversationButton) {
    window.normalizeNewConversationButton = normalizeNewConversationButton;
  }
  normalizeNewConversationButton();

  let isUpdatingSwiss = false;
  let pendingSwissUpdate = false;
  let scheduledRafSwiss = null;

  function triggerSwissUpdate() {
    if (isUpdatingSwiss) {
      pendingSwissUpdate = true;
      return;
    }
    if (scheduledRafSwiss) return;
    scheduledRafSwiss = requestAnimationFrame(() => {
      scheduledRafSwiss = null;
      if (isUpdatingSwiss) {
        pendingSwissUpdate = true;
        return;
      }
      isUpdatingSwiss = true;
      pendingSwissUpdate = false;
      try {
        if (typeof window.__swissUpdateTagsAndDraggables === "function") {
          window.__swissUpdateTagsAndDraggables();
        }
        if (typeof window.normalizeNewConversationButton === "function") {
          window.normalizeNewConversationButton();
        }
      } finally {
        setTimeout(() => {
          isUpdatingSwiss = false;
          if (pendingSwissUpdate) {
            pendingSwissUpdate = false;
            triggerSwissUpdate();
          }
        }, 16);
      }
    });
  }

  try {
    if (typeof updateTagsAndDraggables === "function") {
      updateTagsAndDraggables();
    }
  } catch (_) {}
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
          t.classList?.contains("swiss-pruned-badge") ||
          t.classList?.contains("swiss-convo-tabs-divider") || 
          t.classList?.contains("swiss-convo-tabs-pill") ||
          t.classList?.contains("swiss-project-bottom-spacer") ||
          t.classList?.contains("swiss-project-spacer-line") ||
          t.closest?.("[id^='swiss-'], [class*='swiss-']")) {
        continue;
      }
      if (m.type === "childList" && (m.addedNodes.length > 0 || m.removedNodes.length > 0)) {
        const allSwiss = Array.from(m.addedNodes).concat(Array.from(m.removedNodes)).every(n =>
          n.nodeType === 1 && (
            n.classList?.contains("swiss-project-bottom-spacer") ||
            n.classList?.contains("swiss-project-spacer-line") ||
            n.classList?.contains("swiss-pruned-badge") ||
            n.classList?.contains("swiss-convo-tabs-divider")
          )
        );
        if (allSwiss) continue;
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
    attributeFilter: ["data-selected", "data-index", "data-testid", "data-open", "role", "aria-expanded"]
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
})();`, string(cssJSON), enabled, colorStylingEnabled, dragRearrangeEnabled, string(orderJSON), string(archivedJSON), string(colorsJSON), tabsMode, tabsFixedLimit, tabsAgeThreshold, tabsMin, tabsMax, replaceSeeAllTriangle, consistentProjectSpacing, consistentProjectSpacingLine, isBorderMode, borderWidth, isLeftBarMode, fontWeight)

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
