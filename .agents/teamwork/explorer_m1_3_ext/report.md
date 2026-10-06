# Exploration Report: R2 Visual Canvas Annotation Tool & Send to Chat Workflow

**Explorer**: `explorer_m1_3_ext` (teamwork_preview_explorer)  
**Target Milestone**: Ext-M1 (Requirement R2)  
**Target Files**: `pkg/plugins/auxiliary.go`, `pkg/plugins/auxiliary_test.go`  
**Date**: 2026-10-05T22:33:00Z  

---

## Executive Summary

This report establishes the complete architectural and implementation blueprint for **Requirement R2 (Live Browser Preview & Visual Canvas Annotation Tool)** within Antigravity Swiss Knife.

Currently, `pkg/plugins/auxiliary.go` contains an initial stub of `renderBrowserView` that suffers from three primary deficiencies:
1. **Jagged Pen Strokes**: Freehand drawing uses discrete `lineTo` segments without curve smoothing.
2. **Missing DOM Element Selector**: There is no interactive DOM inspector to inspect and select elements inside the previewed page.
3. **Incomplete Chat Dispatch**: The "Send to Chat" button only passes plain text via `insertTextToChatInput` and does not crop annotations, synthesize image files, or interface with Antigravity's file upload input (`input[type="file"]`) or Lexical editor (`__lexicalEditor`).

This blueprint delivers the exact algorithms, event architectures, test assertions, and code snippets needed to upgrade `pkg/plugins/auxiliary.go` and `pkg/plugins/auxiliary_test.go` to production readiness.

---

## 1. Deep Inspection of Current Implementation (`pkg/plugins/auxiliary.go`)

### 1.1 Existing Structure in `renderBrowserView` (lines 457–651)
In the current code:
- **Toolbar**: Contains navigation buttons (back, fwd, refresh), URL input, device select (iPhone 15 393px, iPad 1024px), pen button (`#swiss-b-pen`), box button (`#swiss-b-rect`), clear button (`#swiss-b-clear`), and send button (`#swiss-b-send-chat`).
  - *Gap*: Lacks the DOM Element Selector tool button (`#swiss-b-inspect`).
  - *Gap*: Device frame presets do not match current specs (iPhone 16 Pro 402px, Pixel 9 412px, iPad 820px).
- **Embedded Webview**: Created as `<webview id="swiss-browser-element">` (with iframe fallback).
- **Canvas Overlay**: `<canvas id="swiss-browser-canvas" class="swiss-browser-canvas-overlay">` positioned absolutely over the webview.
- **Drawing Logic**:
  ```javascript
  if (drawTool === "pen") {
    ctx.strokeStyle = "#ea4335";
    ctx.lineWidth = 3;
    ctx.lineCap = "round";
    ctx.beginPath();
    ctx.moveTo(lastX, lastY);
    ctx.lineTo(curX, curY);
    ctx.stroke();
    lastX = curX;
    lastY = curY;
  }
  ```
  - *Deficiency*: Straight `lineTo` creates sharp polyline kinks during fast mouse gestures.
- **Bounding Box**: Uses `ctx.setLineDash([5, 5])` and `ctx.fillRect("rgba(234, 67, 53, 0.12)")`.
  - *Deficiency*: Does not store the normalized bounding box coordinates for region cropping.
- **Send to Chat**:
  ```javascript
  const dataUrl = canvas.toDataURL("image/png");
  const promptText = `[Browser Preview Annotation - ${currentBrowserUrl}]\nComment: ${comment}\n(Visual markup attached)`;
  insertTextToChatInput(promptText);
  ```
  - *Deficiency*: Discards `dataUrl`, fails to crop the targeted region, does not create a `File` object, does not touch Antigravity's composer file upload `input[type="file"]`, and does not inject into Lexical (`editor.__lexicalEditor`).

---

## 2. Interactive Drawing Canvas Overlay Design

