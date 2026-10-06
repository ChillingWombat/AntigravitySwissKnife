# Window Geometry & Aspect Ratio Survey Report (`handoff.md`)

**Agent**: `explorer_geom_survey_1` (Window Geometry Explorer)  
**Target Project**: Antigravity Swiss Knife  
**Timestamp**: 2026-10-06T03:50:00Z  
**Directory**: `/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_geom_survey_1`

---

## 1. Observation

### 1.1 Current Electron Window Configuration in `electron/main.js`
In `/mnt/Data/Projects/Antigravity Swiss Knife/electron/main.js`, lines 199–218, the main window is instantiated as follows:

```javascript
199:   mainWindow = new BrowserWindow({
200:     width: 1280,
201:     height: 800,
202:     minWidth: 960,
203:     minHeight: 640,
204:     title: 'Antigravity Swiss Knife',
205:     icon: iconPath,
206:     show: !startMinimized,
207:     autoHideMenuBar: true,
208:     backgroundColor: '#131314', // Google Gemini dark surface token
209:     webPreferences: {
210:       preload: path.join(__dirname, 'preload.js'),
211:       nodeIntegration: false,
212:       contextIsolation: true,
213:       sandbox: false,
214:     },
215:   });
216: 
217:   mainWindow.setMenu(null);
```

- **Default Dimensions**: `width: 1280`, `height: 800`.
  - Ratio: $1280 / 800 = 1.6 = 16:10$ (violates strict 16:9 requirement).
- **Minimum Dimensions**: `minWidth: 960`, `minHeight: 640`.
  - Ratio: $960 / 640 = 1.5 = 3:2$ (violates strict 16:9 requirement).
- **Aspect Ratio Locking**:
  - `grep_search` across the entire codebase for `setAspectRatio` returned 0 matches (`No results found`).
  - There is currently **zero enforcement** of aspect ratio in `electron/main.js`.
- **Resize and Maximization Events**:
  - In `electron/main.js`, lines 220–231 handle the `close` event to minimize/hide to the system tray (`event.preventDefault(); mainWindow.hide();`).
  - There are currently **no event listeners** registered for `resize`, `will-resize`, `maximize`, `unmaximize`, `enter-full-screen`, or `leave-full-screen`.

### 1.2 Desktop Scripts & Packaging in `package.json`
In `/mnt/Data/Projects/Antigravity Swiss Knife/package.json`:
- `"main": "electron/main.js"` (line 5)
- `"desktop": "electron ."` (line 10)
- `"test:desktop": "node scripts/verify-desktop-e2e.js"` (line 13)
- `"electron": "^35.0.0"` in `devDependencies` (line 74). Electron 35 provides native cross-platform support for `BrowserWindow.setAspectRatio(aspectRatio[, extraSize])` and resetting via `setAspectRatio(0)`.

### 1.3 Desktop E2E Verification Harness
In `/mnt/Data/Projects/Antigravity Swiss Knife/scripts/verify-desktop-e2e.js` and `electron/main.js` (lines 319–357):
- `runE2eVerification()` in `electron/main.js`:
  ```javascript
  324: const title = mainWindow ? mainWindow.getTitle() : '';
  325: console.log('[E2E-TEST] Window title:', title);
  ```
  Currently checks only the window title (`Antigravity Swiss Knife`), daemon API status (`http://127.0.0.1:8765/api/status`), and `getLoginItemSettings` IPC.
- It does **not** assert `mainWindow.getSize()`, `mainWindow.getMinimumSize()`, 4-pixel divisibility, or aspect ratio.

### 1.4 Frontend Top-Level Layout Dimensions in `frontend/src`
Direct inspection of `frontend/src/App.tsx`, `components/NavRail.tsx`, and `components/TopRibbon.tsx`:
- `frontend/src/components/NavRail.tsx` line 33: `width: '220px'` ($220 = 55 \times 4$).
- `frontend/src/components/NavRail.tsx` line 46: `height: '64px'`.
- `frontend/src/components/TopRibbon.tsx` line 26: `height: '64px'`.
- `frontend/src/App.tsx` line 115: `height: '64px'`.
- Notice: The header height is currently `64px`, whereas requirement R2 specifies `72px` ($18 \times 4$) at base window size.

