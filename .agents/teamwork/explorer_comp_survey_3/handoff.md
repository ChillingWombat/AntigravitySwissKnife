# Component, Gadgets & Automated Test Suite Survey Report (`handoff.md`)

**Agent**: `explorer_comp_survey_3` (Component & Test Explorer)  
**Target Project**: Antigravity Swiss Knife  
**Timestamp**: 2026-10-06T03:53:00Z  
**Directory**: `/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_comp_survey_3`  
**Parent**: `orchestrator_3` (`22e8a004-e0c2-41d4-92e0-45bd204fac17`)

---

## 1. Observation

### 1.1 Test Infrastructure & Build Diagnostics
1. **Test Runner Configuration in `frontend/package.json`**:
   - Line 9: `"test": "node --test src/**/*.test.ts"`
   - Line 8: `"build": "tsc -b && vite build"`
   - Line 10: `"lint": "oxlint"`
   - Node runtime: `v24.16.0` (executes TypeScript tests natively via `node --test` with sub-100ms startup).
2. **Current Test Execution**:
   Command: `npm test --prefix frontend`
   ```
   > frontend@0.0.0 test
   > node --test src/**/*.test.ts

   ℹ tests 30
   ℹ suites 12
   ℹ pass 30
   ℹ fail 0
   ℹ duration_ms 80.053553
   ```
   All 30 existing unit tests pass:
   - `src/utils/modelExtraction.test.ts` (12 tests)
   - `src/utils/modelFilter.test.ts` (7 tests)
   - `src/utils/schedule.test.ts` (7 tests)
   - `src/utils/totp.test.ts` (4 tests)
3. **TypeScript & Bundler Configuration**:
   - `frontend/tsconfig.json` references `tsconfig.app.json` and `tsconfig.node.json`.
   - `frontend/tsconfig.app.json`:
     - Line 4: `"target": "es2023"`
     - Line 7: `"types": ["vite/client", "node"]`
     - Line 12: `"moduleResolution": "bundler"`
     - Line 13: `"allowImportingTsExtensions": true` (allows explicit `.ts` imports in test and source code).
     - Line 14: `"verbatimModuleSyntax": true`
     - Line 16: `"noEmit": true`
     - Line 25: `"include": ["src"]`
   - `frontend/vite.config.ts`: Vite `^8.3.0` with `@vitejs/plugin-react` (`^6.1.1`), bundling into `../pkg/webgui/dist`.
4. **Current Build Status**:
   `npm run build --prefix frontend` compiles cleanly with output:
   `../pkg/webgui/dist/index.html (0.51 kB)`, `assets/index-AEL7Q-g5.css (3.18 kB)`, `assets/index-fwJK2IkF.js (629.98 kB)`.

---

### 1.2 Viewport Geometry & Top-Level Layout Allocation
- **Minimal Window Target**: $1152 \times 648$ px ($16:9$, exact 4-pixel divisibility: $1152 = 288 \times 4$, $648 = 162 \times 4$).
- **Navigation Rail**:
  - `frontend/src/components/NavRail.tsx` line 33: `width: '220px'` ($55 \times 4$).
  - `frontend/src/components/NavRail.tsx` line 46: `height: '64px'`.
- **Top Header Ribbon**:
  - `frontend/src/components/TopRibbon.tsx` line 26: `height: '64px'`.
  - `frontend/src/App.tsx` line 115: `height: '64px'`.
  - *Observation*: Header height is currently `64px` ($16 \times 4$), whereas requirement R2 specifies `72px` ($18 \times 4$).
- **Content Workspace Calculations**:
  - Content width at base window: $1152 - 220 = 932$ px ($233 \times 4$).
  - Content height at base window: $648 - 72 = 576$ px ($144 \times 4$).
  - Content workspace aspect ratio: $932 / 576 \approx 1.618055555...$ ($\phi \approx 1.6180339887...$, error $\approx 0.0000215 < 0.00003$).
  - Inner workspace width inside `<main>` (padding `24px` on each side, `App.tsx` line 327):
    $W_{\text{inner}} = 932 - 48 = 884$ px ($221 \times 4$).
    $H_{\text{inner}} = 576 - 48 = 528$ px ($132 \times 4$).

---

