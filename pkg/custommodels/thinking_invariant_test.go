package custommodels

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCustomModelValidate_ThinkingCapabilityMatrix(t *testing.T) {
	tests := []struct {
		name         string
		supports     bool
		levels       []string
		active       string
		wantErr      bool
		wantSupports bool
		wantLevels   []string
		wantActive   string
	}{
		{
			name:       "false with absent levels",
			supports:   false,
			wantLevels: nil,
			wantActive: "off",
		},
		{
			name:       "false with invalid level",
			supports:   false,
			levels:     []string{"turbo"},
			wantLevels: nil,
			wantActive: "off",
		},
		{
			name:       "false with stale valid level",
			supports:   false,
			levels:     []string{"low"},
			active:     "low",
			wantLevels: nil,
			wantActive: "off",
		},
		{
			name:         "true with absent levels uses legacy default",
			supports:     true,
			wantSupports: true,
			wantLevels:   []string{"low", "medium", "high"},
			wantActive:   "high",
		},
		{
			name:         "true with only invalid levels is rejected",
			supports:     true,
			levels:       []string{"turbo"},
			active:       "turbo",
			wantErr:      true,
			wantSupports: true,
		},
		{
			name:         "true with mixed levels filters and normalizes active",
			supports:     true,
			levels:       []string{"low", "high", "turbo"},
			active:       "turbo",
			wantSupports: true,
			wantLevels:   []string{"low", "high"},
			wantActive:   "high",
		},
		{
			name:         "true with one valid level remains single",
			supports:     true,
			levels:       []string{"high"},
			wantSupports: true,
			wantLevels:   []string{"high"},
			wantActive:   "high",
		},
		{
			name:         "true filters case and duplicate levels",
			supports:     true,
			levels:       []string{"LOW", "medium", "low", "turbo", "high"},
			active:       "medium",
			wantSupports: true,
			wantLevels:   []string{"low", "medium", "high"},
			wantActive:   "medium",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := CustomModel{
				ID:               "thinking-matrix",
				Name:             "thinking-matrix",
				BaseURL:          "https://example.test/v1",
				SupportsThinking: tt.supports,
				ThinkingLevels:   tt.levels,
				ThinkingLevel:    tt.active,
			}

			err := model.Validate()
			if tt.wantErr {
				if err == nil {
					t.Fatal("Validate accepted a thinking-capable model with no supported levels")
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate failed: %v", err)
			}

			assert427ThinkingCapabilityState(t, &model, tt.wantSupports, tt.wantLevels, tt.wantActive)
		})
	}
}

func TestGenerateCustomModelsScript_ThinkingCapabilityGate(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available; skipping node runtime verification")
	}

	script := GenerateCustomModelsScript(nil)
	fnStart := strings.Index(script, "function getModelThinkingLevels(m)")
	if fnStart == -1 {
		t.Fatal("getModelThinkingLevels not found in script")
	}
	fnEnd := strings.Index(script[fnStart:], "function getActiveThinkingLevel")
	if fnEnd == -1 {
		t.Fatal("could not isolate getModelThinkingLevels")
	}
	helperCode := script[fnStart : fnStart+fnEnd]

	testRunner := fmt.Sprintf(`
%s
const assert = require("assert");

const gcpBase = { name: "gemini-4-argon", provider_type: "gcp" };
assert.deepStrictEqual(getModelThinkingLevels({ ...gcpBase, supports_thinking: false, thinking_levels: ["turbo"] }), []);
assert.deepStrictEqual(getModelThinkingLevels({ ...gcpBase, supports_thinking: false, thinking_levels: ["low"] }), []);
assert.deepStrictEqual(getModelThinkingLevels({ ...gcpBase, supports_thinking: false, thinking_level: "high" }), []);
assert.deepStrictEqual(getModelThinkingLevels({ name: "custom", supports_thinking: true }), ["low", "medium", "high"]);
assert.deepStrictEqual(getModelThinkingLevels({ name: "custom", supports_thinking: true, thinking_levels: ["turbo"] }), []);
assert.deepStrictEqual(getModelThinkingLevels({ name: "custom", supports_thinking: true, thinking_levels: ["low", "high", "turbo", "HIGH"] }), ["low", "high"]);
assert.deepStrictEqual(getModelThinkingLevels({ name: "custom", supports_thinking: true, thinking_levels: ["high"] }), ["high"]);

console.log("ALL_THINKING_CAPABILITY_GATES_VERIFIED");
`, helperCode)

	cmd := exec.Command(nodePath, "-e", testRunner)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node verification failed: %v: %s", err, string(out))
	}
	if !strings.Contains(string(out), "ALL_THINKING_CAPABILITY_GATES_VERIFIED") {
		t.Fatalf("unexpected node output: %s", string(out))
	}
	if !strings.Contains(script, "const supportsThinking = m.supports_thinking === true && levels.length > 0;") {
		t.Error("renderer support gate must require explicit support and a usable level")
	}
	if !strings.Contains(script, "const canChangeThinking = levels.length > 1;") {
		t.Error("renderer submenu gate must require at least two valid levels")
	}
}

