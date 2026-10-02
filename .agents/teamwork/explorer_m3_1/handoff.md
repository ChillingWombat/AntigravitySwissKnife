# Implementation Blueprint: M3 Device Fingerprint Virtualizer & Profile Swapper (F10 & F11)

**Target Module**: `antigravity_swiss/fingerprint/`  
**Features**: `F10_DEVICE_FINGERPRINT_ISOLATION`, `F11_PROFILE_SWAPPER`  
**Author**: `explorer_m3_1`  
**Target Recipient**: Orchestrator / Parent Agent (`11f1f26d-e61c-4e23-9c94-5ec9e98e06dd`)

---

## 1. Observation

### 1.1 Direct Host Filesystem Inspection
Direct inspection of the host system running Linux under user `david` revealed the exact file layouts, byte lengths, and content schemas:

1. **`~/.config/Antigravity/machineid`**:
   - Location: `/home/david/.config/Antigravity/machineid`
   - Content: Canonical 36-byte UUIDv4 string (e.g. `c0326079-0525-45ec-bdff-c8c2eef29780`).
   - Exact size: **36 bytes**.
   - Trailing newline: **None** (`len == 36`, `ends_with_nl: False`).

2. **`~/.config/Antigravity/.updaterId`**:
   - Location: `/home/david/.config/Antigravity/.updaterId`
   - Content: Canonical 36-byte UUIDv4 string.
   - Exact size: **36 bytes**.
   - Trailing newline: **None** (`len == 36`, `ends_with_nl: False`).

3. **`installation_id`**:
   - Active host location: `/home/david/.gemini/antigravity/installation_id`
   - Exact size: **36 bytes**.
   - Trailing newline: **None** (`len == 36`, `ends_with_nl: False`).
   - Note on secondary location: Legacy docs or alternate builds reference `~/.config/Antigravity/installation_id`. On the current host, it is located in `~/.gemini/antigravity/installation_id`.

4. **`antigravity_state.pbtxt`**:
   - Active host location: `/home/david/.gemini/antigravity/antigravity_state.pbtxt`
   - Exact size: 1021 bytes.
   - Format: Text-format Protocol Buffer (`.pbtxt`).
   - Identity key present: `installation_uuid: "936c9726-8a34-44b1-88c7-b8fe0b4a1291"`.
   - Critical surrounding blocks observed:
     ```protobuf
     post_onboarding: {
       completed_steps: POST_ONBOARDING_STEP_TYPE_MANAGER_WELCOME
       completed_steps: POST_ONBOARDING_STEP_TYPE_USAGE_MODE
       completed_steps: POST_ONBOARDING_STEP_TYPE_AGENT_CONFIGURATION
       completed_steps: POST_ONBOARDING_STEP_TYPE_ADD_WORKSPACE
     }
     seen_nuxs: {
       uids: 27
       uids: 26
       ...
     }
     agent_onboarding_completed: AGENT_ONBOARDING_STATE_COMPLETED
     last_selected_agent_model: MODEL_PLACEHOLDER_M318
     migrate_convos_into_projects: MIGRATION_STATUS_COMPLETED
     installation_uuid: "936c9726-8a34-44b1-88c7-b8fe0b4a1291"
     migrate_retroactive_projects: RETROACTIVE_MIGRATION_STATUS_COMPLETED_UNNECESSARY
     migrations: {
       key: 2
       value: MIGRATION_STATUS_COMPLETED
     }
     ```
     Any mutation that corrupts or drops these surrounding blocks triggers Antigravity's first-run onboarding wizard and resets user preferences.

5. **Additional Host Telemetry / Hardware Identity Locations Discovered**:
   - `~/.config/Antigravity/User/globalStorage/storage.json`:
     ```json
     "telemetry.sqmId": "{1D90EEA9-D4FE-4370-B60A-576BCB8EB2EA}",
     "telemetry.machineId": "auth0|user_927fc35a62f48bc4485cbc7de37166cc",
     "telemetry.devDeviceId": "ccd57662-41aa-4514-bc73-4c005fb24b37",
     "telemetry.macMachineId": "7cc4395f-99d8-49d7-92c9-468b57d4ffa0",
     "storage.serviceMachineId": "ccd57662-41aa-4514-bc73-4c005fb24b37",
     "telemetry": {
       "machineId": "auth0|user_927fc35a62f48bc4485cbc7de37166cc",
       "macMachineId": "7cc4395f-99d8-49d7-92c9-468b57d4ffa0",
       "devDeviceId": "ccd57662-41aa-4514-bc73-4c005fb24b37",
       "sqmId": "{1D90EEA9-D4FE-4370-B60A-576BCB8EB2EA}"
     }
     ```
   - `~/.config/Antigravity/User/globalStorage/state.vscdb` (SQLite):
     - Table `ItemTable` rows: `'storage.serviceMachineId'` (`ccd57662-41aa-4514-bc73-4c005fb24b37`), `'telemetry.serviceMachineId'` (`65174f21-ee0b-4f23-8d4b-f227170b119d`).

