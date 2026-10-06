"""
Unit Tests for Quota Calculator & Fleet Synthesis Engine.
"""

import pytest
from antigravity_swiss.quota.calculator import (
    AccountQuotaState,
    build_account_quota_states,
    compute_fleet_quota_summary,
    sort_account_quota_states,
)


def test_account_quota_state_5h_boost():
    """Test that accounts resetting within 5h receive proportional availability boost."""
    # Account resetting in 1 hour (4 hours of fresh 100% capacity in the 5h window)
    acc = AccountQuotaState(
        email="user1@example.com",
        label="User 1",
        is_active=True,
        status="ACTIVE",
        has_totp=True,
        totp_secret="JBSWY3DPEHPK3PXP",
        refresh_token="ref1",
        quota_5h_current=0.40,
        reset_seconds=3600.0,  # 1 hour
        quota_weekly=0.85,
    )
    assert acc.hours_until_reset == 1.0
    # Boost: (1.0 - 0.40) * (4.0 / 5.0) = 0.60 * 0.8 = 0.48
    # Total available: 0.40 + 0.48 = 0.88
    assert pytest.approx(acc.quota_5h_available, 0.01) == 0.88
    assert "1h" in acc.reset_horizon_text


def test_account_quota_state_no_boost_past_5h():
    """Test that accounts resetting after 5h only yield current remaining quota."""
    acc = AccountQuotaState(
        email="user2@example.com",
        label="User 2",
        is_active=False,
        status="STANDBY",
        has_totp=False,
        totp_secret="",
        refresh_token="ref2",
        quota_5h_current=0.50,
        reset_seconds=7 * 3600.0,  # 7 hours
        quota_weekly=0.90,
    )
    assert acc.hours_until_reset == 7.0
    assert acc.quota_5h_available == 0.50
    assert "7h" in acc.reset_horizon_text


def test_compute_fleet_quota_summary():
    """Test aggregate fleet calculations across multiple accounts."""
    acc1 = AccountQuotaState(
        email="a1@example.com",
        label="A1",
        is_active=True,
        status="ACTIVE",
        has_totp=True,
        totp_secret="",
        refresh_token="",
        quota_5h_current=1.0,
        reset_seconds=1800.0,
        quota_weekly=0.90,
    )
    acc2 = AccountQuotaState(
        email="a2@example.com",
        label="A2",
        is_active=False,
        status="STANDBY",
        has_totp=False,
        totp_secret="",
        refresh_token="",
        quota_5h_current=0.50,
        reset_seconds=8 * 3600.0,  # 8h > 5h
        quota_weekly=0.80,
    )

    f5, fw, total = compute_fleet_quota_summary([acc1, acc2])
    assert total == 2
    # acc1 has 1.0, acc2 has 0.50 -> avg = 0.75
    assert pytest.approx(f5, 0.01) == 0.75
    # weekly: 0.90 and 0.80 -> avg = 0.85
    assert pytest.approx(fw, 0.01) == 0.85


def test_build_account_quota_states():
    """Test constructing AccountQuotaState objects from raw store data."""
    raw = [
        {"email": "alpha@example.com", "label": "Alpha", "is_active": True},
        {"email": "beta@example.com", "label": "Beta", "is_active": False},
    ]
    states = build_account_quota_states(raw)
    assert len(states) == 2
    assert states[0].email == "alpha@example.com"
    assert states[0].is_active is True
    assert 0.0 <= states[0].quota_5h_available <= 1.0
    assert 0.0 <= states[1].quota_weekly <= 1.0


