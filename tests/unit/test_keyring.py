"""
Unit tests for antigravity_swiss.keyring: secret_tool, dbus_keyring, and switcher.
"""

import json
import os
from pathlib import Path
import pytest

from antigravity_swiss.core.errors import AccountNotFoundError, InvalidCredentialError
from antigravity_swiss.keyring.secret_tool import SecretToolBackend
from antigravity_swiss.keyring.switcher import (
    AccountRecord,
    AccountStore,
    AccountVault,
    KeyringCredential,
    KeyringService,
    KeyringSwitcher,
)
from tests.fixtures.test_helpers import CredentialBuilder


def test_keyring_credential_roundtrip():
    """Verify KeyringCredential serializes to and parses from Antigravity JSON."""
    raw = CredentialBuilder.build_valid_payload("alice@gmail.com", access_token="ya29.alice123")
    cred = KeyringCredential.from_antigravity_json(raw)
    assert cred.access_token == "ya29.alice123"
    assert cred.token_type == "Bearer"
    assert cred.auth_method == "consumer"
    assert cred.extract_email_from_id_token() == "alice@gmail.com"

    # Re-serialize to JSON and re-parse
    out_json = cred.to_antigravity_json()
    cred2 = KeyringCredential.from_antigravity_json(out_json)
    assert cred2.access_token == cred.access_token
    assert cred2.refresh_token == cred.refresh_token
    assert cred2.id_token == cred.id_token


def test_keyring_credential_invalid_inputs():
    """Verify KeyringCredential rejects empty or malformed inputs."""
    with pytest.raises(InvalidCredentialError):
        KeyringCredential.from_antigravity_json("")

    with pytest.raises(InvalidCredentialError):
        KeyringCredential.from_antigravity_json("{bad_json")

    with pytest.raises(InvalidCredentialError):
        KeyringCredential.from_antigravity_json(json.dumps({"token": {}, "auth_method": "consumer"}))


def test_secret_tool_backend_operations(isolated_env, mock_keyring):
    """Verify SecretToolBackend lookup, store without trailing newline, and clear."""
    backend = SecretToolBackend()
    assert backend.is_available()

    # Lookup seeded credential
    val = backend.lookup(service="gemini", username="antigravity")
    assert val is not None
    assert "user-primary@gmail.com" in val

    # Store with trailing newline in input -> should be stripped
    payload_with_nl = CredentialBuilder.build_valid_payload("bob@gmail.com") + "\n\n"
    backend.store(payload_with_nl, service="gemini", username="antigravity")

    # Read back and assert NO newline at end
    val2 = backend.lookup(service="gemini", username="antigravity")
    assert val2 is not None
    assert not val2.endswith("\n")
    assert "bob@gmail.com" in val2

    # Clear secret
    cleared = backend.clear(service="gemini", username="antigravity")
    assert cleared is True
    assert backend.lookup(service="gemini", username="antigravity") is None

    # Idempotent clear on absent secret succeeds without error
    backend.clear(service="gemini", username="antigravity", ignore_missing=True)
    assert backend.lookup(service="gemini", username="antigravity") is None


def test_account_vault_permissions_and_concurrency(temp_dir):
    """Verify AccountVault permissions (0600 file, 0700 dir) and atomic operations."""
    vault_file = Path(temp_dir) / "accounts" / "accounts.json"
    vault = AccountVault(config_path=vault_file)

    cred = KeyringCredential("token1", "refresh1", id_token="ey.eyJlbWFpbCI6ICJ1c2VyMUBnbWFpbC5jb20ifQ==.sig")
    vault.add_or_update_account("user1@gmail.com", cred, label="User 1")

    assert vault_file.exists()
    assert (vault_file.parent.stat().st_mode & 0o777) == 0o700
    assert (vault_file.stat().st_mode & 0o777) == 0o600

    loaded = vault.load()
    assert "user1@gmail.com" in loaded["accounts"]
    assert vault.get_active_account() == "user1@gmail.com"

    # Add second account and switch active
    cred2 = KeyringCredential("token2", "refresh2", id_token="ey.eyJlbWFpbCI6ICJ1c2VyMkBnbWFpbC5jb20ifQ==.sig")
    vault.add_or_update_account("user2@gmail.com", cred2, label="User 2")
    vault.set_active_account("user2@gmail.com")
    assert vault.get_active_account() == "user2@gmail.com"

    # Listing accounts returns sorted emails
    assert vault.list_accounts() == ["user1@gmail.com", "user2@gmail.com"]

    # Removing account
    assert vault.remove_account("user2@gmail.com") is True
    assert vault.get_active_account() == "user1@gmail.com"
    assert vault.remove_account("nonexistent") is False


def test_keyring_service_switch_and_listener(isolated_env, mock_keyring, temp_dir):
    """Verify KeyringService full switch flow, listener dispatch, and token preservation."""
    vault_file = Path(temp_dir) / "vault" / "accounts.json"
    vault = AccountVault(config_path=vault_file)
    backend = SecretToolBackend()

    service = KeyringService(backend=backend, vault=vault)

    # Initial auto-ingest of primary account from keyring
    accounts = service.list_accounts()
    assert len(accounts) >= 1
    primary_email = accounts[0]

    # Add target standby account
    target_email = "target@gmail.com"
    target_cred = KeyringCredential("ya29.target_token", "1//target_refresh", id_token="ey.eyJlbWFpbCI6ICJ0YXJnZXRAZ21haWwuY29tIn0=.sig")
    vault.add_or_update_account(target_email, target_cred, label="Target Standby")

    # Track listener notifications
    notifications = []
    service.register_switch_listener(lambda email, reason: notifications.append((email, reason)))

    # Execute switch
    result = service.switch_account(target_email, reason="unit_test")
    assert result is True
    assert len(notifications) == 1
    assert notifications[0] == (target_email, "unit_test")

    # Verify active credential in Secret Service is updated
    active_cred = service.get_active_credential()
    assert active_cred.access_token == "ya29.target_token"
    assert vault.get_active_account() == target_email

    # Non-existent account switch raises AccountNotFoundError
    with pytest.raises(AccountNotFoundError):
        service.switch_account("ghost@gmail.com")


def test_account_store_facade(temp_dir):
    """Verify AccountStore facade returns proper dict formatting."""
    vault_file = Path(temp_dir) / "store" / "accounts.json"
    store = AccountStore(accounts_file=vault_file)

    cred = KeyringCredential("tokA", "refA")
    store.add_or_update("accA@gmail.com", cred, label="Account A")

    listed = store.list_accounts()
    assert len(listed) == 1
    assert listed[0]["email"] == "accA@gmail.com"
    assert listed[0]["is_active"] is True
    assert listed[0]["status"] == "ACTIVE"
