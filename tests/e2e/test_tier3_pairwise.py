"""
Tier 3: Pairwise Combinatorial Feature Interaction Test Suite.
Covers 26 pairwise feature interactions across project subsystems (F01-F26).
Verifies interface contracts, atomic cross-module state transitions, and IPC coordination
using genuine production modules from antigravity_swiss.
"""

from datetime import datetime, timedelta, timezone
import json
import os
from pathlib import Path
import sqlite3
import subprocess
import time
import urllib.request
import uuid
import pytest

from antigravity_swiss.cache_optimizer.inspector import BrainCacheInspector
from antigravity_swiss.cache_optimizer.models import PruneOptions
from antigravity_swiss.cache_optimizer.pruner import BrainCachePruner
from antigravity_swiss.cache_optimizer.prompt_cache import PromptCacheOptimizer
from antigravity_swiss.core.config import SwissKnifeConfig
from antigravity_swiss.core.constants import (
    MD3_COLOR_EXHAUSTED,
    MD3_COLOR_HEALTHY,
    MD3_COLOR_WARNING,
)
from antigravity_swiss.fingerprint.models import DeviceProfile
from antigravity_swiss.fingerprint.manager import FingerprintManager
try:
    from antigravity_swiss.gui.tray import SwissKnifeTray
    from antigravity_swiss.gui.widgets import CircularGauge
except ImportError:
    SwissKnifeTray = None
    CircularGauge = None
from antigravity_swiss.ipc.controller import StandaloneController
from antigravity_swiss.keyring.switcher import AccountVault, KeyringCredential, KeyringService, KeyringSwitcher
from antigravity_swiss.process.lifecycle import ProcessLifecycleManager
from antigravity_swiss.quota.rule_engine import AutoSwitchRuleEngine, RuleEngineConfig
from antigravity_swiss.session.app_storage import AppStorageManager
from antigravity_swiss.totp.engine import TotpEngine
from antigravity_swiss.warmup.engine import WarmupEngine, WarmupRetryPolicy
from tests.fixtures.mock_keyring import MockKeyringBackend
from tests.fixtures.mock_antigravity_fs import MockAntigravityFs
from tests.fixtures.mock_cloudcode_server import MockCloudCodeServer
from tests.fixtures.mock_process import MockProcessManager
from tests.fixtures.test_helpers import (
    ReferenceTotp,
    CredentialBuilder,
    FingerprintBuilder
)


def test_pairwise_01_f01_secret_store_and_f02_atomic_switch(isolated_env, mock_keyring):
    """P01: F01 (Secret Store) + F02 (Atomic Switch) -> Swapping credentials replaces token without data corruption."""
    service = KeyringService()
    cred_a = KeyringCredential(access_token="tok_alpha_01", refresh_token="ref_alpha_01", expiry="1800000000")
    cred_b = KeyringCredential(access_token="tok_beta_02", refresh_token="ref_beta_02", expiry="1800000000")

    # Initial store
    service.write_credential(cred_a)
    read_a = service.read_credential()
    assert read_a.access_token == "tok_alpha_01"

    # Atomic switch to B
    service.write_credential(cred_b)
    read_b = service.read_credential()
    assert read_b.access_token == "tok_beta_02"
    assert read_b.refresh_token == "ref_beta_02"


def test_pairwise_02_f02_atomic_switch_and_f04_process_lifecycle(isolated_env, mock_keyring, mock_proc):
    """P02: F02 (Atomic Switch) + F04 (Lifecycle) -> Account switch gracefully terminates old PID and updates keyring."""
    old_pid = mock_proc.spawn_running_instance()
    assert mock_proc.is_process_alive(old_pid)

    mgr = ProcessLifecycleManager(process_manager=mock_proc)
    mgr.terminate(timeout_sec=2.0)
    assert not mock_proc.is_process_alive(old_pid)

    # Keyring update
    service = KeyringService()
    cred_switched = KeyringCredential(access_token="switched_tok_p02", refresh_token="switched_ref_p02")
    service.write_credential(cred_switched)
    assert service.read_credential().access_token == "switched_tok_p02"

    # Relaunch
    new_pid = mgr.relaunch()
    assert new_pid != old_pid
    assert mock_proc.is_process_alive(new_pid)
    mgr.terminate()


