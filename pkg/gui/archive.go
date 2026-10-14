package gui

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
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

	antiDir := core.GetAntigravityDir()
	dbPath := filepath.Join(antiDir, "conversation_summaries.db")
	annotationsDir := filepath.Join(antiDir, "annotations")

	if _, err := os.Stat(dbPath); err != nil {
		return &AutoArchiveResult{
			Success:    false,
			Message:    "Conversation summaries database not found",
			Horizon:    horizon,
			CutoffTime: cutoffStr,
		}, nil
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT conversation_id, title, last_modified_time FROM conversation_summaries WHERE last_modified_time < ?", cutoffStr)
	if err != nil {
		return nil, fmt.Errorf("failed to query conversations: %w", err)
	}
	defer rows.Close()

	var archivedIDs []string
	nowSec := time.Now().Unix()

	for rows.Next() {
		var cid string
		var title sql.NullString
		var lastMod sql.NullString
		if err := rows.Scan(&cid, &title, &lastMod); err != nil {
			continue
		}
		if strings.TrimSpace(cid) == "" {
			continue
		}

		pbtxtPath := filepath.Join(annotationsDir, fmt.Sprintf("%s.pbtxt", cid))
		alreadyArchived := false
		var existingContent string

		if data, err := os.ReadFile(pbtxtPath); err == nil {
			existingContent = string(data)
			if strings.Contains(existingContent, "archived:true") || strings.Contains(existingContent, "archived: true") {
				alreadyArchived = true
			}
		}

		if !alreadyArchived {
			_ = os.MkdirAll(annotationsDir, 0700)
			newAnnotation := fmt.Sprintf("archived:true  archival_status_timestamp:{seconds:%d  nanos:0}", nowSec)
			var updated string
			if strings.TrimSpace(existingContent) != "" {
				updated = strings.TrimSpace(existingContent) + "  " + newAnnotation + "\n"
			} else {
				titleClean := strings.ReplaceAll(title.String, "\"", "\\\"")
				updated = fmt.Sprintf("title:\"%s\"  %s\n", titleClean, newAnnotation)
			}
			if err := os.WriteFile(pbtxtPath, []byte(updated), 0600); err == nil {
				archivedIDs = append(archivedIDs, cid)
			}
		}
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
