# Investigation Report & Implementation Blueprint: M1 Daemon Core & IPC (`F25_DAEMON_IPC_CORE`)

**Explorer**: `explorer_m1_3`  
**Milestone**: M1 Core Daemon, IPC & CLI Entry Point  
**Date**: 2026-10-01T07:55:00Z  
**Target Files**:
- `antigravity_swiss/core/constants.py`
- `antigravity_swiss/core/config.py`
- `antigravity_swiss/core/errors.py`
- `antigravity_swiss/ipc/socket_server.py`
- `antigravity_swiss/ipc/socket_client.py`
- `antigravity_swiss/ipc/controller.py`
- `antigravity_swiss/__main__.py`

---

## 1. Observation

Direct system inspection, file analyses, and environment evidence:

1. **System Python Environment**:
   - Host interpreter: `/usr/bin/python3` (Python 3.14.4 on Linux kernel 6.18).
   - Standard libraries available: `asyncio`, `socket`, `dataclasses`, `json`, `pathlib`, `argparse`, `signal`, `sqlite3`, `typing`, `urllib.request`.
   - External libraries: `PySide6` is not installed globally in `/usr/bin/python3`; `pydantic` and `cryptography` are available.
   - **Critical architectural deduction**: The entire core daemon, IPC socket protocol, configuration engine, domain errors, and CLI can and must be built with **zero external third-party dependencies**, relying strictly on standard Python libraries. This guarantees 100% portable, instant execution for background daemon services and CLI commands.

2. **XDG Paths & Runtime Environment** (from `spec_miner_env_1` and system inspection):
   - Current user UID: `1000`.
   - Default `$XDG_RUNTIME_DIR`: `/run/user/1000`.
   - Dedicated socket directory: `/run/user/1000/antigravity-swiss/`.
   - Target Unix Domain Socket: `/run/user/1000/antigravity-swiss/daemon.sock`.
   - User config home: `~/.config/antigravity-swiss/` (defaults to `$XDG_CONFIG_HOME/antigravity-swiss/`).
   - Host Antigravity config directory: `~/.config/Antigravity/` (holding `SingletonLock`, `app_storage.json`, `machineid`).
   - Host Antigravity data directory: `~/.gemini/antigravity/` (holding `installation_id`, `antigravity_state.pbtxt`, `conversations/`, `conversation_summaries.db`).

3. **IPC Requirements & Interface Contracts** (`PROJECT.md` § Interface Contracts §4):
   - Transport: Unix Domain Socket at `$XDG_RUNTIME_DIR/antigravity-swiss/daemon.sock` (mode `0600`).
   - Protocol: JSON-RPC 2.0 / NDJSON (Newline Delimited JSON, UTF-8 encoded).
   - Methods:
     * `status.get() -> DaemonStatus`
     * `quota.get_summary() -> QuotaSummary`
     * `accounts.list() -> list[AccountItem]`
     * `accounts.switch(email, force, relaunch) -> SwitchResult`
     * `vault.totp_generate(secret) -> TotpCodeResult`
     * `cache.get_breakdown() -> CacheBreakdown`
     * `cache.prune(options) -> PruneResult`
     * `daemon.shutdown() -> dict`
   - Events (Pub-Sub Notifications):
     * `notify.quota_updated { summary }`
     * `notify.account_switched { email, reason }`
     * `notify.warmup_triggered { reset_time, next_window }`

4. **CLI Entry Point Requirements** (`PROJECT.md` § Code Layout & Dispatch):
   - Entry point: `python -m antigravity_swiss <subcommand>`
   - Required subcommands:
     * `daemon`: Runs background daemon process.
     * `status`: Queries active daemon status and displays human-readable summary or JSON.
     * `switch <email>`: Triggers account switch.
     * `gui`: Launches desktop GUI (with fallback diagnostic if PySide6 is missing).

---

## 2. Logic Chain

1. **Path Resolution & Fallback Resiliency**:
   - On standard desktop Linux, `XDG_RUNTIME_DIR` points to `/run/user/<uid>` (a `tmpfs` ramdisk mounted with mode 0700).
   - If running inside a container, SSH session without `systemd-logind`, or non-standard environment where `XDG_RUNTIME_DIR` is unset, falling back to `/tmp/antigravity-swiss-<uid>` with explicit `os.chmod(..., 0o700)` ensures the daemon never crashes on path resolution.
   - For hermetic testing, `ANTIGRAVITY_SWISS_SOCKET_PATH` and `ANTIGRAVITY_SWISS_CONFIG_DIR` environment variables must override default paths, allowing tests to run isolated daemon instances in temp folders without collisions.

2. **Socket Security & Permissions**:
   - Unix domain sockets inherit directory permissions, but setting `0o600` on the socket file itself prevents any other unprivileged user on the host system from eavesdropping or sending unauthorized account switch commands.
   - `os.chmod(socket_path, 0o600)` must occur immediately after socket bind.

3. **Stale Socket Handling & Race Prevention**:
   - If the system or daemon previously crashed without unlinking `daemon.sock`, attempting to bind immediately throws `OSError: [Errno 98] Address already in use`.
   - Simply unlinking `daemon.sock` unconditionally is hazardous: it could hijack an existing live daemon's socket.
   - The robust algorithm:
     1. Test connection with `client.connect()`.
     2. If connection succeeds: raise `DaemonAlreadyRunningError` (exit code 1).
     3. If connection fails (`ConnectionRefusedError`): the file is dead; safely `unlink()` and proceed to bind.

4. **JSON-RPC 2.0 / NDJSON Protocol Engine**:
   - Each frame is terminated with `\n` (`0x0a`).
   - Standard JSON-RPC 2.0 format:
     * Requests include `"id"`.
     * Notifications omit `"id"` (or set to `None`), requiring no reply.
     * Errors return standard codes: Parse error (`-32700`), Invalid Request (`-32600`), Method not found (`-32601`), Invalid params (`-32602`), Internal error (`-32603`), plus custom domain codes (`-32000` to `-32099`).
   - Memory safety: Set `asyncio.start_unix_server(limit=10*1024*1024)` to cap maximum message size to 10MB, preventing memory exhaustion attacks from malformed streams.

5. **Multi-Client Pub-Sub Broadcaster**:
   - Both the desktop GUI (PySide6) and CLI watchers can be connected simultaneously.
   - The server maintains a `set[asyncio.StreamWriter]`.
   - When broadcasting (`broadcast_event`), the server writes to all writers and drains.
   - If a client abruptly terminates (`BrokenPipeError`, `ConnectionResetError`), the server catches the error, discards the dead writer from the active set, and continues broadcasting to surviving clients without crashing.

6. **Unified Controller Pattern (Remote Daemon vs In-Process Standalone)**:
   - When users run `python -m antigravity_swiss status` or `switch <email>`, a daemon may or may not be running.
   - If the daemon is active, operations are dispatched via Unix Domain Socket to the daemon (Remote mode).
   - If the daemon is inactive, the CLI should not simply fail; instead, the `SwissKnifeController` transparently falls back to `StandaloneController`, executing keyring and process operations locally in-process.

---

## 3. Caveats

1. **Linux Socket Path Length Limit**:
   - On Linux, `sockaddr_un.sun_path` has a maximum length of 108 bytes (`UNIX_PATH_MAX`).
   - If unit tests create deeply nested temporary directories (e.g. `/tmp/pytest-of-david/pytest-1234/test_feature_very_long_name/daemon.sock`), path length can exceed 108 bytes and raise `OSError: AF_UNIX path too long`.
   - Config path resolution must validate `len(str(socket_path)) < 108` and fall back to `/tmp/ag-<hash>.sock` if exceeded.
2. **PySide6 Optionality**:
   - `antigravity_swiss` must install and function cleanly without PySide6. The GUI module (`antigravity_swiss.gui`) is imported lazily only when `python -m antigravity_swiss gui` is invoked.
3. **Signal Trapping in Asyncio**:
   - `loop.add_signal_handler(signal.SIGTERM, ...)` works on Linux/macOS main thread. If run in a non-main thread, it raises `ValueError`. The daemon runner must ensure signal handlers are attached to the main thread event loop.

