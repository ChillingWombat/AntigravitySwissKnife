package revival

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/gui"
)

const (
	// IntentsFileName holds every live revival intent as {"intents": [...]}.
	//
	// An account switch can kill N running conversations at once, so the store is
	// a keyed collection (keyed by RootConversationID, addressed by IntentID),
	// not a single slot.
	IntentsFileName = "pending_continuations.json"
	// LegacyIntentsFileName held exactly one flat intent before intents became a
	// collection. It is still read (a flat object decodes as a one-element list)
	// so an upgrade does not drop a pending continuation, and removed once the
	// plural file has been committed.
	LegacyIntentsFileName = "pending_continuation.json"
)

// intentsFile is the on-disk envelope for the pending revival intents.
type intentsFile struct {
	Intents []*RevivalIntent `json:"intents"`
}

// Store manages disk persistence for pending conversation revival intents.
type Store struct {
	dir        string
	filePath   string
	legacyPath string
	mu         sync.Mutex
}

// NewStore creates a new Store instance. If dir is empty, it uses the standard config directory.
func NewStore(dir string) *Store {
	if dir == "" {
		dir = core.GetConfigDir()
	}
	return &Store{
		dir:        dir,
		filePath:   filepath.Join(dir, IntentsFileName),
		legacyPath: filepath.Join(dir, LegacyIntentsFileName),
	}
}

// FilePath returns the absolute path to the pending continuations file.
func (s *Store) FilePath() string {
	return s.filePath
}

// LegacyFilePath returns the absolute path of the superseded single-intent file.
func (s *Store) LegacyFilePath() string {
	return s.legacyPath
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

// SaveIntent upserts a RevivalIntent into the keyed collection and commits it atomically.
//
// Upsert key is RootConversationID: one conversation can never hold two competing
// intents, which is what keeps a single conversation to a single resume prompt.
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
	if err := prepareIntent(intent); err != nil {
		return err
	}

	// A save is authoritative: an unreadable file is replaced, not reported, so
	// a truncated write or a manual edit cannot wedge revival permanently.
	existing, err := s.loadIntentsInternal()
	if err != nil {
		existing = nil
	}

	return s.saveIntentsInternal(upsertIntent(existing, intent))
}

