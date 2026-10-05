"""
Antigravity Swiss Knife Unified CLI and Background Daemon Entry Point.
======================================================================
Subcommands:
- daemon: Run background daemon process (Unix domain socket server)
- status: Query active daemon, Antigravity process, and active account status
- switch: Rotate active Google account and preserve session
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
        try:
            fingerprint_manager.swap_profile_for_account(email)
        except Exception as exc:
            logger.warning("Could not swap fingerprint profile on switch: %s", exc)

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

    @server.register("accounts.update")
    def rpc_accounts_update(
        email: str,
        label: str | None = None,
        plan_tier: str | None = None,
        status: str | None = None,
        totp_secret: str | None = None,
        refresh_token: str | None = None,
        set_active: bool = False,
        priority: str | None = None,
        notes: str | None = None,
        password: str | None = None,
    ) -> dict[str, Any]:
        store = AccountStore(config.accounts_file)
        success = store.update_account_info(
            email=email,
            label=label,
            plan_tier=plan_tier,
            status=status,
            totp_secret=totp_secret,
            refresh_token=refresh_token,
            set_active=set_active,
            priority=priority,
            notes=notes,
            password=password,
        )
        if set_active and (status or "").upper() not in ("BANNED", "ERROR"):
            try:
                keyring_switcher.switch_to_account(email, force=True)
            except Exception as exc:
                logger.warning("Could not set active account on update: %s", exc)
        return {"success": success, "email": email}

    @server.register("accounts.remove")
    def rpc_accounts_remove(email: str) -> dict[str, Any]:
        store = AccountStore(config.accounts_file)
        success = store.remove_account(email)
        return {"success": success, "email": email}

    @server.register("accounts.set_totp")
    def rpc_accounts_set_totp(email: str, totp_secret: str) -> dict[str, Any]:
        store = AccountStore(config.accounts_file)
        success = store.update_account(email=email, totp_secret=totp_secret)
        return {"success": success, "email": email}

    # Initialize Fingerprint Manager
    from antigravity_swiss.fingerprint.manager import FingerprintManager
    fingerprint_manager = FingerprintManager(
        config_dir=config.antigravity_config_dir,
        data_dir=config.antigravity_data_dir,
    )
    server.register_fingerprint_handlers(fingerprint_manager)

    # Initialize Cache Optimizer
    from antigravity_swiss.cache_optimizer.inspector import BrainCacheInspector
    from antigravity_swiss.cache_optimizer.pruner import BrainCachePruner
    from antigravity_swiss.cache_optimizer.prompt_cache import PromptCacheOptimizer
    cache_inspector = BrainCacheInspector(
        data_dir=config.antigravity_data_dir,
        config_dir=config.antigravity_config_dir,
    )
    cache_pruner = BrainCachePruner(
        data_dir=config.antigravity_data_dir,
        config_dir=config.antigravity_config_dir,
    )
    prompt_optimizer = PromptCacheOptimizer(
        data_dir=config.antigravity_data_dir,
    )
    server.register_cache_handlers(cache_inspector, cache_pruner, prompt_optimizer)

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
    print("To view live quota gauges, launch the Web GUI or desktop app: bin/swiss web")
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


def run_cache_breakdown(args: argparse.Namespace) -> int:
    """Display categorized cache breakdown."""
    config = SwissKnifeConfig.load()
    controller = create_controller(config, prefer_daemon=True)
    try:
        data = controller.get_cache_breakdown()
    except Exception as exc:
        print(f"[ERROR] Failed to retrieve cache breakdown: {exc}", file=sys.stderr)
        return 1

    if args.json:
        print(json.dumps(data, indent=2))
        return 0

    print("═" * 66)
    print("                 ANTIGRAVITY CACHE BREAKDOWN")
    print("═" * 66)
    print(f"Total Cache Size     : {data.get('total_mb', 0)} MB ({data.get('total_bytes', 0):,} bytes)")
    print(f"Brain Storage        : {data.get('brain_total_mb', 0)} MB")
    print(f"Conversations DBs    : {data.get('conversations_total_mb', 0)} MB")
    print(f"Active Session       : {data.get('active_session_mb', 0)} MB")
    print(f"Reclaimable Cache    : {data.get('reclaimable_mb', 0)} MB")
    print(f"Conversations Count  : {data.get('conversation_count', 0)}")
    print("─" * 66)
    print(f"{'Category':<16} {'Size (MB)':<12} {'Files':<10} {'Reclaimable (MB)'}")
    print("─" * 66)
    categories = data.get("categories", {})
    for cat_name, cat in categories.items():
        if cat_name in ("steps", "scratch", "tasks", "messages", "logs", "artifacts"):
            continue
        print(f"{cat_name:<16} {cat.get('total_mb', 0):<12} {cat.get('file_count', 0):<10} {cat.get('reclaimable_mb', 0)}")
    print("═" * 66)
    return 0


def run_cache_prune(args: argparse.Namespace) -> int:
    """Execute cache pruning with active session protection."""
    config = SwissKnifeConfig.load()
    controller = create_controller(config, prefer_daemon=True)
    options = {
        "min_age_days": args.min_age_days,
        "dry_run": args.dry_run,
        "prune_scratch": not args.no_scratch,
        "prune_steps": not args.no_steps,
        "prune_tasks": not args.no_tasks,
    }
    try:
        data = controller.prune_cache(options=options)
    except Exception as exc:
        print(f"[ERROR] Failed to prune cache: {exc}", file=sys.stderr)
        return 1

    if args.json:
        print(json.dumps(data, indent=2))
        return 0

    prefix = "[DRY RUN] " if data.get("dry_run") else "[SUCCESS] "
    print(f"{prefix}Cache cleanup complete:")
    print(f"  Freed Space        : {data.get('bytes_freed_mb', 0)} MB ({data.get('bytes_freed', 0):,} bytes)")
    print(f"  Files Deleted      : {data.get('files_deleted', 0)}")
    print(f"  Databases Vacuumed : {data.get('databases_vacuumed', 0)}")
    print(f"  Protected Session  : {data.get('protected_active_id') or 'None'}")
    return 0


def run_cache_analyze_prompts(args: argparse.Namespace) -> int:
    """Analyze prompt context bloat and display recommendations."""
    config = SwissKnifeConfig.load()
    controller = create_controller(config, prefer_daemon=True)
    try:
        data = controller.analyze_prompt_cache(
            conversation_id=args.conversation_id,
            transcript_path=args.transcript_path,
        )
    except Exception as exc:
        print(f"[ERROR] Failed to analyze prompt cache: {exc}", file=sys.stderr)
        return 1

    if args.json:
        print(json.dumps(data, indent=2))
        return 0

    print("═" * 66)
    print("            PROMPT CACHE & CONTEXT BLOAT ANALYSIS")
    print("═" * 66)
    print(f"Conversation ID      : {data.get('conversation_id', 'Unknown')}")
    print(f"Total Steps          : {data.get('total_steps', 0)}")
    print(f"Model Turns          : {data.get('turn_count', 0)}")
    print(f"Cumulative Tokens    : {data.get('total_prompt_tokens', 0):,}")
    print(f"Redundant Tokens     : {data.get('estimated_redundant_tokens', 0):,}")
    print(f"Potential Savings    : {data.get('potential_savings_percent', 0)}%")
    print(f"Oversized Outputs    : {data.get('oversized_tool_outputs_count', 0)}")
    print("─" * 66)
    print("Recommendations:")
    for rec in data.get("optimization_recommendations", []):
        print(f"  • {rec}")
    print("═" * 66)
    return 0


def run_fingerprint_status(args: argparse.Namespace) -> int:
    """Display active hardware fingerprint profile."""
    config = SwissKnifeConfig.load()
    controller = create_controller(config, prefer_daemon=True)
    try:
        data = controller.get_fingerprint_profile()
    except Exception as exc:
        print(f"[ERROR] Failed to fetch fingerprint status: {exc}", file=sys.stderr)
        return 1

    if args.json:
        print(json.dumps(data, indent=2))
        return 0

    print("═" * 66)
    print("             ACTIVE DEVICE FINGERPRINT PROFILE")
    print("═" * 66)
    print(f"Account Email       : {data.get('account_email') or 'Host Default'}")
    print(f"Machine ID          : {data.get('machine_id')}")
    print(f"Updater ID          : {data.get('updater_id')}")
    print(f"Installation ID     : {data.get('installation_id')}")
    print(f"Installation UUID   : {data.get('installation_uuid')}")
    print(f"Is Active           : {data.get('is_active', True)}")
    print("═" * 66)
    return 0


def run_fingerprint_list(args: argparse.Namespace) -> int:
    """List registered device profiles."""
    config = SwissKnifeConfig.load()
    controller = create_controller(config, prefer_daemon=True)
    try:
        data = controller.list_fingerprint_profiles()
    except Exception as exc:
        print(f"[ERROR] Failed to list fingerprint profiles: {exc}", file=sys.stderr)
        return 1

    if args.json:
        print(json.dumps(data, indent=2))
        return 0

    print("═" * 66)
    print("             REGISTERED DEVICE FINGERPRINT PROFILES")
    print("═" * 66)
    if not data:
        print("No device profiles registered in store.")
    else:
        for email, prof in data.items():
            print(f"• {email}:")
            print(f"    Machine ID: {prof.get('machine_id')}")
            print(f"    Updater ID: {prof.get('updater_id')}")
    print("═" * 66)
    return 0


def run_fingerprint_swap(args: argparse.Namespace) -> int:
    """Swap hardware fingerprint profile for target account."""
    config = SwissKnifeConfig.load()
    controller = create_controller(config, prefer_daemon=True)
    try:
        data = controller.swap_fingerprint(args.email)
    except Exception as exc:
        print(f"[ERROR] Failed to swap fingerprint: {exc}", file=sys.stderr)
        return 1

    if args.json:
        print(json.dumps(data, indent=2))
        return 0

    print(f"[SUCCESS] Swapped hardware fingerprint for {data.get('account_email')}:")
    prof = data.get("profile", {})
    print(f"  Machine ID        : {prof.get('machine_id')}")
    print(f"  Updater ID        : {prof.get('updater_id')}")
    print(f"  Installation ID   : {prof.get('installation_id')}")
    print(f"  Installation UUID : {prof.get('installation_uuid')}")
    return 0


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

    # cache
    p_cache = subparsers.add_parser("cache", help="Manage storage and prompt token caches")
    c_sub = p_cache.add_subparsers(dest="cache_command", required=True)

    p_c_break = c_sub.add_parser("breakdown", help="Inspect categorized disk usage")
    p_c_break.add_argument("--json", action="store_true", help="Output raw JSON")
    p_c_break.set_defaults(func=run_cache_breakdown)

    p_c_prune = c_sub.add_parser("prune", help="Clean stale scratchpads, logs, and artifacts")
    p_c_prune.add_argument("--min-age-days", type=float, default=3.0, help="Minimum age in days (default: 3.0)")
    p_c_prune.add_argument("--dry-run", action="store_true", help="Simulate prune without deleting files")
    p_c_prune.add_argument("--no-scratch", action="store_true", help="Do not prune scratchpad directories")
    p_c_prune.add_argument("--no-steps", action="store_true", help="Do not prune step log outputs")
    p_c_prune.add_argument("--no-tasks", action="store_true", help="Do not prune background task logs")
    p_c_prune.add_argument("--json", action="store_true", help="Output raw JSON")
    p_c_prune.set_defaults(func=run_cache_prune)

    p_c_prompt = c_sub.add_parser("analyze-prompts", help="Analyze conversation prompt token bloat")
    p_c_prompt.add_argument("--conversation-id", type=str, help="Target conversation ID")
    p_c_prompt.add_argument("--transcript-path", type=str, help="Direct path to transcript.jsonl")
    p_c_prompt.add_argument("--json", action="store_true", help="Output raw JSON")
    p_c_prompt.set_defaults(func=run_cache_analyze_prompts)

    # fingerprint
    p_fp = subparsers.add_parser("fingerprint", help="Manage virtual hardware identity profiles")
    fp_sub = p_fp.add_subparsers(dest="fingerprint_command", required=True)

    p_fp_status = fp_sub.add_parser("status", help="Query active device fingerprint profile")
    p_fp_status.add_argument("--json", action="store_true", help="Output raw JSON")
    p_fp_status.set_defaults(func=run_fingerprint_status)

    p_fp_list = fp_sub.add_parser("list", help="List configured device profiles")
    p_fp_list.add_argument("--json", action="store_true", help="Output raw JSON")
    p_fp_list.set_defaults(func=run_fingerprint_list)

    p_fp_swap = fp_sub.add_parser("swap", help="Swap device profile for target account")
    p_fp_swap.add_argument("email", type=str, help="Target account email")
    p_fp_swap.add_argument("--json", action="store_true", help="Output raw JSON")
    p_fp_swap.set_defaults(func=run_fingerprint_swap)

    args = parser.parse_args()
    return args.func(args)



if __name__ == "__main__":
    sys.exit(main())
