# Antigravity Swiss Knife — Comprehensive Codebase Review & Optimization Plan (2026-10-30)

> **Purpose**: Self-contained architectural, performance, UX, and hygiene review of `/home/david/Projects/Antigravity Swiss Knife` (`HEAD @ 1d3ea71` + 65-file dirty working tree), cross-referenced against `HANDOFF_OPUS_REVIEW_2026_10_24.md` and `review.md`. Designed to be picked up directly inside the Antigravity app for phased execution.

---

## 1. Quantitative Baseline & Repository Health

| Subsystem | Key Files | Size / LOC | Primary Bottleneck | Target Architecture |
|---|---|---|---|---|
| **In-IDE Injected JS/CSS** | `pkg/plugins/auxiliary.go` (8,145), `pkg/enhancements/script.go` (4,014), `pkg/plugins/github_extension_script.go` (3,892), `pkg/gui/styler.go` (2,726), `pkg/custommodels/script.go` (1,104) | **19,881 LOC** | Raw Go backtick strings bypassing TS/linter, running 5 `MutationObserver`s & 11 `setInterval` timers in the host renderer | Standalone TypeScript + CSS package (`injected-runtime/`) bundled via Vite/esbuild and embedded via `//go:embed` |
| **WebGUI HTTP Server** | `pkg/webgui/server.go` (7,712), `pkg/webgui/token_monitor.go` (2,644) | **10,356 LOC** | 140+ routes, OS path-jail logic, memos I/O, OAuth, and 20–40 line IPC fallback blocks in one file | 6 domain handler files + `pkg/system/pathjail.go` + unified in-process service interface |
| **Daemon & Keyring Sync** | `pkg/daemon/daemon.go` (2,165), `pkg/keyring/store.go` (1,810), `pkg/keyring/sync.go` (1,680) | **5,655 LOC** | Dual-instantiated stores between `daemon` and `webgui`; 11 overlapping `UpdateAccount*` methods | Shared in-process daemon reference when running `--web`; single `UpdateAccount(email, AccountPatch)` API |
| **React 19 Frontend** | `CustomModelsPage.tsx` (3,475), `AppEnhancementsPage.tsx` (2,681), `GitHubWorkspacePage.tsx` (2,368), `QuotaDashboardPage.tsx` (2,349), `TokenMonitorPage.tsx` (2,129), `SystemSettingsPage.tsx` (2,058) | **24,800+ LOC** | Mega-components (up to 74 `useState` hooks in one file), sparse integer routing (`0..11`), redundant 10s polling | Hash-synced typed routes, extracted modal/tab subcomponents, `useReducer` form state, SSE push notifications |
| **Workspace Footprint** | `scratch/` (9.1 GB), `release/` (1.7 GB), `bin/` (154 MB), `swiss.exe` (22.8 MB), `antigravity_swiss/` (1.1 MB) | **11.0 GB** | 9.1 GB of debug dumps/ASAR extractions, old release bundles, and orphaned Python `.pyc` tree | Pruned workspace reclaiming ~10.8 GB + strict `.gitignore` staging |

---

## 2. Cross-Reference Audit: Prior Opus 5.5 Wave (`S1`–`S9`, `P1`–`P6`) & Current Dirty Tree

### 2.1 Verification of Prior Security & Performance Fixes (`HANDOFF_OPUS_REVIEW_2026_10_24.md`)
All checklist items from Section 3 of `HANDOFF_OPUS_REVIEW_2026_10_24.md` were directly inspected in the current working tree:

1. **`S1` (`getClientIP` XFF Brute-Force Bypass — `pkg/bridge/auth.go:254-285`) — VERIFIED INTACT**:
   - `getClientIP(r)` extracts `remoteIP` from `r.RemoteAddr` first and only consults `Cf-Connecting-Ip`, rightmost `X-Forwarded-For`, or `X-Real-IP` when `net.ParseIP(remoteIP).IsLoopback()` is true.
   - `sweepLockoutStateLocked` (`pkg/bridge/auth.go:302-348`) also caps map growth at `maxTrackedLockoutIPs`.
