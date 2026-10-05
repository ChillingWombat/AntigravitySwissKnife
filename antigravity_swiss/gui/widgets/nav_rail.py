"""
Fixed Left Navigation Rail Widget (Minimalist Edition).
======================================================
Minimalist navigation rail for top-level tool switching:
- Account Switcher (Active suite tool)
- Tools Marketplace / Extensions
- System Settings
Clean typographic design with zero emojis or noisy symbols.
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
    MD3_COLOR_HEALTHY,
    MD3_LIGHT_ACCENT_CONTAINER,
    MD3_LIGHT_ACCENT_ON_CONTAINER,
    MD3_LIGHT_ACCENT_PRIMARY,
    MD3_LIGHT_COLOR_HEALTHY,
    MD3_LIGHT_OUTLINE,
    MD3_LIGHT_SURFACE,
    MD3_LIGHT_SURFACE_CONTAINER,
    MD3_LIGHT_SURFACE_CONTAINER_HIGH,
    MD3_LIGHT_TEXT_PRIMARY,
    MD3_LIGHT_TEXT_SECONDARY,
    MD3_TEXT_SECONDARY,
)


class NavigationRail(QFrame):
    """
    Fixed / Collapsible left vertical navigation panel.
    Emits `tool_selected(int)` when an item is clicked.
    """

    tool_selected = Signal(int)

    RAIL_WIDTH_EXPANDED: int = 220
    RAIL_WIDTH_COLLAPSED: int = 72

    def __init__(self, parent: QWidget | None = None, collapsed: bool = False) -> None:
        super().__init__(parent)
        self.setAttribute(Qt.WidgetAttribute.WA_StyledBackground, True)
        self._collapsed = collapsed
        self.setFixedWidth(self.RAIL_WIDTH_COLLAPSED if collapsed else self.RAIL_WIDTH_EXPANDED)
        self._current_index = 0
        self._buttons: list[QPushButton] = []
        self.setObjectName("navigationRail")
        self.setStyleSheet(f"""
            QFrame#navigationRail {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                border: none;
            }}
        """)

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
        layout.setSpacing(8)

        # Header Branding (Pure minimalist typography, no icons/sparkles)
        title_vbox = QVBoxLayout()
        title_vbox.setSpacing(2)
        title_lbl = QLabel("Antigravity")
        title_lbl.setAlignment(Qt.AlignmentFlag.AlignCenter)
        title_lbl.setStyleSheet(f"font-size: 16px; font-weight: 700; color: {MD3_LIGHT_TEXT_PRIMARY}; letter-spacing: -0.3px;")
        subtitle_lbl = QLabel("Swiss Knife")
        subtitle_lbl.setAlignment(Qt.AlignmentFlag.AlignCenter)
        subtitle_lbl.setStyleSheet(f"font-size: 11px; font-weight: 600; color: {MD3_LIGHT_ACCENT_PRIMARY}; text-transform: uppercase; letter-spacing: 1px;")
        title_vbox.addWidget(title_lbl)
        title_vbox.addWidget(subtitle_lbl)
        layout.addLayout(title_vbox)

        # Separator line
        sep = QFrame()
        sep.setFrameShape(QFrame.Shape.HLine)
        sep.setStyleSheet(f"background-color: {MD3_LIGHT_OUTLINE}; max-height: 1px; margin-top: 14px; margin-bottom: 8px;")
        layout.addWidget(sep)

        # Navigation Items (Matching Google Web App)
        items = [
            ("👤 Account Switcher", 0),
            ("🧠 Custom Models", 1),
            ("⚡ App Enhancements", 2),
            ("🕒 Task Automations", 3),
            ("▦ Tools Marketplace", 4),
            ("🗄 Archived Projects", 5),
        ]

        for label, idx in items:
            btn = QPushButton(label)
            btn.setProperty("class", "nav-rail-btn")
            btn.setStyleSheet(f"""
                QPushButton {{
                    text-align: left;
                    padding: 10px 16px;
                    border-radius: 20px;
                    font-size: 13px;
                    font-weight: 500;
                    color: {MD3_LIGHT_TEXT_SECONDARY};
                    background-color: transparent;
                    border: none;
                }}
                QPushButton:hover {{
                    background-color: {MD3_LIGHT_SURFACE_CONTAINER_HIGH};
                    color: {MD3_LIGHT_TEXT_PRIMARY};
                }}
                QPushButton[active="true"] {{
                    background-color: {MD3_LIGHT_ACCENT_CONTAINER};
                    color: {MD3_LIGHT_ACCENT_ON_CONTAINER};
                    font-weight: 600;
                }}
            """)
            btn.clicked.connect(lambda checked=False, i=idx: self.set_current_index(i))
            self._buttons.append(btn)
            layout.addWidget(btn)

        layout.addStretch()

        # Bottom System Status Card (Sleek Google AI Studio tonal container)
        status_card = QFrame()
        status_card.setObjectName("statusCard")
        status_card.setStyleSheet(f"""
            QFrame#statusCard {{
                background-color: {MD3_LIGHT_SURFACE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 10px 14px;
            }}
            QFrame#statusCard QLabel {{
                background: transparent;
                border: none;
            }}
        """)
        sc_layout = QVBoxLayout(status_card)
        sc_layout.setContentsMargins(0, 0, 0, 0)
        sc_layout.setSpacing(4)

        self._daemon_lbl = QLabel("✔ Daemon Active")
        self._daemon_lbl.setStyleSheet(f"color: {MD3_LIGHT_COLOR_HEALTHY}; font-size: 11px; font-weight: 600;")
        sc_layout.addWidget(self._daemon_lbl)

        self._host_lbl = QLabel("🛡 Host Idle")
        self._host_lbl.setStyleSheet(f"color: {MD3_LIGHT_TEXT_SECONDARY}; font-size: 11px;")
        sc_layout.addWidget(self._host_lbl)

        layout.addWidget(status_card)

        # Dedicated System Settings button pinned at bottom-left of whole GUI
        self.btn_system_settings = QPushButton("⚙ System Settings")
        self.btn_system_settings.setObjectName("btnNavSystemSettings")
        self.btn_system_settings.setCursor(Qt.CursorShape.PointingHandCursor)
        self.btn_system_settings.setStyleSheet(f"""
            QPushButton#btnNavSystemSettings {{
                text-align: left;
                padding: 10px 16px;
                border-radius: 20px;
                font-size: 13px;
                font-weight: 500;
                color: {MD3_LIGHT_TEXT_SECONDARY};
                background-color: transparent;
                border: none;
            }}
            QPushButton#btnNavSystemSettings:hover {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER_HIGH};
                color: {MD3_LIGHT_TEXT_PRIMARY};
            }}
            QPushButton#btnNavSystemSettings[active="true"] {{
                background-color: {MD3_LIGHT_ACCENT_CONTAINER};
                color: {MD3_LIGHT_ACCENT_ON_CONTAINER};
                font-weight: 600;
            }}
        """)
        self.btn_system_settings.clicked.connect(lambda: self.set_current_index(6))
        layout.addWidget(self.btn_system_settings)

        self._update_button_styles()

    def set_current_index(self, index: int) -> None:
        self._current_index = index
        self._update_button_styles()
        self.tool_selected.emit(index)

    def _update_button_styles(self) -> None:
        for idx, btn in enumerate(self._buttons):
            is_active = (idx == self._current_index)
            btn.setProperty("active", "true" if is_active else "false")
            btn.style().unpolish(btn)
            btn.style().polish(btn)

        is_settings_active = (self._current_index == 6)
        self.btn_system_settings.setProperty("active", "true" if is_settings_active else "false")
        self.btn_system_settings.style().unpolish(self.btn_system_settings)
        self.btn_system_settings.style().polish(self.btn_system_settings)

    def set_daemon_status(self, is_running: bool) -> None:
        if is_running:
            self._daemon_lbl.setText("✔ Daemon Active")
            self._daemon_lbl.setStyleSheet(f"color: {MD3_LIGHT_COLOR_HEALTHY}; font-size: 11px; font-weight: 600;")
        else:
            self._daemon_lbl.setText("● Standalone Mode")
            self._daemon_lbl.setStyleSheet(f"color: {MD3_LIGHT_TEXT_SECONDARY}; font-size: 11px; font-weight: 500;")

    def set_antigravity_status(self, is_running: bool, pid: int | None = None) -> None:
        if is_running and pid:
            self._host_lbl.setText(f"🛡 Host PID {pid}")
            self._host_lbl.setStyleSheet(f"color: {MD3_LIGHT_TEXT_SECONDARY}; font-size: 11px;")
        elif is_running:
            self._host_lbl.setText("🛡 Host Running")
            self._host_lbl.setStyleSheet(f"color: {MD3_LIGHT_TEXT_SECONDARY}; font-size: 11px;")
        else:
            self._host_lbl.setText("Host Idle")
            self._host_lbl.setStyleSheet(f"color: {MD3_LIGHT_TEXT_SECONDARY}; font-size: 11px;")
