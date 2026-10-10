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
	"sync"
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

	// Query last 999 conversations
	query := `SELECT conversation_id, title, step_count, last_modified_time, workspace_uris, agent_name, not_fully_idle,
	                 COALESCE(parent_conversation_id, ''), COALESCE(nesting_depth, 0)
	          FROM conversation_summaries
	          ORDER BY last_modified_time DESC
	          LIMIT 999`

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
	parentCache := make(map[string]parentInfo)
	rosterMap := loadTeamworkRosters(workspacePath)
	selfReportMap := loadSelfReportedTasks(workspacePath)

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

		// Determine custom agent label / role
		agentLabel := ""
		if at.store != nil {
			agentLabel = at.store.GetAgentLabel(convID)
		}

		// Try extracting subagent role from parent transcript if this is a subagent
		subRole, subPrompt := "", ""
		if parentConvID != "" {
			subRole, subPrompt = getSubagentMetadata(parentConvID, convID)
		}
		if agentLabel == "" && subRole != "" {
			agentLabel = subRole
		}

		if agentLabel == "" {
			if agentName != "" && agentName != "self" {
				agentLabel = agentName
			} else if nestingDepth == 0 && title != "" {
				agentLabel = title
			} else {
				agentLabel = "Agent"
			}
		}

		var workItem string
		var workingIssues []int
		var workingPRs []int

		// 1. Self-reported metadata
		if sr, ok := selfReportMap[convID]; ok {
			if sr.WorkItem != "" {
				workItem = sr.WorkItem
			}
			if sr.AgentLabel != "" {
				agentLabel = sr.AgentLabel
			}
			workingIssues = sr.Issues
			workingPRs = sr.PRs
		}

		// 2. Teamwork roster metadata
		if tw, ok := rosterMap[convID]; ok {
			if workItem == "" && tw.WorkItem != "" {
				workItem = tw.WorkItem
			}
			if tw.Agent != "" && (agentLabel == "" || agentLabel == "Agent") {
				agentLabel = tw.Agent
			}
		}

		// 3. Transcript extraction (inspect for rich issues, PRs, and workItem)
		tsWork, tsIssues, tsPRs := extractTaskAndWorkItemsFromTranscript(convID, subPrompt)
		if workItem == "" && tsWork != "" {
			workItem = tsWork
		}
		workingIssues = appendUniqueInts(workingIssues, tsIssues)
		workingPRs = appendUniqueInts(workingPRs, tsPRs)

		// 4. Fallback: if title is present, use it; if title is empty, prioritize agentLabel over workItem
		if title == "" && agentLabel != "" {
			title = agentLabel
		}
		if workItem == "" && title != "" {
			workItem = title
		}
		if title == "" && workItem != "" {
			title = workItem
		}

		if boundIssue == 0 && len(workingIssues) == 1 {
			boundIssue = workingIssues[0]
		} else if boundIssue > 0 && len(workingIssues) == 0 {
			workingIssues = []int{boundIssue}
		}

		status := "idle"
		if notFullyIdle {
			status = "working"
		}

		// Determine root parent conversation ID and title for subagents
		rootParentID := ""
		rootParentTitle := ""
		if parentConvID != "" {
			if cached, ok := parentCache[parentConvID]; ok {
				rootParentID = cached.rootID
				rootParentTitle = cached.rootTitle
			} else {
				rID, rTitle := resolveRootParentInfo(db, parentConvID)
				rootParentID = rID
				rootParentTitle = rTitle
				parentCache[parentConvID] = parentInfo{rootID: rID, rootTitle: rTitle}
			}
		} else {
			rootParentID = ""
			rootParentTitle = title
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
			WorkItem:                 workItem,
			AgentName:                agentName,
			AgentLabel:               agentLabel,
			Status:                   status,
			NotFullyIdle:             notFullyIdle,
			ParentConversationID:     parentConvID,
			RootParentConversationID: rootParentID,
			RootParentTitle:          rootParentTitle,
			NestingDepth:             nestingDepth,
			IsPruned:                 isPruned,
			BoundIssueNumber:         boundIssue,
			WorkingIssues:            workingIssues,
			WorkingPRs:               workingPRs,
			LastModified:             lastMod,
			WorkspaceURI:             urisJSON,
			StepCount:                stepCount,
		})
	}

	return tasks, nil
}

