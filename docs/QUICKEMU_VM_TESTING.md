# Quickemu VM Testing & Cross-Platform Packaging Guide

This guide documents the verification workflows for Antigravity Swiss Knife on Windows 11 and macOS Ventura using local Quickemu virtual machines, alongside the automated headless packaging verification suite and platform-specific scripts.

---

## 1. Host Virtualization Environment

### System Configuration & Hardware Acceleration
- **Host CPU**: Hardware virtualization support (Intel VT-x or AMD-V).
- **QuickEMU Version**: QuickEMU 4.9.9 with QEMU 8.x+ (`/usr/bin/quickemu`, `/usr/bin/qemu-system-x86_64`).
- **KVM Acceleration**: Direct `/dev/kvm` read/write access. User `david` is granted access via POSIX ACL (`user:david:rw-`). Check acceleration status with:
  ```bash
  /usr/sbin/kvm-ok
  ```
  Verified output: `INFO: /dev/kvm exists; KVM acceleration can be used`.
- **Memory Allocation**: Host system has 30 GiB physical RAM with ~19 GiB active desktop footprint (Docker Desktop capped to 4 GiB / 8 vCPUs / 1 GiB swap via `~/.docker/desktop/settings-store.json`).
- **Rule — Single VM Concurrency**:
  **Run only one VM at a time.** Booting both Windows (8 GiB) and macOS (10 GiB) guests concurrently will exceed physical memory and cause swap thrashing.
- **Graphics Acceleration**: Virtual machines run emulated displays (`vmware-svga`) without Metal or hardware 3D passthrough. Windows 11 configures `gl="off"` so that QEMU monitor screendumps function properly. Antigravity Swiss Knife (Electron + Go) operates with CPU software compositing.

---

## 2. Pre-Configured Virtual Machines

All guest images and configuration files reside in `/home/david/VMs/`.

| Virtual Machine | Configuration File | Allocated Resources | OS Disk Image | Default User / Password | Installed Environment |
|---|---|---|---|---|---|
| **Windows 11 26H2** | `windows-11-English-United-States.conf` | 8 cores, 8 GB RAM | `windows-11-English-United-States/disk.qcow2` (21 GB on disk, 64 GB max) | `Quickemu` / `quickemu` (admin, autologon) | Windows Terminal (MSYS2 zsh), JetBrains Mono Nerd Font, Antigravity 2.0 app (`%LOCALAPPDATA%\Programs\antigravity`), CLI `agy` |
| **macOS Ventura 13** | `macos-ventura.conf` | 8 cores, 10 GB RAM | `macos-ventura/disk.qcow2` (26 GB on disk, 128 GB max) | `david` / `quickemu` | zsh oh-my-zsh setup, CLT/git, Antigravity 2.0 app (`/Applications/Antigravity.app`), CLI `agy` |
| **macOS Monterey 12** | `macos-monterey.conf` | 4 cores, 8 GB RAM | `macos-monterey/disk.qcow2` (30 GB on disk, 128 GB max) | `david` / `quickemu` | Legacy testing image (too old for current Antigravity) |

### Virtual Machine Configuration Details
- **Windows 11 (`windows-11-English-United-States.conf`)**:
  ```ini
  guest_os="windows"
  disk_img="windows-11-English-United-States/disk.qcow2"
  iso="windows-11-English-United-States/windows-11.iso"
  fixed_iso="windows-11-English-United-States/virtio-win.iso"
  disk_size="64G"
  tpm="on"
  secureboot="off"
  gl="off"
  cpu_cores=8
  ram="8G"
  extra_args="-display gtk,grab-on-hover=on,zoom-to-fit=off,gl=off,show-cursor=on"
  ```
- **macOS Ventura (`macos-ventura.conf`)**:
  ```ini
  guest_os="macos"
  disk_img="macos-ventura/disk.qcow2"
  img="macos-ventura/RecoveryImage.img"
  disk_size="128G"
  macos_release="ventura"
  cpu_cores=8
  ram="10G"
  extra_args="-display gtk,grab-on-hover=on,zoom-to-fit=off,gl=on,show-cursor=on"
  ```

### Guest Automation Tools
Host utilities in `/home/david/VMs/bin/`:
- `vmctl.py <monitor.sock> shot <out.ppm>`: Takes a QEMU screendump and converts it to PNG via ffmpeg.
- `vmctl.py <monitor.sock> key <key>`: Sends keystrokes to guest.
- `vmctl.py <monitor.sock> type <text>`: Types strings into guest.
- `qmpctl.py <qmp.sock> click <x> <y>`: Sends mouse events over QMP.
- `patch-ventura-sh.sh`: Re-applies QMP device patches (`id=vga0`, `id=tablet0`) after quickemu conf changes.

