# Exploration Report: Ext-M1 Auxiliary Panel Tab Injector Engine & Styler Integration

**Explorer**: `explorer_m1_1_ext` (teamwork_preview_explorer)  
**Date**: 2026-10-05T22:34:00Z  
**Milestone**: Ext-M1 (R1 Tab Injector Engine & Styler Integration)  
**Target Files**:
- `pkg/plugins/auxiliary.go`
- `pkg/gui/styler.go`
- `pkg/plugins/auxiliary_test.go`
- `pkg/gui/gui_test.go`

---

## 1. Executive Summary

This report establishes the complete architectural specification and concrete implementation blueprint for Milestone **Ext-M1**:
1. **Auxiliary Panel Tab Injector Engine (`pkg/plugins/auxiliary.go`)**:
   - Updates navbar selector targeting to strictly match `.shrink-0.flex.items-center.gap-0.5.border-b`.
   - Sets custom tab button attributes to `data-tab-id="swiss-browser"`, `data-tab-id="swiss-files"`, `data-tab-id="swiss-memos"`.
   - Mounts and toggles `#swiss-aux-container` strictly inside Antigravity's body viewport container `.flex-grow.overflow-hidden`.
   - Implements robust two-way state synchronization and tab restoration with Antigravity factory tabs (`overview`, `review`, `terminal`).
2. **Styler Script Generation Integration (`pkg/gui/styler.go`)**:
   - Fixes `GenerateScript(cfg)` to bundle `plugins.GenerateAuxiliaryPluginsScript()` alongside base, custom models, and enhancements scripts.
   - Resolves a duplication bug in `GenerateScriptWithCustomModels` where `customScript` and `enhScript` were redundantly appended twice.
   - Ensures `swiss patch sync` and live injection (`swiss patch inject`) emit a clean, single-copy bundle into `persistent_script.js`.
3. **Unit Test Suite Design (`pkg/plugins/auxiliary_test.go` & `pkg/gui/gui_test.go`)**:
   - Defines tests asserting exact DOM selectors, custom tab attributes, two-way synchronization logic, and script bundling integrity.

---

## 2. Requirement 1: Auxiliary Panel Tab Injector Engine (`pkg/plugins/auxiliary.go`)

### 2.1 Current Implementation Deficiencies
In `pkg/plugins/auxiliary.go`:
1. **Loose Navbar Selector**:
   Line 366 queries:
   ```javascript
   const tabHeader = auxPanel.querySelector('.shrink-0.flex.items-center') ||
                     auxPanel.querySelector('.border-b') ||
                     auxPanel.firstElementChild;
   ```
   This is overly permissive, risking attachment to non-navbar headers or generic flex containers.
2. **Incorrect Tab Attributes**:
   Line 390 sets `btn.dataset.swissTab = t.id`, which produces `data-swiss-tab="browser"` instead of the required `data-tab-id="swiss-browser"`. In Antigravity 2.0, factory tabs use `data-tab-id` (`overview`, `review`, `terminal`).
3. **Invalid Container Mount Location**:
   Line 427 appends `#swiss-aux-container` directly to `auxPanel` (`auxPanel.appendChild(swissContainer)`). This places the container outside the `.flex-grow.overflow-hidden` viewport hierarchy, breaking scrolling and flex layout.
4. **Fragile State Sync & Missing Tab Restoration**:
   The current code toggles `child.style.display` across all children of `auxPanel` except the first child. It does not:
   - Identify factory tabs by attribute.
   - Restore factory tabs when Antigravity switches tabs internally.
   - Restore the active Swiss tab from `localStorage` upon DOM rebuilds or window reload.

### 2.2 Target Architecture & DOM Structure

Antigravity 2.0 right auxiliary panel DOM hierarchy:
```html
<div class="... auxiliary panel root ...">
  <!-- Navbar Header -->
  <div class="shrink-0 flex items-center gap-0.5 border-b ...">
    <!-- Factory Tabs -->
    <button data-tab-id="overview" class="...">Overview</button>
    <button data-tab-id="review" class="...">Review</button>
    <button data-tab-id="terminal" class="...">Terminal</button>
    
    <!-- Injected Divider & Swiss Button Group -->
    <div class="swiss-aux-tabs-divider"></div>
    <div class="swiss-aux-btn-group">
      <button class="swiss-aux-tab-btn" data-tab-id="swiss-browser" data-swiss-tab="browser">🌐 Browser</button>
      <button class="swiss-aux-tab-btn" data-tab-id="swiss-files" data-swiss-tab="files">📁 Files</button>
      <button class="swiss-aux-tab-btn" data-tab-id="swiss-memos" data-swiss-tab="memos">📝 Memos</button>
    </div>
  </div>

  <!-- Body Viewport Container -->
  <div class="flex-grow overflow-hidden ...">
    <!-- Factory Content Views (hidden when Swiss tab is active) -->
    <div class="factory-view-content" style="...">...</div>
    
    <!-- Injected Swiss Container (mounted inside .flex-grow.overflow-hidden) -->
    <div id="swiss-aux-container" style="display: none;">
      <!-- Active Swiss Subview -->
      <div class="swiss-browser-view">...</div>
      <div class="swiss-files-view">...</div>
      <div class="swiss-memos-view">...</div>
    </div>
  </div>
</div>
```

