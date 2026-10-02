# Reset Horizon & Warmup Engine Blueprint (F08)

**Agent**: `explorer_m2_2` (M2 Reset Horizon & Warmup Explorer)  
**Milestone**: M2 (Upstream Quota Poller, Warmup Engine & Rule Engine - R3)  
**Working Directory**: `/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_m2_2`  
**Target Delivery**: `/mnt/Data/Projects/Antigravity Swiss Knife/.agents/teamwork/explorer_m2_2/handoff.md`  
**Feature Scope**: `F08_RESET_HORIZON_WARMUP` (`antigravity_swiss/warmup/horizon.py` and `antigravity_swiss/warmup/engine.py`)  
**Timestamp**: `2026-10-02T09:12:00Z`  

---

## 1. Observation

### 1.1 Direct Runtime Environment & Dependency Analysis
- Executed `python3 -c "import urllib.request; import aiohttp"`:
  ```
  urllib ok
  ModuleNotFoundError: No module named 'aiohttp'
  ```
- Executed `python3 -c "import requests; import httpx"`:
  ```
  ModuleNotFoundError: No module named 'requests'
  ```
- **Finding**: The execution environment does NOT have external HTTP packages (`requests`, `httpx`, `aiohttp`). All HTTP client interactions must strictly use Python's standard library (`urllib.request`, `http.client`, `urllib.error`). Asynchronous execution within the daemon must wrap these standard library calls via `asyncio.to_thread` or standard thread pools.

### 1.2 HTTP Date Header & RFC 7231 Parsing
- Probed RFC 7231 / RFC 2822 date parsing using Python standard library:
  ```python
  import email.utils
  dt = email.utils.parsedate_to_datetime('Thu, 01 Oct 2026 05:04:53 GMT')
  # Returns: datetime.datetime(2026, 10, 1, 5, 4, 53, tzinfo=datetime.timezone.utc)
  ```
- **Finding**: `email.utils.parsedate_to_datetime()` parses standard HTTP `Date` response headers into timezone-aware UTC `datetime` objects in microseconds with zero external dependencies.

### 1.3 Upstream Protobuf Schema & Keep-Alive Payloads
- In `/opt/Antigravity/resources/bin/language_server` (and verified in `spec_miner_quota_1/handoff.md` lines 128-138):
  - Service: `google.internal.cloud.code.v1internal.PredictionService`
  - RPC: `GenerateContent(GenerateContentRequest) returns (GenerateContentResponse)`
  - HTTP Rule: `POST /v1internal:generateContent`
  - Top-level JSON message fields:
    - `project: string` (empty `""` for consumer accounts)
    - `model: string` (e.g. `"gemini-3.8-flash-high"`, `"gemini-3.5-flash-lite"`)
    - `request: google.cloud.aiplatform.master.GenerateContentRequest`:
      - `contents: repeated Content` (e.g. `[{"role": "user", "parts": [{"text": "ping"}]}]`)
      - `generationConfig: GenerationConfig` (e.g. `{"maxOutputTokens": 1, "temperature": 0.0}`)

### 1.4 Mock Server Behavior on `:generateContent` and `429`
- Probed `tests/fixtures/mock_cloudcode_server.py`:
  - When `MockCloudCodeServer` receives `POST /v1internal:generateContent`:
    - Checks `body.get("request", {}).get("generationConfig", {}).get("maxOutputTokens", 1)`.
    - Always injects `Date: <RFC 7231 date>` header in responses (lines 38-41).
    - If quota is exhausted and $T_{\text{sim}} < T_{\text{reset}}$, returns `HTTP 429` with `status: "RESOURCE_EXHAUSTED"` and emits `Date` header.
    - If $T_{\text{sim}} \ge T_{\text{reset}}$, transitions `warmup_fired = True`, refreshes `current_quota_fraction = 1.0`, rolls reset time forward by 5 hours, and returns `HTTP 200` with `promptTokenCount: 1, candidatesTokenCount: 1, totalTokenCount: 2`.

