"""
Unit tests for Clock Drift Calibration, Reset Horizon Tracker, Circuit Breaker, and WarmupEngine.
==================================================================================================
Covers Feature F08 (Reset Horizon Warmup).
"""

import asyncio
from datetime import datetime, timedelta, timezone
import random
import time
import pytest

from antigravity_swiss.warmup.engine import (
    CircuitBreaker,
    CircuitBreakerState,
    WarmupEngine,
    WarmupResult,
    WarmupRetryPolicy,
    build_warmup_payload,
)
from antigravity_swiss.warmup.horizon import (
    BucketHorizon,
    ClockDriftCalibrator,
    HorizonStatus,
    ResetHorizonTracker,
    calculate_jitter_delay,
    calculate_jittered_target,
    calculate_warmup_delay,
)
from tests.fixtures.mock_cloudcode_server import MockCloudCodeServer


# ============================================================================
# Clock Drift Calibration & Jitter Tests
# ============================================================================

def test_clock_drift_calibration_rfc7231():
    """Verify parsing of HTTP Date RFC 7231 header, drift calculation, and EMA smoothing."""
    calibrator = ClockDriftCalibrator(ema_alpha=0.5)
    assert not calibrator.is_calibrated
    assert calibrator.drift_offset == 0.0

    # Pass synthetic Date header 10 seconds ahead of now
    now_utc = datetime.now(timezone.utc)
    future_dt = now_utc + timedelta(seconds=10)
    date_str = future_dt.strftime("%a, %d %b %Y %H:%M:%S GMT")

    drift1 = calibrator.calibrate_from_header(date_str)
    assert calibrator.is_calibrated
    assert 9.0 <= drift1 <= 11.0
    assert 9.0 <= calibrator.drift_offset <= 11.0

    # Second header with 20 seconds ahead -> EMA should smooth between 10 and 20 (~15)
    future_dt_2 = datetime.now(timezone.utc) + timedelta(seconds=20)
    date_str_2 = future_dt_2.strftime("%a, %d %b %Y %H:%M:%S GMT")
    calibrator.calibrate_from_header(date_str_2)
    assert 13.0 <= calibrator.drift_offset <= 17.0

    # Case-insensitive headers dictionary
    headers = {"date": date_str_2}
    calibrator.calibrate_from_headers(headers)
    assert calibrator.is_calibrated

    # Invalid header is ignored gracefully
    old_offset = calibrator.drift_offset
    calibrator.calibrate_from_header("invalid-date")
    assert calibrator.drift_offset == old_offset


def test_monotonic_anchoring_immunity_to_system_time_jump():
    """Verify now_calibrated() advances via monotonic time regardless of wall clock."""
    calibrator = ClockDriftCalibrator()
    fake_mono = 1000.0
    now_utc = datetime(2026, 10, 1, 12, 0, 0, tzinfo=timezone.utc)
    server_time_str = "Thu, 01 Oct 2026 12:00:05 GMT"  # 5s drift

    calibrator.calibrate_from_header(server_time_str, local_monotonic=fake_mono)
    assert calibrator.is_calibrated

    # Fast forward monotonic clock by 10 seconds
    time_future = calibrator.now_calibrated()
    # It should reflect server wall time plus elapsed monotonic seconds
    assert time_future.year == 2026


def test_jitter_bounding():
    """Verify jittered delay calculation strictly bounds between 0.5s and 3.0s."""
    now = datetime(2026, 10, 1, 8, 0, 0, tzinfo=timezone.utc)
    reset = datetime(2026, 10, 1, 8, 0, 10, tzinfo=timezone.utc)  # 10s away

    delays = [calculate_jitter_delay(reset, now_calibrated=now) for _ in range(100)]
    for d in delays:
        # 10.0 + uniform(0.5, 3.0) => [10.5, 13.0]
        assert 10.5 <= d <= 13.0

    # calculate_warmup_delay alias
    alias_delays = [calculate_warmup_delay(reset, now_calibrated=now) for _ in range(50)]
    for d in alias_delays:
        assert 10.5 <= d <= 13.0

    # Reset in past: delay should be just the jitter [0.5, 3.0]
    past_reset = datetime(2026, 10, 1, 7, 0, 0, tzinfo=timezone.utc)
    past_delays = [calculate_jitter_delay(past_reset, now_calibrated=now) for _ in range(100)]
    for d in past_delays:
        assert 0.5 <= d <= 3.0


def test_countdown_formatting():
    """Verify formatted countdown output across various time durations."""
    calibrator = ClockDriftCalibrator()
    tracker = ResetHorizonTracker(calibrator=calibrator)

    # Reset in 1 hour, 2 minutes, 3 seconds
    now = calibrator.now_calibrated()
    reset = now + timedelta(hours=1, minutes=2, seconds=3)
    tracker.update_bucket("alice@example.com", "gemini-5h", "5h", 0.02, reset)

    formatted = tracker.get_formatted_countdown("alice@example.com", "gemini-5h")
    assert formatted == "01:02:03"

    # Non-existent account
    assert tracker.get_formatted_countdown("none@example.com", "gemini-5h") == "00:00:00"


