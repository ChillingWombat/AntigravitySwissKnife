# Project: Antigravity Swiss Knife Electron Migration

## Architecture

Antigravity Swiss Knife is migrated from a legacy Python PySide6 wrapper to a modern, self-contained standalone Electron desktop application. The application architecture consists of:
1. **Desktop Shell (Electron Main & Preload)**: Standalone Electron runtime managing application lifecycle, single-instance lock, system tray, window minimize-to-tray, and OS login item settings.
2. **Integrated Go Daemon (Bundled Sidecar)**: Electron directly supervises `bin/swiss daemon --web`, which serves the JSON-RPC daemon on Unix socket (`daemon.sock`) and the HTTP Web GUI + REST API on `127.0.0.1:8765`.
3. **Frontend (React 19 + TypeScript + Vite)**: Modern Google Gemini Material Design 3 dark theme single-page app embedded into Go binary and served directly on `127.0.0.1:8765`.
4. **Native System Tray**: DBus SNI / OS native tray icon with health status badges, restore actions, active account status, quick account switch submenu, and full quit action.
5. **System Startup Integration**: Native toggle in System Settings hooked to `app.setLoginItemSettings()` launching minimized to tray on boot across Linux, Windows, and macOS.
6. **Zero Python Runtime**: All 27 files in `antigravity_swiss/gui/` and Python desktop launch paths are completely deleted and retired.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                       Electron Desktop Shell                                │
│  ┌───────────────────────┐  ┌────────────────────────────────────────────┐  │
│  │ Native System Tray    │  │ Main Window (BrowserWindow)                │  │
│  │ • Minimize-to-tray    │  │ • Title: "Antigravity Swiss Knife"         │  │
│  │ • Account Switch menu │  │ • Loads: http://127.0.0.1:8765             │  │
│  │ • Quick restore       │  │ • Preload: contextBridge (loginItemSettings│  │
│  └───────────────────────┘  └────────────────────────────────────────────┘  │
│                                   │                                         │
│                      Child Process Supervision                              │
│                      (Lifecycle & Graceful SIGTERM)                         │
└───────────────────────────────────┼─────────────────────────────────────────┘
                                    ▼
       HTTP & REST API on 127.0.0.1:8765 + Unix Socket daemon.sock
                                    ▲
┌───────────────────────────────────┴─────────────────────────────────────────┐
│               Go Backend Daemon Sidecar (bin/swiss daemon --web)            │
│  ┌─────────────────────────────────┐   ┌─────────────────────────────────┐  │
│  │ Native Secret Service Switcher  │   │ Upstream Quota Poller Engine    │  │
│  │ (secret-tool service=gemini)    │   │ (cloudcode-pa.googleapis.com)   │  │
│  └─────────────────────────────────┘   └─────────────────────────────────┘  │
│  ┌─────────────────────────────────┐   ┌─────────────────────────────────┐  │
│  │ Process Lifecycle & Session Mgr │   │ Embedded Web GUI Server         │  │
│  │ (SingletonLock / app_storage)   │   │ (React 19 M3 dark frontend)     │  │
│  └─────────────────────────────────┘   └─────────────────────────────────┘  │
│  ┌─────────────────────────────────┐   ┌─────────────────────────────────┐  │
│  │ Device Fingerprint Virtualizer  │   │ Brain & Context Cache Optimizer │  │
│  │ (machineid / updaterId / pbtxt) │   │ (brain/ & conversations/ pruner)│  │
│  └─────────────────────────────────┘   └─────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Feature Inventory

Every requirement from `ORIGINAL_REQUEST.md` is inventoried and assigned:

| # | Feature | Description | Milestone | Source |
|---|---------|-------------|-----------|--------|
| F01 | `F_PY_RETIRE` | Completely delete all 27 legacy Python PySide6 GUI files (`antigravity_swiss/gui/`), remove `run_gui` from `__main__.py`, and clean test fixtures in `conftest.py` | M1 | ORIGINAL_REQUEST §R1 |
| F02 | `F_FRONTEND_BUILD_CLEAN` | Fix TypeScript unused variables in `ScheduledTemplatesPage.tsx` and ensure `frontend` builds cleanly to `pkg/webgui/dist` | M1 | ORIGINAL_REQUEST §R1 |
| F03 | `F_ELEC_SHELL` | Set up root `package.json`, install `electron` & dependencies, create `electron/main.js` and `electron/preload.js` | M2 | ORIGINAL_REQUEST §R1 |
| F04 | `F_SIDECAR_SUPERVISOR` | Electron main process supervises Go binary (`bin/swiss daemon --web`), verifies `/api/status`, and handles child lifecycle | M2 | ORIGINAL_REQUEST §R2 |
| F05 | `F_CLEAN_SHUTDOWN` | Graceful SIGTERM child termination on full exit, unlinks socket/lockfiles, ensures `pgrep swiss = 0` | M2 | ORIGINAL_REQUEST §R2 |
| F06 | `F_SINGLE_INSTANCE` | Enforce single-instance lock via `app.requestSingleInstanceLock()`, focusing existing window on second launch | M2 | Architecture Survey |
| F07 | `F_CLOSE_TO_TRAY` | Intercept window close ('X') event to hide window to system tray instead of quitting | M3 | ORIGINAL_REQUEST §R3 |
| F08 | `F_SYSTEM_TRAY` | Native system tray with icon, click/double-click restore, and context menu (Show, Active Account, Quick Switch, Settings, Quit) | M3 | ORIGINAL_REQUEST §R3 |
| F09 | `F_STARTUP_IPC` | Preload context bridge and main IPC handlers for `app.getLoginItemSettings()` and `app.setLoginItemSettings()` | M3 | ORIGINAL_REQUEST §R4 |
| F10 | `F_STARTUP_UI` | System Settings UI card with "Launch at System Startup (Minimized to Tray)" toggle switch | M3 | ORIGINAL_REQUEST §R4 |
| F11 | `F_BOOT_MINIMIZED` | Process `--minimized` CLI argument on boot to launch hidden to tray | M3 | ORIGINAL_REQUEST §R4 |
| F12 | `F_PACKAGING_CONFIG` | `electron-builder` configuration in root `package.json` for Linux (AppImage/deb), Windows (exe/nsis), and macOS (dmg/zip) bundling Go binary via `extraResources` | M4 | ORIGINAL_REQUEST §R5 |
| F13 | `F_DESKTOP_SCRIPTS` | Complete set of scripts in root `package.json` (`npm run desktop`, `npm run build`, `npm run pack`, `npm run dist`) | M4 | ORIGINAL_REQUEST §R5 |
| F14 | `F_AUTOMATED_VERIFY_SCRIPT` | Automated verification script `scripts/verify-desktop-e2e.js` running under XVFB verifying window creation, title "Antigravity Swiss Knife", `/api/status`, and zero dangling processes | M4 | ORIGINAL_REQUEST Acceptance Criteria |
| F15 | `F_E2E_VALIDATION` | 100% pass of E2E verification test suite (`npm run test:desktop`) and Go test suite (`go test ./pkg/... ./cmd/...`) | M5 | ORIGINAL_REQUEST Acceptance Criteria |
| F16 | `F_ADVERSARIAL_HARDENING` | Adversarial coverage hardening with Challengers and Forensic Auditor certifying zero Python runtime, zero external browsers, and zero orphaned processes | M5 | Acceptance Criteria & Audit |

---

## Milestones

| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| M1 | Legacy Python Retirement & Frontend Build Baseline | Features F01, F02. Delete `antigravity_swiss/gui/` (27 files), remove `gui` command from `__main__.py`, clean `conftest.py`, fix `ScheduledTemplatesPage.tsx` TypeScript errors, verify `npm run build` and `go test` pass. | none | DONE (Gate Iteration 1 PASS: Certified by Reviewers, Challengers, and Auditor; 71/71 Python unit tests pass, 16/16 Go packages pass, frontend builds cleanly) |
| M2 | Standalone Electron Shell & Go Sidecar Lifecycle | Features F03, F04, F05, F06. Root `package.json` setup, `electron/main.js` with `DaemonManager` sidecar supervisor, `electron/preload.js`, single instance lock, clean SIGTERM child teardown, loading `http://127.0.0.1:8765`. | M1 | DONE (Certified: Electron shell created, single-instance lock verified, DaemonManager sidecar lifecycle & graceful SIGTERM teardown verified) |
| M3 | Native System Tray & System Startup Integration | Features F07, F08, F09, F10, F11. Minimize-to-tray on close, native tray icon & context menu with account switcher, `app.setLoginItemSettings()` IPC bridge, and System Settings UI toggle. | M2 | DONE (Certified: System Tray menu, close-to-tray intercept, startup settings IPC and SystemSettingsPage live toggle verified) |
| M4 | Cross-Platform Packaging & Automated Verification Harness | Features F12, F13, F14. `electron-builder` packaging configuration for Linux/Windows/macOS bundling `bin/swiss` sidecar, packaging scripts, and `scripts/verify-desktop-e2e.js` test runner. | M3 | DONE (Certified: electron-builder linux-unpacked packaging verified bundling bin/swiss, scripts/verify-desktop-e2e.js test harness passed 100%) |
| M5 | Final Milestone: Full System Integration & E2E Validation | Features F15, F16. Phase 1: 100% pass of automated desktop verification and Go tests. Phase 2: Adversarial coverage hardening with Challengers and Forensic Auditor. | M1, M2, M3, M4 | DONE (Certified: 100% pass on desktop E2E under XVFB, 16/16 Go packages, 71/71 Python unit tests, 12/12 frontend TypeScript tests, zero orphaned processes, zero Python GUI) |

---

## Interface Contracts

### 1. Electron ↔ Go Daemon Sidecar Lifecycle
```typescript
interface DaemonStatus {
  daemon_running: boolean;
  daemon_pid?: number;
  version?: string;
  active_account?: string;
  total_accounts?: number;
}

class DaemonManager {
  resolveBinaryPath(): string;
  checkStatus(timeoutMs?: number): Promise<DaemonStatus | null>;
  start(): Promise<void>;
  stop(): Promise<void>;
}
```
- **Spawn command**: `<binaryPath> daemon --web --addr 127.0.0.1:8765`
- **Liveness probe**: `GET http://127.0.0.1:8765/api/status`
- **Shutdown signal**: `child.kill('SIGTERM')` with 3-second timeout before `SIGKILL`.

### 2. Electron Preload ↔ Renderer (Context Bridge)
```typescript
interface ElectronAPI {
  isElectron: boolean;
  getStartupSetting: () => Promise<{ openAtLogin: boolean; openAsHidden: boolean; platform: string }>;
  setStartupSetting: (enabled: boolean) => Promise<{ success: boolean; openAtLogin: boolean }>;
  onNavigate: (callback: (toolIndex: number) => void) => void;
}
```
- **Channel**: `desktop:get-startup-setting` (invoke)
- **Channel**: `desktop:set-startup-setting` (invoke)
- **Channel**: `desktop:navigate` (send)

### 3. System Tray Context Menu Contract
- Header: `"Antigravity Swiss Knife"` (disabled)
- Item: `"Open Dashboard"` -> `mainWindow.show()`, `mainWindow.focus()`
- Item: `"Active Account: <email>"` (disabled)
- Submenu: `"Quick Account Switch"` -> Populated from `GET /api/accounts`, clicking sends `POST /api/switch` with `{ email }`
- Item: `"System Settings"` -> `mainWindow.show()`, navigate to Settings
- Separator
- Item: `"Quit Antigravity Swiss Knife"` -> `isQuitting = true`, `daemonManager.stop()`, `app.quit()`

---

## Code Layout

```
/mnt/Data/Projects/Antigravity Swiss Knife/
├── package.json                 # Root package.json (electron, electron-builder, scripts)
├── electron/
│   ├── main.js                  # Electron main process (DaemonManager, Tray, window lifecycle, IPC)
│   └── preload.js               # Context bridge exposing electronAPI
├── assets/
│   └── logo.png                 # Application and tray icon asset
├── scripts/
│   └── verify-desktop-e2e.js    # Automated headless XVFB test runner
├── frontend/                    # TypeScript + React 19 single-page application
│   ├── package.json
│   ├── vite.config.ts
│   └── src/
│       ├── api.ts
│       ├── App.tsx
│       └── pages/SystemSettingsPage.tsx  # Startup setting toggle card
├── bin/
│   └── swiss                    # Statically linked Go binary
├── pkg/
│   └── webgui/
│       └── dist/                # Compiled frontend embedded in Go binary
├── cmd/
│   └── swiss/                   # Go CLI and daemon entry point
└── antigravity_swiss/           # Python library (legacy gui/ completely deleted)
```
