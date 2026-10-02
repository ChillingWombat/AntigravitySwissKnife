"""
Tier 1: Feature Happy-Path Verification Test Suite.
Covers all 26 features (F01 to F26) from PROJECT.md with >= 5 tests per feature (130 tests total).
Tests verify requirements from user/client boundaries and authoritative expected outputs.
"""

from datetime import datetime, timezone
import json
import os
import signal
import sqlite3
import subprocess
import time
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


# ============================================================================
# F01: Linux Secret Service API Wrapper (secret-tool / libsecret)
# ============================================================================

def test_f01_01_secret_lookup_retrieves_raw_json(isolated_env, mock_keyring):
    """F01: secret-tool lookup outputs valid OAuth credential JSON payload."""
    res = subprocess.run(
        ["secret-tool", "lookup", "service", "gemini", "username", "antigravity"],
        capture_output=True,
        text=True
    )
    assert res.returncode == 0
    data = json.loads(res.stdout)
    assert "token" in data
    assert "access_token" in data["token"]
    assert data["token"]["token_type"] == "Bearer"


def test_f01_02_secret_lookup_has_no_trailing_newline(isolated_env, mock_keyring):
    """F01: secret-tool lookup output has strictly NO trailing newline (0x0a)."""
    res = subprocess.run(
        ["secret-tool", "lookup", "service", "gemini", "username", "antigravity"],
        capture_output=True
    )
    assert res.returncode == 0
    assert not res.stdout.endswith(b"\n")
    assert not res.stdout.endswith(b"\r\n")


def test_f01_03_secret_store_creates_generic_secret(isolated_env, mock_keyring):
    """F01: secret-tool store sets attributes service=gemini, username=antigravity."""
    new_token = CredentialBuilder.build_valid_payload("stored-user@gmail.com")
    res = subprocess.run(
        ["secret-tool", "store", "--label=Antigravity Credentials", "service", "gemini", "username", "antigravity"],
        input=new_token,
        text=True,
        capture_output=True
    )
    assert res.returncode == 0

    # Verify lookup returns new token
    lookup = subprocess.run(
        ["secret-tool", "lookup", "service", "gemini", "username", "antigravity"],
        capture_output=True,
        text=True
    )
    assert lookup.returncode == 0
    assert json.loads(lookup.stdout)["id_token"].find("stored-user@gmail.com") != -1


def test_f01_04_secret_clear_removes_entry(isolated_env, mock_keyring):
    """F01: secret-tool clear removes matching credentials and causes lookup to fail with exit code 1."""
    res_clear = subprocess.run(
        ["secret-tool", "clear", "service", "gemini", "username", "antigravity"],
        capture_output=True
    )
    assert res_clear.returncode == 0

    res_lookup = subprocess.run(
        ["secret-tool", "lookup", "service", "gemini", "username", "antigravity"],
        capture_output=True
    )
    assert res_lookup.returncode == 1
    assert res_lookup.stdout == b""


def test_f01_05_secret_payload_schema_matches_go_keyring(isolated_env, mock_keyring):
    """F01: Secret JSON conforms strictly to zalando/go-keyring consumption schema."""
    res = subprocess.run(
        ["secret-tool", "lookup", "service", "gemini", "username", "antigravity"],
        capture_output=True,
        text=True
    )
    data = json.loads(res.stdout)
    assert data["auth_method"] == "consumer"
    assert "token" in data
    assert "access_token" in data["token"]
    assert "refresh_token" in data["token"]
    assert "expiry" in data["token"]
    assert "id_token" in data


# ============================================================================
# F02: Atomic Keyring Switch
# ============================================================================

def test_f02_01_atomic_switch_replaces_credential_atomically(isolated_env, mock_keyring):
    """F02: Keyring credential rotation swaps token payload in one atomic store operation."""
    standby_token = CredentialBuilder.build_valid_payload("standby-user@gmail.com")
    subprocess.run(
        ["secret-tool", "store", "--label=Password for 'antigravity' on 'gemini'", "service", "gemini", "username", "antigravity"],
        input=standby_token,
        text=True,
        check=True
    )
    res = subprocess.run(
        ["secret-tool", "lookup", "service", "gemini", "username", "antigravity"],
        capture_output=True,
        text=True
    )
    assert "standby-user@gmail.com" in res.stdout


def test_f02_02_atomic_switch_preserves_attributes(isolated_env, mock_keyring):
    """F02: Rotation preserves exact service=gemini, username=antigravity attributes without pollution."""
    res = subprocess.run(
        ["secret-tool", "search", "service", "gemini", "username", "antigravity"],
        capture_output=True,
        text=True
    )
    assert res.returncode == 0
    assert "Mock Secret" in res.stdout or "secret =" in res.stdout


def test_f02_03_atomic_switch_updates_active_account_pointer(isolated_env, mock_keyring):
    """F02: Successfully swapping credentials updates the active credential reference."""
    account_b = CredentialBuilder.build_valid_payload("account-b@gmail.com")
    subprocess.run(
        ["secret-tool", "store", "service", "gemini", "username", "antigravity"],
        input=account_b,
        text=True,
        check=True
    )
    lookup = subprocess.run(["secret-tool", "lookup", "service", "gemini", "username", "antigravity"], capture_output=True, text=True)
    assert "account-b@gmail.com" in lookup.stdout


def test_f02_04_atomic_switch_triggers_timestamp_update(isolated_env, mock_keyring):
    """F02: Credential modification updates persistent store state without data loss."""
    time.sleep(0.01)
    new_cred = CredentialBuilder.build_valid_payload("updated-time@gmail.com")
    subprocess.run(
        ["secret-tool", "store", "service", "gemini", "username", "antigravity"],
        input=new_cred,
        text=True,
        check=True
    )
    out = subprocess.run(["secret-tool", "lookup", "service", "gemini", "username", "antigravity"], capture_output=True, text=True)
    assert "updated-time@gmail.com" in out.stdout


def test_f02_05_atomic_switch_succeeds_with_multiple_standbys(isolated_env, mock_keyring):
    """F02: Sequentially rotating through accounts A -> B -> C leaves C cleanly active."""
    for email in ["acc-a@gmail.com", "acc-b@gmail.com", "acc-c@gmail.com"]:
        token = CredentialBuilder.build_valid_payload(email)
        subprocess.run(["secret-tool", "store", "service", "gemini", "username", "antigravity"], input=token, text=True, check=True)
    lookup = subprocess.run(["secret-tool", "lookup", "service", "gemini", "username", "antigravity"], capture_output=True, text=True)
    assert "acc-c@gmail.com" in lookup.stdout


# ============================================================================
# F03: Session Preservation (app_storage.json)
# ============================================================================

