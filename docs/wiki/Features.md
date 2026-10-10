# Features Overview

Antigravity Swiss Knife organizes its local desktop and daemon operations into 12 functional subsystems. These subsystems map directly to the application navigation rail and top segmented ribbons, giving developers granular control over account rotation, quota telemetry, model routing, IDE runtime patches, and local storage.

---

## 1. Feature Subsystem Architecture

Every feature subsystem runs locally without external network proxies. The table below lists the 12 subsystems, their navigation locations, primary responsibilities, and associated configuration stores.

| Feature Section | Nav Location | Primary Responsibility | Configuration & State Store |
| :--- | :--- | :--- | :--- |
| [Quota Dashboard](Quota-Dashboard.md) | Rail `0` / Ribbon Tab `0` | Real-time 5-hour and weekly quota telemetry across multi-account fleets | `cloudcode-pa.googleapis.com` / `accounts.json` |
| [Accounts & MFA Vault](Accounts-and-MFA-Vault.md) | Rail `0` / Modal | AES-256-GCM encrypted credential vault, OAuth extraction, client RFC 6238 TOTP | `~/.config/antigravity-swiss/accounts.json` |
| [Device Fingerprints](Device-Fingerprints.md) | Rail `0` / Ribbon Tab `1` | Per-account virtualization of machine IDs and installation telemetry | `antigravity_state.pbtxt` / `storage.json` |
| [Brain Cache Manager](Brain-Cache-Manager.md) | Rail `9` / Tab `0` & Rail `0` | Disk usage inspection, TTL pruning, and task scratchpad maintenance | `~/.gemini/antigravity/brain/` / `conversations/` |
| [Switcher Settings](Switcher-Settings.md) | Rail `0` / Ribbon Tab `2` | Threshold policies, polling cadences, standby jitter, and 1-token warmups | `~/.config/antigravity-swiss/rules.json` |
| [Custom Models](Custom-Models.md) | Rail `3` | BYOK integration for Anthropic, OpenAI, AI Studio, DeepSeek, and local Ollama | `~/.config/antigravity-swiss/custom_models.json` |
| [Extensions](Extensions.md) | Rail `7` | Modular IDE gadgets (Preview Browser, File Explorer, Quick Memos, Mobile Simulator) | `~/.config/antigravity-swiss/enhancements.json` |
| [Token Monitor](Token-Monitor.md) | Rail `8` | Real-time token consumption, TPS rates, cost accounting, and pricing registry | SQLite `token_telemetry.db` / `pricing.json` |
| [App Enhancements](App-Enhancements.md) | Rail `4` | Runtime CDP hooks for chat view navigation, token meters, and project color tags | `persistent_script.js` / `persistent_styles.css` |
| [Utilities](Utilities.md) | Rail `9` | Cross-agent chat importer, ACP agent mesh inspector, and Computer Use grounding | Unix Domain Socket IPC / SQLite WAL |
| [GitHub Workspace](GitHub-Workspace.md) | Rail `10` / Rail `7` Gadget | Interactive Kanban board, issue/PR management, and subagent task tracking | Local Git repo / `github_workspace.json` |
| [System Settings](System-Settings.md) | Rail `2` (Pinned Bottom) | Application lock screen password, runtime modes, storage paths, and diagnostics | `~/.config/antigravity-swiss/settings.json` |

---

## 2. Subsystem Categories

### Account & Quota Operations

These modules coordinate multi-account rotations and hardware identity isolation:
- [Quota Dashboard](Quota-Dashboard.md): Inspects live 5-hour rolling burst limits and weekly quota runway across all registered Google accounts.
- [Accounts & MFA Vault](Accounts-and-MFA-Vault.md): Protects OAuth tokens, passwords, and TOTP secret seeds with AES-256-GCM encryption at rest.
- [Device Fingerprints](Device-Fingerprints.md): Virtualizes distinct machine IDs and installation identifiers per account to prevent identity collisions.
- [Switcher Settings](Switcher-Settings.md): Configures threshold percentages, polling frequencies, standby jitter, and reset horizon keep-alive warmups.

### Intelligence & Model Routing

These modules expand model choices and track token consumption:
- [Custom Models](Custom-Models.md): Connects third-party LLMs and local runtimes with automatic 6-probe security validation.
- [Token Monitor](Token-Monitor.md): Tracks real-time input, cached input, and output token usage, computing dollar valuations and token-per-second throughput.

### IDE Host Integration & Extensions

These modules inject developer tooling directly into the host Antigravity workspace:
- [App Enhancements](App-Enhancements.md): Patches DOM elements via Chrome DevTools Protocol to add prompt jump navigators, TPS monitors, and color palettes.
- [Extensions](Extensions.md): Hosts productivity gadgets in the right auxiliary panel, including preview browsers, file tree inspectors, and scratchpads.
- [GitHub Workspace](GitHub-Workspace.md): Provides an embedded Kanban workflow connecting GitHub issues and pull requests to autonomous subagent runs.

### Storage Maintenance & System Control

These modules maintain system hygiene and runtime policies:
- [Brain Cache Manager](Brain-Cache-Manager.md): Scans local disk caches across 5 storage categories and purges orphaned artifacts based on TTL policies.
- [Utilities](Utilities.md): Imports conversations from other AI tools, monitors Agent Client Protocol nodes, and calibrates Wayland/PipeWire displays.
- [System Settings](System-Settings.md): Gates desktop entry behind PBKDF2-HMAC-SHA256 password hashing, configures binary paths, and manages release updates.