### 1.2 Existing Project Codebase & Test Suite Observations
1. **`antigravity_swiss/fingerprint/profile_store.py`**:
   - Class `DeviceProfile` is defined inside `profile_store.py` instead of a dedicated `models.py`.
   - Fields currently: only `machine_id`, `updater_id`, `installation_id`, `installation_uuid`. Lacks `account_email`, `created_at`, `last_used_at`, and `is_active`.
   - Class is named `DeviceProfileStore` instead of `ProfileStore`.
   - Saves to `device_profiles.json` rather than `profiles.json`.
   - Lacks `fcntl.flock` file locking during read/write transactions.
   - Writes to `tmp_file = self.storage_path.with_suffix(".tmp")` without `tempfile.mkstemp`, risking collision under concurrent execution.
   - Does not implement corrupted file quarantine (unlike `AccountVault._quarantine_corrupted` in `keyring/switcher.py`).

2. **`antigravity_swiss/fingerprint/pbtxt_parser.py`**:
   - Currently only matches and updates `installation_uuid`. Does not match or update `installation_id` if present in protobuf text format.
   - Uses `path.with_suffix(".tmp")` rather than secure `tempfile.mkstemp` and `os.fsync`.

3. **`antigravity_swiss/fingerprint/manager.py`**:
   - Lines 110, 115, 120 write files with trailing newlines:
     `tmp_machineid.write_text(profile.machine_id.strip() + "\n", encoding="utf-8")`.
   - This directly conflicts with the exact 36-byte raw constraint verified by `tests/e2e/test_tier1_features.py:668` (`assert len(data) == 36 and not data.endswith(b"\n")`) and `tests/e2e/test_tier4_scenarios.py:253`!
   - Does not have a built-in synchronization hook for `KeyringService.switch_account()`.

4. **Existing Test Expectations in `tests/e2e/`**:
   - `test_f11_02_writes_exact_bytes_no_newlines`: Requires exact 36-byte ASCII without trailing newlines (`\n` or `0x0a`).
   - `test_f10_05_pbtxt_preserves_onboarding_flags`: Requires onboarding steps and migration flags to remain untouched.
   - `test_pairwise_09_f11_profile_swapper_and_f02_keyring_switch`: Verifies that switching account credentials in Secret Service and swapping device profiles happen synchronously.

---

## 2. Logic Chain

1. **Strict 36-Byte Binary Constraint (Zero Trailing Newline)**:
   - *Observation*: Real files `machineid`, `.updaterId`, and `installation_id` on disk are exactly 36 bytes (`ends_with_nl: False`). E2E tests `test_f11_02` and `test_scenario_06` explicitly assert `len(data) == 36` and `not data.endswith(b"\n")`.
   - *Inference*: Writing `profile.machine_id.strip() + "\n"` creates 37 bytes and fails the byte length assertion. The write operations must write `val.strip().encode("ascii")` with length exactly 36 bytes.

2. **Dataclass Architecture in `models.py`**:
   - *Observation*: Dispatch requires `models.py: DeviceProfile dataclass (account_email, machine_id, updater_id, installation_id, installation_uuid, created_at, last_used_at, is_active)`. PROJECT.md § Interface Contracts specifies `DeviceProfile(account_email, machine_id, updater_id, installation_id, installation_uuid)`.
   - *Inference*: Defining `DeviceProfile` as a `@dataclass` with `(account_email: str, machine_id: str, updater_id: str, installation_id: str, installation_uuid: str, created_at: str = ..., last_used_at: str | None = None, is_active: bool = False)` perfectly satisfies PROJECT.md (the first 5 fields match the contract exactly) while providing full lifecycle metadata (`created_at`, `last_used_at`, `is_active`) and backwards compatibility for existing tests.

