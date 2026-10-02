"""
Hardware and Environment Device Fingerprint Manager (Feature F10 & F11).
========================================================================
Coordinates reading, writing, and swapping virtual hardware profiles:
1. ~/.config/Antigravity/machineid (exact 36B ASCII UUIDv4, zero trailing newline)
2. ~/.config/Antigravity/.updaterId (exact 36B ASCII UUIDv4, zero trailing newline)
3. ~/.gemini/antigravity/installation_id (exact 36B ASCII UUIDv4, zero trailing newline)
4. ~/.gemini/antigravity/antigravity_state.pbtxt (installation_uuid & installation_id)

Synchronizes with KeyringService.switch_account() to ensure atomic rotation.
"""

from __future__ import annotations

import logging
import os
from pathlib import Path
import tempfile
from typing import Any, Callable, Optional

from antigravity_swiss.core.constants import (
    DEFAULT_ANTIGRAVITY_CONFIG_DIR,
    DEFAULT_ANTIGRAVITY_DATA_DIR,
)
from antigravity_swiss.core.errors import FingerprintError
from antigravity_swiss.fingerprint.models import DeviceProfile
from antigravity_swiss.fingerprint.pbtxt_parser import PbtxtParser
from antigravity_swiss.fingerprint.profile_store import ProfileStore

logger = logging.getLogger("antigravity_swiss.fingerprint.manager")


