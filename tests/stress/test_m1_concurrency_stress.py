"""
Adversarial Stress Test Suite for Milestone 1: Keyring Switcher & Account Vault.
Covers:
1. High-concurrency simultaneous account switching & reading across processes and threads.
2. Malformed / corrupted accounts.json recovery & resilience.
3. Trailing newline and binary payload injection into secret-tool and credential parser.
4. Race conditions during rapid account rotation and credential integrity verification.
"""

import base64
import concurrent.futures
import json
import os
import shutil
import subprocess
import sys
import tempfile
import threading
import time
from pathlib import Path

# Ensure project root is in sys.path
PROJECT_ROOT = Path(__file__).resolve().parents[2]
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

from antigravity_swiss.core.errors import (
    AccountNotFoundError,
    AccountVaultCorruptedError,
    InvalidCredentialError,
    KeyringError,
    KeyringNotFoundError,
)
from antigravity_swiss.keyring.secret_tool import SecretToolBackend
from antigravity_swiss.keyring.switcher import (
    AccountRecord,
    AccountStore,
    AccountVault,
    KeyringCredential,
    KeyringService,
    KeyringSwitcher,
)
from tests.fixtures.mock_keyring import create_mock_secret_tool_script


# ============================================================================
# Section 1: Multi-Process & Multi-Thread Concurrency Harnesses
# ============================================================================

def _worker_add_account_proc(vault_path: str, worker_id: int, num_accounts: int) -> dict:
    """Process worker adding accounts to AccountVault."""
    vault = AccountVault(config_path=vault_path)
    added = []
    errors = []
    for i in range(num_accounts):
        email = f"proc_w{worker_id}_acc_{i}@example.com"
        cred = KeyringCredential(access_token=f"token_{worker_id}_{i}", refresh_token=f"ref_{worker_id}_{i}")
        try:
            vault.add_or_update_account(email, cred, label=f"Worker {worker_id} Acc {i}")
            added.append(email)
        except Exception as exc:
            errors.append((email, type(exc).__name__, str(exc)))
    return {"worker_id": worker_id, "added": added, "errors": errors}


def _worker_add_account_thread(vault: AccountVault, thread_id: int, num_accounts: int, results_list: list) -> None:
    """Thread worker adding accounts to AccountVault."""
    added = []
    errors = []
    for i in range(num_accounts):
        email = f"thread_t{thread_id}_acc_{i}@example.com"
        cred = KeyringCredential(access_token=f"thread_token_{thread_id}_{i}", refresh_token=f"ref_{thread_id}_{i}")
        try:
            vault.add_or_update_account(email, cred, label=f"Thread {thread_id} Acc {i}")
            added.append(email)
        except Exception as exc:
            errors.append((email, type(exc).__name__, str(exc)))
    results_list.append({"thread_id": thread_id, "added": added, "errors": errors})


def _worker_concurrent_switch_proc(
    vault_path: str,
    secret_tool_path: str,
    account_emails: list[str],
    iterations: int,
    worker_id: int,
) -> dict:
    """Process worker performing rapid account switching."""
    vault = AccountVault(config_path=vault_path)
    backend = SecretToolBackend(binary_path=secret_tool_path)
    service = KeyringService(backend=backend, vault=vault)
    switches = 0
    errors = []

    for i in range(iterations):
        target = account_emails[(i + worker_id) % len(account_emails)]
        try:
            success = service.switch_account(target, reason=f"worker_{worker_id}")
            if success:
                switches += 1
        except Exception as exc:
            errors.append((target, type(exc).__name__, str(exc)))
        time.sleep(0.001)

    return {"worker_id": worker_id, "switches": switches, "errors": errors}


def _worker_poll_read_proc(vault_path: str, duration_sec: float) -> dict:
    """Process worker continuously reading vault state."""
    vault = AccountVault(config_path=vault_path)
    reads = 0
    errors = []
    end_time = time.time() + duration_sec
    while time.time() < end_time:
        try:
            accs = vault.list_accounts()
            active = vault.get_active_account()
            reads += 1
        except Exception as exc:
            errors.append((type(exc).__name__, str(exc)))
        time.sleep(0.002)
    return {"reads": reads, "errors": errors}


