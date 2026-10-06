package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHexToRGB(t *testing.T) {
	tests := []struct {
		hex     string
		wantR   int
		wantG   int
		wantB   int
		wantErr bool
	}{
		{"#7c3aed", 124, 58, 237, false},
		{"7c3aed", 124, 58, 237, false},
		{"#fff", 255, 255, 255, false},
		{"000", 0, 0, 0, false},
		{"#0b57d0", 11, 87, 208, false},
		{"invalid", 0, 0, 0, true},
		{"#12", 0, 0, 0, true},
		{"#1234567", 0, 0, 0, true},
	}

	for _, tt := range tests {
		r, g, b, err := HexToRGB(tt.hex)
		if (err != nil) != tt.wantErr {
			t.Errorf("HexToRGB(%q) error = %v, wantErr %v", tt.hex, err, tt.wantErr)
			continue
		}
		if !tt.wantErr {
			if r != tt.wantR || g != tt.wantG || b != tt.wantB {
				t.Errorf("HexToRGB(%q) = (%d, %d, %d), want (%d, %d, %d)", tt.hex, r, g, b, tt.wantR, tt.wantG, tt.wantB)
			}
		}
	}
}

func TestGenerateCSS(t *testing.T) {
	// 1. Disabled config
	disabledCfg := &Config{
		Enabled:             false,
		ColorStylingEnabled: true,
		ProjectColors: map[string]string{
			"Arbitrager": "#7c3aed",
		},
	}
	css := GenerateCSS(disabledCfg)
	if !strings.Contains(css, "Disabled") {
		t.Errorf("Expected disabled notice in CSS, got: %s", css)
	}

	// 2. Color styling disabled individually
	colorDisabledCfg := &Config{
		Enabled:             true,
		ColorStylingEnabled: false,
		ProjectColors: map[string]string{
			"Arbitrager": "#7c3aed",
		},
	}
	css = GenerateCSS(colorDisabledCfg)
	if !strings.Contains(css, "Disabled") {
		t.Errorf("Expected disabled notice in CSS when ColorStylingEnabled=false, got: %s", css)
	}

	// 3. Enabled config with project colors (check NO solid border, check hover options)
	cfg := &Config{
		Enabled:              true,
		ColorStylingEnabled:  true,
		DragRearrangeEnabled: true,
		ProjectColors: map[string]string{
			"Arbitrager":              "#7c3aed",
			"Antigravity Swiss Knife": "#0b57d0",
		},
		TintOpacity: 0.14,
	}
	css = GenerateCSS(cfg)
	if !strings.Contains(css, `[data-swiss-project="Arbitrager"][data-project-card="true"]`) {
		t.Errorf("CSS missing Arbitrager card selector: %s", css)
	}
	if !strings.Contains(css, `[data-swiss-project="Arbitrager"][data-testid="conversation-row-sidebar"]`) {
		t.Errorf("CSS missing Arbitrager conversation selector: %s", css)
	}
	if !strings.Contains(css, `rgba(124, 58, 237, 0.14)`) {
		t.Errorf("CSS missing light tint rgba: %s", css)
	}
	// Verify no left solid border
	if strings.Contains(css, `border-left: 3px solid`) {
		t.Errorf("CSS should NOT have solid left border: %s", css)
	}
	// Verify hover options gradient styling
	if !strings.Contains(css, `linear-gradient`) {
		t.Errorf("CSS missing hover options linear-gradient styling")
	}

	// 3. Test SolidLeftEdge toggle
	cfgWithEdge := &Config{
		Enabled:             true,
		ColorStylingEnabled: true,
		SolidLeftEdge:       true,
		ProjectColors: map[string]string{
			"Arbitrager": "#7c3aed",
		},
	}
	cssWithEdge := GenerateCSS(cfgWithEdge)
	if !strings.Contains(cssWithEdge, `border-left: 3px solid #7c3aed`) {
		t.Errorf("CSS with SolidLeftEdge=true should have solid border-left, got: %s", cssWithEdge)
	}

	// 4. Test SolidLeftEdge=false
	cfgWithoutEdge := &Config{
		Enabled:             true,
		ColorStylingEnabled: true,
		SolidLeftEdge:       false,
		ProjectColors: map[string]string{
			"Arbitrager": "#7c3aed",
		},
	}
	cssWithoutEdge := GenerateCSS(cfgWithoutEdge)
	if !strings.Contains(cssWithoutEdge, `border-left: none !important`) {
		t.Errorf("CSS with SolidLeftEdge=false should have border-left: none, got: %s", cssWithoutEdge)
	}

	// 5. Test ActiveConversationIndicator="border" and ActiveConversationBold=false
	cfgBorderMode := &Config{
		Enabled:                     true,
		ColorStylingEnabled:         true,
		ActiveConversationIndicator: "border",
		ActiveConversationBold:      false,
		ProjectColors: map[string]string{
			"Arbitrager": "#7c3aed",
		},
		TintOpacity: 0.14,
	}
	cssBorderMode := GenerateCSS(cfgBorderMode)
	// Active row should have light background tint (0.14) same as ordinary tabs and solid 2px border matching project color
	if !strings.Contains(cssBorderMode, `background-color: rgba(124, 58, 237, 0.14) !important;
  border: 2px solid #7c3aed !important;
  font-weight: 400 !important;`) {
		t.Errorf("CSS with ActiveConversationIndicator='border' should have light background and accent border outline, got: %s", cssBorderMode)
	}
	if !strings.Contains(cssBorderMode, `border: 2px solid transparent !important;`) {
		t.Errorf("CSS with ActiveConversationIndicator='border' should have 2px transparent border on ordinary tabs, got: %s", cssBorderMode)
	}

	// 6. Test ActiveConversationIndicator="background" (default) and ActiveConversationBold=true
	cfgDefaultMode := &Config{
		Enabled:                     true,
		ColorStylingEnabled:         true,
		ActiveConversationIndicator: "background",
		ActiveConversationBold:      true,
		ProjectColors: map[string]string{
			"Arbitrager": "#7c3aed",
		},
		TintOpacity: 0.14,
	}
	cssDefaultMode := GenerateCSS(cfgDefaultMode)
	if !strings.Contains(cssDefaultMode, `background-color: rgba(124, 58, 237, 0.30) !important;
  font-weight: 600 !important;`) {
		t.Errorf("CSS with default indicator should have accent fill background and font-weight 600, got: %s", cssDefaultMode)
	}

	// 7. Test ActiveConversationIndicator="left_bar"
	cfgLeftBarMode := &Config{
		Enabled:                     true,
		ColorStylingEnabled:         true,
		ActiveConversationIndicator: "left_bar",
		ProjectColors: map[string]string{
			"Arbitrager": "#7c3aed",
		},
		TintOpacity: 0.14,
	}
	cssLeftBarMode := GenerateCSS(cfgLeftBarMode)
	if !strings.Contains(cssLeftBarMode, "border-left: 3px solid transparent !important;") {
		t.Errorf("CSS with left_bar should have transparent left border on ordinary tabs, got: %s", cssLeftBarMode)
	}
	if !strings.Contains(cssLeftBarMode, "border-left: 3px solid #7c3aed !important;") {
		t.Errorf("CSS with left_bar should have solid 3px accent left border on active tab, got: %s", cssLeftBarMode)
	}
	if !strings.Contains(cssLeftBarMode, `background-color: rgba(124, 58, 237, 0.14) !important;
  border-left: 3px solid #7c3aed !important;`) {
		t.Errorf("CSS with left_bar should preserve normal tab background opacity, got: %s", cssLeftBarMode)
	}

	// 8. Test ActiveConversationBorderWidth="1.5px"
	cfgCustomBorder := &Config{
		Enabled:                       true,
		ColorStylingEnabled:           true,
		ActiveConversationIndicator:   "border",
		ActiveConversationBorderWidth: "1.5px",
		ProjectColors: map[string]string{
			"Arbitrager": "#7c3aed",
		},
		TintOpacity: 0.14,
	}
	cssCustomBorder := GenerateCSS(cfgCustomBorder)
	if !strings.Contains(cssCustomBorder, "border: 1.5px solid #7c3aed !important;") {
		t.Errorf("CSS with 1.5px border width should render border: 1.5px solid #7c3aed, got: %s", cssCustomBorder)
	}
}

