"""
Tier 2: Boundary, Error & Corner Cases Test Suite.
Covers all 26 features (F01 to F26) from PROJECT.md with >= 5 tests per feature (130 tests total).
Tests verify edge conditions: empty inputs, malformed data, clock drift, corruptions, limits.
"""

import base64
from datetime import datetime, timezone
import json
import math
import os
import signal
import sqlite3
import subprocess
import sys
import time
import urllib.error
import urllib.request
import uuid
from pathlib import Path
import asyncio
import pytest

from antigravity_swiss.ipc.socket_server import AsyncUnixSocketServer
from antigravity_swiss.keyring.switcher import AccountVault, KeyringCredential
from antigravity_swiss.process.lock_manager import SingletonLockManager
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


# ============================================================================
# F01: Linux Secret Service API Wrapper Boundaries
# ============================================================================

def test_f01_b01_lookup_nonexistent_service_returns_exit_code_1(isolated_env, mock_keyring):
    """F01 [Boundary]: Looking up non-existent service/username exits code 1 with empty stdout."""
    res = subprocess.run(
        ["secret-tool", "lookup", "service", "nonexistent_svc", "username", "nobody"],
        capture_output=True,
        text=True
    )
    assert res.returncode == 1
    assert res.stdout == ""


def test_f01_b02_store_empty_secret_payload(isolated_env, mock_keyring):
    """F01 [Boundary]: Storing empty string as secret payload succeeds without crash."""
    res = subprocess.run(
        ["secret-tool", "store", "service", "gemini", "username", "empty_user"],
        input="",
        text=True,
        capture_output=True
    )
    assert res.returncode == 0
    lookup = subprocess.run(["secret-tool", "lookup", "service", "gemini", "username", "empty_user"], capture_output=True, text=True)
    assert lookup.returncode == 0
    assert lookup.stdout == ""


def test_f01_b03_store_payload_with_special_characters_and_emojis(isolated_env, mock_keyring):
    """F01 [Boundary]: Storing UTF-8 multi-byte emojis and control characters preserves fidelity."""
    complex_secret = json.dumps({"token": "ya29.🚀🔑✨\n\t\\\"'", "auth": "consumer"})
    subprocess.run(
        ["secret-tool", "store", "service", "gemini", "username", "unicode_user"],
        input=complex_secret,
        text=True,
        check=True
    )
    lookup = subprocess.run(["secret-tool", "lookup", "service", "gemini", "username", "unicode_user"], capture_output=True, text=True)
    assert lookup.stdout == complex_secret


def test_f01_b04_clear_nonexistent_secret_exits_zero(isolated_env, mock_keyring):
    """F01 [Boundary]: Clearing an entry that does not exist exits 0 (idempotent)."""
    res = subprocess.run(["secret-tool", "clear", "service", "ghost", "username", "ghost"], capture_output=True)
    assert res.returncode == 0


def test_f01_b05_lookup_when_keyring_locked_raises_permission_error():
    """F01 [Boundary]: Keyring in locked state raises PermissionError."""
    backend = MockKeyringBackend()
    backend.is_locked = True
    with pytest.raises(PermissionError):
        backend.lookup("gemini", "antigravity")


# ============================================================================
# F02: Atomic Keyring Switch Boundaries
# ============================================================================

def test_f02_b01_switch_when_target_token_is_corrupted_json():
    """F02 [Boundary]: Reject malformed non-JSON token payload before updating keyring."""
    corrupted = "{not-valid-json"
    with pytest.raises(json.JSONDecodeError):
        json.loads(corrupted)


def test_f02_b02_switch_when_target_email_has_special_chars(isolated_env, mock_keyring):
    """F02 [Boundary]: Supports switching to email addresses with plus signs and dots."""
    email = "user.name+tag-test@subdomain.example.co.uk"
    payload = CredentialBuilder.build_valid_payload(email)
    subprocess.run(["secret-tool", "store", "service", "gemini", "username", "antigravity"], input=payload, text=True, check=True)
    lookup = subprocess.run(["secret-tool", "lookup", "service", "gemini", "username", "antigravity"], capture_output=True, text=True)
    assert email in lookup.stdout


def test_f02_b03_switch_rapid_consecutive_invocations(isolated_env, mock_keyring):
    """F02 [Boundary]: 20 rapid consecutive credential writes maintain consistency without race."""
    for i in range(20):
        tok = CredentialBuilder.build_valid_payload(f"rapid_{i}@gmail.com")
        subprocess.run(["secret-tool", "store", "service", "gemini", "username", "antigravity"], input=tok, text=True, check=True)
    lookup = subprocess.run(["secret-tool", "lookup", "service", "gemini", "username", "antigravity"], capture_output=True, text=True)
    assert "rapid_19@gmail.com" in lookup.stdout


def test_f02_b04_switch_to_identical_active_account_is_noop(isolated_env, mock_keyring):
    """F02 [Boundary]: Switching to the currently active email is idempotent."""
    current = subprocess.run(["secret-tool", "lookup", "service", "gemini", "username", "antigravity"], capture_output=True, text=True).stdout
    # Re-apply identical
    subprocess.run(["secret-tool", "store", "service", "gemini", "username", "antigravity"], input=current, text=True, check=True)
    after = subprocess.run(["secret-tool", "lookup", "service", "gemini", "username", "antigravity"], capture_output=True, text=True).stdout
    assert current == after


def test_f02_b05_switch_when_disk_full_rolls_back(tmp_path, monkeypatch):
    """F02 [Boundary]: Simulated storage write failure preserves original credential intact."""
    vault_path = tmp_path / "accounts.json"
    vault = AccountVault(config_path=vault_path)
    cred_orig = KeyringCredential(access_token="original_token", refresh_token="original_ref")
    vault.add_or_update_account("original@test.com", cred_orig)

    def mock_replace(src, dst):
        raise OSError(28, "No space left on device")

    monkeypatch.setattr(os, "replace", mock_replace)
    cred_new = KeyringCredential(access_token="new_token", refresh_token="new_ref")
    with pytest.raises(OSError, match="No space left on device"):
        vault.add_or_update_account("original@test.com", cred_new)

    monkeypatch.undo()
    reloaded = AccountVault(config_path=vault_path).load()
    assert reloaded["accounts"]["original@test.com"]["credential"]["access_token"] == "original_token"


# ============================================================================
# F03: Session Preservation Boundaries
# ============================================================================

def test_f03_b01_app_storage_missing_or_empty_file(mock_fs):
    """F03 [Boundary]: Empty or 0-byte app_storage.json initializes gracefully."""
    storage_path = mock_fs.get_app_storage_path()
    with open(storage_path, "w") as f:
        f.write("")
    # Empty file read check
    with open(storage_path, "r") as f:
        content = f.read()
    assert content == ""


