# Handoff Report: Milestone 2 Electron Shell, Window Lifecycle & Preload Bridge

**Agent**: `explorer_electron_m2_3` (Teamwork Explorer)  
**Date**: 2026-10-05T11:20:00Z  
**Target Milestone**: M2 — Standalone Electron Shell & Go Sidecar Lifecycle  
**Working Directory**: `/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_electron_m2_3`  
**Scope**: Read-only exploration and technical specification for Worker implementation  

---

## 1. Observation

### 1.1 Existing Electron Codebase Structure
The repository contains an initial draft of the Electron integration:
- **`package.json`**:
  - Line 5: `"main": "electron/main.js"`
  - Lines 10-14: `"desktop": "electron electron/main.js"`, `"test:desktop": "node scripts/verify-desktop-e2e.js"`
  - Lines 66-67: `"devDependencies": { "electron": "^35.0.0", "electron-builder": "^25.1.8" }`
- **`electron/main.js`**:
  - Total 472 lines implementing daemon supervision, tray setup, window creation, and IPC.
- **`electron/preload.js`**:
  - Total 17 lines exposing `window.electronAPI`.
- **`assets/logo.png`**:
  - 1024x1024 PNG image asset (574,104 bytes) present in the repository root.

### 1.2 Observation: Single-Instance Lock Mechanism (`electron/main.js:152-165`)
The current single-instance lock implementation in `electron/main.js` is:
```javascript
152: // Single instance lock
153: const gotTheLock = app.requestSingleInstanceLock();
154: if (!gotTheLock) {
155:   console.log('[Electron] Another instance is already running. Quitting.');
156:   app.quit();
157: } else {
158:   app.on('second-instance', () => {
159:     if (mainWindow) {
160:       if (mainWindow.isMinimized()) mainWindow.restore();
161:       if (!mainWindow.isVisible()) mainWindow.show();
162:       mainWindow.focus();
163:     }
164:   });
165: }
```
- **Vulnerability Observed**: `app.quit()` is an asynchronous call. Top-level statements below line 165 (specifically `app.whenReady().then(...)` at line 419) continue executing synchronously during the second instance's initial event-loop tick. If `app.whenReady()` triggers before `app.quit()` processes, the second instance attempts to spawn a redundant daemon and open duplicate windows. Adding `process.exit(0)` immediately after `app.quit()` guarantees instant termination.
- **Second Instance Restoration Observed**: Lines 158-164 correctly check `isMinimized()`, `isVisible()`, and call `focus()`. However, on Linux window managers (X11/Wayland), if a window is hidden to tray with `skipTaskbar: true`, restoring it requires explicitly ensuring `mainWindow.show()`, `mainWindow.focus()`, and restoring window state.

### 1.3 Observation: `BrowserWindow` Creation (`electron/main.js:315-330`)
Lines 315-330 in `electron/main.js` instantiate the `BrowserWindow`:
```javascript
315:   mainWindow = new BrowserWindow({
316:     width: 1220,
317:     height: 820,
318:     minWidth: 960,
319:     minHeight: 640,
320:     title: 'Antigravity Swiss Knife',
321:     icon: iconPath,
322:     show: !startMinimized,
323:     backgroundColor: '#0f172a',
324:     webPreferences: {
325:       preload: path.join(__dirname, 'preload.js'),
326:       nodeIntegration: false,
327:       contextIsolation: true,
328:       sandbox: false,
329:     },
330:   });
```
- **Dimension & Appearance Observed**:
  - Current size is `width: 1220, height: 820`. The project target is `1280x800` (minWidth: 960, minHeight: 640).
  - Current `backgroundColor` is `#0f172a` (Tailwind slate-900). The official Google Gemini dark surface token specified in `PROJECT.md` and the survey report is `#131314`. Using `#131314` eliminates any visual flicker during initial frame rendering before React mounts.
  - `title` is correctly set to `'Antigravity Swiss Knife'`.
  - `icon` is resolved via `getIconPath()` to `assets/logo.png`.
- **URL & WebPreferences Observed**:
  - Target URL is `http://127.0.0.1:8765/`.
  - `webPreferences` correctly sets `preload: path.join(__dirname, 'preload.js')`, `contextIsolation: true`, and `nodeIntegration: false`.
  - Navigation guards: Lines 359-362 guard `setWindowOpenHandler` with `shell.openExternal(url)`. A missing guard is `mainWindow.webContents.on('will-navigate')`, which should prevent internal webview navigation away from `http://127.0.0.1:8765`.

