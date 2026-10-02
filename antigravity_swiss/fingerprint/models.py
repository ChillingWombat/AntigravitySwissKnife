"""
Device Fingerprint Profile Models (Feature F10).
================================================
Defines DeviceProfile dataclass representing per-account hardware identity:
- account_email: Associated account email
- machine_id: 36B UUIDv4 string
- updater_id: 36B UUIDv4 string
- installation_id: 36B UUIDv4 string
- installation_uuid: 36B UUIDv4 string
- created_at: ISO 8601 UTC timestamp
- last_used_at: ISO 8601 UTC timestamp or None
- is_active: Whether profile is currently applied to disk
"""

from __future__ import annotations

from dataclasses import dataclass, field
import datetime
from typing import Any, Dict, Optional
import uuid


@dataclass
class DeviceProfile:
    """Hardware and environment identity parameters bound to a specific account."""
    account_email: str = ""
    machine_id: str = ""
    updater_id: str = ""
    installation_id: str = ""
    installation_uuid: str = ""
    created_at: str = field(
        default_factory=lambda: datetime.datetime.now(datetime.timezone.utc).isoformat()
    )
    last_used_at: Optional[str] = None
    is_active: bool = False

    @classmethod
    def generate_random(cls, account_email: str = "") -> DeviceProfile:
        """Generates a fresh, realistic device profile using standard UUIDv4 values."""
        now_iso = datetime.datetime.now(datetime.timezone.utc).isoformat()
        return cls(
            account_email=account_email,
            machine_id=str(uuid.uuid4()),
            updater_id=str(uuid.uuid4()),
            installation_id=str(uuid.uuid4()),
            installation_uuid=str(uuid.uuid4()),
            created_at=now_iso,
            last_used_at=None,
            is_active=False,
        )

    def to_dict(self) -> Dict[str, Any]:
        """Serializes to dictionary representation."""
        return {
            "account_email": self.account_email,
            "machine_id": self.machine_id,
            "updater_id": self.updater_id,
            "installation_id": self.installation_id,
            "installation_uuid": self.installation_uuid,
            "created_at": self.created_at,
            "last_used_at": self.last_used_at,
            "is_active": self.is_active,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any], account_email: str = "") -> DeviceProfile:
        """Deserializes from dictionary, accepting snake_case and legacy camelCase keys."""
        email = str(data.get("account_email") or data.get("email") or account_email)
        machine_id = str(data.get("machine_id") or data.get("machineid") or uuid.uuid4())
        updater_id = str(data.get("updater_id") or data.get("updaterId") or uuid.uuid4())
        installation_id = str(data.get("installation_id") or uuid.uuid4())
        installation_uuid = str(data.get("installation_uuid") or uuid.uuid4())
        created_at = str(
            data.get("created_at")
            or datetime.datetime.now(datetime.timezone.utc).isoformat()
        )
        last_used_at = data.get("last_used_at")
        is_active = bool(data.get("is_active", False))

        return cls(
            account_email=email,
            machine_id=machine_id,
            updater_id=updater_id,
            installation_id=installation_id,
            installation_uuid=installation_uuid,
            created_at=created_at,
            last_used_at=last_used_at,
            is_active=is_active,
        )