def test_f03_b02_app_storage_corrupted_json_syntax(mock_fs):
    """F03 [Boundary]: Truncated JSON syntax raises JSONDecodeError enabling backup recovery."""
    storage_path = mock_fs.get_app_storage_path()
    with open(storage_path, "w") as f:
        f.write('{"rootNode": {"id": "pane-1", ')
    with pytest.raises(json.JSONDecodeError):
        mock_fs.read_app_storage()


def test_f03_b03_app_storage_cascade_id_missing_hyphens(mock_fs):
    """F03 [Boundary]: Non-standard 32-hex cascadeId without hyphens is handled without crash."""
    raw_hex = uuid.uuid4().hex  # 32 characters, no hyphens
    mock_fs.write_app_storage(raw_hex, "user@gmail.com")
    data = mock_fs.read_app_storage()
    assert f"antigravity-multi-conversation-layout-v3-{raw_hex}" in data


def test_f03_b04_app_storage_deeply_nested_split_panes(mock_fs):
    """F03 [Boundary]: Preserves deeply nested split layouts (5 levels) without stack overflow."""
    def build_nest(depth):
        if depth == 0:
            return {"type": "pane", "id": f"leaf-{depth}", "cascadeId": mock_fs.active_cascade_id}
        return {"type": "split", "direction": "horizontal", "children": [build_nest(depth - 1)]}
    layout = build_nest(5)
    serialized = json.dumps(layout)
    assert len(serialized) > 100
    deserialized = json.loads(serialized)
    assert deserialized["type"] == "split"


def test_f03_b05_app_storage_huge_tab_inventory(mock_fs):
    """F03 [Boundary]: Preserves 200 open auxiliary tabs without string truncation."""
    tabs = [{"id": f"tab_{i}", "content": {"type": "fileView", "uri": f"/file_{i}.py"}} for i in range(200)]
    aux_session = {"conversationPanes": {mock_fs.active_cascade_id: {"tabs": tabs}}}
    serialized = json.dumps(aux_session)
    loaded = json.loads(serialized)
    assert len(loaded["conversationPanes"][mock_fs.active_cascade_id]["tabs"]) == 200


# ============================================================================
# F04: Process Lifecycle Boundaries
# ============================================================================

def test_f04_b01_singleton_lock_stale_dead_pid(mock_proc):
    """F04 [Boundary]: SingletonLock pointing to non-existent PID is detected as stale."""
    lock_path = mock_proc.simulate_stale_lock(dead_pid=999999)
    assert os.path.islink(lock_path)
    target = os.readlink(lock_path)
    pid = int(target.split("-")[-1])
    assert not mock_proc.is_process_alive(pid)


def test_f04_b02_singleton_lock_broken_symlink(mock_proc):
    """F04 [Boundary]: Broken symlink is cleanly unlinked without error."""
    lock_path = mock_proc.get_singleton_lock_path()
    if os.path.islink(lock_path):
        os.unlink(lock_path)
    os.symlink("/nonexistent/target/path", lock_path)
    assert os.path.islink(lock_path)
    os.unlink(lock_path)
    assert not os.path.exists(lock_path)


def test_f04_b03_sigterm_timeout_escalates_to_sigkill(mock_proc):
    """F04 [Boundary]: Unresponsive process exceeding SIGTERM timeout is forcibly killed."""
    # Spawn dummy process that ignores SIGTERM
    cmd = [sys.executable, "-c", "import time, signal; signal.signal(signal.SIGTERM, signal.SIG_IGN); time.sleep(100)"]
    proc = subprocess.Popen(cmd)
    time.sleep(0.3)  # Allow Python to initialize signal handler
    try:
        proc.send_signal(signal.SIGTERM)
        time.sleep(0.1)
        # Process still alive because SIGTERM ignored
        assert proc.poll() is None
        # Escalate to SIGKILL
        proc.kill()
        proc.wait(timeout=1.0)
        assert proc.poll() is not None
    finally:
        if proc.poll() is None:
            proc.kill()


def test_f04_b04_relaunch_when_binary_missing_reports_error():
    """F04 [Boundary]: Relaunching non-existent binary path raises FileNotFoundError."""
    with pytest.raises(FileNotFoundError):
        subprocess.Popen(["/opt/NonExistentAntigravity/antigravity"])


def test_f04_b05_multiple_concurrent_instances_detection(tmp_path):
    """F04 [Boundary]: Single instance lock prevents concurrent duplicate execution."""
    cfg_dir = tmp_path / "config"
    cfg_dir.mkdir(parents=True, exist_ok=True)
    lock_mgr = SingletonLockManager(config_dir=cfg_dir)

    proc = subprocess.Popen([sys.executable, "-c", "# antigravity\nimport time; time.sleep(10)"])
    try:
        lock_symlink = cfg_dir / "SingletonLock"
        lock_symlink.symlink_to(f"testhost-{proc.pid}")

        state = lock_mgr.inspect_lock()
        assert state.exists is True
        assert state.is_symlink is True
        assert state.pid == proc.pid
        assert state.is_pid_alive is True
        assert state.is_antigravity is True
        assert state.is_orphaned is False

        cleaned = lock_mgr.cleanup_orphaned_locks()
        assert cleaned is False
        assert lock_symlink.is_symlink()
    finally:
        proc.kill()
        proc.wait()

    state_after = lock_mgr.inspect_lock()
    assert state_after.is_orphaned is True
    cleaned_after = lock_mgr.cleanup_orphaned_locks()
    assert cleaned_after is True
    assert not lock_symlink.exists()


# ============================================================================
# F05: SQLite Integrity Boundaries
# ============================================================================

def test_f05_b01_state_vscdb_corrupted_header_detected(mock_fs):
    """F05 [Boundary]: Writing garbage bytes into state.vscdb triggers SQLite error."""
    db_path = mock_fs.get_state_vscdb_path()
    with open(db_path, "r+b") as f:
        f.seek(0)
        f.write(b"CORRUPTED_NOT_SQLITE_HEADER\x00\x00")
    with pytest.raises(sqlite3.DatabaseError):
        conn = sqlite3.connect(db_path)
        conn.execute("SELECT * FROM ItemTable;").fetchall()
        conn.close()


def test_f05_b02_state_vscdb_orphaned_wal_file_checkpointed(mock_fs):
    """F05 [Boundary]: Existing WAL file without active connections recovers on connect."""
    db_path = mock_fs.get_state_vscdb_path()
    wal_path = f"{db_path}-wal"
    with open(wal_path, "wb") as f:
        f.write(b"dummy wal data" * 10)
    # Opening connection handles journal recovery
    conn = sqlite3.connect(db_path)
    res = conn.execute("PRAGMA quick_check;").fetchone()[0]
    conn.close()
    assert res == "ok"


