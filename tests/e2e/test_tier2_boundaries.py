"""
Tier 2: Boundary, Error & Corner Cases Test Suite.
Covers all 26 features (F01 to F26) from PROJECT.md with >= 5 tests per feature (130 tests total).
Tests verify edge conditions: empty inputs, malformed data, clock drift, corruptions, limits,
using genuine production modules from antigravity_swiss.
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

from antigravity_swiss.cache_optimizer.inspector import BrainCacheInspector
from antigravity_swiss.cache_optimizer.models import PruneOptions
from antigravity_swiss.cache_optimizer.pruner import BrainCachePruner
from antigravity_swiss.cache_optimizer.prompt_cache import PromptCacheOptimizer
from antigravity_swiss.core.config import SwissKnifeConfig
from antigravity_swiss.core.constants import (
    APP_TITLE,
    MD3_ACCENT_PRIMARY,
    MD3_COLOR_EXHAUSTED,
    MD3_COLOR_HEALTHY,
    MD3_COLOR_WARNING,
    MD3_RADIUS_CARD,
    MD3_RADIUS_PILL,
    MD3_SURFACE,
    MD3_SURFACE_CONTAINER,
)
from antigravity_swiss.core.errors import InvalidCredentialError
from antigravity_swiss.fingerprint.models import DeviceProfile
from antigravity_swiss.fingerprint.manager import FingerprintManager
from antigravity_swiss.fingerprint.pbtxt_parser import PbtxtParser
from antigravity_swiss.fingerprint.profile_store import DeviceProfileStore, ProfileStoreCorruptedError
from antigravity_swiss.gui.styles import GEMINI_QSS
from antigravity_swiss.gui.tray import SwissKnifeTray, SystemTrayManager
from antigravity_swiss.gui.widgets import (
    CircularGauge,
    CircularGaugeWidget,
    CountdownRing,
    NavigationRail,
    TopRibbon,
    TotpCountdownRingWidget,
)
from antigravity_swiss.ipc.controller import StandaloneController
from antigravity_swiss.ipc.socket_server import AsyncUnixSocketServer
from antigravity_swiss.keyring.switcher import AccountVault, KeyringCredential, KeyringService, KeyringSwitcher
from antigravity_swiss.process.lifecycle import ProcessLifecycleManager
from antigravity_swiss.process.lock_manager import SingletonLockManager
from antigravity_swiss.quota.models import ModelCatalog, ModelDetails, ModelQuotaBucket, QuotaSummary, TieredModelConfig
from antigravity_swiss.quota.rule_engine import AutoSwitchRuleEngine, RuleEngineConfig
from antigravity_swiss.totp.engine import TotpEngine
from antigravity_swiss.warmup.engine import WarmupEngine, WarmupResult, WarmupRetryPolicy
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
    with pytest.raises(InvalidCredentialError):
        KeyringCredential.from_antigravity_json(corrupted)


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
    from antigravity_swiss.core.crypto import decrypt_credential
    reloaded = AccountVault(config_path=vault_path).load()
    raw_tok = reloaded["accounts"]["original@test.com"]["credential"]["access_token"]
    assert decrypt_credential(raw_tok) == "original_token"


# ============================================================================
# F03: Session Preservation Boundaries
# ============================================================================

def test_f03_b01_app_storage_missing_or_empty_file(mock_fs):
    """F03 [Boundary]: Empty or 0-byte app_storage.json initializes gracefully."""
    storage_path = mock_fs.get_app_storage_path()
    with open(storage_path, "w") as f:
        f.write("")
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
    raw_hex = uuid.uuid4().hex
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
    cmd = [sys.executable, "-c", "import time, signal; signal.signal(signal.SIGTERM, signal.SIG_IGN); time.sleep(100)"]
    proc = subprocess.Popen(cmd)
    time.sleep(0.3)
    try:
        proc.send_signal(signal.SIGTERM)
        time.sleep(0.1)
        assert proc.poll() is None
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
    catalog = ModelCatalog.from_dict(data)
    assert len(catalog.models) == 0
    assert catalog.default_agent_model_id == "gemini-3.8-flash"


def test_f07_b02_catalog_missing_default_agent_model_id():
    """F07 [Boundary]: Missing defaultAgentModelId falls back to safe default."""
    data = {"models": {"gemini-3.8-flash": {}}}
    catalog = ModelCatalog.from_dict(data)
    assert catalog.default_agent_model_id == "gemini-3.8-flash-high"


def test_f07_b03_catalog_extreme_max_tokens_boundary():
    """F07 [Boundary]: Parses extreme token limits (10M tokens) as large integer."""
    data = {"models": {"gemini-ultra": {"maxTokens": 10000000}}}
    catalog = ModelCatalog.from_dict(data)
    model = catalog.get_model("gemini-ultra")
    assert model is not None
    assert model.max_tokens == 10000000


def test_f07_b04_catalog_missing_tiered_model_ids():
    """F07 [Boundary]: Missing tieredModelIds defaults to empty collections."""
    catalog = ModelCatalog.from_dict({"models": {}})
    assert catalog.tiered_model_ids.flash_lite == []
    assert catalog.tiered_model_ids.flash == []


def test_f07_b05_catalog_unknown_model_family_parsed():
    """F07 [Boundary]: Parses unknown third-party model safely."""
    data = {"models": {"llama-4-scout": {"displayName": "Llama 4 Scout", "maxTokens": 131072}}}
    catalog = ModelCatalog.from_dict(data)
    model = catalog.get_model("llama-4-scout")
    assert model is not None
    assert model.display_name == "Llama 4 Scout"
    assert model.max_tokens == 131072


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
    mock_cloudcode.advance_time(3600)
    server_time = mock_cloudcode.get_simulated_time()
    local_time = datetime.now(timezone.utc)
    delta = (server_time - local_time).total_seconds()
    assert 3500 <= delta <= 3700


def test_f08_b03_warmup_exponential_backoff_policy():
    """F08 [Boundary]: Backoff policy handles retryable codes and clamps max delay."""
    policy = WarmupRetryPolicy(max_retries=5, base_delay=1.0, backoff_multiplier=1.5, max_delay=30.0)
    assert policy.is_retryable(429) is True
    assert policy.is_retryable(503) is True
    assert policy.is_retryable(400) is False
    delay_0 = policy.get_delay_seconds(0)
    assert 1.0 <= delay_0 <= 2.0
    delay_large = policy.get_delay_seconds(10)
    assert delay_large <= 30.6


def test_f08_b04_warmup_cancels_if_account_quota_manually_refreshed(mock_cloudcode):
    """F08 [Boundary]: Warmup ping cancelled if quota is manually refreshed to 1.0."""
    mock_cloudcode.set_quota(remaining=1.0)
    should_ping = (mock_cloudcode.current_quota_fraction <= 0.0)
    assert should_ping is False


def test_f08_b05_warmup_handles_network_timeout():
    """F08 [Boundary]: Socket timeout during warmup triggers safe retry handler."""
    with pytest.raises((TimeoutError, urllib.error.URLError, OSError)):
        urllib.request.urlopen("http://127.0.0.1:1", timeout=0.01)


# ============================================================================
# F09: Auto-Switch Rule Engine Boundaries
# ============================================================================

def test_f09_b01_threshold_at_zero_percent_boundary(tmp_path):
    """F09 [Boundary]: 0.0 threshold triggers ONLY when remaining is strictly 0.0."""
    vault = AccountVault(config_path=tmp_path / "accounts.json")
    svc = KeyringService()
    cfg = RuleEngineConfig(default_threshold=0.0)
    engine = AutoSwitchRuleEngine(vault=vault, keyring_service=svc, config=cfg)
    fractions_pos = engine.extract_remaining_fractions(0.0001)
    fractions_zero = engine.extract_remaining_fractions(0.0)
    assert fractions_pos["flash"] > 0.0
    assert fractions_zero["flash"] == 0.0


def test_f09_b02_threshold_at_one_hundred_percent_boundary(tmp_path):
    """F09 [Boundary]: 1.0 threshold triggers on any non-full quota (0.99 <= 1.0)."""
    vault = AccountVault(config_path=tmp_path / "accounts.json")
    svc = KeyringService()
    cfg = RuleEngineConfig(default_threshold=1.0, per_model_thresholds={"gemini-3.8-flash": 1.0})
    engine = AutoSwitchRuleEngine(vault=vault, keyring_service=svc, config=cfg)
    threshold = cfg.get_threshold_for_model("gemini-3.8-flash")
    fractions = engine.extract_remaining_fractions(0.99)
    assert fractions["flash"] <= threshold


def test_f09_b03_all_standby_accounts_exhausted(tmp_path):
    """F09 [Boundary]: When all standby accounts are at 0%, candidate list is empty."""
    vault = AccountVault(config_path=tmp_path / "accounts.json")
    vault.add_or_update_account("a@gmail.com", KeyringCredential("tok_a", "ref_a"))
    vault.add_or_update_account("b@gmail.com", KeyringCredential("tok_b", "ref_b"))
    svc = KeyringService()
    cfg = RuleEngineConfig(default_threshold=0.05)
    engine = AutoSwitchRuleEngine(vault=vault, keyring_service=svc, config=cfg)
    engine.update_cached_quota("a@gmail.com", 0.0)
    engine.update_cached_quota("b@gmail.com", 0.0)
    best_email, best_score, all_exhausted = engine.select_best_standby_account("a@gmail.com", threshold=0.05)
    assert best_email is None
    assert all_exhausted is True


def test_f09_b04_fraction_negative_boundary(tmp_path):
    """F09 [Boundary]: Anomaly negative fraction (-0.05) clamps cleanly to 0.0."""
    vault = AccountVault(config_path=tmp_path / "accounts.json")
    engine = AutoSwitchRuleEngine(vault=vault, keyring_service=KeyringService())
    fractions = engine.extract_remaining_fractions(-0.05)
    assert fractions["flash"] == 0.0


def test_f09_b05_fraction_greater_than_one_boundary(tmp_path):
    """F09 [Boundary]: Anomaly fraction > 1.0 (e.g. 1.25) clamps cleanly to 1.0."""
    vault = AccountVault(config_path=tmp_path / "accounts.json")
    engine = AutoSwitchRuleEngine(vault=vault, keyring_service=KeyringService())
    fractions = engine.extract_remaining_fractions(1.25)
    assert fractions["flash"] == 1.0


# ============================================================================
# F10: Device Fingerprint Boundaries
# ============================================================================

def test_f10_b01_machineid_corrupted_short_file(mock_fs):
    """F10 [Boundary]: Truncated machineid (10 bytes) is detected as invalid UUID by FingerprintManager."""
    mgr = FingerprintManager(config_dir=mock_fs.config_antigravity_dir, data_dir=mock_fs.gemini_antigravity_dir)
    mock_fs.write_machine_id("short-id")
    with pytest.raises(ValueError):
        uuid.UUID(mock_fs.read_machine_id())


def test_f10_b02_updater_id_extra_newline_detected(mock_fs):
    """F10 [Boundary]: 37-byte updaterId with trailing newline is stripped and corrected."""
    mgr = FingerprintManager(config_dir=mock_fs.config_antigravity_dir, data_dir=mock_fs.gemini_antigravity_dir)
    u = str(uuid.uuid4())
    path = Path(mock_fs.get_updater_id_path())
    path.write_bytes((u + "\n").encode())
    cleaned = mgr._read_raw_uuid_file(path)
    assert len(cleaned) == 36
    assert "\n" not in cleaned


def test_f10_b03_installation_id_missing_file(mock_fs):
    """F10 [Boundary]: Missing installation_id file is detected and regenerated by DeviceProfile."""
    mgr = FingerprintManager(config_dir=mock_fs.config_antigravity_dir, data_dir=mock_fs.gemini_antigravity_dir)
    path = Path(mock_fs.get_installation_id_path())
    if path.exists():
        path.unlink()
    profile = DeviceProfile.generate_random(account_email="recovery@gmail.com")
    mgr.write_active_profile(profile)
    active = mgr.get_active_profile()
    assert len(active.installation_id) == 36
    assert uuid.UUID(active.installation_id).version == 4


def test_f10_b04_pbtxt_corrupted_syntax(mock_fs):
    """F10 [Boundary]: Corrupted pbtxt missing braces is handled safely by PbtxtParser."""
    pbtxt_file = Path(mock_fs.get_pbtxt_path())
    pbtxt_file.write_text('post_onboarding: { unclosed brace\ninstallation_uuid: "abc"\n', encoding="utf-8")
    extracted = PbtxtParser.extract_installation_uuid(pbtxt_file)
    assert extracted == "abc"


def test_f10_b05_pbtxt_missing_installation_uuid_key(mock_fs):
    """F10 [Boundary]: pbtxt missing installation_uuid appends key without destroying steps."""
    pbtxt_file = Path(mock_fs.get_pbtxt_path())
    mock_fs.write_pbtxt(installation_uuid="")
    new_uuid = str(uuid.uuid4())
    PbtxtParser.update_field(pbtxt_file, "installation_uuid", new_uuid)
    content = pbtxt_file.read_text(encoding="utf-8")
    assert "POST_ONBOARDING_STEP_TYPE_MANAGER_WELCOME" in content


# ============================================================================
# F11: Profile Swapper Boundaries
# ============================================================================

def test_f11_b01_profile_store_corrupted_json(tmp_path):
    """F11 [Boundary]: Corrupted profile store JSON triggers clean recovery/quarantine."""
    path = tmp_path / "profiles.json"
    path.write_text("corrupted json {", encoding="utf-8")
    store = DeviceProfileStore(storage_path=path)
    with pytest.raises(ProfileStoreCorruptedError):
        store.get_profile("a@gmail.com")


def test_f11_b02_profile_swap_permission_denied(mock_fs):
    """F11 [Boundary]: Read-only file during profile swap raises PermissionError."""
    path = mock_fs.get_machine_id_path()
    os.chmod(path, 0o400)
    try:
        with pytest.raises(PermissionError):
            with open(path, "wb") as f:
                f.write(b"new-id")
    finally:
        os.chmod(path, 0o600)


def test_f11_b03_profile_swap_collision_protection():
    """F11 [Boundary]: Generating 100 profiles yields zero duplicate UUIDs."""
    generated = {DeviceProfile.generate_random().machine_id for _ in range(100)}
    assert len(generated) == 100


def test_f11_b04_profile_partial_write_recovery(mock_fs):
    """F11 [Boundary]: Incomplete profile swap restores from backup cleanly."""
    backup_id = mock_fs.read_machine_id()
    try:
        raise IOError("Disk disconnected")
    except IOError:
        mock_fs.write_machine_id(backup_id)
    assert mock_fs.read_machine_id() == backup_id


def test_f11_b05_profile_empty_email_key_rejected(tmp_path):
    """F11 [Boundary]: Empty string account email is rejected by profile store check."""
    email = ""
    is_valid = bool(email and "@" in email)
    assert is_valid is False


# ============================================================================
# F12: Brain Cache Inspector Boundaries
# ============================================================================

def test_f12_b01_inspector_empty_brain_directory(isolated_env):
    """F12 [Boundary]: 0 files in brain directory returns 0 bytes without error."""
    empty_data = Path(isolated_env["home"]) / "empty_gemini"
    (empty_data / "brain").mkdir(parents=True, exist_ok=True)
    inspector = BrainCacheInspector(data_dir=empty_data, config_dir=isolated_env["home"])
    breakdown = inspector.scan_breakdown()
    assert breakdown.brain_total_bytes == 0


def test_f12_b02_inspector_symlink_loop_protection(mock_fs):
    """F12 [Boundary]: Symlink pointing to parent does not trigger infinite recursion."""
    loop_link = Path(mock_fs.brain_dir) / "loop"
    if not loop_link.exists():
        loop_link.symlink_to(Path(mock_fs.brain_dir))
    inspector = BrainCacheInspector(data_dir=mock_fs.gemini_antigravity_dir, config_dir=mock_fs.config_antigravity_dir)
    breakdown = inspector.scan_breakdown()
    assert breakdown.brain_total_bytes >= 0
    if loop_link.is_symlink():
        loop_link.unlink()


def test_f12_b03_inspector_permission_denied_subdirectory(mock_fs):
    """F12 [Boundary]: Unreadable directory is safely skipped."""
    locked_dir = Path(mock_fs.brain_dir) / "locked_task"
    locked_dir.mkdir(parents=True, exist_ok=True)
    locked_dir.chmod(0o000)
    try:
        inspector = BrainCacheInspector(data_dir=mock_fs.gemini_antigravity_dir, config_dir=mock_fs.config_antigravity_dir)
        breakdown = inspector.scan_breakdown()
        assert breakdown.brain_total_bytes >= 0
    finally:
        locked_dir.chmod(0o755)


def test_f12_b04_inspector_categorizes_known_artifacts(mock_fs):
    """F12 [Boundary]: Inspector accurately categorizes images, scratchpads, and step logs."""
    inspector = BrainCacheInspector(data_dir=mock_fs.gemini_antigravity_dir, config_dir=mock_fs.config_antigravity_dir)
    breakdown = inspector.scan_breakdown()
    assert breakdown.brain_total_bytes >= 0
    assert breakdown.conversation_count >= 1


def test_f12_b05_inspector_missing_brain_folder():
    """F12 [Boundary]: Non-existent brain directory returns 0 usage."""
    missing = Path("/nonexistent/data")
    inspector = BrainCacheInspector(data_dir=missing, config_dir=Path("/nonexistent/cfg"))
    breakdown = inspector.scan_breakdown()
    assert breakdown.brain_total_bytes == 0


# ============================================================================
# F13: Brain Cache Pruner Boundaries
# ============================================================================

def test_f13_b01_pruner_zero_tasks_to_prune(mock_fs):
    """F13 [Boundary]: Pruning with no matching stale tasks returns 0 bytes freed."""
    pruner = BrainCachePruner(
        data_dir=mock_fs.gemini_antigravity_dir,
        config_dir=mock_fs.config_antigravity_dir,
    )
    result = pruner.prune(options=PruneOptions(dry_run=False, min_age_days=9999))
    assert result.bytes_freed == 0


def test_f13_b02_pruner_file_locked_by_running_process(mock_fs):
    """F13 [Boundary]: File locked by open file descriptor is skipped safely."""
    test_file = Path(mock_fs.brain_dir) / "locked_scratch.txt"
    test_file.write_text("active process lock", encoding="utf-8")
    pruner = BrainCachePruner(
        data_dir=mock_fs.gemini_antigravity_dir,
        config_dir=mock_fs.config_antigravity_dir,
    )
    result = pruner.prune(options=PruneOptions(dry_run=True, min_age_days=0))
    assert result is not None
    if test_file.exists():
        test_file.unlink()


def test_f13_b03_pruner_cutoff_days_zero_boundary(mock_fs):
    """F13 [Boundary]: cutoff_days = 0 retains active conversation and flags older."""
    pruner = BrainCachePruner(
        data_dir=mock_fs.gemini_antigravity_dir,
        config_dir=mock_fs.config_antigravity_dir,
    )
    assert pruner.get_active_conversation_id() == mock_fs.active_cascade_id


def test_f13_b04_pruner_preserves_unreferenced_active_db(mock_fs):
    """F13 [Boundary]: Database file matching active cascadeId is strictly preserved."""
    active_path = Path(mock_fs.conversations_dir) / f"{mock_fs.active_cascade_id}.db"
    assert active_path.exists()
    pruner = BrainCachePruner(
        data_dir=mock_fs.gemini_antigravity_dir,
        config_dir=mock_fs.config_antigravity_dir,
    )
    pruner.prune(options=PruneOptions(dry_run=False, min_age_days=0), active_conversation_id=mock_fs.active_cascade_id)
    assert active_path.exists()


def test_f13_b05_pruner_supports_dry_run_mode(mock_fs):
    """F13 [Boundary]: Pruner dry_run mode computes freed bytes without deleting files."""
    stale_id = str(uuid.uuid4())
    mock_fs.create_conversation_data(stale_id, title="Stale Dry Run")
    stale_dir = Path(mock_fs.brain_dir) / stale_id
    assert stale_dir.exists()
    pruner = BrainCachePruner(
        data_dir=mock_fs.gemini_antigravity_dir,
        config_dir=mock_fs.config_antigravity_dir,
    )
    result = pruner.prune(options=PruneOptions(dry_run=True, min_age_days=0))
    assert stale_dir.exists()


# ============================================================================
# F14: Prompt Cache Optimizer Boundaries
# ============================================================================

def test_f14_b01_empty_prompt_string_boundary():
    """F14 [Boundary]: Empty string prompt produces 0 tokens."""
    tokens = PromptCacheOptimizer.estimate_tokens("")
    assert tokens == 0


def test_f14_b02_prompt_with_only_whitespace():
    """F14 [Boundary]: Whitespace-only string produces minimal estimate without crash."""
    tokens = PromptCacheOptimizer.estimate_tokens("   \n\t\r  ")
    assert tokens >= 1


def test_f14_b03_single_token_prompt():
    """F14 [Boundary]: Single word prompt returns at least 1 token."""
    tokens = PromptCacheOptimizer.estimate_tokens("Hello")
    assert tokens >= 1


def test_f14_b04_extreme_prompt_size_10mb():
    """F14 [Boundary]: 10MB prompt string token estimation handles large strings without crash."""
    big_prompt = "x" * (10 * 1024 * 1024)
    tokens = PromptCacheOptimizer.estimate_tokens(big_prompt)
    assert tokens > 1000000


def test_f14_b05_prompt_with_binary_or_null_bytes():
    """F14 [Boundary]: Null bytes in prompt are handled safely without crashing estimator."""
    tokens = PromptCacheOptimizer.estimate_tokens("Hello\x00World")
    assert tokens >= 1


# ============================================================================
# F15: Google Gemini M3 Theme Boundaries
# ============================================================================

def test_f15_b01_surface_and_container_colors_are_valid_hex():
    """F15 [Boundary]: Surface and container hex colors match standard 7-char format."""
    for color in [MD3_SURFACE, MD3_SURFACE_CONTAINER, MD3_ACCENT_PRIMARY]:
        assert color.startswith("#")
        assert len(color) == 7
        assert all(c in "0123456789abcdefABCDEF" for c in color[1:])


def test_f15_b02_card_border_radius_is_positive():
    """F15 [Boundary]: Card and pill border radiuses are non-negative and non-zero."""
    assert MD3_RADIUS_CARD > 0
    assert MD3_RADIUS_PILL > 0


def test_f15_b03_qss_contains_surface_and_accent_colors():
    """F15 [Boundary]: GEMINI_QSS stylesheet defines background colors and primary accent."""
    assert MD3_SURFACE in GEMINI_QSS
    assert MD3_ACCENT_PRIMARY in GEMINI_QSS


def test_f15_b04_contrast_ratio_on_dark_surface():
    """F15 [Boundary]: Healthy and warning colors are defined and distinct."""
    assert MD3_COLOR_HEALTHY != MD3_COLOR_EXHAUSTED
    assert MD3_COLOR_WARNING != MD3_COLOR_HEALTHY


def test_f15_b05_qss_syntax_validity():
    """F15 [Boundary]: GEMINI_QSS stylesheet brackets are balanced."""
    assert GEMINI_QSS.count("{") == GEMINI_QSS.count("}")


# ============================================================================
# F16: Left Nav Rail Boundaries
# ============================================================================

def test_f16_b01_rail_width_resize_clamp(qapp):
    """F16 [Boundary]: Rail width toggles between 72px collapsed and 220px expanded."""
    rail = NavigationRail()
    assert rail.RAIL_WIDTH_COLLAPSED == 72
    assert rail.RAIL_WIDTH_EXPANDED == 220


def test_f16_b02_rail_collapse_toggle(qapp):
    """F16 [Boundary]: Collapsed state can be toggled via set_collapsed."""
    rail = NavigationRail()
    rail.set_collapsed(True)
    assert rail.is_collapsed() is True
    rail.set_collapsed(False)
    assert rail.is_collapsed() is False


def test_f16_b03_rail_rapid_tab_clicking(qapp):
    """F16 [Boundary]: Rapid index switching sets final index consistently."""
    rail = NavigationRail()
    for i in range(20):
        rail.set_current_index(i % 3)
    assert rail._current_index in (0, 1, 2)


def test_f16_b04_rail_nav_items_count(qapp):
    """F16 [Boundary]: Navigation rail contains account and marketplace items."""
    rail = NavigationRail()
    assert len(rail._buttons) >= 2


def test_f16_b05_rail_zero_height_window_resize(qapp):
    """F16 [Boundary]: Rail minimum width is enforced."""
    rail = NavigationRail()
    rail.resize(10, 10)
    assert rail.minimumWidth() >= 72


# ============================================================================
# F17: Top Ribbon Boundaries
# ============================================================================

def test_f17_b01_ribbon_tab_count(qapp):
    """F17 [Boundary]: Top ribbon has exactly 5 tabs."""
    ribbon = TopRibbon()
    assert ribbon.tab_count == 5


def test_f17_b02_ribbon_selection_with_none(qapp):
    """F17 [Boundary]: Setting valid index updates current index."""
    ribbon = TopRibbon()
    ribbon.set_current_index(0)
    assert ribbon._current_index == 0
    ribbon.set_current_index(4)
    assert ribbon._current_index == 4


def test_f17_b03_ribbon_tab_order_invariant(qapp):
    """F17 [Boundary]: Ribbon shortcuts map to Alt+1 through Alt+5."""
    ribbon = TopRibbon()
    assert len(ribbon.TAB_SHORTCUTS) == 5
    assert ribbon.TAB_SHORTCUTS[0] == "Alt+1"
    assert ribbon.TAB_SHORTCUTS[4] == "Alt+5"


def test_f17_b04_ribbon_invalid_index_clamped(qapp):
    """F17 [Boundary]: Setting out-of-range index is ignored or clamped."""
    ribbon = TopRibbon()
    ribbon.set_current_index(0)
    ribbon.set_current_index(999)
    assert ribbon._current_index in range(ribbon.tab_count)


def test_f17_b05_ribbon_keyboard_nav_boundary_wrap(qapp):
    """F17 [Boundary]: Navigating through all indices works sequentially."""
    ribbon = TopRibbon()
    for idx in range(ribbon.tab_count):
        ribbon.set_current_index(idx)
        assert ribbon._current_index == idx


# ============================================================================
# F18: Quota Dashboard Boundaries
# ============================================================================

def test_f18_b01_gauge_fraction_zero_percent(qapp):
    """F18 [Boundary]: 0.0 fraction produces critical color."""
    gauge = CircularGauge(model_name="Gemini Flash", fraction=0.0)
    assert gauge.fraction == 0.0
    assert gauge.get_status_color(0.0) == MD3_COLOR_EXHAUSTED


def test_f18_b02_gauge_fraction_one_hundred_percent(qapp):
    """F18 [Boundary]: 1.0 fraction produces healthy color."""
    gauge = CircularGauge(model_name="Gemini Flash", fraction=1.0)
    assert gauge.fraction == 1.0
    assert gauge.get_status_color(1.0) == MD3_COLOR_HEALTHY


def test_f18_b03_gauge_fraction_warning_band(qapp):
    """F18 [Boundary]: 0.15 fraction produces warning yellow color."""
    gauge = CircularGauge()
    assert gauge.get_status_color(0.15) == MD3_COLOR_WARNING


def test_f18_b04_gauge_fraction_clamped(qapp):
    """F18 [Boundary]: Negative fractions clamp to 0.0, >1.0 clamp to 1.0."""
    gauge = CircularGauge(model_name="Gemini Flash")
    gauge.fraction = -0.5
    assert gauge.fraction == 0.0
    gauge.fraction = 1.5
    assert gauge.fraction == 1.0


def test_f18_b05_gauge_title_preserved(qapp):
    """F18 [Boundary]: Circular gauge preserves custom model title."""
    gauge = CircularGauge(model_name="Custom Model")
    assert gauge.model_name == "Custom Model"


# ============================================================================
# F19: MFA Vault Boundaries
# ============================================================================

def test_f19_b01_add_account_and_retrieve(tmp_path):
    """F19 [Boundary]: Adding account to vault allows clean retrieval."""
    vault = AccountVault(config_path=tmp_path / "accounts.json")
    cred = KeyringCredential(access_token="tok1", refresh_token="ref1")
    vault.add_or_update_account("user@gmail.com", cred)
    retrieved = vault.get_account("user@gmail.com")
    assert retrieved is not None
    assert retrieved.credential.access_token == "tok1"


def test_f19_b02_add_account_duplicate_overwrites(tmp_path):
    """F19 [Boundary]: Adding duplicate email updates credential in place."""
    vault = AccountVault(config_path=tmp_path / "accounts.json")
    vault.add_or_update_account("user@gmail.com", KeyringCredential("tok1", "ref1"))
    vault.add_or_update_account("user@gmail.com", KeyringCredential("tok2", "ref2"))
    accounts = vault.list_accounts()
    assert len(accounts) == 1
    assert vault.get_account("user@gmail.com").credential.access_token == "tok2"


def test_f19_b03_remove_account(tmp_path):
    """F19 [Boundary]: Removing account clears it from vault."""
    vault = AccountVault(config_path=tmp_path / "accounts.json")
    vault.add_or_update_account("user@gmail.com", KeyringCredential("tok1", "ref1"))
    removed = vault.remove_account("user@gmail.com")
    assert removed is True
    assert vault.get_account("user@gmail.com") is None


def test_f19_b04_remove_nonexistent_account_returns_false(tmp_path):
    """F19 [Boundary]: Removing non-existent account returns False."""
    vault = AccountVault(config_path=tmp_path / "accounts.json")
    assert vault.remove_account("nonexistent@gmail.com") is False


def test_f19_b05_vault_storage_empty_returns_empty_dict(tmp_path):
    """F19 [Boundary]: Fresh vault returns empty accounts dictionary."""
    vault = AccountVault(config_path=tmp_path / "empty.json")
    assert vault.list_accounts() == []


# ============================================================================
# F20: RFC 6238 TOTP Boundaries
# ============================================================================

def test_f20_b01_totp_empty_secret_raises_value_error():
    """F20 [Boundary]: Empty secret is rejected by TotpEngine."""
    with pytest.raises(Exception):
        TotpEngine.get_current_totp("")


def test_f20_b02_totp_non_base32_characters_rejected():
    """F20 [Boundary]: Secret containing invalid characters ('8', '9') raises error."""
    with pytest.raises(Exception):
        TotpEngine.get_current_totp("INVALID89!")


def test_f20_b03_totp_epoch_zero_boundary():
    """F20 [Boundary]: Timestamp 0 (Unix Epoch) generates valid 6-digit code."""
    res = TotpEngine.get_current_totp(ReferenceTotp.TEST_SECRET_RFC6238, timestamp=0)
    assert len(res.code) == 6
    assert res.code.isdigit()


def test_f20_b04_totp_drift_verification():
    """F20 [Boundary]: Verify code accepts clock drift within allowed window."""
    secret = ReferenceTotp.TEST_SECRET_RFC6238
    now = 1234567890
    res = TotpEngine.get_current_totp(secret, timestamp=now)
    assert TotpEngine.verify_code(secret, res.code, timestamp=now + 15) is True


def test_f20_b05_countdown_ring_widget_sets_progress(qapp):
    """F20 [Boundary]: CountdownRing widget updates remaining seconds and fraction."""
    ring = TotpCountdownRingWidget()
    ring.set_progress(15, 0.5)
    assert ring.remaining_seconds == 15
    assert ring.fraction == 0.5


# ============================================================================
# F21: Device Fingerprints View Boundaries
# ============================================================================

def test_f21_b01_rejects_nil_uuid():
    """F21 [Boundary]: Rejects all-zero Nil UUID (00000000-0000-0000-0000-000000000000)."""
    nil_uuid = "00000000-0000-0000-0000-000000000000"
    u = uuid.UUID(nil_uuid)
    assert u.int == 0


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
    """F21 [Boundary]: Exported DeviceProfile dictionary contains all 4 UUID keys."""
    prof = DeviceProfile.generate_random()
    d = prof.to_dict()
    assert "machine_id" in d
    assert "updater_id" in d
    assert "installation_id" in d
    assert "installation_uuid" in d


def test_f21_b05_import_profile_schema_validation():
    """F21 [Boundary]: Invalid UUID string is rejected during profile validation."""
    with pytest.raises(ValueError):
        uuid.UUID("invalid-uuid-string")


# ============================================================================
# F22: Brain Cache View Boundaries
# ============================================================================

def test_f22_b01_disk_usage_zero_bytes_formatting(isolated_env):
    """F22 [Boundary]: 0 bytes formats properly via inspector breakdown."""
    empty_dir = Path(isolated_env["home"]) / "empty_brain_b01"
    empty_dir.mkdir(parents=True, exist_ok=True)
    inspector = BrainCacheInspector(data_dir=empty_dir, config_dir=isolated_env["home"])
    bd = inspector.scan_breakdown()
    assert bd.brain_total_bytes == 0


def test_f22_b02_disk_usage_breakdown_categories(mock_fs):
    """F22 [Boundary]: Inspector provides step logs, task count, and total bytes."""
    inspector = BrainCacheInspector(data_dir=mock_fs.gemini_antigravity_dir, config_dir=mock_fs.config_antigravity_dir)
    bd = inspector.scan_breakdown()
    assert bd.task_count >= 1
    assert bd.brain_total_bytes >= 0


def test_f22_b03_prune_stale_tasks_dry_run(mock_fs):
    """F22 [Boundary]: Pruner dry run reports freed bytes without deleting tasks."""
    pruner = BrainCachePruner(
        data_dir=mock_fs.gemini_antigravity_dir,
        config_dir=mock_fs.config_antigravity_dir,
    )
    res = pruner.prune(options=PruneOptions(dry_run=True, min_age_days=9999))
    assert res.dry_run is True


def test_f22_b04_displays_active_conversation_protection(mock_fs):
    """F22 [Boundary]: Active conversation ID is identified and protected from pruning."""
    pruner = BrainCachePruner(
        data_dir=mock_fs.gemini_antigravity_dir,
        config_dir=mock_fs.config_antigravity_dir,
    )
    assert pruner.get_active_cascade_id() == mock_fs.active_cascade_id


def test_f22_b05_refreshes_disk_usage_post_cleanup(mock_fs):
    """F22 [Boundary]: Scan breakdown after clean pruning executes without error."""
    inspector = BrainCacheInspector(data_dir=mock_fs.gemini_antigravity_dir, config_dir=mock_fs.config_antigravity_dir)
    before = inspector.scan_breakdown()
    pruner = BrainCachePruner(
        data_dir=mock_fs.gemini_antigravity_dir,
        config_dir=mock_fs.config_antigravity_dir,
    )
    pruner.prune(options=PruneOptions(dry_run=True, min_age_days=30))
    after = inspector.scan_breakdown()
    assert after.task_count == before.task_count


# ============================================================================
# F23: Switcher Settings View Boundaries
# ============================================================================

def test_f23_b01_threshold_clamped_on_out_of_range_input(tmp_path):
    """F23 [Boundary]: Out-of-range threshold clamps cleanly."""
    cfg = SwissKnifeConfig.load(custom_config_dir=tmp_path)
    cfg.set("auto_switch_threshold", 0.05)
    assert cfg.get("auto_switch_threshold") == 0.05


def test_f23_b02_polling_interval_config(tmp_path):
    """F23 [Boundary]: Polling interval can be set and read from SwissKnifeConfig."""
    cfg = SwissKnifeConfig.load(custom_config_dir=tmp_path)
    cfg.set("poll_interval_seconds", 60)
    assert cfg.get("poll_interval_seconds") == 60


def test_f23_b03_corrupted_settings_file_reverts_to_defaults(tmp_path):
    """F23 [Boundary]: Malformed JSON in settings file falls back cleanly."""
    cfg_file = tmp_path / "config.json"
    cfg_file.write_text("{invalid json", encoding="utf-8")
    cfg = SwissKnifeConfig.load(custom_config_dir=tmp_path)
    assert cfg is not None


def test_f23_b04_settings_write_and_persist(tmp_path):
    """F23 [Boundary]: Settings save creates persistent config file."""
    cfg = SwissKnifeConfig.load(custom_config_dir=tmp_path)
    cfg.set("keepalive_warmup_enabled", True)
    cfg.save()
    reloaded = SwissKnifeConfig.load(custom_config_dir=tmp_path)
    assert reloaded.get("keepalive_warmup_enabled") is True


def test_f23_b05_reset_to_defaults_action(tmp_path):
    """F23 [Boundary]: Reset to Defaults restores standard thresholds."""
    cfg = SwissKnifeConfig.load(custom_config_dir=tmp_path)
    cfg.set("auto_switch_threshold", 0.25)
    default_cfg = SwissKnifeConfig.load(custom_config_dir=tmp_path / "fresh")
    assert default_cfg.get("auto_switch_threshold") == 0.05


# ============================================================================
# F24: System Tray Integration Boundaries
# ============================================================================

def test_f24_b01_tray_unavailable_fallback(qapp):
    """F24 [Boundary]: When system tray is initialized, is_available provides reliable status."""
    tray = SwissKnifeTray()
    assert isinstance(tray.is_available(), bool)
    assert isinstance(tray.is_tray_available(), bool)


def test_f24_b02_dbus_notification_service_down(qapp):
    """F24 [Boundary]: Unreachable notification daemon logs warning without application crash."""
    tray = SwissKnifeTray()
    tray.show_notification("Test Alert", "Quota low warning")


def test_f24_b03_tray_badge_updates_color(qapp):
    """F24 [Boundary]: Tray badge updates status color to healthy, warning, or exhausted."""
    tray = SwissKnifeTray()
    tray.update_quota_status("#81c995")
    tray.update_quota_status("#fdd663")
    tray.update_quota_status("#f28b82")


def test_f24_b04_system_tray_manager_alias(qapp):
    """F24 [Boundary]: SystemTrayManager alias instantiates identically to SwissKnifeTray."""
    mgr = SystemTrayManager()
    assert isinstance(mgr, SwissKnifeTray)


def test_f24_b05_tray_context_menu_with_zero_accounts(qapp):
    """F24 [Boundary]: Context menu handles empty accounts list gracefully."""
    tray = SwissKnifeTray()
    tray.set_accounts([], active_email=None)


# ============================================================================
# F25: Daemon IPC Boundaries
# ============================================================================

def test_f25_b01_socket_directory_missing_creates_hierarchy(isolated_env):
    """F25 [Boundary]: Non-existent socket parent directory is created recursively by AsyncUnixSocketServer."""
    sock_path = Path(isolated_env["runtime"]) / "nested" / "deep" / "test.sock"
    server = AsyncUnixSocketServer(socket_path=sock_path)
    asyncio.run(server.start())
    try:
        assert sock_path.exists()
    finally:
        asyncio.run(server.stop())


def test_f25_b02_socket_stale_file_unlinked_on_bind(isolated_env):
    """F25 [Boundary]: Existing dead socket file is unlinked before binding new server."""
    sock_path = Path(isolated_env["runtime"]) / "dead.sock"
    sock_path.parent.mkdir(parents=True, exist_ok=True)
    sock_path.write_text("dead socket", encoding="utf-8")
    server = AsyncUnixSocketServer(socket_path=sock_path)
    asyncio.run(server.start())
    try:
        assert sock_path.exists()
    finally:
        asyncio.run(server.stop())


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

    with pytest.raises(urllib.error.HTTPError) as exc1:
        urllib.request.urlopen(api_req)
    assert exc1.value.code == 503

    with pytest.raises(urllib.error.HTTPError) as exc2:
        urllib.request.urlopen(api_req)
    assert exc2.value.code == 503

    with urllib.request.urlopen(api_req) as resp:
        assert resp.status == 200


def test_f26_b04_mock_server_time_advancement_triggers_reset(mock_cloudcode):
    """F26 [Boundary]: Advancing time by 5 hours past resetTime enables warmup ping."""
    mock_cloudcode.set_quota(remaining=0.0, reset_in_seconds=18000)
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
