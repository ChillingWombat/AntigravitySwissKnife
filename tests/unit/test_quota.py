"""
Unit tests for Quota Models, CloudCodeClient, QuotaPoller, and AutoSwitchRuleEngine.
=====================================================================================
Covers Features F06, F07, and F09.
"""

import asyncio
from datetime import datetime, timedelta, timezone
import json
import time
from typing import Any
import pytest

from antigravity_swiss.core.config import SwissKnifeConfig
from antigravity_swiss.core.errors import (
    QuotaAuthExpiredError,
    QuotaError,
    QuotaRateLimitError,
    QuotaUnavailableError,
)
from antigravity_swiss.keyring.switcher import (
    AccountRecord,
    AccountVault,
    KeyringCredential,
    KeyringService,
)
from antigravity_swiss.quota.client import CloudCodeClient
from antigravity_swiss.quota.models import (
    ModelCatalog,
    ModelDetails,
    ModelQuotaBucket,
    QuotaSummary,
    QuotaSummaryGroup,
    TieredModelConfig,
    format_rfc3339_timestamp,
    parse_rfc3339_timestamp,
)
from antigravity_swiss.quota.poller import QuotaPoller
from antigravity_swiss.quota.rule_engine import (
    AutoSwitchRuleEngine,
    EvaluationResult,
    RuleEngineConfig,
)
from tests.fixtures.mock_cloudcode_server import MockCloudCodeServer


# ============================================================================
# Quota Models Tests
# ============================================================================

def test_rfc3339_parsing_and_formatting():
    """Verify RFC 3339 UTC timestamp parsing, formatting, and edge cases."""
    raw = "2026-10-01T08:53:53Z"
    dt = parse_rfc3339_timestamp(raw)
    assert dt is not None
    assert dt.tzinfo == timezone.utc
    assert dt.year == 2026
    assert dt.hour == 8
    assert dt.minute == 53
    assert dt.second == 53

    # Format back
    formatted = format_rfc3339_timestamp(dt)
    assert formatted == raw

    # Edge cases
    assert parse_rfc3339_timestamp(None) is None
    assert parse_rfc3339_timestamp("") is None
    assert parse_rfc3339_timestamp("not-a-timestamp") is None
    assert format_rfc3339_timestamp(None) is None


def test_model_quota_bucket_properties_and_clamping():
    """Verify ModelQuotaBucket attributes, clamping, and reset calculation."""
    now = datetime(2026, 10, 1, 8, 0, 0, tzinfo=timezone.utc)
    reset = datetime(2026, 10, 1, 9, 30, 0, tzinfo=timezone.utc)

    bucket = ModelQuotaBucket(
        bucket_id="gemini-5h",
        display_name="Gemini 5-Hour",
        window="5h",
        remaining_fraction=0.85,
        reset_time=reset,
    )
    assert bucket.remaining_fraction == 0.85
    assert bucket.seconds_until_reset(now) == 5400.0  # 1.5 hours
    assert not bucket.is_exhausted(0.05)

    # Clamping tests
    b_neg = ModelQuotaBucket("test", "test", "5h", remaining_fraction=-0.5)
    assert b_neg.remaining_fraction == 0.0

    b_over = ModelQuotaBucket("test", "test", "5h", remaining_fraction=1.5)
    assert b_over.remaining_fraction == 1.0

    # Exhaustion check
    b_exhausted = ModelQuotaBucket("test", "test", "5h", remaining_fraction=0.03)
    assert b_exhausted.is_exhausted(0.05)

    # Serialization roundtrip
    d = bucket.to_dict()
    assert d["bucketId"] == "gemini-5h"
    assert d["window"] == "5h"
    assert d["remainingFraction"] == 0.85
    b_copy = ModelQuotaBucket.from_dict(d)
    assert b_copy.bucket_id == bucket.bucket_id
    assert b_copy.remaining_fraction == bucket.remaining_fraction
    assert b_copy.reset_time == bucket.reset_time


