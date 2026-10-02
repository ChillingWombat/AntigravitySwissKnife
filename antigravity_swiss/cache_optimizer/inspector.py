"""
Brain Cache Inspector: Disk Storage Scanner for Antigravity Brain & Conversations (Feature F12).
================================================================================================
Scans ~/.gemini/antigravity/brain/ and ~/.gemini/antigravity/conversations/,
categorizes storage footprint into discrete cache categories,
tracks item count and timestamps, and computes reclaimable bytes.
"""

from __future__ import annotations

import datetime
import logging
import os
from pathlib import Path
from typing import Dict, List, Optional, Set

from antigravity_swiss.cache_optimizer.models import (
    CacheBreakdown,
    CacheCategory,
    CacheCategoryUsage,
    CacheItem,
    ConversationCacheSummary,
)
from antigravity_swiss.core.constants import (
    DEFAULT_ANTIGRAVITY_CONFIG_DIR,
    DEFAULT_ANTIGRAVITY_DATA_DIR,
)
from antigravity_swiss.session.app_storage import AppStorageManager

logger = logging.getLogger("antigravity_swiss.cache_optimizer.inspector")


class BrainCacheInspector:
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
        self.app_storage = AppStorageManager(
            app_storage_path=self.config_dir / "app_storage.json",
            conv_summaries_db=self.data_dir / "conversation_summaries.db",
        )

    def get_active_conversation_id(self) -> Optional[str]:
        """Reads active conversation ID from app_storage.json if available."""
        try:
            return self.app_storage.get_active_conversation_id()
        except Exception:
            return None

    def get_active_cascade_id(self) -> Optional[str]:
        """Alias for get_active_conversation_id."""
        return self.get_active_conversation_id()

    def get_pinned_conversation_ids(self) -> List[str]:
        """Reads pinned conversation IDs from app_storage.json."""
        try:
            return self.app_storage.get_pinned_conversation_ids()
        except Exception:
            return []

    def scan_breakdown(self, active_conversation_id: Optional[str] = None) -> CacheBreakdown:
        """
        Executes a comprehensive disk scan and returns structured breakdown.
        """
        active_id = active_conversation_id or self.get_active_conversation_id()
        pinned_ids: Set[str] = set(self.get_pinned_conversation_ids())

        # Category usage accumulators (for both enum names and legacy names)
        cat_data: Dict[str, Dict[str, Any]] = {
            # Enum categories
            CacheCategory.SCREENSHOTS.value: {"bytes": 0, "files": 0, "reclaimable": 0, "oldest": None, "newest": None, "desc": "Screenshots and user uploaded media (.png, .jpg)"},
            CacheCategory.SCRATCHPADS.value: {"bytes": 0, "files": 0, "reclaimable": 0, "oldest": None, "newest": None, "desc": "Temporary scratchpad scripts and workspaces in scratch/"},
            CacheCategory.TOOL_LOGS.value: {"bytes": 0, "files": 0, "reclaimable": 0, "oldest": None, "newest": None, "desc": "Tool execution and background task logs in .system_generated/tasks/"},
            CacheCategory.STEP_OUTPUTS.value: {"bytes": 0, "files": 0, "reclaimable": 0, "oldest": None, "newest": None, "desc": "Step outputs and execution dumps in .system_generated/steps/"},
            CacheCategory.TRANSCRIPTS.value: {"bytes": 0, "files": 0, "reclaimable": 0, "oldest": None, "newest": None, "desc": "Permanent conversation logs in .system_generated/logs/transcript*.jsonl"},
            CacheCategory.DATABASES.value: {"bytes": 0, "files": 0, "reclaimable": 0, "oldest": None, "newest": None, "desc": "Per-conversation SQLite databases and summaries (.db, -wal, -shm)"},
            CacheCategory.OTHER.value: {"bytes": 0, "files": 0, "reclaimable": 0, "oldest": None, "newest": None, "desc": "Other cache artifacts, messages, and checkpoints"},
            # Legacy category mappings for 100% backward compatibility
            "steps": {"bytes": 0, "files": 0, "reclaimable": 0, "oldest": None, "newest": None, "desc": "Raw tool execution step stdout/stderr logs in .system_generated/steps/"},
            "scratch": {"bytes": 0, "files": 0, "reclaimable": 0, "oldest": None, "newest": None, "desc": "Temporary debugging and test scratch files in scratch/"},
            "tasks": {"bytes": 0, "files": 0, "reclaimable": 0, "oldest": None, "newest": None, "desc": "Background asynchronous task logs in .system_generated/tasks/"},
            "messages": {"bytes": 0, "files": 0, "reclaimable": 0, "oldest": None, "newest": None, "desc": "Inter-agent subagent messages in .system_generated/messages/"},
            "logs": {"bytes": 0, "files": 0, "reclaimable": 0, "oldest": None, "newest": None, "desc": "Main conversation transcripts in .system_generated/logs/"},
            "artifacts": {"bytes": 0, "files": 0, "reclaimable": 0, "oldest": None, "newest": None, "desc": "User-facing generated code artifacts, reports, and diagrams"},
        }

        brain_total = 0
        conv_total = 0
        active_bytes = 0
        reclaimable_bytes = 0
        total_items = 0
        global_oldest: Optional[datetime.datetime] = None
        global_newest: Optional[datetime.datetime] = None

        conv_summaries: List[ConversationCacheSummary] = []

        def _update_cat(cat_key: str, sz: int, dt: datetime.datetime, is_reclaim: bool) -> None:
            nonlocal global_oldest, global_newest
            entry = cat_data[cat_key]
            entry["bytes"] += sz
            entry["files"] += 1
            if is_reclaim:
                entry["reclaimable"] += sz
            if entry["oldest"] is None or dt < entry["oldest"]:
                entry["oldest"] = dt
            if entry["newest"] is None or dt > entry["newest"]:
                entry["newest"] = dt

            if global_oldest is None or dt < global_oldest:
                global_oldest = dt
            if global_newest is None or dt > global_newest:
                global_newest = dt

        # 1. Scan Brain Directory
        if self.brain_dir.exists():
            for conv_entry in self.brain_dir.iterdir():
                if not conv_entry.is_dir():
                    continue

                conv_id = conv_entry.name
                is_active = (conv_id == active_id)
                is_pinned = (conv_id in pinned_ids)
                is_protected = is_active or is_pinned

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
                        total_items += 1
                        dt = datetime.datetime.fromtimestamp(mtime, tz=datetime.timezone.utc)

                        f_lower = f.lower()
                        # Categorize into Enum and Legacy
                        if f_lower.endswith(".png") or f_lower.endswith(".jpg") or ".user_uploaded" in root or "tempmediastorage" in root.lower():
                            _update_cat(CacheCategory.SCREENSHOTS.value, sz, dt, not is_protected)
                            _update_cat("artifacts", sz, dt, not is_protected)
                            if not is_protected:
                                conv_reclaimable += sz
                        elif "steps" in rel_root:
                            _update_cat(CacheCategory.STEP_OUTPUTS.value, sz, dt, not is_protected)
                            _update_cat("steps", sz, dt, not is_protected)
                            if not is_protected:
                                conv_reclaimable += sz
                        elif "scratch" in rel_root:
                            _update_cat(CacheCategory.SCRATCHPADS.value, sz, dt, not is_protected)
                            _update_cat("scratch", sz, dt, not is_protected)
                            if not is_protected:
                                conv_reclaimable += sz
                        elif "tasks" in rel_root:
                            _update_cat(CacheCategory.TOOL_LOGS.value, sz, dt, not is_protected)
                            _update_cat("tasks", sz, dt, not is_protected)
                            if not is_protected:
                                conv_reclaimable += sz
                        elif "logs" in rel_root or f_lower.startswith("transcript"):
                            _update_cat(CacheCategory.TRANSCRIPTS.value, sz, dt, False)
                            _update_cat("logs", sz, dt, False)
                            # Transcripts are NEVER reclaimable
                        elif "messages" in rel_root:
                            _update_cat(CacheCategory.OTHER.value, sz, dt, not is_protected)
                            _update_cat("messages", sz, dt, not is_protected)
                        else:
                            _update_cat(CacheCategory.OTHER.value, sz, dt, not is_protected)
                            _update_cat("artifacts", sz, dt, not is_protected)

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
                        is_pinned=is_pinned,
                        reclaimable_bytes=conv_reclaimable,
                    )
                )

        # 2. Scan Conversations DBs and Master DB
        conv_summary_map = {c.conversation_id: c for c in conv_summaries}

        if self.convs_dir.exists():
            for f in self.convs_dir.iterdir():
                if f.is_file():
                    try:
                        st = f.stat()
                        sz = st.st_size
                        mtime = st.st_mtime
                    except OSError:
                        continue

                    conv_total += sz
                    total_items += 1
                    dt = datetime.datetime.fromtimestamp(mtime, tz=datetime.timezone.utc)
                    _update_cat(CacheCategory.DATABASES.value, sz, dt, False)

                    base_name = f.name
                    for ext in [".db-wal", ".db-shm", ".db"]:
                        if base_name.endswith(ext):
                            base_name = base_name[:-len(ext)]
                            break

                    if base_name in conv_summary_map:
                        conv_summary_map[base_name].db_bytes += sz
                        if conv_summary_map[base_name].is_active:
                            active_bytes += sz

        # Master summaries db
        master_db = self.data_dir / "conversation_summaries.db"
        if master_db.exists():
            try:
                st = master_db.stat()
                sz = st.st_size
                dt = datetime.datetime.fromtimestamp(st.st_mtime, tz=datetime.timezone.utc)
                conv_total += sz
                total_items += 1
                _update_cat(CacheCategory.DATABASES.value, sz, dt, False)
            except OSError:
                pass

        # Build category objects
        category_objects: Dict[str, CacheCategoryUsage] = {}
        for cat_name, info in cat_data.items():
            category_objects[cat_name] = CacheCategoryUsage(
                category=cat_name,
                total_bytes=info["bytes"],
                file_count=info["files"],
                reclaimable_bytes=info["reclaimable"],
                oldest_timestamp=info["oldest"],
                newest_timestamp=info["newest"],
                description=info["desc"],
            )

        conv_summaries.sort(key=lambda c: c.last_modified, reverse=True)

        return CacheBreakdown(
            brain_total_bytes=brain_total,
            conversations_total_bytes=conv_total,
            active_session_bytes=active_bytes,
            reclaimable_bytes=reclaimable_bytes,
            conversation_count=len(conv_summaries),
            item_count=total_items,
            oldest_timestamp=global_oldest,
            newest_timestamp=global_newest,
            categories=category_objects,
            conversations=conv_summaries,
        )


# Backwards compatibility alias
CacheInspector = BrainCacheInspector
