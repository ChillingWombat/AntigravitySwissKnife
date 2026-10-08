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

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	_ "modernc.org/sqlite"
)

var issueRegex = regexp.MustCompile(`(?i)(?:#|issue\s+|issue#)(\d+)`)

// AgentTracker queries conversation summaries to extract agent task activity.
type AgentTracker struct {
	sqlitePath       string
	conversationsDir string
	store            *Store
}

// NewAgentTracker creates a new AgentTracker.
func NewAgentTracker(sqlitePath string, store *Store) *AgentTracker {
	path := sqlitePath
	if path == "" {
		path = filepath.Join(core.GetAntigravityDir(), "conversation_summaries.db")
	}
	return &AgentTracker{
		sqlitePath: path,
		store:      store,
	}
}

// SetConversationsDir sets the conversations directory (useful for testing).
func (at *AgentTracker) SetConversationsDir(dir string) {
	at.conversationsDir = dir
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

	convsDir := at.conversationsDir
	if convsDir == "" {
		if at.sqlitePath != "" {
			parentDir := filepath.Dir(at.sqlitePath)
			candidate := filepath.Join(parentDir, "conversations")
			if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
				convsDir = candidate
			}
		}
		if convsDir == "" {
			convsDir = core.GetConversationsDir()
		}
	}
	parentCache := make(map[string]string)

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

		// Determine root parent conversation ID for subagents
		rootParentID := ""
		if parentConvID != "" {
			if cached, ok := parentCache[parentConvID]; ok {
				rootParentID = cached
			} else {
				rootParentID = resolveRootParentID(db, parentConvID)
				parentCache[parentConvID] = rootParentID
			}
		}

		// Check if physical SQLite db file exists (either for this conversation or its root parent)
		isPruned := false
		if convsDir != "" {
			dbFile := filepath.Join(convsDir, convID+".db")
			if _, err := os.Stat(dbFile); os.IsNotExist(err) {
				isPruned = true
			} else if rootParentID != "" {
				rootFile := filepath.Join(convsDir, rootParentID+".db")
				if _, err := os.Stat(rootFile); os.IsNotExist(err) {
					isPruned = true
				}
			}
		}

		tasks = append(tasks, AgentTaskSummary{
			ConversationID:           convID,
			ConversationTitle:        title,
			AgentName:                agentName,
			AgentLabel:               agentLabel,
			Status:                   status,
			NotFullyIdle:             notFullyIdle,
			ParentConversationID:     parentConvID,
			RootParentConversationID: rootParentID,
			NestingDepth:             nestingDepth,
			IsPruned:                 isPruned,
			BoundIssueNumber:         boundIssue,
			LastModified:             lastMod,
			WorkspaceURI:             urisJSON,
			StepCount:                stepCount,
		})
	}

	return tasks, nil
}

// resolveRootParentID resolves the root parent conversation ID by walking up the ancestor chain.
func resolveRootParentID(db *sql.DB, parentID string) string {
	curr := parentID
	visited := make(map[string]bool)
	for i := 0; i < 50 && curr != ""; i++ {
		if visited[curr] {
			break
		}
		visited[curr] = true
		var nextParent string
		var depth int
		err := db.QueryRow(`SELECT COALESCE(parent_conversation_id, ''), COALESCE(nesting_depth, 0) FROM conversation_summaries WHERE conversation_id = ?`, curr).Scan(&nextParent, &depth)
		if err != nil || nextParent == "" {
			return curr
		}
		curr = nextParent
	}
	return curr
}

