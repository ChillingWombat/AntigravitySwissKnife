# Handoff Report: Legacy Python Retirement & Electron Native Desktop Integration Survey

**Agent**: `explorer_survey_3` (Teamwork Explorer)  
**Date**: 2026-10-05T10:25:00Z  
**Target Milestone**: Pre-M1 Architecture Survey & Integration Mapping  
**Working Directory**: `/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_survey_3`  

---

## 1. Observation

### 1.1 Objective 1: Enumeration of Legacy Python PySide6 GUI Code & Launch Pathways

A direct filesystem audit was performed across the repository to identify all Python GUI code, CLI launch paths, build files, and test files.

#### 1.1.1 Legacy PySide6 Code Directory (`antigravity_swiss/gui/`)
The entire `antigravity_swiss/gui/` package contains exactly **27 source Python files** (plus associated `__pycache__` artifacts) spanning 4,200+ lines of code:

| File Path | Lines | Description / Responsibilities |
|---|---|---|
| `antigravity_swiss/gui/__init__.py` | 13 | Module exports: `create_app`, `run_app`, `run_gui`, `MainWindow`, `GEMINI_QSS`, `SwissKnifeTray` |
| `antigravity_swiss/gui/app.py` | 61 | `QApplication` lifecycle, Google Gemini theme application, single-instance handling, `run_app()` |
| `antigravity_swiss/gui/main_window.py` | 262 | PySide6 `QMainWindow`, left navigation rail, 5-tab top ribbon, page switcher, and `closeEvent` (lines 254-260) |
| `antigravity_swiss/gui/styles.py` | 338 | `GEMINI_QSS` Qt style sheet tokens (Google Gemini dark palette: `#131314`, `#1e1f20`, `#8ab4f8`) |
| `antigravity_swiss/gui/tray.py` | 241 | `SwissKnifeTray(QSystemTrayIcon)` DBus SNI system tray, health badges, context menu, notifications |
| `antigravity_swiss/gui/dialogs/__init__.py` | 8 | Dialog module exports |
| `antigravity_swiss/gui/dialogs/account_detail_dialog.py` | 133 | Modal dialog for displaying per-account credentials, plan tiers, and refresh tokens |
| `antigravity_swiss/gui/dialogs/app_unlock_dialog.py` | 91 | Password unlock modal dialog when app-level encryption password is set |
| `antigravity_swiss/gui/pages/__init__.py` | 17 | Page module exports |
| `antigravity_swiss/gui/pages/account_switcher_tool.py` | 174 | Page holding top ribbon and account tool navigation |
| `antigravity_swiss/gui/pages/app_enhancements.py` | 148 | Page for IDE prompt engineering, rules injection, and enhancements |
| `antigravity_swiss/gui/pages/archived_projects.py` | 239 | Page for inspecting and restoring auto-archived workspaces |
| `antigravity_swiss/gui/pages/brain_cache.py` | 196 | Visual disk usage breakdown, prompt cache inspector, cache pruning controls |
| `antigravity_swiss/gui/pages/custom_models.py` | 263 | Custom upstream Gemini model endpoints tester and manager |
| `antigravity_swiss/gui/pages/fingerprints.py` | 211 | Virtual hardware identity / telemetry inspector and generator |
| `antigravity_swiss/gui/pages/mfa_vault.py` | 268 | Multi-account inventory, RFC 6238 TOTP engine integration, countdown timer |
| `antigravity_swiss/gui/pages/quota_dashboard.py` | 224 | Circular gauge quota dashboard, reset horizons, 1-click account rotation |
| `antigravity_swiss/gui/pages/scheduled_templates.py` | 215 | Cron/scheduled prompt automation task templates manager |
| `antigravity_swiss/gui/pages/switcher_settings.py` | 195 | Auto-switch threshold percentage sliders, polling interval inputs, keep-alive warmup toggle |
| `antigravity_swiss/gui/pages/system_settings.py` | 395 | System runtime diagnostics, IPC socket paths, app access password configuration |
| `antigravity_swiss/gui/pages/tools_marketplace.py` | 134 | Extensions and tools marketplace directory |
| `antigravity_swiss/gui/widgets/__init__.py` | 15 | Widget module exports |
| `antigravity_swiss/gui/widgets/account_quota_bar.py` | 108 | Horizontal percentage bar widget for account quotas |
| `antigravity_swiss/gui/widgets/circular_gauge.py` | 129 | `QPainter` vector circular gauge widget for model quotas |
| `antigravity_swiss/gui/widgets/countdown_ring.py` | 138 | `QPainter` vector animated 30s countdown ring widget for TOTP |
| `antigravity_swiss/gui/widgets/nav_rail.py` | 162 | Fixed 72px left navigation rail with Google Gemini iconography |
| `antigravity_swiss/gui/widgets/top_ribbon.py` | 114 | 5-tab pill navigation ribbon widget |

