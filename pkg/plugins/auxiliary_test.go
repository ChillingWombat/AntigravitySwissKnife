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
		".swiss-port-list",
		".swiss-port-add-box",
		".swiss-port-add-btn",
		".swiss-port-add-icon",
		".swiss-port-add-input",
		".swiss-browser-tools-row",
		".swiss-browser-device-group",
		".swiss-browser-tools-divider",
		".swiss-browser-annotation-group",
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
		".swiss-prompt-backdrop",
		".swiss-prompt-dialog",
		".swiss-prompt-input",
		".swiss-memo-composer",
		".swiss-memo-composer-textarea",
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
		"showSwissPrompt",
		"swiss-m-composer",
		"swiss-m-composer-input",
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
		`id: newMemo.id`,
		`swiss-m-composer`,
		`swiss-m-composer-input`,
		`<span>Save Memo</span>`,
		`showSwissPrompt`,
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

func TestAuxiliaryFileExplorerMultiSelectAndContextMenu(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	css := GenerateAuxiliaryPluginsCSS()

	// 1. Verify CSS user-select none, high z-index, and selected highlight
	cssTokens := []string{
		"user-select: none",
		"-webkit-user-select: none",
		"z-index: 99999",
		".swiss-file-row.selected",
		".swiss-file-row.cut",
		".swiss-context-item.disabled",
	}
	for _, tok := range cssTokens {
		if !strings.Contains(css, tok) {
			t.Errorf("expected CSS to contain %q", tok)
		}
	}

	// 2. Verify Multi-Select with Ctrl / Cmd key
	multiSelectTokens := []string{
		"selectedPaths = new Set()",
		"e.ctrlKey || e.metaKey",
		`selectedPaths.has(item.path)`,
		`selectedPaths.add(item.path)`,
		`selectedPaths.delete(item.path)`,
		`classList.add("selected")`,
		`classList.remove("selected")`,
	}
	for _, tok := range multiSelectTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected JS to contain multi-select token %q", tok)
		}
	}

	// 3. Verify Context Menu options (Copy, Cut, Paste, Blank context menu)
	contextTokens := []string{
		`id="ctx-file-copy"`,
		`<span>Copy</span>`,
		`id="ctx-file-cut"`,
		`<span>Cut</span>`,
		`id="ctx-file-paste"`,
		`<span>Paste</span>`,
		`id="ctx-blank-paste"`,
		`id="ctx-blank-new-file"`,
		`id="ctx-blank-new-dir"`,
		`id="ctx-blank-refresh"`,
		`id="ctx-blank-reveal"`,
		`id="ctx-blank-term"`,
		"positionContextMenu",
		"removeContextMenu",
		"executePaste",
		"executeDelete",
	}
	for _, tok := range contextTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected JS to contain context menu token %q", tok)
		}
	}

	// 4. Verify propagation and bubble prevention
	bubbleTokens := []string{
		"e.stopPropagation()",
		"e.preventDefault()",
		"window.getSelection()?.removeAllRanges()",
	}
	for _, tok := range bubbleTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected JS to contain bubble prevention token %q", tok)
		}
	}

	// 5. Verify spaces in path and URI decoding
	pathTokens := []string{
		"decodeURIComponent",
		"encodeURIComponent(targetPath)",
		"targetPath.replace",
	}
	for _, tok := range pathTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected JS to contain path token %q", tok)
		}
	}
}

