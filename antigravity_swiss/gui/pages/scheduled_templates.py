"""
Scheduled Templates Catalog Page.
==================================
Preset autonomous agent templates inspired by Devin, Codex, and Claude Code:
- Daily News & Content Digest
- Financial Portfolio & Market Tracker
- Daily Agenda & Calendar Planner
- Automated Code Review & Standup
- Security & Vulnerability Scanner
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
    MD3_LIGHT_ACCENT_CONTAINER,
    MD3_LIGHT_ACCENT_PRIMARY,
    MD3_LIGHT_OUTLINE,
    MD3_LIGHT_SURFACE,
    MD3_LIGHT_SURFACE_CONTAINER,
    MD3_LIGHT_SURFACE_CONTAINER_HIGH,
    MD3_LIGHT_TEXT_PRIMARY,
    MD3_LIGHT_TEXT_SECONDARY,
)


TEMPLATES = [
    {
        "id": "news_digest",
        "title": "Daily News & Research Digest",
        "category": "Daily Assistant",
        "description": "Extract, summarize, and synthesize top industry news, arXiv preprints, and feeds from selected sources every morning.",
        "cadence": "Every morning at 08:00 AM",
    },
    {
        "id": "market_monitor",
        "title": "Market & Portfolio Monitor",
        "category": "Daily Assistant",
        "description": "Track watchlist stocks, crypto positions, currency rates, and macroeconomic indicators with automated threshold alerts.",
        "cadence": "Weekdays at 09:30 AM",
    },
    {
        "id": "calendar_agenda",
        "title": "Day Planner & Calendar Organizer",
        "category": "Daily Assistant",
        "description": "Review daily appointments, extract actionable tasks, and prepare context briefing notes for scheduled meetings.",
        "cadence": "Every day at 07:30 AM",
    },
    {
        "id": "code_review",
        "title": "PR Review & Standup Bot",
        "category": "Software Engineering",
        "description": "Inspect newly opened git PRs, run automated linting and security scans, and prepare succinct standup reports.",
        "cadence": "Daily at 09:00 AM",
    },
    {
        "id": "sec_audit",
        "title": "Vulnerability & Dependency Audit",
        "category": "DevOps & Security",
        "description": "Perform scheduled audit of project dependencies, identify CVE vulnerabilities, and propose automated patch PRs.",
        "cadence": "Weekly on Sundays",
    },
    {
        "id": "test_refactor",
        "title": "Continuous Test Coverage Refactor",
        "category": "Software Engineering",
        "description": "Analyze uncovered branches in test suites, synthesize regression unit tests, and verify 100% pass rates.",
        "cadence": "Bi-weekly on Wednesdays",
    },
]


class ScheduledTemplatesPage(QWidget):
    """
    Catalog of Scheduled Agent Templates.
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
        c_layout.setSpacing(16)

        # Header Title
        title_box = QVBoxLayout()
        title_box.setSpacing(4)
        title = QLabel("SCHEDULED AGENT TEMPLATES")
        title.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 1px;")
        subtitle = QLabel("Turn recurring tasks, daily digests, and developer workflows into scheduled agent runs.")
        subtitle.setStyleSheet(f"font-size: 12px; color: {MD3_LIGHT_TEXT_SECONDARY};")
        title_box.addWidget(title)
        title_box.addWidget(subtitle)
        c_layout.addLayout(title_box)

        # Grid of Cards
        grid = QGridLayout()
        grid.setSpacing(16)

        for i, t in enumerate(TEMPLATES):
            card = QFrame()
            card.setObjectName("templateCard")
            card.setStyleSheet(f"""
                QFrame#templateCard {{
                    background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                    border: 1px solid {MD3_LIGHT_OUTLINE};
                    border-radius: 12px;
                    padding: 16px;
                }}
                QFrame#templateCard QLabel {{
                    border: none;
                    background: transparent;
                }}
            """)
            card_layout = QVBoxLayout(card)
            card_layout.setContentsMargins(16, 16, 16, 16)
            card_layout.setSpacing(8)

            cat_lbl = QLabel(t["category"].upper())
            cat_lbl.setStyleSheet(f"font-size: 10px; font-weight: 700; color: {MD3_LIGHT_ACCENT_PRIMARY}; letter-spacing: 0.8px; border: none; background: transparent;")
            card_layout.addWidget(cat_lbl)

            title_lbl = QLabel(t["title"])
            title_lbl.setStyleSheet(f"font-size: 14px; font-weight: 600; color: {MD3_LIGHT_TEXT_PRIMARY}; border: none; background: transparent;")
            card_layout.addWidget(title_lbl)

            desc_lbl = QLabel(t["description"])
            desc_lbl.setWordWrap(True)
            desc_lbl.setStyleSheet(f"font-size: 11px; color: {MD3_LIGHT_TEXT_SECONDARY}; line-height: 1.4; border: none; background: transparent;")
            card_layout.addWidget(desc_lbl)

            card_layout.addStretch()

            bottom_row = QHBoxLayout()
            cadence_lbl = QLabel(t["cadence"])
            cadence_lbl.setStyleSheet(f"font-size: 10px; color: {MD3_LIGHT_TEXT_SECONDARY}; border: none; background: transparent;")
            bottom_row.addWidget(cadence_lbl)
            bottom_row.addStretch()

            btn = QPushButton("Schedule")
            btn.setStyleSheet(f"""
                QPushButton {{
                    background-color: {MD3_LIGHT_SURFACE_CONTAINER_HIGH};
                    color: {MD3_LIGHT_ACCENT_PRIMARY};
                    border: 1px solid {MD3_LIGHT_OUTLINE};
                    border-radius: 6px;
                    padding: 4px 12px;
                    font-size: 11px;
                    font-weight: 600;
                }}
                QPushButton:hover {{
                    background-color: {MD3_LIGHT_ACCENT_CONTAINER};
                }}
            """)
            btn.clicked.connect(lambda checked=False, item=t: self._open_template_dialog(item))
            bottom_row.addWidget(btn)
            card_layout.addLayout(bottom_row)

            row = i // 2
            col = i % 2
            grid.addWidget(card, row, col)

        c_layout.addLayout(grid)
        c_layout.addStretch()

        scroll.setWidget(container)
        main_layout.addWidget(scroll)

    def _open_template_dialog(self, template: dict) -> None:
        """Opens popup dialog to customize template prompt and schedule settings."""
        from PySide6.QtWidgets import QDialog, QLineEdit, QTextEdit, QDialogButtonBox
        dlg = QDialog(self)
        dlg.setWindowTitle(f"Configure {template.get('title')}")
        dlg.setFixedSize(540, 420)
        dlg.setStyleSheet(f"""
            QDialog {{
                background-color: {MD3_LIGHT_SURFACE};
                color: {MD3_LIGHT_TEXT_PRIMARY};
            }}
        """)
        d_layout = QVBoxLayout(dlg)
        d_layout.setContentsMargins(20, 20, 20, 20)
        d_layout.setSpacing(12)

        header = QLabel(f"Schedule: {template.get('title')}")
        header.setStyleSheet("font-size: 15px; font-weight: 600;")
        d_layout.addWidget(header)

        cad_lbl = QLabel("Execution Cadence:")
        cad_lbl.setStyleSheet("font-size: 11px; font-weight: 600; color: #5f6368;")
        d_layout.addWidget(cad_lbl)

        cad_input = QLineEdit(template.get("cadence", ""))
        cad_input.setStyleSheet(f"padding: 6px; border: 1px solid {MD3_LIGHT_OUTLINE}; border-radius: 6px;")
        d_layout.addWidget(cad_input)

        prompt_lbl = QLabel("Custom Instructions & Agent Prompt:")
        prompt_lbl.setStyleSheet("font-size: 11px; font-weight: 600; color: #5f6368;")
        d_layout.addWidget(prompt_lbl)

        prompt_edit = QTextEdit()
        prompt_edit.setPlainText(f"Execute scheduled workflow for {template.get('title')}:\n- {template.get('description')}")
        prompt_edit.setStyleSheet(f"border: 1px solid {MD3_LIGHT_OUTLINE}; border-radius: 6px; padding: 6px;")
        d_layout.addWidget(prompt_edit)

        buttons = QDialogButtonBox(QDialogButtonBox.StandardButton.Ok | QDialogButtonBox.StandardButton.Cancel)
        buttons.accepted.connect(dlg.accept)
        buttons.rejected.connect(dlg.reject)
        d_layout.addWidget(buttons)

        dlg.exec()