### 1.5 System Signals & Antigravity IDE Stability Guard
- As documented in `ORIGINAL_REQUEST.md` (lines 85-96) and hardened in `antigravity_swiss/process/lifecycle.py`:
  - Never scan host `/proc` without strict `--user-data-dir` isolation.
  - Never emit POSIX signals (`SIGTERM`, `SIGKILL`) to host processes.
  - Quota and warmup polling runs entirely out-of-band via HTTP REST and Secret Service keyring lookup, without launching or terminating Antigravity processes.

---

## 2. Logic Chain

1. **Why Warmup Keep-Alive is Required (The 5-Hour Idle Trap)**:
   - Observation: Google CloudCode enforces rolling 5-hour quota windows (`gemini-5h`, `3p-5h`) anchored to the initial user prompt after a reset.
   - Inference: If an account hits 0% quota, Google sets `resetTime` 5 hours in the future. When `resetTime` passes, quota resets to 100%, but the *next* 5-hour countdown is NOT triggered until a request is made.
   - Consequence: If a developer leaves their workstation idle for 12 hours overnight, an account that resets at hour 5 will only start its next window at hour 12 when they return.
   - Solution: By firing an automated 1-token keep-alive ping (`maxOutputTokens: 1`) immediately upon `resetTime` arrival, the next 5-hour window begins immediately at hour 5, allowing full passive replenishment.

2. **Why Clock Drift Calibration is Essential**:
   - Observation: Local client system clocks frequently differ from Google CloudCode backend servers by hundreds of milliseconds to several seconds due to NTP drift, local clock skew, or hypervisor suspension.
   - Inference: If the local client clock is 2 seconds fast, it will fire the keep-alive ping at `resetTime - 2.0s` according to Google's server, resulting in an immediate `HTTP 429 RESOURCE_EXHAUSTED`.
   - Consequence: The client burns a retry attempt and risks thundering herd backoff.
   - Solution: Calibrating local monotonic/UTC time against the HTTP `Date` response header provides the server offset $\Delta t = t_{\text{server}} - t_{\text{local}}$. Extrapolating via `time.monotonic()` guarantees immunity against local wall-clock jumps.

3. **Why Jitter Must Be Scheduled at `resetTime + uniform(0.5, 3.0)` Seconds**:
   - Observation: Google's backend distributed quota cache flush does not happen with microsecond atomic precision across all edge clusters.
   - Inference: A ping arriving at $T_{\text{reset}} + 0.05\text{s}$ has a non-trivial probability of hitting an un-invalidated distributed cache entry.
   - Solution: Enforcing a minimum delay of $+0.5\text{s}$ ensures the backend cache reconciliation has completed. Adding random uniform jitter up to $+3.0\text{s}$ desynchronizes requests from concurrent clients, avoiding rate-limiter spikes.

4. **Why Standby Accounts Benefit from Passive Warmup**:
   - Observation: `AccountVault` stores multiple accounts in `accounts.json`. Only one account is active at any time.
   - Inference: When the active account is working, idle standby accounts whose 5-hour quotas have expired can be warmed up in the background using their stored tokens.
   - Consequence: When the active account eventually runs out of quota, the secondary accounts in the vault are already primed with freshly reset 5-hour horizons.

5. **Why Circuit Breaker and Exponential Backoff Guard the System**:
   - Observation: Upstream API outages (503) or account-level suspensions (429/403) can cause repeated request failures.
   - Inference: Unchecked keep-alive retry loops would thrash system resources, burn network bandwidth, and risk account throttling.
   - Solution: Maximum 3 retries with exponential backoff ($1.0\text{s} \to 2.0\text{s} \to 4.0\text{s} + \text{jitter}$) handles transient hiccups. A 3-state Circuit Breaker (`CLOSED` $\to$ `OPEN` $\to$ `HALF_OPEN`) trips after 5 consecutive failures and halts requests for 60 seconds.

---

## 3. Caveats

1. **Weekly Quota Exhaustion (`gemini-weekly` / `3p-weekly`)**:
   - If an account has `remaining_fraction == 0.0` on its weekly window, firing a warmup ping for the 5-hour window will fail with `HTTP 429` regardless of whether the 5-hour window reached `resetTime`.
   - The horizon engine must check weekly bucket status: if `weekly_remaining == 0.0`, the 5-hour warmup must be inhibited (`WEEKLY_BLOCKED`) until the weekly reset arrives.