def test_quota_summary_group_and_summary_accessors():
    """Verify QuotaSummaryGroup, QuotaSummary convenience properties, and min fraction math."""
    reset = datetime(2026, 10, 1, 10, 0, 0, tzinfo=timezone.utc)
    b_5h = ModelQuotaBucket("gemini-5h", "5h Limit", "5h", 0.40, reset)
    b_weekly = ModelQuotaBucket("gemini-weekly", "Weekly Limit", "weekly", 0.90, reset)
    gemini_group = QuotaSummaryGroup("Gemini Models", "Gemini pool", [b_5h, b_weekly])

    b_3p_5h = ModelQuotaBucket("3p-5h", "Claude 5h", "5h", 0.75, reset)
    b_3p_weekly = ModelQuotaBucket("3p-weekly", "Claude Weekly", "weekly", 1.0, reset)
    claude_group = QuotaSummaryGroup("Claude and GPT models", "Claude pool", [b_3p_5h, b_3p_weekly])

    summary = QuotaSummary(groups=[gemini_group, claude_group], active_account="alice@example.com")

    assert summary.gemini_5h is not None
    assert summary.gemini_5h.remaining_fraction == 0.40
    assert summary.gemini_weekly is not None
    assert summary.gemini_weekly.remaining_fraction == 0.90
    assert summary.p3_5h is not None
    assert summary.p3_5h.remaining_fraction == 0.75
    assert summary.p3_weekly is not None
    assert summary.p3_weekly.remaining_fraction == 1.0

    # Lowest fraction
    assert summary.lowest_remaining_fraction() == 0.40
    assert summary.lowest_remaining_fraction(group_name="Claude") == 0.75
    assert not summary.is_exhausted(0.10)
    assert summary.is_exhausted(0.50)

    # Dictionary roundtrip
    data = summary.to_dict()
    assert data["activeAccount"] == "alice@example.com"
    summary_rt = QuotaSummary.from_dict(data)
    assert summary_rt.gemini_5h.remaining_fraction == 0.40
    assert summary_rt.p3_5h.remaining_fraction == 0.75


def test_model_catalog_and_details():
    """Verify ModelDetails, TieredModelConfig, and ModelCatalog serialization."""
    details = ModelDetails(
        model_id="gemini-3.8-flash-high",
        display_name="Gemini 3.8 Flash (High)",
        supports_images=True,
        supports_thinking=True,
        thinking_budget=1024,
        max_tokens=1048576,
        max_output_tokens=65536,
        remaining_fraction=0.88,
    )
    assert details.model_id == "gemini-3.8-flash-high"
    assert details.supports_images is True

    tiered = TieredModelConfig(
        flash_lite=["gemini-3.5-flash-lite"],
        flash=["gemini-3.8-flash-tiered"],
        pro=["gemini-3.1-pro-low"],
    )

    catalog = ModelCatalog(
        models={"gemini-3.8-flash-high": details},
        default_agent_model_id="gemini-3.8-flash-high",
        tiered_model_ids=tiered,
    )
    assert catalog.get_model("gemini-3.8-flash-high") is not None
    assert catalog.get_model("unknown-model") is None

    cat_dict = catalog.to_dict()
    catalog_rt = ModelCatalog.from_dict(cat_dict)
    assert catalog_rt.default_agent_model_id == "gemini-3.8-flash-high"
    assert catalog_rt.tiered_model_ids.flash == ["gemini-3.8-flash-tiered"]
    assert catalog_rt.get_model("gemini-3.8-flash-high").thinking_budget == 1024


# ============================================================================
# CloudCodeClient Tests
# ============================================================================

def test_client_endpoints_and_date_drift():
    """Verify CloudCodeClient interactions with mock server and Date drift extraction."""
    server = MockCloudCodeServer()
    base_url = server.start()
    try:
        server.set_clock_drift(20.0)
        server.set_quota(0.70, reset_in_seconds=3600)

        client = CloudCodeClient(base_url=base_url)

        # 1. retrieve_user_quota_summary_sync
        data, drift = client.retrieve_user_quota_summary_sync("ya29.alice_test")
        summary = QuotaSummary.from_dict(data)
        assert summary.gemini_5h.remaining_fraction == 0.70
        assert drift > 10.0

        # 2. fetch_available_models_sync
        model_data, _ = client.fetch_available_models_sync("ya29.alice_test")
        catalog = ModelCatalog.from_dict(model_data)
        assert catalog.default_agent_model_id == "gemini-3.8-flash-high"

        # 3. generate_content_warmup_sync
        warmup_data, _ = client.generate_content_warmup_sync("ya29.alice_test")
        assert "candidates" in warmup_data
        assert server.warmup_fired is True

        # 4. Async wrappers
        async def run_async():
            d_async, _ = await client.retrieve_user_quota_summary("ya29.alice_test")
            assert "groups" in d_async

        asyncio.run(run_async())
    finally:
        server.stop()