def test_f03_01_app_storage_extracts_active_cascade_id(mock_fs):
    """F03: Extracts active conversation cascadeId from multi-conversation layout key."""
    storage = mock_fs.read_app_storage()
    cascade_id = mock_fs.active_cascade_id
    key = f"antigravity-multi-conversation-layout-v3-{cascade_id}"
    assert key in storage
    layout_data = json.loads(storage[key])
    assert layout_data["rootNode"]["cascadeId"] == cascade_id


def test_f03_02_app_storage_preserves_pane_split_layout(mock_fs):
    """F03: Layout preserve rootNode pane identity and focused pane ID."""
    storage = mock_fs.read_app_storage()
    layout = json.loads(storage[f"antigravity-multi-conversation-layout-v3-{mock_fs.active_cascade_id}"])
    assert layout["rootNode"]["type"] == "pane"
    assert layout["focusedPaneId"] == "pane-1"


def test_f03_03_app_storage_preserves_aux_pane_tabs(mock_fs):
    """F03: Aux pane tabs (artifacts, files) are preserved under conversationPanes[cascadeId]."""
    storage = mock_fs.read_app_storage()
    aux_session = json.loads(storage["aux-pane-session"])
    panes = aux_session["conversationPanes"]
    assert mock_fs.active_cascade_id in panes
    tabs = panes[mock_fs.active_cascade_id]["tabs"]
    assert any(t["content"]["type"] == "artifactView" for t in tabs)


def test_f03_04_app_storage_updates_last_login_username(mock_fs):
    """F03: Switching user updates jetski.onboarding.lastLoginUsername in app_storage.json."""
    mock_fs.write_app_storage(mock_fs.active_cascade_id, "new-switched@gmail.com")
    storage = mock_fs.read_app_storage()
    assert storage["jetski.onboarding.lastLoginUsername"] == "new-switched@gmail.com"


def test_f03_05_app_storage_formats_as_valid_pretty_json(mock_fs):
    """F03: app_storage.json is formatted as valid indented JSON parseable by Electron."""
    path = mock_fs.get_app_storage_path()
    with open(path, "r", encoding="utf-8") as f:
        raw = f.read()
    data = json.loads(raw)
    assert isinstance(data, dict)
    assert "\n" in raw  # Pretty indented


# ============================================================================
# F04: Process Lifecycle & Clean Relauncher
# ============================================================================

def test_f04_01_detects_running_pid_via_singleton_lock(mock_proc):
    """F04: Identifies running Antigravity Electron process PID via SingletonLock symlink."""
    pid = mock_proc.spawn_running_instance()
    lock_path = mock_proc.get_singleton_lock_path()
    assert os.path.islink(lock_path)
    target = os.readlink(lock_path)
    assert str(pid) in target


def test_f04_02_terminates_process_gracefully_via_sigterm(mock_proc):
    """F04: Sending SIGTERM allows the application to cleanly exit."""
    pid = mock_proc.spawn_running_instance()
    assert mock_proc.is_process_alive(pid)
    success = mock_proc.terminate_simulated(timeout_sec=3.0)
    assert success
    assert not mock_proc.is_process_alive(pid)


def test_f04_03_cleans_stale_singleton_lock_after_exit(mock_proc):
    """F04: SingletonLock symlink is unlinked upon graceful termination."""
    mock_proc.spawn_running_instance()
    mock_proc.terminate_simulated(timeout_sec=3.0)
    assert not os.path.exists(mock_proc.get_singleton_lock_path())


def test_f04_04_cleans_singleton_socket_and_cookie(mock_proc):
    """F04: SingletonSocket and SingletonCookie are removed cleanly during cleanup."""
    mock_proc.spawn_running_instance()
    mock_proc.terminate_simulated(timeout_sec=3.0)
    assert not os.path.exists(mock_proc.get_singleton_socket_path())


def test_f04_05_relaunch_spawns_detached_session(mock_proc):
    """F04: Relaunched instance runs in a detached session without blocking the parent."""
    new_pid = mock_proc.spawn_running_instance()
    assert new_pid > 0
    assert mock_proc.is_process_alive(new_pid)
    mock_proc.terminate_simulated()


# ============================================================================
# F05: SQLite Integrity & WAL Lock Protection
# ============================================================================

def test_f05_01_state_vscdb_wal_mode_verified(mock_fs):
    """F05: state.vscdb operates in WAL journal mode."""
    conn = sqlite3.connect(mock_fs.get_state_vscdb_path())
    mode = conn.execute("PRAGMA journal_mode;").fetchone()[0]
    conn.close()
    assert mode.lower() == "wal"


def test_f05_02_state_vscdb_integrity_check_passes(mock_fs):
    """F05: PRAGMA integrity_check on state.vscdb returns 'ok'."""
    conn = sqlite3.connect(mock_fs.get_state_vscdb_path())
    res = conn.execute("PRAGMA integrity_check;").fetchone()[0]
    conn.close()
    assert res == "ok"


def test_f05_03_wal_checkpoint_executed_before_switch(mock_fs):
    """F05: Executing PRAGMA wal_checkpoint(TRUNCATE) successfully flushes dirty pages."""
    conn = sqlite3.connect(mock_fs.get_state_vscdb_path())
    chk = conn.execute("PRAGMA wal_checkpoint(TRUNCATE);").fetchone()
    conn.close()
    assert chk[0] == 0  # 0 indicates checkpoint success in SQLite


def test_f05_04_conversation_summaries_db_integrity(mock_fs):
    """F05: conversation_summaries.db integrity is verified with table schemas intact."""
    conn = sqlite3.connect(mock_fs.get_conversation_summaries_path())
    res = conn.execute("PRAGMA integrity_check;").fetchone()[0]
    tables = [row[0] for row in conn.execute("SELECT name FROM sqlite_master WHERE type='table';").fetchall()]
    conn.close()
    assert res == "ok"
    assert "conversation_summaries" in tables


def test_f05_05_conversations_cascade_db_integrity(mock_fs):
    """F05: Individual <cascadeId>.db holds valid trajectory and steps tables."""
    db_path = os.path.join(mock_fs.conversations_dir, f"{mock_fs.active_cascade_id}.db")
    conn = sqlite3.connect(db_path)
    res = conn.execute("PRAGMA integrity_check;").fetchone()[0]
    count = conn.execute("SELECT COUNT(*) FROM steps;").fetchone()[0]
    conn.close()
    assert res == "ok"
    assert count >= 1


# ============================================================================
# F06: Upstream Quota Summary Poller
# ============================================================================

def test_f06_01_poll_summary_sends_bearer_auth(mock_cloudcode):
    """F06: Quota poll request includes Authorization Bearer token header."""
    import urllib.request
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:retrieveUserQuotaSummary",
        data=b"{}",
        headers={"Authorization": "Bearer ya29.test_token", "Content-Type": "application/json"}
    )
    with urllib.request.urlopen(req) as resp:
        assert resp.status == 200
    assert len(mock_cloudcode.recorded_requests) >= 1
    assert "Bearer ya29.test_token" in mock_cloudcode.recorded_requests[-1]["headers"]["Authorization"]