---

## 2. Logic Chain

### Step 2.1: Mathematical Validation of Target Geometry (1152×648 px)
1. **Aspect Ratio Verification**:
   $$\frac{1152}{648} = \frac{128 \times 9}{72 \times 9} = \frac{16}{9} = 1.7777777777777777...$$
   $1152 \times 9 = 10368$ and $648 \times 16 = 10368$. The ratio is mathematically identical to 16:9 with zero error.
2. **4-Pixel Grid Alignment Verification**:
   - $1152 \div 4 = 288$ ($1152 \pmod 4 = 0$).
   - $648 \div 4 = 162$ ($648 \pmod 4 = 0$).
   Both width and height are strictly 4-pixel aligned integer multiples.

### Step 2.2: Golden Ratio Proportions for Top-Level Zones (R2 Alignment)
1. When window dimensions are at the minimum of $1152 \times 648$ px:
   - Navigation rail width: $W_{\text{rail}} = 220$ px ($220 \div 4 = 55$).
   - Header ribbon height: $H_{\text{header}} = 72$ px ($72 \div 4 = 18$).
2. The remaining main content workspace area dimensions are:
   - $W_{\text{content}} = 1152 - 220 = 932$ px ($932 \div 4 = 233$).
   - $H_{\text{content}} = 648 - 72 = 576$ px ($576 \div 4 = 144$).
3. Workspace aspect ratio:
   $$\frac{W_{\text{content}}}{H_{\text{content}}} = \frac{932}{576} \approx 1.6180555555555556$$
4. Golden ratio comparison:
   $$\phi = \frac{1 + \sqrt{5}}{2} \approx 1.618033988749895$$
   $$|1.61805556 - 1.61803399| = 0.00002157 < 0.00003$$
   The resulting content workspace matches the golden ratio $\phi$ to within $0.00003$, satisfying R2 with remarkable precision.

### Step 2.3: Window Lifecycle & Window Manager Resizing Behavior
1. **Interactive Manual Resizing**:
   Calling `mainWindow.setAspectRatio(16 / 9)` instructs the operating system window manager (X11 WM hints, Wayland compositor, Windows DWM, macOS NSWindow) to restrict user drag-resizing to the 16:9 ratio.
2. **Maximization and Fullscreen Handling**:
   - When a user maximizes the window or activates fullscreen, the OS forces the window to occupy the monitor's display dimensions or workspace area (excluding taskbars/panels).
   - Display aspect ratios vary widely (e.g., 16:10, 21:9, 4:3) and workspace areas rarely equal 16:9 due to OS taskbars/docks.
   - If `setAspectRatio(16 / 9)` remains active during maximize or fullscreen, window managers on Linux (GNOME/Mutter, KDE/KWin) and macOS can jitter, produce visual artifacts, or fail to maximize smoothly.
   - Calling `mainWindow.setAspectRatio(0)` resets/unconstrains the ratio during maximize and fullscreen.
   - Upon `unmaximize` or `leave-full-screen`, re-calling `mainWindow.setAspectRatio(16 / 9)` cleanly restores 16:9 aspect ratio locking for interactive windowed resizing.
3. **Minimize / Restore**:
   - Minimizing to tray hides the window. Restoring via tray click calls `mainWindow.show()` and `mainWindow.focus()`, preserving the existing 16:9 bounds and constraints.

---

## 3. Caveats

