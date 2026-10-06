package quota

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

func TestGetAvailableModelCatalog_Defaults(t *testing.T) {
	cat := GetAvailableModelCatalog(nil, true)
	if cat == nil {
		t.Fatalf("expected non-nil catalog")
	}
	if !cat.Success {
		t.Errorf("expected success true")
	}
	if cat.DefaultGemini != "gemini-3.8-flash-high" {
		t.Errorf("expected default gemini 'gemini-3.8-flash-high', got '%s'", cat.DefaultGemini)
	}
	if cat.DefaultNonGemini != "claude-opus-4-6" {
		t.Errorf("expected default non-gemini 'claude-opus-4-6', got '%s'", cat.DefaultNonGemini)
	}
	if len(cat.GeminiModels) == 0 {
		t.Errorf("expected non-empty GeminiModels")
	}
	if len(cat.NonGeminiModels) == 0 {
		t.Errorf("expected non-empty NonGeminiModels")
	}

	// Verify first item of Gemini models is gemini-3.8-flash-high
	if cat.GeminiModels[0].ID != "gemini-3.8-flash-high" {
		t.Errorf("expected first Gemini model to be 'gemini-3.8-flash-high', got '%s'", cat.GeminiModels[0].ID)
	}

	// Verify first item of NonGemini models is claude-opus-4-6
	if cat.NonGeminiModels[0].ID != "claude-opus-4-6" {
		t.Errorf("expected first Non-Gemini model to be 'claude-opus-4-6', got '%s'", cat.NonGeminiModels[0].ID)
	}
}

func TestMergeLiveModels(t *testing.T) {
	cat := &AvailableModelsCatalog{
		GeminiModels:    DefaultBaseGeminiModels(),
		NonGeminiModels: DefaultBaseNonGeminiModels(),
	}
	initialGeminiCount := len(cat.GeminiModels)
	initialNonGeminiCount := len(cat.NonGeminiModels)

	live := []ModelOption{
		{
			ID:          "gemini-ultra-4-0",
			DisplayName: "Gemini Ultra 4.0",
		},
		{
			ID:          "claude-4-sonnet",
			DisplayName: "Claude 4 Sonnet",
		},
		{
			ID:          "gemini-3.8-flash", // duplicate
			DisplayName: "Gemini 3.8 Flash",
		},
	}

	mergeLiveModels(cat, live)

	if len(cat.GeminiModels) != initialGeminiCount+1 {
		t.Errorf("expected %d gemini models, got %d", initialGeminiCount+1, len(cat.GeminiModels))
	}
	if len(cat.NonGeminiModels) != initialNonGeminiCount+1 {
		t.Errorf("expected %d non-gemini models, got %d", initialNonGeminiCount+1, len(cat.NonGeminiModels))
	}
}

func TestGetAvailableModelCatalog_LiveMock(t *testing.T) {
	origModelsURLs := CloudCodeModelsURLs
	defer func() { CloudCodeModelsURLs = origModelsURLs }()

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"defaultAgentModelId": "gemini-3.8-flash",
			"tieredModelIds": map[string]interface{}{
				"flash": []string{"gemini-3.8-flash", "gemini-2.5-flash"},
				"pro":   []string{"gemini-3.8-pro", "gemini-experimental-2026"},
			},
			"models": map[string]interface{}{
				"gemini-3.8-flash": map[string]interface{}{
					"displayName":      "Gemini 3.8 Flash",
					"supportsThinking": true,
					"recommended":      true,
				},
				"claude-opus-4-6": map[string]interface{}{
					"displayName":      "Claude Opus 4.6",
					"supportsThinking": true,
					"recommended":      true,
				},
				"claude-experimental-preview": map[string]interface{}{
					"displayName":      "Claude Experimental Preview",
					"supportsThinking": true,
				},
			},
		})
	}))
	defer mockServer.Close()
	CloudCodeModelsURLs = []string{mockServer.URL}

	acc := &keyring.Account{
		Email:       "tester@example.com",
		AccessToken: "mock-valid-token",
	}

	cat := GetAvailableModelCatalog(acc, true)
	if cat == nil || !cat.Success {
		t.Fatalf("expected successful catalog")
	}

	if cat.DefaultGemini != "gemini-3.8-flash" {
		t.Errorf("expected default gemini 'gemini-3.8-flash', got '%s'", cat.DefaultGemini)
	}
	if cat.DefaultNonGemini != "claude-opus-4-6" {
		t.Errorf("expected default non-gemini 'claude-opus-4-6', got '%s'", cat.DefaultNonGemini)
	}

	// Verify live-discovered models were merged
	foundGeminiExp := false
	for _, m := range cat.GeminiModels {
		if m.ID == "gemini-experimental-2026" {
			foundGeminiExp = true
			break
		}
	}
	if !foundGeminiExp {
		t.Errorf("expected live-discovered 'gemini-experimental-2026' in GeminiModels")
	}

	foundClaudeExp := false
	for _, m := range cat.NonGeminiModels {
		if m.ID == "claude-experimental-preview" {
			foundClaudeExp = true
			break
		}
	}
	if !foundClaudeExp {
		t.Errorf("expected live-discovered 'claude-experimental-preview' in NonGeminiModels")
	}
}

