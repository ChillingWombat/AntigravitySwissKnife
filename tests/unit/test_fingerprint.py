"""
Unit tests for Device Fingerprint Manager, Profile Store, and Pbtxt Parser (Requirement R4, Features F10 & F11).
===================================================================================================================
"""

import json
import os
from pathlib import Path
import threading
import time
import uuid
import pytest

from antigravity_swiss.core.errors import FingerprintError
from antigravity_swiss.fingerprint.manager import FingerprintManager
from antigravity_swiss.fingerprint.models import DeviceProfile
from antigravity_swiss.fingerprint.pbtxt_parser import PbtxtParser
from antigravity_swiss.fingerprint.profile_store import (
    DeviceProfileStore,
    ProfileStore,
    ProfileStoreCorruptedError,
)


def test_device_profile_generation_and_serialization():
    """Verify DeviceProfile random generation produces valid UUIDs and roundtrips cleanly."""
    prof = DeviceProfile.generate_random(account_email="test@gmail.com")
    assert prof.account_email == "test@gmail.com"
    # Check valid UUID format
    assert uuid.UUID(prof.machine_id)
    assert uuid.UUID(prof.updater_id)
    assert uuid.UUID(prof.installation_id)
    assert uuid.UUID(prof.installation_uuid)
    assert prof.created_at is not None
    assert prof.last_used_at is None
    assert prof.is_active is False

    d = prof.to_dict()
    assert d["account_email"] == "test@gmail.com"
    assert d["machine_id"] == prof.machine_id

    prof2 = DeviceProfile.from_dict(d)
    assert prof2.account_email == prof.account_email
    assert prof2.machine_id == prof.machine_id
    assert prof2.updater_id == prof.updater_id
    assert prof2.installation_id == prof.installation_id
    assert prof2.installation_uuid == prof.installation_uuid

    # Legacy camelCase tolerance
    legacy_data = {
        "machineid": str(uuid.uuid4()),
        "updaterId": str(uuid.uuid4()),
        "installation_id": str(uuid.uuid4()),
        "installation_uuid": str(uuid.uuid4()),
    }
    prof_legacy = DeviceProfile.from_dict(legacy_data, account_email="legacy@gmail.com")
    assert prof_legacy.machine_id == legacy_data["machineid"]
    assert prof_legacy.updater_id == legacy_data["updaterId"]
    assert prof_legacy.account_email == "legacy@gmail.com"


def test_device_profile_store_crud_and_permissions(temp_dir):
    """Verify ProfileStore saves atomically with 0600 permissions and manages profiles."""
    store_file = Path(temp_dir) / "sub" / "profiles.json"
    store = ProfileStore(storage_path=store_file)

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
    assert p1_cached.machine_id == p1.machine_id

    # Active account tracking
    store.set_active_account("alice@example.com")
    assert store.get_active_account() == "alice@example.com"
    p1_active = store.get_profile("alice@example.com")
    assert p1_active.is_active is True
    assert p1_active.last_used_at is not None

    # Reload fresh instance from disk
    store2 = DeviceProfileStore(storage_path=store_file)
    assert store2.get_profile("alice@example.com").machine_id == p1.machine_id
    assert store2.get_active_account() == "alice@example.com"

    # Delete
    assert store2.delete_profile("alice@example.com") is True
    assert store2.get_profile("alice@example.com") is None
    assert store2.delete_profile("alice@example.com") is False
    assert store2.get_active_account() is None


def test_profile_store_legacy_migration(temp_dir):
    """Verify ProfileStore auto-migrates from device_profiles.json if present."""
    config_dir = Path(temp_dir) / "cfg"
    config_dir.mkdir(parents=True)
    legacy_file = config_dir / "device_profiles.json"
    target_file = config_dir / "profiles.json"

    # Seed legacy store
    legacy_store = DeviceProfileStore(storage_path=legacy_file)
    p = legacy_store.get_or_create_profile("migrated@gmail.com")

    # Creating fresh ProfileStore pointing to profiles.json triggers auto-migration
    new_store = ProfileStore(storage_path=target_file)
    assert target_file.exists()
    assert "migrated@gmail.com" in new_store.list_accounts()
    migrated_prof = new_store.get_profile("migrated@gmail.com")
    assert migrated_prof.machine_id == p.machine_id


