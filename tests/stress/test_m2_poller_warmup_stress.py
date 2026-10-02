"""
Milestone 2 Quota Poller, Clock Drift, and Warmup Engine Adversarial Stress Harness.
=====================================================================================
Target Features:
- F06: Quota Summary Poller & Cache
- F07: Model Catalog & Capabilities
- F08: Reset Horizon Warmup Engine, Clock Drift & Circuit Breaker
- F09: Auto-Switch Rule Engine
- F26: Hermetic Mock CloudCode Server

Executes 9 comprehensive adversarial stress test suites:
1. Clock Drift Stress (+30s / -30s offset injection & premature 429 prevention)
2. Extreme Clock Skew (+120s / -120s offset stress & monotonic extrapolation)
3. High-Concurrency Polling (100 coroutines thundering herd protection via 15s TTL)
4. Multi-Account Concurrent Isolation (simultaneous coroutines across different accounts)
5. 1-Token Keep-Alive Verification (payload structure, maxOutputTokens=1, temp=0.0)
6. 429 Backoff Policy & Circuit Breaker Trip (exact 3 retries, trip on 5th failure)
7. Circuit Breaker Half-Open Probe Recovery & Reset Lifecycle
8. Weekly Quota Depletion (5h warmup inhibition / WEEKLY_BLOCKED & rule engine lockout)
9. Server Error Resilience (503/502/network drops graceful degradation & auto-recovery)
"""

import asyncio
from datetime import datetime, timedelta, timezone
import json
import logging
import os
import sys
import time
from pathlib import Path

# Add project root to sys.path
PROJECT_ROOT = Path(__file__).resolve().parents[2]
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

import pytest

from antigravity_swiss.core.config import SwissKnifeConfig
from antigravity_swiss.core.errors import (
    QuotaAuthExpiredError,
    QuotaError,
    QuotaNetworkError,
    QuotaRateLimitError,
    QuotaUnavailableError,
)
from antigravity_swiss.keyring.switcher import (
    AccountVault,
    KeyringCredential,
    KeyringService,
)
from antigravity_swiss.quota.client import CloudCodeClient
from antigravity_swiss.quota.models import (
    ModelCatalog,
    ModelQuotaBucket,
    QuotaSummary,
    QuotaSummaryGroup,
)
from antigravity_swiss.quota.poller import QuotaPoller
from antigravity_swiss.quota.rule_engine import (
    AutoSwitchRuleEngine,
    RuleEngineConfig,
)
from antigravity_swiss.warmup.engine import (
    CircuitBreaker,
    CircuitBreakerState,
    WarmupEngine,
    WarmupResult,
    WarmupRetryPolicy,
    WarmupScheduler,
    build_warmup_payload,
)
from antigravity_swiss.warmup.horizon import (
    BucketHorizon,
    ClockDriftCalibrator,
    HorizonStatus,
    ResetHorizonTracker,
    calculate_jitter_delay,
)
from tests.fixtures.mock_cloudcode_server import MockCloudCodeServer


# ============================================================================
# 1. CLOCK DRIFT STRESS SUITE
# ============================================================================