# ============================================================================
# Test Cases Execution Functions
# ============================================================================

def run_test_1_1_multiprocess_lost_updates(num_workers: int = 8, accounts_per_worker: int = 25) -> dict:
    """Test 1.1: Multi-process concurrent account additions (TOCTOU read-modify-write)."""
    print(f"\n--- [TEST 1.1] Multi-Process Concurrent Account Addition ({num_workers} procs, {accounts_per_worker} accs each) ---")
    temp_dir = tempfile.mkdtemp(prefix="stress_proc_add_")
    vault_path = os.path.join(temp_dir, "accounts.json")
    total_expected = num_workers * accounts_per_worker
    all_added_emails = set()

    start_time = time.time()
    with concurrent.futures.ProcessPoolExecutor(max_workers=num_workers) as executor:
        futures = [
            executor.submit(_worker_add_account_proc, vault_path, w_id, accounts_per_worker)
            for w_id in range(num_workers)
        ]
        results = [f.result() for f in concurrent.futures.as_completed(futures)]
    elapsed = time.time() - start_time

    for res in results:
        all_added_emails.update(res["added"])

    vault = AccountVault(config_path=vault_path)
    final_accounts = vault.list_accounts()
    lost_accounts = all_added_emails - set(final_accounts)

    passed = len(lost_accounts) == 0
    print(f"Elapsed: {elapsed:.2f}s | Expected: {total_expected} | Actual in vault: {len(final_accounts)} | Lost: {len(lost_accounts)}")
    if not passed:
        print(f"[FAIL] Lost {len(lost_accounts)} accounts due to lack of transaction locking across load/save!")
    else:
        print("[PASS] Zero lost accounts.")

    shutil.rmtree(temp_dir, ignore_errors=True)
    return {
        "name": "1.1 Multi-Process Concurrent Account Addition",
        "expected": total_expected,
        "actual": len(final_accounts),
        "lost": len(lost_accounts),
        "pass": passed,
    }


def run_test_1_2_multithread_lost_updates(num_threads: int = 8, accounts_per_thread: int = 25) -> dict:
    """Test 1.2: Multi-thread concurrent account additions within a single process."""
    print(f"\n--- [TEST 1.2] Multi-Thread Concurrent Account Addition ({num_threads} threads, {accounts_per_thread} accs each) ---")
    temp_dir = tempfile.mkdtemp(prefix="stress_thread_add_")
    vault_path = os.path.join(temp_dir, "accounts.json")
    vault = AccountVault(config_path=vault_path)
    total_expected = num_threads * accounts_per_thread

    threads = []
    results_list = []
    start_time = time.time()
    for t_id in range(num_threads):
        t = threading.Thread(target=_worker_add_account_thread, args=(vault, t_id, accounts_per_thread, results_list))
        threads.append(t)
        t.start()
    for t in threads:
        t.join()
    elapsed = time.time() - start_time

    all_added_emails = set()
    total_thread_errors = 0
    for res in results_list:
        all_added_emails.update(res["added"])
        total_thread_errors += len(res["errors"])

    final_accounts = vault.list_accounts()
    lost_accounts = all_added_emails - set(final_accounts)

    passed = len(lost_accounts) == 0 and total_thread_errors == 0
    print(f"Elapsed: {elapsed:.2f}s | Expected: {total_expected} | Actual in vault: {len(final_accounts)} | Lost: {len(lost_accounts)} | Thread errors: {total_thread_errors}")
    if not passed:
        print(f"[FAIL] Multi-threading lost {len(lost_accounts)} accounts / {total_thread_errors} errors!")
    else:
        print("[PASS] Zero lost accounts in multi-threading.")

    shutil.rmtree(temp_dir, ignore_errors=True)
    return {
        "name": "1.2 Multi-Thread Concurrent Account Addition",
        "expected": total_expected,
        "actual": len(final_accounts),
        "lost": len(lost_accounts),
        "thread_errors": total_thread_errors,
        "pass": passed,
    }


