# Utilities

[Features](Features.md) / Utilities

The Utilities subsystem bundles maintenance and interoperability tools into four specialized tabs: Storage Manager, ACP Agent Mesh, Chat & Project Importer, and Computer Use.

---

## 1. Storage Manager

Provides direct embedded access to the [Brain Cache Manager](Brain-Cache-Manager.md). Developers can inspect local filesystem allocation, view reclaimable disk space across scratchpads and tool artifacts, and trigger manual or scheduled purge passes without leaving the utility dock.

---

## 2. ACP Agent Mesh Inspector

The Agent Client Protocol (ACP) inspector provides observability into local agent runtimes communicating across the workstation:
- **Registered Agent Directory**: Monitors active connections to 9 standard local agent nodes: Antigravity Desktop, Antigravity CLI (`agy`), Claude Code, Cursor, OpenCode, Codex, Devin, Pi, and DeepSeek Harness.
- **Node Topology**: Differentiates GUI-backed nodes from headless background CLI workers.
- **Handshake Validation**: Dispatches synthetic ping probes to measure inter-process socket latency, handshake round-trip times, and protocol compliance.
- **Live Event Log**: Surfaces streaming JSON-RPC message logs exchanged across local agent endpoints.

---

## 3. Chat & Project Importer

Facilitates cross-tool developer migrations by converting conversation histories and context memories from third-party tools into native Antigravity formats:
- **Supported Source Systems**: Imports sessions from OpenCode, Cursor (`state.vscdb`), Claude Code local archives, and ChatGPT JSON exports.
- **Project Match Modes**:
  - `Auto Match`: Automatically maps external conversation paths to existing Antigravity projects by comparing repository root fingerprints.
  - `Manual Target`: Directs imported threads into a designated project workspace.
- **Sync Options**: Supports one-time manual batch imports, scheduled periodic polling, or filesystem watch modes that ingest external conversation updates automatically.
- **Historical Migration Log**: Records all completed migrations with source timestamps and message counts in local storage.

---

## 4. Computer Use Enhancer

Calibrates the operating system environment for autonomous visual desktop agents:
- **DPI Coordinate Normalization**: Corrects mouse coordinate calculations on high-DPI displays (such as 4K monitors with 150% or 200% scaling) to ensure pixel-perfect clicks and drags.
- **Wayland & PipeWire Integration**: On modern Linux desktops, configures screen capture permissions and bridges XDG Desktop Portals to provide visual framebuffer streams without tearing.
- **Accessibility Tree Grounding**: Bridges system accessibility APIs (AT-SPI on Linux, UI Automation on Windows, Accessibility on macOS) to give visual agents semantic DOM metadata alongside raw screenshot pixels.
