"""
Unit Tests for Quota Calculator & Fleet Synthesis Engine.
"""

import pytest
from antigravity_swiss.quota.calculator import (
    AccountQuotaState,
    build_account_quota_states,
    compute_fleet_quota_summary,
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