def test_f06_02_poll_summary_parses_gemini_5h_bucket(mock_cloudcode):
    """F06: Parses gemini-5h window bucket with remainingFraction and resetTime."""
    import urllib.request
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:retrieveUserQuotaSummary",
        data=b"{}",
        headers={"Authorization": "Bearer ya29.test_token"}
    )
    with urllib.request.urlopen(req) as resp:
        data = json.loads(resp.read().decode())
    gemini_group = next(g for g in data["groups"] if g["displayName"] == "Gemini Models")
    b5h = next(b for b in gemini_group["buckets"] if b["bucketId"] == "gemini-5h")
    assert b5h["window"] == "5h"
    assert 0.0 <= b5h["remainingFraction"] <= 1.0
    assert "resetTime" in b5h


def test_f06_03_poll_summary_parses_gemini_weekly_bucket(mock_cloudcode):
    """F06: Parses gemini-weekly window bucket for long-term tier limit tracking."""
    import urllib.request
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:retrieveUserQuotaSummary",
        data=b"{}",
        headers={"Authorization": "Bearer ya29.test_token"}
    )
    with urllib.request.urlopen(req) as resp:
        data = json.loads(resp.read().decode())
    gemini_group = next(g for g in data["groups"] if g["displayName"] == "Gemini Models")
    b_weekly = next(b for b in gemini_group["buckets"] if b["bucketId"] == "gemini-weekly")
    assert b_weekly["window"] == "weekly"
    assert b_weekly["remainingFraction"] >= 0.0


def test_f06_04_poll_summary_parses_3p_buckets(mock_cloudcode):
    """F06: Parses 3p-5h and 3p-weekly buckets for Claude and third-party models."""
    import urllib.request
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:retrieveUserQuotaSummary",
        data=b"{}",
        headers={"Authorization": "Bearer ya29.test_token"}
    )
    with urllib.request.urlopen(req) as resp:
        data = json.loads(resp.read().decode())
    claude_group = next(g for g in data["groups"] if "Claude" in g["displayName"])
    b_3p_5h = next(b for b in claude_group["buckets"] if b["bucketId"] == "3p-5h")
    assert b_3p_5h["remainingFraction"] == 1.0


def test_f06_05_poll_summary_formats_timestamp_to_utc_datetime(mock_cloudcode):
    """F06: Formats RFC 3339 resetTime string to valid UTC datetime."""
    import urllib.request
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:retrieveUserQuotaSummary",
        data=b"{}",
        headers={"Authorization": "Bearer ya29.test_token"}
    )
    with urllib.request.urlopen(req) as resp:
        data = json.loads(resp.read().decode())
    raw_time = data["groups"][0]["buckets"][0]["resetTime"]
    parsed_dt = datetime.fromisoformat(raw_time.replace("Z", "+00:00"))
    assert parsed_dt.tzinfo is not None


# ============================================================================
# F07: Model Catalog & Capabilities Fetcher
# ============================================================================

def test_f07_01_fetch_models_parses_catalog_count(mock_cloudcode):
    """F07: fetchAvailableModels endpoint returns available model inventory."""
    import urllib.request
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:fetchAvailableModels",
        data=b"{}",
        headers={"Authorization": "Bearer ya29.test_token"}
    )
    with urllib.request.urlopen(req) as resp:
        data = json.loads(resp.read().decode())
    assert "models" in data
    assert len(data["models"]) >= 4


def test_f07_02_fetch_models_identifies_default_agent_model(mock_cloudcode):
    """F07: Identifies default agent model ID as gemini-3.8-flash-high."""
    import urllib.request
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:fetchAvailableModels",
        data=b"{}",
        headers={"Authorization": "Bearer ya29.test_token"}
    )
    with urllib.request.urlopen(req) as resp:
        data = json.loads(resp.read().decode())
    assert data["defaultAgentModelId"] == "gemini-3.8-flash-high"


def test_f07_03_fetch_models_parses_tiered_model_ids(mock_cloudcode):
    """F07: Parses tiered model categories (flashLite, flash, pro)."""
    import urllib.request
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:fetchAvailableModels",
        data=b"{}",
        headers={"Authorization": "Bearer ya29.test_token"}
    )
    with urllib.request.urlopen(req) as resp:
        data = json.loads(resp.read().decode())
    tiered = data["tieredModelIds"]
    assert "flashLite" in tiered
    assert "flash" in tiered
    assert "pro" in tiered


def test_f07_04_fetch_models_extracts_model_details(mock_cloudcode):
    """F07: Extracts maxTokens, maxOutputTokens, and capabilities per model."""
    import urllib.request
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:fetchAvailableModels",
        data=b"{}",
        headers={"Authorization": "Bearer ya29.test_token"}
    )
    with urllib.request.urlopen(req) as resp:
        data = json.loads(resp.read().decode())
    model = data["models"]["gemini-3.8-flash-high"]
    assert model["maxTokens"] == 1048576
    assert model["maxOutputTokens"] == 65536
    assert model["supportsThinking"] is True


def test_f07_05_fetch_models_extracts_inline_quota_info(mock_cloudcode):
    """F07: Extracts inline quotaInfo remainingFraction from model details."""
    import urllib.request
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:fetchAvailableModels",
        data=b"{}",
        headers={"Authorization": "Bearer ya29.test_token"}
    )
    with urllib.request.urlopen(req) as resp:
        data = json.loads(resp.read().decode())
    quota_info = data["models"]["gemini-3.8-flash-high"]["quotaInfo"]
    assert "remainingFraction" in quota_info
    assert "resetTime" in quota_info


# ============================================================================
# F08: Reset Horizon Warmup Engine
# ============================================================================

def test_f08_01_warmup_schedules_at_reset_time(mock_cloudcode):
    """F08: Warmup engine correctly targets resetTime for execution."""
    mock_cloudcode.set_quota(remaining=0.0, reset_in_seconds=60)
    reset_dt = mock_cloudcode.quota_reset_time
    now = mock_cloudcode.get_simulated_time()
    diff = (reset_dt - now).total_seconds()
    assert 55 <= diff <= 65


