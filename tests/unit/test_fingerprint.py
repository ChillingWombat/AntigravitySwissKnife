"""
Unit tests for Device Fingerprint Manager, Profile Store, and Pbtxt Parser (Requirement R4).
=============================================================================================
"""

import os
from pathlib import Path
import re
import tempfile
import uuid
import pytest

from antigravity_swiss.fingerprint.manager import FingerprintManager
from antigravity_swiss.fingerprint.pbtxt_parser import PbtxtParser
from antigravity_swiss.fingerprint.profile_store import DeviceProfile, DeviceProfileStore


def test_device_profile_generation_and_serialization():
    """Verify DeviceProfile random generation produces valid UUIDs and roundtrips cleanly."""
    prof = DeviceProfile.generate_random()
    # Check valid UUID format
    assert uuid.UUID(prof.machine_id)
    assert uuid.UUID(prof.updater_id)
    assert uuid.UUID(prof.installation_id)
    assert uuid.UUID(prof.installation_uuid)

    d = prof.to_dict()
    prof2 = DeviceProfile.from_dict(d)
    assert prof2 == prof


def test_device_profile_store_crud_and_permissions(temp_dir):
    """Verify DeviceProfileStore saves atomically with 0600 permissions and manages profiles."""
    store_file = Path(temp_dir) / "sub" / "device_profiles.json"
    store = DeviceProfileStore(storage_path=store_file)

    assert store.list_accounts() == []

    p1 = store.get_or_create_profile("alice@example.com")
    assert p1 is not None
    assert "alice@example.com" in store.list_accounts()

    # Verify file mode is 0600 (owner read/write only)
    assert store_file.exists()
    mode = store_file.stat().st_mode & 0o777
    assert mode == 0o600

    # Retrieve again: should return same profile
    p1_cached = store.get_profile("alice@example.com")
    assert p1_cached == p1

    # Reload fresh instance from disk
    store2 = DeviceProfileStore(storage_path=store_file)
    assert store2.get_profile("alice@example.com") == p1

    # Delete
    assert store2.delete_profile("alice@example.com") is True
    assert store2.get_profile("alice@example.com") is None
    assert store2.delete_profile("alice@example.com") is False


def test_pbtxt_parser_extract_and_update(temp_dir):
    """Verify PbtxtParser preserves comments and message blocks while updating installation_uuid."""
    pbtxt_path = Path(temp_dir) / "antigravity_state.pbtxt"

    sample_content = """# Header comment
post_onboarding: {
  completed_steps: POST_ONBOARDING_STEP_TYPE_MANAGER_WELCOME
  completed_steps: POST_ONBOARDING_STEP_TYPE_USAGE_MODE
}
installation_uuid: "11111111-2222-3333-4444-555555555555" # inline comment
seen_nuxs: {
  uids: 27
}
"""
    pbtxt_path.write_text(sample_content, encoding="utf-8")

    # 1. Extract
    extracted = PbtxtParser.extract_installation_uuid(pbtxt_path)
    assert extracted == "11111111-2222-3333-4444-555555555555"

    # 2. Update in-place
    new_uuid = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
    success = PbtxtParser.update_installation_uuid(pbtxt_path, new_uuid)
    assert success is True

    # 3. Re-extract and verify
    assert PbtxtParser.extract_installation_uuid(pbtxt_path) == new_uuid

    # 4. Verify comments and blocks intact
    updated_text = pbtxt_path.read_text(encoding="utf-8")
    assert "# Header comment" in updated_text
    assert "POST_ONBOARDING_STEP_TYPE_MANAGER_WELCOME" in updated_text
    assert "# inline comment" in updated_text
    assert "uids: 27" in updated_text


def test_pbtxt_parser_missing_key_and_new_file(temp_dir):
    """Verify PbtxtParser appends key when missing or creates new file."""
    pbtxt_path = Path(temp_dir) / "missing_uuid.pbtxt"
    pbtxt_path.write_text("post_onboarding: {}\n", encoding="utf-8")

    assert PbtxtParser.extract_installation_uuid(pbtxt_path) is None

    PbtxtParser.update_installation_uuid(pbtxt_path, "99999999-8888-7777-6666-555555555555")
    assert PbtxtParser.extract_installation_uuid(pbtxt_path) == "99999999-8888-7777-6666-555555555555"
    assert "post_onboarding: {}" in pbtxt_path.read_text()

    # Brand new non-existent file
    brand_new = Path(temp_dir) / "brand_new.pbtxt"
    PbtxtParser.update_installation_uuid(brand_new, "12345678-1234-1234-1234-123456789abc")
    assert PbtxtParser.extract_installation_uuid(brand_new) == "12345678-1234-1234-1234-123456789abc"


def test_fingerprint_manager_atomic_swap(temp_dir):
    """Verify FingerprintManager swaps all 4 fingerprint components across accounts."""
    config_dir = Path(temp_dir) / "config"
    data_dir = Path(temp_dir) / "data"
    store_file = Path(temp_dir) / "device_profiles.json"
    store = DeviceProfileStore(storage_path=store_file)

    mgr = FingerprintManager(config_dir=config_dir, data_dir=data_dir, store=store)

    # Swap to Account A
    prof_a = mgr.swap_profile_for_account("alice@example.com")
    assert mgr.machineid_path.read_text().strip() == prof_a.machine_id
    assert mgr.updater_id_path.read_text().strip() == prof_a.updater_id
    assert mgr.installation_id_path.read_text().strip() == prof_a.installation_id
    assert PbtxtParser.extract_installation_uuid(mgr.pbtxt_path) == prof_a.installation_uuid

    active_a = mgr.get_active_profile()
    assert active_a == prof_a

    # Swap to Account B
    prof_b = mgr.swap_profile_for_account("bob@example.com")
    assert prof_b != prof_a
    assert mgr.machineid_path.read_text().strip() == prof_b.machine_id
    assert mgr.updater_id_path.read_text().strip() == prof_b.updater_id
    assert mgr.installation_id_path.read_text().strip() == prof_b.installation_id
    assert PbtxtParser.extract_installation_uuid(mgr.pbtxt_path) == prof_b.installation_uuid

    # Swap back to Account A
    prof_a_again = mgr.swap_profile_for_account("alice@example.com")
    assert prof_a_again == prof_a
    assert mgr.machineid_path.read_text().strip() == prof_a.machine_id
    assert PbtxtParser.extract_installation_uuid(mgr.pbtxt_path) == prof_a.installation_uuid


def test_fingerprint_manager_safety_shield(monkeypatch):
    """Verify FingerprintManager refuses to overwrite host filesystem when pointing to default dirs during tests."""
    from antigravity_swiss.core.constants import DEFAULT_ANTIGRAVITY_CONFIG_DIR, DEFAULT_ANTIGRAVITY_DATA_DIR

    mgr = FingerprintManager(
        config_dir=DEFAULT_ANTIGRAVITY_CONFIG_DIR,
        data_dir=DEFAULT_ANTIGRAVITY_DATA_DIR,
    )
    assert mgr._is_host_environment_protected() is True

    # write_active_profile should be a no-op that doesn't modify host files
    prof = DeviceProfile.generate_random()
    mgr.write_active_profile(prof)
