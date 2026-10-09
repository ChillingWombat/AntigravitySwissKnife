package revival

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
)

// Store manages disk persistence for pending conversation revival intents.
type Store struct {
	dir      string
	filePath string
	mu       sync.RWMutex
}

// NewStore creates a new Store instance. If dir is empty, it uses the standard config directory.
func NewStore(dir string) *Store {
	if dir == "" {
		dir = core.GetConfigDir()
	}
	return &Store{
		dir:      dir,
		filePath: filepath.Join(dir, "pending_continuation.json"),
	}
}

// FilePath returns the absolute path to the pending continuation file.
func (s *Store) FilePath() string {
	return s.filePath
}

// SaveIntent writes the RevivalIntent atomically to disk.
func (s *Store) SaveIntent(intent *RevivalIntent) error {
	if intent == nil {
		return fmt.Errorf("intent cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Fill compatibility fields if missing
	if intent.CascadeID == "" {
		intent.CascadeID = intent.RootConversationID
	}
	if intent.Prompt == "" {
		intent.Prompt = intent.TriggerPrompt
	}
	if intent.Timestamp == 0 {
		if intent.CreatedAt.IsZero() {
			intent.CreatedAt = time.Now()
		}
		intent.Timestamp = intent.CreatedAt.UnixMilli()
	}
	if intent.TTLSeconds == 0 {
		intent.TTLSeconds = 90
	}
	if intent.MaxAttempts == 0 {
		intent.MaxAttempts = 2
	}
	if intent.Status == IntentStatusRevived {
		intent.Resumed = true
	}

	if !gui.IsValidConversationID(intent.RootConversationID) {
		return fmt.Errorf("refusing to save revival intent: conversation ID %q is invalid", intent.RootConversationID)
	}

	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(intent, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize revival intent: %w", err)
	}

	tmpFile := fmt.Sprintf("%s.%d.%d.tmp", s.filePath, os.Getpid(), time.Now().UnixNano())
	defer func() {
		_ = os.Remove(tmpFile)
	}()

	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write tmp continuation file: %w", err)
	}

	if err := os.Rename(tmpFile, s.filePath); err != nil {
		return fmt.Errorf("failed to atomically commit continuation file: %w", err)
	}

	return nil
}

// LoadIntent reads and deserializes the pending RevivalIntent from disk.
// Returns nil, nil if no pending intent exists or if it has expired.
func (s *Store) LoadIntent() (*RevivalIntent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read continuation file: %w", err)
	}

	var intent RevivalIntent
	if err := json.Unmarshal(data, &intent); err != nil {
		return nil, fmt.Errorf("failed to parse continuation file: %w", err)
	}

	// Reconcile root fields with compatibility fields if one is set
	if intent.RootConversationID == "" && intent.CascadeID != "" {
		intent.RootConversationID = intent.CascadeID
	}
	if intent.TriggerPrompt == "" && intent.Prompt != "" {
		intent.TriggerPrompt = intent.Prompt
	}
	if intent.CreatedAt.IsZero() && intent.Timestamp > 0 {
		intent.CreatedAt = time.UnixMilli(intent.Timestamp)
	}

	// Reject invalid conversation ID
	if !gui.IsValidConversationID(intent.RootConversationID) {
		_ = os.Remove(s.filePath)
		return nil, nil
	}

	// Check TTL expiration
	ttl := intent.TTLSeconds
	if ttl <= 0 {
		ttl = 90
	}
	if intent.CreatedAt.IsZero() {
		if info, statErr := os.Stat(s.filePath); statErr == nil {
			intent.CreatedAt = info.ModTime()
		} else {
			_ = os.Remove(s.filePath)
			return nil, nil
		}
	}
	if time.Since(intent.CreatedAt) > time.Duration(ttl)*time.Second {
		// Stale intent
		_ = os.Remove(s.filePath)
		return nil, nil
	}

	// Dedupe against pinned conversation (in production config directory):
	// If intent path does not match pinned last conversation path, and intent is older than 25s, drop it
	if s.dir == core.GetConfigDir() {
		if pinned := gui.LoadPinnedConversationPath(); gui.IsValidConversationPath(pinned) {
			pinnedID := strings.TrimPrefix(pinned, "/c/")
			if !strings.EqualFold(pinnedID, intent.RootConversationID) && time.Since(intent.CreatedAt) > 25*time.Second {
				_ = os.Remove(s.filePath)
				return nil, nil
			}
		}
	}

	return &intent, nil
}

// ClearIntent removes the pending continuation file.
func (s *Store) ClearIntent() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(s.filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear continuation file: %w", err)
	}
	return nil
}

// UpdateStatus updates the status of an active intent.
func (s *Store) UpdateStatus(intentID string, status IntentStatus) error {
	intent, err := s.LoadIntent()
	if err != nil {
		return err
	}
	if intent == nil {
		return nil
	}
	if intentID != "" && intent.IntentID != intentID {
		return nil
	}

	intent.Status = status
	if status == IntentStatusRevived {
		intent.Resumed = true
	}
	return s.SaveIntent(intent)
}