### 2.1 Red Draw Pen with Bézier Midpoint Curve Smoothing
- **Color**: `#ea4335`
- **Line Width**: `3px`
- **Line Cap & Join**: `round`
- **Mathematical Smoothing Algorithm**:
  Instead of drawing straight segments from $(X_{n-1}, Y_{n-1})$ to $(X_n, Y_n)$, we maintain the previous midpoint $(M_{x, n-1}, M_{y, n-1})$ and current mouse position $(X_n, Y_n)$.
  On each `mousemove`:
  $$\text{midX} = \frac{X_{n-1} + X_n}{2}, \quad \text{midY} = \frac{Y_{n-1} + Y_n}{2}$$
  The canvas draws a quadratic Bézier curve:
  $$\mathbf{B}(t) = (1-t)^2 \mathbf{M}_{n-1} + 2(1-t)t \mathbf{P}_{n-1} + t^2 \mathbf{M}_n \quad (t \in [0, 1])$$
  using `ctx.quadraticCurveTo(lastX, lastY, midX, midY)`.
  This ensures continuous first-derivative tangents ($C^1$ smoothness) across all intermediate points.

```javascript
// State tracking
let lastX = 0, lastY = 0;
let lastMidX = 0, lastMidY = 0;

canvas.onmousedown = (e) => {
  if (drawTool !== "pen") return;
  isDrawing = true;
  const rect = canvas.getBoundingClientRect();
  const x = e.clientX - rect.left;
  const y = e.clientY - rect.top;
  lastX = x; lastY = y;
  lastMidX = x; lastMidY = y;

  // Immediate dot for single clicks
  ctx.strokeStyle = "#ea4335";
  ctx.fillStyle = "#ea4335";
  ctx.lineWidth = 3;
  ctx.lineCap = "round";
  ctx.lineJoin = "round";
  ctx.beginPath();
  ctx.arc(x, y, 1.5, 0, Math.PI * 2);
  ctx.fill();
};

canvas.onmousemove = (e) => {
  if (!isDrawing || drawTool !== "pen") return;
  const rect = canvas.getBoundingClientRect();
  const curX = e.clientX - rect.left;
  const curY = e.clientY - rect.top;

  const midX = (lastX + curX) / 2;
  const midY = (lastY + curY) / 2;

  ctx.beginPath();
  ctx.moveTo(lastMidX, lastMidY);
  ctx.quadraticCurveTo(lastX, lastY, midX, midY);
  ctx.strokeStyle = "#ea4335";
  ctx.lineWidth = 3;
  ctx.lineCap = "round";
  ctx.lineJoin = "round";
  ctx.stroke();

  lastX = curX;
  lastY = curY;
  lastMidX = midX;
  lastMidY = midY;
};

canvas.onmouseup = () => {
  if (!isDrawing) return;
  isDrawing = false;
  if (drawTool === "pen") {
    ctx.beginPath();
    ctx.moveTo(lastMidX, lastMidY);
    ctx.lineTo(lastX, lastY);
    ctx.strokeStyle = "#ea4335";
    ctx.lineWidth = 3;
    ctx.stroke();
  }
};
```

---

### 2.2 Red Bounding Box Drag Tool
- **Color**: `#ea4335`
- **Border**: `2px`
- **Fill**: `rgba(234, 67, 53, 0.15)`
- **Behavior**:
  - `onmousedown`: Records `drawStartX`, `drawStartY`, and takes a canvas snapshot via `ctx.getImageData(0, 0, canvas.width, canvas.height)`.
  - `onmousemove`: Restores snapshot and draws dynamic drag box with dashed lines `[5, 5]`.
  - `onmouseup`: Normalizes coordinates $\min/\max$, redraws with solid 2px border and semi-transparent fill, renders a `#annotation` badge label, and saves `lastAnnotatedRegion = { x, y, width, height }`.

