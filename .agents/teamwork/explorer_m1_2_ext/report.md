# Ext-M1 Exploration Report: R2 Live Browser Preview & Device Frames

**Explorer**: `explorer_m1_2_ext` (teamwork_preview_explorer)  
**Date**: 2026-10-05T22:35:00Z  
**Target Milestone**: Ext-M1 (Requirement R2: Live Browser Preview & Mobile Device Frames)  
**Working Directory**: `/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_m1_2_ext`  

---

## Executive Summary

This report establishes the complete architectural analysis, design specifications, test assertions, and concrete implementation blueprint for **Requirement R2 (Live Browser Preview & Device Frames)** in Google Antigravity 2.0's right auxiliary panel.

Key deliverables designed herein:
1. **Embedded `<webview>` Container with Graceful `<iframe>` Fallback**:
   - Tag configured with `partition="persist:swiss-browser"`, `allowpopups="true"`, and `webpreferences="allowRunningInsecureContent=yes, webSecurity=no"` to eliminate CORS and cross-origin blocking on local development servers.
   - Robust detection and fallback to sandboxed `<iframe>` when running in browser or non-webview contexts.
2. **Navigation Toolbar & Quick Port Shortcuts**:
   - Navigation controls (Back, Forward, Reload, URL address bar with auto-protocol prefixing).
   - Dedicated Quick Port Shortcuts bar for common development servers: `:5173` (Vite), `:3000` (React/Next), `:8080` (Web Dev), `:8765` (Swiss Daemon), `:4173` (Preview), plus interactive `+ Port` custom port prompt.
3. **Realistic Mobile Responsive Device Frames**:
   - **iPhone 16 Pro** (`402 × 874 px`): 11px titanium bezel, 54px corner radius, Dynamic Island hardware notch (`120 × 35 px`), and home indicator bar.
   - **Pixel 9** (`412 × 924 px`): 10px obsidian bezel, 42px corner radius, top-center punch-hole camera (`12 × 12 px`), and gesture navigation bar.
   - **iPad** (`820 × 1180 px`): 14px space gray bezel, 30px corner radius, top camera lens (`6 × 6 px`), and bottom home bar.
   - **Responsive / Desktop**: Fluid 100% × 100% viewport with borderless layout.
4. **Dynamic Scaling & Fit System**:
   - Auto-fit mode (`fit`) dynamically calculating scale factor against auxiliary panel bounds (`scale(factor)`) to prevent horizontal clipping, plus fixed zoom steps (`100%`, `75%`, `50%`).
   - Coordinate transformation ensuring canvas annotations map 1:1 regardless of device CSS scaling.
5. **Touch Emulation System**:
   - Dedicated `#swiss-b-touch` toggle with visual circular touch cursor styling.
   - Injected synthetic `TouchEvent` dispatcher (`touchstart`, `touchmove`, `touchend`) translating mouse actions into native touch events.
6. **Test Suite in `pkg/plugins/auxiliary_test.go`**:
   - Complete assertions verifying CSS device frame styles, notch classes, port shortcut attributes, security partitions, and touch handlers.

---

## 1. Architectural Inspection of `pkg/plugins/auxiliary.go`

### 1.1 Current Implementation State
Inspection of lines 457–556 in `pkg/plugins/auxiliary.go` reveals:
```javascript
// Existing renderBrowserView implementation:
function renderBrowserView(container) {
  const wrap = document.createElement("div");
  wrap.className = "swiss-browser-view";

  const toolbar = document.createElement("div");
  toolbar.className = "swiss-browser-toolbar";
  toolbar.innerHTML = `
    <button class="swiss-browser-btn" id="swiss-b-back" title="Back">←</button>
    <button class="swiss-browser-btn" id="swiss-b-fwd" title="Forward">→</button>
    <button class="swiss-browser-btn" id="swiss-b-refresh" title="Reload">↻</button>
    <input type="text" class="swiss-browser-url-input" id="swiss-b-url" value="${currentBrowserUrl}" />
    <select class="swiss-browser-btn" id="swiss-b-device" style="outline:none;">
      <option value="responsive">Responsive</option>
      <option value="iphone">iPhone 15 (393px)</option>
      <option value="ipad">iPad Pro (1024px)</option>
    </select>
    ...
  `;
  ...
  let webview = document.createElement("webview");
  if (typeof webview.reload !== "function") {
    webview = document.createElement("iframe");
  }
  webview.id = "swiss-browser-element";
  webview.src = currentBrowserUrl;
  webview.setAttribute("allowpopups", "true");
  webview.setAttribute("webpreferences", "allowRunningInsecureContent=yes");
  ...
}
```

