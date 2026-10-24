package cache

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	_ "modernc.org/sqlite"
)

// DefaultMaxSizeMinAgeDuration is the safety floor: files younger than 24 hours
// are never pruned during size-based capacity reduction to safeguard active sessions.
const DefaultMaxSizeMinAgeDuration = 24 * time.Hour

const activeCascadeStaleness = 2 * time.Hour

// PruneOptions specifies rules and safety constraints for cache cleanup.
type PruneOptions struct {
	MinAgeDays       float64  `json:"min_age_days"`
	OlderThanDays    float64  `json:"older_than_days,omitempty"`
	MaxSizeGB        float64  `json:"max_size_gb,omitempty"`
	PruneScratch     bool     `json:"prune_scratch"`
	PruneSteps       bool     `json:"prune_steps"`
	PruneTasks       bool     `json:"prune_tasks"`
	DryRun           bool     `json:"dry_run"`
	ActiveCascadeID  string   `json:"active_cascade_id"`
	ActiveCascadeIDs []string `json:"active_cascade_ids,omitempty"`
}

// PruneResult summarizes the space and files reclaimed.
type PruneResult struct {
	BytesReclaimed int64    `json:"bytes_reclaimed"`
	FreedBytes     int64    `json:"freed_bytes"`
	FilesDeleted   int      `json:"files_deleted"`
	DeletedFiles   int      `json:"deleted_files"`
	SkippedShield  int      `json:"skipped_shield"`
	Errors         []string `json:"errors"`
}

// Pruner executes safe disk reclamation.
type Pruner struct {
	BrainDir string
}

// NewPruner creates a Pruner.
func NewPruner(brainDir string) *Pruner {
	if brainDir == "" {
		brainDir = core.GetBrainDir()
	}
	return &Pruner{BrainDir: brainDir}
}

type prunableFile struct {
	path    string
	size    int64
	modTime time.Time
}

// Prune cleans stale files according to PruneOptions while strictly respecting the conversation shield.
func (p *Pruner) Prune(opts PruneOptions) (*PruneResult, error) {
	res := &PruneResult{}
	brainInfo, err := os.Stat(p.BrainDir)
	if err != nil {
		if os.IsNotExist(err) {
			return res, nil
		}
		return res, err
	}
	if !brainInfo.IsDir() {
		return res, fmt.Errorf("brain cache path is not a directory: %s", p.BrainDir)
	}

	lockPath := filepath.Join(p.BrainDir, ".prune.lock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return res, fmt.Errorf("open prune lock: %w", err)
	}
	if err := core.LockFile(lockFile); err != nil {
		_ = lockFile.Close()
		return res, fmt.Errorf("acquire prune lock: %w", err)
	}
	defer func() {
		_ = core.UnlockFile(lockFile)
		_ = lockFile.Close()
	}()

	activeIDs, err := DiscoverActiveCascadeIDs(p.BrainDir)
	if err != nil {
		return res, fmt.Errorf("discover active cascades: %w", err)
	}
	if len(activeIDs) > 0 {
		seenIDs := make(map[string]bool, len(opts.ActiveCascadeIDs)+len(activeIDs))
		for _, id := range opts.ActiveCascadeIDs {
			seenIDs[id] = true
		}
		for _, id := range activeIDs {
			if !seenIDs[id] {
				seenIDs[id] = true
				opts.ActiveCascadeIDs = append(opts.ActiveCascadeIDs, id)
			}
		}
	}

	if opts.MinAgeDays == 0 && opts.OlderThanDays > 0 {
		opts.MinAgeDays = opts.OlderThanDays
	}
	if !opts.PruneScratch && !opts.PruneSteps && !opts.PruneTasks {
		opts.PruneScratch = true
		opts.PruneSteps = true
		opts.PruneTasks = true
	}

	hasAgeCutoff := opts.MinAgeDays > 0
	var cutoff time.Time
	if hasAgeCutoff {
		cutoff = time.Now().Add(-time.Duration(opts.MinAgeDays * float64(24*time.Hour)))
	}

	var totalRemainingBytes int64
	var remainingPrunable []prunableFile

	err = filepath.Walk(p.BrainDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if path == lockPath {
			return nil
		}

		size := info.Size()
		rel, _ := filepath.Rel(p.BrainDir, path)

		// Active Conversation Shield Check:
		// If the conversation path contains any active cascade ID, shield it!
		if isShielded(rel, opts) {
			res.SkippedShield++
			totalRemainingBytes += size
			return nil
		}

		shouldPrune := false
		if opts.PruneScratch && containsPathSegment(rel, "scratch") {
			shouldPrune = true
		} else if opts.PruneSteps && containsPathSegment(rel, "steps") {
			shouldPrune = true
		} else if opts.PruneTasks && containsPathSegment(rel, "tasks") {
			shouldPrune = true
		}

		if shouldPrune {
			if hasAgeCutoff && info.ModTime().Before(cutoff) {
				if !opts.DryRun {
					if err := os.Remove(path); err != nil {
						if isSharingViolation(err) {
							totalRemainingBytes += size
							return nil
						}
						res.Errors = append(res.Errors, err.Error())
						totalRemainingBytes += size
						return nil
					}
				}
				res.BytesReclaimed += size
				res.FilesDeleted++
				return nil
			}
			// When MaxSizeGB is configured, collect unpruned files for capacity-based pruning
			if opts.MaxSizeGB > 0 {
				remainingPrunable = append(remainingPrunable, prunableFile{
					path:    path,
					size:    size,
					modTime: info.ModTime(),
				})
			}
		}
		totalRemainingBytes += size
		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		res.FreedBytes = res.BytesReclaimed
		res.DeletedFiles = res.FilesDeleted
		return res, err
	}

	// Enforce MaxSizeGB cap if configured
	if opts.MaxSizeGB > 0 && len(remainingPrunable) > 0 {
		maxBytes := int64(opts.MaxSizeGB * 1024 * 1024 * 1024)
		if totalRemainingBytes > maxBytes {
			sort.Slice(remainingPrunable, func(i, j int) bool {
				return remainingPrunable[i].modTime.Before(remainingPrunable[j].modTime)
			})
			for _, f := range remainingPrunable {
				if totalRemainingBytes <= maxBytes {
					break
				}
				// 24-hour minimum age floor: NEVER prune files modified within the last 24 hours!
				if time.Since(f.modTime) < DefaultMaxSizeMinAgeDuration {
					continue
				}
				// When MinAgeDays cutoff is also set, never prune files younger than cutoff
				if hasAgeCutoff && f.modTime.After(cutoff) {
					continue
				}
				if !opts.DryRun {
					if err := os.Remove(f.path); err != nil {
						if isSharingViolation(err) {
							continue
						}
						res.Errors = append(res.Errors, err.Error())
						continue
					}
				}
				res.BytesReclaimed += f.size
				res.FilesDeleted++
				totalRemainingBytes -= f.size
			}
		}
	}

	res.FreedBytes = res.BytesReclaimed
	res.DeletedFiles = res.FilesDeleted
	return res, nil
}

