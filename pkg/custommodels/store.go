package custommodels

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// Store manages the custom models configuration file and project bindings.
type Store struct {
	mu         sync.RWMutex
	configPath string
	config     *Config
}

// NewStore initializes a Store with config path in core.GetConfigDir().
func NewStore(configDir string) (*Store, error) {
	if configDir == "" {
		configDir = core.GetConfigDir()
	}
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config dir: %w", err)
	}

	configPath := filepath.Join(configDir, "custom_models.json")
	s := &Store{
		configPath: configPath,
		config:     DefaultConfig(),
	}

	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}



	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	if cfg.ProjectBinds == nil {
		cfg.ProjectBinds = make(map[string]string)
	}
	s.config = &cfg
	for i := range s.config.Models {
		if s.config.Models[i].InternalID <= 0 {
			s.config.Models[i].InternalID = s.allocateInternalIDLocked()
		}
		s.syncCustomModelToPricingLocked(s.config.Models[i])
	}
	for i := range s.config.PricingRecords {
		if s.config.PricingRecords[i].InternalID <= 0 {
			s.config.PricingRecords[i].InternalID = s.allocateInternalIDLocked()
		}
	}
	return nil
}

func (s *Store) saveLocked() error {
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return err
	}

	tmpFile := s.configPath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpFile, s.configPath)
}

// Save writes the in-memory config to disk atomically.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

// GetConfig returns a deep copy of the current configuration.
func (s *Store) GetConfig() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()

	modelsCopy := make([]CustomModel, len(s.config.Models))
	for i, m := range s.config.Models {
		mCopy := m
		mCopy.QuotaFraction = cloneFloatPtr(m.QuotaFraction)
		mCopy.BudgetCapValue = cloneFloatPtr(m.BudgetCapValue)
		mCopy.InputPricePerM = cloneFloatPtr(m.InputPricePerM)
		mCopy.CachedInputPricePerM = cloneFloatPtr(m.CachedInputPricePerM)
		mCopy.OutputPricePerM = cloneFloatPtr(m.OutputPricePerM)
		modelsCopy[i] = mCopy
	}

	pricingCopy := s.listPricingRecordsLocked()

	deletedCopy := make([]DeletedModelRecord, len(s.config.DeletedModels))
	copy(deletedCopy, s.config.DeletedModels)

	bindsCopy := make(map[string]string)
	for k, v := range s.config.ProjectBinds {
		bindsCopy[k] = v
	}

	return Config{
		Version:        s.config.Version,
		ActiveModelID:  s.config.ActiveModelID,
		NextInternalID: s.config.NextInternalID,
		Models:         modelsCopy,
		PricingRecords: pricingCopy,
		DeletedModels:  deletedCopy,
		ProjectBinds:   bindsCopy,
	}
}

// ListModels returns all configured custom models.
func (s *Store) ListModels() []CustomModel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]CustomModel, len(s.config.Models))
	for i, m := range s.config.Models {
		mCopy := m
		mCopy.QuotaFraction = cloneFloatPtr(m.QuotaFraction)
		mCopy.BudgetCapValue = cloneFloatPtr(m.BudgetCapValue)
		mCopy.InputPricePerM = cloneFloatPtr(m.InputPricePerM)
		mCopy.CachedInputPricePerM = cloneFloatPtr(m.CachedInputPricePerM)
		mCopy.OutputPricePerM = cloneFloatPtr(m.OutputPricePerM)
		res[i] = mCopy
	}
	return res
}

// GetModel retrieves a custom model by ID.
func (s *Store) GetModel(id string) (*CustomModel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, m := range s.config.Models {
		if m.ID == id {
			mCopy := m
			mCopy.QuotaFraction = cloneFloatPtr(m.QuotaFraction)
			mCopy.BudgetCapValue = cloneFloatPtr(m.BudgetCapValue)
			mCopy.InputPricePerM = cloneFloatPtr(m.InputPricePerM)
			mCopy.CachedInputPricePerM = cloneFloatPtr(m.CachedInputPricePerM)
			mCopy.OutputPricePerM = cloneFloatPtr(m.OutputPricePerM)
			return &mCopy, nil
		}
	}
	return nil, fmt.Errorf("custom model with ID %q not found", id)
}

