package cache

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// PruneOptions specifies rules and safety constraints for cache cleanup.
type PruneOptions struct {
	MinAgeDays      float64 `json:"min_age_days"`
	OlderThanDays   float64 `json:"older_than_days,omitempty"`
	MaxSizeGB       float64 `json:"max_size_gb,omitempty"`
	PruneScratch    bool    `json:"prune_scratch"`
	PruneSteps      bool    `json:"prune_steps"`
	PruneTasks      bool    `json:"prune_tasks"`
	DryRun          bool    `json:"dry_run"`
	ActiveCascadeID string  `json:"active_cascade_id"`
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
		cutoff = time.Now().Add(-time.Duration(opts.MinAgeDays*24) * time.Hour)
	}

	var totalRemainingBytes int64
	var remainingPrunable []prunableFile

	err := filepath.Walk(p.BrainDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		size := info.Size()
		rel, _ := filepath.Rel(p.BrainDir, path)

		// Active Conversation Shield Check:
		// If the conversation path contains the active cascade ID, shield it!
		if opts.ActiveCascadeID != "" && strings.Contains(rel, opts.ActiveCascadeID) {
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
						res.Errors = append(res.Errors, err.Error())
						totalRemainingBytes += size
						return nil
					}
				}
				res.BytesReclaimed += size
				res.FilesDeleted++
				return nil
			}
			remainingPrunable = append(remainingPrunable, prunableFile{
				path:    path,
				size:    size,
				modTime: info.ModTime(),
			})
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
	if opts.MaxSizeGB > 0 {
		maxBytes := int64(opts.MaxSizeGB * 1024 * 1024 * 1024)
		if totalRemainingBytes > maxBytes && len(remainingPrunable) > 0 {
			sort.Slice(remainingPrunable, func(i, j int) bool {
				return remainingPrunable[i].modTime.Before(remainingPrunable[j].modTime)
			})
			for _, f := range remainingPrunable {
				if totalRemainingBytes <= maxBytes {
					break
				}
				if !opts.DryRun {
					if err := os.Remove(f.path); err != nil {
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
