"""
Antigravity Swiss Knife Unified CLI and Background Daemon Entry Point.
======================================================================
Subcommands:
- daemon: Run background daemon process (Unix domain socket server)
- status: Query active daemon, Antigravity process, and active account status
- switch: Rotate active Google account and preserve session
- gui: Launch desktop GUI (PySide6)
"""

from __future__ import annotations

import argparse
import asyncio
import json
import logging
import os
from pathlib import Path
import signal
import sys
from typing import Any

from antigravity_swiss.core.config import SwissKnifeConfig
from antigravity_swiss.core.constants import APP_TITLE, APP_VERSION
from antigravity_swiss.core.errors import (
    DaemonAlreadyRunningError,
    DaemonNotRunningError,
    SwissKnifeError,
)
from antigravity_swiss.ipc.controller import create_controller
from antigravity_swiss.ipc.socket_server import AsyncUnixSocketServer


def setup_logging(verbose: bool = False) -> None:
    level = logging.DEBUG if verbose else logging.INFO
    logging.basicConfig(
        level=level,
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
        datefmt="%H:%M:%S",
    )


def run_daemon(args: argparse.Namespace) -> int:
    """Run the background daemon process."""
    setup_logging(args.verbose)
    logger = logging.getLogger("antigravity_swiss.daemon")
    config = SwissKnifeConfig.load(
        custom_config_dir=Path(args.config) if args.config else None,
        custom_socket_path=Path(args.socket) if args.socket else None,
    )

    if args.poll_interval:
        config.poll_interval_sec = float(args.poll_interval)
    if args.threshold:
        config.auto_switch_threshold = float(args.threshold)
    if args.no_warmup:
        config.warmup_enabled = False

    config.ensure_directories()
    server = AsyncUnixSocketServer(config.socket_path)

    # Initialize Keyring & Account Vault
    from antigravity_swiss.keyring.switcher import AccountStore, AccountVault, KeyringService, KeyringSwitcher
    from antigravity_swiss.process.lifecycle import ProcessManager
    from antigravity_swiss.quota.poller import QuotaPoller
    from antigravity_swiss.quota.rule_engine import AutoSwitchRuleEngine, RuleEngineConfig

    vault = AccountVault(config_path=config.accounts_file)
    keyring_service = KeyringService(vault=vault)
    keyring_switcher = KeyringSwitcher(config)
    pm = ProcessManager(config=config)

    rule_config = RuleEngineConfig(
        enabled=config.auto_switch_enabled,
        default_threshold=config.auto_switch_threshold,
    )
    rule_engine = AutoSwitchRuleEngine(vault=vault, keyring_service=keyring_service, config=rule_config)

    def _on_quota_updated(summary: Any) -> None:
        server.broadcast_event_threadsafe("notify.quota_updated", {
            "account": getattr(summary, "active_account", None),
            "summary": summary.to_dict() if hasattr(summary, "to_dict") else summary,
        })

    poller = QuotaPoller(
        config=config,
        vault=vault,
        keyring_service=keyring_service,
        poll_interval_sec=config.poll_interval_sec,
        on_quota_updated=_on_quota_updated,
    )

    # Wire core RPC methods
    @server.register("status.get")
    def rpc_status() -> dict[str, Any]:
        pid = pm.get_running_antigravity_pid()
        active_account = vault.get_active_account()

        return {
            "daemon_running": True,
            "pid": os.getpid(),
            "uptime_seconds": round(server.uptime_seconds, 1),
            "client_connections": server.client_count,
            "antigravity_running": pid is not None,
            "antigravity_pid": pid,
            "active_account": active_account,
            "auto_switch_enabled": config.auto_switch_enabled,
            "auto_switch_threshold": config.auto_switch_threshold,
            "warmup_enabled": config.warmup_enabled,
        }

    @server.register("accounts.list")
    def rpc_accounts_list() -> list[dict[str, Any]]:
        store = AccountStore(config.accounts_file)
        return store.list_accounts()

    @server.register("accounts.switch")
    def rpc_accounts_switch(email: str, force: bool = False, relaunch: bool = True) -> dict[str, Any]:
        res = keyring_switcher.switch_to_account(email, force=force)
        relaunch_pid = None
        if relaunch:
            pm.terminate_gracefully(config.process_timeout_sec)
            try:
                relaunch_pid = pm.relaunch()
            except Exception as exc:
                logger.warning("Could not relaunch Antigravity: %s", exc)

        result_payload = {
            "success": True,
            "active_account": email,
            "relaunch_pid": relaunch_pid,
            "preserved_cascade_id": res.get("cascade_id"),
        }
        server.broadcast_event_threadsafe("notify.account_switched", {"email": email, "reason": "manual_rpc"})
        return result_payload

    # Wire Quota & Rule Engine RPC methods
    server.register_quota_handlers(
        poller=poller,
        rule_engine=rule_engine,
        keyring_switcher=keyring_switcher,
        config=config,
    )

    @server.register("daemon.shutdown")
    async def rpc_shutdown() -> dict[str, Any]:
        asyncio.create_task(server.stop())
        return {"status": "shutting_down"}

    async def main_async() -> None:
        stop_event = asyncio.Event()

        def _sig_handler() -> None:
            logger.info("Signal received, stopping daemon...")
            stop_event.set()

        loop = asyncio.get_running_loop()
        for sig in (signal.SIGTERM, signal.SIGINT):
            try:
                loop.add_signal_handler(sig, _sig_handler)
            except (ValueError, NotImplementedError):
                pass

        try:
            await server.start()
            print(f"[{APP_TITLE}] Daemon listening at {config.socket_path} (PID {os.getpid()})")
            await poller.start()
            await stop_event.wait()
        except DaemonAlreadyRunningError as e:
            logger.error("%s", e)
            sys.exit(1)
        finally:
            await poller.stop()
            await server.stop()

    try:
        asyncio.run(main_async())
        return 0
    except (KeyboardInterrupt, SystemExit):
        return 0
    except Exception as exc:
        logger.error("Daemon crashed: %s", exc, exc_info=True)
        return 1


