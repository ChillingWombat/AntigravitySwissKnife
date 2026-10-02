"""
Device Fingerprint Profile Dataclass and Store (Feature F10).
=============================================================
Manages per-account hardware/device identity profiles:
- machineid (UUIDv4)
- .updaterId (UUIDv4)
- installation_id (UUIDv4)
- installation_uuid (UUIDv4)
Stored in ~/.config/antigravity-swiss/device_profiles.json with mode 0600.
"""

from __future__ import annotations

from dataclasses import asdict, dataclass, field
import json
import logging
import os
from pathlib import Path
import tempfile
from typing import Any, Dict, List, Optional
import uuid

from antigravity_swiss.core.constants import DEFAULT_SWISS_CONFIG_DIR

logger = logging.getLogger("antigravity_swiss.fingerprint.profile_store")


@dataclass
class DeviceProfile:
    """Hardware and environment identity parameters bound to a specific account."""
    machine_id: str         # UUIDv4 or 64-char hex
    updater_id: str         # UUIDv4
    installation_id: str    # UUIDv4
    installation_uuid: str  # UUIDv4

    @classmethod
    def generate_random(cls) -> DeviceProfile:
        """Generates a fresh, realistic device profile using standard UUIDv4 values."""
        return cls(
            machine_id=str(uuid.uuid4()),
            updater_id=str(uuid.uuid4()),
            installation_id=str(uuid.uuid4()),
            installation_uuid=str(uuid.uuid4()),
        )

    def to_dict(self) -> Dict[str, str]:
        return {
            "machine_id": self.machine_id,
            "updater_id": self.updater_id,
            "installation_id": self.installation_id,
            "installation_uuid": self.installation_uuid,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> DeviceProfile:
        return cls(
            machine_id=str(data.get("machine_id") or uuid.uuid4()),
            updater_id=str(data.get("updater_id") or uuid.uuid4()),
            installation_id=str(data.get("installation_id") or uuid.uuid4()),
            installation_uuid=str(data.get("installation_uuid") or uuid.uuid4()),
        )


class DeviceProfileStore:
    """
    Manages persistent storage of device profiles mapped to account emails.
    Atomic writes ensure zero corruption during sudden shutdowns.
    """

    def __init__(self, storage_path: Optional[Path | str] = None) -> None:
        if storage_path:
            self.storage_path = Path(storage_path).expanduser().resolve()
        else:
            self.storage_path = (Path(DEFAULT_SWISS_CONFIG_DIR).expanduser().resolve() / "device_profiles.json")
        self._profiles: Dict[str, DeviceProfile] = {}
        self.load()

    def load(self) -> None:
        """Loads profiles from JSON disk store."""
        if not self.storage_path.exists():
            self._profiles = {}
            return

        try:
            with open(self.storage_path, "r", encoding="utf-8") as f:
                data = json.load(f)
            self._profiles = {
                email: DeviceProfile.from_dict(prof_data)
                for email, prof_data in data.items()
                if isinstance(prof_data, dict)
            }
        except Exception as exc:
            logger.error("Failed to load device profiles from %s: %s", self.storage_path, exc)
            self._profiles = {}

    def save(self) -> None:
        """Atomically saves profiles to JSON disk store with 0600 permissions."""
        self.storage_path.parent.mkdir(parents=True, exist_ok=True)
        data = {email: prof.to_dict() for email, prof in self._profiles.items()}

        tmp_file = self.storage_path.with_suffix(".tmp")
        try:
            with open(tmp_file, "w", encoding="utf-8") as f:
                json.dump(data, f, indent=2)
                f.flush()
                os.fsync(f.fileno())

            # Enforce 0600 (read/write only by owner)
            os.chmod(tmp_file, 0o600)
            os.replace(tmp_file, self.storage_path)
        except Exception as exc:
            logger.error("Failed to save device profiles to %s: %s", self.storage_path, exc)
            if tmp_file.exists():
                tmp_file.unlink(missing_ok=True)
            raise

    def get_profile(self, account_email: str) -> Optional[DeviceProfile]:
        """Returns the device profile for the account, or None if not registered."""
        return self._profiles.get(account_email)

    def set_profile(self, account_email: str, profile: DeviceProfile) -> None:
        """Sets and persists the device profile for the account."""
        self._profiles[account_email] = profile
        self.save()

    def get_or_create_profile(self, account_email: str) -> DeviceProfile:
        """Retrieves existing profile or generates and saves a fresh profile."""
        if account_email not in self._profiles:
            self._profiles[account_email] = DeviceProfile.generate_random()
            self.save()
        return self._profiles[account_email]

    def list_accounts(self) -> List[str]:
        """Returns list of account emails with registered device profiles."""
        return list(self._profiles.keys())

    def delete_profile(self, account_email: str) -> bool:
        """Deletes profile for given account."""
        if account_email in self._profiles:
            del self._profiles[account_email]
            self.save()
            return True
        return False