### 1.2 Identified Gaps & Deficiencies
1. **Missing Port Shortcuts**:
   - The toolbar has no port chips or shortcuts. Developers must manually type `http://localhost:5173` or `http://localhost:3000` every time.
2. **Missing `partition="persist:swiss-browser"` & Incomplete Security Flags**:
   - Current `<webview>` sets `allowRunningInsecureContent=yes` but omits `partition="persist:swiss-browser"` and `webSecurity=no`.
   - Without `partition="persist:swiss-browser"`, cookies and session storage either leak into default partition or reset across launches.
   - Without `webSecurity=no`, cross-origin resource requests between local servers (e.g. frontend on `:5173` fetching backend on `:8080`) are blocked by Chromium's CORS enforcer.
3. **Outdated & Inaccurate Device Frames**:
   - Device selector only offers generic "iPhone 15 (393px)" and "iPad Pro (1024px)" without height constraints.
   - Missing required devices: **iPhone 16 Pro (402 × 874 px)**, **Pixel 9 (412 × 924 px)**, and **iPad (820 × 1180 px)**.
   - No realistic styling: frames lack bezels, shadows, Dynamic Island, punch-hole cutouts, and home bars.
4. **No Panel Fit / Scaling**:
   - Because Antigravity's right auxiliary panel is frequently narrower than 820px (or even 412px), fixed-dimension frames overflow horizontally and clip off-screen.
5. **No Touch Emulation**:
   - Mobile previews still receive standard desktop mouse events, preventing testing of mobile swipe, touch menus, and gesture listeners.

---

## 2. Embedded `<webview>` & Graceful `<iframe>` Fallback Design

### 2.1 Specification & Security Attributes
The embedded browser element must satisfy:
1. **Partition Isolation**: `partition="persist:swiss-browser"`
   - Stores cookies, `localStorage`, `sessionStorage`, and cache in a dedicated persistent partition.
   - Persists dev login states across Antigravity app restarts while keeping Antigravity's host IDE cookies 100% isolated.
2. **CORS & Insecure Content Relaxation**:
   - `webpreferences="allowRunningInsecureContent=yes, webSecurity=no, contextIsolation=no, nodeIntegration=no"`
   - Allows loading `http://localhost:*` without mixed-content warnings.
   - Allows AJAX/fetch calls to any local port without CORS preflight failures.
   - Disables Node.js access for untrusted web pages (`nodeIntegration=no`).
3. **Popups**: `allowpopups="true"` to support OAuth popups and secondary windows.

### 2.2 Dual Engine (Webview vs Iframe) Detection
```javascript
let isWebview = true;
let browserEl;
try {
  browserEl = document.createElement("webview");
  // Test if webview custom element is functional in current Electron context
  if (typeof browserEl.loadURL !== "function" && typeof browserEl.reload !== "function" && typeof process === "undefined") {
    browserEl = document.createElement("iframe");
    isWebview = false;
  }
} catch (e) {
  browserEl = document.createElement("iframe");
  isWebview = false;
}

browserEl.id = "swiss-browser-element";
browserEl.style.width = "100%";
browserEl.style.height = "100%";
browserEl.style.border = "none";

if (isWebview) {
  browserEl.setAttribute("partition", "persist:swiss-browser");
  browserEl.setAttribute("allowpopups", "true");
  browserEl.setAttribute("webpreferences", "allowRunningInsecureContent=yes, webSecurity=no, contextIsolation=no, nodeIntegration=no");
  browserEl.src = currentBrowserUrl;
} else {
  browserEl.setAttribute("sandbox", "allow-scripts allow-same-origin allow-forms allow-popups allow-modals");
  browserEl.setAttribute("allow", "cross-origin-isolated");
  browserEl.src = currentBrowserUrl;
}
```