### 2.3 Two-Way State Synchronization Flow

1. **User clicks Swiss Tab (`swiss-browser`, `swiss-files`, `swiss-memos`)**:
   - Add `.active` and `aria-selected="true"` to clicked button; remove from other Swiss buttons.
   - Remove active state from factory buttons (`tabHeader.querySelectorAll('button:not([data-tab-id^="swiss-"])')`).
   - Query `.flex-grow.overflow-hidden`. Set all direct children (factory views) to `style.display = "none"`.
   - Set `#swiss-aux-container.style.display = "flex"`.
   - Render or unhide the selected view in `#swiss-aux-container`.
   - Persist: `localStorage.setItem('antigravity_active_aux_tab', tabId)`.
2. **User clicks Factory Tab (`overview`, `review`, `terminal`)**:
   - `tabHeader.addEventListener("click", ...)` intercepts clicks on non-Swiss buttons.
   - Invoke `switchAuxTab(null)`.
   - Set `#swiss-aux-container.style.display = "none"`.
   - Restore factory views: iterate direct children of `.flex-grow.overflow-hidden` and set `child.style.display = ""`.
   - Remove `.active` and `aria-selected` from all `.swiss-aux-tab-btn`.
   - Persist: `localStorage.setItem('antigravity_active_aux_tab', 'factory')`.
3. **Tab Restoration on Startup / DOM Rebuild**:
   - When `setupAuxiliaryTabs` runs, check `localStorage.getItem('antigravity_active_aux_tab')`.
   - If it begins with `swiss-`, restore that tab via `switchAuxTab(savedTab)`.
   - If user previously selected a factory tab (`factory`), leave factory views intact.

### 2.4 Concrete Code Implementation for `pkg/plugins/auxiliary.go`

