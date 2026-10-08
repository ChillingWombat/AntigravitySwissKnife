package vault

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// VaultStatus describes the live state of the conversation vault.
type VaultStatus struct {
	Enabled          bool     `json:"enabled"`
	VaultDir         string   `json:"vault_dir"`
	ConversationsDir string   `json:"conversations_dir"`
	LiveCount        int      `json:"live_count"`
	VaultedCount     int      `json:"vaulted_count"`
	RescuedCount     int      `json:"rescued_count"`
	LastSyncTime     string   `json:"last_sync_time"`
	RescuedIDs       []string `json:"rescued_ids,omitempty"`
	Message          string   `json:"message,omitempty"`
}

// VaultSyncResult contains metrics from a vault synchronization cycle.
type VaultSyncResult struct {
	Success      bool     `json:"success"`
	NewVaulted   int      `json:"new_vaulted"`
	RescuedCount int      `json:"rescued_count"`
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
	rescuedCount     int
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
	annoDir := filepath.Join(core.GetAntigravityDir(), "annotations")
	vaultAnnoDir := core.GetAnnotationsVaultDir()

	return &Manager{
		conversationsDir: conversationsDir,
		vaultDir:         vaultDir,
		annotationsDir:   annoDir,
		vaultAnnoDir:     vaultAnnoDir,
	}
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
		LastSyncTime:     syncTimeStr,
		RescuedIDs:       m.rescuedIDs,
		Message:          fmt.Sprintf("%d live, %d vaulted, %d rescued from pruning", liveCount, vaultedCount, m.rescuedCount),
	}, nil
}

// Sync scans live conversations, vaults any unvaulted sessions using zero-overhead hardlinks,
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

	newVaulted := 0
	rescuedThisCycle := 0
	var cycleRescuedIDs []string

	// 1. Live -> Vault: Safeguard all live .db files
	liveEntries, err := os.ReadDir(m.conversationsDir)
	if err == nil {
		for _, entry := range liveEntries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".db") {
				continue
			}
			livePath := filepath.Join(m.conversationsDir, entry.Name())
			vaultPath := filepath.Join(m.vaultDir, entry.Name())

			if _, statErr := os.Stat(vaultPath); os.IsNotExist(statErr) {
				if linkErr := linkOrCopy(livePath, vaultPath); linkErr == nil {
					newVaulted++
				}
			}
		}
	}

	// 2. Vault -> Live: Auto-Restore / Rescue any files unlinked by Antigravity
	vaultEntries, err := os.ReadDir(m.vaultDir)
	if err == nil {
		for _, entry := range vaultEntries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".db") {
				continue
			}
			vaultPath := filepath.Join(m.vaultDir, entry.Name())
			livePath := filepath.Join(m.conversationsDir, entry.Name())

			if _, statErr := os.Stat(livePath); os.IsNotExist(statErr) {
				// Antigravity unlinked this file! Recreate the link back into conversations/
				if restoreErr := linkOrCopy(vaultPath, livePath); restoreErr == nil {
					rescuedThisCycle++
					cid := strings.TrimSuffix(entry.Name(), ".db")
					cycleRescuedIDs = append(cycleRescuedIDs, cid)
				}
			}
		}
	}

	// 3. Annotations Vaulting & Restoration (pbtxt)
	if annoEntries, err := os.ReadDir(m.annotationsDir); err == nil {
		for _, a := range annoEntries {
			if !a.IsDir() && strings.HasSuffix(a.Name(), ".pbtxt") {
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
				vaultP := filepath.Join(m.vaultAnnoDir, a.Name())
				liveP := filepath.Join(m.annotationsDir, a.Name())
				if _, statErr := os.Stat(liveP); os.IsNotExist(statErr) {
					_ = linkOrCopy(vaultP, liveP)
				}
			}
		}
	}

	m.rescuedCount += rescuedThisCycle
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

	msg := fmt.Sprintf("Vault synced: %d newly safeguarded, %d rescued from Antigravity pruning (total vaulted: %d)", newVaulted, rescuedThisCycle, totalVaulted)
	return &VaultSyncResult{
		Success:      true,
		NewVaulted:   newVaulted,
		RescuedCount: rescuedThisCycle,
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
