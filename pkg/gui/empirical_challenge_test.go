package gui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/core"
)

// TestChallenge_UpdateConfigPartial_AdversarialInputs stress-tests UpdateConfigPartial
// with malformed, empty, null, and boundary payloads.
func TestChallenge_UpdateConfigPartial_AdversarialInputs(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	if err := store.SetProjectColor("ProjectAlpha", "#0b57d0"); err != nil {
		t.Fatalf("failed to set project color: %v", err)
	}

	initialCfg := store.GetConfig()

	testCases := []struct {
		name        string
		payload     []byte
		expectError bool
		check       func(t *testing.T, updated *GUIConfig)
	}{
		{
			name:        "Empty JSON object",
			payload:     []byte(`{}`),
			expectError: false,
			check: func(t *testing.T, updated *GUIConfig) {
				if !updated.Enabled || !updated.ColorStylingEnabled || !updated.AutoInject {
					t.Errorf("empty JSON reset flags to false: %+v", updated)
				}
				if len(updated.ProjectColors) == 0 {
					t.Errorf("empty JSON cleared ProjectColors")
				}
			},
		},
		{
			name:        "Null JSON literal",
			payload:     []byte(`null`),
			expectError: false,
			check: func(t *testing.T, updated *GUIConfig) {
				if !updated.Enabled || !updated.ColorStylingEnabled {
					t.Errorf("null payload reset flags: %+v", updated)
				}
				if len(updated.ProjectColors) == 0 {
					t.Errorf("null payload cleared ProjectColors")
				}
			},
		},
		{
			name:        "Whitespace only",
			payload:     []byte(`   `),
			expectError: true,
			check:       nil,
		},
		{
			name:        "Malformed unclosed JSON",
			payload:     []byte(`{"enabled": true`),
			expectError: true,
			check:       nil,
		},
		{
			name:        "JSON array instead of object",
			payload:     []byte(`[1, 2, 3]`),
			expectError: true,
			check:       nil,
		},
		{
			name:        "JSON number literal",
			payload:     []byte(`12345`),
			expectError: true,
			check:       nil,
		},
		{
			name:        "JSON string literal",
			payload:     []byte(`"some_string"`),
			expectError: true,
			check:       nil,
		},
		{
			name:        "Unknown random fields ignored",
			payload:     []byte(`{"unknown_custom_field": "val", "non_existent_id": 999}`),
			expectError: false,
			check: func(t *testing.T, updated *GUIConfig) {
				if !updated.Enabled || !updated.ColorStylingEnabled {
					t.Errorf("unknown fields corrupted config: %+v", updated)
				}
			},
		},
		{
			name:        "Null values for primitive flags",
			payload:     []byte(`{"enabled": null, "color_styling_enabled": null, "project_colors": null}`),
			expectError: false,
			check: func(t *testing.T, updated *GUIConfig) {
				if !updated.Enabled || !updated.ColorStylingEnabled {
					t.Errorf("null values corrupted boolean flags: %+v", updated)
				}
				if len(updated.ProjectColors) == 0 {
					t.Errorf("null project_colors cleared map")
				}
			},
		},
		{
			name:        "Empty project colors map preserves existing",
			payload:     []byte(`{"project_colors": {}}`),
			expectError: false,
			check: func(t *testing.T, updated *GUIConfig) {
				if len(updated.ProjectColors) == 0 {
					t.Errorf("empty project_colors wiped out colors")
				}
				for k, v := range initialCfg.ProjectColors {
					if updated.ProjectColors[k] != v {
						t.Errorf("expected color %q for %q, got %q", v, k, updated.ProjectColors[k])
					}
				}
			},
		},
		{
			name:        "Case insensitivity and mixed case",
			payload:     []byte(`{"ENABLED": false, "Color_Styling_Enabled": false}`),
			expectError: false,
			check: func(t *testing.T, updated *GUIConfig) {
				if updated.Enabled {
					t.Errorf("expected ENABLED=false to take effect")
				}
				if updated.ColorStylingEnabled {
					t.Errorf("expected Color_Styling_Enabled=false to take effect")
				}
				// Other flags must remain preserved
				if !updated.AutoInject {
					t.Errorf("expected AutoInject to remain true")
				}
			},
		},
		{
			name:        "Boundary clamping for tabs fixed limit and opacity",
			payload:     []byte(`{"conversation_tabs_fixed_limit": 999, "tint_opacity": -0.5}`),
			expectError: false,
			check: func(t *testing.T, updated *GUIConfig) {
				if updated.ConversationTabsFixedLimit != 6 {
					t.Errorf("expected ConversationTabsFixedLimit clamped to 6, got %d", updated.ConversationTabsFixedLimit)
				}
				if updated.TintOpacity != 0.15 {
					t.Errorf("expected TintOpacity clamped to 0.15, got %f", updated.TintOpacity)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := store.UpdateConfigPartial(tc.payload)
			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error for payload %q, got nil", string(tc.payload))
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for payload %q: %v", string(tc.payload), err)
				}
				if tc.check != nil {
					tc.check(t, res)
				}
			}
		})
	}
}