```javascript
    // 1. Auxiliary Panel Tab Injector Engine
    function setupAuxiliaryTabs() {
      // Find auxiliary panel header strictly matching .shrink-0.flex.items-center.gap-0.5.border-b
      const tabHeader = document.querySelector('.shrink-0.flex.items-center.gap-0.5.border-b') ||
                        document.querySelector('[data-testid="auxiliary-panel"] .shrink-0.flex.items-center.gap-0.5.border-b') ||
                        document.querySelector('.part.auxiliarybar .shrink-0.flex.items-center.gap-0.5.border-b') ||
                        document.querySelector('.shrink-0.flex.items-center.border-b');
      if (!tabHeader) return;

      // Injection guard: Check if swiss tabs already injected
      if (tabHeader.querySelector('[data-tab-id="swiss-browser"]')) {
        // Ensure state restoration if DOM was re-rendered while a Swiss tab was active
        if (activeAuxTab) {
          switchAuxTab(activeAuxTab);
        }
        return;
      }

      // Create Swiss Tab Buttons: Browser, Files, Memos
      const tabs = [
        { id: "browser", tabId: "swiss-browser", label: "Browser", icon: "🌐" },
        { id: "files", tabId: "swiss-files", label: "Files", icon: "📁" },
        { id: "memos", tabId: "swiss-memos", label: "Memos", icon: "📝" },
      ];

      // Subtle visual divider before Swiss tabs
      let divider = tabHeader.querySelector('.swiss-aux-tabs-divider');
      if (!divider) {
        divider = document.createElement("div");
        divider.className = "swiss-aux-tabs-divider";
        divider.style.height = "16px";
        divider.style.width = "1px";
        divider.style.backgroundColor = "var(--border, #e2e8f0)";
        divider.style.margin = "0 4px";
        divider.style.opacity = "0.7";
        tabHeader.appendChild(divider);
      }

      let btnGroup = tabHeader.querySelector('.swiss-aux-btn-group');
      if (!btnGroup) {
        btnGroup = document.createElement("div");
        btnGroup.className = "swiss-aux-btn-group";
        btnGroup.style.display = "inline-flex";
        btnGroup.style.alignItems = "center";
        btnGroup.style.gap = "2px";
        tabHeader.appendChild(btnGroup);
      }

      tabs.forEach(t => {
        const btn = document.createElement("button");
        btn.className = "swiss-aux-tab-btn";
        btn.setAttribute("data-tab-id", t.tabId);
        btn.dataset.swissTab = t.id;
        btn.title = `Antigravity Swiss Knife: ${t.label}`;
        btn.innerHTML = `<span>${t.icon}</span><span>${t.label}</span>`;
        btn.onclick = (e) => {
          e.stopPropagation();
          switchAuxTab(t.tabId);
        };
        btnGroup.appendChild(btn);
      });

      // Two-way state sync: Listen for clicks on native factory tabs (overview, review, terminal)
      tabHeader.addEventListener("click", (e) => {
        const targetBtn = e.target.closest("button");
        if (!targetBtn) return;
        const targetId = targetBtn.getAttribute("data-tab-id") || "";
        if (targetId.startsWith("swiss-")) {
          return; // Swiss tab clicked; handled by btn.onclick
        }
        // Native tab clicked (overview, review, terminal, etc.)
        switchAuxTab(null);
      });

      // Restore saved tab state from localStorage
      const savedTab = localStorage.getItem("antigravity_active_aux_tab");
      if (savedTab && savedTab.startsWith("swiss-") && !activeAuxTab) {
        switchAuxTab(savedTab);
      }
    }

    // Switch between Swiss tabs and Native factory tabs
    function switchAuxTab(tabId) {
      const normalizedId = tabId ? (tabId.startsWith("swiss-") ? tabId : ("swiss-" + tabId)) : null;
      activeAuxTab = normalizedId;

      // Update button active states
      document.querySelectorAll(".swiss-aux-tab-btn").forEach(b => {
        const isActive = normalizedId !== null && b.getAttribute("data-tab-id") === normalizedId;
        b.classList.toggle("active", isActive);
        b.setAttribute("aria-selected", isActive ? "true" : "false");
      });

      // Locate auxiliary navbar and body container .flex-grow.overflow-hidden
      const tabHeader = document.querySelector('.shrink-0.flex.items-center.gap-0.5.border-b') ||
                        document.querySelector('[data-testid="auxiliary-panel"] .shrink-0.flex.items-center.gap-0.5.border-b') ||
                        document.querySelector('.part.auxiliarybar .shrink-0.flex.items-center.gap-0.5.border-b');
      const auxPanel = tabHeader ? tabHeader.parentElement : (document.querySelector('[data-testid="auxiliary-panel"]') || document.querySelector('.part.auxiliarybar'));
      
      const bodyContainer = (auxPanel ? auxPanel.querySelector('.flex-grow.overflow-hidden') : null) ||
                            document.querySelector('.flex-grow.overflow-hidden');
      if (!bodyContainer) return;

      // Mount #swiss-aux-container strictly inside .flex-grow.overflow-hidden
      let swissContainer = document.getElementById("swiss-aux-container");
      if (!swissContainer) {
        swissContainer = document.createElement("div");
        swissContainer.id = "swiss-aux-container";
        swissContainer.style.display = "none";
        bodyContainer.appendChild(swissContainer);
      } else if (swissContainer.parentElement !== bodyContainer) {
        bodyContainer.appendChild(swissContainer);
      }

      // Hide or show native content views inside bodyContainer
      const children = Array.from(bodyContainer.children);
      if (normalizedId) {
        // Swiss tab active: hide factory content views, show Swiss container
        children.forEach(child => {
          if (child === swissContainer) {
            child.style.display = "flex";
          } else {
            child.style.display = "none";
          }
        });

        // De-highlight factory buttons in tabHeader
        if (tabHeader) {
          tabHeader.querySelectorAll('button:not([data-tab-id^="swiss-"])').forEach(fb => {
            fb.classList.remove("active");
            fb.removeAttribute("data-state");
            fb.setAttribute("aria-selected", "false");
          });
        }

        localStorage.setItem("antigravity_active_aux_tab", normalizedId);
        const cleanId = normalizedId.replace(/^swiss-/, "");
        renderSwissTabContent(swissContainer, cleanId);
      } else {
        // Factory tab active: hide Swiss container, restore factory content views
        swissContainer.style.display = "none";
        children.forEach(child => {
          if (child !== swissContainer) {
            child.style.display = "";
          }
        });

        localStorage.setItem("antigravity_active_aux_tab", "factory");
      }
    }
```

---

## 3. Requirement 2: Styler Script Generation Integration (`pkg/gui/styler.go`)

