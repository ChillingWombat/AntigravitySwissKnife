#!/usr/bin/env node

/**
 * ==============================================================================
 * Antigravity Swiss Knife - Cross-Platform Release Builder & Packager
 * ==============================================================================
 * Automates compiling, cross-compiling, and packaging production-ready
 * standalone releases for Linux, Windows, and macOS into the gitignored
 * release/ directory.
 *
 * Supported commands:
 *   node scripts/build-release.js linux    # Package Linux release
 *   node scripts/build-release.js windows  # Package Windows release
 *   node scripts/build-release.js macos    # Package macOS release
 *   node scripts/build-release.js all      # Package releases for all 3 OS
 *   node scripts/build-release.js run      # Launch the local prod release
 * ==============================================================================
 */

const { execSync, spawn } = require('child_process');
const path = require('path');
const fs = require('fs');

const rootDir = path.resolve(__dirname, '..');
const releaseDir = path.join(rootDir, 'release');
const binDir = path.join(rootDir, 'bin');

function log(msg) {
  console.log(`[Release] ${msg}`);
}

function ensureDir(dir) {
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }
}

function run(cmd, cwd = rootDir, env = {}) {
  log(`Executing: ${cmd}`);
  execSync(cmd, {
    cwd,
    stdio: 'inherit',
    env: { ...process.env, ...env }
  });
}

function buildFrontend() {
  log('Building latest production frontend bundle...');
  run('npm run build:frontend', rootDir);
}

function buildGoBinary(goos, goarch, outputPath) {
  log(`Cross-compiling Go binary for ${goos}/${goarch} -> ${path.relative(rootDir, outputPath)}`);
  ensureDir(path.dirname(outputPath));
  run(
    `go build -ldflags="-s -w" -o "${outputPath}" ./cmd/swiss`,
    rootDir,
    { CGO_ENABLED: '0', GOOS: goos, GOARCH: goarch }
  );
  if (goos !== 'windows') {
    try { fs.chmodSync(outputPath, 0o755); } catch {}
  }
}

function createSymlinkOrCopy(src, dest) {
  try {
    if (fs.existsSync(dest)) {
      try { fs.unlinkSync(dest); } catch {}
    }
    fs.symlinkSync(src, dest, 'dir');
  } catch {
    // If symlink fails, leave as is
  }
}

function buildLinux() {
  log('====================================================');
  log('Building Linux Production Release (amd64)');
  log('====================================================');
  buildFrontend();

  const linuxReleaseDir = path.join(releaseDir, 'linux');
  ensureDir(linuxReleaseDir);

  // 1. Build Linux Go binary in bin/ and release/linux/
  const goBin = path.join(binDir, 'swiss');
  const releaseGoBin = path.join(linuxReleaseDir, 'swiss');
  buildGoBinary('linux', 'amd64', goBin);
  fs.copyFileSync(goBin, releaseGoBin);
  fs.chmodSync(releaseGoBin, 0o755);

  // 2. Package Electron App
  log('Packaging Electron application for Linux...');
  run(`npx electron-builder --linux --dir -c.directories.output="${linuxReleaseDir}"`, rootDir);

  const unpackedDir = path.join(linuxReleaseDir, 'linux-unpacked');
  const appSymlink = path.join(linuxReleaseDir, 'app');
  createSymlinkOrCopy('linux-unpacked', appSymlink);

  // Ensure packaged daemon binary is executable
  const packagedSwiss = path.join(unpackedDir, 'resources', 'bin', 'swiss');
  if (fs.existsSync(packagedSwiss)) {
    fs.chmodSync(packagedSwiss, 0o755);
  }

  // 3. Generate launcher script
  const runScript = path.join(linuxReleaseDir, 'run.sh');
  const runScriptContent = `#!/usr/bin/env bash
set -e
DIR="$(cd "$(dirname "\${BASH_SOURCE[0]}")" && pwd)"
APP="\${DIR}/linux-unpacked/antigravity-swiss-knife"
if [ ! -f "\${APP}" ]; then
  APP="\${DIR}/app/antigravity-swiss-knife"
fi

if [[ "$*" == *"--detached"* ]] || [[ "$*" == *"-d"* ]]; then
  ARGS=()
  for arg in "$@"; do
    if [ "$arg" != "--detached" ] && [ "$arg" != "-d" ]; then
      ARGS+=("$arg")
    fi
  done
  echo "[Production Release] Launching Antigravity Swiss Knife in background: \${APP}"
  if command -v setsid >/dev/null 2>&1; then
    setsid "\${APP}" "\${ARGS[@]}" </dev/null >/dev/null 2>&1 &
    PID=$!
    disown "$PID" 2>/dev/null || true
  else
    nohup "\${APP}" "\${ARGS[@]}" </dev/null >/dev/null 2>&1 &
    PID=$!
    disown "$PID" 2>/dev/null || true
  fi
  echo "✓ Launched in background (PID: $PID)."
else
  echo "[Production Release] Launching Antigravity Swiss Knife from: \${APP}"
  exec "\${APP}" "$@"
fi
`;
  fs.writeFileSync(runScript, runScriptContent, { mode: 0o755 });

  log('✓ Linux release build complete at: release/linux/');
}

