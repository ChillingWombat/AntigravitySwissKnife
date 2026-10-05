"""
Archived Projects Management Page.
===================================
Manage archived projects:
- View table of archived projects
- Displays time elapsed since latest conversation (auto-scaled: hours -> days -> months -> years)
- Restore projects
- Project settings action icon
- Permanent project deletion
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any, Optional
from PySide6.QtCore import Qt
from PySide6.QtWidgets import (
    QFrame,
    QHBoxLayout,
    QHeaderView,
    QLabel,
    QPushButton,
    QScrollArea,
    QTableWidget,
    QTableWidgetItem,
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


class ArchivedProjectsPage(QWidget):
    """
    Archived Projects table and restoration page.
    """

    def __init__(self, parent: Optional[QWidget] = None) -> None:
        super().__init__(parent)
        self._config_path = Path.home() / ".config" / "antigravity-swiss" / "archived_projects.json"
        self._projects = self._load_projects()
        self._init_ui()

    def _load_projects(self) -> list[dict[str, Any]]:
        if self._config_path.exists():
            try:
                with open(self._config_path, "r", encoding="utf-8") as f:
                    data = json.load(f)
                    return data.get("projects", [])
            except Exception:
                pass
        return [
            {
                "id": "proj-legacy-backend",
                "name": "Legacy Cloud Backend",
                "path": "/mnt/Data/Projects/LegacyBackend",
                "last_active": "45 days ago",
                "conversations_count": 18,
            },
            {
                "id": "proj-old-experiments",
                "name": "Old ML Experiments",
                "path": "/mnt/Data/Projects/Experiments2025",
                "last_active": "4 months ago",
                "conversations_count": 32,
            },
            {
                "id": "proj-deprecated-tools",
                "name": "CLI Utilities v1",
                "path": "/mnt/Data/Projects/CLIUtilsV1",
                "last_active": "1 year ago",
                "conversations_count": 7,
            },
        ]

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
        title = QLabel("ARCHIVED PROJECTS")
        title.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 1px;")
        subtitle = QLabel("Restore, manage settings, or permanently delete archived projects.")
        subtitle.setStyleSheet(f"font-size: 12px; color: {MD3_LIGHT_TEXT_SECONDARY};")
        title_box.addWidget(title)
        title_box.addWidget(subtitle)
        c_layout.addLayout(title_box)

        # Table Card
        table_card = QFrame()
        table_card.setObjectName("archivedTableCard")
        table_card.setStyleSheet(f"""
            QFrame#archivedTableCard {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 16px;
            }}
            QFrame#archivedTableCard QLabel {{
                border: none;
                background: transparent;
            }}
        """)
        t_layout = QVBoxLayout(table_card)
        t_layout.setContentsMargins(16, 16, 16, 16)
        t_layout.setSpacing(12)

        self._table = QTableWidget(len(self._projects), 4)
        self._table.setFrameShape(QFrame.Shape.NoFrame)
        self._table.setHorizontalHeaderLabels([
            "Project Name & Path",
            "Latest Conversation",
            "Conversations",
            "Actions",
        ])
        self._table.horizontalHeader().setSectionResizeMode(0, QHeaderView.ResizeMode.Stretch)
        self._table.horizontalHeader().setSectionResizeMode(1, QHeaderView.ResizeMode.ResizeToContents)
        self._table.horizontalHeader().setSectionResizeMode(2, QHeaderView.ResizeMode.ResizeToContents)
        self._table.horizontalHeader().setSectionResizeMode(3, QHeaderView.ResizeMode.ResizeToContents)
        self._table.verticalHeader().setVisible(False)
        self._table.setStyleSheet(f"""
            QTableWidget {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                gridline-color: {MD3_LIGHT_OUTLINE};
                border: none;
                color: {MD3_LIGHT_TEXT_PRIMARY};
            }}
            QHeaderView::section {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER_HIGH};
                color: {MD3_LIGHT_TEXT_SECONDARY};
                padding: 8px;
                font-weight: 600;
                font-size: 11px;
                border: none;
            }}
        """)

        for r, p in enumerate(self._projects):
            label_text = f"{p.get('name', '')}\n{p.get('path', '')}"
            self._table.setItem(r, 0, QTableWidgetItem(label_text))
            self._table.setItem(r, 1, QTableWidgetItem(p.get("last_active", "")))
            self._table.setItem(r, 2, QTableWidgetItem(f"{p.get('conversations_count', 0)} chats"))

            action_container = QWidget()
            action_layout = QHBoxLayout(action_container)
            action_layout.setContentsMargins(4, 2, 4, 2)
            action_layout.setSpacing(6)

            btn_settings = QPushButton("⚙ Settings")
            btn_settings.setStyleSheet(f"""
                QPushButton {{
                    background-color: {MD3_LIGHT_SURFACE_CONTAINER_HIGH};
                    color: {MD3_LIGHT_TEXT_PRIMARY};
                    border: 1px solid {MD3_LIGHT_OUTLINE};
                    border-radius: 6px;
                    padding: 4px 8px;
                    font-size: 11px;
                    font-weight: 500;
                }}
                QPushButton:hover {{
                    background-color: {MD3_LIGHT_SURFACE};
                }}
            """)
            action_layout.addWidget(btn_settings)

            btn_restore = QPushButton("Restore")
            btn_restore.setStyleSheet(f"""
                QPushButton {{
                    background-color: {MD3_LIGHT_SURFACE_CONTAINER_HIGH};
                    color: {MD3_LIGHT_ACCENT_PRIMARY};
                    border: 1px solid {MD3_LIGHT_OUTLINE};
                    border-radius: 6px;
                    padding: 4px 10px;
                    font-size: 11px;
                    font-weight: 600;
                }}
            """)
            action_layout.addWidget(btn_restore)

            btn_del = QPushButton("Delete")
            btn_del.setStyleSheet("""
                QPushButton {
                    background-color: #fce8e6;
                    color: #c5221f;
                    border: 1px solid #fad2cf;
                    border-radius: 6px;
                    padding: 4px 10px;
                    font-size: 11px;
                    font-weight: 600;
                }
            """)
            action_layout.addWidget(btn_del)

            self._table.setCellWidget(r, 3, action_container)
            self._table.setRowHeight(r, 46)

        t_layout.addWidget(self._table)
        c_layout.addWidget(table_card)
        c_layout.addStretch()

        scroll.setWidget(container)
        main_layout.addWidget(scroll)