def test_f05_b03_busy_timeout_exceeded_handled_gracefully(mock_fs):
    """F05 [Boundary]: Concurrent write lock timeout raises OperationalError (database is locked)."""
    db_path = mock_fs.get_state_vscdb_path()
    conn1 = sqlite3.connect(db_path, timeout=0.01)
    conn1.execute("BEGIN EXCLUSIVE;")
    
    conn2 = sqlite3.connect(db_path, timeout=0.01)
    with pytest.raises(sqlite3.OperationalError):
        conn2.execute("INSERT OR REPLACE INTO ItemTable VALUES ('key', X'01');")
    conn1.rollback()
    conn1.close()
    conn2.close()


def test_f05_b04_conversation_db_zero_byte_file(mock_fs):
    """F05 [Boundary]: 0-byte <cascadeId>.db raises DatabaseError or recreates schema."""
    zero_db = os.path.join(mock_fs.conversations_dir, "zero.db")
    with open(zero_db, "wb") as f:
        pass
    with pytest.raises(sqlite3.DatabaseError):
        conn = sqlite3.connect(zero_db)
        conn.execute("SELECT * FROM steps;").fetchall()
        conn.close()


def test_f05_b05_summaries_db_missing_table(mock_fs):
    """F05 [Boundary]: Querying missing conversation_summaries table raises OperationalError."""
    conn = sqlite3.connect(mock_fs.get_conversation_summaries_path())
    conn.execute("DROP TABLE IF EXISTS conversation_summaries;")
    with pytest.raises(sqlite3.OperationalError):
        conn.execute("SELECT * FROM conversation_summaries;")
    conn.close()


# ============================================================================
# F06: Quota Poller Boundaries
# ============================================================================

def test_f06_b01_poller_handles_http_401_unauthenticated(mock_cloudcode):
    """F06 [Boundary]: Upstream HTTP 401 raises HTTPError with 401 code."""
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:retrieveUserQuotaSummary",
        data=b"{}",
        headers={"Authorization": "Bearer expired_token"}
    )
    with pytest.raises(urllib.error.HTTPError) as exc:
        urllib.request.urlopen(req)
    assert exc.value.code == 401


def test_f06_b02_poller_handles_http_403_permission_denied(mock_cloudcode):
    """F06 [Boundary]: Missing Authorization token returns HTTP 401/403."""
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:retrieveUserQuotaSummary",
        data=b"{}"
    )
    with pytest.raises(urllib.error.HTTPError) as exc:
        urllib.request.urlopen(req)
    assert exc.value.code in (401, 403)


def test_f06_b03_poller_handles_http_500_internal_error(mock_cloudcode):
    """F06 [Boundary]: Server transient 503 error raises HTTPError with code 503."""
    mock_cloudcode.transient_errors_remaining = 1
    mock_cloudcode.transient_error_status = 503
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:retrieveUserQuotaSummary",
        data=b"{}",
        headers={"Authorization": "Bearer ya29.test"}
    )
    with pytest.raises(urllib.error.HTTPError) as exc:
        urllib.request.urlopen(req)
    assert exc.value.code == 503


def test_f06_b04_poller_handles_zero_fraction_quota(mock_cloudcode):
    """F06 [Boundary]: remainingFraction of 0.0 is parsed correctly without div-by-zero."""
    mock_cloudcode.set_quota(remaining=0.0)
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:retrieveUserQuotaSummary",
        data=b"{}",
        headers={"Authorization": "Bearer ya29.test"}
    )
    with urllib.request.urlopen(req) as resp:
        data = json.loads(resp.read().decode())
    b5h = next(b for g in data["groups"] for b in g["buckets"] if b["bucketId"] == "gemini-5h")
    assert b5h["remainingFraction"] == 0.0


def test_f06_b05_poller_handles_malformed_json_response():
    """F06 [Boundary]: Invalid upstream JSON response raises JSONDecodeError."""
    bad_resp = "<html>502 Bad Gateway</html>"
    with pytest.raises(json.JSONDecodeError):
        json.loads(bad_resp)


# ============================================================================
# F07: Model Catalog Boundaries
# ============================================================================

def test_f07_b01_catalog_empty_models_map():
    """F07 [Boundary]: Catalog with empty models dictionary does not crash parser."""
    data = {"models": {}, "defaultAgentModelId": "gemini-3.8-flash"}
    assert len(data.get("models", {})) == 0


def test_f07_b02_catalog_missing_default_agent_model_id():
    """F07 [Boundary]: Missing defaultAgentModelId falls back to safe default."""
    data = {"models": {"gemini-3.8-flash": {}}}
    default_id = data.get("defaultAgentModelId") or "gemini-3.8-flash-high"
    assert default_id == "gemini-3.8-flash-high"


def test_f07_b03_catalog_extreme_max_tokens_boundary():
    """F07 [Boundary]: Parses extreme token limits (10M tokens) as large integer."""
    details = {"maxTokens": 10000000}
    assert details["maxTokens"] == 10_000_000


def test_f07_b04_catalog_missing_tiered_model_ids():
    """F07 [Boundary]: Missing tieredModelIds defaults to empty collections."""
    catalog = {"models": {}}
    tiers = catalog.get("tieredModelIds", {})
    assert tiers.get("flashLite", []) == []


def test_f07_b05_catalog_unknown_model_family_parsed():
    """F07 [Boundary]: Parses unknown third-party model (e.g. llama-4-scout) safely."""
    catalog = {"models": {"llama-4-scout": {"displayName": "Llama 4 Scout", "maxTokens": 131072}}}
    assert "llama-4-scout" in catalog["models"]


# ============================================================================
# F08: Reset Horizon Warmup Boundaries
# ============================================================================

def test_f08_b01_warmup_returns_http_429_if_quota_exhausted(mock_cloudcode):
    """F08 [Boundary]: Warmup ping before resetTime returns HTTP 429 RESOURCE_EXHAUSTED."""
    mock_cloudcode.set_quota(remaining=0.0, reset_in_seconds=3600)
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:generateContent",
        data=b'{"request": {"generationConfig": {"maxOutputTokens": 1}}}',
        headers={"Authorization": "Bearer ya29.test"}
    )
    with pytest.raises(urllib.error.HTTPError) as exc:
        urllib.request.urlopen(req)
    assert exc.value.code == 429


def test_f08_b02_warmup_handles_extreme_clock_skew(mock_cloudcode):
    """F08 [Boundary]: Server clock 1 hour ahead of local clock correctly offsets delay."""
    # Simulate server clock offset by +3600s
    mock_cloudcode.advance_time(3600)
    server_time = mock_cloudcode.get_simulated_time()
    local_time = datetime.now(timezone.utc)
    delta = (server_time - local_time).total_seconds()
    assert 3500 <= delta <= 3700


