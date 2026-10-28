package custommodels

import (
	"errors"
	"math"
	"strconv"
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
	ProviderGCP       ProviderType = "gcp"
)

// QuotaType defines how remaining usage is calculated.
type QuotaType string

const (
	QuotaTypeNA      QuotaType = "na"      // N/A - untracked
	QuotaTypeBalance QuotaType = "balance" // Balance - fiat balance
	QuotaTypeQuota   QuotaType = "quota"   // Quota - quota / rate limits

	// Legacy backward-compatibility constants
	QuotaCostBased  QuotaType = "cost_based"
	QuotaQuotaBased QuotaType = "quota_based"
	QuotaNone       QuotaType = "none"
)

// CustomModel defines a third-party or local LLM configured in Antigravity.
type CustomModel struct {
	InternalID           int64        `json:"internal_id,omitempty"`
	ID                   string       `json:"id"`
	Name                 string       `json:"name"`
	DisplayName          string       `json:"display_name"`
	ProviderType         ProviderType `json:"provider_type"`
	BaseURL              string       `json:"base_url"`
	APIKey               string       `json:"api_key,omitempty"`
	GCPProjectID         string       `json:"gcp_project_id,omitempty"`
	GCPRegion            string       `json:"gcp_region,omitempty"`
	GCPServiceAccountKey string       `json:"gcp_service_account_key,omitempty"`
	CustomQuotaEndpoint  string       `json:"custom_quota_endpoint,omitempty"`
	QuotaType            QuotaType    `json:"quota_type"`
	QuotaManualOverride  bool         `json:"quota_manual_override,omitempty"`
	BalanceValue         string       `json:"balance_value,omitempty"` // Formatted fiat e.g. "$12.34" or "¥10.00"
	QuotaValue           string       `json:"quota_value,omitempty"`   // Formatted value e.g. "250,000 tokens" or "$10.00"
	PrepaidBalance       float64      `json:"prepaid_balance"`         // Funds remaining in USD
	TotalBudget          float64      `json:"total_budget"`            // Total initial/prepaid budget in USD
	TokenLimit           int64        `json:"token_limit,omitempty"`   // Total token quota limit when quota_type is "quota"
	QuotaFraction        *float64     `json:"quota_fraction"`          // 0.0 to 1.0; nil if untracked/none
	// QuotaBuckets carries every quota window reported by the provider
	// (5H + 7D/monthly ...). Models with a single window leave this empty.
	QuotaBuckets         []QuotaBucket `json:"quota_buckets,omitempty"`
	BudgetCapType        string        `json:"budget_cap_type,omitempty"` // "none", "dollar", "percentage", "tokens"
	BudgetCapValue       *float64      `json:"budget_cap_value"`          // Cap threshold in USD, %, or tokens
	InputPricePerM       *float64      `json:"input_price_per_m"`
	CachedInputPricePerM *float64      `json:"cached_input_price_per_m"`
	OutputPricePerM      *float64      `json:"output_price_per_m"`
	PriceSource          string        `json:"price_source,omitempty"`
	PriceUpdatedAt       string        `json:"price_updated_at,omitempty"`
	IsDefault            bool          `json:"is_default"`
	LastTestSuccess      *bool         `json:"last_test_success,omitempty"` // nil = never tested
	LastTestedAt         string        `json:"last_tested_at,omitempty"`
	ContextWindow        int           `json:"context_window,omitempty"`
	SupportsThinking     bool          `json:"supports_thinking,omitempty"`
	ThinkingLevels       []string      `json:"thinking_levels,omitempty"` // e.g. ["off", "low", "medium", "high"]
	ThinkingLevel        string        `json:"thinking_level,omitempty"`  // Active level e.g. "medium", "off"
	Enabled              bool          `json:"enabled"`
	Notes                string        `json:"notes,omitempty"` // User notes or description
	SecurityRiskLevel    string        `json:"security_risk_level,omitempty"`
	SecurityAuditScore   int           `json:"security_audit_score,omitempty"`
	SecurityGrade        string        `json:"security_grade,omitempty"`
	LastSecurityAudit    string        `json:"last_security_audit,omitempty"`
	CreatedAt            string        `json:"created_at,omitempty"`
	UpdatedAt            string        `json:"updated_at,omitempty"`
}

