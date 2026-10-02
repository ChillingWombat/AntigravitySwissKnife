"""
Background Quota Polling Engine & Model Catalog Manager.
========================================================
Orchestrates CloudCodeClient, KeyringService, and AccountVault.
Implements:
- Proactive & reactive OAuth token refresh
- In-memory caching with TTL
- Exponential backoff with jitter on errors
- Thread-safe pub-sub notifications to daemon & GUI
"""

from __future__ import annotations

import asyncio
import datetime
import logging
import random
import time
from typing import Any, Callable, Dict, Optional

from antigravity_swiss.core.config import SwissKnifeConfig
from antigravity_swiss.core.constants import DEFAULT_POLLING_INTERVAL_SECONDS
from antigravity_swiss.core.errors import (
    KeyringError,
    QuotaAuthExpiredError,
    QuotaError,
)
from antigravity_swiss.keyring.switcher import AccountVault, KeyringCredential, KeyringService
from antigravity_swiss.quota.client import CloudCodeClient
from antigravity_swiss.quota.models import ModelCatalog, QuotaSummary

logger = logging.getLogger("antigravity_swiss.quota.poller")

DEFAULT_CACHE_TTL_SEC = 15.0
DEFAULT_MODEL_REFRESH_INTERVAL_SEC = 900.0  # 15 minutes


