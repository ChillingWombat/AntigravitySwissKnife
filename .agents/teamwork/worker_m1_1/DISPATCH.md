## 2026-10-01T07:56:19Z
You are the M1 Core & Switcher Implementer for Antigravity Swiss Knife.

Read the authoritative requirements at:
/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/ORIGINAL_REQUEST.md
and the project architecture at:
/mnt/Data/Projects/Antigravity Swiss Knife/PROJECT.md

Read the explorer handoffs and blueprints:
- /mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_m1_1/handoff.md (and prototype blueprints in explorer_m1_1/)
- /mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_m1_2/handoff.md
- /mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_m1_3/handoff.md

Your working directory is:
/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/worker_m1_1

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A teamwork_preview_auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

Scope & Exclusive File Ownership:
Implement Milestone 1 (Features F01, F02, F03, F04, F05, F25) adhering to the contracts in PROJECT.md:
1. `antigravity_swiss/__init__.py`
2. `antigravity_swiss/__main__.py` (CLI entry point: daemon, status, switch, gui commands)
3. `antigravity_swiss/core/`:
   - `config.py`: XDG path resolution, overrides, safe unix socket length (<108 bytes)
   - `constants.py`: Material 3 colors, schema names, default timeouts
   - `errors.py`: Domain exception hierarchy with .to_rpc_error() mapping
4. `antigravity_swiss/keyring/`:
   - `secret_tool.py`: Subprocess wrapper around /usr/bin/secret-tool (lookup, store without trailing \n, clear, idempotent)
   - `dbus_keyring.py`: D-Bus secret service fallback without application attribute pollution
   - `switcher.py`: Multi-account vault in accounts.json (mode 0600), fcntl.flock concurrency, atomic credential rotation
5. `antigravity_swiss/session/`:
   - `app_storage.py`: Safe parsing & atomic file replacement for ~/.config/Antigravity/app_storage.json, cascadeId layout preservation, aux-pane-session retention, jetski.onboarding.lastLoginUsername sync
   - `sqlite_guard.py`: SQLite WAL TRUNCATE checkpoint runner and quick_check verification
6. `antigravity_swiss/process/`:
   - `lock_manager.py`: SingletonLock target parsing (<hostname>-<PID>), /proc cmdline validation, stale lock cleaner
   - `lifecycle.py`: ProcessManager implementation (PID detection, graceful SIGTERM with 10s poll, SIGKILL fallback, detached relaunch via Popen)
7. `antigravity_swiss/ipc/`:
   - `socket_server.py`: Asyncio Unix Domain Socket server ($XDG_RUNTIME_DIR/antigravity-swiss/daemon.sock, mode 0600), JSON-RPC 2.0 / NDJSON router, dead socket detection, multi-client pub-sub event broadcaster
   - `socket_client.py`: AsyncDaemonClient and SyncDaemonClient
   - `controller.py`: SwissKnifeController with transparent fallback to in-process standalone controller
8. Unit Tests in `tests/unit/`:
   - `test_core.py`, `test_keyring.py`, `test_session.py`, `test_process.py`, `test_ipc.py`

Required Execution & Verification:
- You must run all unit tests using pytest (`pytest tests/unit -v`).
- Document all executed commands, test results, and file layout compliance in your handoff report.
- Deliver structured handoff report to:
  /mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/worker_m1_1/handoff.md
Follow Handoff Protocol. Notify parent (11f1f26d-e61c-4e23-9c94-5ec9e98e06dd) via send_message when complete.
