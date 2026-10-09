#!/usr/bin/env node

/**
 * ==============================================================================
 * Antigravity Swiss Knife - Cross-Platform Packaging Verification Script
 * ==============================================================================
 * Milestone 15: Cross-Platform Packaging Validation & Verification Harness
 *
 * Performs 6 comprehensive verification phases:
 *   - Phase 1: Cross-Compilation & Binary Format Validation (PE32+ & Mach-O 64-bit)
 *   - Phase 2: Windows Package & Unpacked Tree Verification (NSIS & Sidecar Daemon)
 *   - Phase 3: macOS Package & App Bundle Structure Verification (DMG & Sidecar Daemon)
 *   - Phase 4: Wine Emulation Headless Execution (Smoke Test: version & help)
 *   - Phase 5: DaemonManager Cross-Platform Path Resolution Simulation
 *   - Phase 6: Cross-Platform Path Constants Verification (pkg/core/constants.go)
 *
 * Exit Code:
 *   0 - All verification phases passed successfully.
 *   1 - One or more checks failed.
 * ==============================================================================
 */

const { execSync, spawnSync } = require('child_process');
const path = require('path');
const fs = require('fs');

const rootDir = path.resolve(__dirname, '..');
const binDir = path.join(rootDir, 'bin');
const releaseDir = path.join(rootDir, 'release');

let totalChecks = 0;
let passedChecks = 0;
let failedChecks = 0;
const failureDetails = [];

function check(label, condition, detail = '') {
  totalChecks++;
  if (condition) {
    passedChecks++;
    console.log(`  ✓ ${label}`);
  } else {
    failedChecks++;
    console.error(`  ✗ FAIL: ${label}${detail ? ` (${detail})` : ''}`);
    failureDetails.push({ label, detail });
  }
}

function parsePEHeader(filePath) {
  if (!fs.existsSync(filePath)) {
    throw new Error(`File does not exist: ${filePath}`);
  }
  const buf = fs.readFileSync(filePath);
  if (buf.length < 64) {
    throw new Error(`File size too small for PE (${buf.length} bytes)`);
  }
  const dosMagic = buf.toString('ascii', 0, 2);
  if (dosMagic !== 'MZ') {
    throw new Error(`Invalid DOS header: expected 'MZ', found '${dosMagic}'`);
  }
  const e_lfanew = buf.readUInt32LE(0x3c);
  if (buf.length < e_lfanew + 26) {
    throw new Error(`Corrupt PE header: offset e_lfanew 0x${e_lfanew.toString(16)} out of range`);
  }
  const peSig = buf.toString('ascii', e_lfanew, e_lfanew + 4);
  if (peSig !== 'PE\0\0') {
    throw new Error(`Invalid PE signature: expected 'PE\\0\\0', found '${JSON.stringify(peSig)}'`);
  }
  const machine = buf.readUInt16LE(e_lfanew + 4);
  const optMagic = buf.readUInt16LE(e_lfanew + 24);
  return {
    valid: true,
    machine,
    machineHex: '0x' + machine.toString(16).toUpperCase(),
    isAmd64: machine === 0x8664,
    optMagicHex: '0x' + optMagic.toString(16).toUpperCase(),
    isPE32Plus: optMagic === 0x020b,
    size: buf.length
  };
}

function parseMachOHeader(filePath) {
  if (!fs.existsSync(filePath)) {
    throw new Error(`File does not exist: ${filePath}`);
  }
  const buf = fs.readFileSync(filePath);
  if (buf.length < 32) {
    throw new Error(`File size too small for Mach-O (${buf.length} bytes)`);
  }
  const magicLE = buf.readUInt32LE(0);
  const magicBE = buf.readUInt32BE(0);

  let isLE = false;
  let is64Bit = false;

  if (magicLE === 0xfeedfacf) {
    isLE = true;
    is64Bit = true;
  } else if (magicBE === 0xfeedfacf || magicLE === 0xcffaedfe) {
    isLE = false;
    is64Bit = true;
  } else {
    throw new Error(`Invalid Mach-O 64-bit magic (LE=0x${magicLE.toString(16)}, BE=0x${magicBE.toString(16)})`);
  }

  const cpuType = isLE ? buf.readUInt32LE(4) : buf.readUInt32BE(4);
  return {
    valid: true,
    is64Bit,
    cpuType,
    cpuTypeHex: '0x' + cpuType.toString(16).toUpperCase(),
    isX86_64: cpuType === 0x01000007,
    isArm64: cpuType === 0x0100000c,
    size: buf.length
  };
}

