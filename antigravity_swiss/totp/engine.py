"""
RFC 6238 Time-Based One-Time Password (TOTP) Engine (Feature F17).
==================================================================
Pure Python standard library implementation of RFC 6238 / RFC 4226:
- HMAC-SHA1 over 30-second time steps
- Standard Base32 secret key decoding (with padding tolerance)
- Dynamic truncation to 6-digit verification codes
- Live remaining countdown calculations (30s window) and ±1 drift validation
"""

from __future__ import annotations

import base64
from dataclasses import dataclass
import hashlib
import hmac
import struct
import time
from typing import Optional, Tuple


@dataclass
class TotpResult:
    """Live computed TOTP code and countdown status."""
    code: str                  # 6-digit string, e.g. "482019"
    remaining_seconds: int     # [1, 30]
    progress_fraction: float   # [0.0, 1.0] for animation rings
    time_step: int             # epoch // 30


class TotpEngine:
    """Pure standard library RFC 6238 TOTP authenticator engine."""

    DEFAULT_STEP_SECONDS: int = 30
    DEFAULT_DIGITS: int = 6

    @staticmethod
    def sanitize_secret(secret: str) -> str:
        """Strips whitespace, hyphens, and converts to uppercase."""
        return secret.replace(" ", "").replace("-", "").upper()

    @classmethod
    def decode_secret(cls, secret: str) -> bytes:
        """
        Decodes a Base32 secret into bytes, padding with '=' as required by RFC 4648.
        """
        clean = cls.sanitize_secret(secret)
        if not clean:
            raise ValueError("Base32 secret cannot be empty")
        # Pad to multiple of 8
        missing_padding = len(clean) % 8
        if missing_padding:
            clean += "=" * (8 - missing_padding)
        try:
            return base64.b32decode(clean, casefold=True)
        except Exception as exc:
            raise ValueError(f"Invalid Base32 secret key: {exc}") from exc

    @classmethod
    def validate_secret(cls, secret: str) -> bool:
        """Checks if a secret is valid Base32 format."""
        try:
            cls.decode_secret(secret)
            return True
        except Exception:
            return False

    @classmethod
    def generate_code_at_step(
        cls,
        secret: str,
        step: int,
        digits: int = DEFAULT_DIGITS,
    ) -> str:
        """
        Computes RFC 4226 HOTP dynamic truncation for a specific time step.
        """
        key = cls.decode_secret(secret)
        # Pack 64-bit integer big-endian
        counter_bytes = struct.pack(">Q", step)

        # HMAC-SHA1 computation
        mac = hmac.new(key, counter_bytes, hashlib.sha1).digest()

        # Dynamic truncation (RFC 4226 §5.3)
        offset = mac[-1] & 0x0F
        binary = struct.unpack(">I", mac[offset:offset + 4])[0] & 0x7FFFFFFF
        code_int = binary % (10 ** digits)

        return f"{code_int:0{digits}d}"

    @classmethod
    def get_current_totp(
        cls,
        secret: str,
        timestamp: Optional[float] = None,
        step_seconds: int = DEFAULT_STEP_SECONDS,
        digits: int = DEFAULT_DIGITS,
    ) -> TotpResult:
        """
        Generates current 6-digit TOTP code, remaining seconds, and progress fraction.
        """
        now = time.time() if timestamp is None else timestamp
        step = int(now // step_seconds)
        rem_sec = step_seconds - int(now % step_seconds)
        if rem_sec == 0:
            rem_sec = step_seconds

        progress = rem_sec / float(step_seconds)
        code = cls.generate_code_at_step(secret, step, digits=digits)

        return TotpResult(
            code=code,
            remaining_seconds=rem_sec,
            progress_fraction=progress,
            time_step=step,
        )

    @classmethod
    def verify_code(
        cls,
        secret: str,
        code: str,
        timestamp: Optional[float] = None,
        step_seconds: int = DEFAULT_STEP_SECONDS,
        window: int = 1,
    ) -> bool:
        """
        Verifies a user-provided 6-digit code with +/- window drift tolerance.
        """
        now = time.time() if timestamp is None else timestamp
        current_step = int(now // step_seconds)
        clean_code = code.strip().replace(" ", "")

        for delta in range(-window, window + 1):
            check_step = current_step + delta
            expected = cls.generate_code_at_step(secret, check_step)
            if hmac.compare_digest(clean_code, expected):
                return True
        return False


TOTPEngine = TotpEngine

__all__ = ["TotpEngine", "TOTPEngine", "TotpResult"]
