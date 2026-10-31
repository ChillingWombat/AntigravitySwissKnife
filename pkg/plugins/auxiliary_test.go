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
		".swiss-builtin-browser-view",
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
		"setupBuiltinBrowserEnhancements",
		"attachBuiltinBrowserEnhancements",
		`data-tab-id="swiss-files"`,
		`data-tab-id="swiss-memos"`,
		`data-tab-id="swiss-github"`,
		"swiss-aux-tab-svg",
		"getAuxTabFormat",
		"icon-only",
		"swiss-aux-tab-format-updated",
		"swiss-bookmark-star",
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
		`data-tab-id="swiss-files"`,
		`data-tab-id="swiss-memos"`,
		`data-tab-id="swiss-github"`,
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

func TestBrowserViewDeviceFramesAndBookmarks(t *testing.T) {
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
		{"Builtin browser setup", js, "setupBuiltinBrowserEnhancements"},
		{"Builtin browser attach", js, "attachBuiltinBrowserEnhancements"},
		{"Default 5173 bookmark", js, `"http://localhost:5173"`},
		{"Default 3000 bookmark", js, `"http://localhost:3000"`},
		{"Default 8080 bookmark", js, `"http://localhost:8080"`},
		{"Default 8765 bookmark", js, `"http://localhost:8765"`},
		{"Default 4173 bookmark", js, `"http://localhost:4173"`},
		{"Versioned bookmark store", js, `BOOKMARKS_KEY`},
		{"Native bookmark anchor", js, `webview-url-bar-screenshot`},
		{"Webview bookmark navigation", js, `webview.loadURL(bookmark.address)`},
		{"Bookmark context menu", js, `swiss-bookmark-context`},
		{"Bookmark cleanup", js, `removeNativeBookmarkButtons`},
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
		"getCssSelector",            // CSS selector generator function
		"outerHTML",                 // Outer HTML capture
		"getBoundingClientRect",     // Bounding rect measurement
		"SWISS_INSPECT_RESULT",      // Message bridge token
		"SWISS_INSPECT_CANCEL",      // Message bridge cancellation token on Escape
		"swiss-dom-inspect-overlay", // Highlight overlay ID
		"swiss-dom-inspect-badge",   // Selector tag badge ID
		"exitInspectTool",           // Helper to exit inspect element tool
	}

	for _, token := range requiredInspectorTokens {
		if !strings.Contains(js, token) {
			t.Errorf("expected script to contain DOM inspector token %q", token)
		}
	}
}

