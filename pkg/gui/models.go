package gui

import (
	"fmt"
	"time"
)

// Config holds user customization settings for Antigravity desktop GUI improvements.
type Config struct {
	Enabled                     bool              `json:"enabled"`                       // Master toggle for GUI improvements
	ColorStylingEnabled         bool              `json:"color_styling_enabled"`        // Toggle: Custom project colors & conversation tinting
	SolidLeftEdge               bool              `json:"solid_left_edge"`              // Toggle: Include solid color edge on conversation tabs (default false)
	TintOpacity                 float64           `json:"tint_opacity"`                 // Conversation tab tint opacity (default 0.15)
	ActiveConversationIndicator string            `json:"active_conversation_indicator"`// "background" (accent fill), "border" (border outline), "left_bar" (left accent bar)
	ActiveConversationBorderWidth string          `json:"active_conversation_border_width,omitempty"` // "1px", "1.5px", "2px" (default), "3px"
	ActiveConversationBold      bool              `json:"active_conversation_bold"`     // Toggle: Bold text on open conversation tab (default false)
	ProjectColors               map[string]string `json:"project_colors"`               // Map of project name -> hex color
	DragRearrangeEnabled        bool              `json:"drag_rearrange_enabled"`       // Toggle: Drag to rearrange projects order
	ProjectOrder                []string          `json:"project_order"`                // Custom ordering of projects
	ArchivedProjects            []string          `json:"archived_projects"`            // Names or IDs of hidden/archived projects
	ConversationTabsMode        string            `json:"conversation_tabs_mode"`        // "dynamic" (default) or "fixed" (by chat age)
	ConversationTabsFixedLimit  int               `json:"conversation_tabs_fixed_limit"` // 1-10 (default 6)
	ConversationTabsAgeThreshold string           `json:"conversation_tabs_age_threshold"` // "1d", "3d", "7d", "14d" (default), "30d"
	ConversationTabsMin         int               `json:"conversation_tabs_min"`         // default 3 (range 1-10)
	ConversationTabsMax         int               `json:"conversation_tabs_max"`         // default 6 (range 1-10)
	ReplaceSeeAllTriangle       bool              `json:"replace_see_all_triangle"`      // Toggle: Replace "See all" and "See less" text buttons with triangle divider (default true)
	AutoArchiveConversations    bool              `json:"auto_archive_conversations"`    // Toggle: Automatically archive stale conversations
	AutoArchiveHorizon          string            `json:"auto_archive_horizon"`          // "3d", "7d", "14d" (default), "30d", "60d", "90d"
	AutoInject                  bool              `json:"auto_inject"`                  // Automatically inject into Antigravity desktop app
}

// DefaultConfig returns the default GUI improvement configuration.
func DefaultConfig() *Config {
	return &Config{
		Enabled:                     true,
		ColorStylingEnabled:         true,
		SolidLeftEdge:               false, // Legacy field preserved for backward compatibility
		TintOpacity:                 0.15,
		ActiveConversationIndicator: "background", // "background" (accent fill), "border" (outline), or "left_bar"
		ActiveConversationBorderWidth: "2px",
		ActiveConversationBold:      false,        // Regular text weight on open conversation tab by default
		ProjectColors: map[string]string{
			"Antigravity Swiss Knife": "#0b57d0", // Gemini blue
			"Arbitrager":              "#7c3aed", // Vibrant purple
			"Obsidian-HomePage":       "#059669", // Emerald green
			"David":                   "#d97706", // Warm amber
		},
		DragRearrangeEnabled: true,
		ProjectOrder: []string{
			"Antigravity Swiss Knife",
			"Arbitrager",
			"Obsidian-HomePage",
			"David",
		},
		ArchivedProjects:            []string{},
		ConversationTabsMode:        "dynamic",
		ConversationTabsFixedLimit:  6,
		ConversationTabsAgeThreshold: "14d",
		ConversationTabsMin:         3,
		ConversationTabsMax:         6,
		ReplaceSeeAllTriangle:       true,
		AutoArchiveConversations:    true,
		AutoArchiveHorizon:          "14d",
		AutoInject:                  true,
	}
}

// ProjectItem represents a detected or configured project in Antigravity.
type ProjectItem struct {
	Name         string `json:"name"`
	Color        string `json:"color"`
	IsConfigured bool   `json:"is_configured"`
	Source       string `json:"source"`
	OrderIndex   int    `json:"order_index"`
}

// ArchivedProjectItem represents an archived project with conversation activity metadata.
type ArchivedProjectItem struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Color              string    `json:"color"`
	ConversationCount  int       `json:"conversation_count"`
	LastActiveTime     time.Time `json:"last_active_time"`
	LastActiveRelative string    `json:"last_active_relative"`
	FolderURI          string    `json:"folder_uri,omitempty"`
	ArchivedAt         time.Time `json:"archived_at"`
}

// FormatRelativeTime formats time difference into automatically scaled units:
// (< 1 hr: minutes; 1-24 hr: hours; 24 hr - 30 d: days; 30 - 365 d: months; >= 365 d: years).
func FormatRelativeTime(t time.Time, now time.Time) string {
	if t.IsZero() {
		return "No conversations"
	}
	diff := now.Sub(t)
	if diff < 0 {
		return "Just now"
	}

	minutes := int64(diff.Minutes())
	hours := int64(diff.Hours())
	days := hours / 24

	if minutes < 1 {
		return "Just now"
	}
	if hours < 1 {
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	}
	if hours < 24 {
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	if days < 30 {
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
	months := days / 30
	if months < 12 {
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
	years := days / 365
	if years <= 1 {
		return "1 year ago"
	}
	return fmt.Sprintf("%d years ago", years)
}

// ApplyResult represents the outcome of applying styles to Antigravity.
type ApplyResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	PID     int    `json:"pid,omitempty"`
	Port    int    `json:"port,omitempty"`
}
