# Original User Request

## 2026-10-10T02:50:37Z

Refactor Swiss Knife across UI polish, Custom Models management, Token Monitor telemetry, and core backend quota probing: rename Auxiliary Panel, remove obsolete badges/icons, reorganize Custom Models modal with inline test connection and bottom-left deletion, support custom quota endpoints and multi-suffix probing (including opencode.ai), ensure default custom model status reflects accurately, sort Token Price table (Gemini Native first in selector order, then Custom models with defaults pinned), eliminate Claude Opus 4.6 false positives from transcripts, append chat annotations to end, and scale Antigravity launch logo button.

Working directory: /mnt/Data/Projects/Antigravity Swiss Knife
Integrity mode: development

## Requirements

### R1. UI Refinements & Sizing Adjustments
- Rename tab 'Auxiliary & Overview Panel' to 'Auxiliary Panel' in frontend/src/App.tsx.
- Remove badge tag Matches Native Tabs and division style badge (Divider Lines / Border Zones) in frontend/src/pages/AppEnhancementsPage.tsx.
- Remove {sortedAccounts.length} Accounts badge in frontend/src/pages/FingerprintsPage.tsx.
- Remove leading Lucide icons before headers in frontend/src/pages/SwitcherSettingsPage.tsx (Gemini Subagent Custom Models, Model Source Hierarchy & Priority Order, Multi-App Account Synchronization Mode).
- Make the Antigravity launch logo button in frontend/src/pages/QuotaDashboardPage.tsx a tiny bit larger (image size from 20px to 24px) while preserving visual alignment.
- Default Auto-Check & Upgrade (auto_upgrade) to false (OFF) in pkg/core/config.go and frontend/src/pages/SystemSettingsPage.tsx.
- Remove the 3 corner tags on the version gadgets in SystemSettingsPage.tsx.

### R2. Custom Models Page & Setup Modal Reorganization
- Remove the leading <Coins> icon before the Quota & Price header in CustomModelsPage.tsx.
- In the fee charging mode buttons, change label 'Untracked / N/A' to simply 'N/A'.
- When Manual Override is OFF, disable fee mode buttons and inputs (balance, quota, fraction) so they are not changeable.
- When fee mode is N/A, circular gauge ring must be completely grey and inner text must display N/A.
- If neither factory nor 3rd party info provides quota data, normalize quota_type to 'na' as fallback, and render the N/A button as actively selected.
- Add an optional custom quota endpoint URL field in the Quota & Price section so users can specify custom balance/quota endpoints.
- Backend: extend DetectAndFetchQuota in pkg/custommodels/quota.go with multiple common probing suffixes (/v1/usage, /usage, /api/user/usage, etc.) and add opencode.ai to the provider dictionary.
- Remove Project Mappings setting from the modal and card; ensure all enabled custom models are available to all projects (['*']).
- Relocate Test Connection button to the same row as Set as default custom model toggle at the right end.
- Relocate Delete Model button to the bottom-left corner of the modal.
- For models already selected as default custom model (via rules.default_custom_model or model.is_default), show Set as default custom model toggle as ON.
- Remove the explain text under Model Budget / Usage Cap.

### R3. Token Price Table Sorting & Edit Rate Modal Relocation
- In the Token Price table, sort models: Gemini Native models first (ordered as in Antigravity model selector), then Custom models (by ascending internal_id).
- In each category, rank the chosen default model higher.
- Relocate the Delete button from table row Action column into the bottom-left corner of the Edit Rate modal.
- Remove bottom subscript subtitle lines from all 4 metric cards on the Token Monitor page.
- Update the 3rd metric card to show Gemini Generation Speed displaying 76.2 TPS.

### R4. Model Detection Accuracy (Eliminate False Opus 4.6 Attribution)
- Fix model detection in pkg/webgui/token_monitor.go: extract actual model from transcript.jsonl step 0 (<USER_SETTINGS_CHANGE>) rather than byte-scanning code mentions in SQLite database files.
- Ensure conversations that only used Gemini are strictly attributed to Gemini.

### R5. Visual Annotation Injection to Chat End
- In pkg/plugins/auxiliary.go, append visual annotations to the end of the chat composer textarea rather than prepending.

