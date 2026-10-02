"""
Antigravity Swiss Knife - Keyring Management Subsystem.

Provides native Linux Secret Service integration (secret-tool & D-Bus / libsecret),
atomic multi-account switching, and secure accounts.json vault management.
"""

from .secret_tool import (
    DEFAULT_LABEL,
    DEFAULT_SERVICE,
    DEFAULT_USERNAME,
    DEFAULT_TIMEOUT_SEC,
    KeyringBackendUnavailableError,
    KeyringError,
    KeyringNotFoundError,
    KeyringTimeoutError,
    SecretToolBackend,
)
from .dbus_keyring import (
    DBusKeyring,
    KeyringBackendProtocol,
    LibsecretBackend,
    SecretStorageBackend,
)
from .switcher import (
    AccountNotFoundError,
    AccountRecord,
    AccountVault,
    AccountVaultCorruptedError,
    InvalidCredentialError,
    KeyringCredential,
    KeyringService,
    get_default_keyring_backend,
)

__all__ = [
    "DEFAULT_LABEL",
    "DEFAULT_SERVICE",
    "DEFAULT_TIMEOUT_SEC",
    "DEFAULT_USERNAME",
    "AccountNotFoundError",
    "AccountRecord",
    "AccountVault",
    "AccountVaultCorruptedError",
    "DBusKeyring",
    "InvalidCredentialError",
    "KeyringBackendProtocol",
    "KeyringBackendUnavailableError",
    "KeyringCredential",
    "KeyringError",
    "KeyringNotFoundError",
    "KeyringService",
    "KeyringTimeoutError",
    "LibsecretBackend",
    "SecretStorageBackend",
    "SecretToolBackend",
    "get_default_keyring_backend",
]