2. **Standard Library Threading vs Asyncio**:
   - Python standard library's `urllib.request` is synchronous blocking I/O.
   - When integrating with the daemon's `asyncio` event loop (`AsyncUnixSocketServer`), keep-alive network requests must be offloaded using `await asyncio.to_thread(warmup_engine.send_keepalive, ...)`.
3. **Clock Skew Granularity**:
   - The HTTP `Date` header has 1-second resolution (no sub-second milliseconds).
   - Measuring round-trip time (RTT) and adding $\frac{\text{RTT}}{2}$ improves accuracy, but the $\ge 0.5\text{s}$ jitter safety buffer already accounts for sub-second precision variances.

---

## 4. Conclusion & Structured Implementation Blueprint

The reset horizon and warmup subsystem shall be organized into two core modules under `antigravity_swiss/warmup/`:
1. `antigravity_swiss/warmup/horizon.py`: Clock drift calibrator, jitter scheduler, bucket horizon models, and countdown engine.
2. `antigravity_swiss/warmup/engine.py`: 1-token keep-alive payload builder, HTTP client, retry policy with exponential backoff, circuit breaker, and background scheduler daemon.
3. `antigravity_swiss/warmup/__init__.py`: Public interface exposing all core classes.

```
antigravity_swiss/warmup/
├── __init__.py           # Re-exports: ClockDriftCalibrator, ResetHorizonTracker, WarmupEngine, CircuitBreaker
├── horizon.py            # Clock calibration, jitter algorithm, horizon models & countdown tracking
└── engine.py             # 1-token payload builder, keep-alive client, backoff retry & circuit breaker
```

### 4.1 Module 1: `antigravity_swiss/warmup/horizon.py`

#### A. Data Models & Status Enums
```python
from __future__ import annotations

from dataclasses import dataclass, field
import datetime
from enum import Enum
import math
import random
import time
from typing import Any, Mapping


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
    target_fire_time: datetime.datetime | None = None  # reset_time + drift + jitter
    status: HorizonStatus = HorizonStatus.IDLE_HEALTHY
    description: str = ""
    last_updated: float = field(default_factory=time.monotonic)
    error_count: int = 0
```

#### B. `ClockDriftCalibrator`
```python
import email.utils

class ClockDriftCalibrator:
    """
    Calibrates server clock drift offset using HTTP Date response headers.
    Anchors to local monotonic clock to prevent disruption from local NTP steps.
    """

    def __init__(self, ema_alpha: float = 0.5) -> None:
        self.ema_alpha = max(0.01, min(1.0, ema_alpha))
        self._drift_offset_seconds: float = 0.0
        self._server_wall_utc: datetime.datetime | None = None
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
        date_header_str: str | None,
        local_monotonic: float | None = None,
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

        # Ensure UTC timezone
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
        local_monotonic: float | None = None,
        rtt_seconds: float = 0.0,
    ) -> float:
        """Convenience method accepting case-insensitive header mapping."""
        # Find 'Date' header case-insensitively
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
```