// Config holds the full custom models configuration file structure.
type Config struct {
	Version              string `json:"version"`
	InferenceSupported   bool   `json:"inference_supported"`
	GeminiQuotaExhausted bool   `json:"gemini_quota_exhausted"`
	DefaultCustomModel   string `json:"default_custom_model,omitempty"`
	ActiveModelID        string `json:"active_model_id,omitempty"`
	// GatewayToken authorises Antigravity surfaces that call the local LLM
	// gateway (AGY_LLM_GATEWAY_API_KEY). Generated on first use.
	GatewayToken string `json:"gateway_token,omitempty"`
	// Integration is how custom models reach the Antigravity surfaces.
	Integration    IntegrationSettings  `json:"integration,omitempty"`
	NextInternalID int64                `json:"next_internal_id,omitempty"`
	Models         []CustomModel        `json:"models"`
	PricingRecords []ModelPricingRecord `json:"pricing_records,omitempty"`
	DeletedModels  []DeletedModelRecord `json:"deleted_models,omitempty"`
}

// Integration modes.
const (
	// IntegrationProfileIsolation gives each app profile a single catalog and runs
	// custom-model profiles separately from the native one.
	IntegrationProfileIsolation = "profile_isolation"
	// IntegrationAPIProxy routes the apps through the local proxy so both model
	// types can share one catalog.
	IntegrationAPIProxy = "api_proxy"

	// ModelSourceNative keeps the app on Antigravity's own model catalog.
	ModelSourceNative = "native"
	// ModelSourceCustom points the app at the Swiss Knife model gateway.
	ModelSourceCustom = "custom"

	// SubagentsApp runs subagents inside the app that asked for them.
	SubagentsApp = "app"
	// SubagentsCLI delegates subagent work to the Antigravity CLI.
	SubagentsCLI = "cli"

	// ProxyScopeAntigravity exposes the proxy to Antigravity surfaces only.
	ProxyScopeAntigravity = "antigravity"
	// ProxyScopeExternal also exposes it to outside agents, with PIN auth.
	ProxyScopeExternal = "external"
)

// AppProfile is the model source for one Antigravity surface.
type AppProfile struct {
	Source    string `json:"source"`    // native | custom
	Subagents string `json:"subagents"` // app | cli
}

// IntegrationSettings holds the Model Source tab state.
type IntegrationSettings struct {
	Mode  string                `json:"mode"` // profile_isolation | api_proxy
	Apps  map[string]AppProfile `json:"apps"` // antigravity | vscode
	Proxy ProxySettings         `json:"proxy"`
}

// ProxySettings configures the API proxy mode.
type ProxySettings struct {
	Scope string `json:"scope"` // antigravity | external
	PIN   string `json:"pin,omitempty"`
}

