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
from PySide6.QtWidgets import QApplication, QFrame

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


def test_circular_gauge_empty_grey(qapp):
    """Test CircularGauge with empty_grey=True and fraction=None renders N/A."""
    gauge = CircularGauge(model_name="Custom Untracked", fraction=None, empty_grey=True)
    assert gauge.empty_grey is True
    assert gauge.fraction == 0.0

    gauge.resize(160, 180)
    gauge.repaint()

    gauge.fraction = 0.5
    gauge.empty_grey = False
    assert gauge.empty_grey is False
    assert gauge.fraction == 0.5
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
    assert "Connected" in rail._daemon_lbl.text() or "Active" in rail._daemon_lbl.text()
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
    """Test QuotaDashboardPage fleet overview, rings, auto-switch, and account rows."""
    page = QuotaDashboardPage(controller=mock_controller)
    assert page._active_account == "test_user@gmail.com"
    assert "test_user@gmail.com" in page._acc_label.text()
    assert "Account" in page._total_accounts_lbl.text()
    assert "next_5h" in page._gauges
    assert "weekly" in page._gauges
    assert 0.0 <= page._ring_5h.fraction <= 1.0
    assert 0.0 <= page._ring_weekly.fraction <= 1.0
    assert page._table.rowCount() >= 1

    # Test auto-switch toggle button
    initial_switch = page._auto_switch_enabled
    page._on_toggle_auto_switch()
    assert page._auto_switch_enabled != initial_switch
    assert ("ON" in page.btn_auto_switch.text()) if page._auto_switch_enabled else ("OFF" in page.btn_auto_switch.text())

    # Test opening detail dialog
    assert len(page._accounts_cache) >= 1
    acc_state = page._accounts_cache[0]
    from antigravity_swiss.gui.dialogs.account_detail_dialog import AccountDetailDialog
    dialog = AccountDetailDialog(account=acc_state, controller=mock_controller)
    assert dialog.account.email == acc_state.email
    assert hasattr(dialog, "btn_save")


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
    assert window.tool_stack.count() == 7
    assert hasattr(window, "v_separator")
    assert window.v_separator.frameShape() == QFrame.Shape.VLine
    assert window.v_separator.width() == 1 or window.v_separator.maximumWidth() == 1

    # Switch to App Enhancements (index 1)
    window.nav_rail.set_current_index(1)
    assert window.tool_stack.currentIndex() == 1

    # Switch to Custom Models (index 2)
    window.nav_rail.set_current_index(2)
    assert window.tool_stack.currentIndex() == 2

    # Switch to Scheduled Templates (index 3)
    window.nav_rail.set_current_index(3)
    assert window.tool_stack.currentIndex() == 3

    # Switch to Marketplace (index 4)
    window.nav_rail.set_current_index(4)
    assert window.tool_stack.currentIndex() == 4

    # Switch to Archived Projects (index 5)
    window.nav_rail.set_current_index(5)
    assert window.tool_stack.currentIndex() == 5

    # Switch to Settings via bottom button (index 6)
    assert hasattr(window, "btn_system_settings")
    window.btn_system_settings.click()
    assert window.tool_stack.currentIndex() == 6

    # Switch back to Account Switcher (index 0)
    window.nav_rail.set_current_index(0)
    assert window.tool_stack.currentIndex() == 0

    # Test Account Switcher Ribbon tabs
    acc_tool = window.page_account_switcher
    for tab_idx in range(5):
        acc_tool.ribbon.set_current_index(tab_idx)
        assert acc_tool.stack.currentIndex() == tab_idx

    # Trigger status sync
    window._sync_status()

    # Verify tray
    assert hasattr(window, "tray")
    assert window.tray is not None


def test_system_tray_integration(qapp, mock_controller):
    """Test SwissKnifeTray icon badges, menu generation, and notification dispatch."""
    from antigravity_swiss.gui.tray import SwissKnifeTray
    tray = SwissKnifeTray(controller=mock_controller)

    # Test badge pixmaps
    for status in ("HEALTHY", "WARNING", "EXHAUSTED"):
        tray.update_icon(status)
        assert tray._current_health == status

    # Test context menu items
    tray.refresh_menu()
    actions = [a.text() for a in tray._menu.actions()]
    assert any("Open Dashboard" in a for a in actions)
    assert any("Refresh Quota" in a for a in actions)
    assert any("Switch Account" in a for a in actions)

    # Test notification dispatch
    tray.dispatch_notification("Test Title", "Test Body")