// ResolveProjectPath resolves a workspace path or bare project name to an absolute directory path when possible.
func ResolveProjectPath(nameOrPath string) string {
	p := strings.TrimSpace(nameOrPath)
	p = strings.Trim(p, "\"'`")
	if p == "" || p == "." || strings.EqualFold(p, "global") {
		if wd, err := os.Getwd(); err == nil && wd != "" {
			return filepath.Clean(wd)
		}
		return "."
	}
	if strings.HasPrefix(p, "file://") {
		p = strings.TrimPrefix(p, "file://")
		if unescaped, err := url.PathUnescape(p); err == nil {
			p = unescaped
		}
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	if wd, err := os.Getwd(); err == nil && wd != "" {
		candidate := filepath.Join(wd, p)
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Clean(candidate)
		}
		firstSeg := p
		rest := ""
		if idx := strings.IndexAny(p, `/\`); idx >= 0 {
			firstSeg = p[:idx]
			rest = p[idx+1:]
		}
		curr := wd
		for i := 0; i < 15 && curr != "" && curr != "/" && curr != "."; i++ {
			if strings.EqualFold(filepath.Base(curr), firstSeg) {
				target := curr
				if rest != "" {
					target = filepath.Join(curr, rest)
				}
				if _, err := os.Stat(target); err == nil {
					return filepath.Clean(target)
				}
				if _, err := os.Stat(filepath.Dir(target)); err == nil {
					return filepath.Clean(target)
				}
			}
			sibling := filepath.Join(curr, firstSeg)
			if fi, err := os.Stat(sibling); err == nil && fi.IsDir() {
				target := sibling
				if rest != "" {
					target = filepath.Join(sibling, rest)
				}
				if _, err := os.Stat(target); err == nil {
					return filepath.Clean(target)
				}
				if _, err := os.Stat(filepath.Dir(target)); err == nil {
					return filepath.Clean(target)
				}
			}
			parent := filepath.Dir(curr)
			if parent == curr {
				break
			}
			curr = parent
		}
	}
	dbPath := filepath.Join(core.GetAntigravityDir(), "conversation_summaries.db")
	if _, err := os.Stat(dbPath); err == nil {
		if db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro", dbPath)); err == nil {
			defer db.Close()
			rows, err := db.Query(`SELECT workspace_uris FROM conversation_summaries WHERE workspace_uris IS NOT NULL AND workspace_uris != '' GROUP BY workspace_uris ORDER BY MAX(last_modified_time) DESC`)
			if err == nil {
				defer rows.Close()
				firstSeg := p
				rest := ""
				if idx := strings.IndexAny(p, `/\`); idx >= 0 {
					firstSeg = p[:idx]
					rest = p[idx+1:]
				}
				for rows.Next() {
					var urisJSON string
					if err := rows.Scan(&urisJSON); err == nil {
						var uris []string
						if json.Unmarshal([]byte(urisJSON), &uris) == nil {
							for _, u := range uris {
								uPath := strings.TrimPrefix(u, "file://")
								if unescaped, err := url.PathUnescape(uPath); err == nil {
									uPath = unescaped
								}
								uPath = filepath.Clean(uPath)
								if strings.EqualFold(filepath.Base(uPath), firstSeg) {
									if rest != "" {
										return filepath.Join(uPath, rest)
									}
									return uPath
								}
							}
						}
					}
				}
			}
		}
	}
	return filepath.Clean(p)
}

func latestWorkspaceFromDB() string {
	dbPath := filepath.Join(core.GetAntigravityDir(), "conversation_summaries.db")
	if _, err := os.Stat(dbPath); err != nil {
		return ""
	}
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro", dbPath))
	if err != nil {
		return ""
	}
	defer db.Close()
	rows, err := db.Query(`SELECT workspace_uris FROM conversation_summaries WHERE workspace_uris IS NOT NULL AND workspace_uris != '' ORDER BY last_modified_time DESC LIMIT 20`)
	if err != nil {
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var urisJSON string
		if err := rows.Scan(&urisJSON); err != nil {
			continue
		}
		var uris []string
		if json.Unmarshal([]byte(urisJSON), &uris) != nil {
			continue
		}
		for _, u := range uris {
			uPath := strings.TrimPrefix(u, "file://")
			if unescaped, err := url.PathUnescape(uPath); err == nil {
				uPath = unescaped
			}
			uPath = filepath.Clean(uPath)
			if fi, err := os.Stat(uPath); err == nil && fi.IsDir() {
				return uPath
			}
		}
	}
	return ""
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
	if normTarget == "" || normTarget == "." || normTarget == "global" {
		return true // Return all if no target specified
	}
	var uris []string
	if err := json.Unmarshal([]byte(urisJSON), &uris); err != nil {
		return strings.Contains(strings.ToLower(urisJSON), normTarget)
	}
	for _, u := range uris {
		nu := normalizePath(u)
		if nu == normTarget || filepath.Base(nu) == normTarget || strings.Contains(nu, normTarget) || (len(nu) > 1 && strings.Contains(normTarget, nu)) {
			return true
		}
	}
	return false
}
