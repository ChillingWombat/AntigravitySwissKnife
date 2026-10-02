"""
MFA / TOTP Authenticator Engine (RFC 6238).
===========================================
"""

from antigravity_swiss.totp.engine import TotpEngine, TotpResult

__all__ = ["TotpEngine", "TotpResult"]
