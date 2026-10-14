# Project: Antigravity Swiss Knife — Stabilization, Privacy Sanitization & Security Hardening

## Architecture

Antigravity Swiss Knife is a native desktop companion and daemon for Google Antigravity 2.0. It operates locally with zero external network proxying, managing credentials via the OS keyring, tracking quotas via Google CloudCode APIs, isolating per-account hardware profiles, optimizing local brain/conversation caches, and providing an Electron + React 19 / TypeScript / Vite desktop GUI with in-IDE DOM enhancements.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                 Desktop UI (Electron Shell + React 19 Frontend)            │
│  ┌───────────────────────┐  ┌────────────────────────────────────────────┐  │
│  │ Navigation Sidebar    │  │ Main Content Panes                         │  │
│  │ (Accounts / Quotas /  │  │  • Quota Dashboard & Fleet Health          │  │
│  │  MFA Vault / Cache /  │  │  • RFC 6238 MFA Vault (TOTP Authenticator)  │  │
│  │  Models / Settings)   │  │  • Device Fingerprints Inspector           │  │
│  │                       │  │  • Cache Pruner & Storage Analysis         │  │
│  │                       │  │  • Custom BYOM Providers & Quotas          │  │
│  │                       │  │  • Host App Enhancements & Color Picker    │  │
│  └───────────────────────┘  └────────────────────────────────────────────┘  │
│                                   │                                         │
│                HTTP REST API + Local Session Token Auth (127.0.0.1:8765)    │
└───────────────────────────────────┼─────────────────────────────────────────┘
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│               Antigravity Swiss Knife Background Daemon (Go)                │
│  ┌─────────────────────────────────┐   ┌─────────────────────────────────┐  │
│  │ WebGUI HTTP Server (127.0.0.1)  │   │ Upstream Quota Poller Engine    │  │
│  │ (Auth Middleware, Redaction)    │   │ (cloudcode-pa.googleapis.com)   │  │
│  └─────────────────────────────────┘   └─────────────────────────────────┘  │
│  ┌─────────────────────────────────┐   ┌─────────────────────────────────┐  │
│  │ Process Lifecycle & Shield Mgr  │   │ Keyring & Credential Store      │  │
│  │ (Win32 OpenProcess, Singleton)  │   │ (Atomic Write, Secret Storage)  │  │
│  └─────────────────────────────────┘   └─────────────────────────────────┘  │
│  ┌─────────────────────────────────┐   ┌─────────────────────────────────┐  │
│  │ Cache Pruner & Storage Guard    │   │ In-IDE DOM Telemetry & Styler   │  │
│  │ (24h Age Floor, Lock Check)     │   │ (Scoped Conversation Targeting) │  │
│  └─────────────────────────────────┘   └─────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌───────────────────────────────────┴─────────────────────────────────────────┐
│ Host System Integrations:                                                   │
│  • Google Antigravity 2.0 Host App (~/.gemini/antigravity)                   │
│  • Windows / Linux / macOS OS Keyring & SQLite Storage                      │
│  • Electron IPC & CDP Injected Scripts                                      │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Feature Inventory

Every feature from the Survey phase is mapped to an assigned milestone:

