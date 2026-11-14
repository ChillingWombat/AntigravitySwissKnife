package quota

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/keyring"
)

func catalogModelIDs(models []ModelOption) []string {
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	return ids
}

func assertCatalogModelIDs(t *testing.T, name string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s length mismatch: want %v, got %v", name, want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s order mismatch at %d: want %v, got %v", name, i, want, got)
		}
	}
}

func containsCatalogModel(models []ModelOption, id string) bool {
	for _, model := range models {
		if model.ID == id {
			return true
		}
	}
	return false
}

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
	if cat.DefaultNonGemini != "claude-opus-4-6-thinking" {
		t.Errorf("expected default non-gemini 'claude-opus-4-6-thinking', got '%s'", cat.DefaultNonGemini)
	}
	if len(cat.GeminiModels) != 12 {
		t.Errorf("expected 12 baseline Gemini models, got %d", len(cat.GeminiModels))
	}
	if len(cat.NonGeminiModels) != 10 {
		t.Errorf("expected 10 baseline Non-Gemini models, got %d", len(cat.NonGeminiModels))
	}
	for _, wantID := range []string{
		"gemini-4-argon-high",
		"gemini-4-argon-medium",
		"gemini-4-argon-low",
		"gemini-4-argon",
		"gemini-4-hydrogen-xhigh",
		"gemini-4-hydrogen-high",
		"gemini-4-hydrogen-medium",
		"gemini-4-hydrogen-low",
		"gemini-pro-orchestrator-g1-pro",
		"gemini-3.8-flash-high",
		"gemini-3.8-flash-medium",
		"gemini-3.8-flash-low",
	} {
		if !containsCatalogModel(cat.GeminiModels, wantID) {
			t.Errorf("expected %s in baseline Gemini models", wantID)
		}
	}
	for _, wantID := range []string{
		"claude-opus-5-5-high",
		"claude-opus-5-5-medium",
		"claude-opus-5-5-low",
		"claude-opus-5-5",
		"claude-sonnet-5-5-high",
		"claude-sonnet-5-5-medium",
		"claude-sonnet-5-5-low",
		"claude-sonnet-4-6",
		"claude-opus-4-6-thinking",
		"gpt-oss-120b-medium",
	} {
		if !containsCatalogModel(cat.NonGeminiModels, wantID) {
			t.Errorf("expected %s in baseline Non-Gemini models", wantID)
		}
	}

	if cat.GeminiModels[0].ID != "gemini-4-argon-high" {
		t.Errorf("expected first Gemini model to be 'gemini-4-argon-high', got '%s'", cat.GeminiModels[0].ID)
	}
	if cat.NonGeminiModels[0].ID != "claude-opus-5-5-high" {
		t.Errorf("expected first Non-Gemini model to be 'claude-opus-5-5-high', got '%s'", cat.NonGeminiModels[0].ID)
	}
	for _, m := range cat.GeminiModels {
		if m.ID == "gemini-4-hydrogen-high" && !m.Recommended {
			t.Errorf("expected gemini-4-hydrogen-high to be marked Recommended")
		}
		if m.ID == "gemini-pro-orchestrator-g1-pro" && (!m.SupportsThinking || len(m.ThinkingLevels) != 1 || m.ThinkingLevels[0] != "high") {
			t.Errorf("unexpected orchestrator thinking metadata: %+v", m)
		}
	}
	for _, m := range cat.NonGeminiModels {
		if (m.ID == "claude-opus-5-5-high" || m.ID == "claude-sonnet-5-5-high") && !m.Recommended {
			t.Errorf("expected %s to be marked Recommended", m.ID)
		}
	}
}