// DefaultIntegrationSettings is the shipped default: profile isolation, native
// models everywhere, subagents inside their own app.
func DefaultIntegrationSettings() IntegrationSettings {
	return IntegrationSettings{
		Mode: IntegrationProfileIsolation,
		Apps: map[string]AppProfile{
			"antigravity": {Source: ModelSourceNative, Subagents: SubagentsApp},
			"vscode":      {Source: ModelSourceNative, Subagents: SubagentsApp},
		},
		Proxy: ProxySettings{Scope: ProxyScopeAntigravity},
	}
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
	if m.ProviderType == ProviderGCP {
		if strings.TrimSpace(m.GCPProjectID) == "" && strings.TrimSpace(m.GCPServiceAccountKey) != "" && m.GCPServiceAccountKey != "[REDACTED]" {
			if key, err := ParseServiceAccountKey([]byte(m.GCPServiceAccountKey)); err == nil && key.ProjectID != "" {
				m.GCPProjectID = key.ProjectID
			}
		}
		if strings.TrimSpace(m.GCPProjectID) == "" && strings.Contains(m.BaseURL, "/projects/") {
			rem := m.BaseURL[strings.Index(m.BaseURL, "/projects/")+len("/projects/"):]
			if slash := strings.Index(rem, "/"); slash != -1 {
				m.GCPProjectID = rem[:slash]
			}
		}
		region := strings.TrimSpace(m.GCPRegion)
		if region == "" || (strings.Contains(strings.ToLower(m.Name), "claude") && region == "us-central1") {
			if strings.Contains(strings.ToLower(m.Name), "claude") {
				region = "us-east5"
			} else {
				region = "us-central1"
			}
		}
		m.GCPRegion = region
		if strings.TrimSpace(m.BaseURL) == "" || strings.Contains(m.BaseURL, "aiplatform.googleapis.com") {
			m.BaseURL = BuildVertexAIEndpoint(region, m.GCPProjectID, m.Name)
		}
		if !m.QuotaManualOverride && (m.QuotaType == "" || m.QuotaType == QuotaTypeNA || m.QuotaType == QuotaNone || m.QuotaType == QuotaTypeQuota) {
			m.QuotaType = QuotaTypeBalance
			if m.BalanceValue == "" {
				m.BalanceValue = "Active (GCP Billing)"
			}
			if m.QuotaValue == "" || m.QuotaValue == "GCP Managed" {
				m.QuotaValue = m.BalanceValue
			}
		}
	}
	if strings.TrimSpace(m.BaseURL) == "" {
		return errors.New("base_url is required")
	}
	if m.ContextWindow <= 0 {
		m.ContextWindow = 1048576
	}
	if m.QuotaType == "" || m.QuotaType == QuotaNone {
		m.QuotaType = QuotaTypeNA
	} else if m.QuotaType == QuotaCostBased {
		m.QuotaType = QuotaTypeBalance
	} else if m.QuotaType == QuotaQuotaBased {
		m.QuotaType = QuotaTypeQuota
	}
	switch strings.ToLower(strings.TrimSpace(m.BudgetCapType)) {
	case "dollar", "usd", "cost":
		m.BudgetCapType = "dollar"
	case "percentage", "percent", "%":
		m.BudgetCapType = "percentage"
	case "tokens", "token":
		m.BudgetCapType = "tokens"
	default:
		m.BudgetCapType = "none"
		m.BudgetCapValue = nil
	}
	var cleanLevels []string
	for _, l := range m.ThinkingLevels {
		trimmed := strings.ToLower(strings.TrimSpace(l))
		if trimmed != "" && trimmed != "off" {
			cleanLevels = append(cleanLevels, trimmed)
		}
	}
	if m.ProviderType == ProviderGCP {
		if _, detectedThinking, _ := detectModelMetadata(m.Name, m.DisplayName, m.ContextWindow); detectedThinking {
			m.SupportsThinking = true
		}
	}
	if m.SupportsThinking || len(cleanLevels) > 0 || (strings.TrimSpace(m.ThinkingLevel) != "" && !strings.EqualFold(m.ThinkingLevel, "off")) {
		m.SupportsThinking = true
		if strings.TrimSpace(m.ThinkingLevel) == "" || strings.EqualFold(m.ThinkingLevel, "off") {
			m.ThinkingLevel = "high"
		} else {
			m.ThinkingLevel = strings.ToLower(strings.TrimSpace(m.ThinkingLevel))
		}
		if len(cleanLevels) < 2 {
			cleanLevels = []string{"low", "medium", "high"}
		}
		hasActive := false
		for _, l := range cleanLevels {
			if l == m.ThinkingLevel {
				hasActive = true
				break
			}
		}
		if !hasActive && m.ThinkingLevel != "" {
			cleanLevels = append(cleanLevels, m.ThinkingLevel)
		}
		m.ThinkingLevels = cleanLevels
	} else if strings.EqualFold(m.ThinkingLevel, "off") {
		m.SupportsThinking = false
		m.ThinkingLevel = "off"
	}
	return nil
}

