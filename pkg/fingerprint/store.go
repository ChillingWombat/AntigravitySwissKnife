package fingerprint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	if err := json.Unmarshal(data, &s.profiles); err != nil {
		return err
	}
	if s.sanitizeAndDeduplicateLocked() {
		_ = s.save()
	}
	return nil
}

func (s *Store) isDefaultHostStoreLocked() bool {
	defaultPath := filepath.Join(core.GetConfigDir(), "device_profiles.json")
	return filepath.Clean(s.path) == filepath.Clean(defaultPath)
}

func isMockFixtureEmail(email string) bool {
	norm := strings.ToLower(strings.TrimSpace(email))
	return norm == "target@gmail.com" || norm == "test.import.user@gmail.com" || strings.HasPrefix(norm, "mock_")
}

// sanitizeAndDeduplicateLocked heals invalid formats and ensures no two accounts share any hardware identifier.
func (s *Store) sanitizeAndDeduplicateLocked() bool {
	changed := false
	isHostDefault := s.isDefaultHostStoreLocked()

	emails := make([]string, 0, len(s.profiles))
	for em := range s.profiles {
		if isHostDefault && isMockFixtureEmail(em) {
			delete(s.profiles, em)
			changed = true
			continue
		}
		emails = append(emails, em)
	}
	sort.Strings(emails)

	seenMachine := make(map[string]bool, len(emails))
	seenUpdater := make(map[string]bool, len(emails))
	seenInstallID := make(map[string]bool, len(emails))
	seenInstallUUID := make(map[string]bool, len(emails))

	for _, em := range emails {
		prof := s.profiles[em]
		if prof == nil {
			prof = &DeviceProfile{AccountEmail: em}
			s.profiles[em] = prof
			changed = true
		}
		if prof.AccountEmail != em {
			prof.AccountEmail = em
			changed = true
		}

		if !machineRegex.MatchString(prof.MachineID) || seenMachine[prof.MachineID] {
			for {
				mID, err := GenerateMachineID()
				if err == nil && !seenMachine[mID] {
					prof.MachineID = mID
					changed = true
					break
				}
			}
		}
		seenMachine[prof.MachineID] = true

		if !uuidRegex.MatchString(prof.UpdaterID) || seenUpdater[prof.UpdaterID] {
			for {
				uID, err := GenerateUUIDv4()
				if err == nil && !seenUpdater[uID] {
					prof.UpdaterID = uID
					changed = true
					break
				}
			}
		}
		seenUpdater[prof.UpdaterID] = true

		if !uuidRegex.MatchString(prof.InstallationID) || seenInstallID[prof.InstallationID] {
			for {
				iID, err := GenerateUUIDv4()
				if err == nil && !seenInstallID[iID] {
					prof.InstallationID = iID
					changed = true
					break
				}
			}
		}
		seenInstallID[prof.InstallationID] = true

		if !uuidRegex.MatchString(prof.InstallationUUID) || seenInstallUUID[prof.InstallationUUID] {
			for {
				sUUID, err := GenerateUUIDv4()
				if err == nil && !seenInstallUUID[sUUID] {
					prof.InstallationUUID = sUUID
					changed = true
					break
				}
			}
		}
		seenInstallUUID[prof.InstallationUUID] = true
	}

	return changed
}

func (s *Store) hasCollisionLocked(excludeEmail string, candidate *DeviceProfile) (string, bool) {
	for em, existing := range s.profiles {
		if em == excludeEmail || existing == nil {
			continue
		}
		if candidate.MachineID != "" && existing.MachineID == candidate.MachineID {
			return fmt.Sprintf("machine_id collides with %s", em), true
		}
		if candidate.UpdaterID != "" && existing.UpdaterID == candidate.UpdaterID {
			return fmt.Sprintf("updater_id collides with %s", em), true
		}
		if candidate.InstallationID != "" && existing.InstallationID == candidate.InstallationID {
			return fmt.Sprintf("installation_id collides with %s", em), true
		}
		if candidate.InstallationUUID != "" && existing.InstallationUUID == candidate.InstallationUUID {
			return fmt.Sprintf("installation_uuid collides with %s", em), true
		}
	}
	return "", false
}

func (s *Store) generateUniqueLocked(email string) (*DeviceProfile, error) {
	for attempt := 0; attempt < 16; attempt++ {
		prof, err := GenerateRandom()
		if err != nil {
			return nil, err
		}
		prof.AccountEmail = email
		if _, collides := s.hasCollisionLocked(email, prof); !collides {
			return prof, nil
		}
	}
	return nil, fmt.Errorf("failed to generate unique device profile for %s", email)
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
	prof := s.profiles[email]
	if prof == nil {
		return nil
	}
	cp := *prof
	cp.AccountEmail = email
	return &cp
}

// GetOrCreateProfile retrieves or creates a valid, unique random profile for an email.
func (s *Store) GetOrCreateProfile(email string) (*DeviceProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if prof, exists := s.profiles[email]; exists && prof != nil {
		if prof.Validate() == nil {
			if _, collides := s.hasCollisionLocked(email, prof); !collides {
				cp := *prof
				cp.AccountEmail = email
				return &cp, nil
			}
		}
	}

	prof, err := s.generateUniqueLocked(email)
	if err != nil {
		return nil, err
	}
	s.profiles[email] = prof
	if err := s.save(); err != nil {
		return nil, err
	}
	cp := *prof
	return &cp, nil
}

// SetProfile associates and persists a profile for an email, rejecting cross-account ID collisions.
func (s *Store) SetProfile(email string, profile *DeviceProfile) error {
	if err := profile.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if reason, collides := s.hasCollisionLocked(email, profile); collides {
		return fmt.Errorf("profile collision rejected: %s", reason)
	}

	cp := *profile
	cp.AccountEmail = email
	s.profiles[email] = &cp
	return s.save()
}

// ListProfiles returns a copy of all registered profiles keyed by email.
func (s *Store) ListProfiles() map[string]*DeviceProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]*DeviceProfile, len(s.profiles))
	for k, v := range s.profiles {
		if v == nil {
			continue
		}
		cp := *v
		cp.AccountEmail = k
		out[k] = &cp
	}
	return out
}

// ListProfilesSlice returns all registered profiles sorted by email with AccountEmail populated.
func (s *Store) ListProfilesSlice() []DeviceProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()

	emails := make([]string, 0, len(s.profiles))
	for k, v := range s.profiles {
		if v != nil {
			emails = append(emails, k)
		}
	}
	sort.Strings(emails)

	out := make([]DeviceProfile, 0, len(emails))
	for _, em := range emails {
		cp := *s.profiles[em]
		cp.AccountEmail = em
		out = append(out, cp)
	}
	return out
}
