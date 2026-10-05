# Handoff Report: Frontend, Packaging & Desktop Architecture Survey

**Agent**: `explorer_survey_1` (teamwork_preview_explorer)  
**Date**: 2026-10-05T10:25:00Z  
**Target Repository**: `/mnt/Data/Projects/Antigravity Swiss Knife`  

---

## 1. Observation

### 1.1 Repository Root & Packaging Landscape
- **Root Directory Structure** (`list_dir /mnt/Data/Projects/Antigravity Swiss Knife`):
  - Contains directories: `antigravity_swiss/`, `assets/`, `bin/`, `cmd/`, `frontend/`, `pkg/`, `tests/`.
  - There is currently **no root `package.json`**. Only `frontend/` contains a `package.json` and `bun.lock`.
  - `assets/` contains `assets/logo.png` (574,104 bytes).
  - `bin/` contains the compiled Go binary `bin/swiss` (12,685,268 bytes).
  - `.gitignore` lines 1-14:
    ```
    # Dependencies
    node_modules/
    .pnpm-store/
    frontend/dist/
    build/
    target/

    # Binaries and scratch scripts
    bin/
    scratch/

    # Embed web assets exception
    !pkg/webgui/dist/
    ```

### 1.2 Frontend Tooling, Dependencies & Build Output
- `frontend/package.json`:
  - Name: `"frontend"`, Version: `"0.0.0"`, Type: `"module"`.
  - Dependencies:
    - `"react": "^19.2.8"`
    - `"react-dom": "^19.2.8"`
    - `"lucide-react": "^1.51.0"`
  - DevDependencies:
    - `"vite": "^8.3.0"`
    - `"@vitejs/plugin-react": "^6.1.1"`
    - `"typescript": "~6.0.2"`
    - `"oxlint": "^1.81.0"`
    - `"@types/node": "^24.13.3"`, `"@types/react": "^19.2.18"`, `"@types/react-dom": "^19.2.7"`
  - Scripts:
    - `"dev": "vite"`
    - `"build": "tsc -b && vite build"`
    - `"test": "node --test src/**/*.test.ts"`
    - `"lint": "oxlint"`
    - `"preview": "vite preview"`
- `frontend/vite.config.ts`:
  ```ts
  import react from '@vitejs/plugin-react'
  import { defineConfig } from 'vite'

  export default defineConfig({
    plugins: [react()],
    base: './',
    build: {
      outDir: '../pkg/webgui/dist',
      emptyOutDir: true,
    },
    server: {
      proxy: {
        '/api': 'http://127.0.0.1:8765',
      },
    },
  })
  ```
- **Observed Build Command**: `npm run build` in `frontend/` runs in ~986ms and outputs:
  - `../pkg/webgui/dist/index.html` (510 bytes)
  - `../pkg/webgui/dist/assets/index-AEL7Q-g5.css` (3.18 kB)
  - `../pkg/webgui/dist/assets/index-D9ufoiHe.js` (409.78 kB)
  - All script and link tags in `index.html` use relative paths:
    `<script type="module" crossorigin src="./assets/index-D9ufoiHe.js"></script>`
- **Observed Test Command**: `npm test` in `frontend/` executes 12 unit tests (schedule utility and TOTP utility) and all 12 pass.

### 1.3 Frontend-to-Backend Communication
- `frontend/src/api.ts`:
  - Every API request is issued through a centralized helper `request<T>(url, options)`:
    ```ts
    async function request<T>(url: string, options?: RequestInit): Promise<T> {
      const res = await fetch(url, options)
      if (!res.ok) {
        const text = await res.text()
        throw new Error(text || `Request failed with status ${res.status}`)
      }
      return res.json() as Promise<T>
    }
    ```
  - Searches for `websocket`, `EventSource`, or alternative `fetch(` calls in `frontend/src/` show **zero** other network calls.
  - All URLs are relative paths starting with `/api/` (e.g. `/api/status`, `/api/quota/fleet`, `/api/accounts`, `/api/switch`, `/api/totp`, `/api/fingerprint`, `/api/cache/scan`, `/api/rules`, `/api/custom_models`, `/api/enhancements`, `/api/templates`, `/api/settings/password`).