def test_sort_account_quota_states():
    """Test sorting account quota states in auto, identity, 5h quota, and weekly quota modes."""
    acc_active = AccountQuotaState(
        email="active@example.com",
        label="Active Lead",
        is_active=True,
        status="ACTIVE",
        has_totp=True,
        totp_secret="",
        refresh_token="",
        quota_5h_current=0.80,
        reset_seconds=10000.0,
        quota_weekly=0.90,
    )
    acc_standby_high = AccountQuotaState(
        email="standby_high@example.com",
        label="Standby High",
        is_active=False,
        status="STANDBY",
        has_totp=True,
        totp_secret="",
        refresh_token="",
        quota_5h_current=0.95,
        reset_seconds=10000.0,
        quota_weekly=0.95,
    )
    acc_standby_mid = AccountQuotaState(
        email="standby_mid@example.com",
        label="Standby Mid",
        is_active=False,
        status="STANDBY",
        has_totp=True,
        totp_secret="",
        refresh_token="",
        quota_5h_current=0.50,
        reset_seconds=10000.0,
        quota_weekly=0.60,
    )
    acc_depleted = AccountQuotaState(
        email="depleted@example.com",
        label="Depleted Acc",
        is_active=False,
        status="STANDBY",
        has_totp=True,
        totp_secret="",
        refresh_token="",
        quota_5h_current=0.05,  # <= 0.10 threshold
        reset_seconds=10000.0,
        quota_weekly=0.04,
    )
    acc_banned = AccountQuotaState(
        email="banned@example.com",
        label="Banned Acc",
        is_active=False,
        status="BANNED",
        has_totp=True,
        totp_secret="",
        refresh_token="",
        quota_5h_current=1.0,
        reset_seconds=10000.0,
        quota_weekly=1.0,
    )
    acc_error = AccountQuotaState(
        email="error@example.com",
        label="Error Acc",
        is_active=False,
        status="ERROR",
        has_totp=True,
        totp_secret="",
        refresh_token="",
        quota_5h_current=1.0,
        reset_seconds=10000.0,
        quota_weekly=1.0,
    )

    accounts = [acc_standby_mid, acc_depleted, acc_banned, acc_standby_high, acc_active, acc_error]

    # 1. AUTO mode: Row 1 = Active healthy, Row 2 = Standby high (next switch candidate),
    # then standby mid, depleted, error, banned at bottom.
    sorted_auto = sort_account_quota_states(accounts, active_email="active@example.com", threshold=0.10, mode="auto")
    assert sorted_auto[0].email == "active@example.com"
    assert sorted_auto[1].email == "standby_high@example.com"
    assert sorted_auto[2].email == "standby_mid@example.com"
    assert sorted_auto[3].email == "depleted@example.com"
    assert sorted_auto[4].email == "error@example.com"
    assert sorted_auto[5].email == "banned@example.com"

    # 2. IDENTITY mode: Alphabetical by label
    sorted_id = sort_account_quota_states(accounts, mode="identity")
    assert [a.email for a in sorted_id] == [
        "active@example.com",
        "banned@example.com",
        "depleted@example.com",
        "error@example.com",
        "standby_high@example.com",
        "standby_mid@example.com",
    ]

    # 3. QUOTA_5H mode: Descending by 5H quota
    sorted_5h = sort_account_quota_states(accounts, mode="quota_5h")
    assert sorted_5h[0].quota_5h_available >= sorted_5h[1].quota_5h_available
    assert sorted_5h[-1].email == "depleted@example.com"

    # 4. QUOTA_WEEKLY mode: Descending by weekly quota
    sorted_wk = sort_account_quota_states(accounts, mode="quota_weekly")
    assert sorted_wk[0].quota_weekly >= sorted_wk[1].quota_weekly
    assert sorted_wk[-1].email == "depleted@example.com"


def test_build_account_quota_states_gemini_group_isolation():
    """Verify that Claude/GPT 3P groups do not overwrite Gemini 5h/weekly quotas."""
    active_summary = {
        "groups": [
            {
                "displayName": "Gemini 2.5 Pro & Flash",
                "buckets": [
                    {"bucketId": "gemini-5h", "window": "5h", "remainingFraction": 0.85},
                    {"bucketId": "gemini-weekly", "window": "weekly", "remainingFraction": 0.92},
                ],
            },
            {
                "displayName": "Claude 3.7 & GPT-4o (3P)",
                "buckets": [
                    {"bucketId": "claude-5h", "window": "5h", "remainingFraction": 0.10},
                    {"bucketId": "claude-weekly", "window": "weekly", "remainingFraction": 0.20},
                ],
            },
        ]
    }

    accounts = [{"email": "user@google.com", "label": "User", "is_active": True}]
    states = build_account_quota_states(accounts, active_quota_summary=active_summary)

    assert len(states) == 1
    # Gemini values should be preserved, NOT overwritten by Claude 3P buckets
    assert pytest.approx(states[0].quota_5h_current, 0.01) == 0.85
    assert pytest.approx(states[0].quota_weekly, 0.01) == 0.92


