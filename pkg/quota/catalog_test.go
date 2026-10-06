package quota

import (
	"testing"
)

func TestGetAvailableModelCatalog_Defaults(t *testing.T) {
	cat := GetAvailableModelCatalog(nil, true)
	if cat == nil {
		t.Fatalf("expected non-nil catalog")
	}
	if !cat.Success {
		t.Errorf("expected success true")
	}
	if cat.DefaultGemini != "gemini-3.8-flash" {
		t.Errorf("expected default gemini 'gemini-3.8-flash', got '%s'", cat.DefaultGemini)
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

	// Verify first item of Gemini models is gemini-3.8-flash
	if cat.GeminiModels[0].ID != "gemini-3.8-flash" {
		t.Errorf("expected first Gemini model to be 'gemini-3.8-flash', got '%s'", cat.GeminiModels[0].ID)
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
