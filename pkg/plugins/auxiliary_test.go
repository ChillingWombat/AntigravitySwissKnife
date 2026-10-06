package plugins

import (
	"strings"
	"testing"
)

func TestGenerateAuxiliaryPluginsCSS(t *testing.T) {
	css := GenerateAuxiliaryPluginsCSS()
	if css == "" {
		t.Fatalf("expected non-empty CSS")
	}

	requiredSelectors := []string{
		".swiss-aux-tab-btn",
		".swiss-aux-tabs-divider",
		".swiss-aux-btn-group",
		"#swiss-aux-container",
		".swiss-browser-view",
		".swiss-browser-toolbar",
		".swiss-browser-nav-row",
		".swiss-browser-port-bar",
		".swiss-port-chip",
		".swiss-port-label",
		".swiss-browser-viewport-wrap",
		".swiss-device-stage",
		".swiss-device-frame",
		".swiss-device-screen",
		".frame-iphone-16-pro",
		".frame-pixel-9",
		".frame-ipad",
		".frame-responsive",
		".swiss-device-notch",
		".dynamic-island",
		".punch-hole",
		".ipad-camera",
		".swiss-device-home-bar",
		".touch-emulation-active",
		".swiss-browser-canvas-overlay",
		".swiss-files-view",
		".swiss-editor-container",
		".swiss-memos-view",
		".swiss-telemetry-badge",
		".swiss-aux-tab-btn.icon-only",
		".swiss-aux-tab-svg",
		".swiss-aux-tab-label",
	}

	for _, sel := range requiredSelectors {
		if !strings.Contains(css, sel) {
			t.Errorf("expected CSS to contain selector %q", sel)
		}
	}
}

func TestGenerateAuxiliaryPluginsScript(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	if js == "" {
		t.Fatalf("expected non-empty JS script")
	}

	requiredIdentifiers := []string{
		"setupAuxiliaryTabs",
		"switchAuxTab",
		"renderBrowserView",
		"renderFilesView",
		"renderMemosView",
		"setupInChatTelemetry",
		"swiss-aux-container",
		"swiss-telemetry-badge",
		"insertTextToChatInput",
		`data-tab-id="swiss-browser"`,
		`data-tab-id="swiss-files"`,
		`data-tab-id="swiss-memos"`,
		"swiss-aux-tab-svg",
		"getAuxTabFormat",
		"icon-only",
		"swiss-aux-tab-format-updated",
		`partition="persist:swiss-browser"`,
		`allowRunningInsecureContent=yes`,
		`webSecurity=no`,
		"swiss-port-chip",
		"iphone-16-pro",
		"pixel-9",
		"ipad",
		"swiss-b-touch",
		"TouchEvent",
		"applyDeviceScale",
	}

	for _, id := range requiredIdentifiers {
		if !strings.Contains(js, id) {
			t.Errorf("expected script to contain identifier %q", id)
		}
	}
}

func TestAuxiliaryTabInjectionSelectors(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	if js == "" {
		t.Fatalf("expected non-empty JS script")
	}

	requiredSelectors := []string{
		`.shrink-0.flex.items-center[class*="gap-0.5"].border-b`,
		".flex-grow.overflow-hidden",
		"#swiss-aux-container",
		".swiss-aux-btn-group",
		".swiss-aux-tabs-divider",
	}

	for _, sel := range requiredSelectors {
		if !strings.Contains(js, sel) {
			t.Errorf("expected script to contain selector %q", sel)
		}
	}
}

func TestAuxiliaryTabAttributes(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	if js == "" {
		t.Fatalf("expected non-empty JS script")
	}

	requiredAttributes := []string{
		`data-tab-id="swiss-browser"`,
		`data-tab-id="swiss-files"`,
		`data-tab-id="swiss-memos"`,
		`btn.dataset.swissTab = t.id`,
	}

	for _, attr := range requiredAttributes {
		if !strings.Contains(js, attr) {
			t.Errorf("expected script to configure tab attribute %q", attr)
		}
	}
}

func TestAuxiliaryTwoWayStateSyncLogic(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	if js == "" {
		t.Fatalf("expected non-empty JS script")
	}

	requiredLogicSnippets := []string{
		`localStorage.setItem("antigravity_active_aux_tab"`,
		`localStorage.getItem("antigravity_active_aux_tab")`,
		`swissContainer.style.display = "none"`,
		`swissContainer.style.display = "flex"`,
		`child.style.display = "none"`,
		`child.style.display = ""`,
		`switchAuxTab(null)`,
		`swissContainer.dataset.renderedTab === cleanId`,
		`container.dataset.renderedTab = tabId`,
		`delete swissContainer.dataset.renderedTab`,
	}

	for _, snippet := range requiredLogicSnippets {
		if !strings.Contains(js, snippet) {
			t.Errorf("expected script to contain state sync logic snippet %q", snippet)
		}
	}
}