// CalculatePercentage computes the percentage to display in the circular gauge.
// Returns (percentage, hasInfo). If hasInfo is false, the gauge should be rendered empty grey with N/A.
func (m *CustomModel) CalculatePercentage() (int, bool) {
	switch m.QuotaType {
	case QuotaTypeBalance, QuotaCostBased:
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
		if m.PrepaidBalance > 0 {
			return 100, true
		}
		if m.BalanceValue != "" {
			clean := strings.TrimLeft(strings.TrimSpace(m.BalanceValue), "$¥€£")
			clean = strings.TrimSpace(clean)
			if val, err := strconv.ParseFloat(clean, 64); err == nil && val > 0 {
				return 100, true
			}
		}
		return 0, false

	case QuotaTypeQuota, QuotaQuotaBased:
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

	case QuotaTypeNA, QuotaNone:
		fallthrough
	default:
		return 0, false
	}
}

// DefaultConfig returns an initial empty configuration.
func DefaultConfig() *Config {
	return &Config{
		Version:            "1.0.0",
		InferenceSupported: true,
		Integration:        DefaultIntegrationSettings(),
		Models:             []CustomModel{},
	}
}

// SamplePresetModels returns a pre-configured starter set for demonstration.
func SamplePresetModels() []CustomModel {
	frac := 0.85
	now := time.Now().UTC().Format(time.RFC3339)
	return []CustomModel{
		{
			ID:             "openai-gpt4o",
			Name:           "gpt-4o",
			DisplayName:    "GPT-4o",
			ProviderType:   ProviderOpenAI,
			BaseURL:        "https://api.openai.com/v1",
			APIKey:         "sk-demo-key-configured",
			QuotaType:      QuotaTypeBalance,
			BalanceValue:   "$34.50",
			PrepaidBalance: 34.50,
			TotalBudget:    50.00,
			QuotaFraction:  nil,
			IsDefault:      true,
			ContextWindow:  128000,
			Enabled:        true,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ID:             "anthropic-claude-37",
			Name:           "claude-3-7-sonnet-20250219",
			DisplayName:    "Claude 3.7 Sonnet",
			ProviderType:   ProviderAnthropic,
			BaseURL:        "https://api.anthropic.com/v1",
			APIKey:         "sk-ant-demo-key",
			QuotaType:      QuotaTypeQuota,
			QuotaValue:     "200,000 tokens",
			PrepaidBalance: 0,
			TotalBudget:    0,
			QuotaFraction:  &frac,
			IsDefault:      false,
			ContextWindow:  200000,
			Enabled:        true,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			ID:             "ollama-local-llama3",
			Name:           "llama3.3:70b",
			DisplayName:    "Llama 3.3 70B (Local)",
			ProviderType:   ProviderLocal,
			BaseURL:        "http://localhost:11434/v1",
			APIKey:         "",
			QuotaType:      QuotaTypeNA, // Local has no quota limit -> Empty grey ring, N/A
			PrepaidBalance: 0,
			TotalBudget:    0,
			QuotaFraction:  nil,
			IsDefault:      false,
			ContextWindow:  131072,
			Enabled:        true,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}
}

// RedactCustomModel returns a copy of CustomModel with sensitive API secrets masked.
func RedactCustomModel(m CustomModel) CustomModel {
	r := m
	if r.APIKey != "" {
		r.APIKey = "[REDACTED]"
	}
	if r.GCPServiceAccountKey != "" {
		r.GCPServiceAccountKey = "[REDACTED]"
	}
	return r
}