- `pkg/webgui/server.go`:
  - Go server embeds the compiled frontend dist:
    ```go
    //go:embed all:dist
    var distFS embed.FS
    ```
  - Default listening address: `127.0.0.1:8765` (`NewServer(addr string, socketPath string)` lines 48-51).
  - Routes root `/` and `/index.html` to embedded `dist/index.html` and routes `/assets/` to `http.FileServer(http.FS(distSub))`.
  - Exposes all `/api/...` endpoints directly on the same port (`127.0.0.1:8765`).

### 1.4 Go Backend Binary & Lifecycle
- `cmd/swiss/main.go`:
  - Command `swiss daemon --web`:
    - Spawns Unix domain socket daemon at `$XDG_RUNTIME_DIR/antigravity-swiss/daemon.sock` (or `/run/user/1000/antigravity-swiss/daemon.sock`).
    - Spawns Web GUI HTTP server on `127.0.0.1:8765` (lines 198-205).
    - Traps `os.Interrupt` and `syscall.SIGTERM` (lines 208-210), gracefully shutting down both the web server and the IPC daemon:
      ```go
      sigCh := make(chan os.Signal, 1)
      signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
      <-sigCh
      fmt.Println("\nShutting down daemon...")
      if webSrv != nil { _ = webSrv.Stop() }
      _ = d.Stop()
      fmt.Println("Daemon gracefully stopped.")
      ```
  - `pkg/ipc/server.go` line 79: `_ = os.Remove(s.socketPath)` removes the Unix socket file upon clean exit.
  - Build command: `go build -o bin/swiss ./cmd/swiss`.
  - Go test execution: `go test ./pkg/... ./cmd/...` executes 16 package test suites and all 16 pass.

### 1.5 Legacy PySide6 Desktop Implementation
- `antigravity_swiss/gui/` contains:
  - `main_window.py`: Previously created a `QMainWindow` and loaded `http://127.0.0.1:8765` in `QWebEngineView`. Spawned `bin/swiss daemon --web` via `subprocess.Popen` in `_ensure_daemon_running()`. Intercepted `closeEvent` (lines 254-260) to call `self.hide()` and minimize to system tray.
  - `tray.py`: Implemented `QSystemTrayIcon` with DBus SNI, context menu ("Open Dashboard", "System Settings", "Refresh Quota Now", "Switch Account" submenu, "Exit Swiss Knife"), and click-to-restore.
  - `app.py`, `dialogs/`, `pages/`, `widgets/`, `styles.py`.
- `antigravity_swiss/__main__.py`: Lines 590-593 wire the CLI subcommand `gui` (`python -m antigravity_swiss gui`), which calls `run_gui` (lines 321-340).

### 1.6 Frontend Feature Pages Inventory
1. **Account Switcher** (`App.tsx` tool index 0):
   - Top Ribbon with 5 sub-views:
     - `QuotaDashboardPage.tsx`: Fleet overview, 4 circular gauges, healthy standby accounts sorting, 1-click manual account switch, auto-switch threshold toggle.
     - `MfaVaultPage.tsx`: RFC 6238 TOTP codes, live 1-second countdown, base32 secret saving.
     - `FingerprintsPage.tsx`: Hardware telemetry & UUID profile inspector.
     - `BrainCachePage.tsx`: Categorized disk usage chart, safe cleanup actions.
     - `SwitcherSettingsPage.tsx`: Rule configuration, threshold sliders.
2. **Tools Marketplace** (`ToolsMarketplacePage.tsx`, tool index 1): Catalog of installed and available tools with routing triggers.
3. **System Settings** (`SystemSettingsPage.tsx`, tool index 2): Runtime paths, Antigravity 2.0 & VS Code extension installation diagnostics, update checks, process safety shield status, app password lock.
4. **Custom Models** (`CustomModelsPage.tsx`, tool index 3): BYOM provider configurations (OpenAI, Anthropic, Gemini, Ollama, vLLM), live test button, thinking level controls.
5. **App Enhancements** (`AppEnhancementsPage.tsx`, tool index 4): Prompt Jump Bar, Tool Density, Breaker Line, 10x10 color palette picker.
6. **Task Automations** (`ScheduledTemplatesPage.tsx`, tool index 5): Scheduled template catalog, cron deployment modal, running sidecars manager.
7. **Archived Projects** (`ArchivedProjectsPage.tsx`, tool index 6): Project archiving, ordering, restore, and auto-archive horizons.

