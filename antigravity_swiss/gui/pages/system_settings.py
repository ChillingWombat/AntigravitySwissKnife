"""
System Settings Page.
====================
System configuration, daemon status, socket paths, and host process diagnostics.
"""

from __future__ import annotations

import os
from pathlib import Path
from typing import Optional
from PySide6.QtCore import Qt
from PySide6.QtWidgets import (
    QFrame,
    QHBoxLayout,
    QLabel,
    QLineEdit,
    QPushButton,
    QScrollArea,
    QVBoxLayout,
    QWidget,
)

from antigravity_swiss.core.constants import (
    DEFAULT_ANTIGRAVITY_BIN,
    DEFAULT_ANTIGRAVITY_CONFIG_DIR,
    DEFAULT_SWISS_CONFIG_DIR,
    MD3_ACCENT_PRIMARY,
    MD3_COLOR_HEALTHY,
    MD3_LIGHT_COLOR_HEALTHY,
    MD3_LIGHT_OUTLINE,
    MD3_LIGHT_SURFACE,
    MD3_LIGHT_SURFACE_CONTAINER,
    MD3_LIGHT_TEXT_PRIMARY,
    MD3_LIGHT_TEXT_SECONDARY,
    MD3_OUTLINE,
    MD3_SURFACE_CONTAINER,
    MD3_SURFACE_CONTAINER_HIGH,
    MD3_TEXT_PRIMARY,
    MD3_TEXT_SECONDARY,
)
from antigravity_swiss.ipc.controller import SwissKnifeController


class SystemSettingsPage(QWidget):
    """
    System Settings page displaying environment paths, IPC socket info, and daemon health.
    """

    def __init__(
        self,
        controller: SwissKnifeController,
        parent: Optional[QWidget] = None,
    ) -> None:
        super().__init__(parent)
        self.controller = controller
        self._init_ui()

    def _init_ui(self) -> None:
        main_layout = QVBoxLayout(self)
        main_layout.setContentsMargins(24, 16, 24, 24)
        main_layout.setSpacing(16)

        scroll = QScrollArea()
        scroll.setWidgetResizable(True)
        scroll.setFrameShape(QFrame.Shape.NoFrame)
        scroll.setStyleSheet("background: transparent;")

        container = QWidget()
        c_layout = QVBoxLayout(container)
        c_layout.setContentsMargins(0, 0, 0, 0)
        c_layout.setSpacing(20)

        # Header Info Card
        header_card = QFrame()
        header_card.setObjectName("headerCard")
        header_card.setStyleSheet(f"""
            QFrame#headerCard {{
                background-color: {MD3_LIGHT_SURFACE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 16px;
            }}
            QFrame#headerCard QLabel {{
                border: none;
                background: transparent;
            }}
        """)
        h_layout = QHBoxLayout(header_card)
        h_vbox = QVBoxLayout()
        h_vbox.setSpacing(4)

        t1 = QLabel("SYSTEM & PROCESS SETTINGS")
        t1.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 0.8px; border: none; background: transparent;")
        t2 = QLabel("Runtime diagnostics, IPC Unix domain sockets, and Antigravity process safety shield.")
        t2.setStyleSheet(f"font-size: 13px; color: {MD3_LIGHT_TEXT_PRIMARY}; border: none; background: transparent;")
        h_vbox.addWidget(t1)
        h_vbox.addWidget(t2)
        h_layout.addLayout(h_vbox)
        c_layout.addWidget(header_card)

        # Diagnostics & Safety Shield Card
        diag_card = QFrame()
        diag_card.setObjectName("diagCard")
        diag_card.setStyleSheet(f"""
            QFrame#diagCard {{
                background-color: {MD3_LIGHT_SURFACE};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 20px;
            }}
            QFrame#diagCard QLabel {{
                border: none;
                background: transparent;
            }}
        """)
        dc_layout = QVBoxLayout(diag_card)
        dc_layout.setSpacing(14)

        dc_title = QLabel("PROCESS SAFETY SHIELD & ENVIRONMENT")
        dc_title.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 0.8px; border: none; background: transparent;")
        dc_layout.addWidget(dc_title)

        shield_status = QLabel("Host Process Shield: ACTIVE (Host IDE PID protected against accidental signals)")
        shield_status.setStyleSheet(f"font-size: 12px; color: {MD3_LIGHT_COLOR_HEALTHY}; font-weight: 600; border: none; background: transparent;")
        dc_layout.addWidget(shield_status)

        # Fields
        env_items = [
            ("Antigravity Binary:", str(DEFAULT_ANTIGRAVITY_BIN)),
            ("Antigravity Config:", str(DEFAULT_ANTIGRAVITY_CONFIG_DIR)),
            ("Swiss Knife Config:", str(DEFAULT_SWISS_CONFIG_DIR)),
            ("Daemon IPC Socket:", f"{os.environ.get('XDG_RUNTIME_DIR', '/run/user/1000')}/antigravity-swiss/daemon.sock"),
        ]

        for label, val in env_items:
            row = QHBoxLayout()
            row.setSpacing(10)
            lbl = QLabel(label)
            lbl.setFixedWidth(160)
            lbl.setStyleSheet(f"color: {MD3_LIGHT_TEXT_SECONDARY}; font-size: 12px; font-weight: 500;")
            row.addWidget(lbl)

            inp = QLineEdit(val)
            inp.setReadOnly(True)
            inp.setStyleSheet(f"""
                QLineEdit {{
                    background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                    color: {MD3_LIGHT_TEXT_PRIMARY};
                    border: 1px solid {MD3_LIGHT_OUTLINE};
                    border-radius: 6px;
                    padding: 6px 10px;
                    font-family: monospace;
                    font-size: 11px;
                }}
            """)
            row.addWidget(inp)
            dc_layout.addLayout(row)

        c_layout.addWidget(diag_card)
        scroll.setWidget(container)
        main_layout.addWidget(scroll)
