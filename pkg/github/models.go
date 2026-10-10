package github

import "time"

// RepoInfo stores detected repository metadata.
type RepoInfo struct {
	Owner         string `json:"owner"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	CurrentBranch string `json:"current_branch"`
	RemoteURL     string `json:"remote_url"`
	WorkspacePath string `json:"workspace_path"`
}

// Issue represents a GitHub Issue.
type Issue struct {
	Number        int               `json:"number"`
	Title         string            `json:"title"`
	Body          string            `json:"body"`
	State         string            `json:"state"`
	Labels        []string          `json:"labels"`
	Assignees     []string          `json:"assignees"`
	Author        string            `json:"author"`
	CommentsCount int               `json:"comments_count"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	URL           string            `json:"url"`
	ClosedAt      *time.Time        `json:"closed_at,omitempty"`
	AssignedAgent *AgentTaskSummary `json:"assigned_agent,omitempty"`
}

// IssueComment represents a comment on an issue or pull request.
type IssueComment struct {
	ID        int64     `json:"id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	URL       string    `json:"url,omitempty"`
}

// IssueDetail contains full issue metadata including comment thread.
type IssueDetail struct {
	Issue
	Comments []IssueComment `json:"comments"`
}

// PullRequest represents a GitHub Pull Request.
type PullRequest struct {
	Number         int               `json:"number"`
	Title          string            `json:"title"`
	Body           string            `json:"body"`
	State          string            `json:"state"` // "OPEN", "CLOSED", "MERGED"
	IsDraft        bool              `json:"is_draft"`
	HeadRef        string            `json:"head_ref"`
	BaseRef        string            `json:"base_ref"`
	Author         string            `json:"author"`
	Labels         []string          `json:"labels"`
	Assignees      []string          `json:"assignees"`
	ReviewDecision string            `json:"review_decision,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	URL            string            `json:"url"`
	MergedAt       *time.Time        `json:"merged_at,omitempty"`
	AssignedAgent  *AgentTaskSummary `json:"assigned_agent,omitempty"`
}

// PullRequestDetail contains PR details and comments.
type PullRequestDetail struct {
	PullRequest
	Comments []IssueComment `json:"comments"`
}

// ProjectBoard represents a GitHub Project or ProjectV2.
type ProjectBoard struct {
	ID          string `json:"id"`
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Closed      bool   `json:"closed"`
}

// KanbanCard represents an issue or pull request card on the Kanban board.
type KanbanCard struct {
	ID            string            `json:"id"`
	Type          string            `json:"type"` // "issue" or "pr"
	Number        int               `json:"number"`
	Title         string            `json:"title"`
	Body          string            `json:"body"`
	State         string            `json:"state"`     // "open", "closed", "merged"
	ColumnID      string            `json:"column_id"` // "todo", "in_progress", "review", "done"
	Labels        []string          `json:"labels"`
	Assignees     []string          `json:"assignees"`
	Author        string            `json:"author"`
	URL           string            `json:"url"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	AssignedAgent *AgentTaskSummary `json:"assigned_agent,omitempty"`
	ProjectItemID string            `json:"project_item_id,omitempty"`
}

// KanbanColumn represents a column on the Kanban board.
type KanbanColumn struct {
	ID    string       `json:"id"`    // "todo", "in_progress", "review", "done"
	Title string       `json:"title"` // "Todo", "In Progress", "Review", "Done"
	Cards []KanbanCard `json:"cards"`
}

// KanbanBoard represents the full Kanban board for the repository or a GitHub Project.
type KanbanBoard struct {
	ProjectID     string         `json:"project_id,omitempty"`
	ProjectTitle  string         `json:"project_title,omitempty"`
	IsSynthesized bool           `json:"is_synthesized"`
	Columns       []KanbanColumn `json:"columns"`
}

// MoveKanbanCardRequest parameters for moving a card to another column.
type MoveKanbanCardRequest struct {
	WorkspacePath string `json:"workspace_path"`
	CardID        string `json:"card_id"`
	CardType      string `json:"card_type"` // "issue" or "pr"
	Number        int    `json:"number"`
	SourceColumn  string `json:"source_column"`
	TargetColumn  string `json:"target_column"`
	ProjectNumber int    `json:"project_number,omitempty"`
	ProjectItemID string `json:"project_item_id,omitempty"`
}

// WorkItemRef represents a linked GitHub issue or pull request with live state and URL.
type WorkItemRef struct {
	Type   string `json:"type"`             // "issue" or "pr"
	Number int    `json:"number"`
	State  string `json:"state"`            // "open", "closed", "merged"
	URL    string `json:"url,omitempty"`
	Title  string `json:"title,omitempty"`
}

// AgentTaskSummary links an Antigravity conversation and agent to a task.
type AgentTaskSummary struct {
	ConversationID           string        `json:"conversation_id"`
	ConversationTitle        string        `json:"conversation_title"`
	WorkItem                 string        `json:"work_item,omitempty"`
	AgentName                string        `json:"agent_name"`
	AgentLabel               string        `json:"agent_label"` // User-assigned persona/label (e.g. "Keyring & Auto-Import Specialist")
	Status                   string        `json:"status"`      // "working", "idle", "completed"
	NotFullyIdle             bool          `json:"not_fully_idle"`
	ParentConversationID     string        `json:"parent_conversation_id,omitempty"`
	RootParentConversationID string        `json:"root_parent_conversation_id,omitempty"`
	RootParentTitle          string        `json:"root_parent_title,omitempty"`
	NestingDepth             int           `json:"nesting_depth,omitempty"`
	IsPruned                 bool          `json:"is_pruned,omitempty"`
	BoundIssueNumber         int           `json:"bound_issue_number,omitempty"`
	BoundPRNumber            int           `json:"bound_pr_number,omitempty"`
	WorkingIssues            []int         `json:"working_issues,omitempty"`
	WorkingPRs               []int         `json:"working_prs,omitempty"`
	WorkItems                []WorkItemRef `json:"work_items,omitempty"`
	LastModified             time.Time     `json:"last_modified"`
	WorkspaceURI             string        `json:"workspace_uri"`
	StepCount                int           `json:"step_count"`
}

// UpdateIssueRequest parameters for modifying an existing issue.
type UpdateIssueRequest struct {
	Title        *string  `json:"title,omitempty"`
	Body         *string  `json:"body,omitempty"`
	State        *string  `json:"state,omitempty"` // "open", "closed"
	AddLabels    []string `json:"add_labels,omitempty"`
	RemoveLabels []string `json:"remove_labels,omitempty"`
	AddAssignees []string `json:"add_assignees,omitempty"`
}

// CreateIssueRequest parameters for creating a new issue.
type CreateIssueRequest struct {
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Labels    []string `json:"labels,omitempty"`
	Assignees []string `json:"assignees,omitempty"`
}

// BindTaskRequest parameters for associating an agent conversation with an issue.
type BindTaskRequest struct {
	ConversationID string `json:"conversation_id"`
	IssueNumber    int    `json:"issue_number"`
	AgentLabel     string `json:"agent_label,omitempty"`
}

// SetLabelRequest parameters for assigning a label to an agent/conversation.
type SetLabelRequest struct {
	ConversationID string `json:"conversation_id"`
	AgentLabel     string `json:"agent_label"`
}
