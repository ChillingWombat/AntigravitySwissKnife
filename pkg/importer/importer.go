package importer

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// CandidateSession represents a discovered session from an external agent.
type CandidateSession struct {
	ID                     string `json:"id"`
	Source                 string `json:"source"`
	Title                  string `json:"title"`
	MessageCount           int    `json:"message_count"`
	ToolCallsCount         int    `json:"tool_calls_count"`
	TokenEstimate          int    `json:"token_estimate"`
	DetectedProjectPath    string `json:"detected_project_path"`
	TargetProjectID        string `json:"target_project_id"`
	TargetAntigravityProj string `json:"target_antigravity_project"`
	MatchStatus            string `json:"match_status"`
	Selected               bool   `json:"selected"`
}

// ImportResult describes the outcome of importing a single session.
type ImportResult struct {
	SourceID       string `json:"source_id"`
	ConversationID string `json:"conversation_id"`
	Title          string `json:"title"`
	ProjectID      string `json:"project_id"`
	StepCount      int    `json:"step_count"`
	Success        bool   `json:"success"`
	Error          string `json:"error,omitempty"`
}

// ParsedStep represents a normalized conversation step.
type ParsedStep struct {
	StepIndex int    `json:"step_index"`
	Source    string `json:"source"` // "USER_EXPLICIT" or "MODEL"
	Type      string `json:"type"`   // "USER_INPUT" or "PLANNER_RESPONSE"
	Status    string `json:"status"` // "DONE"
	CreatedAt string `json:"created_at"`
	Content   string `json:"content"`
}

// ProjectInfo stores Antigravity project metadata.
type ProjectInfo struct {
	ProjectID   string
	ProjectName string
	Paths       []string
}

// DefaultPaths returns standard Antigravity storage directories.
func DefaultPaths() (antigravityBase, appStoragePath string) {
	home, _ := os.UserHomeDir()
	antigravityBase = filepath.Join(home, ".gemini", "antigravity")
	appStoragePath = filepath.Join(home, ".config", "Antigravity", "app_storage.json")
	return
}

// GetKnownProjects parses Antigravity's known projects and workspace associations.
func GetKnownProjects(antigravityBase, appStoragePath string) ([]ProjectInfo, error) {
	var projects []ProjectInfo
	projectMap := make(map[string]*ProjectInfo)

	// 1. Read app_storage.json for projectsOrder
	if data, err := os.ReadFile(appStoragePath); err == nil {
		var storage map[string]interface{}
		if err := json.Unmarshal(data, &storage); err == nil {
			if orderRaw, ok := storage["projectsOrder"]; ok {
				var ids []string
				switch v := orderRaw.(type) {
				case string:
					_ = json.Unmarshal([]byte(v), &ids)
				case []interface{}:
					for _, item := range v {
						if s, ok := item.(string); ok {
							ids = append(ids, s)
						}
					}
				}
				for _, id := range ids {
					p := &ProjectInfo{ProjectID: id, ProjectName: id}
					projectMap[id] = p
					projects = append(projects, *p)
				}
			}
		}
	}

	// 2. Query conversation_summaries.db for workspace paths
	summariesDB := filepath.Join(antigravityBase, "conversation_summaries.db")
	if _, err := os.Stat(summariesDB); err == nil {
		db, err := sql.Open("sqlite", summariesDB)
		if err == nil {
			defer db.Close()
			rows, err := db.Query("SELECT project_id, workspace_uris FROM conversation_summaries WHERE workspace_uris IS NOT NULL AND workspace_uris != '' GROUP BY project_id, workspace_uris")
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var pid, urisJSON string
					if err := rows.Scan(&pid, &urisJSON); err == nil && pid != "" {
						var uris []string
						_ = json.Unmarshal([]byte(urisJSON), &uris)
						var cleanPaths []string
						for _, u := range uris {
							if strings.HasPrefix(u, "file://") {
								parsed, _ := url.PathUnescape(u[7:])
								cleanPaths = append(cleanPaths, filepath.Clean(parsed))
							}
						}
						if len(cleanPaths) > 0 {
							name := filepath.Base(cleanPaths[0])
							if p, exists := projectMap[pid]; exists {
								p.ProjectName = name
								p.Paths = append(p.Paths, cleanPaths...)
							} else {
								p := &ProjectInfo{ProjectID: pid, ProjectName: name, Paths: cleanPaths}
								projectMap[pid] = p
								projects = append(projects, *p)
							}
						}
					}
				}
			}
		}
	}

	// Sync back projectMap changes to projects slice
	for i := range projects {
		if updated, ok := projectMap[projects[i].ProjectID]; ok {
			projects[i] = *updated
		}
	}

	return projects, nil
}

