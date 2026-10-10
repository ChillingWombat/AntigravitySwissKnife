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
