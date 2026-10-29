package revival

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
	_ "modernc.org/sqlite"
)

// MaxRevivalStaleness defines the maximum age of running records before treating them as stale residue.
const MaxRevivalStaleness = 2 * time.Hour

// SessionDetector detects the currently active conversation and subagent state.
type SessionDetector struct {
	BaseDir        string
	AppStoragePath string
	LiveCDPEnabled bool
	// LivePathResolver reports the conversation route the desktop renderer
	// currently shows ("" when the renderer is not reachable). It is the seam
	// the live-CDP path is tested through: without an injectable resolver the
	// only way to exercise that branch was to disable it, which is exactly how
	// the "open conversation ⇒ needs revival" regression hid.
	LivePathResolver func() string
}

// NewSessionDetector creates a SessionDetector with default OS directory paths.
func NewSessionDetector() *SessionDetector {
	return &SessionDetector{
		BaseDir:        core.GetAntigravityDir(),
		AppStoragePath: filepath.Join(core.GetAntigravityHostConfigDir(), "app_storage.json"),
		LiveCDPEnabled: true,
	}
}

// resolveLiveConversationPath returns the conversation route the desktop renderer
// is showing, or "" when no live renderer is reachable.
func (d *SessionDetector) resolveLiveConversationPath() string {
	if d.LivePathResolver != nil {
		return d.LivePathResolver()
	}
	return gui.NewInjector(0).CaptureActiveConversationPath()
}

// liveConversationID returns the validated conversation ID of the live renderer route.
func (d *SessionDetector) liveConversationID() string {
	livePath := d.resolveLiveConversationPath()
	if livePath == "" || !gui.IsValidConversationPath(livePath) {
		return ""
	}
	convID := strings.TrimPrefix(livePath, "/c/")
	if idx := strings.Index(convID, "?"); idx != -1 {
		convID = convID[:idx]
	}
	return strings.TrimSpace(convID)
}

// liveRouteAvailable reports whether a live renderer route can be consulted.
//
// An injected LivePathResolver is an explicit opt-in: the caller owns the source
// of the route, so the host-environment and DevToolsActivePort probes (which only
// describe a real desktop install) do not apply.
func (d *SessionDetector) liveRouteAvailable() bool {
	if !d.LiveCDPEnabled {
		return false
	}
	if d.LivePathResolver != nil {
		return true
	}
	isHostEnv := (d.BaseDir == "" || d.BaseDir == core.GetAntigravityDir())
	return isHostEnv && hasDevToolsActivePortFile()
}

// liveCDPActive reports whether the live-renderer branch may be consulted for appType.
func (d *SessionDetector) liveCDPActive(appType string) bool {
	appMatch := (appType == "" || appType == "desktop" || appType == "all")
	return appMatch && d.liveRouteAvailable()
}

// DetectActiveSession inspects Antigravity persistence layers to find the most recent active conversation.
func DetectActiveSession(appType string) (*ActiveSessionInfo, error) {
	return NewSessionDetector().Detect(appType)
}

// hasDevToolsActivePortFile checks whether DevToolsActivePort exists or ANTIGRAVITY_CDP_PORT is configured.
func hasDevToolsActivePortFile() bool {
	if os.Getenv("ANTIGRAVITY_CDP_PORT") != "" {
		return true
	}
	f1 := filepath.Join(core.GetAntigravityHostConfigDir(), "DevToolsActivePort")
	if _, err := os.Stat(f1); err == nil {
		return true
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		f2 := filepath.Join(home, ".config", "antigravity", "DevToolsActivePort")
		if _, err := os.Stat(f2); err == nil {
			return true
		}
	}
	return false
}

