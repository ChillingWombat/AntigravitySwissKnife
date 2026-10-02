"""
Reset Horizon Tracker & Clock Drift Calibration Subsystem (Feature F08).
========================================================================
Tracks Google CloudCode reset horizons, calculates countdown intervals,
calibrates server clock drift from HTTP Date response headers, and applies
jittered delays before triggering 1-token keep-alive pings.
"""

from __future__ import annotations

from dataclasses import dataclass, field
import datetime
import email.utils
from enum import Enum
import logging
import random
import time
from typing import Any, Mapping, Optional

logger = logging.getLogger("antigravity_swiss.warmup.horizon")


class HorizonStatus(str, Enum):
    """Lifecycle state of an account's quota bucket horizon."""
    IDLE_HEALTHY = "IDLE_HEALTHY"          # Quota > threshold, no reset needed
    COUNTING_DOWN = "COUNTING_DOWN"        # Quota <= threshold, counting down to resetTime
    WARMUP_SCHEDULED = "WARMUP_SCHEDULED"  # Target fire time scheduled with drift & jitter
    WARMING_UP = "WARMING_UP"              # Keep-alive request in flight
    WARMED_UP = "WARMED_UP"                # 1-token ping succeeded, new window activated
    WEEKLY_BLOCKED = "WEEKLY_BLOCKED"      # Weekly limit exhausted, 5h warmup inhibited
    FAILED = "FAILED"                      # Retries exhausted or circuit breaker tripped


@dataclass
class BucketHorizon:
    """Represents an active quota bucket tracking state."""
    account_email: str
    bucket_id: str                          # e.g. "gemini-5h", "3p-5h", "gemini-weekly"
    window: str                             # "5h" or "weekly"
    remaining_fraction: float               # 0.0 to 1.0
    reset_time: datetime.datetime           # UTC timestamp from CloudCode
    target_fire_time: Optional[datetime.datetime] = None  # reset_time + drift + jitter
    status: HorizonStatus = HorizonStatus.IDLE_HEALTHY
    description: str = ""
    last_updated: float = field(default_factory=time.monotonic)
    error_count: int = 0


class ClockDriftCalibrator:
    """
    Calibrates server clock drift offset using HTTP Date response headers.
    Anchors to local monotonic clock to prevent disruption from local NTP steps.
    """

    def __init__(self, ema_alpha: float = 0.5) -> None:
        self.ema_alpha = max(0.01, min(1.0, ema_alpha))
        self._drift_offset_seconds: float = 0.0
        self._server_wall_utc: Optional[datetime.datetime] = None
        self._local_monotonic_anchor: float = 0.0
        self._calibration_count: int = 0

    @property
    def is_calibrated(self) -> bool:
        return self._calibration_count > 0

    @property
    def drift_offset(self) -> float:
        """Offset in seconds: t_server = t_local + drift_offset."""
        return self._drift_offset_seconds

    def calibrate_from_header(
        self,
        date_header_str: Optional[str],
        local_monotonic: Optional[float] = None,
        rtt_seconds: float = 0.0,
    ) -> float:
        """
        Parses RFC 7231 Date header, calculates drift offset, updates EMA smoothed drift.
        Returns the instantaneous measured drift offset in seconds.
        """
        if not date_header_str or not date_header_str.strip():
            return self._drift_offset_seconds

        try:
            server_dt = email.utils.parsedate_to_datetime(date_header_str)
        except Exception:
            return self._drift_offset_seconds

        if server_dt.tzinfo is None:
            server_dt = server_dt.replace(tzinfo=datetime.timezone.utc)
        else:
            server_dt = server_dt.astimezone(datetime.timezone.utc)

        # Approximate one-way latency correction
        latency_correction = max(0.0, rtt_seconds / 2.0)
        corrected_server_dt = server_dt + datetime.timedelta(seconds=latency_correction)

        now_utc = datetime.datetime.now(datetime.timezone.utc)
        instant_drift = (corrected_server_dt - now_utc).total_seconds()

        mono_now = local_monotonic if local_monotonic is not None else time.monotonic()

        if self._calibration_count == 0:
            self._drift_offset_seconds = instant_drift
        else:
            self._drift_offset_seconds = (
                self.ema_alpha * instant_drift + (1.0 - self.ema_alpha) * self._drift_offset_seconds
            )

        self._server_wall_utc = corrected_server_dt
        self._local_monotonic_anchor = mono_now
        self._calibration_count += 1

        return instant_drift

    def calibrate_from_headers(
        self,
        headers: Mapping[str, str],
        local_monotonic: Optional[float] = None,
        rtt_seconds: float = 0.0,
    ) -> float:
        """Convenience method accepting case-insensitive header mapping."""
        date_val = None
        for k, v in headers.items():
            if k.lower() == "date":
                date_val = v
                break
        return self.calibrate_from_header(date_val, local_monotonic, rtt_seconds)

    def now_calibrated(self) -> datetime.datetime:
        """
        Returns estimated current server time in UTC.
        Uses monotonic clock extrapolation if calibrated, falls back to local UTC.
        """
        if not self.is_calibrated or self._server_wall_utc is None:
            return datetime.datetime.now(datetime.timezone.utc)

        elapsed = time.monotonic() - self._local_monotonic_anchor
        return self._server_wall_utc + datetime.timedelta(seconds=elapsed)


