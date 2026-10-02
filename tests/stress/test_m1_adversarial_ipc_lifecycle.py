"""
Empirical Adversarial Stress Test Suite for M1 IPC and Process Lifecycle.
========================================================================
Verified Scenarios:
1. Large payload (>64KB: 100KB, 500KB, 2MB) roundtrip in AsyncDaemonClient without LimitOverrunError.
2. AsyncUnixSocketServer.broadcast_event with unreading/delinquent client: safe pruning & no hanging.
3. relaunch() broken symlink detection: latency < 1.0s vs 5.0s stall.
4. SingletonLockManager zombie process detection: /proc/{pid}/status State: Z marks orphaned & cleaned.
5. Non-UTF8 binary frame handling: returns JSON-RPC -32700 ParseError and connection remains open.
"""

from __future__ import annotations

import asyncio
import json
import os
import signal
import socket
import sys
import tempfile
import time
from pathlib import Path

import pytest

# Ensure testing flag is set
os.environ["ANTIGRAVITY_SWISS_TESTING"] = "1"

from antigravity_swiss.core.constants import MAX_FRAME_SIZE, SINGLETON_LOCK_NAME
from antigravity_swiss.core.errors import DaemonNotRunningError, IPCError
from antigravity_swiss.ipc.socket_client import AsyncDaemonClient
from antigravity_swiss.ipc.socket_server import AsyncUnixSocketServer
from antigravity_swiss.process.lifecycle import ProcessLifecycleManager
from antigravity_swiss.process.lock_manager import SingletonLockManager
from antigravity_swiss.session.sqlite_guard import SQLiteIntegrityGuard


def test_adversarial_large_payload_handling():
    """
    Challenge 1: Verify AsyncDaemonClient and AsyncUnixSocketServer handle
    payloads exceeding standard 64KB stream reader limits (100KB, 500KB, 2MB)
    without raising asyncio.LimitOverrunError.
    """
    async def _test():
        with tempfile.TemporaryDirectory() as tmp_dir:
            sock_path = Path(tmp_dir) / "test_large.sock"
            server = AsyncUnixSocketServer(sock_path)

            # Register echo method
            server.register("echo", lambda data: {"received_size": len(data), "data": data})
            await server.start()

            try:
                client = AsyncDaemonClient(sock_path)
                await client.connect()

                # Test payloads: 100KB, 500KB, 2MB
                sizes = [100 * 1024, 500 * 1024, 2 * 1024 * 1024]
                for size in sizes:
                    t0 = time.monotonic()
                    payload = "A" * size
                    res = await client.call("echo", {"data": payload})
                    dt = time.monotonic() - t0
                    assert res is not None, f"Expected response for size {size}"
                    assert res["received_size"] == size, f"Expected size {size}, got {res['received_size']}"
                    assert len(res["data"]) == size
                    assert dt < 5.0, f"Payload {size} bytes took too long: {dt:.2f}s"

                await client.close()
            finally:
                await server.stop()

    asyncio.run(_test())


def test_adversarial_broadcast_event_unreading_client():
    """
    Challenge 2: Verify that an unreading / stalled client does not hang event broadcasts,
    and delinquent clients are safely pruned concurrently from server._clients within 0.5s drain timeout.
    """
    async def _test():
        with tempfile.TemporaryDirectory() as tmp_dir:
            sock_path = Path(tmp_dir) / "test_broadcast.sock"
            server = AsyncUnixSocketServer(sock_path)
            await server.start()

            try:
                # 1. Normal client
                normal_client = AsyncDaemonClient(sock_path)
                received_events = []
                normal_client.on("notify.test", lambda params: received_events.append(params))
                await normal_client.connect()

                # 2. Delinquent raw client: connects, sets small buffer, never reads
                raw_sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
                raw_sock.setsockopt(socket.SOL_SOCKET, socket.SO_RCVBUF, 1024)
                raw_sock.connect(str(sock_path))
                raw_sock.setblocking(False)

                await asyncio.sleep(0.1)
                assert server.client_count == 2, f"Expected 2 clients connected, got {server.client_count}"

                # 3. Saturate socket buffers with large broadcast frames
                large_chunk = "X" * (64 * 1024)
                t_start = time.monotonic()
                for i in range(10):
                    sent_count = await server.broadcast_event("notify.test", {"index": i, "chunk": large_chunk})
                elapsed = time.monotonic() - t_start

                # Broadcaster should not freeze indefinitely: each iteration bounded by drain timeout (0.5s)
                assert elapsed < 10.0, f"Broadcast took too long: {elapsed}s"

                # 4. Delinquent client should have timed out and been pruned
                assert server.client_count <= 1, f"Expected delinquent client pruned, current count: {server.client_count}"

                # 5. Subsequent broadcast to normal client should be instantaneous
                t_sub = time.monotonic()
                sent_count = await server.broadcast_event("notify.test", {"index": 999, "chunk": "fast"})
                sub_elapsed = time.monotonic() - t_sub
                assert sub_elapsed < 0.2, f"Subsequent broadcast took {sub_elapsed}s; should be fast"

                await asyncio.sleep(0.2)
                assert len(received_events) >= 1, "Normal client should have received events"
                assert any(ev.get("index") == 999 for ev in received_events)

                await normal_client.close()
                raw_sock.close()
            finally:
                await server.stop()

    asyncio.run(_test())


