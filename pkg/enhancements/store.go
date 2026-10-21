package enhancements

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// Store manages loading, saving, and querying enhancements settings.
type Store struct {
	mu       sync.RWMutex
	filePath string
	config   EnhancementsConfig
}

// NewStore initializes a new Store instance with a given path or default config file.
func NewStore(customPath string) (*Store, error) {
	path := customPath
	if path == "" {
		configDir := core.GetConfigDir()
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create config dir: %w", err)
		}
		path = filepath.Join(configDir, "enhancements.json")
	}

	s := &Store{
		filePath: path,
		config:   *DefaultConfig(),
	}

	if err := s.load(); err != nil {
		// If file doesn't exist, create it with default config
		if os.IsNotExist(err) {
			if saveErr := s.save(); saveErr != nil {
				return nil, fmt.Errorf("failed to initialize enhancements file: %w", saveErr)
			}
		} else {
			return nil, fmt.Errorf("failed to read enhancements file: %w", err)
		}
	}

	return s, nil
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var cfg EnhancementsConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	if cfg.Version == "" {
		cfg.Version = "1.0.0"
	}
	if cfg.PromptJumpBar.DashWidth <= 0 {
		cfg.PromptJumpBar.DashWidth = 14
	}
	if cfg.PromptJumpBar.DashThickness <= 0 {
		cfg.PromptJumpBar.DashThickness = 2.5
	}
	if cfg.PromptJumpBar.InactiveThickness <= 0 {
		cfg.PromptJumpBar.InactiveThickness = 1.5
	}
	if cfg.OverviewPanel.DivisionStyle == "" {
		cfg.OverviewPanel = DefaultConfig().OverviewPanel
	}
	if cfg.OverviewPanel.AuxTabsFormat == "" {
		cfg.OverviewPanel.AuxTabsFormat = "icon"
	}
	if cfg.LeftPanelExtensionsMode == "" {
		cfg.LeftPanelExtensionsMode = "single"
	}
	if cfg.AutomationTargetProjectEnabled == nil {
		t := true
		cfg.AutomationTargetProjectEnabled = &t
	}
	if cfg.ContextWindowIndicatorEnabled == nil {
		t := true
		cfg.ContextWindowIndicatorEnabled = &t
	}
	if cfg.ContextWindowClickToCompactEnabled == nil {
		t := true
		cfg.ContextWindowClickToCompactEnabled = &t
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err == nil {
		if opRaw, ok := raw["overview_panel"].(map[string]interface{}); ok {
			if _, ok := opRaw["replace_see_all_triangle"]; !ok {
				cfg.OverviewPanel.ReplaceSeeAllTriangle = true
			}
		} else {
			cfg.OverviewPanel.ReplaceSeeAllTriangle = true
		}
		if _, ok := raw["left_panel_extensions_enabled"]; !ok {
			cfg.LeftPanelExtensionsEnabled = true
		}
		if _, ok := raw["main_section_extensions_enabled"]; !ok {
			cfg.MainSectionExtensionsEnabled = true
		}
	}

	// Prompt preview tooltip on hover and scroll synchronization are permanently enabled core features
	cfg.PromptJumpBar.ShowTooltip = true
	cfg.PromptJumpBar.SyncScroll = true

	// Migrate legacy global extension toggles into the per-extension visibility map
	migratedExtensions := normalizeExtensions(&cfg)

	if cfg.CommunityExtensions == nil {
		cfg.CommunityExtensions = []CommunityExtension{}
	}
	if cfg.Extensions == nil {
		cfg.Extensions = make(map[string]ExtensionVisibility)
	}
	for i := range cfg.CommunityExtensions {
		ext := &cfg.CommunityExtensions[i]
		if vis, ok := cfg.Extensions[ext.ID]; ok {
			ext.AuxPanel = vis.AuxPanel
			ext.MainPage = vis.MainPage
		} else {
			cfg.Extensions[ext.ID] = ExtensionVisibility{
				AuxPanel: ext.AuxPanel,
				MainPage: ext.MainPage,
			}
		}
	}

	s.config = cfg
	if migratedExtensions {
		// Persist the normalized config once so the map is authoritative from now on
		if err := s.save(); err != nil {
			return fmt.Errorf("failed to persist normalized extensions map: %w", err)
		}
	}
	return nil
}

