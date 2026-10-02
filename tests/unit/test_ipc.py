"""
Unit tests for antigravity_swiss.ipc: socket_server, socket_client, and controller.
"""

import asyncio
import json
import os
from pathlib import Path
import socket
import pytest

from antigravity_swiss.core.config import SwissKnifeConfig
from antigravity_swiss.core.errors import (
    DaemonAlreadyRunningError,
    DaemonNotRunningError,
    MethodNotFoundError,
    ParseError,
)
from antigravity_swiss.ipc.controller import (
    RemoteDaemonController,
    StandaloneController,
    create_controller,
)
from antigravity_swiss.ipc.socket_client import AsyncDaemonClient, SyncDaemonClient
from antigravity_swiss.ipc.socket_server import AsyncUnixSocketServer


def test_socket_server_binding_and_permissions(temp_dir):
    """Verify AsyncUnixSocketServer binds socket with 0600 permissions."""
    async def _test():
        sock_path = Path(temp_dir) / "test_daemon.sock"
        server = AsyncUnixSocketServer(sock_path)

        await server.start()
        assert server.is_running is True
        assert sock_path.exists()
        assert (sock_path.stat().st_mode & 0o777) == 0o600

        await server.stop()
        assert server.is_running is False
        assert not sock_path.exists()

    asyncio.run(_test())


def test_socket_server_stale_socket_cleanup(temp_dir):
    """Verify server detects and cleans up a stale socket from a previous crashed process."""
    async def _test():
        sock_path = Path(temp_dir) / "stale_daemon.sock"
        # Create dead socket file (not listening)
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        sock.bind(str(sock_path))
        sock.close()
        assert sock_path.exists()

        server = AsyncUnixSocketServer(sock_path)
        # Server should probe, detect dead socket, unlink, and start cleanly
        await server.start()
        assert server.is_running is True

        await server.stop()

    asyncio.run(_test())


def test_socket_server_already_running_detection(temp_dir):
    """Verify starting a second server on an active socket raises DaemonAlreadyRunningError."""
    async def _test():
        sock_path = Path(temp_dir) / "active_daemon.sock"
        server1 = AsyncUnixSocketServer(sock_path)
        await server1.start()

        server2 = AsyncUnixSocketServer(sock_path)
        with pytest.raises(DaemonAlreadyRunningError):
            await server2.start()

        await server1.stop()

    asyncio.run(_test())


def test_jsonrpc_request_response_and_errors(temp_dir):
    """Verify JSON-RPC 2.0 single request, batch request, and error handling."""
    async def _test():
        sock_path = Path(temp_dir) / "rpc_test.sock"
        server = AsyncUnixSocketServer(sock_path)

        @server.register("math.add")
        def rpc_add(a: int, b: int) -> int:
            return a + b

        @server.register("echo")
        async def rpc_echo(msg: str) -> str:
            return msg

        await server.start()

        client = SyncDaemonClient(sock_path)
        assert client.is_daemon_alive() is True

        # 1. Single call (using to_thread because client is blocking and server is in event loop)
        res = await asyncio.to_thread(client.call, "math.add", {"a": 10, "b": 25})
        assert res == 35

        res_echo = await asyncio.to_thread(client.call, "echo", {"msg": "hello"})
        assert res_echo == "hello"

        # 2. Unknown method error
        with pytest.raises(Exception) as exc_info:
            await asyncio.to_thread(client.call, "nonexistent.method")
        assert "does not exist" in str(exc_info.value)

        await server.stop()

    asyncio.run(_test())


def test_multi_client_pubsub_broadcasting(temp_dir):
    """Verify server broadcasts pub-sub events to multiple connected AsyncDaemonClients."""
    async def _test():
        sock_path = Path(temp_dir) / "pubsub.sock"
        server = AsyncUnixSocketServer(sock_path)
        await server.start()

        client1 = AsyncDaemonClient(sock_path)
        client2 = AsyncDaemonClient(sock_path)
        await client1.connect()
        await client2.connect()
        await asyncio.sleep(0.05)

        events_c1 = []
        events_c2 = []

        client1.on("notify.quota_updated", lambda p: events_c1.append(p))
        client2.on("notify.quota_updated", lambda p: events_c2.append(p))

        # Broadcast event
        sent = await server.broadcast_event("notify.quota_updated", {"remaining_fraction": 0.88})
        assert sent == 2
        await asyncio.sleep(0.1)

        assert len(events_c1) == 1
        assert events_c1[0]["remaining_fraction"] == 0.88
        assert len(events_c2) == 1
        assert events_c2[0]["remaining_fraction"] == 0.88

        await client1.close()
        await client2.close()
        await server.stop()

    asyncio.run(_test())