---

## 4. Conclusion & Implementation Blueprint

Here is the exact code blueprint for all 7 target modules in scope.

### 4.1 Module: `antigravity_swiss/core/constants.py`

```python
"""Antigravity Swiss Knife core constants and defaults."""

from pathlib import Path
import sys

# Application Metadata
APP_NAME = "antigravity-swiss"
APP_TITLE = "Antigravity Swiss Knife"
APP_VERSION = "0.1.0"
APP_AUTHOR = "ChillingWombat"

# Linux Secret Service / Keyring Constants
KEYRING_SERVICE_NAME = "gemini"
KEYRING_USERNAME = "antigravity"
KEYRING_SCHEMA_NAME = "org.freedesktop.Secret.Generic"
KEYRING_LABEL = "Password for 'antigravity' on 'gemini'"

# Default Host Antigravity Paths
DEFAULT_ANTIGRAVITY_BIN = Path("/opt/Antigravity/antigravity")
DEFAULT_ANTIGRAVITY_CONFIG_DIR = Path.home() / ".config" / "Antigravity"
DEFAULT_ANTIGRAVITY_DATA_DIR = Path.home() / ".gemini" / "antigravity"

# Antigravity File Names
SINGLETON_LOCK_NAME = "SingletonLock"
SINGLETON_SOCKET_NAME = "SingletonSocket"
SINGLETON_COOKIE_NAME = "SingletonCookie"
APP_STORAGE_JSON_NAME = "app_storage.json"
CONVERSATION_SUMMARIES_DB_NAME = "conversation_summaries.db"
STATE_VSCDB_NAME = "state.vscdb"
STATE_VSCDB_WAL_NAME = "state.vscdb-wal"

# Fingerprint File Names
FINGERPRINT_MACHINE_ID = "machineid"
FINGERPRINT_UPDATER_ID = ".updaterId"
FINGERPRINT_INSTALLATION_ID = "installation_id"
FINGERPRINT_PBTXT = "antigravity_state.pbtxt"

# Environment Variable Overrides
ENV_CONFIG_DIR = "ANTIGRAVITY_SWISS_CONFIG_DIR"
ENV_SOCKET_PATH = "ANTIGRAVITY_SWISS_SOCKET_PATH"
ENV_ACCOUNTS_FILE = "ANTIGRAVITY_SWISS_ACCOUNTS_FILE"
ENV_SETTINGS_FILE = "ANTIGRAVITY_SWISS_SETTINGS_FILE"
ENV_POLL_INTERVAL = "ANTIGRAVITY_SWISS_POLL_INTERVAL"
ENV_SWITCH_THRESHOLD = "ANTIGRAVITY_SWISS_SWITCH_THRESHOLD"
ENV_AUTO_SWITCH_ENABLED = "ANTIGRAVITY_SWISS_AUTO_SWITCH"
ENV_WARMUP_ENABLED = "ANTIGRAVITY_SWISS_WARMUP_ENABLED"
ENV_ANTIGRAVITY_BIN = "ANTIGRAVITY_BIN_PATH"
ENV_ANTIGRAVITY_CONFIG = "ANTIGRAVITY_CONFIG_DIR"
ENV_ANTIGRAVITY_DATA = "ANTIGRAVITY_DATA_DIR"

# IPC Socket & Wire Specs
SOCKET_DIR_NAME = "antigravity-swiss"
SOCKET_FILE_NAME = "daemon.sock"
SOCKET_DIR_MODE = 0o700
SOCKET_FILE_MODE = 0o600
MAX_SOCKET_PATH_LEN = 104  # Safe margin under Linux 108 limit
MAX_FRAME_SIZE = 10 * 1024 * 1024  # 10 MB limit for JSON-RPC messages
JSONRPC_VERSION = "2.0"
WIRE_DELIMITER = b"\n"

# Default Tuning & Operation Parameters
DEFAULT_POLLING_INTERVAL_SECONDS = 60.0
DEFAULT_AUTO_SWITCH_THRESHOLD_FRACTION = 0.05  # 5% quota
DEFAULT_WARMUP_LEAD_TIME_SECONDS = 2.0
DEFAULT_PROCESS_TERMINATE_TIMEOUT_SECONDS = 10.0
DEFAULT_CLIENT_TIMEOUT_SECONDS = 15.0
DEFAULT_WARMUP_MODEL_ID = "gemini-3.8-flash-high"
DEFAULT_MAX_TOKENS_WARMUP = 1

# Google Upstream Endpoints & OAuth
GOOGLE_OAUTH_TOKEN_URL = "https://oauth2.googleapis.com/token"
GOOGLE_QUOTA_SUMMARY_URL = "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary"
GOOGLE_AVAILABLE_MODELS_URL = "https://cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels"
GOOGLE_GENERATE_CONTENT_URL = "https://cloudcode-pa.googleapis.com/v1internal:generateContent"
GOOGLE_DEFAULT_CLIENT_ID = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"
GOOGLE_DEFAULT_CLIENT_SECRET = "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"
GOOGLE_USER_AGENT = "antigravity/2.18.1"

# Material Design 3 Dark Theme Tokens (Google Gemini Palette)
MD3_SURFACE = "#131314"
MD3_SURFACE_CONTAINER = "#1e1f20"
MD3_SURFACE_CONTAINER_HIGH = "#282a2c"
MD3_ACCENT_PRIMARY = "#8ab4f8"
MD3_TEXT_PRIMARY = "#e3e3e3"
MD3_TEXT_SECONDARY = "#9aa0a6"
MD3_OUTLINE = "#3c4043"
MD3_COLOR_HEALTHY = "#81c995"
MD3_COLOR_WARNING = "#fdd663"
MD3_COLOR_EXHAUSTED = "#f28b82"
MD3_RADIUS_CARD = 16
MD3_RADIUS_PILL = 18
```

---

### 4.2 Module: `antigravity_swiss/core/errors.py`