// TestGetAvailableModelCatalog_LiveMock verifies live picker models are merged
// into the baseline while models-map-only and tiered ids stay excluded.
func TestGetAvailableModelCatalog_LiveMock(t *testing.T) {
	origModelsURLs := CloudCodeModelsURLs
	defer func() { CloudCodeModelsURLs = origModelsURLs }()

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"defaultAgentModelId": "gemini-3.8-flash-medium",
			"tieredModelIds": map[string]interface{}{
				"flash": []string{"gemini-3.8-flash-tiered", "gemini-2.5-flash"},
			},
			"agentModelSorts": []map[string]interface{}{
				{
					"displayName": "Recommended",
					"groups": []map[string]interface{}{
						{
							"modelIds": []string{
								"gemini-3.8-flash-medium",
								"claude-experimental-preview",
							},
						},
					},
				},
			},
			"models": map[string]interface{}{
				"gemini-3.8-flash-medium": map[string]interface{}{
					"displayName":      "Gemini 3.8 Flash (Medium)",
					"supportsThinking": true,
					"recommended":      true,
					"modelProvider":    "MODEL_PROVIDER_GOOGLE",
				},
				"gemini-3.8-flash-tiered": map[string]interface{}{
					"modelProvider": "MODEL_PROVIDER_GOOGLE",
				},
				"gemini-2.5-flash": map[string]interface{}{
					"displayName":   "Gemini 3.5 Flash Lite",
					"modelProvider": "MODEL_PROVIDER_GOOGLE",
				},
				"claude-experimental-preview": map[string]interface{}{
					"displayName":      "Claude Experimental Preview",
					"supportsThinking": true,
					"modelProvider":    "MODEL_PROVIDER_ANTHROPIC",
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

	if cat.DefaultGemini != "gemini-3.8-flash-medium" {
		t.Errorf("expected default gemini 'gemini-3.8-flash-medium', got '%s'", cat.DefaultGemini)
	}
	if cat.DefaultNonGemini != "claude-opus-4-6-thinking" {
		t.Errorf("expected default non-gemini 'claude-opus-4-6-thinking', got '%s'", cat.DefaultNonGemini)
	}

	// Live picker models are unioned with the baseline and deduplicated by ID.
	assertCatalogModelIDs(t, "GeminiModels", catalogModelIDs(cat.GeminiModels), catalogModelIDs(DefaultBaseGeminiModels()))
	wantNonGemini := append(catalogModelIDs(DefaultBaseNonGeminiModels()), "claude-experimental-preview")
	assertCatalogModelIDs(t, "NonGeminiModels", catalogModelIDs(cat.NonGeminiModels), wantNonGemini)

	// Stale baseline ids and non-picker ids must not leak back in.
	for _, m := range cat.GeminiModels {
		if m.ID == "gemini-2.5-pro" || m.ID == "gemini-2.5-flash" || m.ID == "gemini-3.8-flash-tiered" {
			t.Errorf("stale/non-picker id '%s' leaked into GeminiModels", m.ID)
		}
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
	if cat.DefaultNonGemini != "claude-opus-4-6-thinking" {
		t.Errorf("expected fallback default non-gemini 'claude-opus-4-6-thinking', got '%s'", cat.DefaultNonGemini)
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
			"defaultAgentModelId":     "gemini-3.8-flash-high",
			"tabModelIds":             []string{"chat_20706", "chat_23310"},
			"commandModelIds":         []string{"gemini-3-flash"},
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

// TestGetAvailableModelCatalog_CloudCodePickerFixture feeds a trimmed real
// CloudCode fetchAvailableModels response and asserts the picker catalog is
// built exclusively from agentModelSorts[].groups[].modelIds, in order.
func TestGetAvailableModelCatalog_CloudCodePickerFixture(t *testing.T) {
	origModelsURLs := CloudCodeModelsURLs
	defer func() { CloudCodeModelsURLs = origModelsURLs }()

	fixture, err := os.ReadFile("testdata/fetch_available_models.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
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

	wantGemini := []string{
		"gemini-4-argon-high",
		"gemini-4-argon-medium",
		"gemini-4-argon-low",
		"gemini-4-hydrogen-xhigh",
		"gemini-4-hydrogen-high",
		"gemini-4-hydrogen-medium",
		"gemini-4-hydrogen-low",
		"gemini-pro-orchestrator-g1-pro",
		"gemini-3.8-flash-high",
		"gemini-3.8-flash-medium",
		"gemini-3.8-flash-low",
		"gemini-4-argon",
	}
	wantNonGemini := []string{
		"claude-opus-5-5-high",
		"claude-opus-5-5-medium",
		"claude-opus-5-5-low",
		"claude-sonnet-5-5-high",
		"claude-sonnet-5-5-medium",
		"claude-sonnet-5-5-low",
		"claude-sonnet-4-6",
		"claude-opus-4-6-thinking",
		"gpt-oss-120b-medium",
		"claude-opus-5-5",
	}

	gotGemini := make([]string, 0, len(cat.GeminiModels))
	for _, m := range cat.GeminiModels {
		gotGemini = append(gotGemini, m.ID)
	}
	if len(gotGemini) != len(wantGemini) {
		t.Fatalf("expected %d gemini models, got %d: %v", len(wantGemini), len(gotGemini), gotGemini)
	}
	for i, id := range wantGemini {
		if gotGemini[i] != id {
			t.Fatalf("gemini model order mismatch at %d: want %v, got %v", i, wantGemini, gotGemini)
		}
	}

	gotNonGemini := make([]string, 0, len(cat.NonGeminiModels))
	for _, m := range cat.NonGeminiModels {
		gotNonGemini = append(gotNonGemini, m.ID)
	}
	if len(gotNonGemini) != len(wantNonGemini) {
		t.Fatalf("expected %d non-gemini models, got %d: %v", len(wantNonGemini), len(gotNonGemini), gotNonGemini)
	}
	for i, id := range wantNonGemini {
		if gotNonGemini[i] != id {
			t.Fatalf("non-gemini model order mismatch at %d: want %v, got %v", i, wantNonGemini, gotNonGemini)
		}
	}

	if cat.DefaultGemini != "gemini-3.8-flash-high" {
		t.Errorf("expected default gemini 'gemini-3.8-flash-high', got '%s'", cat.DefaultGemini)
	}
	if cat.DefaultNonGemini != "claude-opus-4-6-thinking" {
		t.Errorf("expected default non-gemini 'claude-opus-4-6-thinking', got '%s'", cat.DefaultNonGemini)
	}

	// Internal, deprecated, and non-picker ids must not leak into the catalog.
	forbidden := []string{
		"chat_20706",
		"chat_23310",
		"tab_flash_lite_preview",
		"tab_jump_flash_lite_preview",
		"gemini-3.8-flash-tiered",
		"gemini-3.7-flash-tiered",
		"gemini-3.6-flash-tiered",
		"gemini-3.1-pro-high",
		"gemini-3-flash",
		"gemini-3.1-flash-image",
		"gemini-3.5-flash-lite",
		"gemini-2.5-pro",
		"gemini-2.5-flash",
		"gemini-3-flash-agent",
		"gemini-4-carbon-disabled",
	}
	for _, f := range forbidden {
		for _, m := range cat.GeminiModels {
			if m.ID == f {
				t.Errorf("forbidden model '%s' leaked into GeminiModels", f)
			}
		}
		for _, m := range cat.NonGeminiModels {
			if m.ID == f {
				t.Errorf("forbidden model '%s' leaked into NonGeminiModels", f)
			}
		}
	}

	// Spot-check display names from the models map.
	if cat.GeminiModels[0].DisplayName != "Gemini 4 Argon (High)" {
		t.Errorf("expected display name 'Gemini 4 Argon (High)', got '%s'", cat.GeminiModels[0].DisplayName)
	}
	if cat.GeminiModels[7].DisplayName != "Orchestrator (Argon)" {
		t.Errorf("expected display name 'Orchestrator (Argon)', got '%s'", cat.GeminiModels[7].DisplayName)
	}
	if cat.NonGeminiModels[0].DisplayName != "Claude Opus 5.5 (High)" {
		t.Errorf("expected display name 'Claude Opus 5.5 (High)', got '%s'", cat.NonGeminiModels[0].DisplayName)
	}
}

func TestGetAvailableModelCatalogFleet(t *testing.T) {
	origModelsURLs := CloudCodeModelsURLs
	defer func() { CloudCodeModelsURLs = origModelsURLs }()

	writePickerResponse := func(w http.ResponseWriter, defaultID string, modelIDs []string, models map[string]interface{}) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"defaultAgentModelId": defaultID,
			"agentModelSorts": []map[string]interface{}{
				{
					"groups": []map[string]interface{}{
						{"modelIds": modelIDs},
					},
				},
			},
			"models": models,
		})
	}

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("Authorization") {
		case "Bearer plus-token":
			writePickerResponse(w, "gemini-fleet-plus",
				[]string{"gemini-fleet-plus", "gemini-fleet-shared", "claude-fleet-plus"},
				map[string]interface{}{
					"gemini-fleet-plus": map[string]interface{}{
						"displayName":   "Gemini Fleet Plus",
						"modelProvider": "MODEL_PROVIDER_GOOGLE",
					},
					"gemini-fleet-shared": map[string]interface{}{
						"displayName":   "Gemini Fleet Shared (Plus)",
						"modelProvider": "MODEL_PROVIDER_GOOGLE",
					},
					"claude-fleet-plus": map[string]interface{}{
						"displayName":   "Claude Fleet Plus",
						"modelProvider": "MODEL_PROVIDER_ANTHROPIC",
					},
				})
		case "Bearer pro-token":
			writePickerResponse(w, "gemini-fleet-pro",
				[]string{"gemini-fleet-pro", "gpt-fleet-pro", "gemini-fleet-shared"},
				map[string]interface{}{
					"gemini-fleet-pro": map[string]interface{}{
						"displayName":   "Gemini Fleet Pro",
						"modelProvider": "MODEL_PROVIDER_GOOGLE",
					},
					"gpt-fleet-pro": map[string]interface{}{
						"displayName":   "GPT Fleet Pro",
						"modelProvider": "MODEL_PROVIDER_OPENAI",
					},
					"gemini-fleet-shared": map[string]interface{}{
						"displayName":   "Gemini Fleet Shared (Pro)",
						"modelProvider": "MODEL_PROVIDER_GOOGLE",
					},
				})
		default:
			http.Error(w, "unexpected authorization", http.StatusUnauthorized)
		}
	}))
	defer mockServer.Close()
	CloudCodeModelsURLs = []string{mockServer.URL}

	plus := &keyring.Account{
		Email:       "plus@example.com",
		PlanTier:    PlanTierPlus,
		AccessToken: "plus-token",
	}
	pro := &keyring.Account{
		Email:       "pro@example.com",
		PlanTier:    PlanTierPro,
		AccessToken: "pro-token",
	}

	cat := GetAvailableModelCatalogFleet([]*keyring.Account{plus, pro}, pro, true)
	if cat == nil || !cat.Success {
		t.Fatalf("expected successful fleet catalog")
	}

	wantGemini := append(catalogModelIDs(DefaultBaseGeminiModels()),
		"gemini-fleet-plus", "gemini-fleet-shared", "gemini-fleet-pro")
	wantNonGemini := append(catalogModelIDs(DefaultBaseNonGeminiModels()),
		"claude-fleet-plus", "gpt-fleet-pro")
	assertCatalogModelIDs(t, "fleet GeminiModels", catalogModelIDs(cat.GeminiModels), wantGemini)
	assertCatalogModelIDs(t, "fleet NonGeminiModels", catalogModelIDs(cat.NonGeminiModels), wantNonGemini)

	if cat.DefaultGemini != "gemini-fleet-pro" {
		t.Errorf("expected active account default gemini 'gemini-fleet-pro', got '%s'", cat.DefaultGemini)
	}
	for _, model := range cat.GeminiModels {
		if model.ID == "gemini-fleet-shared" && model.DisplayName != "Gemini Fleet Shared (Pro)" {
			t.Errorf("expected active fleet metadata for shared model, got '%s'", model.DisplayName)
		}
	}
}