### 2.3 Navigation Controller & Event Normalization
```javascript
function navigateBrowser(url) {
  let finalUrl = url.trim();
  if (!/^https?:\/\//i.test(finalUrl)) {
    if (/^(localhost|\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})(:\d+)?/i.test(finalUrl)) {
      finalUrl = "http://" + finalUrl;
    } else {
      finalUrl = "https://" + finalUrl;
    }
  }
  currentBrowserUrl = finalUrl;
  const urlInput = document.getElementById("swiss-b-url");
  if (urlInput) urlInput.value = finalUrl;

  if (isWebview && typeof browserEl.loadURL === "function") {
    browserEl.loadURL(finalUrl);
  } else {
    browserEl.src = finalUrl;
  }
  updateActivePortChip(finalUrl);
}

// Lifecycle event synchronization
if (isWebview) {
  browserEl.addEventListener("did-start-loading", () => {
    const refreshBtn = document.getElementById("swiss-b-refresh");
    if (refreshBtn) refreshBtn.classList.add("loading");
  });
  browserEl.addEventListener("did-stop-loading", () => {
    const refreshBtn = document.getElementById("swiss-b-refresh");
    if (refreshBtn) refreshBtn.classList.remove("loading");
    if (isTouchEmulationActive) injectTouchEmulation(browserEl, true);
  });
  browserEl.addEventListener("did-navigate", (e) => {
    currentBrowserUrl = e.url;
    const urlInput = document.getElementById("swiss-b-url");
    if (urlInput) urlInput.value = e.url;
    updateActivePortChip(e.url);
  });
  browserEl.addEventListener("did-navigate-in-page", (e) => {
    currentBrowserUrl = e.url;
    const urlInput = document.getElementById("swiss-b-url");
    if (urlInput) urlInput.value = e.url;
  });
}
```

---

## 3. Navigation Toolbar & Port Shortcuts Design

### 3.1 Two-Tier Toolbar Structure
The toolbar consists of two coordinated rows:
1. **Tier 1 (Main Navigation Bar)**:
   - History & Reload: `#swiss-b-back` (`←`), `#swiss-b-fwd` (`→`), `#swiss-b-refresh` (`↻`)
   - URL Address Input: `#swiss-b-url` (`class="swiss-browser-url-input"`)
   - Device Selector: `#swiss-b-device`
   - Fit & Zoom Selector: `#swiss-b-scale`
   - Touch Emulation Toggle: `#swiss-b-touch`
   - Annotation Actions: `#swiss-b-pen`, `#swiss-b-rect`, `#swiss-b-inspect`, `#swiss-b-clear`, `#swiss-b-send-chat`
2. **Tier 2 (Quick Port Shortcuts Bar)**:
   - Container: `.swiss-browser-port-bar`
   - Label: `<span class="swiss-port-label">Quick Ports:</span>`
   - Port Chips (`.swiss-port-chip`):
     - `:5173` (Vite dev server)
     - `:3000` (React / Next.js dev server)
     - `:8080` (Generic Web / Java / Vue server)
     - `:8765` (Antigravity Swiss Knife companion daemon)
     - `:4173` (Vite preview build)
     - `+ Port` (Custom prompt dialog for any port e.g. 8000, 5000, 4200)