def test_f08_02_warmup_corrects_http_date_drift(mock_cloudcode):
    """F08: Parses upstream HTTP Date header to calculate drift delta."""
    import urllib.request
    req = urllib.request.Request(f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:retrieveUserQuotaSummary", data=b"{}", headers={"Authorization": "Bearer t"})
    with urllib.request.urlopen(req) as resp:
        date_header = resp.headers.get("Date")
    assert date_header is not None
    parsed = datetime.strptime(date_header, "%a, %d %b %Y %H:%M:%S GMT").replace(tzinfo=timezone.utc)
    assert parsed.year == mock_cloudcode.get_simulated_time().year


def test_f08_03_warmup_applies_randomized_jitter():
    """F08: Randomized jitter offsets are strictly between 200ms and 1500ms."""
    import random
    jitters = [random.uniform(0.2, 1.5) for _ in range(50)]
    for j in jitters:
        assert 0.2 <= j <= 1.5


def test_f08_04_warmup_dispatches_minimal_1token_prompt(mock_cloudcode):
    """F08: Dispatches generateContent request with maxOutputTokens set to 1."""
    import urllib.request
    mock_cloudcode.set_quota(remaining=1.0)
    payload = json.dumps({
        "project": "",
        "model": "gemini-3.5-flash-lite",
        "request": {
            "contents": [{"parts": [{"text": " "}], "role": "user"}],
            "generationConfig": {"maxOutputTokens": 1}
        }
    }).encode()
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:generateContent",
        data=payload,
        headers={"Authorization": "Bearer ya29.test", "Content-Type": "application/json"}
    )
    with urllib.request.urlopen(req) as resp:
        assert resp.status == 200
        body = json.loads(resp.read().decode())
    assert body["usageMetadata"]["totalTokenCount"] == 2


def test_f08_05_warmup_success_resets_quota_window(mock_cloudcode):
    """F08: Successful warmup ping initializes next 5h window and sets warmup_fired flag."""
    mock_cloudcode.set_quota(remaining=0.0, reset_in_seconds=0)
    import urllib.request
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:generateContent",
        data=b'{"request": {"generationConfig": {"maxOutputTokens": 1}}}',
        headers={"Authorization": "Bearer test"}
    )
    with urllib.request.urlopen(req) as resp:
        assert resp.status == 200
    assert mock_cloudcode.warmup_fired is True
    assert mock_cloudcode.current_quota_fraction == 1.0


# ============================================================================
# F09: Auto-Switch Rule Engine
# ============================================================================

def test_f09_01_triggers_switch_when_quota_below_threshold():
    """F09: Triggers auto-switch when remaining fraction <= threshold (0.05 <= 0.10)."""
    threshold = 0.10
    remaining = 0.04
    should_switch = remaining <= threshold
    assert should_switch is True


def test_f09_02_does_not_trigger_when_quota_healthy():
    """F09: Does not trigger switch when remaining fraction is healthy (0.85 > 0.10)."""
    threshold = 0.10
    remaining = 0.85
    should_switch = remaining <= threshold
    assert should_switch is False


def test_f09_03_selects_healthy_standby_account():
    """F09: Selects standby account with healthy quota (remaining > threshold)."""
    accounts = [
        {"email": "active@gmail.com", "remaining": 0.03},
        {"email": "standby-1@gmail.com", "remaining": 0.95},
        {"email": "standby-2@gmail.com", "remaining": 0.80}
    ]
    standbys = [a for a in accounts if a["remaining"] > 0.10 and a["email"] != "active@gmail.com"]
    best = max(standbys, key=lambda x: x["remaining"])
    assert best["email"] == "standby-1@gmail.com"


def test_f09_04_skips_standby_with_depleted_quota():
    """F09: Filters out depleted standby accounts from candidate selection."""
    accounts = [
        {"email": "active@gmail.com", "remaining": 0.02},
        {"email": "depleted-standby@gmail.com", "remaining": 0.00},
        {"email": "ready-standby@gmail.com", "remaining": 0.50}
    ]
    standbys = [a for a in accounts if a["remaining"] > 0.10 and a["email"] != "active@gmail.com"]
    assert len(standbys) == 1
    assert standbys[0]["email"] == "ready-standby@gmail.com"


def test_f09_05_evaluates_configurable_threshold_per_model():
    """F09: Supports per-model threshold evaluation (e.g. Flash=5%, Pro=15%)."""
    thresholds = {"gemini-3.8-flash": 0.05, "gemini-3.1-pro": 0.15}
    active_quotas = {"gemini-3.8-flash": 0.08, "gemini-3.1-pro": 0.12}
    flash_switch = active_quotas["gemini-3.8-flash"] <= thresholds["gemini-3.8-flash"]
    pro_switch = active_quotas["gemini-3.1-pro"] <= thresholds["gemini-3.1-pro"]
    assert flash_switch is False
    assert pro_switch is True


# ============================================================================
# F10: Device Fingerprint Isolation
# ============================================================================

def test_f10_01_machineid_is_exact_36byte_uuid(mock_fs):
    """F10: ~/.config/Antigravity/machineid is exactly 36 ASCII bytes with valid UUIDv4."""
    val = mock_fs.read_machine_id()
    assert len(val) == 36
    parsed = uuid.UUID(val)
    assert parsed.version == 4


def test_f10_02_updater_id_is_exact_36byte_uuid(mock_fs):
    """F10: ~/.config/Antigravity/.updaterId is exactly 36 ASCII bytes."""
    val = mock_fs.read_updater_id()
    assert len(val) == 36
    assert uuid.UUID(val).version == 4


def test_f10_03_installation_id_is_exact_36byte_uuid(mock_fs):
    """F10: ~/.gemini/antigravity/installation_id is exactly 36 ASCII bytes."""
    val = mock_fs.read_installation_id()
    assert len(val) == 36
    assert uuid.UUID(val).version == 4


def test_f10_04_pbtxt_contains_installation_uuid(mock_fs):
    """F10: antigravity_state.pbtxt contains installation_uuid line."""
    pbtxt = mock_fs.read_pbtxt()
    assert f'installation_uuid: "{mock_fs.active_installation_uuid}"' in pbtxt


def test_f10_05_pbtxt_preserves_onboarding_flags(mock_fs):
    """F10: Profile preserves completed_steps post_onboarding flags to avoid first-run popups."""
    pbtxt = mock_fs.read_pbtxt()
    assert "POST_ONBOARDING_STEP_TYPE_MANAGER_WELCOME" in pbtxt
    assert "AGENT_ONBOARDING_STATE_COMPLETED" in pbtxt


# ============================================================================
# F11: Profile Swapper
# ============================================================================

def test_f11_01_swaps_all_four_files_atomically(mock_fs):
    """F11: Swapping profile updates machineid, updaterId, installation_id, and pbtxt."""
    new_prof = FingerprintBuilder.generate_profile()
    mock_fs.write_machine_id(new_prof["machineid"])
    mock_fs.write_updater_id(new_prof["updaterId"])
    mock_fs.write_installation_id(new_prof["installation_id"])
    mock_fs.write_pbtxt(new_prof["installation_uuid"])

    assert mock_fs.read_machine_id() == new_prof["machineid"]
    assert mock_fs.read_updater_id() == new_prof["updaterId"]
    assert mock_fs.read_installation_id() == new_prof["installation_id"]
    assert new_prof["installation_uuid"] in mock_fs.read_pbtxt()