#### 1.1.2 Launch Pathways & CLI Subcommands in Python
- **`antigravity_swiss/__main__.py`**:
  - Line 8: `- gui: Launch desktop GUI (PySide6)`
  - Lines 321-340:
    ```python
    def run_gui(args: argparse.Namespace) -> int:
        """Launch Material Design 3 Desktop GUI."""
        try:
            import PySide6  # noqa: F401
        except ImportError:
            print("[ERROR] PySide6 desktop GUI libraries are not installed in this Python environment.", file=sys.stderr)
            ...
            return 1
        try:
            from antigravity_swiss.gui.app import run_app
            return run_app(standalone=args.standalone)
        except ImportError as e:
            ...
    ```
  - Lines 590-594:
    ```python
    p_gui = subparsers.add_parser("gui", help="Launch Material Design 3 Desktop GUI")
    p_gui.add_argument("--standalone", action="store_true", help="Run in standalone mode without daemon")
    p_gui.set_defaults(func=run_gui)
    ```
- **Packaging / Config Files**:
  - A search for `pyproject.toml`, `requirements.txt`, `setup.py`, `pixi.toml` confirmed **no such files exist** in the repository root. The Python code was run via active interpreter / pixi environment directly (`python -m antigravity_swiss ...`).
  - No `.desktop` files or shell launch scripts exist in the repository root.

#### 1.1.3 Python Test Suite Files Coupling to PySide6
- **`tests/unit/test_gui.py`**:
  - 484 lines dedicated exclusively to testing `antigravity_swiss.gui.*` (QApplication, MainWindow, widgets, pages, dialogs, tray) in offscreen mode.
  - Must be deleted completely upon retirement of `antigravity_swiss/gui/`.
- **`tests/conftest.py`**:
  - Lines 144-155: `_patch_qmessagebox(monkeypatch)` fixture patching `PySide6.QtWidgets.QMessageBox`.
  - Lines 157-169: `qapp` session fixture instantiating `PySide6.QtWidgets.QApplication`.
  - These two fixtures must be removed or guarded so that pytest runs without PySide6 installed.
- **`tests/e2e/test_tier1_features.py`**:
  - Lines 51-61: Imports `MainWindow`, `GEMINI_QSS`, `SwissKnifeTray`, and GUI widgets.
  - Lines 920-1450: Features F15 through F24 specifically test the PySide6 implementation (theme tokens, nav rail, ribbon, gauges, TOTP ring, tray).
- **`tests/e2e/test_tier2_boundaries.py`**:
  - Lines 46-55: Imports `GEMINI_QSS`, `SwissKnifeTray`, GUI widgets.
  - Lines 1240-1275: F24 system tray boundary tests (`test_f24_b01` to `test_f24_b05`).
- **`tests/e2e/test_tier3_pairwise.py`**:
  - Lines 31-32: Imports `SwissKnifeTray`, `CircularGauge`.
  - Lines 256, 299, 363, 402: Pairwise tests using `CircularGauge` and `SwissKnifeTray`.

---

### 1.2 Objective 2: Electron System Tray Integration Requirements

#### 1.2.1 Legacy Tray Behavior (`antigravity_swiss/gui/tray.py`)
In the legacy implementation:
- Tray Icon: Dynamic 32x32 vector icon with health status badge (`#81c995` Healthy, `#fdd663` Warning, `#f28b82` Exhausted).
- Context Menu:
  1. Disabled header title: `"Antigravity Swiss Knife"`
  2. Separator
  3. Action: `"Open Dashboard"` -> restores window
  4. Action: `"System Settings"` -> opens settings page
  5. Action: `"Refresh Quota Now"` -> polls upstream quota
  6. Separator
  7. Submenu: `"Switch Account"` -> lists registered accounts, bolding `[Active] <email>`, clicking triggers account switch
  8. Separator
  9. Action: `"Exit Swiss Knife"` -> full exit via `app.quit()`
