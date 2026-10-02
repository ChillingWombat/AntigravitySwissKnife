"""
Adversarial Stress Test 1: High-Concurrency Account Switching and Vault Operations
Tests multiprocessing and multithreading concurrency across AccountVault, KeyringService,
and KeyringSwitcher to detect race conditions, lost updates, and credential cross-contamination.
"""

import concurrent.futures
import json
import multiprocessing as mp
import os
import shutil
import sys
import tempfile
import time
from pathlib import Path

# Ensure project root is in sys.path
PROJECT_ROOT = Path(__file__).resolve().parents[3]
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

from antigravity_swiss.core.errors import (
    AccountNotFoundError,
    AccountVaultCorruptedError,
    InvalidCredentialError,
    KeyringError,
)
from antigravity_swiss.keyring.secret_tool import SecretToolBackend
from antigravity_swiss.keyring.switcher import (
    AccountRecord,
    AccountVault,
    KeyringCredential,
    KeyringService,
)
from tests.fixtures.mock_keyring import create_mock_secret_tool_script
from tests.fixtures.test_helpers import CredentialBuilder


def _worker_add_account(vault_path: str, worker_id: int, num_accounts: int) -> dict:
    """Worker function adding distinct accounts to the same AccountVault."""
    vault = AccountVault(config_path=vault_path)
    added = []
    errors = []
    for i in range(num_accounts):
        email = f"worker_{worker_id}_acc_{i}@example.com"
        token = f"token_{worker_id}_{i}"
        cred = KeyringCredential(access_token=token, refresh_token=f"ref_{token}")
        try:
            vault.add_or_update_account(email, cred, label=f"Worker {worker_id} Acc {i}")
            added.append(email)
        except Exception as exc:
            errors.append((email, str(exc)))
    return {"worker_id": worker_id, "added": added, "errors": errors}


def _worker_read_vault(vault_path: str, duration_sec: float) -> dict:
    """Worker continuously reading accounts and active account."""
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
            errors.append(str(exc))
        time.sleep(0.005)
    return {"reads": reads, "errors": errors}


def _worker_concurrent_switch(
    vault_path: str,
    secret_tool_path: str,
    account_emails: list[str],
    iterations: int,
    worker_id: int,
) -> dict:
    """Worker switching accounts concurrently using KeyringService."""
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
            errors.append((target, str(exc)))
        time.sleep(0.002)

    return {"worker_id": worker_id, "switches": switches, "errors": errors}


def run_test_concurrent_account_addition(num_workers=8, accounts_per_worker=25):
    """
    Stress-tests concurrent add_or_update_account across multiple processes.
    Verifies if all accounts are preserved without lost updates (TOCTOU race).
    """
    print(f"\n--- [TEST 1.1] Concurrent Account Addition ({num_workers} processes, {accounts_per_worker} accs each) ---")
    temp_dir = tempfile.mkdtemp(prefix="swiss_concur_add_")
    vault_path = os.path.join(temp_dir, "accounts.json")

    total_expected = num_workers * accounts_per_worker
    all_added_emails = set()

    start_time = time.time()
    with concurrent.futures.ProcessPoolExecutor(max_workers=num_workers) as executor:
        futures = [
            executor.submit(_worker_add_account, vault_path, w_id, accounts_per_worker)
            for w_id in range(num_workers)
        ]
        results = [f.result() for f in concurrent.futures.as_completed(futures)]

    elapsed = time.time() - start_time

    for res in results:
        all_added_emails.update(res["added"])
        if res["errors"]:
            print(f"Worker {res['worker_id']} reported errors: {res['errors']}")

    vault = AccountVault(config_path=vault_path)
    final_accounts = vault.list_accounts()
    final_count = len(final_accounts)

    print(f"Time elapsed: {elapsed:.2f}s")
    print(f"Expected accounts: {total_expected}")
    print(f"Unique accounts requested: {len(all_added_emails)}")
    print(f"Actual accounts in vault: {final_count}")

    lost_accounts = all_added_emails - set(final_accounts)
    if lost_accounts:
        print(f"[FAIL / VULNERABILITY CONFIRMED] Lost {len(lost_accounts)} accounts due to read-modify-write race condition!")
        print(f"Sample lost accounts: {list(lost_accounts)[:5]}")
    else:
        print("[PASS] All accounts successfully recorded without data loss.")

    shutil.rmtree(temp_dir, ignore_errors=True)
    return {
        "expected": total_expected,
        "actual": final_count,
        "lost": len(lost_accounts),
        "pass": len(lost_accounts) == 0,
    }