### 1.7 Environment Diagnostics
- Node: `v24.16.0`
- npm: `11.13.0`
- bun: `1.3.14`
- `xvfb-run`: Available at `/usr/bin/xvfb-run`
- `pgrep`: Available at `/usr/bin/pgrep`

---

## 2. Logic Chain

1. **Frontend Serving & Routing (Observation §1.2 & §1.3)**:
   - `frontend/vite.config.ts` builds into `pkg/webgui/dist` with `base: './'`.
   - `pkg/webgui/server.go` embeds this exact directory (`//go:embed all:dist`) and serves it at `http://127.0.0.1:8765/`.
   - Since all API requests in `frontend/src/api.ts` use relative URLs (`/api/...`), loading `http://127.0.0.1:8765` directly in Electron's `BrowserWindow` guarantees:
     - Zero CORS errors.
     - Zero proxy configuration issues.
     - 100% parity between standalone web browser mode and desktop Electron mode.
     - Complete access to all 25+ REST endpoints without needing an IPC bridge for everyday data operations.

2. **Electron Sidecar Supervision (Observation §1.3 & §1.4)**:
   - The Go daemon (`bin/swiss daemon --web`) serves both the REST API and the frontend assets on `127.0.0.1:8765`.
   - The Go binary handles `SIGTERM` gracefully, cleanly terminating its HTTP server and deleting its Unix domain socket file (`pkg/ipc/server.go:79`).
   - Therefore, the Electron main process should supervise the Go binary as a child process:
     - Check if `http://127.0.0.1:8765/api/status` is already healthy upon Electron launch.
     - If not, spawn `bin/swiss daemon --web`.
     - Poll until HTTP 200 is returned, then load the URL in `BrowserWindow`.
     - On Electron app exit (`before-quit` / `will-quit` / `SIGINT`), send `SIGTERM` to the Go daemon child process, await termination, and ensure no orphaned processes remain (`pgrep swiss` == 0).

3. **System Tray & Window Lifecycle (Observation §1.5 & Requirement R3)**:
   - PySide6 legacy GUI implemented minimize-to-tray on close and a dynamic tray context menu.
   - Electron's `BrowserWindow` close event can be intercepted:
     ```javascript
     mainWindow.on('close', (event) => {
       if (!isQuitting) {
         event.preventDefault();
         mainWindow.hide();
       }
     });
     ```
   - Electron's `Tray` API natively supports:
     - Click/double-click to restore and focus `mainWindow`.
     - Context menu with:
       - Header: "Antigravity Swiss Knife"
       - "Open Dashboard" -> `mainWindow.show()`, `mainWindow.focus()`
       - "Active Account: <email>"
       - "Switch Account" -> Submenu with registered accounts
       - "System Settings" -> `mainWindow.show()`, sends IPC event to navigate to Settings
       - "Quit" -> sets `isQuitting = true`, graceful daemon termination, `app.quit()`

4. **System Settings Startup Integration (Observation §1.6 & Requirement R4)**:
   - Electron provides native cross-platform auto-start via `app.getLoginItemSettings()` and `app.setLoginItemSettings()`.
   - Preload script can expose:
     ```javascript
     contextBridge.exposeInMainWorld('electronAPI', {
       isElectron: true,
       getLoginItemSettings: () => ipcRenderer.invoke('get-login-item-settings'),
       setLoginItemSettings: (opts) => ipcRenderer.invoke('set-login-item-settings', opts),
       onNavigate: (cb) => ipcRenderer.on('navigate-tool', (evt, idx) => cb(idx)),
     });
     ```
   - In `SystemSettingsPage.tsx`, an auto-start card with a toggle "Launch at System Startup (Minimized to Tray)" can be added, updating and reflecting `electronAPI.getLoginItemSettings()` / `setLoginItemSettings()`.

5. **Packaging Architecture (Observation §1.1 & §1.7)**:
   - A root `package.json` should be established to coordinate Electron, frontend builds, Go builds, and packaging.
   - Using `electron-builder`:
     - Bundle `electron/` main and preload scripts.
     - Bundle `pkg/webgui/dist/` (frontend build).
     - Bundle `bin/swiss` via `extraResources` (`{"from": "bin", "to": "bin", "filter": ["swiss*"]}`).
     - Targets: Linux (`AppImage`, `deb`), Windows (`nsis`, `zip`), macOS (`dmg`, `zip`).

