"""
Auto-Switch Rule Engine & Account Eligibility Selector (Feature F09).
======================================================================
Evaluates model quota thresholds, sorts accounts by tiered model health,
and triggers atomic keyring switching with anti-thrashing cooldown guardrails.
"""

from __future__ import annotations

import collections
from dataclasses import dataclass, field
import datetime
import logging
import time
from typing import Any, Callable, Dict, List, Optional, Tuple

from antigravity_swiss.core.constants import (
    DEFAULT_AUTO_SWITCH_THRESHOLD_FRACTION,
    DEFAULT_AUTO_SWITCH_WEEKLY_THRESHOLD_FRACTION,
)
from antigravity_swiss.core.errors import SwissKnifeError
from antigravity_swiss.keyring.switcher import AccountRecord, AccountVault, KeyringService

logger = logging.getLogger("antigravity_swiss.quota.rule_engine")


@dataclass
class RuleEngineConfig:
    """Configuration for quota auto-switching and anti-thrashing guardrails."""
    enabled: bool = True
    default_threshold: float = DEFAULT_AUTO_SWITCH_THRESHOLD_FRACTION  # 0.05 (5%)
    per_model_thresholds: Dict[str, float] = field(default_factory=lambda: {
        "gemini-3.8-flash": 0.05,
        "gemini-3.5-flash-lite": 0.05,
        "gemini-3.1-pro": 0.15,
        "claude-sonnet-4-6": 0.10,
    })
    cooldown_seconds: float = 300.0           # 5 minutes minimum between switches to same account
    switch_margin: float = 0.05              # Target must have >= (threshold + margin)
    max_switches_in_window: int = 3          # Maximum switches before rate limiting
    switch_window_seconds: float = 600.0      # 10 minutes rolling rate limit window
    weekly_threshold: float = DEFAULT_AUTO_SWITCH_WEEKLY_THRESHOLD_FRACTION  # 0.05 (5%)
    tier_weights: Dict[str, float] = field(default_factory=lambda: {
        "flash": 0.40,
        "pro": 0.30,
        "claude": 0.20,
        "flash_lite": 0.10,
    })

    def get_threshold_for_model(self, model_id: str) -> float:
        """Resolve threshold for model ID with fuzzy prefix matching."""
        if not model_id:
            return max(0.0, min(1.0, self.default_threshold))
        for key, val in self.per_model_thresholds.items():
            if key in model_id:
                return max(0.0, min(1.0, val))
        return max(0.0, min(1.0, self.default_threshold))


@dataclass
class EvaluationResult:
    """Outcome of rule engine threshold evaluation."""
    should_switch: bool
    reason: str
    current_account: Optional[str]
    target_account: Optional[str] = None
    current_fraction: float = 1.0
    threshold: float = 0.05
    target_score: float = 0.0
    all_exhausted: bool = False
    cooldown_active: bool = False
    details: Dict[str, Any] = field(default_factory=dict)