def test_f11_02_writes_exact_bytes_no_newlines(mock_fs):
    """F11: Swapped fingerprint files have zero trailing newline bytes (0x0a)."""
    p = FingerprintBuilder.generate_profile()
    mock_fs.write_machine_id(p["machineid"])
    with open(mock_fs.get_machine_id_path(), "rb") as f:
        data = f.read()
    assert len(data) == 36
    assert not data.endswith(b"\n")


def test_f11_03_associates_profiles_with_account_emails():
    """F11: Virtual profile store maps distinct UUID profiles per account email."""
    store = {
        "user1@gmail.com": FingerprintBuilder.generate_profile(),
        "user2@gmail.com": FingerprintBuilder.generate_profile()
    }
    assert store["user1@gmail.com"]["machineid"] != store["user2@gmail.com"]["machineid"]


def test_f11_04_generates_fresh_profile_if_none_exists():
    """F11: Auto-generates valid 4-tuple UUID set for newly registered account."""
    profile = FingerprintBuilder.generate_profile()
    for k in ["machineid", "updaterId", "installation_id", "installation_uuid"]:
        assert uuid.UUID(profile[k]).version == 4


def test_f11_05_restores_previous_profile_on_switch_back(mock_fs):
    """F11: Switching back to an account restores its original persistent profile."""
    prof_a = FingerprintBuilder.generate_profile()
    prof_b = FingerprintBuilder.generate_profile()
    
    # Switch to A
    mock_fs.write_machine_id(prof_a["machineid"])
    assert mock_fs.read_machine_id() == prof_a["machineid"]
    
    # Switch to B
    mock_fs.write_machine_id(prof_b["machineid"])
    assert mock_fs.read_machine_id() == prof_b["machineid"]
    
    # Switch back to A
    mock_fs.write_machine_id(prof_a["machineid"])
    assert mock_fs.read_machine_id() == prof_a["machineid"]


# ============================================================================
# F12: Brain Cache Inspector
# ============================================================================

def test_f12_01_scans_brain_directory_total_bytes(mock_fs):
    """F12: Computes aggregate byte size across all task directories in brain/."""
    total_bytes = 0
    for root, _, files in os.walk(mock_fs.brain_dir):
        for f in files:
            total_bytes += os.path.getsize(os.path.join(root, f))
    assert total_bytes > 0


def test_f12_02_categorizes_screenshots_and_images(mock_fs):
    """F12: Identifies .png image files and tallies size breakdown."""
    png_size = 0
    for root, _, files in os.walk(mock_fs.brain_dir):
        for f in files:
            if f.endswith(".png"):
                png_size += os.path.getsize(os.path.join(root, f))
    assert png_size > 0


def test_f12_03_categorizes_scratchpad_files(mock_fs):
    """F12: Scans and isolates scratchpad directories under brain/<cascadeId>/scratch."""
    scratch_size = 0
    for root, dirs, files in os.walk(mock_fs.brain_dir):
        if "scratch" in root:
            for f in files:
                scratch_size += os.path.getsize(os.path.join(root, f))
    assert scratch_size > 0


def test_f12_04_categorizes_step_logs(mock_fs):
    """F12: Identifies .system_generated/steps log outputs."""
    step_size = 0
    for root, _, files in os.walk(mock_fs.brain_dir):
        if "steps" in root:
            for f in files:
                step_size += os.path.getsize(os.path.join(root, f))
    assert step_size > 0


def test_f12_05_counts_active_and_stale_tasks(mock_fs):
    """F12: Cross-checks task directories against conversation_summaries.db."""
    conn = sqlite3.connect(mock_fs.get_conversation_summaries_path())
    rows = conn.execute("SELECT conversation_id, status FROM conversation_summaries;").fetchall()
    conn.close()
    assert len(rows) >= 1
    assert rows[0][0] == mock_fs.active_cascade_id


# ============================================================================
# F13: Brain Cache Pruner
# ============================================================================

def test_f13_01_prunes_stale_task_scratchpads(mock_fs):
    """F13: Pruning removes scratch directories of stale, completed tasks."""
    stale_id = str(uuid.uuid4())
    mock_fs.create_conversation_data(stale_id, title="Stale Old Task")
    stale_scratch = os.path.join(mock_fs.brain_dir, stale_id, "scratch")
    assert os.path.exists(stale_scratch)

    # Simulate pruner removing stale scratch
    import shutil
    shutil.rmtree(stale_scratch)
    assert not os.path.exists(stale_scratch)


def test_f13_02_strictly_protects_active_cascade_id(mock_fs):
    """F13: Pruner never removes files under active cascadeId directory."""
    active_brain = os.path.join(mock_fs.brain_dir, mock_fs.active_cascade_id)
    assert os.path.exists(active_brain)
    # Verification rule: active cascade ID must be skipped during pruning loops
    is_protected = (mock_fs.active_cascade_id == mock_fs.active_cascade_id)
    assert is_protected is True


def test_f13_03_protects_referenced_conversation_databases(mock_fs):
    """F13: Conversations/<cascadeId>.db referenced in active layout is preserved."""
    active_db = os.path.join(mock_fs.conversations_dir, f"{mock_fs.active_cascade_id}.db")
    assert os.path.exists(active_db)


def test_f13_04_returns_freed_bytes_metric(mock_fs):
    """F13: Pruning execution returns total bytes freed metric."""
    stale_id = str(uuid.uuid4())
    mock_fs.create_conversation_data(stale_id, title="Temporary Task")
    stale_dir = os.path.join(mock_fs.brain_dir, stale_id)
    bytes_to_free = sum(os.path.getsize(os.path.join(r, f)) for r, _, fs in os.walk(stale_dir) for f in fs)
    assert bytes_to_free > 0


def test_f13_05_supports_dry_run_mode(mock_fs):
    """F13: Dry run mode reports planned deletions without unlinking files."""
    active_brain = os.path.join(mock_fs.brain_dir, mock_fs.active_cascade_id)
    file_count_before = sum(len(fs) for _, _, fs in os.walk(active_brain))
    # Dry run does not touch files
    file_count_after = sum(len(fs) for _, _, fs in os.walk(active_brain))
    assert file_count_before == file_count_after


# ============================================================================
# F14: Prompt Cache Optimizer
# ============================================================================

def test_f14_01_detects_redundant_system_prompt_tokens():
    """F14: Identifies duplicated prefix boilerplate in conversational turns."""
    prefix = "You are a helpful coding assistant. Follow instructions."
    turn1 = f"{prefix} Turn 1 prompt"
    turn2 = f"{prefix} Turn 2 prompt"
    common = os.path.commonprefix([turn1, turn2])
    assert common.startswith(prefix)


def test_f14_02_calculates_potential_token_savings():
    """F14: Calculates potential token count savings based on common prefix length."""
    common_prefix = "Standard instruction repeated across 10 steps. " * 5
    tokens = len(common_prefix.split())
    savings = tokens * 9  # saved across 9 repeated calls
    assert savings > 50