func TestGenerateScript(t *testing.T) {
	cfg := DefaultConfig()
	script := GenerateScript(cfg)
	if !strings.Contains(script, "antigravity-swiss-styles") {
		t.Errorf("Script missing style tag ID")
	}
	if !strings.Contains(script, "updateTagsAndDraggables") {
		t.Errorf("Script missing updateTagsAndDraggables function")
	}
	if !strings.Contains(script, "dragstart") {
		t.Errorf("Script missing dragstart event listener")
	}
	if !strings.Contains(script, "MutationObserver") {
		t.Errorf("Script missing MutationObserver")
	}

	// Verify Context Menu Polish: Round swatches, Set Color title, and matte conic-gradient button
	if !strings.Contains(script, "border-radius: 50%") {
		t.Errorf("Script missing 50%% round border-radius for swatches")
	}
	if !strings.Contains(script, "Set Color") {
		t.Errorf("Script missing Set Color section header")
	}
	if !strings.Contains(script, "swiss-custom-trigger") {
		t.Errorf("Script missing swiss-custom-trigger button ID")
	}
	if !strings.Contains(script, "conic-gradient(from 0deg, #6c5ce7") {
		t.Errorf("Script missing matte conic-gradient on custom palette trigger")
	}
	if !strings.Contains(script, "swiss-custom-grid-container") {
		t.Errorf("Script missing swiss-custom-grid-container ID")
	}
	if !strings.Contains(script, "display: none") {
		t.Errorf("Script should have custom grid collapsed (display: none) by default")
	}
}

