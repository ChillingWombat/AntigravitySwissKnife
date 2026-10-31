package custommodels

import (
	"os/exec"
	"strings"
	"testing"
)

func generatedFunctionForTest(t *testing.T, script, signature string) string {
	t.Helper()
	start := strings.Index(script, signature)
	if start < 0 {
		t.Fatalf("generated script is missing %q", signature)
	}
	open := strings.Index(script[start:], "{")
	if open < 0 {
		t.Fatalf("generated function %q has no body", signature)
	}
	depth := 0
	for i := start + open; i < len(script); i++ {
		switch script[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return script[start : i+1]
			}
		}
	}
	t.Fatalf("generated function %q has an unterminated body", signature)
	return ""
}

func runGeneratedNode(t *testing.T, source, marker string) {
	t.Helper()
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available; skipping generated DOM behavior test")
	}
	cmd := exec.Command(nodePath, "-e", source)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node verification failed: %v: %s", err, string(out))
	}
	if !strings.Contains(string(out), marker) {
		t.Fatalf("expected marker %q in node output, got: %s", marker, string(out))
	}
}

func TestGenerateCustomModelsScript_OpenThinkingSubmenuExecutes(t *testing.T) {
	script := GenerateCustomModelsScript(nil)
	openSubmenu := generatedFunctionForTest(t, script, "function openThinkingSubmenu()")
	runner := `
const assert = require("assert");

class FakeElement {
  constructor(tagName) {
    this.tagName = tagName;
    this.id = "";
    this.children = [];
    this.parentElement = null;
    this.attributes = {};
    this.style = {};
    this.textContent = "";
  }
  setAttribute(name, value) {
    this.attributes[name] = String(value);
    if (name === "id") this.id = String(value);
  }
  getAttribute(name) {
    return Object.prototype.hasOwnProperty.call(this.attributes, name) ? this.attributes[name] : null;
  }
  appendChild(child) {
    child.parentElement = this;
    this.children.push(child);
    return child;
  }
  remove() {
    if (!this.parentElement) return;
    const index = this.parentElement.children.indexOf(this);
    if (index >= 0) this.parentElement.children.splice(index, 1);
    this.parentElement = null;
  }
  getBoundingClientRect() {
    return { left: 100, right: 360, top: 60, bottom: 200 };
  }
}

const allElements = [];
function createElement(tagName) {
  const element = new FakeElement(tagName);
  allElements.push(element);
  return element;
}

const body = createElement("body");
const document = {
  body,
  createElement,
  getElementById(id) {
    return allElements.find(element => element.id === id) || null;
  }
};
const window = { innerWidth: 1200 };
const clearSubmenuCloseTimer = () => {};
const removeCustomThinkingSubmenu = () => {
  const submenu = document.getElementById("swiss-custom-thinking-submenu");
  if (submenu) submenu.remove();
};
const syncHoverState = () => {};
const selectCustomModelWithLevel = () => {};
const levels = ["low", "high"];
const m = { thinking_level: "low" };
const formatEffortLabel = level => level.charAt(0).toUpperCase() + level.slice(1);
const getActiveThinkingLevel = () => "low";
const FACTORY_CHECK_SVG = "<svg></svg>";
const menuCard = createElement("div");
const popover = menuCard;
const itemEl = createElement("div");
itemEl.getBoundingClientRect = () => ({ left: 120, right: 340, top: 64, bottom: 90 });
const getModelSelectorPopover = () => menuCard;
` + openSubmenu + `

openThinkingSubmenu();
const submenu = document.getElementById("swiss-custom-thinking-submenu");
assert.ok(submenu, "expected the generated handler to append a thinking submenu");
assert.strictEqual(submenu.__sourceItem, itemEl);
assert.strictEqual(submenu.getAttribute("role"), "menu");
assert.strictEqual(submenu.children.length, 2);
assert.match(submenu.style.cssText, /left: 360px;/);
assert.match(submenu.style.cssText, /top: 59px;/);
console.log("THINKING_SUBMENU_RENDERED");
`
	runGeneratedNode(t, runner, "THINKING_SUBMENU_RENDERED")
}

func TestGenerateCustomModelsScript_TriggerAriaLabelNamesModelAndLevel(t *testing.T) {
	script := GenerateCustomModelsScript(nil)
	renderLabel := generatedFunctionForTest(t, script, "function renderTriggerModelLabel(trigger, modelObj)")
	runner := `
const assert = require("assert");

class FakeElement {
  constructor(tagName) {
    this.tagName = tagName;
    this.children = [];
    this.parentElement = null;
    this.attributes = {};
    this.style = {};
    this.textContent = "";
  }
  setAttribute(name, value) {
    this.attributes[name] = String(value);
  }
  getAttribute(name) {
    return Object.prototype.hasOwnProperty.call(this.attributes, name) ? this.attributes[name] : null;
  }
  appendChild(child) {
    child.parentElement = this;
    this.children.push(child);
    return child;
  }
}

const document = { createElement: tagName => new FakeElement(tagName) };
const label = new FakeElement("span");
label.textContent = "Gemini 3.8 Flash";
const trigger = new FakeElement("button");
trigger.querySelector = selector => selector === "span" ? label : null;
trigger.setAttribute("aria-label", "Gemini 3.8 Flash");
const cleanModelDisplayName = raw => raw;
const getModelThinkingLevels = () => ["low", "high"];
const getActiveThinkingLevel = () => "high";
const formatEffortLabel = level => level.charAt(0).toUpperCase() + level.slice(1);
` + renderLabel + `

renderTriggerModelLabel(trigger, {
  id: "claude-sonnet",
  name: "claude-sonnet",
  display_name: "Claude Sonnet",
  supports_thinking: true,
  thinking_levels: ["low", "high"],
  thinking_level: "high"
});
assert.strictEqual(trigger.getAttribute("aria-label"), "Claude Sonnet (High)");
console.log("TRIGGER_ARIA_LABEL_VERIFIED");
`
	runGeneratedNode(t, runner, "TRIGGER_ARIA_LABEL_VERIFIED")
}
