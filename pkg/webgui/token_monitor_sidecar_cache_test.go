package webgui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/custommodels"
)

func TestTokenMonitorSidecarOverrideInvalidatesWarmTranscriptCache(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", configDir)
	store, err := custommodels.NewStore(configDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	convID := "e05a681a-1e3c-47f0-b217-aaadc7625295"
	tPath := filepath.Join(t.TempDir(), "brain", convID, ".system_generated", "logs", "transcript.jsonl")
	if err := os.MkdirAll(filepath.Dir(tPath), 0755); err != nil {
		t.Fatalf("create transcript dir: %v", err)
	}
	step0, err := json.Marshal(map[string]any{
		"step_index": 0,
		"source":     "SYSTEM",
		"type":       "SYSTEM_MESSAGE",
		"content":    "<USER_SETTINGS_CHANGE>\nThe user changed setting `Model Selection` from None to Gemini 3.8 Flash (High). No need to comment on this change if the user doesn't ask about it.\n</USER_SETTINGS_CHANGE>",
	})
	if err != nil {
		t.Fatalf("marshal transcript: %v", err)
	}
	if err := os.WriteFile(tPath, append(step0, '\n'), 0644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}
	custom := []custommodels.CustomModel{{
		ID:           "custom-routed-id",
		Name:         "custom-routed-model",
		DisplayName:  "Custom Routed Model",
		ProviderType: custommodels.ProviderOpenAI,
	}}

	if got, _, _ := detectConversationModelWithTranscript(tPath, "", custom, nil); got != "gemini-3.8-flash" {
		t.Fatalf("warm transcript model = %q, want gemini-3.8-flash", got)
	}
	store.RecordConversationModel(convID, "custom-routed-id")
	got, displayName, provider := detectConversationModelWithTranscript(tPath, "", custom, nil)
	if got != "custom-routed-model" || displayName != "Custom Routed Model" || provider != "openai" {
		t.Fatalf("sidecar override = %q (%q, %q), want custom-routed-model (Custom Routed Model, openai)", got, displayName, provider)
	}
}

func TestResolveConversationModelUsesSidecarWithoutCustomModels(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", configDir)
	store, err := custommodels.NewStore(configDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	model := custommodels.CustomModel{
		ID:           "custom-routed-id",
		Name:         "custom-routed-model",
		DisplayName:  "Custom Routed Model",
		ProviderType: custommodels.ProviderOpenAI,
		BaseURL:      "https://api.example.test/v1",
		QuotaType:    custommodels.QuotaTypeNA,
		Enabled:      true,
	}
	if err := store.SaveModel(model); err != nil {
		t.Fatalf("SaveModel: %v", err)
	}
	convID := "e05a681a-1e3c-47f0-b217-aaadc7625295"
	if err := store.RecordConversationModel(convID, model.ID); err != nil {
		t.Fatalf("RecordConversationModel: %v", err)
	}

	got, displayName, provider := resolveConversationModel(convID)
	if got != model.Name || displayName != model.DisplayName || provider != "openai" {
		t.Fatalf("resolved model = %q (%q, %q), want %q (%q, openai)", got, displayName, provider, model.Name, model.DisplayName)
	}
}
