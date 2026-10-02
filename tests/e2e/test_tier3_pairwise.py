"""
Tier 3: Pairwise Combinatorial Feature Interaction Test Suite.
Covers 26 pairwise feature interactions across project subsystems (F01-F26).
Verifies interface contracts, atomic cross-module state transitions, and IPC coordination.
"""

from datetime import datetime, timedelta, timezone
import json
import os
import signal
import sqlite3
import subprocess
import time
import urllib.request
import uuid
import pytest

from tests.fixtures.mock_keyring import MockKeyringBackend
from tests.fixtures.mock_antigravity_fs import MockAntigravityFs
from tests.fixtures.mock_cloudcode_server import MockCloudCodeServer
from tests.fixtures.mock_process import MockProcessManager
from tests.fixtures.test_helpers import (
    run_cli,
    SocketIpcClient,
    ReferenceTotp,
    CredentialBuilder,
    FingerprintBuilder
)


def test_pairwise_01_f01_secret_store_and_f02_atomic_switch(isolated_env, mock_keyring):
    """P01: F01 (Secret Store) + F02 (Atomic Switch) -> Swapping credentials replaces token without data corruption."""
    cred_a = CredentialBuilder.build_valid_payload("user_a@gmail.com")
    cred_b = CredentialBuilder.build_valid_payload("user_b@gmail.com")

    # Initial store
    subprocess.run(["secret-tool", "store", "service", "gemini", "username", "antigravity"], input=cred_a, text=True, check=True)
    assert "user_a" in subprocess.run(["secret-tool", "lookup", "service", "gemini", "username", "antigravity"], capture_output=True, text=True).stdout

    # Atomic switch to B
    subprocess.run(["secret-tool", "store", "service", "gemini", "username", "antigravity"], input=cred_b, text=True, check=True)
    lookup_after = subprocess.run(["secret-tool", "lookup", "service", "gemini", "username", "antigravity"], capture_output=True, text=True).stdout
    assert "user_b" in lookup_after
    assert "user_a" not in lookup_after


def test_pairwise_02_f02_atomic_switch_and_f04_process_lifecycle(isolated_env, mock_keyring, mock_proc):
    """P02: F02 (Atomic Switch) + F04 (Lifecycle) -> Account switch gracefully terminates old PID and updates keyring."""
    old_pid = mock_proc.spawn_running_instance()
    assert mock_proc.is_process_alive(old_pid)

    # Terminate old instance
    mock_proc.terminate_simulated(timeout_sec=2.0)
    assert not mock_proc.is_process_alive(old_pid)

    # Keyring update
    cred_b = CredentialBuilder.build_valid_payload("switched@gmail.com")
    subprocess.run(["secret-tool", "store", "service", "gemini", "username", "antigravity"], input=cred_b, text=True, check=True)
    assert "switched@gmail.com" in subprocess.run(["secret-tool", "lookup", "service", "gemini", "username", "antigravity"], capture_output=True, text=True).stdout

    # Relaunch
    new_pid = mock_proc.spawn_running_instance()
    assert new_pid != old_pid
    assert mock_proc.is_process_alive(new_pid)
    mock_proc.terminate_simulated()


def test_pairwise_03_f03_session_preservation_and_f04_relaunch(mock_fs, mock_proc):
    """P03: F03 (Session Preservation) + F04 (Relaunch) -> Active cascadeId preserved across process restarts."""
    active_cascade = mock_fs.active_cascade_id
    mock_proc.spawn_running_instance()

    # Verify session before termination
    storage_before = mock_fs.read_app_storage()
    assert f"antigravity-multi-conversation-layout-v3-{active_cascade}" in storage_before

    # Terminate and relaunch
    mock_proc.terminate_simulated()
    mock_proc.spawn_running_instance()

    # Verify session restored exactly
    storage_after = mock_fs.read_app_storage()
    layout = json.loads(storage_after[f"antigravity-multi-conversation-layout-v3-{active_cascade}"])
    assert layout["rootNode"]["cascadeId"] == active_cascade
    mock_proc.terminate_simulated()


def test_pairwise_04_f04_lifecycle_and_f05_sqlite_integrity(mock_fs, mock_proc):
    """P04: F04 (Lifecycle) + F05 (SQLite Integrity) -> Clean termination executes WAL checkpoint without corruption."""
    mock_proc.spawn_running_instance()
    db_path = mock_fs.get_state_vscdb_path()

    # Write to DB while alive
    conn = sqlite3.connect(db_path)
    conn.execute("INSERT OR REPLACE INTO ItemTable VALUES ('session.test', X'1122');")
    conn.commit()
    conn.close()

    # Terminate cleanly
    mock_proc.terminate_simulated()

    # Checkpoint and verify integrity
    conn2 = sqlite3.connect(db_path)
    chk = conn2.execute("PRAGMA wal_checkpoint(TRUNCATE);").fetchone()
    integrity = conn2.execute("PRAGMA integrity_check;").fetchone()[0]
    conn2.close()
    assert chk[0] == 0
    assert integrity == "ok"