// TestChallenge_UpdateConfigPartial_IncrementalColorMerge tests multiple incremental
// partial updates adding and overriding individual project colors.
func TestChallenge_UpdateConfigPartial_IncrementalColorMerge(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	if err := store.SetProjectColor("ProjectBase", "#0b57d0"); err != nil {
		t.Fatalf("failed to set base color: %v", err)
	}

	// 1. Add ProjectA
	u1, err := store.UpdateConfigPartial([]byte(`{"project_colors": {"ProjectA": "#111111"}}`))
	if err != nil {
		t.Fatalf("step 1 failed: %v", err)
	}
	if u1.ProjectColors["ProjectA"] != "#111111" {
		t.Fatalf("expected ProjectA=#111111, got %q", u1.ProjectColors["ProjectA"])
	}
	if u1.ProjectColors["ProjectBase"] != "#0b57d0" {
		t.Fatalf("base color lost after step 1")
	}

	// 2. Add ProjectB and toggle one telemetry flag
	u2, err := store.UpdateConfigPartial([]byte(`{"project_colors": {"ProjectB": "#222222"}, "chat_telemetry_scope": "per_chat"}`))
	if err != nil {
		t.Fatalf("step 2 failed: %v", err)
	}
	if u2.ProjectColors["ProjectA"] != "#111111" || u2.ProjectColors["ProjectB"] != "#222222" {
		t.Fatalf("incremental colors lost in step 2: %+v", u2.ProjectColors)
	}
	if u2.ProjectColors["ProjectBase"] != "#0b57d0" {
		t.Fatalf("base color lost after step 2")
	}
	if u2.ChatTelemetryScope != "per_chat" {
		t.Fatalf("expected per_chat, got %q", u2.ChatTelemetryScope)
	}
	if !u2.Enabled || !u2.ColorStylingEnabled {
		t.Fatalf("flags corrupted in step 2")
	}

	// 3. Update ProjectA color, ProjectB and base colors remain
	u3, err := store.UpdateConfigPartial([]byte(`{"project_colors": {"ProjectA": "#333333"}}`))
	if err != nil {
		t.Fatalf("step 3 failed: %v", err)
	}
	if u3.ProjectColors["ProjectA"] != "#333333" {
		t.Fatalf("expected ProjectA=#333333, got %q", u3.ProjectColors["ProjectA"])
	}
	if u3.ProjectColors["ProjectB"] != "#222222" {
		t.Fatalf("ProjectB lost after step 3")
	}
	if u3.ProjectColors["ProjectBase"] != "#0b57d0" {
		t.Fatalf("base color lost after step 3")
	}
}

// TestChallenge_UpdateConfigPartial_ConcurrentSafety runs high-concurrency partial updates
// with -race to verify lock integrity and data consistency.
func TestChallenge_UpdateConfigPartial_ConcurrentSafety(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	const goroutines = 30
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		idx := i
		go func() {
			defer wg.Done()
			payload := map[string]interface{}{
				"project_colors": map[string]string{
					fmt.Sprintf("Proj_%d", idx): fmt.Sprintf("#%06x", idx*1000),
				},
				"tint_opacity": 0.10 + float64(idx%10)*0.01,
			}
			data, _ := json.Marshal(payload)
			_, uErr := store.UpdateConfigPartial(data)
			if uErr != nil {
				t.Errorf("concurrent update failed: %v", uErr)
			}
		}()
	}

	wg.Wait()

	finalCfg := store.GetConfig()
	if !finalCfg.Enabled || !finalCfg.ColorStylingEnabled || !finalCfg.AutoInject {
		t.Fatalf("concurrent updates corrupted boolean flags: %+v", finalCfg)
	}
	for i := 0; i < goroutines; i++ {
		expectedKey := fmt.Sprintf("Proj_%d", i)
		if _, ok := finalCfg.ProjectColors[expectedKey]; !ok {
			t.Errorf("expected %q to be present in merged project colors", expectedKey)
		}
	}
}