func TestGetAvailableModelCatalog_NetworkFailureFallback(t *testing.T) {
	origModelsURLs := CloudCodeModelsURLs
	defer func() { CloudCodeModelsURLs = origModelsURLs }()

	// Point to non-routable address
	CloudCodeModelsURLs = []string{"http://127.0.0.1:54321/invalid"}

	acc := &keyring.Account{
		Email:       "tester@example.com",
		AccessToken: "mock-token",
	}

	cat := GetAvailableModelCatalog(acc, true)
	if cat == nil || !cat.Success {
		t.Fatalf("expected fallback catalog with success=true")
	}

	if cat.DefaultGemini != "gemini-3.8-flash-high" {
		t.Errorf("expected fallback default gemini 'gemini-3.8-flash-high', got '%s'", cat.DefaultGemini)
	}
	if cat.DefaultNonGemini != "claude-opus-4-6" {
		t.Errorf("expected fallback default non-gemini 'claude-opus-4-6', got '%s'", cat.DefaultNonGemini)
	}
	if len(cat.GeminiModels) == 0 || len(cat.NonGeminiModels) == 0 {
		t.Errorf("expected non-empty baseline models on failure")
	}
}

func TestGetAvailableModelCatalog_ExcludeInternalSubsystemsAndCategorize(t *testing.T) {
	origModelsURLs := CloudCodeModelsURLs
	defer func() { CloudCodeModelsURLs = origModelsURLs }()

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"defaultAgentModelId": "gemini-3.8-flash-high",
			"tabModelIds":         []string{"chat_20706", "chat_23310"},
			"commandModelIds":     []string{"gemini-3-flash"},
			"imageGenerationModelIds": []string{"gemini-3.1-flash-image"},
			"commitMessageModelIds":   []string{"gemini-3.5-flash-lite"},
			"deprecatedModelIds": map[string]interface{}{
				"gemini-3.1-pro-high": map[string]interface{}{
					"newModelId": "gemini-pro-agent",
				},
			},
			"tieredModelIds": map[string]interface{}{
				"flash": []string{"gemini-3.8-flash-tiered"},
				"pro":   []string{"gemini-3.1-pro-low"},
			},
			"agentModelSorts": []map[string]interface{}{
				{
					"displayName": "Recommended",
					"groups": []map[string]interface{}{
						{
							"modelIds": []string{
								"gemini-3.8-flash-high",
								"gemini-pro-agent",
								"gemini-3.1-pro-low",
								"claude-sonnet-4-6",
								"gpt-oss-120b-medium",
							},
						},
					},
				},
			},
			"models": map[string]interface{}{
				"chat_23310": map[string]interface{}{
					"model":         "MODEL_CHAT_23310",
					"modelProvider": "MODEL_PROVIDER_GOOGLE",
				},
				"chat_20706": map[string]interface{}{
					"model":         "MODEL_CHAT_20706",
					"modelProvider": "MODEL_PROVIDER_GOOGLE",
				},
				"tab_jump_flash_lite_preview": map[string]interface{}{
					"model":         "MODEL_PLACEHOLDER_M28",
					"modelProvider": "MODEL_PROVIDER_GOOGLE",
				},
				"gemini-3.8-flash-tiered": map[string]interface{}{
					"model":         "MODEL_PLACEHOLDER_M322",
					"modelProvider": "MODEL_PROVIDER_GOOGLE",
				},
				"gemini-3.8-flash-high": map[string]interface{}{
					"displayName":      "Gemini 3.8 Flash (High)",
					"supportsThinking": true,
					"recommended":      true,
					"modelProvider":    "MODEL_PROVIDER_GOOGLE",
					"apiProvider":      "API_PROVIDER_GOOGLE_GEMINI",
				},
				"gemini-pro-agent": map[string]interface{}{
					"displayName":      "Gemini 3.1 Pro (High)",
					"supportsThinking": true,
					"modelProvider":    "MODEL_PROVIDER_GOOGLE",
					"apiProvider":      "API_PROVIDER_GOOGLE_GEMINI",
				},
				"gemini-3.1-pro-low": map[string]interface{}{
					"displayName":      "Gemini 3.1 Pro (Low)",
					"supportsThinking": true,
					"modelProvider":    "MODEL_PROVIDER_GOOGLE",
					"apiProvider":      "API_PROVIDER_GOOGLE_GEMINI",
				},
				"claude-sonnet-4-6": map[string]interface{}{
					"displayName":      "Claude Sonnet 4.6 (Thinking)",
					"supportsThinking": true,
					"modelProvider":    "MODEL_PROVIDER_ANTHROPIC",
					"apiProvider":      "API_PROVIDER_ANTHROPIC",
				},
				"gpt-oss-120b-medium": map[string]interface{}{
					"displayName":      "GPT-OSS 120B (Medium)",
					"supportsThinking": true,
					"modelProvider":    "MODEL_PROVIDER_OPENAI",
					"apiProvider":      "API_PROVIDER_OPENAI",
				},
			},
		})
	}))
	defer mockServer.Close()
	CloudCodeModelsURLs = []string{mockServer.URL}

	acc := &keyring.Account{
		Email:       "tester@example.com",
		AccessToken: "mock-valid-token",
	}

	cat := GetAvailableModelCatalog(acc, true)
	if cat == nil || !cat.Success {
		t.Fatalf("expected successful catalog")
	}

	// Verify default gemini updated to defaultAgentModelId
	if cat.DefaultGemini != "gemini-3.8-flash-high" {
		t.Errorf("expected default gemini 'gemini-3.8-flash-high', got '%s'", cat.DefaultGemini)
	}

	allDiscoveredIDs := make(map[string]bool)
	for _, m := range cat.GeminiModels {
		allDiscoveredIDs[m.ID] = true
	}
	for _, m := range cat.NonGeminiModels {
		allDiscoveredIDs[m.ID] = true
	}

	// 1. Verify internal and deprecated models are strictly EXCLUDED
	forbidden := []string{
		"chat_23310",
		"chat_20706",
		"tab_jump_flash_lite_preview",
		"gemini-3.8-flash-tiered",
		"gemini-3.1-pro-high",
		"gemini-3-flash",
		"gemini-3.1-flash-image",
	}
	for _, f := range forbidden {
		if allDiscoveredIDs[f] {
			t.Errorf("forbidden internal/deprecated model '%s' was leaked into catalog options", f)
		}
	}

	// 2. Verify Gemini models contain live gemini agent models
	found38High := false
	for _, m := range cat.GeminiModels {
		if m.ID == "gemini-3.8-flash-high" {
			found38High = true
			if m.Provider != "Google" {
				t.Errorf("expected provider Google for gemini-3.8-flash-high, got %s", m.Provider)
			}
			if !m.SupportsThinking {
				t.Errorf("expected supportsThinking true for gemini-3.8-flash-high")
			}
		}
	}
	if !found38High {
		t.Errorf("expected gemini-3.8-flash-high in GeminiModels")
	}

	// 3. Verify NonGeminiModels contains claude-sonnet-4-6 and gpt-oss-120b-medium
	foundClaudeSonnet := false
	foundGPTOss := false
	for _, m := range cat.NonGeminiModels {
		if m.ID == "claude-sonnet-4-6" {
			foundClaudeSonnet = true
			if m.Provider != "Anthropic" {
				t.Errorf("expected provider Anthropic for claude-sonnet-4-6, got %s", m.Provider)
			}
		}
		if m.ID == "gpt-oss-120b-medium" {
			foundGPTOss = true
			if m.Provider != "OpenAI" {
				t.Errorf("expected provider OpenAI for gpt-oss-120b-medium, got %s", m.Provider)
			}
		}
	}
	if !foundClaudeSonnet {
		t.Errorf("expected claude-sonnet-4-6 in NonGeminiModels")
	}
	if !foundGPTOss {
		t.Errorf("expected gpt-oss-120b-medium in NonGeminiModels")
	}
}


