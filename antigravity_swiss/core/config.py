"""
Path Resolution and Configuration Manager for Antigravity Swiss Knife.
======================================================================
Resolves XDG base directories, safely bounds Unix domain socket path lengths
under Linux limits (108 bytes), and provides centralized config loading.
"""

from __future__ import annotations

import dataclasses
import hashlib
import json
import os
from pathlib import Path
import tempfile
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
    uid = os.getuid() if hasattr(os, "getuid") else 1000
    compact_dir = Path(f"/tmp/ag-{uid}-{path_hash}")
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

    def get(self, key: str, default: Any = None) -> Any:
        mapping = {
            "poll_interval_seconds": "poll_interval_sec",
            "keepalive_warmup_enabled": "warmup_enabled",
        }
        attr = mapping.get(key, key)
        return getattr(self, attr, default)

    def set(self, key: str, value: Any) -> None:
        mapping = {
            "poll_interval_seconds": "poll_interval_sec",
            "keepalive_warmup_enabled": "warmup_enabled",
        }
        attr = mapping.get(key, key)
        if hasattr(self, attr):
            setattr(self, attr, value)

    def save(self) -> None:
        self.save_settings()

    def save_settings(self) -> None:
        """Persist mutable settings to settings.json atomically."""
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
        fd, tmp_path_str = tempfile.mkstemp(
            dir=self.config_dir,
            prefix=".settings.tmp.",
            text=True,
        )
        tmp_path = Path(tmp_path_str)
        try:
            os.fchmod(fd, 0o600)
            with os.fdopen(fd, "w", encoding="utf-8") as f:
                json.dump(payload, f, indent=2)
                f.flush()
                os.fsync(f.fileno())
            os.replace(str(tmp_path), str(self.settings_file))
        except Exception:
            if tmp_path.exists():
                try:
                    tmp_path.unlink()
                except OSError:
                    pass
            raise

    @classmethod
    def load(
        cls,
        custom_config_dir: Path | str | None = None,
        custom_socket_path: Path | str | None = None,
    ) -> SwissKnifeConfig:
        """Load configuration with precedence: custom arguments > env vars > settings.json > defaults."""
        # 1. Config Dir Resolution
        if custom_config_dir:
            config_dir = Path(custom_config_dir).expanduser().resolve()
        elif os.environ.get(ENV_CONFIG_DIR):
            config_dir = Path(os.environ[ENV_CONFIG_DIR]).expanduser().resolve()
        else:
            config_dir = get_xdg_config_home() / APP_NAME

        # 2. Socket Path Resolution
        if custom_socket_path:
            socket_path = Path(custom_socket_path).expanduser().resolve()
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
                if "client_timeout_sec" in data:
                    instance.client_timeout_sec = float(data["client_timeout_sec"])
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