class QuotaPoller:
    """
    Periodic background poller fetching live quota fractions and model catalogs.
    """

    def __init__(
        self,
        config: Optional[SwissKnifeConfig] = None,
        client: Optional[CloudCodeClient] = None,
        keyring_service: Optional[KeyringService] = None,
        vault: Optional[AccountVault] = None,
        poll_interval_sec: Optional[float] = None,
        cache_ttl_sec: float = DEFAULT_CACHE_TTL_SEC,
        model_refresh_interval_sec: float = DEFAULT_MODEL_REFRESH_INTERVAL_SEC,
        on_quota_updated: Optional[Callable[[QuotaSummary], Any]] = None,
    ) -> None:
        self.config = config or SwissKnifeConfig.load()
        self.client = client or CloudCodeClient()
        self.vault = vault or AccountVault(config_path=self.config.accounts_file)
        self.keyring_service = keyring_service or KeyringService(vault=self.vault)
        self.poll_interval_sec = (
            poll_interval_sec
            if poll_interval_sec is not None
            else (self.config.poll_interval_sec or DEFAULT_POLLING_INTERVAL_SECONDS)
        )
        self.cache_ttl_sec = cache_ttl_sec
        self.model_refresh_interval_sec = model_refresh_interval_sec
        self.on_quota_updated = on_quota_updated

        # Poller state
        self._is_running: bool = False
        self._task: Optional[asyncio.Task[None]] = None
        self._lock = asyncio.Lock()
        self._cached_summary: Optional[QuotaSummary] = None
        self._cached_summary_time: float = 0.0
        self._account_cache: Dict[str, tuple[QuotaSummary, float]] = {}
        self._cached_catalog: Optional[ModelCatalog] = None
        self._cached_catalog_time: float = 0.0
        self._consecutive_errors: int = 0
        self._last_error: Optional[Exception] = None

    @property
    def is_running(self) -> bool:
        return self._is_running

    @property
    def cached_summary(self) -> Optional[QuotaSummary]:
        return self._cached_summary

    @property
    def cached_catalog(self) -> Optional[ModelCatalog]:
        return self._cached_catalog

    @property
    def active_account_email(self) -> Optional[str]:
        return self.vault.get_active_account()

    def get_cached_summary(self, email: Optional[str] = None) -> Optional[QuotaSummary]:
        """Retrieve in-memory cached quota summary if still valid under TTL."""
        now_mono = time.monotonic()
        if email:
            entry = self._account_cache.get(email)
            if entry and (now_mono - entry[1] < self.cache_ttl_sec):
                return entry[0]
            return None
        if self._cached_summary and (now_mono - self._cached_summary_time < self.cache_ttl_sec):
            return self._cached_summary
        return None

    async def start(self) -> None:
        """Start background polling task."""
        if self._is_running:
            return
        self._is_running = True
        self._task = asyncio.create_task(self._poll_loop())
        logger.info("Quota poller background task started (interval=%.1fs)", self.poll_interval_sec)

    async def stop(self) -> None:
        """Stop background polling task cleanly."""
        if not self._is_running:
            return
        self._is_running = False
        if self._task and not self._task.done():
            self._task.cancel()
            try:
                await self._task
            except asyncio.CancelledError:
                pass
        self._task = None
        logger.info("Quota poller stopped cleanly.")

    async def _poll_loop(self) -> None:
        """Main periodic polling loop with exponential backoff and jitter."""
        while self._is_running:
            sleep_sec = self.poll_interval_sec
            try:
                await self.poll_summary(force=True)
                self._consecutive_errors = 0
                self._last_error = None
                jitter = random.uniform(-1.5, 1.5)
                sleep_sec = max(5.0, self.poll_interval_sec + jitter)
            except asyncio.CancelledError:
                break
            except Exception as exc:
                self._consecutive_errors += 1
                self._last_error = exc
                backoff = min(60.0, 2.0 * (1.5 ** min(self._consecutive_errors, 6)))
                jitter = random.uniform(0.2, 1.5)
                sleep_sec = backoff + jitter
                logger.warning(
                    "Quota poller error (failure #%d): %s; backing off for %.1fs",
                    self._consecutive_errors,
                    exc,
                    sleep_sec,
                )

            try:
                await asyncio.sleep(sleep_sec)
            except asyncio.CancelledError:
                break

    async def _ensure_valid_token(
        self, cred: KeyringCredential, account_email: Optional[str] = None
    ) -> KeyringCredential:
        """
        Inspect token expiration. If expired or expiring within 120s, refresh token proactively.
        """
        now = datetime.datetime.now(datetime.timezone.utc)
        needs_refresh = False

        if not cred.access_token:
            needs_refresh = True
        elif cred.expiry:
            try:
                exp_dt = datetime.datetime.fromisoformat(cred.expiry.replace("Z", "+00:00"))
                if exp_dt.tzinfo is None:
                    exp_dt = exp_dt.replace(tzinfo=datetime.timezone.utc)
                if (exp_dt - now).total_seconds() < 120.0:
                    needs_refresh = True
            except Exception:
                pass

        if not needs_refresh:
            return cred

        if not cred.refresh_token:
            logger.warning("Token expired but no refresh_token present for %s", account_email)
            return cred

        logger.info("Proactively refreshing access token for %s", account_email or "active account")
        new_token_data = await self.client.refresh_access_token(cred.refresh_token)
        new_access_token = new_token_data["access_token"]
        expires_in = int(new_token_data.get("expires_in", 3600))
        new_expiry = (now + datetime.timedelta(seconds=expires_in)).isoformat()

        updated_cred = KeyringCredential(
            access_token=new_access_token,
            refresh_token=cred.refresh_token,
            token_type=new_token_data.get("token_type", "Bearer"),
            expiry=new_expiry,
            auth_method=cred.auth_method,
            id_token=cred.id_token,
        )

        active_email = self.vault.get_active_account()
        if account_email is None or account_email == active_email:
            self.keyring_service.set_active_credential(updated_cred)

        if account_email:
            rec = self.vault.get_account(account_email)
            self.vault.add_or_update_account(
                email=account_email,
                credential=updated_cred,
                label=rec.label if rec else "",
            )

        return updated_cred

    async def poll_account(self, email: Optional[str] = None, force: bool = True) -> QuotaSummary:
        """Alias for poll_summary with explicit account target."""
        return await self.poll_summary(force=force, email=email)

    async def poll_summary(self, force: bool = False, email: Optional[str] = None) -> QuotaSummary:
        """
        Fetch quota summary for the specified account (or active account).
        Uses cache if within cache_ttl_sec and force is False.
        """
        async with self._lock:
            now_mono = time.monotonic()
            if not force:
                if email:
                    entry = self._account_cache.get(email)
                    if entry and (now_mono - entry[1] < self.cache_ttl_sec):
                        return entry[0]
                elif self._cached_summary and (now_mono - self._cached_summary_time < self.cache_ttl_sec):
                    return self._cached_summary

            # 1. Resolve credential
            if email:
                rec = self.vault.get_account(email)
                if not rec:
                    raise KeyringError(f"Account '{email}' not found in vault")
                cred = rec.credential
                target_email = email
            else:
                cred = self.keyring_service.get_active_credential()
                target_email = self.vault.get_active_account() or cred.extract_email_from_id_token() or "active_account"

            # 2. Check token freshness
            cred = await self._ensure_valid_token(cred, target_email)

            # 3. Query CloudCode API with reactive 401 retry
            try:
                raw_data, drift = await self.client.retrieve_user_quota_summary(cred.access_token)
            except QuotaAuthExpiredError:
                if cred.refresh_token:
                    logger.info("HTTP 401 received; executing reactive token refresh")
                    cred = await self._ensure_valid_token(
                        KeyringCredential(
                            access_token="",
                            refresh_token=cred.refresh_token,
                            token_type=cred.token_type,
                            expiry="",
                            auth_method=cred.auth_method,
                            id_token=cred.id_token,
                        ),
                        target_email,
                    )
                    raw_data, drift = await self.client.retrieve_user_quota_summary(cred.access_token)
                else:
                    raise

            # 4. Parse model structure
            summary = QuotaSummary.from_dict(raw_data)
            summary.active_account = target_email
            summary.server_time_drift_seconds = drift

            # 5. Cache result
            self._account_cache[target_email] = (summary, now_mono)
            if not email or email == self.vault.get_active_account():
                self._cached_summary = summary
                self._cached_summary_time = now_mono

            # 6. Notify callbacks
            if self.on_quota_updated and (not email or email == self.vault.get_active_account()):
                try:
                    res = self.on_quota_updated(summary)
                    if asyncio.iscoroutine(res):
                        await res
                except Exception as exc:
                    logger.warning("Error in on_quota_updated callback: %s", exc)

            return summary

    async def fetch_models(self, force: bool = False, email: Optional[str] = None) -> ModelCatalog:
        """
        Fetch available models and capabilities.
        Uses cache if within model_refresh_interval_sec and force is False.
        """
        async with self._lock:
            now_mono = time.monotonic()
            if not force and not email and self._cached_catalog and (now_mono - self._cached_catalog_time < self.model_refresh_interval_sec):
                return self._cached_catalog

            if email:
                rec = self.vault.get_account(email)
                if not rec:
                    raise KeyringError(f"Account '{email}' not found in vault")
                cred = rec.credential
                target_email = email
            else:
                cred = self.keyring_service.get_active_credential()
                target_email = self.vault.get_active_account()

            cred = await self._ensure_valid_token(cred, target_email)

            try:
                raw_data, _ = await self.client.fetch_available_models(cred.access_token)
            except QuotaAuthExpiredError:
                if cred.refresh_token:
                    cred = await self._ensure_valid_token(
                        KeyringCredential(
                            access_token="",
                            refresh_token=cred.refresh_token,
                            token_type=cred.token_type,
                            expiry="",
                            auth_method=cred.auth_method,
                            id_token=cred.id_token,
                        ),
                        target_email,
                    )
                    raw_data, _ = await self.client.fetch_available_models(cred.access_token)
                else:
                    raise

            catalog = ModelCatalog.from_dict(raw_data)
            if not email or email == self.vault.get_active_account():
                self._cached_catalog = catalog
                self._cached_catalog_time = now_mono

            return catalog
