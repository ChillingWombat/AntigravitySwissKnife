# Forensic Integrity Audit Report: Milestone Ext-M1

**Auditor**: `auditor_m1_1_ext` (teamwork_preview_auditor)  
**Parent / Recipient**: `1e9124c8-4e7a-4fbd-80fe-96b480b57931`  
**Date**: 2026-10-05T23:13:00Z  
**Target**: Milestone Ext-M1 (Auxiliary Panel Tab Injector Engine & Live Browser Preview with Visual Annotation Canvas)  
**Integrity Mode**: `development` (read directly from `ORIGINAL_REQUEST.md` line 77)  
**Verdict**: **CLEAN**

---

## Forensic Audit Summary

| Check | Expected | Observed | Status |
|---|---|---|---|
| **Hardcoded Test Results** | Tests execute real logic and check generated artifacts | All 10 tests in `auxiliary_test.go` and tests in `gui_test.go` execute real generators and assert tokens/structure | **PASS** |
| **Facade / Dummy Stubs** | Genuine logic without constant returns or empty stubs | `GenerateAuxiliaryPluginsCSS()` (560 lines) and `GenerateAuxiliaryPluginsScript()` (1400+ lines) contain full implementations | **PASS** |
| **Pre-populated Artifacts** | Zero pre-baked logs, outputs, or test result artifacts | Clean git status; zero pre-populated test output artifacts | **PASS** |
| **DOM Injection & State Sync** | Strictly targets `.shrink-0.flex.items-center.gap-0.5.border-b`, mounts `#swiss-aux-container` in `.flex-grow.overflow-hidden`, sets `data-tab-id="swiss-*"`, syncs with factory tabs | Verified in `pkg/plugins/auxiliary.go` lines 603–744; 2-way event delegation and localStorage persistence | **PASS** |
| **Bézier Curve Smoothing Math** | Authentic $C^1$ continuous quadratic Bézier midpoint interpolation | Verified in `pkg/plugins/auxiliary.go` lines 1155–1172: midpoint calculation $M = \frac{P_{last} + P_{cur}}{2}$ and `ctx.quadraticCurveTo(lastX, lastY, midX, midY)` | **PASS** |
| **Send to Chat File Synthesis** | Real canvas cropping, PNG blob, synthetic `File` via `DataTransfer`, and Lexical editor injection | Verified in `pkg/plugins/auxiliary.go` lines 1372–1485: offscreen canvas, `toBlob`, `new File(...)`, `DataTransfer`, `fileInput.files = dt.files`, and `editor.__lexicalEditor.update` | **PASS** |
| **Device Frames & Touch Emulation** | Authentic metrics (iPhone 16 Pro 402×874, Pixel 9 412×924, iPad 820×1180), Dynamic Island, punch-holes, W3C `TouchEvent` emulation | Verified in `pkg/plugins/auxiliary.go` lines 200–304 & 1003–1071 | **PASS** |
| **Styler Script Bundling** | Clean script composition without redundant duplicates | Verified in `pkg/gui/styler.go` lines 963–990; `generateBaseScript` extracted, single concatenation in `GenerateScriptWithCustomModels` | **PASS** |
| **Independent Test Verification** | All unit and integration tests pass green | `go test -count=1 -v ./pkg/plugins/... ./pkg/gui/...` (23/23 pass, 0 fail)<br>`go test -count=1 ./...` (100% pass across all 18 repo packages) | **PASS** |
| **Frontend Production Build** | TypeScript check and Vite build succeed cleanly | `npm run build` completed in 1.09s with zero errors | **PASS** |

---

## 1. Observation