- Tray Activation: Single click or double click triggers `show_window_requested`.

#### 1.2.2 Electron System Tray Architecture & Assets
In the Electron application:
- **Assets Required**:
  - Base tray icon: 32x32 PNG for Linux, `.ico` for Windows, and 16x16 / 32x32 PNG template icons (`iconTemplate.png` / `iconTemplate@2x.png`) for macOS menu bar.
  - Status badges: Dynamic canvas / `nativeImage` compositing or pre-rendered icon set (`tray-healthy.png`, `tray-warning.png`, `tray-exhausted.png`).
  - Source assets available in repo: `assets/logo.png` (1024x1024 PNG), `frontend/public/favicon.svg`, `frontend/public/icons.svg`.
- **Window 'X' Close Minimization Behavior**:
  ```javascript
  let isQuitting = false;

  mainWindow.on('close', (event) => {
    if (!isQuitting) {
      event.preventDefault();
      mainWindow.hide();
    }
  });

  app.on('before-quit', () => {
    isQuitting = true;
  });
  ```
- **Tray Click / Double-Click Restore**:
  ```javascript
  const restoreWindow = () => {
    if (!mainWindow) return;
    if (mainWindow.isMinimized()) mainWindow.restore();
    if (!mainWindow.isVisible()) mainWindow.show();
    mainWindow.focus();
  };

  tray.on('click', restoreWindow);
  tray.on('double-click', restoreWindow);
  ```
- **Context Menu Actions Contract**:
  - `Show / Restore` -> `restoreWindow()`
  - `Active Account: <email>` (Disabled / bold header item)
  - `Quick Account Switch` (Submenu dynamically populated from Go daemon `/api/accounts` with radio / check marks; clicking dispatches POST `/api/switch`)
  - `System Settings` -> `restoreWindow()` + notifies renderer to navigate to `/settings`
  - Separator
  - `Quit Antigravity Swiss Knife` -> sets `isQuitting = true`, gracefully terminates Go daemon sidecar, calls `app.quit()`

---

### 1.3 Objective 3: System Settings Startup Integration

#### 1.3.1 Electron `loginItemSettings` API
Electron provides `app.getLoginItemSettings(options)` and `app.setLoginItemSettings(settings)`:
- **Linux Behavior**:
  - Writes a `.desktop` file to `~/.config/autostart/<AppName>.desktop`.
  - When `args: ['--minimized']` is provided, the Exec line in `.desktop` contains `--minimized`.
- **Windows Behavior**:
  - Writes a registry entry to `HKEY_CURRENT_USER\Software\Microsoft\Windows\CurrentVersion\Run`.
  - When `args: ['--minimized']` is provided, Windows passes `--minimized` on user login.
- **macOS Behavior**:
  - Uses `openAsHidden: true` to launch hidden into menu bar / dock.

#### 1.3.2 Launch-to-Tray Flag Handling on Boot
In Electron `main.js`:
```javascript
const shouldStartMinimized = process.argv.includes('--minimized') ||
                             process.argv.includes('--hidden') ||
                             app.getLoginItemSettings().wasOpenedAsHidden;

app.whenReady().then(() => {
  createTray();
  createWindow();
  if (shouldStartMinimized) {
    mainWindow.hide();
  } else {
    mainWindow.show();
  }
});
```

#### 1.3.3 IPC Communication Contract
Because the React UI in `frontend/` runs in a sandboxed renderer with `contextIsolation: true`:
- **Preload Bridge (`electron/preload.js`)**:
  ```javascript
  contextBridge.exposeInMainWorld('electronAPI', {
    getStartupSetting: () => ipcRenderer.invoke('desktop:get-startup-setting'),
    setStartupSetting: (enabled) => ipcRenderer.invoke('desktop:set-startup-setting', { enabled }),
  });
  ```