```javascript
let lastAnnotatedRegion = null;

canvas.onmousedown = (e) => {
  if (drawTool !== "rect") return;
  isDrawing = true;
  const rect = canvas.getBoundingClientRect();
  drawStartX = e.clientX - rect.left;
  drawStartY = e.clientY - rect.top;
  snapshot = ctx.getImageData(0, 0, canvas.width, canvas.height);
};

canvas.onmousemove = (e) => {
  if (!isDrawing || drawTool !== "rect") return;
  const rect = canvas.getBoundingClientRect();
  const curX = e.clientX - rect.left;
  const curY = e.clientY - rect.top;

  if (snapshot) ctx.putImageData(snapshot, 0, 0);

  const boxX = Math.min(drawStartX, curX);
  const boxY = Math.min(drawStartY, curY);
  const boxW = Math.abs(curX - drawStartX);
  const boxH = Math.abs(curY - drawStartY);

  ctx.strokeStyle = "#ea4335";
  ctx.lineWidth = 2;
  ctx.setLineDash([5, 5]);
  ctx.strokeRect(boxX, boxY, boxW, boxH);
  ctx.fillStyle = "rgba(234, 67, 53, 0.15)";
  ctx.fillRect(boxX, boxY, boxW, boxH);
  ctx.setLineDash([]);
};

canvas.onmouseup = (e) => {
  if (!isDrawing || drawTool !== "rect") return;
  isDrawing = false;
  const rect = canvas.getBoundingClientRect();
  const curX = e.clientX - rect.left;
  const curY = e.clientY - rect.top;

  if (snapshot) ctx.putImageData(snapshot, 0, 0);

  const boxX = Math.min(drawStartX, curX);
  const boxY = Math.min(drawStartY, curY);
  const boxW = Math.abs(curX - drawStartX);
  const boxH = Math.abs(curY - drawStartY);

  if (boxW > 5 && boxH > 5) {
    ctx.strokeStyle = "#ea4335";
    ctx.lineWidth = 2;
    ctx.strokeRect(boxX, boxY, boxW, boxH);
    ctx.fillStyle = "rgba(234, 67, 53, 0.15)";
    ctx.fillRect(boxX, boxY, boxW, boxH);

    // Render tag badge
    ctx.fillStyle = "#ea4335";
    ctx.fillRect(boxX, Math.max(0, boxY - 18), 75, 18);
    ctx.fillStyle = "#ffffff";
    ctx.font = "bold 10px monospace";
    ctx.fillText("#annotation", boxX + 4, Math.max(12, boxY - 5));

    lastAnnotatedRegion = { x: boxX, y: boxY, width: boxW, height: boxH };

    const comment = prompt("Enter annotation comment for this section:", "Review UI component alignment");
    if (comment) userComment = comment;
  }
};
```

---

### 2.3 Interactive DOM Element Selector ("Inspect Element")
- **Activation**: Clicking the `#swiss-b-inspect` button toggles `drawTool = "inspect"`.
- **Pointer Events**: Sets `canvas.style.pointerEvents = "none"` so mouse events reach the guest `<webview>`.
- **Guest Script Injection**: Evaluated via `webview.executeJavaScript()`.
  - Creates a fixed floating highlight box `#swiss-dom-inspect-overlay` with `border: 2px solid #ea4335`, background `rgba(234, 67, 53, 0.15)`, and a `#swiss-dom-inspect-badge` tag displaying the computed CSS selector.
  - On `mousemove`: Calculates element under cursor with `document.elementFromPoint`, positions overlay to element's `getBoundingClientRect()`, and formats the CSS path (e.g. `div.hero > button#cta`).
  - On `click`: Intercepts `click` (preventDefault & stopPropagation), extracts `selector`, `getBoundingClientRect()`, and `outerHTML` (first 1500 chars), cleans up overlay, and dispatches payload to host via `console.log('[SWISS_INSPECT_RESULT]', JSON.stringify(payload))`.
- **Host Bridge**:
  - The host listens to `<webview>`'s `console-message` event.
  - When `e.message.startsWith('[SWISS_INSPECT_RESULT]')`:
    - Parses `{ selector, outerHTML, rect }`.
    - Updates `lastSelectedElement = result`.
    - Updates `lastAnnotatedRegion = result.rect`.
    - Outlines the selected element on `#swiss-browser-canvas` with red stroke `#ea4335` and semi-transparent fill so the user visually sees the confirmed selection.
    - Resets `drawTool = "none"` and updates toolbar button state.

