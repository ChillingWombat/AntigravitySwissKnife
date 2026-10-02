"""
Multi-Account Vault & Atomic Keyring Switcher.

Manages account vault storage in ~/.config/antigravity-swiss/accounts.json
with strict 0600 file permissions and provides atomic credential rotation
in the Linux Secret Service according to PROJECT.md § Interface Contracts.
"""

from __future__ import annotations

import base64
import datetime
import fcntl
import json
import logging
import os
import shutil
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any, Callable, NamedTuple, Sequence

try:
    from .proposed_dbus_keyring import DBusKeyring, KeyringBackendProtocol
    from .proposed_secret_tool import (
        DEFAULT_LABEL,
        DEFAULT_SERVICE,
        DEFAULT_USERNAME,
        KeyringBackendUnavailableError,
        KeyringError,
        KeyringNotFoundError,
        SecretToolBackend,
    )
except ImportError:
    from proposed_dbus_keyring import DBusKeyring, KeyringBackendProtocol
    from proposed_secret_tool import (
        DEFAULT_LABEL,
        DEFAULT_SERVICE,
        DEFAULT_USERNAME,
        KeyringBackendUnavailableError,
        KeyringError,
        KeyringNotFoundError,
        SecretToolBackend,
    )


logger = logging.getLogger("antigravity_swiss.keyring")

DEFAULT_CONFIG_DIR = Path.home() / ".config" / "antigravity-swiss"
DEFAULT_ACCOUNTS_FILE = DEFAULT_CONFIG_DIR / "accounts.json"
DEFAULT_LOCK_FILE = DEFAULT_CONFIG_DIR / "accounts.lock"
ANTIGRAVITY_STORAGE_FILE = Path.home() / ".config" / "Antigravity" / "app_storage.json"


class AccountNotFoundError(KeyringError):
    """Raised when an account email is not found in the vault."""


class InvalidCredentialError(KeyringError):
    """Raised when credential data is malformed or missing required tokens."""


class AccountVaultCorruptedError(KeyringError):
    """Raised when accounts.json cannot be parsed as valid JSON."""


class KeyringCredential(NamedTuple):
    """
    Antigravity OAuth credential contract matching PROJECT.md § Interface Contracts.
    """

    access_token: str
    refresh_token: str
    token_type: str = "Bearer"
    expiry: str = ""
    auth_method: str = "consumer"
    id_token: str = ""

    def to_antigravity_json(self) -> str:
        """
        Serialize to the exact JSON format expected by Antigravity language_server.
        Matches Go struct: { token: { access_token, token_type, refresh_token, expiry }, auth_method, id_token }
        """
        payload: dict[str, Any] = {
            "token": {
                "access_token": self.access_token,
                "token_type": self.token_type or "Bearer",
                "refresh_token": self.refresh_token,
                "expiry": self.expiry,
            },
            "auth_method": self.auth_method or "consumer",
        }
        if self.id_token:
            payload["id_token"] = self.id_token
        return json.dumps(payload, separators=(",", ":"))

    def to_dict(self) -> dict[str, Any]:
        """Convert to dictionary for storage in accounts.json."""
        return {
            "access_token": self.access_token,
            "refresh_token": self.refresh_token,
            "token_type": self.token_type,
            "expiry": self.expiry,
            "auth_method": self.auth_method,
            "id_token": self.id_token,
        }

    @classmethod
    def from_antigravity_json(cls, raw: str) -> KeyringCredential:
        """
        Parse raw JSON retrieved from Linux Secret Service.
        """
        if not raw or not raw.strip():
            raise InvalidCredentialError("Empty or blank secret payload from keyring")

        try:
            data = json.loads(raw)
        except json.JSONDecodeError as exc:
            raise InvalidCredentialError(f"Secret payload is not valid JSON: {exc}") from exc

        if not isinstance(data, dict):
            raise InvalidCredentialError(f"Expected JSON object in keyring, got {type(data)}")

        token_obj = data.get("token", {})
        if not isinstance(token_obj, dict):
            token_obj = {}

        access_token = token_obj.get("access_token") or data.get("access_token", "")
        refresh_token = token_obj.get("refresh_token") or data.get("refresh_token", "")
        token_type = token_obj.get("token_type") or data.get("token_type", "Bearer")
        expiry = str(token_obj.get("expiry") or data.get("expiry", ""))
        auth_method = str(data.get("auth_method", "consumer"))
        id_token = str(data.get("id_token") or token_obj.get("id_token", ""))

        if not access_token and not refresh_token:
            raise InvalidCredentialError(
                "Credential JSON contains neither access_token nor refresh_token"
            )

        return cls(
            access_token=access_token,
            refresh_token=refresh_token,
            token_type=token_type,
            expiry=expiry,
            auth_method=auth_method,
            id_token=id_token,
        )

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> KeyringCredential:
        """Construct from dictionary stored in accounts.json."""
        return cls(
            access_token=str(data.get("access_token", "")),
            refresh_token=str(data.get("refresh_token", "")),
            token_type=str(data.get("token_type", "Bearer")),
            expiry=str(data.get("expiry", "")),
            auth_method=str(data.get("auth_method", "consumer")),
            id_token=str(data.get("id_token", "")),
        )

    def extract_email_from_id_token(self) -> str | None:
        """Extract email claim from id_token JWT payload without external dependencies."""
        if not self.id_token or self.id_token.count(".") != 2:
            return None
        try:
            parts = self.id_token.split(".")
            payload_b64 = parts[1]
            rem = len(payload_b64) % 4
            if rem > 0:
                payload_b64 += "=" * (4 - rem)
            payload_json = base64.urlsafe_b64decode(payload_b64).decode("utf-8")
            payload = json.loads(payload_json)
            return payload.get("email")
        except Exception:
            return None