// prepareIntent fills the compatibility fields the desktop preload script and the
// in-page dispatcher read, defaults the TTL/attempt bounds, and rejects IDs that
// could never correspond to a real conversation.
func prepareIntent(intent *RevivalIntent) error {
	if intent.RootConversationID == "" && intent.CascadeID != "" {
		intent.RootConversationID = intent.CascadeID
	}
	if intent.CascadeID == "" {
		intent.CascadeID = intent.RootConversationID
	}
	if intent.Prompt == "" {
		intent.Prompt = intent.TriggerPrompt
	}
	if intent.TriggerPrompt == "" {
		intent.TriggerPrompt = intent.Prompt
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
	return nil
}

// upsertIntent replaces the intent for the same conversation (or the same IntentID)
// and appends a new one otherwise. Never returns two intents for one conversation.
func upsertIntent(existing []*RevivalIntent, intent *RevivalIntent) []*RevivalIntent {
	next := make([]*RevivalIntent, 0, len(existing)+1)
	for _, prev := range existing {
		if prev == nil {
			continue
		}
		sameConversation := prev.RootConversationID == intent.RootConversationID
		sameIntent := intent.IntentID != "" && prev.IntentID == intent.IntentID
		if sameConversation || sameIntent {
			continue
		}
		next = append(next, prev)
	}
	next = append(next, intent)
	sortIntentsNewestFirst(next)
	return next
}

// saveIntentsInternal commits the whole collection atomically.
func (s *Store) saveIntentsInternal(intents []*RevivalIntent) error {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(intentsFile{Intents: intents}, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize revival intents: %w", err)
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

	// The legacy single-object file is superseded the moment the plural file
	// lands; leaving it behind would resurrect an already-migrated intent.
	if err := os.Remove(s.legacyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove legacy continuation file: %w", err)
	}
	return nil
}

// LoadIntents returns every live intent, newest first. This is the primary read API.
func (s *Store) LoadIntents() ([]*RevivalIntent, error) {
	fl, err := s.lock()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire store file lock: %w", err)
	}
	defer s.unlock(fl)

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.loadIntentsInternal()
}

// LoadIntent returns the newest live intent, or nil when nothing is pending.
// Kept for callers that only need the primary intent.
func (s *Store) LoadIntent() (*RevivalIntent, error) {
	intents, err := s.LoadIntents()
	if err != nil {
		return nil, err
	}
	if len(intents) == 0 {
		return nil, nil
	}
	return intents[0], nil
}

// loadIntentsInternal reads, normalizes and TTL-prunes the collection.
// Callers must hold the store lock.
func (s *Store) loadIntentsInternal() ([]*RevivalIntent, error) {
	raw, err := s.readIntentsRaw()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	fileMod := s.fileModTime()
	live := make([]*RevivalIntent, 0, len(raw))
	for _, intent := range raw {
		if intent == nil {
			continue
		}
		normalizeLoadedIntent(intent)
		if !gui.IsValidConversationID(intent.RootConversationID) {
			continue
		}
		if intent.CreatedAt.IsZero() {
			if fileMod.IsZero() {
				continue
			}
			intent.CreatedAt = fileMod
		}
		ttl := intent.TTLSeconds
		if ttl <= 0 {
			ttl = DefaultTTLSeconds
		}
		if now.Sub(intent.CreatedAt) > time.Duration(ttl)*time.Second {
			continue
		}
		live = append(live, intent)
	}

	sortIntentsNewestFirst(live)
	return live, nil
}

// readIntentsRaw returns the intents on disk, falling back to the legacy
// single-intent file when the plural file does not exist yet.
func (s *Store) readIntentsRaw() ([]*RevivalIntent, error) {
	data, err := os.ReadFile(s.filePath)
	if err == nil {
		return decodeIntents(data)
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to read continuation file: %w", err)
	}

	legacy, legacyErr := s.readLegacyIntent()
	if legacyErr != nil {
		return nil, legacyErr
	}
	if legacy != nil {
		return []*RevivalIntent{legacy}, nil
	}
	return nil, nil
}

// readLegacyIntent loads the pre-collection file. A missing legacy file is not an error.
func (s *Store) readLegacyIntent() (*RevivalIntent, error) {
	data, err := os.ReadFile(s.legacyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read legacy continuation file: %w", err)
	}
	intents, err := decodeIntents(data)
	if err != nil {
		return nil, err
	}
	for _, intent := range intents {
		if intent != nil {
			return intent, nil
		}
	}
	return nil, nil
}

// decodeIntents accepts the {"intents": [...]} envelope, a bare array, and the
// legacy flat single-intent object (migrated to a one-element list).
func decodeIntents(data []byte) ([]*RevivalIntent, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}

	if trimmed[0] == '[' {
		var list []*RevivalIntent
		if err := json.Unmarshal(trimmed, &list); err != nil {
			return nil, fmt.Errorf("failed to parse continuation file: %w", err)
		}
		return list, nil
	}

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse continuation file: %w", err)
	}
	if raw, ok := probe["intents"]; ok {
		var list []*RevivalIntent
		if body := bytes.TrimSpace(raw); len(body) > 0 && string(body) != "null" {
			if err := json.Unmarshal(body, &list); err != nil {
				return nil, fmt.Errorf("failed to parse continuation intents: %w", err)
			}
		}
		return list, nil
	}

	var single RevivalIntent
	if err := json.Unmarshal(trimmed, &single); err != nil {
		return nil, fmt.Errorf("failed to parse continuation file: %w", err)
	}
	return []*RevivalIntent{&single}, nil
}

// normalizeLoadedIntent reconciles the compatibility fields of a persisted intent.
func normalizeLoadedIntent(intent *RevivalIntent) {
	if intent.RootConversationID == "" && intent.CascadeID != "" {
		intent.RootConversationID = intent.CascadeID
	}
	if intent.CascadeID == "" {
		intent.CascadeID = intent.RootConversationID
	}
	if intent.TriggerPrompt == "" && intent.Prompt != "" {
		intent.TriggerPrompt = intent.Prompt
	}
	if intent.Prompt == "" {
		intent.Prompt = intent.TriggerPrompt
	}
	if intent.CreatedAt.IsZero() && intent.Timestamp > 0 {
		intent.CreatedAt = time.UnixMilli(intent.Timestamp)
	}
	if intent.MaxAttempts <= 0 {
		intent.MaxAttempts = DefaultMaxAttempts
	}
	if intent.Status == IntentStatusRevived {
		intent.Resumed = true
	}
}

func (s *Store) fileModTime() time.Time {
	if info, err := os.Stat(s.filePath); err == nil {
		return info.ModTime()
	}
	if info, err := os.Stat(s.legacyPath); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}