| # | Feature | Description | Milestone | Source |
|---|---------|-------------|-----------|--------|
| F01 | `F01_GITHUB_REMOTE_TRACKING` | Create dedicated remote GitHub issues (#48-#51) and tracking subbranches on origin. | M1 | ORIGINAL_REQUEST §R4 |
| F02 | `F02_PRIVACY_DEHARDCODE` | Eliminate hardcoded personal project names ("Arbitrager", "Obsidian-HomePage", "UniMelb - IT", "PrivacyAI") and personal paths across Go backend, frontend, scripts, and tests. | M2 | ORIGINAL_REQUEST §R1 |
| F03 | `F03_DYNAMIC_PROJECT_COLORS` | Refactor `FactoryProjectColors()` to empty map or deterministic dynamic palette hashing; remove forced store healing; allow clean project color deletion in DOM styler. | M2 | ORIGINAL_REQUEST §R1 |
| F04 | `F04_DOM_CONTAINER_SCOPING` | Restrict chat token telemetry injection exclusively to `[data-testid="conversation-view"]`, ignoring Settings dialogs, Skills lists, Plugins pages, and auxiliary panels. | M3 | ORIGINAL_REQUEST §R2 |
| F05 | `F05_FEEDBACK_ROW_ANCHORING` | Anchor telemetry pills exclusively to native assistant feedback buttons (`Good response` / `Bad response`), with negative exclusion guards for code blocks, modals, and composers. | M3 | ORIGINAL_REQUEST §R2 |
| F06 | `F06_DOM_CLEANUP_CSS_SUPPRESSION` | Active DOM modal cleanup and dual-layer CSS suppression (`display: none !important`) outside active conversation views. | M3 | ORIGINAL_REQUEST §R2 |
| F07 | `F07_SYSTEMIC_AUTH_MIDDLEWARE` | Top-level authentication middleware enforcing local session token (`X-Session-Token` / `Bearer`) with strict public whitelist (`/`, `/healthz`, `/api/auth/unlock`). | M4 | dev/review HANDOFF P0-1 |
| F08 | `F08_STATUS_TOKEN_REDACTION` | Redact raw OAuth tokens (`access_token`, `refresh_token`, `id_token`) from `GET /api/status`, returning non-sensitive account metadata only. | M4 | dev/review HANDOFF P0-2 |
| F09 | `F09_FLEET_CREDENTIAL_SCRUB` | Remove passwords, TOTP secrets, and live OAuth tokens from `GET /api/quota/fleet` output. | M4 | dev/review HANDOFF P0-3 |
| F10 | `F10_DELETE_PATH_JAIL_GUARD` | Enforce explicit protection in `POST /api/files/delete` rejecting `~`, user home root, and filesystem drive root deletions. | M4 | dev/review HANDOFF P0-4 |
| F11 | `F11_BRIDGE_PIN_TOKEN_REDACT` | Redact SecurityPIN and bot tokens in bridge endpoints (`/api/bridge/remote/config`, `/api/bridge/channels/config`, `/api/bridge/status`) and enforce PIN brute-force lockout. | M4 | dev/review HANDOFF P0-5 |
| F12 | `F12_CUSTOM_MODELS_BYOK_REDACT` | Return `GetConfigRedacted()` in `POST /api/custom_models/refresh_quotas` instead of cleartext keys. | M4 | dev/review HANDOFF P0-6 |
| F13 | `F13_CACHE_PRUNER_AGE_FLOOR` | Enforce 24-hour minimum age floor in cache pruner `MaxSizeGB` mode and check file locks to protect active/suspended sessions. | M4 | dev/review HANDOFF P0-7 |
| F14 | `F14_WIN_DAEMON_SOCKET_PATH` | Shorten AF_UNIX socket path in daemon tests on Windows to comply with Winsock 108-byte limit. | M5 | dev/review HANDOFF §1.1 |
| F15 | `F15_WIN_HOME_TEST_ISOLATION` | Abstract test home directory isolation to set `%USERPROFILE%` alongside `$HOME` in Windows tests (`pkg/github`, `pkg/gui`, `pkg/keyring`, `pkg/webgui`). | M5 | dev/review HANDOFF §1.1 |
| F16 | `F16_WIN_PROCESS_LIVENESS` | Implement Win32 `OpenProcess` / `GetExitCodeProcess` in `pkg/process/shield.go` to replace unsupported `Signal(0)` on Windows. | M5 | dev/review HANDOFF P1-18 |
| F17 | `F17_WIN_POSIX_PERM_GATING` | Gate POSIX mode bit assertions (`0600`/`0700`) on Windows NTFS across `pkg/ipc`, `pkg/vault`, `pkg/gui`, `pkg/keyring`. | M5 | dev/review HANDOFF §1.1 |
| F18 | `F18_KEYRING_AUTOIMPORT_WRITEATOMIC` | Fix auto-import guard early return in `pkg/keyring/store.go` and add retry backoff for Windows file sharing collisions in `writeAtomic`. | M5 | dev/review HANDOFF P1-22 |
| F19 | `F19_WIN_REVIVAL_CLI_BUFFER` | Avoid Win32 32KB command line buffer truncation in `pkg/revival` by piping script via stdin or temporary file. | M5 | dev/review HANDOFF §1.1 |
| F20 | `F20_FULL_BUILD_VERIFICATION` | Clean build of Go backend (`go build ./cmd/swiss`) and frontend (`npm run build`), 100% clean test execution (`go test ./... -count=1`), and PR synchronization. | M6 | ORIGINAL_REQUEST §Acceptance Criteria |

---

## Milestones

| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| M1 | Remote GitHub Issue Tracking & Subbranches | Create 4 GitHub issues on origin; create and push 4 dedicated feature branches. | none | DONE |
| M2 | Phase 1: Privacy Sanitization & Factory Defaults | Purge personal project names & paths; dynamic color palette; clean test fixtures. (F02, F03) | M1 | DONE |
| M3 | Phase 1: Windows DOM Telemetry Isolation | Strict conversation container scoping; negative exclusion guards; feedback row anchoring. (F04, F05, F06) | M1 | DONE |
| M4 | Phase 2: Systemic P0 Security Hardening | Auth middleware; token redaction; fleet scrubbing; path jail guard; bridge PIN; BYOK; pruner age floor. (F07-F13) | M2, M3 | DONE |
| M5 | Phase 2: Windows Go Test Suite Stabilization | Fix 9 failing packages: socket limits, USERPROFILE isolation, Win32 liveness, permissions gating, keyring autoimport, revival CLI. (F14-F19) | M4 | DONE |
| M6 | Final Milestone: Full Build & Test Verification | 100% Go test pass across all packages, frontend production build, daemon binary compilation, PR promotion & sync. (F20) | M5 | DONE |

---

## Interface Contracts

### WebGUI ↔ Local Session Token Auth (M4)
- **Token Location**: `%APPDATA%/antigravity-swiss/session.token` (Windows) or `~/.config/antigravity-swiss/session.token` (Linux/macOS) with 0600 permissions.
- **Request Header**: `X-Session-Token: <hex_token>` or `Authorization: Bearer <hex_token>`.
- **Public Whitelist**: `/`, `/healthz`, `/index.html`, `/assets/*`, `/api/auth/unlock`, `/mcp`.
- **Rejection Status**: HTTP 401 Unauthorized (`{"error": "unauthorized", "code": "AUTH_REQUIRED"}`).

### Redacted Account Output (M4)
- **Output Struct**: `redactAccount(acc)` must strip `password`, `totp_secret`, `access_token`, `id_token`.
- `refresh_token` in fleet summary set to `"[REDACTED]"` if present to preserve frontend health calculation without leaking secret.

### Windows DOM Telemetry Scoping (M3)
- **Container**: `document.querySelector('[data-testid="conversation-view"]')`.
- **Feedback Anchors**: `button[aria-label="Good response"]`, `button[aria-label="Bad response"]`.
- **Negative Selectors**: `[role="dialog"]`, `[aria-modal="true"]`, `.settings-tab-content`, `[data-testid*="settings"]`, `#swiss-aux-container`, `pre`, `code`.

---

## Code Layout

```
cmd/swiss/               # CLI and Daemon entry points
pkg/
├── bridge/              # Remote bridge & tunnel (tunnel.go, remote.go, channels.go)
├── cache/               # Brain/conversation cache pruner (pruner.go, inspector.go)
├── custommodels/        # BYOM custom model configurations (store.go, tester.go)
├── daemon/              # Background daemon & RPC (daemon.go, subagent_tracker.go)
├── gui/                 # Project styling & settings store (models.go, store.go, styler.go)
├── keyring/             # OS keyring, account sync & scanner (store.go, sync.go, scanner.go)
├── process/             # Process detection & shield (shield.go, lifecycle.go)
├── plugins/             # In-IDE injected scripts & styles (auxiliary.go)
├── quota/               # Quota calculation & polling (calculator.go, client.go)
└── webgui/              # HTTP API server & embedded UI (server.go, token_monitor.go)
frontend/                # React 19 + TypeScript + Vite frontend
electron/                # Electron desktop wrapper (main.js)
tests/e2e/               # Dual Track Opaque-box E2E test suites
```