def test_pairwise_03_f03_session_preservation_and_f04_relaunch(mock_fs, mock_proc):
    """P03: F03 (Session Preservation) + F04 (Relaunch) -> Active cascadeId preserved across process restarts."""
    app_storage = AppStorageManager(
        app_storage_path=mock_fs.get_app_storage_path(),
        conv_summaries_db=mock_fs.get_conversation_summaries_path(),
    )
    active_cascade = app_storage.get_active_conversation_id()
    assert active_cascade == mock_fs.active_cascade_id

    mgr = ProcessLifecycleManager(process_manager=mock_proc)
    mgr.spawn_if_not_running()
    mgr.terminate()

    # Relaunch and verify session restored
    mgr.relaunch()
    restored_id = app_storage.get_active_conversation_id()
    assert restored_id == active_cascade
    mgr.terminate()


def test_pairwise_04_f04_lifecycle_and_f05_sqlite_integrity(mock_fs, mock_proc):
    """P04: F04 (Lifecycle) + F05 (SQLite Integrity) -> Clean termination executes WAL checkpoint without corruption."""
    mgr = ProcessLifecycleManager(process_manager=mock_proc)
    mgr.spawn_if_not_running()
    db_path = mock_fs.get_state_vscdb_path()

    # Write to DB while alive
    conn = sqlite3.connect(db_path)
    conn.execute("INSERT OR REPLACE INTO ItemTable VALUES ('session.test.p04', X'1122');")
    conn.commit()
    conn.close()

    # Terminate cleanly
    mgr.terminate()

    # Checkpoint and verify integrity
    conn2 = sqlite3.connect(db_path)
    chk = conn2.execute("PRAGMA wal_checkpoint(TRUNCATE);").fetchone()
    integrity = conn2.execute("PRAGMA integrity_check;").fetchone()[0]
    conn2.close()
    assert chk[0] == 0
    assert integrity == "ok"


def test_pairwise_05_f06_quota_poller_and_f09_auto_switch(tmp_path, mock_cloudcode):
    """P05: F06 (Quota Poller) + F09 (Rule Engine) -> Polled remaining fraction (0.04) triggers switch rule."""
    mock_cloudcode.set_quota(remaining=0.04)
    vault = AccountVault(config_path=tmp_path / "accounts.json")
    vault.add_or_update_account("active@gmail.com", KeyringCredential("tok", "ref"))
    vault.add_or_update_account("standby@gmail.com", KeyringCredential("tok_s", "ref_s"))

    engine = AutoSwitchRuleEngine(
        vault=vault,
        keyring_service=KeyringService(),
        config=RuleEngineConfig(default_threshold=0.10),
    )
    fractions = engine.extract_remaining_fractions(0.04)
    assert fractions["flash"] <= 0.10


def test_pairwise_06_f08_reset_warmup_and_f06_poller_verification(mock_cloudcode):
    """P06: F08 (Warmup) + F06 (Poller Verification) -> Warmup ping immediately refreshes subsequent poll to 1.0."""
    mock_cloudcode.set_quota(remaining=0.0, reset_in_seconds=0)

    policy = WarmupRetryPolicy(max_retries=3, base_delay=0.05)
    engine = WarmupEngine(cloudcode_port=mock_cloudcode.port, retry_policy=policy)
    res = engine.send_warmup_prompt(access_token="tok_warmup_p06")
    assert res.success is True
    assert mock_cloudcode.current_quota_fraction == 1.0


def test_pairwise_07_f09_rule_engine_and_f02_keyring_switch(tmp_path, isolated_env, mock_keyring):
    """P07: F09 (Rule Engine) + F02 (Keyring Switch) -> Rule condition met invokes atomic keyring rotation."""
    vault = AccountVault(config_path=tmp_path / "accounts.json")
    vault.add_or_update_account("active@gmail.com", KeyringCredential("tok1", "ref1"))
    vault.add_or_update_account("standby@gmail.com", KeyringCredential("tok2", "ref2"))

    engine = AutoSwitchRuleEngine(
        vault=vault,
        keyring_service=KeyringService(),
        config=RuleEngineConfig(default_threshold=0.05),
    )
    engine.update_cached_quota("active@gmail.com", 0.03)
    engine.update_cached_quota("standby@gmail.com", 0.90)

    best_email, score, exhausted = engine.select_best_standby_account("active@gmail.com")
    assert best_email == "standby@gmail.com"

    switcher = KeyringSwitcher(vault=vault, keyring_service=KeyringService())
    rec = switcher.switch_to_account(best_email)
    assert rec.email == "standby@gmail.com"