def test_client_error_hierarchy_mapping():
    """Verify CloudCodeClient strictly maps 401, 429, 503, and network errors."""
    server = MockCloudCodeServer()
    base_url = server.start()
    try:
        client = CloudCodeClient(base_url=base_url, timeout=2.0)

        # 401 Unauthenticated
        with pytest.raises(QuotaAuthExpiredError) as exc_401:
            client.retrieve_user_quota_summary_sync("invalid_token")
        assert exc_401.value.code == -32031

        # 503 Unavailable
        server.transient_errors_remaining = 1
        server.transient_error_status = 503
        with pytest.raises(QuotaUnavailableError) as exc_503:
            client.retrieve_user_quota_summary_sync("ya29.test")
        assert exc_503.value.code == -32034

        # 429 Resource Exhausted on generateContent before reset
        server.set_quota(0.0, reset_in_seconds=7200)
        with pytest.raises(QuotaRateLimitError) as exc_429:
            client.generate_content_warmup_sync("ya29.test")
        assert exc_429.value.code == -32033

        # Invalid endpoint or network error
        dead_client = CloudCodeClient(base_url="http://127.0.0.1:9", timeout=0.2)
        from antigravity_swiss.core.errors import QuotaNetworkError
        with pytest.raises(QuotaNetworkError):
            dead_client.retrieve_user_quota_summary_sync("ya29.test")
    finally:
        server.stop()


# ============================================================================
# QuotaPoller Tests
# ============================================================================

def test_quota_poller_cache_and_token_refresh(tmp_path):
    """Verify QuotaPoller in-memory TTL caching and reactive 401 token refresh."""
    server = MockCloudCodeServer()
    base_url = server.start()
    try:
        acc_file = tmp_path / "accounts.json"
        vault = AccountVault(config_path=acc_file)
        cred = KeyringCredential(
            access_token="ya29.initial",
            refresh_token="1//valid_refresh",
            token_type="Bearer",
            expiry=(datetime.now(timezone.utc) + timedelta(hours=1)).isoformat(),
            auth_method="consumer",
            id_token="",
        )
        vault.add_or_update_account("user@example.com", cred, label="Primary")
        vault.set_active_account("user@example.com")
        keyring_service = KeyringService(vault=vault)

        client = CloudCodeClient(base_url=base_url)
        notifications = []

        def on_updated(s):
            notifications.append(s)

        poller = QuotaPoller(
            client=client,
            vault=vault,
            keyring_service=keyring_service,
            cache_ttl_sec=5.0,
            on_quota_updated=on_updated,
        )

        async def run():
            # 1. Initial poll
            s1 = await poller.poll_summary()
            assert s1.gemini_5h.remaining_fraction == 0.85
            assert len(notifications) == 1

            # 2. Cached poll within TTL
            reqs_before = len(server.recorded_requests)
            s2 = await poller.poll_summary(force=False)
            assert s2 is s1
            assert len(server.recorded_requests) == reqs_before  # No network hit

            # 3. Force poll bypasses cache
            s3 = await poller.poll_summary(force=True)
            assert len(server.recorded_requests) > reqs_before

            # 4. Reactive 401 refresh test
            # Set access token to expired/invalid
            expired_cred = KeyringCredential(
                access_token="expired_token",
                refresh_token="1//mock_refresh",
                token_type="Bearer",
                expiry=(datetime.now(timezone.utc) + timedelta(hours=1)).isoformat(),
                auth_method="consumer",
                id_token="",
            )
            keyring_service.set_active_credential(expired_cred)
            # Poller should catch 401, refresh token via mock server /token, and succeed!
            s_recovered = await poller.poll_summary(force=True)
            assert s_recovered is not None
            active_cred = keyring_service.get_active_credential()
            assert "mock_refreshed" in active_cred.access_token

        asyncio.run(run())
    finally:
        server.stop()


def test_quota_poller_start_stop():
    """Verify QuotaPoller background loop start and stop lifecycle."""
    server = MockCloudCodeServer()
    base_url = server.start()
    try:
        client = CloudCodeClient(base_url=base_url)
        poller = QuotaPoller(client=client, poll_interval_sec=0.1)

        async def run():
            assert not poller.is_running
            await poller.start()
            assert poller.is_running
            await asyncio.sleep(0.25)
            await poller.stop()
            assert not poller.is_running

        asyncio.run(run())
    finally:
        server.stop()


# ============================================================================
# AutoSwitchRuleEngine Tests
# ============================================================================