// Detect finds the active session for the given appType ("desktop", "agy", "vscode", or empty).
func (d *SessionDetector) Detect(appType string) (*ActiveSessionInfo, error) {
	var dbPath string
	appType = strings.ToLower(strings.TrimSpace(appType))

	// 1. Live CDP check first if desktop is running and DevToolsActivePort exists
	//
	// The live renderer tells us WHICH conversation is open. It says nothing about
	// whether that conversation was interrupted: a finished conversation that
	// merely happens to be open must not be revived. NeedsRevival therefore comes
	// only from the persisted signals (not_fully_idle / RUNNING status / running
	// trajectory steps) computed by DetectForID.
	if d.liveCDPActive(appType) {
		if convID := d.liveConversationID(); convID != "" {
			if info, err := d.DetectForID(convID); err == nil && info != nil {
				info.TargetApp = appType
				return info, nil
			}
		}
	}

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

	// 2. Try querying conversation_summaries.db
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

	// 3. Fallback to app_storage.json layout keys
	info, err := d.fallbackToAppStorage()
	if err == nil && info != nil {
		info.TargetApp = appType
		return info, nil
	}

	return nil, fmt.Errorf("no active Antigravity conversation found")
}

// DetectForID finds the active session info for a specific conversation ID.
func (d *SessionDetector) DetectForID(convID string) (*ActiveSessionInfo, error) {
	convID = strings.TrimSpace(strings.TrimPrefix(convID, "/c/"))
	if convID == "" {
		return d.Detect("")
	}
	base := d.BaseDir
	if base == "" {
		base = core.GetAntigravityDir()
	}

	// Check live CDP if a route is available
	if d.liveRouteAvailable() {
		if strings.EqualFold(d.liveConversationID(), convID) {
			dbPath := filepath.Join(base, "conversation_summaries.db")
			if _, err := os.Stat(dbPath); err == nil {
				if info, err := d.querySummaryDatabaseForID(dbPath, convID); err == nil && info != nil {
					return info, nil
				}
			}
		}
	}

	dbPath := filepath.Join(base, "conversation_summaries.db")
	if _, err := os.Stat(dbPath); err == nil {
		info, err := d.querySummaryDatabaseForID(dbPath, convID)
		if err == nil && info != nil && info.ConversationID != "" {
			if info.TargetPath == "" {
				info.TargetPath = "/c/" + info.ConversationID
			}
			return info, nil
		}
	}
	needsRev := d.checkTrajectoryRunning(base, convID)
	return &ActiveSessionInfo{
		ConversationID: convID,
		TargetPath:     "/c/" + convID,
		IsTopLevel:     true,
		NeedsRevival:   needsRev,
	}, nil
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
	return d.querySummaryDatabaseForID(dbPath, "")
}

// querySummaryDatabaseForID opens the database in read-only WAL mode and resolves either targetID or the latest root conversation.
func (d *SessionDetector) querySummaryDatabaseForID(dbPath string, targetID string) (*ActiveSessionInfo, error) {
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

	targetID = strings.TrimSpace(strings.TrimPrefix(targetID, "/c/"))
	if targetID != "" {
		query := `SELECT conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time 
                  FROM conversation_summaries 
                  WHERE conversation_id = ? LIMIT 1;`
		row := db.QueryRow(query, targetID)
		if err := row.Scan(&convID, &title, &workspaceURI, &parentID, &depth, &notFullyIdle, &status, &rawModTime); err != nil {
			return nil, err
		}
	} else {
		// Prioritize running / active conversations needing revival first (if fresh)
		queryRunning := `SELECT conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time 
                  FROM conversation_summaries 
                  WHERE not_fully_idle != 0 OR status = 'CASCADE_RUN_STATUS_RUNNING'
                  ORDER BY last_modified_time DESC LIMIT 1;`
		row := db.QueryRow(queryRunning)
		runningFound := false
		if err := row.Scan(&convID, &title, &workspaceURI, &parentID, &depth, &notFullyIdle, &status, &rawModTime); err == nil {
			modTime := parseTime(rawModTime)
			if !modTime.IsZero() && time.Since(modTime) <= MaxRevivalStaleness {
				runningFound = true
			}
		}
		if !runningFound {
			queryLatest := `SELECT conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time 
                  FROM conversation_summaries 
                  ORDER BY last_modified_time DESC LIMIT 1;`
			row = db.QueryRow(queryLatest)
			if err := row.Scan(&convID, &title, &workspaceURI, &parentID, &depth, &notFullyIdle, &status, &rawModTime); err != nil {
				return nil, err
			}
		}
	}

	return d.resolveSessionInfo(db, dbPath, convID, parentID, title, workspaceURI, depth, notFullyIdle, status, rawModTime)
}

