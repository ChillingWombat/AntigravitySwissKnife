# Project: Antigravity Swiss Knife

## Architecture

Antigravity Swiss Knife is an independent native desktop manager, background daemon, and multi-tool for Google Antigravity 2.0 on Linux (with cross-platform architecture). It operates 100% locally with zero network proxying, managing credentials via the native Linux Secret Service (`secret-tool` / libsecret), tracking quotas via upstream Google CloudCode APIs, isolating per-account hardware profiles, optimizing local brain/conversation caches, and providing a Google Gemini Material Design 3 dark desktop GUI.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                 Desktop User Interface (PySide6 / Qt 6)                     │
│  ┌───────────────────────┐  ┌────────────────────────────────────────────┐  │
│  │ Fixed Left Nav Rail   │  │ Account Switcher Top Ribbon (5 Sub-Pages)  │  │
│  │ (Switcher / Market /  │  │  1. Quota Dashboard (4 Circular Gauges)    │  │
│  │  Settings)            │  │  2. Accounts & RFC 6238 MFA/TOTP Vault     │  │
│  │                       │  │  3. Device Fingerprints Manager            │  │
│  │                       │  │  4. Brain Cache Manager                    │  │
│  │                       │  │  5. Switcher Settings                      │  │
│  └───────────────────────┘  └────────────────────────────────────────────┘  │
│  ┌───────────────────────┐  ┌────────────────────────────────────────────┐  │
│  │ Custom Gauge Widgets  │  │ Animated TOTP 30s Countdown Ring Widget    │  │
│  └───────────────────────┘  └────────────────────────────────────────────┘  │
│                                   │                                         │
│                       QLocalSocket / JSON-RPC IPC                           │
└───────────────────────────────────┼─────────────────────────────────────────┘
                                    ▼
       Unix Domain Socket: $XDG_RUNTIME_DIR/antigravity-swiss/daemon.sock
                                    ▲
┌───────────────────────────────────┼─────────────────────────────────────────┐
│               Antigravity Swiss Knife Background Daemon                     │
│  ┌─────────────────────────────────┐   ┌─────────────────────────────────┐  │
│  │ Native Secret Service Switcher  │   │ Upstream Quota Poller Engine    │  │
│  │ (secret-tool service=gemini)    │   │ (cloudcode-pa.googleapis.com)   │  │
│  └─────────────────────────────────┘   └─────────────────────────────────┘  │
│  ┌─────────────────────────────────┐   ┌─────────────────────────────────┐  │
│  │ Process Lifecycle & Session Mgr │   │ Reset Horizon Warmup Engine     │  │
│  │ (SingletonLock / app_storage)   │   │ (maxOutputTokens: 1 keep-alive) │  │
│  └─────────────────────────────────┘   └─────────────────────────────────┘  │
│  ┌─────────────────────────────────┐   ┌─────────────────────────────────┐  │
│  │ Device Fingerprint Virtualizer  │   │ Brain & Context Cache Optimizer │  │
│  │ (machineid / updaterId / pbtxt) │   │ (brain/ & conversations/ pruner)│  │
│  └─────────────────────────────────┘   └─────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌───────────────────────────────────┴─────────────────────────────────────────┐
│ Host System Integrations:                                                   │
│  • Linux Secret Service API (org.freedesktop.Secret.Generic)                │
│  • System Tray (org.kde.StatusNotifierItem via DBus)                        │
│  • Antigravity Config (~/.config/Antigravity & ~/.gemini/antigravity)       │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Feature Inventory