function buildWindows() {
  log('====================================================');
  log('Building Windows Production Release (amd64)');
  log('====================================================');
  buildFrontend();

  const winReleaseDir = path.join(releaseDir, 'windows');
  ensureDir(winReleaseDir);

  // 1. Cross-compile Windows Go binary
  const goBinWin = path.join(binDir, 'swiss.exe');
  const releaseGoBinWin = path.join(winReleaseDir, 'swiss.exe');
  buildGoBinary('windows', 'amd64', goBinWin);
  fs.copyFileSync(goBinWin, releaseGoBinWin);

  // Also keep a copy of bin/swiss for Linux host sanity
  const fallbackSwiss = path.join(binDir, 'swiss');
  if (!fs.existsSync(fallbackSwiss)) {
    buildGoBinary('linux', 'amd64', fallbackSwiss);
  }

  // 2. Package Electron App for Windows
  log('Packaging Electron application for Windows...');
  run(`npx electron-builder --win --dir -c.directories.output="${winReleaseDir}"`, rootDir);

  const winUnpackedDir = path.join(winReleaseDir, 'win-unpacked');
  const appSymlink = path.join(winReleaseDir, 'app');
  createSymlinkOrCopy('win-unpacked', appSymlink);

  // 3. Ensure Windows daemon binary exists in packaged resources/bin/swiss.exe
  const packagedWinBinDir = path.join(winUnpackedDir, 'resources', 'bin');
  ensureDir(packagedWinBinDir);
  fs.copyFileSync(goBinWin, path.join(packagedWinBinDir, 'swiss.exe'));

  // 4. Generate launcher batch file
  const runBat = path.join(winReleaseDir, 'run.bat');
  const runBatContent = `@echo off
set DIR=%~dp0
if exist "%DIR%win-unpacked\\Antigravity Swiss Knife.exe" (
  start "" "%DIR%win-unpacked\\Antigravity Swiss Knife.exe" %*
) else (
  start "" "%DIR%app\\Antigravity Swiss Knife.exe" %*
)
`;
  fs.writeFileSync(runBat, runBatContent);

  log('✓ Windows release build complete at: release/windows/');
}

function buildMacOS() {
  log('====================================================');
  log('Building macOS Production Release (arm64 & x64)');
  log('====================================================');
  buildFrontend();

  const macReleaseDir = path.join(releaseDir, 'macos');
  ensureDir(macReleaseDir);

  // 1. Cross-compile macOS Go binaries
  const releaseGoArm = path.join(macReleaseDir, 'swiss-darwin-arm64');
  const releaseGoIntel = path.join(macReleaseDir, 'swiss-darwin-amd64');
  buildGoBinary('darwin', 'arm64', releaseGoArm);
  buildGoBinary('darwin', 'amd64', releaseGoIntel);

  // Prepare bin/swiss for packaging
  const goBinMac = path.join(binDir, 'swiss');
  buildGoBinary('darwin', 'arm64', goBinMac);

  // 2. Package Electron App for macOS (both architectures)
  log('Packaging Electron application for macOS (arm64 & x64)...');
  run(`npx electron-builder --mac --x64 --arm64 --dir -c.directories.output="${macReleaseDir}"`, rootDir);

  const macIntelDir = path.join(macReleaseDir, 'mac');
  const macArmDir = path.join(macReleaseDir, 'mac-arm64');
  const appSymlink = path.join(macReleaseDir, 'app');

  if (fs.existsSync(macArmDir)) {
    createSymlinkOrCopy('mac-arm64', appSymlink);
  } else {
    createSymlinkOrCopy('mac', appSymlink);
  }

  // Ensure arm64 Go binary inside mac-arm64 app
  const armAppBin = path.join(
    macArmDir,
    'Antigravity Swiss Knife.app',
    'Contents',
    'Resources',
    'bin',
    'swiss'
  );
  if (fs.existsSync(path.dirname(armAppBin))) {
    fs.copyFileSync(releaseGoArm, armAppBin);
    fs.chmodSync(armAppBin, 0o755);
  }

  // Ensure intel Go binary inside mac x64 app
  const intelAppBin = path.join(
    macIntelDir,
    'Antigravity Swiss Knife.app',
    'Contents',
    'Resources',
    'bin',
    'swiss'
  );
  if (fs.existsSync(path.dirname(intelAppBin))) {
    fs.copyFileSync(releaseGoIntel, intelAppBin);
    fs.chmodSync(intelAppBin, 0o755);
  }

  // 3. Generate launcher script
  const runScript = path.join(macReleaseDir, 'run.sh');
  const runScriptContent = `#!/usr/bin/env bash
set -e
DIR="$(cd "$(dirname "\${BASH_SOURCE[0]}")" && pwd)"
ARCH="$(uname -m)"

if [ "\${ARCH}" = "arm64" ] && [ -d "\${DIR}/mac-arm64/Antigravity Swiss Knife.app" ]; then
  APP="\${DIR}/mac-arm64/Antigravity Swiss Knife.app"
elif [ -d "\${DIR}/mac/Antigravity Swiss Knife.app" ]; then
  APP="\${DIR}/mac/Antigravity Swiss Knife.app"
elif [ -d "\${DIR}/app/Antigravity Swiss Knife.app" ]; then
  APP="\${DIR}/app/Antigravity Swiss Knife.app"
else
  APP="\${DIR}/mac-arm64/Antigravity Swiss Knife.app"
fi

echo "[Production Release] Launching Antigravity Swiss Knife macOS app (\${ARCH}): \${APP}"
open "\${APP}" --args "$@"
`;
  fs.writeFileSync(runScript, runScriptContent, { mode: 0o755 });

  log('✓ macOS release build complete at: release/macos/');
}