def run_test_1_3_credential_cross_contamination(num_switchers: int = 4, iterations: int = 25) -> dict:
    """Test 1.3: Concurrent account switching and credential cross-contamination detection."""
    print(f"\n--- [TEST 1.3] Concurrent Switching & Credential Integrity ({num_switchers} switchers, {iterations} iters) ---")
    temp_dir = tempfile.mkdtemp(prefix="stress_cross_contam_")
    vault_path = os.path.join(temp_dir, "accounts.json")
    bin_dir = os.path.join(temp_dir, "bin")
    keyring_state = os.path.join(temp_dir, "keyring.json")

    mock_script = create_mock_secret_tool_script(bin_dir, keyring_state)

    vault = AccountVault(config_path=vault_path)
    accounts = []
    for i in range(5):
        email = f"user_{i}@example.com"
        cred = KeyringCredential(
            access_token=f"token_{i}_unique",
            refresh_token=f"refresh_{i}_unique",
            id_token=f"ey.eyJlbWFpbCI6ICJ7ZW1haWx9In0=.sig",
        )
        vault.add_or_update_account(email, cred, label=f"User {i}")
        accounts.append(email)

    # Seed keyring with account 0
    backend = SecretToolBackend(binary_path=mock_script)
    backend.store(vault.get_account(accounts[0]).credential.to_antigravity_json())
    vault.set_active_account(accounts[0])

    start_time = time.time()
    with concurrent.futures.ProcessPoolExecutor(max_workers=num_switchers) as executor:
        futures = [
            executor.submit(_worker_concurrent_switch_proc, vault_path, mock_script, accounts, iterations, w_id)
            for w_id in range(num_switchers)
        ]
        results = [f.result() for f in concurrent.futures.as_completed(futures)]
    elapsed = time.time() - start_time

    total_switches = sum(r["switches"] for r in results)
    total_switch_errors = sum(len(r["errors"]) for r in results)

    # Check for credential cross-contamination in the vault
    cross_contaminated = []
    for i, email in enumerate(accounts):
        rec = vault.get_account(email)
        expected_token = f"token_{i}_unique"
        actual_token = rec.credential.access_token
        if actual_token != expected_token:
            cross_contaminated.append((email, expected_token, actual_token))

    passed = len(cross_contaminated) == 0 and total_switch_errors == 0
    print(f"Elapsed: {elapsed:.2f}s | Switches completed: {total_switches} | Errors: {total_switch_errors}")
    print(f"Cross-contaminated accounts: {len(cross_contaminated)}")
    if not passed:
        print("[FAIL] CRITICAL BUG: Accounts contaminated with foreign credentials!")
        for email, exp, act in cross_contaminated:
            print(f"  - Account {email}: expected '{exp}' but corrupted with '{act}'")
    else:
        print("[PASS] Zero credential cross-contamination.")

    shutil.rmtree(temp_dir, ignore_errors=True)
    return {
        "name": "1.3 Concurrent Switching & Credential Integrity",
        "switches": total_switches,
        "errors": total_switch_errors,
        "cross_contaminated": len(cross_contaminated),
        "contaminated_details": cross_contaminated,
        "pass": passed,
    }


