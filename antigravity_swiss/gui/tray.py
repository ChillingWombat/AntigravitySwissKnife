"""
System Tray Integration (Feature F24).
======================================
Linux DBus StatusNotifierItem (SNI) and QSystemTrayIcon integration.
Features:
- Dynamic context menu with 1-click account rotation, settings, open dashboard, and exit
- Quota health status badges (Healthy, Warning, Exhausted / Critical)
- Desktop notifications via org.freedesktop.Notifications / QSystemTrayIcon
- Headless fallback resilience: graceful window-only or no-op fallback when tray unavailable
- Background minimize-to-tray handling
"""

from __future__ import annotations

import logging
from typing import Optional
from PySide6.QtCore import QObject, Signal, Slot
from PySide6.QtGui import QAction, QColor, QIcon, QPainter, QPixmap
from PySide6.QtWidgets import QApplication, QMenu, QSystemTrayIcon, QWidget

from antigravity_swiss.core.constants import (
    APP_TITLE,
    MD3_ACCENT_PRIMARY,
    MD3_COLOR_EXHAUSTED,
    MD3_COLOR_HEALTHY,
    MD3_COLOR_WARNING,
    MD3_SURFACE,
)
from antigravity_swiss.ipc.controller import SwissKnifeController

logger = logging.getLogger("antigravity_swiss.gui.tray")

# DBus StatusNotifierItem service and interface identifiers
SNI_SERVICE = "org.kde.StatusNotifierItem"
DBUS_NOTIFICATIONS_SERVICE = "org.freedesktop.Notifications"


