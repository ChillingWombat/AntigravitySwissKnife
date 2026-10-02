"""
Antigravity Swiss Knife GUI Page Views.
======================================
Tool views and Account Switcher sub-pages.
"""

from antigravity_swiss.gui.pages.account_switcher_tool import AccountSwitcherToolPage
from antigravity_swiss.gui.pages.brain_cache import BrainCachePage
from antigravity_swiss.gui.pages.fingerprints import DeviceFingerprintsPage
from antigravity_swiss.gui.pages.mfa_vault import MfaVaultPage
from antigravity_swiss.gui.pages.quota_dashboard import QuotaDashboardPage
from antigravity_swiss.gui.pages.switcher_settings import SwitcherSettingsPage
from antigravity_swiss.gui.pages.system_settings import SystemSettingsPage
from antigravity_swiss.gui.pages.tools_marketplace import ToolsMarketplacePage

__all__ = [
    "AccountSwitcherToolPage",
    "BrainCachePage",
    "DeviceFingerprintsPage",
    "MfaVaultPage",
    "QuotaDashboardPage",
    "SwitcherSettingsPage",
    "SystemSettingsPage",
    "ToolsMarketplacePage",
]
