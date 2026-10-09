#!/usr/bin/env node

/**
 * ==============================================================================
 * Antigravity Swiss Knife - Linux Out-of-Tree Installer (Model 1)
 * ==============================================================================
 * Installs the desktop application bundle into a stable, non-volatile location
 * outside the mutable git repository.
 *
 * Targets:
 *   - User mode (default / non-root):
 *       App:     ~/.local/share/antigravity-swiss-knife/app/
 *       Desktop: ~/.local/share/applications/antigravity-swiss-knife.desktop
 *       Icons:   ~/.local/share/icons/hicolor/<size>/apps/antigravity-swiss-knife.png
 *       CLI:     ~/.local/bin/antigravity-swiss-knife
 *   - System mode (root / sudo):
 *       App:     /opt/Antigravity Swiss Knife/
 *       Desktop: /usr/share/applications/antigravity-swiss-knife.desktop
 *       Icons:   /usr/share/icons/hicolor/<size>/apps/antigravity-swiss-knife.png
 *       CLI:     /usr/local/bin/antigravity-swiss-knife
 * ==============================================================================
 */

const { execSync } = require('child_process');
const path = require('path');
const fs = require('fs');
const os = require('os');

const rootDir = path.resolve(__dirname, '..');
const releaseLinuxDir = path.join(rootDir, 'release', 'linux');
const unpackedDir = path.join(releaseLinuxDir, 'linux-unpacked');

function log(msg) {
  console.log(`[Install] ${msg}`);
}

function ensureDir(dir) {
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }
}

function isRoot() {
  return typeof process.getuid === 'function' && process.getuid() === 0;
}

function canSudo() {
  try {
    execSync('sudo -n true', { stdio: 'ignore' });
    return true;
  } catch {
    return false;
  }
}

function cleanStaleRootSymlinks() {
  const rootSymlinks = [
    path.join(rootDir, 'Antigravity-Swiss-Knife.AppImage'),
    path.join(rootDir, 'antigravity-swiss-knife')
  ];
  for (const symlink of rootSymlinks) {
    try {
      if (fs.existsSync(symlink) || fs.lstatSync(symlink).isSymbolicLink()) {
        fs.unlinkSync(symlink);
        log(`Cleaned stale root symlink: ${path.basename(symlink)}`);
      }
    } catch {}
  }
}

function ensureBuildExists(forceRebuild = false) {
  const mainExe = path.join(unpackedDir, 'antigravity-swiss-knife');
  const mainBin = path.join(unpackedDir, 'antigravity-swiss-knife.bin');
  const daemonBin = path.join(unpackedDir, 'resources', 'bin', 'swiss');

  const bundleReady = fs.existsSync(mainExe) && fs.existsSync(mainBin) && fs.existsSync(daemonBin);

  if (!bundleReady || forceRebuild) {
    log('Linux unpacked bundle missing or incomplete. Building release...');
    execSync('node scripts/build-release.js linux --unpacked', {
      cwd: rootDir,
      stdio: 'inherit'
    });
  }

  if (!fs.existsSync(mainExe) || !fs.existsSync(daemonBin)) {
    throw new Error(`Build failed: unpacked artifacts not found in ${unpackedDir}`);
  }
}

function resolveInstallPaths(mode = 'auto') {
  let useSystem = false;
  if (mode === 'system') {
    useSystem = true;
  } else if (mode === 'user') {
    useSystem = false;
  } else {
    // auto mode
    useSystem = isRoot();
  }

  if (useSystem) {
    return {
      type: 'system',
      appDir: '/opt/Antigravity Swiss Knife',
      parentDir: '/opt',
      desktopDir: '/usr/share/applications',
      binDir: '/usr/local/bin',
      iconsDir: '/usr/share/icons/hicolor'
    };
  }

  const home = os.homedir();
  const dataDir = path.join(home, '.local', 'share', 'antigravity-swiss-knife');
  return {
    type: 'user',
    appDir: path.join(dataDir, 'app'),
    parentDir: dataDir,
    desktopDir: path.join(home, '.local', 'share', 'applications'),
    binDir: path.join(home, '.local', 'bin'),
    iconsDir: path.join(home, '.local', 'share', 'icons', 'hicolor')
  };
}