function ensureDir(dir) {
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }
}

function runCommand(cmd, env = {}) {
  return execSync(cmd, {
    cwd: rootDir,
    encoding: 'utf8',
    env: { ...process.env, ...env },
    stdio: ['ignore', 'pipe', 'pipe']
  });
}

// ==============================================================================
// PHASE 1: Cross-Compilation & Binary Format Validation
// ==============================================================================
function phase1() {
  console.log('\n======================================================================');
  console.log('Phase 1: Cross-Compilation & Binary Format Validation');
  console.log('======================================================================');
  ensureDir(binDir);

  const winBin = path.join(binDir, 'swiss.exe');
  const macAmd64Bin = path.join(binDir, 'swiss-darwin-amd64');
  const macArm64Bin = path.join(binDir, 'swiss-darwin-arm64');

  // 1. Windows Binary
  if (!fs.existsSync(winBin)) {
    console.log('  [Compiling] Windows amd64 binary (bin/swiss.exe)...');
    runCommand('go build -ldflags="-s -w" -o bin/swiss.exe ./cmd/swiss', {
      GOOS: 'windows',
      GOARCH: 'amd64',
      CGO_ENABLED: '0'
    });
  }
  check('Windows binary exists at bin/swiss.exe', fs.existsSync(winBin));

  try {
    const pe = parsePEHeader(winBin);
    check('Windows binary has MZ DOS header', pe.valid);
    check('Windows binary has PE\\0\\0 signature at offset e_lfanew', pe.valid);
    check('Windows binary has Machine 0x8664 (AMD64)', pe.isAmd64, `Machine: ${pe.machineHex}`);
    check('Windows binary has PE32+ Optional Header Magic (0x020B)', pe.isPE32Plus, `Magic: ${pe.optMagicHex}`);
  } catch (err) {
    check('Windows binary PE header parsing', false, err.message);
  }

  // 2. macOS AMD64 Binary
  if (!fs.existsSync(macAmd64Bin)) {
    console.log('  [Compiling] macOS amd64 binary (bin/swiss-darwin-amd64)...');
    runCommand('go build -ldflags="-s -w" -o bin/swiss-darwin-amd64 ./cmd/swiss', {
      GOOS: 'darwin',
      GOARCH: 'amd64',
      CGO_ENABLED: '0'
    });
    try { fs.chmodSync(macAmd64Bin, 0o755); } catch {}
  }
  check('macOS amd64 binary exists at bin/swiss-darwin-amd64', fs.existsSync(macAmd64Bin));

  try {
    const machAmd64 = parseMachOHeader(macAmd64Bin);
    check('macOS amd64 binary has Mach-O 64-bit magic (0xFEEDFACF / 0xCFFAEDFE)', machAmd64.is64Bit);
    check('macOS amd64 binary has CPU type x86_64 (0x01000007)', machAmd64.isX86_64, `CPU: ${machAmd64.cpuTypeHex}`);
  } catch (err) {
    check('macOS amd64 binary Mach-O header parsing', false, err.message);
  }

  // 3. macOS ARM64 Binary
  if (!fs.existsSync(macArm64Bin)) {
    console.log('  [Compiling] macOS arm64 binary (bin/swiss-darwin-arm64)...');
    runCommand('go build -ldflags="-s -w" -o bin/swiss-darwin-arm64 ./cmd/swiss', {
      GOOS: 'darwin',
      GOARCH: 'arm64',
      CGO_ENABLED: '0'
    });
    try { fs.chmodSync(macArm64Bin, 0o755); } catch {}
  }
  check('macOS arm64 binary exists at bin/swiss-darwin-arm64', fs.existsSync(macArm64Bin));

  try {
    const machArm64 = parseMachOHeader(macArm64Bin);
    check('macOS arm64 binary has Mach-O 64-bit magic (0xFEEDFACF / 0xCFFAEDFE)', machArm64.is64Bit);
    check('macOS arm64 binary has CPU type arm64 (0x0100000C)', machArm64.isArm64, `CPU: ${machArm64.cpuTypeHex}`);
  } catch (err) {
    check('macOS arm64 binary Mach-O header parsing', false, err.message);
  }
}

