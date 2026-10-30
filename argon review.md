An inspection of /home/david/Projects/Antigravity Swiss Knife (\~202,100 LOC across Go cmd/ \+ pkg/, React 19 \+ TypeScript frontend/src/, and Electron electron/) shows a feature-rich companion daemon and desktop suite whose rapid growth across 15+ milestones has concentrated logic into a handful of monolithic files, duplicated state stores across IPC boundaries, and accumulated 11 GB of untracked/build artifacts on disk.

## **1\. Quantitative Codebase Baseline**

| Subsystem | Primary Files | Lines | Core Bottleneck | Target State |
| :---- | :---- | :---- | :---- | :---- |
| **In-IDE Injected JS/CSS** | pkg/plugins/auxiliary.go, enhancements/script.go, gui/styler.go | 19,881 | Raw Go backtick strings with 5 competing DOM observers | Modular TypeScript bundle compiled via Vite \+ //go:embed |
| **HTTP Server & Routing** | pkg/webgui/server.go, token\_monitor.go | 10,356 | 140+ routes and fallback store logic in one struct | Domain sub-handlers \+ unified service layer |
| **Daemon & Keyring Sync** | pkg/daemon/daemon.go, keyring/store.go, sync.go | 5,655 | Dual-instantiated stores between webgui and daemon | Single shared in-process state or strict RPC delegation |
| **React 19 Frontend** | frontend/src/pages/\*.tsx, components/\*.tsx, App.tsx | 24,800+ | 74 useState hooks in CustomModelsPage.tsx; sparse integer routing | Typed route/tab registry \+ domain hooks & sub-modals |
| **Workspace Disk Footprint** | scratch/, release/, bin/, antigravity\_swiss/ | 11.0 GB | 9.1 GB scratch dumps, 1.7 GB release zips, ghost .pyc tree | Pruned workspace \+ gitignored build staging |

## **2\. Structural & Architectural Optimizations**

### **2.1 Extract Embedded JavaScript/CSS from Go Backtick Monoliths (\~19.8k LOC)**

Currently, the scripts injected into the Antigravity host IDE via Chrome DevTools Protocol (CDP) are maintained inside massive Go string literals:

* pkg/plugins/auxiliary.go (8,145 lines)  
* pkg/enhancements/script.go (4,014 lines)  
* pkg/plugins/github\_extension\_script.go (3,892 lines)  
* pkg/gui/styler.go (2,726 lines)  
* pkg/custommodels/script.go (1,104 lines)

Because these reside inside fmt.Sprintf raw strings, they bypass TypeScript type-checking, oxlint, syntax highlighting, and minification, and rely on fragile %% format escaping.

* **Actionable Fix:** Move all injected client scripts into a dedicated workspace package (e.g., injected- runtime/src/), author them in TypeScript \+ CSS modules, bundle them into a single IIFE via Vite/esbuild at build time, and embed the compiled output into Go using //go:embed dist/injected.js. Pass runtime configuration (EnhancementsConfig, GUIConfig, sessionToken) as a single JSON bootstrap object (window.\_\_SWISS\_BOOTSTRAP\_\_ \= ...) rather than string-interpolating variables into code.

### **2.2 Split pkg/webgui/server.go (7,712 Lines) into Domain Sub-Routers**

pkg/webgui/server.go registers over 140 mux.HandleFunc endpoints and mixes HTTP transport concerns, OS path-jailing (validateJailedPath, find8dot3Aliases), memos file I/O, OAuth flows, and IPC fallback logic in a single file.