### 3.2 HTML Template for Toolbar
```html
<div class="swiss-browser-toolbar">
  <div class="swiss-browser-nav-row">
    <button class="swiss-browser-btn" id="swiss-b-back" title="Back">←</button>
    <button class="swiss-browser-btn" id="swiss-b-fwd" title="Forward">→</button>
    <button class="swiss-browser-btn" id="swiss-b-refresh" title="Reload">↻</button>
    <input type="text" class="swiss-browser-url-input" id="swiss-b-url" value="${currentBrowserUrl}" placeholder="http://localhost:5173" />
    <select class="swiss-browser-btn" id="swiss-b-device" title="Device Frame">
      <option value="responsive">Responsive / Desktop</option>
      <option value="iphone-16-pro">iPhone 16 Pro (402×874)</option>
      <option value="pixel-9">Pixel 9 (412×924)</option>
      <option value="ipad">iPad (820×1180)</option>
    </select>
    <select class="swiss-browser-btn" id="swiss-b-scale" title="Viewport Scale">
      <option value="fit" selected>Fit Screen</option>
      <option value="1">100%</option>
      <option value="0.75">75%</option>
      <option value="0.5">50%</option>
    </select>
    <button class="swiss-browser-btn" id="swiss-b-touch" title="Toggle Touch Emulation">👆 Touch</button>
    <button class="swiss-browser-btn" id="swiss-b-pen" title="Red Pen Drawing">✏️ Pen</button>
    <button class="swiss-browser-btn" id="swiss-b-rect" title="Red Box Annotation">□ Box</button>
    <button class="swiss-browser-btn" id="swiss-b-clear" title="Clear Annotations">✕</button>
    <button class="swiss-browser-btn primary" id="swiss-b-send-chat" title="Send to Antigravity Chat">💬 Send to Chat</button>
  </div>
  <div class="swiss-browser-port-bar">
    <span class="swiss-port-label">Quick Ports:</span>
    <button class="swiss-port-chip" data-port="5173" title="Vite Development Server">:5173</button>
    <button class="swiss-port-chip" data-port="3000" title="React / Next.js Server">:3000</button>
    <button class="swiss-port-chip" data-port="8080" title="Standard Web Server">:8080</button>
    <button class="swiss-port-chip" data-port="8765" title="Antigravity Swiss Knife">:8765</button>
    <button class="swiss-port-chip" data-port="4173" title="Vite Production Preview">:4173</button>
    <button class="swiss-port-chip custom" data-port="custom" title="Connect to custom local port">+ Port</button>
  </div>
</div>
```

### 3.3 Port Shortcut Handlers
```javascript
toolbar.querySelectorAll(".swiss-port-chip").forEach(chip => {
  chip.onclick = () => {
    const port = chip.dataset.port;
    if (port === "custom") {
      const customPort = prompt("Enter local port number (e.g. 8000, 4200, 5000):", "8000");
      if (customPort && /^\d+$/.test(customPort.trim())) {
        navigateBrowser(`http://localhost:${customPort.trim()}`);
      }
    } else {
      navigateBrowser(`http://localhost:${port}`);
    }
  };
});

function updateActivePortChip(url) {
  const match = url.match(/^http:\/\/(?:localhost|127\.0\.0\.1):(\d+)/i);
  const activePort = match ? match[1] : null;
  toolbar.querySelectorAll(".swiss-port-chip").forEach(chip => {
    chip.classList.toggle("active", chip.dataset.port === activePort);
  });
}
```

---

## 4. Mobile Responsive Device Frames & Realistic Styling

### 4.1 Device Dimension Specifications
| Device Identifier | Label | Width | Height | Bezel Specification | Corner Radius (Frame / Screen) | Hardware Cutout | Home Bar |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `iphone-16-pro` | iPhone 16 Pro | 402px | 874px | 11px solid `#1a1a1c` | 54px / 43px | Dynamic Island (`120 × 35 px`, radius 18px) | 134px × 5px |
| `pixel-9` | Pixel 9 | 412px | 924px | 10px solid `#202124` | 42px / 32px | Center Punch Hole (`12 × 12 px` circle) | 110px × 4px |
| `ipad` | iPad | 820px | 1180px | 14px solid `#2c2d30` | 30px / 16px | Top Camera Lens (`6 × 6 px` dot) | 160px × 5px |
| `responsive` | Responsive / Desktop | 100% | 100% | None | 0px / 0px | None | None |