#### C. Jitter Scheduling Algorithm
```python
def calculate_jitter_delay(
    reset_time: datetime.datetime,
    now_calibrated: datetime.datetime | None = None,
    min_jitter: float = 0.5,
    max_jitter: float = 3.0,
    rng: random.Random | None = None,
) -> float:
    """
    Computes local seconds to sleep before firing keep-alive ping.
    Formula: (reset_time - now_calibrated) + Uniform(min_jitter, max_jitter).
    If reset_time is already in the past, applies desynchronizing jitter only.
    """
    if now_calibrated is None:
        now_calibrated = datetime.datetime.now(datetime.timezone.utc)

    # Normalize reset_time to UTC
    if reset_time.tzinfo is None:
        reset_time = reset_time.replace(tzinfo=datetime.timezone.utc)
    else:
        reset_time = reset_time.astimezone(datetime.timezone.utc)

    time_until_reset = (reset_time - now_calibrated).total_seconds()
    r = rng if rng is not None else random
    jitter = r.uniform(min_jitter, max_jitter)

    if time_until_reset > 0:
        return time_until_reset + jitter
    else:
        # Reset already passed: fire after minimal desync jitter
        return jitter


def calculate_jittered_target(
    reset_time: datetime.datetime,
    drift_offset: float = 0.0,
    min_jitter: float = 0.5,
    max_jitter: float = 3.0,
    rng: random.Random | None = None,
) -> datetime.datetime:
    """Calculates absolute UTC target timestamp incorporating drift and jitter."""
    if reset_time.tzinfo is None:
        reset_time = reset_time.replace(tzinfo=datetime.timezone.utc)
    else:
        reset_time = reset_time.astimezone(datetime.timezone.utc)

    r = rng if rng is not None else random
    jitter = r.uniform(min_jitter, max_jitter)
    # Target in server time is reset_time + jitter
    # Target in local wall clock UTC is (reset_time - drift_offset) + jitter
    local_target = reset_time - datetime.timedelta(seconds=drift_offset) + datetime.timedelta(seconds=jitter)
    return local_target
```

#### D. `ResetHorizonTracker`
```python
class ResetHorizonTracker:
    """
    Tracks reset times and countdowns across all accounts and model quota buckets.
    """

    def __init__(
        self,
        calibrator: ClockDriftCalibrator | None = None,
        warmup_threshold: float = 0.05,
    ) -> None:
        self.calibrator = calibrator or ClockDriftCalibrator()
        self.warmup_threshold = warmup_threshold
        # Key: (account_email, bucket_id) -> BucketHorizon
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

    def get_horizon(self, account_email: str, bucket_id: str) -> BucketHorizon | None:
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
        sec = int(self.get_countdown_seconds(account_email, bucket_id))
        hours = sec // 3600
        minutes = (sec % 3600) // 60
        seconds = sec % 60
        return f"{hours:02d}:{minutes:02d}:{seconds:02d}"

    def get_due_warmups(self) -> list[BucketHorizon]:
        """Returns all horizons ready to fire keepalive pings (status WARMUP_SCHEDULED and target reached)."""
        now_utc = datetime.datetime.now(datetime.timezone.utc)
        due = []
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
```

---

### 4.2 Module 2: `antigravity_swiss/warmup/engine.py`

#### A. 1-Token Keep-Alive Payload Builder
```python
from __future__ import annotations

from dataclasses import dataclass, field
from enum import Enum
import json
import logging
import time
from typing import Any, Mapping
import urllib.error
import urllib.request

from antigravity_swiss.core.constants import (
    DEFAULT_MAX_TOKENS_WARMUP,
    DEFAULT_WARMUP_MODEL_ID,
    GOOGLE_GENERATE_CONTENT_URL,
    GOOGLE_USER_AGENT,
)
from antigravity_swiss.warmup.horizon import ClockDriftCalibrator, ResetHorizonTracker

logger = logging.getLogger("antigravity_swiss.warmup")


def build_warmup_payload(
    model: str = DEFAULT_WARMUP_MODEL_ID,
    prompt_text: str = "ping",
    max_output_tokens: int = DEFAULT_MAX_TOKENS_WARMUP,
    project: str = "",
) -> dict[str, Any]:
    """
    Constructs the minimal 1-token keep-alive payload for POST /v1internal:generateContent.
    Matches google.internal.cloud.code.v1internal.PredictionService protobuf schema.
    """
    return {
        "project": project,
        "model": model,
        "request": {
            "contents": [
                {
                    "role": "user",
                    "parts": [{"text": prompt_text}],
                }
            ],
            "generationConfig": {
                "maxOutputTokens": max_output_tokens,
                "temperature": 0.0,
            },
        },
    }
```

