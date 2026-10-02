"""
Linux Secret Service CLI wrapper using /usr/bin/secret-tool.

Matches the behavior and attributes used by Antigravity 2.0 language_server
(zalando/go-keyring): schema org.freedesktop.Secret.Generic with attributes
service=gemini, username=antigravity.
"""

from __future__ import annotations

import os
import shutil
import subprocess
from typing import Any, Mapping


DEFAULT_SERVICE = "gemini"
DEFAULT_USERNAME = "antigravity"
DEFAULT_LABEL = "Password for 'antigravity' on 'gemini'"
DEFAULT_TIMEOUT_SEC = 10.0


class KeyringError(RuntimeError):
    """Base exception for keyring operations."""


class KeyringNotFoundError(KeyringError):
    """Raised when a requested secret was not found in the keyring."""


class KeyringTimeoutError(KeyringError):
    """Raised when a keyring operation times out."""


class KeyringBackendUnavailableError(KeyringError):
    """Raised when the requested keyring backend binary or library is missing."""


class SecretToolBackend:
    """
    Subprocess wrapper around /usr/bin/secret-tool.
    
    Provides lookup, store, clear, and search operations with strict handling of:
    - Zero trailing newlines (0x0a) on stdin during store
    - Returncode 1 on missing secret during lookup
    - Returncode 1 on clearing non-existent secret (treated as idempotent success)
    - Stderr parsing and configurable subprocess timeouts
    """

    def __init__(
        self,
        binary_path: str | None = None,
        default_service: str = DEFAULT_SERVICE,
        default_username: str = DEFAULT_USERNAME,
        default_timeout: float = DEFAULT_TIMEOUT_SEC,
    ) -> None:
        self.binary_path = binary_path or shutil.which("secret-tool") or "/usr/bin/secret-tool"
        self.default_service = default_service
        self.default_username = default_username
        self.default_timeout = default_timeout

    @classmethod
    def is_available(cls, binary_path: str | None = None) -> bool:
        """Check if secret-tool executable exists and is runnable."""
        path = binary_path or shutil.which("secret-tool") or "/usr/bin/secret-tool"
        return os.path.isfile(path) and os.access(path, os.X_OK)

    def _ensure_available(self) -> None:
        if not self.is_available(self.binary_path):
            raise KeyringBackendUnavailableError(
                f"secret-tool binary not found or not executable at '{self.binary_path}'"
            )

    def lookup(
        self,
        service: str | None = None,
        username: str | None = None,
        timeout: float | None = None,
    ) -> str | None:
        """
        Lookup secret matching service and username attributes.
        
        Returns the raw secret string, or None if not found.
        Raises KeyringError or KeyringTimeoutError on failure.
        """
        self._ensure_available()
        svc = service or self.default_service
        user = username or self.default_username
        to = timeout or self.default_timeout

        cmd = [self.binary_path, "lookup", "service", svc, "username", user]
        try:
            res = subprocess.run(
                cmd,
                capture_output=True,
                text=False,  # Read raw bytes to inspect exact byte count
                timeout=to,
                env=os.environ,
            )
        except subprocess.TimeoutExpired as exc:
            raise KeyringTimeoutError(
                f"secret-tool lookup timed out after {to}s for service={svc}, username={user}"
            ) from exc
        except OSError as exc:
            raise KeyringError(f"Failed to execute secret-tool: {exc}") from exc

        # Return code 0: Secret found
        if res.returncode == 0:
            return res.stdout.decode("utf-8")

        # Return code 1 with empty stdout: Not found in keyring
        if res.returncode == 1 and not res.stdout:
            return None

        # Non-zero return with stderr: Real error
        stderr_msg = res.stderr.decode("utf-8", errors="replace").strip()
        raise KeyringError(
            f"secret-tool lookup failed (exit {res.returncode}): {stderr_msg or 'unknown error'}"
        )

    def store(
        self,
        secret: str | bytes,
        service: str | None = None,
        username: str | None = None,
        label: str | None = None,
        timeout: float | None = None,
    ) -> None:
        """
        Store secret for given service and username.
        
        Strictly strips any trailing newline (0x0a / \r\n) before writing to stdin
        to prevent token corruption in zalando/go-keyring consumers.
        """
        self._ensure_available()
        svc = service or self.default_service
        user = username or self.default_username
        lbl = label or f"Password for '{user}' on '{svc}'"
        to = timeout or self.default_timeout

        # Sanitize payload: strip any accidental trailing newline bytes
        if isinstance(secret, str):
            payload_bytes = secret.rstrip("\r\n").encode("utf-8")
        elif isinstance(secret, (bytes, bytearray)):
            payload_bytes = bytes(secret).rstrip(b"\r\n")
        else:
            raise TypeError(f"Secret must be str or bytes, got {type(secret)}")

        cmd = [
            self.binary_path,
            "store",
            f"--label={lbl}",
            "service",
            svc,
            "username",
            user,
        ]

        try:
            res = subprocess.run(
                cmd,
                input=payload_bytes,
                capture_output=True,
                timeout=to,
                env=os.environ,
            )
        except subprocess.TimeoutExpired as exc:
            raise KeyringTimeoutError(
                f"secret-tool store timed out after {to}s for service={svc}, username={user}"
            ) from exc
        except OSError as exc:
            raise KeyringError(f"Failed to execute secret-tool: {exc}") from exc

        if res.returncode != 0:
            stderr_msg = res.stderr.decode("utf-8", errors="replace").strip()
            raise KeyringError(
                f"secret-tool store failed (exit {res.returncode}): {stderr_msg or 'unknown error'}"
            )

    def clear(
        self,
        service: str | None = None,
        username: str | None = None,
        ignore_missing: bool = True,
        timeout: float | None = None,
    ) -> bool:
        """
        Clear secret matching service and username.
        
        Returns True if secret was found and deleted.
        Returns False if secret did not exist.
        If ignore_missing is False and secret did not exist, raises KeyringNotFoundError.
        """
        self._ensure_available()
        svc = service or self.default_service
        user = username or self.default_username
        to = timeout or self.default_timeout

        cmd = [self.binary_path, "clear", "service", svc, "username", user]

        try:
            res = subprocess.run(
                cmd,
                capture_output=True,
                timeout=to,
                env=os.environ,
            )
        except subprocess.TimeoutExpired as exc:
            raise KeyringTimeoutError(
                f"secret-tool clear timed out after {to}s for service={svc}, username={user}"
            ) from exc
        except OSError as exc:
            raise KeyringError(f"Failed to execute secret-tool: {exc}") from exc

        # Return code 0: Successfully deleted
        if res.returncode == 0:
            return True

        # Return code 1 with empty stdout/stderr: Not found / already clear
        if res.returncode == 1 and not res.stdout and not res.stderr:
            if not ignore_missing:
                raise KeyringNotFoundError(
                    f"No secret found to clear for service={svc}, username={user}"
                )
            return False

        stderr_msg = res.stderr.decode("utf-8", errors="replace").strip()
        raise KeyringError(
            f"secret-tool clear failed (exit {res.returncode}): {stderr_msg or 'unknown error'}"
        )

    def search(
        self,
        service: str | None = None,
        username: str | None = None,
        timeout: float | None = None,
    ) -> list[dict[str, str]]:
        """
        Search for items matching service and username.
        
        Returns parsed list of item dictionaries containing label, secret, created, modified, attributes.
        """
        self._ensure_available()
        svc = service or self.default_service
        user = username or self.default_username
        to = timeout or self.default_timeout

        cmd = [self.binary_path, "search", "service", svc, "username", user]
        try:
            res = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=to,
                env=os.environ,
            )
        except subprocess.TimeoutExpired as exc:
            raise KeyringTimeoutError(f"secret-tool search timed out after {to}s") from exc
        except OSError as exc:
            raise KeyringError(f"Failed to execute secret-tool: {exc}") from exc

        if res.returncode != 0:
            return []

        # Parse secret-tool search output blocks: [/item_id]\nkey = value\n...
        items: list[dict[str, str]] = []
        current: dict[str, str] = {}
        for line in res.stdout.splitlines():
            line = line.strip()
            if line.startswith("[/") and line.endswith("]"):
                if current:
                    items.append(current)
                current = {"id": line.strip("[]")}
            elif " = " in line:
                k, v = line.split(" = ", 1)
                current[k.strip()] = v.strip()

        if current:
            items.append(current)

        return items