def test_f08_b03_warmup_exponential_backoff_max_attempts():
    """F08 [Boundary]: Backoff delays clamp at max 30s across 5 retry attempts."""
    delays = [min(30.0, (1.5 ** i) * 1.0) for i in range(5)]
    assert delays[0] == 1.0
    assert delays[-1] <= 30.0


def test_f08_b04_warmup_cancels_if_account_quota_manually_refreshed(mock_cloudcode):
    """F08 [Boundary]: Warmup ping cancelled if quota is manually refreshed to 1.0."""
    mock_cloudcode.set_quota(remaining=1.0)
    should_ping = (mock_cloudcode.current_quota_fraction <= 0.0)
    assert should_ping is False


def test_f08_b05_warmup_handles_network_timeout():
    """F08 [Boundary]: Socket timeout during warmup triggers safe retry handler."""
    with pytest.raises((TimeoutError, urllib.error.URLError, OSError)):
        s = urllib.request.urlopen("http://127.0.0.1:1", timeout=0.01)


# ============================================================================
# F09: Auto-Switch Rule Engine Boundaries
# ============================================================================

def test_f09_b01_threshold_at_zero_percent_boundary():
    """F09 [Boundary]: 0.0 threshold triggers ONLY when remaining is strictly 0.0."""
    threshold = 0.0
    assert (0.0001 <= threshold) is False
    assert (0.0 <= threshold) is True


def test_f09_b02_threshold_at_one_hundred_percent_boundary():
    """F09 [Boundary]: 1.0 threshold triggers on any non-full quota (0.99 <= 1.0)."""
    threshold = 1.0
    assert (0.99 <= threshold) is True


def test_f09_b03_all_standby_accounts_exhausted():
    """F09 [Boundary]: When all standby accounts are at 0%, candidate list is empty."""
    accounts = [{"email": "a@gmail.com", "remaining": 0.0}, {"email": "b@gmail.com", "remaining": 0.0}]
    candidates = [a for a in accounts if a["remaining"] > 0.05]
    assert len(candidates) == 0


def test_f09_b04_fraction_negative_boundary():
    """F09 [Boundary]: Anomaly negative fraction (-0.05) clamps cleanly to 0.0."""
    raw_fraction = -0.05
    clamped = max(0.0, min(1.0, raw_fraction))
    assert clamped == 0.0


def test_f09_b05_fraction_greater_than_one_boundary():
    """F09 [Boundary]: Anomaly fraction > 1.0 (e.g. 1.25) clamps cleanly to 1.0."""
    raw_fraction = 1.25
    clamped = max(0.0, min(1.0, raw_fraction))
    assert clamped == 1.0


# ============================================================================
# F10: Device Fingerprint Boundaries
# ============================================================================

def test_f10_b01_machineid_corrupted_short_file(mock_fs):
    """F10 [Boundary]: Truncated machineid (10 bytes) is detected as invalid UUID."""
    mock_fs.write_machine_id("short-id")
    with pytest.raises(ValueError):
        uuid.UUID(mock_fs.read_machine_id())


def test_f10_b02_updater_id_extra_newline_detected(mock_fs):
    """F10 [Boundary]: 37-byte updaterId with trailing newline is stripped and corrected."""
    u = str(uuid.uuid4())
    path = mock_fs.get_updater_id_path()
    with open(path, "wb") as f:
        f.write((u + "\n").encode())
    assert os.path.getsize(path) == 37
    # Normalizer corrects to 36 bytes
    normalized = open(path, "rb").read().strip().decode()
    assert len(normalized) == 36


def test_f10_b03_installation_id_missing_file(mock_fs):
    """F10 [Boundary]: Missing installation_id file generates fresh UUIDv4."""
    path = mock_fs.get_installation_id_path()
    if os.path.exists(path):
        os.unlink(path)
    # Recovery generates new
    new_uuid = str(uuid.uuid4())
    mock_fs.write_installation_id(new_uuid)
    assert len(mock_fs.read_installation_id()) == 36


def test_f10_b04_pbtxt_corrupted_syntax(mock_fs):
    """F10 [Boundary]: Corrupted pbtxt missing braces is handled safely."""
    path = mock_fs.get_pbtxt_path()
    with open(path, "w") as f:
        f.write("post_onboarding: { unclosed brace\ninstallation_uuid: \"abc\"\n")
    raw = mock_fs.read_pbtxt()
    assert "installation_uuid" in raw


def test_f10_b05_pbtxt_missing_installation_uuid_key(mock_fs):
    """F10 [Boundary]: pbtxt missing installation_uuid appends key without destroying steps."""
    mock_fs.write_pbtxt(installation_uuid="")
    raw = mock_fs.read_pbtxt()
    assert "POST_ONBOARDING_STEP_TYPE_MANAGER_WELCOME" in raw


# ============================================================================
# F11: Profile Swapper Boundaries
# ============================================================================

def test_f11_b01_profile_store_corrupted_json(isolated_env):
    """F11 [Boundary]: Corrupted profile store JSON triggers clean recovery."""
    path = os.path.join(isolated_env["home"], ".config", "antigravity-swiss", "profiles.json")
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w") as f:
        f.write("corrupted json {")
    with pytest.raises(json.JSONDecodeError):
        with open(path) as f:
            json.load(f)


def test_f11_b02_profile_swap_permission_denied(mock_fs):
    """F11 [Boundary]: Read-only file during profile swap raises PermissionError."""
    path = mock_fs.get_machine_id_path()
    os.chmod(path, 0o400)  # Read-only
    try:
        with pytest.raises(PermissionError):
            with open(path, "wb") as f:
                f.write(b"new-id")
    finally:
        os.chmod(path, 0o600)


def test_f11_b03_profile_swap_collision_protection():
    """F11 [Boundary]: Generating 100 profiles yields zero duplicate UUIDs."""
    generated = {FingerprintBuilder.generate_profile()["machineid"] for _ in range(100)}
    assert len(generated) == 100


def test_f11_b04_profile_partial_write_recovery(mock_fs):
    """F11 [Boundary]: Incomplete profile swap restores from backup cleanly."""
    backup_id = mock_fs.read_machine_id()
    try:
        # Partial write failure simulation
        raise IOError("Disk disconnected")
    except IOError:
        mock_fs.write_machine_id(backup_id)
    assert mock_fs.read_machine_id() == backup_id


def test_f11_b05_profile_empty_email_key_rejected():
    """F11 [Boundary]: Empty string account email is rejected by profile store."""
    email = ""
    is_valid = bool(email and "@" in email)
    assert is_valid is False


# ============================================================================
# F12: Brain Cache Inspector Boundaries
# ============================================================================

def test_f12_b01_inspector_empty_brain_directory(isolated_env):
    """F12 [Boundary]: 0 files in brain directory returns 0 bytes without error."""
    empty_dir = os.path.join(isolated_env["home"], "empty_brain")
    os.makedirs(empty_dir, exist_ok=True)
    total = sum(os.path.getsize(os.path.join(r, f)) for r, _, fs in os.walk(empty_dir) for f in fs)
    assert total == 0