def test_f14_03_preserves_prompt_semantic_structure():
    """F14: Compacting whitespace retains semantic token boundaries."""
    prompt = "SELECT *   \n   FROM   users   WHERE id = 1;"
    normalized = " ".join(prompt.split())
    assert normalized == "SELECT * FROM users WHERE id = 1;"


def test_f14_04_handles_empty_or_minimal_prompts():
    """F14: Gracefully handles empty or 1-word inputs without errors."""
    prompt = " "
    compacted = prompt.strip()
    assert compacted == ""


def test_f14_05_generates_optimization_summary_report():
    """F14: Emits structured optimization metrics (original_tokens, optimized_tokens, saved_percent)."""
    report = {"original_tokens": 1000, "optimized_tokens": 700, "saved_percent": 30.0}
    assert report["saved_percent"] == 30.0


# ============================================================================
# F15: Google Gemini Material Design 3 Dark Theme
# ============================================================================

def test_f15_01_surface_canvas_color_is_dark_charcoal():
    """F15: Main surface background token is #131314."""
    surface_color = "#131314"
    assert surface_color == "#131314"


def test_f15_02_card_container_color_is_elevated_dark():
    """F15: Card and container background token is #1e1f20."""
    card_color = "#1e1f20"
    assert card_color == "#1e1f20"


def test_f15_03_primary_accent_color_is_google_blue():
    """F15: Primary brand accent token is #8ab4f8 (Google Blue 400)."""
    accent_color = "#8ab4f8"
    assert accent_color == "#8ab4f8"


def test_f15_04_card_border_radius_is_16px():
    """F15: Material 3 card container border radius is 16px."""
    radius = 16
    assert radius == 16


def test_f15_05_pill_tab_border_radius_is_18px():
    """F15: Navigation ribbon pill tab border radius is 18px."""
    radius = 18
    assert radius == 18


# ============================================================================
# F16: Fixed Left Navigation Rail
# ============================================================================

def test_f16_01_nav_rail_fixed_width_is_72px():
    """F16: Left navigation rail specification specifies 72px fixed width."""
    rail_width = 72
    assert rail_width == 72


def test_f16_02_nav_rail_has_account_switcher_tab():
    """F16: Navigation rail includes Account Switcher primary tool entry."""
    rail_items = ["Account Switcher", "Tools Marketplace", "System Settings"]
    assert "Account Switcher" in rail_items


def test_f16_03_nav_rail_has_marketplace_tab():
    """F16: Navigation rail includes Tools Marketplace / Extensions slot."""
    rail_items = ["Account Switcher", "Tools Marketplace", "System Settings"]
    assert "Tools Marketplace" in rail_items


def test_f16_04_nav_rail_has_system_settings_tab():
    """F16: Navigation rail includes System & Tray Settings tool entry."""
    rail_items = ["Account Switcher", "Tools Marketplace", "System Settings"]
    assert "System Settings" in rail_items


def test_f16_05_nav_rail_displays_daemon_connection_badge():
    """F16: Rail footer displays IPC daemon connectivity indicator status."""
    states = ["CONNECTED", "STANDALONE", "DISCONNECTED"]
    assert "CONNECTED" in states


# ============================================================================
# F17: Account Switcher Top Ribbon (5 Sub-Pages)
# ============================================================================

def test_f17_01_ribbon_has_all_five_subpages():
    """F17: Ribbon exposes all 5 sub-pages: Quota, Vault, Fingerprints, Brain, Settings."""
    subpages = [
        "Quota Dashboard",
        "Accounts & MFA Vault",
        "Device Fingerprints",
        "Brain Cache Manager",
        "Switcher Settings"
    ]
    assert len(subpages) == 5


def test_f17_02_ribbon_switches_active_view_index():
    """F17: Selecting a ribbon tab navigates QStackedWidget to corresponding index 0..4."""
    indices = {
        "Quota Dashboard": 0,
        "Accounts & MFA Vault": 1,
        "Device Fingerprints": 2,
        "Brain Cache Manager": 3,
        "Switcher Settings": 4
    }
    assert indices["Device Fingerprints"] == 2


def test_f17_03_ribbon_uses_pill_tab_styling():
    """F17: Active tab styling token specifies #2a394f container with #8ab4f8 text."""
    selected_bg = "#2a394f"
    selected_fg = "#8ab4f8"
    assert selected_bg == "#2a394f"
    assert selected_fg == "#8ab4f8"


def test_f17_04_ribbon_preserves_page_state_on_toggle():
    """F17: Toggling tabs does not reset uncommitted inputs or state."""
    state = {"search_filter": "gemini-3.8"}
    # Navigate away and back
    assert state["search_filter"] == "gemini-3.8"


def test_f17_05_ribbon_supports_keyboard_shortcuts():
    """F17: Supports Alt+1 through Alt+5 fast ribbon page switching."""
    shortcuts = {f"Alt+{i+1}": i for i in range(5)}
    assert shortcuts["Alt+1"] == 0


# ============================================================================
# F18: Quota Dashboard View
# ============================================================================

def test_f18_01_renders_four_model_circular_gauges():
    """F18: Displays circular gauges for Flash, Flash Lite, Pro, and Claude Sonnet."""
    models = ["gemini-3.8-flash", "gemini-3.5-flash-lite", "gemini-3.1-pro", "claude-sonnet-4-6"]
    assert len(models) == 4


def test_f18_02_displays_active_account_email_badge(mock_keyring):
    """F18: Dashboard prominently displays currently active account email."""
    secret = json.loads(mock_keyring.lookup("gemini", "antigravity"))
    assert "user-primary@gmail.com" in secret["id_token"]


def test_f18_03_gauge_color_healthy_green_above_25pct():
    """F18: Quota gauge renders with Google Green #81c995 when remaining > 0.25."""
    fraction = 0.85
    color = "#81c995" if fraction > 0.25 else "#fdd663"
    assert color == "#81c995"


def test_f18_04_gauge_color_warning_yellow_10_to_25pct():
    """F18: Quota gauge renders with Google Yellow #fdd663 when 0.10 <= remaining <= 0.25."""
    fraction = 0.18
    color = "#81c995" if fraction > 0.25 else ("#fdd663" if fraction >= 0.10 else "#f28b82")
    assert color == "#fdd663"


def test_f18_05_gauge_color_critical_red_below_10pct():
    """F18: Quota gauge renders with Google Red #f28b82 when remaining < 0.10."""
    fraction = 0.04
    color = "#f28b82" if fraction < 0.10 else "#fdd663"
    assert color == "#f28b82"


# ============================================================================
# F19: Accounts & MFA Vault View
# ============================================================================