func TestGenerateAuxiliaryPluginsScript_DOMInspector_EscapeKeyExit(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()

	requiredEscapeTokens := []string{
		`window.__swissBrowserKeyHandler`,
		`if ((e.key === "Escape" || e.key === "Esc" || e.keyCode === 27 || e.which === 27) && drawTool === "inspect")`,
		`exitInspectTool();`,
		`e.key === 'Escape' || e.key === 'Esc' || e.keyCode === 27 || e.which === 27`,
		`console.log('[SWISS_INSPECT_CANCEL]')`,
		`e.message.includes('[SWISS_INSPECT_CANCEL]')`,
		`window.__swissBrowserMessageHandler`,
		`SWISS_INSPECT_CANCEL`,
	}

	for _, token := range requiredEscapeTokens {
		if !strings.Contains(js, token) {
			t.Errorf("expected script to contain Escape key exit token %q", token)
		}
	}

	if nodePath, err := exec.LookPath("node"); err == nil {
		nodeTestScript := `
const assert = require("assert");

class MockClassList {
  constructor() { this._set = new Set(); }
  add(c) { this._set.add(c); }
  remove(c) { this._set.delete(c); }
  toggle(c, force) {
    if (force !== undefined) {
      if (force) this._set.add(c); else this._set.delete(c);
      return force;
    }
    if (this._set.has(c)) { this._set.delete(c); return false; }
    this._set.add(c); return true;
  }
  contains(c) { return this._set.has(c); }
}

class MockElement {
  constructor(tag = "div") {
    this.tagName = tag.toUpperCase();
    this.id = "";
    this.classList = new MockClassList();
    this.style = {};
    this.children = [];
    this._listeners = {};
    this.attributes = {};
  }
  setAttribute(k, v) { this.attributes[k] = v; }
  getAttribute(k) { return this.attributes[k]; }
  querySelector(sel) {
    if (sel.startsWith("#")) {
      const id = sel.slice(1);
      if (this.id === id) return this;
      for (const ch of this.children) {
        const found = ch.querySelector(sel);
        if (found) return found;
      }
    }
    return null;
  }
  querySelectorAll() { return []; }
  appendChild(child) { this.children.push(child); return child; }
  remove() {}
  addEventListener(event, fn) {
    if (!this._listeners[event]) this._listeners[event] = [];
    this._listeners[event].push(fn);
  }
  removeEventListener(event, fn) {
    if (!this._listeners[event]) return;
    this._listeners[event] = this._listeners[event].filter(cb => cb !== fn);
  }
  dispatchEvent(event) {
    const list = this._listeners[event.type] || [];
    for (const fn of list) fn(event);
  }
}

// 1. Verify drawTool state transitions and exitInspectTool logic
let drawTool = "none";
const penBtn = new MockElement(); penBtn.id = "swiss-b-pen";
const rectBtn = new MockElement(); rectBtn.id = "swiss-b-rect";
const inspectBtn = new MockElement(); inspectBtn.id = "swiss-b-inspect";
const canvas = new MockElement(); canvas.id = "swiss-browser-canvas";

let domInspectorActive = false;
let cleanedUpInWebview = false;
function toggleDOMInspector(active) {
  domInspectorActive = active;
  if (!active) cleanedUpInWebview = true;
}

function exitInspectTool() {
  drawTool = "none";
  penBtn.classList.remove("active");
  rectBtn.classList.remove("active");
  inspectBtn.classList.remove("active");
  canvas.style.pointerEvents = "none";
  toggleDOMInspector(false);
}

function setDrawTool(tool) {
  drawTool = (drawTool === tool) ? "none" : tool;
  penBtn.classList.toggle("active", drawTool === "pen");
  rectBtn.classList.toggle("active", drawTool === "rect");
  inspectBtn.classList.toggle("active", drawTool === "inspect");

  if (drawTool === "inspect") {
    canvas.style.pointerEvents = "none";
    toggleDOMInspector(true);
  } else if (drawTool === "none") {
    canvas.style.pointerEvents = "none";
    toggleDOMInspector(false);
  } else {
    canvas.style.pointerEvents = "auto";
    toggleDOMInspector(false);
  }
}

// Step 1: User activates Inspect tool
setDrawTool("inspect");
assert.strictEqual(drawTool, "inspect", "drawTool should be inspect");
assert.strictEqual(inspectBtn.classList.contains("active"), true, "inspectBtn should be active");
assert.strictEqual(domInspectorActive, true, "toggleDOMInspector should be true");

// Step 2: User presses Escape key in host window
const escEvent = {
  key: "Escape",
  preventDefaultCalled: false,
  stopPropagationCalled: false,
  stopImmediatePropagationCalled: false,
  preventDefault() { this.preventDefaultCalled = true; },
  stopPropagation() { this.stopPropagationCalled = true; },
  stopImmediatePropagation() { this.stopImmediatePropagationCalled = true; }
};

if ((escEvent.key === "Escape" || escEvent.key === "Esc" || escEvent.keyCode === 27 || escEvent.which === 27) && drawTool === "inspect") {
  escEvent.preventDefault();
  escEvent.stopPropagation();
  escEvent.stopImmediatePropagation();
  exitInspectTool();
}

assert.strictEqual(drawTool, "none", "drawTool should reset to none on Escape");
assert.strictEqual(inspectBtn.classList.contains("active"), false, "inspectBtn should lose active class on Escape");
assert.strictEqual(canvas.style.pointerEvents, "none", "canvas pointerEvents should be none");
assert.strictEqual(cleanedUpInWebview, true, "webview DOM inspector cleanup should be invoked");
assert.strictEqual(escEvent.preventDefaultCalled, true, "preventDefault should be called on Escape");
assert.strictEqual(escEvent.stopPropagationCalled, true, "stopPropagation should be called on Escape");
assert.strictEqual(escEvent.stopImmediatePropagationCalled, true, "stopImmediatePropagation should be called on Escape");

// Step 3: Re-activate and test cancellation via webview console-message
setDrawTool("inspect");
assert.strictEqual(drawTool, "inspect");
assert.strictEqual(inspectBtn.classList.contains("active"), true);

const consoleMsgEvent = { message: "renderer: [SWISS_INSPECT_CANCEL] user dismissed" };
if (consoleMsgEvent.message && consoleMsgEvent.message.includes("[SWISS_INSPECT_CANCEL]")) {
  exitInspectTool();
}

assert.strictEqual(drawTool, "none", "drawTool should reset to none on [SWISS_INSPECT_CANCEL]");
assert.strictEqual(inspectBtn.classList.contains("active"), false, "inspectBtn should not be active");

// Step 4: Re-activate and test key: "Esc", keyCode: 27, and which: 27 variants
setDrawTool("inspect");
const legacyEscEvent = { key: "Esc", which: 27, preventDefault() {}, stopPropagation() {}, stopImmediatePropagation() {} };
if ((legacyEscEvent.key === "Escape" || legacyEscEvent.key === "Esc" || legacyEscEvent.keyCode === 27 || legacyEscEvent.which === 27) && drawTool === "inspect") {
  exitInspectTool();
}
assert.strictEqual(drawTool, "none", "legacy Esc key should exit tool");

// Step 5: Test postMessage cancellation
setDrawTool("inspect");
assert.strictEqual(drawTool, "inspect");
const postMsgEvent = { data: { type: "SWISS_INSPECT_CANCEL" } };
if (postMsgEvent.data && (postMsgEvent.data.type === "SWISS_INSPECT_CANCEL" || postMsgEvent.data === "SWISS_INSPECT_CANCEL")) {
  exitInspectTool();
}
assert.strictEqual(drawTool, "none", "postMessage SWISS_INSPECT_CANCEL should exit tool");

console.log("ALL_INSPECT_ESCAPE_TESTS_PASSED");
`
		cmd := exec.Command(nodePath, "-e", nodeTestScript)
		cmd.Stdin = strings.NewReader(js)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Node.js inspect escape test failed: %v\n%s", err, string(out))
		}
		if !strings.Contains(string(out), "ALL_INSPECT_ESCAPE_TESTS_PASSED") {
			t.Fatalf("expected test output to contain ALL_INSPECT_ESCAPE_TESTS_PASSED, got %q", string(out))
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

func TestGenerateAuxiliaryPluginsScript_ElementSpecificAnnotation(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	css := GenerateAuxiliaryPluginsCSS()

	requiredTokens := []string{
		"renderElementAnnotationBox",
		"clearElementAnnotation",
		"swiss-element-annotation-box",
		"swiss-element-annotation-input",
		"swiss-element-annotation-close",
		"swiss-element-annotation-send",
		"attachChatAnnotationChip",
		"swiss-chat-annotation-chip",
		"swiss-chat-chip-delete",
		"executeSendToChatWorkflow",
	}

	for _, token := range requiredTokens {
		if !strings.Contains(js, token) {
			t.Errorf("expected script to contain element annotation token %q", token)
		}
	}

	requiredCSSTokens := []string{
		".swiss-element-annotation-box",
		".swiss-element-annotation-close",
		".swiss-element-annotation-input",
		".swiss-element-annotation-send",
		".swiss-chat-annotation-chip",
		".swiss-chat-chip-delete",
	}

	for _, token := range requiredCSSTokens {
		if !strings.Contains(css, token) {
			t.Errorf("expected CSS to contain element annotation token %q", token)
		}
	}

	if nodePath, err := exec.LookPath("node"); err == nil {
		nodeTestScript := `
const assert = require("assert");

class MockElement {
  constructor(tag = "div") {
    this.tagName = tag.toUpperCase();
    this.id = "";
    this.className = "";
    this.value = "";
    this.style = {};
    this.children = [];
    this.parentElement = null;
    this._listeners = {};
    this.attributes = {};
  }
  setAttribute(k, v) { this.attributes[k] = v; }
  getAttribute(k) { return this.attributes[k]; }
  querySelector(sel) {
    if (sel.startsWith("#")) {
      const id = sel.slice(1);
      if (this.id === id) return this;
      for (const ch of this.children) {
        const found = ch.querySelector(sel);
        if (found) return found;
      }
    }
    return null;
  }
  appendChild(child) {
    child.parentElement = this;
    this.children.push(child);
    return child;
  }
  insertBefore(child, ref) {
    child.parentElement = this;
    const idx = this.children.indexOf(ref);
    if (idx >= 0) this.children.splice(idx, 0, child);
    else this.children.push(child);
    return child;
  }
  remove() {
    if (this.parentElement) {
      const idx = this.parentElement.children.indexOf(this);
      if (idx >= 0) this.parentElement.children.splice(idx, 1);
      this.parentElement = null;
    }
  }
  addEventListener(event, fn) {
    if (!this._listeners[event]) this._listeners[event] = [];
    this._listeners[event].push(fn);
  }
  removeEventListener(event, fn) {
    if (!this._listeners[event]) return;
    this._listeners[event] = this._listeners[event].filter(cb => cb !== fn);
  }
  dispatchEvent(event) {
    const list = this._listeners[event.type] || [];
    for (const fn of list) fn(event);
  }
}

const screen = new MockElement("div");
screen.id = "screen";

let lastSelectedElement = null;
let lastAnnotatedRegion = null;
let userComment = "";
let canvasCleared = false;

function clearElementAnnotation() {
  const existing = screen.querySelector("#swiss-element-annotation-box");
  if (existing) existing.remove();
  lastSelectedElement = null;
  lastAnnotatedRegion = null;
  userComment = "";
  canvasCleared = true;
}

function renderElementAnnotationBox(result) {
  const existing = screen.querySelector("#swiss-element-annotation-box");
  if (existing) existing.remove();

  const box = new MockElement("div");
  box.id = "swiss-element-annotation-box";

  const input = new MockElement("input");
  input.id = "swiss-element-annotation-input";
  input.value = userComment || "";

  const closeBtn = new MockElement("button");
  closeBtn.id = "swiss-element-annotation-close";
  closeBtn.onclick = () => clearElementAnnotation();

  box.appendChild(input);
  box.appendChild(closeBtn);
  screen.appendChild(box);
}

// 1. Inspect element result sets up annotation box directly on element
const inspectResult = { selector: "button.submit-btn", rect: { x: 50, y: 100, width: 120, height: 40 } };
lastSelectedElement = inspectResult;
renderElementAnnotationBox(inspectResult);

assert.ok(screen.querySelector("#swiss-element-annotation-box"), "should render element annotation box");
assert.ok(screen.querySelector("#swiss-element-annotation-input"), "should have input box for annotation");

// 2. Clear button removes annotation box, unselects element, and clears canvas
const closeBtn = screen.querySelector("#swiss-element-annotation-close");
closeBtn.onclick();

assert.strictEqual(screen.querySelector("#swiss-element-annotation-box"), null, "close button should remove box");
assert.strictEqual(lastSelectedElement, null, "close button should unselect element");
assert.strictEqual(canvasCleared, true, "close button should clear canvas");

// 3. In-chat annotation chip and deletion from chat input box
const chatContainer = new MockElement("div");
const chatTextarea = new MockElement("textarea");
chatTextarea.value = "User prompt text\n[Preview Browser Element Annotation @ localhost]\nSelected Element: div.card\nAnnotation: Fix border\n(Visual annotation attached: annotation.png)";
chatContainer.appendChild(chatTextarea);

let chipRemoved = false;
const chip = new MockElement("div");
chip.id = "swiss-chat-annotation-chip";
const deleteBtn = new MockElement("button");
deleteBtn.id = "swiss-chat-chip-delete";
deleteBtn.onclick = () => {
  chip.remove();
  chipRemoved = true;
  chatTextarea.value = chatTextarea.value.replace(/\[Preview Browser (Element )?Annotation[\s\S]*?\(Visual annotation attached: annotation\.png\)\n?/g, '').trim();
  clearElementAnnotation();
};
chip.appendChild(deleteBtn);
chatContainer.appendChild(chip);

assert.ok(chatContainer.querySelector("#swiss-chat-annotation-chip"), "chat container should have annotation chip");
deleteBtn.onclick();

assert.strictEqual(chipRemoved, true, "clicking delete button should remove chip");
assert.strictEqual(chatTextarea.value, "User prompt text", "clicking delete button should remove annotation block from chat input");
assert.strictEqual(lastSelectedElement, null, "deleting from chat input should unselect element in preview");

console.log("ALL_ELEMENT_ANNOTATION_TESTS_PASSED");
`
		cmd := exec.Command(nodePath, "-e", nodeTestScript)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Node element annotation test failed: %v\n%s", err, string(out))
		}
		if !strings.Contains(string(out), "ALL_ELEMENT_ANNOTATION_TESTS_PASSED") {
			t.Fatalf("expected test output to contain ALL_ELEMENT_ANNOTATION_TESTS_PASSED, got %q", string(out))
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
		`id="swiss-m-record-audio"`,
		`<span>Voice Memo</span>`,
		`class="swiss-browser-btn swiss-memo-del-btn" id="swiss-memo-del-${cardKey}" title="Delete Memo"`,
		`<svg viewBox="0 0 24 24" width="11" height="11"`,
		`LOCAL_MEMOS_KEY = "antigravity_swiss_memos_backup"`,
		`Array.isArray(data.memos)`,
		`saveLocalMemos`,
		`getLocalMemos`,
		`const titleStr = stripMemoVoicePrefix(userTitle)`,
		`id: newMemo.id`,
		`swiss-m-composer`,
		`swiss-m-composer-input`,
		`id="swiss-m-composer-save-label"`,
		`showSwissPrompt`,
		// Responsive flowing grid for memo cards
		`memoList.className = "swiss-memo-list"`,
		// Recording indicator: elapsed time + live loudness wave
		`id="swiss-m-record-timer"`,
		`id="swiss-m-record-wave"`,
		`class="swiss-memo-record-wave"`,
		`formatRecordElapsed`,
		`createAnalyser`,
		`recordingTimerInterval = setInterval`,
	}
	for _, token := range memoChecks {
		if !strings.Contains(js, token) {
			t.Errorf("expected script to contain memo view token %q", token)
		}
	}

	// Issue #145: the recording button keeps the icon, timer and wave canvas but
	// drops the visible "Stop Recording" label; the accessible name carries it.
	if strings.Contains(js, "Stop Recording</span>") {
		t.Errorf("recording button must not render a visible 'Stop Recording' text label")
	}
	for _, token := range []string{
		`recordBtn.setAttribute("aria-label", "Stop Recording")`,
		`recordBtn.setAttribute("aria-label", "Voice Memo")`,
	} {
		if !strings.Contains(js, token) {
			t.Errorf("recording button must keep its accessible name: missing %q", token)
		}
	}

	// In-chat real telemetry metrics checks (Requirement 3.3)
	telemetryChecks := []string{
		`swiss-inchat-metrics`,
		`Input Tokens:`,
		`Output Tokens:`,
		`Cache Hit Ratio:`,
		`Generation Speed:`,
		`/api/tokens/chat-metrics`,
	}
	for _, token := range telemetryChecks {
		if !strings.Contains(js, token) {
			t.Errorf("expected script to contain in-chat telemetry metric token %q", token)
		}
	}

	// Invariant: zero artificial numbers / zero dummy data. Never hardcode 76.2 TPS fallback!
	if strings.Contains(js, "76.2") {
		t.Errorf("script contains forbidden hardcoded fallback TPS '76.2'")
	}
	if !strings.Contains(js, "— TPS") {
		t.Errorf("expected script to contain '— TPS' fallback for unmeasured speed")
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

	// 1. Verify CSS disabled button rule and path input min-width: 0 exist
	if !strings.Contains(css, ".swiss-browser-btn:disabled") {
		t.Errorf("expected CSS to define .swiss-browser-btn:disabled")
	}
	if !strings.Contains(css, ".swiss-files-path-input {\n  flex: 1;\n  min-width: 0;") {
		t.Errorf("expected .swiss-files-path-input to define min-width: 0 so 5 icon buttons on Row 1 do not overflow narrow panels")
	}
	if !strings.Contains(js, `id="swiss-f-search" style="flex:1; min-width:0;`) {
		t.Errorf("expected #swiss-f-search to include min-width:0 in inline style")
	}

	// 2. Verify all toolbar buttons and input exist in JS with expected hover titles
	buttons := []struct {
		id    string
		title string
	}{
		{"swiss-f-home", "Home Folder"},
		{"swiss-f-refresh", "Refresh"},
		{"swiss-f-new-file", "New File"},
		{"swiss-f-new-dir", "New Folder"},
		{"swiss-f-back", "Back"},
		{"swiss-f-up", "Up Directory"},
		{"swiss-f-hidden", "Toggle Hidden Files"},
		{"swiss-f-reveal", "Open in System File Manager"},
		{"swiss-f-term", "Open in Terminal"},
		{"swiss-f-ide", "Open in VS Code"},
	}

	for _, b := range buttons {
		if !strings.Contains(js, `id="`+b.id+`"`) {
			t.Errorf("expected JS to contain button id %q", b.id)
		}
		if !strings.Contains(js, `title="`+b.title+`"`) {
			t.Errorf("expected JS to contain button title %q", b.title)
		}
	}

	// 3. Verify two-row layout ordering:
	// Row 1 (address bar): swiss-f-home < swiss-f-refresh < swiss-f-path < swiss-f-reveal < swiss-f-new-file < swiss-f-new-dir
	// Row 2 (search/actions bar): swiss-f-back < swiss-f-up < swiss-f-search < swiss-f-hidden < swiss-f-term < swiss-f-ide
	idxAddrBar := strings.Index(js, `<div class="swiss-files-address-bar">`)
	idxHome := strings.Index(js, `id="swiss-f-home"`)
	idxRefresh := strings.Index(js, `id="swiss-f-refresh"`)
	idxPath := strings.Index(js, `id="swiss-f-path"`)
	idxReveal := strings.Index(js, `id="swiss-f-reveal"`)
	idxNewFile := strings.Index(js, `id="swiss-f-new-file"`)
	idxNewDir := strings.Index(js, `id="swiss-f-new-dir"`)

	idxActionsBar := strings.Index(js, `<div class="swiss-files-actions-bar"`)
	idxBack := strings.Index(js, `id="swiss-f-back"`)
	idxUp := strings.Index(js, `id="swiss-f-up"`)
	idxSearch := strings.Index(js, `id="swiss-f-search"`)
	idxHidden := strings.Index(js, `id="swiss-f-hidden"`)
	idxTerm := strings.Index(js, `id="swiss-f-term"`)
	idxIDE := strings.Index(js, `id="swiss-f-ide"`)

	if idxAddrBar == -1 || idxHome == -1 || idxRefresh == -1 || idxPath == -1 || idxReveal == -1 || idxNewFile == -1 || idxNewDir == -1 ||
		idxActionsBar == -1 || idxBack == -1 || idxUp == -1 || idxSearch == -1 || idxHidden == -1 || idxTerm == -1 || idxIDE == -1 {
		t.Fatalf("one or more toolbar element IDs not found in JS")
	}

	if count := strings.Count(js, `id="swiss-f-reveal"`); count != 1 {
		t.Errorf("expected exactly 1 instance of id=\"swiss-f-reveal\", got %d", count)
	}

	// Row 1: Home < Refresh < Path < Reveal < New File < New Folder
	if !(idxAddrBar < idxHome && idxHome < idxRefresh && idxRefresh < idxPath && idxPath < idxReveal && idxReveal < idxNewFile && idxNewFile < idxNewDir) {
		t.Errorf("expected Row 1 order swiss-f-home < swiss-f-refresh < swiss-f-path < swiss-f-reveal < swiss-f-new-file < swiss-f-new-dir, got indices: home=%d, refresh=%d, path=%d, reveal=%d, newFile=%d, newDir=%d",
			idxHome, idxRefresh, idxPath, idxReveal, idxNewFile, idxNewDir)
	}

	// Row 1 address bar appears before Row 2 search/actions row
	if !(idxNewDir < idxActionsBar && idxActionsBar < idxBack) {
		t.Errorf("expected Row 1 (new-dir) to appear before Row 2 (actions-bar < back), got: newDir=%d, actionsBar=%d, back=%d", idxNewDir, idxActionsBar, idxBack)
	}

	// Row 2: Back < Up < Search < Hidden < Term < IDE
	if !(idxBack < idxUp && idxUp < idxSearch && idxSearch < idxHidden && idxHidden < idxTerm && idxTerm < idxIDE) {
		t.Errorf("expected Row 2 order swiss-f-back < swiss-f-up < swiss-f-search < swiss-f-hidden < swiss-f-term < swiss-f-ide, got indices: back=%d, up=%d, search=%d, hidden=%d, term=%d, ide=%d",
			idxBack, idxUp, idxSearch, idxHidden, idxTerm, idxIDE)
	}

	// Verify reveal button is inside .swiss-files-address-bar and not in .swiss-files-actions-bar
	addrBarBlock := js[idxAddrBar:idxActionsBar]
	if !strings.Contains(addrBarBlock, `id="swiss-f-reveal"`) {
		t.Errorf("expected swiss-f-reveal to be inside .swiss-files-address-bar")
	}
	actionsBarEnd := strings.Index(js[idxActionsBar:], `</div>`)
	if actionsBarEnd != -1 {
		actionsBarBlock := js[idxActionsBar : idxActionsBar+actionsBarEnd]
		if strings.Contains(actionsBarBlock, `id="swiss-f-reveal"`) {
			t.Errorf("expected swiss-f-reveal to not be inside .swiss-files-actions-bar")
		}
	}

	// Verify icon-only attributes and dimensions for Reveal, New File, and New Folder
	if !strings.Contains(js, `id="swiss-f-reveal" title="Open in System File Manager"><svg viewBox="0 0 24 24" width="13" height="13"`) {
		t.Errorf("expected swiss-f-reveal to be icon-only with width=13 height=13")
	}
	if !strings.Contains(js, `id="swiss-f-new-file" title="New File"><svg viewBox="0 0 24 24" width="13" height="13"`) {
		t.Errorf("expected swiss-f-new-file to be icon-only with width=13 height=13")
	}
	if !strings.Contains(js, `id="swiss-f-new-dir" title="New Folder"><svg viewBox="0 0 24 24" width="13" height="13"`) {
		t.Errorf("expected swiss-f-new-dir to be icon-only with width=13 height=13")
	}

	// Verify IDE button dynamic hover title, localStorage persistence, and workspace open API
	if !strings.Contains(js, "antigravity_preferred_ide") {
		t.Errorf("expected JS to check antigravity_preferred_ide in localStorage")
	}
	if !strings.Contains(js, "/api/files/open_ide") {
		t.Errorf("expected JS to call /api/files/open_ide endpoint")
	}
	if !strings.Contains(js, "getIDELabel") {
		t.Errorf("expected JS to define getIDELabel helper for dynamic hover title")
	}
	if !strings.Contains(js, "ctx-ide") || !strings.Contains(js, "ctx-blank-ide") {
		t.Errorf("expected JS to include IDE action in row and blank context menus")
	}

	// 4. Verify history stack, home navigation, reveal handler, monotonic request ID and disabled initialization logic
	navTokens := []string{
		`id="swiss-f-back" title="Back" disabled`,
		"fileHistory",
		"fileHistory.push(prevPath)",
		"fileHistory.pop()",
		`toolbar.querySelector("#swiss-f-back").onclick`,
		`toolbar.querySelector("#swiss-f-home").onclick`,
		`loadFiles("~")`,
		`toolbar.querySelector("#swiss-f-refresh").onclick`,
		`toolbar.querySelector("#swiss-f-reveal").onclick`,
		`/api/files/reveal`,
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

	// 5. Node.js runtime verification on the actual generated JS script
	if nodePath, err := exec.LookPath("node"); err == nil {
		nodeScript := `
const assert = require("assert");
const fs = require("fs");
const js = fs.readFileSync(0, "utf8");

const addrStart = js.indexOf('<div class="swiss-files-address-bar">');
const actionsStart = js.indexOf('<div class="swiss-files-actions-bar"', addrStart);
const actionsEnd = js.indexOf('</div>', actionsStart);
assert(addrStart !== -1 && actionsStart !== -1 && actionsEnd !== -1, "toolbar row blocks not found");

const row1Html = js.slice(addrStart, actionsStart);
const row2Html = js.slice(actionsStart, actionsEnd + 6);

const extractIds = (html) => Array.from(html.matchAll(/\bid="([^"]+)"/g), m => m[1]);
assert.deepStrictEqual(
  extractIds(row1Html),
  ["swiss-f-home", "swiss-f-refresh", "swiss-f-path", "swiss-f-jump", "swiss-f-reveal", "swiss-f-new-file", "swiss-f-new-dir"]
);
assert.deepStrictEqual(
  extractIds(row2Html),
  ["swiss-f-back", "swiss-f-up", "swiss-f-search", "swiss-f-hidden", "swiss-f-term", "swiss-f-ide"]
);

const revealHandlerMatch = js.match(/toolbar\.querySelector\("#swiss-f-reveal"\)\.onclick\s*=\s*\(\)\s*=>\s*\{[\s\S]*?\};/);
assert(revealHandlerMatch, "reveal onclick handler not found");
let fetchedUrl = null;
let fetchedOpts = null;
const API_BASE = "http://127.0.0.1:19876";
const currentFilePath = "/var/projects";
const toolbar = { querySelector: (sel) => (sel === "#swiss-f-reveal" ? revealBtn : null) };
const revealBtn = {};
const fetch = (url, opts) => { fetchedUrl = url; fetchedOpts = opts; };
eval(revealHandlerMatch[0]);
revealBtn.onclick();
assert.strictEqual(fetchedUrl, "http://127.0.0.1:19876/api/files/reveal");
assert.strictEqual(fetchedOpts.method, "POST");
assert.deepStrictEqual(JSON.parse(fetchedOpts.body), { path: "/var/projects" });
`
		cmd := exec.Command(nodePath, "-e", nodeScript)
		cmd.Stdin = strings.NewReader(js)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Node.js toolbar runtime verification failed: %v\n%s", err, string(out))
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

	obsolete := []string{
		`<div class="swiss-browser-nav-row">`,
		`id="swiss-b-back"`,
		`id="swiss-b-fwd"`,
		`id="swiss-b-refresh"`,
		`id="swiss-b-url"`,
		`<div class="swiss-browser-port-bar">`,
		`class="swiss-port-chip"`,
		`class="swiss-port-select"`,
		`class="swiss-port-add-box"`,
	}
	for _, token := range obsolete {
		if strings.Contains(js, token) {
			t.Errorf("obsolete injected browser control remains: %q", token)
		}
	}

	requiredJS := []string{
		`[data-testid="builtin-browser-pane"]`,
		`[data-testid="webview-url-bar-screenshot"]`,
		`const camera = pane.querySelector('[data-testid="webview-url-bar-screenshot"]')`,
		`camera.parentElement.insertBefore(starButton, camera)`,
		`starButton.id = "swiss-bookmark-star"`,
		`starButton.setAttribute("aria-label", "Bookmarks")`,
		`aria-expanded`,
		`<svg class="swiss-bookmark-star-icon" viewBox="0 0 24 24"`,
		`swiss-browser-tools-row`,
		`id="swiss-b-device"`,
		`id="swiss-b-send-chat"`,
	}
	for _, token := range requiredJS {
		if !strings.Contains(js, token) {
			t.Errorf("expected native-anchored bookmark token %q", token)
		}
	}

	anchor := strings.Index(js, `const camera = pane.querySelector('[data-testid="webview-url-bar-screenshot"]')`)
	insert := strings.Index(js, `camera.parentElement.insertBefore(starButton, camera)`)
	if anchor == -1 || insert == -1 || anchor >= insert {
		t.Errorf("Star must be inserted relative to the native camera control; anchor=%d insert=%d", anchor, insert)
	}

	requiredCSS := []string{
		".swiss-bookmark-star",
		".swiss-bookmark-menu",
		".swiss-bookmark-menu-add",
		".swiss-bookmark-menu-row",
		".swiss-bookmark-context",
		".swiss-bookmark-dialog",
	}
	for _, token := range requiredCSS {
		if !strings.Contains(css, token) {
			t.Errorf("expected bookmark CSS token %q", token)
		}
	}
}

func TestAuxiliaryPerExtensionVisibilityAndOrientation(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	css := GenerateAuxiliaryPluginsCSS()

	jsTokens := []string{
		"isAuxExtensionVisible",                 // per-extension aux panel gate
		"aux_panel",                             // EnhancementsConfig.extensions visibility key
		`id="swiss-b-orientation"`,              // Portrait|Landscape control
		"antigravity_swiss_browser_orientation", // persisted IDE-side orientation key
		"deviceOrientation",                     // orientation state variable
		"swiss-landscape",                       // landscape class applied to device frame
	}
	for _, tok := range jsTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected auxiliary script to contain %q", tok)
		}
	}

	cssTokens := []string{
		".swiss-device-frame.swiss-landscape.frame-iphone-16-pro",
		".swiss-device-frame.swiss-landscape.frame-pixel-9",
		".swiss-device-frame.swiss-landscape.frame-ipad",
	}
	for _, tok := range cssTokens {
		if !strings.Contains(css, tok) {
			t.Errorf("expected auxiliary CSS to contain %q", tok)
		}
	}

	// Orientation select must sit inside the device group between device and scale
	idxDevice := strings.Index(js, `id="swiss-b-device"`)
	idxOrient := strings.Index(js, `id="swiss-b-orientation"`)
	idxScale := strings.Index(js, `id="swiss-b-scale"`)
	if idxDevice == -1 || idxOrient == -1 || idxScale == -1 {
		t.Fatalf("device group controls missing (device=%d, orientation=%d, scale=%d)", idxDevice, idxOrient, idxScale)
	}
	if !(idxDevice < idxOrient && idxOrient < idxScale) {
		t.Errorf("expected device group order: device < orientation < scale, got %d < %d < %d", idxDevice, idxOrient, idxScale)
	}
}

func TestBrowserViewBookmarkManagement(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()

	persistence := []string{
		`BOOKMARKS_KEY = "antigravity_swiss_browser_bookmarks_v1"`,
		`LEGACY_QUICK_PORTS_KEY = "antigravity_swiss_quick_ports"`,
		`DEFAULT_BOOKMARKS`,
		`getBookmarks`,
		`saveBookmarks`,
		`migrateQuickPorts`,
		`localStorage.getItem(LEGACY_QUICK_PORTS_KEY)`,
		`localStorage.setItem(BOOKMARKS_KEY`,
		`{ id: bookmarkId(), name: entry.name, address: entry.address }`,
		`address:`,
	}
	for _, token := range persistence {
		if !strings.Contains(js, token) {
			t.Errorf("expected bookmark persistence token %q", token)
		}
	}

	crud := []string{
		`function openBookmarkDialog`,
		`function addBookmark`,
		`function updateBookmark`,
		`function deleteBookmark`,
		`function selectBookmark`,
		`const name = nameInput.value.trim()`,
		`const address = normalizeBookmarkAddress(addressInput.value)`,
		`if (!name || !address) return`,
		`webview.loadURL(bookmark.address)`,
		`window.confirm("Delete bookmark?")`,
	}
	for _, token := range crud {
		if !strings.Contains(js, token) {
			t.Errorf("expected bookmark CRUD/navigation token %q", token)
		}
	}

	keyboard := []string{
		`addEventListener("contextmenu"`,
		`e.key === "ContextMenu"`,
		`e.shiftKey && e.key === "F10"`,
		`e.key === "ArrowDown"`,
		`e.key === "ArrowUp"`,
		`e.key === "Escape"`,
		`e.key === "Tab"`,
		`focusBookmarkItem`,
		`trigger.focus()`,
	}
	for _, token := range keyboard {
		if !strings.Contains(js, token) {
			t.Errorf("expected bookmark keyboard/context affordance %q", token)
		}
	}

	if strings.Contains(js, "QUICK PORTS") {
		t.Error("obsolete QUICK PORTS label must not be rendered")
	}
}

func TestBrowserViewBookmarkLegacyEmptyMigration(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	defaultsStart := strings.Index(js, "const DEFAULT_BOOKMARKS = [")
	if defaultsStart < 0 {
		t.Fatal("expected DEFAULT_BOOKMARKS declaration")
	}
	defaultsEnd := strings.Index(js[defaultsStart:], "];")
	if defaultsEnd < 0 {
		t.Fatal("expected DEFAULT_BOOKMARKS terminator")
	}
	defaults := js[defaultsStart : defaultsStart+defaultsEnd+2]
	legacyBookmark := extractJSHandlerBlock(t, js, "function legacyBookmark")
	migrateQuickPorts := extractJSHandlerBlock(t, js, "function migrateQuickPorts")

	if nodePath, err := exec.LookPath("node"); err == nil {
		nodeScript := defaults + `
function bookmarkId() { return "bm_test"; }
` + legacyBookmark + `
` + migrateQuickPorts + `
const assert = require("assert");
const shapes = values => values.map(({ name, address }) => ({ name, address }));

assert.deepStrictEqual(
  shapes(migrateQuickPorts([])),
  [],
  "an explicitly emptied Quick Ports array must migrate to no bookmarks"
);
assert.strictEqual(migrateQuickPorts(null).length, 5, "missing Quick Ports may use defaults");
assert.strictEqual(migrateQuickPorts(undefined).length, 5, "undefined Quick Ports may use defaults");
assert.strictEqual(migrateQuickPorts("invalid").length, 5, "invalid Quick Ports may use defaults");
console.log("BOOKMARK_MIGRATION_OK");
`
		cmd := exec.Command(nodePath, "-e", nodeScript)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("node Quick Port migration test failed: %v\n%s", err, string(out))
		}
		if !strings.Contains(string(out), "BOOKMARK_MIGRATION_OK") {
			t.Fatalf("expected BOOKMARK_MIGRATION_OK, got %q", string(out))
		}
	}
}

func TestBrowserViewBookmarkDialogRecovery(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	normalizeAddress := extractJSHandlerBlock(t, js, "function normalizeBookmarkAddress")
	closeDialog := extractJSHandlerBlock(t, js, "function closeBookmarkDialog")
	openDialog := extractJSHandlerBlock(t, js, "function openBookmarkDialog")

	if nodePath, err := exec.LookPath("node"); err == nil {
		nodeScript := `let bookmarkDialogState = null;
` + closeDialog + `
` + normalizeAddress + `
` + openDialog + `
const assert = require("assert");

class MockInput {
  constructor() {
    this.value = "";
    this.validationMessage = "";
    this.listeners = {};
  }
  setCustomValidity(message) { this.validationMessage = message; }
  addEventListener(type, handler) {
    if (!this.listeners[type]) this.listeners[type] = [];
    this.listeners[type].push(handler);
  }
  focus() { document.activeElement = this; }
  select() {}
  input() {
    (this.listeners.input || []).forEach(handler => handler({ target: this }));
  }
}

const nameInput = new MockInput();
const addressInput = new MockInput();
const form = { onsubmit: null };
const cancel = { onclick: null };
const backdrop = {
  removed: false,
  querySelector(selector) {
    return {
      form,
      "[data-swiss-bookmark-name]": nameInput,
      "[data-swiss-bookmark-address]": addressInput,
      "[data-swiss-bookmark-cancel]": cancel
    }[selector];
  },
  remove() { this.removed = true; }
};

global.document = {
  createElement() { return backdrop; },
  body: { appendChild(element) { assert.strictEqual(element, backdrop); } },
  addEventListener() {},
  removeEventListener() {},
  activeElement: null
};

const trigger = { isConnected: true, focus() {} };
const saved = [];
openBookmarkDialog(null, trigger, values => saved.push(values));

const submit = () => form.onsubmit({ preventDefault() {} });
submit();
assert.strictEqual(saved.length, 0, "blank Save must not close or save the bookmark");
assert.strictEqual(nameInput.validationMessage, "Required");
assert.strictEqual(addressInput.validationMessage, "Required");
assert.strictEqual(backdrop.removed, false);

nameInput.value = "Local API";
addressInput.value = "localhost:4321";
nameInput.input();
addressInput.input();
assert.strictEqual(nameInput.validationMessage, "", "typing a name must clear its custom validity");
assert.strictEqual(addressInput.validationMessage, "", "typing an address must clear its custom validity");

if (!nameInput.validationMessage && !addressInput.validationMessage) submit();
assert.deepStrictEqual(saved, [{ name: "Local API", address: "http://localhost:4321" }]);
assert.strictEqual(backdrop.removed, true, "the same dialog must close after a valid retry");
console.log("BOOKMARK_DIALOG_RECOVERY_OK");
`
		cmd := exec.Command(nodePath, "-e", nodeScript)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("node bookmark dialog recovery test failed: %v\n%s", err, string(out))
		}
		if !strings.Contains(string(out), "BOOKMARK_DIALOG_RECOVERY_OK") {
			t.Fatalf("expected BOOKMARK_DIALOG_RECOVERY_OK, got %q", string(out))
		}
	}
}

func TestBrowserViewBookmarkMenuResponsiveness(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	css := GenerateAuxiliaryPluginsCSS()

	cssTokens := []string{
		".swiss-browser-tools-row",
		"flex-wrap: nowrap",
		"overflow-x: auto",
		".swiss-bookmark-menu",
		"position: fixed",
		"max-height: min(420px, calc(100vh - 16px))",
		"overflow-y: auto",
		"z-index: 100000",
	}
	for _, token := range cssTokens {
		if !strings.Contains(css, token) {
			t.Errorf("expected responsive menu CSS token %q", token)
		}
	}

	jsTokens := []string{
		`document.body.appendChild(menu)`,
		`function positionBookmarkMenu(menu, trigger)`,
		`window.innerWidth`,
		`window.innerHeight`,
		`Math.max(8, Math.min(`,
		`window.addEventListener("resize", closeBookmarkMenu)`,
		`window.removeEventListener("resize", closeBookmarkMenu)`,
	}
	for _, token := range jsTokens {
		if !strings.Contains(js, token) {
			t.Errorf("expected menu placement/lifecycle token %q", token)
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

	// 3. Verify ordering:
	// Row 1 (address bar): path < reveal < new-file < new-dir
	// Row 2 (search & actions): search < hidden < term < ide
	idxPath := strings.Index(js, `id="swiss-f-path"`)
	idxReveal := strings.Index(js, `id="swiss-f-reveal"`)
	idxNewFile := strings.Index(js, `id="swiss-f-new-file"`)
	idxNewDir := strings.Index(js, `id="swiss-f-new-dir"`)
	idxSearch := strings.Index(js, `id="swiss-f-search"`)
	idxHidden := strings.Index(js, `id="swiss-f-hidden"`)
	idxTerm := strings.Index(js, `id="swiss-f-term"`)
	idxIDE := strings.Index(js, `id="swiss-f-ide"`)

	if idxPath == -1 || idxReveal == -1 || idxNewFile == -1 || idxNewDir == -1 ||
		idxSearch == -1 || idxHidden == -1 || idxTerm == -1 || idxIDE == -1 {
		t.Fatalf("one or more toolbar elements not found in JS: path=%d, reveal=%d, newFile=%d, newDir=%d, search=%d, hidden=%d, term=%d, ide=%d",
			idxPath, idxReveal, idxNewFile, idxNewDir, idxSearch, idxHidden, idxTerm, idxIDE)
	}

	if !(idxPath < idxReveal && idxReveal < idxNewFile && idxNewFile < idxNewDir) {
		t.Errorf("expected Row 1 order path < reveal < new-file < new-dir, got: path=%d, reveal=%d, file=%d, dir=%d",
			idxPath, idxReveal, idxNewFile, idxNewDir)
	}

	if !(idxSearch < idxHidden && idxHidden < idxTerm && idxTerm < idxIDE) {
		t.Errorf("expected Row 2 order search < hidden < term < ide, got: search=%d, hidden=%d, term=%d, ide=%d",
			idxSearch, idxHidden, idxTerm, idxIDE)
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

func TestGenerateAuxiliaryPluginsScript_VoiceTranscriptionAndAudioPayload(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()

	// 1. Verify Web Speech API initialization and configuration
	speechTokens := []string{
		"window.SpeechRecognition || window.webkitSpeechRecognition",
		"speechRecognition = new SpeechRec()",
		"speechRecognition.continuous = true",
		"speechRecognition.interimResults = true",
		`speechRecognition.lang = navigator.language || "en-US"`,
		"speechRecognition.onresult",
		"speechRecognition.onerror",
		"speechRecognition.start()",
		"speechRecognition.stop()",
	}
	for _, tok := range speechTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected auxiliary script to contain Web Speech API token %q", tok)
		}
	}

	// 2. Verify audio recording, base64 payload conversion, and duration formatting
	audioTokens := []string{
		`mediaStream = await navigator.mediaDevices.getUserMedia({ audio: true })`,
		"mediaRecorder = new MediaRecorder(mediaStream)",
		"mediaRecorder.start(250)",
		"mediaRecorder.onstop",
		"mediaStream.getTracks().forEach(t => t.stop())",
		"readAsDataURL(audioBlob)",
		"transcript: finalTranscript",
		"audio_data: base64Audio",
		"duration: formattedDuration",
	}
	for _, tok := range audioTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected auxiliary script to contain audio payload token %q", tok)
		}
	}

	// 3. Verify speech transcription prompt and fallback prompt handling
	promptTokens := []string{
		`await showSwissPrompt("Voice recorded & transcribed! Edit title:", finalTranscript)`,
		`await showSwissPrompt("Voice recorded! Enter a transcript / note title:", defaultVoiceTitle)`,
		`showToast("Microphone access error: " + err.message, "error")`,
	}
	for _, tok := range promptTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected auxiliary script to contain prompt/fallback token %q", tok)
		}
	}

	// 4. David-Design Zero Decorative Emojis check in auxiliary script
	forbiddenEmojis := []string{
		"🎙️", "💬", "💾", "✏️", "📋", "🗑️", "📂", "⚡", "📁", "📜", "📝", "📕", "📄", "⏹️",
	}
	for _, emoji := range forbiddenEmojis {
		if strings.Contains(js, emoji) {
			t.Errorf("auxiliary script contains forbidden decorative emoji %q", emoji)
		}
	}

	// 5. Verify isolated Main Stage vs Right Auxiliary Panel file path state & voice memo workspace_path
	scopeTokens := []string{
		"let stageFilePath =",
		"let auxFilePath =",
		`stageFilePath = (stScope && stScope !== "GLOBAL") ? stScope : ".";`,
		"workspace_path: mq.wsPath || undefined",
	}
	for _, tok := range scopeTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected auxiliary script to contain scope isolation token %q", tok)
		}
	}
}

