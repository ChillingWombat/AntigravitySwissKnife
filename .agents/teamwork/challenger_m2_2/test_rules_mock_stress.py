"""
Adversarial Stress Test Suite for AutoSwitchRuleEngine, Mock CloudCode Server, and IPC daemon.
=============================================================================================
Challenges:
1. Thrashing simulation with 3 accounts near threshold with cooldown & hysteresis margin.
2. All accounts exhausted transition to standby state and reset horizon calculation.
3. Multi-account mock server profile isolation across concurrent requests.
4. Rapid concurrent IPC requests over Unix Domain Socket.
"""

import asyncio
from datetime import datetime, timedelta, timezone
import json
import sys
import tempfile
import time
from pathlib import Path
import pytest

sys.path.insert(0, str(Path(__file__).parents[3]))

from antigravity_swiss.core.config import SwissKnifeConfig
from antigravity_swiss.core.errors import QuotaError
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
from tests.fixtures.mock_cloudcode_server import MockCloudCodeServer


def _make_dummy_quota(fraction: float, weekly: float = 1.0) -> dict:
    return {
        "groups": [
            {
                "buckets": [
                    {"bucketId": "gemini-5h", "remainingFraction": fraction},
                    {"bucketId": "gemini-weekly", "remainingFraction": weekly},
                ]
            }
        ]
    }


def test_stress_anti_thrashing_cooldown_and_margin(tmp_path):
    """
    Challenge 1: Thrashing simulation.
    3 accounts rapidly draining to near-threshold (0.04, 0.06, 0.15).
    Verify 300s cooldown and 0.05 margin prevent infinite switch loops.
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

    # acc_b is 0.06 (above threshold 0.05, but below threshold + margin 0.10)
    engine.update_cached_quota("acc_b@example.com", _make_dummy_quota(0.06))
    # acc_c is 0.15 (healthy, > 0.10)
    engine.update_cached_quota("acc_c@example.com", _make_dummy_quota(0.15))

    # Evaluate active account A at 0.03 (breached)
    res = engine.evaluate("acc_a@example.com", _make_dummy_quota(0.03))
    assert res.should_switch is True
    # Must pick C, because B does not meet threshold + margin (0.05 + 0.05 = 0.10)
    assert res.target_account == "acc_c@example.com"

    # Record switch to C
    engine.record_switch("acc_a@example.com", "acc_c@example.com")
    vault.set_active_account("acc_c@example.com")

    # Immediate second evaluation: even if C drops to 0.04, A is under cooldown
    res2 = engine.evaluate("acc_c@example.com", _make_dummy_quota(0.04))
    # A is in cooldown, B is below margin, so no healthy target exists
    assert res2.should_switch is False
    assert res2.target_account is None


def test_stress_all_accounts_exhausted_standby(tmp_path):
    """
    Challenge 2: All accounts exhausted.
    Drain all accounts to 0.0.
    Verify engine transitions to standby state without throwing and marks all_exhausted=True.
    """
    acc_file = tmp_path / "accounts.json"
    vault = AccountVault(config_path=acc_file)
    cred = KeyringCredential("token", "ref", "Bearer", "", "consumer", "")

    vault.add_or_update_account("acc_a@example.com", cred)
    vault.add_or_update_account("acc_b@example.com", cred)
    vault.set_active_account("acc_a@example.com")
    keyring_service = KeyringService(vault=vault)

    engine = AutoSwitchRuleEngine(
        vault=vault,
        keyring_service=keyring_service,
        config=RuleEngineConfig(default_threshold=0.05),
    )

    # acc_b has 0.0
    engine.update_cached_quota("acc_b@example.com", _make_dummy_quota(0.0))

    # Evaluate acc_a at 0.0
    res = engine.evaluate("acc_a@example.com", _make_dummy_quota(0.0))
    assert res.should_switch is False
    assert res.target_account is None
    assert res.all_exhausted is True


def test_stress_mock_cloudcode_multi_account_profile_isolation():
    """
    Challenge 3: Multi-account mock server profile isolation.
    Verify accounts with separate tokens receive independent quota responses.
    """
    server = MockCloudCodeServer()
    base_url = server.start()
    try:
        server.set_account_profile("ya29.user1", remaining=0.88, weekly_remaining=0.95)
        server.set_account_profile("ya29.user2", remaining=0.12, weekly_remaining=0.40)

        client = CloudCodeClient(base_url=base_url)

        data1, drift1 = client.retrieve_user_quota_summary_sync("ya29.user1")
        data2, drift2 = client.retrieve_user_quota_summary_sync("ya29.user2")

        q1 = QuotaSummary.from_dict(data1)
        q1.server_time_drift_seconds = drift1
        q2 = QuotaSummary.from_dict(data2)
        q2.server_time_drift_seconds = drift2

        assert q1.gemini_5h.remaining_fraction == 0.88
        assert q1.gemini_weekly.remaining_fraction == 0.95

        assert q2.gemini_5h.remaining_fraction == 0.12
        assert q2.gemini_weekly.remaining_fraction == 0.40
    finally:
        server.stop()


def test_stress_rapid_concurrent_ipc_requests(tmp_path):
    """
    Challenge 4: Rapid IPC requests.
    Fire rapid quota.get_summary, quota.poll_now, rules.get_config, rules.set_config
    across Unix Domain Socket concurrently.
    """
    async def _test():
        sock_path = tmp_path / "swiss_stress.sock"
        server = AsyncUnixSocketServer(sock_path)

        # Register methods
        config_data = {"min_threshold_fraction": 0.05, "cooldown_seconds": 300.0}

        server.register("quota.get_summary", lambda: {"status": "ok", "active": 0.85})
        server.register("quota.poll_now", lambda: {"status": "polled", "timestamp": time.time()})
        server.register("rules.get_config", lambda: config_data)
        server.register("rules.set_config", lambda cfg: config_data.update(cfg) or config_data)

        await server.start()
        try:
            async def _client_worker(worker_id: int):
                reader, writer = await asyncio.open_unix_connection(str(sock_path))
                try:
                    for i in range(15):
                        req = {
                            "jsonrpc": "2.0",
                            "method": "quota.get_summary" if i % 2 == 0 else "rules.get_config",
                            "id": worker_id * 100 + i,
                        }
                        writer.write((json.dumps(req) + "\n").encode("utf-8"))
                        await writer.drain()
                        line = await reader.readline()
                        assert line
                        resp = json.loads(line.decode("utf-8"))
                        assert resp.get("id") == worker_id * 100 + i
                        assert "result" in resp
                finally:
                    writer.close()
                    await writer.wait_closed()

            # Run 20 concurrent client workers
            workers = [_client_worker(w) for w in range(20)]
            await asyncio.gather(*workers)

        finally:
            await server.stop()

    asyncio.run(_test())