func TestAuxiliaryFileExplorerNavigationToolbar(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	css := GenerateAuxiliaryPluginsCSS()

	// 1. Verify CSS disabled button rule exists
	if !strings.Contains(css, ".swiss-browser-btn:disabled") {
		t.Errorf("expected CSS to define .swiss-browser-btn:disabled")
	}

	// 2. Verify all address bar buttons and input exist in JS
	buttons := []struct {
		id    string
		title string
	}{
		{"swiss-f-back", "Back"},
		{"swiss-f-up", "Up Directory"},
		{"swiss-f-home", "Home Folder"},
		{"swiss-f-refresh", "Refresh"},
		{"swiss-f-reveal", "Open in System File Manager"},
		{"swiss-f-term", "Open in Terminal"},
	}

	for _, b := range buttons {
		if !strings.Contains(js, `id="`+b.id+`"`) {
			t.Errorf("expected JS to contain button id %q", b.id)
		}
		if !strings.Contains(js, `title="`+b.title+`"`) {
			t.Errorf("expected JS to contain button title %q", b.title)
		}
	}

	// 3. Verify ordering: swiss-f-back, swiss-f-up, swiss-f-home, swiss-f-refresh appear BEFORE swiss-f-path;
	// swiss-f-reveal and swiss-f-term appear AFTER swiss-f-path
	idxBack := strings.Index(js, `id="swiss-f-back"`)
	idxUp := strings.Index(js, `id="swiss-f-up"`)
	idxHome := strings.Index(js, `id="swiss-f-home"`)
	idxRefresh := strings.Index(js, `id="swiss-f-refresh"`)
	idxPath := strings.Index(js, `id="swiss-f-path"`)
	idxReveal := strings.Index(js, `id="swiss-f-reveal"`)
	idxTerm := strings.Index(js, `id="swiss-f-term"`)

	if idxBack == -1 || idxUp == -1 || idxHome == -1 || idxRefresh == -1 || idxPath == -1 || idxReveal == -1 || idxTerm == -1 {
		t.Fatalf("one or more address bar element IDs not found in JS")
	}

	// Check that Back, Up, Home, Refresh are strictly before Path input
	if !(idxBack < idxUp && idxUp < idxHome && idxHome < idxRefresh && idxRefresh < idxPath) {
		t.Errorf("expected button order swiss-f-back < swiss-f-up < swiss-f-home < swiss-f-refresh < swiss-f-path, got indices: back=%d, up=%d, home=%d, refresh=%d, path=%d",
			idxBack, idxUp, idxHome, idxRefresh, idxPath)
	}

	// Check that Reveal and Term are strictly after Path input
	if !(idxPath < idxReveal && idxReveal < idxTerm) {
		t.Errorf("expected path input to be before reveal and term buttons, got indices: path=%d, reveal=%d, term=%d",
			idxPath, idxReveal, idxTerm)
	}

	// 4. Verify history stack, home navigation, monotonic request ID and disabled initialization logic
	navTokens := []string{
		`id="swiss-f-back" title="Back" disabled`,
		"fileHistory",
		"fileHistory.push(prevPath)",
		"fileHistory.pop()",
		`toolbar.querySelector("#swiss-f-back").onclick`,
		`toolbar.querySelector("#swiss-f-home").onclick`,
		`loadFiles("~")`,
		`toolbar.querySelector("#swiss-f-refresh").onclick`,
		"updateNavButtons",
		"backBtn.disabled = fileHistory.length === 0",
		"currentFetchId",
		"fetchId !== currentFetchId",
		"loadFiles(targetPath, false)",
	}

	for _, token := range navTokens {
		if !strings.Contains(js, token) {
			t.Errorf("expected JS to contain navigation logic token %q", token)
		}
	}
}

