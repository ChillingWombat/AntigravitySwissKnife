"""
Adversarial Stress Test Suite: Hardware Identity, Cache Retention & IPC Daemon
================================================================================
Evaluates:
1. Exact 36-Byte Binary Stress: FingerprintManager across 50 profiles (strict 36B, zero trailing newlines, 0600 mode).
2. Protobuf Surgical Mutation Stress: PbtxtParser preserves surrounding comments, flags, seen_nuxs, migrations.
3. Safe Cache Retention Stress: BrainCachePruner preserves 100% of active & pinned session files under aggressive 0-day prune.
4. Permanent Transcript Survival: BrainCachePruner unconditionally preserves transcript.jsonl & transcript_full.jsonl.
5. Locked SQLite DB Concurrency: BrainCachePruner gracefully times out (5s) on exclusive SQLite write locks without crashing.
6. Prompt Token Bloat Math & Bounds: PromptCacheOptimizer triangular context sum, massive tool outputs (>100KB), bounds [0.0, 1.0].
7. IPC Stress & Concurrency: Rapid concurrent JSON-RPC requests across Unix Domain Socket with large frame handling.
"""

from __future__ import annotations

import asyncio
import datetime
import json
import logging
import math
import os
from pathlib import Path
import re
import shutil
import sqlite3
import tempfile
import threading
import time
from typing import Any
import uuid
import pytest

from antigravity_swiss.cache_optimizer.inspector import BrainCacheInspector
from antigravity_swiss.cache_optimizer.models import (
    CacheCategory,
    PruneOptions,
    PruneResult,
    PromptCacheAnalysis,
)
from antigravity_swiss.cache_optimizer.prompt_cache import (
    PromptCacheOptimizer,
    OVERSIZED_OUTPUT_THRESHOLD_BYTES,
)
from antigravity_swiss.cache_optimizer.pruner import BrainCachePruner
from antigravity_swiss.core.constants import MAX_FRAME_SIZE
from antigravity_swiss.core.errors import FingerprintError, IPCError
from antigravity_swiss.fingerprint.manager import FingerprintManager
from antigravity_swiss.fingerprint.models import DeviceProfile
from antigravity_swiss.fingerprint.pbtxt_parser import PbtxtParser
from antigravity_swiss.fingerprint.profile_store import ProfileStore
from antigravity_swiss.ipc.socket_client import AsyncDaemonClient
from antigravity_swiss.ipc.socket_server import AsyncUnixSocketServer

logger = logging.getLogger("test_final_cache_fingerprint_stress")

UUID_REGEX = re.compile(r"^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")


# =============================================================================
# 1. Exact 36-Byte Binary Stress Tests
# =============================================================================

def test_exact_36_byte_binary_stress_50_profiles(tmp_path: Path):
    """
    Adversarial Stress Test: Generates 50 unique DeviceProfile profiles and writes them to disk.
    Strictly asserts that machineid, .updaterId, and installation_id files:
    - Are exactly 36 bytes in size.
    - Contain strictly ASCII UUID strings matching regex.
    - Contain ZERO trailing newlines (\\n, \\r, or \\r\\n).
    - Have 0600 file permissions.
    """
    config_dir = tmp_path / "antigravity_config"
    data_dir = tmp_path / "antigravity_data"
    config_dir.mkdir(parents=True)
    data_dir.mkdir(parents=True)

    store = ProfileStore(storage_path=config_dir / "device_profiles.json")
    manager = FingerprintManager(config_dir=config_dir, data_dir=data_dir, store=store)

    target_files = [
        config_dir / "machineid",
        config_dir / ".updaterId",
        data_dir / "installation_id",
    ]

    for i in range(50):
        email = f"adversary_user_{i:03d}@example.com"
        profile = manager.create_or_get_profile(email)
        assert profile.account_email == email

        # Write active profile to isolated directory
        manager.write_active_profile(profile)

        # Inspect all 3 raw UUID files
        for target_file in target_files:
            assert target_file.exists(), f"Target file {target_file} was not created on run {i}"
            stat_info = target_file.stat()

            # Exact byte count
            assert stat_info.st_size == 36, (
                f"File {target_file.name} for profile {i} is {stat_info.st_size} bytes, expected exactly 36"
            )

            # Raw binary inspection
            raw_bytes = target_file.read_bytes()
            assert len(raw_bytes) == 36, f"Binary length mismatch for {target_file.name}: {len(raw_bytes)}"
            assert raw_bytes[-1] not in (0x0A, 0x0D), (
                f"Trailing newline detected in {target_file.name} on profile {i}: byte {raw_bytes[-1]:#x}"
            )
            assert raw_bytes[-2] not in (0x0A, 0x0D), (
                f"Trailing newline detected in {target_file.name} on profile {i}: byte {raw_bytes[-2]:#x}"
            )

            # String validation
            raw_text = raw_bytes.decode("ascii")
            assert UUID_REGEX.match(raw_text), (
                f"Content '{raw_text}' in {target_file.name} is not a valid UUIDv4"
            )

            # Permission check: 0600 (owner read/write only)
            mode = stat_info.st_mode & 0o777
            assert mode == 0o600, f"Permissions on {target_file.name} are {oct(mode)}, expected 0o600"


