"""
Adversarial Stress Test: SingletonLock, Hyphenated Hostnames, and Process Kill Timeouts.
========================================================================================
Tests:
1. Hyphenated hostnames with multiple dashes (e.g. 'compute-node-prod-01-west-987654').
2. Pathologically malformed lock symlink targets (no hyphen, empty string, letters in PID, etc.).
3. Stale SingletonLock pointing to dead PID -> verified orphaned and unlinked with cookie/socket.
4. Stale SingletonLock pointing to live PID of a DIFFERENT non-antigravity process -> verified orphaned.
5. Live SingletonLock pointing to active Antigravity process -> verified protected from cleanup.
6. Process kill timeout & escalation: Stubborn process ignoring SIGTERM is terminated by SIGKILL escalation.
"""

import os
import signal
import socket
import subprocess
import sys
import tempfile
import time
from pathlib import Path

from antigravity_swiss.process.lifecycle import ProcessLifecycleManager
from antigravity_swiss.process.lock_manager import SingletonLockManager


def test_hyphenated_hostnames(config_dir: Path):
    print("\n--- Test: Hyphenated Hostnames in SingletonLock ---")
    lock_mgr = SingletonLockManager(config_dir=config_dir)

    test_cases = [
        ("my-workstation-01", 12345, "my-workstation-01-12345"),
        ("us-west-2-prod-node-09", 98765, "us-west-2-prod-node-09-98765"),
        ("a-b-c-d-e-f-g", 42, "a-b-c-d-e-f-g-42"),
        ("hostname_with_underscores-and-dashes", 1001, "hostname_with_underscores-and-dashes-1001"),
    ]

    for expected_host, expected_pid, target_str in test_cases:
        lock_mgr.lock_file.unlink(missing_ok=True)
        os.symlink(target_str, lock_mgr.lock_file)
        state = lock_mgr.inspect_lock()
        assert state.exists is True
        assert state.is_symlink is True
        assert state.hostname == expected_host, f"Expected {expected_host}, got {state.hostname}"
        assert state.pid == expected_pid, f"Expected {expected_pid}, got {state.pid}"

    print(f"✓ All {len(test_cases)} hyphenated hostnames parsed accurately.")


def test_pathological_lock_symlinks(config_dir: Path):
    print("\n--- Test: Pathologically Malformed Lock Symlinks ---")
    lock_mgr = SingletonLockManager(config_dir=config_dir)

    pathological_targets = [
        "nohyphens",
        "only-letters-not-a-pid",
        "",
        "trailing-hyphen-",
        "-leading-hyphen-123",
        "spaces in target-456",
        "unicode-❄️-999",
        "multiple--dashes--888",
    ]

    for target in pathological_targets:
        lock_mgr.lock_file.unlink(missing_ok=True)
        try:
            os.symlink(target, lock_mgr.lock_file)
        except OSError:
            continue
        state = lock_mgr.inspect_lock()
        assert state.exists is True
        assert state.is_symlink is True
        # Must be classified as orphaned since it cannot point to a verified live Antigravity PID
        assert state.is_orphaned is True
        # Cleanup must safely remove it without throwing
        cleaned = lock_mgr.cleanup_orphaned_locks()
        assert cleaned is True
        assert not lock_mgr.lock_file.exists()

    print(f"✓ All {len(pathological_targets)} pathological targets safely classified as orphaned and unlinked.")


def test_dead_pid_lock_cleanup(config_dir: Path):
    print("\n--- Test: Stale Lock Pointing to Dead PID ---")
    lock_mgr = SingletonLockManager(config_dir=config_dir)

    hostname = socket.gethostname()
    dead_pid = 4194300  # Kernel max PID on 64-bit Linux is 4194304
    os.symlink(f"{hostname}-{dead_pid}", lock_mgr.lock_file)

    # Create dummy socket and cookie files
    lock_mgr.socket_file.write_text("dummy socket")
    lock_mgr.cookie_file.write_text("dummy cookie")

    state = lock_mgr.inspect_lock()
    assert state.exists is True
    assert state.is_pid_alive is False
    assert state.is_orphaned is True

    cleaned = lock_mgr.cleanup_orphaned_locks()
    assert cleaned is True
    assert not lock_mgr.lock_file.exists()
    assert not lock_mgr.socket_file.exists()
    assert not lock_mgr.cookie_file.exists()
    print("✓ Dead PID lock and auxiliary files (socket, cookie) cleaned up.")