---

## 3. Staging Pipeline & Local HTTP Transfer Server

Transfer test artifacts between the host and guests using a lightweight HTTP server on port 8010.

### 3.1 Start Transfer Server
```bash
python3 -m http.server 8010 --directory /home/david/VMs/transfer
```
Guests access the host server at `http://10.0.2.2:8010` through the default QEMU user-mode network gateway.

### 3.2 Build and Stage Artifacts
```bash
# 1. Compile cross-platform Go binaries
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/swiss.exe ./cmd/swiss
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/swiss-darwin-amd64 ./cmd/swiss
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/swiss-darwin-arm64 ./cmd/swiss

# 2. Build packaged releases
node scripts/build-release.js windows
node scripts/build-release.js macos --unpacked

# 3. Stage files to transfer docroot
cp "release/windows/Antigravity Swiss Knife.exe" /home/david/VMs/transfer/ASK-Setup.exe
cp "bin/swiss.exe" /home/david/VMs/transfer/swiss.exe
(cd release/macos/mac && zip -rq /home/david/VMs/transfer/ASK-mac-x64.zip "Antigravity Swiss Knife.app")
cp "bin/swiss-darwin-amd64" /home/david/VMs/transfer/swiss-mac

# 4. Stage automated guest verification scripts
cp scripts/platform/windows/verify-guest.ps1 /home/david/VMs/transfer/win-verify-ask.ps1
cp scripts/platform/macos/verify-guest.sh /home/david/VMs/transfer/mac-verify-ask.sh
```

### 3.3 Staged Artifacts Catalog
| Staged File | Size | Format / Architecture | Purpose |
|---|---|---|---|
| `ASK-Setup.exe` | 83 MB | PE32 NSIS Installer | Full Windows 11 installation package |
| `swiss.exe` | 15 MB | PE32+ (AMD64 x86-64) | Standalone Windows Go binary |
| `ASK-mac-x64.zip` | 325 MB | macOS App Bundle Zip | Full macOS application bundle |
| `swiss-mac` | 15 MB | Mach-O 64-bit (x86_64) | Standalone macOS Go binary |
| `win-verify-ask.ps1` | 4 KB | PowerShell Script | Automated guest test harness for Windows 11 |
| `mac-verify-ask.sh` | 3 KB | Bash Script | Automated guest test harness for macOS Ventura |

---

## 4. Windows 11 Guest Test Execution

### 4.1 Launch VM
```bash
quickemu --vm /home/david/VMs/windows-11-English-United-States.conf
```

### 4.2 Automated Guest Verification (One-Liner)
Inside Windows Terminal, MSYS2 zsh, or via SSH (`ssh -p 22220 Quickemu@localhost`):
```powershell
Invoke-WebRequest "http://10.0.2.2:8010/win-verify-ask.ps1" -OutFile "$env:TEMP\win-verify-ask.ps1"
powershell.exe -ExecutionPolicy Bypass -File "$env:TEMP\win-verify-ask.ps1"
```

### 4.3 Detailed PowerShell Test Flow
The script executes 6 automated steps:
1. **Download artifacts**: Fetches `ASK-Setup.exe` and `swiss.exe` from `http://10.0.2.2:8010`.
2. **Smoke test standalone binary**: Runs `swiss.exe version` (verifies "Antigravity Swiss Knife") and `swiss.exe --help`.
3. **Silent NSIS install**: Executes `ASK-Setup.exe /S` and waits for completion.
4. **Verify installation tree**: Asserts existence of:
   - `%LOCALAPPDATA%\Programs\Antigravity Swiss Knife\Antigravity Swiss Knife.exe`
   - `%LOCALAPPDATA%\Programs\Antigravity Swiss Knife\resources\bin\swiss.exe` (>10 MB)
5. **Launch application**: Starts `Antigravity Swiss Knife.exe`, sleeps 5 seconds, and polls Go daemon health endpoint `http://127.0.0.1:8765/api/status` for `{"daemon_running": true}`.
6. **Verify config paths**: Checks `%APPDATA%\antigravity-swiss` and `%USERPROFILE%\.gemini`.

