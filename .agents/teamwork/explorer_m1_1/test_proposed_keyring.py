"""
Verification test suite for proposed Keyring Switcher modules.
Runs in-memory and against isolated temporary files and mock/test service namespaces.
"""

import os
import sys
import tempfile
import unittest
from pathlib import Path

# Add explorer directory to sys.path
SCRIPT_DIR = Path(__file__).parent.resolve()
sys.path.insert(0, str(SCRIPT_DIR))

from proposed_secret_tool import (
    SecretToolBackend,
    KeyringError,
    KeyringNotFoundError,
    KeyringTimeoutError,
    KeyringBackendUnavailableError,
)
from proposed_dbus_keyring import LibsecretBackend, DBusKeyring
from proposed_switcher import (
    KeyringCredential,
    AccountRecord,
    AccountVault,
    KeyringService,
    AccountNotFoundError,
    InvalidCredentialError,
    AccountVaultCorruptedError,
)


class MockKeyringBackend:
    """In-memory mock backend for pure logic testing."""

    def __init__(self):
        self.store_dict = {}

    def lookup(self, service=None, username=None):
        return self.store_dict.get((service, username))

    def store(self, secret, service=None, username=None, label=None):
        if isinstance(secret, bytes):
            s = secret.rstrip(b"\r\n").decode("utf-8")
        else:
            s = secret.rstrip("\r\n")
        self.store_dict[(service, username)] = s

    def clear(self, service=None, username=None, ignore_missing=True):
        if (service, username) in self.store_dict:
            del self.store_dict[(service, username)]
            return True
        if not ignore_missing:
            raise KeyringNotFoundError("Not found")
        return False


class TestKeyringCredential(unittest.TestCase):
    def test_roundtrip_antigravity_json(self):
        cred = KeyringCredential(
            access_token="ya29.test-access-token",
            refresh_token="1//test-refresh-token",
            token_type="Bearer",
            expiry="2026-10-01T12:00:00Z",
            auth_method="consumer",
            id_token="",
        )
        raw_json = cred.to_antigravity_json()
        # Ensure no trailing newline
        self.assertFalse(raw_json.endswith("\n"))
        self.assertIn('"token":{', raw_json)
        self.assertIn('"access_token":"ya29.test-access-token"', raw_json)
        self.assertIn('"auth_method":"consumer"', raw_json)
        self.assertNotIn("id_token", raw_json)

        # Parse back
        parsed = KeyringCredential.from_antigravity_json(raw_json)
        self.assertEqual(parsed.access_token, cred.access_token)
        self.assertEqual(parsed.refresh_token, cred.refresh_token)
        self.assertEqual(parsed.token_type, "Bearer")
        self.assertEqual(parsed.expiry, cred.expiry)
        self.assertEqual(parsed.auth_method, "consumer")
        self.assertEqual(parsed.id_token, "")

    def test_id_token_preservation(self):
        cred = KeyringCredential(
            access_token="ya29.access",
            refresh_token="1//refresh",
            token_type="Bearer",
            expiry="2026-10-01T12:00:00Z",
            auth_method="consumer",
            id_token="header.eyJlbWFpbCI6ICJ1c2VyQGV4YW1wbGUuY29tIn0.sig",
        )
        raw_json = cred.to_antigravity_json()
        self.assertIn('"id_token":"header.', raw_json)

        parsed = KeyringCredential.from_antigravity_json(raw_json)
        self.assertEqual(parsed.id_token, cred.id_token)
        email = parsed.extract_email_from_id_token()
        self.assertEqual(email, "user@example.com")

    def test_invalid_credential_json(self):
        with self.assertRaises(InvalidCredentialError):
            KeyringCredential.from_antigravity_json("")

        with self.assertRaises(InvalidCredentialError):
            KeyringCredential.from_antigravity_json("not json")

        with self.assertRaises(InvalidCredentialError):
            KeyringCredential.from_antigravity_json('{"other": 123}')