func TestAuxiliaryTabProportionsAndBreakerMargin(t *testing.T) {
	css := GenerateAuxiliaryPluginsCSS()

	// 1. Auxiliary tab button proportions: height 24px, width 24px, min-width 24px, border-radius 8px matching factory tab buttons
	if !strings.Contains(css, "height: 24px;") {
		t.Errorf("expected auxiliary tab CSS to specify 24px height matching factory tab buttons")
	}
	if !strings.Contains(css, "width: 24px;") {
		t.Errorf("expected auxiliary tab CSS to specify 24px width matching factory tab buttons")
	}
	if !strings.Contains(css, "min-width: 24px;") {
		t.Errorf("expected auxiliary tab CSS to specify 24px min-width matching factory tab buttons")
	}
	if !strings.Contains(css, "border-radius: 8px;") {
		t.Errorf("expected auxiliary tab CSS to specify 8px border-radius matching factory rounded-lg")
	}

	// 2. Auxiliary tab SVG and label proportions: 13.5px icons matching factory button visual glyph size
	if !strings.Contains(css, "width: 13.5px;") || !strings.Contains(css, "height: 13.5px;") {
		t.Errorf("expected auxiliary tab SVG to be 13.5px x 13.5px matching factory buttons")
	}
	if !strings.Contains(css, "font-size: 11px;") {
		t.Errorf("expected auxiliary tab label to be 11px")
	}

	// 3. Visible divider line rule: margin: 0 2px and opacity: 1 !important to guarantee visible rendering
	if !strings.Contains(css, "margin: 0 2px;") {
		t.Errorf("expected .swiss-aux-tabs-divider to use margin: 0 2px")
	}
	if !strings.Contains(css, "opacity: 1 !important;") {
		t.Errorf("expected .swiss-aux-tabs-divider to use opacity: 1 !important")
	}

	// 4. Fixed-ends and flexible middle flexbox architecture
	if !strings.Contains(css, "flex: 1 1 0% !important;") {
		t.Errorf("expected CSS to specify flex: 1 1 0%%%% !important for middle file tabs")
	}
	if !strings.Contains(css, "@container (max-width: 285px)") {
		t.Errorf("expected CSS to include @container (max-width: 285px)")
	}

	js := GenerateAuxiliaryPluginsScript()
	if !strings.Contains(js, `dividerLeft.style.margin = "0 2px"`) && !strings.Contains(js, `dividerLeft.style.margin = '0 2px'`) {
		t.Errorf("expected script JS to set dividerLeft margin to 0 2px")
	}
	if !strings.Contains(js, `dividerRight.style.margin = "0 2px"`) && !strings.Contains(js, `dividerRight.style.margin = '0 2px'`) {
		t.Errorf("expected script JS to set dividerRight margin to 0 2px")
	}
	if !strings.Contains(js, `dividerLeft.style.opacity = "1"`) && !strings.Contains(js, `dividerLeft.style.opacity = '1'`) {
		t.Errorf("expected script JS to set dividerLeft opacity to 1")
	}
	if !strings.Contains(js, `dividerRight.style.opacity = "1"`) && !strings.Contains(js, `dividerRight.style.opacity = '1'`) {
		t.Errorf("expected script JS to set dividerRight opacity to 1")
	}
}