function stageAndInstallApp(targetPaths) {
  const { appDir, parentDir } = targetPaths;
  ensureDir(parentDir);

  const stagingDir = path.join(parentDir, `.app-staging-${Date.now()}`);
  log(`Staging application files into: ${stagingDir}`);

  // Copy unpacked tree
  fs.cpSync(unpackedDir, stagingDir, { recursive: true, dereference: false });

  // Copy assets into staging resources/assets and staging assets
  const sourceAssetsDir = path.join(rootDir, 'assets');
  if (fs.existsSync(sourceAssetsDir)) {
    const stagingResourcesAssets = path.join(stagingDir, 'resources', 'assets');
    const stagingAssets = path.join(stagingDir, 'assets');
    ensureDir(stagingResourcesAssets);
    ensureDir(stagingAssets);
    fs.cpSync(sourceAssetsDir, stagingResourcesAssets, { recursive: true });
    fs.cpSync(sourceAssetsDir, stagingAssets, { recursive: true });
  }

  // Ensure execution permissions on binaries and libraries
  const executables = [
    path.join(stagingDir, 'antigravity-swiss-knife'),
    path.join(stagingDir, 'antigravity-swiss-knife.bin'),
    path.join(stagingDir, 'chrome_crashpad_handler'),
    path.join(stagingDir, 'resources', 'bin', 'swiss')
  ];

  for (const bin of executables) {
    if (fs.existsSync(bin)) {
      try { fs.chmodSync(bin, 0o755); } catch {}
    }
  }

  // Chmod .so libraries
  try {
    const files = fs.readdirSync(stagingDir);
    for (const file of files) {
      if (file.endsWith('.so') || file.includes('.so.')) {
        try { fs.chmodSync(path.join(stagingDir, file), 0o755); } catch {}
      }
    }
  } catch {}

  // Atomically replace app directory
  log(`Installing staged files to stable destination: ${appDir}`);
  if (fs.existsSync(appDir)) {
    try {
      fs.rmSync(appDir, { recursive: true, force: true });
    } catch (err) {
      log(`Warning: could not delete existing appDir directly: ${err.message}`);
    }
  }

  try {
    fs.renameSync(stagingDir, appDir);
  } catch {
    // If rename failed (cross-device fallback), copy and delete
    ensureDir(appDir);
    fs.cpSync(stagingDir, appDir, { recursive: true });
    try { fs.rmSync(stagingDir, { recursive: true, force: true }); } catch {}
  }

  // Re-verify permissions on destination
  const destExecutables = [
    path.join(appDir, 'antigravity-swiss-knife'),
    path.join(appDir, 'antigravity-swiss-knife.bin'),
    path.join(appDir, 'chrome_crashpad_handler'),
    path.join(appDir, 'resources', 'bin', 'swiss')
  ];

  for (const bin of destExecutables) {
    if (fs.existsSync(bin)) {
      try { fs.chmodSync(bin, 0o755); } catch {}
    }
  }

  log(`✓ Application tree installed at: ${appDir}`);
}

function installIcons(targetPaths) {
  const { iconsDir } = targetPaths;
  const iconsSourceDir = path.join(rootDir, 'assets', 'icons');
  if (!fs.existsSync(iconsSourceDir)) {
    log('No assets/icons directory found; skipping hicolor icon installation.');
    return;
  }

  const iconSizes = [
    { file: 'icon_16x16.png', size: '16x16' },
    { file: 'icon_24x24.png', size: '24x24' },
    { file: 'icon_32x32.png', size: '32x32' },
    { file: 'icon_48x48.png', size: '48x48' },
    { file: 'icon_64x64.png', size: '64x64' },
    { file: 'icon_128x128.png', size: '128x128' },
    { file: 'icon_256x256.png', size: '256x256' },
    { file: 'icon_512x512.png', size: '512x512' },
    { file: 'icon_1024x1024.png', size: '1024x1024' }
  ];

  for (const item of iconSizes) {
    const srcFile = path.join(iconsSourceDir, item.file);
    if (fs.existsSync(srcFile)) {
      const destDir = path.join(iconsDir, item.size, 'apps');
      ensureDir(destDir);
      const destFile = path.join(destDir, 'antigravity-swiss-knife.png');
      fs.copyFileSync(srcFile, destFile);
      try { fs.chmodSync(destFile, 0o644); } catch {}
    }
  }

  log(`✓ Hicolor icon theme assets installed in: ${iconsDir}`);

  // Update icon cache if tool exists
  try {
    execSync(`gtk-update-icon-cache -f -t "${iconsDir}" 2>/dev/null || true`, { stdio: 'ignore' });
  } catch {}
}