// MatchWorkspaceProject performs 4-tier project matching against known Antigravity projects.
func MatchWorkspaceProject(detectedPath string, projects []ProjectInfo) (projectID, projectName, matchStatus string) {
	if detectedPath == "" {
		if len(projects) > 0 {
			return projects[0].ProjectID, projects[0].ProjectName, "fallback"
		}
		return "standalone", "Standalone Chats", "standalone"
	}

	cleanDetected := filepath.Clean(detectedPath)

	// Tier 1: Exact match
	for _, p := range projects {
		for _, w := range p.Paths {
			if filepath.Clean(w) == cleanDetected {
				return p.ProjectID, p.ProjectName, "exact"
			}
		}
	}

	// Tier 2: Subpath match
	for _, p := range projects {
		for _, w := range p.Paths {
			cleanW := filepath.Clean(w)
			if strings.HasPrefix(cleanDetected, cleanW+string(filepath.Separator)) || strings.HasPrefix(cleanW, cleanDetected+string(filepath.Separator)) {
				return p.ProjectID, p.ProjectName, "heuristic"
			}
		}
	}

	// Tier 3: Basename match
	base := filepath.Base(cleanDetected)
	for _, p := range projects {
		for _, w := range p.Paths {
			if strings.EqualFold(filepath.Base(w), base) {
				return p.ProjectID, p.ProjectName, "heuristic"
			}
		}
	}

	// Tier 4: Fallback
	if len(projects) > 0 {
		return projects[0].ProjectID, projects[0].ProjectName, "fallback"
	}
	return "standalone", base, "fallback"
}

// ScanClaudeCode scans ~/.claude/transcripts for session jsonl logs.
func ScanClaudeCode(customDir string, antigravityBase, appStoragePath string) ([]CandidateSession, error) {
	dir := customDir
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".claude", "transcripts")
	}

	var candidates []CandidateSession
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return candidates, nil
		}
		return nil, err
	}

	projects, _ := GetKnownProjects(antigravityBase, appStoragePath)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		fullPath := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		file, err := os.Open(fullPath)
		if err != nil {
			continue
		}

		msgCount := 0
		toolCount := 0
		title := fmt.Sprintf("Claude Session (%s)", entry.Name())
		detectedPath := ""

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			msgCount++
			if strings.Contains(line, `"type":"tool_use"`) || strings.Contains(line, `"tool_name"`) {
				toolCount++
			}
			if strings.HasPrefix(title, "Claude Session") && (strings.Contains(line, `"type":"user"`) || strings.Contains(line, `"role":"user"`)) {
				var msgObj map[string]interface{}
				if err := json.Unmarshal([]byte(line), &msgObj); err == nil {
					if c, ok := msgObj["content"].(string); ok && c != "" {
						t := strings.TrimSpace(c)
						if len(t) > 75 {
							t = t[:72] + "..."
						}
						title = t
					}
					if cwd, ok := msgObj["cwd"].(string); ok && cwd != "" {
						detectedPath = cwd
					}
				}
			}
		}
		file.Close()

		pid, pName, status := MatchWorkspaceProject(detectedPath, projects)
		tokEst := msgCount * 150
		if info.Size() > 0 {
			tokEst = int(info.Size() / 4)
		}

		candidates = append(candidates, CandidateSession{
			ID:                     entry.Name(),
			Source:                 "claude-code",
			Title:                  title,
			MessageCount:           msgCount,
			ToolCallsCount:         toolCount,
			TokenEstimate:          tokEst,
			DetectedProjectPath:    detectedPath,
			TargetProjectID:        pid,
			TargetAntigravityProj: pName,
			MatchStatus:            status,
			Selected:               true,
		})
	}

	return candidates, nil
}