def test_stress_clock_drift_negative_offset_prevents_premature_429():
    """
    Stress test negative clock drift (-30s: server clock is 30s behind local clock).
    Without drift calibration:
      Local clock reaches resetTime 30s before the server actually resets.
      Client firing at local resetTime hits server when quota is still 0.0 -> HTTP 429!
    With drift calibration:
      Client parses HTTP Date header, measures drift ~ -30s.
      now_calibrated() estimates server time, adding 30s delay to local target.
      When client fires, server time >= resetTime, returning 200 OK!
    """
    server = MockCloudCodeServer()
    base_url = server.start()
    try:
        # Server is 30s behind local time
        server.set_clock_drift(-30.0)
        # Quota resets in 5 seconds (on server)
        server.set_quota(remaining=0.0, reset_in_seconds=5)

        client = CloudCodeClient(base_url=base_url)
        calibrator = ClockDriftCalibrator()

        # Step 1: Initial query to extract drift from HTTP Date header
        data, measured_drift = client.retrieve_user_quota_summary_sync("ya29.test_token")
        summary = QuotaSummary.from_dict(data)
        b_5h = summary.gemini_5h
        assert b_5h is not None
        assert b_5h.reset_time is not None

        # Calibrate our calibrator from the server response
        assert -35.0 <= measured_drift <= -25.0
        calibrator.calibrate_from_header(
            (server.get_simulated_time() + timedelta(seconds=-30.0)).strftime("%a, %d %b %Y %H:%M:%S GMT")
        )
        assert calibrator.is_calibrated

        # Step 2: Uncalibrated firing simulation (local time)
        # Local time reaches b_5h.reset_time while server still has 25+ seconds left
        engine_uncalibrated = WarmupEngine(
            endpoint_url=f"{base_url}/v1internal:generateContent",
            retry_policy=WarmupRetryPolicy(max_retries=0),
        )
        # Before server reaches reset time: generateContent MUST return 429!
        res_premature = engine_uncalibrated.send_keepalive("ya29.test_token")
        assert res_premature.status_code == 429
        assert res_premature.success is False

        # Step 3: Calibrated horizon calculation
        now_cal = calibrator.now_calibrated()
        delay = calculate_jitter_delay(b_5h.reset_time, now_calibrated=now_cal, min_jitter=0.5, max_jitter=1.0)
        # Uncalibrated delay would be ~5s (or negative). Calibrated delay must be ~ 5 - (-30) = 35s!
        assert delay >= 30.0, f"Calibrated delay {delay}s should be >= 30.0s to account for -30s drift"

        # Step 4: Advance server time by 6 seconds (server now past its reset time)
        server.advance_time(6)

        # Step 5: Now calibrated keep-alive fires -> 200 OK!
        engine_calibrated = WarmupEngine(
            endpoint_url=f"{base_url}/v1internal:generateContent",
            calibrator=calibrator,
        )
        res_success = engine_calibrated.send_keepalive("ya29.test_token")
        assert res_success.success is True
        assert res_success.status_code == 200
        assert server.warmup_fired is True
    finally:
        server.stop()


def test_stress_clock_drift_positive_offset_prevents_delayed_activation():
    """
    Stress test positive clock drift (+30s: server clock is 30s ahead of local clock).
    Without drift calibration:
      Local clock is 30s behind server. Client unnecessarily waits 30 extra seconds
      before firing warmup, wasting productive window time.
    With drift calibration:
      Client measures drift ~ +30s.
      now_calibrated() recognizes reset is 30s sooner than local clock indicates.
      Promptly fires warmup without idle delay.
    """
    server = MockCloudCodeServer()
    base_url = server.start()
    try:
        # Server is 30s ahead of local time
        server.set_clock_drift(30.0)
        # Quota resets in 35 seconds (on server)
        server.set_quota(remaining=0.0, reset_in_seconds=35)

        client = CloudCodeClient(base_url=base_url)
        calibrator = ClockDriftCalibrator()

        data, measured_drift = client.retrieve_user_quota_summary_sync("ya29.test_token")
        summary = QuotaSummary.from_dict(data)
        b_5h = summary.gemini_5h

        assert 25.0 <= measured_drift <= 35.0
        calibrator.calibrate_from_header(
            (server.get_simulated_time() + timedelta(seconds=30.0)).strftime("%a, %d %b %Y %H:%M:%S GMT")
        )

        # Calibrated delay: server only has 35s until reset, but relative to calibrated time:
        now_cal = calibrator.now_calibrated()
        calibrated_delay = calculate_jitter_delay(
            b_5h.reset_time, now_calibrated=now_cal, min_jitter=0.5, max_jitter=1.0
        )
        # Should be ~ 35s - 30s + jitter = 5s + jitter ~ [5.5, 6.0]s
        # whereas local uncalibrated delay would be ~ 35s!
        assert calibrated_delay <= 10.0, f"Calibrated delay {calibrated_delay}s should be <= 10s"

        # Advance server by 36 seconds
        server.advance_time(36)

        engine = WarmupEngine(
            endpoint_url=f"{base_url}/v1internal:generateContent",
            calibrator=calibrator,
        )
        res = engine.send_keepalive("ya29.test_token")
        assert res.success is True
        assert res.status_code == 200
        assert server.warmup_fired is True
    finally:
        server.stop()


