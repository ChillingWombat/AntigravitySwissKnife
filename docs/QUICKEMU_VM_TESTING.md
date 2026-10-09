# Quickemu VM Testing & Cross-Platform Packaging Guide

This guide documents the verification workflows for Antigravity Swiss Knife on Windows 11 and macOS Ventura using local Quickemu virtual machines, alongside the automated headless packaging verification suite.

---

## 1. Host Virtualization Environment

### System Requirements & Acceleration
- **Host CPU**: Hardware virtualization support (Intel VT-x or AMD-V).
- **KVM Acceleration**: Direct `/dev/kvm` read/write access. User `david` is granted access via POSIX ACL (`user:david:rw-`). Check acceleration status with:
  ```bash
  /usr/sbin/kvm-ok
  ```
  Expected output: `INFO: /dev/kvm exists; KVM acceleration can be used`.
- **Memory Allocation**: Host system has 30 GiB physical RAM with ~18 GiB active desktop footprint. Windows 11 allocates 8 GiB RAM and macOS Ventura allocates 10 GiB RAM.
- **Rule — Single VM Concurrency**:
  **Run only one VM at a time.** Booting both Windows and macOS guests concurrently will exceed physical memory and cause swap thrashing.
- **Graphics Acceleration**: Virtual machines run emulated displays (`vmware-svga`) without Metal or hardware 3D passthrough. Windows 11 configures `gl="off"` so that QEMU monitor screendumps function properly. Antigravity Swiss Knife (Electron + Go) operates with CPU software compositing.

---

## 2. Pre-Configured Virtual Machines

All guest images reside in `/home/david/VMs/`.

| Virtual Machine | Configuration File | Allocated Resources | OS Disk Image | Default User / Password |
|---|---|---|---|---|
| **Windows 11 26H2** | `/home/david/VMs/windows-11-English-United-States.conf` | 8 cores, 8 GB RAM | `windows-11-English-United-States/disk.qcow2` (64 GB) | `Quickemu` / `quickemu` (autologon) |
| **macOS Ventura 13** | `/home/david/VMs/macos-ventura.conf` | 8 cores, 10 GB RAM | `macos-ventura/disk.qcow2` (128 GB) | `david` / `quickemu` |

### Guest Automation Tools
Host utilities in `/home/david/VMs/bin/`:
- `vmctl.py <monitor.sock> shot <out.ppm>`: Takes a QEMU screendump and converts it to PNG via ffmpeg.
- `vmctl.py <monitor.sock> key <key>`: Sends keystrokes to guest.
- `vmctl.py <monitor.sock> type <text>`: Types strings into guest.
- `qmpctl.py <qmp.sock> click <x> <y>`: Sends mouse events over QMP.

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
cp "release/windows/Antigravity-Swiss-Knife-Setup-0.2.0.exe" /home/david/VMs/transfer/ASK-Setup.exe
cp "bin/swiss.exe" /home/david/VMs/transfer/swiss.exe

(cd release/macos/mac && zip -rq /home/david/VMs/transfer/ASK-mac-x64.zip "Antigravity Swiss Knife.app")
cp "bin/swiss-darwin-amd64" /home/david/VMs/transfer/swiss-mac
```

---

## 4. Windows 11 Guest Test Execution

### 4.1 Launch VM
```bash
quickemu --vm /home/david/VMs/windows-11-English-United-States.conf
```

### 4.2 Guest Verification Script (PowerShell)
Execute directly in Windows Terminal, MSYS2 zsh, or via SSH (`ssh -p 22220 Quickemu@localhost`):

```powershell
$base = "http://10.0.2.2:8010"
$temp = $env:TEMP

# 1. Download installer and standalone binary
Write-Host "Downloading test artifacts..."
Invoke-WebRequest "$base/ASK-Setup.exe" -OutFile "$temp\ASK-Setup.exe"
Invoke-WebRequest "$base/swiss.exe" -OutFile "$temp\swiss.exe"

# 2. Smoke test standalone Go binary
Write-Host "`nTesting standalone Go binary:"
& "$temp\swiss.exe" version
& "$temp\swiss.exe" --help

# 3. Silent NSIS installation
Write-Host "`nRunning silent NSIS install (/S)..."
Start-Process -FilePath "$temp\ASK-Setup.exe" -ArgumentList "/S" -Wait

# 4. Verify installed directories and sidecar binary
$installDir = "$env:LOCALAPPDATA\Programs\Antigravity Swiss Knife"
Write-Host "`nChecking installation paths in $installDir:"
if (!(Test-Path "$installDir\Antigravity Swiss Knife.exe")) {
    throw "Executable missing: Antigravity Swiss Knife.exe"
}
if (!(Test-Path "$installDir\resources\bin\swiss.exe")) {
    throw "Sidecar binary missing: resources\bin\swiss.exe"
}
Write-Host "✓ Installed executable and sidecar binary verified."

# 5. Launch application
Write-Host "`nLaunching application..."
Start-Process -FilePath "$installDir\Antigravity Swiss Knife.exe"
Start-Sleep -Seconds 4

# 6. Verify sidecar daemon health
Write-Host "`nProbing Go daemon health endpoint:"
$status = Invoke-RestMethod -Uri "http://127.0.0.1:8765/api/status"
Write-Host "Daemon running:" $status.daemon_running
Write-Host "Daemon version:" $status.version
if ($status.daemon_running -ne $true) {
    throw "Daemon failed to report daemon_running: true"
}