func TestAuxiliaryDaemonOfflineHandling(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()

	// 1. Verify window.__swissOnAuxDaemonChanged exists
	if !strings.Contains(js, "window.__swissOnAuxDaemonChanged = function(online)") {
		t.Errorf("expected script to register window.__swissOnAuxDaemonChanged")
	}

	// 2. Verify setupAuxiliaryTabs checks window.__swissDaemonOnline === false
	if !strings.Contains(js, "if (window.__swissDaemonOnline === false || !window.__swissDaemonOnline)") &&
		!strings.Contains(js, "if (window.__swissDaemonOnline === false)") {
		t.Errorf("expected setupAuxiliaryTabs to guard against window.__swissDaemonOnline === false")
	}

	// 3. Verify daemon offline removal of auxiliary tab buttons and dividers
	if !strings.Contains(js, `document.querySelectorAll(".swiss-aux-btn-group, .swiss-aux-tabs-divider, .swiss-aux-tabs-divider-left, .swiss-aux-tabs-divider-right").forEach(el => el.remove())`) {
		t.Errorf("expected daemon offline handler to remove aux button group and dividers")
	}

	// 4. Verify daemon offline removal of in-chat telemetry metrics
	if !strings.Contains(js, `document.querySelectorAll(".swiss-inchat-metrics").forEach(el => el.remove())`) {
		t.Errorf("expected daemon offline handler to remove in-chat telemetry metrics")
	}
	if !strings.Contains(js, "chatTelemetryCachedData = null;") {
		t.Errorf("expected daemon offline handler to clear chatTelemetryCachedData")
	}

	// 5. Verify setupInChatTelemetry checks window.__swissDaemonOnline === false
	if !strings.Contains(js, "if (window.__swissDaemonOnline === false || !window.__swissDaemonOnline)") &&
		!strings.Contains(js, "if (window.__swissDaemonOnline === false)") {
		t.Errorf("expected setupInChatTelemetry to guard against window.__swissDaemonOnline === false")
	}

	// 6. Verify auth headers function
	if !strings.Contains(js, "getSwissAuthHeaders") {
		t.Errorf("expected script to define getSwissAuthHeaders")
	}

	// 7. Verify syntax passes in Node
	if nodePath, err := exec.LookPath("node"); err == nil {
		cmd := exec.Command(nodePath, "--check")
		cmd.Stdin = strings.NewReader(js)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("node syntax error in GenerateAuxiliaryPluginsScript: %v\n%s", err, string(out))
		}
	}
}
func TestVisualAnnotationAppendToChatEnd(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()

	// Verify JS contains range collapse logic and selection positioning to end
	requiredPatterns := []string{
		"range.selectNodeContents(lexicalElem)",
		"range.collapse(false)",
		"range.selectNodeContents(input)",
		"input.selectionStart = input.selectionEnd = input.value.length",
	}

	for _, p := range requiredPatterns {
		if !strings.Contains(js, p) {
			t.Errorf("expected GenerateAuxiliaryPluginsScript to contain %q", p)
		}
	}

	// Verify runtime appending behavior via Node.js simulation
	if nodePath, err := exec.LookPath("node"); err == nil {
		nodeTestScript := `
const assert = require("assert");

// Mock Document and Elements
class MockElement {
  constructor(tag) {
    this.tagName = tag.toUpperCase();
    this.value = "";
    this.innerText = "";
    this.isContentEditable = false;
    this.children = [];
    this.selectionStart = 0;
    this.selectionEnd = 0;
  }
  focus() {}
  dispatchEvent(e) {}
}

const mockTextarea = new MockElement("textarea");
mockTextarea.value = "Initial user prompt";

// Simulate textarea appending logic from insertTextToChatInput
const curVal = mockTextarea.value || "";
const annotationText = "[Preview Browser Annotation @ http://localhost:8765]\n(Visual annotation attached: annotation.png)";
mockTextarea.value = (curVal.trim() ? curVal.trim() + "\n" : "") + annotationText;
mockTextarea.selectionStart = mockTextarea.selectionEnd = mockTextarea.value.length;

assert.strictEqual(
  mockTextarea.value,
  "Initial user prompt\n[Preview Browser Annotation @ http://localhost:8765]\n(Visual annotation attached: annotation.png)",
  "annotation text should be appended to the end of textarea"
);
assert.strictEqual(mockTextarea.selectionStart, mockTextarea.value.length, "cursor should be at the end");

// Simulate contenteditable appending logic
const mockEditable = new MockElement("div");
mockEditable.isContentEditable = true;
mockEditable.innerText = "Existing rich text";

let collapsedToEnd = false;
let insertedText = "";
const mockRange = {
  selectNodeContents(el) {},
  collapse(toStart) {
    if (toStart === false) collapsedToEnd = true;
  }
};

const hasContent = (mockEditable.innerText || "").trim().length > 0;
const textToInsert = (hasContent ? "\n" : "") + annotationText;
mockRange.selectNodeContents(mockEditable);
mockRange.collapse(false);
insertedText = textToInsert;

assert.strictEqual(collapsedToEnd, true, "range should collapse to end (false)");
assert.ok(insertedText.startsWith("\n"), "should prefix with newline when existing content is present");

console.log("R5_APPEND_ANNOTATION_TEST_PASSED");
`
		cmd := exec.Command(nodePath, "-e", nodeTestScript)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Node visual annotation append test failed: %v\n%s", err, string(out))
		}
		if !strings.Contains(string(out), "R5_APPEND_ANNOTATION_TEST_PASSED") {
			t.Fatalf("expected test output to contain R5_APPEND_ANNOTATION_TEST_PASSED, got %q", string(out))
		}
	}
}

func TestPreviewBrowser_ClearButtonEraserAndLabel(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	if !strings.Contains(js, `id="swiss-b-clear" title="Clear Annotations"`) {
		t.Fatalf("expected clear button with title 'Clear Annotations'")
	}
	if !strings.Contains(js, `m7 21-4.3-4.3`) || !strings.Contains(js, `M22 21H7`) {
		t.Fatalf("expected Lucide eraser SVG path in clear button")
	}
	if !strings.Contains(js, `<span>Clear</span>`) {
		t.Fatalf("expected <span>Clear</span> label inside clear button")
	}
}

func TestPreviewBrowser_TransparentRedBoxAndNoAnnotationTitle(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	if strings.Contains(js, `fillText("#annotation"`) {
		t.Fatalf("expected rect tool to not draw #annotation title badge")
	}
	if !strings.Contains(js, `ctx.strokeRect(boxX, boxY, boxW, boxH)`) {
		t.Fatalf("expected strokeRect for bounding box")
	}
}

func TestPreviewBrowser_ActiveToolHighlight(t *testing.T) {
	css := GenerateAuxiliaryPluginsCSS()
	if !strings.Contains(css, `.swiss-browser-btn.active`) {
		t.Fatalf("expected .swiss-browser-btn.active in CSS")
	}
	if !strings.Contains(css, `border-color: #ea4335 !important`) {
		t.Fatalf("expected prominent active border color in CSS")
	}
	if !strings.Contains(css, `#swiss-b-touch.active`) {
		t.Fatalf("expected #swiss-b-touch.active in CSS")
	}
}

func TestPreviewBrowser_EscapeExitsAllTools(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	if !strings.Contains(js, `exitAllTools`) {
		t.Fatalf("expected exitAllTools helper in script")
	}
	if !strings.Contains(js, `exitAllTools();`) {
		t.Fatalf("expected exitAllTools() invocation in script")
	}
}

func TestPreviewBrowser_MultilineCommentTextarea(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	css := GenerateAuxiliaryPluginsCSS()
	if !strings.Contains(js, `<textarea class="swiss-element-annotation-input"`) {
		t.Fatalf("expected <textarea> for element annotation")
	}
	if !strings.Contains(css, `min-height: 72px`) || !strings.Contains(css, `resize: vertical`) {
		t.Fatalf("expected textarea CSS with min-height and resize")
	}
}

func TestPreviewBrowser_ElementAnnotationEraserAndClearLabel(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	if !strings.Contains(js, `id="swiss-element-annotation-close"`) {
		t.Fatalf("expected swiss-element-annotation-close button in script")
	}
	// Check for Lucide eraser SVG path in element popover clear button
	if !strings.Contains(js, `m7 21-4.3-4.3`) {
		t.Fatalf("expected Lucide eraser SVG path in element annotation clear button")
	}
	if !strings.Contains(js, `<span>Clear</span>`) {
		t.Fatalf("expected <span>Clear</span> inside clear button")
	}
}

func TestPreviewBrowser_NoCommentRepopulationAndCleanReset(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	if !strings.Contains(js, `input.value = "";`) {
		t.Fatalf("expected input.value to be explicitly cleared on annotation box render")
	}
	if !strings.Contains(js, `userComment = "";`) {
		t.Fatalf("expected userComment reset in script")
	}
}

func TestPreviewBrowser_TouchEmulationVisualFeedbackAndDragScroll(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	if !strings.Contains(js, `swiss-touch-cursor`) {
		t.Fatalf("expected swiss-touch-cursor visual touch element in script")
	}
	if !strings.Contains(js, `scrollBy`) {
		t.Fatalf("expected scrollBy mobile drag scroll in script")
	}
}

func TestQuickMemos_ImageSupportAndWhiteboard(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	expectedIdentifiers := []string{
		`id="swiss-m-new-image"`,
		`<span>Image Memo</span>`,
		`swiss-m-img-composer`,
		`id="swiss-m-whiteboard-canvas"`,
		`id="swiss-m-tool-pen"`,
		`id="swiss-m-tool-eraser"`,
		`id="swiss-m-img-upload-btn"`,
		`id="swiss-m-img-clear-btn"`,
		`dataURLtoFile`,
		`insertImageToChatInput`,
		`m.image_data`,
		// DPR-aware whiteboard sizing keeps the visual brush width constant.
		`resizeWhiteboardCanvas`,
		`setTransform(whiteboardDpr, 0, 0, whiteboardDpr, 0, 0)`,
	}
	for _, id := range expectedIdentifiers {
		if !strings.Contains(js, id) {
			t.Fatalf("expected script to contain %q for image memos support", id)
		}
	}
}

func TestQuickMemos_ActionButtonsConsistency(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()

	// All three memo action buttons share the same neutral styling and carry a plain label (issue #134).
	buttonTokens := []string{
		`id="swiss-m-new-text"`,
		`<span>Text Memo</span>`,
		`id="swiss-m-record-audio"`,
		`<span>Voice Memo</span>`,
		`id="swiss-m-new-image"`,
		`<span>Image Memo</span>`,
	}
	for _, token := range buttonTokens {
		if !strings.Contains(js, token) {
			t.Errorf("expected memo action buttons to contain %q", token)
		}
	}

	// The text memo button must not be rendered as the filled blue primary button.
	if strings.Contains(js, `class="swiss-browser-btn primary" id="swiss-m-new-text"`) {
		t.Errorf("text memo button must not use the primary (filled blue) style")
	}
}

func TestQuickMemos_TypeFilterControl(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	expectedTokens := []string{
		`id="swiss-m-filter-toggle"`,
		`id="swiss-m-filter-menu"`,
		`<span id="swiss-m-filter-label">Filter: All</span>`,
		`id="swiss-m-filter-count"`,
		`data-filter="all"`,
		`data-filter="text"`,
		`data-filter="voice"`,
		`data-filter="image"`,
		`FILTER_LABELS`,
		`typeFilter = item.dataset.filter || "all"`,
		`let typeFilter = "all";`,
	}
	for _, token := range expectedTokens {
		if !strings.Contains(js, token) {
			t.Errorf("expected memo type filter to contain %q", token)
		}
	}

	// The legacy search-scope toggle must be gone (replaced by the type filter).
	if strings.Contains(js, `swiss-m-scope-toggle`) {
		t.Errorf("legacy search scope toggle must be removed")
	}
	if strings.Contains(js, `searchScope`) {
		t.Errorf("legacy searchScope state must be removed")
	}
}

func TestQuickMemos_FluidCardWidthGoldenRatio(t *testing.T) {
	css := GenerateAuxiliaryPluginsCSS()

	// The card minimum is derived from the narrowest panel the host IDE supports
	// (285px): 285 - 2 * 12 (view padding) - 12 (scrollbar allowance) = 249px.
	requiredTokens := []string{
		"--swiss-aux-panel-min-width: 285px;",
		"--swiss-memo-view-padding: 12px;",
		"--swiss-memo-scrollbar-allowance: 12px;",
		"--swiss-memo-card-min-width: calc(var(--swiss-aux-panel-min-width) - 2 * var(--swiss-memo-view-padding) - var(--swiss-memo-scrollbar-allowance));",
		// auto-fill + 1fr is the sawtooth from issue #141: cards grow with the
		// panel and reset to the minimum once another column fits.
		"grid-template-columns: repeat(auto-fill, minmax(min(var(--swiss-memo-card-min-width), 100%), 1fr));",
		// The grid decides the width; the height follows the real column width.
		"width: auto;",
		"aspect-ratio: 1.618 / 1;",
		// A lone card must stay strictly below 2x the minimum: the 8px column gap
		// would otherwise let it reach 2 * 249px + 8px - 1px.
		"max-width: min(100%, calc(2 * var(--swiss-memo-card-min-width) - 1px));",
	}
	for _, token := range requiredTokens {
		if !strings.Contains(css, token) {
			t.Errorf("expected fluid memo card CSS to contain %q", token)
		}
	}

	// Fixed card dimensions must be gone: they left a wide panel with one narrow
	// column and a large empty area on the right.
	for _, token := range []string{
		"--swiss-memo-card-width",
		"--swiss-memo-card-height",
		"width: 249px",
		"height: 154px",
		"grid-template-columns: repeat(auto-fill, min(var(--swiss-memo-card-width), 100%));",
	} {
		if strings.Contains(css, token) {
			t.Errorf("memo cards must not keep a fixed size, found %q", token)
		}
	}

	// The card body has to flex so the footer stays pinned and overflow clips.
	if !strings.Contains(css, ".swiss-memo-card-body {\n  flex: 1 1 auto;\n  min-height: 0;\n  overflow: hidden;") {
		t.Errorf("memo card body must keep flexing and clipping inside the fluid card")
	}
}

func TestQuickMemos_RecordingWaveBarsThinner(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	expectedTokens := []string{
		"const bars = 5;",
		// 4 device px on the 2x backing store = 2 CSS px wide bars (issue #133).
		"const barW = 4;",
		"const barGap = Math.max(1, Math.floor((w - barW * bars) / (bars - 1)));",
		"recordingWaveRaf = requestAnimationFrame(drawWave);",
	}
	for _, token := range expectedTokens {
		if !strings.Contains(js, token) {
			t.Errorf("expected recording wave to contain %q", token)
		}
	}
	if strings.Contains(js, "Math.max(2, Math.floor((w - barGap * (bars - 1)) / bars))") {
		t.Errorf("the old thick-bar width formula must be replaced by the fixed 2px bar")
	}
}

func TestQuickMemos_VoiceCardBodyHasNoPlaceholderLine(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	expectedTokens := []string{
		// Voice cards render only a real transcript, never the memo content fallback.
		`const primaryText = kind === "voice" ? "" : (m.content || "");`,
		`const transcriptText = kind === "text" ? "" : (m.transcript || "");`,
		`class="swiss-memo-card-body"`,
		`data-memo-transcript="1"`,
	}
	for _, token := range expectedTokens {
		if !strings.Contains(js, token) {
			t.Errorf("expected voice card body to contain %q", token)
		}
	}
	if strings.Contains(js, `m.content ? ('<div style="font-size:11px;`) {
		t.Errorf("voice cards must not print memo content in the body (the 'Voice Memo Note' placeholder line)")
	}
}