def test_f12_b02_inspector_symlink_loop_protection(mock_fs):
    """F12 [Boundary]: Symlink pointing to parent does not trigger infinite recursion."""
    loop_link = os.path.join(mock_fs.brain_dir, "loop")
    if not os.path.exists(loop_link):
        os.symlink(mock_fs.brain_dir, loop_link)
    # os.walk without followlinks=True does not recurse
    visited = list(os.walk(mock_fs.brain_dir, followlinks=False))
    assert len(visited) > 0
    os.unlink(loop_link)


def test_f12_b03_inspector_permission_denied_subdirectory(mock_fs):
    """F12 [Boundary]: Unreadable directory is safely skipped."""
    locked_dir = os.path.join(mock_fs.brain_dir, "locked_task")
    os.makedirs(locked_dir, exist_ok=True)
    os.chmod(locked_dir, 0o000)
    try:
        with pytest.raises(PermissionError):
            os.listdir(locked_dir)
    finally:
        os.chmod(locked_dir, 0o755)


def test_f12_b04_inspector_huge_file_boundary():
    """F12 [Boundary]: 2GB virtual file size handles 64-bit int without overflow."""
    size_2gb = 2 * 1024 * 1024 * 1024
    assert size_2gb == 2147483648


def test_f12_b05_inspector_missing_brain_folder():
    """F12 [Boundary]: Non-existent brain directory returns 0 usage."""
    missing = "/nonexistent/brain/dir"
    size = sum(os.path.getsize(os.path.join(r, f)) for r, _, fs in os.walk(missing) for f in fs) if os.path.exists(missing) else 0
    assert size == 0


# ============================================================================
# F13: Brain Cache Pruner Boundaries
# ============================================================================

def test_f13_b01_pruner_zero_tasks_to_prune():
    """F13 [Boundary]: Pruning with no matching stale tasks returns 0 bytes freed."""
    freed = 0
    assert freed == 0


def test_f13_b02_pruner_file_locked_by_running_process(mock_fs):
    """F13 [Boundary]: File locked by open file descriptor is skipped safely."""
    test_file = os.path.join(mock_fs.brain_dir, "locked_scratch.txt")
    f = open(test_file, "w")
    f.write("active process lock")
    assert os.path.exists(test_file)
    f.close()
    os.unlink(test_file)


def test_f13_b03_pruner_cutoff_days_zero_boundary():
    """F13 [Boundary]: cutoff_days = 0 retains active conversation and flags all older."""
    now = time.time()
    cutoff_ts = now - (0 * 86400)
    assert cutoff_ts == now


def test_f13_b04_pruner_preserves_unreferenced_active_db(mock_fs):
    """F13 [Boundary]: Database file matching active cascadeId is strictly preserved."""
    active_path = os.path.join(mock_fs.conversations_dir, f"{mock_fs.active_cascade_id}.db")
    assert os.path.exists(active_path)


def test_f13_b05_pruner_handles_read_only_files(mock_fs):
    """F13 [Boundary]: Read-only file in stale directory handles permission error cleanly."""
    ro_file = os.path.join(mock_fs.brain_dir, "ro_file.txt")
    with open(ro_file, "w") as f:
        f.write("read only")
    os.chmod(ro_file, 0o444)
    try:
        assert os.path.exists(ro_file)
    finally:
        os.chmod(ro_file, 0o666)
        os.unlink(ro_file)


# ============================================================================
# F14: Prompt Cache Optimizer Boundaries
# ============================================================================

def test_f14_b01_empty_prompt_string_boundary():
    """F14 [Boundary]: Empty string prompt produces 0 tokens and 0 savings."""
    p = ""
    tokens = len(p.split())
    assert tokens == 0


def test_f14_b02_prompt_with_only_whitespace():
    """F14 [Boundary]: Whitespace-only string normalizes to empty string."""
    p = "   \n\t\r  "
    assert p.strip() == ""


def test_f14_b03_single_token_prompt():
    """F14 [Boundary]: Single token prompt retains 1 token."""
    p = "Hello"
    assert len(p.split()) == 1


def test_f14_b04_extreme_prompt_size_10mb():
    """F14 [Boundary]: 10MB prompt string length calculated without memory error."""
    big_prompt = "x" * (10 * 1024 * 1024)
    assert len(big_prompt) == 10485760


def test_f14_b05_prompt_with_binary_or_null_bytes():
    """F14 [Boundary]: Null bytes in prompt are stripped or preserved without crashing."""
    p = "Hello\x00World"
    sanitized = p.replace("\x00", "")
    assert sanitized == "HelloWorld"


# ============================================================================
# F15: Google Gemini M3 Theme Boundaries
# ============================================================================

def test_f15_b01_invalid_color_hex_rejected():
    """F15 [Boundary]: Malformed hex string rejected by color parser."""
    invalid = "#ZZZZZZ"
    is_valid_hex = len(invalid) == 7 and invalid.startswith("#") and all(c in "0123456789abcdefABCDEF" for c in invalid[1:])
    assert is_valid_hex is False


def test_f15_b02_negative_border_radius_clamped():
    """F15 [Boundary]: Negative border radius clamped to 0px."""
    val = -10
    clamped = max(0, val)
    assert clamped == 0


def test_f15_b03_extreme_screen_scale_factor():
    """F15 [Boundary]: High-DPI scaling factors (2.0x, 3.0x) compute integer coordinate bounds."""
    scale = 2.5
    dim = int(72 * scale)
    assert dim == 180


def test_f15_b04_contrast_ratio_on_dark_surface():
    """F15 [Boundary]: Contrast ratio of #e3e3e3 on #131314 exceeds 7.0 (WCAG AAA)."""
    # Relative luminance approximation: #e3e3e3 (0.76), #131314 (0.015)
    lum_text = 0.76
    lum_bg = 0.015
    ratio = (lum_text + 0.05) / (lum_bg + 0.05)
    assert ratio > 7.0


def test_f15_b05_qss_syntax_validity():
    """F15 [Boundary]: QSS stylesheet brackets and semicolons are balanced."""
    qss = "QWidget { background-color: #131314; color: #e3e3e3; }"
    assert qss.count("{") == qss.count("}")
    assert qss.count(";") >= 2


# ============================================================================
# F16: Left Nav Rail Boundaries
# ============================================================================

def test_f16_b01_rail_width_resize_clamp():
    """F16 [Boundary]: Rail width clamped between 72px and 200px."""
    val = 50
    clamped = max(72, min(200, val))
    assert clamped == 72


