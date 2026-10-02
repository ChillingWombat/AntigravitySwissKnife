"""
Antigravity Swiss Knife IPC Package.
====================================
Unix Domain Socket JSON-RPC 2.0 / NDJSON server, clients, and controllers.
"""

from antigravity_swiss.ipc.controller import (
    RemoteDaemonController,
    StandaloneController,
    SwissKnifeController,
    create_controller,
)
from antigravity_swiss.ipc.socket_client import AsyncDaemonClient, SyncDaemonClient
from antigravity_swiss.ipc.socket_server import AsyncUnixSocketServer

__all__ = [
    "AsyncUnixSocketServer",
    "AsyncDaemonClient",
    "SyncDaemonClient",
    "SwissKnifeController",
    "RemoteDaemonController",
    "StandaloneController",
    "create_controller",
]