def test_rule_engine_threshold_and_eligibility(tmp_path):
    """Verify AutoSwitchRuleEngine threshold evaluation, candidate scoring, and anti-thrashing."""
    acc_file = tmp_path / "accounts.json"
    vault = AccountVault(config_path=acc_file)

    # Setup accounts:
    # Alice (active): depleted to 0.02
    # Bob (standby 1): healthy 0.95
    # Carol (standby 2): low 0.04 (should be skipped)
    # Dave (standby 3): weekly exhausted (should be skipped)
    cred_a = KeyringCredential("tok_a", "ref_a", "Bearer", "", "consumer", "")
    cred_b = KeyringCredential("tok_b", "ref_b", "Bearer", "", "consumer", "")
    cred_c = KeyringCredential("tok_c", "ref_c", "Bearer", "", "consumer", "")
    cred_d = KeyringCredential("tok_d", "ref_d", "Bearer", "", "consumer", "")

    vault.add_or_update_account("alice@example.com", cred_a, label="Alice")
    vault.add_or_update_account("bob@example.com", cred_b, label="Bob")
    vault.add_or_update_account("carol@example.com", cred_c, label="Carol")
    vault.add_or_update_account("dave@example.com", cred_d, label="Dave")
    vault.set_active_account("alice@example.com")
    keyring_service = KeyringService(vault=vault)

    clock_time = 1000.0
    engine = AutoSwitchRuleEngine(
        vault=vault,
        keyring_service=keyring_service,
        config=RuleEngineConfig(
            default_threshold=0.05,
            cooldown_seconds=300.0,
            switch_margin=0.05,
        ),
        clock_fn=lambda: clock_time,
    )

    # Seed quota cache
    engine.update_cached_quota("bob@example.com", {
        "groups": [{"buckets": [{"bucketId": "gemini-5h", "remainingFraction": 0.95}, {"bucketId": "gemini-weekly", "remainingFraction": 1.0}]}]
    })
    engine.update_cached_quota("carol@example.com", {
        "groups": [{"buckets": [{"bucketId": "gemini-5h", "remainingFraction": 0.04}, {"bucketId": "gemini-weekly", "remainingFraction": 1.0}]}]
    })
    engine.update_cached_quota("dave@example.com", {
        "groups": [{"buckets": [{"bucketId": "gemini-5h", "remainingFraction": 1.0}, {"bucketId": "gemini-weekly", "remainingFraction": 0.0}]}]
    })

    # 1. Evaluate Alice at 0.02 (breached) -> Should switch to Bob!
    res = engine.evaluate(
        "alice@example.com",
        {"groups": [{"buckets": [{"bucketId": "gemini-5h", "remainingFraction": 0.02}, {"bucketId": "gemini-weekly", "remainingFraction": 1.0}]}]}
    )
    assert res.should_switch is True
    assert res.target_account == "bob@example.com"
    assert res.target_score > 0.0

    # Record switch
    engine.record_switch("alice@example.com", "bob@example.com")

    # 2. Alice is now in cooldown
    assert engine.is_account_in_cooldown("alice@example.com") is True
    # Fast forward clock past cooldown
    clock_time += 301.0
    assert engine.is_account_in_cooldown("alice@example.com") is False


def test_rule_engine_all_exhausted_and_rate_limiting(tmp_path):
    """Verify rule engine behavior when all standby accounts are depleted, and rate limiting."""
    acc_file = tmp_path / "accounts.json"
    vault = AccountVault(config_path=acc_file)

    cred = KeyringCredential("t", "r", "Bearer", "", "consumer", "")
    vault.add_or_update_account("user1@example.com", cred)
    vault.add_or_update_account("user2@example.com", cred)
    vault.set_active_account("user1@example.com")
    keyring_service = KeyringService(vault=vault)

    clock_time = 1000.0
    engine = AutoSwitchRuleEngine(
        vault=vault,
        keyring_service=keyring_service,
        config=RuleEngineConfig(
            default_threshold=0.05,
            max_switches_in_window=2,
            switch_window_seconds=100.0,
        ),
        clock_fn=lambda: clock_time,
    )

    # user2 has 0.01 (depleted)
    engine.update_cached_quota("user2@example.com", {
        "groups": [{"buckets": [{"bucketId": "gemini-5h", "remainingFraction": 0.01}, {"bucketId": "gemini-weekly", "remainingFraction": 1.0}]}]
    })

    # user1 depleted to 0.02 -> All exhausted!
    res = engine.evaluate(
        "user1@example.com",
        {"groups": [{"buckets": [{"bucketId": "gemini-5h", "remainingFraction": 0.02}, {"bucketId": "gemini-weekly", "remainingFraction": 1.0}]}]}
    )
    assert res.should_switch is False
    assert res.all_exhausted is True

    # Rate limiting test
    engine.record_switch("a", "b")
    engine.record_switch("b", "a")
    assert engine.is_rate_limited() is True