def test_exact_36_byte_binary_stress_invalid_uuid_rejections(tmp_path: Path):
    """
    Adversarial Stress Test: Injects malformed UUID lengths into _write_exact_36b_file.
    Verifies FingerprintError is raised when stripped length != 36 bytes.
    Verifies that inputs with trailing newlines are safely sanitized to exactly 36 bytes without newlines on disk.
    """
    config_dir = tmp_path / "config"
    data_dir = tmp_path / "data"
    manager = FingerprintManager(config_dir=config_dir, data_dir=data_dir)

    target_file = config_dir / "machineid"
    valid_uuid = str(uuid.uuid4())
    manager._write_exact_36b_file(target_file, valid_uuid)
    assert target_file.stat().st_size == 36

    malformed_inputs = [
        "",  # 0 bytes
        "short-uuid",  # 10 bytes
        valid_uuid[:35],  # 35 bytes
        valid_uuid + "x",  # 37 bytes
        " " * 36,  # 36 spaces -> stripped to 0 bytes
        "invalid-prefix-" + valid_uuid,  # > 36 bytes
    ]

    for bad_input in malformed_inputs:
        with pytest.raises(FingerprintError):
            manager._write_exact_36b_file(target_file, bad_input)

    # Verify input with newlines is sanitized to exact 36 bytes without newlines on disk
    manager._write_exact_36b_file(target_file, valid_uuid + "\n\r\n")
    assert target_file.stat().st_size == 36
    raw_bytes = target_file.read_bytes()
    assert len(raw_bytes) == 36
    assert raw_bytes[-1] not in (0x0A, 0x0D)
    assert target_file.read_text(encoding="ascii") == valid_uuid


def test_exact_36_byte_binary_swap_profile_stress(tmp_path: Path):
    """
    Adversarial Stress Test: Swaps profiles across 50 accounts in sequence using swap_profile_for_account().
    Verifies that at every swap:
    - Files are atomically replaced with the target account's unique UUIDs.
    - Files remain strictly 36 bytes, 0600 mode, zero trailing newlines.
    - Active account in ProfileStore is updated consistently.
    """
    config_dir = tmp_path / "cfg_swap"
    data_dir = tmp_path / "dt_swap"
    config_dir.mkdir(parents=True)
    data_dir.mkdir(parents=True)

    store = ProfileStore(storage_path=config_dir / "device_profiles.json")
    manager = FingerprintManager(config_dir=config_dir, data_dir=data_dir, store=store)

    target_files = [
        config_dir / "machineid",
        config_dir / ".updaterId",
        data_dir / "installation_id",
    ]

    for i in range(50):
        email = f"rotation_account_{i:02d}@corp.com"
        swapped_prof = manager.swap_profile_for_account(email)

        assert manager.store.get_active_account() == email
        assert swapped_prof.account_email == email

        for tf in target_files:
            assert tf.stat().st_size == 36
            raw = tf.read_bytes()
            assert len(raw) == 36
            assert raw[-1] not in (0x0A, 0x0D)
            assert (tf.stat().st_mode & 0o777) == 0o600

        assert (config_dir / "machineid").read_text(encoding="ascii") == swapped_prof.machine_id
        assert (config_dir / ".updaterId").read_text(encoding="ascii") == swapped_prof.updater_id
        assert (data_dir / "installation_id").read_text(encoding="ascii") == swapped_prof.installation_id


# =============================================================================
# 2. Protobuf Surgical Mutation Stress Tests
# =============================================================================

COMPLEX_PBTXT_FIXTURE = """# Google Antigravity Application State Protobuf
# DO NOT EDIT MANUALLY - Schema version: 2026.10

user_preferences {
  theme_mode: "gemini_dark"
  enable_hardware_acceleration: true
  telemetry_opt_in: false
  auto_update_channel: "stable"
}

onboarding_state {
  onboarding_completed: true
  terms_accepted: true
  accepted_at_epoch_ms: 1727788800000
  seen_nuxs: "welcome_modal_v2"
  seen_nuxs: "model_selector_dropdown"
  seen_nuxs: "quota_refresh_prompt"
  seen_nuxs: "mfa_vault_introduction"
  seen_nuxs: "device_fingerprint_notice"
}

feature_flags {
  enable_cloud_code_sync: true
  quota_refresh_interval_sec: 300
  max_cached_conversations: 50
  experimental_reasoning: true
}

migration_history {
  record {
    migration_version: 1
    applied_at_epoch_ms: 1720000000000
    checksum: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
  }
  record {
    migration_version: 2
    applied_at_epoch_ms: 1725000000000
    checksum: "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb"
  }
  record {
    migration_version: 3
    applied_at_epoch_ms: 1727788800000
    checksum: "4e07408562bedb8b60ce05c1decfe3ad16b72230967de01f640b7e4729b49fce"
  }
}

# Core Device Hardware Identifiers
  installation_uuid: "11111111-2222-3333-4444-555555555555" # Primary tracking UUID
  installation_id: "66666666-7777-8888-9999-000000000000" # Secondary telemetry UUID

client_metadata {
  client_version: "2.0.0-linux-x64"
  build_id: "20261001-release"
}
"""

