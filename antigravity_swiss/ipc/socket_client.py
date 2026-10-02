"""
Async and Sync Unix Domain Socket Clients for JSON-RPC 2.0 Daemon Communication.
================================================================================
Provides client connectivity, synchronous one-off requests for CLI commands,
and asynchronous pub-sub event subscription for GUI clients.
"""

from __future__ import annotations

import asyncio
import collections
import json
import logging
from pathlib import Path
import socket
import time
from typing import Any, Callable

from antigravity_swiss.core.constants import (
    DEFAULT_CLIENT_TIMEOUT_SECONDS,
    JSONRPC_VERSION,
    MAX_FRAME_SIZE,
    WIRE_DELIMITER,
)
from antigravity_swiss.core.errors import (
    DaemonNotRunningError,
    IPCError,
    SwissKnifeError,
)

logger = logging.getLogger("antigravity_swiss.ipc.client")

EventHandler = Callable[[dict[str, Any]], Any]


class AsyncDaemonClient:
    """Asynchronous client with auto-reconnect and pub-sub notification dispatch."""

    def __init__(self, socket_path: Path | str, timeout: float = DEFAULT_CLIENT_TIMEOUT_SECONDS) -> None:
        self.socket_path = Path(socket_path).expanduser().resolve()
        self.timeout = timeout
        self._reader: asyncio.StreamReader | None = None
        self._writer: asyncio.StreamWriter | None = None
        self._pending: dict[int | str, asyncio.Future[Any]] = {}
        self._event_handlers: dict[str, list[EventHandler]] = collections.defaultdict(list)
        self._read_task: asyncio.Task[None] | None = None
        self._req_counter = 0
        self._connected = False
        self._lock = asyncio.Lock()

    @property
    def is_connected(self) -> bool:
        return self._connected

    async def connect(self) -> None:
        """Connect to the daemon socket and launch read loop."""
        async with self._lock:
            if self._connected:
                return
            if not self.socket_path.exists():
                raise DaemonNotRunningError(f"Daemon socket not found at {self.socket_path}")

            try:
                self._reader, self._writer = await asyncio.open_unix_connection(
                    str(self.socket_path), limit=MAX_FRAME_SIZE
                )
                self._connected = True
                self._read_task = asyncio.create_task(self._read_loop())
                logger.debug("Connected to daemon at %s", self.socket_path)
            except (ConnectionRefusedError, FileNotFoundError, OSError) as exc:
                self._connected = False
                raise DaemonNotRunningError(f"Failed to connect to daemon socket at {self.socket_path}: {exc}") from exc

    async def close(self) -> None:
        """Disconnect cleanly and cancel pending requests."""
        async with self._lock:
            self._connected = False
            if self._read_task:
                self._read_task.cancel()
                try:
                    await self._read_task
                except asyncio.CancelledError:
                    pass
                self._read_task = None

            if self._writer:
                try:
                    self._writer.close()
                    await self._writer.wait_closed()
                except Exception:
                    pass
                self._writer = None
            self._reader = None

            # Cancel pending requests
            for fut in self._pending.values():
                if not fut.done():
                    fut.set_exception(IPCError("Connection closed"))
            self._pending.clear()

    async def call(self, method: str, params: dict[str, Any] | None = None, timeout: float | None = None) -> Any:
        """Send a JSON-RPC request and await response."""
        timeout = timeout or self.timeout
        if not self._connected:
            await self.connect()

        self._req_counter += 1
        req_id = self._req_counter
        fut: asyncio.Future[Any] = asyncio.get_running_loop().create_future()
        self._pending[req_id] = fut

        body: dict[str, Any] = {
            "jsonrpc": JSONRPC_VERSION,
            "method": method,
            "id": req_id,
        }
        if params is not None:
            body["params"] = params

        data = (json.dumps(body) + "\n").encode("utf-8")
        try:
            if not self._writer:
                raise DaemonNotRunningError("Socket writer unavailable")
            self._writer.write(data)
            await self._writer.drain()
            return await asyncio.wait_for(fut, timeout=timeout)
        except asyncio.TimeoutError:
            self._pending.pop(req_id, None)
            raise IPCError(f"RPC method '{method}' timed out after {timeout}s")
        except Exception:
            self._pending.pop(req_id, None)
            raise

    def on(self, event_name: str, handler: EventHandler) -> None:
        """Subscribe to a pub-sub broadcast event."""
        self._event_handlers[event_name].append(handler)

    async def _read_loop(self) -> None:
        try:
            while self._connected and self._reader:
                line = await self._reader.readline()
                if not line:
                    break
                line_str = line.decode("utf-8").strip()
                if not line_str:
                    continue

                try:
                    msg = json.loads(line_str)
                except json.JSONDecodeError:
                    continue

                if isinstance(msg, dict):
                    req_id = msg.get("id")
                    if req_id is not None and req_id in self._pending:
                        fut = self._pending.pop(req_id)
                        if not fut.done():
                            if "error" in msg:
                                err = msg["error"]
                                fut.set_exception(SwissKnifeError(
                                    message=err.get("message", "RPC Error"),
                                    code=err.get("code", -32000),
                                    data=err.get("data"),
                                ))
                            else:
                                fut.set_result(msg.get("result"))
                    elif "method" in msg and req_id is None:
                        # Event notification
                        method = msg["method"]
                        params = msg.get("params", {})
                        for h in self._event_handlers.get(method, []):
                            try:
                                res = h(params)
                                if asyncio.iscoroutine(res):
                                    asyncio.create_task(res)
                            except Exception as ex:
                                logger.error("Event handler error on %s: %s", method, ex)
        except asyncio.CancelledError:
            pass
        finally:
            self._connected = False


