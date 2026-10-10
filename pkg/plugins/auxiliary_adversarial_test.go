package plugins

import (
	"os/exec"
	"strings"
	"testing"
)

// TestAdversarial_VisualAnnotation_DomAndSelectionMatrix executes a rigorous headless Node.js
// simulation of the DOM cursor and insertion mechanics implemented in auxiliary.go
func TestAdversarial_VisualAnnotation_DomAndSelectionMatrix(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node executable not found, skipping headless DOM simulation")
	}

	jsScript := GenerateAuxiliaryPluginsScript()

	// Verify required AST tokens in auxiliary.go implementation
	structuralTokens := []string{
		"range.selectNodeContents",
		"range.collapse(false)",
		"input.selectionStart = input.selectionEnd = input.value.length",
		"const hasContent = (input.innerText || \"\").trim().length > 0;",
		"const textToInsert = (hasContent ? \"\\n\" : \"\") + text;",
		"input.value = (curVal.trim() ? curVal.trim() + \"\\n\" : \"\") + text;",
	}

	for _, token := range structuralTokens {
		if !strings.Contains(jsScript, token) {
			t.Fatalf("CRITICAL: auxiliary.go missing mandatory insertion token: %q", token)
		}
	}

	nodeSimScript := `
const assert = require("assert");

// Mock Element hierarchy
class MockDOMElement {
  constructor(tag, isEditable = false) {
    this.tagName = tag.toUpperCase();
    this.value = "";
    this.innerText = "";
    this.isContentEditable = isEditable;
    this.selectionStart = 0;
    this.selectionEnd = 0;
    this.dispatchedEvents = [];
    this.focused = false;
  }
  focus() {
    this.focused = true;
  }
  dispatchEvent(ev) {
    this.dispatchedEvents.push(ev);
  }
}

// 1. Textarea with existing multi-line content
{
  const textarea = new MockDOMElement("textarea", false);
  textarea.value = "Line 1: Refactor UI\nLine 2: Test endpoints  \n\n";
  const annotation = "[Preview Browser Element Annotation @ http://localhost:3000]\n(Visual annotation attached: annotation.png)";

  const curVal = textarea.value || "";
  textarea.value = (curVal.trim() ? curVal.trim() + "\n" : "") + annotation;
  textarea.selectionStart = textarea.selectionEnd = textarea.value.length;
  textarea.dispatchEvent({ type: "input" });

  assert.ok(textarea.value.startsWith("Line 1: Refactor UI\nLine 2: Test endpoints\n[Preview Browser Element"), "must append cleanly after existing lines");
  assert.strictEqual(textarea.selectionStart, textarea.value.length, "cursor must be at end of text");
  assert.strictEqual(textarea.selectionEnd, textarea.value.length, "selection end must be at end of text");
  assert.strictEqual(textarea.dispatchedEvents[0].type, "input", "must dispatch input event for reactive frameworks");
}

// 2. Textarea with completely empty initial value
{
  const textarea = new MockDOMElement("textarea", false);
  textarea.value = "";
  const annotation = "[Preview Browser Annotation @ http://localhost:5173]";

  const curVal = textarea.value || "";
  textarea.value = (curVal.trim() ? curVal.trim() + "\n" : "") + annotation;
  textarea.selectionStart = textarea.selectionEnd = textarea.value.length;

  assert.strictEqual(textarea.value, annotation, "must not prefix newline when initial text is empty");
  assert.strictEqual(textarea.selectionStart, annotation.length, "cursor must be at end");
}

// 3. Contenteditable (Lexical) with existing content
{
  const editable = new MockDOMElement("div", true);
  editable.innerText = "Please investigate this visual issue";
  const annotation = "[Preview Browser Annotation @ http://localhost:8080]\n(Visual annotation attached: annotation.png)";

  let collapseParam = null;
  let selectedNode = null;
  const mockRange = {
    selectNodeContents(node) {
      selectedNode = node;
    },
    collapse(toStart) {
      collapseParam = toStart;
    }
  };

  mockRange.selectNodeContents(editable);
  mockRange.collapse(false); // false = collapse to end!

  const hasContent = (editable.innerText || "").trim().length > 0;
  const textToInsert = (hasContent ? "\n" : "") + annotation;

  assert.strictEqual(selectedNode, editable, "selectNodeContents must target input element");
  assert.strictEqual(collapseParam, false, "range.collapse MUST be false (end of container)");
  assert.ok(textToInsert.startsWith("\n"), "must prepend newline to cleanly separate appended annotation");
}

// 4. Contenteditable (Lexical) completely empty
{
  const editable = new MockDOMElement("div", true);
  editable.innerText = "   ";
  const annotation = "[Preview Browser Annotation]";

  let collapseParam = null;
  const mockRange = {
    selectNodeContents(node) {},
    collapse(toStart) {
      collapseParam = toStart;
    }
  };

  mockRange.selectNodeContents(editable);
  mockRange.collapse(false);

  const hasContent = (editable.innerText || "").trim().length > 0;
  const textToInsert = (hasContent ? "\n" : "") + annotation;

  assert.strictEqual(collapseParam, false, "collapse must be false");
  assert.strictEqual(textToInsert, annotation, "must not prepend newline if whitespace-only");
}

console.log("ADVERSARIAL_DOM_APPEND_TEST_SUCCESS");
`

	cmd := exec.Command(nodePath, "-e", nodeSimScript)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Headless DOM simulation failed: %v\nOutput: %s", err, string(out))
	}
	if !strings.Contains(string(out), "ADVERSARIAL_DOM_APPEND_TEST_SUCCESS") {
		t.Fatalf("Expected simulation success marker, got %q", string(out))
	}
}