```python
"""Antigravity Swiss Knife domain exceptions and JSON-RPC error mapping."""

from typing import Any

class SwissKnifeError(Exception):
    """Base exception for all Antigravity Swiss Knife errors with JSON-RPC error mapping."""

    def __init__(self, message: str, code: int = -32000, data: Any = None):
        super().__init__(message)
        self.message = message
        self.code = code
        self.data = data

    def to_rpc_error(self) -> dict[str, Any]:
        """Convert exception to standard JSON-RPC 2.0 error object."""
        err: dict[str, Any] = {"code": self.code, "message": self.message}
        if self.data is not None:
            err["data"] = self.data
        return err

# Standard JSON-RPC 2.0 Specification Errors (-32700, -32600 to -32603)
class ParseError(SwissKnifeError):
    def __init__(self, message: str = "Invalid JSON was received by the server", data: Any = None):
        super().__init__(message, code=-32700, data=data)

class InvalidRequestError(SwissKnifeError):
    def __init__(self, message: str = "The JSON sent is not a valid Request object", data: Any = None):
        super().__init__(message, code=-32600, data=data)

class MethodNotFoundError(SwissKnifeError):
    def __init__(self, method: str, data: Any = None):
        super().__init__(f"The method '{method}' does not exist or is not available", code=-32601, data=data)

class InvalidParamsError(SwissKnifeError):
    def __init__(self, message: str = "Invalid method parameter(s)", data: Any = None):
        super().__init__(message, code=-32602, data=data)

class InternalRPCError(SwissKnifeError):
    def __init__(self, message: str = "Internal JSON-RPC error", data: Any = None):
        super().__init__(message, code=-32603, data=data)

# Domain Specific Errors (-32000 to -32099)
class IPCError(SwissKnifeError):
    """General IPC communication failure."""
    def __init__(self, message: str, code: int = -32001, data: Any = None):
        super().__init__(message, code=code, data=data)

class DaemonNotRunningError(IPCError):
    """Daemon socket is not active or unreachable."""
    def __init__(self, message: str = "Antigravity Swiss Knife daemon is not running", data: Any = None):
        super().__init__(message, code=-32002, data=data)

class DaemonAlreadyRunningError(IPCError):
    """Another daemon instance is already active and listening on the socket."""
    def __init__(self, message: str = "Antigravity Swiss Knife daemon is already running", data: Any = None):
        super().__init__(message, code=-32003, data=data)

class KeyringError(SwissKnifeError):
    """Base error for Linux Secret Service / Keyring operations."""
    def __init__(self, message: str, code: int = -32010, data: Any = None):
        super().__init__(message, code=code, data=data)

class KeyringLockedError(KeyringError):
    """Secret Service collection is locked and requires user authentication."""
    def __init__(self, message: str = "Linux Secret Service keyring collection is locked", data: Any = None):
        super().__init__(message, code=-32011, data=data)

class CredentialNotFoundError(KeyringError):
    """No matching credentials found in the Secret Service."""
    def __init__(self, message: str = "No credential found in Secret Service for service=gemini, username=antigravity", data: Any = None):
        super().__init__(message, code=-32012, data=data)

class AccountNotFoundError(KeyringError):
    """Target account email does not exist in Swiss Knife accounts vault."""
    def __init__(self, email: str, data: Any = None):
        super().__init__(f"Account '{email}' not found in registered accounts", code=-32013, data=data)

class ProcessError(SwissKnifeError):
    """Base error for Antigravity process management."""
    def __init__(self, message: str, code: int = -32020, data: Any = None):
        super().__init__(message, code=code, data=data)

class ProcessTerminationTimeoutError(ProcessError):
    """Antigravity failed to exit cleanly within the grace period."""
    def __init__(self, pid: int, timeout_sec: float, data: Any = None):
        super().__init__(f"Antigravity process (PID {pid}) did not exit within {timeout_sec}s", code=-32021, data=data)

class ProcessLaunchError(ProcessError):
    """Failed to spawn Antigravity binary."""
    def __init__(self, message: str, data: Any = None):
        super().__init__(message, code=-32022, data=data)

class StorageCorruptionError(SwissKnifeError):
    """Integrity failure in app_storage.json, SQLite WAL files, or state.vscdb."""
    def __init__(self, message: str, data: Any = None):
        super().__init__(message, code=-32023, data=data)

class QuotaError(SwissKnifeError):
    """Base error for upstream CloudCode quota operations."""
    def __init__(self, message: str, code: int = -32030, data: Any = None):
        super().__init__(message, code=code, data=data)

class QuotaAuthExpiredError(QuotaError):
    """Google OAuth refresh token expired or invalid."""
    def __init__(self, message: str = "Google OAuth refresh token expired or revoked", data: Any = None):
        super().__init__(message, code=-32031, data=data)

class QuotaNetworkError(QuotaError):
    """Network connection error communicating with Google CloudCode API."""
    def __init__(self, message: str = "Failed to connect to cloudcode-pa.googleapis.com", data: Any = None):
        super().__init__(message, code=-32032, data=data)

class FingerprintError(SwissKnifeError):
    """Error during hardware device profile generation or swapping."""
    def __init__(self, message: str, code: int = -32040, data: Any = None):
        super().__init__(message, code=code, data=data)

class CachePruneError(SwissKnifeError):
    """Error during brain or conversation cache pruning."""
    def __init__(self, message: str, code: int = -32050, data: Any = None):
        super().__init__(message, code=code, data=data)
```

---

### 4.3 Module: `antigravity_swiss/core/config.py`

