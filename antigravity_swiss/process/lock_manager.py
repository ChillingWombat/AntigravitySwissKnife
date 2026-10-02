"""
SingletonLock, SingletonSocket, and SingletonCookie Detector and Sanitizer.
===========================================================================
Parses Electron single instance lock symlinks (<hostname>-<PID>), validates
liveness in /proc, and sanitizes orphaned locks before process relaunch.
"""

from __future__ import annotations

import logging
import os
from dataclasses import dataclass
from pathlib import Path

from antigravity_swiss.core.constants import (
    DEFAULT_ANTIGRAVITY_CONFIG_DIR,
    SINGLETON_COOKIE_NAME,
    SINGLETON_LOCK_NAME,
    SINGLETON_SOCKET_NAME,
)

logger = logging.getLogger(__name__)

DEFAULT_CONFIG_DIR = DEFAULT_ANTIGRAVITY_CONFIG_DIR


@dataclass
class LockState:
    lock_path: Path
    exists: bool
    is_symlink: bool
    target_str: str | None
    hostname: str | None
    pid: int | None
    is_pid_alive: bool
    is_antigravity: bool
    is_orphaned: bool


class SingletonLockManager:
    """Inspects and cleans up Chromium/Electron singleton locks."""

    def __init__(self, config_dir: Path | str = DEFAULT_CONFIG_DIR) -> None:
        self.config_dir = Path(config_dir).expanduser().resolve()
        self.lock_file = self.config_dir / SINGLETON_LOCK_NAME
        self.socket_file = self.config_dir / SINGLETON_SOCKET_NAME
        self.cookie_file = self.config_dir / SINGLETON_COOKIE_NAME

    def inspect_lock(self) -> LockState:
        """Inspects ~/.config/Antigravity/SingletonLock and validates process liveness."""
        if not self.lock_file.exists() and not self.lock_file.is_symlink():
            return LockState(
                lock_path=self.lock_file,
                exists=False,
                is_symlink=False,
                target_str=None,
                hostname=None,
                pid=None,
                is_pid_alive=False,
                is_antigravity=False,
                is_orphaned=False,
            )

        is_symlink = self.lock_file.is_symlink()
        target_str = None
        hostname = None
        pid = None
        is_alive = False
        is_antigravity = False

        if is_symlink:
            try:
                target_str = os.readlink(self.lock_file)
                # Format: <hostname>-<PID> (hostnames can contain dashes, so rsplit)
                parts = target_str.rsplit("-", 1)
                if len(parts) == 2 and parts[1].isdigit():
                    hostname = parts[0]
                    pid = int(parts[1])
            except OSError as err:
                logger.warning("Could not readlink %s: %s", self.lock_file, err)

        if pid is not None:
            proc_path = Path(f"/proc/{pid}")
            if proc_path.exists():
                is_alive = True
                status_path = proc_path / "status"
                if status_path.exists():
                    try:
                        for line in status_path.read_text(encoding="utf-8", errors="ignore").splitlines():
                            if line.startswith("State:"):
                                if "Z" in line:
                                    is_alive = False
                                break
                    except OSError:
                        pass

                if is_alive:
                    try:
                        cmdline = (proc_path / "cmdline").read_bytes().decode("utf-8", errors="ignore")
                        if "antigravity" in cmdline:
                            is_antigravity = True
                    except Exception:
                        pass

        is_orphaned = not (is_alive and is_antigravity)

        return LockState(
            lock_path=self.lock_file,
            exists=True,
            is_symlink=is_symlink,
            target_str=target_str,
            hostname=hostname,
            pid=pid,
            is_pid_alive=is_alive,
            is_antigravity=is_antigravity,
            is_orphaned=is_orphaned,
        )

    def cleanup_orphaned_locks(self) -> bool:
        """
        Unlinks SingletonLock, SingletonSocket, and SingletonCookie
        ONLY if the lock is confirmed orphaned.
        """
        is_testing = bool(os.environ.get("PYTEST_CURRENT_TEST") or os.environ.get("ANTIGRAVITY_SWISS_TESTING"))
        real_default = Path(DEFAULT_CONFIG_DIR).expanduser().resolve()
        if is_testing and self.config_dir.resolve() == real_default:
            logger.info("SAFETY SHIELD: Skipping lock cleanup on real host directory during test mode.")
            return False

        state = self.inspect_lock()
        if not state.exists and not self.lock_file.is_symlink():
            return False

        if not state.is_orphaned and state.is_pid_alive and state.is_antigravity:
            logger.warning(
                "Cannot cleanup locks: PID %s is actively running Antigravity.",
                state.pid,
            )
            return False

        cleaned = False
        for lock_item in [self.lock_file, self.socket_file, self.cookie_file]:
            if lock_item.exists() or lock_item.is_symlink():
                try:
                    lock_item.unlink(missing_ok=True)
                    logger.info("Unlinked orphaned lock file: %s", lock_item.name)
                    cleaned = True
                except OSError as err:
                    logger.error("Failed to unlink %s: %s", lock_item, err)

        return cleaned