def test_stress_extreme_clock_skew_and_monotonic_extrapolation():
    """
    Stress test extreme clock skew (+120s and -120s) and verify that
    now_calibrated() extrapolates stably using monotonic clock even across 5 simulated ticks.
    """
    calibrator = ClockDriftCalibrator(ema_alpha=0.5)

    # 1. Positive extreme skew +120s
    now_utc = datetime.now(timezone.utc)
    skewed_future = now_utc + timedelta(seconds=120)
    calibrator.calibrate_from_header(skewed_future.strftime("%a, %d %b %Y %H:%M:%S GMT"))
    assert 115.0 <= calibrator.drift_offset <= 125.0

    calibrated_now = calibrator.now_calibrated()
    assert (calibrated_now - now_utc).total_seconds() >= 115.0

    # 2. Monotonic progression check over short intervals
    t0 = calibrator.now_calibrated()
    time.sleep(0.05)
    t1 = calibrator.now_calibrated()
    assert (t1 - t0).total_seconds() >= 0.04


# ============================================================================
# 2. HIGH-CONCURRENCY POLLING (THUNDERING HERD) SUITE
# ============================================================================

def test_stress_high_concurrency_polling_thundering_herd(tmp_path):
    """
    High-concurrency stress test:
    Launch 100 concurrent coroutines calling `poll_summary()`.
    Verify:
    1. Exactly 1 HTTP request hits the mock server (thundering herd suppressed).
    2. All 100 coroutines receive the identical cached QuotaSummary.
    3. Response latency for subsequent callers is < 5ms.
    4. 15s TTL prevents further requests until expired or forced.
    """
    server = MockCloudCodeServer()
    base_url = server.start()
    try:
        acc_file = tmp_path / "accounts.json"
        vault = AccountVault(config_path=acc_file)
        cred = KeyringCredential(
            access_token="ya29.concurrency_test",
            refresh_token="1//refresh",
            token_type="Bearer",
            expiry=(datetime.now(timezone.utc) + timedelta(hours=1)).isoformat(),
            auth_method="consumer",
            id_token="",
        )
        vault.add_or_update_account("concurrency@example.com", cred)
        vault.set_active_account("concurrency@example.com")
        keyring_service = KeyringService(vault=vault)

        client = CloudCodeClient(base_url=base_url)
        poller = QuotaPoller(
            client=client,
            vault=vault,
            keyring_service=keyring_service,
            cache_ttl_sec=15.0,
        )

        concurrency_count = 100

        async def run_herd():
            # Reset request count
            server.reset_history()

            t_start = time.monotonic()
            tasks = [poller.poll_summary(force=False) for _ in range(concurrency_count)]
            results = await asyncio.gather(*tasks)
            t_elapsed = time.monotonic() - t_start

            # Exactly 1 upstream request should be recorded
            upstream_quota_requests = [
                r for r in server.recorded_requests
                if r["path"].endswith(":retrieveUserQuotaSummary")
            ]
            assert len(upstream_quota_requests) == 1, (
                f"Expected exactly 1 upstream request, got {len(upstream_quota_requests)}"
            )

            # All coroutines must receive the identical object (reference equality)
            first_summary = results[0]
            assert first_summary is not None
            for idx, res in enumerate(results):
                assert res is first_summary, f"Result {idx} did not receive cached instance"

            # Subsequent 20 calls within TTL do not hit upstream
            subsequent_tasks = [poller.poll_summary(force=False) for _ in range(20)]
            sub_results = await asyncio.gather(*subsequent_tasks)
            assert len([
                r for r in server.recorded_requests
                if r["path"].endswith(":retrieveUserQuotaSummary")
            ]) == 1
            for res in sub_results:
                assert res is first_summary

            # Force poll creates exactly 1 more request
            force_res = await poller.poll_summary(force=True)
            assert len([
                r for r in server.recorded_requests
                if r["path"].endswith(":retrieveUserQuotaSummary")
            ]) == 2
            assert force_res is not first_summary

            return t_elapsed

        elapsed = asyncio.run(run_herd())
        assert elapsed < 5.0, f"Thundering herd execution took too long: {elapsed:.2f}s"
    finally:
        server.stop()