class TestAccountVault(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.config_path = Path(self.temp_dir.name) / "sub" / "accounts.json"
        self.lock_path = Path(self.temp_dir.name) / "sub" / "accounts.lock"
        self.vault = AccountVault(self.config_path, self.lock_path)

    def tearDown(self):
        self.temp_dir.cleanup()

    def test_permissions_and_atomic_save(self):
        cred = KeyringCredential("acc1", "ref1")
        self.vault.add_or_update_account("user1@gmail.com", cred, label="User 1")

        # Verify parent directory permissions (0700)
        parent_mode = self.config_path.parent.stat().st_mode & 0o777
        self.assertEqual(parent_mode, 0o700)

        # Verify file permissions (0600)
        file_mode = self.config_path.stat().st_mode & 0o777
        self.assertEqual(file_mode, 0o600)

        # Verify account retrieval
        accounts = self.vault.list_accounts()
        self.assertEqual(accounts, ["user1@gmail.com"])
        rec = self.vault.get_account("user1@gmail.com")
        self.assertIsNotNone(rec)
        self.assertEqual(rec.credential.access_token, "acc1")
        self.assertEqual(rec.label, "User 1")

    def test_multi_account_and_active_account(self):
        c1 = KeyringCredential("acc1", "ref1")
        c2 = KeyringCredential("acc2", "ref2")
        self.vault.add_or_update_account("u1@gmail.com", c1)
        self.vault.add_or_update_account("u2@gmail.com", c2)

        self.assertEqual(self.vault.list_accounts(), ["u1@gmail.com", "u2@gmail.com"])
        self.assertEqual(self.vault.get_active_account(), "u1@gmail.com")

        self.vault.set_active_account("u2@gmail.com")
        self.assertEqual(self.vault.get_active_account(), "u2@gmail.com")

        # Remove u1
        self.assertTrue(self.vault.remove_account("u1@gmail.com"))
        self.assertEqual(self.vault.list_accounts(), ["u2@gmail.com"])


class TestKeyringService(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.config_path = Path(self.temp_dir.name) / "accounts.json"
        self.vault = AccountVault(self.config_path)
        self.backend = MockKeyringBackend()
        self.storage_path = Path(self.temp_dir.name) / "app_storage.json"
        self.service = KeyringService(
            backend=self.backend,
            vault=self.vault,
            storage_path=self.storage_path,
        )

    def tearDown(self):
        self.temp_dir.cleanup()

    def test_contract_compliance(self):
        # 1. set_active_credential
        initial_cred = KeyringCredential("tok_a", "ref_a", "Bearer", "2026-10-01T10:00:00Z")
        self.service.set_active_credential(initial_cred)

        # 2. get_active_credential
        active = self.service.get_active_credential()
        self.assertEqual(active.access_token, "tok_a")
        self.assertEqual(active.refresh_token, "ref_a")

        # 3. Add accounts to vault
        cred_b = KeyringCredential("tok_b", "ref_b", "Bearer", "2026-10-01T11:00:00Z")
        self.vault.add_or_update_account("a@gmail.com", initial_cred)
        self.vault.add_or_update_account("b@gmail.com", cred_b)
        self.vault.set_active_account("a@gmail.com")

        # 4. list_accounts
        self.assertEqual(self.service.list_accounts(), ["a@gmail.com", "b@gmail.com"])

        # 5. switch_account with event listener
        switched_events = []
        self.service.register_switch_listener(lambda email, reason: switched_events.append((email, reason)))

        success = self.service.switch_account("b@gmail.com", reason="quota_exhausted")
        self.assertTrue(success)
        self.assertEqual(switched_events, [("b@gmail.com", "quota_exhausted")])

        # Active credential in keyring should now be b
        new_active = self.service.get_active_credential()
        self.assertEqual(new_active.access_token, "tok_b")
        self.assertEqual(self.vault.get_active_account(), "b@gmail.com")

    def test_switch_nonexistent_account(self):
        with self.assertRaises(AccountNotFoundError):
            self.service.switch_account("nonexistent@gmail.com")

    def test_auto_ingest_when_empty(self):
        payload = '{"token":{"access_token":"ya29.initial","token_type":"Bearer","refresh_token":"1//ref","expiry":""},"auth_method":"consumer"}'
        self.backend.store(payload, service="gemini", username="antigravity")
        accounts = self.service.list_accounts()
        self.assertEqual(accounts, ["primary@antigravity"])
        self.assertEqual(self.vault.get_active_account(), "primary@antigravity")
        rec = self.vault.get_account("primary@antigravity")
        self.assertIsNotNone(rec)
        self.assertEqual(rec.credential.access_token, "ya29.initial")

    def test_email_discovery_from_app_storage(self):
        with open(self.storage_path, "w", encoding="utf-8") as f:
            f.write('{"jetski.onboarding.lastLoginUsername": "discovered@example.com"}')
        payload = '{"token":{"access_token":"ya29.test","token_type":"Bearer","refresh_token":"1//ref","expiry":""},"auth_method":"consumer"}'
        self.backend.store(payload, service="gemini", username="antigravity")
        accounts = self.service.list_accounts()
        self.assertEqual(accounts, ["discovered@example.com"])
        self.assertEqual(self.vault.get_active_account(), "discovered@example.com")


class TestSecretToolLive(unittest.TestCase):
    """Tests directly against host secret-tool in an isolated test service namespace."""

    def setUp(self):
        self.service_name = "antigravity_test_harness_svc"
        self.username = "test_user_harness"
        self.backend = SecretToolBackend(
            default_service=self.service_name,
            default_username=self.username,
        )

    def tearDown(self):
        try:
            self.backend.clear(self.service_name, self.username)
        except Exception:
            pass

    def test_live_secret_tool_store_lookup_clear(self):
        if not SecretToolBackend.is_available():
            self.skipTest("secret-tool not available on host")

        # Missing secret lookup returns None
        self.assertIsNone(self.backend.lookup())

        # Store with trailing newline (should be cleanly stripped)
        payload = '{"token":{"access_token":"live-test-123","token_type":"Bearer","refresh_token":"ref-123","expiry":""},"auth_method":"consumer"}\n'
        self.backend.store(payload)

        # Lookup
        read_back = self.backend.lookup()
        self.assertIsNotNone(read_back)
        self.assertFalse(read_back.endswith("\n"))
        self.assertIn("live-test-123", read_back)

        # Clear
        cleared = self.backend.clear()
        self.assertTrue(cleared)
        self.assertIsNone(self.backend.lookup())


class TestLibsecretLive(unittest.TestCase):
    """Tests directly against libsecret via PyGObject."""

    def setUp(self):
        self.service_name = "antigravity_test_libsecret_svc"
        self.username = "test_user_libsecret"
        self.backend = LibsecretBackend(
            default_service=self.service_name,
            default_username=self.username,
        )

    def tearDown(self):
        try:
            self.backend.clear(self.service_name, self.username)
        except Exception:
            pass

    def test_live_libsecret_store_lookup_clear(self):
        if not LibsecretBackend.is_available():
            self.skipTest("PyGObject Secret not available")

        # Missing lookup returns None
        self.assertIsNone(self.backend.lookup())

        # Store
        payload = '{"token":{"access_token":"libsecret-test","token_type":"Bearer","refresh_token":"ref","expiry":""},"auth_method":"consumer"}'
        self.backend.store(payload)

        # Lookup
        read_back = self.backend.lookup()
        self.assertEqual(read_back, payload)

        # Clear
        cleared = self.backend.clear()
        self.assertTrue(cleared)
        self.assertIsNone(self.backend.lookup())


if __name__ == "__main__":
    unittest.main(verbosity=2)