### 1.3 Component Sizing, Gadgets, and 1152×648 Viewport Overflow
Direct inspection across `frontend/src/components/` and `frontend/src/pages/`:

#### A. Quota Dashboard Fleet Table Horizontal Overflow Bug
- `frontend/src/pages/QuotaDashboardPage.tsx` line 658–659:
  ```tsx
  <div style={{ overflowX: 'auto', width: '100%' }}>
    <table style={{ width: '100%', minWidth: '960px', borderCollapse: 'collapse', tableLayout: 'fixed' }}>
  ```
- **Direct Cause**: The table enforces `minWidth: '960px'`. Because the available container width at base window size is $884$ px, $960\text{ px} > 884\text{ px}$, which unconditionally spawns a horizontal scrollbar at the base $1152 \times 648$ viewport!
- **Column Width Allocations** (lines 665–785):
  - Account: `minWidth: '220px'`, `width: '26%'`
  - Plan: `minWidth: '85px'`, `width: '90px'`
  - 5-Hour Quota: `minWidth: '160px'`, `width: '22%'`
  - Weekly Quota: `minWidth: '160px'`, `width: '22%'`
  - AI Credits: `minWidth: '100px'`, `width: '110px'`
  - Priority: `minWidth: '75px'`, `width: '80px'`
  - Action: `minWidth: '90px'`, `width: '95px'`
  - Sum of column minimums: $220 + 85 + 160 + 160 + 100 + 75 + 90 = 890$ px ($890 > 884$).

#### B. Circular Gauges Grid Alignment
- `frontend/src/components/CircularGauge.tsx` lines 14–15:
  - Default `size = 130` ($130 / 4 = 32.5$, unaligned).
  - Default `strokeWidth = 11` ($11 / 4 = 2.75$, unaligned).
- Usages in `frontend/src/pages/QuotaDashboardPage.tsx` lines 538, 544, 562, 568:
  - Dual-mode: `size={74}`, `strokeWidth={7}` ($74 / 4 = 18.5$, unaligned; $7 / 4 = 1.75$, unaligned).
- Usages in `frontend/src/pages/CustomModelsPage.tsx` line 788:
  - `size={86}`, `strokeWidth={7}` ($86 / 4 = 21.5$, unaligned; $7 / 4 = 1.75$, unaligned).
- Usage in `frontend/src/pages/MfaVaultPage.tsx` lines 119–127:
  - Ring size `width="56"`, `height="56"` ($56 = 14 \times 4$), but `strokeWidth="5"` ($5 / 4 = 1.25$, unaligned).

#### C. Horizontal Quota Bar
- `frontend/src/components/HorizontalQuotaBar.tsx` lines 12–13, 31:
  - Default `height = 9` ($9 / 4 = 2.25$, unaligned).
  - Default `maxWidth = 160` ($160 = 40 \times 4$, aligned).
  - `gap: '6px'` ($6 / 4 = 1.5$, unaligned).

#### D. Toggle Switches
- `frontend/src/components/ToggleSwitch.tsx` lines 25–28:
  - Small (`sm`): `width = 32` ($8 \times 4$), `height = 18` ($18 / 4 = 4.5$), `knobSize = 14` ($14 / 4 = 3.5$).
  - Medium (`md`): `width = 40` ($10 \times 4$), `height = 22` ($22 / 4 = 5.5$), `knobSize = 18` ($18 / 4 = 4.5$).

#### E. Dashboard Summary Cards & Grids
- `QuotaDashboardPage.tsx` line 387:
  - `gridTemplateColumns: '1.2fr 1.2fr'`, `gap: '16px'`.
  - Available width: $884 - 16 = 868$ px.
  - Symmetrical split: $434$ px each ($434 / 4 = 108.5$).
  - Golden ratio split: $W_{\text{major}} = \lceil 868 / \phi \rceil_4 = 540$ px ($135 \times 4$); $W_{\text{minor}} = 328$ px ($82 \times 4$).
- `TokenMonitorPage.tsx` lines 393–396, 404:
  - `gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))'`, `gap: '14px'` ($14 / 4 = 3.5$), card `padding: '18px'` ($18 / 4 = 4.5$).