def run_test_1_4_concurrent_switch_and_read(num_switchers: int = 4, num_readers: int = 4, duration: float = 2.0) -> dict:
    """Test 1.4: Concurrent switches while readers poll state continuously."""
    print(f"\n--- [TEST 1.4] Concurrent Switch & Continuous Read ({num_switchers} switchers, {num_readers} readers, {duration}s) ---")
    temp_dir = tempfile.mkdtemp(prefix="stress_switch_read_")
    vault_path = os.path.join(temp_dir, "accounts.json")
    bin_dir = os.path.join(temp_dir, "bin")
    keyring_state = os.path.join(temp_dir, "keyring.json")

    mock_script = create_mock_secret_tool_script(bin_dir, keyring_state)

    vault = AccountVault(config_path=vault_path)
    accounts = [f"account_{i}@example.com" for i in range(4)]
    for i, email in enumerate(accounts):
        cred = KeyringCredential(access_token=f"tok_{i}", refresh_token=f"ref_{i}")
        vault.add_or_update_account(email, cred)

    backend = SecretToolBackend(binary_path=mock_script)
    backend.store(vault.get_account(accounts[0]).credential.to_antigravity_json())
    vault.set_active_account(accounts[0])

    start_time = time.time()
    with concurrent.futures.ProcessPoolExecutor(max_workers=num_switchers + num_readers) as executor:
        switch_futures = [
            executor.submit(_worker_concurrent_switch_proc, vault_path, mock_script, accounts, 30, w_id)
            for w_id in range(num_switchers)
        ]
        read_futures = [
            executor.submit(_worker_poll_read_proc, vault_path, duration)
            for _ in range(num_readers)
        ]

        switch_res = [f.result() for f in concurrent.futures.as_completed(switch_futures)]
        read_res = [f.result() for f in concurrent.futures.as_completed(read_futures)]
    elapsed = time.time() - start_time

    total_switches = sum(r["switches"] for r in switch_res)
    total_reads = sum(r["reads"] for r in read_res)
    switch_errors = sum(len(r["errors"]) for r in switch_res)
    read_errors = sum(len(r["errors"]) for r in read_res)

    passed = (switch_errors == 0 and read_errors == 0)
    print(f"Elapsed: {elapsed:.2f}s | Switches: {total_switches} (err: {switch_errors}) | Reads: {total_reads} (err: {read_errors})")
    if not passed:
        print(f"[FAIL] Errors observed during concurrent read/write operations!")
        for r in read_res:
            if r["errors"]:
                print(f"  Read error: {r['errors'][:2]}")
    else:
        print("[PASS] Clean concurrent read and write operations.")

    shutil.rmtree(temp_dir, ignore_errors=True)
    return {
        "name": "1.4 Concurrent Switch & Read",
        "switches": total_switches,
        "reads": total_reads,
        "switch_errors": switch_errors,
        "read_errors": read_errors,
        "pass": passed,
    }


# ============================================================================
# Section 2: Malformed / Corrupted accounts.json Recovery & Resilience
# ============================================================================

def run_test_2_malformed_corrupted_accounts_json() -> dict:
    """Test 2: Malformed accounts.json handling, crash vs recovery, and repairability."""
    print(f"\n--- [TEST 2] Malformed & Corrupted accounts.json Recovery ---")
    temp_dir = tempfile.mkdtemp(prefix="stress_corrupt_vault_")
    vault_path = os.path.join(temp_dir, "accounts.json")
    results = {}

    # Case 2.1: Truncated JSON
    with open(vault_path, "w", encoding="utf-8") as f:
        f.write('{"version": 1, "active_account": "user@test.com", "accounts": {"user@test.com": {"credential": {')
    vault = AccountVault(config_path=vault_path)

    truncated_raised = False
    try:
        vault.load()
    except AccountVaultCorruptedError:
        truncated_raised = True
    except Exception as exc:
        results["case_2_1_unexpected_exc"] = f"{type(exc).__name__}: {exc}"

    # Can AccountVault recover / repair by saving a new account when the file is corrupt?
    repair_succeeded = False
    try:
        cred = KeyringCredential("token_new", "ref_new")
        vault.add_or_update_account("new@test.com", cred)
        repair_succeeded = True
    except AccountVaultCorruptedError:
        repair_succeeded = False
    except Exception as exc:
        repair_succeeded = False
        results["case_2_1_repair_exc"] = f"{type(exc).__name__}: {exc}"

    # Case 2.2: Binary garbage in accounts.json
    with open(vault_path, "wb") as f:
        f.write(b"\x00\xff\xfe\x00\x01\x80\x90\xaa\xbb\xcc\xdd\xee")
    binary_raised = False
    try:
        vault.load()
    except (AccountVaultCorruptedError, UnicodeDecodeError):
        binary_raised = True
    except Exception as exc:
        results["case_2_2_unexpected_exc"] = f"{type(exc).__name__}: {exc}"

    # Case 2.3: Non-dict root JSON (e.g. list '[1, 2, 3]' or integer '123')
    with open(vault_path, "w", encoding="utf-8") as f:
        f.write("[1, 2, 3]")
    non_dict_clean_error = False
    try:
        accs = vault.list_accounts()
    except (AccountVaultCorruptedError, InvalidCredentialError):
        non_dict_clean_error = True
    except AttributeError as exc:
        # Fails with unhandled AttributeError: 'list' object has no attribute 'get'
        non_dict_clean_error = False
        results["case_2_3_attribute_error"] = str(exc)
    except Exception as exc:
        non_dict_clean_error = False
        results["case_2_3_other_exc"] = f"{type(exc).__name__}: {exc}"

    # Case 2.4: Corrupted account record inside JSON (accounts map contains string or None)
    with open(vault_path, "w", encoding="utf-8") as f:
        f.write(json.dumps({"version": 1, "active_account": None, "accounts": {"bad@test.com": None}}))
    record_corrupt_clean = False
    try:
        records = vault.list_account_records()
    except (AccountVaultCorruptedError, InvalidCredentialError):
        record_corrupt_clean = True
    except (TypeError, KeyError, AttributeError) as exc:
        record_corrupt_clean = False
        results["case_2_4_unhandled_exc"] = f"{type(exc).__name__}: {exc}"

    print(f"Case 2.1 Truncated JSON detected properly: {truncated_raised}")
    print(f"Case 2.1 Auto-recovery / repair capability: {repair_succeeded} (False indicates permanent lockup/unusable vault)")
    print(f"Case 2.2 Binary garbage handled: {binary_raised}")
    print(f"Case 2.3 Non-dict JSON root clean error: {non_dict_clean_error} (False = unhandled AttributeError)")
    print(f"Case 2.4 Corrupted record clean error: {record_corrupt_clean} (False = unhandled TypeError/AttributeError)")

    shutil.rmtree(temp_dir, ignore_errors=True)
    all_passed = (truncated_raised and repair_succeeded and binary_raised and non_dict_clean_error and record_corrupt_clean)
    return {
        "name": "2. Malformed / Corrupted accounts.json Recovery",
        "truncated_raised": truncated_raised,
        "repair_succeeded": repair_succeeded,
        "binary_raised": binary_raised,
        "non_dict_clean_error": non_dict_clean_error,
        "record_corrupt_clean": record_corrupt_clean,
        "diagnostics": results,
        "pass": all_passed,
    }