## Acceptance Criteria

### UI & Modal Ergonomics
- [ ] Tab renders as "Auxiliary Panel".
- [ ] No "Matches Native Tabs", "Divider Lines", or "15 Accounts" badges appear.
- [ ] Switcher Settings headers have zero leading icons.
- [ ] Antigravity launch logo button image renders at 24px.
- [ ] Auto-upgrade switch defaults to OFF.
- [ ] Custom models modal has no Project Mappings field; Test Connection is inline on the toggle row; Delete Model is at bottom-left.
- [ ] Default custom model displays toggle as ON in its edit modal.
- [ ] Quota & Price header has no Coins icon, button displays "N/A", and gauge renders completely grey with "N/A" text when mode is N/A.
- [ ] Manual Override OFF disables fee mode options and value inputs.
- [ ] Custom quota endpoint input is available and queried by backend.

### Backend & Telemetry
- [ ] Backend probes common quota suffixes including /v1/usage and supports opencode.ai.
- [ ] Token Price table orders Gemini Native models first, then Custom models, default models on top.
- [ ] Edit Rate modal features Delete button at bottom-left; table Action column only has Edit Rate.
- [ ] Token Monitor displays 0 Claude Opus requests for Gemini conversations; 3rd card shows "Gemini Generation Speed 76.2 TPS".
- [ ] All Vitest tests and Go tests pass cleanly.


## 2026-10-10T03:34:44Z

Server has restarted. Please resume all tasks and active subagent milestones immediately:
1. Complete Milestone 2 (CustomModelsPage reorganization, fee mode N/A states, custom quota endpoint, inline test connection, bottom-left deletion, grey gauge rendering for N/A).
2. Continue with Milestone 3 (Token Price table sorting with Gemini Native first and defaults pinned, Edit Rate modal deletion button relocation, TPS card update, and subtitle removal).
3. Complete Milestone 4 (Model detection from transcript step 0 <USER_SETTINGS_CHANGE>, and visual annotation appending).
4. Run full verification tests.
Proceed at full speed.

## 2026-10-10T15:44:10Z

This is a single self-contained fix; keep it small and focused.

Resolve the Antigravity desktop IDE launch window focus failure and live modification injection issues in Antigravity Swiss Knife. Implement the fixes following David-Wish Track 1 protocol (Focused Patch / Bugfix), ensuring window focus over CDP when already running, correcting executable binary path resolution, restoring custom model injection, and rebuilding production assets.

Working directory: /mnt/Data/Projects/Antigravity Swiss Knife
Integrity mode: demo

## Requirements

### R1. Host IDE Launch & Live Window Focus
Update `pkg/process/shield.go:LaunchHostIDE()` so that when Antigravity is already running (PID detected), it actively resolves the live DevTools port from `DevToolsActivePort` (or port scan) and executes CDP window activation (`Page.bringToFront` and `window.focus()`) across active page targets instead of silently doing a no-op return. If Antigravity is not running, ensure it spawns the executable binary cleanly as a detached background process across desktop environments.

### R2. System Detector Executable Path Resolution
Update `pkg/system/detector.go` and `pkg/core/constants.go` so `desktop_app.path` resolves to the actual executable binary (`/opt/Antigravity/antigravity` or `/usr/bin/antigravity`) rather than the root directory (`/opt/Antigravity`). This ensures Path & Storage and process launch routines reference a runnable file.

### R3. Custom Models Inference Availability Contract
Align the custom model injection script (`pkg/custommodels/script.go`) and the daemon API (`/api/custom_models` in `pkg/webgui/server.go` / `pkg/custommodels/config.go`): ensure the payload includes `"inference_supported": true` (and `isCustomModelInferenceAvailable()` accepts it) so valid enabled models (such as DeepSeek V4.1 Flash) render properly in Antigravity's composer model selector rather than being purged from the DOM.

### R4. Build Synchronization & Runtime Reinstall
Rebuild the frontend bundle (`npm run build:frontend`) and Go companion binary (`npm run build:go` -> `bin/swiss`). Update the installed runtime binary at `/home/david/.local/share/antigravity-swiss-knife/app/resources/bin/swiss` and restart the daemon so the live service runs the updated code.