- `BrainCachePage.tsx` line 85:
  - `gridTemplateColumns: '1fr 1fr 1fr'`, `gap: '16px'`.
  - At $884$ px inner width: $(884 - 32) / 3 = 284$ px per card ($284 = 71 \times 4$, aligned).
- `CustomModelsPage.tsx` line 650:
  - `gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))'`, `gap: '16px'`.
- `FeaturePluginsPage.tsx`:
  - Browser Preview (line 438): `gridTemplateColumns: '1fr 340px'`, `gap: '16px'`.
  - File Explorer (line 856): `gridTemplateColumns: '280px 1fr'`, `gap: '16px'`.

#### F. Modal Dialog Containers
All major modals use 4-pixel aligned widths:
- `AccountDetailModal.tsx` line 164: `width: '580px'` ($145 \times 4$), `padding: '28px'` ($7 \times 4$). Internal gaps at lines 341, 642, 727 currently use `gap: '14px'`.
- `SecurityReportModal.tsx` line 81: `width: '680px'` ($170 \times 4$), `padding: '28px'` ($7 \times 4$).
- `QuotaDashboardPage.tsx`:
  - Error Modal (line 1064): `width: '460px'` ($115 \times 4$), `padding: '24px'` ($6 \times 4$).
  - Discovered Accounts Modal (line 1279): `width: '540px'` ($135 \times 4$), `padding: '24px'` ($6 \times 4$).
- `CustomModelsPage.tsx`:
  - Add/Edit Modal (line 943): `width: '600px'` ($150 \times 4$), `padding: '24px'` ($6 \times 4$).
  - Delete Confirm Modal (line 1494): `width: '440px'` ($110 \times 4$), `padding: '24px'` ($6 \times 4$).
- `ScheduledTemplatesPage.tsx`:
  - Deploy Modal (line 497): `maxWidth: '680px'` ($170 \times 4$), `padding: '28px'` ($7 \times 4$).
  - Edit Modal (line 768): `maxWidth: '680px'` ($170 \times 4$), `padding: '28px'` ($7 \times 4$).
  - Delete Modal (line 937): `maxWidth: '440px'` ($110 \times 4$), `padding: '24px'` ($6 \times 4$).
- `TokenMonitorPage.tsx` line 1046: `width: '420px'` ($105 \times 4$), `padding: '24px'` ($6 \times 4$).
- `BrainCachePage.tsx` line 198: `width: '440px'` ($110 \times 4$), `padding: '24px'` ($6 \times 4$).
- `ArchivedProjectsPage.tsx` line 470: `maxWidth: '440px'` ($110 \times 4$), `padding: '24px'` ($6 \times 4$).
- `UtilitiesPage.tsx` line 691: `width: '540px'` ($135 \times 4$), `padding: '24px'` ($6 \times 4$).

---

## 2. Logic Chain

### Step 2.1: Mathematical Proof of the Ceiling 4-Increment Step Rule
1. **Target Window Aspect Ratio**: $T = 16 / 9 \approx 1.7777778$.
2. **Golden Ratio Constant**: $\phi = (1 + \sqrt{5}) / 2 \approx 1.618034$.
3. **Fundamental Inequality**:
   $$T = \frac{16}{9} > \phi \approx 1.618034 \quad (1.77778 > 1.618034)$$
4. **Aspect Ratio of a Partitioned Element**:
   Let container width be $W$. The continuous golden major width is $W^* = W / \phi$.
   Quantizing to the 4-pixel grid yields:
   $$W_{\text{ceil}} = \lceil W^* \rceil_4 = \lceil (W / \phi) / 4 \rceil \times 4$$
   $$W_{\text{floor}} = \lfloor W^* \rfloor_4 = \lfloor (W / \phi) / 4 \rfloor \times 4$$
   By definition of ceiling and floor:
   $$W_{\text{floor}} \le W^* \le W_{\text{ceil}}$$
5. **Partition Split Ratio**:
   The ratio of major to minor partition is:
   $$R(W_m) = \frac{W_m}{W - W_m}$$
   The derivative $\frac{dR}{dW_m} = \frac{W}{(W - W_m)^2} > 0$ is strictly increasing.
   Therefore:
   $$R(W_{\text{floor}}) \le R(W^*) = \phi \le R(W_{\text{ceil}})$$
   Since target $T = 1.77778 > \phi$, increasing $R$ above $\phi$ moves it closer to $T$, while decreasing $R$ below $\phi$ moves it further away from $T$:
   $$|R(W_{\text{ceil}}) - T| < |R(W_{\text{floor}}) - T|$$