function runProdApp() {
  log('Launching production release application...');
  const platform = process.platform;

  if (platform === 'linux') {
    const linuxApp = path.join(releaseDir, 'linux', 'linux-unpacked', 'antigravity-swiss-knife');
    if (!fs.existsSync(linuxApp)) {
      log('Linux release not built yet. Building Linux release first...');
      buildLinux();
    }
    log(`Spawning detached process: ${linuxApp}`);
    const child = spawn(linuxApp, [], {
      detached: true,
      stdio: 'ignore'
    });
    child.unref();
    log(`✓ Production release application started (PID: ${child.pid}).`);
  } else if (platform === 'darwin') {
    const macArmApp = path.join(releaseDir, 'macos', 'mac-arm64', 'Antigravity Swiss Knife.app');
    const macIntelApp = path.join(releaseDir, 'macos', 'mac', 'Antigravity Swiss Knife.app');
    let macApp = process.arch === 'arm64' ? macArmApp : macIntelApp;
    if (!fs.existsSync(macApp)) {
      macApp = fs.existsSync(macArmApp) ? macArmApp : macIntelApp;
    }
    if (!fs.existsSync(macApp)) {
      log('macOS release not built yet. Building macOS release first...');
      buildMacOS();
      macApp = process.arch === 'arm64' ? macArmApp : macIntelApp;
    }
    log(`Launching macOS application: ${macApp}`);
    const child = spawn('open', [macApp], {
      detached: true,
      stdio: 'ignore'
    });
    child.unref();
    log(`✓ Production release application started via open.`);
  } else if (platform === 'win32') {
    const winApp = path.join(releaseDir, 'windows', 'win-unpacked', 'Antigravity Swiss Knife.exe');
    if (!fs.existsSync(winApp)) {
      log('Windows release not built yet. Building Windows release first...');
      buildWindows();
    }
    log(`Spawning detached process: ${winApp}`);
    const child = spawn(winApp, [], {
      detached: true,
      stdio: 'ignore'
    });
    child.unref();
    log(`✓ Production release application started (PID: ${child.pid}).`);
  } else {
    console.error(`Unsupported platform for running production release: ${platform}`);
    process.exit(1);
  }

  log('You can now switch Git branches or let agents work without interrupting this app.');
}

const target = (process.argv[2] || 'linux').toLowerCase();

switch (target) {
  case 'linux':
    buildLinux();
    break;
  case 'win':
  case 'windows':
    buildWindows();
    break;
  case 'mac':
  case 'macos':
  case 'darwin':
    buildMacOS();
    break;
  case 'all':
    buildLinux();
    buildWindows();
    buildMacOS();
    log('====================================================');
    log('✓ All 3 operating systems packaged in release/');
    log('====================================================');
    break;
  case 'run':
    runProdApp();
    break;
  default:
    console.error(`Unknown release target: ${target}`);
    console.error('Supported targets: linux, windows, macos, all, run');
    process.exit(1);
}