- **Main Process Handlers (`electron/main.js`)**:
  ```javascript
  ipcMain.handle('desktop:get-startup-setting', async () => {
    const settings = app.getLoginItemSettings();
    return {
      openAtLogin: settings.openAtLogin,
      openAsHidden: settings.openAsHidden,
      platform: process.platform,
    };
  });

  ipcMain.handle('desktop:set-startup-setting', async (event, { enabled }) => {
    app.setLoginItemSettings({
      openAtLogin: Boolean(enabled),
      openAsHidden: true, // macOS
      args: enabled ? ['--minimized'] : [], // Linux & Windows
    });
    const updated = app.getLoginItemSettings();
    return {
      success: true,
      openAtLogin: updated.openAtLogin,
    };
  });
  ```
- **UI Integration (`frontend/src/pages/SystemSettingsPage.tsx`)**:
  - The page already imports `ToggleSwitch` (`frontend/src/components/ToggleSwitch.tsx`).
  - Add a dedicated card: "Desktop & Startup Preferences".
  - Toggle: "Launch at System Startup (Minimized to Tray)".
  - Fallback: If `window.electronAPI` is undefined (web browser preview), display informative pill: "Managed by Desktop App".

---

### 1.4 Objective 4: Cross-Platform Desktop Packaging (`electron-builder`)

#### 1.4.1 Sidecar Go Binary Bundling
The Go binary `bin/swiss` is a standalone, statically linked binary compiled from `./cmd/swiss`.
- Verified compilation command: `go build -o bin/swiss ./cmd/swiss` (executable size ~12.7 MB).
- Cross-compilation capability verified with zero external C dependencies (`CGO_ENABLED=0`):
  - Linux: `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/swiss ./cmd/swiss`
  - Windows: `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/swiss.exe ./cmd/swiss`
  - macOS: `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o bin/swiss-darwin ./cmd/swiss`
- In `electron-builder`, sidecar bundling is handled via `extraResources`:
  ```json
  "extraResources": [
    {
      "from": "bin",
      "to": "bin",
      "filter": ["swiss*"]
    }
  ]
  ```
- Runtime binary resolver in Electron `main.js`:
  ```javascript
  function getSidecarBinaryPath() {
    const binName = process.platform === 'win32' ? 'swiss.exe' : 'swiss';
    if (app.isPackaged) {
      return path.join(process.resourcesPath, 'bin', binName);
    }
    return path.join(__dirname, '..', 'bin', binName);
  }
  ```

#### 1.4.2 `electron-builder` Configuration Specification
Configuration placed in `package.json` under `"build"` or `electron-builder.json`:
```json
{
  "appId": "com.chillingwombat.antigravity-swiss",
  "productName": "Antigravity Swiss Knife",
  "copyright": "Copyright © 2026 ChillingWombat",
  "directories": {
    "output": "dist-electron",
    "buildResources": "build"
  },
  "files": [
    "electron/**/*",
    "frontend/dist/**/*"
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
    "icon": "assets/logo.png",
    "synopsis": "Antigravity Swiss Knife - Multi-Account & Workspace Assistant",
    "description": "Unified manager for Google Gemini accounts, quotas, device profiles, and automations.",
    "maintainer": "ChillingWombat"
  },
  "deb": {
    "depends": ["libnotify4", "libnss3"]
  },
  "win": {
    "target": ["nsis"],
    "icon": "build/icon.ico"
  },
  "nsis": {
    "oneClick": false,
    "allowToChangeInstallationDirectory": true,
    "createDesktopShortcut": true,
    "createStartMenuShortcut": true,
    "shortcutName": "Antigravity Swiss Knife"
  },
  "mac": {
    "target": ["dmg", "zip"],
    "category": "public.app-category.developer-tools",
    "icon": "build/icon.icns",
    "hardenedRuntime": true,
    "gatekeeperAssess": false
  },
  "dmg": {
    "contents": [
      { "x": 130, "y": 220 },
      { "x": 410, "y": 220, "type": "link", "path": "/Applications" }
    ]
  }
}
```

---

### 1.5 Objective 5: Automated Verification Requirements

#### 1.5.1 Environment Pre-Check
- **XVFB Runner**: Verified present at `/usr/bin/xvfb-run` (exit code 0).
- **Node.js**: Verified v24.16.0 (`node -v`).
- **NPM**: Verified v11.13.0 (`npm -v`).
- **Go Compiler**: Verified `go1.27.1 linux/amd64` (`go version`).
- **Go Test Suite**: 100% passing across all 16 packages (`go test ./pkg/... ./cmd/...`).
- **Process Check**: Zero orphan `swiss` processes running (`pgrep swiss` returns empty).