class SwissKnifeTray(QSystemTrayIcon):
    """
    Desktop system tray manager with DBus SNI support and live context actions.
    """

    account_switch_requested = Signal(str)
    show_window_requested = Signal()
    open_settings_requested = Signal()

    SNI_SERVICE_NAME = SNI_SERVICE

    HEALTH_COLORS = {
        "HEALTHY": MD3_COLOR_HEALTHY,    # #81c995
        "WARNING": MD3_COLOR_WARNING,    # #fdd663
        "EXHAUSTED": MD3_COLOR_EXHAUSTED, # #f28b82
        "CRITICAL": MD3_COLOR_EXHAUSTED,  # #f28b82
    }

    def __init__(
        self,
        controller: Optional[SwissKnifeController] = None,
        parent: Optional[QWidget] = None,
    ) -> None:
        super().__init__(parent)
        if controller is None:
            from antigravity_swiss.core.config import SwissKnifeConfig
            from antigravity_swiss.ipc.controller import StandaloneController
            self.controller = StandaloneController(config=SwissKnifeConfig.load())
        else:
            self.controller = controller
        self._current_health = "HEALTHY"
        self._active_account: Optional[str] = None
        self.sni_service = SNI_SERVICE

        self._menu = QMenu(parent)
        self.setContextMenu(self._menu)
        self.activated.connect(self._on_tray_activated)

        self.update_icon()
        self.refresh_menu()

    @classmethod
    def is_tray_available(cls) -> bool:
        """Checks if desktop environment supports system tray / StatusNotifierItem."""
        return QSystemTrayIcon.isSystemTrayAvailable()

    def is_available(self) -> bool:
        """Instance check for tray availability."""
        return QSystemTrayIcon.isSystemTrayAvailable()

    def create_tray_pixmap(self, health_status: str = "HEALTHY") -> QPixmap:
        """Generates dynamic tray icon pixmap with health color badge."""
        pixmap = QPixmap(32, 32)
        pixmap.fill(QColor(0, 0, 0, 0))

        painter = QPainter(pixmap)
        painter.setRenderHint(QPainter.RenderHint.Antialiasing)

        # Draw base icon (Gemini sparkle blue outer circle)
        painter.setBrush(QColor(MD3_SURFACE))
        painter.setPen(QColor(MD3_ACCENT_PRIMARY))
        painter.drawEllipse(2, 2, 28, 28)

        # Center dot
        painter.setBrush(QColor(MD3_ACCENT_PRIMARY))
        painter.setPen(QColor(0, 0, 0, 0))
        painter.drawEllipse(9, 9, 14, 14)

        # Health badge indicator at bottom-right
        color_hex = self.HEALTH_COLORS.get(health_status.upper(), MD3_COLOR_HEALTHY)
        painter.setBrush(QColor(color_hex))
        painter.setPen(QColor(MD3_SURFACE))
        painter.drawEllipse(19, 19, 11, 11)

        painter.end()
        return pixmap

    def update_icon(self, health_status: Optional[str] = None) -> None:
        """Updates the system tray icon badge color."""
        if health_status:
            self._current_health = health_status.upper()
        pix = self.create_tray_pixmap(self._current_health)
        self.setIcon(QIcon(pix))
        self.setToolTip(f"{APP_TITLE} — Status: {self._current_health}")

    def refresh_menu(self) -> None:
        """Rebuilds the context menu dynamically with registered accounts and actions."""
        self._menu.clear()

        # Header Title (disabled action)
        title_act = self._menu.addAction(APP_TITLE)
        title_act.setEnabled(False)
        self._menu.addSeparator()

        # Open Window
        open_act = self._menu.addAction("Open Dashboard")
        open_act.triggered.connect(self._on_open_dashboard)

        # Settings
        settings_act = self._menu.addAction("System Settings")
        settings_act.triggered.connect(self._on_open_settings)

        # Refresh Quota
        refresh_act = self._menu.addAction("Refresh Quota Now")
        refresh_act.triggered.connect(self._on_refresh_quota)

        self._menu.addSeparator()

        # Accounts submenu for 1-click rotation
        switch_menu = self._menu.addMenu("Switch Account")
        try:
            status = self.controller.get_status()
            self._active_account = status.get("active_account")
            accounts = self.controller.list_accounts()

            if accounts:
                for acc in accounts:
                    email = acc.get("email", "")
                    if not email:
                        continue
                    is_active = (email == self._active_account)
                    prefix = "[Active] " if is_active else "  "
                    act = switch_menu.addAction(f"{prefix}{email}")
                    if is_active:
                        font = act.font()
                        font.setBold(True)
                        act.setFont(font)
                    act.triggered.connect(lambda checked=False, target=email: self._on_switch_account(target))
            else:
                empty_act = switch_menu.addAction("No Accounts Configured")
                empty_act.setEnabled(False)
        except Exception as exc:
            logger.warning("Could not list accounts for tray menu: %s", exc)

        self._menu.addSeparator()

        # Exit action
        exit_act = self._menu.addAction("Exit Swiss Knife")
        exit_act.triggered.connect(self._on_exit)

    def dispatch_notification(self, title: str, body: str, is_warning: bool = False) -> None:
        """Dispatches desktop notification toast via freedesktop DBus / QSystemTrayIcon."""
        try:
            icon = QSystemTrayIcon.MessageIcon.Warning if is_warning else QSystemTrayIcon.MessageIcon.Information
            if self.is_available() and self.isVisible():
                self.showMessage(title, body, icon, 4000)
            else:
                logger.info("Notification dispatched (headless/fallback): %s - %s", title, body)
        except Exception as exc:
            logger.warning("Failed to dispatch tray notification: %s", exc)

    def show_notification(self, title: str, body: str, is_warning: bool = False) -> None:
        """Alias for dispatch_notification."""
        self.dispatch_notification(title, body, is_warning)

    def update_quota_status(self, color_or_status: str) -> None:
        """Updates health color badge directly via hex color or status string."""
        for name, col in self.HEALTH_COLORS.items():
            if col.lower() == color_or_status.lower():
                self.update_icon(name)
                return
        self.update_icon(color_or_status)

    def set_accounts(self, accounts: list, active_email: Optional[str] = None) -> None:
        """Sets accounts list for context menu."""
        self._active_account = active_email
        self.refresh_menu()

    def _on_tray_activated(self, reason: QSystemTrayIcon.ActivationReason) -> None:
        if reason in (QSystemTrayIcon.ActivationReason.Trigger, QSystemTrayIcon.ActivationReason.DoubleClick):
            self.show_window_requested.emit()

    def _on_open_dashboard(self) -> None:
        self.show_window_requested.emit()

    def _on_open_settings(self) -> None:
        self.open_settings_requested.emit()

    def _on_refresh_quota(self) -> None:
        try:
            self.controller.poll_quota()
            self.dispatch_notification("Quota Updated", "Upstream model quotas refreshed successfully.")
        except Exception as exc:
            self.dispatch_notification("Quota Refresh Failed", str(exc), is_warning=True)

    def _on_switch_account(self, email: str) -> None:
        try:
            self.controller.switch_account(email, force=True, relaunch=True)
            self._active_account = email
            self.dispatch_notification("Account Switched", f"Switched active account to {email}")
            self.account_switch_requested.emit(email)
            self.refresh_menu()
        except Exception as exc:
            self.dispatch_notification("Switch Failed", str(exc), is_warning=True)

    def _on_exit(self) -> None:
        app = QApplication.instance()
        if app:
            app.quit()


# Canonical alias for architectural consistency
SystemTrayManager = SwissKnifeTray
