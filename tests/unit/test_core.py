"""
Unit tests for antigravity_swiss.core: config, constants, and errors.
"""

import os
from pathlib import Path
import pytest

from antigravity_swiss.core.config import (
    SwissKnifeConfig,
    get_xdg_config_home,
    get_xdg_data_home,
    get_xdg_runtime_dir,
    resolve_safe_socket_path,
)
from antigravity_swiss.core.constants import (
    APP_NAME,
    APP_TITLE,
    JSONRPC_VERSION,
    MAX_SOCKET_PATH_LEN,
    MD3_ACCENT_PRIMARY,
    MD3_SURFACE,
)
from antigravity_swiss.core.errors import (
    AccountNotFoundError,
    CredentialNotFoundError,
    DaemonAlreadyRunningError,
    DaemonNotRunningError,
    InternalRPCError,
    InvalidParamsError,
    InvalidRequestError,
    KeyringBackendUnavailableError,
    KeyringError,
    MethodNotFoundError,
    ParseError,
    ProcessError,
    ProcessTerminationTimeoutError,
    SwissKnifeError,
)


def test_constants_definitions():
    """Verify core constants and M3 design tokens."""
    assert APP_NAME == "antigravity-swiss"
    assert "Antigravity Swiss Knife" in APP_TITLE
    assert JSONRPC_VERSION == "2.0"
    assert MD3_SURFACE == "#131314"
    assert MD3_ACCENT_PRIMARY == "#8ab4f8"
    assert MAX_SOCKET_PATH_LEN < 108


def test_errors_hierarchy_and_rpc_mapping():
    """Verify exception hierarchy and JSON-RPC 2.0 error mapping."""
    err = SwissKnifeError("Test error", code=-32000, data={"detail": "sample"})
    rpc_dict = err.to_rpc_error()
    assert rpc_dict["code"] == -32000
    assert rpc_dict["message"] == "Test error"
    assert rpc_dict["data"] == {"detail": "sample"}

    parse_err = ParseError()
    assert parse_err.to_rpc_error()["code"] == -32700

    inv_req = InvalidRequestError()
    assert inv_req.to_rpc_error()["code"] == -32600

    method_err = MethodNotFoundError("nonexistent.method")
    assert method_err.to_rpc_error()["code"] == -32601
    assert "nonexistent.method" in method_err.to_rpc_error()["message"]

    params_err = InvalidParamsError("missing arg")
    assert params_err.to_rpc_error()["code"] == -32602

    internal_err = InternalRPCError()
    assert internal_err.to_rpc_error()["code"] == -32603

    daemon_err = DaemonNotRunningError()
    assert daemon_err.to_rpc_error()["code"] == -32002

    already_running = DaemonAlreadyRunningError()
    assert already_running.to_rpc_error()["code"] == -32003

    acct_err = AccountNotFoundError("test@example.com")
    assert acct_err.to_rpc_error()["code"] == -32013
    assert "test@example.com" in acct_err.to_rpc_error()["message"]

    proc_timeout = ProcessTerminationTimeoutError(1234, 10.0)
    assert proc_timeout.to_rpc_error()["code"] == -32021
    assert "1234" in proc_timeout.to_rpc_error()["message"]


def test_xdg_resolution_defaults(monkeypatch, temp_dir):
    """Test XDG directory resolution with environment overrides."""
    fake_config = os.path.join(temp_dir, "custom_config")
    fake_runtime = os.path.join(temp_dir, "custom_run")
    fake_data = os.path.join(temp_dir, "custom_data")
    os.makedirs(fake_runtime, mode=0o700, exist_ok=True)

    monkeypatch.setenv("XDG_CONFIG_HOME", fake_config)
    monkeypatch.setenv("XDG_RUNTIME_DIR", fake_runtime)
    monkeypatch.setenv("XDG_DATA_HOME", fake_data)

    assert get_xdg_config_home() == Path(fake_config).resolve()
    assert get_xdg_runtime_dir() == Path(fake_runtime).resolve()
    assert get_xdg_data_home() == Path(fake_data).resolve()


def test_safe_socket_path_length_bounding():
    """Verify that socket paths exceeding safe length are hashed into compact paths."""
    short_dir = Path("/tmp/short")
    short_path = resolve_safe_socket_path(short_dir)
    assert len(str(short_path)) <= MAX_SOCKET_PATH_LEN
    assert "daemon.sock" in short_path.name

    long_dir = Path("/tmp/" + "a" * 120 + "/deep/sub/dir")
    long_path = resolve_safe_socket_path(long_dir)
    assert len(str(long_path)) <= MAX_SOCKET_PATH_LEN
    assert str(long_path).startswith("/tmp/ag-")


def test_config_load_and_save_settings(temp_dir):
    """Test loading configuration and persisting settings."""
    cfg_dir = Path(temp_dir) / "cfg"
    sock_path = Path(temp_dir) / "test.sock"

    cfg = SwissKnifeConfig.load(custom_config_dir=cfg_dir, custom_socket_path=sock_path)
    assert cfg.config_dir == cfg_dir.resolve()
    assert cfg.socket_path == sock_path.resolve()
    assert cfg.accounts_file == (cfg_dir / "accounts.json").resolve()

    cfg.ensure_directories()
    assert cfg_dir.exists()
    assert (cfg_dir.stat().st_mode & 0o777) == 0o700

    cfg.auto_switch_threshold = 0.12
    cfg.poll_interval_sec = 45.0
    cfg.save_settings()

    assert cfg.settings_file.exists()
    # Reload to verify persistence
    reloaded = SwissKnifeConfig.load(custom_config_dir=cfg_dir, custom_socket_path=sock_path)
    assert reloaded.auto_switch_threshold == 0.12
    assert reloaded.poll_interval_sec == 45.0
