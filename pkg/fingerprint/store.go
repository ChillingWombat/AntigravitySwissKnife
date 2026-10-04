package fingerprint

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// Store provides thread-safe persistent profile storage keyed by user email.
type Store struct {
	path     string
	profiles map[string]*DeviceProfile
	mu       sync.RWMutex
}

// NewStore initializes a profile store targeting the given file path.
func NewStore(path string) (*Store, error) {
	if path == "" {
		path = filepath.Join(core.GetConfigDir(), "device_profiles.json")
	}

	s := &Store{
		path:     path,
		profiles: make(map[string]*DeviceProfile),
	}

	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.Unmarshal(data, &s.profiles)
}

func (s *Store) save() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s.profiles, "", "  ")
	if err != nil {
		return err
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// GetProfile returns the profile associated with the email, or nil if none exists.
func (s *Store) GetProfile(email string) *DeviceProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.profiles[email]
}

// GetOrCreateProfile retrieves or creates a valid random profile for an email.
func (s *Store) GetOrCreateProfile(email string) (*DeviceProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if prof, exists := s.profiles[email]; exists {
		return prof, nil
	}

	prof, err := GenerateRandom()
	if err != nil {
		return nil, err
	}
	s.profiles[email] = prof
	if err := s.save(); err != nil {
		return nil, err
	}
	return prof, nil
}

// SetProfile associates and persists a profile for an email.
func (s *Store) SetProfile(email string, profile *DeviceProfile) error {
	if err := profile.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.profiles[email] = profile
	return s.save()
}

// ListProfiles returns a copy of all registered profiles.
func (s *Store) ListProfiles() map[string]*DeviceProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]*DeviceProfile, len(s.profiles))
	for k, v := range s.profiles {
		out[k] = v
	}
	return out
}