function installDesktopEntry(targetPaths) {
  const { appDir, desktopDir } = targetPaths;
  ensureDir(desktopDir);

  const desktopFilePath = path.join(desktopDir, 'antigravity-swiss-knife.desktop');
  const exePath = path.join(appDir, 'antigravity-swiss-knife');
  const logoPath = path.join(appDir, 'resources', 'assets', 'logo.png');

  const desktopContent = `[Desktop Entry]
Name=Antigravity Swiss Knife
Comment=Desktop Companion for Google Antigravity
GenericName=Google Antigravity Desktop Companion
Exec="${exePath}" %U
Path=${appDir}
Icon=${logoPath}
Terminal=false
Type=Application
Categories=Development;
StartupWMClass=antigravity-swiss-knife
MimeType=x-scheme-handler/antigravity;
Keywords=antigravity;gemini;daemon;ai;swiss-knife;
`;

  fs.writeFileSync(desktopFilePath, desktopContent, { mode: 0o644 });
  log(`✓ Desktop shortcut written: ${desktopFilePath}`);

  // Validate desktop entry
  try {
    execSync(`desktop-file-validate "${desktopFilePath}"`, { stdio: 'inherit' });
    log('✓ Desktop entry passed desktop-file-validate check.');
  } catch (err) {
    console.warn(`[Install] Warning: desktop-file-validate reported an issue: ${err.message}`);
  }

  // Update desktop database if tool exists
  try {
    execSync(`update-desktop-database "${desktopDir}" 2>/dev/null || true`, { stdio: 'ignore' });
    log(`✓ Desktop application database updated for: ${desktopDir}`);
  } catch {}
}

function installCliSymlink(targetPaths) {
  const { appDir, binDir } = targetPaths;
  ensureDir(binDir);

  const symlinkPath = path.join(binDir, 'antigravity-swiss-knife');
  const targetExe = path.join(appDir, 'antigravity-swiss-knife');

  try {
    if (fs.existsSync(symlinkPath) || fs.lstatSync(symlinkPath).isSymbolicLink()) {
      fs.unlinkSync(symlinkPath);
    }
  } catch {}

  fs.symlinkSync(targetExe, symlinkPath);
  log(`✓ CLI launcher symlink updated: ${symlinkPath} -> ${targetExe}`);
}