### 4.2 DOM Hierarchy for Viewport & Frames
```
.swiss-browser-viewport-wrap
└── .swiss-device-stage
    └── .swiss-device-frame (class: frame-responsive | frame-iphone-16-pro | frame-pixel-9 | frame-ipad)
        ├── .swiss-device-notch (class: dynamic-island | punch-hole | ipad-camera)
        ├── .swiss-device-screen
        │   ├── <webview> / <iframe> (#swiss-browser-element)
        │   └── <canvas> (#swiss-browser-canvas, class="swiss-browser-canvas-overlay")
        └── .swiss-device-home-bar
```

### 4.3 Realistic CSS Implementation
```css
/* Stage Container */
.swiss-device-stage {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  min-height: 100%;
  width: 100%;
  box-sizing: border-box;
}

/* Base Device Frame */
.swiss-device-frame {
  position: relative;
  box-sizing: border-box;
  background: #ffffff;
  transition: transform 0.2s cubic-bezier(0.2, 0, 0, 1), width 0.2s, height 0.2s;
  flex-shrink: 0;
}

/* Screen Wrapper */
.swiss-device-screen {
  position: relative;
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: #ffffff;
}

/* iPhone 16 Pro (402 x 874 px) */
.swiss-device-frame.frame-iphone-16-pro {
  width: 424px; /* 402 + 22 bezel */
  height: 896px; /* 874 + 22 bezel */
  border: 11px solid #1a1a1c;
  border-radius: 54px;
  box-shadow: 0 25px 65px -15px rgba(0, 0, 0, 0.4), 0 0 0 1px rgba(255, 255, 255, 0.12) inset;
}
.swiss-device-frame.frame-iphone-16-pro .swiss-device-screen {
  width: 402px;
  height: 874px;
  border-radius: 43px;
}
.swiss-device-notch.dynamic-island {
  position: absolute;
  top: 12px;
  left: 50%;
  transform: translateX(-50%);
  width: 120px;
  height: 35px;
  background: #000000;
  border-radius: 18px;
  z-index: 60;
  pointer-events: none;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.6);
}

/* Pixel 9 (412 x 924 px) */
.swiss-device-frame.frame-pixel-9 {
  width: 432px; /* 412 + 20 bezel */
  height: 944px; /* 924 + 20 bezel */
  border: 10px solid #202124;
  border-radius: 42px;
  box-shadow: 0 25px 65px -15px rgba(0, 0, 0, 0.4), 0 0 0 1px rgba(255, 255, 255, 0.1) inset;
}
.swiss-device-frame.frame-pixel-9 .swiss-device-screen {
  width: 412px;
  height: 924px;
  border-radius: 32px;
}
.swiss-device-notch.punch-hole {
  position: absolute;
  top: 14px;
  left: 50%;
  transform: translateX(-50%);
  width: 12px;
  height: 12px;
  background: #0a0a0c;
  border-radius: 50%;
  z-index: 60;
  pointer-events: none;
  box-shadow: 0 0 0 2px #202124;
}

/* iPad (820 x 1180 px) */
.swiss-device-frame.frame-ipad {
  width: 848px; /* 820 + 28 bezel */
  height: 1208px; /* 1180 + 28 bezel */
  border: 14px solid #2c2d30;
  border-radius: 30px;
  box-shadow: 0 30px 75px -20px rgba(0, 0, 0, 0.45), 0 0 0 1px rgba(255, 255, 255, 0.1) inset;
}
.swiss-device-frame.frame-ipad .swiss-device-screen {
  width: 820px;
  height: 1180px;
  border-radius: 16px;
}
.swiss-device-notch.ipad-camera {
  position: absolute;
  top: 6px;
  left: 50%;
  transform: translateX(-50%);
  width: 6px;
  height: 6px;
  background: #0a0a0c;
  border-radius: 50%;
  z-index: 60;
  pointer-events: none;
  box-shadow: 0 0 0 1px #3a3a3c;
}

/* Responsive / Desktop */
.swiss-device-frame.frame-responsive {
  width: 100%;
  height: 100%;
  border: none;
  border-radius: 0;
  box-shadow: none;
}
.swiss-device-frame.frame-responsive .swiss-device-screen {
  width: 100%;
  height: 100%;
  border-radius: 0;
}

/* Home Indicator Bar */
.swiss-device-home-bar {
  position: absolute;
  bottom: 8px;
  left: 50%;
  transform: translateX(-50%);
  height: 5px;
  background: rgba(0, 0, 0, 0.55);
  border-radius: 3px;
  z-index: 60;
  pointer-events: none;
}
.swiss-device-frame.frame-iphone-16-pro .swiss-device-home-bar { width: 134px; }
.swiss-device-frame.frame-pixel-9 .swiss-device-home-bar { width: 110px; height: 4px; }
.swiss-device-frame.frame-ipad .swiss-device-home-bar { width: 160px; }
```

