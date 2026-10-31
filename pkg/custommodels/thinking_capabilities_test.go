package custommodels

import (
	"reflect"
	"strings"
	"testing"
)

func thinkingCapabilityModel(levels []string, active string) CustomModel {
	return CustomModel{
		ID:               "custom-thinking",
		Name:             "custom-thinking",
		DisplayName:      "Custom Thinking",
		ProviderType:     ProviderOpenAI,
		BaseURL:          "https://example.test/v1",
		QuotaType:        QuotaTypeNA,
		SupportsThinking: true,
		ThinkingLevels:   levels,
		ThinkingLevel:    active,
		Enabled:          true,
	}
}

func TestStorePreservesThinkingCapabilitiesWhenActiveLevelChanges(t *testing.T) {
	configDir := t.TempDir()
	store, err := NewStore(configDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if err := store.SaveModel(thinkingCapabilityModel([]string{"low", "medium", "high"}, "high")); err != nil {
		t.Fatalf("save model: %v", err)
	}
	if err := store.SetThinkingLevel("custom-thinking", "low"); err != nil {
		t.Fatalf("set thinking level: %v", err)
	}

	reloaded, err := NewStore(configDir)
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}
	model, err := reloaded.GetModel("custom-thinking")
	if err != nil {
		t.Fatalf("get model: %v", err)
	}
	if want := []string{"low", "medium", "high"}; !reflect.DeepEqual(model.ThinkingLevels, want) {
		t.Fatalf("thinking levels after level change = %v, want %v", model.ThinkingLevels, want)
	}
	if model.ThinkingLevel != "low" {
		t.Fatalf("active thinking level = %q, want low", model.ThinkingLevel)
	}
}

func TestStorePreservesSingleThinkingLevel(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if err := store.SaveModel(thinkingCapabilityModel([]string{"high"}, "high")); err != nil {
		t.Fatalf("save model: %v", err)
	}
	model, err := store.GetModel("custom-thinking")
	if err != nil {
		t.Fatalf("get model: %v", err)
	}
	if want := []string{"high"}; !reflect.DeepEqual(model.ThinkingLevels, want) {
		t.Fatalf("single thinking level = %v, want %v", model.ThinkingLevels, want)
	}
	if model.ThinkingLevel != "high" {
		t.Fatalf("active thinking level = %q, want high", model.ThinkingLevel)
	}
}

func TestStoreNormalizesInvalidThinkingSelectionWithoutDroppingCapabilities(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if err := store.SaveModel(thinkingCapabilityModel([]string{"low", "high", "turbo"}, "turbo")); err != nil {
		t.Fatalf("save model: %v", err)
	}
	model, err := store.GetModel("custom-thinking")
	if err != nil {
		t.Fatalf("get model: %v", err)
	}
	if want := []string{"low", "high"}; !reflect.DeepEqual(model.ThinkingLevels, want) {
		t.Fatalf("supported thinking levels = %v, want %v", model.ThinkingLevels, want)
	}
	if model.ThinkingLevel != "high" {
		t.Fatalf("normalized thinking level = %q, want high", model.ThinkingLevel)
	}
}

func TestStoreRejectsThinkingWithoutSupportedLevels(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	err = store.SaveModel(thinkingCapabilityModel([]string{"turbo"}, "turbo"))
	if err == nil {
		t.Fatal("expected unsupported thinking levels to be rejected")
	}
	if !strings.Contains(err.Error(), "supported thinking level") {
		t.Fatalf("error = %q, want supported thinking level failure", err)
	}
	if _, err := store.GetModel("custom-thinking"); err == nil {
		t.Fatal("model with unsupported thinking capabilities was persisted")
	}
}

func TestStoreDefaultsThinkingCapabilitiesWhenListIsAbsent(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if err := store.SaveModel(thinkingCapabilityModel(nil, "high")); err != nil {
		t.Fatalf("save model: %v", err)
	}
	model, err := store.GetModel("custom-thinking")
	if err != nil {
		t.Fatalf("get model: %v", err)
	}
	if want := []string{"low", "medium", "high"}; !reflect.DeepEqual(model.ThinkingLevels, want) {
		t.Fatalf("legacy thinking levels = %v, want %v", model.ThinkingLevels, want)
	}
	if model.ThinkingLevel != "high" {
		t.Fatalf("legacy active thinking level = %q, want high", model.ThinkingLevel)
	}
}
