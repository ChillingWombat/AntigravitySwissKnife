# Handoff Report: Ext-M1 R2 Live Browser Preview & Device Frames

**Agent**: `explorer_m1_2_ext` (teamwork_preview_explorer)  
**Date**: 2026-10-05T22:36:00Z  
**Handoff Type**: Hard (Task Complete)  
**Recipient**: Parent Orchestrator (`1e9124c8-4e7a-4fbd-80fe-96b480b57931`) / Worker  

---

## 1. Observation

1. **`pkg/plugins/auxiliary.go` lines 461–479**:
   Toolbar generation in `renderBrowserView` currently provides only back, forward, refresh, URL input, and a basic device dropdown:
   ```javascript
   <button class="swiss-browser-btn" id="swiss-b-back" title="Back">←</button>
   <button class="swiss-browser-btn" id="swiss-b-fwd" title="Forward">→</button>
   <button class="swiss-browser-btn" id="swiss-b-refresh" title="Reload">↻</button>
   <input type="text" class="swiss-browser-url-input" id="swiss-b-url" value="${currentBrowserUrl}" />
   <select class="swiss-browser-btn" id="swiss-b-device" style="outline:none;">
     <option value="responsive">Responsive</option>
     <option value="iphone">iPhone 15 (393px)</option>
     <option value="ipad">iPad Pro (1024px)</option>
   </select>
   ```
   There are zero port shortcuts (`:5173`, `:3000`, `:8080`, `:8765`), no custom port launcher, and the device resolutions are outdated ("iPhone 15 (393px)" and "iPad Pro (1024px)" instead of iPhone 16 Pro 402×874px, Pixel 9 412×924px, and iPad 820×1180px).

2. **`pkg/plugins/auxiliary.go` lines 490–502**:
   The embedded browser container currently uses:
   ```javascript
   let webview = document.createElement("webview");
   if (typeof webview.reload !== "function") {
     webview = document.createElement("iframe");
   }
   webview.id = "swiss-browser-element";
   webview.src = currentBrowserUrl;
   webview.style.width = "100%";
   webview.style.height = "100%";
   webview.style.border = "none";
   webview.setAttribute("allowpopups", "true");
   webview.setAttribute("webpreferences", "allowRunningInsecureContent=yes");
   ```
   The `<webview>` tag completely lacks `partition="persist:swiss-browser"`, lacks `webSecurity=no`, and lacks `contextIsolation=no, nodeIntegration=no`. Without `partition="persist:swiss-browser"`, cookies are either transient or cross-contaminate. Without `webSecurity=no`, local development servers making cross-port API calls encounter CORS friction.

3. **`pkg/plugins/auxiliary.go` lines 103–129 & 542–556**:
   The frame container `.swiss-browser-frame-container` has no device bezels, no realistic shadows, no hardware notches or Dynamic Island, no punch-hole camera, and no home indicator bars. Furthermore, there is no touch emulation toggle or handler.

4. **`pkg/plugins/auxiliary_test.go` lines 14–54**:
   Unit tests only check for 8 basic CSS selectors and 9 basic JS identifiers. None of the device frame dimensions, port shortcuts, security partitions, or touch emulation handlers are currently asserted in tests.

5. **`go test -v ./pkg/plugins/...`**:
   Exits with code 0 (`PASS`) in 0.00s. All existing tests pass.

---

## 2. Logic Chain

1. **Local Server Usability & CORS Requirement**:
   - Devs using Antigravity Swiss Knife test frontends on `:5173` or `:3000` connected to backends on `:8080` or `:8765`.
   - By adding `partition="persist:swiss-browser"` and `webpreferences="allowRunningInsecureContent=yes, webSecurity=no"`, Chromium's same-origin restrictions and mixed-content blocks are bypassed for local preview while maintaining isolated session storage.
   - By adding Quick Port Shortcuts (`:5173`, `:3000`, `:8080`, `:8765`, `:4173`, `+ Port`), one-click server switching eliminates manual URL typing and ensures instant connectivity.