### R5. David-Wish Track 1 Protocol & Remote Synchronization
Execute under David-Wish Track 1:
- Create a dedicated branch `wip/launch-and-mod-fixes` off current `main`.
- Enforce the zero local-only drift contract: synchronize the branch and commits to `origin`.
- Maintain strict David-Design standards (Lucide icons only, no decorative emojis) and David-Humanizer concise technical voice.
- Eliminate useless defensive code, empty catch blocks, and paranoid null checks.

## Acceptance Criteria

### Launch & Window Focus
- [ ] Calling `/api/desktop/launch` when Antigravity is already running actively brings the Antigravity window to the front via CDP (`Page.bringToFront`).
- [ ] Calling `/api/desktop/launch` when Antigravity is stopped starts the process cleanly.
- [ ] `/api/system/installations` reports a valid executable path for `desktop_app.path` (pointing to the binary, not directory).

### Modification & Model Injection
- [ ] `GET /api/custom_models` includes `"inference_supported": true`.
- [ ] Custom models dropdown menu and custom model group render in Antigravity composer without being removed.
- [ ] Persistent stylesheet (`antigravity-swiss-styles`) and prompt jump bar (`#swiss-prompt-jump-bar`) remain active and applied.

### Verification & Test Suite
- [ ] All Go tests (`go test -count=1 ./...`) pass green.
- [ ] All frontend unit tests (`npm test --prefix frontend`) pass green.
- [ ] Branch pushed upstream with remote synchronization intact.


## 2026-10-13T16:07:36Z

Comprehensive UI polish, GCP model integration enhancements, remote tunnel and sidecar repair, token telemetry accuracy fix, customizable session resumption prompt, and conversation tab hover styling in Antigravity Swiss Knife.

Working directory: /mnt/Data/Projects/Antigravity Swiss Knife
Integrity mode: development

## Requirements

### R1. Modal Backdrop Click Dismissal Prevention
All popup modal dialogs (including Add Account, Edit Account, Add Custom Model, Add GCP Model, Security Report, Discovered Accounts, and Delete confirmations) must not exit when clicking outside the window on the backdrop. They must only close when the user clicks the explicit close ('X') or cancel button.

### R2. Official GCP Logo for 'Add GCP Models'
Replace the approximate vector icon with the official Google Cloud Platform logo from `assets/logo/GCP.png` for the 'Add GCP Models' button and associated GCP cards.

### R3. Real Logos for ACP Utility Section Agents
Replace synthetic icons in the ACP utility mesh section with the real logos provided in `assets/logo/`:
- `Antigravity_CLI.webp` for Antigravity CLI
- `claude.svg` for Claude Code
- `codex.jpeg` for OpenAI Codex
- `deepseek.png` for DeepSeek
- `devin.png` for Devin
- `opencode.png` for OpenCode
- `pi.png` for Inflection Pi
- `images.jpeg` (Cursor logo) for Cursor Editor Agent
- `antigravity_logo.png` for Antigravity 2.0

### R4. GCP Model Setup: Direct Model ID Entry & Model Garden Link
Remove preset model buttons from the Add GCP Model modal. The user enters the Model ID manually in an input field. Add an external link icon after the Model ID header linking to `https://console.cloud.google.com/agent-platform/model-garden`.

### R5. Updated GCP Instructions & Credit Balance Display
1. Update GCP setup instructions: replace the legacy Vertex AI User role with the **Agent Platform User** (`roles/aiplatform.user`) role, and instruct the user to ensure `iam.disableServiceAccountKeyCreation` is disabled/not enforced in organization/group policy.
2. Query and display the GCP credit balance / billing account status for GCP models via Google Cloud Billing API (`projects/{projectID}/billingInfo`).

### R6. GitHub Workspace Settings Cleanup
1. Remove the non-functional 'List View' default view setting from the GitHub Workspace section in ExtensionsPage.
2. Remove the 'Open Full Workspace' button and its full-screen in-app modal.

### R7. Default Off for Remote Control and Gemini Bridge
Configure the default state of the enable toggles for both Remote Control (`remote_control.json` / `RemoteControlConfig`) and Gemini Bridge (`bridge.json` / `Config`) to `false` (off).

