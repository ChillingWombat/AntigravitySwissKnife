package plugins

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/ChillingWombat/antigravity-swiss-knife/pkg/enhancements"
)

func TestCommunityExtensions_HostRuntimeInjection(t *testing.T) {
	css := GenerateAuxiliaryPluginsCSS()
	if !strings.Contains(css, ".swiss-community-ext-view") {
		t.Errorf("expected CSS to define .swiss-community-ext-view")
	}
	if !strings.Contains(css, ".swiss-community-ext-toolbar") {
		t.Errorf("expected CSS to define .swiss-community-ext-toolbar")
	}
	if !strings.Contains(css, ".swiss-community-ext-frame") {
		t.Errorf("expected CSS to define .swiss-community-ext-frame")
	}

	js := GenerateAuxiliaryPluginsScript()
	requiredJS := []string{
		"getCommunityExtensions",
		"getCommunityExtensionSvg",
		"window.getSwissCommunityExtensionSvg",
		"window.getSwissCommunityExtensions",
		"renderCommunityExtensionView",
		"window.renderSwissCommunityExtensionView",
		"swiss-community-ext-view",
		"swiss-community-ext-toolbar",
		"swiss-community-ext-frame",
		"swiss-insert-chat",
	}
	for _, req := range requiredJS {
		if !strings.Contains(js, req) {
			t.Errorf("expected auxiliary JS to contain %q", req)
		}
	}

	ghScript := GenerateGitHubExtensionScript()
	requiredGH := []string{
		"window.getSwissCommunityExtensions",
		"window.getSwissCommunityExtensionSvg",
		"window.renderSwissCommunityExtensionView",
	}
	for _, req := range requiredGH {
		if !strings.Contains(ghScript, req) {
			t.Errorf("expected GitHub extension script to contain %q", req)
		}
	}

	// Node.js runtime validation of pure helpers and rendering functions
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}

	nodeScript := `
const assert = require("assert");

// Mock DOM environment
class MockElement {
  constructor(tag, cls = "") {
    this.tagName = tag.toUpperCase();
    this.className = cls;
    this.style = {};
    this.attributes = {};
    this.children = [];
    this.dataset = {};
    this.innerHTML = "";
    this.textContent = "";
  }
  setAttribute(k, v) { this.attributes[k] = String(v); }
  getAttribute(k) { return this.attributes[k]; }
  removeAttribute(k) { delete this.attributes[k]; }
  appendChild(c) { this.children.push(c); return c; }
  querySelector(sel) { return this.children[0] || null; }
  querySelectorAll(sel) { return this.children; }
}

const listeners = {};
global.window = {
  __SWISS_ENH_CONFIG__: {
    community_extensions: [
      { id: "ext-1", name: "Ext One", icon: "code", enabled: true, source_type: "url", url: "http://example.com/1" },
      { id: "ext-2", name: "Ext Two", icon: "terminal", enabled: false, source_type: "url", url: "http://example.com/2" },
      { id: "ext-3", name: "Ext Three", icon: "cpu", enabled: true, source_type: "html", html: "<p>3</p>" }
    ]
  },
  addEventListener: (event, handler) => { listeners[event] = handler; },
  removeEventListener: (event, handler) => { delete listeners[event]; },
  open: () => {},
  location: { pathname: "/" }
};
global.document = {
  body: new MockElement("body"),
  createElement: (tag) => new MockElement(tag),
  querySelector: () => null,
  querySelectorAll: () => [],
  getElementById: () => null,
  addEventListener: () => {},
  removeEventListener: () => {}
};
global.localStorage = {
  getItem: () => null,
  setItem: () => {},
  removeItem: () => {}
};
global.MutationObserver = class {
  observe() {}
  disconnect() {}
};
global.requestAnimationFrame = () => 1;
global.cancelAnimationFrame = () => {};
global.setInterval = () => ({ unref: () => {} });
global.clearInterval = () => {};

// Evaluate the script inside this context
` + js + `

// 1. Test getCommunityExtensions
assert(typeof window.getSwissCommunityExtensions === "function", "getSwissCommunityExtensions must be exposed");
const exts = window.getSwissCommunityExtensions();
assert.strictEqual(exts.length, 2, "must filter out disabled extensions (ext-2)");
assert.strictEqual(exts[0].id, "ext-1");
assert.strictEqual(exts[1].id, "ext-3");

// 2. Test getCommunityExtensionSvg
assert(typeof window.getSwissCommunityExtensionSvg === "function", "getSwissCommunityExtensionSvg must be exposed");
const icons = ["blocks", "globe", "code", "terminal", "database", "bot", "wrench", "book-open", "layers", "cpu"];
icons.forEach(ic => {
  const svg = window.getSwissCommunityExtensionSvg(ic, "test-cls");
  assert(svg.includes("svg"), ic + " must return svg");
  assert(svg.includes('class="test-cls"'), ic + " must include class");
});
const fallbackSvg = window.getSwissCommunityExtensionSvg("unknown-icon", "test-cls");
assert(fallbackSvg.includes("rect"), "unknown icon must fall back to blocks");

// 3. Test renderCommunityExtensionView for URL mode
assert(typeof window.renderSwissCommunityExtensionView === "function", "renderSwissCommunityExtensionView must be exposed");
const container = new MockElement("div");
window.renderSwissCommunityExtensionView(container, exts[0]);
assert.strictEqual(container.children.length, 1);
const wrap = container.children[0];
assert.strictEqual(wrap.className, "swiss-community-ext-view");
assert.strictEqual(wrap.children.length, 2); // toolbar + iframe
const toolbar = wrap.children[0];
assert.strictEqual(toolbar.className, "swiss-community-ext-toolbar");
const iframe = wrap.children[1];
assert.strictEqual(iframe.className, "swiss-community-ext-frame");
assert.strictEqual(iframe.src, "http://example.com/1");

// 4. Test renderCommunityExtensionView for HTML mode
const htmlContainer = new MockElement("div");
window.renderSwissCommunityExtensionView(htmlContainer, exts[1]);
const htmlIframe = htmlContainer.children[0].children[1];
assert.strictEqual(htmlIframe.getAttribute("sandbox"), "allow-scripts allow-forms allow-modals");
assert(htmlIframe.srcdoc.includes("<p>3</p>"), "srcdoc must contain HTML payload");

console.log("Community extensions host runtime OK");
process.exit(0);
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(nodeScript)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Node.js community extensions runtime check failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "Community extensions host runtime OK") {
		t.Fatalf("unexpected output: %s", string(out))
	}
}

func TestSwissMainStage_OverviewPanelAutoHideAndRestoreRuntime(t *testing.T) {
	ghScript := GenerateGitHubExtensionScript()
	requiredGH := []string{
		"isMainStageActive",
		"findOverviewPanel",
		"findOverviewPanelElements",
		"hideOverviewPanelForMainStage",
		"restoreOverviewPanelFromMainStage",
		"data-swiss-hidden-by-main-stage",
	}
	for _, req := range requiredGH {
		if !strings.Contains(ghScript, req) {
			t.Errorf("expected GitHub extension script to contain %q", req)
		}
	}

	enhScript := enhancements.GenerateEnhancementsScript(enhancements.DefaultConfig())
	requiredEnh := []string{
		"isMainStageActive",
		"findOverviewPanel",
		"findOverviewPanelElements",
		"hideOverviewPanelForMainStage",
		"restoreOverviewPanelFromMainStage",
		"data-swiss-hidden-by-main-stage",
	}
	for _, req := range requiredEnh {
		if !strings.Contains(enhScript, req) {
			t.Errorf("expected enhancements script to contain %q", req)
		}
	}

	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available")
	}

	nodeScript := `
