package gui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig_ChatNavigatorMode(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ChatNavigatorMode != "panel" {
		t.Fatalf("expected default chat navigator mode to be %q, got %q", "panel", cfg.ChatNavigatorMode)
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("failed to marshal default config: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal marshalled config: %v", err)
	}
	if raw["chat_navigator_mode"] != "panel" {
		t.Fatalf("expected JSON key %q to be %q, got %v", "chat_navigator_mode", "panel", raw["chat_navigator_mode"])
	}

	var decoded Config
	if err := json.Unmarshal([]byte(`{"chat_navigator_mode":"line"}`), &decoded); err != nil {
		t.Fatalf("failed to unmarshal config JSON: %v", err)
	}
	if decoded.ChatNavigatorMode != "line" {
		t.Fatalf("expected chat_navigator_mode to unmarshal as %q, got %q", "line", decoded.ChatNavigatorMode)
	}
}

func TestStoreLoad_NormalizesChatNavigatorMode(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		want     string
	}{
		{name: "missing defaults to panel", contents: `{}`, want: "panel"},
		{name: "empty defaults to panel", contents: `{"chat_navigator_mode":""}`, want: "panel"},
		{name: "line is preserved", contents: `{"chat_navigator_mode":"line"}`, want: "line"},
		{name: "line is case-insensitive", contents: `{"chat_navigator_mode":" Line "}`, want: "line"},
		{name: "unknown falls back to panel", contents: `{"chat_navigator_mode":"dashes"}`, want: "panel"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, "gui_improvements.json")
			if err := os.WriteFile(configPath, []byte(tc.contents), 0600); err != nil {
				t.Fatalf("failed to write config: %v", err)
			}

			store, err := NewStore(tempDir)
			if err != nil {
				t.Fatalf("failed to create store: %v", err)
			}

			if got := store.GetConfig().ChatNavigatorMode; got != tc.want {
				t.Fatalf("expected chat navigator mode %q, got %q", tc.want, got)
			}
		})
	}
}

func TestUpdateConfigPartial_ChatNavigatorMode(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	updated, err := store.UpdateConfigPartial([]byte(`{"chat_navigator_mode":"line"}`))
	if err != nil {
		t.Fatalf("UpdateConfigPartial returned error: %v", err)
	}
	if updated.ChatNavigatorMode != "line" {
		t.Fatalf("expected chat navigator mode %q, got %q", "line", updated.ChatNavigatorMode)
	}

	// An omitted key must preserve the current value.
	updated, err = store.UpdateConfigPartial([]byte(`{"chat_telemetry_output_tokens":false}`))
	if err != nil {
		t.Fatalf("UpdateConfigPartial returned error: %v", err)
	}
	if updated.ChatNavigatorMode != "line" {
		t.Fatalf("expected omitted chat navigator mode to stay %q, got %q", "line", updated.ChatNavigatorMode)
	}

	// Unknown values fall back to the default.
	updated, err = store.UpdateConfigPartial([]byte(`{"chat_navigator_mode":"compact"}`))
	if err != nil {
		t.Fatalf("UpdateConfigPartial returned error: %v", err)
	}
	if updated.ChatNavigatorMode != "panel" {
		t.Fatalf("expected unknown chat navigator mode to fall back to %q, got %q", "panel", updated.ChatNavigatorMode)
	}

	// Legacy flat key spelling is accepted too.
	updated, err = store.UpdateConfigPartial([]byte(`{"chatnavigatormode":"line"}`))
	if err != nil {
		t.Fatalf("UpdateConfigPartial returned error: %v", err)
	}
	if updated.ChatNavigatorMode != "line" {
		t.Fatalf("expected legacy key to set chat navigator mode %q, got %q", "line", updated.ChatNavigatorMode)
	}
}