2. **`S2` & `S3` (OAuth PIN Lockout & `authorization_code` Validation — `pkg/bridge/oauth.go:881-978`) — VERIFIED INTACT**:
   - `ServeOAuthToken` (`refresh_token` and `client_credentials` grants at lines 948–978) delegates to `s.authMgr.VerifyPIN(r, candidate)`, returning HTTP 429 when locked out and accepting `am.token` (`GetRawAuthToken()`), PIN, or App Password via constant-time comparison.
   - PKCE `code_challenge_method` strictly rejects unsupported methods with `400 invalid_request`.
3. **`S5` (`core.LoadConfig` Fail-Closed on Corrupt JSON — `pkg/core/config.go:201-204`) — VERIFIED INTACT**:
   - Returns `DefaultConfig(), fmt.Errorf("failed to parse config file %s: %w", cfgPath, err)` on `json.Unmarshal` failure, ensuring `/api/auth/unlock` fails closed with HTTP 500 rather than minting a free session token.
4. **`S7` & `P2` (Remote ReverseProxy `Rewrite` & Loopback TLS — `pkg/bridge/remote.go:428-452`) — VERIFIED INTACT**:
   - Uses `httputil.ReverseProxy.Rewrite` (replacing deprecated `Director`), sets `Origin`, `Referer`, `x-codeium-csrf-token`, and strips `Accept-Encoding` on HTML entry paths. Uses shared package-level `http.Transport` instances (`getRemoteTransport`).
5. **`S8` (`isTrustedSender` & Fallback Page in `electron/main.js:167-194`) — VERIFIED INTACT**:
   - `isTrustedSender(event, targetWindow)` validates `targetWindow.webContents`, `event.senderFrame === mainFrame`, and `isTrustedDaemonOrigin(event.senderFrame?.url)`.
   - `isTrustedDaemonOrigin` explicitly checks `isTrustedConnectingFallbackUrl(targetUrl)` (`trustedConnectingFallbackUrl`), ensuring the "Retry Connection" button on the `data:` fallback screen works when the daemon is starting up.
6. **`P3` & `P6` (Auth Middleware Hot-Path & Token Map Pruning — `pkg/webgui/auth_middleware.go:270-346`) — VERIFIED INTACT**:
   - `validateRequestAuth` checks in-memory `daemonSessionToken` and `s.sessionTokens` via `subtle.ConstantTimeCompare`, deletes expired tokens on lookup, and exits early at line 336 (`if appPasswordCandidate == "" && (candidate == "" || isSessionTokenShaped(candidate)) { return false }`) before touching disk or running 100k-round PBKDF2.
7. **CI Go Version Pinning (`.github/workflows/race.yml`, `.github/workflows/release.yml`) — PARTIALLY UPDATED**:
   - Both workflows were updated from `1.23.x`/`1.24.x` to `go-version: '1.26.x'`, though switching to `go-version-file: 'go.mod'` remains cleaner so future Go bumps do not drift.

### 2.2 Audit of Current Uncommitted Working Tree (65 Modified + 6 Untracked Files)
The working tree currently contains ~3,989 insertions and ~1,006 deletions across three active workstreams that should be cleaned up and committed into logical units before structural refactoring:
1. **A2A Protocol & JSON-RPC Transport**:
   - Modified: `pkg/bridge/a2a.go`, `pkg/webgui/a2a.go`, `pkg/webgui/auth_middleware.go`, `pkg/webgui/server.go`.
   - Untracked: `pkg/bridge/a2a_protocol.go`, `pkg/bridge/a2a_protocol_test.go`, `pkg/bridge/mcp_task_assignment_test.go`, `pkg/webgui/a2a_jsonrpc_routes_test.go`.
2. **CLI (`agy`) Cloud Code Proxy Integration (`CLOUD_CODE_URL`)**:
   - Modified: `pkg/custommodels/cli_integration.go`, `pkg/custommodels/proxy.go`, `pkg/custommodels/proxy_catalog.go`, `pkg/custommodels/store.go`, `pkg/process/antigravity_proxy.go`, plus `pkg/process/antigravity_proxy_test.go` (untracked).
3. **Frontend UI & Token Monitor Range Enhancements**:
   - Modified: `pkg/webgui/token_monitor.go`, `frontend/src/pages/TokenMonitorPage.tsx`, `ACPAgentMeshPage.tsx`, `CustomModelsPage.tsx`, `GeminiBridgePage.tsx`, `QuotaDashboardPage.tsx`, etc.
   - **Cleanup Required Before Commit**:
     - `frontend/src/components/AcpAgentModals.tsx` has mixed CRLF line endings (`warning: CRLF will be replaced by LF`).
     - `pkg/webgui/dist/assets/` has stale/untracked build bundles (`index-DXHxVtKy.js` deleted, `index-DltUxohr.js` staged-deleted, `index-D_wpw6zr.js` untracked). Run a clean `npm run build` in `frontend/` and stage the single canonical `pkg/webgui/dist` output.