const assert = require("assert");

class MockClassList {
  constructor(el) { this.el = el; }
  contains(cls) { return (this.el.className || "").split(/\s+/).includes(cls); }
  add(cls) {
    if (!this.contains(cls)) {
      this.el.className = (this.el.className ? this.el.className + " " + cls : cls).trim();
    }
  }
  remove(cls) {
    this.el.className = (this.el.className || "").split(/\s+/).filter(c => c && c !== cls).join(" ");
  }
}

class MockElement {
  constructor(tag, cls = "") {
    this.tagName = tag.toUpperCase();
    this.className = cls;
    this.id = "";
    this._styleProps = {};
    this.style = {
      display: "",
      setProperty: (k, v, imp) => { this._styleProps[k] = v; this.style[k] = v; },
      removeProperty: (k) => { delete this._styleProps[k]; this.style[k] = ""; }
    };
    this.attributes = {};
    this.children = [];
    this.parentElement = null;
    this.dataset = {};
    this.textContent = "";
    this.classList = new MockClassList(this);
  }
  setAttribute(k, v) { this.attributes[k] = String(v); }
  getAttribute(k) { return this.attributes[k]; }
  removeAttribute(k) { delete this.attributes[k]; }
  appendChild(c) {
    c.parentElement = this;
    this.children.push(c);
    return c;
  }
  removeChild(c) {
    const idx = this.children.indexOf(c);
    if (idx >= 0) {
      this.children.splice(idx, 1);
      c.parentElement = null;
    }
    return c;
  }
  closest(sel) {
    let cur = this;
    while (cur) {
      if (matchesSel(cur, sel)) return cur;
      cur = cur.parentElement;
    }
    return null;
  }
  querySelector(sel) {
    const all = this.querySelectorAll(sel);
    return all.length > 0 ? all[0] : null;
  }
  querySelectorAll(sel) {
    const results = [];
    function walk(node) {
      if (!node) return;
      for (const child of node.children) {
        if (matchesSel(child, sel)) results.push(child);
        walk(child);
      }
    }
    walk(this);
    return results;
  }
}