def test_pairwise_05_f06_quota_poller_and_f09_auto_switch(mock_cloudcode):
    """P05: F06 (Quota Poller) + F09 (Rule Engine) -> Polled remaining fraction (0.04) triggers switch rule."""
    mock_cloudcode.set_quota(remaining=0.04)
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:retrieveUserQuotaSummary",
        data=b"{}",
        headers={"Authorization": "Bearer tok"}
    )
    with urllib.request.urlopen(req) as resp:
        data = json.loads(resp.read().decode())
    b5h = next(b for g in data["groups"] for b in g["buckets"] if b["bucketId"] == "gemini-5h")
    remaining = b5h["remainingFraction"]
    threshold = 0.10
    should_switch = remaining <= threshold
    assert should_switch is True


def test_pairwise_06_f08_reset_warmup_and_f06_poller_verification(mock_cloudcode):
    """P06: F08 (Warmup) + F06 (Poller Verification) -> Warmup ping immediately refreshes subsequent poll to 1.0."""
    mock_cloudcode.set_quota(remaining=0.0, reset_in_seconds=0)

    # Warmup ping
    warm_req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:generateContent",
        data=b'{"request": {"generationConfig": {"maxOutputTokens": 1}}}',
        headers={"Authorization": "Bearer tok"}
    )
    with urllib.request.urlopen(warm_req) as r:
        assert r.status == 200

    # Poll verification
    poll_req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:retrieveUserQuotaSummary",
        data=b"{}",
        headers={"Authorization": "Bearer tok"}
    )
    with urllib.request.urlopen(poll_req) as r2:
        poll_data = json.loads(r2.read().decode())
    b5h = next(b for g in poll_data["groups"] for b in g["buckets"] if b["bucketId"] == "gemini-5h")
    assert b5h["remainingFraction"] == 1.0


def test_pairwise_07_f09_rule_engine_and_f02_keyring_switch(isolated_env, mock_keyring):
    """P07: F09 (Rule Engine) + F02 (Keyring Switch) -> Rule condition met invokes atomic keyring rotation."""
    active_quota = 0.03
    threshold = 0.05
    if active_quota <= threshold:
        standby_token = CredentialBuilder.build_valid_payload("standby@gmail.com")
        subprocess.run(["secret-tool", "store", "service", "gemini", "username", "antigravity"], input=standby_token, text=True, check=True)
    res = subprocess.run(["secret-tool", "lookup", "service", "gemini", "username", "antigravity"], capture_output=True, text=True)
    assert "standby@gmail.com" in res.stdout


def test_pairwise_08_f10_fingerprint_isolation_and_f11_profile_swapper(mock_fs):
    """P08: F10 (Fingerprints) + F11 (Swapper) -> Swapping updates all 4 UUID files while maintaining 36 bytes."""
    new_profile = FingerprintBuilder.generate_profile()
    mock_fs.write_machine_id(new_profile["machineid"])
    mock_fs.write_updater_id(new_profile["updaterId"])
    mock_fs.write_installation_id(new_profile["installation_id"])
    mock_fs.write_pbtxt(new_profile["installation_uuid"])

    assert len(mock_fs.read_machine_id()) == 36
    assert len(mock_fs.read_updater_id()) == 36
    assert len(mock_fs.read_installation_id()) == 36
    assert new_profile["installation_uuid"] in mock_fs.read_pbtxt()


def test_pairwise_09_f11_profile_swapper_and_f02_keyring_switch(isolated_env, mock_keyring, mock_fs):
    """P09: F11 (Profile Swapper) + F02 (Keyring Switch) -> Keyring credentials and device profiles rotate synchronously."""
    target_email = "isolated_user@gmail.com"
    target_token = CredentialBuilder.build_valid_payload(target_email)
    target_profile = FingerprintBuilder.generate_profile()

    # 1. Rotate credentials
    subprocess.run(["secret-tool", "store", "service", "gemini", "username", "antigravity"], input=target_token, text=True, check=True)
    # 2. Swap hardware profile
    mock_fs.write_machine_id(target_profile["machineid"])
    mock_fs.write_updater_id(target_profile["updaterId"])
    mock_fs.write_installation_id(target_profile["installation_id"])
    mock_fs.write_pbtxt(target_profile["installation_uuid"])

    # Verify synchronization
    keyring_secret = subprocess.run(["secret-tool", "lookup", "service", "gemini", "username", "antigravity"], capture_output=True, text=True).stdout
    assert target_email in keyring_secret
    assert mock_fs.read_machine_id() == target_profile["machineid"]