func TestStore_ThinkingCapabilityUpdateReload(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	base := CustomModel{
		ID:               "thinking-store-model",
		Name:             "thinking-store-model",
		DisplayName:      "Thinking Store Model",
		BaseURL:          "https://example.test/v1",
		SupportsThinking: true,
		ThinkingLevels:   []string{"low", "medium", "high"},
		ThinkingLevel:    "medium",
		Enabled:          true,
	}
	if err := store.SaveModel(base); err != nil {
		t.Fatalf("SaveModel(base) failed: %v", err)
	}

	staleDisabled := base
	staleDisabled.SupportsThinking = false
	staleDisabled.ThinkingLevels = []string{"turbo"}
	staleDisabled.ThinkingLevel = "turbo"
	if err := store.SaveModel(staleDisabled); err != nil {
		t.Fatalf("SaveModel(stale disabled) failed: %v", err)
	}
	got, err := store.GetModel(base.ID)
	if err != nil {
		t.Fatalf("GetModel after disabled update failed: %v", err)
	}
	assert427ThinkingCapabilityState(t, got, false, nil, "off")

	reloaded, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore(reload) failed: %v", err)
	}
	got, err = reloaded.GetModel(base.ID)
	if err != nil {
		t.Fatalf("GetModel after reload failed: %v", err)
	}
	assert427ThinkingCapabilityState(t, got, false, nil, "off")

	if err := reloaded.SetThinkingLevel(base.ID, "low"); err == nil {
		t.Fatal("SetThinkingLevel accepted a model without thinking support")
	}

	invalidEnabled := base
	invalidEnabled.ThinkingLevels = []string{"turbo"}
	invalidEnabled.ThinkingLevel = "turbo"
	if err := reloaded.SaveModel(invalidEnabled); err == nil {
		t.Fatal("SaveModel accepted a thinking-capable model with no supported levels")
	}
	got, err = reloaded.GetModel(base.ID)
	if err != nil {
		t.Fatalf("GetModel after rejected update failed: %v", err)
	}
	assert427ThinkingCapabilityState(t, got, false, nil, "off")

	multiLevel := base
	multiLevel.ThinkingLevels = []string{"low", "high", "turbo"}
	multiLevel.ThinkingLevel = "turbo"
	if err := reloaded.SaveModel(multiLevel); err != nil {
		t.Fatalf("SaveModel(multi-level) failed: %v", err)
	}
	got, err = reloaded.GetModel(base.ID)
	if err != nil {
		t.Fatalf("GetModel after multi-level update failed: %v", err)
	}
	assert427ThinkingCapabilityState(t, got, true, []string{"low", "high"}, "high")

	if err := reloaded.SetThinkingLevel(base.ID, "low"); err != nil {
		t.Fatalf("SetThinkingLevel(low) failed: %v", err)
	}
	if err := reloaded.SetThinkingLevel(base.ID, "turbo"); err != nil {
		t.Fatalf("SetThinkingLevel(turbo) failed: %v", err)
	}
	got, err = reloaded.GetModel(base.ID)
	if err != nil {
		t.Fatalf("GetModel after active-level normalization failed: %v", err)
	}
	assert427ThinkingCapabilityState(t, got, true, []string{"low", "high"}, "high")

	finalReload, err := NewStore(filepath.Clean(dir))
	if err != nil {
		t.Fatalf("NewStore(final reload) failed: %v", err)
	}
	got, err = finalReload.GetModel(base.ID)
	if err != nil {
		t.Fatalf("GetModel after final reload failed: %v", err)
	}
	assert427ThinkingCapabilityState(t, got, true, []string{"low", "high"}, "high")
}

func assert427ThinkingCapabilityState(t *testing.T, model *CustomModel, supports bool, levels []string, active string) {
	t.Helper()
	if model == nil {
		t.Fatal("model is nil")
	}
	if model.SupportsThinking != supports {
		t.Errorf("SupportsThinking = %v; want %v", model.SupportsThinking, supports)
	}
	if !reflect.DeepEqual(model.ThinkingLevels, levels) {
		t.Errorf("ThinkingLevels = %#v; want %#v", model.ThinkingLevels, levels)
	}
	if model.ThinkingLevel != active {
		t.Errorf("ThinkingLevel = %q; want %q", model.ThinkingLevel, active)
	}
}