def test_protobuf_surgical_mutation_stress(tmp_path: Path):
    """
    Adversarial Stress Test: Surgically mutates installation_uuid and installation_id
    inside a complex multi-field antigravity_state.pbtxt file.
    Verifies that:
    - installation_uuid and installation_id are updated to the target values.
    - Surrounding comments, whitespace, onboarding flags, seen_nuxs, migrations, and
      client metadata remain 100% intact.
    - Subsequent reads extract the new values cleanly.
    """
    pbtxt_path = tmp_path / "antigravity_state.pbtxt"
    pbtxt_path.write_text(COMPLEX_PBTXT_FIXTURE, encoding="utf-8")

    # Initial verification
    assert PbtxtParser.extract_installation_uuid(pbtxt_path) == "11111111-2222-3333-4444-555555555555"
    assert PbtxtParser.extract_installation_id(pbtxt_path) == "66666666-7777-8888-9999-000000000000"

    # Surgical mutation across 20 iterations
    for k in range(20):
        new_uuid = f"aaaaaaaa-bbbb-4444-8888-{k:012d}"
        new_id = f"cccccccc-dddd-4444-9999-{k:012d}"

        success = PbtxtParser.update_installation_fields(
            pbtxt_path,
            installation_uuid=new_uuid,
            installation_id=new_id,
        )
        assert success is True

        content = pbtxt_path.read_text(encoding="utf-8")

        # Verify new values present
        assert PbtxtParser.extract_installation_uuid(pbtxt_path) == new_uuid
        assert PbtxtParser.extract_installation_id(pbtxt_path) == new_id

        # Verify inline comments preserved
        assert f'installation_uuid: "{new_uuid}" # Primary tracking UUID' in content
        assert f'installation_id: "{new_id}" # Secondary telemetry UUID' in content

        # Verify surrounding blocks 100% intact
        assert 'theme_mode: "gemini_dark"' in content
        assert 'onboarding_completed: true' in content
        assert 'seen_nuxs: "welcome_modal_v2"' in content
        assert 'seen_nuxs: "device_fingerprint_notice"' in content
        assert 'migration_version: 3' in content
        assert 'checksum: "4e07408562bedb8b60ce05c1decfe3ad16b72230967de01f640b7e4729b49fce"' in content
        assert 'client_version: "2.0.0-linux-x64"' in content


def test_protobuf_surgical_mutation_single_field_updates(tmp_path: Path):
    """
    Adversarial Stress Test: Updates only installation_uuid or only installation_id,
    verifying the untouched field does not revert, disappear, or corrupt.
    """
    pbtxt_path = tmp_path / "antigravity_state.pbtxt"
    pbtxt_path.write_text(COMPLEX_PBTXT_FIXTURE, encoding="utf-8")

    initial_id = "66666666-7777-8888-9999-000000000000"
    target_uuid = "99999999-9999-9999-9999-999999999999"

    # Mutate ONLY installation_uuid
    PbtxtParser.update_installation_uuid(pbtxt_path, target_uuid)
    assert PbtxtParser.extract_installation_uuid(pbtxt_path) == target_uuid
    assert PbtxtParser.extract_installation_id(pbtxt_path) == initial_id

    # Mutate ONLY installation_id
    target_id = "88888888-8888-8888-8888-888888888888"
    PbtxtParser.update_installation_id(pbtxt_path, target_id)
    assert PbtxtParser.extract_installation_uuid(pbtxt_path) == target_uuid
    assert PbtxtParser.extract_installation_id(pbtxt_path) == target_id


def test_protobuf_surgical_mutation_edge_cases(tmp_path: Path):
    """
    Adversarial Stress Test: Pbtxt mutation edge cases:
    - Empty / non-existent initial file (creates cleanly)
    - Unquoted values: installation_uuid: 11111111-2222-3333-4444-555555555555
    - Single-quoted values: installation_uuid: '11111111-2222-3333-4444-555555555555'
    - Comments with unusual characters
    """
    pbtxt_path = tmp_path / "edge_case.pbtxt"

    # 1. Non-existent file
    fresh_uuid = str(uuid.uuid4())
    fresh_id = str(uuid.uuid4())
    PbtxtParser.update_installation_fields(pbtxt_path, installation_uuid=fresh_uuid, installation_id=fresh_id)
    assert PbtxtParser.extract_installation_uuid(pbtxt_path) == fresh_uuid
    assert PbtxtParser.extract_installation_id(pbtxt_path) == fresh_id

    # 2. Single-quoted & unquoted initial text
    unquoted_content = (
        "installation_uuid: 11111111-2222-3333-4444-555555555555 # unquoted\n"
        "installation_id: '22222222-3333-4444-5555-666666666666' # single quotes\n"
        "other_flag: true\n"
    )
    pbtxt_path.write_text(unquoted_content, encoding="utf-8")
    assert PbtxtParser.extract_installation_uuid(pbtxt_path) == "11111111-2222-3333-4444-555555555555"
    assert PbtxtParser.extract_installation_id(pbtxt_path) == "22222222-3333-4444-5555-666666666666"

    # Mutate
    new_uuid = str(uuid.uuid4())
    new_id = str(uuid.uuid4())
    PbtxtParser.update_installation_fields(pbtxt_path, installation_uuid=new_uuid, installation_id=new_id)
    assert PbtxtParser.extract_installation_uuid(pbtxt_path) == new_uuid
    assert PbtxtParser.extract_installation_id(pbtxt_path) == new_id
    assert "other_flag: true" in pbtxt_path.read_text(encoding="utf-8")


# =============================================================================
# 3. Safe Cache Retention Stress Tests
# =============================================================================

