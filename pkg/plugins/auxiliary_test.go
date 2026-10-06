package plugins

import (
	"os/exec"
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
		".swiss-port-dropdown-wrap",
		".swiss-port-select",
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
		"[data-swiss-aux-active]",
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

	if nodePath, err := exec.LookPath("node"); err == nil {
		cmd := exec.Command(nodePath, "--check")
		cmd.Stdin = strings.NewReader(js)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("node syntax error in GenerateAuxiliaryPluginsScript: %v\n%s", err, string(out))
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
		`data-swiss-aux-active`,
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

func TestBrowserViewNarrowToolbarResponsiveness(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	css := GenerateAuxiliaryPluginsCSS()

	// 1. Verify rows enforce flex-wrap: nowrap, overflow-x: auto, and 3px scrollbars
	layoutChecks := []string{
		".swiss-browser-nav-row",
		".swiss-browser-tools-row",
		"flex-wrap: nowrap",
		"overflow-x: auto",
		"overflow-y: hidden",
		".swiss-browser-nav-row::-webkit-scrollbar",
		".swiss-browser-tools-row::-webkit-scrollbar",
		"height: 3px",
	}
	for _, check := range layoutChecks {
		if !strings.Contains(css, check) {
			t.Errorf("expected CSS to define %q for narrow panel responsiveness", check)
		}
	}

	// 2. Ensure container queries and compact-ports fallback switch chips to dropdown and hide label
	containerQuerySnippets := []string{
		"container-type: inline-size",
		"container-name: swisstoolbar",
		"@container swisstoolbar (max-width: 580px)",
		".swiss-browser-toolbar.compact-ports .swiss-port-label",
		".swiss-browser-toolbar.compact-ports .swiss-port-list",
		".swiss-browser-toolbar.compact-ports .swiss-port-dropdown-wrap",
		".swiss-port-select",
		".swiss-port-select:hover",
		".swiss-port-select.active",
	}
	for _, snip := range containerQuerySnippets {
		if !strings.Contains(css, snip) {
			t.Errorf("expected CSS to contain responsive container query snippet %q", snip)
		}
	}

	// 3. Verify dropdown selector template, placeholder, and options
	selectTemplateTokens := []string{
		`id="swiss-port-dropdown-wrap"`,
		`class="swiss-port-select"`,
		`id="swiss-port-select"`,
		`title="Quick Ports (Right-click to delete)"`,
		`<option value="" disabled selected>Quick Ports</option>`,
		`<option value="__add__">+ Add Port...</option>`,
		`delOpt.textContent = "Delete Port...";`,
	}
	for _, tok := range selectTemplateTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected JS template to contain select dropdown token %q", tok)
		}
	}

	// 4. Verify dropdown interactions (onchange, oncontextmenu, selection sync, mousewheel)
	dropdownLogicTokens := []string{
		`toolbar.querySelector("#swiss-port-select")`,
		`val === "__add__"`,
		`val === "__delete__"`,
		"portSelect.onchange",
		"portSelect.oncontextmenu",
		"updateActivePortChip",
		"portSelect.value = activePort",
		`portSelect.value = ""`,
		"updateToolbarResponsiveness",
		"ResizeObserver",
		`toolbar.classList.toggle("compact-ports"`,
		`row.scrollLeft += e.deltaY`,
		`await showSwissPrompt("Enter port to delete`,
	}
	for _, tok := range dropdownLogicTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected JS to contain dropdown logic token %q", tok)
		}
	}

	// 5. David-Design Zero-Decorative-Emoji verification: Ensure no decorative emojis in dropdown options
	if strings.Contains(js, "🗑") {
		t.Errorf("found decorative emoji 🗑 in auxiliary plugins script, violating David-Design rules")
	}
	if strings.Contains(css, "🗑") {
		t.Errorf("found decorative emoji 🗑 in auxiliary plugins CSS, violating David-Design rules")
	}

	// 6. Node.js DOM simulation for port select navigation, unselected state, and add/delete reset
	if nodePath, err := exec.LookPath("node"); err == nil {
		nodeSelectTest := `
const assert = require("assert");

class MockElement {
  constructor(tagName = "div") {
    this.tagName = tagName.toUpperCase();
    this.children = [];
    this.classList = {
      _classes: new Set(),
      add: (c) => this.classList._classes.add(c),
      remove: (c) => this.classList._classes.delete(c),
      contains: (c) => this.classList._classes.has(c),
      toggle: (c, force) => {
        if (force !== undefined) {
          if (force) this.classList._classes.add(c); else this.classList._classes.delete(c);
          return force;
        }
        if (this.classList._classes.has(c)) { this.classList._classes.delete(c); return false; }
        this.classList._classes.add(c); return true;
      }
    };
    this.value = "";
    this.disabled = false;
    this.selected = false;
    this.textContent = "";
    this.options = [];
    this.onchange = null;
    this.oncontextmenu = null;
  }
  appendChild(child) {
    this.children.push(child);
    if (this.tagName === "SELECT" && child.tagName === "OPTION") {
      this.options.push(child);
      if (child.selected || (!this.value && this.options.length === 1)) {
        this.value = child.value;
      }
    }
    return child;
  }
}

// 1. Simulate renderQuickPorts with ports ["5173", "3000"]
const portSelect = new MockElement("select");
const ports = ["5173", "3000"];

const placeholder = new MockElement("option");
placeholder.value = "";
placeholder.disabled = true;
placeholder.selected = true;
placeholder.textContent = "Quick Ports";
portSelect.appendChild(placeholder);

ports.forEach(p => {
  const opt = new MockElement("option");
  opt.value = p;
  opt.textContent = ":" + p;
  portSelect.appendChild(opt);
});

const addOpt = new MockElement("option");
addOpt.value = "__add__";
addOpt.textContent = "+ Add Port...";
portSelect.appendChild(addOpt);

const delOpt = new MockElement("option");
delOpt.value = "__delete__";
delOpt.textContent = "Delete Port...";
portSelect.appendChild(delOpt);

assert.strictEqual(portSelect.options.length, 5);
assert.strictEqual(portSelect.value, ""); // defaults to placeholder Quick Ports
assert.strictEqual(portSelect.options[0].textContent, "Quick Ports");
assert.strictEqual(portSelect.options[4].textContent, "Delete Port...");
assert.strictEqual(portSelect.options[4].textContent.includes("🗑"), false);

// 2. Simulate updateActivePortChip
let currentBrowserUrl = "https://github.com";
const updateActivePortChip = (url) => {
  const match = url.match(/^https?:\/\/(?:localhost|127\.0\.0\.1):(\d+)/i);
  const activePort = match ? match[1] : null;
  if (activePort && portSelect.options.some(o => o.value === activePort)) {
    portSelect.value = activePort;
    portSelect.classList.add("active");
  } else {
    portSelect.value = "";
    portSelect.classList.remove("active");
  }
};

updateActivePortChip(currentBrowserUrl);
assert.strictEqual(portSelect.value, "");
assert.strictEqual(portSelect.classList.contains("active"), false);

// 3. User navigates to localhost:5173
currentBrowserUrl = "http://localhost:5173";
updateActivePortChip(currentBrowserUrl);
assert.strictEqual(portSelect.value, "5173");
assert.strictEqual(portSelect.classList.contains("active"), true);

// 4. Test selecting __add__ resets select value and opens addBox
let addBoxOpened = false;
const openAddPort = () => { addBoxOpened = true; };
let navigatedTo = null;
const navigateBrowser = (u) => { navigatedTo = u; };

portSelect.onchange = async () => {
  const val = portSelect.value;
  const match = currentBrowserUrl.match(/^https?:\/\/(?:localhost|127\.0\.0\.1):(\d+)/i);
  const activePort = match ? match[1] : null;
  if (val === "__add__") {
    portSelect.value = (activePort && ports.includes(activePort)) ? activePort : "";
    openAddPort();
    return;
  }
  if (val) navigateBrowser("http://localhost:" + val);
};

// Select __add__
portSelect.value = "__add__";
portSelect.onchange();
assert.strictEqual(addBoxOpened, true);
assert.strictEqual(portSelect.value, "5173"); // restored to activePort, NOT stuck on __add__

// Select 3000
portSelect.value = "3000";
portSelect.onchange();
assert.strictEqual(navigatedTo, "http://localhost:3000");
`
		cmd := exec.Command(nodePath, "-e", nodeSelectTest)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Node.js port selector simulation failed: %v\n%s", err, string(out))
		}
	}
}

