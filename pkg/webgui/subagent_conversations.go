package webgui

import (
	"database/sql"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type subagentConversationCacheEntry struct {
	isSubagent bool
	checkedAt  time.Time
}

var subagentConversationCache = struct {
	sync.Mutex
	entries map[string]subagentConversationCacheEntry
}{entries: make(map[string]subagentConversationCacheEntry)}

// isSubagentConversation reports whether convID is a subagent conversation.
// Positive results are cached longer than negative ones so a newly created
// subagent is not mistaken for the main conversation on its first request.
func (s *Server) isSubagentConversation(convID string) bool {
	convID = strings.TrimSpace(convID)
	if convID == "" {
		return false
	}
	dataDir := s.getAntigravityDataDir()
	cacheKey := dataDir + "|" + convID
	now := time.Now()
	subagentConversationCache.Lock()
	if entry, ok := subagentConversationCache.entries[cacheKey]; ok {
		ttl := 15 * time.Second
		if entry.isSubagent {
			ttl = 5 * time.Minute
		}
		if now.Sub(entry.checkedAt) < ttl {
			subagentConversationCache.Unlock()
			return entry.isSubagent
		}
	}
	subagentConversationCache.Unlock()

	isSubagent := querySubagentConversation(dataDir, convID)

	subagentConversationCache.Lock()
	if len(subagentConversationCache.entries) >= 4096 {
		subagentConversationCache.entries = make(map[string]subagentConversationCacheEntry)
	}
	subagentConversationCache.entries[cacheKey] = subagentConversationCacheEntry{
		isSubagent: isSubagent,
		checkedAt:  now,
	}
	subagentConversationCache.Unlock()
	return isSubagent
}

func querySubagentConversation(dataDir, convID string) bool {
	dbPath := filepath.Join(dataDir, "conversation_summaries.db")
	db, err := sql.Open("sqlite", dbPath+"?mode=ro&_busy_timeout=1000")
	if err != nil {
		return false
	}
	defer db.Close()

	var found int
	err = db.QueryRow(`
		SELECT 1 FROM conversation_summaries
		WHERE conversation_id = ?
		  AND (COALESCE(parent_conversation_id, '') <> '' OR nesting_depth > 0 OR COALESCE(agent_name, '') <> '')
		LIMIT 1`, convID).Scan(&found)
	return err == nil
}