#### 1.5.2 Headless / XVFB Verification Harness Design
An automated test runner script `scripts/verify-desktop-e2e.js` (invoked via `xvfb-run -a node scripts/verify-desktop-e2e.js` or `npm run test:desktop`):
1. **Pre-flight**:
   - Compiles Go binary: `go build -o bin/swiss ./cmd/swiss`
   - Verifies `frontend/dist/index.html` exists
2. **Launch Verification**:
   - Launches Electron in headless test mode: `xvfb-run -a electron . --test-verify`
   - In test mode, Electron main process:
     - Automatically verifies / spawns `bin/swiss daemon --web`
     - Instantiates `BrowserWindow`
     - Asserts `mainWindow.getTitle() === "Antigravity Swiss Knife"`
     - Probes `http://127.0.0.1:8765/api/status` with HTTP GET, asserting status 200 and `"version": "2.0.0"`
     - Emits `[VERIFY_SUCCESS]` to stdout
     - Triggers full graceful exit (`app.quit()`)
3. **Shutdown & Cleanup Verification**:
   - Awaits Electron process exit (code 0)
   - Checks `pgrep -x swiss` -> must return code 1 (0 processes found)
   - Checks lockfile and socket paths are unlinked
   - Prints independent validation PASS summary

---

## 2. Logic Chain

1. **Retirement of PySide6**:
   - Observation: 27 files in `antigravity_swiss/gui/` implement the old Qt6 desktop GUI, and `antigravity_swiss/__main__.py` exposes `gui` which requires `PySide6`.
   - Observation: ORIGINAL_REQUEST.md explicitly mandates zero Python runtime and total retirement of `antigravity_swiss/gui/`.
   - Invariant: Retiring `antigravity_swiss/gui/` eliminates ~200MB+ Qt6 dependencies, ensures no PySide6 runtime errors, and leaves the Python CLI purely for headless scripting/keyring operations until deprecated.
   - Deduction: Deleting all 27 files in `antigravity_swiss/gui/`, removing `run_gui` from `__main__.py`, deleting `tests/unit/test_gui.py`, and sanitizing PySide6 fixtures in `tests/conftest.py` is necessary, sufficient, and clean.

2. **Sidecar Lifecycle Guarantee**:
   - Observation: `swiss daemon --web` serves both the API (`/api/status`, `/api/accounts`, etc.) and the embedded web UI on `127.0.0.1:8765`.
   - Observation: When Electron exits, any unmanaged child process would become an orphaned daemon.
   - Invariant: Electron must own the child process lifecycle using `child_process.spawn()`, track the PID, and register cleanup handlers on `before-quit`, `exit`, and signals (`SIGINT`, `SIGTERM`), sending `SIGTERM` to the child.
   - Deduction: This satisfies Requirement R2, ensuring `pgrep swiss` returns 0 upon application shutdown.

3. **System Tray & Window Minimization**:
   - Observation: Users expect desktop utilities to minimize to tray when closing the window ('X') rather than quitting.
   - Invariant: Intercepting `mainWindow.on('close')` with `event.preventDefault()` and `mainWindow.hide()` keeps the process alive while `tray` remains visible.
   - Invariant: Full exit occurs only via Tray -> "Quit" or explicit `app.quit()`, where `isQuitting = true` is set before invoking close.
   - Deduction: This matches Requirement R3 and provides identical functionality to the legacy PySide6 `closeEvent` and `SwissKnifeTray`.

4. **Startup Integration**:
   - Observation: `app.setLoginItemSettings()` is Electron's native, multi-platform auto-start mechanism.
   - Observation: Linux requires desktop entry files in `~/.config/autostart`, Windows uses the Registry Run key, and macOS uses Login Items.
   - Invariant: Passing `--minimized` in `args` enables launch directly to system tray on boot.
   - Deduction: Exposing this via `desktop:get-startup-setting` and `desktop:set-startup-setting` IPC handlers and wiring it to `ToggleSwitch` in `SystemSettingsPage.tsx` fulfills Requirement R4 with zero external dependencies.

5. **Packaging Architecture**:
   - Observation: Go is statically compiled with zero CGO dependencies.
   - Observation: `electron-builder` packages files and bundles extra resources into `resources/bin`.
   - Invariant: Bundling `bin/swiss` via `extraResources` ensures the sidecar binary is always present next to the Electron application across AppImage, deb, exe, and dmg distributions.
   - Deduction: This satisfies Requirement R5.

