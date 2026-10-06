## 2026-10-05T22:34:43Z

You are worker_m1_1_ext, a teamwork_preview_worker.
Your working directory is:
/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/worker_m1_1_ext

MANDATORY FIRST STEP: Read the authoritative user request at:
/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/ORIGINAL_REQUEST.md
Specifically review the latest section under timestamp: 2026-10-05T22:09:01Z.

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A teamwork_preview_auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

You have exclusive write ownership of the following files:
- `pkg/plugins/auxiliary.go`
- `pkg/plugins/auxiliary_test.go`
- `pkg/gui/styler.go`
- `pkg/gui/gui_test.go`

Read the blueprints developed by the Explorers before implementing:
- /mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_m1_1_ext/handoff.md (Tab Injector, Styler Integration)
- /mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_m1_2_ext/handoff.md (Browser Preview, Device Frames)
- /mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_m1_3_ext/handoff.md (Canvas Annotation, Send to Chat)

Your Implementation Tasks for Milestone Ext-M1 (R1 & R2):
1. **Auxiliary Panel Tab Injector Engine (R1)**:
   - In `pkg/plugins/auxiliary.go`:
     - Update navbar query in `setupAuxiliaryTabs` to strictly match `.shrink-0.flex.items-center.gap-0.5.border-b`.
     - Assign tab attributes `data-tab-id="swiss-browser"`, `data-tab-id="swiss-files"`, `data-tab-id="swiss-memos"`.
     - Mount `#swiss-aux-container` inside `.flex-grow.overflow-hidden`.
     - Implement robust two-way state synchronization and tab restoration with Antigravity factory tabs (`overview`, `review`, `terminal`): hide factory content when a Swiss tab is selected, restore factory content when a factory tab is clicked, and restore the active Swiss tab across DOM rebuilds via `localStorage.getItem('antigravity_active_aux_tab')`.
2. **Styler Script Bundling (R1 & R8)**:
   - In `pkg/gui/styler.go`:
     - Update `GenerateScript(cfg)` to bundle `plugins.GenerateAuxiliaryPluginsScript()` cleanly alongside `baseScript`, `customScript`, and `enhScript`.
     - Refactor `GenerateScriptWithCustomModels` so it does not append duplicate script blocks.
3. **Live Browser Preview & Device Frames (R2)**:
   - In `pkg/plugins/auxiliary.go`:
     - Update toolbar with back, forward, reload, URL input with auto-protocol prefixing, and port shortcuts bar with one-click chips (`:5173`, `:3000`, `:8080`, `:8765`, `:4173`, `+ Port`).
     - Render embedded `<webview>` with `partition="persist:swiss-browser"`, `allowpopups="true"`, `webpreferences="allowRunningInsecureContent=yes, webSecurity=no"`, and sandboxed `<iframe>` fallback.
     - Add device frames: iPhone 16 Pro (402 × 874 px, Dynamic Island, titanium bezel), Pixel 9 (412 × 924 px, punch-hole camera, obsidian bezel), iPad (820 × 1180 px, space gray bezel, camera lens), and Responsive mode.
     - Implement auto-fit dynamic scaling (`applyDeviceScale`), canvas coordinate normalization `(e.clientX - rect.left) * (canvas.width / rect.width)`, and touch emulation toggle with synthetic `TouchEvent` dispatcher.
4. **Visual Canvas Annotation Tool & Send to Chat (R2)**:
   - In `pkg/plugins/auxiliary.go`:
     - Canvas drawing: red draw pen (#ea4335, 3px width) with Bézier midpoint curve smoothing (`quadraticCurveTo`), round line caps and joins.
     - Red bounding box drag tool (#ea4335, 2px border, semi-transparent fill `rgba(234, 67, 53, 0.15)`, and region tracking `lastAnnotatedRegion`).
     - Interactive DOM element selector inspector (`#swiss-b-inspect`): webview script injection, red outline hover highlight with CSS selector tag badge, and `console-message` bridge returning selector, rect, and `outerHTML`.
     - "Send to Chat" workflow (`#swiss-b-send-chat`): crop annotated region to PNG Blob, synthesize `new File([blob], 'annotation.png', { type: 'image/png' })`, inject into composer `document.querySelector('input[type="file"]')` via `DataTransfer`, and inject selector, DOM `outerHTML`, URL, and user comment into `editor.__lexicalEditor` (with fallback to `insertTextToChatInput`).
5. **Unit Tests & Verification**:
   - In `pkg/plugins/auxiliary_test.go` and `pkg/gui/gui_test.go`:
     - Add comprehensive tests validating CSS classes, DOM selectors, tab attributes, device frames, port shortcuts, Bézier curves, DOM inspector, file attachment, and styler bundling.
     - Run `go test -v ./pkg/plugins/... ./pkg/gui/...` and ensure 100% pass.
     - Run `go test ./pkg/...` to ensure zero regressions across all packages.
     - Run `cd frontend && npm run build` to verify frontend build integrity.