---

## 3. Structural & Architectural Optimizations

### 3.1 Migrate In-IDE Injected Scripts from Go Backticks to a Bundled TypeScript Package (`~19.8k LOC`)
**Problem**:
Five Go files hold nearly 20,000 lines of raw JavaScript and CSS inside backtick strings:
- `pkg/plugins/auxiliary.go` (`8,145 lines`)
- `pkg/enhancements/script.go` (`4,014 lines`)
- `pkg/plugins/github_extension_script.go` (`3,892 lines`)
- `pkg/gui/styler.go` (`2,726 lines`)
- `pkg/custommodels/script.go` (`1,104 lines`)

Because these scripts are concatenated at runtime in `GenerateScriptWithCustomModels` (`pkg/gui/styler.go:2700-2725`) via `fmt.Sprintf`, they have zero TypeScript type safety, no IDE linting, manual `%%` escaping hazards, and cannot share utility functions (each script re-implements `getSwissAuthHeaders()`, DOM query helpers, and theme detection).

**Optimization Plan**:
1. Create `injected/` (or `frontend/src/injected/`) with modular TypeScript entrypoints:
   - `core/auth.ts` (session token bootstrap & `fetch`/`XMLHttpRequest` auth header interceptor)
   - `core/domScheduler.ts` (shared `MutationObserver` + `requestAnimationFrame` batcher)
   - `modules/styler.ts`, `modules/enhancements.ts`, `modules/auxiliary.ts`, `modules/githubExtension.ts`, `modules/customModels.ts`
2. Build into a single minified IIFE bundle (`pkg/gui/dist/injected.bundle.js` and `injected.bundle.css`) via Vite/esbuild.
3. Embed into Go using `//go:embed dist/injected.bundle.js` and inject runtime config via a single JSON preamble:
   ```js
   window.__SWISS_BOOTSTRAP__ = { sessionToken, guiConfig, enhancementsConfig, customModelsConfig };
   ```

### 3.2 Decompose `pkg/webgui/server.go` (`7,712 LOC`) into Domain Files
**Problem**:
`pkg/webgui/server.go` registers 140+ HTTP endpoints and mixes HTTP routing, filesystem path-jailing (`lines 4896–5520`), memos CRUD (`lines 4689–4895`), OAuth flows (`lines 3957–4040`), and Cloud Code proxy dispatch (`lines 3337–3448`).

**Optimization Plan**:
Split `pkg/webgui/server.go` into focused files within `pkg/webgui/` (zero package boundary changes required):
- `server.go` (~450 LOC): `Server` struct, lifecycle (`NewServer`, `Start`, `Stop`), `setupRoutes()`, and middlewares (`corsMiddleware`, `maxBytesMiddleware`).
- `handlers_accounts.go`: `/api/accounts/*`, `/api/switch`, `/api/totp`, `/api/fingerprint`, `/api/oauth/google/*`, `/api/vault/*`.
- `handlers_quota_rules.go`: `/api/quota/*`, `/api/rules/*`, `/api/surfaces`, `/api/models/available`.
- `handlers_custom_models.go`: `/api/custom_models/*`, `cloudCodeDispatch`, CLI gateway setup.
- `handlers_gui_enhancements.go`: `/api/gui/*`, `/api/enhancements/*`, `/api/extensions/*`, `/api/conversations/*`, `/api/desktop/*`, `/api/vscode/*`.
- `handlers_files_memos.go`: `/api/files/*`, `/api/memos/*`.
- `pathjail.go` (or move to `pkg/system/pathjail.go`): `validateJailedPath`, `normaliseUserPath`, `find8dot3Aliases`, `isStrictSubpath`.

### 3.3 Eliminate Dual-Store Split-Brain Between `daemon.Daemon` and `webgui.Server`
**Problem**:
1. In `cmd/swiss/main.go:196-206`, when `swiss daemon --web` is launched (the standard mode used by `electron/daemon-manager.js`), **both** `daemon.NewDaemon` (`pkg/daemon/daemon.go:88`) and `webgui.NewServer` (`pkg/webgui/server.go:94`) instantiate their own independent copies of:
   - `gui.NewStore("")`
   - `custommodels.NewStore("")`
   - `enhancements.NewStore("")`
   - `vault.NewManager("", "")`
   - `revival.NewEngine("", 0)`
