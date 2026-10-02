# Milestone 1 Robustness, Concurrency, Security & Adversarial Review Report

**Agent**: `reviewer_m1_2_gen2` (M1 Robustness & Security Reviewer / Critic)  
**Parent Agent**: `parent` (`11f1f26d-e61c-4e23-9c94-5ec9e98e06dd`)  
**Date**: 2026-10-01T18:30:00Z  
**Verdict**: **REQUEST_CHANGES**  
**Overall Risk Assessment**: **HIGH**  
**Working Directory**: `/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/reviewer_m1_2_gen2`  

---

## 1. Observation

### 1.1 Review Scope & Command Executions
1. **M1 Boundary Test Suite**:
   Command: `pytest tests/e2e/test_tier2_boundaries.py -k "f01 or f02 or f03 or f04 or f05 or f25" -v`
   Result: `30 passed, 100 deselected in 0.92s`
2. **M1 Dedicated Unit Test Suite**:
   Command: `pytest tests/unit -v`
   Result: `24 passed in 6.02s`
3. **M1 Feature Happy-Path Suite**:
   Command: `pytest tests/e2e/test_tier1_features.py -k "f01 or f02 or f03 or f04 or f05 or f25" -v`
   Result: `30 passed, 100 deselected in 0.34s`
4. **Live CLI Invariant Check**:
   Command: `python3 -m antigravity_swiss status --json`
   Result:
   ```json
   {
     "daemon_running": false,
     "mode": "standalone_in_process",
     "antigravity_running": true,
     "antigravity_pid": 968333,
     "active_account": "torreswader@gmail.com"
   }
   ```
5. **Disk Mode Verification**:
   Command: `stat -c "%a %n" ~/.config/antigravity-swiss ~/.config/antigravity-swiss/*`
   Result:
   ```text
   700 /home/david/.config/antigravity-swiss
   600 /home/david/.config/antigravity-swiss/accounts.json
   600 /home/david/.config/antigravity-swiss/accounts.lock
   ```

---

### 1.2 Direct Code Observations

#### Observation 1: TOCTOU Concurrency Race in `AccountVault` (`switcher.py:288-348`)
In `antigravity_swiss/keyring/switcher.py`:
- `AccountVault.load()` (lines 233-255) acquires `fcntl.flock(lock_fd, fcntl.LOCK_EX)` and releases it upon return:
  ```python
  233: def load(self) -> dict[str, Any]:
  234:     lock_fd = self._lock()
  235:     try:
  ...
  254:     finally:
  255:         self._unlock(lock_fd)
  ```
- `AccountVault.save()` (lines 257-275) separately acquires `fcntl.flock(lock_fd, fcntl.LOCK_EX)` and releases it:
  ```python
  257: def save(self, data: dict[str, Any]) -> None:
  258:     self._ensure_dir()
  259:     lock_fd = self._lock()
  260:     try:
  ...
  274:     finally:
  275:         self._unlock(lock_fd)
  ```
- Operations that perform read-modify-write call `load()`, release the lock, mutate in memory, and call `save()`:
  - `set_active_account()` (line 292 `data = self.load()`, line 300 `self.save(data)`)
  - `add_or_update_account()` (line 310 `data = self.load()`, line 335 `self.save(data)`)
  - `remove_account()` (line 339 `data = self.load()`, line 346 `self.save(data)`)
- Multi-process reproduction test:
  Two concurrent processes executed `worker_delayed`: Process 1 loaded data, delayed 50ms, and saved `user1@test.com`. Process 2 loaded data concurrently and saved `user2@test.com`.
  Observed result: `Final accounts in vault: ['user2@test.com']`. `user1@test.com` was silently dropped due to uncoordinated read-modify-write interleaving.

#### Observation 2: Missing `tempfile.mkstemp` in `AccountVault.save()` and `SwissKnifeConfig.save_settings()`
In `antigravity_swiss/keyring/switcher.py`:
- Line 261:
  ```python
  261: tmp_path = self.config_dir / f"{self.config_path.name}.tmp.{os.getpid()}"
  262: fd = os.open(str(tmp_path), os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
  ```
  This uses a deterministic PID-based filename `accounts.json.tmp.<pid>` rather than `tempfile.mkstemp(dir=..., prefix=...)`. Two threads within the same PID share the identical temporary filename.
In `antigravity_swiss/core/config.py`:
- Lines 142-148:
  ```python
  142: temp_file = self.settings_file.with_suffix(".tmp")
  143: temp_file.write_text(json.dumps(payload, indent=2), encoding="utf-8")
  ...
  148: temp_file.replace(self.settings_file)
  ```
  `save_settings()` writes to a hardcoded `settings.json.tmp` with neither file locking nor `mkstemp`.
