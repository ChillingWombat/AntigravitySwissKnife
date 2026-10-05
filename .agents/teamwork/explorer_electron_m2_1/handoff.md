# Milestone 2 Exploration Report: Root Packaging, Dependencies, Scripts, and Electron Shell Architecture

**Agent**: `explorer_electron_m2_1` (teamwork_preview_explorer)  
**Date**: 2026-10-05T11:18:00Z  
**Target Repository**: `/mnt/Data/Projects/Antigravity Swiss Knife`  
**Working Directory**: `/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_electron_m2_1`  

---

## 1. Observation

### 1.1 Existing Root Packaging State
- **Root `package.json`** (`/mnt/Data/Projects/Antigravity Swiss Knife/package.json` lines 1-70):
  - `name`: `"antigravity-swiss-knife"`, `version`: `"0.2.0"`, `main`: `"electron/main.js"`.
  - Current scripts (lines 6-15):
    - `"frontend:build": "cd frontend && npm run build"`
    - `"go:build": "go build -o bin/swiss ./cmd/swiss"`
    - `"build": "npm run frontend:build && npm run go:build"`
    - `"desktop": "electron electron/main.js"`
    - `"desktop:minimized": "electron electron/main.js --minimized"`
    - `"pack": "electron-builder --dir"`
    - `"dist": "electron-builder"`
    - `"test:desktop": "node scripts/verify-desktop-e2e.js"`
  - Current devDependencies (lines 65-68):
    - `"electron": "^35.0.0"`
    - `"electron-builder": "^25.1.8"`
  - Missing in devDependencies: `cross-env`, `wait-on`.
  - Script naming discrepancies vs M2 specification:
    - `"frontend:build"` vs requested `"build:frontend": "npm run build --prefix frontend"`.
    - `"go:build"` vs requested `"build:go": "go build -o bin/swiss ./cmd/swiss"`.
    - `"desktop": "electron electron/main.js"` vs requested `"desktop": "electron ."`.
    - Missing `"desktop:dev": "npm run build:frontend && electron ."`.

### 1.2 Frontend Isolation & Coexistence
- **Frontend Manifest** (`frontend/package.json` lines 1-35):
  - `"name": "frontend"`, `"type": "module"`.
  - Dependencies: `"react": "^19.2.8"`, `"react-dom": "^19.2.8"`, `"lucide-react": "^1.51.0"`.
  - DevDependencies: `"vite": "^8.3.0"`, `"@vitejs/plugin-react": "^6.1.1"`, `"typescript": "~6.0.2"`, `"oxlint": "^1.81.0"`.
  - Script `"build": "tsc -b && vite build"`.
- **Target Output** (`frontend/vite.config.ts` line 62):
  - `outDir: '../pkg/webgui/dist'`, `base: './'`.
- **Execution Test**:
  - Running `npm run build --prefix frontend` from the repository root completes with exit code 0 in 463ms, generating `pkg/webgui/dist/index.html` (510 B) and asset chunks.
  - Running `go build -o bin/swiss ./cmd/swiss` from repository root completes with exit code 0 in ~200ms.
- **Git Tracking** (`.gitignore` lines 1-14):
  - `node_modules/` is gitignored.
  - `!pkg/webgui/dist/` is explicitly unignored so that compiled web assets are embedded into the Go binary (`//go:embed all:dist` in `pkg/webgui/server.go`).

