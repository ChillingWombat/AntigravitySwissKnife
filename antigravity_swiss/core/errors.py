"""
Antigravity Swiss Knife Domain Exceptions and JSON-RPC Error Mapping.
=====================================================================
Hierarchy of all domain errors, mapping cleanly to JSON-RPC 2.0 error responses.
"""

from __future__ import annotations

from typing import Any


class SwissKnifeError(Exception):
    """Base exception for all Antigravity Swiss Knife errors with JSON-RPC error mapping."""

    def __init__(self, message: str, code: int = -32000, data: Any = None):
        super().__init__(message)
        self.message = message
        self.code = code
        self.data = data

    def to_rpc_error(self) -> dict[str, Any]:
        """Convert exception to standard JSON-RPC 2.0 error object."""
        err: dict[str, Any] = {"code": self.code, "message": self.message}
        if self.data is not None:
            err["data"] = self.data
        return err


# Standard JSON-RPC 2.0 Specification Errors (-32700, -32600 to -32603)
class ParseError(SwissKnifeError):
    def __init__(self, message: str = "Invalid JSON was received by the server", data: Any = None):
        super().__init__(message, code=-32700, data=data)


class InvalidRequestError(SwissKnifeError):
    def __init__(self, message: str = "The JSON sent is not a valid Request object", data: Any = None):
        super().__init__(message, code=-32600, data=data)


class MethodNotFoundError(SwissKnifeError):
    def __init__(self, method: str, data: Any = None):
        super().__init__(f"The method '{method}' does not exist or is not available", code=-32601, data=data)


class InvalidParamsError(SwissKnifeError):
    def __init__(self, message: str = "Invalid method parameter(s)", data: Any = None):
        super().__init__(message, code=-32602, data=data)


class InternalRPCError(SwissKnifeError):
    def __init__(self, message: str = "Internal JSON-RPC error", data: Any = None):
        super().__init__(message, code=-32603, data=data)


# Domain Specific Errors (-32000 to -32099)
class IPCError(SwissKnifeError):
    """General IPC communication failure."""
    def __init__(self, message: str, code: int = -32001, data: Any = None):
        super().__init__(message, code=code, data=data)


class DaemonNotRunningError(IPCError):
    """Daemon socket is not active or unreachable."""
    def __init__(self, message: str = "Antigravity Swiss Knife daemon is not running", data: Any = None):
        super().__init__(message, code=-32002, data=data)


class DaemonAlreadyRunningError(IPCError):
    """Another daemon instance is already active and listening on the socket."""
    def __init__(self, message: str = "Antigravity Swiss Knife daemon is already running", data: Any = None):
        super().__init__(message, code=-32003, data=data)


class KeyringError(SwissKnifeError):
    """Base error for Linux Secret Service / Keyring operations."""
    def __init__(self, message: str, code: int = -32010, data: Any = None):
        super().__init__(message, code=code, data=data)


class KeyringLockedError(KeyringError):
    """Secret Service collection is locked and requires user authentication."""
    def __init__(self, message: str = "Linux Secret Service keyring collection is locked", data: Any = None):
        super().__init__(message, code=-32011, data=data)


class CredentialNotFoundError(KeyringError):
    """No matching credentials found in the Secret Service."""
    def __init__(self, message: str = "No credential found in Secret Service for service=gemini, username=antigravity", data: Any = None):
        super().__init__(message, code=-32012, data=data)


class KeyringNotFoundError(CredentialNotFoundError):
    """Alias for CredentialNotFoundError."""
    pass


class AccountNotFoundError(KeyringError):
    """Target account email does not exist in Swiss Knife accounts vault."""
    def __init__(self, email: str, data: Any = None):
        super().__init__(f"Account '{email}' not found in registered accounts", code=-32013, data=data)


class KeyringBackendUnavailableError(KeyringError):
    """Raised when the requested keyring backend binary or library is missing."""
    def __init__(self, message: str = "No functional keyring backend available", data: Any = None):
        super().__init__(message, code=-32014, data=data)


class InvalidCredentialError(KeyringError):
    """Raised when credential data is malformed or missing required tokens."""
    def __init__(self, message: str = "Invalid credential data", data: Any = None):
        super().__init__(message, code=-32015, data=data)


class AccountVaultCorruptedError(KeyringError):
    """Raised when accounts.json cannot be parsed as valid JSON."""
    def __init__(self, message: str = "Account vault file corrupted", data: Any = None):
        super().__init__(message, code=-32016, data=data)


class KeyringTimeoutError(KeyringError):
    """Raised when a keyring operation times out."""
    def __init__(self, message: str = "Keyring operation timed out", data: Any = None):
        super().__init__(message, code=-32017, data=data)


class ProcessError(SwissKnifeError):
    """Base error for Antigravity process management."""
    def __init__(self, message: str, code: int = -32020, data: Any = None):
        super().__init__(message, code=code, data=data)


class ProcessTerminationTimeoutError(ProcessError):
    """Antigravity failed to exit cleanly within the grace period."""
    def __init__(self, pid: int, timeout_sec: float, data: Any = None):
        super().__init__(f"Antigravity process (PID {pid}) did not exit within {timeout_sec}s", code=-32021, data=data)


class ProcessLaunchError(ProcessError):
    """Failed to spawn Antigravity binary."""
    def __init__(self, message: str, data: Any = None):
        super().__init__(message, code=-32022, data=data)


class StorageCorruptionError(SwissKnifeError):
    """Integrity failure in app_storage.json, SQLite WAL files, or state.vscdb."""
    def __init__(self, message: str, data: Any = None):
        super().__init__(message, code=-32023, data=data)


class QuotaError(SwissKnifeError):
    """Base error for upstream CloudCode quota operations."""
    def __init__(self, message: str, code: int = -32030, data: Any = None):
        super().__init__(message, code=code, data=data)


class QuotaAuthExpiredError(QuotaError):
    """Google OAuth refresh token expired or invalid."""
    def __init__(self, message: str = "Google OAuth refresh token expired or revoked", data: Any = None):
        super().__init__(message, code=-32031, data=data)


class QuotaNetworkError(QuotaError):
    """Network connection error communicating with Google CloudCode API."""
    def __init__(self, message: str = "Failed to connect to cloudcode-pa.googleapis.com", data: Any = None):
        super().__init__(message, code=-32032, data=data)


class QuotaRateLimitError(QuotaError):
    """Raised on HTTP 429 RESOURCE_EXHAUSTED."""
    def __init__(self, message: str = "Quota exhausted or rate limit reached", data: Any = None):
        super().__init__(message, code=-32033, data=data)


class QuotaUnavailableError(QuotaError):
    """Raised on HTTP 503 / 502 / 504 UNAVAILABLE."""
    def __init__(self, message: str = "Google CloudCode service temporarily unavailable", data: Any = None):
        super().__init__(message, code=-32034, data=data)


class FingerprintError(SwissKnifeError):
    """Error during hardware device profile generation or swapping."""
    def __init__(self, message: str, code: int = -32040, data: Any = None):
        super().__init__(message, code=code, data=data)


class CachePruneError(SwissKnifeError):
    """Error during brain or conversation cache pruning."""
    def __init__(self, message: str, code: int = -32050, data: Any = None):
        super().__init__(message, code=code, data=data)
