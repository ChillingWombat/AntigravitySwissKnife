# Test Infrastructure Specification: Antigravity Swiss Knife

## 1. Test Philosophy & Principles

Antigravity Swiss Knife is a mission-critical Linux desktop companion and daemon for Google Antigravity 2.0. Because it manages real user OAuth credentials, manipulates live processes, updates configuration files, and communicates with Google APIs, the testing philosophy is built on **opaque-box, requirement-driven verification**:

1. **Opaque-Box Boundary Testing**: Tests execute against external user/client boundaries:
   - **CLI Entrypoints**: `python -m antigravity_swiss [daemon|gui|switch|status|cache|vault]`
   - **Unix Domain Socket IPC**: JSON-RPC 2.0 requests, responses, and pub-sub notification streams over `$XDG_RUNTIME_DIR/antigravity-swiss/daemon.sock`
   - **OS Keyring Contract**: Linux Secret Service API (`secret-tool lookup/store/clear` with `service=gemini`, `username=antigravity`)
   - **Filesystem State**: Strict inspection of `~/.config/Antigravity/` and `~/.gemini/antigravity/` file schemas, byte lengths, and database integrity
2. **Hermetic & Isolated**: Zero external network calls. Upstream Google CloudCode endpoints (`cloudcode-pa.googleapis.com`) and token endpoints (`oauth2.googleapis.com`) are served by an in-process / loopback mock HTTP server (`OfflineMockServer`). Host secrets and running user processes are completely shielded by redirecting `HOME`, `XDG_CONFIG_HOME`, `XDG_RUNTIME_DIR`, and secret backends to isolated temporary directory fixtures.
3. **Requirement-Driven Expected Outputs**: All test assertions derive directly from authoritative specifications:
   - `ORIGINAL_REQUEST.md` (§R1 through §R5)
   - `PROJECT.md` Feature Inventory (F01 - F26) and Interface Contracts
   - Verified system traces and binary descriptors mined in Phase 0 (`spec_miner_env_1`, `spec_miner_quota_1`, `explorer_ui_1`)
4. **Adversarial & Boundary Rigor**: Testing empty strings, malformed JSON, corrupted SQLite WAL files, protobuf syntax corruptions, clock skews, missing locks, and quota boundary conditions (0.0%, 100.0%, negative fractions).

---

## 2. Complete Feature Inventory Coverage (F01 to F26)

Every feature in `PROJECT.md` is mapped across Tier 1 (Happy Path), Tier 2 (Boundary & Error Cases), Tier 3 (Pairwise Interactions), and Tier 4 (Real-World Scenarios):