# ============================================================================
# Section 3: Trailing Newline & Binary Payload Injections
# ============================================================================

def run_test_3_payload_and_newline_injections() -> dict:
    """Test 3: Non-dict inputs, binary bytes, newlines, and null bytes injection."""
    print(f"\n--- [TEST 3] Trailing Newline & Binary Payload Injections ---")
    results = {}

    # Case 3.1: Non-dict JSON payload to KeyringCredential.from_antigravity_json
    test_payloads = ["[1, 2, 3]", "12345", '"just_a_string"', "true", "null"]
    case_3_1_failures = []
    for p in test_payloads:
        try:
            KeyringCredential.from_antigravity_json(p)
            case_3_1_failures.append((p, "NoExceptionRaised"))
        except InvalidCredentialError:
            pass  # Expected domain error
        except Exception as exc:
            case_3_1_failures.append((p, f"Unhandled {type(exc).__name__}: {exc}"))

    results["case_3_1_from_json_failures"] = case_3_1_failures
    print(f"Case 3.1 Non-dict payloads in from_antigravity_json: {len(case_3_1_failures)} unhandled exceptions out of {len(test_payloads)}")
    if case_3_1_failures:
        for p, err in case_3_1_failures:
            print(f"  - Payload {p!r} -> {err}")

    # Case 3.2: Non-UTF8 binary payload returned by secret-tool lookup
    temp_dir = tempfile.mkdtemp(prefix="stress_secret_tool_")
    bin_dir = os.path.join(temp_dir, "bin")
    keyring_state = os.path.join(temp_dir, "keyring.json")
    mock_script = create_mock_secret_tool_script(bin_dir, keyring_state)

    # Inject non-UTF8 bytes directly into mock keyring state
    # Create a script that outputs non-utf8 bytes
    non_utf8_script_path = os.path.join(bin_dir, "secret-tool-non-utf8")
    with open(non_utf8_script_path, "wb") as f:
        f.write(b"#!/bin/sh\n/usr/bin/printf '\\x80\\xff\\xfe'\n")
    os.chmod(non_utf8_script_path, 0o755)

    backend_non_utf8 = SecretToolBackend(binary_path=non_utf8_script_path)
    case_3_2_handled = False
    try:
        backend_non_utf8.lookup(service="gemini", username="antigravity")
    except KeyringError:
        case_3_2_handled = True
    except UnicodeDecodeError as exc:
        case_3_2_handled = False
        results["case_3_2_unhandled"] = f"Unhandled UnicodeDecodeError: {exc}"
    except Exception as exc:
        case_3_2_handled = False
        results["case_3_2_unhandled"] = f"Unhandled {type(exc).__name__}: {exc}"

    print(f"Case 3.2 Non-UTF-8 binary output in lookup handled: {case_3_2_handled}")
    if not case_3_2_handled:
        print(f"  - Lookup crashed with: {results.get('case_3_2_unhandled')}")

    # Case 3.3: Trailing newline in lookup output
    # Real secret-tool or CLI wrappers often append '\n'
    newline_script_path = os.path.join(bin_dir, "secret-tool-nl")
    with open(newline_script_path, "w", encoding="utf-8") as f:
        f.write("#!/bin/sh\nprintf 'my_secret_token\\n'\n")
    os.chmod(newline_script_path, 0o755)

    backend_nl = SecretToolBackend(binary_path=newline_script_path)
    lookup_val = backend_nl.lookup(service="gemini", username="antigravity")
    lookup_has_trailing_nl = lookup_val.endswith("\n") if lookup_val else False
    print(f"Case 3.3 Trailing newline stripped by SecretToolBackend.lookup(): {not lookup_has_trailing_nl} (Raw: {lookup_val!r})")

    # Case 3.4: Null bytes in secret payload
    payload_with_null = "token\x00middle\x00nulls"
    backend_mock = SecretToolBackend(binary_path=mock_script)
    backend_mock.store(payload_with_null, service="test_null", username="user_null")
    retrieved_null = backend_mock.lookup(service="test_null", username="user_null")
    null_preserved = (retrieved_null == payload_with_null)
    print(f"Case 3.4 Null byte payload preserved: {null_preserved}")

    shutil.rmtree(temp_dir, ignore_errors=True)
    all_passed = (len(case_3_1_failures) == 0 and case_3_2_handled and not lookup_has_trailing_nl)
    return {
        "name": "3. Trailing Newline & Binary Payload Injections",
        "case_3_1_failures": len(case_3_1_failures),
        "case_3_2_handled": case_3_2_handled,
        "case_3_3_lookup_stripped": not lookup_has_trailing_nl,
        "case_3_4_null_preserved": null_preserved,
        "pass": all_passed,
    }


