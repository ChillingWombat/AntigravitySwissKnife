"""
1-Token Keep-Alive Warmup Engine & Circuit Breaker Subsystem (Feature F08).
==========================================================================
Constructs minimal 1-token keep-alive payloads, dispatches keep-alive requests
to Google CloudCode v1internal:generateContent, and implements exponential
backoff retry and 3-state circuit breaker protection.
"""

from __future__ import annotations

import asyncio
from dataclasses import dataclass, field
import datetime
from enum import Enum
import json
import logging
import random
import time
from typing import Any, Callable, Mapping, Optional
import urllib.error
import urllib.request

from antigravity_swiss.core.constants import (
    DEFAULT_MAX_TOKENS_WARMUP,
    DEFAULT_WARMUP_MODEL_ID,
    GOOGLE_GENERATE_CONTENT_URL,
    GOOGLE_USER_AGENT,
)
from antigravity_swiss.warmup.horizon import BucketHorizon, ClockDriftCalibrator, HorizonStatus, ResetHorizonTracker

logger = logging.getLogger("antigravity_swiss.warmup.engine")


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


class CircuitBreakerState(str, Enum):
    CLOSED = "CLOSED"        # Normal: requests pass through
    OPEN = "OPEN"            # Tripped: requests fail immediately without network call
    HALF_OPEN = "HALF_OPEN"  # Testing: single probe allowed to verify recovery


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

    def record_failure(self, error: Optional[Exception] = None) -> None:
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


@dataclass
class WarmupResult:
    """Outcome of a keep-alive warmup operation."""
    success: bool
    status_code: int
    model_id: str
    latency_ms: float
    response_data: dict[str, Any] = field(default_factory=dict)
    error_message: Optional[str] = None
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
        user_agent: str = f"{GOOGLE_USER_AGENT} linux/amd64",
        calibrator: Optional[ClockDriftCalibrator] = None,
        circuit_breaker: Optional[CircuitBreaker] = None,
        retry_policy: Optional[WarmupRetryPolicy] = None,
        timeout_seconds: float = 15.0,
        cloudcode_port: Optional[int] = None,
    ) -> None:
        if cloudcode_port is not None:
            endpoint_url = f"http://127.0.0.1:{cloudcode_port}/v1internal:generateContent"
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
            "Content-Length": str(len(encoded_data)),
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

                    resp_headers = dict(resp.headers)
                    self.calibrator.calibrate_from_headers(
                        resp_headers, local_monotonic=t_end, rtt_seconds=(t_end - t_start)
                    )

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

                if exc.headers:
                    self.calibrator.calibrate_from_headers(
                        dict(exc.headers), local_monotonic=t_end, rtt_seconds=(t_end - t_start)
                    )

                err_body = exc.read().decode("utf-8", errors="replace")
                last_error_msg = f"HTTP {exc.code}: {err_body}"

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
        return await asyncio.to_thread(self.send_keepalive, access_token, model_id, project)

    async def trigger_keepalive(
        self,
        access_token: str,
        model_id: str = DEFAULT_WARMUP_MODEL_ID,
    ) -> bool:
        """PROJECT.md interface contract: returns True if keepalive succeeded."""
        res = await self.send_keepalive_async(access_token, model_id=model_id)
        return res.success

    def send_warmup_prompt(
        self,
        access_token: str,
        model_id: str = DEFAULT_WARMUP_MODEL_ID,
        project: str = "",
    ) -> WarmupResult:
        """Alias for send_keepalive."""
        return self.send_keepalive(access_token=access_token, model_id=model_id, project=project)


class WarmupScheduler:
    """
    Coordinates the ResetHorizonTracker and WarmupEngine inside the daemon.
    Polls active horizons and triggers keepalives as countdowns expire.
    """

    def __init__(
        self,
        tracker: ResetHorizonTracker,
        engine: WarmupEngine,
        token_provider: Callable[[str], Optional[str]],
        on_warmup_success: Optional[Callable[[BucketHorizon, WarmupResult], Any]] = None,
        on_warmup_failure: Optional[Callable[[BucketHorizon, WarmupResult], Any]] = None,
        check_interval_seconds: float = 1.0,
    ) -> None:
        self.tracker = tracker
        self.engine = engine
        self.token_provider = token_provider
        self.on_warmup_success = on_warmup_success
        self.on_warmup_failure = on_warmup_failure
        self.check_interval_seconds = check_interval_seconds
        self._running: bool = False
        self._task: Optional[asyncio.Task[None]] = None

    async def start(self) -> None:
        """Starts background monitoring task."""
        if self._running:
            return
        self._running = True
        self._task = asyncio.create_task(self._run_loop())

    async def stop(self) -> None:
        """Stops background monitoring task cleanly."""
        if not self._running:
            return
        self._running = False
        if self._task and not self._task.done():
            self._task.cancel()
            try:
                await self._task
            except asyncio.CancelledError:
                pass
        self._task = None

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
                            res = self.on_warmup_success(horizon, result)
                            if asyncio.iscoroutine(res):
                                await res
                    else:
                        self.tracker.mark_failed(horizon.account_email, horizon.bucket_id)
                        if self.on_warmup_failure:
                            res = self.on_warmup_failure(horizon, result)
                            if asyncio.iscoroutine(res):
                                await res

            except asyncio.CancelledError:
                break
            except Exception as exc:
                logger.error("Error in warmup scheduler tick: %s", exc)

            try:
                await asyncio.sleep(self.check_interval_seconds)
            except asyncio.CancelledError:
                break
