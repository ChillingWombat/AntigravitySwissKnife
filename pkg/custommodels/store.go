package custommodels

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

var (
	globalStoreMu sync.RWMutex
	globalStore   *Store
)

// SetGlobalStore sets the default global Store instance for cross-package lookups.
func SetGlobalStore(s *Store) {
	globalStoreMu.Lock()
	defer globalStoreMu.Unlock()
	globalStore = s
}

// GetGlobalStore returns the registered global Store instance, if any.
func GetGlobalStore() *Store {
	globalStoreMu.RLock()
	defer globalStoreMu.RUnlock()
	return globalStore
}

// Store manages the custom models configuration file and project bindings.
type Store struct {
	mu         sync.RWMutex
	configPath string
	config     *Config
	convModels map[string]string // convID -> modelID
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
		convModels: make(map[string]string),
	}
	SetGlobalStore(s)

	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return s, nil
}

func (s *Store) load() error {
	convPath := filepath.Join(filepath.Dir(s.configPath), "conversation_models.json")
	if cdata, err := os.ReadFile(convPath); err == nil && len(cdata) > 0 {
		var mapping map[string]string
		if err := json.Unmarshal(cdata, &mapping); err == nil {
			s.convModels = mapping
		}
	}

	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	cfg.InferenceSupported = true
	s.config = &cfg

	migrated := false
	for i := range s.config.Models {
		prevSupports := s.config.Models[i].SupportsThinking
		prevLevel := s.config.Models[i].ThinkingLevel
		prevLevelsLen := len(s.config.Models[i].ThinkingLevels)
		_ = s.config.Models[i].Validate()
		if s.config.Models[i].SupportsThinking != prevSupports || s.config.Models[i].ThinkingLevel != prevLevel || len(s.config.Models[i].ThinkingLevels) != prevLevelsLen {
			migrated = true
		}
		if s.config.Models[i].ProviderType == ProviderGCP && !s.config.Models[i].QuotaManualOverride {
			if s.config.Models[i].QuotaType == "" || s.config.Models[i].QuotaType == QuotaTypeNA || s.config.Models[i].QuotaType == QuotaNone || s.config.Models[i].QuotaType == QuotaTypeQuota {
				s.config.Models[i].QuotaType = QuotaTypeBalance
				if s.config.Models[i].BalanceValue == "" {
					s.config.Models[i].BalanceValue = "Active (GCP Billing)"
				}
				if s.config.Models[i].QuotaValue == "" || s.config.Models[i].QuotaValue == "GCP Managed" {
					s.config.Models[i].QuotaValue = s.config.Models[i].BalanceValue
				}
				migrated = true
			}
		}
		if migrateOpenCodeGoLegacyBudget(&s.config.Models[i]) {
			migrated = true
		}
		if s.config.Models[i].InternalID <= 0 {
			s.config.Models[i].InternalID = s.allocateInternalIDLocked()
			migrated = true
		}
		s.syncCustomModelToPricingLocked(s.config.Models[i])
	}
	for i := range s.config.PricingRecords {
		if s.config.PricingRecords[i].InternalID <= 0 {
			s.config.PricingRecords[i].InternalID = s.allocateInternalIDLocked()
			migrated = true
		}
	}
	if migrated {
		_ = s.saveLocked()
	}
	return nil
}