func TestIsArgonModel_HydrogenAndCarbonExclusion(t *testing.T) {
	shouldMatch := [][]string{
		{"gemini-4-argon-high"},
		{"gemini-4-argon-medium", "Gemini 4 Argon (Medium)"},
		{"gemini-4-argon-low"},
		{"gemini-pro-orchestrator-g1-pro", "Orchestrator (Argon)"},
		{"MODEL_PLACEHOLDER_M417"},
		{"MODEL_PLACEHOLDER_M418"},
		{"MODEL_PLACEHOLDER_M419"},
		{"MODEL_PLACEHOLDER_M668"},
		{"gemini-4-pro"},
	}
	for _, vals := range shouldMatch {
		if !isArgonModel(vals...) {
			t.Errorf("expected isArgonModel(%v) = true", vals)
		}
	}

	shouldNotMatch := [][]string{
		{"gemini-4-hydrogen-xhigh"},
		{"gemini-4-hydrogen-high", "Gemini 4 Hydrogen (High)"},
		{"gemini-4-hydrogen-medium"},
		{"gemini-4-hydrogen-low"},
		{"Gemini Models", "Models within this group: Gemini 4 Hydrogen, Gemini 3.8 Flash"},
		{"gemini-4-carbon-disabled", "Gemini 4 Carbon"},
		{"gemini-3.8-flash-high"},
	}
	for _, vals := range shouldNotMatch {
		if isArgonModel(vals...) {
			t.Errorf("expected isArgonModel(%v) = false", vals)
		}
	}
}