# 7. Check config and credential paths
$configDir = "$env:APPDATA\antigravity-swiss"
$geminiDir = "$env:USERPROFILE\.gemini"
Write-Host "`nVerifying config and token paths:"
Write-Host "Config dir ($configDir):" (Test-Path $configDir)
Write-Host "Gemini dir ($geminiDir):" (Test-Path $geminiDir)
```

### 4.3 Screenshot Capture & VM Shutdown
```bash
# Capture verification screenshot on host
python3 /home/david/VMs/bin/vmctl.py \
  /home/david/VMs/windows-11-English-United-States/windows-11-English-United-States-monitor.socket \
  shot /tmp/win11-packaging-verification.ppm

# Clean guest shutdown
# Inside guest: Stop-Computer -Force
# Or via QEMU monitor: echo "system_powerdown" | nc -U /home/david/VMs/windows-11-English-United-States/windows-11-English-United-States-monitor.socket
```

---

## 5. macOS Ventura Guest Test Execution

### 5.1 Launch VM
```bash
# Verify Windows VM has completely shut down first
quickemu --vm /home/david/VMs/macos-ventura.conf
```

### 5.2 Guest Verification Script (Bash)
Execute in macOS Terminal or via SSH (`ssh -p 22220 david@localhost`):

```bash
set -e
BASE="http://10.0.2.2:8010"

# 1. Download and test standalone binary
echo "Downloading standalone binary..."
curl -fSL -o /tmp/swiss-mac "$BASE/swiss-mac"
chmod +x /tmp/swiss-mac

echo "Testing standalone Go binary:"
/tmp/swiss-mac version
/tmp/swiss-mac --help

# 2. Download and unpack application bundle
echo "Downloading application bundle zip..."
curl -fSL -o /tmp/ASK-mac-x64.zip "$BASE/ASK-mac-x64.zip"

echo "Installing to /Applications/..."
rm -rf "/Applications/Antigravity Swiss Knife.app"
unzip -q /tmp/ASK-mac-x64.zip -d /Applications/
xattr -dr com.apple.quarantine "/Applications/Antigravity Swiss Knife.app" 2>/dev/null || true

# 3. Verify bundle structure and permissions
APP="/Applications/Antigravity Swiss Knife.app"
test -f "$APP/Contents/MacOS/Antigravity Swiss Knife"
test -x "$APP/Contents/Resources/bin/swiss"
echo "✓ Application bundle structure and executable permissions verified."

# 4. Launch application
echo "Launching Antigravity Swiss Knife.app..."
open "$APP"
sleep 4

# 5. Verify sidecar daemon health
echo "Probing Go daemon health endpoint:"
STATUS=$(curl -s http://127.0.0.1:8765/api/status)
echo "Status response: $STATUS"
echo "$STATUS" | grep -q '"daemon_running":true'
echo "✓ Sidecar daemon is active and responding."

# 6. Check config directory
CONFIG_DIR="$HOME/Library/Application Support/antigravity-swiss"
echo "Config directory: $CONFIG_DIR"
ls -la "$CONFIG_DIR" 2>/dev/null || echo "Config directory will initialize on first daemon write."
```

### 5.3 Screenshot Capture & VM Shutdown
```bash
# Capture verification screenshot on host
python3 /home/david/VMs/bin/vmctl.py \
  /home/david/VMs/macos-ventura/macos-ventura-monitor.socket \
  shot /tmp/macos-packaging-verification.ppm

# Clean guest shutdown
# Inside guest: sudo shutdown -h now
# Or via QEMU monitor: echo "system_powerdown" | nc -U /home/david/VMs/macos-ventura/macos-ventura-monitor.socket
```

---

## 6. Automated Headless Packaging Verification

When full interactive VMs are not needed or during CI runs, run the automated verification script:

```bash
npm run verify:packaging
```

This runs `scripts/verify-crossplatform-packaging.js`, executing 6 automated checks:

| Phase | Check Description | Technical Validation |
|---|---|---|
| **Phase 1** | Cross-Compilation & Binary Headers | Cross-compiles Go binaries if missing; parses PE headers (MZ DOS, PE\0\0 signature, Machine `0x8664`, PE32+ optional header `0x020B`); parses Mach-O headers (magic `0xFEEDFACF`, CPU x86_64 `0x01000007`, CPU arm64 `0x0100000C`). |
| **Phase 2** | Windows Package & Unpacked Tree | Validates `package.json` NSIS settings (`oneClick: true`, `perMachine: false`, `extraResources` sidecar mapping); validates `win-unpacked/resources/bin/swiss.exe` size (>10 MB) and PE32+ header. |
| **Phase 3** | macOS Package & App Bundle | Validates `package.json` DMG configuration and sidecar mapping; validates `Antigravity Swiss Knife.app/Contents/Resources/bin/swiss` presence, executable permissions (`chmod 0755`), size (>10 MB), and Mach-O header. |
| **Phase 4** | Wine Emulation Smoke Test | Detects `/usr/bin/wine`; executes `wine bin/swiss.exe version` (asserts exit code 0, checks output string); executes `wine bin/swiss.exe --help` (asserts exit code 0). |
| **Phase 5** | DaemonManager Path Resolution | Simulates `electron/daemon-manager.js` resolution logic across `win32`, `darwin`, and `linux`; asserts socket unlink and chmod are skipped on Windows; asserts working directory binds to `app.getPath('userData')`. |
| **Phase 6** | Cross-Platform Path Constants | Asserts Windows uses `%APPDATA%\antigravity-swiss` and `%LOCALAPPDATA%\Programs\Antigravity`; asserts macOS uses `~/Library/Application Support/antigravity-swiss` and `/Applications/Antigravity.app`. |

Exits with code `0` when all checks pass.