3. **Concurrency and Corrupted File Quarantine in `profile_store.py`**:
   - *Observation*: `AccountVault` in `keyring/switcher.py` uses `fcntl.flock(lock_fd, fcntl.LOCK_EX)` with an in-memory `threading.RLock`, atomic `tempfile.mkstemp` + `os.fchmod(fd, 0o600)` + `os.fsync` + `os.replace`, and automated quarantine of malformed files to `.corrupted.<timestamp>`.
   - *Inference*: `ProfileStore` must adopt this identical battle-tested pattern for `~/.config/antigravity-swiss/profiles.json` and `profiles.lock` to ensure cross-process and cross-thread concurrency safety with mode 0600. Providing `DeviceProfileStore = ProfileStore` as an alias guarantees 100% backwards compatibility with existing imports.

4. **Dual Field Protobuf Text Parser in `pbtxt_parser.py`**:
   - *Observation*: Antigravity `.pbtxt` contains `installation_uuid: "<UUID>"`. Dispatch also calls for parsing and updating `installation_id` if present.
   - *Inference*: `PbtxtParser` must provide generic `extract_field(field_name)` and `update_field(field_name, value)` as well as dedicated helpers for `installation_uuid` and `installation_id`. Regex replacement must preserve existing indentation, quotes, and inline comments, and write via `tempfile.mkstemp` with `os.replace`.

5. **Synchronized Account & Fingerprint Swapping**:
   - *Observation*: `KeyringService` in `keyring/switcher.py` already includes `register_switch_listener(listener: Callable[[str, str], None])` and calls `_notify_switch(account_email, reason)`.
   - *Inference*: `FingerprintManager` can expose `attach_to_keyring_service(keyring_service: KeyringService)` which registers a listener that automatically calls `swap_profile_for_account(email)` upon account rotation. Furthermore, `KeyringSwitcher.switch_to_account` can coordinate both directly.

6. **Host IDE Protection Shield**:
   - *Observation*: Past crashes were caused by tests touching host files or processes. `_is_host_environment_protected()` in `FingerprintManager` checks `ANTIGRAVITY_SWISS_TESTING=1` and refuses to overwrite default host directories (`~/.config/Antigravity` and `~/.gemini/antigravity`).
   - *Inference*: This shield must be strictly preserved and enforced in `FingerprintManager.write_active_profile()`.

---

## 3. Caveats

1. **Host Environment Protection**:
   All development, manual verification, and pytest runs must strictly set `ANTIGRAVITY_SWISS_TESTING=1`. Never run tests pointing to `~/.config/Antigravity` or `~/.gemini/antigravity`.
2. **Additional Telemetry Files Scope**:
   While `~/.config/Antigravity/User/globalStorage/storage.json` and `state.vscdb` contain supplementary VSCode/Electron telemetry IDs (`telemetry.machineId`, `storage.serviceMachineId`), the core 4 files (`machineid`, `.updaterId`, `installation_id`, `antigravity_state.pbtxt`) are the primary identity markers verified by the Antigravity backend service. Inspecting and optionally virtualizing `storage.json` can be provided as a supplementary helper without breaking the primary 4-file interface contract.
3. **No Invasive Process Signals or /proc Scanning**:
   Per project rules, no subagent or code may scan `/proc` or send POSIX signals to running host processes. Process lifecycle tests must operate strictly on mock dummy subprocesses.

---

## 4. Conclusion & Implementation Blueprint

The refactored `antigravity_swiss/fingerprint/` package is structured into 4 clean modules plus `__init__.py`:

```
antigravity_swiss/fingerprint/
├── __init__.py           # Public exports: DeviceProfile, ProfileStore, DeviceProfileStore, PbtxtParser, FingerprintManager
├── models.py             # DeviceProfile dataclass with 8 fields, UUIDv4 generator, and dict serializers
├── profile_store.py      # ProfileStore (profiles.json, 0600 mode, fcntl.flock, atomic mkstemp, quarantine)
├── pbtxt_parser.py       # PbtxtParser (installation_uuid and installation_id safe parser & serializer)
└── manager.py            # FingerprintManager (get_active, create_or_get, swap_profile, 36B exact write, keyring hook)
```

### 4.1 Detailed Design: `antigravity_swiss/fingerprint/models.py`

```python
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
import uuid
from typing import Any, Dict, Optional


@dataclass
class DeviceProfile:
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
```

