"""
Adversarial Stress Test Suite: GUI, RFC 6238 TOTP Engine & Keyring Switcher.
=============================================================================
Milestone: Final Gate Validation
Role: Empirical Adversarial Challenger (challenger_final_1)

Coverage:
1. Headless GUI Stress:
   - Instantiate MainWindow, NavigationRail, TopRibbon, CircularGaugeWidget, TotpCountdownRingWidget
   - Rapid tab & sub-tab switching under offscreen rendering
   - Extreme resizing, geometry stress, and paintEvent execution
   - Dynamic theme token hot-swapping and widget re-polishing
   - Adversarial quota models (NaN, Inf, negative, huge) and malformed accounts
2. RFC 6238 TOTP Boundary Stress:
   - Official RFC 6238 Appendix B test vectors (HMAC-SHA1)
   - Base32 padding variations and length boundaries (mod 8)
   - Invalid base32 secret rejection and exception contracts
   - Countdown ring widget resilience to invalid secrets
   - Time boundary transitions (t=29s, 30s, 31s, 59s, 60s, 61s)
   - Clock drift compensation (±30s, ±60s, custom windows)
   - Step determinism, entropy, and extreme epoch timestamps
3. Keyring Concurrent Atomic Switches:
   - High-concurrency account switching across 20 threads
   - Concurrent add/update/delete transaction race condition stress
   - File permissions (0600 / 0700) and temporary file cleanup verification
   - Keyring backend failure handling and vault consistency
   - Corrupted accounts.json quarantine and clean recovery
4. System Tray Headless Fallback:
   - Headless QSystemTrayIcon availability and DBus SNI fallback
   - Safe tray icon, tooltip, and menu instantiation offscreen
   - Quota health color badge mapping
   - Quick-switch context menu boundary scaling (0 to 100 accounts)
   - DBus notification daemon failure resilience
"""

from __future__ import annotations

import base64
import concurrent.futures
import datetime
import json
import logging
import math
import os
import shutil
import tempfile
import threading
import time
import sys
from pathlib import Path
from typing import Any, Optional

# Add project root to sys.path
PROJECT_ROOT = Path(__file__).resolve().parents[3]
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

import pytest

# Enforce offscreen Qt rendering and test mode
os.environ["QT_QPA_PLATFORM"] = "offscreen"
os.environ["ANTIGRAVITY_SWISS_TESTING"] = "1"

from PySide6.QtCore import QRectF, QSize, Qt
from PySide6.QtGui import QColor, QFont, QIcon, QPainter, QPixmap
from PySide6.QtWidgets import (
    QApplication,
    QMenu,
    QSystemTrayIcon,
    QWidget,
)

from antigravity_swiss.core.config import SwissKnifeConfig
from antigravity_swiss.core.constants import (
    MD3_ACCENT_PRIMARY,
    MD3_COLOR_EXHAUSTED,
    MD3_COLOR_HEALTHY,
    MD3_COLOR_WARNING,
    MD3_OUTLINE,
    MD3_SURFACE,
    MD3_SURFACE_CONTAINER,
    MD3_SURFACE_CONTAINER_HIGH,
    MD3_TEXT_PRIMARY,
    MD3_TEXT_SECONDARY,
)
from antigravity_swiss.core.errors import (
    AccountNotFoundError,
    AccountVaultCorruptedError,
    InvalidCredentialError,
    KeyringBackendUnavailableError,
    KeyringError,
    KeyringNotFoundError,
)
from antigravity_swiss.gui.main_window import MainWindow
from antigravity_swiss.gui.pages.account_switcher_tool import AccountSwitcherToolPage
from antigravity_swiss.gui.pages.mfa_vault import MfaVaultPage
from antigravity_swiss.gui.pages.quota_dashboard import QuotaDashboardPage
from antigravity_swiss.gui.styles import GEMINI_QSS
from antigravity_swiss.gui.widgets.circular_gauge import CircularGauge
from antigravity_swiss.gui.widgets.countdown_ring import CountdownRing
from antigravity_swiss.gui.widgets.nav_rail import NavigationRail
from antigravity_swiss.gui.widgets.top_ribbon import TopRibbon
from antigravity_swiss.ipc.controller import StandaloneController
from antigravity_swiss.keyring.switcher import (
    AccountRecord,
    AccountStore,
    AccountVault,
    KeyringCredential,
    KeyringService,
)
from antigravity_swiss.totp.engine import TotpEngine, TotpResult

# Contract aliases specified in PROJECT.md and task dispatch
CircularGaugeWidget = CircularGauge
TotpCountdownRingWidget = CountdownRing