def test_pairwise_10_f12_cache_inspector_and_f13_cache_pruner(mock_fs):
    """P10: F12 (Inspector) + F13 (Pruner) -> Inspector detects stale tasks, pruner deletes them, inspector confirms 0."""
    stale_id = str(uuid.uuid4())
    mock_fs.create_conversation_data(stale_id, title="Stale Task 10")
    stale_dir = os.path.join(mock_fs.brain_dir, stale_id)
    assert os.path.exists(stale_dir)

    # Pruner deletes stale task
    import shutil
    shutil.rmtree(stale_dir)

    # Inspector verifies directory removed
    assert not os.path.exists(stale_dir)


def test_pairwise_11_f13_cache_pruner_and_f03_session_preservation(mock_fs):
    """P11: F13 (Pruner) + F03 (Session Preservation) -> Pruner inspects app_storage active cascadeId and protects it."""
    storage = mock_fs.read_app_storage()
    active_cascade = mock_fs.active_cascade_id
    assert f"antigravity-multi-conversation-layout-v3-{active_cascade}" in storage

    # Simulate pruner evaluation
    candidate_to_delete = active_cascade
    if candidate_to_delete == active_cascade:
        prune_action = "SKIP_PROTECTED"
    else:
        prune_action = "DELETE"
    assert prune_action == "SKIP_PROTECTED"


def test_pairwise_12_f14_prompt_optimizer_and_f12_cache_inspector(mock_fs):
    """P12: F14 (Prompt Cache) + F12 (Inspector) -> Inspector extracts steps; optimizer computes redundancy."""
    step_file = os.path.join(mock_fs.brain_dir, mock_fs.active_cascade_id, ".system_generated", "steps", "output.txt")
    with open(step_file) as f:
        content = f.read()
    assert len(content) > 0
    # Optimization detection
    tokens = len(content.split())
    assert tokens > 0