// DiscoverActiveCascadeIDs returns fresh conversations that are running or not fully idle.
func DiscoverActiveCascadeIDs(brainDir string) ([]string, error) {
	if brainDir == "" {
		brainDir = core.GetBrainDir()
	}

	candidates := []string{
		filepath.Join(filepath.Dir(brainDir), "conversation_summaries.db"),
		filepath.Join(brainDir, "conversation_summaries.db"),
	}
	dbPath := ""
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			dbPath = candidate
			break
		}
	}
	if dbPath == "" {
		return nil, nil
	}

	connStr := fmt.Sprintf("file:%s?mode=ro&_busy_timeout=5000", filepath.ToSlash(dbPath))
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("open conversation summaries: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT conversation_id, COALESCE(last_modified_time, '')
		FROM conversation_summaries
		WHERE TRIM(COALESCE(conversation_id, '')) != ''
		  AND (
			COALESCE(not_fully_idle, 0) != 0
			OR UPPER(COALESCE(status, '')) LIKE '%RUNNING%'
		  )`)
	if err != nil {
		return nil, fmt.Errorf("query active conversations: %w", err)
	}
	defer rows.Close()

	seen := make(map[string]bool)
	var activeIDs []string
	for rows.Next() {
		var id, modified string
		if err := rows.Scan(&id, &modified); err != nil {
			return nil, fmt.Errorf("scan active conversation: %w", err)
		}
		id = strings.TrimSpace(strings.TrimPrefix(id, "/c/"))
		if id == "" {
			continue
		}
		modifiedAt := parseConversationTime(modified)
		if !modifiedAt.IsZero() && time.Since(modifiedAt) > activeCascadeStaleness {
			continue
		}
		if !seen[id] {
			seen[id] = true
			activeIDs = append(activeIDs, id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active conversations: %w", err)
	}

	sort.Strings(activeIDs)
	return activeIDs, nil
}

func parseConversationTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

// isShielded checks whether a relative cache path belongs to any active cascade session.
func isShielded(rel string, opts PruneOptions) bool {
	if opts.ActiveCascadeID != "" {
		if containsPathSegment(rel, opts.ActiveCascadeID) || strings.Contains(rel, opts.ActiveCascadeID) {
			return true
		}
	}
	for _, id := range opts.ActiveCascadeIDs {
		if id != "" && (containsPathSegment(rel, id) || strings.Contains(rel, id)) {
			return true
		}
	}
	return false
}

// isSharingViolation checks if an OS error is a Windows file lock or sharing collision.
func isSharingViolation(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "used by another process") ||
		strings.Contains(errStr, "sharing violation") ||
		strings.Contains(errStr, "access is denied")
}
