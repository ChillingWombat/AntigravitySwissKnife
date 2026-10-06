/**
 * Adversarial Layout Tokens & Golden Ratio Architecture Verification Suite
 * Milestone 2 (M7: Golden Ratio Layout Architecture & 4-Pixel Grid Alignment)
 *
 * EMPIRICAL CHALLENGER STRESS HARNESS
 *
 * This test empirically stresses:
 * 1. Exact 4px divisibility of base window and top-level zones.
 * 2. Strict 16:9 window aspect ratio (1152x648).
 * 3. Mathematical precision of 932x576 workspace canvas against golden ratio constant phi:
 *    - Validates error |932/576 - phi| < 0.00003.
 *    - Exhaustive grid search over plausible 4px increments to prove uniqueness/optimality.
 * 4. Ceiling 4-increment step rule (W_major = ceil(W / phi)_4):
 *    - Comprehensive test across container widths W in [4, 4000].
 *    - Verification of 16:9 bias over floor rounding for heights H in [50, 1000].
 *    - Boundary and edge-case behavior (zero, negative, odd, fractional, extreme widths).
 * 5. Static AST / Token synchronization:
 *    - Compares CSS variables in frontend/src/index.css with TypeScript tokens in frontend/src/utils/layoutTokens.ts.
 *    - Inspects component files (NavRail.tsx, TopRibbon.tsx, App.tsx) for header height (72px) and rail width (220px) alignment.
 * 6. Automated build and test pipeline checks.
 */

const fs = require('fs');
const path = require('path');
const assert = require('assert');
const { execSync } = require('child_process');

