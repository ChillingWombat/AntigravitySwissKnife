# App Enhancements

[Features](Features.md) / App Enhancements

The App Enhancements subsystem injects UI controls, visual badges, and productivity hooks directly into Google Antigravity 2.0 using the Chrome DevTools Protocol (CDP).

---

## 1. Runtime Injection Mechanism

Antigravity executes within an Electron runtime. Swiss Knife injects two live assets into the renderer process:
- `persistent_script.js`: DOM observer attaching interactive widgets, keybindings, and telemetry meters.
- `persistent_styles.css`: CSS stylesheet overriding default typography, card paddings, and theme variables.

Injection occurs via the Chrome DevTools Protocol `Page.addScriptToEvaluateOnNewDocument` API upon application launch. If Antigravity restarts during an account switch, Swiss Knife re-injects the scripts automatically without requiring manual reloads.

---

## 2. Chat View Enhancements

The Chat View category refines the main conversation interface:
- **Chat Navigator & Prompt Jump Rail**:
  - **Split-Gap Hit Buffer & Rail Hover**: The prompt jump rail (`#swiss-prompt-jump-bar`) uses `gap: 0px` with `padding: 4px 6px` and `pointer-events: auto`, listening for `mouseenter` across the entire rail section between top and bottom turns. Individual prompt dashes (`.swiss-prompt-dash`) apply `padding: 3.5px 6px; box-sizing: content-box; background-clip: content-box;`, painting lines at 14px × 1.5px (or configured dimensions) while splitting the 7px inter-dash distance 50/50 with zero dead zones and a 6px horizontal buffer.
  - **Direct Overlay Positioning**: When open, the hover panel (`#swiss-prompt-navigator-panel`) positions directly over the rail lines (`let left = rect.left;`, clamped within viewport bounds) rather than floating off to the right.
  - **Underlying Rail Occlusion**: While the panel is displayed, the underlying rail lines are hidden via `data-swiss-panel-open="true"`, `opacity: 0`, `visibility: hidden`, and `pointer-events: none`. Closing the panel restores the rail lines to full opacity, visibility, and pointer responsiveness.
  - **Row Dash Expansion on Hover**: Hovering any row in the navigator panel triggers a smooth width expansion of its left dash indicator (`.swiss-navigator-row-dash`) from 14px to 22px (`Math.max(22, dashWidth + 8)`) with active color highlighting and a subtle row background (`rgba(148, 163, 184, 0.12)` in dark mode, `rgba(100, 116, 139, 0.08)` in light mode).
- **Live TPS Streaming Indicator**: Displays real-time generation speed (tokens per second) in the message header while the model streams output.
- **Thought Block Auto-Collapse**: Automatically folds lengthy internal reasoning traces into compact headers, keeping the active chat clean while preserving one-click expansion.
- **Enhanced Code Copy & Diff Highlights**: One-click copy buttons for code blocks with language indicators, plus inline side-by-side or unified diff rendering for suggested file changes.
- **Click-to-Compact Context Window Ring**:
  - **Context & Flag Override**: Enables Antigravity's native `/compact` slash command by setting `"enable-compact-slash-command": true` in `localStorage` flag overrides (`jetski.developer.customFlagOverrides`).
  - **Interactive Progress Ring**: The context window progress ring (`#swiss-context-window-ring` in the main composer and `.swiss-context-window-ring-sub` in subagent inputs) acts as an interactive button (`role="button"`, `cursor: pointer`, Enter/Space keyboard support) displaying tooltip usage with a `" — Click to compact"` prompt when idle.
  - **Direct `/compact` Invocation**: Traverses the React Fiber tree of `[data-testid="agent-input-box"]` (`S4b` / `AgentInputBox`) to locate `handleSubmit` and `lexicalRef`. It constructs a Lexical editor state with a native `/compact` `contextScopeItemMention` node and sends it immediately via `handleSubmit(editor, 1)` with `MessageDeliveryStrategy.NEXT_INVOCATION = 1` (bypassing the queued `WHEN_IDLE = 2` strategy). If React Fiber props are missing, it falls back to populating the composer's editable element (`[contenteditable="true"]` or `textarea`) with `/compact` and dispatching send.
  - **Continuous Rotation & Completion Polling**: Sets `data-swiss-compacting="true"`, updating the tooltip to `"Compacting context window..."` and animating the SVG ring with `@keyframes swiss-context-ring-spin` (0.8s linear cycle). The ring rotates for at least 1 full 360-degree cycle (minimum 850ms) and continues rotating until background compaction finishes (`[data-testid="agent-loading"]` with "Compacting" or active cancel/stop state clears). Once finished, `data-swiss-compacting` is removed, token metrics are force-refreshed, and the ring returns to idle context display.

---

## 3. Project Panel Enhancements

The Project Panel category customizes project organization and workspace categorization:
- **10x10 Project Color Palette**: A full 100-color matrix (10 greyscale steps plus 9 chromatic hues across 10 lightness levels) allowing developers to assign custom color tags and badges to individual workspaces.
- **Drag-and-Drop Project Reordering**: Reorganizes workspace tiles to reflect active project priorities.
- **Workspace Archival Management**: Moves dormant or completed workspaces to an archived storage tier, decluttering the active project list without deleting underlying files.

---

## 4. Auxiliary Panel Enhancements

The Auxiliary Panel category configures the right-hand inspection dock in Antigravity:
- **Modular Docking**: Controls which extension gadgets (GitHub Workspace, Quick Memos, File Explorer) appear in the auxiliary dock.
- **Dock Pinning & Auto-Collapse**: Allows developers to keep auxiliary tools permanently visible side-by-side with chat or collapse them into a slim 48px navigation strip.
- **Responsive Width Splitting**: Configures default docking widths, allowing wide Kanban boards or preview viewports to expand without clipping conversation text.

---

## 5. Automation & Sidecar Enhancements

The Automation & Sidecar category enhances Antigravity 2.0 task automation workflows:
- **Target Project Selector**:
  - **Context & Flag Override**: Antigravity's `jetski-sidecar-declared-permissions` feature flag normally suppresses the project selection dropdown in the New Automation modal (`tec`), forcing `ManageSidecar` creation calls (`action: 5`, `SIDECAR_ACTION_CREATE`) to pass an empty project ID and isolate automations into dedicated scratch workspaces. Swiss Knife overrides `jetski-sidecar-declared-permissions` to `false` in `localStorage` flag configuration (`jetski.developer.customFlagOverrides`) so the native selector condition evaluates to true.
  - **Live DOM Selector Injection**: When the New Automation modal opens without a native project selector, Swiss Knife injects `#swiss-automation-project-field` with a `<select id="swiss-automation-project-select">` right below the automation name field. Projects are extracted live by inspecting the `#root` React Fiber tree for `sidebarSectionsProvider` project sections, falling back to modal fiber props and left sidebar DOM rows. The first existing workspace is selected by default, alongside an explicit "Create New Dedicated Project" option.
  - **ConnectRPC `ManageSidecar` Interception**: Swiss Knife intercepts renderer requests to `/exa.language_server_pb.LanguageServerService/ManageSidecar`. On creation (`action: 5`), it binds `projectId` to the selected existing workspace before dispatch. When the server returns `sidecarId`, Swiss Knife automatically issues a follow-up `ManageSidecar` request (`action: 6`, `SIDECAR_ACTION_SET_PROJECT`) to guarantee project binding even if the initial creation ignored the project field.