class AutoSwitchRuleEngine:
    """
    Evaluates account quotas against configured thresholds and selects the best
    standby account using multi-model tiered scoring and cooldown guardrails.
    """

    def __init__(
        self,
        vault: AccountVault,
        keyring_service: KeyringService,
        config: Optional[RuleEngineConfig] = None,
        clock_fn: Callable[[], float] = time.time,
    ) -> None:
        self.vault = vault
        self.keyring_service = keyring_service
        self.config = config or RuleEngineConfig()
        self._clock = clock_fn
        self._last_switch_out_times: Dict[str, float] = {}  # account_email -> timestamp
        self._switch_history: collections.deque[float] = collections.deque()
        self._account_quota_cache: Dict[str, Any] = {}

    def update_cached_quota(self, email: str, quota_summary: Any) -> None:
        """Store latest quota summary for an account."""
        self._account_quota_cache[email] = quota_summary

    def is_account_in_cooldown(self, email: str) -> bool:
        """Check if an account was recently switched away from."""
        last_out = self._last_switch_out_times.get(email)
        if last_out is None:
            return False
        return (self._clock() - last_out) < self.config.cooldown_seconds

    def is_rate_limited(self) -> bool:
        """Check if global switch rate limit is exceeded."""
        now = self._clock()
        while self._switch_history and (now - self._switch_history[0]) > self.config.switch_window_seconds:
            self._switch_history.popleft()
        return len(self._switch_history) >= self.config.max_switches_in_window

    def record_switch(self, from_email: str, to_email: str) -> None:
        """Record switch event for cooldown and rate limiting."""
        now = self._clock()
        if from_email:
            self._last_switch_out_times[from_email] = now
        self._switch_history.append(now)

    def extract_remaining_fractions(self, quota_data: Any) -> Dict[str, float]:
        """
        Extract normalized model and bucket fractions from QuotaSummary or dict.
        Returns dict with keys: 'flash', 'pro', 'claude', 'flash_lite', 'weekly_gemini', 'weekly_3p'.
        Clamps all fractions into [0.0, 1.0].
        """
        fractions = {
            "flash": 1.0,
            "pro": 1.0,
            "claude": 1.0,
            "flash_lite": 1.0,
            "weekly_gemini": 1.0,
            "weekly_3p": 1.0,
        }
        if quota_data is None:
            return fractions

        # If quota_data is a float or int directly
        if isinstance(quota_data, (float, int)):
            clamped = max(0.0, min(1.0, float(quota_data)))
            return {k: clamped for k in fractions}

        # Handle QuotaSummary object or dict
        groups = getattr(quota_data, "groups", None)
        if groups is None and isinstance(quota_data, dict):
            groups = quota_data.get("groups", [])

        if isinstance(groups, list):
            for group in groups:
                buckets = group.get("buckets", []) if isinstance(group, dict) else getattr(group, "buckets", [])
                for b in buckets:
                    b_id = b.get("bucketId", "") if isinstance(b, dict) else getattr(b, "bucket_id", "")
                    raw_rem = b.get("remainingFraction", 1.0) if isinstance(b, dict) else getattr(b, "remaining_fraction", 1.0)
                    try:
                        rem = max(0.0, min(1.0, float(raw_rem)))
                    except (ValueError, TypeError):
                        rem = 1.0

                    if b_id == "gemini-5h":
                        fractions["flash"] = rem
                        fractions["pro"] = rem
                        fractions["flash_lite"] = rem
                    elif b_id == "gemini-weekly":
                        fractions["weekly_gemini"] = rem
                    elif b_id == "3p-5h":
                        fractions["claude"] = rem
                    elif b_id == "3p-weekly":
                        fractions["weekly_3p"] = rem

        return fractions

    def calculate_account_score(self, email: str, quota_data: Any) -> float:
        """
        Compute weighted multi-model score for an account:
        Score = W_flash*Q_flash + W_pro*Q_pro + W_claude*Q_claude + W_lite*Q_lite
        Penalizes heavily (returns 0.0) if weekly quota is depleted.
        """
        fractions = self.extract_remaining_fractions(quota_data)
        
        # If weekly quota is exhausted, account score drops to 0.0
        if fractions["weekly_gemini"] <= self.config.weekly_threshold or fractions["weekly_3p"] <= self.config.weekly_threshold:
            return 0.0

        weights = self.config.tier_weights
        score = (
            weights.get("flash", 0.4) * fractions["flash"]
            + weights.get("pro", 0.3) * fractions["pro"]
            + weights.get("claude", 0.2) * fractions["claude"]
            + weights.get("flash_lite", 0.1) * fractions["flash_lite"]
        )
        return round(score, 4)

    def select_best_standby_account(
        self,
        current_email: str,
        threshold: float | None = None,
        weekly_threshold: float | None = None,
    ) -> Tuple[Optional[str], float, bool]:
        """
        Select highest scoring standby account meeting threshold and margin.
        Returns: (best_email, best_score, all_exhausted)
        """
        active_thresh = self.config.default_threshold if threshold is None else threshold
        active_weekly_thresh = self.config.weekly_threshold if weekly_threshold is None else weekly_threshold
        records = self.vault.list_account_records()
        candidates: List[Tuple[str, float, AccountRecord]] = []

        for rec in records:
            if rec.email == current_email:
                continue
            if not rec.is_healthy:
                continue
            if self.is_account_in_cooldown(rec.email):
                continue

            q_data = self._account_quota_cache.get(rec.email)
            fractions = self.extract_remaining_fractions(q_data)
            burst_remaining = fractions.get("flash", 1.0)
            weekly_remaining = fractions.get("weekly_gemini", 1.0)

            # Eligibility requirements:
            # 1. Burst remaining must exceed threshold + switch_margin
            # 2. Weekly remaining must exceed weekly_threshold
            required_min = active_thresh + self.config.switch_margin
            if burst_remaining <= required_min or weekly_remaining <= active_weekly_thresh:
                continue

            score = self.calculate_account_score(rec.email, q_data)
            candidates.append((rec.email, score, rec))

        if not candidates:
            return None, 0.0, True

        def _sort_key(item: Tuple[str, float, AccountRecord]) -> Tuple[int, float, float, str]:
            email, score, rec = item
            q_data = self._account_quota_cache.get(email)
            weekly = self.extract_remaining_fractions(q_data)["weekly_gemini"]
            last_used = rec.last_used_at or ""
            prio_map = {"HIGH": 0, "MID": 1, "LOW": 2}
            prio_rank = prio_map.get((getattr(rec, "priority", "High") or "High").upper(), 0)
            return (prio_rank, -score, -weekly, last_used)

        candidates.sort(key=_sort_key)
        best_email, best_score, _ = candidates[0]
        return best_email, best_score, False

    def evaluate(
        self,
        current_email: str,
        current_quota_data: Any,
        active_model_id: str = "gemini-3.8-flash-high",
        weekly_threshold: float | None = None,
    ) -> EvaluationResult:
        """
        Main evaluation entry point.
        Checks if active account's quota breaches threshold and finds best successor.
        """
        if not self.config.enabled:
            return EvaluationResult(
                should_switch=False,
                reason="Auto-switch disabled in settings",
                current_account=current_email,
            )

        threshold = self.config.get_threshold_for_model(active_model_id)
        active_weekly_thresh = self.config.weekly_threshold if weekly_threshold is None else weekly_threshold
        fractions = self.extract_remaining_fractions(current_quota_data)
        current_burst = fractions.get("flash", 1.0)
        current_weekly = fractions.get("weekly_gemini", 1.0)

        # Update cache for current account
        self.update_cached_quota(current_email, current_quota_data)

        # Check breach conditions
        burst_breached = current_burst <= threshold
        weekly_breached = current_weekly <= active_weekly_thresh

        if not burst_breached and not weekly_breached:
            return EvaluationResult(
                should_switch=False,
                reason="Quota healthy",
                current_account=current_email,
                current_fraction=current_burst,
                threshold=threshold,
            )

        # Check rate limiting guardrail
        if self.is_rate_limited():
            logger.warning(
                "Auto-switch throttled: exceeded %d switches in %ds",
                self.config.max_switches_in_window,
                int(self.config.switch_window_seconds),
            )
            return EvaluationResult(
                should_switch=False,
                reason="Rate limit exceeded (thrashing protection active)",
                current_account=current_email,
                current_fraction=current_burst,
                threshold=threshold,
                cooldown_active=True,
            )

        # Select best successor
        best_email, best_score, all_exhausted = self.select_best_standby_account(
            current_email, threshold, active_weekly_thresh
        )

        if all_exhausted or not best_email:
            reason = "All standby accounts exhausted or below threshold"
            logger.warning("%s (active account: %s)", reason, current_email)
            return EvaluationResult(
                should_switch=False,
                reason=reason,
                current_account=current_email,
                current_fraction=current_burst,
                threshold=threshold,
                all_exhausted=True,
            )

        trigger_reason = (
            f"Weekly quota ({current_weekly:.1%}) below threshold ({active_weekly_thresh:.1%})"
            if weekly_breached
            else f"Quota ({current_burst:.1%}) below threshold ({threshold:.1%})"
        )
        return EvaluationResult(
            should_switch=True,
            reason=trigger_reason,
            current_account=current_email,
            target_account=best_email,
            current_fraction=current_burst,
            threshold=threshold,
            target_score=best_score,
        )