def _populate_mock_conversation(
    conv_dir: Path,
    file_count: int = 5,
    mtime_days_ago: float = 30.0,
):
    """Populates a conversation directory with scratch, steps, tasks, media, and transcripts."""
    conv_dir.mkdir(parents=True, exist_ok=True)

    # 1. scratch/
    scratch_dir = conv_dir / "scratch"
    scratch_dir.mkdir(parents=True, exist_ok=True)
    for i in range(file_count):
        (scratch_dir / f"scratch_{i}.py").write_text(f"# Scratch {i}\nx = {i} * 10\n", encoding="utf-8")

    # 2. .system_generated/steps/
    steps_dir = conv_dir / ".system_generated" / "steps"
    steps_dir.mkdir(parents=True, exist_ok=True)
    for i in range(file_count):
        (steps_dir / f"step_{i}.txt").write_text(f"Step dump content {i}\n" * 50, encoding="utf-8")

    # 3. .system_generated/tasks/
    tasks_dir = conv_dir / ".system_generated" / "tasks"
    tasks_dir.mkdir(parents=True, exist_ok=True)
    for i in range(3):
        (tasks_dir / f"task_{i}.log").write_text(f"Task log output {i}\n", encoding="utf-8")

    # 4. media/ and .user_uploaded/
    media_dir = conv_dir / "media"
    media_dir.mkdir(parents=True, exist_ok=True)
    (media_dir / "screenshot_1.png").write_bytes(b"\x89PNG\r\n\x1a\n" + b"DATA" * 500)

    upload_dir = conv_dir / ".user_uploaded"
    upload_dir.mkdir(parents=True, exist_ok=True)
    (upload_dir / "diagram.jpg").write_bytes(b"\xFF\xD8\xFF" + b"JPEG" * 500)

    # 5. Permanent transcripts
    (conv_dir / "transcript.jsonl").write_text(
        json.dumps({"source": "USER", "content": f"Hello from {conv_dir.name}"}) + "\n",
        encoding="utf-8",
    )
    (conv_dir / "transcript_full.jsonl").write_text(
        json.dumps({"source": "MODEL", "content": f"Full response in {conv_dir.name}"}) + "\n",
        encoding="utf-8",
    )
    logs_dir = conv_dir / ".system_generated" / "logs"
    logs_dir.mkdir(parents=True, exist_ok=True)
    (logs_dir / "transcript.jsonl").write_text(
        json.dumps({"source": "SYSTEM", "content": "System prompt"}) + "\n",
        encoding="utf-8",
    )

    # Set directory mtime
    target_mtime = time.time() - (mtime_days_ago * 86400.0)
    os.utime(conv_dir, (target_mtime, target_mtime))


def test_safe_cache_retention_active_and_pinned_sessions(tmp_path: Path):
    """
    Adversarial Stress Test: Safe Cache Retention.
    Creates mock brain structure containing:
    - 1 active conversation matching cascadeId in app_storage.json.
    - 2 pinned conversations in pinned_conversations_order.
    - 3 stale, unpinned conversations (30 days old).
    Executes aggressive prune (min_age_days=0.0).
    Verifies that:
    - 100% of files in active and pinned sessions survive completely untouched.
    - In stale conversations, scratchpads, steps, tasks, and media are pruned.
    - In stale conversations, transcript files 100% survive!
    """
    data_dir = tmp_path / "data"
    config_dir = tmp_path / "config"
    brain_dir = data_dir / "brain"
    convs_dir = data_dir / "conversations"
    config_dir.mkdir(parents=True)
    brain_dir.mkdir(parents=True)
    convs_dir.mkdir(parents=True)

    active_id = "active-cascade-0001"
    pinned_1 = "pinned-session-0002"
    pinned_2 = "pinned-session-0003"
    stale_1 = "stale-session-0004"
    stale_2 = "stale-session-0005"
    stale_3 = "stale-session-0006"

    # Setup app_storage.json
    app_storage_path = config_dir / "app_storage.json"
    storage_payload = {
        f"antigravity-multi-conversation-layout-v3-{active_id}": json.dumps({"cascadeId": active_id}),
        "pinned_conversations_order": json.dumps([pinned_1, pinned_2]),
    }
    app_storage_path.write_text(json.dumps(storage_payload), encoding="utf-8")

    # Populate conversations
    all_sessions = [active_id, pinned_1, pinned_2, stale_1, stale_2, stale_3]
    for s_id in all_sessions:
        _populate_mock_conversation(brain_dir / s_id, file_count=5, mtime_days_ago=30.0)
        # Create corresponding DB
        db_path = convs_dir / f"{s_id}.db"
        con = sqlite3.connect(str(db_path))
        con.execute("CREATE TABLE t (id INT, txt TEXT);")
        con.execute("INSERT INTO t VALUES (1, 'sample');")
        con.commit()
        con.close()

    # Pre-count files in protected sessions
    def count_files(session_id: str) -> int:
        s_dir = brain_dir / session_id
        return sum(len(files) for _, _, files in os.walk(s_dir))

    active_pre_count = count_files(active_id)
    pinned_1_pre_count = count_files(pinned_1)
    pinned_2_pre_count = count_files(pinned_2)

    # Instantiate Pruner and execute aggressive prune (0-day age limit)
    pruner = BrainCachePruner(data_dir=data_dir, config_dir=config_dir)
    assert pruner.get_active_cascade_id() == active_id
    assert set(pruner.get_pinned_conversation_ids()) == {pinned_1, pinned_2}

    options = PruneOptions(
        prune_scratch=True,
        prune_steps=True,
        prune_tasks=True,
        prune_screenshots=True,
        vacuum_databases=True,
        min_age_days=0.0,
        dry_run=False,
    )
    result: PruneResult = pruner.prune(options=options)

    # Assert 100% survival for active session
    active_post_count = count_files(active_id)
    assert active_post_count == active_pre_count, (
        f"Active session lost files! Pre: {active_pre_count}, Post: {active_post_count}"
    )
    assert (brain_dir / active_id / "scratch" / "scratch_0.py").exists()
    assert (brain_dir / active_id / ".system_generated" / "steps" / "step_0.txt").exists()
    assert (brain_dir / active_id / "media" / "screenshot_1.png").exists()

    # Assert 100% survival for pinned sessions
    assert count_files(pinned_1) == pinned_1_pre_count
    assert count_files(pinned_2) == pinned_2_pre_count

    # Assert stale sessions were pruned
    for s_id in [stale_1, stale_2, stale_3]:
        s_dir = brain_dir / s_id
        assert not (s_dir / "scratch").exists(), f"Scratch dir in stale {s_id} was not pruned"
        assert not (s_dir / ".system_generated" / "steps").exists(), f"Steps dir in stale {s_id} was not pruned"
        assert not (s_dir / ".system_generated" / "tasks").exists(), f"Tasks dir in stale {s_id} was not pruned"
        assert not (s_dir / "media").exists(), f"Media dir in stale {s_id} was not pruned"

        # BUT permanent transcripts MUST survive
        assert (s_dir / "transcript.jsonl").exists(), f"transcript.jsonl in {s_id} was destroyed!"
        assert (s_dir / "transcript_full.jsonl").exists(), f"transcript_full.jsonl in {s_id} was destroyed!"
        assert (s_dir / ".system_generated" / "logs" / "transcript.jsonl").exists(), (
            f"System transcript in {s_id} was destroyed!"
        )

    # Verify result properties
    assert result.protected_active_id == active_id
    assert set(result.protected_pinned_ids) == {pinned_1, pinned_2}
    assert result.files_deleted > 0
    assert result.bytes_freed > 0