### 4.4 Dynamic Auto-Fit Scaling System
Because auxiliary panels vary from 320px to 800px width, `applyDeviceScale` dynamically prevents clipping:
```javascript
function applyDeviceScale() {
  const frame = document.querySelector(".swiss-device-frame");
  const vpWrap = document.querySelector(".swiss-browser-viewport-wrap");
  const scaleSelect = document.getElementById("swiss-b-scale");
  if (!frame || !vpWrap || !scaleSelect) return;

  const scaleMode = scaleSelect.value;
  if (activeDevice === "responsive" || scaleMode === "1") {
    frame.style.transform = "none";
    return;
  }

  if (scaleMode === "fit") {
    const availW = vpWrap.clientWidth - 48;
    const availH = vpWrap.clientHeight - 48;
    let targetW = 424, targetH = 896;
    if (activeDevice === "iphone-16-pro") { targetW = 424; targetH = 896; }
    else if (activeDevice === "pixel-9") { targetW = 432; targetH = 944; }
    else if (activeDevice === "ipad") { targetW = 848; targetH = 1208; }

    const factor = Math.min(1, Math.min(availW / targetW, availH / targetH));
    frame.style.transform = `scale(${Math.max(0.2, factor.toFixed(3))})`;
    frame.style.transformOrigin = "top center";
  } else {
    const factor = parseFloat(scaleMode) || 1;
    frame.style.transform = `scale(${factor})`;
    frame.style.transformOrigin = "top center";
  }
}

// Window / panel resize observer
window.addEventListener("resize", applyDeviceScale);
```

### 4.5 Canvas Coordinate Translation under Scaling
When `frame` has `transform: scale(s)`, mouse coordinates on the annotation `<canvas>` must be normalized:
```javascript
function getCanvasCoords(canvas, e) {
  const rect = canvas.getBoundingClientRect();
  return {
    x: (e.clientX - rect.left) * (canvas.width / rect.width),
    y: (e.clientY - rect.top) * (canvas.height / rect.height)
  };
}
```
This formula completely cancels out CSS transforms, ensuring annotation coordinates align with visual screen pixels at any zoom level.

---

## 5. Touch Emulation System Design

### 5.1 Architecture & Activation
1. **Toolbar Button**: `#swiss-b-touch` (`👆 Touch`) toggles active state.
2. **Auto-Switching**:
   - Selecting `iphone-16-pro`, `pixel-9`, or `ipad` defaults touch emulation to enabled.
   - Selecting `responsive` reverts touch emulation to disabled.
3. **Screen Cursor Styling**:
   When active, adds `.touch-emulation-active` to `.swiss-device-screen`, rendering a translucent 24px circular finger touch point:
   ```css
   .swiss-device-screen.touch-emulation-active {
     cursor: url('data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10" fill="rgba(26,115,232,0.25)" stroke="%231a73e8" stroke-width="2"/><circle cx="12" cy="12" r="3" fill="%231a73e8"/></svg>') 12 12, auto;
   }
   ```