### Exact File Paths & Code Lines Audited:
1. `pkg/plugins/auxiliary.go`:
   - Lines 603–665: `setupAuxiliaryTabs` strictly attaches to `.shrink-0.flex.items-center.gap-0.5.border-b` (with auxiliary fallbacks). Custom buttons assign `data-tab-id="swiss-browser"`, `data-tab-id="swiss-files"`, and `data-tab-id="swiss-memos"`.
   - Lines 667–683: Event listener intercepts clicks on factory buttons (`overview`, `review`, `terminal`) to call `switchAuxTab(null)`, restoring factory subviews. Checks `localStorage.getItem("antigravity_active_aux_tab")` to restore previous Swiss selection across re-renders.
   - Lines 686–744: `switchAuxTab` mounts `#swiss-aux-container` inside `.flex-grow.overflow-hidden`, hides sibling factory views (`child.style.display = "none"`), removes `.active` class from factory buttons, and persists `localStorage.setItem("antigravity_active_aux_tab", normalizedId)`.
   - Lines 763–864: Two-tier toolbar with navigation controls, device frame selector (`#swiss-b-device`), viewport scale selector (`#swiss-b-scale`), touch toggle (`#swiss-b-touch`), red drawing tools (`#swiss-b-pen`, `#swiss-b-rect`, `#swiss-b-inspect`, `#swiss-b-clear`), Send to Chat (`#swiss-b-send-chat`), and dedicated port shortcuts bar (`:5173`, `:3000`, `:8080`, `:8765`, `:4173`, `+ Port`). Embedded `<webview>` element configured with `partition="persist:swiss-browser"`, `allowpopups="true"`, and relaxed CORS webpreferences (`allowRunningInsecureContent=yes, webSecurity=no`).
   - Lines 1003–1071: Synthetic `TouchEvent` emulation script (`touchstart`, `touchmove`, `touchend`) with realistic cursor styling (`.touch-emulation-active`).
   - Lines 1120–1226: Authentic Bézier midpoint smoothing math for red pen (`#ea4335`, 3px):
     ```javascript
     const midX = (lastX + coords.x) / 2;
     const midY = (lastY + coords.y) / 2;
     ctx.beginPath();
     ctx.moveTo(lastMidX, lastMidY);
     ctx.quadraticCurveTo(lastX, lastY, midX, midY);
     ctx.stroke();
     ```
   - Lines 1228–1370: Interactive DOM element inspector injecting guest script, calculating CSS selector (`getCssSelector`), highlighting element with `#swiss-dom-inspect-overlay`, and reporting `{ selector, outerHTML, rect }` back via `[SWISS_INSPECT_RESULT]` console message.
   - Lines 1372–1485: Send to Chat workflow cropping annotated region to offscreen canvas, compositing `webview.capturePage`, generating PNG blob via `toBlob`, synthesizing `new File([blob], "annotation.png", { type: "image/png" })`, attaching to `input[type="file"]` via `new DataTransfer()`, and injecting prompt text into `editor.__lexicalEditor.update(...)`.

2. `pkg/gui/styler.go`:
   - Lines 294–961: `generateBaseScript(cfg *Config) string` encapsulates base project tag and tab styling logic.
   - Lines 964–966: `GenerateScript(cfg *Config)` delegates to `GenerateScriptWithCustomModels(cfg, nil)`.
   - Lines 969–990: `GenerateScriptWithCustomModels` cleanly concatenates `baseScript + ";\n\n" + customScript + ";\n\n" + enhScript + ";\n\n" + pluginsScript + ";"`, eliminating redundant script inclusions.

3. `pkg/plugins/auxiliary_test.go`:
   - 10 unit tests asserting CSS selectors, JS identifiers, DOM injection selectors, tab attributes, state sync logic, device frame dimensions, Bézier smoothing tokens, DOM inspector tokens, Send to Chat workflow tokens, and device frame tokens.

4. `pkg/gui/gui_test.go`:
   - Lines 556–585: `TestGenerateScriptBundlingAuxiliaryPlugins` verifies that `GenerateScript(cfg)` bundles `setupAuxiliaryTabs`, `swiss-aux-container`, and that `function setupAuxiliaryTabs()` occurs exactly once.

---

## 2. Logic Chain

1. **Integrity Mode Ground Truth**:
   - `ORIGINAL_REQUEST.md` (timestamp `2026-10-05T22:09:01Z`, line 77) explicitly specifies `Integrity mode: development`. Under development mode, code reuse is permitted, but dummy facades, hardcoded test results, and fabricated outputs are prohibited. The implementation was audited against these constraints and surpassed standard benchmarks for authentic, from-scratch algorithmic logic.
