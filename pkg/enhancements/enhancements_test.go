package enhancements

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Errorf("expected Enabled to be true by default")
	}
	if !cfg.PromptJumpBar.Enabled {
		t.Errorf("expected PromptJumpBar.Enabled to be true by default")
	}
	if !cfg.PromptJumpBar.ShowTooltip {
		t.Errorf("expected ShowTooltip to be true")
	}
	if !cfg.PromptJumpBar.FocusPulse {
		t.Errorf("expected FocusPulse to be true")
	}
	if !cfg.PromptJumpBar.SyncScroll {
		t.Errorf("expected SyncScroll to be true")
	}
	if cfg.PromptJumpBar.Position != "gutter" {
		t.Errorf("expected Position to be gutter, got %s", cfg.PromptJumpBar.Position)
	}
	if cfg.PromptJumpBar.DashWidth != 14 {
		t.Errorf("expected DashWidth to be 14, got %d", cfg.PromptJumpBar.DashWidth)
	}
	if cfg.PromptJumpBar.DashThickness != 2.5 {
		t.Errorf("expected DashThickness to be 2.5, got %v", cfg.PromptJumpBar.DashThickness)
	}
	if cfg.PromptJumpBar.InactiveThickness != 1.5 {
		t.Errorf("expected InactiveThickness to be 1.5, got %v", cfg.PromptJumpBar.InactiveThickness)
	}
	if cfg.PromptJumpBar.ColorMode != "default" {
		t.Errorf("expected ColorMode to be default, got %s", cfg.PromptJumpBar.ColorMode)
	}
	if cfg.ToolDensityMode != "muted" {
		t.Errorf("expected ToolDensityMode to be muted, got %s", cfg.ToolDensityMode)
	}
	if !cfg.BreakerLineEnabled {
		t.Errorf("expected BreakerLineEnabled to be true by default")
	}
	if !cfg.LeftPanelExtensionsEnabled {
		t.Errorf("expected LeftPanelExtensionsEnabled to be true by default")
	}
	if cfg.LeftPanelExtensionsMode != "individual" {
		t.Errorf("expected LeftPanelExtensionsMode to be 'individual' by default, got %s", cfg.LeftPanelExtensionsMode)
	}
	if cfg.DefaultNewProject != "auto" {
		t.Errorf("expected DefaultNewProject to be 'auto', got %s", cfg.DefaultNewProject)
	}
	if !cfg.OverviewPanel.ReplaceSeeAllTriangle {
		t.Errorf("expected OverviewPanel.ReplaceSeeAllTriangle to be true by default")
	}
	if len(cfg.Extensions) != len(ExtensionIDs) {
		t.Fatalf("expected Extensions map to cover %d ids, got %d", len(ExtensionIDs), len(cfg.Extensions))
	}
	for _, id := range ExtensionIDs {
		vis, ok := cfg.Extensions[id]
		if !ok {
			t.Errorf("expected Extensions map to contain %q", id)
			continue
		}
		if !vis.AuxPanel || !vis.MainPage {
			t.Errorf("expected Extensions[%q] to default to {true, true}, got %+v", id, vis)
		}
	}
}

func TestStore_Operations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "enhancements-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "enhancements.json")
	store, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := store.GetConfig()
	if !cfg.PromptJumpBar.Enabled {
		t.Errorf("expected PromptJumpBar to be enabled")
	}

	// Toggle prompt jump bar off
	if err := store.TogglePromptJumpBar(false); err != nil {
		t.Fatalf("failed to toggle prompt jump bar: %v", err)
	}

	cfgAfter := store.GetConfig()
	if cfgAfter.PromptJumpBar.Enabled {
		t.Errorf("expected PromptJumpBar to be disabled after toggle")
	}

	// Set color mode and custom color
	if err := store.SetPromptJumpBarColor("custom", "#7c3aed"); err != nil {
		t.Fatalf("failed to set color mode: %v", err)
	}

	// Set tool density and breaker line
	if err := store.SetToolDensityMode("hidden"); err != nil {
		t.Fatalf("failed to set tool density: %v", err)
	}
	if err := store.SetBreakerLine(false); err != nil {
		t.Fatalf("failed to set breaker line: %v", err)
	}
	if err := store.SetDefaultNewProject("Antigravity Swiss Knife"); err != nil {
		t.Fatalf("failed to set default new project: %v", err)
	}

	// Reload in another store instance to verify persistence
	store2, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to load store2: %v", err)
	}

	loaded := store2.GetConfig()
	if loaded.PromptJumpBar.ColorMode != "custom" || loaded.PromptJumpBar.CustomColor != "#7c3aed" {
		t.Errorf("expected custom color to persist, got mode=%s color=%s", loaded.PromptJumpBar.ColorMode, loaded.PromptJumpBar.CustomColor)
	}
	if loaded.ToolDensityMode != "hidden" {
		t.Errorf("expected ToolDensityMode hidden to persist, got %s", loaded.ToolDensityMode)
	}
	if loaded.BreakerLineEnabled != false {
		t.Errorf("expected BreakerLineEnabled false to persist")
	}
	if loaded.DefaultNewProject != "Antigravity Swiss Knife" {
		t.Errorf("expected DefaultNewProject to persist, got %s", loaded.DefaultNewProject)
	}
}