2. Every handler in `pkg/webgui/server.go` and CLI command in `cmd/swiss/main.go` duplicates 20–40 lines of fallback logic (`if err := s.client.Call("swiss.xyz", ...); err != nil { store, _ := keyring.NewStore("") ... }`), as seen in `handleAccounts` (`lines 971-992`) and `handleFleetQuota` (`lines 1912-1953`).
3. `pkg/keyring/store.go` has **11 overlapping account update methods** (`UpdateAccount`, `UpdateAccountDetails`, `UpdateAccountFull`, `UpdateAccountStatus`, `UpdateAccountStatusWithError`, `UpdateAccountWithStatus`, `UpdateAccountWithTier`, `UpdateAccountTokens`, `UpdateAccountTokensWithExpiry`, `UpdateAccountQuotaMetadata`, `UpdateAccountTokensAndMetadata` across `lines 1212–1542`).

**Optimization Plan**:
1. Add `webgui.NewServerWithDaemon(addr string, d *daemon.Daemon)` so that when `--web` runs inside the daemon process, `webgui.Server` shares the daemon's live store pointers and invokes daemon methods directly in-memory without Unix socket round-trips or divergent in-memory caches.
2. Centralize the standalone fallback into a single `DaemonOrLocalService` adapter so HTTP handlers and CLI commands call one method without repeating `if err := s.client.Call(...)` fallback blocks.
3. Replace the 11 `UpdateAccount*` variants in `pkg/keyring/store.go` with a single `ApplyAccountPatch(email string, patch AccountPatch) (*Account, error)` method using pointer fields for optional updates.

### 3.4 Decompose React 19 Mega-Pages & Standardize State Management
**Problem**:
Top-level page files in `frontend/src/pages/` contain massive state trees and inline modal definitions:
- `CustomModelsPage.tsx`: **3,475 LOC, 74 `useState` hooks** (model grid + add/edit modal + quota/pricing drawer + GCP Vertex modal + security audit report modal).
- `AppEnhancementsPage.tsx`: **2,681 LOC, 14 `useState` hooks** (3 full pages of settings in one file).
- `GitHubWorkspacePage.tsx`: **2,368 LOC, 26 `useState` hooks**.
- `QuotaDashboardPage.tsx`: **2,349 LOC, 18 `useState` hooks**.
- `TokenMonitorPage.tsx`: **2,129 LOC, 28 `useState` hooks**.
- `SystemSettingsPage.tsx`: **2,058 LOC, 47 `useState` hooks**.
- `SwitcherSettingsPage.tsx`: **1,890 LOC, 43 `useState` hooks**.
- `AccountDetailModal.tsx`: **1,842 LOC, 43 `useState` hooks**.

**Optimization Plan**:
1. **`CustomModelsPage.tsx`**:
   - Extract `CustomModelFormModal.tsx` and replace its ~38 individual form `useState` hooks (`lines 122–180`) with a single typed `useReducer` (`modelFormReducer`).
   - Extract `GcpVertexModal.tsx` (`lines 188–205` state + modal JSX) and `CustomModelCard.tsx`.
2. **`AppEnhancementsPage.tsx` & `SystemSettingsPage.tsx`**:
   - Split `AppEnhancementsPage.tsx` into `enhancements/ChatViewTab.tsx`, `enhancements/ProjectPanelTab.tsx`, and `enhancements/AuxiliaryPanelTab.tsx`.
   - Split `SystemSettingsPage.tsx` into `settings/GeneralSettingsTab.tsx`, `settings/PathStorageTab.tsx`, `settings/PrivacyDiagnosticsTab.tsx`, and `settings/AboutTab.tsx`.

---

## 4. Runtime Efficiency & Performance Optimizations

