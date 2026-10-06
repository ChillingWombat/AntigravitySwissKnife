# Antigravity Swiss Knife

<div align="center">
  <img src="assets/logo.png" alt="Antigravity Swiss Knife Logo" width="280" style="border-radius: 16px; box-shadow: 0 8px 32px rgba(0,0,0,0.3);" />
  <h3>The Definitive Native Companion, Quota Switcher, Security Auditor, and Feature Extender for Google Antigravity 2.0</h3>
  <p><em>Engineered natively in Go & TypeScript for power users, agent developers, and high-velocity engineering workflows.</em></p>
</div>

---

## 🌟 Executive Overview

**Antigravity Swiss Knife** is a native companion daemon and UI extension suite built specifically for **Google Antigravity 2.0**. It supercharges Antigravity with enterprise-grade multi-account fleet quota switching, custom model relay security auditing, in-chat token & TPS telemetry, cross-agent conversation migrations, and an auxiliary productivity workspace (in-app browser preview, lightweight file explorer, mobile simulator, and quick memos).

Unlike brittle proxy-based MITM solutions, Antigravity Swiss Knife runs **100% natively and locally**:
- **Zero-Loss Keyring Integration**: Manages OAuth2 tokens directly in the Linux Secret Service API (`secret-tool` / `libsecret`), macOS Keychain, and Windows Credential Manager.
- **Session Preservation**: Prevents session dropouts, lost conversation IDs, or `state.vscdb` locking by dynamically synchronizing `app_storage.json`.
- **Atomic Reversibility**: All modifications to Antigravity runtime files create pristine `<file>.swiss.bak` backups, guaranteeing 100% crash safety and instant rollback.

---

## 🚀 Complete Suite Modules

```
┌────────────────────────────────────────────────────────────────────────────────────────┐
│                                Antigravity Swiss Knife                                 │
├─────────────────────┬──────────────────────┬────────────────────┬──────────────────────┤
│ 1. Account Switcher │ 2. Custom Models     │ 3. UI Enhancements │ 4. Feature Plugins   │
│ • 2x2 Gauge Matrix  │ • 6-Probe Security   │ • Jump Bar         │ • App/Web Preview    │
│ • Dual Quota Bars   │   Audit Engine       │ • Turn Counter     │ • Auxiliary Explorer │
│ • MFA / TOTP Vault  │ • Model Canary Test  │ • Density Modes    │ • Mobile Simulator   │
│ • FP Virtualization │ • OpenAI / Anthropic │ • Project Coloring │ • Quick Voice Memos  │
├─────────────────────┼──────────────────────┼────────────────────┼──────────────────────┤
│ 5. Token Monitor    │ 6. Utilities & ACP   │ 7. Automations     │ 8. Archived Projects │
│ • k/M & USD Switch  │ • Chat Migration     │ • Cron Automations │ • Storage Inspector  │
│ • Subagent Telemetry│ • ACP Agent Mesh     │ • Dynamic Sidecars │ • 1-Click Restore    │
│ • Live TPS Display  │ • Project Matching   │ • Template Library │ • Disk Reclamation   │
└─────────────────────┴──────────────────────┴────────────────────┴──────────────────────┘
```

### 1. 🔄 Native Zero-Loss Account Switcher
- **Dual Fleet Quotas (2x2 Gauge Grid)**: When non-Gemini models (Claude 3.7 & GPT-4o) are enabled, the top-right indicator renders a 2x2 circular gauge matrix (top: Gemini 5-Hour & Weekly quota; bottom: Claude/GPT 5-Hour & Weekly quota) separated by a clean divider.
- **Dual Stacked Progress Bars**: The account fleet table renders dual stacked progress bars per row, tracking independent Gemini vs Claude/GPT quota health without window resize distortion.
- **Integrated MFA / TOTP Vault**: Built-in RFC 6238 TOTP engine with auto-copy, live 30s countdown rings, and encrypted backup codes.
- **Hardware Profile Virtualization**: Isolates `machineid`, `.updaterId`, `installation_id`, and `installation_uuid` per account to prevent multi-account correlation bans.
- **Reset Horizon Keep-Alive**: Automatically dispatches a 1-token warmup ping upon quota window rollover (`resetTime`) to prime the next quota period immediately.

