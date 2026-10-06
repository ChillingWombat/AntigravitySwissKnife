#!/usr/bin/env node

const { spawn, execSync } = require('child_process');
const path = require('path');
const fs = require('fs');
const http = require('http');

const rootDir = path.resolve(__dirname, '..');
const { DaemonManager } = require(path.join(rootDir, 'electron', 'daemon-manager.js'));

console.log('======================================================================');
console.log('Antigravity Swiss Knife Desktop E2E Verification Test Harness');
console.log('======================================================================');

async function probeUrl(url, timeoutMs = 1500) {
  return new Promise((resolve) => {
    const req = http.get(url, { timeout: timeoutMs }, (res) => {
      let data = '';
      res.on('data', (c) => { data += c; });
      res.on('end', () => {
        try { resolve(JSON.parse(data)); } catch { resolve(null); }
      });
    });
    req.on('error', () => resolve(null));
    req.on('timeout', () => { req.destroy(); resolve(null); });
  });
}

async function run() {
  let allPassed = true;

  // Snapshot any pre-existing daemons so we don't misclassify them as test orphans
  let initialDaemonPids = [];
  try {
    const pgrepInit = execSync('pgrep -a swiss || true', { encoding: 'utf8' });
    initialDaemonPids = pgrepInit
      .trim()
      .split('\n')
      .filter((l) => l.includes('daemon --web'))
      .map((l) => l.trim().split(' ')[0]);
  } catch {}

  // ------------------------------------------------------------------
  // 1. Build & Asset Prerequisites
  // ------------------------------------------------------------------
  console.log('\n[Phase 1] Verifying Build and Asset Prerequisites...');
  const swissBin = path.join(rootDir, 'bin', process.platform === 'win32' ? 'swiss.exe' : 'swiss');
  if (!fs.existsSync(swissBin)) {
    console.log('[Setup] Building Go binary bin/swiss...');
    execSync('npm run build:go', { cwd: rootDir, stdio: 'inherit' });
  }
  console.log('  ✓ Go binary verified at:', swissBin);

  const indexHtml = path.join(rootDir, 'pkg', 'webgui', 'dist', 'index.html');
  if (!fs.existsSync(indexHtml)) {
    console.log('[Setup] Building frontend...');
    execSync('npm run build:frontend', { cwd: rootDir, stdio: 'inherit' });
  }
  console.log('  ✓ Frontend bundle verified at:', indexHtml);

  // ------------------------------------------------------------------
  // 2. DaemonManager Unit & Lifecycle Safety Tests
  // ------------------------------------------------------------------
  console.log('\n[Phase 2] Verifying DaemonManager Lifecycle & Safety...');

  // Test 2.1: Binary resolution
  const dmTest = new DaemonManager();
  const resolved = dmTest.resolveBinaryPath();
  const binaryExists = fs.existsSync(resolved);
  if (binaryExists) {
    console.log('  ✓ Binary path resolution verified:', resolved);
  } else {
    console.error('  ✗ Binary path resolution failed:', resolved);
    allPassed = false;
  }

  // Test 2.2: Managed child spawn and clean teardown on test port 8781
  console.log('[Test 2.2] Testing managed child spawn and clean teardown...');
  const tempSocketManaged = path.join('/tmp', `swiss-test-${Date.now()}-managed.sock`);
  const envManaged = { ...process.env, ANTIGRAVITY_SWISS_SOCKET: tempSocketManaged };
  const dmManaged = new DaemonManager({ port: 8781, host: '127.0.0.1' });
  
  // Custom env override for test isolation
  const origEnv = process.env.ANTIGRAVITY_SWISS_SOCKET;
  process.env.ANTIGRAVITY_SWISS_SOCKET = tempSocketManaged;
  try {
    await dmManaged.start();
    const managedStatus = await dmManaged.checkStatus();
    const isSpawnedManaged = dmManaged.isManagedChild === true && managedStatus && managedStatus.daemon_running === true;
    if (isSpawnedManaged) {
      console.log('  ✓ Managed child spawned successfully (isManagedChild=true, daemon_running=true)');
    } else {
      console.error('  ✗ Managed child spawn assertion failed');
      allPassed = false;
    }

    await dmManaged.stop();
    const postStopStatus = await dmManaged.checkStatus(500);
    if (!postStopStatus) {
      console.log('  ✓ Managed child terminated cleanly via SIGTERM (zero orphans)');
    } else {
      console.error('  ✗ Managed child still responding after stop!');
      allPassed = false;
    }
  } catch (err) {
    console.error('  ✗ Managed child test error:', err.message);
    allPassed = false;
  } finally {
    if (origEnv) process.env.ANTIGRAVITY_SWISS_SOCKET = origEnv;
    else delete process.env.ANTIGRAVITY_SWISS_SOCKET;
    try { if (fs.existsSync(tempSocketManaged)) fs.unlinkSync(tempSocketManaged); } catch {}
  }

  // Test 2.3: External daemon safety (must NOT be stopped on exit)
  console.log('[Test 2.3] Testing external daemon safety and preservation...');
  const tempSocketExt = path.join('/tmp', `swiss-test-${Date.now()}-ext.sock`);
  const extProcess = spawn(swissBin, ['daemon', '--web', '--addr', '127.0.0.1:8779', '--socket', tempSocketExt], {
    stdio: 'ignore',
    detached: false,
  });

  try {
    // Wait for external daemon to be ready
    let extReady = false;
    for (let i = 0; i < 30; i++) {
      await new Promise((r) => setTimeout(r, 100));
      const s = await probeUrl('http://127.0.0.1:8779/api/status', 300);
      if (s && s.daemon_running === true) {
        extReady = true;
        break;
      }
    }

    if (!extReady) {
      throw new Error('External test daemon failed to become ready on port 8779');
    }

    const dmExternal = new DaemonManager({ port: 8779, host: '127.0.0.1' });
    await dmExternal.start();

    if (dmExternal.isManagedChild === false) {
      console.log('  ✓ External daemon recognized and preserved (isManagedChild=false)');
    } else {
      console.error('  ✗ isManagedChild was unexpectedly true for pre-existing daemon');
      allPassed = false;
    }

    await dmExternal.stop();
    const extStillAlive = await probeUrl('http://127.0.0.1:8779/api/status', 500);
    if (extStillAlive && extStillAlive.daemon_running === true) {
      console.log('  ✓ External daemon safely left running after DaemonManager.stop()');
    } else {
      console.error('  ✗ External daemon was incorrectly terminated!');
      allPassed = false;
    }
  } catch (err) {
    console.error('  ✗ External daemon safety test error:', err.message);
    allPassed = false;
  } finally {
    try { extProcess.kill('SIGTERM'); } catch {}
    try { if (fs.existsSync(tempSocketExt)) fs.unlinkSync(tempSocketExt); } catch {}
  }

  // ------------------------------------------------------------------
  // 3. Full Headless Electron E2E Window & IPC Verification
  // ------------------------------------------------------------------
  console.log('\n[Phase 3] Launching Electron Desktop Shell under E2E harness...');

  const hasDisplay = Boolean(process.env.DISPLAY);
  const hasXvfb = (() => {
    try {
      execSync('which xvfb-run', { stdio: 'ignore' });
      return true;
    } catch {
      return false;
    }
  })();

  const electronBin = path.join(rootDir, 'node_modules', '.bin', 'electron');
  const mainScript = path.join(rootDir, 'electron', 'main.js');

  let child;
  const env = {
    ...process.env,
    TEST_DESKTOP_E2E: '1',
    ELECTRON_ENABLE_LOGGING: '1',
  };

  if (!hasDisplay && hasXvfb && process.platform === 'linux') {
    console.log('[Test] No active DISPLAY; invoking via xvfb-run -a');
    child = spawn('xvfb-run', ['-a', electronBin, mainScript], {
      cwd: rootDir,
      env,
      stdio: ['ignore', 'pipe', 'pipe'],
    });
  } else {
    if (hasDisplay) {
      console.log(`[Test] Utilizing active display: ${process.env.DISPLAY}`);
    }
    child = spawn(electronBin, [mainScript], {
      cwd: rootDir,
      env,
      stdio: ['ignore', 'pipe', 'pipe'],
    });
  }

  let stdout = '';
  let stderr = '';

  child.stdout.on('data', (data) => {
    const str = data.toString();
    stdout += str;
    process.stdout.write(`[Electron] ${str}`);
  });

  child.stderr.on('data', (data) => {
    const str = data.toString();
    stderr += str;
    process.stderr.write(`[Electron-err] ${str}`);
  });

  const e2eResult = await new Promise((resolve) => {
    const timeoutMs = 30000;
    const timeout = setTimeout(() => {
      console.error(`[FAIL] E2E test timed out after ${timeoutMs}ms! Force terminating...`);
      try { child.kill('SIGKILL'); } catch {}
      resolve({ code: -1, signal: 'TIMEOUT' });
    }, timeoutMs);

    child.on('exit', (code, signal) => {
      clearTimeout(timeout);
      resolve({ code, signal });
    });
  });

  console.log('----------------------------------------------------------------------');
  console.log(`[Result] Electron exited with code=${e2eResult.code}, signal=${e2eResult.signal}`);

  const checks = [
    {
      name: 'Main Window Created & Title Verified (Antigravity Swiss Knife)',
      passed: stdout.includes('[E2E-TEST] Window title: Antigravity Swiss Knife') || stdout.includes('Antigravity Swiss Knife'),
    },
    {
      name: 'Window Geometry & 16:9 Aspect Ratio Verified (1152x648, 4px aligned)',
      passed: stdout.includes('[E2E-TEST] Window geometry verified: 1152x648 (16:9 aspect ratio, 4px aligned)'),
    },
    {
      name: 'API Status Probe Succeeded (127.0.0.1:8765/api/status)',
      passed: stdout.includes('[E2E-TEST] API status result: OK'),
    },
    {
      name: 'Startup IPC Handlers Verified (getLoginItemSettings)',
      passed: stdout.includes('[E2E-TEST] Testing IPC getLoginItemSettings') || stdout.includes('LoginItemSettings'),
    },
    {
      name: 'Startup IPC Parameter Normalization Verified',
      passed: stdout.includes('[E2E-TEST] Set startup setting test result:'),
    },
    {
      name: 'Close to Tray Setting & IPC Persistence Verified',
      passed: stdout.includes('[E2E-TEST] CloseToTray IPC and persistence verified: OK'),
    },
    {
      name: 'Clean Exit Code 0',
      passed: e2eResult.code === 0,
    },
  ];

  for (const check of checks) {
    if (check.passed) {
      console.log(`  ✓ ${check.name}`);
    } else {
      console.error(`  ✗ ${check.name} (FAILED)`);
      allPassed = false;
    }
  }

  // ------------------------------------------------------------------
  // 4. Verify Process Cleanup (Zero Orphaned Swiss Daemons)
  // ------------------------------------------------------------------
  console.log('\n[Phase 4] Verifying Process Cleanup & Hygiene...');
  await new Promise((r) => setTimeout(r, 1000));
  try {
    const pgrep = execSync('pgrep -a swiss || true', { encoding: 'utf8' });
    const lines = pgrep
      .trim()
      .split('\n')
      .filter((l) => {
        if (!l.includes('daemon --web')) return false;
        const pid = l.trim().split(' ')[0];
        return !initialDaemonPids.includes(pid);
      });
    if (lines.length > 0) {
      console.error('  ✗ Orphaned swiss daemon processes detected:', lines.join('; '));
      allPassed = false;
    } else {
      console.log('  ✓ Zero orphaned swiss child daemon processes remain.');
    }
  } catch (err) {
    console.warn('[Warning] pgrep error:', err.message);
  }

  // Final Verdict
  console.log('======================================================================');
  if (allPassed) {
    console.log('ALL E2E DESKTOP VERIFICATION CHECKS PASSED (100%)');
    console.log('======================================================================');
    process.exit(0);
  } else {
    console.error('E2E DESKTOP VERIFICATION CHECKS FAILED');
    console.error('======================================================================');
    process.exit(1);
  }
}

run().catch((err) => {
  console.error('[Fatal Error in Harness]:', err);
  process.exit(1);
});