// TestChallenge_RealUserConfigIsolation verifies that creating a store without a custom
// path (defaulting to core.GetConfigDir()) inside tests writes to /tmp and NEVER touches
// the real user configuration directory (~/.config/antigravity-swiss/).
func TestChallenge_RealUserConfigIsolation(t *testing.T) {
	realUserConfigDir := filepath.Join(os.Getenv("HOME"), ".config", "antigravity-swiss")
	realGUIFile := filepath.Join(realUserConfigDir, "gui_improvements.json")
	realCSSFile := filepath.Join(realUserConfigDir, "persistent_styles.css")
	realJSFile := filepath.Join(realUserConfigDir, "persistent_script.js")

	var realGUIMtime, realCSSMtime, realJSMtime int64
	if fi, err := os.Stat(realGUIFile); err == nil {
		realGUIMtime = fi.ModTime().UnixNano()
	}
	if fi, err := os.Stat(realCSSFile); err == nil {
		realCSSMtime = fi.ModTime().UnixNano()
	}
	if fi, err := os.Stat(realJSFile); err == nil {
		realJSMtime = fi.ModTime().UnixNano()
	}

	// Ensure ANTIGRAVITY_SWISS_CONFIG_DIR is not set so GetConfigDir() uses its test branch
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", "")

	// 1. Verify GetConfigDir() in test environment
	configDir := core.GetConfigDir()
	expectedPrefix := filepath.Clean(os.TempDir())
	prefixWithSep := expectedPrefix + string(filepath.Separator)
	if !filepath.IsAbs(configDir) || (configDir != expectedPrefix && !strings.HasPrefix(configDir, prefixWithSep)) {
		t.Fatalf("GetConfigDir() returned non-isolated path %q, expected prefix %q", configDir, expectedPrefix)
	}

	// 2. Instantiate store with empty configDir (invokes core.GetConfigDir())
	store, err := NewStore("")
	if err != nil {
		t.Fatalf("failed to create store with empty dir: %v", err)
	}

	// 3. Mutate store config and trigger sync
	_, err = store.UpdateConfigPartial([]byte(`{"project_colors": {"EmpiricalChallengeProject": "#abcdef"}}`))
	if err != nil {
		t.Fatalf("UpdateConfigPartial failed: %v", err)
	}
	if err := store.SyncPersistentFiles(); err != nil {
		t.Fatalf("SyncPersistentFiles failed: %v", err)
	}

	// 4. Verify test files exist in the isolated test dir
	isolatedFile := filepath.Join(configDir, "gui_improvements.json")
	if _, err := os.Stat(isolatedFile); err != nil {
		t.Fatalf("expected isolated file %q to exist, err: %v", isolatedFile, err)
	}

	// 5. Verify real user config files were NOT modified
	if fi, err := os.Stat(realGUIFile); err == nil {
		if fi.ModTime().UnixNano() != realGUIMtime {
			t.Fatalf("CRITICAL ISOLATION BREACH: real user %q was modified! Mtime changed from %d to %d",
				realGUIFile, realGUIMtime, fi.ModTime().UnixNano())
		}
	}
	if fi, err := os.Stat(realCSSFile); err == nil {
		if fi.ModTime().UnixNano() != realCSSMtime {
			t.Fatalf("CRITICAL ISOLATION BREACH: real user %q was modified! Mtime changed from %d to %d",
				realCSSFile, realCSSMtime, fi.ModTime().UnixNano())
		}
	}
	if fi, err := os.Stat(realJSFile); err == nil {
		if fi.ModTime().UnixNano() != realJSMtime {
			t.Fatalf("CRITICAL ISOLATION BREACH: real user %q was modified! Mtime changed from %d to %d",
				realJSFile, realJSMtime, fi.ModTime().UnixNano())
		}
	}
}