### 4.1 Unify the 5 In-IDE `MutationObserver`s & Cache React Fiber Walk
**Problem**:
Inside the injected Antigravity IDE renderer (`GenerateScriptWithCustomModels`), **5 separate `MutationObserver`s** (`pkg/gui/styler.go:2620`, `pkg/enhancements/script.go:3953`, `pkg/custommodels/script.go:1088`, `pkg/plugins/auxiliary.go:8108`, `pkg/plugins/onboarding_accept.go:44`) watch `document.body` (`{ childList: true, subtree: true }`), alongside **11 `setInterval` loops**:
- Every streaming token in an active Antigravity chat mutates the DOM, firing all 5 `MutationObserver` callbacks back-to-back.
- `syncHostUserStatus` (`pkg/enhancements/script.go:86-126`) runs every **3,000ms** and performs a Breadth-First Search over up to **3,500 React Fiber nodes** (`while (queue.length > 0 && visited.size < 3500)`) from `#root` to locate the internal Zustand/Redux store and resolver.

**Optimization Plan**:
1. **Cache the React Fiber store reference**: In `syncHostUserStatus` (`pkg/enhancements/script.go:86-126`), store the discovered `store` and `resolver` objects on `window.__swissCachedHostStore` (or via `WeakRef`). On subsequent 3s ticks, read directly from `window.__swissCachedHostStore` in $O(1)$ time, re-running the 3,500-node Fiber BFS only if the reference becomes invalid.
2. **Single `SwissDOMScheduler`**: Coalesce the 5 `MutationObserver` instances into a single observer that deduplicates mutated elements and schedules plugin callbacks inside one `requestAnimationFrame` budget (throttled to at most 10 Hz during active LLM token streaming).

### 4.2 Eliminate Redundant Frontend Polling & Add SSE Push Events
**Problem**:
1. In `frontend/src/App.tsx:76-103`, `loadAllData()` runs every **10 seconds** (`setInterval` at line 162) and executes 5 concurrent HTTP requests:
   ```ts
   const [s, f, r, p, accs] = await Promise.allSettled([
     api.getStatus(),
     api.getFleetQuota(),
     api.getRules(),
     api.getGUIProjects(),
     api.getAccounts(),
   ])
   ```
   However, `effectiveAccounts` (`App.tsx:41-48`) only uses `directAccounts` (`api.getAccounts()`) when `fleet.accounts` is empty! Whenever `getFleetQuota()` succeeds, `api.getAccounts()` is 100% redundant, doubling keyring store lock acquisitions and JSON serialization on every poll.
2. `api.getRules()` and `api.getGUIProjects()` rarely change unless the user edits settings or creates a workspace, yet both are polled every 10s.
3. When viewing an account (`AccountDetailModal.tsx:452`) on the dashboard (`QuotaDashboardPage.tsx:336`), two separate 2.5s–4s `checkRecovery` timers run concurrently alongside the 10s `App.tsx` poll and the 15s `electron/main.js:900` tray poll.

**Optimization Plan**:
1. In `frontend/src/App.tsx:76-103`, split `loadAllData()` into:
   - `bootstrapData()` (on mount: fetches status, fleet, rules, projects, and fallback accounts only if `fleet.accounts` is empty).
   - `pollHotData()` (every 10s: fetches **only** `api.getStatus()` and `api.getFleetQuota()`, cutting background HTTP request volume by **60%**).
2. Add a lightweight Server-Sent Events endpoint (`GET /api/events`) on `pkg/webgui/server.go` to push `quota_updated`, `account_switched`, and `config_changed` events to the frontend and Electron main process.

### 4.3 Cache Process Enumeration & Active Account Reconciliation in `/api/status`
**Problem**:
Every `GET /api/status` (`pkg/webgui/server.go:890`) and `swiss.getStatus` (`pkg/daemon/daemon.go:286`) call synchronously runs:
1. `d.Shield.FindAntigravityUIProcesses()` (scans `/proc` on Linux or process snapshots on Windows/macOS).
2. `d.Keyring.ReconcileActiveAccount(...)` (reads SQLite databases `state.vscdb` / `cloud_accounts.db` and config files).
3. `keyring.ResolveRunningAntigravityAccountRedacted("", "")` (re-scans surfaces again).

**Optimization Plan**:
Cache the computed status snapshot on `Daemon` for **3–5 seconds** (invalidated immediately whenever `switchAccount`, `importAccount`, or `setRuleConfig` mutates state), matching the 5-second `cachedPortExpiresAt` pattern already used in `pkg/gui/injector.go:116`.