### 2. 🛡️ Custom Models & API Security Relay Auditor
- **Multi-Provider Architecture**: Configure Anthropic Claude, OpenAI, DeepSeek, OpenRouter, LiteLLM, and self-hosted Ollama/vLLM endpoints.
- **6-Probe Security Audit Engine** (Inspired by `api-relay-audit`):
  1. *TLS Certificate & Cipher Integrity*: Checks TLS version, cipher suites, and MitM proxy intercepts.
  2. *Origin Lineage & Edge Proxy Inspection*: Detects unverified reverse proxies and untrusted CF-Ray intermediaries.
  3. *Model Substitution Canary Probe*: Verifies that upstream relays do not silently downgrade models (e.g. returning Llama 8B or GPT-4o-mini when GPT-4o was requested).
  4. *Prompt Injection & System Prompt Integrity*: Tests whether custom proxy middleware injects hidden adversarial steering prompts.
  5. *Tool Call Tampering & JSON Payload Integrity*: Validates strict schema preservation during tool execution.
  6. *Error & Diagnostic Leakage*: Ensures error traces do not expose API keys, internal IPs, or environment variables.
- **Google Material Design 3 Audit Modal**: Detailed risk score (A+ to F), gauge meter, probe logs, and copyable Markdown reports.

### 3. 🎨 UI & Workspace Enhancements
- **Prompt Jump Bar**: Sticky navigation bar in chat view allowing instant jumping between user prompt turns with pulse animation.
- **Tool Density Modes**: Switch between `normal` (detailed cards), `muted` (compact minimalist chips), and `hidden` (zero visual clutter) for high-token tool runs.
- **Breaker Line Dividers**: Clean visual separation between successive assistant turns.
- **Visual Project Styling**: Assign distinct Google Material pastel accents and badges to each active workspace.
- **Smart Dynamic Tab Limits**: Automatically manage open chat tabs with fixed or age-based auto-archiving.

### 4. 🧩 Feature Plugins (Exclusive to Antigravity 2.0 Desktop)
- **Auxiliary App & Browser Live Preview**:
  - Embedded browser preview with real-time navigation controls.
  - Interactive Annotation Tools: Red Pen tool (`#ea4335`) and Red Rectangle tool with visual snapping.
  - DOM Element Inspector & Right-Click Commenting: Click any UI element to capture DOM structure.
  - Hybrid Chat Payload Engine: Formats annotations into a rich payload combining cropped viewport screenshot + DOM outerHTML snippet + computed CSS selector for optimal Gemini grounding.
  - iPad Mirroring & Apple Pencil Annotation (On Roadmap).
- **Auxiliary Lightweight File Explorer**:
  - Workspace root dropdown and breadcrumb address bar with manual path input and Back/Forward history.
  - File tree with real-time search filter and right-click context menu (Rename, Copy/Cut/Paste, Delete, Reveal in OS).
  - In-App WYSIWYG Markdown Viewer & Editor.
  - PDF Annotator (Select to Highlight & Underline with export to chat).
  - Lightweight Code Editor with syntax highlighting, line numbers, and "Annotate Selection to Chat" button.
  - Univer / SheetJS Office integration for spreadsheets and tabular datasets.
  - Open Folder in Antigravity Terminal.
  - Remote Filesystem Browser (SSH, Google Cloud Storage, AWS S3) (On Roadmap).
- **Quick Memos**:
  - Floating memo board for instant text snippets and audio voice memos (`MediaRecorder` API).
  - Drag-and-drop memos directly into Antigravity chat input.
- **Mobile Simulator**:
  - Mobile viewport presets (iPhone 16 Pro, Google Pixel 9 Pro, iPad Air).
  - Orientation toggling (Portrait / Landscape) and hardware device bezel toggle.
- **Computer Use Enhancer**:
  - Evaluation matrix comparing Antigravity native computer use against open-source alternatives (OS-World, Open-Computer-Use, Cradle).
  - HiDPI Coordinate Normalization and Linux Wayland PipeWire screen capture grounding.