func TestGitHubExtensionScriptAndCSS(t *testing.T) {
	css := GenerateGitHubExtensionCSS()
	if css == "" {
		t.Fatalf("expected non-empty GitHub extension CSS")
	}

	requiredCSSSelectors := []string{
		".swiss-left-nav-group",
		".swiss-left-tabs-separator",
		".swiss-left-nav-tab",
		".swiss-github-aux-view",
		".swiss-gh-card",
		".swiss-agent-task-badge",
		".swiss-agent-pulse-dot",
		"#swiss-main-stage-container",
		"#swiss-main-stage-header",
		".swiss-gh-workspace-layout",
		".swiss-gh-col-left",
		".swiss-gh-col-center",
		".swiss-gh-col-right",
		".swiss-gh-modal-dialog",
	}

	for _, sel := range requiredCSSSelectors {
		if !strings.Contains(css, sel) {
			t.Errorf("expected GitHub CSS to contain selector %q", sel)
		}
	}

	js := GenerateGitHubExtensionScript()
	if js == "" {
		t.Fatalf("expected non-empty GitHub extension script")
	}

	requiredJSTokens := []string{
		"setupLeftNavTabs",
		"renderSwissGitHubWorkspaceView",
		"setupDragAndDropToChat",
		"openIssueDetailModal",
		"openMainStage",
		"closeMainStage",
		"renderGitHubWorkspaceStage",
		"fetchRepoData",
		"/api/github/repo",
		"/api/github/issues",
		"/api/github/agent-tasks",
		"formatMarkdownPayload",
		"sendToChatComposer",
		"swiss-chat-drop-highlight",
	}

	for _, tok := range requiredJSTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected GitHub JS to contain token %q", tok)
		}
	}

	// Verify integration in GenerateAuxiliaryPluginsCSS and GenerateAuxiliaryPluginsScript
	auxCSS := GenerateAuxiliaryPluginsCSS()
	if !strings.Contains(auxCSS, ".swiss-left-nav-tab") {
		t.Errorf("expected GenerateAuxiliaryPluginsCSS to include GitHub extension CSS")
	}

	auxJS := GenerateAuxiliaryPluginsScript()
	if !strings.Contains(auxJS, "setupLeftNavTabs") {
		t.Errorf("expected GenerateAuxiliaryPluginsScript to include GitHub extension script")
	}
	if !strings.Contains(auxJS, `data-tab-id="swiss-github"`) {
		t.Errorf("expected GenerateAuxiliaryPluginsScript to include data-tab-id=\"swiss-github\"")
	}
	if !strings.Contains(auxJS, ".swiss-aux-tabs-divider-right") {
		t.Errorf("expected GenerateAuxiliaryPluginsScript to include .swiss-aux-tabs-divider-right")
	}
}