// migrateOpenCodeGoLegacyBudget removes manually entered dollar-budget fields
// that older builds attached to OpenCode Go subscription models. The provider
// reports rolling usage percentages, not a prepaid balance.
func migrateOpenCodeGoLegacyBudget(model *CustomModel) bool {
	if model == nil || model.QuotaManualOverride || model.QuotaType != QuotaTypeQuota {
		return false
	}
	if !strings.Contains(strings.ToLower(model.BaseURL), "opencode.ai/zen/go") {
		return false
	}
	hasLegacyBudget := model.TotalBudget != 0 || model.PrepaidBalance != 0
	hasLegacyCurrency := strings.HasPrefix(strings.TrimSpace(model.BalanceValue), "$") ||
		strings.HasPrefix(strings.TrimSpace(model.QuotaValue), "$")
	if !hasLegacyBudget && !hasLegacyCurrency {
		return false
	}
	model.TotalBudget = 0
	model.PrepaidBalance = 0
	model.BalanceValue = ""
	model.QuotaValue = ""
	model.QuotaFraction = nil
	return true
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

// RecordConversationModel associates an Antigravity conversation ID with the custom model ID that handled it.
func (s *Store) RecordConversationModel(convID, modelID string) {
	convID = strings.TrimSpace(convID)
	modelID = strings.TrimSpace(modelID)
	if convID == "" || modelID == "" {
		return
	}
	s.mu.Lock()
	if s.convModels == nil {
		s.convModels = make(map[string]string)
	}
	if s.convModels[convID] == modelID {
		s.mu.Unlock()
		return
	}
	s.convModels[convID] = modelID
	data, err := json.MarshalIndent(s.convModels, "", "  ")
	s.mu.Unlock()

	if err == nil {
		convPath := filepath.Join(filepath.Dir(s.configPath), "conversation_models.json")
		_ = os.WriteFile(convPath, data, 0600)
	}
}

// GetConversationModel returns the custom model ID associated with convID, or empty string.
func (s *Store) GetConversationModel(convID string) string {
	convID = strings.TrimSpace(convID)
	if convID == "" {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.convModels != nil {
		return s.convModels[convID]
	}
	return ""
}

// GetRoutedConversationModel looks up the custom model ID for convID from the global store,
// falling back to reading conversation_models.json from core.GetConfigDir().
func GetRoutedConversationModel(convID string) string {
	convID = strings.TrimSpace(convID)
	if convID == "" {
		return ""
	}
	if store := GetGlobalStore(); store != nil {
		if mID := store.GetConversationModel(convID); mID != "" {
			return mID
		}
	}
	convPath := filepath.Join(core.GetConfigDir(), "conversation_models.json")
	if data, err := os.ReadFile(convPath); err == nil && len(data) > 0 {
		var mapping map[string]string
		if err := json.Unmarshal(data, &mapping); err == nil {
			return mapping[convID]
		}
	}
	return ""
}

// GetConfig returns a deep copy of the current configuration.
func (s *Store) GetConfig() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getConfigLocked()
}

func (s *Store) getConfigLocked() Config {
	modelsCopy := make([]CustomModel, len(s.config.Models))
	for i, m := range s.config.Models {
		mCopy := m
		mCopy.QuotaFraction = cloneFloatPtr(m.QuotaFraction)
		mCopy.BudgetCapValue = cloneFloatPtr(m.BudgetCapValue)
		mCopy.InputPricePerM = cloneFloatPtr(m.InputPricePerM)
		mCopy.CachedInputPricePerM = cloneFloatPtr(m.CachedInputPricePerM)
		mCopy.OutputPricePerM = cloneFloatPtr(m.OutputPricePerM)
		mCopy.LastTestSuccess = cloneBoolPtr(m.LastTestSuccess)
		modelsCopy[i] = mCopy
	}

	pricingCopy := s.listPricingRecordsLocked()

	deletedCopy := make([]DeletedModelRecord, len(s.config.DeletedModels))
	copy(deletedCopy, s.config.DeletedModels)

	return Config{
		Version:            s.config.Version,
		InferenceSupported: true,
		ActiveModelID:      s.config.ActiveModelID,
		NextInternalID:     s.config.NextInternalID,
		Models:             modelsCopy,
		PricingRecords:     pricingCopy,
		DeletedModels:      deletedCopy,
		GatewayToken:       s.config.GatewayToken,
		Integration:        integrationSettingsFromConfig(s.config.Integration),
	}
}

// GetConfigRedacted returns a deep copy of the configuration with sensitive secrets masked.
func (s *Store) GetConfigRedacted() Config {
	cfg := s.GetConfig()
	for i := range cfg.Models {
		cfg.Models[i] = RedactCustomModel(cfg.Models[i])
	}
	return cfg
}

// ListModelsRedacted returns all configured custom models with secrets masked.
func (s *Store) ListModelsRedacted() []CustomModel {
	models := s.ListModels()
	for i := range models {
		models[i] = RedactCustomModel(models[i])
	}
	return models
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
		mCopy.LastTestSuccess = cloneBoolPtr(m.LastTestSuccess)
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
			mCopy.LastTestSuccess = cloneBoolPtr(m.LastTestSuccess)
			return &mCopy, nil
		}
	}
	return nil, fmt.Errorf("custom model with ID %q not found", id)
}

