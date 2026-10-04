"""
Account Switcher Top Ribbon Sub-Navigation (Minimalist Edition).
==============================================================
Horizontal ribbon across the top of the Account Switcher page:
1. Quota Dashboard
2. Accounts & MFA Vault
3. Device Fingerprints
4. Brain Cache Manager
5. Switcher Settings
Clean typographic design with zero emojis or noisy symbols.
"""

from __future__ import annotations

from PySide6.QtCore import Qt, Signal
from PySide6.QtWidgets import (
    QFrame,
    QHBoxLayout,
    QLabel,
    QPushButton,
    QWidget,
)

from antigravity_swiss.core.constants import (
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


class TopRibbon(QFrame):
    """
    Sub-navigation ribbon with minimalist pill tabs.
    Emits `tab_selected(int)` when a tab is clicked.
    """

    tab_selected = Signal(int)

    TABS = [
        "Dashboard",
        "Accounts && MFA",
        "Fingerprints",
        "Cache Manager",
        "Settings",
    ]

    TAB_SHORTCUTS: list[str] = ["Alt+1", "Alt+2", "Alt+3", "Alt+4", "Alt+5"]

    def __init__(self, parent: QWidget | None = None) -> None:
        super().__init__(parent)
        self.setAttribute(Qt.WidgetAttribute.WA_StyledBackground, True)
        self._current_index = 0
        self._buttons: list[QPushButton] = []
        self.setObjectName("topRibbon")
        self.setStyleSheet(f"""
            QFrame#topRibbon {{
                background-color: {MD3_LIGHT_SURFACE};
                border-bottom: 1px solid {MD3_LIGHT_OUTLINE};
            }}
        """)
        self._init_ui()

    @property
    def tab_count(self) -> int:
        return len(self._buttons)

    def _init_ui(self) -> None:
        self.setFixedHeight(54)
        layout = QHBoxLayout(self)
        layout.setContentsMargins(16, 6, 16, 6)
        layout.setSpacing(12)

        # Tab container frame (Google AI Studio Segmented Pill Track)
        tabs_frame = QFrame()
        tabs_frame.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER_HIGH};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 20px;
                padding: 2px;
            }}
        """)
        tf_layout = QHBoxLayout(tabs_frame)
        tf_layout.setContentsMargins(2, 2, 2, 2)
        tf_layout.setSpacing(2)

        for idx, label in enumerate(self.TABS):
            btn = QPushButton(label)
            btn.setProperty("class", "ribbon-tab")
            btn.setStyleSheet(f"""
                QPushButton {{
                    background-color: transparent;
                    color: {MD3_LIGHT_TEXT_SECONDARY};
                    border: none;
                    border-radius: 16px;
                    padding: 6px 16px;
                    font-size: 12px;
                    font-weight: 500;
                }}
                QPushButton:hover {{
                    background-color: rgba(255, 255, 255, 0.6);
                    color: {MD3_LIGHT_TEXT_PRIMARY};
                }}
                QPushButton[active="true"] {{
                    background-color: #ffffff;
                    color: {MD3_LIGHT_ACCENT_PRIMARY};
                    border: 1px solid {MD3_LIGHT_OUTLINE};
                    font-weight: 600;
                }}
            """)
            btn.clicked.connect(lambda checked=False, i=idx: self.set_current_index(i))
            self._buttons.append(btn)
            tf_layout.addWidget(btn)

        layout.addWidget(tabs_frame)
        layout.addStretch()

        # Active Account Pill Badge on Right (Google User Identity Chip)
        self._active_badge = QLabel("👤 No Active Account")
        self._active_badge.setStyleSheet(f"""
            QLabel {{
                background-color: #e8f0fe;
                color: {MD3_LIGHT_ACCENT_PRIMARY};
                border: 1px solid {MD3_LIGHT_ACCENT_CONTAINER};
                border-radius: 16px;
                padding: 6px 16px;
                font-size: 12px;
                font-weight: 600;
            }}
        """)
        layout.addWidget(self._active_badge)

        self._update_tab_styles()

    def set_current_index(self, index: int) -> None:
        if 0 <= index < len(self._buttons):
            self._current_index = index
            self._update_tab_styles()
            self.tab_selected.emit(index)

    def _update_tab_styles(self) -> None:
        for idx, btn in enumerate(self._buttons):
            is_active = (idx == self._current_index)
            btn.setProperty("active", "true" if is_active else "false")
            btn.style().unpolish(btn)
            btn.style().polish(btn)

    def set_active_account(self, email: str | None) -> None:
        if email:
            self._active_badge.setText(f"👤 {email}")
        else:
            self._active_badge.setText("👤 No Active Account")
