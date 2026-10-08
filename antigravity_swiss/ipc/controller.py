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
    def set_totp_secret(self, email: str, totp_secret: str) -> bool:
        """Store or update TOTP secret for an account."""
        ...

    @abstractmethod
    def update_account(
        self,
        email: str,
        label: str | None = None,
        totp_secret: str | None = None,
        refresh_token: str | None = None,
        set_active: bool = False,
        plan_tier: str | None = None,
        status: str | None = None,
    ) -> dict[str, Any]:
        """Update account metadata and credentials."""
        ...

    @abstractmethod
    def remove_account(self, email: str) -> bool:
        """Remove account from credential vault."""
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

    @abstractmethod
    def get_fingerprint_profile(self) -> dict[str, Any]:
        """Fetch active device fingerprint profile."""
        ...

    @abstractmethod
    def list_fingerprint_profiles(self) -> dict[str, Any]:
        """List registered device fingerprint profiles."""
        ...

    @abstractmethod
    def swap_fingerprint(self, email: str) -> dict[str, Any]:
        """Swap hardware profile for target account."""
        ...

    @abstractmethod
    def get_cache_breakdown(self, active_conversation_id: str | None = None) -> dict[str, Any]:
        """Retrieve categorized cache breakdown and reclaimable estimates."""
        ...

    @abstractmethod
    def prune_cache(self, options: dict[str, Any] | None = None, active_conversation_id: str | None = None) -> dict[str, Any]:
        """Execute cache pruning and return PruneResult dictionary."""
        ...

    @abstractmethod
    def analyze_prompt_cache(self, conversation_id: str | None = None, transcript_path: str | None = None) -> dict[str, Any]:
        """Analyze prompt context bloat and return recommendations."""
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

    def set_totp_secret(self, email: str, totp_secret: str) -> bool:
        try:
            res = self._client.call("accounts.set_totp", {"email": email, "totp_secret": totp_secret})
            return bool(res.get("success", True))
        except Exception:
            return False

    def update_account(
        self,
        email: str,
        label: str | None = None,
        totp_secret: str | None = None,
        refresh_token: str | None = None,
        set_active: bool = False,
        plan_tier: str | None = None,
        status: str | None = None,
        priority: str | None = None,
        notes: str | None = None,
        password: str | None = None,
    ) -> dict[str, Any]:
        try:
            return self._client.call(
                "accounts.update",
                {
                    "email": email,
                    "label": label,
                    "plan_tier": plan_tier,
                    "status": status,
                    "priority": priority,
                    "notes": notes,
                    "password": password,
                    "totp_secret": totp_secret,
                    "refresh_token": refresh_token,
                    "set_active": set_active,
                },
            )
        except Exception:
            from antigravity_swiss.core.config import SwissKnifeConfig
            from antigravity_swiss.keyring.switcher import AccountStore
            store = AccountStore(SwissKnifeConfig.load().accounts_file)
            success = store.update_account_info(
                email=email,
                label=label,
                plan_tier=plan_tier,
                status=status,
                priority=priority,
                notes=notes,
                password=password,
                totp_secret=totp_secret,
                refresh_token=refresh_token,
                set_active=set_active,
            )
            return {"success": success, "email": email}

    def remove_account(self, email: str) -> bool:
        try:
            res = self._client.call("accounts.remove", {"email": email})
            return bool(res.get("success", False))
        except Exception:
            from antigravity_swiss.core.config import SwissKnifeConfig
            from antigravity_swiss.keyring.switcher import AccountStore
            store = AccountStore(SwissKnifeConfig.load().accounts_file)
            return store.remove_account(email)

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

    def get_fingerprint_profile(self) -> dict[str, Any]:
        return self._client.call("fingerprint.get_profile")

    def list_fingerprint_profiles(self) -> dict[str, Any]:
        return self._client.call("fingerprint.list_profiles")

    def swap_fingerprint(self, email: str) -> dict[str, Any]:
        return self._client.call("fingerprint.swap", {"email": email})

    def get_cache_breakdown(self, active_conversation_id: str | None = None) -> dict[str, Any]:
        params = {"active_conversation_id": active_conversation_id} if active_conversation_id else {}
        return self._client.call("cache.get_breakdown", params)

    def prune_cache(self, options: dict[str, Any] | None = None, active_conversation_id: str | None = None) -> dict[str, Any]:
        params = {"options": options or {}, "active_conversation_id": active_conversation_id}
        return self._client.call("cache.prune", params)

    def analyze_prompt_cache(self, conversation_id: str | None = None, transcript_path: str | None = None) -> dict[str, Any]:
        params = {}
        if conversation_id:
            params["conversation_id"] = conversation_id
        if transcript_path:
            params["transcript_path"] = transcript_path
        return self._client.call("cache.analyze_prompts", params)



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

    def set_totp_secret(self, email: str, totp_secret: str) -> bool:
        from antigravity_swiss.keyring.switcher import AccountStore
        store = AccountStore(self.config.accounts_file)
        return store.set_totp_secret(email, totp_secret)

    def update_account(
        self,
        email: str,
        label: str | None = None,
        totp_secret: str | None = None,
        refresh_token: str | None = None,
        set_active: bool = False,
        plan_tier: str | None = None,
        status: str | None = None,
        priority: str | None = None,
        notes: str | None = None,
        password: str | None = None,
    ) -> dict[str, Any]:
        from antigravity_swiss.keyring.switcher import AccountStore
        store = AccountStore(self.config.accounts_file)
        success = store.update_account_info(
            email=email,
            label=label,
            plan_tier=plan_tier,
            status=status,
            priority=priority,
            notes=notes,
            password=password,
            totp_secret=totp_secret,
            refresh_token=refresh_token,
            set_active=set_active,
        )
        if set_active and (status or "").upper() not in ("BANNED", "ERROR"):
            try:
                self.switch_account(email, force=True, relaunch=False)
            except Exception:
                pass
        return {"success": success, "email": email}

    def remove_account(self, email: str) -> bool:
        from antigravity_swiss.keyring.switcher import AccountStore
        store = AccountStore(self.config.accounts_file)
        return store.remove_account(email)

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
            "auto_switch_weekly_threshold": getattr(self.config, "auto_switch_weekly_threshold", 0.05),
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
        if "auto_switch_weekly_threshold" in kwargs:
            self.config.auto_switch_weekly_threshold = max(0.0, min(1.0, float(kwargs["auto_switch_weekly_threshold"])))
        self.config.save_settings()
        return self.get_rule_config()

    def get_fingerprint_profile(self) -> dict[str, Any]:
        from antigravity_swiss.fingerprint.manager import FingerprintManager
        mgr = FingerprintManager(
            config_dir=self.config.antigravity_config_dir,
            data_dir=self.config.antigravity_data_dir,
        )
        return mgr.get_active_profile().to_dict()

    def list_fingerprint_profiles(self) -> dict[str, Any]:
        from antigravity_swiss.fingerprint.manager import FingerprintManager
        mgr = FingerprintManager(
            config_dir=self.config.antigravity_config_dir,
            data_dir=self.config.antigravity_data_dir,
        )
        accounts = mgr.store.list_accounts()
        res = {}
        for acc in accounts:
            p = mgr.store.get_profile(acc)
            if p:
                res[acc] = p.to_dict()
        return res

    def swap_fingerprint(self, email: str) -> dict[str, Any]:
        from antigravity_swiss.fingerprint.manager import FingerprintManager
        mgr = FingerprintManager(
            config_dir=self.config.antigravity_config_dir,
            data_dir=self.config.antigravity_data_dir,
        )
        profile = mgr.swap_profile_for_account(email)
        return {"success": True, "account_email": email, "profile": profile.to_dict()}

    def get_cache_breakdown(self, active_conversation_id: str | None = None) -> dict[str, Any]:
        from antigravity_swiss.cache_optimizer.inspector import BrainCacheInspector
        inspector = BrainCacheInspector(
            data_dir=self.config.antigravity_data_dir,
            config_dir=self.config.antigravity_config_dir,
        )
        return inspector.scan_breakdown(active_conversation_id=active_conversation_id).to_dict()

    def prune_cache(self, options: dict[str, Any] | None = None, active_conversation_id: str | None = None) -> dict[str, Any]:
        from antigravity_swiss.cache_optimizer.models import PruneOptions
        from antigravity_swiss.cache_optimizer.pruner import BrainCachePruner
        pruner = BrainCachePruner(
            data_dir=self.config.antigravity_data_dir,
            config_dir=self.config.antigravity_config_dir,
        )
        opts_dict = options or {}
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
        return pruner.prune(options=prune_opts, active_conversation_id=active_conversation_id).to_dict()



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
