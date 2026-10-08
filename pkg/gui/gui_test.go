package gui

import (
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
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

	// 9. Test dynamic luminance contrast safeguard for project card text and icons
	cfgContrast := &Config{
		Enabled:             true,
		ColorStylingEnabled: true,
		ProjectColors: map[string]string{
			"LightProject": "#ffffff",
			"DarkProject":  "#0f172a",
		},
	}
	cssContrast := GenerateCSS(cfgContrast)
	// Light project card (lum > 0.6) must use dark slate text #0f172a for legibility
	if !strings.Contains(cssContrast, `[data-swiss-project="LightProject"][data-project-card="true"],
[data-swiss-project="LightProject"] [data-project-card="true"] {
  background-color: #ffffff !important;
  color: #0f172a !important;`) {
		t.Errorf("CSS with light project color should have #0f172a text contrast, got: %s", cssContrast)
	}
	if !strings.Contains(cssContrast, `[data-swiss-project="LightProject"] button[aria-label="Project options"] svg,
[data-swiss-project="LightProject"] button[aria-label*="conversation"] svg {
  color: #0f172a !important;
  fill: #0f172a !important;`) {
		t.Errorf("CSS with light project color should have #0f172a action icon contrast, got: %s", cssContrast)
	}
	// Dark project card (lum <= 0.6) must use white text #ffffff
	if !strings.Contains(cssContrast, `[data-swiss-project="DarkProject"][data-project-card="true"],
[data-swiss-project="DarkProject"] [data-project-card="true"] {
  background-color: #0f172a !important;
  color: #ffffff !important;`) {
		t.Errorf("CSS with dark project color should have #ffffff text contrast, got: %s", cssContrast)
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

	// Verify Context Menu Polish: Round swatches, Set Color title, and vibrant rainbow conic-gradient button
	if !strings.Contains(script, "border-radius: 50%") {
		t.Errorf("Script missing 50%% round border-radius for swatches")
	}
	if !strings.Contains(script, "Set Color") {
		t.Errorf("Script missing Set Color section header")
	}
	if !strings.Contains(script, "swiss-custom-trigger") {
		t.Errorf("Script missing swiss-custom-trigger button ID")
	}
	if !strings.Contains(script, "conic-gradient(red, yellow, lime, aqua, blue, magenta, red)") {
		t.Errorf("Script missing vibrant rainbow conic-gradient on custom palette trigger")
	}
	if !strings.Contains(script, "swiss-custom-grid-container") {
		t.Errorf("Script missing swiss-custom-grid-container ID")
	}
	if !strings.Contains(script, "display: none") {
		t.Errorf("Script should have custom grid collapsed (display: none) by default")
	}
	// Verify 7 quick-pick preset swatches including Slate Black and Slate Grey
	if !strings.Contains(script, "#0f172a") || !strings.Contains(script, "#64748b") {
		t.Errorf("Script missing Slate Black or Slate Grey swatches")
	}
	// Verify clean square 10x10 palette cells use circular border-radius: 50% and crisp neutral inset outline
	if !strings.Contains(script, `<div class="swiss-grid-cell" data-color="' + c + '" style="width: 16px; height: 16px; border-radius: 50%; background: ' + c + '; cursor: pointer; transition: transform 0.12s; box-shadow: inset 0 0 0 1px rgba(128, 128, 128, 0.25); box-sizing: border-box;"></div>`) {
		t.Errorf("Script missing circular swiss-grid-cell with rgba(128, 128, 128, 0.25) inset outline")
	}
	if !strings.Contains(script, `<div class="swiss-preset-swatch swiss-menu-swatch" data-color="' + p.hex + '" title="' + p.name + '" style="width: 15px; height: 15px; border-radius: 50%; background: ' + p.hex + '; cursor: pointer; transition: transform 0.12s; box-shadow: inset 0 0 0 1px rgba(128, 128, 128, 0.25); flex-shrink: 0; box-sizing: border-box;"></div>`) {
		t.Errorf("Script missing swiss-preset-swatch with rgba(128, 128, 128, 0.25) inset outline")
	}
	// Verify greyscale ramp in 10x10 palette
	if !strings.Contains(script, "#ffffff") || !strings.Contains(script, "#09090b") {
		t.Errorf("Script missing greyscale ramp in 10x10 palette")
	}
	// Verify dynamic in-DOM contrast safeguard
	if !strings.Contains(script, "lum > 0.6 ? '#0f172a' : '#ffffff'") {
		t.Errorf("Script missing dynamic luminance contrast safeguard")
	}
	// Verify zero-channel RGB hex parsing does not use falsy || fallback
	if !strings.Contains(script, "if (!isNaN(pr) && !isNaN(pg) && !isNaN(pb))") {
		t.Errorf("Script missing !isNaN RGB hex channel parsing check")
	}
	// Verify bumped menu version tag for live re-injection cache invalidation
	if !strings.Contains(script, `const currentVer = "v3_palette_10x10"`) {
		t.Errorf("Script missing v3_palette_10x10 menu version tag")
	}
	// Verify generated script passes node --check without syntax errors
	if _, err := exec.LookPath("node"); err == nil {
		cmd := exec.Command("node", "--check")
		cmd.Stdin = strings.NewReader(script)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("node syntax error in GenerateScript: %v\n%s", err, string(out))
		}
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
	if !strings.Contains(css, "height: 10px !important;") {
		t.Errorf("GenerateCSS missing slim height: 10px on divider button")
	}
	if !strings.Contains(css, "height: 14px !important;") {
		t.Errorf("GenerateCSS missing height: 14px on divider row container")
	}
	if strings.Contains(css, ".swiss-convo-tabs-line") {
		t.Errorf("GenerateCSS should have removed .swiss-convo-tabs-line from show/hide button")
	}
	if !strings.Contains(css, ".swiss-convo-tabs-pill") {
		t.Errorf("GenerateCSS missing .swiss-convo-tabs-pill rule")
	}
	if !strings.Contains(css, ".swiss-project-bottom-spacer") {
		t.Errorf("GenerateCSS missing .swiss-project-bottom-spacer rule")
	}
	if !strings.Contains(css, "height: 0 !important;") {
		t.Errorf("GenerateCSS missing height: 0 on spacer")
	}
	if !strings.Contains(css, "height: 4px !important;") {
		t.Errorf("GenerateCSS missing height: 4px on divider spacer")
	}
	if !strings.Contains(css, ".swiss-project-spacer-line") {
		t.Errorf("GenerateCSS missing .swiss-project-spacer-line rule")
	}
	if !strings.Contains(css, "top: 2px !important;") {
		t.Errorf("GenerateCSS missing top: 2px on spacer line")
	}
	if !strings.Contains(css, "[data-theme=\"dark\"] .swiss-project-spacer-line") {
		t.Errorf("GenerateCSS missing dark theme rule for .swiss-project-spacer-line")
	}
	if !strings.Contains(css, "flex-direction: column !important;") {
		t.Errorf("GenerateCSS missing flex-direction: column on divider row container to stack button above spacer")
	}
	if !strings.Contains(css, "z-index: 2 !important;") {
		t.Errorf("GenerateCSS missing z-index: 2 on divider container to elevate stacking context")
	}
	if !strings.Contains(css, "justify-content: center !important;") {
		t.Errorf("GenerateCSS missing justify-content: center on divider elements")
	}
	if !strings.Contains(css, "overflow: visible !important;") {
		t.Errorf("GenerateCSS missing overflow: visible on divider container")
	}

	// 2. Verify JS script generation with consistentProjectSpacingLine = true
	script := GenerateScript(cfg)
	if !strings.Contains(script, "const replaceSeeAllTriangle = true;") {
		t.Errorf("GenerateScript missing replaceSeeAllTriangle constant")
	}
	if !strings.Contains(script, `btn.style.setProperty("justify-content", "center", "important")`) {
		t.Errorf("GenerateScript missing inline horizontal centering on divider button")
	}
	if !strings.Contains(script, `btnParent.style.setProperty("justify-content", "center", "important")`) {
		t.Errorf("GenerateScript missing inline horizontal centering on divider button parent")
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
	if !strings.Contains(script, "curProj.emptyPlaceholderIdx = idx;") {
		t.Errorf("GenerateScript missing emptyPlaceholderIdx tracking for empty projects")
	}
	if !strings.Contains(script, `it.type === "section-header"`) || !strings.Contains(script, "curProj = null;") {
		t.Errorf("GenerateScript missing section-header boundary reset for curProj")
	}
	if !strings.Contains(script, "spacerIndices.add(p.emptyPlaceholderIdx)") {
		t.Errorf("GenerateScript missing spacerIndices.add(p.emptyPlaceholderIdx) for bottom/empty projects")
	}
	if !strings.Contains(script, "spacerIndices.add(p.showMoreIdx)") {
		t.Errorf("GenerateScript missing spacerIndices.add(p.showMoreIdx) fallback when replaceSeeAllTriangle is false")
	}
	if !strings.Contains(script, "root.stateNode.current !== root") || !strings.Contains(script, "fiber = fiber.alternate;") {
		t.Errorf("GenerateScript missing active React Fiber alternate tree resolution")
	}
	if !strings.Contains(script, `"aria-expanded"`) {
		t.Errorf("GenerateScript missing aria-expanded in MutationObserver attributeFilter")
	}

	// 2b. Verify GenerateCSS still emits spacer rules when ColorStylingEnabled is false
	cfgSpacingOnly := &Config{
		Enabled:                      true,
		ColorStylingEnabled:          false,
		ConsistentProjectSpacing:     true,
		ConsistentProjectSpacingLine: true,
	}
	cssSpacingOnly := GenerateCSS(cfgSpacingOnly)
	if !strings.Contains(cssSpacingOnly, ".swiss-project-bottom-spacer") || !strings.Contains(cssSpacingOnly, ".swiss-project-spacer-line") {
		t.Errorf("GenerateCSS with ColorStylingEnabled=false and ConsistentProjectSpacing=true should still emit spacer CSS")
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
	port, err := inj.FindDevToolsPort()
	if err != nil {
		t.Skip("Antigravity DevTools not running, skipping live injection test")
	}
	cfg := DefaultConfig()
	if store, sErr := NewStore(""); sErr == nil {
		loaded := store.GetConfig()
		cfg = &loaded
	}
	res, err := inj.ApplyConfig(cfg)
	if err != nil {
		t.Skipf("Live Antigravity instance not responding to DevTools: %v", err)
	}
	if !res.Success {
		t.Skipf("Live injection did not succeed: %+v", res)
	}
	t.Logf("Live injection succeeded: %+v", res)

	pages, err := inj.GetPageTargets(port)
	if err == nil && len(pages) > 0 {
		verifyExpr := `(() => {
			const prevClicked = window.__swissLastClickedProject;
			const mockMenu = document.createElement("div");
			mockMenu.setAttribute("role", "menu");
			mockMenu.style.cssText = "position: fixed; left: -9999px; top: -9999px; pointer-events: none;";
			mockMenu.innerHTML = '<div role="menuitem"><div data-testid="group-header-delete-item">Delete</div></div>';
			document.body.prepend(mockMenu);
			try {
				window.__swissLastClickedProject = { name: "Antigravity Swiss Knife", time: Date.now() };
				if (typeof window.__swissEnhanceProjectOptionsMenu === "function") {
					window.__swissEnhanceProjectOptionsMenu();
				}
				const presets = mockMenu.querySelectorAll(".swiss-preset-swatch");
				const gridCells = mockMenu.querySelectorAll(".swiss-grid-cell");
				const trigger = mockMenu.querySelector("#swiss-custom-trigger");
				const pStyle = presets[0] ? window.getComputedStyle(presets[0]) : null;
				const gStyle = gridCells[0] ? window.getComputedStyle(gridCells[0]) : null;
				const tStyle = trigger ? window.getComputedStyle(trigger) : null;
				return {
					presetCount: presets.length,
					gridCount: gridCells.length,
					presetRadius: pStyle ? pStyle.borderRadius : "",
					presetShadow: pStyle ? pStyle.boxShadow : "",
					gridRadius: gStyle ? gStyle.borderRadius : "",
					gridShadow: gStyle ? gStyle.boxShadow : "",
					triggerBg: tStyle ? tStyle.backgroundImage : ""
				};
			} finally {
				mockMenu.remove();
				window.__swissLastClickedProject = prevClicked;
			}
		})()`
		if domRes, dErr := inj.ExecuteScript(pages[0].WebSocketDebuggerURL, verifyExpr); dErr == nil {
			if pc, ok := domRes["presetCount"].(float64); !ok || int(pc) != 7 {
				t.Errorf("Expected 7 live preset swatches, got %v", domRes["presetCount"])
			}
			if gc, ok := domRes["gridCount"].(float64); !ok || int(gc) != 100 {
				t.Errorf("Expected 100 live grid cells, got %v", domRes["gridCount"])
			}
			if gr, _ := domRes["gridRadius"].(string); gr != "50%" {
				t.Errorf("Expected live .swiss-grid-cell borderRadius 50%%, got %q", gr)
			}
			if ps, _ := domRes["presetShadow"].(string); !strings.Contains(ps, "rgba(128, 128, 128, 0.25)") || !strings.Contains(ps, "inset") {
				t.Errorf("Expected live .swiss-preset-swatch inset boxShadow rgba(128, 128, 128, 0.25), got %q", ps)
			}
			if gs, _ := domRes["gridShadow"].(string); !strings.Contains(gs, "rgba(128, 128, 128, 0.25)") || !strings.Contains(gs, "inset") {
				t.Errorf("Expected live .swiss-grid-cell inset boxShadow rgba(128, 128, 128, 0.25), got %q", gs)
			}
			if tb, _ := domRes["triggerBg"].(string); !strings.Contains(tb, "conic-gradient") {
				t.Errorf("Expected live #swiss-custom-trigger conic-gradient backgroundImage, got %q", tb)
			}
		}
	}
}

func TestInspectLiveSidebar(t *testing.T) {
	inj := NewInjector(0)
	port, err := inj.FindDevToolsPort()
	if err != nil {
		t.Skip("Antigravity DevTools not running")
	}
	pages, err := inj.GetPageTargets(port)
	if err != nil || len(pages) == 0 {
		t.Skip("No page targets")
	}

	cfg := DefaultConfig()
	if _, err := inj.ApplyConfig(cfg); err != nil {
		t.Fatalf("Failed to apply config: %v", err)
	}

	expr := `(() => {
		if (typeof window.__swissUpdateTagsAndDraggables === "function") {
			window.__swissUpdateTagsAndDraggables();
		}
		const details = Array.from(document.querySelectorAll(".swiss-project-bottom-spacer")).map(sp => {
			const parent = sp.parentElement;
			const nextItem = parent ? parent.nextElementSibling : null;
			const pInner = parent ? (parent.querySelector('[data-testid="conversation-row-sidebar"]') || parent.firstElementChild) : null;
			const nextInner = nextItem ? (nextItem.querySelector('[data-project-card]') || nextItem.firstElementChild) : null;
			const line = sp.querySelector(".swiss-project-spacer-line");
			
			const pInnerRect = pInner ? pInner.getBoundingClientRect() : null;
			const nextInnerRect = nextInner ? nextInner.getBoundingClientRect() : null;
			const lineRect = line ? line.getBoundingClientRect() : null;

			return {
				parentIdx: parent ? parent.getAttribute("data-index") : null,
				pInnerBottom: pInnerRect ? pInnerRect.bottom : null,
				nextInnerTop: nextInnerRect ? nextInnerRect.top : null,
				visualGapBetweenInners: (pInnerRect && nextInnerRect) ? (nextInnerRect.top - pInnerRect.bottom) : null,
				lineTop: lineRect ? lineRect.top : null,
				lineBottom: lineRect ? lineRect.bottom : null,
				lineCenter: lineRect ? (lineRect.top + lineRect.bottom) / 2 : null,
				gapCenter: (pInnerRect && nextInnerRect) ? (pInnerRect.bottom + nextInnerRect.top) / 2 : null,
				offsetFromVisualCenter: (pInnerRect && nextInnerRect && lineRect) ? ((lineRect.top + lineRect.bottom) / 2 - (pInnerRect.bottom + nextInnerRect.top) / 2) : null
			};
		});
		return { details };
	})()`

	res, err := inj.ExecuteScript(pages[0].WebSocketDebuggerURL, expr)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}
	out, _ := json.MarshalIndent(res, "", "  ")
	t.Logf("Sidebar inspection:\n%s", string(out))

	detailsVal, ok := res["details"].([]interface{})
	if !ok || len(detailsVal) == 0 {
		t.Log("No spacers active in current live view")
		return
	}
	for i, d := range detailsVal {
		dm, ok := d.(map[string]interface{})
		if !ok {
			continue
		}
		if off, ok := dm["offsetFromVisualCenter"].(float64); ok {
			if off < -0.5 || off > 0.5 {
				t.Errorf("Spacer %d line is not centered in natural project gap: offset=%v", i, off)
			}
		}
	}
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
	if strings.Contains(css, ".swiss-convo-tabs-line") {
		t.Errorf("Expected .swiss-convo-tabs-line to be removed from CSS, got: %s", css)
	}
	if !strings.Contains(css, ".swiss-convo-tabs-pill") {
		t.Errorf("Expected .swiss-convo-tabs-pill in CSS")
	}
	if !strings.Contains(css, ".swiss-convo-tabs-triangle") {
		t.Errorf("Expected .swiss-convo-tabs-triangle in CSS")
	}
	if !strings.Contains(css, "[data-swiss-pruned=\"true\"]") {
		t.Errorf("Expected [data-swiss-pruned=\"true\"] in CSS")
	}
	if !strings.Contains(css, ".swiss-pruned-badge") {
		t.Errorf("Expected .swiss-pruned-badge in CSS")
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
	if !strings.Contains(script, "__swissManuallyExpandedProjects") {
		t.Errorf("Script missing __swissManuallyExpandedProjects")
	}
	if !strings.Contains(script, "data-swiss-can-expand") {
		t.Errorf("Script missing data-swiss-can-expand")
	}
	if !strings.Contains(script, "data-swiss-pruned") {
		t.Errorf("Script missing data-swiss-pruned")
	}
	if !strings.Contains(script, "__swissPrunedClickBound") {
		t.Errorf("Script missing __swissPrunedClickBound click interceptor")
	}
}

func TestScanPrunedConversationIDs(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "conversation_summaries.db")
	convsDir := filepath.Join(tmpDir, "conversations")
	if err := os.MkdirAll(convsDir, 0755); err != nil {
		t.Fatalf("failed to create convsDir: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE conversation_summaries (
		conversation_id TEXT PRIMARY KEY,
		title TEXT
	)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert 3 conversations:
	// conv-1: physical file exists
	// conv-2: physical file is missing
	// conv-3: physical file is missing
	_, err = db.Exec(`INSERT INTO conversation_summaries VALUES ('conv-1', 'Active Conv')`)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}
	_, err = db.Exec(`INSERT INTO conversation_summaries VALUES ('conv-2', 'Pruned Conv 1')`)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}
	_, err = db.Exec(`INSERT INTO conversation_summaries VALUES ('conv-3', 'Pruned Conv 2')`)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Create physical file only for conv-1
	if err := os.WriteFile(filepath.Join(convsDir, "conv-1.db"), []byte("sqlite file"), 0644); err != nil {
		t.Fatalf("failed to write conv-1.db: %v", err)
	}

	pruned := ScanPrunedConversationIDs(dbPath, convsDir)
	if len(pruned) != 2 {
		t.Fatalf("expected 2 pruned conversations, got %d: %v", len(pruned), pruned)
	}
	hasConv2 := false
	hasConv3 := false
	for _, id := range pruned {
		if id == "conv-2" {
			hasConv2 = true
		}
		if id == "conv-3" {
			hasConv3 = true
		}
	}
	if !hasConv2 || !hasConv3 {
		t.Errorf("expected pruned to contain conv-2 and conv-3, got %v", pruned)
	}

	// Test non-existent dbPath returns empty slice
	empty := ScanPrunedConversationIDs(filepath.Join(tmpDir, "missing.db"), convsDir)
	if len(empty) != 0 {
		t.Errorf("expected empty slice for missing db, got %v", empty)
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

func TestProjectColorsOfflinePersistenceAndLocalStorage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ProjectColors = map[string]string{
		"Alpha": "#6366f1",
		"Beta":  "#ec4899",
	}
	script := GenerateScript(cfg)

	// 1. Verify configuredColors in base script
	if !strings.Contains(script, `const configuredColors = {"Alpha":"#6366f1","Beta":"#ec4899"}`) &&
		!strings.Contains(script, `const configuredColors = {"Beta":"#ec4899","Alpha":"#6366f1"}`) {
		t.Errorf("GenerateScript missing configuredColors initialization")
	}

	// 2. Verify localStorage reading and fallback
	if !strings.Contains(script, `localStorage.getItem("antigravity_swiss_project_colors")`) {
		t.Errorf("GenerateScript missing localStorage reading for antigravity_swiss_project_colors")
	}

	// 3. Verify disk persistence function and target file
	if !strings.Contains(script, "persistProjectColorsToDisk") {
		t.Errorf("GenerateScript missing persistProjectColorsToDisk")
	}
	if !strings.Contains(script, "gui_improvements.json") {
		t.Errorf("GenerateScript missing gui_improvements.json persistence target")
	}

	// 4. Verify localStorage writing on color update
	if !strings.Contains(script, `localStorage.setItem("antigravity_swiss_project_colors"`) {
		t.Errorf("GenerateScript missing localStorage writing for antigravity_swiss_project_colors")
	}
	if !strings.Contains(script, "antigravity_swiss_deleted_colors") {
		t.Errorf("GenerateScript missing antigravity_swiss_deleted_colors tracking for offline deletion preservation")
	}

	// 5. Verify renderDynamicProjectStyles called on startup
	if !strings.Contains(script, "window.__swissRenderDynamicProjectStyles = renderDynamicProjectStyles;\n  renderDynamicProjectStyles();") {
		t.Errorf("GenerateScript missing immediate startup invocation of renderDynamicProjectStyles")
	}

	// 6. Verify normalizeNewConversationButton included and hooked into triggerSwissUpdate
	if !strings.Contains(script, "normalizeNewConversationButton") {
		t.Errorf("GenerateScript missing normalizeNewConversationButton")
	}
	if !strings.Contains(script, "window.normalizeNewConversationButton()") {
		t.Errorf("GenerateScript triggerSwissUpdate missing window.normalizeNewConversationButton() call")
	}

	// 7. Verify Node.js syntax check on entire generated bundle
	if nodePath, err := exec.LookPath("node"); err == nil {
		cmd := exec.Command(nodePath, "--check")
		cmd.Stdin = strings.NewReader(script)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("node syntax error in GenerateScript: %v\n%s", err, string(out))
		}
	}

	// 8. Verify precedence: configuredColors baseline assigned before localStorage overlay so offline changes take precedence
	cfgIdx := strings.Index(script, "Object.assign(window.__swissDynamicColors, configuredColors);")
	localIdx := strings.Index(script, `localStorage.getItem("antigravity_swiss_project_colors")`)
	if cfgIdx == -1 || localIdx == -1 || cfgIdx > localIdx {
		t.Errorf("expected configuredColors baseline to be assigned before localStorage cached colors are overlaid")
	}

	// 9. Verify offline persistence semantics: empty localStorage is seeded, offline changes override stale config
	if nodePath, err := exec.LookPath("node"); err == nil {
		simJS := `
		const store = {};
		const localStorage = {
			getItem: (k) => store[k] || null,
			setItem: (k, v) => { store[k] = String(v); }
		};
		const configuredColors = { Alpha: "#6366f1", Beta: "#ec4899" };
		
		// Case A: Fresh run with empty localStorage -> seeded
		let dColors = {};
		if (configuredColors && typeof configuredColors === "object") Object.assign(dColors, configuredColors);
		const cachedA = localStorage.getItem("antigravity_swiss_project_colors");
		if (cachedA) Object.assign(dColors, JSON.parse(cachedA));
		else localStorage.setItem("antigravity_swiss_project_colors", JSON.stringify(dColors));
		if (!store["antigravity_swiss_project_colors"] || !store["antigravity_swiss_project_colors"].includes("#6366f1")) {
			process.exit(1);
		}

		// Case B: Offline change in localStorage -> preserved over stale configuredColors
		store["antigravity_swiss_project_colors"] = JSON.stringify({ Alpha: "#059669" });
		dColors = {};
		if (configuredColors && typeof configuredColors === "object") Object.assign(dColors, configuredColors);
		const cachedB = localStorage.getItem("antigravity_swiss_project_colors");
		if (cachedB) Object.assign(dColors, JSON.parse(cachedB));
		if (dColors.Alpha !== "#059669") {
			process.exit(2);
		}

		// Case C: Offline reset/deletion in localStorage -> preserved over stale configuredColors (not resurrected)
		store["antigravity_swiss_project_colors"] = JSON.stringify({ Alpha: "#059669" });
		store["antigravity_swiss_deleted_colors"] = JSON.stringify(["Beta"]);
		dColors = {};
		if (configuredColors && typeof configuredColors === "object") Object.assign(dColors, configuredColors);
		const cachedC = localStorage.getItem("antigravity_swiss_project_colors");
		if (cachedC) Object.assign(dColors, JSON.parse(cachedC));
		const delCached = localStorage.getItem("antigravity_swiss_deleted_colors");
		if (delCached) {
			const delList = JSON.parse(delCached);
			if (Array.isArray(delList)) delList.forEach(p => delete dColors[p]);
		}
		if (dColors.Beta !== undefined) {
			process.exit(3);
		}
		`
		cmd := exec.Command(nodePath, "-e", simJS)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("offline persistence simulation failed in Node: %v\n%s", err, string(out))
		}
	}
}

