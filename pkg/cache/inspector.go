package cache

import (
	"os"
	"path/filepath"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// CategoryStats stores disk metrics for a specific cache category.
type CategoryStats struct {
	Category    string `json:"category"`
	TotalBytes  int64  `json:"total_bytes"`
	FileCount   int    `json:"file_count"`
	Description string `json:"description"`
}

// Breakdown holds overall disk usage metrics across ~/.gemini/antigravity.
type Breakdown struct {
	BrainTotalBytes         int64                    `json:"brain_total_bytes"`
	ConversationsTotalBytes int64                    `json:"conversations_total_bytes"`
	ReclaimableBytes        int64                    `json:"reclaimable_bytes"`
	Categories              map[string]CategoryStats `json:"categories"`
}

// Inspector scans cache directories and produces metrics.
type Inspector struct {
	BrainDir         string
	ConversationsDir string
}

// NewInspector creates an Inspector targeting default or custom directories.
func NewInspector(brainDir, conversationsDir string) *Inspector {
	if brainDir == "" {
		brainDir = core.GetBrainDir()
	}
	if conversationsDir == "" {
		conversationsDir = core.GetConversationsDir()
	}
	return &Inspector{
		BrainDir:         brainDir,
		ConversationsDir: conversationsDir,
	}
}

// ScanBreakdown walks the brain and conversations directories to calculate metrics.
func (ins *Inspector) ScanBreakdown(minAgeDays float64) (*Breakdown, error) {
	bd := &Breakdown{
		Categories: make(map[string]CategoryStats),
	}

	cutoff := time.Now().Add(-time.Duration(minAgeDays*24) * time.Hour)

	// Categories to track
	categories := map[string]*CategoryStats{
		"scratch":       {Category: "scratch", Description: "Agent scratchpads & temporary files"},
		"steps":         {Category: "steps", Description: "Agent execution step outputs"},
		"tasks":         {Category: "tasks", Description: "Completed background task logs"},
		"conversations": {Category: "conversations", Description: "Conversational logs & sqlite files"},
		"other":         {Category: "other", Description: "Miscellaneous context files"},
	}

	// Scan Brain directory
	if err := filepath.Walk(ins.BrainDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		size := info.Size()
		bd.BrainTotalBytes += size

		rel, _ := filepath.Rel(ins.BrainDir, path)
		catKey := "other"
		if containsPathSegment(rel, "scratch") {
			catKey = "scratch"
		} else if containsPathSegment(rel, "steps") {
			catKey = "steps"
		} else if containsPathSegment(rel, "tasks") {
			catKey = "tasks"
		}

		c := categories[catKey]
		c.TotalBytes += size
		c.FileCount++

		if info.ModTime().Before(cutoff) && catKey != "other" {
			bd.ReclaimableBytes += size
		}
		return nil
	}); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Scan Conversations directory
	if err := filepath.Walk(ins.ConversationsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		size := info.Size()
		bd.ConversationsTotalBytes += size

		c := categories["conversations"]
		c.TotalBytes += size
		c.FileCount++
		return nil
	}); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	for k, v := range categories {
		bd.Categories[k] = *v
	}
	return bd, nil
}

func containsPathSegment(relPath, segment string) bool {
	dir := filepath.Dir(relPath)
	for dir != "." && dir != "/" {
		if filepath.Base(dir) == segment {
			return true
		}
		dir = filepath.Dir(dir)
	}
	return filepath.Base(relPath) == segment
}