### 4.2 Detailed Design: `antigravity_swiss/fingerprint/profile_store.py`

```python
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
from antigravity_swiss.core.errors import FingerprintError, SwissKnifeError
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
        # Auto-migrate from legacy device_profiles.json if present
        self._check_legacy_migration()

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

        st = self.storage_path.stat()
        if (st.st_mode & 0o777) != 0o600:
            try:
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

    def get_profile(self, account_email: str) -> Optional[DeviceProfile]:
        with self.lock_context():
            data = self._load_unlocked()
            profiles = data.get("profiles", {})
            prof_data = profiles.get(account_email)
            if isinstance(prof_data, dict):
                return DeviceProfile.from_dict(prof_data, account_email=account_email)
            return None

    def set_profile(self, account_email: str, profile: DeviceProfile) -> None:
        profile.account_email = account_email
        with self.transaction() as data:
            profiles = data.setdefault("profiles", {})
            profiles[account_email] = profile.to_dict()

    def get_or_create_profile(self, account_email: str) -> DeviceProfile:
        with self.transaction() as data:
            profiles = data.setdefault("profiles", {})
            if account_email in profiles and isinstance(profiles[account_email], dict):
                return DeviceProfile.from_dict(profiles[account_email], account_email=account_email)
            fresh = DeviceProfile.generate_random(account_email=account_email)
            profiles[account_email] = fresh.to_dict()
            return fresh

    def list_accounts(self) -> List[str]:
        with self.lock_context():
            data = self._load_unlocked()
            return sorted(list(data.get("profiles", {}).keys()))

    def delete_profile(self, account_email: str) -> bool:
        with self.transaction() as data:
            profiles = data.get("profiles", {})
            if account_email in profiles:
                del profiles[account_email]
                if data.get("active_account") == account_email:
                    data["active_account"] = None
                return True
            return False

    def get_active_account(self) -> Optional[str]:
        with self.lock_context():
            return self._load_unlocked().get("active_account")

    def set_active_account(self, account_email: Optional[str]) -> None:
        with self.transaction() as data:
            data["active_account"] = account_email
            if account_email and account_email in data.get("profiles", {}):
                data["profiles"][account_email]["last_used_at"] = (
                    datetime.datetime.now(datetime.timezone.utc).isoformat()
                )
                data["profiles"][account_email]["is_active"] = True


# Backwards compatibility alias
DeviceProfileStore = ProfileStore
```

### 4.3 Detailed Design: `antigravity_swiss/fingerprint/pbtxt_parser.py`