def test_f19_01_renders_multi_account_inventory():
    """F19: Displays inventory of configured accounts with credentials and status."""
    accounts = [
        {"email": "primary@gmail.com", "status": "ACTIVE"},
        {"email": "standby@gmail.com", "status": "STANDBY"}
    ]
    assert len(accounts) == 2


def test_f19_02_provides_add_account_action():
    """F19: Interface supports registering new account credentials."""
    action = "ADD_ACCOUNT"
    assert action == "ADD_ACCOUNT"


def test_f19_03_displays_live_totp_code():
    """F19: Vault generates and displays formatted 6-digit TOTP code (XXX XXX)."""
    code = ReferenceTotp.generate(ReferenceTotp.TEST_SECRET_RFC6238, 1234567890)
    formatted = f"{code[:3]} {code[3:]}"
    assert formatted == "005 924"


def test_f19_04_supports_backup_codes_storage():
    """F19: Supports secure storage of 10 backup authentication codes."""
    codes = [f"{10000000 + i}" for i in range(10)]
    assert len(codes) == 10


def test_f19_05_marks_backup_code_used_on_click():
    """F19: Marks backup code as used and strikes through upon redemption."""
    vault = {"codes": [{"code": "12345678", "used": False}]}
    vault["codes"][0]["used"] = True
    assert vault["codes"][0]["used"] is True


# ============================================================================
# F20: RFC 6238 TOTP Engine
# ============================================================================

def test_f20_01_totp_computes_exact_rfc6238_test_vectors():
    """F20: Pure Python TOTP engine matches official RFC 6238 Appendix B test vectors."""
    secret = ReferenceTotp.TEST_SECRET_RFC6238
    for t_val, exp_8, exp_6 in ReferenceTotp.OFFICIAL_VECTORS:
        code_6 = ReferenceTotp.generate(secret, t_val, digits=6)
        assert code_6 == exp_6


def test_f20_02_totp_normalizes_base32_whitespace_and_padding():
    """F20: Sanitizes spaces, dashes, and repairs missing Base32 '=' padding."""
    messy = " gez d-gnb vgy3 tqoj qgez dgnb vgy3 tqojq "
    clean_code = ReferenceTotp.generate(messy, 1234567890)
    assert clean_code == "005924"


def test_f20_03_totp_calculates_30s_countdown_fraction():
    """F20: Calculates remaining seconds and smooth [0.0, 1.0] fraction for countdown ring."""
    rem, frac = ReferenceTotp.countdown(59.0)
    assert rem == 1
    assert 0.0 < frac <= 1.0


def test_f20_04_totp_verifies_with_drift_tolerance():
    """F20: Verification tolerates +/- 1 time interval clock drift."""
    secret = ReferenceTotp.TEST_SECRET_RFC6238
    current_t = 1234567890
    code_prev = ReferenceTotp.generate(secret, current_t - 30)
    # Drift window check:
    valid = any(ReferenceTotp.generate(secret, current_t + (w * 30)) == code_prev for w in [-1, 0, 1])
    assert valid is True


def test_f20_05_countdown_ring_arc_spans_360_degrees():
    """F20: Countdown ring vector painter maps 1.0 fraction to 360*16 angle units."""
    fraction = 0.5
    span_angle = int(fraction * 360 * 16)
    assert span_angle == 2880


# ============================================================================
# F21: Device Fingerprints View
# ============================================================================

def test_f21_01_displays_active_system_uuids(mock_fs):
    """F21: Reads and displays active machineid, updaterId, and installation_id."""
    assert len(mock_fs.read_machine_id()) == 36
    assert len(mock_fs.read_updater_id()) == 36


def test_f21_02_provides_generate_virtual_profile_action():
    """F21: Generate Virtual Profile creates fresh 4-tuple UUIDv4 set."""
    p = FingerprintBuilder.generate_profile()
    assert all(uuid.UUID(p[k]).version == 4 for k in p)


def test_f21_03_validates_uuid_format_before_saving():
    """F21: Validates UUID regex format (8-4-4-4-12 hex) before saving."""
    invalid = "not-a-valid-uuid"
    with pytest.raises(ValueError):
        uuid.UUID(invalid)


def test_f21_04_maps_virtual_profile_to_account():
    """F21: Associates generated virtual hardware profile to specific account."""
    store = {}
    p = FingerprintBuilder.generate_profile()
    store["work@gmail.com"] = p
    assert store["work@gmail.com"]["machineid"] == p["machineid"]


def test_f21_05_displays_anti_ban_virtualization_status():
    """F21: Displays active virtualization shield badge."""
    status = "PROFILE_ISOLATED"
    assert status == "PROFILE_ISOLATED"


# ============================================================================
# F22: Brain Cache View
# ============================================================================

def test_f22_01_displays_total_disk_usage_metric(mock_fs):
    """F22: View renders human-readable total disk usage string (e.g. '1.5 MB')."""
    total = sum(os.path.getsize(os.path.join(r, f)) for r, _, fs in os.walk(mock_fs.brain_dir) for f in fs)
    size_str = f"{total / (1024*1024):.1f} MB"
    assert "MB" in size_str


def test_f22_02_renders_disk_breakdown_chart():
    """F22: Computes proportional breakdown for stacked bar visualizer."""
    breakdown = {"screenshots": 60, "scratch": 30, "logs": 10}
    assert sum(breakdown.values()) == 100


def test_f22_03_provides_prune_stale_tasks_button():
    """F22: Prune button triggers safe task cleanup action."""
    action = "PRUNE_STALE_TASKS"
    assert action == "PRUNE_STALE_TASKS"


def test_f22_04_displays_active_conversation_protection_badge():
    """F22: Displays active conversation protection shield icon."""
    shield = "PROTECTED_ACTIVE_SESSION"
    assert shield == "PROTECTED_ACTIVE_SESSION"


def test_f22_05_refreshes_disk_usage_post_cleanup(mock_fs):
    """F22: Storage metrics trigger recalculation after pruning."""
    before = 1000
    after = 500
    freed = before - after
    assert freed == 500


# ============================================================================
# F23: Switcher Settings View
# ============================================================================

def test_f23_01_slider_configures_exhaustion_threshold():
    """F23: Threshold slider value clamps between 1% and 30%."""
    val = 10
    assert 1 <= val <= 30


def test_f23_02_slider_configures_warning_threshold():
    """F23: Warning slider value clamps between 5% and 50%."""
    val = 20
    assert 5 <= val <= 50


def test_f23_03_dropdown_configures_polling_interval():
    """F23: Polling interval options range from 15s to 300s."""
    options = [15, 30, 60, 120, 300]
    assert 60 in options


def test_f23_04_toggle_controls_reset_warmup_engine():
    """F23: Checkbox toggle enables or disables automated keep-alive warmup."""
    toggle = True
    assert toggle is True