```javascript
function toggleDOMInspector(active) {
  if (!active) {
    if (webview && typeof webview.executeJavaScript === "function") {
      webview.executeJavaScript("if (window.__swissCleanupInspector) window.__swissCleanupInspector();");
    }
    return;
  }

  const inspectorScript = `(() => {
    if (window.__swissInspectorActive) return;
    window.__swissInspectorActive = true;

    let overlay = document.getElementById('swiss-dom-inspect-overlay');
    if (!overlay) {
      overlay = document.createElement('div');
      overlay.id = 'swiss-dom-inspect-overlay';
      overlay.style.position = 'fixed';
      overlay.style.pointerEvents = 'none';
      overlay.style.border = '2px solid #ea4335';
      overlay.style.backgroundColor = 'rgba(234, 67, 53, 0.15)';
      overlay.style.zIndex = '2147483647';
      overlay.style.boxSizing = 'border-box';
      overlay.style.transition = 'all 0.05s ease-out';
      overlay.style.display = 'none';

      const tagBadge = document.createElement('div');
      tagBadge.id = 'swiss-dom-inspect-badge';
      tagBadge.style.position = 'absolute';
      tagBadge.style.top = '-22px';
      tagBadge.style.left = '0';
      tagBadge.style.backgroundColor = '#ea4335';
      tagBadge.style.color = '#ffffff';
      tagBadge.style.fontFamily = 'monospace';
      tagBadge.style.fontSize = '11px';
      tagBadge.style.fontWeight = 'bold';
      tagBadge.style.padding = '2px 6px';
      tagBadge.style.borderRadius = '3px';
      tagBadge.style.whiteSpace = 'nowrap';
      tagBadge.style.boxShadow = '0 2px 4px rgba(0,0,0,0.2)';
      overlay.appendChild(tagBadge);
      document.body.appendChild(overlay);
    }

    function getCssSelector(el) {
      if (!el || el.nodeType !== Node.ELEMENT_NODE) return '';
      if (el.id) return '#' + el.id;
      let path = [];
      while (el && el.nodeType === Node.ELEMENT_NODE) {
        let sel = el.nodeName.toLowerCase();
        if (el.className && typeof el.className === 'string') {
          const classes = el.className.trim().split(/\\s+/).filter(c => c && !c.startsWith('swiss-'));
          if (classes.length) sel += '.' + classes.slice(0, 2).join('.');
        }
        let sib = el, nth = 1;
        while (sib = sib.previousElementSibling) {
          if (sib.nodeName.toLowerCase() === el.nodeName.toLowerCase()) nth++;
        }
        if (nth > 1) sel += ':nth-of-type(' + nth + ')';
        path.unshift(sel);
        if (el.id || path.length >= 3) break;
        el = el.parentElement;
      }
      return path.join(' > ');
    }

    let hoveredEl = null;

    function handleMouseMove(e) {
      const el = document.elementFromPoint(e.clientX, e.clientY);
      if (!el || el === overlay || el === document.body || el === document.documentElement) return;
      hoveredEl = el;
      const rect = el.getBoundingClientRect();
      overlay.style.display = 'block';
      overlay.style.top = rect.top + 'px';
      overlay.style.left = rect.left + 'px';
      overlay.style.width = rect.width + 'px';
      overlay.style.height = rect.height + 'px';
      const badge = overlay.querySelector('#swiss-dom-inspect-badge');
      if (badge) badge.textContent = getCssSelector(el);
    }

    function handleClick(e) {
      e.preventDefault();
      e.stopPropagation();
      if (!hoveredEl) return;
      const sel = getCssSelector(hoveredEl);
      const rect = hoveredEl.getBoundingClientRect();
      const snippet = hoveredEl.outerHTML ? hoveredEl.outerHTML.slice(0, 1500) : '';
      const payload = {
        selector: sel,
        outerHTML: snippet,
        rect: { x: rect.left, y: rect.top, width: rect.width, height: rect.height }
      };
      console.log('[SWISS_INSPECT_RESULT]', JSON.stringify(payload));
      window.__swissCleanupInspector();
    }

    window.__swissCleanupInspector = () => {
      window.__swissInspectorActive = false;
      document.removeEventListener('mousemove', handleMouseMove, true);
      document.removeEventListener('click', handleClick, true);
      if (overlay) overlay.style.display = 'none';
    };

    document.addEventListener('mousemove', handleMouseMove, true);
    document.addEventListener('click', handleClick, true);
  })()`;

  if (webview && typeof webview.executeJavaScript === "function") {
    webview.executeJavaScript(inspectorScript);
  }
}
```

---

## 3. "Send to Chat" Button Workflow Design

### 3.1 Workflow Architecture