#### B. Circuit Breaker
```python
class CircuitBreakerState(str, Enum):
    CLOSED = "CLOSED"      # Normal: requests pass through
    OPEN = "OPEN"          # Tripped: requests fail immediately without network call
    HALF_OPEN = "HALF_OPEN"# Testing: single probe allowed to verify recovery


class CircuitBreaker:
    """
    3-state circuit breaker preventing persistent hammering of upstream APIs
    during severe outages or account-level quota blocks.
    """

    def __init__(
        self,
        failure_threshold: int = 5,
        recovery_timeout_seconds: float = 60.0,
    ) -> None:
        self.failure_threshold = failure_threshold
        self.recovery_timeout_seconds = recovery_timeout_seconds
        self.state: CircuitBreakerState = CircuitBreakerState.CLOSED
        self.consecutive_failures: int = 0
        self.last_state_change: float = time.monotonic()
        self.last_failure_time: float = 0.0
        self.last_success_time: float = 0.0

    def allow_request(self) -> bool:
        """Checks if a request is permitted under circuit breaker rules."""
        now = time.monotonic()
        if self.state == CircuitBreakerState.CLOSED:
            return True

        if self.state == CircuitBreakerState.OPEN:
            if now - self.last_state_change >= self.recovery_timeout_seconds:
                self.state = CircuitBreakerState.HALF_OPEN
                self.last_state_change = now
                logger.info("Circuit breaker transitioning OPEN -> HALF_OPEN (probe permitted)")
                return True
            return False

        if self.state == CircuitBreakerState.HALF_OPEN:
            # Only 1 probe at a time in half-open
            return True

        return False

    def record_success(self) -> None:
        """Records a successful request, resetting failure counters."""
        self.consecutive_failures = 0
        self.last_success_time = time.monotonic()
        if self.state != CircuitBreakerState.CLOSED:
            logger.info("Circuit breaker recovered: transitioning to CLOSED")
            self.state = CircuitBreakerState.CLOSED
            self.last_state_change = time.monotonic()

    def record_failure(self, error: Exception | None = None) -> None:
        """Records a request failure, potentially tripping the circuit."""
        now = time.monotonic()
        self.consecutive_failures += 1
        self.last_failure_time = now

        if self.state == CircuitBreakerState.HALF_OPEN:
            logger.warning("Probe failed in HALF_OPEN: tripping back to OPEN")
            self.state = CircuitBreakerState.OPEN
            self.last_state_change = now
        elif self.state == CircuitBreakerState.CLOSED and self.consecutive_failures >= self.failure_threshold:
            logger.warning(
                "Circuit breaker tripped to OPEN after %d consecutive failures",
                self.consecutive_failures,
            )
            self.state = CircuitBreakerState.OPEN
            self.last_state_change = now

    def reset(self) -> None:
        """Forces circuit breaker back to CLOSED state."""
        self.state = CircuitBreakerState.CLOSED
        self.consecutive_failures = 0
        self.last_state_change = time.monotonic()
```

#### C. Retry Policy with Exponential Backoff
```python
class WarmupRetryPolicy:
    """
    Exponential backoff policy for HTTP 429 / 503 errors with a strict max of 3 retries.
    """

    def __init__(
        self,
        max_retries: int = 3,
        base_delay: float = 1.0,
        backoff_multiplier: float = 2.0,
        max_delay: float = 10.0,
    ) -> None:
        self.max_retries = max_retries
        self.base_delay = base_delay
        self.backoff_multiplier = backoff_multiplier
        self.max_delay = max_delay

    def is_retryable(self, status_code: int) -> bool:
        """Returns True if status code indicates a transient condition."""
        return status_code in (429, 502, 503, 504)

    def get_delay_seconds(self, attempt_index: int) -> float:
        """
        Computes exponential backoff delay:
        min(max_delay, base_delay * (multiplier ** attempt_index)) + uniform(0.1, 0.5)
        """
        exp_delay = self.base_delay * (self.backoff_multiplier ** attempt_index)
        bounded = min(self.max_delay, exp_delay)
        jitter = random.uniform(0.1, 0.5)
        return bounded + jitter
```

