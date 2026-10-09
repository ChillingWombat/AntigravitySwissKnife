# Packaging, VM Validation & Installation

Antigravity Swiss Knife uses standard operating system packaging pipelines, automated virtual machine validation suites, and isolated development workflows to deliver deterministic cross-platform releases.

---

## 1. Standard Installers Overview

The project generates official distribution packages using Electron Builder:

| Platform | Target Format | Architecture | Default Target Directory | Build Command |
| :--- | :--- | :--- | :--- | :--- |
| **Linux** | Debian (`.deb`) | `amd64` (x86_64) | `/opt/antigravity-swiss-knife` | `npm run build:linux` |
| **Windows** | NSIS (`.exe`) | `x64` | `%LOCALAPPDATA%\Programs\Antigravity Swiss Knife` | `npm run build:win` |
| **macOS** | Disk Image (`.dmg`) | Universal (`arm64`, `x64`) | `/Applications/Antigravity Swiss Knife.app` | `npm run build:mac` |

### 1.1 Packaging Configuration in `package.json`
Installer targets are configured strictly with standard system install paths:
- **Linux**: Installs into `/opt/antigravity-swiss-knife` and creates a desktop shortcut at `/usr/share/applications/antigravity-swiss-knife.desktop`.
- **Windows**: Configured with `oneClick: false` and `perMachine: false` to allow non-administrative installations into the user's Local AppData directory.
- **macOS**: Generates a standard Drag-and-Drop `.dmg` volume pointing to `/Applications`.

---

## 2. Dedicated Linux Installer (`scripts/install-linux.js`)

For developers on Linux distributions that do not use `.deb` packages (Fedora, Arch Linux, openSUSE) or users who prefer not to use `sudo`:

```bash
npm run install:linux
```

This script:
1. Compiles frontend assets and builds the Go companion daemon natively.
2. Stages the entire runtime out-of-tree into:
   ```
   ~/.local/share/antigravity-swiss-knife/app/
   ```
3. Resolves absolute paths for binaries and application icons.
4. Generates a standard FreeDesktop desktop entry at:
   ```
   ~/.local/share/applications/antigravity-swiss-knife.desktop
   ```
5. Updates the local desktop database (`update-desktop-database ~/.local/share/applications/`), making the application immediately searchable in GNOME, KDE, and other application menus.

---

## 3. Quickemu Virtual Machine Testing

To verify installers on target operating systems without physical hardware, the repository includes an automated VM test harness documented in `docs/QUICKEMU_VM_TESTING.md`.

### 3.1 VM Configurations
- **Windows 11 26H2 Guest**:
  - Accelerated via KVM with 8 vCPUs and 16 GB RAM.
  - Quickemu configuration: `windows-11-custom.conf`.
  - Verifies NSIS installer execution, Start Menu shortcuts, and Windows Credential Manager integration.
- **macOS Ventura 13 Guest**:
  - Accelerated via KVM (OpenCore).
  - Quickemu configuration: `macos-ventura.conf`.
  - Verifies `.dmg` mounting, application bundle codesigning, and macOS Keychain services.

### 3.2 Automated Artifact Staging
During VM test runs, built installer artifacts are staged locally via an HTTP transfer server on port `8010`:
```bash
python3 -m http.server 8010 --directory release/
```
Guest VMs fetch artifacts automatically, run PowerShell or bash test scripts, and report exit codes back to the host.

---

## 4. Headless Packaging Verification (`scripts/verify-crossplatform-packaging.js`)

Before distributing releases, the packaging pipeline executes an automated verification script to validate binary integrity without requiring active VM boots:

```bash
node scripts/verify-crossplatform-packaging.js
```

### Checks Performed:
1. **PE32+ Executable Header Inspection**: Validates that Windows `.exe` binaries possess valid PE signatures, 64-bit machine architectures (`IMAGE_FILE_MACHINE_AMD64`), and correct subsystem declarations.
2. **Mach-O Universal Binary Inspection**: Validates macOS `.dmg` and app bundle binaries for valid 64-bit Mach-O magic bytes (`MH_MAGIC_64`) across both `arm64` and `x86_64` slices.
3. **Debian Package Control Inspection**: Unpacks `.deb` archives to verify `control` metadata, dependencies (`libc6`, `libsecret-1-0`), file permissions, and directory hierarchies.
4. **Daemon Sidecar Check**: Confirms the companion Go daemon (`swiss` or `swiss.exe`) is bundled inside the installer package with executable permissions.

---

## 5. Multi-Branch Laptop Workflow & Git Worktrees

When collaborating with autonomous agents or testing long-running experimental features, developers must keep their daily production companion app isolated from dirty working trees.

The `scripts/worktree.sh` utility manages decoupled Git worktrees:

```bash
# List all active worktrees
./scripts/worktree.sh list

# Create a dedicated agent worktree
./scripts/worktree.sh create feature/quota-sync

# Remove a completed worktree cleanly
./scripts/worktree.sh remove feature/quota-sync
```

Additionally, running `bash release/run.sh` or `npm run app:prod` launches the pre-built application from `release/` rather than the active working tree, ensuring editor restarts or test builds do not interrupt ongoing development.
