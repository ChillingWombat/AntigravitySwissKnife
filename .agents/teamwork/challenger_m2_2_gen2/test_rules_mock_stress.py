"""
Milestone 2 Auto-Switch Rule Engine, Mock CloudCode Server & IPC Stress Harness.
================================================================================
Adversarial Stress Test Suite:
1. Thrashing simulation: 3 accounts rapidly draining to near-threshold (0.04, 0.05, 0.06);
   verifies 300s cooldown and 0.05 margin completely prevent infinite switch loops.
2. All accounts exhausted: drain all accounts to 0.0; verifies engine transitions to
   standby state without unhandled exceptions, emits `notify.all_accounts_exhausted` via
   Unix Domain Socket broadcast, and calculates nearest reset horizon.
3. Multi-account mock server profile isolation: verifies accounts with separate tokens
   receive independent quota responses without cross-account contamination, under
   high-concurrency request bombardment.
4. Rapid IPC requests: fires rapid `quota.poll_now` and `rules.set_config` calls across
   Unix Domain Socket with concurrent workers, verifying concurrency safety.
5. Boundary and edge cases: malformed quota payloads, missing buckets, all accounts
   unhealthy, out-of-range thresholds, and rate limiting guardrails.
"""

import asyncio
from datetime import datetime, timedelta, timezone
import json
import os
from pathlib import Path
import sys
import tempfile
import time
from typing import Any, Dict, List

import pytest

# Ensure project root is in path
PROJECT_ROOT = Path(__file__).resolve().parents[3]
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

from antigravity_swiss.core.config import SwissKnifeConfig
from antigravity_swiss.core.constants import DEFAULT_AUTO_SWITCH_THRESHOLD_FRACTION
from antigravity_swiss.core.errors import QuotaError, SwissKnifeError
from antigravity_swiss.ipc.socket_server import AsyncUnixSocketServer
from antigravity_swiss.ipc.controller import RemoteDaemonController, SwissKnifeController
from antigravity_swiss.keyring.switcher import (
    AccountRecord,
    AccountVault,
    KeyringCredential,
    KeyringService,
)
from antigravity_swiss.quota.client import CloudCodeClient
from antigravity_swiss.quota.models import (
    ModelQuotaBucket,
    QuotaSummary,
    QuotaSummaryGroup,
)
from antigravity_swiss.quota.poller import QuotaPoller
from antigravity_swiss.quota.rule_engine import (
    AutoSwitchRuleEngine,
    RuleEngineConfig,
    EvaluationResult,
)
from antigravity_swiss.warmup.horizon import (
    BucketHorizon,
    ClockDriftCalibrator,
    HorizonStatus,
    ResetHorizonTracker,
)
from tests.fixtures.mock_cloudcode_server import MockCloudCodeServer


def _make_dummy_quota(
    fraction: float,
    weekly: float = 1.0,
    reset_time: datetime | None = None,
) -> dict:
    """Helper to construct standard CloudCode quota response dictionary."""
    reset_str = (
        (reset_time or (datetime.now(timezone.utc) + timedelta(hours=3))).strftime("%Y-%m-%dT%H:%M:%SZ")
    )
    return {
        "groups": [
            {
                "buckets": [
                    {
                        "bucketId": "gemini-5h",
                        "remainingFraction": fraction,
                        "resetTime": reset_str,
                    },
                    {
                        "bucketId": "gemini-weekly",
                        "remainingFraction": weekly,
                        "resetTime": reset_str,
                    },
                ]
            }
        ]
    }


# ============================================================================
# CHALLENGE 1: THRASHING SIMULATION & COOLDOWN / MARGIN HYSTERESIS
# ============================================================================

