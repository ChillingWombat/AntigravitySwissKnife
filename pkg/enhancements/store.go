package enhancements

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

	s.config = cfg
	return nil
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