// normalizeExtensions derives cfg.Extensions from the legacy global toggles when
// the map is absent/empty. Returns true when the config changed.
func normalizeExtensions(cfg *EnhancementsConfig) bool {
	if len(cfg.Extensions) > 0 {
		return false
	}
	cfg.Extensions = DefaultExtensionVisibility()
	if !cfg.LeftPanelExtensionsEnabled {
		for id, vis := range cfg.Extensions {
			vis.AuxPanel = false
			cfg.Extensions[id] = vis
		}
	}
	if !cfg.MainSectionExtensionsEnabled {
		for id, vis := range cfg.Extensions {
			vis.MainPage = false
			cfg.Extensions[id] = vis
		}
	}
	if cfg.LeftPanelExtensionsMode == "" {
		cfg.LeftPanelExtensionsMode = "single"
	}
	return true
}

func (s *Store) save() error {
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	s.config.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return err
	}

	tmpFile := fmt.Sprintf("%s.tmp.%d", s.filePath, time.Now().UnixNano())
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}

	return os.Rename(tmpFile, s.filePath)
}

// GetConfig returns a copy of current configuration.
func (s *Store) GetConfig() EnhancementsConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// UpdateConfig updates and persists new configuration.
func (s *Store) UpdateConfig(newCfg EnhancementsConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	newCfg.PromptJumpBar.ShowTooltip = true
	newCfg.PromptJumpBar.SyncScroll = true

	if newCfg.CommunityExtensions == nil && len(s.config.CommunityExtensions) > 0 {
		newCfg.CommunityExtensions = s.config.CommunityExtensions
	}
	if newCfg.ContextWindowIndicatorEnabled == nil {
		value := true
		if s.config.ContextWindowIndicatorEnabled != nil {
			value = *s.config.ContextWindowIndicatorEnabled
		}
		newCfg.ContextWindowIndicatorEnabled = &value
	}
	if newCfg.ContextWindowClickToCompactEnabled == nil {
		value := true
		if s.config.ContextWindowClickToCompactEnabled != nil {
			value = *s.config.ContextWindowClickToCompactEnabled
		}
		newCfg.ContextWindowClickToCompactEnabled = &value
	}

	if len(newCfg.Extensions) == 0 {
		// Clients that predate the per-extension map must not wipe it: keep the
		// stored map when present, otherwise derive it from the legacy toggles.
		if len(s.config.Extensions) > 0 {
			newCfg.Extensions = s.config.Extensions
		} else {
			normalizeExtensions(&newCfg)
		}
	}

	if newCfg.Extensions == nil {
		newCfg.Extensions = make(map[string]ExtensionVisibility)
	}

	// Synchronize visibility between Extensions and CommunityExtensions
	for i := range newCfg.CommunityExtensions {
		ext := &newCfg.CommunityExtensions[i]
		if vis, ok := newCfg.Extensions[ext.ID]; ok {
			ext.AuxPanel = vis.AuxPanel
			ext.MainPage = vis.MainPage
		} else {
			newCfg.Extensions[ext.ID] = ExtensionVisibility{
				AuxPanel: ext.AuxPanel,
				MainPage: ext.MainPage,
			}
		}
	}

	s.config = newCfg
	return s.save()
}

// TogglePromptJumpBar toggles the Prompt Jump Bar on or off.
func (s *Store) TogglePromptJumpBar(enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.PromptJumpBar.Enabled = enabled
	return s.save()
}

// SetPromptJumpBarColor sets the active color mode and custom color.
func (s *Store) SetPromptJumpBarColor(colorMode, customColor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.PromptJumpBar.ColorMode = colorMode
	if customColor != "" {
		s.config.PromptJumpBar.CustomColor = customColor
	}
	return s.save()
}

// SetToolDensityMode sets the tool and thinking process density mode ("normal", "muted", "hidden").
func (s *Store) SetToolDensityMode(mode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.ToolDensityMode = mode
	return s.save()
}

// SetBreakerLine toggles the chat separator breaker line.
func (s *Store) SetBreakerLine(enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.BreakerLineEnabled = enabled
	return s.save()
}

// SetDefaultNewProject sets the predefined project for new conversations.
func (s *Store) SetDefaultNewProject(project string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.DefaultNewProject = project
	return s.save()
}

// SetAuxTabsFormat sets the auxiliary extension tab switcher display format ("icon" or "icon_and_name").
func (s *Store) SetAuxTabsFormat(format string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if format != "icon_and_name" {
		format = "icon"
	}
	s.config.OverviewPanel.AuxTabsFormat = format
	return s.save()
}

// ToggleLeftPanelExtensions toggles display of extension buttons in the left navigation sidebar.
func (s *Store) ToggleLeftPanelExtensions(enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.LeftPanelExtensionsEnabled = enabled
	return s.save()
}