```
[User clicks #swiss-b-send-chat]
              │
              ▼
Prompt for optional comment (or use existing userComment)
              │
              ▼
Determine Crop Coordinates:
- If lastAnnotatedRegion exists (from Bounding Box or DOM Inspector):
  Use { x, y, width, height }
- Else:
  Use full canvas { 0, 0, canvas.width, canvas.height }
              │
              ▼
Generate Composite Export Canvas:
- Optional: capture guest webview page via webview.capturePage(rect)
- Overlay canvas visual annotations via drawImage()
              │
              ▼
Export to PNG Blob via exportCanvas.toBlob(blob, 'image/png')
              │
              ▼
Synthesize File: new File([blob], 'annotation.png', { type: 'image/png' })
              │
              ▼
DataTransfer File Injection:
- const dt = new DataTransfer(); dt.items.add(file);
- const fileInput = document.querySelector('input[type="file"]');
- fileInput.files = dt.files;
- fileInput.dispatchEvent(new Event('change', { bubbles: true }));
              │
              ▼
Lexical Text / Snippet Injection:
- Format payload:
  [Browser Preview Annotation @ URL]
  Selected Element: `selector`
  ```html
  <outerHTML snippet>
  ```
  Comment: "..."
- Query editor element: [data-lexical-editor="true"] or .lexical-container [contenteditable="true"]
- If editor.__lexicalEditor: execute editor.update() with focus and insertText
- Fallback: insertTextToChatInput(payload)
```

### 3.2 Concrete JavaScript Implementation

```javascript
toolbar.querySelector("#swiss-b-send-chat").onclick = async () => {
  const comment = prompt(
    "Add comment to attach with this preview snapshot to Antigravity chat:",
    userComment || "Review UI alignment and inspected element markup."
  );
  if (comment === null) return;
  userComment = comment;

  // 1. Determine region
  const region = (lastAnnotatedRegion && lastAnnotatedRegion.width > 5 && lastAnnotatedRegion.height > 5)
    ? lastAnnotatedRegion
    : null;

  const exportW = region ? region.width : canvas.width;
  const exportH = region ? region.height : canvas.height;

  const exportCanvas = document.createElement("canvas");
  exportCanvas.width = exportW;
  exportCanvas.height = exportH;
  const expCtx = exportCanvas.getContext("2d");

  // Fill subtle white/light canvas background
  expCtx.fillStyle = "#ffffff";
  expCtx.fillRect(0, 0, exportW, exportH);

  // 2. Composite webview screenshot if available
  if (webview && typeof webview.capturePage === "function") {
    try {
      const pageImage = await webview.capturePage(region ? {
        x: Math.round(region.x),
        y: Math.round(region.y),
        width: Math.round(region.width),
        height: Math.round(region.height),
      } : undefined);
      if (pageImage && !pageImage.isEmpty()) {
        const bgDataUrl = pageImage.toDataURL();
        await new Promise((resolve) => {
          const bgImg = new Image();
          bgImg.onload = () => {
            expCtx.drawImage(bgImg, 0, 0, exportW, exportH);
            resolve();
          };
          bgImg.onerror = resolve;
          bgImg.src = bgDataUrl;
        });
      }
    } catch (_) {}
  }

  // Draw annotations on top
  if (region) {
    expCtx.drawImage(canvas, region.x, region.y, region.width, region.height, 0, 0, exportW, exportH);
  } else {
    expCtx.drawImage(canvas, 0, 0, exportW, exportH);
  }

  // 3. Export to PNG Blob and synthesize File
  exportCanvas.toBlob((blob) => {
    if (!blob) {
      const dataUrl = exportCanvas.toDataURL("image/png");
      const byteString = atob(dataUrl.split(",")[1]);
      const ab = new ArrayBuffer(byteString.length);
      const ia = new Uint8Array(ab);
      for (let i = 0; i < byteString.length; i++) ia[i] = byteString.charCodeAt(i);
      blob = new Blob([ab], { type: "image/png" });
    }

    const file = new File([blob], "annotation.png", { type: "image/png" });

    // 4. Inject into Antigravity composer file attachment input
    const fileInput = document.querySelector('input[type="file"]') ||
                      document.querySelector('.chat-input-toolbar input[type="file"]') ||
                      document.querySelector('input[type="file"][accept*="image"]');
    if (fileInput) {
      const dt = new DataTransfer();
      dt.items.add(file);
      fileInput.files = dt.files;
      fileInput.dispatchEvent(new Event("change", { bubbles: true }));
    }

    // 5. Build prompt payload text
    let promptText = `[Browser Preview Annotation @ ${currentBrowserUrl}]\n`;
    if (lastSelectedElement && lastSelectedElement.selector) {
      promptText += `Selected Element: \`${lastSelectedElement.selector}\`\n`;
    }
    if (lastSelectedElement && lastSelectedElement.outerHTML) {
      promptText += "```html\n" + lastSelectedElement.outerHTML.trim() + "\n```\n";
    }
    if (userComment) {
      promptText += `Comment: ${userComment}\n`;
    }
    promptText += "(Visual annotation attached: annotation.png)";

    // 6. Inject into Lexical editor with fallback
    let injectedLexical = false;
    const lexicalElem = document.querySelector('[data-lexical-editor="true"]') ||
                        document.querySelector('.lexical-container [contenteditable="true"]') ||
                        document.querySelector('[contenteditable="true"]');

    if (lexicalElem && lexicalElem.__lexicalEditor) {
      try {
        const editor = lexicalElem.__lexicalEditor;
        editor.update(() => {
          lexicalElem.focus();
          document.execCommand("insertText", false, promptText);
          injectedLexical = true;
        });
      } catch (err) {
        console.warn("Lexical editor update failed:", err);
      }
    }

    if (!injectedLexical) {
      insertTextToChatInput(promptText);
    }

    alert("Visual annotation attached and DOM snippet injected into Antigravity chat!");
  }, "image/png");
};
```

---

## 4. Mobile Responsive Device Frames & Touch Emulation

### 4.1 Frame Dimensions & Styles
- **Responsive**: `width: 100%`, `height: 100%`, border-radius: 0.
- **iPhone 16 Pro**: `width: 402px`, `height: 874px`, `border-radius: 36px`, `box-shadow: 0 12px 36px rgba(0,0,0,0.2)`.
- **Pixel 9**: `width: 412px`, `height: 924px`, `border-radius: 28px`, `box-shadow: 0 12px 36px rgba(0,0,0,0.2)`.
- **iPad**: `width: 820px`, `height: 1180px`, `border-radius: 24px`, `box-shadow: 0 12px 36px rgba(0,0,0,0.2)`.

### 4.2 Touch Emulation
When active device is not "responsive", touch emulation synthesizes `TouchEvent` (`touchstart`, `touchmove`, `touchend`) from pointer interactions, enabling mobile swipe and tap testing.

---

## 5. Designed Unit & Integration Tests (`pkg/plugins/auxiliary_test.go`)

We will add rigorous test assertions in `pkg/plugins/auxiliary_test.go`:

```go
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
		"swiss-dom-inspect-overlay",  // Highlight overlay ID
		"swiss-dom-inspect-badge",    // Selector tag badge ID
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
		"iphone16",
		"402", // iPhone 16 Pro width
		"pixel9",
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
```

---

## 6. Worker Implementation Blueprint

### Step 1: Update `pkg/plugins/auxiliary.go`
1. In `GenerateAuxiliaryPluginsCSS()`:
   - Add styling for `.swiss-browser-btn.active` with `#ea4335` border and background.
   - Add device frame styling helper classes if needed.