```python
"""Path resolution and configuration manager for Antigravity Swiss Knife."""

from __future__ import annotations
import dataclasses
import hashlib
import json
import os
from pathlib import Path
from typing import Any

from antigravity_swiss.core.constants import (
    APP_NAME,
    DEFAULT_ANTIGRAVITY_BIN,
    DEFAULT_ANTIGRAVITY_CONFIG_DIR,
    DEFAULT_ANTIGRAVITY_DATA_DIR,
    DEFAULT_AUTO_SWITCH_THRESHOLD_FRACTION,
    DEFAULT_CLIENT_TIMEOUT_SECONDS,
    DEFAULT_POLLING_INTERVAL_SECONDS,
    DEFAULT_PROCESS_TERMINATE_TIMEOUT_SECONDS,
    DEFAULT_WARMUP_MODEL_ID,
    ENV_ACCOUNTS_FILE,
    ENV_ANTIGRAVITY_BIN,
    ENV_ANTIGRAVITY_CONFIG,
    ENV_ANTIGRAVITY_DATA,
    ENV_AUTO_SWITCH_ENABLED,
    ENV_CONFIG_DIR,
    ENV_POLL_INTERVAL,
    ENV_SETTINGS_FILE,
    ENV_SOCKET_PATH,
    ENV_SWITCH_THRESHOLD,
    ENV_WARMUP_ENABLED,
    MAX_SOCKET_PATH_LEN,
    SOCKET_DIR_MODE,
    SOCKET_DIR_NAME,
    SOCKET_FILE_MODE,
    SOCKET_FILE_NAME,
)

def get_xdg_config_home() -> Path:
    """Resolve XDG_CONFIG_HOME or fallback to ~/.config."""
    env = os.environ.get("XDG_CONFIG_HOME")
    if env and env.strip():
        return Path(env).expanduser().resolve()
    return Path.home() / ".config"

def get_xdg_runtime_dir() -> Path:
    """Resolve XDG_RUNTIME_DIR or fallback to /tmp/antigravity-swiss-<uid>."""
    env = os.environ.get("XDG_RUNTIME_DIR")
    if env and env.strip():
        runtime_path = Path(env).expanduser().resolve()
        if runtime_path.exists() and os.access(runtime_path, os.W_OK):
            return runtime_path
    # Safe fallback for containers or headless environments
    uid = os.getuid() if hasattr(os, "getuid") else 1000
    fallback = Path(f"/tmp/antigravity-swiss-{uid}")
    fallback.mkdir(parents=True, exist_ok=True)
    try:
        os.chmod(fallback, SOCKET_DIR_MODE)
    except OSError:
        pass
    return fallback

def get_xdg_data_home() -> Path:
    """Resolve XDG_DATA_HOME or fallback to ~/.local/share."""
    env = os.environ.get("XDG_DATA_HOME")
    if env and env.strip():
        return Path(env).expanduser().resolve()
    return Path.home() / ".local" / "share"

def resolve_safe_socket_path(preferred_dir: Path) -> Path:
    """Resolve a socket path guaranteeing it fits within Linux 108-byte sockaddr_un limit."""
    candidate = preferred_dir / SOCKET_DIR_NAME / SOCKET_FILE_NAME
    if len(str(candidate)) <= MAX_SOCKET_PATH_LEN:
        return candidate
    
    # Path exceeds safe limit; use compact hash in /tmp
    path_hash = hashlib.sha256(str(candidate).encode("utf-8")).hexdigest()[:8]
    compact_dir = Path(f"/tmp/ag-{path_hash}")
    compact_dir.mkdir(parents=True, exist_ok=True)
    try:
        os.chmod(compact_dir, SOCKET_DIR_MODE)
    except OSError:
        pass
    return compact_dir / SOCKET_FILE_NAME

@dataclasses.dataclass
class SwissKnifeConfig:
    """Application configuration and resolved paths."""
    config_dir: Path
    socket_path: Path
    accounts_file: Path
    settings_file: Path
    antigravity_bin: Path
    antigravity_config_dir: Path
    antigravity_data_dir: Path
    poll_interval_sec: float = DEFAULT_POLLING_INTERVAL_SECONDS
    auto_switch_threshold: float = DEFAULT_AUTO_SWITCH_THRESHOLD_FRACTION
    auto_switch_enabled: bool = True
    warmup_enabled: bool = True
    warmup_model_id: str = DEFAULT_WARMUP_MODEL_ID
    process_timeout_sec: float = DEFAULT_PROCESS_TERMINATE_TIMEOUT_SECONDS
    client_timeout_sec: float = DEFAULT_CLIENT_TIMEOUT_SECONDS

    def ensure_directories(self) -> None:
        """Create necessary directories with strict 0700 permissions."""
        self.config_dir.mkdir(parents=True, exist_ok=True)
        try:
            os.chmod(self.config_dir, SOCKET_DIR_MODE)
        except OSError:
            pass

        socket_parent = self.socket_path.parent
        socket_parent.mkdir(parents=True, exist_ok=True)
        try:
            os.chmod(socket_parent, SOCKET_DIR_MODE)
        except OSError:
            pass

    def save_settings(self) -> None:
        """Persist mutable settings to settings.json."""
        self.ensure_directories()
        payload = {
            "poll_interval_sec": self.poll_interval_sec,
            "auto_switch_threshold": self.auto_switch_threshold,
            "auto_switch_enabled": self.auto_switch_enabled,
            "warmup_enabled": self.warmup_enabled,
            "warmup_model_id": self.warmup_model_id,
            "process_timeout_sec": self.process_timeout_sec,
            "client_timeout_sec": self.client_timeout_sec,
        }
        temp_file = self.settings_file.with_suffix(".tmp")
        temp_file.write_text(json.dumps(payload, indent=2), encoding="utf-8")
        try:
            os.chmod(temp_file, 0o600)
        except OSError:
            pass
        temp_file.replace(self.settings_file)

    @classmethod
    def load(
        cls,
        custom_config_dir: Path | None = None,
        custom_socket_path: Path | None = None,
    ) -> SwissKnifeConfig:
        """Load configuration with precedence: custom arguments > env vars > settings.json > defaults."""
        # 1. Config Dir Resolution
        if custom_config_dir:
            config_dir = custom_config_dir.expanduser().resolve()
        elif os.environ.get(ENV_CONFIG_DIR):
            config_dir = Path(os.environ[ENV_CONFIG_DIR]).expanduser().resolve()
        else:
            config_dir = get_xdg_config_home() / APP_NAME

        # 2. Socket Path Resolution
        if custom_socket_path:
            socket_path = custom_socket_path.expanduser().resolve()
        elif os.environ.get(ENV_SOCKET_PATH):
            socket_path = Path(os.environ[ENV_SOCKET_PATH]).expanduser().resolve()
        else:
            socket_path = resolve_safe_socket_path(get_xdg_runtime_dir())

        # 3. File Paths
        accounts_file = (
            Path(os.environ[ENV_ACCOUNTS_FILE]).expanduser().resolve()
            if os.environ.get(ENV_ACCOUNTS_FILE)
            else config_dir / "accounts.json"
        )
        settings_file = (
            Path(os.environ[ENV_SETTINGS_FILE]).expanduser().resolve()
            if os.environ.get(ENV_SETTINGS_FILE)
            else config_dir / "settings.json"
        )

        # 4. Host Antigravity Paths
        antigravity_bin = (
            Path(os.environ[ENV_ANTIGRAVITY_BIN]).expanduser().resolve()
            if os.environ.get(ENV_ANTIGRAVITY_BIN)
            else DEFAULT_ANTIGRAVITY_BIN
        )
        antigravity_config = (
            Path(os.environ[ENV_ANTIGRAVITY_CONFIG]).expanduser().resolve()
            if os.environ.get(ENV_ANTIGRAVITY_CONFIG)
            else DEFAULT_ANTIGRAVITY_CONFIG_DIR
        )
        antigravity_data = (
            Path(os.environ[ENV_ANTIGRAVITY_DATA]).expanduser().resolve()
            if os.environ.get(ENV_ANTIGRAVITY_DATA)
            else DEFAULT_ANTIGRAVITY_DATA_DIR
        )

        instance = cls(
            config_dir=config_dir,
            socket_path=socket_path,
            accounts_file=accounts_file,
            settings_file=settings_file,
            antigravity_bin=antigravity_bin,
            antigravity_config_dir=antigravity_config,
            antigravity_data_dir=antigravity_data,
        )

        # 5. Load settings.json if present
        if settings_file.exists():
            try:
                data = json.loads(settings_file.read_text(encoding="utf-8"))
                if "poll_interval_sec" in data:
                    instance.poll_interval_sec = float(data["poll_interval_sec"])
                if "auto_switch_threshold" in data:
                    instance.auto_switch_threshold = float(data["auto_switch_threshold"])
                if "auto_switch_enabled" in data:
                    instance.auto_switch_enabled = bool(data["auto_switch_enabled"])
                if "warmup_enabled" in data:
                    instance.warmup_enabled = bool(data["warmup_enabled"])
                if "warmup_model_id" in data:
                    instance.warmup_model_id = str(data["warmup_model_id"])
                if "process_timeout_sec" in data:
                    instance.process_timeout_sec = float(data["process_timeout_sec"])
            except Exception:
                pass

        # 6. Apply Environment Variable Overrides
        if os.environ.get(ENV_POLL_INTERVAL):
            try:
                instance.poll_interval_sec = float(os.environ[ENV_POLL_INTERVAL])
            except ValueError:
                pass
        if os.environ.get(ENV_SWITCH_THRESHOLD):
            try:
                instance.auto_switch_threshold = float(os.environ[ENV_SWITCH_THRESHOLD])
            except ValueError:
                pass
        if os.environ.get(ENV_AUTO_SWITCH_ENABLED):
            instance.auto_switch_enabled = os.environ[ENV_AUTO_SWITCH_ENABLED].lower() in ("1", "true", "yes")
        if os.environ.get(ENV_WARMUP_ENABLED):
            instance.warmup_enabled = os.environ[ENV_WARMUP_ENABLED].lower() in ("1", "true", "yes")

        return instance
```

---

### 4.4 Module: `antigravity_swiss/ipc/socket_server.py`