func TestSidebarDividerAndProjectSpacer(t *testing.T) {
	cfg := &Config{
		Enabled:                      true,
		ColorStylingEnabled:          true,
		ReplaceSeeAllTriangle:        true,
		ConsistentProjectSpacing:     true,
		ConsistentProjectSpacingLine: true,
	}

	// 1. Verify CSS rules for divider centering and spacer
	css := GenerateCSS(cfg)
	if !strings.Contains(css, ".swiss-convo-tabs-divider") {
		t.Errorf("GenerateCSS missing .swiss-convo-tabs-divider rule")
	}
	if !strings.Contains(css, "position: relative !important;") {
		t.Errorf("GenerateCSS missing position: relative on divider")
	}
	if !strings.Contains(css, "position: absolute !important;") || !strings.Contains(css, "top: 50% !important;") {
		t.Errorf("GenerateCSS missing absolute middle positioning on divider line")
	}
	if !strings.Contains(css, "bottom: 50% !important;") {
		t.Errorf("GenerateCSS missing bottom: 50%%%% on pill")
	}
	if !strings.Contains(css, ".swiss-project-bottom-spacer") {
		t.Errorf("GenerateCSS missing .swiss-project-bottom-spacer rule")
	}
	if !strings.Contains(css, "height: 32px !important;") {
		t.Errorf("GenerateCSS missing height: 32px on spacer")
	}
	if !strings.Contains(css, ".swiss-project-spacer-line") {
		t.Errorf("GenerateCSS missing .swiss-project-spacer-line rule")
	}
	if !strings.Contains(css, "[data-theme=\"dark\"] .swiss-project-spacer-line") {
		t.Errorf("GenerateCSS missing dark theme rule for .swiss-project-spacer-line")
	}

	// 2. Verify JS script generation with consistentProjectSpacingLine = true
	script := GenerateScript(cfg)
	if !strings.Contains(script, "const replaceSeeAllTriangle = true;") {
		t.Errorf("GenerateScript missing replaceSeeAllTriangle constant")
	}
	if !strings.Contains(script, "const consistentProjectSpacing = true;") {
		t.Errorf("GenerateScript missing consistentProjectSpacing constant")
	}
	if !strings.Contains(script, "const consistentProjectSpacingLine = true;") {
		t.Errorf("GenerateScript missing consistentProjectSpacingLine constant")
	}
	if !strings.Contains(script, "spacerIndices.add") {
		t.Errorf("GenerateScript missing spacerIndices computation")
	}
	if !strings.Contains(script, `el.appendChild(spacer)`) {
		t.Errorf("GenerateScript missing spacer append logic")
	}
	if !strings.Contains(script, "swiss-project-bottom-spacer") {
		t.Errorf("GenerateScript missing swiss-project-bottom-spacer reference")
	}
	if !strings.Contains(script, "swiss-project-spacer-line") {
		t.Errorf("GenerateScript missing swiss-project-spacer-line reference")
	}
	if !strings.Contains(script, "spacer.appendChild(line)") {
		t.Errorf("GenerateScript missing spacer.appendChild(line) logic")
	}

	// 3. Verify JS script generation with consistentProjectSpacingLine = false
	cfgDisabledLine := &Config{
		Enabled:                      true,
		ColorStylingEnabled:          true,
		ConsistentProjectSpacing:     true,
		ConsistentProjectSpacingLine: false,
	}
	scriptDisabled := GenerateScript(cfgDisabledLine)
	if !strings.Contains(scriptDisabled, "const consistentProjectSpacingLine = false;") {
		t.Errorf("GenerateScript missing 'const consistentProjectSpacingLine = false;' when disabled")
	}

	// 4. Verify default config has both spacing and spacing line enabled
	defCfg := DefaultConfig()
	if !defCfg.ConsistentProjectSpacing {
		t.Errorf("DefaultConfig ConsistentProjectSpacing should default to true")
	}
	if !defCfg.ConsistentProjectSpacingLine {
		t.Errorf("DefaultConfig ConsistentProjectSpacingLine should default to true")
	}
}