#### D. `WarmupResult` & `WarmupEngine`
```python
@dataclass
class WarmupResult:
    """Outcome of a keep-alive warmup operation."""
    success: bool
    status_code: int
    model_id: str
    latency_ms: float
    response_data: dict[str, Any] = field(default_factory=dict)
    error_message: str | None = None
    attempts: int = 1
    circuit_breaker_tripped: bool = False


class WarmupEngine:
    """
    Dispatches 1-token keep-alive pings to activate new quota windows upon reset arrival.
    Implements HTTP Date calibration on every response, backoff retries, and circuit breaker.
    """

    def __init__(
        self,
        endpoint_url: str = GOOGLE_GENERATE_CONTENT_URL,
        user_agent: str = GOOGLE_USER_AGENT,
        calibrator: ClockDriftCalibrator | None = None,
        circuit_breaker: CircuitBreaker | None = None,
        retry_policy: WarmupRetryPolicy | None = None,
        timeout_seconds: float = 15.0,
    ) -> None:
        self.endpoint_url = endpoint_url
        self.user_agent = user_agent
        self.calibrator = calibrator or ClockDriftCalibrator()
        self.circuit_breaker = circuit_breaker or CircuitBreaker()
        self.retry_policy = retry_policy or WarmupRetryPolicy()
        self.timeout_seconds = timeout_seconds

    def send_keepalive(
        self,
        access_token: str,
        model_id: str = DEFAULT_WARMUP_MODEL_ID,
        project: str = "",
    ) -> WarmupResult:
        """
        Synchronously sends 1-token keep-alive ping with retry loop and circuit breaker.
        """
        if not self.circuit_breaker.allow_request():
            logger.warning("Warmup rejected: Circuit breaker is OPEN")
            return WarmupResult(
                success=False,
                status_code=-1,
                model_id=model_id,
                latency_ms=0.0,
                error_message="Circuit breaker is OPEN",
                circuit_breaker_tripped=True,
            )

        payload = build_warmup_payload(model=model_id, project=project)
        encoded_data = json.dumps(payload).encode("utf-8")

        headers = {
            "Authorization": f"Bearer {access_token}",
            "Content-Type": "application/json",
            "User-Agent": self.user_agent,
        }

        total_attempts = 0
        last_error_msg = ""
        last_status = 0

        while total_attempts <= self.retry_policy.max_retries:
            total_attempts += 1
            t_start = time.monotonic()

            req = urllib.request.Request(
                self.endpoint_url,
                data=encoded_data,
                headers=headers,
                method="POST",
            )

            try:
                with urllib.request.urlopen(req, timeout=self.timeout_seconds) as resp:
                    t_end = time.monotonic()
                    latency_ms = (t_end - t_start) * 1000.0

                    # Calibrate clock drift from HTTP Date response header
                    resp_headers = dict(resp.headers)
                    self.calibrator.calibrate_from_headers(resp_headers, local_monotonic=t_end, rtt_seconds=(t_end - t_start))

                    body_str = resp.read().decode("utf-8", errors="replace")
                    resp_data = json.loads(body_str) if body_str else {}

                    self.circuit_breaker.record_success()
                    logger.info("Warmup keep-alive succeeded for model '%s' (latency: %.1fms)", model_id, latency_ms)
                    return WarmupResult(
                        success=True,
                        status_code=resp.status,
                        model_id=model_id,
                        latency_ms=latency_ms,
                        response_data=resp_data,
                        attempts=total_attempts,
                    )

            except urllib.error.HTTPError as exc:
                t_end = time.monotonic()
                latency_ms = (t_end - t_start) * 1000.0
                last_status = exc.code

                # Calibrate clock drift even from error responses!
                if exc.headers:
                    self.calibrator.calibrate_from_headers(dict(exc.headers), local_monotonic=t_end, rtt_seconds=(t_end - t_start))

                err_body = exc.read().decode("utf-8", errors="replace")
                last_error_msg = f"HTTP {exc.code}: {err_body}"

                # Non-retryable errors (e.g. 400 Bad Request, 401 Unauthorized, 403 Forbidden)
                if not self.retry_policy.is_retryable(exc.code):
                    self.circuit_breaker.record_failure(exc)
                    logger.error("Non-retryable HTTP error during warmup (%d): %s", exc.code, last_error_msg)
                    return WarmupResult(
                        success=False,
                        status_code=exc.code,
                        model_id=model_id,
                        latency_ms=latency_ms,
                        error_message=last_error_msg,
                        attempts=total_attempts,
                    )

                # Transient errors (429, 503)
                logger.warning(
                    "Transient HTTP %d on warmup attempt %d/%d: %s",
                    exc.code,
                    total_attempts,
                    self.retry_policy.max_retries + 1,
                    last_error_msg,
                )

                if total_attempts <= self.retry_policy.max_retries:
                    sleep_sec = self.retry_policy.get_delay_seconds(total_attempts - 1)
                    time.sleep(sleep_sec)
                else:
                    self.circuit_breaker.record_failure(exc)

            except (urllib.error.URLError, TimeoutError, OSError) as exc:
                t_end = time.monotonic()
                latency_ms = (t_end - t_start) * 1000.0
                last_status = -1
                last_error_msg = f"Network error: {exc}"
                logger.warning("Network error on warmup attempt %d: %s", total_attempts, exc)

                if total_attempts <= self.retry_policy.max_retries:
                    sleep_sec = self.retry_policy.get_delay_seconds(total_attempts - 1)
                    time.sleep(sleep_sec)
                else:
                    self.circuit_breaker.record_failure(exc)

        return WarmupResult(
            success=False,
            status_code=last_status,
            model_id=model_id,
            latency_ms=0.0,
            error_message=last_error_msg,
            attempts=total_attempts,
        )

    async def send_keepalive_async(
        self,
        access_token: str,
        model_id: str = DEFAULT_WARMUP_MODEL_ID,
        project: str = "",
    ) -> WarmupResult:
        """Asynchronously dispatches keepalive via asyncio.to_thread."""
        import asyncio
        return await asyncio.to_thread(self.send_keepalive, access_token, model_id, project)
```

