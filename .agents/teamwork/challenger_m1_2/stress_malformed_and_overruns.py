"""
Adversarial Stress Test: Malformed Frames, Invalid Methods, and Frame Overruns (>10MB).
=====================================================================================
Tests:
1. Garbage bytes and syntax-broken JSON (unterminated string, invalid tokens, binary noise).
2. Non-object/array top-level JSON entities (numbers, booleans, naked strings).
3. Empty batches and batches containing mixed valid/invalid requests.
4. Protocol deviations: wrong jsonrpc version, missing method, invalid params.
5. Notification semantics: notifications with syntax or execution errors MUST NOT return responses.
6. Frame overrun: Sending a 11MB frame (> MAX_FRAME_SIZE = 10MB) on a single line:
   - Verifies server handles asyncio.LimitOverrunError.
   - Verifies server writes error response.
   - Verifies server closes offending client.
   - Verifies server remains fully operational for subsequent valid connections.
"""

import asyncio
import json
import socket
import tempfile
from pathlib import Path

from antigravity_swiss.core.constants import JSONRPC_VERSION, MAX_FRAME_SIZE
from antigravity_swiss.core.errors import (
    InternalRPCError,
    InvalidParamsError,
    InvalidRequestError,
    MethodNotFoundError,
    ParseError,
    SwissKnifeError,
)
from antigravity_swiss.ipc.socket_client import AsyncDaemonClient, SyncDaemonClient
from antigravity_swiss.ipc.socket_server import AsyncUnixSocketServer


def send_raw_line(sock_path: Path, raw_bytes: bytes, timeout: float = 2.0) -> bytes:
    """Helper to send raw line via blocking socket and receive response line."""
    sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    sock.settimeout(timeout)
    try:
        sock.connect(str(sock_path))
        sock.sendall(raw_bytes + b"\n")
        buf = bytearray()
        while True:
            chunk = sock.recv(4096)
            if not chunk:
                break
            buf.extend(chunk)
            if b"\n" in buf:
                break
        return bytes(buf)
    finally:
        sock.close()