* **Actionable Fix:** Partition pkg/webgui/ by domain:  
  * handlers\_accounts.go (/api/accounts/\*, /api/switch, /api/totp, /api/fingerprint)  
  * handlers\_quota\_rules.go (/api/quota/\*, /api/rules/\*, /api/models/available)  
  * handlers\_custom\_models.go (/api/custom\_models/\*, cloudCodeDispatch)  
  * handlers\_gui\_enhancements.go (/api/gui/\*, /api/enhancements/\*, /api/extensions/\*, /api/conversations/\*)  
  * handlers\_files\_memos.go (/api/files/\*, /api/memos/\*, plus moving the 500-line path-jail validator into pkg/system/pathjail.go)  
  * handlers\_settings\_auth.go (/api/auth/\*, /api/settings/\*, /api/system/\*, /api/vault/\*)

### **2.3 Eliminate Dual-Store Split-Brain Between daemon and webgui**

Both daemon.NewDaemon (pkg/daemon/daemon.go:88-165) and webgui.NewServer (pkg/webgui/server.go:94-135) independently instantiate gui.NewStore(""), custommodels.NewStore(""), enhancements.NewStore(""), vault.NewManager("", ""), and revival.NewEngine("", 0). Furthermore, every handler in pkg/webgui/server.go and cmd/swiss/main.go repeats the boilerplate pattern:

Go  
if err := s.client.Call("swiss.someMethod", req, \&res); err \!= nil {  
    store, \_ := keyring.NewStore("")  
    // 20-40 lines of duplicated direct-disk fallback logic  
}

When swiss daemon \--web runs in the same OS process (cmd/swiss/main.go:196-206), webgui.Server still serializes requests over the Unix domain socket to daemon.Daemon and holds separate in-memory store instances that can drift from the daemon's state.

* **Actionable Fix:**  
  1. Introduce a shared BackendService interface (or pass \*daemon.Daemon directly into webgui.NewServer when running in-process) so \--web calls daemon methods in-memory without Unix socket serialization or duplicate store instantiation.  
  2. Consolidate the UpdateAccount\* combinatorial explosion in pkg/keyring/store.go (which currently has 11 overlapping methods: UpdateAccount, UpdateAccountDetails, UpdateAccountFull, UpdateAccountStatus, UpdateAccountStatusWithError, UpdateAccountWithStatus, UpdateAccountWithTier, UpdateAccountTokens, UpdateAccountTokensWithExpiry, UpdateAccountQuotaMetadata, UpdateAccountTokensAndMetadata) into a single functional-options or patch-struct method: UpdateAccount(email string, patch AccountPatch) error.

### **2.4 Decompose Frontend Mega-Components**

Several React pages have grown into multi-thousand-line monoliths with dozens of top-level useState hooks and inline styles:

* frontend/src/pages/CustomModelsPage.tsx (3,475 lines, **74 useState calls**) embeds the main model grid, the Add/Edit Custom Model modal, the Quota & Pricing configuration drawer, the GCP Vertex Service Account modal, and the Security Audit report viewer in one component.  
* frontend/src/pages/AppEnhancementsPage.tsx (2,681 lines), GitHubWorkspacePage.tsx (2,368 lines), QuotaDashboardPage.tsx (2,349 lines), TokenMonitorPage.tsx (2,129 lines), and SystemSettingsPage.tsx (2,058 lines, 47 useState calls).  
* **Actionable Fix:**  
  * Split CustomModelsPage.tsx into CustomModelsGrid.tsx, CustomModelEditModal.tsx (using a single useReducer for form state instead of 38 individual useState hooks), GcpVertexConfigModal.tsx, and QuotaPricingSection.tsx.  
  * Split AppEnhancementsPage.tsx along its existing 3 top-ribbon tabs (ChatViewEnhancementsTab.tsx, ProjectPanelEnhancementsTab.tsx, AuxiliaryPanelEnhancementsTab.tsx).  
  * Extract the repeated segmented-control header buttons in frontend/src/App.tsx:266-435 into a reusable SegmentedTabBar component.

## **3\. Runtime Efficiency & Performance Optimizations**

### **3.1 Consolidate In-IDE DOM MutationObserver and setInterval Loops**