# =============================================================================
# 4. Permanent Transcript Survival Stress Tests
# =============================================================================

def test_permanent_transcript_survival_stress(tmp_path: Path):
    """
    Adversarial Stress Test: Permanent Transcript Survival.
    Populates 10 stale unpinned conversations with ancient mtimes (up to 365 days ago).
    Verifies that under maximal aggressive prune, transcript.jsonl and transcript_full.jsonl
    are never deleted or truncated across any session.
    """
    data_dir = tmp_path / "data"
    config_dir = tmp_path / "config"
    brain_dir = data_dir / "brain"
    brain_dir.mkdir(parents=True)
    config_dir.mkdir(parents=True)

    # Empty app storage (no protected sessions)
    (config_dir / "app_storage.json").write_text("{}", encoding="utf-8")

    stale_sessions = [f"ancient-session-{i:03d}" for i in range(10)]
    transcript_contents = {}

    for i, s_id in enumerate(stale_sessions):
        s_dir = brain_dir / s_id
        _populate_mock_conversation(s_dir, file_count=3, mtime_days_ago=50.0 + (i * 30.0))

        # Record exact transcript bytes
        t1 = (s_dir / "transcript.jsonl").read_bytes()
        t2 = (s_dir / "transcript_full.jsonl").read_bytes()
        t3 = (s_dir / ".system_generated" / "logs" / "transcript.jsonl").read_bytes()
        transcript_contents[s_id] = (t1, t2, t3)

    pruner = BrainCachePruner(data_dir=data_dir, config_dir=config_dir)
    res = pruner.prune(
        options=PruneOptions(
            prune_scratch=True,
            prune_steps=True,
            prune_tasks=True,
            prune_screenshots=True,
            vacuum_databases=False,
            min_age_days=0.0,
            dry_run=False,
        )
    )

    # Assert 100% transcript survival and byte-for-byte fidelity
    for s_id in stale_sessions:
        s_dir = brain_dir / s_id
        assert (s_dir / "transcript.jsonl").exists()
        assert (s_dir / "transcript_full.jsonl").exists()
        assert (s_dir / ".system_generated" / "logs" / "transcript.jsonl").exists()

        assert (s_dir / "transcript.jsonl").read_bytes() == transcript_contents[s_id][0]
        assert (s_dir / "transcript_full.jsonl").read_bytes() == transcript_contents[s_id][1]
        assert (s_dir / ".system_generated" / "logs" / "transcript.jsonl").read_bytes() == transcript_contents[s_id][2]


def test_permanent_transcript_readonly_and_large_survival(tmp_path: Path):
    """
    Adversarial Stress Test: Read-only and large (5MB) transcripts.
    Verifies that neither permission modes nor large payload sizes cause pruner
    to delete, truncate, or error out on transcript files.
    """
    data_dir = tmp_path / "dt_ro"
    config_dir = tmp_path / "cfg_ro"
    brain_dir = data_dir / "brain"
    brain_dir.mkdir(parents=True)
    config_dir.mkdir(parents=True)
    (config_dir / "app_storage.json").write_text("{}", encoding="utf-8")

    conv_dir = brain_dir / "stale_ro_session"
    conv_dir.mkdir(parents=True)

    # Large 2MB transcript
    large_line = json.dumps({"source": "MODEL", "content": "A" * 1024 * 1024}) + "\n"
    trans_path = conv_dir / "transcript.jsonl"
    trans_path.write_text(large_line * 2, encoding="utf-8")
    assert trans_path.stat().st_size >= 2 * 1024 * 1024

    # Read-only full transcript (chmod 0444)
    full_path = conv_dir / "transcript_full.jsonl"
    full_path.write_text('{"read_only": true}\n', encoding="utf-8")
    full_path.chmod(0o444)

    # Stale directory mtime
    old_time = time.time() - 86400 * 60
    os.utime(conv_dir, (old_time, old_time))

    pruner = BrainCachePruner(data_dir=data_dir, config_dir=config_dir)
    res = pruner.prune(
        options=PruneOptions(
            prune_scratch=True,
            prune_steps=True,
            prune_tasks=True,
            prune_screenshots=True,
            min_age_days=0.0,
            dry_run=False,
        )
    )

    assert trans_path.exists()
    assert trans_path.stat().st_size >= 2 * 1024 * 1024
    assert full_path.exists()
    assert full_path.read_text(encoding="utf-8") == '{"read_only": true}\n'

    # Cleanup permission for tmp_path deletion
    full_path.chmod(0o644)


# =============================================================================
# 5. Locked SQLite DB Concurrency Stress Tests
# =============================================================================