func TestAuxiliaryDOMInspectorClassRegex(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	if js == "" {
		t.Fatalf("expected non-empty JS script")
	}

	expectedRegex := `el.className.trim().split(/\s+/)`
	if !strings.Contains(js, expectedRegex) {
		t.Errorf("expected script to contain whitespace split regex %q", expectedRegex)
	}

	forbiddenRegex := `split(/\\s+/)`
	if strings.Contains(js, forbiddenRegex) {
		t.Errorf("script must not contain double-escaped literal regex %q", forbiddenRegex)
	}
}

func TestAuxiliarySynchronousInitialInvocation(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	if js == "" {
		t.Fatalf("expected non-empty JS script")
	}

	if !strings.Contains(js, "setupAuxiliaryTabs();\n    setupInChatTelemetry();\n\n    // Periodic check") &&
		!strings.Contains(js, "setupAuxiliaryTabs();\n    setupInChatTelemetry();") {
		t.Errorf("expected script to perform synchronous initial invocation of setupAuxiliaryTabs and setupInChatTelemetry")
	}
}

func TestAuxiliaryWindowResizeBound(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	if js == "" {
		t.Fatalf("expected non-empty JS script")
	}

	if !strings.Contains(js, "!window.__swissResizeBound") || !strings.Contains(js, "window.__swissResizeBound = true") {
		t.Errorf("expected script to guard window resize listener with __swissResizeBound")
	}
}

func TestBrowserViewDeviceFramesAndPorts(t *testing.T) {
	css := GenerateAuxiliaryPluginsCSS()
	js := GenerateAuxiliaryPluginsScript()

	checks := []struct {
		name    string
		source  string
		pattern string
	}{
		{"iPhone 16 Pro width", js, "402"},
		{"iPhone 16 Pro height", js, "874"},
		{"Pixel 9 width", js, "412"},
		{"Pixel 9 height", js, "924"},
		{"iPad width", js, "820"},
		{"iPad height", js, "1180"},
		{"Webview partition", js, "persist:swiss-browser"},
		{"CORS relaxation", js, "webSecurity=no"},
		{"Insecure content relaxation", js, "allowRunningInsecureContent=yes"},
		{"Port 5173 chip", js, `data-port="5173"`},
		{"Port 3000 chip", js, `data-port="3000"`},
		{"Port 8080 chip", js, `data-port="8080"`},
		{"Port 8765 chip", js, `data-port="8765"`},
		{"Port 4173 chip", js, `data-port="4173"`},
		{"Touch emulation handler", js, "TouchEvent"},
		{"Dynamic Island style", css, ".dynamic-island"},
		{"Punch-hole camera style", css, ".punch-hole"},
		{"iPad camera style", css, ".ipad-camera"},
		{"Touch cursor style", css, ".touch-emulation-active"},
	}

	for _, tc := range checks {
		if !strings.Contains(tc.source, tc.pattern) {
			t.Errorf("test %s failed: expected source to contain %q", tc.name, tc.pattern)
		}
	}
}

func TestGenerateAuxiliaryPluginsScript_CanvasAnnotation(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	if js == "" {
		t.Fatalf("expected non-empty JS script")
	}

	requiredAnnotationTokens := []string{
		"quadraticCurveTo", // Bézier midpoint curve smoothing
		"#ea4335",          // Red pen and bounding box stroke
		"strokeRect",       // Bounding box drag outline
		"fillRect",         // Bounding box semi-transparent fill
		"rgba(234, 67, 53", // Semi-transparent red fill
		"swiss-b-pen",      // Pen button ID
		"swiss-b-rect",     // Box button ID
		"swiss-b-inspect",  // Inspect element button ID
		"swiss-b-clear",    // Clear button ID
	}

	for _, token := range requiredAnnotationTokens {
		if !strings.Contains(js, token) {
			t.Errorf("expected script to contain annotation token %q", token)
		}
	}
}

