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
- **Turn Jump Navigator**: Floating anchor buttons along the chat scroll track that let developers jump directly between user prompts and agent output blocks in long multi-turn sessions.
- **Live TPS Streaming Indicator**: Displays real-time generation speed (tokens per second) in the message header while the model streams output.
- **Thought Block Auto-Collapse**: Automatically folds lengthy internal reasoning traces into compact headers, keeping the active chat clean while preserving one-click expansion.
- **Enhanced Code Copy & Diff Highlights**: One-click copy buttons for code blocks with language indicators, plus inline side-by-side or unified diff rendering for suggested file changes.

---

## 3. Project Panel Enhancements

The Project Panel category customizes project organization and workspace categorization:
- **10x10 Project Color Palette**: A full 100-color matrix (10 greyscale steps plus 9 chromatic hues across 10 lightness levels) allowing developers to assign custom color tags and badges to individual workspaces.
- **Drag-and-Drop Project Reordering**: Reorganizes workspace tiles to reflect active project priorities.
- **Workspace Archival Management**: Moves dormant or completed workspaces to an archived storage tier, decluttering the active project list without deleting underlying files.

---

## 4. Auxiliary Panel Enhancements

The Auxiliary Panel category configures the right-hand inspection dock in Antigravity:
- **Modular Docking**: Controls which extension gadgets (GitHub Workspace, Quick Memos, Preview Browser, File Explorer) appear in the auxiliary dock.
- **Dock Pinning & Auto-Collapse**: Allows developers to keep auxiliary tools permanently visible side-by-side with chat or collapse them into a slim 48px navigation strip.
- **Responsive Width Splitting**: Configures default docking widths, allowing wide Kanban boards or preview viewports to expand without clipping conversation text.
