"""
Pure Python / In-Process D-Bus Secret Service Keyring Backend.

Provides native Linux Secret Service access via libsecret (PyGObject) or
secretstorage (jeepney / D-Bus) without shelling out to external CLI tools.
Strictly sets schema org.freedesktop.Secret.Generic with attributes
service=gemini, username=antigravity (no extraneous 'application' attribute).
"""

from __future__ import annotations

import os
import sys
from typing import Protocol, runtime_checkable

try:
    from .proposed_secret_tool import (
        DEFAULT_LABEL,
        DEFAULT_SERVICE,
        DEFAULT_USERNAME,
        KeyringBackendUnavailableError,
        KeyringError,
        KeyringNotFoundError,
    )
except ImportError:
    from proposed_secret_tool import (
        DEFAULT_LABEL,
        DEFAULT_SERVICE,
        DEFAULT_USERNAME,
        KeyringBackendUnavailableError,
        KeyringError,
        KeyringNotFoundError,
    )


@runtime_checkable
class KeyringBackendProtocol(Protocol):
    """Protocol satisfied by SecretToolBackend, LibsecretBackend, and SecretStorageBackend."""

    def lookup(self, service: str | None = None, username: str | None = None) -> str | None:
        ...

    def store(
        self,
        secret: str | bytes,
        service: str | None = None,
        username: str | None = None,
        label: str | None = None,
    ) -> None:
        ...

    def clear(
        self,
        service: str | None = None,
        username: str | None = None,
        ignore_missing: bool = True,
    ) -> bool:
        ...


class LibsecretBackend:
    """
    In-process Secret Service backend using PyGObject (gi.repository.Secret).
    
    Talks directly to libsecret C library with zero subprocess overhead.
    """

    def __init__(
        self,
        default_service: str = DEFAULT_SERVICE,
        default_username: str = DEFAULT_USERNAME,
    ) -> None:
        self.default_service = default_service
        self.default_username = default_username
        self._secret_mod = None
        self._schema = None

    @classmethod
    def is_available(cls) -> bool:
        """Check if PyGObject and Secret-1 typelib are loadable."""
        try:
            # Check default path and add standard Linux typelib path if missing
            extra_path = "/usr/lib/x86_64-linux-gnu/girepository-1.0"
            if extra_path not in os.environ.get("GI_TYPELIB_PATH", ""):
                os.environ["GI_TYPELIB_PATH"] = (
                    f"{os.environ.get('GI_TYPELIB_PATH', '')}:{extra_path}".strip(":")
                )
            import gi
            gi.require_version("Secret", "1")
            from gi.repository import Secret  # type: ignore # noqa: F401
            return True
        except Exception:
            return False

    def _get_schema(self, Secret: Any) -> Any:
        if self._schema is None:
            self._schema = Secret.Schema.new(
                "org.freedesktop.Secret.Generic",
                Secret.SchemaFlags.NONE,
                {
                    "service": Secret.SchemaAttributeType.STRING,
                    "username": Secret.SchemaAttributeType.STRING,
                },
            )
        return self._schema

    def _ensure_module(self) -> Any:
        if self._secret_mod is None:
            if not self.is_available():
                raise KeyringBackendUnavailableError(
                    "PyGObject gi.repository.Secret (libsecret) is not available on this system."
                )
            import gi
            gi.require_version("Secret", "1")
            from gi.repository import Secret
            self._secret_mod = Secret
        return self._secret_mod

    def lookup(self, service: str | None = None, username: str | None = None) -> str | None:
        Secret = self._ensure_module()
        schema = self._get_schema(Secret)
        svc = service or self.default_service
        user = username or self.default_username

        try:
            res = Secret.password_lookup_sync(schema, {"service": svc, "username": user}, None)
            return res
        except Exception as exc:
            raise KeyringError(f"Libsecret lookup failed for {svc}/{user}: {exc}") from exc

    def store(
        self,
        secret: str | bytes,
        service: str | None = None,
        username: str | None = None,
        label: str | None = None,
    ) -> None:
        Secret = self._ensure_module()
        schema = self._get_schema(Secret)
        svc = service or self.default_service
        user = username or self.default_username
        lbl = label or f"Password for '{user}' on '{svc}'"

        if isinstance(secret, bytes):
            secret_str = secret.rstrip(b"\r\n").decode("utf-8")
        elif isinstance(secret, str):
            secret_str = secret.rstrip("\r\n")
        else:
            raise TypeError(f"Secret must be str or bytes, got {type(secret)}")

        try:
            success = Secret.password_store_sync(
                schema,
                {"service": svc, "username": user},
                Secret.COLLECTION_DEFAULT,
                lbl,
                secret_str,
                None,
            )
            if not success:
                raise KeyringError(f"Libsecret password_store_sync returned False for {svc}/{user}")
        except Exception as exc:
            raise KeyringError(f"Libsecret store failed: {exc}") from exc

    def clear(
        self,
        service: str | None = None,
        username: str | None = None,
        ignore_missing: bool = True,
    ) -> bool:
        Secret = self._ensure_module()
        schema = self._get_schema(Secret)
        svc = service or self.default_service
        user = username or self.default_username

        try:
            cleared = Secret.password_clear_sync(schema, {"service": svc, "username": user}, None)
            if not cleared and not ignore_missing:
                raise KeyringNotFoundError(f"No secret found to clear for {svc}/{user}")
            return bool(cleared)
        except KeyringNotFoundError:
            raise
        except Exception as exc:
            raise KeyringError(f"Libsecret clear failed: {exc}") from exc