type parentInfo struct {
	rootID    string
	rootTitle string
}

func resolveRootParentInfo(db *sql.DB, parentID string) (string, string) {
	curr := parentID
	visited := make(map[string]bool)
	var rootTitle string
	for i := 0; i < 50 && curr != ""; i++ {
		if visited[curr] {
			break
		}
		visited[curr] = true
		var nextParent string
		var title string
		var depth int
		err := db.QueryRow(`SELECT COALESCE(parent_conversation_id, ''), COALESCE(title, ''), COALESCE(nesting_depth, 0) FROM conversation_summaries WHERE conversation_id = ?`, curr).Scan(&nextParent, &title, &depth)
		if err != nil {
			return curr, rootTitle
		}
		if title != "" {
			rootTitle = title
		}
		if nextParent == "" {
			return curr, rootTitle
		}
		curr = nextParent
	}
	return curr, rootTitle
}

// resolveRootParentID resolves the root parent conversation ID by walking up the ancestor chain.
func resolveRootParentID(db *sql.DB, parentID string) string {
	id, _ := resolveRootParentInfo(db, parentID)
	return id
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
		if nu == normTarget {
			return true
		}
		if strings.HasPrefix(nu, normTarget+"/") || strings.HasPrefix(nu, normTarget+"\\") {
			return true
		}
		targetBase := filepath.Base(normTarget)
		if targetBase != "" && targetBase != "." && targetBase != "/" {
			if filepath.Base(nu) == targetBase || strings.EqualFold(filepath.Base(nu), targetBase) {
				return true
			}
		}
	}
	return false
}

type TeamworkAgentInfo struct {
	Agent    string
	Type     string
	WorkItem string
	Status   string
	ConvID   string
}

type SelfReportedTask struct {
	ConversationID string `json:"conversation_id"`
	WorkItem       string `json:"work_item"`
	AgentLabel     string `json:"agent_label"`
	Issues         []int  `json:"issues"`
	PRs            []int  `json:"prs"`
	Status         string `json:"status"`
}

func loadTeamworkRosters(workspacePath string) map[string]TeamworkAgentInfo {
	res := make(map[string]TeamworkAgentInfo)
	if workspacePath == "" || workspacePath == "." {
		workspacePath, _ = os.Getwd()
	}
	matches, _ := filepath.Glob(filepath.Join(workspacePath, ".agents", "teamwork", "*", "BRIEFING.md"))
	for _, m := range matches {
		data, err := os.ReadFile(m)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		inRoster := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "## Team Roster") {
				inRoster = true
				continue
			}
			if inRoster && strings.HasPrefix(trimmed, "## ") {
				inRoster = false
				continue
			}
			if inRoster && strings.HasPrefix(trimmed, "|") {
				parts := strings.Split(trimmed, "|")
				if len(parts) >= 6 {
					agent := strings.TrimSpace(parts[1])
					agentType := strings.TrimSpace(parts[2])
					workItem := strings.TrimSpace(parts[3])
					status := strings.TrimSpace(parts[4])
					convID := strings.TrimSpace(parts[5])
					if convID != "" && convID != "Conv ID" && !strings.Contains(convID, "---") {
						res[convID] = TeamworkAgentInfo{
							Agent:    agent,
							Type:     agentType,
							WorkItem: workItem,
							Status:   status,
							ConvID:   convID,
						}
					}
				}
			}
		}
	}
	return res
}

func loadSelfReportedTasks(workspacePath string) map[string]SelfReportedTask {
	res := make(map[string]SelfReportedTask)
	candidates := []string{
		filepath.Join(workspacePath, ".antigravity", "agent_tasks.json"),
		filepath.Join(workspacePath, ".agents", "agent_tasks.json"),
		filepath.Join(core.GetConfigDir(), "agent_tasks.json"),
	}
	for _, c := range candidates {
		data, err := os.ReadFile(c)
		if err != nil {
			continue
		}
		var m map[string]SelfReportedTask
		if err := json.Unmarshal(data, &m); err == nil {
			for k, v := range m {
				res[k] = v
			}
			continue
		}
		var list []SelfReportedTask
		if err := json.Unmarshal(data, &list); err == nil {
			for _, v := range list {
				if v.ConversationID != "" {
					res[v.ConversationID] = v
				}
			}
		}
	}
	return res
}