// SaveModel inserts or updates a custom model and synchronizes its pricing record.
func (s *Store) SaveModel(m CustomModel) error {
	if strings.TrimSpace(m.ID) == "" {
		slug := strings.ToLower(strings.TrimSpace(m.Name))
		slug = strings.ReplaceAll(slug, " ", "-")
		slug = strings.ReplaceAll(slug, "/", "-")
		slug = strings.ReplaceAll(slug, ":", "-")
		if slug == "" {
			slug = "model"
		}
		prefix := string(m.ProviderType)
		if prefix == "" {
			prefix = "custom"
		}
		m.ID = fmt.Sprintf("%s-%s-%d", prefix, slug, time.Now().Unix())
	}

	if err := m.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	nowShort := time.Now().Format("2006-01-02 15:04")
	found := false
	for i, existing := range s.config.Models {
		if existing.ID == m.ID {
			if existing.InternalID > 0 {
				m.InternalID = existing.InternalID
			} else if m.InternalID <= 0 {
				m.InternalID = s.allocateInternalIDLocked()
			}
			m.CreatedAt = existing.CreatedAt
			m.UpdatedAt = now
			if m.PriceUpdatedAt == "" {
				if existing.PriceUpdatedAt != "" {
					m.PriceUpdatedAt = existing.PriceUpdatedAt
				} else {
					m.PriceUpdatedAt = nowShort
				}
			}
			s.config.Models[i] = m
			found = true
			break
		}
	}

	if !found {
		if m.InternalID <= 0 {
			m.InternalID = s.allocateInternalIDLocked()
		}
		if m.CreatedAt == "" {
			m.CreatedAt = now
		}
		m.UpdatedAt = now
		if m.PriceUpdatedAt == "" {
			m.PriceUpdatedAt = nowShort
		}
		s.config.Models = append(s.config.Models, m)
	}

	s.clearDeletedModelLocked(m.Name, m.ID)
	s.syncCustomModelToPricingLocked(m)

	return s.saveLocked()
}

// DeleteModel removes a custom model by ID along with its pricing record and marks its usage deleted.
func (s *Store) DeleteModel(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var deleted *CustomModel
	newModels := make([]CustomModel, 0, len(s.config.Models))
	for _, m := range s.config.Models {
		if m.ID != id {
			newModels = append(newModels, m)
		} else {
			mCopy := m
			deleted = &mCopy
		}
	}

	if deleted == nil {
		return fmt.Errorf("model %q not found to delete", id)
	}

	s.config.Models = newModels

	// Remove corresponding pricing record(s)
	filteredPricing := make([]ModelPricingRecord, 0, len(s.config.PricingRecords))
	for _, r := range s.config.PricingRecords {
		if (deleted.InternalID > 0 && r.InternalID == deleted.InternalID) || (r.CustomModelID != "" && r.CustomModelID == id) {
			continue
		}
		filteredPricing = append(filteredPricing, r)
	}
	s.config.PricingRecords = filteredPricing

	// Record deletion so Token Monitor usage info is also purged
	s.recordDeletedModelLocked(deleted.InternalID, deleted.Name, deleted.ID)

	// Clean up binds
	for p, boundID := range s.config.ProjectBinds {
		if boundID == id {
			delete(s.config.ProjectBinds, p)
		}
	}

	return s.saveLocked()
}

// BindProject assigns a custom model ID to a project.
func (s *Store) BindProject(projectName, modelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if modelID == "" {
		delete(s.config.ProjectBinds, projectName)
		return s.saveLocked()
	}

	// Verify model exists or is native
	if modelID != "native" {
		exists := false
		for _, m := range s.config.Models {
			if m.ID == modelID {
				exists = true
				break
			}
		}
		if !exists {
			return fmt.Errorf("cannot bind unknown model ID: %s", modelID)
		}
	}

	s.config.ProjectBinds[projectName] = modelID
	return s.saveLocked()
}

// GetModelForProject returns the assigned custom model for a project (or default matching model).
func (s *Store) GetModelForProject(projectName string) *CustomModel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	target := strings.TrimSpace(projectName)

	// 1. Check explicit binding
	for p, boundID := range s.config.ProjectBinds {
		if strings.EqualFold(strings.TrimSpace(p), target) {
			if boundID == "native" {
				return nil
			}
			for _, m := range s.config.Models {
				if m.ID == boundID && m.Enabled {
					mCopy := m
					return &mCopy
				}
			}
		}
	}

	// 2. Check exact project mapping in models list (exact name takes priority over wildcard)
	for _, m := range s.config.Models {
		if !m.Enabled {
			continue
		}
		for _, p := range m.ProjectMappings {
			trimmed := strings.TrimSpace(p)
			if trimmed != "*" && strings.EqualFold(trimmed, target) {
				mCopy := m
				return &mCopy
			}
		}
	}

	// 3. Check wildcard "*" mapping
	for _, m := range s.config.Models {
		if !m.Enabled {
			continue
		}
		for _, p := range m.ProjectMappings {
			if strings.TrimSpace(p) == "*" {
				mCopy := m
				return &mCopy
			}
		}
	}

	return nil
}

// SetThinkingLevel updates the active reasoning/thinking level for a specific custom model.
func (s *Store) SetThinkingLevel(modelID string, level string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	for i := range s.config.Models {
		if s.config.Models[i].ID == modelID {
			s.config.Models[i].ThinkingLevel = level
			s.config.Models[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("model %q not found", modelID)
	}
	return s.saveLocked()
}