class SecretStorageBackend:
    """
    Pure Python Secret Service backend using the `secretstorage` package (D-Bus over jeepney).
    """

    def __init__(
        self,
        default_service: str = DEFAULT_SERVICE,
        default_username: str = DEFAULT_USERNAME,
    ) -> None:
        self.default_service = default_service
        self.default_username = default_username
        self._connection = None
        self._collection = None

    @classmethod
    def is_available(cls) -> bool:
        """Check if secretstorage can be imported and session D-Bus is accessible."""
        try:
            import secretstorage
            conn = secretstorage.dbus_init()
            col = secretstorage.get_default_collection(conn)
            return col is not None
        except Exception:
            return False

    def _ensure_connected(self) -> Any:
        import secretstorage
        if self._connection is None:
            try:
                self._connection = secretstorage.dbus_init()
                self._collection = secretstorage.get_default_collection(self._connection)
                if self._collection.is_locked():
                    self._collection.unlock()
            except Exception as exc:
                raise KeyringError(f"Failed to connect to Secret Service via D-Bus: {exc}") from exc
        return self._collection

    def lookup(self, service: str | None = None, username: str | None = None) -> str | None:
        col = self._ensure_connected()
        svc = service or self.default_service
        user = username or self.default_username

        try:
            items = list(col.search_items({"service": svc, "username": user}))
            if not items:
                return None
            raw_bytes = items[0].get_secret()
            return raw_bytes.decode("utf-8")
        except Exception as exc:
            raise KeyringError(f"secretstorage lookup failed: {exc}") from exc

    def store(
        self,
        secret: str | bytes,
        service: str | None = None,
        username: str | None = None,
        label: str | None = None,
    ) -> None:
        col = self._ensure_connected()
        svc = service or self.default_service
        user = username or self.default_username
        lbl = label or f"Password for '{user}' on '{svc}'"

        if isinstance(secret, str):
            payload_bytes = secret.rstrip("\r\n").encode("utf-8")
        elif isinstance(secret, bytes):
            payload_bytes = secret.rstrip(b"\r\n")
        else:
            raise TypeError(f"Secret must be str or bytes, got {type(secret)}")

        attrs = {"service": svc, "username": user}
        try:
            items = list(col.search_items(attrs))
            if items:
                items[0].set_secret(payload_bytes)
                items[0].set_label(lbl)
            else:
                col.create_item(
                    lbl,
                    attrs,
                    payload_bytes,
                    replace=True,
                    content_type="text/plain; charset=utf-8",
                )
        except Exception as exc:
            raise KeyringError(f"secretstorage store failed: {exc}") from exc

    def clear(
        self,
        service: str | None = None,
        username: str | None = None,
        ignore_missing: bool = True,
    ) -> bool:
        col = self._ensure_connected()
        svc = service or self.default_service
        user = username or self.default_username

        try:
            items = list(col.search_items({"service": svc, "username": user}))
            if not items:
                if not ignore_missing:
                    raise KeyringNotFoundError(f"No secret found to clear for {svc}/{user}")
                return False
            for item in items:
                item.delete()
            return True
        except KeyringNotFoundError:
            raise
        except Exception as exc:
            raise KeyringError(f"secretstorage clear failed: {exc}") from exc


class DBusKeyring:
    """
    Unified In-Process D-Bus Keyring.
    
    Automatically selects LibsecretBackend (if available) or SecretStorageBackend.
    """

    def __init__(
        self,
        default_service: str = DEFAULT_SERVICE,
        default_username: str = DEFAULT_USERNAME,
    ) -> None:
        self.default_service = default_service
        self.default_username = default_username
        self._backend: KeyringBackendProtocol | None = None

    @classmethod
    def is_available(cls) -> bool:
        return LibsecretBackend.is_available() or SecretStorageBackend.is_available()

    def _get_backend(self) -> KeyringBackendProtocol:
        if self._backend is None:
            if LibsecretBackend.is_available():
                self._backend = LibsecretBackend(self.default_service, self.default_username)
            elif SecretStorageBackend.is_available():
                self._backend = SecretStorageBackend(self.default_service, self.default_username)
            else:
                raise KeyringBackendUnavailableError(
                    "No functional D-Bus Secret Service backend available (neither libsecret nor secretstorage)."
                )
        return self._backend

    def lookup(self, service: str | None = None, username: str | None = None) -> str | None:
        return self._get_backend().lookup(service, username)

    def store(
        self,
        secret: str | bytes,
        service: str | None = None,
        username: str | None = None,
        label: str | None = None,
    ) -> None:
        self._get_backend().store(secret, service, username, label)

    def clear(
        self,
        service: str | None = None,
        username: str | None = None,
        ignore_missing: bool = True,
    ) -> bool:
        return self._get_backend().clear(service, username, ignore_missing)