// summaryRow is one conversation_summaries row.
type summaryRow struct {
	convID       string
	title        string
	workspaceURI string
	parentID     string
	status       string
	rawModTime   string
	depth        int
	notFullyIdle int
}

// resolveSessionInfo resolves a raw summaries row into an ActiveSessionInfo:
// it walks parent pointers up to the root conversation, folds in the root's
// directly-running child subagents, and consults the trajectory DB.
//
// This is the single resolution path shared by the single-session query
// (querySummaryDatabaseForID) and the collection query (DetectAll), so the two
// can never drift on what "needs revival" means.
func (d *SessionDetector) resolveSessionInfo(db *sql.DB, dbPath, convID, parentID, title, workspaceURI string, depth, notFullyIdle int, status, rawModTime string) (*ActiveSessionInfo, error) {
	activeSubagentID := ""
	modTime := parseTime(rawModTime)
	isFresh := !modTime.IsZero() && time.Since(modTime) <= MaxRevivalStaleness
	needsRevival := isFresh && ((notFullyIdle != 0) || (status == "CASCADE_RUN_STATUS_RUNNING"))

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

			pModTime := parseTime(pMod)
			pFresh := !pModTime.IsZero() && time.Since(pModTime) <= MaxRevivalStaleness
			if pFresh && (pIdle != 0 || pStatus == "CASCADE_RUN_STATUS_RUNNING") {
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
	subQuery := `SELECT conversation_id, not_fully_idle, status, last_modified_time FROM conversation_summaries WHERE parent_conversation_id = ?;`
	if subRows, subErr := db.Query(subQuery, rootConvID); subErr == nil {
		defer subRows.Close()
		for subRows.Next() {
			var subID, subStatus, subMod string
			var subIdle int
			if err := subRows.Scan(&subID, &subIdle, &subStatus, &subMod); err == nil {
				subT := parseTime(subMod)
				subFresh := !subT.IsZero() && time.Since(subT) <= MaxRevivalStaleness
				if subFresh && (subIdle != 0 || subStatus == "CASCADE_RUN_STATUS_RUNNING" || d.checkTrajectoryRunning(baseDir, subID)) {
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

// DetectAll returns every conversation that is genuinely interrupted, deduped by
// root conversation ID — one entry per root, newest first.
//
// A single account switch can kill several running conversations at once, so
// enumeration is required; Detect only ever reports one.
//
// Three sources are folded together, all freshness-bounded by MaxRevivalStaleness:
// summaries rows whose own flags are set, explicitly named roots, and trajectory
// DBs whose last step is still RUNNING. Enumeration is wider than the single
// query on purpose: a cross-account switch can leave three orphaned rows behind,
// and silently dropping two of them is the bug this replaces.
func (d *SessionDetector) DetectAll(appType string) ([]*ActiveSessionInfo, error) {
	appType = strings.ToLower(strings.TrimSpace(appType))

	base := d.BaseDir
	if base == "" {
		base = core.GetAntigravityDir()
	}

	rows, err := d.loadSummaryRows(base)
	if err != nil {
		return nil, err
	}

	baseDir := base
	dbPath := filepath.Join(base, "conversation_summaries.db")

	// Resolve every root that either carries interruption flags or was named as a
	// root explicitly, in one pass over the summaries table.
	rootOrder := make([]string, 0, len(rows))
	rootSet := make(map[string]bool)
	for convID, row := range rows {
		root := d.walkToRoot(rows, convID)
		if rootSet[root] {
			continue
		}
		rootRow, ok := rows[root]
		if !ok {
			continue
		}
		fresh := isFreshTimestamp(rootRow.rawModTime)
		flagged := rootRow.notFullyIdle != 0 || rootRow.status == "CASCADE_RUN_STATUS_RUNNING"
		named := row.notFullyIdle != 0 || row.status == "CASCADE_RUN_STATUS_RUNNING"
		if !flagged && !named && !fresh {
			continue
		}
		rootSet[root] = true
		rootOrder = append(rootOrder, root)
	}

	sort.SliceStable(rootOrder, func(i, j int) bool {
		return isNewerRow(rows[rootOrder[i]], rows[rootOrder[j]])
	})

	infos := make([]*ActiveSessionInfo, 0, len(rootOrder))
	seenRoot := make(map[string]bool, len(rootOrder))
	for _, root := range rootOrder {
		if seenRoot[root] {
			continue
		}
		info, resolveErr := d.resolveRowInfo(baseDir, dbPath, rows[root])
		if resolveErr != nil || info == nil || !info.NeedsRevival {
			continue
		}
		if !d.hasLiveInterruptionSignal(baseDir, info) {
			continue
		}
		seenRoot[root] = true
		info.TargetApp = appType
		infos = append(infos, info)
	}

	// Freshness-bounded trajectory scan: a conversation whose summaries row went
	// idle but whose trajectory still has a RUNNING step was mid-turn when the
	// process died.
	trajIDs, err := d.runningTrajectoryConversationIDs(baseDir)
	if err != nil {
		return nil, err
	}
	for _, convID := range trajIDs {
		root := d.walkToRoot(rows, convID)
		if seenRoot[root] {
			continue
		}
		seenRoot[root] = true
		info := &ActiveSessionInfo{
			ConversationID: root,
			TargetPath:     "/c/" + root,
			IsTopLevel:     true,
			NeedsRevival:   true,
			TargetApp:      appType,
		}
		if row, ok := rows[root]; ok {
			info.Title = row.title
			info.WorkspaceURI = row.workspaceURI
			info.LastModified = parseTime(row.rawModTime)
			info.Status = row.status
			info.NotFullyIdle = row.notFullyIdle != 0
			info.NestingDepth = row.depth
			info.ParentConversationID = row.parentID
		}
		infos = append(infos, info)
	}

	sort.SliceStable(infos, func(i, j int) bool {
		return infos[i].LastModified.After(infos[j].LastModified)
	})
	return infos, nil
}

// walkToRoot follows parent_conversation_id up to the owning conversation.
func (d *SessionDetector) walkToRoot(rows map[string]summaryRow, convID string) string {
	root := convID
	visited := make(map[string]bool)
	for step := 0; step < 50; step++ {
		if visited[root] {
			break
		}
		visited[root] = true
		row, ok := rows[root]
		if !ok {
			break
		}
		next := strings.TrimSpace(row.parentID)
		if next == "" || next == root {
			break
		}
		root = next
	}
	return root
}

// isFreshTimestamp reports whether a summaries timestamp is within the staleness window.
func isFreshTimestamp(raw string) bool {
	modTime := parseTime(raw)
	return !modTime.IsZero() && time.Since(modTime) <= MaxRevivalStaleness
}

// isNewerRow orders two summaries rows by last_modified_time, newest first.
func isNewerRow(a, b summaryRow) bool {
	return parseTime(a.rawModTime).After(parseTime(b.rawModTime))
}

// resolveRowInfo resolves one summary row through the shared resolution path,
// opening its own read-only connection for the sub-queries it must run.
func (d *SessionDetector) resolveRowInfo(baseDir, dbPath string, row summaryRow) (*ActiveSessionInfo, error) {
	connStr := fmt.Sprintf("file:%s?mode=ro&_busy_timeout=5000", filepath.ToSlash(dbPath))
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open conversation_summaries.db: %w", err)
	}
	defer db.Close()

	return d.resolveSessionInfo(db, filepath.Join(baseDir, "conversation_summaries.db"),
		row.convID, row.parentID, row.title, row.workspaceURI, row.depth, row.notFullyIdle, row.status, row.rawModTime)
}

// hasLiveInterruptionSignal reports whether a resolved session is backed by a
// fresh record or a running trajectory step, so stale residue is not revived.
func (d *SessionDetector) hasLiveInterruptionSignal(baseDir string, info *ActiveSessionInfo) bool {
	if info == nil {
		return false
	}
	if !info.LastModified.IsZero() && time.Since(info.LastModified) <= MaxRevivalStaleness {
		return true
	}
	if d.checkTrajectoryRunning(baseDir, info.ConversationID) {
		return true
	}
	if info.ActiveSubagentID != "" && d.checkTrajectoryRunning(baseDir, info.ActiveSubagentID) {
		return true
	}
	return false
}

// loadSummaryRows reads every conversation_summaries row into an index by ID.
func (d *SessionDetector) loadSummaryRows(baseDir string) (map[string]summaryRow, error) {
	dbPath := filepath.Join(baseDir, "conversation_summaries.db")
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return map[string]summaryRow{}, nil
		}
		return nil, fmt.Errorf("failed to stat conversation_summaries.db: %w", err)
	}

	connStr := fmt.Sprintf("file:%s?mode=ro&_busy_timeout=5000", filepath.ToSlash(dbPath))
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open conversation_summaries.db: %w", err)
	}
	defer db.Close()

	query := `SELECT conversation_id, title, workspace_uris, parent_conversation_id, nesting_depth, not_fully_idle, status, last_modified_time 
              FROM conversation_summaries;`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query conversation_summaries: %w", err)
	}
	defer rows.Close()

	index := make(map[string]summaryRow)
	for rows.Next() {
		var row summaryRow
		if err := rows.Scan(&row.convID, &row.title, &row.workspaceURI, &row.parentID, &row.depth, &row.notFullyIdle, &row.status, &row.rawModTime); err != nil {
			return nil, fmt.Errorf("failed to scan conversation summary row: %w", err)
		}
		index[row.convID] = row
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate conversation summaries: %w", err)
	}
	return index, nil
}

// runningTrajectoryConversationIDs lists conversations whose trajectory DB has a
// RUNNING (status == 2) last step and a recent mtime.
func (d *SessionDetector) runningTrajectoryConversationIDs(baseDir string) ([]string, error) {
	trajDir := filepath.Join(baseDir, "conversations")
	entries, err := os.ReadDir(trajDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read trajectory directory: %w", err)
	}

	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}
		convID := strings.TrimSuffix(entry.Name(), ".db")
		if !gui.IsValidConversationID(convID) {
			continue
		}
		if d.checkTrajectoryRunning(baseDir, convID) {
			ids = append(ids, convID)
		}
	}
	return ids, nil
}

// checkTrajectoryRunning checks if the latest step in conversations/<id>.db is status 2 (RUNNING) and recently modified.
func (d *SessionDetector) checkTrajectoryRunning(baseDir, conversationID string) bool {
	if conversationID == "" {
		return false
	}
	trajDB := filepath.Join(baseDir, "conversations", conversationID+".db")
	info, err := os.Stat(trajDB)
	if err != nil {
		return false
	}
	// Ignore stale trajectories older than MaxRevivalStaleness (crash/kill residue)
	if time.Since(info.ModTime()) > MaxRevivalStaleness {
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

	// 1. Check explicit last conversation path first (most recently recorded)
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

	// 2. Check multi-conversation layout keys deterministically
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
		sort.Strings(candidates)
		chosen := candidates[0]
		for _, c := range candidates {
			if gui.IsUUID(c) {
				chosen = c
				break
			}
		}
		return resolveSession(chosen, "/c/"+chosen), nil
	}

	// 3. Fallback to pinned_conversations_order
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

	return nil, fmt.Errorf("no conversation layout found in app_storage.json")
}
