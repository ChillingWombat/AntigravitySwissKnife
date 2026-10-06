# Dispatch: Worker M1 (Electron Window Geometry & 16:9 Aspect Ratio Locking)

## Objective
Implement minimal window geometry (1152×648 px), strict 16:9 aspect ratio locking via `mainWindow.setAspectRatio(16 / 9)`, window state event handlers, and desktop E2E programmatic verification assertions.

## Working Directory
`/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/worker_geom_m1_1`

## Mandatory Reference
- `/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/ORIGINAL_REQUEST.md` (Must read first, timestamp 2026-10-06T03:39:21Z)
- `/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_geom_survey_1/handoff.md` (Detailed blueprint and line numbers)

## Exclusively Owned Files
- `electron/main.js`
- `scripts/verify-desktop-e2e.js`

## Detailed Tasks
1. Read `ORIGINAL_REQUEST.md` and `explorer_geom_survey_1/handoff.md`.
2. In `electron/main.js`:
   - Set `BrowserWindow` creation options:
     - `width: 1152`
     - `height: 648`
     - `minWidth: 1152`
     - `minHeight: 648`
   - Immediately after window creation, call `mainWindow.setAspectRatio(16 / 9)`.
   - Register window state event listeners:
     - `mainWindow.on('maximize', () => mainWindow.setAspectRatio(0));`
     - `mainWindow.on('unmaximize', () => mainWindow.setAspectRatio(16 / 9));`
     - `mainWindow.on('enter-full-screen', () => mainWindow.setAspectRatio(0));`
     - `mainWindow.on('leave-full-screen', () => mainWindow.setAspectRatio(16 / 9));`
   - In `runE2eVerification()` in `electron/main.js`, add programmatic assertions:
     - Verify `mainWindow.getSize()` matches `[1152, 648]` (or valid 16:9 multiple).
     - Verify `mainWindow.getMinimumSize()` matches `[1152, 648]`.
     - Verify `minWidth % 4 === 0` and `minHeight % 4 === 0`.
     - Verify `Math.abs((minWidth / minHeight) - (16 / 9)) < 0.0001`.
     - Output `[E2E-TEST] Window geometry verified: 1152x648 (16:9 aspect ratio, 4px aligned)`.
3. In `scripts/verify-desktop-e2e.js`:
   - Ensure the verification runner asserts that the geometry verification was successful and logged by the Electron main process.
4. Run verification:
   - Run desktop test or syntax check to ensure zero regressions.
5. Mandatory Integrity Rule:
   DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A teamwork_preview_auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.
6. Write your completion report to `/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/worker_geom_m1_1/handoff.md` and notify parent via `send_message`.

## 2026-10-06T03:54:52Z
[Message] timestamp=2026-10-06T03:54:52Z sender=22e8a004-e0c2-41d4-92e0-45bd204fac17 priority=MESSAGE_PRIORITY_HIGH
You are worker_geom_m1_1 for Milestone 1 (M6: Minimal Window Geometry & 16:9 Aspect Ratio Locking) of Antigravity Swiss Knife.
Your working directory is:
/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/worker_geom_m1_1

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A teamwork_preview_auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

Mandatory Instructions:
1. Read /mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/ORIGINAL_REQUEST.md (specifically timestamp 2026-10-06T03:39:21Z).
2. Read your dispatch file: /mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/worker_geom_m1_1/DISPATCH.md.
3. Read the Explorer handoff: /mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_geom_survey_1/handoff.md.
4. Exclusively owned files:
   - electron/main.js
   - scripts/verify-desktop-e2e.js
5. Implement the following:
   a. In electron/main.js:
      - Set BrowserWindow width: 1152, height: 648, minWidth: 1152, minHeight: 648.
      - Call mainWindow.setAspectRatio(16 / 9) immediately after window creation.
      - Add event listeners:
        mainWindow.on('maximize', () => mainWindow.setAspectRatio(0));
        mainWindow.on('unmaximize', () => mainWindow.setAspectRatio(16 / 9));
        mainWindow.on('enter-full-screen', () => mainWindow.setAspectRatio(0));
        mainWindow.on('leave-full-screen', () => mainWindow.setAspectRatio(16 / 9));
      - In runE2eVerification(), verify mainWindow dimensions, minWidth/minHeight (1152x648), 4px divisibility, and 16:9 aspect ratio.
   b. In scripts/verify-desktop-e2e.js:
      - Verify that the desktop E2E test asserts geometry checks from electron output.
6. Run verification commands (e.g. node syntax check or node scripts/verify-desktop-e2e.js if applicable) and document commands and output.
7. Write your detailed handoff report to /mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/worker_geom_m1_1/handoff.md.
8. Send a message to parent when complete.

## 2026-10-06T03:59:56Z
[Message] timestamp=2026-10-06T03:59:56Z sender=22e8a004-e0c2-41d4-92e0-45bd204fac17 priority=MESSAGE_PRIORITY_HIGH
**Context**: Server restart recovery for Milestone 1 (M6: Window Geometry & 16:9 Aspect Ratio Locking)
**Content**: The server has restarted and your process was paused. Please resume immediately.
Recall your tasks:
1. In electron/main.js:
   - Set BrowserWindow options: width: 1152, height: 648, minWidth: 1152, minHeight: 648.
   - Immediately call mainWindow.setAspectRatio(16 / 9).
   - Add state listeners:
     mainWindow.on('maximize', () => mainWindow.setAspectRatio(0));
     mainWindow.on('unmaximize', () => mainWindow.setAspectRatio(16 / 9));
     mainWindow.on('enter-full-screen', () => mainWindow.setAspectRatio(0));
     mainWindow.on('leave-full-screen', () => mainWindow.setAspectRatio(16 / 9));
   - In runE2eVerification(), add window geometry and aspect ratio assertions.
2. In scripts/verify-desktop-e2e.js:
   - Assert window geometry log output in test execution.
3. Run verification (e.g. node scripts/verify-desktop-e2e.js or syntax check).
4. Write handoff.md to /mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/worker_geom_m1_1/handoff.md.
5. Send completion message back to parent.
**Action**: Resume execution now and report back upon completion.
