"""
Unit tests for Brain & Context Cache Optimizer (Requirement R5, Features F12, F13, F14).
========================================================================================
"""

import datetime
import json
import os
from pathlib import Path
import sqlite3
import time
import pytest

from antigravity_swiss.cache_optimizer.inspector import BrainCacheInspector, CacheInspector
from antigravity_swiss.cache_optimizer.models import (
    CacheBreakdown,
    CacheCategory,
    CacheCategoryUsage,
    CacheItem,
    PruneOptions,
    PruneResult,
    PromptCacheAnalysis,
)
from antigravity_swiss.cache_optimizer.prompt_cache import PromptCacheOptimizer
from antigravity_swiss.cache_optimizer.pruner import BrainCachePruner, CachePruner


def _setup_mock_cache_structure(
    base_dir: Path,
    active_id: str,
    stale_id: str,
    pinned_id: str | None = None,
) -> None:
    brain_dir = base_dir / "brain"
    convs_dir = base_dir / "conversations"

    # Configure app_storage.json with active and pinned
    config_dir = base_dir / "config"
    config_dir.mkdir(parents=True, exist_ok=True)
    app_storage_file = config_dir / "app_storage.json"
    storage_data = {
        f"antigravity-multi-conversation-layout-v3-{active_id}": json.dumps({"cascadeId": active_id}),
    }
    if pinned_id:
        storage_data["pinned_conversations_order"] = json.dumps([pinned_id])
    app_storage_file.write_text(json.dumps(storage_data), encoding="utf-8")

    # 1. Active conversation
    act_dir = brain_dir / active_id
    (act_dir / "scratch").mkdir(parents=True)
    (act_dir / "scratch" / "test.py").write_text("print('active')", encoding="utf-8")
    (act_dir / ".system_generated" / "steps").mkdir(parents=True)
    (act_dir / ".system_generated" / "steps" / "step_1.txt").write_text("x" * 500, encoding="utf-8")
    (act_dir / ".system_generated" / "logs").mkdir(parents=True)
    (act_dir / ".system_generated" / "logs" / "transcript.jsonl").write_text("{}\n", encoding="utf-8")
    (act_dir / ".user_uploaded").mkdir(parents=True)
    (act_dir / ".user_uploaded" / "media_1.png").write_bytes(b"\x89PNG\r\n\x1a\n" + b"0" * 2000)

    # 2. Pinned conversation
    if pinned_id:
        pin_dir = brain_dir / pinned_id
        (pin_dir / "scratch").mkdir(parents=True)
        (pin_dir / "scratch" / "pinned.py").write_text("print('pinned')", encoding="utf-8")
        (pin_dir / ".system_generated" / "steps").mkdir(parents=True)
        (pin_dir / ".system_generated" / "steps" / "step_p.txt").write_text("p" * 600, encoding="utf-8")
        (pin_dir / ".system_generated" / "logs").mkdir(parents=True)
        (pin_dir / ".system_generated" / "logs" / "transcript.jsonl").write_text("{}\n", encoding="utf-8")
        old_time = time.time() - (15 * 86400)
        os.utime(pin_dir, (old_time, old_time))

    # 3. Stale conversation
    stale_dir = brain_dir / stale_id
    (stale_dir / "scratch").mkdir(parents=True)
    (stale_dir / "scratch" / "old.py").write_text("print('stale')", encoding="utf-8")
    (stale_dir / ".system_generated" / "steps").mkdir(parents=True)
    (stale_dir / ".system_generated" / "steps" / "step_old.txt").write_text("y" * 1500, encoding="utf-8")
    (stale_dir / ".system_generated" / "tasks").mkdir(parents=True)
    (stale_dir / ".system_generated" / "tasks" / "task_1.log").write_text("done", encoding="utf-8")
    (stale_dir / ".system_generated" / "logs").mkdir(parents=True)
    (stale_dir / ".system_generated" / "logs" / "transcript.jsonl").write_text("{}\n", encoding="utf-8")
    (stale_dir / ".user_uploaded").mkdir(parents=True)
    (stale_dir / ".user_uploaded" / "media_old.png").write_bytes(b"\x89PNG\r\n\x1a\n" + b"1" * 3000)

    # Set old mtime on stale directory (10 days ago)
    old_time = time.time() - (10 * 86400)
    os.utime(stale_dir, (old_time, old_time))

    # 4. Conversations DBs
    convs_dir.mkdir(parents=True, exist_ok=True)
    (convs_dir / f"{active_id}.db").write_bytes(b"0" * 4096)
    (convs_dir / f"{stale_id}.db").write_bytes(b"0" * 8192)


