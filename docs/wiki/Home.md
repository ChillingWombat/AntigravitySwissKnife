# Antigravity Swiss Knife Documentation Wiki

Welcome to the technical documentation wiki for Antigravity Swiss Knife, the local-first desktop companion and background daemon for Google Antigravity 2.0.

This wiki provides architectural references, subsystem specifications, protocol schemas, and deployment instructions for developers and systems engineers.

---

## Technical Guides Index

1. [Architecture & Multi-Process Daemon](Architecture-and-Multi-Process-Daemon.md)  
   Three-tier architecture, pure-Go daemon process, local IPC over Unix domain sockets and loopback HTTP, SQLite WAL concurrency, and Chrome DevTools Protocol (CDP) live patching.

2. [Multi-App Fleet Sync & Account Switching](Multi-App-Fleet-Sync-and-Account-Switching.md)  
   Fleet management across Antigravity Desktop, CLI (`agy`), and Extension. Details on shared vs per-app fleet modes, standby account selection scoring, offline credential swapping, reconcile latch, hardware fingerprint virtualization, and 1-token keep-alive probes.

3. [Active Conversation Continuation & Subagent Revival](Active-Conversation-Continuation-and-Subagent-Revival.md)  
   Session state preservation across account switches and application restarts using `pkg/revival`, `SessionDetector`, `RevivalIntent` store (90s TTL, 2 retry limit), `app_storage.json` layout pinning, and post-relaunch CDP prompt injection.

4. [ACP Agent Mesh Expansion](ACP-Agent-Mesh-Expansion.md)  
   Agent Client Protocol mesh architecture and directory of the 9 registered local agent nodes: Antigravity 2.0, Antigravity CLI, Devin, OpenCode, DeepSeek Harness, Pi, Codex, Claude Code, and Cursor. GUI vs CLI node representations and latency inspection.

5. [Packaging, VM Validation & Installation](Packaging,-VM-Validation-and-Installation.md)  
   Cross-platform release packaging (`.deb`, `.nsis`, `.dmg`), dedicated out-of-tree Linux installer (`scripts/install-linux.js`), automated Quickemu VM test matrix on Windows 11 and macOS Ventura, headless binary validation, and Git worktree developer workflows.

6. [Security, Compliance & Cache Management](Security,-Compliance-and-Cache-Management.md)  
   Alignment with Google Cloud Terms of Service, zero-proxy security architecture, risks of external reverse proxies, 6-probe custom model security auditing, and disk cache auto-pruning across 5 storage categories.

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
