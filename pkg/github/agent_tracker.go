package github

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var issueRegex = regexp.MustCompile(`(?i)(?:#|issue\s+|issue#)(\d+)`)

// AgentTracker queries conversation summaries to extract agent task activity.
type AgentTracker struct {
	sqlitePath string
	store      *Store
}

// NewAgentTracker creates a new AgentTracker.
func NewAgentTracker(sqlitePath string, store *Store) *AgentTracker {
	path := sqlitePath
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".gemini", "antigravity", "conversation_summaries.db")
	}
	return &AgentTracker{
		sqlitePath: path,
		store:      store,
	}
}

// ListWorkspaceTasks returns active and recent conversations belonging to the workspace.
func (at *AgentTracker) ListWorkspaceTasks(workspacePath string) ([]AgentTaskSummary, error) {
	if _, err := os.Stat(at.sqlitePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("conversation summaries database not found at %s", at.sqlitePath)
	}

	// Open read-only to avoid SQLite lock contention
	dsn := fmt.Sprintf("file:%s?mode=ro", at.sqlitePath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open conversation summaries db: %w", err)
	}
	defer db.Close()

	// Query last 100 conversations
	query := `SELECT conversation_id, title, step_count, last_modified_time, workspace_uris, agent_name, not_fully_idle,
	                 COALESCE(parent_conversation_id, ''), COALESCE(nesting_depth, 0)
	          FROM conversation_summaries
	          ORDER BY last_modified_time DESC
	          LIMIT 100`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query conversation summaries: %w", err)
	}
	defer rows.Close()

	var tasks []AgentTaskSummary
	normTarget := normalizePath(workspacePath)

	for rows.Next() {
		var (
			convID       string
			title        string
			stepCount    int
			lastModStr   string
			urisJSON     string
			agentName    string
			notFullyIdle bool
			parentConvID string
			nestingDepth int
		)

		if err := rows.Scan(&convID, &title, &stepCount, &lastModStr, &urisJSON, &agentName, &notFullyIdle, &parentConvID, &nestingDepth); err != nil {
			continue
		}

		// Check if this conversation belongs to the requested workspace
		if !matchesWorkspace(urisJSON, normTarget) {
			continue
		}

		lastMod, _ := time.Parse(time.RFC3339Nano, lastModStr)
		if lastMod.IsZero() {
			lastMod, _ = time.Parse("2006-01-02 15:04:05.999999999-07:00", lastModStr)
		}

		// Determine bound issue number: first check explicit store binding, then infer from title
		boundIssue := 0
		if at.store != nil {
			boundIssue = at.store.GetBoundIssue(convID)
		}
		if boundIssue == 0 {
			if m := issueRegex.FindStringSubmatch(title); len(m) > 1 {
				boundIssue, _ = strconv.Atoi(m[1])
			}
		}

		// Determine custom agent label
		agentLabel := ""
		if at.store != nil {
			agentLabel = at.store.GetAgentLabel(convID)
		}
		if agentLabel == "" {
			if agentName != "" && agentName != "self" {
				agentLabel = agentName
			} else {
				agentLabel = "Agent"
			}
		}

		status := "idle"
		if notFullyIdle {
			status = "working"
		}

		tasks = append(tasks, AgentTaskSummary{
			ConversationID:       convID,
			ConversationTitle:    title,
			AgentName:            agentName,
			AgentLabel:           agentLabel,
			Status:               status,
			NotFullyIdle:         notFullyIdle,
			ParentConversationID: parentConvID,
			NestingDepth:         nestingDepth,
			BoundIssueNumber:     boundIssue,
			LastModified:         lastMod,
			WorkspaceURI:         urisJSON,
			StepCount:            stepCount,
		})
	}

	return tasks, nil
}

func normalizePath(p string) string {
	if p == "" {
		return ""
	}
	p = strings.TrimPrefix(p, "file://")
	if unescaped, err := url.PathUnescape(p); err == nil {
		p = unescaped
	}
	return filepath.Clean(strings.ToLower(p))
}

func matchesWorkspace(urisJSON string, normTarget string) bool {
	if normTarget == "" {
		return true // Return all if no target specified
	}
	var uris []string
	if err := json.Unmarshal([]byte(urisJSON), &uris); err != nil {
		return strings.Contains(strings.ToLower(urisJSON), normTarget)
	}
	for _, u := range uris {
		if normalizePath(u) == normTarget || strings.Contains(normalizePath(u), normTarget) {
			return true
		}
	}
	return false
}