When GenerateScriptWithCustomModels (pkg/gui/styler.go:2700) injects into the Antigravity Electron renderer, it starts **5 separate MutationObserver instances** (styler.go:2620, enhancements/script.go:3953, custommodels/script.go:1088, auxiliary.go:8108, onboarding\_accept.go:44) all observing document.body with { childList: true, subtree: true }, plus **11 concurrent setInterval timers** running every 250ms to 5,000ms:

* syncHostUserStatus (enhancements/script.go:126) traverses up to **3,500 React Fiber nodes via BFS** (visited.size \< 3500\) every 3,000ms on the main UI thread.  
* styler.go:918 polls syncCurrentConvoPath every 1,000ms and triggerSwissUpdate every 2,000ms (styler.go:2678).  
* github\_extension\_script.go:3882-3885 runs 3 separate timers (1500ms, 3000ms, 30000ms).  
* **Actionable Fix:**  
  1. Replace the 5 MutationObserver instances with a **single shared SwissDOMScheduler** in the injected runtime that batches DOM mutations inside a single requestAnimationFrame tick and dispatches filtered target nodes to registered plugins (styler, enhancements, auxiliary, customModels).  
  2. Cache the discovered React Fiber store/resolver reference in WeakRef after the first successful BFS traversal in syncHostUserStatus (enhancements/script.go:99-125), falling back to the 3,500-node Fiber walk only if the WeakRef is dereferenced as undefined.

### **3.2 Coordinate Frontend Polling & Background Fan-Out**

In the React GUI, multiple overlapping poll loops fire against 127.0.0.1:8765:

* App.tsx:86-92 runs loadAllData() every 10s, firing 5 parallel HTTP requests (getStatus, getFleetQuota, getRules, getGUIProjects, getAccounts). Notably, getFleetQuota already returns the full redacted account list (fleet.accounts), making the simultaneous api.getAccounts() call redundant whenever fleet.accounts is non-empty (App.tsx:41-48).  
* When QuotaDashboardPage.tsx:336 and AccountDetailModal.tsx:452 are mounted simultaneously, both run independent checkRecovery timers every 2.5s–4s.  
* electron/main.js:900 polls the daemon every 15s for the system tray menu (updateTrayMenu).  
* **Actionable Fix:**  
  1. Drop api.getAccounts() from the 10s hot loop in App.tsx:86 and fetch it only on initial bootstrap or when fleet.accounts is empty.  
  2. Add a lightweight Server-Sent Events (SSE) endpoint (GET /api/events) on webgui.Server that pushes state-change notifications (quota\_updated, account\_switched, config\_updated) to both the React frontend and electron/main.js, reducing idle polling frequency from 2.5s–10s to 60s fallback heartbeats.

### **3.3 Reduce Redundant Process Scans & Reconciliations on Status Calls**

Every call to GET /api/status (pkg/webgui/server.go:890) or swiss.getStatus (pkg/daemon/daemon.go:286) invokes d.Shield.FindAntigravityUIProcesses() and d.Keyring.ReconcileActiveAccount(...), which scans /proc (or spawns process enumeration on Windows) and queries SQLite (state.vscdb / cloud\_accounts.db). Because both the Electron tray and the React UI poll /api/status every 10–15 seconds, this repeats full process-tree and SQLite reads continuously.

* **Actionable Fix:** Cache the reconciled host PID and active surface account inside Daemon with a 5-second TTL (or invalidate immediately on switchMu events), matching the 5-second cachedPortExpiresAt pattern already used in pkg/gui/injector.go:116.

### **3.4 Bound Memory in token\_monitor.go Step Cache**

In pkg/webgui/token\_monitor.go:130-134, transcriptCacheMap stores cachedTranscriptStats (including steps \[\]rawStepEvent for every LLM step across all historical transcript.jsonl files) in an unbounded package-level map. Over months of heavy agent usage, deleted or archived conversations remain in transcriptCacheMap for the lifetime of the server.