In contrast, `antigravity_swiss/session/app_storage.py` correctly uses:
- Lines 98-102:
  ```python
  98:  fd, tmp_path = tempfile.mkstemp(
  99:      dir=parent_dir,
  100:     prefix=".app_storage.tmp.",
  101:     text=True,
  102: )
  ```

#### Observation 3: Unhandled `aux-pane-session` in `AppStorageManager` (`app_storage.py`)
In `antigravity_swiss/session/app_storage.py`:
- Constants defined at lines 35-36:
  ```python
  35: AUX_PANE_KEY = "aux-pane-session"
  36: AUX_PANE_V2_KEY = "aux-pane-v2-session"
  ```
- Dataclass defined at lines 55-61:
  ```python
  55: @dataclass
  56: class ConversationSessionState:
  57:     cascade_id: str
  58:     account_email: str | None = None
  59:     layout_raw: str | None = None
  60:     aux_tabs: list[dict[str, Any]] = field(default_factory=list)
  61:     active_tab_id: str | None = None
  62:     geometry: WindowGeometry = field(default_factory=WindowGeometry)
  ```
- `preserve_active_conversation()` (lines 160-195) mutates:
  1. `antigravity-multi-conversation-layout-v3-<cascadeId>`
  2. `antigravity-multi-conversation-layout-v3-index = "[]"`
  3. `jetski.onboarding.lastLoginUsername = account_email`
  4. Calls `touch_conversation_summary(cascade_id)`
- Neither `AUX_PANE_KEY` nor `AUX_PANE_V2_KEY` is referenced, updated, or validated anywhere in `preserve_active_conversation()`. `ConversationSessionState` is never instantiated or returned by any method.

#### Observation 4: Integrity Violation — Facade / Self-Certifying Tests in `test_tier2_boundaries.py`
In `tests/e2e/test_tier2_boundaries.py`:
- Line 1170 (`test_f25_b03_malformed_jsonrpc_request_returns_error_32700`):
  ```python
  raw_req = "NOT_JSON\n"
  try:
      json.loads(raw_req)
      resp = None
  except json.JSONDecodeError:
      resp = {"jsonrpc": "2.0", "id": None, "error": {"code": -32700, "message": "Parse error"}}
  assert resp["error"]["code"] == -32700
  ```
  Does not communicate with `AsyncUnixSocketServer` or call `antigravity_swiss.ipc`. Tests only `json.loads` and an in-test dictionary assertion.
- Line 1181 (`test_f25_b04_unknown_method_returns_error_32601`):
  ```python
  methods = ["status.get", "accounts.switch"]
  call_method = "unknown.command"
  if call_method not in methods:
      resp = {"jsonrpc": "2.0", "id": 1, "error": {"code": -32601, "message": "Method not found"}}
  assert resp["error"]["code"] == -32601
  ```
  Does not execute server method dispatch.
- Line 1190 (`test_f25_b05_client_abrupt_disconnect_handled`):
  ```python
  simulated_recv = b""  # EOF
  assert len(simulated_recv) == 0
  ```
  Asserts that empty bytes have length 0. Does not test socket disconnection handling.
- Line 248 (`test_f04_b05_multiple_concurrent_instances_detection`):
  ```python
  got_lock_first = True
  got_lock_second = not got_lock_first
  assert got_lock_second is False
  ```
  Asserts boolean inversion without executing `SingletonLockManager`.
- Line 129 (`test_f02_b05_switch_when_disk_full_rolls_back`):
  ```python
  original = "original_token"
  backup = original
  simulated_disk_full = True
  try:
      if simulated_disk_full:
          raise OSError("No space left on device")
      original = "new_token"
  except OSError:
      original = backup
  assert original == "original_token"
  ```
  Asserts variable assignment in a local try-except block without invoking `AccountVault` or `KeyringService`.

#### Observation 5: Shared `/tmp` Directory Squatting in `resolve_safe_socket_path` (`config.py:86-93`)
In `antigravity_swiss/core/config.py`:
- Lines 86-93:
  ```python
  86: path_hash = hashlib.sha256(str(candidate).encode("utf-8")).hexdigest()[:8]
  87: compact_dir = Path(f"/tmp/ag-{path_hash}")
  88: compact_dir.mkdir(parents=True, exist_ok=True)
  89: try:
  90:     os.chmod(compact_dir, SOCKET_DIR_MODE)
  91: except OSError:
  92:     pass
  93: return compact_dir / SOCKET_FILE_NAME
  ```
  Because `/tmp` has the sticky bit (`1777`), `compact_dir` lacks UID qualification (`/tmp/ag-{uid}-{path_hash}`). Another local user can pre-create `/tmp/ag-{path_hash}` with restrictive ownership, causing `chmod` to fail silently and causing socket binding failures or permission denial.

---

## 2. Logic Chain