def calculate_jitter_delay(
    reset_time: datetime.datetime,
    now_calibrated: Optional[datetime.datetime] = None,
    min_jitter: float = 0.5,
    max_jitter: float = 3.0,
    rng: Optional[random.Random] = None,
) -> float:
    """
    Computes local seconds to sleep before firing keep-alive ping.
    Formula: max(0, reset_time - now_calibrated) + Uniform(min_jitter, max_jitter).
    If reset_time is already in the past, applies desynchronizing jitter only.
    """
    if now_calibrated is None:
        now_calibrated = datetime.datetime.now(datetime.timezone.utc)
    if now_calibrated.tzinfo is None:
        now_calibrated = now_calibrated.replace(tzinfo=datetime.timezone.utc)

    if reset_time.tzinfo is None:
        reset_time = reset_time.replace(tzinfo=datetime.timezone.utc)
    else:
        reset_time = reset_time.astimezone(datetime.timezone.utc)

    time_until_reset = (reset_time - now_calibrated).total_seconds()
    r = rng if rng is not None else random
    jitter = r.uniform(min_jitter, max_jitter)

    if time_until_reset > 0:
        return time_until_reset + jitter
    return jitter


# Explicit alias matching dispatch instructions:
calculate_warmup_delay = calculate_jitter_delay


def calculate_jittered_target(
    reset_time: datetime.datetime,
    drift_offset: float = 0.0,
    min_jitter: float = 0.5,
    max_jitter: float = 3.0,
    rng: Optional[random.Random] = None,
) -> datetime.datetime:
    """Calculates absolute UTC target timestamp incorporating drift and jitter."""
    if reset_time.tzinfo is None:
        reset_time = reset_time.replace(tzinfo=datetime.timezone.utc)
    else:
        reset_time = reset_time.astimezone(datetime.timezone.utc)

    r = rng if rng is not None else random
    jitter = r.uniform(min_jitter, max_jitter)
    local_target = reset_time - datetime.timedelta(seconds=drift_offset) + datetime.timedelta(seconds=jitter)
    return local_target


class ResetHorizonTracker:
    """
    Tracks reset times and countdowns across all accounts and model quota buckets.
    """

    def __init__(
        self,
        calibrator: Optional[ClockDriftCalibrator] = None,
        warmup_threshold: float = 0.05,
    ) -> None:
        self.calibrator = calibrator or ClockDriftCalibrator()
        self.warmup_threshold = warmup_threshold
        self._buckets: dict[tuple[str, str], BucketHorizon] = {}

    def update_bucket(
        self,
        account_email: str,
        bucket_id: str,
        window: str,
        remaining_fraction: float,
        reset_time: datetime.datetime,
        weekly_exhausted: bool = False,
        description: str = "",
    ) -> BucketHorizon:
        """Updates or registers a quota bucket horizon."""
        if reset_time.tzinfo is None:
            reset_time = reset_time.replace(tzinfo=datetime.timezone.utc)
        else:
            reset_time = reset_time.astimezone(datetime.timezone.utc)

        key = (account_email, bucket_id)
        now_cal = self.calibrator.now_calibrated()

        # Determine status
        if weekly_exhausted and window != "weekly":
            status = HorizonStatus.WEEKLY_BLOCKED
            target_fire = None
        elif remaining_fraction <= self.warmup_threshold:
            status = HorizonStatus.WARMUP_SCHEDULED
            delay = calculate_jitter_delay(reset_time, now_cal)
            target_fire = datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(seconds=delay)
        else:
            status = HorizonStatus.IDLE_HEALTHY
            target_fire = None

        horizon = BucketHorizon(
            account_email=account_email,
            bucket_id=bucket_id,
            window=window,
            remaining_fraction=remaining_fraction,
            reset_time=reset_time,
            target_fire_time=target_fire,
            status=status,
            description=description,
            last_updated=time.monotonic(),
        )
        self._buckets[key] = horizon
        return horizon

    def get_horizon(self, account_email: str, bucket_id: str) -> Optional[BucketHorizon]:
        return self._buckets.get((account_email, bucket_id))

    def get_all_horizons(self) -> list[BucketHorizon]:
        return list(self._buckets.values())

    def get_countdown_seconds(self, account_email: str, bucket_id: str) -> float:
        """Returns seconds remaining until reset_time in calibrated server time."""
        horizon = self.get_horizon(account_email, bucket_id)
        if not horizon:
            return 0.0
        now_cal = self.calibrator.now_calibrated()
        rem = (horizon.reset_time - now_cal).total_seconds()
        return max(0.0, rem)

    def get_formatted_countdown(self, account_email: str, bucket_id: str) -> str:
        """Returns human-readable countdown string (e.g. '03:45:12' or '00:00:00')."""
        sec = int(round(self.get_countdown_seconds(account_email, bucket_id)))
        hours = sec // 3600
        minutes = (sec % 3600) // 60
        seconds = sec % 60
        return f"{hours:02d}:{minutes:02d}:{seconds:02d}"

    def get_due_warmups(self) -> list[BucketHorizon]:
        """Returns all horizons ready to fire keepalive pings (status WARMUP_SCHEDULED and target reached)."""
        now_utc = datetime.datetime.now(datetime.timezone.utc)
        due: list[BucketHorizon] = []
        for horizon in self._buckets.values():
            if (
                horizon.status == HorizonStatus.WARMUP_SCHEDULED
                and horizon.target_fire_time is not None
                and now_utc >= horizon.target_fire_time
            ):
                due.append(horizon)
        return due

    def mark_warmed_up(self, account_email: str, bucket_id: str) -> None:
        horizon = self.get_horizon(account_email, bucket_id)
        if horizon:
            horizon.status = HorizonStatus.WARMED_UP
            horizon.target_fire_time = None
            horizon.remaining_fraction = 1.0

    def mark_failed(self, account_email: str, bucket_id: str) -> None:
        horizon = self.get_horizon(account_email, bucket_id)
        if horizon:
            horizon.status = HorizonStatus.FAILED
            horizon.error_count += 1