### 4.4 Screenshot Capture & VM Shutdown
```bash
# Capture verification screenshot on host
python3 /home/david/VMs/bin/vmctl.py \
  /home/david/VMs/windows-11-English-United-States/windows-11-English-United-States-monitor.socket \
  shot /tmp/win11-packaging-verification.ppm

# Clean guest shutdown via QEMU monitor
echo "system_powerdown" | nc -U /home/david/VMs/windows-11-English-United-States/windows-11-English-United-States-monitor.socket
```

---

## 5. macOS Ventura Guest Test Execution

### 5.1 Launch VM
```bash
# Ensure Windows VM has completely shut down first
quickemu --vm /home/david/VMs/macos-ventura.conf
# Or via patched script:
# ./macos-ventura/macos-ventura.sh
```

### 5.2 Automated Guest Verification (One-Liner)
Inside macOS Terminal.app or via SSH (`ssh -p 22220 david@localhost`):
```bash
curl -fsSL http://10.0.2.2:8010/mac-verify-ask.sh | bash
```

### 5.3 Detailed Bash Test Flow
The script executes 6 automated steps:
1. **Download standalone binary**: Fetches `swiss-mac` from `http://10.0.2.2:8010`, marks executable (`chmod +x`), runs `version` and `--help`.
2. **Download bundle**: Fetches `ASK-mac-x64.zip`.
3. **Install application**: Unpacks to `/Applications/Antigravity Swiss Knife.app` and strips quarantine attribute (`xattr -dr com.apple.quarantine`).
4. **Verify bundle integrity**: Asserts existence of:
   - `/Applications/Antigravity Swiss Knife.app/Contents/MacOS/Antigravity Swiss Knife`
   - `/Applications/Antigravity Swiss Knife.app/Contents/Resources/bin/swiss` (executable, >10 MB)
5. **Launch application**: Runs `open "$APP"`, sleeps 5 seconds, and probes `http://127.0.0.1:8765/api/status` for `{"daemon_running": true}`.
6. **Verify config paths**: Checks `~/Library/Application Support/antigravity-swiss`.

### 5.4 Screenshot Capture & VM Shutdown
```bash
# Capture verification screenshot on host
python3 /home/david/VMs/bin/vmctl.py \
  /home/david/VMs/macos-ventura/macos-ventura-monitor.socket \
  shot /tmp/macos-packaging-verification.ppm

# Clean guest shutdown via QEMU monitor
echo "system_powerdown" | nc -U /home/david/VMs/macos-ventura/macos-ventura-monitor.socket
```

---

## 6. Dedicated Platform-Specific Scripts & Directory Structure

Platform-specific codes and scripts are organized under `scripts/platform/`:

```
scripts/platform/
├── README.md              # Documentation of platform-specific workflows
├── linux/
│   ├── run.sh             # Launch packaged AppImage or unpacked binary (--no-sandbox)
│   └── verify-guest.sh    # Verify Linux packages (.deb, AppImage), binary, daemon, and paths
├── macos/
│   ├── run.sh             # Launch macOS .app bundle
│   └── verify-guest.sh    # macOS guest verification test harness
└── windows/
    ├── run.bat            # Launch Windows executable or unpacked bundle
    └── verify-guest.ps1   # Windows guest verification test harness (PowerShell)
```

In addition:
- **Go OS Detached Process Handling**:
  - `pkg/process/detach_windows.go`: `CreationFlags: 0x00000008` (`DETACHED_PROCESS`)
  - `pkg/process/detach_unix.go`: `SysProcAttr: { Setsid: true }`
- **Go OS Path Constants**:
  - `pkg/core/constants.go`: Windows (`%APPDATA%`, `%LOCALAPPDATA%`), macOS (`~/Library/Application Support`), Linux (`~/.config`).
- **Electron Daemon Manager**:
  - `electron/daemon-manager.js`: Resolves `.exe` on win32, skips Unix socket unlinks and chmod on Windows, binds `runCwd` to `app.getPath('userData')`.

---

## 7. Automated Verification Test Results

### 7.1 Cross-Platform Packaging Verification (`npm run verify:packaging`)
Executed via `scripts/verify-crossplatform-packaging.js`:
- **Phase 1: Cross-Compilation & Binary Format Validation**:
  - `bin/swiss.exe`: Verified MZ DOS header, `PE\0\0` signature, Machine `0x8664` (AMD64), PE32+ Optional Magic `0x020B`.
  - `bin/swiss-darwin-amd64`: Verified Mach-O 64-bit magic (`0xFEEDFACF`), CPU type x86_64 (`0x01000007`).
  - `bin/swiss-darwin-arm64`: Verified Mach-O 64-bit magic (`0xFEEDFACF`), CPU type arm64 (`0x0100000C`).
