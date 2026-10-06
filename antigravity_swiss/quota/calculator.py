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
    priority: str = "High"
    notes: str = ""
    password: str = ""
    reset_seconds_weekly: float = 0.0

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
        gemini_groups = [g for g in groups if "gemini" in str(g.get("displayName", "")).lower()]
        target_groups = gemini_groups if gemini_groups else [g for g in groups if not any(x in str(g.get("displayName", "")).lower() for x in ("claude", "gpt", "3p"))]
        for g in target_groups:
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

        prio = str(acc.get("priority", "High") or "High").strip().capitalize()
        if prio not in ("High", "Mid", "Low"):
            prio = "High"
        notes = str(acc.get("notes", "") or "")
        password = str(acc.get("password", "") or "")

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
                priority=prio,
                notes=notes,
                password=password,
            )
        )

    return results


def classify_error_status(status_code: int = 0, error_code: str = "", error_msg: str = "") -> str:
    """Classifies an API error response or error code into 'BANNED', 'ERROR', or 'STANDBY'."""
    combined = f"{error_code} {error_msg}".lower()
    if any(k in combined for k in ("suspend", "banned", "disabled", "terminated", "violates", "account_disabled", "user_suspended")):
        return "BANNED"
    if status_code in (401, 403) or any(k in combined for k in ("invalid_grant", "invalid_token", "unauthenticated", "interaction_required", "challenge", "verification", "reauth", "expired")):
        return "ERROR"
    return "ERROR"


def normalize_plan_tier(tier: str) -> str:
    """Normalizes raw plan tier strings to canonical representations."""
    if not tier:
        return "Free"
    t = tier.strip().lower()
    if t in ("free", "free-tier", "tier_free"):
        return "Free"
    if "trial" in t:
        return "Pro - Trial"
    if "20x" in t or "ultra_20x" in t or "ultra 20x" in t:
        return "Ultra 20X"
    if "10x" in t or "ultra_10x" in t or "ultra 10x" in t:
        return "Ultra 10X"
    if "5x" in t or "ultra_5x" in t or "ultra 5x" in t:
        return "Ultra 5X"
    if "ultra" in t:
        return "Ultra 20X"
    if any(k in t for k in ("edu", "education", "student", "academic")):
        return "Edu"
    if any(k in t for k in ("enterprise", "teams_tier_enterprise")):
        return "Enterprise"
    if "plus" in t:
        return "Plus"
    if any(k in t for k in ("pro", "standard", "code assist", "ai premium", "g1_ai", "team")):
        return "Pro"
    return tier.strip()


def plan_tier_rank(tier: str) -> int:
    """Numeric ordering of plan tiers (Enterprise=8 down to Free=0)."""
    norm = normalize_plan_tier(tier)
    ranks = {
        "Enterprise": 8,
        "Ultra 20X": 7,
        "Ultra 10X": 6,
        "Ultra 5X": 5,
        "Pro": 4,
        "Edu": 3,
        "Pro - Trial": 2,
        "Plus": 1,
        "Free": 0,
    }
    return ranks.get(norm, 0)


def plan_tier_capacity_multiplier(tier: str) -> float:
    """Capacity multiplier relative to Pro (1.0). Free is heavily penalized to 0.25."""
    norm = normalize_plan_tier(tier)
    mults = {
        "Enterprise": 1.45,
        "Ultra 20X": 1.40,
        "Ultra 10X": 1.30,
        "Ultra 5X": 1.20,
        "Pro": 1.00,
        "Edu": 0.98,
        "Pro - Trial": 0.92,
        "Plus": 0.80,
        "Free": 0.25,
    }
    return mults.get(norm, 1.0)


def is_free_plan_tier(email: str, tier: str) -> bool:
    """Determines whether an account belongs to the Free tier."""
    norm = normalize_plan_tier(tier)
    if tier and tier.strip():
        return norm == "Free"
    lower = email.lower()
    if any(k in lower for k in ("ultra", ".edu", "student", "trial", "pro", "dev", "plus")):
        return False
    return True


def sort_account_quota_states(
    accounts: List[AccountQuotaState],
    active_email: str = "",
    threshold: float = 0.10,
    mode: str = "auto",
    switch_mode: str = "balanced",
) -> List[AccountQuotaState]:
    """
    Sorts a list of AccountQuotaState objects according to the specified mode:
    - 'auto': Active account in Row 1. Healthy Paid Standby accounts in Row 2+ ordered by switch_mode,
      followed by Healthy Free Standby accounts, cooling down accounts, error accounts, and banned accounts.
    - 'identity': Alphabetical by friendly label or email.
    - 'quota_5h': Highest 5H quota available first.
    - 'quota_weekly': Highest weekly quota available first.
    """
    items = list(accounts)

    if mode == "identity":
        return sorted(items, key=lambda a: (a.label or a.email).lower())

    if mode == "quota_5h":
        return sorted(items, key=lambda a: (-a.quota_5h_available, -a.quota_weekly, (a.label or a.email).lower()))

    if mode == "quota_weekly":
        return sorted(items, key=lambda a: (-a.quota_weekly, -a.quota_5h_available, (a.label or a.email).lower()))

    # mode == "auto" (Default)
    def _auto_sort_key(a: AccountQuotaState) -> Tuple[int, int, float, float, str]:
        is_act = a.is_active or (active_email and a.email.lower() == active_email.lower())
        st = (a.status or "").upper()
        is_ban = st == "BANNED"
        is_err = st == "ERROR"

        q5h_cur = max(0.0, min(1.0, float(a.quota_5h_current)))
        q5h_avail = a.quota_5h_available
        qwk = max(0.0, min(1.0, float(a.quota_weekly)))
        is_below = q5h_cur <= threshold or qwk <= 0.05
        is_free = is_free_plan_tier(a.email, a.plan_tier)
        tier_mult = plan_tier_capacity_multiplier(a.plan_tier)

        # 6 Structural Tiers:
        # Tier 0: Active account (Row 1 pinned)
        # Tier 1: Healthy Paid Standby successors
        # Tier 2: Healthy Free Standby successors (Free ranked strictly after paid)
        # Tier 3: Cooling down / Below threshold accounts
        # Tier 4: Error accounts
        # Tier 5: Banned accounts
        if is_act:
            tier = 0
        elif is_ban:
            tier = 5
        elif is_err:
            tier = 4
        elif not is_below:
            tier = 2 if is_free else 1
        else:
            tier = 3

        prio_map = {"HIGH": 0, "MID": 1, "LOW": 2}
        prio_rank = prio_map.get((a.priority or "High").upper(), 0)

        sm = (switch_mode or "balanced").lower()
        if sm == "max_continuous":
            score = 0.75 * (tier_mult * q5h_avail) + 0.10 * qwk
        elif sm == "max_tokens":
            sec5h = float(a.reset_seconds or 0)
            soonness = max(0.0, min(1.0, 1.0 - (sec5h / 18000.0))) if 0 < sec5h <= 18000.0 else (0.85 if sec5h <= 0 and q5h_cur >= 0.98 else 0.5)
            urgency = q5h_cur * (0.45 + 0.55 * soonness)
            score = tier_mult * (0.45 * urgency + 0.25 * qwk + 0.15 * q5h_avail)
        else:
            # balanced
            score = tier_mult * (0.42 * q5h_cur + 0.12 * q5h_avail + 0.32 * qwk)

        return (tier, prio_rank, -round(score, 4), -round(q5h_cur, 4), (a.label or a.email).lower())

    return sorted(items, key=_auto_sort_key)