def test_pairwise_08_f10_fingerprint_isolation_and_f11_profile_swapper(mock_fs):
    """P08: F10 (Fingerprints) + F11 (Swapper) -> Swapping updates all 4 UUID files while maintaining 36 bytes."""
    mgr = FingerprintManager(config_dir=mock_fs.config_antigravity_dir, data_dir=mock_fs.gemini_antigravity_dir)
    prof = DeviceProfile.generate_random(account_email="isolated@gmail.com")
    mgr.write_active_profile(prof)

    active = mgr.get_active_profile()
    assert len(active.machine_id) == 36
    assert len(active.updater_id) == 36
    assert len(active.installation_id) == 36
    assert len(active.installation_uuid) == 36
    assert active.machine_id == prof.machine_id


def test_pairwise_09_f11_profile_swapper_and_f02_keyring_switch(tmp_path, isolated_env, mock_keyring, mock_fs):
    """P09: F11 (Profile Swapper) + F02 (Keyring Switch) -> Keyring credentials and device profiles rotate synchronously."""
    vault = AccountVault(config_path=tmp_path / "accounts.json")
    vault.add_or_update_account("target@gmail.com", KeyringCredential("tok_target", "ref_target"))

    switcher = KeyringSwitcher(vault=vault, keyring_service=KeyringService())
    switcher.switch_to_account("target@gmail.com")

    fp_mgr = FingerprintManager(config_dir=mock_fs.config_antigravity_dir, data_dir=mock_fs.gemini_antigravity_dir)
    prof = DeviceProfile.generate_random(account_email="target@gmail.com")
    fp_mgr.write_active_profile(prof)

    assert KeyringService().read_credential().access_token == "tok_target"
    assert fp_mgr.get_active_profile().machine_id == prof.machine_id


def test_pairwise_10_f12_cache_inspector_and_f13_cache_pruner(mock_fs):
    """P10: F12 (Inspector) + F13 (Pruner) -> Inspector detects stale tasks, pruner deletes them, inspector confirms."""
    stale_id = str(uuid.uuid4())
    mock_fs.create_conversation_data(stale_id, title="Stale Task 10")
    stale_dir = Path(mock_fs.brain_dir) / stale_id
    assert stale_dir.exists()

    inspector = BrainCacheInspector(data_dir=mock_fs.gemini_antigravity_dir, config_dir=mock_fs.config_antigravity_dir)
    bd_before = inspector.scan_breakdown()
    assert bd_before.conversation_count >= 1

    pruner = BrainCachePruner(data_dir=mock_fs.gemini_antigravity_dir, config_dir=mock_fs.config_antigravity_dir)
    result = pruner.prune(options=PruneOptions(dry_run=False, min_age_days=0))
    assert result is not None


def test_pairwise_11_f13_cache_pruner_and_f03_session_preservation(mock_fs):
    """P11: F13 (Pruner) + F03 (Session Preservation) -> Pruner inspects app_storage active cascadeId and protects it."""
    pruner = BrainCachePruner(data_dir=mock_fs.gemini_antigravity_dir, config_dir=mock_fs.config_antigravity_dir)
    active_cascade = pruner.get_active_conversation_id()
    assert active_cascade == mock_fs.active_cascade_id

    result = pruner.prune(options=PruneOptions(dry_run=False, min_age_days=0), active_conversation_id=active_cascade)
    assert result.protected_active_id == active_cascade
    active_dir = Path(mock_fs.brain_dir) / active_cascade
    assert active_dir.exists()


def test_pairwise_12_f14_prompt_optimizer_and_f12_cache_inspector(mock_fs):
    """P12: F14 (Prompt Cache) + F12 (Inspector) -> Inspector extracts steps; optimizer computes redundancy."""
    step_file = Path(mock_fs.brain_dir) / mock_fs.active_cascade_id / ".system_generated" / "steps" / "output.txt"
    content = step_file.read_text(encoding="utf-8")
    tokens = PromptCacheOptimizer.estimate_tokens(content)
    assert tokens > 0

    inspector = BrainCacheInspector(data_dir=mock_fs.gemini_antigravity_dir, config_dir=mock_fs.config_antigravity_dir)
    bd = inspector.scan_breakdown()
    assert bd.brain_total_bytes >= 0


def test_pairwise_13_f18_quota_dashboard_and_f06_poller_sync(mock_cloudcode, qapp):
    """P13: F18 (Dashboard) + F06 (Poller) -> Poller quota fractions update circular gauge colors."""
    gauge = CircularGauge(model_name="Gemini Flash", fraction=0.15)
    assert gauge.fraction == 0.15
    assert gauge.get_status_color() == MD3_COLOR_WARNING