def test_sort_account_quota_states_exhausted_current_demoted():
    """Verify that an account with 0% current 5h quota (even if resetting soon) is demoted below healthy standbys."""
    acc_active = AccountQuotaState(
        email="active@example.com",
        label="Active",
        is_active=True,
        status="ACTIVE",
        has_totp=True,
        totp_secret="",
        refresh_token="",
        quota_5h_current=0.75,
        reset_seconds=7200.0,
        quota_weekly=0.80,
    )
    # 0% current, but resets in 1 hour -> quota_5h_available is 0.80!
    acc_exhausted_current = AccountQuotaState(
        email="exhausted@example.com",
        label="Exhausted Now",
        is_active=False,
        status="STANDBY",
        has_totp=True,
        totp_secret="",
        refresh_token="",
        quota_5h_current=0.00,
        reset_seconds=3600.0,
        quota_weekly=0.90,
    )
    acc_healthy_standby = AccountQuotaState(
        email="healthy@example.com",
        label="Healthy Standby",
        is_active=False,
        status="STANDBY",
        has_totp=True,
        totp_secret="",
        refresh_token="",
        quota_5h_current=0.60,
        reset_seconds=14000.0,
        quota_weekly=0.70,
    )

    sorted_accounts = sort_account_quota_states(
        [acc_exhausted_current, acc_healthy_standby, acc_active],
        active_email="active@example.com",
        threshold=0.10,
        mode="auto",
    )

    # Row 1: active, Row 2: healthy standby (NOT exhausted now!), Row 3: exhausted now (Tier 2 cooling down)
    assert sorted_accounts[0].email == "active@example.com"
    assert sorted_accounts[1].email == "healthy@example.com"
    assert sorted_accounts[2].email == "exhausted@example.com"


def test_sort_account_quota_states_depleted_active_remains_row_1():
    """Verify that an active account below threshold is pinned to Row 1 even when quota is depleted."""
    acc_active_depleted = AccountQuotaState(
        email="active_depleted@example.com",
        label="Depleted Active",
        is_active=True,
        status="ACTIVE",
        has_totp=True,
        totp_secret="",
        refresh_token="",
        quota_5h_current=0.00,
        reset_seconds=0.0,
        quota_weekly=0.04,
    )
    acc_healthy_standby = AccountQuotaState(
        email="healthy@example.com",
        label="Healthy Standby",
        is_active=False,
        status="STANDBY",
        has_totp=True,
        totp_secret="",
        refresh_token="",
        quota_5h_current=0.90,
        reset_seconds=0.0,
        quota_weekly=0.80,
    )

    sorted_accounts = sort_account_quota_states(
        [acc_healthy_standby, acc_active_depleted],
        active_email="active_depleted@example.com",
        threshold=0.05,
        mode="auto",
    )

    assert sorted_accounts[0].email == "active_depleted@example.com"
    assert sorted_accounts[1].email == "healthy@example.com"


def test_sort_account_quota_states_switch_modes_and_tiers():
    """Verify that Free tier is demoted behind paid standbys and Ultra is prioritized in max_continuous."""
    acc_free = AccountQuotaState(
        email="free@example.com",
        label="Free User",
        is_active=False,
        status="STANDBY",
        has_totp=False,
        totp_secret="",
        refresh_token="",
        quota_5h_current=1.0,
        reset_seconds=0.0,
        quota_weekly=1.0,
        plan_tier="Free",
    )
    acc_pro = AccountQuotaState(
        email="pro@example.com",
        label="Pro User",
        is_active=False,
        status="STANDBY",
        has_totp=False,
        totp_secret="",
        refresh_token="",
        quota_5h_current=0.6,
        reset_seconds=0.0,
        quota_weekly=0.7,
        plan_tier="Pro",
    )
    acc_ultra = AccountQuotaState(
        email="ultra@example.com",
        label="Ultra User",
        is_active=False,
        status="STANDBY",
        has_totp=False,
        totp_secret="",
        refresh_token="",
        quota_5h_current=0.9,
        reset_seconds=0.0,
        quota_weekly=0.9,
        plan_tier="Ultra 20X",
    )
    acc_active = AccountQuotaState(
        email="active@example.com",
        label="Active User",
        is_active=True,
        status="ACTIVE",
        has_totp=False,
        totp_secret="",
        refresh_token="",
        quota_5h_current=0.8,
        reset_seconds=0.0,
        quota_weekly=0.8,
        plan_tier="Pro",
    )

    # 1. Balanced: Free (100%) must be placed in Tier 2, behind Pro (60%) in Tier 1
    sorted_bal = sort_account_quota_states([acc_free, acc_pro, acc_active], active_email="active@example.com", threshold=0.05, mode="auto", switch_mode="balanced")
    assert sorted_bal[0].email == "active@example.com"
    assert sorted_bal[1].email == "pro@example.com"
    assert sorted_bal[2].email == "free@example.com"

    # 2. Max Continuous: Ultra 20X ranked ahead of Pro
    sorted_cont = sort_account_quota_states([acc_free, acc_pro, acc_ultra, acc_active], active_email="active@example.com", threshold=0.05, mode="auto", switch_mode="max_continuous")
    assert sorted_cont[0].email == "active@example.com"
    assert sorted_cont[1].email == "ultra@example.com"
    assert sorted_cont[2].email == "pro@example.com"
    assert sorted_cont[3].email == "free@example.com"