---

## 3. Caveats

1. **TypeScript Build Pre-condition in `frontend/`**:
   - During survey, running `npm run build` in `frontend/` surfaced 6 unused variable errors in `frontend/src/pages/ScheduledTemplatesPage.tsx` (`Trash2`, `AlertCircle`, `CheckCircle2`, `isDeleting`, `pageFeedback`, `confirmDeleteSidecar`) failing under TypeScript `noUnusedLocals: true`.
   - *Recommendation*: Implementer must remove these unused imports/variables so that `tsc -b && vite build` completes cleanly.
2. **Linux Tray DBus Dependency**:
   - On Linux desktops, system tray integration relies on StatusNotifierItem (SNI) or AppIndicator (`libappindicator3-1` or `libayatana-appindicator3-1`). In headless environments without an active notification daemon or tray host, Electron's `new Tray()` may log a warning or no-op.
   - *Mitigation*: The test runner must gracefully handle tray initialization in XVFB without failing if DBus SNI host is absent.
3. **`app.setLoginItemSettings()` in Development vs Packaged Mode on Linux**:
   - On Linux, in development mode (`electron .`), Electron's `setLoginItemSettings` targets the `node_modules/electron/dist/electron` executable. In packaged mode (AppImage or deb), it targets the actual application executable.
4. **Existing Python E2E Tests**:
   - Existing E2E tests in `tests/e2e/test_tier1_features.py` tested the PySide6 widgets (F15-F24). Once `antigravity_swiss/gui/` is retired, running those specific tests in Python will fail unless updated or retired in favor of the new Electron verification suite.

---

## 4. Conclusion

The legacy Python PySide6 implementation (`antigravity_swiss/gui/`, 27 files) is completely decoupled from the Go daemon core and is ready for immediate retirement. The Electron standalone architecture cleanly replaces all legacy GUI functionality:
- **Zero Python Runtime**: Desktop operation, packaging, and UI rendering require zero Python or PySide6 components.
- **Sidecar Lifecycle**: Electron manages `bin/swiss daemon --web`, verifying `/api/status` on start and cleanly terminating the child process on quit with zero orphans.
- **System Tray**: Provides restore, account status, quick account switch, settings navigation, and close-to-tray minimization.
- **Startup Integration**: Uses native `app.setLoginItemSettings({ openAtLogin, args: ['--minimized'] })` with an IPC bridge to React's `ToggleSwitch`.
- **Packaging**: `electron-builder` with `extraResources` packages the Go binary sidecar into Linux (AppImage/deb), Windows (nsis/exe), and macOS (dmg/zip).
- **Automated Verification**: Headless validation script runs under `/usr/bin/xvfb-run`, validating window title, live API status, and clean process termination (`pgrep swiss = 0`).

---

## 5. Verification Method

To independently verify the observations and findings in this report, execute the following commands from the repository root (`/mnt/Data/Projects/Antigravity Swiss Knife`):

1. **Verify Legacy GUI File Count & Paths**:
   ```bash
   find antigravity_swiss/gui -type f -not -name "*.pyc" | sort
   # Output: Exactly 27 files listed in Section 1.1.1
   ```

2. **Verify Python CLI GUI Subcommand**:
   ```bash
   grep -n "run_gui" antigravity_swiss/__main__.py
   # Output: Lines 321 and 593
   ```

3. **Verify PySide6 Test Coupling**:
   ```bash
   ls -la tests/unit/test_gui.py
   grep -n "PySide6" tests/conftest.py
   ```

4. **Verify Go Daemon & Statically Linked Binary**:
   ```bash
   file bin/swiss
   ./bin/swiss version
   # Output: Antigravity Swiss Knife v2.0.0 (Go 1.24.6)
   ```

5. **Verify 100% Pass of Go Test Suite**:
   ```bash
   go test ./pkg/... ./cmd/...
   # Output: ok across all 16 packages
   ```

6. **Verify XVFB & Node Tooling**:
   ```bash
   which xvfb-run
   # Output: /usr/bin/xvfb-run
   node -v
   # Output: v24.16.0
   ```

7. **Verify Process Cleanup Baseline**:
   ```bash
   pgrep swiss
   # Output: exit code 1 (zero running processes)
   ```