6. **Empirical Verification on Codebase Values**:
   - For $W = 884$ px (workspace inner width):
     - $W^* = 884 / 1.618034 = 546.34$
     - $W_{\text{ceil}} = 548$ px ($137 \times 4$), $W_{\text{minor}} = 336$ px ($84 \times 4$):
       $R_{\text{ceil}} = 548 / 336 = 1.63095 \implies |1.63095 - 1.77778| = 0.14683$
     - $W_{\text{floor}} = 544$ px ($136 \times 4$), $W_{\text{minor}} = 340$ px ($85 \times 4$):
       $R_{\text{floor}} = 544 / 340 = 1.60000 \implies |1.60000 - 1.77778| = 0.17778$
     - Ratio error with ceiling: $0.14683 < 0.17778$. Ceiling rounding is strictly closer to 16:9!
   - For $W = 868$ px (two cards with 16px gap):
     - $W^* = 868 / 1.618034 = 536.45$
     - $W_{\text{ceil}} = 540$ px ($135 \times 4$), $W_{\text{minor}} = 328$ px ($82 \times 4$):
       $R_{\text{ceil}} = 540 / 328 = 1.64634 \implies |1.64634 - 1.77778| = 0.13144$
     - $W_{\text{floor}} = 536$ px ($134 \times 4$), $W_{\text{minor}} = 332$ px ($83 \times 4$):
       $R_{\text{floor}} = 536 / 332 = 1.61446 \implies |1.61446 - 1.77778| = 0.16332$
     - Ratio error with ceiling: $0.13144 < 0.16332$. Ceiling rounding wins definitively.

---

### Step 2.2: Resolving the 960px Fleet Table Overflow
1. From Observation 1.3-A: At $1152 \times 648$ window, $W_{\text{inner}} = 884$ px.
2. In `QuotaDashboardPage.tsx`:
   - Replace `minWidth: '960px'` with `minWidth: '880px'` (or remove fixed `minWidth` and rely on column percentage/pixel constraints).
   - Adjust column widths so their sum is exactly $880$ px ($220 \times 4 \le 884$):
     - Account: $220$ px ($55 \times 4$)
     - Plan: $80$ px ($20 \times 4$)
     - 5-Hour Quota: $156$ px ($39 \times 4$)
     - Weekly Quota: $156$ px ($39 \times 4$)
     - AI Credits: $100$ px ($25 \times 4$)
     - Priority: $76$ px ($19 \times 4$)
     - Action: $92$ px ($23 \times 4$)
     - Sum: $220 + 80 + 156 + 156 + 100 + 76 + 92 = 880$ px.
3. Because $880\text{ px} \le 884\text{ px}$, the entire table fits inside the base viewport with zero horizontal scrollbar and zero clipping!

---

### Step 2.3: Snapping Interactive Gadgets to the 4-Pixel Grid
1. **Circular Gauges**:
   - `CircularGauge.tsx`: Set default `size = 128` ($32 \times 4$), `strokeWidth = 12` ($3 \times 4$).
   - Dual-mode in `QuotaDashboardPage.tsx`: Set `size={72}` ($18 \times 4$) or `size={76}` ($19 \times 4$), `strokeWidth={8}` ($2 \times 4$).
   - `CustomModelsPage.tsx`: Set `size={88}` ($22 \times 4$), `strokeWidth={8}` ($2 \times 4$).
   - `MfaVaultPage.tsx`: Set `strokeWidth="4"` or `"8"`.
2. **Horizontal Quota Bar**:
   - `HorizontalQuotaBar.tsx`: Set default `height = 8` ($2 \times 4$), `gap = 8` ($2 \times 4$).
3. **Toggle Switches**:
   - `ToggleSwitch.tsx`:
     - Small: `width = 32` ($8 \times 4$), `height = 16` ($4 \times 4$), `knobSize = 12` ($3 \times 4$), `translate = 16` ($4 \times 4$).
     - Medium: `width = 40` ($10 \times 4$), `height = 20` ($5 \times 4$), `knobSize = 16` ($4 \times 4$), `translate = 20` ($5 \times 4$).
