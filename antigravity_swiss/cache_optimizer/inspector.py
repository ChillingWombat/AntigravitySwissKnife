"""
Cache Inspector: Disk Storage Scanner for Antigravity Brain & Conversations (Feature F14).
===========================================================================================
Scans ~/.gemini/antigravity/brain/ and ~/.gemini/antigravity/conversations/,
categorizes storage footprint, and identifies reclaimable token/scratch caches.
"""

from __future__ import annotations

import datetime
import logging
import os
from pathlib import Path
from typing import Dict, List, Optional

from antigravity_swiss.cache_optimizer.models import (
    CacheBreakdown,
    CacheCategoryUsage,
    ConversationCacheSummary,
)
from antigravity_swiss.core.constants import (
    DEFAULT_ANTIGRAVITY_CONFIG_DIR,
    DEFAULT_ANTIGRAVITY_DATA_DIR,
)
from antigravity_swiss.session.app_storage import AppStorageManager

logger = logging.getLogger("antigravity_swiss.cache_optimizer.inspector")


class CacheInspector:
    """Scans and analyzes disk footprint across Antigravity conversation and brain caches."""

    def __init__(
        self,
        data_dir: Optional[Path | str] = None,
        config_dir: Optional[Path | str] = None,
    ) -> None:
        self.data_dir = Path(data_dir or DEFAULT_ANTIGRAVITY_DATA_DIR).expanduser().resolve()
        self.config_dir = Path(config_dir or DEFAULT_ANTIGRAVITY_CONFIG_DIR).expanduser().resolve()
        self.brain_dir = self.data_dir / "brain"
        self.convs_dir = self.data_dir / "conversations"
        self.app_storage = AppStorageManager(self.config_dir / "app_storage.json")

    def get_active_conversation_id(self) -> Optional[str]:
        """Reads active conversation ID from app_storage.json if available."""
        try:
            return self.app_storage.get_active_cascade_id()
        except Exception:
            return None

    def scan_breakdown(self, active_conversation_id: Optional[str] = None) -> CacheBreakdown:
        """
        Executes a comprehensive disk scan and returns structured breakdown.
        """
        active_id = active_conversation_id or self.get_active_conversation_id()

        cat_bytes = {
            "steps": 0,
            "scratch": 0,
            "tasks": 0,
            "messages": 0,
            "logs": 0,
            "artifacts": 0,
            "databases": 0,
        }
        cat_files = {k: 0 for k in cat_bytes}

        brain_total = 0
        conv_total = 0
        active_bytes = 0
        reclaimable_bytes = 0

        conv_summaries: List[ConversationCacheSummary] = []

        # 1. Scan Brain Directory
        if self.brain_dir.exists():
            for conv_entry in self.brain_dir.iterdir():
                if not conv_entry.is_dir():
                    continue

                conv_id = conv_entry.name
                is_active = (conv_id == active_id)
                conv_brain_bytes = 0
                conv_reclaimable = 0
                last_mtime = 0.0

                for root, _, files in os.walk(conv_entry):
                    rel_root = os.path.relpath(root, conv_entry)
                    for f in files:
                        fp = os.path.join(root, f)
                        try:
                            st = os.stat(fp)
                            sz = st.st_size
                            mtime = st.st_mtime
                            last_mtime = max(last_mtime, mtime)
                        except OSError:
                            continue

                        conv_brain_bytes += sz
                        brain_total += sz

                        # Categorize
                        if "steps" in rel_root:
                            cat_bytes["steps"] += sz
                            cat_files["steps"] += 1
                            if not is_active:
                                conv_reclaimable += sz
                        elif "scratch" in rel_root:
                            cat_bytes["scratch"] += sz
                            cat_files["scratch"] += 1
                            if not is_active:
                                conv_reclaimable += sz
                        elif "tasks" in rel_root:
                            cat_bytes["tasks"] += sz
                            cat_files["tasks"] += 1
                            if not is_active:
                                conv_reclaimable += sz
                        elif "messages" in rel_root:
                            cat_bytes["messages"] += sz
                            cat_files["messages"] += 1
                        elif "logs" in rel_root:
                            cat_bytes["logs"] += sz
                            cat_files["logs"] += 1
                        else:
                            cat_bytes["artifacts"] += sz
                            cat_files["artifacts"] += 1

                reclaimable_bytes += conv_reclaimable
                if is_active:
                    active_bytes += conv_brain_bytes

                dt_mtime = (
                    datetime.datetime.fromtimestamp(last_mtime, tz=datetime.timezone.utc)
                    if last_mtime > 0
                    else datetime.datetime.now(datetime.timezone.utc)
                )

                conv_summaries.append(
                    ConversationCacheSummary(
                        conversation_id=conv_id,
                        last_modified=dt_mtime,
                        brain_bytes=conv_brain_bytes,
                        db_bytes=0,
                        is_active=is_active,
                        reclaimable_bytes=conv_reclaimable,
                    )
                )

        # 2. Scan Conversations DBs
        conv_summary_map = {c.conversation_id: c for c in conv_summaries}

        if self.convs_dir.exists():
            for f in self.convs_dir.iterdir():
                if f.is_file():
                    try:
                        sz = f.stat().st_size
                    except OSError:
                        continue

                    conv_total += sz
                    cat_bytes["databases"] += sz
                    cat_files["databases"] += 1

                    # Associate with conversation summary if matches UUID
                    base_name = f.name
                    for ext in [".db-wal", ".db-shm", ".db"]:
                        if base_name.endswith(ext):
                            base_name = base_name[:-len(ext)]
                            break

                    if base_name in conv_summary_map:
                        conv_summary_map[base_name].db_bytes += sz
                        if conv_summary_map[base_name].is_active:
                            active_bytes += sz

        # Build category descriptors
        category_objects: Dict[str, CacheCategoryUsage] = {
            "steps": CacheCategoryUsage(
                category="steps",
                total_bytes=cat_bytes["steps"],
                file_count=cat_files["steps"],
                description="Raw tool execution step stdout/stderr logs in .system_generated/steps/",
            ),
            "scratch": CacheCategoryUsage(
                category="scratch",
                total_bytes=cat_bytes["scratch"],
                file_count=cat_files["scratch"],
                description="Temporary debugging and test scratch files in scratch/",
            ),
            "tasks": CacheCategoryUsage(
                category="tasks",
                total_bytes=cat_bytes["tasks"],
                file_count=cat_files["tasks"],
                description="Background asynchronous task logs in .system_generated/tasks/",
            ),
            "messages": CacheCategoryUsage(
                category="messages",
                total_bytes=cat_bytes["messages"],
                file_count=cat_files["messages"],
                description="Inter-agent subagent messages in .system_generated/messages/",
            ),
            "logs": CacheCategoryUsage(
                category="logs",
                total_bytes=cat_bytes["logs"],
                file_count=cat_files["logs"],
                description="Main conversation transcripts in .system_generated/logs/",
            ),
            "artifacts": CacheCategoryUsage(
                category="artifacts",
                total_bytes=cat_bytes["artifacts"],
                file_count=cat_files["artifacts"],
                description="User-facing generated code artifacts, reports, and diagrams",
            ),
            "databases": CacheCategoryUsage(
                category="databases",
                total_bytes=cat_bytes["databases"],
                file_count=cat_files["databases"],
                description="Per-session SQLite databases (.db, .db-wal, .db-shm)",
            ),
        }

        conv_summaries.sort(key=lambda c: c.last_modified, reverse=True)

        return CacheBreakdown(
            brain_total_bytes=brain_total,
            conversations_total_bytes=conv_total,
            active_session_bytes=active_bytes,
            reclaimable_bytes=reclaimable_bytes,
            conversation_count=len(conv_summaries),
            categories=category_objects,
            conversations=conv_summaries,
        )