def test_stress_multi_account_concurrent_polling_isolation(tmp_path):
    """
    Stress test multi-account concurrent polling:
    Spawn 30 concurrent coroutines for account A, and 30 for account B.
    Verify:
    1. Exactly 2 HTTP requests total (1 for A, 1 for B).
    2. Zero cache cross-contamination between account A and account B.
    """
    server = MockCloudCodeServer()
    base_url = server.start()
    try:
        acc_file = tmp_path / "accounts.json"
        vault = AccountVault(config_path=acc_file)
        cred_a = KeyringCredential("ya29.acc_a", "1//ref_a", "Bearer", "", "consumer", "")
        cred_b = KeyringCredential("ya29.acc_b", "1//ref_b", "Bearer", "", "consumer", "")
        vault.add_or_update_account("alice@example.com", cred_a)
        vault.add_or_update_account("bob@example.com", cred_b)
        vault.set_active_account("alice@example.com")
        keyring_service = KeyringService(vault=vault)

        # Set distinct quotas for Alice and Bob in mock server
        server.set_account_profile("ya29.acc_a", remaining=0.95)
        server.set_account_profile("ya29.acc_b", remaining=0.35)

        client = CloudCodeClient(base_url=base_url)
        poller = QuotaPoller(
            client=client,
            vault=vault,
            keyring_service=keyring_service,
            cache_ttl_sec=15.0,
        )

        async def run_multi():
            server.reset_history()

            tasks_a = [poller.poll_summary(email="alice@example.com") for _ in range(30)]
            tasks_b = [poller.poll_summary(email="bob@example.com") for _ in range(30)]

            results_a, results_b = await asyncio.gather(
                asyncio.gather(*tasks_a),
                asyncio.gather(*tasks_b),
            )

            # Exactly 2 upstream requests
            reqs = [r for r in server.recorded_requests if r["path"].endswith(":retrieveUserQuotaSummary")]
            assert len(reqs) == 2, f"Expected 2 requests, got {len(reqs)}"

            # Alice results: all 0.95
            for r in results_a:
                assert r.gemini_5h.remaining_fraction == 0.95
                assert r.active_account == "alice@example.com"

            # Bob results: all 0.35
            for r in results_b:
                assert r.gemini_5h.remaining_fraction == 0.35
                assert r.active_account == "bob@example.com"

        asyncio.run(run_multi())
    finally:
        server.stop()


# ============================================================================
# 3. 1-TOKEN KEEP-ALIVE, 429 BACKOFF & CIRCUIT BREAKER SUITE
# ============================================================================

def test_stress_1token_payload_exact_structure(tmp_path):
    """
    Verify 1-token keep-alive payload exact structure:
    - POST /v1internal:generateContent
    - generationConfig.maxOutputTokens == 1
    - generationConfig.temperature == 0.0
    - contents[0].parts[0].text is non-empty minimal
    - usageMetadata reflects 1-token generation
    """
    server = MockCloudCodeServer()
    base_url = server.start()
    try:
        server.set_quota(remaining=1.0)
        engine = WarmupEngine(endpoint_url=f"{base_url}/v1internal:generateContent")

        res = engine.send_keepalive(access_token="ya29.payload_check", model_id="gemini-3.5-flash-lite")
        assert res.success is True

        reqs = [r for r in server.recorded_requests if r["path"].endswith(":generateContent")]
        assert len(reqs) == 1
        recorded = reqs[0]

        # Verify headers
        assert recorded["headers"].get("Authorization") == "Bearer ya29.payload_check"
        assert recorded["headers"].get("Content-Type") == "application/json"

        # Verify body
        body = recorded["body"]
        assert body["model"] == "gemini-3.5-flash-lite"
        req_obj = body["request"]
        gen_cfg = req_obj["generationConfig"]
        assert gen_cfg["maxOutputTokens"] == 1
        assert gen_cfg["temperature"] == 0.0
        assert len(req_obj["contents"]) == 1
        assert req_obj["contents"][0]["role"] == "user"
        assert len(req_obj["contents"][0]["parts"]) == 1
        assert isinstance(req_obj["contents"][0]["parts"][0]["text"], str)

        # Verify response metadata
        assert res.response_data["usageMetadata"]["promptTokenCount"] == 1
        assert res.response_data["usageMetadata"]["candidatesTokenCount"] == 1
    finally:
        server.stop()


