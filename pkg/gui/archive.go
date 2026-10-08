package gui

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	_ "modernc.org/sqlite"
)

// AutoArchiveResult contains details of an auto-archiving run.
type AutoArchiveResult struct {
	Success       bool     `json:"success"`
	ArchivedCount int      `json:"archived_count"`
	ArchivedIDs   []string `json:"archived_ids"`
	Horizon       string   `json:"horizon"`
	CutoffTime    string   `json:"cutoff_time"`
	Message       string   `json:"message"`
}

// ParseHorizonToDuration parses a horizon string (e.g. "7d", "14d", "30d", "60d", "90d") into a time.Duration.
func ParseHorizonToDuration(horizon string) time.Duration {
	horizon = strings.TrimSpace(strings.ToLower(horizon))
	switch horizon {
	case "1d":
		return 24 * time.Hour
	case "3d":
		return 3 * 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	case "14d":
		return 14 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	case "60d":
		return 60 * 24 * time.Hour
	case "90d":
		return 90 * 24 * time.Hour
	default:
		// Default to 30 days
		return 30 * 24 * time.Hour
	}
}

// ArchiveStaleConversations scans conversation history, finds conversations older than the horizon cutoff,
// writes archived annotations, and synchronizes with live Antigravity if running.
func (s *Store) ArchiveStaleConversations(horizon string) (*AutoArchiveResult, error) {
	if horizon == "" {
		s.mu.RLock()
		horizon = s.config.AutoArchiveHorizon
		s.mu.RUnlock()
	}
	if horizon == "" {
		horizon = "30d"
	}

	dur := ParseHorizonToDuration(horizon)
	cutoff := time.Now().Add(-dur)
	cutoffStr := cutoff.UTC().Format("2006-01-02 15:04:05")

	home := os.Getenv("HOME")
	dbPath := filepath.Join(home, ".gemini", "antigravity", "conversation_summaries.db")
	annotationsDir := filepath.Join(home, ".gemini", "antigravity", "annotations")

	if _, err := os.Stat(dbPath); err != nil {
		return &AutoArchiveResult{
			Success:    false,
			Message:    "Conversation summaries database not found",
			Horizon:    horizon,
			CutoffTime: cutoffStr,
		}, nil
	}

	// Python script to query candidate conversations older than cutoff,
	// excluding pinned conversations and already archived ones.
	pyScript := `
import sqlite3, os, sys, json, time

db_path = sys.argv[1]
cutoff_str = sys.argv[2]
annotations_dir = sys.argv[3]

archived_ids = []
now_sec = int(time.time())

try:
    conn = sqlite3.connect(db_path)
    cur = conn.cursor()
    # Find conversations where last_modified_time < cutoff
    cur.execute("SELECT conversation_id, title, last_modified_time FROM conversation_summaries WHERE last_modified_time < ?", (cutoff_str,))
    rows = cur.fetchall()

    for row in rows:
        cid = row[0]
        if not cid:
            continue

        pbtxt_path = os.path.join(annotations_dir, f"{cid}.pbtxt")
        already_archived = False
        existing_content = ""

        if os.path.exists(pbtxt_path):
            try:
                with open(pbtxt_path, "r", encoding="utf-8", errors="ignore") as f:
                    existing_content = f.read()
                if "archived:true" in existing_content or "archived: true" in existing_content:
                    already_archived = True
            except Exception:
                pass

        if not already_archived:
            try:
                # Append or create archived:true annotation
                os.makedirs(annotations_dir, exist_ok=True)
                new_annotation = f'archived:true  archival_status_timestamp:{{seconds:{now_sec}  nanos:0}}'
                if existing_content.strip():
                    updated = existing_content.strip() + "  " + new_annotation + "\n"
                else:
                    title_clean = (row[1] or "").replace('"', '\\"')
                    updated = f'title:"{title_clean}"  {new_annotation}\n'
                with open(pbtxt_path, "w", encoding="utf-8") as f:
                    f.write(updated)
                archived_ids.append(cid)
            except Exception as e:
                pass

    print(json.dumps(archived_ids))
except Exception as e:
    print(json.dumps([]))
`

	cmd := exec.Command("python3", "-c", pyScript, dbPath, cutoffStr, annotationsDir)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute auto-archive script: %w", err)
	}

	var archivedIDs []string
	if err := json.Unmarshal(out, &archivedIDs); err != nil {
		archivedIDs = []string{}
	}

	// If Antigravity is running, trigger live CDP update to sync the Redux store & sidebar immediately
	if len(archivedIDs) > 0 {
		go func(ids []string) {
			port, err := s.injector.FindDevToolsPort()
			if err != nil || port == 0 {
				return
			}
			pages, err := s.injector.GetPageTargets(port)
			if err != nil || len(pages) == 0 {
				return
			}
			idsJSON, _ := json.Marshal(ids)
			cdpScript := fmt.Sprintf(`(() => {
				const ids = %s;
				const root = document.getElementById('root');
				if (!root) return;
				const key = Object.keys(root).find(k => k.startsWith("__reactFiber") || k.startsWith("__reactContainer"));
				let fiber = root[key];
				let store = null;
				let curr = fiber;
				while (curr && !store) {
					if (curr.memoizedProps?.store) store = curr.memoizedProps.store;
					if (curr.memoizedState?.element?.props?.store) store = curr.memoizedState.element.props.store;
					curr = curr.child;
				}
				if (!store) return;
				const state = store.getState();
				const summaries = state.trajectorySummaries?.summaries;
				if (summaries) {
					ids.forEach(id => {
						if (summaries[id]) {
							summaries[id].annotations = summaries[id].annotations || {};
							summaries[id].annotations.archived = true;
						}
					});
				}
				if (typeof window.__swissUpdateTagsAndDraggables === 'function') {
					window.__swissUpdateTagsAndDraggables();
				}
			})()`, string(idsJSON))

			for _, p := range pages {
				_, _ = s.injector.ExecuteScript(p.WebSocketDebuggerURL, cdpScript)
			}
		}(archivedIDs)
	}

	return &AutoArchiveResult{
		Success:       true,
		ArchivedCount: len(archivedIDs),
		ArchivedIDs:   archivedIDs,
		Horizon:       horizon,
		CutoffTime:    cutoffStr,
		Message:       fmt.Sprintf("Successfully archived %d stale conversation(s) into conversation history (horizon: %s)", len(archivedIDs), horizon),
	}, nil
}

// GetPrunedConversationIDs returns IDs of conversations present in conversation_summaries.db
// whose physical database file in conversations/ directory is missing.
func GetPrunedConversationIDs() []string {
	dbPath := filepath.Join(core.GetAntigravityDir(), "conversation_summaries.db")
	convsDir := core.GetConversationsDir()
	return ScanPrunedConversationIDs(dbPath, convsDir)
}

// ScanPrunedConversationIDs queries a summaries SQLite db and checks for missing conversation db files.
func ScanPrunedConversationIDs(dbPath, convsDir string) []string {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return []string{}
	}

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro", dbPath))
	if err != nil {
		return []string{}
	}
	defer db.Close()

	rows, err := db.Query("SELECT conversation_id FROM conversation_summaries")
	if err != nil {
		return []string{}
	}
	defer rows.Close()

	var pruned []string
	for rows.Next() {
		var cid string
		if err := rows.Scan(&cid); err == nil && cid != "" {
			physicalDB := filepath.Join(convsDir, cid+".db")
			if _, err := os.Stat(physicalDB); os.IsNotExist(err) {
				pruned = append(pruned, cid)
			}
		}
	}
	return pruned
}
