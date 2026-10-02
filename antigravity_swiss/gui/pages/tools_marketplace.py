"""
Tools Marketplace & Extensions Page.
===================================
Hub for Swiss Knife modules and future extensible tools.
"""

from __future__ import annotations

from typing import Optional
from PySide6.QtCore import Qt
from PySide6.QtWidgets import (
    QFrame,
    QGridLayout,
    QHBoxLayout,
    QLabel,
    QPushButton,
    QScrollArea,
    QVBoxLayout,
    QWidget,
)

from antigravity_swiss.core.constants import (
    MD3_ACCENT_PRIMARY,
    MD3_COLOR_HEALTHY,
    MD3_OUTLINE,
    MD3_SURFACE_CONTAINER,
    MD3_SURFACE_CONTAINER_HIGH,
    MD3_TEXT_PRIMARY,
    MD3_TEXT_SECONDARY,
)


class ToolsMarketplacePage(QWidget):
    """
    Marketplace showcasing Swiss Knife modules and future slots.
    """

    def __init__(self, parent: Optional[QWidget] = None) -> None:
        super().__init__(parent)
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
        header_card.setStyleSheet(f"""
            QFrame {{
                background-color: {MD3_SURFACE_CONTAINER};
                border: 1px solid {MD3_OUTLINE};
                border-radius: 16px;
                padding: 16px;
            }}
        """)
        h_layout = QHBoxLayout(header_card)
        h_vbox = QVBoxLayout()
        h_vbox.setSpacing(4)

        t1 = QLabel("SWISS KNIFE TOOLS MARKETPLACE")
        t1.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_TEXT_SECONDARY}; letter-spacing: 1px;")
        t2 = QLabel("Extend your Antigravity companion with native productivity and automation modules.")
        t2.setStyleSheet(f"font-size: 13px; color: {MD3_TEXT_PRIMARY};")
        h_vbox.addWidget(t1)
        h_vbox.addWidget(t2)
        h_layout.addLayout(h_vbox)
        c_layout.addWidget(header_card)

        # Grid of Modules
        grid = QGridLayout()
        grid.setSpacing(16)

        modules = [
            ("🔄", "Account Switcher", "Atomic zero-loss OAuth credential rotation with session preservation & MFA.", "ACTIVE", MD3_COLOR_HEALTHY),
            ("🧠", "Brain Cache Optimizer", "Deep scanner & safe disk reclamation for ~/.gemini/ with cascade immunity.", "INSTALLED", MD3_COLOR_HEALTHY),
            ("💻", "Fingerprint Virtualizer", "Anti-ban per-account hardware & UUID profile virtualization.", "INSTALLED", MD3_COLOR_HEALTHY),
            ("📊", "Token Cost Tracker", "Real-time consumption analytics and quota burn rate forecasting.", "COMING SOON", MD3_TEXT_SECONDARY),
            ("📑", "Prompt Bloat Compressor", "Automatic context bloat stripper and JSON-RPC deduplicator.", "BETA", MD3_ACCENT_PRIMARY),
            ("🔖", "Session Bookmarker", "Preserve named workspace states and quick-jump between tasks.", "COMING SOON", MD3_TEXT_SECONDARY),
        ]

        for idx, (icon, title, desc, badge, badge_color) in enumerate(modules):
            row = idx // 2
            col = idx % 2

            card = QFrame()
            card.setStyleSheet(f"""
                QFrame {{
                    background-color: {MD3_SURFACE_CONTAINER};
                    border: 1px solid {MD3_OUTLINE};
                    border-radius: 16px;
                    padding: 18px;
                }}
                QFrame:hover {{
                    border-color: {MD3_ACCENT_PRIMARY};
                }}
            """)
            card_vbox = QVBoxLayout(card)
            card_vbox.setSpacing(10)

            top_row = QHBoxLayout()
            icon_lbl = QLabel(icon)
            icon_lbl.setStyleSheet("font-size: 24px;")
            top_row.addWidget(icon_lbl)

            title_lbl = QLabel(title)
            title_lbl.setStyleSheet(f"font-size: 15px; font-weight: 700; color: {MD3_TEXT_PRIMARY};")
            top_row.addWidget(title_lbl)
            top_row.addStretch()

            badge_lbl = QLabel(badge)
            badge_lbl.setStyleSheet(f"""
                QLabel {{
                    background-color: {MD3_SURFACE_CONTAINER_HIGH};
                    color: {badge_color};
                    border: 1px solid {MD3_OUTLINE};
                    border-radius: 10px;
                    padding: 3px 8px;
                    font-size: 10px;
                    font-weight: 700;
                }}
            """)
            top_row.addWidget(badge_lbl)
            card_vbox.addLayout(top_row)

            desc_lbl = QLabel(desc)
            desc_lbl.setWordWrap(True)
            desc_lbl.setStyleSheet(f"font-size: 12px; color: {MD3_TEXT_SECONDARY}; line-height: 1.4;")
            card_vbox.addWidget(desc_lbl)

            card_vbox.addStretch()
            grid.addWidget(card, row, col)

        c_layout.addLayout(grid)
        scroll.setWidget(container)
        main_layout.addWidget(scroll)