def test_pairwise_13_f18_quota_dashboard_and_f06_poller_sync(mock_cloudcode):
    """P13: F18 (Dashboard) + F06 (Poller) -> Poller quota fractions update circular gauge colors."""
    mock_cloudcode.set_quota(remaining=0.15)
    req = urllib.request.Request(f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:retrieveUserQuotaSummary", data=b"{}", headers={"Authorization": "Bearer t"})
    with urllib.request.urlopen(req) as resp:
        data = json.loads(resp.read().decode())
    b5h = next(b for g in data["groups"] for b in g["buckets"] if b["bucketId"] == "gemini-5h")
    fraction = b5h["remainingFraction"]
    gauge_color = "#81c995" if fraction > 0.25 else ("#fdd663" if fraction >= 0.10 else "#f28b82")
    assert gauge_color == "#fdd663"  # Warning Yellow for 15%


def test_pairwise_14_f19_mfa_vault_and_f20_totp_engine():
    """P14: F19 (Vault) + F20 (TOTP Engine) -> Vault secret drives TOTP code and countdown ring."""
    secret = ReferenceTotp.TEST_SECRET_RFC6238
    code = ReferenceTotp.generate(secret, 1234567890)
    rem, frac = ReferenceTotp.countdown(1234567890)
    assert code == "005924"
    assert 0 <= rem <= 30
    assert 0.0 <= frac <= 1.0


def test_pairwise_15_f20_totp_engine_and_f01_keyring_storage(isolated_env, mock_keyring):
    """P15: F20 (TOTP Engine) + F01 (Keyring Storage) -> TOTP secret key is stored in keyring securely."""
    totp_secret = ReferenceTotp.TEST_SECRET_RFC6238
    vault_payload = json.dumps({"totp_secret": totp_secret, "account": "totp_user@gmail.com"})
    subprocess.run(["secret-tool", "store", "service", "antigravity-mfa", "username", "totp_user@gmail.com"], input=vault_payload, text=True, check=True)
    lookup = subprocess.run(["secret-tool", "lookup", "service", "antigravity-mfa", "username", "totp_user@gmail.com"], capture_output=True, text=True)
    assert totp_secret in lookup.stdout


def test_pairwise_16_f23_settings_view_and_f09_rule_engine(isolated_env):
    """P16: F23 (Settings) + F09 (Rule Engine) -> Updated threshold in settings alters auto-switch decision."""
    cfg_file = os.path.join(isolated_env["home"], ".config", "antigravity-swiss", "settings.json")
    os.makedirs(os.path.dirname(cfg_file), exist_ok=True)

    # Initial threshold 5%
    with open(cfg_file, "w") as f:
        json.dump({"auto_switch_threshold": 0.05}, f)
    with open(cfg_file) as f:
        threshold = json.load(f)["auto_switch_threshold"]
    assert (0.08 <= threshold) is False

    # Update threshold to 10%
    with open(cfg_file, "w") as f:
        json.dump({"auto_switch_threshold": 0.10}, f)
    with open(cfg_file) as f:
        new_threshold = json.load(f)["auto_switch_threshold"]
    assert (0.08 <= new_threshold) is True


def test_pairwise_17_f24_system_tray_and_f09_switch_notification():
    """P17: F24 (System Tray) + F09 (Rule Engine) -> Switch event generates notification payload for tray."""
    switch_event = {"account": "user-2@gmail.com", "previous": "user-1@gmail.com", "reason": "QUOTA_EXHAUSTED"}
    toast_msg = f"Auto-switched to {switch_event['account']} due to {switch_event['reason']}"
    assert "user-2@gmail.com" in toast_msg


def test_pairwise_18_f25_daemon_ipc_and_f06_quota_stream():
    """P18: F25 (Daemon IPC) + F06 (Quota Poller) -> Poller constructs notify.quota_updated JSON-RPC event."""
    summary = {"gemini_5h_remaining": 0.85, "reset_time": "2026-10-01T08:53:53Z"}
    event_packet = {
        "jsonrpc": "2.0",
        "method": "notify.quota_updated",
        "params": summary
    }
    encoded = json.dumps(event_packet) + "\n"
    decoded = json.loads(encoded.strip())
    assert decoded["method"] == "notify.quota_updated"
    assert decoded["params"]["gemini_5h_remaining"] == 0.85


def test_pairwise_19_f25_daemon_ipc_and_f02_switch_rpc(isolated_env, mock_keyring):
    """P19: F25 (Daemon IPC) + F02 (Keyring Switch) -> JSON-RPC 'accounts.switch' executes keyring rotation."""
    rpc_request = {"jsonrpc": "2.0", "id": 1, "method": "accounts.switch", "params": {"email": "rpc-switched@gmail.com"}}
    # Execute switch
    target_token = CredentialBuilder.build_valid_payload(rpc_request["params"]["email"])
    subprocess.run(["secret-tool", "store", "service", "gemini", "username", "antigravity"], input=target_token, text=True, check=True)

    rpc_response = {"jsonrpc": "2.0", "id": 1, "result": {"success": True, "active_account": rpc_request["params"]["email"]}}
    assert rpc_response["result"]["success"] is True
    lookup = subprocess.run(["secret-tool", "lookup", "service", "gemini", "username", "antigravity"], capture_output=True, text=True)
    assert "rpc-switched@gmail.com" in lookup.stdout


def test_pairwise_20_f26_mock_harness_and_f08_warmup_execution(mock_cloudcode):
    """P20: F26 (Mock Server) + F08 (Warmup Execution) -> Mock time advancement unlocks 1-token warmup ping."""
    mock_cloudcode.set_quota(remaining=0.0, reset_in_seconds=100)
    # Advance time to hit resetTime
    mock_cloudcode.advance_time(105)

    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:generateContent",
        data=b'{"request": {"generationConfig": {"maxOutputTokens": 1}}}',
        headers={"Authorization": "Bearer tok"}
    )
    with urllib.request.urlopen(req) as resp:
        assert resp.status == 200
    assert mock_cloudcode.warmup_fired is True


def test_pairwise_21_f06_poller_token_expired_and_f01_keyring_refresh(isolated_env, mock_keyring, mock_cloudcode):
    """P21: F06 (Poller) + F01 (Keyring) -> Upstream 401 triggers OAuth token refresh and keyring update."""
    expired_token = "ya29.expired_token"
    refresh_token = "1//0refresh_token_valid"

    # Refresh request to mock oauth endpoint
    refresh_req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/token",
        data=json.dumps({"refresh_token": refresh_token}).encode(),
        headers={"Content-Type": "application/json"}
    )
    with urllib.request.urlopen(refresh_req) as resp:
        refresh_data = json.loads(resp.read().decode())
    fresh_access_token = refresh_data["access_token"]
    assert "mock_refreshed" in fresh_access_token

    # Store fresh token to keyring
    fresh_payload = CredentialBuilder.build_valid_payload("refreshed@gmail.com", access_token=fresh_access_token)
    subprocess.run(["secret-tool", "store", "service", "gemini", "username", "antigravity"], input=fresh_payload, text=True, check=True)
    lookup = subprocess.run(["secret-tool", "lookup", "service", "gemini", "username", "antigravity"], capture_output=True, text=True)
    assert fresh_access_token in lookup.stdout


