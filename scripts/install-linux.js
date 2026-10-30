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

const { execSync, spawn } = require('child_process');
const http = require('http');
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

function isMainAppCommand(argv, appDir) {
  if (!Array.isArray(argv) || argv.length === 0) {
    return false;
  }
  if (argv.some((arg) => typeof arg === 'string' && arg.startsWith('--type='))) {
    return false;
  }
  const executable = path.resolve(argv[0]);
  return executable === path.join(appDir, 'antigravity-swiss-knife') ||
    executable === path.join(appDir, 'antigravity-swiss-knife.bin');
}

function findInstalledMainAppPids(appDir) {
  if (process.platform !== 'linux') {
    return [];
  }
  const pids = [];
  for (const entry of fs.readdirSync('/proc')) {
    if (!/^\d+$/.test(entry)) continue;
    const pid = Number(entry);
    if (pid === process.pid || pid <= 1) continue;
    try {
      const argv = fs.readFileSync(`/proc/${pid}/cmdline`, 'utf8').split('\0').filter(Boolean);
      if (isMainAppCommand(argv, appDir)) {
        pids.push(pid);
      }
    } catch {}
  }
  return pids;
}

function isProcessAlive(pid) {
  try {
    process.kill(pid, 0);
    return fs.existsSync(`/proc/${pid}`);
  } catch {
    return false;
  }
}

function sleepSync(ms) {
  Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms);
}

function stopInstalledApp(appDir) {
  const pids = findInstalledMainAppPids(appDir);
  if (pids.length === 0) {
    log('No running installed app instance to stop.');
    return;
  }

  log(`Stopping installed app instance${pids.length === 1 ? '' : 's'}: ${pids.join(', ')}`);
  for (const pid of pids) {
    try {
      process.kill(pid, 'SIGTERM');
    } catch {}
  }

  const deadline = Date.now() + 6000;
  while (Date.now() < deadline) {
    if (pids.every((pid) => !isProcessAlive(pid))) {
      log('Installed app stopped cleanly.');
      return;
    }
    sleepSync(200);
  }

  for (const pid of pids) {
    if (!isProcessAlive(pid)) continue;
    log(`Escalating stop for PID ${pid}`);
    try {
      process.kill(pid, 'SIGKILL');
    } catch {}
  }
}

function launchInstalledApp(targetPaths) {
  const launcher = path.join(targetPaths.appDir, 'antigravity-swiss-knife');
  if (!fs.existsSync(launcher)) {
    throw new Error(`Installed launcher missing: ${launcher}`);
  }
  const child = spawn(launcher, [], {
    cwd: targetPaths.appDir,
    detached: true,
    stdio: 'ignore',
    env: { ...process.env }
  });
  child.unref();
  log(`Relaunched installed app detached (PID ${child.pid}).`);
  return child.pid;
}

function probeDaemonHealth() {
  return new Promise((resolve) => {
    const req = http.get({
      host: '127.0.0.1',
      port: 8765,
      path: '/health',
      timeout: 750
    }, (res) => {
      let body = '';
      res.setEncoding('utf8');
      res.on('data', (chunk) => {
        body += chunk;
      });
      res.on('end', () => {
        try {
          const status = JSON.parse(body);
          resolve(res.statusCode === 200 && status.status === 'ok' && status.daemon_running === true);
        } catch {
          resolve(false);
        }
      });
    });
    req.on('timeout', () => {
      req.destroy();
      resolve(false);
    });
    req.on('error', () => resolve(false));
  });
}