def test_cache_models_and_enums():
    """Verify CacheCategory enum, CacheItem, CacheBreakdown, and PruneResult properties."""
    assert CacheCategory.SCREENSHOTS.value == "screenshots"
    assert CacheCategory.SCRATCHPADS.value == "scratchpads"
    assert CacheCategory.TOOL_LOGS.value == "tool_logs"
    assert CacheCategory.STEP_OUTPUTS.value == "step_outputs"
    assert CacheCategory.TRANSCRIPTS.value == "transcripts"
    assert CacheCategory.DATABASES.value == "databases"

    item = CacheItem(
        path=Path("/tmp/media.png"),
        category=CacheCategory.SCREENSHOTS,
        size_bytes=2048,
        mtime=datetime.datetime.now(datetime.timezone.utc),
        conversation_id="conv-1",
        is_active=False,
        is_pinned=False,
        is_prunable=True,
    )
    assert item.size_bytes == 2048

    res = PruneResult(
        bytes_freed=50000,
        files_deleted=12,
        categories_affected=["scratchpads", "step_outputs"],
        elapsed_seconds=0.045,
        protected_active_id="act-1",
        protected_pinned_ids=["pin-1"],
        dry_run=False,
        databases_vacuumed=1,
        details=["Pruned scratch directory"],
    )
    assert res.bytes_freed == 50000
    assert res.reclaimed_bytes == 50000
    assert res.files_deleted == 12
    assert res.pruned_files_count == 12
    assert res.pruned_directories_count == 1
    d = res.to_dict()
    assert d["bytes_freed"] == 50000
    assert d["reclaimed_bytes"] == 50000
    assert d["protected_active_id"] == "act-1"


def test_cache_inspector_breakdown(temp_dir):
    """Verify BrainCacheInspector correctly categorizes files and detects active and pinned conversations."""
    base_dir = Path(temp_dir)
    active_id = "11111111-1111-1111-1111-111111111111"
    stale_id = "22222222-2222-2222-2222-222222222222"
    pinned_id = "33333333-3333-3333-3333-333333333333"

    _setup_mock_cache_structure(base_dir, active_id, stale_id, pinned_id=pinned_id)

    inspector = BrainCacheInspector(data_dir=base_dir, config_dir=base_dir / "config")
    breakdown = inspector.scan_breakdown()

    assert breakdown.conversation_count == 3
    assert breakdown.brain_total_bytes > 0
    assert breakdown.conversations_total_bytes == (4096 + 8192)
    assert breakdown.active_session_bytes > 0
    assert breakdown.item_count > 0
    assert breakdown.oldest_timestamp is not None
    assert breakdown.newest_timestamp is not None

    # Stale conversation steps, scratch, and media should be counted as reclaimable
    assert breakdown.reclaimable_bytes >= 4500

    # Check both enum keys and legacy keys in categories
    assert "steps" in breakdown.categories
    assert "scratch" in breakdown.categories
    assert "databases" in breakdown.categories
    assert CacheCategory.SCREENSHOTS.value in breakdown.categories
    assert CacheCategory.STEP_OUTPUTS.value in breakdown.categories
    assert CacheCategory.SCRATCHPADS.value in breakdown.categories
    assert CacheCategory.TRANSCRIPTS.value in breakdown.categories


def test_cache_pruner_safe_cleanup(temp_dir):
    """Verify BrainCachePruner prunes stale directories while strictly preserving active and pinned sessions."""
    base_dir = Path(temp_dir)
    active_id = "11111111-1111-1111-1111-111111111111"
    stale_id = "22222222-2222-2222-2222-222222222222"
    pinned_id = "33333333-3333-3333-3333-333333333333"

    _setup_mock_cache_structure(base_dir, active_id, stale_id, pinned_id=pinned_id)

    pruner = BrainCachePruner(data_dir=base_dir, config_dir=base_dir / "config")

    # 1. Dry run
    res_dry = pruner.prune(
        options=PruneOptions(min_age_days=5.0, dry_run=True),
    )
    assert res_dry.dry_run is True
    assert res_dry.bytes_freed > 0
    assert res_dry.reclaimed_bytes > 0
    assert res_dry.protected_active_id == active_id
    assert pinned_id in res_dry.protected_pinned_ids
    # Files must still exist after dry run
    assert (base_dir / "brain" / stale_id / "scratch" / "old.py").exists()

    # 2. Live prune
    res_live = pruner.prune(
        options=PruneOptions(min_age_days=5.0, dry_run=False),
    )
    assert res_live.dry_run is False
    assert res_live.bytes_freed > 0
    assert res_live.files_deleted > 0
    assert res_live.elapsed_seconds >= 0.0

    # Stale scratch, steps, tasks, media pruned
    assert not (base_dir / "brain" / stale_id / "scratch").exists()
    assert not (base_dir / "brain" / stale_id / ".system_generated" / "steps").exists()
    assert not (base_dir / "brain" / stale_id / ".user_uploaded").exists()

    # Stale transcripts unconditionally preserved!
    assert (base_dir / "brain" / stale_id / ".system_generated" / "logs" / "transcript.jsonl").exists()

    # ACTIVE SESSION COMPLETELY UNTOUCHED!
    assert (base_dir / "brain" / active_id / "scratch" / "test.py").exists()
    assert (base_dir / "brain" / active_id / ".system_generated" / "steps" / "step_1.txt").exists()
    assert (base_dir / "brain" / active_id / ".user_uploaded" / "media_1.png").exists()

    # PINNED SESSION COMPLETELY UNTOUCHED!
    assert (base_dir / "brain" / pinned_id / "scratch" / "pinned.py").exists()
    assert (base_dir / "brain" / pinned_id / ".system_generated" / "steps" / "step_p.txt").exists()