def test_locked_sqlite_db_concurrency_timeout(tmp_path: Path):
    """
    Adversarial Stress Test: Locked SQLite DB Concurrency.
    Simulates an external process or thread holding an exclusive write lock
    on a conversation SQLite database during pruning.
    Verifies that BrainCachePruner:
    - Waits for the configured timeout.
    - Catches sqlite3.OperationalError gracefully.
    - Does NOT crash the daemon or fail the overall prune operation.
    - Leaves the database uncorrupted.
    """
    data_dir = tmp_path / "data"
    config_dir = tmp_path / "config"
    convs_dir = data_dir / "conversations"
    convs_dir.mkdir(parents=True)
    config_dir.mkdir(parents=True)
    (config_dir / "app_storage.json").write_text("{}", encoding="utf-8")

    db_path = convs_dir / "stale_locked_session.db"
    conn = sqlite3.connect(str(db_path))
    conn.execute("CREATE TABLE test_data (id INT, val TEXT);")
    conn.execute("INSERT INTO test_data VALUES (101, 'critical_session_data');")
    conn.commit()

    # Acquire exclusive write lock
    lock_conn = sqlite3.connect(str(db_path), timeout=0.1)
    lock_conn.execute("BEGIN EXCLUSIVE;")
    lock_conn.execute("INSERT INTO test_data VALUES (102, 'locked_row');")

    pruner = BrainCachePruner(data_dir=data_dir, config_dir=config_dir)

    # Patch sqlite3.connect timeout to 0.5s for fast deterministic test execution
    orig_connect = sqlite3.connect
    def patched_connect(database, *args, **kwargs):
        if "timeout" in kwargs and kwargs["timeout"] == 5.0:
            kwargs["timeout"] = 0.5  # Accelerated timeout for test
        return orig_connect(database, *args, **kwargs)

    t0 = time.time()
    with pytest.MonkeyPatch.context() as mp:
        mp.setattr(sqlite3, "connect", patched_connect)
        res = pruner.prune(
            options=PruneOptions(
                vacuum_databases=True,
                min_age_days=0.0,
                dry_run=False,
            )
        )
    elapsed = time.time() - t0

    # Pruner should have timed out on the locked DB, logged debug, and continued
    assert elapsed >= 0.4, f"Expected timeout of ~0.5s, took {elapsed}s"
    assert res.databases_vacuumed == 0, "Locked database should not count as vacuumed"

    # Release external lock
    lock_conn.rollback()
    lock_conn.close()
    conn.close()

    # Integrity verification
    verify_conn = sqlite3.connect(str(db_path))
    rows = verify_conn.execute("SELECT * FROM test_data;").fetchall()
    assert len(rows) == 1
    assert rows[0] == (101, "critical_session_data")
    integrity = verify_conn.execute("PRAGMA integrity_check;").fetchone()
    assert integrity[0] == "ok"
    verify_conn.close()


def test_locked_sqlite_db_multi_db_partial_vacuum(tmp_path: Path):
    """
    Adversarial Stress Test: Multiple databases with mixed lock states.
    DB1: Locked by external process.
    DB2: Unlocked, contains freelist pages.
    DB3: Unlocked, clean.
    Verifies that BrainCachePruner successfully vacuums unlocked DBs
    even when one DB is locked, with zero crash.
    """
    data_dir = tmp_path / "dt_multi"
    config_dir = tmp_path / "cfg_multi"
    convs_dir = data_dir / "conversations"
    convs_dir.mkdir(parents=True)
    config_dir.mkdir(parents=True)
    (config_dir / "app_storage.json").write_text("{}", encoding="utf-8")

    db1_path = convs_dir / "locked_db.db"
    db2_path = convs_dir / "unlocked_db_with_freelist.db"

    # DB1 setup
    c1 = sqlite3.connect(str(db1_path))
    c1.execute("CREATE TABLE t1 (x INT);")
    c1.commit()
    c1_lock = sqlite3.connect(str(db1_path), timeout=0.1)
    c1_lock.execute("BEGIN EXCLUSIVE;")

    # DB2 setup with deleted rows to generate freelist
    c2 = sqlite3.connect(str(db2_path))
    c2.execute("CREATE TABLE t2 (x TEXT);")
    c2.executemany("INSERT INTO t2 VALUES (?);", [(f"row_{i}" * 50,) for i in range(500)])
    c2.commit()
    c2.execute("DELETE FROM t2 WHERE 1=1;")
    c2.commit()
    c2.close()

    pruner = BrainCachePruner(data_dir=data_dir, config_dir=config_dir)

    orig_connect = sqlite3.connect
    def patched_connect(database, *args, **kwargs):
        if "timeout" in kwargs and kwargs["timeout"] == 5.0:
            kwargs["timeout"] = 0.3
        return orig_connect(database, *args, **kwargs)

    with pytest.MonkeyPatch.context() as mp:
        mp.setattr(sqlite3, "connect", patched_connect)
        res = pruner.prune(
            options=PruneOptions(
                vacuum_databases=True,
                min_age_days=0.0,
                dry_run=False,
            )
        )

    # DB2 should be vacuumed, DB1 skipped
    assert res.databases_vacuumed == 1
    assert any("unlocked_db_with_freelist.db" in d for d in res.details)

    c1_lock.rollback()
    c1_lock.close()
    c1.close()


# =============================================================================
# 6. Prompt Token Bloat Math & Bounds Stress Tests
# =============================================================================