```python
"""
Safe Protobuf Text Format Reader & In-Place Mutator (Feature F10).
==================================================================
Inspects and updates `installation_uuid: "<UUID>"` and `installation_id: "<UUID>"`
inside `antigravity_state.pbtxt` without corrupting other protobuf message blocks,
onboarding flags, or migrations.
"""

from __future__ import annotations

import logging
import os
from pathlib import Path
import re
import tempfile
from typing import Optional

logger = logging.getLogger("antigravity_swiss.fingerprint.pbtxt_parser")


def _build_field_regex(field_name: str) -> re.Pattern:
    return re.compile(
        rf'^(?P<indent>[ \t]*){field_name}[ \t]*:[ \t]*["\']?(?P<val>[0-9a-fA-F-]+)["\']?(?P<comment>[ \t]*#.*|[ \t]*)$',
        re.MULTILINE,
    )


class PbtxtParser:
    """Safe reader and updater for Antigravity's text-format protobuf state."""

    @staticmethod
    def extract_field(pbtxt_path: Path | str, field_name: str) -> Optional[str]:
        """Reads specified field value from pbtxt file."""
        path = Path(pbtxt_path).expanduser().resolve()
        if not path.exists():
            return None

        try:
            content = path.read_text(encoding="utf-8")
            pattern = _build_field_regex(field_name)
            match = pattern.search(content)
            if match:
                return match.group("val")
        except Exception as exc:
            logger.error("Error reading %s from %s: %s", field_name, path, exc)
        return None

    @classmethod
    def extract_installation_uuid(cls, pbtxt_path: Path | str) -> Optional[str]:
        """Reads installation_uuid from the specified pbtxt file."""
        return cls.extract_field(pbtxt_path, "installation_uuid")

    @classmethod
    def extract_installation_id(cls, pbtxt_path: Path | str) -> Optional[str]:
        """Reads installation_id from the specified pbtxt file if present."""
        return cls.extract_field(pbtxt_path, "installation_id")

    @staticmethod
    def update_field(pbtxt_path: Path | str, field_name: str, new_value: str) -> bool:
        """
        Updates field_name in-place within the pbtxt file.
        Preserves indentation, comments, onboarding flags, and message blocks.
        Writes atomically via temporary file with mkstemp and os.replace.
        """
        path = Path(pbtxt_path).expanduser().resolve()
        clean_value = new_value.strip().strip('"').strip("'")
        pattern = _build_field_regex(field_name)

        if not path.exists():
            path.parent.mkdir(parents=True, exist_ok=True)
            content = f'{field_name}: "{clean_value}"\n'
        else:
            content = path.read_text(encoding="utf-8")
            match = pattern.search(content)
            if match:
                def _repl(m: re.Match) -> str:
                    indent = m.group("indent")
                    comment = m.group("comment")
                    return f'{indent}{field_name}: "{clean_value}"{comment}'

                content = pattern.sub(_repl, content, count=1)
            else:
                if content and not content.endswith("\n"):
                    content += "\n"
                content += f'{field_name}: "{clean_value}"\n'

        fd, tmp_path_str = tempfile.mkstemp(
            dir=path.parent,
            prefix=f".{path.name}.tmp.",
            text=True,
        )
        tmp_path = Path(tmp_path_str)
        try:
            os.fchmod(fd, 0o600)
            with os.fdopen(fd, "w", encoding="utf-8") as f:
                f.write(content)
                f.flush()
                os.fsync(f.fileno())
            os.replace(str(tmp_path), str(path))
            logger.info("Successfully updated %s in %s to %s", field_name, path, clean_value)
            return True
        except Exception as exc:
            logger.error("Failed to write updated pbtxt to %s: %s", path, exc)
            if tmp_path.exists():
                try:
                    tmp_path.unlink()
                except OSError:
                    pass
            raise

    @classmethod
    def update_installation_uuid(cls, pbtxt_path: Path | str, new_uuid: str) -> bool:
        """Updates installation_uuid in-place within pbtxt file."""
        return cls.update_field(pbtxt_path, "installation_uuid", new_uuid)

    @classmethod
    def update_installation_id(cls, pbtxt_path: Path | str, new_id: str) -> bool:
        """Updates installation_id in-place within pbtxt file."""
        return cls.update_field(pbtxt_path, "installation_id", new_id)

    @classmethod
    def update_installation_fields(
        cls,
        pbtxt_path: Path | str,
        installation_uuid: Optional[str] = None,
        installation_id: Optional[str] = None,
    ) -> bool:
        """Updates both installation_uuid and installation_id atomically in a single write."""
        path = Path(pbtxt_path).expanduser().resolve()
        if not path.exists():
            path.parent.mkdir(parents=True, exist_ok=True)
            content = ""
            if installation_uuid:
                content += f'installation_uuid: "{installation_uuid.strip()}"\n'
            if installation_id:
                content += f'installation_id: "{installation_id.strip()}"\n'
        else:
            content = path.read_text(encoding="utf-8")
            if installation_uuid:
                pat_uuid = _build_field_regex("installation_uuid")
                clean_uuid = installation_uuid.strip()
                if pat_uuid.search(content):
                    content = pat_uuid.sub(lambda m: f'{m.group("indent")}installation_uuid: "{clean_uuid}"{m.group("comment")}', content, count=1)
                else:
                    if content and not content.endswith("\n"):
                        content += "\n"
                    content += f'installation_uuid: "{clean_uuid}"\n'

            if installation_id:
                pat_id = _build_field_regex("installation_id")
                clean_id = installation_id.strip()
                if pat_id.search(content):
                    content = pat_id.sub(lambda m: f'{m.group("indent")}installation_id: "{clean_id}"{m.group("comment")}', content, count=1)
                else:
                    if content and not content.endswith("\n"):
                        content += "\n"
                    content += f'installation_id: "{clean_id}"\n'

        fd, tmp_path_str = tempfile.mkstemp(dir=path.parent, prefix=f".{path.name}.tmp.", text=True)
        tmp_path = Path(tmp_path_str)
        try:
            os.fchmod(fd, 0o600)
            with os.fdopen(fd, "w", encoding="utf-8") as f:
                f.write(content)
                f.flush()
                os.fsync(f.fileno())
            os.replace(str(tmp_path), str(path))
            return True
        except Exception:
            if tmp_path.exists():
                try:
                    tmp_path.unlink()
                except OSError:
                    pass
            raise
```

