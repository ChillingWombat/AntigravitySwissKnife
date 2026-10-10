# System Settings

[Features](Features.md) / System Settings

System Settings is the central configuration console in Antigravity Swiss Knife. Pinned to the bottom-left of the navigation rail, it controls daemon runtime modes, application password protection, local filesystem paths, update checks, and diagnostic logs.

---

## 1. General Settings

The General category governs workstation integration and application lifecycle behavior:
- **Runtime Mode Selection**:
  - `Standalone App Mode`: The Electron desktop application runs the background daemon internally as a child process.
  - `Dedicated Daemon Mode`: The pure-Go background daemon runs continuously in the background (via systemd or launchd), and the desktop GUI connects as a lightweight client over loopback HTTP or Unix domain sockets.
- **Autostart at Boot**: Configures operating system autostart entries (`~/.config/autostart/` on Linux, Registry `Run` keys on Windows, LaunchAgents on macOS).
- **System Tray Behavior**: Toggles whether closing the main window exits the application or minimizes it to the system status tray.
- **Persistent Visual Effects**: Controls high-contrast accents, theme transitions, and CSS animation overrides.
- **Release Channel & Updates**: Checks for new tagged releases on GitHub, showing change summaries and offering one-click updates.

---

## 2. App Access Password & Workstation Gating

To protect multi-account credentials and active session states on shared or office workstations, developers can enable App Access Password protection:
- **Key Derivation Standards**: Passwords are hashed using RFC 2898 PBKDF2-HMAC-SHA256 with 100,000 iterations and a cryptographically secure 16-byte random salt (`pbkdf2:sha256:100000:<salt>:<hash>`).
- **Complexity Enforcement**: Rejects passcodes shorter than 6 characters and requires combinations of letters, numbers, or symbols.
- **Secure Application Lock Screen**: When enabled, launching the application presents a full-screen authentication modal. The user cannot view accounts, inspect tokens, or trigger rotations until the correct master password is entered.

---

## 3. Path & Storage Configuration

Manages local filesystem paths across the three supported Antigravity application environments:
- **Antigravity Desktop Path**: Defaults to `~/.config/Antigravity` (Linux), `%APPDATA%\Antigravity` (Windows), or `~/Library/Application Support/Antigravity` (macOS).
- **Antigravity CLI Binary & State Path**: Paths for the headless `agy` command-line utility.
- **Code Extension Installation Path**: Extensions folder under VS Code or cursor environments.
- **Custom Path Overrides**: Allows developers using custom portable installations or out-of-tree directories to redirect Swiss Knife hooks to non-standard locations.
- **Storage Metrics**: Reports total disk space consumed by Swiss Knife logs, backups, and caches.

---

## 4. Error, Privacy & Diagnostic Logs

Provides privacy controls and debugging exports:
- **Zero-Cloud Privacy Invariant**: Affirms that zero telemetry packets or account credentials leave the local machine.
- **Diagnostic Bundle Exporter**: Compiles local daemon logs, system architecture details, and IPC handshake traces into a sanitized ZIP archive for troubleshooting.
- **Crash Recovery & Backup Restore**: Restores previous configuration snapshots (`.swiss.bak`) if host configuration files become corrupted.

---

## 5. About & Version Information

Displays the active build metadata:
- Semantic release version (e.g., `v0.2.0`).
- Git commit hash and build timestamp.
- Embedded Go runtime version and Electron build.
- Direct links to the GitHub issue tracker and documentation wiki.