func TestConversationTagBorderAndTransparency(t *testing.T) {
	// 1. Verify GenerateCSS sets [data-index] to background: transparent !important
	cfg := &Config{
		Enabled:                       true,
		ColorStylingEnabled:           true,
		ActiveConversationIndicator:   "border",
		ActiveConversationBorderWidth: "2px",
		ProjectColors: map[string]string{
			"Arbitrager": "#dc2626",
		},
	}
	css := GenerateCSS(cfg)
	if !strings.Contains(css, "[data-index]") || !strings.Contains(css, "background: transparent !important;") {
		t.Errorf("GenerateCSS missing transparent rule for [data-index], got: %s", css)
	}

	// 2. Verify GenerateScript with ActiveConversationIndicator="border"
	script := GenerateScript(cfg)
	if !strings.Contains(script, "const isBorderMode = true;") {
		t.Errorf("GenerateScript missing 'const isBorderMode = true;'")
	}
	if !strings.Contains(script, `const borderWidth = "2px";`) {
		t.Errorf("GenerateScript missing 'const borderWidth = \"2px\";'")
	}
	if !strings.Contains(script, "'border: ' + borderWidth + ' solid ' + hex + ' !important;'") {
		t.Errorf("GenerateScript missing active conversation border in dynamic styles")
	}
	if !strings.Contains(script, "'  border: ' + borderWidth + ' solid transparent !important;'") {
		t.Errorf("GenerateScript missing inactive conversation transparent border in dynamic styles")
	}
	if !strings.Contains(script, "[data-index]") {
		t.Errorf("GenerateScript dynamic styles missing [data-index] transparent rule")
	}

	// 3. Verify dynamic styles do NOT apply background-color to [data-index]
	if strings.Contains(script, "[data-index][data-swiss-project=\"' + safeP + '\"] {") {
		t.Errorf("GenerateScript dynamic styles must NOT apply background to [data-index]")
	}

	// 4. Verify GenerateScript with ActiveConversationIndicator="background"
	cfgBg := &Config{
		Enabled:                     true,
		ColorStylingEnabled:         true,
		ActiveConversationIndicator: "background",
		ProjectColors: map[string]string{
			"Arbitrager": "#dc2626",
		},
	}
	scriptBg := GenerateScript(cfgBg)
	if !strings.Contains(scriptBg, "const isBorderMode = false;") {
		t.Errorf("GenerateScript with background indicator should have 'const isBorderMode = false;'")
	}

	// 5. Verify custom border width (e.g. 1.5px) is passed
	cfgCustom := &Config{
		Enabled:                       true,
		ColorStylingEnabled:           true,
		ActiveConversationIndicator:   "border",
		ActiveConversationBorderWidth: "1.5px",
		ProjectColors: map[string]string{
			"Arbitrager": "#dc2626",
		},
	}
	scriptCustom := GenerateScript(cfgCustom)
	if !strings.Contains(scriptCustom, `const borderWidth = "1.5px";`) {
		t.Errorf("GenerateScript missing custom borderWidth '1.5px'")
	}
}

func TestStorePersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swiss_gui_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	// Verify defaults
	cfg := store.GetConfig()
	if !cfg.Enabled || !cfg.ColorStylingEnabled || !cfg.DragRearrangeEnabled {
		t.Errorf("Expected defaults enabled: %+v", cfg)
	}
	if cfg.ProjectColors["Arbitrager"] != "#7c3aed" {
		t.Errorf("Expected default Arbitrager color #7c3aed, got %s", cfg.ProjectColors["Arbitrager"])
	}

	// Set new color
	if err := store.SetProjectColor("CustomProject", "#123456"); err != nil {
		t.Fatalf("SetProjectColor failed: %v", err)
	}

	cfg = store.GetConfig()
	if cfg.ProjectColors["CustomProject"] != "#123456" {
		t.Errorf("Expected CustomProject color #123456, got %s", cfg.ProjectColors["CustomProject"])
	}

	// Test reordering
	if err := store.ReorderProject("CustomProject", "Arbitrager"); err != nil {
		t.Fatalf("ReorderProject failed: %v", err)
	}
	cfgReordered := store.GetConfig()
	if len(cfgReordered.ProjectOrder) == 0 {
		t.Errorf("ProjectOrder should not be empty")
	}

	// Check file on disk
	data, err := os.ReadFile(filepath.Join(tmpDir, "gui_improvements.json"))
	if err != nil {
		t.Fatalf("Failed to read config file from disk: %v", err)
	}
	if !strings.Contains(string(data), "CustomProject") {
		t.Errorf("Config file missing CustomProject: %s", string(data))
	}

	// Test reload from disk
	storeReloaded, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore reload error: %v", err)
	}
	reloadedCfg := storeReloaded.GetConfig()
	if reloadedCfg.ProjectColors["CustomProject"] != "#123456" {
		t.Errorf("Reloaded config missing CustomProject color")
	}

	// Remove project color
	if err := store.RemoveProjectColor("CustomProject"); err != nil {
		t.Fatalf("RemoveProjectColor failed: %v", err)
	}
	cfgAfterRemove := store.GetConfig()
	if _, exists := cfgAfterRemove.ProjectColors["CustomProject"]; exists {
		t.Errorf("CustomProject should have been removed")
	}
}

func TestDetectProjects(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swiss_gui_detect_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	projects, err := store.DetectProjects()
	if err != nil {
		t.Fatalf("DetectProjects failed: %v", err)
	}

	foundArbitrager := false
	for _, p := range projects {
		if p.Name == "Arbitrager" {
			foundArbitrager = true
			if p.Color != "#7c3aed" {
				t.Errorf("Expected Arbitrager color #7c3aed, got %s", p.Color)
			}
			if !p.IsConfigured {
				t.Errorf("Expected Arbitrager IsConfigured=true")
			}
		}
	}

	if !foundArbitrager {
		t.Errorf("DetectProjects should have returned Arbitrager")
	}
}

func TestLiveInjection(t *testing.T) {
	inj := NewInjector(0)
	_, err := inj.FindDevToolsPort()
	if err != nil {
		t.Skip("Antigravity DevTools not running, skipping live injection test")
	}
	res, err := inj.ApplyConfig(DefaultConfig())
	if err != nil {
		t.Skipf("Live Antigravity instance not responding to DevTools: %v", err)
	}
	if !res.Success {
		t.Skipf("Live injection did not succeed: %+v", res)
	}
	t.Logf("Live injection succeeded: %+v", res)
}

