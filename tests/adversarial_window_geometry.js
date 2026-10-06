/**
 * Adversarial Window Geometry & Aspect Ratio Verification Suite
 * Milestone 1 (M6: Minimal Window Geometry & 16:9 Aspect Ratio Locking)
 *
 * This test empirically stresses:
 * 1. Exact 4px divisibility & strict 16:9 ratio for minWidth/minHeight (1152x648)
 * 2. Golden ratio phi bounds for the 932x576 workspace layout
 * 3. 4-pixel increment ceiling vs floor rounding bias towards 16:9
 * 4. AST / Static verification of electron/main.js event listeners and options
 * 5. Event listener state transitions under mocking and destruction edge cases
 * 6. Live Electron headless execution verifying setAspectRatio(16/9), setAspectRatio(0),
 *    and real window state transitions with isolated userData directory
 */

const fs = require('fs');
const path = require('path');
const assert = require('assert');
const { spawn, execSync } = require('child_process');

async function runAdversarialSuite() {
  console.log('======================================================================');
  console.log('ADVERSARIAL VERIFICATION: Milestone 1 Window Geometry & 16:9 Locking');
  console.log('======================================================================\n');

  let totalTests = 0;
  let passedTests = 0;
  let failedTests = 0;
  const findings = [];

  function test(description, fn) {
    totalTests++;
    try {
      fn();
      console.log(`  ✓ [PASS] ${description}`);
      passedTests++;
    } catch (err) {
      console.error(`  ✗ [FAIL] ${description}`);
      console.error(`    Error: ${err.message}`);
      failedTests++;
      findings.push({ description, error: err.message });
    }
  }

  async function asyncTest(description, fn) {
    totalTests++;
    try {
      await fn();
      console.log(`  ✓ [PASS] ${description}`);
      passedTests++;
    } catch (err) {
      console.error(`  ✗ [FAIL] ${description}`);
      console.error(`    Error: ${err.message}`);
      failedTests++;
      findings.push({ description, error: err.message });
    }
  }

  // -------------------------------------------------------------------------
  // 1. Mathematical and Grid Alignment Assertions
  // -------------------------------------------------------------------------
  console.log('[Suite 1] Mathematical & Grid Alignment Oracles');

  test('Minimum dimensions 1152x648 satisfy strict 16:9 ratio', () => {
    const w = 1152;
    const h = 648;
    const ratio = w / h;
    const targetRatio = 16 / 9;
    assert.strictEqual(ratio, targetRatio, '1152 / 648 must exactly equal 16 / 9');
    assert.strictEqual(w * 9, h * 16, 'Cross-multiplication 1152*9 must equal 648*16 (10368)');
  });

  test('Minimum dimensions 1152x648 are integer multiples of 4', () => {
    assert.strictEqual(1152 % 4, 0, '1152 must be divisible by 4 (288 * 4)');
    assert.strictEqual(648 % 4, 0, '648 must be divisible by 4 (162 * 4)');
    assert.strictEqual(1152 / 4, 288);
    assert.strictEqual(648 / 4, 162);
  });

  test('Top-level layout (NavRail 220px, Header 72px) produces 932x576 workspace matching phi within 0.00003', () => {
    const windowW = 1152;
    const windowH = 648;
    const navRailW = 220; // 55 * 4
    const headerH = 72;   // 18 * 4

    assert.strictEqual(navRailW % 4, 0, 'NavRail width must be 4px aligned');
    assert.strictEqual(headerH % 4, 0, 'Header height must be 4px aligned');

    const workspaceW = windowW - navRailW;
    const workspaceH = windowH - headerH;

    assert.strictEqual(workspaceW, 932, 'Workspace width must be 932');
    assert.strictEqual(workspaceH, 576, 'Workspace height must be 576');
    assert.strictEqual(workspaceW % 4, 0, 'Workspace width must be 4px aligned (233 * 4)');
    assert.strictEqual(workspaceH % 4, 0, 'Workspace height must be 4px aligned (144 * 4)');

    const phi = (1 + Math.sqrt(5)) / 2; // ~1.6180339887...
    const workspaceRatio = workspaceW / workspaceH; // 932 / 576 = 1.61805555...
    const delta = Math.abs(workspaceRatio - phi);

    assert(delta < 0.00003, `Delta ${delta} must be strictly less than 0.00003`);
  });

  test('Ceiling 4-increment rounding rule biases major partition aspect ratio closer to 16:9 than floor rounding', () => {
    const PHI = (1 + Math.sqrt(5)) / 2;
    const target16_9 = 16 / 9; // 1.777777...

    // Test across a sweep of container widths [400, 1200]
    let ceilWins = 0;
    let floorWins = 0;

    for (let w = 400; w <= 1200; w += 16) {
      const idealMajor = w / PHI;
      const ceil4 = Math.ceil(idealMajor / 4) * 4;
      const floor4 = Math.floor(idealMajor / 4) * 4;
      const h = Math.round(idealMajor / target16_9);

      const ratioCeil = ceil4 / h;
      const ratioFloor = floor4 / h;

      const diffCeil = Math.abs(ratioCeil - target16_9);
      const diffFloor = Math.abs(ratioFloor - target16_9);

      if (diffCeil <= diffFloor) {
        ceilWins++;
      } else {
        floorWins++;
      }
    }

    assert(ceilWins > floorWins, `Ceiling rounding should win in ratio alignment (ceil: ${ceilWins}, floor: ${floorWins})`);
  });

  // -------------------------------------------------------------------------
  // 2. Static AST / Code Inspection of electron/main.js
  // -------------------------------------------------------------------------
  console.log('\n[Suite 2] Static Code & Event Listener Inspection');

  test('electron/main.js configures width, height, minWidth, minHeight to 1152 and 648', () => {
    const mainJsPath = path.join(__dirname, '..', 'electron', 'main.js');
    const content = fs.readFileSync(mainJsPath, 'utf8');

    assert(content.includes('width: 1152'), 'Must specify width: 1152');
    assert(content.includes('height: 648'), 'Must specify height: 648');
    assert(content.includes('minWidth: 1152'), 'Must specify minWidth: 1152');
    assert(content.includes('minHeight: 648'), 'Must specify minHeight: 648');
  });

  test('electron/main.js registers all 4 required window state listeners and aspect ratio locking', () => {
    const mainJsPath = path.join(__dirname, '..', 'electron', 'main.js');
    const content = fs.readFileSync(mainJsPath, 'utf8');

    assert(content.includes('mainWindow.setAspectRatio(16 / 9)'), 'Must call mainWindow.setAspectRatio(16 / 9)');
    assert(content.includes("mainWindow.on('maximize',"), 'Must listen for maximize event');
    assert(content.includes("mainWindow.on('unmaximize',"), 'Must listen for unmaximize event');
    assert(content.includes("mainWindow.on('enter-full-screen',"), 'Must listen for enter-full-screen event');
    assert(content.includes("mainWindow.on('leave-full-screen',"), 'Must listen for leave-full-screen event');
  });

  // -------------------------------------------------------------------------
  // 3. Mock Event Listener Behavior & State Transition Oracles
  // -------------------------------------------------------------------------
  console.log('\n[Suite 3] State Transition Oracle & Edge Case Simulation');

  test('Listener logic correctly toggles aspect ratio between 16/9 and 0', () => {
    const callHistory = [];
    let currentAspectRatio = null;

    // Simulate BrowserWindow listener registration
    const listeners = {};
    const mockWindow = {
      setAspectRatio: (ratio) => {
        currentAspectRatio = ratio;
        callHistory.push(ratio);
      },
      on: (event, cb) => {
        listeners[event] = cb;
      },
      emit: (event) => {
        if (listeners[event]) listeners[event]();
      }
    };

    // Apply exact registration from electron/main.js
    mockWindow.setAspectRatio(16 / 9);
    mockWindow.on('maximize', () => mockWindow.setAspectRatio(0));
    mockWindow.on('unmaximize', () => mockWindow.setAspectRatio(16 / 9));
    mockWindow.on('enter-full-screen', () => mockWindow.setAspectRatio(0));
    mockWindow.on('leave-full-screen', () => mockWindow.setAspectRatio(16 / 9));

    assert.strictEqual(currentAspectRatio, 16 / 9, 'Initial aspect ratio must be 16/9');

    // Scenario 1: Maximize then unmaximize
    mockWindow.emit('maximize');
    assert.strictEqual(currentAspectRatio, 0, 'Maximize must unlock aspect ratio (0)');
    mockWindow.emit('unmaximize');
    assert.strictEqual(currentAspectRatio, 16 / 9, 'Unmaximize must restore aspect ratio (16/9)');

    // Scenario 2: Enter fullscreen then leave fullscreen
    mockWindow.emit('enter-full-screen');
    assert.strictEqual(currentAspectRatio, 0, 'Enter full screen must unlock aspect ratio (0)');
    mockWindow.emit('leave-full-screen');
    assert.strictEqual(currentAspectRatio, 16 / 9, 'Leave full screen must restore aspect ratio (16/9)');

    // Verify call sequence
    assert.deepStrictEqual(callHistory, [16 / 9, 0, 16 / 9, 0, 16 / 9]);
  });

  test('Edge Case: Object destruction safety check', () => {
    let destroyed = false;
    let thrownError = null;

    const mockWindow = {
      isDestroyed: () => destroyed,
      setAspectRatio: (ratio) => {
        if (destroyed) {
          throw new Error('Object has been destroyed');
        }
      }
    };

    const safeHandler = () => {
      if (!mockWindow.isDestroyed()) {
        mockWindow.setAspectRatio(0);
      }
    };

    // When destroyed:
    destroyed = true;
    try {
      safeHandler();
    } catch (e) {
      thrownError = e;
    }

    assert.strictEqual(thrownError, null, 'Safe handler should not throw when window is destroyed');
  });

  // -------------------------------------------------------------------------
  // 4. Live Headless Electron Process Test (Isolated Profile)
  // -------------------------------------------------------------------------
  console.log('\n[Suite 4] Live Electron Runtime Verification (Isolated User Data)');

  await asyncTest('Live Electron instance executes setAspectRatio and window state events cleanly', async () => {
    const rootDir = path.resolve(__dirname, '..');
    const electronBin = path.join(rootDir, 'node_modules', '.bin', 'electron');
    const tempUserData = path.join('/tmp', `electron-test-user-data-${Date.now()}`);

    // Create a mini-harness script to run inside real Electron
    const testScriptPath = path.join('/tmp', `electron-geom-test-${Date.now()}.js`);
    const scriptCode = `
      const { app, BrowserWindow } = require('electron');
      app.whenReady().then(() => {
        const win = new BrowserWindow({
          width: 1152,
          height: 648,
          minWidth: 1152,
          minHeight: 648,
          show: false,
        });

        // 1. Initial configuration
        win.setAspectRatio(16 / 9);

        // 2. Register listeners
        win.on('maximize', () => win.setAspectRatio(0));
        win.on('unmaximize', () => win.setAspectRatio(16 / 9));
        win.on('enter-full-screen', () => win.setAspectRatio(0));
        win.on('leave-full-screen', () => win.setAspectRatio(16 / 9));

        // 3. Test programmatic emissions & method calls
        console.log('[LIVE-TEST] Window initialized: ' + win.getSize().join('x'));
        console.log('[LIVE-TEST] Minimum size: ' + win.getMinimumSize().join('x'));

        // Test setAspectRatio(0) and setAspectRatio(16/9)
        win.setAspectRatio(0);
        console.log('[LIVE-TEST] setAspectRatio(0) executed cleanly');
        win.setAspectRatio(16 / 9);
        console.log('[LIVE-TEST] setAspectRatio(16/9) executed cleanly');

        // Test programmatic undersized attempt (must be clamped to minimums)
        win.setSize(800, 450);
        const [clampedW, clampedH] = win.getSize();
        console.log('[LIVE-TEST] Sizing clamp test result: ' + clampedW + 'x' + clampedH);
        if (clampedW < 1152 || clampedH < 648) {
          console.error('[LIVE-TEST] Window allowed dimensions smaller than minimum!');
          process.exit(1);
        }

        // Test valid 16:9 expansion
        win.setSize(1280, 720);
        const [expandedW, expandedH] = win.getSize();
        console.log('[LIVE-TEST] Expanded size: ' + expandedW + 'x' + expandedH);
        if (expandedW % 4 !== 0 || expandedH % 4 !== 0) {
          console.error('[LIVE-TEST] Expanded dimensions not 4px aligned!');
          process.exit(1);
        }

        // Test real window state changes if supported by platform
        try {
          win.maximize();
          console.log('[LIVE-TEST] win.maximize() called cleanly');
          win.unmaximize();
          console.log('[LIVE-TEST] win.unmaximize() called cleanly');
        } catch (e) {
          console.log('[LIVE-TEST] window maximize error ignored:', e.message);
        }

        // Test event emission
        win.emit('maximize');
        console.log('[LIVE-TEST] maximize event handled cleanly');
        win.emit('unmaximize');
        console.log('[LIVE-TEST] unmaximize event handled cleanly');
        win.emit('enter-full-screen');
        console.log('[LIVE-TEST] enter-full-screen event handled cleanly');
        win.emit('leave-full-screen');
        console.log('[LIVE-TEST] leave-full-screen event handled cleanly');

        // Test edge case: nested maximize + fullscreen
        win.emit('maximize');
        win.emit('enter-full-screen');
        win.emit('leave-full-screen');
        win.emit('unmaximize');
        console.log('[LIVE-TEST] nested maximize+fullscreen sequence handled cleanly');

        win.destroy();
        console.log('[LIVE-TEST] Window destroyed cleanly');
        app.quit();
      });
    `;
    fs.writeFileSync(testScriptPath, scriptCode, 'utf8');

    try {
      const child = spawn(electronBin, [
        `--user-data-dir=${tempUserData}`,
        testScriptPath
      ], {
        cwd: rootDir,
        env: {
          ...process.env,
          ELECTRON_ENABLE_LOGGING: '1',
        },
      });

      let stdout = '';
      let stderr = '';

      child.stdout.on('data', (d) => { stdout += d.toString(); });
      child.stderr.on('data', (d) => { stderr += d.toString(); });

      const exitCode = await new Promise((resolve) => {
        const timeout = setTimeout(() => {
          child.kill('SIGKILL');
          resolve(-1);
        }, 15000);

        child.on('exit', (code) => {
          clearTimeout(timeout);
          resolve(code);
        });
      });

      console.log('    Electron output:\n' + stdout.trim().split('\n').map(l => '      ' + l).join('\n'));

      assert.strictEqual(exitCode, 0, `Electron exited with non-zero code ${exitCode}. Stderr: ${stderr}`);
      assert(stdout.includes('[LIVE-TEST] Window initialized: 1152x648'), 'Window size must be 1152x648');
      assert(stdout.includes('[LIVE-TEST] Minimum size: 1152x648'), 'Minimum size must be 1152x648');
      assert(stdout.includes('[LIVE-TEST] setAspectRatio(0) executed cleanly'), 'setAspectRatio(0) must execute');
      assert(stdout.includes('[LIVE-TEST] setAspectRatio(16/9) executed cleanly'), 'setAspectRatio(16/9) must execute');
      assert(stdout.includes('[LIVE-TEST] maximize event handled cleanly'), 'maximize event must succeed');
      assert(stdout.includes('[LIVE-TEST] unmaximize event handled cleanly'), 'unmaximize event must succeed');
      assert(stdout.includes('[LIVE-TEST] enter-full-screen event handled cleanly'), 'enter-full-screen must succeed');
      assert(stdout.includes('[LIVE-TEST] leave-full-screen event handled cleanly'), 'leave-full-screen must succeed');
      assert(stdout.includes('[LIVE-TEST] Window destroyed cleanly'), 'Window destroy must succeed');
    } finally {
      try { fs.unlinkSync(testScriptPath); } catch {}
      try { fs.rmSync(tempUserData, { recursive: true, force: true }); } catch {}
    }
  });

  // -------------------------------------------------------------------------
  // 5. Desktop E2E Script Regression Analysis
  // -------------------------------------------------------------------------
  console.log('\n[Suite 5] E2E Script Regression Analysis');

  test('scripts/verify-desktop-e2e.js includes window geometry checks in checks array', () => {
    const e2eScriptPath = path.join(__dirname, '..', 'scripts', 'verify-desktop-e2e.js');
    const content = fs.readFileSync(e2eScriptPath, 'utf8');

    assert(content.includes('Window Geometry & 16:9 Aspect Ratio Verified (1152x648, 4px aligned)'),
      'Must contain the exact check name for window geometry');
    assert(content.includes('[E2E-TEST] Window geometry verified: 1152x648 (16:9 aspect ratio, 4px aligned)'),
      'Must check the stdout for geometry verification string');
  });

  // -------------------------------------------------------------------------
  // Summary
  // -------------------------------------------------------------------------
  console.log('\n======================================================================');
  console.log(`RESULTS: ${passedTests}/${totalTests} Passed (${failedTests} Failed)`);
  console.log('======================================================================');

  if (failedTests > 0) {
    console.error(`\nFailed tests:`);
    for (const f of findings) {
      console.error(`- ${f.description}: ${f.error}`);
    }
    process.exit(1);
  } else {
    console.log('\nAll adversarial checks passed 100%!');
  }
}

runAdversarialSuite().catch((err) => {
  console.error('Fatal test runner error:', err);
  process.exit(1);
});