func TestLiveVerificationR1R2R3(t *testing.T) {
	inj := NewInjector(0)
	port, err := inj.FindDevToolsPort()
	if err != nil {
		t.Skip("Antigravity DevTools not running, skipping live verification")
	}
	pages, err := inj.GetPageTargets(port)
	if err != nil || len(pages) == 0 {
		t.Skip("No page targets available")
	}

	wsURL := pages[0].WebSocketDebuggerURL

	// 1. Verify R1: Project card background and text visibility with [data-swiss-stage-active] set
	r1Expr := `(() => {
		const card = document.querySelector("[data-project-card]");
		if (!card) return { found: false };
		const prev = document.body.getAttribute("data-swiss-stage-active");
		document.body.setAttribute("data-swiss-stage-active", "swiss-knife");
		const cs = window.getComputedStyle(card);
		const res = {
			found: true,
			bg: cs.backgroundColor,
			color: cs.color,
			isTransparent: cs.backgroundColor === "rgba(0, 0, 0, 0)" || cs.backgroundColor === "transparent"
		};
		if (prev) document.body.setAttribute("data-swiss-stage-active", prev);
		else document.body.removeAttribute("data-swiss-stage-active");
		return res;
	})()`
	res1, err := inj.ExecuteScript(wsURL, r1Expr)
	if err != nil {
		t.Fatalf("R1 live execution error: %v", err)
	}
	t.Logf("R1 Live Result: %+v", res1)
	if found, _ := res1["found"].(bool); found {
		if isTrans, _ := res1["isTransparent"].(bool); isTrans {
			t.Errorf("R1 Failure: project card background is transparent under data-swiss-stage-active!")
		}
	}

	// 2. Verify R2: New conversation button normalization
	r2Expr := `(() => {
		const btn = document.querySelector('[data-testid="new-conversation-button"]');
		if (!btn) return { found: false };
		if (typeof window.normalizeNewConversationButton === "function") {
			window.normalizeNewConversationButton();
		}
		const cs = window.getComputedStyle(btn);
		return {
			found: true,
			dataSelected: btn.getAttribute("data-selected"),
			bg: cs.backgroundColor,
			borderTopWidth: cs.borderTopWidth
		};
	})()`
	res2, err := inj.ExecuteScript(wsURL, r2Expr)
	if err != nil {
		t.Fatalf("R2 live execution error: %v", err)
	}
	t.Logf("R2 Live Result: %+v", res2)

	// 3. Verify R3: Offline localStorage and dynamic styles
	r3Expr := `(() => {
		window.__swissRenderDynamicProjectStyles("LiveVerificationProject", "#10b981");
		const cached = localStorage.getItem("antigravity_swiss_project_colors");
		const dynStyle = document.getElementById("antigravity-swiss-dynamic-colors")?.textContent || "";
		window.__swissRenderDynamicProjectStyles("LiveVerificationProject", null);
		const cleaned = localStorage.getItem("antigravity_swiss_project_colors");
		return {
			savedToLocal: cached ? cached.includes("LiveVerificationProject") : false,
			styledInDOM: dynStyle.includes("LiveVerificationProject"),
			cleanedUp: cleaned ? !cleaned.includes("LiveVerificationProject") : true
		};
	})()`
	res3, err := inj.ExecuteScript(wsURL, r3Expr)
	if err != nil {
		t.Fatalf("R3 live execution error: %v", err)
	}
	t.Logf("R3 Live Result: %+v", res3)
	if saved, _ := res3["savedToLocal"].(bool); !saved {
		t.Errorf("R3 Failure: dynamic color not saved to localStorage")
	}
	if styled, _ := res3["styledInDOM"].(bool); !styled {
		t.Errorf("R3 Failure: dynamic color not styled in DOM")
	}
	if cleaned, _ := res3["cleanedUp"].(bool); !cleaned {
		t.Errorf("R3 Failure: dynamic color not cleaned up")
	}
}