# ============================================================================
# Section 4: Race Conditions during Rapid Account Rotation
# ============================================================================

def run_test_4_rapid_rotation_races() -> dict:
    """Test 4: Rapid rotation between accounts in tight loop to verify active pointer consistency."""
    print(f"\n--- [TEST 4] Rapid Account Rotation & Active State Synchronization ---")
    temp_dir = tempfile.mkdtemp(prefix="stress_rapid_rot_")
    vault_path = os.path.join(temp_dir, "accounts.json")
    bin_dir = os.path.join(temp_dir, "bin")
    keyring_state = os.path.join(temp_dir, "keyring.json")

    mock_script = create_mock_secret_tool_script(bin_dir, keyring_state)
    backend = SecretToolBackend(binary_path=mock_script)
    vault = AccountVault(config_path=vault_path)
    service = KeyringService(backend=backend, vault=vault)

    # Seed 3 distinct accounts
    accounts = ["alpha@gmail.com", "beta@gmail.com", "gamma@gmail.com"]
    tokens = {"alpha@gmail.com": "ya29.tok_alpha", "beta@gmail.com": "ya29.tok_beta", "gamma@gmail.com": "ya29.tok_gamma"}
    for email in accounts:
        cred = KeyringCredential(access_token=tokens[email], refresh_token=f"ref_{email}")
        vault.add_or_update_account(email, cred)

    backend.store(vault.get_account(accounts[0]).credential.to_antigravity_json())
    vault.set_active_account(accounts[0])

    mismatches = []
    # Rapidly rotate through accounts 30 times
    for cycle in range(30):
        target = accounts[cycle % len(accounts)]
        service.switch_account(target, reason="rapid_test")

        active_in_vault = vault.get_active_account()
        active_cred = service.get_active_credential()
        expected_token = tokens[target]

        if active_in_vault != target:
            mismatches.append((cycle, target, f"Vault active is {active_in_vault}"))
        if active_cred.access_token != expected_token:
            mismatches.append((cycle, target, f"Keyring token is {active_cred.access_token} (expected {expected_token})"))

    passed = len(mismatches) == 0
    print(f"Completed 30 rapid switches | State mismatches: {len(mismatches)}")
    if not passed:
        print(f"[FAIL] State mismatches observed during rapid rotation: {mismatches[:3]}")
    else:
        print("[PASS] Vault active account strictly matches Secret Service at every rotation step.")

    shutil.rmtree(temp_dir, ignore_errors=True)
    return {
        "name": "4. Rapid Account Rotation & Active State Synchronization",
        "mismatches": len(mismatches),
        "pass": passed,
    }


