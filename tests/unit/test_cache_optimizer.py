"""
Unit tests for Brain & Context Cache Optimizer (Requirement R5).
================================================================
"""

import datetime
import json
import os
from pathlib import Path
import time
import pytest

from antigravity_swiss.cache_optimizer.inspector import CacheInspector
from antigravity_swiss.cache_optimizer.models import PruneOptions
from antigravity_swiss.cache_optimizer.prompt_cache import PromptCacheOptimizer
from antigravity_swiss.cache_optimizer.pruner import CachePruner


def _setup_mock_cache_structure(base_dir: Path, active_id: str, stale_id: str) -> None:
    brain_dir = base_dir / "brain"
    convs_dir = base_dir / "conversations"

    # 1. Active conversation
    act_dir = brain_dir / active_id
    (act_dir / "scratch").mkdir(parents=True)
    (act_dir / "scratch" / "test.py").write_text("print('active')", encoding="utf-8")
    (act_dir / ".system_generated" / "steps").mkdir(parents=True)
    (act_dir / ".system_generated" / "steps" / "step_1.txt").write_text("x" * 500, encoding="utf-8")
    (act_dir / ".system_generated" / "logs").mkdir(parents=True)
    (act_dir / ".system_generated" / "logs" / "transcript.jsonl").write_text("{}\n", encoding="utf-8")

    # 2. Stale conversation
    stale_dir = brain_dir / stale_id
    (stale_dir / "scratch").mkdir(parents=True)
    (stale_dir / "scratch" / "old.py").write_text("print('stale')", encoding="utf-8")
    (stale_dir / ".system_generated" / "steps").mkdir(parents=True)
    (stale_dir / ".system_generated" / "steps" / "step_old.txt").write_text("y" * 1500, encoding="utf-8")
    (stale_dir / ".system_generated" / "tasks").mkdir(parents=True)
    (stale_dir / ".system_generated" / "tasks" / "task_1.log").write_text("done", encoding="utf-8")
    (stale_dir / ".system_generated" / "logs").mkdir(parents=True)
    (stale_dir / ".system_generated" / "logs" / "transcript.jsonl").write_text("{}\n", encoding="utf-8")

    # Set old mtime on stale directory (10 days ago)
    old_time = time.time() - (10 * 86400)
    os.utime(stale_dir, (old_time, old_time))

    # 3. Conversations DBs
    convs_dir.mkdir(parents=True)
    (convs_dir / f"{active_id}.db").write_bytes(b"0" * 4096)
    (convs_dir / f"{stale_id}.db").write_bytes(b"0" * 8192)


def test_cache_inspector_breakdown(temp_dir):
    """Verify CacheInspector correctly categorizes files and detects active conversation."""
    base_dir = Path(temp_dir)
    active_id = "11111111-1111-1111-1111-111111111111"
    stale_id = "22222222-2222-2222-2222-222222222222"

    _setup_mock_cache_structure(base_dir, active_id, stale_id)

    inspector = CacheInspector(data_dir=base_dir)
    breakdown = inspector.scan_breakdown(active_conversation_id=active_id)

    assert breakdown.conversation_count == 2
    assert breakdown.brain_total_bytes > 0
    assert breakdown.conversations_total_bytes == (4096 + 8192)
    assert breakdown.active_session_bytes > 0

    # Stale conversation steps and scratch should be counted as reclaimable
    assert breakdown.reclaimable_bytes >= 1500
    assert "steps" in breakdown.categories
    assert "scratch" in breakdown.categories
    assert "databases" in breakdown.categories
    assert breakdown.categories["databases"].total_bytes == (4096 + 8192)


def test_cache_pruner_safe_cleanup(temp_dir):
    """Verify CachePruner prunes stale directories while strictly preserving active session."""
    base_dir = Path(temp_dir)
    active_id = "11111111-1111-1111-1111-111111111111"
    stale_id = "22222222-2222-2222-2222-222222222222"

    _setup_mock_cache_structure(base_dir, active_id, stale_id)

    pruner = CachePruner(data_dir=base_dir)

    # 1. Dry run
    res_dry = pruner.prune(
        options=PruneOptions(min_age_days=5.0, dry_run=True),
        active_conversation_id=active_id,
    )
    assert res_dry.dry_run is True
    assert res_dry.reclaimed_bytes > 0
    # Files must still exist after dry run
    assert (base_dir / "brain" / stale_id / "scratch" / "old.py").exists()

    # 2. Live prune
    res_live = pruner.prune(
        options=PruneOptions(min_age_days=5.0, dry_run=False),
        active_conversation_id=active_id,
    )
    assert res_live.dry_run is False
    assert res_live.reclaimed_bytes > 0
    assert res_live.pruned_files_count > 0

    # Stale scratch and steps pruned
    assert not (base_dir / "brain" / stale_id / "scratch").exists()
    assert not (base_dir / "brain" / stale_id / ".system_generated" / "steps").exists()

    # Stale logs preserved!
    assert (base_dir / "brain" / stale_id / ".system_generated" / "logs" / "transcript.jsonl").exists()

    # ACTIVE SESSION COMPLETELY UNTOUCHED!
    assert (base_dir / "brain" / active_id / "scratch" / "test.py").exists()
    assert (base_dir / "brain" / active_id / ".system_generated" / "steps" / "step_1.txt").exists()


def test_prompt_cache_optimizer_analysis(temp_dir):
    """Verify PromptCacheOptimizer detects oversized step outputs in transcripts."""
    transcript_file = Path(temp_dir) / "brain" / "conv-uuid" / ".system_generated" / "logs" / "transcript.jsonl"
    transcript_file.parent.mkdir(parents=True)

    # Create dummy steps: one normal, one with 20KB stdout
    steps = [
        {"step_index": 1, "type": "USER_INPUT", "content": "hello world"},
        {"step_index": 2, "type": "GENERIC", "content": "A" * 20000},
    ]
    with open(transcript_file, "w", encoding="utf-8") as f:
        for s in steps:
            f.write(json.dumps(s) + "\n")

    opt = PromptCacheOptimizer()
    report = opt.analyze_transcript(transcript_file)
    assert report is not None
    assert report.total_steps == 2
    assert report.oversized_tool_outputs_count == 1
    assert report.bloat_tokens > 0
    assert report.savings_potential_fraction > 0.0
