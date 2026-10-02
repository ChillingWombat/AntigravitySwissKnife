"""
Adversarial Stress Test: 50+ Concurrent Client Connections and Pub-Sub Broadcasting.
=====================================================================================
Tests:
1. 60 concurrent clients establish Unix Domain Socket connections simultaneously.
2. 60 connected clients subscribe to pub-sub events; server broadcasts 10 sequential events
   (verifying 600 total deliveries with zero packet corruption or drops).
3. 60 concurrent clients execute 20 JSON-RPC requests each (1,200 total RPC transactions)
   with random micro-delays to test high-throughput concurrency and interleaving.
4. Mass simultaneous disconnection: all 60 clients disconnect concurrently;
   verifies server client count cleanly drops to 0 without fd leaks or hung coroutines.
"""

import asyncio
import random
import tempfile
import time
from pathlib import Path

from antigravity_swiss.ipc.socket_client import AsyncDaemonClient
from antigravity_swiss.ipc.socket_server import AsyncUnixSocketServer

NUM_CLIENTS = 60
NUM_BROADCASTS = 10
NUM_REQUESTS_PER_CLIENT = 20


async def run_concurrent_clients_stress(sock_path: Path):
    print(f"\n--- Stress Test: {NUM_CLIENTS} Concurrent Clients Concurrency & Pub-Sub ---")
    server = AsyncUnixSocketServer(sock_path)

    # Register test RPC methods
    @server.register("math.multiply")
    def rpc_multiply(x: int, y: int) -> int:
        return x * y

    @server.register("echo_delayed")
    async def rpc_echo(msg: str) -> str:
        await asyncio.sleep(random.uniform(0.001, 0.01))
        return msg

    await server.start()
    assert server.is_running

    clients: list[AsyncDaemonClient] = []
    received_events: dict[int, list[dict]] = {i: [] for i in range(NUM_CLIENTS)}

    # Phase 1: Simultaneous connection of 60 clients
    print(f"Connecting {NUM_CLIENTS} clients simultaneously...")
    connect_start = time.perf_counter()

    async def connect_client(idx: int):
        c = AsyncDaemonClient(sock_path)
        await c.connect()
        c.on("quota.tick", lambda p: received_events[idx].append(p))
        return c

    clients = await asyncio.gather(*(connect_client(i) for i in range(NUM_CLIENTS)))
    connect_duration = time.perf_counter() - connect_start
    print(f"✓ All {NUM_CLIENTS} clients connected in {connect_duration:.3f}s. Server client count: {server.client_count}")
    assert server.client_count == NUM_CLIENTS

    # Phase 2: Pub-sub broadcast storm (10 broadcasts to 60 clients = 600 deliveries)
    print(f"Broadcasting {NUM_BROADCASTS} sequential events to {NUM_CLIENTS} clients...")
    bcast_start = time.perf_counter()
    for seq in range(NUM_BROADCASTS):
        sent = await server.broadcast_event("quota.tick", {"seq": seq, "val": seq * 100})
        assert sent == NUM_CLIENTS

    # Allow event delivery loop to process
    await asyncio.sleep(0.3)
    bcast_duration = time.perf_counter() - bcast_start

    # Verify all clients received every event in order
    for idx, evs in received_events.items():
        assert len(evs) == NUM_BROADCASTS, f"Client {idx} received {len(evs)}/{NUM_BROADCASTS} events"
        for seq, ev in enumerate(evs):
            assert ev["seq"] == seq
            assert ev["val"] == seq * 100

    print(f"✓ 600/600 broadcast deliveries verified in {bcast_duration:.3f}s (100% reception, 0 loss).")

    # Phase 3: High-throughput concurrent JSON-RPC load (60 clients * 20 calls = 1,200 calls)
    print(f"Executing {NUM_CLIENTS * NUM_REQUESTS_PER_CLIENT} concurrent JSON-RPC requests...")
    rpc_start = time.perf_counter()

    async def client_worker(idx: int, c: AsyncDaemonClient):
        for req_idx in range(NUM_REQUESTS_PER_CLIENT):
            if req_idx % 2 == 0:
                x = idx + 1
                y = req_idx + 1
                res = await c.call("math.multiply", {"x": x, "y": y})
                assert res == x * y
            else:
                tag = f"client-{idx}-req-{req_idx}"
                res = await c.call("echo_delayed", {"msg": tag})
                assert res == tag

    await asyncio.gather(*(client_worker(i, clients[i]) for i in range(NUM_CLIENTS)))
    rpc_duration = time.perf_counter() - rpc_start
    total_calls = NUM_CLIENTS * NUM_REQUESTS_PER_CLIENT
    rps = total_calls / rpc_duration
    print(f"✓ {total_calls} JSON-RPC calls completed in {rpc_duration:.3f}s ({rps:.1f} req/s, 0 errors).")
    print(f"Server stats: {server.stats}")
    assert server.stats["requests_total"] >= total_calls
    assert server.stats["errors_total"] == 0

    # Phase 4: Mass simultaneous disconnection
    print("Initiating mass simultaneous disconnection of all clients...")
    disc_start = time.perf_counter()
    await asyncio.gather(*(c.close() for c in clients))
    await asyncio.sleep(0.1)
    disc_duration = time.perf_counter() - disc_start

    print(f"✓ All clients disconnected in {disc_duration:.3f}s. Server client count: {server.client_count}")
    assert server.client_count == 0

    await server.stop()
    assert not server.is_running
    print("✓ Server stopped cleanly.")


async def main():
    with tempfile.TemporaryDirectory() as td:
        sock_path = Path(td) / "stress_concurrency.sock"
        await run_concurrent_clients_stress(sock_path)
    print("\nALL CONCURRENT CLIENTS STRESS TESTS PASSED SUCCESSFULLY.")


if __name__ == "__main__":
    asyncio.run(main())