func TestGenerateEnhancementsScript(t *testing.T) {
	cfg := DefaultConfig()
	script := GenerateEnhancementsScript(cfg)

	if !strings.Contains(script, "swiss-prompt-jump-bar") {
		t.Errorf("expected script to contain swiss-prompt-jump-bar element ID")
	}
	if !strings.Contains(script, "user-input-step") {
		t.Errorf("expected script to query user-input-step testid")
	}
	if !strings.Contains(script, "swiss-prompt-tooltip") {
		t.Errorf("expected script to manage swiss-prompt-tooltip")
	}
	if !strings.Contains(script, "scrollIntoView") {
		t.Errorf("expected script to call scrollIntoView")
	}
	if !strings.Contains(script, "getLowestPromptOnScreen") {
		t.Errorf("expected script to implement getLowestPromptOnScreen")
	}
	if !strings.Contains(script, "getActiveAndHoverColor") {
		t.Errorf("expected script to implement getActiveAndHoverColor")
	}
	if !strings.Contains(script, "worked-for-collapsible") {
		t.Errorf("expected script to target worked-for-collapsible")
	}
	if !strings.Contains(script, "thinking-collapsible-trigger") {
		t.Errorf("expected script to target thinking-collapsible-trigger")
	}
	if !strings.Contains(script, "applyDefaultProjectHandler") {
		t.Errorf("expected script to implement applyDefaultProjectHandler")
	}
	if !strings.Contains(script, "new-conversation-button") {
		t.Errorf("expected script to intercept new-conversation-button")
	}
	if !strings.Contains(script, "swiss-overview-tabs-divider") {
		t.Errorf("expected script to contain swiss-overview-tabs-divider")
	}
	if !strings.Contains(script, "swiss-overview-bottom-spacer") {
		t.Errorf("expected script to contain swiss-overview-bottom-spacer cleanup")
	}
}

func TestOverviewPanel_Divider(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "enhancements-overview-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "enhancements.json")
	store, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := store.GetConfig()
	if !cfg.OverviewPanel.ReplaceSeeAllTriangle {
		t.Errorf("expected ReplaceSeeAllTriangle to default to true")
	}

	// Update and verify persistence
	cfg.OverviewPanel.ReplaceSeeAllTriangle = false
	if err := store.UpdateConfig(cfg); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	store2, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to load store2: %v", err)
	}
	loaded := store2.GetConfig()
	if loaded.OverviewPanel.ReplaceSeeAllTriangle {
		t.Errorf("expected ReplaceSeeAllTriangle false to persist")
	}
}

func TestOverviewPanel_AuxTabsFormat(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.OverviewPanel.AuxTabsFormat != "icon" {
		t.Errorf("expected AuxTabsFormat to be 'icon' by default, got %s", cfg.OverviewPanel.AuxTabsFormat)
	}

	tmpDir, err := os.MkdirTemp("", "enhancements-aux-tabs-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(filepath.Join(tmpDir, "enhancements.json"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	if err := store.SetAuxTabsFormat("icon_and_name"); err != nil {
		t.Fatalf("failed to set aux tabs format: %v", err)
	}

	if store.GetConfig().OverviewPanel.AuxTabsFormat != "icon_and_name" {
		t.Errorf("expected AuxTabsFormat to be 'icon_and_name', got %s", store.GetConfig().OverviewPanel.AuxTabsFormat)
	}

	// Invalid fallback to "icon"
	if err := store.SetAuxTabsFormat("something_else"); err != nil {
		t.Fatalf("failed to set aux tabs format: %v", err)
	}
	if store.GetConfig().OverviewPanel.AuxTabsFormat != "icon" {
		t.Errorf("expected AuxTabsFormat fallback to 'icon', got %s", store.GetConfig().OverviewPanel.AuxTabsFormat)
	}
}