func TestIsInternalModelID_DisabledSuffix(t *testing.T) {
	if !isInternalModelID("gemini-4-carbon-disabled") {
		t.Errorf("expected gemini-4-carbon-disabled to be internal/filtered")
	}
	if !isInternalModelID("custom-model-DISABLED") {
		t.Errorf("expected -disabled suffix to match case-insensitively")
	}
	if isInternalModelID("gemini-4-hydrogen-high") {
		t.Errorf("did not expect gemini-4-hydrogen-high to be marked internal")
	}
}

func TestIsTrialWarningText_GPTOssExclusionAndNov18Notice(t *testing.T) {
	nonTrial := []string{
		"GPT-OSS will be removed in an upcoming release.",
		"gpt-oss will be removed soon; switch to Claude or Gemini.",
		"GPT-OSS 120B will no longer be available on your current plan after deprecation.",
	}
	for _, msg := range nonTrial {
		if IsTrialWarningText(msg) {
			t.Errorf("expected IsTrialWarningText(%q) = false", msg)
		}
	}

	trialNotices := []string{
		"Third-party model access will no longer be available on your current plan starting on November 18, 2026.",
		"Third-party model access will be restricted on your current plan starting November 18, 2026.",
	}
	for _, msg := range trialNotices {
		if !IsTrialWarningText(msg) {
			t.Errorf("expected IsTrialWarningText(%q) = true", msg)
		}
	}
}