### 1.3 Electron Main Process (`electron/main.js`)
- Located at `/mnt/Data/Projects/Antigravity Swiss Knife/electron/main.js` (472 lines, 13,088 bytes).
- Key Architectural Components observed:
  - **Constants** (lines 7-10): `DAEMON_PORT = 8765`, `DAEMON_HOST = '127.0.0.1'`, `DAEMON_URL = 'http://127.0.0.1:8765'`.
  - **Liveness Probe** (lines 18-42): `probeDaemonStatus(timeoutMs = 1500)` makes HTTP GET to `/api/status`.
  - **Daemon Manager** (lines 44-150):
    - `resolveBinaryPath()`: Resolves binary from packaged `process.resourcesPath/bin/swiss` or local `../bin/swiss`.
    - `ensureRunning()`: First checks if Go daemon is already alive on port 8765. If not running, spawns `['daemon', '--web', '--addr', '127.0.0.1:8765']` and polls `/api/status` for up to 35 attempts (7s).
    - `stop()`: If child process was spawned by Electron, sends `SIGTERM`, waits up to 3000ms, and escalates to `SIGKILL` if necessary. If daemon was pre-existing (external), it is not touched.
  - **Single Instance Lock** (lines 152-165): Enforced via `app.requestSingleInstanceLock()`. Quits secondary instance and focuses existing window.
  - **System Tray** (lines 175-309): Dynamic tray icon (`assets/logo.png`), status polling, account switcher submenu (`POST /api/switch`), settings navigation (`desktop:navigate`, 2), and full quit handler.
  - **Window Lifecycle** (lines 311-367): Intercepts `close` event to hide to tray (`event.preventDefault(); mainWindow.hide();`) unless `isQuitting = true`.
  - **IPC Handlers** (lines 369-416): Handlers for `desktop:get-startup-setting` (`app.getLoginItemSettings()`), `desktop:set-startup-setting` (`app.setLoginItemSettings()`), `desktop:notify`, and `desktop:open-external`.
  - **Exports**: Currently lacks `module.exports` for programmatic testing.

### 1.4 Electron Preload Script (`electron/preload.js`)
- Located at `/mnt/Data/Projects/Antigravity Swiss Knife/electron/preload.js` (17 lines, 661 bytes).
- Context bridge exposes `window.electronAPI`:
  - `isElectron: true`
  - `getStartupSetting: () => ipcRenderer.invoke('desktop:get-startup-setting')`
  - `setStartupSetting: (enabled) => ipcRenderer.invoke('desktop:set-startup-setting', enabled)`
  - `onNavigate: (callback) => ipcRenderer.on('desktop:navigate', ...)`
  - `sendNotification: (title, body) => ipcRenderer.invoke('desktop:notify', { title, body })`
  - `openExternal: (url) => ipcRenderer.invoke('desktop:open-external', url)`
- Verified against frontend consumption:
  - `frontend/src/App.tsx:61-63`: Listens to `electronAPI.onNavigate`.
  - `frontend/src/pages/SystemSettingsPage.tsx:49-70`: Calls `electronAPI.getStartupSetting()` and `electronAPI.setStartupSetting(enabled)`.

### 1.5 Verification Harness Execution
- Executing `node scripts/verify-desktop-e2e.js` under headless XVFB in test environment:
  ```
  [E2E-TEST] Window title: Antigravity Swiss Knife
  [E2E-TEST] API status result: OK
  [E2E-TEST] LoginItemSettings: {"openAtLogin":false,"openAsHidden":false,"restoreState":false,"wasOpenedAtLogin":false,"wasOpenedAsHidden":false}
  [E2E-TEST] All E2E desktop assertions passed! Initiating graceful shutdown...
  ----------------------------------------------------------------------
  [Result] Electron exited with code=0, signal=null
    ✓ Main Window Created & Title Verified
    ✓ API Status Probe Succeeded (127.0.0.1:8765/api/status)
    ✓ Startup IPC Handlers Verified (getLoginItemSettings)
    ✓ Clean Exit Code 0
  [Test] Verifying process cleanup...
    ✓ No orphaned swiss child daemon processes remain.
  ======================================================================
  ALL E2E DESKTOP VERIFICATION CHECKS PASSED (100%)
  ```

---

## 2. Logic Chain

1. **Independent Package Ecosystems vs Monorepo (Observation §1.1 & §1.2)**:
   - Root `package.json` operates as CommonJS (`electron/main.js` uses `require(...)`).
   - `frontend/package.json` operates as ESM (`"type": "module"` with Vite and React 19).
   - If root and `frontend/` are merged into an npm workspaces monorepo with hoisting, ESM/CJS conflicts occur, and toolings like Vite and Electron clash over module loaders.
   - Using `--prefix frontend` keeps dependencies strictly segregated:
     - `node_modules/` in root contains desktop tools (`electron`, `electron-builder`, `cross-env`, `wait-on`).
     - `frontend/node_modules/` contains frontend compilation tools (`vite`, `react`, `@types/...`).
     - Zero namespace collisions, zero hoisting bugs.