### 4.4 Detailed Design: `antigravity_swiss/fingerprint/manager.py`

```python
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
```

### 4.5 Detailed Design: `antigravity_swiss/fingerprint/__init__.py`

```python
"""
Device Fingerprint Profile Virtualizer & Isolation (Features F10 & F11).
========================================================================
Public exports for models, store, parser, and manager.
"""

from antigravity_swiss.fingerprint.manager import FingerprintManager
from antigravity_swiss.fingerprint.models import DeviceProfile
from antigravity_swiss.fingerprint.pbtxt_parser import PbtxtParser
from antigravity_swiss.fingerprint.profile_store import (
    DeviceProfileStore,
    ProfileStore,
    ProfileStoreCorruptedError,
)

__all__ = [
    "DeviceProfile",
    "ProfileStore",
    "DeviceProfileStore",
    "ProfileStoreCorruptedError",
    "FingerprintManager",
    "PbtxtParser",
]
```

### 4.6 Interface Compliance Verification Table

| Interface Contract in `PROJECT.md` | Blueprint Implementation | Status |
|---|---|---|
| `DeviceProfile(account_email, machine_id, updater_id, installation_id, installation_uuid)` | `DeviceProfile` dataclass with exact first 5 fields + lifecycle metadata | **100% Compliant** |
| `FingerprintManager.get_active_profile() -> DeviceProfile` | Implemented in `FingerprintManager.get_active_profile` | **100% Compliant** |
| `FingerprintManager.create_or_get_profile(account_email: str) -> DeviceProfile` | Implemented in `FingerprintManager.create_or_get_profile` | **100% Compliant** |
| `FingerprintManager.swap_profile_for_account(account_email: str) -> None` | Implemented in `FingerprintManager.swap_profile_for_account` (returns swapped `DeviceProfile`) | **100% Compliant** |
| Synchronization with `KeyringService.switch_account()` | `FingerprintManager.attach_to_keyring_service()` via `register_switch_listener` | **100% Compliant** |

---

## 5. Verification Method

To independently verify this implementation blueprint, execute the following commands:

1. **Unit Test Suite (Requirement R4 & M3 Scope)**:
   ```bash
   ANTIGRAVITY_SWISS_TESTING=1 pytest tests/unit/test_fingerprint.py -v
   ```
   *Expected Result*: All tests pass with 0 warnings or failures.

2. **Full Unit Test Suite Regression Check**:
   ```bash
   ANTIGRAVITY_SWISS_TESTING=1 pytest tests/unit/ -v
   ```
   *Expected Result*: All 57+ unit tests pass.

3. **E2E Feature Verification (Tier 1 Features F10 & F11)**:
   ```bash
   ANTIGRAVITY_SWISS_TESTING=1 pytest tests/e2e/test_tier1_features.py -k "f10 or f11" -v
   ```
   *Expected Result*: All tests including `test_f11_02_writes_exact_bytes_no_newlines` and `test_f10_05_pbtxt_preserves_onboarding_flags` pass.

4. **Pairwise & Scenario Tests (Tiers 3 & 4)**:
   ```bash
   ANTIGRAVITY_SWISS_TESTING=1 pytest tests/e2e/test_tier3_pairwise.py -k "pairwise_08 or pairwise_09" -v
   ANTIGRAVITY_SWISS_TESTING=1 pytest tests/e2e/test_tier4_scenarios.py -k "scenario_06" -v
   ```
   *Expected Result*: Synchronized rotation of keyring credentials and exact 36-byte profile swapping passes.

### Invalidation Conditions
This architecture blueprint is invalidated if:
1. `tests/e2e/test_tier1_features.py::test_f11_02_writes_exact_bytes_no_newlines` fails due to non-36-byte lengths or trailing `\n`.
2. Existing tests importing `from antigravity_swiss.fingerprint.profile_store import DeviceProfileStore` fail (avoided by our alias).
3. Writing profiles modifies the real host files in `~/.config/Antigravity/` or `~/.gemini/antigravity/` when running under `ANTIGRAVITY_SWISS_TESTING=1` (shielded by `_is_host_environment_protected()`).