```python
"""Asyncio Unix Domain Socket Server implementing JSON-RPC 2.0 / NDJSON and Pub-Sub Broadcasting."""

from __future__ import annotations
import asyncio
import json
import logging
import os
from pathlib import Path
import socket
import time
from typing import Any, Callable, Coroutine

from antigravity_swiss.core.constants import (
    JSONRPC_VERSION,
    MAX_FRAME_SIZE,
    SOCKET_DIR_MODE,
    SOCKET_FILE_MODE,
    WIRE_DELIMITER,
)
from antigravity_swiss.core.errors import (
    DaemonAlreadyRunningError,
    InternalRPCError,
    InvalidParamsError,
    InvalidRequestError,
    MethodNotFoundError,
    ParseError,
    SwissKnifeError,
)

logger = logging.getLogger("antigravity_swiss.ipc.server")

RPCMethod = Callable[..., Coroutine[Any, Any, Any] | Any]

class AsyncUnixSocketServer:
    """High-performance Asyncio Unix Domain Socket Server with JSON-RPC 2.0 and Pub-Sub Broadcaster."""

    def __init__(self, socket_path: Path) -> None:
        self.socket_path = socket_path.resolve()
        self._server: asyncio.AbstractServer | None = None
        self._clients: set[asyncio.StreamWriter] = set()
        self._methods: dict[str, RPCMethod] = {}
        self._running: bool = False
        self._start_time: float = 0.0
        self._loop: asyncio.AbstractEventLoop | None = None
        self._stats = {"requests_total": 0, "errors_total": 0, "events_broadcast": 0}

    @property
    def is_running(self) -> bool:
        return self._running

    @property
    def uptime_seconds(self) -> float:
        return time.time() - self._start_time if self._running else 0.0

    @property
    def client_count(self) -> int:
        return len(self._clients)

    def register(self, method_name: str, handler: RPCMethod | None = None) -> Any:
        """Register an RPC method handler. Can be used as a decorator or direct call."""
        def decorator(fn: RPCMethod) -> RPCMethod:
            self._methods[method_name] = fn
            return fn

        if handler is not None:
            return decorator(handler)
        return decorator

    async def start(self) -> None:
        """Bind socket, set permissions to 0600, and start listening."""
        if self._running:
            return

        # 1. Ensure socket parent directory exists with 0700
        socket_dir = self.socket_path.parent
        socket_dir.mkdir(parents=True, exist_ok=True)
        try:
            os.chmod(socket_dir, SOCKET_DIR_MODE)
        except OSError as e:
            logger.warning("Could not chmod socket dir: %s", e)

        # 2. Check for existing socket file (detect live daemon vs stale socket)
        if self.socket_path.exists():
            test_sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
            test_sock.settimeout(0.5)
            try:
                test_sock.connect(str(self.socket_path))
                test_sock.close()
                # Active daemon is listening!
                raise DaemonAlreadyRunningError(
                    f"Antigravity Swiss Knife daemon is already active at {self.socket_path}"
                )
            except (ConnectionRefusedError, socket.timeout, OSError):
                # Socket is stale; safely remove it
                try:
                    self.socket_path.unlink()
                    logger.info("Unlinked stale socket file at %s", self.socket_path)
                except OSError as e:
                    logger.error("Failed to remove stale socket: %s", e)
                    raise

        # 3. Start Unix Domain Server
        self._loop = asyncio.get_running_loop()
        self._server = await asyncio.start_unix_server(
            client_connected_cb=self._handle_client,
            path=str(self.socket_path),
            limit=MAX_FRAME_SIZE,
        )

        # 4. Enforce strict 0600 permissions on the socket file
        try:
            os.chmod(self.socket_path, SOCKET_FILE_MODE)
        except OSError as e:
            logger.warning("Could not chmod socket file: %s", e)

        self._running = True
        self._start_time = time.time()
        logger.info("Daemon socket server active at %s (mode 0600)", self.socket_path)

    async def stop(self) -> None:
        """Gracefully stop server, disconnect clients, and clean up socket file."""
        if not self._running:
            return
        self._running = False

        # Close all connected client writers
        for writer in list(self._clients):
            try:
                writer.close()
                await writer.wait_closed()
            except Exception:
                pass
        self._clients.clear()

        # Stop listening
        if self._server:
            self._server.close()
            await self._server.wait_closed()
            self._server = None

        # Unlink socket file
        if self.socket_path.exists():
            try:
                self.socket_path.unlink()
            except OSError:
                pass
        logger.info("Daemon socket server stopped cleanly.")

    async def broadcast_event(self, event_name: str, payload: dict[str, Any]) -> int:
        """Broadcast a JSON-RPC 2.0 notification to all connected clients."""
        notification = {
            "jsonrpc": JSONRPC_VERSION,
            "method": event_name,
            "params": payload,
        }
        msg = (json.dumps(notification, ensure_ascii=False) + "\n").encode("utf-8")
        dead_clients: list[asyncio.StreamWriter] = []
        sent = 0

        for writer in list(self._clients):
            try:
                writer.write(msg)
                await writer.drain()
                sent += 1
            except (ConnectionResetError, BrokenPipeError, OSError):
                dead_clients.append(writer)

        for writer in dead_clients:
            self._clients.discard(writer)
            try:
                writer.close()
            except Exception:
                pass

        self._stats["events_broadcast"] += 1
        return sent

    def broadcast_event_threadsafe(self, event_name: str, payload: dict[str, Any]) -> None:
        """Thread-safe event broadcast from non-async contexts or background threads."""
        if self._loop and self._loop.is_running():
            asyncio.run_coroutine_threadsafe(self.broadcast_event(event_name, payload), self._loop)

    async def _handle_client(self, reader: asyncio.StreamReader, writer: asyncio.StreamWriter) -> None:
        self._clients.add(writer)
        try:
            while self._running:
                try:
                    line = await reader.readline()
                except asyncio.LimitOverrunError:
                    err_res = ParseError("Message exceeds 10MB limit").to_rpc_error()
                    writer.write((json.dumps({"jsonrpc": JSONRPC_VERSION, "error": err_res, "id": None}) + "\n").encode("utf-8"))
                    await writer.drain()
                    break

                if not line:
                    break  # Client closed connection

                line_str = line.decode("utf-8").strip()
                if not line_str:
                    continue

                res_bytes = await self._process_raw_line(line_str)
                if res_bytes:
                    writer.write(res_bytes + WIRE_DELIMITER)
                    await writer.drain()
        except (ConnectionResetError, BrokenPipeError, asyncio.IncompleteReadError):
            pass
        except Exception as exc:
            logger.error("Client connection error: %s", exc, exc_info=True)
        finally:
            self._clients.discard(writer)
            try:
                writer.close()
                await writer.wait_closed()
            except Exception:
                pass

    async def _process_raw_line(self, line: str) -> bytes | None:
        """Parse raw line and execute batch or single JSON-RPC request."""
        try:
            parsed = json.loads(line)
        except json.JSONDecodeError as jde:
            self._stats["errors_total"] += 1
            err = ParseError(f"Parse error: {jde}").to_rpc_error()
            return json.dumps({"jsonrpc": JSONRPC_VERSION, "error": err, "id": None}).encode("utf-8")

        if isinstance(parsed, list):
            # Batch request
            if not parsed:
                err = InvalidRequestError("Batch cannot be empty").to_rpc_error()
                return json.dumps({"jsonrpc": JSONRPC_VERSION, "error": err, "id": None}).encode("utf-8")
            results = []
            for item in parsed:
                res = await self._dispatch_single(item)
                if res is not None:
                    results.append(res)
            return json.dumps(results).encode("utf-8") if results else None
        elif isinstance(parsed, dict):
            res = await self._dispatch_single(parsed)
            return json.dumps(res).encode("utf-8") if res is not None else None
        else:
            err = InvalidRequestError("Top-level JSON entity must be object or array").to_rpc_error()
            return json.dumps({"jsonrpc": JSONRPC_VERSION, "error": err, "id": None}).encode("utf-8")

    async def _dispatch_single(self, req: Any) -> dict[str, Any] | None:
        if not isinstance(req, dict):
            return {"jsonrpc": JSONRPC_VERSION, "error": InvalidRequestError().to_rpc_error(), "id": None}

        req_id = req.get("id")
        is_notification = ("id" not in req) or (req_id is None and not ("method" in req and "id" in req))

        if req.get("jsonrpc") != JSONRPC_VERSION:
            if is_notification:
                return None
            return {"jsonrpc": JSONRPC_VERSION, "error": InvalidRequestError("jsonrpc must be '2.0'").to_rpc_error(), "id": req_id}

        method = req.get("method")
        if not isinstance(method, str):
            if is_notification:
                return None
            return {"jsonrpc": JSONRPC_VERSION, "error": InvalidRequestError("method must be string").to_rpc_error(), "id": req_id}

        if method not in self._methods:
            if is_notification:
                return None
            return {"jsonrpc": JSONRPC_VERSION, "error": MethodNotFoundError(method).to_rpc_error(), "id": req_id}

        handler = self._methods[method]
        params = req.get("params")
        self._stats["requests_total"] += 1

        try:
            if params is None:
                ret = handler()
            elif isinstance(params, dict):
                ret = handler(**params)
            elif isinstance(params, list):
                ret = handler(*params)
            else:
                if is_notification:
                    return None
                return {"jsonrpc": JSONRPC_VERSION, "error": InvalidParamsError().to_rpc_error(), "id": req_id}

            if asyncio.iscoroutine(ret):
                ret = await ret

            if is_notification:
                return None
            return {"jsonrpc": JSONRPC_VERSION, "result": ret, "id": req_id}
        except TypeError as te:
            self._stats["errors_total"] += 1
            if is_notification:
                return None
            return {"jsonrpc": JSONRPC_VERSION, "error": InvalidParamsError(str(te)).to_rpc_error(), "id": req_id}
        except SwissKnifeError as ske:
            self._stats["errors_total"] += 1
            if is_notification:
                return None
            return {"jsonrpc": JSONRPC_VERSION, "error": ske.to_rpc_error(), "id": req_id}
        except Exception as exc:
            self._stats["errors_total"] += 1
            logger.exception("Internal error executing %s", method)
            if is_notification:
                return None
            return {"jsonrpc": JSONRPC_VERSION, "error": InternalRPCError(str(exc)).to_rpc_error(), "id": req_id}
```

---

### 4.5 Module: `antigravity_swiss/ipc/socket_client.py`