@dataclass
class AccountRecord:
    """Record of a managed account inside accounts.json."""

    email: str
    label: str
    credential: KeyringCredential
    added_at: str
    last_used_at: str | None = None
    totp_secret: str = ""
    is_healthy: bool = True

    def to_dict(self) -> dict[str, Any]:
        return {
            "email": self.email,
            "label": self.label,
            "credential": self.credential.to_dict(),
            "added_at": self.added_at,
            "last_used_at": self.last_used_at,
            "totp_secret": self.totp_secret,
            "is_healthy": self.is_healthy,
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> AccountRecord:
        cred_dict = data.get("credential", {})
        return cls(
            email=str(data["email"]),
            label=str(data.get("label", "")),
            credential=KeyringCredential.from_dict(cred_dict),
            added_at=str(data.get("added_at", "")),
            last_used_at=data.get("last_used_at"),
            totp_secret=str(data.get("totp_secret", "")),
            is_healthy=bool(data.get("is_healthy", True)),
        )


class AccountVault:
    """
    Storage manager for ~/.config/antigravity-swiss/accounts.json.
    
    Guarantees:
    - Parent directory created with 0700 permissions
    - accounts.json created and updated with strict 0600 permissions
    - Multi-process safe concurrency via fcntl.flock on accounts.lock
    - Atomic file replacement via temporary file
    """

    def __init__(
        self,
        config_path: Path | str | None = None,
        lock_path: Path | str | None = None,
    ) -> None:
        self.config_path = Path(config_path or DEFAULT_ACCOUNTS_FILE).expanduser().resolve()
        self.config_dir = self.config_path.parent
        if lock_path is not None:
            self.lock_path = Path(lock_path).expanduser().resolve()
        else:
            self.lock_path = self.config_dir / f"{self.config_path.stem}.lock"

    def _ensure_dir(self) -> None:
        """Create config and lock directory with mode 0700 if not existing."""
        for d in (self.config_dir, self.lock_path.parent):
            if not d.exists():
                d.mkdir(parents=True, mode=0o700, exist_ok=True)
            else:
                current_mode = d.stat().st_mode & 0o777
                if current_mode != 0o700:
                    try:
                        d.chmod(0o700)
                    except OSError:
                        pass

    def _lock(self) -> Any:
        """Acquire exclusive file lock for concurrent process safety."""
        self._ensure_dir()
        lock_fd = os.open(str(self.lock_path), os.O_CREAT | os.O_RDWR, 0o600)
        fcntl.flock(lock_fd, fcntl.LOCK_EX)
        return lock_fd

    def _unlock(self, lock_fd: Any) -> None:
        try:
            fcntl.flock(lock_fd, fcntl.LOCK_UN)
            os.close(lock_fd)
        except OSError:
            pass

    def load(self) -> dict[str, Any]:
        """
        Load vault data from disk. Returns dictionary with version, active_account, and accounts.
        """
        lock_fd = self._lock()
        try:
            if not self.config_path.exists():
                return {"version": 1, "active_account": None, "accounts": {}}

            try:
                with open(self.config_path, "r", encoding="utf-8") as f:
                    content = f.read().strip()
                    if not content:
                        return {"version": 1, "active_account": None, "accounts": {}}
                    data = json.loads(content)
            except json.JSONDecodeError as exc:
                logger.error("Corrupted accounts.json detected: %s", exc)
                raise AccountVaultCorruptedError(f"Failed to parse {self.config_path}: {exc}") from exc

            # Verify permissions
            st = self.config_path.stat()
            if (st.st_mode & 0o777) != 0o600:
                try:
                    self.config_path.chmod(0o600)
                except OSError:
                    pass

            return data
        finally:
            self._unlock(lock_fd)

    def save(self, data: dict[str, Any]) -> None:
        """
        Atomically save vault data to disk with 0600 mode.
        """
        self._ensure_dir()
        lock_fd = self._lock()
        try:
            tmp_path = self.config_dir / f"{self.config_path.name}.tmp.{os.getpid()}"
            flags = os.O_WRONLY | os.O_CREAT | os.O_TRUNC
            fd = os.open(str(tmp_path), flags, 0o600)
            try:
                # Ensure 0600 regardless of umask
                os.fchmod(fd, 0o600)
                with os.fdopen(fd, "w", encoding="utf-8") as f:
                    json.dump(data, f, indent=2, sort_keys=True)
                    f.flush()
                    os.fsync(fd)
            except Exception:
                if tmp_path.exists():
                    tmp_path.unlink()
                raise

            # Atomic rename/replace
            os.replace(str(tmp_path), str(self.config_path))
        finally:
            self._unlock(lock_fd)

    def list_accounts(self) -> list[str]:
        data = self.load()
        return sorted(list(data.get("accounts", {}).keys()))

    def get_account(self, email: str) -> AccountRecord | None:
        data = self.load()
        acc = data.get("accounts", {}).get(email)
        if not acc:
            return None
        return AccountRecord.from_dict(acc)

    def get_active_account(self) -> str | None:
        data = self.load()
        return data.get("active_account")

    def set_active_account(self, email: str | None) -> None:
        data = self.load()
        if email is not None and email not in data.get("accounts", {}):
            raise AccountNotFoundError(f"Account '{email}' does not exist in vault")
        data["active_account"] = email
        if email:
            data["accounts"][email]["last_used_at"] = (
                datetime.datetime.now(datetime.timezone.utc).isoformat()
            )
        self.save(data)

    def add_or_update_account(
        self,
        email: str,
        credential: KeyringCredential,
        label: str = "",
        totp_secret: str = "",
        is_healthy: bool = True,
    ) -> AccountRecord:
        data = self.load()
        accounts = data.setdefault("accounts", {})
        now_iso = datetime.datetime.now(datetime.timezone.utc).isoformat()

        if email in accounts:
            record = AccountRecord.from_dict(accounts[email])
            record.credential = credential
            if label:
                record.label = label
            if totp_secret:
                record.totp_secret = totp_secret
            record.is_healthy = is_healthy
        else:
            record = AccountRecord(
                email=email,
                label=label or email,
                credential=credential,
                added_at=now_iso,
                totp_secret=totp_secret,
                is_healthy=is_healthy,
            )

        accounts[email] = record.to_dict()
        if data.get("active_account") is None:
            data["active_account"] = email

        self.save(data)
        return record

    def remove_account(self, email: str) -> bool:
        data = self.load()
        accounts = data.get("accounts", {})
        if email not in accounts:
            return False
        del accounts[email]
        if data.get("active_account") == email:
            # Pick next remaining account or None
            data["active_account"] = next(iter(accounts.keys())) if accounts else None
        self.save(data)
        return True


def get_default_keyring_backend() -> KeyringBackendProtocol:
    """
    Select the optimal keyring backend:
    1. SecretToolBackend (native CLI, 100% zalando/go-keyring compatible, zero python dependencies)
    2. DBusKeyring (in-process libsecret / secretstorage)
    """
    if SecretToolBackend.is_available():
        return SecretToolBackend()
    if DBusKeyring.is_available():
        return DBusKeyring()
    raise KeyringBackendUnavailableError(
        "Neither secret-tool nor a functional D-Bus Secret Service backend is available on this system."
    )


class KeyringService:
    """
    Implementation of PROJECT.md § KeyringService contract.
    
    Provides:
    - get_active_credential() -> KeyringCredential
    - set_active_credential(cred: KeyringCredential) -> None
    - list_accounts() -> list[str]
    - switch_account(account_email: str) -> bool
    
    Plus multi-account vault synchronization, auto-ingestion, and event emission.
    """

    def __init__(
        self,
        backend: KeyringBackendProtocol | None = None,
        vault: AccountVault | None = None,
        service: str = DEFAULT_SERVICE,
        username: str = DEFAULT_USERNAME,
        storage_path: Path | str | None = None,
    ) -> None:
        self.backend = backend or get_default_keyring_backend()
        self.vault = vault or AccountVault()
        self.service = service
        self.username = username
        self.storage_path = Path(storage_path or ANTIGRAVITY_STORAGE_FILE).expanduser().resolve()
        self._switch_listeners: list[Callable[[str, str], None]] = []

    def register_switch_listener(self, listener: Callable[[str, str], None]) -> None:
        """Register callback for (account_email, reason) switch events."""
        self._switch_listeners.append(listener)

    def _notify_switch(self, account_email: str, reason: str = "manual") -> None:
        for listener in self._switch_listeners:
            try:
                listener(account_email, reason)
            except Exception as exc:
                logger.warning("Error in switch listener callback: %s", exc)

    def get_active_credential(self) -> KeyringCredential:
        """
        Retrieve active credential from Secret Service keyring.
        Conforms strictly to KeyringService.get_active_credential().
        """
        raw = self.backend.lookup(service=self.service, username=self.username)
        if not raw:
            raise KeyringNotFoundError(
                f"No credential found in keyring for service='{self.service}', username='{self.username}'"
            )
        return KeyringCredential.from_antigravity_json(raw)

    def set_active_credential(self, cred: KeyringCredential) -> None:
        """
        Store credential in Secret Service keyring.
        Conforms strictly to KeyringService.set_active_credential(cred).
        """
        if not isinstance(cred, KeyringCredential):
            raise TypeError(f"Expected KeyringCredential, got {type(cred)}")

        payload_json = cred.to_antigravity_json()
        label = f"Password for '{self.username}' on '{self.service}'"
        self.backend.store(
            secret=payload_json,
            service=self.service,
            username=self.username,
            label=label,
        )

    def list_accounts(self) -> list[str]:
        """
        List all managed account emails from accounts.json.
        Conforms strictly to KeyringService.list_accounts().
        """
        accounts = self.vault.list_accounts()
        if not accounts:
            # Attempt auto-ingest of active keyring account if empty
            ingested = self.auto_ingest_current_keyring_if_empty()
            if ingested:
                accounts = [ingested]
        return accounts

    def switch_account(self, account_email: str, reason: str = "manual") -> bool:
        """
        Atomically switch active credential in Secret Service to the specified account.
        Conforms strictly to KeyringService.switch_account(account_email).
        
        Execution steps:
        1. Validate account exists in vault.
        2. Sync current keyring tokens back into vault for the previously active account
           (preserves fresh refreshed tokens).
        3. Store target account credential into Secret Service keyring.
        4. Update vault state (active_account and last_used_at).
        5. Emit switch notification events.
        """
        target_record = self.vault.get_account(account_email)
        if not target_record:
            raise AccountNotFoundError(
                f"Cannot switch to '{account_email}': account not registered in vault"
            )

        # Step 2: Attempt to sync currently active keyring tokens to vault
        active_email = self.vault.get_active_account()
        if active_email and active_email != account_email:
            try:
                current_cred = self.get_active_credential()
                # Update vault's copy of current_cred so any newly refreshed tokens are preserved
                self.vault.add_or_update_account(
                    email=active_email,
                    credential=current_cred,
                    label=self.vault.get_account(active_email).label if self.vault.get_account(active_email) else "",
                )
            except Exception as exc:
                logger.debug("Could not read current keyring credential before switch: %s", exc)

        # Step 3: Write new target credential to Secret Service
        self.set_active_credential(target_record.credential)

        # Step 4: Update vault active account
        self.vault.set_active_account(account_email)

        # Step 5: Notify listeners
        self._notify_switch(account_email, reason)

        return True

    def auto_ingest_current_keyring_if_empty(self) -> str | None:
        """
        If the vault is empty, attempt to read the active credential from Secret Service,
        determine its email, and initialize accounts.json automatically.
        """
        try:
            cred = self.get_active_credential()
        except KeyringNotFoundError:
            return None
        except Exception:
            return None

        # Discover email
        email = self._discover_email_for_credential(cred) or "primary@antigravity"
        self.vault.add_or_update_account(
            email=email,
            credential=cred,
            label="Primary Account (Imported)",
        )
        self.vault.set_active_account(email)
        return email

    def _discover_email_for_credential(self, cred: KeyringCredential) -> str | None:
        """Try to discover user email from app_storage.json or id_token."""
        # 1. Try id_token JWT claim
        jwt_email = cred.extract_email_from_id_token()
        if jwt_email:
            return jwt_email

        # 2. Try app_storage.json
        if self.storage_path.exists():
            try:
                with open(self.storage_path, "r", encoding="utf-8") as f:
                    storage = json.load(f)
                    email = storage.get("jetski.onboarding.lastLoginUsername")
                    if email and isinstance(email, str) and "@" in email:
                        return email
            except Exception:
                pass

        return None