### 1.4 Observation: Preload Bridge & Frontend Contract
`electron/preload.js` lines 1-17:
```javascript
1: const { contextBridge, ipcRenderer } = require('electron');
2: 
3: contextBridge.exposeInMainWorld('electronAPI', {
4:   isElectron: true,
5:   getStartupSetting: () => ipcRenderer.invoke('desktop:get-startup-setting'),
6:   setStartupSetting: (enabled) => ipcRenderer.invoke('desktop:set-startup-setting', enabled),
7:   onNavigate: (callback) => {
8:     ipcRenderer.on('desktop:navigate', (_event, toolIndex) => {
9:       if (typeof callback === 'function') {
10:         callback(toolIndex);
11:       }
12:     });
13:   },
14:   sendNotification: (title, body) => ipcRenderer.invoke('desktop:notify', { title, body }),
15:   openExternal: (url) => ipcRenderer.invoke('desktop:open-external', url),
16: });
```
- **Frontend Usage Verified**:
  - `frontend/src/App.tsx:61-68`: Consumes `electronAPI.onNavigate((toolIdx: number) => { setCurrentTool(toolIdx) })` for tray navigation.
  - `frontend/src/pages/SystemSettingsPage.tsx:50-56`: Calls `electronAPI.getStartupSetting()` expecting `{ openAtLogin: boolean }`.
  - `frontend/src/pages/SystemSettingsPage.tsx:64-70`: Calls `electronAPI.setStartupSetting(enabled)`.
- **IPC Signature Discrepancy**:
  - `PROJECT.md:113` specifies `setStartupSetting: (enabled: boolean) => Promise<...>`, while `PROJECT.md:195` noted passing `{ enabled }`.
  - `electron/main.js:385` handles `ipcMain.handle('desktop:set-startup-setting', async (_event, enabled) => ...)`. If `enabled` is passed as an object `{ enabled: true }`, evaluating `Boolean(enabled)` would evaluate truthy even for `{ enabled: false }`. Normalizing `typeof enabled === 'object' ? Boolean(enabled.enabled) : Boolean(enabled)` provides complete resilience.

### 1.5 Observation: Verification Harness Execution
Executing `xvfb-run -a node scripts/verify-desktop-e2e.js`:
- Verified output:
  ```
  [PASS] Go binary verified at: /mnt/Data/Projects/Antigravity Swiss Knife/bin/swiss
  [PASS] Frontend bundle verified at: /mnt/Data/Projects/Antigravity Swiss Knife/pkg/webgui/dist/index.html
  [Test] Spawning Electron main process under XVFB...
  [Electron] [E2E-TEST] Window title: Antigravity Swiss Knife
  [Electron] [E2E-TEST] API status result: OK
  [Electron] [E2E-TEST] Testing IPC getLoginItemSettings...
  [Result] Electron exited with code=0, signal=null
    ✓ Main Window Created & Title Verified
    ✓ API Status Probe Succeeded (127.0.0.1:8765/api/status)
    ✓ Startup IPC Handlers Verified (getLoginItemSettings)
    ✓ Clean Exit Code 0
  [Test] Verifying process cleanup...
    ✓ No orphaned swiss child daemon processes remain.
  ALL E2E DESKTOP VERIFICATION CHECKS PASSED (100%)
  ```

---

## 2. Logic Chain

1. **Single-Instance Enforcement**:
   - Observation: When a user attempts to launch a second instance, `app.requestSingleInstanceLock()` returns `false`.
   - Logic: Calling `app.quit()` alone does not immediately break the current tick of the Node.js event loop, allowing `app.whenReady()` handlers to register or fire.
   - Inference: Appending `process.exit(0)` immediately following `app.quit()` guarantees instantaneous and clean termination without spawning redundant daemon sidecars.
   - Logic: In the primary instance, `app.on('second-instance')` must handle windows in various states: minimized, hidden in tray, or obscured behind other windows.
   - Inference: Sequence `if (mainWindow.isMinimized()) mainWindow.restore()`, `if (!mainWindow.isVisible()) mainWindow.show()`, `mainWindow.focus()` correctly handles all display states.