# ============================================================================
# Main Runner
# ============================================================================

def main():
    print("================================================================================")
    print("   ADVERSARIAL STRESS TEST SUITE: M1 KEYRING SWITCHER & ACCOUNT VAULT          ")
    print("================================================================================")

    res_1_1 = run_test_1_1_multiprocess_lost_updates()
    res_1_2 = run_test_1_2_multithread_lost_updates()
    res_1_3 = run_test_1_3_credential_cross_contamination()
    res_1_4 = run_test_1_4_concurrent_switch_and_read()
    res_2 = run_test_2_malformed_corrupted_accounts_json()
    res_3 = run_test_3_payload_and_newline_injections()
    res_4 = run_test_4_rapid_rotation_races()

    all_results = [res_1_1, res_1_2, res_1_3, res_1_4, res_2, res_3, res_4]

    print("\n================================================================================")
    print("                         ADVERSARIAL SUMMARY MATRIX                             ")
    print("================================================================================")
    for r in all_results:
        status = "PASS" if r["pass"] else "FAIL"
        print(f"[{status:4s}] {r['name']}")

    total_passed = sum(1 for r in all_results if r["pass"])
    total_tests = len(all_results)
    print(f"\nFinal Score: {total_passed}/{total_tests} passed ({total_passed/total_tests*100:.1f}%)")

    # Output detailed JSON for reporting
    json_out = {
        "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "summary": {
            "total": total_tests,
            "passed": total_passed,
            "failed": total_tests - total_passed,
        },
        "results": all_results,
    }
    with open("tests/stress_results.json", "w", encoding="utf-8") as f:
        json.dump(json_out, f, indent=2)

    return 0 if total_passed == total_tests else 1


# ============================================================================
# PyTest Integrations
# ============================================================================

def test_adversarial_multiprocess_lost_updates():
    res = run_test_1_1_multiprocess_lost_updates(num_workers=4, accounts_per_worker=15)
    assert res["pass"], f"Lost {res['lost']} accounts in multi-process additions!"


def test_adversarial_multithread_lost_updates():
    res = run_test_1_2_multithread_lost_updates(num_threads=4, accounts_per_thread=15)
    assert res["pass"], f"Lost {res['lost']} accounts in multi-threaded additions!"


def test_adversarial_credential_cross_contamination():
    res = run_test_1_3_credential_cross_contamination(num_switchers=3, iterations=15)
    assert res["pass"], f"Detected credential cross-contamination in {res['cross_contaminated']} accounts!"


def test_adversarial_concurrent_switch_and_read():
    res = run_test_1_4_concurrent_switch_and_read(num_switchers=3, num_readers=3, duration=1.0)
    assert res["pass"], f"Encountered errors during concurrent switch and read!"


def test_adversarial_malformed_accounts_json():
    res = run_test_2_malformed_corrupted_accounts_json()
    assert res["pass"], f"AccountVault failed corrupted JSON recovery/handling!"


def test_adversarial_payload_and_newline_injections():
    res = run_test_3_payload_and_newline_injections()
    assert res["pass"], f"SecretTool / Credential parser failed injection handling!"


def test_adversarial_rapid_rotation_races():
    res = run_test_4_rapid_rotation_races()
    assert res["pass"], f"Detected state mismatches during rapid rotation!"


if __name__ == "__main__":
    sys.exit(main())