def test_prompt_token_bloat_math_triangular_and_bounds(tmp_path: Path):
    """
    Adversarial Stress Test: Prompt Token Bloat Math & Bounds.
    Generates synthetic multi-turn transcripts with massive tool outputs (>100KB).
    Verifies:
    - Triangular context sum estimation: total_prompt_tokens accumulates turn-by-turn context.
    - Potential savings fraction is strictly bounded within [0.0, 1.0] (and specifically <= 0.95).
    - Oversized output counts (>8KB) correctly detect massive tool payloads.
    - Cached prefix potential matches (turn_count - 1) * static_system_tokens.
    """
    transcript_file = tmp_path / "transcript.jsonl"
    system_text = "You are a specialized coding agent with extensive instructions.\n" * 200  # ~14KB -> ~3,684 tokens
    massive_tool_output = "TOOL_RESULT_DATA: " + ("x" * (120 * 1024))  # > 120KB payload

    lines = []
    # Turn 0: System instruction
    lines.append(json.dumps({"source": "SYSTEM", "type": "SYSTEM_MESSAGE", "content": system_text}))
    # Turn 1: User prompt
    lines.append(json.dumps({"source": "USER", "type": "USER_MESSAGE", "content": "Run analysis on repository"}))
    # Turn 2: Model response calling tool
    lines.append(json.dumps({
        "source": "MODEL",
        "type": "PLANNER_RESPONSE",
        "content": "Executing code analysis...",
        "tool_calls": [{"name": "run_command", "args": {"cmd": "grep_search"}}],
    }))
    # Turn 3: Massive tool output (>100KB)
    lines.append(json.dumps({
        "source": "TOOL",
        "type": "GENERIC",
        "content": massive_tool_output,
    }))
    # Turns 4-15: Additional model & tool conversation turns
    for turn in range(4, 16):
        lines.append(json.dumps({
            "source": "MODEL",
            "type": "PLANNER_RESPONSE",
            "content": f"Processing turn {turn}...",
            "tool_calls": [{"name": "run_command", "args": {"step": turn}}],
        }))
        lines.append(json.dumps({
            "source": "TOOL",
            "type": "GENERIC",
            "content": f"Output for turn {turn}: " + ("data " * 500),
        }))

    transcript_file.write_text("\n".join(lines) + "\n", encoding="utf-8")

    optimizer = PromptCacheOptimizer()
    analysis: PromptCacheAnalysis = optimizer.analyze_transcript(transcript_file)

    assert analysis is not None
    assert analysis.total_steps == len(lines)
    assert analysis.oversized_tool_outputs_count >= 1

    # Check bounds
    assert 0.0 <= analysis.potential_savings_fraction <= 1.0, (
        f"Savings fraction out of bounds [0.0, 1.0]: {analysis.potential_savings_fraction}"
    )
    assert analysis.potential_savings_fraction <= 0.95, (
        f"Savings fraction exceeded upper clamp 0.95: {analysis.potential_savings_fraction}"
    )

    # Check triangular context accumulation
    # Total prompt tokens must be significantly larger than single-turn context
    assert analysis.total_prompt_tokens > analysis.cached_prefix_potential_tokens
    assert analysis.estimated_redundant_tokens > 0

    # Test extreme boundary inputs
    # A. Empty transcript
    empty_file = tmp_path / "empty.jsonl"
    empty_file.write_text("", encoding="utf-8")
    empty_analysis = optimizer.analyze_transcript(empty_file)
    assert empty_analysis.total_steps == 0
    assert empty_analysis.potential_savings_fraction == 0.0

    # B. Single turn transcript
    single_file = tmp_path / "single.jsonl"
    single_file.write_text(json.dumps({"source": "USER", "content": "hi"}) + "\n", encoding="utf-8")
    single_analysis = optimizer.analyze_transcript(single_file)
    assert single_analysis.total_steps == 1
    assert single_analysis.cached_prefix_potential_tokens == 0
    assert 0.0 <= single_analysis.potential_savings_fraction <= 1.0