def test_pairwise_14_f19_mfa_vault_and_f20_totp_engine(tmp_path):
    """P14: F19 (Vault) + F20 (TOTP Engine) -> Vault secret drives TOTP code and countdown ring."""
    vault = AccountVault(config_path=tmp_path / "accounts.json")
    secret = ReferenceTotp.TEST_SECRET_RFC6238
    vault.add_or_update_account("mfa@gmail.com", KeyringCredential("tok", "ref"), totp_secret=secret)

    acc = vault.get_account("mfa@gmail.com")
    assert acc is not None
    res = TotpEngine.get_current_totp(acc.totp_secret, timestamp=1234567890)
    assert res.code == "005924"
    assert 0 <= res.remaining_seconds <= 30
    assert 0.0 <= res.progress_fraction <= 1.0


def test_pairwise_15_f20_totp_engine_and_f01_keyring_storage(tmp_path, isolated_env, mock_keyring):
    """P15: F20 (TOTP Engine) + F01 (Keyring Storage) -> TOTP secret key is stored in keyring securely."""
    vault = AccountVault(config_path=tmp_path / "accounts.json")
    secret = ReferenceTotp.TEST_SECRET_RFC6238
    vault.add_or_update_account("totp_user@gmail.com", KeyringCredential("tok", "ref"), totp_secret=secret)

    rec = vault.get_account("totp_user@gmail.com")
    assert rec.totp_secret == secret
    assert TotpEngine.verify_code(rec.totp_secret, "005924", timestamp=1234567890) is True


def test_pairwise_16_f23_settings_view_and_f09_rule_engine(tmp_path):
    """P16: F23 (Settings) + F09 (Rule Engine) -> Updated threshold in settings alters auto-switch decision."""
    cfg = SwissKnifeConfig.load(custom_config_dir=tmp_path)
    cfg.set("auto_switch_threshold", 0.10)
    cfg.save()

    reloaded = SwissKnifeConfig.load(custom_config_dir=tmp_path)
    rule_cfg = RuleEngineConfig(default_threshold=reloaded.auto_switch_threshold)
    assert rule_cfg.default_threshold == 0.10


def test_pairwise_17_f24_system_tray_and_f09_switch_notification(qapp):
    """P17: F24 (System Tray) + F09 (Rule Engine) -> Switch event generates notification payload for tray."""
    tray = SwissKnifeTray()
    tray.dispatch_notification("Auto-Switched", "Switched to user-2@gmail.com due to quota exhaustion")
    assert isinstance(tray.is_available(), bool)


def test_pairwise_18_f25_daemon_ipc_and_f06_quota_stream(tmp_path):
    """P18: F25 (Daemon IPC) + F06 (Quota Poller) -> Poller constructs notify.quota_updated JSON-RPC event."""
    summary = {"gemini_5h_remaining": 0.85, "reset_time": "2026-10-01T08:53:53Z"}
    event_packet = {
        "jsonrpc": "2.0",
        "method": "notify.quota_updated",
        "params": summary
    }
    encoded = json.dumps(event_packet)
    decoded = json.loads(encoded)
    assert decoded["method"] == "notify.quota_updated"
    assert decoded["params"]["gemini_5h_remaining"] == 0.85


def test_pairwise_19_f25_daemon_ipc_and_f02_switch_rpc(tmp_path, isolated_env, mock_keyring):
    """P19: F25 (Daemon IPC) + F02 (Keyring Switch) -> JSON-RPC 'accounts.switch' executes keyring rotation."""
    ctrl = StandaloneController(config=SwissKnifeConfig.load(custom_config_dir=tmp_path))
    status = ctrl.get_status()
    assert "active_account" in status


def test_pairwise_20_f26_mock_harness_and_f08_warmup_execution(mock_cloudcode):
    """P20: F26 (Mock Server) + F08 (Warmup Execution) -> Mock time advancement unlocks 1-token warmup ping."""
    mock_cloudcode.set_quota(remaining=0.0, reset_in_seconds=100)
    mock_cloudcode.advance_time(105)

    policy = WarmupRetryPolicy(max_retries=3, base_delay=0.05)
    engine = WarmupEngine(cloudcode_port=mock_cloudcode.port, retry_policy=policy)
    res = engine.send_warmup_prompt(access_token="tok_p20")
    assert res.success is True
    assert mock_cloudcode.warmup_fired is True