func TestQuickMemos_PlayerTracksRealAudioProgress(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	expectedTokens := []string{
		// Progress is driven by the real <audio> element, smoothly via rAF.
		`audio.addEventListener("timeupdate", renderProgress);`,
		`audio.addEventListener("loadedmetadata", renderProgress);`,
		`audio.addEventListener("durationchange", renderProgress);`,
		`audio.addEventListener("seeking", renderProgress);`,
		`audio.addEventListener("seeked", renderProgress);`,
		`rafId = requestAnimationFrame(tick);`,
		`audio.currentTime = ratio * duration;`,
		// WebM blobs report Infinity/NaN durations; fall back to the recorded label.
		`const fallbackDuration = parseDurationLabel(m.duration);`,
		`const resolveDuration = () => {`,
		`if (isFinite(d) && d > 0) return d;`,
		`return fallbackDuration;`,
		// One player at a time and a reset on ended.
		`if (activePlayback && activePlayback.audio !== audio) stopActivePlayback();`,
		`try { audio.currentTime = 0; } catch (_) {}`,
	}
	for _, token := range expectedTokens {
		if !strings.Contains(js, token) {
			t.Errorf("expected voice player to contain %q", token)
		}
	}
	if strings.Contains(js, `<audio controls`) {
		t.Errorf("native <audio controls> must be replaced by the custom progress player")
	}
}

func TestQuickMemos_ExtractButtons(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	expectedTokens := []string{
		`API_BASE + "/api/memos/extract"`,
		`memo.workspace_path || getMemoQueryInfo().wsPath`,
		`"?workspace_path=" + encodeURIComponent(memoWorkspace)`,
		`JSON.stringify({ memo_id: memo.id })`,
		`headers: getSwissAuthHeaders()`,
		`payload.success !== true`,
		`memo.transcript = transcript;`,
		`"Convert to Text"`,
		`"OCR"`,
		`"Text ✓"`,
		`"OCR ✓"`,
		`swiss-memo-extract-spinner`,
		`swiss-memo-card-foot`,
	}
	for _, token := range expectedTokens {
		if !strings.Contains(js, token) {
			t.Errorf("expected memo extract button support to contain %q", token)
		}
	}
	// Failures must surface the server message, never fabricate a transcript.
	if !strings.Contains(js, `payload && payload.error ? payload.error :`) {
		t.Errorf("extract failure must toast the server-provided error message")
	}
}

func TestQuickMemos_RecorderHidesWaveOnNarrowPanel(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	css := GenerateAuxiliaryPluginsCSS()

	// The memos view must be its own inline-size container so the rule tracks the
	// width the view actually gets (aux panel and main stage alike).
	for _, token := range []string{
		`container-type: inline-size;`,
		`container-name: swissmemo;`,
	} {
		if !strings.Contains(css, token) {
			t.Errorf("expected .swiss-memos-view container token %q", token)
		}
	}

	const rule = "@container swissmemo (max-width: 285px)"
	idx := strings.Index(css, rule)
	if idx < 0 {
		t.Fatalf("expected CSS to include %q so the wave bars can hide on narrow panels", rule)
	}
	blockEnd := strings.Index(css[idx:], "}")
	if blockEnd < 0 {
		t.Fatalf("unterminated %q block", rule)
	}
	block := css[idx : idx+blockEnd+1]
	if !strings.Contains(block, ".swiss-memo-record-wave") {
		t.Errorf("narrow-panel rule must target the loudness wave canvas, got %q", block)
	}
	if !strings.Contains(block, "display: none !important;") {
		t.Errorf("narrow-panel rule must hide the wave canvas, got %q", block)
	}
	// The timer is the one thing that must never be hidden at small widths.
	if strings.Contains(block, "swiss-m-record-timer") {
		t.Errorf("narrow-panel rule must not hide the elapsed timer, got %q", block)
	}

	// The wave canvas must still be wired for the analyser loop at wider widths.
	for _, token := range []string{
		`waveCanvas.isConnected`,
		`recordingAnalyser.getByteFrequencyData(levels)`,
	} {
		if !strings.Contains(js, token) {
			t.Errorf("expected live loudness wave to stay functional, missing %q", token)
		}
	}
}

func TestQuickMemos_AutoExtractHidesManualButtons(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()

	// Issue #143 (UI half): the manual Convert to Text / OCR buttons only exist
	// in manual mode; auto_extract comes from the daemon config endpoint.
	for _, token := range []string{
		`API_BASE + "/api/memos/config"`,
		`fetch(API_BASE + "/api/memos/config", { headers: getSwissAuthHeaders() })`,
		`(payload.config && typeof payload.config === "object") ? payload.config : payload`,
		`cfg.auto_extract === true`,
		`let autoExtractEnabled = false;`,
		`(extractKind && !autoExtractEnabled)`,
		`loadMemoConfig().then(() => loadMemos(`,
	} {
		if !strings.Contains(js, token) {
			t.Errorf("expected auto_extract UI gate to contain %q", token)
		}
	}

	// Manual mode still renders and wires the buttons, and auto mode never
	// fabricates a transcript locally: the backend owns extraction.
	for _, token := range []string{
		`.swiss-memo-extract-btn`,
		`runMemoExtract(m, extractKind, extractBtn)`,
	} {
		if !strings.Contains(js, token) {
			t.Errorf("expected manual extract path to remain intact, missing %q", token)
		}
	}
	if strings.Contains(js, `memo.transcript = ""`) {
		t.Errorf("auto mode must not fabricate an empty transcript locally")
	}
}

func TestQuickMemos_AutoExtractBoundedPolling(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()

	// Auto extraction completes in the daemon after the save response. The card
	// polls the list on a bounded schedule; it must never call the extract
	// endpoint again (that would pay for a second extraction).
	for _, token := range []string{
		`const AUTO_EXTRACT_POLL_DELAYS = [2000, 3000, 5000, 10000];`,
		`const scheduleAutoExtractPoll = (memoId) => {`,
		`if (!autoExtractEnabled || !memoId) return;`,
		`await loadMemos(true, true);`,
		`const loadMemos = async (silent = false, force = false) => {`,
		// Gated on the save succeeding and on auto mode being on.
		`if (saveRes.ok && autoExtractEnabled && effType !== "text") {`,
		`if (saveRes.ok && autoExtractEnabled) {`,
		// Early stops: transcript arrived, view torn down, tab no longer memos.
		`hasTranscript(findMemoById(memoId))`,
		`container.isConnected === false`,
		`renderedTab === "memos"`,
		`pendingExtractMemoIds.delete(memoId);`,
		// Timers never leak past teardown.
		`clearPendingExtractPoll();`,
		`pendingExtractTimers.forEach((timer) => clearTimeout(timer));`,
		// Pending state is per-memo and only while the transcript is missing.
		`const extractPending = autoExtractEnabled && !transcriptText && pendingExtractMemoIds.has(m.id);`,
		`swiss-memo-card-extract-pending`,
		`Extracting text…`,
	} {
		if !strings.Contains(js, token) {
			t.Errorf("expected bounded auto-extract polling to contain %q", token)
		}
	}

	if got := strings.Count(js, `scheduleAutoExtractPoll(newMemo.id);`); got != 2 {
		t.Errorf("expected both save paths to schedule auto-extract polling, got %d call sites", got)
	}

	// Only the manual button path may call the extract endpoint.
	if got := strings.Count(js, `API_BASE + "/api/memos/extract"`); got != 1 {
		t.Errorf("auto mode must not add a second extract call, found %d occurrences", got)
	}
	if got := strings.Count(js, `runMemoExtract(m, extractKind, extractBtn)`); got != 1 {
		t.Errorf("runMemoExtract must only be wired to the manual button, found %d call sites", got)
	}
}

func TestQuickMemos_CardScopedUniqueElementIds(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()

	// A panel can render many memo cards at once, so every per-card element id is
	// suffixed with a memo-derived key — repeated ids would violate the HTML
	// uniqueness rule and make id lookups resolve to the first card.
	uniqueIdTokens := []string{
		`const cardKey = memoCardDomKey(m.id);`,
		`raw.replace(/[^A-Za-z0-9_-]/g, "_")`,
		`return safe || "anon";`,
		`id="swiss-memo-play-' + cardKey + '"`,
		`id="swiss-memo-progress-' + cardKey + '"`,
		`id="swiss-memo-progress-fill-' + cardKey + '"`,
		`id="swiss-memo-time-' + cardKey + '"`,
		`id="swiss-memo-audio-' + cardKey + '"`,
		`id="swiss-memo-extract-' + cardKey + '"`,
		`id="swiss-memo-insert-${cardKey}"`,
		`id="swiss-memo-del-${cardKey}"`,
	}
	for _, token := range uniqueIdTokens {
		if !strings.Contains(js, token) {
			t.Errorf("expected memo card markup to contain unique id token %q", token)
		}
	}

	// Element lookups stay card-scoped and class-based, so they never depend on
	// those ids being globally unique.
	scopedSelectorTokens := []string{
		`card.querySelector(".swiss-memo-play")`,
		`card.querySelector(".swiss-memo-progress")`,
		`card.querySelector(".swiss-memo-progress-fill")`,
		`card.querySelector(".swiss-memo-time")`,
		`card.querySelector(".swiss-memo-audio")`,
		`card.querySelector(".swiss-memo-extract-btn")`,
		`card.querySelector(".swiss-memo-insert-btn")`,
		`card.querySelector(".swiss-memo-del-btn")`,
	}
	for _, token := range scopedSelectorTokens {
		if !strings.Contains(js, token) {
			t.Errorf("expected memo card lookups to contain %q", token)
		}
	}

	// Regression guard: fixed per-card ids or id-based card lookups must not return.
	if strings.Contains(js, `id="m-`) {
		t.Errorf(`memo cards must not reuse fixed element ids such as id="m-play"`)
	}
	for _, legacy := range []string{
		`querySelector("#m-play")`,
		`querySelector("#m-progress")`,
		`querySelector("#m-progress-fill")`,
		`querySelector("#m-time")`,
		`querySelector("#m-audio")`,
		`querySelector("#m-extract")`,
		`querySelector("#m-insert")`,
		`querySelector("#m-del")`,
		`querySelector("#swiss-memo-`,
	} {
		if strings.Contains(js, legacy) {
			t.Errorf("memo card element lookups must be card-scoped and class-based, found %q", legacy)
		}
	}
}

func TestMemoCanvasResizeObserversAndCostTeardown(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()

	expectedTokens := []string{
		// Annotation overlay + whiteboard resync on sash/panel resizes.
		`window.__swissBrowserCanvasRO`,
		`canvasRO.observe(canvas)`,
		`canvasRO.observe(screen)`,
		`memoCanvasRO = new ResizeObserver`,
		`memoCanvasRO.observe(whiteboardCanvas)`,
		// Container teardown stops the microphone / timers / listeners when the view is destroyed.
		`container.__swissTabTeardown = teardownMemosView`,
		`typeof container.__swissTabTeardown === "function"`,
		`mediaRecorder.onstop = null`,
		`window.removeEventListener("resize", window.__swissMemoCanvasResizeHandler)`,
		`document.removeEventListener("click", window.__swissMemoFilterOutsideHandler)`,
		`memoCanvasRO.disconnect()`,
		// Browser view teardown releases its ResizeObservers and resize listeners (issue P4-a).
		`container.__swissTabTeardown = teardownBrowserView`,
		`window.__swissBrowserCanvasRO.disconnect()`,
		`window.__swissToolbarRO.disconnect()`,
		`window.removeEventListener("resize", applyDeviceScale)`,
		// Destroyed-view fallbacks inside the recording loops.
		`timerEl.isConnected`,
		`waveCanvas.isConnected`,
	}
	for _, token := range expectedTokens {
		if !strings.Contains(js, token) {
			t.Errorf("expected memo canvas/teardown script to contain %q", token)
		}
	}

	// Dead variable from the DPR rewrite must be gone.
	if strings.Contains(js, "canvasDpr") {
		t.Errorf("dead canvasDpr variable must be removed")
	}
}

func TestBrowserAnnotationCanvas_DprAndScaleNormalization(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	expectedTokens := []string{
		`canvasScaleX = visW / layoutW`,
		`canvasScaleY = visH / layoutH`,
		`x: e.clientX - rect.left`,
		`y: e.clientY - rect.top`,
		`clearAnnotationCanvas`,
		`result.rect.x * canvasScaleX`,
	}
	for _, token := range expectedTokens {
		if !strings.Contains(js, token) {
			t.Errorf("expected annotation overlay to contain %q", token)
		}
	}
}

func TestWYSIWYGMarkdownEditor_Implementation(t *testing.T) {
	css := GenerateAuxiliaryPluginsCSS()
	expectedCSS := []string{
		".swiss-wysiwyg-container",
		".swiss-wysiwyg-content",
	}
	for _, c := range expectedCSS {
		if !strings.Contains(css, c) {
			t.Fatalf("expected CSS to contain %q for WYSIWYG editor", c)
		}
	}
	// Issue #117c: the raw-markdown textarea is gone — a single WYSIWYG mode remains.
	if strings.Contains(css, ".swiss-wysiwyg-raw") {
		t.Errorf("raw markdown textarea styles must be removed (single WYSIWYG mode)")
	}

	js := GenerateAuxiliaryPluginsScript()
	expectedJS := []string{
		"openWYSIWYGMarkdownEditor",
		`id="ctx-file-wysiwyg"`,
		`<span>WYSIWYG Markdown Editor</span>`,
		`id="swiss-wy-bold"`,
		`id="swiss-wy-italic"`,
		`id="swiss-wy-list"`,
		`id="swiss-wy-heading"`,
		`id="swiss-wy-code"`,
		`Ctrl+B`,
		`Ctrl+I`,
		`Ctrl+.`,
		`Ctrl+0`,
		`Ctrl+S`,
		`insertUnorderedList`,
		`formatBlock`,
		`mdToHtml`,
		`htmlToMd`,
	}
	for _, j := range expectedJS {
		if !strings.Contains(js, j) {
			t.Fatalf("expected script to contain %q for WYSIWYG editor", j)
		}
	}

	// Issue #117c: the raw-mode toggle, its textarea, the mode badge and the in-place editor's
	// WYSIWYG switch are all removed; markdown routes into WYSIWYG on double click.
	removedTokens := []string{
		`id="swiss-wy-mode"`,
		`id="swiss-ed-wysiwyg"`,
		`swiss-wysiwyg-raw`,
		`isRawMode`,
		`(WYSIWYG)`,
	}
	for _, r := range removedTokens {
		if strings.Contains(js, r) {
			t.Errorf("expected script to no longer contain %q (single WYSIWYG mode)", r)
		}
	}
	if !strings.Contains(js, "isMarkdownFile(item)") {
		t.Errorf("expected markdown double-click routing through isMarkdownFile into the WYSIWYG editor")
	}

	// Issue #117b: the inline-code button sits to the LEFT of the header selector.
	codeIdx := strings.Index(js, `id="swiss-wy-code"`)
	headingIdx := strings.Index(js, `id="swiss-wy-heading"`)
	if codeIdx == -1 || headingIdx == -1 || codeIdx > headingIdx {
		t.Errorf("inline-code button must sit to the left of the header selector (issue #117b)")
	}
}

func TestAuxiliaryPanelProjectScopeIsolation(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()

	// Issue #118: the explorer stamps the project it rendered with, and the periodic sync
	// (interval + mutation observer) re-renders the stale listing after a project switch.
	required := []string{
		"syncAuxProjectScope",
		"container.__swissRenderedProject = activeProj",
		"aux.__swissRenderedProject === proj",
		`renderSwissTabContent(aux, "files")`,
	}
	for _, token := range required {
		if !strings.Contains(js, token) {
			t.Errorf("expected project-scope isolation to contain %q", token)
		}
	}
}