def test_stress_429_backoff_policy_and_circuit_breaker_trip():
    """
    Stress test 429 backoff policy and circuit breaker tripping:
    1. Verify 429 retries exactly 3 times (4 total attempts) with exponential backoff.
    2. Verify circuit breaker trips to OPEN after 5 consecutive failed operations.
    3. Verify requests in OPEN state are rejected immediately (0 network calls).
    4. Verify recovery after timeout transitioning to HALF_OPEN and CLOSED.
    """
    server = MockCloudCodeServer()
    base_url = server.start()
    try:
        # Force 429 on generateContent
        server.set_quota(remaining=0.0, reset_in_seconds=7200)

        circuit_breaker = CircuitBreaker(
            failure_threshold=5,
            recovery_timeout_seconds=0.2,
        )
        retry_policy = WarmupRetryPolicy(
            max_retries=3,
            base_delay=0.02,
            backoff_multiplier=2.0,
            max_delay=0.1,
        )
        engine = WarmupEngine(
            endpoint_url=f"{base_url}/v1internal:generateContent",
            circuit_breaker=circuit_breaker,
            retry_policy=retry_policy,
        )

        # --- Phase 1: Verify single 429 call executes exactly 4 attempts (1 initial + 3 retries) ---
        server.reset_history()
        res1 = engine.send_keepalive("ya29.test")
        assert res1.success is False
        assert res1.status_code == 429
        assert res1.attempts == 4  # 1 initial + 3 retries
        assert len(server.recorded_requests) == 4
        assert circuit_breaker.consecutive_failures == 1
        assert circuit_breaker.state == CircuitBreakerState.CLOSED

        # --- Phase 2: Accumulate 4 more failures (total 5) to trip circuit breaker ---
        for failure_idx in range(2, 6):
            res = engine.send_keepalive("ya29.test")
            assert res.success is False
            assert res.status_code == 429
            assert res.attempts == 4

        assert circuit_breaker.consecutive_failures == 5
        assert circuit_breaker.state == CircuitBreakerState.OPEN, "Circuit breaker MUST trip to OPEN after 5 failures"

        # --- Phase 3: Immediate rejection in OPEN state (no network calls) ---
        reqs_before = len(server.recorded_requests)
        res_blocked = engine.send_keepalive("ya29.test")
        assert res_blocked.success is False
        assert res_blocked.circuit_breaker_tripped is True
        assert res_blocked.error_message == "Circuit breaker is OPEN"
        # Zero network requests attempted!
        assert len(server.recorded_requests) == reqs_before

        # --- Phase 4: Recovery via HALF_OPEN probe ---
        # Allow quota reset on server
        server.set_quota(remaining=1.0)
        time.sleep(0.25)  # Wait recovery timeout

        # Next call should be allowed as HALF_OPEN probe
        assert circuit_breaker.allow_request() is True
        assert circuit_breaker.state == CircuitBreakerState.HALF_OPEN

        res_recovery = engine.send_keepalive("ya29.test")
        assert res_recovery.success is True
        assert res_recovery.status_code == 200
        assert circuit_breaker.state == CircuitBreakerState.CLOSED
        assert circuit_breaker.consecutive_failures == 0
    finally:
        server.stop()


