package revival

import "time"

// ActiveSessionInfo holds metadata about the currently or most recently active conversation and subagent.
type ActiveSessionInfo struct {
	ConversationID       string    `json:"conversation_id"`
	Title                string    `json:"title"`
	WorkspaceURI         string    `json:"workspace_uri"`
	LastModified         time.Time `json:"last_modified"`
	IsTopLevel           bool      `json:"is_top_level"`
	ParentConversationID string    `json:"parent_conversation_id"`
	NestingDepth         int       `json:"nesting_depth"`
	NotFullyIdle         bool      `json:"not_fully_idle"`
	Status               string    `json:"status"`
	NeedsRevival         bool      `json:"needs_revival"`
	ActiveSubagentID     string    `json:"active_subagent_id,omitempty"`
	TargetPath           string    `json:"target_path,omitempty"`
	TargetApp            string    `json:"target_app,omitempty"`
}

// IntentStatus defines the lifecycle status of a revival intent.
type IntentStatus string

const (
	IntentStatusPending   IntentStatus = "pending"
	IntentStatusReviving  IntentStatus = "reviving"
	IntentStatusRevived   IntentStatus = "revived"
	IntentStatusCancelled IntentStatus = "cancelled"
	IntentStatusFailed    IntentStatus = "failed"
)

// DefaultTriggerPrompt is the standard continuation prompt dispatched to the agent composer upon revival.
const DefaultTriggerPrompt = "Please continue ongoing tasks and subagents."

// RevivalIntent represents an intent to restore and continue a conversation after an account switch or restart.
type RevivalIntent struct {
	IntentID           string       `json:"intent_id"`
	TargetApp          string       `json:"target_app"`
	RootConversationID string       `json:"root_conversation_id"`
	ActiveSubagentID   string       `json:"active_subagent_id,omitempty"`
	TriggerPrompt      string       `json:"trigger_prompt"`
	CreatedAt          time.Time    `json:"created_at"`
	Status             IntentStatus `json:"status"`

	// Compatibility fields for desktop preload script (pending_continuation.json)
	CascadeID   string `json:"cascade_id,omitempty"`
	SectionID   string `json:"section_id,omitempty"`
	Prompt      string `json:"prompt,omitempty"`
	Timestamp   int64  `json:"timestamp,omitempty"`
	Attempts    int    `json:"attempts"`
	MaxAttempts int    `json:"max_attempts,omitempty"`
	TTLSeconds  int    `json:"ttl_seconds,omitempty"`
	Resumed     bool   `json:"resumed"`
}

// RevivalStatus represents the current status of conversation continuation and active revival intents.
type RevivalStatus struct {
	ActiveSession *ActiveSessionInfo `json:"active_session,omitempty"`
	PendingIntent *RevivalIntent     `json:"pending_intent,omitempty"`
	LastRevivedAt *time.Time         `json:"last_revived_at,omitempty"`
	Success       bool               `json:"success"`
}
