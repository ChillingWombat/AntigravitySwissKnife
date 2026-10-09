#!/usr/bin/env node

/**
 * ==============================================================================
 * Antigravity Swiss Knife - Cross-Platform Release Builder & Packager
 * ==============================================================================
 * Automates compiling, cross-compiling, and packaging production-ready
 * standalone releases for Linux, Windows, and macOS into the gitignored
 * release/ directory.
 *
 * Defaults to standard OS installer artifacts (Model 1):
 *   - Linux: Debian package (.deb) installing into /opt/Antigravity Swiss Knife
 *   - Windows: NSIS installer (.exe) installing into %LOCALAPPDATA%\Programs\Antigravity Swiss Knife
 *   - macOS: Apple DMG (.dmg) installing into /Applications
 *
 * Developer options (--unpacked, --portable) allow fast local testing without installer generation.
 * ==============================================================================
 */

const { execSync, spawn } = require('child_process');
const path = require('path');
const fs = require('fs');
const os = require('os');

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
  log('Building production frontend bundle...');
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
    if (fs.existsSync(dest) || fs.lstatSync(dest).isSymbolicLink()) {
      try { fs.unlinkSync(dest); } catch {}
    }
    fs.symlinkSync(src, dest, 'dir');
  } catch {
    // Leave as is if symlink cannot be created
  }
}

function cleanStaleLinks() {
  const rootSymlinks = [
    path.join(rootDir, 'Antigravity-Swiss-Knife.AppImage'),
    path.join(rootDir, 'antigravity-swiss-knife')
  ];
  for (const symlink of rootSymlinks) {
    try {
      if (fs.existsSync(symlink) || fs.lstatSync(symlink).isSymbolicLink()) {
        fs.unlinkSync(symlink);
      }
    } catch {}
  }

  const linuxReleaseDir = path.join(releaseDir, 'linux');
  if (fs.existsSync(linuxReleaseDir)) {
    const staleLinuxLinks = [
      path.join(linuxReleaseDir, 'antigravity-swiss-knife'),
      path.join(linuxReleaseDir, 'Antigravity-Swiss-Knife.AppImage')
    ];
    for (const link of staleLinuxLinks) {
      try {
        if (fs.existsSync(link) || fs.lstatSync(link).isSymbolicLink()) {
          fs.unlinkSync(link);
        }
      } catch {}
    }
  }
}