```python
"""Async and Sync Unix Domain Socket clients for JSON-RPC 2.0 communication with the daemon."""

from __future__ import annotations
import asyncio
import collections
import json
import logging
from pathlib import Path
import socket
import time
from typing import Any, Callable

from antigravity_swiss.core.constants import (
    DEFAULT_CLIENT_TIMEOUT_SECONDS,
    JSONRPC_VERSION,
    WIRE_DELIMITER,
)
from antigravity_swiss.core.errors import (
    DaemonNotRunningError,
    IPCError,
    SwissKnifeError,
)

logger = logging.getLogger("antigravity_swiss.ipc.client")

EventHandler = Callable[[dict[str, Any]], Any]

class AsyncDaemonClient:
    """Asynchronous client with auto-reconnect and pub-sub notification dispatch."""

    def __init__(self, socket_path: Path, timeout: float = DEFAULT_CLIENT_TIMEOUT_SECONDS) -> None:
        self.socket_path = socket_path.resolve()
        self.timeout = timeout
        self._reader: asyncio.StreamReader | None = None
        self._writer: asyncio.StreamWriter | None = None
        self._pending: dict[int | str, asyncio.Future[Any]] = {}
        self._event_handlers: dict[str, list[EventHandler]] = collections.defaultdict(list)
        self._read_task: asyncio.Task[None] | None = None
        self._req_counter = 0
        self._connected = False
        self._lock = asyncio.Lock()

    @property
    def is_connected(self) -> bool:
        return self._connected

    async def connect(self) -> None:
        """Connect to the daemon socket and launch read loop."""
        async with self._lock:
            if self._connected:
                return
            if not self.socket_path.exists():
                raise DaemonNotRunningError(f"Daemon socket not found at {self.socket_path}")

            try:
                self._reader, self._writer = await asyncio.open_unix_connection(str(self.socket_path))
                self._connected = True
                self._read_task = asyncio.create_task(self._read_loop())
                logger.debug("Connected to daemon at %s", self.socket_path)
            except (ConnectionRefusedError, FileNotFoundError, OSError) as exc:
                self._connected = False
                raise DaemonNotRunningError(f"Failed to connect to daemon socket at {self.socket_path}: {exc}") from exc

    async def close(self) -> None:
        """Disconnect cleanly and cancel pending requests."""
        async with self._lock:
            self._connected = False
            if self._read_task:
                self._read_task.cancel()
                try:
                    await self._read_task
                except asyncio.CancelledError:
                    pass
                self._read_task = None

            if self._writer:
                try:
                    self._writer.close()
                    await self._writer.wait_closed()
                except Exception:
                    pass
                self._writer = None
            self._reader = None

            # Cancel pending requests
            for fut in self._pending.values():
                if not fut.done():
                    fut.set_exception(IPCError("Connection closed"))
            self._pending.clear()

    async def call(self, method: str, params: dict[str, Any] | None = None, timeout: float | None = None) -> Any:
        """Send a JSON-RPC request and await response."""
        timeout = timeout or self.timeout
        if not self._connected:
            await self.connect()

        self._req_counter += 1
        req_id = self._req_counter
        fut: asyncio.Future[Any] = asyncio.get_running_loop().create_future()
        self._pending[req_id] = fut

        body: dict[str, Any] = {
            "jsonrpc": JSONRPC_VERSION,
            "method": method,
            "id": req_id,
        }
        if params is not None:
            body["params"] = params

        data = (json.dumps(body) + "\n").encode("utf-8")
        try:
            if not self._writer:
                raise DaemonNotRunningError("Socket writer unavailable")
            self._writer.write(data)
            await self._writer.drain()
            return await asyncio.wait_for(fut, timeout=timeout)
        except asyncio.TimeoutError:
            self._pending.pop(req_id, None)
            raise IPCError(f"RPC method '{method}' timed out after {timeout}s")
        except Exception:
            self._pending.pop(req_id, None)
            raise

    def on(self, event_name: str, handler: EventHandler) -> None:
        """Subscribe to a pub-sub broadcast event."""
        self._event_handlers[event_name].append(handler)

    async def _read_loop(self) -> None:
        try:
            while self._connected and self._reader:
                line = await self._reader.readline()
                if not line:
                    break
                line_str = line.decode("utf-8").strip()
                if not line_str:
                    continue

                try:
                    msg = json.loads(line_str)
                except json.JSONDecodeError:
                    continue

                if isinstance(msg, dict):
                    req_id = msg.get("id")
                    if req_id is not None and req_id in self._pending:
                        fut = self._pending.pop(req_id)
                        if not fut.done():
                            if "error" in msg:
                                err = msg["error"]
                                fut.set_exception(SwissKnifeError(
                                    message=err.get("message", "RPC Error"),
                                    code=err.get("code", -32000),
                                    data=err.get("data"),
                                ))
                            else:
                                fut.set_result(msg.get("result"))
                    elif "method" in msg and req_id is None:
                        # Event notification
                        method = msg["method"]
                        params = msg.get("params", {})
                        for h in self._event_handlers.get(method, []):
                            try:
                                res = h(params)
                                if asyncio.iscoroutine(res):
                                    asyncio.create_task(res)
                            except Exception as ex:
                                logger.error("Event handler error on %s: %s", method, ex)
        except asyncio.CancelledError:
            pass
        finally:
            self._connected = False


class SyncDaemonClient:
    """Synchronous socket client for one-off CLI commands."""

    def __init__(self, socket_path: Path, timeout: float = DEFAULT_CLIENT_TIMEOUT_SECONDS) -> None:
        self.socket_path = socket_path.resolve()
        self.timeout = timeout

    def is_daemon_alive(self) -> bool:
        """Quickly check if daemon socket accepts connections."""
        if not self.socket_path.exists():
            return False
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        sock.settimeout(0.5)
        try:
            sock.connect(str(self.socket_path))
            sock.close()
            return True
        except (ConnectionRefusedError, socket.timeout, OSError):
            return False

    def call(self, method: str, params: dict[str, Any] | None = None) -> Any:
        """Execute a blocking JSON-RPC call."""
        if not self.socket_path.exists():
            raise DaemonNotRunningError(f"Daemon socket does not exist at {self.socket_path}")

        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        sock.settimeout(self.timeout)
        try:
            sock.connect(str(self.socket_path))
            body: dict[str, Any] = {
                "jsonrpc": JSONRPC_VERSION,
                "method": method,
                "id": 1,
            }
            if params is not None:
                body["params"] = params

            sock.sendall((json.dumps(body) + "\n").encode("utf-8"))

            buf = bytearray()
            while True:
                chunk = sock.recv(4096)
                if not chunk:
                    raise IPCError("Connection closed before response received")
                buf.extend(chunk)
                if b"\n" in buf:
                    break

            line = buf.split(b"\n", 1)[0].decode("utf-8")
            data = json.loads(line)

            if "error" in data:
                err = data["error"]
                raise SwissKnifeError(
                    message=err.get("message", "RPC Error"),
                    code=err.get("code", -32000),
                    data=err.get("data"),
                )
            return data.get("result")
        except socket.timeout:
            raise IPCError(f"RPC call '{method}' timed out after {self.timeout}s")
        except ConnectionRefusedError as cre:
            raise DaemonNotRunningError(f"Daemon refused connection at {self.socket_path}") from cre
        finally:
            try:
                sock.close()
            except Exception:
                pass
```

---

### 4.6 Module: `antigravity_swiss/ipc/controller.py`