def test_stress_anti_thrashing_cooldown_and_margin(tmp_path):
    """
    Challenge 1: Thrashing simulation.
    3 accounts rapidly draining to near-threshold (0.04, 0.05, 0.06).
    Verify 300s cooldown and 0.05 margin completely prevent infinite switch loops.
    """
    acc_file = tmp_path / "accounts.json"
    vault = AccountVault(config_path=acc_file)
    cred = KeyringCredential("token", "ref", "Bearer", "", "consumer", "")

    vault.add_or_update_account("acc_a@example.com", cred)
    vault.add_or_update_account("acc_b@example.com", cred)
    vault.add_or_update_account("acc_c@example.com", cred)
    vault.set_active_account("acc_a@example.com")
    keyring_service = KeyringService(vault=vault)

    clock_time = 1000.0
    cfg = RuleEngineConfig(
        default_threshold=0.05,
        switch_margin=0.05,
        cooldown_seconds=300.0,
        max_switches_in_window=10,
    )
    engine = AutoSwitchRuleEngine(
        vault=vault,
        keyring_service=keyring_service,
        config=cfg,
        clock_fn=lambda: clock_time,
    )

    # acc_b is at 0.06 (above threshold 0.05, but below threshold + margin = 0.10)
    engine.update_cached_quota("acc_b@example.com", _make_dummy_quota(0.06))
    # acc_c is at 0.15 (healthy, > 0.10)
    engine.update_cached_quota("acc_c@example.com", _make_dummy_quota(0.15))

    # 1. Evaluate active account A at 0.04 (breached <= 0.05)
    res1 = engine.evaluate("acc_a@example.com", _make_dummy_quota(0.04))
    assert res1.should_switch is True
    # Must pick C, because B does not meet threshold + margin (0.05 + 0.05 = 0.10)
    assert res1.target_account == "acc_c@example.com"
    assert res1.current_fraction == 0.04

    # Record switch from A to C
    engine.record_switch("acc_a@example.com", "acc_c@example.com")
    vault.set_active_account("acc_c@example.com")

    # 2. Rapid second evaluation at t=1010s: C immediately drops to 0.04
    clock_time = 1010.0
    res2 = engine.evaluate("acc_c@example.com", _make_dummy_quota(0.04))
    # A is in cooldown (switched out 10s ago < 300s)
    # B is at 0.06 (below threshold + margin 0.10)
    # Result: infinite loop prevented! No switch allowed!
    assert res2.should_switch is False
    assert res2.target_account is None
    assert res2.all_exhausted is True

    # 3. Simulate oscillation attempt at t=1100s: C drops further to 0.02
    clock_time = 1100.0
    res3 = engine.evaluate("acc_c@example.com", _make_dummy_quota(0.02))
    assert res3.should_switch is False
    assert res3.target_account is None

    # 4. Advance time past 300s cooldown (t = 1305s > 1000 + 300)
    clock_time = 1305.0
    # If A has NOT recovered (still at 0.04), A is STILL rejected (fails margin)
    engine.update_cached_quota("acc_a@example.com", _make_dummy_quota(0.04))
    res4 = engine.evaluate("acc_c@example.com", _make_dummy_quota(0.02))
    assert res4.should_switch is False

    # 5. If A HAS recovered (now at 0.80), cooldown is expired -> switch to A is permitted
    engine.update_cached_quota("acc_a@example.com", _make_dummy_quota(0.80))
    res5 = engine.evaluate("acc_c@example.com", _make_dummy_quota(0.02))
    assert res5.should_switch is True
    assert res5.target_account == "acc_a@example.com"


def test_stress_rate_limiting_rapid_switch_guardrail(tmp_path):
    """
    Stress test rolling switch window rate limiting.
    Verify that excessive switches within switch_window_seconds trip thrashing protection.
    """
    acc_file = tmp_path / "accounts.json"
    vault = AccountVault(config_path=acc_file)
    cred = KeyringCredential("token", "ref", "Bearer", "", "consumer", "")

    for i in range(5):
        vault.add_or_update_account(f"user{i}@example.com", cred)
    vault.set_active_account("user0@example.com")
    keyring_service = KeyringService(vault=vault)

    clock_time = 2000.0
    cfg = RuleEngineConfig(
        default_threshold=0.05,
        switch_margin=0.01,
        cooldown_seconds=10.0,
        max_switches_in_window=3,
        switch_window_seconds=600.0,
    )
    engine = AutoSwitchRuleEngine(
        vault=vault,
        keyring_service=keyring_service,
        config=cfg,
        clock_fn=lambda: clock_time,
    )

    for i in range(1, 5):
        engine.update_cached_quota(f"user{i}@example.com", _make_dummy_quota(0.90))

    # Record 3 switches within 30 seconds
    engine.record_switch("user0@example.com", "user1@example.com")
    clock_time += 10.0
    engine.record_switch("user1@example.com", "user2@example.com")
    clock_time += 10.0
    engine.record_switch("user2@example.com", "user3@example.com")
    clock_time += 10.0

    # 4th switch evaluation should be throttled by rate limit
    res = engine.evaluate("user3@example.com", _make_dummy_quota(0.01))
    assert res.should_switch is False
    assert res.cooldown_active is True
    assert "Rate limit exceeded" in res.reason


# ============================================================================
# CHALLENGE 2: ALL ACCOUNTS EXHAUSTED, STANDBY & RESET HORIZON CALCULATION
# ============================================================================