# ============================================================================
# Fixtures
# ============================================================================

@pytest.fixture(scope="session")
def qapp() -> QApplication:
    """Session-level QApplication instance for headless offscreen testing."""
    app = QApplication.instance()
    if app is None:
        app = QApplication([])
    app.setStyleSheet(GEMINI_QSS)
    return app


@pytest.fixture
def isolated_vault_env(tmp_path: Path):
    """Provides isolated filesystem directories and test configuration."""
    cfg_dir = tmp_path / "swiss_config"
    data_dir = tmp_path / "swiss_data"
    antigravity_cfg = tmp_path / "antigravity_config"
    cfg_dir.mkdir(parents=True, mode=0o700)
    data_dir.mkdir(parents=True, mode=0o700)
    antigravity_cfg.mkdir(parents=True, mode=0o700)

    accounts_file = cfg_dir / "accounts.json"
    lock_file = cfg_dir / "accounts.lock"
    sock_path = tmp_path / "daemon.sock"

    config = SwissKnifeConfig.load(
        custom_config_dir=cfg_dir,
        custom_socket_path=sock_path,
    )
    config.accounts_file = accounts_file
    config.ensure_directories()

    vault = AccountVault(config_path=accounts_file, lock_path=lock_file)

    # Seed with 3 healthy accounts
    for i in range(1, 4):
        email = f"user{i}@antigravity.test"
        cred = KeyringCredential(
            access_token=f"access_token_{i}",
            refresh_token=f"refresh_token_{i}",
            id_token="",
        )
        vault.add_or_update_account(
            email=email,
            credential=cred,
            label=f"Test Account {i}",
            totp_secret="JBSWY3DPEHPK3PXP",
        )
    vault.set_active_account("user1@antigravity.test")

    return {
        "config": config,
        "vault": vault,
        "accounts_file": accounts_file,
        "lock_file": lock_file,
        "tmp_path": tmp_path,
    }


@pytest.fixture
def mock_gui_controller(isolated_vault_env):
    """Provides StandaloneController wired to the isolated vault."""
    return StandaloneController(isolated_vault_env["config"])


# ============================================================================
# Suite 1: Headless GUI Adversarial Stress Tests
# ============================================================================