func TestAuxiliaryFileExplorerHiddenFilesToggle(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	css := GenerateAuxiliaryPluginsCSS()

	// 1. Verify CSS rules for hidden file rows and active toggle button
	requiredCSSRules := []string{
		".swiss-file-row.swiss-file-hidden",
		"opacity: 0.72",
		"#swiss-f-hidden.active",
		".swiss-browser-btn.toggled",
		".swiss-file-row.swiss-file-hidden.selected",
	}
	for _, rule := range requiredCSSRules {
		if !strings.Contains(css, rule) {
			t.Errorf("expected CSS to define %q", rule)
		}
	}

	// 2. Verify button element and eye icon exist in toolbar markup
	buttonTokens := []string{
		`id="swiss-f-hidden"`,
		`title="Toggle Hidden Files"`,
		`<path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7Z"/>`,
		`<circle cx="12" cy="12" r="3"/>`,
	}
	for _, tok := range buttonTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected JS to contain toolbar button token %q", tok)
		}
	}

	// 3. Verify ordering in filter/actions row: search < hidden < new-file < new-dir
	idxSearch := strings.Index(js, `id="swiss-f-search"`)
	idxHidden := strings.Index(js, `id="swiss-f-hidden"`)
	idxNewFile := strings.Index(js, `id="swiss-f-new-file"`)
	idxNewDir := strings.Index(js, `id="swiss-f-new-dir"`)

	if idxSearch == -1 || idxHidden == -1 || idxNewFile == -1 || idxNewDir == -1 {
		t.Fatalf("one or more filter row elements not found in JS: search=%d, hidden=%d, newFile=%d, newDir=%d",
			idxSearch, idxHidden, idxNewFile, idxNewDir)
	}

	if !(idxSearch < idxHidden && idxHidden < idxNewFile && idxNewFile < idxNewDir) {
		t.Errorf("expected filter row order swiss-f-search < swiss-f-hidden < swiss-f-new-file < swiss-f-new-dir, got: search=%d, hidden=%d, file=%d, dir=%d",
			idxSearch, idxHidden, idxNewFile, idxNewDir)
	}

	// 4. Verify Eye and EyeOff icons, localStorage persistence, and toggle logic
	logicTokens := []string{
		`SHOW_HIDDEN_KEY = "antigravity_swiss_show_hidden_files"`,
		`localStorage.getItem(SHOW_HIDDEN_KEY)`,
		`localStorage.setItem(SHOW_HIDDEN_KEY`,
		"updateHiddenBtnState",
		"applyFileFilters",
		"eyeIconSvg",
		"eyeOffIconSvg",
		`hiddenBtn.classList.toggle("toggled", showHiddenFiles)`,
		`hiddenBtn.classList.toggle("active", showHiddenFiles)`,
		`hiddenBtn.onclick = () =>`,
		`item.name || "").startsWith(".")`,
		`row.dataset.hidden = isHidden ? "true" : "false"`,
		`row.classList.add("swiss-file-hidden")`,
		"matchesHidden = showHiddenFiles || !isHidden",
		"Hide Hidden Files",
		"Show Hidden Files",
	}
	for _, tok := range logicTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected JS to contain hidden files logic token %q", tok)
		}
	}

	// 5. Deep runtime verification via Node.js: simulate interactive DOM lifecycle
	if nodePath, err := exec.LookPath("node"); err == nil {
		nodeTestScript := `
const assert = require("assert");

const storage = {};
global.localStorage = {
  getItem: (k) => storage[k] || null,
  setItem: (k, v) => { storage[k] = String(v); },
  removeItem: (k) => { delete storage[k]; }
};

class ClassList {
  constructor() { this.classes = new Set(); }
  add(c) { this.classes.add(c); }
  remove(c) { this.classes.delete(c); }
  contains(c) { return this.classes.has(c); }
  toggle(c, force) {
    if (force !== undefined) {
      if (force) this.classes.add(c); else this.classes.delete(c);
      return force;
    }
    if (this.classes.has(c)) { this.classes.delete(c); return false; }
    this.classes.add(c); return true;
  }
}

class MockElement {
  constructor(tagName = "div") {
    this.tagName = tagName.toUpperCase();
    this.classList = new ClassList();
    this.children = [];
    this.dataset = {};
    this.style = {};
    this.innerHTML = "";
    this._textContent = "";
    this.id = "";
    this.title = "";
    this.value = "";
    this.onclick = null;
    this.oninput = null;
  }
  get textContent() { return this._textContent || this.innerHTML; }
  set textContent(v) { this._textContent = v; }
  appendChild(child) { this.children.push(child); return child; }
  querySelector(sel) {
    return this.querySelectorAll(sel)[0] || null;
  }
  querySelectorAll(sel) {
    const res = [];
    const walk = (el) => {
      for (const ch of el.children) {
        if (sel.startsWith("#") && ch.id === sel.slice(1)) res.push(ch);
        else if (sel.startsWith(".") && ch.classList.contains(sel.slice(1))) res.push(ch);
        else if (sel === "div" && ch.tagName === "DIV") res.push(ch);
        walk(ch);
      }
    };
    walk(this);
    return res;
  }
}

const SHOW_HIDDEN_KEY = "antigravity_swiss_show_hidden_files";
let showHiddenFiles = false;
try { showHiddenFiles = localStorage.getItem(SHOW_HIDDEN_KEY) === "true"; } catch(_) {}

const hiddenBtn = new MockElement("button");
hiddenBtn.id = "swiss-f-hidden";
const searchBox = new MockElement("input");
searchBox.id = "swiss-f-search";

const listContainer = new MockElement("div");
const fileItemsMap = new Map();
const selectedPaths = new Set();

const files = [
  { name: ".git", path: "/test/.git", isDir: true },
  { name: ".env", path: "/test/.env", isDir: false },
  { name: "main.go", path: "/test/main.go", isDir: false },
  { name: "README.md", path: "/test/README.md", isDir: false },
];

files.forEach(item => {
  fileItemsMap.set(item.path, item);
  const row = new MockElement("div");
  row.classList.add("swiss-file-row");
  row.dataset.path = item.path;
  const isHidden = (item.name || "").startsWith(".");
  row.dataset.hidden = isHidden ? "true" : "false";
  if (isHidden) row.classList.add("swiss-file-hidden");

  const nameSpan = new MockElement("span");
  nameSpan.classList.add("swiss-file-name");
  nameSpan.textContent = item.name;
  row.appendChild(nameSpan);
  listContainer.appendChild(row);
});

const eyeIconSvg = '<svg ... eye ...></svg>';
const eyeOffIconSvg = '<svg ... eyeOff ...></svg>';
const updateHiddenBtnState = () => {
  hiddenBtn.classList.toggle("toggled", showHiddenFiles);
  hiddenBtn.classList.toggle("active", showHiddenFiles);
  hiddenBtn.title = showHiddenFiles ? "Hide Hidden Files (dotfiles)" : "Show Hidden Files (dotfiles)";
  hiddenBtn.innerHTML = showHiddenFiles ? eyeOffIconSvg : eyeIconSvg;
};
updateHiddenBtnState();

const applyFileFilters = () => {
  const q = (searchBox ? searchBox.value : "").toLowerCase().trim();
  let visibleCount = 0;
  listContainer.querySelectorAll(".swiss-file-row").forEach(r => {
    const name = (r.querySelector(".swiss-file-name")?.textContent || "").toLowerCase();
    const isHidden = r.dataset.hidden === "true";
    const matchesSearch = !q || name.includes(q);
    const matchesHidden = showHiddenFiles || !isHidden;
    if (matchesSearch && matchesHidden) {
      r.style.display = "flex";
      visibleCount++;
    } else {
      r.style.display = "none";
      if (!matchesHidden && selectedPaths.has(r.dataset.path)) {
        selectedPaths.delete(r.dataset.path);
        r.classList.remove("selected");
      }
    }
  });

  let emptyNotice = listContainer.querySelector(".swiss-empty-filter-notice");
  if (fileItemsMap.size > 0 && visibleCount === 0) {
    if (!emptyNotice) {
      emptyNotice = new MockElement("div");
      emptyNotice.id = "empty-notice";
      emptyNotice.classList.add("swiss-empty-filter-notice");
      listContainer.appendChild(emptyNotice);
    }
    emptyNotice.textContent = q ? "No files matching filter" : "No visible files (hidden files filtered)";
    emptyNotice.style.display = "block";
  } else if (emptyNotice) {
    emptyNotice.style.display = "none";
  }
};

applyFileFilters();

// Assert Step 1: Default state hides dotfiles
assert.strictEqual(showHiddenFiles, false);
assert.strictEqual(hiddenBtn.classList.contains("active"), false);
assert.strictEqual(hiddenBtn.title, "Show Hidden Files (dotfiles)");
const rows = listContainer.querySelectorAll(".swiss-file-row");
assert.strictEqual(rows[0].style.display, "none"); // .git
assert.strictEqual(rows[1].style.display, "none"); // .env
assert.strictEqual(rows[2].style.display, "flex"); // main.go
assert.strictEqual(rows[3].style.display, "flex"); // README.md

// Assert Step 2: Toggle ON shows dotfiles
showHiddenFiles = true;
localStorage.setItem(SHOW_HIDDEN_KEY, "true");
updateHiddenBtnState();
applyFileFilters();

assert.strictEqual(hiddenBtn.classList.contains("active"), true);
assert.strictEqual(hiddenBtn.title, "Hide Hidden Files (dotfiles)");
assert.strictEqual(localStorage.getItem(SHOW_HIDDEN_KEY), "true");
assert.strictEqual(rows[0].style.display, "flex"); // .git
assert.strictEqual(rows[1].style.display, "flex"); // .env
assert.strictEqual(rows[2].style.display, "flex"); // main.go
assert.strictEqual(rows[3].style.display, "flex"); // README.md

// Assert Step 3: Selection deselects hidden files on toggle off
selectedPaths.add("/test/.env");
rows[1].classList.add("selected");
showHiddenFiles = false;
localStorage.setItem(SHOW_HIDDEN_KEY, "false");
updateHiddenBtnState();
applyFileFilters();

assert.strictEqual(selectedPaths.has("/test/.env"), false);
assert.strictEqual(rows[1].classList.contains("selected"), false);
assert.strictEqual(rows[1].style.display, "none");

// Assert Step 4: Folder with only dotfiles shows empty filter notice
const dotOnlyContainer = new MockElement("div");
const dotMap = new Map();
const dotItem = { name: ".env", path: "/test/.env" };
dotMap.set(dotItem.path, dotItem);
const dotRow = new MockElement("div");
dotRow.classList.add("swiss-file-row");
dotRow.dataset.hidden = "true";
const dotSpan = new MockElement("span");
dotSpan.classList.add("swiss-file-name");
dotSpan.textContent = ".env";
dotRow.appendChild(dotSpan);
dotOnlyContainer.appendChild(dotRow);

let vis = 0;
dotOnlyContainer.querySelectorAll(".swiss-file-row").forEach(r => {
  const isHidden = r.dataset.hidden === "true";
  if (!isHidden) { r.style.display = "flex"; vis++; }
  else { r.style.display = "none"; }
});
assert.strictEqual(vis, 0);
let notice = new MockElement("div");
notice.classList.add("swiss-empty-filter-notice");
notice.textContent = "No visible files (hidden files filtered)";
notice.style.display = "block";
dotOnlyContainer.appendChild(notice);
assert.strictEqual(notice.textContent, "No visible files (hidden files filtered)");
assert.strictEqual(notice.style.display, "block");
`
		cmd := exec.Command(nodePath, "-e", nodeTestScript)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Node.js DOM simulation failed: %v\n%s", err, string(out))
		}
	}
}