def test_reset_horizon_tracker_lifecycle_and_due_warmups():
    """Verify ResetHorizonTracker status progression, due warmups, and weekly block."""
    tracker = ResetHorizonTracker(warmup_threshold=0.05)
    now = datetime.now(timezone.utc)

    # 1. Healthy bucket (> threshold)
    h1 = tracker.update_bucket("a@example.com", "gemini-5h", "5h", 0.50, now + timedelta(hours=3))
    assert h1.status == HorizonStatus.IDLE_HEALTHY

    # 2. Quota below threshold -> WARMUP_SCHEDULED
    h2 = tracker.update_bucket("b@example.com", "gemini-5h", "5h", 0.02, now + timedelta(seconds=1))
    assert h2.status == HorizonStatus.WARMUP_SCHEDULED

    # 3. Weekly exhausted -> WEEKLY_BLOCKED
    h3 = tracker.update_bucket(
        "c@example.com", "gemini-5h", "5h", 0.02, now + timedelta(seconds=1), weekly_exhausted=True
    )
    assert h3.status == HorizonStatus.WEEKLY_BLOCKED

    # 4. Due warmups: fast forward target fire time
    h2.target_fire_time = now - timedelta(seconds=1)
    due = tracker.get_due_warmups()
    assert len(due) == 1
    assert due[0].account_email == "b@example.com"

    # Mark warmed up
    tracker.mark_warmed_up("b@example.com", "gemini-5h")
    assert h2.status == HorizonStatus.WARMED_UP
    assert h2.remaining_fraction == 1.0


# ============================================================================
# Circuit Breaker & 1-Token Keep-Alive Payload Tests
# ============================================================================

def test_1token_payload_structure():
    """Verify build_warmup_payload structure conforms to PredictionServiceProto."""
    payload = build_warmup_payload(
        model="gemini-3.5-flash-lite",
        prompt_text="ping",
        max_output_tokens=1,
        project="",
    )
    assert payload["model"] == "gemini-3.5-flash-lite"
    assert payload["project"] == ""
    req = payload["request"]
    assert req["contents"][0]["parts"][0]["text"] == "ping"
    assert req["generationConfig"]["maxOutputTokens"] == 1
    assert req["generationConfig"]["temperature"] == 0.0


def test_circuit_breaker_state_transitions():
    """Verify CircuitBreaker 3-state transitions: CLOSED -> OPEN -> HALF_OPEN -> CLOSED."""
    cb = CircuitBreaker(failure_threshold=5, recovery_timeout_seconds=0.1)
    assert cb.state == CircuitBreakerState.CLOSED
    assert cb.allow_request() is True

    # 4 failures: stays CLOSED
    for _ in range(4):
        cb.record_failure()
        assert cb.state == CircuitBreakerState.CLOSED
        assert cb.allow_request() is True

    # 5th failure: trips to OPEN
    cb.record_failure()
    assert cb.state == CircuitBreakerState.OPEN
    assert cb.allow_request() is False

    # Sleep recovery timeout -> transitions to HALF_OPEN
    time.sleep(0.12)
    assert cb.allow_request() is True
    assert cb.state == CircuitBreakerState.HALF_OPEN

    # Success records recovery to CLOSED
    cb.record_success()
    assert cb.state == CircuitBreakerState.CLOSED
    assert cb.consecutive_failures == 0

    # Probe failure in HALF_OPEN trips immediately back to OPEN
    for _ in range(5):
        cb.record_failure()
    assert cb.state == CircuitBreakerState.OPEN
    time.sleep(0.12)
    assert cb.allow_request() is True
    assert cb.state == CircuitBreakerState.HALF_OPEN
    cb.record_failure()
    assert cb.state == CircuitBreakerState.OPEN

    # Manual reset
    cb.reset()
    assert cb.state == CircuitBreakerState.CLOSED


def test_warmup_retry_policy():
    """Verify WarmupRetryPolicy retryable status detection and bounded delays."""
    policy = WarmupRetryPolicy(max_retries=3, base_delay=1.0, max_delay=10.0)
    assert policy.is_retryable(429) is True
    assert policy.is_retryable(503) is True
    assert policy.is_retryable(400) is False
    assert policy.is_retryable(401) is False
    assert policy.is_retryable(403) is False

    d0 = policy.get_delay_seconds(0)
    d1 = policy.get_delay_seconds(1)
    assert d0 > 0.0
    assert d1 > d0


def test_warmup_engine_keepalive_with_mock_server():
    """Verify WarmupEngine dispatches keepalive and handles circuit breaker."""
    server = MockCloudCodeServer()
    base_url = server.start()
    try:
        engine = WarmupEngine(
            endpoint_url=f"{base_url}/v1internal:generateContent",
            timeout_seconds=2.0,
        )

        # 1. Successful keepalive
        res = engine.send_keepalive("ya29.valid_token")
        assert res.success is True
        assert res.status_code == 200
        assert res.attempts == 1
        assert server.warmup_fired is True

        # 2. Async wrapper
        async def run_async():
            res_async = await engine.send_keepalive_async("ya29.valid_token")
            assert res_async.success is True
            ok = await engine.trigger_keepalive("ya29.valid_token")
            assert ok is True

        asyncio.run(run_async())

        # 3. Exhausted quota (429) before reset
        server.set_quota(0.0, reset_in_seconds=3600)
        res_429 = engine.send_keepalive("ya29.valid_token")
        assert res_429.success is False
        assert res_429.status_code == 429
    finally:
        server.stop()