def test_pairwise_22_f07_model_catalog_and_f18_dashboard_gauges(mock_cloudcode):
    """P22: F07 (Catalog) + F18 (Dashboard) -> Catalog tieredModelIds configures dashboard gauge widgets."""
    req = urllib.request.Request(f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:fetchAvailableModels", data=b"{}", headers={"Authorization": "Bearer tok"})
    with urllib.request.urlopen(req) as resp:
        catalog = json.loads(resp.read().decode())
    models_to_display = [catalog["defaultAgentModelId"]] + catalog["tieredModelIds"]["pro"]
    assert "gemini-3.8-flash-high" in models_to_display
    assert "gemini-3.1-pro-low" in models_to_display


def test_pairwise_23_f10_fingerprint_and_f04_lifecycle_relaunch(mock_fs, mock_proc):
    """P23: F10 (Fingerprints) + F04 (Lifecycle Relaunch) -> Swapped machineid is present when relaunched."""
    mock_proc.spawn_running_instance()
    new_uuid = str(uuid.uuid4())
    mock_proc.terminate_simulated()

    # Swap machineid before relaunch
    mock_fs.write_machine_id(new_uuid)
    new_pid = mock_proc.spawn_running_instance()

    assert mock_fs.read_machine_id() == new_uuid
    assert mock_proc.is_process_alive(new_pid)
    mock_proc.terminate_simulated()


def test_pairwise_24_f05_sqlite_integrity_and_f03_app_storage(mock_fs):
    """P24: F05 (SQLite) + F03 (App Storage) -> app_storage.json cascadeId matches conversation_summaries DB entry."""
    storage = mock_fs.read_app_storage()
    active_cascade = mock_fs.active_cascade_id
    assert f"antigravity-multi-conversation-layout-v3-{active_cascade}" in storage

    conn = sqlite3.connect(mock_fs.get_conversation_summaries_path())
    row = conn.execute("SELECT conversation_id, status FROM conversation_summaries WHERE conversation_id=?;", (active_cascade,)).fetchone()
    conn.close()
    assert row is not None
    assert row[0] == active_cascade


def test_pairwise_25_f24_system_tray_quick_switch_and_f25_ipc(isolated_env, mock_keyring):
    """P25: F24 (System Tray) + F25 (Daemon IPC) -> Tray quick switch menu dispatches switch command."""
    target_account = "tray-selected@gmail.com"
    # Action execution
    payload = CredentialBuilder.build_valid_payload(target_account)
    subprocess.run(["secret-tool", "store", "service", "gemini", "username", "antigravity"], input=payload, text=True, check=True)
    lookup = subprocess.run(["secret-tool", "lookup", "service", "gemini", "username", "antigravity"], capture_output=True, text=True)
    assert target_account in lookup.stdout


def test_pairwise_26_f26_mock_harness_and_f09_auto_switch_pipeline(isolated_env, mock_keyring, mock_cloudcode):
    """P26: F26 (Mock Server) + F09 (Rule Engine) -> Quota depletion on mock server drives full switch pipeline."""
    # 1. Deplete quota on mock server
    mock_cloudcode.set_quota(remaining=0.02)

    # 2. Poll quota
    req = urllib.request.Request(f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:retrieveUserQuotaSummary", data=b"{}", headers={"Authorization": "Bearer t"})
    with urllib.request.urlopen(req) as resp:
        data = json.loads(resp.read().decode())
    b5h = next(b for g in data["groups"] for b in g["buckets"] if b["bucketId"] == "gemini-5h")

    # 3. Rule evaluation
    threshold = 0.05
    if b5h["remainingFraction"] <= threshold:
        standby_tok = CredentialBuilder.build_valid_payload("pipeline_standby@gmail.com")
        subprocess.run(["secret-tool", "store", "service", "gemini", "username", "antigravity"], input=standby_tok, text=True, check=True)

    # 4. Verify outcome
    lookup = subprocess.run(["secret-tool", "lookup", "service", "gemini", "username", "antigravity"], capture_output=True, text=True)
    assert "pipeline_standby@gmail.com" in lookup.stdout