### 5.2 Touch Event Synthesizer Script
Injected into `<webview>` or `<iframe>` to map mouse drags into genuine W3C `TouchEvent` instances:
```javascript
function injectTouchEmulation(browserEl, enabled) {
  const code = `
    (() => {
      if (window.__swissTouchInstalled) {
        window.__swissTouchEnabled = ${enabled};
        return;
      }
      window.__swissTouchInstalled = true;
      window.__swissTouchEnabled = ${enabled};
      let isTouching = false;
      let id = 1;

      function createTouch(e) {
        return new Touch({
          identifier: id,
          target: e.target,
          clientX: e.clientX,
          clientY: e.clientY,
          screenX: e.screenX,
          screenY: e.screenY,
          pageX: e.pageX,
          pageY: e.pageY,
          radiusX: 12,
          radiusY: 12,
          rotationAngle: 0,
          force: 1.0
        });
      }

      function emit(type, e) {
        if (!window.__swissTouchEnabled) return;
        const touch = createTouch(e);
        const evt = new TouchEvent(type, {
          bubbles: true,
          cancelable: true,
          composed: true,
          touches: type === 'touchend' ? [] : [touch],
          targetTouches: type === 'touchend' ? [] : [touch],
          changedTouches: [touch]
        });
        e.target.dispatchEvent(evt);
      }

      document.addEventListener('mousedown', (e) => {
        if (!window.__swissTouchEnabled || e.button !== 0) return;
        isTouching = true;
        id++;
        emit('touchstart', e);
      }, true);

      document.addEventListener('mousemove', (e) => {
        if (!window.__swissTouchEnabled || !isTouching) return;
        emit('touchmove', e);
      }, true);

      document.addEventListener('mouseup', (e) => {
        if (!window.__swissTouchEnabled || !isTouching) return;
        isTouching = false;
        emit('touchend', e);
      }, true);
    })();
  `;

  if (isWebview && typeof browserEl.executeJavaScript === "function") {
    browserEl.executeJavaScript(code).catch(() => {});
  } else if (browserEl.contentWindow) {
    try {
      browserEl.contentWindow.eval(code);
    } catch (e) {}
  }
}
```

---

## 6. Test Suite Design in `pkg/plugins/auxiliary_test.go`

To verify all Ext-M1 R2 requirements independently, the Go test suite will test both CSS selectors/rules and JS script identifiers:

### 6.1 Assertions for `TestGenerateAuxiliaryPluginsCSS`
Add the following selectors to `requiredSelectors`:
```go
".swiss-browser-port-bar",
".swiss-port-chip",
".swiss-device-stage",
".swiss-device-frame",
".frame-iphone-16-pro",
".frame-pixel-9",
".frame-ipad",
".frame-responsive",
".swiss-device-notch",
".dynamic-island",
".punch-hole",
".ipad-camera",
".swiss-device-home-bar",
".swiss-device-screen",
".touch-emulation-active",
```

### 6.2 Assertions for `TestGenerateAuxiliaryPluginsScript`
Add the following identifiers to `requiredIdentifiers`:
```go
"partition=\"persist:swiss-browser\"",
"allowRunningInsecureContent=yes",
"webSecurity=no",
"swiss-b-port",
"swiss-port-chip",
"5173",
"3000",
"8080",
"8765",
"iphone-16-pro",
"402",
"874",
"pixel-9",
"412",
"924",
"ipad",
"820",
"1180",
"swiss-b-touch",
"TouchEvent",
"swiss-b-scale",
"applyDeviceScale",
```

### 6.3 Dedicated Test: `TestBrowserViewDeviceFramesAndPorts`
```go
func TestBrowserViewDeviceFramesAndPorts(t *testing.T) {
	css := GenerateAuxiliaryPluginsCSS()
	js := GenerateAuxiliaryPluginsScript()

	// Verify exact device dimensions in CSS/JS
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
		{"Port 5173 chip", js, "data-port=\"5173\""},
		{"Port 3000 chip", js, "data-port=\"3000\""},
		{"Port 8080 chip", js, "data-port=\"8080\""},
		{"Port 8765 chip", js, "data-port=\"8765\""},
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
```

---

## 7. Concrete Implementation Blueprint for the Worker

