"""
Fixed Left Navigation Rail Widget.
=================================
Google Gemini / Material Design 3 style navigation rail for top-level tool switching:
- Account Switcher (Active suite tool)
- Tools Marketplace / Extensions
- System Settings
"""

from __future__ import annotations

from PySide6.QtCore import Qt, Signal
from PySide6.QtWidgets import (
    QFrame,
    QHBoxLayout,
    QLabel,
    QPushButton,
    QVBoxLayout,
    QWidget,
)

from antigravity_swiss.core.constants import (
    APP_TITLE,
    MD3_ACCENT_PRIMARY,
    MD3_COLOR_HEALTHY,
    MD3_OUTLINE,
    MD3_SURFACE_CONTAINER,
    MD3_TEXT_PRIMARY,
    MD3_TEXT_SECONDARY,
)


class NavigationRail(QWidget):
    """
    Fixed / Collapsible left vertical navigation panel.
    Emits `tool_selected(int)` when an item is clicked.
    """

    tool_selected = Signal(int)

    RAIL_WIDTH_EXPANDED: int = 220
    RAIL_WIDTH_COLLAPSED: int = 72

    def __init__(self, parent: QWidget | None = None, collapsed: bool = False) -> None:
        super().__init__(parent)
        self._collapsed = collapsed
        self.setFixedWidth(self.RAIL_WIDTH_COLLAPSED if collapsed else self.RAIL_WIDTH_EXPANDED)
        self._current_index = 0
        self._buttons: list[QPushButton] = []

        self._init_ui()

    def set_collapsed(self, collapsed: bool) -> None:
        """Toggles between 72px icon rail and 220px expanded panel."""
        self._collapsed = collapsed
        self.setFixedWidth(self.RAIL_WIDTH_COLLAPSED if collapsed else self.RAIL_WIDTH_EXPANDED)

    def is_collapsed(self) -> bool:
        return self._collapsed


    def _init_ui(self) -> None:
        layout = QVBoxLayout(self)
        layout.setContentsMargins(16, 24, 16, 20)
        layout.setSpacing(12)

        # Header Branding
        brand_layout = QHBoxLayout()
        brand_layout.setSpacing(10)

        # Gemini Sparkle Icon Label (✦)
        sparkle = QLabel("✦")
        sparkle.setStyleSheet(f"font-size: 22px; color: {MD3_ACCENT_PRIMARY}; font-weight: bold;")
        brand_layout.addWidget(sparkle)

        title_vbox = QVBoxLayout()
        title_vbox.setSpacing(2)
        title_lbl = QLabel("Antigravity")
        title_lbl.setStyleSheet(f"font-size: 15px; font-weight: 700; color: {MD3_TEXT_PRIMARY};")
        subtitle_lbl = QLabel("Swiss Knife")
        subtitle_lbl.setStyleSheet(f"font-size: 12px; font-weight: 500; color: {MD3_ACCENT_PRIMARY};")
        title_vbox.addWidget(title_lbl)
        title_vbox.addWidget(subtitle_lbl)

        brand_layout.addLayout(title_vbox)
        brand_layout.addStretch()
        layout.addLayout(brand_layout)

        # Separator line
        sep = QFrame()
        sep.setFrameShape(QFrame.Shape.HLine)
        sep.setStyleSheet(f"background-color: {MD3_OUTLINE}; max-height: 1px; margin-top: 10px; margin-bottom: 12px;")
        layout.addWidget(sep)

        # Section Label
        tools_lbl = QLabel("MODULES")
        tools_lbl.setStyleSheet(f"font-size: 10px; font-weight: 700; color: {MD3_TEXT_SECONDARY}; letter-spacing: 1px; padding-left: 6px;")
        layout.addWidget(tools_lbl)

        # Navigation Items
        items = [
            ("🔄", "Account Switcher", 0),
            ("🧩", "Tools Marketplace", 1),
            ("⚙️", "System Settings", 2),
        ]

        for icon, label, idx in items:
            btn = QPushButton(f"  {icon}   {label}")
            btn.setProperty("class", "nav-rail-btn")
            btn.setStyleSheet(f"""
                QPushButton {{
                    text-align: left;
                    padding: 12px 14px;
                    border-radius: 12px;
                    font-size: 13px;
                    font-weight: 500;
                    color: {MD3_TEXT_SECONDARY};
                    background-color: transparent;
                    border: none;
                }}
                QPushButton:hover {{
                    background-color: #282a2c;
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
            layout.addWidget(btn)

        layout.addStretch()

        # Bottom System Status Card
        status_card = QFrame()
        status_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_SURFACE_CONTAINER};
                border: 1px solid {MD3_OUTLINE};
                border-radius: 12px;
                padding: 10px;
            }}
        """)
        sc_layout = QVBoxLayout(status_card)
        sc_layout.setContentsMargins(8, 8, 8, 8)
        sc_layout.setSpacing(6)

        self._daemon_lbl = QLabel("● Daemon Active")
        self._daemon_lbl.setStyleSheet(f"color: {MD3_COLOR_HEALTHY}; font-size: 11px; font-weight: 600;")
        sc_layout.addWidget(self._daemon_lbl)

        self._host_lbl = QLabel("● Antigravity 2.0")
        self._host_lbl.setStyleSheet(f"color: {MD3_TEXT_SECONDARY}; font-size: 11px;")
        sc_layout.addWidget(self._host_lbl)

        layout.addWidget(status_card)

        self._update_button_styles()

    def set_current_index(self, index: int) -> None:
        if 0 <= index < len(self._buttons):
            self._current_index = index
            self._update_button_styles()
            self.tool_selected.emit(index)

    def _update_button_styles(self) -> None:
        for idx, btn in enumerate(self._buttons):
            is_active = (idx == self._current_index)
            btn.setProperty("active", "true" if is_active else "false")
            btn.style().unpolish(btn)
            btn.style().polish(btn)

    def set_daemon_status(self, is_running: bool) -> None:
        if is_running:
            self._daemon_lbl.setText("● Daemon Connected")
            self._daemon_lbl.setStyleSheet(f"color: {MD3_COLOR_HEALTHY}; font-size: 11px; font-weight: 600;")
        else:
            self._daemon_lbl.setText("○ Standalone Mode")
            self._daemon_lbl.setStyleSheet(f"color: {MD3_TEXT_SECONDARY}; font-size: 11px; font-weight: 500;")

    def set_antigravity_status(self, is_running: bool, pid: int | None = None) -> None:
        if is_running:
            self._host_lbl.setText(f"● Host Running (PID {pid})")
            self._host_lbl.setStyleSheet(f"color: {MD3_COLOR_HEALTHY}; font-size: 11px;")
        else:
            self._host_lbl.setText("○ Host Idle")
            self._host_lbl.setStyleSheet(f"color: {MD3_TEXT_SECONDARY}; font-size: 11px;")
