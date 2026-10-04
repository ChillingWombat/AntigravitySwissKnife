"""
App Enhancements & Usability Add-Ons Page.
==========================================
Interactive controls for in-app UI improvements:
- Prompt Jumper / Indicator Navigation
- Dimming or Hiding Tool Calls and Thinking Processes
- Conversation Turn Breaker Lines
- Predefined Project for New Conversations
- Project Drag & Reorder
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any, Optional
from PySide6.QtCore import Qt
from PySide6.QtWidgets import (
    QCheckBox,
    QColorDialog,
    QComboBox,
    QFrame,
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


class AppEnhancementsPage(QWidget):
    """
    App Enhancements & Usability Add-Ons settings page.
    """

    def __init__(self, parent: Optional[QWidget] = None) -> None:
        super().__init__(parent)
        self._config_path = Path.home() / ".config" / "antigravity-swiss" / "enhancements.json"
        self._data = self._load_data()
        self._init_ui()

    def _load_data(self) -> dict[str, Any]:
        defaults = {
            "prompt_jumper_enabled": True,
            "color_mode": "default",
            "custom_color": "",
            "dim_tools": False,
            "hide_tools": False,
            "breaker_lines": True,
            "project_reorder": True,
            "fixed_project_enabled": False,
            "fixed_project_name": "",
        }
        if self._config_path.exists():
            try:
                with open(self._config_path, "r", encoding="utf-8") as f:
                    data = json.load(f)
                    defaults.update(data)
            except Exception:
                pass
        return defaults

    def _save_data(self) -> None:
        self._config_path.parent.mkdir(parents=True, exist_ok=True)
        try:
            with open(self._config_path, "w", encoding="utf-8") as f:
                json.dump(self._data, f, indent=2)
        except Exception:
            pass

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
        title = QLabel("APP ENHANCEMENTS & USABILITY")
        title.setStyleSheet(f"font-size: 11px; font-weight: 700; color: {MD3_LIGHT_TEXT_SECONDARY}; letter-spacing: 1px;")
        subtitle = QLabel("Enhance and personalize your native Google Antigravity chat experience.")
        subtitle.setStyleSheet(f"font-size: 12px; color: {MD3_LIGHT_TEXT_SECONDARY};")
        title_box.addWidget(title)
        title_box.addWidget(subtitle)
        c_layout.addLayout(title_box)

        # Card 1: Prompt Jumper / Navigation Bar
        pj_card, pj_layout = self._create_card("Prompt Jumper & Prompt Markers")

        self.cb_jumper = QCheckBox("Enable Prompt Jumper rail in conversation views")
        self.cb_jumper.setChecked(bool(self._data.get("prompt_jumper_enabled", True)))
        self.cb_jumper.stateChanged.connect(self._on_jumper_toggled)
        pj_layout.addWidget(self.cb_jumper)

        color_row = QHBoxLayout()
        color_lbl = QLabel("Active Indicator Color Mode:")
        color_lbl.setStyleSheet(f"font-size: 12px; color: {MD3_LIGHT_TEXT_PRIMARY};")
        color_row.addWidget(color_lbl)

        self.combo_color = QComboBox()
        self.combo_color.addItems(["Default (Slate Grey #475569 / #94a3b8)", "Match Project Color", "Custom Accent Color"])
        current_mode = self._data.get("color_mode", "default")
        if current_mode == "project":
            self.combo_color.setCurrentIndex(1)
        elif current_mode == "custom":
            self.combo_color.setCurrentIndex(2)
        else:
            self.combo_color.setCurrentIndex(0)
        self.combo_color.currentIndexChanged.connect(self._on_color_mode_changed)
        color_row.addWidget(self.combo_color)

        self.btn_pick_color = QPushButton("Pick Color...")
        self.btn_pick_color.setStyleSheet(f"""
            QPushButton {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER_HIGH};
                color: {MD3_LIGHT_TEXT_PRIMARY};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 6px;
                padding: 4px 12px;
                font-size: 11px;
            }}
        """)
        self.btn_pick_color.clicked.connect(self._on_pick_color)
        color_row.addWidget(self.btn_pick_color)
        color_row.addStretch()
        pj_layout.addLayout(color_row)

        desc_hover = QLabel("Hovering on any indicator dash smoothly expands its length from 14px to 22px.")
        desc_hover.setStyleSheet(f"font-size: 11px; color: {MD3_LIGHT_TEXT_SECONDARY};")
        pj_layout.addWidget(desc_hover)

        c_layout.addWidget(pj_card)

        # Card 2: Visual Density & Thinking Process
        vd_card, vd_layout = self._create_card("Tool Calls & Thinking Process")

        self.cb_dim = QCheckBox("Dim / Grey out tool calls and agent thinking processes")
        self.cb_dim.setChecked(bool(self._data.get("dim_tools", False)))
        self.cb_dim.stateChanged.connect(self._on_dim_changed)
        vd_layout.addWidget(self.cb_dim)

        self.cb_hide = QCheckBox("Hide tool calls and thinking blocks completely (show answers only)")
        self.cb_hide.setChecked(bool(self._data.get("hide_tools", False)))
        self.cb_hide.stateChanged.connect(self._on_hide_changed)
        vd_layout.addWidget(self.cb_hide)

        self.cb_breaker = QCheckBox("Add visual breaker lines between agent answers and new user prompts")
        self.cb_breaker.setChecked(bool(self._data.get("breaker_lines", True)))
        self.cb_breaker.stateChanged.connect(self._on_breaker_changed)
        vd_layout.addWidget(self.cb_breaker)

        c_layout.addWidget(vd_card)

        # Card 3: Project Organization & Defaults
        po_card, po_layout = self._create_card("Project Organization & Navigation")

        self.cb_reorder = QCheckBox("Allow dragging project labels in sidebar to customize order")
        self.cb_reorder.setChecked(bool(self._data.get("project_reorder", True)))
        self.cb_reorder.stateChanged.connect(self._on_reorder_changed)
        po_layout.addWidget(self.cb_reorder)

        proj_row = QHBoxLayout()
        self.cb_fixed_project = QCheckBox("Set fixed predefined project for new conversations:")
        self.cb_fixed_project.setChecked(bool(self._data.get("fixed_project_enabled", False)))
        self.cb_fixed_project.stateChanged.connect(self._on_fixed_project_changed)
        proj_row.addWidget(self.cb_fixed_project)

        self.combo_fixed_project = QComboBox()
        self.combo_fixed_project.addItems(["Auto (Latest Active)", "Antigravity Swiss Knife", "Default Workspace"])
        self.combo_fixed_project.setStyleSheet(f"""
            QComboBox {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER_HIGH};
                color: {MD3_LIGHT_TEXT_PRIMARY};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 6px;
                padding: 4px 8px;
                font-size: 11px;
            }}
        """)
        proj_row.addWidget(self.combo_fixed_project)
        proj_row.addStretch()
        po_layout.addLayout(proj_row)

        c_layout.addWidget(po_card)
        c_layout.addStretch()

        scroll.setWidget(container)
        main_layout.addWidget(scroll)

    def _create_card(self, title_text: str) -> tuple[QFrame, QVBoxLayout]:
        card = QFrame()
        card.setObjectName("enhancementCard")
        card.setStyleSheet(f"""
            QFrame#enhancementCard {{
                background-color: {MD3_LIGHT_SURFACE_CONTAINER};
                border: 1px solid {MD3_LIGHT_OUTLINE};
                border-radius: 12px;
                padding: 16px;
            }}
            QFrame#enhancementCard QLabel {{
                border: none;
                background: transparent;
            }}
        """)
        layout = QVBoxLayout(card)
        layout.setContentsMargins(16, 16, 16, 16)
        layout.setSpacing(12)
        t = QLabel(title_text)
        t.setStyleSheet(f"font-size: 13px; font-weight: 600; color: {MD3_LIGHT_TEXT_PRIMARY}; border: none; background: transparent;")
        layout.addWidget(t)
        return card, layout

    def _on_jumper_toggled(self, state: int) -> None:
        self._data["prompt_jumper_enabled"] = bool(state)
        self._save_data()

    def _on_color_mode_changed(self, idx: int) -> None:
        modes = ["default", "project", "custom"]
        self._data["color_mode"] = modes[idx]
        self._save_data()

    def _on_pick_color(self) -> None:
        color = QColorDialog.getColor()
        if color.isValid():
            self._data["custom_color"] = color.name()
            self._data["color_mode"] = "custom"
            self.combo_color.setCurrentIndex(2)
            self._save_data()

    def _on_dim_changed(self, state: int) -> None:
        self._data["dim_tools"] = bool(state)
        self._save_data()

    def _on_hide_changed(self, state: int) -> None:
        self._data["hide_tools"] = bool(state)
        self._save_data()

    def _on_breaker_changed(self, state: int) -> None:
        self._data["breaker_lines"] = bool(state)
        self._save_data()

    def _on_reorder_changed(self, state: int) -> None:
        self._data["project_reorder"] = bool(state)
        self._save_data()

    def _on_fixed_project_changed(self, state: int) -> None:
        self._data["fixed_project_enabled"] = bool(state)
        self._save_data()