class FingerprintManager:
    """Coordinates reading, storing, and swapping device profiles across accounts."""

    def __init__(
        self,
        config_dir: Optional[Path | str] = None,
        data_dir: Optional[Path | str] = None,
        store: Optional[ProfileStore] = None,
    ) -> None:
        self.config_dir = Path(config_dir or DEFAULT_ANTIGRAVITY_CONFIG_DIR).expanduser().resolve()
        self.data_dir = Path(data_dir or DEFAULT_ANTIGRAVITY_DATA_DIR).expanduser().resolve()
        self.store = store or ProfileStore()

        self.machineid_path = self.config_dir / "machineid"
        self.updater_id_path = self.config_dir / ".updaterId"
        self.installation_id_path = self.data_dir / "installation_id"
        self.config_installation_id_path = self.config_dir / "installation_id"
        self.pbtxt_path = self.data_dir / "antigravity_state.pbtxt"
        self.config_pbtxt_path = self.config_dir / "antigravity_state.pbtxt"

    def _is_host_environment_protected(self) -> bool:
        """Determines if operation should be blocked to protect host IDE during tests."""
        is_testing = bool(
            os.environ.get("PYTEST_CURRENT_TEST") or os.environ.get("ANTIGRAVITY_SWISS_TESTING")
        )
        real_config = Path(DEFAULT_ANTIGRAVITY_CONFIG_DIR).expanduser().resolve()
        real_data = Path(DEFAULT_ANTIGRAVITY_DATA_DIR).expanduser().resolve()
        return is_testing and (self.config_dir == real_config or self.data_dir == real_data)

    def _read_raw_uuid_file(self, path: Path) -> str:
        """Reads raw 36B UUID file, stripping whitespace."""
        if path.exists():
            try:
                return path.read_text(encoding="ascii").strip()
            except Exception as exc:
                logger.warning("Could not read %s: %s", path, exc)
        return ""

    def _write_exact_36b_file(self, path: Path, val: str) -> None:
        """
        Writes exactly 36 bytes ASCII with zero trailing newlines atomically.
        Uses tempfile.mkstemp, os.fchmod 0600, os.fsync, and os.replace.
        """
        clean_val = val.strip().encode("ascii")
        if len(clean_val) != 36:
            raise FingerprintError(f"UUID string must be exactly 36 bytes, got {len(clean_val)} for {path.name}")

        path.parent.mkdir(parents=True, exist_ok=True)
        fd, tmp_path_str = tempfile.mkstemp(
            dir=path.parent,
            prefix=f".{path.name}.tmp.",
            text=False,
        )
        tmp_path = Path(tmp_path_str)
        try:
            os.fchmod(fd, 0o600)
            with os.fdopen(fd, "wb") as f:
                f.write(clean_val)
                f.flush()
                os.fsync(f.fileno())
            os.replace(str(tmp_path), str(path))
        except Exception:
            if tmp_path.exists():
                try:
                    tmp_path.unlink()
                except OSError:
                    pass
            raise

    def get_active_profile(self) -> DeviceProfile:
        """
        Inspects the active filesystem state and reads current device fingerprint IDs.
        If files do not exist, falls back to freshly generated values.
        """
        machine_id = self._read_raw_uuid_file(self.machineid_path)
        updater_id = self._read_raw_uuid_file(self.updater_id_path)

        # Check data_dir then config_dir for installation_id
        installation_id = self._read_raw_uuid_file(self.installation_id_path)
        if not installation_id and self.config_installation_id_path.exists():
            installation_id = self._read_raw_uuid_file(self.config_installation_id_path)

        # Check data_dir then config_dir for pbtxt
        target_pbtxt = self.pbtxt_path if self.pbtxt_path.exists() else self.config_pbtxt_path
        installation_uuid = PbtxtParser.extract_installation_uuid(target_pbtxt) or ""

        fallback = DeviceProfile.generate_random()
        active_email = self.store.get_active_account() or ""

        return DeviceProfile(
            account_email=active_email,
            machine_id=machine_id or fallback.machine_id,
            updater_id=updater_id or fallback.updater_id,
            installation_id=installation_id or fallback.installation_id,
            installation_uuid=installation_uuid or fallback.installation_uuid,
            is_active=True,
        )

    def create_or_get_profile(self, account_email: str) -> DeviceProfile:
        """Returns registered profile for account or creates and persists a new one."""
        return self.store.get_or_create_profile(account_email)

    def write_active_profile(self, profile: DeviceProfile) -> None:
        """
        Writes all 4 fingerprint components to disk atomically.
        Guarded against overwriting the real host system during test runs.
        """
        if self._is_host_environment_protected():
            logger.info("SAFETY SHIELD: Skipping write of device profile to real host files in test mode.")
            return

        self.config_dir.mkdir(parents=True, exist_ok=True)
        self.data_dir.mkdir(parents=True, exist_ok=True)

        # 1. machineid (exact 36B, no newline)
        self._write_exact_36b_file(self.machineid_path, profile.machine_id)

        # 2. .updaterId (exact 36B, no newline)
        self._write_exact_36b_file(self.updater_id_path, profile.updater_id)

        # 3. installation_id (exact 36B, no newline)
        self._write_exact_36b_file(self.installation_id_path, profile.installation_id)
        if self.config_installation_id_path.exists():
            self._write_exact_36b_file(self.config_installation_id_path, profile.installation_id)

        # 4. antigravity_state.pbtxt (installation_uuid & installation_id)
        target_pbtxt = self.pbtxt_path if (self.pbtxt_path.exists() or not self.config_pbtxt_path.exists()) else self.config_pbtxt_path
        PbtxtParser.update_installation_fields(
            target_pbtxt,
            installation_uuid=profile.installation_uuid,
            installation_id=profile.installation_id,
        )

        logger.info("Successfully updated device fingerprint profile on disk.")

    def swap_profile_for_account(self, account_email: str) -> DeviceProfile:
        """
        Swaps the physical fingerprint files on disk to match the account's profile.
        If no profile exists for the account, a fresh profile is generated, saved, and applied.
        """
        profile = self.create_or_get_profile(account_email)
        self.write_active_profile(profile)
        self.store.set_active_account(account_email)
        return profile

    def attach_to_keyring_service(self, keyring_service: Any) -> None:
        """
        Attaches a switch listener to KeyringService so that whenever credentials rotate,
        the corresponding virtual device profile is automatically swapped on disk.
        """
        def _on_keyring_switch(email: str, reason: str) -> None:
            logger.info("Keyring switched to %s (reason: %s); swapping device profile.", email, reason)
            self.swap_profile_for_account(email)

        keyring_service.register_switch_listener(_on_keyring_switch)