### 4.4 Evict Stale Entries from `token_monitor.go` Global Caches
**Problem**:
`pkg/webgui/token_monitor.go:130-134` maintains package-level maps `transcriptCacheMap` and `convModelCacheMap` keyed by file path. While entries are updated when `modTimeNano` or `size` changes, deleted or pruned conversation files (e.g., after `cache prune`) are never evicted from memory.

**Optimization Plan**:
At the end of `handleTokensSummary`, pass the `seenPaths map[string]struct{}` collected during the directory walk to a `pruneTokenCaches(seenPaths)` helper that deletes any map keys no longer present on disk.

---

## 5. UX & Intuitiveness Improvements

### 5.1 Replace Sparse Magic Integer Tool IDs with Hash-Synced Semantic Routes
**Problem**:
`frontend/src/App.tsx:24` and `NavRail.tsx:18` represent the active tool with sparse integers (`0, 2, 3, 4, 7, 8, 9, 10, 11`), with legacy redirects in `App.tsx:191-198` (`if (currentTool === 1 || currentTool === 6) setCurrentTool(9); else if (currentTool === 5) setCurrentTool(0)`). Refreshing the browser window always resets navigation to `0` (Account Switcher), and deep-linking to a specific tool/tab is impossible.

**Optimization Plan**:
1. Define explicit route and sub-tab types in `frontend/src/types.ts`:
   ```ts
   export type ToolRoute =
     | 'switcher'
     | 'models'
     | 'tokens'
     | 'bridge'
     | 'acp'
     | 'extensions'
     | 'enhancements'
     | 'utilities'
     | 'settings'
   ```
2. Sync `[currentTool, activeSubTab]` with `window.location.hash` (e.g. `#/models/source`, `#/bridge/gemini`, `#/utilities/cache`) via a `useHashNavigation()` hook while keeping backward compatibility in `electronAPI.onNavigate` for numeric tray indices.

### 5.2 Unify Top-Bar Header Rendering (`createPortal` vs. Hardcoded Segmented Controls)
**Problem**:
In `frontend/src/App.tsx:256-490`, header rendering is split across two incompatible patterns:
- Tools `3` (`CustomModelsPage`) and `11` (`ACPAgentMeshPage`) render empty `<div id="...-top-bar-portal" />` nodes in `App.tsx` and use `ReactDOM.createPortal` inside the child page.
- Tools `2, 4, 7, 8, 9, 10` hardcode 220+ lines of repetitive segmented button JSX directly inside `App.tsx`.

**Optimization Plan**:
Extract a single reusable `<SegmentedTabBar items={...} activeId={...} onSelect={...} />` component and standardize how pages declare their top-ribbon tabs and right-hand action slots.

### 5.3 Add Global Settings Search (`Cmd/Ctrl + K`) & Cross-Link Model Hierarchy Settings
**Problem**:
Settings are distributed across **Account Switcher → Rules** (`SwitcherSettingsPage.tsx`), **Custom Models → Model Source** (`ModelSourceTab.tsx`), **UI Enhancements** (`AppEnhancementsPage.tsx`), and **System Settings** (`SystemSettingsPage.tsx`). Users frequently have to guess whether a setting (such as VS Code injection, session revival prompt, or model fallback priority) lives in Rules, Enhancements, or System Settings.

**Optimization Plan**:
1. Add a lightweight **`Cmd/Ctrl + K` Command & Settings Palette** indexing all setting titles, descriptions, and actions across all 9 tools for instant keyboard navigation.
2. Add a direct navigation link between **Account Switcher → Rules → Model Source Hierarchy** and **Custom Models → Model Source** so users understand how account-level native fallback and custom model routing interact.

---

## 6. Workspace Hygiene & Test Modernization

### 6.1 Reclaim ~10.8 GB of Disk Space
1. **Delete Orphaned Python Bytecode Directory (`antigravity_swiss/`, 1.1 MB)**:
   - Contains only `__pycache__/*.pyc` from the legacy Python implementation with zero `.py` files.
2. **Delete Stray Root/Backup Binaries**:
   - Remove `/home/david/Projects/Antigravity Swiss Knife/swiss.exe` (22.8 MB) and `bin/swiss.exe~`.
3. **Purge or Archive `scratch/` (9.1 GB) and Old `release/` Bundles (1.7 GB)**:
   - `scratch/` holds 9.1 GB of temporary unpacked ASAR archives and one-off debug scripts; `release/` holds superseded `0.1.0` and `0.2.0` packages.