def test_profile_store_flock_concurrency(temp_dir):
    """Verify ProfileStore handles concurrent thread operations safely with flock."""
    store_file = Path(temp_dir) / "concurrent_profiles.json"
    store = ProfileStore(storage_path=store_file)

    def _worker(thread_id: int):
        for i in range(10):
            email = f"user_{thread_id}_{i}@gmail.com"
            prof = store.get_or_create_profile(email)
            assert len(prof.machine_id) == 36

    threads = [threading.Thread(target=_worker, args=(t,)) for t in range(5)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()

    accounts = store.list_accounts()
    assert len(accounts) == 50


def test_profile_store_quarantine_corrupted(temp_dir):
    """Verify corrupted JSON triggers quarantine file creation and raises ProfileStoreCorruptedError."""
    store_file = Path(temp_dir) / "corrupt_store" / "profiles.json"
    store_file.parent.mkdir(parents=True)
    store_file.write_text("{ incomplete json", encoding="utf-8")

    store = ProfileStore(storage_path=store_file)
    with pytest.raises(ProfileStoreCorruptedError):
        store.get_profile("anyone@gmail.com")

    # Verify quarantine file was created
    corrupted_files = list(store_file.parent.glob("profiles.json.corrupted.*"))
    assert len(corrupted_files) >= 1
    assert corrupted_files[0].read_text(encoding="utf-8") == "{ incomplete json"


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


def test_pbtxt_parser_dual_field_update(temp_dir):
    """Verify PbtxtParser updates both installation_uuid and installation_id atomically."""
    pbtxt_path = Path(temp_dir) / "state_dual.pbtxt"
    pbtxt_path.write_text("""post_onboarding: { completed_steps: STEP_A }
installation_uuid: "old-uuid-1111"
installation_id: "old-id-2222"
migrations: { key: 1 }
""", encoding="utf-8")

    success = PbtxtParser.update_installation_fields(
        pbtxt_path,
        installation_uuid="33333333-3333-3333-3333-333333333333",
        installation_id="44444444-4444-4444-4444-444444444444",
    )
    assert success is True

    content = pbtxt_path.read_text(encoding="utf-8")
    assert 'installation_uuid: "33333333-3333-3333-3333-333333333333"' in content
    assert 'installation_id: "44444444-4444-4444-4444-444444444444"' in content
    assert "completed_steps: STEP_A" in content
    assert "migrations: { key: 1 }" in content


def test_exact_36b_raw_ascii_writes(temp_dir):
    """Verify _write_exact_36b_file writes exactly 36 bytes with zero trailing newlines."""
    mgr = FingerprintManager(config_dir=temp_dir, data_dir=temp_dir)
    target_file = Path(temp_dir) / "test_machineid"

    valid_uuid = "12345678-1234-1234-1234-123456789abc"
    mgr._write_exact_36b_file(target_file, valid_uuid)

    data = target_file.read_bytes()
    assert len(data) == 36
    assert not data.endswith(b"\n")
    assert not data.endswith(b"\r")
    assert target_file.read_text(encoding="ascii") == valid_uuid

    # Reject invalid byte length
    with pytest.raises(FingerprintError):
        mgr._write_exact_36b_file(target_file, "too-short")

    with pytest.raises(FingerprintError):
        mgr._write_exact_36b_file(target_file, valid_uuid + "extra")


def test_fingerprint_manager_atomic_swap(temp_dir):
    """Verify FingerprintManager swaps all 4 fingerprint components across accounts."""
    config_dir = Path(temp_dir) / "config"
    data_dir = Path(temp_dir) / "data"
    store_file = Path(temp_dir) / "profiles.json"
    store = ProfileStore(storage_path=store_file)

    mgr = FingerprintManager(config_dir=config_dir, data_dir=data_dir, store=store)

    # Swap to Account A
    prof_a = mgr.swap_profile_for_account("alice@example.com")
    assert len(mgr.machineid_path.read_bytes()) == 36
    assert not mgr.machineid_path.read_bytes().endswith(b"\n")
    assert mgr.machineid_path.read_text().strip() == prof_a.machine_id
    assert mgr.updater_id_path.read_text().strip() == prof_a.updater_id
    assert mgr.installation_id_path.read_text().strip() == prof_a.installation_id
    assert PbtxtParser.extract_installation_uuid(mgr.pbtxt_path) == prof_a.installation_uuid

    active_a = mgr.get_active_profile()
    assert active_a.machine_id == prof_a.machine_id
    assert active_a.account_email == "alice@example.com"

    # Swap to Account B
    prof_b = mgr.swap_profile_for_account("bob@example.com")
    assert prof_b.machine_id != prof_a.machine_id
    assert mgr.machineid_path.read_text().strip() == prof_b.machine_id
    assert mgr.updater_id_path.read_text().strip() == prof_b.updater_id
    assert mgr.installation_id_path.read_text().strip() == prof_b.installation_id
    assert PbtxtParser.extract_installation_uuid(mgr.pbtxt_path) == prof_b.installation_uuid

    # Swap back to Account A restores identical original IDs
    prof_a_again = mgr.swap_profile_for_account("alice@example.com")
    assert prof_a_again.machine_id == prof_a.machine_id
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


def test_fingerprint_manager_keyring_hook_sync(temp_dir):
    """Verify attach_to_keyring_service swaps profile on credential rotation."""
    config_dir = Path(temp_dir) / "cfg"
    data_dir = Path(temp_dir) / "data"
    mgr = FingerprintManager(config_dir=config_dir, data_dir=data_dir)

    class DummyKeyringService:
        def __init__(self):
            self.listeners = []

        def register_switch_listener(self, listener):
            self.listeners.append(listener)

        def simulate_switch(self, email: str, reason: str):
            for l in self.listeners:
                l(email, reason)

    keyring_svc = DummyKeyringService()
    mgr.attach_to_keyring_service(keyring_svc)

    keyring_svc.simulate_switch("synced_user@gmail.com", "quota_rotation")
    active = mgr.get_active_profile()
    assert active.account_email == "synced_user@gmail.com"
    assert mgr.machineid_path.exists()
    assert len(mgr.machineid_path.read_bytes()) == 36