| Feature ID | Feature Name | Description | Tier 1 (Happy Path) | Tier 2 (Boundaries) | Tier 3 (Pairwise) | Tier 4 (Scenario) |
|---|---|---|:---:|:---:|:---:|:---:|
| **F01** | `F01_SECRET_LOOKUP_STORE` | Linux Secret Service API wrapper via `secret-tool` / libsecret (`service=gemini`, `username=antigravity`) | 5 tests | 5 tests | Covered | Scenario 1, 9, 10 |
| **F02** | `F02_ATOMIC_KEYRING_SWITCH` | Atomic rotation of credentials in Secret Service to the next healthy account | 5 tests | 5 tests | Covered | Scenario 1, 7, 9 |
| **F03** | `F03_SESSION_PRESERVATION` | Preservation of `cascadeId` and open pane layouts in `app_storage.json` | 5 tests | 5 tests | Covered | Scenario 1, 4, 9 |
| **F04** | `F04_PROCESS_LIFECYCLE_MGR` | Graceful `SIGTERM` termination, exit polling, lock cleanup, and detached relaunch | 5 tests | 5 tests | Covered | Scenario 1, 2, 9 |
| **F05** | `F05_SQLITE_INTEGRITY` | Synchronize and protect SQLite WAL databases (`state.vscdb`, `conversation_summaries.db`) | 5 tests | 5 tests | Covered | Scenario 1, 2 |
| **F06** | `F06_QUOTA_SUMMARY_POLLER` | Polling `retrieveUserQuotaSummary`, parsing 5h and weekly buckets, remaining fractions | 5 tests | 5 tests | Covered | Scenario 1, 3, 7, 8 |
| **F07** | `F07_MODEL_CATALOG_FETCHER` | Polling `fetchAvailableModels`, parsing model catalogs, tiered allocations | 5 tests | 5 tests | Covered | Scenario 1, 8 |
| **F08** | `F08_RESET_HORIZON_WARMUP` | Automated 1-token keep-alive warmup engine (`POST /v1internal:generateContent`), drift and jitter | 5 tests | 5 tests | Covered | Scenario 3 |
| **F09** | `F09_AUTO_SWITCH_RULE_ENGINE` | Threshold evaluator comparing active quota against limits (e.g. 5%) and triggering switch | 5 tests | 5 tests | Covered | Scenario 1, 7 |
| **F10** | `F10_DEVICE_FINGERPRINT_ISOLATION`| Per-account isolation of `machineid`, `.updaterId`, `installation_id`, `installation_uuid` | 5 tests | 5 tests | Covered | Scenario 6 |
| **F11** | `F11_PROFILE_SWAPPER` | Atomic swapping of virtual device fingerprint profiles concurrently with keyring rotation | 5 tests | 5 tests | Covered | Scenario 1, 6, 9 |
| **F12** | `F12_BRAIN_CACHE_INSPECTOR` | Deep disk usage scanning and categorization of `brain/` and `conversations/` | 5 tests | 5 tests | Covered | Scenario 4 |
| **F13** | `F13_BRAIN_CACHE_PRUNER` | Safe cleanup routines for stale tasks and scratch directories preserving active sessions | 5 tests | 5 tests | Covered | Scenario 4 |
| **F14** | `F14_PROMPT_CACHE_OPTIMIZER` | Context cache analyzer identifying redundant prompts and token overhead | 5 tests | 5 tests | Covered | Scenario 4 |
| **F15** | `F15_GEMINI_M3_THEME` | Google Gemini Material Design 3 dark theme tokens (#131314, #1e1f20, #8ab4f8, 16px radius) | 5 tests | 5 tests | Covered | Scenario 8, 11 |
| **F16** | `F16_FIXED_LEFT_NAV_RAIL` | Fixed 72px navigation rail switching between Account Switcher, Marketplace, Settings | 5 tests | 5 tests | Covered | Scenario 8, 11 |
| **F17** | `F17_ACCOUNT_SWITCHER_TOP_RIBBON`| 5-tab top navigation ribbon (Quota, Vault, Fingerprints, Brain Cache, Settings) | 5 tests | 5 tests | Covered | Scenario 8, 11 |
| **F18** | `F18_QUOTA_DASHBOARD_VIEW` | Real-time circular vector gauges for Gemini 3.8 Flash, Flash Lite, Pro, and Claude | 5 tests | 5 tests | Covered | Scenario 8, 9 |
| **F19** | `F19_ACCOUNTS_MFA_VAULT_VIEW` | Multi-account inventory, credential management, backup codes view | 5 tests | 5 tests | Covered | Scenario 5 |
| **F20** | `F20_RFC6238_TOTP_ENGINE` | Pure Python RFC 6238 TOTP computation (HMAC-SHA1, 30s step) and countdown ring | 5 tests | 5 tests | Covered | Scenario 5 |
| **F21** | `F21_DEVICE_FINGERPRINTS_VIEW`| Interactive inspector and generator for virtualized hardware profiles | 5 tests | 5 tests | Covered | Scenario 6 |
| **F22** | `F22_BRAIN_CACHE_VIEW` | Visual disk breakdown chart and cleanup action triggers | 5 tests | 5 tests | Covered | Scenario 4 |
| **F23** | `F23_SWITCHER_SETTINGS_VIEW` | Sliders for threshold percentages, polling interval inputs, keep-alive toggle | 5 tests | 5 tests | Covered | Scenario 1, 3 |
| **F24** | `F24_SYSTEM_TRAY_INTEGRATION` | DBus StatusNotifierItem (SNI) integration via `QSystemTrayIcon` with status badge | 5 tests | 5 tests | Covered | Scenario 13 |
| **F25** | `F25_DAEMON_IPC_CORE` | Unix Domain Socket JSON-RPC 2.0 / NDJSON server, pub-sub notifications, in-process fallback | 5 tests | 5 tests | Covered | Scenario 8, 11 |
| **F26** | `F26_OFFLINE_MOCK_HARNESS` | Hermetic offline mock server simulating Google CloudCode responses and control plane | 5 tests | 5 tests | Covered | Scenario 1-13 |

---

## 3. Test Architecture & Directory Layout

### 3.1 Runner Command
The test suite is driven using standard pytest:
```bash
pytest tests/e2e -v
```
To run specific test tiers:
```bash
pytest tests/e2e/test_tier1_features.py -v     # Tier 1: Happy paths (130 tests)
pytest tests/e2e/test_tier2_boundaries.py -v   # Tier 2: Boundary & error cases (130 tests)
pytest tests/e2e/test_tier3_pairwise.py -v     # Tier 3: Combinatorial interactions (26 tests)
pytest tests/e2e/test_tier4_scenarios.py -v    # Tier 4: Real-world scenarios (13 scenarios)
```

### 3.2 Directory Layout
```
tests/
├── conftest.py                       # Global test hooks and pytest configuration
├── fixtures/
│   ├── __init__.py
│   ├── mock_keyring.py               # Hermetic Secret Service / secret-tool CLI & D-Bus mock
│   ├── mock_antigravity_fs.py        # Isolated ~/.config/Antigravity & ~/.gemini/antigravity fixture
│   ├── mock_cloudcode_server.py      # HTTP server simulating cloudcode-pa.googleapis.com
│   ├── mock_process.py               # Simulated Electron PID, SingletonLock, and process tree
│   └── test_helpers.py               # CLI runner wrappers, JSON-RPC socket client, sample payloads
└── e2e/
    ├── __init__.py
    ├── test_tier1_features.py        # Tier 1: Feature happy path verification (>= 130 tests)
    ├── test_tier2_boundaries.py      # Tier 2: Edge cases, corruptions, limits (>= 130 tests)
    ├── test_tier3_pairwise.py        # Tier 3: Pairwise combinatorial feature interactions (>= 26 tests)
    └── test_tier4_scenarios.py       # Tier 4: End-to-end real-world user scenarios (>= 13 tests)
```

---

## 4. Real-World Application Scenarios (Tier 4)

Tier 4 features 13 end-to-end scenarios executing complex user journeys across multiple subsystems:

1. **`SCENARIO_01_FULL_MULTI_ACCOUNT_ROTATION_LIFECYCLE`**:
   - Active account `user-alpha@gmail.com` quota drops from 12% to 3% during an intensive coding task.
   - Threshold rule engine detects `quota <= 5%` trigger.
   - Daemon selects standby `user-beta@gmail.com` with 95% quota.
   - Active `cascadeId` (`cascade-7788-99aa`) and split layout are preserved in `app_storage.json`.
   - Running Antigravity instance (`PID 4001`) receives `SIGTERM` and exits cleanly within timeout.
   - `SingletonLock` is verified removed; SQLite WAL files (`state.vscdb-wal`) checkpointed.
   - Linux Secret Service updated with `user-beta` OAuth token (`service=gemini`, `username=antigravity`).
   - Virtual device profile for `user-beta` swapped (`machineid`, `.updaterId`, `installation_id`, `antigravity_state.pbtxt`).
   - Antigravity relaunches in detached session; UI reconnects with restored conversation.

2. **`SCENARIO_02_APP_CRASH_RECOVERY_AND_STALE_LOCKS`**:
   - Simulated power cut or abnormal kill leaves `SingletonLock` symlink pointing to dead PID 99999 and dirty SQLite WAL files.
   - Swiss Knife daemon detects dead PID from `SingletonLock`.
   - Cleans stale `SingletonLock`, `SingletonSocket`, and `SingletonCookie`.
   - Performs SQLite integrity check and WAL checkpoint on `state.vscdb`.
   - Relaunches Antigravity successfully without `SingleInstanceLock` abort or "corrupted state.vscdb" popup.

3. **`SCENARIO_03_RESET_HORIZON_AUTONOMOUS_WARMUP`**:
   - Account hits 0% quota; Google sets `resetTime` to `T0 + 5 hours`.
   - Swiss Knife poller enters horizon watch mode.
   - Simulated time reaches `resetTime`.
   - Engine inspects HTTP `Date` header from upstream to correct local clock drift.
   - Injects randomized jitter (+350ms) to avoid gateway burst throttling.
   - Dispatches 1-token keep-alive prompt (`POST /v1internal:generateContent`, `maxOutputTokens: 1`).
   - Receives HTTP 200 OK; quota resets to 100% and next 5-hour rolling window initializes autonomously.

4. **`SCENARIO_04_BRAIN_CACHE_PRUNING_ACTIVE_SESSION_PROTECTION`**:
   - `~/.gemini/antigravity/brain/` reaches 3.8GB across 269 tasks.
   - User triggers cache cleanup for tasks older than 7 days.
   - Pruner cross-checks `app_storage.json` active `cascadeId` and `conversation_summaries.db`.
   - Safely removes 917MB of old scratchpad files and stale tool logs.
   - Verifies active conversation's files (`brain/<active_cascadeId>/`) and SQLite databases remain untouched and 100% accessible.

5. **`SCENARIO_05_RFC6238_MFA_VAULT_LIFECYCLE`**:
   - User registers a secondary account with Base32 TOTP secret `GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ`.
   - Engine strips whitespace, normalizes padding, and generates verified 6-digit TOTP code.
   - Visual countdown ring calculates remaining seconds and smooth angular fraction.
   - Code verified against RFC 6238 test vectors with drift tolerance $\pm 1$ step.
   - Backup codes are loaded into vault, decrypted, and marked as used upon redemption.

6. **`SCENARIO_06_PER_ACCOUNT_DEVICE_FINGERPRINT_ISOLATION`**:
   - Configures distinct virtual hardware profiles for Accounts A and B.
   - Ensures each profile contains valid 36-byte UUIDv4 strings with zero trailing newlines.
   - Swapping profile replaces `machineid`, `.updaterId`, `installation_id`, and `installation_uuid`.
   - Preserves `antigravity_state.pbtxt` onboarding completion flags (`POST_ONBOARDING_STEP_TYPE_...`) so Antigravity does not trigger first-run onboarding tutorial.

7. **`SCENARIO_07_DUAL_WINDOW_QUOTA_EXHAUSTION_HANDLING`**:
   - Account has 100% remaining on 5-hour window, but 0.0% remaining on 7-day weekly tier limit.
   - Poller parses dual-bucket hierarchy (`gemini-5h` vs `gemini-weekly`).
   - Rule engine detects weekly saturation; recognizes that 5-hour reset cannot restore service.
   - Marks account as globally exhausted and skips useless keep-alive warmup.
   - Automatically switches to alternate account with valid weekly quota.

8. **`SCENARIO_08_DAEMON_IPC_CLIENT_SYNC_AND_STREAMING`**:
   - Headless background daemon starts on Unix Domain Socket.
   - PySide6 GUI client connects to `$XDG_RUNTIME_DIR/antigravity-swiss/daemon.sock`.
   - GUI calls `status.get` and receives initial accounts and quota state.
   - Daemon polls upstream API and broadcasts `notify.quota_updated` event.
   - GUI receives push event over UDS and updates circular gauges and model tables in real-time.

9. **`SCENARIO_09_MANUAL_1CLICK_ACCOUNT_SWITCH`**:
   - User clicks `[Manual Switch]` on the Quota Dashboard to switch to `user-gamma@gmail.com`.
   - GUI sends JSON-RPC `account.switch("user-gamma@gmail.com")`.
   - Daemon coordinates graceful shutdown, credential rotation, fingerprint swapping, and application relaunch.
   - GUI receives completion response and updates active account badge and gauge values.

10. **`SCENARIO_10_OAUTH_TOKEN_EXPIRATION_AND_AUTO_REFRESH`**:
    - Poller encounters HTTP 401 `UNAUTHENTICATED` during routine quota retrieval.
    - Captures auth error and extracts `refresh_token` from stored keyring secret.
    - Calls Google OAuth token endpoint (`https://oauth2.googleapis.com/token`) with client ID and secret.
    - Receives new `access_token` and updated expiry timestamp.
    - Stores fresh token back to Linux Secret Service (`secret-tool store`).
    - Retries quota request and succeeds without user intervention.

11. **`SCENARIO_11_STANDALONE_IN_PROCESS_CONTROLLER_FALLBACK`**:
    - User executes GUI with `--standalone` flag or in an environment where the daemon socket is absent.
    - Application initializes `SwissKnifeController` in-process on a secondary worker thread.
    - Verifies all GUI sub-pages (Dashboard, Vault, Fingerprints, Brain Cache, Settings) function identically to client-server mode.

12. **`SCENARIO_12_TRANSIENT_GATEWAY_ERROR_BACKOFF`**:
    - Upstream CloudCode API returns transient HTTP 503 `UNAVAILABLE` followed by HTTP 429 `RESOURCE_EXHAUSTED`.
    - Engine detects transient status codes and enters truncated exponential backoff loop.
    - Jitter is applied to each backoff step.
    - Poller successfully completes request on subsequent retry without crashing or flooding upstream APIs.

13. **`SCENARIO_13_SYSTEM_TRAY_NOTIFICATION_AND_MINIMIZE`**:
    - Quota drops below 15% warning threshold.
    - System tray icon updates status badge to warning yellow `#fdd663`.
    - Triggers native desktop notification via DBus `org.freedesktop.Notifications`.
    - Window close event minimizes to tray instead of terminating application when configured in Switcher Settings.

---

## 5. Coverage Thresholds & Target Counts

| Tier | Category | Minimum Required | Planned Count |
|---|---|:---:|:---:|
| **Tier 1** | Feature Happy Path Verification (>= 5 tests per F01-F26) | 130 | 130 |
| **Tier 2** | Boundary, Error & Corner Cases (>= 5 tests per F01-F26) | 130 | 130 |
| **Tier 3** | Combinatorial Pairwise Feature Interactions | 26 | 26 |
| **Tier 4** | Real-World Application User Scenarios | 13 | 13 |
| **Total** | **Full Opaque-Box Test Suite** | **299** | **299** |

---

## 6. Execution & Verification

Run the full suite using pytest:
```bash
# Full test suite execution
pytest tests/e2e -v

# Verification with coverage reporting
pytest tests/e2e -v --tb=short
```
All tests run hermetically within isolated mock environments, requiring zero live network connectivity and zero modification of system credentials or user files.