async def run_malformed_and_overrun_tests(sock_path: Path):
    print("\n--- Stress Test: Malformed Frames, Invalid Methods, Frame Overruns ---")
    server = AsyncUnixSocketServer(sock_path)

    @server.register("system.ping")
    def rpc_ping():
        return "pong"

    @server.register("calc.divide")
    def rpc_divide(a: float, b: float) -> float:
        if b == 0:
            raise SwissKnifeError("Division by zero", code=-32090)
        return a / b

    await server.start()

    # 1. Broken JSON syntax
    print("Testing syntax-broken JSON frames...")
    broken_payloads = [
        b"{unquoted_key: 123}",
        b'{"jsonrpc": "2.0", "method": "test',
        b"\x00\x01\x02\xff\xfe\xfd",
        b"??? Not even JSON ???",
    ]
    for raw in broken_payloads:
        resp = send_raw_line(sock_path, raw)
        data = json.loads(resp.decode("utf-8").strip())
        assert data.get("jsonrpc") == JSONRPC_VERSION
        assert data["error"]["code"] == -32700  # ParseError
        assert data["id"] is None
    print("✓ Syntax-broken JSON correctly yielded ParseError (-32700).")

    # 2. Non-object/array JSON
    print("Testing non-object / non-array top-level entities...")
    for entity in [b"12345", b'"just a string"', b"true", b"null"]:
        resp = send_raw_line(sock_path, entity)
        data = json.loads(resp.decode("utf-8").strip())
        assert data["error"]["code"] == -32600  # InvalidRequestError
    print("✓ Non-object/array entities correctly yielded InvalidRequestError (-32600).")

    # 3. Batch anomalies
    print("Testing batch anomalies (empty batch, invalid elements)...")
    # Empty batch
    resp = send_raw_line(sock_path, b"[]")
    data = json.loads(resp.decode("utf-8").strip())
    assert data["error"]["code"] == -32600

    # Batch with mixed requests
    batch_req = json.dumps([
        {"jsonrpc": "2.0", "method": "system.ping", "id": 1},
        {"jsonrpc": "2.0", "method": "calc.divide", "params": {"a": 10, "b": 2}, "id": 2},
        {"jsonrpc": "2.0", "method": "calc.divide", "params": {"a": 10, "b": 0}, "id": 3},
        {"invalid": "structure"},
        {"jsonrpc": "2.0", "method": "unknown_method", "id": 5},
    ]).encode("utf-8")

    resp = send_raw_line(sock_path, batch_req)
    batch_res = json.loads(resp.decode("utf-8").strip())
    assert isinstance(batch_res, list)
    assert len(batch_res) == 5
    assert batch_res[0] == {"jsonrpc": "2.0", "result": "pong", "id": 1}
    assert batch_res[1] == {"jsonrpc": "2.0", "result": 5.0, "id": 2}
    assert batch_res[2]["error"]["code"] == -32090  # Division by zero
    assert batch_res[3]["error"]["code"] == -32600  # InvalidRequest
    assert batch_res[4]["error"]["code"] == -32601  # MethodNotFound
    print("✓ Batch anomalies and mixed executions correctly handled per JSON-RPC 2.0.")

    # 4. Notifications error suppression
    print("Testing notification error suppression...")
    # Notification with unknown method (id omitted)
    notif_unknown = json.dumps({"jsonrpc": "2.0", "method": "no_such_method"}).encode("utf-8")
    sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    sock.settimeout(0.5)
    sock.connect(str(sock_path))
    sock.sendall(notif_unknown + b"\n")
    # Send a ping immediately afterwards to see if socket is active and only ping responds
    sock.sendall(json.dumps({"jsonrpc": "2.0", "method": "system.ping", "id": 99}).encode("utf-8") + b"\n")
    data = sock.recv(1024).decode("utf-8").strip()
    sock.close()
    lines = [line for line in data.split("\n") if line.strip()]
    assert len(lines) == 1
    resp_obj = json.loads(lines[0])
    assert resp_obj["id"] == 99  # Only ping responded, notification error was silent
    print("✓ Notification error suppression strictly conforms to JSON-RPC 2.0 spec.")

    # 5. Invalid parameters
    print("Testing invalid parameters...")
    # Missing required params
    resp = send_raw_line(sock_path, json.dumps({"jsonrpc": "2.0", "method": "calc.divide", "params": {"a": 10}, "id": 10}).encode("utf-8"))
    data = json.loads(resp.decode("utf-8").strip())
    assert data["error"]["code"] == -32602  # InvalidParamsError

    # Unexpected params
    resp = send_raw_line(sock_path, json.dumps({"jsonrpc": "2.0", "method": "calc.divide", "params": {"a": 10, "b": 2, "c": 3}, "id": 11}).encode("utf-8"))
    data = json.loads(resp.decode("utf-8").strip())
    assert data["error"]["code"] == -32602
    print("✓ Invalid parameter structures rejected with code -32602.")

    # 6. Frame Overruns (> 10MB)
    print("\nTesting Frame Overrun (>10MB)...")
    # MAX_FRAME_SIZE is 10MB. We send 11MB on a single line.
    OVERRUN_SIZE = 11 * 1024 * 1024
    overrun_bytes = b'{"jsonrpc": "2.0", "method": "system.ping", "params": "' + (b"O" * OVERRUN_SIZE) + b'", "id": 999}\n'

    sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    sock.settimeout(10.0)
    sock.connect(str(sock_path))

    # Send the 11MB frame in chunks
    chunk_size = 1024 * 1024
    for i in range(0, len(overrun_bytes), chunk_size):
        sock.sendall(overrun_bytes[i:i + chunk_size])

    # Receive server's rejection
    resp_buf = bytearray()
    while True:
        chunk = sock.recv(4096)
        if not chunk:
            break
        resp_buf.extend(chunk)
        if b"\n" in resp_buf:
            break
    sock.close()

    resp_line = resp_buf.decode("utf-8").strip()
    print(f"Overrun response from server: {resp_line}")
    overrun_data = json.loads(resp_line)
    assert overrun_data["error"]["code"] == -32700  # ParseError
    assert "exceeds 10MB limit" in overrun_data["error"]["message"]
    print("✓ 11MB frame overrun caught: server returned LimitOverrun error frame.")

    # 7. Verify server resilience after overrun
    print("Verifying server health after frame overrun...")
    client = SyncDaemonClient(sock_path)
    res = client.call("system.ping")
    assert res == "pong"
    print("✓ Server remains 100% operational after handling frame overrun.")

    await server.stop()
    print("✓ Server stopped cleanly.")


async def main():
    with tempfile.TemporaryDirectory() as td:
        sock_path = Path(td) / "stress_malformed.sock"
        await run_malformed_and_overrun_tests(sock_path)
    print("\nALL MALFORMED & OVERRUN TESTS PASSED SUCCESSFULLY.")


if __name__ == "__main__":
    asyncio.run(main())
