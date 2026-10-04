"""
Antigravity Swiss Knife Main Window.
====================================
Desktop application main window strictly following Google Gemini / Material Design 3:
- Fixed Left Navigation Rail (Account Switcher, Tools Marketplace, System Settings)
- Top Ribbon Sub-Navigation for Account Switcher (Dashboard, MFA, Fingerprints, Cache, Settings)
- Circular vector quota gauges and animated RFC 6238 TOTP countdown rings
"""

from __future__ import annotations

from typing import Optional
from PySide6.QtCore import Qt, QTimer
from PySide6.QtGui import QIcon
from PySide6.QtWidgets import (
    QFrame,
    QHBoxLayout,
    QLabel,
    QMainWindow,
    QPushButton,
    QStackedWidget,
    QVBoxLayout,
    QWidget,
)

from antigravity_swiss.core.constants import (
    APP_TITLE,
    MD3_LIGHT_ACCENT_CONTAINER,
    MD3_LIGHT_ACCENT_ON_CONTAINER,
    MD3_LIGHT_ACCENT_PRIMARY,
    MD3_LIGHT_OUTLINE,
    MD3_LIGHT_SURFACE,
    MD3_LIGHT_SURFACE_CONTAINER,
    MD3_LIGHT_SURFACE_CONTAINER_HIGH,
    MD3_LIGHT_TEXT_PRIMARY,
    MD3_LIGHT_TEXT_SECONDARY,
)
from antigravity_swiss.gui.pages.account_switcher_tool import AccountSwitcherToolPage
from antigravity_swiss.gui.pages.system_settings import SystemSettingsPage
from antigravity_swiss.gui.pages.tools_marketplace import ToolsMarketplacePage
from antigravity_swiss.gui.styles import GEMINI_QSS
from antigravity_swiss.gui.widgets.nav_rail import NavigationRail
from antigravity_swiss.ipc.controller import SwissKnifeController


class MainWindow(QMainWindow):
    """
    Main application window for Antigravity Swiss Knife.
    """

    def __init__(
        self,
        controller: SwissKnifeController,
        parent: Optional[QWidget] = None,
    ) -> None:
        super().__init__(parent)
        self.controller = controller

        self.setWindowTitle(APP_TITLE)
        self.resize(1180, 760)
        self.setMinimumSize(960, 640)
        self.setStyleSheet(GEMINI_QSS)

        self._init_ui()

        # System Tray Integration (Feature F24)
        from antigravity_swiss.gui.tray import SwissKnifeTray
        self.tray = SwissKnifeTray(controller=self.controller, parent=self)
        self.tray.show_window_requested.connect(self._restore_from_tray)
        self.tray.open_settings_requested.connect(self._open_settings_from_tray)
        self.tray.account_switch_requested.connect(self.page_account_switcher._on_account_switched)
        self.tray.show()


        # Status sync timer (every 10s)
        self._sync_timer = QTimer(self)
        self._sync_timer.setInterval(10000)
        self._sync_timer.timeout.connect(self._sync_status)
        self._sync_timer.start()

        self._sync_status()

    def _init_ui(self) -> None:
        central_widget = QWidget(self)
        central_widget.setObjectName("centralWidget")
        central_widget.setStyleSheet(f"background-color: {MD3_LIGHT_SURFACE};")
        self.setCentralWidget(central_widget)

        main_layout = QHBoxLayout(central_widget)
        main_layout.setContentsMargins(0, 0, 0, 0)
        main_layout.setSpacing(0)

        # 1. Fixed Left Navigation Rail
        self.nav_rail = NavigationRail(self)
        self.nav_rail.tool_selected.connect(self._on_tool_selected)
        main_layout.addWidget(self.nav_rail)

        # 2. Vertical Separation Line between Left Panel and Feature Pages
        self.v_separator = QFrame(self)
        self.v_separator.setObjectName("railVerticalSeparator")
        self.v_separator.setFrameShape(QFrame.Shape.VLine)
        self.v_separator.setFrameShadow(QFrame.Shadow.Plain)
        self.v_separator.setFixedWidth(1)
        self.v_separator.setStyleSheet(f"""
            QFrame#railVerticalSeparator {{
                background-color: {MD3_LIGHT_OUTLINE};
                border: none;
            }}
        """)
        main_layout.addWidget(self.v_separator)

        # Right Feature Container
        right_panel = QWidget(self)
        right_layout = QVBoxLayout(right_panel)
        right_layout.setContentsMargins(0, 0, 0, 0)
        right_layout.setSpacing(0)

        # 3. Main Tool Stack (Switched by Nav Rail or Footer)
        self.tool_stack = QStackedWidget(self)

        # Tool 0: Account Switcher (with Top Ribbon and 5 sub-pages)
        self.page_account_switcher = AccountSwitcherToolPage(self.controller, self)
        self.tool_stack.addWidget(self.page_account_switcher)

        # Tool 1: Tools Marketplace / Extensions
        self.page_marketplace = ToolsMarketplacePage(self)
        self.tool_stack.addWidget(self.page_marketplace)

        # Tool 2: System Settings
        self.page_system = SystemSettingsPage(self.controller, self)
        self.tool_stack.addWidget(self.page_system)

        right_layout.addWidget(self.tool_stack, stretch=1)
        main_layout.addWidget(right_panel, stretch=1)

        # Expose reference to nav rail's bottom-left System Settings button for tests/tray
        self.btn_system_settings = self.nav_rail.btn_system_settings

    def _open_system_settings_page(self) -> None:
        """Switches to System Settings tool and selects settings in nav rail."""
        self.nav_rail.set_current_index(2)

    def _on_tool_selected(self, index: int) -> None:
        if 0 <= index < self.tool_stack.count():
            self.tool_stack.setCurrentIndex(index)

    def _sync_status(self) -> None:
        try:
            status = self.controller.get_status()
            daemon_online = status.get("daemon_running", False)
            antigravity_running = status.get("antigravity_running", False)
            pid = status.get("antigravity_pid")

            self.nav_rail.set_daemon_status(daemon_online)
            self.nav_rail.set_antigravity_status(antigravity_running, pid)
        except Exception:
            self.nav_rail.set_daemon_status(False)
            self.nav_rail.set_antigravity_status(False)

    def _restore_from_tray(self) -> None:
        self.showNormal()
        self.activateWindow()

    def _open_settings_from_tray(self) -> None:
        self._restore_from_tray()
        self._open_system_settings_page()

    def closeEvent(self, event) -> None:
        if hasattr(self, "tray") and self.tray.is_available() and self.tray.isVisible():
            self.hide()
            self.tray.dispatch_notification(APP_TITLE, "Minimized to system tray. Running in background.")
            event.ignore()
        else:
            event.accept()