2. **BrowserWindow Configuration Consistency**:
   - Observation: Google Gemini Material Design 3 surface token is `#131314`. Current `electron/main.js` has `#0f172a`.
   - Logic: If `backgroundColor` differs from the frontend background, a visible color flicker occurs between the native window rendering and the HTML canvas mount.
   - Inference: Setting `backgroundColor: '#131314'` ensures seamless dark surface alignment.
   - Logic: Sizing `width: 1280, height: 800` (minWidth: 960, minHeight: 640) matches standard desktop dashboard proportions and accommodates the 220px navigation rail plus multi-column cards.

3. **Context Bridge Security & Memory Safety**:
   - Observation: In `preload.js`, `onNavigate` binds `ipcRenderer.on('desktop:navigate', ...)`.
   - Logic: Exposing a listener without a deregistration mechanism causes event listener accumulation if the React component re-mounts.
   - Inference: Returning an unbind callback `() => ipcRenderer.removeListener('desktop:navigate', subscription)` or checking single binding prevents listener leaks.
   - Logic: Exposing explicit APIs under `window.electronAPI` while setting `contextIsolation: true` and `nodeIntegration: false` strictly isolates renderer DOM from Node.js runtime primitives.

4. **IPC Parameter Normalization**:
   - Observation: React components may invoke `setStartupSetting(true)` or `setStartupSetting({ enabled: true })`.
   - Logic: In JavaScript, `Boolean({ enabled: false }) === true`.
   - Inference: The main IPC handler must extract `const isEnabled = typeof enabled === 'object' && enabled !== null ? Boolean(enabled.enabled) : Boolean(enabled)` before passing to `app.setLoginItemSettings()`.

---

## 3. Caveats

1. **Background Daemon Presence During Testing**:
   - Observation: An existing `./bin/swiss web` process was running in the background (`PID: 2005109`).
   - Impact: In this condition, `DaemonManager` detects port 8765 as already active and bypasses child process spawning.
   - Recommendation: The test verification suite should explicitly test both paths: (a) port 8765 occupied (reuses existing daemon), and (b) port 8765 unoccupied (spawns `bin/swiss daemon --web` and terminates child cleanly via `SIGTERM`).
2. **Linux Tray DBus Host in Headless / XVFB**:
   - In environments without an active System Tray / StatusNotifierItem host, `new Tray()` may log warnings (`Could not create system tray icon`).
   - Impact: `electron/main.js` already catches tray initialization errors in a `try...catch` block, ensuring window and daemon lifecycle remain completely unaffected.

---

## 4. Conclusion

The architecture for Milestone 2 (Standalone Electron Shell & Go Sidecar Lifecycle) is sound and verified. To achieve 100% compliance with requirements, the Worker should execute the following concrete modifications:

### 4.1 Implementation Specification for Worker

#### A. Single Instance Lock (`electron/main.js`)
```javascript
// Single instance lock
const gotTheLock = app.requestSingleInstanceLock();
if (!gotTheLock) {
  console.log('[Electron] Another instance is already running. Quitting.');
  app.quit();
  process.exit(0);
} else {
  app.on('second-instance', (_event, _commandLine, _workingDirectory) => {
    if (mainWindow) {
      if (mainWindow.isMinimized()) mainWindow.restore();
      if (!mainWindow.isVisible()) mainWindow.show();
      mainWindow.focus();
    }
  });
}
```