| # | Feature | Description | Milestone | Source |
|---|---------|-------------|-----------|--------|
| F01 | `F01_SECRET_LOOKUP_STORE` | Linux Secret Service API wrapper using `secret-tool` / libsecret with attributes `service=gemini`, `username=antigravity` matching `zalando/go-keyring`. | M1 | ORIGINAL_REQUEST §R1 |
| F02 | `F02_ATOMIC_KEYRING_SWITCH` | Atomic rotation of credentials in Secret Service to the next healthy account upon quota exhaustion or manual trigger. | M1 | ORIGINAL_REQUEST §R1 |
| F03 | `F03_SESSION_PRESERVATION` | Preservation of active conversation ID (`cascadeId`), multi-conversation layout nodes, and open aux pane tabs in `app_storage.json`. | M1 | ORIGINAL_REQUEST §R1 |
| F04 | `F04_PROCESS_LIFECYCLE_MGR` | Graceful process termination via `SIGTERM` to the main Electron PID (`SingletonLock`), clean exit polling, lock cleanup, and detached relaunch. | M1 | ORIGINAL_REQUEST §R1 |
| F05 | `F05_SQLITE_INTEGRITY` | Synchronize and protect SQLite WAL databases (`state.vscdb`, `conversation_summaries.db`) against corruption during process termination. | M1 | ORIGINAL_REQUEST §R1 |
| F06 | `F06_QUOTA_SUMMARY_POLLER` | Polling `POST /v1internal:retrieveUserQuotaSummary` with OAuth token, parsing buckets (5h and weekly windows), `remainingFraction`, and `resetTime`. | M2 | ORIGINAL_REQUEST §R3 |
| F07 | `F07_MODEL_CATALOG_FETCHER` | Polling `POST /v1internal:fetchAvailableModels`, parsing model catalogs, `tieredModelIds` (flashLite, flash, pro), and per-model quota info. | M2 | ORIGINAL_REQUEST §R3 |
| F08 | `F08_RESET_HORIZON_WARMUP` | Automated 1-token keep-alive warmup engine (`POST /v1internal:generateContent` with `maxOutputTokens: 1`), HTTP Date drift correction, jitter, and backoff. | M2 | ORIGINAL_REQUEST §R3 |
| F09 | `F09_AUTO_SWITCH_RULE_ENGINE` | Configurable threshold evaluator comparing active model quota against limits (e.g. 5%), selecting the next eligible account and initiating switch. | M2 | ORIGINAL_REQUEST §R1, R3 |
| F10 | `F10_DEVICE_FINGERPRINT_ISOLATION` | Per-account isolation and virtualization of hardware profiles: `machineid` (36B UUID), `.updaterId` (36B UUID), `installation_id` (36B UUID), and `installation_uuid` in `antigravity_state.pbtxt`. | M3 | ORIGINAL_REQUEST §R4 |
| F11 | `F11_PROFILE_SWAPPER` | Atomic synchronization and swapping of virtual device fingerprint profiles concurrently with keyring credential rotation. | M3 | ORIGINAL_REQUEST §R4 |
| F12 | `F12_BRAIN_CACHE_INSPECTOR` | Deep disk usage scanning and categorization of `~/.gemini/antigravity/brain/` (screenshots, scratch, tool logs, tasks, step outputs) and `conversations/` SQLite files. | M3 | ORIGINAL_REQUEST §R5 |
| F13 | `F13_BRAIN_CACHE_PRUNER` | Safe cleanup routines for stale tasks and scratch directories without disturbing active sessions or referenced artifacts. | M3 | ORIGINAL_REQUEST §R5 |
| F14 | `F14_PROMPT_CACHE_OPTIMIZER` | Context cache analyzer identifying redundant prompts and token overhead to optimize memory and API usage. | M3 | ORIGINAL_REQUEST §R5 |
| F15 | `F15_GEMINI_M3_THEME` | Complete Google Gemini Material Design 3 dark theme styling (`#131314` surface, `#1e1f20` cards, `#8ab4f8` accents, `#81c995`/`#fdd663`/`#f28b82` status colors, 16px card radius, 18px pill tabs). | M4 | ORIGINAL_REQUEST §R2 |
| F16 | `F16_FIXED_LEFT_NAV_RAIL` | Collapsible/fixed navigation rail (72px) switching between top-level Swiss Knife suite tools (Account Switcher, Tools Marketplace / Extensions, System & Tray Settings). | M4 | ORIGINAL_REQUEST §R2 |
| F17 | `F17_ACCOUNT_SWITCHER_TOP_RIBBON` | 5-tab top navigation ribbon (Quota Dashboard, Accounts & MFA Vault, Device Fingerprints, Brain Cache Manager, Switcher Settings). | M4 | ORIGINAL_REQUEST §R2 |
| F18 | `F18_QUOTA_DASHBOARD_VIEW` | Real-time vector gauge meters for Gemini 3.8 Flash, Flash Lite, Pro, and Claude; active account status badge; reset countdown; 1-click manual switch. | M4 | ORIGINAL_REQUEST §R2 |
| F19 | `F19_ACCOUNTS_MFA_VAULT_VIEW` | Multi-account inventory, credential management, backup codes, and live RFC 6238 TOTP engine. | M4 | ORIGINAL_REQUEST §R2 |
| F20 | `F20_RFC6238_TOTP_ENGINE` | Pure Python RFC 6238 TOTP computation (HMAC-SHA1, 30s step, 6-digit codes) with live animated countdown ring widget (`TotpCountdownRingWidget`). | M4 | ORIGINAL_REQUEST §R2 |
| F21 | `F21_DEVICE_FINGERPRINTS_VIEW` | Interactive inspector and generator for virtualized hardware profiles. | M4 | ORIGINAL_REQUEST §R2 |
| F22 | `F22_BRAIN_CACHE_VIEW` | Visual disk breakdown chart and safe cleanup action triggers. | M4 | ORIGINAL_REQUEST §R2 |
| F23 | `F23_SWITCHER_SETTINGS_VIEW` | Sliders for threshold percentages, polling interval inputs, keep-alive warmup toggle. | M4 | ORIGINAL_REQUEST §R2 |
| F24 | `F24_SYSTEM_TRAY_INTEGRATION` | DBus StatusNotifierItem (SNI) integration via `QSystemTrayIcon` with status badge, notifications, and quick-switch context menu. | M4 | ORIGINAL_REQUEST §R2 |
| F25 | `F25_DAEMON_IPC_CORE` | Headless daemon with Unix Domain Socket (`$XDG_RUNTIME_DIR/antigravity-swiss/daemon.sock`) JSON-RPC 2.0 / NDJSON server, pub-sub event notifications, and in-process fallback controller. | M1 | ORIGINAL_REQUEST §R1 |
| F26 | `F26_OFFLINE_MOCK_HARNESS` | Offline mock server test harness simulating `cloudcode-pa.googleapis.com` responses, quota exhaustion, reset countdowns, and warmup pings for hermetic CI. | M2 | Survey / Architecture |

