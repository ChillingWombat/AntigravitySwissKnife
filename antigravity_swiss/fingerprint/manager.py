"""
Hardware and Environment Device Fingerprint Manager (Feature F10, F11, F13).
=============================================================================
Manages physical device identification files and coordinates atomic profile swaps:
1. ~/.config/Antigravity/machineid
2. ~/.config/Antigravity/.updaterId
3. ~/.gemini/antigravity/installation_id
4. ~/.gemini/antigravity/antigravity_state.pbtxt (installation_uuid)
"""

from __future__ import annotations

import logging
import os
from pathlib import Path
import tempfile
from typing import Optional

from antigravity_swiss.core.constants import (
    DEFAULT_ANTIGRAVITY_CONFIG_DIR,
    DEFAULT_ANTIGRAVITY_DATA_DIR,
)
from antigravity_swiss.core.errors import SwissKnifeError
from antigravity_swiss.fingerprint.pbtxt_parser import PbtxtParser
from antigravity_swiss.fingerprint.profile_store import DeviceProfile, DeviceProfileStore

logger = logging.getLogger("antigravity_swiss.fingerprint.manager")


class FingerprintManager:
    """Coordinates reading, storing, and swapping device profiles across accounts."""

    def __init__(
        self,
        config_dir: Optional[Path | str] = None,
        data_dir: Optional[Path | str] = None,
        store: Optional[DeviceProfileStore] = None,
    ) -> None:
        self.config_dir = Path(config_dir or DEFAULT_ANTIGRAVITY_CONFIG_DIR).expanduser().resolve()
        self.data_dir = Path(data_dir or DEFAULT_ANTIGRAVITY_DATA_DIR).expanduser().resolve()
        self.store = store or DeviceProfileStore()

        self.machineid_path = self.config_dir / "machineid"
        self.updater_id_path = self.config_dir / ".updaterId"
        self.installation_id_path = self.data_dir / "installation_id"
        self.pbtxt_path = self.data_dir / "antigravity_state.pbtxt"

    def _is_host_environment_protected(self) -> bool:
        """Determines if operation should be blocked to protect host IDE during tests."""
        is_testing = bool(os.environ.get("PYTEST_CURRENT_TEST") or os.environ.get("ANTIGRAVITY_SWISS_TESTING"))
        real_config = Path(DEFAULT_ANTIGRAVITY_CONFIG_DIR).expanduser().resolve()
        real_data = Path(DEFAULT_ANTIGRAVITY_DATA_DIR).expanduser().resolve()
        return is_testing and (self.config_dir == real_config or self.data_dir == real_data)

    def get_active_profile(self) -> DeviceProfile:
        """
        Inspects the active filesystem state and reads current device fingerprint IDs.
        If files do not exist, falls back to freshly generated values.
        """
        machine_id = ""
        if self.machineid_path.exists():
            try:
                machine_id = self.machineid_path.read_text(encoding="utf-8").strip()
            except Exception as exc:
                logger.warning("Could not read %s: %s", self.machineid_path, exc)

        updater_id = ""
        if self.updater_id_path.exists():
            try:
                updater_id = self.updater_id_path.read_text(encoding="utf-8").strip()
            except Exception as exc:
                logger.warning("Could not read %s: %s", self.updater_id_path, exc)

        installation_id = ""
        if self.installation_id_path.exists():
            try:
                installation_id = self.installation_id_path.read_text(encoding="utf-8").strip()
            except Exception as exc:
                logger.warning("Could not read %s: %s", self.installation_id_path, exc)

        installation_uuid = PbtxtParser.extract_installation_uuid(self.pbtxt_path) or ""

        # Fill any missing values with valid random values
        fallback = DeviceProfile.generate_random()
        return DeviceProfile(
            machine_id=machine_id or fallback.machine_id,
            updater_id=updater_id or fallback.updater_id,
            installation_id=installation_id or fallback.installation_id,
            installation_uuid=installation_uuid or fallback.installation_uuid,
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

        # 1. machineid
        tmp_machineid = self.machineid_path.with_suffix(".tmp")
        tmp_machineid.write_text(profile.machine_id.strip() + "\n", encoding="utf-8")
        os.replace(tmp_machineid, self.machineid_path)

        # 2. .updaterId
        tmp_updater = self.updater_id_path.with_suffix(".tmp")
        tmp_updater.write_text(profile.updater_id.strip() + "\n", encoding="utf-8")
        os.replace(tmp_updater, self.updater_id_path)

        # 3. installation_id
        tmp_inst = self.installation_id_path.with_suffix(".tmp")
        tmp_inst.write_text(profile.installation_id.strip() + "\n", encoding="utf-8")
        os.replace(tmp_inst, self.installation_id_path)

        # 4. antigravity_state.pbtxt (installation_uuid)
        PbtxtParser.update_installation_uuid(self.pbtxt_path, profile.installation_uuid.strip())

        logger.info("Successfully updated device fingerprint profile on disk.")

    def swap_profile_for_account(self, account_email: str) -> DeviceProfile:
        """
        Swaps the physical fingerprint files on disk to match the account's profile.
        If no profile exists for the account, a fresh profile is generated, saved, and applied.
        """
        profile = self.create_or_get_profile(account_email)
        self.write_active_profile(profile)
        return profile