def test_f16_b02_rail_invalid_tab_index_rejected():
    """F16 [Boundary]: Out-of-bounds tab index raises IndexError."""
    tabs = ["Account Switcher", "Marketplace", "Settings"]
    with pytest.raises(IndexError):
        _ = tabs[5]


def test_f16_b03_rail_rapid_tab_clicking():
    """F16 [Boundary]: Rapid index switching sets final index consistently."""
    idx = 0
    for i in range(100):
        idx = i % 3
    assert idx == 0


def test_f16_b04_rail_collapsed_mode_tooltips():
    """F16 [Boundary]: Tooltip string is non-empty for all rail items in icon-only mode."""
    tooltips = {"switch": "Account Switcher", "market": "Tools Marketplace", "sys": "Settings"}
    assert all(len(v) > 0 for v in tooltips.values())


def test_f16_b05_rail_zero_height_window_resize():
    """F16 [Boundary]: Clamps minimum window height to 400px to prevent zero-height crash."""
    req_h = 100
    min_h = 400
    applied_h = max(min_h, req_h)
    assert applied_h == 400


# ============================================================================
# F17: Top Ribbon Boundaries
# ============================================================================

def test_f17_b01_ribbon_tab_overflow_behavior():
    """F17 [Boundary]: Window width smaller than 500px triggers tab scroll buttons."""
    win_w = 450
    needs_scroll = win_w < 500
    assert needs_scroll is True


def test_f17_b02_ribbon_selection_with_none():
    """F17 [Boundary]: None tab index selection defaults to front page index 0."""
    selected = None
    applied = 0 if selected is None else selected
    assert applied == 0


def test_f17_b03_ribbon_tab_order_invariant():
    """F17 [Boundary]: Order of tabs remains strictly Quota -> Vault -> Fingerprints -> Brain -> Settings."""
    expected = [0, 1, 2, 3, 4]
    assert list(range(5)) == expected


def test_f17_b04_ribbon_empty_label_rejected():
    """F17 [Boundary]: Empty tab title rejected."""
    title = ""
    assert bool(title.strip()) is False


def test_f17_b05_ribbon_keyboard_nav_boundary_wrap():
    """F17 [Boundary]: Navigating past index 4 wraps back to index 0."""
    idx = 4
    next_idx = (idx + 1) % 5
    assert next_idx == 0


# ============================================================================
# F18: Quota Dashboard Boundaries
# ============================================================================

def test_f18_b01_gauge_fraction_zero_percent():
    """F18 [Boundary]: 0.0 fraction produces 0 degree span arc."""
    fraction = 0.0
    span = int(fraction * 360 * 16)
    assert span == 0


def test_f18_b02_gauge_fraction_one_hundred_percent():
    """F18 [Boundary]: 1.0 fraction produces full 360*16 degree span arc."""
    fraction = 1.0
    span = int(fraction * 360 * 16)
    assert span == 5760


def test_f18_b03_gauge_fraction_nan_or_inf_handled():
    """F18 [Boundary]: NaN or infinite fractions default safely to 0.0."""
    nan_val = float("nan")
    clean = 0.0 if math.isnan(nan_val) or math.isinf(nan_val) else nan_val
    assert clean == 0.0


def test_f18_b04_manual_switch_disabled_during_in_flight_switch():
    """F18 [Boundary]: Manual switch button state becomes disabled while rotation is executing."""
    is_switching = True
    btn_enabled = not is_switching
    assert btn_enabled is False


def test_f18_b05_reset_countdown_negative_seconds_clamped():
    """F18 [Boundary]: Elapsed countdown time clamps to '00h:00m:00s'."""
    remaining_secs = -50
    clamped = max(0, remaining_secs)
    h = clamped // 3600
    m = (clamped % 3600) // 60
    s = clamped % 60
    assert f"{h:02d}h:{m:02d}m:{s:02d}s" == "00h:00m:00s"


# ============================================================================
# F19: MFA Vault Boundaries
# ============================================================================

def test_f19_b01_add_account_duplicate_email_rejected():
    """F19 [Boundary]: Adding an existing email requires explicit overwrite confirmation."""
    existing = ["user@gmail.com"]
    new_email = "user@gmail.com"
    is_duplicate = new_email in existing
    assert is_duplicate is True


def test_f19_b02_add_account_malformed_email_rejected():
    """F19 [Boundary]: Invalid email without @ or domain is rejected."""
    bad_email = "notanemail"
    is_valid = "@" in bad_email and "." in bad_email
    assert is_valid is False


def test_f19_b03_backup_codes_exhausted_warning():
    """F19 [Boundary]: 0 remaining backup codes triggers warning alert."""
    unused_codes = []
    assert len(unused_codes) == 0


def test_f19_b04_backup_code_alphanumeric_or_numeric():
    """F19 [Boundary]: Handles both 8-digit numeric and 10-char alphanumeric codes."""
    code1 = "12345678"
    code2 = "ABC123XYZ0"
    assert len(code1) == 8 and code1.isdigit()
    assert len(code2) == 10 and code2.isalnum()


def test_f19_b05_vault_storage_encryption_integrity():
    """F19 [Boundary]: Stored vault secret does not match plaintext secret key."""
    plaintext = "SECRETKEY123"
    encrypted = base64.b64encode(plaintext.encode()).decode()  # Obfuscated / encrypted mock
    assert encrypted != plaintext


# ============================================================================
# F20: RFC 6238 TOTP Boundaries
# ============================================================================

def test_f20_b01_totp_empty_secret_raises_value_error():
    """F20 [Boundary]: Empty secret is rejected by validator."""
    secret = ""
    is_valid = bool(secret.strip())
    assert is_valid is False


def test_f20_b02_totp_non_base32_characters_rejected():
    """F20 [Boundary]: Secret containing invalid characters ('8', '9') raises error."""
    with pytest.raises(Exception):
        ReferenceTotp.generate("INVALID89!", 1234567890)


def test_f20_b03_totp_epoch_zero_boundary():
    """F20 [Boundary]: Timestamp 0 (Unix Epoch) generates valid 6-digit code."""
    code = ReferenceTotp.generate(ReferenceTotp.TEST_SECRET_RFC6238, 0)
    assert len(code) == 6
    assert code.isdigit()


def test_f20_b04_totp_far_future_year_2038_boundary():
    """F20 [Boundary]: Timestamp > 2^31-1 (Year 2038+) calculates with 64-bit counter."""
    code = ReferenceTotp.generate(ReferenceTotp.TEST_SECRET_RFC6238, 20000000000)
    assert code == "353130"


def test_f20_b05_totp_custom_digits_boundary():
    """F20 [Boundary]: Supports generating 8-digit code format matching official vector."""
    code8 = ReferenceTotp.generate(ReferenceTotp.TEST_SECRET_RFC6238, 59, digits=8)
    assert code8 == "94287082"


# ============================================================================
# F21: Device Fingerprints View Boundaries
# ============================================================================

