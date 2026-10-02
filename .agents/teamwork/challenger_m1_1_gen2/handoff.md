# M1 Concurrency & Keyring Adversarial Challenge Report

**Agent**: `challenger_m1_1_gen2` (Empirical Challenger)  
**Date**: 2026-10-01T08:27:00Z  
**Parent Agent**: `parent` (`11f1f26d-e61c-4e23-9c94-5ec9e98e06dd`)  
**Scope**: Milestone 1 Keyring Switcher & Account Vault Adversarial Challenge  
**Target Modules**: `antigravity_swiss/keyring/switcher.py`, `antigravity_swiss/keyring/secret_tool.py`, `antigravity_swiss/core/errors.py`  
**Verdict**: **REQUEST_CHANGES** (Blocking Critical Security & Data Integrity Flaws)

---

## 1. Observation

Adversarial stress harness `tests/stress/test_m1_concurrency_stress.py` was constructed and executed against the Milestone 1 codebase. Across 7 empirical stress scenarios, **5 scenarios failed** with critical data corruption and security failures, while 2 passed.

### 1.1 Test Suite Execution Output
Command executed:
```bash
pytest tests/stress/test_m1_concurrency_stress.py -v
```
Verbatim console output:
```text
============================= test session starts ==============================
platform linux -- Python 3.14.4, pytest-9.0.2, pluggy-1.6.0 -- /usr/bin/python3
cachedir: .pytest_cache
rootdir: /mnt/Data/Projects/Antigravity Swiss Knife
plugins: typeguard-4.4.4
collected 7 items

tests/stress/test_m1_concurrency_stress.py::test_adversarial_multiprocess_lost_updates FAILED [ 14%]
tests/stress/test_m1_concurrency_stress.py::test_adversarial_multithread_lost_updates FAILED [ 28%]
tests/stress/test_m1_concurrency_stress.py::test_adversarial_credential_cross_contamination FAILED [ 42%]
tests/stress/test_m1_concurrency_stress.py::test_adversarial_concurrent_switch_and_read PASSED [ 57%]
tests/stress/test_m1_concurrency_stress.py::test_adversarial_malformed_accounts_json FAILED [ 71%]
tests/stress/test_m1_concurrency_stress.py::test_adversarial_payload_and_newline_injections FAILED [ 85%]
tests/stress/test_m1_concurrency_stress.py::test_adversarial_rapid_rotation_races PASSED [100%]

=================================== FAILURES ===================================
__________________ test_adversarial_multiprocess_lost_updates __________________
    def test_adversarial_multiprocess_lost_updates():
        res = run_test_1_1_multiprocess_lost_updates(num_workers=4, accounts_per_worker=15)
>       assert res["pass"], f"Lost {res['lost']} accounts in multi-process additions!"
E       AssertionError: Lost 39 accounts in multi-process additions!
E       assert False
----------------------------- Captured stdout call -----------------------------
--- [TEST 1.1] Multi-Process Concurrent Account Addition (4 procs, 15 accs each) ---
Elapsed: 0.15s | Expected: 60 | Actual in vault: 21 | Lost: 39
[FAIL] Lost 39 accounts due to lack of transaction locking across load/save!

__________________ test_adversarial_multithread_lost_updates ___________________
    def test_adversarial_multithread_lost_updates():
        res = run_test_1_2_multithread_lost_updates(num_threads=4, accounts_per_thread=15)
>       assert res["pass"], f"Lost {res['lost']} accounts in multi-threaded additions!"
E       AssertionError: Lost 41 accounts in multi-threaded additions!
E       assert False
----------------------------- Captured stdout call -----------------------------
--- [TEST 1.2] Multi-Thread Concurrent Account Addition (4 threads, 15 accs each) ---
Elapsed: 0.01s | Expected: 60 | Actual in vault: 19 | Lost: 41 | Thread errors: 0
[FAIL] Multi-threading lost 41 accounts / 0 errors!

_______________ test_adversarial_credential_cross_contamination ________________
    def test_adversarial_credential_cross_contamination():
        res = run_test_1_3_credential_cross_contamination(num_switchers=3, iterations=15)
>       assert res["pass"], f"Detected credential cross-contamination in {res['cross_contaminated']} accounts!"
E       AssertionError: Detected credential cross-contamination in 5 accounts!
E       assert False
----------------------------- Captured stdout call -----------------------------
--- [TEST 1.3] Concurrent Switching & Credential Integrity (3 switchers, 15 iters) ---
Elapsed: 0.50s | Switches completed: 45 | Errors: 0
Cross-contaminated accounts: 5
[FAIL] CRITICAL BUG: Accounts contaminated with foreign credentials!
  - Account user_0@example.com: expected 'token_0_unique' but corrupted with 'token_3_unique'
  - Account user_1@example.com: expected 'token_1_unique' but corrupted with 'token_4_unique'
  - Account user_2@example.com: expected 'token_2_unique' but corrupted with 'token_4_unique'
  - Account user_3@example.com: expected 'token_3_unique' but corrupted with 'token_4_unique'
  - Account user_4@example.com: expected 'token_4_unique' but corrupted with 'token_3_unique'

___________________ test_adversarial_malformed_accounts_json ___________________
    def test_adversarial_malformed_accounts_json():
        res = run_test_2_malformed_corrupted_accounts_json()
>       assert res["pass"], f"AccountVault failed corrupted JSON recovery/handling!"
E       AssertionError: AccountVault failed corrupted JSON recovery/handling!
E       assert False
----------------------------- Captured stdout call -----------------------------
--- [TEST 2] Malformed & Corrupted accounts.json Recovery ---
Case 2.1 Truncated JSON detected properly: True
Case 2.1 Auto-recovery / repair capability: False (False indicates permanent lockup/unusable vault)
Case 2.2 Binary garbage handled: True
Case 2.3 Non-dict JSON root clean error: False (False = unhandled AttributeError)
Case 2.4 Corrupted record clean error: False (False = unhandled TypeError/AttributeError)

_______________ test_adversarial_payload_and_newline_injections ________________
    def test_adversarial_payload_and_newline_injections():
        res = run_test_3_payload_and_newline_injections()
>       assert res["pass"], f"SecretTool / Credential parser failed injection handling!"
E       AssertionError: SecretTool / Credential parser failed injection handling!
E       assert False
----------------------------- Captured stdout call -----------------------------
--- [TEST 3] Trailing Newline & Binary Payload Injections ---
Case 3.1 Non-dict payloads in from_antigravity_json: 5 unhandled exceptions out of 5
  - Payload '[1, 2, 3]' -> Unhandled AttributeError: 'list' object has no attribute 'get'
  - Payload '12345' -> Unhandled AttributeError: 'int' object has no attribute 'get'
  - Payload '"just_a_string"' -> Unhandled AttributeError: 'str' object has no attribute 'get'
  - Payload 'true' -> Unhandled AttributeError: 'bool' object has no attribute 'get'
  - Payload 'null' -> Unhandled AttributeError: 'NoneType' object has no attribute 'get'
Case 3.2 Non-UTF-8 binary output in lookup handled: False
  - Lookup crashed with: Unhandled UnicodeDecodeError: 'utf-8' codec can't decode byte 0x80 in position 0: invalid start byte
Case 3.3 Trailing newline stripped by SecretToolBackend.lookup(): False (Raw: 'my_secret_token\n')
Case 3.4 Null byte payload preserved: True

=========================== short test summary info ============================
FAILED tests/stress/test_m1_concurrency_stress.py::test_adversarial_multiprocess_lost_updates
FAILED tests/stress/test_m1_concurrency_stress.py::test_adversarial_multithread_lost_updates
FAILED tests/stress/test_m1_concurrency_stress.py::test_adversarial_credential_cross_contamination
FAILED tests/stress/test_m1_concurrency_stress.py::test_adversarial_malformed_accounts_json
FAILED tests/stress/test_m1_concurrency_stress.py::test_adversarial_payload_and_newline_injections
========================= 5 failed, 2 passed in 3.15s ==========================
```