#### B. BrowserWindow Creation (`electron/main.js`)
```javascript
async function createWindow() {
  const iconPath = getIconPath();
  const startMinimized = process.argv.includes('--minimized') || 
                         process.argv.includes('--hidden') ||
                         app.getLoginItemSettings().wasOpenedAsHidden;

  mainWindow = new BrowserWindow({
    width: 1280,
    height: 800,
    minWidth: 960,
    minHeight: 640,
    title: 'Antigravity Swiss Knife',
    icon: iconPath,
    show: !startMinimized,
    backgroundColor: '#131314', // Google Gemini dark surface token
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      nodeIntegration: false,
      contextIsolation: true,
      sandbox: false,
    },
  });

  // Intercept window close ('X') to minimize to system tray
  mainWindow.on('close', (event) => {
    if (!isQuitting) {
      event.preventDefault();
      mainWindow.hide();
    }
  });

  // Security: Prevent window from navigating away from local daemon
  mainWindow.webContents.on('will-navigate', (event, url) => {
    if (!url.startsWith(DAEMON_URL)) {
      event.preventDefault();
      shell.openExternal(url);
    }
  });

  // Open external links in default browser
  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    if (url.startsWith('http:') || url.startsWith('https:')) {
      shell.openExternal(url);
    }
    return { action: 'deny' };
  });

  // Load backend web GUI URL with resilient retry
  mainWindow.loadURL(DAEMON_URL).catch(() => {
    console.log('[Window] Waiting for daemon URL and retrying loadURL...');
    setTimeout(() => {
      if (mainWindow && !mainWindow.isDestroyed()) {
        mainWindow.loadURL(DAEMON_URL).catch((err) => {
          console.error('[Window] Failed to load URL:', err.message);
        });
      }
    }, 1000);
  });

  mainWindow.on('closed', () => {
    mainWindow = null;
  });
}
```

#### C. Preload Context Bridge (`electron/preload.js`)
```javascript
const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('electronAPI', {
  isElectron: true,
  getStartupSetting: () => ipcRenderer.invoke('desktop:get-startup-setting'),
  setStartupSetting: (enabled) => ipcRenderer.invoke('desktop:set-startup-setting', enabled),
  onNavigate: (callback) => {
    if (typeof callback === 'function') {
      const subscription = (_event, toolIndex) => callback(toolIndex);
      ipcRenderer.on('desktop:navigate', subscription);
      return () => ipcRenderer.removeListener('desktop:navigate', subscription);
    }
  },
  sendNotification: (title, body) => ipcRenderer.invoke('desktop:notify', { title, body }),
  openExternal: (url) => ipcRenderer.invoke('desktop:open-external', url),
});
```

#### D. IPC Parameter Normalization (`electron/main.js`)
```javascript
ipcMain.handle('desktop:set-startup-setting', async (_event, enabled) => {
  try {
    const isEnabled = typeof enabled === 'object' && enabled !== null 
      ? Boolean(enabled.enabled) 
      : Boolean(enabled);

    app.setLoginItemSettings({
      openAtLogin: isEnabled,
      openAsHidden: true,
      args: isEnabled ? ['--minimized'] : [],
    });
    const updated = app.getLoginItemSettings();
    return { success: true, openAtLogin: updated.openAtLogin };
  } catch (err) {
    console.error('[IPC] setLoginItemSettings error:', err);
    return { success: false, error: err.message };
  }
});
```

---

## 5. Verification Method

To independently verify the implementation, execute the following commands in order:

### 5.1 Verification Commands for the Worker

1. **Verify Binary & Frontend Prereqs**:
   ```bash
   go build -o bin/swiss ./cmd/swiss
   ./bin/swiss version
   cd frontend && npm run build && cd ..
   ```

2. **Verify Single-Instance Rejection**:
   Launch primary instance in background under XVFB:
   ```bash
   xvfb-run -a electron electron/main.js &
   PRIMARY_PID=$!
   sleep 2
   ```
   Launch secondary instance (must exit immediately with code 0):
   ```bash
   electron electron/main.js
   # Expected output: "[Electron] Another instance is already running. Quitting."
   # Expected exit code: 0
   ```
   Clean up primary instance:
   ```bash
   kill -TERM $PRIMARY_PID
   ```

3. **Verify Automated Headless Desktop E2E**:
   ```bash
   xvfb-run -a node scripts/verify-desktop-e2e.js
   ```
   *Expected Output*:
   - Main Window Created & Title Verified (`Antigravity Swiss Knife`)
   - API Status Probe Succeeded (`http://127.0.0.1:8765/api/status`)
   - Startup IPC Handlers Verified (`getLoginItemSettings`)
   - Clean Exit Code 0
   - Process cleanup verification: 0 orphaned `swiss` processes

4. **Verify Process Cleanup**:
   ```bash
   pgrep -a swiss || true
   # Ensure no unexpected orphaned "swiss daemon --web" processes exist
   ```

5. **Verify Full Go Test Suite**:
   ```bash
   go test ./pkg/... ./cmd/...
   # Expected: ok across all 16 packages
   ```