func TestAuxiliaryPromptReplacementAndInlineComposer(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	css := GenerateAuxiliaryPluginsCSS()

	// 1. Strict assertion: window.prompt and prompt( are completely eliminated from script
	if strings.Contains(js, "window.prompt") {
		t.Errorf("expected script to not contain window.prompt")
	}
	if strings.Contains(js, "prompt(") {
		t.Errorf("expected script to not contain prompt( calls")
	}

	// 2. Custom prompt modal exists and has proper keyboard and DOM handling
	promptTokens := []string{
		"function showSwissPrompt",
		"swiss-prompt-backdrop",
		"swiss-prompt-dialog",
		"swiss-prompt-title",
		"swiss-prompt-input",
		"swiss-prompt-actions",
		`document.addEventListener("keydown", onKeyDown, true)`,
		`e.key === "Escape"`,
		`e.key === "Enter"`,
		"inputEl.focus()",
	}
	for _, tok := range promptTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected script to contain prompt token %q", tok)
		}
	}

	// 3. Inline memo composer exists and is wired to + Text Memo button
	composerTokens := []string{
		"swiss-memo-composer",
		"swiss-m-composer",
		"swiss-m-composer-input",
		"swiss-m-composer-cancel",
		"swiss-m-composer-save",
		`topBar.querySelector("#swiss-m-new-text").onclick`,
		"openComposer",
		"closeComposer",
		"createAndSaveMemo",
	}
	for _, tok := range composerTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected script to contain composer token %q", tok)
		}
	}

	// 4. Voice memo uses showSwissPrompt and releases microphone tracks
	voiceTokens := []string{
		`await showSwissPrompt("Voice recorded! Enter a transcript / note title:"`,
		`mediaStream.getTracks().forEach(t => t.stop())`,
		`loadMemos(true)`,
		`created_at: newMemo.created_at`,
		`e.isComposing || e.keyCode === 229`,
		`resolved = true`,
	}
	for _, tok := range voiceTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected script to contain voice/robustness token %q", tok)
		}
	}

	// 5. CSS definitions for modal and inline composer
	cssSelectors := []string{
		".swiss-memo-composer",
		".swiss-memo-composer-textarea",
		".swiss-memo-composer-actions",
		".swiss-prompt-backdrop",
		".swiss-prompt-dialog",
		".swiss-prompt-title",
		".swiss-prompt-input",
		".swiss-prompt-actions",
	}
	for _, sel := range cssSelectors {
		if !strings.Contains(css, sel) {
			t.Errorf("expected CSS to contain selector %q", sel)
		}
	}
}

func TestBrowserViewToolbarLayout(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	css := GenerateAuxiliaryPluginsCSS()

	// 1. Verify CSS classes for two-tier layout exist
	layoutClasses := []string{
		".swiss-browser-nav-row",
		".swiss-browser-tools-row",
		".swiss-browser-device-group",
		".swiss-browser-tools-divider",
		".swiss-browser-annotation-group",
		".swiss-browser-port-bar",
		".swiss-port-list",
		".swiss-port-add-box",
	}
	for _, cls := range layoutClasses {
		if !strings.Contains(css, cls) {
			t.Errorf("expected CSS to define %q", cls)
		}
	}

	// 2. Verify upper row contains nav buttons, URL input, and quick ports bar
	idxNavRow := strings.Index(js, `<div class="swiss-browser-nav-row">`)
	idxBack := strings.Index(js, `id="swiss-b-back"`)
	idxFwd := strings.Index(js, `id="swiss-b-fwd"`)
	idxRefresh := strings.Index(js, `id="swiss-b-refresh"`)
	idxUrl := strings.Index(js, `id="swiss-b-url"`)
	idxPortBar := strings.Index(js, `<div class="swiss-browser-port-bar">`)
	idxToolsRow := strings.Index(js, `<div class="swiss-browser-tools-row">`)

	if idxNavRow == -1 || idxBack == -1 || idxFwd == -1 || idxRefresh == -1 || idxUrl == -1 || idxPortBar == -1 || idxToolsRow == -1 {
		t.Fatalf("one or more upper row elements missing in JS")
	}

	if !(idxNavRow < idxBack && idxBack < idxFwd && idxFwd < idxRefresh && idxRefresh < idxUrl && idxUrl < idxPortBar && idxPortBar < idxToolsRow) {
		t.Errorf("expected upper row order: nav-row < back < fwd < refresh < url < port-bar < tools-row, got indices: nav=%d, back=%d, fwd=%d, ref=%d, url=%d, ports=%d, tools=%d",
			idxNavRow, idxBack, idxFwd, idxRefresh, idxUrl, idxPortBar, idxToolsRow)
	}

	// 3. Verify lower row contains device controls and annotation buttons
	idxDeviceGroup := strings.Index(js, `<div class="swiss-browser-device-group">`)
	idxDevice := strings.Index(js, `id="swiss-b-device"`)
	idxScale := strings.Index(js, `id="swiss-b-scale"`)
	idxTouch := strings.Index(js, `id="swiss-b-touch"`)
	idxDivider := strings.Index(js, `<div class="swiss-browser-tools-divider">`)
	idxAnnotateGroup := strings.Index(js, `<div class="swiss-browser-annotation-group">`)
	idxPen := strings.Index(js, `id="swiss-b-pen"`)
	idxRect := strings.Index(js, `id="swiss-b-rect"`)
	idxInspect := strings.Index(js, `id="swiss-b-inspect"`)
	idxClear := strings.Index(js, `id="swiss-b-clear"`)
	idxSendChat := strings.Index(js, `id="swiss-b-send-chat"`)

	if idxDeviceGroup == -1 || idxDevice == -1 || idxScale == -1 || idxTouch == -1 || idxDivider == -1 || idxAnnotateGroup == -1 || idxPen == -1 || idxRect == -1 || idxInspect == -1 || idxClear == -1 || idxSendChat == -1 {
		t.Fatalf("one or more lower row elements missing in JS")
	}

	if !(idxToolsRow < idxDeviceGroup && idxDeviceGroup < idxDevice && idxDevice < idxScale && idxScale < idxTouch && idxTouch < idxDivider && idxDivider < idxAnnotateGroup && idxAnnotateGroup < idxPen && idxPen < idxRect && idxRect < idxInspect && idxInspect < idxClear && idxClear < idxSendChat) {
		t.Errorf("expected lower row order: tools-row < device-group < device < scale < touch < divider < annotate-group < pen < rect < inspect < clear < send-chat")
	}
}