function buildLinux(options = {}) {
  const modeLabel = options.unpacked
    ? 'unpacked directory'
    : options.portable
      ? 'portable AppImage'
      : 'Debian package (.deb)';
  log('====================================================');
  log(`Building Linux Release (amd64) [${modeLabel}]`);
  log('====================================================');
  cleanStaleLinks();
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
  let builderArgs = '--linux deb';
  if (options.unpacked) {
    builderArgs = '--linux --dir';
  } else if (options.portable) {
    builderArgs = '--linux AppImage';
  }
  run(`npx electron-builder ${builderArgs} -c.directories.output="${linuxReleaseDir}"`, rootDir);

  const unpackedDir = path.join(linuxReleaseDir, 'linux-unpacked');
  if (fs.existsSync(unpackedDir)) {
    const appSymlink = path.join(linuxReleaseDir, 'app');
    createSymlinkOrCopy('linux-unpacked', appSymlink);

    const packagedSwiss = path.join(unpackedDir, 'resources', 'bin', 'swiss');
    if (fs.existsSync(packagedSwiss)) {
      fs.chmodSync(packagedSwiss, 0o755);
    }
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
if [ ! -f "\${APP}" ] && [ -f "/opt/Antigravity Swiss Knife/antigravity-swiss-knife" ]; then
  APP="/opt/Antigravity Swiss Knife/antigravity-swiss-knife"
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

function buildWindows(options = {}) {
  const modeLabel = options.unpacked
    ? 'unpacked directory'
    : options.portable
      ? 'portable executable'
      : 'NSIS Setup.exe installer';
  log('====================================================');
  log(`Building Windows Release (amd64) [${modeLabel}]`);
  log('====================================================');
  buildFrontend();

  const winReleaseDir = path.join(releaseDir, 'windows');
  ensureDir(winReleaseDir);

  // 1. Cross-compile Windows Go binary
  const goBinWin = path.join(binDir, 'swiss.exe');
  const releaseGoBinWin = path.join(winReleaseDir, 'swiss.exe');
  buildGoBinary('windows', 'amd64', goBinWin);
  fs.copyFileSync(goBinWin, releaseGoBinWin);

  // Keep a copy of bin/swiss for Linux host sanity
  const fallbackSwiss = path.join(binDir, 'swiss');
  if (!fs.existsSync(fallbackSwiss)) {
    buildGoBinary('linux', 'amd64', fallbackSwiss);
  }

  // 2. Package Electron App for Windows
  log('Packaging Electron application for Windows...');
  let builderArgs = '--win nsis';
  if (options.unpacked) {
    builderArgs = '--win --dir';
  } else if (options.portable) {
    builderArgs = '--win portable';
  }
  run(`npx electron-builder ${builderArgs} -c.directories.output="${winReleaseDir}"`, rootDir);

  const winUnpackedDir = path.join(winReleaseDir, 'win-unpacked');
  if (fs.existsSync(winUnpackedDir)) {
    const appSymlink = path.join(winReleaseDir, 'app');
    createSymlinkOrCopy('win-unpacked', appSymlink);

    const packagedWinBinDir = path.join(winUnpackedDir, 'resources', 'bin');
    ensureDir(packagedWinBinDir);
    fs.copyFileSync(goBinWin, path.join(packagedWinBinDir, 'swiss.exe'));
  }

  // 3. Generate launcher batch file
  const runBat = path.join(winReleaseDir, 'run.bat');
  const runBatContent = `@echo off
set DIR=%~dp0
if exist "%DIR%win-unpacked\\Antigravity Swiss Knife.exe" (
  start "" "%DIR%win-unpacked\\Antigravity Swiss Knife.exe" %*
) else if exist "%DIR%app\\Antigravity Swiss Knife.exe" (
  start "" "%DIR%app\\Antigravity Swiss Knife.exe" %*
) else (
  start "" "%LOCALAPPDATA%\\Programs\\Antigravity Swiss Knife\\Antigravity Swiss Knife.exe" %*
)
`;
  fs.writeFileSync(runBat, runBatContent);

  log('✓ Windows release build complete at: release/windows/');
}

function buildMacOS(options = {}) {
  const modeLabel = options.unpacked
    ? 'unpacked directory'
    : options.portable
      ? 'portable zip'
      : 'Apple DMG installer';
  log('====================================================');
  log(`Building macOS Release (arm64 & x64) [${modeLabel}]`);
  log('====================================================');
  buildFrontend();

  const macReleaseDir = path.join(releaseDir, 'macos');
  ensureDir(macReleaseDir);

  // 1. Cross-compile macOS Go binaries
  const releaseGoArm = path.join(macReleaseDir, 'swiss-darwin-arm64');
  const releaseGoIntel = path.join(macReleaseDir, 'swiss-darwin-amd64');
  buildGoBinary('darwin', 'arm64', releaseGoArm);
  buildGoBinary('darwin', 'amd64', releaseGoIntel);

  const goBinMac = path.join(binDir, 'swiss');
  buildGoBinary('darwin', 'arm64', goBinMac);

  // 2. Package Electron App for macOS
  log('Packaging Electron application for macOS (arm64 & x64)...');
  let builderArgs = '--mac dmg --x64 --arm64';
  if (options.unpacked) {
    builderArgs = '--mac --x64 --arm64 --dir';
  } else if (options.portable) {
    builderArgs = '--mac zip --x64 --arm64';
  }
  run(`npx electron-builder ${builderArgs} -c.directories.output="${macReleaseDir}"`, rootDir);

  const macIntelDir = path.join(macReleaseDir, 'mac');
  const macArmDir = path.join(macReleaseDir, 'mac-arm64');
  const appSymlink = path.join(macReleaseDir, 'app');

  if (fs.existsSync(macArmDir)) {
    createSymlinkOrCopy('mac-arm64', appSymlink);
  } else if (fs.existsSync(macIntelDir)) {
    createSymlinkOrCopy('mac', appSymlink);
  }

  // Ensure binaries inside app bundles
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
elif [ -d "/Applications/Antigravity Swiss Knife.app" ]; then
  APP="/Applications/Antigravity Swiss Knife.app"
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
    const installedApp = '/opt/Antigravity Swiss Knife/antigravity-swiss-knife';
    const userApp = path.join(os.homedir(), '.local', 'share', 'antigravity-swiss-knife', 'app', 'antigravity-swiss-knife');
    const unpackedApp = path.join(releaseDir, 'linux', 'linux-unpacked', 'antigravity-swiss-knife');

    let linuxApp = null;
    if (fs.existsSync(installedApp)) {
      linuxApp = installedApp;
    } else if (fs.existsSync(userApp)) {
      linuxApp = userApp;
    } else if (fs.existsSync(unpackedApp)) {
      linuxApp = unpackedApp;
    } else {
      log('No installed or unpacked Linux build found. Building unpacked bundle first...');
      buildLinux({ unpacked: true });
      linuxApp = unpackedApp;
    }

    log(`Spawning detached process: ${linuxApp}`);
    const child = spawn(linuxApp, [], {
      detached: true,
      stdio: 'ignore'
    });
    child.unref();
    log(`✓ Production release application started (PID: ${child.pid}).`);
  } else if (platform === 'darwin') {
    const installedApp = '/Applications/Antigravity Swiss Knife.app';
    const macArmApp = path.join(releaseDir, 'macos', 'mac-arm64', 'Antigravity Swiss Knife.app');
    const macIntelApp = path.join(releaseDir, 'macos', 'mac', 'Antigravity Swiss Knife.app');

    let macApp = null;
    if (fs.existsSync(installedApp)) {
      macApp = installedApp;
    } else if (process.arch === 'arm64' && fs.existsSync(macArmApp)) {
      macApp = macArmApp;
    } else if (fs.existsSync(macIntelApp)) {
      macApp = macIntelApp;
    } else {
      log('macOS release not built yet. Building unpacked bundle first...');
      buildMacOS({ unpacked: true });
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
    const installedApp = path.join(
      process.env.LOCALAPPDATA || path.join(os.homedir(), 'AppData', 'Local'),
      'Programs',
      'Antigravity Swiss Knife',
      'Antigravity Swiss Knife.exe'
    );
    const winUnpacked = path.join(releaseDir, 'windows', 'win-unpacked', 'Antigravity Swiss Knife.exe');

    let winApp = null;
    if (fs.existsSync(installedApp)) {
      winApp = installedApp;
    } else if (fs.existsSync(winUnpacked)) {
      winApp = winUnpacked;
    } else {
      log('Windows release not built yet. Building unpacked bundle first...');
      buildWindows({ unpacked: true });
      winApp = winUnpacked;
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

  log('Application running in background.');
}

function printHelp() {
  console.log(`
Antigravity Swiss Knife - Cross-Platform Release Builder & Packager

Usage:
  node scripts/build-release.js [target] [options]

Targets:
  linux             Build Linux release (default: Debian .deb package)
  windows, win      Build Windows release (default: NSIS Setup.exe installer)
  macos, mac        Build macOS release (default: Apple DMG installer)
  all               Build standard releases for all 3 operating systems
  run               Launch the production release application
  install-linux     Install the Linux release (system-wide or user-level)

Options:
  --unpacked, --dir Build unpacked directory bundle for local development/testing
  --portable        Build standalone portable bundle (AppImage / portable exe / zip)
  --help, -h        Show this help message

Default Target Artifacts (Model 1 Standard Installers):
  Linux:   Debian package (.deb) -> installs into /opt/Antigravity Swiss Knife
  Windows: NSIS installer (.exe) -> installs into %LOCALAPPDATA%\\Programs\\Antigravity Swiss Knife
  macOS:   Apple disk image (.dmg) -> installs into /Applications
`);
}

function parseArgs(args) {
  const options = {
    target: 'linux',
    unpacked: false,
    portable: false,
    help: false
  };

  const positional = [];

  for (const arg of args) {
    if (arg === '--help' || arg === '-h' || arg === 'help') {
      options.help = true;
    } else if (arg === '--unpacked' || arg === '--dir') {
      options.unpacked = true;
    } else if (arg === '--portable') {
      options.portable = true;
    } else if (arg.startsWith('-')) {
      console.warn(`[Release] Warning: unrecognized flag ${arg}`);
    } else {
      positional.push(arg.toLowerCase());
    }
  }

  if (positional.length > 0) {
    options.target = positional[0];
  }

  return options;
}

const rawArgs = process.argv.slice(2);
const options = parseArgs(rawArgs);

if (options.help) {
  printHelp();
  process.exit(0);
}

switch (options.target) {
  case 'linux':
    buildLinux(options);
    break;
  case 'win':
  case 'windows':
    buildWindows(options);
    break;
  case 'mac':
  case 'macos':
  case 'darwin':
    buildMacOS(options);
    break;
  case 'all':
    buildLinux(options);
    buildWindows(options);
    buildMacOS(options);
    log('====================================================');
    log('✓ All 3 operating systems packaged in release/');
    log('====================================================');
    break;
  case 'run':
    runProdApp();
    break;
  case 'install-linux':
  case 'install:linux': {
    const installerScript = path.join(rootDir, 'scripts', 'install-linux.js');
    if (fs.existsSync(installerScript)) {
      run(`node "${installerScript}"`, rootDir);
    } else {
      log('scripts/install-linux.js not yet present. Run: npm run build:linux');
    }
    break;
  }
  default:
    console.error(`Unknown release target: ${options.target}`);
    printHelp();
    process.exit(1);
}
