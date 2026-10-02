"""
Controller Abstraction Providing Unified Interface for Remote Daemon and Standalone In-Process Modes.
====================================================================================================
Dispatches requests over Unix Domain Socket when the background daemon is active,
with transparent, zero-friction fallback to local in-process execution.
"""

from __future__ import annotations

from abc import ABC, abstractmethod
import logging
from pathlib import Path
from typing import Any

from antigravity_swiss.core.config import SwissKnifeConfig
from antigravity_swiss.core.errors import DaemonNotRunningError
from antigravity_swiss.ipc.socket_client import SyncDaemonClient

logger = logging.getLogger("antigravity_swiss.ipc.controller")


class SwissKnifeController(ABC):
    """Abstract facade for Swiss Knife operations."""

    @abstractmethod
    def is_daemon_running(self) -> bool:
        """Check if background daemon is active."""
        ...

    @abstractmethod
    def get_status(self) -> dict[str, Any]:
        """Fetch daemon, Antigravity process, and active account status."""
        ...

    @abstractmethod
    def list_accounts(self) -> list[dict[str, Any]]:
        """List registered accounts in the vault."""
        ...

    @abstractmethod
    def switch_account(self, email: str, force: bool = False, relaunch: bool = True) -> dict[str, Any]:
        """Switch active account and preserve conversation session."""
        ...

    @abstractmethod
    def get_quota_summary(self, account: str | None = None) -> dict[str, Any]:
        """Retrieve quota summary for active account or specified account."""
        ...

    @abstractmethod
    def poll_quota(self, account: str | None = None) -> dict[str, Any]:
        """Trigger an immediate quota poll and return updated summary."""
        ...

    @abstractmethod
    def get_rule_config(self) -> dict[str, Any]:
        """Retrieve rule engine configuration."""
        ...

    @abstractmethod
    def set_rule_config(self, **kwargs: Any) -> dict[str, Any]:
        """Update rule engine configuration."""
        ...


class RemoteDaemonController(SwissKnifeController):
    """Dispatches calls over Unix Domain Socket to running daemon."""

    def __init__(self, socket_path: Path | str, timeout: float = 15.0) -> None:
        self.socket_path = Path(socket_path).expanduser().resolve()
        self._client = SyncDaemonClient(self.socket_path, timeout=timeout)

    def is_daemon_running(self) -> bool:
        return self._client.is_daemon_alive()

    def get_status(self) -> dict[str, Any]:
        return self._client.call("status.get")

    def list_accounts(self) -> list[dict[str, Any]]:
        return self._client.call("accounts.list")

    def switch_account(self, email: str, force: bool = False, relaunch: bool = True) -> dict[str, Any]:
        return self._client.call("accounts.switch", {"email": email, "force": force, "relaunch": relaunch})

    def get_quota_summary(self, account: str | None = None) -> dict[str, Any]:
        params = {"account": account} if account else {}
        return self._client.call("quota.get_summary", params)

    def poll_quota(self, account: str | None = None) -> dict[str, Any]:
        params = {"account": account} if account else {}
        return self._client.call("quota.poll_now", params)

    def get_rule_config(self) -> dict[str, Any]:
        return self._client.call("rules.get_config")

    def set_rule_config(self, **kwargs: Any) -> dict[str, Any]:
        return self._client.call("rules.set_config", kwargs)


class StandaloneController(SwissKnifeController):
    """In-process fallback when daemon is not running (standalone CLI/GUI mode)."""

    def __init__(self, config: SwissKnifeConfig) -> None:
        self.config = config

    def is_daemon_running(self) -> bool:
        return False

    def get_status(self) -> dict[str, Any]:
        from antigravity_swiss.keyring.switcher import AccountVault, KeyringService
        from antigravity_swiss.process.lifecycle import ProcessManager

        pm = ProcessManager(config=self.config)
        pid = pm.get_running_antigravity_pid()

        vault = AccountVault(config_path=self.config.accounts_file)
        active_account = vault.get_active_account()

        if not active_account:
            try:
                service = KeyringService(vault=vault)
                active_account = service.auto_ingest_current_keyring_if_empty()
            except Exception:
                pass

        return {
            "daemon_running": False,
            "mode": "standalone_in_process",
            "antigravity_running": pid is not None,
            "antigravity_pid": pid,
            "active_account": active_account,
        }

    def list_accounts(self) -> list[dict[str, Any]]:
        from antigravity_swiss.keyring.switcher import AccountStore
        store = AccountStore(self.config.accounts_file)
        return store.list_accounts()

    def switch_account(self, email: str, force: bool = False, relaunch: bool = True) -> dict[str, Any]:
        from antigravity_swiss.keyring.switcher import KeyringSwitcher
        from antigravity_swiss.process.lifecycle import ProcessManager

        switcher = KeyringSwitcher(self.config)
        pm = ProcessManager(config=self.config)

        res = switcher.switch_to_account(email, force=force)
        relaunch_pid = None
        if relaunch:
            pm.terminate_gracefully(self.config.process_timeout_sec)
            try:
                relaunch_pid = pm.relaunch()
            except Exception as exc:
                logger.warning("Could not relaunch Antigravity: %s", exc)

        return {
            "success": True,
            "active_account": email,
            "relaunch_pid": relaunch_pid,
            "preserved_cascade_id": res.get("cascade_id"),
        }

    def get_quota_summary(self, account: str | None = None) -> dict[str, Any]:
        # In standalone mode without poller, return summary format
        return {
            "groups": [],
            "timestamp": "2026-10-01T08:00:00Z",
            "message": "Quota poller requires background daemon or network session",
            "activeAccount": account,
        }

    def poll_quota(self, account: str | None = None) -> dict[str, Any]:
        return {
            "account": account,
            "summary": self.get_quota_summary(account),
            "switched": False,
            "active_account": account,
        }

    def get_rule_config(self) -> dict[str, Any]:
        return {
            "auto_switch_enabled": self.config.auto_switch_enabled,
            "auto_switch_threshold": self.config.auto_switch_threshold,
            "cooldown_seconds": 300.0,
            "switch_margin": 0.05,
            "per_model_thresholds": {
                "gemini-3.8-flash": 0.05,
                "gemini-3.5-flash-lite": 0.05,
                "gemini-3.1-pro": 0.15,
                "claude-sonnet-4-6": 0.10,
            },
            "max_switches_in_window": 3,
            "switch_window_seconds": 600.0,
        }

    def set_rule_config(self, **kwargs: Any) -> dict[str, Any]:
        if "auto_switch_enabled" in kwargs:
            self.config.auto_switch_enabled = bool(kwargs["auto_switch_enabled"])
        if "auto_switch_threshold" in kwargs:
            self.config.auto_switch_threshold = max(0.0, min(1.0, float(kwargs["auto_switch_threshold"])))
        self.config.save_settings()
        return self.get_rule_config()


def create_controller(
    config: SwissKnifeConfig | None = None,
    prefer_daemon: bool = True,
    force_remote: bool = False,
) -> SwissKnifeController:
    """Factory creating RemoteDaemonController if daemon is active, or StandaloneController as fallback."""
    cfg = config or SwissKnifeConfig.load()
    remote = RemoteDaemonController(cfg.socket_path, timeout=cfg.client_timeout_sec)

    if prefer_daemon and remote.is_daemon_running():
        return remote

    if force_remote:
        raise DaemonNotRunningError(f"Daemon is not active at {cfg.socket_path}")

    logger.info("Daemon not detected; using in-process StandaloneController")
    return StandaloneController(cfg)
