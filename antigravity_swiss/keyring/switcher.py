"""
Multi-Account Vault & Atomic Keyring Switcher.
==============================================
Manages credentials in accounts.json (mode 0600) with fcntl.flock concurrency
protection and performs atomic credential rotation in Linux Secret Service.
"""

from __future__ import annotations

import base64
import contextlib
import datetime
import fcntl
import json
import logging
import os
import shutil
import tempfile
import threading
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable, Generator, NamedTuple

from antigravity_swiss.core.constants import (
    DEFAULT_ANTIGRAVITY_CONFIG_DIR,
    ENV_ACCOUNTS_FILE,
    ENV_CONFIG_DIR,
    KEYRING_SERVICE_NAME,
    KEYRING_USERNAME,
)
from antigravity_swiss.core.errors import (
    AccountNotFoundError,
    AccountVaultCorruptedError,
    InvalidCredentialError,
    KeyringBackendUnavailableError,
    KeyringError,
    KeyringNotFoundError,
)
from antigravity_swiss.core.crypto import decrypt_credential, encrypt_credential
from antigravity_swiss.keyring.dbus_keyring import DBusKeyring, KeyringBackendProtocol
from antigravity_swiss.keyring.secret_tool import (
    DEFAULT_LABEL,
    DEFAULT_SERVICE,
    DEFAULT_USERNAME,
    SecretToolBackend,
)

logger = logging.getLogger("antigravity_swiss.keyring")

DEFAULT_CONFIG_DIR = Path.home() / ".config" / "antigravity-swiss"
DEFAULT_ACCOUNTS_FILE = DEFAULT_CONFIG_DIR / "accounts.json"
DEFAULT_LOCK_FILE = DEFAULT_CONFIG_DIR / "accounts.lock"
ANTIGRAVITY_STORAGE_FILE = Path.home() / ".config" / "Antigravity" / "app_storage.json"


class KeyringCredential(NamedTuple):
    """
    Representation of an Antigravity OAuth credential matching zalando/go-keyring schema.
    Conforms strictly to PROJECT.md § Interface Contracts.
    """
    access_token: str
    refresh_token: str
    token_type: str = "Bearer"
    expiry: str = ""
    auth_method: str = "consumer"
    id_token: str = ""

    def to_antigravity_json(self) -> str:
        """Serializes to the exact JSON structure expected by Antigravity language_server."""
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

    def to_dict(self, encrypt: bool = True) -> dict[str, Any]:
        return {
            "access_token": encrypt_credential(self.access_token) if encrypt else self.access_token,
            "refresh_token": encrypt_credential(self.refresh_token) if encrypt else self.refresh_token,
            "token_type": self.token_type,
            "expiry": self.expiry,
            "auth_method": self.auth_method,
            "id_token": self.id_token,
        }

    @classmethod
    def from_antigravity_json(cls, raw: str) -> KeyringCredential:
        if not raw or not raw.strip():
            raise InvalidCredentialError("Empty secret payload from keyring")
        try:
            data = json.loads(raw)
        except json.JSONDecodeError as exc:
            raise InvalidCredentialError(f"Secret payload is not valid JSON: {exc}") from exc

        if not isinstance(data, dict):
            raise InvalidCredentialError(f"Expected JSON object, got {type(data).__name__}")

        token_obj = data.get("token", {}) if isinstance(data.get("token"), dict) else {}
        access_token = decrypt_credential(token_obj.get("access_token") or data.get("access_token", ""))
        refresh_token = decrypt_credential(token_obj.get("refresh_token") or data.get("refresh_token", ""))
        token_type = token_obj.get("token_type") or data.get("token_type", "Bearer")
        expiry = str(token_obj.get("expiry") or data.get("expiry", ""))
        auth_method = str(data.get("auth_method", "consumer"))
        id_token = str(data.get("id_token") or token_obj.get("id_token", ""))

        if not access_token and not refresh_token:
            raise InvalidCredentialError("Credential contains neither access_token nor refresh_token")

        return cls(access_token, refresh_token, token_type, expiry, auth_method, id_token)

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> KeyringCredential:
        return cls(
            access_token=decrypt_credential(str(data.get("access_token", ""))),
            refresh_token=decrypt_credential(str(data.get("refresh_token", ""))),
            token_type=str(data.get("token_type", "Bearer")),
            expiry=str(data.get("expiry", "")),
            auth_method=str(data.get("auth_method", "consumer")),
            id_token=str(data.get("id_token", "")),
        )

    def extract_email_from_id_token(self) -> str | None:
        """Extracts email claim from embedded JWT id_token without external libraries."""
        if not self.id_token:
            return None
        import re
        if self.id_token.count(".") == 2:
            try:
                part = self.id_token.split(".")[1]
                padded = part + "=" * ((4 - len(part) % 4) % 4)
                decoded = json.loads(base64.urlsafe_b64decode(padded).decode("utf-8"))
                email = decoded.get("email")
                if email and "@" in email:
                    return email
            except Exception:
                pass

        m = re.search(r"(?<=\.)[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\.[a-zA-Z0-9]+(?=\.|$)", self.id_token)
        if m:
            return m.group(0)
        m2 = re.search(r"[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\.[a-zA-Z0-9]+", self.id_token)
        return m2.group(0) if m2 else None




