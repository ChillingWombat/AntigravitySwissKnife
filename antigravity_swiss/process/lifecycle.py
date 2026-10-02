"""
Antigravity Process Lifecycle Manager.
======================================
Implements ProcessManager contract:
- get_running_antigravity_pid() -> int | None
- terminate_gracefully(timeout_sec: float = 10.0) -> bool
- relaunch(conversation_id: str | None = None) -> int
"""

from __future__ import annotations

import logging
import os
import signal
import subprocess
import time
from pathlib import Path
from typing import Any, Protocol, runtime_checkable

from antigravity_swiss.core.constants import (
    DEFAULT_ANTIGRAVITY_BIN,
    DEFAULT_ANTIGRAVITY_CONFIG_DIR,
    DEFAULT_PROCESS_TERMINATE_TIMEOUT_SECONDS,
)
from antigravity_swiss.core.errors import (
    ProcessLaunchError,
    ProcessTerminationTimeoutError,
)
from antigravity_swiss.process.lock_manager import SingletonLockManager
from antigravity_swiss.session.app_storage import AppStorageManager
from antigravity_swiss.session.sqlite_guard import SQLiteIntegrityGuard

logger = logging.getLogger(__name__)


@runtime_checkable
class ProcessManagerProtocol(Protocol):
    """Interface Contract defined in PROJECT.md § Interface Contracts."""
    def get_running_antigravity_pid(self) -> int | None: ...
    def terminate_gracefully(self, timeout_sec: float = 10.0) -> bool: ...
    def relaunch(self, conversation_id: str | None = None) -> int: ...