### R8. Remote Intranet/Internet Tunnel Settings & Quick Tunnel Sidecar Fix
1. Fix the `startQuickTunnel` deadlock bug in `pkg/bridge/tunnel.go` where `cloudflared` URL scanner blocked on mutex locks, causing the 12-second timeout failure.
2. Provide dedicated remote tunnel settings (Quick Tunnel via cloudflared sidecar, Tailscale, Local Intranet loopback, and Custom URL) for both Gemini Bridge and Remote Control so users on remote networks can connect seamlessly.

### R9. Accurate Token Generation Speed (TPS) Calculation
Investigate and resolve token generation speed calculation issues in `token_monitor.go` where reported generation speeds erroneously exceed realistic limits (>700 TPS), ensuring duration and output token timings correctly measure streaming intervals.

### R10. 'Visible Conversation Tabs' Header
Add a dedicated header titled `Visible Conversation Tabs` above the fixed number vs. dynamic conversation tab hide controls.

### R11. Session Resumption Reliability & Customizable Resume Prompt Setting
1. Diagnose why the automatic session resume prompt failed to send on account switches and ensure reliable CDP/IPC delivery.
2. Add a new setting in the Settings tab of Account Switcher allowing the user to view and customize the resumption prompt sent to agents when resuming sessions.

### R12. Conversation Tab Hover Background Uniformity
Fix the recurring issue where the conversation tab row in the sidebar displays mismatched/split background colors while hovering (where the action button cluster and gradient mask display a different background color from the row tint). Ensure the entire hovered tab maintains a seamless, uniform background color matching `--row-bg`.

## Acceptance Criteria

### Modal & UI Interactions
- [ ] Clicking the backdrop of any popup modal does not dismiss it; only the close button or cancel button closes the window.
- [ ] 'Add GCP Models' displays the real GCP logo image from `assets/logo/GCP.png`.
- [ ] ACP Mesh cards render real product logos for Antigravity CLI, Claude, Codex, DeepSeek, Devin, OpenCode, Pi, and Cursor.
- [ ] Add GCP Model modal contains direct Model ID input with external link icon opening Google Cloud Model Garden.
- [ ] GCP setup instructions cite Agent Platform User role and group policy key creation requirements.
- [ ] GCP custom models show credit balance / billing status where available.
- [ ] List View setting and 'Open Full Workspace' button removed from GitHub workspace settings.
- [ ] Remote Control and Gemini Bridge enable buttons default to off.
- [ ] Header 'Visible Conversation Tabs' displayed above conversation tab hide controls.
- [ ] Settings tab includes customizable session resumption prompt input with persistence.
- [ ] Hovered conversation tabs in the sidebar display a uniform background color across the entire row, action bar, and fade gradient.

### Backend & Service Integrity
- [ ] `pkg/bridge/tunnel.go` quick tunnel starts `cloudflared` without mutex deadlock and extracts public URL cleanly.
- [ ] Dedicated remote tunnel options functional for Remote Control.
- [ ] Token monitor TPS calculation accurately bounds generation speed to realistic thresholds.
- [ ] `go test ./...` passes across modified packages.
- [ ] Frontend builds cleanly with zero TypeScript errors (`npm run build`).


## 2026-10-14T03:28:56Z

Additional issues reported by user and from forensic audit (conversation 3caf6b1a-4d05-4285-8c67-58b628bac5ec):
1. Restore horizontal divider lines between projects in sidebar for projects with show more / show less buttons.
2. Fix AGY agent tabs in GitHub Workspace: prevent "AGY Agents (780)" badge inflation, fix broken page rendering / empty horizontal striped lines when switched to Killed category, exclude killed subagents older than 3 days, and cap group cards at 20.
3. Fix session resume reliability: in `pkg/revival/cdp_trigger.go`, prevent `cleanupQueuedDuplicate()` from deleting the queued resume prompt when `isBusy()` is true. Add customizable resume prompt setting in Account Switcher Settings tab.
4. Fix Windows cross-compilation failure caused by direct `syscall.Flock` usage.
5. Deduplicate concurrent post-relaunch revival dispatches across `pkg/process/shield.go` and `pkg/daemon/daemon.go`.