def test_account_quota_bar_widget(qapp):
    """Test AccountQuotaBarWidget progress bar clamping, colors, and percentage labels."""
    from antigravity_swiss.gui.widgets.account_quota_bar import AccountQuotaBarWidget
    
    # Healthy (85%)
    bar1 = AccountQuotaBarWidget(0.85)
    assert bar1.fraction == 0.85
    assert bar1.lbl.text() == "85%"
    assert bar1.bar.value() == 85
    
    # Warning (20%)
    bar2 = AccountQuotaBarWidget(0.20)
    assert bar2.fraction == 0.20
    assert bar2.lbl.text() == "20%"
    assert bar2.bar.value() == 20
    
    # Exhausted (5%)
    bar3 = AccountQuotaBarWidget(0.05)
    assert bar3.fraction == 0.05
    assert bar3.lbl.text() == "5%"
    assert bar3.bar.value() == 5
    
    # Clamping
    bar_over = AccountQuotaBarWidget(1.5)
    assert bar_over.fraction == 1.0
    assert bar_over.bar.value() == 100
    
    bar_under = AccountQuotaBarWidget(-0.2)
    assert bar_under.fraction == 0.0
    assert bar_under.bar.value() == 0


def test_account_detail_dialog_full_flow(qapp, mock_controller):
    """Test AccountDetailDialog editing label, TOTP secret, refresh token, and saving."""
    from antigravity_swiss.gui.dialogs.account_detail_dialog import AccountDetailDialog
    from antigravity_swiss.quota.calculator import AccountQuotaState
    from antigravity_swiss.keyring.switcher import AccountStore
    from PySide6.QtWidgets import QLineEdit

    acc_state = AccountQuotaState(
        email="test_user@gmail.com",
        label="Original Label",
        is_active=True,
        status="ACTIVE",
        has_totp=True,
        totp_secret="JBSWY3DPEHPK3PXP",
        refresh_token="1//original_refresh",
        quota_5h_current=0.75,
        reset_seconds=7200.0,
        quota_weekly=0.88,
    )

    dialog = AccountDetailDialog(account=acc_state, controller=mock_controller)
    assert dialog.label_edit.text() == "Original Label"
    assert dialog.totp_edit.text() == "JBSWY3DPEHPK3PXP"
    assert dialog.totp_edit.echoMode() == QLineEdit.EchoMode.Password

    # Toggle echo mode
    dialog._toggle_totp_echo()
    assert dialog.totp_edit.echoMode() == QLineEdit.EchoMode.Normal
    assert dialog.btn_toggle_totp.text() == "Hide"
    dialog._toggle_totp_echo()
    assert dialog.totp_edit.echoMode() == QLineEdit.EchoMode.Password
    assert dialog.btn_toggle_totp.text() == "Show"

    # Toggle refresh token echo mode
    assert dialog.ref_edit.echoMode() == QLineEdit.EchoMode.Password
    dialog._toggle_ref_echo()
    assert dialog.ref_edit.echoMode() == QLineEdit.EchoMode.Normal
    assert dialog.btn_toggle_ref.text() == "Hide"

    # Edit fields
    dialog.label_edit.setText("Updated Engineering Lead")
    dialog.totp_edit.setText("HXDMVJECJJWSRB3HWIZR4IFUGFTMXBOZ")
    assert "Live MFA Preview" in dialog.totp_preview_lbl.text()
    dialog.ref_edit.setText("1//updated_refresh_token")

    # Save
    saved_signals = []
    dialog.account_saved.connect(lambda em: saved_signals.append(em))
    dialog._on_save_changes()

    assert saved_signals == ["test_user@gmail.com"]

    # Verify store updated
    store = AccountStore(mock_controller.config.accounts_file)
    acc = store.vault.get_account("test_user@gmail.com")
    assert acc.label == "Updated Engineering Lead"
    assert acc.totp_secret == "HXDMVJECJJWSRB3HWIZR4IFUGFTMXBOZ"
    assert acc.credential.refresh_token == "1//updated_refresh_token"