6. **Python Retirement (Observation §1.5 & Requirement R1)**:
   - `antigravity_swiss/gui/` can be deleted in its entirety (8 files, 4 subdirectories).
   - In `antigravity_swiss/__main__.py`, remove the `gui` subcommand (`run_gui`).
   - This eliminates all Python desktop GUI dependencies (PySide6) with zero impact on CLI or Go daemon services.

---

## 3. Caveats

1. **Single Instance Locking**: If a developer launches multiple instances of `npm run desktop`, Electron must use `app.requestSingleInstanceLock()`. If the lock is not acquired, the second instance must focus the first window and terminate immediately.
2. **Pre-existing Daemon**: If a user already has `swiss daemon --web` running externally (e.g. systemd or background CLI), Electron should detect the running daemon via `http://127.0.0.1:8765/api/status` and NOT terminate it on exit if it did not spawn it. Electron should only terminate the daemon if Electron spawned it as a child process.
3. **Headless Verification Environment**: In CI or headless Linux environments, Electron requires a virtual frame buffer. The environment has `/usr/bin/xvfb-run` installed, so `xvfb-run -a electron scripts/verify-desktop.js` can run without physical display servers.
4. **Relative Asset Base**: If Electron were ever configured to load static files via `file://` instead of `http://127.0.0.1:8765`, `fetch('/api/...')` would fail unless an absolute base URL (`http://127.0.0.1:8765`) is specified in `frontend/src/api.ts`. Loading `http://127.0.0.1:8765` directly in Electron avoids this issue entirely.

---

## 4. Conclusion & Concrete Architecture Recommendations

### Recommended Directory Structure
```
/mnt/Data/Projects/Antigravity Swiss Knife/
├── package.json                 # Project root package.json (scripts, electron, electron-builder)
├── electron/
│   ├── main.js                  # Electron main process (lifecycle, daemon supervisor, tray, IPC)
│   └── preload.js               # Context bridge (login item settings, navigation, window controls)
├── scripts/
│   └── verify-desktop.js        # Automated verification harness for headless Electron + Go test
├── assets/
│   └── logo.png                 # Window & tray icon source
├── frontend/                    # React 19 + TypeScript + Vite app
│   ├── package.json
│   ├── vite.config.ts
│   └── src/
│       ├── api.ts
│       ├── App.tsx
│       └── pages/SystemSettingsPage.tsx  # Add "Launch at System Startup" toggle card
├── bin/
│   └── swiss                    # Go binary (bundled as sidecar)
├── pkg/
│   └── webgui/
│       └── dist/                # Compiled React 19 single-page app (embedded in Go binary)
└── antigravity_swiss/
    └── gui/                     # RETIRED / DELETED
```

### Required Dependencies (Root `package.json`)
```json
{
  "name": "antigravity-swiss-knife",
  "version": "2.0.0",
  "description": "Antigravity Swiss Knife Standalone Electron Desktop Application",
  "main": "electron/main.js",
  "scripts": {
    "build:frontend": "npm run build --prefix frontend",
    "build:go": "go build -o bin/swiss ./cmd/swiss",
    "build": "npm run build:frontend && npm run build:go",
    "desktop": "electron .",
    "desktop:dev": "npm run build:frontend && electron .",
    "test:desktop": "xvfb-run -a electron scripts/verify-desktop.js",
    "pack": "npm run build && electron-builder --dir",
    "dist": "npm run build && electron-builder",
    "dist:linux": "npm run build && electron-builder --linux",
    "dist:win": "npm run build && electron-builder --win",
    "dist:mac": "npm run build && electron-builder --mac"
  },
  "devDependencies": {
    "electron": "^34.2.0",
    "electron-builder": "^25.1.8",
    "cross-env": "^7.0.3",
    "wait-on": "^8.0.2"
  },
  "build": {
    "appId": "com.chillingwombat.antigravity-swiss-knife",
    "productName": "Antigravity Swiss Knife",
    "directories": {
      "output": "dist-electron"
    },
    "files": [
      "electron/**/*",
      "pkg/webgui/dist/**/*",
      "assets/**/*"
    ],
    "extraResources": [
      {
        "from": "bin",
        "to": "bin",
        "filter": ["swiss*"]
      }
    ],
    "linux": {
      "target": ["AppImage", "deb"],
      "category": "Utility",
      "icon": "assets/logo.png"
    },
    "win": {
      "target": ["nsis", "zip"],
      "icon": "assets/logo.png"
    },
    "mac": {
      "target": ["dmg", "zip"],
      "icon": "assets/logo.png",
      "category": "public.app-category.developer-tools"
    }
  }
}
```