2. **Mobile Device Fidelity & Panel Constraints**:
   - Mobile web apps require accurate screen dimensions: iPhone 16 Pro (`402 × 874 px`), Pixel 9 (`412 × 924 px`), and iPad (`820 × 1180 px`).
   - Adding realistic outer bezels (11px titanium, 10px obsidian, 14px space gray aluminum), hardware cutouts (Dynamic Island, punch hole, iPad lens), and home indicator bars provides authentic preview context.
   - Because Antigravity's auxiliary panel width is user-adjustable and typically ranges from 350px to 600px, fixed devices like iPad (820px) would clip off-screen. Adding `applyDeviceScale` with dynamic `fit` (`Math.min(availW / targetW, availH / targetH)`) prevents overflow while preserving 1:1 annotation accuracy via canvas coordinate normalization `(e.clientX - rect.left) * (canvas.width / rect.width)`.

3. **Touch Emulation Necessity**:
   - Mobile components (carousels, drawers, pull-to-refresh) listen to `touchstart`, `touchmove`, `touchend`.
   - By creating `#swiss-b-touch` and injecting a synthetic `TouchEvent` dispatcher script into the webview/iframe along with `.touch-emulation-active` cursor styling, mobile interactions can be tested without a physical mobile device.

4. **Testability & Verifiability**:
   - Expanding `pkg/plugins/auxiliary_test.go` with `TestBrowserViewDeviceFramesAndPorts` and updating selector tables guarantees that the Worker's implementation matches every required dimension and attribute.

---

## 3. Caveats

1. **Electron `webviewTag` Host Setting**:
   - `<webview>` tags in Electron require `webviewTag: true` in the host `webPreferences`. If Antigravity 2.0 has not enabled `webviewTag` in its BrowserWindow options, our graceful fallback to `<iframe>` ensures uninterrupted functionality.
2. **Third-Party Iframe Cross-Origin Scripting Restrictions**:
   - For remote websites loaded inside the `<iframe>` fallback (not `<webview>`), browsers enforce the Same-Origin Policy, preventing injected touch scripts from accessing cross-origin iframe DOM. However, for local servers (`http://localhost:*`) and when using `<webview>` with `webSecurity=no`, script injection succeeds unconditionally.

---

## 4. Conclusion

The design for Ext-M1 R2 Live Browser Preview & Device Frames is fully specified and ready for implementation by the Worker:
1. `pkg/plugins/auxiliary.go`:
   - `GenerateAuxiliaryPluginsCSS()`: Add `.swiss-browser-port-bar`, `.swiss-port-chip`, `.swiss-device-stage`, `.swiss-device-frame`, `.frame-iphone-16-pro`, `.frame-pixel-9`, `.frame-ipad`, `.swiss-device-notch`, `.dynamic-island`, `.punch-hole`, `.ipad-camera`, `.swiss-device-home-bar`, and `.touch-emulation-active`.
   - `renderBrowserView()`: Implement two-tier toolbar with navigation + port shortcuts (`:5173`, `:3000`, `:8080`, `:8765`, `:4173`, `+ Port`), scale selector (`fit`, `1`, `0.75`, `0.5`), device selector (`responsive`, `iphone-16-pro`, `pixel-9`, `ipad`), touch emulation toggle, `<webview>` with `partition="persist:swiss-browser"`, `allowpopups="true"`, `webpreferences="allowRunningInsecureContent=yes, webSecurity=no"`, iframe fallback, and normalized canvas coordinate translation.
2. `pkg/plugins/auxiliary_test.go`:
   - Add `TestBrowserViewDeviceFramesAndPorts` and expand `TestGenerateAuxiliaryPluginsCSS` / `TestGenerateAuxiliaryPluginsScript`.
3. Report and blueprint are published in `report.md`.

---

## 5. Verification Method

1. **Run Unit Tests**:
   ```bash
   go test -v ./pkg/plugins/...
   ```
   *Expected Result*: `TestGenerateAuxiliaryPluginsCSS`, `TestGenerateAuxiliaryPluginsScript`, and `TestBrowserViewDeviceFramesAndPorts` pass with 0 errors.

2. **Verify Full Project Build**:
   ```bash
   go test ./pkg/... ./cmd/...
   ```
   *Expected Result*: All Go unit and integration tests pass cleanly.

3. **Verify Generated Code Artifacts**:
   Inspect output of `plugins.GenerateAuxiliaryPluginsScript()` to ensure `partition="persist:swiss-browser"`, `webSecurity=no`, `iphone-16-pro`, `pixel-9`, `ipad`, `5173`, and `TouchEvent` are present.