#### E. Background Warmup Scheduler (`WarmupScheduler`)
```python
import asyncio
from typing import Callable, Coroutine

class WarmupScheduler:
    """
    Coordinates the ResetHorizonTracker and WarmupEngine inside the daemon.
    Polls active horizons and triggers keepalives as countdowns expire.
    """

    def __init__(
        self,
        tracker: ResetHorizonTracker,
        engine: WarmupEngine,
        token_provider: Callable[[str], str | None],  # (account_email) -> access_token
        on_warmup_success: Callable[[BucketHorizon, WarmupResult], None] | None = None,
        on_warmup_failure: Callable[[BucketHorizon, WarmupResult], None] | None = None,
        check_interval_seconds: float = 1.0,
    ) -> None:
        self.tracker = tracker
        self.engine = engine
        self.token_provider = token_provider
        self.on_warmup_success = on_warmup_success
        self.on_warmup_failure = on_warmup_failure
        self.check_interval_seconds = check_interval_seconds
        self._running = False
        self._task: asyncio.Task[None] | None = None

    async def start(self) -> None:
        """Starts background monitoring task."""
        self._running = True
        self._task = asyncio.create_task(self._run_loop())

    async def stop(self) -> None:
        """Stops background monitoring task cleanly."""
        self._running = False
        if self._task and not self._task.done():
            self._task.cancel()
            try:
                await self._task
            except asyncio.CancelledError:
                pass

    async def _run_loop(self) -> None:
        while self._running:
            try:
                due_horizons = self.tracker.get_due_warmups()
                for horizon in due_horizons:
                    token = self.token_provider(horizon.account_email)
                    if not token:
                        logger.warning("No token available for account '%s'", horizon.account_email)
                        continue

                    horizon.status = HorizonStatus.WARMING_UP
                    result = await self.engine.send_keepalive_async(token)

                    if result.success:
                        self.tracker.mark_warmed_up(horizon.account_email, horizon.bucket_id)
                        if self.on_warmup_success:
                            self.on_warmup_success(horizon, result)
                    else:
                        self.tracker.mark_failed(horizon.account_email, horizon.bucket_id)
                        if self.on_warmup_failure:
                            self.on_warmup_failure(horizon, result)

            except Exception as exc:
                logger.error("Error in warmup scheduler tick: %s", exc)

            await asyncio.sleep(self.check_interval_seconds)
```

---

### 4.3 Integration Contracts with Peer M2 Modules