def test_stress_all_accounts_exhausted_standby(tmp_path):
    """
    Challenge 2: All accounts exhausted.
    Drain all accounts to 0.0.
    Verify engine transitions to standby state without throwing unhandled exceptions,
    marks all_exhausted=True, and calculates nearest reset horizon.
    """
    acc_file = tmp_path / "accounts.json"
    vault = AccountVault(config_path=acc_file)
    cred = KeyringCredential("token", "ref", "Bearer", "", "consumer", "")

    now_utc = datetime.now(timezone.utc)
    reset_a = now_utc + timedelta(hours=2)           # Resets in 2 hours (7200s)
    reset_b = now_utc + timedelta(minutes=25)        # Resets in 25 min (1500s) -> NEAREST!
    reset_c = now_utc + timedelta(hours=4, minutes=10) # Resets in 250 min (15000s)

    vault.add_or_update_account("acc_a@example.com", cred)
    vault.add_or_update_account("acc_b@example.com", cred)
    vault.add_or_update_account("acc_c@example.com", cred)
    vault.set_active_account("acc_a@example.com")
    keyring_service = KeyringService(vault=vault)

    engine = AutoSwitchRuleEngine(
        vault=vault,
        keyring_service=keyring_service,
        config=RuleEngineConfig(default_threshold=0.05),
    )

    # Update all accounts with 0.0 remaining quota
    engine.update_cached_quota("acc_b@example.com", _make_dummy_quota(0.0, reset_time=reset_b))
    engine.update_cached_quota("acc_c@example.com", _make_dummy_quota(0.0, reset_time=reset_c))

    # Evaluate acc_a at 0.0
    res = engine.evaluate("acc_a@example.com", _make_dummy_quota(0.0, reset_time=reset_a))
    assert res.should_switch is False
    assert res.target_account is None
    assert res.all_exhausted is True
    assert "All standby accounts exhausted" in res.reason

    # Nearest reset horizon calculation via ResetHorizonTracker
    tracker = ResetHorizonTracker()
    tracker.update_bucket("acc_a@example.com", "gemini-5h", "5h", 0.0, reset_a)
    tracker.update_bucket("acc_b@example.com", "gemini-5h", "5h", 0.0, reset_b)
    tracker.update_bucket("acc_c@example.com", "gemini-5h", "5h", 0.0, reset_c)

    all_horizons = tracker.get_all_horizons()
    assert len(all_horizons) == 3

    # Calculate nearest reset horizon
    nearest_horizon = min(
        all_horizons,
        key=lambda h: tracker.get_countdown_seconds(h.account_email, h.bucket_id),
    )
    assert nearest_horizon.account_email == "acc_b@example.com"
    assert nearest_horizon.bucket_id == "gemini-5h"
    countdown = tracker.get_countdown_seconds("acc_b@example.com", "gemini-5h")
    assert 1490.0 <= countdown <= 1510.0
    formatted = tracker.get_formatted_countdown("acc_b@example.com", "gemini-5h")
    assert formatted.startswith("00:2") or formatted.startswith("00:25")


def test_stress_all_accounts_exhausted_ipc_broadcast(tmp_path):
    """
    Challenge 2 (Integration): Verify notify.all_accounts_exhausted is emitted
    across Unix Domain Socket when all accounts are exhausted during quota.poll_now.
    """
    async def _test():
        sock_path = tmp_path / "exhausted_broadcast.sock"
        server = AsyncUnixSocketServer(sock_path)

        acc_file = tmp_path / "accounts.json"
        vault = AccountVault(config_path=acc_file)
        cred = KeyringCredential("token", "ref", "Bearer", "", "consumer", "")
        vault.add_or_update_account("primary@example.com", cred)
        vault.add_or_update_account("secondary@example.com", cred)
        vault.set_active_account("primary@example.com")
        keyring_service = KeyringService(vault=vault)

        engine = AutoSwitchRuleEngine(
            vault=vault,
            keyring_service=keyring_service,
            config=RuleEngineConfig(default_threshold=0.05),
        )

        # Secondary account is at 0.0
        engine.update_cached_quota("secondary@example.com", _make_dummy_quota(0.0))

        # Mock poller returning 0.0 for primary
        class MockPoller:
            active_account_email = "primary@example.com"
            async def poll_account(self, email, force=True):
                return _make_dummy_quota(0.0)

        poller = MockPoller()
        server.register_quota_handlers(
            poller=poller,
            rule_engine=engine,
            keyring_switcher=keyring_service,
        )

        await server.start()
        try:
            # Client connects and subscribes
            reader, writer = await asyncio.open_unix_connection(str(sock_path))
            try:
                # Issue quota.poll_now request
                req = {
                    "jsonrpc": "2.0",
                    "method": "quota.poll_now",
                    "params": {"account": "primary@example.com"},
                    "id": 42,
                }
                writer.write((json.dumps(req) + "\n").encode("utf-8"))
                await writer.drain()

                # Read messages from server:
                # Expected:
                # 1. notify.all_accounts_exhausted event
                # 2. notify.quota_updated event
                # 3. RPC response with id=42
                events_received = []
                rpc_response = None

                for _ in range(3):
                    line = await asyncio.wait_for(reader.readline(), timeout=3.0)
                    assert line
                    msg = json.loads(line.decode("utf-8"))
                    if "method" in msg:
                        events_received.append(msg)
                    elif msg.get("id") == 42:
                        rpc_response = msg

                assert rpc_response is not None
                assert rpc_response["result"]["switched"] is False

                exhausted_events = [
                    e for e in events_received if e["method"] == "notify.all_accounts_exhausted"
                ]
                assert len(exhausted_events) == 1
                payload = exhausted_events[0]["params"]
                assert payload["active_account"] == "primary@example.com"
                assert "All standby accounts exhausted" in payload["reason"]

            finally:
                writer.close()
                await writer.wait_closed()
        finally:
            await server.stop()

    asyncio.run(_test())