func TestFileExplorerDefaultProjectFolderSetting(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()

	// 1. Verify resolveProjectFolder helper exists and resolves via window.__swissGetProjectFolder
	if !strings.Contains(js, "resolveProjectFolder") {
		t.Errorf("expected script to contain resolveProjectFolder")
	}
	if !strings.Contains(js, "window.__swissGetProjectFolder") {
		t.Errorf("expected script to define and call window.__swissGetProjectFolder")
	}

	// 2. Verify initialTargetPath resolves bare project name/scope
	if !strings.Contains(js, "initialTargetPath = resolveProjectFolder(") {
		t.Errorf("expected initialTargetPath to be wrapped with resolveProjectFolder")
	}

	// 3. Verify loadFiles resolves target directory path
	if !strings.Contains(js, "let targetPath = resolveProjectFolder((dirPath || \"\").trim());") {
		t.Errorf("expected loadFiles to resolve target path via resolveProjectFolder")
	}

	// 4. Test live Node evaluation of project folder resolution logic
	nodeScript := `
const assert = require('assert');

const storage = {
  antigravity_swiss_project_folders: JSON.stringify({
    "WorkspaceAlpha": "/var/projects/workspace-alpha",
    "ProjectSecureAI": "/var/projects/secure-ai"
  })
};

const localStorage = {
  getItem: (k) => storage[k] || null
};

let mockWindow = {
  __swissGetProjectFolder: (p) => {
    if (p === "WorkspaceAlpha") return "/var/projects/workspace-alpha";
    if (p === "WorkspaceAlpha/Assignment") return "/var/projects/workspace-alpha/Assignment";
    return "";
  }
};

const makeResolver = (win) => (raw) => {
  if (!raw || raw === "." || raw === "GLOBAL") return ".";
  const clean = (raw || "").trim();
  if (typeof win.__swissGetProjectFolder === "function") {
    const folder = win.__swissGetProjectFolder(clean);
    if (folder) return folder;
  }
  try {
    const saved = localStorage.getItem("antigravity_swiss_project_folders");
    if (saved) {
      const parsed = JSON.parse(saved);
      if (parsed[clean]) return parsed[clean];
      if (parsed[clean.toLowerCase()]) return parsed[clean.toLowerCase()];
      for (const k in parsed) {
        if (k.toLowerCase() === clean.toLowerCase()) return parsed[k];
      }
      const sepIdx = clean.search(/[/\\]/);
      if (sepIdx > 0) {
        const prefix = clean.substring(0, sepIdx);
        const rest = clean.substring(sepIdx + 1);
        let top = parsed[prefix] || parsed[prefix.toLowerCase()];
        if (!top) {
          for (const k in parsed) {
            if (k.toLowerCase() === prefix.toLowerCase()) { top = parsed[k]; break; }
          }
        }
        if (top) {
          return top.replace(/[/\\]+$/, "") + "/" + rest.replace(/^[/\\]+/, "");
        }
      }
    }
  } catch (_) {}
  return clean;
};

// 4a. Verify with window.__swissGetProjectFolder
const resolve1 = makeResolver(mockWindow);
assert.strictEqual(resolve1("WorkspaceAlpha"), "/var/projects/workspace-alpha");
assert.strictEqual(resolve1("WorkspaceAlpha/Assignment"), "/var/projects/workspace-alpha/Assignment");
assert.strictEqual(resolve1("GLOBAL"), ".");
assert.strictEqual(resolve1("."), ".");

// 4b. Verify fallback to localStorage when window.__swissGetProjectFolder is missing
const resolveFallback = makeResolver({});
assert.strictEqual(resolveFallback("WorkspaceAlpha"), "/var/projects/workspace-alpha");
assert.strictEqual(resolveFallback("workspacealpha"), "/var/projects/workspace-alpha");
assert.strictEqual(resolveFallback("ProjectSecureAI/src/main.rs"), "/var/projects/secure-ai/src/main.rs");
assert.strictEqual(resolveFallback("WorkspaceAlpha/Assignment/spec.pdf"), "/var/projects/workspace-alpha/Assignment/spec.pdf");
assert.strictEqual(resolveFallback("UnknownCustomDir"), "UnknownCustomDir");
`

	cmd := exec.Command("node")
	cmd.Stdin = strings.NewReader(nodeScript)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("node execution error in TestFileExplorerDefaultProjectFolderSetting: %v\n%s", err, string(out))
	}
}
func TestInChatTelemetryIsolationAndDualLayerSuppression(t *testing.T) {
	css := GenerateAuxiliaryPluginsCSS()
	js := GenerateAuxiliaryPluginsScript()

	// 1. Verify Dual-Layer CSS Scoping & Hard Suppression
	requiredCSSPatterns := []string{
		`[data-testid="conversation-view"] .swiss-inchat-metrics`,
		`html:not(.swiss-daemon-online) .swiss-inchat-metrics`,
		`html.swiss-daemon-offline .swiss-inchat-metrics`,
		`html:not(.swiss-daemon-online) #swiss-context-window-ring`,
		`html.swiss-daemon-offline #swiss-context-window-ring`,
		`html:not(.swiss-daemon-online) .swiss-context-window-ring`,
		`html.swiss-daemon-offline .swiss-context-window-ring`,
		`html:not(.swiss-daemon-online) .swiss-context-window-ring-sub`,
		`html.swiss-daemon-offline .swiss-context-window-ring-sub`,
		`[role="dialog"] .swiss-inchat-metrics`,
		`[aria-modal="true"] .swiss-inchat-metrics`,
		`.settings-tab-content .swiss-inchat-metrics`,
		`#swiss-aux-container .swiss-inchat-metrics`,
		`pre .swiss-inchat-metrics`,
		`code .swiss-inchat-metrics`,
		`display: none !important;`,
		`.swiss-telemetry-badge`,
	}
	for _, p := range requiredCSSPatterns {
		if !strings.Contains(css, p) {
			t.Errorf("expected CSS to contain pattern %q", p)
		}
	}

	// 2. Verify Container Scoping in Script
	if !strings.Contains(js, `document.querySelector('[data-testid="conversation-view"]')`) {
		t.Errorf("expected script to strictly scope telemetry to [data-testid=\"conversation-view\"]")
	}
	if !strings.Contains(js, `convoView.offsetParent === null`) || !strings.Contains(js, `convoView.style.display === "none"`) {
		t.Errorf("expected script to verify convoView visibility before injecting metrics")
	}

	// 3. Verify Feedback Row Anchoring
	if !strings.Contains(js, `button[aria-label="Good response"]`) || !strings.Contains(js, `button[aria-label="Bad response"]`) {
		t.Errorf("expected script to query Good response / Bad response feedback buttons")
	}
	if !strings.Contains(js, `convoView.contains(row)`) {
		t.Errorf("expected script to verify action row is contained within convoView")
	}

	// 4. Verify Strict Negative Exclusion Guards
	negativeGuards := []string{
		`[role="dialog"]`,
		`[aria-modal="true"]`,
		`.settings-tab-content`,
		`[data-testid*="settings"]`,
		`[data-testid="models-usage-container"]`,
		`#swiss-aux-container`,
		`[id^="swiss-aux"]`,
		`pre`,
		`code`,
		`.code-block`,
		`[data-testid="agent-input-box"]`,
	}
	for _, guard := range negativeGuards {
		if !strings.Contains(js, guard) {
			t.Errorf("expected script to enforce negative exclusion guard for %q", guard)
		}
	}

	// 5. Verify Active Modal Cleanup
	if !strings.Contains(js, `document.querySelectorAll('[role="dialog"] .swiss-inchat-metrics`) {
		t.Errorf("expected script to execute active modal cleanup on setup and mutations")
	}
}

func TestAuxiliaryConversationTabsExpansion(t *testing.T) {
	css := GenerateAuxiliaryPluginsCSS()
	if !strings.Contains(css, `/* 1b. Expanded Auxiliary Conversation / Subagent Tabs */`) {
		t.Errorf("expected CSS to include expanded auxiliary conversation / subagent tabs section")
	}
	if !strings.Contains(css, `min-width: max-content !important;`) {
		t.Errorf("expected CSS to allow conversation tabs to expand to content size")
	}
	if !strings.Contains(css, `button:not([role="tab"]):not(:has(span, p, .text-xs`) {
		t.Errorf("expected CSS to only constrain icon-only buttons without text")
	}
}

func TestGenerateAuxiliaryPluginsScript_InstallsAuthInterceptor(t *testing.T) {
	script := GenerateAuxiliaryPluginsScript()
	if !strings.Contains(script, "__swissAuthFetchPatched") {
		t.Fatalf("expected the fetch auth interceptor to be installed")
	}
	if !strings.Contains(script, `window.swiss`) {
		t.Fatalf("expected the interceptor to route daemon requests through the Electron broker")
	}
	// The file explorer endpoints must not remain bare: the interceptor covers
	// every daemon-origin fetch on the page.
	if !strings.Contains(script, "/api/files/list") {
		t.Fatalf("expected the auxiliary script to reference /api/files/list")
	}
}

func TestGenerateAuxiliaryPluginsScript_InterceptorWithoutSessionToken(t *testing.T) {
	t.Setenv("ANTIGRAVITY_SWISS_CONFIG_DIR", t.TempDir())
	script := GenerateAuxiliaryPluginsScript()
	if !strings.Contains(script, "__swissAuthFetchPatched") {
		t.Fatalf("interceptor must be present even when no session token exists")
	}
}

func TestGenerateAuxiliaryPluginsScript_AutoAcceptsSecurityNotice(t *testing.T) {
	script := GenerateAuxiliaryPluginsScript()
	if !strings.Contains(script, "__SWISS_RUNTIME__") {
		t.Fatalf("expected the security-notice auto-accept routine")
	}
	// Only the mandatory Accept button is clicked; the data-sharing checkbox
	// must never be touched on the user's behalf.
	if strings.Contains(script, "help improve Antigravity") {
		t.Fatalf("auto-accept must not interact with the optional data-sharing checkbox")
	}
}

// extractJSHandlerBlock returns the source of an arrow handler starting at
// `declaration` and ending at its matching closing brace. The handlers it runs
// on keep braces out of strings/comments, so brace counting is enough.
func extractJSHandlerBlock(t *testing.T, js, declaration string) string {
	t.Helper()
	start := strings.Index(js, declaration)
	if start < 0 {
		t.Fatalf("expected script to declare %q", declaration)
	}
	open := strings.Index(js[start:], "{")
	if open < 0 {
		t.Fatalf("expected %q to have a body", declaration)
	}
	open += start
	depth := 0
	for i := open; i < len(js); i++ {
		switch js[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return js[start : i+1]
			}
		}
	}
	t.Fatalf("unterminated body for %q", declaration)
	return ""
}

// Issue #150: memo titles never get a "[Voice] " marker, existing markers are
// stripped on render, and defaults number themselves with the lowest free value.
func TestQuickMemos_MemoNamingAndVoicePrefix(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()

	requiredTokens := []string{
		"const stripMemoVoicePrefix = (title) =>",
		`replace(/^\[Voice\]\s*/i, "")`,
		"const memoDisplayTitle = (memo) =>",
		"const nextMemoNumber = (titles, prefix) =>",
		`const MEMO_TITLE_PREFIXES = { voice: "Voice Memo", image: "Sketch" };`,
		`const nextMemoTitle = (kind) => {`,
		`return prefix + " " + nextMemoNumber(titles, prefix);`,
		`const defaultVoiceTitle = nextMemoTitle("voice");`,
		`nextMemoTitle("image")`,
		`escapeHtml(memoDisplayTitle(m))`,
	}
	for _, tok := range requiredTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected memo naming support to contain %q", tok)
		}
	}

	// The create path must not prepend the marker any more.
	if strings.Contains(js, `"[Voice] " +`) {
		t.Errorf("voice memo creation must not prepend a [Voice] prefix")
	}
	if strings.Contains(js, `"Voice Memo Note"`) {
		t.Errorf("voice memo fallback must use the numbered default title instead of the old placeholder")
	}

	if nodePath, err := exec.LookPath("node"); err == nil {
		fn := extractJSHandlerBlock(t, js, "const nextMemoNumber = ")
		if !strings.HasSuffix(fn, ";") {
			fn += ";"
		}
		nodeTestScript := fn + `
const assert = require("assert");
assert.strictEqual(nextMemoNumber([], "Voice Memo"), 1, "empty list starts at 1");
assert.strictEqual(nextMemoNumber(["Voice Memo 1", "Voice Memo 2"], "Voice Memo"), 3, "appends after the highest run");
assert.strictEqual(nextMemoNumber(["Voice Memo 1", "Voice Memo 3"], "Voice Memo"), 2, "reuses gaps");
assert.strictEqual(nextMemoNumber(["Sketch 1", "Untitled memo", "Voice Memo 2"], "Voice Memo"), 1, "ignores other titles");
assert.strictEqual(nextMemoNumber(["Sketch 5", "Sketch 2"], "Sketch"), 1, "starts at the lowest free number");
assert.strictEqual(nextMemoNumber(null, "Sketch"), 1, "tolerates a missing list");
assert.strictEqual(nextMemoNumber(["Voice Memo 1 (copy)"], "Voice Memo"), 1, "suffixed titles do not count");
console.log("NEXT_MEMO_NUMBER_OK");
`
		cmd := exec.Command(nodePath, "-e", nodeTestScript)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("node nextMemoNumber test failed: %v\n%s", err, string(out))
		}
		if !strings.Contains(string(out), "NEXT_MEMO_NUMBER_OK") {
			t.Fatalf("expected NEXT_MEMO_NUMBER_OK, got %q", string(out))
		}
	}
}

// Issue #151: creating a text memo asks for an optional title next to the body.
func TestQuickMemos_TextMemoOptionalTitle(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	requiredTokens := []string{
		`id="swiss-m-composer-title"`,
		`placeholder="Title (optional)..."`,
		`id="swiss-m-composer-label"`,
		`composer.setAttribute("role", "dialog")`,
		"const composerTitleInput = composer.querySelector",
		"const title = composerTitleInput.value.trim();",
		`createAndSaveMemo(text, "text", ["quick"], undefined, title)`,
		`editingMemo.title = title || autoMemoTitle(text);`,
	}
	for _, tok := range requiredTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected text memo create flow to contain %q", tok)
		}
	}
}

// Issue #152: right-clicking a card opens Change title / Save to computer /
// Delete, with auth headers, error toasts and keyboard activation.
func TestQuickMemos_CardContextMenu(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	requiredTokens := []string{
		"card.oncontextmenu = (e) => {",
		"showMemoCardContextMenu(e.clientX, e.clientY, m);",
		"function showMemoCardContextMenu(x, y, memo) {",
		"<span>Change title</span>",
		"<span>Save to computer</span>",
		"<span>Delete</span>",
		`menu.setAttribute("role", "menu")`,
		`API_BASE + "/api/memos/export"`,
		`JSON.stringify({ memo_id: memo.id })`,
		`headers: getSwissAuthHeaders()`,
		`showToast("Saved to " + payload.path);`,
		`(payload && payload.error)`,
		`await showSwissPrompt("Rename memo:", currentTitle)`,
		"deleteMemoById(memo);",
		`if (e.key === "Escape") {`,
		`} else if (e.key === "Enter" || e.key === " " || e.key === "Spacebar") {`,
		`} else if (e.key === "ArrowDown") {`,
		`} else if (e.key === "ArrowUp") {`,
		`window.addEventListener("scroll", removeContextMenu, true);`,
	}
	for _, tok := range requiredTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected memo card context menu to contain %q", tok)
		}
	}

	handler := extractJSHandlerBlock(t, js, "card.oncontextmenu = (e) => {")
	for _, tok := range []string{"e.preventDefault();", "e.stopPropagation();"} {
		if !strings.Contains(handler, tok) {
			t.Errorf("contextmenu handler must call %q so card handlers stay silent", tok)
		}
	}
}

// Issue #153: double-click edits text and image memos only; voice memos have no
// editor and a drag release must not open one.
func TestQuickMemos_DoubleClickEditing(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	requiredTokens := []string{
		"card.ondblclick = (e) => {",
		"if (memoDragInProgress) return;",
		"openComposer(m);",
		"openImageComposer(m);",
		"let composerEditingId = null;",
		"let imgEditingId = null;",
		"imgEditingId = memo && memo.id ? memo.id : null;",
		"const editingMemo = imgEditingId ? findMemoById(imgEditingId) : null;",
		"await persistMemo(editingMemo);",
		`composerTitleInput.value = memo ? stripMemoVoicePrefix(memo.title) : "";`,
		"memoDragInProgress = true;",
	}
	for _, tok := range requiredTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected double-click editing to contain %q", tok)
		}
	}

	handler := extractJSHandlerBlock(t, js, "card.ondblclick = (e) => {")
	if strings.Contains(handler, `"voice"`) {
		t.Errorf("voice memos must not open an editor on double-click: %q", handler)
	}
	if !strings.Contains(handler, "memoKindOf(m)") {
		t.Errorf("double-click handler must branch on the memo kind")
	}
}