function verifyInstallation(targetPaths) {
  log('====================================================');
  log('Verifying out-of-tree installation integrity...');
  log('====================================================');

  const { appDir, desktopDir, binDir } = targetPaths;
  const desktopFile = path.join(desktopDir, 'antigravity-swiss-knife.desktop');
  const launcherBin = path.join(appDir, 'antigravity-swiss-knife');
  const daemonBin = path.join(appDir, 'resources', 'bin', 'swiss');
  const cliSymlink = path.join(binDir, 'antigravity-swiss-knife');
  const iconFile = path.join(appDir, 'resources', 'assets', 'logo.png');

  let passed = true;

  if (fs.existsSync(desktopFile)) {
    log(`[Verify] Desktop file exists: ${desktopFile}`);
    try {
      execSync(`desktop-file-validate "${desktopFile}"`, { stdio: 'ignore' });
      log('[Verify] desktop-file-validate: PASS');
    } catch {
      console.error('[Verify] desktop-file-validate: FAIL');
      passed = false;
    }
  } else {
    console.error(`[Verify] Desktop file missing: ${desktopFile}`);
    passed = false;
  }

  if (fs.existsSync(launcherBin)) {
    log(`[Verify] Installed app executable exists: ${launcherBin}`);
    try {
      fs.accessSync(launcherBin, fs.constants.X_OK);
      log('[Verify] Launcher executable permissions: PASS (0755)');
    } catch {
      console.error('[Verify] Launcher executable permissions: FAIL (Not executable)');
      passed = false;
    }
  } else {
    console.error(`[Verify] Launcher executable missing: ${launcherBin}`);
    passed = false;
  }

  if (fs.existsSync(daemonBin)) {
    log(`[Verify] Bundled Go daemon exists: ${daemonBin}`);
    try {
      fs.accessSync(daemonBin, fs.constants.X_OK);
      const versionStr = execSync(`"${daemonBin}" --version`, { encoding: 'utf8' }).trim();
      log(`[Verify] Daemon executable execution: PASS (${versionStr})`);
    } catch (err) {
      console.error(`[Verify] Daemon executable execution: FAIL (${err.message})`);
      passed = false;
    }
  } else {
    console.error(`[Verify] Bundled Go daemon missing: ${daemonBin}`);
    passed = false;
  }

  if (fs.existsSync(cliSymlink)) {
    try {
      const realTarget = fs.realpathSync(cliSymlink);
      if (realTarget === launcherBin) {
        log(`[Verify] CLI symlink resolution: PASS (${cliSymlink} -> ${realTarget})`);
      } else {
        console.error(`[Verify] CLI symlink resolution mismatch: ${realTarget} != ${launcherBin}`);
        passed = false;
      }
    } catch (err) {
      console.error(`[Verify] CLI symlink error: ${err.message}`);
      passed = false;
    }
  } else {
    console.error(`[Verify] CLI symlink missing: ${cliSymlink}`);
    passed = false;
  }

  if (fs.existsSync(iconFile)) {
    log(`[Verify] Icon asset exists: ${iconFile}`);
  } else {
    console.error(`[Verify] Icon asset missing: ${iconFile}`);
    passed = false;
  }

  if (!passed) {
    throw new Error('Installation verification failed.');
  }

  log('====================================================');
  log('✓ All installation verification checks PASSED (100%).');
  log('====================================================');
}

function parseCliArgs() {
  const args = process.argv.slice(2);
  const options = {
    mode: 'auto',
    forceRebuild: false,
    verifyOnly: false,
    help: false
  };

  for (const arg of args) {
    if (arg === '--user') {
      options.mode = 'user';
    } else if (arg === '--system') {
      options.mode = 'system';
    } else if (arg === '--rebuild' || arg === '--force-build') {
      options.forceRebuild = true;
    } else if (arg === '--verify-only') {
      options.verifyOnly = true;
    } else if (arg === '--help' || arg === '-h') {
      options.help = true;
    }
  }
  return options;
}

function printHelp() {
  console.log(`
Antigravity Swiss Knife - Linux Out-of-Tree Installer

Usage:
  node scripts/install-linux.js [options]
  npm run install:linux

Options:
  --user          Install to user directory (~/.local/share/antigravity-swiss-knife/app)
  --system        Install to system directory (/opt/Antigravity Swiss Knife) [requires root]
  --rebuild       Force rebuilding unpacked release before installing
  --verify-only   Run integrity verification checks on current installation
  --help, -h      Display this help message
`);
}

function main() {
  const options = parseCliArgs();
  if (options.help) {
    printHelp();
    process.exit(0);
  }

  log('Starting Antigravity Swiss Knife Linux Installer (Model 1)...');
  cleanStaleRootSymlinks();

  const targetPaths = resolveInstallPaths(options.mode);
  log(`Installation target mode: [${targetPaths.type}] -> ${targetPaths.appDir}`);

  if (options.verifyOnly) {
    verifyInstallation(targetPaths);
    return;
  }

  ensureBuildExists(options.forceRebuild);
  stageAndInstallApp(targetPaths);
  installIcons(targetPaths);
  installDesktopEntry(targetPaths);
  installCliSymlink(targetPaths);
  verifyInstallation(targetPaths);

  log('Installation completed successfully.');
}

if (require.main === module) {
  try {
    main();
  } catch (err) {
    console.error(`[Install Fatal Error]: ${err.message}`);
    process.exit(1);
  }
}

module.exports = {
  resolveInstallPaths,
  stageAndInstallApp,
  installIcons,
  installDesktopEntry,
  installCliSymlink,
  verifyInstallation
};