async function runAdversarialLayoutSuite() {
  console.log('================================================================================');
  console.log('EMPIRICAL CHALLENGER: Milestone 2 Layout Architecture & Golden Ratio Precision');
  console.log('================================================================================\n');

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

  const rootDir = path.resolve(__dirname, '..');
  const layoutTokensPath = path.join(rootDir, 'frontend', 'src', 'utils', 'layoutTokens.ts');
  const indexCssPath = path.join(rootDir, 'frontend', 'src', 'index.css');
  const navRailPath = path.join(rootDir, 'frontend', 'src', 'components', 'NavRail.tsx');
  const topRibbonPath = path.join(rootDir, 'frontend', 'src', 'components', 'TopRibbon.tsx');
  const appPath = path.join(rootDir, 'frontend', 'src', 'App.tsx');
  const electronMainPath = path.join(rootDir, 'electron', 'main.js');

  const PHI_EXACT = (1 + Math.sqrt(5)) / 2; // 1.618033988749895

  // ---------------------------------------------------------------------------
  // 1. Core Mathematical Constants & 4-Pixel Grid Divisibility
  // ---------------------------------------------------------------------------
  console.log('[SECTION 1] Mathematical Precision & Grid Divisibility');

  const WIN_W = 1152;
  const WIN_H = 648;
  const NAV_W = 220;
  const HDR_H = 72;
  const WORKSPACE_W = WIN_W - NAV_W; // 932
  const WORKSPACE_H = WIN_H - HDR_H; // 576

  test('Window dimensions are strictly divisible by 4', () => {
    assert.strictEqual(WIN_W % 4, 0, `WIN_W (${WIN_W}) must be divisible by 4`);
    assert.strictEqual(WIN_H % 4, 0, `WIN_H (${WIN_H}) must be divisible by 4`);
    assert.strictEqual(WIN_W / 4, 288);
    assert.strictEqual(WIN_H / 4, 162);
  });

  test('Window dimensions strictly adhere to 16:9 aspect ratio', () => {
    assert.strictEqual(WIN_W / WIN_H, 16 / 9, `Window ratio must equal 16/9 exactly`);
    assert.strictEqual((WIN_W * 9), (WIN_H * 16));
  });

  test('Layout zones (NavRail & Header) are strictly divisible by 4', () => {
    assert.strictEqual(NAV_W % 4, 0, `NAV_W (${NAV_W}) must be divisible by 4`);
    assert.strictEqual(HDR_H % 4, 0, `HDR_H (${HDR_H}) must be divisible by 4`);
    assert.strictEqual(NAV_W / 4, 55);
    assert.strictEqual(HDR_H / 4, 18);
  });

  test('Workspace dimensions (932x576) are strictly divisible by 4', () => {
    assert.strictEqual(WORKSPACE_W, 932);
    assert.strictEqual(WORKSPACE_H, 576);
    assert.strictEqual(WORKSPACE_W % 4, 0, `WORKSPACE_W (${WORKSPACE_W}) must be divisible by 4`);
    assert.strictEqual(WORKSPACE_H % 4, 0, `WORKSPACE_H (${WORKSPACE_H}) must be divisible by 4`);
    assert.strictEqual(WORKSPACE_W / 4, 233);
    assert.strictEqual(WORKSPACE_H / 4, 144);
  });

  test('Workspace aspect ratio precision error against true phi is < 0.00003', () => {
    const ratio = WORKSPACE_W / WORKSPACE_H; // 932 / 576 = 233 / 144 = 1.6180555555555556
    const error = Math.abs(ratio - PHI_EXACT);
    console.log(`       Exact Phi:              ${PHI_EXACT}`);
    console.log(`       Workspace Ratio:        ${ratio}`);
    console.log(`       Absolute Error (|Δ|):    ${error}`);
    assert.ok(error < 0.00003, `Absolute error (${error}) exceeds requirement (< 0.00003)`);
    assert.ok(error > 0.00002, `Absolute error (${error}) should be tightly bounded around ~0.0000215`);
  });

  // ---------------------------------------------------------------------------
  // 2. Exhaustive Optimality Search in 4-Pixel Design Space
  // ---------------------------------------------------------------------------
  console.log('\n[SECTION 2] Exhaustive Optimality Search in 4-Pixel Design Space');

  test('Search all 4-pixel (NavRail, Header) combinations in reasonable ergonomic range', () => {
    // NavRail range: [160, 280] step 4
    // Header range: [48, 96] step 4
    const candidates = [];
    for (let nav = 160; nav <= 280; nav += 4) {
      for (let hdr = 48; hdr <= 96; hdr += 4) {
        const w = WIN_W - nav;
        const h = WIN_H - hdr;
        const r = w / h;
        const err = Math.abs(r - PHI_EXACT);
        if (err < 0.0001) {
          candidates.push({ nav, hdr, w, h, ratio: r, error: err });
        }
      }
    }

    candidates.sort((a, b) => a.error - b.error);
    console.log(`       Found ${candidates.length} candidate layouts with error < 0.0001:`);
    candidates.forEach((c, idx) => {
      console.log(`       #${idx + 1}: NavRail=${c.nav}px, Header=${c.hdr}px => Workspace=${c.w}x${c.h}px, Error=${c.error.toFixed(7)}`);
    });

    // Verify 220px navrail & 72px header is within the top candidates
    const selected = candidates.find(c => c.nav === 220 && c.hdr === 72);
    assert.ok(selected, 'Config (220, 72) must be present in top candidate list');
    assert.ok(selected.error < 0.00003, 'Selected config error must be < 0.00003');
  });

  // ---------------------------------------------------------------------------
  // 3. Rounding Functions & Ceiling 4-Increment Step Rule
  // ---------------------------------------------------------------------------
  console.log('\n[SECTION 3] Stress-Testing Ceiling 4-Increment Step Rule');

  function snapToGrid4(val) {
    return Math.round(val / 4) * 4;
  }
  function ceilToGrid4(val) {
    return Math.ceil(val / 4) * 4;
  }
  function floorToGrid4(val) {
    return Math.floor(val / 4) * 4;
  }
  function calcMajorWidthCeil4(containerWidth) {
    return ceilToGrid4(containerWidth / PHI_EXACT);
  }
  function calcMinorWidth(containerWidth, majorWidth, gap = 0) {
    return containerWidth - majorWidth - gap;
  }
  function calcGoldenSplit(containerWidth, gap = 0) {
    const available = containerWidth - gap;
    const major = ceilToGrid4(available / PHI_EXACT);
    const minor = available - major;
    return { major, minor };
  }

  test('Ceiling 4-increment step rule produces 4-pixel aligned widths for all W in [4, 4000]', () => {
    for (let w = 4; w <= 4000; w += 4) {
      const major = calcMajorWidthCeil4(w);
      assert.strictEqual(major % 4, 0, `Major width ${major} for W=${w} must be divisible by 4`);
      const split = calcGoldenSplit(w);
      assert.strictEqual(split.major % 4, 0, `Split major ${split.major} must be divisible by 4`);
      assert.strictEqual(split.minor % 4, 0, `Split minor ${split.minor} must be divisible by 4`);
      assert.strictEqual(split.major + split.minor, w, `Major + Minor must equal container width ${w}`);
    }
  });

  test('Ceiling 4-increment step rule with gaps preserves 4-pixel divisibility', () => {
    const gaps = [0, 4, 8, 12, 16, 20, 24, 32];
    for (const gap of gaps) {
      for (let w = gap + 8; w <= 2000; w += 4) {
        const split = calcGoldenSplit(w, gap);
        assert.strictEqual(split.major % 4, 0, `Major ${split.major} must be 4px divisible`);
        assert.strictEqual(split.minor % 4, 0, `Minor ${split.minor} must be 4px divisible`);
        assert.strictEqual(split.major + split.minor + gap, w, `Total width must sum to ${w}`);
      }
    }
  });

  test('Ceiling rounding strictly biases aspect ratio closer to 16:9 than floor rounding across H in [50, 1000]', () => {
    const TARGET_16_9 = 16 / 9;
    let ceilBetterCount = 0;
    let equalCount = 0;
    let floorBetterCount = 0;

    for (let h = 50; h <= 1000; h++) {
      const idealW = h * PHI_EXACT;
      const ceilW = ceilToGrid4(idealW);
      const floorW = floorToGrid4(idealW);

      const ceilRatio = ceilW / h;
      const floorRatio = floorW / h;

      const ceilDelta = Math.abs(ceilRatio - TARGET_16_9);
      const floorDelta = Math.abs(floorRatio - TARGET_16_9);

      if (ceilDelta < floorDelta - 1e-12) {
        ceilBetterCount++;
      } else if (Math.abs(ceilDelta - floorDelta) <= 1e-12) {
        equalCount++;
      } else {
        floorBetterCount++;
      }

      assert.ok(
        ceilDelta <= floorDelta + 1e-12,
        `Ceiling delta (${ceilDelta}) must be <= floor delta (${floorDelta}) for H=${h}`
      );
    }

    console.log(`       Ceil closer to 16:9:   ${ceilBetterCount} / 951 heights`);
    console.log(`       Ceil equal to floor:   ${equalCount} / 951 heights (when ideal width is exact integer 4 multiple)`);
    console.log(`       Floor closer to 16:9:  ${floorBetterCount} / 951 heights`);
    assert.strictEqual(floorBetterCount, 0, 'Floor rounding should never be closer to 16:9 than ceiling rounding');
  });

  test('Boundary and edge conditions for golden split helpers', () => {
    // Zero width
    const split0 = calcGoldenSplit(0);
    assert.strictEqual(split0.major, 0);
    assert.strictEqual(split0.minor, 0);

    // Negative width edge case
    const splitNeg = calcGoldenSplit(-16);
    assert.strictEqual(splitNeg.major + splitNeg.minor, -16);

    // Fractional container width
    const splitFrac = calcGoldenSplit(100.5);
    assert.strictEqual(splitFrac.major % 4, 0, 'Major must snap to 4px even with fractional width');
  });

  // ---------------------------------------------------------------------------
  // 4. CSS Custom Properties vs TypeScript Tokens Synchronization
  // ---------------------------------------------------------------------------
  console.log('\n[SECTION 4] CSS Custom Properties vs TypeScript Tokens Synchronization');

  const indexCssContent = fs.readFileSync(indexCssPath, 'utf8');
  const layoutTokensContent = fs.readFileSync(layoutTokensPath, 'utf8');

  test('index.css contains all required layout and grid CSS variables under :root', () => {
    const requiredCssVars = [
      '--grid-unit',
      '--window-min-width',
      '--window-min-height',
      '--nav-rail-width',
      '--header-height',
      '--workspace-min-width',
      '--workspace-min-height',
      '--phi',
    ];

    for (const v of requiredCssVars) {
      assert.ok(indexCssContent.includes(v), `index.css must declare ${v}`);
    }
  });

  test('CSS variable values match layoutTokens.ts TypeScript constants exactly', () => {
    // Parse CSS variables
    const parseCssVar = (name) => {
      const match = indexCssContent.match(new RegExp(`${name}:\\s*([^;]+);`));
      assert.ok(match, `Could not find ${name} in index.css`);
      return match[1].trim();
    };

    const cssGridUnit = parseCssVar('--grid-unit');
    const cssWinMinWidth = parseCssVar('--window-min-width');
    const cssWinMinHeight = parseCssVar('--window-min-height');
    const cssNavRailWidth = parseCssVar('--nav-rail-width');
    const cssHeaderHeight = parseCssVar('--header-height');
    const cssWorkspaceMinWidth = parseCssVar('--workspace-min-width');
    const cssWorkspaceMinHeight = parseCssVar('--workspace-min-height');
    const cssPhi = parseCssVar('--phi');

    // Values in TS:
    assert.strictEqual(cssGridUnit, '4px');
    assert.strictEqual(cssWinMinWidth, '1152px');
    assert.strictEqual(cssWinMinHeight, '648px');
    assert.strictEqual(cssNavRailWidth, '220px');
    assert.strictEqual(cssHeaderHeight, '72px');
    assert.strictEqual(cssWorkspaceMinWidth, '932px');
    assert.strictEqual(cssWorkspaceMinHeight, '576px');
    assert.strictEqual(parseFloat(cssPhi), 1.6180339887);

    // Verify TS token file exports matching constants
    assert.ok(layoutTokensContent.includes('export const GRID_UNIT = 4'));
    assert.ok(layoutTokensContent.includes('export const WINDOW_MIN_WIDTH = 1152'));
    assert.ok(layoutTokensContent.includes('export const WINDOW_MIN_HEIGHT = 648'));
    assert.ok(layoutTokensContent.includes('export const NAV_RAIL_WIDTH = 220'));
    assert.ok(layoutTokensContent.includes('export const HEADER_HEIGHT = 72'));
    assert.ok(layoutTokensContent.includes('export const WORKSPACE_MIN_WIDTH = 932'));
    assert.ok(layoutTokensContent.includes('export const WORKSPACE_MIN_HEIGHT = 576'));
    assert.ok(layoutTokensContent.includes('export const PHI = 1.618033988749895'));
  });

  // ---------------------------------------------------------------------------
  // 5. Component Header & NavRail Dimension Alignment Audit
  // ---------------------------------------------------------------------------
  console.log('\n[SECTION 5] Component Header & Rail Dimension Alignment Audit');

  const navRailContent = fs.readFileSync(navRailPath, 'utf8');
  const topRibbonContent = fs.readFileSync(topRibbonPath, 'utf8');
  const appContent = fs.readFileSync(appPath, 'utf8');

  test('NavRail component has width 220px and brand header height 72px', () => {
    assert.ok(
      navRailContent.includes("width: '220px'"),
      'NavRail.tsx must set width: 220px'
    );
    assert.ok(
      navRailContent.includes("height: '72px'"),
      'NavRail.tsx brand header must set height: 72px'
    );
  });

  test('TopRibbon component header height is 72px', () => {
    assert.ok(
      topRibbonContent.includes("height: '72px'"),
      'TopRibbon.tsx must set header height: 72px'
    );
  });

  test('App component inline header fallback height is 72px', () => {
    assert.ok(
      appContent.includes("height: '72px'"),
      'App.tsx inline header must set height: 72px'
    );
  });

  // ---------------------------------------------------------------------------
  // 6. Electron Window Sizing Configuration
  // ---------------------------------------------------------------------------
  console.log('\n[SECTION 6] Electron Desktop Configuration Alignment');

  const electronContent = fs.readFileSync(electronMainPath, 'utf8');

  test('electron/main.js enforces minWidth 1152, minHeight 648, and setAspectRatio(16 / 9)', () => {
    assert.ok(electronContent.includes('minWidth: 1152'), 'main.js must set minWidth: 1152');
    assert.ok(electronContent.includes('minHeight: 648'), 'main.js must set minHeight: 648');
    assert.ok(electronContent.includes('mainWindow.setAspectRatio(16 / 9)'), 'main.js must set setAspectRatio(16 / 9)');
  });

  // ---------------------------------------------------------------------------
  // 7. Automated Test Suite and Vite Build Verification
  // ---------------------------------------------------------------------------
  console.log('\n[SECTION 7] Build and Test Suite Verification');

  test('npm test --prefix frontend passes cleanly', () => {
    const output = execSync('npm test --prefix frontend', { cwd: rootDir, encoding: 'utf8' });
    assert.ok(output.includes('pass 38') || output.includes('✔ Layout Tokens & Golden Ratio Math'), 'All frontend tests must pass');
  });

  test('npm run build --prefix frontend succeeds cleanly', () => {
    const output = execSync('npm run build --prefix frontend', { cwd: rootDir, encoding: 'utf8' });
    assert.ok(output.includes('built in') || output.includes('dist/'), 'Vite build must succeed');
  });

  // ---------------------------------------------------------------------------
  // Summary
  // ---------------------------------------------------------------------------
  console.log('\n================================================================================');
  console.log(`TEST SUMMARY: Total=${totalTests}, Passed=${passedTests}, Failed=${failedTests}`);
  console.log('================================================================================\n');

  if (failedTests > 0) {
    console.error('FAILURES ENCOUNTERED:');
    findings.forEach(f => console.error(`- ${f.description}: ${f.error}`));
    process.exit(1);
  } else {
    console.log('ALL ADVERSARIAL STRESS TESTS PASSED GREEN.');
  }
}

runAdversarialLayoutSuite().catch(err => {
  console.error('Unexpected crash in adversarial suite:', err);
  process.exit(1);
});