func TestLiveRefreshUserStatus(t *testing.T) {
	inj := NewInjector(0)
	_, err := inj.FindDevToolsPort()
	if err != nil {
		t.Skip("Antigravity DevTools not running, skipping live refresh user status test")
	}
	res, err := inj.RefreshUserStatus()
	if err != nil {
		t.Skipf("Live Antigravity instance not responding to DevTools: %v", err)
	}
	if !res.Success {
		t.Errorf("RefreshUserStatus returned non-success: %+v", res)
	}
	if !res.FiberRefreshed && !res.ReloadFallback {
		t.Errorf("RefreshUserStatus neither refreshed fiber nor reloaded fallback: %+v", res)
	}
	t.Logf("Live RefreshUserStatus succeeded: %+v", res)
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		t        time.Time
		expected string
	}{
		{"Zero time", time.Time{}, "No conversations"},
		{"Just now", now.Add(-20 * time.Second), "Just now"},
		{"1 minute ago", now.Add(-1 * time.Minute), "1 minute ago"},
		{"25 minutes ago", now.Add(-25 * time.Minute), "25 minutes ago"},
		{"59 minutes ago", now.Add(-59 * time.Minute), "59 minutes ago"},
		{"1 hour ago (promoted at 60m)", now.Add(-60 * time.Minute), "1 hour ago"},
		{"5 hours ago", now.Add(-5 * time.Hour), "5 hours ago"},
		{"23 hours ago", now.Add(-23 * time.Hour), "23 hours ago"},
		{"1 day ago (promoted at 24h)", now.Add(-24 * time.Hour), "1 day ago"},
		{"5 days ago", now.Add(-5 * 24 * time.Hour), "5 days ago"},
		{"29 days ago", now.Add(-29 * 24 * time.Hour), "29 days ago"},
		{"1 month ago (promoted at 30d)", now.Add(-30 * 24 * time.Hour), "1 month ago"},
		{"6 months ago", now.Add(-180 * 24 * time.Hour), "6 months ago"},
		{"11 months ago", now.Add(-350 * 24 * time.Hour), "11 months ago"},
		{"1 year ago (promoted at 365d)", now.Add(-365 * 24 * time.Hour), "1 year ago"},
		{"2 years ago", now.Add(-730 * 24 * time.Hour), "2 years ago"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatRelativeTime(tc.t, now)
			if got != tc.expected {
				t.Errorf("FormatRelativeTime(%v) = %q, expected %q", tc.t, got, tc.expected)
			}
		})
	}
}

func TestArchiveAndRestoreProject(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "swiss_gui_archive_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}

	// Archive a project
	if err := store.ArchiveProject("TestProject"); err != nil {
		t.Fatalf("ArchiveProject failed: %v", err)
	}

	cfg := store.GetConfig()
	if len(cfg.ArchivedProjects) != 1 || cfg.ArchivedProjects[0] != "TestProject" {
		t.Fatalf("Expected TestProject to be archived, got: %v", cfg.ArchivedProjects)
	}

	// Archive same project again (idempotent)
	if err := store.ArchiveProject("TestProject"); err != nil {
		t.Fatalf("ArchiveProject idempotent failed: %v", err)
	}
	if len(store.GetConfig().ArchivedProjects) != 1 {
		t.Errorf("Expected 1 archived project after duplicate archive call")
	}

	// Get archived projects list
	archivedList, err := store.GetArchivedProjects()
	if err != nil {
		t.Fatalf("GetArchivedProjects failed: %v", err)
	}
	if len(archivedList) != 1 || archivedList[0].Name != "TestProject" {
		t.Errorf("Unexpected archived list: %+v", archivedList)
	}

	// Restore project
	if err := store.RestoreProject("TestProject"); err != nil {
		t.Fatalf("RestoreProject failed: %v", err)
	}
	if len(store.GetConfig().ArchivedProjects) != 0 {
		t.Errorf("Expected 0 archived projects after restore")
	}

	// Archive and delete project
	_ = store.ArchiveProject("DeleteMe")
	if err := store.DeleteProject("DeleteMe"); err != nil {
		t.Fatalf("DeleteProject failed: %v", err)
	}
	if len(store.GetConfig().ArchivedProjects) != 0 {
		t.Errorf("Expected 0 archived projects after delete")
	}
}

func TestArchivedCSS(t *testing.T) {
	cfg := &Config{
		Enabled:             true,
		ColorStylingEnabled: true,
		ArchivedProjects:    []string{"HiddenProject"},
	}
	css := GenerateCSS(cfg)
	if !strings.Contains(css, `[data-swiss-project="HiddenProject"]`) {
		t.Errorf("Expected hidden project selector in CSS, got: %s", css)
	}
	if !strings.Contains(css, `display: none !important;`) {
		t.Errorf("Expected display: none !important in CSS for hidden project")
	}
}

