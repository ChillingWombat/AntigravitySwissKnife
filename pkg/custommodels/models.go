package custommodels

import (
	"errors"
	"math"
	"strings"
	"time"
)

// ProviderType represents the API protocol used to communicate with the model.
type ProviderType string

const (
	ProviderOpenAI    ProviderType = "openai"
	ProviderAnthropic ProviderType = "anthropic"
	ProviderGemini    ProviderType = "gemini"
	ProviderCustom    ProviderType = "custom"
	ProviderLocal     ProviderType = "local"
)

// QuotaType defines how remaining usage is calculated.
type QuotaType string

const (
	QuotaCostBased  QuotaType = "cost_based"  // Progress based on prepaid deposit funds left
	QuotaQuotaBased QuotaType = "quota_based" // Progress based on percentage/fraction
	QuotaNone       QuotaType = "none"        // Untracked / no quota info -> empty grey ring, N/A
)

// CustomModel defines a third-party or local LLM configured in Antigravity.
type CustomModel struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	DisplayName      string       `json:"display_name"`
	ProviderType     ProviderType `json:"provider_type"`
	BaseURL          string       `json:"base_url"`
	APIKey           string       `json:"api_key,omitempty"`
	ProjectMappings  []string     `json:"project_mappings"` // Specific projects or ["*"] for all
	QuotaType        QuotaType    `json:"quota_type"`
	PrepaidBalance   float64      `json:"prepaid_balance"` // Funds remaining in USD
	TotalBudget      float64      `json:"total_budget"`    // Total initial/prepaid budget in USD
	QuotaFraction    *float64     `json:"quota_fraction"`  // 0.0 to 1.0; nil if untracked/none
	IsDefault        bool         `json:"is_default"`
	ContextWindow    int          `json:"context_window,omitempty"`
	SupportsThinking bool         `json:"supports_thinking,omitempty"`
	ThinkingLevels   []string     `json:"thinking_levels,omitempty"` // e.g. ["off", "low", "medium", "high"]
	ThinkingLevel    string       `json:"thinking_level,omitempty"`    // Active level e.g. "medium", "off"
	Enabled          bool         `json:"enabled"`
	CreatedAt        string       `json:"created_at,omitempty"`
	UpdatedAt        string       `json:"updated_at,omitempty"`
}

// Config holds the full custom models configuration file structure.
type Config struct {
	Version       string            `json:"version"`
	ActiveModelID string            `json:"active_model_id,omitempty"`
	Models        []CustomModel     `json:"models"`
	ProjectBinds  map[string]string `json:"project_binds"` // project name -> model ID
}

// Validate checks model fields for completeness and validity.
func (m *CustomModel) Validate() error {
	if strings.TrimSpace(m.ID) == "" {
		return errors.New("model id is required")
	}
	if strings.TrimSpace(m.Name) == "" {
		return errors.New("model name is required")
	}
	if strings.TrimSpace(m.DisplayName) == "" {
		m.DisplayName = m.Name
	}
	if m.ProviderType == "" {
		m.ProviderType = ProviderOpenAI
	}
	if strings.TrimSpace(m.BaseURL) == "" {
		return errors.New("base_url is required")
	}
	if m.ContextWindow <= 0 {
		m.ContextWindow = 1000000
	}
	if m.QuotaType == "" {
		m.QuotaType = QuotaNone
	}
	if len(m.ProjectMappings) == 0 {
		m.ProjectMappings = []string{"*"}
	}
	if strings.EqualFold(m.ThinkingLevel, "off") {
		m.SupportsThinking = false
		m.ThinkingLevel = "off"
	} else if m.SupportsThinking || (strings.TrimSpace(m.ThinkingLevel) != "" && !strings.EqualFold(m.ThinkingLevel, "off")) {
		m.SupportsThinking = true
		if strings.TrimSpace(m.ThinkingLevel) == "" {
			m.ThinkingLevel = "high"
		}
		if len(m.ThinkingLevels) == 0 {
			m.ThinkingLevels = []string{"off", "low", "medium", "high"}
		}
	}
	return nil
}

// CalculatePercentage computes the percentage to display in the circular gauge.
// Returns (percentage, hasInfo). If hasInfo is false, the gauge should be rendered empty grey with N/A.
func (m *CustomModel) CalculatePercentage() (int, bool) {
	switch m.QuotaType {
	case QuotaCostBased:
		if m.TotalBudget > 0 {
			fraction := m.PrepaidBalance / m.TotalBudget
			if fraction < 0 {
				fraction = 0
			}
			if fraction > 1 {
				fraction = 1
			}
			return int(math.Round(fraction * 100)), true
		}
		return 0, false

	case QuotaQuotaBased:
		if m.QuotaFraction != nil {
			f := *m.QuotaFraction
			if f < 0 {
				f = 0
			}
			if f > 1 {
				f = 1
			}
			return int(math.Round(f * 100)), true
		}
		return 0, false

	case QuotaNone:
		fallthrough
	default:
		return 0, false
	}
}

// MatchesProject checks if this model is mapped to a specific project.
func (m *CustomModel) MatchesProject(projectName string) bool {
	if !m.Enabled {
		return false
	}
	target := strings.TrimSpace(projectName)
	for _, p := range m.ProjectMappings {
		trimmed := strings.TrimSpace(p)
		if trimmed == "*" || strings.EqualFold(trimmed, target) {
			return true
		}
	}
	return false
}

// DefaultConfig returns an initial empty configuration.
func DefaultConfig() *Config {
	return &Config{
		Version:      "1.0.0",
		Models:       []CustomModel{},
		ProjectBinds: make(map[string]string),
	}
}

// SamplePresetModels returns a pre-configured starter set for demonstration.
func SamplePresetModels() []CustomModel {
	frac := 0.85
	now := time.Now().UTC().Format(time.RFC3339)
	return []CustomModel{
		{
			ID:              "openai-gpt4o",
			Name:            "gpt-4o",
			DisplayName:     "GPT-4o",
			ProviderType:    ProviderOpenAI,
			BaseURL:         "https://api.openai.com/v1",
			APIKey:          "sk-demo-key-configured",
			ProjectMappings: []string{"*"},
			QuotaType:       QuotaCostBased,
			PrepaidBalance:  34.50,
			TotalBudget:     50.00,
			QuotaFraction:   nil,
			IsDefault:       true,
			ContextWindow:   128000,
			Enabled:         true,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "anthropic-claude-37",
			Name:            "claude-3-7-sonnet-20250219",
			DisplayName:     "Claude 3.7 Sonnet",
			ProviderType:    ProviderAnthropic,
			BaseURL:         "https://api.anthropic.com/v1",
			APIKey:          "sk-ant-demo-key",
			ProjectMappings: []string{"*"},
			QuotaType:       QuotaQuotaBased,
			PrepaidBalance:  0,
			TotalBudget:     0,
			QuotaFraction:   &frac,
			IsDefault:       false,
			ContextWindow:   200000,
			Enabled:         true,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              "ollama-local-llama3",
			Name:            "llama3.3:70b",
			DisplayName:     "Llama 3.3 70B (Local)",
			ProviderType:    ProviderLocal,
			BaseURL:         "http://localhost:11434/v1",
			APIKey:          "",
			ProjectMappings: []string{"Antigravity Swiss Knife"},
			QuotaType:       QuotaNone, // Local has no quota limit -> Empty grey ring, N/A
			PrepaidBalance:  0,
			TotalBudget:     0,
			QuotaFraction:   nil,
			IsDefault:       false,
			ContextWindow:   131072,
			Enabled:         true,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}
}