// FindModelByIdOrName retrieves a custom model by ID or by model name.
func (s *Store) FindModelByIdOrName(idOrName string) (*CustomModel, error) {
	key := strings.TrimSpace(idOrName)
	if key == "" {
		return nil, fmt.Errorf("empty model identifier")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, m := range s.config.Models {
		if m.ID == key || m.Name == key {
			mCopy := m
			mCopy.QuotaFraction = cloneFloatPtr(m.QuotaFraction)
			mCopy.BudgetCapValue = cloneFloatPtr(m.BudgetCapValue)
			mCopy.InputPricePerM = cloneFloatPtr(m.InputPricePerM)
			mCopy.CachedInputPricePerM = cloneFloatPtr(m.CachedInputPricePerM)
			mCopy.OutputPricePerM = cloneFloatPtr(m.OutputPricePerM)
			mCopy.LastTestSuccess = cloneBoolPtr(m.LastTestSuccess)
			return &mCopy, nil
		}
	}
	return nil, fmt.Errorf("custom model %q not found", key)
}

// RecordModelTestResult persists the outcome of the latest connectivity test for a model
// so the default-model gate can refuse models whose last test failed.
func (s *Store) RecordModelTestResult(id string, success bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.config.Models {
		if s.config.Models[i].ID == id {
			ok := success
			s.config.Models[i].LastTestSuccess = &ok
			s.config.Models[i].LastTestedAt = time.Now().UTC().Format(time.RFC3339)
			return s.saveLocked()
		}
	}
	return fmt.Errorf("custom model with ID %q not found", id)
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

	s.mu.Lock()
	defer s.mu.Unlock()

	// Resolve the stored record (if any) so fresh last-test state from the caller can be
	// merged with the persisted one and the default-model gate can be enforced.
	var existing *CustomModel
	for i := range s.config.Models {
		if s.config.Models[i].ID == m.ID {
			stored := s.config.Models[i]
			existing = &stored
			break
		}
	}
	if existing != nil {
		// Prefer whichever last-test record is newer; fall back to the stored one when the
		// incoming payload carries none (e.g. plain field edits from the UI).
		if m.LastTestedAt == "" || (existing.LastTestedAt != "" && existing.LastTestedAt > m.LastTestedAt) {
			m.LastTestSuccess = cloneBoolPtr(existing.LastTestSuccess)
			m.LastTestedAt = existing.LastTestedAt
		}
		// Refuse to promote a model whose latest connectivity test failed to the default model.
		if m.IsDefault && !existing.IsDefault && m.LastTestSuccess != nil && !*m.LastTestSuccess {
			return fmt.Errorf("cannot set %q as the default model: its last connectivity test failed", m.Name)
		}
	}

	for _, existing := range s.config.Models {
		if existing.ID == m.ID {
			if m.APIKey == "" || m.APIKey == "[REDACTED]" {
				m.APIKey = existing.APIKey
			}
			if m.GCPServiceAccountKey == "" || m.GCPServiceAccountKey == "[REDACTED]" {
				m.GCPServiceAccountKey = existing.GCPServiceAccountKey
			}
			if m.GCPProjectID == "" {
				m.GCPProjectID = existing.GCPProjectID
			}
			if m.GCPRegion == "" {
				m.GCPRegion = existing.GCPRegion
			}
			break
		}
	}

	if err := m.Validate(); err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	nowShort := time.Now().Format("2006-01-02 15:04")
	found := false
	for i, existing := range s.config.Models {
		if existing.ID == m.ID {
			if m.APIKey == "" || m.APIKey == "[REDACTED]" {
				m.APIKey = existing.APIKey
			}
			if m.GCPServiceAccountKey == "" || m.GCPServiceAccountKey == "[REDACTED]" {
				m.GCPServiceAccountKey = existing.GCPServiceAccountKey
			}
			if m.GCPProjectID == "" {
				m.GCPProjectID = existing.GCPProjectID
			}
			if m.GCPRegion == "" {
				m.GCPRegion = existing.GCPRegion
			}
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

	if m.IsDefault {
		for j := range s.config.Models {
			if s.config.Models[j].ID != m.ID {
				s.config.Models[j].IsDefault = false
			}
		}
	}

	s.clearDeletedModelLocked(m.Name, m.ID)
	s.syncCustomModelToPricingLocked(m)

	return s.saveLocked()
}

// ClearDefaultFlags resets the stored default marker on every model so the store
// stops advertising a default that the routing config no longer points at.
func (s *Store) ClearDefaultFlags() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	changed := false
	for i := range s.config.Models {
		if s.config.Models[i].IsDefault {
			s.config.Models[i].IsDefault = false
			changed = true
		}
	}
	if !changed {
		return nil
	}
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

	return s.saveLocked()
}

// IntegrationSettings returns the Model Source settings, filled with defaults for
// any missing field so callers never see a partially-initialised struct.
func (s *Store) IntegrationSettings() IntegrationSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return integrationSettingsFromConfig(s.config.Integration)
}

func integrationSettingsFromConfig(cfg IntegrationSettings) IntegrationSettings {
	out := DefaultIntegrationSettings()
	if cfg.Mode == IntegrationProfileIsolation || cfg.Mode == IntegrationAPIProxy {
		out.Mode = cfg.Mode
	}
	for name, profile := range cfg.Apps {
		merged := out.Apps[name]
		if profile.Source == ModelSourceNative || profile.Source == ModelSourceCustom {
			merged.Source = profile.Source
		}
		if profile.Subagents == SubagentsApp || profile.Subagents == SubagentsCLI {
			merged.Subagents = profile.Subagents
		}
		out.Apps[name] = merged
	}
	switch cfg.Proxy.Scope {
	case ProxyScopeAntigravity, ProxyScopeExternal:
		out.Proxy.Scope = cfg.Proxy.Scope
	}
	out.Proxy.PIN = cfg.Proxy.PIN
	return out
}

// SetIntegrationSettings validates and persists the Model Source settings.
func (s *Store) SetIntegrationSettings(in IntegrationSettings) error {
	if in.Mode != IntegrationProfileIsolation && in.Mode != IntegrationAPIProxy {
		return fmt.Errorf("unsupported integration mode %q", in.Mode)
	}
	for name, profile := range in.Apps {
		if profile.Source != ModelSourceNative && profile.Source != ModelSourceCustom {
			return fmt.Errorf("app %q: unsupported model source %q", name, profile.Source)
		}
		if profile.Subagents != SubagentsApp && profile.Subagents != SubagentsCLI {
			return fmt.Errorf("app %q: unsupported subagent target %q", name, profile.Subagents)
		}
	}
	if in.Proxy.Scope != ProxyScopeAntigravity && in.Proxy.Scope != ProxyScopeExternal {
		return fmt.Errorf("unsupported proxy scope %q", in.Proxy.Scope)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Integration = in
	return s.saveLocked()
}

// IsProxyModeActive reports whether API proxy integration is active or any app
// profile routes to the custom model proxy.
func (s *Store) IsProxyModeActive() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.config.Integration.Mode == IntegrationAPIProxy {
		return true
	}
	if s.config.ActiveModelID != "" {
		return true
	}
	for _, app := range s.config.Integration.Apps {
		if app.Source == ModelSourceCustom {
			return true
		}
	}
	return false
}

// GatewayToken returns the bearer token for the local LLM gateway, creating and
// persisting one on first use.
func (s *Store) GatewayToken() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.config == nil {
		return ""
	}
	if strings.TrimSpace(s.config.GatewayToken) != "" {
		return s.config.GatewayToken
	}
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	s.config.GatewayToken = hex.EncodeToString(buf)
	if err := s.saveLocked(); err != nil {
		return ""
	}
	return s.config.GatewayToken
}

// RotateGatewayToken invalidates the current gateway token.
func (s *Store) RotateGatewayToken() (string, error) {
	s.mu.Lock()
	s.config.GatewayToken = ""
	s.mu.Unlock()
	return s.GatewayToken(), nil
}

// SetActiveModel records the custom model the app currently uses. An empty
// modelID clears it and hands the session back to the native model. Selection is
// app-wide on purpose: models are not bound to projects.
func (s *Store) SetActiveModel(modelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	trimmed := strings.TrimSpace(modelID)
	if trimmed != "" && trimmed != "native" {
		found := false
		for _, m := range s.config.Models {
			if m.ID == trimmed || m.Name == trimmed {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("cannot activate unknown model ID: %s", trimmed)
		}
	}
	if trimmed == "native" {
		trimmed = ""
	}
	s.config.ActiveModelID = trimmed
	return s.saveLocked()
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