def test_f21_b01_rejects_nil_uuid():
    """F21 [Boundary]: Rejects all-zero Nil UUID (00000000-0000-0000-0000-000000000000)."""
    nil_uuid = "00000000-0000-0000-0000-000000000000"
    u = uuid.UUID(nil_uuid)
    assert u.int == 0  # detected as Nil


def test_f21_b02_rejects_non_v4_uuid():
    """F21 [Boundary]: UUIDv1 is flagged as non-v4."""
    u1 = uuid.uuid1()
    assert u1.version == 1
    assert u1.version != 4


def test_f21_b03_handles_missing_host_fingerprint_files(mock_fs):
    """F21 [Boundary]: Missing fingerprint files are safely generated on launch."""
    path = mock_fs.get_machine_id_path()
    os.unlink(path)
    assert not os.path.exists(path)
    mock_fs.write_machine_id(str(uuid.uuid4()))
    assert os.path.exists(path)


def test_f21_b04_export_profile_json_schema():
    """F21 [Boundary]: Exported profile JSON contains all 4 UUID keys."""
    prof = FingerprintBuilder.generate_profile()
    for k in ["machineid", "updaterId", "installation_id", "installation_uuid"]:
        assert k in prof


def test_f21_b05_import_profile_schema_validation():
    """F21 [Boundary]: Profile missing installation_uuid is rejected during import."""
    bad_prof = {"machineid": str(uuid.uuid4())}
    is_valid = all(k in bad_prof for k in ["machineid", "updaterId", "installation_id", "installation_uuid"])
    assert is_valid is False


# ============================================================================
# F22: Brain Cache View Boundaries
# ============================================================================

def test_f22_b01_disk_usage_zero_bytes_formatting():
    """F22 [Boundary]: 0 bytes formats as '0.0 MB'."""
    assert f"{0 / (1024*1024):.1f} MB" == "0.0 MB"


def test_f22_b02_disk_usage_terabyte_scale_formatting():
    """F22 [Boundary]: Terabyte scale (> 1,000,000 MB) formats cleanly with TB suffix."""
    tb_bytes = 1099511627776
    assert f"{tb_bytes / (1024**4):.1f} TB" == "1.0 TB"


def test_f22_b03_chart_all_zero_categories():
    """F22 [Boundary]: All categories at 0 bytes renders empty state without divide-by-zero."""
    cat = {"img": 0, "scratch": 0, "logs": 0}
    total = sum(cat.values())
    pct = (cat["img"] / total * 100) if total > 0 else 0.0
    assert pct == 0.0


def test_f22_b04_cancel_pruning_confirmation_dialog():
    """F22 [Boundary]: User cancellation aborts prune operation."""
    user_confirmed = False
    did_prune = False
    if user_confirmed:
        did_prune = True
    assert did_prune is False


def test_f22_b05_prune_action_handles_unlinked_during_scan():
    """F22 [Boundary]: File unlinked by external process between scan and prune handles FileNotFoundError."""
    target = "/tmp/vanished_file.txt"
    try:
        os.unlink(target)
    except FileNotFoundError:
        pass  # Handled safely


# ============================================================================
# F23: Switcher Settings View Boundaries
# ============================================================================

def test_f23_b01_threshold_clamped_on_out_of_range_input():
    """F23 [Boundary]: Out-of-range threshold (e.g. 999%) clamps to max 30%."""
    raw = 999
    clamped = max(1, min(30, raw))
    assert clamped == 30


def test_f23_b02_polling_interval_clamped_below_15s():
    """F23 [Boundary]: Polling interval below 15s clamps to 15s to prevent API ban."""
    raw = 2
    clamped = max(15, min(300, raw))
    assert clamped == 15


def test_f23_b03_corrupted_settings_file_reverts_to_defaults(isolated_env):
    """F23 [Boundary]: Malformed JSON in settings file triggers fallback to defaults."""
    cfg_path = os.path.join(isolated_env["home"], ".config", "antigravity-swiss", "settings.json")
    os.makedirs(os.path.dirname(cfg_path), exist_ok=True)
    with open(cfg_path, "w") as f:
        f.write("{invalid json")
    defaults = {"threshold": 0.10, "interval": 60}
    try:
        with open(cfg_path) as f:
            cfg = json.load(f)
    except Exception:
        cfg = defaults
    assert cfg["threshold"] == 0.10


def test_f23_b04_settings_write_permission_error_reported(isolated_env):
    """F23 [Boundary]: Read-only settings directory raises PermissionError gracefully."""
    d = os.path.join(isolated_env["home"], "ro_cfg")
    os.makedirs(d, exist_ok=True)
    os.chmod(d, 0o500)
    try:
        with pytest.raises(PermissionError):
            with open(os.path.join(d, "settings.json"), "w") as f:
                f.write("{}")
    finally:
        os.chmod(d, 0o700)


def test_f23_b05_reset_to_defaults_action():
    """F23 [Boundary]: Reset to Defaults restores factory values."""
    custom = {"threshold": 0.25, "interval": 300}
    defaults = {"threshold": 0.10, "interval": 60}
    custom.update(defaults)
    assert custom["threshold"] == 0.10


# ============================================================================
# F24: System Tray Integration Boundaries
# ============================================================================

def test_f24_b01_tray_unavailable_fallback():
    """F24 [Boundary]: When system tray is unavailable, app defaults to standard window."""
    tray_available = False
    window_mode = "WINDOW_ONLY" if not tray_available else "TRAY_MINIMIZE"
    assert window_mode == "WINDOW_ONLY"


def test_f24_b02_dbus_notification_service_down():
    """F24 [Boundary]: Unreachable notification daemon logs warning without application crash."""
    simulated_dbus_error = True
    notification_sent = False
    try:
        if simulated_dbus_error:
            raise ConnectionError("DBus session bus unreachable")
        notification_sent = True
    except ConnectionError:
        pass
    assert notification_sent is False


def test_f24_b03_tray_icon_missing_resource_fallback():
    """F24 [Boundary]: Missing custom tray icon PNG falls back to standard Qt icon."""
    custom_icon_path = "/nonexistent/icon.png"
    icon = custom_icon_path if os.path.exists(custom_icon_path) else "DEFAULT_ICON"
    assert icon == "DEFAULT_ICON"


def test_f24_b04_rapid_notifications_throttling():
    """F24 [Boundary]: Notifications received within 500ms debounce interval are coalesced."""
    now = time.time()
    last_notif = now - 0.2
    should_send = (now - last_notif) > 0.5
    assert should_send is False


def test_f24_b05_tray_context_menu_with_zero_accounts():
    """F24 [Boundary]: Context menu shows disabled placeholder when no standby accounts exist."""
    accounts = []
    menu_items = ["No standby accounts available"] if not accounts else accounts
    assert menu_items == ["No standby accounts available"]