func TestGenerateEnhancementsScript_Syntax(t *testing.T) {
	js := GenerateEnhancementsScript(nil)
	if js == "" {
		t.Fatalf("expected non-empty JS script")
	}

	if _, err := exec.LookPath("node"); err == nil {
		cmd := exec.Command("node", "--check")
		cmd.Stdin = strings.NewReader(js)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("node syntax error in GenerateEnhancementsScript: %v\n%s", err, string(out))
		}
	}
}

func TestGenerateEnhancementsScript_TerminalScopePreservesFactoryStyle(t *testing.T) {
	cfg := DefaultConfig()
	script := GenerateEnhancementsScript(cfg)

	// 1. Ensure "Terminals" is NOT treated as an overview panel section title
	if strings.Contains(script, "\"Terminals\"") {
		t.Errorf("expected script NOT to include 'Terminals' in overview titles so terminal scope selector retains factory style")
	}

	// 2. Ensure script explicitly protects terminal scope selector from custom zone styling
	if !strings.Contains(script, "terminal scope selector") {
		t.Errorf("expected script to explicitly reference and protect terminal scope selector")
	}
}

func TestOverviewPanel_DefaultDivisionStyle_IsDividerLine(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.OverviewPanel.DivisionStyle != "divider_line" {
		t.Errorf("expected default DivisionStyle to be 'divider_line', got %s", cfg.OverviewPanel.DivisionStyle)
	}

	script := GenerateEnhancementsScript(cfg)
	if !strings.Contains(script, "swiss-overview-divider") {
		t.Errorf("expected script to contain swiss-overview-divider")
	}
	if !strings.Contains(script, "swiss-overview-zone") {
		t.Errorf("expected script to contain cleanup logic for swiss-overview-zone")
	}
}

func TestGenerateEnhancementsScript_ContrastSafeguard(t *testing.T) {
	cfg := DefaultConfig()
	script := GenerateEnhancementsScript(cfg)

	if !strings.Contains(script, `if (dark && lum < 0.15)`) || !strings.Contains(script, `return "#e2e8f0";`) {
		t.Errorf("expected dark-mode luminance safeguard (lum < 0.15 -> #e2e8f0) in getActiveAndHoverColor")
	}
	if !strings.Contains(script, `if (!dark && lum > 0.85)`) || !strings.Contains(script, `return "#334155";`) {
		t.Errorf("expected light-mode luminance safeguard (lum > 0.85 -> #334155) in getActiveAndHoverColor")
	}
}

func TestOverviewPanel_ShowMoreButton_TriangleOnlyNoHorizontalLine(t *testing.T) {
	cfg := DefaultConfig()
	script := GenerateEnhancementsScript(cfg)

	if !strings.Contains(script, "swiss-overview-tabs-triangle") {
		t.Errorf("expected script to contain swiss-overview-tabs-triangle")
	}
	if !strings.Contains(script, "swiss-overview-tabs-divider") {
		t.Errorf("expected script to contain swiss-overview-tabs-divider")
	}
	// The show more/less button in the overview panel must NOT render a horizontal line
	if strings.Contains(script, "<div class=\"swiss-overview-tabs-line\"></div>") {
		t.Errorf("expected overview show more/less button HTML NOT to contain swiss-overview-tabs-line horizontal line element")
	}
	if strings.Contains(script, ".swiss-overview-tabs-line {") {
		t.Errorf("expected overview CSS NOT to define .swiss-overview-tabs-line rule")
	}

	// Verify both "See all/less" and "Show more/less" text patterns are matched
	if !strings.Contains(script, `isMore = /^(see|show)\s+(all|more)/i`) {
		t.Errorf("expected script to match both 'see' and 'show' expand patterns")
	}
	if !strings.Contains(script, `isLess = /^(see|show)\s+(less|fewer)/i`) {
		t.Errorf("expected script to match both 'less' and 'fewer' contract patterns")
	}
	if !strings.Contains(script, `lower.includes("show more")`) {
		t.Errorf("expected script to handle 'show more' in triangle direction determination")
	}
}

