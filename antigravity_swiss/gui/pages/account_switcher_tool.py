"""
Account Switcher Tool Page.
==========================
Primary Stage 1 tool page containing:
- Top Ribbon sub-navigation bar
- 5 Embedded sub-pages:
  1. Quota Dashboard (Front Page)
  2. Accounts & MFA Vault
  3. Device Fingerprints
  4. Brain Cache Manager
  5. Switcher Settings
"""

from __future__ import annotations

from typing import Optional
from PySide6.QtCore import Qt, Signal
from PySide6.QtWidgets import (
    QStackedWidget,
    QVBoxLayout,
    QWidget,
)

from antigravity_swiss.gui.pages.brain_cache import BrainCachePage
from antigravity_swiss.gui.pages.fingerprints import DeviceFingerprintsPage
from antigravity_swiss.gui.pages.mfa_vault import MfaVaultPage
from antigravity_swiss.gui.pages.quota_dashboard import QuotaDashboardPage
from antigravity_swiss.gui.pages.switcher_settings import SwitcherSettingsPage
from antigravity_swiss.gui.widgets.top_ribbon import TopRibbon
from antigravity_swiss.ipc.controller import SwissKnifeController


class AccountSwitcherToolPage(QWidget):
    """
    Main Account Switcher Tool container page with top ribbon and sub-page stack.
    """

    account_switched = Signal(str)

    def __init__(
        self,
        controller: SwissKnifeController,
        parent: Optional[QWidget] = None,
    ) -> None:
        super().__init__(parent)
        self.controller = controller
        self._init_ui()

    def _init_ui(self) -> None:
        layout = QVBoxLayout(self)
        layout.setContentsMargins(0, 0, 0, 0)
        layout.setSpacing(0)

        # Top Ribbon
        self.ribbon = TopRibbon(self)
        self.ribbon.tab_selected.connect(self._on_ribbon_tab_selected)
        layout.addWidget(self.ribbon)

        # Sub-page Stack
        self.stack = QStackedWidget(self)

        # 0: Quota Dashboard
        self.page_dashboard = QuotaDashboardPage(self.controller, self)
        self.page_dashboard.account_switched.connect(self._on_account_switched)
        self.stack.addWidget(self.page_dashboard)

        # 1: Accounts & MFA Vault
        self.page_mfa = MfaVaultPage(self.controller, self)
        self.page_mfa.account_updated.connect(self._on_mfa_updated)
        self.stack.addWidget(self.page_mfa)

        # 2: Device Fingerprints
        self.page_fingerprints = DeviceFingerprintsPage(self.controller, self)
        self.stack.addWidget(self.page_fingerprints)

        # 3: Brain Cache Manager
        self.page_brain_cache = BrainCachePage(self.controller, self)
        self.stack.addWidget(self.page_brain_cache)

        # 4: Switcher Settings
        self.page_settings = SwitcherSettingsPage(self.controller, self)
        self.stack.addWidget(self.page_settings)

        layout.addWidget(self.stack)

        # Set initial active account on ribbon
        self._sync_active_account()

    def _on_ribbon_tab_selected(self, index: int) -> None:
        if 0 <= index < self.stack.count():
            self.stack.setCurrentIndex(index)
            # Trigger refresh when switching to certain pages
            if index == 0:
                self.page_dashboard.refresh_quota()
            elif index == 1:
                self.page_mfa.load_accounts()
            elif index == 2:
                self.page_fingerprints.load_data()
            elif index == 3:
                self.page_brain_cache.scan_cache()
            elif index == 4:
                self.page_settings.load_config()

    def _on_account_switched(self, email: str) -> None:
        self.ribbon.set_active_account(email)
        self.page_mfa.load_accounts()
        self.page_fingerprints.load_data()
        self.account_switched.emit(email)

    def _on_mfa_updated(self, email: str) -> None:
        self.page_dashboard.load_data()

    def _sync_active_account(self) -> None:
        try:
            status = self.controller.get_status()
            active = status.get("active_account")
            self.ribbon.set_active_account(active)
        except Exception:
            pass
