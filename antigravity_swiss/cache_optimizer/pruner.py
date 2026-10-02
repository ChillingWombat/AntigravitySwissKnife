"""
Brain Cache Pruner: Safe Disk Cleanup for Inactive Antigravity Sessions (Feature F13).
======================================================================================
Prunes stale scratchpads, tool execution dumps, completed task logs, and screenshots
while unconditionally protecting active conversations, pinned sessions, and transcripts.
Performs safe SQLite database VACUUM compaction on inactive databases.
"""

from __future__ import annotations

import datetime
import logging
import os
from pathlib import Path
import shutil
import sqlite3
import time
from typing import List, Optional, Set

from antigravity_swiss.cache_optimizer.models import CacheCategory, PruneOptions, PruneResult
from antigravity_swiss.core.constants import (
    DEFAULT_ANTIGRAVITY_CONFIG_DIR,
    DEFAULT_ANTIGRAVITY_DATA_DIR,
)
from antigravity_swiss.session.app_storage import AppStorageManager

logger = logging.getLogger("antigravity_swiss.cache_optimizer.pruner")


class BrainCachePruner:
    """Safely cleans stale caches without disturbing active or pinned conversations."""

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

    def _is_host_environment_protected(self) -> bool:
        """Host Process Shield: Prevent accidental deletion of user files in tests."""
        is_testing = bool(os.environ.get("PYTEST_CURRENT_TEST") or os.environ.get("ANTIGRAVITY_SWISS_TESTING"))
        real_data = Path(DEFAULT_ANTIGRAVITY_DATA_DIR).expanduser().resolve()
        return is_testing and (self.data_dir == real_data)

    def get_active_conversation_id(self) -> Optional[str]:
        """Resolves active conversation ID."""
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

    def prune(
        self,
        options: Optional[PruneOptions] = None,
        active_conversation_id: Optional[str] = None,
    ) -> PruneResult:
        """
        Executes cache cleanup based on options.
        Guarantees that active_conversation_id, pinned conversations,
        and permanent transcripts are completely untouched.
        """
        start_time = time.time()
        opts = options or PruneOptions()
        active_id = active_conversation_id or self.get_active_conversation_id()
        pinned_ids: Set[str] = set(self.get_pinned_conversation_ids())

        # If pointing to host files during testing, treat as a dry run for safety
        dry_run = opts.dry_run or self._is_host_environment_protected()

        bytes_freed = 0
        files_deleted = 0
        categories_affected: Set[str] = set()
        databases_vacuumed = 0
        details: List[str] = []

        now_ts = time.time()
        cutoff_ts = now_ts - (opts.min_age_days * 86400.0)

        if not self.brain_dir.exists() and not (opts.vacuum_databases and self.convs_dir.exists()):
            return PruneResult(
                bytes_freed=0,
                files_deleted=0,
                categories_affected=[],
                elapsed_seconds=round(time.time() - start_time, 4),
                protected_active_id=active_id,
                protected_pinned_ids=sorted(list(pinned_ids)),
                dry_run=dry_run,
                databases_vacuumed=0,
                details=["Brain and conversations directories do not exist."],
            )

        # 1. Prune stale conversation sessions in brain/
        brain_entries = list(self.brain_dir.iterdir()) if self.brain_dir.exists() else []
        for conv_entry in brain_entries:
            if not conv_entry.is_dir():
                continue

            conv_id = conv_entry.name

            # STRICT SHIELD: Never touch active conversation
            if active_id and conv_id == active_id:
                details.append(f"PROTECTED active conversation: {conv_id}")
                continue

            # STRICT SHIELD: Never touch pinned conversation
            if conv_id in pinned_ids:
                details.append(f"PROTECTED pinned conversation: {conv_id}")
                continue

            # Check directory modification time against age cutoff
            try:
                dir_mtime = conv_entry.stat().st_mtime
            except OSError:
                continue

            if dir_mtime > cutoff_ts:
                # Still recent, skip
                continue

            # A. Prune scratch/
            if opts.prune_scratch:
                scratch_dir = conv_entry / "scratch"
                if scratch_dir.exists() and scratch_dir.is_dir():
                    cat_affected = False
                    for root, _, files in os.walk(scratch_dir):
                        for f in files:
                            fp = os.path.join(root, f)
                            try:
                                sz = os.path.getsize(fp)
                                bytes_freed += sz
                                files_deleted += 1
                                cat_affected = True
                                if not dry_run:
                                    os.remove(fp)
                            except OSError:
                                pass
                    if not dry_run:
                        try:
                            shutil.rmtree(scratch_dir, ignore_errors=True)
                        except OSError:
                            pass
                    if cat_affected:
                        categories_affected.add(CacheCategory.SCRATCHPADS.value)
                        details.append(f"Pruned scratch: {conv_id}/scratch")

            # B. Prune .system_generated/steps/
            if opts.prune_steps:
                steps_dir = conv_entry / ".system_generated" / "steps"
                if steps_dir.exists() and steps_dir.is_dir():
                    cat_affected = False
                    for root, _, files in os.walk(steps_dir):
                        for f in files:
                            fp = os.path.join(root, f)
                            try:
                                sz = os.path.getsize(fp)
                                bytes_freed += sz
                                files_deleted += 1
                                cat_affected = True
                                if not dry_run:
                                    os.remove(fp)
                            except OSError:
                                pass
                    if not dry_run:
                        try:
                            shutil.rmtree(steps_dir, ignore_errors=True)
                        except OSError:
                            pass
                    if cat_affected:
                        categories_affected.add(CacheCategory.STEP_OUTPUTS.value)
                        details.append(f"Pruned tool steps: {conv_id}/.system_generated/steps")

            # C. Prune .system_generated/tasks/
            if opts.prune_tasks:
                tasks_dir = conv_entry / ".system_generated" / "tasks"
                if tasks_dir.exists() and tasks_dir.is_dir():
                    cat_affected = False
                    for root, _, files in os.walk(tasks_dir):
                        for f in files:
                            fp = os.path.join(root, f)
                            try:
                                sz = os.path.getsize(fp)
                                bytes_freed += sz
                                files_deleted += 1
                                cat_affected = True
                                if not dry_run:
                                    os.remove(fp)
                            except OSError:
                                pass
                    if not dry_run:
                        try:
                            shutil.rmtree(tasks_dir, ignore_errors=True)
                        except OSError:
                            pass
                    if cat_affected:
                        categories_affected.add(CacheCategory.TOOL_LOGS.value)
                        details.append(f"Pruned background tasks: {conv_id}/.system_generated/tasks")

            # D. Prune screenshots and uploaded media
            if opts.prune_screenshots:
                media_dirs = [
                    conv_entry / ".user_uploaded",
                    conv_entry / "media",
                ]
                for m_dir in media_dirs:
                    if m_dir.exists() and m_dir.is_dir():
                        cat_affected = False
                        for root, _, files in os.walk(m_dir):
                            for f in files:
                                fp = os.path.join(root, f)
                                try:
                                    sz = os.path.getsize(fp)
                                    bytes_freed += sz
                                    files_deleted += 1
                                    cat_affected = True
                                    if not dry_run:
                                        os.remove(fp)
                                except OSError:
                                    pass
                        if not dry_run:
                            try:
                                shutil.rmtree(m_dir, ignore_errors=True)
                            except OSError:
                                pass
                        if cat_affected:
                            categories_affected.add(CacheCategory.SCREENSHOTS.value)
                            details.append(f"Pruned media: {conv_id}/{m_dir.name}")

        # 2. Prune global tempmediaStorage
        if opts.prune_screenshots:
            global_tempmedia = self.brain_dir / "tempmediaStorage"
            if global_tempmedia.exists() and global_tempmedia.is_dir():
                cat_affected = False
                for root, _, files in os.walk(global_tempmedia):
                    for f in files:
                        fp = os.path.join(root, f)
                        try:
                            st = os.stat(fp)
                            if st.st_mtime <= cutoff_ts:
                                bytes_freed += st.st_size
                                files_deleted += 1
                                cat_affected = True
                                if not dry_run:
                                    os.remove(fp)
                        except OSError:
                            pass
                if cat_affected:
                    categories_affected.add(CacheCategory.SCREENSHOTS.value)
                    details.append("Pruned global tempmediaStorage")

        # 3. SQLite Database Compaction (VACUUM)
        if opts.vacuum_databases and self.convs_dir.exists():
            for db_path in self.convs_dir.glob("*.db"):
                # Strictly protect active session and pinned session DBs from VACUUM locking
                db_id = db_path.stem
                if (active_id and db_id == active_id) or (db_id in pinned_ids):
                    continue

                try:
                    if dry_run:
                        # Estimate freelist space in dry run
                        con = sqlite3.connect(f"file:{db_path}?mode=ro", uri=True, timeout=1.0)
                        try:
                            cur = con.cursor()
                            page_count = cur.execute("PRAGMA freelist_count;").fetchone()[0]
                            page_size = cur.execute("PRAGMA page_size;").fetchone()[0]
                            est_freed = page_count * page_size
                            if est_freed > 0:
                                bytes_freed += est_freed
                                databases_vacuumed += 1
                                categories_affected.add(CacheCategory.DATABASES.value)
                                details.append(f"Dry-run VACUUM candidate: {db_path.name} (~{est_freed} bytes)")
                        finally:
                            con.close()
                    else:
                        sz_before = db_path.stat().st_size
                        con = sqlite3.connect(str(db_path), timeout=5.0)
                        try:
                            con.execute("PRAGMA wal_checkpoint(TRUNCATE);")
                            con.execute("VACUUM;")
                            con.commit()
                        finally:
                            con.close()
                        sz_after = db_path.stat().st_size
                        freed = max(0, sz_before - sz_after)
                        bytes_freed += freed
                        databases_vacuumed += 1
                        categories_affected.add(CacheCategory.DATABASES.value)
                        details.append(f"Vacuumed database: {db_path.name} (freed {freed} bytes)")
                except (sqlite3.OperationalError, sqlite3.DatabaseError, OSError) as exc:
                    logger.debug("Skipping VACUUM on %s: %s", db_path.name, exc)

        elapsed = time.time() - start_time
        return PruneResult(
            bytes_freed=bytes_freed,
            files_deleted=files_deleted,
            categories_affected=sorted(list(categories_affected)),
            elapsed_seconds=round(elapsed, 4),
            protected_active_id=active_id,
            protected_pinned_ids=sorted(list(pinned_ids)),
            dry_run=dry_run,
            databases_vacuumed=databases_vacuumed,
            details=details,
        )


# Backwards compatibility alias
CachePruner = BrainCachePruner