func TestDetectTierFromAvailableModels_Antigravity2321(t *testing.T) {
	origModelsURLs := CloudCodeModelsURLs
	defer func() { CloudCodeModelsURLs = origModelsURLs }()

	cases := []struct {
		name     string
		models   map[string]map[string]string
		wantTier string
	}{
		{
			name: "free account with hydrogen and flash",
			models: map[string]map[string]string{
				"gemini-4-hydrogen-high":   {"displayName": "Gemini 4 Hydrogen (High)"},
				"gemini-3.8-flash-high":    {"displayName": "Gemini 3.8 Flash (High)"},
				"gemini-4-carbon-disabled": {"displayName": "Gemini 4 Carbon"},
			},
			wantTier: PlanTierFree,
		},
		{
			name: "pro trial with argon but without 5.5 third-party models",
			models: map[string]map[string]string{
				"gemini-4-argon-high":    {"displayName": "Gemini 4 Argon (High)"},
				"gemini-4-hydrogen-high": {"displayName": "Gemini 4 Hydrogen (High)"},
				"claude-sonnet-4-6":      {"displayName": "Claude Sonnet 4.6 (Thinking)"},
			},
			wantTier: PlanTierProTrial,
		},
		{
			name: "paid pro with argon and claude sonnet 5.5",
			models: map[string]map[string]string{
				"gemini-4-argon-high":    {"displayName": "Gemini 4 Argon (High)"},
				"gemini-4-hydrogen-high": {"displayName": "Gemini 4 Hydrogen (High)"},
				"claude-sonnet-5-5-high": {"displayName": "Claude Sonnet 5.5 (High)"},
			},
			wantTier: PlanTierPro,
		},
		{
			name: "paid pro with opus 5.5",
			models: map[string]map[string]string{
				"gemini-pro-orchestrator-g1-pro": {"displayName": "Orchestrator (Argon)"},
				"claude-opus-5-5-high":           {"displayName": "Claude Opus 5.5 (High)"},
			},
			wantTier: PlanTierPro,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"models": tc.models})
			}))
			defer srv.Close()
			CloudCodeModelsURLs = []string{srv.URL}

			tier, err := DetectTierFromAvailableModels("mock-token")
			if err != nil {
				t.Fatalf("DetectTierFromAvailableModels() error = %v", err)
			}
			if tier != tc.wantTier {
				t.Errorf("got tier %q, want %q", tier, tc.wantTier)
			}
		})
	}
}
