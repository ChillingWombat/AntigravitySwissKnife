package enhancements

import (
	"time"
)

// PromptJumpBarConfig defines settings for the Devin-style conversation turn quick jump bar.
type PromptJumpBarConfig struct {
	Enabled     bool   `json:"enabled"`      // Master switch for the prompt jump bar
	ShowTooltip bool   `json:"show_tooltip"` // Show floating preview tooltip on hover
	FocusPulse  bool   `json:"focus_pulse"`  // Highlight target prompt card with brief pulse on jump
	SyncScroll  bool   `json:"sync_scroll"`  // Highlight active dash based on current scroll position
	Position    string `json:"position"`     // "gutter" (top-left conversation margin) or "floating"
	DashWidth   int    `json:"dash_width"`   // Default dash width in pixels (default: 14)
	ColorMode   string `json:"color_mode"`   // "default" (grey), "project" (match project color), "custom"
	CustomColor string `json:"custom_color"` // Hex color when ColorMode is "custom" (default: "#0b57d0")
}

// EnhancementsConfig holds configuration for usability improvements and add-on features.
type EnhancementsConfig struct {
	Version            string              `json:"version"`
	Enabled            bool                `json:"enabled"` // Master switch for all enhancements
	PromptJumpBar      PromptJumpBarConfig `json:"prompt_jump_bar"`
	ToolDensityMode    string              `json:"tool_density_mode"`    // "normal", "muted", "hidden"
	BreakerLineEnabled bool                `json:"breaker_line_enabled"` // Breaker line between previous answer and new prompt
	DefaultNewProject  string              `json:"default_new_project"`  // "auto" (default: latest active) or predefined project name
	ScrollToBottom     bool                `json:"scroll_to_bottom"`
	TurnCounter        bool                `json:"turn_counter"`
	UpdatedAt          time.Time           `json:"updated_at"`
}

// DefaultConfig returns the recommended default enhancements configuration.
func DefaultConfig() *EnhancementsConfig {
	return &EnhancementsConfig{
		Version: "1.0.0",
		Enabled: true,
		PromptJumpBar: PromptJumpBarConfig{
			Enabled:     true,
			ShowTooltip: true,
			FocusPulse:  true,
			SyncScroll:  true,
			Position:    "gutter",
			DashWidth:   14,
			ColorMode:   "default", // Default is grey as requested by the user
			CustomColor: "",        // Empty by default (falls back to slate grey)
		},
		ToolDensityMode:    "muted",
		BreakerLineEnabled: true,
		DefaultNewProject:  "auto",
		ScrollToBottom:     true,
		TurnCounter:        true,
		UpdatedAt:          time.Now().UTC(),
	}
}