func TestBrowserViewQuickPortsManagement(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	css := GenerateAuxiliaryPluginsCSS()

	// 1. Verify CSS styles for port chips, context menu, and add-port box
	portCssClasses := []string{
		".swiss-port-chip",
		".swiss-port-chip:hover",
		".swiss-port-chip.active",
		".swiss-port-add-box",
		".swiss-port-add-box.expanded",
		".swiss-port-add-btn",
		".swiss-port-add-icon",
		".swiss-port-add-input",
		".swiss-context-menu",
		".swiss-context-item",
	}
	for _, cls := range portCssClasses {
		if !strings.Contains(css, cls) {
			t.Errorf("expected CSS to define %q", cls)
		}
	}

	// 2. Verify localStorage persistence logic
	persistenceTokens := []string{
		`QUICK_PORTS_KEY = "antigravity_swiss_quick_ports"`,
		"DEFAULT_PORTS",
		"getSavedPorts",
		"savePorts",
		"renderQuickPorts",
		`localStorage.getItem(QUICK_PORTS_KEY)`,
		`localStorage.setItem(QUICK_PORTS_KEY`,
	}
	for _, tok := range persistenceTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected JS to contain persistence token %q", tok)
		}
	}

	// 3. Verify right-click context menu deletion
	contextMenuTokens := []string{
		"chip.oncontextmenu",
		"showPortContextMenu",
		`className = "swiss-context-menu"`,
		`id="ctx-port-delete"`,
		"deletePort",
		"removeContextMenu",
		"positionContextMenu",
	}
	for _, tok := range contextMenuTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected JS to contain context menu token %q", tok)
		}
	}

	// 4. Verify expandable add button and inline input
	addBoxTokens := []string{
		`id="swiss-port-add-box"`,
		`id="swiss-port-add-btn"`,
		`class="swiss-port-add-icon"`,
		`id="swiss-port-add-input"`,
		"openAddPort",
		"closeAddPort",
		"submitNewPort",
		`addBox.classList.add("expanded")`,
		`addBox.classList.remove("expanded")`,
		`e.key === "Enter"`,
		`e.key === "Escape"`,
		"onOutsideAddPortMousedown",
	}
	for _, tok := range addBoxTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected JS to contain add-port token %q", tok)
		}
	}
}