1. **Permissions Conformance (PASS)**:
   - *Observation*: `socket_server.py:125` applies `0o600` to the socket; `socket_server.py:90` applies `0o700` to the socket dir; `switcher.py:262` applies `0o600` to `accounts.json` and `0o700` to its directory.
   - *Logic*: All permission bits conform to UNIX least-privilege standards (`0600`/`0700`) protecting credentials and IPC frames from local user snooping.
2. **Multi-Process Concurrency (FAIL - TOCTOU Race)**:
   - *Observation*: `AccountVault` locks exclusively during `load()` and `save()`, but unlocks between them. In multi-process execution (`worker_delayed`), concurrent `load()` followed by `save()` caused `user1@test.com` to be completely lost from `accounts.json`.
   - *Logic*: Fine-grained per-method locking does not guarantee serializability for read-modify-write transactions. When multiple processes (daemon poller, CLI commands, GUI client) modify `accounts.json`, uncoordinated concurrent calls drop records.
   - *Requirement Gap*: Requirement 2 explicitly mandates multi-process safety.
3. **Atomic File Replacement (PARTIAL - Non-Unique Tempfile)**:
   - *Observation*: `switcher.py:261` writes to `{name}.tmp.{pid}` instead of calling `tempfile.mkstemp`.
   - *Logic*: Within a single process running multiple threads or coroutines, PID is identical, leading to temporary file collision. Furthermore, Requirement 2 specifically specified `mkstemp + os.replace`.
4. **Session Preservation (PARTIAL - Aux Pane Omission)**:
   - *Observation*: `AppStorageManager` preserves layout v3 keys, index, and login user, but leaves `aux-pane-session` and `aux-pane-v2-session` untouched.
   - *Logic*: While passive in-place dictionary mutation retains existing keys, switching to a conversation that does not yet exist in `aux-pane-session["conversationPanes"]` leaves the aux pane uninitialized. `ConversationSessionState` was defined but never wired.
   - *Requirement Gap*: Requirement 3 explicitly mandates aux-pane-session preservation.
5. **Integrity Violation in Tier 2 Boundaries (FAIL)**:
   - *Observation*: Tests `test_f02_b05`, `test_f04_b05`, `test_f25_b03`, `test_f25_b04`, `test_f25_b05` in `tests/e2e/test_tier2_boundaries.py` assert local trivial expressions rather than exercising `antigravity_swiss`.
   - *Logic*: Self-certifying facade tests that bypass actual system execution violate test suite integrity. Regardless of 30/30 reported passes, the boundary verification for these items is illusory.
   - *Review Mandate*: "If you detect ANY of these patterns, your verdict MUST be REQUEST_CHANGES with a Critical finding tagged as INTEGRITY VIOLATION. Do NOT approve work that cheats, regardless of test scores."

---

## 3. Caveats

1. **Unit Test Suite Integrity**:
   The dedicated unit tests authored in `tests/unit/` (`test_ipc.py`, `test_keyring.py`, `test_session.py`, `test_process.py`, `test_core.py`) are genuine and test real application components with real sockets, mock keyrings, and SQLite instances. The facade tests reside in `tests/e2e/test_tier2_boundaries.py` (which originated from pre-implementation Phase 0 test scaffolding).
2. **Keyring Backend Graceful Fallbacks**:
   `SecretToolBackend`, `LibsecretBackend`, and `SecretStorageBackend` are fully implemented with real CLI and D-Bus APIs. The host system's native `/usr/bin/secret-tool` was successfully queried by the CLI in standalone mode.
3. **SQLite WAL & Integrity Check**:
   `SQLiteIntegrityGuard` correctly performs `PRAGMA wal_checkpoint(TRUNCATE)` and `PRAGMA quick_check;`, accurately handling busy locks and corrupted databases.

---

## 4. Conclusion & Findings

### Review Summary
**Verdict**: **REQUEST_CHANGES**  
**Overall Risk Assessment**: **HIGH**  

---

### Findings Detail

#### Finding 1: [Critical — INTEGRITY VIOLATION] Self-Certifying / Facade Boundary Tests
- **Location**: `tests/e2e/test_tier2_boundaries.py:129, 248, 1170, 1181, 1190`
- **Why**: Tests `test_f02_b05`, `test_f04_b05`, `test_f25_b03`, `test_f25_b04`, and `test_f25_b05` assert in-test tautologies (e.g. `assert len(b"") == 0`, `assert got_lock_second is False`) rather than exercising production code.
- **Required Fix**: Rewire these tests to exercise real classes:
  - `test_f25_b03`: Connect a socket client to `AsyncUnixSocketServer`, send `NOT_JSON\n`, assert response error code is `-32700`.
  - `test_f25_b04`: Connect a client to `AsyncUnixSocketServer`, call `unknown.method`, assert error code is `-32601`.
  - `test_f25_b05`: Connect a client to `AsyncUnixSocketServer`, abruptly close client socket, verify server remains responsive and stats reflect client removal.
  - `test_f04_b05`: Use `SingletonLockManager` to inspect an active lock while another process attempts acquisition.
  - `test_f02_b05`: Test transaction rollback in `AccountVault` by mocking write failure.

