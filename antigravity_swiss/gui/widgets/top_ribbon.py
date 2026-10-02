"""
Account Switcher Top Ribbon Sub-Navigation.
===========================================
Horizontal ribbon across the top of the Account Switcher page toggling between:
1. Quota Dashboard (Front Page)
2. Accounts & MFA Vault
3. Device Fingerprints
4. Brain Cache Manager
5. Switcher Settings
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
    MD3_ACCENT_PRIMARY,
    MD3_OUTLINE,
    MD3_SURFACE_CONTAINER,
    MD3_SURFACE_CONTAINER_HIGH,
    MD3_TEXT_PRIMARY,
    MD3_TEXT_SECONDARY,
)


class TopRibbon(QWidget):
    """
    Sub-navigation ribbon with Google Material 3 pill tabs.
    Emits `tab_selected(int)` when a tab is clicked.
    """

    tab_selected = Signal(int)

    TABS = [
        ("📊", "Quota Dashboard"),
        ("🔐", "Accounts & MFA Vault"),
        ("💻", "Device Fingerprints"),
        ("🧠", "Brain Cache Manager"),
        ("⚙️", "Switcher Settings"),
    ]

    TAB_SHORTCUTS: list[str] = ["Alt+1", "Alt+2", "Alt+3", "Alt+4", "Alt+5"]

    def __init__(self, parent: QWidget | None = None) -> None:
        super().__init__(parent)
        self._current_index = 0
        self._buttons: list[QPushButton] = []
        self._init_ui()

    @property
    def tab_count(self) -> int:
        return len(self._buttons)


    def _init_ui(self) -> None:
        self.setFixedHeight(58)
        layout = QHBoxLayout(self)
        layout.setContentsMargins(20, 10, 20, 10)
        layout.setSpacing(8)

        # Tab container frame
        tabs_frame = QFrame()
        tabs_frame.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_SURFACE_CONTAINER};
                border: 1px solid {MD3_OUTLINE};
                border-radius: 20px;
                padding: 2px 4px;
            }}
        """)
        tf_layout = QHBoxLayout(tabs_frame)
        tf_layout.setContentsMargins(4, 2, 4, 2)
        tf_layout.setSpacing(4)

        for idx, (icon, label) in enumerate(self.TABS):
            btn = QPushButton(f"{icon}  {label}")
            btn.setProperty("class", "ribbon-tab")
            btn.setStyleSheet(f"""
                QPushButton {{
                    background-color: transparent;
                    color: {MD3_TEXT_SECONDARY};
                    border: none;
                    border-radius: 16px;
                    padding: 6px 14px;
                    font-size: 13px;
                    font-weight: 500;
                }}
                QPushButton:hover {{
                    background-color: {MD3_SURFACE_CONTAINER_HIGH};
                    color: {MD3_TEXT_PRIMARY};
                }}
                QPushButton[active="true"] {{
                    background-color: #2b394f;
                    color: {MD3_ACCENT_PRIMARY};
                    font-weight: 600;
                }}
            """)
            btn.clicked.connect(lambda checked=False, i=idx: self.set_current_index(i))
            self._buttons.append(btn)
            tf_layout.addWidget(btn)

        layout.addWidget(tabs_frame)
        layout.addStretch()

        # Active Account Pill Badge on Right
        self._active_badge = QLabel("👤 No Active Account")
        self._active_badge.setStyleSheet(f"""
            QLabel {{
                background-color: {MD3_SURFACE_CONTAINER};
                color: {MD3_ACCENT_PRIMARY};
                border: 1px solid {MD3_OUTLINE};
                border-radius: 14px;
                padding: 5px 12px;
                font-size: 12px;
                font-weight: 500;
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