* **Actionable Fix:** Sweep entries from transcriptCacheMap and convModelCacheMap whose file paths were not seen during the current directory walk in handleTokensSummary.

## **4\. UX & Intuitiveness Improvements**

### **4.1 Replace Sparse Integer Tool IDs with Semantic Route & Tab Identifiers**

In frontend/src/App.tsx:24 and NavRail.tsx:18, navigation state uses sparse magic integers with historical gaps:

* 0: Account Switcher (1, 5, 6 are dead/legacy IDs redirected via useEffect in App.tsx:191-198 to 9 or 0\)  
* 2: System Settings, 3: Custom Models, 4: UI Enhancements, 7: Extensions, 8: Token Monitor, 9: Utilities, 10: Remote Connect, 11: ACP Adapter.  
  Furthermore, deep-linking (window.location.hash or URL state) is not bound to currentTool / currentTab, so refreshing the page in a browser always resets the user back to currentTool \= 0 (Account Switcher), and browser Back/Forward buttons do not work.  
* **Actionable Fix:**  
  1. Replace magic numbers with a string union type:  
     TypeScript  
     export type ToolId \= 'switcher' \| 'models' \| 'tokens' \| 'bridge' \| 'acp' \| 'extensions' \| 'enhancements' \| 'utilities' \| 'settings'

  2. Synchronize \[currentTool, activeSubTab\] with window.location.hash (e.g., \#/models/source, \#/bridge/gemini) so page reloads preserve context and Electron tray navigation (namedRouteMap in App.tsx:114-137) maps directly without integer translation tables.

### **4.2 Unify Fragmented Settings Surfaces**

Configuration is currently split across **three separate top-level locations** that users must hunt through:

> 1. **Account Switcher → Rules Tab** (SwitcherSettingsPage.tsx, currentTool \=== 0 && currentTab \=== 2): Contains auto-switch thresholds, polling intervals, warmup lead time, default native/custom model fallbacks, and multi-app fleet sync mode.  
> 2. **UI Enhancements** (AppEnhancementsPage.tsx, currentTool \=== 4): Contains Chat View settings, Project Panel styling, Auxiliary Panel toggles, AND host flags like apply\_enhancements\_to\_vscode and session\_resume\_prompt.  
> 3. **System Settings** (SystemSettingsPage.tsx, currentTool \=== 2): Contains General, Path & Storage, Error & Privacy, and About.  
* **Actionable Fix:**  
  * Add a global **Cmd/Ctrl \+ K Quick Command & Settings Search Palette** in the React GUI so users can type *"warmup"*, *"debug port"*, *"resume prompt"*, or *"prune"* and jump straight to the exact setting card and tab.  
  * Add cross-link breadcrumbs between **Account Switcher → Rules → Model Source Hierarchy** and **Custom Models → Model Source Tab** (ModelSourceTab.tsx), which configure closely coupled fallback behaviors in two different tools.

### **4.3 Standardize Top-Bar Portal Pattern vs. Inline Segmented Controls**

In App.tsx:256-265, CustomModelsPage (currentTool \=== 3\) and ACPAgentMeshPage (currentTool \=== 11\) inject their top-bar controls via ReactDOM.createPortal into \#custom-models-top-bar-portal and \#acp-adapter-top-bar-portal, whereas tools 4, 7, 8, 9, 10, and 2 hardcode their segmented tab buttons directly inside App.tsx:266-490.

* **Actionable Fix:** Standardize on a declarative PageHeaderConfig (or a single \<TopHeaderSlot\> context) so every page defines its own tabs and right-hand action buttons locally instead of splitting page header UI between App.tsx and individual page files.

## **5\. Repository Hygiene & Testing Modernization**

### **5.1 Reclaim \~11 GB of Workspace Bloat & Ghost Artifacts**

A disk audit of /home/david/Projects/Antigravity Swiss Knife revealed significant local clutter:

* **scratch/ (9.1 GB):** Contains hundreds of one-off debug scripts (capture\_\*.go, adversarial\_\*.mjs), extracted ASAR trees (asar\_extracted), screenshots, and logs.  
* **release/ (1.7 GB) & root binaries (swiss.exe 22.8 MB, bin/swiss.exe\~, bin/\* 154 MB):** Multi-OS build outputs and backup binaries sitting in the working tree.  
* **antigravity\_swiss/ (1.1 MB):** A leftover directory from the pre-Go Python architecture containing **only** \_\_pycache\_\_/\*.pyc files (cpython-312.pyc and cpython-314.pyc) with zero .py source files remaining.  
* **Tracked build hash churn in pkg/webgui/dist/assets/:** git status shows deleted/untracked hashed bundle pairs (index-DXHxVtKy.js vs index-D\_wpw6zr.js) alongside 45 modified files from the in-progress A2A/CLI integration wave.  
* **Actionable Fix:**  
  1. Delete the orphaned antigravity\_swiss/ directory and root swiss.exe / bin/swiss.exe\~.  
  2. Archive or purge scratch/ (9.1 GB) and old release/ artifacts (0.1.0/0.2.0 bundles), saving \~10.8 GB immediately.

### **5.2 Complete the Transition from Source-Grep Tests to Behavioral Mounting**

7 of the 19 test files in frontend/src/tests/ (and several in pkg/gui/gui\_test.go and pkg/plugins/auxiliary\_test.go) still use fs.readFileSync(..., 'utf8') and substring/regex assertions against .tsx or .go source code. These tests break whenever code is refactored into subcomponents or hooks, even when runtime behavior is unchanged.

* **Actionable Fix:** Continue the M8 test modernization pattern (using happy-dom / @testing-library/react in frontend/src/tests/ and the Node VM harness used in pkg/plugins/community\_extensions\_runtime\_test.go) so tests assert against rendered DOM state and executed JS behavior rather than raw source text.

## **6\. Recommended Execution Roadmap**

> 1. **Phase 1 — Immediate Hygiene & Quick Wins (Low Risk, High Impact):**  
   * Remove ghost antigravity\_swiss/ .pyc tree, bin/swiss.exe\~, root swiss.exe, and prune scratch/ (reclaims \~10.8 GB).  
   * Drop redundant api.getAccounts() from the 10s poll in frontend/src/App.tsx and cache FindAntigravityUIProcesses() / ReconcileActiveAccount() for 5s on /api/status.  
> 2. **Phase 2 — Frontend Routing & Component Decomposition:**  
   * Replace numeric currentTool IDs (0..11) with typed string routes synced to window.location.hash and extract SegmentedTabBar.  
   * Split CustomModelsPage.tsx (3,475 LOC, 74 useStates) and AppEnhancementsPage.tsx (2,681 LOC) into focused sub-modals and tab views.  
> 3. **Phase 3 — Backend Consolidation (pkg/webgui & pkg/keyring):**  
   * Split pkg/webgui/server.go (7,712 LOC) into 6 domain handler files \+ pkg/system/pathjail.go.  
   * Wire in-process daemon.Daemon directly into webgui.Server when launched with swiss daemon \--web, and unify the 11 UpdateAccount\* methods in pkg/keyring/store.go.  
> 4. **Phase 4 — Injected Runtime Build Pipeline:**  
   * Migrate the \~19.8k lines of inline JS/CSS in pkg/plugins/, pkg/enhancements/, pkg/gui/styler.go, and pkg/custommodels/script.go into a typed, Vite-bundled injected-runtime/ package with a single shared MutationObserver scheduler.

Would you like me to start executing **Phase 1** (cleaning up the orphaned antigravity\_swiss/ directory, stray binaries, and the redundant 10s poll/status scan) or begin decomposing pkg/webgui/server.go and CustomModelsPage.tsx first?