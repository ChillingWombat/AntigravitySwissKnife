"""
Device Fingerprint Profile Store (Feature F10).
===============================================
Manages persistent storage of device profiles in:
  ~/.config/antigravity-swiss/profiles.json (mode 0600)
Protected by fcntl.flock concurrency control, atomic tempfile replacement,
and automated quarantine of corrupted JSON stores.
"""

from __future__ import annotations

import contextlib
import datetime
import fcntl
import json
import logging
import os
from pathlib import Path
import shutil
import tempfile
import threading
import time
from typing import Any, Dict, Generator, List, Optional

from antigravity_swiss.core.constants import DEFAULT_SWISS_CONFIG_DIR
from antigravity_swiss.core.errors import FingerprintError
from antigravity_swiss.fingerprint.models import DeviceProfile

logger = logging.getLogger("antigravity_swiss.fingerprint.profile_store")

DEFAULT_PROFILES_FILE = Path(DEFAULT_SWISS_CONFIG_DIR).expanduser() / "profiles.json"


class ProfileStoreCorruptedError(FingerprintError):
    """Raised when profiles.json is malformed or corrupted."""
    pass


class ProfileStore:
    """
    Manages persistent storage of device profiles mapped to account emails.
    Enforces mode 0600 on the file, mode 0700 on the directory,
    and protects all read/write transactions via fcntl.flock and threading.RLock.
    """

    _lock_registry: Dict[Path, tuple[threading.RLock, Dict[str, Any]]] = {}
    _registry_lock = threading.Lock()

    @classmethod
    def _get_lock_state(cls, lock_path: Path) -> tuple[threading.RLock, Dict[str, Any]]:
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
        storage_path: Optional[Path | str] = None,
        lock_path: Optional[Path | str] = None,
    ) -> None:
        if storage_path:
            self.storage_path = Path(storage_path).expanduser().resolve()
        else:
            self.storage_path = DEFAULT_PROFILES_FILE.resolve()

        self.config_dir = self.storage_path.parent
        if lock_path:
            self.lock_path = Path(lock_path).expanduser().resolve()
        else:
            self.lock_path = self.config_dir / f"{self.storage_path.stem}.lock"

        self._ensure_dir()
        self._check_legacy_migration()

    def _ensure_dir(self) -> None:
        for d in (self.config_dir, self.lock_path.parent):
            if not d.exists():
                d.mkdir(parents=True, mode=0o700, exist_ok=True)
            else:
                try:
                    current_mode = d.stat().st_mode & 0o777
                    if current_mode != 0o700:
                        d.chmod(0o700)
                except OSError:
                    pass

    def _check_legacy_migration(self) -> None:
        legacy_path = self.config_dir / "device_profiles.json"
        if not self.storage_path.exists() and legacy_path.exists():
            try:
                shutil.copy2(legacy_path, self.storage_path)
                logger.info("Migrated legacy device_profiles.json to %s", self.storage_path)
            except Exception as exc:
                logger.warning("Could not auto-migrate legacy profiles file: %s", exc)

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
            if self.storage_path.exists():
                ts = int(time.time() * 1000)
                quarantine_path = self.storage_path.with_name(
                    f"{self.storage_path.name}.corrupted.{ts}"
                )
                shutil.copy2(str(self.storage_path), str(quarantine_path))
                try:
                    os.chmod(str(quarantine_path), 0o600)
                except OSError:
                    pass
                logger.warning("Quarantined corrupted profile store to %s", quarantine_path)
        except Exception as exc:
            logger.error("Failed to quarantine corrupted profiles file: %s", exc)

    def _load_unlocked(self) -> Dict[str, Any]:
        if not self.storage_path.exists():
            return {"version": 1, "active_account": None, "profiles": {}}

        try:
            with open(self.storage_path, "r", encoding="utf-8") as f:
                content = f.read().strip()
                if not content:
                    return {"version": 1, "active_account": None, "profiles": {}}
                data = json.loads(content)
        except (json.JSONDecodeError, UnicodeDecodeError) as exc:
            self._quarantine_corrupted()
            raise ProfileStoreCorruptedError(f"Malformed JSON in {self.storage_path}: {exc}") from exc

        if not isinstance(data, dict):
            self._quarantine_corrupted()
            raise ProfileStoreCorruptedError(f"Root object must be dict, got {type(data).__name__}")

        # Support both flat schema {email: prof} and structured schema {"profiles": {email: prof}}
        if "profiles" not in data:
            data = {"version": 1, "active_account": None, "profiles": data}

        try:
            st = self.storage_path.stat()
            if (st.st_mode & 0o777) != 0o600:
                self.storage_path.chmod(0o600)
        except OSError:
            pass

        return data

    def _save_unlocked(self, data: Dict[str, Any]) -> None:
        self._ensure_dir()
        fd, tmp_path_str = tempfile.mkstemp(
            dir=self.config_dir,
            prefix=f".{self.storage_path.name}.tmp.",
            text=True,
        )
        tmp_path = Path(tmp_path_str)
        try:
            os.fchmod(fd, 0o600)
            with os.fdopen(fd, "w", encoding="utf-8") as f:
                json.dump(data, f, indent=2, sort_keys=True)
                f.flush()
                os.fsync(f.fileno())
            os.replace(str(tmp_path), str(self.storage_path))
        except Exception:
            if tmp_path.exists():
                try:
                    tmp_path.unlink()
                except OSError:
                    pass
            raise

    @contextlib.contextmanager
    def transaction(self) -> Generator[Dict[str, Any], None, None]:
        lock_fd = self._lock()
        try:
            try:
                data = self._load_unlocked()
            except ProfileStoreCorruptedError:
                data = {"version": 1, "active_account": None, "profiles": {}}
            yield data
            self._save_unlocked(data)
        finally:
            self._unlock(lock_fd)

    def load(self) -> None:
        """Loads and verifies profiles store (backward compatibility)."""
        with self.lock_context():
            self._load_unlocked()

    def save(self) -> None:
        """Saves current state (backward compatibility)."""
        with self.lock_context():
            data = self._load_unlocked()
            self._save_unlocked(data)

    def get_profile(self, account_email: str) -> Optional[DeviceProfile]:
        """Retrieves profile associated with given account email."""
        with self.lock_context():
            data = self._load_unlocked()
            profiles = data.get("profiles", {})
            prof_data = profiles.get(account_email)
            if isinstance(prof_data, dict):
                return DeviceProfile.from_dict(prof_data, account_email=account_email)
            return None

    def set_profile(self, account_email: str, profile: DeviceProfile) -> None:
        """Persists profile associated with given account email."""
        profile.account_email = account_email
        with self.transaction() as data:
            profiles = data.setdefault("profiles", {})
            profiles[account_email] = profile.to_dict()

    def get_or_create_profile(self, account_email: str) -> DeviceProfile:
        """Retrieves existing profile or creates, saves, and returns a new random profile."""
        with self.transaction() as data:
            profiles = data.setdefault("profiles", {})
            if account_email in profiles and isinstance(profiles[account_email], dict):
                return DeviceProfile.from_dict(profiles[account_email], account_email=account_email)
            fresh = DeviceProfile.generate_random(account_email=account_email)
            profiles[account_email] = fresh.to_dict()
            return fresh

    def list_accounts(self) -> List[str]:
        """Lists all registered account emails."""
        with self.lock_context():
            data = self._load_unlocked()
            return sorted(list(data.get("profiles", {}).keys()))

    def delete_profile(self, account_email: str) -> bool:
        """Deletes profile associated with given account email."""
        with self.transaction() as data:
            profiles = data.get("profiles", {})
            if account_email in profiles:
                del profiles[account_email]
                if data.get("active_account") == account_email:
                    data["active_account"] = None
                return True
            return False

    def get_active_account(self) -> Optional[str]:
        """Returns currently active account email in profile store."""
        with self.lock_context():
            return self._load_unlocked().get("active_account")

    def set_active_account(self, account_email: Optional[str]) -> None:
        """Sets active account email and updates last_used_at timestamp."""
        with self.transaction() as data:
            data["active_account"] = account_email
            profiles = data.get("profiles", {})
            now_iso = datetime.datetime.now(datetime.timezone.utc).isoformat()
            for email, p_data in profiles.items():
                if isinstance(p_data, dict):
                    if email == account_email:
                        p_data["last_used_at"] = now_iso
                        p_data["is_active"] = True
                    else:
                        p_data["is_active"] = False


# Backwards compatibility alias
DeviceProfileStore = ProfileStore