// ScanChatGPT scans a ChatGPT export JSON file (conversations.json).
func ScanChatGPT(filePath string, antigravityBase, appStoragePath string) ([]CandidateSession, error) {
	var candidates []CandidateSession
	data, err := os.ReadFile(filePath)
	if err != nil {
		return candidates, err
	}

	projects, _ := GetKnownProjects(antigravityBase, appStoragePath)

	var sessions []map[string]interface{}
	if err := json.Unmarshal(data, &sessions); err != nil {
		// Could be a single conversation object
		var single map[string]interface{}
		if err2 := json.Unmarshal(data, &single); err2 == nil {
			sessions = []map[string]interface{}{single}
		} else {
			return candidates, fmt.Errorf("invalid ChatGPT JSON: %w", err)
		}
	}

	for _, s := range sessions {
		id, _ := s["id"].(string)
		if id == "" {
			id = uuid.New().String()
		}
		title, _ := s["title"].(string)
		if title == "" {
			title = "ChatGPT Conversation"
		}

		msgCount := 0
		if mapping, ok := s["mapping"].(map[string]interface{}); ok {
			msgCount = len(mapping)
		}

		pid, pName, status := MatchWorkspaceProject("", projects)

		candidates = append(candidates, CandidateSession{
			ID:                     id,
			Source:                 "chatgpt",
			Title:                  "[ChatGPT] " + title,
			MessageCount:           msgCount,
			ToolCallsCount:         0,
			TokenEstimate:          msgCount * 220,
			DetectedProjectPath:    "",
			TargetProjectID:        pid,
			TargetAntigravityProj: pName,
			MatchStatus:            status,
			Selected:               true,
		})
	}

	return candidates, nil
}

// ParseClaudeCodeFile parses a Claude Code jsonl log into normalized ParsedSteps.
func ParseClaudeCodeFile(filePath string) ([]ParsedStep, string, string, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, "", "", "", err
	}
	defer file.Close()

	var steps []ParsedStep
	scanner := bufio.NewScanner(file)
	preview := "Imported Claude Code conversation"
	detectedPath := ""
	firstUserMsg := ""

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var msg map[string]interface{}
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		mType, _ := msg["type"].(string)
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)
		ts, _ := msg["timestamp"].(string)
		if ts == "" {
			ts = time.Now().UTC().Format(time.RFC3339)
		}
		if cwd, ok := msg["cwd"].(string); ok && cwd != "" {
			detectedPath = cwd
		}

		source := "USER_EXPLICIT"
		stepType := "USER_INPUT"

		if mType == "user" || role == "user" {
			source = "USER_EXPLICIT"
			stepType = "USER_INPUT"
			if firstUserMsg == "" && content != "" {
				firstUserMsg = content
				preview = content
				if len(preview) > 150 {
					preview = preview[:147] + "..."
				}
			}
		} else {
			source = "MODEL"
			stepType = "PLANNER_RESPONSE"
			if content == "" {
				if toolName, ok := msg["tool_name"].(string); ok {
					content = fmt.Sprintf("[Tool Call: %s]", toolName)
				}
			}
		}

		steps = append(steps, ParsedStep{
			StepIndex: len(steps),
			Source:    source,
			Type:      stepType,
			Status:    "DONE",
			CreatedAt: ts,
			Content:   content,
		})
	}

	if len(steps) == 0 {
		steps = append(steps, ParsedStep{
			StepIndex: 0,
			Source:    "USER_EXPLICIT",
			Type:      "USER_INPUT",
			Status:    "DONE",
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
			Content:   "Imported Claude Code session",
		})
	}

	title := "Claude Code Conversation"
	if firstUserMsg != "" {
		t := strings.TrimSpace(firstUserMsg)
		if len(t) > 75 {
			t = t[:72] + "..."
		}
		title = "[Claude Code] " + t
	}

	return steps, title, preview, detectedPath, nil
}