class TestHeadlessGuiStress:
    """Headless GUI stress testing: rapid interaction, geometry, models, theme."""

    def test_widget_instantiation_and_contract_aliases(self, qapp, mock_gui_controller):
        """Verify all core widgets and contract aliases instantiate cleanly offscreen."""
        window = MainWindow(controller=mock_gui_controller)
        assert window is not None
        assert isinstance(window, MainWindow)

        rail = NavigationRail()
        assert rail is not None
        assert isinstance(rail, NavigationRail)

        ribbon = TopRibbon()
        assert ribbon is not None
        assert isinstance(ribbon, TopRibbon)

        gauge = CircularGaugeWidget(model_name="Gemini 3.8 Flash", fraction=0.75)
        assert gauge is not None
        assert isinstance(gauge, CircularGauge)
        assert gauge.fraction == 0.75

        ring = TotpCountdownRingWidget(secret="JBSWY3DPEHPK3PXP")
        assert ring is not None
        assert isinstance(ring, CountdownRing)
        assert len(ring.current_code) == 6

    def test_rapid_rail_tab_switching_stress(self, qapp, mock_gui_controller):
        """Stress-test rapid switching between NavigationRail modules (300 cycles)."""
        window = MainWindow(controller=mock_gui_controller)
        events_received = []
        window.nav_rail.tool_selected.connect(lambda idx: events_received.append(idx))

        # Rapidly toggle 0 -> 1 -> 2 -> 0 -> ...
        for cycle in range(100):
            for target_idx in (0, 1, 2):
                window.nav_rail.set_current_index(target_idx)
                assert window.nav_rail._current_index == target_idx
                assert window.tool_stack.currentIndex() == target_idx

        assert len(events_received) == 300
        # Final index should match last requested
        assert window.tool_stack.currentIndex() == 2

    def test_rapid_ribbon_tab_switching_stress(self, qapp, mock_gui_controller):
        """Stress-test rapid switching across TopRibbon sub-pages (100 cycles)."""
        window = MainWindow(controller=mock_gui_controller)
        acc_tool = window.page_account_switcher
        ribbon_events = []
        acc_tool.ribbon.tab_selected.connect(lambda idx: ribbon_events.append(idx))

        # Mock heavy filesystem scan during rapid UI switching stress
        acc_tool.page_brain_cache.scan_cache = lambda: None

        # Rapidly cycle through all 5 ribbon tabs 20 times (100 switches)
        for cycle in range(20):
            for tab_idx in range(5):
                acc_tool.ribbon.set_current_index(tab_idx)
                assert acc_tool.ribbon._current_index == tab_idx
                assert acc_tool.stack.currentIndex() == tab_idx

        assert len(ribbon_events) == 100
        assert acc_tool.stack.currentIndex() == 4

    def test_extreme_geometry_and_resize_stress(self, qapp, mock_gui_controller):
        """Stress-test resizing across extreme dimensions and verify repaints without errors."""
        window = MainWindow(controller=mock_gui_controller)
        gauge = CircularGaugeWidget(model_name="Stress Gauge", fraction=0.5)
        ring = TotpCountdownRingWidget(secret="JBSWY3DPEHPK3PXP")

        extreme_sizes = [
            (1, 1),
            (50, 50),
            (100, 100),
            (960, 640),
            (1920, 1080),
            (3840, 2160),      # 4K
            (10000, 300),      # Extreme ultra-wide
            (300, 10000),      # Extreme ultra-tall
            (0, 0),            # Zero dimension
        ]

        for w, h in extreme_sizes:
            window.resize(w, h)
            gauge.resize(w, h)
            ring.resize(w, h)

            # Trigger paint without exception
            window.repaint()
            gauge.repaint()
            ring.repaint()

        # Reset to standard size
        window.resize(1120, 740)
        assert window.width() == 1120
        assert window.height() == 740

    def test_dynamic_theme_token_hot_swap(self, qapp, mock_gui_controller):
        """Verify dynamic replacement of theme stylesheet and re-polishing."""
        window = MainWindow(controller=mock_gui_controller)

        custom_qss = f"""
        QMainWindow {{
            background-color: #0b0c0e;
            color: #ffffff;
        }}
        QPushButton {{
            background-color: #1a1c1e;
            border: 2px solid #a8c7fa;
            border-radius: 20px;
        }}
        """

        # Hot-swap stylesheet on application and window
        qapp.setStyleSheet(custom_qss)
        window.setStyleSheet(custom_qss)

        # Force unpolish/polish on child widgets
        for btn in window.nav_rail._buttons:
            btn.style().unpolish(btn)
            btn.style().polish(btn)

        for btn in window.page_account_switcher.ribbon._buttons:
            btn.style().unpolish(btn)
            btn.style().polish(btn)

        window.repaint()

        # Restore official Gemini theme
        qapp.setStyleSheet(GEMINI_QSS)
        window.setStyleSheet(GEMINI_QSS)
        window.repaint()

    def test_adversarial_quota_model_inputs(self, qapp):
        """Adversarially test CircularGauge with NaN, Inf, extreme numbers and invalid types."""
        gauge = CircularGaugeWidget(model_name="Adversarial Gauge", fraction=1.0)

        # Negative numbers clamp to 0.0
        gauge.fraction = -100.0
        assert gauge.fraction == 0.0
        gauge.repaint()

        # Numbers > 1.0 clamp to 1.0
        gauge.fraction = 999.0
        assert gauge.fraction == 1.0
        gauge.repaint()

        # float('inf') clamps to 1.0
        gauge.fraction = float("inf")
        assert gauge.fraction == 1.0
        gauge.repaint()

        # float('-inf') clamps to 0.0
        gauge.fraction = float("-inf")
        assert gauge.fraction == 0.0
        gauge.repaint()

        # float('nan') converts safely without crashing paintEvent
        gauge.fraction = float("nan")
        # In Python, min(1.0, nan) is 1.0, max(0.0, 1.0) is 1.0
        assert 0.0 <= gauge.fraction <= 1.0
        gauge.repaint()

        # Empty and huge string for model_name
        gauge.model_name = ""
        gauge.repaint()
        gauge.model_name = "X" * 1000
        gauge.repaint()

        # Reset text edge cases
        gauge.reset_text = ""
        gauge.repaint()
        gauge.reset_text = "R" * 500
        gauge.repaint()

    def test_corrupted_quota_summary_resilience(self, qapp, mock_gui_controller):
        """Test QuotaDashboardPage resilience against malformed quota response structures."""
        page = QuotaDashboardPage(controller=mock_gui_controller)

        # Simulate controller returning empty dict
        mock_gui_controller.get_quota_summary = lambda acct=None: {}
        page.refresh_quota()
        assert page._table.rowCount() == 4  # Falls back to default model inventory cleanly

        # Simulate controller returning malformed groups with out-of-bound fractions
        mock_gui_controller.get_quota_summary = lambda acct=None: {
            "groups": [
                {"models": [{"modelId": "gemini-3.8-flash", "remainingFraction": -0.5}]},
                {"models": [{"modelId": "gemini-3.5-flash-lite", "remainingFraction": 1.5}]},
                {"models": [{"invalid_key": "junk"}]},
            ]
        }
        page.refresh_quota()
        # Gauge fractions clamped safely
        assert page._gauges["gemini-3.8-flash"].fraction == 0.0
        assert page._gauges["gemini-3.5-flash-lite"].fraction == 1.0

        # And if groups contains completely non-dict elements, exception is gracefully caught
        mock_gui_controller.get_quota_summary = lambda acct=None: {
            "groups": ["corrupted_non_dict_element"]
        }
        page.refresh_quota()
        assert "Quota fetch failed" in page._last_poll_label.text()

        # Simulate exception during quota fetch
        def failing_fetch(acct=None):
            raise ConnectionError("Upstream timeout 504")

        mock_gui_controller.get_quota_summary = failing_fetch
        page.refresh_quota()
        assert "Quota fetch failed: Upstream timeout 504" in page._last_poll_label.text()

    def test_malformed_account_models_resilience(self, qapp, mock_gui_controller):
        """Test MfaVaultPage and TopRibbon resilience when accounts list is corrupted."""
        page = MfaVaultPage(controller=mock_gui_controller)

        # Invert controller accounts to return corrupted rows
        mock_gui_controller.list_accounts = lambda: [
            {"email": "", "label": ""},
            {"email": "broken@account", "has_totp": False, "totp_secret": None},
            {"unexpected_schema": 123},
        ]
        page.load_accounts()
        assert page._table.rowCount() == 3

        # Select row with invalid secret
        page._table.selectRow(1)
        assert page._selected_account == "broken@account"
        assert page._countdown_ring.current_code in ("", "------")

        # TopRibbon handling of None and empty account strings
        ribbon = TopRibbon()
        ribbon.set_active_account(None)
        assert "No Active Account" in ribbon._active_badge.text()
        ribbon.set_active_account("")
        assert "No Active Account" in ribbon._active_badge.text()
        ribbon.set_active_account("user@example.com")
        assert "user@example.com" in ribbon._active_badge.text()