```python
"""Controller abstraction providing unified interface for remote daemon and standalone in-process modes."""

from __future__ import annotations
from abc import ABC, abstractmethod
import logging
from pathlib import Path
from typing import Any

from antigravity_swiss.core.config import SwissKnifeConfig
from antigravity_swiss.core.errors import DaemonNotRunningError
from antigravity_swiss.ipc.socket_client import SyncDaemonClient

logger = logging.getLogger("antigravity_swiss.ipc.controller")

class SwissKnifeController(ABC):
    """Abstract facade for Swiss Knife operations."""

    @abstractmethod
    def is_daemon_running(self) -> bool:
        """Check if background daemon is active."""
        ...

    @abstractmethod
    def get_status(self) -> dict[str, Any]:
        """Fetch daemon, Antigravity process, and active account status."""
        ...

    @abstractmethod
    def list_accounts(self) -> list[dict[str, Any]]:
        """List registered accounts in the vault."""
        ...

    @abstractmethod
    def switch_account(self, email: str, force: bool = False, relaunch: bool = True) -> dict[str, Any]:
        """Switch active account and preserve conversation session."""
        ...

    @abstractmethod
    def get_quota_summary(self) -> dict[str, Any]:
        """Retrieve quota summary for active account."""
        ...


class RemoteDaemonController(SwissKnifeController):
    """Dispatches calls over Unix Domain Socket to running daemon."""

    def __init__(self, socket_path: Path, timeout: float = 15.0) -> None:
        self.socket_path = socket_path
        self._client = SyncDaemonClient(socket_path, timeout=timeout)

    def is_daemon_running(self) -> bool:
        return self._client.is_daemon_alive()

    def get_status(self) -> dict[str, Any]:
        return self._client.call("status.get")

    def list_accounts(self) -> list[dict[str, Any]]:
        return self._client.call("accounts.list")

    def switch_account(self, email: str, force: bool = False, relaunch: bool = True) -> dict[str, Any]:
        return self._client.call("accounts.switch", {"email": email, "force": force, "relaunch": relaunch})

    def get_quota_summary(self) -> dict[str, Any]:
        return self._client.call("quota.get_summary")


class StandaloneController(SwissKnifeController):
    """In-process fallback when daemon is not running (standalone CLI/GUI mode)."""

    def __init__(self, config: SwissKnifeConfig) -> None:
        self.config = config

    def is_daemon_running(self) -> bool:
        return False

    def get_status(self) -> dict[str, Any]:
        # Local inspect without daemon
        from antigravity_swiss.process.lifecycle import ProcessManager
        from antigravity_swiss.keyring.secret_tool import SecretToolWrapper

        pm = ProcessManager(self.config)
        pid = pm.get_running_antigravity_pid()

        keyring_active = None
        try:
            stw = SecretToolWrapper()
            cred = stw.lookup_credential()
            if cred:
                keyring_active = "Authenticated (secret-tool)"
        except Exception:
            pass

        return {
            "daemon_running": False,
            "mode": "standalone_in_process",
            "antigravity_running": pid is not None,
            "antigravity_pid": pid,
            "active_account": keyring_active,
        }

    def list_accounts(self) -> list[dict[str, Any]]:
        from antigravity_swiss.keyring.switcher import AccountStore
        store = AccountStore(self.config.accounts_file)
        return store.list_accounts()

    def switch_account(self, email: str, force: bool = False, relaunch: bool = True) -> dict[str, Any]:
        from antigravity_swiss.keyring.switcher import KeyringSwitcher
        from antigravity_swiss.process.lifecycle import ProcessManager

        switcher = KeyringSwitcher(self.config)
        pm = ProcessManager(self.config)

        res = switcher.switch_to_account(email, force=force)
        relaunch_pid = None
        if relaunch:
            pm.terminate_gracefully(self.config.process_timeout_sec)
            relaunch_pid = pm.relaunch()

        return {
            "success": True,
            "active_account": email,
            "relaunch_pid": relaunch_pid,
            "preserved_cascade_id": res.get("cascade_id"),
        }

    def get_quota_summary(self) -> dict[str, Any]:
        from antigravity_swiss.quota.poller import QuotaPoller
        poller = QuotaPoller(self.config)
        import asyncio
        return asyncio.run(poller.poll_active_account_summary())


def create_controller(
    config: SwissKnifeConfig | None = None,
    prefer_daemon: bool = True,
    force_remote: bool = False,
) -> SwissKnifeController:
    """Factory creating RemoteDaemonController if daemon is active, or StandaloneController as fallback."""
    cfg = config or SwissKnifeConfig.load()
    remote = RemoteDaemonController(cfg.socket_path, timeout=cfg.client_timeout_sec)

    if remote.is_daemon_running():
        return remote

    if force_remote:
        raise DaemonNotRunningError(f"Daemon is not active at {cfg.socket_path}")

    logger.info("Daemon not detected; using in-process StandaloneController")
    return StandaloneController(cfg)
```

---

### 4.7 Module: `antigravity_swiss/__main__.py`