### Detailed Feature Compatibility Matrix
| Feature Page | File Path | REST API Endpoints | Electron Parity Status |
|---|---|---|---|
| **Account Switcher** (Quota Gauges) | `frontend/src/pages/QuotaDashboardPage.tsx` | `/api/quota/fleet`, `/api/switch`, `/api/accounts/*` | **100% Compatible** |
| **MFA & RFC 6238 TOTP** | `frontend/src/pages/MfaVaultPage.tsx` | `/api/totp`, `/api/accounts` | **100% Compatible** |
| **Device Fingerprints** | `frontend/src/pages/FingerprintsPage.tsx` | `/api/fingerprint` | **100% Compatible** |
| **Brain Cache Optimizer** | `frontend/src/pages/BrainCachePage.tsx` | `/api/cache/scan`, `/api/cache/prune` | **100% Compatible** |
| **Switcher Settings** | `frontend/src/pages/SwitcherSettingsPage.tsx` | `/api/rules`, `/api/rules/auto_switch` | **100% Compatible** |
| **Tools Marketplace** | `frontend/src/pages/ToolsMarketplacePage.tsx` | In-memory routing | **100% Compatible** |
| **Custom Models (BYOM)** | `frontend/src/pages/CustomModelsPage.tsx` | `/api/custom_models/*` | **100% Compatible** |
| **App Enhancements** | `frontend/src/pages/AppEnhancementsPage.tsx` | `/api/enhancements/*`, `/api/gui/*` | **100% Compatible** |
| **Task Automations** | `frontend/src/pages/ScheduledTemplatesPage.tsx` | `/api/templates/*` | **100% Compatible** |
| **Archived Projects** | `frontend/src/pages/ArchivedProjectsPage.tsx` | `/api/gui/projects/*` | **100% Compatible** |
| **System Settings** | `frontend/src/pages/SystemSettingsPage.tsx` | `/api/system/*`, `/api/auth/*` + Electron Startup IPC | **100% Compatible** (add auto-start toggle) |

---

## 5. Verification Method

To verify these findings and validate the proposed architecture:

1. **Verify Frontend Build & Tests**:
   ```bash
   cd "/mnt/Data/Projects/Antigravity Swiss Knife/frontend"
   npm test
   npm run build
   ```
   *Expected*: 12 unit tests pass; `pkg/webgui/dist/` is cleanly populated with `index.html` and bundled assets.

2. **Verify Go Daemon & WebGUI Server**:
   ```bash
   cd "/mnt/Data/Projects/Antigravity Swiss Knife"
   go test ./pkg/... ./cmd/...
   go build -o bin/swiss ./cmd/swiss
   ./bin/swiss daemon --web &
   SWISS_PID=$!
   sleep 1
   curl -s http://127.0.0.1:8765/api/status | grep "daemon_running"
   kill -TERM $SWISS_PID
   wait $SWISS_PID
   pgrep swiss
   ```
   *Expected*: All 16 Go test packages pass. Status endpoint returns HTTP 200 with JSON payload. SIGTERM cleanly terminates the process, and `pgrep swiss` returns 0 orphaned processes.

3. **Verify Electron Lifecycle & Cleanup (Upon Implementation)**:
   ```bash
   xvfb-run -a electron scripts/verify-desktop.js
   ```
   *Expected*: Electron creates main window titled "Antigravity Swiss Knife", receives HTTP 200 from `http://127.0.0.1:8765/api/status`, exits cleanly, and `pgrep swiss` returns 0 processes.

4. **Invalidation Conditions**:
   - If `bin/swiss daemon --web` fails to serve `pkg/webgui/dist/index.html` on `http://127.0.0.1:8765/`.
   - If any frontend feature page requires native browser extension APIs or webview features unsupported by Electron.
   - If `pgrep swiss` shows lingering daemon processes after Electron `app.quit()`.
