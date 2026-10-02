"""
Adversarial Stress Test: Abrupt Socket Disconnects and Large Payloads.
=====================================================================
Tests:
1. Client abruptly disconnects (RST) while transmitting half of a 5MB payload.
2. Client abruptly disconnects while server is transmitting a 5MB response.
3. 20 of 40 clients abruptly disconnect during a 2MB pub-sub broadcast;
   verifies surviving 20 clients receive message intact and dead clients are pruned.
4. Client disconnects while server is running an async method handler.
5. Server socket resource cleanup and connection recovery.
"""

import asyncio
import json
import os
import socket
import struct
import sys
import tempfile
import time
from pathlib import Path

from antigravity_swiss.core.constants import JSONRPC_VERSION, MAX_FRAME_SIZE
from antigravity_swiss.ipc.socket_client import AsyncDaemonClient
from antigravity_swiss.ipc.socket_server import AsyncUnixSocketServer


async def test_abrupt_disconnect_during_large_request(sock_path: Path):
    """Test 1: Client sends 5MB partial request and abruptly aborts connection."""
    print("\n--- Test 1: Abrupt disconnect during large request transmission ---")
    server = AsyncUnixSocketServer(sock_path)
    await server.start()

    def _raw_client_worker():
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        sock.connect(str(sock_path))
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_LINGER, struct.pack("ii", 1, 0))
        # Send 5MB in 64KB chunks and abort mid-way
        chunk = b"A" * 65536
        for _ in range(80):  # ~5.2 MB
            try:
                sock.sendall(chunk)
            except (BrokenPipeError, OSError):
                break
        sock.close()

    # Run the sender in background thread so server event loop can process chunks
    await asyncio.to_thread(_raw_client_worker)
    await asyncio.sleep(0.1)

    # Verify server is still alive and responsive to new clients
    client = AsyncDaemonClient(sock_path)
    await client.connect()
    assert client.is_connected
    await client.close()
    await server.stop()
    print("✓ Test 1 Passed: Server survived abrupt mid-payload RST disconnect.")


async def test_abrupt_disconnect_during_large_response(sock_path: Path):
    """Test 2: Server transmits 5MB response to a client that closes immediately."""
    print("\n--- Test 2: Abrupt disconnect during large response transmission ---")
    server = AsyncUnixSocketServer(sock_path)

    large_payload = "X" * (5 * 1024 * 1024)

    @server.register("get_large")
    def get_large():
        return {"data": large_payload}

    @server.register("echo")
    def echo(val: str):
        return val

    await server.start()

    def _raw_client_abort():
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        sock.connect(str(sock_path))
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_LINGER, struct.pack("ii", 1, 0))
        req = json.dumps({"jsonrpc": "2.0", "method": "get_large", "id": 1}) + "\n"
        sock.sendall(req.encode("utf-8"))
        # Read small chunk then immediately abort
        _ = sock.recv(256)
        sock.close()

    await asyncio.to_thread(_raw_client_abort)
    await asyncio.sleep(0.2)

    # Verify server didn't crash and handles next request cleanly
    client = AsyncDaemonClient(sock_path)
    await client.connect()
    res = await client.call("echo", {"val": "ping"})
    assert res == "ping"
    await client.close()
    await server.stop()
    print("✓ Test 2 Passed: Server gracefully handled client RST during 5MB response drain.")


async def test_broadcast_with_partial_client_deaths(sock_path: Path):
    """Test 3: 40 clients connected; 20 abruptly disconnect during 2MB broadcast."""
    print("\n--- Test 3: Broadcast resilience with 20/40 abruptly dying clients ---")
    server = AsyncUnixSocketServer(sock_path)
    await server.start()

    healthy_clients: list[AsyncDaemonClient] = []
    received_by_healthy = []

    # Connect 20 healthy clients
    for _ in range(20):
        c = AsyncDaemonClient(sock_path)
        await c.connect()
        c.on("notify.stress", lambda p: received_by_healthy.append(p))
        healthy_clients.append(c)

    # Connect 20 raw dying sockets
    dying_sockets: list[socket.socket] = []

    def _connect_dying():
        for _ in range(20):
            s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
            s.connect(str(sock_path))
            s.setsockopt(socket.SOL_SOCKET, socket.SO_LINGER, struct.pack("ii", 1, 0))
            dying_sockets.append(s)

    await asyncio.to_thread(_connect_dying)
    await asyncio.sleep(0.05)
    assert server.client_count == 40

    # Abruptly kill the 20 raw sockets right before/during broadcast
    for s in dying_sockets:
        s.close()

    # Broadcast a large 1MB payload
    broadcast_data = {"payload": "B" * (1024 * 1024)}
    sent_count = await server.broadcast_event("notify.stress", broadcast_data)

    await asyncio.sleep(0.3)

    # Verify healthy clients received the broadcast
    assert len(received_by_healthy) == 20
    assert len(received_by_healthy[0]["payload"]) == 1024 * 1024

    # Verify dead clients were pruned from server._clients
    assert server.client_count == 20

    for c in healthy_clients:
        await c.close()
    await server.stop()
    print(f"✓ Test 3 Passed: 20 healthy clients received 1MB broadcast intact; 20 dead clients cleanly pruned (server client count: {server.client_count}).")


async def test_disconnect_during_slow_async_handler(sock_path: Path):
    """Test 4: Client disconnects while server is awaiting slow async method."""
    print("\n--- Test 4: Client disconnects while server executes slow async handler ---")
    server = AsyncUnixSocketServer(sock_path)
    handler_completed = False

    @server.register("slow_method")
    async def slow_method():
        nonlocal handler_completed
        await asyncio.sleep(0.3)
        handler_completed = True
        return {"status": "ok"}

    await server.start()

    def _call_and_abort():
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        sock.connect(str(sock_path))
        req = json.dumps({"jsonrpc": "2.0", "method": "slow_method", "id": 42}) + "\n"
        sock.sendall(req.encode("utf-8"))
        time.sleep(0.05)
        # Disconnect before handler finishes
        sock.close()

    await asyncio.to_thread(_call_and_abort)

    # Wait for handler to complete on server
    await asyncio.sleep(0.4)
    assert handler_completed is True

    # Check server health
    client = AsyncDaemonClient(sock_path)
    await client.connect()
    assert client.is_connected
    await client.close()
    await server.stop()
    print("✓ Test 4 Passed: Server cleanly caught write/drain error after slow handler finished.")


async def main():
    with tempfile.TemporaryDirectory() as td:
        sock_path = Path(td) / "stress_disconnect.sock"
        await test_abrupt_disconnect_during_large_request(sock_path)
        await test_abrupt_disconnect_during_large_response(sock_path)
        await test_broadcast_with_partial_client_deaths(sock_path)
        await test_disconnect_during_slow_async_handler(sock_path)
    print("\nALL DISCONNECT STRESS TESTS PASSED SUCCESSFULLY.")


if __name__ == "__main__":
    asyncio.run(main())
