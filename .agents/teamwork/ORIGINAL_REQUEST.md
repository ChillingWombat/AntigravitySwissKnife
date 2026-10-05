# Original User Request

## 2026-10-05T10:08:38Z

Refactor Antigravity Swiss Knife to completely migrate the desktop GUI to a modern, self-contained Electron standalone application. The Electron shell bundles and directly manages the Go backend binary as an integrated sidecar, provides native system tray integration with minimize-to-tray on close, provides a system startup setting, and completely removes the legacy Python/PySide6 wrapper.

Working directory: /mnt/Data/Projects/Antigravity Swiss Knife
Integrity mode: development

## Requirements

### R1. Standalone Electron Desktop Architecture & Python Retirement
- Set up an Electron desktop application shell inside the repository that directly packages and serves the compiled TypeScript + React 19 frontend (`frontend/`).
- Completely retire and delete the legacy Python PySide6 desktop GUI code (`antigravity_swiss/gui/`) and any Python desktop launch paths.
- Ensure the application is 100% self-contained: no external browsers (Chrome/Edge/Firefox), no external helper scripts, and no Python runtime required.

### R2. Integrated Go Daemon Lifecycle Management (Bundled Sidecar)
- The Electron main process must automatically spawn and supervise the Go binary (`bin/swiss daemon --web`) upon application launch if not already running.
- They must not be separated: when the Electron application completely exits (e.g. via Tray -> Quit or App Exit), it must gracefully terminate the Go daemon process, clear any lockfiles, and leave no orphaned processes or stray modifications in the background.

### R3. System Tray & Window Behavior
- Window Close ('X') Behavior: By default, clicking the window close ('X') button must minimize/hide the window to the system tray rather than killing the app.
- System Tray: Implement a native system tray icon with a context menu (Show/Restore, Active Account status, Quick Account Switch, Settings, and Quit).
- Double-clicking or clicking the tray icon restores and focuses the main application window.

### R4. System Settings Startup Integration
- In System Settings, add a user-configurable toggle option: "Launch at System Startup (Minimized to Tray)".
- When enabled, use Electron's native OS auto-launcher (`app.setLoginItemSettings`) to register the application to launch automatically minimized to the tray at system login across Linux, Windows, and macOS.

### R5. Cross-Platform Desktop Packaging Configuration
- Configure `electron-builder` (or equivalent standard packager) with scripts in `package.json` to build standalone distributions:
  - Linux: AppImage / deb (bundling the Linux Go binary).
  - Windows: exe / nsis (bundling the Windows Go binary).
  - macOS: dmg / zip (bundling the macOS Go binary).

## Acceptance Criteria

### Standalone Desktop Shell & Zero Python
- [ ] Running `npm run desktop` (or the equivalent packaged command) opens an independent Electron desktop window displaying the Material Design 3 React UI.
- [ ] No Python process (`python3`, `PySide6`) is spawned or needed at any point during app launch, operation, or packaging.
- [ ] All feature pages (Account Switcher, Quota Dashboard, Custom Models, App Enhancements, Task Automations, Tools Marketplace, System Settings) function seamlessly inside the Electron window.

### Daemon Lifecycle & Process Cleanup
- [ ] Electron main process verifies whether the Go daemon is active; if not, it spawns `bin/swiss daemon --web` automatically.
- [ ] When the Electron app performs a full Quit, the Go daemon child process is cleanly terminated via SIGTERM/graceful shutdown, verified with `pgrep swiss` returning 0 orphaned processes.

### Tray & Close Behavior
- [ ] Clicking window close ('X') hides the window while the tray icon remains active.
- [ ] Context menu on tray icon includes "Open Antigravity Swiss Knife" and "Quit". Clicking "Open" restores the window. Clicking "Quit" completely shuts down both Electron and the Go daemon.

### System Startup Setting
- [ ] System Settings UI includes the startup toggle with persistent state.
- [ ] Toggle correctly reflects and modifies `app.getLoginItemSettings()` / `app.setLoginItemSettings()`.

### Independent Verification
- [ ] Automated verification script executes headless or XVFB-based Electron test verifying:
  1. Main window creation and title matching "Antigravity Swiss Knife".
  2. Live API response from `http://127.0.0.1:8765/api/status`.
  3. Clean termination with zero dangling processes.
- [ ] All existing Go tests (`go test ./pkg/... ./cmd/...`) pass.

## 2026-10-05T10:47:44Z

Please resume and continue the Electron migration tasks across your team and subagents. We are waiting for Milestone 1 completion and progression to Milestone 2.

## 2026-10-05T12:05:51Z

Please continue and conclude the final verification and report.