```python
"""Antigravity Swiss Knife unified CLI and background daemon entry point."""

from __future__ import annotations
import argparse
import asyncio
import json
import logging
import os
from pathlib import Path
import signal
import sys
from typing import Any

from antigravity_swiss.core.config import SwissKnifeConfig
from antigravity_swiss.core.constants import APP_TITLE, APP_VERSION
from antigravity_swiss.core.errors import DaemonAlreadyRunningError, DaemonNotRunningError, SwissKnifeError
from antigravity_swiss.ipc.controller import create_controller
from antigravity_swiss.ipc.socket_server import AsyncUnixSocketServer

def setup_logging(verbose: bool = False) -> None:
    level = logging.DEBUG if verbose else logging.INFO
    logging.basicConfig(
        level=level,
        format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
        datefmt="%H:%M:%S",
    )

def run_daemon(args: argparse.Namespace) -> int:
    """Run the background daemon process."""
    setup_logging(args.verbose)
    logger = logging.getLogger("antigravity_swiss.daemon")
    config = SwissKnifeConfig.load(
        custom_config_dir=Path(args.config) if args.config else None,
        custom_socket_path=Path(args.socket) if args.socket else None,
    )

    if args.poll_interval:
        config.poll_interval_sec = float(args.poll_interval)
    if args.threshold:
        config.auto_switch_threshold = float(args.threshold)
    if args.no_warmup:
        config.warmup_enabled = False

    config.ensure_directories()
    server = AsyncUnixSocketServer(config.socket_path)

    # Wire core RPC methods
    @server.register("status.get")
    def rpc_status() -> dict[str, Any]:
        from antigravity_swiss.process.lifecycle import ProcessManager
        pm = ProcessManager(config)
        pid = pm.get_running_antigravity_pid()
        return {
            "daemon_running": True,
            "pid": os.getpid(),
            "uptime_seconds": round(server.uptime_seconds, 1),
            "client_connections": server.client_count,
            "antigravity_running": pid is not None,
            "antigravity_pid": pid,
            "auto_switch_enabled": config.auto_switch_enabled,
            "auto_switch_threshold": config.auto_switch_threshold,
            "warmup_enabled": config.warmup_enabled,
        }

    @server.register("accounts.list")
    def rpc_accounts_list() -> list[dict[str, Any]]:
        from antigravity_swiss.keyring.switcher import AccountStore
        store = AccountStore(config.accounts_file)
        return store.list_accounts()

    @server.register("accounts.switch")
    def rpc_accounts_switch(email: str, force: bool = False, relaunch: bool = True) -> dict[str, Any]:
        from antigravity_swiss.keyring.switcher import KeyringSwitcher
        from antigravity_swiss.process.lifecycle import ProcessManager

        switcher = KeyringSwitcher(config)
        pm = ProcessManager(config)

        res = switcher.switch_to_account(email, force=force)
        relaunch_pid = None
        if relaunch:
            pm.terminate_gracefully(config.process_timeout_sec)
            relaunch_pid = pm.relaunch()

        result_payload = {
            "success": True,
            "active_account": email,
            "relaunch_pid": relaunch_pid,
            "preserved_cascade_id": res.get("cascade_id"),
        }
        server.broadcast_event_threadsafe("notify.account_switched", {"email": email, "reason": "manual_rpc"})
        return result_payload

    @server.register("quota.get_summary")
    async def rpc_quota_summary() -> dict[str, Any]:
        from antigravity_swiss.quota.poller import QuotaPoller
        poller = QuotaPoller(config)
        return await poller.poll_active_account_summary()

    @server.register("daemon.shutdown")
    async def rpc_shutdown() -> dict[str, Any]:
        asyncio.create_task(server.stop())
        return {"status": "shutting_down"}

    async def main_async() -> None:
        stop_event = asyncio.Event()

        def _sig_handler() -> None:
            logger.info("Signal received, stopping daemon...")
            stop_event.set()

        loop = asyncio.get_running_loop()
        for sig in (signal.SIGTERM, signal.SIGINT):
            loop.add_signal_handler(sig, _sig_handler)

        try:
            await server.start()
            print(f"[{APP_TITLE}] Daemon listening at {config.socket_path} (PID {os.getpid()})")
            await stop_event.wait()
        except DaemonAlreadyRunningError as e:
            logger.error("%s", e)
            sys.exit(1)
        finally:
            await server.stop()

    try:
        asyncio.run(main_async())
        return 0
    except (KeyboardInterrupt, SystemExit):
        return 0
    except Exception as exc:
        logger.error("Daemon crashed: %s", exc, exc_info=True)
        return 1

def run_status(args: argparse.Namespace) -> int:
    """Query daemon or local Antigravity status and display summary."""
    config = SwissKnifeConfig.load(
        custom_socket_path=Path(args.socket) if args.socket else None
    )
    controller = create_controller(config, prefer_daemon=True)

    try:
        status_data = controller.get_status()
    except Exception as exc:
        print(f"[ERROR] Failed to query status: {exc}", file=sys.stderr)
        return 1

    if args.json:
        print(json.dumps(status_data, indent=2))
        return 0

    daemon_active = status_data.get("daemon_running", False)
    daemon_label = f"● ACTIVE (PID {status_data.get('pid')}, Uptime {status_data.get('uptime_seconds')}s)" if daemon_active else "○ INACTIVE (standalone fallback)"
    ag_running = status_data.get("antigravity_running", False)
    ag_label = f"● RUNNING (PID {status_data.get('antigravity_pid')})" if ag_running else "○ NOT RUNNING"

    print("═" * 66)
    print(f"               {APP_TITLE} (v{APP_VERSION})")
    print("═" * 66)
    print(f"Daemon Status       : {daemon_label}")
    print(f"Socket Path         : {config.socket_path}")
    print(f"Antigravity App     : {ag_label}")
    print(f"Active Account      : {status_data.get('active_account') or 'Unknown'}")
    print("─" * 66)
    print("To view live quota gauges, launch the GUI: python -m antigravity_swiss gui")
    print("═" * 66)
    return 0

def run_switch(args: argparse.Namespace) -> int:
    """Switch active Google account."""
    config = SwissKnifeConfig.load()
    controller = create_controller(config, prefer_daemon=True)

    target_email = args.email
    print(f"[*] Initiating switch to account: {target_email}...")
    try:
        res = controller.switch_account(
            email=target_email,
            force=args.force,
            relaunch=not args.no_relaunch,
        )
        print(f"[SUCCESS] Switched active account to: {res.get('active_account')}")
        if res.get("relaunch_pid"):
            print(f"[+] Antigravity relaunched with clean session (PID {res.get('relaunch_pid')})")
        return 0
    except SwissKnifeError as ske:
        print(f"[ERROR] Account switch failed: {ske.message}", file=sys.stderr)
        return 1
    except Exception as exc:
        print(f"[ERROR] Unexpected error during switch: {exc}", file=sys.stderr)
        return 1

def run_gui(args: argparse.Namespace) -> int:
    """Launch Material Design 3 Desktop GUI."""
    try:
        import PySide6
    except ImportError:
        print("[ERROR] PySide6 desktop GUI libraries are not installed in this Python environment.", file=sys.stderr)
        print("To install GUI support: pip install PySide6", file=sys.stderr)
        print("You can manage accounts, quotas, and daemon services using the CLI:", file=sys.stderr)
        print("  python -m antigravity_swiss daemon", file=sys.stderr)
        print("  python -m antigravity_swiss status", file=sys.stderr)
        print("  python -m antigravity_swiss switch <email>", file=sys.stderr)
        return 1

    from antigravity_swiss.gui.app import run_app
    return run_app(standalone=args.standalone)

def main() -> int:
    parser = argparse.ArgumentParser(
        prog="python -m antigravity_swiss",
        description=f"{APP_TITLE} (v{APP_VERSION}): Native desktop companion and daemon for Google Antigravity 2.0",
    )
    parser.add_argument("-v", "--verbose", action="store_true", help="Enable verbose debug logging")
    subparsers = parser.add_subparsers(dest="command", required=True, help="Subcommand to execute")

    # daemon
    p_daemon = subparsers.add_parser("daemon", help="Run background daemon process")
    p_daemon.add_argument("--socket", type=str, help="Custom socket path")
    p_daemon.add_argument("--config", type=str, help="Custom configuration directory")
    p_daemon.add_argument("--poll-interval", type=float, help="Quota polling interval in seconds")
    p_daemon.add_argument("--threshold", type=float, help="Auto-switch threshold fraction (e.g. 0.05)")
    p_daemon.add_argument("--no-warmup", action="store_true", help="Disable keep-alive warmup engine")
    p_daemon.set_defaults(func=run_daemon)

    # status
    p_status = subparsers.add_parser("status", help="Query status and active account")
    p_status.add_argument("--json", action="store_true", help="Output raw JSON")
    p_status.add_argument("--socket", type=str, help="Custom socket path")
    p_status.set_defaults(func=run_status)

    # switch
    p_switch = subparsers.add_parser("switch", help="Switch active Google account")
    p_switch.add_argument("email", type=str, help="Target account email")
    p_switch.add_argument("--force", action="store_true", help="Force switch regardless of quota")
    p_switch.add_argument("--no-relaunch", action="store_true", help="Do not relaunch Antigravity")
    p_switch.set_defaults(func=run_switch)

    # gui
    p_gui = subparsers.add_parser("gui", help="Launch Material Design 3 Desktop GUI")
    p_gui.add_argument("--standalone", action="store_true", help="Run in standalone mode without daemon")
    p_gui.set_defaults(func=run_gui)

    args = parser.parse_args()
    return args.func(args)

if __name__ == "__main__":
    sys.exit(main())
```

---

## 5. Verification Method

To independently verify the designs and validate the implementation:

1. **Verify Path Resolution & Permissions**:
   ```bash
   python3 -c "
   from antigravity_swiss.core.config import SwissKnifeConfig, get_xdg_runtime_dir
   cfg = SwissKnifeConfig.load()
   print('Socket path:', cfg.socket_path)
   print('Config dir:', cfg.config_dir)
   assert 'antigravity-swiss' in str(cfg.socket_path)
   "
   ```
2. **Verify JSON-RPC 2.0 Socket Server & Client**:
   Create a hermetic test using a temporary socket in `/tmp/test-ag.sock`:
   ```bash
   pytest tests/e2e/test_tier1_features.py -k "test_ipc or test_f25" -v
   ```
   Specific verification assertions:
   - Server binds socket and sets `stat(socket_path).st_mode & 0o777 == 0o600`.
   - Single request `{"jsonrpc": "2.0", "method": "status.get", "id": 1}` returns `{"jsonrpc": "2.0", "result": {...}, "id": 1}`.
   - Unknown method returns `{"code": -32601, "message": "The method '...' does not exist..."}`.
   - Malformed line returns ParseError (`code: -32700`).
   - Starting a second server while the first is active raises `DaemonAlreadyRunningError`.
   - Killing the daemon and restarting cleanly detects the stale socket, unlinks it, and binds successfully.
   - Pub-sub notifications emit to multiple client connections simultaneously.
3. **Verify CLI Invocation**:
   ```bash
   python3 -m antigravity_swiss status --json
   # Output should parse as valid JSON with daemon_running and antigravity_running fields.
   ```
4. **Invalidation Conditions**:
   - If socket permissions are not `0600`, security invariant is broken.
   - If daemon hangs on shutdown without unlinking `.sock` and subsequent launch fails with `Address already in use`, stale cleanup logic is invalid.
   - If JSON-RPC error response format deviates from `{jsonrpc: "2.0", error: {code, message}, id}`.