### 5. ⚡ Token & Cost Monitor
- **Real-Time Token Tracking**: Monitors input prompt tokens, cached input tokens, and generated output tokens for native Gemini and custom models.
- **Unit Toggle**: 1-click toggle between `Tokens (k/M)` and `USD ($)`.
- **Dynamic Pricing Registry**: Auto-fetches current per-1M token rates from LiteLLM and OpenRouter indices with manual override capabilities.
- **Multi-Agent / Subagent Aggregation Engine**:
  - In Antigravity 2.0 chat, injects a telemetry footer below each agent message.
  - When an orchestrator spawns multiple subagents (e.g. `research`, `code-review`), the daemon aggregates tokens across all spawned tree nodes into a single consolidated response badge:
    `⚡ 18,240 tokens (Prompt: 14,200 | Cached: 9,800 [69%] | Output: 4,040) • 76.2 TPS • $0.0124 (saved $0.0084) [+2 subagents]`
  - Multi-session disambiguation via session UUID and turn step indexing.
- **Multi-Dimensional Breakdowns**: Analyze usage by Model, Fleet Account, Workspace Project, and Time Horizon (24h, 7d, 30d, All Time).
- **Audit Export**: 1-click CSV export of session telemetry.

### 6. 🛠️ Utilities & Agent Interoperability
- **Cross-Agent Chat & Project Migration Tool** (Inspired by `dsh-chat-import`):
  - Import historical conversations and project trees from Claude Code, Cursor Composer, ChatGPT Data Exports, Windsurf, Copilot, and raw JSON.
  - Intelligent Project Matching Engine: Auto-detects workspace paths and git remote origins to route chats into existing Antigravity projects, or re-creates new Antigravity projects automatically.
  - Execution Modes: 1-click manual import, scheduled cron sync, and real-time directory watch (inotify/fsnotify) for continuous mirroring.
- **Agent Client Protocol (ACP) Status Inspector**:
  - Scans and discovers active local agent processes (Antigravity 2.0, Claude Code CLI, Cursor, Copilot).
  - Pings ACP sockets, verifies handshake latency, and inspects negotiated cross-agent tool sharing (filesystem, terminal, MCP proxies).

---

## 🗺️ Comprehensive Feature Roadmap & Feasibility Matrix

This roadmap classifies all existing and planned features based on **Feasibility** (technical complexity), **Google Implementation Likelihood** (probability Google will natively release this in upcoming Antigravity builds), **Importance** to power users, and current **Implementation Status**.

