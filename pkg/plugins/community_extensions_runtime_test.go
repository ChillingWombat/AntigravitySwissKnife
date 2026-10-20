package plugins

import (
	"os/exec"
	"strings"
	"testing"
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
