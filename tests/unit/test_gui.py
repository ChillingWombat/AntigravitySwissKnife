"""
Unit and Component Tests for Antigravity Swiss Knife Desktop GUI.
================================================================
Tests all Google Gemini MD3 widgets, pages, and MainWindow in offscreen mode.
"""

from __future__ import annotations

import os
import pytest

# Ensure headless offscreen platform for Qt
os.environ["QT_QPA_PLATFORM"] = "offscreen"
os.environ["ANTIGRAVITY_SWISS_TESTING"] = "1"

from PySide6.QtCore import Qt
from PySide6.QtGui import QPaintEvent
from PySide6.QtWidgets import QApplication

from antigravity_swiss.core.config import SwissKnifeConfig
from antigravity_swiss.fingerprint.profile_store import DeviceProfileStore
from antigravity_swiss.gui.app import create_app
from antigravity_swiss.gui.main_window import MainWindow
from antigravity_swiss.gui.pages.account_switcher_tool import AccountSwitcherToolPage
from antigravity_swiss.gui.pages.brain_cache import BrainCachePage
from antigravity_swiss.gui.pages.fingerprints import DeviceFingerprintsPage
from antigravity_swiss.gui.pages.mfa_vault import MfaVaultPage
from antigravity_swiss.gui.pages.quota_dashboard import QuotaDashboardPage
from antigravity_swiss.gui.pages.switcher_settings import SwitcherSettingsPage
from antigravity_swiss.gui.pages.system_settings import SystemSettingsPage
from antigravity_swiss.gui.pages.tools_marketplace import ToolsMarketplacePage
from antigravity_swiss.gui.widgets.circular_gauge import CircularGauge
from antigravity_swiss.gui.widgets.countdown_ring import CountdownRing
from antigravity_swiss.gui.widgets.nav_rail import NavigationRail
from antigravity_swiss.gui.widgets.top_ribbon import TopRibbon
from antigravity_swiss.ipc.controller import StandaloneController
from antigravity_swiss.keyring.switcher import AccountStore, KeyringCredential


@pytest.fixture(scope="session")
def qapp():
    """Session-level QApplication instance for offscreen testing."""
    app = QApplication.instance()
    if app is None:
        app = QApplication([])
    return app


@pytest.fixture
def mock_controller(temp_dir):
    """Provides a hermetic StandaloneController with isolated test files."""
    from pathlib import Path
    cfg_dir = Path(temp_dir) / "cfg"
    sock_path = Path(temp_dir) / "test.sock"
    config = SwissKnifeConfig.load(custom_config_dir=cfg_dir, custom_socket_path=sock_path)
    config.ensure_directories()
    accounts_file = config.accounts_file

    # Seed an account in the store with TOTP secret
    store = AccountStore(accounts_file)
    cred = KeyringCredential(access_token="test_tok", refresh_token="test_ref")
    store.vault.add_or_update_account(
        email="test_user@gmail.com",
        credential=cred,
        label="Test Account",
        totp_secret="JBSWY3DPEHPK3PXP",  # Standard RFC secret ("Hello!\xde\xad\xbe\xef")
    )
    store.vault.set_active_account("test_user@gmail.com")

    return StandaloneController(config)


def test_circular_gauge(qapp):
    """Test CircularGauge widget rendering and fraction clamp."""
    gauge = CircularGauge(model_name="Gemini 3.8 Flash", fraction=0.85, reset_text="4h 12m")
    assert gauge.fraction == 0.85
    assert gauge.model_name == "Gemini 3.8 Flash"
    assert gauge.reset_text == "4h 12m"

    # Clamp bounds
    gauge.fraction = 1.5
    assert gauge.fraction == 1.0
    gauge.fraction = -0.5
    assert gauge.fraction == 0.0

    # Ensure paint event renders without raising
    gauge.resize(160, 180)
    gauge.repaint()


def test_countdown_ring(qapp):
    """Test CountdownRing widget with valid Base32 secret."""
    ring = CountdownRing(secret="JBSWY3DPEHPK3PXP")
    assert len(ring.current_code) == 6
    assert ring.current_code.isdigit()

    ticks = []
    ring.tick.connect(lambda sec, prog: ticks.append((sec, prog)))
    ring._refresh_state()
    assert len(ticks) >= 1
    sec, prog = ticks[-1]
    assert 0 <= sec <= 30
    assert 0.0 <= prog <= 1.0

    ring.resize(54, 54)
    ring.repaint()


