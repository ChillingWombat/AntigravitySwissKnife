"""
Asyncio Unix Domain Socket Server implementing JSON-RPC 2.0 / NDJSON and Pub-Sub Broadcasting.
=============================================================================================
Hosts Unix Domain Socket server at $XDG_RUNTIME_DIR/antigravity-swiss/daemon.sock (mode 0600),
handles stale socket detection, multi-client event broadcasting, and batch JSON-RPC 2.0.
"""

from __future__ import annotations

import asyncio
import json
import logging
import os
from pathlib import Path
import socket
import time
from typing import Any, Callable, Coroutine

from antigravity_swiss.core.constants import (
    JSONRPC_VERSION,
    MAX_FRAME_SIZE,
    SOCKET_DIR_MODE,
    SOCKET_FILE_MODE,
    WIRE_DELIMITER,
)
from antigravity_swiss.core.errors import (
    DaemonAlreadyRunningError,
    InternalRPCError,
    InvalidParamsError,
    InvalidRequestError,
    MethodNotFoundError,
    ParseError,
    SwissKnifeError,
)

logger = logging.getLogger("antigravity_swiss.ipc.server")

RPCMethod = Callable[..., Coroutine[Any, Any, Any] | Any]


class AsyncUnixSocketServer:
    """High-performance Asyncio Unix Domain Socket Server with JSON-RPC 2.0 and Pub-Sub Broadcaster."""

    def __init__(self, socket_path: Path | str) -> None:
        self.socket_path = Path(socket_path).expanduser().resolve()
        self._server: asyncio.AbstractServer | None = None
        self._clients: set[asyncio.StreamWriter] = set()
        self._methods: dict[str, RPCMethod] = {}
        self._running: bool = False
        self._start_time: float = 0.0
        self._loop: asyncio.AbstractEventLoop | None = None
        self._stats = {"requests_total": 0, "errors_total": 0, "events_broadcast": 0}

    @property
    def is_running(self) -> bool:
        return self._running

    @property
    def uptime_seconds(self) -> float:
        return time.time() - self._start_time if self._running else 0.0

    @property
    def client_count(self) -> int:
        return len(self._clients)

    @property
    def stats(self) -> dict[str, int]:
        return dict(self._stats)

    def register(self, method_name: str, handler: RPCMethod | None = None) -> Any:
        """Register an RPC method handler. Can be used as a decorator or direct call."""
        def decorator(fn: RPCMethod) -> RPCMethod:
            self._methods[method_name] = fn
            return fn

        if handler is not None:
            return decorator(handler)
        return decorator

    def register_quota_handlers(
        self,
        poller: Any,
        rule_engine: Any = None,
        keyring_switcher: Any = None,
        config: Any = None,
    ) -> None:
        """Wire RPC methods: quota.get_summary, quota.poll_now, rules.get_config, rules.set_config."""
        @self.register("quota.get_summary")
        async def rpc_quota_summary(account: str | None = None) -> dict[str, Any]:
            target = account or getattr(poller, "active_account_email", None)
            summary = poller.get_cached_summary(target) if hasattr(poller, "get_cached_summary") else None
            if not summary:
                if hasattr(poller, "poll_account"):
                    summary = await poller.poll_account(target, force=False)
                elif hasattr(poller, "poll_summary"):
                    summary = await poller.poll_summary(force=False, email=target)
            return summary.to_dict() if hasattr(summary, "to_dict") else (summary or {})

        @self.register("quota.poll_now")
        async def rpc_quota_poll_now(account: str | None = None) -> dict[str, Any]:
            target = account or getattr(poller, "active_account_email", None)
            if hasattr(poller, "poll_account"):
                summary = await poller.poll_account(target, force=True)
            elif hasattr(poller, "poll_summary"):
                summary = await poller.poll_summary(force=True, email=target)
            else:
                summary = {}

            switched = False
            active_acc = target
            if rule_engine:
                eval_result = rule_engine.evaluate(target, summary)
                if eval_result.should_switch and eval_result.target_account:
                    if keyring_switcher:
                        switched = await asyncio.to_thread(
                            keyring_switcher.switch_to_account, eval_result.target_account
                        )
                    rule_engine.record_switch(target, eval_result.target_account)
                    active_acc = eval_result.target_account
                    self.broadcast_event_threadsafe("notify.account_switched", {
                        "previous_account": target,
                        "active_account": eval_result.target_account,
                        "reason": eval_result.reason,
                    })
                elif eval_result.all_exhausted:
                    self.broadcast_event_threadsafe("notify.all_accounts_exhausted", {
                        "active_account": target,
                        "reason": eval_result.reason,
                    })

            self.broadcast_event_threadsafe("notify.quota_updated", {
                "account": target,
                "summary": summary.to_dict() if hasattr(summary, "to_dict") else summary,
            })
            return {
                "account": target,
                "summary": summary.to_dict() if hasattr(summary, "to_dict") else summary,
                "switched": bool(switched),
                "active_account": active_acc,
            }

        @self.register("rules.get_config")
        def rpc_rules_get_config() -> dict[str, Any]:
            cfg = config or (rule_engine.config if rule_engine else None)
            re_cfg = rule_engine.config if rule_engine else None
            return {
                "auto_switch_enabled": getattr(cfg, "auto_switch_enabled", True) if cfg else True,
                "auto_switch_threshold": getattr(cfg, "auto_switch_threshold", 0.05) if cfg else 0.05,
                "cooldown_seconds": getattr(re_cfg, "cooldown_seconds", 300.0) if re_cfg else 300.0,
                "switch_margin": getattr(re_cfg, "switch_margin", 0.05) if re_cfg else 0.05,
                "per_model_thresholds": getattr(re_cfg, "per_model_thresholds", {}) if re_cfg else {},
                "max_switches_in_window": getattr(re_cfg, "max_switches_in_window", 3) if re_cfg else 3,
                "switch_window_seconds": getattr(re_cfg, "switch_window_seconds", 600.0) if re_cfg else 600.0,
            }

        @self.register("rules.set_config")
        def rpc_rules_set_config(**kwargs: Any) -> dict[str, Any]:
            if rule_engine and getattr(rule_engine, "config", None):
                re_cfg = rule_engine.config
                if "auto_switch_enabled" in kwargs:
                    re_cfg.enabled = bool(kwargs["auto_switch_enabled"])
                if "auto_switch_threshold" in kwargs:
                    val = max(0.0, min(1.0, float(kwargs["auto_switch_threshold"])))
                    re_cfg.default_threshold = val
                if "cooldown_seconds" in kwargs:
                    re_cfg.cooldown_seconds = max(0.0, float(kwargs["cooldown_seconds"]))
                if "switch_margin" in kwargs:
                    re_cfg.switch_margin = max(0.0, min(1.0, float(kwargs["switch_margin"])))
                if "per_model_thresholds" in kwargs and isinstance(kwargs["per_model_thresholds"], dict):
                    re_cfg.per_model_thresholds.update(kwargs["per_model_thresholds"])

            if config:
                if "auto_switch_enabled" in kwargs:
                    config.auto_switch_enabled = bool(kwargs["auto_switch_enabled"])
                if "auto_switch_threshold" in kwargs:
                    config.auto_switch_threshold = max(0.0, min(1.0, float(kwargs["auto_switch_threshold"])))
                if hasattr(config, "save_settings"):
                    config.save_settings()

            return rpc_rules_get_config()

    def register_fingerprint_handlers(self, fingerprint_manager: Any) -> None:
        """Register Device Fingerprint RPC methods."""
        @self.register("fingerprint.get_profile")
        def rpc_fingerprint_get_profile() -> dict[str, Any]:
            profile = fingerprint_manager.get_active_profile()
            return profile.to_dict() if hasattr(profile, "to_dict") else profile

        @self.register("fingerprint.list_profiles")
        def rpc_fingerprint_list_profiles() -> dict[str, Any]:
            accounts = fingerprint_manager.store.list_accounts()
            result = {}
            for acc in accounts:
                p = fingerprint_manager.store.get_profile(acc)
                if p:
                    result[acc] = p.to_dict() if hasattr(p, "to_dict") else p
            return result

        @self.register("fingerprint.swap")
        def rpc_fingerprint_swap(email: str | None = None, account_email: str | None = None) -> dict[str, Any]:
            target = email or account_email
            if not target:
                raise InvalidParamsError("Target email or account_email parameter required")
            profile = fingerprint_manager.swap_profile_for_account(target)
            payload = {
                "success": True,
                "account_email": target,
                "profile": profile.to_dict() if hasattr(profile, "to_dict") else profile,
            }
            self.broadcast_event_threadsafe("notify.profile_swapped", payload)
            return payload

    def register_cache_handlers(
        self,
        inspector: Any,
        pruner: Any,
        prompt_optimizer: Any = None,
    ) -> None:
        """Register Cache Optimizer RPC methods."""
        @self.register("cache.get_breakdown")
        def rpc_cache_get_breakdown(active_conversation_id: str | None = None) -> dict[str, Any]:
            breakdown = inspector.scan_breakdown(active_conversation_id=active_conversation_id)
            return breakdown.to_dict() if hasattr(breakdown, "to_dict") else breakdown

        @self.register("cache.prune")
        def rpc_cache_prune(
            options: dict[str, Any] | None = None,
            active_conversation_id: str | None = None,
            **kwargs: Any,
        ) -> dict[str, Any]:
            opts_dict = options or kwargs or {}
            from antigravity_swiss.cache_optimizer.models import PruneOptions
            prune_opts = PruneOptions(
                prune_scratch=opts_dict.get("prune_scratch", True),
                prune_steps=opts_dict.get("prune_steps", True),
                prune_tasks=opts_dict.get("prune_tasks", True),
                prune_screenshots=opts_dict.get("prune_screenshots", True),
                vacuum_databases=opts_dict.get("vacuum_databases", True),
                prune_wal=opts_dict.get("prune_wal", False),
                min_age_days=float(opts_dict.get("min_age_days", 3.0)),
                dry_run=bool(opts_dict.get("dry_run", False)),
            )
            res = pruner.prune(options=prune_opts, active_conversation_id=active_conversation_id)
            res_dict = res.to_dict() if hasattr(res, "to_dict") else res
            self.broadcast_event_threadsafe("notify.cache_pruned", res_dict)
            return res_dict

        @self.register("cache.analyze_prompts")
        def rpc_cache_analyze_prompts(
            conversation_id: str | None = None,
            transcript_path: str | None = None,
        ) -> dict[str, Any]:
            if not prompt_optimizer:
                from antigravity_swiss.cache_optimizer.prompt_cache import PromptCacheOptimizer
                optimizer = PromptCacheOptimizer(data_dir=getattr(inspector, "data_dir", None))
            else:
                optimizer = prompt_optimizer

            if transcript_path:
                analysis = optimizer.analyze_transcript(transcript_path)
            elif conversation_id:
                analysis = optimizer.analyze_conversation(conversation_id)
            else:
                active_id = getattr(inspector, "get_active_conversation_id", lambda: None)()
                if active_id:
                    analysis = optimizer.analyze_conversation(active_id)
                else:
                    results = optimizer.scan_all_conversations(limit=1)
                    analysis = results[0] if results else None

            if not analysis:
                return {"error": "No conversation transcript found to analyze"}
            return analysis.to_dict() if hasattr(analysis, "to_dict") else analysis

    async def start(self) -> None:
        """Bind socket, set permissions to 0600, and start listening."""
        if self._running:
            return

        # 1. Ensure socket parent directory exists with 0700
        socket_dir = self.socket_path.parent
        socket_dir.mkdir(parents=True, exist_ok=True)
        if str(socket_dir) not in ("/tmp", "/var/tmp"):
            try:
                os.chmod(socket_dir, SOCKET_DIR_MODE)
            except OSError as e:
                logger.warning("Could not chmod socket dir: %s", e)


        # 2. Check for existing socket file (detect live daemon vs stale socket)
        if self.socket_path.exists():
            test_sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
            test_sock.settimeout(0.5)
            try:
                test_sock.connect(str(self.socket_path))
                test_sock.close()
                # Active daemon is listening!
                raise DaemonAlreadyRunningError(
                    f"Antigravity Swiss Knife daemon is already active at {self.socket_path}"
                )
            except (ConnectionRefusedError, socket.timeout, OSError):
                # Socket is stale; safely remove it
                try:
                    self.socket_path.unlink()
                    logger.info("Unlinked stale socket file at %s", self.socket_path)
                except OSError as e:
                    logger.error("Failed to remove stale socket: %s", e)
                    raise

        # 3. Start Unix Domain Server
        self._loop = asyncio.get_running_loop()
        self._server = await asyncio.start_unix_server(
            client_connected_cb=self._handle_client,
            path=str(self.socket_path),
            limit=MAX_FRAME_SIZE,
        )

        # 4. Enforce strict 0600 permissions on the socket file
        try:
            os.chmod(self.socket_path, SOCKET_FILE_MODE)
        except OSError as e:
            logger.warning("Could not chmod socket file: %s", e)

        self._running = True
        self._start_time = time.time()
        logger.info("Daemon socket server active at %s (mode 0600)", self.socket_path)

    async def stop(self) -> None:
        """Gracefully stop server, disconnect clients, and clean up socket file."""
        if not self._running:
            return
        self._running = False

        # Close all connected client writers
        for writer in list(self._clients):
            try:
                writer.close()
                await writer.wait_closed()
            except Exception:
                pass
        self._clients.clear()

        # Stop listening
        if self._server:
            self._server.close()
            await self._server.wait_closed()
            self._server = None

        # Unlink socket file
        if self.socket_path.exists():
            try:
                self.socket_path.unlink()
            except OSError:
                pass
        logger.info("Daemon socket server stopped cleanly.")

    async def broadcast_event(self, event_name: str, payload: dict[str, Any]) -> int:
        """Broadcast a JSON-RPC 2.0 notification to all connected clients."""
        notification = {
            "jsonrpc": JSONRPC_VERSION,
            "method": event_name,
            "params": payload,
        }
        msg = (json.dumps(notification, ensure_ascii=False) + "\n").encode("utf-8")
        clients = list(self._clients)
        if not clients:
            self._stats["events_broadcast"] += 1
            return 0

        async def _send_to_client(writer: asyncio.StreamWriter) -> bool:
            try:
                writer.write(msg)
                await asyncio.wait_for(writer.drain(), timeout=0.5)
                return True
            except (ConnectionResetError, BrokenPipeError, OSError, asyncio.TimeoutError):
                return False

        results = await asyncio.gather(*[_send_to_client(w) for w in clients], return_exceptions=True)
        sent = 0
        for writer, res in zip(clients, results):
            if res is True:
                sent += 1
            else:
                self._clients.discard(writer)
                try:
                    writer.close()
                except Exception:
                    pass

        self._stats["events_broadcast"] += 1
        return sent

    def broadcast_event_threadsafe(self, event_name: str, payload: dict[str, Any]) -> None:
        """Thread-safe event broadcast from non-async contexts or background threads."""
        if self._loop and self._loop.is_running():
            asyncio.run_coroutine_threadsafe(self.broadcast_event(event_name, payload), self._loop)

    async def _handle_client(self, reader: asyncio.StreamReader, writer: asyncio.StreamWriter) -> None:
        self._clients.add(writer)
        try:
            while self._running:
                try:
                    line = await reader.readline()
                except asyncio.LimitOverrunError:
                    err_res = ParseError("Message exceeds 10MB limit").to_rpc_error()
                    writer.write((json.dumps({"jsonrpc": JSONRPC_VERSION, "error": err_res, "id": None}) + "\n").encode("utf-8"))
                    await writer.drain()
                    break

                if not line:
                    break  # Client closed connection

                try:
                    line_str = line.decode("utf-8").strip()
                except UnicodeDecodeError:
                    err_res = ParseError("Invalid UTF-8 encoding").to_rpc_error()
                    writer.write((json.dumps({"jsonrpc": JSONRPC_VERSION, "error": err_res, "id": None}) + "\n").encode("utf-8"))
                    await writer.drain()
                    continue

                if not line_str:
                    continue

                res_bytes = await self._process_raw_line(line_str)
                if res_bytes:
                    writer.write(res_bytes + WIRE_DELIMITER)
                    await writer.drain()
        except (ConnectionResetError, BrokenPipeError, asyncio.IncompleteReadError):
            pass
        except Exception as exc:
            logger.error("Client connection error: %s", exc, exc_info=True)
        finally:
            self._clients.discard(writer)
            try:
                writer.close()
                await writer.wait_closed()
            except Exception:
                pass

    async def _process_raw_line(self, line: str) -> bytes | None:
        """Parse raw line and execute batch or single JSON-RPC request."""
        try:
            parsed = json.loads(line)
        except json.JSONDecodeError as jde:
            self._stats["errors_total"] += 1
            err = ParseError(f"Parse error: {jde}").to_rpc_error()
            return json.dumps({"jsonrpc": JSONRPC_VERSION, "error": err, "id": None}).encode("utf-8")

        if isinstance(parsed, list):
            # Batch request
            if not parsed:
                err = InvalidRequestError("Batch cannot be empty").to_rpc_error()
                return json.dumps({"jsonrpc": JSONRPC_VERSION, "error": err, "id": None}).encode("utf-8")
            results = []
            for item in parsed:
                res = await self._dispatch_single(item)
                if res is not None:
                    results.append(res)
            return json.dumps(results).encode("utf-8") if results else None
        elif isinstance(parsed, dict):
            res = await self._dispatch_single(parsed)
            return json.dumps(res).encode("utf-8") if res is not None else None
        else:
            err = InvalidRequestError("Top-level JSON entity must be object or array").to_rpc_error()
            return json.dumps({"jsonrpc": JSONRPC_VERSION, "error": err, "id": None}).encode("utf-8")

    async def _dispatch_single(self, req: Any) -> dict[str, Any] | None:
        if not isinstance(req, dict):
            return {"jsonrpc": JSONRPC_VERSION, "error": InvalidRequestError().to_rpc_error(), "id": None}

        req_id = req.get("id")
        is_notification = ("id" not in req)

        if req.get("jsonrpc") != JSONRPC_VERSION:
            if is_notification:
                return None
            return {"jsonrpc": JSONRPC_VERSION, "error": InvalidRequestError("jsonrpc must be '2.0'").to_rpc_error(), "id": req_id}

        method = req.get("method")
        if not isinstance(method, str):
            if is_notification:
                return None
            return {"jsonrpc": JSONRPC_VERSION, "error": InvalidRequestError("method must be string").to_rpc_error(), "id": req_id}

        if method not in self._methods:
            if is_notification:
                return None
            return {"jsonrpc": JSONRPC_VERSION, "error": MethodNotFoundError(method).to_rpc_error(), "id": req_id}

        handler = self._methods[method]
        params = req.get("params")
        self._stats["requests_total"] += 1

        try:
            if params is None:
                ret = handler()
            elif isinstance(params, dict):
                ret = handler(**params)
            elif isinstance(params, list):
                ret = handler(*params)
            else:
                if is_notification:
                    return None
                return {"jsonrpc": JSONRPC_VERSION, "error": InvalidParamsError().to_rpc_error(), "id": req_id}

            if asyncio.iscoroutine(ret):
                ret = await ret

            if is_notification:
                return None
            return {"jsonrpc": JSONRPC_VERSION, "result": ret, "id": req_id}
        except TypeError as te:
            self._stats["errors_total"] += 1
            if is_notification:
                return None
            return {"jsonrpc": JSONRPC_VERSION, "error": InvalidParamsError(str(te)).to_rpc_error(), "id": req_id}
        except SwissKnifeError as ske:
            self._stats["errors_total"] += 1
            if is_notification:
                return None
            return {"jsonrpc": JSONRPC_VERSION, "error": ske.to_rpc_error(), "id": req_id}
        except Exception as exc:
            self._stats["errors_total"] += 1
            logger.exception("Internal error executing %s", method)
            if is_notification:
                return None
            return {"jsonrpc": JSONRPC_VERSION, "error": InternalRPCError(str(exc)).to_rpc_error(), "id": req_id}