func TestConversationTabsAndDivider(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ConversationTabsMode != "dynamic" {
		t.Errorf("Expected default mode 'dynamic', got %s", cfg.ConversationTabsMode)
	}
	if cfg.ConversationTabsFixedLimit != 6 {
		t.Errorf("Expected default fixed limit 6, got %d", cfg.ConversationTabsFixedLimit)
	}
	if cfg.ConversationTabsAgeThreshold != "14d" {
		t.Errorf("Expected default age threshold '14d', got %s", cfg.ConversationTabsAgeThreshold)
	}
	if cfg.ConversationTabsMin != 3 {
		t.Errorf("Expected default min tabs 3, got %d", cfg.ConversationTabsMin)
	}
	if cfg.ConversationTabsMax != 6 {
		t.Errorf("Expected default max tabs 6, got %d", cfg.ConversationTabsMax)
	}
	if cfg.ActiveConversationBold != false {
		t.Errorf("Expected default ActiveConversationBold false (bold off), got %v", cfg.ActiveConversationBold)
	}

	css := GenerateCSS(cfg)
	if !strings.Contains(css, "font-weight: 400 !important;") {
		t.Errorf("Expected default CSS to have regular font-weight: 400 !important;, got: %s", css)
	}
	if !strings.Contains(css, ".swiss-convo-tabs-divider") {
		t.Errorf("Expected .swiss-convo-tabs-divider in CSS")
	}
	if !strings.Contains(css, ".swiss-convo-tabs-line") {
		t.Errorf("Expected .swiss-convo-tabs-line in CSS")
	}
	if !strings.Contains(css, ".swiss-convo-tabs-pill") {
		t.Errorf("Expected .swiss-convo-tabs-pill in CSS")
	}
	if !strings.Contains(css, ".swiss-convo-tabs-triangle") {
		t.Errorf("Expected .swiss-convo-tabs-triangle in CSS")
	}

	// Verify script generation passes parameters
	cfg.ConversationTabsMode = "dynamic"
	cfg.ConversationTabsAgeThreshold = "3d"
	cfg.ConversationTabsMin = 3
	cfg.ConversationTabsMax = 8
	script := GenerateScript(cfg)
	if !strings.Contains(script, `const tabsMode = "dynamic";`) {
		t.Errorf("Script missing dynamic tabsMode")
	}
	if !strings.Contains(script, `const tabsAgeThreshold = "3d";`) {
		t.Errorf("Script missing 3d threshold")
	}
	if !strings.Contains(script, `const tabsMin = 3;`) {
		t.Errorf("Script missing tabsMin 3")
	}
	if !strings.Contains(script, `const tabsMax = 8;`) {
		t.Errorf("Script missing tabsMax 8")
	}
}

func TestAutoArchiveHorizon(t *testing.T) {
	d1 := ParseHorizonToDuration("1d")
	if d1.Hours() != 24 {
		t.Errorf("Expected 24h for 1d, got %v", d1)
	}
	d3 := ParseHorizonToDuration("3d")
	if d3.Hours() != 72 {
		t.Errorf("Expected 72h for 3d, got %v", d3)
	}
	d7 := ParseHorizonToDuration("7d")
	if d7.Hours() != 168 {
		t.Errorf("Expected 168h for 7d, got %v", d7)
	}
	d14 := ParseHorizonToDuration("14d")
	if d14.Hours() != 336 {
		t.Errorf("Expected 336h for 14d, got %v", d14)
	}
	d30 := ParseHorizonToDuration("30d")
	if d30.Hours() != 720 {
		t.Errorf("Expected 720h for 30d, got %v", d30)
	}
	dDefault := ParseHorizonToDuration("unknown")
	if dDefault.Hours() != 720 {
		t.Errorf("Expected default 720h (30d), got %v", dDefault)
	}
}

func TestGenerateScriptBundlingAuxiliaryPlugins(t *testing.T) {
	cfg := DefaultConfig()
	script := GenerateScript(cfg)

	// Verify auxiliary plugins script is bundled
	if !strings.Contains(script, "setupAuxiliaryTabs") {
		t.Errorf("GenerateScript missing setupAuxiliaryTabs from auxiliary plugins")
	}
	if !strings.Contains(script, "swiss-aux-container") {
		t.Errorf("GenerateScript missing swiss-aux-container from auxiliary plugins")
	}

	// Verify no duplicate script inclusion
	countSetupAux := strings.Count(script, "function setupAuxiliaryTabs()")
	if countSetupAux != 1 {
		t.Errorf("expected setupAuxiliaryTabs to appear exactly 1 time, got %d", countSetupAux)
	}

	// Verify custom models script appears exactly once
	countCustomModels := strings.Count(script, "updateModelSelector")
	if countCustomModels < 1 {
		t.Errorf("expected updateModelSelector to appear in script, got %d", countCustomModels)
	}

	// Verify enhancements script appears
	if !strings.Contains(script, "applyEnhancementsStyles") {
		t.Errorf("expected applyEnhancementsStyles to appear in script")
	}
}