def test_adversarial_relaunch_broken_symlink_speed():
    """
    Challenge 3: Verify relaunch() detects broken symlinks in < 1.0s
    using is_symlink() or os.path.lexists(), rather than waiting for 5.0s timeout.
    """
    with tempfile.TemporaryDirectory() as tmp_dir:
        tmp_path = Path(tmp_dir)
        config_dir = tmp_path / "config"
        config_dir.mkdir()
        lock_file = config_dir / SINGLETON_LOCK_NAME

        # Create mock antigravity executable
        mock_bin = tmp_path / "mock_antigravity.sh"
        mock_bin.write_text(
            f"""#!/bin/sh
ln -s "nonexistent-target-hostname-$$" "{lock_file}"
sleep 15
"""
        )
        mock_bin.chmod(0o755)

        lock_mgr = SingletonLockManager(config_dir=config_dir)
        sqlite_guard = SQLiteIntegrityGuard(config_dir=config_dir, gemini_dir=config_dir)
        lifecycle = ProcessLifecycleManager(
            antigravity_bin=mock_bin,
            lock_manager=lock_mgr,
            sqlite_guard=sqlite_guard,
        )

        t_start = time.perf_counter()
        pid = lifecycle.relaunch()
        elapsed_sec = time.perf_counter() - t_start

        # Clean up spawned test process safely
        try:
            os.kill(pid, signal.SIGKILL)
        except OSError:
            pass

        print(f"\n[Timing Benchmark] relaunch() with broken symlink took {elapsed_sec*1000:.2f} ms")
        assert elapsed_sec < 1.0, f"relaunch() took {elapsed_sec:.3f}s (must be < 1.0s)!"
        assert lock_file.is_symlink(), "Expected lock file to be a symlink"
        assert not lock_file.exists(), "Target file must not exist (broken symlink condition)"


def test_adversarial_zombie_process_detection():
    """
    Challenge 4: Verify that a zombie process (State: Z in /proc/{pid}/status)
    is recognized as dead (is_pid_alive=False, is_orphaned=True) and permits cleanup.
    """
    pid = os.fork()
    if pid == 0:
        os._exit(0)

    try:
        time.sleep(0.05)

        proc_status = Path(f"/proc/{pid}/status")
        assert proc_status.exists(), f"/proc/{pid}/status does not exist"
        status_text = proc_status.read_text(encoding="utf-8", errors="ignore")
        assert "State:\tZ" in status_text or "State: Z" in status_text, f"Expected State: Z, got: {status_text}"

        with tempfile.TemporaryDirectory() as tmp_dir:
            config_dir = Path(tmp_dir)
            lock_mgr = SingletonLockManager(config_dir=config_dir)

            lock_file = config_dir / SINGLETON_LOCK_NAME
            lock_file.symlink_to(f"testhost-{pid}")

            state = lock_mgr.inspect_lock()
            assert state.pid == pid
            assert state.is_symlink is True
            assert state.is_pid_alive is False, f"Zombie PID {pid} was incorrectly detected as alive!"
            assert state.is_orphaned is True, f"Zombie PID {pid} lock was not marked as orphaned!"

            cleaned = lock_mgr.cleanup_orphaned_locks()
            assert cleaned is True, "Failed to clean orphaned zombie lock"
            assert not lock_file.is_symlink() and not lock_file.exists(), "Lock file was not unlinked"
    finally:
        os.waitpid(pid, 0)


def test_adversarial_non_utf8_binary_frame_handling():
    """
    Challenge 5: Verify non-UTF-8 binary frames return JSON-RPC 2.0 -32700 ParseError
    without terminating or dropping the client connection, and subsequent valid requests succeed.
    """
    async def _test():
        with tempfile.TemporaryDirectory() as tmp_dir:
            sock_path = Path(tmp_dir) / "test_binary.sock"
            server = AsyncUnixSocketServer(sock_path)
            server.register("ping", lambda: "pong")
            await server.start()

            try:
                reader, writer = await asyncio.open_unix_connection(str(sock_path))

                # Send illegal non-UTF8 bytes followed by newline
                bad_bytes = b"\xff\xfe\x80\xaa\xbb\xcc\n"
                writer.write(bad_bytes)
                await writer.drain()

                # Read response
                resp_line = await reader.readline()
                assert resp_line, "Server closed connection on non-UTF8 bytes!"

                resp = json.loads(resp_line.decode("utf-8"))
                assert resp.get("jsonrpc") == "2.0"
                assert "error" in resp, f"Expected error response, got {resp}"
                assert resp["error"]["code"] == -32700, f"Expected -32700 ParseError, got {resp['error']['code']}"
                assert "Invalid UTF-8" in resp["error"]["message"]

                # Connection must still be open: send valid request
                valid_req = {"jsonrpc": "2.0", "method": "ping", "id": 123}
                writer.write((json.dumps(valid_req) + "\n").encode("utf-8"))
                await writer.drain()

                resp_line2 = await reader.readline()
                assert resp_line2, "Server closed connection after non-UTF8 recovery!"
                resp2 = json.loads(resp_line2.decode("utf-8"))
                assert resp2.get("result") == "pong"
                assert resp2.get("id") == 123

                writer.close()
                await writer.wait_closed()
            finally:
                await server.stop()

    asyncio.run(_test())


if __name__ == "__main__":
    pytest.main(["-v", "-s", __file__])