func TestOverviewPanel_DividerLine_PreservesFactorySectionDistance(t *testing.T) {
	cfg := DefaultConfig()

	script := GenerateEnhancementsScript(cfg)
	// Check that divider offsets compensate for flex gap to prevent blank areas and match factory spacing
	if !strings.Contains(script, "margin-top: calc(-12px") {
		t.Errorf("expected script CSS to compensate for flex gap with negative margin calculation")
	}
	if !strings.Contains(script, "netMargin") {
		t.Errorf("expected script JS to compute netMargin to match factory section gap")
	}
	if !strings.Contains(script, "min-height: 0px") || !strings.Contains(script, "overflow: hidden") {
		t.Errorf("expected divider CSS to have zero-height resets to avoid unwanted height inflation")
	}
	// Check horizontal insets so the divider line does not touch the edges
	if !strings.Contains(script, "margin-left: 6px !important;") || !strings.Contains(script, "margin-right: 6px !important;") {
		t.Errorf("expected divider CSS to include 6px horizontal margins")
	}
	if !strings.Contains(script, "width: calc(100% - 12px) !important;") {
		t.Errorf("expected divider CSS to shorten width with 12px inset calculation")
	}
	if !strings.Contains(script, `divider.style.setProperty("margin-left", "6px", "important")`) {
		t.Errorf("expected divider JS to set 6px horizontal margin-left")
	}
	if !strings.Contains(script, `divider.style.setProperty("width", "calc(100% - 12px)", "important")`) {
		t.Errorf("expected divider JS to set shortened width with 12px inset")
	}
	// Check that orphaned dividers are pruned
	if !strings.Contains(script, "sectionContainers.indexOf(next) === 0") {
		t.Errorf("expected script to prune orphaned dividers before the first section")
	}
}

func TestOverviewPanel_ShrunkSectionGapsAndCompactTriangle(t *testing.T) {
	cfg := DefaultConfig()
	script := GenerateEnhancementsScript(cfg)

	// Verify shrunken compact divider gap calculation (5px half gap -> 10px total section distance)
	if !strings.Contains(script, "desiredHalfGap = 5") {
		t.Errorf("expected script JS to compute desiredHalfGap = 5 for compact section spacing")
	}
	if !strings.Contains(script, "margin-top: calc(-12px - 7px)") {
		t.Errorf("expected script CSS to specify tightened negative margin calculation (-19px)")
	}

	// Verify tightened show-more button (8px height, 14x6px pill, 8px font-size)
	if !strings.Contains(script, `[data-swiss-overview-divider="true"] {`) {
		t.Errorf("expected script CSS to style data-swiss-overview-divider")
	}
	if strings.Contains(script, `div:has(> [data-swiss-overview-divider="true"])`) {
		t.Errorf("script CSS must NOT target div:has(> [data-swiss-overview-divider=\"true\"]) because that crushes the parent overview section container to 8px height")
	}
	if !strings.Contains(script, "height: 8px !important;") {
		t.Errorf("expected script CSS to tighten show-more button to 8px height")
	}
	if !strings.Contains(script, "width: 14px !important;") || !strings.Contains(script, "height: 6px !important;") {
		t.Errorf("expected script CSS to tighten show-more pill dimensions to 14x6px")
	}

	// Verify border_zone gap zeroing on parent container
	if !strings.Contains(script, `parent.style.setProperty("gap", "0px", "important")`) {
		t.Errorf("expected script JS to zero parent flex gap in border_zone mode")
	}

	// Verify divider lines rendered via ::before with 6px horizontal margins and -3px bottom margin compensation for section gap-2 (8px)
	if !strings.Contains(script, `margin-left: 6px !important;`) || !strings.Contains(script, `margin-right: 6px !important;`) {
		t.Errorf("expected script CSS to inset divider lines with 6px margins")
	}
	if !strings.Contains(script, `gap: 5px !important;`) {
		t.Errorf("expected .swiss-overview-parent to set gap: 5px !important for 5px top half-gap")
	}
	if !strings.Contains(script, `row-gap: 8px !important;`) {
		t.Errorf("expected section containers to lock row-gap: 8px !important so -3px bottom margin compensation is invariant")
	}
	if !strings.Contains(script, `margin-bottom: -3px !important;`) {
		t.Errorf("expected ::before divider to set margin-bottom: -3px !important so 8px flex gap-2 - 3px = 5px bottom half-gap")
	}
	if !strings.Contains(script, `margin: -7px auto 0 auto !important;`) {
		t.Errorf("expected [data-swiss-overview-divider=\"true\"] to pull top margin by -7px to tighten section gap-2 above triangle")
	}

	// Verify live re-injection disconnects and replaces stale MutationObserver rather than skipping
	if strings.Contains(script, "if (!window.__swissEnhancementsObserver)") {
		t.Errorf("script must not skip observer replacement when window.__swissEnhancementsObserver is already set")
	}
	if !strings.Contains(script, `typeof window.__swissEnhancementsObserver.disconnect === "function"`) {
		t.Errorf("expected script to disconnect existing window.__swissEnhancementsObserver on re-injection")
	}

	// Verify chat markdown blocks are excluded from overview section header detection
	if !strings.Contains(script, `el.closest('.md-divider-spacing')`) || !strings.Contains(script, `el.closest('[data-testid="autoscroll-viewport"]')`) {
		t.Errorf("expected script to exclude chat markdown and autoscroll viewport from overview section headers")
	}

	// Verify MutationObserver does not ignore React mutations inside overview section/divider containers
	if !strings.Contains(script, "const isSwissLeafElement = (n) =>") {
		t.Errorf("expected script observer to filter only dedicated Swiss leaf elements")
	}
}