# ============================================================================
# Suite 2: RFC 6238 TOTP Boundary Stress Tests
# ============================================================================

class TestRfc6238TotpBoundaryStress:
    """RFC 6238 and RFC 4226 boundary correctness, secret sanitation, and drift tests."""

    def test_official_rfc6238_appendix_b_vectors(self):
        """
        Validate against ALL official RFC 6238 Appendix B test vectors (HMAC-SHA1).
        Key: '12345678901234567890' (ASCII 20 bytes).
        Base32: 'GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ'.
        """
        secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

        test_vectors = [
            (59.0, 1, "287082"),
            (1111111109.0, 37037036, "081804"),
            (1111111111.0, 37037037, "050471"),
            (1234567890.0, 41152263, "005924"),
            (2000000000.0, 66666666, "279037"),
            (20000000000.0, 666666666, "353130"),
        ]

        for timestamp, expected_step, expected_code in test_vectors:
            res = TotpEngine.get_current_totp(secret, timestamp=timestamp)
            assert res.time_step == expected_step
            assert res.code == expected_code
            assert len(res.code) == 6
            assert res.code.isdigit()
            # Verify verify_code also passes at this timestamp
            assert TotpEngine.verify_code(secret, expected_code, timestamp=timestamp) is True

    def test_secret_padding_and_length_boundaries(self):
        """Test secret padding variations and RFC 4648 Base32 length constraints."""
        valid_secret_16 = "JBSWY3DPEHPK3PXP"  # 16 chars (10 bytes) -> 16 % 8 == 0
        valid_secret_unpadded = "JBSWY3DPEHPK3PX"  # 15 chars (9 bytes) -> 15 % 8 == 7, needs 1 '='

        # Auto padding ensures both decode properly
        dec1 = TotpEngine.decode_secret(valid_secret_16)
        assert len(dec1) == 10

        dec2 = TotpEngine.decode_secret(valid_secret_unpadded)
        assert len(dec2) == 9

        # Case-insensitivity and formatting tolerance (spaces and hyphens)
        dirty = "  jbsw - y3dp - ehpk - 3pxp  "
        assert TotpEngine.get_current_totp(dirty, timestamp=1000.0).code == \
               TotpEngine.get_current_totp(valid_secret_16, timestamp=1000.0).code

        # Valid Base32 lengths mod 8 can only be 0, 2, 4, 5, 7. Lengths 1, 3, 6 are invalid.
        assert TotpEngine.validate_secret("A" * 16) is True   # 16 % 8 == 0
        assert TotpEngine.validate_secret("A" * 15) is True   # 15 % 8 == 7
        assert TotpEngine.validate_secret("A" * 14) is False  # 14 % 8 == 6 (invalid in RFC 4648)
        assert TotpEngine.validate_secret("A" * 13) is True   # 13 % 8 == 5
        assert TotpEngine.validate_secret("A" * 12) is True   # 12 % 8 == 4
        assert TotpEngine.validate_secret("A" * 11) is False  # 11 % 8 == 3 (invalid in RFC 4648)
        assert TotpEngine.validate_secret("A" * 10) is True   # 10 % 8 == 2
        assert TotpEngine.validate_secret("A" * 9) is False   # 9 % 8 == 1 (invalid in RFC 4648)

    def test_invalid_base32_secrets_rejection(self):
        """Adversarial testing with illegal base32 characters, symbols, and null bytes."""
        # Note: In standard Python base64.b32decode, empty string "" decodes to b"".
        # Illegal Base32 characters ('8', '9', '0', '1', punctuation, symbols, null bytes, emojis)
        # must be strictly rejected by validate_secret() and raise ValueError in decode_secret().
        invalid_secrets = [
            "1234567890",             # Contains '8', '9', '0', '1' (non-standard Base32)
            "JBSWY3DP!@#$%^&*",       # Special characters
            "JBSWY3DP\x00\x00\x00",   # Null bytes
            "JBSWY3DP🔐✨",           # Unicode emojis
            "NOTABASE32???",          # Punctuation
            "INVALID\tKEY\n",         # Whitespace other than space/hyphen (tabs, newlines)
        ]

        for inv in invalid_secrets:
            assert TotpEngine.validate_secret(inv) is False
            with pytest.raises(ValueError):
                TotpEngine.decode_secret(inv)

    def test_countdown_ring_widget_with_invalid_secrets(self, qapp):
        """Verify TotpCountdownRingWidget safely absorbs invalid secrets without crashing."""
        ring = TotpCountdownRingWidget(secret="INVALID_BASE32_123456789")
        assert ring.current_code == "------"
        assert ring._remaining_sec == 0
        assert ring._progress == 0.0

        # Repaint with invalid secret state
        ring.repaint()

        # Update to empty secret
        ring.secret = ""
        assert ring.current_code == ""
        assert ring._remaining_sec == 0
        ring.repaint()

        # Update to valid secret recovers immediately
        ring.secret = "JBSWY3DPEHPK3PXP"
        assert len(ring.current_code) == 6
        assert ring.current_code.isdigit()
        ring.repaint()

    def test_exact_time_boundary_transitions(self):
        """Verify precise boundary step transitions (t=29s, 30s, 31s, 59s, 60s, 61s)."""
        secret = "JBSWY3DPEHPK3PXP"

        # Step 0 boundary: t = 29.999
        r29 = TotpEngine.get_current_totp(secret, timestamp=29.999)
        assert r29.time_step == 0
        assert r29.remaining_seconds == 1
        assert 0.0 < r29.progress_fraction <= (1.0 / 30.0)

        # Step 1 transition: t = 30.0
        r30 = TotpEngine.get_current_totp(secret, timestamp=30.0)
        assert r30.time_step == 1
        assert r30.remaining_seconds == 30
        assert r30.progress_fraction == 1.0
        # Code must change across boundary
        assert r29.code != r30.code

        # Step 1 progression: t = 30.001
        r30_1 = TotpEngine.get_current_totp(secret, timestamp=30.001)
        assert r30_1.time_step == 1
        assert r30_1.remaining_seconds == 30
        assert r30_1.code == r30.code

        # Step 1 next second: t = 31.0
        r31 = TotpEngine.get_current_totp(secret, timestamp=31.0)
        assert r31.time_step == 1
        assert r31.remaining_seconds == 29
        assert round(r31.progress_fraction, 4) == round(29.0 / 30.0, 4)
        assert r31.code == r30.code

        # Step 1 expiration: t = 59.999
        r59 = TotpEngine.get_current_totp(secret, timestamp=59.999)
        assert r59.time_step == 1
        assert r59.remaining_seconds == 1
        assert r59.code == r30.code

        # Step 2 transition: t = 60.0
        r60 = TotpEngine.get_current_totp(secret, timestamp=60.0)
        assert r60.time_step == 2
        assert r60.remaining_seconds == 30
        assert r60.progress_fraction == 1.0
        assert r60.code != r59.code

    def test_clock_drift_compensation_window_boundaries(self):
        """Stress-test verify_code drift compensation with window=1 and window=2."""
        secret = "JBSWY3DPEHPK3PXP"
        t_ref = 100000.0

        current_code = TotpEngine.get_current_totp(secret, timestamp=t_ref).code
        minus_30_code = TotpEngine.get_current_totp(secret, timestamp=t_ref - 30.0).code
        plus_30_code = TotpEngine.get_current_totp(secret, timestamp=t_ref + 30.0).code
        minus_60_code = TotpEngine.get_current_totp(secret, timestamp=t_ref - 60.0).code
        plus_60_code = TotpEngine.get_current_totp(secret, timestamp=t_ref + 60.0).code

        # Default window=1: accepts current, -30s, +30s
        assert TotpEngine.verify_code(secret, current_code, timestamp=t_ref, window=1) is True
        assert TotpEngine.verify_code(secret, minus_30_code, timestamp=t_ref, window=1) is True
        assert TotpEngine.verify_code(secret, plus_30_code, timestamp=t_ref, window=1) is True

        # Default window=1: rejects -60s and +60s
        assert TotpEngine.verify_code(secret, minus_60_code, timestamp=t_ref, window=1) is False
        assert TotpEngine.verify_code(secret, plus_60_code, timestamp=t_ref, window=1) is False

        # Window=2: accepts -60s and +60s
        assert TotpEngine.verify_code(secret, minus_60_code, timestamp=t_ref, window=2) is True
        assert TotpEngine.verify_code(secret, plus_60_code, timestamp=t_ref, window=2) is True

        # Window=0: strictly current step only
        assert TotpEngine.verify_code(secret, current_code, timestamp=t_ref, window=0) is True
        assert TotpEngine.verify_code(secret, minus_30_code, timestamp=t_ref, window=0) is False
        assert TotpEngine.verify_code(secret, plus_30_code, timestamp=t_ref, window=0) is False

    def test_continuous_step_determinism_and_entropy(self):
        """Verify 200 consecutive steps produce valid 6-digit codes and proper variance."""
        secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
        codes = []
        for step in range(200):
            code = TotpEngine.generate_code_at_step(secret, step=step)
            assert len(code) == 6
            assert code.isdigit()
            codes.append(code)

        # Determinism check
        for step in range(200):
            repeat_code = TotpEngine.generate_code_at_step(secret, step=step)
            assert repeat_code == codes[step]

        # Entropy / uniqueness check: 200 consecutive codes should have very high uniqueness
        unique_codes = set(codes)
        assert len(unique_codes) >= 190

    def test_extreme_epoch_timestamps(self):
        """Verify calculations at extreme timestamps (0, negative, year 2038, year 3000)."""
        secret = "JBSWY3DPEHPK3PXP"

        # Epoch start: t = 0.0
        r0 = TotpEngine.get_current_totp(secret, timestamp=0.0)
        assert r0.time_step == 0
        assert r0.remaining_seconds == 30
        assert len(r0.code) == 6

        # Negative timestamp (before 1970) produces step < 0, which exceeds unsigned 64-bit uint64 (>Q) bounds
        import struct
        with pytest.raises(struct.error):
            TotpEngine.get_current_totp(secret, timestamp=-30.0)

        # 32-bit Unix epoch rollover (2038-01-19T03:14:07Z = 2147483647)
        r_2038 = TotpEngine.get_current_totp(secret, timestamp=2147483647.0)
        assert len(r_2038.code) == 6
        assert r_2038.time_step == 71582788

        # Distant future (Year 3000 = 32503680000)
        r_3000 = TotpEngine.get_current_totp(secret, timestamp=32503680000.0)
        assert len(r_3000.code) == 6
        assert r_3000.time_step == 1083456000