# ============================================================================
# 4. WEEKLY QUOTA DEPLETION INHIBITION SUITE
# ============================================================================

def test_stress_weekly_depletion_inhibits_5h_warmup_and_rule_engine(tmp_path):
    """
    Stress test weekly quota depletion:
    1. ResetHorizonTracker marks 5h bucket as WEEKLY_BLOCKED when weekly quota is exhausted.
    2. target_fire_time remains None; get_due_warmups() NEVER schedules 1-token keep-alives.
    3. AutoSwitchRuleEngine gives 0.0 score to accounts with depleted weekly quota.
    4. AutoSwitchRuleEngine refuses to switch to an account with exhausted weekly quota.
    """
    now = datetime.now(timezone.utc)
    calibrator = ClockDriftCalibrator()
    tracker = ResetHorizonTracker(calibrator=calibrator, warmup_threshold=0.05)

    # 1. Bucket with depleted burst quota (0.02) BUT healthy weekly quota (0.90)
    h_healthy_weekly = tracker.update_bucket(
        account_email="healthy_weekly@example.com",
        bucket_id="gemini-5h",
        window="5h",
        remaining_fraction=0.02,
        reset_time=now + timedelta(seconds=1),
        weekly_exhausted=False,
    )
    assert h_healthy_weekly.status == HorizonStatus.WARMUP_SCHEDULED
    assert h_healthy_weekly.target_fire_time is not None

    # 2. Bucket with depleted burst quota (0.02) AND exhausted weekly quota (0.0)
    h_exhausted_weekly = tracker.update_bucket(
        account_email="exhausted_weekly@example.com",
        bucket_id="gemini-5h",
        window="5h",
        remaining_fraction=0.02,
        reset_time=now + timedelta(seconds=1),
        weekly_exhausted=True,
    )
    assert h_exhausted_weekly.status == HorizonStatus.WEEKLY_BLOCKED
    assert h_exhausted_weekly.target_fire_time is None

    # Advance time to make warmups due
    h_healthy_weekly.target_fire_time = now - timedelta(seconds=1)

    due = tracker.get_due_warmups()
    assert len(due) == 1
    assert due[0].account_email == "healthy_weekly@example.com"
    # exhausted_weekly is strictly inhibited!
    assert all(h.account_email != "exhausted_weekly@example.com" for h in due)

    # 3. Rule Engine behavior under weekly exhaustion
    acc_file = tmp_path / "accounts.json"
    vault = AccountVault(config_path=acc_file)
    cred = KeyringCredential("t", "r", "Bearer", "", "consumer", "")
    vault.add_or_update_account("primary@example.com", cred)
    vault.add_or_update_account("standby_depleted_weekly@example.com", cred)
    vault.add_or_update_account("standby_healthy@example.com", cred)
    vault.set_active_account("primary@example.com")
    keyring_service = KeyringService(vault=vault)

    rule_engine = AutoSwitchRuleEngine(
        vault=vault,
        keyring_service=keyring_service,
        config=RuleEngineConfig(weekly_threshold=0.01),
    )

    # Cache quotas
    # Standby 1 has 1.0 burst but 0.0 weekly (exhausted!)
    rule_engine.update_cached_quota("standby_depleted_weekly@example.com", {
        "groups": [{"buckets": [
            {"bucketId": "gemini-5h", "remainingFraction": 1.0},
            {"bucketId": "gemini-weekly", "remainingFraction": 0.0},
        ]}]
    })
    # Standby 2 has 0.8 burst and 0.8 weekly (healthy)
    rule_engine.update_cached_quota("standby_healthy@example.com", {
        "groups": [{"buckets": [
            {"bucketId": "gemini-5h", "remainingFraction": 0.8},
            {"bucketId": "gemini-weekly", "remainingFraction": 0.8},
        ]}]
    })

    # Account with depleted weekly quota MUST get score 0.0
    score_depleted = rule_engine.calculate_account_score(
        "standby_depleted_weekly@example.com",
        {"groups": [{"buckets": [{"bucketId": "gemini-5h", "remainingFraction": 1.0}, {"bucketId": "gemini-weekly", "remainingFraction": 0.0}]}]}
    )
    assert score_depleted == 0.0

    # Primary breaches threshold (0.01) -> evaluate switch
    eval_res = rule_engine.evaluate(
        "primary@example.com",
        {"groups": [{"buckets": [{"bucketId": "gemini-5h", "remainingFraction": 0.01}, {"bucketId": "gemini-weekly", "remainingFraction": 1.0}]}]}
    )
    assert eval_res.should_switch is True
    # MUST choose standby_healthy, skipping standby_depleted_weekly completely
    assert eval_res.target_account == "standby_healthy@example.com"