| Feature / Module | Category | Feasibility | Google Likelihood | Importance | Target Surface | Offline Persistence | Status |
|:---|:---|:---:|:---:|:---:|:---|:---:|:---:|
| **Zero-Loss Keyring Account Switcher** | Fleet Management | Medium | Low | Critical | Desktop Only | Daemon-Assisted | **Completed** |
| **Dual Gemini + Claude/GPT 2x2 Gauges** | Fleet Management | Low | Low | High | Desktop Only | Companion UI | **Completed** |
| **Integrated MFA / TOTP Vault** | Security & Auth | Low | Low | High | Desktop Only | Companion UI | **Completed** |
| **Device Fingerprint Virtualizer** | Anti-Correlation | Medium | Low | Critical | Desktop Only | Persistent (App Closed) | **Completed** |
| **Reset Horizon Warmup Keep-Alive** | Automation | Low | Low | High | Desktop Only | Daemon-Assisted | **Completed** |
| **Brain Cache Pruner & Storage Manager**| Performance | Low | Medium | Medium | Desktop + Ext | Companion UI | **Completed** |
| **Custom Models Setup & Routing** | Model Routing | Low | Medium | Critical | Desktop + Ext | Persistent (App Closed) | **Completed** |
| **API Security Relay Auditor (6 Probes)**| Security & Audit | Medium | Low | High | Desktop + Ext | Companion UI | **Completed** |
| **UI Enhancements (Prompt Jump Bar)** | UI/UX | Low | High | High | Desktop Only | Persistent (App Closed) | **Completed** |
| **Tool Call Density Modes (Muted/Hide)**| UI/UX | Low | High | High | Desktop Only | Persistent (App Closed) | **Completed** |
| **Workspace Color Accents & Badges** | UI/UX | Low | Medium | Medium | Desktop Only | Persistent (App Closed) | **Completed** |
| **Dynamic Tab Limits & Auto-Archive** | Project Lifecycle| Low | Low | Medium | Desktop Only | Persistent (App Closed) | **Completed** |
| **Auxiliary Browser & Live App Preview** | Feature Plugins | Medium | Medium | High | Desktop Only | Persistent (App Closed) | **Completed** |
| **Visual Annotation (Red Pen / Rect)** | Feature Plugins | Medium | Medium | High | Desktop Only | Persistent (App Closed) | **Completed** |
| **Hybrid Chat Payload (Crop + DOM Code)**| Feature Plugins | Medium | Medium | High | Desktop Only | Persistent (App Closed) | **Completed** |
| **Auxiliary Lightweight File Explorer** | Feature Plugins | Medium | Low | High | Desktop Only | Persistent (App Closed) | **Completed** |
| **In-App WYSIWYG Markdown & PDF Mark** | Feature Plugins | Medium | Medium | High | Desktop Only | Persistent (App Closed) | **Completed** |
| **Lightweight Editor ("Annotate to Chat")**| Feature Plugins | Medium | Low | High | Desktop Only | Persistent (App Closed) | **Completed** |
| **Univer / SheetJS Office Viewer** | Feature Plugins | Medium | Low | Medium | Desktop Only | Persistent (App Closed) | **Completed** |
| **Quick Memos (Text & Audio Notes)** | Feature Plugins | Low | Low | Medium | Desktop Only | Persistent (App Closed) | **Completed** |
| **Mobile Simulator (iPhone/Pixel/iPad)** | Feature Plugins | Medium | Low | Medium | Desktop Only | Persistent (App Closed) | **Completed** |
| **Computer Use Enhancer (HiDPI/Wayland)** | Feature Plugins | High | High | High | Desktop Only | Companion UI | **Completed** |
| **Token & Cost Monitor Dashboard** | Telemetry | Low | Medium | Critical | Desktop + Ext | Companion UI | **Completed** |
| **USD ($) vs Tokens (k/M) Toggle** | Telemetry | Low | High | High | Desktop + Ext | Companion UI | **Completed** |
| **In-Chat Token & TPS Response Badge** | Telemetry | Medium | Medium | Critical | Desktop Only | Persistent (App Closed) | **Completed** |
| **Subagent Telemetry Aggregation** | Telemetry | Medium | Low | High | Desktop Only | Persistent (App Closed) | **Completed** |
| **Cross-Agent Chat Importer (Claude/Cursor)**| Interoperability| Medium | Low | High | Desktop + Ext | Companion UI | **Completed** |
| **Project Auto-Matching & Re-Creation** | Interoperability| Medium | Low | High | Desktop + Ext | Companion UI | **Completed** |
| **Directory Watch Sync (inotify Auto-Import)**| Interoperability| Medium | Low | Medium | Desktop + Ext | Daemon-Assisted | **Completed** |
| **ACP Agent Mesh & Status Inspector** | Interoperability| Medium | Medium | High | Desktop + Ext | Companion UI | **Completed** |
| **Scheduled Task Automation Library** | Automations | Low | Medium | High | Desktop + Ext | Daemon-Assisted | **Completed** |
| **Agent Kanban Board & GitHub Issues Sync** | Task Orchestration | Medium | Low | Critical | Desktop Only | Daemon-Assisted + Persistent | **Planned** |
| **Google CodeMender Security Agent Manager** | Security & Remediation | Medium | Medium | High | Desktop + Ext | Companion UI + Daemon | **Planned** |
| **Dev Study Buddy & Focus Body Double** | Productivity & Focus | Medium | Low | High | Desktop Only | Companion UI + Persistent | **Planned** |
| **iPad Sidecar Mirroring & Pencil Draw**| Feature Plugins | Extreme | Low | Low | Desktop Only | Roadmap / Planned | **Planned** |
| **Remote Filesystem Browser (SSH / S3)** | Feature Plugins | High | Low | Medium | Desktop Only | Roadmap / Planned | **Planned** |
| **Multi-Platform Test Sandbox (MicroVM/noVNC)** | Sandboxing & QA | High | Low | High | Desktop Only | Daemon-Assisted | **Backlog** |
| **Self-Hosted vLLM / Ollama Auto-Launcher**| Infrastructure | High | Low | Medium | Desktop + Ext | Under Evaluation | **Backlog** |
| **Native Wayland Overlay Annotations** | System UI | Extreme | Low | Low | Desktop Only | Under Evaluation | **Backlog** |
| **Multi-Agent Video Studio Generator** | Multimedia | High | Low | Low | Desktop + Ext | Out of Scope | **Discarded** |