def test_cache_pruner_sqlite_vacuum(temp_dir):
    """Verify BrainCachePruner safely executes VACUUM on eligible inactive databases."""
    base_dir = Path(temp_dir)
    convs_dir = base_dir / "conversations"
    convs_dir.mkdir(parents=True)
    db_path = convs_dir / "stale_vacuum_test.db"

    # Create real SQLite DB with large deleted data to generate freelist pages
    con = sqlite3.connect(str(db_path))
    con.execute("CREATE TABLE big_data (id INTEGER PRIMARY KEY, content TEXT);")
    for i in range(200):
        con.execute("INSERT INTO big_data (content) VALUES (?);", ("X" * 1000,))
    con.commit()
    con.execute("DELETE FROM big_data WHERE id > 10;")
    con.commit()
    con.close()

    initial_size = db_path.stat().st_size
    assert initial_size > 50000

    pruner = BrainCachePruner(data_dir=base_dir, config_dir=base_dir)
    res = pruner.prune(options=PruneOptions(min_age_days=0.0, vacuum_databases=True, dry_run=False))

    final_size = db_path.stat().st_size
    assert final_size < initial_size
    assert res.databases_vacuumed >= 1
    assert res.bytes_freed >= (initial_size - final_size)


def test_cache_pruner_safety_shield(monkeypatch):
    """Verify BrainCachePruner refuses live deletion when pointing to host dirs during tests."""
    from antigravity_swiss.core.constants import DEFAULT_ANTIGRAVITY_CONFIG_DIR, DEFAULT_ANTIGRAVITY_DATA_DIR

    pruner = BrainCachePruner(
        data_dir=DEFAULT_ANTIGRAVITY_DATA_DIR,
        config_dir=DEFAULT_ANTIGRAVITY_CONFIG_DIR,
    )
    assert pruner._is_host_environment_protected() is True

    # Live prune must be converted into dry run
    res = pruner.prune(options=PruneOptions(dry_run=False))
    assert res.dry_run is True


def test_prompt_cache_optimizer_analysis(temp_dir):
    """Verify PromptCacheOptimizer computes triangular context accumulation, prefix potential, and bloat."""
    transcript_file = Path(temp_dir) / "brain" / "conv-uuid" / ".system_generated" / "logs" / "transcript.jsonl"
    transcript_file.parent.mkdir(parents=True)

    # Create multi-turn transcript:
    # System instruction (2000 chars)
    # Turn 1: user input + model response
    # Turn 2: tool execution with oversized 20KB stdout
    # Turn 3: user follow-up + model response with repeated tool call
    steps = [
        {"step_index": 0, "source": "SYSTEM", "type": "SYSTEM_MESSAGE", "content": "You are a helpful software engineer. Follow strict safety protocols. " * 50},
        {"step_index": 1, "source": "USER", "type": "USER_INPUT", "content": "Check project status"},
        {"step_index": 2, "source": "MODEL", "type": "PLANNER_RESPONSE", "thinking": "Let me inspect directory", "tool_calls": [{"name": "run_command", "args": {"cmd": "ls"}}]},
        {"step_index": 3, "source": "TOOL", "type": "GENERIC", "content": "A" * 20000},  # Oversized output (>8KB)
        {"step_index": 4, "source": "USER", "type": "USER_INPUT", "content": "Now run tests"},
        {"step_index": 5, "source": "MODEL", "type": "PLANNER_RESPONSE", "thinking": "Running tests now", "tool_calls": [{"name": "run_command", "args": {"cmd": "pytest"}}]},
    ]
    with open(transcript_file, "w", encoding="utf-8") as f:
        for s in steps:
            f.write(json.dumps(s) + "\n")

    opt = PromptCacheOptimizer()
    report = opt.analyze_transcript(transcript_file)
    assert report is not None
    assert report.total_steps == 6
    assert report.turn_count >= 2
    assert report.total_prompt_tokens > 0
    assert report.estimated_redundant_tokens > 0
    assert report.oversized_tool_outputs_count == 1
    assert report.cached_prefix_potential_tokens > 0
    assert report.potential_savings_fraction > 0.0
    assert len(report.optimization_recommendations) >= 1

    # Check backwards-compatible properties
    assert report.bloat_tokens == report.estimated_redundant_tokens
    assert report.savings_potential_fraction == report.potential_savings_fraction

    d = report.to_dict()
    assert d["total_steps"] == 6
    assert d["potential_savings_percent"] > 0
    assert "breakdown" in d
    assert "optimization_recommendations" in d