func TestGenerateAuxiliaryPluginsScript_DOMInspector(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()

	requiredInspectorTokens := []string{
		"getCssSelector",           // CSS selector generator function
		"outerHTML",                // Outer HTML capture
		"getBoundingClientRect",    // Bounding rect measurement
		"SWISS_INSPECT_RESULT",     // Message bridge token
		"swiss-dom-inspect-overlay", // Highlight overlay ID
		"swiss-dom-inspect-badge",   // Selector tag badge ID
	}

	for _, token := range requiredInspectorTokens {
		if !strings.Contains(js, token) {
			t.Errorf("expected script to contain DOM inspector token %q", token)
		}
	}
}

func TestGenerateAuxiliaryPluginsScript_SendToChatWorkflow(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()

	requiredChatTokens := []string{
		"swiss-b-send-chat",     // Send button
		"toBlob",                // PNG blob conversion
		"annotation.png",        // Synthesized File name
		"DataTransfer",          // File transfer API
		`input[type="file"]`,    // Composer file input query
		"__lexicalEditor",       // Antigravity Lexical editor instance
		"insertTextToChatInput", // Fallback text insertion
	}

	for _, token := range requiredChatTokens {
		if !strings.Contains(js, token) {
			t.Errorf("expected script to contain Send to Chat token %q", token)
		}
	}
}

func TestGenerateAuxiliaryPluginsScript_DeviceFrames(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()

	requiredDeviceTokens := []string{
		"iphone-16-pro",
		"402", // iPhone 16 Pro width
		"pixel-9",
		"412", // Pixel 9 width
		"ipad",
		"820", // iPad width
	}

	for _, token := range requiredDeviceTokens {
		if !strings.Contains(js, token) {
			t.Errorf("expected script to contain device frame token %q", token)
		}
	}
}

func TestGenerateAuxiliaryPluginsScript_EditorAndContextMenuIcons(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()

	// Verify editor toolbar buttons have SVGs and clean labels
	editorChecks := []string{
		`id="swiss-ed-annotate"><svg`,
		`<span>Annotate to Chat</span>`,
		`id="swiss-ed-save"><svg`,
		`<span>Save</span>`,
		`id="swiss-ed-back"><svg`,
		`<span>Files</span>`,
	}
	for _, token := range editorChecks {
		if !strings.Contains(js, token) {
			t.Errorf("expected script to contain editor toolbar token %q", token)
		}
	}

	// Verify context menu items have SVGs and clean labels
	contextChecks := []string{
		`id="ctx-rename"><svg`,
		`<span>Rename</span>`,
		`id="ctx-copy"><svg`,
		`<span>Copy Path</span>`,
		`id="ctx-delete" style="color:#ef4444;"><svg`,
		`<span>Delete</span>`,
		`id="ctx-reveal"><svg`,
		`<span>Reveal in File Manager</span>`,
		`id="ctx-term"><svg`,
		`<span>Open in Terminal</span>`,
	}
	for _, token := range contextChecks {
		if !strings.Contains(js, token) {
			t.Errorf("expected script to contain context menu token %q", token)
		}
	}
}

func TestGenerateAuxiliaryPluginsScript_MemoViewAndTelemetryBadge(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()

	// Memo view checks
	memoChecks := []string{
		`id="swiss-m-record-audio"><svg`,
		`<span>Voice Memo</span>`,
		`id="m-del" title="Delete Memo"`,
		`<svg viewBox="0 0 24 24" width="11" height="11"`,
		`LOCAL_MEMOS_KEY = "antigravity_swiss_memos_backup"`,
		`Array.isArray(data.memos)`,
		`saveLocalMemos`,
		`getLocalMemos`,
		`titleStr = "[Voice] "`,
		`Stop Recording</span>`,
	}
	for _, token := range memoChecks {
		if !strings.Contains(js, token) {
			t.Errorf("expected script to contain memo view token %q", token)
		}
	}

	// In-chat telemetry badge checks
	telemetryChecks := []string{
		`<polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/>`,
		`tokens (Prompt:`,
		`swiss-telemetry-badge`,
	}
	for _, token := range telemetryChecks {
		if !strings.Contains(js, token) {
			t.Errorf("expected script to contain telemetry badge token %q", token)
		}
	}

	// Strict verification: ensure no decorative emojis exist in script
	forbiddenEmojis := []string{
		"🎙️", "💬", "💾", "✏️", "📋", "🗑️", "📂", "⚡", "📁", "📜", "📝", "📕", "📄", "⏹️",
	}
	for _, emoji := range forbiddenEmojis {
		if strings.Contains(js, emoji) {
			t.Errorf("script contains forbidden decorative emoji %q", emoji)
		}
	}
}