func TestLeftPanelExtensions_StoreAndScript(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "enh-left-panel-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "enhancements.json")
	store, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := store.GetConfig()
	if !cfg.LeftPanelExtensionsEnabled {
		t.Errorf("expected LeftPanelExtensionsEnabled to be true by default")
	}
	if cfg.LeftPanelExtensionsMode != "individual" {
		t.Errorf("expected LeftPanelExtensionsMode to be 'individual' by default, got %s", cfg.LeftPanelExtensionsMode)
	}

	if err := store.ToggleLeftPanelExtensions(false); err != nil {
		t.Fatalf("failed to toggle left panel extensions: %v", err)
	}
	if store.GetConfig().LeftPanelExtensionsEnabled {
		t.Errorf("expected LeftPanelExtensionsEnabled to be false after toggle")
	}

	if err := store.SetLeftPanelExtensionsMode("individual"); err != nil {
		t.Fatalf("failed to set left panel extensions mode: %v", err)
	}
	if store.GetConfig().LeftPanelExtensionsMode != "individual" {
		t.Errorf("expected LeftPanelExtensionsMode to be 'individual'")
	}

	if !cfg.MainSectionExtensionsEnabled {
		t.Errorf("expected MainSectionExtensionsEnabled to be true by default")
	}
	if err := store.ToggleMainSectionExtensions(false); err != nil {
		t.Fatalf("failed to toggle main section extensions: %v", err)
	}
	if store.GetConfig().MainSectionExtensionsEnabled {
		t.Errorf("expected MainSectionExtensionsEnabled to be false after toggle")
	}

	curCfg := store.GetConfig()
	script := GenerateEnhancementsScript(&curCfg)
	if !strings.Contains(script, "antigravity_swiss_left_panel_enabled") {
		t.Errorf("expected script to sync antigravity_swiss_left_panel_enabled to localStorage")
	}
	if !strings.Contains(script, "antigravity_swiss_left_panel_mode") {
		t.Errorf("expected script to sync antigravity_swiss_left_panel_mode to localStorage")
	}
	if !strings.Contains(script, "antigravity_swiss_main_section_enabled") {
		t.Errorf("expected script to sync antigravity_swiss_main_section_enabled to localStorage")
	}
	if !strings.Contains(script, "swiss-left-nav-config-updated") {
		t.Errorf("expected script to dispatch swiss-left-nav-config-updated event")
	}
}

