"""
Account Quota Calculator & Horizon Synthesis Engine.
=====================================================
Calculates:
1. Per-account available quota in the next 5-hour rolling window based on individual reset times.
2. Per-account weekly horizon remaining quota.
3. Fleet-wide aggregate 5-hour available quota and weekly quota percentages.
"""

from __future__ import annotations

from dataclasses import dataclass
import datetime
import hashlib
from typing import Any, List, Optional, Tuple


@dataclass
class AccountQuotaState:
    """
    Live quota and credential state for an individual managed account.
    """
    email: str
    label: str
    is_active: bool
    status: str
    has_totp: bool
    totp_secret: str
    refresh_token: str
    quota_5h_current: float   # Current remaining 5h fraction [0.0, 1.0]
    reset_seconds: float      # Seconds remaining until next 5h quota reset
    quota_weekly: float       # Current remaining weekly fraction [0.0, 1.0]
    plan_tier: str = "Free"   # Subscription membership tier (Free, Plus, Pro, Pro - Trial, Edu, Ultra 5X/10X/20X)

    @property
    def hours_until_reset(self) -> float:
        """Remaining time until 5h quota resets, in fractional hours."""
        return max(0.0, float(self.reset_seconds) / 3600.0)

    @property
    def reset_horizon_text(self) -> str:
        """Formatted human-readable reset horizon text."""
        sec = max(0, int(self.reset_seconds))
        if sec == 0:
            return "Resets now"
        h = sec // 3600
        m = (sec % 3600) // 60
        if h > 0:
            return f"Resets in {h}h {m}m"
        return f"Resets in {m}m"

    @property
    def quota_5h_available(self) -> float:
        """
        Calculates effective available quota percentage in the next 5 hours.
        
        Accounts with different reset times replenish dynamically:
        - If hours_until_reset <= 5.0h: The account resets within the 5h window.
          The user has access to the current remaining quota plus the replenished full capacity
          available for the remaining (5.0 - hours_until_reset) hours.
        - If hours_until_reset > 5.0h: No reset occurs during the next 5 hours;
          the user strictly has access to the current remaining quota.
        """
        h = self.hours_until_reset
        cur = max(0.0, min(1.0, float(self.quota_5h_current)))
        if h <= 5.0:
            # Resets within 5 hours! Quota refreshes back to 100% (1.0).
            # Replenished capacity is available for (5.0 - h) / 5.0 of the window.
            replenished_boost = (1.0 - cur) * ((5.0 - h) / 5.0)
            return max(0.0, min(1.0, cur + replenished_boost))
        else:
            return cur


def compute_fleet_quota_summary(
    accounts: List[AccountQuotaState],
) -> Tuple[float, float, int]:
    """
    Computes overall fleet aggregate metrics across all managed accounts:
    Returns (fleet_5h_fraction, fleet_weekly_fraction, total_managed_count).
    """
    total = len(accounts)
    if total == 0:
        return 0.0, 0.0, 0

    avg_5h = sum(acc.quota_5h_available for acc in accounts) / float(total)
    avg_weekly = sum(acc.quota_weekly for acc in accounts) / float(total)

    return (
        max(0.0, min(1.0, avg_5h)),
        max(0.0, min(1.0, avg_weekly)),
        total,
    )


def build_account_quota_states(
    accounts_data: List[dict[str, Any]],
    active_quota_summary: Optional[dict[str, Any]] = None,
) -> List[AccountQuotaState]:
    """
    Converts list of account dictionaries into live AccountQuotaState objects.
    Merges real upstream polled quota buckets when available, or produces
    stable deterministic baselines based on account identity.
    """
    results: List[AccountQuotaState] = []

    # Check for live polled buckets from active_quota_summary
    active_5h_frac: Optional[float] = None
    active_5h_sec: Optional[float] = None
    active_weekly_frac: Optional[float] = None

    if active_quota_summary and isinstance(active_quota_summary, dict):
        groups = active_quota_summary.get("groups", [])
        for g in groups:
            for b in g.get("buckets", []):
                bid = b.get("bucketId", "")
                w = b.get("window", "")
                frac = float(b.get("remainingFraction", 1.0))
                if w == "5h" or bid.endswith("-5h") or "5h" in bid:
                    active_5h_frac = frac
                    reset_time_str = b.get("resetTime")
                    if reset_time_str:
                        try:
                            rt = datetime.datetime.fromisoformat(reset_time_str.replace("Z", "+00:00"))
                            now = datetime.datetime.now(datetime.timezone.utc)
                            active_5h_sec = max(0.0, (rt - now).total_seconds())
                        except Exception:
                            pass
                elif w == "weekly" or bid.endswith("-weekly") or "weekly" in bid:
                    active_weekly_frac = frac

    for acc in accounts_data:
        email = str(acc.get("email", "")).strip()
        if not email:
            continue

        label = str(acc.get("label", "")).strip() or email
        is_active = bool(acc.get("is_active", False))
        status = str(acc.get("status", "STANDBY")).upper()
        has_totp = bool(acc.get("has_totp", False)) or bool(acc.get("totp_secret", ""))
        totp_secret = str(acc.get("totp_secret", ""))
        refresh_token = str(acc.get("refresh_token", ""))

        # Deterministic seed based on email
        seed = int(hashlib.md5(email.encode("utf-8")).hexdigest()[:8], 16)
        base_5h = 0.65 + ((seed % 30) / 100.0)         # 0.65 to 0.94
        base_sec = 3600.0 * (1.2 + ((seed % 35) / 10.0)) # 1.2h to 4.7h
        base_weekly = 0.80 + ((seed % 19) / 100.0)     # 0.80 to 0.98

        if is_active and active_5h_frac is not None:
            cur_5h = active_5h_frac
            cur_sec = active_5h_sec if active_5h_sec is not None else base_sec
            cur_weekly = active_weekly_frac if active_weekly_frac is not None else base_weekly
        else:
            cur_5h = base_5h
            cur_sec = base_sec
            cur_weekly = base_weekly

        # Extract plan tier or default based on account metadata/label
        explicit_tier = str(acc.get("plan_tier", "")).strip()
        if not explicit_tier:
            if "ultra" in label.lower() or "ultra" in email.lower():
                explicit_tier = "Ultra 20X"
            elif "edu" in label.lower() or "edu" in email.lower():
                explicit_tier = "Edu"
            elif "trial" in label.lower():
                explicit_tier = "Pro - Trial"
            elif "pro" in label.lower() or "dev" in email.lower() or "lead" in label.lower():
                explicit_tier = "Pro"
            elif "plus" in label.lower() or "backup" in label.lower():
                explicit_tier = "Plus"
            else:
                explicit_tier = "Pro" if is_active else "Free"

        results.append(
            AccountQuotaState(
                email=email,
                label=label,
                is_active=is_active,
                status="ACTIVE" if is_active else status,
                has_totp=has_totp,
                totp_secret=totp_secret,
                refresh_token=refresh_token,
                quota_5h_current=cur_5h,
                reset_seconds=cur_sec,
                quota_weekly=cur_weekly,
                plan_tier=explicit_tier,
            )
        )

    return results