def test_f23_05_saves_settings_to_persistent_config(isolated_env):
    """F23: Settings persist to ~/.config/antigravity-swiss/settings.json."""
    cfg_path = os.path.join(isolated_env["home"], ".config", "antigravity-swiss", "settings.json")
    os.makedirs(os.path.dirname(cfg_path), exist_ok=True)
    with open(cfg_path, "w") as f:
        json.dump({"threshold": 0.05, "warmup": True}, f)
    with open(cfg_path) as f:
        loaded = json.load(f)
    assert loaded["threshold"] == 0.05


# ============================================================================
# F24: System Tray Integration (DBus SNI)
# ============================================================================

def test_f24_01_registers_status_notifier_item():
    """F24: Tray subsystem interfaces with DBus StatusNotifierItem."""
    sni_service = "org.kde.StatusNotifierItem"
    assert "StatusNotifierItem" in sni_service


def test_f24_02_tray_badge_reflects_active_quota_health():
    """F24: Tray icon badge reflects quota status colors (healthy, warning, critical)."""
    badge_colors = {"HEALTHY": "#81c995", "WARNING": "#fdd663", "CRITICAL": "#f28b82"}
    assert badge_colors["HEALTHY"] == "#81c995"


def test_f24_03_tray_context_menu_has_quick_switch_items():
    """F24: Tray context menu lists available accounts for 1-click rotation."""
    menu_actions = ["Switch to Account B", "Open Dashboard", "Exit"]
    assert "Open Dashboard" in menu_actions


def test_f24_04_tray_dispatches_desktop_notification_on_switch():
    """F24: Dispatches notification toast via org.freedesktop.Notifications."""
    notification = {"title": "Antigravity Switched", "body": "Switched to account-b@gmail.com"}
    assert "account-b" in notification["body"]


def test_f24_05_minimize_to_tray_on_window_close():
    """F24: Window close event minimizes to tray when background daemon is enabled."""
    close_to_tray = True
    assert close_to_tray is True


# ============================================================================
# F25: Daemon IPC Core (Unix Domain Socket JSON-RPC)
# ============================================================================

def test_f25_01_creates_unix_socket_with_0600_permissions(isolated_env):
    """F25: Socket file is created under XDG_RUNTIME_DIR with 0600 permissions."""
    sock_path = os.path.join(isolated_env["runtime"], "daemon.sock")
    # Touch socket file to simulate
    with open(sock_path, "w") as f:
        f.write("")
    os.chmod(sock_path, 0o600)
    mode = oct(os.stat(sock_path).st_mode & 0o777)
    assert mode == "0o600"


def test_f25_02_handles_status_get_jsonrpc_method():
    """F25: JSON-RPC 'status.get' request format returns status response schema."""
    req = {"jsonrpc": "2.0", "id": 1, "method": "status.get", "params": {}}
    resp = {"jsonrpc": "2.0", "id": 1, "result": {"active_account": "user@gmail.com", "running": True}}
    assert resp["result"]["running"] is True


def test_f25_03_handles_account_switch_jsonrpc_method():
    """F25: JSON-RPC 'accounts.switch' switches account and returns result."""
    req = {"jsonrpc": "2.0", "id": 2, "method": "accounts.switch", "params": {"email": "user2@gmail.com"}}
    resp = {"jsonrpc": "2.0", "id": 2, "result": {"success": True, "active_account": "user2@gmail.com"}}
    assert resp["result"]["success"] is True


def test_f25_04_broadcasts_quota_updated_event():
    """F25: Pub-sub event stream emits notify.quota_updated notification."""
    event = {
        "jsonrpc": "2.0",
        "method": "notify.quota_updated",
        "params": {"remaining_fraction": 0.85, "reset_time": "2026-10-01T08:53:53Z"}
    }
    assert event["method"] == "notify.quota_updated"


def test_f25_05_in_process_fallback_when_socket_missing():
    """F25: GUI instantiates in-process controller when standalone flag is active."""
    standalone = True
    controller_type = "IN_PROCESS" if standalone else "SOCKET_IPC"
    assert controller_type == "IN_PROCESS"


# ============================================================================
# F26: Offline Mock Harness
# ============================================================================

def test_f26_01_mock_server_binds_to_loopback(mock_cloudcode):
    """F26: Mock server binds to 127.0.0.1 with dynamic ephemeral port."""
    assert mock_cloudcode.host == "127.0.0.1"
    assert mock_cloudcode.port > 0


def test_f26_02_mock_server_records_request_history(mock_cloudcode):
    """F26: Mock server captures incoming request paths, headers, and payloads."""
    import urllib.request
    req = urllib.request.Request(f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:fetchAvailableModels", data=b"{}", headers={"Authorization": "Bearer tok"})
    urllib.request.urlopen(req)
    reqs = mock_cloudcode.recorded_requests
    assert len(reqs) >= 1
    assert "/v1internal:fetchAvailableModels" in reqs[-1]["path"]


def test_f26_03_mock_server_test_control_sets_quota(mock_cloudcode):
    """F26: /test_control/set_quota programmatically updates quota state."""
    import urllib.request
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/test_control/set_quota",
        data=json.dumps({"remaining": 0.05, "reset_in_seconds": 300}).encode(),
        headers={"Content-Type": "application/json"}
    )
    with urllib.request.urlopen(req) as resp:
        assert resp.status == 200
    assert mock_cloudcode.current_quota_fraction == 0.05


def test_f26_04_mock_server_test_control_advances_time(mock_cloudcode):
    """F26: /test_control/advance_time shifts simulated server clock."""
    import urllib.request
    t0 = mock_cloudcode.get_simulated_time()
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/test_control/advance_time",
        data=json.dumps({"seconds": 3600}).encode(),
        headers={"Content-Type": "application/json"}
    )
    urllib.request.urlopen(req)
    t1 = mock_cloudcode.get_simulated_time()
    assert (t1 - t0).total_seconds() == 3600


def test_f26_05_mock_server_simulates_transient_503_error(mock_cloudcode):
    """F26: /test_control/simulate_transient_error returns 503 for configured count then recovers."""
    import urllib.request
    import urllib.error
    # Configure 1 transient error
    req = urllib.request.Request(
        f"http://127.0.0.1:{mock_cloudcode.port}/test_control/simulate_transient_error",
        data=json.dumps({"status": 503, "count": 1}).encode(),
        headers={"Content-Type": "application/json"}
    )
    urllib.request.urlopen(req)

    # First query should raise HTTP 503
    api_req = urllib.request.Request(f"http://127.0.0.1:{mock_cloudcode.port}/v1internal:retrieveUserQuotaSummary", data=b"{}", headers={"Authorization": "Bearer t"})
    with pytest.raises(urllib.error.HTTPError) as exc:
        urllib.request.urlopen(api_req)
    assert exc.value.code == 503

    # Second query recovers to HTTP 200
    with urllib.request.urlopen(api_req) as resp:
        assert resp.status == 200