func TestExtensionsVisibility_LegacyMigration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "enh-ext-migration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "enhancements.json")

	// Legacy config without an extensions map; both global toggles disabled.
	legacy := `{
  "version": "1.0.0",
  "enabled": true,
  "left_panel_extensions_enabled": false,
  "left_panel_extensions_mode": "single",
  "main_section_extensions_enabled": false
}`
	if err := os.WriteFile(configPath, []byte(legacy), 0644); err != nil {
		t.Fatalf("failed to write legacy config: %v", err)
	}

	store, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := store.GetConfig()
	if len(cfg.Extensions) != len(ExtensionIDs) {
		t.Fatalf("expected migrated Extensions map with %d entries, got %d", len(ExtensionIDs), len(cfg.Extensions))
	}
	for _, id := range ExtensionIDs {
		vis, ok := cfg.Extensions[id]
		if !ok {
			t.Errorf("expected Extensions map to contain %q", id)
			continue
		}
		if vis.AuxPanel {
			t.Errorf("expected Extensions[%q].AuxPanel=false (left_panel_extensions_enabled was false)", id)
		}
		if vis.MainPage {
			t.Errorf("expected Extensions[%q].MainPage=false (main_section_extensions_enabled was false)", id)
		}
	}
	if cfg.LeftPanelExtensionsMode != "individual" {
		t.Errorf("expected LeftPanelExtensionsMode forced to 'individual' after migration, got %s", cfg.LeftPanelExtensionsMode)
	}

	// The normalized config must be persisted once so the map is authoritative.
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to re-read persisted config: %v", err)
	}
	if !strings.Contains(string(raw), `"extensions"`) {
		t.Errorf("expected persisted enhancements.json to contain the extensions map")
	}

	// Reload: the persisted map must not be re-derived from legacy fields.
	store2, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to reload store: %v", err)
	}
	if len(store2.GetConfig().Extensions) != len(ExtensionIDs) {
		t.Errorf("expected reloaded store to keep the persisted extensions map")
	}
}

func TestExtensionsVisibility_LegacyMigration_Enabled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "enh-ext-migration-on-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "enhancements.json")

	// Legacy config with global toggles enabled and no extensions map.
	legacy := `{
  "version": "1.0.0",
  "enabled": true,
  "left_panel_extensions_enabled": true,
  "left_panel_extensions_mode": "single",
  "main_section_extensions_enabled": true
}`
	if err := os.WriteFile(configPath, []byte(legacy), 0644); err != nil {
		t.Fatalf("failed to write legacy config: %v", err)
	}

	store, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	cfg := store.GetConfig()
	for _, id := range ExtensionIDs {
		vis, ok := cfg.Extensions[id]
		if !ok {
			t.Errorf("expected Extensions map to contain %q", id)
			continue
		}
		if !vis.AuxPanel || !vis.MainPage {
			t.Errorf("expected Extensions[%q] to be {true, true}, got %+v", id, vis)
		}
	}
	if cfg.LeftPanelExtensionsMode != "individual" {
		t.Errorf("expected LeftPanelExtensionsMode forced to 'individual', got %s", cfg.LeftPanelExtensionsMode)
	}
}

func TestExtensionsVisibility_UpdatePreservesMap(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "enh-ext-update-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "enhancements.json")
	store, err := NewStore(configPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Toggle one extension off through the map and persist.
	cfg := store.GetConfig()
	cfg.Extensions["github"] = ExtensionVisibility{AuxPanel: true, MainPage: false}
	if err := store.UpdateConfig(cfg); err != nil {
		t.Fatalf("failed to update config: %v", err)
	}

	// A subsequent UpdateConfig from a client without the extensions map must
	// carry the stored map over instead of re-deriving it from legacy toggles.
	stripped := store.GetConfig()
	stripped.Extensions = nil
	if err := store.UpdateConfig(stripped); err != nil {
		t.Fatalf("failed to update config without extensions map: %v", err)
	}
	got := store.GetConfig().Extensions["github"]
	if !got.AuxPanel || got.MainPage {
		t.Errorf("expected stored extensions map to survive a legacy update, got %+v", got)
	}
}

func TestGenerateEnhancementsScript_ExtensionsMap(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Extensions["browser"] = ExtensionVisibility{AuxPanel: false, MainPage: true}
	script := GenerateEnhancementsScript(cfg)

	if !strings.Contains(script, `"extensions"`) {
		t.Errorf("expected injected __SWISS_ENH_CONFIG__ payload to carry the extensions map")
	}
	if !strings.Contains(script, `"aux_panel"`) || !strings.Contains(script, `"main_page"`) {
		t.Errorf("expected injected config to serialize aux_panel/main_page keys")
	}
}