In the maximum 8-worker / 200-account stress run (`python3 tests/stress/test_m1_concurrency_stress.py`), data loss was even more severe:
- Multi-Process Addition: 150 accounts lost out of 200 (75.0% data loss).
- Multi-Thread Addition: 161 accounts lost out of 200 (80.5% data loss).

### 1.2 Code Observations in Target Files
1. **`antigravity_swiss/keyring/switcher.py` Lines 233-275 & 303-337**:
   - `load()` acquires `fcntl.flock(lock_fd, fcntl.LOCK_EX)` and unlocks on return (Lines 234, 255).
   - `save()` acquires `fcntl.flock(lock_fd, fcntl.LOCK_EX)` and unlocks on return (Lines 259, 275).
   - `add_or_update_account()` executes `data = self.load()` on line 310, modifies `accounts[email]` in local memory, and calls `self.save(data)` on line 335.
   - The file lock is **released between load and save**, allowing interleaved concurrent writes.
   - `save()` uses `tmp_path = self.config_dir / f"{self.config_path.name}.tmp.{os.getpid()}"` (Line 261). All threads within a single process share `os.getpid()`, creating tmpfile write races.
2. **`antigravity_swiss/keyring/switcher.py` Lines 452-474 (`switch_account`)**:
   - Lines 457-469 attempt to read refreshed tokens from Secret Service before switching:
     ```python
     active_email = self.vault.get_active_account()
     if active_email and active_email != account_email:
         try:
             current_cred = self.get_active_credential()
             existing_record = self.vault.get_account(active_email)
             self.vault.add_or_update_account(
                 email=active_email,
                 credential=current_cred,
                 label=existing_record.label if existing_record else "",
             )
         except Exception as exc:
             ...
     self.set_active_credential(target_record.credential)
     self.vault.set_active_account(account_email)
     ```
   - No mutex or switch lock exists across the switch transaction.
   - The method performs **zero identity validation** on `current_cred` before saving it into `active_email`'s vault record. If another switch just ran, `current_cred` belongs to the *other* account, which is then written into `active_email`'s record, permanently corrupting the vault.