// ReportAgentTask saves a self-reported agent task to disk so that it's reliably discovered.
func ReportAgentTask(workspacePath string, task SelfReportedTask) error {
	if task.ConversationID == "" {
		return fmt.Errorf("conversation_id is required")
	}
	targetDir := ""
	if workspacePath != "" && workspacePath != "." && workspacePath != "GLOBAL" {
		targetDir = filepath.Join(workspacePath, ".antigravity")
	} else {
		targetDir = core.GetConfigDir()
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		targetDir = core.GetConfigDir()
		_ = os.MkdirAll(targetDir, 0755)
	}
	targetFile := filepath.Join(targetDir, "agent_tasks.json")

	// Read existing map or list
	data, _ := os.ReadFile(targetFile)
	taskMap := make(map[string]SelfReportedTask)
	if len(data) > 0 {
		var m map[string]SelfReportedTask
		if err := json.Unmarshal(data, &m); err == nil {
			taskMap = m
		} else {
			var list []SelfReportedTask
			if err := json.Unmarshal(data, &list); err == nil {
				for _, v := range list {
					if v.ConversationID != "" {
						taskMap[v.ConversationID] = v
					}
				}
			}
		}
	}

	taskMap[task.ConversationID] = task
	out, err := json.MarshalIndent(taskMap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(targetFile, out, 0644)
}

var (
	subagentMetaMu      sync.RWMutex
	subagentRoleCache   = make(map[string]map[string]string)
	subagentPromptCache = make(map[string]map[string]string)

	issueRegexStrict = regexp.MustCompile(`(?i)(?:issue\s*#?|issues/|issue:)\s*(\d+)`)
	prRegexStrict    = regexp.MustCompile(`(?i)(?:pr\s*#?|pull/|pull\s+request\s*#?|pr:)\s*(\d+)`)
	fixesRegex       = regexp.MustCompile(`(?i)(?:fixes|fixed|closes|closed|resolves|resolved)\s*#(\d+)`)
	branchRefRegex   = regexp.MustCompile(`(?i)(?:feature|wip|fix|bugfix)/(?:[a-zA-Z0-9_-]+/)?(?:issue-|pr-)?(\d+)-`)
	commitRefRegex   = regexp.MustCompile(`\(#(\d+)\)`)
)

func getSubagentMetadata(parentID, subID string) (string, string) {
	if parentID == "" || subID == "" {
		return "", ""
	}
	subagentMetaMu.RLock()
	if roles, ok := subagentRoleCache[parentID]; ok {
		role := roles[subID]
		prompt := subagentPromptCache[parentID][subID]
		subagentMetaMu.RUnlock()
		return role, prompt
	}
	subagentMetaMu.RUnlock()

	home, _ := os.UserHomeDir()
	logPath := filepath.Join(home, ".gemini", "antigravity", "brain", parentID, ".system_generated", "logs", "transcript_full.jsonl")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		logPath = filepath.Join(home, ".gemini", "antigravity", "brain", parentID, ".system_generated", "logs", "transcript.jsonl")
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		return "", ""
	}

	roleMap := make(map[string]string)
	promptMap := make(map[string]string)

	lines := strings.Split(string(data), "\n")
	type subagentSpec struct {
		Role     string `json:"Role"`
		Prompt   string `json:"Prompt"`
		TypeName string `json:"TypeName"`
	}
	var pendingSubs []subagentSpec

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "invoke_subagent") {
			var step struct {
				ToolCalls []struct {
					Name string `json:"name"`
					Args struct {
						Subagents []subagentSpec `json:"Subagents"`
					} `json:"args"`
				} `json:"tool_calls"`
			}
			if err := json.Unmarshal([]byte(trimmed), &step); err == nil {
				for _, tc := range step.ToolCalls {
					if tc.Name == "invoke_subagent" && len(tc.Args.Subagents) > 0 {
						pendingSubs = tc.Args.Subagents
					}
				}
			}
		}

		if len(pendingSubs) > 0 && strings.Contains(trimmed, "conversationId") {
			re := regexp.MustCompile(`"conversationId":\s*"([^"]+)"`)
			matches := re.FindAllStringSubmatch(trimmed, -1)
			for i, m := range matches {
				if len(m) > 1 && i < len(pendingSubs) {
					cid := m[1]
					roleMap[cid] = pendingSubs[i].Role
					promptMap[cid] = pendingSubs[i].Prompt
				}
			}
			pendingSubs = nil
		}
	}

	subagentMetaMu.Lock()
	subagentRoleCache[parentID] = roleMap
	subagentPromptCache[parentID] = promptMap
	subagentMetaMu.Unlock()

	return roleMap[subID], promptMap[subID]
}