### 6.2 Replace Remaining Source-Grep Tests with Behavioral Tests
7 of the 19 test files in `frontend/src/tests/` still read raw `.tsx` source files via `fs.readFileSync` and assert substrings. Any component decomposition (such as splitting `CustomModelsPage.tsx` or `App.tsx`) will trip those substring checks unless they are migrated to behavioral/component tests (using `happy-dom` / `@testing-library/react`) or updated alongside the refactor.

---

## 7. Phased Implementation Plan (Ready for Agent Execution)

### Phase 0: Commit/Clean Current Working Tree (Pre-Refactor Checkpoint)
- [ ] Normalize line endings on `frontend/src/components/AcpAgentModals.tsx` (CRLF → LF).
- [ ] Rebuild `frontend` (`cd frontend && npm run build`) so `pkg/webgui/dist/assets/` has a single clean bundle.
- [ ] Run `go test ./pkg/...` and `cd frontend && npm test` to verify the current A2A + CLI `CLOUD_CODE_URL` proxy changes and commit them cleanly.

### Phase 1: Workspace Hygiene & Quick Runtime Wins
- [ ] Remove `antigravity_swiss/`, root `swiss.exe`, and `bin/swiss.exe~`; clean `scratch/`.
- [ ] Update `frontend/src/App.tsx` so the 10s background poll only calls `getStatus()` and `getFleetQuota()`, calling `getAccounts()` only when `fleet.accounts` is empty.
- [ ] Add a 5-second TTL cache on `swiss.getStatus` / `GET /api/status` in `pkg/daemon/daemon.go` and `pkg/webgui/server.go`.
- [ ] Add stale-file eviction (`pruneTokenCaches`) to `transcriptCacheMap` and `convModelCacheMap` in `pkg/webgui/token_monitor.go`.
- [ ] Cache the discovered React Fiber store in `syncHostUserStatus` (`pkg/enhancements/script.go:86-126`) to eliminate the 3,500-node Fiber BFS on every 3-second tick.

### Phase 2: Frontend Routing, Header Unification & Component Decomposition
- [ ] Extract `<SegmentedTabBar />` in `frontend/src/components/SegmentedTabBar.tsx` and replace the 220 lines of duplicated tab button JSX in `frontend/src/App.tsx`.
- [ ] Introduce `ToolRoute` string slugs synced with `window.location.hash` in `App.tsx` and `NavRail.tsx`.
- [ ] Decompose `CustomModelsPage.tsx` (3,475 LOC) into `CustomModelFormModal.tsx` (`useReducer`), `GcpVertexModal.tsx`, and `CustomModelCard.tsx`.
- [ ] Decompose `AppEnhancementsPage.tsx` (2,681 LOC) into `ChatViewTab.tsx`, `ProjectPanelTab.tsx`, and `AuxiliaryPanelTab.tsx`.
- [ ] Update any source-grep tests in `frontend/src/tests/` that reference `CustomModelsPage.tsx` or `AppEnhancementsPage.tsx` directly.

### Phase 3: Go Backend Modularization (`pkg/webgui`, `pkg/daemon`, `pkg/keyring`)
- [ ] Split `pkg/webgui/server.go` (7,712 LOC) into `server.go`, `handlers_accounts.go`, `handlers_quota_rules.go`, `handlers_custom_models.go`, `handlers_gui_enhancements.go`, `handlers_files_memos.go`, and `pathjail.go`.
- [ ] Wire `webgui.NewServerWithDaemon` in `cmd/swiss/main.go` so `swiss daemon --web` shares live stores in-process.
- [ ] Consolidate the 11 `UpdateAccount*` methods in `pkg/keyring/store.go` into `ApplyAccountPatch(email string, patch AccountPatch)`.

### Phase 4: Injected CDP Runtime Bundling (`injected-runtime/`)
- [ ] Extract inline JS/CSS from `pkg/plugins/auxiliary.go`, `pkg/enhancements/script.go`, `pkg/plugins/github_extension_script.go`, `pkg/gui/styler.go`, and `pkg/custommodels/script.go` into a typed TypeScript workspace package with a single shared `SwissDOMScheduler` (`MutationObserver` + `requestAnimationFrame`).
- [ ] Embed the compiled bundle via `//go:embed` and verify all Node VM runtime tests in `pkg/plugins/`, `pkg/enhancements/`, and `pkg/gui/`.
