package enhancements

import (
	"time"
)

// PromptJumpBarConfig defines settings for the Devin-style conversation turn quick jump bar.
type PromptJumpBarConfig struct {
	Enabled           bool    `json:"enabled"`            // Master switch for the prompt jump bar
	ShowTooltip       bool    `json:"show_tooltip"`       // Show floating preview tooltip on hover
	FocusPulse        bool    `json:"focus_pulse"`        // Highlight target prompt card with brief pulse on jump
	SyncScroll        bool    `json:"sync_scroll"`        // Highlight active dash based on current scroll position
	Position          string  `json:"position"`           // "gutter" (top-left conversation margin) or "floating"
	DashWidth         int     `json:"dash_width"`         // Default dash width in pixels (default: 14)
	DashThickness     float64 `json:"dash_thickness"`     // Active dash thickness in pixels (default: 2.5)
	InactiveThickness float64 `json:"inactive_thickness"` // Inactive dash thickness in pixels (default: 1.5)
	ColorMode         string  `json:"color_mode"`         // "default" (grey), "project" (match project color), "custom"
	CustomColor       string  `json:"custom_color"`       // Hex color when ColorMode is "custom" (default: "#0b57d0")
}

// OverviewPanelConfig defines visual section dividing and compact element rules for Antigravity's Overview Panel.
type OverviewPanelConfig struct {
	Enabled                bool   `json:"enabled"`                  // Master switch for Overview Panel dividing enhancements
	DivisionStyle          string `json:"division_style"`           // "divider_line" (horizontal lines) or "border_zone" (card/zone boxes with whiter background)
	ZoneBorderRadius       int    `json:"zone_border_radius"`       // Border radius of each section zone card in px (default: 8)
	ZoneBorderColor        string `json:"zone_border_color"`        // Border color for each section zone card (default: "#e2e8f0")
	ZoneBackgroundContrast string `json:"zone_background_contrast"` // "whiter" (whiter background like chat input box), "subtle", "card" (default: "whiter")
	ZonePadding            int    `json:"zone_padding"`             // Internal padding for section zone in px (default: 10)
	ZoneGap                int    `json:"zone_gap"`                 // Vertical gap between section zones in px (default: 10)
	ReplaceSeeAllTriangle  bool   `json:"replace_see_all_triangle"` // Replace "See all (N)" and "See less" with a compact refined triangle divider (default: true)
	AuxTabsFormat          string `json:"aux_tabs_format"`          // "icon" (default: compact icon-only matching native) or "icon_and_name"
}

// EnhancementsConfig holds configuration for usability improvements and add-on features.
type EnhancementsConfig struct {
	Version            string              `json:"version"`
	Enabled            bool                `json:"enabled"` // Master switch for all enhancements
	PromptJumpBar      PromptJumpBarConfig `json:"prompt_jump_bar"`
	OverviewPanel      OverviewPanelConfig `json:"overview_panel"`       // Section division and compact controls for Overview Panel
	ToolDensityMode    string              `json:"tool_density_mode"`    // "normal", "muted", "hidden"
	BreakerLineEnabled         bool                `json:"breaker_line_enabled"`          // Breaker line between previous answer and new prompt
	LeftPanelExtensionsEnabled bool                `json:"left_panel_extensions_enabled"` // Toggle button for extension in left sidebar (default: true)
	LeftPanelExtensionsMode    string              `json:"left_panel_extensions_mode"`    // "single" (single Swiss Knife button) or "individual" (default: "single")
	MainSectionExtensionsEnabled bool              `json:"main_section_extensions_enabled"` // Toggle to use extension in main section vs auxiliary panel (default: true)
	DefaultNewProject          string              `json:"default_new_project"`           // "auto" (default: latest active) or predefined project name
	ScrollToBottom             bool                `json:"scroll_to_bottom"`
	TurnCounter                bool                `json:"turn_counter"`
	UpdatedAt                  time.Time           `json:"updated_at"`
}

// DefaultConfig returns the recommended default enhancements configuration.
func DefaultConfig() *EnhancementsConfig {
	return &EnhancementsConfig{
		Version: "1.0.0",
		Enabled: true,
		PromptJumpBar: PromptJumpBarConfig{
			Enabled:           true,
			ShowTooltip:       true,
			FocusPulse:        true,
			SyncScroll:        true,
			Position:          "gutter",
			DashWidth:         14,
			DashThickness:     2.5,
			InactiveThickness: 1.5,
			ColorMode:         "default", // Default is grey as requested by the user
			CustomColor:       "",        // Empty by default (falls back to slate grey)
		},
		OverviewPanel: OverviewPanelConfig{
			Enabled:                true,
			DivisionStyle:          "divider_line", // Divider line between sections
			ZoneBorderRadius:       8,
			ZoneBorderColor:        "#e2e8f0",
			ZoneBackgroundContrast: "whiter",
			ZonePadding:            10,
			ZoneGap:                10,
			ReplaceSeeAllTriangle:  true,
			AuxTabsFormat:          "icon",
		},
		ToolDensityMode:            "muted",
		BreakerLineEnabled:         true,
		LeftPanelExtensionsEnabled: true,
		LeftPanelExtensionsMode:    "single",
		MainSectionExtensionsEnabled: true,
		DefaultNewProject:          "auto",
		ScrollToBottom:             true,
		TurnCounter:                true,
		UpdatedAt:                  time.Now().UTC(),
	}
}
