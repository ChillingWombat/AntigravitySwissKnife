"""
Unit tests for antigravity_swiss.process: lock_manager and lifecycle.
"""

import os
from pathlib import Path
import socket
import subprocess
import sys
import time
import pytest

from antigravity_swiss.process.lifecycle import ProcessLifecycleManager, ProcessManager
from antigravity_swiss.process.lock_manager import LockState, SingletonLockManager


def test_lock_manager_inspect_and_cleanup(temp_dir):
    """Verify SingletonLockManager target parsing, liveness check, and orphaned cleanup."""
    lock_mgr = SingletonLockManager(config_dir=temp_dir)

    # 1. No lock file initially
    state0 = lock_mgr.inspect_lock()
    assert state0.exists is False
    assert state0.is_orphaned is False

    # 2. Live PID lock symlink
    hostname = socket.gethostname()
    live_pid = os.getpid()
    target_str = f"{hostname}-{live_pid}"
    os.symlink(target_str, lock_mgr.lock_file)
    with open(lock_mgr.socket_file, "w") as f:
        f.write("")
    with open(lock_mgr.cookie_file, "w") as f:
        f.write("12345")

    state1 = lock_mgr.inspect_lock()
    assert state1.exists is True
    assert state1.is_symlink is True
    assert state1.hostname == hostname
    assert state1.pid == live_pid
    assert state1.is_pid_alive is True

    # 3. Simulate dead PID lock symlink
    lock_mgr.lock_file.unlink()
    dead_pid = 9999999
    dead_target = f"{hostname}-{dead_pid}"
    os.symlink(dead_target, lock_mgr.lock_file)

    state2 = lock_mgr.inspect_lock()
    assert state2.exists is True
    assert state2.pid == dead_pid
    assert state2.is_pid_alive is False
    assert state2.is_orphaned is True

    # 4. Clean up orphaned locks
    cleaned = lock_mgr.cleanup_orphaned_locks()
    assert cleaned is True
    assert not lock_mgr.lock_file.exists()
    assert not lock_mgr.socket_file.exists()
    assert not lock_mgr.cookie_file.exists()


def test_process_lifecycle_manager_graceful_termination(temp_dir):
    """Verify ProcessLifecycleManager terminates simulated process and runs cleanup."""
    config_dir = Path(temp_dir) / "config"
    data_dir = Path(temp_dir) / "data"
    config_dir.mkdir(parents=True)
    data_dir.mkdir(parents=True)

    # Spawn real background sleeping subprocess matching antigravity command line
    proc = subprocess.Popen([sys.executable, "-c", "# antigravity\nimport time; time.sleep(30)"])
    hostname = socket.gethostname()
    lock_file = config_dir / "SingletonLock"
    os.symlink(f"{hostname}-{proc.pid}", lock_file)

    pm = ProcessLifecycleManager(
        antigravity_bin=Path(sys.executable),
        lock_manager=SingletonLockManager(config_dir),
    )

    # Terminate gracefully
    success = pm.terminate_gracefully(timeout_sec=5.0)
    assert success is True
    assert proc.poll() is not None  # Process terminated
    assert not lock_file.exists()   # Lock cleaned up


def test_process_manager_interface_compliance():
    """Verify ProcessManager alias conforms to PROJECT.md interface contract."""
    pm = ProcessManager()
    assert hasattr(pm, "get_running_antigravity_pid")
    assert hasattr(pm, "terminate_gracefully")
    assert hasattr(pm, "relaunch")
