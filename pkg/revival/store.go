package revival

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
)

// Store manages disk persistence for pending conversation revival intents.
type Store struct {
	dir      string
	filePath string
	mu       sync.Mutex
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

func (s *Store) lock() (*os.File, error) {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return nil, err
	}
	lockFile := s.filePath + ".lock"
	f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := core.LockFile(f); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

func (s *Store) unlock(f *os.File) {
	if f != nil {
		_ = core.UnlockFile(f)
		_ = f.Close()
	}
}

// SaveIntent writes the RevivalIntent atomically to disk under file and mutex locks.
func (s *Store) SaveIntent(intent *RevivalIntent) error {
	if intent == nil {
		return fmt.Errorf("intent cannot be nil")
	}

	fl, err := s.lock()
	if err != nil {
		return fmt.Errorf("failed to acquire store file lock: %w", err)
	}
	defer s.unlock(fl)

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.saveIntentInternal(intent)
}

func (s *Store) saveIntentInternal(intent *RevivalIntent) error {
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
	if intent.TTLSeconds <= 0 {
		intent.TTLSeconds = DefaultTTLSeconds
	}
	if intent.MaxAttempts <= 0 {
		intent.MaxAttempts = DefaultMaxAttempts
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
	fl, err := s.lock()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire store file lock: %w", err)
	}
	defer s.unlock(fl)

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.loadIntentInternal()
}

func (s *Store) loadIntentInternal() (*RevivalIntent, error) {
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
	if intent.MaxAttempts <= 0 {
		intent.MaxAttempts = DefaultMaxAttempts
	}

	// Reject invalid conversation ID
	if !gui.IsValidConversationID(intent.RootConversationID) {
		_ = os.Remove(s.filePath)
		return nil, nil
	}

	// Check TTL expiration
	ttl := intent.TTLSeconds
	if ttl <= 0 {
		ttl = DefaultTTLSeconds
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
	return &intent, nil
}

// ClearIntent removes the pending continuation file.
func (s *Store) ClearIntent() error {
	fl, err := s.lock()
	if err != nil {
		return fmt.Errorf("failed to acquire store file lock: %w", err)
	}
	defer s.unlock(fl)

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(s.filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear continuation file: %w", err)
	}
	return nil
}

// UpdateStatus updates the status (and optionally attempts) of an active intent.
func (s *Store) UpdateStatus(intentID string, status IntentStatus, attempts ...int) error {
	fl, err := s.lock()
	if err != nil {
		return fmt.Errorf("failed to acquire store file lock: %w", err)
	}
	defer s.unlock(fl)

	s.mu.Lock()
	defer s.mu.Unlock()

	intent, err := s.loadIntentInternal()
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
	if len(attempts) > 0 && attempts[0] >= 0 {
		intent.Attempts = attempts[0]
	}
	if status == IntentStatusRevived {
		intent.Resumed = true
	}
	return s.saveIntentInternal(intent)
}

// IncrementAttempts atomically increments the attempt count of the pending intent.
func (s *Store) IncrementAttempts(intentID string) (int, error) {
	fl, err := s.lock()
	if err != nil {
		return 0, fmt.Errorf("failed to acquire store file lock: %w", err)
	}
	defer s.unlock(fl)

	s.mu.Lock()
	defer s.mu.Unlock()

	intent, err := s.loadIntentInternal()
	if err != nil {
		return 0, err
	}
	if intent == nil {
		return 0, nil
	}
	if intentID != "" && intent.IntentID != intentID {
		return intent.Attempts, nil
	}

	intent.Attempts++
	if err := s.saveIntentInternal(intent); err != nil {
		return intent.Attempts, err
	}
	return intent.Attempts, nil
}
