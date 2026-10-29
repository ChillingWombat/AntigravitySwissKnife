# Extensions

[Features](Features.md) / Extensions

The Extensions subsystem manages modular auxiliary tools embedded into Antigravity's interface. These tools provide in-context previewing, file tree exploration, developer scratchpads, and mobile layout simulation without leaving the pair-programming session.

---

## Navigation & Header Tabs

The Extensions page provides a top header segmented switcher with two primary views:
- **Built-in**: Core native tools integrated directly into Antigravity's interface (Left Panel Access Mode, GitHub Workspace, Preview Browser, File Explorer, Quick Memos, Mobile Simulator). These tools execute within the IDE window or communicate directly with the local companion daemon.
- **Community**: Custom web and sandboxed frontend extensions. Supports external web services embedded via iframes and inline HTML/CSS/JavaScript components imported through JSON manifests.

---

## 1. Built-in Extension Gadgets

Antigravity Swiss Knife includes five integrated extension gadgets:

### Built-in Browser Enhancements
Enhances Antigravity's native built-in browser with developer tools and visual annotation capabilities:
- **Quick Dev Ports**: One-click navigation to local development servers (`:8765`, `:5173`, `:3000`, `:8080`, `:4173`) with custom port presets.
- **Responsive Device Emulation**: Test pages across mobile profiles (iPhone 16 Pro, Google Pixel 9, iPad) with orientation toggling, touch emulation, and viewport scaling.
- **Visual Annotations & Chat Handoff**: Freehand red pen, box highlights, interactive DOM element inspection, and one-click capture to Antigravity's active chat prompt with screenshot attachments.

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

## 3. Community Extensions

The Community tab allows developers to register, import, and configure custom extensions for Antigravity's auxiliary panel and main stage.

### Extension Types
- **Web URL (`url`)**: Embeds a running web service or local dev server (for example, `http://localhost:3000`) inside Antigravity via an iframe view.
- **Custom HTML / JS (`html`)**: Sandboxed inline frontend snippet with custom HTML markup, CSS styling, and JavaScript logic.

### JSON Import & Export
Extensions can be exported to standalone JSON files for backup or distribution, and imported in bulk or as individual objects via the Import JSON dialog.

### Community Extension Schema

Community extensions are defined by the following TypeScript schema:

```ts
export type CommunityExtensionIcon =
  | 'blocks'
  | 'globe'
  | 'code'
  | 'terminal'
  | 'database'
  | 'bot'
  | 'wrench'
  | 'book-open'
  | 'layers'
  | 'cpu'

export interface CommunityExtension {
  id: string                      // Unique kebab-case identifier
  name: string                    // Display title
  description?: string            // Explainer copy
  author?: string                 // Author name (defaults to 'Community')
  version?: string                // Semver string (defaults to '1.0.0')
  icon?: CommunityExtensionIcon   // Lucide icon identifier
  source_type: 'url' | 'html'     // Embedded URL or sandboxed HTML/JS
  url?: string                    // Target URL when source_type is 'url'
  html?: string                   // HTML markup when source_type is 'html'
  css?: string                    // CSS styles when source_type is 'html'
  js?: string                     // JavaScript code when source_type is 'html'
  enabled: boolean                // Whether extension is active
  aux_panel: boolean              // Available in right auxiliary rail
  main_page: boolean              // Available on main stage tab
  created_at?: string             // Creation timestamp (ISO string)
  updated_at?: string             // Last update timestamp (ISO string)
}
```

---

## 4. Persistence & Configuration Architecture

Extension states, default URLs, preferred editor binaries, and community extension definitions persist through `pkg/enhancements` in the daemon's local SQLite store. All changes apply immediately without requiring full IDE restarts.

---

## 5. Main Stage Overview Panel Auto-Hide

When the Swiss Knife extension stage (`#swiss-main-stage` or `#swiss-main-stage-container`) is opened on the central workspace, Antigravity's floating right-hand Overview and auxiliary inspection panel (`Overview`, `Review`, `Files`, `Terminal`, `Plugins`) can overlap primary stage controls, including the `Board` switcher and header actions.

The injected enhancement runtime automatically suppresses this panel while the main stage is active:

### Automatic Suppression
- The runtime monitors main stage activation (`isMainStageActive`).
- When the container adopts `display: flex`, the runtime scans the DOM for the Overview auxiliary container identified by its tab cluster (`Overview`, `Review`, `Files`, `Terminal`, `Plugins`).
- Matching containers receive the attribute marker:
  ```html
  data-swiss-hidden-by-main-stage="true"
  ```
- Injected runtime CSS enforces complete hiding:
  ```css
  [data-swiss-hidden-by-main-stage="true"] {
    display: none !important;
  }
  ```

### Viewport Restoration
- When leaving the extension stage to return to conversation or project views, the runtime queries all elements marked with `data-swiss-hidden-by-main-stage="true"`.
- The marker attribute is removed, and prior display styles (`__swissPrevDisplay`) are restored immediately, returning the floating Overview panel to view without reloading the workbench.