4. **Spacing & Padding Snapping**:
   - Snap all `14px` gaps and margins (`TokenMonitorPage.tsx`, `AccountDetailModal.tsx`, `ScheduledTemplatesPage.tsx`) to `16px` ($4 \times 4$) or `12px` ($3 \times 4$).
   - Snap all `18px` paddings to `16px` or `20px`.
   - Snap all `10px` gaps to `8px` or `12px`.

---

### Step 2.4: Layout Verification Test Suite Architecture
1. **Module Location**:
   - Implementation: `frontend/src/utils/layoutTokens.ts`
   - Test Suite: `frontend/src/utils/layoutTokens.test.ts`
2. **Integration with Existing Test Infrastructure**:
   - `frontend/package.json` specifies `"test": "node --test src/**/*.test.ts"`.
   - Creating `src/utils/layoutTokens.test.ts` requires zero config changes and runs automatically on `npm test`.
3. **Test Suite Scope**:
   The test suite must comprise 6 distinct test suites with exhaustive assertions:
   - **Suite 1: Window Geometry & 16:9 Aspect Ratio**:
     - `MIN_WINDOW_WIDTH === 1152`, `MIN_WINDOW_HEIGHT === 648`.
     - `1152 % 4 === 0`, `648 % 4 === 0`.
     - `1152 / 648 === 16 / 9` with zero remainder.
   - **Suite 2: Top-Level Zone Alignment & Golden Ratio Proportions**:
     - `NAV_RAIL_WIDTH === 220` (`220 % 4 === 0`).
     - `HEADER_HEIGHT === 72` (`72 % 4 === 0`).
     - `WORKSPACE_WIDTH === 932` (`932 % 4 === 0`).
     - `WORKSPACE_HEIGHT === 576` (`576 % 4 === 0`).
     - Aspect ratio $|(932 / 576) - \phi| < 0.00003$ ($0.00002157 < 0.01$).
     - Inner workspace with 24px padding: $884 \times 528$ px, both divisible by 4.
   - **Suite 3: 4-Pixel Grid Snap & Rounding Helpers**:
     - `snapToGrid(n, 4)` returns nearest multiple of 4.
     - `ceilToGrid(n, 4)` returns smallest multiple of 4 $\ge n$.
     - `floorToGrid(n, 4)` returns largest multiple of 4 $\le n$.
     - Boundary checks across integers, decimals, and negative numbers.
   - **Suite 4: Ceiling vs Floor Rounding Golden Ratio Verification**:
     - Mathematical property test verifying that for any partitioned width $W$ ($884$, $868$, $932$, $680$, $580$):
       $|R_{\text{ceil}} - 16/9| < |R_{\text{floor}} - 16/9|$
     - Confirms ceiling rounding pushes quantized aspect ratio strictly closer to 16:9.
   - **Suite 5: Component Token & Modal Dimensions 4px Divisibility**:
     - Modal widths: 580, 680, 460, 540, 600, 420, 440 are all multiples of 4.
     - Circular gauge sizes (128, 72, 88, 56) and stroke widths (4, 8, 12) are all multiples of 4.
     - Toggle switch dimensions (32×16, 40×20) are all multiples of 4.
     - Padding tokens (8, 12, 16, 20, 24, 28, 32) are all multiples of 4.
   - **Suite 6: Viewport Overflow Budget Verification**:
     - Table column sum ($880$ px) $\le$ inner workspace ($884$ px), preventing viewport overflow.

---

## 3. Multi-Explorer Synthesis

### Consensus
1. **Window Target**: All three surveys agree on minimal window $1152 \times 648$ px ($16:9$), $1152 = 288 \times 4$, $648 = 162 \times 4$.
2. **Top-Level Layout**: All agree on NavRail $220$ px ($55 \times 4$), Header $72$ px ($18 \times 4$), workspace $932 \times 576$ px (aspect ratio $1.61806 \approx \phi$).
3. **Electron Runtime Changes**: All agree `electron/main.js` requires `minWidth: 1152, minHeight: 648`, `setAspectRatio(16 / 9)`, and maximize/fullscreen unconstraining (`setAspectRatio(0)`).
4. **Header Height Deficit**: All confirm current Header is `64px` in `TopRibbon.tsx`, `NavRail.tsx`, and `App.tsx`, and must be updated to `72px`.