def run_test_concurrent_switch_and_read(num_switchers=4, num_readers=4, iterations=30):
    """
    Stress-tests concurrent account switching while background readers poll vault.
    Checks for file corruption, lock contention timeouts, and credential consistency.
    """
    print(f"\n--- [TEST 1.2] Concurrent Switch & Read ({num_switchers} switchers, {num_readers} readers) ---")
    temp_dir = tempfile.mkdtemp(prefix="swiss_concur_switch_")
    vault_path = os.path.join(temp_dir, "accounts.json")
    bin_dir = os.path.join(temp_dir, "bin")
    keyring_state = os.path.join(temp_dir, "keyring.json")

    mock_script = create_mock_secret_tool_script(bin_dir, keyring_state)

    # Seed 5 accounts
    vault = AccountVault(config_path=vault_path)
    accounts = []
    for i in range(5):
        email = f"target_{i}@example.com"
        cred = KeyringCredential(
            access_token=f"ya29.token_{i}",
            refresh_token=f"refresh_{i}",
            id_token=f"ey.eyJlbWFpbCI6ICJ7ZW1haWx9In0=.sig",
        )
        vault.add_or_update_account(email, cred, label=f"Target {i}")
        accounts.append(email)

    # Seed keyring with account 0
    backend = SecretToolBackend(binary_path=mock_script)
    backend.store(vault.get_account(accounts[0]).credential.to_antigravity_json())

    start_time = time.time()
    with concurrent.futures.ProcessPoolExecutor(max_workers=num_switchers + num_readers) as executor:
        switch_futures = [
            executor.submit(_worker_concurrent_switch, vault_path, mock_script, accounts, iterations, w_id)
            for w_id in range(num_switchers)
        ]
        read_futures = [
            executor.submit(_worker_read_vault, vault_path, 2.0)
            for _ in range(num_readers)
        ]

        switch_results = [f.result() for f in concurrent.futures.as_completed(switch_futures)]
        read_results = [f.result() for f in concurrent.futures.as_completed(read_futures)]

    elapsed = time.time() - start_time
    total_switches = sum(r["switches"] for r in switch_results)
    total_switch_errors = sum(len(r["errors"]) for r in switch_results)
    total_reads = sum(r["reads"] for r in read_results)
    total_read_errors = sum(len(r["errors"]) for r in read_results)

    print(f"Time elapsed: {elapsed:.2f}s")
    print(f"Total switches completed: {total_switches}, errors: {total_switch_errors}")
    print(f"Total reads completed: {total_reads}, errors: {total_read_errors}")

    # Check for credential cross-contamination
    # Each account in vault should still hold its own token, NOT another account's token!
    cross_contaminated = []
    for i, email in enumerate(accounts):
        rec = vault.get_account(email)
        expected_token = f"ya29.token_{i}"
        actual_token = rec.credential.access_token
        if actual_token != expected_token:
            cross_contaminated.append((email, expected_token, actual_token))

    if cross_contaminated:
        print(f"[FAIL / CRITICAL BUG] Credential cross-contamination detected!")
        for email, exp, act in cross_contaminated:
            print(f"  Account {email}: expected {exp}, but found {act}!")
    else:
        print("[PASS] No credential cross-contamination found in vault.")

    shutil.rmtree(temp_dir, ignore_errors=True)
    return {
        "switches": total_switches,
        "switch_errors": total_switch_errors,
        "reads": total_reads,
        "read_errors": total_read_errors,
        "cross_contaminated": len(cross_contaminated),
        "pass": (total_switch_errors == 0 and total_read_errors == 0 and len(cross_contaminated) == 0),
    }


if __name__ == "__main__":
    res1 = run_test_concurrent_account_addition()
    res2 = run_test_concurrent_switch_and_read()
    print("\nSummary:")
    print(f"Test 1.1 Result: {'PASS' if res1['pass'] else 'FAIL'}")
    print(f"Test 1.2 Result: {'PASS' if res2['pass'] else 'FAIL'}")