def test_live_non_antigravity_pid(config_dir: Path):
    print("\n--- Test: Lock Pointing to Live Process that is NOT Antigravity ---")
    lock_mgr = SingletonLockManager(config_dir=config_dir)

    # Spawn an unrelated background process (e.g. sleep 30)
    proc = subprocess.Popen(["sleep", "30"])
    try:
        hostname = socket.gethostname()
        os.symlink(f"{hostname}-{proc.pid}", lock_mgr.lock_file)

        state = lock_mgr.inspect_lock()
        assert state.exists is True
        assert state.is_pid_alive is True
        assert state.is_antigravity is False
        assert state.is_orphaned is True, "Live non-antigravity process must be considered orphaned"

        cleaned = lock_mgr.cleanup_orphaned_locks()
        assert cleaned is True
        assert not lock_mgr.lock_file.exists()
        print("✓ Stale lock pointing to non-antigravity process correctly marked orphaned and cleaned.")
    finally:
        proc.kill()
        proc.wait()


def test_live_antigravity_pid_protection(config_dir: Path):
    print("\n--- Test: Protection of Live Antigravity Process Lock ---")
    lock_mgr = SingletonLockManager(config_dir=config_dir)

    # Spawn process whose cmdline contains "antigravity"
    proc = subprocess.Popen([sys.executable, "-c", "# antigravity\nimport time; time.sleep(30)"])
    try:
        hostname = socket.gethostname()
        os.symlink(f"{hostname}-{proc.pid}", lock_mgr.lock_file)

        state = lock_mgr.inspect_lock()
        assert state.exists is True
        assert state.is_pid_alive is True
        assert state.is_antigravity is True
        assert state.is_orphaned is False

        # Attempted cleanup MUST refuse
        cleaned = lock_mgr.cleanup_orphaned_locks()
        assert cleaned is False
        assert lock_mgr.lock_file.exists()
        print("✓ Live Antigravity process lock protected from deletion.")
    finally:
        proc.kill()
        proc.wait()
        lock_mgr.cleanup_orphaned_locks()


def test_stubborn_process_sigkill_escalation(config_dir: Path):
    print("\n--- Test: Stubborn Process Kill Timeout and SIGKILL Escalation ---")
    # Spawn a python process that explicitly ignores SIGTERM
    stubborn_code = (
        "import signal, time\n"
        "signal.signal(signal.SIGTERM, signal.SIG_IGN)\n"
        "while True:\n"
        "    time.sleep(1)\n"
    )
    proc = subprocess.Popen([sys.executable, "-c", f"# antigravity\n{stubborn_code}"])
    hostname = socket.gethostname()
    lock_file = config_dir / "SingletonLock"
    os.symlink(f"{hostname}-{proc.pid}", lock_file)

    lock_mgr = SingletonLockManager(config_dir)
    lifecycle_mgr = ProcessLifecycleManager(
        antigravity_bin=Path(sys.executable),
        lock_manager=lock_mgr,
    )

    t0 = time.monotonic()
    # Request termination with a short 1.0s timeout
    print("Terminating stubborn process (ignoring SIGTERM) with 1.0s grace period...")
    terminated = lifecycle_mgr.terminate_gracefully(timeout_sec=1.0)
    duration = time.monotonic() - t0

    assert terminated is True
    # Verify process was killed (by SIGKILL escalation)
    assert proc.poll() is not None
    # Verify returncode indicates killed by signal (SIGKILL is 9, returncode is -9)
    assert proc.returncode == -signal.SIGKILL
    # Verify duration was between 1.0s and 2.5s (due to 1.0s timeout + 0.5s wait)
    assert 1.0 <= duration <= 3.0, f"Expected 1.0-3.0s, took {duration:.2f}s"
    # Verify lock file cleaned up
    assert not lock_file.exists()

    print(f"✓ Stubborn process successfully terminated via SIGKILL escalation in {duration:.2f}s (returncode: {proc.returncode}).")


def main():
    with tempfile.TemporaryDirectory() as td:
        config_dir = Path(td) / "config"
        config_dir.mkdir(parents=True)
        test_hyphenated_hostnames(config_dir)
        test_pathological_lock_symlinks(config_dir)
        test_dead_pid_lock_cleanup(config_dir)
        test_live_non_antigravity_pid(config_dir)
        test_live_antigravity_pid_protection(config_dir)
        test_stubborn_process_sigkill_escalation(config_dir)
    print("\nALL PROCESS & LOCK STRESS TESTS PASSED SUCCESSFULLY.")


if __name__ == "__main__":
    main()
