## 2026-10-01T08:37:11Z
You are the M1 Remediation Worker (Iteration 2) for Antigravity Swiss Knife.

Your working directory is:
/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/worker_m1_2

Read these authoritative source documents and review handoffs before starting work:
- /mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/ORIGINAL_REQUEST.md
- /mnt/Data/Projects/Antigravity Swiss Knife/PROJECT.md
- /mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/orchestrator_1/GATE_STATUS.md
- /mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/reviewer_m1_1_gen2/handoff.md
- /mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/reviewer_m1_2_gen2/handoff.md
- /mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/challenger_m1_1_gen2/handoff.md
- /mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/challenger_m1_2_gen2/handoff.md

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A teamwork_preview_auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

Scope & Exclusive File Ownership:
You exclusively own and will modify:
1. `antigravity_swiss/keyring/switcher.py`
2. `antigravity_swiss/keyring/secret_tool.py`
3. `antigravity_swiss/core/config.py`
4. `antigravity_swiss/session/app_storage.py`
5. `antigravity_swiss/process/lifecycle.py`
6. `antigravity_swiss/process/lock_manager.py`
7. `antigravity_swiss/ipc/socket_client.py`
8. `antigravity_swiss/ipc/socket_server.py`
9. `tests/e2e/test_tier2_boundaries.py` (specifically tests `test_f02_b05`, `test_f04_b05`, `test_f25_b03`, `test_f25_b04`, `test_f25_b05`)

Specific Remediation Tasks:
1. `AccountVault` Transaction Locking & mkstemp (`antigravity_swiss/keyring/switcher.py`):
   - Implement `@contextlib.contextmanager def transaction(self) -> Generator[dict[str, Any], None, None]:` acquiring `fcntl.flock(lock_fd, fcntl.LOCK_EX)` across the entire read-modify-write cycle.
   - Refactor `add_or_update_account`, `set_active_account`, and `remove_account` to run inside `with self.transaction() as data:`.
   - In `save()`, use `tempfile.mkstemp(dir=self.config_dir, prefix=f".{self.config_path.name}.tmp.")` with `os.fchmod(fd, 0o600)` + `os.replace` to prevent thread collision and guarantee atomic inode replacement.
2. `KeyringService.switch_account` Concurrency & Identity Verification (`antigravity_swiss/keyring/switcher.py`):
   - Wrap switch execution in a cross-process lock (using `accounts.lock` / `vault.transaction()`).
   - Before writing current token back to vault, verify identity (extract email from token payload; if available and does not match active_email, log warning and skip vault back-sync to prevent cross-contamination).
3. Corrupted File Auto-Quarantine & Input Sanitization (`antigravity_swiss/keyring/switcher.py`, `secret_tool.py`):
   - In `AccountVault.load()`, if JSON is malformed or root is not a dict (`not isinstance(data, dict)`), auto-quarantine the corrupted file (`accounts.json.corrupted.<ts>`) and reinitialize a clean vault structure `{"version": 1, "active_account": None, "accounts": {}}`.
   - In `KeyringCredential.from_antigravity_json`, check `if not isinstance(data, dict): raise InvalidCredentialError(f"Expected JSON object, got {type(data).__name__}")`.
   - In `SecretToolBackend.lookup`, catch `UnicodeDecodeError` and wrap in `KeyringError`. Strip trailing `\r\n` symmetrically.
4. Settings Atomicity & Socket Fallback Security (`antigravity_swiss/core/config.py`):
   - In `save_settings()`, use `tempfile.mkstemp(dir=self.config_dir, prefix=".settings.tmp.")` with `os.fchmod(fd, 0o600)` + `os.replace`.
   - In `resolve_safe_socket_path()`, include `os.getuid()` in fallback dir: `Path(f"/tmp/ag-{os.getuid()}-{path_hash}")`.
5. Session Auxiliary Pane Retention (`antigravity_swiss/session/app_storage.py`):
   - In `preserve_active_conversation()`, ensure `aux-pane-session` and `aux-pane-v2-session` keys are populated and preserve active `cascadeId` pane entries.
6. Process Lifecycle Symlink & Zombie Handling (`antigravity_swiss/process/lifecycle.py`, `lock_manager.py`):
   - In `lifecycle.py:204` (`relaunch`), replace `if self.lock_manager.lock_file.exists():` with `if self.lock_manager.lock_file.is_symlink() or os.path.lexists(self.lock_manager.lock_file): break` so broken symlinks are detected immediately without a 5.0s stall.
   - In `lock_manager.py:83-94`, check `/proc/{pid}/status` for `State: Z (zombie)`. If zombie, set `is_alive = False` and `is_orphaned = True`. Handle in `lifecycle.py` poll loops.
7. IPC Client Buffer Limit, Server Broadcast Hang & Binary Noise (`antigravity_swiss/ipc/socket_client.py`, `socket_server.py`):
   - In `socket_client.py:63`, pass `limit=MAX_FRAME_SIZE` in `asyncio.open_unix_connection(str(self.socket_path), limit=MAX_FRAME_SIZE)`.
   - In `socket_server.py:173-180` (`broadcast_event`), wrap client drain in `asyncio.wait_for(writer.drain(), timeout=0.5)` or broadcast concurrently with `asyncio.gather` so slow/stalled clients don't hang the entire server. Prune disconnected/timed out clients.
   - In `socket_server.py:211`, catch `UnicodeDecodeError` and return a standard JSON-RPC 2.0 `-32700` ParseError frame.
8. E2E Boundary Tests Authenticity (`tests/e2e/test_tier2_boundaries.py`):
   - Rewire `test_f02_b05`, `test_f04_b05`, `test_f25_b03`, `test_f25_b04`, `test_f25_b05` to genuinely exercise the real classes, sockets, and managers instead of in-test tautologies.

Verification Commands to Execute:
1. `pytest tests/unit -v`
2. `pytest tests/stress/test_m1_concurrency_stress.py -v` (ALL 7 tests must pass 100%)
3. `pytest tests/e2e/test_tier1_features.py -k "f01 or f02 or f03 or f04 or f05 or f25" -v`
4. `pytest tests/e2e/test_tier2_boundaries.py -k "f01 or f02 or f03 or f04 or f05 or f25" -v`
5. `python3 -m antigravity_swiss status --json`