### Resolved Conflicts
1. **Test Runner Choice**:
   - Initial consideration: Whether Vitest or Jest needed to be installed.
   - Resolution: `frontend/package.json` already has a working Node native runner (`"test": "node --test src/**/*.test.ts"`), which runs in ~80ms on Node 24 with native TypeScript support. No new dependencies or test frameworks are needed.
2. **Quota Dashboard Table Layout**:
   - Issue: Table causes horizontal scrollbars at 1152px window width due to `minWidth: '960px'`.
   - Resolution: Update table `minWidth` to `880px` and adjust column minimums to sum to `880px`, guaranteeing zero horizontal overflow.

### Dissenting Views
None. All geometry, layout tokens, and component dimensions are mathematically harmonious and unanimous across surveys.

### Gaps
1. **E2E Desktop Test**: `scripts/verify-desktop-e2e.js` currently checks title and daemon API, but does not yet assert window dimensions or aspect ratio. A phase should be added to assert `mainWindow.getMinimumSize()` returns `[1152, 648]`.

---

## 4. Caveats
1. **Dual Header Rendering in `App.tsx`**: When `currentTool === 0`, `<TopRibbon>` renders the header; for other tools, an inline `<header>` in `App.tsx` renders it. Both must be updated from `64px` to `72px`.
2. **Zoom / DPI Scaling**: At high DPI scaling (e.g. 150%, 125%), Chromium scales CSS pixels proportionately. Because all dimensions are multiples of 4, they remain integer pixel boundaries at 125% ($4 \times 1.25 = 5$ physical px) and 150% ($4 \times 1.5 = 6$ physical px).
3. **Electron setAspectRatio on Linux**: As identified by `explorer_geom_survey_1`, `setAspectRatio(0)` on maximize/fullscreen prevents compositor stuttering on X11 and Wayland.

---

## 5. Conclusion
1. **Layout Tokens**: Create `frontend/src/utils/layoutTokens.ts` exporting grid constants, golden ratio math helpers, and zone dimensions.
2. **Top-Level Layout**:
   - Update `NavRail.tsx` brand header height from `64px` to `72px`.
   - Update `TopRibbon.tsx` header height from `64px` to `72px`.
   - Update `App.tsx` inline header height from `64px` to `72px`.
3. **Component Refinements**:
   - Fix `QuotaDashboardPage.tsx` table from `minWidth: '960px'` to `880px` with 4px aligned column widths.
   - Snap `CircularGauge.tsx` default size to `128px`, stroke to `12px`, and dual gauge sizes to `72px`/`76px` with stroke `8px`.
   - Snap `HorizontalQuotaBar.tsx` height to `8px`, gap to `8px`.
   - Snap `ToggleSwitch.tsx` height to `16px`/`20px`.
   - Snap gaps and paddings in `TokenMonitorPage.tsx`, `AccountDetailModal.tsx`, `ScheduledTemplatesPage.tsx` from `14px`/`18px` to `16px`/`20px`.
4. **Automated Verification Suite**: Create `frontend/src/utils/layoutTokens.test.ts` testing window geometry, top-level zone ratios, grid quantisation, ceiling vs floor 16:9 bias, component token divisibility, and table viewport budget.

---

## 6. Verification Method

### 6.1 Programmatic Unit Tests
Run the frontend automated test suite:
```bash
npm test --prefix frontend
```
**Expected Result**:
- Discovers and runs `src/utils/layoutTokens.test.ts` alongside existing tests.
- 100% passing tests (expected ~45+ tests), 0 failures.
- Duration < 150ms.

### 6.2 TypeScript Compilation & Vite Build
Run the frontend production build:
```bash
npm run build --prefix frontend
```
**Expected Result**:
- `tsc -b` completes with 0 errors.
- `vite build` succeeds cleanly and emits bundle to `../pkg/webgui/dist`.

### 6.3 Desktop E2E Verification
Run the desktop verification script:
```bash
npm run test:desktop
```
**Expected Result**:
- Electron launches headlessly or under xvfb.
- Window minimum size is asserted at `[1152, 648]`.
- All phases pass green.
