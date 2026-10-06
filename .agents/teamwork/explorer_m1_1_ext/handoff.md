# Handoff Report: Ext-M1 Tab Injector Engine & Styler Integration

**From**: `explorer_m1_1_ext` (teamwork_preview_explorer)  
**To**: Orchestrator (`1e9124c8-4e7a-4fbd-80fe-96b480b57931`) / Worker  
**Date**: 2026-10-05T22:36:00Z  
**Type**: Hard Handoff (Investigation Complete)  
**Associated Report**: `/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_m1_1_ext/report.md`

---

## 1. Observation

1. **`pkg/plugins/auxiliary.go` Loose Targeting & Loose Mounting**:
   - Lines 366–368:
     ```javascript
     const tabHeader = auxPanel.querySelector('.shrink-0.flex.items-center') ||
                       auxPanel.querySelector('.border-b') ||
                       auxPanel.firstElementChild;
     ```
   - Lines 388–391:
     ```javascript
     const btn = document.createElement("button");
     btn.className = "swiss-aux-tab-btn";
     btn.dataset.swissTab = t.id;
     ```
     Sets `data-swiss-tab="browser"` instead of the required `data-tab-id="swiss-browser"`.
   - Lines 423–428:
     ```javascript
     let swissContainer = document.getElementById("swiss-aux-container");
     if (!swissContainer) {
       swissContainer = document.createElement("div");
       swissContainer.id = "swiss-aux-container";
       auxPanel.appendChild(swissContainer);
     }
     ```
     Appends `#swiss-aux-container` directly to root `auxPanel` instead of nesting strictly inside `.flex-grow.overflow-hidden`.
   - Lines 401–409:
     ```javascript
     tabHeader.addEventListener("click", (e) => {
       const target = e.target.closest("button");
       if (target && !target.classList.contains("swiss-aux-tab-btn")) {
         switchAuxTab(null);
       }
     });
     ```
     Lacks tracking for factory tab identifiers (`overview`, `review`, `terminal`), does not store `antigravity_active_aux_tab` in `localStorage`, and does not restore active Swiss tab on DOM rebuilds.

2. **`pkg/gui/styler.go` Missing Call & Redundant Duplication**:
   - Lines 958–975 (`GenerateScript(cfg)`):
     ```go
     customScript := custommodels.GenerateCustomModelsScript(cmCfg)
     enhScript := enhancements.GenerateEnhancementsScript(enhCfg)
     return baseScript + ";\n\n" + customScript + ";\n\n" + enhScript + ";"
     ```
     Omits `plugins.GenerateAuxiliaryPluginsScript()`.
   - Lines 985–1007 (`GenerateScriptWithCustomModels(cfg, cmCfg)`):
     ```go
     base := GenerateScript(cfg)
     ...
     pluginsScript := plugins.GenerateAuxiliaryPluginsScript()

     res := base
     if customScript != "" {
       res += ";\n\n" + customScript
     }
     if enhScript != "" {
       res += ";\n\n" + enhScript
     }
     if pluginsScript != "" {
       res += ";\n\n" + pluginsScript
     }
     return res
     ```
     Because `GenerateScript(cfg)` already generates `customScript` and `enhScript`, `GenerateScriptWithCustomModels` appends duplicates of both scripts into the bundle.

3. **`pkg/plugins/auxiliary_test.go` Test Assertions**:
   - Lines 38–48: Only asserts basic function names (`setupAuxiliaryTabs`, `switchAuxTab`, etc.) and does not check for DOM selector `.shrink-0.flex.items-center.gap-0.5.border-b`, container `.flex-grow.overflow-hidden`, data attributes `data-tab-id="swiss-*"`, or two-way sync logic.

---

## 2. Logic Chain

1. **Step 1 (Navbar Injection Precision)**:
   - Observation 1 shows `setupAuxiliaryTabs` searches for loose `.border-b` or first child.
   - Antigravity 2.0 auxiliary header has exact class `.shrink-0.flex.items-center.gap-0.5.border-b`.
   - Modifying `setupAuxiliaryTabs` to query `.shrink-0.flex.items-center.gap-0.5.border-b` guarantees exact targeting of the navbar strip and prevents hijacking unrelated containers.

2. **Step 2 (Tab Attribute Parity)**:
   - Antigravity's native factory buttons use `data-tab-id="overview"`, `data-tab-id="review"`, `data-tab-id="terminal"`.
   - Observation 1 shows Swiss tabs set `btn.dataset.swissTab = t.id`.
   - Setting `btn.setAttribute("data-tab-id", t.tabId)` sets `data-tab-id="swiss-browser"`, `data-tab-id="swiss-files"`, and `data-tab-id="swiss-memos"`, conforming to Antigravity's tab identification contract.