// WriteConversationDB creates the per-session SQLite database at ~/.gemini/antigravity/conversations/<id>.db.
func WriteConversationDB(conversationsDir, conversationID string, steps []ParsedStep) error {
	if err := os.MkdirAll(conversationsDir, 0755); err != nil {
		return err
	}

	dbPath := filepath.Join(conversationsDir, conversationID+".db")
	_ = os.Remove(dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open conversation db %s: %w", dbPath, err)
	}
	defer db.Close()

	schema := `
	CREATE TABLE trajectory_meta (
		trajectory_id text PRIMARY KEY,
		cascade_id text,
		trajectory_type integer,
		source integer
	);
	CREATE TABLE steps (
		idx integer PRIMARY KEY,
		step_type integer NOT NULL DEFAULT 0,
		status integer NOT NULL DEFAULT 0,
		has_subtrajectory numeric NOT NULL DEFAULT 0,
		metadata blob,
		error_details blob,
		permissions blob,
		task_details blob,
		render_info blob,
		step_payload blob,
		step_format integer NOT NULL DEFAULT 0
	);
	CREATE TABLE gen_metadata (idx integer PRIMARY KEY, data blob, size integer NOT NULL DEFAULT 0);
	CREATE TABLE executor_metadata (idx integer PRIMARY KEY, data blob);
	CREATE TABLE parent_references (idx integer PRIMARY KEY, data blob);
	CREATE TABLE trajectory_metadata_blob (id text PRIMARY KEY DEFAULT "main", data blob);
	CREATE TABLE battle_mode_infos (idx integer PRIMARY KEY, data blob);
	`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	trajID := uuid.New().String()
	_, err = db.Exec("INSERT INTO trajectory_meta (trajectory_id, cascade_id, trajectory_type, source) VALUES (?, ?, 4, 1)", trajID, conversationID)
	if err != nil {
		return fmt.Errorf("failed to insert trajectory_meta: %w", err)
	}

	stmt, err := db.Prepare("INSERT INTO steps (idx, step_type, status, has_subtrajectory, step_payload, step_format) VALUES (?, ?, 5, 0, ?, 0)")
	if err != nil {
		return fmt.Errorf("failed to prepare step insert: %w", err)
	}
	defer stmt.Close()

	for _, st := range steps {
		stepTypeCode := 15 // PLANNER_RESPONSE
		if st.Type == "USER_INPUT" {
			stepTypeCode = 14
		}
		_, err := stmt.Exec(st.StepIndex, stepTypeCode, []byte(st.Content))
		if err != nil {
			return fmt.Errorf("failed to insert step %d: %w", st.StepIndex, err)
		}
	}

	return nil
}