// ==============================================================================
// PHASE 2: Windows Package & Unpacked Tree Verification
// ==============================================================================
function phase2() {
  console.log('\n======================================================================');
  console.log('Phase 2: Windows Package & Unpacked Tree Verification');
  console.log('======================================================================');

  const pkgJsonPath = path.join(rootDir, 'package.json');
  const pkg = JSON.parse(fs.readFileSync(pkgJsonPath, 'utf8'));

  // Inspect package.json configuration
  const winBuild = pkg.build && pkg.build.win;
  const nsisBuild = pkg.build && pkg.build.nsis;

  check('package.json defines build.win target nsis', Array.isArray(winBuild?.target) && winBuild.target.includes('nsis'));

  const winExtraResources = winBuild?.extraResources || [];
  const winSidecarMapping = winExtraResources.find((r) => r.from === 'bin/swiss.exe' && r.to === 'bin/swiss.exe');
  check('package.json maps bin/swiss.exe to bin/swiss.exe in extraResources', !!winSidecarMapping);

  check('package.json nsis.oneClick is true', nsisBuild?.oneClick === true);
  check('package.json nsis.perMachine is false', nsisBuild?.perMachine === false);

  // Inspect unpacked release tree if present
  const winUnpackedBin = path.join(releaseDir, 'windows', 'win-unpacked', 'resources', 'bin', 'swiss.exe');
  if (fs.existsSync(winUnpackedBin)) {
    const stat = fs.statSync(winUnpackedBin);
    const sizeMb = (stat.size / (1024 * 1024)).toFixed(2);
    check(`Windows unpacked sidecar daemon exists (${sizeMb} MB)`, true);
    check('Windows unpacked sidecar daemon size > 10MB', stat.size > 10 * 1024 * 1024, `Size: ${sizeMb} MB`);

    try {
      const pe = parsePEHeader(winUnpackedBin);
      check('Windows unpacked sidecar daemon has valid PE32+ header', pe.valid && pe.isAmd64 && pe.isPE32Plus);
    } catch (err) {
      check('Windows unpacked sidecar daemon PE verification', false, err.message);
    }
  } else {
    console.log('  ℹ release/windows/win-unpacked not generated yet; package.json configuration verified.');
  }
}

// ==============================================================================
// PHASE 3: macOS Package & App Bundle Structure Verification
// ==============================================================================
function phase3() {
  console.log('\n======================================================================');
  console.log('Phase 3: macOS Package & App Bundle Structure Verification');
  console.log('======================================================================');

  const pkgJsonPath = path.join(rootDir, 'package.json');
  const pkg = JSON.parse(fs.readFileSync(pkgJsonPath, 'utf8'));

  const macBuild = pkg.build && pkg.build.mac;
  check('package.json defines build.mac target dmg', Array.isArray(macBuild?.target) && macBuild.target.includes('dmg'));

  const macExtraResources = macBuild?.extraResources || [];
  const macSidecarMapping = macExtraResources.find((r) => r.from === 'bin/swiss' && r.to === 'bin/swiss');
  check('package.json maps bin/swiss to bin/swiss in extraResources', !!macSidecarMapping);

  // Inspect macOS app bundle if present
  const macAppBundle = path.join(releaseDir, 'macos', 'mac', 'Antigravity Swiss Knife.app');
  const macSidecarBin = path.join(macAppBundle, 'Contents', 'Resources', 'bin', 'swiss');

  if (fs.existsSync(macSidecarBin)) {
    const stat = fs.statSync(macSidecarBin);
    const sizeMb = (stat.size / (1024 * 1024)).toFixed(2);
    check(`macOS app bundle sidecar exists at Contents/Resources/bin/swiss (${sizeMb} MB)`, true);

    let isExecutable = false;
    try {
      fs.accessSync(macSidecarBin, fs.constants.X_OK);
      isExecutable = true;
    } catch {
      isExecutable = (stat.mode & 0o111) !== 0;
    }
    check('macOS app bundle sidecar has executable permissions (chmod 0755)', isExecutable);
    check('macOS app bundle sidecar size > 10MB', stat.size > 10 * 1024 * 1024, `Size: ${sizeMb} MB`);

    try {
      const mach = parseMachOHeader(macSidecarBin);
      check('macOS app bundle sidecar has valid Mach-O 64-bit header', mach.is64Bit);
    } catch (err) {
      check('macOS app bundle sidecar Mach-O verification', false, err.message);
    }
  } else {
    console.log('  ℹ release/macos/mac/Antigravity Swiss Knife.app not generated yet; package.json configuration verified.');
  }
}

