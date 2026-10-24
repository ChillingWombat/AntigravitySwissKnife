# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Developers and technical operators who use Google Antigravity 2.0, its CLI, or its VS Code extension. They work inside an active coding session and need to switch accounts, inspect quota and telemetry, manage models and extensions, or operate remote agent bridges without leaving that session. Linux is the primary development platform; Windows and macOS are supported targets.

## Product Purpose

Antigravity Swiss Knife is a desktop companion and local background daemon for Google Antigravity. It exposes account, quota, model, extension, cache, telemetry, and remote-access controls in one operational surface. Success means the interface reports and changes the real local runtime state without requiring users to edit configuration files or expose credentials.

## Positioning

Swiss Knife is a local control plane, not an API proxy. It coordinates Antigravity through the OS keyring, SQLite WAL state, loopback HTTP, and the Chrome DevTools Protocol while keeping provider traffic on its native path. The product makes multi-account and multi-app operation observable and reversible from the desktop.

## Operating Context

The Electron shell talks to a loopback daemon at `127.0.0.1:8765`. The daemon can be stopped or unresolved while the shell is starting, so navigation must remain usable during partial status. Users work in dense, repeated sessions with a mouse and keyboard, across light and dark themes, and may inspect the same state through Antigravity's app, CLI, or VS Code extension.

Remote workflows include Cloudflare and Tailscale tunnels, pairing PINs, application passwords, OAuth clients, and chat bridges. Those surfaces handle sensitive local credentials and must distinguish live, stopped, loading, unavailable, and error states.

## Capabilities and Constraints

- Manage Antigravity accounts, fleet quota, hardware fingerprints, and account rotation.
- Configure custom models, thinking levels, app enhancements, extensions, and GitHub workspace tools.
- Inspect token consumption, logs, cache data, utilities, and ACP agent nodes.
- Run remote control, Gemini, WhatsApp, Telegram, and Discord bridges.
- Keep credential and provider traffic local to the machine; do not introduce an interception proxy.
- Preserve the full call chain from UI action to daemon, provider, and verified response.
- Never display a connected, ready, or successful state without backing runtime evidence.
- Support a 200px navigation rail, 72px header, 4px layout grid, and dense desktop information layouts.

## Brand Commitments

The product is named Antigravity Swiss Knife and is an independent community project, not affiliated with or endorsed by Google. Its visual language is functional minimalism: high signal, low ornament, restrained blue accents, precise borders, Lucide icons only, and no decorative emojis. Copy is direct, technical, and operational rather than promotional or tutorial-like.

## Evidence on Hand

- `README.md` documents product scope, features, installation, and platform support.
- `docs/wiki/` contains workflow and architecture evidence for accounts, quota, extensions, models, remote access, cache, telemetry, and packaging.
- `frontend/src/` contains the shipped React interface, component tests, semantic tokens, and state handling.
- `cmd/swiss/` and `pkg/` contain the companion daemon and local integration layer.
- No fabricated customer names, adoption metrics, testimonials, or benchmark claims are available.

## Product Principles

1. Show the runtime truth; unresolved state stays visibly unresolved.
2. Optimize for repeated operator tasks, scanability, and fast recovery.
3. Keep local data and credentials under local control.
4. Make actions explicit and reversible, with confirmation for destructive work.
5. Prefer familiar controls and exact status over decorative novelty.

## Accessibility & Inclusion

All primary workflows must be keyboard-operable. Focus must remain visible on buttons, links, inputs, and custom controls. Status cannot rely on color alone; pair it with text or a control label. Body text and form text must meet WCAG AA contrast in both themes, and motion must communicate state rather than delay the task.