3. **Step 3 (Container Viewport Mount & State Sync)**:
   - Antigravity's auxiliary panel body is `.flex-grow.overflow-hidden`.
   - Observation 1 shows `#swiss-aux-container` was mounted on `auxPanel`.
   - Moving `#swiss-aux-container` into `.flex-grow.overflow-hidden` ensures it sits alongside factory view panels.
   - When a Swiss tab is clicked, setting factory sibling views (`child !== swissContainer`) to `display = "none"` and `#swiss-aux-container` to `display = "flex"` renders the Swiss view.
   - When a factory tab button is clicked, setting `#swiss-aux-container` to `display = "none"` and factory sibling views to `display = ""` restores native views.
   - Storing the state in `localStorage.setItem('antigravity_active_aux_tab', ...)` allows restoring the view across DOM mutations.

4. **Step 4 (Styler Integration & De-duplication)**:
   - Observation 2 demonstrates that `GenerateScript(cfg)` does not bundle `pluginsScript`, and `GenerateScriptWithCustomModels` appends duplicate copies of `customScript` and `enhScript`.
   - By extracting `generateBaseScript(cfg)` and defining `GenerateScriptWithCustomModels(cfg, cmCfg)` as the single composition point returning `baseScript + customScript + enhScript + pluginsScript`, both `GenerateScript(cfg)` (which delegates to `GenerateScriptWithCustomModels(cfg, nil)`) and `GenerateScriptWithCustomModels` emit complete, duplicate-free code.
   - Consequently, `swiss patch sync` (calling `store.SyncPersistentFiles()`) and `swiss patch inject` (calling `injector.go`) generate the complete bundle into `persistent_script.js`.

5. **Step 5 (Unit Test Verification)**:
   - Observation 3 shows the existing tests lack selector and attribute validation.
   - Adding tests verifying the exact selector string `.shrink-0.flex.items-center.gap-0.5.border-b`, container `.flex-grow.overflow-hidden`, tab attributes `data-tab-id="swiss-*"`, and two-way sync logic in `auxiliary_test.go` and `gui_test.go` locks in regression prevention.

---

## 3. Caveats

- **Factory Sub-Panel Routing**: Antigravity's factory views are managed by Electron/React internal routing. While toggling `style.display = "none"` and `style.display = ""` cleanly hides and reveals the active factory panel DOM tree without unmounting React nodes, if Antigravity dynamically replaces the DOM nodes inside `.flex-grow.overflow-hidden` when a factory tab is clicked, our click listener on `tabHeader` cleanly clears `#swiss-aux-container` to `display: none` and lets React mount its content unhindered.
- **R2–R7 Dependencies**: This handoff covers Ext-M1 (R1 Tab Injector Engine and Styler Integration). The subview bodies (`renderBrowserView`, `renderFilesView`, `renderMemosView`) already exist as initial implementations in `pkg/plugins/auxiliary.go`, and their full feature sets (R2 canvas annotation, R3 file mutation endpoints, etc.) will be progressively deepened in subsequent milestones.

---

## 4. Conclusion

The architectural investigation for **Ext-M1** is complete.
1. The DOM targeting and container mounting modifications for `pkg/plugins/auxiliary.go` are defined with verbatim JavaScript code.
2. The script bundling and de-duplication fix for `pkg/gui/styler.go` is defined.
3. Unit test suites for `pkg/plugins/auxiliary_test.go` and `pkg/gui/gui_test.go` are designed.
4. The implementation is ready for immediate Worker execution.

---

## 5. Verification Method

To independently verify the implementation:
1. **Auxiliary Unit Tests**:
   ```bash
   go test -v ./pkg/plugins/...
   ```
   Validates `TestGenerateAuxiliaryPluginsCSS`, `TestGenerateAuxiliaryPluginsScript`, `TestAuxiliaryTabInjectionSelectors`, `TestAuxiliaryTabAttributes`, and `TestAuxiliaryTwoWayStateSyncLogic`.
2. **GUI Styler Unit Tests**:
   ```bash
   go test -v -run "TestGenerateScript" ./pkg/gui/...
   ```
   Validates bundling of `plugins.GenerateAuxiliaryPluginsScript()` and absence of duplicate script blocks.
3. **Full Repository Test Suite**:
   ```bash
   go test ./...
   ```
   Ensures zero regressions across the entire Go codebase.
4. **CLI Persistent Script Generation**:
   ```bash
   go run ./cmd/swiss patch sync
   ```
   Inspect `~/.config/antigravity-swiss/persistent_script.js` to confirm it contains `.shrink-0.flex.items-center.gap-0.5.border-b`, `data-tab-id="swiss-browser"`, `#swiss-aux-container`, and `setupAuxiliaryTabs`.