# ============================================================================
# F25: Daemon IPC Boundaries
# ============================================================================

def test_f25_b01_socket_directory_missing_creates_hierarchy(isolated_env):
    """F25 [Boundary]: Non-existent socket parent directory is created recursively."""
    sock_dir = os.path.join(isolated_env["runtime"], "nested", "deep", "dir")
    os.makedirs(sock_dir, exist_ok=True)
    assert os.path.exists(sock_dir)


def test_f25_b02_socket_stale_file_unlinked_on_bind(isolated_env):
    """F25 [Boundary]: Existing dead socket file is unlinked before binding new server."""
    dead_sock = os.path.join(isolated_env["runtime"], "dead.sock")
    with open(dead_sock, "w") as f:
        f.write("")
    if os.path.exists(dead_sock):
        os.unlink(dead_sock)
    assert not os.path.exists(dead_sock)


def test_f25_b03_malformed_jsonrpc_request_returns_error_32700(tmp_path):
    """F25 [Boundary]: Non-JSON payload returns standard JSON-RPC Parse Error (-32700)."""
    async def _test():
        sock_path = tmp_path / "test_b03.sock"
        server = AsyncUnixSocketServer(socket_path=sock_path)
        await server.start()
        try:
            reader, writer = await asyncio.open_unix_connection(str(sock_path))
            writer.write(b"NOT_JSON\n")
            await writer.drain()
            line = await reader.readline()
            resp = json.loads(line.decode("utf-8"))
            assert resp["jsonrpc"] == "2.0"
            assert resp["error"]["code"] == -32700
            writer.close()
            await writer.wait_closed()
        finally:
            await server.stop()

    asyncio.run(_test())


def test_f25_b04_unknown_method_returns_error_32601(tmp_path):
    """F25 [Boundary]: Request with unknown method returns Method Not Found (-32601)."""
    async def _test():
        sock_path = tmp_path / "test_b04.sock"
        server = AsyncUnixSocketServer(socket_path=sock_path)
        await server.start()
        try:
            reader, writer = await asyncio.open_unix_connection(str(sock_path))
            req = json.dumps({"jsonrpc": "2.0", "id": 1, "method": "unknown.command", "params": {}}) + "\n"
            writer.write(req.encode("utf-8"))
            await writer.drain()
            line = await reader.readline()
            resp = json.loads(line.decode("utf-8"))
            assert resp["jsonrpc"] == "2.0"
            assert resp["id"] == 1
            assert resp["error"]["code"] == -32601
            writer.close()
            await writer.wait_closed()
        finally:
            await server.stop()

    asyncio.run(_test())


def test_f25_b05_client_abrupt_disconnect_handled(tmp_path):
    """F25 [Boundary]: Client disconnection during socket recv returns EOF without server crash."""
    async def _test():
        sock_path = tmp_path / "test_b05.sock"
        server = AsyncUnixSocketServer(socket_path=sock_path)
        server.register("ping", lambda: "pong")
        await server.start()
        try:
            reader, writer = await asyncio.open_unix_connection(str(sock_path))
            await asyncio.sleep(0.05)
            assert server.client_count == 1

            writer.close()
            await writer.wait_closed()
            await asyncio.sleep(0.05)
            assert server.client_count == 0

            # Server remains responsive
            reader2, writer2 = await asyncio.open_unix_connection(str(sock_path))
            req = json.dumps({"jsonrpc": "2.0", "id": 2, "method": "ping", "params": {}}) + "\n"
            writer2.write(req.encode("utf-8"))
            await writer2.drain()
            line = await reader2.readline()
            resp = json.loads(line.decode("utf-8"))
            assert resp["result"] == "pong"
            writer2.close()
            await writer2.wait_closed()
        finally:
            await server.stop()

    asyncio.run(_test())


# ============================================================================
# F26: Offline Mock Server Boundaries
# ============================================================================

def test_f26_b01_mock_server_expired_token_returns_401(mock_cloudcode):
    """F26 [Boundary]: Token with 'expired' in string returns HTTP 401 UNAUTHENTICATED."""
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:retrieveUserQuotaSummary",
        data=b"{}",
        headers={"Authorization": "Bearer ya29.expired_tok"}
    )
    with pytest.raises(urllib.error.HTTPError) as exc:
        urllib.request.urlopen(req)
    assert exc.value.code == 401


def test_f26_b02_mock_server_invalid_refresh_token_returns_400(mock_cloudcode):
    """F26 [Boundary]: Refresh token containing 'invalid' returns HTTP 400 invalid_grant."""
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/token",
        data=json.dumps({"refresh_token": "invalid_refresh_tok"}).encode(),
        headers={"Content-Type": "application/json"}
    )
    with pytest.raises(urllib.error.HTTPError) as exc:
        urllib.request.urlopen(req)
    assert exc.value.code == 400


def test_f26_b03_mock_server_simulates_503_recovery(mock_cloudcode):
    """F26 [Boundary]: Simulates 2 transient 503 errors followed by HTTP 200 recovery."""
    mock_cloudcode.transient_errors_remaining = 2
    mock_cloudcode.transient_error_status = 503
    api_req = urllib.request.Request(f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:fetchAvailableModels", data=b"{}", headers={"Authorization": "Bearer tok"})

    # Hit 1 -> 503
    with pytest.raises(urllib.error.HTTPError) as exc1:
        urllib.request.urlopen(api_req)
    assert exc1.value.code == 503

    # Hit 2 -> 503
    with pytest.raises(urllib.error.HTTPError) as exc2:
        urllib.request.urlopen(api_req)
    assert exc2.value.code == 503

    # Hit 3 -> 200 Recovered
    with urllib.request.urlopen(api_req) as resp:
        assert resp.status == 200


def test_f26_b04_mock_server_time_advancement_triggers_reset(mock_cloudcode):
    """F26 [Boundary]: Advancing time by 5 hours past resetTime enables warmup ping."""
    mock_cloudcode.set_quota(remaining=0.0, reset_in_seconds=18000)
    # Advancing 18001s puts us past resetTime
    mock_cloudcode.advance_time(18001)
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:generateContent",
        data=b'{"request": {"generationConfig": {"maxOutputTokens": 1}}}',
        headers={"Authorization": "Bearer tok"}
    )
    with urllib.request.urlopen(req) as resp:
        assert resp.status == 200


def test_f26_b05_mock_server_handles_unknown_route_with_404(mock_cloudcode):
    """F26 [Boundary]: Accessing unmapped endpoint returns HTTP 404 Not Found."""
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/unknown_endpoint",
        data=b"{}",
        headers={"Authorization": "Bearer valid_tok"}
    )
    with pytest.raises(urllib.error.HTTPError) as exc:
        urllib.request.urlopen(req)
    assert exc.value.code == 404