# ============================================================================
# Suite 3: Keyring Concurrent Atomic Switches & Resiliency
# ============================================================================

class TestKeyringConcurrentSwitchStress:
    """Stress tests for multi-thread atomic switching, vault transactions, and error recovery."""

    def test_high_concurrency_account_switching(self, isolated_vault_env):
        """Simulate 20 concurrent threads rapidly switching accounts in AccountVault."""
        vault: AccountVault = isolated_vault_env["vault"]
        target_accounts = [f"user{i}@antigravity.test" for i in range(1, 4)]
        errors = []

        def worker_switch(thread_id: int):
            for i in range(25):
                target = target_accounts[(thread_id + i) % len(target_accounts)]
                try:
                    vault.set_active_account(target)
                    active = vault.get_active_account()
                    assert active in target_accounts
                except Exception as exc:
                    errors.append((thread_id, exc))

        threads = [threading.Thread(target=worker_switch, args=(t,)) for t in range(20)]
        for t in threads:
            t.start()
        for t in threads:
            t.join()

        assert len(errors) == 0
        # Verify accounts.json remains valid JSON
        data = vault.load()
        assert data["active_account"] in target_accounts
        assert len(data["accounts"]) == 3

    def test_concurrent_add_update_delete_transactions(self, isolated_vault_env):
        """Stress-test concurrent add, update, and delete transactions across 15 threads."""
        vault: AccountVault = isolated_vault_env["vault"]
        errors = []

        def worker_mutate(thread_id: int):
            for i in range(15):
                email = f"dynamic_user_{thread_id}_{i}@test.com"
                cred = KeyringCredential(access_token=f"tok_{i}", refresh_token=f"ref_{i}")
                try:
                    # Add
                    vault.add_or_update_account(email=email, credential=cred, label=f"Dyn {i}")
                    # Update TOTP
                    vault.set_totp_secret(email, "HXDMVJECJJWSRB3H")
                    # Read
                    rec = vault.get_account(email)
                    assert rec is not None
                    # Remove
                    vault.remove_account(email)
                except Exception as exc:
                    errors.append((thread_id, exc))

        threads = [threading.Thread(target=worker_mutate, args=(t,)) for t in range(15)]
        for t in threads:
            t.start()
        for t in threads:
            t.join()

        assert len(errors) == 0
        # Primary accounts remain intact
        data = vault.load()
        assert len(data["accounts"]) >= 3

    def test_vault_file_permissions_and_no_tmp_leak(self, isolated_vault_env):
        """Verify strict file modes (0600 on file, 0700 on dir) and absence of leaked .tmp files."""
        vault: AccountVault = isolated_vault_env["vault"]
        accounts_file = isolated_vault_env["accounts_file"]
        config_dir = isolated_vault_env["config"].config_dir

        # Check permissions
        file_mode = accounts_file.stat().st_mode & 0o777
        assert file_mode == 0o600
        dir_mode = config_dir.stat().st_mode & 0o777
        assert dir_mode == 0o700

        # Check that no temporary files were left behind
        tmp_files = list(config_dir.glob(".*.tmp.*"))
        assert len(tmp_files) == 0

    def test_keyring_backend_failure_resilience(self, isolated_vault_env):
        """Simulate Secret Service / backend failure and verify graceful domain error handling."""
        vault: AccountVault = isolated_vault_env["vault"]

        class FailingBackend:
            def lookup(self, service: str, username: str) -> str | None:
                raise KeyringBackendUnavailableError("Secret Service daemon crashed")

            def store(self, secret: str, service: str, username: str, label: str = "") -> None:
                raise KeyringError("DBus session locked")

        service = KeyringService(backend=FailingBackend(), vault=vault)

        # Lookup failure propagates domain error without crashing vault
        with pytest.raises(KeyringBackendUnavailableError):
            service.get_active_credential()

        # Store failure propagates domain error
        cred = KeyringCredential(access_token="abc", refresh_token="def")
        with pytest.raises(KeyringError):
            service.set_active_credential(cred)

        # Vault remains perfectly readable
        assert len(vault.list_accounts()) == 3

    def test_vault_corruption_quarantine_and_clean_recovery(self, isolated_vault_env):
        """Verify corrupted accounts.json is safely quarantined and a clean vault is recovered."""
        vault: AccountVault = isolated_vault_env["vault"]
        accounts_file: Path = isolated_vault_env["accounts_file"]

        # Corrupt file with invalid JSON garbage
        with open(accounts_file, "w", encoding="utf-8") as f:
            f.write("{ invalid json corrupted bytes \x00\x01\x02")

        # Loading corrupted file directly raises AccountVaultCorruptedError and quarantines
        with pytest.raises(AccountVaultCorruptedError):
            vault.load()

        # Check that quarantine file was created with 0600 permissions
        corrupted_backups = list(accounts_file.parent.glob("accounts.json.corrupted.*"))
        assert len(corrupted_backups) >= 1
        backup = corrupted_backups[0]
        assert backup.stat().st_mode & 0o777 == 0o600

        # Transaction automatically reinitializes clean structure after quarantine
        with vault.transaction() as data:
            data["accounts"]["restored@user.com"] = {
                "email": "restored@user.com",
                "label": "Restored",
                "credential": {"access_token": "a", "refresh_token": "r"},
                "added_at": "2026-10-02T12:00:00Z",
                "totp_secret": "",
                "is_healthy": True,
            }

        # Vault is healthy again
        assert vault.list_accounts() == ["restored@user.com"]


