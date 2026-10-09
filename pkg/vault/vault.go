package vault

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	_ "modernc.org/sqlite"
)

// VaultStatus describes the live state of the conversation vault.
type VaultStatus struct {
	Enabled          bool     `json:"enabled"`
	VaultDir         string   `json:"vault_dir"`
	ConversationsDir string   `json:"conversations_dir"`
	LiveCount        int      `json:"live_count"`
	VaultedCount     int      `json:"vaulted_count"`
	RescuedCount     int      `json:"rescued_count"`
	DeletedCount     int      `json:"deleted_count,omitempty"`
	LastSyncTime     string   `json:"last_sync_time"`
	RescuedIDs       []string `json:"rescued_ids,omitempty"`
	Message          string   `json:"message,omitempty"`
}

// VaultSyncResult contains metrics from a vault synchronization cycle.
type VaultSyncResult struct {
	Success      bool     `json:"success"`
	NewVaulted   int      `json:"new_vaulted"`
	RescuedCount int      `json:"rescued_count"`
	DeletedCount int      `json:"deleted_count"`
	RescuedIDs   []string `json:"rescued_ids"`
	TotalLive    int      `json:"total_live"`
	TotalVaulted int      `json:"total_vaulted"`
	Message      string   `json:"message"`
}

// Manager coordinates conversation vaulting and auto-shield recovery.
type Manager struct {
	mu               sync.RWMutex
	conversationsDir string
	vaultDir         string
	annotationsDir   string
	vaultAnnoDir     string
	summariesDBPath  string
	rescuedCount     int
	deletedCount     int
	lastSyncTime     time.Time
	rescuedIDs       []string
}

// NewManager creates a vault Manager with configured or default paths.
func NewManager(conversationsDir, vaultDir string) *Manager {
	if conversationsDir == "" {
		conversationsDir = core.GetConversationsDir()
	}
	if vaultDir == "" {
		vaultDir = core.GetConversationVaultDir()
	}
	baseDir := filepath.Dir(conversationsDir)
	annoDir := filepath.Join(baseDir, "annotations")
	vaultAnnoDir := core.GetAnnotationsVaultDir()
	summariesDBPath := filepath.Join(baseDir, "conversation_summaries.db")

	return &Manager{
		conversationsDir: conversationsDir,
		vaultDir:         vaultDir,
		annotationsDir:   annoDir,
		vaultAnnoDir:     vaultAnnoDir,
		summariesDBPath:  summariesDBPath,
	}
}

// loadSummaryConversationIDs reads conversation_summaries.db if present and returns the set of active conversation IDs.
func (m *Manager) loadSummaryConversationIDs() (map[string]bool, bool) {
	dbPath := m.summariesDBPath
	if dbPath == "" && m.conversationsDir != "" {
		dbPath = filepath.Join(filepath.Dir(m.conversationsDir), "conversation_summaries.db")
	}
	if dbPath == "" {
		return nil, false
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, false
	}

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro", dbPath))
	if err != nil {
		return nil, false
	}
	defer db.Close()

	rows, err := db.Query("SELECT conversation_id FROM conversation_summaries")
	if err != nil {
		return nil, false
	}
	defer rows.Close()

	ids := make(map[string]bool)
	for rows.Next() {
		var cid string
		if err := rows.Scan(&cid); err == nil && cid != "" {
			ids[cid] = true
		}
	}
	return ids, true
}

// GetStatus computes current statistics about vaulted and live conversations.
func (m *Manager) GetStatus(enabled bool) (*VaultStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	liveCount := 0
	if entries, err := os.ReadDir(m.conversationsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".db") {
				liveCount++
			}
		}
	}

	vaultedCount := 0
	if entries, err := os.ReadDir(m.vaultDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".db") {
				vaultedCount++
			}
		}
	}

	syncTimeStr := ""
	if !m.lastSyncTime.IsZero() {
		syncTimeStr = m.lastSyncTime.UTC().Format(time.RFC3339)
	}

	return &VaultStatus{
		Enabled:          enabled,
		VaultDir:         m.vaultDir,
		ConversationsDir: m.conversationsDir,
		LiveCount:        liveCount,
		VaultedCount:     vaultedCount,
		RescuedCount:     m.rescuedCount,
		DeletedCount:     m.deletedCount,
		LastSyncTime:     syncTimeStr,
		RescuedIDs:       m.rescuedIDs,
		Message:          fmt.Sprintf("%d live, %d vaulted, %d rescued from pruning", liveCount, vaultedCount, m.rescuedCount),
	}, nil
}