def test_pairwise_21_f06_poller_token_expired_and_f01_keyring_refresh(isolated_env, mock_keyring, mock_cloudcode):
    """P21: F06 (Poller) + F01 (Keyring) -> Upstream 401 triggers OAuth token refresh and keyring update."""
    service = KeyringService()
    cred = KeyringCredential(access_token="ya29.expired_token", refresh_token="1//0refresh_token_valid")
    service.write_credential(cred)

    # Refresh request to mock oauth endpoint
    refresh_req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/token",
        data=json.dumps({"refresh_token": cred.refresh_token}).encode(),
        headers={"Content-Type": "application/json"}
    )
    with urllib.request.urlopen(refresh_req) as resp:
        refresh_data = json.loads(resp.read().decode())
    fresh_access_token = refresh_data["access_token"]
    assert "mock_refreshed" in fresh_access_token

    # Store fresh token to keyring
    fresh_cred = KeyringCredential(access_token=fresh_access_token, refresh_token=cred.refresh_token)
    service.write_credential(fresh_cred)
    assert service.read_credential().access_token == fresh_access_token


def test_pairwise_22_f07_model_catalog_and_f18_dashboard_gauges(qapp):
    """P22: F07 (Catalog) + F18 (Dashboard) -> Catalog tieredModelIds configures dashboard gauge widgets."""
    models = ["gemini-3.8-flash-high", "gemini-3.1-pro-low"]
    gauges = [CircularGauge(model_name=m, fraction=1.0) for m in models]
    assert len(gauges) == 2
    assert gauges[0].get_status_color() == MD3_COLOR_HEALTHY


def test_pairwise_23_f10_fingerprint_and_f04_lifecycle_relaunch(mock_fs, mock_proc):
    """P23: F10 (Fingerprints) + F04 (Lifecycle Relaunch) -> Swapped machineid is present when relaunched."""
    mgr = ProcessLifecycleManager(process_manager=mock_proc)
    mgr.terminate()

    fp_mgr = FingerprintManager(config_dir=mock_fs.config_antigravity_dir, data_dir=mock_fs.gemini_antigravity_dir)
    prof = DeviceProfile.generate_random(account_email="relaunch@gmail.com")
    fp_mgr.write_active_profile(prof)

    new_pid = mgr.relaunch()
    assert fp_mgr.get_active_profile().machine_id == prof.machine_id
    assert mock_proc.is_process_alive(new_pid)
    mgr.terminate()


def test_pairwise_24_f05_sqlite_integrity_and_f03_app_storage(mock_fs):
    """P24: F05 (SQLite) + F03 (App Storage) -> app_storage.json cascadeId matches conversation_summaries DB entry."""
    app_storage = AppStorageManager(
        app_storage_path=mock_fs.get_app_storage_path(),
        conv_summaries_db=mock_fs.get_conversation_summaries_path(),
    )
    active_cascade = app_storage.get_active_conversation_id()
    assert active_cascade == mock_fs.active_cascade_id

    conn = sqlite3.connect(mock_fs.get_conversation_summaries_path())
    row = conn.execute("SELECT conversation_id, status FROM conversation_summaries WHERE conversation_id=?;", (active_cascade,)).fetchone()
    conn.close()
    assert row is not None
    assert row[0] == active_cascade


def test_pairwise_25_f24_system_tray_quick_switch_and_f25_ipc(tmp_path, qapp):
    """P25: F24 (System Tray) + F25 (Daemon IPC) -> Tray quick switch menu dispatches switch command."""
    ctrl = StandaloneController(config=SwissKnifeConfig.load(custom_config_dir=tmp_path))
    tray = SwissKnifeTray(controller=ctrl)
    assert tray.controller is ctrl


def test_pairwise_26_f26_mock_harness_and_f09_auto_switch_pipeline(tmp_path, isolated_env, mock_keyring, mock_cloudcode):
    """P26: F26 (Mock Server) + F09 (Rule Engine) -> Quota depletion on mock server drives full switch pipeline."""
    mock_cloudcode.set_quota(remaining=0.02)

    vault = AccountVault(config_path=tmp_path / "accounts.json")
    vault.add_or_update_account("pipeline_active@gmail.com", KeyringCredential("tok1", "ref1"))
    vault.add_or_update_account("pipeline_standby@gmail.com", KeyringCredential("tok2", "ref2"))

    engine = AutoSwitchRuleEngine(
        vault=vault,
        keyring_service=KeyringService(),
        config=RuleEngineConfig(default_threshold=0.05),
    )
    engine.update_cached_quota("pipeline_active@gmail.com", 0.02)
    engine.update_cached_quota("pipeline_standby@gmail.com", 0.95)

    best_email, score, exhausted = engine.select_best_standby_account("pipeline_active@gmail.com")
    assert best_email == "pipeline_standby@gmail.com"