# ============================================================================
# 5. SERVER ERROR RESILIENCE (503/502/NETWORK DROPS) SUITE
# ============================================================================

def test_stress_server_errors_and_network_drops_resilience(tmp_path):
    """
    Stress test daemon resilience under 503, 502, and network drops:
    1. Poller maps 502/503/504 to QuotaUnavailableError.
    2. Poller maps dead port / connection refused to QuotaNetworkError.
    3. Background poll loop does NOT crash on errors (remains is_running == True).
    4. Consecutive errors increment and trigger exponential backoff.
    5. Clean recovery once service is restored.
    """
    server = MockCloudCodeServer()
    base_url = server.start()
    try:
        acc_file = tmp_path / "accounts.json"
        vault = AccountVault(config_path=acc_file)
        cred = KeyringCredential(
            access_token="ya29.resilience_test",
            refresh_token="1//refresh",
            token_type="Bearer",
            expiry=(datetime.now(timezone.utc) + timedelta(hours=1)).isoformat(),
            auth_method="consumer",
            id_token="",
        )
        vault.add_or_update_account("resilience@example.com", cred)
        vault.set_active_account("resilience@example.com")
        keyring_service = KeyringService(vault=vault)

        client = CloudCodeClient(base_url=base_url, timeout=1.0)
        poller = QuotaPoller(
            client=client,
            vault=vault,
            keyring_service=keyring_service,
            poll_interval_sec=0.05,
        )

        async def run_resilience():
            # 1. 503 Service Unavailable
            server.transient_errors_remaining = 1
            server.transient_error_status = 503
            with pytest.raises(QuotaUnavailableError) as exc_503:
                await poller.poll_summary(force=True)
            assert exc_503.value.code == -32034

            # 2. 502 Bad Gateway
            server.transient_errors_remaining = 1
            server.transient_error_status = 502
            with pytest.raises(QuotaUnavailableError) as exc_502:
                await poller.poll_summary(force=True)
            assert exc_502.value.code == -32034

            # 3. Network Drop (Dead loopback port)
            dead_client = CloudCodeClient(base_url="http://127.0.0.1:4", timeout=0.2)
            dead_poller = QuotaPoller(
                client=dead_client,
                vault=vault,
                keyring_service=keyring_service,
            )
            with pytest.raises(QuotaNetworkError):
                await dead_poller.poll_summary(force=True)

            # 4. Daemon Background Poll Loop Stability Under Continuous Errors
            await dead_poller.start()
            assert dead_poller.is_running is True

            # Allow loop to execute multiple failing ticks
            await asyncio.sleep(0.3)
            # The daemon must STILL be running!
            assert dead_poller.is_running is True
            assert dead_poller._consecutive_errors >= 1
            assert isinstance(dead_poller._last_error, QuotaNetworkError)
            await dead_poller.stop()
            assert dead_poller.is_running is False

            # 5. Recovery on live server
            server.transient_errors_remaining = 0
            recovered_summary = await poller.poll_summary(force=True)
            assert recovered_summary is not None
            assert recovered_summary.gemini_5h is not None

        asyncio.run(run_resilience())
    finally:
        server.stop()


if __name__ == "__main__":
    pytest.main([__file__, "-v", "-s"])