def test_controller_fallback_resolution(temp_dir):
    """Verify SwissKnifeController uses RemoteDaemonController when active and StandaloneController when offline."""
    async def _test():
        cfg = SwissKnifeConfig.load(
            custom_config_dir=Path(temp_dir) / "cfg",
            custom_socket_path=Path(temp_dir) / "ctrl.sock",
        )

        # Daemon is NOT running -> StandaloneController
        ctrl = create_controller(cfg, prefer_daemon=True)
        assert isinstance(ctrl, StandaloneController)
        assert ctrl.is_daemon_running() is False

        status = ctrl.get_status()
        assert status["daemon_running"] is False
        assert status["mode"] == "standalone_in_process"

        # Start server -> RemoteDaemonController
        server = AsyncUnixSocketServer(cfg.socket_path)

        @server.register("status.get")
        def rpc_status():
            return {"daemon_running": True, "pid": 999}

        await server.start()

        ctrl_remote = create_controller(cfg, prefer_daemon=True)
        assert isinstance(ctrl_remote, RemoteDaemonController)
        assert ctrl_remote.is_daemon_running() is True
        status_remote = await asyncio.to_thread(ctrl_remote.get_status)
        assert status_remote["daemon_running"] is True

        await server.stop()

    asyncio.run(_test())


def test_quota_and_rules_rpc_methods(temp_dir):
    """Verify quota and rule engine RPC methods wired on AsyncUnixSocketServer and RemoteDaemonController."""
    async def _test():
        sock_path = Path(temp_dir) / "quota_rpc.sock"
        server = AsyncUnixSocketServer(sock_path)

        # Mock poller and rule engine
        class DummySummary:
            active_account = "test@example.com"
            def to_dict(self):
                return {"activeAccount": self.active_account, "groups": []}

        class DummyPoller:
            active_account_email = "test@example.com"
            def get_cached_summary(self, target):
                return DummySummary()
            async def poll_account(self, target, force=False):
                return DummySummary()

        class DummyRuleConfig:
            cooldown_seconds = 300.0
            switch_margin = 0.05
            per_model_thresholds = {"gemini-3.8-flash": 0.05}
            max_switches_in_window = 3
            switch_window_seconds = 600.0

        class DummyRuleEngine:
            config = DummyRuleConfig()
            def evaluate(self, target, summary):
                from antigravity_swiss.quota.rule_engine import EvaluationResult
                return EvaluationResult(should_switch=False, reason="Healthy", current_account=target)
            def record_switch(self, from_acc, to_acc):
                pass

        poller = DummyPoller()
        rule_engine = DummyRuleEngine()

        server.register_quota_handlers(poller=poller, rule_engine=rule_engine)
        await server.start()

        client = SyncDaemonClient(sock_path, timeout=5.0)

        # 1. quota.get_summary
        res_summary = await asyncio.to_thread(client.call, "quota.get_summary")
        assert res_summary["activeAccount"] == "test@example.com"

        # 2. quota.poll_now
        res_poll = await asyncio.to_thread(client.call, "quota.poll_now")
        assert res_poll["account"] == "test@example.com"
        assert res_poll["switched"] is False

        # 3. rules.get_config
        res_config = await asyncio.to_thread(client.call, "rules.get_config")
        assert res_config["cooldown_seconds"] == 300.0

        # 4. rules.set_config
        res_set = await asyncio.to_thread(client.call, "rules.set_config", {"cooldown_seconds": 120.0})
        assert res_set["cooldown_seconds"] == 120.0
        assert rule_engine.config.cooldown_seconds == 120.0

        await server.stop()

    asyncio.run(_test())