class SyncDaemonClient:
    """Synchronous socket client for one-off CLI commands."""

    def __init__(self, socket_path: Path | str, timeout: float = DEFAULT_CLIENT_TIMEOUT_SECONDS) -> None:
        self.socket_path = Path(socket_path).expanduser().resolve()
        self.timeout = timeout

    def is_daemon_alive(self) -> bool:
        """Quickly check if daemon socket accepts connections."""
        if not self.socket_path.exists():
            return False
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        sock.settimeout(0.5)
        try:
            sock.connect(str(self.socket_path))
            sock.close()
            return True
        except (ConnectionRefusedError, socket.timeout, OSError):
            return False

    def call(self, method: str, params: dict[str, Any] | None = None) -> Any:
        """Execute a blocking JSON-RPC call."""
        if not self.socket_path.exists():
            raise DaemonNotRunningError(f"Daemon socket does not exist at {self.socket_path}")

        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        sock.settimeout(self.timeout)
        try:
            sock.connect(str(self.socket_path))
            body: dict[str, Any] = {
                "jsonrpc": JSONRPC_VERSION,
                "method": method,
                "id": 1,
            }
            if params is not None:
                body["params"] = params

            sock.sendall((json.dumps(body) + "\n").encode("utf-8"))

            buf = bytearray()
            while True:
                chunk = sock.recv(4096)
                if not chunk:
                    raise IPCError("Connection closed before response received")
                buf.extend(chunk)
                if b"\n" in buf:
                    break

            line = buf.split(b"\n", 1)[0].decode("utf-8")
            data = json.loads(line)

            if "error" in data:
                err = data["error"]
                raise SwissKnifeError(
                    message=err.get("message", "RPC Error"),
                    code=err.get("code", -32000),
                    data=err.get("data"),
                )
            return data.get("result")
        except socket.timeout:
            raise IPCError(f"RPC call '{method}' timed out after {self.timeout}s")
        except ConnectionRefusedError as cre:
            raise DaemonNotRunningError(f"Daemon refused connection at {self.socket_path}") from cre
        finally:
            try:
                sock.close()
            except Exception:
                pass