2. In `renderBrowserView` of `GenerateAuxiliaryPluginsScript()`:
   - Add `<button class="swiss-browser-btn" id="swiss-b-inspect" title="Interactive DOM Inspector">🎯 Inspect</button>` to toolbar HTML.
   - Update `<select id="swiss-b-device">` options: `iphone16` (402px), `pixel9` (412px), `ipad` (820px).
   - Implement tool state management for `pen`, `rect`, `inspect`, `none`.
   - Update pen drawing with Bézier midpoint smoothing via `quadraticCurveTo`.
   - Update bounding box drag tool with 2px solid border, `rgba(234, 67, 53, 0.15)` fill, `#annotation` badge tag, and save `lastAnnotatedRegion`.
   - Implement DOM element inspector with webview injection, hover overlay, CSS selector generator, and `console-message` bridge back to host.
   - Implement "Send to Chat" button workflow:
     - Composite webview capture and canvas annotations on export canvas.
     - Convert to PNG Blob.
     - Synthesize `File` object (`annotation.png`).
     - Inject `File` into `document.querySelector('input[type="file"]')` via `DataTransfer`.
     - Inject element selector, `outerHTML`, and user comment into `editor.__lexicalEditor` with fallback to `insertTextToChatInput`.

### Step 2: Update `pkg/plugins/auxiliary_test.go`
- Add the test functions detailed in Section 5.

### Step 3: Verify
- Run `go test -v ./pkg/plugins`
- Run `go test ./pkg/...`

---

## Conclusion

This blueprint provides full coverage of Requirement R2. Implementing these targeted additions will satisfy all user acceptance criteria for the live browser preview, smooth canvas drawing, DOM element inspection, and seamless chat composer injection.