---

## Milestones

| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| M1 | Core Daemon, IPC & Linux Keyring Account Switcher | Features F01, F02, F03, F04, F05, F25. Linux Secret Service wrapper (`secret-tool`), app_storage.json session preservation, process lifecycle & clean relauncher, Unix Domain Socket JSON-RPC daemon. | none | DONE (Certified by Reviewers, Challengers, Auditor; 335 tests pass) |
| M2 | Upstream Quota Poller, Warmup Engine & Rule Engine | Features F06, F07, F08, F09, F26. CloudCode retrieveUserQuotaSummary / fetchAvailableModels poller, 1-token keep-alive ping engine, threshold auto-switch engine, and hermetic offline mock server. | M1 | IN_PROGRESS |
| M3 | Device Fingerprint Virtualizer & Brain Cache Optimizer | Features F10, F11, F12, F13, F14. Per-account machineid/.updaterId/installation_id/antigravity_state.pbtxt profile manager & atomic swapper; brain/ and conversations/ disk inspector and safe pruning engine. | M1 | PLANNED |
| M4 | Google Gemini M3 Desktop GUI & RFC 6238 TOTP Vault | Features F15, F16, F17, F18, F19, F20, F21, F22, F23, F24. PySide6 desktop GUI with strict Google Gemini dark styling (#131314, #1e1f20, #8ab4f8), fixed left rail, 5-tab top ribbon, circular gauges, RFC 6238 TOTP engine with countdown ring, and DBus SNI system tray. | M1, M2, M3 | PLANNED |
| M5 | Final Milestone: Full System Integration & E2E Validation | Phase 1: 100% pass of E2E test suite (Tiers 1-4). Phase 2: Adversarial coverage hardening (Tier 5) with Challengers and Forensic Auditor. | M1, M2, M3, M4 | PLANNED |

---

## Interface Contracts

### 1. `antigravity_swiss.keyring` ↔ `antigravity_swiss.process`
```python
class KeyringCredential(NamedTuple):
    access_token: str
    refresh_token: str
    token_type: str
    expiry: str
    auth_method: str
    id_token: str

class KeyringService:
    def get_active_credential() -> KeyringCredential: ...
    def set_active_credential(cred: KeyringCredential) -> None: ...
    def list_accounts() -> list[str]: ...
    def switch_account(account_email: str) -> bool: ...

class ProcessManager:
    def get_running_antigravity_pid() -> int | None: ...
    def terminate_gracefully(timeout_sec: float = 10.0) -> bool: ...
    def relaunch(conversation_id: str | None = None) -> int: ...
```

### 2. `antigravity_swiss.quota` ↔ `antigravity_swiss.warmup`
```python
class ModelQuotaBucket(NamedTuple):
    bucket_id: str
    display_name: str
    window: str  # "5h" | "weekly"
    remaining_fraction: float  # 0.0 to 1.0
    reset_time: datetime.datetime
    description: str

class QuotaSummary(NamedTuple):
    groups: dict[str, list[ModelQuotaBucket]]
    timestamp: datetime.datetime

class QuotaPoller:
    async def poll_summary(access_token: str) -> QuotaSummary: ...
    async def fetch_models(access_token: str) -> dict[str, Any]: ...

class WarmupEngine:
    async def trigger_keepalive(access_token: str, model_id: str = "gemini-3.8-flash-high") -> bool: ...
```

### 3. `antigravity_swiss.fingerprint` ↔ `antigravity_swiss.keyring`
```python
class DeviceProfile(NamedTuple):
    account_email: str
    machine_id: str        # 36B UUIDv4
    updater_id: str        # 36B UUIDv4
    installation_id: str   # 36B UUIDv4
    installation_uuid: str # 36B UUIDv4

class FingerprintManager:
    def get_active_profile() -> DeviceProfile: ...
    def create_or_get_profile(account_email: str) -> DeviceProfile: ...
    def swap_profile_for_account(account_email: str) -> None: ...
```

### 4. `antigravity_swiss.ipc` (Daemon ↔ GUI)
- **Transport**: Unix Domain Socket `$XDG_RUNTIME_DIR/antigravity-swiss/daemon.sock` (mode `0600`)
- **Protocol**: JSON-RPC 2.0 / NDJSON
- **Methods**:
  - `status.get() -> DaemonStatus`
  - `quota.get_summary() -> QuotaSummary`
  - `accounts.list() -> list[AccountItem]`
  - `accounts.switch(email) -> SwitchResult`
  - `vault.totp_generate(secret) -> TotpCodeResult`
  - `cache.get_breakdown() -> CacheBreakdown`
  - `cache.prune(options) -> PruneResult`
- **Events (Pub-Sub)**:
  - `notify.quota_updated { summary }`
  - `notify.account_switched { email, reason }`
  - `notify.warmup_triggered { reset_time, next_window }`

---

## Code Layout

```
antigravity_swiss/
├── __init__.py
├── __main__.py               # CLI entry point: "python -m antigravity_swiss [daemon|gui|switch|status]"
├── core/
│   ├── __init__.py
│   ├── config.py             # App configuration, paths, defaults
│   ├── constants.py          # Material 3 colors, Google endpoints, schema names
│   └── errors.py             # Custom domain exceptions
├── keyring/
│   ├── __init__.py
│   ├── secret_tool.py        # CLI secret-tool wrapper
│   ├── dbus_keyring.py       # Pure Python D-Bus Secret Service fallback
│   └── switcher.py           # Atomic credential rotation logic
├── session/
│   ├── __init__.py
│   ├── app_storage.py        # app_storage.json reader, serializer, cascadeId extractor
│   └── sqlite_guard.py       # SQLite WAL lock detector and corruption guard
├── process/
│   ├── __init__.py
│   ├── lifecycle.py          # PID detection (SingletonLock), SIGTERM, exit poller, clean relauncher
│   └── lock_manager.py       # SingletonLock / SingletonSocket / SingletonCookie cleaner
├── quota/
│   ├── __init__.py
│   ├── poller.py             # Upstream API client (retrieveUserQuotaSummary, fetchAvailableModels)
│   ├── models.py             # Quota dataclasses and bucket parser
│   └── rule_engine.py        # Auto-switch threshold evaluation engine
├── warmup/
│   ├── __init__.py
│   ├── horizon.py            # Reset horizon scheduler and countdown calculator
│   └── keepalive.py          # 1-token prompt generator and dispatch client
├── fingerprint/
│   ├── __init__.py
│   ├── manager.py            # Hardware profile isolation manager
│   ├── profile_store.py      # Secure store for per-account UUIDs
│   └── pbtxt_parser.py       # antigravity_state.pbtxt safe reader/updater
├── cache_optimizer/
│   ├── __init__.py
│   ├── inspector.py          # Disk usage scanner for brain/ and conversations/
│   ├── pruner.py             # Safe pruning for stale scratchpads and step logs
│   └── prompt_cache.py       # Redundant prompt overhead detector
├── totp/
│   ├── __init__.py
│   └── engine.py             # Pure Python RFC 6238 TOTP computation (HMAC-SHA1, 30s)
├── ipc/
│   ├── __init__.py
│   ├── socket_server.py      # Unix Domain Socket JSON-RPC server (Daemon)
│   ├── socket_client.py      # Unix Domain Socket client (GUI)
│   └── controller.py         # In-process controller fallback for standalone mode
└── gui/
    ├── __init__.py
    ├── app.py                # PySide6 QApplication entry point
    ├── main_window.py        # Main container with left nav rail and top ribbon
    ├── styles.py             # Google Gemini Material Design 3 QSS theme tokens
    ├── tray.py               # DBus StatusNotifierItem (QSystemTrayIcon)
    ├── widgets/
    │   ├── __init__.py
    │   ├── nav_rail.py       # Fixed left navigation rail
    │   ├── top_ribbon.py     # Account Switcher 5-tab top ribbon
    │   ├── circular_gauge.py # QPainter vector circular quota progress gauge
    │   └── countdown_ring.py # Animated vector 30s TOTP countdown ring widget
    └── pages/
        ├── __init__.py
        ├── quota_dashboard.py # Quota gauges, active account, manual switch
        ├── mfa_vault.py       # Accounts list & RFC 6238 TOTP live authenticator
        ├── fingerprints.py    # Hardware profile inspection and generator
        ├── brain_cache.py     # Disk breakdown chart and cleanup actions
        ├── switcher_settings.py # Threshold sliders, polling frequencies, warmup
        ├── extensions.py      # Tools Marketplace placeholder
        └── system_settings.py # System & tray preferences
```