| Interface | Peer Module | Interaction |
|-----------|-------------|-------------|
| `QuotaSummary` Ingestion | `antigravity_swiss.quota.poller` (explorer_m2_1) | When `QuotaPoller` fetches `retrieveUserQuotaSummary`, it passes the parsed `groups` to `tracker.update_bucket()` for all accounts. |
| Clock Drift Propagation | `antigravity_swiss.quota.client` (explorer_m2_1) | `QuotaClient` extracts `Date` headers from its responses and passes them to `calibrator.calibrate_from_headers()`, ensuring drift offset stays fresh. |
| Account Token Resolution | `antigravity_swiss.keyring.switcher` (M1) | `WarmupScheduler` calls `vault.get_account(email).credential.access_token` to authenticate keepalive pings without switching the active OS keyring. |
| Auto-Switch Coordination | `antigravity_swiss.quota.rule_engine` (explorer_m2_3) | When an account warms up, `notify.warmup_triggered` is broadcast via IPC, informing the rule engine that fresh quota is available. |
| Offline Mock Validation | `tests/fixtures/mock_cloudcode_server.py` (explorer_m2_3) | Tests run against `MockCloudCodeServer` endpoints `:generateContent` with `/test_control/set_quota` and `/test_control/advance_time`. |

---

## 5. Verification Method

To independently verify the implementation blueprint:

### 5.1 Unit Tests for `ClockDriftCalibrator` and Jitter
Create `tests/unit/test_warmup_horizon.py`:
- **Test 1 (`test_clock_drift_calibration_rfc7231`)**: Pass synthetic HTTP `Date` headers ("Fri, 02 Oct 2026 09:00:00 GMT") with mocked local time, verify positive and negative drift offsets are computed accurately and smoothed via EMA.
- **Test 2 (`test_monotonic_anchoring_immunity_to_system_time_jump`)**: Advance local monotonic clock, verify `now_calibrated()` advances identically without skew.
- **Test 3 (`test_jitter_bounding`)**: Generate 1,000 jitter calculations with reset times; assert all values strictly satisfy $0.5 \le \text{jitter} \le 3.0$ seconds.
- **Test 4 (`test_countdown_formatting`)**: Test 0 seconds (`"00:00:00"`), 65 seconds (`"00:01:05"`), and 3 hours 45 minutes 12 seconds (`"03:45:12"`).

### 5.2 Unit Tests for `WarmupEngine` and Circuit Breaker
Create `tests/unit/test_warmup_engine.py`:
- **Test 1 (`test_1token_payload_structure`)**: Assert `build_warmup_payload()` matches `PredictionServiceProto` JSON format (`maxOutputTokens: 1`, text `"ping"`).
- **Test 2 (`test_circuit_breaker_trips_after_failures`)**: Record 5 failures; assert state becomes `OPEN` and `allow_request()` returns `False`. Fast-forward time past `recovery_timeout_seconds`; assert state becomes `HALF_OPEN`. Record success; assert state returns to `CLOSED`.
- **Test 3 (`test_retry_backoff_on_transient_errors`)**: Run against `MockCloudCodeServer` with `/test_control/simulate_transient_error` set to 503 count=2; assert engine retries twice, succeeds on attempt 3, and returns `attempts: 3`.

### 5.3 Integration Test with `MockCloudCodeServer`
Create `tests/integration/test_warmup_integration.py`:
- Start `MockCloudCodeServer` via pytest fixture `mock_cloudcode`.
- Set quota to 0.0 with reset in 10 seconds.
- Advance mock time by 10 seconds via `/test_control/advance_time`.
- Trigger keepalive ping via `WarmupEngine`.
- Assert `MockCloudCodeServer` registers `warmup_fired = True`, updates quota to 1.0, and resets horizon by 5 hours.

### 5.4 Command Execution
```bash
pytest tests/unit/ -k "warmup or horizon" -v
pytest tests/ -v
```
- Invalidation condition: Any failure to parse RFC 7231 Date, jitter falling outside `[0.5, 3.0]`, retries exceeding 3, or dependency on uninstalled packages (`aiohttp`, `requests`).