def run_status(args: argparse.Namespace) -> int:
    """Query daemon or local Antigravity status and display summary."""
    config = SwissKnifeConfig.load(
        custom_socket_path=Path(args.socket) if args.socket else None
    )
    controller = create_controller(config, prefer_daemon=True)

    try:
        status_data = controller.get_status()
    except Exception as exc:
        print(f"[ERROR] Failed to query status: {exc}", file=sys.stderr)
        return 1

    if args.json:
        print(json.dumps(status_data, indent=2))
        return 0

    daemon_active = status_data.get("daemon_running", False)
    daemon_label = f"● ACTIVE (PID {status_data.get('pid')}, Uptime {status_data.get('uptime_seconds')}s)" if daemon_active else "○ INACTIVE (standalone fallback)"
    ag_running = status_data.get("antigravity_running", False)
    ag_label = f"● RUNNING (PID {status_data.get('antigravity_pid')})" if ag_running else "○ NOT RUNNING"

    print("═" * 66)
    print(f"               {APP_TITLE} (v{APP_VERSION})")
    print("═" * 66)
    print(f"Daemon Status       : {daemon_label}")
    print(f"Socket Path         : {config.socket_path}")
    print(f"Antigravity App     : {ag_label}")
    print(f"Active Account      : {status_data.get('active_account') or 'Unknown'}")
    print("─" * 66)
    print("To view live quota gauges, launch the GUI: python -m antigravity_swiss gui")
    print("═" * 66)
    return 0


def run_switch(args: argparse.Namespace) -> int:
    """Switch active Google account."""
    config = SwissKnifeConfig.load()
    controller = create_controller(config, prefer_daemon=True)

    target_email = args.email
    print(f"[*] Initiating switch to account: {target_email}...")
    try:
        res = controller.switch_account(
            email=target_email,
            force=args.force,
            relaunch=not args.no_relaunch,
        )
        print(f"[SUCCESS] Switched active account to: {res.get('active_account')}")
        if res.get("relaunch_pid"):
            print(f"[+] Antigravity relaunched with clean session (PID {res.get('relaunch_pid')})")
        return 0
    except SwissKnifeError as ske:
        print(f"[ERROR] Account switch failed: {ske.message}", file=sys.stderr)
        return 1
    except Exception as exc:
        print(f"[ERROR] Unexpected error during switch: {exc}", file=sys.stderr)
        return 1


def run_gui(args: argparse.Namespace) -> int:
    """Launch Material Design 3 Desktop GUI."""
    try:
        import PySide6  # noqa: F401
    except ImportError:
        print("[ERROR] PySide6 desktop GUI libraries are not installed in this Python environment.", file=sys.stderr)
        print("To install GUI support: pip install PySide6", file=sys.stderr)
        print("You can manage accounts, quotas, and daemon services using the CLI:", file=sys.stderr)
        print("  python -m antigravity_swiss daemon", file=sys.stderr)
        print("  python -m antigravity_swiss status", file=sys.stderr)
        print("  python -m antigravity_swiss switch <email>", file=sys.stderr)
        return 1

    try:
        from antigravity_swiss.gui.app import run_app
        return run_app(standalone=args.standalone)
    except ImportError as e:
        print(f"[INFO] GUI module not yet installed: {e}", file=sys.stderr)
        return 1


def main() -> int:
    parser = argparse.ArgumentParser(
        prog="python -m antigravity_swiss",
        description=f"{APP_TITLE} (v{APP_VERSION}): Native desktop companion and daemon for Google Antigravity 2.0",
    )
    parser.add_argument("-v", "--verbose", action="store_true", help="Enable verbose debug logging")
    subparsers = parser.add_subparsers(dest="command", required=True, help="Subcommand to execute")

    # daemon
    p_daemon = subparsers.add_parser("daemon", help="Run background daemon process")
    p_daemon.add_argument("--socket", type=str, help="Custom socket path")
    p_daemon.add_argument("--config", type=str, help="Custom configuration directory")
    p_daemon.add_argument("--poll-interval", type=float, help="Quota polling interval in seconds")
    p_daemon.add_argument("--threshold", type=float, help="Auto-switch threshold fraction (e.g. 0.05)")
    p_daemon.add_argument("--no-warmup", action="store_true", help="Disable keep-alive warmup engine")
    p_daemon.set_defaults(func=run_daemon)

    # status
    p_status = subparsers.add_parser("status", help="Query status and active account")
    p_status.add_argument("--json", action="store_true", help="Output raw JSON")
    p_status.add_argument("--socket", type=str, help="Custom socket path")
    p_status.set_defaults(func=run_status)

    # switch
    p_switch = subparsers.add_parser("switch", help="Switch active Google account")
    p_switch.add_argument("email", type=str, help="Target account email")
    p_switch.add_argument("--force", action="store_true", help="Force switch regardless of quota")
    p_switch.add_argument("--no-relaunch", action="store_true", help="Do not relaunch Antigravity")
    p_switch.set_defaults(func=run_switch)

    # gui
    p_gui = subparsers.add_parser("gui", help="Launch Material Design 3 Desktop GUI")
    p_gui.add_argument("--standalone", action="store_true", help="Run in standalone mode without daemon")
    p_gui.set_defaults(func=run_gui)

    args = parser.parse_args()
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())