// sortIntentsNewestFirst orders intents by creation time, newest first, with
// IntentID as a deterministic tie-break for intents created in the same tick.
func sortIntentsNewestFirst(intents []*RevivalIntent) {
	sort.SliceStable(intents, func(i, j int) bool {
		a, b := intents[i], intents[j]
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.After(b.CreatedAt)
		}
		if a.Timestamp != b.Timestamp {
			return a.Timestamp > b.Timestamp
		}
		return a.IntentID > b.IntentID
	})
}

// ClearIntent removes every pending intent.
func (s *Store) ClearIntent() error {
	fl, err := s.lock()
	if err != nil {
		return fmt.Errorf("failed to acquire store file lock: %w", err)
	}
	defer s.unlock(fl)

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, path := range []string{s.filePath, s.legacyPath} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to clear continuation file %s: %w", path, err)
		}
	}
	return nil
}

// ClearIntentFor removes the intent for one conversation, leaving the others
// intact. Unknown or already-cleared conversations are a no-op.
func (s *Store) ClearIntentFor(convID string) error {
	target := cleanConversationID(convID)
	if target == "" {
		return fmt.Errorf("conversation ID cannot be empty")
	}

	fl, err := s.lock()
	if err != nil {
		return fmt.Errorf("failed to acquire store file lock: %w", err)
	}
	defer s.unlock(fl)

	s.mu.Lock()
	defer s.mu.Unlock()

	live, err := s.loadIntentsInternal()
	if err != nil {
		return err
	}

	next := make([]*RevivalIntent, 0, len(live))
	for _, intent := range live {
		if cleanConversationID(intent.RootConversationID) == target ||
			cleanConversationID(intent.CascadeID) == target {
			continue
		}
		next = append(next, intent)
	}
	if len(next) == len(live) {
		return nil
	}
	return s.saveIntentsInternal(next)
}

// UpdateStatus updates the status (and optionally attempts) of one intent,
// matched by IntentID. An empty intentID targets the newest live intent.
func (s *Store) UpdateStatus(intentID string, status IntentStatus, attempts ...int) error {
	fl, err := s.lock()
	if err != nil {
		return fmt.Errorf("failed to acquire store file lock: %w", err)
	}
	defer s.unlock(fl)

	s.mu.Lock()
	defer s.mu.Unlock()

	intents, err := s.loadIntentsInternal()
	if err != nil {
		return err
	}
	idx := findIntentIndex(intents, intentID)
	if idx < 0 {
		return nil
	}

	intent := intents[idx]
	intent.Status = status
	if len(attempts) > 0 && attempts[0] >= 0 {
		intent.Attempts = attempts[0]
	}
	if status == IntentStatusRevived {
		intent.Resumed = true
	}
	return s.saveIntentsInternal(intents)
}

// IncrementAttempts atomically increments the attempt count of one intent.
func (s *Store) IncrementAttempts(intentID string) (int, error) {
	fl, err := s.lock()
	if err != nil {
		return 0, fmt.Errorf("failed to acquire store file lock: %w", err)
	}
	defer s.unlock(fl)

	s.mu.Lock()
	defer s.mu.Unlock()

	intents, err := s.loadIntentsInternal()
	if err != nil {
		return 0, err
	}
	idx := findIntentIndex(intents, intentID)
	if idx < 0 {
		return 0, nil
	}

	intent := intents[idx]
	intent.Attempts++
	if err := s.saveIntentsInternal(intents); err != nil {
		return intent.Attempts, err
	}
	return intent.Attempts, nil
}

// findIntentIndex resolves an intent by ID, falling back to the newest intent
// when no ID is supplied (preserves the pre-collection single-slot behaviour).
func findIntentIndex(intents []*RevivalIntent, intentID string) int {
	if len(intents) == 0 {
		return -1
	}
	if intentID == "" {
		return 0
	}
	for i, intent := range intents {
		if intent.IntentID == intentID {
			return i
		}
	}
	return -1
}

// cleanConversationID strips the route prefix and any query/fragment suffix.
func cleanConversationID(convID string) string {
	cleaned := strings.TrimSpace(convID)
	cleaned = strings.TrimPrefix(cleaned, "/c/")
	if idx := strings.IndexAny(cleaned, "?#"); idx != -1 {
		cleaned = cleaned[:idx]
	}
	return strings.TrimSpace(cleaned)
}
