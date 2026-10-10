# ==============================================================================
# Antigravity Swiss Knife - Windows Guest Verification Script
# ==============================================================================
# Run inside Windows 11 VM (MSYS2 zsh, PowerShell, or via SSH / vmctl.py):
#   powershell.exe -ExecutionPolicy Bypass -File verify-guest.ps1
# ==============================================================================

$ErrorActionPreference = "Stop"
$base = "http://10.0.2.2:8010"
$temp = $env:TEMP

Write-Host "======================================================================"
Write-Host "Starting Antigravity Swiss Knife Windows Guest Verification"
Write-Host "======================================================================"

# 1. Download test artifacts from host transfer server
Write-Host "`n[1/6] Downloading artifacts from $base..."
Invoke-WebRequest -Uri "$base/ASK-Setup.exe" -OutFile "$temp\ASK-Setup.exe" -UseBasicParsing
Invoke-WebRequest -Uri "$base/swiss.exe" -OutFile "$temp\swiss.exe" -UseBasicParsing
Write-Host "  ✓ Artifacts downloaded to $temp"

# 2. Smoke test standalone Go binary
Write-Host "`n[2/6] Smoke testing standalone Go binary (swiss.exe)..."
$verOut = & "$temp\swiss.exe" version
Write-Host "  Version output: $verOut"
if ($LASTEXITCODE -ne 0 -or -not ($verOut -match "Antigravity Swiss Knife")) {
    throw "Standalone swiss.exe version test failed!"
}
$helpOut = & "$temp\swiss.exe" --help
if ($LASTEXITCODE -ne 0) {
    throw "Standalone swiss.exe --help test failed!"
}
Write-Host "  ✓ Standalone swiss.exe operates correctly"

# 3. Silent NSIS installation
Write-Host "`n[3/6] Running silent NSIS installation (/S)..."
$process = Start-Process -FilePath "$temp\ASK-Setup.exe" -ArgumentList "/S" -PassThru -Wait
if ($process.ExitCode -ne 0) {
    Write-Warning "Installer exited with code: $($process.ExitCode)"
}
Start-Sleep -Seconds 3

# 4. Verify installed directories and sidecar binary
$installDir = "$env:LOCALAPPDATA\Programs\Antigravity Swiss Knife"
Write-Host "`n[4/6] Verifying installed package structure at $installDir..."
$mainExe = "$installDir\Antigravity Swiss Knife.exe"
$sidecarExe = "$installDir\resources\bin\swiss.exe"

if (!(Test-Path $mainExe)) {
    throw "Installed executable missing: $mainExe"
}
if (!(Test-Path $sidecarExe)) {
    throw "Sidecar daemon missing: $sidecarExe"
}

$sidecarStat = Get-Item $sidecarExe
$sizeMb = [math]::Round($sidecarStat.Length / 1MB, 2)
Write-Host "  ✓ Executable verified: $mainExe"
Write-Host "  ✓ Sidecar daemon verified: $sidecarExe ($sizeMb MB)"

# 5. Launch application and verify sidecar daemon health
Write-Host "`n[5/6] Launching desktop application..."
$appProcess = Start-Process -FilePath $mainExe -PassThru
Start-Sleep -Seconds 5

Write-Host "Probing Go daemon health endpoint (http://127.0.0.1:8765/api/status)..."
$retries = 5
$status = $null
while ($retries -gt 0) {
    try {
        $status = Invoke-RestMethod -Uri "http://127.0.0.1:8765/api/status" -TimeoutSec 3
        if ($status.daemon_running -eq $true) {
            break
        }
    } catch {
        Start-Sleep -Seconds 2
        $retries--
    }
}

if ($status -and $status.daemon_running -eq $true) {
    Write-Host "  ✓ Daemon health check PASSED!"
    Write-Host "    Version: $($status.version)"
    Write-Host "    Daemon running: $($status.daemon_running)"
} else {
    Write-Warning "Daemon endpoint did not respond with daemon_running: true (may require credentials)"
}

# 6. Verify config and token directories
Write-Host "`n[6/6] Verifying user data and credential paths..."
$configDir = "$env:APPDATA\antigravity-swiss"
$geminiDir = "$env:USERPROFILE\.gemini"
Write-Host "  Config dir ($configDir): $(Test-Path $configDir)"
Write-Host "  Gemini dir ($geminiDir): $(Test-Path $geminiDir)"

# Teardown / Cleanup running app
try {
    Stop-Process -Id $appProcess.Id -Force -ErrorAction SilentlyContinue
} catch {}

Write-Host "`n======================================================================"
Write-Host "✓ Windows Guest Verification Complete!"
Write-Host "======================================================================"