func extractTaskAndWorkItemsFromTranscript(convID string, extraPrompt string) (string, []int, []int) {
	home, _ := os.UserHomeDir()
	logPath := filepath.Join(home, ".gemini", "antigravity", "brain", convID, ".system_generated", "logs", "transcript.jsonl")
	data, err := os.ReadFile(logPath)

	issueSet := make(map[int]bool)
	prSet := make(map[int]bool)
	var workItem string

	scanText := func(text string) {
		if text == "" {
			return
		}
		for _, m := range prRegexStrict.FindAllStringSubmatch(text, -1) {
			if len(m) > 1 {
				if num, err := strconv.Atoi(m[1]); err == nil && num > 0 && num < 1000 {
					prSet[num] = true
				}
			}
		}
		for _, m := range issueRegexStrict.FindAllStringSubmatch(text, -1) {
			if len(m) > 1 {
				if num, err := strconv.Atoi(m[1]); err == nil && num > 0 && num < 1000 {
					issueSet[num] = true
				}
			}
		}
		for _, m := range branchRefRegex.FindAllStringSubmatch(text, -1) {
			if len(m) > 1 {
				if num, err := strconv.Atoi(m[1]); err == nil && num > 0 && num < 1000 {
					issueSet[num] = true
				}
			}
		}
		for _, m := range commitRefRegex.FindAllStringSubmatch(text, -1) {
			if len(m) > 1 {
				if num, err := strconv.Atoi(m[1]); err == nil && num > 0 && num < 10000 {
					issueSet[num] = true
				}
			}
		}
		for _, m := range fixesRegex.FindAllStringSubmatch(text, -1) {
			if len(m) > 1 {
				if num, err := strconv.Atoi(m[1]); err == nil && num > 0 && num < 10000 {
					issueSet[num] = true
				}
			}
		}
	}

	if extraPrompt != "" {
		scanText(extraPrompt)
	}

	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			var step struct {
				Content string `json:"content"`
				Role    string `json:"role"`
				Type    string `json:"type"`
			}
			if err := json.Unmarshal([]byte(trimmed), &step); err == nil {
				isUserInput := step.Type == "USER_INPUT" || step.Role == "user"
				if isUserInput && step.Content != "" {
					c := step.Content
					if idx := strings.Index(c, "content="); idx != -1 {
						c = c[idx+len("content="):]
					}
					c = strings.TrimSpace(c)
					if workItem == "" && len(c) > 0 {
						firstLine := strings.Split(c, "\n")[0]
						if len(firstLine) > 100 {
							firstLine = firstLine[:100] + "..."
						}
						workItem = strings.TrimSpace(firstLine)
					}
					scanText(step.Content)
				}
			}
		}
	}

	var issues []int
	for n := range issueSet {
		if !prSet[n] {
			issues = append(issues, n)
		}
	}
	var prs []int
	for n := range prSet {
		prs = append(prs, n)
	}

	return workItem, issues, prs
}

func extractTaskFromTranscript(convID string) (string, []int) {
	workItem, issues, _ := extractTaskAndWorkItemsFromTranscript(convID, "")
	return workItem, issues
}

func appendUniqueInts(base []int, items []int) []int {
	seen := make(map[int]bool)
	for _, n := range base {
		seen[n] = true
	}
	res := append([]int{}, base...)
	for _, n := range items {
		if !seen[n] && n > 0 {
			seen[n] = true
			res = append(res, n)
		}
	}
	return res
}