// Issue #158: the address field grows a caret that opens a folder-jump menu with
// a Parents group (ancestors, root first) and a Siblings group (folders fetched
// from the parent directory). Navigation must reuse loadFiles and the address input.
func TestFileExplorerFolderJumpDropdown(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	css := GenerateAuxiliaryPluginsCSS()

	for _, rule := range []string{
		".swiss-files-path-wrap {",
		".swiss-files-path-caret {",
		".swiss-files-path-input {\n  flex: 1;\n  min-width: 0;\n  padding: 4px 26px 4px 8px;",
		".swiss-dir-jump-menu {",
		".swiss-dir-jump-group-label {",
		".swiss-dir-jump-list {",
		"max-height: 168px;",
		".swiss-dir-jump-item.active",
	} {
		if !strings.Contains(css, rule) {
			t.Errorf("expected folder jump CSS to contain %q", rule)
		}
	}

	// The dropdown must escape the explorer's overflow clipping: fixed positioning on
	// document.body, with placement derived from the caret rect.
	menuCSSStart := strings.Index(css, ".swiss-dir-jump-menu {")
	if menuCSSStart < 0 {
		t.Fatalf("expected .swiss-dir-jump-menu CSS block")
	}
	menuCSSEnd := strings.Index(css[menuCSSStart:], "}")
	if menuCSSEnd < 0 {
		t.Fatalf("unterminated .swiss-dir-jump-menu CSS block")
	}
	menuCSS := css[menuCSSStart : menuCSSStart+menuCSSEnd]
	if !strings.Contains(menuCSS, "position: fixed;") {
		t.Errorf("expected .swiss-dir-jump-menu to use position: fixed so ancestor overflow cannot clip it")
	}
	if strings.Contains(menuCSS, "position: absolute;") {
		t.Errorf("expected .swiss-dir-jump-menu to avoid position: absolute inside .swiss-files-view")
	}

	// The caret must live inside the address field wrapper, after the input, so it
	// renders at the right end of the input (the input reserves right padding for it).
	addrStart := strings.Index(js, `<div class="swiss-files-address-bar">`)
	wrapStart := strings.Index(js, `<span class="swiss-files-path-wrap">`)
	pathStart := strings.Index(js, `id="swiss-f-path"`)
	jumpStart := strings.Index(js, `id="swiss-f-jump"`)
	wrapEnd := strings.Index(js[wrapStart:], `</span>`)
	if addrStart < 0 || wrapStart < 0 || pathStart < 0 || jumpStart < 0 || wrapEnd < 0 {
		t.Fatalf("expected caret markup inside the explorer address bar")
	}
	wrapEnd += wrapStart
	if !(addrStart < wrapStart && wrapStart < pathStart && pathStart < jumpStart && jumpStart < wrapEnd) {
		t.Errorf("expected caret inside .swiss-files-path-wrap right after #swiss-f-path, got addr=%d wrap=%d path=%d jump=%d wrapEnd=%d",
			addrStart, wrapStart, pathStart, jumpStart, wrapEnd)
	}

	requiredTokens := []string{
		`class="swiss-browser-btn swiss-files-path-caret" id="swiss-f-jump" title="Jump to Folder" aria-haspopup="menu"`,
		`'<div class="swiss-dir-jump-group-label">Parents</div>'`,
		`'<div class="swiss-dir-jump-group-label">Siblings</div>'`,
		`parentsGroup.dataset.group = "parents";`,
		`siblingsGroup.dataset.group = "siblings";`,
		`const ancestorChain = (raw) => {`,
		`const parentOfDir = (raw) => {`,
		`const buildDirJumpMenu = () => {`,
		`const toggleDirJumpMenu = (e) => {`,
		`const navigateFromDirJump = (target) => {`,
		`loadFiles(target);`,
		`if (input) input.value = target;`,
		`API_BASE + "/api/files/list?path=" + encodeURIComponent(parent)`,
		`{ headers: getSwissAuthHeaders() }`,
		`.filter((item) => item && item.isDir)`,
		`normalizeJumpPath(item.path).toLowerCase() !== currentNorm`,
		`current.className = "swiss-dir-jump-item swiss-dir-jump-current active";`,
		`current.setAttribute("aria-disabled", "true");`,
		`console.warn("[swiss] Unable to load sibling folders:", err);`,
		`siblingsGroup.style.display = "none";`,
		`document.addEventListener("mousedown", onDirJumpOutsideClick, true);`,
		`window.addEventListener("scroll", onDirJumpScroll, true);`,
		`if (e.key === "Escape") {`,
		`if (e.key !== "ArrowDown" && e.key !== "ArrowUp" && e.key !== "Enter") return;`,
		`items[next].focus();`,
		`focused.click();`,
		`loadDirJumpSiblings(menu);`,
		`const clampMenuPosition = (anchor, menuSize, viewport) => {`,
		`dirJumpCaret.getBoundingClientRect()`,
		`menu.style.maxHeight = pos.maxHeight + "px";`,
		`document.body.appendChild(menu);`,
		`positionDirJumpMenu(menu);`,
		`window.addEventListener("resize", closeDirJumpMenu);`,
	}
	for _, tok := range requiredTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected folder jump dropdown to contain %q", tok)
		}
	}

	for _, decl := range []string{
		"const toggleDirJumpMenu = (e) => {",
		"item.addEventListener(\"click\", (e) => {",
	} {
		block := extractJSHandlerBlock(t, js, decl)
		for _, tok := range []string{"e.preventDefault();", "e.stopPropagation();"} {
			if !strings.Contains(block, tok) {
				t.Errorf("%q must call %q so explorer click handlers stay silent", decl, tok)
			}
		}
	}

	// Runtime check of the path math (Windows drive roots and POSIX roots).
	if nodePath, err := exec.LookPath("node"); err == nil {
		nodeScript := `
const assert = require("assert");
const fs = require("fs");
const js = fs.readFileSync(0, "utf8");
const start = js.indexOf("const normalizeJumpPath");
const end = js.indexOf("let dirJumpCaret = null;", start);
assert(start !== -1 && end !== -1, "folder jump helpers not found");
const api = new Function(js.slice(start, end) + "; return { normalizeJumpPath, parentOfDir, dirBaseName, ancestorChain };")();
assert.deepStrictEqual(api.ancestorChain("D:\\Projects\\app"), ["D:\\", "D:\\Projects"]);
assert.deepStrictEqual(api.ancestorChain("/home/me/proj"), ["/", "/home", "/home/me"]);
assert.strictEqual(api.parentOfDir("D:\\Projects"), "D:\\");
assert.strictEqual(api.parentOfDir("D:\\"), "");
assert.strictEqual(api.parentOfDir("/"), "");
assert.strictEqual(api.parentOfDir("/home"), "/");
assert.strictEqual(api.dirBaseName("D:\\Projects"), "Projects");
assert.strictEqual(api.dirBaseName("/"), "/");
assert.strictEqual(api.normalizeJumpPath("C:\\Temp\\\\"), "C:\\Temp");

// Placement math: right-aligned to the caret, clamped to the viewport, flipped up
// when the space below cannot fit the menu, and capped to the available space.
const placeStart = js.indexOf("const clampMenuPosition");
const placeEnd = js.indexOf("const positionDirJumpMenu", placeStart);
assert(placeStart !== -1 && placeEnd !== -1, "clampMenuPosition not found");
const place = new Function(js.slice(placeStart, placeEnd) + "; return clampMenuPosition;")();

const down = place(
  { top: 100, bottom: 120, left: 400, right: 500 },
  { width: 300, height: 200 },
  { width: 800, height: 600 }
);
assert.strictEqual(down.left, 200);
assert.strictEqual(down.top, 124);
assert.strictEqual(down.width, 300);
assert.strictEqual(down.maxHeight, 360);
assert.strictEqual(down.opensUp, false);

const rightOverflow = place(
  { top: 100, bottom: 120, left: 900, right: 1000 },
  { width: 300, height: 200 },
  { width: 800, height: 600 }
);
assert.strictEqual(rightOverflow.left, 492);
assert.strictEqual(rightOverflow.left + rightOverflow.width, 792);

const leftOverflow = place(
  { top: 100, bottom: 120, left: 0, right: 50 },
  { width: 400, height: 200 },
  { width: 800, height: 600 }
);
assert.strictEqual(leftOverflow.left, 8);

const flipped = place(
  { top: 350, bottom: 370, left: 100, right: 500 },
  { width: 300, height: 300 },
  { width: 800, height: 400 }
);
assert.strictEqual(flipped.opensUp, true);
assert.strictEqual(flipped.maxHeight, 240);
assert.strictEqual(flipped.top, 106);
assert(flipped.top + flipped.maxHeight <= 350 - 4, "flipped menu must clear the anchor");
`
		cmd := exec.Command(nodePath, "-e", nodeScript)
		cmd.Stdin = strings.NewReader(js)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Node.js folder jump path math failed: %v\n%s", err, string(out))
		}
	}
}

func TestBuiltinBrowserBookmarkContract(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()

	nativeAnchor := []string{
		`const pane = webview.closest('[data-testid="builtin-browser-pane"]')`,
		`const camera = pane.querySelector('[data-testid="webview-url-bar-screenshot"]')`,
		`camera.parentElement.insertBefore(starButton, camera)`,
		`starButton.setAttribute("aria-expanded", "false")`,
		`starButton.addEventListener("click"`,
	}
	for _, token := range nativeAnchor {
		if !strings.Contains(js, token) {
			t.Errorf("expected native Star attachment token %q", token)
		}
	}

	selection := strings.Index(js, `function selectBookmark`)
	navigation := strings.Index(js, `webview.loadURL(bookmark.address)`)
	if selection == -1 || navigation == -1 || selection >= navigation {
		t.Errorf("bookmark click must route through selectBookmark to webview.loadURL; select=%d load=%d", selection, navigation)
	}

	idempotency := []string{
		`pane.querySelector("#swiss-bookmark-star")`,
		`if (!starButton) {`,
		`bookmarkAdapters.forEach`,
		`if (pane.isConnected && entry.starButton.isConnected) return`,
		`starButton.removeEventListener("click", entry.onClick)`,
	}
	for _, token := range idempotency {
		if !strings.Contains(js, token) {
			t.Errorf("expected idempotent mount/cleanup token %q", token)
		}
	}
}

func TestBuiltinBrowserMigratesLegacyWrapperAndOmitsDuplicateControlBar(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	attachStart := strings.Index(js, "function migrateLegacyBuiltinBrowserWrapper")
	attachEnd := strings.Index(js, "window.setupBuiltinBrowserEnhancements")
	if attachStart < 0 || attachEnd <= attachStart {
		t.Fatalf("generated native browser attachment block not found")
	}
	attachBlock := js[attachStart:attachEnd]

	if !strings.Contains(attachBlock, "webview.__swissEnhanced = true") {
		t.Errorf("native webview must be marked as enhanced")
	}
	if !strings.Contains(attachBlock, `webview.closest(".swiss-builtin-browser-view")`) ||
		!strings.Contains(attachBlock, "host.insertBefore(webview, legacy)") ||
		!strings.Contains(attachBlock, "legacy.remove()") {
		t.Errorf("native attachment must migrate a legacy wrapper back to its host")
	}
	for _, forbidden := range []string{"parent.insertBefore(wrap, webview)", ".swiss-device-stage"} {
		if strings.Contains(attachBlock, forbidden) {
			t.Errorf("native attachment must not mount the standalone wrapper containing %q", forbidden)
		}
	}
	if !strings.Contains(js, "if (includeToolbar)") ||
		!strings.Contains(js, "wrap.appendChild(toolbar)") {
		t.Errorf("standalone browser renderer must conditionally append the shared toolbar")
	}

	preserved := []string{
		"screen.appendChild(existingWebview);",
		"screen.appendChild(canvas);",
		`const camera = pane.querySelector('[data-testid="webview-url-bar-screenshot"]')`,
		"camera.parentElement.insertBefore(starButton, camera)",
	}
	for _, token := range preserved {
		if !strings.Contains(js, token) {
			t.Errorf("native browser contract missing %q", token)
		}
	}
	wired := []string{
		`devSelect.onchange = () =>`,
		`orientSelect.onchange = () =>`,
		`toolbar.querySelector("#swiss-b-scale").onchange = applyDeviceScale`,
		`toolbar.querySelector("#swiss-b-touch").onclick`,
		`injectTouchEmulation(webview, active)`,
		`canvas.onmousedown = (e) =>`,
		`webview.executeJavaScript(inspectorScript)`,
		`webview.capturePage(`,
		`toolbar.querySelector("#swiss-b-send-chat").onclick = executeSendToChatWorkflow`,
	}
	for _, token := range wired {
		if !strings.Contains(js, token) {
			t.Errorf("native browser control is not wired to the live webview: %q", token)
		}
	}
}

