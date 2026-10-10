# Extensions

[Features](Features.md) / Extensions

The Extensions subsystem manages modular auxiliary tools embedded into Antigravity's interface. These tools provide in-context previewing, file tree exploration, developer scratchpads, and mobile layout simulation without leaving the pair-programming session.

---

## 1. Extension Gadget Directory

Antigravity Swiss Knife includes five integrated extension gadgets:

### Preview Browser
An embedded Chromium web view that renders local development servers in real time alongside active chat conversations:
- **Default Port**: Targets `http://localhost:8765` or configurable local dev server URLs (Vite, Next.js, Django, Flask).
- **Console & Network Sync**: Bridges frontend errors back to the agent session for automated debugging.
- **Auto-Reload**: Triggers frame refreshes when files in the workspace change.

### File Explorer
A lightweight filesystem browser integrated into the auxiliary inspection rail:
- **Project Scope**: Confines file trees to the active repository root.
- **External IDE Launcher**: One-click opening of files in the developer's preferred editor (presets for VS Code, Cursor, Windsurf, VSCodium, and Zed, plus custom binary command line overrides).

### Quick Memos
A persistent scratchpad for storing notes, operational commands, and context snippets:
- **Markdown Support**: Syntax highlighting for code blocks and task lists.
- **Storage Scope**: Toggleable between global scope (shared across all projects) and project-specific scope.
- **Search Engine**: Full-text keyword search indexing all stored memos.

### Mobile Simulator
A responsive device frame for testing web applications across standardized mobile viewports:
- **Supported Profiles**: iPhone 16 (393x852 px), Google Pixel 9 (412x924 px), and iPad Air (820x1180 px).
- **Device Chrome & Bezels**: Optional toggle to display hardware bezel styling or clean viewport boundaries.
- **Orientation Control**: One-click rotation between portrait and landscape modes.

### GitHub Workspace Gadget
Compact auxiliary rail presentation of the full-screen [GitHub Workspace](GitHub-Workspace.md), allowing developers to inspect pull requests and subagent assignments directly beside chat messages.

---

## 2. Visibility Matrix & Surface Placement

Developers can customize where each extension appears via the Extension Visibility Switches:
- **Auxiliary Panel (`aux_panel`)**: Enables the gadget in the collapsible right-hand rail of Antigravity's window.
- **Main Stage (`main_page`)**: Renders the tool as an expansive primary workspace tab on the central stage.

---

## 3. Persistence & Configuration Architecture

Extension states, default URLs, preferred editor binaries, and scratchpad contents persist through `pkg/enhancements` in the daemon's local SQLite store. All changes apply immediately without requiring full IDE restarts.