@dataclass
class AccountRecord:
    """Record storing an account in accounts.json."""
    email: str
    label: str
    credential: KeyringCredential
    added_at: str
    last_used_at: str | None = None
    totp_secret: str = ""
    is_healthy: bool = True
    plan_tier: str = "Free"
    status: str = "STANDBY"
    priority: str = "High"
    notes: str = ""
    password: str = ""

    def to_dict(self, encrypt: bool = True) -> dict[str, Any]:
        return {
            "email": self.email,
            "label": self.label,
            "credential": self.credential.to_dict(encrypt=encrypt),
            "added_at": self.added_at,
            "last_used_at": self.last_used_at,
            "totp_secret": encrypt_credential(self.totp_secret) if encrypt else self.totp_secret,
            "password": encrypt_credential(self.password) if encrypt else self.password,
            "is_healthy": self.is_healthy,
            "plan_tier": self.plan_tier,
            "status": self.status,
            "priority": self.priority or "High",
            "notes": self.notes or "",
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> AccountRecord:
        raw_status = data.get("status")
        is_healthy = bool(data.get("is_healthy", True))
        if raw_status:
            status = str(raw_status).upper()
        else:
            status = "STANDBY" if is_healthy else "ERROR"
        priority = str(data.get("priority", "High") or "High").strip().capitalize()
        if priority not in ("High", "Mid", "Low"):
            priority = "High"
        notes = str(data.get("notes", "") or "")
        totp_secret = decrypt_credential(str(data.get("totp_secret", "") or ""))
        password = decrypt_credential(str(data.get("password", "") or ""))
        return cls(
            email=str(data["email"]),
            label=str(data.get("label", "")),
            credential=KeyringCredential.from_dict(data.get("credential", {})),
            added_at=str(data.get("added_at", "")),
            last_used_at=data.get("last_used_at"),
            totp_secret=totp_secret,
            is_healthy=is_healthy and (status not in ("ERROR", "BANNED")),
            plan_tier=str(data.get("plan_tier", "Free")),
            status=status,
            priority=priority,
            notes=notes,
            password=password,
        )


class AccountVault:
    """
    Manages accounts in accounts.json.
    Ensures mode 0600 on the file, mode 0700 on the parent directory,
    and protects all read/write transactions via fcntl.flock concurrency locking.
    """

    _lock_registry: dict[Path, tuple[threading.RLock, dict[str, Any]]] = {}
    _registry_lock = threading.Lock()

    @classmethod
    def _get_lock_state(cls, lock_path: Path) -> tuple[threading.RLock, dict[str, Any]]:
        norm_path = lock_path.resolve()
        with cls._registry_lock:
            if norm_path not in cls._lock_registry:
                cls._lock_registry[norm_path] = (
                    threading.RLock(),
                    {"fd": None, "owner": None, "depth": 0},
                )
            return cls._lock_registry[norm_path]

    def __init__(
        self,
        config_path: Path | str | None = None,
        lock_path: Path | str | None = None,
    ) -> None:
        if config_path is not None:
            self.config_path = Path(config_path).expanduser().resolve()
        elif os.environ.get(ENV_ACCOUNTS_FILE):
            self.config_path = Path(os.environ[ENV_ACCOUNTS_FILE]).expanduser().resolve()
        elif os.environ.get(ENV_CONFIG_DIR):
            self.config_path = Path(os.environ[ENV_CONFIG_DIR]).expanduser().resolve() / "accounts.json"
        else:
            self.config_path = DEFAULT_ACCOUNTS_FILE.expanduser().resolve()

        self.config_dir = self.config_path.parent
        if lock_path is not None:
            self.lock_path = Path(lock_path).expanduser().resolve()
        else:
            self.lock_path = self.config_dir / f"{self.config_path.stem}.lock"

    def _ensure_dir(self) -> None:
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

    def _lock(self) -> int:
        self._ensure_dir()
        thread_lock, state = self._get_lock_state(self.lock_path)
        thread_lock.acquire()
        current_thread = threading.get_ident()
        if state["depth"] > 0 and state["owner"] == current_thread:
            state["depth"] += 1
            return state["fd"]

        lock_fd = os.open(str(self.lock_path), os.O_CREAT | os.O_RDWR, 0o600)
        try:
            os.fchmod(lock_fd, 0o600)
        except OSError:
            pass
        fcntl.flock(lock_fd, fcntl.LOCK_EX)
        state["fd"] = lock_fd
        state["owner"] = current_thread
        state["depth"] = 1
        return lock_fd

    def _unlock(self, lock_fd: int) -> None:
        thread_lock, state = self._get_lock_state(self.lock_path)
        current_thread = threading.get_ident()
        if state["owner"] == current_thread:
            state["depth"] -= 1
            if state["depth"] == 0:
                fd = state["fd"]
                state["fd"] = None
                state["owner"] = None
                try:
                    fcntl.flock(fd, fcntl.LOCK_UN)
                    os.close(fd)
                except OSError:
                    pass
        thread_lock.release()

    @contextlib.contextmanager
    def lock_context(self) -> Generator[None, None, None]:
        lock_fd = self._lock()
        try:
            yield
        finally:
            self._unlock(lock_fd)

    def _quarantine_corrupted(self) -> None:
        try:
            if self.config_path.exists():
                ts = int(time.time())
                quarantine_path = self.config_path.with_name(f"{self.config_path.name}.corrupted.{ts}")
                if quarantine_path.exists():
                    quarantine_path = self.config_path.with_name(
                        f"{self.config_path.name}.corrupted.{int(time.time() * 1000)}"
                    )
                shutil.copy2(str(self.config_path), str(quarantine_path))
                try:
                    os.chmod(str(quarantine_path), 0o600)
                except OSError:
                    pass
                logger.warning("Quarantined corrupted vault file to %s", quarantine_path)
        except Exception as exc:
            logger.error("Failed to quarantine corrupted file %s: %s", self.config_path, exc)

    def _load_unlocked(self) -> dict[str, Any]:
        if not self.config_path.exists():
            return {"version": 1, "active_account": None, "accounts": {}}
        try:
            with open(self.config_path, "r", encoding="utf-8") as f:
                content = f.read().strip()
                if not content:
                    return {"version": 1, "active_account": None, "accounts": {}}
                data = json.loads(content)
        except (json.JSONDecodeError, UnicodeDecodeError) as exc:
            self._quarantine_corrupted()
            raise AccountVaultCorruptedError(f"Failed to parse {self.config_path}: {exc}") from exc

        if not isinstance(data, dict):
            self._quarantine_corrupted()
            raise AccountVaultCorruptedError(
                f"Root JSON object in {self.config_path} must be a dict, got {type(data).__name__}"
            )

        st = self.config_path.stat()
        if (st.st_mode & 0o777) != 0o600:
            try:
                self.config_path.chmod(0o600)
            except OSError:
                pass
        return data

    def _save_unlocked(self, data: dict[str, Any]) -> None:
        self._ensure_dir()
        fd, tmp_path_str = tempfile.mkstemp(
            dir=self.config_dir,
            prefix=f".{self.config_path.name}.tmp.",
            text=True,
        )
        tmp_path = Path(tmp_path_str)
        try:
            os.fchmod(fd, 0o600)
            with os.fdopen(fd, "w", encoding="utf-8") as f:
                json.dump(data, f, indent=2, sort_keys=True)
                f.flush()
                os.fsync(f.fileno())
            os.replace(str(tmp_path), str(self.config_path))
        except Exception:
            if tmp_path.exists():
                try:
                    tmp_path.unlink()
                except OSError:
                    pass
            raise

    def load(self) -> dict[str, Any]:
        lock_fd = self._lock()
        try:
            return self._load_unlocked()
        finally:
            self._unlock(lock_fd)

    def save(self, data: dict[str, Any]) -> None:
        lock_fd = self._lock()
        try:
            self._save_unlocked(data)
        finally:
            self._unlock(lock_fd)

    @contextlib.contextmanager
    def transaction(self) -> Generator[dict[str, Any], None, None]:
        lock_fd = self._lock()
        try:
            try:
                data = self._load_unlocked()
            except AccountVaultCorruptedError:
                # Malformed file was quarantined in _load_unlocked; reinitialize clean structure
                data = {"version": 1, "active_account": None, "accounts": {}}
            yield data
            self._save_unlocked(data)
        finally:
            self._unlock(lock_fd)

    def list_accounts(self) -> list[str]:
        return sorted(list(self.load().get("accounts", {}).keys()))

    def list_account_records(self) -> list[AccountRecord]:
        data = self.load()
        accs = data.get("accounts", {})
        if not isinstance(accs, dict):
            raise AccountVaultCorruptedError(f"accounts field in {self.config_path} must be a dict")
        records: list[AccountRecord] = []
        for email, v in accs.items():
            if not isinstance(v, dict):
                raise AccountVaultCorruptedError(f"Account record for '{email}' is not a dict: {v!r}")
            try:
                records.append(AccountRecord.from_dict(v))
            except Exception as exc:
                raise AccountVaultCorruptedError(f"Failed to parse account record for '{email}': {exc}") from exc
        return records

    def get_account(self, email: str) -> AccountRecord | None:
        acc = self.load().get("accounts", {}).get(email)
        return AccountRecord.from_dict(acc) if isinstance(acc, dict) else None

    def get_active_account(self) -> str | None:
        return self.load().get("active_account")

    def set_active_account(self, email: str | None) -> None:
        with self.transaction() as data:
            accounts = data.setdefault("accounts", {})
            if email is not None and email not in accounts:
                raise AccountNotFoundError(email)
            data["active_account"] = email
            if email:
                accounts[email]["last_used_at"] = (
                    datetime.datetime.now(datetime.timezone.utc).isoformat()
                )

    def add_or_update_account(
        self,
        email: str,
        credential: KeyringCredential,
        label: str = "",
        totp_secret: str = "",
        is_healthy: bool = True,
        plan_tier: str | None = None,
        priority: str = "High",
        notes: str = "",
        password: str = "",
    ) -> AccountRecord:
        with self.transaction() as data:
            accounts = data.setdefault("accounts", {})
            now_iso = datetime.datetime.now(datetime.timezone.utc).isoformat()

            if email in accounts and isinstance(accounts[email], dict):
                record = AccountRecord.from_dict(accounts[email])
                record.credential = credential
                if label:
                    record.label = label
                if totp_secret:
                    record.totp_secret = totp_secret
                record.is_healthy = is_healthy
                if plan_tier:
                    record.plan_tier = plan_tier
                if priority:
                    p = priority.strip().capitalize()
                    record.priority = p if p in ("High", "Mid", "Low") else "High"
                if notes is not None:
                    record.notes = notes
                if password:
                    record.password = password
            else:
                p = (priority or "High").strip().capitalize()
                record = AccountRecord(
                    email=email,
                    label=label or email,
                    credential=credential,
                    added_at=now_iso,
                    totp_secret=totp_secret,
                    is_healthy=is_healthy,
                    plan_tier=plan_tier or "Free",
                    priority=p if p in ("High", "Mid", "Low") else "High",
                    notes=notes or "",
                    password=password or "",
                )

            accounts[email] = record.to_dict()
            if data.get("active_account") is None:
                data["active_account"] = email
            return record

    def set_totp_secret(self, email: str, totp_secret: str) -> bool:
        with self.transaction() as data:
            accounts = data.get("accounts", {})
            if not isinstance(accounts, dict) or email not in accounts:
                return False
            accounts[email]["totp_secret"] = totp_secret
            return True

    def remove_account(self, email: str) -> bool:
        with self.transaction() as data:
            accounts = data.get("accounts", {})
            if not isinstance(accounts, dict) or email not in accounts:
                return False
            del accounts[email]
            if data.get("active_account") == email:
                data["active_account"] = next(iter(accounts.keys())) if accounts else None
            return True

    def update_account_info(
        self,
        email: str,
        label: str | None = None,
        totp_secret: str | None = None,
        refresh_token: str | None = None,
        status: str | None = None,
        set_active: bool = False,
        plan_tier: str | None = None,
        priority: str | None = None,
        notes: str | None = None,
        password: str | None = None,
    ) -> bool:
        with self.transaction() as data:
            accounts = data.get("accounts", {})
            if not isinstance(accounts, dict) or email not in accounts:
                return False
            rec_dict = accounts[email]
            if label is not None:
                rec_dict["label"] = label
            if plan_tier is not None and plan_tier.strip():
                rec_dict["plan_tier"] = plan_tier.strip()
            if priority is not None and priority.strip():
                p = priority.strip().capitalize()
                rec_dict["priority"] = p if p in ("High", "Mid", "Low") else "High"
            if notes is not None:
                rec_dict["notes"] = str(notes)
            if password is not None:
                rec_dict["password"] = encrypt_credential(str(password))
            if status is not None and status.strip():
                norm = status.strip().upper()
                rec_dict["status"] = norm
                rec_dict["is_healthy"] = norm not in ("ERROR", "BANNED")
            if totp_secret is not None:
                rec_dict["totp_secret"] = encrypt_credential(str(totp_secret))
            if refresh_token is not None and refresh_token.strip():
                if "credential" not in rec_dict or not isinstance(rec_dict["credential"], dict):
                    rec_dict["credential"] = {}
                rec_dict["credential"]["refresh_token"] = encrypt_credential(refresh_token.strip())
            if set_active:
                data["active_account"] = email
            return True


# Facade/Alias for AccountVault
class AccountStore:
    """Facade for AccountVault providing list of dicts for RPC and GUI consumption."""

    def __init__(self, accounts_file: Path | str | None = None) -> None:
        self.vault = AccountVault(config_path=accounts_file)

    def list_accounts(self) -> list[dict[str, Any]]:
        records = self.vault.list_account_records()
        active = self.vault.get_active_account()
        return [
            {
                "email": r.email,
                "label": r.label,
                "status": "ACTIVE" if r.email == active else (r.status.upper() if r.status else ("STANDBY" if r.is_healthy else "ERROR")),
                "is_active": r.email == active,
                "is_healthy": r.is_healthy and (r.status not in ("ERROR", "BANNED") if r.status else True),
                "last_used_at": r.last_used_at,
                "has_totp": bool(r.totp_secret),
                "totp_secret": r.totp_secret,
                "plan_tier": getattr(r, "plan_tier", "Free"),
                "priority": getattr(r, "priority", "High") or "High",
                "notes": getattr(r, "notes", "") or "",
                "password": getattr(r, "password", "") or "",
                "refresh_token": r.credential.refresh_token if r.credential else "",
            }
            for r in records
        ]

    def set_totp_secret(self, email: str, totp_secret: str) -> bool:
        return self.vault.set_totp_secret(email, totp_secret)

    def update_account(
        self,
        email: str,
        label: str | None = None,
        totp_secret: str | None = None,
        refresh_token: str | None = None,
        status: str | None = None,
        set_active: bool = False,
        plan_tier: str | None = None,
        priority: str | None = None,
        notes: str | None = None,
        password: str | None = None,
    ) -> bool:
        return self.vault.update_account_info(
            email=email,
            label=label,
            totp_secret=totp_secret,
            refresh_token=refresh_token,
            status=status,
            set_active=set_active,
            plan_tier=plan_tier,
            priority=priority,
            notes=notes,
            password=password,
        )

    update_account_info = update_account

    def remove_account(self, email: str) -> bool:
        return self.vault.remove_account(email)

    def get_account(self, email: str) -> AccountRecord | None:
        return self.vault.get_account(email)

    def add_or_update(
        self,
        email: str,
        credential: KeyringCredential,
        label: str = "",
        plan_tier: str | None = None,
    ) -> AccountRecord:
        return self.vault.add_or_update_account(email, credential, label, plan_tier=plan_tier)


def get_default_keyring_backend() -> KeyringBackendProtocol:
    """Detects available keyring backend, preferring secret-tool then D-Bus."""
    if SecretToolBackend.is_available():
        return SecretToolBackend()
    if DBusKeyring.is_available():
        return DBusKeyring()
    raise KeyringBackendUnavailableError(
        "Neither secret-tool nor a functional D-Bus Secret Service backend is available on this system."
    )


class KeyringService:
    """
    Production implementation of the KeyringService contract from PROJECT.md:
      - get_active_credential() -> KeyringCredential
      - set_active_credential(cred: KeyringCredential) -> None
      - list_accounts() -> list[str]
      - switch_account(account_email: str) -> bool
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
        self._switch_listeners.append(listener)

    def _notify_switch(self, account_email: str, reason: str = "manual") -> None:
        for listener in self._switch_listeners:
            try:
                listener(account_email, reason)
            except Exception as exc:
                logger.warning("Error in switch listener callback: %s", exc)

    def get_active_credential(self) -> KeyringCredential:
        raw = self.backend.lookup(service=self.service, username=self.username)
        if not raw:
            raise KeyringNotFoundError(
                f"No credential found in keyring for service='{self.service}', username='{self.username}'"
            )
        return KeyringCredential.from_antigravity_json(raw)

    def set_active_credential(self, cred: KeyringCredential) -> None:
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

    def write_credential(self, cred: KeyringCredential) -> None:
        """Convenience alias for set_active_credential."""
        self.set_active_credential(cred)

    def read_credential(self) -> KeyringCredential:
        """Convenience alias for get_active_credential."""
        return self.get_active_credential()

    def list_accounts(self) -> list[str]:
        accounts = self.vault.list_accounts()
        if not accounts:
            ingested = self.auto_ingest_current_keyring_if_empty()
            if ingested:
                accounts = [ingested]
        return accounts

    def switch_account(self, account_email: str, reason: str = "manual") -> bool:
        with self.vault.lock_context():
            target_record = self.vault.get_account(account_email)
            if not target_record:
                raise AccountNotFoundError(account_email)
            st = (target_record.status or "").strip().upper()
            if st == "COOLDOWN":
                raise ValueError(f"account {account_email} is in cooldown waiting for quota reset and cannot be switched on")
            if st == "BANNED":
                raise ValueError(f"account {account_email} is banned and cannot be switched on")

            active_email = self.vault.get_active_account()
            if active_email and active_email != account_email:
                try:
                    current_cred = self.get_active_credential()
                    jwt_email = current_cred.extract_email_from_id_token()
                    if jwt_email and jwt_email != active_email:
                        logger.warning(
                            "Keyring token belongs to %s, not %s; skipping back-sync to prevent cross-contamination",
                            jwt_email,
                            active_email,
                        )
                    else:
                        existing_record = self.vault.get_account(active_email)
                        self.vault.add_or_update_account(
                            email=active_email,
                            credential=current_cred,
                            label=existing_record.label if existing_record else "",
                        )
                except Exception as exc:
                    logger.debug("Could not read current keyring credential before switch: %s", exc)

            self.set_active_credential(target_record.credential)
            self.vault.set_active_account(account_email)
            self._notify_switch(account_email, reason)
            return True

    def auto_ingest_current_keyring_if_empty(self) -> str | None:
        try:
            cred = self.get_active_credential()
        except (KeyringNotFoundError, KeyringError):
            return None
        except Exception:
            return None

        email = self._discover_email_for_credential(cred) or "primary@antigravity"
        self.vault.add_or_update_account(
            email=email,
            credential=cred,
            label="Primary Account (Imported)",
        )
        self.vault.set_active_account(email)
        return email

    def _discover_email_for_credential(self, cred: KeyringCredential) -> str | None:
        jwt_email = cred.extract_email_from_id_token()
        if jwt_email:
            return jwt_email

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


class SwitchResult(dict):
    @property
    def email(self) -> str:
        return self.get("email", "")

    @property
    def active_account(self) -> str:
        return self.get("active_account", "")

    @property
    def cascade_id(self) -> str | None:
        return self.get("cascade_id")

    @property
    def success(self) -> bool:
        return self.get("success", False)


class KeyringSwitcher:
    """Convenience wrapper around KeyringService and AccountVault for Swiss Knife controllers."""

    def __init__(
        self,
        config: Any = None,
        vault: AccountVault | None = None,
        keyring_service: KeyringService | None = None,
    ) -> None:
        from antigravity_swiss.core.config import SwissKnifeConfig
        cfg = config or SwissKnifeConfig.load()
        self.config = cfg
        self.vault = vault or AccountVault(config_path=cfg.accounts_file)
        if keyring_service is not None:
            self.service = keyring_service
            if vault is not None:
                self.service.vault = vault
        else:
            self.service = KeyringService(vault=self.vault)

    def switch_to_account(self, email: str, force: bool = False) -> SwitchResult:
        """Switches to account and returns dictionary summary."""
        from antigravity_swiss.session.app_storage import AppStorageManager
        app_storage = AppStorageManager(
            app_storage_path=self.config.antigravity_config_dir / "app_storage.json",
            conv_summaries_db=self.config.antigravity_data_dir / "conversation_summaries.db",
        )
        cascade_id = app_storage.get_active_conversation_id()

        self.service.switch_account(email, reason="controller_switch")
        if cascade_id:
            app_storage.preserve_active_conversation(cascade_id, account_email=email)

        return SwitchResult({
            "success": True,
            "active_account": email,
            "email": email,
            "cascade_id": cascade_id,
        })