2. **Npm Scripts Formulation (Observation §1.1 & Mission Requirements)**:
   - Replacing `"frontend:build": "cd frontend && npm run build"` with `"build:frontend": "npm run build --prefix frontend"`:
     - Avoids shell-dependent `cd` commands (which can fail on different OS shells).
     - Standardizes script naming using the conventional `build:<target>` hierarchy.
   - Adding `"build:go": "go build -o bin/swiss ./cmd/swiss"`.
   - Formulating `"build": "npm run build:frontend && npm run build:go"` ensures full deterministic compilation order: web assets are built first into `pkg/webgui/dist/`, then embedded into `bin/swiss` via Go's `//go:embed all:dist`.
   - In Electron, `electron .` automatically reads `package.json` in the current working directory, extracting `"main": "electron/main.js"`.
   - Formulating `"desktop:dev": "npm run build:frontend && electron ."` ensures frontend updates are recompiled into `pkg/webgui/dist` before launching the desktop window during development.

3. **Required DevDependencies (`cross-env`, `wait-on`) (Observation §1.1)**:
   - `cross-env` (`^7.0.3`): Enables scripts to set environment variables across Windows, macOS, and Linux (e.g. `cross-env TEST_DESKTOP_E2E=1 node scripts/verify-desktop-e2e.js`).
   - `wait-on` (`^8.0.2`): Enables external integration scripts or CI workflows to wait on HTTP status (`wait-on http://127.0.0.1:8765/api/status`) before executing browser or automation tasks.
   - `electron` (`^35.0.0`): Core desktop shell runtime.
   - `electron-builder` (`^25.1.8`): Standalone cross-platform distribution packager.

4. **Electron Shell & Preload Module Exports (Observation §1.3 & §1.4)**:
   - `electron/main.js` is currently structured as an application entry point script without exports.
   - Adding a structured `module.exports` block exporting `DaemonManager`, `probeDaemonStatus`, `createWindow`, `createTray`, `registerIpcHandlers`, and configuration constants allows unit test suites and integration runners to test lifecycle functions in isolation without launching an entire graphical display.
   - In `electron/preload.js`, encapsulating the API object and checking `typeof module !== 'undefined' && module.exports` permits mocking and contract-testing within test suites.

---

## 3. Caveats

1. **Backwards Compatibility for Scripts**: If existing workflows or documentation reference `frontend:build` or `go:build`, removing them could cause friction. Recommendation: keep `frontend:build` and `go:build` as aliases that invoke `build:frontend` and `build:go`.
2. **Read-Only Scope Boundary**: Under Milestone 2 exploration rules, no source files were modified directly. All formulations below are drop-in ready specifications for the implementation phase.
3. **Electron Binary Cache on Linux**: In certain environments, if `node_modules/electron/path.txt` has a trailing newline from legacy shell writes, `child_process.spawn` can throw ENOENT. The canonical `npm install` handles this cleanly.
4. **Gitignore for Packaging Artifacts**: `electron-builder` outputs to `dist-desktop/` (specified in `package.json:20`). Adding `dist-desktop/` to `.gitignore` prevents multi-hundred-megabyte installer bundles from being committed.

---

## 4. Conclusion & Concrete Specifications

### 4.1 Exact Proposed Root `package.json`

