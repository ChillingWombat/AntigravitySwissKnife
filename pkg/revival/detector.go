package revival

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
	_ "modernc.org/sqlite"
)

// SessionDetector detects the currently active conversation and subagent state.
type SessionDetector struct {
	BaseDir        string
	AppStoragePath string
	LiveCDPEnabled bool
}

// NewSessionDetector creates a SessionDetector with default OS directory paths.
func NewSessionDetector() *SessionDetector {
	return &SessionDetector{
		BaseDir:        core.GetAntigravityDir(),
		AppStoragePath: filepath.Join(core.GetAntigravityHostConfigDir(), "app_storage.json"),
		LiveCDPEnabled: true,
	}
}

// DetectActiveSession inspects Antigravity persistence layers to find the most recent active conversation.
func DetectActiveSession(appType string) (*ActiveSessionInfo, error) {
	return NewSessionDetector().Detect(appType)
}

// Detect finds the active session for the given appType ("desktop", "agy", "vscode", or empty).
func (d *SessionDetector) Detect(appType string) (*ActiveSessionInfo, error) {
	var dbPath string
	appType = strings.ToLower(strings.TrimSpace(appType))

	home, _ := os.UserHomeDir()
	if appType == "agy" && home != "" {
		cliDB := filepath.Join(home, ".gemini", "antigravity-cli", "conversation_summaries.db")
		if _, err := os.Stat(cliDB); err == nil {
			dbPath = cliDB
		}
	}

	if dbPath == "" {
		base := d.BaseDir
		if base == "" {
			base = core.GetAntigravityDir()
		}
		dbPath = filepath.Join(base, "conversation_summaries.db")
	}

	// 1. Try querying conversation_summaries.db
	if _, err := os.Stat(dbPath); err == nil {
		info, err := d.querySummaryDatabase(dbPath)
		if err == nil && info != nil && info.ConversationID != "" {
			info.TargetApp = appType
			if info.TargetPath == "" {
				info.TargetPath = "/c/" + info.ConversationID
			}
			return info, nil
		}
	}

	// 2. Fallback to live CDP if desktop is running
	if d.LiveCDPEnabled && (appType == "" || appType == "desktop" || appType == "all") {
		livePath := gui.NewInjector(0).CaptureActiveConversationPath()
		if livePath != "" && gui.IsValidConversationPath(livePath) {
			convID := strings.TrimPrefix(livePath, "/c/")
			if idx := strings.Index(convID, "?"); idx != -1 {
				convID = convID[:idx]
			}
			convID = strings.TrimSpace(convID)
			if convID != "" {
				baseDir := d.BaseDir
				if baseDir == "" {
					baseDir = core.GetAntigravityDir()
				}
				needsRev := d.checkTrajectoryRunning(baseDir, convID)
				return &ActiveSessionInfo{
					ConversationID: convID,
					TargetPath:     livePath,
					IsTopLevel:     true,
					TargetApp:      appType,
					NeedsRevival:   needsRev,
				}, nil
			}
		}
	}

	// 3. Fallback to app_storage.json layout keys
	info, err := d.fallbackToAppStorage()
	if err == nil && info != nil {
		info.TargetApp = appType
		return info, nil
	}

	return nil, fmt.Errorf("no active Antigravity conversation found")
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// querySummaryDatabase opens the database in read-only WAL mode and resolves the root conversation.
func (d *SessionDetector) querySummaryDatabase(dbPath string) (*ActiveSessionInfo, error) {
	// WAL read-only connection with busy timeout
	connStr := fmt.Sprintf("file:%s?mode=ro&_busy_timeout=5000", filepath.ToSlash(dbPath))
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open conversation_summaries.db: %w", err)
	}
	defer db.Close()

	var (
		convID, title, workspaceURI, parentID, status, rawModTime string
		depth, notFullyIdle                                       int
	)

	query := `SELECT conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time 
              FROM conversation_summaries 
              ORDER BY last_modified_time DESC LIMIT 1;`

	row := db.QueryRow(query)
	if err := row.Scan(&convID, &title, &workspaceURI, &parentID, &depth, &notFullyIdle, &status, &rawModTime); err != nil {
		return nil, err
	}

	activeSubagentID := ""
	needsRevival := (notFullyIdle != 0) || (status == "CASCADE_RUN_STATUS_RUNNING")

	rootConvID := convID
	rootTitle := title
	rootWorkspace := workspaceURI
	rootParentID := parentID
	rootDepth := depth
	rootNotFullyIdle := notFullyIdle
	rootStatus := status
	rootModTime := parseTime(rawModTime)

	// If latest record is a subagent (depth > 0 and parentID != ""), walk up the tree
	if depth > 0 && strings.TrimSpace(parentID) != "" {
		activeSubagentID = convID
		currParent := strings.TrimSpace(parentID)
		visited := make(map[string]bool)
		visited[convID] = true

		// Loop up to 50 levels deep to find the root user conversation
		for step := 0; step < 50 && currParent != ""; step++ {
			if visited[currParent] {
				break
			}
			visited[currParent] = true

			var (
				pID, pTitle, pWork, pNextParent, pStatus, pMod string
				pDepth, pIdle                                  int
			)
			pQuery := `SELECT conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time 
                       FROM conversation_summaries 
                       WHERE conversation_id = ?;`
			pRow := db.QueryRow(pQuery, currParent)
			if err := pRow.Scan(&pID, &pTitle, &pWork, &pNextParent, &pDepth, &pIdle, &pStatus, &pMod); err != nil {
				break
			}

			if pIdle != 0 || pStatus == "CASCADE_RUN_STATUS_RUNNING" {
				needsRevival = true
			}

			rootConvID = pID
			rootTitle = pTitle
			rootWorkspace = pWork
			rootParentID = pNextParent
			rootDepth = pDepth
			rootNotFullyIdle = pIdle
			rootStatus = pStatus
			rootModTime = parseTime(pMod)

			if pDepth == 0 || strings.TrimSpace(pNextParent) == "" {
				break
			}
			currParent = strings.TrimSpace(pNextParent)
		}
	}

	baseDir := filepath.Dir(dbPath)

	// Check if this root conversation has any running child subagents in the summaries DB
	subQuery := `SELECT conversation_id, not_fully_idle, status FROM conversation_summaries WHERE parent_conversation_id = ?;`
	if subRows, subErr := db.Query(subQuery, rootConvID); subErr == nil {
		defer subRows.Close()
		for subRows.Next() {
			var subID, subStatus string
			var subIdle int
			if err := subRows.Scan(&subID, &subIdle, &subStatus); err == nil {
				if subIdle != 0 || subStatus == "CASCADE_RUN_STATUS_RUNNING" || d.checkTrajectoryRunning(baseDir, subID) {
					needsRevival = true
					if activeSubagentID == "" {
						activeSubagentID = subID
					}
				}
			}
		}
	}

	// Check trajectory database for running step status (status == 2)
	if d.checkTrajectoryRunning(baseDir, rootConvID) || (activeSubagentID != "" && d.checkTrajectoryRunning(baseDir, activeSubagentID)) {
		needsRevival = true
	}

	isTopLevel := (rootDepth == 0 && rootParentID == "")

	return &ActiveSessionInfo{
		ConversationID:       rootConvID,
		Title:                rootTitle,
		WorkspaceURI:         rootWorkspace,
		LastModified:         rootModTime,
		IsTopLevel:           isTopLevel,
		ParentConversationID: rootParentID,
		NestingDepth:         rootDepth,
		NotFullyIdle:         rootNotFullyIdle != 0,
		Status:               rootStatus,
		NeedsRevival:         needsRevival,
		ActiveSubagentID:     activeSubagentID,
		TargetPath:           "/c/" + rootConvID,
	}, nil
}