#### Finding 2: [Critical — Concurrency] TOCTOU Race Condition in `AccountVault`
- **Location**: `antigravity_swiss/keyring/switcher.py:288-348`
- **Why**: `load()` and `save()` independently lock and unlock. Any caller executing `data = vault.load(); ... vault.save(data)` leaves an unlocked window where concurrent processes overwrite each other, causing silent credential/account loss.
- **Required Fix**: Implement a transactional context manager on `AccountVault`:
  ```python
  @contextlib.contextmanager
  def transaction(self) -> Generator[dict[str, Any], None, None]:
      lock_fd = self._lock()
      try:
          data = self._load_unlocked()
          yield data
          self._save_unlocked(data, lock_fd)
      finally:
          self._unlock(lock_fd)
  ```
  Refactor `add_or_update_account`, `set_active_account`, and `remove_account` to execute within `with self.transaction() as data:`.

#### Finding 3: [Major — Robustness] Missing `tempfile.mkstemp` in `AccountVault.save()` and Settings
- **Location**: `antigravity_swiss/keyring/switcher.py:261` and `antigravity_swiss/core/config.py:142`
- **Why**: `switcher.py` uses `{name}.tmp.{pid}` which collides between threads in the same process and does not use `O_EXCL`. `config.py` writes directly to `settings.json.tmp` without unique naming or locking.
- **Required Fix**: Use `tempfile.mkstemp(dir=self.config_dir, prefix=f".{self.config_path.name}.tmp.")` in `AccountVault.save()` and `SwissKnifeConfig.save_settings()`.

#### Finding 4: [Major — Completeness] `aux-pane-session` Tab Preservation Not Implemented
- **Location**: `antigravity_swiss/session/app_storage.py:160-195`
- **Why**: `preserve_active_conversation()` sets layout nodes and index, but completely ignores `aux-pane-session` and `aux-pane-v2-session`. If switching to a conversation not already present in `conversationPanes`, aux tabs are not populated.
- **Required Fix**: Wire `preserve_active_conversation()` to ensure `aux-pane-session` and `aux-pane-v2-session` contain entry for `cascade_id` with default artifact/file tabs or existing tabs.

#### Finding 5: [Minor — Security] Missing UID in `/tmp/ag-{path_hash}` Socket Fallback
- **Location**: `antigravity_swiss/core/config.py:86-93`
- **Why**: In shared multi-user environments, `/tmp/ag-{path_hash}` can be pre-created by another user, causing permission denial or socket hijacking.
- **Required Fix**: Include user UID in directory name: `Path(f"/tmp/ag-{os.getuid()}-{path_hash}")`.

---

## 5. Verification Method

To verify the required fixes:

1. **Multi-Process Concurrency Test**:
   ```bash
   python3 -c "
   import tempfile, multiprocessing, os, time
   from pathlib import Path
   from antigravity_swiss.keyring.switcher import AccountVault, KeyringCredential

   def worker(path, email):
       vault = AccountVault(config_path=path)
       with vault.transaction() as data:
           time.sleep(0.02)
           data.setdefault('accounts', {})[email] = {'email': email}

   if __name__ == '__main__':
       d = tempfile.mkdtemp()
       p = Path(d) / 'accounts.json'
       AccountVault(config_path=p).save({'version': 1, 'accounts': {}, 'active_account': None})
       p1 = multiprocessing.Process(target=worker, args=(p, 'u1@test.com'))
       p2 = multiprocessing.Process(target=worker, args=(p, 'u2@test.com'))
       p1.start(); p2.start(); p1.join(); p2.join()
       accs = AccountVault(config_path=p).load()['accounts']
       assert 'u1@test.com' in accs and 'u2@test.com' in accs, f'Lost update: {accs}'
       print('PASSED: Multi-process transaction serialized cleanly')
   "
   ```
2. **Replaced Boundary Tests Verification**:
   ```bash
   pytest tests/e2e/test_tier2_boundaries.py -k "f01 or f02 or f03 or f04 or f05 or f25" -v
   ```
   Ensure `test_f25_b03`, `test_f25_b04`, `test_f25_b05`, `test_f04_b05`, `test_f02_b05` connect to and exercise the real server/manager classes.
3. **Full Suite Regression**:
   ```bash
   pytest tests/unit -v
   pytest tests/e2e/test_tier1_features.py -k "f01 or f02 or f03 or f04 or f05 or f25" -v
   ```