// Sync scans live conversations, vaults any unvaulted sessions using zero-overhead hardlinks,
// removes sessions from the vault that were manually deleted in Antigravity,
// and auto-restores (rescues) any sessions unlinked by Antigravity's 500-session limit.
func (m *Manager) Sync() (*VaultSyncResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.vaultDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create vault dir: %w", err)
	}
	if err := os.MkdirAll(m.conversationsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to ensure conversations dir: %w", err)
	}
	if err := os.MkdirAll(m.vaultAnnoDir, 0755); err != nil {
		_ = err
	}

	summaryIDs, hasSummaryDB := m.loadSummaryConversationIDs()

	newVaulted := 0
	rescuedThisCycle := 0
	deletedThisCycle := 0
	var cycleRescuedIDs []string

	// 1. Live -> Vault: Safeguard live .db files that have not been manually deleted
	liveEntries, err := os.ReadDir(m.conversationsDir)
	if err == nil {
		for _, entry := range liveEntries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".db") {
				continue
			}
			cid := strings.TrimSuffix(entry.Name(), ".db")
			livePath := filepath.Join(m.conversationsDir, entry.Name())
			vaultPath := filepath.Join(m.vaultDir, entry.Name())

			if hasSummaryDB && !summaryIDs[cid] {
				// If a file is older than 60s and absent from conversation_summaries.db, it was manually deleted
				if info, infoErr := entry.Info(); infoErr == nil && time.Since(info.ModTime()) > 60*time.Second {
					continue
				}
			}

			if _, statErr := os.Stat(vaultPath); os.IsNotExist(statErr) {
				if linkErr := linkOrCopy(livePath, vaultPath); linkErr == nil {
					newVaulted++
				}
			}
		}
	}

	// 2. Vault -> Live: Purge manually deleted conversations from Vault, or Auto-Restore sessions unlinked by Antigravity's 500-session limit
	vaultEntries, err := os.ReadDir(m.vaultDir)
	if err == nil {
		for _, entry := range vaultEntries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".db") {
				continue
			}
			cid := strings.TrimSuffix(entry.Name(), ".db")
			vaultPath := filepath.Join(m.vaultDir, entry.Name())
			livePath := filepath.Join(m.conversationsDir, entry.Name())

			// If conversation_summaries.db exists and cid is no longer in conversation_summaries,
			// the user manually deleted this conversation in Antigravity -> delete from vault too.
			if hasSummaryDB && !summaryIDs[cid] {
				isBrandNew := false
				if liveInfo, statErr := os.Stat(livePath); statErr == nil && time.Since(liveInfo.ModTime()) <= 60*time.Second {
					isBrandNew = true
				}
				if !isBrandNew {
					if rmErr := os.Remove(vaultPath); rmErr == nil {
						deletedThisCycle++
					}
					_ = os.Remove(livePath)
					_ = os.Remove(filepath.Join(m.vaultAnnoDir, cid+".pbtxt"))
					_ = os.Remove(filepath.Join(m.annotationsDir, cid+".pbtxt"))
					continue
				}
			}

			if _, statErr := os.Stat(livePath); os.IsNotExist(statErr) {
				// Antigravity's 500-session limit unlinked this file while it remains in summaries! Recreate the link back into conversations/
				if restoreErr := linkOrCopy(vaultPath, livePath); restoreErr == nil {
					rescuedThisCycle++
					cycleRescuedIDs = append(cycleRescuedIDs, cid)
				}
			}
		}
	}

	// 3. Annotations Vaulting & Restoration (pbtxt)
	if annoEntries, err := os.ReadDir(m.annotationsDir); err == nil {
		for _, a := range annoEntries {
			if !a.IsDir() && strings.HasSuffix(a.Name(), ".pbtxt") {
				cid := strings.TrimSuffix(a.Name(), ".pbtxt")
				if hasSummaryDB && !summaryIDs[cid] {
					continue
				}
				liveP := filepath.Join(m.annotationsDir, a.Name())
				vaultP := filepath.Join(m.vaultAnnoDir, a.Name())
				if _, statErr := os.Stat(vaultP); os.IsNotExist(statErr) {
					_ = linkOrCopy(liveP, vaultP)
				}
			}
		}
	}
	if vAnnoEntries, err := os.ReadDir(m.vaultAnnoDir); err == nil {
		for _, a := range vAnnoEntries {
			if !a.IsDir() && strings.HasSuffix(a.Name(), ".pbtxt") {
				cid := strings.TrimSuffix(a.Name(), ".pbtxt")
				vaultP := filepath.Join(m.vaultAnnoDir, a.Name())
				liveP := filepath.Join(m.annotationsDir, a.Name())
				if hasSummaryDB && !summaryIDs[cid] {
					_ = os.Remove(vaultP)
					continue
				}
				if _, statErr := os.Stat(liveP); os.IsNotExist(statErr) {
					_ = linkOrCopy(vaultP, liveP)
				}
			}
		}
	}

	m.rescuedCount += rescuedThisCycle
	m.deletedCount += deletedThisCycle
	if len(cycleRescuedIDs) > 0 {
		m.rescuedIDs = append(m.rescuedIDs, cycleRescuedIDs...)
	}
	m.lastSyncTime = time.Now()

	// Recount totals
	totalLive := 0
	if entries, err := os.ReadDir(m.conversationsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".db") {
				totalLive++
			}
		}
	}
	totalVaulted := 0
	if entries, err := os.ReadDir(m.vaultDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".db") {
				totalVaulted++
			}
		}
	}

	msg := fmt.Sprintf("Vault scanned: %d newly safeguarded, %d rescued from 500-session pruning, %d deleted sessions purged (total vaulted: %d)", newVaulted, rescuedThisCycle, deletedThisCycle, totalVaulted)
	return &VaultSyncResult{
		Success:      true,
		NewVaulted:   newVaulted,
		RescuedCount: rescuedThisCycle,
		DeletedCount: deletedThisCycle,
		RescuedIDs:   cycleRescuedIDs,
		TotalLive:    totalLive,
		TotalVaulted: totalVaulted,
		Message:      msg,
	}, nil
}