// checkTrajectoryRunning checks if the latest step in conversations/<id>.db is status 2 (RUNNING).
func (d *SessionDetector) checkTrajectoryRunning(baseDir, conversationID string) bool {
	if conversationID == "" {
		return false
	}
	trajDB := filepath.Join(baseDir, "conversations", conversationID+".db")
	if _, err := os.Stat(trajDB); err != nil {
		return false
	}

	connStr := fmt.Sprintf("file:%s?mode=ro&_busy_timeout=2000", filepath.ToSlash(trajDB))
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return false
	}
	defer db.Close()

	var lastStepStatus int
	err = db.QueryRow(`SELECT status FROM steps ORDER BY idx DESC LIMIT 1;`).Scan(&lastStepStatus)
	if err == nil && lastStepStatus == 2 {
		return true
	}
	return false
}

// fallbackToAppStorage attempts to extract active conversation from app_storage.json.
func (d *SessionDetector) fallbackToAppStorage() (*ActiveSessionInfo, error) {
	storagePath := d.AppStoragePath
	if storagePath == "" {
		storagePath = filepath.Join(core.GetAntigravityHostConfigDir(), "app_storage.json")
	}

	data, err := os.ReadFile(storagePath)
	if err != nil {
		return nil, err
	}

	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return nil, err
	}

	baseDir := d.BaseDir
	if baseDir == "" {
		baseDir = core.GetAntigravityDir()
	}

	resolveSession := func(convID string, targetPath string) *ActiveSessionInfo {
		needsRev := d.checkTrajectoryRunning(baseDir, convID)
		if !needsRev {
			dbPath := filepath.Join(baseDir, "conversation_summaries.db")
			if _, statErr := os.Stat(dbPath); statErr == nil {
				if info, qErr := d.querySummaryDatabase(dbPath); qErr == nil && info != nil {
					if info.ConversationID == convID || info.ActiveSubagentID == convID {
						needsRev = info.NeedsRevival
					}
				}
			}
		}
		return &ActiveSessionInfo{
			ConversationID: convID,
			TargetPath:     targetPath,
			IsTopLevel:     true,
			NeedsRevival:   needsRev,
		}
	}

	// 1. Check pinned_conversations_order
	if pinnedRaw, ok := rawMap["pinned_conversations_order"].(string); ok && pinnedRaw != "" {
		var pinned []string
		if err := json.Unmarshal([]byte(pinnedRaw), &pinned); err == nil && len(pinned) > 0 {
			for _, p := range pinned {
				p = strings.TrimSpace(p)
				if gui.IsValidConversationID(p) {
					return resolveSession(p, "/c/"+p), nil
				}
			}
		}
	}

	// 2. Check multi-conversation layout keys: "antigravity-multi-conversation-layout-v3-<cascadeId>"
	const layoutPrefix = "antigravity-multi-conversation-layout-v3-"
	var candidates []string
	for k := range rawMap {
		if strings.HasPrefix(k, layoutPrefix) && k != layoutPrefix+"index" {
			convID := strings.TrimSpace(strings.TrimPrefix(k, layoutPrefix))
			if gui.IsValidConversationID(convID) {
				candidates = append(candidates, convID)
			}
		}
	}
	if len(candidates) > 0 {
		chosen := candidates[0]
		for _, c := range candidates {
			if gui.IsUUID(c) {
				chosen = c
				break
			}
		}
		return resolveSession(chosen, "/c/"+chosen), nil
	}

	// 3. Check explicit last conversation path
	if v, ok := rawMap["antigravity_swiss_last_conversation_path"].(string); ok && gui.IsValidConversationPath(v) {
		convID := strings.TrimPrefix(v, "/c/")
		if idx := strings.Index(convID, "?"); idx != -1 {
			convID = convID[:idx]
		}
		convID = strings.TrimSpace(convID)
		if convID != "" && gui.IsValidConversationID(convID) {
			return resolveSession(convID, v), nil
		}
	}

	return nil, fmt.Errorf("no conversation layout found in app_storage.json")
}