1. **OS-Level Window Snapping**: On Windows (Aero Snap) and certain Linux tiling window managers, dragging a window to screen edges forces it into a half-screen or quadrant tile. OS snapping takes precedence over `setAspectRatio`. When unsnapped or resized manually, 16:9 locking resumes.
2. **Wayland vs XWayland Compositors**: In Linux Wayland sessions without XWayland, compositor-specific implementations of aspect ratio hints can vary slightly between GNOME Mutter and KDE KWin. However, Electron 35's Chromium base handles size hints predictably.
3. **Pre-existing Daemon in E2E Test Phase 4**: In `scripts/verify-desktop-e2e.js`, Phase 4 performs `pgrep -a swiss || true` to check for orphaned daemons. If a developer or external daemon was already running before the test, this check flags it. The test harness should ensure it isolates the test-spawned child PID rather than false-flagging pre-existing external daemons.

---

## 4. Conclusion

1. **Current Code State**:
   - `electron/main.js` currently specifies `width: 1280`, `height: 800`, `minWidth: 960`, `minHeight: 640`.
   - `mainWindow.setAspectRatio` is completely missing.
   - Window event lifecycle handlers for maximize/unmaximize and fullscreen are missing.
2. **Target Modifications Required in `electron/main.js`**:
   - Update `createWindow()` constructor options:
     ```javascript
     mainWindow = new BrowserWindow({
       width: 1152,
       height: 648,
       minWidth: 1152,
       minHeight: 648,
       ...
     });
     ```
   - Lock aspect ratio immediately after creation:
     ```javascript
     mainWindow.setAspectRatio(16 / 9);
     ```
   - Add state lifecycle handlers:
     ```javascript
     mainWindow.on('maximize', () => mainWindow.setAspectRatio(0));
     mainWindow.on('unmaximize', () => mainWindow.setAspectRatio(16 / 9));
     mainWindow.on('enter-full-screen', () => mainWindow.setAspectRatio(0));
     mainWindow.on('leave-full-screen', () => mainWindow.setAspectRatio(16 / 9));
     ```
3. **Required Enhancements in E2E Verification**:
   - In `electron/main.js:runE2eVerification()`, verify:
     - `mainWindow.getSize()` matches `[1152, 648]` (or 16:9 multiple).
     - `mainWindow.getMinimumSize()` matches `[1152, 648]`.
     - `minWidth % 4 === 0` and `minHeight % 4 === 0`.
     - `Math.abs((minWidth / minHeight) - (16 / 9)) < 0.0001`.
   - In `scripts/verify-desktop-e2e.js`, assert the geometry verification outputs in stdout.

---

## 5. Verification Method

### 5.1 Programmatic Math Verification
Run node one-liner to verify mathematical properties:
```bash
node -e "
const w = 1152, h = 648;
console.log('16:9 check:', (w / h) === (16 / 9));
console.log('4px divisible:', (w % 4 === 0) && (h % 4 === 0));
const navW = 220, headerH = 72;
const contentW = w - navW, contentH = h - headerH;
console.log('Content dimensions:', contentW, 'x', contentH);
console.log('Content 4px divisible:', (contentW % 4 === 0) && (contentH % 4 === 0));
const phi = (1 + Math.sqrt(5)) / 2;
console.log('Content aspect ratio:', contentW / contentH);
console.log('Phi error:', Math.abs((contentW / contentH) - phi));
"
```
**Expected Output**:
- `16:9 check: true`
- `4px divisible: true`
- `Content dimensions: 932 x 576`
- `Content 4px divisible: true`
- `Content aspect ratio: 1.6180555555555556`
- `Phi error: 0.000021566805660729798` ($< 0.00003$)

### 5.2 Desktop E2E Verification Command
Run the desktop verification suite:
```bash
npm run test:desktop
```
Confirm the Electron shell launches, verifies window title, dimensions, aspect ratio, API status, and terminates cleanly with exit code 0.

### 5.3 Frontend Build & Unit Test Verification
```bash
npm test --prefix frontend
npm run build --prefix frontend
```
Confirm all frontend tests pass 100% and Vite production bundle compiles cleanly.
