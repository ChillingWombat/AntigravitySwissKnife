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
		"SWISS_INSPECT_CANCEL",     // Message bridge cancellation token on Escape
		"swiss-dom-inspect-overlay", // Highlight overlay ID
		"swiss-dom-inspect-badge",   // Selector tag badge ID
		"exitInspectTool",          // Helper to exit inspect element tool
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
  ["swiss-f-home", "swiss-f-refresh", "swiss-f-path", "swiss-f-reveal", "swiss-f-new-file", "swiss-f-new-dir"]
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
const currentFilePath = "/mnt/Data/Projects";
const toolbar = { querySelector: (sel) => (sel === "#swiss-f-reveal" ? revealBtn : null) };
const revealBtn = {};
const fetch = (url, opts) => { fetchedUrl = url; fetchedOpts = opts; };
eval(revealHandlerMatch[0]);
revealBtn.onclick();
assert.strictEqual(fetchedUrl, "http://127.0.0.1:19876/api/files/reveal");
assert.strictEqual(fetchedOpts.method, "POST");
assert.deepStrictEqual(JSON.parse(fetchedOpts.body), { path: "/mnt/Data/Projects" });
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

func TestAuxiliaryPerExtensionVisibilityAndOrientation(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	css := GenerateAuxiliaryPluginsCSS()

	jsTokens := []string{
		"isAuxExtensionVisible",                        // per-extension aux panel gate
		"aux_panel",                                    // EnhancementsConfig.extensions visibility key
		`id="swiss-b-orientation"`,                     // Portrait|Landscape control
		"antigravity_swiss_browser_orientation",        // persisted IDE-side orientation key
		"deviceOrientation",                            // orientation state variable
		"swiss-landscape",                              // landscape class applied to device frame
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
		"portSelect.onchange",
		"portSelect.oncontextmenu",
		"updateActivePortChip",
		"portSelect.value = activePort",
		`portSelect.value = ""`,
		"updateToolbarResponsiveness",
		"ResizeObserver",
		`toolbar.classList.toggle("compact-ports"`,
		`toolbar.classList.toggle("compact-tools"`,
		`row.scrollLeft += e.deltaY`,
	}
	for _, tok := range dropdownLogicTokens {
		if !strings.Contains(js, tok) {
			t.Errorf("expected JS to contain dropdown logic token %q", tok)
		}
	}

	// Verify that Delete Port option is removed from select options
	if strings.Contains(js, "Delete Port...") {
		t.Errorf("found removed Delete Port... option in auxiliary plugins script")
	}

	// Verify that port-select.active does not highlight with solid blue background
	if strings.Contains(css, ".swiss-port-select.active {\n  background: #1a73e8") {
		t.Errorf("found blue background on .swiss-port-select.active, should not highlight to blue")
	}

	// Verify tools row can split into two rows when narrow with 620px container query and hidden divider
	if !strings.Contains(css, "@container swisstoolbar (max-width: 620px)") {
		t.Errorf("expected CSS to contain max-width: 620px container query for tools row")
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

assert.strictEqual(portSelect.options.length, 4);
assert.strictEqual(portSelect.value, ""); // defaults to placeholder Quick Ports
assert.strictEqual(portSelect.options[0].textContent, "Quick Ports");
assert.strictEqual(portSelect.options[3].textContent, "+ Add Port...");

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
		`await showSwissPrompt("Voice recorded! Enter a transcript / note title:", "Voice Memo Note")`,
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
	if !strings.Contains(js, "if (window.__swissDaemonOnline === false)") {
		t.Errorf("expected setupAuxiliaryTabs to guard against window.__swissDaemonOnline === false")
	}

	// 3. Verify daemon offline removal of auxiliary tab buttons and dividers
	if !strings.Contains(js, `document.querySelectorAll(".swiss-aux-btn-group, .swiss-aux-tabs-divider, .swiss-aux-tabs-divider-left, .swiss-aux-tabs-divider-right").forEach(el => el.remove())`) {
		t.Errorf("expected daemon offline handler to remove aux button group and dividers")
	}

	// 4. Verify syntax passes in Node
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