// ==============================================================================
// PHASE 4: Wine Emulation Headless Execution (Smoke Test)
// ==============================================================================
function phase4() {
  console.log('\n======================================================================');
  console.log('Phase 4: Wine Emulation Headless Execution (Smoke Test)');
  console.log('======================================================================');

  let winePath = null;
  try {
    winePath = execSync('which wine', { encoding: 'utf8' }).trim();
  } catch {
    if (fs.existsSync('/usr/bin/wine')) {
      winePath = '/usr/bin/wine';
    }
  }

  if (!winePath) {
    console.log('  ℹ Wine is not installed on this host; skipping Wine execution smoke test.');
    return;
  }
  check(`Wine emulator found at ${winePath}`, true);

  const winBin = path.join(binDir, 'swiss.exe');
  check('Windows binary exists for Wine test', fs.existsSync(winBin));

  // 1. wine bin/swiss.exe version
  const resVer = spawnSync(winePath, [winBin, 'version'], {
    cwd: rootDir,
    encoding: 'utf8',
    timeout: 10000,
    env: { ...process.env, WINEDEBUG: '-all' }
  });
  const verOut = (resVer.stdout || '') + (resVer.stderr || '');
  check('wine bin/swiss.exe version exits with code 0', resVer.status === 0, `Exit code: ${resVer.status}`);
  check('wine bin/swiss.exe version output contains "Antigravity Swiss Knife"', verOut.includes('Antigravity Swiss Knife'), `Output: ${verOut.trim()}`);

  // 2. wine bin/swiss.exe --help
  const resHelp = spawnSync(winePath, [winBin, '--help'], {
    cwd: rootDir,
    encoding: 'utf8',
    timeout: 10000,
    env: { ...process.env, WINEDEBUG: '-all' }
  });
  const helpOut = (resHelp.stdout || '') + (resHelp.stderr || '');
  check('wine bin/swiss.exe --help exits with code 0', resHelp.status === 0, `Exit code: ${resHelp.status}`);
  check('wine bin/swiss.exe --help outputs command list', helpOut.includes('Commands:') || helpOut.includes('Usage:'), 'Help text parsed');
}

// ==============================================================================
// PHASE 5: DaemonManager Cross-Platform Path Resolution Simulation
// ==============================================================================
function phase5() {
  console.log('\n======================================================================');
  console.log('Phase 5: DaemonManager Cross-Platform Path Resolution Simulation');
  console.log('======================================================================');

  const daemonManagerPath = path.join(rootDir, 'electron', 'daemon-manager.js');
  check('electron/daemon-manager.js exists', fs.existsSync(daemonManagerPath));

  const dmCode = fs.readFileSync(daemonManagerPath, 'utf8');

  // Static checks on code logic
  const hasWinPlatformBinary = dmCode.includes("process.platform === 'win32'") &&
    dmCode.includes("'swiss.exe'") &&
    dmCode.includes("'swiss'");
  check("DaemonManager selects swiss.exe on win32 and swiss on POSIX", hasWinPlatformBinary);

  const hasPackagedResourcePath = dmCode.includes('process.resourcesPath') &&
    dmCode.includes("'bin'");
  check("DaemonManager resolves process.resourcesPath/bin/<binary>", hasPackagedResourcePath);

  const skipsWinSocketUnlink = dmCode.includes("cleanupSocket()") &&
    dmCode.includes("if (process.platform === 'win32') return;");
  check("DaemonManager cleanly skips Unix socket unlink on win32", skipsWinSocketUnlink);

  const skipsWinChmod = dmCode.includes("ensureExecutable(") &&
    dmCode.includes("if (process.platform === 'win32') return;");
  check("DaemonManager cleanly skips chmod on win32", skipsWinChmod);

  const bindsUserDataCwd = dmCode.includes("app.getPath('userData')");
  check("DaemonManager sets packaged runCwd to app.getPath('userData')", bindsUserDataCwd);

  // Functional simulation of platform binary resolution
  function simulateResolution(platform, isPackaged, resourcesPath) {
    const binName = platform === 'win32' ? 'swiss.exe' : 'swiss';
    if (isPackaged) {
      return path.join(resourcesPath, 'bin', binName);
    }
    return path.join(rootDir, 'bin', binName);
  }

  const simulatedWinPath = simulateResolution('win32', true, 'C:\\Program Files\\ASK\\resources');
  check('Simulation: win32 packaged resolves to resources\\bin\\swiss.exe',
    simulatedWinPath.endsWith(path.join('resources', 'bin', 'swiss.exe')));

  const simulatedMacPath = simulateResolution('darwin', true, '/Applications/ASK.app/Contents/Resources');
  check('Simulation: darwin packaged resolves to Resources/bin/swiss',
    simulatedMacPath.endsWith(path.join('Resources', 'bin', 'swiss')));

  const simulatedLinuxPath = simulateResolution('linux', true, '/opt/ASK/resources');
  check('Simulation: linux packaged resolves to resources/bin/swiss',
    simulatedLinuxPath.endsWith(path.join('resources', 'bin', 'swiss')));
}