class ProcessLifecycleManager:
    """Production implementation of ProcessManager contract."""

    def __init__(
        self,
        config: Any = None,
        antigravity_bin: Path | str | None = None,
        lock_manager: SingletonLockManager | None = None,
        sqlite_guard: SQLiteIntegrityGuard | None = None,
        storage_manager: AppStorageManager | None = None,
    ) -> None:
        if config is not None:
            self.antigravity_bin = Path(antigravity_bin or config.antigravity_bin).expanduser().resolve()
            self.lock_manager = lock_manager or SingletonLockManager(config.antigravity_config_dir)
            self.sqlite_guard = sqlite_guard or SQLiteIntegrityGuard(
                config_dir=config.antigravity_config_dir,
                gemini_dir=config.antigravity_data_dir,
            )
            self.storage_manager = storage_manager or AppStorageManager(
                app_storage_path=config.antigravity_config_dir / "app_storage.json",
                storage_json_path=config.antigravity_config_dir / "User" / "globalStorage" / "storage.json",
                conv_summaries_db=config.antigravity_data_dir / "conversation_summaries.db",
            )
        else:
            self.antigravity_bin = Path(antigravity_bin or DEFAULT_ANTIGRAVITY_BIN).expanduser().resolve()
            self.lock_manager = lock_manager or SingletonLockManager()
            self.sqlite_guard = sqlite_guard or SQLiteIntegrityGuard()
            self.storage_manager = storage_manager or AppStorageManager()

    def get_running_antigravity_pid(self) -> int | None:
        """
        Determines the PID of the running root Antigravity process for THIS instance.
        Uses SingletonLock detection with verification, only falling back to /proc scanning
        if the process commandline explicitly references this instance's config_dir.
        NEVER matches external Antigravity processes belonging to other profiles or tests.
        """
        is_testing = bool(os.environ.get("PYTEST_CURRENT_TEST") or os.environ.get("ANTIGRAVITY_SWISS_TESTING"))
        real_default = Path(DEFAULT_ANTIGRAVITY_CONFIG_DIR).expanduser().resolve()

        # ABSOLUTE SAFETY SHIELD:
        # In testing mode, never inspect real host locks or return real host Antigravity PIDs!
        if is_testing and self.lock_manager.config_dir.resolve() == real_default:
            return None

        lock_state = self.lock_manager.inspect_lock()
        if lock_state.is_pid_alive and lock_state.is_antigravity:
            # Additional safety: Verify PID is not the host Antigravity IDE if testing
            if is_testing:
                try:
                    cmdline = Path(f"/proc/{lock_state.pid}/cmdline").read_bytes().decode("utf-8", errors="ignore")
                    if "/opt/Antigravity" in cmdline:
                        logger.warning("SAFETY SHIELD: Blocked access to host Antigravity PID %d in testing mode.", lock_state.pid)
                        return None
                except Exception:
                    pass
            return lock_state.pid

        # If running under pytest or testing mode, do not scan system /proc to avoid terminating user IDE
        if is_testing:
            return None

        # Fallback /proc scan ONLY for processes explicitly tied to this lock_manager's config directory
        config_dir_str = str(self.lock_manager.config_dir.resolve())
        proc_root = Path("/proc")
        if proc_root.exists():
            for entry in proc_root.iterdir():
                if not entry.name.isdigit():
                    continue
                try:
                    cmdline = (entry / "cmdline").read_bytes().decode("utf-8", errors="ignore")
                    # Must contain antigravity, be a root process (no --type=), AND match this exact user-data-dir
                    if "antigravity" in cmdline and "--type=" not in cmdline and config_dir_str in cmdline:
                        pid = int(entry.name)
                        return pid
                except (OSError, PermissionError):
                    continue

        return None

    def terminate_gracefully(self, timeout_sec: float = DEFAULT_PROCESS_TERMINATE_TIMEOUT_SECONDS) -> bool:
        """
        Terminates running Antigravity instance cleanly:
        1. Sends SIGTERM to main PID (triggers before-quit + killLanguageServer).
        2. Polls /proc/<PID> every 100ms up to timeout_sec.
        3. If unresponsive, escalates to SIGKILL on process.
        4. Cleans up orphaned singleton locks.
        5. Flushes and checkpoints all SQLite WAL databases.
        """
        is_testing = bool(os.environ.get("PYTEST_CURRENT_TEST") or os.environ.get("ANTIGRAVITY_SWISS_TESTING"))
        real_default = Path(DEFAULT_ANTIGRAVITY_CONFIG_DIR).expanduser().resolve()

        pid = self.get_running_antigravity_pid()
        if pid is None:
            logger.info("Antigravity is not currently running.")
            # NEVER clean locks or flush host DBs if in testing pointing to host config
            if not (is_testing and self.lock_manager.config_dir.resolve() == real_default):
                self.lock_manager.cleanup_orphaned_locks()
                self.sqlite_guard.flush_and_verify_all(timeout_sec=5.0)
            return True

        # Safety check: Never kill host IDE
        try:
            cmdline = Path(f"/proc/{pid}/cmdline").read_bytes().decode("utf-8", errors="ignore")
            if "/opt/Antigravity" in cmdline and is_testing:
                logger.critical("SAFETY SHIELD: Refused to send SIGTERM/SIGKILL to host Antigravity PID %d!", pid)
                return True
        except Exception:
            pass

        logger.info("Sending SIGTERM to Antigravity (PID %d)...", pid)
        try:
            os.kill(pid, signal.SIGTERM)
        except ProcessLookupError:
            pass

        # Poll for graceful termination
        poll_interval = 0.1
        deadline = time.monotonic() + timeout_sec
        clean_exit = False

        while time.monotonic() < deadline:
            proc_path = Path(f"/proc/{pid}")
            if not proc_path.exists():
                clean_exit = True
                break
            status_path = proc_path / "status"
            if status_path.exists():
                try:
                    for line in status_path.read_text(encoding="utf-8", errors="ignore").splitlines():
                        if line.startswith("State:") and "Z" in line:
                            clean_exit = True
                            break
                except OSError:
                    pass
            if clean_exit:
                break
            time.sleep(poll_interval)

        # Fallback escalation to SIGKILL
        if not clean_exit:
            logger.warning(
                "PID %d did not terminate within %.1fs. Escalating to SIGKILL...",
                pid,
                timeout_sec,
            )
            try:
                os.kill(pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            time.sleep(0.5)

        # Remove orphaned lock files if left behind
        self.lock_manager.cleanup_orphaned_locks()

        # Flush and checkpoint SQLite databases
        self.sqlite_guard.flush_and_verify_all(timeout_sec=5.0)

        return True

    def relaunch(
        self,
        conversation_id: str | None = None,
        extra_args: list[str] | None = None,
    ) -> int:
        """
        Safely relaunches Antigravity:
        1. Ensures existing processes are cleanly terminated.
        2. Preserves active conversation layout and timestamp if provided.
        3. Spawns detached process with start_new_session=True.
        4. Verifies startup and returns new PID.
        """
        # Ensure clean state
        self.terminate_gracefully(timeout_sec=DEFAULT_PROCESS_TERMINATE_TIMEOUT_SECONDS)

        # Apply conversation session preservation if target given
        if conversation_id:
            self.storage_manager.preserve_active_conversation(conversation_id)

        if not self.antigravity_bin.exists():
            raise ProcessLaunchError(f"Antigravity binary not found at {self.antigravity_bin}")

        # Assemble launch arguments
        args = [str(self.antigravity_bin)]
        if extra_args:
            args.extend(extra_args)

        env = os.environ.copy()

        logger.info("Launching detached Antigravity instance: %s", args)
        try:
            proc = subprocess.Popen(
                args,
                start_new_session=True,  # Detached process group
                stdin=subprocess.DEVNULL,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                close_fds=True,
                env=env,
            )
        except OSError as exc:
            raise ProcessLaunchError(f"Failed to spawn Antigravity binary: {exc}") from exc

        new_pid = proc.pid
        logger.info("Antigravity launched with PID %d", new_pid)

        # Startup verification (poll up to 5s)
        deadline = time.monotonic() + 5.0
        while time.monotonic() < deadline:
            if proc.poll() is not None:
                raise ProcessLaunchError(
                    f"Antigravity exited prematurely with return code {proc.returncode}"
                )
            if self.lock_manager.lock_file.is_symlink() or os.path.lexists(self.lock_manager.lock_file):
                break
            time.sleep(0.2)

        return new_pid


# Direct alias conforming to PROJECT.md interface contract
ProcessManager = ProcessLifecycleManager