3. **`antigravity_swiss/keyring/switcher.py` Lines 233-255 & 89-108**:
   - `load()` assumes `data` is a `dict`. If `accounts.json` contains a JSON array `[1, 2, 3]` or non-dict, `list_accounts()` crashes with `AttributeError: 'list' object has no attribute 'get'`.
   - If `accounts.json` contains invalid syntax, `load()` raises `AccountVaultCorruptedError`. However, `add_or_update_account()` and `save()` call `load()`, which immediately raises `AccountVaultCorruptedError`. As a result, the application **cannot write a new account or overwrite the corrupted file**, causing permanent operational lockout.
   - `KeyringCredential.from_antigravity_json` (Line 97) calls `data.get("token", {})`. When passed non-dict JSON (e.g. `[1, 2, 3]`, `"string"`, `123`, `true`, `null`), it raises unhandled `AttributeError` instead of `InvalidCredentialError`.
4. **`antigravity_swiss/keyring/secret_tool.py` Lines 90 & 108-120**:
   - `lookup()` executes `return res.stdout.decode("utf-8")` without wrapping `UnicodeDecodeError` in `KeyringError`.
   - `store()` strips trailing `\r\n` (Line 116), but `lookup()` (Line 90) does not strip `\r\n`, breaking symmetry when secrets are populated by standard CLI utilities.

---

## 2. Logic Chain

1. **TOCTOU Read-Modify-Write Race in Account Vault (F02)**:
   - *Premise*: When two workers concurrently add accounts to `AccountVault`, each calls `load()` then `save()`.
   - *Mechanism*: Worker A calls `load()` (reads 0 accounts) and releases lock. Worker B calls `load()` (reads 0 accounts) and releases lock. Worker A saves `{Account A}`. Worker B saves `{Account B}`, overwriting Worker A's file.
   - *Impact*: 65% to 80% of accounts are lost. In our tests, 150 out of 200 accounts were permanently discarded within 80ms.
2. **Credential Cross-Contamination & Account Hijacking (F01, F02)**:
   - *Premise*: Multiple processes or asynchronous tasks trigger account switches (e.g. CLI switch while daemon evaluates quota threshold).
   - *Mechanism*: Switcher 1 switches from Account 0 to Account 4 and writes Account 4's token to Secret Service. Switcher 2 switches from Account 0 to Account 1. Switcher 2 inspects `active_email` (still Account 0), reads Secret Service (now containing Account 4's token), and calls `vault.add_or_update_account(email=Account 0, credential=Account 4 token)`.
   - *Impact*: Account 0 in the vault now permanently contains Account 4's OAuth credentials. 5 out of 5 accounts in our test suffered credential contamination. This is a severe security vulnerability.
3. **Permanent Lockout on Malformed `accounts.json` (F02)**:
   - *Premise*: A crash or malformed edit corrupts `accounts.json`.
   - *Mechanism*: Any attempt to read or repair the vault calls `load()`, which raises `AccountVaultCorruptedError`. Because `add_or_update_account` also calls `load()`, the system cannot overwrite or repair the vault.
   - *Impact*: The entire Swiss Knife daemon and CLI become permanently non-functional until a human manually deletes or repairs the file.
4. **Unhandled Runtime Exceptions on Non-Standard Payloads (F01)**:
   - *Premise*: Secret Service or payload input contains non-dict JSON or raw binary bytes.
   - *Mechanism*: `from_antigravity_json` calls `.get()` directly on parsed JSON without checking `isinstance(data, dict)`, raising raw `AttributeError`. `lookup()` decodes UTF-8 without exception handling, raising raw `UnicodeDecodeError`.
   - *Impact*: Bypasses domain exception mapping and crashes caller threads/IPC handlers.

