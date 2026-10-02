"""
Unit tests for RFC 6238 TOTP Engine (Feature F17).
==================================================
Validates against standard RFC 6238 Appendix B test vectors and countdown timers.
"""

import pytest
from antigravity_swiss.totp.engine import TotpEngine, TotpResult


def test_rfc6238_standard_test_vectors():
    """
    Verify TOTP implementation against official RFC 6238 Appendix B test vectors.
    Seed: '12345678901234567890' (ASCII 20 bytes).
    Base32 representation: 'GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ'.
    """
    secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

    # Vector 1: Time = 59 (Step 1) -> 287082
    res_59 = TotpEngine.get_current_totp(secret, timestamp=59.0)
    assert res_59.code == "287082"
    assert res_59.time_step == 1
    assert res_59.remaining_seconds == 1  # 59 % 30 = 29 -> remaining 1s

    # Vector 2: Time = 1111111109 (Step 37037036) -> 081804
    res_111 = TotpEngine.get_current_totp(secret, timestamp=1111111109.0)
    assert res_111.code == "081804"
    assert res_111.time_step == 37037036

    # Vector 3: Time = 1234567890 (Step 41152263) -> 005924
    res_123 = TotpEngine.get_current_totp(secret, timestamp=1234567890.0)
    assert res_123.code == "005924"
    assert res_123.time_step == 41152263


def test_secret_cleaning_and_padding_tolerance():
    """Verify secret sanitation handles spaces, hyphens, and unpadded strings."""
    raw = "gez dgnb vgy3 tqoj qgez dgnb vgy3 tqoj q"
    code1 = TotpEngine.get_current_totp(raw, timestamp=1000.0).code

    cleaned = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
    code2 = TotpEngine.get_current_totp(cleaned, timestamp=1000.0).code

    assert code1 == code2


def test_totp_countdown_and_progress_fraction():
    """Verify remaining_seconds and progress_fraction compute correctly across window."""
    secret = "JBSWY3DPEHPK3PXP"  # Standard test secret

    # Exactly at start of window: T = 30.0 -> rem = 30, progress = 1.0
    r0 = TotpEngine.get_current_totp(secret, timestamp=30.0)
    assert r0.remaining_seconds == 30
    assert r0.progress_fraction == 1.0

    # Halfway through window: T = 45.0 -> rem = 15, progress = 0.5
    r15 = TotpEngine.get_current_totp(secret, timestamp=45.0)
    assert r15.remaining_seconds == 15
    assert r15.progress_fraction == 0.5

    # 1 second before expiration: T = 59.0 -> rem = 1, progress = 1/30
    r29 = TotpEngine.get_current_totp(secret, timestamp=59.0)
    assert r29.remaining_seconds == 1
    assert round(r29.progress_fraction, 2) == 0.03


def test_totp_code_verification_with_drift():
    """Verify verify_code checks current and adjacent steps (+/- 1 window)."""
    secret = "JBSWY3DPEHPK3PXP"
    now = 1000.0

    current_code = TotpEngine.get_current_totp(secret, timestamp=now).code
    prev_code = TotpEngine.get_current_totp(secret, timestamp=now - 30.0).code
    next_code = TotpEngine.get_current_totp(secret, timestamp=now + 30.0).code
    stale_code = TotpEngine.get_current_totp(secret, timestamp=now - 90.0).code

    # Current code is valid
    assert TotpEngine.verify_code(secret, current_code, timestamp=now) is True
    # Prev code (+/- 1 step) is valid
    assert TotpEngine.verify_code(secret, prev_code, timestamp=now, window=1) is True
    # Next code (+/- 1 step) is valid
    assert TotpEngine.verify_code(secret, next_code, timestamp=now, window=1) is True
    # Stale code (-3 steps) is invalid
    assert TotpEngine.verify_code(secret, stale_code, timestamp=now, window=1) is False
    # Invalid code format
    assert TotpEngine.verify_code(secret, "000000", timestamp=now) is False