# ============================================================================
# CHALLENGE 3: MULTI-ACCOUNT MOCK SERVER PROFILE ISOLATION
# ============================================================================

def test_stress_mock_cloudcode_multi_account_profile_isolation():
    """
    Challenge 3: Multi-account mock server profile isolation.
    Verify accounts with separate tokens receive independent quota responses
    without cross-account contamination under rapid parallel requests.
    """
    server = MockCloudCodeServer()
    base_url = server.start()
    try:
        # Define 4 distinct profiles
        profiles = {
            "ya29.user_healthy": {"rem": 0.95, "weekly": 1.0, "reset_in": 10000},
            "ya29.user_half": {"rem": 0.50, "weekly": 0.75, "reset_in": 5000},
            "ya29.user_low": {"rem": 0.04, "weekly": 0.20, "reset_in": 2000},
            "ya29.user_depleted": {"rem": 0.00, "weekly": 0.00, "reset_in": 1000},
        }

        for token, p in profiles.items():
            server.set_account_profile(
                token,
                remaining=p["rem"],
                weekly_remaining=p["weekly"],
                reset_in_seconds=p["reset_in"],
            )

        client = CloudCodeClient(base_url=base_url)

        sem = asyncio.Semaphore(4)
        # Run 40 concurrent async requests across all 4 tokens
        async def _fetch_and_verify(token: str, expected_rem: float, expected_weekly: float):
            async with sem:
                summary_dict, _ = await client.retrieve_user_quota_summary(token)
                summary = QuotaSummary.from_dict(summary_dict)
                assert round(summary.gemini_5h.remaining_fraction, 2) == expected_rem
                assert round(summary.gemini_weekly.remaining_fraction, 2) == expected_weekly
                return True

        async def _run_concurrent():
            tasks = []
            for _ in range(10):
                for token, p in profiles.items():
                    tasks.append(_fetch_and_verify(token, p["rem"], p["weekly"]))
            results = await asyncio.gather(*tasks)
            assert all(results)
            assert len(results) == 40

        asyncio.run(_run_concurrent())

        # Test keepalive isolation: firing keepalive for user_low resets only user_low
        client.generate_content_warmup_sync("ya29.user_low")
        summary_low_dict, _ = client.retrieve_user_quota_summary_sync("ya29.user_low")
        summary_depleted_dict, _ = client.retrieve_user_quota_summary_sync("ya29.user_depleted")

        q_low = QuotaSummary.from_dict(summary_low_dict)
        q_depleted = QuotaSummary.from_dict(summary_depleted_dict)

        assert q_low.gemini_5h.remaining_fraction == 1.0  # Reset by keepalive
        assert q_depleted.gemini_5h.remaining_fraction == 0.0  # Unaffected!

    finally:
        server.stop()


# ============================================================================
# CHALLENGE 4: RAPID IPC REQUESTS (quota.poll_now & rules.set_config)
# ============================================================================