---

## 3. Caveats

1. **Sequential Single-Thread Workloads**: In purely sequential single-process executions with well-formed data, the worker's unit tests pass (24/24). The failure modes emerge strictly under concurrent multi-process/multi-thread access, rapid rotation, and malformed inputs.
2. **Mock Secret-Tool vs Live DBus**: Concurrency stress tests utilized `create_mock_secret_tool_script` to prevent polluting the host user's personal GNOME Keyring. However, host `/usr/bin/secret-tool` was independently verified to confirm that native Linux `secret-tool` preserves binary null bytes and returns exit code 1 on missing attributes.
3. **Process Lifecycle Scope**: `ProcessLifecycleManager` and `AppStorageManager` were not modified during this challenge and passed their boundary checks.

---

## 4. Conclusion & Required Mitigations

### 4.1 Verdict: **REQUEST_CHANGES**
Milestone 1 **cannot be approved** in its current state due to critical race conditions, lost updates, and credential cross-contamination in `antigravity_swiss.keyring`.

### 4.2 Concrete Action Items for Implementer

#### Action Item 1: Transaction-Scoped Lock in `AccountVault`
Replace separate per-operation locks with a re-entrant transaction context manager:
```python
@contextlib.contextmanager
def transaction(self):
    self._ensure_dir()
    lock_fd = os.open(str(self.lock_path), os.O_CREAT | os.O_RDWR, 0o600)
    fcntl.flock(lock_fd, fcntl.LOCK_EX)
    try:
        data = self._load_unlocked()
        yield data
        self._save_unlocked(data)
    finally:
        fcntl.flock(lock_fd, fcntl.LOCK_UN)
        os.close(lock_fd)
```
Update `add_or_update_account`, `set_active_account`, and `remove_account` to perform modifications strictly inside `with self.transaction() as data:`.
In `_save_unlocked`, ensure temporary files use unique names per thread and process (e.g. `f"{self.config_path.name}.tmp.{os.getpid()}_{threading.get_ident()}"`).

#### Action Item 2: Switch Transaction Mutex & Credential Identity Verification
In `KeyringService.switch_account`:
1. Acquire an exclusive lock across the entire switch flow so that multiple switch operations cannot interleave.
2. When reading `current_cred = self.get_active_credential()` before switching, verify identity:
   ```python
   jwt_email = current_cred.extract_email_from_id_token()
   if jwt_email and jwt_email != active_email:
       logger.warning("Keyring token belongs to %s, not %s; skipping back-sync.", jwt_email, active_email)
   else:
       # Safe to update vault record
   ```

#### Action Item 3: Corrupted File Quarantine & Recovery
In `AccountVault.load`:
1. If JSON is invalid or root is not a dict (`not isinstance(data, dict)`), automatically quarantine the bad file:
   `shutil.copy(self.config_path, f"{self.config_path}.corrupted.{int(time.time())}")`
2. Re-initialize a clean structure `{"version": 1, "active_account": None, "accounts": {}}` or restore from `.bak` backup.
3. Validate each record in `list_account_records()`: skip or log records that are not valid dicts.

#### Action Item 4: Sanitization in `SecretToolBackend` and `KeyringCredential`
1. In `KeyringCredential.from_antigravity_json`:
   ```python
   if not isinstance(data, dict):
       raise InvalidCredentialError(f"Expected JSON object, got {type(data).__name__}")
   ```
2. In `SecretToolBackend.lookup`:
   ```python
   try:
       val = res.stdout.decode("utf-8")
       return val.rstrip("\r\n") if val else ""
   except UnicodeDecodeError as exc:
       raise KeyringError(f"Secret Service returned non-UTF8 binary data: {exc}") from exc
   ```

---

## 5. Verification Method

To independently reproduce the failures and verify fixes, execute:

### 5.1 Run the Full Adversarial Stress Suite via PyTest
```bash
pytest tests/stress/test_m1_concurrency_stress.py -v
```
**Current Result**: 5 FAILED, 2 PASSED.  
**Passing Condition**: All 7 tests PASS.

### 5.2 Run Standalone Runner with Metrics Output
```bash
python3 tests/stress/test_m1_concurrency_stress.py
```
**Current Result**: 2/7 passed (28.6%).  
**Passing Condition**: 7/7 passed (100.0%) with 0 lost accounts and 0 cross-contaminations.

### 5.3 Verify Baseline Unit Tests Remain Green
```bash
pytest tests/unit -v
```
Expected result: 24 passed in ~6s.