function matchesSel(el, sel) {
  if (!el || !sel) return false;
  const parts = sel.split(",").map(s => s.trim());
  return parts.some(part => matchSingle(el, part));
}

function matchSingle(el, sel) {
  if (sel === "*") return true;
  if (sel.startsWith("#")) return el.id === sel.slice(1);
  if (sel.startsWith(".")) return el.classList.contains(sel.slice(1));
  if (sel.startsWith("[") && sel.endsWith("]")) {
    const inside = sel.slice(1, -1);
    if (inside.includes("=")) {
      const eqIdx = inside.indexOf("=");
      const attr = inside.slice(0, eqIdx);
      const val = inside.slice(eqIdx + 1).replace(/[\x27"]/g, "");
      if (attr.endsWith("*")) {
        const a = attr.slice(0, -1);
        return (el.getAttribute(a) || "").includes(val);
      }
      return el.getAttribute(attr) === val;
    }
    return el.getAttribute(inside) !== undefined;
  }
  return el.tagName.toLowerCase() === sel.toLowerCase();
}

const listeners = {};
global.window = {
  addEventListener: (event, handler) => { listeners[event] = handler; },
  removeEventListener: (event, handler) => { delete listeners[event]; },
  getComputedStyle: (el) => ({ display: (el.style && el.style.display) || "" }),
  location: { pathname: "/c/test-chat" }
};

const docBody = new MockElement("body");
global.document = {
  body: docBody,
  head: new MockElement("head"),
  createElement: (tag) => new MockElement(tag),
  getElementById: (id) => {
    function find(node) {
      if (node.id === id) return node;
      for (const c of node.children) {
        const found = find(c);
        if (found) return found;
      }
      return null;
    }
    return find(docBody);
  },
  querySelector: (sel) => {
    if (matchesSel(docBody, sel)) return docBody;
    return docBody.querySelector(sel);
  },
  querySelectorAll: (sel) => {
    const list = docBody.querySelectorAll(sel);
    if (matchesSel(docBody, sel)) list.unshift(docBody);
    return list;
  },
  addEventListener: () => {},
  removeEventListener: () => {}
};

global.localStorage = {
  getItem: () => null,
  setItem: () => {},
  removeItem: () => {}
};
global.MutationObserver = class {
  observe() {}
  disconnect() {}
};
global.requestAnimationFrame = (fn) => setTimeout(fn, 0);
global.cancelAnimationFrame = () => {};
global.setInterval = () => ({ unref: () => {} });
global.clearInterval = () => {};

// Build DOM elements before evaluating scripts
const navContainer = new MockElement("div", "sidebar-nav");
const autoBtn = new MockElement("button");
autoBtn.setAttribute("data-testid", "automations-button");
navContainer.appendChild(autoBtn);
docBody.appendChild(navContainer);

const mainEl = new MockElement("main");
const convoView = new MockElement("div");
convoView.setAttribute("data-testid", "conversation-view");
mainEl.appendChild(convoView);
docBody.appendChild(mainEl);

// 1. Native floating overview panel overlay (Antigravity 2.0 factory structure)
const overlayHost = new MockElement("div", "shadow-xl animate-overview-panel");
overlayHost.setAttribute("data-testid", "overview-panel-host-overlay");
overlayHost.setAttribute("role", "dialog");
overlayHost.setAttribute("aria-label", "Overview");
const rowReview = new MockElement("button");
rowReview.setAttribute("data-testid", "overview-panel-row-review");
rowReview.textContent = "Review";
overlayHost.appendChild(rowReview);
docBody.appendChild(overlayHost);

// 2. Native backdrop
const backdrop = new MockElement("div", "absolute inset-0 z-40");
backdrop.setAttribute("data-testid", "overview-panel-overlay-backdrop");
docBody.appendChild(backdrop);

// 3. Custom overview panel
const customPanel = new MockElement("div");
customPanel.setAttribute("data-swiss-overview-panel", "true");
customPanel.textContent = "Overview Review Files Terminal Plugins";
docBody.appendChild(customPanel);

// 4. Native overview panel inner
const nativePanel = new MockElement("div");
nativePanel.setAttribute("data-testid", "overview-panel");
docBody.appendChild(nativePanel);

// Evaluate scripts in this environment
` + ghScript + `
` + enhScript + `

// Verification 1: Exposed API
assert.strictEqual(typeof window.isMainStageActive, "function", "window.isMainStageActive must be function");
assert.strictEqual(typeof window.findOverviewPanel, "function", "window.findOverviewPanel must be function");
assert.strictEqual(typeof window.findOverviewPanelElements, "function", "window.findOverviewPanelElements must be function");
assert.strictEqual(typeof window.hideOverviewPanelForMainStage, "function", "window.hideOverviewPanelForMainStage must be function");
assert.strictEqual(typeof window.restoreOverviewPanelFromMainStage, "function", "window.restoreOverviewPanelFromMainStage must be function");

// Initial state: stage inactive
assert.strictEqual(window.isMainStageActive(), false, "isMainStageActive() must be false initially");
assert.strictEqual(overlayHost.getAttribute("data-swiss-hidden-by-main-stage"), undefined);
assert.strictEqual(backdrop.getAttribute("data-swiss-hidden-by-main-stage"), undefined);
assert.strictEqual(customPanel.getAttribute("data-swiss-hidden-by-main-stage"), undefined);

const foundElements = window.findOverviewPanelElements();
assert(foundElements.length >= 3, "must detect overview panel elements (overlay, backdrop, custom, panel)");
assert(foundElements.includes(overlayHost), "must detect overlayHost");
assert(foundElements.includes(backdrop), "must detect backdrop");
assert(foundElements.includes(customPanel), "must detect customPanel");

// Scenario A: #swiss-main-stage active
const stage1 = new MockElement("div");
stage1.id = "swiss-main-stage";
stage1.style.display = "flex";
docBody.appendChild(stage1);

assert.strictEqual(window.isMainStageActive(), true, "isMainStageActive() must be true when #swiss-main-stage has display:flex");

window.hideOverviewPanelForMainStage();
assert.strictEqual(overlayHost.getAttribute("data-swiss-hidden-by-main-stage"), "true");
assert.strictEqual(overlayHost.style.display, "none");
assert.strictEqual(backdrop.getAttribute("data-swiss-hidden-by-main-stage"), "true");
assert.strictEqual(backdrop.style.display, "none");
assert.strictEqual(customPanel.getAttribute("data-swiss-hidden-by-main-stage"), "true");
assert.strictEqual(customPanel.style.display, "none");

// Scenario B: Closing stage restores overview panels
stage1.style.display = "none";
assert.strictEqual(window.isMainStageActive(), false, "isMainStageActive() must be false when #swiss-main-stage display is none");

window.restoreOverviewPanelFromMainStage();
assert.strictEqual(overlayHost.getAttribute("data-swiss-hidden-by-main-stage"), undefined);
assert.strictEqual(overlayHost.style.display, "");
assert.strictEqual(backdrop.getAttribute("data-swiss-hidden-by-main-stage"), undefined);
assert.strictEqual(backdrop.style.display, "");
assert.strictEqual(customPanel.getAttribute("data-swiss-hidden-by-main-stage"), undefined);
assert.strictEqual(customPanel.style.display, "");

// Scenario C: #swiss-main-stage-container with openSwissMainStage and closeMainStage
docBody.removeChild(stage1);
const stageContainer = new MockElement("div");
stageContainer.id = "swiss-main-stage-container";
stageContainer.style.display = "none";
docBody.appendChild(stageContainer);

stageContainer.style.display = "flex";
assert.strictEqual(window.isMainStageActive(), true, "isMainStageActive() true for active stage container");
window.hideOverviewPanelForMainStage();
assert.strictEqual(overlayHost.getAttribute("data-swiss-hidden-by-main-stage"), "true");
assert.strictEqual(overlayHost.style.display, "none");

window.closeMainStage();
assert.strictEqual(window.isMainStageActive(), false, "isMainStageActive() false after closeMainStage()");
assert.strictEqual(stageContainer.style.display, "none");
assert.strictEqual(overlayHost.getAttribute("data-swiss-hidden-by-main-stage"), undefined);
assert.strictEqual(overlayHost.style.display, "");

console.log("Overview panel auto-hide and restore runtime OK");
process.exit(0);
`

	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(nodeScript)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Node.js overview panel auto-hide check failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "Overview panel auto-hide and restore runtime OK") {
		t.Fatalf("unexpected output: %s", string(out))
	}
}