The Worker will implement the changes in two files:
1. `pkg/plugins/auxiliary.go`
2. `pkg/plugins/auxiliary_test.go`

### Step-by-Step Implementation Guide

#### Step 1: Update `GenerateAuxiliaryPluginsCSS()` in `pkg/plugins/auxiliary.go`
- Locate `.swiss-browser-toolbar` around line 54.
- Add CSS rules for `.swiss-browser-nav-row` and `.swiss-browser-port-bar`.
- Add CSS styling for `.swiss-port-chip`, `.swiss-port-label`, and `.swiss-port-chip.active`.
- Add CSS rules for `.swiss-device-stage`, `.swiss-device-frame`, `.swiss-device-screen`.
- Add realistic device frame classes:
  - `.frame-iphone-16-pro`, `.frame-pixel-9`, `.frame-ipad`, `.frame-responsive`.
  - Notch cutouts: `.swiss-device-notch`, `.dynamic-island`, `.punch-hole`, `.ipad-camera`.
  - Home bar: `.swiss-device-home-bar`.
- Add touch cursor styling: `.swiss-device-screen.touch-emulation-active`.

#### Step 2: Update `renderBrowserView()` in `pkg/plugins/auxiliary.go`
- Replace existing `toolbar.innerHTML` with two-tier template:
  - Navigation row: Back, Forward, Reload, URL Input, Device dropdown (`responsive`, `iphone-16-pro`, `pixel-9`, `ipad`), Scale dropdown (`fit`, `1`, `0.75`, `0.5`), Touch button (`#swiss-b-touch`), Drawing tools, Send to Chat.
  - Port shortcuts row: chips for `:5173`, `:3000`, `:8080`, `:8765`, `:4173`, and `+ Port`.
- Replace viewport markup:
  - Wrap frame in `.swiss-device-stage`.
  - Inside `.swiss-device-frame`, append `.swiss-device-notch`, `.swiss-device-screen`, and `.swiss-device-home-bar`.
- Create `<webview>` element with `partition="persist:swiss-browser"`, `allowpopups="true"`, and `webpreferences="allowRunningInsecureContent=yes, webSecurity=no"`.
- Implement graceful fallback to `<iframe>` with sandbox attributes.
- Wire navigation events:
  - Enter key on URL bar auto-prepends `http://`.
  - Port chips trigger instant navigation and highlight active port.
  - Back, Forward, Reload wired to webview/iframe methods.
- Implement device frame switcher (`applyDeviceFrame`):
  - Toggles CSS classes on `frameBox`.
  - Sets exact screen dimensions (`402 × 874`, `412 × 924`, `820 × 1180`, `100% × 100%`).
  - Calls `applyDeviceScale()` and `resizeCanvas()`.
- Implement dynamic scale calculator (`applyDeviceScale`) computing aspect ratio against `vpWrap.clientWidth` and `vpWrap.clientHeight`.
- Implement touch emulation toggle and script injector (`injectTouchEmulation`).
- Ensure canvas drawing functions use normalized coordinates (`(e.clientX - rect.left) * (canvas.width / rect.width)`).

#### Step 3: Update `pkg/plugins/auxiliary_test.go`
- Extend `requiredSelectors` in `TestGenerateAuxiliaryPluginsCSS`.
- Extend `requiredIdentifiers` in `TestGenerateAuxiliaryPluginsScript`.
- Add `TestBrowserViewDeviceFramesAndPorts(t *testing.T)`.
- Verify with `go test -v ./pkg/plugins/...`.

---

## 8. Verification Strategy
1. **Unit Tests**:
   - Run `go test -v ./pkg/plugins/...` to verify all CSS classes, script identifiers, and dimension checks pass.
2. **Build Verification**:
   - Run `go test ./pkg/... ./cmd/...` to verify zero regression across the Go backend.
   - Run `npm run build` in `frontend/` to verify frontend integrity.
3. **Persistent Script Sync**:
   - Run `swiss patch sync` (or verify styler integration) to confirm that the generated `persistent_script.js` contains the complete browser preview engine.