### 3.1 Current Implementation Deficiencies
In `pkg/gui/styler.go`:
1. `GenerateScript(cfg)` (lines 958–975) only concatenates:
   `baseScript + ";\n\n" + customScript + ";\n\n" + enhScript + ";"`
   It **never calls** `plugins.GenerateAuxiliaryPluginsScript()`.
   Consequently, live injection (`swiss patch inject`) never injects auxiliary plugins.
2. `GenerateScriptWithCustomModels(cfg, cmCfg)` (lines 978–1009):
   - Calls `base := GenerateScript(cfg)` (which already included `customScript` and `enhScript`).
   - Then it tests `if customScript != ""` and appends `customScript` **again**.
   - Then it tests `if enhScript != ""` and appends `enhScript` **again**.
   - This duplicates the entire custom models and enhancements scripts in `persistent_script.js`.

### 3.2 Refactoring Design
Extract `generateBaseScript(cfg *Config) string` to cleanly encapsulate the base project tag and conversation tabs styling script (lines 294–958).
Then implement:
```go
// GenerateScript generates the JavaScript snippet to evaluate inside Antigravity Electron renderer.
func GenerateScript(cfg *Config) string {
	return GenerateScriptWithCustomModels(cfg, nil)
}

// GenerateScriptWithCustomModels generates the complete unified script bundling:
// 1. Base project tags & conversation tabs script
// 2. Custom models dropdown selector script
// 3. App enhancements script
// 4. Auxiliary panel plugins script (browser preview, file explorer, memos, telemetry)
func GenerateScriptWithCustomModels(cfg *Config, cmCfg *custommodels.Config) string {
	baseScript := generateBaseScript(cfg)

	if cmCfg == nil {
		if cmStore, err := custommodels.NewStore(""); err == nil {
			c := cmStore.GetConfig()
			cmCfg = &c
		}
	}
	customScript := custommodels.GenerateCustomModelsScript(cmCfg)

	var enhCfg *enhancements.EnhancementsConfig
	if enhStore, err := enhancements.NewStore(""); err == nil {
		c := enhStore.GetConfig()
		enhCfg = &c
	}
	enhScript := enhancements.GenerateEnhancementsScript(enhCfg)

	pluginsScript := plugins.GenerateAuxiliaryPluginsScript()

	return baseScript + ";\n\n" + customScript + ";\n\n" + enhScript + ";\n\n" + pluginsScript + ";"
}
```

### 3.3 Benefits of this Architecture
1. **Single Source of Truth**: Both `GenerateScript(cfg)` and `GenerateScriptWithCustomModels(cfg, cmCfg)` use the exact same composition pipeline.
2. **Zero Duplication**: `customScript`, `enhScript`, and `pluginsScript` appear exactly once in the emitted output.
3. **Full CLI & Daemon Synchronization**:
   - `swiss patch sync` (calling `store.SyncPersistentFiles()` -> `GenerateScriptWithCustomModels(cfg, nil)`) writes the complete bundle to `~/.config/antigravity-swiss/persistent_script.js`.
   - `swiss patch inject` (calling `injector.go` -> `GenerateScript(cfg)`) injects the complete bundle into the running Electron instance.

---

## 4. Requirement 3: Unit Tests Design (`pkg/plugins/auxiliary_test.go` & `pkg/gui/gui_test.go`)

### 4.1 Unit Tests for `pkg/plugins/auxiliary_test.go`

Add the following new test functions:

```go
func TestAuxiliaryTabInjectionSelectors(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	if js == "" {
		t.Fatalf("expected non-empty JS script")
	}

	requiredSelectors := []string{
		".shrink-0.flex.items-center.gap-0.5.border-b",
		".flex-grow.overflow-hidden",
		"#swiss-aux-container",
		".swiss-aux-btn-group",
		".swiss-aux-tabs-divider",
	}

	for _, sel := range requiredSelectors {
		if !strings.Contains(js, sel) {
			t.Errorf("expected script to contain selector %q", sel)
		}
	}
}

func TestAuxiliaryTabAttributes(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	if js == "" {
		t.Fatalf("expected non-empty JS script")
	}

	requiredAttributes := []string{
		`data-tab-id="swiss-browser"`,
		`data-tab-id="swiss-files"`,
		`data-tab-id="swiss-memos"`,
		`dataset.swissTab = t.id`,
	}

	for _, attr := range requiredAttributes {
		if !strings.Contains(js, attr) {
			t.Errorf("expected script to configure tab attribute %q", attr)
		}
	}
}

func TestAuxiliaryTwoWayStateSyncLogic(t *testing.T) {
	js := GenerateAuxiliaryPluginsScript()
	if js == "" {
		t.Fatalf("expected non-empty JS script")
	}

	requiredLogicSnippets := []string{
		`localStorage.setItem("antigravity_active_aux_tab"`,
		`localStorage.getItem("antigravity_active_aux_tab")`,
		`swissContainer.style.display = "none"`,
		`swissContainer.style.display = "flex"`,
		`child.style.display = "none"`,
		`child.style.display = ""`,
		`switchAuxTab(null)`,
	}

	for _, snippet := range requiredLogicSnippets {
		if !strings.Contains(js, snippet) {
			t.Errorf("expected script to contain state sync logic snippet %q", snippet)
		}
	}
}
```

### 4.2 Unit Tests for `pkg/gui/gui_test.go`

Add `TestGenerateScriptBundlingAuxiliaryPlugins(t *testing.T)`:
```go
func TestGenerateScriptBundlingAuxiliaryPlugins(t *testing.T) {
	cfg := DefaultConfig()
	script := GenerateScript(cfg)

	// Verify auxiliary plugins script is bundled
	if !strings.Contains(script, "setupAuxiliaryTabs") {
		t.Errorf("GenerateScript missing setupAuxiliaryTabs from auxiliary plugins")
	}
	if !strings.Contains(script, "swiss-aux-container") {
		t.Errorf("GenerateScript missing swiss-aux-container from auxiliary plugins")
	}
	if !strings.Contains(script, `data-tab-id="swiss-browser"`) {
		t.Errorf("GenerateScript missing swiss-browser tab attribute")
	}

	// Verify no duplicate script inclusion
	countSetupAux := strings.Count(script, "function setupAuxiliaryTabs()")
	if countSetupAux != 1 {
		t.Errorf("expected setupAuxiliaryTabs to appear exactly 1 time, got %d", countSetupAux)
	}

	// Verify custom models script appears exactly once
	countCustomModels := strings.Count(script, "setupCustomModelsDropdown")
	if countCustomModels != 1 {
		t.Errorf("expected setupCustomModelsDropdown to appear exactly 1 time, got %d", countCustomModels)
	}
}
```

---

## 5. Concrete Implementation Blueprint for Worker

The Worker should execute the following plan sequentially:

### Step 1: Update `pkg/plugins/auxiliary.go`
1. In `GenerateAuxiliaryPluginsCSS()`:
   Ensure `.swiss-aux-tab-btn`, `.swiss-aux-tabs-divider`, `.swiss-aux-btn-group`, and `#swiss-aux-container` use CSS custom properties (`var(--muted-foreground)`, `var(--secondary)`, `var(--border)`, `var(--primary)`).
2. In `GenerateAuxiliaryPluginsScript()`:
   Replace `setupAuxiliaryTabs()` and `switchAuxTab()` with the implementation specified in Section 2.4:
   - Target `.shrink-0.flex.items-center.gap-0.5.border-b`.
   - Add `data-tab-id="swiss-browser"`, `data-tab-id="swiss-files"`, `data-tab-id="swiss-memos"`.
   - Mount `#swiss-aux-container` strictly inside `.flex-grow.overflow-hidden`.
   - Implement two-way sync on `tabHeader` click and `localStorage` restoration.

### Step 2: Refactor `pkg/gui/styler.go`
1. Split `GenerateScript(cfg *Config) string`:
   - Move lines 294–958 into a helper function `generateBaseScript(cfg *Config) string`.
2. Rewrite `GenerateScriptWithCustomModels(cfg *Config, cmCfg *custommodels.Config) string`:
   - Concatenate `baseScript`, `customScript`, `enhScript`, and `pluginsScript`.
3. Rewrite `GenerateScript(cfg *Config) string`:
   - Simply return `GenerateScriptWithCustomModels(cfg, nil)`.

### Step 3: Expand Unit Tests
1. In `pkg/plugins/auxiliary_test.go`:
   Add `TestAuxiliaryTabInjectionSelectors`, `TestAuxiliaryTabAttributes`, and `TestAuxiliaryTwoWayStateSyncLogic`.
2. In `pkg/gui/gui_test.go`:
   Add `TestGenerateScriptBundlingAuxiliaryPlugins`.

### Step 4: Verification & Build
1. Run `go test -v ./pkg/plugins/...` to verify auxiliary plugins tests.
2. Run `go test -v ./pkg/gui/...` to verify styler bundling tests.
3. Run `go test ./...` to verify all repo tests pass 100% green.
4. Run `go run ./cmd/swiss patch sync` (or verify with a test) to ensure `persistent_script.js` is generated without error.