```json
{
  "name": "antigravity-swiss-knife",
  "version": "0.2.0",
  "description": "Native standalone desktop application companion and daemon for Google Antigravity 2.0",
  "main": "electron/main.js",
  "scripts": {
    "build:frontend": "npm run build --prefix frontend",
    "build:go": "go build -o bin/swiss ./cmd/swiss",
    "build": "npm run build:frontend && npm run build:go",
    "desktop": "electron .",
    "desktop:dev": "npm run build:frontend && electron .",
    "desktop:minimized": "electron . --minimized",
    "test:desktop": "node scripts/verify-desktop-e2e.js",
    "pack": "npm run build && electron-builder --dir",
    "dist": "npm run build && electron-builder",
    "dist:linux": "npm run build && electron-builder --linux",
    "dist:win": "npm run build && electron-builder --win",
    "dist:mac": "npm run build && electron-builder --mac",
    "frontend:build": "npm run build:frontend",
    "go:build": "npm run build:go"
  },
  "build": {
    "appId": "com.antigravity.swiss-knife",
    "productName": "Antigravity Swiss Knife",
    "directories": {
      "output": "dist-desktop"
    },
    "files": [
      "electron/**/*",
      "assets/**/*",
      "pkg/webgui/dist/**/*"
    ],
    "extraResources": [
      {
        "from": "bin/swiss",
        "to": "bin/swiss"
      }
    ],
    "linux": {
      "target": [
        "AppImage",
        "deb"
      ],
      "category": "Development",
      "icon": "assets/logo.png"
    },
    "win": {
      "target": [
        "nsis",
        "portable"
      ],
      "icon": "assets/logo.png"
    },
    "mac": {
      "target": [
        "dmg",
        "zip"
      ],
      "category": "public.app-category.developer-tools",
      "icon": "assets/logo.png"
    }
  },
  "keywords": [
    "antigravity",
    "electron",
    "gemini",
    "quota-switcher",
    "desktop"
  ],
  "author": "Antigravity Swiss Knife Authors",
  "license": "MIT",
  "devDependencies": {
    "cross-env": "^7.0.3",
    "electron": "^35.0.0",
    "electron-builder": "^25.1.8",
    "wait-on": "^8.0.2"
  }
}
```

### 4.2 Installation and Coexistence Strategy
- **Root installation**: `npm install` in project root installs desktop dev dependencies (`electron`, `electron-builder`, `cross-env`, `wait-on`).
- **Frontend installation**: `npm install --prefix frontend` installs frontend UI dependencies (`react`, `react-dom`, `vite`, `typescript`, etc.).
- **Coexistence Guarantee**:
  - Root directory remains CommonJS for Electron compatibility.
  - `frontend/` directory remains ESM (`"type": "module"`) for Vite compatibility.
  - `--prefix frontend` guarantees full path encapsulation without changing working directory context.

### 4.3 Formulated Structure & Module Exports for `electron/main.js`

```javascript
/**
 * electron/main.js - Structure & Module Exports Architecture
 */

// 1. Core Imports
const { app, BrowserWindow, Tray, Menu, ipcMain, shell, Notification } = require('electron');
const path = require('path');
const http = require('http');
const { spawn } = require('child_process');
const fs = require('fs');

// 2. Constants & Configuration
const DAEMON_PORT = 8765;
const DAEMON_HOST = '127.0.0.1';
const DAEMON_URL = `http://${DAEMON_HOST}:${DAEMON_PORT}`;

// 3. Module State
let mainWindow = null;
let tray = null;
let isQuitting = false;
let spawnedDaemonProcess = null;

// 4. Liveness Probe
function probeDaemonStatus(timeoutMs = 1500) { /* ... */ }

// 5. DaemonManager Class
class DaemonManager {
  static resolveBinaryPath() { /* ... */ }
  static async ensureRunning() { /* ... */ }
  static async stop() { /* ... */ }
  static isSpawned() { return spawnedDaemonProcess !== null; }
  static getProcess() { return spawnedDaemonProcess; }
}

// 6. UI Helpers
function getIconPath() { /* ... */ }
async function updateTrayMenu() { /* ... */ }
function createTray() { /* ... */ }
async function createWindow() { /* ... */ }

// 7. IPC Handlers
function registerIpcHandlers() { /* ... */ }

