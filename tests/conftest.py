"""
Global pytest configuration and hermetic fixtures for Antigravity Swiss Knife test suite.
Ensures zero live network calls and zero modification to host system files or user keyrings.
"""

import os
import shutil
import sys
import tempfile
import pytest

# Absolute Safety Shield: Flag testing mode globally
os.environ["ANTIGRAVITY_SWISS_TESTING"] = "1"

# Protect host Antigravity 2.0 from accidental SIGTERM/SIGKILL in tests
_orig_os_kill = os.kill

def _shielded_os_kill(pid: int, sig: int):
    try:
        cmdline_path = f"/proc/{pid}/cmdline"
        if os.path.exists(cmdline_path):
            with open(cmdline_path, "rb") as f:
                cmdline = f.read().decode("utf-8", errors="ignore")
            cmdline_lower = cmdline.lower()
            if (
                "/opt/antigravity" in cmdline_lower
                or "antigravity-manager" in cmdline_lower
                or "/usr/lib/antigravity" in cmdline_lower
                or "language_server" in cmdline_lower
                or ".config/antigravity" in cmdline_lower
            ):
                import logging
                logging.getLogger("conftest").critical(
                    "SAFETY SHIELD: Blocked attempt to kill host Antigravity (PID %d, signal %s)!",
                    pid, sig
                )
                return None
    except Exception:
        pass
    return _orig_os_kill(pid, sig)

os.kill = _shielded_os_kill

# Ensure project root and tests directory are in sys.path
PROJECT_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
if PROJECT_ROOT not in sys.path:
    sys.path.insert(0, PROJECT_ROOT)

from tests.fixtures.mock_keyring import MockKeyringBackend, create_mock_secret_tool_script
from tests.fixtures.mock_antigravity_fs import MockAntigravityFs
from tests.fixtures.mock_cloudcode_server import MockCloudCodeServer
from tests.fixtures.mock_process import MockProcessManager
from tests.fixtures.test_helpers import CredentialBuilder


@pytest.fixture
def temp_dir():
    """Provides a fresh isolated temporary directory per test."""
    d = tempfile.mkdtemp(prefix="swiss_test_")
    yield d
    try:
        shutil.rmtree(d, ignore_errors=True)
    except Exception:
        pass


@pytest.fixture
def isolated_env(temp_dir, monkeypatch):
    """
    Sets up hermetic environment variables:
    - Custom PATH with mock secret-tool
    - HOME redirected to temp_dir/home
    - XDG_CONFIG_HOME redirected to temp_dir/home/.config
    - XDG_RUNTIME_DIR redirected to temp_dir/run
    """
    bin_dir = os.path.join(temp_dir, "bin")
    home_dir = os.path.join(temp_dir, "home")
    runtime_dir = os.path.join(temp_dir, "run")
    keyring_state = os.path.join(temp_dir, "mock_keyring_state.json")

    os.makedirs(bin_dir, exist_ok=True)
    os.makedirs(home_dir, exist_ok=True)
    os.makedirs(runtime_dir, mode=0o700, exist_ok=True)

    # Deploy mock secret-tool script in bin_dir
    create_mock_secret_tool_script(bin_dir, keyring_state)

    # Monkeypatch environment
    orig_path = os.environ.get("PATH", "")
    monkeypatch.setenv("PATH", f"{bin_dir}:{orig_path}")
    monkeypatch.setenv("HOME", home_dir)
    monkeypatch.setenv("XDG_CONFIG_HOME", os.path.join(home_dir, ".config"))
    monkeypatch.setenv("XDG_RUNTIME_DIR", runtime_dir)

    return {
        "root": temp_dir,
        "bin": bin_dir,
        "home": home_dir,
        "runtime": runtime_dir,
        "keyring_state": keyring_state
    }


@pytest.fixture
def mock_keyring(isolated_env):
    """Provides access to the mock keyring backend and seeds default credentials."""
    backend = MockKeyringBackend()
    default_cred = CredentialBuilder.build_valid_payload("user-primary@gmail.com")
    backend.store("gemini", "antigravity", default_cred)

    # Sync with mock secret-tool file
    import json
    with open(isolated_env["keyring_state"], "w") as f:
        json.dump({"gemini:antigravity": default_cred}, f)

    return backend


@pytest.fixture
def mock_fs(isolated_env):
    """Provides a pre-configured Antigravity 2.0 filesystem structure."""
    fs = MockAntigravityFs(isolated_env["root"])
    fs.setup()
    return fs


@pytest.fixture
def mock_cloudcode():
    """Provides a running in-process mock server emulating cloudcode-pa.googleapis.com."""
    server = MockCloudCodeServer()
    base_url = server.start()
    yield server
    server.stop()


@pytest.fixture
def mock_proc(mock_fs):
    """Provides simulated Antigravity process and lock manager."""
    proc = MockProcessManager(mock_fs.config_antigravity_dir)
    yield proc
    proc.cleanup()
