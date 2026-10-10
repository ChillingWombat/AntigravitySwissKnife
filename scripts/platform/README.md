# Platform-Specific Codes and Scripts

This directory organizes dedicated OS-specific scripts for Windows, macOS, and Linux:

```
scripts/platform/
├── linux/
│   ├── run.sh             # Launch packaged Linux AppImage or unpacked binary
│   └── verify-guest.sh    # Verify Linux packages (.deb, AppImage), binary, daemon, and paths
├── macos/
│   ├── run.sh             # Launch macOS .app bundle
│   └── verify-guest.sh    # macOS guest verification script (downloads, installs, probes daemon)
└── windows/
    ├── run.bat            # Launch Windows executable or unpacked bundle
    └── verify-guest.ps1   # Windows guest verification script (PowerShell for Win 11 VM)
```

## QuickEMU Guest Execution

The guest scripts (`verify-guest.ps1` and `verify-guest.sh`) are staged directly to `/home/david/VMs/transfer/` as:
- `/home/david/VMs/transfer/win-verify-ask.ps1`
- `/home/david/VMs/transfer/mac-verify-ask.sh`

### In Windows 11 VM (`windows-11-English-United-States`):
Run in PowerShell or MSYS2 zsh:
```powershell
Invoke-WebRequest "http://10.0.2.2:8010/win-verify-ask.ps1" -OutFile "$env:TEMP\win-verify-ask.ps1"
powershell.exe -ExecutionPolicy Bypass -File "$env:TEMP\win-verify-ask.ps1"
```

### In macOS Ventura VM (`macos-ventura`):
Run in Terminal.app or SSH (`ssh -p 22220 david@localhost`):
```bash
curl -fsSL http://10.0.2.2:8010/mac-verify-ask.sh | bash
```