def test_stress_rapid_concurrent_ipc_requests(tmp_path):
    """
    Challenge 4: Rapid IPC requests.
    Fire rapid quota.poll_now and rules.set_config calls across Unix Domain Socket,
    verifying concurrency safety under high volume.
    """
    async def _test():
        sock_path = tmp_path / "swiss_rapid_ipc.sock"
        server = AsyncUnixSocketServer(sock_path)

        acc_file = tmp_path / "accounts.json"
        vault = AccountVault(config_path=acc_file)
        cred = KeyringCredential("token", "ref", "Bearer", "", "consumer", "")
        vault.add_or_update_account("active@example.com", cred)
        vault.set_active_account("active@example.com")
        keyring_service = KeyringService(vault=vault)

        engine = AutoSwitchRuleEngine(
            vault=vault,
            keyring_service=keyring_service,
            config=RuleEngineConfig(default_threshold=0.05),
        )

        class MockPoller:
            active_account_email = "active@example.com"
            async def poll_account(self, email, force=True):
                # Return quota with small mock delay to simulate I/O
                await asyncio.sleep(0.001)
                return _make_dummy_quota(0.85)

        poller = MockPoller()
        config_obj = SwissKnifeConfig.load(custom_config_dir=tmp_path)

        server.register_quota_handlers(
            poller=poller,
            rule_engine=engine,
            keyring_switcher=keyring_service,
            config=config_obj,
        )

        await server.start()
        try:
            num_workers = 15
            requests_per_worker = 20

            async def _worker(worker_id: int):
                reader, writer = await asyncio.open_unix_connection(str(sock_path))
                try:
                    for i in range(requests_per_worker):
                        req_id = worker_id * 1000 + i
                        if i % 2 == 0:
                            # quota.poll_now request
                            req = {
                                "jsonrpc": "2.0",
                                "method": "quota.poll_now",
                                "params": {"account": "active@example.com"},
                                "id": req_id,
                            }
                        else:
                            # rules.set_config request with dynamic threshold
                            new_thresh = 0.05 + (i * 0.005)
                            req = {
                                "jsonrpc": "2.0",
                                "method": "rules.set_config",
                                "params": {
                                    "auto_switch_threshold": round(new_thresh, 3),
                                    "cooldown_seconds": 250.0 + i,
                                },
                                "id": req_id,
                            }

                        writer.write((json.dumps(req) + "\n").encode("utf-8"))
                        await writer.drain()

                        # Read response (filtering any broadcast notifications if present)
                        while True:
                            line = await asyncio.wait_for(reader.readline(), timeout=5.0)
                            assert line, "Server closed socket unexpectedly"
                            msg = json.loads(line.decode("utf-8"))
                            if msg.get("id") == req_id:
                                assert "result" in msg
                                break

                finally:
                    writer.close()
                    await writer.wait_closed()

            # Execute 15 workers concurrently (300 rapid operations)
            workers = [_worker(w) for w in range(num_workers)]
            await asyncio.gather(*workers)

        finally:
            await server.stop()

    asyncio.run(_test())


# ============================================================================
# CHALLENGE 5: EDGE CASES & RESILIENCE (Malformed Data, All Unhealthy)
# ============================================================================

def test_stress_malformed_quota_data_and_unhealthy_accounts(tmp_path):
    """
    Challenge 5: Stress test resilience against malformed quota data, None payloads,
    and all accounts marked unhealthy.
    """
    acc_file = tmp_path / "accounts.json"
    vault = AccountVault(config_path=acc_file)
    cred = KeyringCredential("token", "ref", "Bearer", "", "consumer", "")

    vault.add_or_update_account("user1@example.com", cred)
    vault.add_or_update_account("user2@example.com", cred)
    vault.set_active_account("user1@example.com")
    keyring_service = KeyringService(vault=vault)

    engine = AutoSwitchRuleEngine(vault=vault, keyring_service=keyring_service)

    # 1. Evaluate with None payload -> graceful fallback to 1.0 (no crash)
    res_none = engine.evaluate("user1@example.com", None)
    assert res_none.should_switch is False
    assert res_none.current_fraction == 1.0

    # 2. Evaluate with empty dict -> fallback to 1.0 (no crash)
    res_empty = engine.evaluate("user1@example.com", {})
    assert res_empty.should_switch is False
    assert res_empty.current_fraction == 1.0

    # 3. Evaluate with string / invalid numeric remainingFraction
    malformed = {
        "groups": [
            {
                "buckets": [
                    {"bucketId": "gemini-5h", "remainingFraction": "invalid"},
                    {"bucketId": "gemini-weekly", "remainingFraction": None},
                ]
            }
        ]
    }
    res_malformed = engine.evaluate("user1@example.com", malformed)
    assert res_malformed.should_switch is False

    # 4. Mark all standby accounts unhealthy
    vault.add_or_update_account("user2@example.com", cred, is_healthy=False)

    res_unhealthy = engine.evaluate("user1@example.com", _make_dummy_quota(0.01))
    assert res_unhealthy.should_switch is False
    assert res_unhealthy.all_exhausted is True