2. **Mathematical Authenticity of Curve Smoothing**:
   - For rapid pen strokes on an HTML canvas, connecting raw sampled coordinates with straight lines creates jagged polylines. Connecting midpoints $M_i = \frac{P_{i-1} + P_i}{2}$ with quadratic Bézier curves using $P_i$ as control points guarantees that adjacent curve segments share identical derivative tangents at junction points ($B'_i(1) = B'_{i+1}(0) = P_{i+1} - P_i$). This achieves $C^1$ continuity and smooth visual curves. Inspection of `pkg/plugins/auxiliary.go` confirms exact implementation of this formulation.
3. **DOM Injection & Host Integrity**:
   - The injector targets `.shrink-0.flex.items-center.gap-0.5.border-b` and injects tab buttons with `data-tab-id="swiss-*"`. When a Swiss tab is selected, `#swiss-aux-container` (nested in `.flex-grow.overflow-hidden`) is displayed while factory views are hidden. When a factory tab button (`overview`, `review`, `terminal`) is clicked, event bubbling detects the non-Swiss button, hides `#swiss-aux-container`, and unhides factory views. State is persisted in `localStorage`. This guarantees 100% two-way synchronization without breaking factory views.
4. **Chat Integration Authenticity**:
   - The "Send to Chat" pipeline uses native browser primitives: an offscreen `<canvas>` captures and crops the visual region, converts it to an image blob, constructs a W3C `File` object (`annotation.png`), and adds it to a `DataTransfer` object assigned to the composer's `input[type="file"]`. For text, it interfaces with Antigravity's `editor.__lexicalEditor.update()` and provides a fallback to `insertTextToChatInput`.
5. **Clean Bundling Architecture**:
   - By extracting `generateBaseScript` and routing `GenerateScript` through `GenerateScriptWithCustomModels`, the styler generator ensures that each subsystem script appears exactly once in the final bundle, eliminating duplicate execution.

---

## 3. Caveats

- **Electron Host Webview Configuration**: In standard Electron applications, `<webview>` requires `webviewTag: true` in the host window's `webPreferences`. The implementation safely falls back to a sandboxed `<iframe>` if `<webview>` creation fails.
- **Milestone Scope Boundary**: File Explorer filesystem endpoints (`/api/files/*`) and Quick Memos audio backend are scheduled for subsequent milestones (Ext-M2 and Ext-M5). The client-side UI shells in `pkg/plugins/auxiliary.go` correctly anticipate these APIs.

---

## 4. Conclusion

**Verdict: CLEAN**

Milestone Ext-M1 contains zero integrity violations:
- Zero hardcoded test outputs or mock passes.
- Zero facade or stub implementations.
- Authentic quadratic Bézier curve smoothing mathematics ($C^1$ continuity).
- Authentic two-way DOM injection and factory tab synchronization.
- Authentic Send to Chat workflow utilizing offscreen canvas cropping, synthetic `File` via `DataTransfer`, and Lexical editor injection.
- Authentic mobile device frames and W3C `TouchEvent` emulation.
- Clean script bundling with zero duplicate script sections.
- 100% passing uncached test suite across all 18 repository packages.
- 100% clean frontend production build.

---

## 5. Verification Method

To independently reproduce the forensic verification:

1. **Verify Unit Tests in Affected Packages**:
   ```bash
   go test -count=1 -v ./pkg/plugins/... ./pkg/gui/...
   ```
   *Expected*: PASS across all 23 test functions in `pkg/plugins` and `pkg/gui`.

2. **Verify Full Repository Test Suite (Uncached)**:
   ```bash
   go test -count=1 ./...
   ```
   *Expected*: `ok` across all 18 packages (`cmd/swiss`, `pkg/cache`, `pkg/core`, `pkg/custommodels`, `pkg/daemon`, `pkg/enhancements`, `pkg/fingerprint`, `pkg/gui`, `pkg/importer`, `pkg/ipc`, `pkg/keyring`, `pkg/plugins`, `pkg/process`, `pkg/quota`, `pkg/system`, `pkg/templates`, `pkg/totp`, `pkg/webgui`).

3. **Verify Frontend Build**:
   ```bash
   cd frontend && npm run build
   ```
   *Expected*: `built in ...s` with zero TypeScript or Vite errors.