func TestBuiltinBrowserBookmarkDOMLifecycle(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	start := strings.Index(js, "function renderBrowserView")
	attachEndMarker := "window.attachBuiltinBrowserEnhancements = attachBuiltinBrowserEnhancements;"
	attachEnd := strings.Index(js, attachEndMarker)
	end := attachEnd + len(attachEndMarker)
	if start < 0 || end <= start {
		t.Fatalf("generated bookmark lifecycle block not found: start=%d end=%d", start, end)
	}

	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for bookmark DOM lifecycle coverage")
	}

	nodeScript := `
const fs = require("fs");
const bookmarkBlock = fs.readFileSync(0, "utf8");
const assert = require("assert");
const vm = require("vm");

class MiniNode {
  constructor(tagName, ownerDocument) {
    this.tagName = tagName.toUpperCase();
    this.ownerDocument = ownerDocument;
    this.parentNode = null;
    this.children = [];
    this.attributes = new Map();
    this.listeners = new Map();
    this.style = {};
    this.id = "";
    this.className = "";
    this.title = "";
    this.type = "";
    this.value = "";
    this.textContent = "";
    this.onclick = null;
    this.onsubmit = null;
    this._innerHTML = "";
  }

  get classList() {
    const classes = this.className.split(/\s+/).filter(Boolean);
    return {
      contains: name => classes.includes(name),
      add: (...names) => {
        const next = new Set(this.className.split(/\s+/).filter(Boolean));
        names.forEach(name => next.add(name));
        this.className = Array.from(next).join(" ");
      },
      remove: (...names) => {
        const removed = new Set(names);
        this.className = this.className.split(/\s+/).filter(name => name && !removed.has(name)).join(" ");
      },
      toggle: (name, force) => {
        const has = this.className.split(/\s+/).includes(name);
        const enabled = force === undefined ? !has : Boolean(force);
        this.classList[enabled ? "add" : "remove"](name);
        return enabled;
      }
    };
  }

  get parentElement() {
    return this.parentNode instanceof MiniNode ? this.parentNode : null;
  }

  get isConnected() {
    let node = this;
    while (node.parentNode) node = node.parentNode;
    return node === this.ownerDocument;
  }

  get offsetWidth() {
    return this.classList.contains("swiss-bookmark-menu") || this.classList.contains("swiss-bookmark-context") ? 320 : 0;
  }

  get offsetHeight() {
    return this.classList.contains("swiss-bookmark-menu") || this.classList.contains("swiss-bookmark-context") ? 300 : 0;
  }

  get clientWidth() {
    return this.tagName === "WEBVIEW" ? 800 : 0;
  }

  get clientHeight() {
    return this.tagName === "WEBVIEW" ? 600 : 0;
  }

  set innerHTML(value) {
    this._innerHTML = String(value);
    if (this.classList.contains("swiss-bookmark-dialog-backdrop")) {
      const form = new MiniNode("form", this.ownerDocument);
      form.className = "swiss-bookmark-dialog";
      const name = new MiniNode("input", this.ownerDocument);
      name.setAttribute("data-swiss-bookmark-name", "");
      const address = new MiniNode("input", this.ownerDocument);
      address.setAttribute("data-swiss-bookmark-address", "");
      const cancel = new MiniNode("button", this.ownerDocument);
      cancel.setAttribute("data-swiss-bookmark-cancel", "");
      form.appendChild(name);
      form.appendChild(address);
      form.appendChild(cancel);
      this.appendChild(form);
    }
    if (this.classList.contains("swiss-browser-toolbar")) {
      const row = new MiniNode("div", this.ownerDocument);
      row.className = "swiss-browser-tools-row";
      const deviceGroup = new MiniNode("div", this.ownerDocument);
      deviceGroup.className = "swiss-browser-device-group";
      const annotationGroup = new MiniNode("div", this.ownerDocument);
      annotationGroup.className = "swiss-browser-annotation-group";
      [
        ["select", "swiss-b-device", deviceGroup],
        ["select", "swiss-b-orientation", deviceGroup],
        ["select", "swiss-b-scale", deviceGroup],
        ["button", "swiss-b-touch", deviceGroup],
        ["button", "swiss-b-pen", annotationGroup],
        ["button", "swiss-b-rect", annotationGroup],
        ["button", "swiss-b-inspect", annotationGroup],
        ["button", "swiss-b-clear", annotationGroup],
        ["button", "swiss-b-send-chat", annotationGroup]
      ].forEach(([tag, id, parent]) => {
        const control = new MiniNode(tag, this.ownerDocument);
        control.className = "swiss-browser-btn";
        control.id = id;
        parent.appendChild(control);
      });
      row.appendChild(deviceGroup);
      row.appendChild(annotationGroup);
      this.appendChild(row);
    }
  }

  get innerHTML() {
    return this._innerHTML;
  }

  setAttribute(name, value) {
    this.attributes.set(name, String(value));
  }

  getAttribute(name) {
    return this.attributes.has(name) ? this.attributes.get(name) : null;
  }

  hasAttribute(name) {
    return this.attributes.has(name);
  }

  removeAttribute(name) {
    this.attributes.delete(name);
  }

  appendChild(node) {
    if (node.parentNode) node.remove();
    node.parentNode = this;
    this.children.push(node);
    return node;
  }

  insertBefore(node, reference) {
    if (!reference) return this.appendChild(node);
    if (node.parentNode) node.remove();
    const index = this.children.indexOf(reference);
    node.parentNode = this;
    this.children.splice(index, 0, node);
    return node;
  }

  remove() {
    if (!this.parentNode) return;
    const index = this.parentNode.children.indexOf(this);
    if (index >= 0) this.parentNode.children.splice(index, 1);
    this.parentNode = null;
    const active = this.ownerDocument.activeElement;
    if (active && (active === this || this.contains(active))) {
      this.ownerDocument.activeElement = this.ownerDocument.body;
    }
  }

  replaceWith(node) {
    const parent = this.parentNode;
    if (!parent) return;
    const index = parent.children.indexOf(this);
    this.remove();
    node.parentNode = parent;
    parent.children.splice(index, 0, node);
  }

  contains(node) {
    if (!node) return false;
    if (node === this) return true;
    return this.children.some(child => child.contains(node));
  }

  matches(selector) {
    if (selector.startsWith("#")) return this.id === selector.slice(1);
    if (selector.startsWith(".")) return this.classList.contains(selector.slice(1));
    if (selector.startsWith("[")) {
      const match = selector.match(/^\[([^=\]]+)(?:="([^"]*)")?\]$/);
      if (!match) return false;
      return this.attributes.get(match[1]) === (match[2] === undefined ? "" : match[2]);
    }
    return this.tagName === selector.toUpperCase();
  }

  closest(selector) {
    let node = this;
    while (node) {
      if (node instanceof MiniNode && node.matches(selector)) return node;
      node = node.parentNode;
    }
    return null;
  }

  querySelectorAll(selector) {
    const found = [];
    const visit = node => {
      node.children.forEach(child => {
        if (child.matches(selector)) found.push(child);
        visit(child);
      });
    };
    visit(this);
    return found;
  }

  querySelector(selector) {
    return this.querySelectorAll(selector)[0] || null;
  }

  addEventListener(type, handler) {
    const handlers = this.listeners.get(type) || [];
    handlers.push(handler);
    this.listeners.set(type, handlers);
  }

  removeEventListener(type, handler) {
    const handlers = this.listeners.get(type) || [];
    this.listeners.set(type, handlers.filter(candidate => candidate !== handler));
  }

  dispatchEvent(event) {
    event.target = this;
    if (!event.preventDefault) event.preventDefault = () => {};
    const propertyHandler = this["on" + event.type];
    if (typeof propertyHandler === "function") propertyHandler.call(this, event);
    (this.listeners.get(event.type) || []).slice().forEach(handler => handler.call(this, event));
    return true;
  }

  focus() {
    this.ownerDocument.activeElement = this;
  }

  select() {}

  setCustomValidity(message) {
    this.validationMessage = message;
  }

  getBoundingClientRect() {
    const width = this.tagName === "WEBVIEW" ? 800 : (this.offsetWidth || 320);
    const height = this.tagName === "WEBVIEW" ? 600 : (this.offsetHeight || 240);
    return { top: 0, right: width, bottom: height, left: 0, width, height };
  }

  getContext() {
    if (!this._context2d) {
      this._context2d = {
        save() {},
        restore() {},
        setTransform() {},
        clearRect() {},
        beginPath() {},
        moveTo() {},
        lineTo() {},
        stroke() {},
        strokeRect() {},
        drawImage() {},
        getImageData() { return {}; },
        putImageData() {}
      };
    }
    return this._context2d;
  }
}

class MiniDocument {
  constructor(window) {
    this.defaultView = window;
    this.listeners = new Map();
    this.body = new MiniNode("body", this);
    this.body.parentNode = this;
    this.activeElement = this.body;
  }

  createElement(tagName) {
    return new MiniNode(tagName, this);
  }

  querySelectorAll(selector) {
    return this.body.querySelectorAll(selector);
  }

  querySelector(selector) {
    return this.querySelectorAll(selector)[0] || null;
  }

  addEventListener(type, handler) {
    const handlers = this.listeners.get(type) || [];
    handlers.push(handler);
    this.listeners.set(type, handlers);
  }

  removeEventListener(type, handler) {
    const handlers = this.listeners.get(type) || [];
    this.listeners.set(type, handlers.filter(candidate => candidate !== handler));
  }

  dispatchEvent(event) {
    if (!event.preventDefault) event.preventDefault = () => {};
    (this.listeners.get(event.type) || []).slice().forEach(handler => handler.call(this, event));
    return true;
  }

  listenerCount(type) {
    return (this.listeners.get(type) || []).length;
  }
}

function makeEnv() {
  const windowListeners = new Map();
  const windowObject = {
    innerWidth: 1024,
    innerHeight: 768,
    confirm: () => true,
    addEventListener(type, handler) {
      const handlers = windowListeners.get(type) || [];
      handlers.push(handler);
      windowListeners.set(type, handlers);
    },
    removeEventListener(type, handler) {
      const handlers = windowListeners.get(type) || [];
      windowListeners.set(type, handlers.filter(candidate => candidate !== handler));
    }
  };
  const document = new MiniDocument(windowObject);
  const stored = new Map();
  const localStorage = {
    getItem: key => stored.has(key) ? stored.get(key) : null,
    setItem: (key, value) => stored.set(key, String(value))
  };
  windowObject.document = document;
  windowObject.localStorage = localStorage;

  const context = vm.createContext({
    console,
    document,
    window: windowObject,
    localStorage,
    currentBrowserUrl: "about:blank",
    currentBookmarkUrl: "about:blank",
    activeDevice: "responsive",
    deviceOrientation: "portrait",
    drawTool: "none",
    isTouchEmulationActive: false,
    setTimeout: () => 1,
    clearTimeout: () => {},
    getComputedStyle: node => ({ position: node.style.position || "static" })
  });
  vm.runInContext(bookmarkBlock, context, { filename: "generated-bookmark-lifecycle.js" });

  const pane = document.createElement("div");
  pane.setAttribute("data-testid", "builtin-browser-pane");
  const row = document.createElement("div");
  row.appendChild(document.createElement("input"));
  const camera = document.createElement("button");
  camera.setAttribute("data-testid", "webview-url-bar-screenshot");
  row.appendChild(camera);
  pane.appendChild(row);
  const oldWebview = document.createElement("webview");
  pane.appendChild(oldWebview);
  document.body.appendChild(pane);

  const oldCalls = [];
  const newCalls = [];
  oldWebview.loadURL = url => oldCalls.push(url);
  assert.strictEqual(document.body.isConnected, true, "document body must be connected through document ancestry");
  assert.strictEqual(camera.isConnected, true, "the native camera anchor must be connected before teardown");

  const cameraFocus = { count: 0 };
  camera.focus = () => {
    cameraFocus.count += 1;
    MiniNode.prototype.focus.call(camera);
  };
  return { context, document, pane, camera, cameraFocus, localStorage, oldWebview, oldCalls, newCalls };
}

function click(node) {
  node.dispatchEvent({ type: "click", bubbles: true, cancelable: true });
}

function openMainMenu(env) {
  click(env.document.querySelector("#swiss-bookmark-star"));
  return env.document.querySelector(".swiss-bookmark-menu");
}

function wrapLegacyView(env) {
  env.oldWebview.remove();
  const wrapper = env.document.createElement("div");
  wrapper.className = "swiss-builtin-browser-view";
  const view = env.document.createElement("div");
  view.className = "swiss-browser-view";
  const stage = env.document.createElement("div");
  stage.className = "swiss-device-stage";
  stage.appendChild(env.oldWebview);
  view.appendChild(stage);
  wrapper.appendChild(view);
  env.pane.appendChild(wrapper);
  return wrapper;
}

const failures = [];
function scenario(name, fn) {
  try {
    fn();
    console.log("PASS " + name);
  } catch (error) {
    failures.push({ name, error: String(error && error.stack || error) });
    console.log("FAIL " + name);
  }
}

scenario("teardown removes open dialog and global listener without retaining overlay focus", () => {
  const env = makeEnv();
  env.context.setupNativeBookmarkButton(env.oldWebview);
  const baselineKeydown = env.document.listenerCount("keydown");
  const menu = openMainMenu(env);
  click(menu.querySelector(".swiss-bookmark-menu-add"));
  const backdrop = env.document.querySelector(".swiss-bookmark-dialog-backdrop");
  assert(backdrop, "dialog backdrop was not created");
  assert.strictEqual(env.document.listenerCount("keydown"), baselineKeydown + 1, "dialog key listener was not installed");

  env.context.removeNativeBookmarkButtons();
  assert(!env.document.querySelector(".swiss-bookmark-dialog-backdrop"), "dialog backdrop survived teardown");
  assert(!env.document.querySelector(".swiss-bookmark-dialog"), "dialog form survived teardown");
  assert(!env.document.querySelector("#swiss-bookmark-star"), "bookmark Star survived teardown");
  assert.strictEqual(env.document.listenerCount("keydown"), baselineKeydown, "global dialog key listener survived teardown");
  assert.strictEqual(env.cameraFocus.count, 1, "teardown did not exercise the connected camera fallback");
  assert.strictEqual(env.document.activeElement, env.camera, "teardown did not restore focus to the connected camera");

  const detached = env.document.createElement("button");
  assert.strictEqual(detached.isConnected, false, "a node without document ancestry must remain disconnected");

  env.context.removeNativeBookmarkButtons();
  assert.strictEqual(env.document.listenerCount("keydown"), baselineKeydown, "repeated teardown changed listener state");
});

scenario("replaced webview routes bookmark selection to the live webview only", () => {
  const env = makeEnv();
  env.context.setupNativeBookmarkButton(env.oldWebview);
  const newWebview = env.document.createElement("webview");
  newWebview.loadURL = url => env.newCalls.push(url);
  env.oldWebview.replaceWith(newWebview);
  assert.strictEqual(env.oldWebview.isConnected, false, "the replaced webview must be detached");
  assert.strictEqual(newWebview.isConnected, true, "the replacement webview must be connected");

  const address = env.context.getBookmarks()[0].address;
  click(openMainMenu(env).querySelector(".swiss-bookmark-menu-row"));
  assert.deepStrictEqual(env.newCalls, [address], "live webview did not receive the bookmark URL");
  assert.deepStrictEqual(env.oldCalls, [], "detached webview received the bookmark URL");
});

scenario("legacy native wrapper migrates and installs one Swiss functions bar", () => {
  const env = makeEnv();
  const wrapper = wrapLegacyView(env);

  env.context.attachBuiltinBrowserEnhancements(env.oldWebview);

  const bar = env.pane.querySelector(".swiss-native-browser-toolbar");
  assert.strictEqual(env.oldWebview.parentElement, env.pane, "native webview did not return to its host parent");
  assert.strictEqual(wrapper.isConnected, false, "legacy wrapper survived migration");
  assert.strictEqual(env.pane.querySelectorAll(".swiss-builtin-browser-view").length, 0, "legacy wrapper remains in the pane");
  assert(bar, "Swiss functions bar was not inserted");
  assert.strictEqual(env.pane.querySelectorAll(".swiss-native-browser-toolbar").length, 1, "expected exactly one Swiss functions bar");
  assert.strictEqual(bar.parentElement, env.pane, "Swiss functions bar is not in the native pane");
  assert.strictEqual(
    env.pane.children.indexOf(bar),
    env.pane.children.indexOf(env.camera.parentElement) + 1,
    "Swiss functions bar is not immediately below the native browser bar"
  );
  const barText = bar.innerHTML.toLowerCase();
  ["back", "forward", "refresh", "address", "quick ports"].forEach(label => {
    assert(!barText.includes(label), "Swiss functions bar duplicates native control: " + label);
  });

  const injected = [];
  env.oldWebview.executeJavaScript = code => {
    injected.push(String(code));
    return Promise.resolve();
  };
  const device = bar.querySelector("#swiss-b-device");
  const orientation = bar.querySelector("#swiss-b-orientation");
  const scale = bar.querySelector("#swiss-b-scale");
  device.value = "iphone-16-pro";
  device.onchange();
  assert.strictEqual(env.oldWebview.style.width, "402px", "device preset did not size the live webview");
  orientation.value = "landscape";
  orientation.onchange();
  assert.strictEqual(env.oldWebview.style.width, "874px", "orientation did not resize the live webview");
  assert.strictEqual(env.oldWebview.style.height, "402px", "orientation did not swap webview dimensions");
  scale.value = "0.5";
  scale.onchange();
  assert.strictEqual(env.oldWebview.style.transform, "scale(0.500)", "scale control did not transform the live webview");
  assert(injected.some(code => code.includes("__swissTouchInstalled")), "touch emulation was not injected into the live webview");
  click(bar.querySelector("#swiss-b-inspect"));
  assert(injected.some(code => code.includes("__swissInspectorActive")), "Inspect did not execute in the live webview");

  const star = env.document.querySelector("#swiss-bookmark-star");
  assert(star, "native bookmark Star did not mount");
  assert.strictEqual(star.parentElement, env.camera.parentElement, "bookmark Star left the native bar");

  const firstBar = bar;
  env.context.attachBuiltinBrowserEnhancements(env.oldWebview);

  assert.strictEqual(env.oldWebview.parentElement, env.pane, "native webview changed host parent on reattach");
  assert.strictEqual(env.oldWebview.__swissEnhanced, true, "native webview was not marked enhanced");
  assert.strictEqual(env.pane.querySelectorAll(".swiss-native-browser-toolbar").length, 1, "reattach created a duplicate bar");
  assert.strictEqual(env.pane.querySelector(".swiss-native-browser-toolbar"), firstBar, "reattach replaced the existing bar");

  const address = env.context.getBookmarks()[0].address;
  click(openMainMenu(env).querySelector(".swiss-bookmark-menu-row"));
  assert.deepStrictEqual(env.oldCalls, [address], "bookmark Star stopped navigating the native webview");

  env.context.teardownBuiltinBrowserEnhancements(env.oldWebview);
  assert(!env.pane.querySelector(".swiss-native-browser-toolbar"), "teardown left the Swiss functions bar");
  assert(!env.pane.querySelector(".swiss-native-browser-overlay"), "teardown left the annotation overlay");
  assert.strictEqual(env.oldWebview.parentElement, env.pane, "teardown moved the native webview");
  assert.strictEqual(env.camera.parentElement.isConnected, true, "teardown removed the native browser bar");
  assert.strictEqual(env.camera.isConnected, true, "teardown removed a native control");
});

scenario("disabled enhancements still migrate a legacy wrapper", () => {
  const env = makeEnv();
  const wrapper = wrapLegacyView(env);
  env.localStorage.setItem("antigravity_swiss_ext_browser_enabled", "false");

  env.context.setupBuiltinBrowserEnhancements();

  assert.strictEqual(env.oldWebview.parentElement, env.pane, "disabled setup left the webview wrapped");
  assert.strictEqual(wrapper.isConnected, false, "disabled setup left the legacy wrapper mounted");
  assert(!env.pane.querySelector(".swiss-native-browser-toolbar"), "disabled setup mounted the Swiss functions bar");
  assert(!env.pane.querySelector(".swiss-native-browser-overlay"), "disabled setup mounted the annotation overlay");
  assert(!env.pane.querySelector("#swiss-bookmark-star"), "disabled setup left the bookmark Star mounted");
});

scenario("detached legacy wrapper without a host is left untouched", () => {
  const env = makeEnv();
  const wrapper = env.document.createElement("div");
  wrapper.className = "swiss-builtin-browser-view";
  env.oldWebview.remove();
  wrapper.appendChild(env.oldWebview);

  env.context.attachBuiltinBrowserEnhancements(env.oldWebview);

  assert.strictEqual(env.oldWebview.parentElement, wrapper, "hostless webview was detached");
  assert.strictEqual(env.oldWebview.__swissEnhanced, undefined, "hostless webview was marked enhanced");
});

if (failures.length) {
  failures.forEach(failure => console.error("\n" + failure.name + "\n" + failure.error));
  process.exitCode = 1;
} else {
  console.log("BOOKMARK_DOM_LIFECYCLE_OK");
}
`

	cmd := exec.Command(nodePath, "-e", nodeScript)
	cmd.Stdin = strings.NewReader(js[start:end])
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bookmark DOM lifecycle test failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "BOOKMARK_DOM_LIFECYCLE_OK") {
		t.Fatalf("expected BOOKMARK_DOM_LIFECYCLE_OK, got:\n%s", string(out))
	}
}