def test_navigation_rail(qapp):
    """Test NavigationRail selection and status indicators."""
    rail = NavigationRail()
    selected_indices = []
    rail.tool_selected.connect(lambda idx: selected_indices.append(idx))

    rail.set_current_index(1)
    assert rail._current_index == 1
    assert selected_indices == [1]

    rail.set_daemon_status(True)
    assert "Connected" in rail._daemon_lbl.text()
    rail.set_daemon_status(False)
    assert "Standalone" in rail._daemon_lbl.text()

    rail.set_antigravity_status(True, pid=12345)
    assert "12345" in rail._host_lbl.text()


def test_top_ribbon(qapp):
    """Test TopRibbon pill tab navigation and active badge."""
    ribbon = TopRibbon()
    selected_tabs = []
    ribbon.tab_selected.connect(lambda idx: selected_tabs.append(idx))

    ribbon.set_current_index(2)
    assert ribbon._current_index == 2
    assert selected_tabs == [2]

    ribbon.set_active_account("user@example.com")
    assert "user@example.com" in ribbon._active_badge.text()


def test_quota_dashboard_page(qapp, mock_controller):
    """Test QuotaDashboardPage data loading and model gauges."""
    page = QuotaDashboardPage(controller=mock_controller)
    assert page._active_account == "test_user@gmail.com"
    assert "test_user@gmail.com" in page._acc_label.text()
    assert len(page._gauges) == 4
    assert "gemini-3.8-flash" in page._gauges
    assert page._table.rowCount() == 4


def test_mfa_vault_page(qapp, mock_controller):
    """Test MfaVaultPage with live TOTP code and secret updates."""
    page = MfaVaultPage(controller=mock_controller)
    assert page._table.rowCount() >= 1

    # Select the row
    page._table.selectRow(0)
    assert page._selected_account == "test_user@gmail.com"
    assert page._selected_secret == "JBSWY3DPEHPK3PXP"
    assert page._code_label.text() != "------"

    # Test copy code
    page._on_copy_code()

    # Test update secret
    page._secret_input.setText("HXDMVJECJJWSRB3HWIZR4IFUGFTMXBOZ")
    page._on_save_secret()
    assert page._selected_secret == "HXDMVJECJJWSRB3HWIZR4IFUGFTMXBOZ"


def test_fingerprints_page(qapp, mock_controller):
    """Test DeviceFingerprintsPage random generator and profile saving."""
    page = DeviceFingerprintsPage(controller=mock_controller)
    assert page._acc_combo.count() >= 1

    # Generate fresh
    page._on_generate_random()
    m_id = page._inputs["machine_id"].text()
    assert len(m_id) > 10

    # Save
    page._on_save_profile()
    assert page._table.rowCount() >= 1


def test_brain_cache_page(qapp, mock_controller):
    """Test BrainCachePage scanning and breakdown display."""
    page = BrainCachePage(controller=mock_controller)
    page.scan_cache()
    assert page._breakdown is not None
    assert page._table.rowCount() > 0


def test_switcher_settings_page(qapp, mock_controller):
    """Test SwitcherSettingsPage rule adjustment and save."""
    page = SwitcherSettingsPage(controller=mock_controller)
    page._chk_auto_switch.setChecked(False)
    page._slider_thresh.setValue(12)
    page._on_save_settings()

    cfg = mock_controller.get_rule_config()
    assert cfg["auto_switch_enabled"] is False
    assert cfg["auto_switch_threshold"] == 0.12


def test_main_window_full_integration(qapp, mock_controller):
    """Test complete MainWindow with rail navigation and ribbon switching."""
    window = MainWindow(controller=mock_controller)
    assert window.tool_stack.count() == 3

    # Switch to Marketplace
    window.nav_rail.set_current_index(1)
    assert window.tool_stack.currentIndex() == 1

    # Switch to Settings
    window.nav_rail.set_current_index(2)
    assert window.tool_stack.currentIndex() == 2

    # Switch back to Account Switcher
    window.nav_rail.set_current_index(0)
    assert window.tool_stack.currentIndex() == 0

    # Test Account Switcher Ribbon tabs
    acc_tool = window.page_account_switcher
    for tab_idx in range(5):
        acc_tool.ribbon.set_current_index(tab_idx)
        assert acc_tool.stack.currentIndex() == tab_idx

    # Trigger status sync
    window._sync_status()