// WriteTranscriptLog writes the transcript.jsonl file into ~/.gemini/antigravity/brain/<id>/.system_generated/logs/.
func WriteTranscriptLog(brainBaseDir, conversationID string, steps []ParsedStep) error {
	logDir := filepath.Join(brainBaseDir, conversationID, ".system_generated", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(logDir, "transcript.jsonl"))
	if err != nil {
		return err
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	for _, st := range steps {
		data, err := json.Marshal(st)
		if err == nil {
			writer.Write(data)
			writer.WriteString("\n")
		}
	}
	return writer.Flush()
}

// InsertConversationSummary inserts or updates the master catalog row in conversation_summaries.db.
func InsertConversationSummary(summariesDBPath, conversationID, title, preview string, stepCount int, workspaceURIs []string, projectID, source, agentName string) error {
	db, err := sql.Open("sqlite", summariesDBPath)
	if err != nil {
		return fmt.Errorf("failed to open summaries db: %w", err)
	}
	defer db.Close()

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS conversation_summaries (
		conversation_id text PRIMARY KEY,
		title text NOT NULL DEFAULT "",
		preview text NOT NULL DEFAULT "",
		step_count integer NOT NULL DEFAULT 0,
		last_modified_time datetime NOT NULL,
		workspace_uris text NOT NULL,
		status text NOT NULL DEFAULT "",
		source text NOT NULL DEFAULT "",
		project_id text NOT NULL DEFAULT "",
		agent_name text NOT NULL DEFAULT "",
		parent_conversation_id text NOT NULL DEFAULT "",
		nesting_depth integer NOT NULL DEFAULT 0,
		battle_id text NOT NULL DEFAULT "",
		winning_conversation_id text NOT NULL DEFAULT "",
		not_fully_idle numeric NOT NULL DEFAULT 0,
		killed numeric NOT NULL DEFAULT 0,
		last_user_input_time datetime NOT NULL,
		last_user_input_step_index integer NOT NULL DEFAULT -1,
		app_data_dir text NOT NULL DEFAULT "",
		raw_summary blob,
		group_id text NOT NULL DEFAULT ""
	);`
	if _, err := db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("failed to ensure conversation_summaries table: %w", err)
	}

	urisJSON, _ := json.Marshal(workspaceURIs)
	now := time.Now().UTC().Format(time.RFC3339)

	insertSQL := `
	INSERT OR REPLACE INTO conversation_summaries (
		conversation_id, title, preview, step_count, last_modified_time, workspace_uris,
		status, source, project_id, agent_name, last_user_input_time, last_user_input_step_index
	) VALUES (?, ?, ?, ?, ?, ?, 'DONE', ?, ?, ?, ?, 0);
	`
	_, err = db.Exec(insertSQL, conversationID, title, preview, stepCount, now, string(urisJSON), source, projectID, agentName, now)
	return err
}

// ImportClaudeCodeSession imports a single Claude Code transcript file into Antigravity SQLite storage.
func ImportClaudeCodeSession(transcriptPath, antigravityBase, appStoragePath string, targetProjectID string) (*ImportResult, error) {
	steps, title, preview, detectedPath, err := ParseClaudeCodeFile(transcriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transcript: %w", err)
	}

	cid := uuid.New().String()

	// Project Matching
	projects, _ := GetKnownProjects(antigravityBase, appStoragePath)
	projectID := targetProjectID
	workspaceURIs := []string{}

	if projectID == "" {
		pID, _, _ := MatchWorkspaceProject(detectedPath, projects)
		projectID = pID
	}

	// Find workspace URIs for matching project
	for _, p := range projects {
		if p.ProjectID == projectID {
			for _, w := range p.Paths {
				workspaceURIs = append(workspaceURIs, "file://"+url.PathEscape(w))
			}
			break
		}
	}
	if len(workspaceURIs) == 0 {
		workspaceURIs = []string{"file://" + url.PathEscape("/mnt/Data/Projects/Antigravity Swiss Knife")}
	}

	// 1. Write per-session DB
	convDir := filepath.Join(antigravityBase, "conversations")
	if err := WriteConversationDB(convDir, cid, steps); err != nil {
		return nil, fmt.Errorf("failed to write conversation db: %w", err)
	}

	// 2. Write transcript log
	brainDir := filepath.Join(antigravityBase, "brain")
	_ = WriteTranscriptLog(brainDir, cid, steps)

	// 3. Write conversation_summaries catalog entry
	summariesDB := filepath.Join(antigravityBase, "conversation_summaries.db")
	if err := InsertConversationSummary(summariesDB, cid, title, preview, len(steps), workspaceURIs, projectID, "CLAUDE_CODE_IMPORT", "Claude Code"); err != nil {
		return nil, fmt.Errorf("failed to insert summary catalog: %w", err)
	}

	return &ImportResult{
		SourceID:       filepath.Base(transcriptPath),
		ConversationID: cid,
		Title:          title,
		ProjectID:      projectID,
		StepCount:      len(steps),
		Success:        true,
	}, nil
}