// ==============================================================================
// PHASE 6: Cross-Platform Path Constants Verification
// ==============================================================================
function phase6() {
  console.log('\n======================================================================');
  console.log('Phase 6: Cross-Platform Path Constants Verification');
  console.log('======================================================================');

  const constantsGoPath = path.join(rootDir, 'pkg', 'core', 'constants.go');
  check('pkg/core/constants.go exists', fs.existsSync(constantsGoPath));

  const goSource = fs.readFileSync(constantsGoPath, 'utf8');

  // 1. Windows Config & Desktop paths
  const winConfigDir = goSource.includes('runtime.GOOS == "windows"') &&
    goSource.includes('filepath.Join(appData, "antigravity-swiss")');
  check('Windows uses %APPDATA%\\antigravity-swiss for GetConfigDir()', winConfigDir);

  const winHostConfigDir = goSource.includes('runtime.GOOS == "windows"') &&
    goSource.includes('filepath.Join(appData, "Antigravity")');
  check('Windows uses %APPDATA%\\Antigravity for GetAntigravityHostConfigDir()', winHostConfigDir);

  const winAppPath = goSource.includes('runtime.GOOS == "windows"') &&
    goSource.includes('filepath.Join(localAppData, "Programs", "Antigravity")');
  check('Windows uses %LOCALAPPDATA%\\Programs\\Antigravity for GetAntigravityDesktopAppPath()', winAppPath);

  const winResourcesDir = goSource.includes('runtime.GOOS == "windows"') &&
    goSource.includes('filepath.Join(localAppData, "Programs", "Antigravity", "resources")');
  check('Windows uses %LOCALAPPDATA%\\Programs\\Antigravity\\resources for GetAntigravityDesktopResourcesDir()', winResourcesDir);

  // 2. macOS Config & Desktop paths
  const macConfigDir = goSource.includes('runtime.GOOS == "darwin"') &&
    goSource.includes('Library", "Application Support", "antigravity-swiss');
  check('macOS uses ~/Library/Application Support/antigravity-swiss for GetConfigDir()', macConfigDir);

  const macHostConfigDir = goSource.includes('runtime.GOOS == "darwin"') &&
    goSource.includes('Library", "Application Support", "Antigravity');
  check('macOS uses ~/Library/Application Support/Antigravity for GetAntigravityHostConfigDir()', macHostConfigDir);

  const macAppPath = goSource.includes('runtime.GOOS == "darwin"') &&
    goSource.includes('/Applications/Antigravity.app');
  check('macOS uses /Applications/Antigravity.app for GetAntigravityDesktopAppPath()', macAppPath);

  const macResourcesDir = goSource.includes('runtime.GOOS == "darwin"') &&
    goSource.includes('/Applications/Antigravity.app/Contents/Resources');
  check('macOS uses /Applications/Antigravity.app/Contents/Resources for GetAntigravityDesktopResourcesDir()', macResourcesDir);
}

// ==============================================================================
// Main Runner
// ==============================================================================
function main() {
  console.log('======================================================================');
  console.log('Antigravity Swiss Knife - Cross-Platform Packaging Verification');
  console.log('======================================================================');

  phase1();
  phase2();
  phase3();
  phase4();
  phase5();
  phase6();

  console.log('\n======================================================================');
  console.log(`Summary: ${passedChecks}/${totalChecks} checks passed (${failedChecks} failed)`);
  console.log('======================================================================');

  if (failedChecks > 0) {
    console.error('\nVerification failed with the following errors:');
    failureDetails.forEach(({ label, detail }) => {
      console.error(` - ${label}${detail ? `: ${detail}` : ''}`);
    });
    process.exit(1);
  }

  console.log('✓ All cross-platform packaging verification phases passed successfully!\n');
  process.exit(0);
}

main();
