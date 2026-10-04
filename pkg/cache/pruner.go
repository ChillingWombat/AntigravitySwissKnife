package cache

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// PruneOptions specifies rules and safety constraints for cache cleanup.
type PruneOptions struct {
	MinAgeDays      float64 `json:"min_age_days"`
	PruneScratch    bool    `json:"prune_scratch"`
	PruneSteps      bool    `json:"prune_steps"`
	PruneTasks      bool    `json:"prune_tasks"`
	DryRun          bool    `json:"dry_run"`
	ActiveCascadeID string  `json:"active_cascade_id"`
}

// PruneResult summarizes the space and files reclaimed.
type PruneResult struct {
	BytesReclaimed int64    `json:"bytes_reclaimed"`
	FilesDeleted   int      `json:"files_deleted"`
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

// Prune cleans stale files according to PruneOptions while strictly respecting the conversation shield.
func (p *Pruner) Prune(opts PruneOptions) (*PruneResult, error) {
	res := &PruneResult{}
	cutoff := time.Now().Add(-time.Duration(opts.MinAgeDays*24) * time.Hour)

	err := filepath.Walk(p.BrainDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		rel, _ := filepath.Rel(p.BrainDir, path)

		// Active Conversation Shield Check:
		// If the conversation path contains the active cascade ID, shield it!
		if opts.ActiveCascadeID != "" && strings.Contains(rel, opts.ActiveCascadeID) {
			res.SkippedShield++
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

		if shouldPrune && info.ModTime().Before(cutoff) {
			size := info.Size()
			if !opts.DryRun {
				if err := os.Remove(path); err != nil {
					res.Errors = append(res.Errors, err.Error())
					return nil
				}
			}
			res.BytesReclaimed += size
			res.FilesDeleted++
		}
		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return res, err
	}
	return res, nil
}