// 8. Application Lifecycle Initialization (executed when running as entry point)
if (require.main === module || !process.env.ELECTRON_TEST_MODULE) {
  // Enforce single instance lock
  const gotTheLock = app.requestSingleInstanceLock();
  if (!gotTheLock) {
    console.log('[Electron] Another instance is already running. Quitting.');
    app.quit();
  } else {
    app.on('second-instance', () => {
      if (mainWindow) {
        if (mainWindow.isMinimized()) mainWindow.restore();
        if (!mainWindow.isVisible()) mainWindow.show();
        mainWindow.focus();
      }
    });

    app.whenReady().then(async () => {
      registerIpcHandlers();
      await DaemonManager.ensureRunning();
      createTray();
      await createWindow();

      app.on('activate', () => {
        if (BrowserWindow.getAllWindows().length === 0) {
          createWindow();
        } else if (mainWindow) {
          mainWindow.show();
        }
      });
    });

    app.on('before-quit', async (event) => {
      if (!isQuitting) {
        isQuitting = true;
        event.preventDefault();
        await DaemonManager.stop();
        app.quit();
      }
    });

    app.on('window-all-closed', () => {
      // Keep running in tray on Linux/Windows
    });
  }
}

// 9. Module Exports (for testability and headless orchestration)
module.exports = {
  DaemonManager,
  probeDaemonStatus,
  createWindow,
  createTray,
  updateTrayMenu,
  registerIpcHandlers,
  getMainWindow: () => mainWindow,
  getTray: () => tray,
  getSpawnedDaemon: () => spawnedDaemonProcess,
  DAEMON_URL,
  DAEMON_PORT,
  DAEMON_HOST,
};
```

### 4.4 Formulated Structure & Module Exports for `electron/preload.js`

```javascript
/**
 * electron/preload.js - Context Bridge & Export Architecture
 */

const { contextBridge, ipcRenderer } = require('electron');

const electronAPI = {
  isElectron: true,
  getStartupSetting: () => ipcRenderer.invoke('desktop:get-startup-setting'),
  setStartupSetting: (enabled) => ipcRenderer.invoke('desktop:set-startup-setting', Boolean(enabled)),
  onNavigate: (callback) => {
    ipcRenderer.on('desktop:navigate', (_event, toolIndex) => {
      if (typeof callback === 'function') {
        callback(toolIndex);
      }
    });
  },
  sendNotification: (title, body) => ipcRenderer.invoke('desktop:notify', { title, body }),
  openExternal: (url) => ipcRenderer.invoke('desktop:open-external', url),
};

// Safe Context Bridge exposure
if (process.contextIsolated) {
  contextBridge.exposeInMainWorld('electronAPI', electronAPI);
} else {
  window.electronAPI = electronAPI;
}

// Export for test suites and headless verification
if (typeof module !== 'undefined' && module.exports) {
  module.exports = { electronAPI };
}
```

---

## 5. Verification Method

To independently verify all findings and confirm that the packaging and scripts meet every Milestone 2 requirement:

1. **Verify Frontend Build via `--prefix frontend`**:
   ```bash
   npm run build --prefix frontend
   ```
   *Expected Output*: Exit code 0, bundles to `pkg/webgui/dist/`, completes in under 1 second.

2. **Verify Go Daemon Binary Build**:
   ```bash
   go build -o bin/swiss ./cmd/swiss
   ```
   *Expected Output*: Exit code 0, produces statically linked `bin/swiss` containing embedded web assets.

3. **Verify Combined Build Script**:
   ```bash
   npm run build:frontend && npm run build:go
   ```
   *Expected Output*: Complete build chain runs cleanly from root without errors.

4. **Verify Headless Desktop E2E Verification Harness**:
   ```bash
   node scripts/verify-desktop-e2e.js
   ```
   *Expected Output*:
   ```
   ✓ Main Window Created & Title Verified
   ✓ API Status Probe Succeeded (127.0.0.1:8765/api/status)
   ✓ Startup IPC Handlers Verified (getLoginItemSettings)
   ✓ Clean Exit Code 0
   ✓ No orphaned swiss child daemon processes remain.
   ALL E2E DESKTOP VERIFICATION CHECKS PASSED (100%)
   ```

5. **Invalidation Conditions**:
   - If `npm run build --prefix frontend` alters the root `package-lock.json` or mutates root `node_modules/`.
   - If `electron .` fails to locate `electron/main.js`.
   - If `node scripts/verify-desktop-e2e.js` fails any assertion or leaves orphaned Go processes.