*Legend*:
- **Feasibility**: Low (Straightforward DOM/API), Medium (Moderate reverse-engineering/IPC), High (Advanced protocol emulation), Extreme (Kernel/OS level display streaming).
- **Google Likelihood**: Low (<20% chance Google builds it), Medium (40-60% chance), High (>80% chance Google incorporates into native roadmap).
- **Offline Persistence**:
  - `Persistent (App Closed)`: Functions continuously inside Antigravity 2.0 even when the Swiss Knife application is completely closed. Injected directly into Antigravity's persistent renderer scripts (`persistent_styles.css`, `persistent_script.js`).
  - `Daemon-Assisted`: Runs in the background via the headless Go daemon without requiring the graphical Swiss Knife companion window.
  - `Companion UI`: Interactive control panel or auditor modals rendered inside the Swiss Knife GUI.
- **Status**: Completed (Shipped and functional), WIP (In development), Planned (On roadmap), Backlog (Under evaluation), Discarded (Out of scope).

---

## 💻 Surface Compatibility: Desktop App vs VS Code Extension

| Dimension | Antigravity 2.0 Desktop App | Antigravity VS Code Extension |
|:---|:---|:---|
| **Underlying Architecture** | Standalone Electron application (`/opt/Antigravity/antigravity`) with raw DOM access via `preload.js` and `app.asar`. | Sandboxed Webview / Language Server extension running within VS Code core process boundaries. |
| **DOM & UI Cosmetic Injection** | Full unrestricted access to sidebar DOM, chat message nodes, top titlebar, and auxiliary panels. | Strictly limited to standard VS Code Webview views; cannot alter parent editor chrome or inject chat buttons. |
| **Auxiliary Browser & App Preview** | Fully supported via Electron `BrowserView` / `WebviewTag` with bypassed CORS and iframe frame-ancestors. | Restricted by VS Code CSP and webview sandbox restrictions; cannot render arbitrary external HTTP pages. |
| **File Explorer & Office Viewers** | Fully supported as an integrated custom auxiliary panel tab. | Redundant with VS Code's native file explorer; office previews require heavy third-party extension dependencies. |
| **Keyring Credential Management** | Native access via loopback IPC daemon to OS secret storage (`secret-tool`, Keychain). | Supported (can communicate with the local Go daemon via HTTP/IPC). |
| **Token Monitor & Pricing** | Full support with in-chat response telemetry badge injected into chat DOM. | Supported via Swiss Knife web dashboard; in-chat footer requires custom webview wrapper. |
| **Chat & Project Migration** | Full support with auto-project recreation directly in Antigravity's storage. | Supported for data transformation, but project recreation is constrained to VS Code workspace files. |

---

## 🔒 Safety, Reversibility, and Persistence Lifecycle

### 1. Offline Persistence (When Swiss Knife Electron App is Closed)
- The core Swiss Knife engine runs as a lightweight headless Go daemon (`bin/swiss daemon`).
- When the user closes the Swiss Knife GUI window, the daemon **continues running in the background** (or via `systemd` user service).
- Quota polling, auto-account switching, background sync, and reset horizon keep-alives remain **100% active** without requiring the Electron app to be open.

### 2. Zero-Risk Atomic Reversibility (`.swiss.bak`)
- Every system or runtime file modified by Antigravity Swiss Knife is preceded by an atomic backup:
  `workbench.desktop.main.js` ➔ `workbench.desktop.main.js.swiss.bak`
- A single command (`bin/swiss uninject` or via the System Settings GUI) restores pristine factory files and restarts the Antigravity desktop app safely.
- If an Antigravity auto-update occurs, Swiss Knife detects signature changes and gracefully pauses injections rather than crashing.

---

## 🛠️ Build & Installation

### Prerequisites
- Go 1.22+
- Node.js 20+ & npm
- Linux (Secret Service API / `secret-tool`), macOS (Keychain), or Windows

### Building from Source

```bash
# 1. Clone repository
git clone https://github.com/ChillingWombat/AntigravitySwissKnife.git
cd "AntigravitySwissKnife"

# 2. Build Web GUI
cd frontend
npm install
npm run build
cd ..

# 3. Build Go Daemon & CLI
go build -o bin/swiss ./cmd/swiss

# 4. Start Daemon & Launch Companion GUI
./bin/swiss daemon --web
```

---

## 📄 License & Compliance

Antigravity Swiss Knife is released under the **MIT License**. It does not redistribute proprietary Google Antigravity binaries or bypass authentication protocols. All credential storage complies strictly with local OS secret management standards.
