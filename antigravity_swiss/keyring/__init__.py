"""
Antigravity Swiss Knife Keyring Package.
========================================
Linux Secret Service integration and atomic account switcher.
"""

from antigravity_swiss.keyring.dbus_keyring import DBusKeyring, LibsecretBackend, SecretStorageBackend
from antigravity_swiss.keyring.secret_tool import SecretToolBackend, SecretToolWrapper
from antigravity_swiss.keyring.switcher import (
    AccountRecord,
    AccountStore,
    AccountVault,
    KeyringCredential,
    KeyringService,
    KeyringSwitcher,
    get_default_keyring_backend,
)

__all__ = [
    "SecretToolBackend",
    "SecretToolWrapper",
    "DBusKeyring",
    "LibsecretBackend",
    "SecretStorageBackend",
    "KeyringCredential",
    "KeyringService",
    "AccountRecord",
    "AccountVault",
    "AccountStore",
    "KeyringSwitcher",
    "get_default_keyring_backend",
]
