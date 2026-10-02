"""
Cache Pruner: Safe Disk Cleanup for Inactive Antigravity Sessions (Feature F15).
==============================================================================
Prunes stale scratchpads, tool execution dumps, and completed task logs
while preserving active conversations, transcripts, and artifacts.
"""

from __future__ import annotations

import datetime
import logging
import os
from pathlib import Path
import shutil
import time
from typing import List, Optional

from antigravity_swiss.cache_optimizer.models import PruneOptions, PruneResult
from antigravity_swiss.core.constants import (
    DEFAULT_ANTIGRAVITY_CONFIG_DIR,
    DEFAULT_ANTIGRAVITY_DATA_DIR,
)
from antigravity_swiss.session.app_storage import AppStorageManager

logger = logging.getLogger("antigravity_swiss.cache_optimizer.pruner")


class CachePruner:
    """Safely cleans stale caches without disturbing active conversations."""

    def __init__(
        self,
        data_dir: Optional[Path | str] = None,
        config_dir: Optional[Path | str] = None,
    ) -> None:
        self.data_dir = Path(data_dir or DEFAULT_ANTIGRAVITY_DATA_DIR).expanduser().resolve()
        self.config_dir = Path(config_dir or DEFAULT_ANTIGRAVITY_CONFIG_DIR).expanduser().resolve()
        self.brain_dir = self.data_dir / "brain"
        self.app_storage = AppStorageManager(self.config_dir / "app_storage.json")

    def _is_host_environment_protected(self) -> bool:
        """Host Process Shield: Prevent accidental deletion of user files in tests."""
        is_testing = bool(os.environ.get("PYTEST_CURRENT_TEST") or os.environ.get("ANTIGRAVITY_SWISS_TESTING"))
        real_data = Path(DEFAULT_ANTIGRAVITY_DATA_DIR).expanduser().resolve()
        return is_testing and (self.data_dir == real_data)

    def get_active_conversation_id(self) -> Optional[str]:
        """Resolves active conversation ID."""
        try:
            return self.app_storage.get_active_cascade_id()
        except Exception:
            return None

    def prune(
        self,
        options: Optional[PruneOptions] = None,
        active_conversation_id: Optional[str] = None,
    ) -> PruneResult:
        """
        Executes cache cleanup based on options.
        Guarantees that active_conversation_id is completely untouched.
        """
        opts = options or PruneOptions()
        active_id = active_conversation_id or self.get_active_conversation_id()

        # If pointing to host files during testing, treat as a dry run for safety
        dry_run = opts.dry_run or self._is_host_environment_protected()

        reclaimed_bytes = 0
        pruned_files = 0
        pruned_dirs = 0
        details: List[str] = []

        now_ts = time.time()
        cutoff_ts = now_ts - (opts.min_age_days * 86400.0)

        if not self.brain_dir.exists():
            return PruneResult(
                reclaimed_bytes=0,
                pruned_files_count=0,
                pruned_directories_count=0,
                protected_active_id=active_id,
                dry_run=dry_run,
                details=["Brain directory does not exist."],
            )

        for conv_entry in self.brain_dir.iterdir():
            if not conv_entry.is_dir():
                continue

            conv_id = conv_entry.name

            # STRICT SHIELD: Never touch active conversation
            if active_id and conv_id == active_id:
                details.append(f"PROTECTED active conversation: {conv_id}")
                continue

            # Check directory modification time against age cutoff
            try:
                dir_mtime = conv_entry.stat().st_mtime
            except OSError:
                continue

            if dir_mtime > cutoff_ts:
                # Still recent, skip
                continue

            # Prune scratch/
            if opts.prune_scratch:
                scratch_dir = conv_entry / "scratch"
                if scratch_dir.exists() and scratch_dir.is_dir():
                    for root, _, files in os.walk(scratch_dir):
                        for f in files:
                            fp = os.path.join(root, f)
                            try:
                                sz = os.path.getsize(fp)
                                reclaimed_bytes += sz
                                pruned_files += 1
                                if not dry_run:
                                    os.remove(fp)
                            except OSError:
                                pass
                    if not dry_run:
                        try:
                            shutil.rmtree(scratch_dir, ignore_errors=True)
                            pruned_dirs += 1
                        except OSError:
                            pass
                    details.append(f"Pruned scratch: {conv_id}/scratch")

            # Prune .system_generated/steps/
            if opts.prune_steps:
                steps_dir = conv_entry / ".system_generated" / "steps"
                if steps_dir.exists() and steps_dir.is_dir():
                    for root, _, files in os.walk(steps_dir):
                        for f in files:
                            fp = os.path.join(root, f)
                            try:
                                sz = os.path.getsize(fp)
                                reclaimed_bytes += sz
                                pruned_files += 1
                                if not dry_run:
                                    os.remove(fp)
                            except OSError:
                                pass
                    if not dry_run:
                        try:
                            shutil.rmtree(steps_dir, ignore_errors=True)
                            pruned_dirs += 1
                        except OSError:
                            pass
                    details.append(f"Pruned tool steps: {conv_id}/.system_generated/steps")

            # Prune .system_generated/tasks/
            if opts.prune_tasks:
                tasks_dir = conv_entry / ".system_generated" / "tasks"
                if tasks_dir.exists() and tasks_dir.is_dir():
                    for root, _, files in os.walk(tasks_dir):
                        for f in files:
                            fp = os.path.join(root, f)
                            try:
                                sz = os.path.getsize(fp)
                                reclaimed_bytes += sz
                                pruned_files += 1
                                if not dry_run:
                                    os.remove(fp)
                            except OSError:
                                pass
                    if not dry_run:
                        try:
                            shutil.rmtree(tasks_dir, ignore_errors=True)
                            pruned_dirs += 1
                        except OSError:
                            pass
                    details.append(f"Pruned background tasks: {conv_id}/.system_generated/tasks")

        return PruneResult(
            reclaimed_bytes=reclaimed_bytes,
            pruned_files_count=pruned_files,
            pruned_directories_count=pruned_dirs,
            protected_active_id=active_id,
            dry_run=dry_run,
            details=details,
        )