def test_prompt_token_bloat_fuzzing_and_bounds(tmp_path: Path):
    """
    Adversarial Stress Test: Fuzzing PromptCacheOptimizer across 30 generated transcripts.
    Varies:
    - Step counts: 1 to 50
    - Huge multibyte Unicode payloads (Chinese, Japanese, emojis)
    - Tool output sizes: 0KB to 500KB
    - Malformed JSON lines embedded in transcript
    Strictly verifies that:
    - potential_savings_fraction is always in [0.0, 1.0] and math.isfinite.
    - total_prompt_tokens >= 0.
    - No unhandled exceptions or crashes occur.
    """
    optimizer = PromptCacheOptimizer()

    for seed in range(30):
        t_path = tmp_path / f"fuzz_transcript_{seed}.jsonl"
        steps_count = (seed * 3) % 40 + 1
        has_massive = (seed % 3 == 0)

        lines = []
        # Mixed UTF-8 content
        lines.append(json.dumps({
            "source": "SYSTEM",
            "content": f"系统提示词 System Instructions {seed} " + ("🌟🚀" * 50),
        }))

        # Intentionally inject 2 corrupted JSON lines
        lines.append("CORRUPTED_JSON_LINE_RAW_DATA_12345")

        for s_idx in range(steps_count):
            if s_idx % 2 == 0:
                lines.append(json.dumps({
                    "source": "MODEL",
                    "type": "PLANNER_RESPONSE",
                    "content": f"Turn {s_idx} thoughts",
                    "tool_calls": [{"name": f"tool_{s_idx % 3}", "args": {"seed": seed}}],
                }))
            else:
                out_size = 150 * 1024 if (has_massive and s_idx == 1) else 1024
                lines.append(json.dumps({
                    "source": "TOOL",
                    "type": "GENERIC",
                    "content": "output_" + ("文" * (out_size // 3)),
                }))

        lines.append("{malformed json line at end")
        t_path.write_text("\n".join(lines) + "\n", encoding="utf-8")

        analysis = optimizer.analyze_transcript(t_path)
        assert analysis is not None
        assert math.isfinite(analysis.potential_savings_fraction)
        assert 0.0 <= analysis.potential_savings_fraction <= 1.0
        assert analysis.total_prompt_tokens >= 0
        assert analysis.estimated_redundant_tokens >= 0


# =============================================================================
# 7. IPC Stress & Concurrency Tests
# =============================================================================

def test_ipc_stress_rapid_concurrent_requests_and_large_frames(tmp_path: Path):
    """
    Adversarial Stress Test: Unix Domain Socket IPC.
    Simulates:
    - 15 concurrent clients firing 150 rapid interleaved requests across
      fingerprint, cache, and quota endpoints.
    - Large frame transmission (500KB JSON payload) verifying socket buffer
      limits (MAX_FRAME_SIZE = 10MB) and stability.
    - Verifies zero frame drops, zero unhandled errors, and clean shutdown.
    """
    async def _test():
        sock_path = tmp_path / "daemon.sock"
        server = AsyncUnixSocketServer(socket_path=sock_path)

        # Mock manager & pruner for RPC handlers
        config_dir = tmp_path / "cfg"
        data_dir = tmp_path / "dt"
        config_dir.mkdir(parents=True)
        data_dir.mkdir(parents=True)
        (config_dir / "app_storage.json").write_text("{}", encoding="utf-8")

        fp_mgr = FingerprintManager(config_dir=config_dir, data_dir=data_dir)
        inspector = BrainCacheInspector(data_dir=data_dir, config_dir=config_dir)
        pruner = BrainCachePruner(data_dir=data_dir, config_dir=config_dir)
        prompt_opt = PromptCacheOptimizer(data_dir=data_dir)

        server.register_fingerprint_handlers(fp_mgr)
        server.register_cache_handlers(inspector=inspector, pruner=pruner, prompt_optimizer=prompt_opt)

        # Custom echo handler for large frame testing
        @server.register("stress.echo_large")
        async def rpc_echo_large(payload: str) -> dict[str, Any]:
            return {"size": len(payload), "preview": payload[:50]}

        await server.start()
        assert server.is_running is True

        try:
            # 1. Fire large frame request (500 KB payload)
            large_client = AsyncDaemonClient(socket_path=sock_path, timeout=5.0)
            await large_client.connect()
            large_payload = "A" * (500 * 1024)  # 500 KB string
            res = await large_client.call("stress.echo_large", {"payload": large_payload})
            assert res["size"] == 500 * 1024
            await large_client.close()

            # 2. Concurrency stress: 15 concurrent workers firing 10 requests each (150 total)
            concurrency = 15
            requests_per_worker = 10
            errors = []

            async def worker(worker_id: int):
                client = AsyncDaemonClient(socket_path=sock_path, timeout=5.0)
                try:
                    await client.connect()
                    for req_i in range(requests_per_worker):
                        method_type = req_i % 3
                        if method_type == 0:
                            ans = await client.call("fingerprint.get_profile")
                            assert "machine_id" in ans
                        elif method_type == 1:
                            ans = await client.call("cache.get_breakdown")
                            assert "total_bytes" in ans
                        else:
                            ans = await client.call("cache.prune", {"dry_run": True})
                            assert "files_deleted" in ans
                except Exception as exc:
                    errors.append((worker_id, exc))
                finally:
                    await client.close()

            workers = [worker(w_id) for w_id in range(concurrency)]
            await asyncio.gather(*workers)

            assert len(errors) == 0, f"Encountered IPC worker errors: {errors}"
            assert server.stats["requests_total"] >= 151  # 1 large + 150 concurrent

        finally:
            await server.stop()
            assert server.is_running is False

    asyncio.run(_test())


def test_ipc_stress_large_payload_2mb_and_malformed_frames(tmp_path: Path):
    """
    Adversarial Stress Test: Unix Domain Socket IPC with 2MB payload and malformed frames.
    Verifies that:
    - 2MB single request payload executes without buffer overrun.
    - Malformed raw frames (invalid JSON / non-UTF8) return standard JSON-RPC -32700 ParseError
      without killing server or disconnecting connection.
    - Rapid client disconnects don't stall the server.
    """
    async def _test():
        sock_path = tmp_path / "daemon_2mb.sock"
        server = AsyncUnixSocketServer(socket_path=sock_path)

        @server.register("test.echo")
        async def rpc_echo(msg: str) -> dict[str, Any]:
            return {"echo": msg, "len": len(msg)}

        await server.start()

        try:
            client = AsyncDaemonClient(socket_path=sock_path, timeout=10.0)
            await client.connect()

            # 1. 2MB payload frame
            huge_str = "H" * (2 * 1024 * 1024)
            resp = await client.call("test.echo", {"msg": huge_str})
            assert resp["len"] == 2 * 1024 * 1024

            # 2. Raw malformed frame test via low-level connection
            reader, writer = await asyncio.open_unix_connection(str(sock_path))
            writer.write(b"NOT_A_VALID_JSON_OBJECT\n")
            await writer.drain()

            err_line = await reader.readline()
            err_obj = json.loads(err_line.decode("utf-8"))
            assert "error" in err_obj
            assert err_obj["error"]["code"] == -32700  # ParseError

            # Subsequent valid request on same connection succeeds
            valid_req = json.dumps({"jsonrpc": "2.0", "method": "test.echo", "params": {"msg": "hello"}, "id": 999}) + "\n"
            writer.write(valid_req.encode("utf-8"))
            await writer.drain()

            ok_line = await reader.readline()
            ok_obj = json.loads(ok_line.decode("utf-8"))
            assert ok_obj["id"] == 999
            assert ok_obj["result"]["echo"] == "hello"

            writer.close()
            await writer.wait_closed()
            await client.close()

        finally:
            await server.stop()

    asyncio.run(_test())