// SetLeftPanelExtensionsMode sets the display mode for left panel extension buttons ("single" or "individual").
func (s *Store) SetLeftPanelExtensionsMode(mode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if mode != "individual" {
		mode = "single"
	}
	s.config.LeftPanelExtensionsMode = mode
	return s.save()
}

// ToggleMainSectionExtensions toggles opening extensions in the main section vs auxiliary panel.
func (s *Store) ToggleMainSectionExtensions(enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.MainSectionExtensionsEnabled = enabled
	return s.save()
}

func sanitizeOrSlugifyID(id, name string) string {
	raw := strings.TrimSpace(id)
	if raw == "" {
		raw = strings.TrimSpace(name)
	}
	raw = strings.ToLower(raw)
	var b strings.Builder
	lastDash := false
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
			lastDash = false
		} else if r == '-' || r == ' ' || r == '.' || r == '/' || r == ':' {
			if !lastDash && b.Len() > 0 {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// GetCommunityExtensions returns a copy of registered community extensions.
func (s *Store) GetCommunityExtensions() []CommunityExtension {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.config.CommunityExtensions) == 0 {
		return []CommunityExtension{}
	}
	res := make([]CommunityExtension, len(s.config.CommunityExtensions))
	copy(res, s.config.CommunityExtensions)
	return res
}

// SaveCommunityExtension creates or updates a community extension gadget.
func (s *Store) SaveCommunityExtension(ext CommunityExtension) (CommunityExtension, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ext.Name = strings.TrimSpace(ext.Name)
	if ext.Name == "" {
		return ext, errors.New("extension name cannot be empty")
	}

	ext.ID = sanitizeOrSlugifyID(ext.ID, ext.Name)
	if ext.ID == "" {
		return ext, errors.New("extension id cannot be empty")
	}

	if IsBuiltinExtensionID(ext.ID) {
		return ext, fmt.Errorf("extension id %q conflicts with builtin extension", ext.ID)
	}

	if ext.Version == "" {
		ext.Version = "1.0.0"
	}
	if ext.Author == "" {
		ext.Author = "Community"
	}
	if ext.Icon == "" {
		ext.Icon = "blocks"
	}

	ext.SourceType = strings.ToLower(strings.TrimSpace(ext.SourceType))
	if ext.SourceType == "" {
		if strings.TrimSpace(ext.URL) != "" {
			ext.SourceType = "url"
		} else if strings.TrimSpace(ext.HTML) != "" {
			ext.SourceType = "html"
		} else {
			ext.SourceType = "url"
		}
	}

	if ext.SourceType != "url" && ext.SourceType != "html" {
		return ext, fmt.Errorf("invalid source_type %q: must be 'url' or 'html'", ext.SourceType)
	}

	if ext.SourceType == "url" && strings.TrimSpace(ext.URL) == "" {
		return ext, errors.New("url cannot be empty when source_type is url")
	}

	now := time.Now().UTC()
	foundIdx := -1
	for i, existing := range s.config.CommunityExtensions {
		if existing.ID == ext.ID {
			foundIdx = i
			break
		}
	}

	if foundIdx >= 0 {
		if ext.CreatedAt.IsZero() {
			ext.CreatedAt = s.config.CommunityExtensions[foundIdx].CreatedAt
		}
	} else {
		if ext.CreatedAt.IsZero() {
			ext.CreatedAt = now
		}
	}
	ext.UpdatedAt = now

	if foundIdx >= 0 {
		s.config.CommunityExtensions[foundIdx] = ext
	} else {
		s.config.CommunityExtensions = append(s.config.CommunityExtensions, ext)
	}

	if s.config.Extensions == nil {
		s.config.Extensions = make(map[string]ExtensionVisibility)
	}
	s.config.Extensions[ext.ID] = ExtensionVisibility{
		AuxPanel: ext.AuxPanel,
		MainPage: ext.MainPage,
	}

	if err := s.save(); err != nil {
		return ext, err
	}
	return ext, nil
}

// DeleteCommunityExtension removes a community extension by its ID.
func (s *Store) DeleteCommunityExtension(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cleanID := strings.TrimSpace(id)
	if cleanID == "" {
		return errors.New("extension id cannot be empty")
	}

	foundIdx := -1
	for i, ext := range s.config.CommunityExtensions {
		if ext.ID == cleanID {
			foundIdx = i
			break
		}
	}
	if foundIdx < 0 {
		return fmt.Errorf("community extension %q not found", cleanID)
	}

	s.config.CommunityExtensions = append(s.config.CommunityExtensions[:foundIdx], s.config.CommunityExtensions[foundIdx+1:]...)
	if s.config.Extensions != nil {
		delete(s.config.Extensions, cleanID)
	}
	return s.save()
}
