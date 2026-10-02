"""
Mock process manager simulating Electron Antigravity lifecycle,
SingletonLock symlinks, SIGTERM exit polling, and clean relaunching.
"""

import os
import signal
import socket
import subprocess
import sys
import time
from typing import Optional


class MockProcessManager:
    """Manages simulated Antigravity Electron process lifecycles and SingletonLock symlinks."""

    def __init__(self, config_dir: str):
        self.config_dir = config_dir
        self.dummy_proc: Optional[subprocess.Popen] = None
        self.simulated_pid: Optional[int] = None
        self.hostname = socket.gethostname()

    def get_singleton_lock_path(self) -> str:
        return os.path.join(self.config_dir, "SingletonLock")

    def get_singleton_socket_path(self) -> str:
        return os.path.join(self.config_dir, "SingletonSocket")

    def get_singleton_cookie_path(self) -> str:
        return os.path.join(self.config_dir, "SingletonCookie")

    def spawn_running_instance(self) -> int:
        """Spawns a real sleeping subprocess to act as the running Antigravity PID."""
        cmd = [sys.executable, "-c", "import time, signal; signal.signal(signal.SIGTERM, lambda s, f: exit(0)); time.sleep(300)"]
        self.dummy_proc = subprocess.Popen(cmd)
        self.simulated_pid = self.dummy_proc.pid

        # Create SingletonLock symlink to <hostname>-<PID>
        lock_path = self.get_singleton_lock_path()
        target = f"{self.hostname}-{self.simulated_pid}"
        if os.path.islink(lock_path) or os.path.exists(lock_path):
            os.unlink(lock_path)
        os.symlink(target, lock_path)

        # Create dummy socket & cookie
        sock_path = self.get_singleton_socket_path()
        if os.path.islink(sock_path) or os.path.exists(sock_path):
            os.unlink(sock_path)
        os.symlink("/tmp/mock_socket", sock_path)

        with open(self.get_singleton_cookie_path(), "w") as f:
            f.write("123456789\n")

        return self.simulated_pid

    def simulate_stale_lock(self, dead_pid: int = 999999) -> str:
        """Creates a SingletonLock pointing to a non-existent PID."""
        lock_path = self.get_singleton_lock_path()
        target = f"{self.hostname}-{dead_pid}"
        if os.path.islink(lock_path) or os.path.exists(lock_path):
            os.unlink(lock_path)
        os.symlink(target, lock_path)
        return lock_path

    def is_process_alive(self, pid: Optional[int] = None) -> bool:
        check_pid = pid or self.simulated_pid
        if not check_pid:
            return False
        try:
            os.kill(check_pid, 0)
            return True
        except OSError:
            return False

    def terminate_simulated(self, timeout_sec: float = 5.0) -> bool:
        """Sends SIGTERM to the simulated process and waits for it to exit."""
        if self.dummy_proc and self.dummy_proc.poll() is None:
            self.dummy_proc.send_signal(signal.SIGTERM)
            try:
                self.dummy_proc.wait(timeout=timeout_sec)
            except subprocess.TimeoutExpired:
                self.dummy_proc.kill()
                self.dummy_proc.wait()

        # Clean up symlink as Electron before-quit handler does
        lock_path = self.get_singleton_lock_path()
        if os.path.islink(lock_path):
            os.unlink(lock_path)

        sock_path = self.get_singleton_socket_path()
        if os.path.islink(sock_path):
            os.unlink(sock_path)

        self.simulated_pid = None
        return True

    def cleanup(self) -> None:
        if self.dummy_proc and self.dummy_proc.poll() is None:
            try:
                self.dummy_proc.kill()
                self.dummy_proc.wait()
            except Exception:
                pass
        for p in [self.get_singleton_lock_path(), self.get_singleton_socket_path(), self.get_singleton_cookie_path()]:
            if os.path.islink(p) or os.path.exists(p):
                try:
                    os.unlink(p)
                except Exception:
                    pass