async function waitForDaemonHealth(timeoutMs = 15000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (await probeDaemonHealth()) {
      log('Relaunched daemon is healthy on http://127.0.0.1:8765.');
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  throw new Error('Relaunched app did not bring the daemon to healthy state within 15s');
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

  let bundleReady = fs.existsSync(mainExe) && fs.existsSync(mainBin) && fs.existsSync(daemonBin);

  if (bundleReady && !forceRebuild) {
    try {
      const bundleMtime = fs.statSync(mainExe).mtimeMs;
      const keyFiles = [
        'package.json',
        'electron/main.js',
        'electron/daemon-manager.js',
        'scripts/build-release.js',
        'cmd/swiss/main.go',
        'antigravity-acp/cmd/antigravity-acp/main.go',
        'frontend/src/App.tsx',
        'frontend/src/pages/CustomModelsPage.tsx',
        'pkg/webgui/server.go'
      ];
      for (const f of keyFiles) {
        const fullPath = path.join(rootDir, f);
        if (fs.existsSync(fullPath) && fs.statSync(fullPath).mtimeMs > bundleMtime) {
          log(`Source file ${f} is newer than unpacked bundle. Forcing release rebuild...`);
          bundleReady = false;
          break;
        }
      }
    } catch (_) {}
  }

  if (!bundleReady || forceRebuild) {
    log('Linux unpacked bundle missing, incomplete or stale. Building release...');
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

function stageAndInstallApp(targetPaths, options = {}) {
  const { appDir, parentDir } = targetPaths;
  ensureDir(parentDir);

  const stagingDir = path.join(parentDir, `.app-staging-${Date.now()}`);
  log(`Staging application files into: ${stagingDir}`);

  // Copy unpacked tree
  fs.cpSync(unpackedDir, stagingDir, { recursive: true, dereference: false });

  // Older unpacked bundles may not contain the ACP sidecar. Install the
  // standalone build output when it is available so the preset works.
  const stagingAcp = path.join(stagingDir, 'resources', 'bin', 'antigravity-acp');
  if (!fs.existsSync(stagingAcp)) {
    const builtAcp = path.join(rootDir, 'bin', 'antigravity-acp');
    if (fs.existsSync(builtAcp)) {
      ensureDir(path.dirname(stagingAcp));
      fs.copyFileSync(builtAcp, stagingAcp);
    }
  }

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
    path.join(stagingDir, 'resources', 'bin', 'swiss'),
    path.join(stagingDir, 'resources', 'bin', 'antigravity-acp')
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

  if (typeof options.beforeReplace === 'function') {
    options.beforeReplace();
  }

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
    path.join(appDir, 'resources', 'bin', 'swiss'),
    path.join(appDir, 'resources', 'bin', 'antigravity-acp')
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

function findInstalledAcpBinary(appDir) {
  const candidates = [
    path.join(appDir, 'resources', 'bin', 'antigravity-acp'),
    path.join(appDir, 'bin', 'antigravity-acp')
  ];
  return candidates.find((candidate) => fs.existsSync(candidate)) || null;
}

function installAcpCliSymlink(targetPaths) {
  const { appDir, binDir } = targetPaths;
  const targetAcp = findInstalledAcpBinary(appDir);
  if (!targetAcp) {
    log('Bundled antigravity-acp not found; skipping ACP CLI symlink.');
    return null;
  }

  ensureDir(binDir);
  try { fs.chmodSync(targetAcp, 0o755); } catch {}

  const symlinkPath = path.join(binDir, 'antigravity-acp');
  try {
    if (fs.existsSync(symlinkPath) || fs.lstatSync(symlinkPath).isSymbolicLink()) {
      fs.unlinkSync(symlinkPath);
    }
  } catch {}

  fs.symlinkSync(targetAcp, symlinkPath);
  log(`✓ ACP CLI symlink updated: ${symlinkPath} -> ${targetAcp}`);
  return symlinkPath;
}

function ensureAgyCliGateway(targetPaths) {
  const { binDir } = targetPaths;
  const agyPath = path.join(binDir, "agy");
  const agyBinPath = path.join(binDir, "agy.bin");
  if (!fs.existsSync(agyPath) && !fs.existsSync(agyBinPath)) {
    return;
  }
  try {
    if (fs.existsSync(agyPath)) {
      const content = fs.readFileSync(agyPath, "utf8");
      if (!content.includes("Swiss Knife Integration Wrapper") && !content.includes("Swiss Knife Gateway Integration Wrapper")) {
        fs.copyFileSync(agyPath, agyBinPath);
        fs.chmodSync(agyBinPath, 0o755);
      }
    }
  } catch {
    try {
      fs.copyFileSync(agyPath, agyBinPath);
      fs.chmodSync(agyBinPath, 0o755);
    } catch {}
  }

  const wrapperScript = `#!/usr/bin/env bash
# Antigravity CLI (agy) - Swiss Knife Integration Wrapper
set -e

DIR="$(cd "$(dirname "\${BASH_SOURCE[0]}")" && pwd)"
REAL_BIN="\${DIR}/agy.bin"
if [ ! -f "$REAL_BIN" ]; then
    REAL_BIN="\${DIR}/agy.real"
fi
if [ ! -f "$REAL_BIN" ]; then
    REAL_BIN="${path.join(os.homedir(), '.gemini', 'bin', 'agy')}"
fi

# Route agy through Swiss Knife Cloud Code proxy if active; fallback to native if closed
PROXY_PORT=8765
if (exec 3<>/dev/tcp/127.0.0.1/\${PROXY_PORT}) 2>/dev/null; then
    exec 3>&-
    export CLOUD_CODE_URL="http://127.0.0.1:\${PROXY_PORT}"
    unset AGY_LLM_GATEWAY_URL AGY_LLM_GATEWAY_API_KEY AGY_LLM_GATEWAY_MODELS 2>/dev/null || true
else
    unset CLOUD_CODE_URL AGY_LLM_GATEWAY_URL AGY_LLM_GATEWAY_API_KEY AGY_LLM_GATEWAY_MODELS 2>/dev/null || true
fi

exec "$REAL_BIN" "$@"
`;

  try {
    fs.writeFileSync(agyPath, wrapperScript, { mode: 0o755 });
    log(`✓ AGY CLI wrapper configured at: ${agyPath}`);
  } catch (err) {
    log(`Notice: Could not write AGY CLI wrapper: ${err.message}`);
  }

  cleanShellRCHooks();
}

function cleanShellRCHooks(homeDir = os.homedir()) {
  const markerPairs = [
    ['# >>> Antigravity Swiss Knife CLI Integration >>>', '# <<< Antigravity Swiss Knife CLI Integration <<<'],
    ['# >>> Antigravity Swiss Knife >>>', '# <<< Antigravity Swiss Knife <<<'],
    ['# >>> Swiss Knife CLI Integration >>>', '# <<< Swiss Knife CLI Integration <<<'],
    ['# BEGIN Antigravity Swiss Knife CLI Integration', '# END Antigravity Swiss Knife CLI Integration'],
    ['# BEGIN Swiss Knife CLI Integration', '# END Swiss Knife CLI Integration'],
    ['# BEGIN Antigravity Swiss Knife', '# END Antigravity Swiss Knife'],
  ];
  const beginMarkers = new Map(markerPairs);
  const endMarkers = new Set(markerPairs.map(([, endMarker]) => endMarker));
  const legacyHeaders = new Set([
    '# Antigravity Swiss Knife - CLI Integration Environment',
    '# Antigravity Swiss Knife - CLI Gateway Environment',
    '# Antigravity Swiss Knife - CLI Integration',
    '# Antigravity Swiss Knife - CLI Gateway',
  ]);
  const legacyExports = [
    'export CLOUD_CODE_URL=',
    'export AGY_LLM_GATEWAY_URL=',
    'export AGY_LLM_GATEWAY_API_KEY=',
    'export AGY_LLM_GATEWAY_MODELS=',
  ];
  const lineText = (line) => line.replace(/\r?\n$/, '').trim();

  for (const rc of ['.zshrc', '.bashrc']) {
    const rcPath = path.join(homeDir, rc);
    let lockPath;
    let lockFd;
    try {
      let target;
      try {
        target = fs.realpathSync(rcPath);
      } catch (err) {
        if (err.code === 'ENOENT') continue;
        throw err;
      }

      // The Go cleanup keeps its advisory .swiss.lock inode; O_EXCL needs its own path.
      lockPath = `${target}.swiss-install.lock`;
      lockFd = acquireShellRCLock(lockPath);
      const data = fs.readFileSync(target, 'utf8');
      const lines = data.match(/[^\n]*\n|[^\n]+$/g) || [];
      const kept = [];
      let owned = false;
      let changed = false;
      let legacy = false;

      for (let i = 0; i < lines.length;) {
        const text = lineText(lines[i]);
        if (beginMarkers.has(text)) {
          owned = true;
          const endMarker = beginMarkers.get(text);
          let j = i + 1;
          for (; j < lines.length; j++) {
            const candidate = lineText(lines[j]);
            if (beginMarkers.has(candidate)) {
              throw new Error(`managed block restarted before ${endMarker}`);
            }
            if (endMarkers.has(candidate)) {
              if (candidate !== endMarker) {
                throw new Error(`managed block ended with ${candidate}`);
              }
              break;
            }
          }
          if (j === lines.length) {
            throw new Error(`managed block is missing ${endMarker}`);
          }
          changed = true;
          i = j + 1;
          continue;
        }
        if (endMarkers.has(text)) {
          throw new Error(`stray managed block end ${text}`);
        }
        if (legacyHeaders.has(text)) {
          let j = i + 1;
          while (j < lines.length && legacyExports.some((prefix) => lineText(lines[j]).startsWith(prefix))) {
            j++;
          }
          if (j > i + 1) {
            owned = true;
            changed = true;
            legacy = true;
            i = j;
            continue;
          }
        }
        kept.push(lines[i]);
        i++;
      }

      if (!owned || !changed) continue;
      const stat = fs.statSync(target);
      if (legacy) {
        const stamp = new Date().toISOString().replace(/[-:.]/g, '');
        const backupPath = `${target}.swiss-backup-${stamp}`;
        writeShellRCBackup(backupPath, data, stat);
        log(`Shell RC legacy block backed up: ${backupPath}`);
      }
      replaceShellRCAtomically(target, kept.join(''), stat);
      log(`Shell RC managed hooks removed: ${rcPath}`);
    } catch (err) {
      log(`Notice: shell RC cleanup skipped for ${rc}: ${err.message}`);
    } finally {
      if (lockFd !== undefined) {
        try {
          fs.closeSync(lockFd);
        } finally {
          try {
            fs.unlinkSync(lockPath);
          } catch (err) {
            if (err.code !== 'ENOENT') {
              log(`Notice: shell RC cleanup lock release failed for ${rc}: ${err.message}`);
            }
          }
        }
      }
    }
  }
}

function acquireShellRCLock(lockPath) {
  const deadline = Date.now() + 5000;
  let delay = 10;
  for (;;) {
    try {
      return fs.openSync(
        lockPath,
        fs.constants.O_CREAT | fs.constants.O_EXCL | fs.constants.O_WRONLY,
        0o600
      );
    } catch (err) {
      if (err.code !== 'EEXIST') {
        throw err;
      }
      if (Date.now() >= deadline) {
        throw new Error(`timed out waiting for ${lockPath}`);
      }
      sleepSync(delay);
      delay = Math.min(delay * 2, 100);
    }
  }
}

function writeShellRCBackup(backupPath, data, stat) {
  let fd = fs.openSync(backupPath, 'wx', 0o600);
  let complete = false;
  try {
    fs.writeFileSync(fd, data, 'utf8');
    fs.fchmodSync(fd, stat.mode);
    try {
      fs.fchownSync(fd, stat.uid, stat.gid);
    } catch {}
    fs.fsyncSync(fd);
    fs.closeSync(fd);
    fd = undefined;
    complete = true;
  } finally {
    if (fd !== undefined) {
      try {
        fs.closeSync(fd);
      } catch {}
    }
    if (!complete) {
      try {
        fs.unlinkSync(backupPath);
      } catch {}
    }
  }
}

function replaceShellRCAtomically(target, data, stat) {
  const dir = path.dirname(target);
  const tempPath = path.join(
    dir,
    `.${path.basename(target)}.swiss-tmp-${process.pid}-${Date.now()}-${Math.random().toString(16).slice(2)}`
  );
  let fd = fs.openSync(tempPath, 'wx', 0o600);
  let renamed = false;
  try {
    fs.writeFileSync(fd, data, 'utf8');
    fs.fchmodSync(fd, stat.mode);
    try {
      fs.fchownSync(fd, stat.uid, stat.gid);
    } catch {}
    fs.fsyncSync(fd);
    fs.closeSync(fd);
    fd = undefined;
    fs.renameSync(tempPath, target);
    renamed = true;
    const dirFd = fs.openSync(dir, 'r');
    try {
      fs.fsyncSync(dirFd);
    } finally {
      fs.closeSync(dirFd);
    }
  } finally {
    if (fd !== undefined) {
      try {
        fs.closeSync(fd);
      } catch {}
    }
    if (!renamed) {
      try {
        fs.unlinkSync(tempPath);
      } catch {}
    }
  }
}

function verifyInstallation(targetPaths) {
  log('====================================================');
  log('Verifying out-of-tree installation integrity...');
  log('====================================================');

  const { appDir, desktopDir, binDir } = targetPaths;
  const desktopFile = path.join(desktopDir, 'antigravity-swiss-knife.desktop');
  const launcherBin = path.join(appDir, 'antigravity-swiss-knife');
  const daemonBin = path.join(appDir, 'resources', 'bin', 'swiss');
  const bundledAcp = findInstalledAcpBinary(appDir);
  const cliSymlink = path.join(binDir, 'antigravity-swiss-knife');
  const acpCliSymlink = path.join(binDir, 'antigravity-acp');
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

  if (bundledAcp) {
    if (fs.existsSync(acpCliSymlink)) {
      try {
        const realTarget = fs.realpathSync(acpCliSymlink);
        const realAcp = fs.realpathSync(bundledAcp);
        if (realTarget === realAcp) {
          log(`[Verify] ACP CLI symlink resolution: PASS (${acpCliSymlink} -> ${realTarget})`);
        } else {
          console.error(`[Verify] ACP CLI symlink resolution mismatch: ${realTarget} != ${realAcp}`);
          passed = false;
        }
      } catch (err) {
        console.error(`[Verify] ACP CLI symlink error: ${err.message}`);
        passed = false;
      }
    } else {
      console.error(`[Verify] ACP CLI symlink missing: ${acpCliSymlink}`);
      passed = false;
    }
  } else {
    log('[Verify] Bundled antigravity-acp not present; ACP CLI symlink check skipped.');
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
  const nonInteractive = process.env.CI === 'true' ||
    process.env.ANTIGRAVITY_INSTALL_NO_LAUNCH === '1';
  const args = process.argv.slice(2);
  const options = {
    mode: 'auto',
    forceRebuild: false,
    verifyOnly: false,
    restart: !nonInteractive,
    launch: !nonInteractive,
    help: false
  };

  for (const arg of args) {
    if (arg === '--user') {
      options.mode = 'user';
    } else if (arg === '--system') {
      options.mode = 'system';
    } else if (arg === '--rebuild' || arg === '--force-build') {
      options.forceRebuild = true;
    } else if (arg === '--no-restart') {
      options.restart = false;
    } else if (arg === '--no-launch') {
      options.launch = false;
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
  --no-restart    Do not stop an already-running installed app
  --no-launch     Do not relaunch the installed app after installation
  --verify-only   Run integrity verification checks on current installation
  --help, -h      Display this help message
`);
}

async function main() {
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
  stageAndInstallApp(targetPaths, {
    beforeReplace: options.restart && targetPaths.type === 'user'
      ? () => stopInstalledApp(targetPaths.appDir)
      : undefined
  });
  installIcons(targetPaths);
  installDesktopEntry(targetPaths);
  installCliSymlink(targetPaths);
  installAcpCliSymlink(targetPaths);
  ensureAgyCliGateway(targetPaths);
  verifyInstallation(targetPaths);

  if (options.launch && targetPaths.type === 'user') {
    launchInstalledApp(targetPaths);
    await waitForDaemonHealth();
  } else if (targetPaths.type === 'system') {
    log('System installation does not auto-launch the desktop app.');
  }

  log('Installation completed successfully.');
}

if (require.main === module) {
  main().catch((err) => {
    console.error(`[Install Fatal Error]: ${err.message}`);
    process.exit(1);
  });
}

module.exports = {
  isMainAppCommand,
  findInstalledMainAppPids,
  stopInstalledApp,
  launchInstalledApp,
  probeDaemonHealth,
  parseCliArgs,
  resolveInstallPaths,
  stageAndInstallApp,
  installIcons,
  installDesktopEntry,
  installCliSymlink,
  installAcpCliSymlink,
  findInstalledAcpBinary,
  cleanShellRCHooks,
  verifyInstallation
};
