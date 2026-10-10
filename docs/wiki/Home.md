# Antigravity Swiss Knife Documentation Wiki

Welcome to the technical documentation wiki for Antigravity Swiss Knife, the local-first desktop companion and background daemon for Google Antigravity 2.0.

This wiki provides architectural references, subsystem specifications, protocol schemas, and deployment instructions for developers and systems engineers.

---

## Features Index

Antigravity Swiss Knife organizes its capabilities into 12 core functional subsystems under the [Features Overview](Features.md) parent guide:

1. [Quota Dashboard](Quota-Dashboard.md): Real-time 5-hour and weekly quota telemetry, circular gauge meters, and autonomous standby account rotation.
2. [Accounts & MFA Vault](Accounts-and-MFA-Vault.md): AES-256-GCM encrypted credential vault, Google OAuth loopback extractor, and client-side RFC 6238 TOTP generation.
3. [Device Fingerprints](Device-Fingerprints.md): Virtualization and per-account isolation of machine IDs, installation UUIDs, and hardware telemetry tokens.
4. [Brain Cache Manager](Brain-Cache-Manager.md): Filesystem inspection, TTL retention policies, and storage pruning across local session directories.
5. [Switcher Settings](Switcher-Settings.md): Quota switch thresholds, rotation algorithms, polling intervals, standby jitter, and 1-token keep-alive warmups.
6. [Custom Models](Custom-Models.md): Zero-proxy Bring-Your-Own-Key gateway for Anthropic, OpenAI, AI Studio, DeepSeek, and local Ollama runtimes with 6-probe security auditing.
7. [Extensions](Extensions.md): Embedded IDE gadgets including Preview Browser, File Explorer, Quick Memos, and Mobile Simulator viewports.
8. [Token Monitor](Token-Monitor.md): Context accounting, tokens-per-second velocity, cost tracking, and multi-agent token simulation.
9. [App Enhancements](App-Enhancements.md): Chrome DevTools Protocol runtime DOM injection for turn navigators, live TPS indicators, and project color tags.
10. [Utilities](Utilities.md): Cross-agent conversation importer, ACP agent mesh inspector, and Wayland/PipeWire display grounding for visual agents.
11. [GitHub Workspace](GitHub-Workspace.md): Embedded Kanban task board, issue and pull request management, and live subagent execution tracking.
12. [System Settings](System-Settings.md): PBKDF2-HMAC-SHA256 password protection, runtime mode switching, binary paths, and diagnostic log exports.

---

## Technical Deep Dives

1. [Architecture & Multi-Process Daemon](Architecture-and-Multi-Process-Daemon.md): Three-tier architecture, pure-Go daemon process, local IPC over Unix domain sockets and loopback HTTP, SQLite WAL concurrency, and Chrome DevTools Protocol (CDP) live patching.
2. [Multi-App Fleet Sync & Account Switching](Multi-App-Fleet-Sync-and-Account-Switching.md): Fleet management across Antigravity Desktop, CLI (`agy`), and Extension. Shared vs per-app fleet modes, standby account selection scoring, offline credential swapping, reconcile latch, hardware fingerprint virtualization, and 1-token keep-alive probes.
3. [Active Conversation Continuation & Subagent Revival](Active-Conversation-Continuation-and-Subagent-Revival.md): Session state preservation across account switches and application restarts using `pkg/revival`, `SessionDetector`, `RevivalIntent` store (90s TTL, 2 retry limit), `app_storage.json` layout pinning, and post-relaunch CDP prompt injection.
4. [ACP Agent Mesh Expansion](ACP-Agent-Mesh-Expansion.md): Agent Client Protocol mesh architecture and directory of the 9 registered local agent nodes: Antigravity 2.0, Antigravity CLI, Devin, OpenCode, DeepSeek Harness, Pi, Codex, Claude Code, and Cursor. GUI vs CLI node representations and latency inspection.
5. [Packaging, VM Validation & Installation](Packaging,-VM-Validation-and-Installation.md): Cross-platform release packaging (`.deb`, `.nsis`, `.dmg`), dedicated out-of-tree Linux installer (`scripts/install-linux.js`), automated Quickemu VM test matrix on Windows 11 and macOS Ventura, headless binary validation, and Git worktree developer workflows.
6. [Security, Compliance & Cache Management](Security,-Compliance-and-Cache-Management.md): Alignment with Google Cloud Terms of Service, zero-proxy security architecture, risks of external reverse proxies, 6-probe custom model security auditing, and disk cache auto-pruning across 5 storage categories.

---

## Project Specifications & Community

- [Project Roadmap](Roadmap.md): Completed milestones, near-term priorities, and planned future capabilities including fine-grained token breakdown (skills vs harness overhead).
- [Credits & Acknowledgements](Credits.md): Open-source libraries, protocol standards, and tools that contributed to Antigravity Swiss Knife.

---

## Platform Support & Compatibility Matrix

Antigravity Swiss Knife targets modern 64-bit desktop operating systems. Keyring access uses native OS APIs directly without third-party cloud intermediaries.

| Operating System | Supported Architecture | Native Keyring Backend | Installer Format | Default Install Path |
| :--- | :--- | :--- | :--- | :--- |
| **Linux** (Debian, Ubuntu, Fedora, Arch) | x86_64 (`amd64`) | Secret Service API (`libsecret` / DBus) | `.deb` package or `scripts/install-linux.js` | `/opt/antigravity-swiss-knife` or `~/.local/share/antigravity-swiss-knife/app` |
| **macOS** (12 Monterey through 15 Sequoia) | Apple Silicon (`arm64`), Intel (`x64`) | macOS Keychain Services (`security` framework) | Apple Disk Image (`.dmg`) | `/Applications/Antigravity Swiss Knife.app` |
| **Windows** (Windows 10, Windows 11) | x86_64 (`x64`) | Windows Credential Manager (`wincred.h`) | NSIS Installer (`.exe`) | `%LOCALAPPDATA%\Programs\Antigravity Swiss Knife` |

---

## Core Technical Invariants

All modules and submodules in Antigravity Swiss Knife maintain four non-negotiable operational invariants:

1. **Local-First Execution**: The supervisory interface, Go daemon binary, and runtime scripts execute exclusively on the local host machine. No telemetry or credentials leave the local machine.
2. **Zero-Proxy Architecture**: The tool operates through direct runtime hooks and native local IPC. It never deploys an HTTP MITM proxy or tunnel between Antigravity and upstream Google endpoints, eliminating credential exposure and account flags.
3. **Native Secret Isolation**: Authentication tokens and refresh grants reside in operating system keyrings. Disk files only store non-sensitive configuration parameters and account metadata (email, account UUID, plan tier).
4. **Deterministic Reversibility**: Any modification made to Antigravity runtime files or configuration state creates a `.swiss.bak` snapshot prior to writing. The system can be cleanly uninstalled or reverted at any time without data loss.