- **Phase 2: Windows Package & Unpacked Tree Verification**:
  - `package.json` NSIS target, `oneClick: true`, `perMachine: false`, sidecar mapping `bin/swiss.exe -> bin/swiss.exe`.
  - `release/windows/win-unpacked/resources/bin/swiss.exe`: Exists, 14.33 MB, valid PE32+ header.
- **Phase 3: macOS Package & App Bundle Structure Verification**:
  - `package.json` DMG target, sidecar mapping `bin/swiss -> bin/swiss`.
  - `release/macos/mac/Antigravity Swiss Knife.app/Contents/Resources/bin/swiss`: Exists, 14.25 MB, executable permissions (`chmod 0755`), valid Mach-O 64-bit header.
- **Phase 4: Wine Emulation Headless Execution**:
  - Wine 11.0 detected at `/usr/bin/wine`.
  - `wine bin/swiss.exe version`: Exited with code `0`, output matched `Antigravity Swiss Knife`.
  - `wine bin/swiss.exe --help`: Exited with code `0`, CLI commands listed.
- **Phase 5: DaemonManager Cross-Platform Path Resolution Simulation**:
  - Simulated `win32`, `darwin`, and `linux` paths; verified Windows socket unlink and chmod bypass.
- **Phase 6: Cross-Platform Path Constants Verification**:
  - Verified `pkg/core/constants.go` paths for Windows and macOS.
- **Result: 48/48 checks passed (0 failures).**

### 7.2 Adversarial Stress Testing (`node tests/adversarial_packaging_m15.js`)
- Binary Header Parser Fuzzing:
  - Synthetic empty buffers (0 bytes) and sub-64-byte truncated buffers rejected cleanly.
  - Corrupt DOS magic and invalid `e_lfanew` offsets caught with descriptive errors.
  - 32-bit x86 PE rejected (`isAmd64: false`).
  - ELF magic (`0x7F454C46`) and 32-bit Mach-O magic rejected cleanly.
- Real Wine Emulation:
  - Verified `version`, `--help`, and verified non-zero exit code on invalid CLI commands.
- Failure Injection:
  - Verified that corrupted mock headers properly increment failure counter and cause exit code 1.
- **Result: 21/21 stress tests passed (0 failures).**

### 7.3 Release Pipeline & Security Scan (`node scripts/verify-release-pipeline.js`)
- GitHub Actions workflow (`.github/workflows/release.yml`) validated.
- Matrix targets verified: `ubuntu-latest` (.deb), `windows-latest` (.exe), `macos-latest` (.dmg).
- Codex Security scan preflight: Zero exposed secrets or vulnerabilities detected.
- **Result: 45/45 checks passed (0 failures).**

### 7.4 Linux Verification Script (`scripts/platform/linux/verify-guest.sh`)
- Standalone binary `bin/swiss` verified (Go 1.24.6 ELF 64-bit).
- Debian package `Antigravity-Swiss-Knife-0.2.0-x86_64.deb` verified (contains `resources/bin/swiss`).
- AppImage verified.
- Unpacked distribution tree verified.
- Daemon endpoint probed (`http://127.0.0.1:8765/api/status`).
- Config directory verified (`~/.config/antigravity-swiss`).
- **Result: 5/5 phases passed (0 failures).**

### 7.5 Physical Binary Verification Summary
```
bin/swiss:                                                                            ELF 64-bit LSB executable, x86-64, Go 1.24
bin/swiss-darwin-amd64:                                                               Mach-O 64-bit x86_64 executable
bin/swiss-darwin-arm64:                                                               Mach-O 64-bit arm64 executable
bin/swiss.exe:                                                                        PE32+ executable for MS Windows (console), x86-64
release/linux/Antigravity-Swiss-Knife-0.2.0-x86_64.deb:                               Debian binary package (format 2.0)
release/linux/Antigravity-Swiss-Knife-0.2.0-x86_64.AppImage:                          ELF 64-bit LSB executable, x86-64
release/windows/Antigravity Swiss Knife.exe:                                          PE32 executable Nullsoft Installer self-extracting archive
release/macos/mac/Antigravity Swiss Knife.app/Contents/MacOS/Antigravity Swiss Knife: Mach-O 64-bit x86_64 executable
```