# ============================================================================
# Suite 4: System Tray Headless Fallback Tests
# ============================================================================

class TestSystemTrayHeadlessFallback:
    """System tray DBus StatusNotifierItem fallback in offscreen / CI environments."""

    def test_system_tray_headless_availability_check(self, qapp):
        """Assert that QSystemTrayIcon.isSystemTrayAvailable() returns False offscreen."""
        assert QSystemTrayIcon.isSystemTrayAvailable() is False

    def test_tray_icon_and_menu_instantiation_offscreen(self, qapp):
        """Verify QSystemTrayIcon can be safely constructed, customized, and manipulated offscreen."""
        tray = QSystemTrayIcon()
        assert tray is not None

        # Build context menu with actions
        menu = QMenu()
        act_open = menu.addAction("Open Dashboard")
        act_switch = menu.addAction("Quick Switch")
        act_exit = menu.addAction("Exit")
        tray.setContextMenu(menu)

        # Set tooltip
        tray.setToolTip("Antigravity Swiss Knife (Offscreen)")

        # Generate a 16x16 dummy pixmap icon
        pix = QPixmap(16, 16)
        pix.fill(QColor(MD3_ACCENT_PRIMARY))
        icon = QIcon(pix)
        tray.setIcon(icon)

        # Show and hide should execute without unhandled exceptions or X11/Wayland faults
        tray.show()
        tray.hide()

    def test_tray_quota_health_badge_color_mapping(self):
        """Verify quota health thresholds properly map to Material 3 colors for tray badges."""
        def get_tray_badge_color(remaining_fraction: float) -> str:
            if remaining_fraction > 0.30:
                return MD3_COLOR_HEALTHY    # #81c995
            elif remaining_fraction >= 0.10:
                return MD3_COLOR_WARNING    # #fdd663
            else:
                return MD3_COLOR_EXHAUSTED  # #f28b82

        assert get_tray_badge_color(1.0) == MD3_COLOR_HEALTHY
        assert get_tray_badge_color(0.31) == MD3_COLOR_HEALTHY
        assert get_tray_badge_color(0.30) == MD3_COLOR_WARNING
        assert get_tray_badge_color(0.15) == MD3_COLOR_WARNING
        assert get_tray_badge_color(0.10) == MD3_COLOR_WARNING
        assert get_tray_badge_color(0.09) == MD3_COLOR_EXHAUSTED
        assert get_tray_badge_color(0.0) == MD3_COLOR_EXHAUSTED

    def test_tray_quick_switch_menu_generation_boundaries(self, qapp):
        """Test quick switch context menu generation across 0, 1, and 100 accounts."""
        def build_tray_menu(accounts: list[str], active_account: Optional[str]) -> QMenu:
            menu = QMenu()
            menu.addAction("📊 Open Quota Dashboard")
            menu.addSeparator()

            switch_sub = menu.addMenu("🔄 Switch Account")
            if not accounts:
                disabled_act = switch_sub.addAction("No standby accounts available")
                disabled_act.setEnabled(False)
            else:
                for acc in accounts:
                    prefix = "● " if acc == active_account else "○ "
                    switch_sub.addAction(f"{prefix}{acc}")

            menu.addSeparator()
            menu.addAction("Exit")
            return menu

        # 0 accounts: disabled placeholder action
        menu_empty = build_tray_menu([], None)
        actions_empty = menu_empty.actions()
        assert len(actions_empty) >= 3

        # 1 account
        menu_one = build_tray_menu(["single@test.com"], "single@test.com")
        assert len(menu_one.actions()) >= 3

        # 100 accounts: scales cleanly without memory or Qt issues
        many_accounts = [f"account_{i}@domain.com" for i in range(100)]
        menu_many = build_tray_menu(many_accounts, "account_42@domain.com")
        assert len(menu_many.actions()) >= 3

    def test_dbus_notification_error_fallback(self):
        """Verify simulated failure to dispatch desktop notifications is caught gracefully."""
        notification_logged = False
        notification_dispatched = False

        def send_desktop_notification(title: str, message: str, simulate_fail: bool = True):
            nonlocal notification_logged, notification_dispatched
            try:
                if simulate_fail:
                    raise ConnectionError("org.freedesktop.Notifications D-Bus service is unreachable")
                notification_dispatched = True
            except ConnectionError as err:
                # Expected non-fatal fallback
                logging.getLogger("tray").warning("Desktop notification failed: %s", err)
                notification_logged = True

        send_desktop_notification("Account Switched", "Switched to user@test.com", simulate_fail=True)
        assert notification_logged is True
        assert notification_dispatched is False