// RestoreConversation restores a single conversation from the vault if missing from live directory.
func (m *Manager) RestoreConversation(conversationID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cid := strings.TrimSuffix(conversationID, ".db")
	vaultPath := filepath.Join(m.vaultDir, cid+".db")
	livePath := filepath.Join(m.conversationsDir, cid+".db")

	if _, err := os.Stat(vaultPath); os.IsNotExist(err) {
		return false, fmt.Errorf("conversation %s not present in vault", cid)
	}

	if _, err := os.Stat(livePath); err == nil {
		return false, nil // Already exists
	}

	if err := linkOrCopy(vaultPath, livePath); err != nil {
		return false, fmt.Errorf("failed to restore conversation: %w", err)
	}

	m.rescuedCount++
	m.rescuedIDs = append(m.rescuedIDs, cid)
	return true, nil
}

// VaultConversation vaults a single conversation immediately.
func (m *Manager) VaultConversation(conversationID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cid := strings.TrimSuffix(conversationID, ".db")
	livePath := filepath.Join(m.conversationsDir, cid+".db")
	vaultPath := filepath.Join(m.vaultDir, cid+".db")

	if _, err := os.Stat(livePath); os.IsNotExist(err) {
		return false, fmt.Errorf("live conversation %s does not exist", cid)
	}

	if _, err := os.Stat(vaultPath); err == nil {
		return false, nil // Already vaulted
	}

	if err := os.MkdirAll(m.vaultDir, 0755); err != nil {
		return false, err
	}

	if err := linkOrCopy(livePath, vaultPath); err != nil {
		return false, err
	}

	return true, nil
}

// linkOrCopy attempts a zero-overhead hard link; if unsupported (e.g. cross-device link), falls back to copying.
func linkOrCopy(src, dst string) error {
	// Remove dst if it exists as an empty or stale file
	_ = os.Remove(dst)

	// Attempt native filesystem hardlink (works on Linux, macOS APFS, Windows NTFS)
	if err := os.Link(src, dst); err == nil {
		return nil
	}

	// Fallback to byte copy if hardlinks fail across filesystems
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return dstFile.Sync()
}
